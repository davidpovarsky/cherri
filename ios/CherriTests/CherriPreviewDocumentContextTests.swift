import Foundation
import Combine
import CryptoKit
import XCTest
@testable import Cherri

/// Actual Swift -> C ABI -> shared compiler and OS file-notification producers.
/// These do not claim a Shortcut runtime or actual WebKit UI interaction.
@MainActor
final class CherriPreviewDocumentContextTests: XCTestCase {
    override func setUpWithError() throws { continueAfterFailure = false }

    private func directory() throws -> URL {
        let root = FileManager.default.temporaryDirectory
            .appendingPathComponent("CherriBRG28-\(UUID().uuidString)", isDirectory: true)
        try FileManager.default.createDirectory(at: root, withIntermediateDirectories: true)
        return root.resolvingSymlinksInPath()
    }

    private func context(_ entry: URL?, id: UUID = UUID(), revision: UInt64 = 0,
                         assetRevision: UInt64 = 0) -> CherriDocumentCompileContext {
        CherriDocumentCompileContext(uri: entry?.absoluteString ?? "", documentID: id,
                                     revision: revision, assetRevision: assetRevision)
    }

    private func request(_ source: String, mode: String = "v2",
                         document: CherriDocumentCompileContext) -> CherriEditorBuildRequest {
        CherriEditorBuildRequest(source: source, mode: mode, name: "BRG28Native",
                                document: document)
    }

    private func actions(_ compiled: CompiledShortcut) throws -> [[String: Any]] {
        let root = try PropertyListSerialization.propertyList(from: compiled.plist, options: [], format: nil)
        return try XCTUnwrap((root as? [String: Any])?["WFWorkflowActions"] as? [[String: Any]])
    }

    private func assertAsset(_ compiled: CompiledShortcut, _ payload: Data,
                             forbidden: Data? = nil) throws {
        let values = try actions(compiled).compactMap {
            ($0["WFWorkflowActionParameters"] as? [String: Any])?["WFTextActionText"] as? String
        }
        XCTAssertEqual(values.filter { $0 == payload.base64EncodedString() }.count, 1)
        if let forbidden { XCTAssertFalse(values.contains(forbidden.base64EncodedString())) }
        XCTAssertNil(compiled.signedShortcut)
    }

    func testBRG28SavedDocumentAssetsAndImportedOriginsAcrossBothDialects() async throws {
        let root = try directory()
        defer { try? FileManager.default.removeItem(at: root) }
        let entry = root.appendingPathComponent("entry שלום.cherri")
        let nested = root.appendingPathComponent("sub", isDirectory: true)
        try FileManager.default.createDirectory(at: nested, withIntermediateDirectories: true)
        let expected = Data("PUBLIC_NESTED_ORIGIN".utf8)
        let wrong = Data("PUBLIC_WRONG_ROOT".utf8)
        try expected.write(to: nested.appendingPathComponent("payload.dat"))
        try wrong.write(to: root.appendingPathComponent("payload.dat"))
        for mode in ["v2", "legacy"] {
            let source: String
            let helper: String
            if mode == "v2" {
                source = "import \"sub/helper.cherri\" as helper\nlet value = helper.load()\n"
                helper = "function load() -> AnyContent { return embedFile(\"payload.dat\") }\n"
            } else {
                source = "#include 'sub/helper.cherri'\nconst value = load()\n"
                helper = "function load(): text {\n    const data = embedFile(\"payload.dat\")\n    output(data)\n}\n"
            }
            try Data(helper.utf8).write(to: nested.appendingPathComponent("helper.cherri"))
            try Data(source.utf8).write(to: entry)
            let sourceRequest = request(source, mode: mode, document: context(entry))
            let compiled = try await CherriCompiler.compile(request: sourceRequest)
            XCTAssertEqual(compiled.actualMode, mode)
            XCTAssertEqual(compiled.name, sourceRequest.name)
            XCTAssertEqual(compiled.sourceRequest, sourceRequest)
            try assertAsset(compiled, expected, forbidden: wrong)
        }
    }

