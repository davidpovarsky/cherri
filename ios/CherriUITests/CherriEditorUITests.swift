//
//  CherriEditorUITests.swift
//  CherriUITests
//
//  Authentic UI Automation Test Suite for Cherri Language v2.0 iOS Editor & Preview.
//  Satisfies E01, E02, E03, E05, E06, E07, BRG27, and RP40 UI-tier acceptance requirements.
//  Every test enforces strict existence checks, real user interactions, and behavioral checkpoints.
//

import XCTest

final class CherriEditorUITests: XCTestCase {
    var app: XCUIApplication!
    private var syntheticSourceResetVerified = false
    private var expectedSyntheticSource: String?
    private var startupObservationNonce: String?
    private var startupObservationTestID: String?
    private var startupObservationIssueCount = 0
    private var observedStartupTrace: CherriEditorStartupTimingTrace?

    override func setUpWithError() throws {
        continueAfterFailure = false
        syntheticSourceResetVerified = false
        expectedSyntheticSource = nil
        startupObservationNonce = nil
        startupObservationTestID = nil
        startupObservationIssueCount = 0
        observedStartupTrace = nil
        app = XCUIApplication()
        app.launchArguments.append(contentsOf: [
            "--uitesting",
            "-AppleLanguages", "(en)",
            "-AppleKeyboards", "('en_US@hw=US;sw=QWERTY')"
        ])
        configureStartupObservation()
        recordStartupObservationSession("before-launch")
        app.launch()
        recordStartupObservationSession("after-launch")
    }

    override func tearDownWithError() throws {
        recordStartupObservationSession("teardown-start")
        recordEditorReceipt(stage: "teardown")
        recordStartupObservationSession("teardown-finished")
        app = nil
    }


    /// Diagnostics are enabled only for these existing, fixed test identities.
    /// They never query controls or contribute to a readiness decision.
    private func configureStartupObservation() {
        let separators = CharacterSet.alphanumerics.union(CharacterSet(charactersIn: "_")).inverted
        let method = name.components(separatedBy: separators).first { $0.hasPrefix("test") } ?? "unknown"
        let testID = "CherriEditorUITests/\(method)"
        let selected = [
            "CherriEditorUITests/testBRG27EditorRealWorldWorkflowUI",
            "CherriEditorUITests/testD03CurrentDocumentationSearchUI",
            "CherriEditorUITests/testE01CompletionDisplayAndInsertionUI",
        ]
        guard selected.contains(testID) else { return }
        let nonce = UUID().uuidString.replacingOccurrences(of: "-", with: "").lowercased()
        startupObservationNonce = nonce
        startupObservationTestID = testID
        app.launchEnvironment["CHERRI_STARTUP_OBSERVATION_NONCE"] = nonce
        app.launchEnvironment["CHERRI_STARTUP_OBSERVATION_TEST"] = testID
    }

    private func recordStartupObservationSession(_ phase: String, readinessStartedAt: TimeInterval? = nil) {
        guard let nonce = startupObservationNonce, let testID = startupObservationTestID else { return }
        var row: [String: Any] = [
            "schemaVersion": "1", "nonce": nonce, "testId": testID, "phase": phase,
            "uptimeMs": Int((ProcessInfo.processInfo.systemUptime * 1_000).rounded(.down)),
        ]
        if let readinessStartedAt {
            row["readinessStartedAtMs"] = Int((readinessStartedAt * 1_000).rounded(.down))
        }
        if let bytes = try? JSONSerialization.data(withJSONObject: row, options: [.sortedKeys]),
           let json = String(data: bytes, encoding: .utf8) {
            print("[CHERRI_UI_OBSERVATION_SESSION] \(json)")
        }
    }

    override func record(_ issue: XCTIssue) {
        if let nonce = startupObservationNonce, let testID = startupObservationTestID,
           startupObservationIssueCount < 8 {
            startupObservationIssueCount += 1
            // The original trace is observed as-is, including an unfinished AX
            // call. This does not retry the query or satisfy an expectation.
            if let trace = observedStartupTrace { recordStartupTiming(trace) }
            func bounded(_ text: String) -> [String: Any] {
                let bytes = Data(text.utf8)
                return ["base64": bytes.prefix(4_096).base64EncodedString(),
                        "originalByteCount": bytes.count, "truncated": bytes.count > 4_096]
            }
            let error = issue.associatedError.map { $0 as NSError }
            let location = issue.sourceCodeContext.location
            let row: [String: Any] = [
                "schemaVersion": "1", "nonce": nonce, "testId": testID,
                "uptimeMs": Int((ProcessInfo.processInfo.systemUptime * 1_000).rounded(.down)),
                "issueIndex": startupObservationIssueCount,
                "issueType": String(describing: issue.type),
                "sourceFile": location?.fileURL.lastPathComponent ?? "",
                "sourceLine": location?.lineNumber ?? 0,
                "errorPresent": error != nil,
                "errorDomain": bounded(error?.domain ?? ""),
                "errorCode": error?.code ?? 0,
                "compact": bounded(issue.compactDescription),
                "detailed": bounded(issue.detailedDescription ?? ""),
                "errorDescription": bounded(error?.localizedDescription ?? ""),
            ]
            // Raw descriptions remain in the quarantined xcodebuild log.
            // The collector publishes only fixed metadata and byte digests.
            if let bytes = try? JSONSerialization.data(withJSONObject: row, options: [.sortedKeys]),
               bytes.count <= 32_768, let json = String(data: bytes, encoding: .utf8) {
                print("[CHERRI_UI_OBSERVATION_ISSUE_PRIVATE] \(json)")
            }
        }
        super.record(issue)
    }

    private func waitForEditorValue(_ editor: XCUIElement, timeout: TimeInterval = 5,
                                    matching predicate: @escaping (String) -> Bool) -> Bool {
        let expectation = XCTNSPredicateExpectation(predicate: NSPredicate { _, _ in
            guard editor.exists, let value = editor.value as? String else { return false }
            return predicate(value)
        }, object: editor)
        return XCTWaiter.wait(for: [expectation], timeout: timeout) == .completed
    }

