# Exact K11 full179 durable launch proposal

Status: **HELD — prepared code only. No unit, producer, AI request, signing operation, or test has been launched.**

This proposal supersedes only the local179 operational section of the retained
K10 execution plan. It does not prepare a new seed, alter a sealed bundle,
repeat a private S4 parent, change source, or promote RemoteBuilder. The source
may change after the current CI diagnosis; such a change requires an explicit
new review of constants, source witness, spec and output location. There is no
automatic substitution of a newer checkout or commit.

## Fixed source and inputs

| Role | Pin |
|---|---|
| Implementation K11 | `9b4135e705b08f765b7304e01212d3b9658806fb` |
| Implementation tree | `d3688f219f7454e80778cbcb3013967b6d339aad` |
| Ordinary source | `/dev/shm/cherri-p10r15-runtime-20261123/cherri-k11-source` |
| Named branch | `agent/cherri-p10r15-k11-verification-20261123` |
| Compiler R7 source | `ca316a3dee252456c4bec751594cf249c709b331` |
| Compiler checkout | `/mnt/development/projects/releases/cherri-p10r15-r7-compiler-source-20261123` |
| Compiler SHA256 | `42f6e42e7b6a8ea6f523a752fd2f06b96265e733fa00ad808190e062d1266301` |
| Migrator SHA256 | `050e33ad7b078118c0c9dea8ca32a0123926d4a975154244311a5a9f4fe64f91` |
| Documentation D11 | `365b08c5b4c71c6d22b591babb8d1ab852e5a3b9` |
| Documentation checkout | `/mnt/development/projects/worktrees/cherrilang-p10r15-d11-20261123` |
| Future original output | `/mnt/development/projects/evidence/cherri-p10r15-k11-local179-20261123/attempt-01` |
| Expected identity | ubuntu, UID1001/GID1001, actual HOME `/home/ubuntu` |

The source clone is an ordinary, clean, canonical-origin, nonshallow checkout.
It remains immutable. All unique originals go to persistent development
storage. The supervisor rechecks its exact SHA/tree/branch/origin, source
witness, the three immutable contracts, the D11 checkout, R7 checkout, and both
R7 binaries before and after execution. It does not label the R7 documentation
compiler as a K11 compiler.

## Release of the hold

The file `launch-spec.PROPOSAL.json` is deliberately incomplete, with
`launchEnabled: false`. Neither script executes by default. A root-reviewed
`launch-spec.REVIEWED.json` and its independently supplied SHA256 are required
for dispatch. Root fills the missing values from retained actual originals:

1. A completed original live evaluation bundle, its original manifest/report
   hashes, and its terminal receipt. The manifest must be sealed for this exact
   implementation SHA. The twelve original task/arm outcomes must all be
   measured (including truthful measured refusal/truncation failures).
2. Independently retained authenticated BEFORE and AFTER Asana originals,
   outside the bundle, with exact hashes and actual `Z` observation timestamps;
   distinct files/inodes; exact story ID and commitment-text hash. The launcher
   cannot authenticate supplied JSON, and explicitly does not claim to do so.
3. Four original GitHub REST run responses for the same exact source SHA and
   branch: **Build & Test**, **OpenMinis Skill**, **iOS Build**, and
   **iOS 27 Shortcuts Runtime PoC**. Bind each response's hash, run ID, attempt,
   event, terminal status and actual conclusion. A terminal failure remains a
   failure. The production CLI performs its own authenticated `gh` discovery
   and grants CI credit only for successful required workflows.
4. A contemporaneous capacity review with explicit minimum available persistent
   storage, tmpfs storage and RAM. These are launch reserves, not output quotas
   or changed producer limits. The K8 archive expanded to 102,725,239 bytes;
   that is a planning reference, not an upper bound for this run.
5. Root's explicit one-run authorization and the exact proposal-file hashes.

The canonical Go AI consumer at
`tools/language-acceptance/ai_measurement_execution.go:303–309` requires the
intake's `implementation_sha` to equal the running producer's exact SHA. An AI
bundle sealed for another commit cannot be restamped or silently reused for
this report. A changed candidate or an incomplete original AI run must be
resolved by root before launch; this proposal does not invent an exception.

Readiness checks here are operational guards. The original Go consumer still
executes the two canonical Python intakes, each with its original two-minute
bound and exact source witness, and validates all original provider/validator
receipts, chronology, signatures, inventory and scores. No separate duplicate
standalone intake is proposed.

## Original producer and time envelope

The sole production command is the original operation, using absolute paths
for the already observed executables:

```sh
/usr/bin/timeout --signal=TERM --kill-after=30s 80m \
  /usr/local/bin/go run ./tools/language-acceptance --phase local \
  --out /mnt/development/projects/evidence/cherri-p10r15-k11-local179-20261123/attempt-01/acceptance-report.json
```

