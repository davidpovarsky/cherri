# R4 macro compatibility harness repair

The original OpenMinis run 44714722383 on consumer source b09f48564adf8d317911a0d552785426a43d4a18 remains FAILED: its macro compatibility step produced 6 passing and 15 failing checks out of 21. Later packaging/ARM64/macro stages were skipped. Its original artifact 14619042515 and log are preserved in the preceding consumer proof 62a5e5ffe08fccc30c4c84497956e10b83fa46ce.

## Repair and exact execution

Implementation d6c899aa25ddcd60c9858dfc9c43918816f2d9d5 changes only the Python compatibility harness and the workflow argument supplying the already-built migration companion. It verifies checksum receipts before execution, authenticates both real Go tool roles/clean source revisions/platforms with the real migrator inspector, and compares all six capability identity fields.

Full doctor controls use the actual canonical Linux amd64 compiler and migration companion from frozen source fd691b96fb661697a1cbaddab92476d457636ba9. Deliberately altered capability-field responses call the existing field validator directly and carry an explicit controlled-response scope plus an execution witness. A separate negative proves that a self-receipted shell proxy cannot impersonate the installed compiler through full doctor validation. The old-manifest control commits its disposable documentation variant and binds that exact clean fixture commit.

A dirty precommit execution passed all 23 checks and is retained separately. The unchanged clean implementation then passed all 23 checks in 32.656 seconds, exit 0, under actual Linux WSL, job job_32806c08cefc. The source was clean before and after and both changed file hashes were stable. There was no Go build, new checkout, dependency replacement or remote redispatch in this replay. Original stdout is SHA256 ec18e19aeb9ad927512965141158d3fb007d0802fc86200e6c269a583e95fc82; stderr is empty.

The 23 checks comprise 15 actual installed-pair cases, 7 explicitly controlled capability-field cases, and 1 controlled-proxy installed-identity rejection. The generated canonical documentation body is used in isolated Git fixtures; this harness is not an additional Pages publication or a new final consumer tuple.

## Receipts and limits

The ZIP retains exact original reports, process receipts, stdout/stderr and precommit diff, plus the actual launch scripts and explicitly normalized Sentinel notifications. The member manifest independently verifies every archived byte. The immutable source witness hashes the workflow, harness and every tracked Skill file from the exact Git commit with replacement objects disabled. Existing large canonical binaries remain in the prior evidence directory and were rehashed against their authenticated identities.

Two receipt-writer attempts stopped before any test execution (stdin/input argument conflict, then an already-created empty output directory); their original failure outputs are retained in the archive. The actual clean test was not rerun or changed.

This is a bounded harness/component success. It neither changes the original failed workflow outcome nor grants new 179-row, Apple signing/runtime, final CI, final package or project-closure credit. No remote rerun was dispatched. Compiler/docs/manifest pins remain unchanged.
