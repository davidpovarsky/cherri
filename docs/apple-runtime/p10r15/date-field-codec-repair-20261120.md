# Date arithmetic field codec repair

Implementation `e259c829fa0345702e0989a25b5bbac1296d1fa6` routes only `is.workflow.actions.gettimebetweendates` fields `WFTimeUntilFromDate` and `WFInput` through the existing text-reference encoder. The narrow hook is shared by Legacy and V2, after explicit Reference handling. No action is added and no public parameter type, default or runtime expectation is changed.

## Causal evidence

Actual Apple run44696174000 showed absent native Date state for the original bare references and preserved exact UUID/name tokens for the wrapped equivalents over three native-object hops. Mixed-slot, literal, missing-parameter and unchanged format controls isolate that boundary. This was serialization observation, not action execution.

Clean tests-only source0ba11bf reproduced the rejected bare wrappers in both dialects for immutable Date outputs and mutable variables. Seven corrupted reference controls were rejected; the existing format control stayed valid. The corrected implementation then passed37 named test outcomes, zero failures and zero skips, in10.328seconds on a clean checkout. The exact command, all source hashes and raw RED/GREEN log hashes are in the adjacent JSON.

The final controls also preserve omitted arguments, supported literal values, the existing V2 first-slot reference restriction, unrelated native inputs and strict original P12_PAR_09 dual-dialect import roundtrip with zero Stage3 demotion. Earlier test-authoring and command-path mistakes remain explicitly recorded in the receipts; they are not described as production failures.

## Remaining gate

The unchanged original27-field Date diagnostic must now run on the exact corrected component source. A local compiler test and native parameter-state observation do not prove corrected runtime behavior, label Date retention or formal acceptance. No consumer pin is updated by this component.
