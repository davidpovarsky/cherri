"""One source-bound FN04 hosted native attempt using the original signed packets.

Build, packet compiler, signer and GitHub controller identities stay separate.
The original native owner performs import/runtime verification. This adapter
selects one fresh owned simulator, builds the original harness, and seals originals.
"""
from pathlib import Path, PurePosixPath
import contextlib
import hashlib
import importlib.util
import io
import json
import os
import re
import shutil
import signal
import stat
import sys
import time
import zipfile

import native_caller_execution_context as execution
import native_caller_input_owner as base
import native_signed_caller_input_owner as signed
import native_caller_hosted_build as build_recipe

SCHEMA = "cherri-fn04-hosted-native/1"
WHOLE_SECONDS = 2580
FINAL_RESERVE_SECONDS = 300
BUILD_CHILD_SECONDS = 800
NATIVE_CHILD_SECONDS = 1230
NATIVE_INCREMENTAL_BYTES = 128 * 1024**2
DEVICE_INCREMENTAL_BYTES = 1024**3
PACKET_SOURCE = "5b689c938c73420ef36b3f077764d30f8946f31a"
PACKET_INTAKE_SHA = "505933619bcb8bea604bbc375b31cf57824bddfaa514e354784ebb86f00e3678"
PACKET_ZIP = ("tests/language-v2/fn04-caller-packet-originals.zip",
              "012e4a67114965b4bab9335e8117580c59279ed6ff85b0e9005e12d479bf1430")
SIGNING_ZIPS = {
    "initial": ("tests/language-v2/fn04-exact-file-signing-originals.zip",
                "686b3ebc098ad3cf89726b3186b3381ff863b062870f697a9329b36928464482",
                "16b3797037def5451f84cec38b80ba2159a2499873681b1d7be9565a8c5b177c"),
    "retry": ("tests/language-v2/fn04-exact-file-signing-companion-retry-originals.zip",
              "0d56e045c4346faefeca2dbc7886dc1dd64286f205d3831c896d87a8f7fac92c",
              "a782dc5ef24667421eb86574b39e94cc1c4ca4e5ea95f292c114b41199dd5f47"),
}
SEAL_PATH = "ios/scripts/seal_rp39_diagnostic.py"
SEAL_SHA = "77bc46a766b6548774984780f695be88b4037b8a821508ad7d9e745b473784a0"
RUNTIME_ID = "com.apple.CoreSimulator.SimRuntime.iOS-27-0"
DEVICE_TYPE = "com.apple.CoreSimulator.SimDeviceType.iPhone-16-Pro"
MAX_ZIP_BYTES = 8 * 1024**2
MAX_EXPANDED_BYTES = 16 * 1024**2
MAX_ZIP_MEMBERS = 64
require, sha, record, read = base.require, base.sha, base.record, base.read


def extract_original_bundle(path, expected_sha, output):
    raw = read(path, MAX_ZIP_BYTES)
    require(sha(raw) == expected_sha, "AUTHENTIC_FIXTURE_ZIP")
    with zipfile.ZipFile(io.BytesIO(raw)) as archive:
        rows = archive.infolist()
        require(0 < len(rows) <= MAX_ZIP_MEMBERS and len({r.filename for r in rows}) == len(rows),
                "FIXTURE_ZIP_MEMBERS")
        total = 0
        for row in rows:
            name = PurePosixPath(row.filename)
            require(not row.is_dir() and not name.is_absolute() and row.filename == str(name)
                    and name.parts and all(part not in ("", ".", "..") for part in name.parts)
                    and all(c not in row.filename for c in ("\n", "\r", "\0", "\\"))
                    and len(row.filename.encode()) <= 1024
                    and stat.S_ISREG(row.external_attr >> 16), "FIXTURE_ZIP_PATH")
            total += row.file_size
            require(0 <= row.file_size <= MAX_EXPANDED_BYTES and total <= MAX_EXPANDED_BYTES,
                    "FIXTURE_ZIP_BOUND")
        output.mkdir(mode=0o700)
        for row in rows:
            data = archive.read(row)
            require(len(data) == row.file_size, "FIXTURE_ZIP_BYTES")
            target = output / row.filename
            target.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
            base.original(target, data)
    return {"bytes": len(raw), "sha256": sha(raw), "memberCount": len(rows),
            "expandedBytes": total}


