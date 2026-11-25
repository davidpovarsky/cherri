import Foundation

@main
enum QADifferentialImportTimingContracts {
    static func main() throws {
        let formatter = ISO8601DateFormatter()
        formatter.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
        let base = formatter.date(from: "2026-11-25T16:48:00.000000+00:00")!
        func at(_ seconds: TimeInterval) -> Date { base.addingTimeInterval(seconds) }
        func stamp(_ seconds: TimeInterval) -> String { formatter.string(from: at(seconds)) }
        func encoded(_ fields: [String: Any]) throws -> Data {
            try JSONSerialization.data(withJSONObject: fields, options: [.sortedKeys])
        }
        let pending: [String: Any] = [
            "schemaVersion": 1, "command": ["xcrun", "simctl", "openurl", "owned-simulator", "file:///qa/exact.shortcut"],
            "timeoutSeconds": 60, "startedUTC": "2026-11-25T16:48:00.000000+00:00",
            "pid": 123, "outcome": "LAUNCHED", "exitCode": NSNull(), "timedOut": false,
            "phasePublishedUTC": "2026-11-25T16:48:00.002000+00:00"
        ]
        func exited(_ seconds: TimeInterval) -> [String: Any] {
            var fields = pending
            fields["outcome"] = "EXITED"
            fields["exitCode"] = 0
            fields["exitObservedUTC"] = stamp(seconds)
            fields["completedUTC"] = stamp(seconds)
            fields["elapsedSeconds"] = seconds
            fields["processReturncode"] = 0
            fields["parentReaped"] = true
            return fields
        }
        var checks: [[String: Any]] = []
        func require(_ condition: Bool, _ name: String) {
            checks.append(["name": name, "outcome": condition ? "PASS" : "FAIL"])
        }
        func permits(_ decision: QADifferentialImportTiming.AddDecision) -> Bool {
            if case .search = decision { return true }
            return false
        }
        func observe(_ policy: inout QADifferentialImportTiming, _ fields: [String: Any],
                     _ seconds: TimeInterval) throws -> QADifferentialImportTiming.AddDecision {
            let state = policy.observeDispatch(try encoded(fields), now: at(seconds))
            return policy.addDecision(after: state, now: at(seconds))
        }

        // Fixed clock observations represent the measured failure class. These
        // are actual Swift policy checks, not simulated Apple/XCTest successes.
        var delayed = QADifferentialImportTiming(searchStartedAt: at(1.59))
        require(permits(try observe(&delayed, pending, 16.60)),
                "pending dispatch does not consume the Add window at 15 seconds")
        require(permits(try observe(&delayed, pending, 30.90)),
                "Add inspection remains available before delayed delivery")
        require(try observe(&delayed, exited(31), 33) == .search(deadline: at(46)),
                "31-second successful delivery anchors a full 15-second Add window")
        require(try observe(&delayed, exited(31), 45.999) == .search(deadline: at(46)),
                "repeated receipt reads never move the deadline")
        require(try observe(&delayed, exited(31), 46) == .rejected("IMPORT_BUTTON_MISSING"),
                "exact Add deadline rejects a result returning from a blocking AX query")
        require(!permits(try observe(&delayed, exited(31), 70)),
                "successful dispatch cannot create a blind 75-second Add wait")

        var fast = QADifferentialImportTiming(searchStartedAt: at(2))
        require(try observe(&fast, exited(1), 2) == .search(deadline: at(17)),
                "fast delivery preserves 15 seconds from actual search start")
        require(!permits(try observe(&fast, exited(1), 17)),
                "fast delivery gets no extension beyond its original window")
        var searchAfterExit = QADifferentialImportTiming(searchStartedAt: at(40))
        require(try observe(&searchAfterExit, exited(31), 40) == .search(deadline: at(55)),
                "search starting after delivery receives exactly 15 seconds")
        require(!permits(try observe(&searchAfterExit, exited(31), 55)),
                "late search still has a bounded window")

        var delivery = QADifferentialImportTiming(searchStartedAt: at(1))
        require(try observe(&delivery, pending, 59.999) == .search(deadline: at(60)),
                "pending dispatch uses the existing 60-second host budget")
        require(try observe(&delivery, pending, 60) == .rejected("IMPORT_DISPATCH_DEADLINE_EXCEEDED"),
                "pending dispatch expires at the exact host deadline")
        require(!permits(try observe(&delivery, exited(59), 61)),
                "a failed observation cannot be revived by a later successful receipt")

        var exitAtBudget = QADifferentialImportTiming(searchStartedAt: at(1))
        require(try observe(&exitAtBudget, exited(60), 60) == .search(deadline: at(75)),
                "actual successful exit at the host boundary still anchors exactly 15 seconds")
        require(!permits(try observe(&exitAtBudget, exited(60), 75)),
                "even the latest valid exit grants no more than 15 seconds")

        var terminal = QADifferentialImportTiming(searchStartedAt: at(1))
        let initialState = terminal.observeDispatch(try encoded(pending), now: at(2))
        require(!initialState.succeeded && initialState.failure == nil,
                "pending delivery permits consent but cannot prove import success")
        let terminalState = terminal.observeDispatch(try encoded(exited(31)), now: at(33))
        require(terminalState.succeeded && terminalState.failure == nil,
                "actual exit0 and reaping permit the independent library verification")
        require(terminal.observeDispatch(try encoded(pending), now: at(34)).failure != nil,
                "completed dispatch cannot regress to a pending receipt")

        var noPublished = pending
        noPublished.removeValue(forKey: "phasePublishedUTC")
        var publicationRace = QADifferentialImportTiming(searchStartedAt: at(1))
        require(permits(try observe(&publicationRace, noPublished, 2)),
                "initial atomic LAUNCHED receipt is valid before publication timestamp")
        require(permits(try observe(&publicationRace, pending, 3)),
                "phase timestamp may be added without changing launch identity")

        var microseconds = exited(31)
        microseconds["startedUTC"] = "2026-11-25T16:48:00.000000+00:00"
        microseconds["exitObservedUTC"] = "2026-11-25T16:48:31.038168+00:00"
        microseconds["completedUTC"] = "2026-11-25T16:48:31.042282+00:00"
        microseconds["elapsedSeconds"] = 31.042282
        var pythonTimestamp = QADifferentialImportTiming(searchStartedAt: at(1.59))
        let fractionState = pythonTimestamp.observeDispatch(try encoded(microseconds), now: at(33))
        if case .exited(let exit) = fractionState {
            require(abs(exit.timeIntervalSince(base) - 31.038168) < 0.001,
                    "Python fractional ISO8601 UTC timestamps retain subsecond timing")
        } else {
            require(false, "Python fractional ISO8601 UTC timestamps retain subsecond timing")
        }

        struct Invalid {
            let name: String
            let fields: [String: Any]
            let now: TimeInterval
        }
        var invalid: [Invalid] = []
        func mutate(_ name: String, _ replacements: [String: Any],
                    base: [String: Any]? = nil, now: TimeInterval = 35, remove: [String] = []) {
            var fields = base ?? exited(31)
            replacements.forEach { fields[$0.key] = $0.value }
            remove.forEach { fields.removeValue(forKey: $0) }
            invalid.append(Invalid(name: name, fields: fields, now: now))
        }
        mutate("nonzero process exit", ["outcome": "EXIT_NONZERO", "exitCode": 7, "processReturncode": 7])
        mutate("timeout even with an otherwise successful UI", ["outcome": "TIMED_OUT", "timedOut": true])
        mutate("failed dispatch", ["outcome": "FAILED"])
        mutate("unknown dispatch outcome", ["outcome": "UNKNOWN"])
        mutate("false exit0 with nonzero reaped process", ["processReturncode": 7])
        mutate("unreaped process", ["parentReaped": false])
        mutate("missing process exit proof", [:], remove: ["processReturncode"])
        mutate("boolean is not an integer exit status", ["exitCode": false])
        mutate("wrong receipt schema", ["schemaVersion": 2])
        mutate("expanded host budget", ["timeoutSeconds": 75])
        mutate("negative elapsed duration", ["elapsedSeconds": -1])
        mutate("late actual exit", ["elapsedSeconds": 59], base: exited(60.001), now: 61)
        mutate("elapsed host budget exceeded", ["elapsedSeconds": 60.001])
        mutate("future actual exit", [:], base: exited(40))
        mutate("completion before exit", ["completedUTC": stamp(30)])
        mutate("publication after exit", ["phasePublishedUTC": stamp(32)])
        mutate("malformed start timestamp", ["startedUTC": "yesterday"])
        mutate("missing exit timestamp", [:], remove: ["exitObservedUTC"])
        mutate("missing completion timestamp", [:], remove: ["completedUTC"])
        mutate("missing final phase timestamp", [:], remove: ["phasePublishedUTC"])
        mutate("pending receipt with terminal exit fields", ["exitCode": 0], base: pending)
        mutate("receipt for a runtime URL", ["command": ["xcrun", "simctl", "openurl", "owned-simulator", "cherriqa://trigger"]])
        for item in invalid {
            var policy = QADifferentialImportTiming(searchStartedAt: at(1.59))
            let state = policy.observeDispatch(try encoded(item.fields), now: at(item.now))
            require(state.failure != nil && !state.succeeded &&
                    !permits(policy.addDecision(after: state, now: at(item.now))),
                    item.name + " fails closed")
        }
        for (name, data) in [
            ("missing receipt", Optional<Data>.none),
            ("malformed JSON", Optional<Data>.some(Data("{".utf8))),
            ("wrong JSON root", Optional<Data>.some(Data("[]".utf8)))
        ] {
            var policy = QADifferentialImportTiming(searchStartedAt: at(1))
            require(policy.observeDispatch(data, now: at(2)).failure != nil,
                    name + " fails closed")
        }

        for key in ["startedUTC", "pid", "command"] {
            var changed = pending
            if key == "startedUTC" {
                changed[key] = stamp(0.5)
                changed["phasePublishedUTC"] = stamp(0.6)
            }
            if key == "pid" { changed[key] = 456 }
            if key == "command" {
                changed[key] = ["xcrun", "simctl", "openurl", "other-simulator", "file:///qa/exact.shortcut"]
            }
            var policy = QADifferentialImportTiming(searchStartedAt: at(1))
            _ = policy.observeDispatch(try encoded(pending), now: at(2))
            require(policy.observeDispatch(try encoded(changed), now: at(3)) == .rejected("IMPORT_DISPATCH_RECEIPT_CHANGED"),
                    "changed launch " + key + " cannot reset the deadline")
        }
        var missingAfterSuccess = QADifferentialImportTiming(searchStartedAt: at(1))
        _ = missingAfterSuccess.observeDispatch(try encoded(exited(31)), now: at(33))
        require(missingAfterSuccess.observeDispatch(nil, now: at(34)).failure != nil,
                "missing receipt is rejected even after an earlier valid success")


        // Library observation has its own unchanged30s budget after actual
        // delivery. The Add timestamp is immutable; polling cannot renew it.
        func library(_ policy: inout QADifferentialImportTiming, _ fields: [String: Any],
                     add: TimeInterval, now: TimeInterval) throws -> QADifferentialImportTiming.LibraryDecision {
            let state = policy.observeDispatch(try encoded(fields), now: at(now))
            return policy.libraryDecision(after: state, addTappedAt: at(add), now: at(now))
        }
        // The oracle is the exact decoded UTC instant, not a second floating-point addition.
        let delayedLibraryDeadline = formatter.date(from: "2026-11-25T16:49:07.639Z")!
        var libraryDelayed = QADifferentialImportTiming(searchStartedAt: at(0.098))
        require(try library(&libraryDelayed, pending, add: 2.912, now: 34.392)
                == .inspect(deadline: at(60), canConfirm: false),
                "library inspection at original failure time retains the pending60s delivery budget")
        require(try library(&libraryDelayed, exited(37.639), add: 2.912, now: 38)
                == .inspect(deadline: delayedLibraryDeadline, canConfirm: true),
                "actual37.639s delivery anchors full30s library observation")
        require(try library(&libraryDelayed, exited(37.639), add: 2.912, now: 67.638)
                == .inspect(deadline: delayedLibraryDeadline, canConfirm: true),
                "repeated library observations do not extend actual delivery deadline")
        require(!(try library(&libraryDelayed, exited(37.639), add: 2.912, now: 67.639)).canConfirm,
                "library confirmation is forbidden at the exact30s deadline")
        require(!(try library(&libraryDelayed, exited(37.639), add: 2.912, now: 90)).canConfirm,
                "blocking library AX return cannot grant late success")

        var libraryFast = QADifferentialImportTiming(searchStartedAt: at(1))
        require(try library(&libraryFast, exited(2), add: 3, now: 4)
                == .inspect(deadline: at(33), canConfirm: true),
                "delivery before Add preserves exactly30s from immutable Add")
        require(try library(&libraryFast, exited(2), add: 3, now: 20)
                == .inspect(deadline: at(33), canConfirm: true),
                "fast library deadline does not move on repeat reads")
        require(!(try library(&libraryFast, exited(2), add: 3, now: 33)).canConfirm,
                "fast library has no extension beyond original30s")

        var libraryPending = QADifferentialImportTiming(searchStartedAt: at(1))
        require(try library(&libraryPending, pending, add: 2, now: 59.999)
                == .inspect(deadline: at(60), canConfirm: false),
                "pending library inspection never permits import confirmation")
        require(try library(&libraryPending, pending, add: 2, now: 60)
                == .rejected("IMPORT_DISPATCH_DEADLINE_EXCEEDED"),
                "library pending delivery keeps original host60s deadline")
        require(!(try library(&libraryPending, exited(59), add: 2, now: 61)).canConfirm,
                "expired library delivery cannot revive on a later receipt")

        var libraryLatest = QADifferentialImportTiming(searchStartedAt: at(1))
        require(try library(&libraryLatest, exited(60), add: 2, now: 60)
                == .inspect(deadline: at(90), canConfirm: true),
                "latest valid exit anchors only30s library budget")
        require(!(try library(&libraryLatest, exited(60), add: 2, now: 90)).canConfirm,
                "latest library deadline rejects the exact boundary")
        require(libraryLatest.libraryDecision(after: .rejected("IMPORT_DISPATCH_EXIT_NONZERO"),
                                              addTappedAt: at(2), now: at(61))
                == .rejected("IMPORT_DISPATCH_EXIT_NONZERO"),
                "library cannot mask a process failure")

        for (name, add, now) in [
            ("Add before actual search", at(0), at(2)),
            ("clock before Add", at(3), at(2)),
            ("nonfinite Add", Date(timeIntervalSince1970: .nan), at(2)),
            ("nonfinite observation", at(2), Date(timeIntervalSince1970: .nan))
        ] {
            let policy = QADifferentialImportTiming(searchStartedAt: at(1))
            require(policy.libraryDecision(after: .exited(at: at(1)),
                                           addTappedAt: add, now: now)
                    == .rejected("IMPORT_LIBRARY_CLOCK_INVALID"), name + " rejects library confirmation")
        }

        let failures = checks.filter { $0["outcome"] as? String == "FAIL" }
        let output: [String: Any] = [
            "schemaVersion": 1, "contract": "dispatch-exit-anchored-add-v1",
            "localFoundationContract": failures.isEmpty ? "PASS" : "FAIL",
            "checks": checks.count, "failures": failures.count, "results": checks,
            "appleRuntime": "NOT_RUN", "oldImplementationExecution": "NOT_RUN"
        ]
        FileHandle.standardOutput.write(try JSONSerialization.data(withJSONObject: output, options: [.sortedKeys]))
        FileHandle.standardOutput.write(Data([0x0a]))
        if !failures.isEmpty {
            throw NSError(domain: "QADifferentialImportTimingContracts", code: 1,
                          userInfo: [NSLocalizedDescriptionKey: "\(failures.count) timing controls failed"])
        }
    }
}
