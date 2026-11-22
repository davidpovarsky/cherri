# Function-usage scan: preserved semantics and bounded public comparison

## Status and source

Implementation `0f50e542a20a580db1f62d99dbb96a90149762a8` replaces only the
Legacy `checkFunctionUsage` discovery loop. The unchanged regex oracle and public
comparison fixtures were first committed as
`5dbfd07a4931c031dfe50e66984b9c025d3d21ef`, based on
`b697d12ab68696e7860decaab8381515397dcbc7`. Both measured sources were clean
before and after execution. This work used a real delegated agent and independent
root review.

The entire original `functions.go` blob is identical at the historical S4 source
`b791ce0616d1821cebfc57909a6bc50a195ca68c`, the integration base, and the
tests-only baseline: Git blob `223816c58118c47bc812ba042232aa6db741e127`,
SHA-256 `2edc499d643130123331e3baa0b6662cae7124d7443ebde526bd5f7e0dd9ca63`.
The focused comparison therefore measures the original implementation sampled
in that timeout. It does **not** establish the cause or resolution of the whole
private S4 failure.

## Narrow implementation

The original expression is `([a-zA-Z0-9]+)\(`. The replacement finds each opening
parenthesis and walks backward through its immediately preceding ASCII letter or
digit run. It then performs the same function-map lookup and sets the same
`used` flag. It resumes after the parenthesis, retaining nested occurrences.

This preserves the existing lexical behavior: leading digits count; underscores
and non-ASCII bytes split names; whitespace immediately before `(` prevents a
match; calls inside quoted text and comments still count. It adds no parser
interpretation, function reachability policy, shared cache, parallel compiler
execution, early success, or timeout change. Input bytes and existing function
definitions remain unchanged. Each input region is searched forward once and an
ASCII run immediately before `(` is inspected backward once; submatch arrays are
no longer collected.

## Executed comparisons

The unchanged `function_usage_scan_test.go` was run three times at both baseline
and implementation sources. Each process passed **72 named events**, with zero
failures or skips. Each repetition includes:

- Seventeen explicit ASCII, digit, case, underscore, Unicode, invalid UTF-8,
  NUL, whitespace, quoted/comment, escaped, adjacent, repeated, and nested cases.
- **20,120** exhaustive short-string and all-byte boundary comparisons against
  the original regex. Every ASCII substring in each bounded input is available
  as a function, making erroneous shortened matches observable. Prior used state
  is preserved.
- Four generated public payload/body fixtures. Both implementations must mark
  the same known functions and preserve unused and already-used states.

The same source also passed **10 named events** from the existing Legacy inline
function CLI, complete standard-function Legacy/V2 native-graph differential,
and compile/decompile state-isolation tests in an ordinary clone without Git
alternates. This is local CLI/native-structure evidence, not Apple execution or
formal acceptance credit.

## Bounded measurements

The implementation source ran the original regex oracle and the production
scanner in one Go benchmark process. Environment: Go 1.24.13, Linux/amd64,
`GOMAXPROCS=2`. Each cell below is the median of **three single-iteration
samples**; fixture construction and compilation are outside the timed scan.

| Public fixture | Original regex, ms/scan | Production scanner, ms/scan |
|---|---:|---:|
| 64 KiB encoded payload plus calls | 2.749536 | 0.002273 |
| 1 MiB encoded payload plus calls | 46.424718 | 0.041040 |
| 4 MiB encoded payload plus calls | 192.062933 | 0.144773 |
| 256 KiB call-dense function body | 14.136225 | 0.302566 |

Every measured scanner sample reported **zero allocated bytes and zero
allocations per scan**. For the call-dense body the original regex allocated
2,726,232–2,732,384 bytes and 30,859–30,874 allocations per scan. The independent
baseline process also exercised the unchanged production function, including
223.1–274.4 ms samples for the 4 MiB payload.

These are small, local algorithm comparisons with synthetic public data, not a
stable performance threshold, Windows measurement, full compiler benchmark, or
prediction of private corpus completion. The original S4 stack is a sample of
where execution was observed; other compilation and reconciliation work remains
outside this experiment.

## Preserved failures and scope

The original S4 invocation on `b791ce06` exhausted its **15-minute Go deadline**.
Its first corpus case remained incomplete; the outer process did not time out,
and source/input preservation checks passed. Its original private streams remain
on Windows. The existing public safe reconciliation is retained unchanged here.
The separate startup diagnostic file is a submitted driver, not an execution
receipt, and grants no test or timing result.

The first new native-regression invocation from a linked worktree failed before
Legacy CLI execution because the helper's default `go build` could not obtain
VCS status. Its seven passing named events and original failure are retained.
The exact same source and test command then passed all ten named events from an
ordinary clone. VCS stamping, test assertions and deadlines were not disabled or
relaxed.

No private corpus input was transferred or replayed, no S4 gate was marked
passed, no original budget changed, and no release or formal acceptance record
was created. Final integrated CI, exact-source private replay, consumer tuple
refresh and native acceptance remain separate work.

## Reproduction and originals

Run the scan comparison on either recorded source:

```sh
GOMAXPROCS=2 go test -count=3 -timeout=2m \
  -run '^TestP15FunctionUsage' -bench '^BenchmarkP15FunctionUsageScan$' \
  -benchtime=1x -benchmem -json .
```

Run the existing regressions from an ordinary source clone:

```sh
GOMAXPROCS=2 go test -count=1 -timeout=3m \
  -run '^(TestP15LegacyInlineFunctionCLI|TestP15RP28StandardFunctionLegacyDifferential|TestCompileStateIsolation|TestDecompileStateIsolation)$' \
  -json .
```

The measured invocations also used `GOFLAGS=-p=2` and `GOTOOLCHAIN=local` with
the verified Go 1.24.13 executable. Commands, source hashes, original Go JSON,
stderr, process receipts, all benchmark samples, and the preserved failure are
in the [source/member manifest](evidence/p15-function-usage-scan-20261122/manifest.json)
and [15-member original archive](evidence/p15-function-usage-scan-20261122/originals.zip).
Archive size: **20,382 bytes**. SHA-256:
`797f15937b178940698340b42fd0586ab4065ea6e4617a3aaa523746b99fca0f`.
