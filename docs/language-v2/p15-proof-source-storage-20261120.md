# Keep proof source snapshots out of executable Go discovery

The actual full Go diagnostic at fd691b96fb661697a1cbaddab92476d457636ba9 discovered the evidence/source directory below p15-optional-native-diagnostic-20261120 as an extra Go package. Its copied test snippets could not build because they intentionally contained only the original proof subset, not a complete compiler package. The original undefined-symbol output is retained unchanged in the companion evidence.

The two evidence copies now have a .go.txt suffix, as the neighboring proof snapshots already do. Their exact bytes and SHA256 values are unchanged and were independently compared with the immutable fd691b96 Git blobs. Active root production/test files were not renamed or excluded. No test selector, build tag, go.mod, assertion or CI gate was changed.

The original manifest is retained byte-for-byte as manifest-before-storage-repair.json. Its current successor records the two storage-name changes and the original manifest hash while retaining the original component source and outcomes. Every listed evidence member was reverified.

Actual go list ./... changed from31 to30 packages: only the nonexecutable evidence snapshot package disappeared; all30 real packages remain. This read-only discovery verification occurred with the unrelated M02 draft present and unchanged; it does not claim a clean full-suite pass. The existing frozen-source full run remains failed. Final clean full Go/CI must run on the delivered source.

See [repair receipt](evidence/p15-proof-source-storage-20261120/repair.json), [before inventory](evidence/p15-proof-source-storage-20261120/go-list-before.txt), [after inventory](evidence/p15-proof-source-storage-20261120/go-list-after.txt), and [original compile error](evidence/p15-proof-source-storage-20261120/original-full-go-compile.stderr).
