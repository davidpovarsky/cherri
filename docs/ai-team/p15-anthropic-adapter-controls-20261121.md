# Anthropic provider adapter: offline source-bound controls

Source: 9487d04f45798ca68b6030bf8f3d18af2469c0ec, based on cd0bade0801aa2832e38b3dc4d4090c8dd857301.

The exact clean source passed all 55 fixed offline controls on 21 November 2026, 22:04:37–22:04:41 UTC. The actual Python subprocess exited 0 in 3.359 seconds under its unchanged 60-second test envelope. Git was clean before and after; all four adapter/test/runner/documentation source blobs matched Git and retained the same SHA256. The test inventory comes from source AST and is reconciled exactly against the 55 original terminal ok events.

The retained [original evidence archive](evidence/20261121-ai-provider-controls/originals.zip) preserves the receipt, full stderr/stdout and fixed inventory byte for byte, including Windows CRLF. Its [bindings](evidence/20261121-ai-provider-controls/bindings.json) record the archive and all four member hashes. Receipt SHA256: a02dc6f218e44556c860bb48485d593ffb0c304bb11a80669e09ae317af35c16. Original test output SHA256: ffd191067a0e027efb4d332341a56dd2b9b02b85ba8fef9fca5893e41ccda962.

The adapter enforces the anchored public request, pinned model/settings/code, fixed HTTPS endpoint, single count plus at most one generation, exclusive zero-retry claim, original source preservation, genuine request IDs, all input and cache accounting, actual usage caps, retained refusals/truncations and safe error receipts. Mock controls also cover unapproved input, changed code/caps, redirects, malformed responses, no retries after cancellation/success, bounded reads and reflected-credential withholding.

These were mock transport controls. No actual provider request, token-count request, compiler invocation, paid generation, semantic validation or Apple execution occurred. The fake bundle in these tests exercises the adapter interface; integration with the independently implemented bundle/runner/intake still requires review. The adapter's HTTP Date is only a server receipt. AI01 quality and final 179-requirement acceptance remain unproved.
