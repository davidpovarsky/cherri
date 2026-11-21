# K05 native observer recovery

Source: 26b3f79de15df5b073928eefa8725909aae6fddf. Base: d90b69a92e83b5034728c22e062a3dd96153a2a8.

The old two-action expectation failed on the actual V2 public fixture. The compiler emits Text, Show Result, then the exact bare implicit-Void Nothing action. Only test code changes: the observer independently specifies every action/parameter/envelope, validates a nonzero producer UUID and its exact Show Result attachment/name/range, rejects 21 actual native corruptions and accepts a consistent alternate UUID.

## Verified results

- Untouched K1 existing-file baseline failed only its healthy unsigned two-action assertion; all five other preservation/Legacy subcases passed. Original Go SHA256 2d93797b52414cc5882acd43deb1fb66a1a5748954de071d52df2ba488d3b2ce.
- Dirty development actual wrapper/oracle replay passed 24 named events.
- Clean seven-parent attempt at the source above reached the unchanged three-minute test deadline. It produced 38 named PASS events, including the complete existing-file parent, all 21 corruption controls, prepare-original, import/edit and signing-gate parents. It did not complete the remaining three parents and is retained as an incomplete failed attempt, never a seven-parent PASS.
- First live-only attempt exceeded the same test deadline while building the genuine source-pinned compiler, before fixture preparation or signing. The build finished afterward; its clean Git source, genuine Go VCS metadata and binary SHA were verified. No owned build/signing process remained before the fresh run.
- Fresh live-only process exited zero. TestP15K05LiveSigning passed in 153.32 seconds, below the unchanged 180-second Go test budget. Process wall time was 217.015 seconds including pre-test Go package compilation. Source was clean and unchanged before/after.

## Actual signing evidence

The compiler binary SHA256 is 935253f4d0b205e0a7ac1de0f85c43ebcb50e60eaade0a07619f6f8270abccd3. Original and edited graphs pass the exact native observer. One explicitly allowed edit is recorded. Gated unsigned bytes and actual sign input match SHA256 27b11a0198bb15f7158ad799f2f8fd2b07349dac5582d2dfd263e44a2d2d510a. The actual configured service reported Shortcuty Sign and produced 22182 bytes, SHA256 c03c54e45b8cd0a25c1ad35d43c0530a778578cf2969ade0e400070e11ae169f, with AEA1_HEADER_PASS container validation. No signing configuration or authorization was changed.

The archive retains original/edited sources, allowed-delta policy, gated bytes, actual sign input, signed artifact, gate/provenance receipts, full Go streams, process metadata and all unsuccessful attempts. The completed 25 MB compiler is retained externally with its build-info/hash receipt; it is not duplicated in Git.

## Scope and limits

This verifies the two corrected K05 observers and actual synthetic signing path. The full seven-parent group and final 179-requirement run still require successful final integration execution. No Apple import/runtime, installed consumer, compiler release or global completion is claimed. Neither timeout was relaxed or discarded.

Original evidence ZIP: f24fa35522c030bb4d9b362e1e23b4dcb98b6f211585e3d24cb521bfec5cef56 (40625 bytes; 36 original files).
