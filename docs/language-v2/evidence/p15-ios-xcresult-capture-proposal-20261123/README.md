# Inactive proposal: bounded full-iOS issue capture

**Status: investigation only; no implementation, test change or native dispatch.**

The original K11 full-iOS run [45108352089](https://github.com/davidpovarsky/cherri/actions/runs/45108352089)
remains failed: 53/57 passed, four failed, xcodebuild exit 65. Its source is
`9b4135e705b08f765b7304e01212d3b9658806fb`. D03's original full test passed.
This proposal neither changes those facts nor supplies a cause for the failures.

## Observed producer gap

The original input-paste failure at line 210 and generated-closer UI failure at
the shared startup query on line 215 have no retained structured issue text.
Their classifications remain `UNCLASSIFIED`. Timing and locations are not
substitutes for the missing issue.

`CherriEditorUITests.swift`:51–65 configures nonce-owned issue recording only for
BRG27, D03 and E01; `record(_:)`:83–118 requires that state. GeneratedCloser is
outside that scope, and the input unit class has no corresponding observer.
Extending Python's test allowlist alone cannot create those missing issues.

`collect_ios_test_evidence.py`:445–450 retains test ID, source line and the
existing fixed category, then drops the arbitrary message. It cannot recreate
an XCTIssue type or NSError domain/code that it never received.
No digest-only change or speculative classifier expansion is proposed.

## Smallest credible capture direction

A future reviewed change could give the original xcodebuild invocation an
explicit fresh result-bundle path, then run bounded structured extraction
after the same required full run. The existing reviewed command is:

`xcrun xcresulttool get test-results summary --path RUN_OWNED_RESULT_BUNDLE`

It is already used in `qa_terminal_receipts.py`:272. Its existing observer
checks source/context and owned paths, captures bounded stdout/stderr and
retains command timeouts separately from native results.

Apple's [Xcode 16.3 release notes](https://developer.apple.com/documentation/xcode-release-notes/xcode-16_3-release-notes.md)
confirm summary, tests and test-details commands and direct users to command
help for the exact output format.
Apple's [result-bundle guide](https://help.apple.com/xcode/mac/current/en.lproj/devc38fc7392.html)
documents result-bundle creation and structured extraction.
The [WWDC19 explanation](https://developer.apple.com/videos/play/wwdc2019/413/)
also describes structured failure records and tool-provided schema; that
historical description is not a verified Xcode 16.4 failure schema.

The actual full-iOS job used Xcode 16.4 and an iPhone 16 Pro Simulator, not the
xcode-27 QA environment. The exact Simulator OS version was not retained.
No original full-iOS xcresult bundle or verified 16.4 failure-summary schema is
available in the retained artifacts. The existing QA parser validates a JSON
object and test counts; it does not establish the shape of failed issues.

Therefore **no concrete parser is implemented or treated as ready**. Any future
extractor must first use the actual observed schema, preserve original test and
run identity, and retain absent type/domain/code as `UNOBSERVED`. Only justified
fixed diagnostic metadata, known source basenames/lines, bounded byte counts
and digests may be public. Raw bundles, activities, attachments, private text,
screenshots and hierarchy are outside this proposal.

## Existing encryption route: not supported for this purpose

The reviewed prerequisite collector has an encryption primitive and pinned
public recipient. Its existing end-to-end contract is narrower:

- `project_prerequisite_observation.py`:755–800 constructs only fixed
  prerequisite/source-journal members, at most 23 in QA mode.
- `verify_public` and `validate_public_summary`:866–901 require exactly
  `project-prerequisite.zip.p7m` plus `summary.json`, `prerequisiteOnly=true`,
  and `CURRENT_PREREQUISITE_CALL_ONLY`. The stage/budget schema fixes the
  existing install/generate prerequisite operation.
- The QA workflow uploads only those two declared prerequisite outputs.
  `ios-build.yml` publishes its existing sanitized test/startup files and has
  no encrypted issue-summary output.
- The already reviewed in-place private consumer is bound to that prerequisite
  inventory and original run/source/fixture protocol. It has no xcresult
  summary intake contract.

Calling the lower-level encryption function on different content would not
reuse a supported producer/consumer protocol. No such bypass, archive-membership
change, key query, credential flow or private-corpus expansion was performed.
The route is recorded as `UNSUPPORTED_WITH_CURRENT_ROUTE`.

## Controls and activation boundary

Existing meaningful controls cover exact private issue digests and fixed
domain redaction, malformed metadata, nonce/test binding, repeated test
invocations, source blob binding, symlink rejection, private-log exclusion,
command timeout and exhausted-budget handling. They are referenced by the
exact source files in `review.json`; none was rerun for this read-only review.

A schema-specific implementation would additionally need causal controls for
unknown/missing fields, wrong test/target/run, duplicated or stale results,
oversize/truncated capture, private-value canaries, and retention of the
original failed result. Existing controls do not prove an unimplemented parser.

Any later activation remains limited to post-test diagnostics in a separately
required final-source full-iOS run. It must preserve all original assertions,
test selections, deadlines, process exit and protected Swift bytes.
It must not trigger a same-source diagnostic rerun or claim native acceptance.
The current four full-iOS failures remain open.

## Separately named extension candidate

The original route remains unsupported for xcresult content. The subsequently
requested [new protocol outline](extension-proposal.md) describes a separate,
strict encrypted-summary producer and consumer for review. It remains inactive;
no code, credential action, test or native rerun has been performed.
