#!/usr/bin/env python3
"""Bind recorded native evidence to final compiled programs and list exact bundle files.

This tool performs no native runs, signing, compilation, or plist rewriting.
It keeps source differences, typed wire differences, and proof strength explicit.
"""
import argparse
import collections
import datetime
import difflib
import importlib.util
import json
from pathlib import Path
import plistlib

from native_ledger import evidence, load, number, resolve_creation, save, sha, source_file, text_expected


def module_at(path):
    spec = importlib.util.spec_from_file_location("cherri_native_binding_comparator", path)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def source_binding(old_path, new_path, old_name, new_name):
    old, new = Path(old_path).read_bytes(), Path(new_path).read_bytes()
    equal = old == new
    changed_name_only = False
    if not equal and old_name != new_name:
        for old_prefix, new_prefix in (
            (("#define name " + old_name).encode(), ("#define name " + new_name).encode()),
            (('shortcut "' + old_name + '"').encode(), ('shortcut "' + new_name + '"').encode()),
        ):
            lines = old.splitlines(keepends=True)
            matches = [i for i, line in enumerate(lines)
                       if line.startswith(old_prefix)
                       and (len(line) == len(old_prefix) or line[len(old_prefix):len(old_prefix)+1] in (b" ", b"\t", b"\n", b"\r"))]
            if len(matches) == 1:
                i = matches[0]
                lines[i] = new_prefix + lines[i][len(old_prefix):]
                if b"".join(lines) == new:
                    changed_name_only = True
                    break
    return {
        "native_source": evidence(old_path), "final_source": evidence(new_path),
        "native_compiled_name": old_name, "final_compiled_name": new_name,
        "source_bytes_equal": equal, "declared_workflow_name_only_difference": changed_name_only,
        "source_binding_valid": equal or changed_name_only,
        "source_diff": "".join(difflib.unified_diff(
            old.decode("utf-8").splitlines(keepends=True), new.decode("utf-8").splitlines(keepends=True),
            fromfile=str(old_path), tofile=str(new_path))),
    }


def signing_index(snapshots):
    index = collections.defaultdict(list)
    seen = set()
    def visit(value, path, remote, selector):
        if isinstance(value, list):
            for i, item in enumerate(value):
                visit(item, path, remote, selector + [i])
        elif isinstance(value, dict):
            unsigned, signed = value.get("unsignedPlistSha256"), value.get("signedArtifactSha256")
            if unsigned and signed:
                key = (sha(str(path)), json.dumps(selector))
                if key not in seen:
                    seen.add(key)
                    index[unsigned, signed].append({
                        "receipt": evidence(path), "remote_path": remote,
                        "json_selector": selector, "payload": value})
            for k, item in value.items():
                if isinstance(item, (list, dict)):
                    visit(item, path, remote, selector + [k])
    for snapshot in snapshots:
        transfer = load(str(Path(snapshot) / "transfer-index.json"))
        for row in transfer["files"]:
            if row["relative"].startswith("differential/"):
                path = Path(snapshot) / row["relative"]
                assert sha(str(path)) == row["sha256"]
                visit(load(str(path)), path, transfer["remote_root"] + "/" + row["relative"], [])
    return index