    func testBRG28ActualPreviewTransactionPreservesDocumentAndRejectsContextRelabels() async throws {
        let root = try directory()
        defer { try? FileManager.default.removeItem(at: root) }
        let entry = root.appendingPathComponent("entry.cherri")
        let source = "// keep café 😀\nwait(2)\nshow(\"keep\")\n"
        try Data(source.utf8).write(to: entry)
        var document = CherriDocument(text: source)
        let originalContext = document.compileContext(fileURL: entry, assetRevision: 7)
        let sourceRequest = request(source, document: originalContext)
        let compiled = try await CherriCompiler.compile(request: sourceRequest)
        let map = try XCTUnwrap(compiled.previewSourceMap)
        XCTAssertEqual(map.document, originalContext)
        let index = try XCTUnwrap(map.uniqueParameterIndex(actionIdentifier: "is.workflow.actions.delay",
                                                         wireKey: "WFDelayTime"))
        let contexts: [CherriDocumentCompileContext?] = [
            context(root.appendingPathComponent("other.cherri"), id: originalContext.documentID,
                    revision: originalContext.revision, assetRevision: 7),
            context(entry, revision: originalContext.revision, assetRevision: 7),
            context(entry, id: originalContext.documentID, revision: originalContext.revision + 1, assetRevision: 7),
            context(entry, id: originalContext.documentID, revision: originalContext.revision, assetRevision: 8),
            nil,
        ]
        for stale in contexts {
            let relabelled = CompiledShortcut(
                name: compiled.name, actualMode: compiled.actualMode, plist: compiled.plist,
                signedShortcut: nil,
                sourceRequest: CherriEditorBuildRequest(source: source, mode: "v2", name: sourceRequest.name,
                                                       document: stale),
                previewSourceMap: map
            )
            let response = try await CherriCompiler.editPreviewSource(
                compiled: relabelled, actionIndex: index, parameterKey: "WFDelayTime", newExpression: "3"
            )
            XCTAssertFalse(response.ok)
            XCTAssertEqual(Array(response.source?.utf8 ?? "".utf8), Array(source.utf8))
        }
        let transaction = CherriPreviewEditTransaction(original: compiled, buildGeneration: 1, editGeneration: 1)
        var rebuilt: CompiledShortcut?
        let published = try await transaction.perform(
            .actionParameter(actionIndex: index, key: "WFDelayTime", value: .integer(3)),
            operations: .live, isCurrent: { true },
            publish: { updated, result in
                document.text = updated
                rebuilt = result
            }
        )
        XCTAssertTrue(published)
        let result = try XCTUnwrap(rebuilt)
        XCTAssertEqual(Array(document.text.utf8), Array(source.replacingOccurrences(of: "wait(2)", with: "wait(3)").utf8))
        XCTAssertEqual(result.sourceRequest.document, document.compileContext(fileURL: entry, assetRevision: 7))
        XCTAssertEqual(result.previewSourceMap?.document, result.sourceRequest.document)
        XCTAssertNil(result.signedShortcut)

        let editedContext = document.compileContext(fileURL: entry, assetRevision: 7)
        document.text = source // A -> B -> A keeps a newer revision, not the old map identity.
        XCTAssertNotEqual(document.compileContext(fileURL: entry, assetRevision: 7), originalContext)
        XCTAssertEqual(document.compileContext(fileURL: entry, assetRevision: 7).revision,
                       editedContext.revision + 1)
        let other = CherriDocument(text: source)
        XCTAssertNotEqual(other.compileContext(fileURL: entry, assetRevision: 7).documentID,
                          originalContext.documentID)
    }

    func testBRG28UnsavedMissingAndEscapingResourcesFailThroughActualBridge() async throws {
        let root = try directory()
        defer { try? FileManager.default.removeItem(at: root) }
        let workspace = root.appendingPathComponent("workspace", isDirectory: true)
        try FileManager.default.createDirectory(at: workspace, withIntermediateDirectories: true)
        let entry = workspace.appendingPathComponent("entry.cherri")
        let asset = workspace.appendingPathComponent("payload.dat")
        let outside = root.appendingPathComponent("outside.dat")
        let payload = Data("PUBLIC_AUTHORIZED".utf8)
        try Data("PUBLIC_OUTSIDE".utf8).write(to: outside)
        for mode in ["v2", "legacy"] {
            try? FileManager.default.removeItem(at: asset)
            try payload.write(to: asset)
            let source = "embedFile(\"payload.dat\")\n"
            let good = try await CherriCompiler.compile(request: request(source, mode: mode, document: context(entry)))
            try assertAsset(good, payload)
            try FileManager.default.removeItem(at: asset)
            try await rejects(source, mode: mode, document: context(entry))
            try FileManager.default.createSymbolicLink(at: asset, withDestinationURL: outside)
            try await rejects(source, mode: mode, document: context(entry))
            try FileManager.default.removeItem(at: asset)
            try await rejects("embedFile(\"../outside.dat\")\n", mode: mode, document: context(entry))
            let absolute = "embedFile(\"\(outside.path)\")\n"
            try await rejects(absolute, mode: mode, document: context(nil))
            let plain = try await CherriCompiler.compile(request: request("show(\"unsaved\")\n", mode: mode,
                                                                         document: context(nil)))
            XCTAssertEqual(plain.actualMode, mode)
            XCTAssertNil(plain.signedShortcut)
        }
    }

    private func rejects(_ source: String, mode: String,
                         document: CherriDocumentCompileContext) async throws {
        do {
            _ = try await CherriCompiler.compile(request: request(source, mode: mode, document: document))
            XCTFail("The actual native resource boundary must reject this source.")
        } catch let error as CompilationDiagnostic {
            XCTAssertFalse(error.message.isEmpty)
        }
    }

