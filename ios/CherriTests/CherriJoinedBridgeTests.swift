import CherriCore
import Foundation
import XCTest
@testable import Cherri

/// These methods are native C ABI/app-model producers. They do not claim
/// an LSP session, a Shortcut runtime, an Inspector result or a benchmark.
final class CherriJoinedBridgeTests: XCTestCase {
    override func setUpWithError() throws {
        continueAfterFailure = false
    }

    private func propertyList(_ data: Data) throws -> [String: Any] {
        try XCTUnwrap(try PropertyListSerialization.propertyList(
            from: data, options: [], format: nil
        ) as? [String: Any])
    }

    private func actionContains(_ value: Any, text: String) -> Bool {
        if let string = value as? String { return string.utf16.elementsEqual(text.utf16) }
        if let array = value as? [Any] { return array.contains { actionContains($0, text: text) } }
        if let dictionary = value as? [String: Any] {
            return dictionary.values.contains { actionContains($0, text: text) }
        }
        return false
    }

    private func assertDiagnostics(_ actual: CherriAnalyzeResponse, fixture: JoinedFixture) throws {
        let expected = try XCTUnwrap(fixture.diagnostics)
        XCTAssertFalse(actual.valid)
        XCTAssertEqual(actual.languageVersion, "2.0")
        XCTAssertEqual(actual.diagnostics.count, expected.count)
        for (observed, wanted) in zip(actual.diagnostics, expected) {
            XCTAssertEqual(observed.code, wanted.code)
            XCTAssertEqual(observed.severity, wanted.severity)
            XCTAssertEqual(observed.range.start,
                           CherriWirePosition(line: wanted.range.start.line, character: wanted.range.start.character))
            XCTAssertEqual(observed.range.end,
                           CherriWirePosition(line: wanted.range.end.line, character: wanted.range.end.character))
        }
    }

    /// Calls the same exported functions used by CherriCompiler. Keeping the
    /// original error response allows explicit no-plist/no-signature checks.
    private func rawCompile(source: String, mode: String, signed: Bool) throws -> [String: Any] {
        let pointer: UnsafeMutablePointer<CChar>? = source.withCString { sourcePointer in
            "BRG25 Native Joined".withCString { namePointer in
                mode.withCString { modePointer in
                    "debug".withCString { profilePointer in
                        signed ? CherriCompileSignedProfile(
                            UnsafeMutablePointer(mutating: sourcePointer),
                            UnsafeMutablePointer(mutating: namePointer),
                            UnsafeMutablePointer(mutating: modePointer),
                            UnsafeMutablePointer(mutating: profilePointer)
                        ) : CherriCompileProfile(
                            UnsafeMutablePointer(mutating: sourcePointer),
                            UnsafeMutablePointer(mutating: namePointer),
                            UnsafeMutablePointer(mutating: modePointer),
                            UnsafeMutablePointer(mutating: profilePointer)
                        )
                    }
                }
            }
        }
        let response = try XCTUnwrap(pointer, "Actual C ABI response is required")
        defer { CherriFree(response) }
        return try XCTUnwrap(try JSONSerialization.jsonObject(
            with: Data(String(cString: response).utf8)
        ) as? [String: Any])
    }

    func testBRG25ActualMobileRouteJoinedContract() async throws {
        let contract = try CherriJoinedContract(
            testId: "CherriJoinedBridgeTests/testBRG25ActualMobileRouteJoinedContract"
        )
        for (id, literal) in [
            ("literal-email", "owner@example.invalid"),
            ("literal-ask", "Ask"),
            ("literal-directive", "#define from sharesheet // cherri: legacy"),
        ] {
            let fixture = try contract.fixture(id)
            let analysis = try await CherriCompiler.analyze(source: fixture.source)
            XCTAssertTrue(analysis.valid)
            XCTAssertTrue(analysis.diagnostics.isEmpty)
            XCTAssertEqual(analysis.languageVersion, "2.0")
            for mode in ["v2", "auto"] {
                let compiled = try await CherriCompiler.compile(
                    source: fixture.source, name: "BRG25 Literal", mode: mode
                )
                XCTAssertEqual(compiled.actualMode, "v2")
                XCTAssertNil(compiled.signedShortcut)
                XCTAssertTrue(compiled.sourceRequest.source.utf16.elementsEqual(fixture.source.utf16))
                let actions = try XCTUnwrap(try propertyList(compiled.plist)["WFWorkflowActions"] as? [[String: Any]])
                XCTAssertFalse(actions.isEmpty)
                XCTAssertTrue(actionContains(actions, text: literal),
                              "The literal must survive in an actual action parameter")
            }
            try contract.stage(id + "-v2-auto", inputSource: fixture.source, observedSource: fixture.source)
        }

        for (id, stage) in [
            ("rejected-type", "type-all-mobile-routes-rejected"),
            ("rejected-function", "malformed-all-mobile-routes-rejected"),
        ] {
            let fixture = try contract.fixture(id)
            let analysis = try await CherriCompiler.analyze(source: fixture.source)
            try assertDiagnostics(analysis, fixture: fixture)
            let code = try XCTUnwrap(fixture.diagnostics?.first?.code)
            for mode in ["v2", "auto"] {
                for signed in [false, true] {
                    let response = try rawCompile(source: fixture.source, mode: mode, signed: signed)
                    XCTAssertEqual(response["ok"] as? Bool, false)
                    XCTAssertTrue((response["error"] as? String)?.contains(code) == true)
                    XCTAssertNil(response["plistBase64"])
                    XCTAssertNil(response["signedBase64"])
                    do {
                        _ = try await CherriCompiler.compile(
                            source: fixture.source, name: "BRG25 Rejected", mode: mode, signed: signed
                        )
                        XCTFail("The actual app wrapper must reject this current invalid V2 source")
                    } catch let diagnostic as CompilationDiagnostic {
                        XCTAssertTrue(diagnostic.message.contains(code))
                    }
                }
            }
            try contract.stage(stage, inputSource: fixture.source, observedSource: fixture.source)
        }
        try contract.finish()
    }

