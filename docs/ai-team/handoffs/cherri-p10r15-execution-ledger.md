# Cherri P10R15 execution ledger

## Mandate and baseline

- Owner execution request: 2026-11-18 (Asia/Jerusalem), including code, tests, commits, pushes, documentation and Asana handoffs.
- Source repository: https://github.com/davidpovarsky/cherri
- Verified P10R14 base: `ca8a62327e429d5574c8f2a5d440de2c5e553ab2`.
- Fresh origin fetch: local and remote P10R14 identical; original worktree clean.
- Both P10R12 `8ee9910e7968eaecb5b9b095c9f7b05cdb7b6800` and P10R13 `11428e8db52c46ee3d616aa4b923d06cac0e48df` are ancestors (git merge-base exit 0).
- Fresh divergence: main-only 15; P10R14-only 288. Integration must preserve both histories.
- Integration branch: `agent/cherri-p10r15-project-closure-20261118`.
- Integration worktree: `C:\Users\DAVID\Code\cherri-p10r15-project-closure-20261118`.
- Separate direct ChatGPT agents: `/root/core_acceptance`, `/root/inventory_import`, `/root/runtime_inspector`, each in its own P10R15 worktree.
- Current Asana master: https://app.asana.com/0/1219603572114432/1220635996500993
- Existing latest CI independently read: https://github.com/davidpovarsky/cherri/actions/runs/43950196497 — Linux PASS; Nix PASS; macOS CANCELLED with no steps. Overall failure, not all-platform PASS.
- Only one operational SentinelX host is enrolled: Windows. Existing GitHub xcode-27 workflow is the simulator route. Physical iPad is not an enrolled execution surface.

## Evidence and status vocabulary

EXECUTED_PASS / FAILED / IMPLEMENTED_UNVERIFIED / NOT_RUN / BLOCKED_EXTERNAL / BLOCKED_DEVICE.
A unit or structural test does not substitute for signed Shortcuts execution or physical-device acceptance.
Existing private corpus remains on the authorized Windows host; raw source and private values must never enter public CI, HubSign or this ledger.

## Stage ledger

| Phase | Task GID | Acceptance scope | Source SHA | Tests / evidence | Status | Blocker | Rollback impact |
|---|---|---|---|---|---|---|---|
| 0 | 1220635996500993 | Branch identity, clean worktree, ancestry, divergence | ca8a62327e429d5574c8f2a5d440de2c5e553ab2 | SentinelX git fetch/status/merge-base/rev-list, GitHub exact-run jobs | EXECUTED_PASS | None for baseline identity | New isolated branches only |
| 0 | 1219603546838978 | Baseline build and full Go suite | ca8a62327e429d5574c8f2a5d440de2c5e553ab2 | go build ./...; go test -count=1 -timeout 15m ./...; local evidence directory | EXECUTED_PASS | None for baseline Go suite | No deployment |
| 1 | 1220432417414104 | F01 plus 94/RP52/BRG33 truthful evidence mapping | ca8a62327e429d5574c8f2a5d440de2c5e553ab2 | Production-path reproduction in isolated core branch | IN_PROGRESS | No new PASS claimed | Feature branch only |
| 2 | 1220085240857296 | Complete canonical catalog proof ledger | ca8a62327e429d5574c8f2a5d440de2c5e553ab2 | Reuse tools/p10r3 generators and current catalog execution test | IN_PROGRESS | Native/device proof remains separate | Feature branch only |
| 3 | 1220544599390563 | Typed macro roadmap | ca8a62327e429d5574c8f2a5d440de2c5e553ab2 | Existing debug infrastructure and dependencies being inspected | IN_PROGRESS | Storage persistence and catchable native errors unproven | Feature branch only |
| 4 | 1220655985907047; 1220646564229070; 1220546287612106 | AX Inspector, private artifact quarantine, real Legacy target | ca8a62327e429d5574c8f2a5d440de2c5e553ab2 | Existing P14 harness reused; historical synthetic AX evidence available | IN_PROGRESS | New signed runs not executed yet | Feature branch only |
| 5 | 1220432417485023 | Consumer tuple and installed verification | ca8a62327e429d5574c8f2a5d440de2c5e553ab2 | Pending coherent source changes | NOT_RUN | Dependent on new source candidate | No installed consumer changed |
| 6 | 1220500827828558 | Exact candidate platform/release matrix | ca8a62327e429d5574c8f2a5d440de2c5e553ab2 | Pending source integration | NOT_RUN | Physical device unavailable; CI not yet dispatched | Main unchanged |

## Known baseline defects under reproduction

