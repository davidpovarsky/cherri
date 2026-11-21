# Xcode 27 read-only inventory diagnostic

This branch is a one-run diagnostic based on immutable source
`c543d9001e7b9c486fd441c2e775cfee3de5a927`. **Do not merge its replacement
Simulator QA workflow into an integration or release branch.** The registered
workflow path is reused only on `audit/cherri-p10r15-apple-inventory-20261121`.
The Apple agent owns the single authorized dispatch and original artifact intake.

## Scope

The collector executes eight fixed read-only commands: Git HEAD/status and
receiver-project tracking, current Xcode selection, xcrun's simctl/xcodebuild
paths, and JSON simctl device/runtime lists. It inspects only
`/Applications/Xcode*.app/Contents/Developer/Applications`, the corresponding
Xcode/Simulator Info.plist files, and the resolved tool identities. It reads
only the DEVELOPER_DIR environment value; it does not dump the environment.

The existing source contains the receiver's project.yml and source files but
no tracked CherriSimulatorQA.xcodeproj. Its absence is recorded. The inventory
does not generate a project or run showdestinations against an untracked one.
It does not change Xcode selection, boot/open a simulator, build, test, install,
delete, inspect device contents, or run Go.

Host Simulator.app layout and CoreSimulator device/runtime state are recorded
separately. Missing GUI application files do not establish that headless XCTest
or the runtime is unavailable. This diagnostic grants zero formal requirement
records and makes no native-test success claim.

## Bounds and privacy

The collector has a 150-second aggregate alarm, a 125-second collection window,
fixed command ceilings of 5/10/25 seconds, bounded process-group termination and
stream collection, and at most 20 seconds for encryption within the aggregate
deadline. The workflow has a five-minute infrastructure limit. Each stream
retains at most 2 MiB plus the overflow witness; overflow and timeouts are
explicitly incomplete. Up to eight Xcode bundles and 32 Developer/Applications
entries per bundle are observed. Metadata files are limited to 1 MiB and tool
identity reads to 64 MiB. Source HEAD and clean status are checked against the
actual workflow commit. An unexpected source is reported without acceptance.

All original command stdout/stderr, process receipts, paths, device names,
UDIDs, runtime identifiers and metadata remain in the private raw directory.
Their exact inventory is zipped, then encrypted with the fixed, dedicated
public certificate using OpenSSL SMIME, DER and AES-256. The private key is
host-local outside Git. The plaintext ZIP SHA-256 permits exact post-decryption
reconciliation; encryption is not described as a provider signature or timestamp.

The workflow uploads **only** `inventory.zip.p7m` and `summary.json`. The summary
is an explicit allowlist of status codes, counts, source/run identity, budgets,
certificate/ciphertext/plaintext hashes and scope flags. It never copies raw
metadata, command arguments, names, UDIDs or paths. Encryption failure cannot
fall back to plaintext upload. Raw encryption diagnostics are not printed.

## Offline validation and actual result

The checked-in controls exercise real synthetic subprocess stdout/stderr,
nonzero exits, a short timeout and output overflow, plus exhausted deadlines,
bounded reads, layout limits, missing GUI vs present runtime data, public
projection, exact archive inventory and encryption failure/recipient guards.
They invoke no Apple command, compiler or provider. A separate actual OpenSSL
encrypt/decrypt control must reconcile a synthetic ZIP byte-for-byte using the
dedicated certificate before dispatch. Local controls do not prove macOS
runner behavior; the actual inventory result remains pending until the single
authorized run is downloaded, decrypted and inspected.
