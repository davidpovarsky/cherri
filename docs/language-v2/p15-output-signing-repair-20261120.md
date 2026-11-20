# RP43 output/signing repair and Linux evidence preparation

Date: 2026-11-20. This report preserves component evidence; it does not close RP43 or the 179-requirement project. Actual Linux execution is still required at this checkpoint.

## Source and result

The production repair is commit `b351ba06c0728197376955629dfd4701c7a6a153`, on the isolated RP43 branch based on `8ba17fcccade143d21adf21a28d8ad12fcfaf73a`. The strict producer and bounded manual workflow are commit `468392aae763fe22ef7f6dcde4dcdc6e7f489e91`.

On clean `b351ba06`, a freshly built compiler passed 127 named new and existing output/signing/CLI compatibility tests, with zero failures or skips, in 18.547 seconds. The unchanged official Legacy Parity gate also passed all 47 fixtures (9 verified parity, 38 proven equivalent) and 17 of 17 actions in 9.547 seconds. These observations include `globals.cherri`, `standard.cherri`, and `menus.cherri`.

On clean `468392aa`, the new intake inventory, causal parser controls, and actual Windows host guard passed 97 named events in 7.532 seconds. The host guard requires RP43 to remain `NOT_RUN`, with zero evidence records, on Windows. These 97 controls are not Linux permission evidence.

## Reproduced defects and bounded repair

Four public routes are tested independently: `compile` V2, file-invocation V2, `compile --legacy`, and file-invocation Legacy. The original Legacy path wrote the source-relative unsigned sibling before moving it to the requested destination. Its fallback ignored a failed destination write and removed that sibling. This deleted or replaced preexisting user bytes and could return success without creating the selected output.

The first clean baseline (`622e9d09d56091e3418c7d05efa48c9b03557384`) has eight genuine failing Legacy leaf cases, including both successful explicit-output sibling preservation and impossible destinations. The corrected signer baseline (`da537c66bc899f3734cd22a52ed2508fb84080f8`) has twelve independently reset Legacy preservation failures: six real loopback signer faults on each of two Legacy routes. The encoder baseline (`51a8733d8670d2254caecca0b9c95cf190d40cb3`) proves a real encoder error truncated an existing selected output to empty bytes before returning nonzero.

The repair changes only `output.go` and the new `legacy_output_transaction.go`:

- The existing native encoder completes into memory before the selected file is atomically published. Encoding errors return without opening or truncating that destination.
- Explicit unsigned output writes directly to its requested destination. The existing default unsigned naming rule is preserved.
- Signed Legacy output stages its unsigned input in an owned temporary directory beside the destination and uses the existing signing transaction. It never selects a preexisting source-relative sibling as scratch.
- Temporary cleanup runs before the error reaches the existing CLI handler. Ordinary cleanup is observed by the tests. The deferred removal error is not surfaced; this does not claim cleanup under an independently injected removal/permission failure, power loss, or concurrent interference.

No action definition, serializer policy, signing verifier, Legacy signing default, or immutable acceptance contract was changed. Bare Legacy/macOS signing behavior remains documented compatibility behavior.

## Retained test-development failures

The archive retains every original attempt rather than replacing failures with later passes:

1. The first security attempt expected bare Legacy file invocation to be unsigned. Both existing fallback remote requests were intercepted and rejected by an owned loopback proxy. Repository documentation supports the old Legacy behavior; only the new test was corrected to use explicit `--skip-sign`. This is a fixture expectation error, not a compiler defect.
2. The first encoder subprocess test required exit code 1, while the existing Go-error handler exits through panic with code 2. The requirement is a completed nonzero failure. The corrected test requires a positive nonzero exit plus the observed encoder marker and actual encoding diagnostic; the next clean run then exposed real output truncation.
3. The first intake draft did not compile because its new registration hook returned an unavailable variable. It executed zero tests. The hook was corrected to append to the existing validation-error collection, and the original raw build failure remains archived.
4. One upload script failed Python parsing before executing any statement, due to source-string substitution in the launcher. The complete failed launcher/result is retained separately with `NO_TEST_EXECUTION`; the corrected launcher first verified the unchanged clean source before applying the draft.

The local precommit 73-pass repair run and 97-pass intake run are preserved as draft observations. Clean-source receipts above are the component verification evidence.

## Exact RP43 producer

The producer binds the immutable RP43 title, scenario, expected observation, and the `integration` and `security` tiers. One actual Linux subprocess must execute all six selected root tests and all 78 named parent/child events. The source inventory contains 47 immutable Git blobs, including production entry points, real test helpers, both acceptance contracts, strict intake helpers, registrar hooks, and the manual workflow.

The full scenario includes real same-identity write denial followed by restored permissions; destinations with spaces and Hebrew names; directory, missing-parent, and parent-is-file failures; real XML/binary encoding faults; six actual observed signer faults on five routes; exact prior destination/input/sibling preservation; and unsigned/check filesystem, network, and process boundaries. The signer operates only on public synthetic fixture bytes through owned loopback HTTP receivers. A healthy `AEA1` protocol response is explicitly classified only as `AEA1_HEADER_PASS`, not an Apple signature or runtime result.

Intake requires exact source, command, selected nested event ordering, original byte digests, process interval, clean before/after state, successful exit, no timeout, and both tiers. Causal controls remove required events, reorder route parents, alter source hashes while reconciling the outer artifact, corrupt raw bytes/timing, change the host, and attempt linked-ID inheritance. No imported archive acquisition path exists. The final required CI gate remains independent; this component cannot close the full project.

## Original artifacts

The accompanying evidence directory contains `original-receipts.zip`, its complete member manifest, immutable source inventory, and this stage's review/result summary. Original bytes remain unchanged within the ZIP; compiler binaries are retained at the owned evidence path and represented by their exact digest in the build receipt.

Key digests:

| Observation | SHA-256 |
|---|---|
| Original output failure raw | `1eefadf345112c70b60c2fda42eed60ab2d517d1ba738d1360d8ac037f4ef895` |
| Original security raw, including policy fixture error | `ed3a1f6f5f90a7b23f779e6297459a1e6a901ae807503c1f7004d31c637ced41` |
| Corrected independent signer baseline raw | `b2b0c528cba930f74c4889b767de3573d31f64eef167ea65a2189316fd50a13f` |
| Actual encoder truncation baseline raw | `1b7659f990868e506b814fe5bed5f14150f99cb3b7262866bd70cf18add9bac2` |
| Clean repaired 127-event raw | `7addaac4e5fc6afb3aa9a75aab3eb9bf71012f06f1bdb2d550b396832e4449bb` |
| Clean repaired official parity report | `c406ca67dfd2e8d0a33d3fb53f7d133efe93e9d0630a7d90ebef20602efeb756` |
| Fresh repaired compiler | `7b34561b1c6989396d0bb9002401e9f08f8c5c83a2fbf5bad62c755024cb028b` |
| Clean intake/Windows guard raw | `4bd6c3bfc6142ee7925ce07355616b063dff4da3c0deb7b54b8c7b3db8592f55` |
| Manual Linux workflow | `6b5b4abab47db1ec79841ebc750eabb68131d6713c48d38174e43099cf484f28` |

Independent source review found no concrete destination-preservation or intake/no-inheritance blocker. Review is distinct from execution. A separately reviewed, exact-source Linux dispatch must supply actual permission execution and strict complete process evidence before RP43 can pass at that source. Final integrated acceptance must rerun on its own immutable source.
