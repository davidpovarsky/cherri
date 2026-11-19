# Empty List yield: native cardinality failure preserved

[Run 44431490222](https://github.com/davidpovarsky/cherri/actions/runs/44431490222) executed exact source `ff04ee8887c701824e5161f3fff1431ff204bb65`. Import, signing, library selection and an identity-verified SUCCESS callback occurred. At 2026-11-19T02:37:06.556Z the callback was:

```
CHERRI_QA_F04_EMPTY_LIST_RESULT:outer=0:scalars=2:shape=COUNT_MISMATCH
```

The unchanged oracle required `CHERRI_QA_F04_EMPTY_LIST_RESULT:outer=2:scalars=2:shape=0,0`. Two iterations yielding an empty List therefore produced zero collected elements, while the scalar control retained two. The signed 59-action Shortcut SHA256 is `d8e102a6f8b487afb2855b5ce5c304ae19ddfc80bacaab08b0b3db26c00a272a`. Both strict runtime/trace gates failed because the result differed from the expected contract. This is an observed native cardinality failure, not an import or harness failure.

The independent audit verified all 20 published hashes. Its causal oracle mutations are NOT_RUN_BASELINE_NOT_PASS: a failing baseline cannot establish successful negative controls. Thirteen raw/raster inputs were quarantined. Editor status remains NEEDS_REVIEW; no physical-device result is claimed. The retained artifact field `compiledShortcutRuntimeExecuted=false` denotes the failed strict contract; the separately preserved SUCCESS callback proves that the actual target did execute.

This corroborates the earlier nonempty List-yield observation of outer count four instead of two in run 44431486363. The later analyzer guard rejects statically known List-valued value-loop results with E_TARGET_VALUE_SHAPE. The guard is a fail-closed boundary, not native nested-List support, and does not close F04. Unknown/AnyContent shapes remain unverified. All historical fixture expectations and the observed failure are retained unchanged.

The original source fixture, sanitized publication, actual run metadata and independent audit are preserved byte for byte. `archive-manifest.json` binds the archived files.
