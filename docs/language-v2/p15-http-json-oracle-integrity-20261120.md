# HTTP JSON oracle integrity — 2026-11-20

## Verified result and scope

Clean source `3936ef41aa501b1dc5f6e201a5a1987888d0b7fd` passed **75 named tests**, with no failure or skip, in **24.375 seconds**. This component changes two Go test files. It does not change production compiler code, acceptance contracts, a consumer tuple, or an Apple runtime fixture.

Five false-acceptance cases were independently reproduced before their repairs: ignored property and coercion metadata, duplicated producer identity, a consumer aliasing the producer UUID, and a producer moved after its HTTP consumer. The final clean run rejects all five while retaining the healthy compiled-wire and Go loopback echo assertions.

The Go echo request is sent by the test process. This is not an observation of Shortcuts sending HTTP, a proof of general dynamic Dictionary transport, or compiler support for JSON null.

## Original request and valid present-value counterpart

The old B1/B2 positive contained `"empty_val": nil` and did not assert that field. Current compiler semantics reject this unproved Dictionary absence transport. The original request literal is retained verbatim as `p10OriginalJSONLiteralSource`. Its **365 immutable bytes** match the original Git literal at `89d0a327259bb97150c5dd410165e9097ff39ae9`, SHA-256 `1f22bb15f3ba1fe956b5f35d81f1c843329521466faf966656f3776a6aedff02`.

Both the original `nil` form and a separately identified `none` form now require the exact analyzer and editor-service target-shape rejection, with code, span and context corruption controls. The actual public compiler must return an error and zero output bytes. The guard is not weakened, and no omitted field, empty Text or plist null is substituted for absence.

The separate healthy source replaces that one field with an explicitly present empty Text and adds a zero control. Its expected complete JSON body includes the dynamic model Text, false, zero, empty Text, nested Boolean arrays and a two-dimensional numeric matrix. The decoded native field tree and the actual Go HTTP echo body must both match the complete expected typed structure. Existing request-header and API boundary checks remain active.

## Strict reference and producer checks

The decoder accepts the supported typed field forms and rejects unknown item kinds, missing values, malformed Boolean/Number fields, invalid wrappers, duplicate keys, unresolved references and unsupported interpolation shapes.

The reference scope is explicit:

| Reference kind | Required identity | Additional permitted metadata |
| --- | --- | --- |
| ActionOutput | Nonempty string `OutputUUID` | Optional string `OutputName` |
| Variable | Nonempty string `VariableName` | None |

`Type` must identify the matching kind. Unknown fields, property/coercion Aggrandizements, fields from another reference kind and invalid metadata types are rejected. The test decoder does not simulate those transformations. Separately labeled synthetic controls verify that a bound empty Text, false and zero remain present values, and that missing or nil bindings fail; those controls are not compiler or Apple observations.

Producer bindings come from the actual compiled action list. They count actual model producers, validate the producer's exact Text payload and metadata, and reject duplicated serialized UUID strings across the actual actions. The fixed source must contain one JSON HTTP consumer, and its model producer must precede it. Missing, duplicated or wrong-mode consumers fail. A canonical parameterless `Nothing` action may omit its parameter envelope; other absent or malformed parameter envelopes remain rejected.

## Retained execution sequence

| Source or stage | Named pass | Named fail | Result and qualification |
| --- | ---: | ---: | --- |
| Original clean `89d0a327` | 0 | 2 | Original nil-positive leaf and parent fail |
| Dirty draft on `89d0a327` | 28 | 0 | Exploratory run, explicitly not clean-source acceptance |
| Clean `ab791d18` | 28 | 0 | Initial characterization and limited wire controls |
| Clean `79cffde0` | 26 | 5 | Two transformation failures; new producer parent stops on omitted Nothing parameters |
| Clean `49aa43be` | 32 | 7 | Four actual false-acceptance leaves plus their parents |
| Clean `2f14d418` | 71 | 0 | First strict repair; retained without claiming untested ordering |
| Clean `ff6a75e6` | 16 | 2 | Reordered producer leaf and parent fail |
| Clean `3936ef41` | **75** | **0** | Final complete selected replay |

