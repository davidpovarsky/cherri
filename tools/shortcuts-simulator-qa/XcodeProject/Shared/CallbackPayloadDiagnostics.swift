import Foundation
import CryptoKit

/// Counts and an exact UTF-8 digest; no raw callback result, error message, URL,
/// or arbitrary token is persisted. Existing payload redaction stays in force.
struct CallbackPayloadShape: Codable {
    let utf8ByteCount: Int
    // Optional only to decode historical receipts without altering them. New
    // callbacks always set this, and current host acceptance requires it.
    let utf8SHA256: String?
    let nonemptyLineCount: Int
    let collectorMarkerCount: Int
    let inlineMarkerCount: Int
    let sentinelLineCount: Int

    init(_ raw: String) {
        let lines = raw.components(separatedBy: .newlines)
            .map { $0.trimmingCharacters(in: .whitespacesAndNewlines) }
            .filter { !$0.isEmpty }
        let rawBytes = Data(raw.utf8)
        utf8ByteCount = rawBytes.count
        utf8SHA256 = SHA256.hash(data: rawBytes).map { String(format: "%02x", $0) }.joined()
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
