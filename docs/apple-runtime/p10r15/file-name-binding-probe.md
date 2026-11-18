# File name binding diagnostic

Native run [44346951190](https://github.com/davidpovarsky/cherri/actions/runs/44346951190), on exact source4e7956d2519307861fe703a11fbe401fc9faca05, returned `CHERRI_QA_EXPORT_FILE_BYTES:NAME=bad:TEXT=ok`. The independent audit rehashed20/20 published files and replayed the unchanged strict oracle. This remains a runtime failure. The observation alone does not identify Set Name, Get Name, output naming, or File conversion as its cause.

The new explicit sample `qa-file-name-binding` uses the existing signed Simulator QA workflow and four independent copies of the same reviewed Base64 literal. It compares:

- A named immutable Set Name result consumed by Get Name and Get Text.
- An unnamed Set Name result stored in a mutable variable and read through that variable.
- Direct nested Set Name calls consumed by Get Name and Get Text, each with its own independent decoded input. These results are captured into mutable variables so an enclosing immutable binding does not name an intermediate producer.

Every Set Name requests exactly `cherri-export-control.json`. The expected text is exactly the fixed UTF-8 content `{"ok":true}`. The strict required result is:

```text
CHERRI_QA_FILE_NAME_BINDING:L=ok:V=ok:I=ok:LT=ok:VT=ok:IT=ok
```

The three name fields report only bounded classifications: `ok`, `binding`, `source`, `stem`, `double`, or `other`. These classify matches against constants in the reviewed source; no arbitrary native name or file path is published. Only `ok` satisfies the oracle. Three independent text flags also must pass. The original failing fixture and its expected result remain byte-identical.

An early local draft used immutable bindings around nested calls. Actual emitted graphs showed that the nested Set Name also acquired the enclosing output name, so the graph validator rejected that draft. The corrected diagnostic explicitly uses mutable outer bindings for the direct-call controls. This changes the diagnostic design, not the production compiler.

The Go test compiles the real source and checks native attachment shape, four distinct decoded producers, exact names, the immutable output name, absence of names on the other Set Name producers, the single mutable store, and all six readback edges. Nine independently altered graphs are rejected. Existing Python dispatch tests verify the exact allowlisted selector, source, expected result, fresh callback identity and mandatory Debug trace; each of the six flags is independently corrupted and rejected. Shell syntax is checked.

The fixture digest and narrow scope are recorded in `file-name-binding-probe.json`. Local structure and harness tests do not establish Apple runtime behavior. A new exact-source signed run is required.
