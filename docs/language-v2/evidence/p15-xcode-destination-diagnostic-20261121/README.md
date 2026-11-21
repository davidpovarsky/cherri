# Active-Xcode Simulator launch and bounded diagnosis — 2026-11-21

Execution mode: [SUBAGENT /root/apple_runtime].
Source: `6742e125a185ec985726132ec115daea9f2c0661`, based on K1 `d90b69a92e83b5034728c22e062a3dd96153a2a8`.

## Original native problem and scoped change

The K1 File and N03 runs both returned MISSING for the exact owned Xcode destination, followed by an `open` command exit code 1. The original helper discarded that command's stdout/stderr to DEVNULL, so its exact OS failure cause is unknown. Their byte-identical public artifacts are preserved separately in `../p15-apple-k1-startup-scope-20261121/`.

The candidate resolves the active Xcode's `simctl` path with `xcrun --find simctl`, verifies the expected tool/app layout and existing Simulator.app, then launches that explicit application for the same owned device. Resolution (at most 5 s) and launch share the existing 15 s allowance. The 90 s destination gate and 900 s test phase remain unchanged. There is still at most one refresh, without another import, a reboot or an alternate device.

Original combined stdout/stderr for resolution/launch is retained in the existing quarantined builds area. A closed public diagnostic preserves hashes and only fixed error classes, numeric known-domain codes and destination row properties. It is recomputed from the original bytes at publication and bound to the canonical destination receipt. The new receipt schema requires that diagnostic; unchanged legacy schema remains supported.

These are infrastructure and evidence changes. No native runtime semantics or formal acceptance is granted. A read-only root review found no blocker within that scope.

## Actual local results, including contradictory evidence

Two targeted development runs each completed 32 cases: 31 pass and one existing POSIX-only skip. The clean source's later remaining-case run also passed all 31 destination controls and retained its existing POSIX skip.

The first full clean replay reached its original 900 s limit. It remains TIMEOUT with no exit code. Its 42,244-byte stderr SHA-256 is `8ee506b7952371db5f2234693c6641b878e7f91b166b9065e81608803d5d92e9`.

A fixed inventory enumerated 396 actual case IDs. The original log explicitly completed 240 IDs. Only the remaining 156 were replayed at the same clean source, completing in 448.365 s: 151 pass, four existing POSIX skips, one failure and zero errors. Their 29,617-byte stderr SHA-256 is `d73ff77bf301597bad229edf7e79e8b1c74ddd46985e414c1448c59c74e8754d`.

The single failure was the existing `test_terminal_receipts.SourceAndShellTests.test_actual_cli_uses_exact_owned_checkout_and_source_fixture_bytes`: its real checkpoint CLI returned 1 with the intentionally protected generic acquisition-rejection message. The original fixture was cleaned up by unittest; its internal exception was not retained, so a root cause is not proved.

A separate preserved fresh-fixture original CLI replay at unchanged 6742 succeeded in 1.687 s. A separate instrumented diagnostic replay also succeeded in 1.593 s; its ten actual Git calls each took 0.062–0.125 s and raised no exception. These successful replays do not explain or erase the earlier failure.

The complete existing SourceAndShell test class was then executed once at the same clean source. All seven actual tests passed in a 13.875 s process, including the original failing assertion path and dirty-source, foreign-checkout, replacement-object and shell/finalizer controls. Its stderr SHA-256 is `4b7018e49774ed4457ede3aa084caf3d9d081e5ebb399d2714e2a2d02d2473d1`. The original ten-second source guard was not changed.

All 396 IDs have completed results; the first full inventory remains **391 pass, four skips and one failure**. Every ID has a passing or skipped observation after the focused replay, but the contrary earlier failure remains unresolved. This packet deliberately records `singleProcessFullSuitePass: false`, `stableSuitePass: false` and `unexplainedPriorFailure: true`. It is not a successful full-process CI gate.

Source HEAD and status remained identical and clean before/after all source-bound runs. The exact logs, process receipts, fixed inventory, replay driver and instrumented observations are preserved in `originals.zip` and independently hashed. The preserved synthetic replay checkout directories remain under the external host evidence directory; no user-private fixture was used or published.

## Next evidence boundary

Integrate only as a reviewed diagnostic candidate with the contrary local result disclosed. The combined source requires its own CI gate. A single new File run on the frozen integration should exercise the repaired destination path; N03 and nested fixtures follow only after test02 actually starts, or after a distinct evidence-based change. No original K1 failure is relabeled, and no original native fixture is rerun unchanged.
