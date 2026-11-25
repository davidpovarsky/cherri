import SwiftUI
import UIKit

struct CallbackEventRecord: Codable {
    let schemaVersion: String
    let runId: String
    let fixtureId: String?
    let expectedShortcutName: String?
    let eventSource: String
    let status: String
    let host: String
    let resultSummary: String?
    let tracePayload: String?
    let errorMessage: String?
    let errorCode: String?
    let resultShape: CallbackPayloadShape?
    let receiverSelfTest: Bool
    let timestampISO8601: String
}

final class CallbackEventStore: ObservableObject {
    static let shared = CallbackEventStore()
    let primaryNoInputLaunchGate: QAPrimaryNoInputLaunchGate

    @Published var events: [CallbackEventRecord] = []
    @Published var lastStatus: String = "IDLE"
    @Published var lastRunId: String = "none"
    @Published var lastResult: String = "none"
    @Published var lastTriggerStatus: String = "idle"

    var triggerDebugFileURL: URL {
        let docs = FileManager.default.urls(for: .documentDirectory, in: .userDomainMask).first!
        return docs.appendingPathComponent("trigger-debug.txt")
    }

    func logTrigger(_ message: String) {
        let clean = Self.containsForbiddenSecret(message) ? "[REDACTED_SECRET_OR_PII]" : message
        NSLog("[CherriReceiver] %@", clean)
        DispatchQueue.main.async {
            self.lastTriggerStatus = String(clean.prefix(240))
        }
        let line = "\(formatter.string(from: Date())) \(clean)\n"
        if let data = line.data(using: .utf8) {
            if FileManager.default.fileExists(atPath: triggerDebugFileURL.path),
               let handle = try? FileHandle(forWritingTo: triggerDebugFileURL) {
                _ = try? handle.seekToEnd()
                try? handle.write(contentsOf: data)
                try? handle.close()
            } else {
                try? data.write(to: triggerDebugFileURL, options: .atomic)
            }
        }
    }

    private let formatter: ISO8601DateFormatter = {
        let f = ISO8601DateFormatter()
        f.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
        return f
    }()

    init() {
        let documents = FileManager.default.urls(for: .documentDirectory, in: .userDomainMask).first!
        primaryNoInputLaunchGate = QAPrimaryNoInputLaunchGate(
            rawContract: ProcessInfo.processInfo.environment[QAPrimaryNoInputLaunchGate.environmentKey],
            evidenceRoot: documents)
        loadExisting()
    }

    var eventsFileURL: URL {
        let docs = FileManager.default.urls(for: .documentDirectory, in: .userDomainMask).first!
        return docs.appendingPathComponent("callback-events.json")
    }

    func resetStore() {
        events = []
        lastStatus = "IDLE"
        lastRunId = "none"
        lastResult = "none"
        lastTriggerStatus = "idle"
        try? FileManager.default.removeItem(at: eventsFileURL)
        try? FileManager.default.removeItem(at: triggerDebugFileURL)
        persist()
    }

    func loadExisting() {
        guard let data = try? Data(contentsOf: eventsFileURL),
              let decoded = try? JSONDecoder().decode([CallbackEventRecord].self, from: data) else {
            return
        }
        events = decoded
        if let last = decoded.last {
            lastStatus = last.status
            lastRunId = last.runId
            lastResult = last.resultSummary ?? last.errorMessage ?? "none"
        }
    }

    private static let forbiddenPattern: NSRegularExpression? = {
        let pattern = "(?i)(cherri-shortcut-audit-20261110|\\b[0-3][0-9]\\.plist\\b|BEGIN [A-Z ]*PRIVATE KEY|ghp_[A-Za-z0-9_]+|github_pat_[A-Za-z0-9_]+|sk-[A-Za-z0-9]{16,}|Bearer\\s+[A-Za-z0-9._\\-]+|(?<![A-Za-z0-9._%+\\-])[A-Za-z0-9._%+\\-]+@[A-Za-z0-9.\\-]+\\.[A-Za-z]{2,}|/Users/[A-Za-z0-9._\\-]+)"
        return try? NSRegularExpression(pattern: pattern)
    }()

