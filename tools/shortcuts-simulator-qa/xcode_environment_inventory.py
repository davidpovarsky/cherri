#!/usr/bin/env python3
"""Bounded, read-only Xcode inventory. Only ciphertext and an allowlist leave the runner."""

import argparse
import datetime as dt
import hashlib
import json
import os
from pathlib import Path
import plistlib
import re
import signal
import stat
import subprocess
import sys
import threading
import time
import zipfile

SCHEMA = "cherri-xcode-environment-inventory/v1"
TOTAL_SECONDS = 150
COLLECTION_SECONDS = 125
ENCRYPT_SECONDS = 20
MAX_STREAM_BYTES = 2 * 1024 * 1024
MAX_METADATA_BYTES = 1024 * 1024
MAX_XCODES = 8
MAX_APPLICATIONS = 32
CERTIFICATE_SHA256 = "55956e825597a2e60acca4045e4ea8040ae27f17caa7964e9b4493824fa0c1f1"
RECIPIENT_NAME = "xcode_environment_inventory_recipient.pem"
RECEIVER_PROJECT = "tools/shortcuts-simulator-qa/XcodeProject/CherriSimulatorQA.xcodeproj"


class InventoryError(ValueError):
    pass


def require(condition, code):
    if not condition:
        raise InventoryError(code)


def canonical(value):
    return (json.dumps(value, ensure_ascii=False, sort_keys=True, separators=(",", ":"), allow_nan=False) + "\n").encode()


def digest(raw):
    return hashlib.sha256(raw).hexdigest()


def utc_now():
    return dt.datetime.now(dt.timezone.utc).isoformat().replace("+00:00", "Z")


def write_new(path, raw):
    with Path(path).open("xb") as stream:
        stream.write(raw)


def read_regular(path, limit):
    path = Path(path)
    before = path.lstat()
    require(stat.S_ISREG(before.st_mode) and not path.is_symlink(), "NOT_REGULAR")
    require(0 <= before.st_size <= limit, "INPUT_LIMIT")
    with path.open("rb") as stream:
        opened = os.fstat(stream.fileno())
        require((opened.st_dev, opened.st_ino) == (before.st_dev, before.st_ino), "INPUT_REPLACED")
        raw = stream.read(limit + 1)
        after_fd = os.fstat(stream.fileno())
    after = path.lstat()
    require(stat.S_ISREG(after.st_mode) and not path.is_symlink()
            and (after.st_dev, after.st_ino) == (before.st_dev, before.st_ino)
            and before.st_size == after_fd.st_size == after.st_size == len(raw)
            and before.st_mtime_ns == after_fd.st_mtime_ns == after.st_mtime_ns
            and len(raw) <= limit, "INPUT_CHANGED_OR_LIMIT")
    return raw


def command_plan():
    # These fixed arguments cannot boot, open, install, erase, build or select Xcode.
    return (
        ("source-head", ("/usr/bin/git", "rev-parse", "HEAD"), 5),
        ("source-status", ("/usr/bin/git", "status", "--porcelain"), 5),
        ("receiver-project-tracked", ("/usr/bin/git", "ls-files", "--", RECEIVER_PROJECT + "/project.pbxproj"), 5),
        ("selected-developer", ("/usr/bin/xcode-select", "-p"), 10),
        ("resolved-simctl", ("/usr/bin/xcrun", "--find", "simctl"), 10),
        ("resolved-xcodebuild", ("/usr/bin/xcrun", "--find", "xcodebuild"), 10),
        ("simulator-devices", ("/usr/bin/xcrun", "simctl", "list", "devices", "--json"), 25),
        ("simulator-runtimes", ("/usr/bin/xcrun", "simctl", "list", "runtimes", "--json"), 25),
    )


def stop_process(process):
    try:
        if os.name == "posix":
            os.killpg(process.pid, signal.SIGKILL)
        else:
            process.kill()
    except (ProcessLookupError, OSError):
        pass


