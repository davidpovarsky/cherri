# Literal Number input evidence

## Corrected contract

The canonical `prompt` action keeps its general output type `Unknown`.
The existing variant-result analysis now establishes `Number` only for the
builtin, non-custom `is.workflow.actions.ask` descriptor whose `inputType`
parameter binds `WFInputType`, belongs to the canonical `inputType` enum,
and contains the native `Number` value. The argument must resolve to that
literal value through the existing literal/enum analysis.

An omitted mode, a runtime value (including a bound string), another mode,
an opaque result, or an altered descriptor supplies no such proof. This does
not relax authored Unknown/AnyContent assignment or comparison rules. It
does not insert a conversion action, change a native serializer, or alter
the static action catalog/fingerprint.

## Independent source and actual local controls

The checked-in macOS 27 Apple metadata identifies `WFAskForInputAction`,
a non-multiple output containing `NSDecimalNumber`, the native enum mapping
`number -> Number` at `WFInputType`, and the dedicated number-default key
`WFAskActionDefaultAnswerNumber`. The regression test reads this metadata
independently of the inference branch.

The source-level tests execute the production analyzer, compiler and language
service. Six literal/enum forms must infer Number and produce an ordered
conditional consuming the actual Ask UUID. Four graph mutations per form
(mode, producer identity, condition and comparison value) must be rejected by
the independent output oracle. Seven unproved modes/results remain Unknown
and fail the ordered comparison in both compilation and the service. Six
descriptor mutations cannot manufacture Number. Service edits must remove
stale Number evidence.

A six-action synthetic native Shortcut independently reproduces the import
regression. Before the change its If/Else/End region incurred three Stage-3
`V2_LINE_COMPILE_ERROR` demotions. After the change both dialects meet strict
full-tree parity and every natural action also passes pre-overlay semantic
comparison, with zero demotions.

## Observed limits

This is local semantic and serialized-output evidence. It adds no Apple
runtime, input UI, cancellation, or non-number-mode guarantee.

A separately isolated host-only input reproduced the same three demotions in
a fresh process before the change. A narrow repeat after the change reported
449 total actions, 416 natural actions, 33 fallback actions, zero invalid
actions and zero demotions (previously 413/36/0/3). Original contents, names,
generated private source and raw logs remain on the host. This single-input
result does not establish a new complete-corpus total or explain every
difference in the earlier aggregate natural-action count.