There is no `--evidence`, `--generate-manifest`, selection filter, second suite,
or changed per-family deadline. All 179 IDs must match the original contracts:
94 acceptance, 52 repair and 33 recovery gates.

Sentinel's 3600-second background ceiling is too short for the original
80-minute producer. The proposed root dispatch uses a **system** transient
service running as ubuntu. Read-only inspection found systemd255 and a
reachable Ubuntu user manager, but `Linger=no`; a system service avoids relying
on that user session remaining alive. No linger or service configuration is
changed, and no unit has been created during preparation.

The system service uses `RuntimeMaxSec=5340s`, `TimeoutStopSec=30s`,
`KillMode=control-group`, `KillSignal=SIGTERM`, `FinalKillSignal=SIGKILL`, and
`Restart=no`. This allocates 89m active service time, at most30s termination,
and at most30s for the separate `ExecStopPost` receipt within the original90m
outer envelope; the nested producer retains its full80m+30s allowance.
The source-check and receipt overhead uses the remaining
margin. The supervisor also requires the expected service cgroup, so direct
invocation cannot bypass the host envelope. The original per-family limits
are unchanged.

Root's future launch, **not executed during this preparation**, is:

```sh
python3 -B /mnt/development/projects/evidence/cherri-p10r15-k11-local179-plan-20261123/launch_reviewed.py \
  --launch-reviewed --spec-sha256 EXACT_REVIEWED_SPEC_SHA256
```

The launcher exclusively claims one named unit/attempt before asking systemd
to start it. It never deletes that claim. A nonzero dispatch result, lost
transport response or absent receipt requires inspection of the exact unit and
durable files; there is no automatic retry. A dispatch exit0 is not a producer
result. `ExecStopPost` retains the host's actual exit classification and a final
hash inventory after remaining service processes have been stopped. That
metadata-only stop command has its own29s timeout plus1s kill grace; it runs
no producer. Service activation has a separate30s failure bound and no
randomized runtime extension. These operational guards do not enlarge any
acceptance deadline.

## Environment and signing scope

The producer receives an explicit environment, not the parent process's
credentials. It uses the actual Ubuntu HOME and existing ordinary `gh`
authentication. No token is read, exported or copied by the launcher.

| Setting | Exact value |
|---|---|
| GOTOOLCHAIN | `go1.24.13` |
| GOMAXPROCS / GOFLAGS | `2` / `-p=2` |
| GOMODCACHE | `/mnt/development/tooling/cherri-go/mod` |
| GOCACHE | `/dev/shm/cherri-p10r15-runtime-20261123/go-build-cache` |
| GOTMPDIR / TMPDIR | `/dev/shm/cherri-p10r15-runtime-20261123/go-tmp-ubuntu` |
| CHERRI_ACCEPTANCE_PYTHON | `/usr/bin/python3` |
| PYTHONDONTWRITEBYTECODE | `1` |
| Git | no replacement objects, no optional locks, no interactive prompt |

The retained Go toolchain must actually report `go1.24.13 linux/arm64` before
the sole production command. This version read does not run a test or compile
an extra candidate.

`CHERRI_K05_LIVE_SIGNING=1` enables only the existing K05 public synthetic
fixture in the original full179 producer. It retains its original Go8m /
process9m family and nested build5m limits, the original fresh output policy,
one public fixture signature and zero retries. The test builds its exact
current-source compiler as already required. The separate R7 documentation
binary is not substituted for it. `CHERRI_SIGNING_SERVER` remains unset; the
existing hub signing selection is retained. This is no Apple import/runtime
claim and no RemoteBuilder activation.

Provider credentials, generation authorization and private corpus opt-ins are
absent from the constructed environment. The five private S4 original R7
receipts remain separate evidence; this179 producer has no S4 receipt-import
option and does not rerun those parents. D03 and native/platform requirements
retain the statuses produced by their original contracts.

## Durable outcome and truthful interpretation

Preserve the original stdout, stderr, PID, argv, GNU timeout exit, wall times,
source checks, before/after capacity and every generated companion artifact.
The supervisor's result records the producer's actual exit separately from
report integrity. It checks all179 unique IDs and recomputes status counts,
retains the original `closure_complete`, and marks its own
`formalAcceptanceGranted` false. Exit0, a complete report, a successful K05
signature and a measured AI score are distinct facts; none substitutes for
global closure.

Partial reports, timeout exits, source changes, original failures and missing
artifacts remain in place. No process result or report is normalized into a
pass. If the outer service must kill the supervisor, the host stop receipt and
retained partial originals remain the authoritative terminal evidence.

Only syntax parsing and static source review are appropriate for this proposal.
Actual systemd dispatch behavior and the complete producer have **not** been
executed in this lane. Root reviews the exact code/spec before any launch.
