import Foundation
import XCTest
@testable import Cherri

final class CherriCoreIntegrationTests: XCTestCase {
    func testRP52DefaultRejectsLegacyAndKeepsExplicitReference() async throws {
        let legacyBody = "const route = text(\"RP52_LEGACY_REFERENCE\")\nshow(route)\n"
        for prefix in ["", "// cherri: legacy\n", "#define from sharesheet\n"] {
            let source = prefix + legacyBody
            do {
                _ = try await CherriCompiler.compile(source: source, name: "RP52 Default")
                XCTFail("Default source entry silently compiled Legacy input")
            } catch let diagnostic as CompilationDiagnostic {
                XCTAssertTrue(diagnostic.message.contains("E_LEGACY_SYNTAX"), diagnostic.message)
            }
            let explicit = try await CherriCompiler.compile(source: source, name: "RP52 Explicit Reference", mode: "legacy")
            let actions = try XCTUnwrap(try propertyList(explicit.plist)["WFWorkflowActions"] as? [[String: Any]])
            let textRows = actions.filter { action in
                let parameters = action["WFWorkflowActionParameters"] as? [String: Any]
                return action["WFWorkflowActionIdentifier"] as? String == "is.workflow.actions.gettext"
                    && parameters?["WFTextActionText"] as? String == "RP52_LEGACY_REFERENCE"
            }
            XCTAssertEqual(textRows.count, 1)
        }
        let recovery = try await CherriCompiler.compile(source: "let route = text(\"RP52_V2_RECOVERY\")\nshow(route)\n", name: "RP52 Recovery")
        XCTAssertFalse(recovery.plist.isEmpty)
    }

    func testCompileProducesShortcutPlist() async throws {
        let result = try await CherriCompiler.compile(
            source: "show(\"Hello from iOS tests\")\n",
            name: "Compile Test"
        )

        let plist = try propertyList(result.plist)
        let actions = try XCTUnwrap(plist["WFWorkflowActions"] as? [[String: Any]])
        XCTAssertFalse(actions.isEmpty)
    }

    func testRepeatedCompileResetsIncludedActions() async throws {
        let source = "#include 'actions/text'\nshow(\"Repeated compile\")\n"

        let first = try await CherriCompiler.compile(source: source, name: "First", mode: "legacy")
        let second = try await CherriCompiler.compile(source: source, name: "Second", mode: "legacy")

        let firstActions = try XCTUnwrap(try propertyList(first.plist)["WFWorkflowActions"] as? [[String: Any]])
        let secondActions = try XCTUnwrap(try propertyList(second.plist)["WFWorkflowActions"] as? [[String: Any]])
        XCTAssertEqual(firstActions.count, secondActions.count)
        XCTAssertFalse(secondActions.isEmpty)
    }

    func testSimpleCompileDecompileRoundTrip() async throws {
        let original = try await CherriCompiler.compile(
            source: "show(\"Round trip\")\n",
            name: "Round Trip"
        )
        let decompiled = try await CherriCompiler.decompile(plist: original.plist, name: "Round Trip")
        XCTAssertFalse(decompiled.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty)

        let rebuilt = try await CherriCompiler.compile(source: decompiled, name: "Round Trip Rebuilt", mode: "legacy")
        let rebuiltActions = try XCTUnwrap(try propertyList(rebuilt.plist)["WFWorkflowActions"] as? [[String: Any]])
        XCTAssertFalse(rebuiltActions.isEmpty)
    }

    func testActionCatalogUsesCompilerDefinitions() async throws {
        _ = try await CherriCompiler.compile(
            source: "show(\"Catalog\")\n",
            name: "Catalog Test"
        )

        let actions = try await CherriCompiler.actionCatalog()
        let showAction = try XCTUnwrap(actions.first(where: { $0.name == "show" }))
        XCTAssertFalse(showAction.signature.isEmpty)
        XCTAssertEqual(showAction.shortcutIdentifier, "is.workflow.actions.showresult")
    }

    private func assertUnfilledSnippetRejected(_ source: String) async throws {
        do {
            _ = try await CherriCompiler.compile(source: source, name: "Unfilled Palette")
            XCTFail("An unfilled editor hole must not become an Ask Each Time prompt")
        } catch is CompilationDiagnostic {
            // The expected parser diagnostic proves no native program was returned.
        }
    }

