# AX canvas grouping from actual Legacy evidence

## Result and scope

The Inspector now recognizes the outer action regions exposed by the actual iPadOS Shortcuts AX hierarchy. Replaying the published synthetic tree from [run 44238251950](https://github.com/davidpovarsky/cherri/actions/runs/44238251950) changes the observation from eight nested Dictionary parameter rows to 61 outer action-region observations across seven scroll pages. Those 61 observations are **not 61 unique native actions**. The compiled shortcut has 34 actions, and page overlap remains unresolved because the observed groups expose no native action identifier or UUID. Native one-to-one comparison remains `UNKNOWN_AX_UNEXPOSED`; the release gate remains open.

The exact native run, runtime callback, trace and publication audit remain recorded in `legacy-runtime-44238251950-audit.json`. This change replays that existing evidence; it does not claim another signed execution.

## Concrete defect and repair

All 4,403 nodes in the saved tree have `actionContainerHint=false`; none exposes native identity. The prior extractor admitted `Cell` nodes and explicit action hints. Its eight selected cells were four Dictionary parameter rows repeated on two pages. The actual action regions are full-width `Other` siblings directly below the identified editor canvas. Their headers contain action controls and a separate title branch; the Number action exposes an editable title Button rather than StaticText.

The repair follows that actual hierarchy and checks header containment, canvas width, top alignment and geometric order. A nested Dictionary Table cannot contribute parameter rows as actions simply because its outer header is unavailable. Input-array order is ignored in favor of recorded sibling order. Conflicting sibling order, duplicate node identity and overlapping group geometry remain reported as ambiguity.

The extractor does not consult expected action count, order, labels, values or native identifiers. It does not use OCR or AI vision. Repeated page observations remain separate unless the already-existing stable native identity contract proves a duplicate. A new count-scope field identifies unresolved page observations. Visible text alone also no longer receives a native parameter `MATCH` when its containing group cannot be bound to the expected native action.

| Page | Observed outer regions |
|---|---:|
| 1 | 8 |
| 2 | 10 |
| 3 | 11 |
| 4 | 11 |
| 5 | 7 |
| 6 | 7 |
| 7 | 7 |

## Verification

`ax-canvas-grouping-validation.json` records the original published tree SHA-256, 340 exact copied fixture nodes and full-tree replay. Each checked-in fixture node was compared for equality with its original published node; the fixture contains complete selected subtrees and ancestors, not a fabricated complete editor capture.

The first eight causal tests produced six failures against the old extractor. The final suite executes 74 tests successfully, including ten real-tree grouping tests, the existing Inspector/OCR controls and runtime/publication/workflow contracts. Negative controls remove actual header roles or the editable Number title, duplicate a node, conflict sibling order, overlap geometry, repeat a page and rearrange the expected plan. An exact visible anchor still cannot become a native parameter match without native identity.

```sh
python3 -m unittest discover -s tools/shortcuts-simulator-qa -p 'test_*.py' -v
```

## Remaining native limitation

Opaque parameter-control identifier digests are reused and do not identify outer actions. Identical labels such as repeated Text or Add actions cannot prove cross-page identity. The captured AX tree therefore cannot establish a unique action count, native one-to-one correspondence, hidden parameter values or token attachment identity. No expected-plan interpolation is used to fill those gaps. Additional trustworthy native identity exposure or an independently validated observation mechanism is needed before the Inspector gate can close.
