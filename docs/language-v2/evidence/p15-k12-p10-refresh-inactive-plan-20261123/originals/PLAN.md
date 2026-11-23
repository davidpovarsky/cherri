# Inactive P10 report refresh plan

Prepared against exact **12f3884e97b4fdcfc86c43fd50fb418700fb9099**; that remains the input-source pin.
The initial read-only observation was clean **6dc2a60980de63f134ce7102f5cc2a74dace93f9**,
whose only difference from the input source was the living master Markdown update.
The packet-creation before/after guards later observed clean
**4443b3ab847528b8408ca29fedd1254113df2a96**. Its actual delta contains that living master update,
seven PoC evidence files and five checkpoint evidence files: thirteen documentation
paths, all outside the dependency set. Both observations have the same 596 input
blobs and fingerprint as the pinned input source. Exact diffs and independent blob
comparison are retained in observation-correction.json and its original diff files.
No source pin was rebound and no production command below has run.

## Findings

Three stale JSON reports come from the existing catalog generator; the fourth
requires a new actual 56-gate execution. The generator writes seven artifacts
together. Editing source/fingerprint fields in the old gate result would not
provide execution evidence.

Independent pinned-blob reconstruction gives **9c6702f5888e2ef89fb52feaa2f8340599c50786f812205a3d16e2fbd94cebdc** over
**596** dependency files. All four reports still record
e29b07449bd5f3758128bbef51466196a14f2179246b48ba1b3df58c75e784f0.
This read-only calculation is not a metadata-verifier execution.

R23 still uses fixed baseline 76d722502e20db977e84a095444d0172442655b8.
The owner prompt returned no choice; the separately qualified DRAFT proposal
is inactive. Metadata repair will let CI reach this real failing gate; it
does not authorize a policy change. The command plan is disabled, with final
source, root execution handoff and policy disposition deliberately null.

## Exact sequence and original budgets

| Stage | Existing command | Source/order | Recorded bound |
| --- | --- | --- | --- |
| Derive once | python3 -B tools/p10r3/generate_catalog_and_pack_ledgers.py | Clean C0 | 600s, then TERM5s/KILL reap5s |
| Run gates once | python3 -B tools/p10r3/run_release_gates.py --run-all --out-results <external>/gate-results.original.json | Clean C1 after derived commit | 1800s outer guard |
| Verify metadata once | python3 -B tools/p10r3/run_release_gates.py --verify-commit | After copying actual report bytes | No explicit CLI timeout in original command |

The optional existing --ci-attestation <external>/ci-attestation.original.json
can be added to the same gate execution. It never calls for a second run.
--run-all already validates all56 registry entries. Its41 Go commands,
13 Python commands and two special gates retain their original selectors,
assertions and child bounds. Ordinary subprocess calls have no explicit
runner timeout; preserve the recorded1800-second aggregate guard.

The generator itself invokes actual go run . --actions-json --no-ansi and
checks505 entries. Use that current source command; do not substitute the
separately pinned R7 executable.

1. Finalize and commit all code, chosen policy and fingerprint inputs as clean
   C0. Record exact SHA/tree/branch and raw/dependency witnesses. Confirm local
   historical Pure bcc0095754e2cc0baaeb4d93dae172b8153b4716 and Pack
   141b9cc96cf1df3f9d1593548dc49ae8a46a256a objects, required by the generator.
   Revalidate raw-source equivalence and the runtime/capacity handoff.
2. Run the unchanged generator once with external original streams and
   monotonic process receipt. Permit only its seven declared output paths.
   Require identical full dependency witnesses before/after, unchanged raw
   hashes, schema/denominator checks and accurate measured/native scope.
3. Commit actual derived outputs and non-input proof as C1. Generated
   sourceCommit remains C0; do not restamp it to the new commit.
4. Execute all56 gates once on clean C1 with the original report outside Git.
   Preserve real exit/status/timing and every original gate ID. A FAIL exits1.
   Original-policy R23 remains FAIL; Oracle R22 remains BLOCKED_PLATFORM.
5. Copy the completed actual report byte-for-byte into the tracked consumer
   p10r3-gate-results.json. Run --verify-commit once, retain its real streams
   and exit, then commit exact report/proof as C2. Report sourceCommit stays C1.
6. Reconstruct the final dependency fingerprint and check source guards.
   C0/C1 are actual ancestors of C2, exactly as the validator requires.
   No producer rerun is justified merely because the final commit advanced.

Do not run the four-report verifier between generation and replacing the
stale gate report. The actual --run-all command does not require a forged
fresh header first. A successful metadata check is not an all-gates PASS.

## Outputs and absence of circular fingerprints

The seven generator-owned paths under docs/apple-runtime/p10/ are:

- p10-505-catalog-coverage-matrix.json
- p10-pack-15-go-only-audit.json
- p10-apple-cherri-crosswalk.json
- catalog-supported-by-compiler.md
- apple-discovered-not-supported.md
- pure-pack-portability.md
- baseline-state.json

The eighth possible tracked update is p10r3-gate-results.json from actual
execution. All eight are excluded from the present dependency set.
The raw execution artifact is included. If raw bytes need replacing, commit
them before choosing C0. Likewise any later approved policy/contract/protected
input changes must be committed before C0; recompute the final dependency set,
not this preparation's fingerprint.