    func testPaletteSnippetSuppliesRequiredArguments() async throws {
        _ = try await CherriCompiler.compile(
            source: "show(\"Initialize catalog\")\n",
            name: "Snippet Test"
        )
        let actions = try await CherriCompiler.actionCatalog()
        let show = try XCTUnwrap(actions.first(where: { $0.name == "show" }))
        XCTAssertEqual(show.insertionSnippet, "show(<#input#>)")
        try await assertUnfilledSnippetRejected(show.insertionSnippet)
        _ = try await CherriCompiler.compile(
            source: show.insertionSnippet.replacingOccurrences(of: "<#input#>", with: "\"P15_INPUT\""),
            name: "Explicitly Filled Show Snippet"
        )

        let alert = try XCTUnwrap(actions.first(where: { $0.name == "alert" }))
        XCTAssertEqual(alert.insertionSnippet, "alert(<#alert#>)")
        try await assertUnfilledSnippetRejected(alert.insertionSnippet)
        _ = try await CherriCompiler.compile(
            source: alert.insertionSnippet.replacingOccurrences(of: "<#alert#>", with: "\"P15_ALERT\""),
            name: "Explicitly Filled Alert Snippet"
        )

        let pdf = try XCTUnwrap(actions.first(where: { $0.name == "getPDFText" }))
        XCTAssertEqual(pdf.insertionSnippet, "getPDFText(<#pdfFile#>)")
        try await assertUnfilledSnippetRejected(pdf.insertionSnippet)
        // An explicitly authored Ask remains supported; the palette never chooses it.
        _ = try await CherriCompiler.compile(
            source: pdf.insertionSnippet.replacingOccurrences(of: "<#pdfFile#>", with: "Ask"),
            name: "Explicit Ask Control"
        )
    }

    func testVisualMetadataDecompileRoundTrip() async throws {
        let source = """
        #define from sharesheet, search, quickactions
        #define quickactions finder, services

        show("Metadata")
        """

        let original = try await CherriCompiler.compile(source: source, name: "Metadata", mode: "legacy")
        let decompiled = try await CherriCompiler.decompile(plist: original.plist, name: "Metadata")
        XCTAssertTrue(decompiled.contains("#define from"))
        XCTAssertTrue(decompiled.contains("sharesheet"))
        XCTAssertTrue(decompiled.contains("quickactions"))
        XCTAssertTrue(decompiled.contains("#define quickactions finder, services"))

        let rebuilt = try await CherriCompiler.compile(source: decompiled, name: "Metadata", mode: "legacy")
        let plist = try propertyList(rebuilt.plist)
        let workflowTypes = Set((plist["WFWorkflowTypes"] as? [String]) ?? [])
        let quickActions = Set((plist["WFQuickActionSurfaces"] as? [String]) ?? [])
        XCTAssertTrue(workflowTypes.contains("ActionExtension"))
        XCTAssertTrue(workflowTypes.contains("WFWorkflowTypeShowInSearch"))
        XCTAssertTrue(workflowTypes.contains("QuickActions"))
        XCTAssertEqual(quickActions, Set(["Finder", "Services"]))
    }

    func testShortcutPlistEditorAppliesPreviewEdits() throws {
        let shortcut: [String: Any] = [
            "WFWorkflowTypes": [],
            "WFQuickActionSurfaces": [],
            "WFWorkflowActions": [[
                "WFWorkflowActionIdentifier": "is.workflow.actions.test",
                "WFWorkflowActionParameters": ["Enabled": false, "Title": "Old"]
            ]]
        ]
        var data = try PropertyListSerialization.data(fromPropertyList: shortcut, format: .xml, options: 0)
        data = try ShortcutPlistEditor.applying(.workflowType(value: "ActionExtension", enabled: true), to: data)
        data = try ShortcutPlistEditor.applying(.quickActionSurface(value: "Finder", enabled: true), to: data)
        data = try ShortcutPlistEditor.applying(
            .actionParameter(actionIndex: 0, key: "Enabled", value: .bool(true)),
            to: data
        )
        data = try ShortcutPlistEditor.applying(
            .actionParameter(actionIndex: 0, key: "Title", value: .string("New")),
            to: data
        )

        let edited = try propertyList(data)
        XCTAssertTrue(((edited["WFWorkflowTypes"] as? [String]) ?? []).contains("ActionExtension"))
        XCTAssertTrue(((edited["WFWorkflowTypes"] as? [String]) ?? []).contains("QuickActions"))
        XCTAssertEqual(edited["WFQuickActionSurfaces"] as? [String], ["Finder"])
        let actions = try XCTUnwrap(edited["WFWorkflowActions"] as? [[String: Any]])
        let parameters = try XCTUnwrap(actions.first?["WFWorkflowActionParameters"] as? [String: Any])
        XCTAssertEqual(parameters["Enabled"] as? Bool, true)
        XCTAssertEqual(parameters["Title"] as? String, "New")
    }

