# Selected simulator session boundaries after XCTest

The original K control run [45548514795](https://github.com/davidpovarsky/cherri/actions/runs/45548514795)
did not establish a nested List runtime result. Its successful first XCTest process
ended at 05:02:34 UTC; a getenv probe passed at 05:02:39; the external import then
received CoreSimulator Shutdown/405 at 05:03:04. Test02 also exited zero, but the
subsequent receiver launch/openurl again encountered Shutdown/405. These original
observations identify an unsafe phase boundary. They do not identify a particular
CoreSimulator internal defect or prove a compiler semantic failure.

The orchestrator now reacquires the same selected device before the external
pre-import and post-XCTest runtime phases. The new helper retains the actual
device-state queries, source HEAD checks, boot/bootstatus/getenv commands, return
codes, streams and capture/cleanup details. It uses the existing active-Xcode
Simulator resolver, including the resolver's explicit headless result; neither a
resolver success nor a getenv result is treated as an editor/runtime witness.
Fresh device-state observations must also show the exact available iOS 27 device
booted after bootstatus and after the final liveness probe.

A stopped device can receive one boot command. This helper never shuts down,
erases, deletes or recreates a device. A device still shutting down, a failed or
late command, changed source or a final state regression blocks the phase.
The original session allowance stays 240 seconds; existing per-command bounds
remain 15 seconds for liveness/launch, 30 seconds for boot and 120 seconds for
bootstatus. Original stream capture and encryption reuse the existing reviewed
helpers and recipient certificate. No native test or Shortcut is run by the helper.

Only encrypted originals and a bounded public summary are uploaded in the
separate session-boundary artifact. The public summary contains fixed states,
source/run identity, process outcomes and hashes, while raw device names,
identifiers, paths and command streams remain encrypted. The ordinary synthetic
artifact publication allowlist is unchanged.

The complete runtime callback oracle and all final runtime/trace/acquisition
gates remain byte-identical to reviewed source 9986c410:
38,963 bytes, SHA-256
eade1b5fc51006518c7c393c4dc8c5f77b7fb22803028e83ac3b268176b34ce1.

Local verification exercised 20 new phase controls plus the existing session
and destination suites: 78 tests passed. The controls include source changes,
duplicate/wrong/unavailable devices, late zero exits, a shutdown after a successful
getenv, one owned boot, exact stream retention and actual encryption to the
existing recipient. Shell syntax passed. A broader local selection encountered
the existing nested-original test's compiler-building setup in this sparse
checkout; it failed before compilation because no go.mod is checked out. That
result is retained and grants no coverage. The hosted workflow continues to run
its unchanged complete original QA suite.

No macOS session result is claimed from these Linux controls. The next native
diagnostic must bind the published source and reviewed dispatch plan, retain its
original packet and continue to satisfy the original signed import, fresh
callback, trace and nested-control goals. The old K failure remains unchanged,
and a candidate remains gated on an independently verified control.
