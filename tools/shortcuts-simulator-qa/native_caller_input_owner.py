"""Closed FN04 native owner using the existing source, XCTest and import guards.

Build, compiler/export and controller identities are deliberately separate.
A successful observation supplies raw caller-input transport evidence only;
collection types, Void semantics and formal acceptance are never inferred.
"""
from __future__ import annotations

import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import signal
import subprocess
import sys
import threading
import time

import function_envelope_observer as native
import native_caller_input_observation as caller
import native_caller_execution_context as execution
import native_contract_fixtures as fixtures
import owned_simulator_selection as selection
import project_prerequisite_observation as capture
import shortcut_library_identity as library
import source_binding_observation as source_binding
from simulator_session_recovery import defer_cancellation_signals

SCHEMA = "cherri-native-caller-input-owner/1"
PLAN_SCHEMA = "cherri-native-caller-input-plan/1"
PLAN_KEYS = frozenset(("schema", "repo", "root", "controllerCommitSha",
    "compilerCommitSha", "builtHarnessCommitSha", "buildRoot", "buildResultSHA256",
    "bundleRoot", "bundleIntakeSHA256", "runId", "runNonce", "caseId",
    "deviceRecord", "estimatedIncrementalBytes", "pythonExecutableSHA256"))
PROJECT = "tools/shortcuts-simulator-qa/XcodeProject/"
WHOLE_SECONDS = 1200
FINAL_RESERVE_SECONDS = 300
LIVE_FLOOR_BYTES = native.LIVE_FLOOR_BYTES
BUILD_LABELS = ("data-free", "vm-stat", "xcode-version", "available-runtimes",
                "existing-devices", "xcodegen", "xcodebuild",
                "codesign-verify-receiver", "codesign-verify-runner")
BUNDLE_INPUTS = frozenset(("canonical-go.stdout.original", "packet-go.stdout.original",
    "packet-context.original.json", "producer-result.original.json",
    "canonical-intake.original.json"))
CRITICAL = (
    "native_caller_input_owner.py", "native_caller_execution_context.py",
    "native_caller_input_observation.py",
    "native_contract_fixtures.py", "function_envelope_observer.py",
    "function_envelope_launch.py", "function_envelope_runtime.py",
    "owned_simulator_selection.py", "project_prerequisite_observation.py",
    "shortcut_library_identity.py", "source_binding_observation.py",
    "simulator_session_recovery.py", "simulator_phase_boundary.py",
    "run_differential_batch.py",
)


def require(value, code):
    if not value:
        raise ValueError(code)


def sha(raw):
    return hashlib.sha256(raw).hexdigest()


def read(path, limit=16 * 1024 * 1024):
    return capture.bound_read(Path(path), limit)[0]


def original(path, raw):
    with Path(path).open("xb") as stream:
        stream.write(raw)
        stream.flush()
        os.fsync(stream.fileno())
    return {"bytes": len(raw), "sha256": sha(raw)}


def record(path, value):
    return original(path, library.canonical(value) + b"\n")


def pinned(path, pin, limit=16 * 1024 * 1024):
    require(type(pin) is dict and set(pin) == {"bytes", "sha256"}
            and type(pin["bytes"]) is int and 0 <= pin["bytes"] <= limit
            and type(pin["sha256"]) is str and
            re.fullmatch("[0-9a-f]{64}", pin["sha256"]) is not None, "ORIGINAL_PIN")
    raw = read(path, limit)
    require(len(raw) == pin["bytes"] and sha(raw) == pin["sha256"], "ORIGINAL_BYTES_CHANGED")
    return raw


