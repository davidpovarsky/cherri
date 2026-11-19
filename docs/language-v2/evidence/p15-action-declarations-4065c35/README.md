# Public custom action declaration validation

Verified source: `4065c3549b5fdde2e0a1aa94f2bcae2f7f4b1a05`.

The exact original b34 compiler incorrectly reported `valid:true` and emitted native artifacts for an unknown explicit codec and a binding whose source parameter did not exist. The causal tests-only baseline at ddc6c02 reproduced nine malformed declarations through actual CLI check/compile, embedded/mobile compilation and editor service, including unintended signing requests. Its healthy control passed: eleven named outcomes, one pass and ten failure events including the parent.

The repair validates declarations before registering their local schema: existing codec identity, declared binding/primary, nonempty and distinct wire keys, and unique parameter names. The parser rejects duplicate binding entries/properties before map overwrite. Existing implicit mappings and omitted codec selection remain covered by healthy controls. Codec identifiers are centralized with the generator; no second action catalog or serializer was introduced.

Clean source validation passed 82 named root cases and 80 named affected-package cases, with zero failures or skips and a clean source tree before/after. The root scope includes exact diagnostics, no emitted artifact, unchanged prior destination and zero signer calls, plus existing custom/enum/registry regressions. Packages cover analysis, backend, syntax, service and the actual schema generator. This is local source verification, not Apple runtime, full179 or release closure.

A development orchestration string replacement initially wrote the candidate summary using the baseline log path. That invalid summary is retained in the raw ZIP. Both original raw JSON logs were intact and their corrected labeled summaries were reconstructed explicitly. The final clean driver uses separate explicit log paths and its recorded hashes/counts were independently checked; no test outcome was manufactured or suppressed.

The ZIP preserves original bytes. Readable JSON copies use repository line endings. Raw members and all archive files are SHA256 bound.
