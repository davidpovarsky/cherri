#!/usr/bin/env python3
"""Observe the retained source899 storage fixture with the reviewed native owner.

This is an engineering observation, never final acceptance. The compiler,
signature transport, built Receiver and current controller remain distinct.
"""
import argparse
import datetime
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import signal
import sys
import threading
import time
import urllib.parse
import uuid

import function_envelope_observer as native

COMPILER = "899b8513f08e02ba38420b522806c58977049f57"
BUILD = "d481a90d231f2b240379d2898dfca78f293182e4"
BUILD_TREE = "685bb6f453cad65cf107748f2b1eef3f14235e07"
FIXTURE = "qa-storage-delete-v2"
EXPECTED = "CHERRI_QA_STORAGE_DELETE_RESULT:BEFORE_OK:NO_VALUE_OK"
INPUT = "CHERRI_QA_INPUT"
PACKET_PINS = {
    "builds/qa-storage-delete-v2.cherri": "4361d16d59d97a81c93da7e8d3d81168f2c10caa25db67229a18bb848edd05fc",
    "builds/qa-storage-delete-v2.shortcut": "693ec6fff76b7d3fc6b169697d6451ebe2f22cca4001277d9392eb8f67b77e7f",
    "builds/signed/qa-storage-delete-v2.shortcut": "d5c74ee3f53521b042a3c3ec33cdb42e3c7330eb851cffaefd7e91d81cadaae7",
    "compiler-manifest.original.json": "928a42bcf825404c192646584d0bb6456ea7871f60fdd6395ce79e494ed3be7d",
    "phase0-process.original.json": "aee717acf871cd50ae198e691b556f8d1d8fedc701bf3af1c3ec6bb948010ac6",
    "input-manifest.original.json": "557364f64f474c27952ad5e283da1e3b4ad1f9fd0ab8985d82100a86ecd97b1c",
    "signing-provenance-target.original.json": "ccb7b7bba446057d73da736d5e7ba0a8ca1fcd736ec90ca08d437a5db5142010",
    "signing-result.original.json": "9c8bf23d28fcd35a367d189f2abc1d3761f46224c2062091396c2385525dc436",
}
RUNTIME_EVALUATOR_SHA256 = "6d17a2084e2dbb8df055ba3b36d3fe3f457165d7ed4d8e732e6fe07ebe0b101c"
PROJECT = "tools/shortcuts-simulator-qa/XcodeProject/"
BUILD_SCOPE = "standalone-project-and-python-helper-closure"
BUILD_PROJECT_FILES = [
    "tools/shortcuts-simulator-qa/XcodeProject/ReceiverApp/Info.plist",
    "tools/shortcuts-simulator-qa/XcodeProject/ReceiverApp/ReceiverApp.swift",
    "tools/shortcuts-simulator-qa/XcodeProject/Shared/CallbackPayloadDiagnostics.swift",
    "tools/shortcuts-simulator-qa/XcodeProject/Shared/DebugLabelPreviewPolicy.swift",
    "tools/shortcuts-simulator-qa/XcodeProject/Shared/DifferentialImportTiming.swift",
    "tools/shortcuts-simulator-qa/XcodeProject/Shared/EditorForegroundRecovery.swift",
    "tools/shortcuts-simulator-qa/XcodeProject/Shared/EvidenceGeometry.swift",
    "tools/shortcuts-simulator-qa/XcodeProject/Shared/ImportSheetSelection.swift",
    "tools/shortcuts-simulator-qa/XcodeProject/Shared/LibraryForegroundGate.swift",
    "tools/shortcuts-simulator-qa/XcodeProject/Shared/PrimaryNoInputLaunch.swift",
    "tools/shortcuts-simulator-qa/XcodeProject/UITests/CherriDifferentialBatchUITests.swift",
    "tools/shortcuts-simulator-qa/XcodeProject/UITests/CherriSimulatorQAUITests.swift",
    "tools/shortcuts-simulator-qa/XcodeProject/UITests/DebugLabelPreviewObserver.swift",
    "tools/shortcuts-simulator-qa/XcodeProject/project.yml"
]
BUILD_HELPER_FILES = [
    "tools/shortcuts-simulator-qa/project_prerequisite.py",
    "tools/shortcuts-simulator-qa/project_prerequisite_observation.py",
    "tools/shortcuts-simulator-qa/qa_terminal_receipts.py",
    "tools/shortcuts-simulator-qa/run_differential_batch.py",
    "tools/shortcuts-simulator-qa/simulator_session_recovery.py"
]
BUILD_QUALIFICATION_FILES = [
    "tools/shortcuts-simulator-qa/verify_differential_import_timing.swift",
    "tools/shortcuts-simulator-qa/verify_import_sheet_selection.swift",
    "tools/shortcuts-simulator-qa/verify_primary_no_input_r2.py"
]
BUILD_SCOPE_PATHS = [PROJECT, *BUILD_HELPER_FILES, *BUILD_QUALIFICATION_FILES]
BUILD_INPUT_FILES = set(BUILD_PROJECT_FILES + BUILD_HELPER_FILES + BUILD_QUALIFICATION_FILES)
require, read, record, original = native.require, native.read, native.record, native.original
sha = lambda value: hashlib.sha256(value).hexdigest()