    private func assertCompletion(_ fixture: JoinedFixture) async throws -> String {
        let expected = try XCTUnwrap(fixture.completion)
        let items = try await CherriCompiler.complete(
            source: fixture.source, line: expected.caret.line + 1, column: expected.caret.character + 1
        )
        let matches = items.filter { $0.label == expected.label }
        XCTAssertEqual(matches.count, 1)
        let item = try XCTUnwrap(matches.first)
        let edit = try XCTUnwrap(item.textEdit, "The actual native response must forward the shared edit")
        XCTAssertEqual(edit.range.start, CherriWirePosition(
            line: expected.textEdit.range.start.line, character: expected.textEdit.range.start.character
        ))
        XCTAssertEqual(edit.range.end, CherriWirePosition(
            line: expected.textEdit.range.end.line, character: expected.textEdit.range.end.character
        ))
        XCTAssertEqual(edit.newText, expected.textEdit.newText)
        let lines = fixture.source.components(separatedBy: "\n")
        let lineOffset = lines.prefix(expected.caret.line).reduce(0) { $0 + $1.utf16.count + 1 }
        let caret = NSRange(location: lineOffset + expected.caret.character, length: 0)
        let request = CherriCompletionRequest(source: fixture.source, selection: caret)
        let changed = try XCTUnwrap(request.applying(item, to: request))
        XCTAssertTrue(changed.utf16.elementsEqual(fixture.expectedSource.utf16))
        XCTAssertFalse(changed.utf16.elementsEqual(fixture.source.utf16), "A no-op cannot prove replacement")
        XCTAssertNil(request.applying(item, to: CherriCompletionRequest(
            source: fixture.source + "x", selection: caret
        )))
        return changed
    }

