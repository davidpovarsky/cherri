import Foundation

struct NoInputVector: Decodable {
    let name: String
    let contract: String
    let accepted: Bool
    let reason: String?
}

final class UIWindowScene {
    func open(_ url: URL, options: Any?, completionHandler: @escaping (Bool) -> Void) {
        UIApplication.shared.sceneURLs.append(url.absoluteString)
        completionHandler(true)
    }
}

/// Only this local Foundation executable supplies the platform stub. The
/// compiled trigger method below is copied byte-for-byte from ReceiverApp.swift.
final class UIApplication {
    static let shared = UIApplication()
    var primaryURLs: [String] = []
    var sceneURLs: [String] = []
    var completionSuccess = true
    var willOpen: (() -> Void)?
    var connectedScenes: [Any] { [UIWindowScene()] }
    func canOpenURL(_ url: URL) -> Bool { true }
    func open(_ url: URL, options: [String: Any],
              completionHandler: @escaping (Bool) -> Void) {
        willOpen?()
        primaryURLs.append(url.absoluteString)
        completionHandler(completionSuccess)
    }
    func clear(success: Bool) {
        primaryURLs = []
        sceneURLs = []
        completionSuccess = success
        willOpen = nil
    }
}

final class ActualStoreBoundary {
    let primaryNoInputLaunchGate: QAPrimaryNoInputLaunchGate
    let root: URL
    var events: [Int] = [1]
    var lastStatus = "OLD", lastRunId = "OLD", lastResult = "OLD", lastTriggerStatus = "OLD"
    var eventsFileURL: URL { root.appendingPathComponent("callback-events.json") }
    var triggerDebugFileURL: URL { root.appendingPathComponent("trigger-debug.txt") }
    init(raw: String?, root: URL) {
        self.root = root
        primaryNoInputLaunchGate = QAPrimaryNoInputLaunchGate(rawContract: raw, evidenceRoot: root)
    }
    func logTrigger(_ message: String) {}
    private func persist() {}
/*ACTUAL_RESET_METHOD*/
}

final class ActualTriggerBoundary {
    let store: ActualStoreBoundary
    init(_ store: ActualStoreBoundary) { self.store = store }
    func invoke(_ url: String, source: String) {
        triggerExternalShortcutURL(url, source: source, delay: 0)
    }
/*ACTUAL_TRIGGER_METHOD*/
}

