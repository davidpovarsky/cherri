import Foundation

@main
struct VerifyEditorStartupTiming {
    static func main() throws {
        var assertions = 0
        func check(_ condition: @autoclosure () -> Bool, _ message: String) {
            assertions += 1
            precondition(condition(), message)
        }
        func decoded(_ trace: CherriEditorStartupTimingTrace) throws -> [String: Any] {
            let bytes = trace.encoded(testName: "CherriEditorUITests/testTimingControl")!
            return try JSONSerialization.jsonObject(with: bytes) as! [String: Any]
        }
        var now: TimeInterval = 100
        let deadline = CherriEditorStartupDeadline(now: { now })
        let trace = CherriEditorStartupTimingTrace(startedAt: deadline.diagnosticStartedAt, now: { now })
        var calls = 0
        let observed = trace.measure(.windowSnapshot, .editorExists) {
            calls += 1
            now = 100.5
            return false
        }
        check(!observed, "diagnostics must return the real false result")
        check(calls == 1, "diagnostics must not repeat an accessibility query")
        var receipt = try decoded(trace)
        var entries = receipt["entries"] as! [[String: Any]]
        check(entries.count == 1, "one observation")
        check(entries[0]["startedMilliseconds"] as? Int == 0, "same origin as the strict deadline")
        check(entries[0]["completedMilliseconds"] as? Int == 500, "real query duration")
        check(entries[0]["value"] as? Bool == false, "false observation is preserved")
        check(entries[0]["mainThread"] as? Bool == Thread.isMainThread, "observed execution context")
        check(receipt["inFlightObservations"] as? Int == 0, "completed observation is no longer in flight")

        var observedInFlight = -1
        _ = trace.measure(.editorWait, .editorExists) {
            observedInFlight = (try! decoded(trace))["inFlightObservations"] as! Int
            now = 129
            return true
        }
        check(observedInFlight == 1, "a blocked query remains distinguishable from no query")
        receipt = try decoded(trace)
        entries = receipt["entries"] as! [[String: Any]]
        check(entries[1]["completedMilliseconds"] as? Int == 29000, "late state retains its late timestamp")
        check(entries[1]["value"] as? Bool == true, "late true is not rewritten as false")
        let ready = CherriEditorStartupObservation(windowExists: true, appRunningForeground: true,
            createDocumentExists: false, createDocumentHittable: false, editorExists: true, editorHittable: true)
        check(!deadline.accepts(ready, waitCompleted: true), "trace cannot override the original 28-second deadline")
        trace.mark(.editorWait, .returned, value: false)
        receipt = try decoded(trace)
        entries = receipt["entries"] as! [[String: Any]]
        check(entries[2]["value"] as? Bool == false, "actual waiter failure is not synthesized as success")
        check(entries[2]["startedMilliseconds"] as? Int == entries[2]["completedMilliseconds"] as? Int,
              "wait boundaries are instantaneous marks, not invented command durations")
        // Exercise the real Swift encoder through the Python intake locally.
        // This is an explicitly synthetic control, never an Apple test receipt.
        print("SYNTHETIC_TIMING_CONTROL=" + String(data: trace.encoded(testName: "CherriEditorUITests/testTimingControl")!, encoding: .utf8)!)

        for _ in 0..<130 { trace.mark(.editorWait, .entered, value: true) }
        receipt = try decoded(trace)
        entries = receipt["entries"] as! [[String: Any]]
        check(entries.count == 128, "bounded retained observations")
        check(receipt["droppedEntries"] as? Int == 5, "overflow is explicit")
        check(entries.enumerated().allSatisfy { $0.element["sequence"] as? Int == $0.offset }, "serialized observation order")
        let countBefore = calls
        check(trace.measure(.editorWait, .editorExists) { calls += 1; return true }, "capacity never changes the query result")
        check(calls == countBefore + 1, "capacity never skips the real query")
        receipt = try decoded(trace)
        check(receipt["droppedEntries"] as? Int == 6, "discarded completion remains visible in the count")
        check(receipt["inFlightObservations"] as? Int == 0, "overflow cannot leak active-query state")
        check(Set(receipt.keys) == ["schemaVersion", "testName", "budgetMilliseconds", "elapsedMilliseconds",
                                   "entries", "droppedEntries", "inFlightObservations"], "fixed metadata-only receipt")
        check(receipt["budgetMilliseconds"] as? Int == 28000, "unchanged acceptance budget")
        print("PASS: \(assertions) startup timing controls; diagnostic only, Apple NOT_RUN")
    }
}
