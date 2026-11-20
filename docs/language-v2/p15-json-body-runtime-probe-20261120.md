# Fixed public JSON body observation: source and local controls

Implementation `a8b69008d70f48989bf3c3db6258e0e22ee85eda` adds a two-case diagnostic to the existing signed Simulator QA route. It does not change compiler JSON/Optional support, existing runtime goldens, the signing service or formal requirement memberships.

The `qa-json-body-control` fixture posts a fixed public body to a numeric loopback address. Its empty Text, Boolean false, numeric zero, deliberately missing key, nested Boolean flags and nested matrix are distinct controls. These fields preserve the corresponding captured frozen-K native forms; only the original model variable is replaced with fixed public text, and explicit zero/marker fields are added. The `qa-json-body-null-candidate` fixture adds one experimental `WFItemType:5` field with absent `WFValue`. Its semantics are explicitly **not assumed**. Neither metadata survival nor a successful diagnostic callback establishes JSON-null support.

## Bounded real observation path

Each case receives only an ephemeral loopback port and fresh run nonce through ShortcutInput. The exact compiled native graph contains one POST and one separate GET continuation. The reviewed receiver has a finite1800-second lifetime, maximum4 requests,8192-byte request body and2-second socket allowance. It records only the fixed public body bytes when every control and field is recognized; unknown body values are represented by a digest, size and redacted category. No request header, arbitrary path or unknown body is published.

The ordinary import, signature, source, correlated callback, exact output and required trace gates remain required. A separate diagnostic binds the original request bytes, callback bytes, source, contract, unsigned artifact, full graph and24 causal mutation receipts. It distinguishes candidate `NULL`, `ABSENT`, `EMPTY_TEXT`, `BOOLEAN_FALSE`, `NUMBER_ZERO` and redacted/unknown observations. `controlledBodyVerified` requires the exact POST→GET sequence, completed writes, finite bounds and successful actual callback. `candidateNullObserved` is a separate observation; `formalAcceptanceGranted` remains false.

Publication adds only `runtime/json-body-diagnostic.json`. Its validator re-parses the original bounded request bytes and requires exact source/callback bindings. Raw server receipts and inputs remain outside the public allowlist. The listener binds `127.0.0.1`; there is no external endpoint or user payload.

## Actual local validation

A clean fresh compiler build and the unchanged full Phase0 heredoc passed on the implementation SHA:83 compile attempts,81 native artifacts and2 existing expected diagnostic rejections. Both new actual16-action graphs passed and rejected all24 causal mutations each. The mutations cover external hosts, borrowed input, wrong offsets/method/body/type, lost zero, false-to-text, extra requests, clear seeds, response edges, trace edges and final output.

The clean Windows replay passed13 JSON tests,25 existing retry tests,17 existing publication tests and17 existing session tests. Two session process-group controls are platform-skipped on Windows; no native result is inferred from them. The new controls include actual local HTTP requests, distinct null/absent/empty/false/zero bodies, actual publication with byte equality, receipt/callback tampering and the closed shell dispatcher. These local clients and constructed callback controls are **not Shortcuts runtime evidence**.

[The machine-readable proof](evidence/json-body-runtime-probe-controls-20261120.json) contains every source digest, actual compiler/artifact hashes, clean command receipts and retained draft failures. The initial missing native producer UUID, an overstrict unused-GET UUID assumption, truncated transfer, mistyped0-test invocation and standalone output-directory setup error are retained as failed preparation steps. Their corrected controls passed without a compiler/golden change.

## Native execution scope

At this proof commit, native execution is NOT_RUN. The manual `qa-json-body-diagnostic` selector runs exactly these two cases with existing45-minute per-job bounds and fail-fast disabled. An observation may show that the type5 candidate is omitted, empty or otherwise unsupported. Such an outcome must remain visible and must not relax the compiler's current null/Optional rejection or close T05/L01/179. Any later native result must retain its exact run/ref/source and original artifacts.
