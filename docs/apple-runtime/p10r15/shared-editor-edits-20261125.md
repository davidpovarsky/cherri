# Shared editor source edits and snapshot ownership — 2026-11-25

This checkpoint repairs the common Go language service and its LSP/mobile wire adapters. It does not claim native C ABI, Swift UI, signed Shortcut execution, or complete BRG27/179 acceptance.

## Production behavior

- Completion candidates carry a source-owned text edit. The common service uses actual AST identity for interpolation references and the production lexer for incomplete identifiers, parameter labels and enum members. Replacing the complete identifier preserves its suffix boundary; selecting a parameter also replaces an existing colon and horizontal spacing without consuming its value.
- Both LSP and the actual `completeForMobile` producer emit `textEdit: {range: {start, end}, newText}` with zero-based UTF-16 positions. Existing completion metadata and insertion text remain available. The C export already delegates to this exact mobile producer; native ABI execution is a separate gate.
- Formatting obtains canonical output and the original complete document span from the same immutable document snapshot. LSP uses the real EOF instead of a fabricated line number.
- LSP diagnostic publication obtains diagnostics and their active document version under one service lock. A stale incoming edit cannot relabel the retained newer diagnostics.
- The existing E06 exact notification oracle now requires the known version 5. Its original scenario IDs and all corruption assertions remain.

## Actual causal and regression evidence

All evidence is original Oracle output outside the Git checkout.

| Execution | Result | Original receipt SHA256 |
| --- | --- | --- |
| Unchanged production, new joined LSP tests | Expected RED: all four parents failed for missing versions, missing completion edits and fabricated format EOF | `5abc941b8865d24ce052793f0b3f400d87017359c95b3eb23e08bc3a1ec4a050` |
| All LSP, language-service and protocol packages after repair | 274 named tests passed, 0 failed/skipped, 8.826 seconds | `6eb41641119b2c10d3d0c80b3255fed8895c60928e4880b7f79ab00af245acc2` |
| Actual mobile JSON producer and existing E06 mobile/coordinate/export-delegation controls | 53 named tests passed, 0 failed/skipped, 23.081 seconds | `013be373f5d0b88d8143d6ce299b66023e9f4858a05919d5332dd1be7ef73ba0` |
| Final actionable completion fixtures through both LSP and mobile JSON, including ten corruption controls | 25 named tests passed, 0 failed/skipped, 26.114 seconds | `00ad78b4a73da4ec82ff245d3145c104d473257e1fbab8835146fd27cc99a655` |

Receipt directories under `/mnt/development/projects/evidence/cherri-p10r15-final-completion-20261125/`:

1. `brg27-lsp-red-01/result.original.json`
2. `brg27-lsp-green-01/result.original.json`
3. `brg27-mobile-green-02/result.original.json`
4. `brg27-actionable-completion-green-01/result.original.json`

The first mobile owner script had a Python syntax error before execution; its original Sentinel job `job_d65b70c911e5` is retained and is not counted as a test run. The corrected owner used a fresh attempt.

The CRLF formatting fixture explicitly preserves the original comment's CRLF while canonical formatting controls statement spacing. The initial authored expectation assumed comment newline normalization; that expectation was corrected before GREEN, without changing the formatter or the original RED receipt.

The final parameter and qualified-enum fixtures deliberately contain `heighx` and `Color.Rex`. Their actual edits must produce `height` and `Color.Red`; a UI that ignores the edit cannot satisfy the corresponding future native scenario. Their caret positions and expected edit spans are unchanged from the earlier controls. The final receipt binds the final Go inputs, and publication verifies those inputs before committing.

## Exact shared examples

Parameter source:

```cherri
resizeImage(Clipboard, width: 800, heighx: 600)
```

The zero-based UTF-16 caret is line 0, character 37. Selecting `height` returns range 0:35–0:43 and `newText: "height: "`. Only `heighx: ` changes; `600` remains.

Qualified enum source:

```cherri
enum Color { Red = "native-red", Blue = "native-blue" }
function paint(color: Color, note: Text = "") -> Text { return note }
paint(Color.Rex)
```

The caret is line 2, character 14. Selecting `Red` returns range 2:12–2:15 and `newText: "Red"`. The qualifier and all declarations remain unchanged.

The existing mobile C request coordinates are one-based UTF-16; the returned edit coordinates are zero-based UTF-16, as before for rename and diagnostics. The LSP wire uses zero-based positions throughout.

## Remaining joined acceptance

The corresponding Swift code must decode and apply this common edit against the captured source/revision. Actual native bridge and UI tests, their original XCTest evidence, and the combined canonical BRG27 intake remain required. This checkpoint supplies no automatic evidence credit to unrelated E01/E02/E03/E04/E05/E06/P05 requirements.
