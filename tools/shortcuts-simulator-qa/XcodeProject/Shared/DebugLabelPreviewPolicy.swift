import Foundation

// A public-fixture-only UI observer. Raw labels, AX trees and screen pixels are
// never written by this receipt. Native values are checked by the separate,
// unchanged exact callback oracle after Quick Look returns.
struct QADebugLabelPreviewContext: Codable {
    let schemaVersion: String
    let fixtureId: String
    let sourceSha256: String
    let compilerCommitSha: String
    let runId: String
    let runAttempt: Int
    let runNonce: String
    let mode: String

    var isValid: Bool {
        let modes = ["plain", "dated", "typed", "dated-typed", "text", "dated-text"]
        func matches(_ value: String, _ expression: String) -> Bool {
            value.range(of: expression, options: .regularExpression) != nil
        }
        return schemaVersion == "1" && modes.contains(mode)
            && ["qa-debug-label-\(mode)-v2", "qa-debug-label-\(mode)-legacy"].contains(fixtureId)
            && matches(sourceSha256, "^[0-9a-f]{64}$")
            && matches(compilerCommitSha, "^[0-9a-f]{40}$")
            && matches(runId, "^[1-9][0-9]{0,19}$") && (1...100).contains(runAttempt)
            && matches(runNonce, "^" + NSRegularExpression.escapedPattern(for: fixtureId + "-" + runId + "-") + "[0-9a-f]{32}$")
    }
    var requiresPreview: Bool { ["typed", "dated-typed", "text", "dated-text"].contains(mode) }
    var requiresDateSuffix: Bool { mode.hasPrefix("dated-") }
    var showsText: Bool { mode == "text" || mode == "dated-text" }
}

struct QADebugLabelPreviewObservation: Codable {
    let sequence: Int
    let startedMs: Int
    let observedMs: Int
    let foreground: Bool
    let previewContainer: Bool
    let doneExists: Bool
    let doneHittable: Bool
    let visibleLabels: [Bool]
    let visibleDateSuffixes: [Bool]
    var action: String
    var actionCompletedMs: Int
}

struct QADebugLabelPreviewReceipt: Codable {
    let schemaVersion: String
    let context: QADebugLabelPreviewContext
    let budgetMs: Int
    let elapsedMs: Int
    let maximumSwipes: Int
    let observations: [QADebugLabelPreviewObservation]
    let visibleLabels: [Bool]
    let visibleDateSuffixes: [Bool]
    let swipeCount: Int
    let dismissed: Bool
    let ownershipLost: Bool
    let result: String
    let creationDateValueIndependentlyVerified: Bool
    let physicalDeviceVerified: Bool
}

struct QADebugLabelPreviewPolicy {
    enum Decision { case wait, swipe, dismiss, stop }
    static let budgetMs = 22_000
    static let maximumSwipes = 6
    static let maximumObservations = 16
    // The empty label is tested by exact metadata key/value observations; it is
    // deliberately not replaced with a visible title or a synthetic UI token.
    static let labels = ["TEXT_שָׁלוֹם_🍒", "MAP_LABEL", "LIST_LABEL", "DATE_LABEL", "FILE_LABEL"]

    static func labelVisible(_ value: String, label: String, context: QADebugLabelPreviewContext) -> Bool {
        if context.showsText { return value.hasPrefix(label + ": ") }
        if context.requiresDateSuffix { return value.hasPrefix(label + " - ") && dateSuffixVisible(value, label: label) }
        return value == label || [".txt", ".json", ".bin"].contains { value == label + $0 }
    }

    static func dateSuffixVisible(_ value: String, label: String) -> Bool {
        guard value.hasPrefix(label), let separator = value.range(of: " - ", options: .backwards) else { return false }
        let suffix = value[separator.upperBound...].trimmingCharacters(in: .whitespacesAndNewlines)
        return !suffix.isEmpty && suffix.count <= 128 && suffix.rangeOfCharacter(from: .decimalDigits) != nil
    }

    static func reviewedPreviewTitle(_ title: String, context: QADebugLabelPreviewContext,
                                     visibleLabel: Bool, previouslyOwned: Bool) -> Bool {
        if labels.contains(where: { labelVisible(title, label: $0, context: context) }) { return true }
        let generic = ["Text", "Quick Look", "Preview", "Number", "Date", "7.25"].contains(title)
            || title.range(of: "^[1-7] of 7$", options: .regularExpression) != nil
        // Every later observation still needs a reviewed concrete title. A Done
        // button on an unrelated sheet never inherits prior preview ownership.
        return generic && (visibleLabel || previouslyOwned)
    }

