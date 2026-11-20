# Copied-Skill signing harness repair — 2026-11-20

## Observed failure and bounded change

Original OpenMinis run44720295409, attempt1, at381579cd59bf4fb59787f6bbb31fdcb772131191 remains FAILED. Its signing report has34 checks:2 unsigned CLI controls passed,21 CLI fault cases rejected without an artifact, and11 copied-Skill cases failed before reaching the local signer. Every failed copied-Skill route reports the missing scripts/json-object.awk required by the unchanged common.sh. All7 original ZIPs,10 original members, GitHub API metadata and the original job log are retained and independently reconciled in this archive.

Implementation7e36478a9ab287bbb038756cde2957294ac4f232 changes only the copied fixture dependency list in tools/p10r15/verify_installed_signing.py. It copies the same repository JSON parser beside common.sh. It changes no compiler behavior, expected verdict, output rule, signing endpoint policy, timeout, compatibility manifest or workflow.

## Actual verification

The draft replay passed34 checks. The source-clean replay passed the same34 checks in81.172 seconds, exit0, zero failures, clean before and after. This includes3 unsigned controls and31 observed loopback fault rejections. The per-call55-second harness budget and original compiler timeout remain unchanged. The complete34 stdout rows match the complete original JSON report. All negative cases made exactly one multipart request, exited nonzero and left no target artifact. Unsigned controls made zero requests and produced parseable native workflows.

This used the existing authenticated Linux amd64 compiler built from clean source fd691b96fb661697a1cbaddab92476d457636ba9, binary SHA256 cedaada3df00b89b6d8fd6d863715397088f1b3a4ebac5ade47cc054cb2ab1bb. The actual build, inspector and capability receipts are included; the executable remains at its original evidence path. This is protocol/exit/artifact evidence, not an Apple cryptographic signature or native runtime claim. No new workflow dispatch or final179 credit is asserted.

## Append-only correction to prior consumer proof62a

The prior392-member consumer archive contains389 independently matched original files and3 sealed launcher-context strings. Original disk paths and authenticated submitted payload identity for those3 strings have not been established. They must be described as sealed context only. The prior proof and archive are unchanged. consumer62a-provenance-correction.json identifies all3 exact members and hashes; the independently retained root audit receipt is included.

This clarification does not alter the independently verified D03 documentation results,20 process receipts,26 K05 run-to-pass observations, original failed and subsequent118.39-second real-source replay, or3 actual package ZIP hashes. App integration, complete consumer promotion and final acceptance remain open.

The new replay scripts in this proof are separately captured from the actual executing __file__ before execution and bound by their process receipts. Normalized connector notifications are explicitly labeled; they do not replace original raw stdout/stderr or the GitHub artifacts.
