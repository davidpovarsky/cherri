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

## Remaining T01–T08 obligations

| ID | Current evidence boundary |
| --- | --- |
| T01 | Explicit false/zero/empty/default presence and signed iOS behavior require dedicated evidence; the old base64 smoke is not formal proof. |
| T02 | Dedicated production unit producer described above; clean exact-commit receipt is recorded separately after commit. |
| T03 | Exact semantic Number to number-as-text wire producer is still being selected and independently verified. |
| T04 | Distinct type names alone do not establish Unknown/AnyContent/Void behavior; production diagnostics and service evidence remain required. |
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
