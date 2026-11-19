# Intermediate immutable Remote Builder candidate: S71225 / D7f4c65 / Kd6872ba

This is source-bound candidate evidence, not a completed release or a live installation.

- Compiler S: 71225b71f5353a0f661722d154a8b670b52e76f6.
- Docs D: 7f4c65d274548cb2c563626781de2a07a456638e.
- Skill/tooling K: d6872ba6f17f3cc592a587430541b803078a212e.
- The ordinary-clone Linux ARM64 build embeds S and vcs.modified=false. Build receipts record the actual Go version and flags. This host build's archived provenance records CGO_ENABLED=0; Alpine portability is NOT established by this archive.
- Source package:20 files; runtime package:23 files. Exact create/verify/unpack integrity passed. ZIPs/binary remain external build outputs; their hashes are recorded. Package integrity alone is not installed execution.
- Actual candidate entrypoints and causal integrity:36 PASS,0 FAIL,1 explicitly NOT_RUN (the runner Go test, resource-serialized with private regression). activationGatePassed remains false. Live files were unchanged, and every candidate mutation was restored.
- Actual loopback signing matrix:58 checks,5 valid unsigned plist controls,53 nonzero/no-artifact rejections,0 failures. Five surfaces include both Remote Builder language routes. All22 remote signing requests match the exact validated preflight bytes. Endpoint-instrumented wrapper copies are used; no external signer or Apple cryptographic validation was exercised by this matrix.
- Independent verifier controls:2 actual emitted plist controls plus14 damaged native-graph rejections. The old timeout behavior reproduced a surviving child late write; owned process-group cleanup prevented it. These verifier fixtures came from8f1a458, explicitly separate from current-release execution.
- Actual CI failure44435923463 showed loss of executable mode when copying the compiler. The POSIX copy control reproduces PermissionError and verifies byte-identical executable preservation with copy2.
- Actual CI failure44436635162 could not check out the pinned compiler tree in the borrowed repository. The corrected direct exact-pin fetch was verified separately. Two exploratory local clone probes did not establish the precise shallow-clone cause; no such causal claim is made. CI package execution must still pass the updated workflow.

The broader private zero-demotion/natural-coverage gate and Legacy rawAction integer-kind regressions were still being repaired when this candidate was measured. Native Simulator/physical gates remain independent. No live promotion, plugin package version update, project-completion or all179-pass claim is made.
