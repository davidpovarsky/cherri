# P15 type acceptance: bounded production evidence

## T02: literal-required arguments

The historical probe compared Number assignment with Text. It never called a literal-only parameter. The replacement executes the production parser, analyzer, complete compiler and native emission against a confirmed native enum parameter, declared literal parameters, and compiler macro literal options.

Causal RED on base `48828b7ba7eada5237396ef5461e91d13cf286d4`: Sentinel job `job_21cbeb1d7692`. Runtime immutable bindings produced the wrong diagnostic code. Dynamic concatenation, interpolation, control expressions, declared `Literal: true` parameters and require options bypassed analysis entirely. A folded invalid enum value also bypassed enum validation.

The analyzer now enforces explicit literal requirements independently of inferred native evidence. For native accepted-form metadata, it enforces confirmed literal-only slots. The diagnostic is `E_LITERAL_REQUIRED` on the actual argument. Expressions are checked recursively so a dynamic value cannot hide behind concatenation or interpolation. Runtime `let` snapshots are not treated as compile-time literals.

Existing eligible scalar literals, pure text concatenation/text interpolation, pure Boolean expressions, enums and literal containers remain accepted. This is deliberately bounded to expressions already lowered without runtime evaluation; it does not introduce a general constant evaluator or expand numeric arithmetic folding. Folded text is checked against the same enum domain as direct text. HTTP body/header dictionary parameters retain their established literal-container contract with dynamic leaf values.

`TestP15T02LiteralRequiredProduction` covers 13 runtime-value rejection cases, exact stable diagnostic code/span, complete compile failure before plist emission, 12 primary/named literal cases with exact native enum output and no extra actions, declared literal/macro positive cases, folded-invalid-enum rejection, and dynamic HTTP dictionary leaf preservation. Wrong diagnostic code and wrong span mutations are rejected. A list index additionally has its independent optional-value type diagnostic; the test requires the exact literal-required diagnostic without suppressing that type error.

Execution:

- Focused T02 GREEN: `job_85f8f72a1927`, root package 0.230 seconds.
- Full analysis, service and lower packages: `job_e0544a8f76bd`, PASS (1.554 / 1.643 / 1.166 seconds).
- Existing require/Release tests and HTTP literal/dynamic local echo, raw file request and imported wire edit tests: `job_2cdcf25c2ab3`, root package PASS in 8.329 seconds.

The acceptance runner executes the named production test and records only the contract's required unit tier. Missing or unauthorized producer records fail closed. The 179-obligation ledger and unrelated CLI/parser evidence are preserved.

## T03: semantic Number and exact number-as-text wire

The dedicated producers execute the canonical parser, analyzer and complete compiler. Independent checked-in Apple metadata identifies `WFNumberAction` as `NSDecimalNumber` output and `WFNumberActionNumber` as a `WFNumberFieldParameter`; the captured parameter state is `WFNumberStringSubstitutableState`. The same native metadata independently identifies Round Number's numeric output and substitutable input state. No catalog output or codec is changed by this stage.

`TestP15T03NumberStringSemanticSignature` checks Number return inference and the public Number input signatures of `wait` and `round`. It compiles explicit conversion from literal and produced Text and rejects an unconverted Text producer in a Number parameter with the exact diagnostic span. Existing numeric string literals are intentionally eligible in numeric native fields; that compatibility is not generalized to arbitrary Text action output.

`TestP15T03NumberStringNativeWire` independently decodes emitted plist values. Both Legacy and V2 preserve the exact strings `0`, `-5`, `42.125` and `9007199254740993` in the native Number field, and the consumer references that actual producer UUID. V2 Round Number likewise retains those strings in its public numeric parameter. A separate Text-to-Number-to-Wait graph checks both reference edges. Replacing the string with a plist numeric value, changing the value, retargeting the consumer, or substituting a different native action is rejected for each frontend/value case. These checks establish serialization and source typing, not Apple arithmetic precision or execution.

