import Foundation

/// One budget for the existing window, document and editor readiness waits.
/// The budget observes only fixed control state and never opens a second document.
struct CherriEditorStartupObservation {
    let windowExists: Bool
    let appRunningForeground: Bool
    let createDocumentExists: Bool
    let createDocumentHittable: Bool
    let editorExists: Bool
    let editorHittable: Bool

    var interactiveEditor: Bool {
        windowExists && appRunningForeground && editorExists && editorHittable
    }

    /// The caller has already evaluated one query restricted to the two fixed
    /// startup controls. Never resolve a missing element to ask if it is hittable.
    /// Ambiguous controls cannot authorize a tap or an interactive editor.
    static func matchedControls(windowExists: Bool, appRunningForeground: Bool,
                                createDocumentCount: Int, editorCount: Int,
                                unexpectedControlCount: Int = 0,
                                createDocumentHittable: () -> Bool,
                                editorHittable: () -> Bool) -> Self {
        let unambiguous = (0...1).contains(createDocumentCount) && (0...1).contains(editorCount) &&
            unexpectedControlCount == 0
        let canInspect = windowExists && appRunningForeground && unambiguous
        return Self(
            windowExists: windowExists, appRunningForeground: appRunningForeground,
            createDocumentExists: createDocumentCount > 0,
            createDocumentHittable: canInspect && createDocumentCount == 1 && createDocumentHittable(),
            editorExists: editorCount > 0,
            editorHittable: canInspect && editorCount == 1 && editorHittable())
    }
}

struct CherriEditorStartupDeadline {
    // Original staged wait allowance: window10 + initial editor2 + document6 + editor10.
    // This is an acceptance/wait budget; a blocked AX call may finish later and fail.
    static let budgetSeconds: TimeInterval = 28
    private let startedAt: TimeInterval
    var diagnosticStartedAt: TimeInterval { startedAt }
    private let now: () -> TimeInterval
    private(set) var documentCreationIssued = false

    init(now: @escaping () -> TimeInterval = { ProcessInfo.processInfo.systemUptime }) {
        self.now = now
        self.startedAt = now()
    }

    private var elapsed: TimeInterval? {
        let value = now() - startedAt
        return value.isFinite && value >= 0 ? value : nil
    }

    var elapsedMilliseconds: Int {
        guard let elapsed else { return 120_000 }
        return Int(min(120_000, elapsed * 1_000))
    }

    func remaining(maximum: TimeInterval = budgetSeconds) -> TimeInterval {
        guard maximum.isFinite, maximum >= 0, let elapsed else { return 0 }
        return min(maximum, max(0, Self.budgetSeconds - elapsed))
    }

    /// An already interactive creation control needs a fresh pre-tap check,
    /// not another wait for an absent editor or the same ready button.
    func needsDocumentWait(_ state: CherriEditorStartupObservation) -> Bool {
        !documentCreationIssued && remaining() > 0 && state.windowExists && state.appRunningForeground &&
            !state.editorExists && !(state.createDocumentExists && state.createDocumentHittable)
    }

    mutating func claimDocumentCreation(_ state: CherriEditorStartupObservation) -> Bool {
        guard !documentCreationIssued, remaining() > 0,
              state.windowExists, state.appRunningForeground,
              state.createDocumentExists, state.createDocumentHittable,
              !state.editorExists else { return false }
        documentCreationIssued = true
        return true
    }

    func accepts(_ state: CherriEditorStartupObservation, waitCompleted: Bool) -> Bool {
        guard let elapsed, elapsed <= Self.budgetSeconds else { return false }
        return waitCompleted && state.interactiveEditor
    }
}
