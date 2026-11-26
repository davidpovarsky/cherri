# Native iOS diagnostic retention: Preview transaction tests

The original full iOS run [45780580372](https://github.com/davidpovarsky/cherri/actions/runs/45780580372), attempt 1, executed source `1cafc8cea0950edc182b1a0656c7db11a975df47` and exited 65. Its authenticated xcresult summary reports **89 tests: 86 passed, 3 failed, 0 skipped**. The existing public diagnostic parser retained 78 named outcomes: 75 passed and the same 3 failures.

The parser's fixed class list omitted `CherriPreviewEditTransactionTests`, which has eleven original methods. This change adds that exact hosted-app class to diagnostic case retention. Its failure location is retained only for the exact `CherriTests` bundle/class and active case. No requirement-to-test or acceptance mapping changes.

## Actual qualification

Three new collector controls were added before the implementation. The original parser failed the all-eleven outcome and failure-location controls; the wrong-bundle/class/active-case rejection control passed. The unchanged three controls then passed with the narrow implementation, alongside two existing controls for E01 acceptance and failed-run retention.

- RED receipt: `preview-transaction-retention-red-01/result.original.json`, SHA256 `4e5755cd54ec967b0e50d3256501616e212a57aa01d92027e0be1523f5abebbb`.
- GREEN receipt: `preview-transaction-retention-green-01/result.original.json`, SHA256 `f0481fe146983203e79a7201b8e4c4dfd2db2d0deda7efccbd72f4f186a3f39c`.
- Evidence root: `/var/cache/cherri-e07-pane-transition-20261126`.
- The native transaction source is unchanged, SHA256 `01bfaf1fdd7d39e6f02706e59456340e89613e0e37957eb6325172d3a59a635f`.
- Independent original xcresult verification: `xcresult-private-intake-originals-01/intake-result.original.json`, SHA256 `8d1dcea1a822c095b7154c639a8043be90607390d193dd66a3f9da6d71ca5731`.

These five Python passes qualify diagnostic retention only. They do not rerun the native suite, reconstruct the eleven discarded native log lines, or grant acceptance to the three failed tests. The old 78-row diagnostic and the broader authenticated 89-test aggregate remain separate originals.

The verified failure messages identify E07 startup readiness, BRG25 Sign availability after malformed-source Build rejection, and BRG28 the first pair of WebKit Text-field lookups after a successful Build. No source-data corruption has been established by these observations.
