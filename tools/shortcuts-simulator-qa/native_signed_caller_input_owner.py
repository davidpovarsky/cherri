"""Consume authenticated exact-file signing originals in the unchanged FN04 owner.

The remote signing command is a separate, retained operation. This owner never
repeats it and never substitutes its signer compiler for the packet compiler.
AEA1 container checks do not grant Apple signature or native acceptance credit.
"""
from __future__ import annotations

import argparse
import json
from pathlib import Path, PurePosixPath
import re

import native_caller_input_owner as base

SELECTION_SCHEMA = "cherri-fn04-signed-native-selection/1"
BUNDLE_SCHEMA = "cherri-fn04-exact-file-signing-bundle/1"
SIGNER_SOURCE = "9eed0d556c6b11c78100c99daf8e0a73bb864a26"
SIGNER_SHA256 = "32e87ea2feca9ff9acb4f1e709343d71c2364e746718b59a639fabdab599c95d"
SIGNER_BYTES = 24891121
AUTHORITY_SHA256 = "d84f28a3dad9d2e09fe462a1ce48fa1bf4a73e68dbe7495d946b5306f105b390"
BUILD_INFO_SHA256 = "17e2f84d960903cb0b3641804392187876e60100fc212423b8c7333b0a68dba5"
SIGNING_OWNER_SHA256 = "99e92f53c292271cabadd848d6de12a7243993ba479ffd0abef04fd938c69e9a"
RETRY_OWNER_SHA256 = "3746622de601310721d3c4aea916f3fdc617ec4960b7ccd7cb8a8e7e674145b6"
PREVIOUS_RESULT_SHA256 = "840eb9a4f68b67d4bc5ac126f76413003cd754676a49ff9070a4d343a7aec144"
PREVIOUS_INTAKE_SHA256 = "16b3797037def5451f84cec38b80ba2159a2499873681b1d7be9565a8c5b177c"
PREVIOUS_STDERR_SHA256 = "2ee6fe3339cc5dd071933e210da427d6be94ef36f4492651e3afb5e316cca14d"
SIGNING_SERVER = "https://sign.shortcuty.app/api/v1/sign?response=json"
CASES = ("nested_false", "nested_true", "without_functions")
ROOT_FILES = frozenset(("owner.original.py", "result.original.json",
    "signer-before.original.json", "signer-after.original.json",
    "signer-build-authority.original.json", "signer-build-info.original.txt",
    "packet-intake.original.json"))
RETRY_FILES = frozenset(("previous-result.original.json",
    "previous-failed-signing.stderr.original", "previous-intake.original.json"))
COMMAND_SUFFIXES = ("process.original.json", "start.original.json",
                    "stdout.original", "stderr.original")
CASE_INPUTS = ("packet.derived.json", "source.original.cherri", "unsigned.original.plist")
CASE_OUTPUTS = ("signed.original.shortcut", "provenance.original.json")
MAX_BUNDLE_BYTES = 16 * 1024 * 1024

require = base.require
sha = base.sha
strict = base.library.strict_json


def validate_selection(value, plan):
    require(type(value) is dict and set(value) ==
            {"schema", "planSHA256", "bundleRoot", "bundleIntakeSHA256"}
            and value["schema"] == SELECTION_SCHEMA, "CLOSED_SIGNING_SELECTION")
    require(value["planSHA256"] == sha(base.library.canonical(plan) + b"\n"),
            "EXACT_NATIVE_PLAN_SELECTION")
    require(type(value["bundleIntakeSHA256"]) is str and
            re.fullmatch("[0-9a-f]{64}", value["bundleIntakeSHA256"]) is not None,
            "SIGNING_INTAKE_PIN")
    path = value["bundleRoot"]
    require(type(path) is str and path.startswith("/Users/m1/") and
            len(path.encode()) <= 4096 and "\n" not in path and
            str(PurePosixPath(path)) == path and ".." not in PurePosixPath(path).parts,
            "OWNED_SIGNING_BUNDLE_PATH")
    bundle = Path(path)
    require(all(bundle != Path(plan[key]) and bundle not in Path(plan[key]).parents
                and Path(plan[key]) not in bundle.parents
                for key in ("repo", "root", "buildRoot", "bundleRoot")),
            "SEPARATE_SIGNING_NAMESPACE")
    return value


