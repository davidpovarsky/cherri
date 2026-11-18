# Explicit Number coercion: clean local proof

Source 087f0c34a3a05be6d7720e42e27fce6d5c0d5b54 was built in a clean normal clone with matching VCS revision and vcs.modified=false. All five recorded steps passed, including 59 actual CLI fixtures, 12 graph corruptions, three release-body mutations and five publication mutations. This is local proof only. Apple runtime and signing are NOT_RUN in this receipt.

The unchanged original number-comparison fixture retains its observed Apple failure from run 44386191992. The new synthetic control changes four forwarded/bound comparison operands to the existing explicit Number coercion. No compiler emission or original expected output changes. Only a new signed native run can establish whether this intervention changes the result.

Original log bytes and their hashes remain on the authorized host. Explicitly named normalized derivatives have independent hashes in archive-index.json.
