# Authored native Text reference carrier repair — 2026-11-25

## Finding and independent contract

The pre-repair 39-case differential run identified a real shared compiler error in case 33. The current upstream compiler produced the expected URL components. Both fork frontends emitted empty components in native execution. The declared expected value remains `CHERRI_QA_33|https:example.org:/alpha/beta:mode=qa:section`.

The canonical DSL and Apple's captured `WFURLGetComponentAction` metadata agree on the input key `WFURL`. The difference is the parameter envelope. Upstream uses `WFTextTokenString` with one actual object replacement character and a `{0, 1}` attachment range. The fork incorrectly changed that envelope to `WFTextTokenAttachment`; the inner variable or producer reference remained identifiable.

The independent evidence is already captured in this repository:

- `docs/apple-runtime/macos-27/builtin-actions.json`: `is.workflow.actions.geturlcomponent`, input `WFURL`, `WFTextInputParameter`.
- `docs/apple-runtime/macos-27/parameter-encodings.json`: `WFVariableStringParameterState` for the affected text fields, including custom split/join separators.
- `docs/apple-runtime/macos-27/encoding-table.json`: both `variable:MyVar` and `variable:ActionOutput` serialize as `WFTextTokenString` for VariableString state. Their NumberString counterparts serialize as `WFTextTokenAttachment`.
- `tools/shortcut-runtime/required-encoding-expectations.json`: the required text-state forms are string/token-string forms.

These are parameter-state observations, not an inference that all native actions accept every possible envelope. The prior fidelity-promotion fixtures establish imported wire preservation; their bare attachment examples do not establish the correct default for authored text fields on this runtime.

## Shared repair and preservation boundary

`argumentValue` now follows its canonical declared String parameter and the existing text encoder. This fixes URL-component input, the six case-conversion aliases, spelling, split input, URL opening/detection/expansion and dynamic split/join separators through one shared policy. No URL-specific plist rewrite or alternate action catalog was introduced.

The numeric image exceptions keep their NumberString attachment encoding. A join's primary input remains a declared Variable carrying a list. Quantity magnitude and date-field policies remain independent and unchanged.

Imported bare attachment parameters remain representable by their natural calls. The importer now renders an actual bare Text attachment slot using the existing explicit `rawRef(...)` / `native.ref(...)` source boundary. A compiler-only `ExplicitNative` origin travels through the IR, backend reference and shared descriptor. Only a direct reference argument at an already supported imported-attachment slot uses that intent. The marker is not an Apple dictionary field and is not emitted in the native plist. There is no new public syntax and no workflow-wide mode switch.

Imported token strings continue through the interpolation renderer, including token strings whose inner attachment requires an explicit reference for an exact output name. The outer text encoder owns that token-string envelope. A new ordinary authored reference appended to the same imported workflow uses the corrected text envelope. Import provenance's metadata allowlist and semantic hash remain unchanged.

The schema generator no longer infers bare attachment support solely from a Text parameter's `variableTypes`: those kinds describe reference contents inside the token string. Generated accepted forms now match the captured state envelope. The registry and complete action reference are regenerated; the codec fingerprint records the new shared contract. The Skill compatibility fingerprints are updated, with the compiler commit pin reserved for a separate integration metadata commit.

## Verification

`native_text_parameter_test.go` checks 18 actual text argument slots across both production frontends and both action-output/mutable reference kinds (72 observations). It compares exact decoded parameter maps with captured Apple state samples, verifies literal URL behavior and the join list attachment control, and rejects bare-envelope, escaped-placeholder, missing-reference, wrong-range and wrong-producer corruptions.

A mixed import fixture preserves bare and token-string envelopes in the same workflow, including an inner explicit output-name override. It verifies natural classification, exact pre-overlay semantics and strict recursive post-overlay equality, then compiles a new authored call appended to each imported source. The unchanged P13-FID-02/03 import fixtures also pass their existing natural-count and exactness requirements.

The bounded regression run covers quantity producer identities and snapshots, constructor arity/type/literal failures, RP11 numeric/text parameter samples, Date reference codecs and custom codec families. No full Go suite or native execution was run by this component task; the integration owner runs those gates with the final compiler.

Evidence directory: `/mnt/development/projects/evidence/cherri-differential-30-20261125/compatibility/url-text/`. `diagnosis-inputs.json` pins the primary input hashes and original/fork baseline artifacts. All receipts retain their actual result; early test-harness setup failures and the intermediate authored-only run remain present and are not counted as the final gate.

| Receipt | Exit | Log SHA-256 |
| --- | ---: | --- |
| `boundary-green` | 0 | `7772f2c4b096f95535003b7cf38029a04f4e3b298b47d16550ff27204af28f23` |
| `schema-red` | 1 | `eca450046f581854e4f60bf404cc5f165d2e0bc5d822d263e400e9beefaf051e` |
| `production-import-boundary` | 0 | `5200ed478be6c711eb1f327d93cb5348e809bc805ff67a015aa464ce5f5a3248` |
| `production-red-v2` | 1 | `96ebcacf6807a4c23685dac3037a4ffa47cb47e36ded424a5c0be8134b345dd0` |
| `reference-verify` | 0 | `eb36a084f1d57ee848b414c0fa9a1608e0951565267a9a90c59dcadcc5f638fb` |
| `schema-verify` | 0 | `ef03b7be992784093a638f4565b3281a809f76abe47162c1381b981d23e79fda` |
| `schema-green` | 0 | `673a47df16f36c1c1c11b198251af7bbe6c16dc9f7fec8a333d16a24bc29d1ad` |
| `shared-green-v2` | 0 | `284e5d7c400ecbe88f3b7de142f2ceb6f5138d816b0c5655f66b926793add4e7` |

The new generator test fails before the repair and passes afterward. Generated-registry verification and complete-reference verification pass. Native case 33 and supplemental neighboring-text fixture 41 still require fresh uniquely named signed artifacts and strict artifact identities from the integration owner; these focused tests alone do not claim native execution success.

## Difference taxonomy

The carrier change is a preserved typed difference with demonstrated relevance to the case-33 failure. It is not a blanket normalization rule. The separate `PropertyUserInfo: 0` versus missing-field difference remains visible and has unknown native impact. The case-33 source programs and expected output are unchanged.
