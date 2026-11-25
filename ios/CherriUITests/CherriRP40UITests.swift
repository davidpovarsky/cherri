import XCTest

/// A separate UI producer for RP40. The existing protected UI suite and startup
/// helpers are unchanged. This class never calls CherriCompiler or injects app
/// state: all source, completion, diagnostics, rename and Build work uses UI.
final class CherriRP40UITests: XCTestCase {
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
            XCTFail("RP40 requires a foreground app window within the existing startup budget")
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
                    XCTFail("RP40 document readiness did not complete within the original allowance")
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
            XCTFail("RP40 editor must be interactive within the unchanged startup budget")
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
        try require(wait {
            guard editor.exists, let value = editor.value as? String else { return false }
            return value.utf16.elementsEqual(source.utf16)
        }, "RP40 editor must contain the exact current synthetic UTF-16 source")
    }

    /// No source or title is logged. Only an observed empty editor permits typing
    /// the fixed synthetic fixture; a failed reset stops this test.
    private func replaceSource(_ editor: XCUIElement, _ source: String) throws {
        let clear = app.buttons["clearEditorButton"]
        try require(clear.waitForExistence(timeout: 2) && clear.isHittable,
                    "RP40 requires the existing Clear action")
        clear.tap()
        try expectSource(editor, "")
        if !source.isEmpty {
            editor.coordinate(withNormalizedOffset: CGVector(dx: 0.5, dy: 0.5)).tap()
            try require(app.keyboards.firstMatch.waitForExistence(timeout: 2),
                        "RP40 synthetic typing requires the real keyboard")
            editor.typeText(source)
            try expectSource(editor, source)
        }
    }

    private func requireManualBuildPending(_ editor: XCUIElement, source: String) throws {
        try expectSource(editor, source)
        let compiled = app.staticTexts["compiledStatusText"]
        let signed = app.buttons["shareSignedShortcutButton"]
        let diagnostic = app.staticTexts["diagnosticStatusText"]
        let item = app.staticTexts["diagnosticItem_0"]
        try require(wait { !compiled.exists && !signed.exists && !diagnostic.exists && !item.exists },
                    "RP40 current source must invalidate old artifacts and stale diagnostics")
        // Observe across the existing 650ms live-preview debounce. This is an
        // inverted UI expectation, not a delay that assumes an outcome.
        let automaticResult = XCTNSPredicateExpectation(predicate: NSPredicate { _, _ in
            compiled.exists || signed.exists || diagnostic.exists || item.exists
        }, object: app)
        automaticResult.isInverted = true
        try require(XCTWaiter.wait(for: [automaticResult], timeout: 1.5) == .completed,
                    "RP40 must publish no automatic result before its real manual Build tap")
        try expectSource(editor, source)
    }

    private func build(_ editor: XCUIElement, source: String, succeeds: Bool) throws {
        try expectSource(editor, source)
        let button = app.buttons["buildButton"]
        try require(wait { button.exists && button.isHittable && button.isEnabled },
                    "RP40 Build must be enabled and hittable")
        button.tap()
        let compiled = app.staticTexts["compiledStatusText"]
        let diagnostic = app.staticTexts["diagnosticStatusText"]
        try require(wait(8) {
            succeeds ? (compiled.exists && !diagnostic.exists)
                     : (diagnostic.exists && !compiled.exists)
        }, succeeds ? "RP40 current source must compile through Build"
                    : "RP40 invalid current source must render a diagnostic")
        try expectSource(editor, source)
    }

    private func stage(_ value: String) {
        // Caller values below are fixed protocol tokens, never UI/user text.
        print("[CHERRI_RP40_UI_STAGE] \(value)")
    }

    func testRP40ActualAppDocumentWorkflowUI() throws {
        let editor = try openedEditor()
        try replaceSource(editor, "")
        stage("opened-synthetic-document")

        let prefix = "// RP40 value comment שלום 🌍\n"
            + "let value = text(\"RP40_VALUE\")\n"
            + "let untouched = \"value remains literal\"\n"
        try replaceSource(editor, prefix + "sho")
        let candidate = app.buttons["completionItem_show"]
        try require(wait(3) { candidate.exists && candidate.isHittable },
                    "RP40 requires the exact contextual service completion, with no catalog fallback")
        candidate.tap()
        let unfilled = prefix + "show(<#input#>)"
        try expectSource(editor, unfilled)
        stage("contextual-completion")

        try build(editor, source: unfilled, succeeds: false)
        stage("unfilled-rejected")

        let complete = prefix + "show(value)\n"
        try replaceSource(editor, complete)
        try requireManualBuildPending(editor, source: complete)
        try build(editor, source: complete, succeeds: true)
        stage("source-compiled")

        let invalid = complete + "unknownRP40()\n"
        try replaceSource(editor, invalid)
        try build(editor, source: invalid, succeeds: false)
        let diagnostic = app.staticTexts["diagnosticItem_0"]
        try require(wait {
            diagnostic.exists && diagnostic.label.hasPrefix("L5:C1 (len:11):")
                && diagnostic.label.contains("unknownRP40")
        }, "RP40 current unknown action must render its exact line, column and span")
        stage("diagnostics-rendered")

        try replaceSource(editor, complete)
        let rename = app.buttons["renameButton"]
        try require(rename.waitForExistence(timeout: 5) && rename.isHittable,
                    "RP40 requires the real Rename action")
        rename.tap()
        let alert = app.alerts["Rename Symbol"]
        try require(alert.waitForExistence(timeout: 5), "RP40 Rename dialog must appear")
        // SwiftUI's native alert may expose the authored placeholder instead of
        // its view identifier. Require one exact field in this named dialog.
        let fields = alert.textFields.matching(NSPredicate(
            format: "identifier == %@ OR placeholderValue == %@", "renameTextField", "New Name"))
        try require(wait { fields.count == 1 && fields.element(boundBy: 0).isHittable },
                    "RP40 requires one exact Rename text field")
        let field = fields.element(boundBy: 0)
        field.tap()
        field.typeText("renamedValue")
        let confirmations = alert.buttons.matching(NSPredicate(
            format: "identifier == %@ OR label == %@", "confirmRenameButton", "Rename"))
        try require(wait {
            confirmations.count == 1 && confirmations.element(boundBy: 0).isHittable
                && confirmations.element(boundBy: 0).isEnabled
        }, "RP40 requires one exact Rename confirmation")
        let confirm = confirmations.element(boundBy: 0)
        confirm.tap()
        let renamed = "// RP40 value comment שלום 🌍\n"
            + "let renamedValue = text(\"RP40_VALUE\")\n"
            + "let untouched = \"value remains literal\"\n"
            + "show(renamedValue)\n"
        try expectSource(editor, renamed)
        try require(alert.waitForNonExistence(timeout: 5), "RP40 Rename dialog must dismiss")
        stage("semantic-rename")

        try requireManualBuildPending(editor, source: renamed)
        try build(editor, source: renamed, succeeds: true)
        try require(!app.otherElements["diagnosticsBanner"].exists,
                    "RP40 successful renamed-source build must clear rendered diagnostics")
        stage("renamed-source-compiled")
        print("[CHECKPOINT:RP40_APP_UI_JOINED_VERIFIED]")
    }
}