    func testEveryCatalogActionEmptyCallReturnsWithoutCrashingBridge() async throws {
        _ = try await CherriCompiler.compile(
            source: "show(\"Initialize catalog\")\n",
            name: "Action Catalog Probe"
        )

        let actions = try await CherriCompiler.actionCatalog()
        XCTAssertGreaterThanOrEqual(actions.count, 70)

        for action in actions {
            do {
                _ = try await CherriCompiler.compile(
                    source: "\(action.name)()\n",
                    name: "Probe \(action.name)"
                )
            } catch is CompilationDiagnostic {
                // Missing required arguments are expected for many actions. The
                // important invariant is that the in-process Go bridge returns a
                // diagnostic instead of terminating the iOS process.
                continue
            } catch {
                XCTFail("Unexpected bridge error for \(action.name): \(error)")
            }
        }
    }

    func testV2LanguageLetAndFStringCompile() async throws {
        let source = """
        let greeting = "Hello"
        let msg = f"{greeting}, world!"
        show(msg)
        """
        let result = try await CherriCompiler.compile(source: source, name: "V2 Test")
        let plist = try propertyList(result.plist)
        let actions = try XCTUnwrap(plist["WFWorkflowActions"] as? [[String: Any]])
        XCTAssertFalse(actions.isEmpty)
    }

    func testCherriAnalyzeReturnsMultipleDiagnostics() async throws {
        let source = """
        let x = 10
        x = 20
        let s: Text = 123
        """
        let resp = try await CherriCompiler.analyze(source: source)
        XCTAssertFalse(resp.valid)
        XCTAssertGreaterThanOrEqual(resp.diagnostics.count, 2)
    }

    func testCompletionRequestBindsExactUTF16SourceAndCaret() {
        let request = CherriCompletionRequest(source: "sho", selection: NSRange(location: 3, length: 0))
        XCTAssertEqual(request, CherriCompletionRequest(source: "sho", selection: NSRange(location: 3, length: 0)))
        XCTAssertNotEqual(request, CherriCompletionRequest(source: "tex", selection: NSRange(location: 3, length: 0)))
        XCTAssertNotEqual(request, CherriCompletionRequest(source: "sho", selection: NSRange(location: 2, length: 0)))
        XCTAssertNil(CherriCompletionRequest(source: "sho", selection: NSRange(location: 4, length: 0)).position)
        XCTAssertNil(CherriCompletionRequest(source: "sho", selection: NSRange(location: 1, length: 1)).position)
        let composed = CherriCompletionRequest(source: "\u{00e9}", selection: NSRange(location: 0, length: 0))
        let decomposed = CherriCompletionRequest(source: "e\u{0301}", selection: NSRange(location: 0, length: 0))
        XCTAssertEqual(composed.source, decomposed.source, "Swift normalizes String comparison")
        XCTAssertNotEqual(composed, decomposed, "The completion snapshot must preserve exact UTF-16 identity")
        let unicode = CherriCompletionRequest(source: "\u{05e9}\u{05dc}\u{05d5}\u{05dd} \u{1f469}\u{200d}\u{1f4bb}\nsho", selection: NSRange(location: 14, length: 0))
        XCTAssertEqual(unicode.position?.line, 2)
        XCTAssertEqual(unicode.position?.column, 4)
    }

    func testExactShoCompletionUsesCurrentBridgeSnapshot() async throws {
        let request = CherriCompletionRequest(source: "sho", selection: NSRange(location: 3, length: 0))
        let position = try XCTUnwrap(request.position)
        XCTAssertEqual(position.line, 1)
        XCTAssertEqual(position.column, 4)
        let items = try await CherriCompiler.complete(source: request.source, line: position.line, column: position.column)
        XCTAssertFalse(items.prefix(8).contains { $0.label == "show" },
                       "Regression control: truncating the unfiltered service set loses the typed action")
        let visible = request.matchingItems(items, prefix: "sho")
        XCTAssertTrue(visible.allSatisfy { $0.label.lowercased().hasPrefix("sho") })
        let show = try XCTUnwrap(visible.prefix(8).first { $0.label == "show" })
        XCTAssertEqual(request.matchingItems(items, prefix: "noSuchSyntheticAction"), [])
        XCTAssertEqual(request.matchingItems(items, prefix: nil), items)
        XCTAssertEqual(show.insertText, "show(<#input#>)")
        let insertion = try XCTUnwrap(show.insertText)
        try await assertUnfilledSnippetRejected(insertion)
        _ = try await CherriCompiler.compile(source: insertion.replacingOccurrences(of: "<#input#>", with: "\"P15_COMPLETION\""), name: "Current Completion")
    }