    func testBRG28CoordinatedAssetNotificationsInvalidateSameSizeAndTimestamp() async throws {
        let root = try directory()
        defer { try? FileManager.default.removeItem(at: root) }
        let entry = root.appendingPathComponent("entry.cherri")
        let asset = root.appendingPathComponent("payload.dat")
        let first = Data(repeating: 0x41, count: 2 * 1024 * 1024)
        let second = Data(repeating: 0x42, count: first.count)
        let source = "embedFile(\"payload.dat\")\n"
        try Data(source.utf8).write(to: entry)
        try first.write(to: asset)
        // Foundation's Date -> file-attribute setter can round subsecond
        // timestamps. Establish a set-and-read filesystem baseline before the
        // cache is populated; the final exact mtime equality remains required.
        try FileManager.default.setAttributes(
            [.modificationDate: Date(timeIntervalSince1970: 1_700_000_000)],
            ofItemAtPath: asset.path
        )
        let timestamp = try XCTUnwrap(try FileManager.default.attributesOfItem(atPath: asset.path)[.modificationDate] as? Date)
        let observer = CherriProjectAssetObserver()
        defer { observer.stop() }
        try await observer.prepare(fileURL: entry)
        let revision = observer.revision
        let initialContext = context(entry, assetRevision: revision)
        let before = try await CherriCompiler.assetCacheMetrics()
        let compiled = try await CherriCompiler.compile(request: request(source, document: initialContext))
        try assertAsset(compiled, first)
        let cold = try await CherriCompiler.assetCacheMetrics()
        XCTAssertEqual(cold.reads, before.reads + 1)
        XCTAssertEqual(cold.encodes, before.encodes + 1)

        let entryExpectation = expectation(description: "Actual coordinated entry write notification")
        let entryCount = observer.entryNotifications
        let entryToken = observer.$entryNotifications
            .filter { $0 > entryCount }.prefix(1).sink { _ in entryExpectation.fulfill() }
        let changedSource = source + "// unrelated source edit\n"
        try await coordinatedWrite(Data(changedSource.utf8), to: entry)
        await fulfillment(of: [entryExpectation], timeout: 8)
        withExtendedLifetime(entryToken) {}
        try await observer.prepare(fileURL: entry)
        XCTAssertEqual(observer.revision, revision, "An entry-only edit must preserve heavy asset work.")
        let warmRequest = request(changedSource, document: context(entry, id: initialContext.documentID,
                                                                  revision: 1, assetRevision: observer.revision))
        try assertAsset(try await CherriCompiler.compile(request: warmRequest), first)
        let warm = try await CherriCompiler.assetCacheMetrics()
        XCTAssertEqual(warm.reads, cold.reads)
        XCTAssertEqual(warm.encodes, cold.encodes)
        XCTAssertGreaterThan(warm.hits, cold.hits)

        let assetExpectation = expectation(description: "Actual coordinated asset write notification")
        let assetCount = observer.assetNotifications
        let assetToken = observer.$assetNotifications
            .filter { $0 > assetCount }.prefix(1).sink { _ in assetExpectation.fulfill() }
        try await coordinatedWrite(second, to: asset, preservingTimestamp: timestamp)
        await fulfillment(of: [assetExpectation], timeout: 8)
        withExtendedLifetime(assetToken) {}
        try await observer.prepare(fileURL: entry)
        XCTAssertGreaterThan(observer.revision, revision)
        let changed = request(changedSource, document: context(entry, id: initialContext.documentID,
                                                              revision: 1, assetRevision: observer.revision))
        XCTAssertNotEqual(changed, warmRequest)
        try assertAsset(try await CherriCompiler.compile(request: changed), second, forbidden: first)
        let after = try await CherriCompiler.assetCacheMetrics()
        XCTAssertEqual(after.reads, warm.reads + 1)
        XCTAssertEqual(after.encodes, warm.encodes + 1)
        XCTAssertGreaterThan(after.invalidations, warm.invalidations)
        XCTAssertEqual(try FileManager.default.attributesOfItem(atPath: asset.path)[.size] as? Int, first.count)
        XCTAssertEqual(try FileManager.default.attributesOfItem(atPath: asset.path)[.modificationDate] as? Date, timestamp)
    }

    private func coordinatedWrite(_ data: Data, to url: URL,
                                  preservingTimestamp: Date? = nil) async throws {
        try await Task.detached(priority: .utility) {
            var coordinationError: NSError?
            var writeError: Error?
            NSFileCoordinator(filePresenter: nil).coordinate(writingItemAt: url, options: [],
                                                            error: &coordinationError) { target in
                do {
                    try data.write(to: target)
                    if let preservingTimestamp {
                        try FileManager.default.setAttributes([.modificationDate: preservingTimestamp], ofItemAtPath: target.path)
                    }
                } catch { writeError = error }
            }
            if let coordinationError { throw coordinationError }
            if let writeError { throw writeError }
        }.value
    }

