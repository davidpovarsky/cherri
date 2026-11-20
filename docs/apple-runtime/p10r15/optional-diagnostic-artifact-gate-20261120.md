# Optional diagnostic: unsigned artifact and callback gate

This component prepares the public Optional forwarding diagnostic for a real signed Simulator run. Its implementation is `3e1fe7933713077374d8c3cbaec3e864ebd21ad0`, based on the Optional v2.2 compiler and independently reviewed fixture at `c513b2d6d6cac9a94b58e3f7fc834f130afc1d80`. No compiler semantics, fixture source, expected values, signing endpoint, or formal acceptance membership changed.

## What is now checked

The existing QA route compiles the fixed public source, then submits the exact unsigned Shortcut file to the original complete Go graph oracle. The file must be a bounded regular file under the owned build directory, keep its name and SHA256, and contain all 399 expected native actions and UUID relationships. The original source and freshly compiled positive checks still run. A single typed receipt binds source, expected contract, artifact, action count, and process log hashes before signing.

The fresh callback oracle requires that same receipt, exact eleven-field output, nonce, fixture identity, successful callback, and normal NDJSON trace. The receiver now recognizes only the exact additional public marker `CHERRI_OPTIONAL_FORWARDING_DIAGNOSTIC_V1:`; its existing secret and personal-data rejection runs first. Actual receiver sanitizer and trace-splitting methods are extracted and compiled for fourteen controls; they retain contradictory public results for diagnosis.

## Actual local verification

At clean implementation source: 66 named Go tests, 9 Python artifact-gate tests, 27 strict callback tests, 15 immutable-source auditor tests, and 14 compiled Swift policy controls passed. The actual CLI output passed Python-to-Go artifact intake. The unmodified compilation heredoc completed all 70 attempts: 68 compiled public fixtures and 2 expected diagnostic rejections.

The first local heredoc extraction omitted the outer shell's runtime output directory and failed while writing its report. That failure and original logs are retained. The corrected local setup reproduces the original shell's directories; the exact heredoc then passed without any source or oracle change.

The proof JSON records source hashes, original receipts, output artifact digests, and the failed setup control. Raw compiler inputs and process logs remain outside the publication allowlist. The linked-worktree compiler is bound by the clean checkout, actual build command, and binary digest; embedded VCS metadata is reported separately.

## Remaining native gate

Apple execution, signing, and runtime value fidelity are NOT_RUN for this component. Local graph checks and synthetic callback controls cannot establish them. The proposed single component run uses the existing Simulator workflow with selector `qa-optional-forwarding-diagnostic`, runtime required, a fresh run nonce, and the unchanged public signing and artifact privacy controls. Its actual source/run/artifact identity must be retained before any conclusion. This diagnostic does not inherit prior K or S01 runtime results and grants no final 179-requirement acceptance.