def validate_plan(plan):
    keys = {"schema", "repo", "root", "controllerCommitSha", "compilerCommitSha", "builtHarnessCommitSha",
            "buildRoot", "buildQualificationSHA256", "runId", "runAttempt", "runNonce", "deviceUdid"}
    require(isinstance(plan, dict) and set(plan) == keys, "PLAN_FIELDS")
    require(plan["schema"] == "cherri-storage-native-observer-plan/1"
            and plan["compilerCommitSha"] == COMPILER and plan["builtHarnessCommitSha"] == BUILD
            and plan["deviceUdid"] == native.DEVICE, "PLAN_IDENTITIES")
    require(isinstance(plan["controllerCommitSha"], str)
            and re.fullmatch(r"[0-9a-f]{40}", plan["controllerCommitSha"]) is not None
            and isinstance(plan["buildQualificationSHA256"], str)
            and re.fullmatch(r"[0-9a-f]{64}", plan["buildQualificationSHA256"]) is not None, "PLAN_SOURCE_HASHES")
    require(type(plan["runAttempt"]) is int and plan["runAttempt"] == 1
            and all(isinstance(plan[k], str) and re.fullmatch(r"[A-Za-z0-9_-]{1,128}", plan[k])
                    for k in ("runId", "runNonce")), "PLAN_RUN")
    for key in ("repo", "root", "buildRoot"):
        require(isinstance(plan[key], str) and Path(plan[key]).is_absolute()
                and ".." not in Path(plan[key]).parts, "PLAN_PATH")
    require(plan["root"] != plan["repo"] and plan["root"] != plan["buildRoot"], "PLAN_ROOT_COLLISION")
    return plan


def validate_import(dispatch, receipt, argv, phase):
    require(isinstance(dispatch, dict) and dispatch.get("command") == argv
            and dispatch.get("outcome") == "EXITED" and dispatch.get("timedOut") is False
            and type(dispatch.get("exitCode")) is int and dispatch["exitCode"] == 0
            and type(dispatch.get("processReturncode")) is int and dispatch["processReturncode"] == 0
            and dispatch.get("parentReaped") is True and dispatch.get("timeoutSeconds") == 60,
            "IMPORT_PROCESS")
    require(isinstance(receipt, dict) and receipt.get("id") == phase and receipt.get("operation") == "import"
            and receipt.get("outcome") == "IMPORTED" and receipt.get("name") == FIXTURE
            and receipt.get("identityVerified") is True and receipt.get("libraryIdentityVerified") is True,
            "IMPORT_IDENTITY")
    timing = receipt.get("importTiming")
    require(isinstance(timing, dict) and timing.get("contract") == "dispatch-exit-anchored-add-v1"
            and timing.get("dispatchState") == "EXITED"
            and timing.get("dispatchReceipt") == "dispatch-" + phase + ".json", "IMPORT_TIMING")
    require(timing.get("sheetContract") == "requested-identity-and-add-v1"
            and all(timing.get(name) is True for name in ("sheetButtonExists", "sheetButtonHittable",
                "sheetIdentityMatched", "sheetCanTap")), "IMPORT_SHEET_READINESS")
    require(timing.get("sheetSelectionContract") == "current-container-bound-add-v1"
            and timing.get("sheetSelectionReason") == "UNIQUE_CURRENT_NAMED_CONTAINER",
            "IMPORT_CURRENT_CONTAINER_CONTRACT")
    for name, low, high in (("sheetQueryCandidateCount", 1, 32), ("sheetCurrentContainerCount", 1, 8192),
                            ("sheetMatchedCandidateCount", 1, 1), ("sheetSelectedQueryIndex", 0, 31)):
        value = timing.get(name)
        require(type(value) is int and low <= value <= high, "IMPORT_CURRENT_CONTAINER_COUNTS")
    require(timing["sheetSelectedQueryIndex"] < timing["sheetQueryCandidateCount"],
            "IMPORT_CURRENT_CONTAINER_INDEX")
    paths = []
    for name in ("sheetSelectedContainerPath", "sheetSelectedTitlePath", "sheetSelectedButtonPath"):
        path = timing.get(name)
        require(isinstance(path, list) and 0 < len(path) <= 64
                and all(type(index) is int and 0 <= index < 8192 for index in path),
                "IMPORT_CURRENT_CONTAINER_PATH")
        paths.append(path)
    container, title, button = paths
    require(all(len(path) > len(container) and path[:len(container)] == container
                for path in (title, button))
            and title[len(container)] != button[len(container)], "IMPORT_CURRENT_CONTAINER_ANCESTRY")
    instants = []
    for name in ("addObservedUTC", "lastSheetCheckUTC", "sheetMatchedAfterCaptureUTC"):
        raw = timing.get(name)
        require(isinstance(raw, str) and raw.endswith("Z"), "IMPORT_SHEET_CLOCK")
        instant = datetime.datetime.fromisoformat(raw[:-1] + "+00:00")
        require(instant.tzinfo is not None, "IMPORT_SHEET_CLOCK")
        instants.append(instant)
    require(instants[0] <= instants[1] <= instants[2], "IMPORT_SHEET_CLOCK_ORDER")


