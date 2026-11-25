# Function envelope: one no-input receiver launch

This is a diagnostic transport boundary, not RP26/BRG15 semantic acceptance.
The immutable fixture is `qa-function-envelope-presence-v2`; its compiled
self-call/import name is `CherriFunctionEnvelopePresenceProbe`. The compiler,
unsigned graph, signed artifact, receiver build and actual launch each need
their own source and original-artifact binding.

## Exact supplied contract

`CHERRI_QA_PRIMARY_NO_INPUT_CONTRACT_JSON`, when present, enables the constrained
mode for the receiver process. It must contain exactly these six string keys:

| Key | Required value |
| --- | --- |
| schema | `cherri-function-envelope-no-input-launch/1` |
| mode | `primary-no-input` |
| fixtureId | `qa-function-envelope-presence-v2` |
| workflowName | `CherriFunctionEnvelopePresenceProbe` |
| runNonce | 1–128 ASCII letters, digits, hyphen or underscore |
| primaryURL | The exact approved UTF-8 URL bytes |

The JSON is compact, sorted by key, UTF-8 with Unicode emitted directly, no
escaped slashes and no trailing newline. Canonical byte comparison rejects
duplicate keys, including equal duplicates, alternate escaping, extra fields,
reordered fields and whitespace. A present but malformed contract never falls
back to ordinary QA behavior.

The URL is at most 8 KiB and uses exactly
`shortcuts://x-callback-url/run-shortcut` with five query parameters:
`name`, `x-source`, `x-success`, `x-error`, `x-cancel`.
Name is the exact compiled workflow name and x-source is
`CherriSimulatorQAReceiver`. The three callbacks have their respective
`cherriqa://success`, `cherriqa://error`, `cherriqa://cancel` routes and exactly
`runId`, `fixtureId`, `shortcutName`, `source`. Their nonce, fixture and name
match the contract by UTF-8 bytes; source is `shortcuts-x-callback`.
Neither level admits input/text, duplicates or unknown semantic parameters.

## Single actual attempt and private originals

One gate lives in `CallbackEventStore.shared` for the app process. The existing
callback reset does not reset it. A first semantic trigger consumes the gate
even when its source, URL, contract or evidence write is rejected. An approved
intent is written and read back before the existing UIApplication open call.
An unsuccessful completion is terminal; scene.open fallback is disabled in
this mode. Receiving a success/error/cancel callback does not request a new
semantic launch.

The ordinary mode, with the environment key absent, preserves its previous
opening and fallback behavior. The new gate does not authorize signing,
import, alternate URLs, extra attempts, simulation reset or source promotion.

Private originals live under:

`Documents/primary-no-input-launch/<contract-sha256>/`

The directory and files are create-only. Owner and exact supplied contract
originals precede `NNN-intent.original.json`,
`NNN-completion.original.json` and any rejected request records. Event sequence
is separate from `attemptOrdinal`; a rejected duplicate is not a second open.
Intent alone is not proof of an invocation or successful Apple execution.

Normal rejected URLs up to 64 KiB retain their exact UTF-8 base64, hash and
length. Oversized input is blocked before opening, explicitly reports
`retentionFailure` / `ORIGINAL_EVIDENCE_WRITE`, and does not claim a complete
original. A maximum of 32 event files bounds repeated rejected requests.
Any missing original, retention failure, second actual invocation, missing
completion or fallback disqualifies the runtime provenance join.

## Controls and remaining work

`verify_primary_no_input.py` compiles the production Foundation policy and
byte-identical receiver trigger/reset method bodies. Its UIApplication and
UIWindowScene implementations are explicitly local test doubles. They measure
method calls and file creation order; they never execute Apple Shortcuts.
The original callsite at `2e4b3ece` produced 129 controls with 17 causal
failures for repeated opening, fallback and absent original receipts.
Canonical contract, malformed/duplicate/Unicode inputs, exact rejected bytes,
private write refusal and replay controls are independently exercised.

The normal receiver sanitizer accepts only the exact versioned public
`CHERRI_FUNCTION_ENVELOPE_PRESENCE_V1:` prefix, retains contrary observations,
and preserves its existing secret/PII rejection and 16,384-character limit.
The general callback parsing and all existing XCTest bodies are unchanged.

Actual standalone receiver build, one no-input launch, signed/import lineage,
three CASE observations and independent trace recomputation remain separate
required operations. A Foundation PASS grants no native or formal credit.

### Full-input privacy scan

The original 50 sanitizer controls reached the unchanged 30-second executable
deadline. The prior email alternative retried its local-part scan at each
character of a long run without an at-sign. The retained original executable
took 0.082, 0.191 and 0.586 seconds for the same 1,000/2,000/4,000-character
nonmatching-tail controls. This is component diagnosis, not Apple performance.

The email alternative now begins only at the start of a maximal local-part
character run. Any prior substring email match can extend to that same run
start, so its presence remains detectable. The scanner still reads the full
input before the output cap, and all other secret/PII alternatives are unchanged.
All original 50 controls remain; the added boundary controls include adjacent
local-part characters, Unicode boundaries, nonemails and secrets beyond 16 KiB.
No timeout or output bound is increased.
