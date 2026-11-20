// Concatenate after EditorStartupDeadline.swift, then execute with Swift.
// These are controls for the actual helper, not Apple UI execution evidence.
final class StartupTestClock { var value: TimeInterval = 0 }
var checked = 0
func check(_ value: Bool, _ label: String) {
    guard value else { fatalError("FAIL: " + label) }
    checked += 1
}
let readyState = CherriEditorStartupObservation(
    windowExists: true, appRunningForeground: true,
    createDocumentExists: false, createDocumentHittable: false,
    editorExists: true, editorHittable: true)
let creationState = CherriEditorStartupObservation(
    windowExists: true, appRunningForeground: true,
    createDocumentExists: true, createDocumentHittable: true,
    editorExists: false, editorHittable: false)
let clock = StartupTestClock()
var budget = CherriEditorStartupDeadline(now: { clock.value })
check(budget.remaining() == 28, "original total budget")
check(budget.remaining(maximum: 10) == 10, "stage remains bounded")
clock.value = 4
check(budget.remaining() == 24, "unused earlier time reaches final transition")
check(budget.claimDocumentCreation(creationState), "first owned creation allowed")
check(!budget.claimDocumentCreation(creationState), "second creation refused")
clock.value = 16
check(budget.accepts(readyState, waitCompleted: true), "real ready state within shared budget")
check(!budget.accepts(readyState, waitCompleted: false), "expired wait cannot claim success")
let states = [
    CherriEditorStartupObservation(windowExists: false, appRunningForeground: true, createDocumentExists: false, createDocumentHittable: false, editorExists: true, editorHittable: true),
    CherriEditorStartupObservation(windowExists: true, appRunningForeground: false, createDocumentExists: false, createDocumentHittable: false, editorExists: true, editorHittable: true),
    CherriEditorStartupObservation(windowExists: true, appRunningForeground: true, createDocumentExists: false, createDocumentHittable: false, editorExists: false, editorHittable: true),
    CherriEditorStartupObservation(windowExists: true, appRunningForeground: true, createDocumentExists: false, createDocumentHittable: false, editorExists: true, editorHittable: false),
]
for (index, state) in states.enumerated() {
    check(!budget.accepts(state, waitCompleted: true), "current required state missing \(index)")
}
clock.value = 28.001
check(!budget.accepts(readyState, waitCompleted: true), "late interactive editor rejected")
check(budget.remaining() == 0, "expired budget cannot authorize another wait")
let expiredClock = StartupTestClock()
var expired = CherriEditorStartupDeadline(now: { expiredClock.value })
expiredClock.value = 29
check(!expired.claimDocumentCreation(creationState), "late creation rejected")
let raceClock = StartupTestClock()
var race = CherriEditorStartupDeadline(now: { raceClock.value })
raceClock.value = 2
let editorAppeared = CherriEditorStartupObservation(
    windowExists: true, appRunningForeground: true,
    createDocumentExists: true, createDocumentHittable: true,
    editorExists: true, editorHittable: true)
check(!race.claimDocumentCreation(editorAppeared), "editor arriving during creation wait forbids a tap")
check(!race.documentCreationIssued, "co-present editor and button issue no creation")
check(race.accepts(editorAppeared, waitCompleted: true), "appeared editor can pass final readiness")
let invalidClock = StartupTestClock()
invalidClock.value = 10
let invalid = CherriEditorStartupDeadline(now: { invalidClock.value })
invalidClock.value = 9
check(invalid.remaining() == 0, "clock reversal fails closed")
check(!invalid.accepts(readyState, waitCompleted: true), "clock reversal cannot authorize readiness")
invalidClock.value = .nan
check(invalid.remaining() == 0, "nonfinite clock fails closed")
clock.value = 20
check(budget.remaining(maximum: -1) == 0, "negative wait refused")
check(budget.remaining(maximum: .nan) == 0, "nonfinite wait refused")
check(budget.elapsedMilliseconds == 20_000, "observed monotonic time retained")
// Actual policy used by ensureEditorReady. These times come from the K run;
// they establish local control flow only, never a repaired Apple UI result.
let fastClock = StartupTestClock()
var fast = CherriEditorStartupDeadline(now: { fastClock.value })
fastClock.value = 4.743
check(!fast.needsDocumentWait(creationState), "ready Create Document must not trigger another wait")
check(!fast.needsDocumentWait(editorAppeared), "existing editor bypasses creation wait")
var waits = 0
if fast.needsDocumentWait(creationState) { waits += 1; fastClock.value += 26 }
check(waits == 0, "ready observation avoids the reproduced redundant waiting path")
check(fast.claimDocumentCreation(creationState), "fresh creation state is actionable before the unchanged deadline")
check(!fast.claimDocumentCreation(creationState), "optimized path still permits only one tap")
let pendingCreation = CherriEditorStartupObservation(
    windowExists: true, appRunningForeground: true,
    createDocumentExists: true, createDocumentHittable: false,
    editorExists: false, editorHittable: false)
var pending = CherriEditorStartupDeadline(now: { fastClock.value })
check(pending.needsDocumentWait(pendingCreation), "non-hittable control still needs real readiness")
check(!fast.needsDocumentWait(pendingCreation), "already-issued creation does not start another document wait")
check(!pending.claimDocumentCreation(pendingCreation), "non-hittable control never authorizes a tap")
check(!pending.needsDocumentWait(states[0]), "missing window does not enter another wait")
check(!pending.needsDocumentWait(states[1]), "background app does not enter another wait")
fastClock.value = 28.001
check(!fast.needsDocumentWait(pendingCreation), "expired total budget cannot start a control wait")
check(!fast.claimDocumentCreation(creationState), "optimized path cannot issue a late tap")
check(!fast.accepts(readyState, waitCompleted: true), "optimized path still rejects late readiness")
check(!pending.claimDocumentCreation(editorAppeared), "editor arriving during wait remains protected")
print("PASS_LOCAL_STARTUP_DEADLINE_CONTROLS: \(checked); Apple UI NOT_RUN")
