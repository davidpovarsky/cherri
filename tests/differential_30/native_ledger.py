#!/usr/bin/env python3
"""Build an evidence ledger from preserved native receipts; never run native code.

Each historical observation keeps its recorded expected value and verdict.
The current view selects the latest semantic observation whose oracle matches
the current contract. Artifact identity and proof strength are separate fields.
"""
import argparse
import collections
import datetime
import functools
import hashlib
import json
from pathlib import Path
import re

LANES = ("original", "fork_legacy", "fork_v2")
DIGEST = re.compile(r"^[0-9a-f]{64}$")

@functools.lru_cache(maxsize=None)
def sha(path):
    return hashlib.sha256(Path(path).read_bytes()).hexdigest()

@functools.lru_cache(maxsize=None)
def load(path):
    return json.loads(Path(path).read_text())

def evidence(path):
    path = Path(path)
    return {"path": str(path), "sha256": sha(str(path)), "bytes": path.stat().st_size}

def save(path, value):
    Path(path).write_text(json.dumps(value, ensure_ascii=False, indent=2) + "\n")

def number(identifier):
    match = re.match(r"^(\d+)(?:_|$)", str(identifier))
    return int(match.group(1)) if match else None

def text_expected(value):
    return value.get("value") if isinstance(value, dict) else value

def source_file(row, receipt_path, command):
    candidates = [row.get("source"), row.get("staged_source")]
    if command and len(command) > 1:
        candidates.append(command[1])
    if row.get("source"):
        candidates.append(str(Path(receipt_path).parent / "sources" / row.get("lane", "") / Path(row["source"]).name))
    expected = row.get("source_sha256", row.get("staged_source_sha256"))
    for candidate in candidates:
        if candidate and Path(candidate).is_file() and sha(str(candidate)) == expected:
            return str(candidate)
    return None

def resolve_creation(receipt_path, row, depth=0):
    if depth > 8:
        raise ValueError("compiler receipt reuse chain exceeds eight")
    chain = [evidence(receipt_path)]
    if row.get("command"):
        return row["command"], row.get("returncode"), chain
    previous = row.get("reused_from")
    if not previous:
        return None, row.get("returncode"), chain
    path = str(Path(previous) / "results.json")
    prior = next(x for x in load(path)["compilations"]
                 if x["fixture_id"] == row["fixture_id"] and x["lane"] == row["lane"])
    for key in ("source_sha256", "binary_sha256", "artifact_sha256", "compiler_flags"):
        assert prior.get(key) == row.get(key), (path, key)
    assert prior["status"] == "COMPILED"
    command, code, older = resolve_creation(path, prior, depth + 1)
    return command, code, chain + older

