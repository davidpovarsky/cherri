"""One separately bound materialized-Text clipboard diagnostic.

This does not replace an original FN04 packet, exporter, oracle or acceptance
record. It reuses the existing supported signed-import/XCTest transport and
keeps the original packet compiler, the new diagnostic compiler, and the
existing compiled harness identities distinct. No signing occurs on the Mac.
"""
from __future__ import annotations

import argparse
import base64
import json
from pathlib import Path, PurePosixPath
import plistlib
import re
import signal
import sys
import time
import urllib.parse
import uuid

import native_caller_input_owner as base
import native_caller_input_observation as caller
import native_signed_caller_input_owner as signed_owner

native = base.native
library = base.library
source_binding = base.source_binding
defer_cancellation_signals = base.defer_cancellation_signals
fixtures = caller.fixtures
differential = caller.differential
require = base.require
sha = base.sha
read = base.read
original = base.original
record = base.record
pinned = base.pinned
LIVE_FLOOR_BYTES = base.LIVE_FLOOR_BYTES
CRITICAL = base.CRITICAL + ("native_materialized_caller_diagnostic.py",)

SCHEMA = "cherri-fn04-materialized-text-native-diagnostic/1"
SELECTION_SCHEMA = "cherri-fn04-materialized-text-selection/1"
BUNDLE_SCHEMA = "cherri-fn04-materialized-text-original-bundle/1"
COMPILER = "a7631a04b36e3a4ed0dffbb919d6508252024365"
COMPILER_TREE = "995c36967cfd881363cd0eb08cf33051ec831a4c"
COMPILER_BINARY_SHA256 = "eb79fb4145ed3927dc55f7fe623bc78a484ed35d2390d43ceed1b3777468c4fa"
BUILD_EQUIVALENT_SOURCE = "5b689c938c73420ef36b3f077764d30f8946f31a"
BUILT_HARNESS = "bff591477f7fc34d79f4a9a2235d087788c169da"
BUILD_RESULT_SHA256 = "d60b66c736f8760ff874a2e8fee82906adbb2c2a3903f54762fb4d3439424a07"
PACKET_SHA256 = "32c6ded2f5a87f0b368323d968fdfec69908ee8aa2909892dc3d05f4e1374110"
PACKET_CANONICAL_SHA256 = "e7e8dad54d1f114a1fc083c7a009454f674b654cb7176a0accf14924a7dfccf0"
COMPILE_RESULT_SHA256 = "0c87c40a71a2af7a1f7d71bddb92732a129ce300a7563c24718f68e464a1065e"
COMPILE_OWNER_SHA256 = "4d4d41c1e39760851d3b3cac8dad5e424974181b9c48dfff588b85f60426a5a3"
CAPTURE_SHA256 = "4672474ddb270a5857914041ebb279f20431f87e9da173e6cd8bf83b3fb11c85"
COMPLETE_GRAPH_SHA256 = "3457402e390f7c8590560508152ac61e7f308a3ac12bf12b0a74a09cd5ed0227"
SIGNING_OWNER_SHA256 = "298f1edc646dfc10080f8b0378589897bb985d22a5f30842adf822d008441921"
SIGNING_RESULT_SHA256 = "bc59c1d047666b947e58fba2c9126346412f48ab45c24d062f1572fa64a7d980"
NAME = "CherriFN04Materialized_9f4ac00a6d8c4acba51a9f37a9ce232d"
TOKEN = "9f4ac00a6d8c4acba51a9f37a9ce232d"
DIAGNOSTIC_CASE = "without_functions_materialized_text"
SOURCE_SHA256 = "c210761621b475cfb62d17eaa443b906d10e7e9d2b4fe39e82fa4853886c1c1c"
UNSIGNED_SHA256 = "75db5a42f6e3c1e9a6d566e278992c8900a741b2f97cf2a2c6eac61bde32b970"
SOURCE = (b"let first=getVariable(system.input)\n"
          b"let second=getVariable(ShortcutInput)\n"
          b'let clipboardText=f"{first}|{second}"\n'
          b"setClipboard(clipboardText)")


def validate_selection(value, plan):
    require(type(value) is dict and set(value) == {"schema", "planSHA256", "bundleRoot", "bundleIntakeSHA256"}
            and value.get("schema") == SELECTION_SCHEMA,
            "DIAGNOSTIC_SELECTION_SCHEMA")
    require(value["planSHA256"] == sha(library.canonical(plan) + b"\n") and
            type(value["bundleIntakeSHA256"]) is str and
            re.fullmatch("[0-9a-f]{64}", value["bundleIntakeSHA256"]) is not None,
            "EXACT_DIAGNOSTIC_PLAN_SELECTION")
    path = value["bundleRoot"]
    require(type(path) is str and path.startswith("/Users/m1/") and len(path.encode()) <= 4096 and
            "\n" not in path and str(PurePosixPath(path)) == path and ".." not in PurePosixPath(path).parts,
            "OWNED_DIAGNOSTIC_BUNDLE_PATH")
    bundle = Path(path)
    require(all(bundle != Path(plan[key]) and bundle not in Path(plan[key]).parents and
                Path(plan[key]) not in bundle.parents for key in ("repo", "root", "buildRoot")),
            "SEPARATE_DIAGNOSTIC_NAMESPACE")
    require(plan["compilerCommitSha"] == COMPILER and
            plan["builtHarnessCommitSha"] == BUILT_HARNESS and
            plan["buildResultSHA256"] == BUILD_RESULT_SHA256 and
            plan["caseId"] == "without_functions" and
            plan["bundleRoot"] == value["bundleRoot"] and
            plan["bundleIntakeSHA256"] == value["bundleIntakeSHA256"],
            "DIAGNOSTIC_PLAN_SOURCE_BINDING")
    return value


