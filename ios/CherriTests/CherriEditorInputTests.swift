import UIKit
import Darwin
import XCTest
@testable import CodeEditorView
@testable import Cherri

@MainActor
private final class PasteTimingObservationBuffer {
    var nativeChangeCount = 0
    var receipts: [String] = []
}

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

    private func recordInputReceipt(_ view: CodeView, fixture: String, stage: String,
                                    keyIndex: Int, key: String = "", pasteElapsedMilliseconds: Int? = nil,
                                    bufferingTo buffer: PasteTimingObservationBuffer? = nil) {
        let separators = CharacterSet.alphanumerics.union(CharacterSet(charactersIn: "_")).inverted
        let method = name.components(separatedBy: separators).first { $0.hasPrefix("test") } ?? "unknown"
        var receipt: [String: Any] = [
            "schemaVersion": "1.0", "testName": "CherriEditorInputTests/\(method)",
            "isolatedSyntheticEditor": true, "fixtureId": fixture, "stage": stage,
            "keyIndex": keyIndex, "inputUTF16": Array(key.utf16).map(Int.init),
            "actualSource": view.text ?? "", "actualUTF16Length": ((view.text ?? "") as NSString).length,
            "selectionLocation": view.selectedRange.location, "selectionLength": view.selectedRange.length,
        ]
        if fixture == "single-character-paste", let elapsed = pasteElapsedMilliseconds {
            let allowed = ["", "text(\"x\")", "text(\"x\"))", "text(\"x\")))", "text(\"x\"))))"]
            if !allowed.contains(view.text ?? "") {
                receipt["actualSource"] = "[REDACTED_UNEXPECTED_CONTENT]"
            }
            receipt["pasteElapsedMilliseconds"] = min(120_000, max(0, elapsed))
            receipt["viewIsFirstResponder"] = view.isFirstResponder
            receipt["windowAttached"] = view.window != nil
            receipt["windowIsKey"] = view.window?.isKeyWindow == true
        }
        if let bytes = try? JSONSerialization.data(withJSONObject: receipt, options: [.sortedKeys]),
           let json = String(data: bytes, encoding: .utf8) {
            if let buffer {
                if buffer.receipts.count < 5 { buffer.receipts.append(json) }
            } else {
                print("[CHERRI_INPUT_RECEIPT] \(json)")
                fflush(stdout)
            }
        }
    }

    private func assertExactKeyboardFixture(_ source: String, fixture: String,
                                            file: StaticString = #filePath, line: UInt = #line) {
        let view = editor()
        XCTAssertEqual(view.text, "", "Synthetic editor must start empty", file: file, line: line)
        guard view.text.isEmpty else { return }
        for (index, character) in source.enumerated() {
            let key = String(character)
            recordInputReceipt(view, fixture: fixture, stage: "before-key", keyIndex: index, key: key)
            view.insertText(key)
            recordInputReceipt(view, fixture: fixture, stage: "after-key", keyIndex: index, key: key)
        }
        recordInputReceipt(view, fixture: fixture, stage: "fixture-complete", keyIndex: source.count)
        XCTAssertEqual(Array(view.text.utf16), Array(source.utf16), file: file, line: line)
    }

    func testNestedGeneratedClosersAndCompleteRealInput() {
        assertExactKeyboardFixture("text(text(\"x\"))\n", fixture: "nested-call")
    }

    func testListGeneratedClosersAndCompleteRealInput() {
        assertExactKeyboardFixture("let xs = [1, 2]\n", fixture: "list")
    }

    func testDictionaryGeneratedClosersAndCompleteRealInput() {
        assertExactKeyboardFixture("let d = {\"value\": 1}\n", fixture: "dictionary")
    }

    func testCurlyBlockGeneratedClosersAndCompleteRealInput() {
        assertExactKeyboardFixture("if true {\ntext(\"x\")\n}\n", fixture: "curly-block")
    }

    func testCommentGeneratedClosersAndCompleteRealInput() {
        assertExactKeyboardFixture("/* block */\ntext(\"x\")\n", fixture: "block-comment")
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
        guard UIPasteboard.general.string == ")" else {
            XCTFail("The isolated synthetic pasteboard write must be retained before paste")
            return
        }
        let startedAt = ProcessInfo.processInfo.systemUptime
        let elapsedMilliseconds: @Sendable () -> Int = {
            Int(min(120_000, max(0, ProcessInfo.processInfo.systemUptime - startedAt) * 1_000))
        }
        // Capture the real edit notification and predicate-match boundary. These
        // observations never fulfill or replace the original three-second wait.
        let observations = PasteTimingObservationBuffer()
        let changeObserver = NotificationCenter.default.addObserver(
            forName: UITextView.textDidChangeNotification, object: view, queue: nil
        ) { [weak self, weak view] _ in
            guard Thread.isMainThread else { return }
            MainActor.assumeIsolated {
                guard let self, let view, observations.nativeChangeCount < 4 else { return }
                observations.nativeChangeCount += 1
                self.recordInputReceipt(view, fixture: "single-character-paste", stage: "paste-native-text-change",
                    keyIndex: observations.nativeChangeCount, key: ")", pasteElapsedMilliseconds: elapsedMilliseconds(),
                    bufferingTo: observations)
            }
        }
        defer { NotificationCenter.default.removeObserver(changeObserver) }
        let pasted = expectation(for: NSPredicate(format: "text == %@", "text(\"x\"))"), evaluatedWith: view) { [weak self, weak view] in
            if Thread.isMainThread {
                MainActor.assumeIsolated {
                    if let self, let view {
                        self.recordInputReceipt(view, fixture: "single-character-paste", stage: "paste-predicate-match",
                            keyIndex: 0, key: ")", pasteElapsedMilliseconds: elapsedMilliseconds(),
                            bufferingTo: observations)
                    }
                }
            }
            return true
        }
        recordInputReceipt(view, fixture: "single-character-paste", stage: "before-paste",
                           keyIndex: 0, key: ")", pasteElapsedMilliseconds: elapsedMilliseconds())
        view.paste(nil)
        recordInputReceipt(view, fixture: "single-character-paste", stage: "after-paste-call",
                           keyIndex: 1, key: ")", pasteElapsedMilliseconds: elapsedMilliseconds())
        await fulfillment(of: [pasted], timeout: 3)
        // Avoid log I/O inside UIKit notification and XCTest predicate callbacks.
        for json in observations.receipts { print("[CHERRI_INPUT_RECEIPT] \(json)") }
        fflush(stdout)
        recordInputReceipt(view, fixture: "single-character-paste", stage: "after-paste-wait",
                           keyIndex: 2, key: ")", pasteElapsedMilliseconds: elapsedMilliseconds())
        XCTAssertEqual(view.text, "text(\"x\"))")
    }
}