def run_command(label, argv, seconds, raw_root, deadline, *, clock=time.monotonic):
    start = clock()
    row = {"id": label, "argv": list(argv), "budget_seconds": seconds,
           "started_at": utc_now(), "status": "NOT_STARTED", "exit_code": None,
           "timed_out": False, "output_limit": False, "streams_complete": False}
    remaining = min(seconds, deadline - start)
    if remaining <= 0:
        row.update(finished_at=utc_now(), elapsed_seconds=0.0)
        write_new(raw_root / (label + ".process.json"), canonical(row))
        return row
    overflow, reader_error = threading.Event(), threading.Event()
    paths = {name: raw_root / (label + "." + name + ".original") for name in ("stdout", "stderr")}
    options = {"start_new_session": True} if os.name == "posix" else {"creationflags": subprocess.CREATE_NEW_PROCESS_GROUP}
    process = None
    readers = []
    try:
        process = subprocess.Popen(list(argv), stdin=subprocess.DEVNULL, stdout=subprocess.PIPE,
                                   stderr=subprocess.PIPE, **options)

        def drain(pipe, target):
            try:
                with target.open("xb") as destination:
                    retained = 0
                    while True:
                        chunk = pipe.read(65536)
                        if not chunk:
                            break
                        take = min(len(chunk), MAX_STREAM_BYTES + 1 - retained)
                        if take > 0:
                            destination.write(chunk[:take])
                            retained += take
                        if retained > MAX_STREAM_BYTES or take < len(chunk):
                            overflow.set()
                            stop_process(process)
                            break
            except (OSError, ValueError):
                reader_error.set()
                stop_process(process)
            finally:
                pipe.close()

        for name in ("stdout", "stderr"):
            thread = threading.Thread(target=drain, args=(getattr(process, name), paths[name]), daemon=True)
            thread.start()
            readers.append(thread)
        try:
            process.wait(timeout=remaining)
        except subprocess.TimeoutExpired:
            row["timed_out"] = True
            stop_process(process)
            process.wait(timeout=3)
        for thread in readers:
            thread.join(timeout=1)
        if any(thread.is_alive() for thread in readers):
            stop_process(process)
            for thread in readers:
                thread.join(timeout=1)
            reader_error.set()
        row["exit_code"] = process.returncode
        row["output_limit"] = overflow.is_set()
        row["streams_complete"] = not (row["timed_out"] or overflow.is_set() or reader_error.is_set())
        row["status"] = ("TIMEOUT" if row["timed_out"] else "OUTPUT_LIMIT" if overflow.is_set()
                         else "CAPTURE_ERROR" if reader_error.is_set() else "SUCCESS" if process.returncode == 0 else "EXIT_NONZERO")
    except InventoryError:
        if process is not None:
            stop_process(process)
        raise
    except (OSError, subprocess.SubprocessError):
        if process is not None:
            stop_process(process)
        row["status"] = "PROCESS_ERROR"
    row.update(finished_at=utc_now(), elapsed_seconds=round(clock() - start, 6))
    row["originals"] = {}
    for name, path in paths.items():
        if path.exists():
            raw = read_regular(path, MAX_STREAM_BYTES + 1)
            row["originals"][path.name] = {"sha256": digest(raw), "bytes": len(raw), "complete": row["streams_complete"]}
    write_new(raw_root / (label + ".process.json"), canonical(row))
    return row


def path_observation(path):
    path = Path(path)
    result = {"path": str(path), "realpath": os.path.realpath(path), "exists": path.exists(),
              "is_symlink": path.is_symlink(), "is_directory": path.is_dir(), "is_file": path.is_file()}
    if path.exists():
        st = path.stat()
        result.update(size=st.st_size, mode=stat.S_IMODE(st.st_mode), modified_ns=st.st_mtime_ns)
    return result


