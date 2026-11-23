"""Strict, source-bound evidence requirements for Remote Builder promotion."""
import hashlib
import json
from pathlib import Path
import re
import math

TUPLE_KEYS = frozenset(("label", "compilerCommit", "docsCommit", "schemaFingerprint",
                        "codecFingerprint", "macroFingerprint", "compilerSHA256",
                        "migrationSHA256", "sourceBranch", "skillSourceHead"))
BASE_CHECKS = (
    "runner_status", "legacy_status", "registered_status", "runner_check",
    "legacy_check", "registered_check", "runner_compile_unsigned",
    "legacy_compile_unsigned", "registered_compile_unsigned", "runner_catalog",
    "runner_actions", "runner_docs", "immutable_sync", "immutable_sync_p15",
    "immutable_sync_p10_compatibility", "skill_full_catalog", "runner_existing_tests",
    "reject_dirty_skill", "reject_dirty_compiler", "reject_dirty_docs",
    "reject_binary_drift", "reject_migration_drift", "reject_toolchain_dependency_drift",
    "reject_implicit_language_switch", "reject_validator_drift_legacy",
    "reject_validator_drift_v2", "reject_registered_runner_drift",
    "restored_registered_status", "restored_runner_status")
FAULT_SURFACES = ("cli_v2", "cli_legacy", "skill_copy", "remote_legacy_copy", "remote_v2_copy")
FAULT_CASES = ("unsigned_control", "xml", "html", "json_error", "non_200", "empty",
               "corrupt_json", "corrupt_base64", "digest_mismatch", "header_only", "zero_archive")
EVIDENCE_KEYS = frozenset(("stage", "entrypoints", "faults", "sourceIntegrity",
                          "runtimeIntegrity", "unpackedIntegrity", "candidateSmoke",
                          "sourceArchive", "runtimeArchive"))
PROTECTED = ("active.json", "bin/cherri-remote-runner.sh", "bin/cherri-remote-legacy.sh",
             "bin/cherri-shortcut-lint.py", "config/default-language-mode")
TOOL_PATHS = frozenset(("tools/go/VERSION", "tools/go/bin/go", "tools/go/pkg/tool/linux_arm64/asm",
                        "tools/go/pkg/tool/linux_arm64/compile", "tools/go/pkg/tool/linux_arm64/link"))


def require(condition, message):
    if not condition:
        raise ValueError(message)


def sha(data):
    return hashlib.sha256(data).hexdigest()


def unique_object(pairs):
    result = {}
    for key, value in pairs:
        require(key not in result, "Duplicate JSON key: " + key)
        result[key] = value
    return result


def read_json(path):
    return json.loads(Path(path).read_bytes(), object_pairs_hook=unique_object)


def is_hash(value, length=64):
    return isinstance(value, str) and re.fullmatch("[0-9a-f]{%d}" % length, value) is not None


def exact_rows(rows, fields, expected, description):
    require(isinstance(rows, list), description + " rows missing")
    keys = [tuple(row.get(field) for field in fields) for row in rows]
    require(len(keys) == len(set(keys)) and set(keys) == set(expected),
            description + " missing, duplicate or unexpected rows")
    return rows


def exact_summary(actual, expected, description):
    require(isinstance(actual, dict) and set(actual) == set(expected)
            and all(type(value) is int for value in actual.values()) and actual == expected,
            "Incomplete or incorrectly typed " + description + " summary")


def validate_tuple(release):
    require(isinstance(release, dict) and set(release) == TUPLE_KEYS, "Incomplete release tuple")
    for key in TUPLE_KEYS - {"label", "sourceBranch"}:
        require(is_hash(release[key], 40 if key.endswith("Commit") or key == "skillSourceHead" else 64),
                "Invalid immutable tuple field: " + key)
    require(re.fullmatch(r"p15-[a-z0-9-]+", release["label"]) is not None, "Invalid release label")
    require(re.fullmatch(r"agent/[A-Za-z0-9/_-]+", release["sourceBranch"]) is not None, "Invalid branch label")


