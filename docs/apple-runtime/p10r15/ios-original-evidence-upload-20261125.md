# iOS original evidence upload repair

The iOS collector already emits two joined-editor originals and two preview
originals referenced by its six dedicated native tier records. The workflow
upload list omitted these four files, so a green native test run could still
lose the original proofs required by canonical intake.

The upload now adds exactly `joined-editor-contract.json`,
`joined-editor-proof.json`, `preview-document-contract.json` and
`preview-native-proof.json`. The existing six file paths remain unchanged.
No wildcard, raw xcresult or private artifact is added. The existing collector
test command remains and is followed by the new transport contract control.

The causal test runs the actual collector with its explicitly synthetic joined
and preview log controls, verifies every emitted artifact hash, and compares
their names with the actual workflow upload list. Before the workflow repair it
failed with precisely the four missing names. The same unchanged test bytes
passed afterward: three parent tests, including four omitted-file controls and
three raw/wildcard scope controls. These are offline transport checks, not
native acceptance.

Original RED and GREEN receipts are retained under:
`/mnt/development/projects/evidence/cherri-p10r15-apple-completion-20261125/ios-evidence-upload-repair-01/`.

GREEN receipt SHA-256:
`962299f47040994b72f41ee0a0a1bf9da32676e1b91dea07b5a4c977d4632e3c`.
The unchanged causal test SHA-256 is
`be37e62e4fed36bc68a0d90875dc14cbc4359a79b8f4639e5abfef2a77005954`.

A successful full iOS run on the integrated candidate and original artifact
transport/intake remain required.
