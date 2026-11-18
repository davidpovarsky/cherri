# Empty List source and artifact verification

Source commit: `d61b59b9d1589a06b75fa7b545370b925337f8c7`.

The clean normal Git clone passed 15 top-level production contract tests, the full lowerer package, and three actual CLI fixture compiles. The binary embeds the exact source revision and `vcs.modified=false`; its SHA-256 is `814814e56f50624a90da59e17e6e8ab3d0fcba66916c0fe436de74364196705c`. Source remained clean before and after execution.

The same final test source failed under a Go overlay that reverted only the four production empty-list hooks. Ten binding/Repeat/Return/Yield identity scenarios expose the old representation; Legacy clearing, singleton empty Text, and exact imports remain passing controls. Six empty-value corruptions and twelve differential-fixture corruptions are rejected on the corrected graph. The broader compatibility selection passed before the commit and is explicitly recorded as such, not relabeled as clean-commit evidence.

| Unchanged fixture | Actual CLI native actions | Native runtime after this repair |
| --- | ---: | --- |
| `qa-collect-empty-split-v2` | 31 | Not executed |
| `qa-collect-counts-v2` | 136 | Not executed |
| `qa-collect-counts-legacy` | 239 | Not executed |

The detailed receipt pins each original source hash and compiled plist hash. Original expectations remain `direct=0:repeat=0:cleared=0:emptyText=1` and `empty=0:single=1:one:many=100:0:99`. Fresh signed source-specific pilots are still required. No fixture, expected native result, callback/trace condition, or publication rule was relaxed.

The first linked-worktree binary lacked embedded VCS fields despite an explicit build flag and was rejected before fixture use. The normal-clone build satisfied the provenance requirement. Captured public logs are unchanged bytes with their hashes recorded.

L01 nested List hierarchy, explicit nil assignment semantics, Inspector completeness, physical-device execution, and overall release completion remain outside this proof.