def load_evidence(manifest_path, stage):
    manifest_path, stage = Path(manifest_path).resolve(), Path(stage).resolve()
    manifest = read_json(manifest_path)
    require(set(manifest) == {"formatVersion", "tuple", "files"} and type(manifest["formatVersion"]) is int
            and manifest["formatVersion"] == 1,
            "Invalid evidence manifest")
    validate_tuple(manifest["tuple"])
    require(set(manifest["files"]) == EVIDENCE_KEYS, "Missing or extra evidence input")
    paths, records = {}, {}
    for name, record in manifest["files"].items():
        require(set(record) == {"path", "sha256"} and is_hash(record["sha256"]), "Invalid evidence reference")
        path = (manifest_path.parent / record["path"]).resolve()
        require(path.is_file(), "Missing evidence: " + name)
        raw = path.read_bytes()
        require(sha(raw) == record["sha256"], "Stale evidence: " + name)
        paths[name] = path
        if not name.endswith("Archive"):
            records[name] = json.loads(raw, object_pairs_hook=unique_object)
    require(len(set(paths.values())) == len(paths), "Duplicate evidence files")
    require(paths["stage"] == stage / "staging-receipt.json", "Receipt belongs to another stage")
    require(records["stage"]["tuple"] == manifest["tuple"], "Manifest tuple differs from stage")
    return manifest, paths, records


