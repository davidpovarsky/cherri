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

    enum CodingKeys: String, CodingKey {
        case uri, version, languageVersion, schemaFingerprint, diagnostics, valid
    }

    init(from decoder: Decoder) throws {
        let container = try decoder.container(keyedBy: CodingKeys.self)
        uri = try container.decode(String.self, forKey: .uri)
        version = try container.decode(Int.self, forKey: .version)
        languageVersion = try container.decode(String.self, forKey: .languageVersion)
        schemaFingerprint = try container.decode(String.self, forKey: .schemaFingerprint)
        diagnostics = try container.decodeIfPresent([CherriWireDiagnostic].self, forKey: .diagnostics) ?? []
        valid = try container.decode(Bool.self, forKey: .valid)
    }
}

struct CherriHoverResponse: Decodable, Sendable {
    let ok: Bool
    let contents: String?
    let range: CherriWireRange?
}

struct CherriDefinitionResponse: Decodable, Sendable {
    let ok: Bool
    let uri: String?
    let range: CherriWireRange?
}

struct CherriSignatureHelpResponse: Decodable, Sendable {
    let ok: Bool
    let label: String?
    let documentation: String?
    let parameters: [String]?
    let activeParameter: Int?
}

struct CherriTextEditItem: Decodable, Hashable, Sendable {
    let range: CherriWireRange
    let newText: String
}

struct CherriRenameResponse: Decodable, Sendable {
    let ok: Bool
    let source: String?
    let edits: [CherriTextEditItem]?
    let error: String?
}

struct CherriFormatResponse: Decodable, Sendable {
    let ok: Bool
    let source: String?
    let error: String?
}