The five-line `49aa43be` harness correction only accepted the already established canonical omitted parameter envelope on `Nothing`. It did not change producer counting or reference decoding. This let the actual duplicate controls run, rather than mislabeling the earlier parent failure as a demonstrated duplicate-identity bug.

The ordering mutation moves the real producer action immediately after the real HTTP consumer. It preserves its fields, UUIDs and the terminal action; the original global map still supplied the expected value, so the missing availability check was actually reproduced.

All original streams, receipts, submitted scripts, intermediate patches and the failed template-copy hash guard remain retained. That guard compared a Windows convenience text copy containing 20 CRLF sequences against the immutable LF literal and failed before a commit or test. The convenience copy is preserved unchanged and explicitly classified as derived. The independently extracted original Git literal matches the new const byte for byte without normalization.

## Exact final command and bytes

```text
go test -count=1 -json -timeout=4m -run '^(TestP10_B1_B2_JsonRequest_DynamicAndLiteralContract_And_LocalEchoServer|TestP10JSONOriginalNilRequestFailsPrecisely|TestP10JSONProducerBindingIntegrity|TestP10JSONPlainReferenceShape)$' .
```

The process exited 0 with no timeout. Source was clean before and after execution. Final original stdout is **80,556 bytes**, SHA-256 `aa713b26bcb6e2eb391498268ca5318751ccdc3c381194cb16fdbebb4b02ab54`; stderr is empty. Free disk space was 44,407,148,544 bytes before and 44,301,447,168 bytes after. The four-minute Go deadline was unchanged.

Independent intake strictly parsed every original JSON event, checked the actual process identity, digest, time interval and unique run/terminal pairs, and enforced observed ancestor ordering. It verified the eight retained execution stages, all five previously failing names in the final pass set, **128 current immutable source blobs**, **eight historical test blobs**, the exact original request literal and the unchanged bytes of the three acceptance contracts.

The derived audit is `originals/derived-final-intake/audit.json`, SHA-256 `28aa24aab473ad495e864647d28a638175af533dcd28f7da055a532630badf5e`. It does not replace the original process files. The artifact manifest binds each retained file to its original location, size and hash.

## Source and proof sequence

The implementation ancestry from the original clean baseline is:

```text
89d0a327259bb97150c5dd410165e9097ff39ae9
  → ab791d1848b0c461908f4d4020ffd37f27c5e024
  → 79cffde04145685710da2a9b8687e5953d7e587c
  → 49aa43beb1a5b77a84ddae58584205e70ef89760
  → 2f14d4186d84679da356562327a6132610396852
  → ff6a75e60f41eb4e64be55104cfb10e785b3b5cf
  → 3936ef41aa501b1dc5f6e201a5a1987888d0b7fd
```

The final source was committed before its clean replay. This evidence and report are committed separately afterward. No original failed run is rewritten as a success.

## Remaining acceptance

The local oracle repair is complete. General dynamic Dictionary request transport, compiler JSON-null semantics, File-body runtime behavior, iPad/Apple HTTP execution and final formal179 acceptance remain separate requirements. The JSON Asana tasks remain open, with this bounded verified stage appended:

- `1219800356179243` — dynamic JSON Dictionary proof.
- `1219800511200890` — HTTP JSON/File regression and iPad acceptance.
- `1219800392994574` — upstream interpolation and JSON end-to-end scope.

### Proof tooling correction

The first proof-script submission had a Python syntax error before execution because JavaScript replacement interpreted a dollar-apostrophe sequence in the displayed command. Its submitted script and original failure response are retained separately. The source remained clean at the tested commit, no test ran in that failed sealing attempt, and the corrected structured payload changes no original evidence.
