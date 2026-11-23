import Foundation
import SwiftUI
import WebKit

struct ShortcutPreviewView: UIViewRepresentable {
    let plist: Data?
    let name: String
    let compilationID: UUID
    var actionMetadataJSON: String?
    let onEdit: (ShortcutPreviewEdit) -> Void

    func makeCoordinator() -> Coordinator {
        Coordinator(parent: self)
    }

    func makeUIView(context: Context) -> WKWebView {
        let configuration = WKWebViewConfiguration()
        configuration.userContentController.add(context.coordinator, name: Coordinator.editMessageName)

        let webView = WKWebView(frame: .zero, configuration: configuration)
        webView.navigationDelegate = context.coordinator
        webView.isOpaque = false
        webView.backgroundColor = .clear
        webView.scrollView.backgroundColor = .clear

        guard let indexURL = Bundle.main.url(
            forResource: "index",
            withExtension: "html",
            subdirectory: "PreviewShortcut"
        ) else {
            context.coordinator.showError("Preview resources are missing from the app bundle.", in: webView)
            return webView
        }

        webView.loadFileURL(
            indexURL,
            allowingReadAccessTo: indexURL.deletingLastPathComponent()
        )
        return webView
    }

    func updateUIView(_ webView: WKWebView, context: Context) {
        context.coordinator.parent = self
        context.coordinator.registerActionMetadataIfNeeded(in: webView)
        context.coordinator.renderIfReady(in: webView)
    }

    static func dismantleUIView(_ uiView: WKWebView, coordinator: Coordinator) {
        coordinator.invalidateRendering()
        uiView.configuration.userContentController.removeScriptMessageHandler(forName: Coordinator.editMessageName)
    }

    final class Coordinator: NSObject, WKNavigationDelegate, WKScriptMessageHandler {
        static let editMessageName = "cherriPreviewEdit"

        var parent: ShortcutPreviewView
        private var isReady = false
        private var renderState = CherriPreviewRenderState()
        private var renderedOnEdit: ((ShortcutPreviewEdit) -> Void)?
        private var lastRegisteredMetadata: String?

        init(parent: ShortcutPreviewView) {
            self.parent = parent
        }

        func webView(_ webView: WKWebView, didFinish navigation: WKNavigation!) {
            isReady = true
            registerActionMetadataIfNeeded(in: webView)
            renderIfReady(in: webView)
        }

        // Shared Cherri catalog metadata flows into preview-shortcut's generic
        // fallback renderer so actions without a built-in definition still get
        // a titled card. Registration is idempotent per metadata payload.
        func registerActionMetadataIfNeeded(in webView: WKWebView) {
            guard let metadata = parent.actionMetadataJSON,
                  !metadata.isEmpty,
                  metadata != lastRegisteredMetadata else { return }

            let encoded = Self.javaScriptString(metadata)
            lastRegisteredMetadata = metadata
            webView.evaluateJavaScript(
                "window.registerCherriActionMetadata?.(JSON.parse(\(encoded)))"
            )
        }

        func webView(
            _ webView: WKWebView,
            didFail navigation: WKNavigation!,
            withError error: Error
        ) {
            showError("Preview navigation failed: \(error.localizedDescription)", in: webView)
        }

        func webView(
            _ webView: WKWebView,
            didFailProvisionalNavigation navigation: WKNavigation!,
            withError error: Error
        ) {
            showError("Preview failed to load: \(error.localizedDescription)", in: webView)
        }

        func webViewWebContentProcessDidTerminate(_ webView: WKWebView) {
            isReady = false
            invalidateRendering()
            webView.reload()
        }

        func userContentController(_ userContentController: WKUserContentController, didReceive message: WKScriptMessage) {
            guard message.name == Self.editMessageName, let webView = message.webView else { return }
            let payload = message.body as? [String: Any]
            let messageID = (payload?["compilationID"] as? String).flatMap(UUID.init(uuidString:))
            let callback = renderState.accepts(messageID, committedID: parent.compilationID) ? renderedOnEdit : nil
            let edit = payload.flatMap(Self.previewEdit)

            // Web editing has already changed the DOM. Restore committed bytes
            // even when the message is missing a context or belongs to an old
            // render. A later successful transaction supplies a new compilation.
            invalidateRendering()
            renderIfReady(in: webView)
            if let callback, let edit { callback(edit) }
        }

