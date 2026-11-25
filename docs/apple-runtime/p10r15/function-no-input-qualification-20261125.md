# No-input receiver component qualification — 2026-11-25

All entries below retain their actual source identity. They are component
qualifications; none executes Apple Shortcuts or grants a native/formal tier.

| Producer/source | Actual result | Scope |
| --- | --- | --- |
| Tagged policy `61d39c69` | Causal failure at control 27 | Original receiver hides the new tagged prefix. |
| Tagged policy `de63c9b4` | 38/38 PASS | Same exact controls and generated policy apart from one prefix line. |
| No-input callsite `2e4b3ece` | 129 controls, 17 FAIL | Original actual trigger/reset methods with explicitly local UIKit doubles. |
| No-input callsite `8da0bec6` | 129/129 PASS | Same policy, vectors and test template bytes; intent precedes the measured call, no fallback/retry, reset does not reopen the gate. |
| Source inventory `a571f196` | 1 parent, 12 physical tamper controls PASS | Actual Git-source guard rejects changed helper, policy and verifier bytes. |
| Sanitizer `a571f196` | Timeout at the original 30-second executable limit | All 50 originals retained; no assertion or deadline was removed. |
| Sanitizer `0a3b0751` | 81/81 PASS | Exact original 50 inputs plus 31 privacy-boundary and after-cap controls. |

The original no-input 129 controls have identical names and input bytes in
RED and GREEN. The reset body is byte-identical; the copied trigger changes
only through its reviewed gate wiring. The executable's UIApplication and
UIWindowScene are test doubles, so the result does not claim real UIKit,
Simulator, signing, import or runtime execution.

The source guard adds three new policy/verification dependencies to its prior
nine tamper controls. The current scanner repair does not alter that guard.
The actual original 50 sanitizer control bytes are reconstructed exactly
from the first 50 new controls and compared against the retained timed-out
input. The added cases check maximal local-part boundaries, Unicode neighbors,
nonemail strings and secret detection after 16 KiB. All prior secret/PII
alternatives and the output cap remain.

## Original evidence

Oracle evidence root:
`/mnt/development/projects/evidence/cherri-p10r15-apple-completion-20261125/`

- `tagged-receiver-policy-01/original-intake-01/intake.original.json` — SHA256 `81de82e67e6b969858d0d36b12f898b44e47a336c2df0f8f470c65fbbf7298a7`; 65 originals, including the earlier NOT_RUN capacity and SDK setup failure.
- `function-no-input-policy-01/original-intake-01/intake.original.json` — SHA256 `8ae1d5fc9f76b5798b5bd96ecb08c548cabcd8d63311955093fc18600986851b`; 185 byte-verified originals (2,119,338 bytes), archive SHA256 `7e0da7f2257febe990b384a317c4479502e67c93aaecf0b4de356181e22ba1ac`.
- `function-no-input-policy-01/source-guard-01/result.original.json` — SHA256 `6c723c64b3dd063bd1c726d96ebe35ed54d54b0120930c2cab7de3d130194c3a`.

All Mac originals remain in place. The explicit Xcode SDKROOT correction belongs
to the bounded qualification owner; it neither modifies test assertions nor
substitutes an old binary for newly compiled source.

A standalone receiver/harness build, the source-bound signed/imported function
graph, exact first-and-only no-input launch and independent callback/trace
intake remain separate operations. The final integrated application suite
and final acceptance run are still required.
