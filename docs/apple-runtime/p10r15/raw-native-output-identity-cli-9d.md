# Raw native output identity: committed CLI verification

## Observed result

All **36 actual CLI contracts passed** on clean compiler source `9d5ff641e1aa04301b0267aaa4a6c753c596e52e` at 2026-11-18T17:17:27Z. The binary was built with Go 1.25.6 for Windows amd64. Its embedded build metadata records the exact commit and `vcs.modified=false`.

Compiler binary SHA-256: `5ca0a807b41c03bed543342879e806fbf033aeb18ea3240645c8ba0402511bf1`.

| CLI result | Cases | Required evidence |
| --- | ---: | --- |
| Successful compilation | 22 | Real plist parses; exact raw parameters remain unchanged where required; each source-generated reference resolves to one producer |
| `NATIVE_OUTPUT_ID_MISSING` | 10 | Nonzero exit, exact diagnostic, no output artifact |
| `NATIVE_OUTPUT_IDENTITY_CONFLICT` | 2 | Nonzero exit, exact diagnostic, no output artifact |
| `NATIVE_OUTPUT_AMBIGUOUS` | 2 | Nonzero exit, exact diagnostic, no output artifact |

The 32 source specimens come from the committed `rawNativeResultCasesJSON` table in `p15_raw_native_result_test.go`. Four additional CLI cases verify the Debug and Release boundary. Debug trace consumption fails for an exact raw producer without a UUID; a removed Release trace leaves exactly one unchanged raw producer; a real Release output and a preserved Release `require` still fail for the missing UUID. The ordinary raw producer, explicit UUID, alias, mutable binding, f-string, dictionary, list, date backfill, lowercase identity, duplicate producer, and opaque invalid-input controls all retain their individual contracts.

## Reproduction and provenance

A clean, detached ordinary Git clone of the exact local commit was used. Build output was written outside the clone, and the clone remained clean:

```sh
go build -buildvcs=true -p 2 -o <artifact>/cherri-clean-vcs.exe .
go version -m <artifact>/cherri-clean-vcs.exe
```

For each fresh source file, the actual executable was invoked with:

```sh
cherri-clean-vcs.exe <case>.cherri --skip-sign --output=<case>.shortcut --build-profile=debug
```

The four profile specimens use their declared Debug or Release profile. No stale output file was accepted. The verifier parsed the actual `.shortcut` bytes, collected top-level producer UUIDs and consumer ActionOutput references, and checked the exact explicit identity and opaque-import controls. Nested `WFWorkflow` dictionaries retain their separate identity scope.

The adjacent JSON receipt records all 36 source hashes, exit codes, expected diagnostics, log hashes, and successful plist/action-JSON hashes. Raw synthetic CLI inputs, outputs, build logs, and the verification script remain in the owned ignored evidence directory. Two earlier linked-worktree builds omitted VCS fields despite succeeding; they were excluded from this proof, and no provenance requirement was relaxed.

## Scope

This is compiler and serialized-wire evidence. It does not establish native Apple execution, physical-device behavior, or complete Inspector mapping. The queued signed diagnostics at frozen source `4e7956d` remain separate. The prior library-test receipt also covers preservation of typed opaque values and three causal finalization corruptions; those checks are not relabeled as CLI cases here.
