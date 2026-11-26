import Foundation

/// Diagnostics from the original predicate reads only. This type performs no AX
/// queries, waits, taps, readiness decisions, or source/label capture.
final class CherriUIReadinessTrace {
    enum Boundary: String {
        case typeSign = "brg25-type-sign"
        case malformedSign = "brg25-malformed-sign"
        case firstText = "brg28-first-text"
        case keptText = "brg28-kept-text"

        var isSign: Bool { self == .typeSign || self == .malformedSign }
        var testID: String {
            isSign
                ? "CherriJoinedContractUITests/testBRG25ActualAppRejectedAndLiteralRoutesUI"
                : "CherriPreviewDocumentUITests/testBRG28ActualWebKitFieldEditAndNumericRejectionUI"
        }
    }

    private let boundary: Boundary
    private let startedAt = ProcessInfo.processInfo.systemUptime
    private var observations = 0
    private var state: [String: Any] = [:]
    private let pane: [String: Any]?

    init(_ boundary: Boundary, pane: [String: Any]? = nil) {
        self.boundary = boundary
        self.pane = pane
    }

    func sign(exists: Bool, hittable: Bool?, enabled: Bool?) {
        observations += 1
        state = [
            "exists": exists,
            "hittable": hittable.map { $0 as Any } ?? NSNull(),
            "enabled": enabled.map { $0 as Any } ?? NSNull(),
        ]
    }

    func fields(webExists: Bool, candidateCount: Int, matchingCount: Int) {
        observations += 1
        state = [
            "webExists": webExists,
            "candidateCount": min(255, candidateCount),
            "matchingCount": min(255, matchingCount),
            "countsClamped": candidateCount > 255 || matchingCount > 255,
        ]
    }

    func finish(waitCompleted: Bool) {
        let elapsed = ProcessInfo.processInfo.systemUptime - startedAt
        guard elapsed.isFinite, elapsed >= 0 else { return }
        var row: [String: Any] = [
            "schema": "cherri-ui-readiness-observation/1",
            "testId": boundary.testID, "boundary": boundary.rawValue,
            "query": boundary.isSign ? "button:signButton" : "firstWebView:Text",
            "budgetMilliseconds": boundary.isSign ? 5_000 : 8_000,
            "elapsedMilliseconds": Int(min(elapsed * 1_000, 120_000)),
            "observations": min(observations, 128),
            "observationsClamped": observations > 128,
            "waitCompleted": waitCompleted,
            "state": state,
        ]
        if !boundary.isSign {
            row["pane"] = pane ?? [
                "target": "Preview", "exists": NSNull(), "hittable": NSNull(),
                "tapIssued": false, "selectionObserved": false,
            ]
        }
        if let bytes = try? JSONSerialization.data(withJSONObject: row, options: [.sortedKeys]),
           bytes.count <= 2_048, let json = String(data: bytes, encoding: .utf8) {
            print("[CHERRI_UI_READINESS] \(json)")
        }
    }
}
