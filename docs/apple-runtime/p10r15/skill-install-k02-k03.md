# Actual Skill installation and interrupted-update proof

The production installer now stages and validates an immutable compiler/docs pair before promotion. It requires a SHA-256 digest for a downloaded binary and verifies it before executing that binary. Local modified source/docs are preserved. The implementation stays within the existing Skill setup, compatibility and resolution scripts.

## Sources and observed failures

Implementation b1a1e232a4a13ad73c12d9fdc4dad2ae2b08728e fixes the actual late-docs overwrite observed on 8c42876. Its clean K02/K03/K04 formal producers passed in 513.828 seconds. These results describe the tests that existed at that commit.

Independent review subsequently produced two real counterexamples on unchanged b1a1e23: a TERM delivered immediately after a completed backup rename lost the previous compiler, and corruption of the newly promoted cache was masked by a healthy bundled binary. Both original failures are retained. That earlier K03 pass is insufficient evidence for the expanded contract.

Correction a08b538332590569337482ed2e8be4ed59ade169 records pre-mutation state and derives rollback from actual staged/backup files. The cleanup trap completes bounded rollback without being interrupted by a second handled signal. Final candidate validation addresses the promoted cache directly and cannot silently select a fallback compiler.

## Executed evidence

| Source | Actual producer | Result |
|---|---|---|
| b1a1e23 | Fresh source build with advanced remote branches but immutable compiler/docs pins; Go VCS revision and clean-build metadata | PASS |
| b1a1e23 | Actual compatible/incompatible cache, bundled and explicit compiler selection | PASS |
| b1a1e23 | 18 actual offline compatibility cases | PASS |
| a08b538 | Download checksum rejection before execution; late invalid docs; successful validated update; local-state preservation | PASS |
| a08b538 | Four command/promotion failure boundaries using real candidate and previous files | PASS |
| a08b538 | Ten actual TERM interruptions after backup/promotion renames, including fresh installation | PASS |
| a08b538 | Corrupted promoted cache with a healthy bundled fallback available | Rejected and previous pair restored |
| a08b538 | Clean formal TestSkillUpdateProductionEvidence | PASS, 435.79 seconds; wrapper 440.906 seconds |

The formal K03 producer runs all nine named production test families and verifies actual test discovery. The raw log digest is 3a5353bb21e1994c9f81f7ee456faa7d99cd764a74e7fdd22cb48cbaa24f5452. Structured receipts and earlier negative results are preserved under evidence/skill-install-a08b538; raw synthetic test logs remain available on the authorized development host.

Two development test-harness failures are retained: Windows Bash PATH initially did not activate a fault shim, and equivalent Windows/POSIX paths initially failed a string comparison. The harness now verifies the selected shim and file identity. Neither failed attempt is counted as rollback or binary-selection proof.

## Limits

This evidence covers handled process failures and signals at the tested rename boundaries. It does not promise survival of power loss or a simultaneously reading process observing every file as one filesystem transaction. Packaging (K01), the full structured edit/sign contract (K05), remote installed consumers and final release gates remain separate. The complete 179-case acceptance report must be regenerated from the integrated source; these isolated producer receipts do not modify its counts.
