# P10R15 Legacy parity repair — 2026-11-19

## Verified result

The clean implementation `d0f19a62f80a65a507fdd45ff3e2fc48343ebaf9`, including
the menus runner repair `4bbfd98f0f003d92faec35bab4720c228c198cb9`, passes the
complete public differential gate: **47/47 fixtures and 17/17 action mappings**.
The original `globals.cherri`, `standard.cherri` and `menus.cherri` all compile
through both routes and compare as `PROVEN_EQUIVALENT`.

This is local compiler and normalized graph evidence. It does not establish
Apple execution, formal acceptance of V01/V03, the complete 179 requirements,
or a released combined integration candidate.

## Baseline and causes

The exact requested baseline `d7052f4568db26dba169bdefc72623e48c04968a` was built
locally before changing production source. Its real differential run reproduced
the handoff:

| Gate result | Baseline | Clean implementation |
| --- | ---: | ---: |
| Strict equality | 9 | 9 |
| Normalized semantic equality | 35 | 38 |
| V2 compile failures | 1 | 0 |
| Structural differences | 2 | 0 |
| Action mappings passing both routes | 17/17 | 17/17 |

`globals.cherri` produced seven structural differences. Legacy compilation
emitted a declaration-time Set Variable from Clipboard, whereas V2 emitted
Get Variable with a Clipboard system attachment. The old parity normalizer
then removed the legacy read and forwarded a live Clipboard token. That
transformation can hide a later clipboard mutation.

There was also a real native producer defect. The independent public native
diagnostic [run 44466065158](https://github.com/davidpovarsky/cherri/actions/runs/44466065158)
(source `f7f1447263ff99deb891e6044b62b270b6e8797c`, artifact `14502699926`)
reported an empty Get Variable Clipboard observation and failed initial-value
capture flags. Its canonical Get Clipboard and direct token observations
returned content. Those observations motivate the producer correction; they
are baseline evidence and are not a successful execution of this repair.

`standard.cherri` produced nine differences confined to internal function
identity: `runJS` and `chooseFromVCard` now have the
`module:builtin:stdfunc::` prefix in their declarations, dispatcher fields and
synthetic parameter identities. The bodies and argument routes otherwise
matched. Module qualification remains in the compiler.

`menus.cherri` failed because the migrated temporary workspace lacked the
original public asset. The separate menus repair stages the exact repository
asset inside that workspace and keeps compiler workspace access checks active.

## Repair boundaries

Immutable Clipboard bindings now emit the existing parameterless
`is.workflow.actions.getclipboard` producer at declaration time. Aliases and
later consumers retain its UUID, including after a later clipboard write.
Reference aggrandizements remain on the captured reference.

The comparator preserves that same read for an exact, untransformed,
single-write legacy Set Variable Clipboard input. It canonicalizes the input
to a Get Clipboard producer at the same action position, then forwards
references to that producer. It leaves transformed inputs, unexpected
parameters and multiple-write assignments unnormalized. It does not accept
the known-bad Get Variable Clipboard action as equivalent.

Builtin function identities are normalized only for parsed declarations in
the exact `module:builtin:stdfunc::` namespace, with no colliding local name.
The rewrite applies to the declaration, exact parameter identity and the
function field in recognized dispatcher envelopes. Foreign module names,
unknown targets, body text and ordinary argument payloads retain their
identity. No global namespace stripping was added.

The existing binding oracle and native diagnostic graph validator now require
the canonical producer. Original runtime fixture bodies, expected values and
the four independent diagnostic observer routes are unchanged by this commit.

## Verification

A new snapshot test first failed against the unchanged normalizer: the legacy
snapshot disappeared and the action count fell from four to three. After the
repair it passes while rejecting eight corruptions: a late read, a live
deferred token, a wrong producer reference, Get Variable Clipboard, a missing
read, a transformed input, an unknown capture parameter and multiple writes.

The real `standard.cherri` output is recompiled for the integration test.
Changed body values, a foreign module call, an unknown function call and a
wrong parameter edge all remain unequal.

Clean affected-package verification reported **97 named passes, including 46
top-level tests, zero failures and zero skips**, covering lowering, the
comparator and the differential runner. The runner tests also retain the
entry-asset/decoy-working-directory, missing-asset and exact-restoration checks.

The focused V01/V03 and CLI/mobile/service binding/defaults tests reported
**37 named passes**. These were run before the implementation commit with the
same production bytes; their stream is labeled focused rather than a clean
commit replay.

The actual native diagnostic fixture compiled and passed its graph/source
contracts. The original binding body was byte-identical, all 22 existing
causal graph mutations were rejected, and two additional in-memory capture
corruptions were rejected. The receipt explicitly states
`PASS_LOCAL_CAUSAL_GRAPH_ONLY` and `appleRuntime: NOT_RUN`.

For the clean commit replay, the compiler was rebuilt explicitly and provided
through `CHERRI_BIN`; the gate did not reuse an old root executable. Its
SHA-256 is
`899b0c8aa6a28b9990f12f97176cf59d87b7b90be386cec3016a855ebc05495e`.
The complete differential run exited zero in 10.297 seconds; the affected
package command exited zero in 13.906 seconds. The worktree remained clean.

## Reproduction and receipts

Build a fresh compiler outside the worktree and set `CHERRI_BIN` to its
absolute path before executing:

```text
go test -count=1 -v ./internal/language/lower ./internal/shortcutcompare ./tools/legacy-parity
go test -count=1 -run "^TestP15(V01|V03|BindingDefaultsPublicRoutes)" -v .
go run ./tools/legacy-parity -output <evidence-directory>/full-parity.json
```

Public receipts are in
[evidence/p15-legacy-parity-20261119](evidence/p15-legacy-parity-20261119/):

- `baseline-parity.json` and `clean-parity.json` contain all fixture and action results.
- `snapshot-before.log` records the causal pre-repair failure.
- `clean-tests.log` and `binding-focused.log` retain actual test streams.
- `native-capture-graph.json` records graph/source verification and its runtime limit.
- `verification.json` records the exact implementation, binary hash, commands and exits.
- `manifest.json` hashes the published receipt bytes.

Only public fixture evidence is included. Absolute task paths are replaced
with `$WORKTREE` and `$EVIDENCE`; binaries and temporary Shortcut files are
outside the repository.

The integrator must replay relevant checks after combining the independent
migration/signing, native debug seed and QA observer repairs, then run the
authorized Apple/runtime and acceptance gates. No formal acceptance status
was changed by this repair.
