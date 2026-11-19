import Foundation

struct DocumentationBlock: Decodable, Hashable {
    let kind: String
    let language: String
    let text: String
}

struct DocumentationSection: Decodable, Identifiable, Hashable {
    let id: String
    let sourcePath: String
    let sourceLine: Int
    let documentTitle: String
    let title: String
    let migrationOnly: Bool
    let blocks: [DocumentationBlock]

    var searchText: String {
        ([title, documentTitle] + blocks.map(\.text)).joined(separator: "\n")
    }
}

struct DocumentationIndex: Decodable {
    struct SourceFile: Decodable {
        let path: String
        let sha256: String
    }

    let formatVersion: Int
    let languageVersion: String
    let docsCommit: String
    let compilerCommit: String
    let schemaFingerprint: String
    let macroFingerprint: String
    let sourceFiles: [SourceFile]
    let sections: [DocumentationSection]

    static let sourcePaths: Set<String> = [
        "fork-language/index.md", "fork-language/guide.md",
        "fork-language/actions.md", "fork-language/migration.md"
    ]

    static func load(bundle: Bundle = .main) throws -> DocumentationIndex {
        guard let url = bundle.url(forResource: "documentation-index", withExtension: "json",
                                   subdirectory: "Documentation") else {
            throw DocumentationError.unavailable
        }
        return try decode(Data(contentsOf: url))
    }

    static func decode(_ data: Data) throws -> DocumentationIndex {
        guard data.count <= 2_000_000 else { throw DocumentationError.invalid }
        let index = try JSONDecoder().decode(DocumentationIndex.self, from: data)
        guard index.formatVersion == 1, index.languageVersion == "2.0",
              isHex(index.docsCommit, length: 40), isHex(index.compilerCommit, length: 40),
              isHex(index.schemaFingerprint, length: 64), isHex(index.macroFingerprint, length: 64),
              index.sourceFiles.count == sourcePaths.count,
              Set(index.sourceFiles.map(\.path)) == sourcePaths,
              index.sourceFiles.allSatisfy({ isHex($0.sha256, length: 64) }),
              !index.sections.isEmpty,
              Set(index.sections.map(\.id)).count == index.sections.count,
              index.sections.allSatisfy({ section in
                  sourcePaths.contains(section.sourcePath) && section.sourceLine > 0 &&
                  section.id == "\(section.sourcePath):\(section.sourceLine)" &&
                  section.migrationOnly == (section.sourcePath == "fork-language/migration.md") &&
                  !section.title.isEmpty && !section.blocks.isEmpty &&
                  section.blocks.allSatisfy {
                      ["prose", "code"].contains($0.kind) &&
                      ($0.language != "cherri-legacy" || section.migrationOnly)
                  }
              }) else {
            throw DocumentationError.invalid
        }
        return index
    }

    func search(_ query: String, includeMigration: Bool = false) -> [DocumentationSection] {
        let words = Self.fold(query).split(whereSeparator: \.isWhitespace).map(String.init)
        let candidates = sections.filter { section in
            (includeMigration || !section.migrationOnly) &&
            words.allSatisfy { Self.fold(section.searchText).contains($0) }
        }
        guard !words.isEmpty else { return candidates }
        let exact = Self.fold(query.trimmingCharacters(in: .whitespacesAndNewlines))
        return candidates.enumerated().sorted { lhs, rhs in
            let left = Self.fold(lhs.element.title) == exact
            let right = Self.fold(rhs.element.title) == exact
            return left == right ? lhs.offset < rhs.offset : left
        }.map(\.element)
    }

    private static func fold(_ value: String) -> String {
        value.folding(options: [.caseInsensitive, .diacriticInsensitive],
                      locale: Locale(identifier: "en_US_POSIX"))
    }

    private static func isHex(_ value: String, length: Int) -> Bool {
        value.utf8.count == length && value.utf8.allSatisfy {
            (48...57).contains($0) || (97...102).contains($0)
        }
    }
}

enum DocumentationError: LocalizedError {
    case unavailable, invalid

    var errorDescription: String? {
        switch self {
        case .unavailable: return "The bundled documentation is unavailable."
        case .invalid: return "The bundled documentation could not be verified."
        }
    }
}
