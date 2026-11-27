import Foundation

// A Foundation-only protocol control. It does not import or execute a shortcut.
// The first input is the immutable actual failed native dispatch receipt.
// The second is a real harmless /bin/true process control with literal xcrun argv.
@main
enum VerifyCallerImportDispatch {
    static func require(_ value: Bool, _ message: String) throws {
        if !value { throw NSError(domain: "CherriCallerImportDispatchControl", code: 1,
            userInfo: [NSLocalizedDescriptionKey: message]) }
    }

    static func timestamp(_ text: String) throws -> Date {
        let parser = ISO8601DateFormatter()
        parser.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
        if let value = parser.date(from: text) { return value }
        parser.formatOptions = [.withInternetDateTime]
        guard let value = parser.date(from: text) else {
            throw NSError(domain: "CherriCallerImportDispatchControl", code: 2)
        }
        return value
    }

    static func main() throws {
        try require(CommandLine.arguments.count == 3, "two exact receipt inputs")
        let actual = try Data(contentsOf: URL(fileURLWithPath: CommandLine.arguments[1]))
        let folder = URL(fileURLWithPath: CommandLine.arguments[2], isDirectory: true)
        let candidate = try Data(contentsOf: folder.appendingPathComponent("dispatch-phase.json"))
        let launches = try JSONSerialization.jsonObject(with:
            Data(contentsOf: folder.appendingPathComponent("launch-receipts.original.json"))) as! [[String: Any]]
        let original = try JSONSerialization.jsonObject(with: actual) as! [String: Any]
        let produced = try JSONSerialization.jsonObject(with: candidate) as! [String: Any]
        let originalSearch = try timestamp(original["phasePublishedUTC"] as! String)
        let originalNow = try timestamp(original["completedUTC"] as! String).addingTimeInterval(1)
        var oldReader = QADifferentialImportTiming(searchStartedAt: originalSearch)
        try require(oldReader.observeDispatch(actual, now: originalNow)
            == .rejected("IMPORT_DISPATCH_RECEIPT_MALFORMED"), "actual absolute-argv receipt stays rejected")
        let search = try timestamp(produced["phasePublishedUTC"] as! String)
        let completed = try timestamp(produced["completedUTC"] as! String)
        let exited = try timestamp(produced["exitObservedUTC"] as! String)
        var completedReader = QADifferentialImportTiming(searchStartedAt: search)
        try require(completedReader.observeDispatch(candidate, now: completed.addingTimeInterval(1))
            == .exited(at: exited), "truthful literal-argv completed producer receipt")
        try require(launches.count == 2, "both actual publication receipts")
        for (index, launch) in launches.enumerated() {
            let bytes = try JSONSerialization.data(withJSONObject: launch, options: [.sortedKeys])
            let started = try timestamp(launch["startedUTC"] as! String)
            let observed = try timestamp((launch["phasePublishedUTC"] as? String) ?? (launch["startedUTC"] as! String))
            var reader = QADifferentialImportTiming(searchStartedAt: started)
            guard case .pending = reader.observeDispatch(bytes, now: observed.addingTimeInterval(0.01)) else {
                throw NSError(domain: "CherriCallerImportDispatchControl", code: 3,
                    userInfo: [NSLocalizedDescriptionKey: "launch receipt " + String(index)])
            }
            var malformed = launch
            malformed["parentReaped"] = false
            let bad = try JSONSerialization.data(withJSONObject: malformed, options: [.sortedKeys])
            var rejecting = QADifferentialImportTiming(searchStartedAt: started)
            try require(rejecting.observeDispatch(bad, now: observed.addingTimeInterval(0.01))
                == .rejected("IMPORT_DISPATCH_RECEIPT_MALFORMED"), "premature completion field stays rejected")
        }
        print("{\"schema\":\"cherri-caller-import-dispatch-foundation-control/1\","
            + "\"actualOriginalRejected\":true,\"actualSyntheticLaunchesPending\":2,"
            + "\"actualSyntheticTerminalAccepted\":true,\"prematureCompletionRejections\":2,"
            + "\"nativeExecuted\":false,\"consumerChanged\":false}")
    }
}