def validate_plan(plan, execution_context=None):
    require(type(plan) is dict and set(plan) == PLAN_KEYS and plan["schema"] == PLAN_SCHEMA,
            "CLOSED_NATIVE_PLAN")
    for key in ("controllerCommitSha", "compilerCommitSha", "builtHarnessCommitSha"):
        require(type(plan[key]) is str and re.fullmatch("[0-9a-f]{40}", plan[key]) is not None,
                "PLAN_SOURCE_IDENTITY")
    for key in ("buildResultSHA256", "bundleIntakeSHA256", "pythonExecutableSHA256"):
        require(type(plan[key]) is str and re.fullmatch("[0-9a-f]{64}", plan[key]) is not None,
                "PLAN_ORIGINAL_PIN")
    require(type(plan["runId"]) is str and re.fullmatch("[A-Za-z0-9_-]{1,64}", plan["runId"]) is not None,
            "LOCAL_RUN_ID")
    require(type(plan["runNonce"]) is str and re.fullmatch("[0-9a-f]{32}", plan["runNonce"]) is not None
            and plan["runNonce"] != "0" * 32, "FRESH_NATIVE_NONCE")
    require(type(plan["caseId"]) is str and plan["caseId"] in caller.PREFIXES, "CLOSED_FN04_CASE")
    roots = {}
    for key in ("repo", "root", "buildRoot", "bundleRoot"):
        value = plan[key]
        require(type(value) is str and
                (value.startswith("/Users/m1/") if execution_context is None else Path(value).is_absolute()) and
                len(value.encode()) <= 4096 and "\n" not in value and
                ".." not in Path(value).parts, "OWNED_LOCAL_PATH")
        roots[key] = Path(value)
    require(len(set(roots.values())) == 4 and all(
        left not in right.parents for a, left in roots.items()
        for b, right in roots.items() if a != b), "SEPARATE_NATIVE_NAMESPACES")
    if execution_context is not None:
        execution.validate_plan_paths(plan, execution_context)
    owner = plan["deviceRecord"]
    require(type(owner) is dict and set(owner) == selection.RECORD_FIELDS and
            owner["schema"] == selection.SCHEMA and
            owner["sourceCommit"] == plan["controllerCommitSha"] and
            owner["runId"] == plan["runId"], "DEVICE_OWNERSHIP_CONTEXT")
    require(type(plan["estimatedIncrementalBytes"]) is int and
            0 < plan["estimatedIncrementalBytes"] <= 1024**3, "MEASURED_PHASE_ESTIMATE")
    return plan


def validate_build_metadata(value, before, after, plan, current_project):
    require(type(value) is dict and value.get("schema") == "cherri-native-caller-standalone-build/1"
            and value.get("status") == "BUILD_VALIDATED"
            and value.get("builtHarnessCommitSha") == plan["builtHarnessCommitSha"]
            and value.get("compiledInputEquivalentSource") == plan["compilerCommitSha"],
            "ACTUAL_BUILD_SOURCE_BINDING")
    require(value.get("sourceUnchanged") is True and value.get("generatedProjectRetained") is True
            and all(value.get(k) is False for k in ("nativeInvocation", "testsExecuted", "compilerRebuilt"))
            and value.get("firstInterruption") is None and value.get("error") is None
            and value.get("sourceAfterError") is None, "ACTUAL_BUILD_QUALIFIED")
    require(value.get("wholeOwnerSeconds") == 780 and value.get("buildSeconds") == 600,
            "ORIGINAL_BUILD_BOUNDS")
    require(before == after and before.get("commit") == plan["builtHarnessCommitSha"]
            and before.get("tree") == value.get("builtHarnessTree")
            and before.get("completeTrackedSource") is True
            and before.get("criticalSource", {}).get("sourceClean") is True,
            "BUILD_COMPLETE_CLEAN_SOURCE")
    project = {name: pin for name, pin in before["files"].items() if name.startswith(PROJECT)}
    require(len(project) == value.get("projectInputCount") == 15
            and project == current_project, "ALL_COMPILED_INPUTS_EQUIVALENT")
    processes = value.get("processes")
    require(type(processes) is list and [p.get("label") for p in processes] == list(BUILD_LABELS),
            "EXACT_BUILD_PROCESS_INVENTORY")
    for process in processes:
        observed, details = process.get("observation", {}), process.get("process", {})
        require(observed.get("result") == "PASS" and type(observed.get("exitCode")) is int
                and observed["exitCode"] == 0 and details.get("cleanupParentReaped") is True
                and details.get("groupPresentAfterCleanup") is False
                and process.get("firstInterruption") is None, "BUILD_PROCESS_ORIGINAL_PASS")
        for side in ("stdout", "stderr"):
            require(details.get(side, {}).get("eof") is True
                    and details[side].get("overflow") is False, "COMPLETE_BUILD_STREAM")
    return project