def action(identifier, parameters=None):
    value = {"WFWorkflowActionIdentifier": "is.workflow.actions." + identifier}
    if parameters is not None:
        value["WFWorkflowActionParameters"] = parameters
    return value


def attachment(value):
    return {"WFSerializationType": "WFTextTokenAttachment", "Value": value}


def output_ref(output_uuid, name):
    return {"Type": "ActionOutput", "OutputUUID": output_uuid, "OutputName": name}


def validate_complete_graph(raw):
    require(type(raw) is bytes and 0 < len(raw) <= 1024 * 1024, "DIAGNOSTIC_PLIST_BOUND")
    graph = plistlib.loads(raw)
    require(type(graph) is dict and type(graph.get("WFWorkflowActions")) is list,
            "DIAGNOSTIC_ACTION_ARRAY")
    actions = graph["WFWorkflowActions"]
    require(len(actions) == 5 and all(type(row) is dict for row in actions),
            "EXACT_FIVE_DIAGNOSTIC_ACTIONS")
    uuids = []
    for index, name in enumerate(("first", "second", "clipboardText")):
        parameters = actions[index].get("WFWorkflowActionParameters")
        require(type(parameters) is dict and parameters.get("CustomOutputName") == name
                and type(parameters.get("UUID")) is str, "DIAGNOSTIC_NAMED_OUTPUT")
        value = parameters["UUID"]
        require(str(uuid.UUID(value)).upper() == value.upper(), "DIAGNOSTIC_VALID_UUID")
        uuids.append(value)
    require(len(set(uuids)) == 3, "DIAGNOSTIC_UNIQUE_UUIDS")
    text = {"WFSerializationType": "WFTextTokenString", "Value": {
        "string": "\ufffc|\ufffc", "attachmentsByRange": {
            "{0, 1}": output_ref(uuids[0], "first"),
            "{2, 1}": output_ref(uuids[1], "second")}}}
    expected = [
        action("getvariable", {"UUID": uuids[0], "CustomOutputName": "first",
                              "WFVariable": attachment({"Type": "ExtensionInput"})}),
        action("getvariable", {"UUID": uuids[1], "CustomOutputName": "second",
                              "WFVariable": attachment({"Type": "ExtensionInput"})}),
        action("gettext", {"UUID": uuids[2], "CustomOutputName": "clipboardText",
                           "WFTextActionText": text}),
        action("setclipboard", {"WFInput": attachment(output_ref(uuids[2], "clipboardText"))}),
        action("nothing"),
    ]
    require(actions == expected, "COMPLETE_DIAGNOSTIC_ACTION_GRAPH")
    require(graph.get("WFWorkflowHasShortcutInputVariables") is True and
            graph.get("WFWorkflowHasOutputFallback") is False,
            "DIAGNOSTIC_INPUT_AND_FALLTHROUGH")
    require("WFWorkflowName" not in graph, "PRESERVED_ABSENT_TOP_WORKFLOW_NAME")
    return graph


def validate_packet(value, canonical_fixture):
    require(canonical_fixture is None and type(value) is dict,
            "DIAGNOSTIC_IS_NOT_CANONICAL_FN04")
    encoded = json.dumps(value, sort_keys=True, ensure_ascii=True, separators=(",", ":")).encode()
    require(sha(encoded) == PACKET_CANONICAL_SHA256 and
            value["schema"] == "cherri-fn04-materialized-text-diagnostic-packet/1" and
            value["caseId"] == DIAGNOSTIC_CASE and value["diagnosticOnly"] is True,
            "EXACT_DIAGNOSTIC_PACKET")
    require(value["compilerSource"] == COMPILER and value["compilerTree"] == COMPILER_TREE and
            value["compilerBinarySHA256"] == COMPILER_BINARY_SHA256 and
            value["compilerBuildRunId"] == 46012790470 and
            value["priorCompilerSource"] == BUILD_EQUIVALENT_SOURCE,
            "DIAGNOSTIC_COMPILER_LINEAGE")
    require(all(value[key] is False for key in
                ("nativeExecuted", "signed", "formalAcceptanceGranted", "originalAcceptanceReplacement")),
            "DIAGNOSTIC_PACKET_OVERCLAIM")
    source = fixtures._decoded(value["sourceBase64"], value["sourceBytes"], value["sourceSha256"], 1024)
    unsigned = fixtures._decoded(value["unsignedBase64"], value["unsignedBytes"], value["unsignedSha256"], 1024 * 1024)
    require(source == SOURCE and len(source) == 139 and sha(source) == SOURCE_SHA256 and
            len(unsigned) == 4972 and sha(unsigned) == UNSIGNED_SHA256 and
            value["workflowName"] == NAME and value["compileFilename"] == NAME + ".cherri" and
            value["actionCount"] == 5 and value["topLevelWorkflowNamePresent"] is False and
            value["expectedClipboardExpression"] == "inputText + '|' + inputText" and
            value["expectedClipboardBytesForFreshProtocolInput"] == 93,
            "DIAGNOSTIC_EXACT_SOURCE_AND_VALUE_CONTRACT")
    validate_complete_graph(unsigned)
    # The existing without-functions transport oracle remains the exact
    # nonce + separator + nonce byte predicate. This is only its protocol role.
    return {"caseId": "without_functions", "diagnosticCaseId": DIAGNOSTIC_CASE,
            "name": NAME, "token": TOKEN, "source": source, "unsigned": unsigned,
            "sourceSha256": SOURCE_SHA256, "unsignedSha256": UNSIGNED_SHA256}