        private static func previewEdit(_ payload: [String: Any]) -> ShortcutPreviewEdit? {
            guard let kind = payload["kind"] as? String else { return nil }
            switch kind {
            case "workflowType":
                guard let value = payload["value"] as? String,
                      let enabled = boolValue(payload["enabled"]) else { return nil }
                return .workflowType(value: value, enabled: enabled)

            case "quickActionSurface":
                guard let value = payload["value"] as? String,
                      let enabled = boolValue(payload["enabled"]) else { return nil }
                return .quickActionSurface(value: value, enabled: enabled)

            case "actionParameter":
                guard let index = (payload["actionIndex"] as? NSNumber)?.intValue,
                      let key = payload["key"] as? String,
                      let valueType = payload["valueType"] as? String else { return nil }

                let value: ShortcutPreviewValue?
                switch valueType {
                case "string":
                    value = (payload["value"] as? String).map(ShortcutPreviewValue.string)
                case "integer":
                    value = (payload["value"] as? NSNumber).map { .integer($0.intValue) }
                case "real":
                    value = (payload["value"] as? NSNumber).map { .real($0.doubleValue) }
                case "bool":
                    value = boolValue(payload["value"]).map(ShortcutPreviewValue.bool)
                default:
                    value = nil
                }
                guard let value else { return nil }
                return .actionParameter(actionIndex: index, key: key, value: value)

            default:
                return nil
            }
        }

        func invalidateRendering() {
            renderState.invalidate()
            renderedOnEdit = nil
        }

        func renderIfReady(in webView: WKWebView) {
            guard isReady else { return }

            guard let plist = parent.plist else {
                invalidateRendering()
                webView.evaluateJavaScript("window.clearShortcutPreview?.()")
                return
            }

            let payloadKey = parent.compilationID
            guard renderState.needsRender(payloadKey) else { return }
            let generation = renderState.begin(payloadKey)
            renderedOnEdit = nil
            let onEdit = parent.onEdit
            let encodedBase64 = Self.javaScriptString(plist.base64EncodedString())
            let encodedName = Self.javaScriptString(parent.name)
            let encodedContext = Self.javaScriptString(payloadKey.uuidString)
            let script = """
            (() => {
                if (typeof window.renderShortcutFromBase64 !== 'function') {
                    throw new Error('preview-shortcut bundle did not initialize');
                }
                return window.renderShortcutFromBase64(\(encodedBase64), \(encodedName), \(encodedContext));
            })()
            """

            webView.evaluateJavaScript(script) { [weak self, weak webView] _, error in
                guard let self, let webView,
                      self.renderState.isPending(payloadKey, generation: generation) else { return }
                if let error {
                    self.showError("Shortcut preview failed: \(error.localizedDescription)", in: webView)
                    return
                }
                self.renderState.complete(payloadKey, generation: generation)
                self.renderedOnEdit = onEdit
                self.renderIfReady(in: webView)
            }
        }

        func showError(_ message: String, in webView: WKWebView) {
            isReady = true
            invalidateRendering()

            let escapedMessage = Self.htmlEscaped(message)
            let html = """
            <!doctype html>
            <html>
            <head>
              <meta name="viewport" content="width=device-width, initial-scale=1.0">
              <style>
                body { font: -apple-system-body; margin: 0; padding: 20px; color: #8b1a1a; background: transparent; }
                .error { border: 1px solid rgba(139, 26, 26, .25); border-radius: 12px; padding: 14px; background: rgba(255, 0, 0, .04); }
              </style>
            </head>
            <body><div class="error">\(escapedMessage)</div></body>
            </html>
            """
            webView.loadHTMLString(html, baseURL: nil)
        }

        private static func boolValue(_ value: Any?) -> Bool? {
            if let bool = value as? Bool {
                return bool
            }
            return (value as? NSNumber)?.boolValue
        }

        private static func javaScriptString(_ value: String) -> String {
            guard let data = try? JSONEncoder().encode(value),
                  let string = String(data: data, encoding: .utf8) else {
                return "\"\""
            }
            return string
        }

        private static func htmlEscaped(_ value: String) -> String {
            value
                .replacingOccurrences(of: "&", with: "&amp;")
                .replacingOccurrences(of: "<", with: "&lt;")
                .replacingOccurrences(of: ">", with: "&gt;")
                .replacingOccurrences(of: "\"", with: "&quot;")
                .replacingOccurrences(of: "'", with: "&#39;")
        }
    }
}
