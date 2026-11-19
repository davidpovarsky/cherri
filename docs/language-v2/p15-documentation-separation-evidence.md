# D03 documentation consumer evidence

The immutable D03 requirement remains **docs+integration**: active fork
documentation is separate from historical upstream material; Skill and app
search use the current subtree and do not silently recommend historical
examples. This adapter does not change that contract or any of the 179 counts.

The local acceptance runner records D03 as unexecuted. File existence and
successful evidence-model tests cannot close it. Two dedicated producers are
required at the same immutable implementation/Skill commit K, compiler pin S,
docs pin D, and schema/codec/macro fingerprints.

## Published site and installed Skill: docs tier

Run the collector on the actual POSIX consumer host with an already installed
Skill, an ordinary clean source checkout at K, and the canonical clean docs
checkout at D:

~~~sh
python3 tools/p10r15/verify_documentation_consumers.py \
  --source /path/to/source-at-K \
  --skill /path/to/installed/skill \
  --docs /path/to/pinned/docs-at-D \
  --implementation-sha FULL_K_COMMIT \
  --site-build /path/to/cherri-docs-site-evidence.json \
  --site-controls /path/to/cherri-docs-site-controls.json \
  --deployment /path/to/pages-run-metadata.json \
  --output /path/to/new-evidence-directory
~~~

The build and corruption-control receipts must come from the actual deployed
docs revision. Deployment metadata is the real completed GitHub Pages run,
including successful build and deploy jobs. The collector fetches seven live
project URLs and requires the action page and default search index to match
the pinned deployment build bytes. Default search entries may only point at
the current fork guide subtree or current home page.

Eight actual installed wrapper invocations exercise current and explicit
historical scope, generated action search, a clean no-match query, preserved
search execution errors, missing-current failure without historical fallback,
and a restored current query. The missing-current control uses a temporary
directory. Installed docs and Skill bytes remain unchanged.

The output includes 34 hashed artifacts, actual command/exit/stdout/stderr
records, four pinned docs files, installed wrapper/common/manifest bytes,
live HTTP bodies and metadata, and the original deployment/build controls.
`documentation-consumer-evidence.json` contains one D03 **docs** record.
It does not assert an app run.

## Actual app search: integration tier

The iOS collector requires all three exact tests passed once, with complete
noninterleaved blocks in the same successful source-bound iOS Build run:

- `DocumentationIndexTests/testD03BundledCurrentDocumentationSearch`
- `DocumentationIndexTests/testD03RejectsHistoricalAndDamagedBundleRows`
- `CherriEditorUITests/testD03CurrentDocumentationSearchUI`

The positive unit test must emit one allowed raw `CHERRI_DOCS_RECEIPT` inside
its test block. The UI test must emit exactly
`[CHECKPOINT:D03_CURRENT_DOCS_SEARCH_VERIFIED]` inside its own block.
The receipt binds the exact bundle SHA256, S/D and schema/macro pins, the four
canonical source path/hash pairs, section count, exact action-first result,
zero default migration results, explicit migration results, and a zero
missing-query result.

The formal integration record uses exactly five hashed artifacts, all bound
to the same CI run:

- `xcodetest.log`, containing bounded allowed raw test events and receipts;
- `ios-test-diagnostics.json`, with actual process exit and individual outcomes;
- `documentation-index.json`, the exact generated bundle bytes;
- `documentation-receipt.json`, the actual canonical generator stdout;
- `documentation-compatibility-manifest.json`, the exact manifest at K.

Missing generation evidence, duplicate/failed/incomplete tests, wrong targets,
misplaced receipts/checkpoints, stale pins, altered bundle/source bytes, and
summary-only claims fail intake. The compiler S pin is not confused with the
application/Skill implementation K commit.

## Formal intake and scope

The ordinary acceptance manifest validator re-reads hash-bound artifact bytes.
It checks the compatibility manifest against the Git blob at K and source
file hashes against canonical Git blobs at D. It then runs the exact canonical
generator from K against those four D blobs and compares the complete generated
bundle bytes. Changing a body or moving historical prose into a current section
cannot pass by recalculating the supplied hashes and receipts.

This regeneration uses the generator's pure helper, in a temporary directory,
with a 30-second process deadline. It requires Python; set
`CHERRI_D03_PYTHON` to an explicit interpreter when it is not on PATH.
For imported artifacts, set
`CHERRI_D03_DOCS_REPO` to a retained canonical docs repository containing D;
the default is the sibling `cherrilang.org` repository. This input is read-only
and does not fetch or reset a checkout.

Both dedicated records must pass before D03's unchanged two tiers are
satisfied. A valid app record alone, a valid site/Skill record alone, or all
local model tests still leaves D03 incomplete. These records do not close
RP46, BRG29, Apple Shortcut runtime, physical-device, or release acceptance
requirements by inheritance.

The collector unit tests use actual shell subprocesses and explicitly modeled
HTTP responses in temporary synthetic Git repositories. The Go intake tests
model external receipts and execute the real iOS collector adapter. These are
causal verification of the evidence boundary, not external execution evidence.
