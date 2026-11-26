import SwiftUI
import UIKit
import WebKit
import XCTest
@testable import Cherri

@MainActor
private final class PresentationDocument: ObservableObject {
    @Published var document: CherriDocument

    init(source: String) {
        document = CherriDocument(text: source)
    }
}

private struct CompactPresentationFixture: View {
    @ObservedObject var model: PresentationDocument

    var body: some View {
        WorkspaceView(document: $model.document, fileURL: nil)
            .environment(\.horizontalSizeClass, .compact)
    }
}

/// Component controls for the real SwiftUI/UIKit hierarchy. They do not grant
/// the existing editor UI or WebKit field-edit acceptance tiers.
@MainActor
final class CherriWorkspacePresentationTests: XCTestCase {
    @MainActor
    private struct Mounted {
        let window: UIWindow
        let previousKeyWindow: UIWindow?
        let controller: UIHostingController<AnyView>
        let defaults: UserDefaults
        let suite: String

        func close() {
            window.isHidden = true
            window.rootViewController = nil
            previousKeyWindow?.makeKeyAndVisible()
            defaults.removePersistentDomain(forName: suite)
        }
    }

    private func mount<V: View>(_ root: V, livePreview: Bool = true) throws -> Mounted {
        let scene = try XCTUnwrap(UIApplication.shared.connectedScenes.compactMap { $0 as? UIWindowScene }.first)
        let suite = "CherriWorkspacePresentationTests.\(UUID().uuidString)"
        let defaults = try XCTUnwrap(UserDefaults(suiteName: suite))
        defaults.set(livePreview, forKey: "Cherri.livePreview")
        let controller = UIHostingController(rootView: AnyView(root.defaultAppStorage(defaults)))
        let previous = scene.windows.first { $0.isKeyWindow }
        let window = UIWindow(windowScene: scene)
        window.rootViewController = controller
        window.makeKeyAndVisible()
        controller.view.layoutIfNeeded()
        return Mounted(window: window, previousKeyWindow: previous, controller: controller,
                       defaults: defaults, suite: suite)
    }

    private func descendants<T: UIView>(_ type: T.Type, in view: UIView) -> [T] {
        var matches: [T] = []
        if let match = view as? T { matches.append(match) }
        for child in view.subviews { matches.append(contentsOf: descendants(type, in: child)) }
        return matches
    }

    private func wait(_ seconds: TimeInterval, _ predicate: () -> Bool) async throws -> Bool {
        let deadline = ProcessInfo.processInfo.systemUptime + seconds
        repeat {
            if predicate(), ProcessInfo.processInfo.systemUptime <= deadline { return true }
            try await Task.sleep(for: .milliseconds(25))
        } while ProcessInfo.processInfo.systemUptime < deadline
        return false
    }

    private func hasWindowGeometry(_ view: UIView, in mounted: Mounted) -> Bool {
        guard view.window === mounted.window else { return false }
        let rect = view.convert(view.bounds, to: mounted.window)
        guard rect.origin.x.isFinite, rect.origin.y.isFinite,
              rect.width.isFinite, rect.height.isFinite,
              rect.width > 1, rect.height > 1 else { return false }
        return rect.intersection(mounted.window.bounds).width > 1
            && rect.intersection(mounted.window.bounds).height > 1
    }

    private func isNativeHitTarget(_ view: UIView, in mounted: Mounted) -> Bool {
        guard hasWindowGeometry(view, in: mounted) else { return false }
        let rect = view.convert(view.bounds, to: mounted.window).intersection(mounted.window.bounds)
        let point = CGPoint(x: rect.midX, y: rect.midY)
        guard let hit = mounted.window.hitTest(point, with: nil) else { return false }
        return hit === view || hit.isDescendant(of: view)
    }

    /// Records only values already read by the original short-circuit predicate.
    /// -1 means not evaluated; the two masks retain prior true/false observations.
    @MainActor
    private final class PaneTrace {
        let stage: String
        let started = ProcessInfo.processInfo.systemUptime
        var last: [Int]
        var seenTrue = 0
        var seenFalse = 0
        var polls = 0

        init(stage: String, terms: Int) {
            self.stage = stage
            last = Array(repeating: -1, count: terms)
        }

        func beginPoll() {
            polls += 1
            last = Array(repeating: -1, count: last.count)
        }

        func term(_ index: Int, _ value: Bool) -> Bool {
            last[index] = value ? 1 : 0
            if value { seenTrue |= 1 << index } else { seenFalse |= 1 << index }
            return value
        }
    }

