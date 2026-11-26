import Foundation
import Dispatch

/// Event-only diagnostics for fixed UI tests. This observer never queries
/// accessibility, creates a document, or participates in readiness decisions.
final class CherriStartupLifecycleObserver: @unchecked Sendable {
    enum Phase: String, Sendable {
        case observerOpened = "observer-opened"
        case heartbeat
        case mainQueueEnqueued = "main-queue-enqueued"
        case mainQueueExecuted = "main-queue-executed"
        case observerExpired = "observer-expired"
        case observerStopped = "observer-stopped"
        case eventLimitReached = "event-limit-reached"
        case appInitialized = "app-initialized"
        case documentInitialized = "document-initialized"
        case documentReadStarted = "document-read-started"
        case documentReadFinished = "document-read-finished"
        case documentWriteStarted = "document-write-started"
        case documentWriteFinished = "document-write-finished"
        case workspaceBody = "workspace-body"
        case workspaceAppeared = "workspace-appeared"
        case editorBody = "editor-host-body"
        case editorAppeared = "editor-host-appeared"
        case editorContainerCreated = "editor-container-created"
        case editorContainerDocumentChanged = "editor-container-document-changed"
        case editorContainerAttached = "editor-container-attached"
        case editorContainerDetached = "editor-container-detached"
    }

    struct Context: Sendable {
        static let tests: Set<String> = [
            "CherriEditorUITests/testBRG27EditorRealWorldWorkflowUI",
            "CherriEditorUITests/testD03CurrentDocumentationSearchUI",
            "CherriEditorUITests/testE01CompletionDisplayAndInsertionUI",
            "CherriEditorUITests/testE07VisualParameterEditingCanvasUI",
        ]
        let nonce: String
        let testID: String

        init?(arguments: [String], environment: [String: String]) {
            guard arguments.contains("--uitesting"),
                  let nonce = environment["CHERRI_STARTUP_OBSERVATION_NONCE"],
                  nonce.utf8.count == 32,
                  nonce.range(of: "^[0-9a-f]{32}$", options: .regularExpression) != nil,
                  let test = environment["CHERRI_STARTUP_OBSERVATION_TEST"],
                  Self.tests.contains(test) else { return nil }
            self.nonce = nonce
            self.testID = test
        }
    }

    static let directoryName = "CherriStartupObservation"
    static let maximumEvents = 96
    static let maximumBytes = 65_536
    static let lifetimeSeconds: TimeInterval = 120
    static let shared: CherriStartupLifecycleObserver? = {
        guard let context = Context(arguments: CommandLine.arguments,
                                    environment: ProcessInfo.processInfo.environment),
              let caches = FileManager.default.urls(for: .cachesDirectory, in: .userDomainMask).first else {
            return nil
        }
        let output = caches.appendingPathComponent(directoryName, isDirectory: true)
            .appendingPathComponent(context.nonce + ".jsonl", isDirectory: false)
        let observer = CherriStartupLifecycleObserver(context: context, output: output)
        observer.start()
        observer.observeMainQueueOnce()
        return observer
    }()

    private let context: Context
    private let output: URL
    private let now: @Sendable () -> TimeInterval
    private let lifetime: TimeInterval
    private let interval: TimeInterval
    private let maximum: Int
    private let enqueueMain: @Sendable (@escaping @Sendable () -> Void) -> Void
    private var mainQueueProbeRequested = false
    // Mutable writer state is confined to this queue; callers capture only fixed values.
    private let queue = DispatchQueue(label: "org.cherrilang.startup-observation", qos: .utility)
    private var file: FileHandle?
    private var timer: DispatchSourceTimer?
    private var startedAt: TimeInterval?
    private var writtenBytes = 0
    private var sequence = 0
    private var seen: Set<String> = []
    private var closed = false

    init(context: Context, output: URL,
         now: @escaping @Sendable () -> TimeInterval = { ProcessInfo.processInfo.systemUptime },
         lifetime: TimeInterval = lifetimeSeconds, interval: TimeInterval = 2,
         maximum: Int = maximumEvents,
         enqueueMain: @escaping @Sendable (@escaping @Sendable () -> Void) -> Void = {
             DispatchQueue.main.async(execute: $0)
         }) {
        self.context = context
        self.output = output
        self.now = now
        self.lifetime = lifetime
        self.interval = interval
        self.maximum = maximum
        self.enqueueMain = enqueueMain
    }

    static func newDocumentID() -> String {
        UUID().uuidString.replacingOccurrences(of: "-", with: "").lowercased()
    }

    private static func milliseconds(_ seconds: TimeInterval) -> Int? {
        guard seconds.isFinite, seconds >= 0, seconds <= 9_000_000_000 else { return nil }
        return Int((seconds * 1_000).rounded(.down))
    }