def relative_name(name):
    require(type(name) is str and 0 < len(name) <= 200 and
            re.fullmatch("[A-Za-z0-9_./-]+", name) is not None and
            not name.startswith("/") and str(PurePosixPath(name)) == name and
            all(p not in (".", "..") for p in PurePosixPath(name).parts),
            "CLOSED_SIGNING_MEMBER")
    return name


def load_bundle(root, expected_sha256):
    root = base.native.launch.owned_root(root)
    intake_raw = base.read(root / "intake.original.json", 128 * 1024)
    require(sha(intake_raw) == expected_sha256, "SIGNING_BUNDLE_PIN")
    intake = strict(intake_raw)
    require(type(intake) is dict and intake.get("schema") == BUNDLE_SCHEMA and
            type(intake.get("files")) is dict and 1 <= len(intake["files"]) <= 64,
            "SIGNING_BUNDLE_SCHEMA")
    files = {}
    total = 0
    for name, pin in intake["files"].items():
        path = root / relative_name(name)
        require(all(not p.is_symlink() for p in path.parents), "SIGNING_DIRECTORY_REDIRECT")
        raw = base.pinned(path, pin, MAX_BUNDLE_BYTES)
        total += len(raw)
        require(total <= MAX_BUNDLE_BYTES, "SIGNING_BUNDLE_BOUND")
        files[name] = raw
    return intake_raw, intake, files


def original_json(files, name):
    require(name in files, "MISSING_SIGNING_ORIGINAL")
    return strict(files[name])


def expected_argv(result, prepared):
    return ["/usr/sbin/runuser", "-u", "chatgpt-agent", "--",
        result["signerBinary"]["path"], "sign", prepared["unsigned"],
        "--output", prepared["signed"], "--signing-server", SIGNING_SERVER,
        "--source-file", prepared["source"], "--compiler-sha", result["compilerCommitSha"],
        "--manifest-out", prepared["manifest"]]


