import XCTest

private struct DifferentialCommand: Decodable {
    let id: String
    let operation: String
    let shortcutName: String?
}

final class CherriDifferentialBatchUITests: XCTestCase {
    private var runDirectory: URL!
    private let shortcuts = XCUIApplication(bundleIdentifier: "com.apple.shortcuts")
    private let springboard = XCUIApplication(bundleIdentifier: "com.apple.springboard")

    override func setUpWithError() throws {
        continueAfterFailure = true
        executionTimeAllowance = 3300
        let path = try XCTUnwrap(ProcessInfo.processInfo.environment["CHERRI_QA_DIFF_RUN_DIR"])
        runDirectory = URL(fileURLWithPath: path, isDirectory: true)
        try FileManager.default.createDirectory(at: runDirectory, withIntermediateDirectories: true)
    }

    private func write(_ value: [String: Any], to name: String) throws {
        let data = try JSONSerialization.data(withJSONObject: value, options: [.prettyPrinted, .sortedKeys])
        try data.write(to: runDirectory.appendingPathComponent(name), options: .atomic)
    }

    private func capture(_ name: String) {
        try? XCUIScreen.main.screenshot().pngRepresentation.write(
            to: runDirectory.appendingPathComponent(name + ".png"), options: .atomic)
    }

    private func hasName(_ snapshot: XCUIElementSnapshot, name: String) -> Bool {
        if snapshot.label.contains(name) || snapshot.identifier.contains(name) { return true }
        return snapshot.children.contains { hasName($0, name: name) }
    }

    private func consentFrame(_ snapshot: XCUIElementSnapshot, label: String) -> CGRect? {
        if snapshot.label == label, snapshot.isEnabled, !snapshot.frame.isEmpty {
            return snapshot.frame
        }
        for child in snapshot.children {
            if let result = consentFrame(child, label: label) { return result }
        }
        return nil
    }

    private func importShortcut(_ command: DifferentialCommand) throws -> [String: Any] {
        let name = try XCTUnwrap(command.shortcutName)
        if shortcuts.state != .runningForeground {
            shortcuts.activate()
            _ = shortcuts.wait(for: .runningForeground, timeout: 8)
        }
        let button = shortcuts.buttons.matching(
            NSPredicate(format: "label == 'Add Shortcut' OR label == '+ Add Shortcut' OR label == 'Replace'")).firstMatch
        var timing = QADifferentialImportTiming(searchStartedAt: Date())
        let dispatchPath = runDirectory.appendingPathComponent("dispatch-" + command.id + ".json")
        let formatter = ISO8601DateFormatter()
        formatter.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
        var timingReceipt: [String: Any] = [
            "contract": "dispatch-exit-anchored-add-v1",
            "dispatchReceipt": dispatchPath.lastPathComponent,
            "addSearchStartedUTC": formatter.string(from: timing.searchStartedAt)
        ]
        func observeDispatch() -> QADifferentialImportTiming.DispatchState {
            let data = try? Data(contentsOf: dispatchPath)
            let now = Date()
            let state = timing.observeDispatch(data, now: now)
            timingReceipt["lastDispatchCheckUTC"] = formatter.string(from: now)
            switch state {
            case .pending(let deadline):
                timingReceipt["dispatchState"] = "PENDING"
                timingReceipt["dispatchDeadlineUTC"] = formatter.string(from: deadline)
            case .exited(let exit):
                timingReceipt["dispatchState"] = "EXITED"
                timingReceipt["dispatchExitObservedUTC"] = formatter.string(from: exit)
            case .rejected(let reason):
                timingReceipt["dispatchState"] = "REJECTED"
                timingReceipt["dispatchFailure"] = reason
            }
            return state
        }
        func checkAdd() -> QADifferentialImportTiming.AddDecision {
            let dispatch = observeDispatch()
            let now = Date()
            let decision = timing.addDecision(after: dispatch, now: now)
            timingReceipt["lastAddCheckUTC"] = formatter.string(from: now)
            if let deadline = decision.deadline {
                timingReceipt["currentSearchDeadlineUTC"] = formatter.string(from: deadline)
            }
            return decision
        }
        func receipt(_ fields: [String: Any]) -> [String: Any] {
            var fields = fields
            fields["importTiming"] = timingReceipt
            return fields
        }
        func stop(_ outcome: String) -> [String: Any] {
            capture(command.id + "-import")
            XCTFail(outcome + " for " + name)
            return receipt(["outcome": outcome, "name": name])
        }

        // AX calls can outlive their nominal wait. Read the actual process
        // receipt and the clock again after each call, before using its result.
        while true {
            if let failure = checkAdd().failure { return stop(failure) }
            let found = button.exists
            let decision = checkAdd()
            if let failure = decision.failure { return stop(failure) }
            if found { break }
            let remaining = decision.deadline!.timeIntervalSinceNow
            Thread.sleep(forTimeInterval: max(0, min(0.2, remaining)))
        }
        timingReceipt["addObservedUTC"] = formatter.string(from: Date())
        capture(command.id + "-import")
        if let failure = checkAdd().failure { return stop(failure) }
        let snapshot = try shortcuts.snapshot()
        if let failure = checkAdd().failure { return stop(failure) }
        let identity = hasName(snapshot, name: name)
        XCTAssertTrue(identity, "Signed import sheet must identify " + name)
        guard identity else {
            return receipt(["outcome": "IMPORT_IDENTITY_MISMATCH", "name": name])
        }
        button.tap()
        if let failure = checkAdd().failure { return stop(failure) }
        timingReceipt["addTappedUTC"] = formatter.string(from: Date())
        let libraryItem = shortcuts.cells.matching(
            NSPredicate(format: "label == %@ OR label BEGINSWITH %@", name, name + ", ")).firstMatch
        let deadline = Date().addingTimeInterval(30)
        var replaceTapped = false
        while Date() < deadline {
            if let failure = observeDispatch().failure { return stop(failure) }
            let exists = libraryItem.exists
            if let failure = observeDispatch().failure { return stop(failure) }
            if Date() >= deadline { break }
            if exists {
                let hittable = libraryItem.isHittable
                let dispatch = observeDispatch()
                if let failure = dispatch.failure { return stop(failure) }
                if Date() >= deadline { break }
                if hittable && dispatch.succeeded {
                    capture(command.id + "-library")
                    if let failure = observeDispatch().failure { return stop(failure) }
                    if Date() >= deadline { break }
                    return receipt(["outcome": "IMPORTED", "name": name, "identityVerified": identity,
                                    "libraryIdentityVerified": true, "replaceTapped": replaceTapped])
                }
            }
            let replacement = shortcuts.buttons["Replace"].firstMatch
            let replacementExists = replacement.exists
            if let failure = observeDispatch().failure { return stop(failure) }
            if Date() >= deadline { break }
            if replacementExists {
                let hittable = replacement.isHittable
                if let failure = observeDispatch().failure { return stop(failure) }
                if Date() >= deadline { break }
                if hittable {
                    capture(command.id + "-replace")
                    if let failure = observeDispatch().failure { return stop(failure) }
                    if Date() >= deadline { break }
                    replacement.tap()
                    replaceTapped = true
                    if let failure = observeDispatch().failure { return stop(failure) }
                }
            }
            Thread.sleep(forTimeInterval: 0.5)
        }
        capture(command.id + "-library-timeout")
        let debugState = shortcuts.debugDescription
        let debugPath = runDirectory.appendingPathComponent(command.id + "-library-timeout.txt")
        try? debugState.write(to: debugPath, atomically: true, encoding: String.Encoding.utf8)
        XCTFail("No accessible native library item after import for " + name)
        return receipt(["outcome": "LIBRARY_IDENTITY_NOT_OBSERVED", "name": name,
                        "identityVerified": identity, "replaceTapped": replaceTapped])
    }