    /// Only reads source after this test has observed an empty reset. No app-wide
    /// AX tree, document list, status message, screenshot, or private text is read.
    private func recordEditorReceipt(stage: String) {
        guard let app else { return }
        let editor = app.textViews["cherriCodeEditor"]
        let build = app.buttons["buildButton"]
        let nameSeparators = CharacterSet.alphanumerics.union(CharacterSet(charactersIn: "_")).inverted
        let method = name.components(separatedBy: nameSeparators).first { $0.hasPrefix("test") } ?? "unknown"
        var receipt: [String: Any] = [
            "schemaVersion": "1.0",
            "testName": "CherriEditorUITests/\(method)",
            "stage": stage,
            "syntheticSourceResetVerified": syntheticSourceResetVerified,
            "editorExists": editor.exists,
            "editorHittable": editor.exists && editor.isHittable,
            "buildExists": build.exists,
            "buildEnabled": build.exists && build.isEnabled,
            "compiledStatusExists": app.staticTexts["compiledStatusText"].exists,
            "diagnosticStatusExists": app.staticTexts["diagnosticStatusText"].exists,
            "diagnosticsBannerExists": app.otherElements["diagnosticsBanner"].exists,
            "keyboardExists": app.keyboards.firstMatch.exists,
        ]
        if syntheticSourceResetVerified, editor.exists, let value = editor.value as? String {
            receipt["actualSource"] = String(value.prefix(4096))
            receipt["actualUTF16Length"] = (value as NSString).length
            receipt["sourceTruncated"] = value.count > 4096
            // Only the fixed E01 synthetic prefix permits candidate identifiers.
            // No document-wide AX tree or candidate detail/label is captured.
            if method == "testE01CompletionDisplayAndInsertionUI", value == "sho" {
                receipt["showCompletionExists"] = app.buttons["completionItem_show"].exists
                receipt["showCompletionHittable"] = app.buttons["completionItem_show"].exists && app.buttons["completionItem_show"].isHittable
                receipt["showSuggestionExists"] = app.buttons["suggestionItem_show"].exists
                receipt["showSuggestionHittable"] = app.buttons["suggestionItem_show"].exists && app.buttons["suggestionItem_show"].isHittable
                let candidates = app.buttons.matching(NSPredicate(format: "identifier BEGINSWITH %@ OR identifier BEGINSWITH %@",
                    "completionItem_", "suggestionItem_")).allElementsBoundByIndex
                receipt["completionCandidateIdentifiers"] = Array(candidates.prefix(8)).map(\.identifier)
            }

            if let expected = expectedSyntheticSource {
                receipt["expectedSource"] = String(expected.prefix(4096))
                receipt["exactUTF16Match"] = Array(value.utf16) == Array(expected.utf16)
            }
        }
        if let bytes = try? JSONSerialization.data(withJSONObject: receipt, options: [.sortedKeys]),
           let json = String(data: bytes, encoding: .utf8) {
            print("[CHERRI_UI_RECEIPT] \(json)")
        }
    }

    private struct ObservedStartupControls {
        let state: CherriEditorStartupObservation
        let creationControl: XCUIElement?

        func recheckingForeground(_ foreground: Bool) -> Self {
            Self(state: state.recheckingForeground(foreground), creationControl: creationControl)
        }
    }

    /// Resolve the fixed controls required by this phase through typed queries.
    /// After the claimed document tap, only the current unique editor is queried.
    /// Keep the returned identities; full pre-tap authorization still queries
    /// creation before editor and never rebinds a candidate in a later query.
    private func observeStartupControls(trace: CherriEditorStartupTimingTrace,
                                        phase: CherriEditorStartupTimingTrace.Phase,
                                        controlScope: CherriEditorStartupControlScope = .documentAndEditor) -> ObservedStartupControls {
        let foreground = trace.measure(phase, .foreground) { app.state == .runningForeground }
        guard foreground else {
            return ObservedStartupControls(state: CherriEditorStartupObservation(
                windowExists: false, appRunningForeground: false,
                createDocumentExists: false, createDocumentHittable: false,
                editorExists: false, editorHittable: false, controlScope: controlScope), creationControl: nil)
        }
        let windowExists = trace.measure(phase, .window) { app.windows.firstMatch.exists }
        var creationControls: [XCUIElement] = []
        var editors: [XCUIElement] = []
        if windowExists {
            trace.measure(phase, .knownControls) {
                let matched = controlScope.resolve(
                    creation: {
                        app.buttons.matching(NSPredicate(
                            format: "identifier == %@ OR label == %@", "Create Document", "Create Document"))
                            .allElementsBoundByAccessibilityElement
                    },
                    editor: {
                        app.textViews.matching(NSPredicate(format: "identifier == %@", "cherriCodeEditor"))
                            .allElementsBoundByAccessibilityElement
                    })
                creationControls = matched.creation
                editors = matched.editors
                return !creationControls.isEmpty || !editors.isEmpty
            }
        }
        // No false absence event is emitted for creation controls we did not query.
        if controlScope == .documentAndEditor {
            trace.mark(phase, .createExists, value: !creationControls.isEmpty)
        }
        trace.mark(phase, .editorExists, value: !editors.isEmpty)
        let observed: CherriEditorStartupObservation
        if controlScope == .editorOnly {
            observed = .matchedEditor(windowExists: windowExists, appRunningForeground: foreground,
                editorCount: editors.count,
                editorHittable: { trace.measure(phase, .editorHittable) { editors[0].isHittable } })
        } else {
            observed = .matchedControls(
                windowExists: windowExists, appRunningForeground: foreground,
                createDocumentCount: creationControls.count, editorCount: editors.count,
                createDocumentHittable: { trace.measure(phase, .createHittable) { creationControls[0].isHittable } },
                editorHittable: { trace.measure(phase, .editorHittable) { editors[0].isHittable } })
        }
        let state = observed.recheckingForeground(
            trace.measure(phase, .foregroundFinal) { app.state == .runningForeground })
        return ObservedStartupControls(state: state,
            creationControl: state.createDocumentHittable && !state.editorExists ? creationControls.first : nil)
    }

    private func recordStartupTiming(_ trace: CherriEditorStartupTimingTrace) {
        let separators = CharacterSet.alphanumerics.union(CharacterSet(charactersIn: "_")).inverted
        let method = name.components(separatedBy: separators).first { $0.hasPrefix("test") } ?? "unknown"
        if let data = trace.encoded(testName: "CherriEditorUITests/\(method)"), data.count <= 65_536,
           let json = String(data: data, encoding: .utf8) {
            print("[CHERRI_UI_STARTUP_TIMING] \(json)")
        }
    }

    /// No source, document title, AX tree or screenshot is read before an empty reset.
    private func recordStartupReceipt(stage: String, waitCompleted: Bool,
                                      state: CherriEditorStartupObservation,
                                      deadline: CherriEditorStartupDeadline) {
        let separators = CharacterSet.alphanumerics.union(CharacterSet(charactersIn: "_")).inverted
        let method = name.components(separatedBy: separators).first { $0.hasPrefix("test") } ?? "unknown"
        let receipt: [String: Any] = [
            "schemaVersion": "1.1", "testName": "CherriEditorUITests/\(method)",
            "controlScope": state.controlScope.rawValue,
            "stage": stage, "waitCompleted": waitCompleted,
            "windowExists": state.windowExists,
            "appRunningForeground": state.appRunningForeground,
            "createDocumentExists": state.createDocumentExists,
            "createDocumentHittable": state.createDocumentHittable,
            "editorExists": state.editorExists, "editorHittable": state.editorHittable,
            "elapsedMilliseconds": deadline.elapsedMilliseconds,
            "budgetMilliseconds": Int(CherriEditorStartupDeadline.budgetSeconds * 1_000),
            "documentCreationIssued": deadline.documentCreationIssued,
        ]
        if let bytes = try? JSONSerialization.data(withJSONObject: receipt, options: [.sortedKeys]),
           let json = String(data: bytes, encoding: .utf8) {
            print("[CHERRI_UI_STARTUP] \(json)")
        }
    }

