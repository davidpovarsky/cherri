# Optional compiler macro compatibility validation

Observed 2026-11-18. Scope: actual compiler capability output and canonical generated documentation, with isolated single-field faults. This is consumer compatibility evidence, not Apple runtime or a release promotion.

## Result

- Candidate shell validation: 22/22 checks passed in Sentinel job job_2842a7809f49 (11.98 s).
- Unchanged source controls: 12 positive checks passed and all 9 injected macro drift states were incorrectly accepted, as expected for the pre-fix baseline. Job job_d48e4df22762 returned 1 (8.08 s). The retained baseline report explicitly records those failures.
- Existing complete Skill wrapper self-test and its ten pinning/offline/atomic-update scenarios passed with an isolated Skill copy that declares the new macro contract. Job job_603136b59c64 (10.56 s).
- Shell syntax, Python syntax and diff checks passed. CI now generates documentation from the checked-out compiler, runs the same audit against its actual binary and uploads the evidence even on failure.

The real candidate compiler is ab9a264ba370890397d74042e00c11a3b4c84ffd. Its 13 compiler macros have fingerprint f63d8b0a323c7045af4abf20723684506c9f63b9d31776a408c39766e7c2cf44. The backward compatibility control uses the actual d00bc0b9e9a4d8c460623de3d883680b94289e04 compiler and canonical d00 documentation. Binary and documentation hashes are recorded in the JSON receipt.

## Contract

A manifest without macroFingerprint retains the older compatibility contract. Once the field is present, it must contain 64 hexadecimal characters; compiler capabilities and generated documentation must expose the same value. Missing, malformed and mismatched values fail with distinct actionable diagnostics. The native action schema and codec checks remain independent.

The audit first proves the real healthy tuple and the healthy capabilities-only proxy. It mutates only one macro field at a time in the compiler capability response, manifest or documentation. Every mutation is followed by a restored positive control. It requires the intended diagnostic, so a missing compiler cannot count as a successful drift rejection. The proxy delegates every command except capabilities to the real compiler. Nothing is installed live.

## Reproduction

Use tools/p10r15/verify_macro_compatibility.py with --compiler, --compiler-commit, --skill, --docs-reference, --expected-macro-fingerprint and --report. The optional --legacy-compiler, --legacy-compiler-commit and --legacy-docs-reference add a real old-binary control; without them CI runs 21 checks.

For the wrapper self-test, use an isolated Skill copy with the actual compiler pin and macro fingerprint in its compatibility-manifest.json, then run scripts/self-test.sh --skip-setup --evidence-out PATH with CHERRI_BIN set to that binary. Do not globally override CHERRI_REF or CHERRI_MANIFEST_FILE during this self-test: its internal pinning scenarios deliberately create independent manifests and commits.

Two setup probes demonstrated that protection: the source manifest correctly rejected the ab9 sidecar when still pinned to 1204e1f7, and a global manifest override correctly conflicted with the scenario's temporary pinned compiler. The final run used the isolated copy without either override and passed.

## Files

- macro-compatibility-isolation.json: all 22 candidate checks.
- macro-compatibility-baseline.json: expected 9 causal failures against unchanged source.
- macro-skill-self-test.json: complete wrapper scenario evidence.

Final consumer pinning and promotion require the final frozen compiler and docs commits; these checks do not promote the earlier validation candidate.