def bind_build(plan, current_source, execution_context=None):
    root = native.launch.owned_root(plan["buildRoot"])
    raw = read(root / "result.original.json")
    require(sha(raw) == plan["buildResultSHA256"], "BUILD_RESULT_PIN")
    value = library.strict_json(raw)
    if execution_context is not None:
        execution.validate_build_origin(value, execution_context)
    before_raw = read(root / "source-before/snapshot.original.json")
    after_raw = read(root / "source-after/snapshot.original.json")
    require(sha(before_raw) == value["sourceBeforeSHA256"] and
            sha(after_raw) == value["sourceAfterSHA256"], "BUILD_SOURCE_ORIGINALS")
    before, after = map(library.strict_json, (before_raw, after_raw))
    current_project = {name: pin for name, pin in current_source["files"].items()
                       if name.startswith(PROJECT)}
    project = validate_build_metadata(value, before, after, plan, current_project)
    for process in value["processes"]:
        folder = root / "processes" / process["label"]
        require(library.strict_json(read(folder / "result.original.json")) == process,
                "BUILD_PROCESS_RECEIPT")
        for side in ("stdout", "stderr"):
            details = process["process"][side]
            pinned(folder / (side + ".original"),
                   {"bytes": details["bytes"], "sha256": details["sha256"]})
    expected = library.strict_json(read(root / "compiled-inputs.expected.original.json"))
    require(expected == {"source": plan["compilerCommitSha"], "files": project} and
            library.strict_json(read(root / "compiled-inputs.actual.original.json")) == project,
            "BUILD_COMPILED_ORIGINALS")
    products = library.strict_json(pinned(root / "build-products.original.json", value["productInventory"]))
    image = library.strict_json(pinned(root / "receiver-image.original.json", value["receiverImage"]))
    xct, app = Path(value["xctestrun"]), Path(value["receiverApp"])
    require(xct.is_absolute() and app.is_absolute() and
            (str(xct).startswith("/Users/m1/caches/DerivedData/") if execution_context is None
             else execution.product_scope(xct, execution_context)) and
            xct.parent == Path(products["actualProductRoot"]) and
            app == xct.parent / "Debug-iphonesimulator/CherriSimulatorQAReceiver.app",
            "CURRENT_PRODUCT_SCOPE")
    require(native.test_products_inventory(xct) == products and
            native.differential.receiver_image_inventory(app) == image, "CURRENT_BUILT_PRODUCTS")
    return {"receiverApp": str(app), "xctestrun": str(xct), "products": products,
            "receiverImage": image, "compiledProject": project,
            "builtHarnessCommitSha": plan["builtHarnessCommitSha"],
            "compilerCommitSha": plan["compilerCommitSha"],
            "controllerCommitSha": plan["controllerCommitSha"],
            "buildResultSHA256": sha(raw)}