def compilation_index(config):
    index = collections.defaultdict(list)
    blocked = {}
    receipts = []
    def add(entry):
        artifact = entry["artifact"]
        entry["artifact_hash_verified"] = Path(artifact).is_file() and sha(artifact) == entry["artifact_sha256"]
        assert entry["artifact_hash_verified"], artifact
        compiler = entry.get("compiler_path")
        entry["compiler_hash_verified"] = bool(compiler and Path(compiler).is_file()
            and sha(compiler) == entry.get("compiler_sha256"))
        src = entry.get("verified_source")
        entry["source_hash_verified"] = bool(src and Path(src).is_file()
            and sha(src) == entry.get("source_sha256"))
        entry["lineage_verified"] = (entry["artifact_hash_verified"] and entry["compiler_hash_verified"]
            and entry["source_hash_verified"] and entry.get("exit") in (0, None) and bool(entry.get("command")))
        entry["creation_exit_code_recorded"] = entry.get("exit") is not None
        entry["lineage_id"] = hashlib.sha256(json.dumps(entry, sort_keys=True).encode()).hexdigest()[:20]
        index[entry["artifact_sha256"]].append(entry)
    for path in config.get("compile_receipts", []):
        data = load(path)
        receipts.append(evidence(path))
        for row in data["compilations"]:
            if row["lane"] not in LANES:
                continue
            if row["status"] != "COMPILED":
                blocked[number(row["fixture_id"]), row["lane"]] = {
                    "status": row["status"], "diagnostics": row.get("diagnostics", []),
                    "receipt": evidence(path), "row_fixture_id": row["fixture_id"],
                    "command": row.get("command"), "exit": row.get("returncode")}
                continue
            command, code, chain = resolve_creation(path, row)
            binary = data["binaries"][row["lane"]]
            add({"number": number(row["fixture_id"]), "fixture_id": row["fixture_id"], "lane": row["lane"],
                 "name": row["shortcut_name"], "artifact": row["artifact"],
                 "artifact_sha256": row["artifact_sha256"], "declared_source": row["source"],
                 "verified_source": source_file(row, path, command), "source_sha256": row["source_sha256"],
                 "compiler_path": binary["path"], "compiler_sha256": row["binary_sha256"],
                 "compiler_source_sha": data["original_sha"] if row["lane"] == "original" else data["fork_sha"],
                 "compiler_source_state": "PINNED_RECEIPT_REVISION",
                 "command": command, "exit": code, "receipt_chain": chain,
                 "graph_valid_as_recorded": (row.get("graph") or {}).get("valid"), "graph_errors_as_recorded": (row.get("graph") or {}).get("errors"), "expected_at_compile": row["expected"]})
    for path in config.get("legacy_compile_receipts", []):
        data = load(path)
        receipts.append(evidence(path))
        for row in data["cases"]:
            if row["status"] != "COMPILED":
                continue
            base = Path(path).parent
            add({"number": row["number"], "fixture_id": row["id"], "lane": row["lane"],
                 "name": row["name"], "artifact": str(base / row["unsignedFile"]),
                 "artifact_sha256": row["unsignedSHA256"],
                 "declared_source": row["source"], "verified_source": str(base / row["source"]),
                 "source_sha256": row["sourceSHA256"], "compiler_path": data["compilerPath"],
                 "compiler_sha256": data["compilerSHA256"], "compiler_source_sha": data["declaredCompilerCommitSHA"],
                 "compiler_source_state": "PINNED_RECEIPT_REVISION", "command": row["command"], "exit": row["exit"],
                 "receipt_chain": [evidence(path)], "input_source_sha256": row["inputSourceSHA256"],
                 "original_name": row["originalName"], "expected_at_compile": row["expected"]})
    for path in config.get("candidate_compile_receipts", []):
        data = load(path)
        receipts.append(evidence(path))
        if "candidates" in data:
            # Clean revision candidates retain their exact public compilation
            # command and exit code. This is distinct from the earlier dirty
            # statistics receipt, which never recorded an exit field.
            for row in data["candidates"]:
                add({"number": number(row.get("fixtureID", row["id"])),
                     "fixture_id": row.get("fixtureID", row["id"]), "lane": row["lane"],
                     "name": row["name"], "artifact": row["unsignedPath"],
                     "artifact_sha256": row["unsignedSHA256"],
                     "declared_source": row["sourcePath"], "verified_source": row["sourcePath"],
                     "source_sha256": row["sourceSHA256"], "compiler_path": data["compilerPath"],
                     "compiler_sha256": data["compilerSHA256"], "compiler_source_sha": data["sourceCommit"],
                     "compiler_source_state": "PINNED_CLEAN_RECEIPT_REVISION" if data.get("sourceClean") else "PINNED_RECEIPT_REVISION",
                     "command": row["command"], "exit": row["returncode"],
                     "receipt_chain": [evidence(path)], "expected_at_compile": row["expected"],
                     "graph_valid_as_recorded": (row.get("graph") or {}).get("valid"),
                     "graph_errors_as_recorded": (row.get("graph") or {}).get("errors")})
            continue
        for row in data["cases"]:
            lane = {"v2": "fork_v2", "legacy": "fork_legacy"}.get(row["lane"], row["lane"])
            add({"number": number(row["id"]), "fixture_id": row["id"], "lane": lane,
                 "name": row["name"], "artifact": row["artifact"], "artifact_sha256": row["artifact_sha256"],
                 "declared_source": row["staged_source"], "verified_source": row["staged_source"],
                 "source_sha256": row["staged_source_sha256"], "compiler_path": data["candidate_binary"],
                 "compiler_sha256": data["candidate_binary_sha256"], "compiler_source_sha": data["base_commit"],
                 "compiler_source_state": "CANDIDATE_BASE_PLUS_RECORDED_PRODUCTION_FILE_HASHES",
                 "production_file_hashes": data["dirty_production_files"],
                 "command": row["argv"], "exit": None, "receipt_chain": [evidence(path)],
                 "expected_at_compile": row["expected_output"],
                 "exit_evidence": "Candidate receipt includes successfully decoded compiler artifact; no exit field in this receipt."})
    return index, blocked, receipts

