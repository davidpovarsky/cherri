# Native evidence and final artifact bindings

These tools turn preserved compiler, signing, import, callback and receiver-image receipts into reviewable evidence. They do not compile, sign, import or execute a shortcut.

## Two separate reports

`native_ledger.py` preserves every historical observation and selects the latest semantic observation under the current independent expected value. Earlier import/runtime failures remain in history. A run that observed zero cases remains in the run inventory, with its fatal condition and receiver-image status. The case35 correction does not alter historical expected values or verdicts.

`native_bindings.py` compares each selected native unsigned artifact with the exact output of a pinned final compiler. It verifies source bytes, binary bytes, creation commands, recorded exits, artifact hashes and the typed native graph. It writes all raw and UUID-alpha differences; it does not replace them with an equality label.

A result is carried to the final program only when:

- The selected callback, manifest, import and completion receipts are correlated, and the signed file hash is verified.
- The native compiler/source/artifact lineage is verified.
- The current independent expected value agrees with the native run and the final compile manifest.
- Source bytes are identical, or differ only in the single declared workflow name needed to import lanes together.
- Both graphs are valid and the action trees are equal under the fixed comparator's native identity bijection.
- Workflow metadata is exact, except an explicitly retained workflow-name change corresponding to the source declaration.

All checks are reported individually. An original compiler failure remains a failure when bound. A new artifact with different generated UUID bytes is not described as having been executed directly.

## Proof strength remains visible

The ledger distinguishes raw UTF-8 digest plus receiver-image proof, raw UTF-8 digest without a recorded per-run image, and the bounded older single-line summary/raw-length evidence. Older observations are never assigned a digest or receiver-image proof they did not record.

The fixed comparator recognizes only native producer, control-group and typed-reference identities. Ordinary literal keys, types, defaults, display names and property metadata remain exact. Graph equality is not a substitute for native expected-output evidence.

## Generation

Use a new output directory for each generation. Both tools reject an existing output directory.

```sh
python3 -B tests/differential_30/native_ledger.py \
  --config /absolute/path/ledger-config.json \
  --out /absolute/path/new-native-ledger

python3 -B tests/differential_30/native_bindings.py \
  --ledger /absolute/path/new-native-ledger/native-evidence-ledger.json \
  --config /absolute/path/binding-config.json \
  --out /absolute/path/new-final-bindings
```

The ledger configuration is illustrated by `native-ledger-config.example.json`. The final binding configuration is illustrated by `native-bindings-config.example.json`. A combined configuration may contain both sets of fields.

The binding tool requires the exact comparator hash, final fork revision, frozen-original revision, saved action catalog and actual final compile receipts. It resolves reused artifacts back through their original creation receipts and records the real command and exit, rather than presenting a copy operation as a compile. The earlier dirty statistics candidate explicitly retains its absent exit-code field.

## Exact selected artifact file list

`selected-native-artifact-files.json` lists each selected case/lane with:

- Native compiled name, actual import name and final compiled name.
- Mac signed and unsigned artifact paths and verified hashes.
- Native and final source files, unsigned outputs, compilation receipts, signing receipts and native proof receipts.
- Each file's host, size, hash, roles, case associations and hash-verification receipt.

Some early runtime manifests recorded no Mac unsigned path. Those paths are resolved only through a read-only exact hash match against the known compiler upload directories. The originally missing manifest value and the resolution method remain visible. A filename alias does not change the signed bytes or the compilation identity.

The tool creates a transfer list; it does not transfer files. It returns a nonzero exit if a selected semantic observation cannot bind or a required selected artifact/signing receipt is missing. Pending native cases remain pending and are not converted into failures or passes.

## Outputs

- `native-evidence-ledger.json`: complete observation history, zero-observation runs and current semantic selection.
- `native-evidence-compact.json` and table: concise current view with proof tiers.
- `artifact-command-lineage.json`: all final successful and failed compilations, real creation commands, exact hashes and current graph audits.
- `comparisons/*.json`: full raw and typed alpha differences for every selected native-to-final pair.
- `final-native-bindings.json`: each binding check and its evidence.
- `selected-native-artifact-files.json`: exact review/transfer manifest.
- `final-native-evidence-compact.json` and table: bound results, structural-only cases, compile blocks and native cases still pending.

These are source-bound component observations. They do not grant a full Go-suite or release acceptance verdict.
