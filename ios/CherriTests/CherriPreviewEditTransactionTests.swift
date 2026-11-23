import Foundation
import XCTest
@testable import Cherri

@MainActor
final class CherriPreviewEditTransactionTests: XCTestCase {
    private let edit = ShortcutPreviewEdit.actionParameter(actionIndex: 4, key: "WFDelayTime", value: .integer(9))

    @MainActor
    private final class State {
        var request: CherriEditorBuildRequest
        var compiled: CompiledShortcut
        var buildGeneration: UInt64 = 7
        var editGeneration = 3
        var signedURL: URL? = URL(fileURLWithPath: "/preview-test/original.shortcut")
        var publications = 0

        init() {
            let request = CherriEditorBuildRequest(source: "// café\nwait(2)\n", mode: "auto", name: "Café.cherri", buildProfile: .release)
            self.request = request
            compiled = CompiledShortcut(
                name: "Café", actualMode: "v2", plist: Data("original-preview".utf8), signedShortcut: Data("original-signature".utf8),
                sourceRequest: request, previewSourceMap: nil
            )
        }

        var transaction: CherriPreviewEditTransaction {
            CherriPreviewEditTransaction(original: compiled, buildGeneration: buildGeneration, editGeneration: editGeneration)
        }

        func current(_ transaction: CherriPreviewEditTransaction) -> Bool {
            transaction.matches(compiled: compiled, request: request,
                                buildGeneration: buildGeneration, editGeneration: editGeneration)
        }

        func publish(_ source: String, _ rebuilt: CompiledShortcut) {
            publications += 1
            request = rebuilt.sourceRequest
            compiled = rebuilt
            signedURL = nil
            XCTAssertEqual(Array(source.utf16), Array(request.source.utf16))
        }
    }

    private func rebuilt(_ request: CherriEditorBuildRequest) -> CompiledShortcut {
        CompiledShortcut(name: "Café", actualMode: "v2", plist: Data("rebuilt-preview".utf8), signedShortcut: nil,
                         sourceRequest: request, previewSourceMap: nil)
    }

    func testSuccessfulEditPublishesOnlyAfterOneRebuildOfCapturedRequest() async throws {
        let state = State()
        let transaction = state.transaction
        let signed = state.signedURL
        var edits = 0
        var builds = 0
        let updated = "// café\nwait(9)\n"
        let operations = CherriPreviewEditTransaction.Operations(
            editSource: { compiled, index, key, expression in
                edits += 1
                XCTAssertEqual(compiled.id, transaction.original.id)
                XCTAssertEqual(index, 4)
                XCTAssertEqual(key, "WFDelayTime")
                XCTAssertEqual(expression, "9")
                await Task.yield()
                XCTAssertEqual(state.publications, 0)
                XCTAssertEqual(state.signedURL, signed)
                return CherriSourceEditResponse(ok: true, source: updated, error: nil)
            },
            decompile: { _, _ in XCTFail("V2 must not decompile the graph"); return "" },
            compile: { request in
                builds += 1
                XCTAssertEqual(request.mode, "auto")
                XCTAssertEqual(request.name, transaction.original.sourceRequest.name)
                XCTAssertEqual(request.buildProfile, .release)
                XCTAssertEqual(request.source, updated)
                await Task.yield()
                XCTAssertEqual(state.publications, 0)
                XCTAssertEqual(state.compiled.id, transaction.original.id)
                XCTAssertEqual(state.signedURL, signed)
                return self.rebuilt(request)
            }
        )
        let published = try await transaction.perform(edit, operations: operations,
                                                      isCurrent: { state.current(transaction) }, publish: state.publish)
        XCTAssertTrue(published)
        XCTAssertEqual(edits, 1)
        XCTAssertEqual(builds, 1)
        XCTAssertEqual(state.publications, 1)
        XCTAssertEqual(state.request.source, updated)
        XCTAssertNotEqual(state.compiled.id, transaction.original.id)
        XCTAssertNil(state.signedURL)
    }

