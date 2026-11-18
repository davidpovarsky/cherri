# Pinned editor input patch

The upstream dependency and revision in `ios/project.yml` are unchanged:
`mchakravarty/CodeEditorView@5386056ab53d43363083cb96069715a9608aa048`.
The original upstream license accompanies this small patch.

## Observed defect

Actual iOS run [44234466807](https://github.com/davidpovarsky/cherri/actions/runs/44234466807),
source `d2406cf1fc9443cb862a18042ffff4b19a66e7c4`, recorded six real keyboard
fixtures whose final UTF-16 source was exactly the requested source plus `)`.
Verified-empty resets and declarations without parentheses were exact. The
minimal reproducer typed `text("x")` followed by newline; the recorded source
was `text("x")` followed by newline and an extra closing parenthesis.

The pinned upstream storage delegate inserts a completion suffix after an opening
delimiter. Its UIKit input path has no handler for typing the generated closer.
The patch records only the exact suffix that this existing routine inserted and
allows a matching single-character keyboard input to move past that known unit.
It includes the generated newline before a multiline closing brace and generated
comment delimiter units. This preserves existing completion and exact full-block
input. Authored equal text is never marked. Offsets shift with ordinary UTF-16
insertions; deletion, replacement, navigation, composition and paste invalidate
provenance. A consumed unit cannot be consumed again.

## Reproducible application

`ios/scripts/bootstrap.sh` and `.github/workflows/ios-build.yml` resolve packages
with `-clonedSourcePackagesDirPath .build/SourcePackages`. The preparation script
checks the exact revision, every original Git blob and full original or patched
SHA-256 before writing any file. Unique replacement anchors and final hashes are
also required. An unrecognized revision, local edit, changed patch or missing
application fails closed. `--check-only` cannot apply a missing patch or claim
success for original files. Repeated application is idempotent.

Every configured native build/test keeps the shared checkout, disables automatic
package resolution and checks the patched hashes immediately before xcodebuild.
Use the local commands in `ios/README.md`. A direct GUI build using another
SwiftPM cache is outside this prepared path; it is not proof of the patched app.

## Native controls

`CherriEditorInputTests` exercises the actual pinned CodeView: generated closer
presence, one-time consumption, nesting, list/dictionary delimiters, multiline
blocks and comments, Hebrew/combining marks/emoji, authored closers, movement,
deletion, multicharacter input and paste. UI tests retain actual keyboard typing
and exact UTF-16 receipts; no fixture paste replaces this path. Existing compile,
diagnostics, rename, Unicode and visual edit assertions remain.

Local Python/source verification is not UIKit evidence. The new Swift tests and
full UI suite must pass on an immutable iOS CI commit before this defect is
accepted as closed. The initial diagnostic run remains failed, with zero
acceptance records.
