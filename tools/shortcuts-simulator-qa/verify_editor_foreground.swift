import Foundation

@main
enum QAEditorForegroundContracts {
    static func main() throws {
        func require(_ condition: @autoclosure () -> Bool, _ message: String) throws {
            if !condition() {
                throw NSError(domain: "QAEditorForegroundContracts", code: 1,
                              userInfo: [NSLocalizedDescriptionKey: message])
            }
        }
        struct Scenario {
            let name: String
            let before: QAAppLivenessState
            let afterActivate: QAAppLivenessState
            let afterLaunch: QAAppLivenessState
            let activationWait: Bool
            let launchWait: Bool
            let activationElapsed: TimeInterval
            let expectedAllowed: Bool
            let expectedActivate: Int
            let expectedLaunch: Int
        }
        let cases = [
            Scenario(name: "already-foreground", before: .runningForeground, afterActivate: .unknown, afterLaunch: .unknown, activationWait: false, launchWait: false, activationElapsed: 0, expectedAllowed: true, expectedActivate: 0, expectedLaunch: 0),
            Scenario(name: "activate-background", before: .runningBackground, afterActivate: .runningForeground, afterLaunch: .unknown, activationWait: true, launchWait: false, activationElapsed: 0, expectedAllowed: true, expectedActivate: 1, expectedLaunch: 0),
            Scenario(name: "launch-stopped-process", before: .notRunning, afterActivate: .notRunning, afterLaunch: .runningForeground, activationWait: false, launchWait: true, activationElapsed: 0, expectedAllowed: true, expectedActivate: 1, expectedLaunch: 1),
            Scenario(name: "unknown-never-recovers", before: .unknown, afterActivate: .unknown, afterLaunch: .notRunning, activationWait: false, launchWait: false, activationElapsed: 0, expectedAllowed: false, expectedActivate: 1, expectedLaunch: 1),
            Scenario(name: "do-not-relaunch-live-import", before: .runningBackground, afterActivate: .runningBackground, afterLaunch: .runningForeground, activationWait: false, launchWait: true, activationElapsed: 0, expectedAllowed: false, expectedActivate: 1, expectedLaunch: 0),
            Scenario(name: "false-wait-late-foreground", before: .runningBackground, afterActivate: .runningForeground, afterLaunch: .unknown, activationWait: false, launchWait: false, activationElapsed: 0, expectedAllowed: false, expectedActivate: 1, expectedLaunch: 0),
            Scenario(name: "true-wait-missing-process", before: .notRunning, afterActivate: .notRunning, afterLaunch: .notRunning, activationWait: true, launchWait: true, activationElapsed: 0, expectedAllowed: false, expectedActivate: 1, expectedLaunch: 1),
            Scenario(name: "activation-consumes-budget", before: .notRunning, afterActivate: .notRunning, afterLaunch: .runningForeground, activationWait: false, launchWait: true, activationElapsed: 10, expectedAllowed: false, expectedActivate: 1, expectedLaunch: 0),
            Scenario(name: "late-activation-fails-closed", before: .notRunning, afterActivate: .runningForeground, afterLaunch: .unknown, activationWait: true, launchWait: false, activationElapsed: 11, expectedAllowed: false, expectedActivate: 1, expectedLaunch: 0),
        ]
        var observations: [QAEditorForegroundObservation] = []
        var assertions = 0
        for item in cases {
            var current = item.before
            var clock: TimeInterval = 100
            var activations = 0
            var launches = 0
            var waits: [TimeInterval] = []
            let observation = QAEditorForegroundRecovery.acquire(
                budgetSeconds: 10, activationWaitSeconds: 5,
                state: { current },
                activate: { activations += 1; current = item.afterActivate; clock += item.activationElapsed },
                launch: { launches += 1; current = item.afterLaunch },
                waitForForeground: { timeout in
                    waits.append(timeout)
                    clock += timeout
                    return launches == 0 ? item.activationWait : item.launchWait
                }, now: { clock })
            try require(observation.canInspect == item.expectedAllowed, item.name + ": exact readiness")
            try require(activations == item.expectedActivate, item.name + ": activation bounded to one")
            try require(launches == item.expectedLaunch, item.name + ": launch bounded to unavailable process")
            try require(waits.reduce(0, +) <= 10, item.name + ": waits share deadline")
            try require(observation.waitTimeoutSeconds == waits, item.name + ": actual wait receipt")
            try require(observation.priorState == item.before && observation.finalState == current, item.name + ": actual before/after states")
            var queries = 0
            let result: Int? = QAEditorForegroundRecovery.inspect(observation, state: { current }, query: { queries += 1; return 7 })
            try require(queries == (item.expectedAllowed ? 1 : 0), item.name + ": unavailable AX suppressed")
            try require(result == (item.expectedAllowed ? 7 : nil), item.name + ": exact query result")
            observations.append(observation)
            assertions += 8
        }
        var lostQueries = 0
        let lost: Int? = QAEditorForegroundRecovery.inspect(observations[1], state: { .notRunning }, query: { lostQueries += 1; return 7 })
        try require(lost == nil && lostQueries == 0, "fresh loss suppresses previously ready query")
        var zeroBudgetOperations = 0
        let zero = QAEditorForegroundRecovery.acquire(budgetSeconds: 0, activationWaitSeconds: 5,
            state: { .notRunning }, activate: { zeroBudgetOperations += 1 }, launch: { zeroBudgetOperations += 1 },
            waitForForeground: { _ in zeroBudgetOperations += 1; return true }, now: { 100 })
        try require(!zero.canInspect && zeroBudgetOperations == 0, "expired budget issues no operation")
        for issued in [false, true] {
            let receipt = QAEditorForegroundReceipt(schemaVersion: "1.0", stage: "editor-entry",
                transition: observations[3], importRequestIssuedBefore: issued, importRequestIssuedAfter: issued,
                retryRequestIssuedBefore: issued, retryRequestIssuedAfter: issued)
            let data = try JSONEncoder().encode(receipt)
            let decoded = try JSONDecoder().decode(QAEditorForegroundReceipt.self, from: data)
            try require(decoded.importRequestStatePreserved, "existing import state is retained")
            try require(!decoded.transition.canInspect, "failure survives serialization")
            assertions += 2
        }
        let changed = QAEditorForegroundReceipt(schemaVersion: "1.0", stage: "editor-entry",
            transition: observations[1], importRequestIssuedBefore: true, importRequestIssuedAfter: false,
            retryRequestIssuedBefore: false, retryRequestIssuedAfter: false)
        try require(!changed.importRequestStatePreserved, "lost issued request rejected")
        assertions += 3
        let output: [String: Any] = ["localFoundationContract": "PASS", "scenarios": cases.count,
            "assertions": assertions, "oneRecoverySequence": true, "waitBudgetSeconds": 10,
            "unavailableAXSuppressed": true, "importStatePreserved": true, "appleRuntime": "NOT_RUN"]
        print(String(decoding: try JSONSerialization.data(withJSONObject: output, options: [.sortedKeys]), as: UTF8.self))
    }
}