    func testRejectedMappingAndFailedRebuildPreserveCommittedState() async throws {
        for failure in ["mapping", "compile", "wrong-rebuild-request"] {
            let state = State()
            let transaction = state.transaction
            let signed = state.signedURL
            let operations = CherriPreviewEditTransaction.Operations(
                editSource: { _, _, _, _ in
                    CherriSourceEditResponse(ok: failure != "mapping", source: failure == "mapping" ? nil : "wait(9)\n",
                                             error: failure == "mapping" ? "unmapped generated action" : nil)
                },
                decompile: { _, _ in XCTFail("Unexpected decompile"); return "" },
                compile: { request in
                    if failure == "compile" { throw CompilationDiagnostic(message: "Rejected rebuild", line: 1, column: 1) }
                    return self.rebuilt(CherriEditorBuildRequest(source: "wrong", mode: request.mode, name: request.name))
                }
            )
            do {
                _ = try await transaction.perform(edit, operations: operations,
                                                  isCurrent: { state.current(transaction) }, publish: state.publish)
                XCTFail("Failure must be reported: \(failure)")
            } catch {
                XCTAssertFalse(error.localizedDescription.isEmpty)
            }
            XCTAssertEqual(state.publications, 0, failure)
            XCTAssertEqual(state.compiled.id, transaction.original.id, failure)
            XCTAssertEqual(state.request, transaction.original.sourceRequest, failure)
            XCTAssertEqual(state.signedURL, signed, failure)
        }
    }

    func testEachSnapshotChangeDuringEitherAwaitRejectsPublication() async throws {
        let mutations: [(String, @MainActor (State) -> Void)] = [
            ("source", { $0.request = CherriEditorBuildRequest(source: "wait(7)", mode: $0.request.mode, name: $0.request.name, buildProfile: .release) }),
            ("source-utf16", { $0.request = CherriEditorBuildRequest(source: $0.request.source.decomposedStringWithCanonicalMapping, mode: $0.request.mode, name: $0.request.name, buildProfile: .release) }),
            ("mode", { $0.request = CherriEditorBuildRequest(source: $0.request.source, mode: "v2", name: $0.request.name, buildProfile: .release) }),
            ("name-utf16", { $0.request = CherriEditorBuildRequest(source: $0.request.source, mode: $0.request.mode, name: $0.request.name.decomposedStringWithCanonicalMapping, buildProfile: .release) }),
            ("profile", { $0.request = CherriEditorBuildRequest(source: $0.request.source, mode: $0.request.mode, name: $0.request.name, buildProfile: .debug) }),
            ("build-generation", { $0.buildGeneration += 1 }),
            ("edit-generation", { $0.editGeneration += 1 }),
            ("compiled-identity", { $0.compiled = self.rebuilt($0.request) })
        ]
        for stage in ["mapping", "rebuild"] {
            for (name, mutate) in mutations {
                let state = State()
                let transaction = state.transaction
                let signed = state.signedURL
                var builds = 0
                let operations = CherriPreviewEditTransaction.Operations(
                    editSource: { _, _, _, _ in
                        await Task.yield()
                        if stage == "mapping" { mutate(state) }
                        return CherriSourceEditResponse(ok: true, source: "wait(9)\n", error: nil)
                    },
                    decompile: { _, _ in XCTFail("Unexpected decompile"); return "" },
                    compile: { request in
                        builds += 1
                        await Task.yield()
                        if stage == "rebuild" { mutate(state) }
                        return self.rebuilt(request)
                    }
                )
                let published = try await transaction.perform(edit, operations: operations,
                                                              isCurrent: { state.current(transaction) }, publish: state.publish)
                XCTAssertFalse(published, "\(stage)/\(name)")
                XCTAssertEqual(state.publications, 0, "\(stage)/\(name)")
                XCTAssertEqual(state.signedURL, signed, "\(stage)/\(name)")
                XCTAssertEqual(builds, stage == "mapping" ? 0 : 1, "\(stage)/\(name)")
            }
        }
    }

