import Foundation
import CryptoKit
import XCTest
@testable import Cherri

final class DocumentationIndexTests: XCTestCase {
    func testD03BundledCurrentDocumentationSearch() throws {
        let index = try DocumentationIndex.load()
        let bundleURL = try XCTUnwrap(Bundle.main.url(forResource: "documentation-index",
                                                       withExtension: "json", subdirectory: "Documentation"))
        let bundleBytes = try Data(contentsOf: bundleURL)
        let bundleSHA256 = SHA256.hash(data: bundleBytes).map { String(format: "%02x", $0) }.joined()
        XCTAssertGreaterThan(index.sections.count, 500)
        XCTAssertEqual(Set(index.sourceFiles.map(\.path)), DocumentationIndex.sourcePaths)
        let exact = index.search("show")
        XCTAssertEqual(exact.first?.title, "show")
        XCTAssertEqual(exact.first?.sourcePath, "fork-language/actions.md")
        XCTAssertTrue(exact.first?.blocks.contains { $0.kind == "code" && $0.text.contains("show(") } ?? false)
        XCTAssertFalse(index.search("").contains { $0.migrationOnly })
        XCTAssertFalse(index.search("Migration", includeMigration: true).isEmpty)
        XCTAssertTrue(index.search("@count").isEmpty)
        let legacyMatches = index.search("@count", includeMigration: true)
        XCTAssertEqual(legacyMatches.count, 1)
        XCTAssertEqual(legacyMatches.first?.sourcePath, "fork-language/migration.md")
        XCTAssertTrue(legacyMatches.first?.migrationOnly == true)
        XCTAssertTrue(legacyMatches.first?.blocks.contains {
            $0.language == "cherri-legacy" && $0.text.contains("@count")
        } ?? false)
        XCTAssertTrue(index.search("ZZZ_MISSING_DOCUMENTATION_937").isEmpty)
        let record: [String: Any] = ["schemaVersion": "1.0",
                                     "bundleSHA256": bundleSHA256,
                                     "docsCommit": index.docsCommit,
                                     "compilerCommit": index.compilerCommit,
                                     "schemaFingerprint": index.schemaFingerprint,
                                     "macroFingerprint": index.macroFingerprint,
                                     "sourcePaths": index.sourceFiles.map(\.path),
                                     "sourceHashes": index.sourceFiles.map(\.sha256),
                                     "sections": index.sections.count,
                                     "exactActionFirst": exact.first?.title ?? "",
                                     "defaultMigrationResults": index.search("").filter(\.migrationOnly).count,
                                     "explicitMigrationResults": index.search("", includeMigration: true).filter(\.migrationOnly).count,
                                     "missingResults": index.search("ZZZ_MISSING_DOCUMENTATION_937").count]
        let data = try JSONSerialization.data(withJSONObject: record, options: [.sortedKeys])
        print("[CHERRI_DOCS_RECEIPT] \(String(decoding: data, as: UTF8.self))")
    }

    func testD03RejectsHistoricalAndDamagedBundleRows() throws {
        let url = try XCTUnwrap(Bundle.main.url(forResource: "documentation-index",
                                                withExtension: "json", subdirectory: "Documentation"))
        let bytes = try Data(contentsOf: url)
        var original = try XCTUnwrap(JSONSerialization.jsonObject(with: bytes) as? [String: Any])
        var sections = try XCTUnwrap(original["sections"] as? [[String: Any]])
        sections[0]["sourcePath"] = "language/obsolete.md"
        original["sections"] = sections
        XCTAssertThrowsError(try DocumentationIndex.decode(JSONSerialization.data(withJSONObject: original)))
        original = try XCTUnwrap(JSONSerialization.jsonObject(with: bytes) as? [String: Any])
        original["docsCommit"] = "main"
        XCTAssertThrowsError(try DocumentationIndex.decode(JSONSerialization.data(withJSONObject: original)))
        original = try XCTUnwrap(JSONSerialization.jsonObject(with: bytes) as? [String: Any])
        sections = try XCTUnwrap(original["sections"] as? [[String: Any]])
        sections.append(sections[0])
        original["sections"] = sections
        XCTAssertThrowsError(try DocumentationIndex.decode(JSONSerialization.data(withJSONObject: original)))
    }
}
