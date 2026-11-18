# P10R15 CLI option values and compile mode evidence

## Scope and corrected behavior

The CLI preserves the complete value after the first equals sign in a prefixed option. Signing URLs such as `--signing-server=http://127.0.0.1:PORT/sign?response=json&opaque=a=b==` and output names such as `--output=result=with=value.shortcut` no longer lose their suffix. This repairs the existing args-parser dependency's value truncation in place; option aliases and last-occurrence precedence are retained.

Both `cherri file.cherri` and `cherri compile file.cherri` now use the same compile language selector:

| Options | Compile pipeline |
| --- | --- |
| No language option | V2 |
| `--mode=v2` | V2 |
| `--legacy` | Legacy |
| `--mode=legacy` | Legacy |
| `--legacy --mode=legacy` | Legacy |
| `--legacy --mode=v2` | Reject with conflicting compile mode diagnostic |
| `--mode=future` | Reject with unsupported compile mode diagnostic |
| `--mode=` | Reject with missing mode value diagnostic |

The Legacy branch reuses the pre-existing file compilation pipeline, including action loading, profile selection, native plist generation, output handling, and signing. The compile subcommand passes its resolved output and skip-sign options into that pipeline. Import retains its existing independent mode handling.

## Reproduced baseline failures

The new causal test was first run against the actual frozen r1 executable built from `d00bc0b9e9a4d8c460623de3d883680b94289e04`.

- A file-form V2 signing URL reached the loopback server as `/sign?response` instead of the full query, selecting the wrong request adapter.
- A Legacy file-form output path containing equals signs was truncated before the request reached the signer.
- File-form `--mode=legacy` still attempted V2 compilation and rejected the Legacy-only fixture.
- The compile subcommand ignored both Legacy selectors.
- Invalid and conflicting compile mode values did not consistently produce a mode-selection error.

Baseline command: `go test -count=1 -timeout=5m -run '^TestP15CLIOptionValuesAndLanguageMode$' -v .`, with `CHERRI_CLI_VALUE_TEST_BIN` pointing to that frozen executable. Sentinel evidence job: `job_dc245df92c7d` (expected nonzero result).

## Candidate validation

Command: `go test -count=1 -timeout=5m -run '^TestP15CLIOption' -v .`, with the external binary override unset. The test builds and executes the candidate CLI. Sentinel job `job_3139d06eaa1f` passed on Windows; the Go test reported 7.389 seconds, with 16.96 seconds total job duration.

| Check group | Named cases | Required evidence |
| --- | ---: | --- |
| Signing query routing | 5 | Healthy unsigned control; exact full request URI; actual multipart request; unique injected error cause; nonzero CLI exit; no signed destination |
| Exact output path | 5 | Actual successful CLI; full output filename including equals; nonempty parsed native plist; truncated filename absent |
| Compile language selection | 18 | V2/Legacy fixtures through both entry forms; nonempty native plist on success; correct diagnostic and no output on rejection |
| Duplicate option/value preservation | 7 | Full URL, long and short output option, repeated values, final simple/empty value, positional input unchanged |

All 35 named cases passed. The tests use synthetic source and a loopback signing fault endpoint. They do not establish Apple signature validity or on-device execution. The existing real signed workflow and runtime gates remain separate release evidence.

## Reproduction and integration

`p15_cli_option_values_test.go` is part of the normal Go test suite. `CHERRI_CLI_VALUE_TEST_BIN` exists only to rerun the same causal checks against a separately identified baseline; leave it unset to validate the current source.

Production files changed: `args.go`, `main.go`, and `language_cli_adapter.go`. No importer, private corpus, signing payload validator, or live Remote Builder tuple was modified by this stage.
