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