The initial test construction exposed two test errors, retained in `job_473fb45e3045`: it incorrectly rejected a numeric string literal already permitted by the compiler, and the Legacy fixture omitted its established whitespace around `=`. Corrected actual compiler controls passed in `job_b07ec31eec48`; neither correction changed production behavior.

A separate genuine evidence-gate defect was reproduced in `job_eebffce213d6`: a semantic-only test identifier could be relabeled as the required native-structure tier and mint T03 success. The validator now resolves each T03 tier's authorized producer from the existing named production test inventory. Missing, wrong, cross-contract and cross-tier producer controls remain mandatory. The earlier `job_bb158e783256` failed the new wrapper's `native` versus normalized `native-structure` naming before reaching that mutation and is not cited as the causal defect.

Only T03's unit and native-structure obligations are eligible for this producer. Signed runtime, numeric precision, other type contracts and overall release closure remain unproven by these tests. The exact clean source execution receipt is recorded separately after commit.

## T04: opaque type knowledge, value use and actual service diagnostics

The named production tests distinguish canonical Unknown (for example an opaque selection result), AnyContent (external Clipboard content), explicit Number conversion output, and Void statements. They execute the real analyzer, complete compiler and Service APIs. The formal producer requires a distinct unit and service test identifier; missing or substituted tiers cannot mint a pass.

Causal RED `job_e50f39223916` demonstrated 13 unchecked paths: Unknown/AnyContent could acquire concrete or Void annotations, typed function arguments/defaults/returns, mutable contracts and homogeneous collection types; opaque arithmetic and captured/coerced Void could also manufacture values. Existing Void arithmetic rejection was a positive control. Earlier `job_0888bb58e0bc` used a nonexistent action name and never reached this contract; it is a fixture error, not compiler evidence.

Authored type guarantees now reuse the existing recursive structural evidence relation. They are separate from the existing native parameter attachment compatibility: sending an attachment to a native action does not establish a user annotation. No cast, action, serializer, registry or runtime validator is introduced. Numeric arithmetic requires existing numeric evidence, or an explicit native conversion producer. Void remains valid as a statement but is rejected in the tested value positions: bindings, argument values, views/indexes, containers, interpolation and operators.

Explicit empty List and Map literals retain their vacuous element/value compatibility; opaque collections are not considered empty merely because their element type is Unknown. Optional destinations accept an existing value under the inner contract, and an explicit `none` literal remains an absence only for an Optional destination. This preserves existing optional authoring without conflating JSON null with omission; T05's serialization contract remains separate. An uncommitted hardening candidate rejected an eligible empty List (`job_768e9c04463a`); the correction is limited to the authored empty literal and optional destination rules and has paired opaque negatives.

The historical T06 positive `let payload: JsonValue = ShortcutInput` was an unvalidated external-content assertion. T04 now tests its rejection explicitly. T06 retains a positive JsonValue-to-JsonValue function forwarding case, together with its unchanged record rejection and native-content controls. This does not turn a JSON annotation into runtime validation.

Service checks inspect actual contextual Unknown/Void returns, exact diagnostic category and span for AnyContent/Unknown/Void, and replacement of an invalid source with an explicit Number conversion without stale errors. Each negative also requires complete compilation to stop before native bytes, with causal wrong-span controls. This producer claims only unit/service behavior of the canonical type model; it does not validate every catalog output declaration, Apple conversion success, signed runtime or release closure.

Working validation before the final source freeze: all 61 internal types/analysis/service/lower top-level tests passed in `job_c74e8fc3c377`. Its root compatibility run exposed the obsolete JsonValue assumption above. After correction, `job_dae3efa2574a` passed 21 root production families (including T02/T03/T06/T08, V02, raw bindings and Legacy scoped values) and all five named type evidence wrappers. Final exact clean source evidence follows separately.

## T06: record annotations require prior structural evidence