def validate_originals(intake, files, plan, checked):
    """Check original per-case outcomes without promoting the aggregate failure."""
    result = original_json(files, "result.original.json")
    require(result.get("schema") == "cherri-fn04-exact-file-canonical-signing/1" and
            result.get("compilerCommitSha") == plan["compilerCommitSha"] and
            result.get("packetIntakeSHA256") == plan["bundleIntakeSHA256"] and
            sha(files["packet-intake.original.json"]) == plan["bundleIntakeSHA256"],
            "SIGNING_PACKET_COMPILER_BINDING")
    require(intake.get("originalResultSHA256") == sha(files["result.original.json"]) and
            intake.get("originalOverallStatus") == result.get("status") and
            intake.get("compilerCommitSha") == result["compilerCommitSha"] and
            intake.get("packetIntakeSHA256") == result["packetIntakeSHA256"],
            "ORIGINAL_SIGNING_RESULT_JOIN")
    require(result.get("signingServer") == SIGNING_SERVER and
            result.get("signingMethod") == "HUBSIGN_REMOTE_SERVICE" and
            result.get("providerFallback") is False and
            result.get("wholeOwnerSeconds") == 420 and result.get("childSeconds") == 120 and
            result.get("httpSeconds") == 45, "ORIGINAL_EXACT_FILE_SIGNING_ROUTE")
    require(all(result.get(k) is False for k in
                ("compiled", "nativeExecuted", "appleSignatureValidated", "formalAcceptanceGranted"))
            and all(intake.get(k) is False for k in
                ("nativeExecuted", "appleSignatureValidated", "formalAcceptanceGranted")),
            "SIGNING_TIER_OVERCLAIM")
    require(result.get("sourceAndSignerUnchanged") is True and
            "closingError" not in result and intake.get("sourceAndSignerUnchanged") is True and
            intake.get("allParentsReapedAndGroupsAbsent") is True, "SIGNING_CLOSING_PROOF")
    owner_sha = sha(files["owner.original.py"])
    require(owner_sha in (SIGNING_OWNER_SHA256, RETRY_OWNER_SHA256),
            "QUALIFIED_EXACT_FILE_SIGNING_OWNER")
    retry = owner_sha == RETRY_OWNER_SHA256
    attempt_cases = ("without_functions",) if retry else CASES
    if retry:
        require(sha(files["previous-result.original.json"]) ==
                result.get("previousResultSHA256") == intake.get("previousResultSHA256") ==
                PREVIOUS_RESULT_SHA256 and
                sha(files["previous-intake.original.json"]) ==
                result.get("previousIntakeSHA256") == intake.get("previousIntakeSHA256") ==
                PREVIOUS_INTAKE_SHA256 and
                sha(files["previous-failed-signing.stderr.original"]) == PREVIOUS_STDERR_SHA256 and
                result.get("retryCaseId") == "without_functions" and
                result.get("retryReason") == "ORIGINAL_HTTP_502",
                "ONE_COMPANION_RETRY_PRESERVES_ORIGINAL_502")
    authority = original_json(files, "signer-build-authority.original.json")
    require(sha(files["signer-build-authority.original.json"]) == AUTHORITY_SHA256 ==
            result.get("signerBuildAuthoritySHA256") and
            sha(files["signer-build-info.original.txt"]) == BUILD_INFO_SHA256 ==
            result.get("signerBuildInfoSHA256"), "SIGNER_AUTHORITY_ORIGINALS")
    require(authority.get("status") == "PASS_ORIGINAL_PAIR_RESTORED_AND_ACTUAL_CAPABILITIES"
            and authority.get("compilerSource") == SIGNER_SOURCE
            and authority.get("sourceUnchanged") is True
            and authority["products"]["cherri"]["sha256"] == SIGNER_SHA256
            and authority["products"]["cherri"]["bytes"] == SIGNER_BYTES,
            "AUTHENTICATED_SIGNER_BUILD")
    before = original_json(files, "signer-before.original.json")
    after = original_json(files, "signer-after.original.json")
    require(before == after == result.get("signerBinary") and
            before.get("sha256") == SIGNER_SHA256 and before.get("bytes") == SIGNER_BYTES and
            result.get("signerCompilerCommitSha") == intake.get("signerCompilerCommitSha") ==
            SIGNER_SOURCE and intake.get("signerBinarySHA256") == SIGNER_SHA256,
            "SIGNER_DISTINCT_SOURCE_AND_BYTES")
    require(type(before.get("path")) is str and before["path"].startswith(
            "/mnt/development/projects/agent-workspaces/cherri-remote/") and
            Path(before["path"]).name == "cherri" and before.get("uid") == 1002 and
            before.get("mode") == 0o700, "ORIGINAL_SIGNER_EXECUTABLE")
    processes, completed = result.get("processes"), result.get("cases")
    require(type(processes) is list and len(processes) == len(attempt_cases) and
            [p.get("name") for p in processes] == ["sign-" + c for c in attempt_cases]
            and type(completed) is list, "EXACT_DISTINCT_SIGNING_ATTEMPTS")
    require(type(result.get("prepared")) is dict and set(result["prepared"]) == set(attempt_cases),
            "ORIGINAL_SIGNING_CASES")
    successes, failed, owned_ids = [], [], set()
    expected_files = set(ROOT_FILES) | (set(RETRY_FILES) if retry else set())
    for case, process in zip(attempt_cases, processes):
        prefix = "commands/sign-" + case + "."
        expected_files.update(prefix + suffix for suffix in COMMAND_SUFFIXES)
        expected_files.update("cases/" + case + "/" + name for name in CASE_INPUTS)
        prepared = result["prepared"][case]
        require(process.get("argv") == expected_argv(result, prepared) and
                process.get("commandSeconds") == 120 and process.get("wholeOwnerSeconds") == 420,
                "EXACT_SIGNER_ARGV_AND_BOUNDS")
        start = original_json(files, prefix + "start.original.json")
        require(start.get("ownershipVerified") is True and
                all(start.get(k) == process.get(k) for k in
                    ("argv", "name", "pid", "pgid", "sid", "startedAt", "commandSeconds", "wholeOwnerSeconds")),
                "OWNED_SIGNER_PROCESS_START")
        require(original_json(files, prefix + "process.original.json") == process and
                type(process.get("pid")) is int and process["pid"] > 1 and
                process["pid"] == process.get("pgid") == process.get("sid") and
                process["pid"] not in owned_ids, "DISTINCT_ORIGINAL_SIGNER_PROCESS")
        owned_ids.add(process["pid"])
        cleanup = process.get("cleanup", {})
        require(cleanup.get("parentReaped") is True and
                cleanup.get("after", {}).get("state") == "ABSENT" and
                cleanup.get("errors") == [], "SIGNER_PROCESS_CLEANUP")
        for key in ("cancelled", "timedOut", "wholeDeadlineExceeded"):
            require(process.get(key) is False, "SIGNER_PROCESS_INTERRUPTED")
        for side in ("stdout", "stderr"):
            stream = files[prefix + side + ".original"]
            pin = process.get(side, {})
            require(pin.get("complete") is True and pin.get("bytes") == len(stream) and
                    pin.get("sha256") == sha(stream), "COMPLETE_ORIGINAL_SIGNER_STREAM")
        require(type(process.get("exitCode")) is int and process.get("error") is None,
                "OBSERVED_SIGNER_EXIT")
        if process.get("status") == "PASS" and process["exitCode"] == 0:
            require(not failed, "NO_SIGNING_AFTER_FAILURE")
            successes.append(case)
            expected_files.update("cases/" + case + "/" + name for name in CASE_OUTPUTS)
        else:
            require(not retry and case == CASES[-1] and process.get("status") == "EXIT_NONZERO"
                    and process["exitCode"] != 0, "ORIGINAL_SIGNER_FAILURE_BOUNDARY")
            failed.append(case)
    require(set(files) == expected_files and
            [c.get("caseId") for c in completed] == successes and
            intake.get("successfulCases") == successes and intake.get("failedCases") == failed,
            "EXACT_SIGNING_FILE_AND_RESULT_INVENTORY")
    if failed:
        require(result["status"] == "FAIL" and
                result.get("error", {}).get("type") == "OwnerAbort" and
                result["error"].get("message") == "PROCESS_FAILED:sign-" + failed[0] and
                intake.get("status") == "VALIDATED_ORIGINALS_TWO_SIGNED_ONE_SIGNER_FAILURE",
                "PRESERVE_AGGREGATE_SIGNING_FAILURE")
    else:
        expected_status = ("PASS_ONE_EXACT_UNSIGNED_PACKET_SIGNED_STRUCTURAL_ONLY" if retry else
                           "PASS_THREE_EXACT_UNSIGNED_PACKETS_SIGNED_STRUCTURAL_ONLY")
        require(result["status"] == expected_status and "error" not in result,
                "COMPLETE_SIGNING_OUTCOME")
        if retry:
            require(intake.get("status") == "VALIDATED_ORIGINALS_ONE_COMPANION_RETRY_SIGNED",
                    "SEPARATE_COMPANION_RETRY_INTAKE")
    require(plan["caseId"] in successes and checked["caseId"] == plan["caseId"],
            "SELECTED_PACKET_NOT_SIGNED")
    case = plan["caseId"]
    row = next(c for c in completed if c["caseId"] == case)
    prepared = result["prepared"][case]
    prefix = "cases/" + case + "/"
    source, unsigned, signed = (files[prefix + name] for name in
        ("source.original.cherri", "unsigned.original.plist", "signed.original.shortcut"))
    require(source == checked["source"] and unsigned == checked["unsigned"] and
            prepared["name"] == row.get("workflowName") == checked["name"] and
            prepared["sourcePin"] == {"bytes": len(source), "sha256": sha(source)} and
            prepared["unsignedPin"] == {"bytes": len(unsigned), "sha256": sha(unsigned)},
            "EXACT_SIGNED_PACKET_INPUTS")
    packet = original_json(files, prefix + "packet.derived.json")
    require(packet.get("workflowName") == checked["name"] and packet.get("caseId") == case and
            packet.get("sourceSha256") == sha(source) and packet.get("unsignedSha256") == sha(unsigned)
            and packet.get("actionCount") == prepared["actions"], "ORIGINAL_SIGNED_PACKET_GRAPH")
    manifest_raw = files[prefix + "provenance.original.json"]
    manifest = strict(manifest_raw)
    expected = {"schemaVersion": "1.4", "compilerCommitSha": plan["compilerCommitSha"],
        "signingMethod": "HUBSIGN_REMOTE_SERVICE", "serviceName": "Custom Signing Server",
        "shortcutName": checked["name"], "sourceFile": Path(prepared["source"]).name,
        "sourceSha256": sha(source), "unsignedFile": Path(prepared["unsigned"]).name,
        "unsignedPlistSha256": sha(unsigned), "unsignedByteSize": len(unsigned),
        "nativeActionCount": prepared["actions"], "signedFile": Path(prepared["signed"]).name,
        "signedArtifactSha256": sha(signed), "signedByteSize": len(signed),
        "headerMagic": "AEA1", "verificationTier": "AEA1_HEADER_PASS"}
    require(manifest == expected and row.get("manifest") ==
            {"bytes": len(manifest_raw), "sha256": sha(manifest_raw)} and
            row.get("signed") == {"bytes": len(signed), "sha256": sha(signed)} and
            row.get("verificationTier") == "AEA1_HEADER_PASS", "EXACT_SIGNING_MANIFEST")
    require(len(signed) >= 16 and signed.startswith(b"AEA1") and any(signed[4:]),
            "SIGNED_CONTAINER_STRUCTURE_ONLY")
    return signed, {"schema": "cherri-fn04-bound-exact-file-signature/1",
        "caseId": case, "workflowName": checked["name"], "compilerCommitSha": plan["compilerCommitSha"],
        "signerCompilerCommitSha": SIGNER_SOURCE, "signerBinarySHA256": SIGNER_SHA256,
        "originalOverallStatus": result["status"], "successfulCases": successes, "failedCases": failed,
        "previousResultSHA256": PREVIOUS_RESULT_SHA256 if retry else None,
        "unsignedSHA256": sha(unsigned), "signed": row["signed"],
        "verificationTier": "AEA1_HEADER_PASS", "nativeExecuted": False,
        "appleSignatureValidated": False, "formalAcceptanceGranted": False}


