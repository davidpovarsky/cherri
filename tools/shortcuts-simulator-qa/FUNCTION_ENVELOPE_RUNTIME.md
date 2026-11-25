# Function envelope no-input runtime adapter

This is a fixed engineering observation for `qa-function-envelope-presence-v2`.
It does not close RP26 or BRG15, authenticate an Apple import, or grant formal
acceptance. The independent outer QA owner must retain the actual import,
installed Receiver/harness identity, signing authority and runtime observer.

## Two source identities

The compiler input remains the retained original preparation:

- Compiler commit: `b82bd63dad0b72078d98d57438a61b463b7dcff7`.
- Fixture source SHA256: `e3d9c1de1ef9ee78b8a11e73b3c296a746a30cf35b69f6a5fddf683f2318241a`.
- Original unsigned plist SHA256: `54889840e546acc60ffe05c8399e1d4bd54f0dc846ec3fd520b4fffc0aa6c7d0`.
- Original compiler export SHA256: `dc22aeac90454dd18a9b27eaee0fa5f14ad9b3f763158985309e7ef946a6655d`.

The harness commit is separate. It must contain the launch/runtime adapters,
fixed graph/parser/fixture, qualified process capture helper and the reviewed
Swift primary-no-input policy/Receiver. The launch records a clean source
snapshot and verifies eleven critical physical files against that harness Git
commit. The outer publisher provides the complete harness source inventory.
The unsigned artifact is not relabeled as having been compiled by the harness.

## Preparation and fixed invocation

The adapter accepts these commands:

~~~sh
python3 -B tools/shortcuts-simulator-qa/function_envelope_launch.py prepare \
  --repo HARNESS_SOURCE_ROOT --root FRESH_ARTIFACT_ROOT \
  --harness-source HARNESS_SHA --compiler-source b82bd63dad0b72078d98d57438a61b463b7dcff7 \
  --run-id OWNER_ID --attempt 1 --nonce FRESH_ASCII_NONCE --udid OWNED_DEVICE_UUID

python3 -B tools/shortcuts-simulator-qa/function_envelope_launch.py launch \
  --repo HARNESS_SOURCE_ROOT --root FRESH_ARTIFACT_ROOT

python3 -B tools/shortcuts-simulator-qa/function_envelope_launch.py collect \
  --root FRESH_ARTIFACT_ROOT --documents ACTUAL_RECEIVER_DOCUMENTS
~~~

The root and its runtime directory must already exist outside the source tree.
Preparation creates a new launch directory and refuses existing paths or
symlinked ancestors. The outer owner imports the exact freshly signed retained
artifact and stops only its owned Receiver process after import, before launch.
The launch command does not contain `--terminate-running-process`; the app
must start as a new process to receive its environment contract.

The only semantic invocation is:

~~~text
/usr/bin/env
SIMCTL_CHILD_CHERRI_QA_PRIMARY_NO_INPUT_CONTRACT_JSON=<exact canonical JSON>
SIMCTL_CHILD_CHERRI_TRIGGER_SHORTCUT_URL=<exact primary URL>
/usr/bin/xcrun simctl launch <owned UDID> org.cherrilang.qa.Receiver
~~~

The contract has exactly six fields: schema, mode, fixtureId, workflowName,
runNonce and primaryURL. Its mode is `primary-no-input`; the URL carries
only name and four callback/source keys, with no input or text parameter.
The Swift policy owns one original intent and completion and refuses subsequent
opens, including input-changing fallbacks. A Python create-only attempt claim
also prevents owner replay for the same artifact root. The child budget is
25 seconds inside a 35-second launch owner. Cleanup is restricted to that
capture helper's owned process group.

This module does not select a device, run a simulator UI test, import/sign a
Shortcut or choose a fallback. An independently source-bound Differential
observer is compatible: its import and runtime-consent evidence stay explicit
outer authorities, and its normal input=text execution route is not invoked.

## Required original artifact layout

The outer adapter writes a compiler manifest with these top-level fields:

~~~json
{
  "syntheticOnly": true,
  "compilerCommitSha": "<original b82 compiler SHA>",
  "harnessCommitSha": "<actual harness SHA>",
  "compilerArtifactOrigin": "retained-original-preparation",
  "fixtures": ["the exact fixture row described below"]
}
~~~

The unique fixture row has fixtureId, fileName
`CherriFunctionEnvelopePresenceProbe.cherri`, workflowName
`CherriFunctionEnvelopePresenceProbe`, dialect v2, buildProfile debug,
runtimeTraceExpectation REQUIRED_NDJSON, sourceSha256, unsignedPlistSha256,
nativeActionCount 379, and functionEnvelopeProbe. The last value is obtained
from `function_envelope_runtime.prepared(repo, fixtureId, sourceBytes,
unsignedBytes)`; the original 379-action graph validator checks the complete
compiled graph. This does not authenticate an invocation.

Retain these originals:

- `compiler-manifest.json`.
- `builds/CherriFunctionEnvelopePresenceProbe.cherri`.
- `builds/CherriFunctionEnvelopePresenceProbe.shortcut`.
- `builds/signed/CherriFunctionEnvelopePresenceProbe.shortcut`.
- `builds/function-envelope-compiler-export.original.json`, copied unchanged.
- `runtime/signing-provenance.json`, containing actual signed-byte/hash/size
  and targetProvenance joins to the original compiler/source/unsigned bytes.
- All files written under `runtime/function-envelope-launch`, including
  claim, stdout/stderr, started/finished journals, process/result and source
  before/after records.
- Byte-identical Receiver owner, contract, intent and completion under
  `runtime/function-envelope-trigger`, with the acquisition manifest.
- Actual `runtime/callback-events.json` and normalized
  `runtime/runtime-trace.jsonl`.

The signing summary binds compilerCommitSha, signedArtifactValidAEA1,
signedArtifactSha256, signedByteSize, and targetProvenance. The target contains
compilerCommitSha, sourceSha256, unsignedPlistSha256, unsignedByteSize,
nativeActionCount, signedArtifactSha256, signedByteSize and headerMagic AEA1.
The header/hash checks do not themselves authenticate an Apple signature.

## Runtime API and credit boundaries

`evaluate(repo, root, fixtureId, compilerSHA, harnessSHA, nonce, "")` joins
the complete graph, retained compiler export, signed bytes, fixed invocation,
Receiver PID and four exact trace events with one correlated callback.

`emit(repo, callbackPath, nonce, fixtureId, "", tracePath, manifestPath,
compilerSHA, harnessSHA)` prints that observation and retains the exact
normalized trace. It refuses an existing different trace.

`apply_assertions(assertions, oracle, repo, root, compilerSHA, harnessSHA, "")`
recomputes the observation and requires agreement before setting engineering
observation fields. `validate_public_diagnostic(value, root)` independently
recomputes it again. Both source identities remain explicit in every join.

A missing caller key remains UNOBSERVED. Contrary but well-formed CASE fields
remain verified observations with declaredNoInputGoalsMatched false. A valid
callback alone, workflow success, empty type witnesses, fake/absent process
receipts, failed cleanup, repeated invocation or missing original do not pass.
importTransportAuthenticated, appleExecutionAuthenticated,
semanticContractVerified and formalAcceptanceGranted remain false here.

## Qualification

The original Python qualification selected all twenty new parent controls and
163 subtests. All passed in 21.99 seconds, with identical source snapshots.
The process, Receiver and AEA1 signature in these controls are synthetic
doubles; no simctl, Apple signing/import/execution or Go command ran.

Original receipt:
`/mnt/development/projects/evidence/cherri-p10r15-ci-release-20261125/function-no-input-runtime-qualification-01/result.original.json`

SHA256:
`cddc89d70b0b6090eb89d42e2f56d35e41b3f25e6a162f564e23ff33c03fe593`.

The existing graph and observation parser were unchanged. Their earlier
original controls remain separate; this qualification does not relabel their
source or claim a fresh native observation.