    /// Synthetic diagnostics only. The snapshot does not re-run hit tests, query
    /// accessibility, read source/URLs, or issue JavaScript or responder actions.
    /// Window roles refer to these two fixture windows, never object addresses.
    /// Elapsed time starts at trace creation and includes the before-event
    /// snapshot; it is not the duration or an extension of the unchanged wait.
    private func observePane(_ trace: PaneTrace, phase: String, completed: Bool?,
                             mounted: Mounted, selector: UISegmentedControl,
                             editor: UITextView, web: WKWebView,
                             otherWindow: UIWindow? = nil, otherInput: UITextField? = nil) {
        func relation(_ window: UIWindow?) -> String {
            guard let window else { return "NONE" }
            if window === mounted.window { return "MOUNTED" }
            if let otherWindow, window === otherWindow { return "OTHER" }
            return "ELSEWHERE"
        }

        func responderViews(_ window: UIWindow?) -> [String: Any] {
            guard let window else { return ["role": "NO_WINDOW", "visited": 0, "matches": 0] }
            var queue: [UIView] = [window]
            var index = 0
            var matches = 0
            var role = "NONE_OBSERVED"
            var truncated = false
            while index < queue.count {
                let view = queue[index]
                index += 1
                if view.isFirstResponder {
                    matches += 1
                    if view === editor { role = "EDITOR" }
                    else if view === web { role = "WEB" }
                    else if view.isDescendant(of: web) { role = "WEB_DESCENDANT" }
                    else if let otherInput, view === otherInput { role = "OTHER_INPUT" }
                    else { role = "OTHER_VIEW" }
                }
                let children = view.subviews
                let remaining = 512 - queue.count
                if children.count > remaining { truncated = true }
                queue.append(contentsOf: children.prefix(remaining))
            }
            if truncated { role = "TRUNCATED" }
            else if matches > 1 { role = "MULTIPLE" }
            return ["role": role, "visited": index, "matches": matches]
        }

        let segment: String
        switch selector.selectedSegmentIndex {
        case UISegmentedControl.noSegment: segment = "NONE"
        case 0: segment = "CODE"
        case 1: segment = "PREVIEW"
        default: segment = "OTHER"
        }
        let progress = web.estimatedProgress
        let progressPermille = progress.isFinite && (0...1).contains(progress)
            ? Int((progress * 1000).rounded()) : -1
        let state: [String: Any] = [
            "selectedSegment": segment,
            "mountedKey": mounted.window.isKeyWindow,
            "otherPresent": otherWindow != nil,
            "otherKey": otherWindow?.isKeyWindow ?? false,
            "selectorWindow": relation(selector.window),
            "editorWindow": relation(editor.window),
            "webWindow": relation(web.window),
            "otherInputWindow": relation(otherInput?.window),
            "editorFirstResponder": editor.isFirstResponder,
            "otherInputFirstResponder": otherInput?.isFirstResponder ?? false,
            "webFirstResponder": web.isFirstResponder,
            "editorInteractionEnabled": editor.isUserInteractionEnabled,
            "webInteractionEnabled": web.isUserInteractionEnabled,
            "webAccessibilityHidden": web.accessibilityElementsHidden,
            "webLoading": web.isLoading,
            "webProgressPermille": progressPermille,
            "webHasURL": web.url != nil,
            "mountedResponderViews": responderViews(mounted.window),
            "otherResponderViews": responderViews(otherWindow),
        ]
        let row: [String: Any] = [
            "schema": "cherri-workspace-presentation-observation/1",
            "testId": "CherriWorkspacePresentationTests/testCompactPaneTransitionsKeepNativeViewsAndDocumentState",
            "diagnosticOnly": true, "stage": trace.stage, "phase": phase,
            "budgetMilliseconds": 3000,
            "elapsedMilliseconds": Int(max(0, ProcessInfo.processInfo.systemUptime - trace.started) * 1000),
            "waitCompleted": completed.map { $0 as Any } ?? NSNull(),
            "polls": trace.polls, "lastTerms": trace.last,
            "seenTrueMask": trace.seenTrue, "seenFalseMask": trace.seenFalse,
            "state": state,
        ]
        if let data = try? JSONSerialization.data(withJSONObject: row, options: [.sortedKeys]),
           data.count <= 4000, let text = String(data: data, encoding: .utf8) {
            print("[CHERRI_WORKSPACE_PRESENTATION] " + text)
        }
    }

