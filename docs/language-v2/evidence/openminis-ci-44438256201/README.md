# OpenMinis CI 44438256201: scoped successful package evidence

[The completed GitHub Actions run](https://github.com/davidpovarsky/cherri/actions/runs/44438256201) succeeded at workflow/Skill commit **0e577e2476b7097479f5d2f50d8e3ee2ce7d2571** (K). Its one job recorded 42 successful steps, including setup and post-job steps. This archive preserves the reports actually uploaded by that run, their byte hashes, the exact workflow, and the package-step transcript.

The packaged compiler is separately pinned to **71225b71f5353a0f661722d154a8b670b52e76f6** (S); documentation is pinned to **7f4c65d274548cb2c563626781de2a07a456638e** (D). These are historical intermediate commits. This run does not verify the later combined compiler repairs or establish release/project completion.

## Independently checked package evidence

All eight downloaded GitHub artifact ZIPs matched the SHA-256 digests returned by the artifacts API and passed ZIP CRC checks. They contain 21 top-level files. This archive retains the **17 small reports/transcripts (292,046 bytes)**; the two package ZIPs and two standalone executable files remain outside Git. The artifact inventory records every member's size and hash, including omitted binaries and archives.

| Actual package report | Observed result | Independent byte checks |
| --- | --- | --- |
| Source integrity | PASS; 20 files including generated manifest | 19 payload files match their exact committed K bytes; declared sizes, hashes and POSIX modes match |
| Runtime integrity | PASS; 23 files including generated manifest | Same 19 source files, compiler and two compiler sidecars; every declared size, hash and mode matches |
| Unpacked runtime integrity | PASS; 23 files | Matches the runtime integrity receipt, with unpacked=true |
| Package production evidence | K01: 1 passed, 0 failed, 0 skipped | The actual TestSkillPackageProductionEvidence JSONL contains one matching receipt, one pass, no failure or skip |
| Unpacked Alpine self-test | PASS_WRAPPER_SELF_TEST | Eight actual CLI operations and ten explicitly mock installation scenarios; zero formal requirement records |

The runtime compiler SHA-256 is **4d97bfe1cd7092a65615705a5821525d4ad601efd70de3e10e682e6507a27d6f**. Inspection of its unique Go build-info header, without running or rebuilding it, records Go **1.24.13**, **CGO_ENABLED=0**, **linux/arm64**, exact S, and **vcs.modified=false**. The two standalone executables uploaded alongside the ZIP are builds of K; they are distinct from the S compiler inside the runtime package.

The actual successful CI package-step transcript records a fresh repository under RUNNER_TEMP, an exact fetch and detached clean checkout of S, then an explicit CGO_ENABLED=0 GOOS=linux GOARCH=arm64 build with buildvcs=true, trimpath and -s -w. It creates and verifies the packages using that output. The retained runtime reports and the downloaded binary agree on its hash and source. This is evidence of the CI pinned build, not an inference from the workflow declaration alone.

The Alpine step uses docker run --platform linux/arm64 alpine:3.22 under ARM64 emulation. Its observed image digest is **sha256:49340b346798875b5592e5d420241f2f9cfba5f47e62ebb1446a1b34734e898f**. It executes the unpacked package with an empty CHERRI_HOME and --skip-setup. The eight actual operations are version, action discovery, catalog, unsigned build, decompile, recompile, prepare-edit, and signed AEA1 input rejection. The ten installation scenarios inside the self-test remain **mock controls**; this report does not convert them into a real remote installation.

## Other retained reports and their exact scope

| Report | Recorded checks | Boundary |
| --- | --- | --- |
| Macro contract ledger | 19 successful compiles, 8 causal mutations | Synthetic CLI structure and lossless native wire only; all 11 Asana children remain open |
| Macro ledger verifier | 19 passed, 0 failed | Actual verifier exits against copied report mutations |
| Macro compatibility isolation | 21 passed, 0 failed | Actual compiler/docs plus isolated single-field mutations |
| Debug export | 16 compiles/roundtrips, 80 model cases, 16 CLI negatives, 7 causal faults, 12 probe compiles | Native executions: 0 |
| Bounded pagination | 32 compiles/roundtrips, 144 model cases, 44 CLI negatives, 7 causal faults | Native executions: 0 |
| Storage probes | 62 CLI fixture compiles, 3 exact native-action roundtrips, 57 Python checks; three named Go tests | Source/CLI only; signing not run, persistence and typed archive not proven |
| Continuation probes | 4 CLI fixtures/roundtrips, 13 Python checks | Two-invocation orchestration and graph checks only; native executions: 0 |
| Signing fault matrix | 34 checks: 3 unsigned controls, 31 rejected negatives, 0 failures | Instrumented wrapper copies and loopback endpoint only; not Apple's cryptographic or native-device validation |

The signing matrix in this run is the **34-check** CI matrix. It is separate from the later **58-check** Remote Builder matrix.

The core workflow step executes exactly:

    go test -run TestCherriNoSign -v ./...
    go test -run TestDecomp -v ./...

The workflow also records successful focused pinned-install/K02/K03 tests. Neither this scoped core selection nor the one-record K01 package receipt establishes a full repository Go pass, the five private corpus gates, all 179 acceptance requirements, Apple native execution, or the final combined source. The K01 report explicitly retains **closure_complete: false**.

## Retained evidence

The run metadata preserves every job-step name, timestamp and result. The workflow snapshot is the exact blob at K. The selected package-job transcript contains the actual package, Alpine and K01 producer steps. The two package manifests are exact nested-archive manifests. The archive audit records independent hash, inventory, committed-file, build-info and receipt checks.

The complete job transcript remains outside Git: **200,732 bytes**, SHA-256 **ad140ecd5ae918577b2ea36c70b7ed9d3a37d2321ed5aff4328baa5e935e34f3**. The original downloaded ZIPs also remain outside Git. All report files under artifacts/ are the exact bytes uploaded by the run; no status has been rewritten.
