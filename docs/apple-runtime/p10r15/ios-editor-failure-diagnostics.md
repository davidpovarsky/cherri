# iOS editor failure diagnostics after R1

The actual R1 `iOS Build` run 44223787270 built the app and CherriCore framework.
Its log records 24 passing `CherriCoreIntegrationTests`, including
`testBuildProfilesPreserveSourceThroughCABI`, compilation, contextual completion,
Unicode, scoped rename, and bounded visual edits. The editor UI suite ran nine
tests and failed six. E02 diagnostics and the two negative UI tests passed.

| Failed UI case | Observed failed assertion |
|---|---|
| BRG27 workflow | Compiled status absent |
| E01 completion | Source did not yet contain the selected action |
| E03 affordances | Initial fixture did not reach compiled status |
| E05 rapid edits | Compiled status absent after typing |
| E06 Hebrew/emoji | Initial typed fixture did not reach compiled status |
| E07 preview | Initial fixture did not reach compiled status |

These are actual failures. The passing service layer does not substitute for
UI acceptance. No new Swift build or editor run is claimed by this diagnostic
patch; its next Apple run is required.

## Confirmed evidence-loss and synchronization defects

The old workflow exited on an unsuccessful `xcodebuild test` before generating
`ios-test-evidence.json`. Its upload step also lacked `always()`. As a result,
the R1 run retained the compiled app but no test-evidence artifact. The available
GitHub job log contains failure assertions and a bounded tail, not the source
value that the editor actually held at the failed compile.

E01 also read the source immediately after selecting an action. The app's
existing palette insertion intentionally waits 300 milliseconds for dismissal;
waiting for the editor to exist does not wait for that mutation. The test now
waits on the actual inserted-text condition within its existing five-second
budget, then applies both original content assertions.

## Diagnostic changes

Every fixture entry now requires the real Clear button and an observed empty
editor value before typing. Each typed source is checked against the expected
UTF-16 sequence. The rapid-typing case keeps its real keyboard burst and checks
the complete source. A small additional case distinguishes clearing a longer
document, entering a shorter declaration, and entering `text("x")` through the
keyboard. It deliberately does not paste a whole source string.

Bounded receipts record known control identifiers, booleans, and the actual
source only after that empty reset was verified. If reset is unavailable or
unverified, source is omitted. No app-wide AX tree, document list, raw diagnostic
text, or screenshot is exported. Existing assertions and compile checks remain.
Native selection ranges are not exposed by this receipt; they remain unknown.

The workflow preserves the original test exit code, runs the existing evidence
mapping in an `always()` collector, and uploads only three bounded text files:
`xcodetest.log`, `ios-test-evidence.json`, and `ios-test-diagnostics.json` from a
separate staging directory. The staged log keeps test boundaries, known
checkpoints, sanitized receipts, and failure test/line identifiers. Raw Xcode
logs and screenshots remain outside the upload set.

A failed process still records individual observed passed/failed outcomes for
diagnosis. It produces no acceptance records. Missing process status, missing
source identity, duplicate execution, incomplete cases, and checkpoints outside
their own test body cannot create a success record. Existing evidence output is
never overwritten.

## Remaining source-level hypothesis

The pinned `CodeEditorView` revision is
`5386056ab53d43363083cb96069715a9608aa048`. Its primary source already disables
smart quotes and autocorrection. Its token-completion code inserts a matching
closing bracket after the next typed character. The macOS selection delegate
accounts for `tokenCompletionCharacters`; the iOS delegate does not have that
same correction. Its programmatic text update also restores the selection from
the position model. These observations identify places to investigate; they do
not establish which state occurred in R1.

The exact typed-source receipts and the minimal keyboard probe must establish
whether the failure is a duplicated/misplaced closing delimiter, a reset/selection
problem, or a different state. This patch does not change the editor dependency,
replace the editor, disable typing features, or declare the five compile failures
fixed. Upstream source was read at the pinned commit through the repository API.

## Local verification

The new collector's 12 Python tests execute its real code and CLI. They cover
failed-run retention, exact test boundaries, duplicate and incomplete cases,
raw-log exclusion, unverified-reset source omission, secret canaries, Unicode,
output bounds, preservation of prior evidence, and the actual workflow's saved
nonzero exit status. YAML parsing and shell syntax checks cover every workflow
run step. Swift and actual UI results remain pending the diagnostic CI run.

Rollback is a revert of this diagnostic commit. It changes no shared compiler,
signing service, physical device, or runtime-Shortcut harness.
