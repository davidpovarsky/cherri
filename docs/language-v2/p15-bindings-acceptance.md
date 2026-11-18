# P15 V02 binding scope and flow acceptance

This component supplies production evidence for the original V02 unit contract: scope, shadowing, immutable reassignment and read-before-initialization. It does not supply Apple runtime evidence, V01 snapshot execution, native optional-predicate execution, or release closure. The integrated 179-ID snapshot is generated separately by the root agent from its frozen source.

## Causal defects and repair

The initial production probes reproduced unsafe accepted reads after only one branch initialized a variable, after a possibly empty loop, after one menu case, and after a conditionally skipped logical RHS. Compound assignments also read an uninitialized value without an error. Conversely, valid optional values checked against `none` remained unusable as their concrete type inside the checked path. The service resolved a shadow declaration's initializer to the new local rather than the outer declaration; it also failed to resolve an earlier function parameter used in a later parameter default.

Native serialization exposed a separate lexical defect: after a local `let` inside If/Else, the final outer read referenced the inner output UUID. Mutable shadows reused the outer Shortcuts variable name. These are observed graph defects, not claims about an unobserved device run.

The analyzer now preserves each symbol's declared assignment type separately from its path-local read type. A value is definitely initialized after a branch only when every continuing path initializes it. Loop bodies are treated conservatively as possibly unexecuted, including known positive literal counts. Short-circuit RHS writes cannot establish unconditional initialization. A later loop iteration cannot reuse a pre-loop optional refinement when the body writes that binding. A local shadow does not invalidate the outer binding's facts.

Optional refinement is limited to an initialized optional symbol checked against `none`, including reversed operands, negation, the corresponding Else path, and logical RHS conditions. Truthiness, JSON null, unknown content and arbitrary external values do not establish a cast. Reassignment clears or replaces the read fact without changing the declaration's assignment contract. Returning branches are excluded from the continuing-path merge.

The lowerer restores only declarations introduced by a structured body, so real writes to an enclosing variable remain writes to that variable. It uses a distinct native variable name for a shadow. Initializers still use the enclosing environment and original expression objects; no source reconstruction or second serializer is introduced. The service makes a binding visible after its initializer, and makes each parameter visible after its complete type/default expression. Declaration navigation still resolves to the declaration itself.

## Executed contract matrix

| Producer | Directly asserted behavior |
|---|---|
| `TestP15V02BindingFlowProduction` | 26 invalid scenarios: exact error code and byte span, rejected production compilation with no output; 18 valid scenarios compile through the production pipeline. Wrong diagnostic substitutions are rejected. |
| `TestP15V02BindingScopeService` | Six exact definition targets across outer/local/initializer scopes and one exact shadowing warning. |
| `TestP15V02BindingServiceAndCLI` | The same 26 negatives through the shared service and a built public CLI, including real nonzero exit, JSON validity, stable code and UTF-16 range; all 18 positives return valid with exit zero. Correcting the live document clears stale errors. Original AST identity and five per-expression inferred types are checked. |
| `TestP15V02ParameterScopes` | Earlier parameter visible in a later default; a parameter is absent from its own default; exact definition and rename behavior preserve annotations/defaults and compile. |
| `TestP15V02LexicalWire` | 14 If/Else, native compound predicate, Repeat, For, Menu and initializer cases check actual serialized outer references, distinct mutable names and inner uses. Mutating the actual final serialized reference is rejected by the unchanged oracle. An outer assignment remains an outer write. |
| `TestP15V02ImportedScopeIdentity` | Real synthetic native import and both renderers preserve full trees and pre-overlay action semantics. Appended same-name declarations retain the original outer output/variable/opaque-producer identity and get a new local identity. Same-name Bool shadows use Boolean equality, not inherited imported presence. A native variable first defined in a control region retains its original global identity with zero Stage-3 demotions. |
| `TestP15V02RepeatedImportedLoopIdentity` | Repeated original native loop item/index names retain full-tree and pre-overlay parity in both dialects, with zero Stage-3 demotions. |

The Unicode diagnostic fixture is represented by Go escapes for a supplementary-plane symbol and a Hebrew identifier; it is not dependent on PowerShell transporting literal non-ASCII source bytes. The assertion checks actual runtime UTF-8 offsets and UTF-16 columns.

## Import identity boundary

Import metadata pins original native output UUIDs and variable names. A whole-document or name-only exception would incorrectly exempt a newly authored shadow, so the lowerer tracks whether the original identity has actually been materialized, including opaque raw producers. A later declaration with the same spelling follows lexical rules. Scanning advances incrementally through emitted actions.

The metadata does not provide immutable source-node identities. The first definition of a pinned symbol whose native identity has not yet been emitted is treated as the original definition. Arbitrarily inserting or moving a same-name declaration before that first original definition is not proved by this component. The synthetic appended-shadow, opaque-producer, original-control and repeated-loop cases above are the tested boundary. No private-corpus thresholds or fidelity checks are relaxed.

## Evidence and integration

The development RED logs are retained in the owned checkout under `artifacts/p15-bindings/`. They show the rejected or wrongly accepted production cases before each repair. The final implementation is followed by a separate receipt commit containing exact-source commands, process exit codes, source cleanliness, source hashes and raw test events. Formal V02 evidence is emitted only by `TestBindingScopeProductionEvidence`, which executes all seven named production tests and records the unchanged unit tier.

Other contracts remain independent: argument source-effect ordering (C01), native value snapshot behavior (V01), optional predicate execution on Apple hardware, private-corpus compatibility of the integrated candidate, and final release acceptance are not promoted by V02.
