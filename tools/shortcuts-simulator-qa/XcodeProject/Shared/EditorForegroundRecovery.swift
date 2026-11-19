import Foundation

/// Fixed application states only; no accessibility strings or user content.
enum QAAppLivenessState: String, Codable {
    case unknown, notRunning, runningBackgroundSuspended, runningBackground, runningForeground
}

struct QAEditorForegroundObservation: Codable {
    let priorState: QAAppLivenessState
    let afterActivationState: QAAppLivenessState?
    let finalState: QAAppLivenessState
    let activationAttempted: Bool
    let launchAttempted: Bool
    let waitSucceeded: Bool
    let waitTimeoutSeconds: [TimeInterval]
    let budgetSeconds: TimeInterval
    let elapsedSeconds: TimeInterval
    let budgetExceeded: Bool

    var canInspect: Bool {
        !budgetExceeded && finalState == .runningForeground &&
            (priorState == .runningForeground || (activationAttempted && waitSucceeded))
    }
}

enum QAEditorForegroundRecovery {
    /// One activation and, only if the process is unavailable, one launch.
    /// Both waits share the same deadline. A failed wait or a late state change
    /// is not successful recovery. Never terminate a running import session.
    static func acquire(
        budgetSeconds: TimeInterval,
        activationWaitSeconds: TimeInterval,
        state: () -> QAAppLivenessState,
        activate: () -> Void,
        launch: () -> Void,
        waitForForeground: (TimeInterval) -> Bool,
        now: () -> TimeInterval = { ProcessInfo.processInfo.systemUptime }
    ) -> QAEditorForegroundObservation {
        let started = now()
        let budget = min(25, max(0, budgetSeconds))
        let deadline = started + budget
        let before = state()
        var afterActivation: QAAppLivenessState?
        var activated = false
        var launched = false
        var waited = false
        var waits: [TimeInterval] = []
        func remaining() -> TimeInterval { max(0, deadline - now()) }
        func wait(_ maximum: TimeInterval) -> Bool {
            let timeout = min(max(0, maximum), remaining())
            guard timeout > 0 else { return false }
            waits.append(timeout)
            return waitForForeground(timeout)
        }
        if before != .runningForeground && remaining() > 0 {
            activated = true
            activate()
            afterActivation = state()
            waited = wait(activationWaitSeconds)
            let afterWait = state()
            if afterWait == .notRunning || afterWait == .unknown {
                if remaining() > 0 {
                    launched = true
                    launch()
                    waited = wait(remaining())
                }
            }
        }
        let final = state()
        let elapsed = max(0, now() - started)
        return QAEditorForegroundObservation(
            priorState: before, afterActivationState: afterActivation, finalState: final,
            activationAttempted: activated, launchAttempted: launched,
            waitSucceeded: waited, waitTimeoutSeconds: waits,
            budgetSeconds: budget, elapsedSeconds: elapsed,
            budgetExceeded: elapsed > budget)
    }

    /// Check the live state again immediately before entering accessibility.
    static func inspect<T>(_ observation: QAEditorForegroundObservation,
                           state: () -> QAAppLivenessState,
                           query: () -> T) -> T? {
        guard observation.canInspect, state() == .runningForeground else { return nil }
        return query()
    }
}

struct QAEditorForegroundReceipt: Codable {
    let schemaVersion: String
    let stage: String
    let transition: QAEditorForegroundObservation
    let importRequestIssuedBefore: Bool
    let importRequestIssuedAfter: Bool
    let retryRequestIssuedBefore: Bool
    let retryRequestIssuedAfter: Bool

    var importRequestStatePreserved: Bool {
        importRequestIssuedBefore == importRequestIssuedAfter &&
            retryRequestIssuedBefore == retryRequestIssuedAfter
    }
}