    static func containsForbiddenSecret(_ s: String) -> Bool {
        guard let regex = forbiddenPattern else { return false }
        let range = NSRange(s.startIndex..<s.endIndex, in: s)
        return regex.firstMatch(in: s, options: [], range: range) != nil
    }

    /// Redacts arbitrary strings unless they match synthetic allowlisted prefixes or validated NDJSON trace records.
    static func sanitizeAllowlisted(_ raw: String?) -> String? {
        guard let raw = raw, !raw.isEmpty else { return nil }
        let trimmed = raw.trimmingCharacters(in: .whitespacesAndNewlines)
        if trimmed.isEmpty { return nil }
        if containsForbiddenSecret(trimmed) {
            return "[REDACTED_SECRET_OR_PII]"
        }
        if trimmed.hasPrefix("CHERRI_QA_") ||
           trimmed.hasPrefix("CHERRI_OPTIONAL_FORWARDING_DIAGNOSTIC_V1:") ||
           trimmed.hasPrefix("CHERRI_NESTED_LIST_CONTROL_V1:") ||
           trimmed.hasPrefix("CHERRI_NESTED_LIST_CANDIDATE_V1:") ||
           trimmed.hasPrefix("CHERRI_TAGGED_VALUE_CARRIER_V1:") ||
           trimmed.hasPrefix("CHERRI_FUNCTION_ENVELOPE_PRESENCE_V1:") ||
           trimmed.hasPrefix("{\"schemaVersion\":\"1.4\"") ||
           trimmed.hasPrefix("{\"schemaVersion\":\"1.0\"") ||
           trimmed.hasPrefix("===CHERRI_QA_TRACE_NDJSON_V14===") ||
           trimmed.hasPrefix("ANCHOR_") ||
           trimmed == "ok" ||
           trimmed == "cancelled" {
            return String(trimmed.prefix(16384))
        }
        return "[REDACTED_NON_ALLOWLISTED_LEN_\(trimmed.count)]"
    }

    static func splitResultAndTrace(_ raw: String?) -> (summary: String?, trace: String?) {
        guard let raw = raw, !raw.isEmpty else { return (nil, nil) }
        var summaryLines: [String] = []
        var traceLines: [String] = []
        for line in raw.components(separatedBy: .newlines) {
            let t = line.trimmingCharacters(in: .whitespacesAndNewlines)
            if t.isEmpty { continue }
            if t == "===CHERRI_QA_TRACE_NDJSON_V14===" ||
               (t.hasPrefix("{") && t.hasSuffix("}") && (t.contains("\"kind\":\"COLLECTOR_EVENT\"") || t.contains("\"kind\":\"INLINE_DEBUG_ACTION\""))) {
                traceLines.append(t)
            } else {
                summaryLines.append(t)
            }
        }
        let summaryStr = summaryLines.isEmpty ? nil : sanitizeAllowlisted(summaryLines.joined(separator: "\n"))
        let traceStr = traceLines.isEmpty ? nil : sanitizeAllowlisted(traceLines.joined(separator: "\n"))
        return (summaryStr, traceStr)
    }

