# P15 verified Python selection — 2026-11-19

The clean Windows tools-package replay at `ea715ce298530acd86a836866264f79dc34b7f79` failed six existing canonical documentation tests with `exit status 9009: Python`. `documentationPython` had accepted the discoverable Windows Store `python3` alias without executing it. That path did not run the canonical generator.

The shared acceptance resolver now executes a bounded isolated Python 3 probe before selecting an interpreter. Its response must have the exact probe protocol, major version3 and isolated mode. Default selection skips unusable candidates; an explicitly configured interpreter that is absent, fails execution or reports the wrong protocol/version produces an error without silently selecting another runtime. Windows prefers `python`, while all candidates still undergo the same probe. `CHERRI_D03_PYTHON` remains the explicit canonical-documentation override; collector/exporter controls retain `CHERRI_TEST_PYTHON`.

The same existing six documentation tests now pass on Windows without an interpreter override. A focused run also executed the real non-Python executable/fallback/configuration controls, wrong-version/protocol controls and Python-to-Go formal exporter regression:9 top-level tests and81 test/subtest pass events,0 failures,78.08seconds (`dist/p15-acceptance-causal/python-portability-fixed.jsonl`). Canonical content, source identities, consumer requirements and all negative assertions are unchanged.

The partial broad replay remains canceled/incomplete with its original failures and logs preserved. This focused repair does not establish that the entire tools package or final179 acceptance has passed.

The shared CLI test builder additionally reports `ctx.Err()` and elapsed time on build failure, preserving its existing90second deadline and complete combined compiler output. This diagnostic change makes a future deadline distinguishable from a compiler error; it does not retroactively classify the earlier empty-output failures as proven timeouts.
