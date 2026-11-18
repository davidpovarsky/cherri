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

## Remaining T01–T08 obligations

| ID | Current evidence boundary |
| --- | --- |
| T01 | Explicit false/zero/empty/default presence and signed iOS behavior require dedicated evidence; the old base64 smoke is not formal proof. |
| T02 | Dedicated production unit producer described above; clean exact-commit receipt is recorded separately after commit. |
| T03 | Exact semantic Number to number-as-text wire producer is still being selected and independently verified. |
| T04 | Distinct type names alone do not establish Unknown/AnyContent/Void behavior; production diagnostics and service evidence remain required. |
| T05 | Optional type construction alone does not prove omission versus explicit optional value and JSON null in native serialization. |
| T06 | A structural record constructor does not prove rejection or honest Unknown/JsonValue treatment of external content. |
| T07 | The production analyzer currently parses and compares target versions as float64. The schema helper compares components but is not used there. Existing action min/max metadata is also float64 and lacks independent per-platform version facets. No T07 pass is claimed. |
| T08 | Production analysis currently returns fixed ActionSchema output types. Separate aliases do not establish literal-mode-dependent single/multiple cardinality. No T08 pass is claimed without a concrete verified producer. |

No native runtime, UI, platform availability or release closure is inferred from this local type work.