def validate_reports(records, candidate, skill_manifest, skill_build_hash):
    stage = records["stage"]
    release = stage["tuple"]
    validate_tuple(release)
    require(stage["status"] == "STAGED_NOT_ACTIVATED" and stage["livePromotion"] is False,
            "Stage is not inactive")
    require(Path(stage["candidateHome"]).resolve() == Path(candidate).resolve(), "Stage candidate mismatch")
    require(stage["liveHashesBefore"] == stage["liveHashesAfter"], "Staging modified live")
    require(set(stage["candidateHashes"]) == set(PROTECTED) | TOOL_PATHS, "Incomplete staged protected/toolchain inventory")
    require(set(stage["candidateHashes"]) == set(stage["liveHashesBefore"]), "Protected inventories differ")
    require(set(stage["externalToolchainDependencies"]) == TOOL_PATHS
            and all(is_hash(value) and stage["candidateHashes"][key] == stage["liveHashesBefore"][key] == value
                    for key, value in stage["externalToolchainDependencies"].items()), "External toolchain binding differs")
    require(all(is_hash(value) for key in ("candidateHashes", "liveHashesBefore") for value in stage[key].values()),
            "Invalid staged file hash")
    provenance = stage["binaryProvenance"]
    migration = {"compilerCommit": release["compilerCommit"], "goarch": "arm64", "goos": "linux",
                 "sha256": release["migrationSHA256"], "vcsModified": False,
                 "modulePath": "github.com/electrikmilk/cherri/cmd/cherri-migrate"}
    require(provenance == {"compilerCommit": release["compilerCommit"], "goarch": "arm64", "goos": "linux",
                           "sha256": release["compilerSHA256"], "vcsModified": False,
                           "modulePath": "github.com/electrikmilk/cherri", "migration": migration},
            "Staged runtime pair provenance mismatch")
    require(stage["registeredEntrypoint"] == "bin/cherri-remote-builder -> cherri-remote-legacy.sh",
            "Registered route mismatch")
    for key in ("compilerCommit", "docsCommit", "schemaFingerprint", "codecFingerprint", "macroFingerprint"):
        require(skill_manifest[key] == release[key], "Committed Skill tuple mismatch: " + key)

    entry = records["entrypoints"]
    require(type(entry["formatVersion"]) is int and entry["formatVersion"] == 1, "Unsupported entrypoint receipt version")
    require(entry["scope"] == "INACTIVE_CANDIDATE_REAL_ENTRYPOINTS_AND_CAUSAL_INTEGRITY", "Wrong entrypoint scope")
    require(entry["tuple"] == release and entry["activationGatePassed"] is True and entry["fatalError"] is None,
            "Entrypoint gate is incomplete")
    require(entry["privateCorpusUsed"] is False and entry["livePromotion"] is False, "Entrypoint scope changed")
    for prefix, stage_key in (("live", "liveHashesBefore"), ("candidate", "candidateHashes")):
        require(entry[prefix + "HashesBefore"] == entry[prefix + "HashesAfter"] == stage[stage_key],
                "Entrypoint evidence is stale: " + prefix)
    require(entry["liveUnchanged"] is True and entry["candidateRestored"] is True, "Entrypoint restoration missing")
    expected_checks = {(name,) for name in BASE_CHECKS} | {("reject_tuple_" + key,) for key in TUPLE_KEYS}
    rows = exact_rows(entry["results"], ("check",), expected_checks, "Entrypoint")
    exact_summary(entry["summary"], {"checks": 39, "passed": 39, "failed": 0, "notRun": 0}, "entrypoint")
    for row in rows:
        negative = row["check"].startswith("reject_")
        require(row["verdict"] == "PASS" and type(row["exitCode"]) is int, "Entrypoint did not pass")
        require((row["exitCode"] != 0) if negative else (row["exitCode"] == 0), "Incorrect entrypoint exit")
        require(row["kind"] == ("CAUSAL_REJECTION" if negative else "ENTRYPOINT"), "Incorrect entrypoint kind")
        require(is_hash(row["stdoutSha256"]) and type(row["stdoutBytes"]) is int and row["stdoutBytes"] >= 0
                and type(row["seconds"]) in (float, int) and math.isfinite(row["seconds"]) and row["seconds"] >= 0
                and isinstance(row["diagnosticTail"], str), "Entrypoint execution metadata is absent")
        if negative:
            if row["check"].startswith("reject_tuple_"):
                marker = "IMMUTABLE_RELEASE_MISMATCH"
            elif row["check"].startswith("reject_dirty_"):
                marker = "IMMUTABLE_SOURCE_MISMATCH"
            else:
                marker = {"reject_binary_drift": "IMMUTABLE_COMPILER_MISMATCH",
                          "reject_migration_drift": "IMMUTABLE_MIGRATION_MISMATCH",
                          "reject_toolchain_dependency_drift": "TOOLCHAIN_DEPENDENCY_MISMATCH",
                          "reject_implicit_language_switch": "default language mode is not Legacy",
                          "reject_validator_drift_legacy": "checksum", "reject_validator_drift_v2": "checksum",
                          "reject_registered_runner_drift": "runner checksum changed"}[row["check"]]
            require(marker in row["diagnosticTail"], "Missing causal rejection diagnostic")
        if row["check"].endswith("compile_unsigned"):
            native = row["nativeArtifacts"]
            require(len(native) == 1 and native[0]["syntheticSemanticsMatch"] is True,
                    "Missing actual synthetic native graph")
            require(type(native[0]["actions"]) is int and native[0]["actions"] > 0 and is_hash(native[0]["sha256"]),
                    "Invalid native artifact receipt")
            ids = ["is.workflow.actions.gettext", "is.workflow.actions.showresult"]
            if row["check"] != "runner_compile_unsigned":
                ids.insert(1, "is.workflow.actions.setvariable")
            else:
                # The entrypoint collector records sorted unique native IDs.
                ids.insert(1, "is.workflow.actions.nothing")
            require(native[0]["nativeActionIDs"] == ids and native[0]["actions"] == len(ids), "Unexpected native producer/consumer graph")
        else:
            require(row["nativeArtifacts"] == [], "Unexpected entrypoint artifact")

    fault = records["faults"]
    require(type(fault["formatVersion"]) is int and fault["formatVersion"] == 1, "Unsupported fault receipt version")
    require(fault["scope"] == "ACTUAL_CLI_AND_ENDPOINT_INSTRUMENTED_WRAPPER_COPIES_LOOPBACK_ONLY"
            and fault["appleCryptographicValidation"] == "NOT_TESTED" and fault["privateCorpusUsed"] is False,
            "Fault evidence scope mismatch")
    p = fault["provenance"]
    require(p["installedTuple"] == release and p["compilerSha256"] == release["compilerSHA256"], "Fault tuple/compiler drift")
    require(p["skillManifest"] == skill_manifest and p["skillBuildSha256"] == skill_build_hash, "Fault Skill drift")
    require(p["remoteLegacySha256"] == stage["candidateHashes"][PROTECTED[2]]
            and p["remoteRunnerSha256"] == stage["candidateHashes"][PROTECTED[1]], "Fault wrapper drift")
    for key in ("schemaFingerprint", "codecFingerprint", "macroFingerprint"):
        require(p["capabilities"][key] == release[key], "Fault capability drift")
    expected_faults = {(s, c) for s in FAULT_SURFACES for c in FAULT_CASES}
    expected_faults |= {(s, "timeout") for s in ("cli_v2", "remote_legacy_copy", "remote_v2_copy")}
    rows = exact_rows(fault["results"], ("surface", "case"), expected_faults, "Signing fault")
    exact_summary(fault["summary"], {"checks": 58, "controlsPassed": 5, "negativesRejected": 53, "failures": 0}, "signing")
    remote_requests = 0
    for row in rows:
        require(type(row["exitCode"]) is int and type(row["localRequests"]) is int, "Fault row has no actual process exit/request count")
        if row["case"] == "unsigned_control":
            require(row["verdict"] == "CONTROL_PASS" and row["exitCode"] == 0
                    and row["localRequests"] == 0 and row["requestEvidence"] == [] and len(row["artifacts"]) == 1,
                    "Invalid unsigned positive control")
            control = row["artifacts"][0]
            require(type(control["bytes"]) is int and control["bytes"] > 12 and is_hash(control["sha256"])
                    and control["magicHex"] in ("3c3f786d", "62706c69"), "Control is not an observed unsigned plist")
            continue
        require(row["verdict"] == "REJECTED_NONZERO_NO_ARTIFACT" and row["exitCode"] != 0 and row["artifacts"] == [],
                "Signing negative did not reject without output")
        require(row["localRequests"] == 1 and row["multipartProtocolObserved"] is True
                and len(row["requestEvidence"]) == 1, "Signing request was not observed exactly once")
        request = row["requestEvidence"][0]
        require(request["method"] == "POST" and request["contentType"].startswith("multipart/form-data;")
                and type(request["requestBytes"]) is int and request["requestBytes"] > 0, "Invalid multipart request evidence")
        if row["surface"].startswith("remote_"):
            require(row["exactPreflightBytesObserved"] is True and request["exactPreflightBytes"] is True
                    and is_hash(request["uploadedPlistSha256"])
                    and request["preflightSha256"] == request["uploadedPlistSha256"], "Remote signing did not use exact preflight bytes")
            remote_requests += 1
    require(remote_requests == 22, "Both remote signing routes must have 22 exact requests")

    for name, kind, count, unpacked in (("sourceIntegrity", "source", 23, False),
                                       ("runtimeIntegrity", "runtime", 29, False),
                                       ("unpackedIntegrity", "runtime", 29, True)):
        r = records[name]
        require(r["status"] == "PASS_PACKAGE_INTEGRITY" and r["kind"] == kind
                and type(r["verifiedFiles"]) is int and r["verifiedFiles"] == count
                and r["unpacked"] is unpacked, "Incomplete package integrity: " + name)
        require((r["skillCommit"], r["compilerCommit"], r["docsCommit"]) ==
                (release["skillSourceHead"], release["compilerCommit"], release["docsCommit"]), "Package tuple drift")
        require(is_hash(r["archiveSha256"]), "Missing package hash")
        require(r["runtime"] == (None if kind == "source" else provenance), "Package compiler mismatch")
    require({k: v for k, v in records["runtimeIntegrity"].items() if k != "unpacked"} ==
            {k: v for k, v in records["unpackedIntegrity"].items() if k != "unpacked"}, "Unpacked receipt drift")
    validate_smoke(records["candidateSmoke"], release, candidate, stage["candidateHashes"], default_home=False)
    return release