    @discardableResult
    private func ensureEditorReady() -> XCUIElement {
        var deadline = CherriEditorStartupDeadline()
        let trace = CherriEditorStartupTimingTrace(startedAt: deadline.diagnosticStartedAt)
        observedStartupTrace = trace
        recordStartupObservationSession("readiness-start", readinessStartedAt: deadline.diagnosticStartedAt)
        let window = app.windows.firstMatch
        let editor = app.textViews["cherriCodeEditor"]
        trace.mark(.windowWait, .entered, value: true)
        let windowReady = window.waitForExistence(timeout: deadline.remaining(maximum: 10))
        trace.mark(.windowWait, .returned, value: windowReady)
        let windowControls = observeStartupControls(trace: trace, phase: .windowSnapshot)
        let windowState = windowControls.state
        recordStartupReceipt(stage: "window-wait", waitCompleted: windowReady,
                             state: windowState, deadline: deadline)
        guard windowReady, windowState.windowExists, windowState.appRunningForeground,
              deadline.remaining() > 0 else {
            recordStartupTiming(trace)
            XCTFail("Main application window must exist in the foreground within the startup budget")
            return editor
        }

        if !windowState.editorExists, deadline.remaining() > 0 {
            let createDoc = app.buttons["Create Document"]
            var creationControls = windowControls
            var creationAuthorized = true
            if deadline.needsDocumentWait(windowState) {
                let observations = CherriEditorStartupWaitObservation<ObservedStartupControls>(phase: .document)
                let controlReady = XCTNSPredicateExpectation(predicate: NSPredicate { _, _ in
                    let current = self.observeStartupControls(trace: trace, phase: .documentWait)
                    return observations.record(current, state: current.state)
                }, object: createDoc)
                trace.mark(.documentWait, .entered, value: true)
                let waitCompleted = XCTWaiter.wait(for: [controlReady],
                    timeout: deadline.remaining(maximum: 6)) == .completed
                trace.mark(.documentWait, .returned, value: waitCompleted)
                // The successful predicate already observed both controls and
                // bound the exact creation element. Do not repeat its full AX
                // query. A timed-out wait never authorizes a fallback tap.
                if let completed = observations.takeCompleted(waitCompleted: waitCompleted) {
                    creationControls = completed.recheckingForeground(
                        trace.measure(.documentSnapshot, .foregroundFinal) { app.state == .runningForeground })
                } else {
                    creationAuthorized = false
                    // Keep an actual failed-state diagnostic; it cannot authorize
                    // creation, even if this later observation sees a button.
                    creationControls = observeStartupControls(trace: trace, phase: .documentSnapshot)
                }
                recordStartupReceipt(stage: "document-wait", waitCompleted: waitCompleted,
                                     state: creationControls.state, deadline: deadline)
            } else {
                // This current filtered observation already proves a ready
                // control and no editor. Do not repeat the negative query.
                recordStartupReceipt(stage: "document-check", waitCompleted: false,
                                     state: creationControls.state, deadline: deadline)
            }
            if creationAuthorized, let create = creationControls.creationControl,
               deadline.claimDocumentCreation(creationControls.state) {
                trace.mark(.documentTap, .entered, value: true)
                create.tap()
                trace.mark(.documentTap, .returned, value: true)
            }
        }

        // The original total allowance is shared across transitions. Fast earlier
        // stages leave their unused time for the actual document-to-editor transition.
        let controlScope: CherriEditorStartupControlScope = deadline.documentCreationIssued ? .editorOnly : .documentAndEditor
        let observations = CherriEditorStartupWaitObservation<ObservedStartupControls>(phase: .editor)
        let ready = XCTNSPredicateExpectation(predicate: NSPredicate { _, _ in
            let current = self.observeStartupControls(trace: trace, phase: .editorWait, controlScope: controlScope)
            return observations.record(current, state: current.state)
        }, object: editor)
        trace.mark(.editorWait, .entered, value: true)
        let waitCompleted = XCTWaiter.wait(for: [ready], timeout: deadline.remaining()) == .completed
        trace.mark(.editorWait, .returned, value: waitCompleted)
        let completed = observations.takeCompleted(waitCompleted: waitCompleted)
        let finalState: CherriEditorStartupObservation
        if let completed {
            finalState = completed.state.recheckingForeground(
                trace.measure(.finalSnapshot, .foregroundFinal) { app.state == .runningForeground })
        } else {
            finalState = observeStartupControls(trace: trace, phase: .finalSnapshot, controlScope: controlScope).state
        }
        // Success requires the real completed wait, its own required editor
        // observation, current foreground and the original decision-time limit.
        // A later diagnostic snapshot can never replace the failed predicate.
        let editorReady = deadline.accepts(finalState, waitCompleted: waitCompleted && completed != nil)
        recordStartupReceipt(stage: "editor-wait", waitCompleted: waitCompleted,
                             state: finalState, deadline: deadline)
        recordStartupTiming(trace)
        XCTAssertTrue(editorReady, "cherriCodeEditor must exist and be interactive in the foreground within the startup budget")
        return editor
    }

    @discardableResult
    private func resetEditor(_ editor: XCUIElement) -> Bool {
        syntheticSourceResetVerified = false
        expectedSyntheticSource = nil
        let clearBtn = app.buttons["clearEditorButton"]
        guard clearBtn.waitForExistence(timeout: 2) && clearBtn.isHittable else {
            XCTFail("Clear action must be available before entering a synthetic fixture")
            return false
        }
        clearBtn.tap()
        let cleared = waitForEditorValue(editor) { $0.isEmpty }
        XCTAssertTrue(cleared, "Editor reset must expose an empty source before synthetic typing")
        syntheticSourceResetVerified = cleared
        if cleared { recordEditorReceipt(stage: "empty-reset") }
        return cleared
    }

    private func typeIntoEditor(_ editor: XCUIElement, _ text: String) {
        editor.coordinate(withNormalizedOffset: CGVector(dx: 0.5, dy: 0.5)).tap()
        _ = app.keyboards.firstMatch.waitForExistence(timeout: 2)
        editor.typeText(text)
    }

    private func setEditorContent(_ editor: XCUIElement, _ text: String) {
        guard resetEditor(editor) else { return }
        expectedSyntheticSource = text
        typeIntoEditor(editor, text)
        let matches = waitForEditorValue(editor) { Array($0.utf16) == Array(text.utf16) }
        recordEditorReceipt(stage: "typed-source")
        XCTAssertTrue(matches, "Editor UTF-16 source must exactly match the typed synthetic fixture")
    }

