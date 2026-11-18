# P10R15 runtime harness, Editor Inspector and publication gate

Date: 2026-11-18. Base: `ca8a62327e429d5574c8f2a5d440de2c5e553ab2`.
Asana: `1220546287612106`, `1220655985907047`, `1220646564229070`.
Execution: ChatGPT subagent `/root/runtime_inspector`; central integration and CI dispatch remain with the main agent.

## Implemented scope

The existing HubSign, receiver, import handshake and fourteen runtime gates are retained. Explicit workflow choices now select `qa-functional-legacy`, `qa-bool-list` and `qa-short-circuit`. Legacy is the primary imported and executed file when selected; signing a counterpart is no longer the only available Legacy path. The Bool/List fixture consumes ordered Dictionary objects from a List and derives its callback values from those native results. The F01 fixture exercises false-valued variables, negation, statement and expression short circuit, precedence and guarded nonexistent Shortcut invocations.

The production callback verifier now requires a non-self-test event from `shortcuts-x-callback`, exact fixture/name and fresh run ID, an exact result marker, and valid NDJSON collector records. A marker embedded only in unrelated text or trace output is rejected. Existing success/error/cancel distinctions remain fail closed; interactive negative cases are not newly claimed as executed.

`editor_inspector.py` independently decodes the actual compiled plist and consumes the AX hierarchy. It excludes toolbar/library ancestors, groups observed containers within the canvas, retains identical adjacent cards, and only deduplicates page overlap with stable native identity. Missing trees, identity, typed fields, values, incomplete scroll evidence and ambiguous overlap remain explicit unknowns. It writes `editor-actions.json`, `editor-comparison.json`, and `editor-report.md`; the historical comparison filename is an alias of the new report. It does not infer action identity from English titles or global text counts. No OCR, paid vision, or screenshot interpretation is used.

Swift capture records finite geometry, identifier digests, available native identity hints and the current run ID. A missing or unencodable AX tree fails explicitly instead of being silently skipped. Full capture still has its narrower historical meaning; summary readiness becomes `RUNTIME_VERIFIED_EDITOR_NEEDS_REVIEW` if runtime passes but the independent inspector cannot establish one-to-one correspondence.

## Explanation of the historical 33 versus 34 discrepancy

