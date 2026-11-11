# Cherri P10R7 — Consumer Compatibility, SSoT Propagation & CI Integration

**Branch:** `agent/cherri-p10r7-verified-semantic-fidelity-20261110`  
**Schema Fingerprint (`--capabilities-json`):** `787e9842bbe92b14dbf195bf896c371cb622cd0b6178532b146dc1da6164cc46` (preserved unchanged)  

---

## 1. Shared SSoT & Cross-Consumer Contract Preservation

All P10R7 compiler/decompiler enhancements were implemented without duplicating catalogs or fragmenting downstream consumers:

| Surface / Consumer | SSoT Contract Check | Status | Notes |
| --- | --- | --- | --- |
| **Action Schema & Generated Registry** | `go run ./tools/language-schema --verify-generated` | **PASS** | Scoping `IntentAppDefinition` flat-dictionary normalization to `rawAction` (`handleRawActionParams`) instead of `handleRawParams` (`action.go:collectAdditionalParams`) preserved upstream `actions/contacts.cherri` `StaticParameters` and kept `generated_registry.go` 100% aligned with zero schema drift. |
| **Control-Flow Lowering Ownership** | `internal/language/backend/control.go` + `shared_control.go` + `action-emitter-ownership.json` | **PASS** | Added `BuildMenuStartItems` and `BuildMenuCaseWithTitle` in shared backend `control.go` with thin delegation from `BuildMenuStart`/`BuildMenuCase` and `shared_control.go` so V1 and V2 share one canonical `WFMenuItems` / `WFMenuItemAttributedTitle` emitter. |
| **Documentation & Open Minis Skill** | `python tools/p10r3/verify_consumer_compatibility.py --check --mutate-check` | **PASS** (7 local `PASS`, `Remote Builder` `BLOCKED_EXTERNAL`) | `docs/language-v2/actions-reference.md`, `skills/cherri-shortcuts/compatibility-manifest.json`, sibling `cherrilang.org`, iOS Bridge, iOS UI, Preview (`preview-shortcut` pin), and Pure+Pack (`verify_or_generate_pack.py --verify-generated`) all match live compiler `schemaFingerprint` and `codecFingerprint`. |
| **P10 505 Ledgers & Execution Evidence** | `TestP10_VerifyAndRecord505CatalogExecution` + `tools/p10r3/generate_catalog_and_pack_ledgers.py` + `verify_ledger_consistency.py` | Regenerated cleanly before final check | Routing `appendVariable` (`is.workflow.actions.appendvariable`) through `rawAction` changed `appendVariable`'s verified `decompileMode` from `native_control_flow` to `rawAction_lossless_fallback` while preserving `505/505` structural round-trip parity (`native_cherri_call=497`, `native_control_flow=7`, `rawAction_lossless_fallback=1`). |
| **Legacy vs v2 Parity** | `go test ./tools/legacy-parity/...` + `go run ./tools/legacy-parity` | **PASS** | `47/47` fixtures (`9` bit-for-bit, `38` semantic equivalence) + `17/17` Apple actions verified across Legacy and v2. |
| **Machine-Checked Test Discovery** | `go run ./tools/ci-test-discovery -run '^Test(CORE\|CODEC\|CTRL\|META\|LANG\|IOS\|GEN)'` | **PASS** | All matrix suites executed and verified. |

---

## 2. P10R7 Gate & Report Integrity Tooling (CI-Ready Offline Verification)

1. **`tools/p10r7/independent_fidelity_oracle.py`**
   - Independent non-tautological evaluator (`compare_values`, `BijectiveUUIDScope`, `evaluate_shortcut_fidelity`) enforcing all 13 strict gates (`RAW_JSON_DECODE_PASS`, `BPLIST_OR_XML_DECODE_PASS`, `DECOMPILE_PASS`, `LEGACY_COMPILE_PASS`, `V2_COMPILE_PASS`, `STRICT_ACTION_SEQUENCE_PASS`, `ACTION_PARAMETER_WIRE_PASS`, `VARIABLE_GRAPH_PASS`, `CONTROL_FLOW_PASS`, `WORKFLOW_HEADER_PASS`, `THIRD_PARTY_DESCRIPTOR_WIRE_PASS`, `FULL_WORKFLOW_SEMANTIC_PASS`, `APPLE_RUNTIME_EXECUTED`).
2. **`tools/p10r7/test_independent_oracle_mutations.py`**
   - Adversarial negative mutation suite proving fail-closed detection (`M01`–`M36` mutations: dropped/reordered/extra actions, parameter wire mutations, attachment/UUID breaks, header corruption, third-party descriptor wrapping/dropping, and M31–M36 report-tampering & privacy leaks).
3. **`tools/p10r7/reconcile_private_and_sanitized_reports.py`**
   - Supports `--verify` (runs safely in GitHub Actions without access to private Shortcuts corpus), `--sync` (recomputes sanitized diff matrix and baseline table from private audit outputs locally), and `--mutate-check` (proves fail-closed tamper rejection on summary drift, false `PASS`, `BLOCKED_DEVICE` spoofing, scrambled rows, and private path leaks).
4. **`tools/p10r6/corpus_audit.py` Fix**
   - Removed historical hardcoded `"PASS"` assignments for `ACTION_SEQUENCE_PASS` and `FULL_WORKFLOW_SEMANTIC_PASS`; now delegates directly to `independent_fidelity_oracle.evaluate_shortcut_fidelity`, sets `THIRD_PARTY_DESCRIPTOR_PRESERVED` from `THIRD_PARTY_DESCRIPTOR_WIRE_PASS`, accepts `.json` inputs, and prints verified row sums.

---

## 3. GitHub Actions Workflow Impact

- `.github/workflows/build-test.yml` executes on push to `main` and `agent/**`:
  - `go build ./...`, full `go test -count=1 -timeout=15m -v ./...`, matrix discovery gate, `TestDecomp`, `TestPackages`, `legacy-parity`, `language-schema --verify-generated`, `verify_or_generate_pack.py --verify-generated`, `verify_ledger_consistency.py` (+ `--mutate-check`), `verify_consumer_compatibility.py` (+ `--mutate-check`), `test_coverage_and_gates.py`, and `run_release_gates.py --verify-registry --verify-commit --run-all`.
- Because private real-world Shortcuts (`001`–`038`) are never committed to Git (`cherri-shortcut-audit-20261110`), CI validates sanitized report consistency and adversarial oracle mutations offline (`reconcile_private_and_sanitized_reports.py --verify --mutate-check` and `test_independent_oracle_mutations.py`) while keeping live 38-corpus compilation anchored to the local private audit harness (`run_38_audit.py`).
- All provenance metadata for compiler/decompiler/test/oracle infrastructure additions will be recorded under `"infrastructure"` in `docs/fork-action-provenance.json` in a dedicated non-circular follow-up provenance commit carrying the exact implementation commit SHA in `firstForkCommit`.