    func testBRG27ActualEditorBridgeJoinedContract() async throws {
        let contract = try CherriJoinedContract(
            testId: "CherriJoinedBridgeTests/testBRG27ActualEditorBridgeJoinedContract"
        )
        let incomplete = try contract.fixture("incomplete-unicode")
        let incompleteResult = try await CherriCompiler.analyze(source: incomplete.source)
        XCTAssertFalse(incompleteResult.valid)
        XCTAssertFalse(incompleteResult.diagnostics.isEmpty)
        XCTAssertEqual(incompleteResult.languageVersion, "2.0")
        try contract.stage("incomplete-unicode", inputSource: incomplete.source, observedSource: incomplete.source)

        let multiple = try contract.fixture("multi-error-utf16")
        let multipleResult = try await CherriCompiler.analyze(source: multiple.source)
        try assertDiagnostics(multipleResult, fixture: multiple)
        try contract.stage("multi-error-utf16", inputSource: multiple.source, observedSource: multiple.source)

        for (id, stage) in [("parameter-midword", "parameter-replacement"),
                            ("enum-midword", "enum-replacement")] {
            let fixture = try contract.fixture(id)
            let changed = try await assertCompletion(fixture)
            try contract.stage(stage, inputSource: fixture.source, observedSource: changed)
        }

        let newest = try contract.fixture("newest-version")
        let stale = try contract.fixture("stale-version")
        let uri = "file:///native-joined-\(UUID().uuidString).cherri"
        let versionTwo = try await CherriCompiler.analyzeVersioned(uri: uri, version: 2, source: newest.source)
        XCTAssertEqual(versionTwo.version, 2)
        XCTAssertTrue(versionTwo.valid)
        let versionOne = try await CherriCompiler.analyzeVersioned(uri: uri, version: 1, source: stale.source)
        XCTAssertEqual(versionOne.version, 2)
        XCTAssertTrue(versionOne.valid)
        XCTAssertTrue(versionOne.diagnostics.isEmpty)
        try await withThrowingTaskGroup(of: CherriAnalyzeResponse.self) { group in
            for version in 3...8 {
                let source = version.isMultiple(of: 2) ? newest.source : stale.source
                group.addTask {
                    try await CherriCompiler.analyzeVersioned(uri: uri, version: version, source: source)
                }
            }
            for try await response in group {
                XCTAssertTrue((3...8).contains(response.version))
                XCTAssertEqual(response.languageVersion, versionTwo.languageVersion)
                XCTAssertEqual(response.schemaFingerprint, versionTwo.schemaFingerprint)
            }
        }
        let last = try await CherriCompiler.analyzeVersioned(uri: uri, version: 1, source: stale.source)
        XCTAssertEqual(last.version, 8)
        XCTAssertTrue(last.valid)
        XCTAssertTrue(last.diagnostics.isEmpty)
        try contract.stage("newest-and-stale-analysis", inputSource: newest.source, observedSource: newest.source)

        var document = CherriDocument(text: newest.source)
        let previous = document.beginEditorAction()
        let current = document.beginEditorAction()
        XCTAssertFalse(document.acceptsEditorAction(previous))
        XCTAssertTrue(document.acceptsEditorAction(current))
        document.text = stale.source
        document.text = newest.source
        XCTAssertFalse(document.acceptsEditorAction(current), "A changed-and-reverted document still has a newer revision")
        var other = CherriDocument(text: newest.source)
        XCTAssertFalse(other.acceptsEditorAction(document.beginEditorAction()))
        let otherCurrent = other.beginEditorAction()
        XCTAssertTrue(other.acceptsEditorAction(otherCurrent))
        var unicode = CherriDocument(text: "show(\"e\u{301}\")\n")
        let decomposed = unicode.beginEditorAction()
        unicode.text = "show(\"\u{e9}\")\n"
        XCTAssertFalse(unicode.acceptsEditorAction(decomposed), "Canonical Unicode equality must not reuse UTF-16 ranges")
        try contract.stage("document-request-guards", inputSource: newest.source, observedSource: document.text)

        let rename = try contract.fixture("hover-rename")
        let hover = try await CherriCompiler.hover(source: rename.source, line: 2, column: 6)
        XCTAssertTrue(hover.ok)
        XCTAssertEqual(hover.contents, "```cherri\n(variable) value\n```")
        let definition = try await CherriCompiler.definition(source: rename.source, line: 4, column: 7)
        XCTAssertTrue(definition.ok)
        XCTAssertEqual(definition.range?.start, CherriWirePosition(line: 1, character: 4))
        XCTAssertEqual(definition.range?.end, CherriWirePosition(line: 1, character: 9))
        try contract.stage("hover-definition", inputSource: rename.source, observedSource: rename.source)

        let renamed = try await CherriCompiler.rename(source: rename.source, line: 2, column: 6, newName: "renamedValue")
        XCTAssertTrue(renamed.ok)
        let edited = try XCTUnwrap(renamed.source)
        XCTAssertTrue(edited.utf16.elementsEqual(rename.expectedSource.utf16))
        let edits = try XCTUnwrap(renamed.edits)
        XCTAssertEqual(Set(edits), Set([
            CherriTextEditItem(range: CherriWireRange(start: CherriWirePosition(line: 1, character: 4),
                                                    end: CherriWirePosition(line: 1, character: 9)), newText: "renamedValue"),
            CherriTextEditItem(range: CherriWireRange(start: CherriWirePosition(line: 3, character: 5),
                                                    end: CherriWirePosition(line: 3, character: 10)), newText: "renamedValue"),
        ]))
        XCTAssertEqual(edits.count, 2)
        try contract.stage("scoped-rename", inputSource: rename.source, observedSource: edited)

        let format = try contract.fixture("format")
        let formatted = try await CherriCompiler.format(source: format.source)
        XCTAssertTrue(formatted.utf16.elementsEqual(format.expectedSource.utf16))
        let again = try await CherriCompiler.format(source: formatted)
        XCTAssertTrue(again.utf16.elementsEqual(formatted.utf16))
        try contract.stage("format-preserved-idempotent", inputSource: format.source, observedSource: formatted)

        let currentSource = try contract.fixture("current-source")
        XCTAssertTrue(currentSource.source.utf16.elementsEqual(formatted.utf16))
        let compiled = try await CherriCompiler.compile(source: formatted, name: "BRG27 Current", mode: "v2")
        XCTAssertEqual(compiled.actualMode, "v2")
        XCTAssertNil(compiled.signedShortcut)
        XCTAssertTrue(compiled.sourceRequest.source.utf16.elementsEqual(formatted.utf16))
        let actions = try XCTUnwrap(try propertyList(compiled.plist)["WFWorkflowActions"] as? [[String: Any]])
        XCTAssertFalse(actions.isEmpty)
        XCTAssertTrue(actionContains(actions, text: "BRG27_VALUE"))
        try contract.stage("current-source-compiled", inputSource: currentSource.source,
                           observedSource: compiled.sourceRequest.source)
        try contract.finish()
    }
}
