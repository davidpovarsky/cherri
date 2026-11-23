import Foundation

/// Owns one immutable compilation and both publication generations. It never
/// updates visible state until the source edit and its rebuild have succeeded.
struct CherriPreviewEditTransaction {
    let original: CompiledShortcut
    let buildGeneration: UInt64
    let editGeneration: Int

    func matches(compiled: CompiledShortcut?, request: CherriEditorBuildRequest,
                 buildGeneration: UInt64, editGeneration: Int) -> Bool {
        compiled?.id == original.id
            && request == original.sourceRequest
            && buildGeneration == self.buildGeneration
            && editGeneration == self.editGeneration
    }

    @MainActor
    struct Operations {
        let editSource: (CompiledShortcut, Int, String, String) async throws -> CherriSourceEditResponse
        let decompile: (Data, String) async throws -> String
        let compile: (CherriEditorBuildRequest) async throws -> CompiledShortcut

        static var live: Self {
            Self(
                editSource: { try await CherriCompiler.editPreviewSource(compiled: $0, actionIndex: $1, parameterKey: $2, newExpression: $3) },
                decompile: { try await CherriCompiler.decompile(plist: $0, name: $1) },
                compile: {
                    try await CherriCompiler.compile(
                        source: $0.source, name: $0.name, mode: $0.mode, buildProfile: $0.buildProfile
                    )
                }
            )
        }
    }

    /// The current-state check and publication are synchronous on MainActor.
    /// Every suspension is followed by a fresh check; there is no await between
    /// the final check and the caller's single source/preview/signature commit.
    @MainActor
    func perform(_ edit: ShortcutPreviewEdit, operations: Operations,
                 isCurrent: () -> Bool,
                 publish: (String, CompiledShortcut) -> Void) async throws -> Bool {
        guard !Task.isCancelled, isCurrent() else { return false }
        let source: String
        if original.actualMode == "legacy" {
            let edited = try ShortcutPlistEditor.applying(edit, to: original.plist)
            let decompiled = try await operations.decompile(edited, original.name)
            // Semantic Legacy decompilation need not emit a dialect directive.
            // Preserve its already selected dialect when rebuilding a raw Auto request.
            source = original.sourceRequest.mode == "auto"
                ? "// cherri: legacy\n" + decompiled : decompiled
        } else {
            guard case .actionParameter(let index, let key, let value) = edit else {
                throw CompilationDiagnostic(
                    message: "Visual metadata edit cannot safely update v2 source directly. Source preserved.",
                    line: 1, column: 1
                )
            }
            let response = try await operations.editSource(original, index, key, Self.expression(for: value))
            guard !Task.isCancelled, isCurrent() else { return false }
            guard response.ok, let updated = response.source else {
                throw CompilationDiagnostic(
                    message: "Visual edit cannot safely update this source (\(response.error ?? "unsupported parameter")). Source preserved.",
                    line: 1, column: 1
                )
            }
            source = updated
        }
        guard !Task.isCancelled, isCurrent() else { return false }
        let request = CherriEditorBuildRequest(
            source: source, mode: original.sourceRequest.mode,
            name: original.sourceRequest.name, buildProfile: original.sourceRequest.buildProfile
        )
        let rebuilt = try await operations.compile(request)
        guard !Task.isCancelled, isCurrent() else { return false }
        guard rebuilt.sourceRequest == request, rebuilt.actualMode == original.actualMode else {
            throw CompilationDiagnostic(message: "Rebuilt preview does not match the edited source request.", line: 1, column: 1)
        }
        publish(source, rebuilt)
        return true
    }

    private static func expression(for value: ShortcutPreviewValue) -> String {
        switch value {
        case .string(let value):
            // Match syntax/format_literals.go's Cherri vocabulary. JSON and
            // Go %q can emit \b, \f, \x etc. which this lexer does not decode.
            var literal = "\""
            for scalar in value.unicodeScalars {
                switch scalar.value {
                case 0x5c: literal += "\\\\"
                case 0x22: literal += "\\\""
                case 0x0a: literal += "\\n"
                case 0x0d: literal += "\\r"
                case 0x09: literal += "\\t"
                case 0..<0x20, 0x7f: literal += String(format: "\\u%04x", scalar.value)
                default: literal.unicodeScalars.append(scalar)
                }
            }
            return literal + "\""
        case .integer(let i):
            return "\(i)"
        case .real(let r):
            return "\(r)"
        case .bool(let b):
            return b ? "true" : "false"
        }
    }
}
