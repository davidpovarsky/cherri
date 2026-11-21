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

// The production observer evaluates a filtered query once, then invokes these
// hit-test closures only for unique matches. An absent control has no AX lookup.
var creationHitQueries = 0
var editorHitQueries = 0
func matched(_ creates: Int, _ editors: Int, unexpected: Int = 0,
             window: Bool = true, foreground: Bool = true) -> CherriEditorStartupObservation {
    CherriEditorStartupObservation.matchedControls(
        windowExists: window, appRunningForeground: foreground,
        createDocumentCount: creates, editorCount: editors, unexpectedControlCount: unexpected,
        createDocumentHittable: { creationHitQueries += 1; return true },
        editorHittable: { editorHitQueries += 1; return true })
}
let queryClock = StartupTestClock()
var queryBudget = CherriEditorStartupDeadline(now: { queryClock.value })
let missingControls = matched(0, 0)
check(creationHitQueries == 0 && editorHitQueries == 0, "zero matches never resolve missing controls")
check(!queryBudget.claimDocumentCreation(missingControls), "zero results cannot authorize a tap")
check(!queryBudget.accepts(missingControls, waitCompleted: true), "zero results cannot authorize readiness")
check(queryBudget.needsDocumentWait(missingControls), "no known control requires a real wait")
let soleCreation = matched(1, 0)
check(creationHitQueries == 1 && editorHitQueries == 0, "ready creation never resolves absent editor")
check(!queryBudget.needsDocumentWait(soleCreation), "current ready control does not require another observation wait")
check(queryBudget.claimDocumentCreation(soleCreation), "unique ready creation can be tapped once")
check(!queryBudget.claimDocumentCreation(soleCreation), "same current observation cannot tap twice")
let soleEditor = matched(0, 1)
check(creationHitQueries == 1 && editorHitQueries == 1, "ready editor never resolves absent creation")
check(queryBudget.accepts(soleEditor, waitCompleted: true), "unique editor plus completed wait is valid")
check(!queryBudget.accepts(soleEditor, waitCompleted: false), "query result never substitutes for completed wait")
let coPresent = matched(1, 1)
var coPresentBudget = CherriEditorStartupDeadline(now: { queryClock.value })
check(!coPresentBudget.claimDocumentCreation(coPresent), "same filtered result sees editor and forbids second document")
check(!coPresentBudget.documentCreationIssued, "co-presence leaves creation unissued")
check(coPresentBudget.accepts(coPresent, waitCompleted: true), "co-present unique editor may complete readiness")
for (creates, editors, unexpected) in [(2, 0, 0), (0, 2, 0), (2, 1, 0), (1, 2, 0), (1, 0, 1), (0, 1, 1), (-1, 0, 0), (0, -1, 0)] {
    let before = creationHitQueries + editorHitQueries
    let ambiguous = matched(creates, editors, unexpected: unexpected)
    var ambiguousBudget = CherriEditorStartupDeadline(now: { queryClock.value })
    check(creationHitQueries + editorHitQueries == before, "ambiguous query must not resolve or hit-test a candidate")
    check(!ambiguousBudget.claimDocumentCreation(ambiguous), "ambiguous candidate never authorizes creation")
    check(!ambiguousBudget.accepts(ambiguous, waitCompleted: true), "ambiguous candidate never authorizes readiness")
}
for (window, foreground) in [(false, true), (true, false)] {
    let before = creationHitQueries + editorHitQueries
    let unavailable = matched(1, 1, window: window, foreground: foreground)
    check(creationHitQueries + editorHitQueries == before, "unavailable app does not query candidate hittability")
    check(!coPresentBudget.accepts(unavailable, waitCompleted: true), "current window and foreground remain mandatory")
}
queryClock.value = 28.001
var lateCreationBudget = CherriEditorStartupDeadline(now: { queryClock.value })
queryClock.value += 28.001
check(!lateCreationBudget.claimDocumentCreation(soleCreation), "query completing after deadline cannot authorize tap")
check(!coPresentBudget.accepts(soleEditor, waitCompleted: true), "late resolved editor cannot authorize readiness")

