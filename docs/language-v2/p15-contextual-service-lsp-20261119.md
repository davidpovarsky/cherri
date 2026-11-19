# P15 RP38: contextual service and actual LSP protocol

## Implemented behavior

The LSP adapter now converts incoming zero-based UTF-16 coordinates with the shared source map before calling the service's one-based rune API. Hover, definition, rename and diagnostic ranges use that same source map. Initialization advertises UTF-16. A request after two astral characters is verified through real framed bytes, including exact definition and both rename edits.

Completion uses the existing lexical declaration resolver, including function parameters and local blocks. Bindings are visible after their initializer, closed block/function ends are exclusive, and program-level bindings remain available at EOF. The existing completion test now checks both absence before declaration and presence after it.

Named completion and signature help share the existing production lexer-backed call context. Commas or labels inside strings, nested values and multiline calls cannot select an unrelated argument. Incomplete lists, maps and grouped expressions retain their nearest enclosing real call without leaking that call's enum into a nested value. Declared function parameter and return types are retained in signatures.

The adapter maps service completion kinds to the official LSP wire values. It preserves large numeric request IDs, emits explicit null results, returns method/parameter errors, handles shutdown/exit, and propagates write errors and short writes. Framing rejects negative, overflowing, duplicate and incomplete lengths without panic. Input bodies are bounded to 32 MiB and header bytes to 16 KiB; an individual header line is bounded by the buffered reader.

## Executed causal verification

The original implementation failed six new test groups: UTF-16 request behavior, contextual wire kinds/signatures, lexical visibility, JSON-RPC result/error behavior, malformed framing, and output failures. The original multiple-diagnostic UTF-16 output already passed and remains covered.

A read-only peer review identified two further edges. Added tests actually failed for a local/parameter visible immediately after a closing brace, and a missing enclosing signature inside an incomplete list. Those tests pass after the corresponding resolver/context fixes.

The complete LSP and service packages passed 28 top-level tests with no failures or skips. The existing C04 selection passed five top-level enum tests, including service/CLI, contexts, shadowing, functions and provided-registry domains. Evidence is retained outside the repository in the continuation evidence directory under rp38; the final clean-source producer receipt is recorded after this implementation commit.

The independent RP38 producer executes nine explicitly discovered real LSP/service tests for the immutable service+protocol tiers. Its validator controls reject capability-only selection, missing protocol tier, missing discovery assertion, stale source, invented iOS tier, linked-original inheritance and wrong producer. It cannot complete E01/E02/E03/E06, an iOS editor test, or release closure by association.

The native resizeImage height label is checked against its existing AnyContent schema; typed Number/Text parameter labels and signatures are checked separately using actual function declarations. No schema type was changed to fit the tests.

## Reference and limits

Wire completion kinds follow the official Microsoft LSP types:
https://github.com/microsoft/vscode-languageserver-node/blob/main/types/src/main.ts

This is local protocol and shared-service evidence. It makes no claim about Apple UI execution, remote consumers, performance thresholds, or completion of the full 179 requirements. No immutable contract or native parity fixture was edited.