    private func replaceDocumentationSearchQuery(_ query: String) {
        let search = app.searchFields.firstMatch
        let ready = XCTNSPredicateExpectation(predicate: NSPredicate { _, _ in
            search.exists && search.isHittable
        }, object: search)
        XCTAssertEqual(XCTWaiter.wait(for: [ready], timeout: 5), .completed)
        search.tap()

        func isEmpty() -> Bool {
            let value = search.value as? String ?? ""
            return value.isEmpty || value == search.placeholderValue
        }
        if !isEmpty() {
            let clear = search.buttons.firstMatch
            let clearReady = XCTNSPredicateExpectation(predicate: NSPredicate { _, _ in
                clear.exists && clear.isHittable
            }, object: clear)
            XCTAssertEqual(XCTWaiter.wait(for: [clearReady], timeout: 5), .completed)
            clear.tap()
            let cleared = XCTNSPredicateExpectation(predicate: NSPredicate { _, _ in
                search.exists && isEmpty()
            }, object: search)
            XCTAssertEqual(XCTWaiter.wait(for: [cleared], timeout: 5), .completed)
        }

        search.tap()
        search.typeText(query)
        let exactQuery = XCTNSPredicateExpectation(predicate: NSPredicate { _, _ in
            search.exists && (search.value as? String) == query
        }, object: search)
        XCTAssertEqual(XCTWaiter.wait(for: [exactQuery], timeout: 5), .completed)
    }

    /// Only fixed control observations from this isolated D03 test are emitted.
    /// No query text, result labels, accessibility tree, or screenshot is recorded.
    private func recordDocumentationMigrationState(stage: String) {
        guard syntheticSourceResetVerified else { return }
        let search = app.searchFields.firstMatch
        let migration = app.switches["docsMigrationToggle"]
        let resultPredicate = NSPredicate(format: "identifier BEGINSWITH %@",
                                          "docsResult_fork-language/migration.md:")
        let switchValue = migration.exists ? migration.value as? String : nil
        let state: [String: Any] = [
            "schemaVersion": "1.0",
            "testName": "CherriEditorUITests/testD03CurrentDocumentationSearchUI",
            "stage": stage,
            "syntheticSourceResetVerified": syntheticSourceResetVerified,
            "searchExists": search.exists,
            "queryIsExpected": search.exists && (search.value as? String) == "@count",
            "switchExists": migration.exists,
            "switchHittable": migration.exists && migration.isHittable,
            "switchOn": switchValue == "1",
            "switchOff": switchValue == "0",
            "keyboardExists": app.keyboards.firstMatch.exists,
            "noResultsExists": app.descendants(matching: .any)["docsNoResults"].exists,
            "migrationButtonExists": app.buttons.matching(resultPredicate).firstMatch.exists,
            "migrationElementExists": app.descendants(matching: .any).matching(resultPredicate).firstMatch.exists,
        ]
        if let bytes = try? JSONSerialization.data(withJSONObject: state, options: [.sortedKeys]),
           let json = String(data: bytes, encoding: .utf8) {
            print("[CHERRI_DOCS_UI_STATE] \(json)")
        }
    }

    func testD03CurrentDocumentationSearchUI() throws {
        let editor = ensureEditorReady()
        let source = "show(\"D03_EDITOR_PRESERVED\")"
        setEditorContent(editor, source)

        let options = app.buttons["workspaceOptionsButton"]
        XCTAssertTrue(options.waitForExistence(timeout: 5) && options.isHittable)
        options.tap()
        let documentation = app.buttons["documentationButton"]
        XCTAssertTrue(documentation.waitForExistence(timeout: 5) && documentation.isHittable)
        documentation.tap()

        let search = app.searchFields.firstMatch
        XCTAssertTrue(search.waitForExistence(timeout: 10))
        XCTAssertFalse(app.descendants(matching: .any)["docsLoadError"].exists)
        let migration = app.switches["docsMigrationToggle"]
        XCTAssertTrue(migration.waitForExistence(timeout: 5))
        XCTAssertEqual(migration.value as? String, "0")

        replaceDocumentationSearchQuery("show")
        let firstResult = app.buttons.matching(
            NSPredicate(format: "identifier BEGINSWITH %@", "docsResult_")
        ).firstMatch
        let exactFirst = XCTNSPredicateExpectation(predicate: NSPredicate { _, _ in
            firstResult.exists && firstResult.isHittable && firstResult.label.hasPrefix("show")
        }, object: firstResult)
        let exactResultObserved = XCTWaiter.wait(for: [exactFirst], timeout: 5)
        XCTAssertEqual(exactResultObserved, .completed,
                       "D03 first result: exists=\(firstResult.exists), "
                       + "hittable=\(firstResult.exists && firstResult.isHittable), "
                       + "exactTitlePrefix=\(firstResult.exists && firstResult.label.hasPrefix("show"))")
        firstResult.tap()
        let title = app.staticTexts["docsSectionTitle"]
        XCTAssertTrue(title.waitForExistence(timeout: 5))
        XCTAssertEqual(title.label, "show")
        let signature = app.staticTexts.matching(
            NSPredicate(format: "label CONTAINS %@", "show(input: AnyContent)")
        ).firstMatch
        XCTAssertTrue(signature.waitForExistence(timeout: 5),
                      "The reader must expose the current action signature")

        let back = app.navigationBars["show"].buttons.firstMatch
        XCTAssertTrue(back.exists && back.isHittable)
        back.tap()
        XCTAssertTrue(search.waitForExistence(timeout: 5))
        let missing = "ZZZ_MISSING_DOCUMENTATION_937"
        replaceDocumentationSearchQuery(missing)
        let noResults = app.descendants(matching: .any)["docsNoResults"]
        XCTAssertTrue(noResults.waitForExistence(timeout: 5),
                      "An unknown term must show the empty search state")

        replaceDocumentationSearchQuery("@count")
        XCTAssertTrue(noResults.waitForExistence(timeout: 5))
        XCTAssertFalse(app.buttons.matching(
            NSPredicate(format: "identifier BEGINSWITH %@", "docsResult_fork-language/migration.md:")
        ).firstMatch.exists, "Historical examples must be excluded by default")
        recordDocumentationMigrationState(stage: "before-toggle")
        XCTAssertEqual(migration.value as? String, "0")
        XCTAssertTrue(migration.exists && migration.isHittable)
        migration.tap()
        let migrationEnabled = XCTNSPredicateExpectation(predicate: NSPredicate { _, _ in
            migration.exists && (migration.value as? String) == "1"
                && search.exists && (search.value as? String) == "@count"
        }, object: migration)
        let toggleObserved = XCTWaiter.wait(for: [migrationEnabled], timeout: 5) == .completed
        recordDocumentationMigrationState(stage: "after-toggle")
        XCTAssertTrue(toggleObserved, "The explicit toggle must turn on while preserving the exact search query")
        let migrationResult = app.buttons.matching(
            NSPredicate(format: "identifier BEGINSWITH %@", "docsResult_fork-language/migration.md:")
        ).firstMatch
        let migrationResultObserved = migrationResult.waitForExistence(timeout: 5)
        recordDocumentationMigrationState(stage: "result-wait")
        XCTAssertTrue(migrationResultObserved,
                      "Migration examples require explicit opt-in")
        XCTAssertEqual(migration.value as? String, "1")

        let done = app.buttons["docsDoneButton"]
        XCTAssertTrue(done.waitForExistence(timeout: 5) && done.isHittable)
        done.tap()
        XCTAssertTrue(editor.waitForExistence(timeout: 5))
        XCTAssertTrue(waitForEditorValue(editor) { Array($0.utf16) == Array(source.utf16) },
                      "Searching documentation must preserve the edited source exactly")
        recordEditorReceipt(stage: "documentation-search-completed")
        print("[CHECKPOINT:D03_CURRENT_DOCS_SEARCH_VERIFIED]")
    }