// Consume the actual successful predicate once, without querying the full AX
// tree again. D03's original wait returned at 3.750s; its redundant known-control
// query then took 20.684s. This reproduces the orchestration decision locally,
// not the simulator timing or a repaired feature result.
let observedClock = StartupTestClock()
var observedBudget = CherriEditorStartupDeadline(now: { observedClock.value })
let documentWait = CherriEditorStartupWaitObservation<String>(phase: .document)
var fullQueries = 0
fullQueries += 1
observedClock.value = 3.750
check(documentWait.record("owned-create-element", state: creationState), "actual document predicate observes an actionable control")
let completedDocument = documentWait.takeCompleted(waitCompleted: true)
check(completedDocument == "owned-create-element", "completed wait retains the same element identity")
check(fullQueries == 1, "taking the completed observation performs no second AX query")
check(observedBudget.claimDocumentCreation(creationState.recheckingForeground(true)), "completed observation authorizes one timely owned tap")
check(!observedBudget.claimDocumentCreation(creationState), "completed observation cannot issue a second document")
check(documentWait.takeCompleted(waitCompleted: true) == nil, "completed observation is consumed once")
check(!documentWait.record("late-element", state: creationState), "closed wait rejects a late predicate callback")

let staleWait = CherriEditorStartupWaitObservation<String>(phase: .editor)
check(staleWait.record("ready", state: readyState), "first ready predicate is captured")
check(!staleWait.record("lost-editor", state: states[2]), "later failed predicate invalidates readiness")
check(staleWait.takeCompleted(waitCompleted: true) == nil, "stale earlier success cannot be consumed")
let timedOutWait = CherriEditorStartupWaitObservation<String>(phase: .editor)
check(timedOutWait.record("ready", state: readyState), "timed-out control has a positive observation")
check(timedOutWait.takeCompleted(waitCompleted: false) == nil, "positive observation cannot substitute for actual completed wait")
check(timedOutWait.takeCompleted(waitCompleted: true) == nil, "timeout cannot later be relabeled completed")
let emptyWait = CherriEditorStartupWaitObservation<String>(phase: .editor)
check(emptyWait.takeCompleted(waitCompleted: true) == nil, "completed result with no predicate observation fails closed")
let anotherWait = CherriEditorStartupWaitObservation<String>(phase: .editor)
check(anotherWait.takeCompleted(waitCompleted: true) == nil, "one phase cannot borrow another phase's observed state")

let editorWait = CherriEditorStartupWaitObservation<CherriEditorStartupObservation>(phase: .editor)
observedClock.value = 20
check(editorWait.record(readyState, state: readyState), "full interactive-editor predicate is mandatory")
let completedEditor = editorWait.takeCompleted(waitCompleted: true)!
check(observedBudget.accepts(completedEditor.recheckingForeground(true), waitCompleted: true), "completed predicate and fresh foreground may pass before original deadline")
check(!observedBudget.accepts(completedEditor.recheckingForeground(false), waitCompleted: true), "foreground loss after predicate rejects success")
check(!states[1].recheckingForeground(true).interactiveEditor, "fresh foreground never repairs an earlier unavailable observation")
for (index, state) in states.enumerated() {
    check(!state.recheckingForeground(true).interactiveEditor, "foreground refresh cannot repair missing readiness field \(index)")
}
observedClock.value = 28.001
check(!observedBudget.accepts(completedEditor.recheckingForeground(true), waitCompleted: true), "decision after deadline rejects an earlier timely observation")
check(!observedBudget.claimDocumentCreation(creationState.recheckingForeground(true)), "foreground check finishing late cannot authorize a tap")

let coPresentWait = CherriEditorStartupWaitObservation<CherriEditorStartupObservation>(phase: .document)
var noSecondDocument = CherriEditorStartupDeadline()
check(coPresentWait.record(coPresent, state: coPresent), "editor arriving during document wait is retained")
let coPresentCompleted = coPresentWait.takeCompleted(waitCompleted: true)!
check(!noSecondDocument.claimDocumentCreation(coPresentCompleted), "co-present editor and button still forbid a document tap")
for (creates, editors, unexpected) in [(0, 0, 0), (2, 0, 0), (0, 2, 0), (1, 0, 1), (0, 1, 1)] {
    let rejected = matched(creates, editors, unexpected: unexpected)
    let waiting = CherriEditorStartupWaitObservation<CherriEditorStartupObservation>(phase: .editor)
    check(!waiting.record(rejected, state: rejected), "empty or ambiguous matched controls cannot satisfy editor predicate")
    check(waiting.takeCompleted(waitCompleted: true) == nil, "empty or ambiguous result cannot be borrowed at completion")
    var tapBudget = CherriEditorStartupDeadline()
    check(!tapBudget.claimDocumentCreation(rejected), "empty or ambiguous result cannot authorize a tap")
}

