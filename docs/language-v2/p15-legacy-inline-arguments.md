# Legacy inline action arguments: producer and evaluation repair

## Reproduced failures

The unchanged combined source 57a72ed05f97158c85d2ed591801b439041caa41 lost value-producing native actions used directly as Legacy arguments. The actual source url(text("FIRST"), text("SECOND"), text("THIRD")) emitted only its URL consumer instead of the three written Text producers and their consumer. Three-deep Text calls and a typed Text -> Get File of Type -> Get Type chain showed the same omission. Equivalent explicit constant bindings produced the complete native graph.

The original scoped-body helper only materialized an Action argument while parsing a scoped body or when the argument itself carried a block. Outside those cases, the native consumer had no emitted producer. Nested action parsing also retained the child's current action context, and nested function parsing advanced to the end of the line instead of consuming only its own close parenthesis. A following TAIL statement disappeared in the actual CLI control.

Expanded tests found a separate evaluation boundary: an earlier mutable variable argument was still a live native Variable reference when a later measure argument assigned it. The graph contained two stores and no preceding Get Variable capture. This failure is retained in candidate-expanded-boundaries-retry.jsonl.

The actual custom-call control exposed another concrete parser cursor defect: a typed action declaration immediately followed by a call could compile successfully with zero actions. collectDefinedAction advanced past the declaration newline before replaceLines, which erased the following call. The original adjacent-line test remains; no blank separator was inserted to hide it.

## Implementation

- General inline actions now use the same structured constant-token producer path already used by scoped bodies. The original parsed action and its arguments are retained, and consumers refer to the exact producer UUID.
- Nested action and function collectors restore their parent context. Functions validate even an empty parameter list and consume exactly their closing parenthesis.
- Before staging a later argument's emitted tokens, the compiler captures earlier mutable references through native Get Variable and constant output references. Direct references and interpolated Text operands retain their values across a later assignment; scoped-body environments retain the private capture definitions.
- Interpolated text uses the existing parser interpolation grammar. Literal text and property/coercion suffixes are retained. Existing immutable producer references are not captured again.
- Generation uses the already selected action definition, falling back to the historical callable lookup only for unresolved legacy action tokens. A causal late-lookup corruption proves that the compiler-owned Get Variable snapshot keeps its native operation.
- Action declarations skip inline whitespace without crossing their terminating newline, preserving the following call. Both immediate next-line declarations and declarations with an explicit parameter body are covered.
- The existing collect test now validates the established empty-input representation: Nothing, Set Variable without WFInput, Get Variable, and the exact Repeat input UUID. Nonempty Number/List checks and the no-accumulator assertion remain. Five corruptions of the empty-input graph must fail the oracle.

No new native serializer, parser, macro framework or registry was introduced.

## Validation and causal controls

Baseline job_81470b77285a executed healthy explicit-binding controls and reproduced seven inline/function/control-region failures. The source baseline JSONL SHA256 is acf0ef6f8180e5f22c242f47d521f7001f8771e07ae0ac1ccf68e5b39fa91843. Its unchanged collect control failed only the obsolete zero-size List assertion; the paginate public catalog denominator passed.

Working candidate job_0025da9becaa passed 385 named tests in 40.078 seconds, with zero failures and zero skips. The exact command covers:

- new actual Legacy compiler/CLI producer, order, cursor and rejection checks;
- direct and interpolated mutable input capture, including a nested scoped body;
- existing Legacy measure, C01, C02, C03, registry-lifetime, menus and vCard suites;
- the corrected empty collect oracle and the unchanged paginate metadata boundary;
- existing public Fork action round trips.

The working log SHA256 is d51025c23e9632c1337cdc34669681fccbc476365be4fb63e57a448409263660. A clean immutable-source execution receipt is recorded separately after this implementation commit.

Native graph oracles reject missing/repeated/reordered producers, dangling/foreign/forward UUID references, omitted snapshots and snapshots moved after the assignment. Actual CLI negative controls reject void argument expressions, missing required parent arguments, literal-only dynamic arguments, invalid mixed contact/app inputs, extra arguments to a zero-parameter function and duplicate canonical callable declarations. Rejected sources must leave no output file.

The review concern about an authored typed action shadowing a canonical Void callable was tested and disproved as an accepted-source route: both show and stopStopwatch declarations failed earlier with Duplicate declaration. Those original observations remain in the logs. A distinct custom callable supplies the positive source control; no duplicate-name policy was weakened. An early calculate test was also corrected because its existing Legacy enum parameter was not declared literal-only; makeVCard provides the actual literal-only control.

A failed expanded attempt, job_f76c647f26b4, executed zero tests because the Windows linker reported insufficient disk space. It is retained as an infrastructure failure, separately from the actual source failures and later passing execution.

## Evidence scope

This is local compiler, actual CLI and native-structure evidence. It adds no formal acceptance ID and makes no Apple import, execution or signing claim. The production-path checks cover the source forms described above; they are not a claim that every Legacy aggregate evaluation shape has been proved. No private-corpus or external signing request was made by this component. The 47 original public root fixtures are unchanged, with exact hashes checked again around the clean source run.

The existing C03 migration evidence retains its historical inline omission. This follow-up repairs that omission independently; it does not rewrite the original baseline or relax strict authored type/literal boundaries.

## Clean immutable-source receipt

Source 1244f448408bc7680a5160e2445c7dbc4914a6ab was clean before and after job_2a53c4717f48. The same bounded command passed 385 named tests in 39.203 seconds, with zero failures and zero skips. All 47 original public root fixture hashes matched the earlier preserved inventory and stayed unchanged.

The full clean JSONL SHA256 is b852d5e1a550daaf43e19d11f9c55aa88ea8d47dba1a5405048bdf735f831a55. The files in evidence/p15-legacy-inline-1244f44 retain the actual baseline, later causal failures, disproved review hypotheses, infrastructure-only failure, working execution and exact-source receipt. The audit hashes all eleven payloads and the ten files in the implementation commit. No acceptance ID or Apple runtime status is promoted.

The independent read-only review found no concrete defect in the parent context restoration, function cursor boundary, staged snapshot order, preserved interpolation suffixes or definition-first generation. That review did not run tests and is recorded separately from the actual execution proof.
