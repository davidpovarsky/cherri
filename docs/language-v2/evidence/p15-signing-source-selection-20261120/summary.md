# Signing source selection: actual current CLI and pinned Skill inputs

Implementation: b4e04f3c6050ca841e892cc4337546424588e6d7, two harness/workflow files only. The manifest and S/D6 pins are unchanged.

The original remote run 44725798960 used its authentic 486 compiler for copied Skill checks, whose unchanged manifest requires fd. All 11 Skill cases stopped at the source identity check before a signing request. The original 23 direct CLI successes remain valid. A fresh real 486 ordinary-clone build reproduced the 11 identity failures locally; its 33-case bounded baseline excludes the optional timeout case, which remains in the original remote 34-case attempt.

The repaired harness accepts a separate Skill compiler and records the actual compiler hash per row. The workflow builds the manifest-pinned source in an ordinary clean clone, verifies its embedded role/source/platform/clean identity with the migration inspector and hashes the bytes before using it. Current-HEAD direct CLI checks remain distinct; later pinned runtime packaging reuses the verified source. No copied manifest pins are rewritten.

The clean b4 full local replay used the genuine 486 direct CLI and genuine fd pinned Skill binary. It completed 34 cases in 111.5 seconds: 3 unsigned controls and 31 actual one-request multipart fault rejections, no failures or failure artifacts. The exact inline workflow identity assertion accepted the real inspector receipt and rejected 7 individually corrupted fields. Missing --skill with --skill-compiler rejects before a report. All 3 changed workflow shell steps passed syntax validation.

report.json preserves exact commands, source identities, hashes, timings, scope and the prior failure. source-witnesses.json binds 30 immutable blobs across harness b4, original 486 and pinned fd. originals.zip preserves the original remote artifacts/log/API data, actual local build/baseline/replay/control receipts and submitted scripts; artifact-manifest.json maps every member to its byte-equal retained original.

No remote repaired-workflow result, Apple cryptographic validation, new M02 consumer tuple, full CI, native-container cardinality or 179 closure is claimed here. The initial upload syntax failure is retained as no test execution. Root independently reviewed the exact two-file implementation before authorizing one conditional remote replay.
