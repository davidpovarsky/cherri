# R23 inactive revision 4: one-parent synthetic qualification

Status: **PASS — synthetic fixtures only**. This is not owner authorization and is not a real-repository R23 v2 PASS.

The root-reviewed inactive patch SHA256 is `65b4faad9bfdafd5b0bea4383c38f010f8098845e4eafff1e92593f5c309fbca`. Exactly one existing parent ran once:

```text
timeout --signal=TERM --kill-after=5s 60s python3 -B tools/p10r3/test_coverage_and_gates.py TestP10CoverageAndGates.test_c5_06_r23_protected_ios_file_modified_fails_closed -v
```

Original process exit: **0**. Actual supervisor duration: **1.118164637 seconds**. Original unittest stderr reports **Ran 1 test in 0.930s — OK**. No timeout, retry, deadline change, failure suppression or additional test parent was used.

The qualification copy contains 13 exactly hashed files and has no real Git repository. Only the existing test's owned temporary Git fixtures receive explicitly labeled SYNTHETIC_TEST_ONLY policies. The production record in the qualification copy remains DRAFT_NOT_APPROVED with proposalOnly=true and null approval source/time fields, unchanged SHA256 `feeee0f607d170ccb9602ede2213871c4a0dfcf8e82547b5a381b828d52fa6c7`.

All 22 original parent identities remain unchanged. The original 11-action/assertion block is exactly 3,473 bytes, SHA256 `63f878bd2aec9f75e1926d6e5a7d67bb1cbcba81ded5c8ffa22afb6791bcb64b`. The selected parent completed without failures or skips; its added policy/context/ancestry/working-state controls and real synthetic --gate child are part of that parent. No broader suite was executed.

Before/after guards confirm the active owned source stayed clean at `14a7440e839afdceca56a54c608f294e1d01073a`, and all 13 copied files remained unchanged. The owned persistent temporary root was empty after unittest cleanup. No active source patch, actual production R23 execution, Go execution, provider call, signing, workflow dispatch, or policy approval occurred.

Original command, timestamps, monotonic duration, copied-file/source guards and hashes are in launch-plan.original.json and result.original.json. Original stdout/stderr and the exact driver are retained. The original execution manifest remains unchanged; packet-manifest.json additionally binds this intake and the preserved driver.
