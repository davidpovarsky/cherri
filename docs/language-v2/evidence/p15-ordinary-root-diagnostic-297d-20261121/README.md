# Historical ordinary-root diagnostic

Task 1220737847318371 required a bounded diagnosis on clean 297d5a657885ec2144f7366be1bcf721a0032a47, not a passing full suite or formal acceptance. The original actual inventory contains 1,089 unique parents. The exact committed BRG30 list excludes 15; the remaining 1,074-name partition was verified complete and disjoint.

One actual run executed go test -count=1 -json -timeout=15m for the root package, with ^Test selection and the anchored exact 15-name exclusion expression. Source remained clean and unchanged. Go reached its original 15-minute package deadline; process exit 1 after 941.703 seconds including compilation. The outer process guard did not fire.

Independent re-parsing of all 7,821 original events found 567 started parents, 566 terminal parents (560 PASS, 3 SKIP, 3 FAIL), one incomplete parent and 507 unstarted included names. No excluded test started. There were no unexpected, duplicate or orphan parents, foreign packages or malformed JSON lines. Every event timestamp falls inside the observed process interval. The full included/excluded/unstarted lists are retained in metadata-evidence.zip.

The actual historical failures were:

- TestP10_B1_B2_JsonRequest_DynamicAndLiteralContract_And_LocalEchoServer: E_TARGET_VALUE_SHAPE for optional absence in a dynamic JSON dictionary field; its child also recorded the parent-FailNow/Goexit failure.
- TestP15V02ImportedScopeIdentity: output, variable and opaque cases rejected native imported final Text against the stale main-fallthrough observer.
- TestP15CanonicalBehaviorSourcePropagation: rebuilt BRG24 baseline rejected the actual Base64 action plus bare Nothing cardinality.

The deadline interrupted TestP15BRG07DirectReferenceKindsRoundTrip. This interruption is not a separate asserted semantic failure. Three skipped parents and all 507 unstarted names remain explicitly recorded. No source, contract, fixture or assertion was repaired during this diagnostic, and no second attempt was made.

The bounded diagnostic definition of done is now supported by exact inventory/scope, actual process/raw evidence, independent event accounting and a precise remaining-failure report. Closure is handed back to the root owner. The test run failed. No current-source pass, fullGo, 179-requirement, Apple or consumer readiness is inferred.

Full byte-preserved originals, including the complete Go event stream, are sealed on the SentinelX host in the archive recorded in summary.json. The committed metadata archive contains its full hash manifest, original inventory, process/start records, complete event accounting, original public failure details and submitted launcher. It does not duplicate the complete raw Go stream.
