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
                               generatingClosingParenthesis: Bool = false,
                               removingGeneratedClosingParenthesis: Bool = false) throws {
        try require(!(generatingClosingParenthesis && removingGeneratedClosingParenthesis),
                    "The authored fixture must select one keyboard closing behavior")
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
            if removingGeneratedClosingParenthesis {
                // The malformed function intentionally omits ')'. Real typing
                // leaves the protected auto-generated closer at the caret.
                // Overtype it, verify unchanged bytes, then remove it with the
                // real Delete key before testing the exact authored source.
                try expectSource(editor, source + ")")
                editor.typeText(")")
                try expectSource(editor, source + ")")
                editor.typeText(XCUIKeyboardKey.delete.rawValue)
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

    private func signRejected(_ editor: XCUIElement, source: String,
                              boundary: CherriUIReadinessTrace.Boundary) throws {
        try expectSource(editor, source)
        let diagnostic = app.staticTexts["diagnosticStatusText"]
        try require(diagnostic.exists && diagnostic.value as? String == "Build failed",
                    "The preceding actual Build rejection must be distinguishable from a Sign result")
        let sign = app.buttons["signButton"]
        let trace = CherriUIReadinessTrace(boundary)
        let signReady = wait { () -> Bool in
            let exists = sign.exists
            guard exists else {
                trace.sign(exists: exists, hittable: nil, enabled: nil)
                return false
            }
            let hittable = sign.isHittable
            guard hittable else {
                trace.sign(exists: exists, hittable: hittable, enabled: nil)
                return false
            }
            let enabled = sign.isEnabled
            trace.sign(exists: exists, hittable: hittable, enabled: enabled)
            return enabled
        }
        trace.finish(waitCompleted: signReady)
        try require(signReady, "The actual Sign action must be available")
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
            try replaceSource(editor, fixture.source,
                              removingGeneratedClosingParenthesis: id == "rejected-function")
            try build(editor, source: fixture.source, succeeds: false)
            try exactDiagnostic(fixture)
            try require(!app.buttons["shareSignedShortcutButton"].exists,
                        "A rejected current source must expose no signed export")
            try stage(contract, token + "-build-rejected", fixture: fixture, editor: editor)
            try signRejected(editor, source: fixture.source,
                             boundary: id == "rejected-type" ? .typeSign : .malformedSign)
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
        try require(wait {
            fields.count == 1 && (fields.element(boundBy: 0).value as? String) == "renamedValue"
        }, "The actual Rename field must contain the exact requested identifier")
        // This test launches in English. Match the semantic action inside the
        // named native alert; an optional SwiftUI identifier must not also admit
        // a different action. Keep original AX identities in the private bundle.
        let nativeActions = alert.buttons.allElementsBoundByAccessibilityElement.prefix(8).map {
            ["identifier": $0.identifier, "label": $0.label,
             "enabled": $0.isEnabled, "hittable": $0.isHittable] as [String: Any]
        }
        let identity = XCTAttachment(
            data: try JSONSerialization.data(withJSONObject: nativeActions, options: [.sortedKeys]),
            uniformTypeIdentifier: "public.json"
        )
        identity.name = "Rename action identities for public synthetic fixture"
        identity.lifetime = .keepAlways
        add(identity)
        let confirmations = alert.buttons.matching(NSPredicate(format: "label == %@", "Rename"))
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
    private func rp39Observation() -> [String: Any]? {
        let element = app.otherElements["rp39NativeEditorObservation"]
        guard element.exists, let text = element.value as? String,
              text.utf8.count <= 8192, let data = text.data(using: .utf8),
              let value = try? JSONSerialization.jsonObject(with: data) as? [String: Any],
              Set(value.keys) == ["native", "delivery"] else { return nil }
        return value
    }

    private func rp39Native(_ source: String) throws -> [String: Any] {
        var observed: [String: Any]?
        try require(wait {
            guard let value = self.rp39Observation()?["native"] as? [String: Any],
                  value["status"] as? String == "observed",
                  value["editorCount"] as? Int == 1,
                  value["sourceSHA256"] as? String == CherriJoinedContract.digest(source),
                  value["utf16Length"] as? Int == source.utf16.count else { return false }
            observed = value
            return true
        }, "RP39 requires the real unique UITextView and exact current UTF-16 source")
        return try XCTUnwrap(observed)
    }

    private func rp39Select(_ editor: XCUIElement, source: String, offset: Int) throws {
        let native = try rp39Native(source)
        let dx = try XCTUnwrap(native["targetDX"] as? Double)
        let dy = try XCTUnwrap(native["targetDY"] as? Double)
        try require(native["targetOffset"] as? Int == offset && (0...1).contains(dx) && (0...1).contains(dy),
                    "RP39 caret coordinate must come from this exact native source and target")
        let target = editor.coordinate(withNormalizedOffset: CGVector(dx: CGFloat(dx), dy: CGFloat(dy)))
        target.tap()
        // A coordinate tap is not an exact character-placement contract. Admit
        // one real hardware-arrow event only when this fresh native witness
        // predicts the exact requested offset in one unambiguous direction.
        // The unchanged post-input predicate remains the success authority.
        let afterTap = rp39Observation()?["native"] as? [String: Any]
        var keyboardDirection: String?
        if let observed = afterTap,
           observed["status"] as? String == "observed",
           observed["editorCount"] as? Int == 1,
           observed["sourceSHA256"] as? String == CherriJoinedContract.digest(source),
           observed["utf16Length"] as? Int == source.utf16.count,
           observed["targetOffset"] as? Int == offset,
           observed["firstResponder"] as? Bool == true,
           observed["selectionLength"] as? Int == 0,
           let current = observed["selectionLocation"] as? Int,
           observed["currentCaretOffset"] as? Int == current,
           current != offset {
            let directions = [
                ("left", "currentCaretLeftUTF16Offset", XCUIKeyboardKey.leftArrow.rawValue),
                ("right", "currentCaretRightUTF16Offset", XCUIKeyboardKey.rightArrow.rawValue),
            ].filter { observed[$0.1] as? Int == offset }
            if directions.count == 1, let direction = directions.first {
                keyboardDirection = direction.0
                editor.typeKey(direction.2, modifierFlags: [])
            }
        }
        var lastNative: [String: Any]?
        var observedPolls = 0
        let selected = wait {
            guard let value = self.rp39Observation()?["native"] as? [String: Any] else { return false }
            lastNative = value
            observedPolls += 1
            return value["sourceSHA256"] as? String == CherriJoinedContract.digest(source)
                && value["selectionLocation"] as? Int == offset
                && value["selectionLength"] as? Int == 0
                && value["firstResponder"] as? Bool == true
        }
        if !selected {
            // Retain the actual pre-input, key-admission and in-wait reads.
            // This failure attachment adds no further query, event or deadline.
            let fields = ["status", "editorCount", "sourceSHA256", "utf16Length",
                          "selectionLocation", "selectionLength", "firstResponder",
                          "targetOffset", "targetDX", "targetDY", "targetEditorPoint",
                          "targetWindowPoint", "editorWindowFrame", "targetClosestUTF16Offset",
                          "currentCaretOffset", "currentCaretDX", "currentCaretDY",
                          "currentCaretEditorPoint", "currentCaretWindowPoint",
                          "currentCaretLeftUTF16Offset", "currentCaretRightUTF16Offset"]
            func retained(_ observation: [String: Any]?) -> [String: Any] {
                guard let observation else { return ["status": "not-observed"] }
                return Dictionary(uniqueKeysWithValues: fields.compactMap { key in
                    observation[key].map { (key, $0) }
                })
            }
            let observation: [String: Any] = [
                "schema": "cherri-rp39-caret-failure-observation/1",
                "diagnosticOnly": true, "requestedUTF16Offset": offset,
                "gesture": "target-tap-and-at-most-one-native-direction-key",
                "keyboardDirection": keyboardDirection as Any? ?? NSNull(),
                "keyInvocationCount": keyboardDirection == nil ? 0 : 1,
                "observedPolls": observedPolls, "waitCompleted": selected,
                "before": retained(native), "afterTap": retained(afterTap), "last": retained(lastNative),
            ]
            if let data = try? JSONSerialization.data(withJSONObject: observation, options: [.sortedKeys]),
               data.count <= 4096 {
                let attachment = XCTAttachment(data: data, uniformTypeIdentifier: "public.json")
                attachment.name = "RP39 native caret failure observation"
                attachment.lifetime = .keepAlways
                add(attachment)
            }
        }
        try require(selected, "The real pointer event must place the actual native caret at the authored UTF-16 offset")
    }

    private func rp39Format(_ editor: XCUIElement, fixture: JoinedFixture) throws {
        try expectSource(editor, fixture.source)
        let options = app.buttons["workspaceOptionsButton"]
        try require(options.waitForExistence(timeout: 3) && options.isHittable,
                    "RP39 requires the actual workspace menu")
        options.tap()
        let actions = app.buttons.matching(NSPredicate(
            format: "identifier == %@ OR label == %@", "formatSourceButton", "Format Source"))
        try require(wait {
            actions.count == 1 && actions.element(boundBy: 0).isHittable
                && actions.element(boundBy: 0).isEnabled
        }, "RP39 requires the unique enabled Format Source action")
        actions.element(boundBy: 0).tap()
        try expectSource(editor, fixture.expectedSource)
    }

    private func rp39Rename(_ editor: XCUIElement, fixture: JoinedFixture,
                           target: Int, name: String) throws {
        try expectSource(editor, fixture.source)
        try rp39Select(editor, source: fixture.source, offset: target)
        let rename = app.buttons["renameButton"]
        try require(rename.waitForExistence(timeout: 3) && rename.isHittable && rename.isEnabled,
                    "RP39 semantic Rename must use its actual control")
        rename.tap()
        let alert = app.alerts["Rename Symbol"]
        try require(alert.waitForExistence(timeout: 5), "RP39 requires the native Rename dialog")
        let field = alert.textFields["renameTextField"]
        try require(field.waitForExistence(timeout: 2) && field.isHittable,
                    "RP39 requires the actual rename input")
        field.tap()
        field.typeText(name)
        let confirmations = alert.buttons.matching(NSPredicate(format: "label == %@", "Rename"))
        try require(wait {
            confirmations.count == 1 && confirmations.element(boundBy: 0).isHittable
                && confirmations.element(boundBy: 0).isEnabled
        }, "RP39 requires one actual Rename confirmation")
        confirmations.element(boundBy: 0).tap()
        try expectSource(editor, fixture.expectedSource)
        try require(alert.waitForNonExistence(timeout: 5), "The real Rename dialog must finish")
    }

    private func rp39ExpectedDiagnostics(_ fixture: JoinedFixture) throws -> [[String: Any]] {
        try XCTUnwrap(fixture.diagnostics).map {
            ["code": $0.code, "severity": $0.severity,
             "startLine": $0.range.start.line, "startCharacter": $0.range.start.character,
             "endLine": $0.range.end.line, "endCharacter": $0.range.end.character]
        }
    }

    /// RP39 is one actual document workflow. Only response delivery is gated:
    /// every compiler analysis, source edit, caret move and publication is real.
    func testRP39ActualCombinedEditorAndDelayedAnalysisUI() throws {
        let contract = try CherriJoinedContract(
            testId: "CherriJoinedContractUITests/testRP39ActualCombinedEditorAndDelayedAnalysisUI"
        )
        let format = try contract.fixture("rp39-format")
        let idempotent = try contract.fixture("rp39-format-idempotent")
        let outer = try contract.fixture("rp39-outer-rename")
        let inner = try contract.fixture("rp39-inner-rename")
        let older = try contract.fixture("rp39-analysis-old")
        let current = try contract.fixture("rp39-analysis-new")
        let recovery = try contract.fixture("rp39-recovery")
        func offset(_ source: String, call: String, prefix: String) throws -> Int {
            let range = (source as NSString).range(of: call)
            try require(range.location != NSNotFound, "The independent Unicode caret fixture must be present")
            return range.location + (prefix as NSString).length + 1
        }
        let outerOffset = try offset(outer.source, call: "show(f\"שלום🙂{ערך}\")", prefix: "show(f\"שלום🙂{")
        let innerOffset = try offset(inner.source, call: "show(f\"🙂{ערך}\")", prefix: "show(f\"🙂{")
        let nonce = UUID().uuidString
        let configuration: [String: Any] = [
            "schema": "cherri-rp39-ui-observation/1", "nonce": nonce,
            "olderSourceSHA256": older.sourceSha256,
            "currentSourceSHA256": current.sourceSha256,
            "caretTargets": [outer.sourceSha256: outerOffset, inner.sourceSha256: innerOffset,
                             older.sourceSha256: older.source.utf16.count,
                             current.sourceSha256: current.source.utf16.count],
        ]
        // A dedicated test launch enables read-only native observations and a
        // bounded delivery gate; all other original UI methods stay unchanged.
        app.terminate()
        app.launchEnvironment["CHERRI_RP39_UI_OBSERVATION"] = String(
            decoding: try JSONSerialization.data(withJSONObject: configuration, options: [.sortedKeys]),
            as: UTF8.self
        )
        app.launch()
        let editor = try openedEditor()
        try replaceSource(editor, format.source)
        try rp39Format(editor, fixture: format)
        try stage(contract, "first-actual-format", fixture: format, editor: editor)
        let firstCaret = try rp39Native(idempotent.source)
        try rp39Format(editor, fixture: idempotent)
        let secondCaret = try rp39Native(idempotent.source)
        try require(firstCaret["selectionLocation"] as? Int == secondCaret["selectionLocation"] as? Int
                    && firstCaret["selectionLength"] as? Int == secondCaret["selectionLength"] as? Int,
                    "The idempotent second Format must preserve the actual native caret")
        try stage(contract, "second-format-idempotent-caret", fixture: idempotent, editor: editor)
        try rp39Rename(editor, fixture: outer, target: outerOffset, name: "סכום")
        try stage(contract, "outer-scope-unicode-rename", fixture: outer, editor: editor)
        try rp39Rename(editor, fixture: inner, target: innerOffset, name: "פנים")
        try stage(contract, "inner-scope-unicode-rename", fixture: inner, editor: editor)

        try replaceSource(editor, older.source)
        let options = app.buttons["workspaceOptionsButton"]
        try require(options.waitForExistence(timeout: 3) && options.isHittable,
                    "RP39 requires the actual Live Preview option")
        options.tap()
        let toggles = app.descendants(matching: .any).matching(identifier: "livePreviewToggle")
        try require(wait { toggles.count == 1 && toggles.element(boundBy: 0).isHittable },
                    "RP39 requires one actual Live Preview toggle")
        toggles.element(boundBy: 0).tap()
        try rp39Select(editor, source: older.source, offset: older.source.utf16.count)
        let buildButton = app.buttons["buildButton"]
        try require(wait { buildButton.exists && buildButton.isHittable && buildButton.isEnabled },
                    "The delayed older request must start from the real manual Build")
        buildButton.tap()
        try require(wait(8) {
            guard let delivery = self.rp39Observation()?["delivery"] as? [String: Any] else { return false }
            return delivery["nonce"] as? String == nonce
                && delivery["phase"] as? String == "older-real-analysis-held"
        }, "The gate must hold an actual completed older analysis inside the original Build allowance")
        try expectSource(editor, older.source)
        try stage(contract, "older-real-analysis-held", fixture: older, editor: editor)

        // Only five real Delete events and the replacement suffix are needed.
        // No source injection or artificial completion can create the newer result.
        let beforeTyping = try rp39Native(older.source)
        try require(beforeTyping["selectionLocation"] as? Int == older.source.utf16.count
                    && beforeTyping["selectionLength"] as? Int == 0
                    && beforeTyping["firstResponder"] as? Bool == true,
                    "The actual older editor caret must still own keyboard input")
        editor.typeText(String(repeating: XCUIKeyboardKey.delete.rawValue, count: 5) + "Newer)\n")
        try expectSource(editor, current.source)
        var delivered: [String: Any]?
        try require(wait(8) {
            guard let delivery = self.rp39Observation()?["delivery"] as? [String: Any],
                  delivery["nonce"] as? String == nonce,
                  delivery["phase"] as? String == "older-analysis-discarded" else { return false }
            delivered = delivery
            return true
        }, "A real newer result must publish before the unchanged guard rejects the delayed older result")
        let delivery = try XCTUnwrap(delivered)
        let oldIdentity = try XCTUnwrap(delivery["older"] as? [String: Any])
        let newIdentity = try XCTUnwrap(delivery["current"] as? [String: Any])
        try require(oldIdentity["documentID"] as? String == newIdentity["documentID"] as? String
                    && oldIdentity["sourceSHA256"] as? String == older.sourceSha256
                    && newIdentity["sourceSHA256"] as? String == current.sourceSha256
                    && oldIdentity["liveBuild"] as? Bool == false
                    && newIdentity["liveBuild"] as? Bool == true,
                    "Both real analyses must belong to one document and their actual manual/live requests")
        let oldRevision = try XCTUnwrap(oldIdentity["revision"] as? UInt64)
        let newRevision = try XCTUnwrap(newIdentity["revision"] as? UInt64)
        let oldGeneration = try XCTUnwrap(oldIdentity["generation"] as? UInt64)
        let newGeneration = try XCTUnwrap(newIdentity["generation"] as? UInt64)
        try require(newRevision > oldRevision && newGeneration > oldGeneration,
                    "Actual document and Build generations must advance")
        let oldDiagnostics = try XCTUnwrap(delivery["olderDiagnostics"] as? [[String: Any]])
        let newDiagnostics = try XCTUnwrap(delivery["currentDiagnostics"] as? [[String: Any]])
        try require(NSArray(array: oldDiagnostics).isEqual(to: try rp39ExpectedDiagnostics(older))
                    && NSArray(array: newDiagnostics).isEqual(to: try rp39ExpectedDiagnostics(current)),
                    "Both delayed and current values must be the distinct real compiler diagnostics with exact UTF-16 spans")
        try exactDiagnostic(current)
        try stage(contract, "current-real-analysis-published", fixture: current, editor: editor)
        let publishedCaret = try XCTUnwrap(delivery["currentCaret"] as? [String: Any])
        let discardedCaret = try XCTUnwrap(delivery["discardedCaret"] as? [String: Any])
        let actualCaret = try rp39Native(current.source)
        for caret in [publishedCaret, discardedCaret, actualCaret] {
            try require(caret["status"] as? String == "observed"
                        && caret["sourceSHA256"] as? String == current.sourceSha256
                        && caret["selectionLocation"] as? Int == current.source.utf16.count
                        && caret["selectionLength"] as? Int == 0,
                        "New publication, old rejection and current UI must preserve the same actual native caret")
        }
        try expectSource(editor, current.source)
        try require(!app.staticTexts["compiledStatusText"].exists
                    && !app.buttons["shareSignedShortcutButton"].exists,
                    "Neither invalid analysis may publish a compiled or signed artifact")
        try stage(contract, "delayed-old-rejected-caret-preserved", fixture: current, editor: editor)

        try replaceSource(editor, recovery.source)
        try build(editor, source: recovery.source, succeeds: true)
        try require(!app.otherElements["diagnosticsBanner"].exists,
                    "The subsequent actual valid Build must clear the previous diagnostics")
        try stage(contract, "current-source-recovery-built", fixture: recovery, editor: editor)
        try contract.finish()
    }
}
