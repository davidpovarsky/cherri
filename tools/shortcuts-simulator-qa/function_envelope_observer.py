#!/usr/bin/env python3
"""One fixed function-envelope import and no-input native observation.

This adapter reuses the original Differential XCTest and process helpers.
Compiler, receiver build, and controller source identities remain distinct.
It never recompiles, signs, substitutes a device, retries a semantic launch,
or supplies formal RP26/BRG15 acceptance.
"""
import argparse
import hashlib
import json
import os
import plistlib
import shutil
from pathlib import Path
import re
import signal
import stat
import subprocess
import sys
import threading
import time
import uuid

import function_envelope_launch as launch
import function_envelope_runtime as runtime
import owned_simulator_selection as selection
import project_prerequisite_observation as capture
import run_differential_batch as differential

BUILD_SOURCE = "2a49baac855632c3deec2ec1918e3c43b9cf3376"
DEVICE = "D631D98E-2271-4948-B465-8EDBA9BD4D68"
PROJECT = "tools/shortcuts-simulator-qa/XcodeProject/"
TEST = "CherriSimulatorQAUITests/CherriDifferentialBatchUITests/testDifferentialImportsAndRuntimeConsent"
TEST_LABEL = "CherriSimulatorQAUITests.CherriDifferentialBatchUITests testDifferentialImportsAndRuntimeConsent"
PLAN_KEYS = {"schema", "repo", "root", "controllerCommitSha", "builtHarnessCommitSha",
             "compilerCommitSha", "receiverBuildRoot", "receiverBuildQualificationSHA256",
             "udid", "runId", "runAttempt", "runNonce"}
MAX_ORIGINAL = 16 * 1024 * 1024
BUILD_QUALIFICATION = "064973dd92a2aa6f0fefb45513254387eee1ec5f409b944462fa350884cabf3e"
OBSERVATION_SECONDS = 1200
LIVE_FLOOR_BYTES = 4 * 1024 ** 3
SIGNING_RECEIPT = "f082a8a1113102c6b086bcab05b59a96fc022300b033b3ad13fce307845923e5"
SIGNED_SHA256 = "e96cfcfa00fecb875345674ce5c61272ab8d3d5de3a0d78d610d0bb35b04e594"


def require(value, code):
    if not value:
        raise ValueError("FUNCTION_OBSERVER_" + code)


def sha(raw):
    return hashlib.sha256(raw).hexdigest()


def validate_plan(value):
    require(isinstance(value, dict) and set(value) == PLAN_KEYS, "PLAN_FIELDS")
    require(value["schema"] == "cherri-function-envelope-native-observer-plan/1"
            and value["compilerCommitSha"] == runtime.COMPILER_SOURCE
            and value["builtHarnessCommitSha"] == BUILD_SOURCE
            and value["udid"] == DEVICE, "PLAN_IDENTITIES")
    require(isinstance(value["controllerCommitSha"], str)
            and re.fullmatch(r"[a-f0-9]{40}", value["controllerCommitSha"])
            and isinstance(value["receiverBuildQualificationSHA256"], str)
            and value["receiverBuildQualificationSHA256"] == BUILD_QUALIFICATION, "PLAN_DIGESTS")
    for key, maximum in (("runId", 64), ("runNonce", 128)):
        require(isinstance(value[key], str)
                and re.fullmatch(r"[A-Za-z0-9_-]{1,%d}" % maximum, value[key]), "PLAN_RUN")
    require(type(value["runAttempt"]) is int and 1 <= value["runAttempt"] <= 100, "PLAN_ATTEMPT")
    for key in ("repo", "root", "receiverBuildRoot"):
        path = value[key]
        require(isinstance(path, str) and path.startswith("/Users/m1/")
                and len(path.encode()) <= 4096 and "\n" not in path
                and ".." not in Path(path).parts, "PLAN_PATH")
    paths = {key: Path(value[key]) for key in ("repo", "root", "receiverBuildRoot")}
    require(len(set(paths.values())) == 3 and all(
        left not in right.parents for a, left in paths.items() for b, right in paths.items() if a != b),
        "PLAN_PATH_ALIAS")
    return value


def original(path, raw):
    path = Path(path)
    require(isinstance(raw, bytes) and len(raw) <= MAX_ORIGINAL, "ORIGINAL_SIZE")
    launch.owned_root(path.parent)
    with path.open("xb") as f:
        os.chmod(path, 0o600)
        f.write(raw)
        f.flush()
        os.fsync(f.fileno())
    require(path.read_bytes() == raw, "ORIGINAL_READBACK")


def record(path, value):
    original(path, launch.canonical(value) + b"\n")


def read(path, maximum=MAX_ORIGINAL):
    return launch.regular(path, maximum, empty=False)


def git(repo, *args):
    env = {k: v for k, v in os.environ.items() if not k.startswith("GIT_")}
    env.update(GIT_NO_REPLACE_OBJECTS="1", GIT_CONFIG_NOSYSTEM="1",
               GIT_CONFIG_GLOBAL="/dev/null", GIT_TERMINAL_PROMPT="0")
    return subprocess.check_output(["git", "--no-replace-objects", "-C", str(repo), *args],
                                   env=env, stdin=subprocess.DEVNULL, timeout=15)


