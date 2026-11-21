import SwiftUI
import UIKit
import WebKit
import XCTest
@testable import Cherri

@MainActor
private final class PreviewWorkspaceDocument: ObservableObject {
    @Published var document: CherriDocument

    init(source: String) {
        document = CherriDocument(text: source)
    }
}

private struct PreviewWorkspaceFixture: View {
    @ObservedObject var model: PreviewWorkspaceDocument

    var body: some View {
        WorkspaceView(document: $model.document, fileURL: nil)
            .environment(\.horizontalSizeClass, .regular)
    }
}

/// Actual SwiftUI/UIKit view lifecycle controls. These do not replace any
/// editor UI acceptance assertion or claim to explain an XCTest AX failure.
@MainActor
final class CherriPreviewLifecycleTests: XCTestCase {
    @MainActor
    private struct MountedWorkspace {
        let window: UIWindow
        let previousKeyWindow: UIWindow?
        let controller: UIHostingController<AnyView>
        let defaults: UserDefaults
        let defaultsSuite: String

        func close() {
            window.isHidden = true
            window.rootViewController = nil
            previousKeyWindow?.makeKeyAndVisible()
            defaults.removePersistentDomain(forName: defaultsSuite)
        }
    }

    private func mount(_ model: PreviewWorkspaceDocument, livePreview: Bool = true) throws -> MountedWorkspace {
        let scene = try XCTUnwrap(UIApplication.shared.connectedScenes.compactMap { $0 as? UIWindowScene }.first)
        let suite = "CherriPreviewLifecycleTests.\(UUID().uuidString)"
        let defaults = try XCTUnwrap(UserDefaults(suiteName: suite))
        defaults.set(livePreview, forKey: "Cherri.livePreview")
        let root = AnyView(PreviewWorkspaceFixture(model: model).defaultAppStorage(defaults))
        let controller = UIHostingController(rootView: root)
        let previous = scene.windows.first { $0.isKeyWindow }
        let window = UIWindow(windowScene: scene)
        window.rootViewController = controller
        window.makeKeyAndVisible()
        controller.view.layoutIfNeeded()
        return MountedWorkspace(window: window, previousKeyWindow: previous, controller: controller,
                                defaults: defaults, defaultsSuite: suite)
    }

    private func descendants<T: UIView>(_ type: T.Type, in view: UIView) -> [T] {
        var result: [T] = []
        if let match = view as? T { result.append(match) }
        for child in view.subviews {
            result.append(contentsOf: descendants(type, in: child))
        }
        return result
    }

    private func waitForHierarchy(timeout: TimeInterval, _ predicate: () -> Bool) async throws -> Bool {
        let deadline = ProcessInfo.processInfo.systemUptime + timeout
        repeat {
            let matched = predicate()
            if matched && ProcessInfo.processInfo.systemUptime <= deadline { return true }
            try await Task.sleep(for: .milliseconds(25))
        } while ProcessInfo.processInfo.systemUptime < deadline
        return false
    }

    func testEmptyWorkspaceMountsEditorWithoutWebPreview() async throws {
        let model = PreviewWorkspaceDocument(source: "")
        let mounted = try mount(model)
        defer { mounted.close() }
        let editorMounted = try await waitForHierarchy(timeout: 5) {
            !self.descendants(UITextView.self, in: mounted.controller.view).isEmpty
        }
        XCTAssertTrue(editorMounted, "The absence check requires the real editor view to be mounted")
        // Cross the original 650ms live-build debounce without substituting a
        // ready flag. A blank document still must not create an invisible web view.
        try await Task.sleep(for: .milliseconds(1000))
        XCTAssertTrue(descendants(WKWebView.self, in: mounted.controller.view).isEmpty)
        XCTAssertEqual(model.document.text, "")
    }

    func testCompiledPreviewAppearsAndIsRemovedWhenSourceClears() async throws {
        let model = PreviewWorkspaceDocument(source: "text(\"PREVIEW_STARTUP_CONTROL\")\n")
        let mounted = try mount(model)
        defer { mounted.close() }
        let previewMounted = try await waitForHierarchy(timeout: 10) {
            !self.descendants(UITextView.self, in: mounted.controller.view).isEmpty
                && self.descendants(WKWebView.self, in: mounted.controller.view).count == 1
        }
        XCTAssertTrue(previewMounted, "Nonempty source must still compile into an actual WebKit preview")
        guard previewMounted else { return }
        model.document.text = ""
        let previewRemoved = try await waitForHierarchy(timeout: 5) {
            !self.descendants(UITextView.self, in: mounted.controller.view).isEmpty
                && self.descendants(WKWebView.self, in: mounted.controller.view).isEmpty
        }
        XCTAssertTrue(previewRemoved, "Clearing source must remove stale compiled preview while retaining the editor")
    }

    func testDisabledLivePreviewDoesNotCreateWebViewForUncompiledSource() async throws {
        let model = PreviewWorkspaceDocument(source: "text(\"MANUAL_BUILD_ONLY\")\n")
        let mounted = try mount(model, livePreview: false)
        defer { mounted.close() }
        let editorMounted = try await waitForHierarchy(timeout: 5) {
            !self.descendants(UITextView.self, in: mounted.controller.view).isEmpty
        }
        XCTAssertTrue(editorMounted)
        try await Task.sleep(for: .milliseconds(1000))
        XCTAssertTrue(descendants(WKWebView.self, in: mounted.controller.view).isEmpty)
        XCTAssertEqual(model.document.text, "text(\"MANUAL_BUILD_ONLY\")\n")
    }
}
