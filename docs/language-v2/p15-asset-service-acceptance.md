# S03/E08 embedded asset service and performance evidence

## Scope and original requirements

The unchanged S03 contract requires service and benchmark evidence that a large local embedded asset stays cached across unrelated edits. E08 requires real small/large/embedded-asset measurements, with hardware, input/version identity and latency percentiles. Neither contract specifies a numerical latency budget.

This implementation uses the shared rooted workspace reader. It does not add a second serializer: the canonical `embedFile` action still emits the original file bytes as base64 through `WFTextActionText`.

## Production behavior

V2 compilation resolves a local file against the source document that contains its call. An imported module retains its workspace-relative source location while sharing the entry document's authorization root. The editor uses the actual action selected by semantic binding, so a user function or custom action named `embedFile` does not authorize file access.

Service analysis and canonical compilation share a bounded asset cache. Each access still opens the authorized resource and checks file identity, size and modification time. A hit reuses the existing encoded bytes without another full read, UTF-8 classification, digest or base64 encoding. Missing/replaced resources invalidate a previous entry. Changed bytes are read and encoded again. A read that observes a file change fails instead of retaining an inconsistent snapshot.

The defaults are a 64 MiB file limit, 128 MiB of resident encoded asset bytes and 64 cached entries. Host callers can supply an explicit `assets.Cache` to the service and `CompileOptions`. The raw file byte slice is not retained alongside its encoded representation. Dependency inspection returns path/size/kind/digest metadata without the encoded file content.

An explicit `workspace/didChangeWatchedFiles` notification invalidates cached assets and republishes diagnostics for open documents. This covers same-size edits with preserved timestamps. Ordinary modification-time/size/file-identity changes are detected on analysis even without that notification. A client must deliver the notification to cover a change that deliberately preserves all inspected metadata; this work does not establish an iOS filesystem watcher.

## Public causal controls

The original source failed the new entry-relative compilation and missing/traversal editor-diagnostic controls. The baseline has two passing named controls and four failure events including the parent; the raw log and exact source/test identity are retained separately.

The repaired public scenario covers both text and binary files larger than 8 MiB, 25 unrelated edits, exact canonical output bytes, service-to-compiler reuse, changed-file invalidation, same-metadata notifications, deletion/recreation, recovery after invalid source, shadowed action names, imported source location and built-in module aliases. Independent cache tests exercise eviction, byte limits, empty files, workspace isolation, replacement by an unauthorized link and simultaneous readers. The LSP test sends the actual notification message.

The first module integration control also exposed two separate namespace defects. Uppercase local module aliases were repaired in the shared module path. Qualified built-in schemas must preserve their stable canonical action ID while qualifying the callable name. The unqualified healthy control and both failed alias controls are retained; no adapter fallback by Apple identifier or spelling was added.

## Measurements

The benchmark executes 20 individual samples for each declared phase:

| Fixture | Actual input |
| --- | --- |
| Small source | 64 bindings, 1,068 bytes |
| Medium source | 1,024 bindings, 19,284 bytes |
| Large source | 8,192 bindings, 169,812 bytes |
| Inline text | 4 MiB of raw text plus its source wrapper |
| Local text | An 8 MiB file and a real embedFile call |
| Local binary | An 8 MiB file and a real embedFile call |

All six fixtures measure a new service/cache, warm analysis and an unrelated source edit. The two local-file fixtures also measure an actual asset edit. That is 20 distinct phases and 400 raw samples. File writes, input hashes and fixture preparation occur outside the measured editor operation. The recorded cold scope is the Cherri service/cache; the OS filesystem cache is not flushed.

The report records the exact source SHA and whether the tree was clean, Go/OS/architecture, CPU model or available processor identity, logical CPU count, GOMAXPROCS, physical memory, every input/asset version digest, raw counter deltas, clock frequency and nearest-rank p50/p95. Windows uses QueryPerformanceCounter. The initial interrupt-clock measurement yielded zero for the smallest warm operations; those rejected measurements are retained and are not converted into artificial nonzero timings.

The benchmark verifies that local-file warm/unrelated phases cause zero heavy reads or encodings, while cold/asset-edit phases perform all 20 actual reads and encodings. Reported latency is an observation on the recorded host. No UI responsiveness, Apple runtime latency or user-experience threshold is inferred.

## Acceptance evidence

S03 and E08 have separate production executions. The S03 execution selects the complete public cache/service scenario plus the benchmark. E08 independently reruns all benchmark fixtures. A clean exact source is required for formal evidence.

Each producer preserves four artifacts: raw Go JSONL, stderr, the benchmark payload and the process receipt. Intake checks complete named-test discovery, all six benchmark subtests, source/command/file digests, clean before/after execution, exit/timeout state, exact fixture sizes and distinct phases, input edits, asset version changes, cache counters, raw counter conversion and recomputed percentiles. Summary-only evidence cannot supply a pass.

Negative intake controls reject missing or duplicate artifacts, changed bytes, stale/dirty source, incomplete test selection, skipped/missing fixture executions, duplicate JSON keys, tiny-text proxies, invented percentiles or latency budgets and native/UI claims. A real owned subprocess-tree timeout test verifies that descendants stop writing.

The formal inventory remains 179. These two local contracts do not close RP29's complete security scenario, RP51's race tier, BRG28's iOS UI tier or any installed-consumer/Apple runtime requirement. Clean implementation and executed-producer receipts are archived separately after their gates complete.
