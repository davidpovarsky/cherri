import Foundation

@main
enum QAImportForegroundContracts {
    static func main() throws {
        func require(_ condition: @autoclosure () -> Bool, _ message: String) throws {
            if !condition() {
                throw NSError(domain: "QAImportForegroundContracts", code: 1,
                              userInfo: [NSLocalizedDescriptionKey: message])
            }
        }
        struct Scenario {
            let name: String
            let before: Bool
            let after: Bool
            let waitResult: Bool
            let expectedAllowed: Bool
            let expectedActivation: Int
        }
        let cases = [
            Scenario(name: "already-foreground", before: true, after: true, waitResult: false, expectedAllowed: true, expectedActivation: 0),
            Scenario(name: "reacquired", before: false, after: true, waitResult: true, expectedAllowed: true, expectedActivation: 1),
            Scenario(name: "activation-without-foreground", before: false, after: false, waitResult: false, expectedAllowed: false, expectedActivation: 1),
            Scenario(name: "wait-failed-even-if-late-state", before: false, after: true, waitResult: false, expectedAllowed: false, expectedActivation: 1),
            Scenario(name: "wait-succeeded-but-app-gone", before: false, after: false, waitResult: true, expectedAllowed: false, expectedActivation: 1),
            Scenario(name: "foreground-lost-before-return", before: true, after: false, waitResult: false, expectedAllowed: false, expectedActivation: 0),
        ]
        var observations: [QALibraryForegroundObservation] = []
        var assertions = 0
        for item in cases {
            var reads = 0
            var activations = 0
            var waits = 0
            var deadlines: [TimeInterval] = []
            let observation = QALibraryForegroundGate.acquire(
                isForeground: { reads += 1; return reads == 1 ? item.before : item.after },
                activate: { activations += 1 },
                waitForForeground: { waits += 1; deadlines.append($0); return item.waitResult })
            try require(reads == 2, item.name + ": must reread actual state")
            try require(activations == item.expectedActivation, item.name + ": activation must be bounded")
            try require(waits == item.expectedActivation, item.name + ": wait must accompany activation")
            try require(deadlines.allSatisfy { $0 == 10 }, item.name + ": exact bounded timeout")
            try require(observation.canInspect == item.expectedAllowed, item.name + ": readiness")
            var queries = 0
            let result: Int? = QALibraryForegroundGate.inspect(
                observation, isForeground: { item.after }, query: { queries += 1; return 7 })
            try require(queries == (item.expectedAllowed ? 1 : 0), item.name + ": no unready descendant query")
            try require(result == (item.expectedAllowed ? 7 : nil), item.name + ": exact query result")
            observations.append(observation)
            assertions += 7
        }
        let ready = observations[1]
        var unsafeQueryCalls = 0
        // Reproduce the previous unconditional library-query behavior using
        // an explicit unavailable-app control, without claiming Apple execution.
        let unavailable = false
        func unsafeQuery() -> Int { unsafeQueryCalls += 1; return unavailable ? 7 : -1 }
        try require(unsafeQuery() == -1 && unsafeQueryCalls == 1, "old unconditional query reaches unavailable app")
        unsafeQueryCalls = 0
        let suppressed: Int? = QALibraryForegroundGate.inspect(
            ready, isForeground: { unavailable }, query: unsafeQuery)
        try require(suppressed == nil && unsafeQueryCalls == 0, "fresh state must suppress descendant query")
        let lost = ready.stoppingAfterForegroundLoss()
        let notRecovered: Int? = QALibraryForegroundGate.inspect(
            lost, isForeground: { true }, query: unsafeQuery)
        try require(notRecovered == nil && unsafeQueryCalls == 0, "lost gate cannot silently regain acceptance")
        let data = try JSONEncoder().encode(lost)
        let dictionary = try JSONSerialization.jsonObject(with: data) as! [String: Any]
        try require(Set(dictionary.keys) == Set(["initiallyForeground", "activationAttempted", "waitAttempted", "waitSucceeded", "finallyForeground", "lostBeforeQuery"]), "receipt exact key allowlist")
        try require(dictionary.values.allSatisfy { $0 is Bool }, "receipt booleans only")
        let restored = try JSONDecoder().decode(QALibraryForegroundObservation.self, from: data)
        try require(!restored.canInspect && restored.lostBeforeQuery, "failed receipt survives serialization")
        assertions += 6
        let output: [String: Any] = [
            "schemaVersion": "1.0", "localFoundationContract": "PASS",
            "scenarios": cases.count, "assertions": assertions,
            "oldUnconditionalQueryReproduced": true,
            "unavailableAppQuerySuppressed": true,
            "activationBoundedToOne": true, "waitTimeoutSeconds": 10,
            "receiptBooleansOnly": true, "appleRuntime": "NOT_RUN"
        ]
        print(String(decoding: try JSONSerialization.data(withJSONObject: output, options: [.sortedKeys]), as: UTF8.self))
    }
}