    func handleIncomingURL(_ url: URL) {
        guard url.scheme?.lowercased() == "cherriqa" else { return }
        let host = (url.host ?? "unknown").lowercased()
        let comps = URLComponents(url: url, resolvingAgainstBaseURL: false)
        var params: [String: String] = [:]
        for item in comps?.queryItems ?? [] {
            if let existing = params[item.name], !existing.isEmpty {
                continue
            }
            params[item.name] = item.value ?? ""
        }
        // Also handle path-encoded metadata: cherriqa://success/<runId>/<fixtureId>/<shortcutName>?result=...
        let pathParts = url.pathComponents.filter { $0 != "/" && !$0.isEmpty }
        let pathRunId = pathParts.count >= 1 ? pathParts[0] : nil
        let pathFixtureId = pathParts.count >= 2 ? pathParts[1] : nil
        let pathShortcutName = pathParts.count >= 3 ? pathParts[2] : nil

        let runId = params["runId"] ?? params["run_id"] ?? pathRunId ?? "unspecified-run"
        let fixtureId = params["fixtureId"] ?? params["fixture_id"] ?? pathFixtureId
        let expectedShortcut = params["shortcutName"] ?? params["shortcut_name"] ?? pathShortcutName
        let selfTest = (params["selfTest"] == "1" || runId.hasPrefix("receiver-self-test"))
        let eventSource = selfTest ? "receiver-self-test" : (params["source"] ?? "shortcuts-x-callback")

        let status: String
        switch host {
        case "success":
            status = "SUCCESS"
        case "cancel", "cancelled":
            status = "CANCELLED"
        case "error":
            status = "ERROR"
        default:
            status = "UNKNOWN_\(host.uppercased())"
        }

        let rawResult = params["result"] ?? params["output"] ?? params["input"] ?? params["text"]
        let rawErr = params["errorMessage"] ?? params["error"]
        let (cleanSummary, cleanTrace) = Self.splitResultAndTrace(rawResult)

        let record = CallbackEventRecord(
            schemaVersion: "1.4",
            runId: String(runId.prefix(128)),
            fixtureId: fixtureId.map { String($0.prefix(128)) },
            expectedShortcutName: expectedShortcut.map { String($0.prefix(128)) },
            eventSource: String(eventSource.prefix(64)),
            status: status,
            host: host,
            resultSummary: cleanSummary,
            tracePayload: cleanTrace,
            errorMessage: Self.sanitizeAllowlisted(rawErr),
            errorCode: QACallbackDiagnostics.numericErrorCode(params["errorCode"] ?? params["error_code"]),
            resultShape: rawResult.map { CallbackPayloadShape($0) },
            receiverSelfTest: selfTest,
            timestampISO8601: formatter.string(from: Date())
        )
        events.append(record)
        lastStatus = record.status
        lastRunId = record.runId
        lastResult = record.resultSummary ?? record.errorMessage ?? "none"
        persist()
    }

    private func persist() {
        let encoder = JSONEncoder()
        encoder.outputFormatting = [.prettyPrinted, .sortedKeys]
        if let data = try? encoder.encode(events) {
            try? data.write(to: eventsFileURL, options: .atomic)
        }
    }
}

struct ShareSheetView: UIViewControllerRepresentable {
    let activityItems: [Any]

    func makeUIViewController(context: Context) -> UIActivityViewController {
        UIActivityViewController(activityItems: activityItems, applicationActivities: nil)
    }

    func updateUIViewController(_ uiViewController: UIActivityViewController, context: Context) {}
}

@main
struct CherriSimulatorQAReceiverApp: App {
    @StateObject private var store = CallbackEventStore.shared
    @State private var showShareSheet = false
    @State private var shareFileURL: URL?
    @State private var didTriggerInitialURL = false

    var body: some Scene {
        WindowGroup {
            VStack(spacing: 16) {
                Text("Cherri iPadOS 27 Simulator QA Receiver")
                    .font(.headline)
                    .accessibilityIdentifier("qa.receiver.title")

                Text("Status: \(store.lastStatus)")
                    .accessibilityIdentifier("qa.receiver.status")

                Text("Last RunID: \(store.lastRunId)")
                    .accessibilityIdentifier("qa.receiver.lastRunId")

                Text("Last Result: \(store.lastResult)")
                    .accessibilityIdentifier("qa.receiver.lastResult")

                Text("Callback Count: \(store.events.count)")
                    .accessibilityIdentifier("qa.receiver.callbackCount")

                Text("Trigger: \(store.lastTriggerStatus)")
                    .font(.caption)
                    .accessibilityIdentifier("qa.receiver.triggerStatus")

                Button("Trigger Self Callback") {
                    if let u = URL(string: "cherriqa://success?runId=receiver-self-test&selfTest=1&result=CHERRI_QA_RECEIVER_OK") {
                        store.handleIncomingURL(u)
                    }
                }
                .accessibilityIdentifier("qa.receiver.selfCallbackButton")

                Button("Open Shortcuts Create") {
                    if let u = URL(string: "shortcuts://create-shortcut") {
                        UIApplication.shared.open(u)
                    }
                }
                .accessibilityIdentifier("qa.receiver.openShortcutsCreateButton")

                if let fileURL = shareFileURL {
                    Button("Share Shortcut File") {
                        showShareSheet = true
                    }
                    .accessibilityIdentifier("qa.receiver.shareShortcutButton")
                    .sheet(isPresented: $showShareSheet) {
                        ShareSheetView(activityItems: [fileURL])
                    }
                }
            }
            .padding(24)
            .onOpenURL { url in
                if url.scheme?.lowercased() == "cherriqa",
                   url.host?.lowercased() == "trigger",
                   let comps = URLComponents(url: url, resolvingAgainstBaseURL: false),
                   let target = comps.queryItems?.first(where: { $0.name == "target" })?.value,
                   !target.isEmpty {
                    store.logTrigger("onOpenURL trigger received len=\(target.count)")
                    triggerExternalShortcutURL(target, source: "onOpenURL", delay: 0.3)
                    return
                }
                store.logTrigger("onOpenURL callback scheme=\(url.scheme ?? "nil") host=\(url.host ?? "nil")")
                store.handleIncomingURL(url)
            }
            .onAppear {
                handleLaunchArguments()
            }
        }
    }

