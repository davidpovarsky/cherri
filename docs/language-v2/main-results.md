# Explicit main results

V2 main code returns a value only through an authored return or an explicit native terminal action. Natural fallthrough has type Void: an unused final action, an empty body, or a partial return path must not expose an incidental native result or inherited ShortcutInput.

## Lowering boundary

The lowerer emits the existing parameter-free `is.workflow.actions.nothing` action after ordinary main fallthrough. It tracks the final raw or canonically resolved action identity, rather than relying on the intermediate action collection, because resolved calls can emit directly to the canonical session. Each statement resets that identity so an earlier terminal action cannot suppress a later clear.

A guaranteed structured return, or an actual terminal Nothing, Output or Stop action, needs no extra clear. A loop is not a guaranteed return because it may execute zero times. A menu is not a guaranteed return because it may be cancelled. Function dispatch keeps its existing child-return behavior; the final main clear is outside the caller branch. Explicit return values, including empty Text, false, zero and None, retain their existing native representation.

A real `native.workflow` import remains exact. Only the flag set by that lowering route suppresses the added main boundary; text or unrelated metadata cannot impersonate it.

## Compatibility metadata

The entry header accepts exactly one literal `outputMode` value, either `"explicit"` or `"legacy"`. Omission means `"explicit"`. The shared syntax resolver is used by analysis, checked lowering and migration, so duplicate declarations, unsupported values and dynamic expressions are rejected before emission. This is compiler metadata and does not create an Apple plist field.

Legacy entry migration writes `outputMode: "legacy"` once into the parsed header. Repeated migration preserves an existing valid explicit or Legacy choice. Comments and literal contents do not count as header metadata. Imported Legacy modules use the separate module conversion context and do not gain a synthetic entry declaration; genuine module metadata remains subject to the existing module validator. Reserved Legacy preprocessor definitions cannot override this policy.

The Legacy parity corpus and comparator remain unchanged. The generated probes whose purpose is Legacy action equivalence declare the compatibility mode explicitly.

## Verification scope

The M02 tests model complete native action graphs, exact UUID edges and return ownership. Their corruption controls cover missing, extra and changed main boundaries and invalid output references. The affected value-flow, function, Optional, macro, frontend and import oracles explicitly account for the real final action; they do not strip actions from compiler output.

Clean implementation execution, retained causal failures and source-bound raw receipts are recorded separately. Native graph success does not establish Apple execution. M02's original iOS requirement, the broader BRG14 scope, the full Go/CI gates and final 179-row acceptance remain independent.
