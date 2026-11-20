# iOS startup and paste observations — component verification

Implementation: 8d2c30e65b050d34de36bbf374f37e3183d3aea0.

[The source-bound local receipt](ios-startup-paste-controls-20261120.json)
records the actual clean replay and exact hashes. This component has not run
UIKit or the Apple Simulator. It cannot close the iOS or documentation gates.

## Observed cause and remaining uncertainty

The original R2 Intel run 44634890917 retained 42 passed and 11 failed test
terminals before the 35-minute job deadline. Ten UI failures occurred at the
shared initial editor wait. Several fixed-state receipts observed the editor
after that earlier 10-second wait had already returned false. The single-
character paste failure was the 3-second asynchronous expectation, not a
recorded native type or generated-bracket value observation. Those failed
results and original artifact bytes remain unchanged.

## Startup policy

The existing staged wait allowance is shared through one monotonic 28-second
deadline: window 10 + early editor 2 + document 6 + final editor 10. Unused earlier
time is available to the document-to-editor transition. The current foreground,
window, editor existence and hittability must all hold, the readiness wait must
complete, and the observation must be within the deadline. A late observation
or a timed-out wait cannot be converted into success.

Document creation is allowed only once, using the fresh state after its wait.
If the editor appeared meanwhile, even while Create Document remains visible,
no creation tap is issued. Independent review found this race in the first
draft; the committed source and causal control include the correction.

The 28-second value is an acceptance/wait budget. An individual AX call can
block beyond its requested duration; such an overrun fails closed and cannot
be counted as readiness. The separate full-job 75-minute limit remains finite.

Fixed startup receipts reuse observed control state rather than issuing fresh
AX reads solely for reporting. Timing fields are bounded and cannot authorize
restored source, document labels, screenshots or arbitrary accessibility trees.

## Paste observation

The original UIKit paste action, 3-second expectation and exact final text
assertion remain. A verified write of the isolated single-character pasteboard
input is required before paste. Three bounded receipts record the actual
source/selection, first-responder/window state and elapsed time before paste,
after the call, and after the existing wait. Only the fixed synthetic source
variants are publishable; unexpected content is redacted.

No production bracket or clipboard implementation changes are claimed. The
next exact-source native run must distinguish a delayed paste, observer timing,
responder problem or incorrect resulting text from those actual observations.

## Actual local checks

A clean source replay passed all 27 Python collector tests, including failure
preservation, timing/type mutations, private-source rejection and actual CLI
collection. The real Swift helper compiled and passed 23 controls, including
time expiry, late readiness, lost foreground, one creation request and an editor
appearing during the creation wait. Swift syntax parsing also passed.

The Windows Swift interpreter's JIT lacked required runtime symbols on an
earlier precommit attempt. Compiled Swift execution succeeded; CI therefore
compiles and executes the same controls explicitly. The original failed JIT
log is retained. None of these local checks constitutes Apple runtime proof.