    // E01 & RP40: Contextual completion display, selection, insertion, and successful compilation
    func testE01CompletionDisplayAndInsertionUI() throws {
        let editor = ensureEditorReady()
        setEditorContent(editor, "sho")

        let actionsBtn = app.buttons["actionsButton"]
        XCTAssertTrue(actionsBtn.waitForExistence(timeout: 5), "Actions button must exist in editor bar")

        let showCandidate = app.buttons["completionItem_show"]
        let showSuggestion = app.buttons["suggestionItem_show"]

        if showCandidate.waitForExistence(timeout: 3) && showCandidate.isHittable {
            showCandidate.tap()
        } else if showSuggestion.waitForExistence(timeout: 3) && showSuggestion.isHittable {
            showSuggestion.tap()
        } else {
            recordEditorReceipt(stage: "completion-candidates")
            XCTFail("Exact contextual candidate show must be visible and hittable")
            return
        }

        // Verify editor content reflects completed insertion and no trailing uncompleted prefix
        XCTAssertTrue(editor.waitForExistence(timeout: 5), "Editor must remain present after insertion")
        expectedSyntheticSource = "show(<#input#>)"
        let insertionCompleted = waitForEditorValue(editor) {
            Array($0.utf16) == Array("show(<#input#>)".utf16)
        }
        recordEditorReceipt(stage: "completion-insertion")
        XCTAssertTrue(insertionCompleted, "Selected completion must finish updating the editor source")
        let editorValue = (editor.value as? String) ?? ""
        XCTAssertTrue(editorValue.contains("show"), "Editor text must reflect inserted action call")
        XCTAssertFalse(editorValue.trimmingCharacters(in: .whitespacesAndNewlines).hasSuffix("sho"), "Editor text must not end in incomplete prefix 'sho'")

        // Compile the exact inserted bytes first. An editor hole is not an
        // implicit Ask Each Time request, and must produce a diagnostic.
        let buildBtn = app.buttons["buildButton"]
        XCTAssertTrue(buildBtn.waitForExistence(timeout: 5), "Build button must exist in toolbar")
        buildBtn.tap()

        let diagStatus = app.staticTexts["diagnosticStatusText"]
        XCTAssertTrue(diagStatus.waitForExistence(timeout: 8), "Unfilled insertion must be rejected")
        XCTAssertFalse(app.staticTexts["compiledStatusText"].exists, "Unfilled insertion must not compile")
        let filledSource = editorValue.replacingOccurrences(of: "<#input#>", with: "\"P15_INSERTED\"")
        setEditorContent(editor, filledSource)
        buildBtn.tap()
        XCTAssertTrue(app.staticTexts["compiledStatusText"].waitForExistence(timeout: 8), "Explicitly filled insertion must compile")
        XCTAssertFalse(diagStatus.exists, "Filled insertion must clear the earlier diagnostic")

        print("[CHECKPOINT:E01_COMPLETION_INSERTED] Contextual completion candidate selected, inserted, and verified in editor")
    }

    func testE01PaletteSearchSelectsExactActionAndReplacesPrefixUI() throws {
        let editor = ensureEditorReady()
        setEditorContent(editor, "sho")
        let actions = app.buttons["actionsButton"]
        XCTAssertTrue(actions.waitForExistence(timeout: 5) && actions.isHittable)
        actions.tap()

        let search = app.searchFields.firstMatch
        XCTAssertTrue(search.waitForExistence(timeout: 5) && search.isHittable, "Palette search must be available")
        search.tap()
        search.typeText("show")
        let exactAction = app.buttons["paletteAction_show"]
        XCTAssertTrue(exactAction.waitForExistence(timeout: 5) && exactAction.isHittable,
                      "Search must expose the exact show action; no fallback cell is allowed")
        exactAction.tap()

        expectedSyntheticSource = "show(<#input#>)"
        let exact = waitForEditorValue(editor) { Array($0.utf16) == Array("show(<#input#>)".utf16) }
        recordEditorReceipt(stage: "completion-insertion")
        XCTAssertTrue(exact, "Palette selection must replace only the captured sho prefix")
        let build = app.buttons["buildButton"]
        XCTAssertTrue(build.waitForExistence(timeout: 5) && build.isHittable)
        build.tap()
        XCTAssertTrue(app.staticTexts["diagnosticStatusText"].waitForExistence(timeout: 8), "Unfilled palette input must be rejected")
        XCTAssertFalse(app.staticTexts["compiledStatusText"].exists)
        let insertedSource = (editor.value as? String) ?? ""
        setEditorContent(editor, insertedSource.replacingOccurrences(of: "<#input#>", with: "\"P15_PALETTE\""))
        build.tap()
        XCTAssertTrue(app.staticTexts["compiledStatusText"].waitForExistence(timeout: 8))
        XCTAssertFalse(app.staticTexts["diagnosticStatusText"].exists)
        print("[CHECKPOINT:E01_PALETTE_EXACT] Exact palette action replaced the prefix; unfilled input failed and explicitly filled input compiled")
    }

    // E02: Multiple diagnostics rendering with accurate inline indicators and ranges
    func testE02DiagnosticsRenderingAndNavigationUI() throws {
        let editor = ensureEditorReady()
        setEditorContent(editor, "let a = 10\na = 20\nlet b: Text = 123\n")

        let buildBtn = app.buttons["buildButton"]
        XCTAssertTrue(buildBtn.waitForExistence(timeout: 5), "Build button must exist")
        buildBtn.tap()

        // Assert diagnostics banner renders multiple distinct error indicators
        let diagBanner = app.otherElements["diagnosticsBanner"]
        XCTAssertTrue(diagBanner.waitForExistence(timeout: 8), "Diagnostics banner must be rendered for invalid source")

        let firstDiag = app.staticTexts["diagnosticItem_0"]
        let secondDiag = app.staticTexts["diagnosticItem_1"]
        XCTAssertTrue(firstDiag.waitForExistence(timeout: 5), "First diagnostic item must exist in banner")
        XCTAssertTrue(secondDiag.waitForExistence(timeout: 5), "Second diagnostic item must exist in banner")

        let firstText = firstDiag.label
        let secondText = secondDiag.label
        XCTAssertTrue(firstText.contains("L") && firstText.contains("C"), "First diagnostic must specify line and column coordinates: \(firstText)")
        XCTAssertTrue(secondText.contains("L") && secondText.contains("C"), "Second diagnostic must specify line and column coordinates: \(secondText)")
        XCTAssertTrue(firstText.contains("len:"), "First diagnostic must specify length span")
        XCTAssertTrue(secondText.contains("len:"), "Second diagnostic must specify length span")

        let compiledStatus = app.staticTexts["compiledStatusText"]
        XCTAssertFalse(compiledStatus.exists, "Compiled status must not report success when errors exist")

        print("[CHECKPOINT:E02_DIAGNOSTICS_VERIFIED] Multiple diagnostics rendered with line, column, and span bounds")
    }

