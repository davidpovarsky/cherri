# Imported native OutputName field-presence control

Exact clean test source: 1315db3d367674e1f23d2b298ec45fec5609703a, containing the previously verified production fixes at 06da801. Four named cases (three children and their parent) passed with zero failures or skips. The 77.35-second wall time includes the first cold-cache rebuild after the coordinated build-cache cleanup.

The test constructs an actual native output attachment, sets its OutputName field to absent, explicitly empty, or a nonempty name, and imports/recompiles it through both Legacy and V2. It checks strict whole-workflow identity and separately checks the actual native field's presence and exact value.

Observed representation:

| Native field | Natural actions | Exact fallback actions | Stage-3 demotions |
| --- | ---: | ---: | ---: |
| Absent | 2 | 0 | 0 |
| Present, empty string | 1 | 1 | 1 |
| Present, named | 2 | 0 | 0 |

The explicitly empty native field is retained exactly through the existing fail-closed fallback. This passing wire-preservation control does not claim natural representation or zero Stage-3 demotions for that case. Authored native.ref clear-name syntax and an imported native field containing an empty string are distinct contracts. The current representation limitation is preserved in these results; no production change or relaxed parity oracle was introduced by this follow-up.