def source_inventory(repo, commit):
    critical = launch.source_snapshot(repo, commit)
    raw = git(repo, "ls-tree", "-rz", "--full-tree", commit)
    pins = {}
    for item in raw.split(b"\0"):
        if not item:
            continue
        metadata, rel = item.split(b"\t", 1)
        mode, kind, blob = metadata.decode().split()
        require(kind == "blob", "SOURCE_KIND")
        name = os.fsdecode(rel)
        path = Path(repo) / name
        st = path.lstat()
        require(stat.S_ISREG(st.st_mode) or mode == "120000" and stat.S_ISLNK(st.st_mode), "SOURCE_FILE")
        data = os.fsencode(os.readlink(path)) if mode == "120000" else path.read_bytes()
        require(hashlib.sha1(b"blob " + str(len(data)).encode() + b"\0" + data).hexdigest() == blob,
                "SOURCE_BLOB")
        require(mode == "120000" or bool(st.st_mode & 0o111) == (mode == "100755"), "SOURCE_MODE")
        pins[name] = {"mode": mode, "gitBlob": blob, "bytes": len(data), "sha256": sha(data)}
    return {"commit": commit, "tree": critical["tree"], "criticalSource": critical,
            "completeTrackedSource": True, "files": pins}


def validate_import(dispatch, receipt, argv, phase):
    require(isinstance(dispatch, dict) and dispatch.get("command") == argv
            and dispatch.get("outcome") == "EXITED" and dispatch.get("timedOut") is False
            and type(dispatch.get("exitCode")) is int and dispatch["exitCode"] == 0
            and type(dispatch.get("processReturncode")) is int and dispatch["processReturncode"] == 0
            and dispatch.get("parentReaped") is True and dispatch.get("timeoutSeconds") == 60,
            "IMPORT_PROCESS")
    require(isinstance(receipt, dict) and receipt.get("id") == phase
            and receipt.get("operation") == "import" and receipt.get("outcome") == "IMPORTED"
            and receipt.get("name") == launch.WORKFLOW
            and receipt.get("identityVerified") is True
            and receipt.get("libraryIdentityVerified") is True, "IMPORT_IDENTITY")
    timing = receipt.get("importTiming")
    require(isinstance(timing, dict) and timing.get("contract") == "dispatch-exit-anchored-add-v1"
            and timing.get("dispatchState") == "EXITED"
            and timing.get("dispatchReceipt") == "dispatch-" + phase + ".json", "IMPORT_TIMING")


def validate_xctest(raw, code):
    require(type(code) is int and code == 0, "XCTEST_EXIT")
    text = raw.decode("utf-8", errors="strict")
    start = "Test Case '-[" + TEST_LABEL + "]' started."
    terminal = re.compile(r"^Test Case '-\[" + re.escape(TEST_LABEL) + r"\]' passed \([0-9]+(?:\.[0-9]+)? seconds\)\.$",
                          re.MULTILINE)
    case_lines = re.findall(r"^Test Case .*", text, re.MULTILINE)
    require(text.count(start) == 1 and len(terminal.findall(text)) == 1
            and len(case_lines) == 2 and case_lines[0] == start
            and terminal.fullmatch(case_lines[1]) is not None, "XCTEST_EXACT_METHOD")
    require(text.index(start) < terminal.search(text).start()
            and not re.search(r"^Test Case .* (?:failed|skipped) ", text, re.MULTILINE)
            and re.search(r"Executed 1 test, with 0 failures \(0 unexpected\)", text), "XCTEST_TERMINAL")


def termination_is_stopped(observed, stderr):
    if not isinstance(observed, dict) or not isinstance(stderr, bytes):
        return False
    if observed.get("result") == "PASS" and type(observed.get("exitCode")) is int and observed["exitCode"] == 0:
        return True
    return (observed.get("result") == "EXIT_NONZERO" and type(observed.get("exitCode")) is int
            and observed["exitCode"] == 3 and b"NSPOSIXErrorDomain, code=3" in stderr
            and b"No such process" in stderr)


