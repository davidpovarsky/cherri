# D03 collector causal evidence

Implementation: e48a9430c6285c89ba785fe85f2437e41153656e.
Baseline collector: d37b4336d82fdb103cfeb46518c6ee334fd0f014.

The baseline synthetic three-test log lost both DocumentationIndexTests blocks
and the raw documentation receipt, leaving no D03 integration record.
The revised collector requires all three exact passed tests, their unique raw
receipt and UI checkpoint, successful process/source identity, and all five
source-bound artifacts.

The independent review baseline preserves two actual candidate credit gaps
(missing generation stdout and app-invalid language grammar), plus a direct
helper wrong-target gap that the outer collector already rejected. The final
implementation rejects all three. Those observations are candidate review
evidence, not failures of a released app.

The exact clean implementation replay passed 45 local Python test methods,
with no failed or skipped tests. These include process-level CLI collection,
negative boundaries, artifact digest reconciliation, private-field exclusion,
and the existing collector controls. The supplied public 562-section index
also passed the helper's schema/manifest binding check with its original bytes.

Every XCTest log in this directory is intentionally synthetic. No Swift,
iOS app, Simulator, Shortcuts, website, or installed Skill success follows from
this archive. Actual source-bound native results remain separate requirements.
The collector's D03 app record also does not close the website/Skill obligations.
