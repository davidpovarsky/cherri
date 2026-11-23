# Bounded iOS xcresult summary: implementation and actual offline qualification

Tested source: adf78c827fa7d9c5676a38258d2a6f423991ea1e. Base: eb2d0d38a45cb4f7dc0740a63916397fbb995000.
This diagnostic source pin is separate from the future compiler/app freeze.

## Actual result and preserved chain

Final attempt 03 passed all 22 parents, zero failures/skips, exit 0:
3.37727396376431s supervisor and 3.214s unittest. GNU timeout remained 120s
with 5s termination grace. Original streams, claims and before/after guards
bind all four implementation files and all nine original inputs to clean
source. Native execution credit and formalAcceptanceGranted are false.

Preserve source commits in order:

1. 305b734b03f86a53eeff58ab8e803dd4a65a3823: four-path implementation.
2. e610fd195fdbe9d3c6f902a12868e7768e44ff66: fresh-directory fd-bound mode fix.
3. adf78c827fa7d9c5676a38258d2a6f423991ea1e: one-line synthetic-fixture group setup.

Attempt 01 stopped before any parent: the setgid Oracle ancestor made
mkdir(0700) yield2700. The correction opens only the exclusively created
directory with O_DIRECTORY|O_NOFOLLOW, compares fd device/inode/owner,
fchmods0700 and verifies the same path identity afterward. Existing parent
permissions and the strict accepted0700 invariant remain unchanged.

Attempt 02 ran all22 parents: the original21 passed; the new setgid fixture
failed before prepare. Ubuntu primary gid1001 does not belong to inherited
gid1002, so chmod2700 cleared that bit. The test-only correction assigns
only the new fixture directory to the process primary group before its
unchanged chmod2700/assertion. Attempt03 then passed all22.

All three original attempts remain byte-for-byte in this packet. The
duplicate-ZIP-name warning and unchanged capture helper ResourceWarning
during a timeout negative remain in stderr. The negative returned non-PASS;
this packet makes no stronger at-return cleanup claim. Root independently
observed the final warning PID absent. No original helper was changed.

## Source and native-contract preservation

Only three new Python sources and ios-build.yml implement the new protocol.
Removing the29 inserted workflow lines exactly restores all original bytes
(workflow-final-preservation.json). The native xcodebuild argv adds only
-resultBundlePath. Original selectors, status assignment, exit-file write,
exit status and75-minute job deadline remain unchanged. The original native
block SHA256 is a30d21e7dd7446ef2dd355ad2de0643e50d91296cb025af93e12eea14952ada3.

qualification-intake.json records all four final file hashes, nine untouched
source inputs (including public recipient), and the protected absent
CodeEditorView.swift state. Old23-member prerequisite protocol/consumers
remain byte-identical. No compiler, Swift assertion, R23 policy, private key,
Go producer, provider call or native workflow was changed/run by this lane.

## New diagnostic boundary

cherri-ios-xcresult-summary/1 uses a separate15s pre-native reservation,
bound to source/run/attempt and original native-exit-file absence. Post-test
collection verifies that marker, identity and source before capture and
publication. Preflight15s, summary20s and encryption20s stay inside60s.
Limits are1MiB per stream,32KiB metadata/member,3MiB plaintext ZIP,
3MiB+64KiB cipher and3MiB+128KiB outer artifact.

Private ZIP inventory is exactly identity.json, command.json,
xcresult-summary.stdout, xcresult-summary.stderr, xcodebuild.exitcode,
manifest.json. The public result is ciphertext plus fixed receipt, or a
receipt-only rejection with null archive/cipher. Cap+1 originals remain
private and are never clipped into a sealed complete result.
Bounded incomplete originals may be SEALED_INCOMPLETE, never acceptance.

The consumer requires caller-owned independently authenticated expected
GitHub tuple, original artifact digest and pinned source records before
public admission and again before already-decrypted private inspection.
It cannot obtain provenance from receipt self-claims. Repo code does not
discover keys or decrypt data.

## Limits of this evidence

These are synthetic offline process controls. Completed suites performed
one real encryption to the existing public recipient; known plaintext was
supplied directly for synthetic private intake. No decryption is claimed.
The positive case preserves native exit65 without acceptance. Actual failed
Xcode16.4 issue schema remains unobserved and uninterpreted. Actual native
diagnostic capture requires the next separately authorized full-iOS run.
No producer execution occurred after the final PASS.
