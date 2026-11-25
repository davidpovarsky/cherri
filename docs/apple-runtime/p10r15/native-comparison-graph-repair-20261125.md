# S6 native comparison graph verifier repair

The F01 run 45326630017 and nested-list run 45332640102 stopped in Phase 0
before signing or Apple execution. The native-boundary QA verifier required a
direct Text comparison adjacent to each flag assignment. S6 already lowers
statically typed Text equality to an exact, case-sensitive, fully anchored
quoted match and a Boolean result. The old QA graph shape was stale.

The shared verifier follows producer UUIDs and control groups through both
operand snapshots, literal regex escaping, full-input matching, item count,
the equality/inequality predicate, Boolean arms, and the final flag guard.
It rejects unreviewed actions and parameters, changed operands, altered
polarity, and a downgrade to direct Text comparison. Native dynamic
Clipboard/dictionary/Get Variable comparisons and numeric comparisons remain
explicit, separately checked paths; their semantics are not relabeled as
exact Text equality.

The native-boundary, Unicode export and nested Dictionary feasibility
verifiers now consume this graph contract. All original fixture bytes and
expected runtime outputs remain unchanged. Existing negative controls remain;
21 additional corruption controls run against the actual compiled graph.

## Executed validation

- Base/compiler source: 6dca0432e5887221ff0e4a27c70ed16025b24a8f.
- Real Phase 0 and its negative controls: exit 0, 40.82 seconds.
- Actual CLI attempts: 87; compiled artifacts: 85; expected unsupported-shape
  rejections: 2.
- Native-boundary corruption controls: 43 (22 retained, 21 new).
- Record/empty-list controls: 17; Unicode controls: 15.
- Compile, rejection and publication controls also passed.
- A first launcher stopped at its 30-second tool budget; its incomplete output
  is retained. The successful run used a suitable bounded background budget.

These results are local graph and compilation evidence. Signing, Shortcuts
runtime, UI equivalence and formal acceptance are not claimed.

The original receipt and outputs are retained on Oracle:
/mnt/development/projects/evidence/cherri-p10r15-apple-completion-20261125/.

| Evidence | SHA-256 |
| --- | --- |
| Phase 0 validation JSON | 81fe871fa0ed08855df8defc706486be5c951d745cacb5c0fccd9b6d79ee0835 |
| Phase 0 log | dac673240d30d948f966b6a98101c3bfc333209ff6ff2f9bdd2e6ce3866c4e8c |
| S6 executable | 84d2bf9368d75dcc74fad3bc548d84ae79e2c5cfb0136170d259322ad865500f |

The native-graph-repair-validation.json receipt binds each changed verifier's
exact bytes and records the dirty worktree used for this focused validation.
Final native and integration gates must run on the published candidate.