def verify_build(plan, repo):
    root = launch.owned_root(plan["receiverBuildRoot"])
    qraw = read(root / "qualification-01/result.original.json")
    require(sha(qraw) == plan["receiverBuildQualificationSHA256"], "BUILD_QUALIFICATION_HASH")
    q = launch.strict_json(qraw)
    require(q.get("schema") == "cherri-standalone-receiver-build-qualification/1"
            and q.get("status") == "BUILD_VALIDATED" and q.get("builtHarnessCommit") == BUILD_SOURCE
            and q.get("sourceTrackedBytesUnchanged") is True
            and q.get("sourceCleanAfterRetainedProjectRelocation") is True
            and q.get("compilerRebuilt") is False and q.get("nativeInvocation") is False,
            "BUILD_QUALIFICATION")
    raw = read(root / "result.original.json")
    require(sha(raw) == q["originalOwnerResultSHA256"], "BUILD_OWNER_HASH")
    owner = launch.strict_json(raw)
    require(owner.get("builtHarnessCommit") == BUILD_SOURCE
            and owner.get("compilerRebuilt") is False
            and owner.get("nativeInvocation") is False, "BUILD_OWNER")
    for label in ("xcodegen", "xcodebuild", "codesign-verify", "codesign-display"):
        rows = [row for row in owner["processes"] if row.get("label") == label]
        require(len(rows) == 1 and type(rows[0].get("exitCode")) is int
                and rows[0]["exitCode"] == 0 and rows[0].get("parentReaped") is True, "BUILD_PROCESS")
        for side in ("stdout", "stderr"):
            stream = launch.regular(root / (label + "." + side + ".original"), MAX_ORIGINAL, empty=True)
            require(type(rows[0][side + "Bytes"]) is int and len(stream) == rows[0][side + "Bytes"]
                    and sha(stream) == rows[0][side + "SHA256"], "BUILD_STREAM")
    before = read(root / "source-before.original.json")
    after = read(root / "qualification-01/source.after.original.json")
    require(sha(before) == q["sourceBeforeSHA256"] and sha(after) == q["sourceAfterSHA256"]
            and before == after, "BUILD_SOURCE_ORIGINALS")
    built_pins = launch.strict_json(before)["files"]
    rows = {name: row for name, row in built_pins.items() if name.startswith(PROJECT)}
    require(len(rows) == 13, "BUILD_PROJECT_INVENTORY")
    for name, row in rows.items():
        data = git(repo, "show", plan["controllerCommitSha"] + ":" + name)
        require(len(data) == row["bytes"] and sha(data) == row["sha256"]
                and read(Path(repo) / name) == data, "BUILD_CONTROLLER_EQUIVALENCE")
    product_raw = read(root / "build-products.original.json")
    image_raw = read(root / "receiver-image.original.json")
    require(sha(product_raw) == q["buildProductsSHA256"]
            and sha(image_raw) == q["receiverImageSHA256"], "BUILD_PRODUCTS")
    products, image = map(launch.strict_json, (product_raw, image_raw))
    app, xct = Path(products["receiverApp"]), Path(products["xctestrun"]["path"])
    require(differential.receiver_image_inventory(app) == image, "BUILD_RECEIVER_CHANGED")
    require(sha(read(xct)) == products["xctestrun"]["sha256"] == q["xctestrunSHA256"], "BUILD_XCTESTRUN")
    return {"receiverApp": str(app), "xctestrun": str(xct),
            "qualificationSHA256": sha(qraw), "receiverImageSHA256": sha(image_raw),
            "projectSources": rows, "builtHarnessCommitSha": BUILD_SOURCE}


def verify_signing(root):
    directory = Path(root) / "runtime/signing-originals"
    raw = read(directory / "result.original.json")
    require(sha(raw) == SIGNING_RECEIPT, "SIGNING_RECEIPT_HASH")
    result = launch.strict_json(raw)
    require(result.get("status") == "SIGNED_ORIGINAL"
            and result.get("compilerCommitSha") == runtime.COMPILER_SOURCE
            and result.get("compilePerformed") is False
            and result.get("processExitCode") == 0 and type(result.get("processExitCode")) is int
            and result.get("parentReaped") is True
            and result.get("signedSHA256") == SIGNED_SHA256
            and result.get("sourceUnchanged") is True, "SIGNING_PROCESS")
    signed = read(Path(root) / "builds/signed" / (launch.WORKFLOW + ".shortcut"))
    require(len(signed) == result["signedBytes"] and sha(signed) == SIGNED_SHA256, "SIGNED_ORIGINAL")
    for name in ("stdout.original", "stderr.original"):
        stream = launch.regular(directory / name, MAX_ORIGINAL, empty=True)
        require(result[name] == {"bytes": len(stream), "sha256": sha(stream)}, "SIGNING_STREAM")
    target = read(directory / "signing-provenance-target.original.json")
    require(sha(target) == result["targetProvenanceSHA256"], "SIGNING_TARGET")
    summary = launch.strict_json(read(Path(root) / "runtime/signing-provenance.json"))
    require(launch.canonical(summary["targetProvenance"]) == launch.canonical(launch.strict_json(target)),
            "SIGNING_SUMMARY_ORIGINAL")
    return {"actualSigningReceiptSHA256": sha(raw), "targetProvenanceSHA256": sha(target),
            "compilerCommitSha": runtime.COMPILER_SOURCE, "signerCompilerSource": result["signerCompilerSource"],
            "signedArtifactSHA256": sha(signed), "compilePerformed": False}


