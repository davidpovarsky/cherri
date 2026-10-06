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
    let key: String?
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

    var isRequired: Bool {
        optional != true
    }
}

struct CherriAppIntent: Decodable, Hashable, Sendable {
    let name: String
    let bundleIdentifier: String
    let appIntentIdentifier: String
    let teamIdentifier: String?
    let requiresAppInstallation: Bool?
}

struct CherriActionInfo: Decodable, Identifiable, Hashable, Sendable {
    var id: String { name }

    let name: String
    let shortcutIdentifier: String?
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
    let compilerConstruct: Bool?
    let appIntent: CherriAppIntent?
    let emittedKeys: [String]?
    let catalogInsertionSnippet: String?

    enum CodingKeys: String, CodingKey {
        case name, shortcutIdentifier, title, description, category, subcategory
        case parameters, outputType, macOnly, nonMacOnly, minVersion, maxVersion
        case compilerConstruct, appIntent, emittedKeys
        case catalogInsertionSnippet = "insertionSnippet"
    }

    var signature: String {
        let arguments = (parameters ?? []).map(\.signature).joined(separator: ", ")
        var value = "\(name)(\(arguments))"
        if let outputType, !outputType.isEmpty {
            value += ": \(outputType)"
        }
        return value
    }

    // Cherri's Ask global is a valid “Ask Each Time” value for any action
    // parameter. Supplying it for required arguments gives the palette a safe,
    // runnable insertion without inventing fake type-specific values. Optional
    // positional parameters before a later required argument are represented by
    // nil so the later required slot keeps its correct position.
    // Prefers the shared catalog-provided insertionSnippet if present.
    var insertionSnippet: String {
        if let catalogInsertionSnippet, !catalogInsertionSnippet.isEmpty {
            return catalogInsertionSnippet
        }
        let parameters = parameters ?? []
        guard let lastRequired = parameters.lastIndex(where: { $0.isRequired }) else {
            return "\(name)()"
        }

        let arguments = parameters[...lastRequired].map { parameter in
            parameter.isRequired ? "Ask" : "nil"
        }
        return "\(name)(\(arguments.joined(separator: ", ")))"
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

struct CherriCompletionItem: Decodable, Identifiable, Hashable, Sendable {
    var id: String { "\(label)_\(kind)" }
    let label: String
    let kind: Int
    let detail: String?
    let documentation: String?
    let insertText: String?
}

struct CherriWirePosition: Decodable, Hashable, Sendable {
    let line: Int
    let character: Int
}

struct CherriWireRange: Decodable, Hashable, Sendable {
    let start: CherriWirePosition
    let end: CherriWirePosition
}

struct CherriWireDiagnostic: Decodable, Identifiable, Hashable, Sendable {
    var id: String { "\(code)_\(range.start.line)_\(range.start.character)_\(message)" }
    let code: String
    let severity: String
    let message: String
    let range: CherriWireRange
}

struct CherriAnalyzeResponse: Decodable, Sendable {
    let uri: String
    let version: Int
    let languageVersion: String
    let schemaFingerprint: String
    let diagnostics: [CherriWireDiagnostic]
    let valid: Bool
}

private struct CompleteBridgeResponse: Decodable {
    let uri: String?
    let version: Int?
    let languageVersion: String?
    let schemaFingerprint: String?
    let items: [CherriCompletionItem]?
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

    static func analyze(source: String) async throws -> CherriAnalyzeResponse {
        try await Task.detached(priority: .userInitiated) {
            try callAnalyzeBridge(source: source)
        }.value
    }

    static func complete(source: String, line: Int, column: Int) async throws -> [CherriCompletionItem] {
        try await Task.detached(priority: .userInitiated) {
            try callCompleteBridge(source: source, line: line, column: column)
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

    private static func callAnalyzeBridge(source: String) throws -> CherriAnalyzeResponse {
        let resultPointer: UnsafeMutablePointer<CChar>? = source.withCString { sourcePointer in
            CherriAnalyze(UnsafeMutablePointer(mutating: sourcePointer))
        }
        guard let resultPointer else {
            throw CompilationDiagnostic(message: "Cherri analyze returned no response.", line: 1, column: 1)
        }
        defer { CherriFree(resultPointer) }

        let responseData = Data(String(cString: resultPointer).utf8)
        return try JSONDecoder().decode(CherriAnalyzeResponse.self, from: responseData)
    }

    private static func callCompleteBridge(source: String, line: Int, column: Int) throws -> [CherriCompletionItem] {
        let resultPointer: UnsafeMutablePointer<CChar>? = source.withCString { sourcePointer in
            CherriComplete(UnsafeMutablePointer(mutating: sourcePointer), Int32(line), Int32(column))
        }
        guard let resultPointer else {
            throw CompilationDiagnostic(message: "Cherri complete returned no response.", line: 1, column: 1)
        }
        defer { CherriFree(resultPointer) }

        let responseData = Data(String(cString: resultPointer).utf8)
        let resp = try JSONDecoder().decode(CompleteBridgeResponse.self, from: responseData)
        return resp.items ?? []
    }
}