def bind_packet_bundle(plan):
    root = native.launch.owned_root(plan["bundleRoot"])
    raw = read(root / "intake.original.json")
    require(sha(raw) == plan["bundleIntakeSHA256"], "PACKET_INTAKE_PIN")
    intake = library.strict_json(raw)
    require(intake.get("schema") == "cherri-native-caller-packet-bundle/1" and
            intake.get("status") == "PASS_BOUND_THREE_PACKETS_FOUR_EVENTS" and
            intake.get("compilerCommitSha") == plan["compilerCommitSha"]
            and intake.get("packetCount") == 3 and intake.get("namedPasses") == 4
            and intake.get("sourceUnchanged") is True and
            intake.get("allParentsReapedAndGroupsAbsent") is True
            and all(intake.get(k) is False for k in
                    ("nativeExecuted", "rawNativeTypesObserved", "formalAcceptanceGranted")),
            "QUALIFIED_PACKET_INTAKE")
    require(intake.get("goTestInternalSeconds") == 180 and
            intake.get("compilationInclusiveChildSeconds") == 600 and
            intake.get("wholeOwnerSeconds") == 1200 and
            type(intake.get("inputs")) is dict and set(intake["inputs"]) == BUNDLE_INPUTS,
            "PACKET_EXECUTION_BOUNDS_AND_INPUTS")
    inputs = {name: pinned(root / name, pin, fixtures.MAX_JSONL)
              for name, pin in intake["inputs"].items()}
    canonical_intake = library.strict_json(inputs["canonical-intake.original.json"])
    require(sha(inputs["canonical-intake.original.json"]) == intake["canonicalQualificationSHA256"]
            and canonical_intake.get("status") == "PASS_53_ORIGINAL_COMPILED_FIXTURES_57_EVENTS",
            "QUALIFIED_CANONICAL_ORIGINAL")
    canonical = fixtures.parse_exports(inputs["canonical-go.stdout.original"],
        expected_sha256=canonical_intake["goJSONLSha256"])
    context = library.strict_json(inputs["packet-context.original.json"])
    producer = library.strict_json(inputs["producer-result.original.json"])
    require(context["source"] == producer["source"] == plan["compilerCommitSha"]
            and context["tree"] == producer["tree"] == intake["compilerTree"]
            and context["controller"] == producer["controller"] == intake["exportControllerCommitSha"]
            and context["runId"] == producer["runId"] == intake["exportRunId"]
            and context["attempt"] == producer["attempt"] == intake["exportAttempt"] == 1
            and context["packetToken"] == intake["packetToken"]
            and context["environmentOverride"] == {"CHERRI_NATIVE_PACKET_TOKEN": intake["packetToken"]}
            and context["argv"] == list(caller.PACKET_COMMAND), "ACTUAL_EXPORT_CONTEXT")
    rows = [p for p in producer["processes"]
            if p["argv"] == list(caller.PACKET_COMMAND)]
    require(len(rows) == 1 and producer["status"] == "PASS_GO_PACKET_TESTS_PENDING_CANONICAL_INTAKE",
            "ONE_ORIGINAL_PACKET_EXPORT")
    process = rows[0]
    require(process["status"] == "PASS" and process["exitCode"] == 0 and
            process["commandSeconds"] == 600 and process["stdout"]["complete"] is True
            and process["stderr"]["complete"] is True
            and process["cleanup"]["parentReaped"] is True
            and process["cleanup"]["after"]["state"] == "ABSENT", "PACKET_EXPORT_PROCESS")
    packets = caller.parse_packets(inputs["packet-go.stdout.original"],
        expected_sha256=process["stdout"]["sha256"], canonical_exports=canonical,
        packet_token=intake["packetToken"])
    selected = [v for v in packets if v["caseId"] == plan["caseId"]]
    require(len(selected) == 1, "EXACT_NATIVE_CASE")
    packet = selected[0]
    case = fixtures.CASE_BY_TEST[fixtures.EXPORT_PARENT + "/FN04/" + plan["caseId"]]
    canonical_fixture = canonical[fixtures.CASES.index(case)]
    checked = caller.validate_packet(packet, canonical_fixture)
    manifest = [v for v in intake["packets"] if v["caseId"] == plan["caseId"]]
    require(len(manifest) == 1 and manifest[0]["workflowName"] == checked["name"], "PACKET_CASE_MANIFEST")
    entry, directory = manifest[0], root / plan["caseId"]
    require(library.strict_json(pinned(directory / "packet.derived.json", entry["packet"])) == packet
            and pinned(directory / "source.original.cherri", entry["source"]) == checked["source"]
            and pinned(directory / "unsigned.original.plist", entry["unsigned"]) == checked["unsigned"],
            "EXACT_PACKET_SOURCE_AND_BYTES")
    return packet, canonical_fixture, checked, {"intakeSHA256": sha(raw),
        "exportRunId": intake["exportRunId"], "exportControllerCommitSha": intake["exportControllerCommitSha"],
        "canonicalQualificationSHA256": intake["canonicalQualificationSHA256"]}


