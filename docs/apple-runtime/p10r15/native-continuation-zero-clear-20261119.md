# Continuation graph gate: exact zero-output debug clear

OpenMinis run44622158293 at e61e0d306bb363c39ade612ef35ccfc8051ddf3c failed in verify_native_continuation_probes.py with an unreviewed native effect. The unchanged verifier was replayed against four actual CLI outputs and rejected each one for the same Nothing action introduced by the corrected zero-entry debug-log reset.

Commit25b18add2f5a74144fb221f91729a73574a218f6 repairs only this graph validator. Every reviewed source must retain the public top anchor followed by exactly one Nothing with only a valid UUID, then SetVariable for __cherri_debug_log without an explicit input. Nothing is not added to the general action allowlist. Extra clears, a Text seed, extra parameters, explicit input, missing UUID, wrong target, reordered actions, additional log assignments and consumption of the zero-output identity are rejected. The original storage, HTTP endpoint/type and numeric comparison checks remain intact.

The clean committed-source producer rebuilt the compiler and passed all four real CLI compiles and all four lossless import/recompile roundtrips, preserving exact native action graphs. All63 causal mutations were rejected:23 existing controls plus10 new clear corruptions per fixture. The13 two-invocation storage policy/oracle tests also passed. Source remained clean throughout; Apple execution count is0.

The accompanying public JSON is a bounded summary of the actual producer report, retaining source/compiler/artifact hashes, each exact roundtrip and rejected mutation, the Python transcript hash and the original validation/stdout digests. It is not a newly generated native runtime receipt. Producer stdout SHA256:236b774f43d40f830e858f2cfbb3c3dbac6e0f0a6c44a3f742dbf682a8db381f.

The failed e61 CI run remains unchanged. The next integrated candidate still requires actual OpenMinis/CI execution; this local pass grants no storage persistence, cache implementation, Apple runtime or formal acceptance credit.
