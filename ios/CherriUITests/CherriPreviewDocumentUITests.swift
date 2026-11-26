import XCTest

/// Actual native WebKit field editing with the existing bounded document startup.
final class CherriPreviewDocumentUITests: XCTestCase {
    private var app: XCUIApplication!
    private enum Stop: Error { case failed }

    override func setUpWithError() throws {
        continueAfterFailure = false
        app = XCUIApplication()
        app.launchArguments = [
            "--uitesting",
            "-AppleLanguages", "(en)",
            "-AppleKeyboards", "('en_US@hw=US;sw=QWERTY')",
            // The existing AppStorage option is disabled only in the launch
            // argument domain. Every compilation below requires a real Build tap.
            "-Cherri.livePreview", "NO",
        ]
        app.launch()
    }

    override func tearDownWithError() throws {
        app = nil
    }

    private struct Controls {
        let state: CherriEditorStartupObservation
        let creation: XCUIElement?

        func recheckingForeground(_ value: Bool) -> Self {
            Self(state: state.recheckingForeground(value), creation: creation)
        }
    }

    private func startupControls(_ scope: CherriEditorStartupControlScope) -> Controls {
        let window = app.windows.firstMatch.exists
        let foreground = app.state == .runningForeground
        let matched = scope.resolve(
            creation: {
                app.buttons.matching(NSPredicate(
                    format: "identifier == %@ OR label == %@", "Create Document", "Create Document"))
                    .allElementsBoundByAccessibilityElement
            },
            editor: {
                app.textViews.matching(NSPredicate(format: "identifier == %@", "cherriCodeEditor"))
                    .allElementsBoundByAccessibilityElement
            }
        )
        let state: CherriEditorStartupObservation
        if scope == .editorOnly {
            state = .matchedEditor(windowExists: window, appRunningForeground: foreground,
                                   editorCount: matched.editors.count,
                                   editorHittable: { matched.editors[0].isHittable })
        } else {
            state = .matchedControls(
                windowExists: window, appRunningForeground: foreground,
                createDocumentCount: matched.creation.count, editorCount: matched.editors.count,
                createDocumentHittable: { matched.creation[0].isHittable },
                editorHittable: { matched.editors[0].isHittable }
            )
        }
        let current = state.recheckingForeground(app.state == .runningForeground)
        return Controls(state: current,
                        creation: current.createDocumentHittable && !current.editorExists
                            ? matched.creation.first : nil)
    }

    /// Uses the unchanged shared 28-second decision budget, 10-second window
    /// allowance and 6-second document-control allowance. A late/failed predicate
    /// never authorizes a fallback creation tap; at most one document is created.
    private func openedEditor() throws -> XCUIElement {
        var deadline = CherriEditorStartupDeadline()
        let window = app.windows.firstMatch
        let windowReady = window.waitForExistence(timeout: deadline.remaining(maximum: 10))
        let first = startupControls(.documentAndEditor)
        guard windowReady, first.state.windowExists, first.state.appRunningForeground,
              deadline.remaining() > 0 else {
            XCTFail("Preview requires a foreground app window within the existing startup budget")
            throw Stop.failed
        }
        if !first.state.editorExists {
            var controls = first
            if deadline.needsDocumentWait(first.state) {
                let observations = CherriEditorStartupWaitObservation<Controls>(phase: .document)
                let expectation = XCTNSPredicateExpectation(predicate: NSPredicate { _, _ in
                    let observed = self.startupControls(.documentAndEditor)
                    return observations.record(observed, state: observed.state)
                }, object: app.buttons["Create Document"])
                let completed = XCTWaiter.wait(
                    for: [expectation], timeout: deadline.remaining(maximum: 6)
                ) == .completed
                guard let observed = observations.takeCompleted(waitCompleted: completed) else {
                    XCTFail("Preview document readiness did not complete within the original allowance")
                    throw Stop.failed
                }
                controls = observed.recheckingForeground(app.state == .runningForeground)
            }
            if let create = controls.creation, deadline.claimDocumentCreation(controls.state) {
                create.tap()
            }
        }
        let editor = app.textViews["cherriCodeEditor"]
        let scope: CherriEditorStartupControlScope = deadline.documentCreationIssued ? .editorOnly : .documentAndEditor
        let observations = CherriEditorStartupWaitObservation<Controls>(phase: .editor)
        let expectation = XCTNSPredicateExpectation(predicate: NSPredicate { _, _ in
            let observed = self.startupControls(scope)
            return observations.record(observed, state: observed.state)
        }, object: editor)
        let completed = XCTWaiter.wait(for: [expectation], timeout: deadline.remaining()) == .completed
        guard let observed = observations.takeCompleted(waitCompleted: completed),
              deadline.accepts(observed.state.recheckingForeground(app.state == .runningForeground),
                               waitCompleted: completed) else {
            XCTFail("Preview editor must be interactive within the unchanged startup budget")
            throw Stop.failed
        }
        return editor
    }

