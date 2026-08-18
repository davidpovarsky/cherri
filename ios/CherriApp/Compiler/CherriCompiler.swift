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

private struct CompilerBridgeResponse: Decodable {
    let ok: Bool
    let name: String?
    let plistBase64: String?
    let signedBase64: String?
    let error: String?
    let line: Int?
    let column: Int?
}

enum CherriCompiler {
    static func compile(source: String, name: String, signed: Bool = false) async throws -> CompiledShortcut {
        try await Task.detached(priority: .userInitiated) {
            try callBridge(source: source, name: name, signed: signed)
        }.value
    }

    private static func callBridge(source: String, name: String, signed: Bool) throws -> CompiledShortcut {
        let resultPointer: UnsafeMutablePointer<CChar>? = source.withCString { sourcePointer in
            name.withCString { namePointer in
                let mutableSource = UnsafeMutablePointer(mutating: sourcePointer)
                let mutableName = UnsafeMutablePointer(mutating: namePointer)
                return signed
                    ? CherriCompileSigned(mutableSource, mutableName)
                    : CherriCompile(mutableSource, mutableName)
            }
        }

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

        guard let plistBase64 = response.plistBase64,
              let plist = Data(base64Encoded: plistBase64) else {
            throw CompilationDiagnostic(message: "Compiler response did not contain plist data.", line: 1, column: 1)
        }

        let signedData = response.signedBase64.flatMap(Data.init(base64Encoded:))
        return CompiledShortcut(
            name: response.name ?? name,
            plist: plist,
            signedShortcut: signedData
        )
    }
}