    func testBRG28CancelledCompileCannotReturnAnArtifact() async throws {
        let source = "show(\"cancelled-before-native-call\")\n"
        let sourceRequest = request(source, document: context(nil))
        let task = Task { @MainActor in
            withUnsafeCurrentTask { $0?.cancel() }
            return try await CherriCompiler.compile(request: sourceRequest)
        }
        do {
            _ = try await task.value
            XCTFail("A cancelled request must not publish a native artifact.")
        } catch is CancellationError {
            // Exact scope: pre-entry cooperative cancellation. Existing preview
            // transaction tests separately cover cancellation during each await.
        }
    }

    func testBRG28NativeSmallLargeAndEmbeddedAssetMeasurements() async throws {
        let root = try directory()
        defer { try? FileManager.default.removeItem(at: root) }
        let entry = root.appendingPathComponent("benchmark.cherri")
        let asset = Data(repeating: 0x63, count: 2 * 1024 * 1024)
        try asset.write(to: root.appendingPathComponent("payload.dat"))
        let fixtures: [(String, String, Int)] = [
            ("small", "show(\"BRG28_SMALL\")\n", 1),
            ("large", (0..<200).map { "show(\"BRG28_LARGE_\($0)\")" }.joined(separator: "\n") + "\n", 200),
            ("embedded-2mib", "embedFile(\"payload.dat\")\n", 0),
        ]
        for (name, source, expectedShows) in fixtures {
            try Data(source.utf8).write(to: entry)
            _ = try await CherriCompiler.invalidateAssetCache()
            let before = try await CherriCompiler.assetCacheMetrics()
            var timings: [Double] = []
            var sourceHashes: [String] = []
            let documentID = UUID()
            for index in 0..<20 {
                let edited = source + "// measured unrelated revision \(index)\n"
                let sourceRequest = request(edited, document: context(entry, id: documentID, revision: UInt64(index)))
                let started = DispatchTime.now().uptimeNanoseconds
                let compiled = try await CherriCompiler.compile(request: sourceRequest)
                let elapsed = DispatchTime.now().uptimeNanoseconds - started
                timings.append(Double(elapsed) / 1_000_000)
                sourceHashes.append(Self.digest(Data(edited.utf8)))
                XCTAssertEqual(compiled.sourceRequest, sourceRequest)
                let output = try actions(compiled)
                XCTAssertEqual(output.filter { $0["WFWorkflowActionIdentifier"] as? String == "is.workflow.actions.showresult" }.count,
                               expectedShows)
                if name == "embedded-2mib" { try assertAsset(compiled, asset) }
            }
            let after = try await CherriCompiler.assetCacheMetrics()
            XCTAssertEqual(timings.count, 20)
            XCTAssertTrue(timings.allSatisfy { $0.isFinite && $0 > 0 })
            if name == "embedded-2mib" {
                XCTAssertEqual(after.reads, before.reads + 1)
                XCTAssertEqual(after.encodes, before.encodes + 1)
                XCTAssertGreaterThanOrEqual(after.hits, before.hits + 19)
            }
            let sorted = timings.sorted()
            let receipt: [String: Any] = [
                "schema": "cherri-native-preview-benchmark/1",
                "fixture": name, "sourceSha256": Self.digest(Data(source.utf8)),
                "sampleSourceSha256": sourceHashes,
                "assetSha256": name == "embedded-2mib" ? Self.digest(asset) : "",
                "sampleCount": timings.count, "totalCompileMilliseconds": timings,
                "p50Milliseconds": sorted[9], "p95Milliseconds": sorted[18],
                "osVersion": ProcessInfo.processInfo.operatingSystemVersionString,
                "simulatorModel": ProcessInfo.processInfo.environment["SIMULATOR_MODEL_IDENTIFIER"] ?? "physical-unreported",
                "mode": "v2", "buildProfile": "debug",
                "cacheReadsDelta": after.reads - before.reads,
                "cacheEncodesDelta": after.encodes - before.encodes,
                "cacheHitsDelta": after.hits - before.hits,
                "scope": "actual-swift-cabi-compilation-no-render-or-shortcut-runtime",
            ]
            let bytes = try JSONSerialization.data(withJSONObject: receipt, options: [.sortedKeys])
            print("CHERRI_NATIVE_PREVIEW_BENCHMARK \(String(decoding: bytes, as: UTF8.self))")
        }
    }

    private static func digest(_ data: Data) -> String {
        SHA256.hash(data: data).map { String(format: "%02x", $0) }.joined()
    }
}