// The production query resolver now uses only the editor after the owned tap.
// Query counts prove the eliminated work; these controlled timings are not iOS evidence.
var phaseQueries: [String] = []
let fullControls = CherriEditorStartupControlScope.documentAndEditor.resolve(
    creation: { phaseQueries.append("creation"); return ["owned-create"] },
    editor: { phaseQueries.append("editor"); return ["owned-editor"] })
check(phaseQueries == ["creation", "editor"], "pre-tap queries preserve full scope and editor-last order")
check(fullControls.creation == ["owned-create"] && fullControls.editors == ["owned-editor"],
      "pre-tap resolver retains exact control identities")
phaseQueries = []
let editorControls = CherriEditorStartupControlScope.editorOnly.resolve(
    creation: { phaseQueries.append("creation"); return ["unexpected-create"] },
    editor: { phaseQueries.append("editor"); return ["owned-editor"] })
check(phaseQueries == ["editor"], "post-tap editor scope never executes the creation query")
check(editorControls.creation.isEmpty && editorControls.editors == ["owned-editor"],
      "editor scope retains only the current editor identity")
var scopedHitQueries = 0
func editorScoped(_ count: Int, window: Bool = true, foreground: Bool = true) -> CherriEditorStartupObservation {
    .matchedEditor(windowExists: window, appRunningForeground: foreground, editorCount: count,
                   editorHittable: { scopedHitQueries += 1; return true })
}
let scopedReady = editorScoped(1)
check(scopedReady.interactiveEditor && scopedHitQueries == 1, "unique editor receives its actual hit test")
check(scopedReady.controlScope == .editorOnly && !scopedReady.createDocumentExists &&
      !scopedReady.createDocumentHittable, "editor-only scope explicitly records unobserved creation controls")
check(scopedReady.recheckingForeground(true).controlScope == .editorOnly, "foreground refresh preserves query scope")
for count in [0, 2, -1] {
    let before = scopedHitQueries
    let unavailable = editorScoped(count)
    check(!unavailable.interactiveEditor && scopedHitQueries == before,
          "missing or ambiguous editor cannot be hit-tested or accepted")
}
for (window, foreground) in [(false, true), (true, false)] {
    let before = scopedHitQueries
    let unavailable = editorScoped(1, window: window, foreground: foreground)
    check(!unavailable.interactiveEditor && scopedHitQueries == before,
          "editor scope retains fresh window and foreground requirements")
}
let scopeClock = StartupTestClock()
var scopeBudget = CherriEditorStartupDeadline(now: { scopeClock.value })
let forgedCreationInEditorScope = CherriEditorStartupObservation(
    windowExists: true, appRunningForeground: true, createDocumentExists: true,
    createDocumentHittable: true, editorExists: false, editorHittable: false, controlScope: .editorOnly)
check(!scopeBudget.claimDocumentCreation(forgedCreationInEditorScope),
      "an editor-only snapshot cannot authorize document creation")
check(!scopeBudget.needsDocumentWait(forgedCreationInEditorScope),
      "an editor-only snapshot cannot start a document wait")
let scopedDocumentWait = CherriEditorStartupWaitObservation<String>(phase: .document)
check(!scopedDocumentWait.record("borrowed", state: scopedReady),
      "document wait cannot borrow editor-only scope")
check(scopedDocumentWait.takeCompleted(waitCompleted: true) == nil,
      "completed document result without full scope cannot be consumed")
let scopedEditorWait = CherriEditorStartupWaitObservation<CherriEditorStartupObservation>(phase: .editor)
check(scopedEditorWait.record(scopedReady, state: scopedReady), "editor predicate accepts its full required observation")
let scopedCompleted = scopedEditorWait.takeCompleted(waitCompleted: true)!
scopeClock.value = 27.999
check(scopeBudget.accepts(scopedCompleted.recheckingForeground(true), waitCompleted: true),
      "scoped completed editor is accepted within the original 28 seconds")
check(!scopeBudget.accepts(scopedCompleted.recheckingForeground(false), waitCompleted: true),
      "foreground lost after the scoped predicate still rejects readiness")
check(!scopeBudget.accepts(scopedCompleted, waitCompleted: false),
      "scoped positive observation cannot replace a completed XCTest wait")
scopeClock.value = 28.001
check(!scopeBudget.accepts(scopedCompleted, waitCompleted: true),
      "late scoped editor still fails the unchanged 28 second deadline")
check(scopedEditorWait.takeCompleted(waitCompleted: true) == nil,
      "scoped observation still permits only one consumption")

print("PASS_LOCAL_STARTUP_DEADLINE_CONTROLS: \(checked); Apple UI NOT_RUN")
