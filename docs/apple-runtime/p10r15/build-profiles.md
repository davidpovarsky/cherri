# Compiler build profiles

P10R15 adds an explicit `--build-profile=debug|release` option to the existing compiler. The default is `debug`, preserving existing diagnostic expansions. For Legacy source use `--legacy`; `--mode=legacy` is the import dialect selector and does not select the Legacy compilation path.

```sh
cherri workflow.cherri --skip-sign --build-profile=release
cherri workflow.cherri --legacy --skip-sign --build-profile=release
```

The shared macro dispatcher removes only the existing diagnostic operations: debugLog, debugLogPrevious, debugShowLogs, debugFinish, debugClearLogs, debugTrace, debugStartTimer, debugStopTimer, debugTraceEvent and debugExportTrace. It does not remove source producers, function arguments, Run Shortcut calls, ordinary actions or debugAssert validation. This is compile-time expansion control; no device runtime toggle or persistent global setting is added.

## Value and failure contract

Source effects and their native UUID/OutputName edges remain in place. A program consuming the output of a removed diagnostic, such as the result of debugStopTimer, fails with `RELEASE_DIAGNOSTIC_VALUE` instead of leaving a dangling magic variable. Literal text that happens to contain a removed UUID is not mistaken for a reference. Reserved diagnostic variables cannot be consumed after instrumentation removal. Invalid profiles fail with `BUILD_PROFILE_INVALID`; nonliteral options, conflicting display modes, invalid trace stages and empty timer labels remain errors in release.

The Go V2 entrypoint is `CompileSourceToPlistWithOptions(filePath, source, CompileOptions{BuildProfile: BuildProfileRelease})`. Existing entrypoints retain debug defaults. Build profile state is reset and isolated between compilation transactions. Consumer adapters that do not expose this option keep existing debug behavior; no installed consumer is claimed updated by this compiler change.

## Executed evidence

- Six new test functions exercise the production Legacy and V2 compilers, source-effect preservation, exact producer/consumer edges, consumed timer rejection, nested reference mutation, invalid options and sequential profile isolation.
- All existing Debug Stdlib tests pass, including all six display modes in both dialects. Combined command: `go test -count=1 -timeout 3m . -run '^TestBuildProfile|^TestDebugStdlib' -v` (exit 0).
- Actual Windows CLI executable, eight controlled executions: Legacy uses `--legacy`; V2 uses the default frontend. Each dialect passed debug, release, omitted-profile default and invalid-profile rejection. The fixture emitted 7/4 actions in Legacy debug/release and 10/7 in V2; default equals debug. Both preserve the assertion message and source markers; release contains no diagnostic marker or appendvariable. Both invalid-profile executions return exit 1 and create no output.
- Independent native plist parsing verifies the actual CLI artifacts. Raw evidence remains in the local P10R15 evidence directory. Initial `--mode=legacy` CLI probes were recognized as V2 and replaced by the explicit `--legacy` runs; those initial probes are not Legacy evidence.

## Remaining gates

Full integrated suite, exact-SHA Linux/macOS/Nix CI, signed simulator execution of release behavior and physical-device type semantics remain separate gates. This feature does not complete the typed-macro roadmap. Diagnostic removal does not promise that arbitrary source arguments containing sensitive values are removed; argument producers intentionally still execute.