    func testCherriCompleteReturnsContextualItems() async throws {
        let source = "show("
        let items = try await CherriCompiler.complete(source: source, line: 1, column: 6)
        XCTAssertFalse(items.isEmpty)
    }

    func testUnicodeIdentifierCompilationAndAnalysis() async throws {
        let source = """
        let שלום = "world"
        let תוצאה = f"hello {שלום}"
        show(תוצאה)
        """
        let analysisResp = try await CherriCompiler.analyze(source: source)
        XCTAssertTrue(analysisResp.valid, "Unicode identifiers should analyze cleanly: \(analysisResp.diagnostics)")

        let compileResult = try await CherriCompiler.compile(source: source, name: "Unicode Test")
        let plist = try propertyList(compileResult.plist)
        let actions = try XCTUnwrap(plist["WFWorkflowActions"] as? [[String: Any]])
        XCTAssertFalse(actions.isEmpty)
    }

    func testE01ContextualCompletionService() async throws {
        let source = """
        function greet(person: Text) -> Text {
            return f"Hi {person}"
        }
        let targetName = "Alice"
        let msg = greet(targetName)
        show(
        """
        let items = try await CherriCompiler.complete(source: source, line: 6, column: 6)
        XCTAssertFalse(items.isEmpty)
        XCTAssertTrue(items.contains(where: { $0.label == "targetName" || $0.label == "text" }))

        let hoverResp = try await CherriCompiler.hover(source: source, line: 5, column: 12)
        XCTAssertTrue(hoverResp.ok)
        XCTAssertTrue((hoverResp.contents ?? "").contains("greet"))

        let sigResp = try await CherriCompiler.signatureHelp(source: "greet(", line: 1, column: 7)
        // Also test built-in action signature help
        let showSig = try await CherriCompiler.signatureHelp(source: source, line: 6, column: 6)
        XCTAssertTrue(showSig.ok || sigResp.ok)
        XCTAssertTrue((showSig.label ?? "").contains("show"))

        let defResp = try await CherriCompiler.definition(source: source, line: 5, column: 19)
        XCTAssertTrue(defResp.ok)
        let defRange = try XCTUnwrap(defResp.range)
        XCTAssertEqual(defRange.start.line, 3)
    }

    func testE02MultiDiagnosticAnalysisService() async throws {
        let source = """
        let immutable = 10
        immutable = 20
        let wrongType: Text = 123
        unknownActionCall("test")
        """
        let resp = try await CherriCompiler.analyze(source: source)
        XCTAssertFalse(resp.valid)
        XCTAssertGreaterThanOrEqual(resp.diagnostics.count, 3)
        let codes = Set(resp.diagnostics.map(\.code))
        XCTAssertTrue(codes.contains("E_ASSIGN_IMMUTABLE") || codes.contains("E_IMMUTABLE_ASSIGN"))
        XCTAssertTrue(codes.contains("E_ARGUMENT_TYPE"))
        XCTAssertTrue(codes.contains("E_UNKNOWN_ACTION"))
    }

    func testRP40SwiftIntegrationService() async throws {
        let resp = try await CherriCompiler.analyze(source: "let valid = \"ok\"\nshow(valid)\n")
        XCTAssertTrue(resp.valid)
        let comp = try await CherriCompiler.complete(source: "show(", line: 1, column: 6)
        XCTAssertFalse(comp.isEmpty)
    }