The existing algorithm normalizes CRLF to LF, sorts relative paths, and hashes
each path plus its normalized file digest. Future raw/mode R23 checks remain
separate. The generator's historical branch label and blocker fields are
preserved; record the actual branch in the process receipt and do not rewrite
generated fields or upgrade native acceptance by hand.

## Raw 505 reuse is conditional on the final source

| File | Original measured source | SHA-256 |
| --- | --- | --- |
| p10-505-execution-evidence.json | 11f3c80ca261dd59f198cb3693f893c22e6c340f | 6a99ea3f0e541b4dca416f6cfdef775a7492eef118ddc33f9cc5632398d4aeb8 |
| p10-505-parameter-contract-evidence.json | 9ce44842761ec4bb639e1f2c80c08d86c022b778 | 74c5ed6ffb65594937aefcf248cef7f1d5ed47388fc57352c148f478e14f8eb5 |

Both raw files remain exact originals. The execution and parameter whole test
files are byte-identical to the corrected measured source. Since prior
derived ancestor aa4ce3838b51669326b153262331b88b0c4b8e79, the only matched source changes are four first-line
scheduler calls in three other test files. Removing exactly those calls
reproduces the old bytes. They add no initializer and do not change the
selected producer bodies; the existing scheduler keeps selected runs serial.
The earlier parameter-reuse receipt separately proves the corrected observer's
caller isolation from the unchanged parameter producer.

A raw505 rerun is therefore not justified solely by the demonstrated metadata
drift. Keep the original measured source attribution and all earlier flawed
observations. Recheck final C0: actual compiler/fixture/selected-producer drift
requires a separate bounded handoff. Raw JSON lacks its own source header;
source provenance comes from preserved process witnesses, not a new stamp.

Conditional fallback, disabled by default:

    go test -count=1 -json -timeout=15m -parallel=2 -run '^(TestP10_VerifyAndRecord505CatalogExecution|TestP10_ParameterContractMatrix_AllCatalogActions)$' .

Original Go bound900s, external ceiling1200s, TERM5s/KILL reap5s. Write through
CHERRI_CATALOG_EVIDENCE_DIR to a new external directory, then commit raw bytes
before C0. Ordinary full-suite PASS output uses temporary raw artifact paths;
it is not automatically a recovered evidence file. No unchanged observer
control replay is included without a concrete affected change.

## Future persistent environment

Root explicitly handed off these already prepared paths:

    GOTOOLCHAIN=go1.24.13
    GOMODCACHE=/mnt/development/tooling/cherri-go/mod
    GOCACHE=/mnt/development/tooling/cherri-go/runtime-persistent-20261123/build-cache
    GOTMPDIR=/mnt/development/tooling/cherri-go/runtime-persistent-20261123/tmp-ubuntu
    TMPDIR=/mnt/development/tooling/cherri-go/runtime-persistent-20261123/tmp-ubuntu
    PYTHONDONTWRITEBYTECODE=1
    GIT_NO_REPLACE_OBJECTS=1

Read/write/search access was checked as Ubuntu uid1001 only; no Go/cache warmup
or directory creation occurred. Runtime preparation receipt:
/mnt/development/projects/diagnostics/cherri-tmpfs-source-loss-recovery-20261123/persistent-runtime-preparation.json
SHA2565fdce2555b00204e419441abe92d019c8934fe786171a91ccec775ab01e9d0a6.
Old tmpfs Ubuntu temporary path is absent.

The original retained drivers hard-code superseded runtime paths. Preserve
their historical bytes. A future reviewed task supervisor must substitute
only concrete source/output/environment parameters while preserving existing
producer commands, checks and bounds. Use a new persistent evidence directory
outside source. Retain the prior basic environment allowlist and omit provider
credentials, live AI configuration and private corpus inputs.

## Source and original receipt references

source-audit.json retains all596 pinned dependency witnesses, current headers,
body equivalence checks, source blobs/hashes/ranges and original receipt hashes.

- Generator28–70: dependency algorithm;139–209: Git history objects;
  541–566: live catalog/raw input;785–789 and1126–1155: headers;
  1167–1198: seven output writes.
- Release runner198–206: original R23;656–706: actual dispatch;
  737–786: four-header verifier;799–846: early verification mode;
  874–970: real56 execution, report and exit propagation.
- p10_structural_roundtrip_test.go1708–1782: raw artifact destination.
- p15_install_actual_tools_test.go19–42: selected-run scheduler guard.
- .github/workflows/build-test.yml92–105 and239–252: mandatory CI ordering.

Previous generator execution at aa4ce3838b51669326b153262331b88b0c4b8e79 took52.512742s, exit0, and committed as
1c07e3fee1368d7379616143168c072c79ffe8ad. The real56 run on that clean
descendant took451.427025s, exit1, with54PASS/R23FAIL/R22BLOCKED_PLATFORM.
Original report SHA59b9b4bd18b1c3fd38b25156d7a55c057bdcbbbc522dd623c6ec33b7e6d0f8d6
was copied byte-for-byte and metadata verified. That remained NOT_READY.

No generator, Go, test, gate, metadata verifier, policy activation, workflow,
AI, RemoteBuilder or formal179 was executed for this preparation.

## Additional raw-input comparison

The complete testdata, actions and internal trees, go.mod, go.sum and
p10_native_specimen_test.go are exact Git-object matches at the preparation
source, corrected execution source11f3c80c and parameter source9ce44842.
This covers the native-specimen manifest/data path used by the raw producer.
The compared object IDs are retained in source-audit.json. No native execution
or new measurement is implied.