def choose_new_device(runtimes, types, devices, context):
    require(type(runtimes) is dict and type(runtimes.get("runtimes")) is list
            and type(types) is dict and type(types.get("devicetypes")) is list
            and type(devices) is dict and type(devices.get("devices")) is dict,
            "ORIGINAL_DEVICE_INVENTORIES")
    candidates = [r for r in runtimes["runtimes"] if r.get("identifier") == RUNTIME_ID]
    kinds = [r for r in types["devicetypes"] if r.get("identifier") == DEVICE_TYPE]
    require(len(candidates) == len(kinds) == 1 and candidates[0].get("isAvailable") is True
            and candidates[0].get("name") == "iOS 27.0" and candidates[0].get("version") == "27.0"
            and kinds[0].get("name", "").startswith("iPhone"), "EXACT_AVAILABLE_IOS27_PHONE")
    name = "Cherri-FN04-" + context["runId"] + "-1"
    require(all(type(group) is list and all(type(d) is dict for d in group)
                for group in devices["devices"].values()), "DEVICE_ROWS")
    require(not any(d.get("name") == name for group in devices["devices"].values()
                    for d in group), "FRESH_HOSTED_DEVICE_NAME")
    return {"schema": base.selection.SCHEMA, "sourceCommit": context["sourceCommitSha"],
            "runId": context["runId"], "udid": None, "deviceName": name,
            "deviceTypeIdentifier": DEVICE_TYPE, "runtimeId": RUNTIME_ID,
            "runtimeName": candidates[0]["name"], "runtimeVersion": candidates[0]["version"]}


def validate_native_result(value, plan, context):
    require(type(value) is dict and value.get("schema") == base.SCHEMA
            and value.get("plan") == plan and value.get("hostedContext") == context
            and value.get("executionLocation") == "GITHUB_ACTIONS_MACOS",
            "EXACT_NATIVE_RESULT_CONTEXT")
    require(value.get("status") == "OBSERVATION_VERIFIED" and
            value.get("runtimeObservationAuthenticated") is True and
            type(value.get("actualRuntimeDispatches")) is int and
            value["actualRuntimeDispatches"] == 1 and
            value.get("sourceUnchanged") is True and value.get("firstInterruption") is None
            and not value.get("finalizationErrors") and not value.get("error"),
            "ORIGINAL_NATIVE_OBSERVATION_REQUIRED")
    require(value.get("wholeOwnerSeconds") == 1200 and value.get("finalReserveSeconds") == 300
            and value.get("originalLiveFloorBytes") == 4*1024**3
            and all(value.get(k) is False for k in
                ("formalAcceptanceGranted", "rawNativeTypesObserved", "compilerRebuilt", "harnessRebuilt")),
            "ORIGINAL_NATIVE_SCOPE_AND_BOUNDS")
    return value


def archive_members(private, seal, guard):
    members, directories, total = {}, {}, 0
    for root, dirs, files in os.walk(private, followlinks=False):
        guard()
        root = Path(root)
        info = root.lstat()
        require(stat.S_ISDIR(info.st_mode) and not root.is_symlink()
                and len(directories) < seal.MEMBER_LIMIT, "PRIVATE_DIRECTORY_BOUND")
        key = root.relative_to(private).as_posix()
        directories[key] = {"mode": stat.S_IMODE(info.st_mode), "uid": info.st_uid,
                            "gid": info.st_gid, "mtimeNs": info.st_mtime_ns}
        require(all(not (root / name).is_symlink() for name in dirs), "PRIVATE_DIRECTORY_LINK")
        for name in sorted(files):
            guard()
            path = root / name
            key = path.relative_to(private).as_posix()
            require(key != "manifest.original.json" and len(key.encode()) <= 1024
                    and all(c not in key for c in ("\n", "\r", "\0", "\\"))
                    and len(members) < seal.MEMBER_LIMIT, "PRIVATE_MEMBER_BOUND")
            original = seal.file_record(path)
            total += original["bytes"]
            require(total <= seal.LOGICAL_LIMIT, "PRIVATE_LOGICAL_BOUND")
            members[key] = {"path": path, "original": original}
    require(members, "PRIVATE_ORIGINALS_REQUIRED")
    return members, directories, total