def exact_storage_trace(callback, nonce):
    require(isinstance(callback, dict) and callback.get("receiverSelfTest") is False
            and callback.get("eventSource") == "shortcuts-x-callback" and callback.get("runId") == nonce
            and callback.get("expectedShortcutName") == FIXTURE and callback.get("fixtureId") == FIXTURE
            and callback.get("status") == "SUCCESS" and callback.get("resultSummary") == EXPECTED,
            "STORAGE_CALLBACK")
    raw = callback.get("tracePayload")
    require(isinstance(raw, str) and 0 < len(raw.encode("utf-8")) <= 65536, "STORAGE_TRACE_BYTES")
    lines = [line.strip() for line in raw.splitlines()
             if line.strip() and line.strip() != "===CHERRI_QA_TRACE_NDJSON_V14==="]
    require(len(lines) == 4, "STORAGE_TRACE_COUNT")
    rows = [native.launch.strict_json(line.encode("utf-8")) for line in lines]
    for index, stage in ((0, "collector_begin"), (3, "collector_end")):
        expected = {"schemaVersion": "1.4", "kind": "COLLECTOR_EVENT", "stage": stage,
                    "runId": FIXTURE, "traceTransport": "ndjson-stream",
                    "collectorSentinel": "===CHERRI_QA_TRACE_NDJSON_V14==="}
        require(rows[index] == expected, "STORAGE_COLLECTOR_IDENTITY")
    for index, stage, step, action in ((1, "before", 3, "setvariable"), (2, "after", 24, "comment")):
        row = rows[index]
        require(isinstance(row, dict) and isinstance(row.get("timestamp"), str)
                and row["timestamp"].strip() and row["timestamp"] != "\ufffc", "STORAGE_TIMESTAMP")
        expected = {"schemaVersion": "1.4", "kind": "INLINE_DEBUG_ACTION", "stage": stage,
                    "label": "delete", "stepSequence": step,
                    "actionIdentifier": "is.workflow.actions." + action,
                    "timestampPrecision": "apple-currentdate", "valueType": "UNKNOWN_AT_TRACE_EVENT",
                    "timestamp": row["timestamp"]}
        require(type(row.get("stepSequence")) is int and row == expected, "STORAGE_INLINE_IDENTITY")
    return rows


def packet_inventory(root):
    pins = {}
    for name, digest in PACKET_PINS.items():
        raw = read(root / name)
        require(sha(raw) == digest, "RETAINED_PACKET_BYTES")
        pins[name] = {"bytes": len(raw), "sha256": digest}
    manifest = native.launch.strict_json(read(root / "compiler-manifest.original.json"))
    fixtures = [row for row in manifest["fixtures"] if row.get("fixtureId") == FIXTURE]
    require(manifest.get("compilerCommitSha") == COMPILER and manifest.get("syntheticOnly") is True
            and len(fixtures) == 1 and fixtures[0].get("sourceSha256") == PACKET_PINS["builds/" + FIXTURE + ".cherri"]
            and fixtures[0].get("unsignedPlistSha256") == PACKET_PINS["builds/" + FIXTURE + ".shortcut"],
            "RETAINED_COMPILER_MANIFEST")
    require(read(root / ("builds/signed/" + FIXTURE + ".shortcut")).startswith(b"AEA1"), "SIGNED_CONTAINER")
    signing = native.launch.strict_json(read(root / "signing-result.original.json"))
    require(signing.get("status") == "SIGNED_ORIGINAL" and signing.get("compilerCommitSha") == COMPILER
            and signing.get("compilePerformed") is False and signing.get("nativeExecutionPerformed") is False
            and type(signing.get("signingInvocationCount")) is int and signing["signingInvocationCount"] == 1
            and signing.get("sourceUnchanged") is True
            and signing.get("signedSHA256") == PACKET_PINS["builds/signed/" + FIXTURE + ".shortcut"]
            and signing.get("targetProvenanceSHA256") == PACKET_PINS["signing-provenance-target.original.json"]
            and signing.get("observation", {}).get("result") == "PASS"
            and signing.get("process", {}).get("cleanupParentReaped") is True
            and signing["process"].get("groupPresentAfterCleanup") is False, "ORIGINAL_SIGNING_PROCESS")
    return pins