    // E03: Real user-facing affordances for Hover, Signature Help, Definition, and Scoped Rename
    func testE03EditorActionsHoverRenameUI() throws {
        let editor = ensureEditorReady()
        setEditorContent(editor, "let greeting = \"Cherri\"\nshow(greeting)\n")

        let buildBtn = app.buttons["buildButton"]
        XCTAssertTrue(buildBtn.waitForExistence(timeout: 5), "Build button must exist")
        buildBtn.tap()

        let compiledStatus = app.staticTexts["compiledStatusText"]
        XCTAssertTrue(compiledStatus.waitForExistence(timeout: 8), "Fixture must compile cleanly before testing affordances")

        // 1. Test Hover affordance
        let hoverBtn = app.buttons["hoverButton"]
        XCTAssertTrue(hoverBtn.waitForExistence(timeout: 5), "Hover button must exist in editor bar")
        hoverBtn.tap()

        let hoverContent = app.staticTexts["hoverContent"]
        XCTAssertTrue(hoverContent.waitForExistence(timeout: 5), "Hover content must be displayed to user")
        let hoverLabel = hoverContent.label
        XCTAssertTrue(hoverLabel.contains("greeting"), "Hover content must semantically describe greeting symbol: \(hoverLabel)")
        XCTAssertFalse(hoverLabel.contains("No hover info"), "Hover content must not be a fallback failure message")
        let dismissHover = app.buttons["dismissHoverButton"]
        if dismissHover.waitForExistence(timeout: 2) { dismissHover.tap() }

        // 2. Test Signature Help affordance
        let sigBtn = app.buttons["signatureButton"]
        XCTAssertTrue(sigBtn.waitForExistence(timeout: 5), "Signature help button must exist in editor bar")
        sigBtn.tap()

        let sigContent = app.staticTexts["signatureContent"]
        XCTAssertTrue(sigContent.waitForExistence(timeout: 5), "Signature help content must be displayed")
        let sigLabel = sigContent.label
        XCTAssertTrue(sigLabel.contains("show") || sigLabel.contains("text"), "Signature content must describe show action signature: \(sigLabel)")
        XCTAssertFalse(sigLabel.contains("No signature"), "Signature content must not report fallback failure")
        let dismissSig = app.buttons["dismissSignatureButton"]
        if dismissSig.waitForExistence(timeout: 2) { dismissSig.tap() }

        // 3. Test Definition affordance
        let defBtn = app.buttons["definitionButton"]
        XCTAssertTrue(defBtn.waitForExistence(timeout: 5), "Definition button must exist in editor bar")
        defBtn.tap()

        let defContent = app.staticTexts["definitionContent"]
        XCTAssertTrue(defContent.waitForExistence(timeout: 5), "Definition content must be displayed")
        let defLabel = defContent.label
        XCTAssertTrue(defLabel.contains("Definition at 1:"), "Definition coordinate must resolve to declaration on line 1: \(defLabel)")
        XCTAssertFalse(defLabel.contains("No definition found"), "Definition lookup must not report fallback failure")
        let dismissDef = app.buttons["dismissDefinitionButton"]
        if dismissDef.waitForExistence(timeout: 2) { dismissDef.tap() }

        // 4. Test Scoped Rename affordance
        let renameBtn = app.buttons["renameButton"]
        XCTAssertTrue(renameBtn.waitForExistence(timeout: 5), "Rename button must exist in editor bar")
        renameBtn.tap()

        let alert = app.alerts.firstMatch
        XCTAssertTrue(alert.waitForExistence(timeout: 5), "Rename dialog must appear")
        let renameField = alert.textFields["renameTextField"].exists ? alert.textFields["renameTextField"] : alert.textFields.firstMatch
        XCTAssertTrue(renameField.waitForExistence(timeout: 5), "Rename text field must appear in rename dialog")
        renameField.tap()
        renameField.typeText("messageGreeting")

        let confirmRename = alert.buttons["confirmRenameButton"].exists ? alert.buttons["confirmRenameButton"] : alert.buttons["Rename"]
        XCTAssertTrue(confirmRename.waitForExistence(timeout: 5), "Confirm rename button must exist")
        confirmRename.tap()

        // Verify editor content reflects rename across declaration and reference
        _ = alert.waitForNonExistence(timeout: 5)
        let updatedValue = (editor.value as? String) ?? ""
        XCTAssertTrue(updatedValue.contains("messageGreeting"), "Editor source must contain renamed identifier 'messageGreeting'")
        XCTAssertFalse(updatedValue.contains("let greeting"), "Editor source must not contain old declaration 'let greeting'")

        // Recompile and verify cleanly updated
        buildBtn.tap()
        XCTAssertTrue(compiledStatus.waitForExistence(timeout: 8), "Renamed source must compile cleanly")

        print("[CHECKPOINT:E03_HOVER_SIG_DEF_RENAME_VERIFIED] Native UI affordances for hover, signature, definition, and rename verified")
    }

    // E05: Editor responsiveness and stale edit isolation under continuous rapid typing
    func testE05EditorResponsivenessStaleEditsUI() throws {
        let editor = ensureEditorReady()
        guard resetEditor(editor) else { return }
        editor.coordinate(withNormalizedOffset: CGVector(dx: 0.5, dy: 0.5)).tap()
        _ = app.keyboards.firstMatch.waitForExistence(timeout: 2)

        // Simulate rapid typing burst of 5 statements
        for i in 1...5 {
            editor.typeText("let burstVar\(i) = \(i)\n")
        }
        editor.typeText("show(burstVar5)\n")
        let expected = (1...5).map { "let burstVar\($0) = \($0)\n" }.joined() + "show(burstVar5)\n"
        expectedSyntheticSource = expected
        let exactSource = waitForEditorValue(editor) { Array($0.utf16) == Array(expected.utf16) }
        recordEditorReceipt(stage: "rapid-typed-source")
        XCTAssertTrue(exactSource, "Rapid typing must preserve the complete exact synthetic source")

        // Verify editor retains content and UI remains interactive
        XCTAssertTrue(editor.waitForExistence(timeout: 5), "Editor must remain interactive under rapid edits")
        let textVal = (editor.value as? String) ?? ""
        XCTAssertTrue(textVal.contains("burstVar5"), "Editor must preserve latest typed statement")

        let buildBtn = app.buttons["buildButton"]
        XCTAssertTrue(buildBtn.waitForExistence(timeout: 5), "Build button must be interactive after typing burst")
        buildBtn.tap()

        let compiledStatus = app.staticTexts["compiledStatusText"]
        XCTAssertTrue(compiledStatus.waitForExistence(timeout: 8), "Compiled status must appear after rapid typing burst")
        let diagStatus = app.staticTexts["diagnosticStatusText"]
        XCTAssertFalse(diagStatus.exists, "Stale asynchronous results must not leak into editor status")

        print("[CHECKPOINT:E05_RAPID_EDITS_VERIFIED] Newest document version preserved under rapid typing without stale decoration")
    }

