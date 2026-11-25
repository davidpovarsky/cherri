import XCTest

/// Dedicated actual app UI producers for BRG25 and BRG27. The existing protected UI suite and startup
/// helpers are unchanged. This class never calls CherriCompiler or injects app
/// state: all source, completion, diagnostics, rename and Build work uses UI.
final class CherriJoinedContractUITests: XCTestCase {
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
            XCTFail("Joined requires a foreground app window within the existing startup budget")
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
                    XCTFail("Joined document readiness did not complete within the original allowance")
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
            XCTFail("Joined editor must be interactive within the unchanged startup budget")
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
            attachment.name = "Joined exact source mismatch"
            attachment.lifetime = .keepAlways
            add(attachment)
        }
        try require(matched, "Joined editor must contain the exact current synthetic UTF-16 source")
    }

    /// No source or title is logged. Only an observed empty editor permits typing
    /// the fixed synthetic fixture; a failed reset stops this test.
    private func replaceSource(_ editor: XCUIElement, _ source: String,
                               generatingClosingParenthesis: Bool = false) throws {
        let clear = app.buttons["clearEditorButton"]
        try require(clear.waitForExistence(timeout: 2) && clear.isHittable,
                    "Joined requires the existing Clear action")
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
                        "Joined synthetic typing requires the real keyboard")
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
                    "Joined Build must be enabled and hittable")
        button.tap()
        let compiled = app.staticTexts["compiledStatusText"]
        let diagnostic = app.staticTexts["diagnosticStatusText"]
        try require(wait(8) {
            succeeds ? (compiled.exists && !diagnostic.exists)
                     : (diagnostic.exists && !compiled.exists)
        }, succeeds ? "Joined current source must compile through Build"
                    : "Joined invalid current source must render a diagnostic")
        try expectSource(editor, source)
    }

    private func observedSource(_ editor: XCUIElement) throws -> String {
        try XCTUnwrap(editor.value as? String, "The real editor value is required")
    }

    private func stage(_ contract: CherriJoinedContract, _ id: String,
                       fixture: JoinedFixture, editor: XCUIElement) throws {
        try contract.stage(id, inputSource: fixture.source, observedSource: observedSource(editor))
    }

    private func exactDiagnostic(_ fixture: JoinedFixture) throws {
        let expected = try XCTUnwrap(fixture.diagnostics)
        for (index, diagnostic) in expected.enumerated() {
            let label = app.staticTexts["diagnosticItem_\(index)"]
            let start = diagnostic.range.start
            let end = diagnostic.range.end
            let length = end.character - start.character
            let prefix = "L\(start.line + 1):C\(start.character + 1) (len:\(length)):"
            try require(wait { label.exists && label.label.hasPrefix(prefix) },
                        "Each actual diagnostic must retain its authored UTF-16 line, column and span")
        }
        try require(!app.staticTexts["diagnosticItem_\(expected.count)"].exists,
                    "Unexpected extra diagnostic rows cannot be hidden by the assertion")
    }

    private func signRejected(_ editor: XCUIElement, source: String) throws {
        try expectSource(editor, source)
        let diagnostic = app.staticTexts["diagnosticStatusText"]
        try require(diagnostic.exists && diagnostic.value as? String == "Build failed",
                    "The preceding actual Build rejection must be distinguishable from a Sign result")
        let sign = app.buttons["signButton"]
        try require(wait { sign.exists && sign.isHittable && sign.isEnabled },
                    "The actual Sign action must be available")
        sign.tap()
        let alert = app.alerts["Sign Shortcut with HubSign?"]
        try require(alert.waitForExistence(timeout: 5), "The actual signing confirmation must appear")
        let confirm = alert.buttons["Send and Sign"]
        try require(confirm.waitForExistence(timeout: 2) && confirm.isHittable,
                    "Only the existing explicit signing confirmation is used")
        confirm.tap()
        try require(wait(8) {
            diagnostic.exists && diagnostic.value as? String == "Signing failed"
                && !self.app.staticTexts["compiledStatusText"].exists
                && !self.app.buttons["shareSignedShortcutButton"].exists
        }, "This exact Sign attempt must finish with a rejection and no exported artifact")
        try expectSource(editor, source)
    }

    private func tapCompletion(_ label: String, editor: XCUIElement, fixture: JoinedFixture) throws {
        try replaceSource(editor, fixture.source, generatingClosingParenthesis: true)
        let candidate = app.buttons["completionItem_\(label)"]
        try require(wait(5) { candidate.exists && candidate.isHittable },
                    "A current shared-service completion must appear without a catalog fallback")
        candidate.tap()
        try expectSource(editor, fixture.expectedSource)
        try require(!fixture.source.utf16.elementsEqual(fixture.expectedSource.utf16),
                    "The completion scenario must cause an actual source change")
    }

    func testBRG25ActualAppRejectedAndLiteralRoutesUI() throws {
        let contract = try CherriJoinedContract(
            testId: "CherriJoinedContractUITests/testBRG25ActualAppRejectedAndLiteralRoutesUI"
        )
        let editor = try openedEditor()
        let empty = try contract.fixture("empty")
        try replaceSource(editor, empty.source)
        try stage(contract, "opened-synthetic-document", fixture: empty, editor: editor)

        for id in ["literal-email", "literal-ask", "literal-directive"] {
            let fixture = try contract.fixture(id)
            try replaceSource(editor, fixture.source)
            try build(editor, source: fixture.source, succeeds: true)
            let status = app.staticTexts["compiledStatusText"]
            try require(status.label.contains("V2") && status.value as? String == "Build succeeded",
                        "The actual current literal must compile through default V2 Build")
            try stage(contract, id + "-build", fixture: fixture, editor: editor)
        }
        for (id, token) in [("rejected-type", "type"), ("rejected-function", "malformed")] {
            let fixture = try contract.fixture(id)
            try replaceSource(editor, fixture.source)
            try build(editor, source: fixture.source, succeeds: false)
            try exactDiagnostic(fixture)
            try require(!app.buttons["shareSignedShortcutButton"].exists,
                        "A rejected current source must expose no signed export")
            try stage(contract, token + "-build-rejected", fixture: fixture, editor: editor)
            try signRejected(editor, source: fixture.source)
            try exactDiagnostic(fixture)
            try stage(contract, token + "-sign-rejected", fixture: fixture, editor: editor)
        }
        try contract.finish()
    }

    func testBRG27ActualEditorJoinedWorkflowUI() throws {
        let contract = try CherriJoinedContract(
            testId: "CherriJoinedContractUITests/testBRG27ActualEditorJoinedWorkflowUI"
        )
        let editor = try openedEditor()
        let empty = try contract.fixture("empty")
        try replaceSource(editor, empty.source)
        try stage(contract, "opened-synthetic-document", fixture: empty, editor: editor)

        let incomplete = try contract.fixture("incomplete-unicode")
        try replaceSource(editor, incomplete.source)
        try build(editor, source: incomplete.source, succeeds: false)
        try stage(contract, "incomplete-unicode-rejected", fixture: incomplete, editor: editor)

        let multiple = try contract.fixture("multi-error-utf16")
        try replaceSource(editor, multiple.source)
        try build(editor, source: multiple.source, succeeds: false)
        try exactDiagnostic(multiple)
        try stage(contract, "multi-error-utf16-rendered", fixture: multiple, editor: editor)

        let parameter = try contract.fixture("parameter-ui")
        try tapCompletion("height", editor: editor, fixture: parameter)
        try stage(contract, "parameter-replacement-ui", fixture: parameter, editor: editor)
        let enumeration = try contract.fixture("enum-ui")
        try tapCompletion("Red", editor: editor, fixture: enumeration)
        try stage(contract, "enum-replacement-ui", fixture: enumeration, editor: editor)

        let newest = try contract.fixture("newest-version")
        let stale = try contract.fixture("stale-version")
        // Repeated real keyboard mutations exercise current-source publication.
        // The separate C ABI producer supplies actual out-of-order version proof.
        for source in [stale.source, newest.source, stale.source, newest.source] {
            try replaceSource(editor, source)
        }
        try build(editor, source: newest.source, succeeds: true)
        try require(!app.otherElements["diagnosticsBanner"].exists
                    && !app.staticTexts["hoverContent"].exists,
                    "Only the current source may retain diagnostic or hover UI")
        try stage(contract, "rapid-edits-current-only", fixture: newest, editor: editor)

        let rename = try contract.fixture("hover-rename")
        try replaceSource(editor, rename.source)
        let hover = app.buttons["hoverButton"]
        try require(hover.waitForExistence(timeout: 5) && hover.isHittable,
                    "The real Hover action must be available")
        hover.tap()
        let contents = app.staticTexts["hoverContent"]
        try require(wait { contents.exists && contents.label.contains("(variable) value") },
                    "Actual shared hover must identify the current semantic symbol")
        try stage(contract, "hover-current-symbol", fixture: rename, editor: editor)

        let renameButton = app.buttons["renameButton"]
        try require(renameButton.waitForExistence(timeout: 5) && renameButton.isHittable,
                    "The actual Rename action must be available")
        renameButton.tap()
        let alert = app.alerts["Rename Symbol"]
        try require(alert.waitForExistence(timeout: 5), "The actual Rename dialog must appear")
        let fields = alert.textFields.matching(NSPredicate(
            format: "identifier == %@ OR placeholderValue == %@", "renameTextField", "New Name"))
        try require(wait { fields.count == 1 && fields.element(boundBy: 0).isHittable },
                    "Exactly one real Rename field is required")
        fields.element(boundBy: 0).tap()
        fields.element(boundBy: 0).typeText("renamedValue")
        let confirmations = alert.buttons.matching(NSPredicate(
            format: "identifier == %@ OR label == %@", "confirmRenameButton", "Rename"))
        try require(wait {
            confirmations.count == 1 && confirmations.element(boundBy: 0).isHittable
                && confirmations.element(boundBy: 0).isEnabled
        }, "Exactly one enabled real Rename confirmation is required")
        confirmations.element(boundBy: 0).tap()
        try expectSource(editor, rename.expectedSource)
        try require(alert.waitForNonExistence(timeout: 5), "The Rename dialog must dismiss")
        try require(!contents.exists, "Hover for the old source must not survive a semantic rename")
        try stage(contract, "scoped-rename-ui", fixture: rename, editor: editor)

        let format = try contract.fixture("format")
        try replaceSource(editor, format.source)
        let options = app.buttons["workspaceOptionsButton"]
        try require(options.waitForExistence(timeout: 3) && options.isHittable,
                    "The actual workspace menu must be available")
        options.tap()
        let formatActions = app.buttons.matching(NSPredicate(
            format: "identifier == %@ OR label == %@", "formatSourceButton", "Format Source"))
        try require(wait {
            formatActions.count == 1 && formatActions.element(boundBy: 0).isHittable
                && formatActions.element(boundBy: 0).isEnabled
        }, "The actual Format Source action must be enabled")
        formatActions.element(boundBy: 0).tap()
        try expectSource(editor, format.expectedSource)
        try stage(contract, "format-current-source", fixture: format, editor: editor)

        let current = try contract.fixture("current-source")
        try expectSource(editor, current.source)
        try build(editor, source: current.source, succeeds: true)
        try require(!app.otherElements["diagnosticsBanner"].exists,
                    "Current formatted-source Build must clear all diagnostics")
        try stage(contract, "current-source-compiled", fixture: current, editor: editor)
        try contract.finish()
    }
}