struct CherriSourceEditResponse: Decodable, Sendable {
    let ok: Bool
    let source: String?
    let error: String?
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
    static func compile(source: String, name: String, mode: String = "auto", signed: Bool = false) async throws -> CompiledShortcut {
        try await Task.detached(priority: .userInitiated) {
            try callCompileBridge(source: source, name: name, mode: mode, signed: signed)
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

    static func analyzeVersioned(uri: String = "file:///mobile.cherri", version: Int, source: String) async throws -> CherriAnalyzeResponse {
        try await Task.detached(priority: .userInitiated) {
            try callAnalyzeVersionedBridge(uri: uri, version: version, source: source)
        }.value
    }

    static func complete(source: String, line: Int, column: Int) async throws -> [CherriCompletionItem] {
        try await Task.detached(priority: .userInitiated) {
            try callCompleteBridge(source: source, line: line, column: column)
        }.value
    }

    static func hover(source: String, line: Int, column: Int) async throws -> CherriHoverResponse {
        try await Task.detached(priority: .userInitiated) {
            try callHoverBridge(source: source, line: line, column: column)
        }.value
    }

    static func definition(source: String, line: Int, column: Int) async throws -> CherriDefinitionResponse {
        try await Task.detached(priority: .userInitiated) {
            try callDefinitionBridge(source: source, line: line, column: column)
        }.value
    }

    static func signatureHelp(source: String, line: Int, column: Int) async throws -> CherriSignatureHelpResponse {
        try await Task.detached(priority: .userInitiated) {
            try callSignatureHelpBridge(source: source, line: line, column: column)
        }.value
    }

    static func rename(source: String, line: Int, column: Int, newName: String) async throws -> CherriRenameResponse {
        try await Task.detached(priority: .userInitiated) {
            try callRenameBridge(source: source, line: line, column: column, newName: newName)
        }.value
    }

    static func format(source: String) async throws -> String {
        try await Task.detached(priority: .userInitiated) {
            try callFormatBridge(source: source)
        }.value
    }

    static func editSourceNode(source: String, actionIndex: Int, paramLabel: String, newExpr: String) async throws -> CherriSourceEditResponse {
        try await Task.detached(priority: .userInitiated) {
            try callEditSourceNodeBridge(source: source, actionIndex: actionIndex, paramLabel: paramLabel, newExpr: newExpr)
        }.value
    }

    private static func callCompileBridge(source: String, name: String, mode: String, signed: Bool) throws -> CompiledShortcut {
        let resultPointer: UnsafeMutablePointer<CChar>? = source.withCString { sourcePointer in
            name.withCString { namePointer in
                mode.withCString { modePointer in
                    let mutableSource = UnsafeMutablePointer(mutating: sourcePointer)
                    let mutableName = UnsafeMutablePointer(mutating: namePointer)
                    let mutableMode = UnsafeMutablePointer(mutating: modePointer)
                    return signed
                        ? CherriCompileSignedMode(mutableSource, mutableName, mutableMode)
                        : CherriCompileMode(mutableSource, mutableName, mutableMode)
                }
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

    private static func callAnalyzeVersionedBridge(uri: String, version: Int, source: String) throws -> CherriAnalyzeResponse {
        let resultPointer: UnsafeMutablePointer<CChar>? = uri.withCString { uriPointer in
            source.withCString { sourcePointer in
                CherriAnalyzeVersioned(
                    UnsafeMutablePointer(mutating: uriPointer),
                    Int32(version),
                    UnsafeMutablePointer(mutating: sourcePointer)
                )
            }
        }
        guard let resultPointer else {
            throw CompilationDiagnostic(message: "Cherri analyzeVersioned returned no response.", line: 1, column: 1)
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

    private static func callHoverBridge(source: String, line: Int, column: Int) throws -> CherriHoverResponse {
        let resultPointer: UnsafeMutablePointer<CChar>? = source.withCString { sourcePointer in
            CherriHover(UnsafeMutablePointer(mutating: sourcePointer), Int32(line), Int32(column))
        }
        guard let resultPointer else {
            throw CompilationDiagnostic(message: "Cherri hover returned no response.", line: 1, column: 1)
        }
        defer { CherriFree(resultPointer) }

        let responseData = Data(String(cString: resultPointer).utf8)
        return try JSONDecoder().decode(CherriHoverResponse.self, from: responseData)
    }

    private static func callDefinitionBridge(source: String, line: Int, column: Int) throws -> CherriDefinitionResponse {
        let resultPointer: UnsafeMutablePointer<CChar>? = source.withCString { sourcePointer in
            CherriDefinition(UnsafeMutablePointer(mutating: sourcePointer), Int32(line), Int32(column))
        }
        guard let resultPointer else {
            throw CompilationDiagnostic(message: "Cherri definition returned no response.", line: 1, column: 1)
        }
        defer { CherriFree(resultPointer) }

        let responseData = Data(String(cString: resultPointer).utf8)
        return try JSONDecoder().decode(CherriDefinitionResponse.self, from: responseData)
    }

    private static func callSignatureHelpBridge(source: String, line: Int, column: Int) throws -> CherriSignatureHelpResponse {
        let resultPointer: UnsafeMutablePointer<CChar>? = source.withCString { sourcePointer in
            CherriSignatureHelp(UnsafeMutablePointer(mutating: sourcePointer), Int32(line), Int32(column))
        }
        guard let resultPointer else {
            throw CompilationDiagnostic(message: "Cherri signatureHelp returned no response.", line: 1, column: 1)
        }
        defer { CherriFree(resultPointer) }

        let responseData = Data(String(cString: resultPointer).utf8)
        return try JSONDecoder().decode(CherriSignatureHelpResponse.self, from: responseData)
    }

    private static func callRenameBridge(source: String, line: Int, column: Int, newName: String) throws -> CherriRenameResponse {
        let resultPointer: UnsafeMutablePointer<CChar>? = source.withCString { sourcePointer in
            newName.withCString { namePointer in
                CherriRename(
                    UnsafeMutablePointer(mutating: sourcePointer),
                    Int32(line),
                    Int32(column),
                    UnsafeMutablePointer(mutating: namePointer)
                )
            }
        }
        guard let resultPointer else {
            throw CompilationDiagnostic(message: "Cherri rename returned no response.", line: 1, column: 1)
        }
        defer { CherriFree(resultPointer) }

        let responseData = Data(String(cString: resultPointer).utf8)
        return try JSONDecoder().decode(CherriRenameResponse.self, from: responseData)
    }

    private static func callFormatBridge(source: String) throws -> String {
        let resultPointer: UnsafeMutablePointer<CChar>? = source.withCString { sourcePointer in
            CherriFormat(UnsafeMutablePointer(mutating: sourcePointer))
        }
        guard let resultPointer else {
            throw CompilationDiagnostic(message: "Cherri format returned no response.", line: 1, column: 1)
        }
        defer { CherriFree(resultPointer) }

        let responseData = Data(String(cString: resultPointer).utf8)
        let resp = try JSONDecoder().decode(CherriFormatResponse.self, from: responseData)
        guard resp.ok, let formatted = resp.source else {
            throw CompilationDiagnostic(message: resp.error ?? "Format failed.", line: 1, column: 1)
        }
        return formatted
    }

    private static func callEditSourceNodeBridge(source: String, actionIndex: Int, paramLabel: String, newExpr: String) throws -> CherriSourceEditResponse {
        let resultPointer: UnsafeMutablePointer<CChar>? = source.withCString { sourcePointer in
            paramLabel.withCString { labelPointer in
                newExpr.withCString { exprPointer in
                    CherriEditSourceNode(
                        UnsafeMutablePointer(mutating: sourcePointer),
                        Int32(actionIndex),
                        UnsafeMutablePointer(mutating: labelPointer),
                        UnsafeMutablePointer(mutating: exprPointer)
                    )
                }
            }
        }
        guard let resultPointer else {
            throw CompilationDiagnostic(message: "Cherri editSourceNode returned no response.", line: 1, column: 1)
        }
        defer { CherriFree(resultPointer) }

        let responseData = Data(String(cString: resultPointer).utf8)
        return try JSONDecoder().decode(CherriSourceEditResponse.self, from: responseData)
    }
}
