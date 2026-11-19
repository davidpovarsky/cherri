# Acceptance CLI output isolation

Source fix: `ab812295e321f26376fb079313c68f547dd7f060`.
The clean repeat used a separate ordinary Linux Git clone at this exact commit, with an empty source diff before and after the run. The Windows authoring checkout contained identical source and test bytes.

The actual CLI is built and executed by the focused tests. An explicit external report file or directory receives all evidence companions, including raw local-runner evidence; the unrelated caller directory and the source checkout remain unchanged. Default invocation retains the established manifest-discovery layout. Reserved output names and failed mandatory evidence writes reject the invocation.

The CLI children deliberately have an empty executable search path so missing external producers reach the real local failure path promptly. The tests require a nonzero CLI exit, the unchanged 179 obligations, failed requirements, and incomplete closure. These output-routing tests do not claim that the 179 scenarios passed.

| Run | Source | Named cases | Result |
| --- | --- | --- | --- |
| Causal baseline | K4ff original main.go with the new regression tests | 19 discovered, 12 passed, 7 failed | Expected failures in reserved raw-evidence names, external destinations, and mandatory write failure |
| Working fix | Exact source/test bytes later committed as ab81229 | 19 discovered, 19 passed | Exit 0 |
| Clean repeat | ab81229, clean before and after | 19 discovered, 19 passed | Exit 0; 4.761 seconds |

The seven baseline failures include parent test events; they are not seven independent production defects. Existing output checks and the legacy default layout remained green.

Each receipt contains the exact command, source/test digests, source status, discovered cases, and raw-log SHA-256. The clean JSONL digest is `f68201821620399f5481304e8c133a2eecf6a767472f65866fbcc23d7b8222c1`.

The retained driver records the original execution paths. Its baseline source input is derivable without ambiguity using `git show 4ff1d089ec49528bb796fe315a74777d69008d9b:tools/language-acceptance/main.go`; expected SHA-256 is `913a6990077997a82d18ba28ca40bb4f8bcb8fa6c3d9cfb2da46ed6335a3f9ea`. Adjust only the driver paths when reproducing on another host.