The original probe constructed a RecordType in memory. The replacement executes authored annotations through the parser, analyzer and complete compiler. Opaque external input, Run Shortcut output, a native dictionary content view and require's dictionary-category guard cannot establish a user-defined record shape. The rule follows optional, List, Map and union wrappers; it does not convert those values or insert a runtime validator.

Causal RED on clean T02 source `33cd20c0d4256dff063c5f4b8e261fd6d0755985`: job `job_700bb4fd526c` demonstrated unchecked external record annotations, mutable assignment, declared returns and unknown record-field narrowing. The same test also found that a record type's optional suffix did not parse. The first correction exposed a separate call-path defect: user-function arguments were evaluated but never checked against parameter annotations. The correction now checks the production lowerer's primary/named/positional argument binding order, including underscore-prefixed public parameter names. Existing default-value analysis remains in place.

Unknown and AnyContent retain their ordinary content compatibility but cannot manufacture a record contract. Within an established record, an unknown field cannot become Text or another concrete type through an annotation, including nested List and Map fields. Optional fields cannot silently become required. Verified record identity, structural subsets, optional destinations and union destinations remain accepted. This is a bounded record-proof correction; it does not claim that the separate T04 Unknown/AnyContent/Void contract is complete.

The production tests check four external origins against five record destinations, mutable assignment, primary/named/later positional function arguments, optional and list-valued record parameters, defaults, external and JsonValue returns, uncertain fields and field presence. Every negative case requires the diagnostic on the actual argument or return span and complete compile failure before native bytes are emitted. Wrong diagnostic-code mutations are rejected. Positive controls compile ordinary external-content use, verified record forwarding, optional/subset/union relations and ordinary Text/Number values. Unannotated external results remain opaque.

Execution before commit:

- Initial record correction exposed the remaining function-call bypass: `job_e5016d00bc38` (expected failing test).
- Focused production T06 GREEN: `job_927de422d68e`, root package PASS in 0.196 seconds.
- Full types, syntax, analysis, service and lower packages: `job_36f326df4273`, PASS (1.219 / 1.196 / 1.759 / 0.963 / 0.917 seconds).
- T02 and T06 production evidence wrappers plus smoke-evidence safeguards: `job_086c25a40c0b`, PASS in 6.991 seconds.

The T06 formal producer records only its required unit tier. It proves static rejection of an unvalidated external record guarantee, not native JSON validation or signed execution. Each producer's bounded evidence test verifies exact contract text, implementation SHA, producer command, diagnostic controls, missing/wrong-producer rejection and unchanged 179-obligation accounting. The clean source receipt is added after the implementation commit.

## T08: literal selection mode and actual result cardinality

The old probe compared two unrelated base64 aliases. The replacement uses the checked-in Apple `WFChooseFromListAction` contract: the input determines the selected element's type, the Select Multiple switch defaults to false, and its native description explicitly permits multiple selections when enabled. The nominal `Output.Multiple=false` field is not treated as proof for every mode.

Causal RED on `b56eaa83e30ef923b0a3e17c0b11385533a9be7c`: job `job_f8eb39f755a0` reproduced fixed Unknown output, first-element-only List and Map inference, accepted invalid scalar/field use, and missing contextual hover. The corrected analyzer joins every List element and Map value with the existing union representation; opaque members do not disappear depending on position. For a known List<T>, omitted/false mode produces T, true produces List<T>, and runtime mode produces T | List<T>. Pure Boolean literals already folded by the compiler retain their mode. Runtime let bindings are not constant-propagated. An opaque input keeps an Unknown element category, including when its multiple-result container is known.

Result inference uses the production argument binder's wire-key associations, covering primary, named and later positional arguments. No native action is added or changed. The tests compile actual selection calls and independently check the emitted input/output producer UUIDs and omitted, explicit or attached native mode. Wrong-mode and wrong-producer mutations fail.