def final_compilations(config, comparator):
    index, rows, failures = {}, [], []
    for receipt_path in config["final_compile_receipts"]:
        data = load(receipt_path)
        manifest_path = Path(receipt_path).parent / "manifest.json"
        assert sha(str(manifest_path)) == data["manifest_sha256"]
        assert data["runner_sha256"] == config["comparator_sha256"]
        assert data["fork_sha"] == config["final_fork_sha"]
        for row in data["compilations"]:
            item = {"number": number(row["fixture_id"]), "fixture_id": row["fixture_id"], "lane": row["lane"],
                    "status": row["status"], "receipt": evidence(receipt_path),
                    "manifest": evidence(manifest_path), "expected": row["expected"],
                    "compiled_name": row["shortcut_name"], "runtime_enabled": row["runtime"]["enabled"],
                    "recorded_returncode": row.get("returncode"), "reused_from": row.get("reused_from"),
                    "compiler_flags": row["compiler_flags"], "diagnostics": row.get("diagnostics", [])}
            if row["status"] != "COMPILED":
                item.update(command=row.get("command"), log=evidence(row["log"]))
                failures.append(item)
                continue
            command, code, chain = resolve_creation(receipt_path, row)
            src = source_file(row, receipt_path, command)
            assert src, (row["fixture_id"], row["lane"])
            assert sha(row["artifact"]) == row["artifact_sha256"]
            binary = data["binaries"][row["lane"]]
            assert sha(binary["path"]) == row["binary_sha256"] == binary["sha256"]
            assert command and code == 0
            assert sha(command[1]) == row["source_sha256"]
            assert sha(command[0]) == row["binary_sha256"]
            graph = comparator.graph_audit(plistlib.loads(Path(row["artifact"]).read_bytes()))
            compiler_revision = (
                config["frozen_original_sha"] if row["lane"] == "frozen_original"
                else data["original_sha"] if row["lane"] == "original"
                else data["fork_sha"])
            item.update({
                "artifact": evidence(row["artifact"]), "source": evidence(src),
                "declared_source": row["source"], "creation_source": evidence(command[1]),
                "compiler": evidence(binary["path"]), "compiler_source_sha": compiler_revision,
                "creation_command": command, "creation_exit": code, "receipt_chain": chain,
                "log": evidence(row["log"]), "typed_output": evidence(row["typed_output"]),
                "graph": graph, "all_file_hashes_verified": True,
                "artifact_was_reused": bool(row.get("reused_from")),
            })
            index[item["number"], item["lane"]] = item
            rows.append(item)
    return index, rows, failures


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--ledger", type=Path, required=True)
    parser.add_argument("--config", type=Path, required=True)
    parser.add_argument("--out", type=Path, required=True)
    args = parser.parse_args()
    args.out.mkdir(parents=True, exist_ok=False)
    (args.out / "comparisons").mkdir()
    config, ledger = load(str(args.config)), load(str(args.ledger))
    assert sha(config["comparator_path"]) == config["comparator_sha256"]
    comparator = module_at(config["comparator_path"])
    catalog = {}
    for action in load(config["action_catalog"]).get("actions", []):
        keys = catalog.setdefault(action.get("shortcutIdentifier"), set())
        keys.update(p.get("key", "") for p in action.get("parameters", []) if "default" in p)
        keys.update(action.get("staticParameters", {}).keys())
    final_index, all_compiled, all_failed = final_compilations(config, comparator)
    lineage_path = args.out / "artifact-command-lineage.json"
    save(lineage_path, {
        "schema_version": 1, "final_fork_sha": config["final_fork_sha"],
        "generator": evidence(__file__), "config": evidence(args.config),
        "compiled": all_compiled, "failed": all_failed,
        "summary": {"compiled": len(all_compiled), "failed": len(all_failed),
                    "reused_exact_artifacts": sum(r["artifact_was_reused"] for r in all_compiled),
                    "fresh_artifacts": sum(not r["artifact_was_reused"] for r in all_compiled),
                    "invalid_graphs": [{"number": r["number"], "lane": r["lane"], "errors": r["graph"]["errors"]}
                                       for r in all_compiled if not r["graph"]["valid"]]},
    })
    signatures = signing_index(config["snapshots"])
    remote_files = {}
    for snapshot in config["snapshots"]:
        for filename in ("manifest-artifact-files.json", "unsigned-artifact-file-discovery.json"):
            path = Path(snapshot) / filename
            if path.exists():
                for row in load(str(path))["files"]:
                    remote_files[row["path"]] = dict(row, verification_receipt=evidence(path))
    observations = {r["observation_id"]: r for r in ledger["observations"]}
    bindings, selected, files = [], [], {}

    def add_file(host, path, digest, byte_count, role, n, lane, verification):
        key = host + ":" + path
        if key in files:
            assert files[key]["sha256"] == digest, key
        else:
            files[key] = {"host": host, "path": path, "sha256": digest, "bytes": byte_count,
                          "roles": [], "cases": [], "verification": verification}
        if role not in files[key]["roles"]:
            files[key]["roles"].append(role)
        association = {"number": n, "lane": lane}
        if association not in files[key]["cases"]:
            files[key]["cases"].append(association)
        return key

    def local_file(path, role, n, lane):
        proof = evidence(path)
        return add_file("oracle", proof["path"], proof["sha256"], proof["bytes"], role, n, lane, proof)

    def remote_file(path, expected, role, n, lane):
        proof = remote_files.get(path)
        if not proof or not proof.get("exists") or proof.get("sha256") != expected:
            return None
        return add_file("mac", path, expected, proof["bytes"], role, n, lane, proof["verification_receipt"])

    for current in ledger["current"]:
        n, lane = current["number"], current["lane"]
        binding = {"number": n, "fixture_id": current["fixture_id"], "lane": lane,
                   "scope": current["scope"], "expected": current["expected"],
                   "selected_observation_id": current["selected_observation_id"],
                   "selected_semantic_result": current["semantic_result"],
                   "selected_proof_tier": current["selected_proof_tier"],
                   "final_compiler_source_sha": config["final_fork_sha"] if lane.startswith("fork_") else None,
                   "final_output_exact_bytes_executed": False,
                   "native_evidence_applies_to_final_program": False,
                   "native_operations_performed_by_this_tool": False}
        observation = observations.get(current["selected_observation_id"])
        final = final_index.get((n, lane))
        if not observation or not final:
            binding["status"] = current["semantic_result"]
            binding["reason"] = "No selected semantic observation or no compiled final artifact."
            bindings.append(binding)
            continue
        eligible = [r for r in observation["compiler_lineage"] if r["lineage_verified"]]
        if not eligible:
            binding.update(status="UNBOUND_NATIVE_OBSERVATION", reason="No verified compiler lineage for native bytes.")
            bindings.append(binding)
            continue
        native = eligible[0]
        assert sha(native["artifact"]) == native["artifact_sha256"]
        assert sha(native["verified_source"]) == native["source_sha256"]
        report = comparator.compare(native["artifact"], final["artifact"]["path"], catalog)
        comparison_path = args.out / "comparisons" / (str(n).zfill(2) + "." + lane + ".json")
        save(comparison_path, report)
        source = source_binding(native["verified_source"], final["source"]["path"],
                                native["name"], final["compiled_name"])
        header_diffs = [d for d in report["alpha_differences"] if d["path"][:1] != ["WFWorkflowActions"]]
        allowed_name_diff = bool(header_diffs) and source["declared_workflow_name_only_difference"] and all(
            d["path"] == ["WFWorkflowName"] and d.get("before") == {"type": "text", "value": native["name"]}
            and d.get("after") == {"type": "text", "value": final["compiled_name"]} for d in header_diffs)
        contract_matches = (observation["expected_at_run"] == current["expected"]
                            == text_expected(final["expected"]))
        receipt_checks = observation["individual_receipt_checks"]
        signed_proof = observation.get("signed_file_verification") or {}
        checks = {
            "native_semantic_observation_present": observation["semantic_status"] in ("PASS", "FAIL_OUTPUT"),
            "native_envelope_valid": observation["envelope_valid"],
            "native_manifest_binding_valid": observation["manifest_binding_valid"],
            "native_individual_receipts_match": bool(receipt_checks) and all(receipt_checks.values()),
            "native_signed_bytes_verified": signed_proof.get("exists") is True and signed_proof.get("matches_manifest") is True,
            "native_compiler_lineage_verified": observation["compiler_lineage_verified"],
            "independent_expected_contract_matches": contract_matches,
            "source_bytes_or_declared_name_only": source["source_binding_valid"],
            "graphs_valid": report["graphs_valid"],
            "action_tree_equal_after_uuid_alpha": report["action_tree_equal_after_uuid_alpha"],
            "workflow_metadata_exact_or_declared_name_only": not header_diffs or allowed_name_diff,
            "final_artifact_hash_verified": final["all_file_hashes_verified"],
        }
        bound = all(checks.values())
        binding.update({
            "status": "BOUND_NATIVE_" + observation["semantic_status"] if bound else "UNBOUND_NATIVE_OBSERVATION",
            "native_evidence_applies_to_final_program": bound,
            "final_output_exact_bytes_executed": bound and native["artifact_sha256"] == final["artifact"]["sha256"],
            "observed_compiler_is_final_compiler": native["compiler_sha256"] == final["compiler"]["sha256"],
            "checks": checks, "failed_checks": [k for k, value in checks.items() if not value],
            "source_binding": source, "native_compiler_lineage": native,
            "final_compilation": final, "comparison": evidence(comparison_path),
            "raw_difference_count": report["raw_difference_count"],
            "alpha_difference_count": report["alpha_difference_count"],
            "retained_workflow_metadata_differences": header_diffs,
            "permitted_declared_workflow_name_difference": allowed_name_diff,
            "observed_import_name": observation["name"],
            "native_compiled_name": native["name"], "final_compiled_name": final["compiled_name"],
            "native_compile_expected": native["expected_at_compile"],
            "native_compile_expected_matches_current_oracle": text_expected(native["expected_at_compile"]) == current["expected"],
            "history_preserved_in_ledger": True, "inherited_proof_tier": observation["proof_tier"],
            "interpretation": "The exact recorded semantic result applies to the source-bound typed program. UUID-alpha equality does not claim these newly generated bytes were run." if bound else "No native result is carried forward across an unproven difference.",
        })
        bindings.append(binding)
        manifest_case = observation["manifest_case"]
        package = {"number": n, "fixture_id": current["fixture_id"], "lane": lane,
                   "selected_observation_id": observation["observation_id"], "run": observation["run"],
                   "semantic_result": observation["semantic_status"], "proof_tier": observation["proof_tier"],
                   "compiled_name": native["name"], "import_name": observation["name"],
                   "final_compiled_name": final["compiled_name"],
                   "import_filename_alias": native["name"] != observation["name"],
                   "unsigned_sha256": native["artifact_sha256"], "signed_sha256": manifest_case["signedSHA256"],
                   "native_compiler_source_sha": native["compiler_source_sha"],
                   "final_binding_status": binding["status"], "files": [], "missing": []}
        signed_path = manifest_case.get("signedPath")
        package["signedPath"] = signed_path
        package["unsignedPath_as_recorded_in_runtime_manifest"] = manifest_case.get("unsignedPath")
        unsigned_path = manifest_case.get("unsignedPath")
        discovered_unsigned = [r for r in remote_files.values()
                               if r.get("exists") and r.get("sha256") == native["artifact_sha256"]]
        if not unsigned_path and discovered_unsigned:
            discovered_unsigned.sort(key=lambda r: (Path(r["path"]).name != native["name"] + ".shortcut", r["path"]))
            unsigned_path = discovered_unsigned[0]["path"]
        package["unsignedPath"] = unsigned_path
        package["unsignedPath_resolution"] = ("RECORDED_RUNTIME_MANIFEST" if manifest_case.get("unsignedPath")
                                               else "READ_ONLY_FILE_HASH_MATCH" if unsigned_path else "NOT_LOCATED")
        package["unsigned_path_candidates_by_exact_hash"] = [r["path"] for r in discovered_unsigned]
        if not unsigned_path:
            package["missing"].append({"role": "native_unsigned_artifact_mac", "path": None})
        for role, path, digest in [
            ("native_signed_artifact", signed_path, manifest_case["signedSHA256"]),
            ("native_unsigned_artifact_mac", unsigned_path, native["artifact_sha256"]),
            ("native_signed_original_filename", manifest_case.get("sourceSignedPath"), manifest_case["signedSHA256"]),
        ]:
            if path:
                file_id = remote_file(path, digest, role, n, lane)
                if file_id:
                    package["files"].append(file_id)
                else:
                    package["missing"].append({"role": role, "path": path, "expected_sha256": digest})
        for role, path in [
            ("native_unsigned_artifact_oracle", native["artifact"]), ("native_compiled_source", native["verified_source"]),
            ("final_unsigned_artifact", final["artifact"]["path"]), ("final_compiled_source", final["source"]["path"]),
            ("native_runtime_results", observation["results_receipt"]["path"]),
            ("native_runtime_manifest", observation["runtime_manifest"]["path"]),
            ("native_completion_receipt", (observation.get("completion_receipt") or {}).get("path")),
            ("native_import_receipt", (observation.get("import_receipt") or {}).get("path")),
            ("receiver_image_identity", (observation.get("receiver_image_identity") or {}).get("path")),
        ]:
            if path:
                package["files"].append(local_file(path, role, n, lane))
        for receipt in native["receipt_chain"]:
            package["files"].append(local_file(receipt["path"], "native_compilation_receipt", n, lane))
        signing = signatures.get((native["artifact_sha256"], manifest_case["signedSHA256"]), [])
        claimed = manifest_case.get("signingManifestSHA256")
        exact = [r for r in signing if r["receipt"]["sha256"] == claimed] if claimed else []
        package["signing_receipts"] = exact or signing
        package["claimed_signing_receipt_sha256"] = claimed
        package["claimed_signing_receipt_found"] = bool(exact) if claimed else None
        if not signing:
            package["missing"].append({"role": "signing_receipt", "path": None})
        for receipt in package["signing_receipts"]:
            package["files"].append(local_file(receipt["receipt"]["path"], "signing_receipt", n, lane))
        package["files"] = sorted(set(package["files"]))
        selected.append(package)

    binding_path = args.out / "final-native-bindings.json"
    summary = {
        "selected_semantic_observations": sum(bool(r["selected_observation_id"]) for r in bindings),
        "bound": sum(r["native_evidence_applies_to_final_program"] for r in bindings),
        "by_status": dict(collections.Counter(r["status"] for r in bindings)),
        "exact_native_bytes_among_final_outputs": sum(r["final_output_exact_bytes_executed"] for r in bindings),
        "unbound_selected": [{"number": r["number"], "lane": r["lane"], "failed_checks": r.get("failed_checks", [])}
                            for r in bindings if r["selected_observation_id"] and not r["native_evidence_applies_to_final_program"]],
        "selected_files_missing": [{"number": r["number"], "lane": r["lane"], "missing": r["missing"]}
                                   for r in selected if r["missing"]],
    }
    save(binding_path, {"schema_version": 1, "created_utc": datetime.datetime.now(datetime.timezone.utc).isoformat(),
                        "generator": evidence(__file__), "config": evidence(args.config),
                        "native_ledger": evidence(args.ledger), "comparator": evidence(config["comparator_path"]),
                        "final_fork_sha": config["final_fork_sha"], "artifact_command_lineage": evidence(lineage_path),
                        "summary": summary, "bindings": bindings,
                        "boundaries": [
                            "All raw and alpha wire differences are retained in per-case comparison files.",
                            "Source changes are allowed only when exact bytes differ solely in the declared workflow name.",
                            "Literal keys, defaults, types, display names, property metadata, and action parameters are not normalized.",
                            "The original statistics failure remains a bound FAIL_OUTPUT; graph equality never manufactures PASS.",
                            "Each inherited result keeps its original raw-digest, receiver-image, or bounded-summary proof tier.",
                            "Zero-observation infrastructure runs remain in the native ledger and do not become semantic verdicts.",
                            "This report does not execute final compiler outputs, sign artifacts, or mutate any previous receipt.",
                        ]})
    package_path = args.out / "selected-native-artifact-files.json"
    save(package_path, {"schema_version": 1, "final_fork_sha": config["final_fork_sha"],
                        "native_ledger": evidence(args.ledger), "bindings": evidence(binding_path),
                        "transfer_performed": False, "selected_observations": selected,
                        "files": [files[k] for k in sorted(files)],
                        "summary": {"selected_observations": len(selected), "unique_files": len(files),
                                    "by_host": dict(collections.Counter(v["host"] for v in files.values())),
                                    "missing": summary["selected_files_missing"]}})
    compact_path = args.out / "final-native-evidence-compact.json"
    compact_rows = []
    for row in bindings:
        compact_rows.append({k: row.get(k) for k in (
            "number", "fixture_id", "lane", "scope", "expected", "status", "selected_semantic_result",
            "selected_proof_tier", "selected_observation_id", "native_evidence_applies_to_final_program",
            "final_output_exact_bytes_executed", "native_compiled_name", "observed_import_name",
            "final_compiled_name", "raw_difference_count", "alpha_difference_count",
            "comparison", "failed_checks")})
    save(compact_path, {"schema_version": 1, "final_fork_sha": config["final_fork_sha"],
                        "summary": summary, "bindings": evidence(binding_path),
                        "native_ledger": evidence(args.ledger), "selected_files": evidence(package_path),
                        "current": compact_rows})
    lines = ["# Final native evidence bindings", "",
             "Final compiler source: `" + config["final_fork_sha"] + "`.",
             "Results retain their observed proof tier. Source and typed-action equality carry the result to the final program; this is not a claim of direct execution of every newly generated artifact.", "",
             "| Case | Lane | Result | Proof | Final binding |", "|---|---|---|---|---|"]
    for row in bindings:
        lines.append("| " + str(row["number"]).zfill(2) + " | " + row["lane"] + " | " + row["selected_semantic_result"]
                     + " | " + (row["selected_proof_tier"] or "—") + " | " + row["status"] + " |")
    lines += ["", "Full binding report SHA256: `" + sha(str(binding_path)) + "`.",
              "Artifact file-list SHA256: `" + sha(str(package_path)) + "`.", ""]
    table_path = args.out / "final-native-evidence-table.md"
    table_path.write_text("\n".join(lines))
    print(json.dumps({"summary": summary, "bindings": evidence(binding_path),
                      "lineage": evidence(lineage_path), "files": evidence(package_path),
                      "compact": evidence(compact_path), "table": evidence(table_path)}, ensure_ascii=False, indent=2))
    return int(bool(summary["unbound_selected"] or summary["selected_files_missing"]))


if __name__ == "__main__":
    raise SystemExit(main())