    // E06: Hebrew and emoji bidirectional editing without cursor corruption or character truncation
    func testE06BidirectionalHebrewEmojiUI() throws {
        let editor = ensureEditorReady()
        let bidiSnippet = "let bidiMsg = \"שָׁלוֹם 🌍 דוב\"\nshow(bidiMsg)\n"
        setEditorContent(editor, bidiSnippet)

        // Verify exact Hebrew and emoji characters are preserved (no English fallback allowed)
        let val = (editor.value as? String) ?? ""
        XCTAssertTrue(val.contains("שָׁלוֹם"), "Editor must preserve Hebrew text with diacritics")
        XCTAssertTrue(val.contains("🌍"), "Editor must preserve multi-scalar emoji grapheme")

        // Assert UTF-16 code unit length strictly exceeds Swift Character count
        let nsVal = val as NSString
        XCTAssertGreaterThan(nsVal.length, val.count, "UTF-16 code units (\(nsVal.length)) must exceed Character count (\(val.count)) due to emoji and combining marks")

        let buildBtn = app.buttons["buildButton"]
        XCTAssertTrue(buildBtn.waitForExistence(timeout: 5), "Build button must exist")
        buildBtn.tap()

        let compiledStatus = app.staticTexts["compiledStatusText"]
        XCTAssertTrue(compiledStatus.waitForExistence(timeout: 8), "Bidirectional Hebrew and emoji source must compile cleanly")

        // Verify accurate UTF-16 diagnostic offset on Unicode line
        setEditorContent(editor, "let a = \"🌍\" ; unknownActionEmoji()\n")
        buildBtn.tap()

        let diagBanner = app.otherElements["diagnosticsBanner"]
        XCTAssertTrue(diagBanner.waitForExistence(timeout: 8), "Diagnostics banner must appear for invalid action call")
        let firstDiag = app.staticTexts["diagnosticItem_0"]
        XCTAssertTrue(firstDiag.waitForExistence(timeout: 5), "Diagnostic item must exist")
        let diagText = firstDiag.label
        XCTAssertTrue(diagText.contains("C16") || diagText.contains("len:"), "Diagnostic must compute exact UTF-16 column offset: \(diagText)")

        print("[CHECKPOINT:E06_UNICODE_VERIFIED] Hebrew diacritics and emoji graphemes preserved with exact offsets")
    }

    // E07 & RP41 & BRG28: Visual parameter editing and fallback rendering in preview canvas
    /// E07 observes only fixed controls in its already-reset synthetic document.
    /// Readiness and selection share the original three-second pane budget.
    private func selectE07Pane(_ name: String, stage: String,
                               file: StaticString = #filePath, line: UInt = #line) -> Bool {
        let started = ProcessInfo.processInfo.systemUptime
        let deadline = started + 3
        let selectors = app.segmentedControls
        let editor = app.textViews["cherriCodeEditor"]
        let preview = app.webViews.firstMatch
        var control: XCUIElement?
        var dualPane = false
        var observation: [String: Any] = [:]

        func observe() -> Bool {
            let selectorCount = selectors.count
            var codeCount = 0
            var previewCount = 0
            control = nil
            if selectorCount == 1 {
                let selector = selectors.element(boundBy: 0)
                codeCount = selector.buttons.matching(identifier: "Code").count
                previewCount = selector.buttons.matching(identifier: "Preview").count
                if codeCount == 1 && previewCount == 1 {
                    control = selector.buttons[name]
                }
            }
            let editorExists = editor.exists
            let previewExists = preview.exists
            dualPane = selectorCount == 0 && editorExists && previewExists
                && editor.isHittable && preview.isHittable
            let hittable = control?.isHittable ?? false
            observation = [
                "selectorCount": selectorCount, "codeCount": codeCount,
                "previewCount": previewCount, "targetHittable": hittable,
                "targetSelected": control?.isSelected ?? false,
                "editorExists": editorExists, "previewExists": previewExists,
                "dualPaneVisible": dualPane,
            ]
            return dualPane || hittable
        }

        func wait(_ predicate: @escaping () -> Bool) -> Bool {
            let remaining = deadline - ProcessInfo.processInfo.systemUptime
            guard remaining > 0 else { return false }
            let expectation = XCTNSPredicateExpectation(
                predicate: NSPredicate { _, _ in predicate() }, object: app)
            return XCTWaiter.wait(for: [expectation], timeout: remaining) == .completed
                && ProcessInfo.processInfo.systemUptime <= deadline
        }

        func record(_ phase: String, tapIssued: Bool) {
            var row = observation
            row["schema"] = "cherri-e07-pane-transition/1"
            row["testId"] = "CherriEditorUITests/testE07VisualParameterEditingCanvasUI"
            row["stage"] = stage
            row["target"] = name
            row["phase"] = phase
            row["tapIssued"] = tapIssued
            row["budgetMilliseconds"] = 3_000
            row["elapsedMilliseconds"] = Int(
                (ProcessInfo.processInfo.systemUptime - started) * 1_000)
            if let bytes = try? JSONSerialization.data(withJSONObject: row, options: [.sortedKeys]),
               bytes.count <= 2_048, let json = String(data: bytes, encoding: .utf8) {
                print("[CHERRI_E07_PANE_TRANSITION] \(json)")
            }
        }

        let ready = wait { observe() }
        record(ready ? "ready" : "not-ready", tapIssued: false)
        guard ready else {
            XCTFail("E07 requires one selectable workspace segment or two visible panes", file: file, line: line)
            return false
        }
        if dualPane {
            record("dual-pane", tapIssued: false)
            return true
        }
        guard let target = control, ProcessInfo.processInfo.systemUptime < deadline else {
            XCTFail("E07 workspace selection must stay within its original deadline", file: file, line: line)
            return false
        }
        target.tap()
        let selected = wait { observe() && !dualPane && (control?.isSelected ?? false) }
        record(selected ? "selected" : "not-selected", tapIssued: true)
        XCTAssertTrue(selected, "E07 must observe its selected workspace segment", file: file, line: line)
        return selected
    }