    func testCompactPaneTransitionsKeepNativeViewsAndDocumentState() async throws {
        let source = "show(\"COMPACT_PRESENTATION_CONTROL\")\n"
        let model = PresentationDocument(source: source)
        let mounted = try mount(CompactPresentationFixture(model: model))
        defer { mounted.close() }
        let ready = try await wait(10) {
            let selectors = self.descendants(UISegmentedControl.self, in: mounted.controller.view)
            return selectors.count == 1 && selectors[0].numberOfSegments == 2
                && selectors[0].titleForSegment(at: 0) == "Code"
                && selectors[0].titleForSegment(at: 1) == "Preview"
                && !self.descendants(UITextView.self, in: mounted.controller.view).isEmpty
                && self.descendants(WKWebView.self, in: mounted.controller.view).count == 1
        }
        XCTAssertTrue(ready, "A successful live compilation must prepare one real compact preview without replacing the editor")
        guard ready else { return }
        let selector = try XCTUnwrap(descendants(UISegmentedControl.self, in: mounted.controller.view).first)
        let editor = try XCTUnwrap(descendants(UITextView.self, in: mounted.controller.view).first)
        let web = try XCTUnwrap(descendants(WKWebView.self, in: mounted.controller.view).first)
        XCTAssertEqual(selector.selectedSegmentIndex, 0)
        XCTAssertTrue(hasWindowGeometry(editor, in: mounted))
        XCTAssertTrue(isNativeHitTarget(editor, in: mounted))
        XCTAssertFalse(isNativeHitTarget(web, in: mounted))
        XCTAssertTrue(web.accessibilityElementsHidden)
        XCTAssertEqual(editor.text, source)
        XCTAssertTrue(editor.becomeFirstResponder())
        let originalSelection = NSRange(location: 6, length: 4)
        editor.selectedRange = originalSelection
        editor.delegate?.textViewDidChangeSelection?(editor)
        XCTAssertEqual(editor.selectedRange, originalSelection)
        XCTAssertTrue(editor.isFirstResponder)

        selector.selectedSegmentIndex = 1
        // A native control value alone must not satisfy the pane oracle. The
        // actual SwiftUI binding and window hit target change after its event.
        await Task.yield()
        XCTAssertTrue(isNativeHitTarget(editor, in: mounted))
        XCTAssertFalse(isNativeHitTarget(web, in: mounted))
        let previewTrace = PaneTrace(stage: "preview", terms: 7)
        observePane(previewTrace, phase: "before-event", completed: nil, mounted: mounted,
                    selector: selector, editor: editor, web: web)
        selector.sendActions(for: .valueChanged)
        let previewSelected = try await wait(3) {
            previewTrace.beginPoll()
            return previewTrace.term(0, selector.selectedSegmentIndex == 1)
                && previewTrace.term(1, self.hasWindowGeometry(web, in: mounted))
                && previewTrace.term(2, self.descendants(WKWebView.self, in: mounted.controller.view).first === web)
                && previewTrace.term(3, self.isNativeHitTarget(web, in: mounted))
                && previewTrace.term(4, !self.isNativeHitTarget(editor, in: mounted))
                && previewTrace.term(5, !editor.isFirstResponder)
                && previewTrace.term(6, !web.accessibilityElementsHidden)
        }
        observePane(previewTrace, phase: "after-wait", completed: previewSelected, mounted: mounted,
                    selector: selector, editor: editor, web: web)
        XCTAssertTrue(previewSelected, "The native segment event must select the real preview and exclude the hidden editor from touch and keyboard input")
        XCTAssertTrue(descendants(UITextView.self, in: mounted.controller.view).contains { $0 === editor })
        XCTAssertEqual(model.document.text, source)
        XCTAssertEqual(editor.text, source)

        selector.selectedSegmentIndex = 0
        let codeTrace = PaneTrace(stage: "code", terms: 6)
        observePane(codeTrace, phase: "before-event", completed: nil, mounted: mounted,
                    selector: selector, editor: editor, web: web)
        selector.sendActions(for: .valueChanged)
        let codeSelected = try await wait(3) {
            codeTrace.beginPoll()
            return codeTrace.term(0, selector.selectedSegmentIndex == 0)
                && codeTrace.term(1, self.hasWindowGeometry(editor, in: mounted))
                && codeTrace.term(2, self.descendants(UITextView.self, in: mounted.controller.view).contains { $0 === editor })
                && codeTrace.term(3, self.isNativeHitTarget(editor, in: mounted))
                && codeTrace.term(4, !self.isNativeHitTarget(web, in: mounted))
                && codeTrace.term(5, web.accessibilityElementsHidden)
        }
        observePane(codeTrace, phase: "after-wait", completed: codeSelected, mounted: mounted,
                    selector: selector, editor: editor, web: web)
        XCTAssertTrue(codeSelected, "Returning to Code must retain the existing native editor and its geometry")
        XCTAssertTrue(descendants(WKWebView.self, in: mounted.controller.view).contains { $0 === web })
        XCTAssertEqual(editor.selectedRange, originalSelection)
        XCTAssertEqual(editor.text, source)
        XCTAssertEqual(model.document.text, source)

        // A pane change in this workspace must not dismiss another window's
        // keyboard. A global resignFirstResponder action would fail this case.
        do {
            let scene = try XCTUnwrap(mounted.window.windowScene)
            let otherController = UIViewController()
            let otherInput = UITextField(frame: CGRect(x: 20, y: 80, width: 240, height: 44))
            otherInput.text = "OTHER_WINDOW_INPUT"
            otherController.view.addSubview(otherInput)
            let otherWindow = UIWindow(windowScene: scene)
            otherWindow.rootViewController = otherController
            otherWindow.makeKeyAndVisible()
            defer {
                otherWindow.isHidden = true
                otherWindow.rootViewController = nil
                mounted.window.makeKeyAndVisible()
            }
            XCTAssertTrue(otherInput.becomeFirstResponder())
            for target in [1, 0] {
                selector.selectedSegmentIndex = target
                let otherTrace = PaneTrace(stage: target == 1 ? "other-preview" : "other-code", terms: 3)
                observePane(otherTrace, phase: "before-event", completed: nil, mounted: mounted,
                            selector: selector, editor: editor, web: web,
                            otherWindow: otherWindow, otherInput: otherInput)
                selector.sendActions(for: .valueChanged)
                let changed = try await wait(3) {
                    otherTrace.beginPoll()
                    return otherTrace.term(0, self.isNativeHitTarget(target == 1 ? web : editor, in: mounted))
                        && otherTrace.term(1, !self.isNativeHitTarget(target == 1 ? editor : web, in: mounted))
                        && otherTrace.term(2, web.accessibilityElementsHidden == (target == 0))
                }
                observePane(otherTrace, phase: "after-wait", completed: changed, mounted: mounted,
                            selector: selector, editor: editor, web: web,
                            otherWindow: otherWindow, otherInput: otherInput)
                XCTAssertTrue(changed)
                XCTAssertTrue(otherInput.isFirstResponder)
                XCTAssertEqual(otherInput.text, "OTHER_WINDOW_INPUT")
                XCTAssertEqual(model.document.text, source)
            }
        }

        model.document.text = ""
        let cleared = try await wait(5) {
            self.descendants(UITextView.self, in: mounted.controller.view).contains { $0 === editor }
                && editor.text.isEmpty
                && self.descendants(WKWebView.self, in: mounted.controller.view).isEmpty
        }
        XCTAssertTrue(cleared, "Clearing compact source must remove its stale compiled WebKit instance without replacing the editor")
    }