def load_bundle(root, expected_sha256):
    root = native.launch.owned_root(root)
    raw = read(root / "intake.original.json", 128 * 1024)
    require(sha(raw) == expected_sha256, "DIAGNOSTIC_BUNDLE_PIN")
    intake = library.strict_json(raw)
    require(type(intake) is dict and intake.get("schema") == BUNDLE_SCHEMA and
            intake.get("diagnosticOnly") is True and intake.get("formalAcceptanceGranted") is False and
            type(intake.get("files")) is dict and 1 <= len(intake["files"]) <= 96,
            "CLOSED_DIAGNOSTIC_BUNDLE")
    files, total = {}, 0
    for name, pin in intake["files"].items():
        path = root / signed_owner.relative_name(name)
        require(all(not parent.is_symlink() for parent in path.parents), "DIAGNOSTIC_DIRECTORY_REDIRECT")
        content = pinned(path, pin, signed_owner.MAX_BUNDLE_BYTES)
        total += len(content)
        require(total <= signed_owner.MAX_BUNDLE_BYTES, "DIAGNOSTIC_BUNDLE_BOUND")
        files[name] = content
    return raw, intake, files


def pinned_member(files, name, expected):
    require(name in files and sha(files[name]) == expected, "DIAGNOSTIC_ORIGINAL_PIN:" + name)
    return files[name]


def json_member(files, name):
    require(name in files, "DIAGNOSTIC_ORIGINAL_MISSING:" + name)
    return library.strict_json(files[name])


def validate_process(files, prefix, process, argv, seconds, whole):
    require(type(process) is dict and process.get("argv") == argv and
            process.get("commandSeconds") == seconds and process.get("wholeOwnerSeconds") == whole and
            process.get("status") == "PASS" and process.get("exitCode") == 0 and
            process.get("error") is None and
            all(process.get(key) is False for key in ("cancelled", "timedOut", "wholeDeadlineExceeded")),
            "AUTHENTIC_DIAGNOSTIC_PROCESS")
    start = json_member(files, prefix + "start.original.json")
    require(start.get("ownershipVerified") is True and
            all(start.get(key) == process.get(key) for key in
                ("argv", "name", "pid", "pgid", "sid", "startedAt", "commandSeconds", "wholeOwnerSeconds")),
            "DIAGNOSTIC_PROCESS_START")
    require(type(process.get("pid")) is int and process["pid"] > 1 and
            process["pid"] == process.get("pgid") == process.get("sid") and
            json_member(files, prefix + "process.original.json") == process,
            "DIAGNOSTIC_PROCESS_IDENTITY")
    cleanup = process.get("cleanup", {})
    require(cleanup.get("parentReaped") is True and cleanup.get("after", {}).get("state") == "ABSENT"
            and cleanup.get("errors") == [], "DIAGNOSTIC_PROCESS_CLOSURE")
    for side in ("stdout", "stderr"):
        stream = files[prefix + side + ".original"]
        pin = process.get(side, {})
        require(pin.get("complete") is True and pin.get("bytes") == len(stream) and
                pin.get("sha256") == sha(stream), "DIAGNOSTIC_COMPLETE_PROCESS_STREAM")


