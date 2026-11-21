# Apple component c543: original K2 results and independent intake

Source c543d9001e7b9c486fd441c2e775cfee3de5a927 was frozen on audit/cherri-p10r15-apple-k2-c543d900-20261121. Each workflow was dispatched once, attempt 1. This packet preserves the original failed observations as well as the successful Runtime PoC. It does not transfer a CI result to another integration source or grant full requirement acceptance.

| Workflow | Actual run | Result |
|---|---|---|
| Startup observation | https://github.com/davidpovarsky/cherri/actions/runs/44953909996 | Failure. BRG27, D03 and E01 failed under the unchanged 28-second startup clock; zero formal UI evidence records. |
| Signed FileUTF8 Simulator QA | https://github.com/davidpovarsky/cherri/actions/runs/44953915448 | Failure. Three receiver XCTest controls passed, but actual test02 never started. Destination resolution stopped after 6.659 seconds because the active Xcode-derived Simulator.app directory was absent. |
| iOS 27 Shortcuts Runtime PoC | https://github.com/davidpovarsky/cherri/actions/runs/44953920775 | Success. Original source-bound signed fixtures, runtime outputs, fresh clipboard reset, device cleanup and publication passed. |

## Independent Runtime PoC verification

Original GitHub artifact14728825702 is 136809 bytes, SHA256 980b1096780ec28875b6f3634edeed64c78af3e61f2372900c020b2511573246, matching the API digest. All 32 extracted original files were hashed. The exact c543 source replayed the source/signing/device/artifact publisher successfully, and independently replayed the fresh-reset coverage observer. The canonical formal exporter then recomputed all six original formal records identically from the 17 retained diagnostic rows and original clipboard bytes; 11 source files were byte-matched to Git blobs.

The resulting runtime facets are V01/V03, L02/L04, N01, FN01/FN02/FN03, S01 and F01. The main observed values include updated/initial captures, List indexing trace alpha:gamma:::::beta, numeric trace 2.5:1:11:0:1:0, function21/caller22, explicit false/zero/empty-Text defaults, omitted defaults and factorial3=6. The separate S01/F01 source-bound coverage observed exact Unicode bytes and nine short-circuit cases with the prescribed effect trace after a unique fresh reset. The owned iPhone16 iOS27.2 Simulator was deleted successfully.

These six records supply only their reviewed runtime facets. They do not establish RP12/RP14/BRG10, all numeric behavior, caller input, structured defaults, main return, dynamic Text case sensitivity, broader mixed-expression acceptance, UI success, or all179 acceptance. The historical broader diagnostic labels remain unchanged and are not formal authority.

## Failed observations remain active

Startup BRG27 still has an AX query in flight beyond the28-second clock. D03 observes an editor only after the failed wait; that late observation is not promoted. E01 has no approved creation tap. The original startup receipts, trace drops and failed logs remain in the archive.

File QA resolves active simctl to /Applications/Xcode_27.1.app/Contents/Developer/usr/bin/simctl; the tool exists, the derived host Simulator.app directory does not. This does not establish that CoreSimulator/headless XCTest is unavailable, particularly because the receiver XCTest controls passed. A separate bounded read-only inventory was dispatched once as run44959045886 on its diagnostic branch; its results are not included or assumed here. No N03 or nested runtime diagnostic was dispatched and no native run was repeated unchanged.

## Preservation

manifest.json lists every member's exact bytes and SHA256. originals.zip contains the four original GitHub artifact ZIPs, run/job/artifact API originals, original public logs, frozen input/ref/workflow receipts, and the independent replay receipts. Extracted duplicate trees are omitted because their original ZIPs are retained. No runtime fixture, test budget or expected value was altered during intake.
