# AI01 profile v2: Sonnet 5.5 transport and native compiler calibration

Status: the exact reviewed R7 Linux ARM64 compiler is pinned and all twelve
original public baseline compilations and graph replays passed. Final
independent source/protocol review and frozen-source execution handoff remain
required before any real evaluation bundle. No fresh evaluation seed, provider request or measured
quality outcome is part of this change. The historical v1 protocol and receipts
remain unchanged.

## Why this is a new profile

The original `fresh-six-exact-instances/v1` profile pins Sonnet 5.5 but sends
`thinking.type=disabled`. On 2026-11-23, Anthropic's official
[Sonnet 5.5 migration guide](https://platform.claude.com/docs/en/models/sonnet-5-5/migration-guide)
states that this value returns HTTP 400 for that model. This finding is a
documented compatibility failure, not an observed paid API attempt.

The same official guide specifies `between_tools` as the setting without
up-front thinking and says a request without tools returns text only. Profile
v2 uses that exact setting and explicitly pins `output_config.effort=high`,
the model's documented default. It does not enable tools or tool-use loops.
[Model identity, lifecycle and default effort](https://platform.claude.com/docs/en/models/sonnet-5-5/overview)
were checked on the same date: the canonical API ID is `claude-sonnet-5-5`,
its status is active, and retirement is not before 2027-09-28.

Selection is explicit: `--profile fresh-six-exact-instances/v2`.
The default remains v1, including its original settings and S4 compiler pin.
Unknown profiles and mixed profile/provider/compiler/environment combinations
are rejected. Original v1 bundles continue to require their exact original
source bytes; they must not be restamped with newer producer hashes or renamed
as v2. A signature and every outcome seal bind the original manifest.

## Settings that remain fixed

| Setting | Both profiles |
|---|---|
| Model / API version | `claude-sonnet-5-5` / `2026-10-21` |
| Tasks | The same six public generation/edit patterns, paired across Legacy and V2 |
| Outcomes / retries | Exactly twelve declared outcomes / zero retries |
| Input / output caps | 12,000 / 2,000 tokens per outcome, equal for both arms |
| Transport ceilings | Count 15 seconds, generation 90 seconds |
| Runner / compiler ceilings | 120 / 35 seconds |
| Context | One fixed system instruction and one stateless public user payload |
| Excluded capabilities | Streaming, tools, MCP, cache requests, shared conversations, fallbacks, redirects |
| Quality / provenance | Actual measured outcomes; fresh exact instances only; no unseen-pattern or Apple-runtime claim |

The immutable AI01, RP50 and BRG32 requirements and all 179 requirement IDs are
unchanged. The six task templates, guides, generator, original HMAC domain,
numeric domain and oracle semantics are unchanged. Keeping the original
instance derivation also keeps the twelve authored public calibration inputs
identical. A new real evaluation still requires a new CSPRNG seed after source,
model and compiler calibration are frozen; there is no seed search or reuse of
earlier outcomes.

V2 provider and validator receipts explicitly name their evaluation profile.
Canonical intake requires the selected profile's exact HTTP field inventory,
thinking mode, effort, compiler pair and isolated environment. V2 also binds
this protocol file in its seven-file sealed producer inventory and canonical
Git-source intake, as well as the Go source witness. V1 retains its original
six-file inventory. Any post-seal protocol change is rejected. It retains the
original rejection of thinking/tool content or usage, unexpected state/cache,
budget violations, incomplete responses, source drift and invalid signatures.
Valid refusals/truncations remain measured task failures; incomplete transport
or adjudication cannot produce a complete quality score.

## Compiler and environment boundary

Evaluation implementation source and compiler source are distinct identities.
The new profile fixes compiler source to R7
`ca316a3dee252456c4bec751594cf249c709b331`.
The exact native binary SHA256 is
`42f6e42e7b6a8ea6f523a752fd2f06b96265e733fa00ad808190e062d1266301`.
It was built from a clean canonical R7 clone with Go1.24.13, CGO disabled,
Linux/ARM64 and embedded unmodified R7 VCS identity. The actual pair receipt
has SHA256 `d3bae0c50203bf0237289f8d18b242db1997d95feac5ebcbb2f2beee721fdcfc`.
Identity pinning and the actual twelve-case calibration below are separate
requirements before a real seed. Preparation and loading reject an absent compiler pin
with `UNREVIEWED_PROFILE_COMPILER`; no environment or CLI override bypasses it.

The bundle retains the existing `compiler.exe` filename as its envelope.
For v2 it contains the exact Linux ARM64 binary and receives owner-only mode
0700 when copied. POSIX loading requires its owner-execute bit. The hash is
checked before and after compilation. V1's filename and mode behavior remain
unchanged.

V2's `isolated-defaults-linux/v2` environment adds an attempt-local `TMPDIR`
to the existing isolated OS/locale and TEMP/TMP environment. Credentials,
HOME/USERPROFILE, inherited PATH/temp paths and Cherri corpus/feature overrides
remain absent. This is an environment policy, not an OS sandbox claim.

Before a real seed, compile the same twelve authored public baseline inputs
once using that exact R7 binary, one per task/arm, with unchanged 35-second
per-case and 450-second aggregate limits. Retain the original plan, source
bytes, commands, binary/VCS identity, streams, plists and every failure.
Independently replay the same graph oracle against those actual outputs.
Preserve the S3/S4 calibration originals separately. Any demonstrated mismatch
must be repaired and reviewed before freezing a new evaluation implementation.

The actual R7 calibration ran at clean implementation
`752c41b74f64fd633e63792fd4685156fabe913c`: twelve original source files,
twelve compiler exits0 and twelve graph replays PASS in0.808499 seconds.
All source and binary bytes remained unchanged. See the
[original calibration packet](../../docs/language-v2/evidence/p15-ai01-v2-r7-calibration-20261123/README.md).
This documentation follow-up changes no executable calibration producer.
No real evaluation seed, provider request or model-quality measurement occurred.

## Budget and authorization

The reviewed session authorizes the bounded twelve-outcome continuation.
As checked on 2026-11-23, the official model page prices Sonnet 5.5 at $2 per
million input tokens and $10 per million output tokens.
Twelve outcomes at the accepted 12,000/2,000 token caps total a nominal
`12 * (12000 * 2 + 2000 * 10) / 1000000 = $0.528`.
[Token counting is free](https://platform.claude.com/docs/en/build-with-claude/token-counting).

This arithmetic is not a provider-enforced dollar ceiling. Token counting is
an estimate; actual over-cap usage can already have been charged when intake
rejects it. Actual usage and all unsuccessful attempts must be retained.
No account quota or monetary limit is changed by this profile. No credential
value belongs in source, arguments, plans, receipts or public evidence.

## Freeze, anchor and execution handoff

1. Independently review the profile implementation, cost bounds, actual exact
   compiler receipt and twelve-case calibration. Pin the real compiler hash.
2. Freeze the final evaluation implementation in a clean Git commit. Canonical
   Go intake requires this exact implementation SHA, even when compiler source
   is the separate R7 pin. Do not label a later tooling commit as K8.
3. Preserve an original authenticated model-identity response and receipt.
   Key-variable presence or public model documentation alone does not prove
   current account entitlement or replace that response.
4. Prepare one new absolute bundle directory with explicit v2 selection:
   `evaluation_bundle.py --prepare --profile fresh-six-exact-instances/v2`,
   supplying its exact clean code root, compiler, model receipt and response.
   The ordinary required arguments and source/seed/signature guards remain.
5. Publish the generated commitment to the designated Asana task and retain two
   independent authenticated pre-call observations. Keep the trusted before
   observation outside the bundle. Only the reviewed execution handoff may set
   `CHERRI_AI_EVAL_ALLOW_GENERATION=1` for the exact anchored bundle.
6. Run the existing sealed runner with all twelve original outcome slots, then
   retain an independent authenticated after observation outside the bundle.
   Execute canonical Python intake against both observations and the unchanged
   production Go acceptance path. No restart or outcome-driven replacement
   bundle is allowed.

## Offline verification scope

Existing test methods and their assertions remain unchanged. Added controls
exercise real Ed25519 sealing of explicitly synthetic deterministic fixtures,
unreviewed compiler admission before seed/output creation, both exact profiles,
signed config/cap/compiler/environment mutations, v1 relabeling, v2 wire
allowlists and receipt/profile tampering, unchanged twelve-outcome accounting,
and the isolated Linux temporary directory. All provider transport and compiler
execution in these unit controls are local fakes.

The macOS CI failure at K8 was an independent environment defect:
its default temporary directory traversed the `/var` alias and correctly hit
`INPUT_PARENT_SYMLINK`. The Build & Test AI steps now select the physical path
of the existing `RUNNER_TEMP` as TMPDIR before launching the unchanged unittest
discovery command. Production `read_regular` and its no-symlink policy are
unchanged; arbitrary evidence inputs are never silently resolved.
