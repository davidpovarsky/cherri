# Bounded Simulator session recovery — local controls

Implementation: a52bd7c29b34a6b70771425f34399a2b657aa468.

The original carrier run 44644166680 entered the existing session recovery path and later ended with a cancellation. The retained Dictionary job log does not prove its actor or cause. Source inspection established that shutdown, boot and bootstatus could wait without a subprocess bound, and the previous display loop counted sleeps instead of elapsed wall time.

## Change and limits

The existing recovery path now shares one 240-second monotonic deadline. Individual command allowances remain finite: getenv 15s, shutdown/boot 30s each, bootstatus 120s, Simulator activation 15s, screenshot 25s; each is shortened to the remaining allowance. A timed-out command group is terminated. A late zero exit cannot establish readiness. The original getenv-only quick path is recorded as PROBE_READY, without claiming a displayed editor or import. Recovery requires fresh successful getenv and display probes with the existing 85,000-byte threshold. Each probe uses a new owned scratch location and removes its raster output.

These are command-wait and readiness-acceptance bounds, not a hard real-time guarantee about OS process creation or scheduling. Late failed observations are retained truthfully. This component does not remove signed import, exact output, callback, trace, editor or physical-device gates.

The public diagnostic contains only fixed enums, bounded timing/byte counts, exact source/run/fixture identity and a hash of the selected device identifier. The publisher accepts only that exact path and validates identity, command ordering, exit/timing consistency and fresh probe causality. Raw command output and screenshots remain outside publication. Failed attempts are retained before any later successful attempt.

## Actual verification

Clean Windows replay discovered 19 recovery controls: 17 passed and 2 explicit POSIX-only controls skipped; all 17 pre-existing publication controls passed. Linux execution of the identical four Git-verified source files passed all 19 recovery controls without skips, including actual child-process timeout, process-group descendant termination and the actual Bash wrapper invoking the helper. Bash syntax validation passed. The receipt records exact log/source digests and clean source identity.

Corruption controls reject reused failed screenshots, absent/undersized display, late success, continued commands after a recovery timeout, wrong source/run/fixture/device binding, duplicate history keys, false success/exit codes, understated aggregate elapsed time, and successful commands beyond stage allowances.

Apple execution of the changed component remains pending. No formal requirement or macro task is closed by these local results.