def bind_signed_bundle(selection, plan, checked):
    intake_raw, intake, files = load_bundle(selection["bundleRoot"], selection["bundleIntakeSHA256"])
    signed, receipt = validate_originals(intake, files, plan, checked)
    return signed, receipt, intake_raw, intake, files


class SignedNativeCallerObservation(base.NativeCallerObservation):
    """Only the signature acquisition step differs from the qualified owner."""
    def __init__(self, plan, selection):
        self.signing_selection = validate_selection(selection, plan)
        super().__init__(plan)
        base.record(self.root / "signing-selection.original.json", self.signing_selection)

    def sign_original(self):
        self.packet, self.canonical_fixture, self.checked, evidence = base.bind_packet_bundle(self.plan)
        base.record(self.folder / "packet-binding.original.json", evidence)
        signed_raw, receipt, intake_raw, intake, files = bind_signed_bundle(
            self.signing_selection, self.plan, self.checked)
        original_root = self.folder / "exact-file-signing-originals"
        original_root.mkdir(mode=0o700)
        base.original(original_root / "intake.original.json", intake_raw)
        for name, raw in files.items():
            path = original_root / relative_name(name)
            path.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
            base.original(path, raw)
        # Bind the retained copies immediately; the final native result retains
        # every pin, allowing its independent intake to repeat this byte join.
        retained = load_bundle(original_root, sha(intake_raw))
        require(retained[0] == intake_raw and retained[2] == files, "RETAINED_SIGNING_ORIGINALS")
        signed_root = self.folder / "signed"
        signed_root.mkdir(mode=0o700)
        unsigned = signed_root / (self.checked["name"] + ".unsigned.shortcut")
        signed = signed_root / (self.checked["name"] + ".shortcut")
        base.original(unsigned, self.checked["unsigned"])
        self.signed_pin = base.original(signed, signed_raw)
        self.unsigned, self.signed = unsigned, signed
        receipt.update(method="EXISTING_CANONICAL_EXACT_FILE_SIGNING_CLI",
            packetToken=self.checked["token"], signingIntakeSHA256=sha(intake_raw),
            compilePerformed=False, signingRepeated=False)
        base.record(self.folder / "signing.original.json", receipt)
        self.summary["signingOriginals"] = {"intake": {"bytes": len(intake_raw),
            "sha256": sha(intake_raw)}, "files": intake["files"], "binding": receipt}


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--plan", required=True)
    parser.add_argument("--signing-selection", required=True)
    args = parser.parse_args(argv)
    plan = base.validate_plan(strict(base.read(Path(args.plan))))
    selection = validate_selection(strict(base.read(Path(args.signing_selection))), plan)
    owner = SignedNativeCallerObservation(plan, selection)
    result = owner.run()
    print(json.dumps({"status": result["status"], "runId": plan["runId"],
        "caseId": plan["caseId"], "runtimeObservationAuthenticated":
        result["runtimeObservationAuthenticated"], "formalAcceptanceGranted": False}))
    return 0 if result["status"] == "OBSERVATION_VERIFIED" else 1


if __name__ == "__main__":
    raise SystemExit(main())
