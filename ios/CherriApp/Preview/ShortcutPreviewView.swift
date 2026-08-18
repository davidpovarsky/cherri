import Foundation
import SwiftUI
import WebKit

struct ShortcutPreviewView: UIViewRepresentable {
    let plist: Data?
    let name: String

    func makeCoordinator() -> Coordinator {
        Coordinator(parent: self)
    }

    func makeUIView(context: Context) -> WKWebView {
        let webView = WKWebView(frame: .zero)
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
        context.coordinator.renderIfReady(in: webView)
    }

    final class Coordinator: NSObject, WKNavigationDelegate {
        var parent: ShortcutPreviewView
        private var isReady = false
        private var lastPayloadKey: String?

        init(parent: ShortcutPreviewView) {
            self.parent = parent
        }

        func webView(_ webView: WKWebView, didFinish navigation: WKNavigation!) {
            isReady = true
            renderIfReady(in: webView)
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
            lastPayloadKey = nil
            webView.reload()
        }

        func renderIfReady(in webView: WKWebView) {
            guard isReady else { return }

            guard let plist = parent.plist else {
                guard lastPayloadKey != nil else { return }
                lastPayloadKey = nil
                webView.evaluateJavaScript("window.clearShortcutPreview?.()")
                return
            }

            let base64 = plist.base64EncodedString()
            let payloadKey = "\(parent.name):\(base64.hashValue)"
            guard payloadKey != lastPayloadKey else { return }

            let encodedBase64 = Self.javaScriptString(base64)
            let encodedName = Self.javaScriptString(parent.name)
            let script = """
            (() => {
                if (typeof window.renderShortcutFromBase64 !== 'function') {
                    throw new Error('preview-shortcut bundle did not initialize');
                }
                return window.renderShortcutFromBase64(\(encodedBase64), \(encodedName));
            })()
            """

            webView.evaluateJavaScript(script) { [weak self, weak webView] _, error in
                guard let self, let webView else { return }
                if let error {
                    self.showError("Shortcut preview failed: \(error.localizedDescription)", in: webView)
                    return
                }
                self.lastPayloadKey = payloadKey
            }
        }

        func showError(_ message: String, in webView: WKWebView) {
            isReady = true
            lastPayloadKey = nil

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
