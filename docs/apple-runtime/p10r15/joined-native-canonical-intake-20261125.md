# Canonical joined native evidence intake — 2026-11-25

## Qualified change

Production commit: `b50aedb203cdbadb1d478dc7a6c6157261896946`

The canonical manifest path now uses the existing original-log validator for exactly three dedicated native records:

| Requirement | Tier | Dedicated producer |
|---|---|---|
| BRG25 | ios-build | CherriJoinedBridgeTests/testBRG25ActualMobileRouteJoinedContract |
| BRG25 | ios-ui | CherriJoinedContractUITests/testBRG25ActualAppRejectedAndLiteralRoutesUI |
| BRG27 | ios-ui | CherriJoinedContractUITests/testBRG27ActualEditorJoinedWorkflowUI |

BRG27's dedicated C ABI bridge is mandatory companion evidence inside its UI proof. It does not supply the service, integration, LSP, or UI tier by itself. Existing generic bridge and UI probes remain unauthorized.

## What changed

The already-installed canonical manifest validator calls the joined-public validator. Its native branch now delegates to the dedicated native original validator instead of rejecting every native record as unenrolled. Local process validation is unchanged.

The requirements map enrolls the exact three method identities and their precise UI assertions. It declares the actual native source inventory. The execution audit retains linked dedicated evidence identities when adding the local test inventory; declaration identities still do not add executed tiers or close a requirement.

The original validator remains strict: current implementation and checkout SHA, successful workflow and attempt, four immutable original artifacts, artifact digests, exact current Swift fixtures and source pins, ordered log stages, original test boundaries, companion case, and exact collector summary must agree. The patch does not relax these checks or the original required tiers.

## Actual causal and regression executions

Test-only checkpoint `ab163c018b7bafee7a253a40c6b4c38405324d29` reproduced all 17 new failing leaves through the actual canonical manifest and audit paths. Its 54 existing original-validator controls passed. The 17 leaves and their four parents were retained unchanged for qualification.

The final actual Oracle execution passed **127 named events, zero failed and zero skipped**, with no timeout, capacity stop, or source change:

- 21 new canonical events, including 17 leaves and four parents.
- 54 existing dedicated native validator and collector events.
- 47 existing joined-public inventory, immutable-contract, local-tier, and corruption events.
- Five existing partial Apple probe rejection events.

These are synthetic intake tests executed in isolated temporary directories. They are not native runs and do not provide BRG25, BRG27, or full 179-requirement acceptance credit. Native-only healthy controls explicitly retain missing local obligations; corrupted original logs, proof claims, and borrowed producer names acquire no executed tier.

The first green attempt stopped before Go execution because the RAM-cache admission floor was not met. It remains recorded as NOT_RUN. The subsequent run used the same source and unchanged tests after exact verified restoration of the owned cache to persistent storage.

## Original evidence

Base: /mnt/development/projects/evidence/cherri-p10r15-final-completion-20261125/

| Evidence | Path | SHA-256 |
|---|---|---|
| Causal RED | `joined-native-canonical-red-01/result.original.json` | `57d6faafd72defad16498b12aff694cea75594f8374727cbdad79f116ead6af6` |
| Implementation publication | `joined-native-canonical-implementation-01/result.original.json` | `cf967e36ea289aa34abd02350db177e4cddb71ffed5254f074a1d7c8d183652e` |
| First admission, NOT_RUN | `joined-native-canonical-green-01/result.original.json` | `25c9e5e1fef5afffcb54cc560d3543971ebac72c598090f2a8c4d1a03cf56859` |
| Actual GREEN | `joined-native-canonical-green-02/result.original.json` | `b520864a57029c64e31f0b263e85911993b09bfdfe0e2282fb48e590b08343a1` |
| Exact persistent cache restoration | `owned-go-cache-persistent-restoration-01/result.original.json` | `50d53c189982956776f7d686091b236f7d6e9baef285d4ac1b11a81e91384e05` |

The execution receipt retains the original Go event stream, exact command and owner, complete tracked physical source inventories before and after, resource floors, and discovered test identities. No production evidence was fabricated or restamped.
