# P15 C01 source-order call acceptance

This component implements and proves the original C01 contract: named arguments supplied outside parameter order evaluate in source order, with each argument's effects evaluated once. Its required tiers are `unit+native`; here the native tier inspects actual serialized Apple action structure, producer UUIDs, value references and conditional default branches. Apple function execution, function bridge runtime, system-value capture contracts and release closure remain separate requirements.

## Reproduced defects

Production probes showed user functions emitted named argument producers in parameter order. The analyzer used that same incorrect order: it accepted an earlier uninitialized read when a later argument initialized the variable, and rejected the inverse valid source order. Positional and ordinary native-action controls distinguished the failing path.

Omitted function defaults were lowered once in the caller and again in the dispatcher's omitted-argument branch. The caller also evaluated parameter-dependent defaults in the wrong environment. Function and custom-action arguments inherited the enclosing statement's unused-result context, causing a first nested producer to lose the UUID needed to pass its value. A consumed custom action itself could return a reference to an absent output UUID.

Further causal probes exposed an earlier argument retained as a live native variable reference after a later `measure` argument assigned that variable. Direct and interpolated Text, List and Map argument boundaries now preserve the earlier value before the later write. These are observed compiler and native-structure defects; they are not inferred results from an Apple execution.

## Implementation

Argument identity binding is separate from expression evaluation. The analyzer first resolves parameter contexts, then analyzes the primary argument and following arguments once in written order. Contextual enums, underscore aliases, duplicate/unknown labels and positional compatibility retain their existing checks. The lowerer likewise evaluates supplied values in written order before placing them into parameter-indexed function envelope fields.

The existing dispatcher is the sole owner of omitted function defaults. It evaluates defaults in declaration order under each `has_arg_i == 0` branch, after preceding parameters have been bound. The caller records supplied/omitted flags and supplied values; it does not execute omitted defaults. No function protocol or alternate serializer is introduced.

Consumed values use a value-expression context even if their enclosing call is a statement. A custom action that produces a value receives an output UUID when that value is consumed; unused standalone outputs retain their prior behavior.

For an earlier argument threatened by a later source-visible assignment, the lowerer reuses the existing native Get Variable or typed Text/List/Dictionary producer at the argument boundary. It shares the lexical assignment walker with V02 analysis. Local shadows and unrelated writes do not turn a plain outer variable into an unnecessary capture. Arguments with no later visible assignment retain their existing imported reference wire. Explicit raw/native payloads are not reclassified through guessed dictionary field names.

## Production evidence

| Producer | Direct checks |
|---|---|
| `TestP15C01SourceOrderFlow` | Named and positional write-before-read/read-before-write controls; exact stable diagnostic and byte span; invalid source emits no native output. |
| `TestP15C01SourceOrderNativeStructure` | Reversed named, mixed primary/named, positional, underscore alias, custom non-first-primary and ordinary native calls. Actual producer order, once-only count and parameter-to-UUID references are checked. Swapped, missing, duplicated producers and wrong actual consumer UUIDs are rejected. |
| `TestP15C01DefaultEffectsOnce` | Both omitted, one positional supplied, one named supplied and both supplied forms; one compiled default producer per guarded dispatcher branch and exact presence flags. |
| `TestP15C01DefaultGuardStructure` | Defaults remain in the omitted-argument Else branch inside the dispatcher, in dependency order. Wrong supplied/dispatcher comparisons and moving a default outside its guard are rejected. A later default references the earlier parameter result even when the caller has a same-name local. |
| `TestP15C01ArgumentValueSnapshot` | Function/custom direct and interpolated Text arguments preserve the original value before a later argument's actual assignment. |
| `TestP15C01SnapshotBoundaries` | Ordinary native numeric argument capture, local-shadow/unrelated-write controls, and unchanged unused standalone identity. |
| `TestP15C01NestedCustomResult` | A nested custom action produces exactly one real UUID that the function payload consumes, with its nested argument effects in source order. |
| `TestP15C01AggregateArgumentSnapshots` | List and Map arguments preserve typed native producers and original variable references before the later mutation. |
| `TestP15C01CLIAndService` | Built public CLI and shared service agree on the causal valid/invalid source orders, real JSON ranges/exit codes, and the CLI's unsigned compiled native file has the correct order and envelope references. |

The formal producer executes all nine tests, records separate unit/native-structure evidence, and rejects detached producer, missing record, wrong requirement and missing required-tier mutations. It does not change the original contract or promote other IDs. C02/C04, V02, type checks, short-circuit behavior and synthetic import compatibility are verified separately before integration.

## Evidence boundary

Development RED logs and later correction/compatibility runs are retained under the owned checkout's `artifacts/p15-call-order/`. The final source is followed by a separate immutable receipt with clean-source checks, actual process exits, source hashes and raw Go events. The root agent owns the combined formal snapshot, private 38-input corpus, installed consumers and signed runtime gates.

The assignment capture in this component concerns source-visible writes in later arguments. It is not proof of system-value snapshot semantics, arbitrary native action side effects, function platform runtime or every internal evaluation rule of nested containers; those retain their own contracts. No corpus threshold, imported fidelity check, external gate or release condition is relaxed.
