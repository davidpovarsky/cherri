# Independent encoding evidence reconciliation

Repaired source: `41bacffd89772f287af006d26daf605b17b92dee`.
Causal intake baseline: `50aa0d298bbff416dbea14cb8e44c15b5c5e3f0e`.

## Reproduced gap

The first repaired JXA producer validated its actual per-form outputs, but the
Go intake initially checked only summary fields and artifact/run/hash bindings.
It did not open each phase report and process receipt. A review identified
that recomputing those hashes could conceal internally contradictory contents.

The original production code was kept unchanged while a complete synthetic
receipt fixture and nine content mutations per requirement were added. All
18 mutations were incorrectly accepted by the real final manifest aggregator;
the rejection suite failed with 18 failing leaves plus its parent test. Those
raw baseline failures remain here. This was a real intake defect, not an
Apple runtime result.

## Repair and exact clean verification

Intake now independently parses the source-bound canonical EXPECT fixture,
all three actual per-form reports, and all three process receipts. It derives
each verdict from the observed state-class/form matrix, checks all 23 required
identities and seven forms, reconciles failure lists and the sole negative
case, and requires process receipts to match summary commands and exits.
It checks the real osascript executable, unchanged phase inputs, and isolated
negative input. The negative diagnostic is recomputed from actual stderr.
Duplicate JSON keys, malformed reports, absent fields, generic failures and
contradictory report/receipt contents cannot pass through new hashes.

On exact clean source 41bacff, 101 named Go intake/verification cases passed,
with zero failures or skips. All 18 previously accepted content mutations
were rejected. Additional controls reject a missing explicit exit, a false
diagnostic flag backed by generic stderr, and ambiguous JSON. The 12 existing
Python/Node process/protocol/CLI tests also passed. Working and clean logs
are separate, and the source was clean both before and after the final run.

## Scope

These tests use explicitly synthetic receipts and a test-only Node engine
fixture. They do not run WorkflowKit, a Shortcut, Simulator, or a physical
device. Formal CI01/RP47 Apple-engine acceptance remains NOT_RUN until the
mandatory macOS producer executes the real correct/corrupted/restored triplet
and its independently reconciled source/run-bound artifact is available.
No local fixture creates a formal native acceptance record.

The original 50aa0d2 producer/process proof and its limitations are preserved
in the sibling `required-encoding-gate-50aa0d2` archive. This follow-up records
the additional independently reproduced intake gap and its repair.
