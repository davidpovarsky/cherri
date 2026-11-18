# Catalog evidence output isolation

The full Windows Go suite at immutable `800270786c5d02c7fd1f9a36ed1ac6d44d64ec3c`
changed the tracked parameter-contract report while acceptance tests inspected
source cleanliness. The suite also reached its package-wide 40-minute timeout
during packaging; it is not recorded as a complete pass. Linux full-Go passed
at that source, but its later Legacy parity stage failed independently.

Both existing 505-action producers now keep their ordinary JSON output in Go's
test-owned temporary directories. All execution, round-trip, parameter and
negative assertions remain unchanged. A caller collecting durable evidence
sets `CHERRI_CATALOG_EVIDENCE_DIR` to the intended directory. Every write error
is reported rather than ignored outside a Git checkout.

For explicit regeneration of the historical ledgers, select
`docs/apple-runtime/p10` as that directory and run the named producers. Prefer
an external evidence directory during source-bound acceptance runs, then review
and commit the resulting reports separately. The existing ledger generators
continue reading their established committed paths.

```sh
CHERRI_CATALOG_EVIDENCE_DIR=/absolute/evidence/dir go test -count=1 -json \
  -run '^(TestP10_VerifyAndRecord505CatalogExecution|TestP10_ParameterContractMatrix_AllCatalogActions)$' .
```

The validation runs the real producers in both default and explicit-export
modes, compares the tracked reports before and after, checks both exported
505-entry JSON reports, and retains raw command receipts outside the checkout.
This output repair provides no Apple runtime or overall release assertion.

## Acceptance CLI report destinations

An explicit --out now keeps the generated requirements map beside the resolved
report file instead of also overwriting the tracked historical map. The default
local path and explicit --build-evidence-map command remain available.
A failed map build or write now reports an error and exits unsuccessfully.

The actual CLI control first reproduced a hidden mapping-write failure by
placing a directory at the output filename. After repair it reports the cause
and preserves that directory. A normal external output produces all 179 map
rows without changing the tracked map. Both controls use an empty final
manifest, which correctly remains incomplete and unsuccessful; they provide
output-integrity evidence, not acceptance of any language requirement.
