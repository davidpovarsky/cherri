# Simulator QA command ownership

The existing run_cmd_timeout shell boundary left its separate command process
group running after an outer cancellation. It also accepted a zero parent exit
while an owned descendant remained alive. Two actual synthetic POSIX controls
failed for those exact reasons on source T (618423ffb40f43b2cc8179d4e5b00a01811bfa73).

The narrow repair reuses project_prerequisite_observation.capture. An optional,
validated pair of inherited descriptors lets an exec adapter preserve the
caller's live stdout/stderr, including uncapped Xcode logs and command
substitution output. The adapter replaces itself with the original command in
the capture-owned process group. The existing default capture arguments remain
unchanged. All 21 shell callsites, argv, redirections, fixture logic, assertions,
and timeout values remain byte-identical; the original five-second cleanup
envelope is retained.

Actual Oracle qualification on the three exact implementation bytes in the
manifest: both original causal controls pass, and three additional controls
prove timeout124, exit37, literal arguments and lossless streams larger than
3MiB. Five named controls pass with no errors or skips. Both synthetic descendant
groups are observed absent afterward. The earlier GREEN owner failed before
launching any test because it embedded JSON null as a Python literal; that
original failure is retained alongside the corrected owner and actual results.

originals.zip and its manifest retain all source, command, process, stdout,
stderr, test fixtures and result originals for these controls. This is source
and process-lifecycle qualification only. It contains no Xcode build, simulator,
Shortcut import/runtime, or formal acceptance result. Actual local F01 remains
pending its separately bound source, device, build and runtime invocation.
