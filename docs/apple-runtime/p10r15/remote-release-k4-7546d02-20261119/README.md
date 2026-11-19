# K4 candidate release verification — 19 November 2026

This archive preserves the original observed JSON and logs for candidate consumer K4 7546d0203405ba326e4e71ff9f74115dd0d34fa8, compiler S4 b97ae752205f1e747b6c98ff7511f3484dd53bc2, and documentation D4 205217a256b5f220724363b44b5ae516ffe6a539. The compiler binary SHA-256 is 563d0c763644bbd0ec9f33b94627c6e3143ee19ce41f5aa5675afb71c803d1bd. The release label is p15-release-s4-k4-20261119.

## Actual completed checks

- The canonical staging tool created an isolated candidate from clean ordinary Git clones and the source-bound S4 binary. Source and documentation clones are shallow; no complete historical Git inventory is claimed.
- Source archive creation and an independent archive verification each checked 20 files. Runtime archive creation, independent verification, and unpacked verification each checked 23 files, including actual compiler bytes and modes.
- The canonical entrypoint verifier executed all 37 rows with no skip flag: 37 passed, zero failed, and zero not run. This includes its two actual installed Go selections, TestCherriNoSign and TestDecomp. It is not a full Go-suite result.
- The installed signing fault verifier executed all 58 rows: five healthy controls and 53 correctly rejected negatives. The three timeout controls and both remote routes' 22 exact preflight requests were included.
- The candidate smoke verifier executed all eight rows, including both actual canonical signing requests using public synthetic source. Signed transport and AEA1 container checks are recorded. They do not establish Apple execution or cryptographic acceptance.
- The separately source-bound operational tool at 8261cb8add28af419bc9b52817b4d4a0a9256afc passed the strict read-only preflight against all nine original inputs. Its exact committed snapshot previously passed 16 causal Python tests. The tool repairs an actual source-package preflight error without changing packaged K4 files.

## State and limits

No activation, promotion journal, rollback, or reapply was begun. The preflight records unchanged live and candidate protected hashes, file modes, and the registered runner link. The user's existing Legacy preference remained unchanged. The candidate is retained while actual Apple diagnostics and the subsequent coherent app/consumer tuple are resolved.

The nine-input promotion manifest is an original execution artifact. Its relative paths describe the original Linux layout. The source/runtime ZIPs and binary remain at the exact original paths recorded in archive-index.json; their bytes are intentionally not duplicated in this Git evidence archive. A checkout of this archive alone is not a complete deployable stage.

The original release root is /home/chatgpt-agent/workspaces/cherri-p10r15-release-s4-20261119. Its candidate stage, material packages, original receipts, and public synthetic execution artifacts remain intact. The archive index records every copied member's original path, size, and SHA-256. Empty stderr logs and running-state receipts are retained exactly. In particular, entrypoints-execution.json contains a null generic passed field because the canonical verifier exposes activationGatePassed; the canonical candidate-entrypoints.json is the authoritative 37-row result. No observed file was rewritten to normalize that field.

This evidence establishes candidate package, entrypoint, fault, smoke, and read-only preflight behavior for the exact tuple above. It does not establish a live installation, D03 app acceptance, Apple runtime acceptance, complete formal179 acceptance, or final release completion.