    private func wait(_ seconds: TimeInterval = 5, _ predicate: @escaping () -> Bool) -> Bool {
        let expectation = XCTNSPredicateExpectation(predicate: NSPredicate { _, _ in predicate() }, object: nil)
        return XCTWaiter.wait(for: [expectation], timeout: seconds) == .completed
    }

    private func require(_ condition: Bool, _ message: String) throws {
        guard condition else {
            XCTFail(message)
            throw Stop.failed
        }
    }

    private func expectSource(_ editor: XCUIElement, _ source: String) throws {
        let matched = wait {
            guard editor.exists, let value = editor.value as? String else { return false }
            return value.utf16.elementsEqual(source.utf16)
        }
        if !matched {
            // Retain bounded mismatch geometry and hashes, never arbitrary source.
            let observed = (editor.value as? String) ?? ""
            let expectedUnits = Array(source.utf16)
            let observedUnits = Array(observed.utf16)
            let commonCount = min(expectedUnits.count, observedUnits.count)
            let mismatch = (0..<commonCount).first { expectedUnits[$0] != observedUnits[$0] }
                ?? commonCount
            let receipt: [String: Any] = [
                "expectedSha256": CherriJoinedContract.digest(source),
                "observedSha256": CherriJoinedContract.digest(observed),
                "expectedUTF16Length": expectedUnits.count,
                "observedUTF16Length": observedUnits.count,
                "firstMismatchUTF16Offset": mismatch,
                "hasGeneratedParenthesisSuffix": observed.utf16.elementsEqual((source + ")").utf16),
            ]
            let attachment = XCTAttachment(data: try JSONSerialization.data(withJSONObject: receipt),
                                           uniformTypeIdentifier: "public.json")
            attachment.name = "Preview exact source mismatch"
            attachment.lifetime = .keepAlways
            add(attachment)
        }
        try require(matched, "Preview editor must contain the exact current synthetic UTF-16 source")
    }

    /// No source or title is logged. Only an observed empty editor permits typing
    /// the fixed synthetic fixture; a failed reset stops this test.
    private func replaceSource(_ editor: XCUIElement, _ source: String,
                               generatingClosingParenthesis: Bool = false) throws {
        let clear = app.buttons["clearEditorButton"]
        try require(clear.waitForExistence(timeout: 2) && clear.isHittable,
                    "Preview requires the existing Clear action")
        clear.tap()
        try expectSource(editor, "")
        try require(wait {
            !self.app.staticTexts["diagnosticStatusText"].exists
                && !self.app.staticTexts["diagnosticItem_0"].exists
                && !self.app.staticTexts["compiledStatusText"].exists
                && !self.app.buttons["shareSignedShortcutButton"].exists
        }, "Clearing source must invalidate old diagnostics, compilation and signed export")
        if !source.isEmpty {
            editor.coordinate(withNormalizedOffset: CGVector(dx: 0.5, dy: 0.5)).tap()
            try require(app.keyboards.firstMatch.waitForExistence(timeout: 2),
                        "Preview synthetic typing requires the real keyboard")
            if generatingClosingParenthesis {
                try require(source.hasSuffix(")"),
                            "The authored completion fixture must include its generated parenthesis")
                // Real keyboard input intentionally exercises the existing token
                // completion contract. The caret stays before the generated ')'.
                editor.typeText(String(source.dropLast()))
            } else {
                editor.typeText(source)
            }
            try expectSource(editor, source)
        }
    }

    private func build(_ editor: XCUIElement, source: String, succeeds: Bool) throws {
        try expectSource(editor, source)
        let button = app.buttons["buildButton"]
        try require(wait { button.exists && button.isHittable && button.isEnabled },
                    "Preview Build must be enabled and hittable")
        button.tap()
        let compiled = app.staticTexts["compiledStatusText"]
        let diagnostic = app.staticTexts["diagnosticStatusText"]
        try require(wait(8) {
            succeeds ? (compiled.exists && !diagnostic.exists)
                     : (diagnostic.exists && !compiled.exists)
        }, succeeds ? "Preview current source must compile through Build"
                    : "Preview invalid current source must render a diagnostic")
        try expectSource(editor, source)
    }

    private func observedSource(_ editor: XCUIElement) throws -> String {
        try XCTUnwrap(editor.value as? String, "The real editor value is required")
    }


    @discardableResult
    private func pane(_ name: String) throws -> [String: Any] {
        let control = app.segmentedControls.buttons[name]
        let exists = control.exists
        if exists {
            let hittable = control.isHittable
            try require(hittable, "The real workspace pane must be selectable")
            control.tap()
            return ["target": name, "exists": exists, "hittable": hittable,
                    "tapIssued": true, "selectionObserved": false]
        }
        return ["target": name, "exists": exists, "hittable": NSNull(),
                "tapIssued": false, "selectionObserved": false]
    }

