# Actual R7 calibration of the twelve original public inputs

At clean implementation `752c41b74f64fd633e63792fd4685156fabe913c`,
the exact R7 Linux ARM64 compiler
`42f6e42e7b6a8ea6f523a752fd2f06b96265e733fa00ad808190e062d1266301` executed each of
the twelve unchanged public S4 baseline source files exactly once. Every
compilation returned0; all12 emitted plists passed the existing independent
Python graph oracle. There were no retries or source repairs.

Total elapsed0.808499 seconds, with the original35-second per-case and
450-second aggregate bounds. Compiler bytes, all recorded source hashes, Git
HEAD and clean status were unchanged before/after. All1406 source bytes match
the historical original ZIP byte-for-byte. The twelve graph replays observed
48 explicit checks. Separate mechanical intake passed91 checks
without invoking the compiler again.

This proves compatibility of the reviewed six-task public calibration subset
with that exact native binary and validator. It is not an AI quality result,
an unseen-pattern claim, an Apple Shortcuts execution or a general compiler
correctness guarantee. No provider call or fresh evaluation seed occurred.

The original plan, all sources, commands, stdout/stderr, actual plists, compile
receipts, graph observations, source fingerprints and driver are retained.
The earlier S3/S4 and unconfigured-v2 records remain unchanged. This later
documentation does not restamp calibration as a different implementation.

Original ZIP SHA256: `a34c368139c1a22d9629e0bbe124c578d17b7f15caf234acf3c80e299123b193`.
External manifest SHA256: `ab87f153a5930b049af2464ec44e2c99bbdb76d4aa07c92255ca38ab91937167`.