    private func consentForRuntime(_ command: DifferentialCommand) throws -> [String: Any] {
        let completion = runDirectory.appendingPathComponent("completion-" + command.id + ".json")
        let deadline = Date().addingTimeInterval(40)
        var taps: [String] = []
        var lastInspection = Date.distantPast
        while Date() < deadline {
            if FileManager.default.fileExists(atPath: completion.path) {
                return ["outcome": "CALLBACK_OBSERVED_BY_HOST", "consentTaps": taps]
            }
            if Date().timeIntervalSince(lastInspection) > 2 {
                lastInspection = Date()
                for app in [shortcuts, springboard] where app.state == .runningForeground {
                    let snapshot = try app.snapshot()
                    for label in ["Always Allow", "Allow Once", "Allow", "Open"] {
                        if let frame = consentFrame(snapshot, label: label) {
                            capture(command.id + "-consent-" + String(taps.count))
                            app.coordinate(withNormalizedOffset: CGVector(dx: 0, dy: 0))
                                .withOffset(CGVector(dx: frame.midX - app.frame.minX,
                                                     dy: frame.midY - app.frame.minY)).tap()
                            taps.append(label)
                            Thread.sleep(forTimeInterval: 0.5)
                            break
                        }
                    }
                }
            }
            Thread.sleep(forTimeInterval: 0.2)
        }
        capture(command.id + "-runtime-timeout")
        return ["outcome": "CALLBACK_NOT_OBSERVED", "consentTaps": taps]
    }

    func testDifferentialImportsAndRuntimeConsent() throws {
        try write(["ready": true], to: "ready.json")
        let deadline = Date().addingTimeInterval(3200)
        var lastID = ""
        while Date() < deadline {
            let path = runDirectory.appendingPathComponent("command.json")
            guard let data = try? Data(contentsOf: path),
                  let command = try? JSONDecoder().decode(DifferentialCommand.self, from: data),
                  command.id != lastID else {
                Thread.sleep(forTimeInterval: 0.2)
                continue
            }
            XCTAssertNotNil(command.id.range(of: "^[A-Za-z0-9_-]+$", options: .regularExpression))
            lastID = command.id
            var started: [String: Any] = [
                "id": command.id, "operation": command.operation, "outcome": "STARTED",
                "timestamp": ISO8601DateFormatter().string(from: Date())
            ]
            if let name = command.shortcutName {
                started["shortcutName"] = name
            }
            try write(started, to: "phase-started-" + command.id + ".json")
            var receipt: [String: Any]
            switch command.operation {
            case "import": receipt = try importShortcut(command)
            case "runtime": receipt = try consentForRuntime(command)
            case "finish":
                try write(["id": command.id, "outcome": "FINISHED"], to: "ack-" + command.id + ".json")
                return
            default:
                XCTFail("Unknown differential command")
                receipt = ["outcome": "UNKNOWN_COMMAND"]
            }
            receipt["id"] = command.id
            receipt["operation"] = command.operation
            receipt["timestamp"] = ISO8601DateFormatter().string(from: Date())
            try write(receipt, to: "ack-" + command.id + ".json")
        }
        XCTFail("Differential batch exceeded its bounded deadline")
    }
}
