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

    // This explicit diagnostic command records the current AX hierarchy only.
    // It never opens a shortcut, taps consent or supplies runtime evidence.
    private func observeImportSheet(_ command: DifferentialCommand) throws -> [String: Any] {
        let name = try XCTUnwrap(command.shortcutName)
        let folder = runDirectory.appendingPathComponent(command.id + "-sheet-observation", isDirectory: true)
        guard !FileManager.default.fileExists(atPath: folder.path) else {
            throw NSError(domain: "CherriQAImportObservation", code: 1,
                          userInfo: [NSLocalizedDescriptionKey: "OBSERVATION_OUTPUT_EXISTS"])
        }
        try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: false,
                                               attributes: [.posixPermissions: 0o700])
        let started = Date()
        let deadline = started.addingTimeInterval(25)
        let formatter = ISO8601DateFormatter()
        formatter.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
        func require(_ condition: Bool, _ code: String) throws {
            guard condition else {
                throw NSError(domain: "CherriQAImportObservation", code: 2,
                              userInfo: [NSLocalizedDescriptionKey: code])
            }
        }
        func budget() throws { try require(Date() < deadline, "OBSERVATION_DEADLINE") }
        func save(_ value: Any, _ filename: String) throws {
            let bytes = try JSONSerialization.data(withJSONObject: value, options: [.prettyPrinted, .sortedKeys])
            try require(bytes.count <= 2 * 1024 * 1024, "OBSERVATION_BYTES")
            let url = folder.appendingPathComponent(filename)
            try bytes.write(to: url, options: .withoutOverwriting)
            try FileManager.default.setAttributes([.posixPermissions: 0o600], ofItemAtPath: url.path)
        }
        func flatten(_ snapshot: XCUIElementSnapshot) throws -> [[String: Any]] {
            var rows: [[String: Any]] = []
            func visit(_ node: XCUIElementSnapshot, _ path: [Int]) throws {
                try require(rows.count < 8192 && path.count <= 64, "OBSERVATION_TREE_BOUNDS")
                try require(node.label.utf8.count <= 4096 && node.identifier.utf8.count <= 4096,
                            "OBSERVATION_LABEL_BOUNDS")
                let f = node.frame
                try require([f.minX, f.minY, f.width, f.height].allSatisfy { $0.isFinite },
                            "OBSERVATION_FRAME")
                rows.append(["path": path, "type": Int(node.elementType.rawValue),
                             "label": node.label, "identifier": node.identifier,
                             "enabled": node.isEnabled, "selected": node.isSelected,
                             "frame": ["x": Double(f.minX), "y": Double(f.minY),
                                       "width": Double(f.width), "height": Double(f.height)],
                             "childCount": node.children.count])
                for (index, child) in node.children.enumerated() {
                    try visit(child, path + [index])
                }
            }
            try visit(snapshot, [])
            return rows
        }
        try save(["schema": "cherri-import-sheet-observation/1", "expectedName": name,
                  "startedUTC": formatter.string(from: started), "nativeImportInvoked": false,
                  "nativeRuntimeInvoked": false, "tapCount": 0], "intent.original.json")
        let state = shortcuts.state
        try budget()
        try require(state == .runningForeground || state == .runningBackground,
                    "OBSERVATION_SHORTCUTS_NOT_RUNNING")
        if state != .runningForeground {
            shortcuts.activate()
            try budget()
            let foreground = shortcuts.wait(for: .runningForeground, timeout: min(8, deadline.timeIntervalSinceNow))
            try budget()
            try require(foreground, "OBSERVATION_SHORTCUTS_NOT_FOREGROUND")
        }
        let before = try shortcuts.snapshot()
        try save(try flatten(before), "snapshot.before.original.json")
        try budget()
        let buttons = shortcuts.buttons.matching(NSPredicate(
            format: "label == 'Add Shortcut' OR label == '+ Add Shortcut' OR label == 'Replace'")).allElementsBoundByIndex
        try budget()
        try require(buttons.count <= 32, "OBSERVATION_CANDIDATE_BOUNDS")
        var candidates: [[String: Any]] = []
        for (index, button) in buttons.enumerated() {
            let exists = button.exists
            try budget()
            let hittable = exists && button.isHittable
            try budget()
            var row: [String: Any] = ["queryIndex": index, "exists": exists,
                                     "hittable": hittable, "observedUTC": formatter.string(from: Date())]
            if exists {
                let candidate = try button.snapshot()
                try budget()
                row["snapshot"] = try flatten(candidate)
            }
            candidates.append(row)
            try save(row, String(format: "candidate-%02d.original.json", index))
        }
        let after = try shortcuts.snapshot()
        try save(try flatten(after), "snapshot.after.original.json")
        try budget()
        let png = XCUIScreen.main.screenshot().pngRepresentation
        try require(png.count <= 8 * 1024 * 1024, "OBSERVATION_IMAGE_BOUNDS")
        try png.write(to: folder.appendingPathComponent("screen.original.png"), options: .withoutOverwriting)
        try FileManager.default.setAttributes([.posixPermissions: 0o600],
            ofItemAtPath: folder.appendingPathComponent("screen.original.png").path)
        try budget()
        let result: [String: Any] = ["schema": "cherri-import-sheet-observation/1",
            "outcome": "OBSERVED_AX_ONLY", "expectedName": name, "candidateCount": candidates.count,
            "completedUTC": formatter.string(from: Date()), "nativeImportInvoked": false,
            "nativeRuntimeInvoked": false, "tapCount": 0, "formalAcceptanceGranted": false]
        try save(result, "result.original.json")
        return result
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

        func checkSheet(found: Bool, hittable: Bool,
                        identity: Bool) -> QADifferentialImportTiming.SheetDecision {
            let dispatch = observeDispatch()
            let now = Date()
            let decision = timing.sheetDecision(after: dispatch, now: now,
                buttonExists: found, buttonHittable: hittable, identityMatches: identity)
            timingReceipt["sheetContract"] = "requested-identity-and-add-v1"
            timingReceipt["lastSheetCheckUTC"] = formatter.string(from: now)
            timingReceipt["sheetButtonExists"] = found
            timingReceipt["sheetButtonHittable"] = hittable
            timingReceipt["sheetIdentityMatched"] = identity
            timingReceipt["sheetCanTap"] = decision.canTap
            if let deadline = decision.deadline {
                timingReceipt["currentSearchDeadlineUTC"] = formatter.string(from: deadline)
            }
            return decision
        }
        // AX calls can outlive their nominal wait. Read the actual process
        // receipt and the clock again after each call, before using its result.
        // Every invocation reads fresh values, including after capture().
        var identity = false
        func observeSheet() throws -> QADifferentialImportTiming.SheetDecision {
            if let failure = checkAdd().failure { return .rejected(failure) }
            let found = button.exists
            if let failure = checkAdd().failure { return .rejected(failure) }
            var hittable = false
            identity = false
            if found {
                hittable = button.isHittable
                if let failure = checkAdd().failure { return .rejected(failure) }
                let snapshot = try shortcuts.snapshot()
                if let failure = checkAdd().failure { return .rejected(failure) }
                identity = hasName(snapshot, name: name)
            }
            return checkSheet(found: found, hittable: hittable, identity: identity)
        }
        while true {
            let decision = try QADifferentialImportTiming.inspectSheetForConsent(
                observe: observeSheet,
                capture: {
                    timingReceipt["addObservedUTC"] = formatter.string(from: Date())
                    self.capture(command.id + "-import")
                })
            if let failure = decision.failure { return stop(failure) }
            if decision.canTap {
                timingReceipt["sheetMatchedAfterCaptureUTC"] = formatter.string(from: Date())
                break
            }
            let remaining = decision.deadline!.timeIntervalSinceNow
            Thread.sleep(forTimeInterval: max(0, min(0.2, remaining)))
        }
        XCTAssertTrue(identity, "Signed import sheet must identify " + name)
        guard identity else {
            return receipt(["outcome": "IMPORT_IDENTITY_MISMATCH", "name": name])
        }
        button.tap()
        if let failure = checkAdd().failure { return stop(failure) }
        let addTappedAt = Date()
        timingReceipt["addTappedUTC"] = formatter.string(from: addTappedAt)
        timingReceipt["libraryContract"] = "dispatch-exit-anchored-library-v1"
        let libraryItem = shortcuts.cells.matching(
            NSPredicate(format: "label == %@ OR label BEGINSWITH %@", name, name + ", ")).firstMatch
        func checkLibrary() -> QADifferentialImportTiming.LibraryDecision {
            let dispatch = observeDispatch()
            let now = Date()
            let decision = timing.libraryDecision(after: dispatch, addTappedAt: addTappedAt, now: now)
            timingReceipt["lastLibraryCheckUTC"] = formatter.string(from: now)
            if let deadline = decision.deadline {
                timingReceipt["libraryDeadlineUTC"] = formatter.string(from: deadline)
            }
            timingReceipt["libraryCanConfirm"] = decision.canConfirm
            return decision
        }
        var replaceTapped = false
        while true {
            var decision = checkLibrary()
            if let failure = decision.failure { return stop(failure) }
            if Date() >= decision.deadline! { break }
            let exists = libraryItem.exists
            decision = checkLibrary()
            if let failure = decision.failure { return stop(failure) }
            if Date() >= decision.deadline! { break }
            if exists {
                let hittable = libraryItem.isHittable
                decision = checkLibrary()
                if let failure = decision.failure { return stop(failure) }
                if Date() >= decision.deadline! { break }
                if hittable && decision.canConfirm {
                    capture(command.id + "-library")
                    decision = checkLibrary()
                    if let failure = decision.failure { return stop(failure) }
                    if Date() >= decision.deadline! { break }
                    if decision.canConfirm {
                        return receipt(["outcome": "IMPORTED", "name": name, "identityVerified": identity,
                                        "libraryIdentityVerified": true, "replaceTapped": replaceTapped])
                    }
                }
            }
            let replacement = shortcuts.buttons["Replace"].firstMatch
            let replacementExists = replacement.exists
            decision = checkLibrary()
            if let failure = decision.failure { return stop(failure) }
            if Date() >= decision.deadline! { break }
            if replacementExists {
                let hittable = replacement.isHittable
                decision = checkLibrary()
                if let failure = decision.failure { return stop(failure) }
                if Date() >= decision.deadline! { break }
                if hittable {
                    capture(command.id + "-replace")
                    decision = checkLibrary()
                    if let failure = decision.failure { return stop(failure) }
                    if Date() >= decision.deadline! { break }
                    replacement.tap()
                    replaceTapped = true
                    decision = checkLibrary()
                    if let failure = decision.failure { return stop(failure) }
                    if Date() >= decision.deadline! { break }
                }
            }
            Thread.sleep(forTimeInterval: max(0, min(0.5, decision.deadline!.timeIntervalSinceNow)))
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
            case "observe-import": receipt = try observeImportSheet(command)
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
