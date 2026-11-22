# P15 full-suite scheduling: measured costs and bounded change

## Status and scope

The original complete CI run [44971815217](https://github.com/davidpovarsky/cherri/actions/runs/44971815217) remains failed. In that run the macOS ordinary root package exhausted its unchanged 15-minute deadline; the Linux ordinary suite completed. The separate acceptance invocation completed on both hosts. A later component observation does not replace that failed complete run.

The isolated four-parent observation at source `db59215024e4a95985d756f459bbc87c97d48a61` passed on Linux and in the actual macOS [run 45029497251](https://github.com/davidpovarsky/cherri/actions/runs/45029497251). Each execution contained exactly 29 parent/child identities and 235 paired phase observations, with clean source before and after, zero failed or skipped selected tests, and retained original process streams. The wrapper commit is `1e624c89645b6b974f47c9bc7b483dbfa9242cd2`; the observed source tree is `9799b35d3d4799a30efade932f591428e5443e3e`.

These observations identify real subprocess costs. They do not grant full-suite, formal179, Apple language-runtime, or release acceptance.

## Measured work

| Observed parent body | Linux seconds | macOS seconds |
| --- | ---: | ---: |
| P10 isolated compiler mutations | 74.608 | 86.698 |
| Canonical behavior propagation | 47.108 | 48.425 |
| Extracted install and rollback | 29.435 | 62.904 |
| Extracted public CLI | 3.544 | 10.164 |
| Entire selected Go process | 157.119 | 262.720 |

The parent-body measurements exclude outer Go preparation; the entire process includes it. They are observations from different hosts and cache histories, not a controlled cross-platform speed ratio. The macOS first mutation baseline took 35.423 seconds; subsequent actual mutations took 9.655–10.839 seconds. New canonical source probes took 9.633–10.842 seconds. Installer verification was also materially slower on that macOS runner. The actual-tool byte cache recorded one miss followed by one hit, so this evidence does not support claiming fourteen redundant compiler builds.

## Reviewed scheduling change

Commit `080b5552218460c44005ce643a12a6869b5c4be3` adds one entry hook to exactly 27 audited top-level tests whose expensive work occurs in owned subprocess fixtures. Their original bodies, assertions, fixture copies, commands, child budgets and cleanup operations are unchanged. Independent reconstruction removed only the 27 added hook lines and reproduced each affected original test-body file byte for byte.

Only the complete ordinary invocation opts in, using an owned copy of its environment with `CHERRI_FULL_SUITE_SUBPROCESS_PARALLEL=2` and the actual Go argument `-parallel=2`. The shared guard additionally requires an empty `test.run`, a top-level test name and the exact limit of two. An otherwise enabled invocation with another limit fails closed. Selected producers and nested subtests keep sequential event streams. The acceptance invocation explicitly removes an inherited scheduling marker. The strict Go event parser remains unchanged.

The shared cache is mutex protected and stores immutable executable bytes; callers receive separate files. Reviewed installer homes, source/documentation clones, HTTP listeners and signal targets are owned by each fixture. Canonical mutation tests create and remove their own detached worktrees. This change schedules existing work; it does not substitute cached test results or remove test execution.

## Focused integration evidence

The scheduling implementation was integrated with the reviewed FN05, RP24 and RP25 producers at clean source `3921e26ef0e1e5282f369bf22f6d1e3fd160ecff`, tree `f0e70be03d3bcce0d84e3826211002a5b08bfd2b`.

- Nine Python runner controls passed, including independent environments, removal of inherited acceptance markers, invalid limits, exact package coverage and failure propagation.
- The scheduling-policy parent and thirteen policy cases passed; zero failures or skips.
- Fresh exact-source FN05, RP24 and RP25 producer/intake checks passed together in 21.594 seconds. RP24 and RP25 retain their missing actual Apple-runtime tiers; no complete requirement closure is inferred.

The original development controls from the component branch are retained separately. Those pre-commit controls are not represented as exact-source execution at the later component commit. The integration receipts above identify their own actual clean source before and after execution.

## Preserved evidence

The adjacent [evidence manifest](evidence/p15-full-suite-scheduling-20261122/manifest.json) binds the retained ZIPs, original streams, hashes, source identities and run identity. The downloaded macOS artifact is preserved byte for byte: artifact `14757124343`, 782660 bytes, SHA-256 `ca832afc29078da4176a85be19129feb310e8ef16d3459ca0bce967c1adc57af`, 668 members. Its member paths and original-stream hashes were independently checked during intake. Linux originals and focused integration receipts are also retained.

The observation-only instrumentation stays on its dedicated source branch. It is not part of the integrated production candidate. A fresh complete suite at the final combined source is still required before claiming a successful deadline or performance improvement.