    func testCompactUncompiledDocumentsDoNotInstantiateWebKit() async throws {
        for (source, livePreview) in [("", true), ("show(\"MANUAL_COMPACT_CONTROL\")\n", false)] {
            let model = PresentationDocument(source: source)
            let mounted = try mount(CompactPresentationFixture(model: model), livePreview: livePreview)
            defer { mounted.close() }
            let ready = try await wait(5) {
                !self.descendants(UITextView.self, in: mounted.controller.view).isEmpty
            }
            XCTAssertTrue(ready)
            try await Task.sleep(for: .milliseconds(1000))
            XCTAssertTrue(descendants(WKWebView.self, in: mounted.controller.view).isEmpty)
            XCTAssertEqual(model.document.text, source)

            let editor = try XCTUnwrap(descendants(UITextView.self, in: mounted.controller.view).first)
            let selector = try XCTUnwrap(descendants(UISegmentedControl.self, in: mounted.controller.view).first)
            XCTAssertTrue(editor.becomeFirstResponder())
            selector.selectedSegmentIndex = 1
            selector.sendActions(for: .valueChanged)
            let previewSelected = try await wait(3) {
                selector.selectedSegmentIndex == 1
                    && !self.isNativeHitTarget(editor, in: mounted)
                    && !editor.isFirstResponder
            }
            XCTAssertTrue(previewSelected)
            XCTAssertTrue(descendants(UITextView.self, in: mounted.controller.view).contains { $0 === editor })
            XCTAssertTrue(descendants(WKWebView.self, in: mounted.controller.view).isEmpty)
            XCTAssertEqual(model.document.text, source)
        }
    }

    func testDocumentationSearchUsesLiteralNativeInputTraits() async throws {
        let mounted = try mount(DocumentationSearchView())
        defer { mounted.close() }
        let ready = try await wait(5) {
            let fields = self.descendants(UISearchTextField.self, in: mounted.controller.view)
            return fields.count == 1 && self.hasWindowGeometry(fields[0], in: mounted)
        }
        XCTAssertTrue(ready, "The real documentation search field must be mounted before inspecting its native traits")
        guard ready else { return }
        let field = try XCTUnwrap(descendants(UISearchTextField.self, in: mounted.controller.view).first)
        XCTAssertEqual(field.autocapitalizationType, .none)
        XCTAssertEqual(field.autocorrectionType, .no)
    }
}
