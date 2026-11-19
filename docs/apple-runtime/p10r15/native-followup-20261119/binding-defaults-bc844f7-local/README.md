# Local binding/default signed-QA preparation

Verified source: bc844f7ed48226ce59e92774625b5467731dd63f, clean before
and after all checks. The source chain begins at 7fc5087. This archive is
local compilation and gate evidence; Apple runtime and signing are NOT_RUN.

The exact clean cached Windows build passed in 2.640 seconds. The real
Phase0 compiler gate passed in 7.953 seconds: 65 attempts, 63 emitted
artifacts, and two expected E_TARGET_VALUE_SHAPE rejections for the
unchanged unsupported F04 sources. The rejected fixtures are not counted
as native passes or compiled artifacts. Related Python input, callback,
source, and publication gates passed all 29 test methods with no skips.

Both new public adapters preserve the original source bodies byte-for-byte,
their original Shortcut names, and the authored exact expected payloads.
The observer reads the actual final Clipboard, emits Base64 with line-break
mode None, and caps its encoded witness at 512 characters. Explicit empty
List and Map counts remain exactly zero; no expectation was reinterpreted.

The actual emitted graphs passed the observer and output checks. Thirteen
damaged graphs were rejected: five binding controls and eight typed-default
controls, including wrong observation, raised cap, hardcoded summary,
incorrect output edge, unexpected tail, wrong self-call name or identity,
and a mismatched function-dispatch closure. The callback controls also
reject altered values, omitted defaults, wrong headers, extra newline,
oversize or invalid Base64, missing trace and stale run identity.

The initial clean 7fc5087 Phase0 failed because the new gate incorrectly
expected output to be the last native action. The typed-default program's
function dispatcher has a closing End If after main output. Its real
compiled plist and the failed receipt are retained here. The final gate
checks the exact output edge and that single balanced dispatch closure;
it does not discard arbitrary actions after output.

The workflow bundle qa-binding-defaults expands into the exact
qa-binding-capture and qa-typed-defaults selectors, with max-parallel 1
and fail-fast false. Raw shell invocation of the bundle selector is
rejected; only the workflow expands it. Artifact names are distinct.

Compiler binary SHA256:
0102a52168ac0ee9963e18ee5a25cd0a89ffead169dced4b866115e81f991396
(24,815,104 bytes). The binary was built from this exact clean worktree;
its external receipt records that source. No embedded-VCS claim is made
for this isolated worktree build. The stamped release compiler remains
a separate packaging requirement.

The signed Unicode diagnostic must run first, followed by this one bundle
on the same frozen S3 source. No run or formal acceptance is claimed here.
