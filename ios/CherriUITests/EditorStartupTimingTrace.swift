import Foundation

/// Diagnostic timing only. No observation here authorizes readiness or a tap.
/// The original deadline and completed-wait predicates remain the acceptance gate.
final class CherriEditorStartupTimingTrace {
    enum Phase: String {
        case windowSnapshot = "window-snapshot"
        case documentSnapshot = "document-snapshot"
        case finalSnapshot = "final-snapshot"
        case windowWait = "window-wait"
        case documentWait = "document-wait"
        case editorWait = "editor-wait"
        case documentTap = "document-tap"
    }

    enum Operation: String {
        case foreground, window, entered, returned
        case createExists = "create-exists"
        case createHittable = "create-hittable"
        case editorExists = "editor-exists"
        case editorHittable = "editor-hittable"
        case foregroundFinal = "foreground-final"
    }

    static let maximumEntries = 128
    private let startedAt: TimeInterval
    private let now: () -> TimeInterval
    private let lock = NSLock()
    private var entries: [[String: Any]] = []
    private var droppedEntries = 0
    private var inFlightObservations = 0

    init(startedAt: TimeInterval, now: @escaping () -> TimeInterval = { ProcessInfo.processInfo.systemUptime }) {
        self.startedAt = startedAt
        self.now = now
    }

    private var elapsedMilliseconds: Int {
        let elapsed = now() - startedAt
        guard elapsed.isFinite, elapsed >= 0 else { return 120_000 }
        return Int(min(120_000, elapsed * 1_000))
    }

    @discardableResult
    func measure(_ phase: Phase, _ operation: Operation, _ observation: () -> Bool) -> Bool {
        let started = elapsedMilliseconds
        lock.lock()
        inFlightObservations += 1
        lock.unlock()
        let value = observation()
        append(phase, operation, started: started, completed: elapsedMilliseconds, value: value, completesObservation: true)
        return value
    }

    func mark(_ phase: Phase, _ operation: Operation, value: Bool) {
        let elapsed = elapsedMilliseconds
        append(phase, operation, started: elapsed, completed: elapsed, value: value, completesObservation: false)
    }

    private func append(_ phase: Phase, _ operation: Operation, started: Int, completed: Int, value: Bool,
                        completesObservation: Bool) {
        lock.lock()
        defer { lock.unlock() }
        if completesObservation { inFlightObservations -= 1 }
        guard entries.count < Self.maximumEntries else {
            droppedEntries = min(10_000, droppedEntries + 1)
            return
        }
        entries.append([
            "sequence": entries.count, "phase": phase.rawValue, "operation": operation.rawValue,
            "startedMilliseconds": started, "completedMilliseconds": completed, "value": value,
            "mainThread": Thread.isMainThread,
        ])
    }

    func encoded(testName: String) -> Data? {
        lock.lock()
        let observed = entries
        let dropped = droppedEntries
        let inFlight = inFlightObservations
        lock.unlock()
        let receipt: [String: Any] = [
            "schemaVersion": "1.0", "testName": testName,
            "budgetMilliseconds": 28_000, "elapsedMilliseconds": elapsedMilliseconds,
            "entries": observed, "droppedEntries": dropped, "inFlightObservations": inFlight,
        ]
        return try? JSONSerialization.data(withJSONObject: receipt, options: [.sortedKeys])
    }
}