def validate_originals(intake, files, plan):
    packet_raw = pinned_member(files, "diagnostic/packet.derived.json", PACKET_SHA256)
    packet = library.strict_json(packet_raw)
    checked = validate_packet(packet, None)
    require(files["diagnostic/source.original.cherri"] == checked["source"] and
            files["diagnostic/unsigned.original.shortcut"] == checked["unsigned"],
            "DIAGNOSTIC_DUPLICATE_INPUTS_EXACT")
    compile_raw = pinned_member(files, "compile/result.original.json", COMPILE_RESULT_SHA256)
    compiled = library.strict_json(compile_raw)
    pinned_member(files, "compile/owner.original.py", COMPILE_OWNER_SHA256)
    pinned_member(files, "compile/capture.original.py", CAPTURE_SHA256)
    pinned_member(files, "compile/complete-graph.derived.json", COMPLETE_GRAPH_SHA256)
    require(compiled.get("status") == "PASS_ONE_MATERIALIZED_TEXT_DIAGNOSTIC_COMPILED_AND_GRAPH_BOUND" and
            compiled.get("compilerSource") == COMPILER and compiled.get("compilerTree") == COMPILER_TREE and
            compiled.get("compilerBinarySHA256") == COMPILER_BINARY_SHA256 and
            compiled.get("compilerInvocations") == 1 and compiled.get("sourceAndBinaryUnchanged") is True and
            compiled.get("allOriginalInputsUnchanged") is True and
            compiled.get("wholeSeconds") == 180 and compiled.get("childSeconds") == 45 and
            compiled.get("cleanupSeconds") == 5 and compiled.get("finalReserveSeconds") == 20 and
            compiled.get("diagnosticOnly") is True and
            all(compiled.get(key) is False for key in
                ("nativeExecuted", "signed", "formalAcceptanceGranted", "originalAcceptanceReplacement", "newGoBuild")),
            "GENUINE_DIAGNOSTIC_COMPILE")
    compile_before = json_member(files, "compile/before.original.json")
    require(json_member(files, "compile/after.original.json") == compile_before and
            compile_before["binary"]["sha256"] == COMPILER_BINARY_SHA256 and
            compile_before["source"]["sha256"] == SOURCE_SHA256,
            "DIAGNOSTIC_COMPILE_SOURCE_CLOSURE")
    intent = json_member(files, "compile/intent.original.json")
    expected_compile = ["/usr/sbin/runuser", "-u", "ubuntu", "--", compile_before["binary"]["path"],
        "compile", packet["sourcePath"], "--mode=v2", "--skip-sign", "--build-profile=debug", "--output", packet["unsignedPath"]]
    require(intent["argv"] == expected_compile and len(compiled["processes"]) == 1,
            "ONE_AUTHENTIC_DIAGNOSTIC_COMPILE")
    validate_process(files, "compile/commands/compile-materialized-text.", compiled["processes"][0], expected_compile, 45, 180)
    handoff_raw = pinned_member(files, "compile/compiler-handoff.original.json", "b63207f73e9d6aa091c43933ad1d7457b5c1cd7552173215e69d078bd2859b2d")
    restoration_raw = pinned_member(files, "compile/compiler-restoration.original.json", "dec4ac18477456fdc3cc9a6593b9387910dc1c5ef47002c90f59e6c95040bb0f")
    info = pinned_member(files, "compile/compiler-build-info.original.txt", "ef97383f2af8d86753beb128d75eabd38894e3da12752619fc21c897e2834f1b")
    require(library.strict_json(handoff_raw)["source"] == library.strict_json(restoration_raw)["source"] == COMPILER and
            b"vcs.revision=" + COMPILER.encode() in info and b"vcs.modified=false" in info,
            "DIAGNOSTIC_EXECUTABLE_SOURCE")
    sign_raw = pinned_member(files, "signing/result.original.json", SIGNING_RESULT_SHA256)
    result = library.strict_json(sign_raw)
    pinned_member(files, "signing/owner.original.py", SIGNING_OWNER_SHA256)
    require(result.get("schema") == "cherri-fn04-materialized-text-exact-signing/1" and
            result.get("status") == "PASS_ONE_DIAGNOSTIC_UNSIGNED_PACKET_SIGNED_STRUCTURAL_ONLY" and
            result.get("compilerCommitSha") == plan["compilerCommitSha"] == COMPILER and
            result.get("packetIntakeSHA256") == COMPILE_RESULT_SHA256 and
            files["signing/packet-intake.original.json"] == compile_raw and
            result.get("sourceAndSignerUnchanged") is True and "closingError" not in result and "error" not in result and
            result.get("diagnosticOnly") is True and
            all(result.get(key) is False for key in
                ("compiled", "nativeExecuted", "appleSignatureValidated", "formalAcceptanceGranted", "originalAcceptanceReplacement", "providerFallback")),
            "GENUINE_DIAGNOSTIC_SIGNATURE")
    require(result.get("signingServer") == signed_owner.SIGNING_SERVER and
            result.get("signingMethod") == "HUBSIGN_REMOTE_SERVICE" and
            result.get("wholeOwnerSeconds") == 420 and result.get("childSeconds") == 120 and result.get("httpSeconds") == 45,
            "UNCHANGED_EXACT_FILE_SIGNING_ROUTE")
    authority_raw = pinned_member(files, "signing/signer-build-authority.original.json", signed_owner.AUTHORITY_SHA256)
    authority = library.strict_json(authority_raw)
    signer_info = pinned_member(files, "signing/signer-build-info.original.txt", signed_owner.BUILD_INFO_SHA256)
    before = json_member(files, "signing/signer-before.original.json")
    require(before == json_member(files, "signing/signer-after.original.json") == result["signerBinary"] and
            before["sha256"] == signed_owner.SIGNER_SHA256 and before["bytes"] == signed_owner.SIGNER_BYTES and
            before["uid"] == 1002 and before["mode"] == 0o700 and
            result["signerCompilerCommitSha"] == authority["compilerSource"] == signed_owner.SIGNER_SOURCE and
            authority["status"] == "PASS_ORIGINAL_PAIR_RESTORED_AND_ACTUAL_CAPABILITIES" and
            authority["sourceUnchanged"] is True and
            b"vcs.revision=" + signed_owner.SIGNER_SOURCE.encode() in signer_info and b"vcs.modified=false" in signer_info,
            "DISTINCT_QUALIFIED_SIGNER")
    require(set(result["prepared"]) == {DIAGNOSTIC_CASE} and len(result["cases"]) == len(result["processes"]) == 1,
            "ONE_DIAGNOSTIC_SIGNING_CALL")
    prepared, row, process = result["prepared"][DIAGNOSTIC_CASE], result["cases"][0], result["processes"][0]
    require(row["caseId"] == DIAGNOSTIC_CASE and row["workflowName"] == prepared["name"] == NAME and
            prepared["actions"] == 5 and prepared["sourcePin"] == {"bytes": 139, "sha256": SOURCE_SHA256} and
            prepared["unsignedPin"] == {"bytes": 4972, "sha256": UNSIGNED_SHA256},
            "SIGNING_EXACT_DIAGNOSTIC_INPUTS")
    validate_process(files, "signing/commands/sign-" + DIAGNOSTIC_CASE + ".", process,
                     signed_owner.expected_argv(result, prepared), 120, 420)
    prefix = "signing/case/"
    require(files[prefix + "packet.derived.json"] == packet_raw and
            files[prefix + "source.original.cherri"] == checked["source"] and
            files[prefix + "unsigned.original.shortcut"] == checked["unsigned"],
            "SIGNED_DIAGNOSTIC_BYTES_EXACT")
    signed = files[prefix + "signed.original.shortcut"]
    manifest_raw = files[prefix + "provenance.original.json"]
    expected = {"schemaVersion": "1.4", "compilerCommitSha": COMPILER,
        "signingMethod": "HUBSIGN_REMOTE_SERVICE", "serviceName": "Custom Signing Server",
        "shortcutName": NAME, "sourceFile": "source.original.cherri", "sourceSha256": SOURCE_SHA256,
        "unsignedFile": Path(prepared["unsigned"]).name, "unsignedPlistSha256": UNSIGNED_SHA256,
        "unsignedByteSize": 4972, "nativeActionCount": 5, "signedFile": Path(prepared["signed"]).name,
        "signedArtifactSha256": sha(signed), "signedByteSize": len(signed), "headerMagic": "AEA1",
        "verificationTier": "AEA1_HEADER_PASS"}
    require(library.strict_json(manifest_raw) == expected and
            row["manifest"] == {"bytes": len(manifest_raw), "sha256": sha(manifest_raw)} and
            row["signed"] == {"bytes": len(signed), "sha256": sha(signed)} and
            row["verificationTier"] == "AEA1_HEADER_PASS" and
            len(signed) >= 16 and signed.startswith(b"AEA1") and any(signed[4:]),
            "SIGNED_DIAGNOSTIC_PROVENANCE_STRUCTURE_ONLY")
    return packet, None, checked, signed, {"schema": BUNDLE_SCHEMA,
        "diagnosticOnly": True, "compilerCommitSha": COMPILER, "originalPacketCompilerCommitSha": BUILD_EQUIVALENT_SOURCE,
        "compileResultSHA256": COMPILE_RESULT_SHA256, "signingResultSHA256": SIGNING_RESULT_SHA256,
        "signerCompilerCommitSha": signed_owner.SIGNER_SOURCE, "signerBinarySHA256": signed_owner.SIGNER_SHA256,
        "sourceSHA256": SOURCE_SHA256, "unsignedSHA256": UNSIGNED_SHA256, "signedSHA256": sha(signed),
        "nativeExecuted": False, "appleSignatureValidated": False, "formalAcceptanceGranted": False}


