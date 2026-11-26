import Foundation
import Dispatch

private final class LockedClock: @unchecked Sendable {
    private let lock = NSLock()
    private var value: TimeInterval
    private var queuedValues: [TimeInterval] = []
    init(_ value: TimeInterval) { self.value = value }
    func now() -> TimeInterval {
        lock.lock(); defer { lock.unlock() }
        if !queuedValues.isEmpty { value = queuedValues.removeFirst() }
        return value
    }
    func sequence(_ values: [TimeInterval]) { lock.lock(); queuedValues = values; lock.unlock() }
    func set(_ next: TimeInterval) { lock.lock(); value = next; lock.unlock() }
}

private final class HeldMainQueue: @unchecked Sendable {
    private let lock = NSLock()
    private var blocks: [@Sendable () -> Void] = []
    func enqueue(_ block: @escaping @Sendable () -> Void) {
        lock.lock(); blocks.append(block); lock.unlock()
    }
    var count: Int { lock.lock(); defer { lock.unlock() }; return blocks.count }
    func take() -> (@Sendable () -> Void)? {
        lock.lock(); defer { lock.unlock() }
        return blocks.isEmpty ? nil : blocks.removeFirst()
    }
}

@main
enum VerifyStartupLifecycleObserver {
    static var assertions = 0
    static func expect(_ condition: @autoclosure () -> Bool, _ reason: String) {
        assertions += 1
        if !condition() { fatalError(reason) }
    }

    static func read(_ path: URL) throws -> [[String: Any]] {
        let bytes = try Data(contentsOf: path)
        expect(bytes.count <= CherriStartupLifecycleObserver.maximumBytes, "bounded bytes")
        let text = String(data: bytes, encoding: .utf8)!
        return try text.split(separator: "\n").map {
            try JSONSerialization.jsonObject(with: Data($0.utf8)) as! [String: Any]
        }
    }

    static func main() throws {
        let nonce = String(repeating: "a", count: 32)
        let testID = "CherriEditorUITests/testD03CurrentDocumentationSearchUI"
        let env = ["CHERRI_STARTUP_OBSERVATION_NONCE": nonce,
                   "CHERRI_STARTUP_OBSERVATION_TEST": testID]
        let context = CherriStartupLifecycleObserver.Context(arguments: ["--uitesting"], environment: env)!
        var e07 = env
        e07["CHERRI_STARTUP_OBSERVATION_TEST"] = "CherriEditorUITests/testE07VisualParameterEditingCanvasUI"
        expect(CherriStartupLifecycleObserver.Context(arguments: ["--uitesting"], environment: e07) != nil,
               "the exact E07 diagnostic context uses the original nonce admission")
        expect(CherriStartupLifecycleObserver.Context(arguments: [], environment: env) == nil, "normal launch disabled")
        for bad in ["", "../private", String(repeating: "a", count: 31), String(repeating: "A", count: 32), nonce + "\n"] {
            var invalid = env; invalid["CHERRI_STARTUP_OBSERVATION_NONCE"] = bad
            expect(CherriStartupLifecycleObserver.Context(arguments: ["--uitesting"], environment: invalid) == nil, "invalid nonce")
        }
        var foreign = env; foreign["CHERRI_STARTUP_OBSERVATION_TEST"] = "private-test"
        expect(CherriStartupLifecycleObserver.Context(arguments: ["--uitesting"], environment: foreign) == nil, "unreviewed test")
        expect(CherriStartupLifecycleObserver.Context(arguments: ["--uitesting"], environment: [:]) == nil, "missing context")

        let root = FileManager.default.temporaryDirectory.appendingPathComponent("cherri-startup-observer-" + UUID().uuidString)
        try FileManager.default.createDirectory(at: root, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: root) }
        func path(_ part: String) -> URL { root.appendingPathComponent(part).appendingPathComponent(nonce + ".jsonl") }

