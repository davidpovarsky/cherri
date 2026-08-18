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

    private func propertyList(_ data: Data) throws -> [String: Any] {
        let object = try PropertyListSerialization.propertyList(from: data, options: [], format: nil)
        return try XCTUnwrap(object as? [String: Any])
    }
}