- V2 short-circuit lowering may eagerly evaluate right-hand operands before branching and use presence rather than Boolean truth in statement context.
- Acceptance runner can attach successful evidence to a record whose Boolean assertion failed; historical F01 contains contradictory status fields.
- Existing simulator primary target selection always chooses V2, so signing a Legacy counterpart is insufficient evidence of Legacy execution.
- Existing editor string/count heuristics are not one-to-one action or parameter verification.
- PNG byte scans cannot prove pixel secrecy; publication must fail closed for unproven artifacts.

## Protected delivery

No release candidate or project completion is claimed. Keep all required unrun gates open. Every later milestone must append its exact implementation SHA, actual commands, evidence scope, Asana update and limitations. Broad owner authorization does not remove technical acceptance gates or private-data constraints.

## Checkpoint — baseline and catalog, 2026-11-18

- Baseline build and full Go suite on `ca8a62327e429d5574c8f2a5d440de2c5e553ab2`: both exit 0. Main package 370.782s; 20 tested packages pass; 5 packages have no tests. Job ended 2026-11-18T01:51:33Z.
- Catalog generator integrated as `239d4a6d0b68d316f50bc1857a366b4928327f94`; generated evidence `3ff84b1fa3c000d8796a3a722da339c315bc8239`.
- Fresh 505 catalog execution: 38.45s, Legacy/V2 compile and structural routes pass; 497 callables, 382 unique native identifiers, 8 constructs. This is not independent Apple native/runtime proof.
- Twelve catalog negative/positive tests pass; final source needs evidence refresh.
- Asana existing tasks `1219603546838978` and `1220085240857296` updated with exact scope and left OPEN.

## Checkpoint ? integrated implementation and actual evidence, 2026-11-18

- Baseline private-corpus gates completed locally at exact P14 `ca8a62327e429d5574c8f2a5d440de2c5e553ab2`: TestP10R10CorpusMeasure exit0/149.67s; TestP11_WIRE_01_Fresh38x2NativeActionWire exit0/248.12s; TestP13_CorpusFidelityAndSourceSizeAudit exit0/176.42s. No private inputs or raw logs were published. New candidate corpus verification is still required.
- Core F01 implementation `e49d707fe5bbd1922834987e5de0503737687fbe`, corrected header-bearing edit semantics `9c15325acb22296e68ec870fffef0d3eda71ccb0`, and formal assertion scope repair `c2400eb1c227c388784900d3b68649bbc0d1cfc9` integrated. A header-bearing edited &&/|| case was reproduced failing and then passing through the actual hybrid import/render/compile path. Direct imported symbol presence remains preserved; new Boolean conditions use value semantics.
- Acceptance runner now distinguishes measured smoke observations from formal proof.86 historical main probes do not prove their full named contracts. All179 obligations remain explicit; unexecuted RP52/BRG33 cannot be minted as passing by ID references. Revised formal totals await the actual snapshot run; historical headline percentages are not release evidence.
- Shared build-profile implementation `8be3ea09563c530f49dd167ab97296185752d872`, mobile/Swift extension `311bac1c75dfad1fa5c8d4930d6213e11c7b97cb`. Production tests, six existing iOS Go integration checks and eight actual CLI cases passed. CLI Legacy explicitly used --legacy; --mode=legacy probes were rejected as Legacy evidence. Swift CABI compilation/runtime remains pending iOS Build.
- Inspector/privacy implementation `b199be57cbadc384a4fa962e42088a84aa06af84`, runtime hardening `104d57e9c31690444fd067cb4676d32222b0f9c2`.40 Python tests pass on the focused branch, including invalid order/schema/nonce callbacks and publication canaries. Root independently reran the first31tests and focused Go runtime tests on b199be5: both exit0 (5.02s and35.78s). HistoricalP14 callback compatibility is not new Apple runtime evidence.
- Skill correction `d664087d14d58fdfa57d57d7dbea05847837b1ac`: nine causal real-compiler checks plus existing10installation/cache/pin scenarios pass. Actual installed signing audit `4ad9813cde1cc7c226b0bb3f93b8ef5bce0b4ac6` records old installed false successes (12/46cases) separately from current-source rejection controls (34checks,0failures). Current signing.go already rejects the tested faults; deployed tuple remains old.
- Live Pure/Pure+Pack/Fork unsigned mode isolation evidence `f1788ba08c946b80d131eb460351d3f2d5b4326c`: four expected outcomes and checksum guards pass. Pure upstream and package files were not changed.
- Asana handoffs appended to QA1219603546838978, catalog1220085240857296, Inspector1220655985907047, privacy1220646564229070 and Skill1219603547340941. They remain OPEN at their broader acceptance scope. Master execution status1220663200268350 is at risk.
- Next candidate gates: full integrated Go suite; exact-SHA Linux/macOS/Nix; actual signed Legacy/BoolList/F01 callbacks; full iOS bridge build/test; refreshed corpus and generated metadata; coherent consumer tuple. Remaining typed macros are implementation work, not falsely labeled external blockers. Physical iPad remains BLOCKED_DEVICE.
