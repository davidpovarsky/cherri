# E01 contextual completion: actual service repair

Tested source: 73d0c7b3b914d943b4200139dda018f5d6ed02ed. Base: 1bb5cd1cbd742a04821943bc0f0b44b84aa9a6b9. This component fixes expected-type variable completion and parameter ownership in the shared service. It does not complete E01's required UI tier or any full179 row. A dedicated formal producer is a separate next step.

## Actual defect and repair

The original service offered the same full visible variable set in Number, Text, Bool, List, Optional and alias argument contexts. It also exposed forbidden main-scope captures inside functions and offered already-bound primary labels. The clean expanded baseline independently confirmed19 failing leaves. The analyzer now captures initialized, flow-aware lexical types at the exact resolved callee and written argument slot. Completion uses existing types.HasAssignmentEvidence; it does not invent a second type parser or infer a type from a matching global name. Parameter labels use the same actual callable ownership and primary binding. Existing canonical parameter documentation is preserved.

A separate clean replay exposed one qualified-module case: an incomplete named value erased otherwise valid imported signatures because the service skipped all module resolution when any entry parse error existed. Service analysis now retains valid import context while preserving the entry diagnostics and the unchanged rooted dependency validator. Actual remote/outside/malformed-dependency cases reject, issue no loopback requests and write no artifacts.

## Retained failures and test expectation corrections

Every original attempt is retained byte-for-byte. Initial function-candidate expectations mistakenly included a main variable that FN03 forbids capturing; the corrected source excludes it and independently analyzes every offered insertion. Another original expectation wrongly treated canonical number(...) input as Number. Its exact canonical declaration says AnyContent input with Number result; round(...) is the strict Number-input control. Source/registry assertions and actual inserted-source analysis establish this correction independently. The original failed test and draft were not rewritten.

The expanded clean baseline at3bbea9f2 includes the corrected broad/strict canonical controls and yielded19 genuine leaf mismatches plus2 failing parents. The later2f66c81a clean attempt retained the qualified named-argument failure. Dirty draft runs are explicitly identified below and are not substituted for the final clean result.

## Exact executed evidence

| Attempt | Source state | Exit | Named PASS | Named FAIL | Named SKIP | Seconds |
| --- | --- | ---: | ---: | ---: | ---: | ---: |
| e01-baseline-2a4230d9 | 2a4230d9 clean | 1 | 6 | 10 | 0 | 5.922 |
| e01-corrected-baseline-6809ba06 | 6809ba06 clean | 1 | 16 | 10 | 0 | 5.797 |
| e01-first-repair-draft | 6809ba06 dirty | 1 | 24 | 2 | 0 | 5.860 |
| e01-expanded-baseline-3bbea9f2 | 3bbea9f2 clean | 1 | 35 | 21 | 0 | 7.282 |
| e01-expanded-repair-draft | 3bbea9f2 dirty | 0 | 56 | 0 | 0 | 6.453 |
| e01-initial-clean-2f66c81a | 2f66c81a clean | 1 | 154 | 2 | 0 | 6.281 |
| e01-recovery-clean-8da4c027 | 8da4c027 clean | 0 | 160 | 0 | 0 | 9.031 |
| e01-final-clean-73d0c7b3 | 73d0c7b3 clean | 0 | 163 | 0 | 0 | 12.563 |

The final command was:

    go test -count=1 -json -timeout=3m ./internal/language/service ./internal/language/analysis ./internal/language/lsp

It executed163 named PASS,0 FAIL,0 SKIP, exit0 in12.563 seconds. Source was clean before and after. Original raw149322 bytes SHA2561b5431cb315ca3847cd85642965c1de962bd1e81a482672e523d73c27c0009be. The process environment override was exactly GOMAXPROCS=2; the test budget was unchanged and no timeout was promoted to success.

New scenarios cover typed and broad input distinctions, Optional narrowing/reassignment, uninitialized bindings, lexical shadowing, loop/function/default scopes, written named-argument order, labels/enums, defining-module aliases, partial qualified identifiers, unknown exports, copied metadata, reset/reopen isolation and canonical parameter metadata. Actual completion outputs are corrupted with missing, unrelated, duplicated and wrong-kind candidates. Qualified completed-source controls use the real analyzer. Existing full service/analysis/LSP tests also ran.

## Proof and remaining work

evidence/p15-e01-editor-context-20261120/provenance.json binds every original process/script/source-snapshot file and immutable Git witnesses. The ZIP contains the same original bytes as the individual files. The three immutable179 contracts are byte-identical to the base. All wrapper receipts preserve the actual child exit code; a successful Sentinel wrapper is not counted as a successful failed Go child.

No compiler lowering, codec descriptor, consumer tuple, UI, Simulator, performance benchmark, complete179 report or unrelated requirement is credited by this component. E02/E03/E05/E06 and the strict per-row evidence producers remain separate work.