    /// Runs in the real Cherri app test host. UI gestures and publication are
    /// separate requirements; this method exercises the app's production APIs.
    @MainActor
    func testRP40ActualAppEntrypointsJoinedContract() async throws {
        let prefix = "// RP40 value comment שלום 🌍\n"
            + "let value = text(\"RP40_VALUE\")\n"
            + "let untouched = \"value remains literal\"\n"
        var document = CherriDocument(text: prefix + "sho")
        let request = CherriCompletionRequest(
            source: document.text,
            selection: NSRange(location: document.text.utf16.count, length: 0)
        )
        let position = try XCTUnwrap(request.position)
        XCTAssertEqual(position.line, 4)
        XCTAssertEqual(position.column, 4)
        let items = try await CherriCompiler.complete(
            source: request.source, line: position.line, column: position.column
        )
        let matches = request.matchingItems(items, prefix: "sho").filter { $0.label == "show" }
        XCTAssertEqual(matches.count, 1)
        let show = try XCTUnwrap(matches.first)
        XCTAssertEqual(show.insertText, "show(<#input#>)")
        XCTAssertEqual(request.matchingItems(items, prefix: "RP40_no_such_action"), [])
        XCTAssertNotEqual(request, CherriCompletionRequest(
            source: document.text + "w",
            selection: NSRange(location: document.text.utf16.count, length: 0)
        ))
        let snippet = try XCTUnwrap(show.insertText)
        try await assertUnfilledSnippetRejected(prefix + snippet + "\n")
        let replacement = snippet.replacingOccurrences(of: "<#input#>", with: "value")
        document.text = (document.text as NSString).replacingCharacters(
            in: NSRange(location: document.text.utf16.count - 3, length: 3),
            with: replacement + "\n"
        )
        let completed = prefix + "show(value)\n"
        XCTAssertTrue(document.text.utf16.elementsEqual(completed.utf16))

        let valid = try await CherriCompiler.analyze(source: document.text)
        XCTAssertTrue(valid.valid)
        XCTAssertTrue(valid.diagnostics.isEmpty)
        XCTAssertEqual(valid.languageVersion, "2.0")
        XCTAssertEqual(valid.schemaFingerprint.count, 64)
        XCTAssertTrue(valid.schemaFingerprint.allSatisfy { "0123456789abcdef".contains($0) })

        let invalid = document.text + "unknownRP40()\n"
        let diagnostic = try await CherriCompiler.analyze(source: invalid)
        XCTAssertFalse(diagnostic.valid)
        XCTAssertEqual(diagnostic.languageVersion, valid.languageVersion)
        XCTAssertEqual(diagnostic.schemaFingerprint, valid.schemaFingerprint)
        XCTAssertEqual(diagnostic.diagnostics.count, 1)
        let unknown = try XCTUnwrap(diagnostic.diagnostics.first)
        XCTAssertEqual(unknown.code, "E_UNKNOWN_ACTION")
        XCTAssertEqual(unknown.range.start, CherriWirePosition(line: 4, character: 0))
        XCTAssertEqual(unknown.range.end, CherriWirePosition(line: 4, character: 11))
        do {
            _ = try await CherriCompiler.compile(source: invalid, name: "RP40 Rejected")
            XCTFail("The app compiler accepted the diagnosed unknown action")
        } catch let failure as CompilationDiagnostic {
            XCTAssertTrue(failure.message.contains("E_UNKNOWN_ACTION"))
        }

        let rejectedRename = try await CherriCompiler.rename(
            source: document.text, line: 2, column: 6, newName: "1invalid"
        )
        XCTAssertFalse(rejectedRename.ok)
        XCTAssertNil(rejectedRename.source)
        XCTAssertNotNil(rejectedRename.error)
        XCTAssertTrue(document.text.utf16.elementsEqual(completed.utf16))
        let renamed = try await CherriCompiler.rename(
            source: document.text, line: 2, column: 6, newName: "renamedValue"
        )
        XCTAssertTrue(renamed.ok)
        let edited = try XCTUnwrap(renamed.source)
        let expected = "// RP40 value comment שלום 🌍\n"
            + "let renamedValue = text(\"RP40_VALUE\")\n"
            + "let untouched = \"value remains literal\"\n"
            + "show(renamedValue)\n"
        XCTAssertTrue(edited.utf16.elementsEqual(expected.utf16))
        let expectedEdits = [
            CherriTextEditItem(
                range: CherriWireRange(
                    start: CherriWirePosition(line: 1, character: 4),
                    end: CherriWirePosition(line: 1, character: 9)
                ), newText: "renamedValue"
            ),
            CherriTextEditItem(
                range: CherriWireRange(
                    start: CherriWirePosition(line: 3, character: 5),
                    end: CherriWirePosition(line: 3, character: 10)
                ), newText: "renamedValue"
            ),
        ]
        let edits = try XCTUnwrap(renamed.edits)
        XCTAssertEqual(edits.count, 2)
        XCTAssertEqual(Set(edits), Set(expectedEdits))
        document.text = edited

        let compiled = try await CherriCompiler.compile(
            source: document.text, name: "RP40 App Contract", mode: "v2", buildProfile: .debug
        )
        XCTAssertEqual(compiled.actualMode, "v2")
        XCTAssertNil(compiled.signedShortcut)
        XCTAssertTrue(compiled.sourceRequest.source.utf16.elementsEqual(expected.utf16))
        XCTAssertEqual(compiled.sourceRequest.name, "RP40 App Contract")
        XCTAssertEqual(compiled.sourceRequest.mode, "v2")
        XCTAssertEqual(compiled.sourceRequest.buildProfile, .debug)
        let actions = try XCTUnwrap(
            try propertyList(compiled.plist)["WFWorkflowActions"] as? [[String: Any]]
        )
        let catalog = try await CherriCompiler.actionCatalog()
        let catalogShow = catalog.filter { $0.name == show.label }
        XCTAssertEqual(catalogShow.count, 1)
        let sharedShow = try XCTUnwrap(catalogShow.first)
        XCTAssertEqual(sharedShow.shortcutIdentifier, "is.workflow.actions.showresult")
        let shown = actions.filter {
            $0["WFWorkflowActionIdentifier"] as? String == sharedShow.shortcutIdentifier
        }
        XCTAssertEqual(shown.count, 1)
        let shownParameters = try XCTUnwrap(
            shown.first?["WFWorkflowActionParameters"] as? [String: Any]
        )
        XCTAssertNotNil(shownParameters["Text"])
        let namedTexts = actions.filter { action in
            let parameters = action["WFWorkflowActionParameters"] as? [String: Any]
            return action["WFWorkflowActionIdentifier"] as? String == "is.workflow.actions.gettext"
                && parameters?["CustomOutputName"] as? String == "renamedValue"
                && parameters?["WFTextActionText"] as? String == "RP40_VALUE"
        }
        XCTAssertEqual(namedTexts.count, 1)
        let producer = try XCTUnwrap(
            namedTexts.first?["WFWorkflowActionParameters"] as? [String: Any]
        )
        let producerUUID = try XCTUnwrap(producer["UUID"] as? String)
        XCTAssertNotNil(UUID(uuidString: producerUUID))
        let producerPositions = actions.indices.filter {
            (actions[$0]["WFWorkflowActionParameters"] as? [String: Any])?["UUID"] as? String
                == producerUUID
        }
        XCTAssertEqual(producerPositions.count, 1)
        let showPosition = try XCTUnwrap(actions.firstIndex {
            $0["WFWorkflowActionIdentifier"] as? String == sharedShow.shortcutIdentifier
        })
        XCTAssertLessThan(try XCTUnwrap(producerPositions.first), showPosition)

        // The ordinary show(Text) writer serializes this immutable reference
        // as one text-token attachment. Require its exact producer edge.
        func shownTextMatchesProducer(_ candidate: Any?) -> Bool {
            guard let token = candidate as? [String: Any],
                  Set(token.keys) == Set(["WFSerializationType", "Value"]),
                  token["WFSerializationType"] as? String == "WFTextTokenString",
                  let value = token["Value"] as? [String: Any],
                  Set(value.keys) == Set(["string", "attachmentsByRange"]),
                  value["string"] as? String == "\u{FFFC}",
                  let attachments = value["attachmentsByRange"] as? [String: Any],
                  Set(attachments.keys) == Set(["{0, 1}"]),
                  let reference = attachments["{0, 1}"] as? [String: Any] else {
                return false
            }
            let expectedReference: [String: Any] = [
                "Type": "ActionOutput", "OutputUUID": producerUUID, "OutputName": "renamedValue",
            ]
            return NSDictionary(dictionary: reference).isEqual(to: expectedReference)
        }
        XCTAssertTrue(shownTextMatchesProducer(shownParameters["Text"]))
        var constantInput = shownParameters
        constantInput["Text"] = "RP40_UNRELATED_CONSTANT"
        XCTAssertFalse(shownTextMatchesProducer(constantInput["Text"]))
        var wrongToken = try XCTUnwrap(shownParameters["Text"] as? [String: Any])
        var wrongValue = try XCTUnwrap(wrongToken["Value"] as? [String: Any])
        var wrongAttachments = try XCTUnwrap(wrongValue["attachmentsByRange"] as? [String: Any])
        var wrongReference = try XCTUnwrap(wrongAttachments["{0, 1}"] as? [String: Any])
        let wrongUUID = producerUUID == "00000000-0000-0000-0000-000000000000"
            ? "11111111-1111-1111-1111-111111111111" : "00000000-0000-0000-0000-000000000000"
        wrongReference["OutputUUID"] = wrongUUID
        wrongAttachments["{0, 1}"] = wrongReference
        wrongValue["attachmentsByRange"] = wrongAttachments
        wrongToken["Value"] = wrongValue
        var detachedInput = shownParameters
        detachedInput["Text"] = wrongToken
        XCTAssertFalse(shownTextMatchesProducer(detachedInput["Text"]))
        XCTAssertTrue(shownTextMatchesProducer(shownParameters["Text"]))
        let recovery = try await CherriCompiler.analyze(source: document.text)
        XCTAssertTrue(recovery.valid)
        XCTAssertTrue(recovery.diagnostics.isEmpty)
        XCTAssertEqual(recovery.schemaFingerprint, valid.schemaFingerprint)
        XCTAssertTrue(document.text.utf16.elementsEqual(expected.utf16))
        print("[CHECKPOINT:RP40_APP_ENTRYPOINTS_VERIFIED]")
    }

