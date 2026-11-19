# P15 binding and typed-default probes

These public fixtures exercise V01, V03, and T01 through the production compiler.
Each immutable requirement still requires unit, native structure, and iOS
execution evidence. Compilation, model output, and unsigned plist inspection
alone do not satisfy the iOS tier.

## Binding capture

Compile and run `CherriBindingCapturePOC.cherri` on iOS. The final clipboard
payload must contain these exact values:

```text
CHERRI_BINDING_CAPTURE_OK
NUM_CAPTURED=1
NUM_AGAIN=1
CLIP_CAPTURED=clip_initial
CLIP_AGAIN=clip_initial
```

The mutable number is changed from 1 to 2 after the immutable capture.
The clipboard is changed from clip_initial to clip_overwritten after its
immutable capture. The second use goes through an immutable alias; it must use
the same captured producer. Clipboard is the supported case-sensitive system
identifier.

The local production tests inspect the serialized Number, Set Variable, Get
Variable, Text, and Set Clipboard graph. They reject missing or repeated
captures, captures after mutation, source substitution, a substituted producer
UUID, and deferred reads. Direct native references are also covered by the
existing independent reference-kind roundtrip controls.

## Typed defaults

Compile and run `CherriTypedDefaultsPOC.cherri` on iOS. Its self-call dispatch
requires that the imported Shortcut name remain CherriTypedDefaultsPOC. The
final clipboard payload must contain these exact values:

```text
CHERRI_TYPED_DEFAULTS_OK
DEFAULTS_EXPLICIT=0:0::0:0
DEFAULTS_OMITTED=1:9:def:1:1
DEFAULTS_MIXED=0:9:def:0:1
```

The five fields are Bool, Number, Text, List item count, and Map key count.
Explicit false, zero, empty text, empty List, and empty Map are tested
independently of omission. The mixed call supplies only false and an empty List.
Each default branch must be selected by its explicit has_arg presence marker,
not by whether the supplied value is truthy or nonempty.

The local tests prove scalar wire values, argument-field presence, one presence
guard per parameter, the default producers within the omitted branches, and
actual CLI/mobile compilation. The current function-envelope serializer uses
the historical empty Array/Dictionary wrappers: the Array wrapper omits Value,
and the Dictionary wrapper contains an empty Value map. Authored typed literals
use a more explicit empty-container representation. Local characterization of
those wrappers does not prove Apple's runtime interpretation. The two actual
zero counts above remain mandatory runtime assertions.

## Evidence boundary

No V01, V03, or T01 formal pass is created by these local tests. A complete
record needs the exact source SHA, compiled artifact hashes, actual native
callback or clipboard bytes, and successful verification against the expected
values. Record failures and unavailable runtime tiers explicitly.

## Signed Simulator QA registration

The existing Simulator QA workflow offers qa-binding-defaults, which executes
the fixed qa-binding-capture and qa-typed-defaults selections serially with
separate artifacts. Both target names remain exactly CherriBindingCapturePOC
and CherriTypedDefaultsPOC, including the typed-default function self-calls.

The reviewed adapters in tools/shortcuts-simulator-qa/fixtures/p15-binding-defaults
contain the original bodies byte-for-byte. Their added observer reads the
actual final Clipboard after Set Clipboard, encodes its UTF-8 content as
Base64 with no line breaks, caps that witness at 512 characters, and returns
it with the source-bound callback and NDJSON trace. The expected decoded
bytes are the exact payloads above; no trailing newline is expected.
The existing zero counts for explicit empty List and Map remain mandatory.

Phase0 verifies the original source hashes, adapter bytes, actual observer
edges, bounded output and original-name self-calls before signing. Local
checks establish these contracts only; the signed runtime result still has
to be observed on the exact dispatched source. Neither failed callbacks nor
missing trace can be accepted as a pass.