    let context: QADebugLabelPreviewContext
    private(set) var observations: [QADebugLabelPreviewObservation] = []
    private(set) var hasOwnedPreview = false
    private(set) var ownershipLost = false
    private(set) var visibleLabels = Array(repeating: false, count: 5)
    private(set) var visibleDateSuffixes = Array(repeating: false, count: 5)
    private(set) var swipeCount = 0
    private(set) var dismissed = false
    private var pendingDecision: Decision = .wait

    init?(context: QADebugLabelPreviewContext) {
        guard context.isValid && context.requiresPreview else { return nil }
        self.context = context
    }

    mutating func observe(startedMs: Int, observedMs: Int, foreground: Bool,
                          previewContainer: Bool, doneExists: Bool, doneHittable: Bool,
                          labels: [Bool], dateSuffixes: [Bool]) -> Decision {
        guard !dismissed, !ownershipLost, observations.count < Self.maximumObservations,
              labels.count == 5, dateSuffixes.count == 5,
              startedMs >= (observations.last?.actionCompletedMs ?? 0), observedMs >= startedMs,
              zip(dateSuffixes, labels).allSatisfy({ !$0.0 || $0.1 }) else {
            pendingDecision = .stop
            return .stop
        }
        observations.append(QADebugLabelPreviewObservation(sequence: observations.count,
            startedMs: startedMs, observedMs: observedMs, foreground: foreground,
            previewContainer: previewContainer, doneExists: doneExists,
            doneHittable: doneHittable, visibleLabels: labels,
            visibleDateSuffixes: dateSuffixes, action: "NONE", actionCompletedMs: observedMs))
        guard observedMs < Self.budgetMs else { pendingDecision = .stop; return .stop }
        guard foreground && previewContainer && doneExists && doneHittable else {
            if hasOwnedPreview { ownershipLost = true; pendingDecision = .stop; return .stop }
            pendingDecision = .wait
            return .wait
        }
        hasOwnedPreview = true
        for index in 0..<5 {
            visibleLabels[index] = visibleLabels[index] || labels[index]
            visibleDateSuffixes[index] = visibleDateSuffixes[index] || dateSuffixes[index]
        }
        let complete = visibleLabels.allSatisfy { $0 }
            && (!context.requiresDateSuffix || visibleDateSuffixes.allSatisfy { $0 })
        pendingDecision = complete || swipeCount >= Self.maximumSwipes
            || Self.budgetMs - observedMs < 2_000 ? .dismiss : .swipe
        return pendingDecision
    }

    mutating func completed(_ decision: Decision, at elapsedMs: Int) -> Bool {
        guard let last = observations.last, last.action == "NONE",
              last.foreground && last.previewContainer && last.doneExists && last.doneHittable,
              last.observedMs < Self.budgetMs && elapsedMs >= last.observedMs,
              (decision == .swipe || decision == .dismiss), decision == pendingDecision else { return false }
        if decision == .swipe {
            guard swipeCount < Self.maximumSwipes else { return false }
            swipeCount += 1
        } else {
            dismissed = true
        }
        observations[observations.count - 1].action = decision == .swipe ? "SWIPE" : "DISMISS"
        observations[observations.count - 1].actionCompletedMs = elapsedMs
        pendingDecision = .wait
        return true
    }

    func receipt(elapsedMs: Int) -> QADebugLabelPreviewReceipt {
        let late = elapsedMs > Self.budgetMs || elapsedMs < (observations.last?.actionCompletedMs ?? 0)
        let result: String
        if !hasOwnedPreview { result = "BLOCKED_PREVIEW_NOT_OBSERVED" }
        else if ownershipLost { result = "BLOCKED_PREVIEW_OWNERSHIP_LOST" }
        else if late { result = "BLOCKED_PREVIEW_DEADLINE" }
        else if !dismissed { result = "BLOCKED_PREVIEW_NOT_DISMISSED" }
        else if !visibleLabels.allSatisfy({ $0 }) { result = "BLOCKED_PREVIEW_LABELS_INCOMPLETE" }
        else if !context.requiresDateSuffix && visibleDateSuffixes.contains(true) { result = "BLOCKED_PREVIEW_UNEXPECTED_DATES" }
        else if context.requiresDateSuffix && !visibleDateSuffixes.allSatisfy({ $0 }) { result = "BLOCKED_PREVIEW_DATES_INCOMPLETE" }
        else { result = "PASS_VISIBLE_LABELS" }
        return QADebugLabelPreviewReceipt(schemaVersion: "1", context: context,
            budgetMs: Self.budgetMs, elapsedMs: elapsedMs, maximumSwipes: Self.maximumSwipes,
            observations: observations, visibleLabels: visibleLabels,
            visibleDateSuffixes: visibleDateSuffixes, swipeCount: swipeCount,
            dismissed: dismissed, ownershipLost: ownershipLost, result: result,
            creationDateValueIndependentlyVerified: false, physicalDeviceVerified: false)
    }
}