def validate_clean_build_metadata(build, qualification):
    require(isinstance(build, dict) and isinstance(qualification, dict), "BUILD_METADATA_TYPES")
    require(qualification.get("schema") == "cherri-standalone-storage-harness-build-qualification/3"
            and qualification.get("status") == "BUILD_VALIDATED"
            and qualification.get("builtHarnessCommit") == BUILD and qualification.get("builtHarnessTree") == BUILD_TREE
            and qualification.get("sourceTrackedBytesUnchanged") is True
            and qualification.get("sourceCleanAfterRetainedProjectRelocation") is True
            and qualification.get("xcodeBuildReexecuted") is False and qualification.get("compilerRebuilt") is False
            and qualification.get("nativeInvocation") is False
            and qualification.get("completeRepositoryInventory") is False
            and qualification.get("completeTrackedProject") is True
            and qualification.get("sourceScope") == BUILD_SCOPE
            and qualification.get("scopePaths") == BUILD_SCOPE_PATHS
            and type(qualification.get("sourceFiles")) is int and qualification["sourceFiles"] == 22
            and type(qualification.get("projectInputCount")) is int and qualification["projectInputCount"] == 14,
            "BUILD_QUALIFICATION")
    require(build.get("schema") == "cherri-standalone-storage-harness-build/2"
            and build.get("status") == "BUILD_VALIDATED" and build.get("builtHarnessCommit") == BUILD
            and build.get("builtHarnessTree") == BUILD_TREE and build.get("error") is None
            and build.get("sourceGuardError") is None and build.get("sourceUnchanged") is True
            and build.get("generatedProjectRetained") is True
            and build.get("compilerBuilt") is False and build.get("shortcutExecuted") is False
            and build.get("testsExecuted") is False
            and build.get("completeRepositoryInventory") is False and build.get("sourceScope") == BUILD_SCOPE
            and type(build.get("sourceFiles")) is int and build["sourceFiles"] == 22
            and type(build.get("projectInputCount")) is int and build["projectInputCount"] == 14,
            "BUILD_ORIGINAL_OUTCOME")


def validate_build_source(source):
    require(isinstance(source, dict) and source.get("commit") == BUILD and source.get("tree") == BUILD_TREE
            and source.get("completeTrackedSource") is False and source.get("completeTrackedProject") is True
            and source.get("sourceScope") == BUILD_SCOPE and source.get("scopePaths") == BUILD_SCOPE_PATHS,
            "BUILD_SOURCE_IDENTITY")
    files = source.get("files")
    require(isinstance(files, dict) and set(files) == BUILD_INPUT_FILES, "BUILD_SOURCE_SCOPE")
    for pin in files.values():
        require(isinstance(pin, dict) and set(pin) == {"bytes", "gitBlob", "mode", "sha256"}
                and type(pin["bytes"]) is int and pin["bytes"] >= 0 and pin["mode"] in ("100644", "100755")
                and isinstance(pin["gitBlob"], str) and re.fullmatch(r"[0-9a-f]{40}", pin["gitBlob"]) is not None
                and isinstance(pin["sha256"], str) and re.fullmatch(r"[0-9a-f]{64}", pin["sha256"]) is not None,
                "BUILD_SOURCE_PIN")
    return files