def test_products_inventory(xctestrun):
    """Retain the actual built products before and after this observation."""
    xctestrun = Path(xctestrun)
    config = plistlib.loads(read(xctestrun))
    require(config.get("__xctestrun_metadata__", {}).get("FormatVersion") == 2, "TEST_PRODUCTS_FORMAT")
    configs = config.get("TestConfigurations")
    require(isinstance(configs, list) and len(configs) == 1 and configs[0].get("IsEnabled") is True,
            "TEST_PRODUCTS_CONFIG")
    targets = configs[0].get("TestTargets")
    require(isinstance(targets, list) and len(targets) == 1, "TEST_PRODUCTS_TARGETS")
    target = targets[0]
    require(target.get("BlueprintName") == "CherriSimulatorQAUITests"
            and target.get("TestHostBundleIdentifier") == "org.cherrilang.qa.UITests.xctrunner"
            and target.get("IsUITestBundle") is True
            and target.get("TestHostPath") == "__TESTROOT__/Debug-iphonesimulator/CherriSimulatorQAUITests-Runner.app"
            and target.get("TestBundlePath") == "__TESTHOST__/PlugIns/CherriSimulatorQAUITests.xctest"
            and target.get("UITargetAppPath") == "__TESTROOT__/Debug-iphonesimulator/CherriSimulatorQAReceiver.app",
            "TEST_PRODUCTS_PATHS")
    root = launch.owned_root(xctestrun.parent)
    rows, total = {}, 0
    for name in ("CherriSimulatorQAUITests-Runner.app", "CherriSimulatorQAReceiver.app"):
        bundle = launch.owned_root(root / "Debug-iphonesimulator" / name)
        files = sorted(p for p in bundle.rglob("*") if p.is_file() or p.is_symlink())
        require(0 < len(files) <= 2000, "TEST_PRODUCTS_COUNT")
        for path in files:
            st = path.lstat()
            if path.is_symlink():
                resolved = path.resolve(strict=True)
                require(bundle == resolved or bundle in resolved.parents, "TEST_PRODUCTS_SYMLINK_ESCAPE")
                raw = os.fsencode(os.readlink(path))
                mode = "120000"
            else:
                raw = launch.regular(path, 128 * 1024 * 1024, empty=True)
                mode = oct(stat.S_IMODE(st.st_mode))
            total += len(raw)
            require(total <= 512 * 1024 * 1024, "TEST_PRODUCTS_TOTAL")
            rows[path.relative_to(root).as_posix()] = {"bytes": len(raw), "sha256": sha(raw), "mode": mode}
    return {"xctestrunSHA256": sha(read(xctestrun)), "actualProductRoot": str(root),
            "files": rows, "bytes": total, "scope": "current products at observation time; original build identity separately verified"}


def retain_failure_receiver(root, documents, destination):
    """Copy bounded partial originals even when the success collector cannot run."""
    destination = Path(destination)
    destination.mkdir(mode=0o700)
    rows, missing = [], []
    prepared = Path(root) / launch.RELATIVE / "contract.original.json"
    if prepared.is_file():
        contract_raw = read(prepared, launch.MAX_JSON)
        directory = Path(documents) / "primary-no-input-launch" / sha(contract_raw)
        if directory.exists():
            launch.owned_root(directory)
            candidates = sorted(directory.iterdir())
            require(len(candidates) <= 34, "FAILED_TRIGGER_COUNT")
            for path in candidates:
                require(path.name in {"owner.original.json", "contract.original.json"}
                        or re.fullmatch(r"[0-9]{3}-(?:intent|completion|rejection)\.original\.json", path.name),
                        "FAILED_TRIGGER_PATH")
                raw = launch.regular(path, launch.MAX_JSON, empty=True)
                original(destination / path.name, raw)
                rows.append({"path": path.name, "bytes": len(raw), "sha256": sha(raw)})
        else:
            missing.append("trigger-directory")
    else:
        missing.append("prepared-contract")
    callbacks = Path(documents) / "callback-events.json"
    if callbacks.is_file():
        raw = launch.regular(callbacks, runtime.MAX_CALLBACK, empty=True)
        original(destination / "callback-events.original.json", raw)
        rows.append({"path": "callback-events.original.json", "bytes": len(raw), "sha256": sha(raw)})
    else:
        missing.append("callback-events")
    value = {"schema": "cherri-function-envelope-partial-original-acquisition/1",
             "sourceDocuments": str(documents), "files": rows, "missing": missing,
             "successCollector": False, "nativeExecutionAuthenticated": False,
             "formalAcceptanceGranted": False}
    record(destination / "acquisition.original.json", value)
    return value


