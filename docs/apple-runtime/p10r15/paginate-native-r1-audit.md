# P15 paginate native R1: retained runtime failures

Both exact-source pilots imported the signed target and produced a fresh correlated SUCCESS callback, but neither callback contained resultSummary, resultShape or tracePayload. The ordinary oracle correctly retained FAIL_RUNTIME for both the result and trace. No native pagination semantics are accepted by this evidence.

| Run | Fixture | Native actions | Published hashes | Result / trace |
|---|---|---:|---:|---|
| [44322656619](https://github.com/davidpovarsky/cherri/actions/runs/44322656619) | qa-paginate-ordered-v2 | 290 | 20/20 verified | FAIL_RUNTIME / FAIL_RUNTIME |
| [44322666393](https://github.com/davidpovarsky/cherri/actions/runs/44322666393) | qa-paginate-cycle-v2 | 214 | 20/20 verified | FAIL_RUNTIME / FAIL_RUNTIME |

Source: 35a6b6006d202cb08883d6f809c696e3faec5ce3. Source bytes, compiler manifest, unsigned hash, signing provenance and fresh nonce/fixture identity were cross-checked. The diagnostic is `callback result/trace must be text`. No callback error or specific guard result was observed; Alert/Stop, HTTP conversion and other early-exit causes remain hypotheses.

The pages are immutable public GitHub raw files at38c7098772f4b822d6a3555137c0c7cf8e5b1e5e. They do not expose nonce-bound server counters. Successful six-page preflight fetches establish published data bytes, not runtime GET count/order/status. The preflight file remains quarantined by the existing publication rules.

AX persistence retained8175/8221 nodes, but Inspector remains NEEDS_REVIEW and physical-device execution is not done. No raster or original private corpus data was published. The complete bounded audit with20file hashes per run is [paginate-native-r1-audit.json](paginate-native-r1-audit.json).

Next causal boundaries are a minimal native GET/Dictionary control, independent type/count/key witnesses, then the original ordered/cycle/budget scenarios. The full fixtures and failed observations remain unchanged; no success marker or trace exception has been substituted.

## Bounded follow-up: public response content type

A subsequent host GET of the exact immutable GitHub raw `one.json` URL returned HTTP 200 without a redirect, `Content-Type: text/plain; charset=utf-8`, and all 335 bytes matched the committed synthetic page (SHA-256 `25baf7da5af19f5f23a4790762c82502b39e90a17d0626e53e930b09d94efac1`). This is not an observation of the earlier iPad response. Independent Apple action metadata declares `downloadurl` output as `public.data`; `detect.dictionary` is a separate coercion action. The native response type is still unobserved. A direct GET/type/conversion diagnostic is justified before changing the macro; no MIME/conversion cause has been proved and both original FAIL_RUNTIME outcomes remain retained.