    // Deprecated forwarders for backward compatibility
    func testE01MultiDiagnosticAnalysis() async throws {
        try await testE02MultiDiagnosticAnalysisService()
    }

    func testE02CompletionHoverSignatureDefinition() async throws {
        try await testE01ContextualCompletionService()
    }

    func testE03ScopedRenameIgnoresStringsCommentsAndShadowing() async throws {
        let source = """
        // count comment should stay count
        let count = 1
        let literal = "count in string stays count"
        if true {
            let count = 99
            show(f"{count}")
        }
        show(f"{count}")
        """
        let renameResp = try await CherriCompiler.rename(source: source, line: 2, column: 6, newName: "totalCount")
        XCTAssertTrue(renameResp.ok, "Rename failed: \(renameResp.error ?? "")")
        let updated = try XCTUnwrap(renameResp.source)
        XCTAssertTrue(updated.contains("// count comment should stay count"))
        XCTAssertTrue(updated.contains("\"count in string stays count\""))
        XCTAssertTrue(updated.contains("let totalCount = 1"))
        XCTAssertTrue(updated.contains("let count = 99"))
        XCTAssertTrue(updated.contains("show(f\"{totalCount}\")"))
        XCTAssertEqual(renameResp.edits?.count, 2)
    }

    func testE04FormatterPreservesCommentsAndSemantics() async throws {
        let messy = """
        // Header comment preserved
        let   x=10
        // Mid comment preserved
        show(  f"val={x}"  )
        """
        let formatted1 = try await CherriCompiler.format(source: messy)
        let formatted2 = try await CherriCompiler.format(source: formatted1)
        XCTAssertEqual(formatted1, formatted2, "Formatter must be idempotent")
        XCTAssertTrue(formatted1.contains("// Header comment preserved"))
        XCTAssertTrue(formatted1.contains("// Mid comment preserved"))

        let c1 = try await CherriCompiler.compile(source: messy, name: "FormatBefore")
        let c2 = try await CherriCompiler.compile(source: formatted1, name: "FormatAfter")
        let a1 = try XCTUnwrap(try propertyList(c1.plist)["WFWorkflowActions"] as? [[String: Any]])
        let a2 = try XCTUnwrap(try propertyList(c2.plist)["WFWorkflowActions"] as? [[String: Any]])
        XCTAssertEqual(a1.count, a2.count)
    }