    func testCancellationAfterMappingDoesNotRebuildOrPublish() async throws {
        let state = State()
        let transaction = state.transaction
        let task = Task { @MainActor in
            let operations = CherriPreviewEditTransaction.Operations(
                editSource: { _, _, _, _ in
                    withUnsafeCurrentTask { $0?.cancel() }
                    return CherriSourceEditResponse(ok: true, source: "wait(9)", error: nil)
                },
                decompile: { _, _ in XCTFail("Unexpected decompile"); return "" },
                compile: { request in XCTFail("Cancelled edit must not rebuild"); return self.rebuilt(request) }
            )
            return try await transaction.perform(self.edit, operations: operations,
                                                  isCurrent: { state.current(transaction) }, publish: state.publish)
        }
        let published = try await task.value
        XCTAssertFalse(published)
        XCTAssertEqual(state.publications, 0)
        XCTAssertEqual(state.compiled.id, transaction.original.id)
        XCTAssertNotNil(state.signedURL)
    }

    func testLegacyDecompileFailureDoesNotPublishLocalPlistMutation() async throws {
        let request = CherriEditorBuildRequest(source: "legacy source", mode: "legacy", name: "Legacy")
        let plist = try PropertyListSerialization.data(fromPropertyList: ["WFWorkflowActions": []], format: .xml, options: 0)
        let original = CompiledShortcut(name: "Legacy", actualMode: "legacy", plist: plist, signedShortcut: Data([1]),
                                        sourceRequest: request, previewSourceMap: nil)
        let transaction = CherriPreviewEditTransaction(original: original, buildGeneration: 0, editGeneration: 0)
        var publications = 0
        let operations = CherriPreviewEditTransaction.Operations(
            editSource: { _, _, _, _ in XCTFail("Legacy must not use V2 mapping"); return CherriSourceEditResponse(ok: false, source: nil, error: "unexpected") },
            decompile: { edited, _ in
                XCTAssertNotEqual(edited, original.plist)
                throw CompilationDiagnostic(message: "Decompile failed", line: 1, column: 1)
            },
            compile: { request in XCTFail("Failed decompile must not rebuild"); return self.rebuilt(request) }
        )
        do {
            _ = try await transaction.perform(.workflowType(value: "QuickActions", enabled: true), operations: operations,
                                              isCurrent: { true }, publish: { _, _ in publications += 1 })
            XCTFail("Expected decompile rejection")
        } catch { XCTAssertEqual(publications, 0) }
        XCTAssertEqual(original.plist, plist)
        XCTAssertEqual(original.signedShortcut, Data([1]))
    }

    func testLiveTaskIdentityResumesChangedSourceAfterPendingEdit() {
        let old = CherriEditorBuildRequest(source: "wait(2)", mode: "v2", name: "Preview")
        let changed = CherriEditorBuildRequest(source: "wait(3)", mode: "v2", name: "Preview")
        let paused = CherriLivePreviewRequest(request: changed, isApplyingPreviewEdit: true)
        let resumed = CherriLivePreviewRequest(request: changed, isApplyingPreviewEdit: false)
        XCTAssertNotEqual(paused, resumed)
        XCTAssertNotEqual(old, resumed.request, "The committed old preview must not suppress the resumed source build")
    }

    func testQueuedDOMMessageCannotAcquireNewRenderIdentity() {
        var state = CherriPreviewRenderState()
        let old = UUID()
        let new = UUID()
        let first = state.begin(old)
        state.complete(old, generation: first)
        XCTAssertTrue(state.accepts(old, committedID: old))
        XCTAssertFalse(state.accepts(nil, committedID: old))
        XCTAssertFalse(state.accepts(UUID(), committedID: old))
        let second = state.begin(new)
        XCTAssertFalse(state.accepts(old, committedID: new))
        state.complete(old, generation: first) // Late completion cannot bind old callback.
        XCTAssertFalse(state.accepts(old, committedID: new))
        XCTAssertFalse(state.accepts(new, committedID: new), "Pending render must not accept edits")
        state.complete(new, generation: second)
        XCTAssertFalse(state.accepts(old, committedID: new), "Queued old message keeps its old UUID")
        XCTAssertTrue(state.accepts(new, committedID: new))
        state.invalidate() // Receiving an edit forces committed DOM restoration.
        XCTAssertFalse(state.accepts(new, committedID: new))
        let restored = state.begin(new)
        state.complete(new, generation: restored)
        XCTAssertTrue(state.accepts(new, committedID: new))
    }

