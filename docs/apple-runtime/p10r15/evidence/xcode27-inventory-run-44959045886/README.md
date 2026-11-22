# Xcode inventory: actual run 44959045886

The single read-only inventory workflow completed successfully at execution
commit cbdd417d8e7ef6d6ef472e2db3d8cbdbfab7ccdf, attempt 1. Collector source
915d99bafd01097f43f6fb247f79f39452e5e327 and its prior 14-control/encryption
proof are a separate boundary. This packet grants no Apple or release acceptance.

Run: https://github.com/davidpovarsky/cherri/actions/runs/44959045886

All eight fixed commands returned success with complete bounded streams. The
original GitHub artifact ZIP and encrypted inventory bytes are retained, with
provider API receipts, exact dispatch/ref bindings, and the decryption receipt.
The latter reports 35 verified manifest entries across 36 original files. No
decryption is repeated by this packaging step. Private plaintext, device names,
device identifiers and private-key material are excluded from the public packet.

Independent aggregate extraction from the hash-bound original layout and simctl
outputs found eight Xcode entries/aliases resolving to three physical roots.
Their Developer/Applications directories and standard Simulator.app directories
were absent. The active simctl was a regular file inside the selected physical
Developer root. Separately, simctl listed 40 available devices, all Shutdown,
and 10 available runtimes. These are inventory observations, not a headless
execution result. GUI absence does not establish CoreSimulator unavailability.

The receiver project is generated during normal QA and was not present in this
read-only checkout. showdestinations therefore remained explicitly not started.
There was no boot, build, import, native XCTest or test02 attempt in this run.
The original c543 File run 44953915448 remains failed with test02 not started;
the separately reviewed headless preparation candidate needs its own actual
destination and test evidence. No N03 or nested List semantic claim follows.

`manifest.json` binds every public archive member. `observations.json` contains
only fixed safe counts/flags/hashes recomputed from the preserved originals.
The plaintext ZIP hash is published solely for private reproducibility; the ZIP
itself and its decryption key remain outside version control. Source and workflow
changes on the inventory branch are not part of this documentation-only commit.
