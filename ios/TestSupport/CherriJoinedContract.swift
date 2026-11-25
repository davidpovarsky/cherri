import CryptoKit
import Foundation
import XCTest

struct JoinedWirePosition: Decodable {
    let line: Int
    let character: Int
}

struct JoinedWireRange: Decodable {
    let start: JoinedWirePosition
    let end: JoinedWirePosition
}

struct JoinedExpectedDiagnostic: Decodable {
    let code: String
    let severity: String
    let range: JoinedWireRange
}

struct JoinedCompletion: Decodable {
    struct Edit: Decodable {
        let range: JoinedWireRange
        let newText: String
    }
    let label: String
    let caret: JoinedWirePosition
    let textEdit: Edit
}

struct JoinedFixture: Decodable {
    let source: String
    let sourceSha256: String
    let expectedSource: String
    let expectedSourceSha256: String
    let diagnostics: [JoinedExpectedDiagnostic]?
    let completion: JoinedCompletion?
}

/// Compiled into both native test bundles. It reads one source-controlled
/// contract resource and emits only fixed protocol tokens and synthetic hashes.
final class CherriJoinedContract {
    struct Stage: Decodable {
        let id: String
        let fixtureId: String
        let observedSourceField: String
    }
    struct Method: Decodable {
        let requirement: String
        let producer: String
        let checkpoint: String
        let stages: [Stage]
    }
    struct Model: Decodable {
        let schema: String
        let eventSchema: String
        let eventPrefix: String
        let fixtures: [String: JoinedFixture]
        let methods: [String: Method]
    }
    private enum Invalid: Error { case contract }
    let testId: String
    let model: Model
    let method: Method
    let contractSha256: String
    private var next = 0

    static func digest(_ source: String) -> String {
        digest(Data(source.utf8))
    }

    static func digest(_ data: Data) -> String {
        SHA256.hash(data: data).map { String(format: "%02x", $0) }.joined()
    }

    init(testId: String) throws {
        self.testId = testId
        let url = try XCTUnwrap(Bundle(for: CherriJoinedContract.self).url(
            forResource: "joined-editor-contract", withExtension: "json", subdirectory: "TestFixtures"
        ), "The actual test bundle must contain its shared source contract")
        let raw = try Data(contentsOf: url)
        let decoded = try JSONDecoder().decode(Model.self, from: raw)
        model = decoded
        contractSha256 = Self.digest(raw)
        method = try XCTUnwrap(decoded.methods[testId], "Only a dedicated authored test owns this contract")
        guard model.schema == "cherri-joined-editor-contract/1",
              model.eventSchema == "cherri-ios-joined-contract-event/1",
              model.eventPrefix == "[CHERRI_JOINED_CONTRACT] ",
              !method.stages.isEmpty else {
            XCTFail("Unexpected joined editor contract schema")
            throw Invalid.contract
        }
    }

    func fixture(_ id: String) throws -> JoinedFixture {
        let value = try XCTUnwrap(model.fixtures[id], "The exact public fixture must exist")
        guard Self.digest(value.source) == value.sourceSha256,
              Self.digest(value.expectedSource) == value.expectedSourceSha256 else {
            XCTFail("Joined fixture bytes do not match their authored hash")
            throw Invalid.contract
        }
        return value
    }

    func stage(_ id: String, inputSource: String, observedSource: String) throws {
        guard next < method.stages.count else {
            XCTFail("Unexpected extra joined contract stage")
            throw Invalid.contract
        }
        let expected = method.stages[next]
        let value = try fixture(expected.fixtureId)
        let wanted = expected.observedSourceField == "source" ? value.source : value.expectedSource
        guard expected.id == id,
              inputSource.utf16.elementsEqual(value.source.utf16),
              observedSource.utf16.elementsEqual(wanted.utf16) else {
            XCTFail("The ordered stage must observe the exact current source through its actual producer")
            throw Invalid.contract
        }
        let event: [String: Any] = [
            "schema": model.eventSchema, "testId": testId,
            "requirement": method.requirement, "producer": method.producer,
            "stage": id, "sequence": next, "fixtureId": expected.fixtureId,
            "inputSha256": Self.digest(inputSource),
            "observedSourceSha256": Self.digest(observedSource),
            "contractSha256": contractSha256,
        ]
        let raw = try JSONSerialization.data(withJSONObject: event, options: [.sortedKeys])
        print(model.eventPrefix + String(decoding: raw, as: UTF8.self))
        next += 1
    }

    func finish() throws {
        guard next == method.stages.count else {
            XCTFail("A joined native checkpoint requires the complete original stage sequence")
            throw Invalid.contract
        }
        print("[CHECKPOINT:\(method.checkpoint)]")
    }
}
