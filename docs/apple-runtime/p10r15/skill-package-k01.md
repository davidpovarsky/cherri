# K01: committed packages and actual unpacked execution

Implementation 46299c418179326f2c2fa15c04bf08e999c1e483 replaces current-HEAD binary copying with exact pinned compiler packaging. The existing workflow previously copied its development binary into a Skill whose compatibility manifest declared a different compiler commit; capability compatibility alone cannot prove the package's source identity.

The standard-library packaging tool reads the exact committed Git blobs, including the license, and checks the real compiler's Go VCS revision and clean-build marker against the manifest pin. The runtime ZIP carries the binary, commit and SHA-256 receipts. Both source and runtime ZIPs have an exact file inventory. Verification compares archive bytes and modes against the trusted Git tree and compiler, without trusting only the archive's self-reported hashes.

## Actual acceptance

The clean 46299c4 formal K01 producer passed in 191.94 seconds. It built the pinned compiler in a clean normal clone, made and verified actual ZIPs, unpacked the runtime package into a new directory, and ran its real CLI self-test with an empty cache. The test exercises version/actions/catalog, unsigned build, decompile, recompilation, prepare-edit and signed-container rejection.

Eight actual ZIP corruptions were rejected: missing wrapper, unexpected file, duplicate path, traversal path, altered binary, nonexecutable binary mode, wrong compiler commit receipt, and changed source with a forged matching internal hash. A second real clean compiler built from current HEAD was rejected under the older manifest pin. An occupied extraction destination was preserved. Repeated identical inputs produced identical ZIP bytes.

The wrapper's ten mock installation cases are explicitly labeled as mocks. Its report now has no formal records; it cannot grant K01/K02/K03/K04/K05, docs or roadmap completion. Dedicated production producers own those requirements.

## Receipt framing correction

The Go test passed, but the first capture helper failed because test2json split one long receipt into two Output events. The raw passing stream is retained unchanged. Recovery concatenated only Output events belonging to the same test before decoding logical lines; it did not rerun the test or fabricate a result.

CI correction 41121a36c83c3fc609ba3a97251c19a171c91400 applies that framing rule and verifies the test PASS, exact source SHA and K01-only scope. Replay of the actual split stream passed; missing continuation, missing PASS, explicit FAIL, wrong source and wrong requirement all failed without writing a receipt. Independent read-only review identified the same framing defect.

## Durable package execution at 41121a3

| Package | Actual environment | Inventory | Outcome |
|---|---|---:|---|
| Source ZIP | Verified against committed Git files | 20 files | PASS integrity |
| Windows AMD64 runtime | Actual unpacked Windows compiler and shell wrappers | 23 files | PASS self-test, 110.266 seconds |
| Linux AMD64 runtime | Fresh clean pinned cross-build, actual existing Ubuntu/WSL execution | 23 files | PASS self-test, 18.453 seconds |

All packages use compiler 17ba3ca3387f204a0fd80b092624e21f311db8d4 and docs d8ad5e604c9e3585876ae95b0d1f628381a33c42, as declared by their exact Skill source. Windows compiler digest ce32d4c462013e402692194db1f8b3b2ec62ee4aa8bcbb68d91857cd93bad58d; Linux compiler digest 0daca37c8429d4fc49b6b102d5296027a024efd17246e8f15a1e80a64967ae97.

Source ZIP SHA-256 a058d5e1449fbf2ad127783408d5a0f7f81baed7188b9fe8b8f113305b283fd3; Windows ZIP 646832e1cff718bfcf9a688c2ab445903904a90d662e5f5a9c885299e55c9ce0; Linux ZIP f2e33d7a75f4af23fa0a7913d8d5cb7ee44bd8456208911e831e3e8cb8253f14. Durable archives and raw wrapper logs remain on the authorized development host under skill-package-k01; repository receipts include their hashes.

## Remaining gates

The existing CI workflow now builds the manifest-pinned ARM64 compiler, verifies the ZIP and runs the self-test after extraction in its ARM64 container. That workflow has not yet executed for this source. The local Docker Linux engine was unavailable; the Linux result above is actual AMD64 Ubuntu, not ARM64 emulation.

No live consumer was upgraded. This staged tuple predates forthcoming native-envelope/schema and edit/sign changes and must be rebound and reverified for the final candidate. Full allowed-delta editing/signing, installed remote consumers, Apple runtime and project release remain distinct.
