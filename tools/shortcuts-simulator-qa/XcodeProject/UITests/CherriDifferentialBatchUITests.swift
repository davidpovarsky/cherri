import XCTest

private typealias DifferentialCommand = QAImportCommand

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



    // Separate diagnostic: one observed Editor navigation, no import or Run.
    private func navigateExistingEditor(_ command: DifferentialCommand) throws -> [String: Any] {
        let name = try XCTUnwrap(command.shortcutName)
        guard command.operation == "navigate-existing-editor",
              command.id.range(of: "^editor-nav-[0-9a-f]{32}$", options: .regularExpression) != nil,
              !name.isEmpty, name.utf8.count <= 4096 else {
            throw NSError(domain: "CherriQAEditorNavigation", code: 1,
                          userInfo: [NSLocalizedDescriptionKey: "EDITOR_COMMAND_CONTEXT"])
        }
        let folder = runDirectory.appendingPathComponent(command.id + "-editor-navigation", isDirectory: true)
        guard !FileManager.default.fileExists(atPath: folder.path) else {
            throw NSError(domain: "CherriQAEditorNavigation", code: 1,
                          userInfo: [NSLocalizedDescriptionKey: "EDITOR_OUTPUT_EXISTS"])
        }
        try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: false,
                                               attributes: [.posixPermissions: 0o700])
        let started = Date()
        let deadline = started.addingTimeInterval(25)
        let formatter = ISO8601DateFormatter()
        formatter.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
        var invocationCount = 0
        var selectedButton: XCUIElement?
        func require(_ condition: Bool, _ code: String) throws {
            guard condition else {
                throw NSError(domain: "CherriQAEditorNavigation", code: 2,
                              userInfo: [NSLocalizedDescriptionKey: code])
            }
        }
        func budget() throws { try require(Date() < deadline, "EDITOR_NAVIGATION_DEADLINE") }
        func save(_ value: Any, _ filename: String) throws {
            let bytes = try JSONSerialization.data(withJSONObject: value, options: [.prettyPrinted, .sortedKeys])
            try require(bytes.count <= 2 * 1024 * 1024, "EDITOR_OBSERVATION_BYTES")
            let url = folder.appendingPathComponent(filename)
            try bytes.write(to: url, options: .withoutOverwriting)
            try FileManager.default.setAttributes([.posixPermissions: 0o600], ofItemAtPath: url.path)
        }
        func model(_ snapshot: XCUIElementSnapshot) throws -> (QAImportSheetSelection.Node, [[String: Any]]) {
            var rows: [[String: Any]] = []
            func visit(_ node: XCUIElementSnapshot, _ path: [Int]) throws -> QAImportSheetSelection.Node {
                try require(rows.count < 8192 && path.count <= 64, "EDITOR_TREE_BOUNDS")
                try require(node.label.utf8.count <= 4096 && node.identifier.utf8.count <= 4096,
                            "EDITOR_LABEL_BOUNDS")
                let f = node.frame
                try require([f.minX, f.minY, f.width, f.height].allSatisfy { $0.isFinite }, "EDITOR_FRAME")
                rows.append(["path": path, "type": Int(node.elementType.rawValue),
                             "label": node.label, "identifier": node.identifier,
                             "enabled": node.isEnabled, "selected": node.isSelected,
                             "frame": ["x": Double(f.minX), "y": Double(f.minY),
                                       "width": Double(f.width), "height": Double(f.height)],
                             "childCount": node.children.count])
                let kind: QAImportSheetSelection.Kind
                switch node.elementType {
                case .application: kind = .application
                case .window: kind = .window
                case .other: kind = .other
                case .sheet: kind = .sheet
                case .scrollView: kind = .scrollView
                case .navigationBar: kind = .navigationBar
                case .cell: kind = .cell
                case .collectionView: kind = .collectionView
                case .button: kind = .button
                case .staticText: kind = .staticText
                default: kind = .unsupported
                }
                let children = try node.children.enumerated().map { try visit($0.element, path + [$0.offset]) }
                return QAImportSheetSelection.Node(kind: kind, label: node.label, identifier: node.identifier,
                    enabled: node.isEnabled, selected: node.isSelected, frame: f, children: children)
            }
            let root = try visit(snapshot, [])
            return (root, rows)
        }
        func screenshot(_ filename: String) throws {
            try budget()
            let png = XCUIScreen.main.screenshot().pngRepresentation
            try budget()
            try require(png.count <= 8 * 1024 * 1024, "EDITOR_IMAGE_BOUNDS")
            let url = folder.appendingPathComponent(filename)
            try png.write(to: url, options: .withoutOverwriting)
            try FileManager.default.setAttributes([.posixPermissions: 0o600], ofItemAtPath: url.path)
            try budget()
        }
        var ordinal = 0
        func observe() throws -> QAExistingEditorNavigation.Decision {
            selectedButton = nil
            try budget()
            let foreground = shortcuts.state == .runningForeground
            try budget()
            let bars = shortcuts.navigationBars.matching(identifier: name).allElementsBoundByIndex
            try budget()
            let buttons = bars.count == 1 ? bars[0].buttons.matching(
                NSPredicate(format: "label == %@", "Editor")).allElementsBoundByIndex : []
            try budget()
            try save(["navigationBarQueryCount": bars.count, "buttonQueryCount": buttons.count,
                "observedUTC": formatter.string(from: Date())], String(format: "query-%02d.original.json", ordinal))
            try budget()
            try require(buttons.count <= 1, "EDITOR_QUERY_NOT_UNIQUE")
            var candidates: [QAImportSheetSelection.Candidate] = []
            for (index, button) in buttons.enumerated() {
                let exists = button.exists
                try budget()
                let hittable = exists && button.isHittable
                try budget()
                var node: QAImportSheetSelection.Node?
                var row: [String: Any] = ["queryIndex": index, "exists": exists,
                    "hittable": hittable, "observedUTC": formatter.string(from: Date())]
                if exists {
                    let snapshot = try button.snapshot()
                    try budget()
                    let observed = try model(snapshot)
                    node = observed.0
                    row["snapshot"] = observed.1
                }
                try save(row, String(format: "candidate-%02d.original.json", ordinal))
                try budget()
                candidates.append(QAImportSheetSelection.Candidate(queryIndex: index, exists: exists,
                    hittable: hittable, snapshot: node))
            }
            let snapshot = try shortcuts.snapshot()
            try budget()
            let current = try model(snapshot)
            try save(current.1, String(format: "snapshot-%02d.original.json", ordinal))
            try budget()
            let decision = bars.count == 1 ? QAExistingEditorNavigation.select(current: current.0,
                candidates: candidates, context: .init(expectedName: name, foreground: foreground,
                alreadyInvoked: invocationCount > 0, now: Date(), deadline: deadline)) :
                QAExistingEditorNavigation.rejected("NO_UNIQUE_REQUESTED_EDITOR_QUERY")
            var evidence: [String: Any] = ["reason": decision.reason,
                "navigationBarQueryCount": bars.count, "buttonQueryCount": buttons.count,
                "currentNavigationBarCount": decision.currentNavigationBarCount,
                "matchedCandidateCount": decision.matchedCandidateCount,
                "observedUTC": formatter.string(from: Date()), "canNavigate": decision.canNavigate]
            if let binding = decision.binding {
                selectedButton = buttons[binding.queryIndex]
                evidence["queryIndex"] = binding.queryIndex
                evidence["navigationBarPath"] = binding.navigationBarPath
                evidence["buttonPath"] = binding.buttonPath
            }
            try save(evidence, String(format: "selection-%02d.original.json", ordinal))
            ordinal += 1
            try budget()
            return decision
        }
        try save(["schema": "cherri-existing-editor-navigation/1", "expectedName": name,
            "startedUTC": formatter.string(from: started), "deadlineUTC": formatter.string(from: deadline),
            "nativeImportInvoked": false, "nativeRuntimeInvoked": false,
            "navigationInvocationCount": 0, "formalAcceptanceGranted": false], "intent.original.json")
        do {
            try require(!name.isEmpty && name.utf8.count <= 4096, "EDITOR_NAME_BOUND")
            let state = shortcuts.state
            try budget()
            try require(state == .runningForeground || state == .runningBackground, "EDITOR_SHORTCUTS_NOT_RUNNING")
            if state != .runningForeground {
                shortcuts.activate()
                try budget()
                let foreground = shortcuts.wait(for: .runningForeground, timeout: min(8, deadline.timeIntervalSinceNow))
                try budget()
                try require(foreground, "EDITOR_SHORTCUTS_NOT_FOREGROUND")
            }
            let selected = try QAExistingEditorNavigation.inspectForNavigation(observe: observe,
                capture: { try screenshot("screen.before.original.png") })
            try budget()
            try require(selected.canNavigate, selected.reason)
            let button = try XCTUnwrap(selectedButton)
            try require(invocationCount == 0, "EDITOR_NAVIGATION_ALREADY_INVOKED")
            // Claim once before the native call. A returned tap is not itself
            // proof that the editor displayed any action or executed a workflow.
            invocationCount = 1
            try save(["expectedName": name, "navigationInvocationCount": invocationCount,
                "invokedUTC": formatter.string(from: Date()), "buttonLabel": "Editor",
                "nativeImportInvoked": false, "nativeRuntimeInvoked": false,
                "formalAcceptanceGranted": false], "navigation-intent.original.json")
            try budget()
            button.tap()
            try budget()
            try save(["returnedUTC": formatter.string(from: Date()), "navigationInvocationCount": invocationCount],
                     "navigation-return.original.json")
            let after = try shortcuts.snapshot()
            try budget()
            let current = try model(after)
            try save(current.1, "snapshot.after.original.json")
            try budget()
            try screenshot("screen.after.original.png")
            try budget()
            let result: [String: Any] = ["schema": "cherri-existing-editor-navigation/1",
                "outcome": "EDITOR_NAVIGATION_AND_AX_RETAINED", "expectedName": name,
                "completedUTC": formatter.string(from: Date()), "selectionObservations": ordinal,
                "navigationInvocationCount": invocationCount, "nativeImportInvoked": false,
                "nativeRuntimeInvoked": false, "formalAcceptanceGranted": false,
                "actionListRequiresOriginalIntake": true]
            try save(result, "result.original.json")
            return result
        } catch {
            let failure: [String: Any] = ["schema": "cherri-existing-editor-navigation/1",
                "outcome": "EDITOR_NAVIGATION_FAILED", "expectedName": name,
                "completedUTC": formatter.string(from: Date()), "reason": (error as NSError).localizedDescription,
                "selectionObservations": ordinal, "navigationInvocationCount": invocationCount,
                "nativeImportInvoked": false, "nativeRuntimeInvoked": false,
                "formalAcceptanceGranted": false]
            try? save(failure, "failure.original.json")
            throw error
        }
    }

    private func importShortcut(_ command: DifferentialCommand) throws -> [String: Any] {
        try QAImportedShortcutObserver(runDirectory: runDirectory, shortcuts: shortcuts,
            capture: { self.capture($0) }).importShortcut(command)
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
            case "navigate-existing-editor": receipt = try navigateExistingEditor(command)
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
