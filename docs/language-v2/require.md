# Boolean condition and typed `require` guard (P10R15 implementation)

`require(value, ...)` evaluates a value once, requires a source-proved Boolean condition or validates supported native content guards, and returns the original value through a native Get Variable alias. It does not serialize the value to JSON or rebuild it from text.

## API

```cherri
let condition: Bool = require(true, message: "Condition must hold")
let numberValue: Number = require(0, type: "number", nonEmpty: true)
let textValue: Text = require(text("ready"), type: "text", nonEmpty: true)
let dictionary = require({"key": "ready"}, type: "dictionary", nonEmpty: true)
let list: List<Text> = require([""], nonEmpty: true)
let file = require(base64Decode("AQ=="), nonEmpty: true)
```

The Legacy frontend uses positional arguments:

```cherri
const checked = require("ready", "text", true, "Expected nonempty text")
```

| Option | Contract |
| --- | --- |
| `value` | Required native value or producer expression. |
| `type` | Optional literal native category: `"text"`, `"number"`, or `"dictionary"`. |
| `nonEmpty` | Optional literal Boolean. When true, apply the content rule below. |
| `message` | Optional text or text token for the native failure alert; default `"Requirement failed"`. |

With neither a nonempty `type` nor `nonEmpty: true`, the input must be a proved `Bool`. `require(false)` emits the failure alert and Stop This Shortcut; a successful condition returns the original Boolean. Optional or unknown native values, text and numbers do not acquire truthiness. In Legacy, the equivalent explicit call is `require(true, "", false, "Condition must hold")`.

Providing a supported `type` or `nonEmpty: true` selects the existing content guard. Thus `require(false, nonEmpty: true)` checks that false is present; it does not assert that false is true. Unsupported type names and dynamic guard options are rejected. The implementation deliberately exposes only native categories for which it has a concrete witness strategy; `type: "file"`, `"list"`, and `"boolean"` are unsupported. Native File and List values can still pass through a `nonEmpty` guard without conversion.

The V2 analyzer preserves the input's inferred type, including list element types and dictionary/record shape. A successful guard removes an optional wrapper. For unknown native content, a supported type guard refines the result to its checked category. A statically known incompatible type is rejected before emission, including a known List presented as scalar Text. Legacy source uses its existing variable metadata and native runtime checks.

## Native semantics

The Boolean condition path uses the existing native Number representation of Boolean 0/1 and explicit equality to zero. It does not use Has Any Value, re-evaluate a native producer, or convert an arbitrary value to Boolean. Actual source type evidence and a complete native reference identity are required. Both Debug and Release retain the condition and its failure actions.

Type guards first require exactly one content item. Zero or multiple items fail before type-label comparison. The compiler then emits Get Type for the input and for a native witness value of the requested category. It compares these runtime-derived labels rather than assuming an English type label. The witness is used only for validation; the result retains the original input reference. Both inspections use the existing canonical `typeOf` action (`is.workflow.actions.getitemtype`). The independent Apple action metadata identifies it as `WFGetItemTypeAction`, with text output named Type; its identity and input wire key are checked against actual production `typeOf` compilation.

`nonEmpty` has these definitions:

| Source category | Check |
| --- | --- |
| Text | Native Count with Characters. |
| Dictionary or record | Get All Keys, then Count with Items. |
| Known List | Count with Items, including a singleton list containing empty text. |
| Number, Boolean, File, or another known native scalar category | Count with Items. Both zero and false remain present values. |
| Unknown content | Reject zero items; accept multiple items; for a single item, distinguish native text and dictionary with runtime witnesses before checking characters or keys. |

The checks never use Has Any Value as a substitute for Boolean truth. On failure, a native Alert displays the message with no cancel button, followed by Stop This Shortcut. The success path emits a native Get Variable alias to the original value.

A plain Action Output attachment is reused as-is. Mutable variables, system values, and references with aggrandizements are captured once before inspection. Literal inputs use the existing native literal codecs. The final alias has its own output UUID; its attachment chain points back to the original producer or once-only snapshot. Preservation means preserving the native value and producer reference, not reusing the alias action's UUID as the producer UUID.

## Limits and proof status

Apple's native content model can erase a dynamic singleton collection's distinction from its single scalar item. Static source metadata prevents known `[""]` from being checked as empty scalar text. For an unknown singleton value, this implementation cannot promise to recover a container identity absent from native output. The API therefore does not offer a native List identity guard. Apple documents the broader content conversion model in its [Content Graph guide](https://support.apple.com/en-gb/guide/shortcuts/apd4618db957/ios).

The compiler metadata carrying static source type is private. It is not a public action parameter and is never emitted into the shortcut. `require` is a compiler macro and does not alter the frozen Apple action catalog. Its emitted actions are classified as semantic source actions, even though it shares macro dispatch infrastructure with debug helpers. Release builds must retain these guards; integrated profile verification is a separate gate.

Local production-path tests cover both frontends, original producer UUID chains, native File/List/Text/Number/Dictionary passthrough, one producer execution, mutable/system capture, zero/false/empty shapes, unsupported and dynamic options, exact failure message escaping, canonical Count wire keys, and inferred output types. A substitution mutation confirms that the passthrough oracle rejects a changed producer reference. A causal regression test distinguishes singleton empty-text List from empty Text.

These structural and compiler checks do **not** constitute signed Apple runtime proof. The fixture `tests/language-v2/v2-require-runtime.cherri` returns `P15_REQUIRE_OK` only after its supported success scenarios. It is intended for a signed run against an immutable integrated commit, with correlated trace evidence. Failure dialogs, native execution details, supported OS behavior, and physical-device behavior remain open until independently observed. This scoped implementation is not closure of the full release task.

The Boolean-condition implementation and exact executed qualification are recorded in [the 25 November component contract](require-boolean-condition-contract-20261125.md). Its 180 distinct planned passes join two original production runs, with their failed originals preserved; they are not a single-run or signed runtime claim.