        let clock = LockedClock(100)
        let observer = CherriStartupLifecycleObserver(context: context, output: path("events"),
                                                      now: { clock.now() }, lifetime: 120, interval: 120)
        observer.start()
        observer.record(.appInitialized)
        let document = CherriStartupLifecycleObserver.newDocumentID()
        observer.record(.documentInitialized, documentID: document)
        observer.record(.workspaceBody, documentID: document)
        observer.record(.workspaceBody, documentID: document)
        observer.record(.editorBody, documentID: "PRIVATE_SOURCE_DO_NOT_PUBLISH")
        observer.record(.heartbeat)
        observer.drainForTesting()
        let rows = try read(path("events"))
        expect(rows.count == 4, "one start, three actual phases, deduplicated")
        for (index, row) in rows.enumerated() {
            expect(Set(row.keys) == Set(["schemaVersion", "nonce", "testId", "sequence", "phase", "documentId",
                "observedUptimeMs", "writtenUptimeMs", "mainThread", "processId"]), "closed schema")
            expect(row["nonce"] as? String == nonce, "nonce bound")
            expect(row["testId"] as? String == testID, "test bound")
            expect(row["sequence"] as? Int == index, "sequence")
            expect(row["observedUptimeMs"] as? Int == 100_000, "observation clock")
            expect(row["writtenUptimeMs"] as? Int == 100_000, "write clock")
            expect(row["processId"] as? Int == Int(ProcessInfo.processInfo.processIdentifier), "owned process")
        }
        expect(rows[2]["documentId"] as? String == document, "document identity retained")
        expect(rows[3]["documentId"] as? String == document, "same workspace identity")
        let observedText = try String(contentsOf: path("events"), encoding: .utf8)
        expect(!observedText.contains("PRIVATE_SOURCE"), "invalid identifier not emitted")
        observer.stopAndDrainForTesting()

        let heartbeat = CherriStartupLifecycleObserver(context: context, output: path("heartbeat"),
                                                        lifetime: 0.30, interval: 0.02)
        heartbeat.start()
        // This blocks the caller; the independent writer queue must still progress.
        Thread.sleep(forTimeInterval: 0.12)
        heartbeat.drainForTesting()
        let heartbeats = try read(path("heartbeat")).filter { $0["phase"] as? String == "heartbeat" }
        expect(heartbeats.count >= 2, "real independent timer advanced")
        expect(heartbeats.allSatisfy { $0["mainThread"] as? Bool == false }, "heartbeat is not a main-thread readiness signal")
        heartbeat.stopAndDrainForTesting()

        let lateClock = LockedClock(200)
        let late = CherriStartupLifecycleObserver(context: context, output: path("late"),
                                                 now: { lateClock.now() }, lifetime: 1, interval: 1)
        late.start(); late.drainForTesting()
        lateClock.set(202); late.record(.workspaceBody, documentID: document); late.drainForTesting()
        let lateRows = try read(path("late"))
        expect(lateRows.map { $0["phase"] as! String } == ["observer-opened", "observer-expired"], "late lifecycle event rejected")
        late.stopAndDrainForTesting()



        var association = CherriStartupDocumentAssociation(documentID: document)
        expect(!association.update(document), "same document identity is not a change")
        let nextDocument = CherriStartupLifecycleObserver.newDocumentID()
        expect(association.update(nextDocument), "different document updates association")
        expect(association.documentID == nextDocument, "current document identity is B")
        let identityObserver = CherriStartupLifecycleObserver(context: context, output: path("identity"),
                                                              lifetime: 120, interval: 120)
        identityObserver.start()
        identityObserver.record(.editorContainerDocumentChanged, documentID: association.documentID, once: false)
        identityObserver.record(.editorContainerAttached, documentID: association.documentID, once: false)
        identityObserver.drainForTesting()
        let identityRows = try read(path("identity"))
        expect(identityRows.count == 3, "one actual association and later container attachment")
        expect(identityRows[1]["documentId"] as? String == nextDocument, "changed association names B")
        expect(identityRows[2]["documentId"] as? String == nextDocument, "later event cannot retain stale A")
        expect(association.update(nil) && association.documentID == nil, "unknown association is explicit")
        identityObserver.stopAndDrainForTesting()

        let queuedClock = LockedClock(200)
        let queued = CherriStartupLifecycleObserver(context: context, output: path("queued-expiry"),
                                                   now: { queuedClock.now() }, lifetime: 1, interval: 1)
        queued.start(); queued.drainForTesting()
        queuedClock.sequence([200, 202, 202])
        queued.record(.workspaceBody, documentID: document)
        queued.drainForTesting()
        let queuedRows = try read(path("queued-expiry"))
        expect(queuedRows.count == 2 && queuedRows[1]["phase"] as? String == "observer-expired", "queued event expires")
        expect(queuedRows[1]["observedUptimeMs"] as? Int == 202_000, "expiry uses actual writer decision time, not original event time")
        queued.stopAndDrainForTesting()

        let reverseClock = LockedClock(200)
        let reverse = CherriStartupLifecycleObserver(context: context, output: path("reverse"),
                                                    now: { reverseClock.now() }, lifetime: 1, interval: 1)
        reverse.start(); reverse.drainForTesting()
        reverseClock.set(199); reverse.record(.workspaceBody, documentID: document); reverse.drainForTesting()
        let reversedRows = try read(path("reverse"))
        expect(reversedRows.count == 1, "backward clock cannot produce lifecycle observation")
        reverse.stopAndDrainForTesting()

