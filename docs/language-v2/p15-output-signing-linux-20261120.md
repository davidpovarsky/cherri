# RP43: actual Linux output and signing component

Date: 2026-11-20. This is bounded component evidence, not final P10R15 closure.

## Observed result

The complete RP43 integration and security scenario passed on actual Linux at immutable source `53daeae947f729e434168a8a09661e735386c661`. The workflow executed all 78 required root test events and 253 outer producer/intake events with no failure or skip. Both RP43-only tier records passed strict local intake; the report explicitly retains `closure_complete: false`.

| Identity | Original value |
| --- | --- |
| Workflow | P10R15 CLI Output Evidence, ID `424796388` |
| Manual run / attempt | [44705394599](https://github.com/davidpovarsky/cherri/actions/runs/44705394599), attempt 1 |
| Actual Linux job | `136183611457` |
| Source and frozen probe | `53daeae947f729e434168a8a09661e735386c661`, `probe/cherri-p10r15-cli-output-evidence-20261120` |
| Original artifact | `14614159034`, 31,146 bytes |
| Artifact ZIP SHA-256 | `f1f22ee5acd3e6db5ec1b6ec75a353ad4310faeb7f8b010d1f64e659cb348c60` |
| Original job log SHA-256 | `d8f17914015b868f27802e2b7922679f4c4f6b4a55fb4d79423886c8b343de88` |
| Actual toolchain | `go version go1.25.14 linux/amd64` |
| Outer elapsed time | 59.821268243 seconds, exit 0, no timeout |
| Inner elapsed time | 40.446206683 seconds, exit 0, no timeout |
| Outer raw Go JSON SHA-256 | `82ea3ec53034d77474662f5f7c1c63a58785996dc481a196bebb5d019d5ab805` |
| Inner raw Go JSON SHA-256 | `419c52e2e40ef1d083b2bb821c1fcf7d088f1643e080336080684fb9a6447241` |
| Workflow blob SHA-256 | `6b5b4abab47db1ec79841ebc750eabb68131d6713c48d38174e43099cf484f28` |

The probe was pushed once. Push run `44705293782` registered the workflow only. A separate, single manual dispatch produced the actual Linux run above. Original request/result/verification files preserve that distinction. The registration-only job is intentionally skipped during the manual run; every step in the actual Linux job succeeded. No semantic test is counted from registration.

## What actually executed

The six root parents cover complete independent observations:

1. Four public V2/Legacy compile routes exercise absolute and relative destinations containing spaces and non-ASCII text, destination directories, a file used as a parent, and missing parents.
2. All four routes execute actual Linux permission denial. The same test process first proves it cannot write through a directory with mode 0500; a privileged process that could write would fail the control. Each route preserves original files, reports the error, and succeeds after permissions are restored.
3. A separate process invokes the real Legacy encoder with an unencodable value and proves the preexisting destination is not truncated. Canonical XML and binary encoder faults return no partial bytes, with healthy recovery controls.
4. Five public signing routes each encounter six independently reset faults from an owned loopback signer: HTTP failure, explicit JSON rejection, malformed JSON, unsigned XML returned as signed, a header-only result, and a truncated result. The receiver checks the exact signing input; the tests retain the destination and unrelated source sibling bytes and observe recovery.
5. Documented unsigned defaults and explicit Legacy `--skip-sign` do not contact the observed network or launch the controlled command observers. The `check` route preserves its analysis-only behavior even with hostile output/signing flags. A positive explicit remote-sign control proves the receiver is live.

The successful synthetic signer result is classified only as `AEA1_HEADER_PASS`. It does not prove an Apple signature, native execution, or an external signing-service deployment. Documented bare Legacy/macOS signing defaults are preserved. Ordinary private staging cleanup was observed; deferred cleanup does not establish power-loss or concurrent-reader atomicity, nor guaranteed cleanup under an injected cleanup failure.

## Independent intake audit

After download, an independent read-only audit verified the original run, job, attempt, source and artifact identity; the ZIP hash matched the GitHub artifact digest. It verified all eight artifact members, both raw streams and stderr hashes, exact subprocess commands, finite process intervals and clean source before and after execution.

All 78 required inner names had one actual `run` and `pass`, with no absent child, skip, failure, duplicate or completed parent preceding its required child. The 253 outer names also had complete run/pass pairs and the expected three parents. The single assembled production-receipt marker matched `component-proof.json` exactly. The two records both name only RP43 and bind the complete scenario to its integration or security producer.

The audit read all 47 listed source files directly from the immutable Git commit using `git --no-replace-objects cat-file --batch`; every original SHA-256 matched. It separately checked the exact executed workflow blob. No source mutation or semantic rerun was needed for this audit. Original stderr contains dependency-download messages and is retained without normalization.

Original files are retained in `evidence/p15-output-signing-linux-20261120/original-receipts.zip`. `archive-manifest.json` hashes every retained member; `source-manifest.json` records the immutable Git blob and SHA-256 inventory. `summary.json` records the actual local result and its open final gates. The earlier Windows, failed baseline and fixture-error receipts remain unchanged in [the causal repair report](p15-output-signing-repair-20261120.md).

## Integration boundary

The source chain from base `8ba17fcccade143d21adf21a28d8ad12fcfaf73a` is:

| Commit | Scope |
| --- | --- |
| `622e9d09d56091e3418c7d05efa48c9b03557384` | Output destination causal tests |
| `069dc89647f74bc7982ef853a7c2dc4104a3fb2c` | Encoding and signing causal tests |
| `da537c66bc899f3734cd22a52ed2508fb84080f8` | Preserve documented Legacy policy and probe encoder output |
| `51a8733d8670d2254caecca0b9c95cf190d40cb3` | Correct the new encoder control to require documented nonzero failure |
| `b351ba06c0728197376955629dfd4701c7a6a153` | Encode before atomic publication and isolate signing input |
| `468392aae763fe22ef7f6dcde4dcdc6e7f489e91` | Strict Linux producer, causal intake controls and manual workflow |
| `53daeae947f729e434168a8a09661e735386c661` | Preserve pre-Linux proof; exact executed source |

Production source `b351ba06` previously passed 127 focused and compatibility events plus the official 47 Legacy parity cases and all 17 action checks. Those original results remain bound to that source, separate from this Linux result.

The actual Linux component supports RP43 at the exact source above. CL01, CL02, CI04, BRG26 and packaging requirements receive no inherited credit. The integrated source must independently rerun the complete formal 179-row acceptance and required CI. Apple native proof and final compiler/documentation/consumer publication remain separate obligations. No final manifest or consumer pin changed during this component.