def execute_with_runtime_observer(repo, root, folder, phase, capacity_check):
    """Invoke the reviewed owner in this main thread; no nested Python PGID."""
    folder, root = Path(folder), Path(root)
    stop = threading.Event()
    state = {"schema": "cherri-function-runtime-journal-arming/1", "status": "WAITING",
             "armCount": 0, "journalObserved": False, "incompleteReads": 0}
    path = root / launch.RELATIVE / "process-started.original.json"
    started = time.monotonic()
    def watch():
        deadline = time.monotonic() + 15
        try:
            while time.monotonic() < deadline and not stop.is_set():
                if path.is_file():
                    data = launch.regular(path, launch.MAX_JSON, empty=True)
                    try:
                        row = launch.strict_json(data)
                    except json.JSONDecodeError:
                        state["incompleteReads"] += 1
                        stop.wait(.05)
                        continue
                    require(type(row.get("pid")) is int and row["pid"] > 0, "LAUNCH_JOURNAL_PID")
                    state.update(journalObserved=True, journalSHA256=sha(data), processPID=row["pid"])
                    if stop.is_set():
                        state["status"] = "STOPPED"
                        return
                    differential.atomic_json(folder/"command.json", phase)
                    state.update(status="PUBLISHED", armCount=1, publishedUTC=launch.now())
                    return
                stop.wait(.05)
            state["status"] = "STOPPED" if stop.is_set() else "TIMEOUT"
        except BaseException as exc:
            state.update(status="FAIL", errorType=type(exc).__name__, error=str(exc)[:1024])
    worker = threading.Thread(target=watch, daemon=True)
    result, error = None, None
    worker.start()
    try:
        capacity_check()
        result = launch.execute(repo, root)
    except BaseException as exc:
        error = exc
    finally:
        # A very fast simctl return still allows the retained start journal to be observed.
        if error is None:
            worker.join(timeout=2)
        stop.set()
        worker.join(timeout=2)
        state["workerStopped"] = not worker.is_alive()
        state["elapsedSeconds"] = time.monotonic() - started
        record(folder/"runtime-arm.original.json", dict(state))
        call = {"schema": "cherri-function-direct-launch-call/1",
                "callable": "function_envelope_launch.execute",
                "nestedPythonProcess": False, "elapsedSeconds": time.monotonic() - started,
                "reviewedChildBudgetSeconds": 25, "reviewedOwnerBudgetSeconds": 35,
                "outerCallBudgetSeconds": 45,
                "returnedStatus": result.get("status") if isinstance(result, dict) else None,
                "errorType": type(error).__name__ if error is not None else None,
                "error": str(error)[:1024] if error is not None else None,
                "runtimeArmingSHA256": sha(read(folder/"runtime-arm.original.json")),
                "nativeExecutionAuthenticated": False, "formalAcceptanceGranted": False}
        record(folder/"runtime-owner-call.original.json", call)
    if error is not None:
        raise error
    require(state["workerStopped"] and state["status"] == "PUBLISHED" and state["armCount"] == 1,
            "RUNTIME_ARMING")
    require(time.monotonic() - started < 45 and isinstance(result, dict) and result.get("status") == "PASS",
            "DIRECT_LAUNCH_RESULT")
    return result