    func testRealBridgeMapsExpandedNativeWaitAndPreservesExactSource() async throws {
        let source = "// Preserve the original comment: שלום 🍒\nif (true) {\n\twait(1)\n}\n\n// Only the next authored Wait parameter may change.\nwait(2) // editable call\nwait(3)\nwait(4)\nwait(5)\nwait(6)\nwait(7)\nwait(8)\n// Preserve the final comment and all unrelated calls.\n"
        let compiled = try await CherriCompiler.compile(source: source, name: "  RP41Preview  ", mode: "auto")
        let map = try XCTUnwrap(compiled.previewSourceMap)
        XCTAssertEqual(map.version, 1)
        XCTAssertEqual(map.name, compiled.name)
        XCTAssertEqual(map.mode, "v2")
        XCTAssertEqual(compiled.actualMode, "v2")
        XCTAssertEqual(compiled.name, "RP41Preview")
        XCTAssertEqual(compiled.sourceRequest.name, "  RP41Preview  ")
        let actions = try nativeActions(compiled.plist)
        let catalog = try await CherriCompiler.actionCatalog()
        let waitIdentifier = try XCTUnwrap(catalog.first { $0.name == "wait" }?.shortcutIdentifier)
        let selected = actions.indices.filter {
            actions[$0]["WFWorkflowActionIdentifier"] as? String == waitIdentifier
                && ((actions[$0]["WFWorkflowActionParameters"] as? [String: Any])?["WFDelayTime"] as? NSNumber)?.doubleValue == 2
        }
        XCTAssertEqual(selected.count, 1)
        let index = try XCTUnwrap(selected.first)
        XCTAssertNotEqual(index, 1, "Exercise native-index/AST-statement divergence")
        let response = try await CherriCompiler.editPreviewSource(compiled: compiled, actionIndex: index,
                                                                  parameterKey: "WFDelayTime", newExpression: "9")
        XCTAssertTrue(response.ok, response.error ?? "")
        let updated = try XCTUnwrap(response.source)
        let expected = source.replacingOccurrences(of: "wait(2)", with: "wait(9)")
        XCTAssertEqual(Array(updated.utf8), Array(expected.utf8))
        let rebuilt = try await CherriCompiler.compile(source: updated, name: compiled.sourceRequest.name, mode: compiled.sourceRequest.mode)
        let after = try nativeActions(rebuilt.plist)
        XCTAssertEqual(after.count, actions.count)
        for row in actions.indices {
            XCTAssertEqual(after[row]["WFWorkflowActionIdentifier"] as? String, actions[row]["WFWorkflowActionIdentifier"] as? String)
        }
        XCTAssertEqual(((after[index]["WFWorkflowActionParameters"] as? [String: Any])?["WFDelayTime"] as? NSNumber)?.doubleValue, 9)

        let mixed = CompiledShortcut(name: compiled.name, actualMode: compiled.actualMode, plist: rebuilt.plist, signedShortcut: nil,
                                      sourceRequest: compiled.sourceRequest, previewSourceMap: map)
        let rejected = try await CherriCompiler.editPreviewSource(compiled: mixed, actionIndex: index,
                                                                  parameterKey: "WFDelayTime", newExpression: "6")
        XCTAssertFalse(rejected.ok, "A map from another unsigned graph must fail closed")
        XCTAssertEqual(rejected.source, source)
    }

