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