class NativeObservation:
    def __init__(self, plan):
        require(sys.platform == "darwin", "DARWIN_REQUIRED")
        self.plan = validate_plan(plan)
        self.repo, self.root = launch.owned_root(plan["repo"]), launch.owned_root(plan["root"])
        require(self.repo not in self.root.parents and self.root not in self.repo.parents, "ROOT_SCOPE")
        self.folder = self.root / "runtime/function-envelope-observer"
        self.folder.mkdir(mode=0o700)
        self.process_folder = self.folder / "processes"
        self.process_folder.mkdir(mode=0o700)
        self.xctest = None
        self.test_log = None
        self.documents = None
        self.product_inventory = None
        self.build = None
        self.minimum_free = shutil.disk_usage("/System/Volumes/Data").free
        self.capacity_stop = threading.Event()
        self.capacity_thread = None
        self.summary = {**plan, "schema": "cherri-function-envelope-native-observer/1",
                        "status": "FAIL", "importTransportAuthenticated": False,
                        "appleExecutionAuthenticated": False, "formalAcceptanceGranted": False,
                        "semanticLaunchJournalObserved": False, "launchOwnerStarted": False}
        self.before = None
        record(self.folder / "plan.original.json", plan)

    def check_capacity(self):
        free = shutil.disk_usage("/System/Volumes/Data").free
        self.minimum_free = min(self.minimum_free, free)
        require(free >= LIVE_FLOOR_BYTES, "CAPACITY_LIVE_FLOOR")
        return free

    def start_capacity_watch(self):
        def watch():
            while not self.capacity_stop.wait(1):
                free = shutil.disk_usage("/System/Volumes/Data").free
                self.minimum_free = min(self.minimum_free, free)
                if free < LIVE_FLOOR_BYTES:
                    os.kill(os.getpid(), signal.SIGUSR1)
                    return
        self.capacity_thread = threading.Thread(target=watch, daemon=True)
        self.capacity_thread.start()

    def capture(self, label, argv, seconds, *, require_pass=True):
        self.check_capacity()
        folder = self.process_folder / label
        folder.mkdir(mode=0o700)
        def journal(event, value):
            record(folder / ("process-" + event + ".original.json"), value)
        with (folder / "stdout.original").open("xb") as out, (folder / "stderr.original").open("xb") as err:
            observed, details = capture.capture(argv, seconds, output=out, error_output=err,
                hard_deadline=time.monotonic() + seconds + 5, stream_limit=2 * 1024 * 1024, journal=journal)
        record(folder / "process.original.json", {"argv": argv, "observation": observed, "details": details})
        require(details.get("cleanupParentReaped") is True and details.get("groupPresentAfterCleanup") is False,
                "OWNED_PROCESS_CLEANUP")
        if require_pass:
            require(observed.get("result") == "PASS" and type(observed.get("exitCode")) is int
                    and observed["exitCode"] == 0, "PROCESS_" + label)
        self.check_capacity()
        return observed, (folder / "stdout.original").read_bytes(), (folder / "stderr.original").read_bytes()

    def select_device(self):
        folder = self.folder / "device"
        folder.mkdir(mode=0o700)
        for part, kind in (("runtimes", "runtimes"), ("devicetypes", "devicetypes"), ("devices", "devices")):
            _, raw, _ = self.capture("inventory-" + part, ["/usr/bin/xcrun", "simctl", "list", kind, "--json"], 15)
            original(folder / ("sim-" + part + ".json"), raw)
        owner = {"schema": selection.SCHEMA, "sourceCommit": self.plan["controllerCommitSha"],
                 "runId": self.plan["runId"], "udid": DEVICE, "deviceName": "Cherri-P10R15-D03",
                 "deviceTypeIdentifier": "com.apple.CoreSimulator.SimDeviceType.iPhone-16-Pro",
                 "runtimeId": "com.apple.CoreSimulator.SimRuntime.iOS-27-0",
                 "runtimeName": "iOS 27.0", "runtimeVersion": "27.0"}
        record(folder / "owner.original.json", owner)
        code = selection.main(["--record", str(folder/"owner.original.json"), "--inventory-root", str(folder),
            "--output", str(folder/"selection.original.json"), "--source-sha", self.plan["controllerCommitSha"],
            "--run-id", self.plan["runId"], "--profile", "iphone-fallback"])
        require(code == 0, "OWNED_DEVICE_SELECTION")
        # The original same-device phase owner retains every actual process.
        self.capture("session-pre-import", [sys.executable, "-B", str(self.repo/"tools/shortcuts-simulator-qa/simulator_phase_boundary.py"),
            "--repo", str(self.repo), "--root", str(self.folder/"session-pre-import"), "--udid", DEVICE,
            "--source-sha", self.plan["controllerCommitSha"], "--run-id", self.plan["runId"],
            "--attempt", str(self.plan["runAttempt"]), "--fixture-id", launch.FIXTURE, "--phase", "pre-import"], 310)
        self.summary["sessionSourceCommitMeaning"] = "controller harness, not the retained b82 compiler artifact"

    def start_observer(self, build):
        args = ["xcodebuild", "test-without-building", "-xctestrun", build["xctestrun"],
                "-destination", "platform=iOS Simulator,id=" + DEVICE, "-parallel-testing-enabled", "NO",
                "-only-testing:" + TEST, "-resultBundlePath", str(self.folder/"NativeFunction.xcresult")]
        env = os.environ.copy()
        env["TEST_RUNNER_CHERRI_QA_DIFF_RUN_DIR"] = str(self.folder)
        self.test_log = (self.folder/"xctest.log").open("x")
        self.xctest = subprocess.Popen(args, stdout=self.test_log, stderr=subprocess.STDOUT,
                                      env=env, stdin=subprocess.DEVNULL, start_new_session=True)
        record(self.folder/"xctest-process.started.original.json",
               {"argv": args, "pid": self.xctest.pid, "startupTimeoutSeconds": 120, "startedUTC": launch.now()})
        ready = differential.wait_file(self.folder/"ready.json", 120, self.xctest)
        require(ready == {"ready": True}, "OBSERVER_READY")
        self.capture("receiver-install", ["/usr/bin/xcrun", "simctl", "install", DEVICE, build["receiverApp"]], 90)
        _, raw, _ = self.capture("receiver-app-container", ["/usr/bin/xcrun", "simctl", "get_app_container", DEVICE, launch.RECEIVER, "app"], 25)
        installed = Path(raw.decode().strip())
        identity = differential.receiver_image_identity(Path(build["receiverApp"]), installed)
        record(self.folder/"receiver-image-identity.original.json", identity)
        require(identity["outcome"] == "MATCH", "RECEIVER_IMAGE_IDENTITY")

    def import_original(self):
        signed = self.root / "builds/signed" / (launch.WORKFLOW + ".shortcut")
        phase = "import-" + uuid.uuid4().hex
        argv = ["xcrun", "simctl", "openurl", DEVICE, signed.as_uri()]
        path = self.folder / ("dispatch-" + phase + ".json")
        command = {"id": phase, "operation": "import", "shortcutName": launch.WORKFLOW}
        differential.command(argv, timeout=60,
            on_started=lambda: differential.atomic_json(self.folder/"command.json", command), receipt_path=path)
        dispatch = launch.strict_json(read(path))
        receipt = differential.wait_file(self.folder/("ack-" + phase + ".json"), 90, self.xctest)
        validate_import(dispatch, receipt, argv, phase)
        self.summary.update(importTransportAuthenticated=True, importPhase=phase,
                            importedSignedSHA256=sha(read(signed)), importReceiptSHA256=sha(read(self.folder/("ack-"+phase+".json"))))
        # Import can start Receiver itself, so stop it here, after confirmed import.
        observed, _, err = self.capture("receiver-stop-after-import",
            ["/usr/bin/xcrun", "simctl", "terminate", DEVICE, launch.RECEIVER], 15, require_pass=False)
        require(termination_is_stopped(observed, err), "FRESH_RECEIVER_NOT_PROVED")
        self.capture("selected-device-liveness", ["/usr/bin/xcrun", "simctl", "getenv", DEVICE, "HOME"], 15)

    def one_runtime(self):
        launch.prepare(self.repo, self.root, self.plan["controllerCommitSha"], runtime.COMPILER_SOURCE,
                       self.plan["runId"], self.plan["runAttempt"], self.plan["runNonce"], DEVICE)
        runtime_id = "runtime-" + uuid.uuid4().hex
        phase = {"id": runtime_id, "operation": "runtime", "shortcutName": launch.WORKFLOW}
        self.summary["launchOwnerStarted"] = True
        self.summary["launchOwnerKind"] = "IN_PROCESS_REVIEWED_EXECUTE"
        try:
            execute_with_runtime_observer(self.repo, self.root, self.folder, phase, self.check_capacity)
        finally:
            arm = self.folder/"runtime-arm.original.json"
            if arm.is_file():
                self.summary["semanticLaunchJournalObserved"] = launch.strict_json(read(arm)).get("journalObserved") is True
        _, raw, _ = self.capture("receiver-data-container",
            ["/usr/bin/xcrun", "simctl", "get_app_container", DEVICE, launch.RECEIVER, "data"], 25)
        documents = launch.owned_root(Path(raw.decode().strip())/"Documents")
        self.documents = documents
        callbacks = documents/"callback-events.json"
        end = time.monotonic() + 48
        callback = None
        while time.monotonic() < end:
            if callbacks.is_file():
                raw = read(callbacks, runtime.MAX_CALLBACK)
                events = launch.strict_json(raw)
                require(isinstance(events, list) and all(isinstance(x, dict) for x in events), "CALLBACK_ARRAY")
                matched = [x for x in events if x.get("runId") == self.plan["runNonce"]]
                require(len(matched) <= 1, "CALLBACK_DUPLICATE")
                if matched:
                    callback = matched[0]
                    original(self.root/"runtime/callback-events.json", raw)
                    differential.atomic_json(self.folder/("completion-"+runtime_id+".json"), callback)
                    break
            require(self.xctest.poll() is None, "XCTEST_ENDED_DURING_RUNTIME")
            self.check_capacity()
            time.sleep(.25)
        if callback is None and callbacks.is_file():
            original(self.root/"runtime/callback-events.json", read(callbacks, runtime.MAX_CALLBACK))
        ui = differential.wait_file(self.folder/("ack-"+runtime_id+".json"), 60, self.xctest)
        self.summary["runtimeUIReceipt"] = ui
        require(ui.get("id") == runtime_id and ui.get("operation") == "runtime"
                and ui.get("outcome") == "CALLBACK_OBSERVED_BY_HOST", "RUNTIME_UI")
        require(callback is not None, "CALLBACK_NOT_OBSERVED")
        launch.collect_receiver(self.root, documents)
        observation = runtime.evaluate(self.repo, self.root, launch.FIXTURE, runtime.COMPILER_SOURCE,
                                       self.plan["controllerCommitSha"], self.plan["runNonce"], "")
        record(self.folder/"runtime-observation.original.json", observation)
        require(observation["runtimeObservationVerified"] is True, "RUNTIME_ORIGINAL_JOIN")
        original(self.root/"runtime/runtime-trace.jsonl", runtime.trace_bytes(observation["traceRecords"]))
        self.summary["declaredNoInputGoalsMatched"] = observation["declaredNoInputGoalsMatched"]
        self.summary["runtimeObservationSHA256"] = sha(read(self.folder/"runtime-observation.original.json"))

    def finish(self):
        if self.xctest is None:
            return
        pending_error = None
        signals = []
        try:
            if self.xctest.poll() is None:
                differential.atomic_json(self.folder/"command.json", {"id": "finish", "operation": "finish"})
                try:
                    self.xctest.wait(timeout=90)
                except subprocess.TimeoutExpired:
                    for sig in (signal.SIGINT, signal.SIGTERM, signal.SIGKILL):
                        if self.xctest.poll() is not None:
                            break
                        try:
                            os.killpg(self.xctest.pid, sig)
                            signals.append({"signal": int(sig), "sent": True})
                        except ProcessLookupError:
                            signals.append({"signal": int(sig), "sent": False, "absent": True})
                        try:
                            self.xctest.wait(timeout=15 if sig != signal.SIGKILL else 5)
                        except subprocess.TimeoutExpired:
                            pass
        except BaseException as exc:
            pending_error = exc
        # Parent reaping never proves that its start_new_session group is empty.
        before = capture.group_observation(self.xctest.pid)
        cleanup = {"state": "NOT_NEEDED", "signalSent": False, "errno": None}
        if before["present"] is not False:
            cleanup = capture.kill_group(self.xctest)
        if self.xctest.poll() is None:
            try:
                self.xctest.wait(timeout=2)
            except subprocess.TimeoutExpired:
                pass
        after = capture.group_observation(self.xctest.pid)
        deadline = time.monotonic() + 5
        while after["present"] is True and time.monotonic() < deadline:
            time.sleep(.05)
            after = capture.group_observation(self.xctest.pid)
        self.test_log.close()
        record(self.folder/"xctest-process.finished.original.json",
               {"pid": self.xctest.pid, "exitCode": self.xctest.poll(),
                "parentReaped": self.xctest.returncode is not None, "completedUTC": launch.now(),
                "finishSignals": signals, "groupBeforeCleanup": before,
                "ownedGroupCleanup": cleanup, "groupAfterCleanup": after,
                "pendingError": type(pending_error).__name__ + ":" + str(pending_error)[:1024]
                    if pending_error is not None else None})
        self.summary["xctestExit"] = self.xctest.poll()
        require(self.xctest.returncode is not None, "XCTEST_NOT_REAPED")
        require(after["present"] is False, "XCTEST_OWNED_GROUP_NOT_ABSENT")
        require(before["present"] is False, "XCTEST_OWNED_GROUP_REMAINED")
        if pending_error is not None:
            raise pending_error

    def preserve_partial_receiver(self):
        if not self.summary["launchOwnerStarted"]:
            return
        if self.documents is None:
            _, raw, _ = self.capture("failure-receiver-data-container",
                ["/usr/bin/xcrun", "simctl", "get_app_container", DEVICE, launch.RECEIVER, "data"], 15)
            self.documents = launch.owned_root(Path(raw.decode().strip())/"Documents")
        value = retain_failure_receiver(self.root, self.documents, self.folder/"failed-receiver-originals")
        self.summary["partialOriginalFiles"] = len(value["files"])

    def run(self):
        try:
            self.check_capacity()
            self.start_capacity_watch()
            self.before = source_inventory(self.repo, self.plan["controllerCommitSha"])
            record(self.folder/"source.before.original.json", self.before)
            self.check_capacity()
            build = self.build = verify_build(self.plan, self.repo)
            record(self.folder/"build-binding.original.json", build)
            self.product_inventory = test_products_inventory(build["xctestrun"])
            record(self.folder/"xctest-products.before.original.json", self.product_inventory)
            runtime.provenance(self.repo, self.root, launch.FIXTURE, runtime.COMPILER_SOURCE,
                               self.plan["controllerCommitSha"])
            record(self.folder/"signing-binding.original.json", verify_signing(self.root))
            self.select_device()
            self.start_observer(build)
            self.import_original()
            self.one_runtime()
            self.summary["status"] = "OBSERVATION_ACQUIRED"
        except BaseException as exc:
            self.summary.update(errorType=type(exc).__name__, error=str(exc)[:1024])
        finally:
            # Stop new observer cancellations before bounded evidence finalization.
            signal.signal(signal.SIGUSR1, signal.SIG_IGN)
            signal.alarm(0)
            self.capacity_stop.set()
            if self.capacity_thread is not None:
                self.capacity_thread.join(timeout=2)
            if self.summary["status"] != "OBSERVATION_ACQUIRED":
                try:
                    self.preserve_partial_receiver()
                except BaseException as exc:
                    self.summary["partialOriginalRetentionError"] = type(exc).__name__ + ":" + str(exc)[:1024]
            errors = []
            try:
                self.finish()
            except BaseException as exc:
                errors.append("cleanup:" + type(exc).__name__ + ":" + str(exc)[:1024])
            if self.summary["status"] == "OBSERVATION_ACQUIRED" and not errors:
                try:
                    validate_xctest(read(self.folder/"xctest.log"), self.xctest.returncode)
                except BaseException as exc:
                    errors.append("xctest:" + type(exc).__name__ + ":" + str(exc)[:1024])
            # A failed child or cleanup must not suppress independent source/product receipts.
            if self.before is not None:
                try:
                    after = source_inventory(self.repo, self.plan["controllerCommitSha"])
                    record(self.folder/"source.after.original.json", after)
                    require(after == self.before, "SOURCE_CHANGED")
                except BaseException as exc:
                    errors.append("source:" + type(exc).__name__ + ":" + str(exc)[:1024])
            if self.product_inventory is not None:
                try:
                    products_after = test_products_inventory(self.build["xctestrun"])
                    record(self.folder/"xctest-products.after.original.json", products_after)
                    require(products_after == self.product_inventory, "TEST_PRODUCTS_CHANGED")
                except BaseException as exc:
                    errors.append("products:" + type(exc).__name__ + ":" + str(exc)[:1024])
            if errors:
                self.summary.update(status="FAIL", finalizationErrors=errors,
                                    appleExecutionAuthenticated=False)
            elif self.summary["status"] == "OBSERVATION_ACQUIRED":
                self.summary["appleExecutionAuthenticated"] = True
            self.summary["minimumFreeBytes"] = self.minimum_free
            self.summary["liveFloorBytes"] = LIVE_FLOOR_BYTES
            record(self.folder/"result.original.json", self.summary)
        return self.summary


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--plan", required=True)
    args = parser.parse_args()
    plan = validate_plan(launch.strict_json(read(args.plan, 65536)))
    def deadline(signum, frame):
        raise TimeoutError("FUNCTION_OBSERVER_WHOLE_DEADLINE")
    def capacity(signum, frame):
        raise RuntimeError("FUNCTION_OBSERVER_CAPACITY_LIVE_FLOOR")
    def terminated(signum, frame):
        raise KeyboardInterrupt("FUNCTION_OBSERVER_TERMINATED")
    signal.signal(signal.SIGTERM, terminated)
    signal.signal(signal.SIGALRM, deadline)
    signal.signal(signal.SIGUSR1, capacity)
    signal.alarm(OBSERVATION_SECONDS)
    try:
        result = NativeObservation(plan).run()
    finally:
        signal.alarm(0)
    print(json.dumps({k: result.get(k) for k in ("status", "declaredNoInputGoalsMatched",
                      "importTransportAuthenticated", "appleExecutionAuthenticated", "formalAcceptanceGranted",
                      "error", "finalizationErrors")}, sort_keys=True))
    return 0 if result["status"] == "OBSERVATION_ACQUIRED" else 1


if __name__ == "__main__":
    raise SystemExit(main())