Direct field access on a known possible list result is rejected, while a scalar selected record retains its established field type. A mixed collection cannot satisfy a homogeneous annotation solely because of its first value. These diagnostics prevent complete native emission. This is a cardinality and static evidence correction, not an implicit runtime cast or proof that opaque input has a record shape.

Hover and Signature Help read the analyzer's actual call result at that callee's source span. Different calls to the same action show their own cardinality, and edits change both displayed types and diagnostics. The fixed schema output remains Unknown for contexts without an analyzed call; no blanket catalog guarantee is introduced.

Execution before commit:

- Focused T08 GREEN: `job_7d5075bcb139`, PASS in 0.229 seconds.
- T02/T06/T08 production tests including mode/UUID/diagnostic mutations and editor edits: `job_a9e0972e2f58`, root PASS in 0.229 seconds.
- The same job passed full types, syntax, analysis, service and lower packages (0.926 / 1.332 / 1.268 / 1.542 / 1.331 seconds).
- Actual T02/T06/T08 formal producers and smoke safeguards: `job_fd121b5cc053`, PASS in 7.547 seconds.

T08 records its required unit and service tiers with their actual distinct production test identifiers. Removing the service tier cannot mint a pass. The exact clean implementation receipt is retained after commit. Native metadata and emitted plist inspection establish the bounded static contract; this report makes no signed Apple execution or per-platform availability claim.

## Type obligations

| ID | Current evidence boundary |
| --- | --- |
| T01 | Explicit false/zero/empty/default presence and signed iOS behavior require dedicated evidence; the old base64 smoke is not formal proof. |
| T02 | Dedicated production unit producer described above; clean exact-commit receipt is recorded separately after commit. |
| T03 | Dedicated semantic and decoded native-wire producers verify exact number-as-text values and producer identity; strict tier authorization, no Apple runtime or precision claim. |
| T04 | Dedicated authored-assignment and value-use unit/service producers; strict tier authorization, preserved native attachment compatibility, no runtime conversion promise. |
| T05 | Optional type construction alone does not prove omission versus explicit optional value and JSON null in native serialization. |
| T06 | Dedicated production unit producer rejects unvalidated external record guarantees; no runtime validator or native execution is claimed. |
| T07 | Production source versions now retain and compare components (see correction below). Existing action min/max metadata remains float64 and lacks independent per-platform version facets. No formal T07 pass is claimed. |
| T08 | Dedicated chooseFromList unit/service producer checks native mode facts, emitted wire, actual call types and contextual editor behavior; no signed runtime or platform-availability claim. |

No native runtime, UI, platform availability or release closure is inferred from this local type work.

## Exact committed execution

Commit b250282a269268610bbb865f847e1fbc8e82b57f was executed with a clean tree in Sentinel job job_7aa9e96b50ee. The actual named production producer and formal evidence validator passed. The bounded report has one passed requirement (T02), zero failed/skipped, and closure_complete=false. The receipt and raw-log verification digest are retained under evidence/p15-types-b250282. No other type requirement inherits this result.

### T06 exact committed execution

Clean implementation commit 642d276723cf6a80a5f3ba7f5165454ccafc42d5 passed its actual production producer and formal evidence validator in Sentinel job job_70670aceddf8. The bounded report contains one passed requirement (T06), zero failed/skipped, and closure_complete=false. Receipt and raw-log digest are under evidence/p15-types-642d276; unrelated requirements inherit no success.

### T08 exact committed execution

Clean implementation commit 501a46f6db3c26971037ac4e87ab8f71e1c2251e passed its actual unit/service producers and formal evidence validator in Sentinel job job_30ddabb0b1c3. The bounded report contains one passed requirement (T08), two tier records, zero failed/skipped, and closure_complete=false. Receipt and raw-log digest are under evidence/p15-types-501a46f.

## T06 follow-up: final control results, all branches