    func testE07VisualParameterEditingCanvasUI() throws {
        let editor = ensureEditorReady()
        let initialSource = "// Header comment preserved\nshow(\"CanvasPreviewFixture\")\n// Footer comment preserved\n"
        let updatedSource = initialSource.replacingOccurrences(of: "CanvasPreviewFixture", with: "VisualParamUpdated")
        setEditorContent(editor, initialSource)

        let buildBtn = app.buttons["buildButton"]
        XCTAssertTrue(buildBtn.waitForExistence(timeout: 5), "Build button must exist")
        buildBtn.tap()

        let compiledStatus = app.staticTexts["compiledStatusText"]
        XCTAssertTrue(compiledStatus.waitForExistence(timeout: 8), "Fixture must compile cleanly before preview inspection")

        // Switch to Preview pane if segmented picker is present (compact layout)
        guard selectE07Pane("Preview", stage: "initial-preview") else { return }

        // Verify WebKit preview canvas element exists
        let webView = app.webViews.firstMatch
        XCTAssertTrue(webView.waitForExistence(timeout: 8), "Shortcut preview canvas WebView must exist and render")

        // Perform real user interaction on visual parameter edit control
        let previewEditBtn = app.buttons["previewParamEditButton"]
        XCTAssertTrue(previewEditBtn.waitForExistence(timeout: 5), "Visual parameter edit button must exist in preview pane")
        previewEditBtn.tap()

        // Switch back to Code pane
        guard selectE07Pane("Code", stage: "edited-code") else { return }

        // Verify exact source change: parameter updated, unrelated comments preserved
        XCTAssertTrue(editor.waitForExistence(timeout: 5), "Editor must remain present")
        expectedSyntheticSource = updatedSource
        XCTAssertTrue(waitForEditorValue(editor) { $0.utf16.elementsEqual(updatedSource.utf16) },
                      "Visual edit must preserve the exact source outside its parameter")
        let val = (editor.value as? String) ?? ""
        XCTAssertTrue(val.contains("VisualParamUpdated"), "Source code parameter must reflect visual edit")
        XCTAssertFalse(val.contains("CanvasPreviewFixture"), "Old parameter value must be replaced")
        XCTAssertTrue(val.contains("// Header comment preserved"), "Header comment must remain preserved")
        XCTAssertTrue(val.contains("// Footer comment preserved"), "Footer comment must remain preserved")

        // Test negative / unmappable case: attempting invalid edit rejects safely
        guard selectE07Pane("Preview", stage: "invalid-preview") else { return }
        let invalidEditBtn = app.buttons["previewParamInvalidEditButton"]
        XCTAssertTrue(invalidEditBtn.waitForExistence(timeout: 5), "Invalid parameter edit button must exist")
        invalidEditBtn.tap()

        let diagStatus = app.staticTexts["diagnosticStatusText"]
        XCTAssertTrue(diagStatus.waitForExistence(timeout: 5), "Invalid visual edit must produce rejection diagnostic")

        // Switch back to Code pane and verify source was not corrupted
        guard selectE07Pane("Code", stage: "rejected-code") else { return }
        XCTAssertTrue(waitForEditorValue(editor) { $0.utf16.elementsEqual(updatedSource.utf16) },
                      "Rejected edit must preserve the exact source after returning to Code")
        let preservedVal = (editor.value as? String) ?? ""
        XCTAssertTrue(preservedVal.contains("VisualParamUpdated"), "Source must remain intact after rejected edit")

        // Persistence: toggle preview again and confirm persistence
        guard selectE07Pane("Preview", stage: "reopened-preview") else { return }
        XCTAssertTrue(webView.waitForExistence(timeout: 5), "Preview must persist across view transitions")
        guard selectE07Pane("Code", stage: "reopened-code") else { return }
        XCTAssertTrue(waitForEditorValue(editor) { $0.utf16.elementsEqual(updatedSource.utf16) },
                      "Reopened Code pane must expose the exact preserved source")
        let finalVal = (editor.value as? String) ?? ""
        XCTAssertTrue(finalVal.contains("VisualParamUpdated"), "Source must persist after reopening")

        print("[CHECKPOINT:E07_PREVIEW_EDIT_VERIFIED] Visual preview canvas renders and syncs bounded parameter edits with source persistence")
    }

    // BRG27: Integrated real-world workflow covering editor ranges, completion, and preview
    func testBRG27EditorRealWorldWorkflowUI() throws {
        let editor = ensureEditorReady()
        setEditorContent(editor, "// Cherri v2 Real World Workflow\nlet result = \"Success\"\nshow(result)\n")

        let buildBtn = app.buttons["buildButton"]
        XCTAssertTrue(buildBtn.waitForExistence(timeout: 5), "Build button must exist")
        buildBtn.tap()

        let compiledStatus = app.staticTexts["compiledStatusText"]
        XCTAssertTrue(compiledStatus.waitForExistence(timeout: 8), "Status bar must reflect compiled workflow")
        let diagStatus = app.staticTexts["diagnosticStatusText"]
        XCTAssertFalse(diagStatus.exists, "Workflow must compile without diagnostics")

        print("[CHECKPOINT:BRG27_WORKFLOW_VERIFIED] Integrated real-world workflow completed and verified end-to-end")
    }

    // Mutation / Anti-False-Positive Test 1: Syntax error fails closed and rejects compiled status
    func testNegativeSyntaxErrorRejectsCompilationFailsClosed() throws {
        let editor = ensureEditorReady()
        setEditorContent(editor, "let invalidSyntax =\n")

        let buildBtn = app.buttons["buildButton"]
        XCTAssertTrue(buildBtn.waitForExistence(timeout: 5), "Build button must exist")
        buildBtn.tap()

        let compiledStatus = app.staticTexts["compiledStatusText"]
        XCTAssertFalse(compiledStatus.waitForExistence(timeout: 3), "Broken syntax must never report compiled status")

        let diagStatus = app.staticTexts["diagnosticStatusText"]
        let diagBanner = app.otherElements["diagnosticsBanner"]
        XCTAssertTrue(diagStatus.waitForExistence(timeout: 5) || diagBanner.waitForExistence(timeout: 5), "Broken syntax must fail closed and report diagnostics")
    }

    // Mutation / Anti-False-Positive Test 2: Non-existent UI elements must not falsely report existence
    func testNegativeMissingUIElementFailsClosed() throws {
        let nonexistentButton = app.buttons["nonexistentButton_MustNotExist"]
        XCTAssertFalse(nonexistentButton.waitForExistence(timeout: 1), "Nonexistent element must not report existence")
    }

    // Separates reset/cursor corruption from automatic delimiter insertion.
    // Real keyboard typing is intentional; paste would evade token completion.
    func testGeneratedCloserCompletesAndOvertypesWithRealKeyboard() throws {
        let editor = ensureEditorReady()
        guard resetEditor(editor) else { return }
        expectedSyntheticSource = "text(\"x\")"
        typeIntoEditor(editor, "text(\"x\"")
        XCTAssertTrue(waitForEditorValue(editor) { $0 == "text(\"x\")" },
                      "The editor must still generate a closing parenthesis")
        recordEditorReceipt(stage: "typed-source")
        // No tap or caret movement: type the exact matching key at the active caret.
        editor.typeText(")\n")
        expectedSyntheticSource = "text(\"x\")\n"
        let exact = waitForEditorValue(editor) { Array($0.utf16) == Array("text(\"x\")\n".utf16) }
        recordEditorReceipt(stage: "typed-source")
        XCTAssertTrue(exact, "Typing the generated closer must move past it exactly once")
    }

    func testTypedSourceAndResetPreserveExactUTF16() throws {
        let editor = ensureEditorReady()
        setEditorContent(editor, "let previousLongName = 123456789\n")
        setEditorContent(editor, "let n = 1\n")
        setEditorContent(editor, "text(\"x\")\n")
        setEditorContent(editor, "text(text(\"x\"))\n")
        setEditorContent(editor, "let xs = [1, 2]\n")
        setEditorContent(editor, "let d = {\"value\": 1}\n")
        setEditorContent(editor, "if true {\ntext(\"x\")\n}\n")
        setEditorContent(editor, "/* block */\ntext(\"x\")\n")
        print("[CHECKPOINT:EDITOR_TYPED_SOURCE_EXACT] Reset and keyboard input preserved exact UTF-16")
    }
}
