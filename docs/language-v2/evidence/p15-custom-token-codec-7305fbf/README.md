# Public custom token codec repair

Verified source: `7305fbf8774738bd8c18394ee69a21bdddd49d66`.

The actual compiler previously emitted a bare scalar for an explicitly token-coded public custom parameter. Canonical positional token arguments materialized a native producer and attachment, while canonical named arguments also bypassed that materialization. The fix shares the existing scalar producer and reference path between canonical argument spellings and explicit custom token bindings. Accepted literal forms, existing references, non-token values, defaults and source-order snapshots keep their established paths; no new serializer was introduced.

Clean verification executed 131 named root checks and 28 lower/backend package checks, with zero failures/skips and clean source before/after. The root group covers public CLI/mobile output, positional/named/default/secondary values, false/zero/empty/negative values, exact native producer edges, deliberate wire corruption, shared-codec mutation/restoration, call order, registry isolation and canonical differential compatibility.

The corrected tests-only baseline at a432646 had 11 failure events and 4 healthy passes. The first baseline additionally contained one overly strict test assumption about an ordinary text producer's absent CustomOutputName; the corrected oracle requires its actual default display name and exact UUID edge. Both original baseline logs are preserved. The negative-number case was added for candidate verification. The working candidate passed16 named checks before the clean159-check verification.

The archive also retains the initial public synthetic CLI diagnosis using the previously stamped b34ae47 binary. Those probes correctly established prior-source behavior; they are not relabeled as current-source proof. All content is synthetic and public. No private corpus data is included.

This source-bound proof does not grant AC02 full acceptance by itself, Apple runtime execution, or release closure. Formal complete custom-action codec-family/schema evidence is a separate task.

Archive SHA256: `45967163daad63d9b380b0cd4e584e2e7027f978f4256744e23fbd8d0f1fdc53`.