A separate causal probe on source cc9ba50853f3573485aebb4c8a942b43ada9437b found a real gap in the initial T06 correction. Ordinary if/menu/repeat/for inference still used the first encountered yield. Job job_da37de3c1b38 reproduced nine unchecked record-return paths, including a known record followed by an external alternative, a missing final yield, an early yield overwritten by a later action, and nested variants. Five mixed Text/Number controls also inferred only the first type. The earlier 642d276 receipt is historical evidence for its listed direct cases; it is insufficient to prove these control paths.

The analyzer now determines each branch's final explicit result with the existing BlockResultExpression helper and joins those results with the existing union/list types. A missing or overwritten final yield remains AnyContent. Final nested control expressions are followed recursively. Existing implicit native-last-action behavior remains untyped; no conversion, native action or runtime validator is inserted. The known record in one branch cannot validate another branch's external content.

TestP15T06ControlResultsRequireAllBranches covers ten negative/control variants, exact return diagnostics, complete compilation blocked before bytes, wrong-code rejection, verified-record positive branches/loops, and five actual mixed-control type/compile checks. The formal T06 producer now requires this test alongside its original authored-value and structural-relation tests.

Validation before commit: job_4e31d2bac06d passed all T02/T06/T08 production tests in 0.276 seconds. Job_00866dd28cfe passed full analysis/service/lower packages (0.911 / 0.965 / 0.896 seconds), existing Require/Measure/ShortCircuit/GeneratedGuard matching tests (0.365 seconds), and all three formal type evidence producers (6.518 seconds). No native runtime or overall release closure is inferred.

### Strengthened T06 exact execution

Clean implementation commit b0d949f4914be45bc50dbd345ff0f475f248969d passed the strengthened formal T06 producer in Sentinel job job_16b88642fee9. Its actual command names all three required production tests, including the final-control-branch regression. The bounded unit report has one passed requirement, zero failed/skipped, and closure_complete=false. Receipt and raw-log digest are under evidence/p15-types-b0d949f.

## Generic version comparison correction (T07 remains partial)

The production analyzer previously converted authored version literals to `float64`: `16.10` became `16.1`, `16.02` was compared as a fractional number, and a three-component literal such as `16.1.9` failed conversion and silently removed the bound. The same defect affected a maximum such as `26.0.1`. Baseline regression job `job_b3e3a1292e02` reproduced those failures through real canonical actions and the language service.

The analyzer now retains the complete source spelling, validates literal dotted decimal components, and uses the existing production `schema.CompareVersions` helper. Empty, malformed, nonliteral, negative, or overflowing bounds produce `E_TARGET_VALUE_SHAPE` at the value. An unsupported action produces `E_UNSUPPORTED_TARGET` at the call, preserving the complete authored version in its diagnostic. An explicit zero bound is checked; an absent bound still uses the existing default. Incomplete editor metadata retains parser recovery. Existing numeric setters/accessors remain compatible; new string accessors expose the preserved components. A float setter or float action-schema field cannot recover digits already lost before the analyzer receives it.

`TestP15TargetVersionComponentsProduction` covers 31 analyzer/actual-compile cases across all four existing minimum-field aliases and the maximum field, using the canonical `isCharging` 16.2 requirement and `askLLM` 26 min/max declarations. It checks exact diagnostic spans, absence of native emission on failure, actual native identifiers/variant parameters on success, and a wrong-diagnostic mutation. Another 26 invalid-bound cases verify fail-closed behavior. Service tests change the same document through accepted, unsupported, malformed, recovered, and incomplete states. Analysis-package tests exercise existing float setters and precise source accessors.

The focused production batch passed in `job_0dc229141886`. After adding editor recovery and setter controls, `job_9e6ed60d9a97` passed the complete analysis, service, and lower packages plus the production version regression selection. No platform metadata, canonical schema, native action declaration, compiler profile, or schema fingerprint changes are included.