class MaterializedCallerDiagnostic(base.NativeCallerObservation):
    def __init__(self, plan, selection):
        self.diagnostic_selection = validate_selection(selection, plan)
        super().__init__(plan)
        self.summary.update(schema=SCHEMA, diagnosticOnly=True, diagnosticCaseId=DIAGNOSTIC_CASE,
            originalAcceptanceReplacement=False, originalPacketCompilerCommitSha=BUILD_EQUIVALENT_SOURCE,
            buildCompiledInputEquivalentSource=BUILD_EQUIVALENT_SOURCE,
            diagnosticPacketCompilerCommitSha=COMPILER)
        record(self.root / "diagnostic-selection.original.json", selection)

    def bind_diagnostic_build(self, plan, current_source, execution_context=None):
        require(execution_context is None and plan["compilerCommitSha"] == COMPILER and
                plan["builtHarnessCommitSha"] == BUILT_HARNESS and
                plan["buildResultSHA256"] == BUILD_RESULT_SHA256,
                "DIAGNOSTIC_BUILD_AUTHORITY")
        build_plan = {**plan, "compilerCommitSha": BUILD_EQUIVALENT_SOURCE}
        # The original validator still checks all 15 current compiled inputs
        # against both original build snapshots and the exact retained products.
        build = base.bind_build(build_plan, current_source)
        build.update(packetCompilerCommitSha=COMPILER,
                     buildCompiledInputEquivalentSource=BUILD_EQUIVALENT_SOURCE,
                     diagnosticOnly=True, originalAcceptanceReplacement=False)
        return build

    def bind_diagnostic_packet(self, plan):
        require(plan == self.plan, "DIAGNOSTIC_PLAN_UNCHANGED")
        _, intake, files = load_bundle(plan["bundleRoot"], plan["bundleIntakeSHA256"])
        packet, canonical, checked, signed, evidence = validate_originals(intake, files, plan)
        return packet, canonical, checked, evidence

    def sign_original(self):
        raw, intake, files = load_bundle(self.plan["bundleRoot"], self.plan["bundleIntakeSHA256"])
        self.packet, self.canonical_fixture, self.checked, signed, evidence = validate_originals(intake, files, self.plan)
        record(self.folder / "packet-binding.original.json", evidence)
        retained = self.folder / "exact-diagnostic-originals"
        retained.mkdir(mode=0o700)
        original(retained / "intake.original.json", raw)
        for name, content in files.items():
            path = retained / signed_owner.relative_name(name)
            path.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
            original(path, content)
        reread = load_bundle(retained, sha(raw))
        require(reread[0] == raw and reread[2] == files, "RETAINED_DIAGNOSTIC_ORIGINALS")
        signed_root = self.folder / "signed"
        signed_root.mkdir(mode=0o700)
        self.unsigned = signed_root / (NAME + ".unsigned.shortcut")
        self.signed = signed_root / (NAME + ".shortcut")
        original(self.unsigned, self.checked["unsigned"])
        self.signed_pin = original(self.signed, signed)
        evidence.update(method="EXISTING_CANONICAL_EXACT_FILE_SIGNING_CLI",
                        compilePerformed=False, signingRepeated=False, signingIntakeSHA256=sha(raw))
        record(self.folder / "signing.original.json", evidence)
        self.summary["diagnosticOriginals"] = {"intakeSHA256": sha(raw), "files": intake["files"], "binding": evidence}

    def run(self):
        require(signal.getitimer(signal.ITIMER_REAL) == (0.0, 0.0), "NO_FOREIGN_OWNER_ALARM")
        prior = {sig: signal.getsignal(sig) for sig in (signal.SIGALRM, signal.SIGTERM, signal.SIGINT)}
        def interrupt(signum, frame):
            self.latch.interrupt(self.capacity_reason or "NATIVE_OWNER_SIGNAL_" + str(signum))
        for sig in prior:
            signal.signal(sig, interrupt)
        signal.setitimer(signal.ITIMER_REAL, max(0.001, self.deadline - time.monotonic()))
        errors = []
        try:
            free = self.check_capacity()
            require(free >= LIVE_FLOOR_BYTES + self.plan["estimatedIncrementalBytes"],
                    "MEASURED_NATIVE_INCREMENTAL_ADMISSION")
            self.summary["capacityAdmission"] = {"freeBytes": free,
                "estimatedIncrementalBytes": self.plan["estimatedIncrementalBytes"],
                "originalLiveFloorBytes": LIVE_FLOOR_BYTES}
            self.start_capacity_watch()
            critical = ["tools/shortcuts-simulator-qa/" + p for p in CRITICAL]
            self.before = source_binding.source_inventory(self.repo,
                self.plan["controllerCommitSha"], self.folder / "source-before", critical)
            require(self.before["completeTrackedSource"] is True, "COMPLETE_CONTROLLER_SOURCE")
            context = getattr(self, "execution_context", None)
            self.build = (self.bind_diagnostic_build(self.plan, self.before) if context is None else
                          self.bind_diagnostic_build(self.plan, self.before, context))
            record(self.folder / "build-binding.original.json", self.build)
            self.products_before = native.test_products_inventory(self.build["xctestrun"])
            record(self.folder / "products-before.original.json", self.products_before)
            self.sign_original()
            self.select_device()
            self.start_observer()
            expected = library.context({"schema": library.CONTEXT_SCHEMA,
                "runId": self.plan["runId"], "runNonce": self.plan["runNonce"],
                "deviceUdid": self.device, "phaseId": "import-" + self.plan["runNonce"],
                "expectedName": self.checked["name"],
                "compilerCommitSha": self.plan["compilerCommitSha"],
                "builtHarnessCommitSha": self.plan["builtHarnessCommitSha"],
                "controllerCommitSha": self.plan["controllerCommitSha"],
                "unsignedSHA256": self.checked["unsignedSha256"],
                "signedSHA256": self.signed_pin["sha256"]})
            record(self.folder / "import-context.original.json", expected)
            self.caller_entered = True
            observed = observe_materialized_input(self, self.packet, self.canonical_fixture,
                                                   self.unsigned, self.signed, expected)
            self.summary.update(status="OBSERVATION_ACQUIRED", observation=observed,
                                actualRuntimeDispatches=observed["actualRuntimeDispatches"])
        except BaseException as error:
            self.summary["error"] = {"type": type(error).__name__, "message": str(error)[:1200]}
        finally:
            self.stop.set()
            if self.watcher is not None:
                self.watcher.join(timeout=2)
            # No second interruption may escape the existing child cleanup.
            # A normal finish is still bounded by the original whole deadline.
            if self.latch.reason is None:
                signal.setitimer(signal.ITIMER_REAL, max(0.001, self.hard_deadline - time.monotonic()))
            try:
                # The first cancellation may arrive after the inherited wait
                # catch, during group observation/kill/reap. Drain that entire
                # existing bounded closure before delivering the interruption.
                with defer_cancellation_signals():
                    native.NativeObservation.finish(self)
            except BaseException as error:
                errors.append("cleanup:" + type(error).__name__ + ":" + str(error)[:600])
            if self.test_log is not None and not self.test_log.closed:
                self.test_log.close()
            if self.summary["status"] == "OBSERVATION_ACQUIRED" and not errors:
                try:
                    native.validate_xctest(read(self.folder / "xctest.log"), self.xctest.returncode)
                except BaseException as error:
                    errors.append("xctest:" + type(error).__name__ + ":" + str(error)[:600])
            if self.latch.reason is None:
                try:
                    if self.before is not None:
                        after = source_binding.source_inventory(self.repo,
                            self.plan["controllerCommitSha"], self.folder / "source-after", critical)
                        require(after == self.before, "CONTROLLER_SOURCE_CHANGED")
                        self.summary["sourceUnchanged"] = True
                    if self.products_before is not None:
                        products = native.test_products_inventory(self.build["xctestrun"])
                        record(self.folder / "products-after.original.json", products)
                        require(products == self.products_before, "BUILT_PRODUCTS_CHANGED")
                    if self.packet is not None:
                        packet, canonical, checked, evidence = self.bind_diagnostic_packet(self.plan)
                        require(packet == self.packet and canonical == self.canonical_fixture
                                and checked == self.checked, "PACKET_BUNDLE_CHANGED")
                        if self.unsigned is not None:
                            require(read(self.unsigned) == checked["unsigned"], "UNSIGNED_PACKET_CHANGED")
                        if self.signed_pin is not None:
                            require(pinned(self.signed, self.signed_pin).startswith(b"AEA1"),
                                    "SIGNED_PACKET_CHANGED")
                except BaseException as error:
                    errors.append("closing:" + type(error).__name__ + ":" + str(error)[:600])
            else:
                errors.append("owner-interrupted:" + self.latch.reason)
            if time.monotonic() >= self.hard_deadline:
                errors.append("whole-owner-deadline")
            if errors:
                self.summary.update(status="FAIL", finalizationErrors=errors)
            elif self.summary["status"] == "OBSERVATION_ACQUIRED":
                self.summary.update(status="OBSERVATION_VERIFIED",
                                    runtimeObservationAuthenticated=True)
            if not self.caller_entered:
                self.summary["actualRuntimeDispatches"] = 0
            self.summary["callerObservationEntered"] = self.caller_entered
            self.summary.update(firstInterruption=self.latch.reason,
                minimumFreeBytes=self.minimum_free, originalLiveFloorBytes=LIVE_FLOOR_BYTES,
                elapsedSeconds=time.monotonic() - self.started, finishedUTC=library.now(),
                privateXCTestResult=str(self.folder / "NativeCaller.xcresult"))
            try:
                record(self.folder / "result.original.json", self.summary)
            finally:
                signal.setitimer(signal.ITIMER_REAL, 0)
                for sig, handler in prior.items():
                    signal.signal(sig, handler)
        return self.summary


