# Main-output mode graph integration — 2026-11-20

## Verified outcome

Clean source `4adf57a713e7386992aad7cb6dd39c3d8304c328` passed **815 named root tests**, with no failure or skip, in **185.937 seconds**. This is a bounded integration repair of three test files after the M02 main-output behavior change. It changes no production compiler code, immutable acceptance contract, consumer manifest or runtime fixture.

The original root run at `8b8093eebf6544aad649a668c7afef147ff0cc32` remains retained as **98 named passes and 42 failures**. The 41 failure events within this component now pass. The separate `TestP15OptionalNativeDiagnosticPreparedArtifactControls` failure belongs to the root Optional artifact repair and is not credited by this report.

## Why the graph expectations changed

Every value-control function discovery fixture explicitly returns from the authored main body. Its correct complete graph ends after the dispatcher boundary; adding a fallthrough `Nothing` after that boundary is a different expected graph. The new `finishExplicitMainReturn` helper authors this exact end. The existing `finish` helper retains its `Nothing` and remains the expectation for natural main fallthrough. All original discovery cases and corruption controls remain active, including rejection of an appended extra action.

The five T04 declared custom result sources naturally fall through after consuming the custom action output. Their complete graph is the declared producer, its `showresult` consumer, and the main `Nothing`. The oracle now checks all three actions, exact input and output metadata, producer identity, the consumer's precise ActionOutput UUID reference, and an empty terminal `Nothing`. Nine causal mutations reject a missing, displaced or altered clear, an additional result, an incorrect producer/input/consumer/reference, or a missing consumer. Existing type, service and Unknown/Void boundary checks remain.

These expectations come from each authored source's return mode. The oracle does not strip terminal actions, infer mode from the actual graph, or switch all fixtures to Legacy mode.

## Actual selected execution

```text
go test -count=1 -json -timeout=5m -run '^TestP15(ValueControlFunctionDiscovery|T04Opaque|M02|FN0[1-5]|T01(TypedDefaultBoundary|MapDefault|OptionalMapDefault|WrongTypedDefaults)|FunctionOptionalPresence)' .
```

| Actual family | Named passes |
| --- | ---: |
| FN01–FN03 typing and native graph tests | 60 |
| FN04 caller input graphs and corruption controls | 33 |
| T01 typed and Map default boundary tests | 290 |
| Function Optional presence graphs and corruption controls | 47 |
| M02 complete main-output, mode and migration tests | 172 |
| T04 opaque type and service tests | 50 |
| Value-control function discovery graphs and corruption controls | 163 |
| **Total** | **815** |

The exact discovered 32 top-level test names and every original JSON event are retained. The regular expression's possible FN05 match is not evidence of an executed FN05 family.

The process exited 0 and did not time out. Source was clean at the same commit before and after execution. Original stdout is **755,578 bytes**, SHA-256 `0fa8a4332cc9eda67f36228cf753b5db13c77ea81b620bc127ac6d80b50ad11c`; stderr is empty. Free C: space was 3,086,110,720 bytes before and 2,766,508,032 bytes after. The existing five-minute Go budget was unchanged.

## Original failures and independent intake

The original root stdout is **143,058 bytes**, SHA-256 `8e6bdfb5875aee9eee860823267da7b1a8f2969cb3b4c1d4f238fd6605bd6345`. Its five retained process files were independently compared byte for byte with the original root evidence directory.

The derived intake re-parsed every JSON event with strict UTF-8 and JSON decoding, required 815 unique run/pass pairs with active observed ancestors and one successful package terminal event, checked event times against the actual process interval, and required all 41 previously failing owned names to pass. It verified **83 immutable source blobs**, the three baseline test blobs, and the unchanged bytes of all three acceptance contracts. The clean source diff from the integration merge contains exactly the three test files described above.

The audit is `originals/derived-audit/audit.json`, SHA-256 `087a579f8c280f39032a8524d6022c85058675c52fbe254aad5dd1b671aa7b7f`. Its submitted script is retained. These derived checks do not replace any original process receipt.

An independent peer audit of the preceding 718e/3dc producer component is also retained verbatim. Its SHA-256 is `c4adcd51955d4b3c7221b4b7f56304c0347730045cf4dd4cf6e95a549dd45fbd`. That audit concerns the preceding component and supplies no execution claim for this source.

## Integration ancestry

1. `718e7a16ce0bfb1d5b3d132df06ab96f6c9793e9`: public-route/main-output signature and bounded source inventory integration.
2. `3dc23214a08128123257affb9f8d72dbe0d734a6`: separate original-byte/source proof of the actual four-producer replay.
3. `74e8a55dded1fbc3b4a13f26cee0e98e644d6599`: merge original root `8b8093ee` ancestry without rewriting its commits.
4. `4adf57a713e7386992aad7cb6dd39c3d8304c328`: the three-file test-oracle repair, committed before the clean replay.
5. This report and evidence are committed separately after the actual execution and intake.

The stored `oracle-alignment.patch` is the exact component diff, SHA-256 `020fa5ed771116f8eeb199a4bd09c5c1aaab3823349a06f5704e9c0570fd2307`. Raw files are protected from line-ending conversion. The artifact manifest binds each original location, byte size and digest.

## Scope still open

This component does not claim a complete Go suite, final formal179 acceptance, Apple/iOS execution, or a current-source full BRG30 run. The M02 local native-structure producer retains its own exact-source evidence and still requires Apple execution. Root Optional artifact and JSON repairs remain separate. The immutable original failure is not rewritten as a successful run.

Asana bounded integration task: https://app.asana.com/1/1219597879016953/project/1219603572114432/task/1220737551721032