def capture_layout(raw_root, command_rows, deadline, *, applications=Path("/Applications"), clock=time.monotonic):
    layout = {"schema": SCHEMA, "developer_dir_environment": os.environ.get("DEVELOPER_DIR"),
              "applications_root": path_observation(applications), "xcodes": [], "resolved_tools": {},
              "limits": {"xcodes": MAX_XCODES, "developer_applications_per_xcode": MAX_APPLICATIONS}}
    for label in ("selected-developer", "resolved-simctl", "resolved-xcodebuild"):
        row = next(item for item in command_rows if item["id"] == label)
        if row["status"] == "SUCCESS":
            raw = read_regular(raw_root / (label + ".stdout.original"), MAX_STREAM_BYTES)
            value = raw.decode("utf-8", errors="strict").strip()
            if value.startswith("/") and "\n" not in value and "\x00" not in value:
                layout["resolved_tools"][label] = path_observation(value)
                if label != "selected-developer" and Path(value).is_file():
                    require(clock() < deadline, "COLLECTION_DEADLINE")
                    tool_bytes = read_regular(Path(os.path.realpath(value)), 64 * 1024 * 1024)
                    layout["resolved_tools"][label]["sha256"] = digest(tool_bytes)
    # One directory level only. No home directory, device content or SDK tree walk.
    candidates = sorted(path for path in applications.iterdir()
                        if path.name.startswith("Xcode") and path.name.endswith(".app")) if applications.is_dir() else []
    layout["xcode_candidates_seen"] = len(candidates)
    layout["xcode_candidates_truncated"] = len(candidates) > MAX_XCODES
    for index, xcode in enumerate(candidates[:MAX_XCODES]):
        require(clock() < deadline, "COLLECTION_DEADLINE")
        developer_apps = xcode / "Contents" / "Developer" / "Applications"
        entry = {"bundle": path_observation(xcode), "developer_applications": path_observation(developer_apps),
                 "simulator": path_observation(developer_apps / "Simulator.app"), "applications": []}
        apps = sorted(developer_apps.iterdir()) if developer_apps.is_dir() else []
        entry["application_entries_seen"] = len(apps)
        entry["application_entries_truncated"] = len(apps) > MAX_APPLICATIONS
        for app in apps[:MAX_APPLICATIONS]:
            require(clock() < deadline, "COLLECTION_DEADLINE")
            entry["applications"].append(path_observation(app))
        for tag, info in (("xcode", xcode / "Contents" / "Info.plist"),
                          ("simulator", developer_apps / "Simulator.app" / "Contents" / "Info.plist")):
            if info.is_file():
                try:
                    data = read_regular(info, MAX_METADATA_BYTES)
                    name = f"bundle-{index:02d}-{tag}.Info.plist.original"
                    write_new(raw_root / name, data)
                    parsed = plistlib.loads(data)
                    require(isinstance(parsed, dict), "APP_METADATA_TYPE")
                    entry[tag + "_metadata"] = {"status": "PARSED", "original": name, "sha256": digest(data), "bytes": len(data),
                        "selected_values": {key: parsed[key] for key in ("CFBundleIdentifier", "CFBundleName", "CFBundleVersion", "CFBundleShortVersionString", "DTXcodeBuild") if key in parsed and isinstance(parsed[key], (str, int, float, bool))}}
                except (OSError, ValueError, plistlib.InvalidFileException) as error:
                    entry[tag + "_metadata"] = {"status": "METADATA_UNADJUDICATED", "error_type": type(error).__name__}
        layout["xcodes"].append(entry)
    write_new(raw_root / "layout.original.json", canonical(layout))
    return layout


def capture_simulator_state(raw_root, rows, *, repo=Path(".")):
    result = {"schema": SCHEMA, "scope": "read_only_inventory_not_headless_xctest_or_gui_proof"}
    for kind, key in (("devices", "devices"), ("runtimes", "runtimes")):
        row = next(item for item in rows if item["id"] == "simulator-" + kind)
        parsed = {"status": "NOT_PARSED", "entries": []}
        if row["status"] == "SUCCESS" and row["streams_complete"]:
            try:
                body = json.loads(read_regular(raw_root / ("simulator-" + kind + ".stdout.original"), MAX_STREAM_BYTES))
                values = body[key]
                if kind == "devices":
                    require(isinstance(values, dict) and all(isinstance(v, list) for v in values.values()), "DEVICE_SHAPE")
                    for runtime, devices in values.items():
                        for device in devices:
                            require(isinstance(device, dict), "DEVICE_SHAPE")
                            parsed["entries"].append({"runtime": runtime, **{name: device[name] for name in ("name", "udid", "state", "isAvailable") if name in device}})
                else:
                    require(isinstance(values, list) and all(isinstance(v, dict) for v in values), "RUNTIME_SHAPE")
                    parsed["entries"] = [{name: runtime[name] for name in ("name", "identifier", "version", "buildversion", "isAvailable") if name in runtime} for runtime in values]
                parsed["status"] = "PARSED"
            except (OSError, ValueError, KeyError, TypeError) as error:
                parsed = {"status": "UNADJUDICATED", "entries": [], "error_type": type(error).__name__}
        result[kind] = parsed
    tracked_row = next(item for item in rows if item["id"] == "receiver-project-tracked")
    tracked = False
    if tracked_row["status"] == "SUCCESS":
        tracked = read_regular(raw_root / "receiver-project-tracked.stdout.original", MAX_STREAM_BYTES).decode().strip() == RECEIVER_PROJECT + "/project.pbxproj"
    project = repo / RECEIVER_PROJECT
    result["receiver_project"] = {"path": str(project), "present": project.is_dir(), "tracked": tracked,
        "scheme": "CherriSimulatorQAReceiver", "showdestinations_status": "NOT_STARTED_SOURCE_PROJECT_ABSENT" if not tracked else "NOT_STARTED_UNEXPECTED_PROJECT",
        "project_generated": False}
    write_new(raw_root / "simulator-state.original.json", canonical(result))
    return result


