#!/usr/bin/env python3
"""Select one existing simulator from an explicit source/run-bound owner record.

This module only reads the original inventories. It never boots, creates, erases,
opens, or substitutes another device. The ordinary dedicated-runner selector
remains in the shell and is used only when no owner record was requested.
"""
import argparse
import hashlib
import json
from pathlib import Path
import re
import sys

SCHEMA = "cherri-owned-simulator-selection/1"
RECORD_FIELDS = {"schema", "sourceCommit", "runId", "udid", "deviceName",
                 "deviceTypeIdentifier", "runtimeId", "runtimeName", "runtimeVersion"}
UUID = re.compile(r"[0-9A-F]{8}(?:-[0-9A-F]{4}){3}-[0-9A-F]{12}")
SOURCE = re.compile(r"[0-9a-f]{40}")
RUN = re.compile(r"[A-Za-z0-9_-]{1,64}")


def require(ok, code):
    if not ok:
        raise ValueError("OWNED_SIMULATOR_" + code)


def unique_json(raw):
    def unique(pairs):
        value = {}
        for key, item in pairs:
            require(key not in value, "DUPLICATE_KEY")
            value[key] = item
        return value
    return json.loads(raw, object_pairs_hook=unique)


def read_input(path, limit, bindings, label, retained_dir=None, parse_json=True):
    path = Path(path)
    require(path.is_file() and not path.is_symlink(), "INPUT_NOT_REGULAR")
    require(path.stat().st_size <= limit, "INPUT_TOO_LARGE")
    raw = path.read_bytes()
    require(len(raw) <= limit, "INPUT_TOO_LARGE")
    bindings[label] = {"bytes": len(raw), "sha256": hashlib.sha256(raw).hexdigest()}
    if retained_dir is not None:
        require(label in ("ownerRecord", "runtimes", "deviceTypes", "devices"), "INPUT_LABEL")
        retained = retained_dir / (label + ".json")
        with retained.open("xb") as stream:
            stream.write(raw)
        bindings[label]["retainedOriginal"] = str(Path(retained_dir.name) / retained.name)
    return unique_json(raw) if parse_json else raw


def select_owned(record, runtimes, types, devices, source, run_id, profile):
    require(isinstance(source, str) and SOURCE.fullmatch(source), "SOURCE_INVALID")
    require(isinstance(run_id, str) and RUN.fullmatch(run_id), "RUN_INVALID")
    require(profile in ("ipad-default", "iphone-fallback"), "PROFILE_INVALID")
    require(isinstance(record, dict) and set(record) == RECORD_FIELDS, "RECORD_FIELDS")
    require(all(isinstance(v, str) and v and len(v) <= 256 for v in record.values()), "RECORD_VALUES")
    require(record["schema"] == SCHEMA, "SCHEMA")
    require(record["sourceCommit"] == source, "SOURCE_MISMATCH")
    require(record["runId"] == run_id, "RUN_MISMATCH")
    require(UUID.fullmatch(record["udid"]), "DEVICE_ID_INVALID")
    require(isinstance(runtimes, dict) and isinstance(runtimes.get("runtimes"), list), "RUNTIME_INVENTORY")
    require(isinstance(types, dict) and isinstance(types.get("devicetypes"), list), "TYPE_INVENTORY")
    require(isinstance(devices, dict) and isinstance(devices.get("devices"), dict), "DEVICE_INVENTORY")
    runtime_rows = runtimes["runtimes"]
    type_rows = types["devicetypes"]
    device_groups = devices["devices"]
    require(all(isinstance(r, dict) for r in runtime_rows + type_rows), "INVENTORY_ROW")
    require(all(isinstance(group, list) and all(isinstance(d, dict) for d in group)
                for group in device_groups.values()), "DEVICE_ROW")
    runtime_matches = [r for r in runtime_rows if r.get("identifier") == record["runtimeId"]]
    require(len(runtime_matches) == 1, "RUNTIME_MISSING_OR_AMBIGUOUS")
    runtime = runtime_matches[0]
    require(runtime.get("isAvailable") is True, "RUNTIME_UNAVAILABLE")
    require(runtime.get("name") == record["runtimeName"]
            and runtime.get("version") == record["runtimeVersion"], "RUNTIME_IDENTITY")
    require(record["runtimeId"].startswith("com.apple.CoreSimulator.SimRuntime.iOS-")
            and re.fullmatch(r"27(?:\.[0-9]+){1,2}", record["runtimeVersion"])
            and record["runtimeName"].startswith("iOS "), "RUNTIME_PLATFORM")
    found = [(rid, d) for rid, group in device_groups.items()
             for d in group if d.get("udid") == record["udid"]]
    require(len(found) == 1, "OWNED_DEVICE_MISSING_OR_AMBIGUOUS")
    runtime_id, device = found[0]
    require(runtime_id == record["runtimeId"], "DEVICE_RUNTIME_MISMATCH")
    require(device.get("isAvailable") is True, "DEVICE_UNAVAILABLE")
    require(device.get("name") == record["deviceName"], "DEVICE_NAME_MISMATCH")
    require(device.get("deviceTypeIdentifier") == record["deviceTypeIdentifier"], "DEVICE_TYPE_MISMATCH")
    type_matches = [t for t in type_rows if t.get("identifier") == record["deviceTypeIdentifier"]]
    require(len(type_matches) == 1, "TYPE_MISSING_OR_AMBIGUOUS")
    kind = type_matches[0]
    family = "iPad" if profile == "ipad-default" else "iPhone"
    require(str(kind.get("name", "")).startswith(family)
            and record["deviceTypeIdentifier"].startswith(
                "com.apple.CoreSimulator.SimDeviceType." + family + "-"), "PROFILE_MISMATCH")
    return {"udid": record["udid"], "deviceName": record["deviceName"],
            "runtimeId": record["runtimeId"], "runtimeName": record["runtimeName"],
            "runtimeVersion": record["runtimeVersion"], "isIPad": family == "iPad",
            "selectionMode": "EXACT_OWNED_EXISTING_DEVICE"}