def snapshot_files(snapshot):
    index = load(str(Path(snapshot) / "transfer-index.json"))
    for row in index["files"]:
        path = str(Path(snapshot) / row["relative"])
        assert sha(path) == row["sha256"], path
    return index

def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--config", type=Path, required=True)
    parser.add_argument("--out", type=Path, required=True)
    args = parser.parse_args()
    args.out.mkdir(parents=True, exist_ok=False)
    config = load(str(args.config))
    main_manifest = load(config["main_manifest"])
    old_manifest = load(config["old_manifest"])
    audit = load(config["legacy_summary_audit"])
    assert audit["manifest_sha256"] == sha(config["old_manifest"])
    old_cases = {c["number"]: c for c in old_manifest["fixtures"]}
    contracts = {c["number"]: {"id": c["id"], "number": c["number"],
        "expected": text_expected(c["expected"]), "runtime_enabled": c["runtime"]["enabled"],
        "families": c.get("families"), "scope": "MAIN_CORPUS",
        "oracle": evidence(config["main_manifest"])} for c in main_manifest["fixtures"]}
    for path in config.get("supplemental_manifests", []):
        for case in load(path)["fixtures"]:
            contracts[case["number"]] = {"id": case["id"], "number": case["number"],
                "expected": text_expected(case["expected"]), "runtime_enabled": case["runtime"]["enabled"],
                "families": case.get("families"), "scope": "SUPPLEMENTAL",
                "oracle": evidence(path)}
    for case in config.get("pending_cases", []):
        contracts.setdefault(case["number"], dict(case, scope="SUPPLEMENTAL", oracle=None))
    compile_index, blocked, compile_receipts = compilation_index(config)
    observations = []
    run_records = []
    snapshots = []
    known_observations = set()
    signed_file_index = {}
    for snapshot in config["snapshots"]:
        transfer = snapshot_files(snapshot)
        snapshots.append(evidence(Path(snapshot) / "transfer-index.json"))
        artifact_path = Path(snapshot) / "manifest-artifact-files.json"
        if artifact_path.exists():
            for record in load(str(artifact_path))["files"]:
                signed_file_index[record["path"]] = (record, evidence(artifact_path))
        for directory in sorted((Path(snapshot) / "runtime").glob("differential*")):
            if not directory.is_dir() or not (directory / "manifest.json").exists():
                continue
            manifest_copy_path = directory / "manifest.json"
            manifest_path = manifest_copy_path
            manifest = load(str(manifest_copy_path))
            summary_path = directory / "summary.json"
            summary = load(str(summary_path)) if summary_path.exists() else {}
            claimed_manifest_hash = summary.get("manifestSHA256")
            # Early harness versions reserialized their local manifest copy.
            # Bind to the preserved exact input bytes named by the run hash,
            # and separately prove that the copied JSON retains every value.
            copied_manifest_json = json.dumps(manifest, sort_keys=True, ensure_ascii=False, separators=(",", ":"))
            input_manifest_json_matches_copy = False
            for transfer_row in transfer["files"]:
                if transfer_row["sha256"] == claimed_manifest_hash:
                    candidate_path = Path(snapshot) / transfer_row["relative"]
                    candidate = load(str(candidate_path))
                    candidate_json = json.dumps(candidate, sort_keys=True, ensure_ascii=False, separators=(",", ":"))
                    if candidate_json == copied_manifest_json:
                        manifest_path = candidate_path
                        input_manifest_json_matches_copy = True
                        break
            manifest_binding = {
                "claimed_input_sha256": claimed_manifest_hash,
                "input_manifest": evidence(manifest_path),
                "runtime_copy": evidence(manifest_copy_path),
                "exact_input_hash_found": sha(str(manifest_path)) == claimed_manifest_hash,
                "input_and_copy_json_values_equal": input_manifest_json_matches_copy,
                "input_and_copy_bytes_equal": sha(str(manifest_path)) == sha(str(manifest_copy_path)),
            }
            result_path = directory / "results.json"
            rows = load(str(result_path)) if result_path.exists() else []
            if not isinstance(rows, list):
                rows = rows.get("cases", rows.get("results", []))
            cases = {(r.get("id"), r.get("lane"), r.get("name")): r for r in manifest.get("cases", [])}
            run_records.append({"run": directory.name, "manifest": evidence(manifest_path),
                "summary": evidence(summary_path) if summary_path.exists() else None,
                "results": evidence(result_path) if result_path.exists() else None,
                "summary_payload": summary,
                "manifest_hash_matches_summary": summary.get("manifestSHA256") == sha(str(manifest_path)),
                "input_manifest_binding": manifest_binding,
                "observed_rows": len(rows),
                "missing_results": not result_path.exists()})
            image_path = directory / "receiver-image-identity.json"
            image = load(str(image_path)) if image_path.exists() else None
            for position, row in enumerate(rows):
                stable = hashlib.sha256(json.dumps(row, sort_keys=True).encode()).hexdigest()
                duplicate = (directory.name, stable)
                if duplicate in known_observations:
                    continue
                known_observations.add(duplicate)
                n = number(row.get("id"))
                lane = row.get("lane")
                case = cases.get((row.get("id"), lane, row.get("name")))
                callback = row.get("callback") or {}
                shape = callback.get("resultShape") or {}
                import_receipt = row.get("importReceipt") or {}
                expected = row.get("expected")
                expected_bytes = expected.encode("utf-8") if isinstance(expected, str) else None
                expected_digest = hashlib.sha256(expected_bytes).hexdigest() if expected_bytes is not None else None
                actual_digest = shape.get("utf8SHA256")
                raw_digest_present = isinstance(actual_digest, str) and DIGEST.fullmatch(actual_digest) is not None
                summary_text = callback.get("resultSummary")
                summary_bytes = summary_text.encode("utf-8") if isinstance(summary_text, str) else None
                exact_summary_raw = bool(raw_digest_present and summary_bytes is not None
                    and hashlib.sha256(summary_bytes).hexdigest() == actual_digest
                    and len(summary_bytes) == shape.get("utf8ByteCount"))
                envelope = {
                    "callback_from_shortcuts": callback.get("eventSource") == "shortcuts-x-callback",
                    "receiver_is_not_self_test": callback.get("receiverSelfTest") is False,
                    "nonce_matches": bool(row.get("nonce")) and callback.get("runId") == row["nonce"],
                    "fixture_matches": callback.get("fixtureId") == row.get("id"),
                    "shortcut_name_matches": callback.get("expectedShortcutName") == row.get("name"),
                    "import_identity_verified": import_receipt.get("identityVerified") is True,
                    "import_name_matches": import_receipt.get("name") == row.get("name"),
                    "import_succeeded": import_receipt.get("outcome") == "IMPORTED",
                    "callback_success": callback.get("status") == "SUCCESS" and callback.get("host") == "success",
                }
                envelope_valid = all(envelope.values())
                manifest_valid = bool(case and case.get("expected") == expected
                    and case.get("signedSHA256") == row.get("signedSHA256")
                    and summary.get("manifestSHA256") == sha(str(manifest_path))
                    and input_manifest_json_matches_copy)
                source_case = old_cases.get(n)
                bounded = bool(envelope_valid and manifest_valid and row.get("status") == "PASS"
                    and not raw_digest_present and source_case
                    and source_case["id"] in audit["fixture_ids"]
                    and text_expected(source_case["expected"]) == expected
                    and audit.get("all_single_line") and audit.get("all_no_edge_whitespace")
                    and summary_text == expected and isinstance(expected, str)
                    and "\n" not in expected and "\r" not in expected and expected.strip() == expected
                    and shape.get("nonemptyLineCount") == 1
                    and shape.get("utf8ByteCount") == len(expected_bytes))
                raw_match = bool(envelope_valid and manifest_valid and raw_digest_present
                    and actual_digest == expected_digest and shape.get("utf8ByteCount") == len(expected_bytes or b"")
                    and summary_text == expected)
                image_match = bool(image and image.get("outcome") == "MATCH"
                    and image.get("executableNameMatches") is True
                    and not image.get("missingImages") and not image.get("extraImages") and not image.get("changedImages")
                    and image.get("built", {}).get("images") == image.get("installed", {}).get("images")
                    and bool(image.get("built", {}).get("images"))
                    and row.get("receiverImageIdentitySHA256") == sha(str(image_path))
                    and summary.get("receiverImageIdentitySHA256") == sha(str(image_path)))
                if raw_match:
                    proof = "RAW_UTF8_DIGEST_AND_RECEIVER_IMAGE_MATCH" if image_match else "RAW_UTF8_DIGEST_RECEIVER_IMAGE_NOT_RECORDED"
                elif bounded:
                    proof = "BOUNDED_SINGLE_LINE_SUMMARY_AND_RAW_LENGTH_NO_DIGEST"
                elif raw_digest_present and envelope_valid and manifest_valid:
                    proof = "RAW_UTF8_DIGEST_OBSERVED_OUTPUT_DIFFERS"
                else:
                    proof = "RECORDED_OBSERVATION_WITHOUT_EXACT_RAW_PROOF"
                selected_lineage = []
                if case:
                    for lineage in compile_index.get(case.get("unsignedSHA256"), []):
                        if lineage["number"] == n and lineage["lane"] == lane:
                            selected_lineage.append(lineage)
                signed_validation = None
                if case and case.get("signedPath") in signed_file_index:
                    signed_record, signed_proof = signed_file_index[case["signedPath"]]
                    signed_validation = {"file": signed_record["path"], "exists": signed_record["exists"],
                        "actual_sha256": signed_record.get("sha256"), "bytes": signed_record.get("bytes"),
                        "matches_manifest": signed_record.get("sha256") == case.get("signedSHA256"),
                        "proof": signed_proof}
                completion_id = (row.get("runtimeUIReceipt") or {}).get("id")
                completion_path = directory / ("completion-" + str(completion_id) + ".json")
                import_path = directory / ("ack-" + str(import_receipt.get("id")) + ".json")
                checks = {
                    "completion_receipt_matches": completion_path.exists() and load(str(completion_path)) == callback,
                    "import_receipt_matches": import_path.exists() and load(str(import_path)) == import_receipt,
                }
                semantic_status = "NO_SEMANTIC_VERDICT"
                if row.get("status") == "PASS":
                    semantic_status = "PASS" if (raw_match or bounded) else "PASS_RECORDED_PROOF_INCOMPLETE"
                elif row.get("status") == "OUTPUT_MISMATCH" and envelope_valid and manifest_valid:
                    semantic_status = "FAIL_OUTPUT"
                obs_id = directory.name + ":" + stable[:16]
                observations.append({
                    "observation_id": obs_id, "number": n, "fixture_id": row.get("id"), "lane": lane,
                    "name": row.get("name"), "run": directory.name, "recorded_status": row.get("status"),
                    "started_utc": row.get("startedUTC"), "callback_utc": callback.get("timestampISO8601"),
                    "results_receipt": evidence(result_path), "row_index": position, "row_sha256": stable,
                    "runtime_manifest": evidence(manifest_path), "input_manifest_binding": manifest_binding, "manifest_case": case,
                    "expected_at_run": expected, "expected_utf8_sha256": expected_digest,
                    "matches_current_oracle": n in contracts and expected == contracts[n]["expected"],
                    "result_summary": summary_text, "result_shape": shape,
                    "actual_raw_utf8_sha256": actual_digest if raw_digest_present else None,
                    "exact_actual_utf8_text": summary_text if exact_summary_raw else None,
                    "actual_raw_digest_invented": False,
                    "envelope_checks": envelope, "envelope_valid": envelope_valid,
                    "manifest_binding_valid": manifest_valid, "raw_expected_match": raw_match,
                    "bounded_legacy_summary": bounded, "proof_tier": proof,
                    "receiver_image_identity": evidence(image_path) if image_path.exists() else None,
                    "receiver_image_match": image_match,
                    "library_identity_verified": import_receipt.get("libraryIdentityVerified"),
                    "individual_receipt_checks": checks,
                    "completion_receipt": evidence(completion_path) if completion_path.exists() else None,
                    "import_receipt": evidence(import_path) if import_path.exists() else None,
                    "signed_file_verification": signed_validation,
                    "compiler_lineage": selected_lineage,
                    "compiler_lineage_verified": bool(selected_lineage) and all(r["lineage_verified"] for r in selected_lineage),
                    "semantic_status": semantic_status,
                    "historical_record": row,
                })
    observations.sort(key=lambda r: (r.get("started_utc") or "", r.get("callback_utc") or "", r["observation_id"]))
    current = []
    for n, contract in sorted(contracts.items()):
        for lane in LANES:
            history = [r for r in observations if r["number"] == n and r["lane"] == lane]
            applicable = [r for r in history if r["matches_current_oracle"]]
            semantic = [r for r in applicable if r["semantic_status"] in ("PASS", "FAIL_OUTPUT")]
            latest = semantic[-1] if semantic else None
            prior_semantic = [r for r in history if r["semantic_status"] in ("PASS", "FAIL_OUTPUT")]
            if not contract.get("runtime_enabled", True):
                status = "STRUCTURAL_ONLY_NOT_EXECUTED"
            elif latest:
                status = latest["semantic_status"]
            elif (n, lane) in blocked:
                status = "COMPILE_BLOCKED"
            else:
                status = "PENDING_NATIVE_PROOF"
            current.append({
                "number": n, "fixture_id": contract["id"], "lane": lane, "scope": contract["scope"],
                "expected": contract.get("expected"), "oracle": contract.get("oracle"),
                "runtime_enabled": contract.get("runtime_enabled", True), "semantic_result": status,
                "selected_observation_id": latest["observation_id"] if latest else None,
                "selected_proof_tier": latest["proof_tier"] if latest else None,
                "selected_actual_summary": latest["result_summary"] if latest else None,
                "selected_actual_raw_sha256": latest["actual_raw_utf8_sha256"] if latest else None,
                "selected_compiler_source_sha": sorted(set(r["compiler_source_sha"] for r in latest["compiler_lineage"])) if latest else [],
                "selected_unsigned_sha256": (latest["manifest_case"] or {}).get("unsignedSHA256") if latest else None,
                "selected_signed_sha256": (latest["manifest_case"] or {}).get("signedSHA256") if latest else None,
                "selected_lineage_verified": latest["compiler_lineage_verified"] if latest else None,
                "latest_attempt_observation_id": history[-1]["observation_id"] if history else None,
                "all_observation_ids": [r["observation_id"] for r in history],
                "historical_nonpass_observation_ids": [r["observation_id"] for r in history if r["recorded_status"] != "PASS"],
                "old_oracle_observation_ids": [r["observation_id"] for r in history if not r["matches_current_oracle"]],
                "compile_block": blocked.get((n, lane)),
                "final_compiler_binding": "SEPARATE_ARTIFACT_AND_SOURCE_BINDING_REQUIRED" if lane.startswith("fork_") else "ACTUAL_PINNED_ORIGINAL_ARTIFACT",
                "historical_results_restamped": False,
            })
    summary = {}
    for scope in ("MAIN_CORPUS", "SUPPLEMENTAL"):
        summary[scope] = {}
        for lane in LANES:
            rows = [r for r in current if r["scope"] == scope and r["lane"] == lane]
            summary[scope][lane] = {"cases": len(rows), "semantic_results": dict(collections.Counter(r["semantic_result"] for r in rows)),
                "proof_tiers": dict(collections.Counter(r["selected_proof_tier"] for r in rows if r["selected_proof_tier"])),
                "unverified_selected_lineage_numbers": [r["number"] for r in rows if r["selected_observation_id"] and not r["selected_lineage_verified"]]}
    result = {
        "schema_version": 1, "created_utc": datetime.datetime.now(datetime.timezone.utc).isoformat(),
        "lifecycle": config.get("lifecycle", "DRAFT"),
        "generator": evidence(__file__), "config": evidence(args.config),
        "main_manifest": evidence(config["main_manifest"]), "old_manifest": evidence(config["old_manifest"]),
        "legacy_summary_audit": evidence(config["legacy_summary_audit"]),
        "supporting_proofs": [evidence(p) for p in config.get("supporting_proofs", [])],
        "snapshots": snapshots, "compile_receipts": compile_receipts,
        "summary": summary, "current": current, "observations": observations, "runs": run_records,
        "all_historical_recorded_statuses": dict(collections.Counter(r["recorded_status"] for r in observations)),
        "boundaries": [
            "No native operations, signing, compilation, full Go tests, or plist rewriting are performed.",
            "Every historical expected value and verdict remains unchanged. Case35 old-oracle failures are retained.",
            "The current view selects the latest semantic observation under the current expected value; environment/import failures remain in history.",
            "A summary-only PASS is explicitly bounded by the old manifest audit, exact summary, raw byte length, and correlated callback identity. No raw digest is invented.",
            "Receiver image proof is joined only to runs carrying its exact receipt hash; older runs are not restamped.",
            "Compiler lineage uses exact unsigned artifact hashes and actual compiler/source hashes, not lane names or timestamps alone.",
            "Native observations certify the recorded artifacts. A separate final-binding report may carry their proof to byte-identical or strictly alpha-equivalent source-bound programs; it does not claim the new bytes were executed."
        ],
    }
    save(args.out / "native-evidence-ledger.json", result)
    compact = {k: result[k] for k in ("schema_version", "created_utc", "lifecycle", "main_manifest", "summary", "boundaries")}
    compact["full_ledger"] = evidence(args.out / "native-evidence-ledger.json")
    compact["current"] = current
    save(args.out / "native-evidence-compact.json", compact)
    rows_by_number = collections.defaultdict(dict)
    for row in current:
        rows_by_number[row["number"]][row["lane"]] = row
    lines = ["# Native evidence ledger", "", "**" + result["lifecycle"] + "**. Historical native results apply to the exact artifacts recorded below; final compiler binding remains separate.", "",
             "| Case | Current original | Fork Legacy | Fork V2 |", "|---|---|---|---|"]
    for n, rows in sorted(rows_by_number.items()):
        cells = []
        for lane in LANES:
            row = rows[lane]
            label = row["semantic_result"]
            tier = row["selected_proof_tier"]
            if tier == "BOUNDED_SINGLE_LINE_SUMMARY_AND_RAW_LENGTH_NO_DIGEST":
                label += " (bounded summary)"
            elif tier == "RAW_UTF8_DIGEST_RECEIVER_IMAGE_NOT_RECORDED":
                label += " (raw digest)"
            elif tier == "RAW_UTF8_DIGEST_AND_RECEIVER_IMAGE_MATCH":
                label += " (raw digest + image)"
            cells.append(label)
        lines.append("| " + str(n).zfill(2) + " " + contracts[n]["id"] + " | " + " | ".join(cells) + " |")
    lines += ["", "## Evidence boundaries", ""] + ["- " + x for x in result["boundaries"]]
    lines += ["", "Full ledger SHA256: `" + sha(str(args.out / "native-evidence-ledger.json")) + "`.", ""]
    (args.out / "native-evidence-table.md").write_text("\n".join(lines))
    print(json.dumps({"summary": summary, "observations": len(observations), "runs": len(run_records),
        "historical_statuses": result["all_historical_recorded_statuses"],
        "ledger": evidence(args.out / "native-evidence-ledger.json"),
        "compact": evidence(args.out / "native-evidence-compact.json"),
        "table": evidence(args.out / "native-evidence-table.md")}, ensure_ascii=False, indent=2))

if __name__ == "__main__":
    main()