    func testE05OutOfOrderVersionedEdits() async throws {
        let uri = "file:///version-test.cherri"
        let validNewer = "let okVal = \"clean\"\nshow(okVal)\n"
        let invalidStale = "let broken: Text = 999\n"

        let r2 = try await CherriCompiler.analyzeVersioned(uri: uri, version: 2, source: validNewer)
        XCTAssertTrue(r2.valid)
        XCTAssertEqual(r2.version, 2)

        // Send stale version 1 after version 2
        let r1Stale = try await CherriCompiler.analyzeVersioned(uri: uri, version: 1, source: invalidStale)
        XCTAssertTrue(r1Stale.valid, "Stale version 1 edit must not overwrite version 2 state")
        XCTAssertEqual(r1Stale.version, 2)
        XCTAssertTrue(r1Stale.diagnostics.isEmpty)
    }

    func testE06UTF16DiagnosticSpansEmojiAndHebrew() async throws {
        // "🌍" is 1 rune, 4 UTF-8 bytes, 2 UTF-16 code units.
        // Line 1: let a = "🌍" ; unknownEmoji()
        // Indices in UTF-16:
        // "let a = \"" -> 9 code units
        // "🌍" -> 2 code units (pos 9..11)
        // "\" ; " -> 4 code units (pos 11..15)
        // "unknownEmoji()" -> starts at UTF-16 character 15, ends at 29
        let emojiLine = "let a = \"🌍\" ; unknownEmoji()\n"
        let resp1 = try await CherriCompiler.analyze(source: emojiLine)
        let diag1 = try XCTUnwrap(resp1.diagnostics.first)
        XCTAssertEqual(diag1.range.start.line, 0)
        XCTAssertEqual(diag1.range.start.character, 15)
        XCTAssertEqual(diag1.range.end.character, 27)

        // Hebrew with combining mark: "שָׁ" (Shin + Qamats + Shin Dot = 3 BMP code points = 3 UTF-16 units)
        let hebrewLine = "let x = \"שָׁ\" ; unknownHeb()\n"
        let resp2 = try await CherriCompiler.analyze(source: hebrewLine)
        let diag2 = try XCTUnwrap(resp2.diagnostics.first)
        XCTAssertEqual(diag2.range.start.line, 0)
        XCTAssertEqual(diag2.range.start.character, 16)
        XCTAssertEqual(diag2.range.end.character, 26)
    }

