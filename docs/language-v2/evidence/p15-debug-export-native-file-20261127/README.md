# Native File carrier evidence — revision 03

Prepared 2026-11-27 for repository publication. This directory contains exact acquired archives, exact acquisition and independent intake receipts, selected original source/plist/runtime records, a complete [hash manifest](manifest.json), a [bounded privacy scan](privacy-scan.original.json), and an [ordinary Shortcuts handoff](ordinary-shortcuts-handoff.md). Revisions 01 and 02 remain unchanged. These files are staged for copying; this preparation did not commit, publish, sign, import or run a shortcut.

## Result and scope

**Native File acceptance remains open.** The original carrier preserved bytes and name but returned a Dictionary. The guarded archive candidate never reached engine execution. Separately named linear direct and zip programs reached Make Archive and failed. A separate bounded NSError observer recorded an explicit access-denied message; it did not identify the denied resource, permission, entitlement, policy or remedy.

The retained compiler is source [5bc62d057e4bfa8bdd895e7228ca59eedac20b40](https://github.com/davidpovarsky/cherri/commit/5bc62d057e4bfa8bdd895e7228ca59eedac20b40), with binary SHA256 278834c032c2ec3a246b2909b607b907b79d33ce5c19be05dfc853299464278a. No binary was rebuilt or executed for this publication revision.

| Retained experiment | Engine calls / successful actions | Observed outcome |
|---|---:|---|
| Original independent witnesses | 1 / 13 | Binary, Text and Dictionary produced WFGenericFileContentItem, WFStringContentItem and WFDictionaryContentItem; each Count was 1. |
| Original Encode → Decode → Set Name baseline | 1 / 10 | Exact 83 bytes, Count 1 and the expected name survived; the producer was WFDictionaryContentItem. The required File class failed. |
| Original guarded archive candidate, 14 actions | 0 / 0 | ORIGINAL_ACTION_IDENTITY rejected construction; no engine or carrier outcome. |
| Separate construction-only identity diagnostic | 0 / 0 | Original indices 0–4 matched. Original index 5 had actual index 6, with the same identifier and UUID. |
| Separate direct program, format .zip, 11 actions | 1 / 1 | Text 0 matched the 83-byte oracle. Make Archive 1 failed; actions 2–10 did not complete. |
| Separate zip raw-source program, format zip, 11 actions | 1 / 1 | Same observed Make Archive failure. Changing only the format after explicit identity mapping did not resolve this run. |
| Separate bounded-error observer on the exact zip program | 1 / 1 | Same failure, with two bounded error records saying access was denied. This was a diagnostic observer variant. |

Successful-action counts exclude callbacks that carried an error. An owner exit code of 0 or ENGINE_CALLBACKS_RETAINED_VALIDATION_PENDING means the capture was retained; neither means workflow success or acceptance.

## Original negative baseline and unexecuted candidate

The original payload is exactly 83 UTF-8 bytes, SHA256 539a079afd9ef25a7e6c893494300f86f9f8db22c9dc4b58bbb433d4e492c365. The baseline kept those bytes and the expected name, but its actual decoded and named producer was a Dictionary. A .json filename, matching Base64 or declared File return type does not change that observation.

The separate identity diagnostic retained both original identity and UUID guards. It added an event before the guard and an unconditional stop before engine setup. Original index 5 was is.workflow.actions.getitemfromlist, UUID 052D79F3-4DE7-5039-A7B1-7A4C2E61354C; actual index was 6 while the identifier and UUID matched. The guard rejected before the explicit stop was reached. Nothing identifies an inserted/missing action, its cause, or the unobserved original indices 6–13. The guarded candidate remains NOT_RUN by the engine.

The independent intake preserved the strict stored-text helper's subclass rejections and the actual captured NSString subclasses; it did not rewrite native values to fit that helper.

## Direct and zip: two retained runtime failures

Both programs passed the complete extracted collection directly into Set Name, with Count before and after. Neither contains If or First Item. In each actual execution the model bound all 11 original actions; the initial Text matched the independent oracle. Make Archive at index 1 then returned NSError, domain VCVoiceShortcutsErrorDomain, code 1011, output cardinality 0 and a null callback UUID despite a bound model UUID. The terminal callback reported the same error, with cancelled=false. Actions 2–10 did not complete, so no extraction, count, name, bytes or File observations followed.

The unchanged original-stream validator rejected ACTION_MODEL_BINDING. Both failures and all original capture statuses remain intact. No meaning is inferred for code 1011 or the null callback UUID.

Committed Apple metadata distinguishes format id zip from display title .zip and declares default zip. Its snapshot build is 26A434; these retained runtime observations used 26A428. The canonical makeArchive source currently declares dotted enum values. The zip program therefore used the already supported native.action/native.ref source mechanism and the same compiler, with no hand-edited plist or production change.

A full-workflow comparison mapped exactly 10 producer UUID fields and 9 exact ActionOutput reference UUID fields. Output names, headers, other parameters and literal strings were not ignored. The only remaining difference was WFWorkflowActions[1].WFWorkflowActionParameters.WFArchiveFormat, .zip → zip. Source and plist identities remain distinct. [The original comparison](evidence/native-file-archive-zip/originals/normalized-graph-comparison.original.json) and source/plist diffs are retained. This variation did not resolve the observed failure and does not establish its cause or repair the canonical action.

## Bounded NSError observation

The diagnostic reused the exact zip source and plist without compilation. It added one helper and changed two error expressions in a separately pinned observer. Reversing those changes restores the original observer bytes exactly. Original error fields, action/UUID guards, callback control, engine setup, capture limits and permissions were unchanged.

Both action and terminal errors recorded:

| Field | Original observed value | UTF-8 bytes |
|---|---|---:|
| localizedDescription | The operation couldn’t be completed. Access was denied | 56 |
| localizedFailureReason | Access was denied | 17 |
| localizedRecoverySuggestion | NIL | — |
| NSUnderlyingError | NIL | — |

Each record contained 73 text bytes and 464 added serialized metadata bytes, within the variant's limits. Only the three allowed text getters and one underlying-error key were eligible; userInfo was not enumerated or dumped. No second underlying error was read.

The evidence identifies an access-denied result in this headless context. It does not identify a specific denied resource, permission, entitlement, policy or corrective action. No permission change was made. This is not a successful original-observer run and provides no File or macro acceptance.

## Original records

All links below point to exact byte copies. The manifest additionally records every member digest for all five archives, selected extracted originals, full source bindings, native pilot/event/result pins and predecessor revision hashes.

| Evidence set | Original archive | Independent intake |
|---|---|---|
| Witnesses, baseline, guarded candidate | [archive](evidence/native-file-original/originals.mac.zip) | [intake](evidence/native-file-original/independent-intake.original.json) |
| Construction-only identity | [archive](evidence/native-file-identity/originals.mac.zip) | [intake](evidence/native-file-identity/independent-intake.original.json) |
| Linear direct | [archive](evidence/native-file-archive-direct/originals.mac.zip) | [intake](evidence/native-file-archive-direct/independent-intake.original.json) |
| Zip format variation | [archive](evidence/native-file-archive-zip/originals.mac.zip) | [intake](evidence/native-file-archive-zip/independent-intake.original.json) |
| Bounded error observer | [archive](evidence/native-file-archive-error/originals.mac.zip) | [intake](evidence/native-file-archive-error/independent-intake.original.json) |

All independent intakes checked original event/pilot joins, archive and member hashes, source snapshots and owned process closure. Parent reaping and process-group ABSENT were retained. The per-owner temporary-directory observation is not a claim that the OS made no temporary writes elsewhere.

## Remaining acceptance and publication boundaries

The [next handoff](ordinary-shortcuts-handoff.md) belongs to the primary Apple owner and the existing ordinary Shortcuts route. No additional headless replay is proposed here. Full debugExport, canonical makeArchive, signed import, physical iPad, formal 179 acceptance and the original guarded candidate remain separate obligations; none inherits PASS from this publication or an error description.

Fixture content is synthetic. Host paths and process/job identifiers retained in originals are provenance, not credentials. A bounded scan covered all five archive member streams, ten acquisition/intake receipts, staged prose and the final manifest for the named credential/private-continuity patterns. It found no matches. This is not an environment-wide or exhaustive secret audit. No private conversation/session continuity identifier was added to public prose or metadata.
