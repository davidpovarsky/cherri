# F04 native collection probes

The unchanged probes from consolidated source 8f1a4580db20e699aa7fc1b5542811cb16a50742 are now selectable through the existing signed native workflow and exact-source audit. No compiler behavior or acceptance status is changed here.

The List probe requires the exact summary:

    CHERRI_QA_F04_LIST_RESULT:pairs=2:scalars=2:shape=LEFT,TAIL|RIGHT,TAIL

The empty-List probe requires:

    CHERRI_QA_F04_EMPTY_LIST_RESULT:outer=2:scalars=2:shape=0,0

Both use two actual Repeat Each loops: one yielding the value under inspection, the other yielding scalar controls. The outer count consumers reference native Repeat Results UUIDs directly. Neither loop body contains an accumulator. The first probe checks each inner count and exact indexed values; the second checks two preserved empty inner collections.

## Verified local boundary

A clean Windows build from source 8f1a458 completed successfully in 44 seconds; the exact compiler SHA-256, command and Go metadata are retained. This executable lacks embedded VCS settings, so its source binding is the clean build receipt, not an embedded claim. The native workflow builds its own exact-SHA compiler.

Actual CLI compilation succeeded for both unchanged sources, producing 109 and 59 native actions. Original unsigned output bytes and hashes are retained here. A preliminary inspection script incorrectly guessed the existing debug trace variable name; the final graph check instead examines the loop boundaries and exact count-consumer UUIDs, and records that correction.

Four Python contract tests passed. These exercise literal reviewed selector/source bindings, exact fresh callbacks and trace, and rejection of flattened results, dropped empties, wrong scalar controls and changed inner elements. Shell syntax and the code whitespace check passed. Raw logs retain their original Windows CRLF bytes.

These checks do not establish Apple collection semantics. Signed native results are still required. Even a passing cardinality/content probe alone does not prove every arbitrary nested native type, real-device behavior, or the complete F04 requirement. The original F04 non-promotion guard remains unchanged.
