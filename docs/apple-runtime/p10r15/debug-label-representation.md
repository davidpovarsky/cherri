# Typed debugLog label representation probes

Asana child 1220544718902445 remains OPEN. No label option has been registered or advertised. The existing raw typed log, six display modes and exact producer identities remain unchanged.

## Independent native contract boundary

The checked-in Apple metadata describes Add to Variable as accepting multiple WFContentItem values. List includes the contents of variable inputs. A one-label-per-call positional sidecar therefore has no established one-to-one relationship with the flattened raw log.

Set Name accepts one WFContentItem (Multiple=false), returns Renamed Item and says that it sets the input item's name. Its InputTypeDeterminesOutputType flag describes type behavior; it does not guarantee an independent copy or preservation of a whole multi-item list.

Set Dictionary Value uses WFTextInputParameter for WFDictionaryValue. Get Dictionary Value lists String/Number/Date/Dictionary/Boolean output classes and does not declare File/Image. Both compiled Dictionary payload probes use WFTextTokenString with a native ActionOutput attachment for that value. This is not proof of a universal typed object carrier.

The existing user-authored dated-preview shape provides evidence for Set Name's parameter encoding. It does not independently prove alias isolation for arbitrary native objects. Changing CustomOutputName would label an editor output, not attach a stable runtime label to every collected entry.

## Executable probes

Each row has an explicit V2 and Legacy source under tools/shortcuts-simulator-qa/fixtures/p15-label-probes. All data is synthetic and in memory.

| Candidate | Intended native assertion | V2 / Legacy actions |
|---|---|---:|
| dictionary-carrier | Label including Hebrew/emoji/quotes survives; Dictionary type, nested list, false flag type/value and original fields survive a Set Dictionary Value/Get Dictionary Value carrier | 59 / 58 |
| file-carrier | Same native type and exact AQIDBA== bytes after carrier; original bytes unchanged; label unchanged | 47 / 46 |
| setname-alias | Renamed result keeps native type/bytes; original name and bytes are unchanged | 46 / 45 |
| raw-grouping | Logging two items then one scalar yields raw counts two then three | 29 / 35 |
| raw-empty | Logging an empty List yields zero entries without a native missing-input failure | 22 / 24 |

The fixture's OK marker is emitted only after its explicit conditions. A failed type/value assertion changes the marker; a native exception or missing callback is not a pass. Before/after trace, exact fixture ID and existing anchor conventions are present. Three V2 probes now have explicit signed-QA selections: qa-label-file-carrier, qa-label-raw-empty and qa-label-raw-grouping. Each binds its reviewed nested source hash, exact expected callback and fresh nonce, and requires normal NDJSON trace. The remaining probes stay outside the runtime allowlist. Registration does not establish native execution; copying a probe into an unrelated known target would invalidate provenance.

Suggested first selected target: qa-label-file-carrier-v2. Expected success marker: P15_LABEL_FILE_CARRIER_OK, in the CHERRI_QA_LABEL_PROBE_RESULT envelope. That result alone would not establish Image/Date/mixed-list behavior or completion of the full label feature.

## What was actually run

Sentinel job job_0c3258d3ad5b passed in 5.26 s on 2026-11-18. The producer built the compiler itself from source base 0102da8ad779f9f5d5979ffa081475f1c095d52e and recorded the actual worktree/source/fixture fingerprint, compiler hash and native schema/macro fingerprints.

All ten real CLI compiles succeeded. All ten unsigned outputs imported through the existing V2 lossless path and recompiled with exact WFWorkflowActions equality. Every ActionOutput binding points to an exact preceding producer. The producer checks that no download, file-save, stored-content, Run Shortcut/Run Self or share action appears.

Sentinel job job_5e7d186de5d1 passed five existing focused tests with 24 total pass events, including all twelve Legacy/V2 six-mode cases, preservation of original producer UUIDs/downstream consumers, and signed-fixture reference graph checks. These are compiler checks; no new signed or physical runtime was executed.

Initial fixture probes were correctly rejected for a literal dictionary passed where V2 requires an attachment, and Legacy array constants. The corrected sources bind the dictionary first and use ordinary Legacy array variables plus Get Variable. No compiler relaxation, new native shape, or coercion workaround was added.

## Reproduction

    python3 tools/p10r15/verify_label_representation_probes.py

The command builds from the current checkout, creates unsigned files in dist/p15-label-probes and writes docs/apple-runtime/p10r15/debug-label-probes.json. It binds all compiler source, native metadata and exact probe source bytes before/after execution. It returns success only for compile/graph/lossless readiness; the report always records nativeExecuted=0 and OPEN_REPRESENTATION_UNPROVEN.

## Next implementation boundary

A plausible safe design keeps the raw object list untouched and records only primitive label metadata indexed by the actual raw log range before/after each append. It must establish Count/Repeat cardinality for empty/nested values, initialization and clearing, skipped branches, late labels, repeated calls and no overhead when no label is used. A separate label per source call without that association is unsafe. Typed preview must present labels without assuming Set Name copies an object or coercing the original payload to Text.

Until these representation questions have native answers, adding an apparently supported label parameter would overstate the contract. This is a specific representation prerequisite plus remaining source implementation, not a claim that all label engineering is blocked by unavailable hardware.


## Unicode source correction

An independent byte read after commit 92b7533 found that the PowerShell stdin
encoding had replaced Hebrew and emoji in six probe files with question marks.
The preceding compile and exact-roundtrip checks were valid for those stored
bytes, but did not establish the intended Unicode scope. This is a retained
evidence defect, not a native runtime failure.

The six sources now contain actual UTF-8 Hebrew and U+1F352 in both setup and
assertion. The producer checks those exact code points and UTF-8 bytes before
building, records their SHA-256 values, and rejects question-mark substitutions.
The explicit mutation audit must fail before compilation, followed by restored
10-probe structural verification. Native execution remains zero.
