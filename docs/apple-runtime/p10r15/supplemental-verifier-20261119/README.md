# Supplemental verifier execution evidence — 2026-11-19

The clean production execution is bound to compiler repository commit `8d64ef5e2b02b7e4f1d96d97295f923030051dbf`. It executed the full RP01, RP02, RP03 and BRG04 scenarios through their independent registered producers: four requirements, seven tier records, 48 outer named tests/subtests, zero failed or skipped tests. The report retains all original 179 obligations and leaves release closure incomplete.

The causal baseline used production `main.go` and `manifest_generator.go` from `7f7a37b1108fc1dcdd1e8ae3c887742de1b1d3a6`, with the new mutation tests present. Its valid manifest control passed before each mutation. Fourteen invalid-record variants were incorrectly counted as a requirement success, and four malformed declared producer payloads were silently ignored. Including their parent tests, the baseline recorded 20 failed tests/subtests.

The first fixed run passed 71 tests/subtests and exposed two additional gaps: explicit partial local NOT_RUN was rewritten as FAILED, and fresh Windows checkout line-ending conversion changed immutable contract bytes. The final source preserves the actual partial levels while rejecting final closure, and pins LF for the three immutable contract files. The clean RP03 producer clones with `core.autocrlf=true`, verifies the original digests and 94/52/33 counts, builds the actual CLI, and loads repository defaults and explicit paths from an unrelated working directory.

Comparator diagnostic validation separately passed all 56 package tests/subtests after preserving both JSON mismatch values, including zero, false, empty text and a missing side. This earlier run was on the declared dirty development tree; it is not relabelled as a clean commit run.

All validator fixture runtime/UI artifacts and their source identities are synthetic test inputs. These tests establish validator behavior, not FN01, E01, P01, Apple Simulator or physical-device acceptance. The full supplemental wrapper including RP35/RP36/RP37 still requires a final integrated rerun with the rebound compiler/docs/Skill tuple.

`archive-manifest.json` gives byte lengths and SHA-256 hashes for the original retained logs, summaries and clean receipt. No private corpus data is included.
