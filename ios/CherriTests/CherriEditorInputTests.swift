import UIKit
import XCTest
@testable import CodeEditorView
@testable import Cherri

/// Uses the actual pinned UITextView implementation, with ordinary keyboard
/// insertion. These tests do not reimplement its text-storage algorithm.
@MainActor
final class CherriEditorInputTests: XCTestCase {
    private func editor(source: String = "") -> CodeView {
        let view = CodeView(
            frame: CGRect(x: 0, y: 0, width: 320, height: 240),
            with: .cherri(), viewLayout: .standard, indentation: .standard,
            theme: .defaultLight, setText: { _ in }, setMessages: { _ in }
        )
        view.text = source
        view.selectedRange = NSRange(location: (source as NSString).length, length: 0)
        return view
    }

    private func type(_ source: String, into view: CodeView) {
        for character in source { view.insertText(String(character)) }
    }

    func testGeneratedCloserSurvivesUntilMatchingKeyAndIsConsumedOnce() {
        let view = editor()
        type("text(\"x\"", into: view)
        XCTAssertEqual(view.text, "text(\"x\")", "Automatic completion must remain enabled")
        XCTAssertEqual(view.selectedRange, NSRange(location: 8, length: 0))
        type(")", into: view)
        XCTAssertEqual(view.text, "text(\"x\")", "A typed match must consume the generated closer")
        XCTAssertEqual(view.selectedRange, NSRange(location: 9, length: 0))
        type(")", into: view)
        XCTAssertEqual(view.text, "text(\"x\"))", "The generated position may be consumed only once")
    }

    func testNestedGeneratedClosersAndCompleteRealInput() {
        for source in [
            "text(text(\"x\"))\n", "let xs = [1, 2]\n", "let d = {\"value\": 1}\n",
            "if true {\ntext(\"x\")\n}\n", "/* block */\ntext(\"x\")\n"
        ] {
            let view = editor()
            type(source, into: view)
            XCTAssertEqual(Array(view.text.utf16), Array(source.utf16))
        }
    }

    func testUnicodeUTF16OffsetsPreserveGeneratedCloser() {
        let view = editor()
        let source = "text(\"שלום שָׁלוֹם 👩‍💻\")\n"
        type(source, into: view)
        XCTAssertEqual(Array(view.text.utf16), Array(source.utf16))
    }

    func testAuthoredCloserIsNeverConsumed() {
        let view = editor(source: "text()")
        view.selectedRange = NSRange(location: 5, length: 0)
        type(")", into: view)
        XCTAssertEqual(view.text, "text())")
    }

    func testSelectionMovementInvalidatesGeneratedProvenance() {
        let view = editor()
        type("text(\"x\"", into: view)
        view.selectedRange = NSRange(location: 0, length: 0)
        view.selectedRange = NSRange(location: 8, length: 0)
        type(")", into: view)
        XCTAssertEqual(view.text, "text(\"x\"))")
    }

    func testDeletionInvalidatesGeneratedProvenance() {
        let view = editor()
        type("text(\"x\"", into: view)
        view.deleteBackward()
        type("\")", into: view)
        XCTAssertEqual(view.text, "text(\"x\"))")
    }

    func testMultiCharacterInsertionDoesNotConsumeGeneratedText() {
        let view = editor()
        type("text(\"x\"", into: view)
        view.insertText("))")
        XCTAssertEqual(view.text, "text(\"x\")))")
    }

    func testSingleCharacterPasteDoesNotConsumeGeneratedText() async throws {
        let scene = try XCTUnwrap(UIApplication.shared.connectedScenes.compactMap { $0 as? UIWindowScene }.first)
        let window = UIWindow(windowScene: scene)
        let controller = UIViewController()
        let view = editor()
        controller.view.addSubview(view)
        window.rootViewController = controller
        window.makeKeyAndVisible()
        XCTAssertTrue(view.becomeFirstResponder())
        defer {
            view.resignFirstResponder()
            window.isHidden = true
            window.rootViewController = nil
            UIPasteboard.general.items = []
        }
        type("text(\"x\"", into: view)
        // This is the disposable simulator's own synthetic clipboard content.
        UIPasteboard.general.string = ")"
        let pasted = expectation(for: NSPredicate(format: "text == %@", "text(\"x\"))"), evaluatedWith: view)
        view.paste(nil)
        await fulfillment(of: [pasted], timeout: 3)
        XCTAssertEqual(view.text, "text(\"x\"))")
    }
}