**This corrects generic source-version decisions only. T07's per-platform contract remains UNVERIFIED:** the schema still exposes generic float min/max fields and lacks independently evidenced platform-specific version facts. These tests do not justify adding a formal T07 pass or asserting Apple runtime availability for an operating system.

Exact verification receipt: clean source `d06b885f6a89469b53ed1cbdadd7dc486441616a` passed 60 top-level analysis/service/lower tests and all four production version tests in `job_c0d30c47bfa4`; the worktree was clean before and after. [Verification, raw outputs, and the causal baseline failure](evidence/p15-version-d06b885/receipt.json) retain the measured command results and hashes. This receipt does not change formal T07 status.

### T03 exact committed execution

Clean source `fd63e4f1ffd3b87a632392163a784f02be11044a` passed `TestTypeNumberStringProductionEvidence` in `job_84ee7fa5b43b` (producer 3.710 seconds, package 5.564 seconds). The actual producer executes both semantic and decoded native-wire tests, rejects missing and substituted tier evidence, and emits two records for T03 alone. The report retains `closure_complete=false`. Receipt and verified JSONL are in `evidence/p15-types-fd63e4f/`. The first clean-run receipt collector failed while treating a chunked Go output event as complete JSON; this documented rerun joins the test output correctly and exits zero. Neither stage makes an Apple runtime or numeric precision claim.

### T04 final working source validation

`job_f5068ddc9e7c` passed the final source before commit: 21 root production families, all 61 internal types/analysis/service/lower top-level tests, and the T04 formal unit/service producer. The root selection completed in 6.690 seconds, internal packages in 1.261-1.669 seconds each, and formal producer package in 4.575 seconds. No test failed or skipped in these selected commands. The exact clean source receipt is separate.

### T04 exact committed execution

Clean source `07f5e294e6cdfd7c31587ac9ed36a5a99ab4ffe4` passed its actual unit/service production wrapper in `job_51538dbb53f6` (5.720 seconds; package 7.314 seconds). The tree was clean before and after. Two distinct producer records satisfy T04 alone; missing and cross-tier evidence substitutions fail, and `closure_complete=false` remains. The raw JSONL digest is `0705399b6c1775e82000b5ddf8384da38c18c3c4b415ea51517c80fabef4a3c8`. The source-bound receipt, raw-log digest and normalized committed log are in `evidence/p15-types-07f5e29/`. No Apple runtime or complete catalog guarantee is inferred.

### T04 follow-up: known union assignment order

An independent read-only review found an additional valid case missing from the initial bounded producer: identical known `Text | Number` function contracts. Actual `job_f90c227ba74a` reproduced rejection with `E_RETURN_TYPE` even though source and target were the same union. The authored wrapper now follows the existing structural relation's ordering: every source alternative must match the destination, and different source alternatives may select different target alternatives. Known function identity and annotated If union results are positive controls; narrowing a mixed union to Text and promoting an Unknown member remain rejected in compiler and Service. The prior 07f5e29 receipt is retained for its original scope, not treated as proof for this subsequently added case.

## T04 union follow-up: exact source execution

The source at `44ff2199779f80126c9add57ef9d4b87ea623d22` was clean before and after the dedicated formal T04 execution (`job_67e084d9ea7a`). The production and service producers passed with all required tiers; the formal wrapper took 2.520 seconds (3.691 seconds for its package). The receipt and complete JSONL are in `evidence/p15-types-44ff219/`. The retained causal RED rejected an identical `Text | Number` return contract before the actual-union-first correction. Known union forwarding and annotated branch results now pass; narrowing and opaque union members remain rejected. This receipt covers that source and these local scopes only.

## Declared custom-action result evidence

The unchanged C01 nested custom-action result test exposed a concrete loss of an explicitly declared `Text` return type: action registration stored its display name but omitted the resolved semantic type. Registration now keeps the existing resolved type object. Missing return declarations remain opaque at the analyzer boundary; explicit `Unknown` and `AnyContent` do not acquire a concrete authored guarantee, and consumed `Void` remains invalid.

