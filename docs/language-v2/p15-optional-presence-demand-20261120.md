# Unconsumed Optional presence metadata: Nil/Legacy repair

The actual clean post-index baseline ddd3c610 reproduced the reported Linux failures. Both original Nil-clear graph tests lost their immediately preceding nonempty native seed. The unchanged canonical nil fixture produced 14 Legacy actions versus 18 migrated V2 actions because private presence Text/Set operations were emitted even though no source consumer used the flag. The separate require(Dictionary) indexed-presence error also reproduced and remains outside this repair.

## Source and verification

- Tests-only native sequence baseline: c75ac67a0f640f1ba841ec8a46c0a6b17803218e.
- Repaired source and paired characterization: e09b3e5231c1142e9cf01385014b0aa577ad0226.
- Clean affected root replay: 994 named passes, zero failures/skips, exit 0, 26.609 seconds.
- Entire syntax, analysis, lower and service packages: 105 named passes, zero failures/skips, exit 0, 6.812 seconds.
- Source status was clean before and after both commands.

Exact commands, all raw Go event streams, stderr and process receipts are retained under evidence/p15-optional-presence-demand-20261120/. The full994 scope includes the original V02, Optional, Optional function, function Map/container, Dictionary/numeric/empty List, L01/F04, NilClear and NilMigration families. It is not the full repository or final179 report.

## Production change

Before emission, the lowerer collects actual presence consumers: authored None comparisons, returns/defaults and written arguments to user functions. It propagates demand through source binding/assignment/alias dependencies. Name collisions deliberately overapproximate demand. The final map is keyed by the original binding AST node; cloned, renamed or otherwise unrecognized compiler bindings retain tracking conservatively.

Only a source binding proved to have no presence consumer omits its private metadata. Its authored native value, explicit clear, assignments and ordinary consumers still emit. A required mutable flag keeps the existing separate identity, source evaluation order, paired writes, alias snapshots and function transport. No native truthiness observation replaces compiler-owned presence. No post-emission action pruning is used.

Original Nil-clear fixture/oracles, canonical tests/nil.cherri, actual migration CLI differential test and require macro tests are byte-identical to the clean baseline. Their immutable blob hashes are recorded. Both Nil-clear tests and the real14-action Legacy/V2 comparison now pass unchanged.

## Actual retained failures and controls

The clean original regression baseline at ddd3c610 records27 named passes and four failed top-level tests, exit1,6.5 seconds. All four are real observations; the require(Dictionary) error is still open.

The tests-only c75ac67a baseline records eight failed leaf scenarios plus their parent. Complete independent native graphs require no unused metadata for None, empty Text, false, zero, reassignment, alias, branch and loop cases. This baseline intentionally failed before the repair.

The first upload guard stopped before production changes or Go execution because it found two existing BindingRef initializers where the script expected one. The no-test receipt is retained; it is not a semantic test result.

The first actual production draft records272 root passes and one failure, plus12 demand-unit passes. The failed newer metadata/input-order characterization had no source consumer of candidate presence, so its requirement for helper actions conflicted with legitimate metadata elimination. That failed raw stream and exact draft source ZIP are retained.

The paired correction preserves that original source verbatim in its own exact no-metadata graph. The demanded variant adds candidate!=none only after every existing implicit-input observation and authors the exact added predicate/consumer graph. All three previous metadata-before-RHS, metadata-after-value and lost-clear corruption controls remain. Removing the added source consumer recovers the original source bytes and graph. The source hash and correction are recorded in implicit-characterization-correction.json.

The corrected development replay passes279 named root events. The final clean994/105 replay includes it, all original mutable/alias/forwarding graph controls, eight complete no-metadata graphs and six corruptions, original/removal implicit-input variants, and eleven demand cases plus their parent. The latter cover reassignment, transitive alias, return, primary/named/qualified argument, branch/loop use, shadowed-name conservative retention and ordinary value-only inputs. Syntax is unchanged by the demand pass, and an unknown cloned binding always retains tracking.

## Limits

The private Optional argument/result protocol and codec descriptor are unchanged. This is a compiler emission repair at a pinned source, not Apple runtime evidence. No immutable requirement, source fixture or native golden was modified. General Optional value joins, dynamic unknown dictionary presence and full container closure remain open.

The next component addresses the still-failing original require(Dictionary) indexed lookup using independently verified source provenance; this report does not claim that failure fixed.

## Clean raw hashes

- Root:162874b1945af466992619ebc6f51eb7cea0a16d8cbf40e383a3e1b426e8b3d5.
- Packages:48a84bd3a4b8d616ce5add2f6b0c7d2a3848c6376c4a5c3ada66958e694f273b.