    private func fields(_ key: String, value: String,
                        trace: CherriUIReadinessTrace? = nil) -> [XCUIElement] {
        let web = app.webViews.firstMatch
        let webExists = web.exists
        guard webExists else {
            trace?.fields(webExists: webExists, candidateCount: 0, matchingCount: 0)
            return []
        }
        let predicate = NSPredicate(format: "label == %@ OR identifier == %@", key, key)
        let candidates = web.textViews.matching(predicate).allElementsBoundByAccessibilityElement
            + web.textFields.matching(predicate).allElementsBoundByAccessibilityElement
        let matches = candidates.filter { ($0.value as? String)?.utf16.elementsEqual(value.utf16) == true }
        trace?.fields(webExists: webExists, candidateCount: candidates.count, matchingCount: matches.count)
        return matches
    }

    private func field(_ key: String, value: String,
                       boundary: CherriUIReadinessTrace.Boundary? = nil,
                       pane: [String: Any]? = nil) throws -> XCUIElement {
        let trace = boundary.map { CherriUIReadinessTrace($0, pane: pane) }
        let fieldReady = wait(8) { self.fields(key, value: value, trace: trace).count == 1 }
        trace?.finish(waitCompleted: fieldReady)
        try require(fieldReady,
                    "Exactly one actual WebKit editable field must expose this key and synthetic value")
        return try XCTUnwrap(fields(key, value: value).first)
    }

    /// Uses the DOM's actual contentEditable control. No compiler call, JS
    /// injection, app-state injection or convenience preview button is used.
    private func replaceField(_ field: XCUIElement, old: String, new: String) throws {
        try require(field.isHittable, "The actual WebKit field must be hittable")
        field.coordinate(withNormalizedOffset: CGVector(dx: 0.98, dy: 0.5)).tap()
        try require(app.keyboards.firstMatch.waitForExistence(timeout: 3), "Real field editing requires the keyboard")
        field.typeText(String(repeating: XCUIKeyboardKey.delete.rawValue, count: old.utf16.count) + new)
        try require(wait { (field.value as? String)?.utf16.elementsEqual(new.utf16) == true },
                    "The original WebKit field must contain the exact typed value before blur")
    }

    private func previewStage(_ stage: String, source: String, elapsed: Double? = nil) throws {
        var value: [String: Any] = [
            "schema": "cherri-native-preview-ui/1", "stage": stage,
            "sourceSha256": CherriJoinedContract.digest(source),
            "testId": "CherriPreviewDocumentUITests/testBRG28ActualWebKitFieldEditAndNumericRejectionUI",
        ]
        if let elapsed { value["elapsedSeconds"] = elapsed }
        let bytes = try JSONSerialization.data(withJSONObject: value, options: [.sortedKeys])
        print("CHERRI_NATIVE_PREVIEW_UI \(String(decoding: bytes, as: UTF8.self))")
    }

    func testBRG28ActualWebKitFieldEditAndNumericRejectionUI() throws {
        let source = "// BRG28 public source preserved\nshow(\"BRG28_KEEP\")\nshow(\"BRG28_EDIT\")\nwait(2)\n// שלום 🍒 preserve tail\n"
        let updated = source.replacingOccurrences(of: "show(\"BRG28_EDIT\")", with: "show(\"BRG28_CHANGED\")")
        let editor = try openedEditor()
        try replaceSource(editor, source)
        try build(editor, source: source, succeeds: true)
        try previewStage("actual-source-built", source: source)
        let initialPane = try pane("Preview")
        let target = try field("Text", value: "BRG28_EDIT", boundary: .firstText, pane: initialPane)
        let kept = try field("Text", value: "BRG28_KEEP", boundary: .keptText, pane: initialPane)
        let started = ProcessInfo.processInfo.systemUptime
        try replaceField(target, old: "BRG28_EDIT", new: "BRG28_CHANGED")
        // Blur through another real DOM textbox; this invokes the existing
        // WebKit message -> native source-map -> rebuild -> publication route.
        kept.tap()
        try pane("Code")
        try expectSource(editor, updated)
        try require(app.staticTexts["compiledStatusText"].exists
                    && !app.staticTexts["diagnosticStatusText"].exists
                    && !app.buttons["shareSignedShortcutButton"].exists,
                    "The committed edit requires its successful new native compilation and no stale signature")
        try previewStage("source-and-native-rebuild-published", source: updated,
                         elapsed: ProcessInfo.processInfo.systemUptime - started)
        try pane("Preview")
        _ = try field("Text", value: "BRG28_CHANGED")
        _ = try field("Text", value: "BRG28_KEEP")
        try require(fields("Text", value: "BRG28_EDIT").isEmpty, "The old rendered value must be gone")
        try previewStage("new-native-graph-rendered", source: updated)

        let number = try field("WFDelayTime", value: "2")
        try replaceField(number, old: "2", new: "invalid-number")
        (try field("Text", value: "BRG28_KEEP")).tap()
        _ = try field("WFDelayTime", value: "2")
        try pane("Code")
        try expectSource(editor, updated)
        try require(app.staticTexts["compiledStatusText"].exists
                    && !app.buttons["shareSignedShortcutButton"].exists,
                    "Rejected numeric DOM input must preserve the committed source and preview")
        try previewStage("invalid-dom-number-preserved-source", source: updated)
        print("CHERRI_NATIVE_PREVIEW_UI_CHECKPOINT=BRG28_WEBKIT_FIELDS_COMPLETE")
    }
}
