import Foundation
import XCTest
@testable import Cherri

final class CherriCoreIntegrationTests: XCTestCase {
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

        let first = try await CherriCompiler.compile(source: source, name: "First")
        let second = try await CherriCompiler.compile(source: source, name: "Second")

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

        let rebuilt = try await CherriCompiler.compile(source: decompiled, name: "Round Trip Rebuilt")
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

    func testPaletteSnippetSuppliesRequiredArguments() async throws {
        _ = try await CherriCompiler.compile(
            source: "show(\"Initialize catalog\")\n",
            name: "Snippet Test"
        )
        let actions = try await CherriCompiler.actionCatalog()

        let alert = try XCTUnwrap(actions.first(where: { $0.name == "alert" }))
        XCTAssertEqual(alert.insertionSnippet, "alert(Ask)")
        _ = try await CherriCompiler.compile(source: alert.insertionSnippet + "\n", name: "Alert Snippet")

        let pdf = try XCTUnwrap(actions.first(where: { $0.name == "getPDFText" }))
        XCTAssertTrue(pdf.insertionSnippet.contains("Ask"))
        _ = try await CherriCompiler.compile(source: pdf.insertionSnippet + "\n", name: "PDF Snippet")
    }

    func testVisualMetadataDecompileRoundTrip() async throws {
        let source = """
        #define from sharesheet, search, quickactions
        #define quickactions finder, services

        show("Metadata")
        """

        let original = try await CherriCompiler.compile(source: source, name: "Metadata")
        let decompiled = try await CherriCompiler.decompile(plist: original.plist, name: "Metadata")
        XCTAssertTrue(decompiled.contains("#define from"))
        XCTAssertTrue(decompiled.contains("sharesheet"))
        XCTAssertTrue(decompiled.contains("quickactions"))
        XCTAssertTrue(decompiled.contains("#define quickactions finder, services"))

        let rebuilt = try await CherriCompiler.compile(source: decompiled, name: "Metadata")
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

    func testE01MultiDiagnosticAnalysis() async throws {
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

    func testE02CompletionHoverSignatureDefinition() async throws {
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

    private func propertyList(_ data: Data) throws -> [String: Any] {
        let object = try PropertyListSerialization.propertyList(from: data, options: [], format: nil)
        return try XCTUnwrap(object as? [String: Any])
    }
}