    func testE07BoundedVisualSourceEditAndRejection() async throws {
        let source = """
        // Keep this header comment
        show("Original Title")
        // Keep this footer comment
        """
        let editResp = try await CherriCompiler.editSourceNode(
            source: source,
            actionIndex: 0,
            paramLabel: "text",
            newExpr: "\"Updated From Preview\""
        )
        XCTAssertTrue(editResp.ok, "Expected visual source edit to succeed: \(editResp.error ?? "")")
        let editedSource = try XCTUnwrap(editResp.source)
        XCTAssertTrue(editedSource.contains("// Keep this header comment"))
        XCTAssertTrue(editedSource.contains("show(\"Updated From Preview\")"))
        XCTAssertTrue(editedSource.contains("// Keep this footer comment"))

        // Reject out-of-range / generated action index without corrupting source
        let rejectResp = try await CherriCompiler.editSourceNode(
            source: editedSource,
            actionIndex: 99,
            paramLabel: "text",
            newExpr: "\"Should Reject\""
        )
        XCTAssertFalse(rejectResp.ok)
        XCTAssertEqual(rejectResp.source, editedSource)
    }

    func testBRG25SharedAnalysisBridgeContract() async throws {
        let resp = try await CherriCompiler.analyze(source: "show(\"Bridge\")\n")
        XCTAssertTrue(resp.valid)
        XCTAssertEqual(resp.languageVersion, "2.0")
        XCTAssertEqual(resp.schemaFingerprint.count, 64)
    }

    func testBuildProfilesPreserveSourceThroughCABI() async throws {
        let source = "text(\"bridge source\")\ndebugLog()\ndebugAssert(false, \"bridge validation\")\n"
        for mode in ["legacy", "v2"] {
            for profile in [CherriBuildProfile.release, .debug] {
                let result = try await CherriCompiler.compile(
                    source: source, name: "Profile Bridge", mode: mode, buildProfile: profile
                )
                let actions = try XCTUnwrap(try propertyList(result.plist)["WFWorkflowActions"] as? [[String: Any]])
                let identifiers = actions.compactMap { $0["WFWorkflowActionIdentifier"] as? String }
                XCTAssertEqual(identifiers.contains("is.workflow.actions.appendvariable"), profile == .debug)
                XCTAssertTrue(identifiers.contains("is.workflow.actions.gettext"))
                XCTAssertTrue(identifiers.contains("is.workflow.actions.alert"))
                XCTAssertTrue(identifiers.contains("is.workflow.actions.exit"))
                XCTAssertNil(result.signedShortcut)
            }
        }
    }

    private func propertyList(_ data: Data) throws -> [String: Any] {
        let object = try PropertyListSerialization.propertyList(from: data, options: [], format: nil)
        return try XCTUnwrap(object as? [String: Any])
    }
}