def validate_smoke(report, release, home, hashes, default_home):
    require(type(report["formatVersion"]) is int and report["formatVersion"] == 1, "Unsupported smoke receipt version")
    require(report["scope"] == "ACTUAL_REMOTE_ROUTES_PUBLIC_SYNTHETIC_SIGNING" and report["tuple"] == release,
            "Smoke scope/tuple mismatch")
    require(Path(report["remoteHome"]).resolve() == Path(home).resolve() and report["usedDefaultHome"] is default_home,
            "Smoke ran through a different home")
    require(report["passed"] is True and report["fatalError"] is None and report["protectedUnchanged"] is True,
            "Smoke is incomplete")
    require(report["externalSigningRequested"] is True and type(report["signingCommandAttempts"]) is int
            and report["signingCommandAttempts"] == 2, "Both real signed smoke commands must execute")
    require(report["protectedModesBefore"] == report["protectedModesAfter"]
            and set(report["protectedModesBefore"]) == set(PROTECTED), "Smoke protected modes changed")
    require(report["registeredLinkBefore"] == report["registeredLinkAfter"]
            and report["registeredLinkBefore"]["target"] == "cherri-remote-legacy.sh", "Smoke registered link changed")
    require(report["appleCryptographicValidation"] == "NOT_TESTED" and report["deviceExecution"] == "NOT_TESTED"
            and report["privateCorpusUsed"] is False, "Smoke overclaims native proof")
    expected_hashes = {p: hashes[p] for p in PROTECTED}
    require(report["protectedHashesBefore"] == report["protectedHashesAfter"] == expected_hashes, "Smoke script drift")
    expected = {(route, op) for route in ("v2", "legacy") for op in ("status", "check", "unsigned", "signed")}
    rows = exact_rows(report["results"], ("route", "operation"), expected, "Smoke")
    exact_summary(report["summary"], {"checks": 8, "passed": 8, "failed": 0}, "smoke")
    for row in rows:
        require(row["verdict"] == "PASS" and type(row["exitCode"]) is int and row["exitCode"] == 0
                and row["sourceUnchanged"] is True, "Smoke operation did not pass")
        require(all(is_hash(row[key]) for key in ("stdoutSha256", "stderrSha256"))
                and all(type(row[key]) is int and row[key] >= 0 for key in ("stdoutBytes", "stderrBytes"))
                and type(row["seconds"]) in (int, float) and math.isfinite(row["seconds"]) and row["seconds"] >= 0
                and isinstance(row["diagnosticTail"], str), "Smoke execution metadata is absent")
        if row["operation"] in ("status", "check"):
            require(row["artifacts"] == [], "Read-only smoke operation emitted an artifact")
        else:
            require(len(row["artifacts"]) == 1 and is_hash(row["artifacts"][0]["sha256"]), "Smoke output missing")
            artifact = row["artifacts"][0]
            if row["operation"] == "unsigned":
                require(artifact["syntheticSemanticsMatch"] is True, "Smoke graph is not the expected synthetic workflow")
                ids = ["is.workflow.actions.gettext", "is.workflow.actions.showresult"]
                if row["route"] == "legacy":
                    ids.insert(1, "is.workflow.actions.setvariable")
                else:
                    # Smoke records actual wire order, including the V2 void tail.
                    ids.append("is.workflow.actions.nothing")
                require(artifact["nativeActionIDs"] == ids and type(artifact["nativeActions"]) is int
                        and artifact["nativeActions"] == len(ids), "Smoke native graph shape differs")
            else:
                require(artifact["aea1EnvelopeObserved"] is True and artifact["magicHex"] == "41454131"
                        and artifact["bytes"] > 12, "Smoke signed envelope missing")