        let cap = CherriStartupLifecycleObserver(context: context, output: path("cap"),
                                                lifetime: 120, interval: 120, maximum: 4)
        cap.start()
        for _ in 0..<20 { cap.record(.editorContainerAttached, documentID: document, once: false) }
        cap.drainForTesting()
        let capRows = try read(path("cap"))
        expect(capRows.count == 4, "hard event cap")
        expect(capRows.last?["phase"] as? String == "event-limit-reached", "cap remains explicit")
        cap.stopAndDrainForTesting()

        let existing = path("existing")
        try FileManager.default.createDirectory(at: existing.deletingLastPathComponent(), withIntermediateDirectories: true)
        let original = Data("ORIGINAL_OBSERVATION".utf8)
        try original.write(to: existing)
        let duplicate = CherriStartupLifecycleObserver(context: context, output: existing)
        duplicate.start(); duplicate.record(.appInitialized); duplicate.stopAndDrainForTesting()
        let retained = try Data(contentsOf: existing)
        expect(retained == original, "prior nonce evidence preserved")

        for (label, lifetime, interval, maximum) in [
            ("bad-life", 121.0, 1.0, 96), ("bad-interval", 1.0, 2.0, 96),
            ("bad-cap", 1.0, 1.0, 97), ("small-cap", 1.0, 1.0, 1),
        ] {
            let bad = CherriStartupLifecycleObserver(context: context, output: path(label),
                                                     lifetime: lifetime, interval: interval, maximum: maximum)
            bad.start(); bad.record(.appInitialized); bad.stopAndDrainForTesting()
            expect(!FileManager.default.fileExists(atPath: path(label).path), "invalid bounds never write")
        }
        let invalidClock = CherriStartupLifecycleObserver(context: context, output: path("nonfinite"), now: { .nan })
        invalidClock.start(); invalidClock.record(.appInitialized); invalidClock.stopAndDrainForTesting()
        expect(!FileManager.default.fileExists(atPath: path("nonfinite").path), "invalid clock never writes")

        let heldMain = HeldMainQueue()
        let probeClock = LockedClock(300)
        let probe = CherriStartupLifecycleObserver(context: context, output: path("main-probe"),
            now: { probeClock.now() }, lifetime: 120, interval: 120,
            enqueueMain: { heldMain.enqueue($0) })
        probe.start()
        for _ in 0..<12 { probe.observeMainQueueOnce() }
        probe.record(.mainQueueExecuted)
        probe.drainForTesting()
        let pendingRows = try read(path("main-probe"))
        expect(heldMain.count == 1, "at most one queued block")
        expect(pendingRows.map { $0["phase"] as! String } == ["observer-opened", "main-queue-enqueued"],
               "ordinary record cannot manufacture main execution")
        probeClock.set(301)
        heldMain.take()?()
        probe.drainForTesting()
        let executedRows = try read(path("main-probe"))
        expect(executedRows.count == 3, "one exact response")
        expect(executedRows[2]["phase"] as? String == "main-queue-executed", "response is distinct")
        expect(executedRows[2]["observedUptimeMs"] as? Int == 301_000, "actual response observation time")
        expect(executedRows[2]["mainThread"] as? Bool == Thread.isMainThread, "actual caller thread observed")
        probe.observeMainQueueOnce(); probe.drainForTesting()
        expect(heldMain.count == 0, "no second request after response")
        probe.stopAndDrainForTesting()

        let expiredQueue = HeldMainQueue()
        let expiryClock = LockedClock(500)
        let expiredProbe = CherriStartupLifecycleObserver(context: context, output: path("expired-main-probe"),
            now: { expiryClock.now() }, lifetime: 1, interval: 1,
            enqueueMain: { expiredQueue.enqueue($0) })
        expiredProbe.start(); expiredProbe.observeMainQueueOnce(); expiredProbe.drainForTesting()
        expiryClock.set(502); expiredQueue.take()?(); expiredProbe.drainForTesting()
        let expiredProbeRows = try read(path("expired-main-probe"))
        expect(expiredProbeRows.map { $0["phase"] as! String } ==
            ["observer-opened", "main-queue-enqueued", "observer-expired"], "late response cannot claim progress")
        expiredProbe.stopAndDrainForTesting()

        print("{\"status\":\"PASS_LOCAL_OBSERVER_CONTROLS\",\"assertions\":\(assertions),\"nativeAppExecution\":false,\"readinessDecisions\":false}")
    }
}
