# RP50 discovery detail and cancellation evidence

The actual focused RP50 producer passed at clean source
`9a9e44798fde521b4869497166969728dedb5e0e` in **34.136746 seconds**.
Its original Go child actually discovered and passed all **15 original Python
controls**, including the unchanged assertion that a timed-out descendant's
process lock is already released at return. The narrow parent and its **9
existing corruption subcases** passed. RP50 still reports `PENDING_EXTERNAL`;
AI01 remains `AI_EVAL_NOT_RUN`. No provider, seed or model-quality measurement
is claimed.

## Original failures remain visible

K8 Linux job 137425839599 failed the generic supplemental assertion because
RP50's Detail omitted `TestP15RP50AIHarnessControlFlow`, although the exact
Go child and all 15 Python controls had passed. The original 10 MB CI log remains
at its recorded server path, SHA256
`f7a60ff605f70704845cddd7aa3aa7651241e96e25f3a07a4d831a49d88cd459`.
This packet includes its precise intake receipt, not a relabeled CI success.

Commit `c331645657768f08e7854e7f4f8f0eb7ea8ce489` derives Detail from actual
pass identities extracted only after strict validation of the original Go
JSONL and Python discovery receipt. It does not populate a claimed test list.
The new extraction parent and 3 causal controls passed, including exclusion
of an invented identity in free-form output.

The first actual narrow replay then failed in **26.422059 seconds**:14 of 15
Python controls passed. The original timeout test saw the worker lock still
held at return, then an absent worker and a released lock. POSIX group SIGKILL
delivery is asynchronous; waiting only for the direct child did not establish
that its descendants had finished. The complete failure, raw streams and
source witnesses remain unchanged in `focused-attempt-01`.

## Production repair and retained boundaries

Commit `9a9e44798fde521b4869497166969728dedb5e0e` adds an actual process-group
quiescence barrier after cancellation. Direct-child waiting and observation
share a five-second cleanup bound, each observer at most one second. The
original request timeout is unchanged. Observations contain only PGID/state
from portable `ps -A -o pgid=,stat=`; no process arguments, environment values,
credential or arbitrary diagnostic is collected or published.

The barrier waits for observed nonexecution. It adds no grace sleep or
adapter/test retry. True zombies have terminated, while Linux's `Zl` case
with remaining live threads stays active. Uncertain, malformed, failed or
over-budget observation fails closed. Only established quiescence permits the
original TimeoutExpired to be raised. Other cleanup failure retains streams
and yields a distinct safe failure. Windows cancellation behavior is unchanged.

Five new focused offline controls passed in **0.281771 seconds**. Their
receipt explicitly records working-source status; the tested bytes were
verified against the subsequent commit, not restamped as a clean execution.
They occupy a separate new test file, preserving RP50's fixed 15 inventory.

The original `test_runner.py`, all 15 methods and their timeout/lock assertions,
generic supplemental test, strict Go-event parser, three immutable 179
contracts, model settings and provider deadlines are unchanged. The Go source
witness additively binds the new control file.

## Actual second replay and independent verification

`focused-attempt-02` contains the clean-source command, outer streams, actual
child process receipt, all 48 source bindings, original child JSONL and sole
Python discovery receipt, plus the RP50 record. Independent mechanical
verification checked 82 conditions without rerunning the child,
compiler, provider or full supplemental parent. Every archive member was
checked byte-for-byte after ZIP creation.

The outer command selected only
`TestAIEvaluationHarnessEvidenceRetainsExternalRequirement`; its original
child kept the three-minute Go timeout and original Python bounds. All 9
existing corruption cases passed. The Detail contains the observed child
identity and satisfies the unchanged generic assertion. The next full CI
will exercise the complete supplemental parent; no extra 231-second parent
replay is claimed here.

`original-manifest.json` binds every archived original by size and SHA256.
The packet contains public harness/source evidence, no private corpus or live
outcome. An initial mechanical intake script used the wrong casing for the
JSON `status` key and stopped before writing proof; its diagnostic is
retained separately. Correcting that reader did not rerun or alter the tests.

Because `runner.py` is a v2 producer dependency, the final implementation
requires a new final-source public calibration and seal before live evaluation.
The historical 752 calibration and original packets remain untouched.

## Primary process-state references

- Apple ps options, header suppression and state definitions:
  https://github.com/apple-oss-distributions/adv_cmds/blob/main/ps/ps.1
- Procps live-thread zombie-leader exception:
  https://gitlab.com/procps-ng/procps/-/merge_requests/306
