# C02: production argument diagnostics

## Contract and observed defects

The immutable C02 requirement in `tests/language-v2/acceptance-contract.json` is "Duplicate, missing, unknown labels and extra positional arguments," with a unit tier and specific stable diagnostics at parameter ranges.

The previous local runner checked one unknown label. A production regression matrix found additional defects:

- User-function calls could omit required parameters or silently discard duplicate, unknown, and excess arguments. The existing checker examined the types of selected arguments without accounting for every supplied argument.
- A public underscore alias could cause one supplied argument to be reused for two function parameters.
- An excess positional argument to a native action produced an unknown-empty-label diagnostic instead of `E_POSITIONAL_ARGUMENT`.
- An explicitly declared custom-action primary parameter at an index other than zero was marked as parameter zero. This misreported missing and duplicate arguments and rejected valid calls.

Baseline job `job_8efc71829c64` reproduced these failures. The tests execute the production analyzer and actual compiler, using canonical native actions and public source declarations rather than injected action schemas.

## Corrected behavior

The function checker tracks which source arguments the existing envelope-binding order consumes: primary, matching explicit/public parameter name, and remaining positional arguments. It reports every unconsumed argument and rejects reuse. Missing required parameters now produce `E_MISSING_ARGUMENT`; a parameter marked optional or given a default remains omittable. An optional value type alone does not make its argument optional.

Native actions record the actual index of their declared primary parameter. Existing positional compatibility fills the remaining unbound parameters. Unknown labels use `E_UNKNOWN_ARGUMENT`, duplicates use `E_DUPLICATE_ARGUMENT`, and excess positional values use `E_POSITIONAL_ARGUMENT`. Missing arguments identify the missing parameter at the call range, since there is no argument token to underline.

Source that previously supplied invalid user-function arguments now fails before native emission. Valid positional calls within arity, named calls, defaults, explicit optional parameters, and unambiguous public aliases remain accepted.

## Production proof

`p15_call_acceptance_test.go` contains three producers:

| Producer | Assertions |
| --- | --- |
| `TestP15C02ArgumentDiagnosticsProduction` | 24 invalid call scenarios; exact code, severity, parameter identity, byte range and UTF-16 offsets; actual compilation emits no bytes; wrong code, range and severity mutations are rejected |
| `TestP15C02AcceptedBindingsProduction` | 19 accepted call scenarios covering reordered labels, positional compatibility, defaults, optional presence/value distinctions and aliases; actual native custom-action parameters prove that a non-first primary maps to the correct wire key; a wrong-value mutation is rejected; user-function calls retain their native envelope reference |
| `TestP15C02ArgumentServiceAndCLI` | Every invalid scenario passes through the actual CLI `check --json` command and the shared service; nonzero exit status, validity and exact UTF-16 protocol ranges are checked; edits back to valid code clear the errors; corrected CLI input succeeds |

Non-BMP and Hebrew characters are constructed with Go Unicode escapes, so shell encoding cannot silently replace them. The Unicode unknown-label case also places a non-BMP value on the diagnostic line. Protocol ranges are calculated independently in the test from Unicode code units.

The focused production batch passed in `job_e8b2cbddea0a`. The expanded Unicode/alias controls and bounded compatibility tests passed in `job_746ab7d0138f`. The formal producer and existing type-evidence producers passed in `job_be47552cd94f`; the receipt helper is shared so every group uses the same provenance and bounded-scope validation.

## Formal accounting

The C02 runner now executes all three named producers and binds its unit record to their command and the actual source SHA. Its evidence test rejects an unrelated producer, an absent record and the wrong contract. The generated audit still has all 179 requirement rows, and no unexecuted sibling requirement inherits C02's result. The historical single-label smoke observation no longer supplies C02 evidence.

This closes only the tested C02 unit contract when its complete production command passes on the recorded source. C01 effect ordering, C03 variadic migration, native execution of functions, special native escape constructs, and release closure have separate requirements. The contract file and other requirement statuses are unchanged.

## Immutable source receipt

The production-evidence command was rerun on clean source `70cd603e31f547e907c0ba2d9a4978b21e012665` in job `job_6dd2d8845d70`. It exited zero and passed the C02 producer plus the existing T02, T06 and T08 producer checks. The source-bound C02 record satisfies its unit tier; the audit retains independent statuses for all other requirements. See [the receipt and artifact hashes](evidence/p15-calls-70cd603/receipt.json), [the exact formal record](evidence/p15-calls-70cd603/verification.json), [raw Go JSON output](evidence/p15-calls-70cd603/verification.jsonl), and [the causal baseline failures](evidence/p15-calls-70cd603/baseline-red.log).
