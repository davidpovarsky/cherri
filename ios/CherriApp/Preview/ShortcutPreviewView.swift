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
            webView.loadHTMLString(
                "<html><body><p>Preview resources are missing. Run the iOS bootstrap build.</p></body></html>",
                baseURL: nil
            )
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
            lastPayloadKey = payloadKey

            let encodedBase64 = Self.javaScriptString(base64)
            let encodedName = Self.javaScriptString(parent.name)
            let script = "window.renderShortcutFromBase64?.(\(encodedBase64), \(encodedName))"
            webView.evaluateJavaScript(script)
        }

        private static func javaScriptString(_ value: String) -> String {
            guard let data = try? JSONEncoder().encode(value),
                  let string = String(data: data, encoding: .utf8) else {
                return "\"\""
            }
            return string
        }
    }
}
