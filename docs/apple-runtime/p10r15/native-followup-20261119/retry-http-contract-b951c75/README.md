# HTTP probe request accounting and current-readiness proof

Source `b951c750931d80e6c30d94d1b7fbfe1ce9e0cccc` repairs two independently reproduced local defects. The unchanged `a432646e28c419d91995927a5eb3d34db1337466` production helper failed ten causal assertions: seven parsed non-GET methods disappeared from the receipt; STOPPED and EXPIRED services reused old ready files; and the real `input` CLI returned success for an already stopped service.

The final gate ran from the clean committed source and passed 42 named tests with zero failures, errors or skips. It covers the real loopback HTTP server, existing classifier/receipt consumers, publication quarantine and the new terminal-state/identity controls. Nine additional state mutations are followed by restoration and a real valid request sequence. POST and HEAD retain their existing counted-rejection behavior.

The unchanged production baseline, original regression tests, targeted repair output, wider working-tree output and exact clean-source repeat are all preserved. The baseline test snapshot contains the three original causal test methods; the fixed source adds the independent readiness identity/state controls. All files listed in the manifest are byte-preserved Git blobs.

This receipt grants no cache(), retry(), TTL, typed archive or Apple runtime acceptance. The task-by-task audit is in `docs/apple-runtime/p10r15/cache-retry-storage-contract-audit-20261119.md`. Historical native successes and failures retain their original scopes, including the failed transport invocation and the failed F04 nested/empty List cardinality controls.
