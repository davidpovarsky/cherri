import Foundation

@main
struct DebugLabelPreviewControls {
    static var assertions = 0
    static func check(_ condition: @autoclosure () -> Bool, _ name: String) {
        precondition(condition(), name)
        assertions += 1
    }
    static func context(_ mode: String = "typed", nonce: String? = nil) -> QADebugLabelPreviewContext {
        let fixture = "qa-debug-label-\(mode)-v2"
        return QADebugLabelPreviewContext(schemaVersion: "1", fixtureId: fixture,
            sourceSha256: String(repeating: "b", count: 64),
            compilerCommitSha: String(repeating: "a", count: 40), runId: "12345", runAttempt: 1,
            runNonce: nonce ?? fixture + "-12345-" + String(repeating: "c", count: 32), mode: mode)
    }
    static func observe(_ policy: inout QADebugLabelPreviewPolicy, at time: Int,
                        foreground: Bool = true, container: Bool = true,
                        done: Bool = true, labels: [Bool] = Array(repeating: true, count: 5),
                        dates: [Bool] = Array(repeating: false, count: 5)) -> QADebugLabelPreviewPolicy.Decision {
        policy.observe(startedMs: time, observedMs: time + 1, foreground: foreground,
                       previewContainer: container, doneExists: done, doneHittable: done,
                       labels: labels, dateSuffixes: dates)
    }
    static func main() throws {
        for mode in ["plain", "dated"] {
            check(QADebugLabelPreviewPolicy(context: context(mode)) == nil, "no preview for \(mode)")
        }
        check(QADebugLabelPreviewPolicy(context: context("private")) == nil, "unknown mode")
        check(QADebugLabelPreviewPolicy(context: context(nonce: "stale")) == nil, "stale nonce")
        check(!QADebugLabelPreviewPolicy.reviewedPreviewTitle("Unrelated sheet", context: context(), visibleLabel: true, previouslyOwned: true), "unrelated Done sheet never inherits ownership")
        check(QADebugLabelPreviewPolicy.reviewedPreviewTitle("Number", context: context(), visibleLabel: false, previouslyOwned: true), "reviewed empty-label Number continuation")
        check(!QADebugLabelPreviewPolicy.reviewedPreviewTitle("Number", context: context(), visibleLabel: false, previouslyOwned: false), "generic initial page has no public label witness")
        check(QADebugLabelPreviewPolicy.labelVisible("FILE_LABEL.bin", label: "FILE_LABEL", context: context()), "reviewed native extension")
        check(!QADebugLabelPreviewPolicy.labelVisible("FILE_LABEL_WRONG", label: "FILE_LABEL", context: context()), "prefix lookalike rejected")
        check(!QADebugLabelPreviewPolicy.labelVisible("FILE_LABEL - Nov 20, 2026", label: "FILE_LABEL", context: context()), "dated title cannot satisfy typed mode")
        check(QADebugLabelPreviewPolicy.labelVisible("TEXT_שָׁלוֹם_🍒: ערך 🍒", label: "TEXT_שָׁלוֹם_🍒", context: context("text")), "exact Unicode label boundary")
        for mode in ["typed", "dated-typed", "text", "dated-text"] {
            var policy = QADebugLabelPreviewPolicy(context: context(mode))!
            let dates = Array(repeating: mode.hasPrefix("dated-"), count: 5)
            check(observe(&policy, at: 10, dates: dates) == .dismiss, "all current visible labels")
            check(policy.completed(.dismiss, at: 20), "actual completed dismissal")
            check(policy.receipt(elapsedMs: 25).result == "PASS_VISIBLE_LABELS", "bounded visible result")
            check(!policy.completed(.dismiss, at: 26), "no duplicated dismissal")
            check(observe(&policy, at: 27, dates: dates) == .stop, "no queries after dismissal")
        }
        for field in 0..<3 {
            var policy = QADebugLabelPreviewPolicy(context: context())!
            check(observe(&policy, at: 10, foreground: field != 0,
                          container: field != 1, done: field != 2) == .wait, "ownership prerequisite")
            check(!policy.completed(.dismiss, at: 20), "no synthetic completion without owner")
            check(policy.receipt(elapsedMs: 25).result == "BLOCKED_PREVIEW_NOT_OBSERVED", "unobserved preview")
        }
        var late = QADebugLabelPreviewPolicy(context: context())!
        check(observe(&late, at: 21999) == .stop, "deadline checked after AX observations")
        check(!late.completed(.dismiss, at: 22001), "no tap after observation deadline")
        check(late.receipt(elapsedMs: 22002).result != "PASS_VISIBLE_LABELS", "late first observation")
        var lateTap = QADebugLabelPreviewPolicy(context: context())!
        check(observe(&lateTap, at: 21900) == .dismiss, "in-budget tap request")
        check(lateTap.completed(.dismiss, at: 22001), "retain actual late tap")
        check(lateTap.receipt(elapsedMs: 22002).result == "BLOCKED_PREVIEW_DEADLINE", "late action never passes")
        var lost = QADebugLabelPreviewPolicy(context: context())!
        let first = [true, false, false, false, false]
        check(observe(&lost, at: 1, labels: first) == .swipe, "bounded pagination")
        check(lost.completed(.swipe, at: 3), "record real swipe")
        check(observe(&lost, at: 4, foreground: false) == .stop, "foreground loss stops")
        check(lost.receipt(elapsedMs: 5).result == "BLOCKED_PREVIEW_OWNERSHIP_LOST", "no borrowed later state")
        check(observe(&lost, at: 6) == .stop, "no second ownership recovery")
        var differentSheet = QADebugLabelPreviewPolicy(context: context())!
        check(observe(&differentSheet, at: 1, labels: first) == .swipe, "own first public preview")
        check(differentSheet.completed(.swipe, at: 3), "first swipe completed")
        check(observe(&differentSheet, at: 4, container: false) == .stop, "unrelated foreground Done sheet stops")
        check(!differentSheet.completed(.dismiss, at: 6), "never tap unrelated Done")
        var incomplete = QADebugLabelPreviewPolicy(context: context())!
        for index in 0..<6 {
            check(observe(&incomplete, at: index*3, labels: first) == .swipe, "finite swipe \(index)")
            check(incomplete.completed(.swipe, at: index*3+2), "record swipe \(index)")
        }
        check(observe(&incomplete, at: 20, labels: first) == .dismiss, "bounded stop after six swipes")
        check(incomplete.completed(.dismiss, at: 22), "dismiss incomplete preview truthfully")
        check(incomplete.receipt(elapsedMs: 24).result == "BLOCKED_PREVIEW_LABELS_INCOMPLETE", "do not substitute one visible label")
        var dateMissing = QADebugLabelPreviewPolicy(context: context("dated-text"))!
        check(observe(&dateMissing, at: 21000) == .dismiss, "time remaining bounds collection")
        check(dateMissing.completed(.dismiss, at: 21002), "actual incomplete date dismissal")
        check(dateMissing.receipt(elapsedMs: 21004).result == "BLOCKED_PREVIEW_DATES_INCOMPLETE", "date suffix is still required")
        var unexpectedDate = QADebugLabelPreviewPolicy(context: context("text"))!
        check(observe(&unexpectedDate, at: 1, dates: Array(repeating: true, count: 5)) == .dismiss, "retain unexpected date display")
        check(unexpectedDate.completed(.dismiss, at: 3), "dismiss unexpected date preview truthfully")
        check(unexpectedDate.receipt(elapsedMs: 4).result == "BLOCKED_PREVIEW_UNEXPECTED_DATES", "non-dated mode never accepts dates")
        var temporal = QADebugLabelPreviewPolicy(context: context())!
        check(observe(&temporal, at: 10, labels: first) == .swipe, "temporal baseline")
        check(!temporal.completed(.dismiss, at: 12), "cannot replace planned swipe with completion")
        check(!temporal.completed(.swipe, at: 9), "action cannot predate observation")
        check(temporal.completed(.swipe, at: 12), "retain valid action")
        check(observe(&temporal, at: 5) == .stop, "no backward clock observation")
        var badFlags = QADebugLabelPreviewPolicy(context: context())!
        check(observe(&badFlags, at: 1, labels: first, dates: Array(repeating: true, count: 5)) == .stop,
              "date suffix must have visible label")
        let raw = try JSONEncoder().encode(incomplete.receipt(elapsedMs: 24))
        let decoded = try JSONDecoder().decode(QADebugLabelPreviewReceipt.self, from: raw)
        check(decoded.result == "BLOCKED_PREVIEW_LABELS_INCOMPLETE", "Codable preserves failure")
        check(!decoded.creationDateValueIndependentlyVerified && !decoded.physicalDeviceVerified, "honest observation scope")
        print("PASS: \(assertions) debug-label preview policy assertions; synthetic controls only, Apple NOT_RUN")
    }
}