class InterruptionLatch:
    """Preserve the first interruption while the qualified capture drains."""
    def __init__(self):
        self.reason = None

    def interrupt(self, reason):
        if self.reason is not None:
            return
        self.reason = reason
        signal.setitimer(signal.ITIMER_REAL, 0)
        raise TimeoutError(reason)


def start_registered_xctest(owner, argv, env):
    """Do not allow cancellation between Popen and storing the owned handle."""
    with defer_cancellation_signals():
        owner.xctest = subprocess.Popen(argv, stdout=owner.test_log,
            stderr=subprocess.STDOUT, stdin=subprocess.DEVNULL, env=env,
            start_new_session=True)
        record(owner.folder / "xctest-process.started.original.json",
            {"argv": argv, "pid": owner.xctest.pid, "startupTimeoutSeconds": 120,
             "startedUTC": library.now(), "ownerWholeSeconds": WHOLE_SECONDS,
             "privateResultBundle": str(owner.folder / "NativeCaller.xcresult")})


class NativeCallerObservation:
    def __init__(self, plan, execution_context=None):
        self.execution_context = (execution.validate_context(execution_context)
                                  if execution_context is not None else None)
        self.plan = validate_plan(plan, self.execution_context)
        require(sys.platform == "darwin" and sys.version_info >= (3, 10),
                "DARWIN_SHARED_MONOTONIC_CLOCK")
        require(sha(Path(sys.executable).resolve(strict=True).read_bytes()) ==
                plan["pythonExecutableSHA256"], "CONSISTENT_EXISTING_PYTHON")
        if self.execution_context is None:
            require(not any(value for key, value in os.environ.items() if key.startswith("GITHUB_")),
                    "TRUTHFUL_LOCAL_PROVENANCE")
        self.repo = native.launch.owned_root(plan["repo"])
        self.root = Path(plan["root"])
        native.launch.owned_root(self.root.parent)
        self.root.mkdir(mode=0o700)
        self.folder = self.root / "runtime"
        self.folder.mkdir(mode=0o700)
        self.process_folder = self.folder / "processes"
        self.process_folder.mkdir(mode=0o700)
        self.device = plan["deviceRecord"]["udid"]
        self.device_data_path = None
        self.xctest = self.test_log = self.documents = self.build = None
        self.before = self.packet = self.canonical_fixture = self.checked = None
        self.products_before = None
        self.unsigned = self.signed = self.signed_pin = None
        self.caller_entered = False
        self.admission_verified = False
        self.started = time.monotonic()
        self.hard_deadline = self.started + WHOLE_SECONDS
        self.deadline = self.hard_deadline - FINAL_RESERVE_SECONDS
        self.latch = InterruptionLatch()
        self.stop = threading.Event()
        self.watcher = None
        self.minimum_free = None
        self.capacity_reason = None
        self.summary = {"schema": SCHEMA, "status": "FAIL", "plan": plan,
            "startedUTC": library.now(), "executionLocation": "EXISTING_LOCAL_MAC",
            "wholeOwnerSeconds": WHOLE_SECONDS, "finalReserveSeconds": FINAL_RESERVE_SECONDS,
            "runtimeObservationAuthenticated": False, "actualRuntimeDispatches": None,
            "processes": [],
            "rawNativeTypesObserved": False, "formalAcceptanceGranted": False,
            "compilerRebuilt": False, "harnessRebuilt": False}
        if self.execution_context is not None:
            self.summary["executionLocation"] = "GITHUB_ACTIONS_MACOS"
            self.summary["hostedContext"] = self.execution_context
            record(self.root / "execution-context.original.json", self.execution_context)
        record(self.root / "plan.original.json", plan)

    def check_capacity(self):
        free = shutil.disk_usage("/System/Volumes/Data").free
        self.minimum_free = free if self.minimum_free is None else min(self.minimum_free, free)
        require(free >= LIVE_FLOOR_BYTES, "ORIGINAL_NATIVE_LIVE_FLOOR")
        return free

    def capture(self, label, argv, seconds, *, require_pass=True):
        self.check_capacity()
        require(time.monotonic() < self.deadline, "NATIVE_EXECUTION_RESERVE")
        folder = self.process_folder / label
        folder.mkdir(mode=0o700)
        observed, details, last = None, None, {}
        def journal(event, value):
            last.clear()
            last.update(value)
            record(folder / ("process-" + event + ".original.json"), value)
        try:
            with (folder / "stdout.original").open("xb") as out, (folder / "stderr.original").open("xb") as err:
                observed, details = capture.capture(argv, seconds, output=out, error_output=err,
                    hard_deadline=min(self.deadline, time.monotonic() + seconds + 5),
                    journal=journal)
        finally:
            receipt = {"argv": argv, "observation": observed,
                "details": details or last, "firstInterruption": self.latch.reason}
            pin = record(folder / "process.original.json", receipt)
            self.summary["processes"].append({"label": label, "receipt": pin})

        require(details.get("cleanupParentReaped") is True
                and details.get("groupPresentAfterCleanup") is False, "OWNED_PROCESS_CLEANUP")
        require(all(details.get(side, {}).get("eof") is True and
                    details[side].get("overflow") is False for side in ("stdout", "stderr")),
                "COMPLETE_OWNED_PROCESS_STREAMS")
        if require_pass:
            require(observed.get("result") == "PASS" and type(observed.get("exitCode")) is int
                    and observed["exitCode"] == 0, "PROCESS_" + label)
        self.check_capacity()
        return observed, read(folder / "stdout.original"), read(folder / "stderr.original")

    def start_capacity_watch(self):
        def watch():
            while not self.stop.wait(1):
                try:
                    self.check_capacity()
                    path = self.folder / "xctest.log"
                    require(not path.exists() or path.stat().st_size <= native.MAX_ORIGINAL,
                            "ORIGINAL_XCTEST_LOG_BOUND")
                except BaseException as error:
                    self.capacity_reason = type(error).__name__ + ":" + str(error)
                    os.kill(os.getpid(), signal.SIGTERM)
                    return
        self.watcher = threading.Thread(target=watch, daemon=True)
        self.watcher.start()

    def select_device(self):
        folder = self.folder / "device"
        folder.mkdir(mode=0o700)
        originals = {}
        for part in ("runtimes", "devicetypes", "devices"):
            _, raw, _ = self.capture("inventory-" + part,
                ["/usr/bin/xcrun", "simctl", "list", part, "--json"], 15)
            original(folder / ("sim-" + part + ".json"), raw)
            originals[part] = library.strict_json(raw)
        selected = selection.select_owned(self.plan["deviceRecord"],
            originals["runtimes"], originals["devicetypes"], originals["devices"],
            self.plan["controllerCommitSha"], self.plan["runId"], "iphone-fallback")
        record(folder / "owned-selection.original.json", selected)
        device = [d for values in originals["devices"]["devices"].values()
                  for d in values if d.get("udid") == self.device]
        require(len(device) == 1 and type(device[0].get("dataPath")) is str,
                "CURRENT_DEVICE_DATA_PATH")
        self.device_data_path = device[0]["dataPath"]
        # This is the unchanged same-device pre-import owner, including its
        # original source, reset/boot/readiness and owned-process checks.
        self.capture("session-pre-import",
            [sys.executable, "-B",
             str(self.repo / "tools/shortcuts-simulator-qa/simulator_phase_boundary.py"),
             "--repo", str(self.repo), "--root", str(self.folder / "session-pre-import"),
             "--udid", self.device, "--source-sha", self.plan["controllerCommitSha"],
             "--run-id", self.plan["runId"], "--attempt", "1",
             "--fixture-id", "fn04-" + self.plan["caseId"], "--phase", "pre-import"], 310)

    def sign_original(self):
        self.packet, self.canonical_fixture, self.checked, evidence = bind_packet_bundle(self.plan)
        record(self.folder / "packet-binding.original.json", evidence)
        signed_root = self.folder / "signed"
        signed_root.mkdir(mode=0o700)
        unsigned = signed_root / (self.checked["name"] + ".unsigned.shortcut")
        signed = signed_root / (self.checked["name"] + ".shortcut")
        original(unsigned, self.checked["unsigned"])
        self.unsigned, self.signed = unsigned, signed
        require(Path("/usr/bin/shortcuts").is_file(), "EXISTING_SHORTCUTS_SIGNER")
        self.capture("sign-original-packet",
            ["/usr/bin/shortcuts", "sign", "--mode", "anyone",
             "--input", str(unsigned), "--output", str(signed)], 120)
        signed_raw = read(signed, library.MAX_FILE)
        require(signed_raw.startswith(b"AEA1") and read(unsigned) == self.checked["unsigned"],
                "ACTUAL_SIGNED_ORIGINAL_PACKET")
        self.unsigned, self.signed = unsigned, signed
        self.signed_pin = {"bytes": len(signed_raw), "sha256": sha(signed_raw)}
        record(self.folder / "signing.original.json",
            {"schema": "cherri-native-caller-signing/1", "status": "SIGNED_ORIGINAL",
             "compilerCommitSha": self.plan["compilerCommitSha"],
             "packetToken": self.checked["token"], "workflowName": self.checked["name"],
             "method": "MACOS_HOST_SHORTCUTS_CLI", "unsignedSHA256": self.checked["unsignedSha256"],
             "signed": self.signed_pin, "compilePerformed": False,
             "formalAcceptanceGranted": False})

    def start_observer(self):
        argv = ["/usr/bin/xcodebuild", "test-without-building",
            "-xctestrun", self.build["xctestrun"],
            "-destination", "platform=iOS Simulator,id=" + self.device,
            "-parallel-testing-enabled", "NO", "-only-testing:" + native.TEST,
            "-resultBundlePath", str(self.folder / "NativeCaller.xcresult")]
        env = os.environ.copy()
        env["TEST_RUNNER_CHERRI_QA_DIFF_RUN_DIR"] = str(self.folder)
        self.test_log = (self.folder / "xctest.log").open("x")
        start_registered_xctest(self, argv, env)
        ready = native.differential.wait_file(self.folder / "ready.json", 120, self.xctest)
        require(ready == {"ready": True}, "ORIGINAL_XCTEST_READY")
        self.capture("receiver-install",
            ["/usr/bin/xcrun", "simctl", "install", self.device, self.build["receiverApp"]], 90)
        _, raw, _ = self.capture("receiver-app-container",
            ["/usr/bin/xcrun", "simctl", "get_app_container", self.device,
             native.launch.RECEIVER, "app"], 25)
        installed = Path(raw.decode().strip())
        identity = native.differential.receiver_image_identity(Path(self.build["receiverApp"]), installed)
        record(self.folder / "receiver-image-identity.original.json", identity)
        require(identity["outcome"] == "MATCH", "INSTALLED_RECEIVER_IMAGE")
        self.capture("receiver-fresh-launch",
            ["/usr/bin/xcrun", "simctl", "launch", "--terminate-running-process",
             self.device, native.launch.RECEIVER], 60)
        _, raw, _ = self.capture("receiver-data-container",
            ["/usr/bin/xcrun", "simctl", "get_app_container", self.device,
             native.launch.RECEIVER, "data"], 25)
        self.documents = native.launch.owned_root(Path(raw.decode().strip()) / "Documents")
        self.admission_verified = True

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
            self.build = (bind_build(self.plan, self.before) if context is None else
                          bind_build(self.plan, self.before, context))
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
            observed = caller.observe_caller_input(self, self.packet, self.canonical_fixture,
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
                        packet, canonical, checked, evidence = bind_packet_bundle(self.plan)
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


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--plan", required=True)
    args = parser.parse_args(argv)
    plan = validate_plan(library.strict_json(read(Path(args.plan))))
    owner = NativeCallerObservation(plan)
    result = owner.run()
    print(json.dumps({"status": result["status"], "runId": plan["runId"],
        "caseId": plan["caseId"], "runtimeObservationAuthenticated":
        result["runtimeObservationAuthenticated"], "formalAcceptanceGranted": False}))
    return 0 if result["status"] == "OBSERVATION_VERIFIED" else 1


if __name__ == "__main__":
    raise SystemExit(main())
