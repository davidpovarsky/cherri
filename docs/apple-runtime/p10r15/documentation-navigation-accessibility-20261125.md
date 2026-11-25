# D03 documentation navigation accessibility repair — 2026-11-25

## Observed failure

The complete iOS run [45532537268](https://github.com/davidpovarsky/cherri/actions/runs/45532537268)
on `a25a65784c22e53bd1f110e1fc7baaaab13daac6` ran 70 tests: 69 passed,
one failed, and none were skipped. The remaining failure was
`CherriEditorUITests/testD03CurrentDocumentationSearchUI()`.
The original encrypted xcresult summary was source-bound and privately validated.
Its exact failure showed that the first result button did not exist after the
search field had already been verified to contain the complete query `show`.

A focused native component harness compiled the actual production
`DocumentationSearchView.swift` and `DocumentationIndex.swift`, with the
original generated documentation bundle, on the existing Scaleway Mac.
It did not substitute search data or mock accessibility.

## Causal observation and fix

The native baseline reproduced the problem on iOS 27 / Xcode 27.0 (27A266a):

- The typed query matched `show`.
- Ten result accessibility elements existed.
- The first element was the expected `show, Actions Reference` and was hittable.
- All result elements exposed the generic `.other` role (XCUI raw type 1).
- No result button existed, so the original button readiness contract failed.

The row's explicit grouped accessibility element lacked its actionable
navigation role. The production fix adds `.accessibilityAddTraits(.isButton)`
to that group. It preserves the existing NavigationLink, label, identifier,
search ranking, navigation destination, query state, and migration filter.

The same native harness then found ten buttons (XCUI raw type 9), including
the hittable first `show` result. It completed real navigation to the section,
the exact title and `show(input: AnyContent)` signature check, back navigation,
a missing-result query, and migration opt-in while retaining the query.
It emitted `CHERRI_DOCS_COMPONENT_NAVIGATION=PASS`, with xcodebuild exit 0.

No protected Swift test, timeout, expected title, or source-preservation
assertion was changed. The full app's unchanged D03 test remains required on
the final integrated candidate; this component result does not itself establish
the surrounding editor-preservation contract or all 179 requirements.

## Exact source and original evidence

Base: `6f083ca95b9b6f6cf39932eb6985fac140959c50`.

Production file SHA256:

- Before: `4b56578baa41732e7be8f15e4a25d0deb5ae5f8f0a40ae26cb172a9741fd14b5`
- After: `831b577f91ef657f173c2d37b357b109e855252632954850093dae20a506f2e6`

Canonical harness sources and the byte-identical validation receipt are retained at
`/mnt/development/projects/evidence/cherri-p10r15-apple-completion-20261125/`.
The harness source directory is `d03-native-harness-source/`; the receipt is
`d03-native-red-green.original.json`.

The original xcodebuild logs, receipts and xcresult bundles remain on the Mac at
`/Users/m1/builds/cherri-d03-component-native/`.

| Original | SHA256 |
| --- | --- |
| UITests.swift, identical for red and green | `316b98a194ebecc16d58150b8290dcde99ca2cd5feef28dda89415600c327615` |
| baseline02-xcodebuild.original.log | `6b65872c3a3eee984626bf6e3fd8d11b25937449a4325be5f446876eefec80f5` |
| trait-fix01-xcodebuild.original.log | `7252c2954d81c2fb735c0aefb3b413743777ad9cf47cae53de8660c234563f88` |
| red-green-validation.original.json | `088df4f8c7cef37239cc38cd9ea6921702eb7a058b655be95e0cfbb57639e537` |

The native baseline failed with exit 65; the fixed run passed with exit 0.
The test durations were 49.084 seconds and 47.904 seconds respectively.
The earlier baseline setup attempt stopped on an owned generated untracked
project before starting xcodebuild; that receipt is also retained.