    func start() {
        queue.async { [self] in
            guard startedAt == nil, !closed,
                  lifetime.isFinite, lifetime > 0, lifetime <= Self.lifetimeSeconds,
                  interval.isFinite, interval > 0, interval <= lifetime,
                  (2...Self.maximumEvents).contains(maximum),
                  let first = Self.milliseconds(now()) else { return }
            startedAt = TimeInterval(first) / 1_000
            do {
                try FileManager.default.createDirectory(at: output.deletingLastPathComponent(),
                                                        withIntermediateDirectories: true)
                // A repeated nonce must leave the original observation untouched.
                try Data().write(to: output, options: .withoutOverwriting)
                file = try FileHandle(forWritingTo: output)
            } catch {
                closed = true
                return
            }
            emit(.observerOpened, documentID: nil, observed: first, mainThread: false)
            let timer = DispatchSource.makeTimerSource(queue: queue)
            timer.schedule(deadline: .now() + interval, repeating: interval)
            timer.setEventHandler { [weak self] in
                guard let self, !self.closed else { return }
                guard let start = self.startedAt,
                      let timestamp = Self.milliseconds(self.now()) else {
                    self.close()
                    return
                }
                let elapsed = self.now() - start
                guard elapsed.isFinite, elapsed >= 0 else {
                    self.close()
                    return
                }
                if elapsed >= self.lifetime {
                    self.emit(.observerExpired, documentID: nil, observed: timestamp, mainThread: false)
                    self.close()
                } else {
                    self.emit(.heartbeat, documentID: nil, observed: timestamp, mainThread: false)
                }
            }
            self.timer = timer
            timer.resume()
        }
    }

    /// One event-only request. A missing response is never a readiness verdict.
    /// Only this observer's writer queue owns the one-shot reservation.
    func observeMainQueueOnce() {
        queue.async { [self] in
            guard !closed, file != nil, !mainQueueProbeRequested,
                  let start = startedAt, let timestamp = Self.milliseconds(now()),
                  now() >= start, now() - start < lifetime else { return }
            mainQueueProbeRequested = true
            emit(.mainQueueEnqueued, documentID: nil, observed: timestamp, mainThread: false)
            guard !closed else { return }
            enqueueMain { [weak self] in
                guard let self, let observed = Self.milliseconds(self.now()) else { return }
                let main = Thread.isMainThread
                self.queue.async { [self] in
                    guard !self.closed, self.file != nil, let start = self.startedAt else { return }
                    let decisionTime = self.now()
                    guard decisionTime.isFinite, decisionTime >= start else { self.close(); return }
                    guard decisionTime - start < self.lifetime else {
                        if let expiredAt = Self.milliseconds(decisionTime) {
                            self.emit(.observerExpired, documentID: nil, observed: expiredAt, mainThread: main)
                        }
                        self.close()
                        return
                    }
                    self.emit(.mainQueueExecuted, documentID: nil, observed: observed, mainThread: main)
                }
            }
        }
    }

    func record(_ phase: Phase, documentID: String? = nil, once: Bool = true) {
        guard let observed = Self.milliseconds(now()) else { return }
        let main = Thread.isMainThread
        queue.async { [self] in
            guard !closed, file != nil, let start = startedAt else { return }
            guard ![Phase.observerOpened, .heartbeat, .observerExpired, .observerStopped, .eventLimitReached, .mainQueueEnqueued, .mainQueueExecuted].contains(phase) else { return }
            let decisionTime = now()
            let elapsed = decisionTime - start
            guard elapsed.isFinite, elapsed >= 0 else {
                close()
                return
            }
            if elapsed >= lifetime {
                guard let expiredAt = Self.milliseconds(decisionTime) else { close(); return }
                emit(.observerExpired, documentID: nil, observed: expiredAt, mainThread: main)
                close()
                return
            }
            if let documentID,
               documentID.utf8.count != 32 || documentID.range(of: "^[0-9a-f]{32}$", options: .regularExpression) == nil { return }
            let key = phase.rawValue + ":" + (documentID ?? "-")
            guard !once || !seen.contains(key) else { return }
            if once { seen.insert(key) }
            emit(phase, documentID: documentID, observed: observed, mainThread: main)
        }
    }

    private func emit(_ phase: Phase, documentID: String?, observed: Int, mainThread: Bool) {
        guard !closed, let file else { return }
        guard let written = Self.milliseconds(now()) else {
            close()
            return
        }
        let capped = sequence >= maximum - 1
        let row: [String: Any] = [
            "schemaVersion": "1", "nonce": context.nonce, "testId": context.testID,
            "sequence": sequence, "phase": capped ? Phase.eventLimitReached.rawValue : phase.rawValue,
            "documentId": capped ? NSNull() : (documentID as Any? ?? NSNull()),
            "observedUptimeMs": observed, "writtenUptimeMs": written,
            "mainThread": mainThread, "processId": ProcessInfo.processInfo.processIdentifier,
        ]
        guard let bytes = try? JSONSerialization.data(withJSONObject: row, options: [.sortedKeys]),
              writtenBytes + bytes.count + 1 <= Self.maximumBytes else {
            close()
            return
        }
        do {
            try file.write(contentsOf: bytes + Data([10]))
            writtenBytes += bytes.count + 1
            sequence += 1
        } catch {
            close()
            return
        }
        if capped { close() }
    }

    private func close() {
        timer?.cancel()
        timer = nil
        try? file?.close()
        file = nil
        closed = true
    }

    /// Used only by the standalone observer controls, never by app readiness.
    func stopAndDrainForTesting() {
        queue.sync {
            if !closed, let timestamp = Self.milliseconds(now()) {
                emit(.observerStopped, documentID: nil, observed: timestamp, mainThread: false)
            }
            close()
        }
    }

    func drainForTesting() { queue.sync {} }
}

/// The owned UIKit witness reads this current association for subsequent events.
/// A document change is not a fabricated window attachment or editor-ready event.
struct CherriStartupDocumentAssociation: Sendable {
    private(set) var documentID: String?

    @discardableResult
    mutating func update(_ next: String?) -> Bool {
        guard next != documentID else { return false }
        documentID = next
        return true
    }
}