def verify_build(plan, repo, current_sources):
    root = native.launch.owned_root(plan["buildRoot"])
    qraw = read(root / "qualification/result.original.json")
    require(sha(qraw) == plan["buildQualificationSHA256"], "BUILD_QUALIFICATION_HASH")
    qualification = native.launch.strict_json(qraw)
    require(isinstance(qualification, dict), "BUILD_QUALIFICATION_TYPE")
    raw = read(root / "result.original.json")
    require(sha(raw) == qualification.get("originalOwnerResultSHA256"), "BUILD_RECEIPT_HASH")
    build = native.launch.strict_json(raw)
    validate_clean_build_metadata(build, qualification)
    for label in ("xcodegen", "xcodebuild", "codesign-verify-0", "codesign-verify-1"):
        rows = [row for row in build["processes"] if row.get("label") == label]
        require(len(rows) == 1, "BUILD_PROCESS_COUNT")
        row = rows[0]
        require(row.get("observation", {}).get("result") == "PASS"
                and type(row["observation"].get("exitCode")) is int and row["observation"]["exitCode"] == 0
                and row.get("process", {}).get("cleanupParentReaped") is True
                and row["process"].get("groupPresentAfterCleanup") is False, "BUILD_PROCESS")
        for side in ("stdout", "stderr"):
            stream = native.launch.regular(root / (label + "." + side + ".original"), native.MAX_ORIGINAL, empty=True)
            require(len(stream) == row["process"][side]["bytes"]
                    and sha(stream) == row["process"][side]["sha256"], "BUILD_PROCESS_STREAM")
    before, after = read(root / "source-before.original.json"), read(root / "qualification/source.after.original.json")
    require(before == after and sha(before) == qualification["sourceBeforeSHA256"]
            and sha(after) == qualification["sourceAfterSHA256"], "BUILD_SOURCE_CHANGED")
    source = native.launch.strict_json(before)
    files = validate_build_source(source)
    project = {k: files[k] for k in BUILD_PROJECT_FILES}
    for name, pin in files.items():
        data = read(repo / name)
        blob = hashlib.sha1(b"blob " + str(len(data)).encode("ascii") + b"\0" + data).hexdigest()
        mode = "100755" if (repo / name).stat().st_mode & 0o111 else "100644"
        require(len(data) == pin["bytes"] and sha(data) == pin["sha256"]
                and blob == pin["gitBlob"] and mode == pin["mode"]
                and current_sources["files"][name]["sha256"] == pin["sha256"], "BUILD_CONTROLLER_INPUT_BYTES")
    products_raw = read(root / "build-products.original.json")
    require(sha(products_raw) == qualification["buildProductsSHA256"], "BUILD_PRODUCTS_MANIFEST")
    products = native.launch.strict_json(products_raw)
    product_root = native.launch.owned_root(products["root"])
    for name, pin in products["files"].items():
        require(not Path(name).is_absolute() and ".." not in Path(name).parts, "BUILD_PRODUCT_PATH")
        data = native.launch.regular(product_root / name, 128 * 1024 * 1024, empty=True)
        require(len(data) == pin["bytes"] and sha(data) == pin["sha256"], "BUILD_PRODUCT_BYTES")
    require(len(products["files"]) == build["productFiles"], "BUILD_PRODUCT_COUNT")
    image_raw = read(root / "receiver-image.original.json")
    require(sha(image_raw) == qualification["receiverImageSHA256"], "BUILD_IMAGE_MANIFEST")
    image = native.launch.strict_json(image_raw)
    require(native.differential.receiver_image_inventory(Path(build["receiverApp"])) == image, "BUILD_RECEIVER_IMAGE")
    require(Path(build["xctestrun"]).parent == product_root
            and sha(read(build["xctestrun"])) == build["xctestrunSHA256"] == qualification["xctestrunSHA256"], "BUILD_XCTESTRUN")
    return {"receiverApp": build["receiverApp"], "xctestrun": build["xctestrun"],
            "builtHarnessCommitSha": BUILD, "buildReceiptSHA256": sha(raw),
            "qualificationSHA256": sha(qraw), "projectSources": project}


