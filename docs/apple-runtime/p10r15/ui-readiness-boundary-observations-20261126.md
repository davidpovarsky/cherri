# Closed observations for three unresolved iOS UI boundaries

## Scope

This component adds diagnostics for the three failed boundaries observed in the
original full iOS run [45780580372](https://github.com/davidpovarsky/cherri/actions/runs/45780580372),
whose source was `1cafc8cea0950edc182b1a0656c7db11a975df47`.
It does not repair an established application data defect, change a readiness
condition, or grant native acceptance.

The original public collector retained 78 named outcomes (75 passed, 3 failed).
The authenticated encrypted xcresult summary contains the aggregate of 89 tests
(86 passed, 3 failed, zero skipped). The eleven missing named rows belong to
`CherriPreviewEditTransactionTests`; the separate 79b70c retention change fixes
future collection. Those eleven historical rows have not been reconstructed.

## Observed boundaries and retained semantics

| Boundary | Actual original observation | Diagnostic addition |
| --- | --- | --- |
| E07 startup | The editor was absent at the original readiness assertion, before the first pane transition or source reset. The preserved trace completed 22 polls after Create Document without observing an editor. | Add only the exact E07 identity to the existing nonce-owned test, app-observer, and capture allowlists. Original readiness remains 28 seconds; observer bounds remain 96 events, 65,536 bytes, and 120 seconds. |
| BRG25 Sign | Build rejection and the malformed-build stage were observed; the next assertion could not establish that Sign existed, was hittable, and was enabled. | Record only the Boolean values already read by that exact five-second short-circuit predicate. Unread properties remain null. |
| BRG28 initial Text lookup | The actual-source-built stage was observed. The first Text/kept-Text lookup failed before editing. | Record the existing first-WebView presence, candidate count, exact-value matching count, and already-read Preview pane properties. The original eight-second predicate still requires exactly one match. |

The new trace performs no accessibility queries, waits, taps, source extraction,
or readiness decisions. Sign keeps the original exists → hittable → enabled
short circuit. WebKit keeps the same candidate query, UTF-16 value comparison,
fresh post-wait lookup, field editing, and source assertions. Pane observations
explicitly state that selected state was not queried; a tap is not presented as
proof of selection. The fixed query name identifies the authored query, not a
claim that a unique accessible control has been observed.

The collector admits only four fixed boundary identities under their exact
active test contexts. It requires an exact key set, fixed query and budget,
typed bounded counts/timing, consistent nullable short-circuit values, and
consistent pane observations. Elapsed values are capped at 120,000 ms and
counts at their declared limits; the counts expose explicit clamping flags.
Duplicate JSON keys, duplicate boundaries, foreign contexts, unobserved
selection claims, extra source/AX fields, oversized raw UTF-8 rows (including
whitespace padding), and impossible states are rejected. At most eight rows
including rejections are retained. No row contributes an acceptance record or
changes an original passed/failed terminal.

## Actual Python qualification

All paths below are under
`/var/cache/cherri-e07-pane-transition-20261126`.

| Receipt | Actual result | SHA-256 |
| --- | --- | --- |
| `ui-readiness-proposal-01/result.original.json` | Frozen source proposal from clean 79b70c; not an execution | `18407c6985b8f89103651958b9ddbf2429c14fd36f0ded93d0e34826c1418fd4` |
| `ui-readiness-red-01/result.original.json` | Nine new methods failed on the unchanged collector/startup allowlist: 48 failing subcases, no setup errors | `30b1a84a1ea780c9ac145ab8b6e3f6fcf680e9ab6d41acf7a952e336ea124660` |
| `ui-readiness-green-01/result.original.json` | 30 methods passed: eight new collector methods, two unchanged collector controls, and all 20 startup-capture methods including the new E07 control; 0.474 seconds | `21e922c62d5e27f2102fcc717aa7b4e333935e4c285398d6260385d3ec55ec0b` |
| `ui-readiness-preservation-01/result.original.json` | Exact reversal of each narrow source edit recovers the original bytes; all original test-method inventories and existing AX-read/tap/deadline token counts retained; all collector non-logging function ASTs unchanged | `5d7b4700e3ce883c8193b99e9f62df91d18ebf9361420ce788f3ade792cf379a` |
| `ui-readiness-entrypoint-correction-01/result.original.json` | The unchanged main guard moved to the physical end. Direct-entry and original workflow discovery both enumerate 87 methods, including all eight new methods; no test body executed or changed | `ce989c12dddebc770a854d5d9c6266941eb5944a44fbde3c8cec7fe63844d0c8` |

The GREEN owner ran one bounded Python command under UID 1001, retained the
original stdout/stderr and before/after source pins, reaped its parent, proved
its process group absent, and removed only its empty temporary directory.
Original RED sources and streams remain unchanged. The new controls use
synthetic public rows and test rejection of source/AX canaries; they do not
supply missing native observations.

The protected E07 source change is one extra allowlisted identity. Every
existing E07 test body, fixture, assertion, three-second pane transition, and
downstream deadline is byte-preserved. The R23 successor review must pin the
actual implementation commit while retaining both immutable baseline snapshots
and both protected paths. The R23 verifier itself is unchanged.

## Remaining validation

Swift compilation and actual execution of these diagnostic hooks have not run
in this component. The existing Swift observer verifier gains one E07-context
admission assertion and will run through the original workflow. The next
integrated full iOS workflow must compile this source and retain actual
observations for all original UI tests, with no skips, selector changes, timeout
extensions, or isolated native rerun solely for collection. Native E07, BRG25,
and BRG28 remain unresolved until that real evidence identifies and qualifies a
necessary repair.