def make_archive(raw_root, destination):
    entries = {}
    for path in sorted(raw_root.iterdir()):
        raw = read_regular(path, MAX_STREAM_BYTES + 65536)
        entries[path.name] = {"sha256": digest(raw), "bytes": len(raw)}
    write_new(raw_root / "originals-manifest.json", canonical({"schema": SCHEMA, "originals": entries}))
    with zipfile.ZipFile(destination, "x", compression=zipfile.ZIP_DEFLATED) as archive:
        for path in sorted(raw_root.iterdir()):
            archive.writestr(path.name, read_regular(path, MAX_STREAM_BYTES + 65536))
    return entries


def encrypt_archive(archive, recipient, ciphertext, deadline, *, openssl="/usr/bin/openssl", clock=time.monotonic):
    certificate = read_regular(recipient, 8192)
    require(digest(certificate) == CERTIFICATE_SHA256, "RECIPIENT_CHANGED")
    require(not ciphertext.exists(), "CIPHERTEXT_EXISTS")
    remaining = min(ENCRYPT_SECONDS, deadline - clock())
    require(remaining > 0, "ENCRYPTION_DEADLINE")
    # Use the explicit SMIME DER/AES-256 route; actual runner support is observed.
    # The plaintext ZIP digest is separately retained for post-decryption equality.
    result = subprocess.run([openssl, "smime", "-encrypt", "-binary", "-aes256", "-outform", "DER",
                             "-in", str(archive), "-out", str(ciphertext), str(recipient)],
                            stdin=subprocess.DEVNULL, capture_output=True, timeout=remaining, check=False)
    require(result.returncode == 0 and not result.stdout and not result.stderr, "ENCRYPTION_FAILED")
    require(ciphertext.is_file() and ciphertext.stat().st_size > 0 and clock() <= deadline, "ENCRYPTION_INCOMPLETE")
    return {"status": "ENCRYPTED", "recipient_sha256": digest(certificate), "cipher": "SMIME-DER-AES256",
            "stdout_sha256": digest(result.stdout), "stderr_sha256": digest(result.stderr)}