    func testShowResultConvenienceSelectionUsesUniqueCompilerEntry() async throws {
        let source = "// Header comment preserved\nshow(\"CanvasPreviewFixture\")\n// Footer comment preserved\n"
        let compiled = try await CherriCompiler.compile(source: source, name: "Canvas", mode: "v2")
        let map = try XCTUnwrap(compiled.previewSourceMap)
        let index = try XCTUnwrap(map.uniqueParameterIndex(actionIdentifier: "is.workflow.actions.showresult", wireKey: "Text"))
        let result = try await CherriCompiler.editPreviewSource(compiled: compiled, actionIndex: index,
                                                               parameterKey: "Text", newExpression: "\"VisualParamUpdated\"")
        XCTAssertTrue(result.ok, result.error ?? "")
        XCTAssertEqual(result.source, source.replacingOccurrences(of: "CanvasPreviewFixture", with: "VisualParamUpdated"))
        XCTAssertNil(map.uniqueParameterIndex(actionIdentifier: "is.workflow.actions.showresult", wireKey: "text"))
        let duplicated = CherriPreviewSourceMap(version: map.version, name: map.name, mode: map.mode,
                                                buildProfile: map.buildProfile, sourceSHA256: map.sourceSHA256,
                                                plistSHA256: map.plistSHA256, actionCount: map.actionCount,
                                                entries: map.entries + map.entries, integritySHA256: map.integritySHA256)
        XCTAssertNil(duplicated.uniqueParameterIndex(actionIdentifier: "is.workflow.actions.showresult", wireKey: "Text"))
    }

    func testAutoLegacyRoutesByActualModeAndKeepsRequestedAutoContext() async throws {
        // Existing RP52 public auto-Legacy fixture, unchanged.
        let source = "// cherri: legacy\nconst route = text(\"RP52_LEGACY_REFERENCE\")\nshow(route)\n"
        let compiled = try await CherriCompiler.compile(source: source, name: "  LegacyPreview  ", mode: "auto")
        XCTAssertEqual(compiled.actualMode, "legacy")
        XCTAssertEqual(compiled.sourceRequest.mode, "auto")
        XCTAssertNil(compiled.previewSourceMap)
        let transaction = CherriPreviewEditTransaction(original: compiled, buildGeneration: 0, editGeneration: 0)
        var result: CompiledShortcut?
        var publications = 0
        let published = try await transaction.perform(
            .workflowType(value: "QuickActions", enabled: true), operations: .live,
            isCurrent: { true }, publish: { _, rebuilt in result = rebuilt; publications += 1 }
        )
        XCTAssertTrue(published)
        XCTAssertEqual(publications, 1)
        let rebuilt = try XCTUnwrap(result)
        XCTAssertEqual(rebuilt.actualMode, "legacy")
        XCTAssertEqual(rebuilt.sourceRequest.mode, "auto")
        XCTAssertEqual(rebuilt.sourceRequest.name, "  LegacyPreview  ")
        let object = try PropertyListSerialization.propertyList(from: rebuilt.plist, options: [], format: nil)
        let types = try XCTUnwrap((object as? [String: Any])?["WFWorkflowTypes"] as? [String])
        XCTAssertTrue(types.contains("QuickActions"))
    }

    func testMultilineUnicodePreviewValueUsesCherriLiteralEscapes() async throws {
        let source = "show(\"Original\")\n"
        let compiled = try await CherriCompiler.compile(source: source, name: "Literal", mode: "v2")
        let index = try XCTUnwrap(compiled.previewSourceMap?.uniqueParameterIndex(
            actionIdentifier: "is.workflow.actions.showresult", wireKey: "Text"
        ))
        let value = "שלום 🍒 \"quoted\" \\ path\nnext\rline\ttab {literal braces}"
        let transaction = CherriPreviewEditTransaction(original: compiled, buildGeneration: 0, editGeneration: 0)
        var result: CompiledShortcut?
        let published = try await transaction.perform(
            .actionParameter(actionIndex: index, key: "Text", value: .string(value)), operations: .live,
            isCurrent: { true }, publish: { _, rebuilt in result = rebuilt }
        )
        XCTAssertTrue(published)
        let rebuilt = try XCTUnwrap(result)
        let actions = try nativeActions(rebuilt.plist)
        let parameters = try XCTUnwrap(actions[index]["WFWorkflowActionParameters"] as? [String: Any])
        let actual = try XCTUnwrap(parameters["Text"] as? String)
        XCTAssertEqual(Array(actual.utf8), Array(value.utf8))
    }

    private func nativeActions(_ plist: Data) throws -> [[String: Any]] {
        let object = try PropertyListSerialization.propertyList(from: plist, options: [], format: nil)
        return try XCTUnwrap((object as? [String: Any])?["WFWorkflowActions"] as? [[String: Any]])
    }
}
