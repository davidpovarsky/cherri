import Foundation
import XCTest
@testable import Cherri

final class CherriCompletionTextEditTests: XCTestCase {
    private func item(_ start: CherriWirePosition, _ end: CherriWirePosition, _ text: String) -> CherriCompletionItem {
        CherriCompletionItem(label: "height", kind: 2, textEdit: CherriTextEditItem(
            range: CherriWireRange(start: start, end: end), newText: text
        ))
    }

    func testActualWireFieldDecodesWithoutInventingLegacyReplacement() throws {
        let raw = Data(#"{"label":"height","kind":2,"textEdit":{"range":{"start":{"line":0,"character":35},"end":{"line":0,"character":43}},"newText":"height: "}}"#.utf8)
        let decoded = try JSONDecoder().decode(CherriCompletionItem.self, from: raw)
        XCTAssertEqual(decoded.textEdit?.range.start.character, 35)
        XCTAssertEqual(decoded.textEdit?.range.end.character, 43)
        XCTAssertEqual(decoded.textEdit?.newText, "height: ")
        XCTAssertNil(decoded.insertText)
        let old = try JSONDecoder().decode(CherriCompletionItem.self,
            from: Data(#"{"label":"height","kind":2,"insertText":"height: "}"#.utf8))
        XCTAssertNil(old.textEdit)
    }

    func testSharedSpanReplacesSuffixAndExistingColonExactly() {
        let source = "resizeImage(Clipboard, width: 800, heighx: 600)\n"
        let request = CherriCompletionRequest(source: source, selection: NSRange(location: 37, length: 0))
        let suggestion = item(.init(line: 0, character: 35), .init(line: 0, character: 43), "height: ")
        XCTAssertEqual(request.applying(suggestion, to: request),
                       "resizeImage(Clipboard, width: 800, height: 600)\n")
    }

    func testUTF16AndCRLFCoordinatesPreserveUneditedHebrewAndEmoji() {
        let source = "// שָׁ 🌍\r\nlet שלום = \"ערך\"\r\nsho"
        let request = CherriCompletionRequest(source: source, selection: NSRange(location: source.utf16.count, length: 0))
        let suggestion = item(.init(line: 2, character: 0), .init(line: 2, character: 3), "show(<#input#>)")
        XCTAssertEqual(request.applying(suggestion, to: request),
                       "// שָׁ 🌍\r\nlet שלום = \"ערך\"\r\nshow(<#input#>)")
    }

    func testInvalidOutOfBoundsReversedAndNonCaretRangesAreRejected() {
        let request = CherriCompletionRequest(source: "show", selection: NSRange(location: 2, length: 0))
        for suggestion in [
            item(.init(line: -1, character: 0), .init(line: 0, character: 3), "x"),
            item(.init(line: 0, character: -1), .init(line: 0, character: 3), "x"),
            item(.init(line: 0, character: 0), .init(line: 0, character: 5), "x"),
            item(.init(line: 0, character: 0), .init(line: 1, character: 0), "x"),
            item(.init(line: 0, character: 3), .init(line: 0, character: 1), "x"),
            item(.init(line: 0, character: 3), .init(line: 0, character: 4), "x"),
        ] {
            XCTAssertNil(request.applying(suggestion, to: request))
        }
        XCTAssertEqual(request.source, "show")
    }

    func testSurrogatePairCannotBeSplitByEitherBoundary() {
        let request = CherriCompletionRequest(source: "🌍x", selection: NSRange(location: 2, length: 0))
        XCTAssertNil(request.applying(item(.init(line: 0, character: 1), .init(line: 0, character: 3), "x"), to: request))
        XCTAssertNil(request.applying(item(.init(line: 0, character: 0), .init(line: 0, character: 1), "x"), to: request))
    }

    func testCanonicalUnicodeEqualityDoesNotReuseOldUTF16Snapshot() {
        let old = "// e\u{301}\nsho"
        let new = "// \u{e9}\nsho"
        XCTAssertEqual(old, new)
        let before = CherriCompletionRequest(source: old, selection: NSRange(location: old.utf16.count, length: 0))
        let current = CherriCompletionRequest(source: new, selection: NSRange(location: new.utf16.count, length: 0))
        let suggestion = item(.init(line: 1, character: 0), .init(line: 1, character: 3), "show(<#input#>)")
        XCTAssertNil(before.applying(suggestion, to: current))
    }

    func testMovedCaretAndSelectionInvalidateResponse() {
        let request = CherriCompletionRequest(source: "sho", selection: NSRange(location: 3, length: 0))
        let suggestion = item(.init(line: 0, character: 0), .init(line: 0, character: 3), "show(<#input#>)")
        for selection in [NSRange(location: 2, length: 0), NSRange(location: 0, length: 3)] {
            XCTAssertNil(request.applying(suggestion, to: CherriCompletionRequest(source: "sho", selection: selection)))
        }
    }

    func testMissingSharedEditCannotFallBackToIndependentInsertion() {
        let request = CherriCompletionRequest(source: "sho", selection: NSRange(location: 3, length: 0))
        let legacyShape = CherriCompletionItem(label: "show", kind: 3, insertText: "show(<#input#>)")
        XCTAssertNil(request.applying(legacyShape, to: request))
    }
}