The historical `detectedActionCards` array is a set of distinct strings from Cell, StaticText, Button, TextField and TextView nodes. The downloaded synthetic artifact from run [43941751610](https://github.com/davidpovarsky/cherri/actions/runs/43941751610) includes Share Shortcut, Run Shortcut, Scripting, Continue, parameter buttons and actual action labels in that array. Therefore its 34 entries are not 34 observed native actions. The artifact also lacks `editor-tree.json`; its import-sheet tree is not a substitute. New code reports this evidence gap rather than inventing a corrected count. The exact cause of the historical missing tree is not established.

## Publication policy and limitations

`prepare_public_artifacts.py` verifies the generated synthetic manifest against the repository fixture source hashes and stages a new, explicit text-only publication directory. It never changes the raw evidence directory. Raster media, raw Shortcut/plist/source inputs, xcresult bundles, archives, unknown paths and binary disguises are quarantined outside the upload set. An invalid/unsafe allowlisted text artifact blocks publication before staging completes. Secret matches are not echoed in diagnostic output. The exact scanned bytes are staged, avoiding a scan-then-reread gap.

This is publication quarantine, not pixel redaction. OCR cannot prove arbitrary private images safe; no raw screenshot is uploaded by this workflow. The uploaded publication manifest records file hashes and reasons, and declares `pixelRedactionClaimed=false`. Raw files remaining on an ephemeral CI runner are not durable public deliverables. Private user inputs remain outside this synthetic workflow.

## Verification and open gates

Focused Python tests cover AX cardinality/order/identity/value mutations, repeated cards, stale runs, Hebrew/emoji, missing trees, partial geometry, virtualized identity reuse, publication provenance, nested text secrets, callback URL tokens, raw plist, PNG metadata, raster-only English/Hebrew/low-contrast text and binary disguises. A separate suite executes the actual shell-embedded callback verifier against positive and negative events. These are deterministic synthetic unit fixtures; they are not a human-reviewed Apple AX ground-truth corpus.

The central ledger must record the final test counts, code SHA and new CI URLs. The initial legacy and Bool/List compilation on the P14 binary produced 34 and 30 native actions respectively. F01 runtime requires the shared-core repair integrated by the main agent.

Remaining acceptance: actual new xcode-27 runtime for each selected fixture; real AX tree and exact card mapping where Apple exposes it; optional local OCR with verified language support; expanded-parameter UI tests; independent human-reviewed precision/recall corpus; live Cancel/Error/Denied and Run Shortcut pair cases; >100-action long fixture; physical iPad tests. None of these is closed by synthetic unit test success. `PHYSICAL_DEVICE_TESTED=BLOCKED_DEVICE` remains unchanged.

## Reproduction

```sh
python3 -m unittest discover -s tools/shortcuts-simulator-qa -p 'test_*.py' -v
go test -count=1 -run 'TestP14_|TestP13_SimulatorQA|TestP10_A1_' .
# On the existing xcode-27 runner, after the core repair is integrated:
./scripts/ios27_shortcuts_simulator_qa.sh full ipad-default qa-functional-legacy true 20 true
./scripts/ios27_shortcuts_simulator_qa.sh full ipad-default qa-bool-list true 20 true
./scripts/ios27_shortcuts_simulator_qa.sh full ipad-default qa-short-circuit true 20 true
```

Rollback: revert this coherent harness commit on the feature branch. It makes no signer, private corpus, production server, installed device or default-branch changes.

## Follow-up: dispatch and report contracts

Workflow inputs are transferred through environment variables into quoted shell arguments. This prevents a string input from being evaluated as shell source. An explicit `capture_full_editor=false` survives GitHub's fallback expression. Page limits accept only canonical integers 1 through 20; oversized integer strings and unsupported modes fail before compilation or simulator side effects.

Each invocation generates a 128-bit random suffix for the runtime nonce. GitHub reruns retain their run ID, so the earlier deterministic fixture/run-ID combination alone could not distinguish an old callback from a new attempt. The receiver's existing nonce field and launch URL are reused.

The output assertion report now requires the strict oracle's acceptance and the same exact source, fixture, Shortcut name, nonce, and semantic output. A stale or unrelated SUCCESS event cannot set `compiledShortcutCallbackVerified=true`. ERROR and CANCELLED remain separately observed statuses, with causes unverified. Trace validation requires schema 1.4, a single ordered collector envelope, exact collector identity, native action identifiers, valid field types, and increasing sequence numbers for these linear fixtures. Unknown, malformed, repeated or out-of-order records cannot silently pass. The report explicitly records `typedValueCoverage=UNKNOWN_AT_TRACE_EVENT` and `perActionCoverageVerified=false`; this closes validation defects without pretending the exporter records all typed values.

## Exact-candidate CI handoff

The existing `build-test.yml` automatically runs on `agent/**` pushes. Its Linux and macOS jobs build and test, run the discovery and parity gates, and verify the P10 registry/coverage/release gates. Its Nix job runs `nix build`, which includes the package checks. Avoid a duplicate manual Build & Test dispatch when the push run already exists for the same commit.

The signed simulator workflow has push filters for P13/P14 branches and is explicitly dispatched for P15. Its runner label is `xcode-27`, timeout is 45 minutes, repository permission is read-only, and distinct sample IDs have separate concurrency groups. `sample_id=all` compiles the synthetic fixture set but executes `qa-functional-v2` as its primary target; it is not a runtime matrix.

For a fixed integration branch at the candidate commit:

```sh
gh workflow run cherri-ios27-shortcuts-simulator-qa.yml \
  --repo davidpovarsky/cherri --ref <candidate-branch> \
  -f probe_level=full -f device_profile=ipad-default \
  -f sample_id=qa-functional-legacy -f capture_full_editor=true \
  -f max_editor_pages=20 -f require_runtime=true \
  -f upload_sanitized_artifacts=true
```

Use separate dispatches for `qa-bool-list` and `qa-short-circuit` after the compiler repair is integrated. Record each run's `headSha`, jobs, conclusion and published manifest. A workflow run on a different SHA cannot close the candidate's gate. New pushes to the same branch can cancel Build & Test; keep the candidate fixed while its checks run. Only the text publication directory is uploaded, subject to the existing quarantine gate. No new Apple runtime success is asserted by this handoff.
