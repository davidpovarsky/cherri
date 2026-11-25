# Paired Cherri differential fixtures

This suite contains 39 new, synthetic workflows: 37 deterministic workflows
eligible for native execution, and two timer quantity sentinels that are
structural only. It compares the real current original compiler, the frozen
original revision, the fork's legacy path, and idiomatic V2. No fixture or runner
hand-authors a Shortcut plist.

## Fixed semantics and native contract

The manifest records each fixture's action families, complexity, sources,
independent expected Unicode text, and reason for that expected result. Inputs
are embedded in the source. Output is the only terminal action; there are no
alerts, Show Result dialogs, clipboard writes, messages, or network requests.

Runtime output must equal expected.value exactly. Do not trim whitespace,
coerce types, rewrite Unicode, or change an expected output to agree with an
observed compiler result. The manifest was authored before native execution.

The runtime pair uses distinct names CHERRI_QA_D30_O_NN and
CHERRI_QA_D30_V_NN. That naming difference is deliberate metadata so both
versions can coexist in Shortcuts. The fork legacy lane uses the **exact same
source bytes** as the current original lane. The frozen original lane preserves
the older loop-variable spelling in frozen_legacy/, explicitly recorded in
the manifest; it is not silently substituted for the current original.

Fixtures 38 and 39 call Start Timer. Their runtime.enabled is false; do not
execute these side effects as part of the automated pure suite.

## Run

From the repository root, with binaries built from the pinned source revisions:

    python3 tests/differential_30/run.py \
      --original /path/to/pristine-current-cherri \
      --original-sha 98d5191ae121a6bcf663ed0e55495f4ee4c7a6e2 \
      --frozen-original /path/to/pristine-frozen-cherri \
      --fork /path/to/fork-cherri \
      --fork-sha FULL_SOURCE_SHA \
      --lanes original,frozen_original,fork_legacy,fork_v2 \
      --out /path/to/new-evidence-directory

The runner refuses an output directory that already contains a receipt.
To compare a repaired fork, use a new --out and --reuse-from the earlier run.
It reuses only successful compilations with matching source bytes, compiler
binary hash, and compiler flags. Changed fork binaries are compiled again;
unchanged original outputs need not be rerun.

Use --only 04,16,39 for a focused follow-up. --workers sets independent
compiler subprocess concurrency. No shared compiler process is used.

The normal run uses standard compiler UUID generation. --derive-uuids is an
explicit extra mode for auditing that option. The first baseline exposed
duplicate producers and duplicate nested group identities in both original
legacy compilers and the fork under that option; those artifacts and findings
were retained separately before selecting the ordinary compiler mode for
native semantics.

Original compiler versions ignore --output with --skip-sign and emit
SOURCE_unsigned.shortcut. The runner records that exact produced path and
copies those compiler bytes to the artifact path. It does not reconstruct or
repair their plist.

## Evidence and interpretation

A run writes:

- manifest.json: independent expected results used by that run.
- results.json: source/binary hashes, actual commands and exit codes, compile
  status, artifact hashes, graph checks, and comparison paths.
- sources/LANE/: exact source copies and any compiler debug products.
- logs/LANE/: complete compiler output, including failures.
- artifacts/LANE/*.shortcut: untouched unsigned compiler bytes.
- artifacts/LANE/*.typed.json: complete decoded trees with explicit scalar
  tags; Boolean, integer, real, text, data and date remain distinct.
- comparisons/: every raw and alpha-renamed typed difference, action sequence,
  reference graph, control nesting, and the existing independent fidelity
  oracle's report.
- fork-action-catalog.json: actual fork catalog used to label possible
  static/default parameter differences.

The runner imports tools/p10r7/independent_fidelity_oracle.py; it does not
replace the project's strict fidelity gate. Its supplemental exact diff retains
values because all data here is explicitly synthetic. UUID renaming is permitted
only for structural identity keys through a bijective mapping over an identical
action sequence. Ordinary UUID-looking text remains exact. Invalid producer or
reference graphs cannot pass the strict wire result.

Workflow metadata, display candidates, and action-specific catalog
static/default candidates receive separate labels. **No label removes a
difference or establishes runtime equivalence.** Both raw and alpha differences
are saved without a truncation limit. When V2 generates a different action
sequence for immutable bindings, zero-based indexing or value blocks, the
structure remains different and native execution must decide semantic parity.

WIRE_DIFFERENT is not a synonym for a compiler defect. Compile failures,
upstream language changes, fork-only actions, intentional V2 lowering,
preserved original defects, and observed runtime failures require distinct
interpretation. Runtime status is always NOT_RUN in this runner: signing,
importing and executing are owned by the native runtime harness.

## Known version boundaries retained as evidence

- Current original 98d5191 uses bare constant loop bindings; frozen original
  bcc00957 uses @ references. Baseline fork 6ce0a398 rejects the bare numeric
  repeat binding in fixture 23. The repaired fork accepts both forms; the
  original failure remains in the retained baseline receipt.
- Original catalogs do not define getTimeBetweenDates used by fixture 31.
  That compile failure is retained and the workflow is not counted as an
  executed original runtime pass.
- Frozen original rejects raw-text dictionary coercion in fixture 36;
  current original and the fork accept it.
- Fixture 39 specifically requires a typed reference inside quantity
  Magnitude. A printable implementation-struct string is a defect signal,
  never a valid quantity or a normalization candidate.

## Comparator adversarial verification

Run this against compiler-produced artifacts after the compiler pass:

    python3 tests/differential_30/verify_runner.py /path/to/run \
      --out /path/to/runner-verification.json

The checks mutate actual compiled trees to verify UUID bijections, dangling
references, identity collisions, nested grouping, scalar type differences,
metadata and display changes, explicit defaults, and action removal. They do
not manufacture native workflow fixtures. They verify the comparison adapter;
they are not a substitute for Apple runtime execution.

## Explicit oracle correction and native evidence

Case 35 originally expected the renamed Text item to retain its extension in
its Name value. The pre-existing project contract states that Name returns the
stem. The correction to `CHERRI_QA_35|qa:qa` is recorded in
`oracle-corrections.json` and the associated audit; every original manifest and
failed native verdict remains retained. No authored program changed.

The statistics defect in case 15 is different: both original and baseline fork
emitted `WFInput`, while the real Apple action consumes `Input`. The repaired
compiler corrects the shared definition and the original independent expected
output remains unchanged.

Current native acceptance requires a fresh receiver process, exact installed
executable-image verification, fixture and nonce identity, and the SHA-256 and
byte count of the original UTF-8 output. Older single-line receipts retain their
original contract and attribution; no missing digest is fabricated. Native
results and exact compiler program bindings are reported separately from wire
equality and from UI/import failures.
