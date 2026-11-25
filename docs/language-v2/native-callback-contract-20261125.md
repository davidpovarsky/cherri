# Exact native callback output and explicit app restart

## Reproduced acceptance gap

The receiver splits callback output on `CharacterSet.newlines`, trims each line,
removes empty and diagnostic trace lines, and joins the summary with LF. A raw
`CHERRI_QA_LINE1\rשלום 🚀` therefore produces the same summary as
`CHERRI_QA_LINE1\nשלום 🚀`. Their UTF-8 byte counts are equal. The prior host
classifier accepted that changed raw output when the expected result used LF.

The new `resultShape.utf8SHA256` is SHA-256 of the original callback text's UTF-8
bytes, before any summary processing. The host requires exact summary, original
byte count, and digest agreement, as well as the existing fresh nonce, fixture,
shortcut-name, source, non-self-test, and SUCCESS checks. The original callback
text remains private; only its digest and existing counts are added to the
receipt. This proves exact decoded callback text, not URL percent-encoding or
internal Shortcuts object identity.

`utf8SHA256` is optional only for Swift decoding of historical receipt files.
New receiver records always provide it. A historical record without a digest
fails the current host's acceptance check. Old receipts are never upgraded,
rewritten, or given a fabricated digest. New runs require the receiver and host
changes together; an old receiver cannot satisfy the new contract.

## Bound on the existing single-line corpus

The source manifest SHA-256 is
`ea664fe90090e76ac60835f49c72abd35b9c8bf708f01d0bb1a0c1591aa31388`.
It contains 37 runtime-enabled cases with unique fixture IDs. Each expected
value begins with `CHERRI_QA_`, contains no newline or Unicode line separator,
has no leading/trailing whitespace, and is at most 58 Unicode scalars long.

For these exact unchanged expected strings, the prior summary-plus-raw-length
check excludes the normalization gap. An accepted single-line summary has only
one retained nonempty line: any additional retained line would introduce LF and
fail exact summary equality. Removing whitespace, discarded lines, traces, or
truncated suffixes would make the original UTF-8 byte count strictly greater
than the expected count. Equal-length CR-to-LF replacement needs a multiline
expected value, which this pinned manifest does not contain. The JSON transport
and Python string comparison retain the Unicode content of the surviving line.

This is a bounded explanation of the earlier contract for that manifest, not a
new acceptance stamp for historical runs. It does not establish that any case
ran, that a callback was genuine, or that an import succeeded; the original
receipts must establish those facts separately. This component audited the
source manifest, without inspecting or operating the active Mac batches.

## Explicit Shortcuts restart option

`run_differential_batch.py --restart-shortcuts-after-import` terminates only
`com.apple.shortcuts` on the explicitly selected simulator, after an IMPORTED
receipt. The normal receiver-triggered run then starts Shortcuts again. The
option defaults to false and is recorded in every row and summary. Each
requested termination has a separate persisted receipt with the exact command,
fixture/import identity, and requested/completed/failed outcome. A termination
error records failure and ends the batch; it does not trigger a hidden fallback
or change the expected result. Trigger `simctl openurl` now has a 60-second
command timeout. This option is an explicitly observable experiment on app
process state; it does not establish that Apple's name lookup cache caused an
earlier error.

## Focused verification

On unchanged production code, the classifier controls failed for a missing
digest, a changed digest, and the equal-byte CR/LF substitution (three expected
RED failures). After the repair, all 17 focused host tests pass: 13 callback
acceptance controls and four mocked import/restart controls. The latter verify
command order, selected simulator/app, default no-termination behavior, no run
after an unverified import, and failure retention without fallback.

The Swift Foundation contract includes the standard SHA-256 `abc` vector, an
independently computed Hebrew/emoji UTF-8 vector, the equal-length CR/LF pair,
and decoding an old receipt without inventing a digest. Those Swift tests and
the receiver build remain NOT_RUN in this component task. No Mac build,
simulator action, native test, or full compiler gate was performed here.

See `evidence/native-callback-20261125/verification.json` for the source pin,
manifest audit, commands, and retained external RED/GREEN log locations.
