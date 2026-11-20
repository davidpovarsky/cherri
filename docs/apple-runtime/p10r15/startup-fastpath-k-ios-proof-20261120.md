# K iOS terminal observation and bounded startup follow-up

The complete iOS run 44653152140 on K 8e66254d56ea3e4ac70f6cda6ef9d31217e0937c reached a real test failure, xcodebuild 65. It observed 48 passing and 5 failing tests, and completed publication of the original bounded test artifact 14594367314. It did not hit the 75-minute job limit.

## Actual observations

The real UIKit single-character paste test passed its original exact assertion: source text("x") became text("x")), UTF-16 length 9→10 and selection 8→9. The view was attached to a key window and was first responder. Both DocumentationIndex unit tests passed.

Five UI tests failed at the shared editor readiness assertion: BRG27 workflow, D03 search, E01 completion display, E03 hover/rename and typed-source/reset. D03's body was not reached: there are no documentation UI-state records, and app-integration acceptance remains unverified. The collector emitted no formal records because the complete xcodebuild result failed.

BRG27 had an existing, hittable Create Document control at 4.743s but no creation was issued before expiry. D03 had the same ready control at 3.680s but creation waited until 26.900s; no editor was observed at 40.136s. Typed-source/reset created at 19.788s and observed an interactive editor only at 30.179s, correctly rejected as late. E01 completion and E03 had a non-hittable creation control and no creation was issued. These different outcomes are retained without asserting one unproven cause for all five failures.

## Separate source follow-up

Implementation 0d1fcac738c381986987fab1dc2965c7943507c5 removes the unconditional absent-editor wait and does not wait again for a creation control that a live observation already found ready. It obtains fresh pre-tap state, checking the editor after the creation control so an arriving editor suppresses creation. A non-hittable control still requires a real bounded wait. The original one-shot creation policy and 28-second total budget remain unchanged.

The final assertion still requires an actual completed wait and a current foreground app, window, existing editor and hittability before the deadline. A direct document-check receipt explicitly reports waitCompleted=false; the collector rejects attempts to pretend that a direct check completed a wait. Source/private document content remains unavailable before a verified empty reset.

Clean local validation on the implementation commit passed 37 compiled Swift policy controls, 28 Python collector tests and Swift UI parsing. That is local control verification, not a new UIKit result. A source-bound Apple replay must still prove whether this removes the repeated-wait delay and whether the two non-hittable cases remain blocked.

The JSON proof retains exact source, original artifact/log digests, all 53 named outcomes, failed startup states, actual paste observations and the separate clean local checks. No requirement is closed by this proof.