def public_summary(source_sha, run_id, rows, layout, simulator, entries, archive, ciphertext, encryption, *, source_valid=True):
    # Deliberately construct every field. Never copy an arbitrary metadata/result dictionary.
    return {"schema": SCHEMA, "status": "DIAGNOSTIC_CAPTURED" if source_valid else "DIAGNOSTIC_CAPTURED_SOURCE_REJECTED", "source_sha": source_sha, "run_id": run_id,
        "commands": [{key: row[key] for key in ("id", "status", "exit_code", "timed_out", "output_limit", "streams_complete", "budget_seconds", "elapsed_seconds")} for row in rows],
        "xcode_bundle_count": len(layout.get("xcodes", [])),
        "standard_simulator_directory_count": sum(item["simulator"]["is_directory"] for item in layout.get("xcodes", [])),
        "device_inventory_parsed": simulator["devices"]["status"] == "PARSED",
        "runtime_inventory_parsed": simulator["runtimes"]["status"] == "PARSED",
        "device_count": len(simulator["devices"]["entries"]), "runtime_count": len(simulator["runtimes"]["entries"]),
        "available_device_count": sum(item.get("isAvailable") is True for item in simulator["devices"]["entries"]),
        "available_runtime_count": sum(item.get("isAvailable") is True for item in simulator["runtimes"]["entries"]),
        "unknown_device_availability_count": sum(type(item.get("isAvailable")) is not bool for item in simulator["devices"]["entries"]),
        "unknown_runtime_availability_count": sum(type(item.get("isAvailable")) is not bool for item in simulator["runtimes"]["entries"]),
        "device_state_counts": {state: sum(item.get("state") == state for item in simulator["devices"]["entries"]) for state in ("Booted", "Booting", "Shutdown", "Shutting Down", "Creating")},
        "receiver_project_present": simulator["receiver_project"]["present"],
        "receiver_project_tracked": simulator["receiver_project"]["tracked"],
        "showdestinations_status": simulator["receiver_project"]["showdestinations_status"],
        "layout_complete": not layout.get("xcode_candidates_truncated", False) and not any(item.get("application_entries_truncated", False) for item in layout.get("xcodes", [])),
        "metadata_unadjudicated_count": sum(value.get("status") == "METADATA_UNADJUDICATED" for item in layout.get("xcodes", []) for key, value in item.items() if key.endswith("_metadata")),
        "original_file_count": len(entries) + 1, "plaintext_zip_sha256": digest(archive.read_bytes()),
        "ciphertext_sha256": digest(ciphertext.read_bytes()), "ciphertext_bytes": ciphertext.stat().st_size,
        "recipient_sha256": encryption["recipient_sha256"], "encryption": encryption["cipher"],
        "budgets_seconds": {"aggregate": TOTAL_SECONDS, "collection": COLLECTION_SECONDS, "encryption": ENCRYPT_SECONDS},
        "source_clean": source_valid, "read_only_inventory": True, "boot_attempted": False, "build_attempted": False,
        "apple_test_executed": False, "headless_xctest_verified": False,
        "gui_absence_proves_runtime_unavailable": False, "formal_acceptance_granted": False}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", required=True, type=Path)
    parser.add_argument("--source-sha", required=True)
    parser.add_argument("--run-id", required=True)
    args = parser.parse_args()
    start = time.monotonic()
    old_alarm = None
    try:
        require(sys.platform == "darwin", "MACOS_ONLY")
        def aggregate_deadline(_signum, _frame):
            raise InventoryError("AGGREGATE_DEADLINE")
        old_alarm = signal.signal(signal.SIGALRM, aggregate_deadline)
        signal.alarm(TOTAL_SECONDS)
        require(re.fullmatch(r"[0-9a-f]{40}", args.source_sha) is not None and re.fullmatch(r"[0-9]{1,20}", args.run_id) is not None, "EXECUTION_IDENTITY")
        require(args.output.is_absolute() and not args.output.exists(), "OUTPUT_MUST_BE_EXCLUSIVE")
        args.output.mkdir(mode=0o700)
        raw_root, public = args.output / "private-raw", args.output / "public"
        raw_root.mkdir(mode=0o700)
        public.mkdir()
        rows = [run_command(label, argv, seconds, raw_root, start + COLLECTION_SECONDS)
                for label, argv, seconds in command_plan()]
        source_head_path, source_status_path = raw_root / "source-head.stdout.original", raw_root / "source-status.stdout.original"
        source_head = read_regular(source_head_path, MAX_STREAM_BYTES + 1).decode(errors="replace").strip() if source_head_path.exists() else ""
        source_status = read_regular(source_status_path, MAX_STREAM_BYTES + 1) if source_status_path.exists() else b"missing"
        source_valid = rows[0]["status"] == rows[1]["status"] == "SUCCESS" and source_head == args.source_sha and source_status == b""
        layout = capture_layout(raw_root, rows, start + COLLECTION_SECONDS)
        simulator = capture_simulator_state(raw_root, rows)
        write_new(raw_root / "execution.original.json", canonical({"schema": SCHEMA, "source_sha": args.source_sha,
            "run_id": args.run_id, "started_at": rows[0]["started_at"], "finished_at": utc_now(), "commands": rows,
            "collector_sha256": digest(read_regular(Path(__file__), MAX_METADATA_BYTES))}))
        archive = args.output / "inventory.original.zip"
        entries = make_archive(raw_root, archive)
        ciphertext = public / "inventory.zip.p7m"
        encryption = encrypt_archive(archive, Path(__file__).with_name(RECIPIENT_NAME), ciphertext, start + TOTAL_SECONDS)
        summary = public_summary(args.source_sha, args.run_id, rows, layout, simulator, entries, archive, ciphertext, encryption, source_valid=source_valid)
        require(time.monotonic() <= start + TOTAL_SECONDS, "AGGREGATE_DEADLINE")
        write_new(public / "summary.json", canonical(summary))
        print(json.dumps({"status": summary["status"], "formal_acceptance_granted": False}))
        return 0 if source_valid else 1
    except (OSError, ValueError, UnicodeError, subprocess.SubprocessError, plistlib.InvalidFileException):
        # Raw diagnostics and paths are never printed or added to the upload list.
        print("XCODE_INVENTORY_NOT_COMPLETED", file=sys.stderr)
        return 1
    finally:
        if old_alarm is not None:
            signal.alarm(0)
            signal.signal(signal.SIGALRM, old_alarm)


if __name__ == "__main__":
    raise SystemExit(main())
