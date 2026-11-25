# Fork Legacy native differential preparation

`generate_legacy_differential_manifest.py` prepares the 37 native-eligible
Legacy cases from `tests/differential_30/manifest.json`. It selects exactly
01–37, checks their pure runtime contract, and changes only the first source
name directive to `CHERRI_QA_D30_L_NN`. Every other source byte is preserved.
The final real compiler runs in explicit Legacy mode. The tool never writes,
repairs, normalizes or renames a plist, and never signs or executes a Shortcut.

## Compile the final binary

Use the full source commit and binary SHA-256 from the separate build receipt:

```sh
python3 tools/shortcuts-simulator-qa/generate_legacy_differential_manifest.py compile \
  --compiler /absolute/path/to/final-cherri \
  --compiler-sha256 BINARY_SHA256 \
  --compiler-commit FULL_SOURCE_SHA \
  --out /absolute/path/to/new-legacy-preparation
```

`--manifest` can select an explicit suite manifest. `--derive-uuids` is opt-in;
ordinary UUID generation is the default, matching the principal native lane.
The output directory must be new. Compilation failures stay in the receipt and
cause nonzero exit; no cases are silently skipped. The compiler hash is checked
before and after compilation. The source SHA is explicitly declared provenance
from the caller's build receipt, not inferred from binary bytes.

Outputs include the exact suite manifest, original and renamed sources, full
compiler logs, untouched compiler artifacts, and `compilation-receipt.json`.
The receipt records source, log, binary and artifact hashes, actual commands,
exit codes, and `NOT_RUN` runtime status. A receipt marked `COMPILED` is a build
claim only. Its SHA-256 is printed for the finalization command.

## Sign through the existing owner-controlled signer

The coordinator signs `artifacts/CHERRI_QA_D30_L_NN.shortcut` using the existing
Cherri `sign` command. Each signed directory entry must retain the name
`CHERRI_QA_D30_L_NN.shortcut`, with its actual existing signer manifest named
`CHERRI_QA_D30_L_NN.signing.json`. The generator does not call the signer.

## Bind signed bytes into the native runner manifest

The preparation directory may be copied intact to the Mac; paths inside the
compilation receipt are relative to that directory.

```sh
python3 tools/shortcuts-simulator-qa/generate_legacy_differential_manifest.py finalize \
  --out /absolute/path/to/legacy-preparation \
  --compilation-sha256 COMPILATION_RECEIPT_SHA256 \
  --signed-dir /absolute/path/to/signed-legacy
```

Finalization rechecks the receipt, suite, exact source transformation, compiler
logs and unsigned artifacts. It uses the existing Cherri signer fields
`unsignedPlistSha256`, `signedArtifactSha256`, byte sizes, names and action count.
Optional signer source/commit hashes are checked when present. Every signed
file must match its signer hash and have an AEA1 header. These are exact-byte,
signer-record and header checks; they are not independent cryptographic
signature verification or evidence of successful import/execution.

The resulting `native-manifest.json` has the existing
`run_differential_batch.py` case fields: `id`, `lane`, `name`, `signedPath`,
`signedSHA256`, `unsignedSHA256`, exact `expected`, and `input` when present.
Additional provenance fields do not change those runtime inputs. Finalization
refuses incomplete case sets, changed expectations, hash mismatches and an
existing native manifest. Cases 38/39 are never eligible for this preparation.

No comparison taxonomy changes are made. All strict typed differences remain
in the differential suite evidence. In particular, an explicit integer-zero
`PropertyUserInfo` field versus an absent field has unknown native impact;
this generator grants it no normalization or equivalence exception.
