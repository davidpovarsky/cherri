# Upstream declaration boundary — P10R15

## Enforced scope

All25 action declaration files remain Legacy build-time inputs. They are read by the catalog/schema adapter and are not converted to the public v2 language. The preserved stdfunc.cherri source is also included. Existing fork additions differ from the historical upstream snapshot; they must not be mislabeled as unchanged upstream bytes.

The exact machine-readable boundary is [upstream-preservation-boundary.json](upstream-preservation-boundary.json). It records byte lengths and SHA-256 values for every file at four immutable revisions:

| Role | Exact revision |
| --- | --- |
| upstream | `a66db15b7f247f3121726c2a2b72becfeef3d96b` |
| language | `559abceadda1dbf0cd7d7deb1a26315b8ac44c3f` |
| continuation | `d7052f4568db26dba169bdefc72623e48c04968a` |
| recorded_source | `8e66254d56ea3e4ac70f6cda6ef9d31217e0937c` |

The continuation base is the exact integration commit supplied for this task. Of the26 files,24 are byte-identical to that base. Only the two corrections below differ, each by one source parameter-label token. No file is added or removed, and no other byte change is allowed by this boundary.

6 of26 files are byte-identical to the recorded historical upstream revision; 6 are byte-identical to the original language-project baseline. These are distinct comparisons. The JSON manifest contains all full digests, including inherited fork differences.

## Reviewed narrow corrections

| File | Existing native key | Original source label | Corrected source label | Reason |
| --- | --- | --- | --- | --- |
| actions/device.cherri | endTime | startTime | endTime | Remove a duplicate label while preserving the end-time parameter binding. |
| actions/location.cherri | WFAddressLine1 | addressLine2 | addressLine1 | Remove a duplicate label while preserving the first address-line binding. |

The emitted native keys, types, optionality, parameter order and remaining declaration bytes are unchanged. These are semantic source-identity corrections, not a translation of declaration syntax. Their implementation commits are recorded per file in the manifest.

## Per-file classification

| Input | Same as historical upstream | Same as language baseline | Continuation change |
| --- | --- | --- | --- |
| actions/a11y.cherri | yes | yes | unchanged |
| actions/basic.cherri | no: inherited fork difference | no | unchanged |
| actions/calendar.cherri | no: inherited fork difference | no | unchanged |
| actions/contacts.cherri | no: inherited fork difference | no | unchanged |
| actions/crypto.cherri | no: inherited fork difference | no | unchanged |
| actions/device.cherri | no: inherited fork difference | no | parameter-label-correction |
| actions/documents.cherri | no: inherited fork difference | no | unchanged |
| actions/dropbox.cherri | yes | yes | unchanged |
| actions/images.cherri | no: inherited fork difference | no | unchanged |
| actions/intelligence.cherri | yes | yes | unchanged |
| actions/location.cherri | no: inherited fork difference | no | parameter-label-correction |
| actions/mac.cherri | no: inherited fork difference | no | unchanged |
| actions/math.cherri | no: inherited fork difference | no | unchanged |
| actions/media.cherri | no: inherited fork difference | no | unchanged |
| actions/music.cherri | no: inherited fork difference | no | unchanged |
| actions/network.cherri | no: inherited fork difference | no | unchanged |
| actions/pdf.cherri | no: inherited fork difference | no | unchanged |
| actions/photos.cherri | no: inherited fork difference | no | unchanged |
| actions/settings.cherri | no: inherited fork difference | no | unchanged |
| actions/sharing.cherri | no: inherited fork difference | no | unchanged |
| actions/shortcuts.cherri | no: inherited fork difference | no | unchanged |
| actions/storage.cherri | yes | yes | unchanged |
| actions/text.cherri | no: inherited fork difference | no | unchanged |
| actions/translation.cherri | yes | yes | unchanged |
| actions/web.cherri | no: inherited fork difference | no | unchanged |
| stdfunc.cherri | yes | yes | unchanged |

## Build-time and public-language boundaries

The live action catalog and language schema generator remain the source ingestion path. Public-language parsing is separate; preserving these declaration inputs does not authorize a hidden Legacy user-program fallback. The standard-function source remains unchanged and is migrated through its existing build-time module path. U03 generation propagation, approved frontend behavior and stdfunc semantic acceptance have their own producers; this declaration boundary awards none of those rows.

## Verification and future changes

Repository verification must read the actual immutable Git blobs and the current tracked inventory, reconcile every full digest, and compare each allowed correction against the original byte sequence. Merely checking that this document or two action files exist is insufficient. Corruptions of the file inventory, baseline identities, preserved bytes or allowed correction boundaries must be rejected.

Any future intentional declaration change requires a reviewed update of this explicit boundary and fresh execution; the verifier must not infer an allowlist from its current working tree. The original contracts remain unchanged. This document replaces the old planning table that called all files upstream-unchanged; prior revisions remain in Git history.

Repository boundary proof does not establish interrupted-worktree preservation, privacy/signing, Apple execution, consumer compatibility or final delivery closure.
