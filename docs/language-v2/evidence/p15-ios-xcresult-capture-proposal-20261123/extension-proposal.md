# Separate diagnostic protocol candidate — inactive

This is a new end-to-end extension proposal. It is not a supported use of the
existing prerequisite protocol, and no implementation or native dispatch has
been authorized by this document. Original K11 failures remain unchanged.

## Proposed file ownership

| Proposed path | Purpose |
| --- | --- |
| `ios/scripts/collect_ios_xcresult_summary.py` | New diagnostic producer and strict new public receipt validator |
| `ios/scripts/verify_ios_xcresult_summary.py` | Separate in-place consumer with independent exact tuple/source binding |
| `ios/scripts/test_ios_xcresult_summary.py` | Causal identity, privacy, capture and publication controls |
| `.github/workflows/ios-build.yml` | Fresh explicit result path, bounded post-test capture, exact two-file upload |

Existing prerequisite source, the 23-member archive, public validators and
already reviewed c742/155081 consumers remain byte-unchanged. Protected Swift
tests, assertions, selection and original deadlines remain unchanged.

## New protocol and execution boundary

The proposed schema is `cherri-ios-xcresult-summary/1`. Reserve a fresh
run/attempt-owned directory and add only an explicit `-resultBundlePath` to the
original xcodebuild command. Reject stale/symlink output before testing.

After that original test invocation, use the already observed command once:

`xcrun xcresulttool get test-results summary --path RUN_OWNED_RESULT_BUNDLE`

Only the summary is captured. No bundle traversal, activities, attachments,
screenshots, AX hierarchy, application container scan or console-log export is
added. Keep the original native exit code independent of capture outcome.

Proposed private ZIP inventory, exactly six regular members:

1. `identity.json` — exact repository/workflow/run/attempt/source plus source-file bindings.
2. `command.json` — exact command, process outcome, bounds and complete stream bindings.
3. `xcresult-summary.stdout` — original bounded stdout, never public plaintext.
4. `xcresult-summary.stderr` — original bounded stderr, never public plaintext.
5. `xcodebuild.exitcode` — original process result; missing is not success.
6. `manifest.json` — exact inventory, sizes, digests and completion flags.

No issue-field schema is assumed. A complete zero-exit summary may be checked
only for duplicate-free JSON-object syntax; that does not classify its issues
or authorize acceptance. Partial/timeout/overflow output stays explicitly
incomplete and cannot be interpreted as a complete summary.

## Proposed hard bounds

| Phase or content | Bound |
| --- | --- |
| Identity/source preflight | 15 seconds |
| One summary command | 20 seconds |
| Encryption | 20 seconds |
| Whole post-test envelope | 60 seconds |
| Each stdout/stderr stream | 1 MiB |
| Each metadata member | 32 KiB |
| Plaintext ZIP | 3 MiB |
| Ciphertext | 3 MiB + 64 KiB |

These are new diagnostic bounds inside the original workflow deadline, not
larger native/test limits. No retry or time-budget extension is proposed.
Capture failures produce fixed diagnostics and preserve original native failure.

## Reuse of existing encryption without protocol substitution

Use the source-verified `capture()` and `encrypt_archive()` primitives from
`project_prerequisite_observation.py`. The latter verifies the existing tracked
public certificate before and after encryption; its pin is
`55956e825597a2e60acca4045e4ea8040ae27f17caa7964e9b4493824fa0c1f1` at
`tools/shortcuts-simulator-qa/xcode_environment_inventory_recipient.pem`.
No private key is needed on the runner.

The primitive's hard-coded old ciphertext basename is used only inside a fresh
0700 private encryption staging directory. A successful result is moved
atomically to the separately named new public artifact. It is never submitted
to the old prerequisite validator or declared `prerequisiteOnly`. Do not inherit
the old collector's macOS27 environment assertion or prerequisite stage/budget
schema; the full-iOS diagnostic belongs to its actual original toolchain.

The only proposed public files are `ios-xcresult-summary.zip.p7m` and
`receipt.json`. The latter uses the new schema, fixed capture/result fields,
known run/workflow/source IDs, source-file hashes, byte counts and digests.
No raw summary field, private path, process stderr or issue text is public.

## Separate in-place consumer

Before decryption, independently retained authenticated GitHub run/artifact
metadata must supply the exact expected tuple and source-file hashes. The public
receipt's own claims cannot authorize intake. Bind repository, workflow path/ID,
event, run ID, attempt, exact source and original artifact identity.

Reuse only the existing owner-held Windows private workspace and quiet local
OpenSSL decrypt route, with the already used certificate/key locator. This
proposal does not read that locator, retrieve a key, change access or establish
a new credential flow. No key contents or private paths enter output.

The new consumer checks its independent tuple, cipher/plain hashes, six-member
inventory, fixed metadata schema, per-member/total size limits, no duplicate,
absolute/traversal/symlink/extra member, exact source bindings, and original
stream completion before in-place JSON inspection.

The initial public result is limited to fixed validation booleans, counts and
digests. Meaningful issue interpretation must wait for review of the actual
captured schema. Any later public type/domain/code field must be justified by
that observed schema; absent values remain `UNOBSERVED`. Plaintext stays in
the existing private Windows location. No automatic arbitrary-field export is
proposed.

## Meaningful controls required before activation

- Wrong source/run/attempt/workflow/artifact, source changes during capture and
  changed protected/helper/public-recipient bytes are rejected.
- Stale output, replaced files and symlink inputs are rejected.
- Mismatched cipher/plain/member digests and missing/extra/duplicate/traversal
  or oversized ZIP members are rejected.
- Summary timeout, overflow, incomplete capture, malformed/duplicate-key JSON
  and decrypt failure never produce a complete diagnostic claim.
- Private canary strings, paths and message values remain absent from every
  public file and exception output; encryption errors remain fixed codes.
- Original xcodebuild exit 65 remains 65. Diagnostic success never creates
  acceptance records or changes a failed test result.

Existing unchanged primitive controls can be reused. New protocol tests must
validate these boundaries; none has been implemented or run at this stage.
Activation requires separate review and applies only to the next independently
required final-source full-iOS run, never a same-source diagnostic retry.