    private func triggerExternalShortcutURL(_ rawURL: String, source: String, delay: TimeInterval) {
        let permit = store.primaryNoInputLaunchGate.begin(rawURL: rawURL, source: source)
        guard permit.allowed else {
            store.logTrigger("source=\(source) noInputLaunchRejected=\(permit.reason ?? "UNKNOWN")")
            return
        }
        guard let targetURL = URL(string: rawURL) else {
            store.logTrigger("source=\(source) invalidURL len=\(rawURL.count)")
            return
        }
        let canOpen = UIApplication.shared.canOpenURL(targetURL)
        store.logTrigger("source=\(source) scheduling open scheme=\(targetURL.scheme ?? "nil") host=\(targetURL.host ?? "nil") canOpen=\(canOpen) delay=\(delay)")
        DispatchQueue.main.asyncAfter(deadline: .now() + delay) {
            UIApplication.shared.open(targetURL, options: [:]) { success in
                if !self.store.primaryNoInputLaunchGate.complete(permit, success: success) {
                    self.store.logTrigger("source=\(source) originalReceipt=FAILED")
                }
                self.store.logTrigger("source=\(source) UIApplication.openCompletion=\(success)")
                if !success && permit.allowsFallback {
                    if let scene = UIApplication.shared.connectedScenes.compactMap({ $0 as? UIWindowScene }).first {
                        scene.open(targetURL, options: nil) { sceneSuccess in
                            self.store.logTrigger("source=\(source) scene.openCompletion=\(sceneSuccess)")
                        }
                    } else {
                        self.store.logTrigger("source=\(source) no UIWindowScene for fallback open")
                    }
                }
            }
        }
    }

    private func handleLaunchArguments() {
        let args = ProcessInfo.processInfo.arguments
        let envTrigger = ProcessInfo.processInfo.environment["CHERRI_TRIGGER_SHORTCUT_URL"]
        let hasArgTrigger = args.contains("--trigger-shortcut-url")
        store.logTrigger("onAppear argsCount=\(args.count) hasArgTrigger=\(hasArgTrigger) hasEnvTrigger=\(envTrigger != nil && !(envTrigger?.isEmpty ?? true))")
        if args.contains("--reset-callback-store") {
            store.resetStore()
        }
        if args.contains("--trigger-self-callback") {
            if let u = URL(string: "cherriqa://success?runId=receiver-self-test&selfTest=1&result=CHERRI_QA_RECEIVER_OK") {
                store.handleIncomingURL(u)
            }
        }
        if !didTriggerInitialURL {
            if let envURL = envTrigger, !envURL.isEmpty {
                didTriggerInitialURL = true
                triggerExternalShortcutURL(envURL, source: "env", delay: 0.8)
            } else if let idx = args.firstIndex(of: "--trigger-shortcut-url"), idx + 1 < args.count {
                didTriggerInitialURL = true
                triggerExternalShortcutURL(args[idx + 1], source: "argv", delay: 0.8)
            }
        }
        if let idx = args.firstIndex(of: "--share-shortcut-path"), idx + 1 < args.count {
            let path = args[idx + 1]
            let url = URL(fileURLWithPath: path)
            if FileManager.default.fileExists(atPath: url.path) {
                shareFileURL = url
                DispatchQueue.main.asyncAfter(deadline: .now() + 0.6) {
                    showShareSheet = true
                }
            }
        }
    }
}
