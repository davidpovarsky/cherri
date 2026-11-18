import Foundation

/// Counts only; no raw callback result, error message, URL, or arbitrary token
/// is persisted by this diagnostic. Existing payload redaction stays in force.
struct CallbackPayloadShape: Codable {
    let utf8ByteCount: Int
    let nonemptyLineCount: Int
    let collectorMarkerCount: Int
    let inlineMarkerCount: Int
    let sentinelLineCount: Int

    init(_ raw: String) {
        let lines = raw.components(separatedBy: .newlines)
            .map { $0.trimmingCharacters(in: .whitespacesAndNewlines) }
            .filter { !$0.isEmpty }
        utf8ByteCount = raw.utf8.count
        nonemptyLineCount = lines.count
        collectorMarkerCount = lines.filter { $0.contains("\"kind\":\"COLLECTOR_EVENT\"") }.count
        inlineMarkerCount = lines.filter { $0.contains("\"kind\":\"INLINE_DEBUG_ACTION\"") }.count
        sentinelLineCount = lines.filter { $0 == "===CHERRI_QA_TRACE_NDJSON_V14===" }.count
    }
}

enum QACallbackDiagnostics {
    static func numericErrorCode(_ raw: String?) -> String? {
        guard let raw, raw.range(of: "^-?[0-9]{1,12}$", options: .regularExpression) != nil,
              let value = Int64(raw) else { return nil }
        return String(value)
    }
}
