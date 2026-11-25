# Case 35 naming oracle correction — 2026-11-25

## Decision

Case 35's correct expected text is **CHERRI_QA_35|qa:qa**. The original expectation **CHERRI_QA_35|qa.txt:qa.txt** came from an incorrect assumption about native name retrieval. This is an explicit oracle correction. It does not repair or alter compiler output.

The source assigns the filename qa.txt to a text item, reads that item's name through getName, and reads its Name property. The established native contract exposes the stem qa through these name reads; the extension is a separate property.

The initial independent audit's case 35 conclusion was wrong. It inferred that retaining the extension while assigning a filename means the name getter returns that extension. The audit's original bytes and conclusion remain preserved. This addendum supersedes its case 35 judgment and qualifies its earlier statement that all 39 expected strings were independently supported. The other cases are outside this correction.

## Evidence predating the differential result

1. The [living master plan](CHERRI-V2-LIVING-MASTER-PLAN.md), section 4.3, route C, explicitly records setName(item, "sample.json") followed by getName returning sample. It states that getName strips the extension. Git blame pins this contract to **b47f6816a59ac119015d777006248f788f7a00f3**, dated 2026-10-14, before this suite existed. The retained version of that document hashes to 14c0d5ba9bc7f7477d1f130c14340723f0987ca3937a5eaa262d778f8ab7caa2.

2. The pre-existing [file materialization diagnostic](../../tools/shortcuts-simulator-qa/file_materialization_diagnostic.py) obtains namedStem through getitemname and namedExtension through the File Extension property, then joins the two with a dot to reconstruct boundary.json. Its introduction is pinned to **1c642be259a496978a201769e2ba3aa39230fee6**; the retained file hashes to edbbea85d38c2332a5ad4a5393d3b90687c935776f4a9aecec9e9b7ef6a19f01.

3. The earlier [native diagnostic audit](../apple-runtime/p10r15/r2-file-materialization-20261119-audit.json) records source 4e56dbcfe895baa8f944b83715ecff0d07bfd24e and run 44634901025. Its NAME64 observation reconstructs boundary.json through that separate stem/extension route. The audit hashes to 38c648b99e0550fe56c85a7e03b747e6913230a153d63a3b5a0807383c12f13c. That whole run remains FAIL_RUNTIME_PRESERVED because four independent content-type expectations failed; this naming observation does not promote the run.

4. The extracted [Apple action metadata](../apple-runtime/macos-27/builtin-actions.json), SHA256 8fb303591888bcb01fe5935b1571d590add428c9bd044f0ab52dcdf99490571a, describes WFDontIncludeFileExtension as a setter option governing automatic extension creation when no extension is specified. It is not a getter option. The getitemname metadata has no extension-inclusion switch; its optional switch concerns fetching a URL's web-page title. Case 35's input is Text, not a URL. The snapshot manifest binds this metadata to macOS 27.0.1, build 26A434, generated 2026-10-15.

The authored case omits the optional setter flag. Its canonical false/default meaning therefore cannot establish that getName returns a full filename. Inspection of the retained compiled actions also shows WFName still contains qa.txt; compilation did not truncate the assigned literal.

## Preserved original results

The fresh recovery run independently reported CHERRI_QA_35|qa:qa for both current original and fork V2. Those observations corroborate the pre-existing contract. Their statuses remain **OUTPUT_MISMATCH** against the original declared expectation.

Preservation directory:

/mnt/development/projects/evidence/cherri-differential-30-20261125/compatibility/case35-oracle

It retains the complete v1 manifest, original independent audit, native run manifest/results/summary, both exact callback completion files, and pinned prior evidence. The preservation receipt SHA256 is cc33dff75d34bc38797c79577da8717fe197928e0e2b2df72d0ae136908892e0.

| Preserved item | SHA256 |
| --- | --- |
| v1 differential manifest | ea664fe90090e76ac60835f49c72abd35b9c8bf708f01d0bb1a0c1591aa31388 |
| Initial independent oracle audit | 4dc1c5a2e0d5add207d1d0059e3d9b9436579bb1999a2290271722f8ccc32ccd |
| Original native results snapshot | 74a806c54caa763787dced5f89b05ff267a5a70c768d07522fe98488300ffd37 |
| Prior Apple naming metadata extract | 515a2ca097a4f456d36073a5c6613712ac6be85a7bc45d6eb530c3bf77e6b143 |

## Change and validation boundary

Only case 35's expected value and oracle reason change in the suite manifest. [The correction ledger](../../tests/differential_30/oracle-corrections.json) records both expectations, both manifest hashes, prior source bindings, preserved failures, and this audit correction. The corrected manifest SHA256 is 282e847654ec9edd6bfec97e4dcc5e549694dbc18b7ee526c5a56856859a23f8.

All authored Legacy/V2 programs, other expectations, production code, and compiled artifacts remain unchanged. PropertyUserInfo integer-zero versus absent remains a visible typed difference with unknown general native impact; this result introduces no normalization rule.

Root must run original, fork Legacy, and fork V2 case 35 again under fresh names using the corrected expectation and strict raw UTF-8 length/SHA256 checks. No historical receipt is relabeled PASS. At this correction, fresh three-lane verification remains NOT_RUN. No simulator interaction, signing, compiler execution, or full Go suite was performed for this manifest-only change.