class StorageObservation(native.NativeObservation):
    # The reviewed capture/start_observer/finish implementations remain inherited.
    # In particular, parent exit never substitutes for an absent owned XCTest PGID.
    def __init__(self, plan):
        require(sys.platform == "darwin", "DARWIN_REQUIRED")
        self.plan = validate_plan(plan)
        self.repo, self.root = native.launch.owned_root(plan["repo"]), native.launch.owned_root(plan["root"])
        require(self.repo not in self.root.parents and self.root not in self.repo.parents, "ROOT_SCOPE")
        (self.root / "runtime").mkdir(mode=0o700, exist_ok=True)
        native.launch.owned_root(self.root / "runtime")
        self.folder = self.root / "runtime/storage-observer"
        self.folder.mkdir(mode=0o700)
        self.process_folder = self.folder / "processes"
        self.process_folder.mkdir(mode=0o700)
        self.xctest = self.test_log = self.documents = self.product_inventory = self.build = None
        self.minimum_free = shutil.disk_usage("/System/Volumes/Data").free
        self.capacity_stop = threading.Event()
        self.capacity_thread = None
        self.before = self.packet_before = None
        self.summary = {**plan, "schema": "cherri-storage-native-observer/1", "status": "FAIL",
                        "importTransportAuthenticated": False, "appleExecutionAuthenticated": False,
                        "formalAcceptanceGranted": False, "finalCandidateCredit": False,
                        "runtimeDispatchIntentCount": 0, "runtimeDispatchCount": 0, "storageGoalsMatched": False}
        record(self.folder / "plan.original.json", plan)

    def select_device(self):
        folder = self.folder / "device"
        folder.mkdir(mode=0o700)
        for name in ("runtimes", "devicetypes", "devices"):
            _, raw, _ = self.capture("inventory-" + name, ["/usr/bin/xcrun", "simctl", "list", name, "--json"], 15)
            original(folder / ("sim-" + name + ".json"), raw)
        record(folder / "owner.original.json", {"schema": native.selection.SCHEMA,
            "sourceCommit": self.plan["controllerCommitSha"], "runId": self.plan["runId"],
            "udid": native.DEVICE, "deviceName": "Cherri-P10R15-D03",
            "deviceTypeIdentifier": "com.apple.CoreSimulator.SimDeviceType.iPhone-16-Pro",
            "runtimeId": "com.apple.CoreSimulator.SimRuntime.iOS-27-0", "runtimeName": "iOS 27.0", "runtimeVersion": "27.0"})
        code = native.selection.main(["--record", str(folder / "owner.original.json"), "--inventory-root", str(folder),
            "--output", str(folder / "selection.original.json"), "--source-sha", self.plan["controllerCommitSha"],
            "--run-id", self.plan["runId"], "--profile", "iphone-fallback"])
        require(code == 0, "OWNED_DEVICE_SELECTION")
        self.capture("session-pre-import", [sys.executable, "-B", str(self.repo / "tools/shortcuts-simulator-qa/simulator_phase_boundary.py"),
            "--repo", str(self.repo), "--root", str(self.folder / "session-pre-import"), "--udid", native.DEVICE,
            "--source-sha", self.plan["controllerCommitSha"], "--run-id", self.plan["runId"],
            "--attempt", str(self.plan["runAttempt"]), "--fixture-id", FIXTURE, "--phase", "pre-import"], 310)

    def import_original(self):
        # Preserve the original storage route's explicit fresh Receiver launch.
        self.capture("receiver-fresh-launch", ["/usr/bin/xcrun", "simctl", "launch", "--terminate-running-process",
                                             native.DEVICE, native.launch.RECEIVER], 60)
        signed = self.root / ("builds/signed/" + FIXTURE + ".shortcut")
        phase = "import-" + uuid.uuid4().hex
        argv = ["xcrun", "simctl", "openurl", native.DEVICE, signed.as_uri()]
        dispatch = self.folder / ("dispatch-" + phase + ".json")
        command = {"id": phase, "operation": "import", "shortcutName": FIXTURE}
        native.differential.command(argv, timeout=60,
            on_started=lambda: native.differential.atomic_json(self.folder / "command.json", command), receipt_path=dispatch)
        receipt = native.differential.wait_file(self.folder / ("ack-" + phase + ".json"), 90, self.xctest)
        validate_import(native.launch.strict_json(read(dispatch)), receipt, argv, phase)
        self.summary.update(importTransportAuthenticated=True, importPhase=phase,
                            importedSignedSHA256=sha(read(signed)),
                            importReceiptSHA256=sha(read(self.folder / ("ack-" + phase + ".json"))))

    def one_runtime(self):
        nonce = self.plan["runNonce"]
        query = {"name": FIXTURE, "x-source": "CherriSimulatorQAReceiver", "input": "text", "text": INPUT}
        for status in ("success", "error", "cancel"):
            query["x-" + status] = "cherriqa://" + status + "?" + urllib.parse.urlencode(
                {"runId": nonce, "fixtureId": FIXTURE, "shortcutName": FIXTURE, "source": "shortcuts-x-callback"})
        target = "shortcuts://x-callback-url/run-shortcut?" + urllib.parse.urlencode(query)
        trigger = "cherriqa://trigger?" + urllib.parse.urlencode({"target": target})
        phase = "runtime-" + uuid.uuid4().hex
        argv = ["xcrun", "simctl", "openurl", native.DEVICE, trigger]
        record(self.folder / "runtime-dispatch-claim.original.json", {"argv": argv, "phase": phase,
            "targetURL": target, "inputText": INPUT, "dispatchOrdinal": 1, "compilerCommitSha": COMPILER,
            "scope": "One prescribed original storage dispatch; normal Receiver fallback policy is unchanged."})
        self.summary["runtimeDispatchIntentCount"] = 1
        dispatch = self.folder / ("dispatch-" + phase + ".json")
        command = {"id": phase, "operation": "runtime", "shortcutName": FIXTURE}
        def arm_started_runtime():
            require(self.summary["runtimeDispatchCount"] == 0, "RUNTIME_DISPATCH_ALREADY_STARTED")
            self.summary["runtimeDispatchCount"] = 1
            native.differential.atomic_json(self.folder / "command.json", command)
        native.differential.command(argv, timeout=60, on_started=arm_started_runtime, receipt_path=dispatch)
        _, raw, _ = self.capture("receiver-data-container", ["/usr/bin/xcrun", "simctl", "get_app_container",
                                                           native.DEVICE, native.launch.RECEIVER, "data"], 25)
        self.documents = native.launch.owned_root(Path(raw.decode().strip()) / "Documents")
        callbacks = self.documents / "callback-events.json"
        end = time.monotonic() + 48
        callback = None
        while time.monotonic() < end:
            if callbacks.is_file():
                raw = read(callbacks)
                events = native.launch.strict_json(raw)
                require(isinstance(events, list) and all(isinstance(e, dict) for e in events), "CALLBACK_ARRAY")
                matched = [e for e in events if e.get("runId") == nonce]
                require(len(matched) <= 1, "CALLBACK_DUPLICATE")
                if matched:
                    callback = matched[0]
                    original(self.folder / "callback-events.original.json", raw)
                    native.differential.atomic_json(self.folder / ("completion-" + phase + ".json"), callback)
                    break
            require(self.xctest.poll() is None, "XCTEST_ENDED_DURING_RUNTIME")
            self.check_capacity()
            time.sleep(.25)
        ui = native.differential.wait_file(self.folder / ("ack-" + phase + ".json"), 60, self.xctest)
        self.summary["runtimeUIReceipt"] = ui
        require(ui.get("id") == phase and ui.get("operation") == "runtime"
                and ui.get("outcome") == "CALLBACK_OBSERVED_BY_HOST", "RUNTIME_UI")
        require(callback is not None, "CALLBACK_NOT_OBSERVED")
        observed = native.differential.classify_callback(callback, {"id": FIXTURE, "name": FIXTURE, "expected": None}, nonce)
        record(self.folder / "original-differential-classification.json", {"status": observed, "expected": None,
            "scope": "Original OBSERVED_ONLY classification is preserved; separate storage oracle follows."})
        require(observed == "OBSERVED_ONLY", "ORIGINAL_CALLBACK_CLASSIFICATION")
        original(self.folder / "trace-payload.original.txt", callback.get("tracePayload", "").encode("utf-8"))
        shell = read(self.repo / "scripts/ios27_shortcuts_simulator_qa.sh").decode("utf-8")
        marker = shell.index('      RUNTIME_EVAL="$(python3 - ')
        begin = shell.index("<<'PY'\n", marker) + len("<<'PY'\n")
        evaluator = shell[begin:shell.index("\nPY\n", begin)]
        require(sha(evaluator.encode()) == RUNTIME_EVALUATOR_SHA256, "ORIGINAL_RUNTIME_EVALUATOR_BYTES")
        original(self.folder / "runtime-evaluator.original.py", evaluator.encode())
        _, raw, _ = self.capture("original-storage-intake", [sys.executable, "-B", "-c", evaluator,
            str(self.folder / "callback-events.original.json"), nonce, FIXTURE, EXPECTED,
            str(self.folder / "runtime-trace.derived.jsonl"), str(self.root / "compiler-manifest.original.json"), COMPILER], 20)
        intake = native.launch.strict_json(raw)
        record(self.folder / "original-storage-intake.original.json", intake)
        require(intake.get("runGate") == "PASS" and intake.get("traceGate") == "PASS", "ORIGINAL_STORAGE_INTAKE")
        rows = exact_storage_trace(callback, nonce)
        record(self.folder / "source-derived-trace-join.original.json", {"compilerCommitSha": COMPILER,
            "unsignedSHA256": PACKET_PINS["builds/" + FIXTURE + ".shortcut"], "records": rows,
            "tracePayloadSHA256": sha(callback["tracePayload"].encode()), "formalAcceptanceGranted": False})
        self.summary.update(storageGoalsMatched=True, actualTraceRecords=4, callbackSource="original Receiver event store")

    def run(self):
        try:
            self.check_capacity()
            self.start_capacity_watch()
            self.before = native.source_inventory(self.repo, self.plan["controllerCommitSha"])
            record(self.folder / "source.before.original.json", self.before)
            self.packet_before = packet_inventory(self.root)
            record(self.folder / "packet.before.original.json", self.packet_before)
            self.build = verify_build(self.plan, self.repo, self.before)
            record(self.folder / "build-binding.original.json", self.build)
            self.product_inventory = native.test_products_inventory(self.build["xctestrun"])
            record(self.folder / "products.before.original.json", self.product_inventory)
            self.select_device()
            self.start_observer(self.build)
            self.import_original()
            self.one_runtime()
            self.summary["status"] = "OBSERVATION_ACQUIRED"
        except BaseException as error:
            self.summary.update(errorType=type(error).__name__, error=str(error)[:1024])
        finally:
            signal.signal(signal.SIGUSR1, signal.SIG_IGN)
            signal.alarm(0)
            self.capacity_stop.set()
            if self.capacity_thread is not None:
                self.capacity_thread.join(timeout=2)
            errors = []
            if self.summary["runtimeDispatchCount"] and not (self.folder / "callback-events.original.json").exists():
                try:
                    if self.documents is None:
                        _, raw, _ = self.capture("failure-receiver-container", ["/usr/bin/xcrun", "simctl", "get_app_container",
                            native.DEVICE, native.launch.RECEIVER, "data"], 15)
                        self.documents = native.launch.owned_root(Path(raw.decode().strip()) / "Documents")
                    callback_path = self.documents / "callback-events.json"
                    if callback_path.is_file():
                        original(self.folder / "callback-events.partial.original.json", read(callback_path))
                except BaseException as error:
                    errors.append("partial-callback:" + type(error).__name__ + ":" + str(error)[:512])
            try:
                self.finish()
            except BaseException as error:
                errors.append("cleanup:" + type(error).__name__ + ":" + str(error)[:512])
            if self.summary["status"] == "OBSERVATION_ACQUIRED" and not errors:
                try:
                    native.validate_xctest(read(self.folder / "xctest.log"), self.xctest.returncode)
                except BaseException as error:
                    errors.append("xctest:" + type(error).__name__ + ":" + str(error)[:512])
            for label, before, produce in (
                ("source", self.before, lambda: native.source_inventory(self.repo, self.plan["controllerCommitSha"])),
                ("packet", self.packet_before, lambda: packet_inventory(self.root)),
                ("products", self.product_inventory, lambda: native.test_products_inventory(self.build["xctestrun"]))):
                if before is not None:
                    try:
                        after = produce()
                        record(self.folder / (label + ".after.original.json"), after)
                        require(before == after, label.upper() + "_CHANGED")
                    except BaseException as error:
                        errors.append(label + ":" + type(error).__name__ + ":" + str(error)[:512])
            if errors:
                self.summary.update(status="FAIL", finalizationErrors=errors)
            elif self.summary["status"] == "OBSERVATION_ACQUIRED":
                self.summary["appleExecutionAuthenticated"] = True
            self.summary.update(minimumFreeBytes=self.minimum_free, liveFloorBytes=native.LIVE_FLOOR_BYTES)
            record(self.folder / "result.original.json", self.summary)
        return self.summary


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--plan", required=True)
    args = parser.parse_args()
    plan = validate_plan(native.launch.strict_json(read(args.plan, 65536)))
    def deadline(signum, frame):
        raise TimeoutError("STORAGE_OBSERVER_WHOLE_DEADLINE")
    def capacity(signum, frame):
        raise RuntimeError("STORAGE_OBSERVER_CAPACITY_LIVE_FLOOR")
    def terminated(signum, frame):
        raise KeyboardInterrupt("STORAGE_OBSERVER_TERMINATED")
    signal.signal(signal.SIGTERM, terminated)
    signal.signal(signal.SIGALRM, deadline)
    signal.signal(signal.SIGUSR1, capacity)
    signal.alarm(native.OBSERVATION_SECONDS)
    try:
        result = StorageObservation(plan).run()
    finally:
        signal.alarm(0)
    print(json.dumps({k: result.get(k) for k in ("status", "storageGoalsMatched", "runtimeDispatchCount",
        "importTransportAuthenticated", "appleExecutionAuthenticated", "formalAcceptanceGranted", "error", "finalizationErrors")}, sort_keys=True))
    return 0 if result["status"] == "OBSERVATION_ACQUIRED" else 1


if __name__ == "__main__":
    raise SystemExit(main())
