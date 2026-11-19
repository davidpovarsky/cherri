import Foundation

/// Fixed booleans only: no accessibility labels, clipboard contents, paths, or
/// application error messages are recorded by this transition receipt.
struct QALibraryForegroundObservation: Codable {
    let initiallyForeground: Bool
    let activationAttempted: Bool
    let waitAttempted: Bool
    let waitSucceeded: Bool
    let finallyForeground: Bool
    let lostBeforeQuery: Bool

    var canInspect: Bool {
        !lostBeforeQuery && finallyForeground &&
            (initiallyForeground || (activationAttempted && waitAttempted && waitSucceeded))
    }

    func stoppingAfterForegroundLoss() -> Self {
        Self(initiallyForeground: initiallyForeground,
             activationAttempted: activationAttempted,
             waitAttempted: waitAttempted,
             waitSucceeded: waitSucceeded,
             finallyForeground: finallyForeground,
             lostBeforeQuery: true)
    }
}

enum QALibraryForegroundGate {
    /// Reacquire once after the import/consent transition. Never infer a
    /// successful state from activation alone, and never loop or relaunch.
    static func acquire(isForeground: () -> Bool,
                        activate: () -> Void,
                        waitForForeground: (TimeInterval) -> Bool) -> QALibraryForegroundObservation {
        let before = isForeground()
        var waited = false
        if !before {
            activate()
            waited = waitForForeground(10)
        }
        return QALibraryForegroundObservation(
            initiallyForeground: before,
            activationAttempted: !before,
            waitAttempted: !before,
            waitSucceeded: waited,
            finallyForeground: isForeground(),
            lostBeforeQuery: false)
    }

    /// The descendant snapshot is evaluated only after both the transition
    /// receipt and a fresh app-state read permit it. A later app crash is still
    /// a real XCTest failure; this is not a general crash-recovery mechanism.
    static func inspect<T>(_ observation: QALibraryForegroundObservation,
                           isForeground: () -> Bool,
                           query: () -> T) -> T? {
        guard observation.canInspect, isForeground() else { return nil }
        return query()
    }
}
