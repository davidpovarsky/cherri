# P10R15 main-history integration

The integration worktree merges origin/main a35312a7435e5648989234c45ed6f478e3ded6e6 into the P10R15 descendant 69913a285a13ecbf29281232d178ac75e448774b. The pre-merge divergence was 15 main-only and 328 feature-only commits. This operation changes only the isolated integration branch; origin/main is not moved.

## Conflict decisions

Five conflicts were confined to agent documentation. Two add/add conflicts (GEMINI.md and the repository Asana coordination skill) were identical after normalizing CRLF/LF; the feature copies were retained without content modification. The router preserves an additional existing instruction to respect repository gates. The feature TEAM document preserves the current Codex collaboration-tool names, alongside the same main two-tier policy.

AGENTS.md retains the feature operating contract and its architecture, provenance and privacy rules. Main's Living Project Board section is added verbatim. The older technical reference is already preserved at docs/agent-technical-reference.md. Three historical lines differ there because the feature corrected the obsolete global-state/reset and stdlib description; those known obsolete instructions are not restored. No permission or release rule is removed to authorize this task.

The main-only manual builder/signing workflows, Live Activity workflow, Lumen diagnostics and MOT sample are retained. No compiler, canonical action definition, native codec or iOS implementation file is changed by this merge.

## Checks and rollback

The resolution check requires zero unmerged paths, no unresolved marker, the exact main versions of each newly added main-only executable file, and no compiler/action/iOS source delta relative to the feature parent. New workflow YAML, Bash syntax, and Python syntax are checked. The next candidate's full test matrix will cover the combined history.

Both parent SHAs and the 15 main-only commit IDs remain reachable from the merge commit. Returning a review branch to a pre-merge state can be done by a new revert of the merge using parent one; do not reset or force-push the user's branches. A production merge still depends on the actual release gates, which are currently open.
