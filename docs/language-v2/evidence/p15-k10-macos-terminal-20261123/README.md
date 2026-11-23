# K10 macOS terminal failure and required encoding component

## Status and identities

This packet preserves original K10 evidence and the separately reviewed K04
scheduler extension. It does not replace a failed workflow with component
success, relabel K10 evidence as K11, or claim fresh runtime verification.

- Observed K10 source: `4feadab9db4d351628f78593304373631693869a`.
- Actual Build & Test run: [45100706965](https://github.com/davidpovarsky/cherri/actions/runs/45100706965), attempt 1, **failure**.
- Actual macOS job: `137450662143`, completed `2026-11-23T03:23:28Z`.
- Reviewed scheduling base: `b3eb60c23e69c0c5700d8cd00487c299f2d13b32`.
- Scheduling implementation: `2ac6e23aab3d69fe445ef018864d734d37183274`.
- Root integrated the same patch as `171faf12580f6e5f1f918ca6cea77234673f098d`.
- Execution mode: real platform `[SUBAGENT]` `/root/ci_source_audit`;
  independent static reviewer `/root/asana_inventory`, integration reviewer
  `/root`. No external coding/model CLI or provider generation was used.

The manifest binds every packet member. Original large job logs remain outside
Git, with exact paths, lengths and hashes in `originals-index.json`. Byte copies
and original excerpts are identified separately from derived intake reports.
The source patch is not a production compiler change.

## Actual macOS outcome

| Surface | Original observation |
| --- | --- |
| Ordinary packages | 23 PASS, 1 package timeout FAIL, 5 without test files |
| Ordinary top-level tests | 1,423 PASS, 10 SKIP, 1 interrupted parent |
| Acceptance top-level tests | 240 PASS, 3 SKIP, 3 FAIL |
| Ordinary invocation | Exit 1, monotonic 911.673 seconds, original 15-minute package limit |
| Acceptance invocation | Exit 1, monotonic 1,658.569 seconds, original 35-minute package limit |
| Source before/after suite | Exact K10, clean |
| AI standalone controls | 154 PASS, reported 5.646 seconds |
| RP50 original parent | PASS, 4.14 seconds, exact 15 Python controls found and passed |
| Native encoding process controls | 12 PASS, reported 0.840 seconds |

The three acceptance failures are the installed BRG32 package-inventory defect
and its two propagated failures:
`TestP15BRG32InstalledAIHarness`,
`TestSupplementalObligationHarnessProductionEvidence`, and
`TestSupplementalAcceptanceProductionEvidence`.
The separate four-line repair was committed as
`d5b82e3a4e21416556d4a6c81ed12b05b1365b2f`; it is not a successful rerun of K10.

The ordinary root package hit its original cumulative 900-second alarm. Its
only remaining active parent was `TestP15CanonicalBehaviorSourcePropagation`
(52 seconds), with the `codec` child only 2 seconds old. The exact original
stack shows `p15SchemaCommand` waiting for its child through `CombinedOutput`.
The original baseline, static mutation/restoration and callback
mutation/restoration already produced their actual hashes. The interrupted
work was the codec mutation rebuild, followed by its original restoration and
final clean-tree assertions. No individual child timeout or blocked-cleanup
event is established by the retained stack. The parent is not credited PASS.

The original full-suite report establishes that all 30 packages were invoked
once in the two original groups. Its `all_packages_executed_once` field does
not establish that every test completed: the root timeout interrupted one
parent. Go-emitted parent times, monotonic invocation durations and GitHub
step wall times are labeled separately. Buffered GitHub log timestamps are
not used to infer per-test execution duration.

## Genuine required-encoding component, failed-run ineligibility

The original `required-encoding-gate` artifact is `14785316588`: 13,063 bytes,
SHA-256 `11c062e73f1d75ecba80b52b4627af1500857e62861e3ba45daf3958bba94c5d`.
All 15 ZIP members passed CRC/name/path checks, and their original hashes are
retained. Independent mechanical intake reconciled all six input hashes,
including the exact pinned ShortcutKit Git blob, and all 26 evidence-artifact
references.

| Actual WorkflowKit phase | Exit | Required observations |
| --- | ---: | --- |
| Positive | 0 | 23/23 PASS |
| Isolated corrupted EXPECT | 1 | Exactly the intended target fails; 22 other observations pass |
| Restored | 0 | 23/23 PASS; report bytes equal the positive report |

Each phase retains 50 state classes across all seven forms: 350 per-form
observations. The single mutated target is
`plainString.WFStringParameterState.string`, whose EXPECT changes from accept
to reject while the native observation remains accept. The exact original
nonzero diagnostic is retained. The canonical input set also contains its
original expected rejection.

This is real Darwin ARM64 WorkflowKit parameter-state API execution. It is
not Shortcut runtime execution. Original CI01 and RP47 records name only
the `negative-ci` tier; no BRG31 encoding record exists.

The unchanged canonical aggregator rejects non-success manifest runs and
artifacts referencing those runs (`tools/language-acceptance/main.go`).
Consequently, the successful component inside this failed Build & Test run
is **formally ineligible** for requirement closure. Independent mechanical
reconciliation is not a newly executed canonical Go admission. The complete
K11 gate must provide its own same-source evidence; no K10 record is restamped.

The existing upload glob omits assembled `verify.js`. Its committed assembly
recipe and the two exact constituent source blobs were independently checked.
The packet preserves the original pinned-source API response, not an invented
uploaded script.

## Two-hook scheduling extension and validation boundary

The reviewed patch adds `p15FullSuiteSubprocessParallel(t)` as the first
statement of exactly two existing parents in `p15_skill_acceptance_test.go`:

1. `TestP15K04OfflineCacheProduction`.
2. `TestP15K04OfflineOracleRejectsBypasses`.

Removing these two statements reproduces the entire original file byte for
byte. No other test-body statement, fixture, assertion, inventory, child
command, scheduling helper, suite runner, 15-minute ordinary limit, 35-minute
acceptance limit, or original child deadline changed. The three immutable
contracts and all seven AI producer files remain identical to the reviewed
base.

The original K10 two-slot schedule resumed all 27 opted-in parents and
completed 26 before the alarm. The K04 bodies remained serial and consumed
31.67 plus 90.30 seconds. Their combined 121.97 seconds would ideally overlap
by approximately 60.985 seconds when moved into the existing two-slot queue,
if runtime costs remain comparable and scheduling balances. This is an
estimate; host speed, resume order and shared cache waiting can change it.
**A complete macOS run under 15 minutes has not been demonstrated for this
extension.**

Independent static review found owned temporary compiler/migrator files,
copied oracle scripts, per-invocation shell homes/docs/Skill/network shims,
serial mutation subtests, and no process-global environment or cwd mutation
in these dependencies. The shared immutable tool-byte cache is mutex
protected. The resulting 29 opted-in parents retain the same two-slot cap
inside the root test package; this is not a cap on every descendant OS
process. Existing direct-shell `CommandContext` and shell traps are unchanged;
no stronger descendant-group cleanup guarantee is claimed.

The existing helper deliberately rejects opt-in for a selected `-run`.
A focused K04 replay would therefore leave the new hooks inactive and
would not prove their parallel behavior. No selected-producer guard was
bypassed, and no costly body replay or local whole-suite replay was performed.
The packet includes the original K10 policy parent and its 13 passing cases
as historical evidence for unchanged helper behavior. The meaningful new
execution gate is the final complete macOS CI with the existing marker `2`,
empty `test.run`, and `-parallel=2`. Concurrency benefit remains unverified
until that actual result.

`proposal.original.json` and `candidate-review.original.json` predate final
review and still show their original unapplied/pending state. They are
preserved unchanged. The later implementation receipt and independent review
record describe the authorized two-line commit without overwriting history.

## Linux BOM-aware count supplement

The earlier original Linux intake remains preserved. Five original GitHub
log segments contain a leading UTF-8 BOM; the initial anchored counter missed
one actual PASS line in each invocation. Removing only the optional BOM and
GitHub timestamp for parsing produces the corrected counts:

- Ordinary: **1,426 PASS / 8 SKIP**, all 1,434 top-level parents accounted for.
- Acceptance: **240 PASS / 3 SKIP / 3 FAIL**, all 246 parents accounted for.

The failure identities, package outcomes, RP50 result and recorded durations
did not change. No test was rerun and no original log or receipt was rewritten.

## Remaining gate

The scheduling extension requires actual full macOS CI at the next frozen
source. Failed K10 Build/OpenMinis runs, absent formal encoding eligibility,
pending native/device tiers, original R23 failure and unmeasured AI quality
remain explicit. This packet grants no formal179, release or project closure.

## Original-byte retention diagnostics

Git whitespace diagnostics remain only in the exact original timeout excerpt
(seven timestamped blank lines) and the original unified patch (eight context
line diagnostics). These 15 diagnostics are retained to preserve original
bytes. The two-line source implementation itself passed git diff --check.
No original log line or patch context was reformatted.
