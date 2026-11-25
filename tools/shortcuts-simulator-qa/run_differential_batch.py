#!/usr/bin/env python3
"""Run signed differential fixtures through real Shortcuts and the existing receiver."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import signal
import subprocess
import time
import urllib.parse
import uuid

def atomic_json(path, value):
    tmp = path.with_suffix(path.suffix + ".tmp")
    tmp.write_text(json.dumps(value, ensure_ascii=False, indent=2) + "\n")
    tmp.replace(path)

def load_json(path, default=None):
    try:
        return json.loads(path.read_text())
    except (FileNotFoundError, json.JSONDecodeError):
        return default

def wait_file(path, seconds, process):
    end = time.monotonic() + seconds
    while time.monotonic() < end:
        value = load_json(path)
        if value is not None:
            return value
        if process.poll() is not None:
            raise RuntimeError("XCTest ended before " + path.name)
        time.sleep(0.25)
    raise TimeoutError(path.name)

def command(args, timeout=25):
    result = subprocess.run(args, text=True, capture_output=True, timeout=timeout)
    if result.returncode:
        raise RuntimeError(json.dumps({"command": args, "exit": result.returncode,
                                      "stderr": result.stderr[-1500:]}))
    return result.stdout.strip()

def classify_callback(callback, case, nonce):
    if callback is None:
        return "NO_CALLBACK"
    if (callback.get("receiverSelfTest") is not False
            or callback.get("eventSource") != "shortcuts-x-callback"
            or callback.get("runId") != nonce
            or callback.get("expectedShortcutName") != case["name"]
            or callback.get("fixtureId") != case["id"]):
        return "INVALID_CALLBACK_IDENTITY"
    if callback.get("status") != "SUCCESS":
        return "RUNTIME_ERROR"
    expected = case.get("expected")
    if expected is None:
        return "OBSERVED_ONLY"
    if callback.get("resultSummary") != expected:
        return "OUTPUT_MISMATCH"
    # Summary normalization can turn CR into LF without changing byte count.
    # Require the receiver's digest of the original UTF-8 bytes as well, and
    # never promote historical receipts that did not record that digest.
    expected_bytes = expected.encode("utf-8")
    shape = callback.get("resultShape", {})
    if shape.get("utf8ByteCount") != len(expected_bytes):
        return "RAW_OUTPUT_LENGTH_MISMATCH"
    if shape.get("utf8SHA256") != hashlib.sha256(expected_bytes).hexdigest():
        return "RAW_OUTPUT_DIGEST_MISMATCH"
    return "PASS"

def run(manifest_path, directory, simulator, harness, derived, xctestrun=None,
        restart_shortcuts_after_import=False):
    manifest = load_json(manifest_path)
    assert isinstance(manifest, dict) and manifest["cases"]
    directory.mkdir(parents=True, exist_ok=False)
    atomic_json(directory / "manifest.json", manifest)
    names = [case["name"] for case in manifest["cases"]]
    assert len(names) == len(set(names)), "Native fixture names must be unique"
    args = ["xcodebuild", "test-without-building", "-project",
            str(harness / "CherriSimulatorQA.xcodeproj"), "-scheme", "CherriSimulatorQAReceiver",
            "-destination", "platform=iOS Simulator,id=" + simulator,
            "-derivedDataPath", str(derived), "-parallel-testing-enabled", "NO",
            "-only-testing:CherriSimulatorQAUITests/CherriDifferentialBatchUITests/testDifferentialImportsAndRuntimeConsent",
            "-resultBundlePath", str(directory / "NativeDifferential.xcresult")]
    if xctestrun is not None:
        args = ["xcodebuild", "test-without-building", "-xctestrun", str(xctestrun)] + args[6:8] + args[10:]
    env = os.environ.copy()
    env["TEST_RUNNER_CHERRI_QA_DIFF_RUN_DIR"] = str(directory)
    results = []
    process = None
    fatal = None
    with (directory / "xctest.log").open("w") as log:
        try:
            process = subprocess.Popen(args, stdout=log, stderr=subprocess.STDOUT,
                                       env=env, start_new_session=True)
            atomic_json(directory / "process.json", {"pid": process.pid, "command": args})
            wait_file(directory / "ready.json", 120, process)
            container = Path(command(["xcrun", "simctl", "get_app_container",
                                      simulator, "org.cherrilang.qa.Receiver", "data"]))
            callbacks_path = container / "Documents/callback-events.json"
            for case in manifest["cases"]:
                case_id = case["id"]
                signed = Path(case["signedPath"])
                data = signed.read_bytes()
                assert data[:4] == b"AEA1", "Unsigned native input"
                signed_hash = hashlib.sha256(data).hexdigest()
                assert signed_hash == case["signedSHA256"], "Signed input changed"
                row = {"id": case_id, "lane": case["lane"], "name": case["name"],
                       "signedSHA256": signed_hash, "expected": case.get("expected"),
                       "restartShortcutsAfterImport": restart_shortcuts_after_import,
                       "startedUTC": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())}
                import_id = "import-" + uuid.uuid4().hex
                command(["xcrun", "simctl", "openurl", simulator, signed.as_uri()], timeout=60)
                atomic_json(directory / "command.json",
                            {"id": import_id, "operation": "import", "shortcutName": case["name"]})
                receipt = wait_file(directory / ("ack-" + import_id + ".json"), 90, process)
                row["importReceipt"] = receipt
                if receipt["outcome"] != "IMPORTED":
                    row["status"] = "IMPORT_FAILED"
                    results.append(row)
                    atomic_json(directory / "results.json", results)
                    continue
                if restart_shortcuts_after_import:
                    restart_command = ["xcrun", "simctl", "terminate", simulator, "com.apple.shortcuts"]
                    restart_receipt = {"command": restart_command, "outcome": "REQUESTED",
                                       "importId": import_id, "fixtureId": case_id,
                                       "shortcutName": case["name"],
                                       "startedUTC": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())}
                    restart_path = directory / ("restart-" + import_id + ".json")
                    row["restartReceipt"] = restart_receipt
                    atomic_json(restart_path, restart_receipt)
                    try:
                        command(restart_command)
                    except Exception as exc:
                        restart_receipt.update({"outcome": "FAILED", "error": str(exc)})
                        atomic_json(restart_path, restart_receipt)
                        row["status"] = "SHORTCUTS_RESTART_FAILED"
                        results.append(row)
                        atomic_json(directory / "results.json", results)
                        raise
                    restart_receipt.update({"outcome": "TERMINATED",
                                            "completedUTC": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())})
                    atomic_json(restart_path, restart_receipt)
                nonce = "diff-" + uuid.uuid4().hex
                row["nonce"] = nonce
                query = {"name": case["name"], "x-source": "CherriSimulatorQAReceiver"}
                if case.get("input") is not None:
                    query["input"] = "text"
                    query["text"] = (case["input"] if isinstance(case["input"], str)
                                     else json.dumps(case["input"], ensure_ascii=False))
                for status in ("success", "error", "cancel"):
                    query["x-" + status] = "cherriqa://" + status + "?" + urllib.parse.urlencode(
                        {"runId": nonce, "fixtureId": case_id, "shortcutName": case["name"],
                         "source": "shortcuts-x-callback"})
                target = "shortcuts://x-callback-url/run-shortcut?" + urllib.parse.urlencode(query)
                trigger = "cherriqa://trigger?" + urllib.parse.urlencode({"target": target})
                command(["xcrun", "simctl", "openurl", simulator, trigger], timeout=60)
                runtime_id = "runtime-" + uuid.uuid4().hex
                atomic_json(directory / "command.json",
                            {"id": runtime_id, "operation": "runtime", "shortcutName": case["name"]})
                callback = None
                end = time.monotonic() + 48
                while time.monotonic() < end:
                    events = load_json(callbacks_path, [])
                    matches = [event for event in events if event.get("runId") == nonce]
                    if matches:
                        assert len(matches) == 1, "Duplicate callback nonce"
                        callback = matches[0]
                        atomic_json(directory / ("completion-" + runtime_id + ".json"), callback)
                        break
                    if process.poll() is not None:
                        raise RuntimeError("XCTest ended during runtime")
                    time.sleep(0.25)
                row["runtimeUIReceipt"] = wait_file(directory / ("ack-" + runtime_id + ".json"), 60, process)
                row["callback"] = callback
                row["status"] = classify_callback(callback, case, nonce)
                results.append(row)
                atomic_json(directory / "results.json", results)
                print(json.dumps({"id": case_id, "lane": case["lane"], "status": row["status"],
                                  "actual": (callback or {}).get("resultSummary")}), flush=True)
        except Exception as exc:
            fatal = str(exc)
        finally:
            if process is not None and process.poll() is None:
                atomic_json(directory / "command.json", {"id": "finish", "operation": "finish"})
                try:
                    process.wait(timeout=90)
                except subprocess.TimeoutExpired:
                    os.killpg(process.pid, signal.SIGINT)
                    try:
                        process.wait(timeout=15)
                    except subprocess.TimeoutExpired:
                        os.killpg(process.pid, signal.SIGTERM)
                        process.wait(timeout=15)
    summary = {"schemaVersion": 1, "simulator": simulator, "casesRequested": len(manifest["cases"]),
               "casesObserved": len(results), "counts": {}, "fatal": fatal,
               "xctestExit": process.returncode if process else None,
               "manifestSHA256": hashlib.sha256(manifest_path.read_bytes()).hexdigest(),
               "callbackContract": "raw-utf8-sha256-v1",
               "restartShortcutsAfterImport": restart_shortcuts_after_import}
    for row in results:
        summary["counts"][row["status"]] = summary["counts"].get(row["status"], 0) + 1
    atomic_json(directory / "summary.json", summary)
    print(json.dumps(summary), flush=True)
    return 0 if not fatal and summary["counts"].get("PASS", 0) == len(manifest["cases"]) and summary["xctestExit"] == 0 else 1

if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--manifest", type=Path, required=True)
    parser.add_argument("--out", type=Path, required=True)
    parser.add_argument("--simulator", required=True)
    parser.add_argument("--harness", type=Path, required=True)
    parser.add_argument("--derived-data", type=Path, required=True)
    parser.add_argument("--xctestrun", type=Path)
    parser.add_argument("--restart-shortcuts-after-import", action="store_true",
                        help="Explicitly terminate Shortcuts on the selected simulator after each verified import.")
    options = parser.parse_args()
    raise SystemExit(run(options.manifest, options.out, options.simulator, options.harness, options.derived_data, options.xctestrun,
                         options.restart_shortcuts_after_import))