@main enum VerifyPrimaryNoInput {
    static func main() throws {
        let fm = FileManager.default
        let input = URL(fileURLWithPath: CommandLine.arguments[1])
        let root = URL(fileURLWithPath: CommandLine.arguments[2])
        try fm.createDirectory(at: root, withIntermediateDirectories: false)
        let vectors = try JSONDecoder().decode([NoInputVector].self, from: Data(contentsOf: input))
        var checks: [[String: Any]] = []
        func require(_ condition: Bool, _ name: String, scope: String = "actual-policy") {
            checks.append(["name": name, "outcome": condition ? "PASS" : "FAIL", "scope": scope])
        }
        for vector in vectors {
            do {
                let contract = try QAPrimaryNoInputContract(vector.contract)
                require(vector.accepted && contract.original == Data(vector.contract.utf8),
                        "contract/" + vector.name)
            } catch {
                let reason = (error as? QAPrimaryNoInputFailure)?.rawValue
                require(!vector.accepted && (vector.reason == nil || reason == vector.reason),
                        "contract/" + vector.name)
            }
        }
        let healthyRaw = vectors.first { $0.name == "healthy" }!.contract
        let healthy = try QAPrimaryNoInputContract(healthyRaw)
        func makeStore(_ name: String, raw: String? = healthyRaw) throws -> ActualStoreBoundary {
            let directory = root.appendingPathComponent(name, isDirectory: true)
            try fm.createDirectory(at: directory, withIntermediateDirectories: false)
            return ActualStoreBoundary(raw: raw, root: directory)
        }
        func directory(_ store: ActualStoreBoundary) -> URL {
            store.root.appendingPathComponent("primary-no-input-launch", isDirectory: true)
                .appendingPathComponent(healthy.sha256, isDirectory: true)
        }
        func rows(_ store: ActualStoreBoundary) -> [[String: Any]] {
            let urls = (try? fm.contentsOfDirectory(at: directory(store),
                            includingPropertiesForKeys: nil)) ?? []
            return urls.filter { $0.lastPathComponent.contains("-") && $0.pathExtension == "json" }
                .sorted { $0.lastPathComponent < $1.lastPathComponent }
                .compactMap { try? JSONSerialization.jsonObject(with: Data(contentsOf: $0)) as? [String: Any] }
        }
        func drain() throws {
            var drained = false
            DispatchQueue.main.asyncAfter(deadline: .now() + 0.05) { drained = true }
            let deadline = Date().addingTimeInterval(2)
            while !drained && Date() < deadline { RunLoop.main.run(until: Date().addingTimeInterval(0.005)) }
            if !drained { throw NSError(domain: "PrimaryNoInputHarness", code: 1) }
        }
        let app = UIApplication.shared

        // This is the real pre-existing receiver callsite and real gate with
        // only UIApplication/UIWindowScene replaced by observable test doubles.
        let absent = try makeStore("unconfigured", raw: nil)
        app.clear(success: false)
        ActualTriggerBoundary(absent).invoke("shortcuts://run-shortcut?name=ordinary", source: "env")
        try drain()
        require(app.primaryURLs.count == 1 && app.sceneURLs.count == 1,
                "callsite/unconfigured_preserves_fallback", scope: "actual-trigger-method")
        require(!fm.fileExists(atPath: absent.root.appendingPathComponent("primary-no-input-launch").path),
                "callsite/unconfigured_does_not_create_contract_evidence")

        for success in [true, false] {
            let store = try makeStore(success ? "completed-true" : "completed-false")
            let trigger = ActualTriggerBoundary(store)
            app.clear(success: success)
            var intentBeforeOpen = false
            app.willOpen = { intentBeforeOpen = rows(store).contains { $0["event"] as? String == "intent" } }
            trigger.invoke(healthy.primaryURL, source: "env")
            try drain()
            let originals = rows(store)
            require(app.primaryURLs == [healthy.primaryURL] && app.sceneURLs.isEmpty,
                    "callsite/first_only_" + String(success), scope: "actual-trigger-method")
            require(intentBeforeOpen, "callsite/intent_precedes_open_" + String(success),
                    scope: "actual-trigger-method")
            require(originals.map { $0["event"] as? String } == ["intent", "completion"] &&
                    originals.map { $0["sequence"] as? Int } == [1, 2] &&
                    originals.allSatisfy { $0["attemptOrdinal"] as? Int == 1 } &&
                    originals.last?["success"] as? Bool == success &&
                    originals.allSatisfy { $0["retentionFailure"] as? Bool == false },
                    "callsite/ordered_receipts_" + String(success), scope: "actual-trigger-method")
            let originalContract = try? Data(contentsOf: directory(store).appendingPathComponent("contract.original.json"))
            require(originalContract == Data(healthyRaw.utf8), "callsite/exact_contract_" + String(success))
            trigger.invoke(healthy.primaryURL, source: "argv")
            store.resetStore()
            trigger.invoke(healthy.primaryURL, source: "onOpenURL")
            try drain()
            require(app.primaryURLs.count == 1 && app.sceneURLs.isEmpty,
                    "callsite/reset_cannot_retry_" + String(success), scope: "actual-trigger-and-reset-methods")
            let after = rows(store)
            require(after.count == 4 &&
                    after.suffix(2).allSatisfy { $0["event"] as? String == "rejection" &&
                        $0["attemptOrdinal"] as? Int == 1 &&
                        $0["reason"] as? String == "ATTEMPT_ALREADY_CONSUMED" &&
                        $0["requestedURLUTF8Base64"] as? String == Data(healthy.primaryURL.utf8).base64EncodedString() },
                    "callsite/rejections_are_not_new_attempts_" + String(success))
        }

        let wrong = try makeStore("wrong-first")
        app.clear(success: true)
        let wrongURL = healthy.primaryURL + "&input=text"
        let wrongTrigger = ActualTriggerBoundary(wrong)
        wrongTrigger.invoke(wrongURL, source: "env")
        try drain()
        wrongTrigger.invoke(healthy.primaryURL, source: "argv")
        try drain()
        require(app.primaryURLs.isEmpty && app.sceneURLs.isEmpty, "callsite/wrong_first_consumes_attempt",
                scope: "actual-trigger-method")
        let wrongRows = rows(wrong)
        require(wrongRows.count == 2 &&
                wrongRows.first?["requestedURLUTF8Base64"] as? String == Data(wrongURL.utf8).base64EncodedString() &&
                wrongRows.allSatisfy { $0["attemptOrdinal"] as? Int == 0 },
                "callsite/rejected_url_exact_private_original")

        let malformed = try makeStore("malformed", raw: healthyRaw + "\n")
        app.clear(success: true)
        ActualTriggerBoundary(malformed).invoke(healthy.primaryURL, source: "env")
        try drain()
        require(app.primaryURLs.isEmpty && app.sceneURLs.isEmpty, "callsite/malformed_present_fails_closed",
                scope: "actual-trigger-method")

        let rootMissing = root.appendingPathComponent("missing-root", isDirectory: true)
        let missing = ActualStoreBoundary(raw: healthyRaw, root: rootMissing)
        app.clear(success: true)
        ActualTriggerBoundary(missing).invoke(healthy.primaryURL, source: "env")
        try drain()
        require(app.primaryURLs.isEmpty && app.sceneURLs.isEmpty, "callsite/evidence_failure_blocks_open",
                scope: "actual-trigger-method")

        let collision = try makeStore("completion-collision")
        app.clear(success: false)
        let marker = Data("preserved-existing-file".utf8)
        app.willOpen = {
            try? marker.write(to: directory(collision).appendingPathComponent("002-completion.original.json"),
                              options: .withoutOverwriting)
        }
        ActualTriggerBoundary(collision).invoke(healthy.primaryURL, source: "env")
        try drain()
        require(app.primaryURLs.count == 1 && app.sceneURLs.isEmpty,
                "callsite/completion_write_failure_never_falls_back", scope: "actual-trigger-method")
        require((try? Data(contentsOf: directory(collision).appendingPathComponent("002-completion.original.json"))) == marker,
                "callsite/completion_original_not_overwritten")

        for source in ["env", "argv", "onOpenURL"] {
            let store = try makeStore("source-" + source)
            let permit = store.primaryNoInputLaunchGate.begin(rawURL: healthy.primaryURL, source: source)
            require(permit.allowed && !permit.allowsFallback && permit.reason == nil,
                    "gate/accepted_source_" + source)
            require(store.primaryNoInputLaunchGate.complete(permit, success: false), "gate/completion_" + source)
            require(!store.primaryNoInputLaunchGate.complete(permit, success: true), "gate/duplicate_completion_" + source)
            require(rows(store).count == 2, "gate/duplicate_completion_has_no_new_receipt_" + source)
        }
        let badSource = try makeStore("bad-source")
        let rejected = badSource.primaryNoInputLaunchGate.begin(rawURL: healthy.primaryURL, source: "unknown")
        require(!rejected.allowed && !rejected.allowsFallback && rejected.reason == "TRIGGER_SOURCE",
                "gate/unknown_source_rejected")
        require(!badSource.primaryNoInputLaunchGate.begin(rawURL: healthy.primaryURL, source: "env").allowed,
                "gate/unknown_source_cannot_retry")

        let stale = try makeStore("stale-directory")
        let first = stale.primaryNoInputLaunchGate.begin(rawURL: healthy.primaryURL, source: "env")
        require(first.allowed, "gate/original_owner_created")
        let originalOwner = try Data(contentsOf: directory(stale).appendingPathComponent("owner.original.json"))
        let newProcess = QAPrimaryNoInputLaunchGate(rawContract: healthyRaw, evidenceRoot: stale.root)
        let replay = newProcess.begin(rawURL: healthy.primaryURL, source: "env")
        require(!replay.allowed && replay.reason == "ORIGINAL_EVIDENCE_WRITE", "gate/existing_owner_cannot_replay")
        require(try Data(contentsOf: directory(stale).appendingPathComponent("owner.original.json")) == originalOwner,
                "gate/existing_owner_bytes_preserved")

        let oversized = try makeStore("oversized-url")
        let oversizedURL = String(repeating: "X", count: 65537)
        let oversizedPermit = oversized.primaryNoInputLaunchGate.begin(rawURL: oversizedURL, source: "env")
        let oversizedRows = rows(oversized)
        require(!oversizedPermit.allowed && oversizedPermit.reason == "ORIGINAL_EVIDENCE_WRITE",
                "gate/oversized_original_failure_blocks_open")
        require(oversizedRows.count == 1 && oversizedRows[0]["retentionFailure"] as? Bool == true &&
                oversizedRows[0]["requestedURLOriginalRetained"] as? Bool == false &&
                oversizedRows[0]["requestedURLUTF8Bytes"] as? Int == 65537 &&
                oversizedRows[0]["requestedURL"] == nil && oversizedRows[0]["requestedURLUTF8Base64"] == nil,
                "gate/oversized_original_is_not_claimed_retained")

        let boundedRejection = try makeStore("unicode-rejection")
        let unicodeURL = "shortcuts://run-shortcut?name=שלום🙂"
        let unicodePermit = boundedRejection.primaryNoInputLaunchGate.begin(rawURL: unicodeURL, source: "env")
        let unicodeRows = rows(boundedRejection)
        require(!unicodePermit.allowed && unicodePermit.reason == "REQUESTED_URL_MISMATCH" &&
                unicodeRows.first?["requestedURLUTF8Base64"] as? String == Data(unicodeURL.utf8).base64EncodedString() &&
                unicodeRows.first?["requestedURLUTF8Bytes"] as? Int == unicodeURL.utf8.count,
                "gate/rejected_unicode_utf8_is_exact")

        let failures = checks.filter { $0["outcome"] as? String == "FAIL" }
        let result: [String: Any] = [
            "schema": "cherri-primary-no-input-policy-controls/1", "checks": checks,
            "count": checks.count, "failures": failures.count,
            "actualReceiverTriggerAndResetMethods": true,
            "UIApplicationImplementation": "LOCAL_TEST_DOUBLE",
            "appleRuntimeExecuted": false, "formalAcceptanceGranted": false
        ]
        let resultData = try JSONSerialization.data(withJSONObject: result, options: [.sortedKeys])
        try resultData.write(to: root.appendingPathComponent("result.original.json"), options: .withoutOverwriting)
        print("PRIMARY_NO_INPUT_POLICY_CONTROLS count=\(checks.count) failures=\(failures.count) nativeRuntime=NOT_RUN")
        if !failures.isEmpty {
            throw NSError(domain: "PrimaryNoInputControls", code: failures.count,
                          userInfo: [NSLocalizedDescriptionKey: "Actual policy controls failed"])
        }
    }
}