def main(argv=None):
    parser = argparse.ArgumentParser()
    parser.add_argument("--record", required=True)
    parser.add_argument("--inventory-root", required=True)
    parser.add_argument("--output", required=True)
    parser.add_argument("--source-sha", required=True)
    parser.add_argument("--run-id", required=True)
    parser.add_argument("--profile", required=True)
    args = parser.parse_args(argv)
    output = Path(args.output)
    if output.exists() or output.is_symlink():
        raise FileExistsError(output)
    retained_dir = output.with_name(output.name + ".inputs")
    retained_dir.mkdir(mode=0o700, exist_ok=False)
    bindings = {}
    receipt = {"schema": SCHEMA, "sourceCommit": args.source_sha, "runId": args.run_id,
               "status": "REJECTED", "inputBindings": bindings,
               "nativeExecution": False, "fallbackAttempted": False,
               "deviceMutationAttempted": False}
    code = 2
    selection = None
    try:
        root = Path(args.inventory_root)
        require(root.is_dir() and not root.is_symlink(), "INVENTORY_ROOT")
        inputs = ((args.record, 65536, "ownerRecord"),
                  (root / "sim-runtimes.json", 2 * 1024**2, "runtimes"),
                  (root / "sim-devicetypes.json", 2 * 1024**2, "deviceTypes"),
                  (root / "sim-devices.json", 2 * 1024**2, "devices"))
        # Preserve the exact bounded bytes before parsing any input. The shell
        # may later remove its inventory scratch files; these originals remain.
        raw_inputs = {label: read_input(path, limit, bindings, label, retained_dir, False)
                      for path, limit, label in inputs}
        record, runtimes, types, devices = (
            unique_json(raw_inputs[label]) for _, _, label in inputs)
        selection = select_owned(record, runtimes, types, devices,
                                 args.source_sha, args.run_id, args.profile)
        receipt.update(status="SELECTED", selection=selection)
        code = 0
    except (OSError, ValueError) as error:
        receipt["error"] = str(error) if isinstance(error, ValueError) else "OWNED_SIMULATOR_INPUT_IO"
    # Even rejection retains the exact available input bytes and hashes. No old
    # receipt may be overwritten into an apparent newly selected destination.
    with output.open("x", encoding="utf-8") as stream:
        json.dump(receipt, stream, indent=2, sort_keys=True)
        stream.write("\n")
    if code == 0:
        print(json.dumps(selection))
    else:
        print(receipt["error"], file=sys.stderr)
    return code


if __name__ == "__main__":
    raise SystemExit(main())
