import Foundation

@main
enum QAHostedImportDeliveryContracts {
    static func main() throws {
        let base = Date(timeIntervalSince1970: 1_790_000_000)
        let formatter = ISO8601DateFormatter()
        formatter.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
        func at(_ value: TimeInterval) -> Date { base.addingTimeInterval(value) }
        func stamp(_ value: TimeInterval) -> String { formatter.string(from: at(value)) }
        func receipt(_ timeout: Double = 45, exit: Double? = nil) throws -> Data {
            var value: [String: Any] = [
                "schemaVersion": 1, "command": ["xcrun", "simctl", "openurl", "owned", "file:///owned/exact.shortcut"],
                "timeoutSeconds": timeout, "startedUTC": stamp(0), "pid": 123,
                "outcome": "LAUNCHED", "timedOut": false, "exitCode": NSNull(),
                "phasePublishedUTC": stamp(0.001)
            ]
            if let exit = exit {
                value["outcome"] = "EXITED"
                value["exitCode"] = 0
                value["exitObservedUTC"] = stamp(exit)
                value["completedUTC"] = stamp(exit)
                value["elapsedSeconds"] = exit
                value["processReturncode"] = 0
                value["parentReaped"] = true
            }
            return try JSONSerialization.data(withJSONObject: value, options: [.sortedKeys])
        }
        var checks: [[String: Any]] = []
        func require(_ condition: Bool, _ name: String) {
            checks.append(["name": name, "outcome": condition ? "PASS" : "FAIL"])
        }
        var hosted = QADifferentialImportTiming(searchStartedAt: at(1), deliveryBudget: .hostedPrimary)
        let pending = hosted.observeDispatch(try receipt(), now: at(44.999))
        require(pending == .pending(deadline: at(45)), "hosted pending uses the actual 45-second command bound")
        require(hosted.sheetDecision(after: pending, now: at(44.999), buttonExists: true,
            buttonHittable: true, identityMatches: true).canTap, "matched consent may precede delivery exit")
        require(!hosted.libraryDecision(after: pending, addTappedAt: at(3), now: at(44)).canConfirm,
                "pending delivery cannot authenticate a library cell")
        require(hosted.observeDispatch(try receipt(), now: at(45)).failure == "IMPORT_DISPATCH_DEADLINE_EXCEEDED",
                "hosted pending expires at exactly 45 seconds")
        require(!hosted.observeDispatch(try receipt(exit: 40), now: at(46)).succeeded,
                "late success cannot revive an expired hosted observation")
        var success = QADifferentialImportTiming(searchStartedAt: at(1), deliveryBudget: .hostedPrimary)
        let exit = success.observeDispatch(try receipt(exit: 44), now: at(44))
        require(exit.succeeded, "actual completed hosted delivery is accepted")
        require(success.addDecision(after: exit, now: at(44)) == .search(deadline: at(59)),
                "hosted exit keeps the original 15-second Add window")
        require(success.libraryDecision(after: exit, addTappedAt: at(3), now: at(44)).deadline == at(74),
                "hosted exit keeps the original 30-second library window")
        require(!success.libraryDecision(after: exit, addTappedAt: at(3), now: at(74)).canConfirm,
                "hosted library deadline cannot be renewed")
        var late = QADifferentialImportTiming(searchStartedAt: at(1), deliveryBudget: .hostedPrimary)
        require(late.observeDispatch(try receipt(exit: 45.001), now: at(46)).failure == "IMPORT_DISPATCH_LATE_EXIT",
                "hosted child exit beyond 45 seconds is rejected")
        var wrong = QADifferentialImportTiming(searchStartedAt: at(1), deliveryBudget: .hostedPrimary)
        require(wrong.observeDispatch(try receipt(60), now: at(2)).failure != nil,
                "hosted receipt cannot claim the Differential 60-second allowance")
        var legacy = QADifferentialImportTiming(searchStartedAt: at(1))
        require(legacy.observeDispatch(try receipt(60), now: at(59)).failure == nil,
                "default Differential allowance remains 60 seconds")
        var legacyWrong = QADifferentialImportTiming(searchStartedAt: at(1))
        require(legacyWrong.observeDispatch(try receipt(), now: at(2)).failure != nil,
                "default Differential source does not silently adopt a hosted receipt")
        for label in ["Add Shortcut", "+ Add Shortcut"] {
            require(QAImportConsentPolicy.permits(buttonLabel: label, requiresAbsentLibraryRecord: true),
                    "fresh import permits exact " + label)
        }
        let rejectedLabels: [String?] = ["Replace", "Keep Both", "Add Shortcut other", "add shortcut", nil]
        for label in rejectedLabels {
            require(!QAImportConsentPolicy.permits(buttonLabel: label, requiresAbsentLibraryRecord: true),
                    "fresh import rejects non-Add " + (label ?? "nil"))
        }
        require(QAImportConsentPolicy.permits(buttonLabel: "Replace", requiresAbsentLibraryRecord: false),
                "legacy replacement policy remains explicit")
        let failed = checks.filter { $0["outcome"] as? String != "PASS" }
        let output: [String: Any] = ["schema": "cherri-hosted-import-delivery-controls/1",
            "checks": checks, "count": checks.count, "failed": failed.count,
            "nativeExecution": false, "formalAcceptanceGranted": false]
        let data = try JSONSerialization.data(withJSONObject: output, options: [.sortedKeys])
        print(String(decoding: data, as: UTF8.self))
        if !failed.isEmpty { throw NSError(domain: "QAHostedImportDelivery", code: 1) }
    }
}
