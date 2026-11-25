# Function Dictionary array observation

This is a fixed, synthetic WorkflowKit observation. It does not import, sign,
run, or modify a Shortcut and does not launch a simulator. No result grants
function, iOS, or 179-requirement acceptance.

The original b82 function probe remains unchanged: unsigned SHA-256
`54889840e546acc60ffe05c8399e1d4bd54f0dc846ec3fd520b4fffc0aa6c7d0`,
379 actions. Its actual iOS import log reported three NSArray-class warnings
followed by a serialization exception and Shortcuts SIGABRT. Preserved Apple
authored originals also contain immediate top-level Dictionary array wrappers.
Consequently, a direct array constructor accepting an unwrapped NSArray cannot
alone establish the correct Dictionary action boundary.

The seven predeclared inputs distinguish missing empty Value, explicit empty
Value, empty plain NSArray, one empty Text item, and one synthetic ActionOutput
item. Wrapped and plain forms are compared without rewriting any observation.
The synthetic attachment does not establish resolution of a real producer UUID.

Each input first uses the existing `probe_single_action.js` action entry point:
`WFBundledActionProvider` selects exactly `is.workflow.actions.dictionary`,
`copyWithSerializedParameters` receives UUID plus the complete WFItems wrapper,
and `parameterStateForKey("WFItems")` reads the resulting parameter. Advertised
serialization and nested value accessors are observed, without action execution.
Only then are the direct array, Dictionary row, field-value, and parameter-state
constructors observed. Their unavailable selectors or failures remain explicit.
Stage and completed-surface files survive a later native exception.

Run only from a clean, exact Git checkout using the existing pinned prelude:
commit `ee52c32937053268e4a035eba4b372fe77ec6411`, SHA-256
`d6fbcaad5104562b04004b39ca098486449399c15c83176c49b2ac28f47a3461`.
The owner uses a fresh output directory, 90 seconds overall, at most 8 seconds
per child, and the existing capture helper's owned-process-group cleanup. It
retains raw streams, process journals, physical/Git source witnesses, original
prelude-combined bytes, and before/after source equality. `OBSERVED` means the
fixed observation protocol completed; it does not mean a serialization form is
correct or that a Shortcut ran. A process failure keeps partial originals.

The six Python controls validate only this fixed controller inventory and
create-only/source-closure boundaries. They do not simulate WorkflowKit.
