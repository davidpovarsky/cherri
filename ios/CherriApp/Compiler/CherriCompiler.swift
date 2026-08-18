import CherriCore
import Foundation

struct CompilationDiagnostic: Error, Identifiable, Equatable, Sendable {
    let id = UUID()
    let message: String
    let line: Int
    let column: Int

    var locationDescription: String {
        "\(line):\(column)"
    }
}

struct CompiledShortcut: Sendable {
    let name: String
    let plist: Data
    let signedShortcut: Data?
}

struct CherriActionParameter: Decodable, Hashable, Sendable {
    let name: String
    let type: String
    let optional: Bool?
    let infinite: Bool?
    let reference: Bool?
    let literal: Bool?
    let `enum`: String?
    let enumValues: [String]?
    let `default`: String?

    var displayType: String {
        let base = (`enum`?.isEmpty == false ? `enum` : nil) ?? type
        return (reference == true ? "&" : "") + base
    }

    var signature: String {
        var result = displayType
        if infinite == true { result += "..." }
        result += " "
        if optional == true { result += "?" }
        result += name
        if let defaultValue = `default`, !defaultValue.isEmpty {
            result += " = \(defaultValue)"
        }
        return result
    }
}

struct CherriActionInfo: Decodable, Identifiable, Hashable, Sendable {
    var id: String { name }

    let name: String
    let title: String?
    let description: String?
    let category: String?
    let subcategory: String?
    let parameters: [CherriActionParameter]?
    let outputType: String?
    let macOnly: Bool?
    let nonMacOnly: Bool?
    let minVersion: Double?
    let maxVersion: Double?

    var signature: String {
        let arguments = (parameters ?? []).map(\.signature).joined(separator: ", ")
        var value = "\(name)(\(arguments))"
        if let outputType, !outputType.isEmpty {
            value += ": \(outputType)"
        }
        return value
    }
}

private struct CompilerBridgeResponse: Decodable {
    let ok: Bool
    let name: String?
    let plistBase64: String?
    let signedBase64: String?
    let source: String?
    let error: String?
    let line: Int?
    let column: Int?
}

private struct ActionCatalogBridgeResponse: Decodable {
    let ok: Bool
    let actions: [CherriActionInfo]?
    let error: String?
}

enum CherriCompiler {
    static func compile(source: String, name: String, signed: Bool = false) async throws -> CompiledShortcut {
        try await Task.detached(priority: .userInitiated) {
            try callCompileBridge(source: source, name: name, signed: signed)
        }.value
    }

    static func decompile(plist: Data, name: String) async throws -> String {
        try await Task.detached(priority: .userInitiated) {
            try callDecompileBridge(plist: plist, name: name)
        }.value
    }

    static func actionCatalog() async throws -> [CherriActionInfo] {
        try await Task.detached(priority: .utility) {
            try callActionCatalogBridge()
        }.value
    }

    private static func callCompileBridge(source: String, name: String, signed: Bool) throws -> CompiledShortcut {
        let resultPointer: UnsafeMutablePointer<CChar>? = source.withCString { sourcePointer in
            name.withCString { namePointer in
                let mutableSource = UnsafeMutablePointer(mutating: sourcePointer)
                let mutableName = UnsafeMutablePointer(mutating: namePointer)
                return signed
                    ? CherriCompileSigned(mutableSource, mutableName)
                    : CherriCompile(mutableSource, mutableName)
            }
        }

        let response = try decodeBridgeResponse(resultPointer)

        guard let plistBase64 = response.plistBase64,
              let plist = Data(base64Encoded: plistBase64) else {
            throw CompilationDiagnostic(message: "Compiler response did not contain plist data.", line: 1, column: 1)
        }

        let signedData = response.signedBase64.flatMap { Data(base64Encoded: $0) }
        return CompiledShortcut(
            name: response.name ?? name,
            plist: plist,
            signedShortcut: signedData
        )
    }

    private static func callDecompileBridge(plist: Data, name: String) throws -> String {
        let base64 = plist.base64EncodedString()
        let resultPointer: UnsafeMutablePointer<CChar>? = base64.withCString { dataPointer in
            name.withCString { namePointer in
                CherriDecompilePlist(
                    UnsafeMutablePointer(mutating: dataPointer),
                    UnsafeMutablePointer(mutating: namePointer)
                )
            }
        }

        let response = try decodeBridgeResponse(resultPointer)
        guard let source = response.source else {
            throw CompilationDiagnostic(message: "Decompiler response did not contain Cherri source.", line: 1, column: 1)
        }
        return source
    }

    private static func callActionCatalogBridge() throws -> [CherriActionInfo] {
        guard let resultPointer = CherriActionCatalog() else {
            throw CompilationDiagnostic(message: "Cherri action catalog returned no response.", line: 1, column: 1)
        }
        defer { CherriFree(resultPointer) }

        let responseData = Data(String(cString: resultPointer).utf8)
        let response = try JSONDecoder().decode(ActionCatalogBridgeResponse.self, from: responseData)
        guard response.ok else {
            throw CompilationDiagnostic(
                message: response.error ?? "Unable to load Cherri actions.",
                line: 1,
                column: 1
            )
        }
        return response.actions ?? []
    }

    private static func decodeBridgeResponse(_ resultPointer: UnsafeMutablePointer<CChar>?) throws -> CompilerBridgeResponse {
        guard let resultPointer else {
            throw CompilationDiagnostic(message: "Cherri compiler returned no response.", line: 1, column: 1)
        }
        defer { CherriFree(resultPointer) }

        let responseData = Data(String(cString: resultPointer).utf8)
        let response = try JSONDecoder().decode(CompilerBridgeResponse.self, from: responseData)

        guard response.ok else {
            throw CompilationDiagnostic(
                message: response.error ?? "Unknown Cherri compiler error.",
                line: response.line ?? 1,
                column: response.column ?? 1
            )
        }
        return response
    }
}