class HostedCaller:
    def __init__(self):
        require(sys.platform == "darwin" and sys.version_info >= (3, 10),
                "DARWIN_SHARED_MONOTONIC_CLOCK")
        self.context = execution.context_from_environment()
        self.paths = execution.paths(self.context)
        self.repo = self.paths["repo"]
        self.started = time.monotonic()
        self.hard_deadline = self.started + WHOLE_SECONDS
        self.execution_deadline = self.hard_deadline - FINAL_RESERVE_SECONDS
        self.latch = base.InterruptionLatch()
        self.paths["namespace"].mkdir(mode=0o700)
        self.paths["private"].mkdir(mode=0o700)
        self.paths["public"].mkdir(mode=0o700)
        self.private = self.paths["private"]
        (self.private / "processes").mkdir(mode=0o700)
        self.device_record = None
        self.before = self.plan = self.native_result = None
        self.state = {"schema": SCHEMA, "status": "FAIL", "context": self.context,
            "wholeOwnerSeconds": WHOLE_SECONDS, "finalReserveSeconds": FINAL_RESERVE_SECONDS,
            "nativeOwnerSeconds": 1200, "nativeExecutionSeconds": 900,
            "buildOwnerSeconds": 780, "xcodeBuildSeconds": 600,
            "signingRepeated": False, "compilerRebuilt": False,
            "nativeChildInvocations": 0, "nativeOwnerInvocations": 0,
            "inProcessInvocations": [], "formalAcceptanceGranted": False,
            "rawNativeTypesObserved": False, "processes": [], "errors": []}
        self.context_path = self.private / "execution-context.original.json"
        record(self.context_path, self.context)
        _, event_raw = execution.read_create_event(os.environ)
        require(sha(event_raw) == self.context["eventPayloadSHA256"], "CREATE_EVENT_CONTEXT_CHANGED")
        base.original(self.private / "github-event.original.json", event_raw)
        record(self.private / "owner.original.json", {"pid": os.getpid(), "pgid": os.getpgrp(),
            "python": str(Path(sys.executable).resolve()), "pythonSHA256":
            sha(Path(sys.executable).resolve(strict=True).read_bytes()),
            "startedUTC": base.library.now()})

    def capacity(self):
        free = shutil.disk_usage("/System/Volumes/Data").free
        require(free >= base.LIVE_FLOOR_BYTES, "ORIGINAL_NATIVE_LIVE_FLOOR")
        return free

    def command(self, label, argv, seconds, *, require_pass=True, final=False):
        deadline = self.hard_deadline if final else self.execution_deadline
        require(self.latch.reason is None and time.monotonic()+seconds+5 < deadline,
                "HOSTED_COMMAND_BUDGET")
        self.capacity()
        folder = self.private / "processes" / label
        folder.mkdir(mode=0o700)
        observed = details = None
        last = {}
        def journal(phase, value):
            last.clear()
            last.update(value)
            record(folder / (phase + ".original.json"), value)
        try:
            with (folder / "stdout.original").open("xb") as out, \
                    (folder / "stderr.original").open("xb") as err:
                observed, details = base.capture.capture(argv, seconds, output=out,
                    error_output=err, hard_deadline=min(deadline, time.monotonic()+seconds+5),
                    journal=journal)
        finally:
            row = {"label": label, "argv": argv, "observation": observed,
                   "process": details or last, "firstInterruption": self.latch.reason}
            record(folder / "result.original.json", row)
            self.state["processes"].append(row)
        require(details.get("cleanupParentReaped") is True and
                details.get("groupPresentAfterCleanup") is False, "HOSTED_CHILD_CLEANUP")
        require(all(details.get(side, {}).get("eof") is True and
                    details[side].get("overflow") is False for side in ("stdout", "stderr")),
                "HOSTED_CHILD_STREAMS")
        if require_pass:
            require(observed.get("result") == "PASS" and observed.get("exitCode") == 0,
                    "HOSTED_CHILD_" + label)
        return observed, read(folder / "stdout.original")


    def preserve_inner_interruption(self, label, reason):
        if reason is not None and self.latch.reason is None:
            self.latch.reason = "INNER_" + label + ":" + str(reason)

    def invoke_original_owner(self, label, entry, seconds, result_path):
        """Call the original owner in this process; its children keep one owner."""
        require(label in ("standalone-build-owner", "native-owner") and
                self.latch.reason is None and time.monotonic()+seconds+5 < self.execution_deadline,
                "HOSTED_ORIGINAL_OWNER_BUDGET")
        self.capacity()
        folder = self.private / "invocations" / label
        folder.mkdir(parents=True, mode=0o700)
        previous = {s: signal.getsignal(s) for s in
                    (signal.SIGALRM, signal.SIGTERM, signal.SIGINT)}
        begin = time.monotonic()
        row = {"label": label, "execution": "IN_PROCESS_ORIGINAL_OWNER",
               "controllerPid": os.getpid(), "controllerPgid": os.getpgrp(),
               "wholeControllerDeadlineMonotonic": self.hard_deadline,
               "executionDeadlineMonotonic": self.execution_deadline,
               "admittedOwnerAndSetupSeconds": seconds,
               "originalOwnerSeconds": 780 if label == "standalone-build-owner" else 1200,
               "startedUTC": base.library.now(), "subprocessOwnerLaunched": False}
        record(folder / "started.original.json", row)
        returned = original_result = None
        try:
            with (folder / "stdout.original").open("x") as out, \
                    (folder / "stderr.original").open("x") as err:
                with contextlib.redirect_stdout(out), contextlib.redirect_stderr(err):
                    returned = entry()
            return returned
        except BaseException as error:
            row["error"] = {"type": type(error).__name__, "message": str(error)[:1200]}
            raise
        finally:
            # The inner owners finish and reap their own separately-sessioned
            # Xcode children before returning. Do not introduce an outer Popen
            # that could SIGKILL those owners before that closure.
            try:
                if result_path.is_file():
                    raw = read(result_path)
                    original_result = base.library.strict_json(raw)
                    row["originalResult"] = {"bytes": len(raw), "sha256": sha(raw)}
                    reason = original_result.get("firstInterruption")
                    self.preserve_inner_interruption(label, reason)
            finally:
                signal.setitimer(signal.ITIMER_REAL, 0)
                for signum, handler in previous.items():
                    signal.signal(signum, handler)
                if self.latch.reason is None:
                    signal.setitimer(signal.ITIMER_REAL,
                        max(0.001, self.hard_deadline-time.monotonic()))
                row.update(firstInterruption=self.latch.reason,
                    elapsedSeconds=time.monotonic()-begin, finishedUTC=base.library.now(),
                    originalOwnerReturned=returned is not None)
                record(folder / "finished.original.json", row)
                self.state["inProcessInvocations"].append(row)

    def run_native_original(self, selection):
        def enter():
            owner = signed.SignedNativeCallerObservation(self.plan, selection,
                                                         execution_context=self.context)
            require(owner.hard_deadline < self.execution_deadline,
                    "ORIGINAL_NATIVE_OWNER_FITS_CONTROLLER")
            # The original run method requires no inherited alarm, then arms
            # its existing 1200-second deadline. Construction remains covered
            # by the controller alarm and has launched no child.
            signal.setitimer(signal.ITIMER_REAL, 0)
            self.state["nativeOwnerInvocations"] += 1
            require(self.state["nativeOwnerInvocations"] == 1, "ONE_NATIVE_OWNER")
            try:
                return owner.run()
            finally:
                self.preserve_inner_interruption("native-owner", owner.latch.reason)
        return self.invoke_original_owner("native-owner", enter, NATIVE_CHILD_SECONDS,
            self.paths["root"] / "runtime/result.original.json")


    def create_device(self):
        inventories = {}
        for name in ("runtimes", "devicetypes", "devices"):
            _, raw = self.command("preparation-" + name,
                ["/usr/bin/xcrun", "simctl", "list", name, "--json"], 15)
            inventories[name] = base.library.strict_json(raw)
        value = choose_new_device(inventories["runtimes"], inventories["devicetypes"],
                                  inventories["devices"], self.context)
        record(self.private / "device-creation-intent.original.json", value)
        _, raw = self.command("create-owned-device", ["/usr/bin/xcrun", "simctl", "create",
            value["deviceName"], value["deviceTypeIdentifier"], value["runtimeId"]], 30)
        udid = raw.decode("ascii").strip().upper()
        require(base.selection.UUID.fullmatch(udid) is not None, "ACTUAL_CREATED_DEVICE_UUID")
        value["udid"] = udid
        self.device_record = value
        record(self.private / "device-owner.original.json", value)
        _, raw = self.command("created-device-inventory",
            ["/usr/bin/xcrun", "simctl", "list", "devices", "--json"], 15)
        base.selection.select_owned(value, inventories["runtimes"], inventories["devicetypes"],
            base.library.strict_json(raw), self.context["sourceCommitSha"],
            self.context["runId"], "iphone-fallback")

    def close_device(self):
        if self.device_record is None or self.latch.reason is not None:
            return
        _, raw = self.command("closing-device-inventory",
            ["/usr/bin/xcrun", "simctl", "list", "devices", "--json"], 15, final=True)
        inventory = base.library.strict_json(raw)
        found = [(rid, d) for rid, group in inventory["devices"].items() for d in group
                 if d.get("udid") == self.device_record["udid"]]
        require(len(found) == 1, "CLOSING_OWNED_DEVICE")
        rid, device = found[0]
        require(rid == self.device_record["runtimeId"] and
                device.get("name") == self.device_record["deviceName"] and
                device.get("deviceTypeIdentifier") == self.device_record["deviceTypeIdentifier"],
                "CLOSING_DEVICE_IDENTITY")
        require(device.get("state") in ("Booted", "Shutdown"), "CLOSING_DEVICE_STATE")
        if device["state"] == "Booted":
            self.command("shutdown-owned-device", ["/usr/bin/xcrun", "simctl", "shutdown",
                         self.device_record["udid"]], 30, final=True)
        self.state["ownedDeviceShutdownCommandCompleted"] = True

    def prepare_and_run(self):
        critical = ["tools/shortcuts-simulator-qa/" + name for name in base.CRITICAL] + [
            "tools/shortcuts-simulator-qa/native_caller_hosted_run.py",
            "tools/shortcuts-simulator-qa/native_caller_hosted_build.py",
            "tools/shortcuts-simulator-qa/native_signed_caller_input_owner.py",
            execution.WORKFLOW_FILE, SEAL_PATH, PACKET_ZIP[0],
            SIGNING_ZIPS["initial"][0], SIGNING_ZIPS["retry"][0]]
        self.critical = critical
        self.before = base.source_binding.source_inventory(self.repo,
            self.context["sourceCommitSha"], self.private / "source-before", critical)
        free = self.capacity()
        require(free >= base.LIVE_FLOOR_BYTES + DEVICE_INCREMENTAL_BYTES +
                build_recipe.ADDITIONAL_ESTIMATE + NATIVE_INCREMENTAL_BYTES,
                "HOSTED_NEXT_PHASE_INCREMENTAL_ADMISSION")
        record(self.private / "incremental-admission.original.json", {
            "freeBytes": free, "nativeLiveFloorBytes": base.LIVE_FLOOR_BYTES,
            "buildIncrementalBytes": build_recipe.ADDITIONAL_ESTIMATE,
            "freshDeviceAllowanceBytes": DEVICE_INCREMENTAL_BYTES,
            "nativeIncrementalBytes": NATIVE_INCREMENTAL_BYTES,
            "basis": "Original fifteen-input harness uses one-GiB build allowance; fresh owned simulator adds one-GiB preparation allowance; native owner retains its original four-GiB floor and 128-MiB phase estimate."})
        self.state["packetArchive"] = extract_original_bundle(self.repo / PACKET_ZIP[0],
            PACKET_ZIP[1], self.paths["bundleRoot"])
        signature = SIGNING_ZIPS["retry" if self.context["caseId"] == "without_functions" else "initial"]
        self.state["signatureArchive"] = extract_original_bundle(self.repo / signature[0],
            signature[1], self.paths["signingRoot"])
        require(sha(read(self.paths["bundleRoot"] / "intake.original.json")) == PACKET_INTAKE_SHA
                and sha(read(self.paths["signingRoot"] / "intake.original.json")) == signature[2],
                "AUTHENTIC_INTAKE_PINS")
        self.create_device()
        qa = self.repo / "tools/shortcuts-simulator-qa"
        build_deadline = min(self.execution_deadline, time.monotonic()+BUILD_CHILD_SECONDS)
        build_exit = self.invoke_original_owner("standalone-build-owner",
            lambda: build_recipe.main(["--execution-context", str(self.context_path),
                "--device-record", str(self.private / "device-owner.original.json")],
                parent_deadline=build_deadline,
                parent_interruption=lambda reason: self.preserve_inner_interruption(
                    "standalone-build-owner", reason)), BUILD_CHILD_SECONDS,
            self.paths["buildRoot"] / "result.original.json")
        require(type(build_exit) is int and build_exit == 0, "ORIGINAL_BUILD_OWNER_RESULT")
        build_raw = read(self.paths["buildRoot"] / "result.original.json")
        build_result = base.library.strict_json(build_raw)
        require(build_result.get("status") == "BUILD_VALIDATED" and
                build_result.get("hostedContext") == self.context, "ACTUAL_HOSTED_BUILD_REQUIRED")
        self.plan = {"schema": base.PLAN_SCHEMA, "repo": str(self.repo),
            "root": str(self.paths["root"]), "controllerCommitSha": self.context["sourceCommitSha"],
            "compilerCommitSha": PACKET_SOURCE, "builtHarnessCommitSha": self.context["sourceCommitSha"],
            "buildRoot": str(self.paths["buildRoot"]), "buildResultSHA256": sha(build_raw),
            "bundleRoot": str(self.paths["bundleRoot"]), "bundleIntakeSHA256": PACKET_INTAKE_SHA,
            "runId": self.context["runId"], "runNonce": os.urandom(16).hex(),
            "caseId": self.context["caseId"], "deviceRecord": self.device_record,
            "estimatedIncrementalBytes": NATIVE_INCREMENTAL_BYTES,
            "pythonExecutableSHA256": sha(Path(sys.executable).resolve(strict=True).read_bytes())}
        base.validate_plan(self.plan, self.context)
        # Bind the completed build before the sole native child is dispatched.
        base.bind_build(self.plan, self.before, self.context)
        selection = {"schema": signed.SELECTION_SCHEMA,
            "planSHA256": sha(base.library.canonical(self.plan)+b"\n"),
            "bundleRoot": str(self.paths["signingRoot"]), "bundleIntakeSHA256": signature[2]}
        signed.validate_selection(selection, self.plan, self.context)
        _, _, checked, _ = base.bind_packet_bundle(self.plan)
        signed.bind_signed_bundle(selection, self.plan, checked)
        record(self.private / "native-plan.original.json", self.plan)
        record(self.private / "signature-selection.original.json", selection)
        require(self.capacity() >= base.LIVE_FLOOR_BYTES + NATIVE_INCREMENTAL_BYTES,
                "ORIGINAL_NATIVE_INCREMENTAL_ADMISSION")
        returned = self.run_native_original(selection)
        raw = read(self.paths["root"] / "runtime/result.original.json")
        self.state["nativeResult"] = {"bytes": len(raw), "sha256": sha(raw)}
        self.native_result = base.library.strict_json(raw)
        validate_native_result(self.native_result, self.plan, self.context)
        require(returned == self.native_result and self.state["nativeOwnerInvocations"] == 1
                and self.state["nativeChildInvocations"] == 0,
                "ACTUAL_IN_PROCESS_NATIVE_OWNER_RESULT")
        self.state["status"] = "OBSERVATION_VERIFIED"

    def seal_originals(self):
        path = self.repo / SEAL_PATH
        require(sha(path.read_bytes()) == SEAL_SHA, "REVIEWED_SEAL_SOURCE")
        spec = importlib.util.spec_from_file_location("fn04_shared_original_seal", path)
        seal = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(seal)
        require(seal.WHOLE_SECONDS == 300 and seal.ENCRYPT_SECONDS == 20
                and seal.LOGICAL_LIMIT == 512*1024**2 and seal.ARCHIVE_LIMIT == 256*1024**2
                and seal.MEMBER_LIMIT == 10000 and seal.LIVE_FLOOR == 3*1024**3,
                "SHARED_SEAL_BOUNDS")
        started = time.monotonic()
        deadline = min(self.hard_deadline, started + seal.WHOLE_SECONDS)
        def guard():
            require(time.monotonic() < deadline and
                    shutil.disk_usage("/System/Volumes/Data").free >= seal.LIVE_FLOOR,
                    "SHARED_SEAL_DEADLINE_OR_CAPACITY")
        members, directories, logical = archive_members(self.private, seal, guard)
        require(shutil.disk_usage("/System/Volumes/Data").free >=
                seal.LIVE_FLOOR + 2*logical + seal.OVERHEAD_BYTES, "SHARED_SEAL_INCREMENTAL_ADMISSION")
        seal_root = self.paths["namespace"] / "seal"
        seal_root.mkdir(mode=0o700)
        stage = seal_root / "public-stage"
        stage.mkdir(mode=0o700)
        identity = {"consumer": SCHEMA, "context": self.context, "directories": directories,
            "nativeResult": self.state.get("nativeResult"), "formalAcceptanceGranted": False}
        archive = seal_root / "fn04-originals.zip"
        archive_record, manifest = seal.make_archive(archive, members, identity, guard=guard,
            manifest_schema="cherri-native-original-diagnostics/1")
        recipient = self.repo / seal.base.RECIPIENT
        ciphertext, encryption = seal.encrypt_archive(seal_root, stage, archive, archive_record,
            recipient, base.capture, deadline)
        guard()
        after, after_dirs, after_logical = archive_members(self.private, seal, guard)
        require(after == members and after_dirs == directories and after_logical == logical,
                "PRIVATE_ORIGINALS_CHANGED_AFTER_SEAL")
        source = stage / "rp39-originals.zip.p7m"
        target = self.paths["public"] / "fn04-originals.zip.p7m"
        require(not target.exists() and not target.is_symlink(), "CIPHERTEXT_COLLISION")
        source.rename(target)
        require(seal.file_record(target, seal.CIPHER_LIMIT) == ciphertext, "CIPHERTEXT_RENAME_CHANGED")
        return {"transportSchema": seal.SCHEMA, "manifestSchema": manifest["schema"],
            "manifest": {"bytes": len(seal.base.canonical(manifest)),
                         "sha256": sha(seal.base.canonical(manifest))},
            "plaintextArchive": {k: archive_record[k] for k in ("bytes", "sha256")},
            "ciphertext": {k: ciphertext[k] for k in ("bytes", "sha256")},
            "memberCount": len(members)+1, "recipientSha256": seal.base.RECIPIENT_SHA256,
            "encryptionProcess": encryption, "sealSourceSHA256": SEAL_SHA,
            "elapsedMs": round((time.monotonic()-started)*1000)}

    def run(self):
        require(signal.getitimer(signal.ITIMER_REAL) == (0.0, 0.0), "NO_FOREIGN_CONTROLLER_ALARM")
        previous = {s: signal.getsignal(s) for s in (signal.SIGALRM, signal.SIGTERM, signal.SIGINT)}
        def interrupted(signum, frame):
            self.latch.interrupt("HOSTED_CONTROLLER_SIGNAL_" + str(signum))
        for signum in previous:
            signal.signal(signum, interrupted)
        signal.setitimer(signal.ITIMER_REAL, max(0.001, self.hard_deadline-time.monotonic()))
        sealed = None
        try:
            self.prepare_and_run()
        except BaseException as error:
            self.state["errors"].append({"phase": "execution", "type": type(error).__name__,
                                         "message": str(error)[:1200]})
        finally:
            if self.latch.reason is None:
                try:
                    self.close_device()
                except BaseException as error:
                    self.state["errors"].append({"phase": "device-close", "type": type(error).__name__,
                                                 "message": str(error)[:1200]})
                try:
                    if self.before is not None:
                        after = base.source_binding.source_inventory(self.repo,
                            self.context["sourceCommitSha"], self.private / "source-after", self.critical)
                        require(after == self.before, "HOSTED_CONTROLLER_SOURCE_CHANGED")
                        self.state["sourceUnchanged"] = True
                except BaseException as error:
                    self.state["errors"].append({"phase": "source-close", "type": type(error).__name__,
                                                 "message": str(error)[:1200]})
            if self.state["errors"] or self.latch.reason is not None:
                self.state["status"] = "FAIL"
            self.state.update(firstInterruption=self.latch.reason,
                elapsedSeconds=time.monotonic()-self.started, finishedUTC=base.library.now())
            record(self.private / "controller-result.original.json", self.state)
            try:
                sealed = self.seal_originals()
            except BaseException as error:
                self.state["status"] = "FAIL"
                self.state["errors"].append({"phase": "seal", "type": type(error).__name__})
            summary = {"schema": SCHEMA, "sourceCommitSha": self.context["sourceCommitSha"],
                "runId": self.context["runId"], "runAttempt": self.context["runAttempt"],
                "caseId": self.context["caseId"], "executionLocation": "GITHUB_ACTIONS_MACOS",
                "status": self.state["status"], "sourceUnchanged": self.state.get("sourceUnchanged", False),
                "nativeChildInvocations": self.state["nativeChildInvocations"],
                "nativeChildInvocationScope": "OUTER_OWNER_SUBPROCESSES_ONLY",
                "innerNativeResultParsed": self.native_result is not None,
                "nativeOwnerInvocations": self.state["nativeOwnerInvocations"],
                "runtimeObservationAuthenticated": bool(self.native_result and
                    self.native_result.get("runtimeObservationAuthenticated") is True),
                "privateOriginalsEncrypted": sealed is not None, "encryption": sealed,
                "errorCategories": [{"phase": e["phase"], "type": e["type"]} for e in self.state["errors"]],
                "signingRepeated": False, "rawNativeTypesObserved": False, "formalAcceptanceGranted": False}
            try:
                record(self.paths["public"] / "summary.json", summary)
                require({p.name for p in self.paths["public"].iterdir()} ==
                        ({"summary.json", "fn04-originals.zip.p7m"} if sealed else {"summary.json"}),
                        "CLOSED_PUBLIC_OUTPUTS")
            finally:
                signal.setitimer(signal.ITIMER_REAL, 0)
                for signum, handler in previous.items():
                    signal.signal(signum, handler)
        print(json.dumps({key: summary[key] for key in ("status", "runId", "caseId",
            "privateOriginalsEncrypted", "runtimeObservationAuthenticated", "formalAcceptanceGranted")},
            sort_keys=True))
        return 0 if summary["status"] == "OBSERVATION_VERIFIED" and sealed else 1


def main():
    os.umask(0o077)
    os.environ["PATH"] = str(Path(sys.executable).parent) + os.pathsep + os.environ.get("PATH", "")
    return HostedCaller().run()


if __name__ == "__main__":
    raise SystemExit(main())
