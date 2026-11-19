# Docs release evidence: compiler 71225b / docs 7f4c65d

Compiler source: `71225b71f5353a0f661722d154a8b670b52e76f6`. Docs source: `7f4c65d274548cb2c563626781de2a07a456638e`. Both were clean during the recorded validation.

The complete generated action reference was regenerated from the exact compiler and matched the staged body byte-for-byte after its existing Jekyll frontmatter. Native schema fingerprint is `7bfff2b8c3fddf3892e943ef45b79bbbc4cc984ad051a15099a5da0fc6fccc0b`; codec is `3e7e3555eefe1e030d254df0a22c62fdc3a139f92e39da34f6d841f8bfa1689e`; macro fingerprint is `1fe7f7993e2473aec591341712f42c521f913974b8e84091a5c2950232230dec`.

Local Windows validation passed all 37 executable/diagnostic snippet checks and all 12 real compiler cache controls. Four explicitly historical migration fragments and 520 generated signature declarations remain reference-only. They are not counted as executable tests. The normal compiler clone and built binary have exact immutable VCS provenance; no existing installed compiler was replaced.

[GitHub validation run](https://github.com/davidpovarsky/cherrilang.org/actions/runs/44435674614) succeeded for both snippet/cache and Jekyll site jobs at the same docs SHA. The frozen Gemfile.lock was built with Ruby 3.3 and Bundler 2.5.11. The site verifier checked the actual rendered page and generated JSON search index: 505 native entries, 520 signatures, 415 search entries, 21 action-reference search sections, both canonical fingerprints and a clean source checkout. The 7.3 MB built site remains in the CI artifact; compact receipts and raw test logs are archived here.

This verifies the source/build tuple. Site publication, Skill activation, Apple runtime and physical-device acceptance are separate operations and are not claimed by these receipts. The original Authoring Patterns ZIP and INSTALL-AGENT.md remain missing from the bounded discovery recorded in the project handoff.
