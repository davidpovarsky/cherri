# LANG002 canonical scalar-reference consumer

The actual S2 Linux and macOS jobs in run44452839062 both failed the old ValidEnum assertion, which expected the raw string PRODUCTION in Quick Look WFInput. A clean Windows replay of exact source 7b0100a2a9ce5991ca8ffe82c805278e5aa7754e reproduced that failure. InvalidEnum passed. The two complete hosted job logs remain archived in ../ci-7b0100a-44452839062.

The production behavior follows the canonical declaration in actions/basic.cherri: Quick Look input is variable WFInput, and its schema uses token codec with attachment(output)/attachment(variable) accepted forms. The shared scalar codec materializes an enum string exactly as an ordinary string argument.

## Narrow test-only repair

Source 4b9daa22126e4b57ec8d0b10d04f6bae89382b39 replaced the obsolete literal assertion with a precise consumer oracle: exactly two ordered actions, one Text containing exactly PRODUCTION, followed by Quick Look consuming that preceding producer UUID through the canonical attachment without coercions. The enum graph must equal the ordinary literal graph after normalizing only generated UUIDs. The existing InvalidEnum body is unchanged.

Nine causal mutations of the actual compiled graph must be rejected: wrong text, unbound UUID, literal bypass, wrong variable carrier, unexpected coercion, missing or duplicated producer, wrong ordering, and wrong native consumer. The existing C04 production and negative enum cases are also retained and executed.

The initial candidate exposed a defect in the new test: its parity comparison opened a second canonical backend session while the first still held the global transaction mutex. That bounded test timeout and stack are preserved under initial-candidate-timeout. This is a test authoring defect, not a production compiler or hosted runner failure.

Source 731fe7610e87c8f2a74dcc91c169ce20f595cf84 fixes the session lifetime by snapshotting/closing each independent compilation before the next. The exact clean replay passed all named cases with zero failures/skips, exit0 in26.11seconds, GOMAXPROCS=1 and -p=1. The clean-fixed summary lists every case and binds the raw JSONL digest.

No production compiler, accepted source semantics, native acceptance oracle or immutable mandate was changed. Full combined CI must still verify the integrated source. These local tests do not execute Apple runtime.