def observe_materialized_input(owner, packet, canonical_fixture, unsigned, signed, expected):
    """One real imported FN04 case; the external closed owner binds source/build.

    owner is the existing native owner with repo/product/source admission,
    capture(), deadline, folder, device, device_data_path, documents and xctest.
    It retains all raw XCTest/AX/private originals and closes the owned group.
    This function never constructs or assumes a successful owner admission.
    """
    caller.require(sys.platform == "darwin" and sys.version_info >= (3, 10), "DARWIN_SHARED_MONOTONIC_CLOCK")
    checked = validate_packet(packet, canonical_fixture)
    expected = library.context(expected)
    caller.require(expected["expectedName"] == checked["name"] and
            expected["unsignedSHA256"] == checked["unsignedSha256"] and
            expected["deviceUdid"] == owner.device, "NATIVE_CASE_CONTEXT")
    unsigned, signed = Path(unsigned), Path(signed)
    caller.require(library.read(unsigned) == checked["unsigned"], "UNSIGNED_PACKET_CHANGED")
    signed_raw = library.read(signed)
    caller.require(signed_raw.startswith(b"AEA1") and fixtures.sha(signed_raw) == expected["signedSHA256"],
            "BOUND_SIGNED_PACKET")
    caller.require(owner.admission_verified is True and owner.xctest.poll() is None, "EXISTING_OWNER_ADMISSION")
    binding = caller.imported_unique(owner, expected, unsigned, signed)
    nonce = expected["runNonce"]
    caller.require(re.fullmatch("[0-9a-f]{32}", nonce) is not None and nonce != "0" * 32,
            "FRESH_NATIVE_RUN_NONCE")
    input_text = "CHERRI-CALLER-" + nonce
    fixture_id = "fn04-" + checked["caseId"] + "-" + checked["token"]
    reset = ("CHERRI-CALLER-RESET-" + uuid.uuid4().hex).encode()
    reset_path = owner.folder / (nonce + "-clipboard-reset.original.txt")
    caller.original(reset_path, reset)
    owner.capture(nonce + "-clipboard-reset",
        [sys.executable, "-I", "-B", "-c", caller.PBCOPY_EXEC, owner.device, str(reset_path)], 25)
    _, observed_reset, _ = owner.capture(nonce + "-clipboard-reset-readback",
        ["/usr/bin/xcrun", "simctl", "pbpaste", owner.device], 25)
    caller.require(observed_reset == reset, "CLIPBOARD_RESET_NOT_OBSERVED")
    query = {"name": checked["name"], "input": "text", "text": input_text,
             "x-source": "CherriSimulatorQAReceiver"}
    for status in ("success", "error", "cancel"):
        query["x-" + status] = "cherriqa://" + status + "?" + urllib.parse.urlencode({
            "runId": nonce, "fixtureId": fixture_id, "shortcutName": checked["name"],
            "source": "shortcuts-x-callback"})
    target = "shortcuts://x-callback-url/run-shortcut?" + urllib.parse.urlencode(query)
    trigger = "cherriqa://trigger?" + urllib.parse.urlencode({"target": target})
    phase = "runtime-" + nonce
    claim = owner.folder / (checked["caseId"] + "-runtime-claim.original.json")
    caller.record(claim, {"schema": SCHEMA, "caseId": checked["caseId"], "runNonce": nonce,
        "inputText": input_text, "targetURL": target, "dispatchOrdinal": 1,
        "unsignedSha256": checked["unsignedSha256"], "signedSha256": expected["signedSHA256"],
        "libraryNativeUUID": binding["nativeRecord"]["workflowUUID"],
        "formalAcceptanceGranted": False})
    dispatch = caller.dispatch_ui(owner, phase, "runtime", checked["name"],
        ["/usr/bin/xcrun", "simctl", "openurl", owner.device, trigger])
    callback = None
    deadline = min(owner.deadline, time.monotonic() + 48)
    while time.monotonic() < deadline:
        path = owner.documents / "callback-events.json"
        if path.is_file():
            raw = library.read(path)
            events = library.strict_json(raw)
            caller.require(type(events) is list and all(type(e) is dict for e in events), "CALLBACK_EVENT_ARRAY")
            matched = [e for e in events if e.get("runId") == nonce]
            caller.require(len(matched) <= 1, "DUPLICATE_NATIVE_CALLBACK")
            if matched:
                callback = matched[0]
                caller.original(owner.folder / (nonce + "-callback-events.original.json"), raw)
                differential.atomic_json(owner.folder / ("completion-" + phase + ".json"), callback)
                break
        caller.require(owner.xctest.poll() is None, "XCTEST_ENDED_BEFORE_CALLBACK")
        owner.check_capacity()
        time.sleep(0.25)
    ui = differential.wait_file(owner.folder / ("ack-" + phase + ".json"), 60, owner.xctest)
    caller.require(ui.get("id") == phase and ui.get("operation") == "runtime" and
            ui.get("outcome") == "CALLBACK_OBSERVED_BY_HOST", "NATIVE_RUNTIME_UI")
    caller.require(callback is not None, "NATIVE_CALLBACK_NOT_OBSERVED")
    _, raw, _ = owner.capture(nonce + "-actual-clipboard",
        ["/usr/bin/xcrun", "simctl", "pbpaste", owner.device], 25)
    caller.original(owner.folder / (nonce + "-clipboard-result.original.txt"), raw)
    result = caller.validate_runtime_bytes(callback, checked["caseId"], fixture_id, checked["name"],
                                    nonce, raw, input_text)
    caller.require(library.read(unsigned) == checked["unsigned"] and library.read(signed) == signed_raw,
            "PACKET_CHANGED_DURING_NATIVE_EXECUTION")
    result.update(schema=SCHEMA, status="OBSERVED_EXACT_CALLER_INPUT", context=expected,
        runNonce=nonce, signedSha256=expected["signedSHA256"], unsignedSha256=checked["unsignedSha256"],
        nativeLibraryUUID=binding["nativeRecord"]["workflowUUID"], actualRuntimeDispatches=1,
        callbackSHA256=fixtures.sha(library.canonical(callback)), nativeExecuted=True,
        dispatchReceiptSHA256=fixtures.sha(library.canonical(dispatch)))
    result.update(diagnosticOnly=True, diagnosticCaseId=DIAGNOSTIC_CASE, originalAcceptanceReplacement=False)
    caller.record(owner.folder / (checked["caseId"] + "-result.original.json"), result)
    return result


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--plan", required=True)
    parser.add_argument("--signing-selection", required=True)
    args = parser.parse_args(argv)
    plan = base.validate_plan(library.strict_json(read(Path(args.plan))))
    selection = validate_selection(library.strict_json(read(Path(args.signing_selection))), plan)
    owner = MaterializedCallerDiagnostic(plan, selection)
    result = owner.run()
    print(json.dumps({"status": result["status"], "runId": plan["runId"],
        "diagnosticCaseId": DIAGNOSTIC_CASE, "diagnosticOnly": True,
        "runtimeObservationAuthenticated": result["runtimeObservationAuthenticated"],
        "originalAcceptanceReplacement": False, "formalAcceptanceGranted": False}))
    return 0 if result["status"] == "OBSERVATION_VERIFIED" else 1


if __name__ == "__main__":
    raise SystemExit(main())
