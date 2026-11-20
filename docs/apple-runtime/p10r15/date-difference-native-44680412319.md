# Date difference native failure — run 44680412319

## Preserved result

Exact source 7f044196d042dcdbbcc7401dcc96177f059b0db3 ran on iPadOS 27 Simulator and ended **FAILURE**. Its original strict output oracle remains **FAIL_RUNTIME**. The native callback itself was a fresh, source-bound SUCCESS callback and contained all 27 diagnostic fields plus before/after trace events. Callback success is not a passing semantic contract.

The companion JSON retains every expected and observed field, decoded UTF-8 witnesses, original artifact hashes, run/source identity and actual independent intake outcome. The original fixture and expected output were not changed.

## What was actually observed

| Control | Actual type | Actual item count | Actual value |
|---|---|---|---|
| Start Date | Date | 1 | 2001-02-03T04:05:06Z |
| Same Date | Date | 1 | 2001-02-03T04:05:06Z |
| Later Date | Date | 1 | 2001-02-03T04:06:06Z |
| Independent numeric zero | Number | 1 | 0 |
| Zero interval, Seconds | empty | 0 | empty |
| Positive interval, Seconds (target 60) | empty | 0 | empty |
| Zero interval, Minutes | empty | 0 | empty |
| Positive interval, Minutes (target 1) | empty | 0 | empty |
| Same-producer interval, Seconds | empty | 0 | empty |

Twelve fields match and fifteen interval fields contradict the independent target. Positive intervals fail as well as zero intervals, so the observation is not limited to zero text formatting. This fixture contains no debugLog; it does not establish a Date-retention defect in the label macro.

## Original evidence and actual intake

The original artifact is 14608034750, ZIP SHA256 fe639f4db6051344e24d12ae083459ec9c26ff369377dcd156aab47253c1b729. It was retained without alteration, with original run/job metadata and job log.

The unchanged auditor from the claimed source was executed against the downloaded artifact and actual failed run metadata. It verified all 21 published file hashes, exact source/signing/nonce bindings and immutable expected output, then reproduced FAIL_RUNTIME_PRESERVED with exit 2. Its report SHA256 is cbc5b7fe7ad2841ca74b11727b9c1cf5e7201ac4fa9eac356efe419490e5922a. Because the actual baseline is not passing, its false-success mutation stage is explicitly NOT_RUN_BASELINE_NOT_PASS; no rejection count is invented.

Four Receiver/import XCTest cases passed. Those helper results do not replace the failing Date contract or establish six-mode label acceptance.

## Wire investigation boundary

The retained clean local compilation emits bare WFTextTokenAttachment ActionOutput references for both WFTimeUntilFromDate and WFInput in all five interval actions. This is a same-production-source local artifact, not a downloaded copy of the run's unsigned bytes; its distinct digest and source are recorded.

Retained Apple metadata confirms the existing key names and identifies both parameters as WFDateFieldParameterState. The existing parameter probe wraps references for that state in WFTextTokenString. This is a concrete codec hypothesis to test through native serialization, not yet a proved runtime cause. No parameter name, golden, production codec or formal membership is changed by this report.