Expanded T04 production and service controls cover declared Text, Number, union, List and Map results, exact consumed output UUIDs, and unknown/no-declaration/AnyContent/Void negatives. The union control also exposed a nil dereference in the existing lowerer result-identity decision. That decision now reads the already analyzed call result first and uses a nil-safe existing annotation fallback for lower-only callers. It does not parse a second type name, change custom wire parameters, or promise that the synthetic native identifiers are installed or satisfy their declared runtime contract.

Exact clean source execution at `01182577c85bc4d94565af873e114839be62a9cd` passed in `job_b0cf0115e4ad`: formal T04 unit/service (5.720s test, 7.092s package), all nine C01 production families (5.785s package), and both analysis/lower packages (48 top-level tests, no failures or skips). Source cleanliness was checked before and after. `evidence/p15-types-0118257/` contains full JSONL and source-bound receipts, including hashes of the original missing-return RED and subsequent real union-lowering panic.

## Date comparison duration evidence

The unchanged `tests/date-comparisons.cherri` fixture failed after T04 at the migrated direct `CurrentDate > qty(3, "hr")` expression. The native backend already routes this scalar constructor to `WFDuration`; its ordinary expression type remains AnyContent because a quantity parameter is not a general Content Graph value. The analyzer now recognizes only a direct, unshadowed builtin constructor at a Date greater-than/less-than comparison, with a finite Number literal and a literal unit present in the existing canonical `dateUnit` enum. It does not grant an authored type guarantee to arbitrary content, bound quantities, runtime units or other unit families.

The T04 production/service producers execute the exact boundary. Both Legacy and V2 emitted graphs are checked for the current-date input, balanced group, duration serialization tag, string magnitude, exact unit and condition code. Five independent graph corruptions per frontend/operator are rejected; removing the canonical unit closes the exception. The complete unchanged Date fixture also passes both actual CLI paths and the existing normalized differential comparator. This is local serialization/parity evidence, not native Date arithmetic execution.

Independent review identified an adjacent existing negative-integer problem: unary minus materializes an integer through Math, after which the qty constructor stringifies the output reference. Unary quantities remain outside this stage until that separate scalar-folding repair is executed. No native serializer or numeric emitter is modified in this Date stage.

Exact clean Date source `8f42d0c261d413af4ffe348605b24c7eb958bdc5` passed `job_e03549775b12`: the dedicated formal T04 unit/service execution (3.650s test / 5.140s package) and complete unchanged Date fixture actual CLI differential check (2.410s test / 2.662s package). Both pre/post source-clean checks passed. Full JSONL, original causal RED hash and source-bound records are retained under `evidence/p15-types-8f42d0c/`. The full 47-fixture gate is a separate integration check and is not claimed by this two-command receipt.

## Unary integer scalar follow-up

The subsequent causal native-output test reproduced the negative-duration bug: V2 emitted an unnecessary Math action and serialized its attachment debug representation as the duration magnitude; the Legacy scalar control passed. The cause was that integer literals had been upgraded to exact int64/uint64 values while unary folding still handled only float64. Unary minus now folds both integer categories exactly, handles MinInt64 and its inverse without signed overflow, and fails closed for a negative integral literal beyond the signed 64-bit range, including a unary-plus wrapper. Decimal behavior and the existing runtime Math path remain intact. Native numeric literals retain numeric wire values; this change does not convert them to text or modify the shared serializer.

The direct Date-duration boundary now permits these proven scalar signs. Production tests check ordinary integers, decimal control, minimum signed value, its positive unsigned inverse, the unchanged maximum unsigned control, exact producer UUIDs, the retained runtime Math path, negative duration wire in both frontends, and four overflow rejections with no output bytes. Bound quantities, runtime magnitudes and dynamic units still do not acquire this Date field guarantee. No Apple execution or arbitrary-precision arithmetic claim follows from these emitted-value checks.
