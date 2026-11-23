#!/usr/bin/env python3
"""Verify authenticated xcresult diagnostics before and after external decryption."""
import argparse
import hashlib
import io
import json
import math
from pathlib import Path, PurePosixPath
import re
import stat
import sys
import types
import zipfile

EXPECTED_SCHEMA = "cherri-ios-xcresult-summary-expected/1"
METADATA_LIMIT = 32 * 1024
PROTOCOL_SOURCE = "ios/scripts/collect_ios_xcresult_summary.py"
VERIFIER_SOURCE = "ios/scripts/verify_ios_xcresult_summary.py"


class Rejected(ValueError):
    pass


def require(condition):
    if not condition:
        raise Rejected("IOS_XCRESULT_INTAKE_REJECTED")


def bootstrap_read(path, limit):
    path = Path(path).absolute()
    before = path.lstat()
    require(stat.S_ISREG(before.st_mode) and before.st_size <= limit)
    parents = []
    for parent in path.parents:
        info = parent.lstat()
        require(stat.S_ISDIR(info.st_mode) and
                not (hasattr(parent, "is_junction") and parent.is_junction()))
        parents.append((parent, info.st_dev, info.st_ino, info.st_mode))
    with path.open("rb") as handle:
        import os
        opened = os.fstat(handle.fileno())
        identity = lambda x: (x.st_dev, x.st_ino, x.st_mode, x.st_size,
                              x.st_mtime_ns, x.st_ctime_ns)
        require(identity(before) == identity(opened))
        raw = handle.read(limit + 1)
        require(identity(before) == identity(os.fstat(handle.fileno())))
    require(len(raw) == before.st_size and len(raw) <= limit and
            identity(before) == identity(path.lstat()))
    for parent, device, inode, mode in parents:
        info = parent.lstat()
        require((info.st_dev, info.st_ino, info.st_mode) == (device, inode, mode))
    return raw


def bootstrap_json(raw):
    def unique(pairs):
        result = {}
        for key, value in pairs:
            require(key not in result)
            result[key] = value
        return result

    def nonfinite(value):
        raise Rejected("IOS_XCRESULT_INTAKE_REJECTED")

    def finite(raw_number):
        value = float(raw_number)
        require(math.isfinite(value))
        return value
    return json.loads(raw, object_pairs_hook=unique,
                      parse_constant=nonfinite, parse_float=finite)


def load_protocol(expected):
    # The coordinator supplies source hashes independently of the downloaded receipt.
    root = Path(__file__).resolve(strict=True).parents[2]
    require(type(expected) is dict and expected.get("schema") == EXPECTED_SCHEMA)
    records = expected["sources"]["files"]
    source = None
    for name in (VERIFIER_SOURCE, PROTOCOL_SOURCE):
        raw = bootstrap_read(root / name, 1024 * 1024)
        record = records[name]
        require(type(record) is dict and type(record.get("bytes")) is int
                and record["bytes"] == len(raw)
                and record.get("sha256") == hashlib.sha256(raw).hexdigest())
        if name == PROTOCOL_SOURCE:
            source = raw
    module = types.ModuleType("_verified_ios_xcresult_protocol")
    module.__file__ = str(root / PROTOCOL_SOURCE)
    exec(compile(source, module.__file__, "exec"), module.__dict__)
    return module


def validate_expected(expected, protocol):
    protocol.exact_keys(expected, {"schema", "context", "workflowId", "artifact", "sources"})
    require(expected["schema"] == EXPECTED_SCHEMA and
            protocol.integer(expected["workflowId"], 1))
    protocol.validate_context(expected["context"])
    protocol.validate_sources(expected["sources"], expected["context"]["sourceSha"])
    artifact = expected["artifact"]
    protocol.exact_keys(artifact, {"id", "name", "bytes", "sha256",
                                    "runId", "attempt", "sourceSha"})
    require(protocol.integer(artifact["id"], 1) and artifact["name"] == protocol.ARTIFACT_NAME
            and protocol.integer(artifact["bytes"], 1, protocol.OUTER_ZIP_LIMIT)
            and type(artifact["sha256"]) is str and protocol.HEX64.fullmatch(artifact["sha256"])
            and artifact["runId"] == expected["context"]["runId"]
            and type(artifact["runId"]) is int
            and artifact["attempt"] == expected["context"]["attempt"]
            and type(artifact["attempt"]) is int
            and artifact["sourceSha"] == expected["context"]["sourceSha"])
    return expected


def zip_members(raw, limits, *, private=False):
    # Read only bounded members in memory; never materialize archive paths.
    result = {}
    with zipfile.ZipFile(io.BytesIO(raw)) as archive:
        infos = archive.infolist()
        require(len(infos) == len(limits) and
                [x.filename for x in infos] == list(dict.fromkeys(x.filename for x in infos))
                and {x.filename for x in infos} == set(limits))
        for info in infos:
            mode = info.external_attr >> 16
            require(info.orig_filename == info.filename and not info.is_dir() and not (info.flag_bits & 1) and
                    info.compress_type in {zipfile.ZIP_STORED, zipfile.ZIP_DEFLATED}
                    and info.file_size <= limits[info.filename]
                    and not info.filename.startswith("/") and
                    PurePosixPath(info.filename).parts == (info.filename,))
            if private:
                require(info.create_system == 3 and mode == stat.S_IFREG | 0o600
                        and info.compress_type == zipfile.ZIP_STORED)
            else:
                require(stat.S_IFMT(mode) in {0, stat.S_IFREG})
            with archive.open(info) as member:
                value = member.read(limits[info.filename] + 1)
                require(len(value) == info.file_size and len(value) <= limits[info.filename]
                        and member.read(1) == b"")
            result[info.filename] = value
    return result


def public_intake(artifact_path, expected, protocol):
    validate_expected(expected, protocol)
    original = protocol.read_regular(artifact_path, protocol.OUTER_ZIP_LIMIT)
    require(protocol.digest_record(original) ==
            {key: expected["artifact"][key] for key in ("bytes", "sha256")})
    # Determine only the fixed one/two-file envelope; the receipt cannot relax it.
    with zipfile.ZipFile(io.BytesIO(original)) as archive:
        names = [row.filename for row in archive.infolist()]
    require(set(names) in ({"receipt.json"}, protocol.PUBLIC_FILES))
    limits = {"receipt.json": protocol.METADATA_LIMIT}
    if "ios-xcresult-summary.zip.p7m" in names:
        limits["ios-xcresult-summary.zip.p7m"] = protocol.CIPHER_LIMIT
    members = zip_members(original, limits)
    receipt = protocol.json_value(members["receipt.json"])
    protocol.validate_receipt(receipt)
    require(receipt["context"] == expected["context"] and receipt["sources"] == expected["sources"])
    if receipt["disposition"] == "REJECTED":
        require(set(members) == {"receipt.json"})
    else:
        require(set(members) == protocol.PUBLIC_FILES and
                protocol.digest_record(members["ios-xcresult-summary.zip.p7m"]) == receipt["ciphertext"]
                and members["ios-xcresult-summary.zip.p7m"].startswith(b"\x30"))
    return receipt


def validate_group(value, *, signal_record=False):
    require(type(value) is dict)
    if signal_record:
        require(set(value) == {"state", "signalSent", "errno"} and
                value["state"] in {"SENT", "ABSENT", "PERMISSION_DENIED", "SIGNAL_FAILED"}
                and type(value["signalSent"]) is bool)
    else:
        require(set(value) == {"state", "present", "errno"} and
                value["state"] in {"PRESENT", "ABSENT", "PERMISSION_DENIED", "QUERY_FAILED"}
                and (value["present"] is None or type(value["present"]) is bool))
    require(value["errno"] is None or type(value["errno"]) is int)
    if signal_record:
        require(value["signalSent"] == (value["state"] == "SENT"))
    else:
        require(value["present"] is (True if value["state"] == "PRESENT" else
                                    False if value["state"] == "ABSENT" else None))


def validate_capture(command, receipt, raw, protocol):
    protocol.exact_keys(command, {"schema", "argv", "allowanceMs",
                                  "result", "exitCode", "elapsedMs", "capture"})
    require(command["schema"] == protocol.SCHEMA and command["allowanceMs"] == 20000
            and type(command["allowanceMs"]) is int and
            command["result"] == receipt["captureResult"] and
            protocol.integer(command["elapsedMs"], 0, 60000) and
            (command["exitCode"] is None or protocol.integer(command["exitCode"], -255, 255)))
    capture = command["capture"]
    protocol.exact_keys(capture, {
        "startedAt", "finishedAt", "stdin", "pid", "returnCodeObserved",
        "cleanupGroupSignalSent", "cleanupParentReaped", "groupPresentAfterCleanup",
        "groupBeforeCleanup", "groupAfterCleanup", "cleanupGroupSignal", "stdout", "stderr"})
    require(capture["stdin"] == "DEVNULL" and
            (capture["pid"] is None or protocol.integer(capture["pid"], 1)) and
            (capture["returnCodeObserved"] is None or
             protocol.integer(capture["returnCodeObserved"], -255, 255)) and
            type(capture["cleanupGroupSignalSent"]) is bool and
            type(capture["cleanupParentReaped"]) is bool and
            (capture["groupPresentAfterCleanup"] is None or
             type(capture["groupPresentAfterCleanup"]) is bool))
    for field in ("startedAt", "finishedAt"):
        require(type(capture[field]) is str and
                re.fullmatch(r"[0-9T:+.Z-]{20,40}", capture[field]))
    for field in ("groupBeforeCleanup", "groupAfterCleanup"):
        if capture[field] is not None:
            validate_group(capture[field])
    if capture["cleanupGroupSignal"] is not None:
        validate_group(capture["cleanupGroupSignal"], signal_record=True)
    for name in ("stdout", "stderr"):
        row = capture[name]
        protocol.exact_keys(row, {"bytes", "sha256", "overflow", "eof"})
        require(type(row["overflow"]) is bool and row["overflow"] is False
                and type(row["eof"]) is bool)
        protocol.validate_digest({k: row[k] for k in ("bytes", "sha256")}, protocol.STREAM_LIMIT)
        expected = protocol.digest_record(raw["xcresult-summary." + name])
        require({k: row[k] for k in ("bytes", "sha256")} == expected and
                receipt["streams"][name] == {**expected, "complete": row["eof"]})
    require(command["result"] != "OUTPUT_OVERFLOW")
    if capture["cleanupGroupSignal"] is not None:
        require(capture["cleanupGroupSignalSent"] == capture["cleanupGroupSignal"]["signalSent"])
    if capture["groupAfterCleanup"] is not None:
        require(capture["groupPresentAfterCleanup"] is capture["groupAfterCleanup"]["present"])
    if command["result"] == "PASS":
        require(command["exitCode"] == 0 and command["elapsedMs"] <= 20000 and
                capture["returnCodeObserved"] == 0 and
                capture["cleanupParentReaped"] is True and
                capture["groupPresentAfterCleanup"] is False and
                capture["cleanupGroupSignal"] is not None and
                capture["cleanupGroupSignal"]["state"] in {"SENT", "ABSENT"} and
                all(capture[n]["eof"] for n in ("stdout", "stderr")))


def validate_private_identity(identity, receipt, expected, protocol):
    protocol.exact_keys(identity, {"schema", "reservation", "reservationSha256", "sourcesAfter"})
    require(identity["schema"] == protocol.SCHEMA and
            identity["reservationSha256"] == receipt["reservationSha256"] and
            identity["sourcesAfter"] == expected["sources"])
    marker = identity["reservation"]
    protocol.exact_keys(marker, {"schema", "context", "sources", "root", "rootIdentity",
                                 "resultPath", "resultAbsent", "exitPath", "exitAbsent"})
    require(marker["schema"] == protocol.SCHEMA and marker["context"] == expected["context"]
            and marker["sources"] == expected["sources"] and
            protocol.sha(protocol.canonical(marker)) == receipt["reservationSha256"]
            and marker["resultAbsent"] is True and marker["exitAbsent"] is True)
    for key in ("root", "resultPath", "exitPath"):
        require(type(marker[key]) is str and 0 < len(marker[key].encode()) <= 4096
                and marker[key].startswith("/") and
                ".." not in PurePosixPath(marker[key]).parts)
    require(PurePosixPath(marker["root"]).name == protocol.root_name(expected["context"])
            and marker["resultPath"] == marker["root"] + "/tests.xcresult"
            and marker["exitPath"].endswith("/ios/xcodetest.exitcode"))
    protocol.exact_keys(marker["rootIdentity"], {"device", "inode", "owner", "mode"})
    require(all(protocol.integer(v) for v in marker["rootIdentity"].values())
            and marker["rootIdentity"]["inode"] > 0 and marker["rootIdentity"]["mode"] == 0o700)
    return marker


def private_intake(artifact_path, plaintext_path, expected, protocol):
    # Always repeat caller-owned public provenance checks before reading plaintext.
    receipt = public_intake(artifact_path, expected, protocol)
    require(receipt["disposition"] != "REJECTED")
    encoded = protocol.read_regular(plaintext_path, protocol.ARCHIVE_LIMIT)
    require(protocol.digest_record(encoded) == receipt["plaintextArchive"])
    limits = {name: protocol.STREAM_LIMIT if name.endswith((".stdout", ".stderr"))
              else protocol.METADATA_LIMIT for name in protocol.MEMBERS}
    members = zip_members(encoded, limits, private=True)
    for row in receipt["members"]:
        require(protocol.digest_record(members[row["name"]]) ==
                {key: row[key] for key in ("bytes", "sha256")})
    manifest = protocol.json_value(members["manifest.json"])
    protocol.exact_keys(manifest, {"schema", "members"})
    require(manifest["schema"] == protocol.SCHEMA and
            manifest["members"] == receipt["members"][:-1] and
            receipt["members"][-1]["name"] == "manifest.json" and
            receipt["members"][-1]["complete"] is True)
    identity = protocol.json_value(members["identity.json"])
    marker = validate_private_identity(identity, receipt, expected, protocol)
    command = protocol.json_value(members["command.json"])
    require(command.get("argv") == protocol.summary_command(marker["resultPath"]))
    validate_capture(command, receipt, members, protocol)
    require(protocol.native_exit(members["xcodebuild.exitcode"]) == receipt["nativeExitCode"])
    parse = "NOT_CHECKED"
    if command["result"] == "PASS" and all(x["complete"] for x in receipt["streams"].values()):
        try:
            require(type(protocol.json_value(members["xcresult-summary.stdout"])) is dict)
            parse = "JSON_OBJECT"
        except (ValueError, TypeError):
            parse = "INVALID_JSON"
    require(parse == receipt["jsonStatus"])
    return {
        "schema": protocol.SCHEMA, "status": "PRIVATE_DIAGNOSTIC_VERIFIED",
        "completeSummaryJsonObject": receipt["disposition"] == "SEALED_COMPLETE",
        "nativeExitCode": receipt["nativeExitCode"], "memberCount": len(members),
        "plaintextSha256": receipt["plaintextArchive"]["sha256"],
        "artifactSha256": expected["artifact"]["sha256"],
        "issueSemantics": "UNOBSERVED", "formalAcceptanceGranted": False,
    }


class FixedParser(argparse.ArgumentParser):
    def error(self, message):
        raise Rejected("IOS_XCRESULT_INTAKE_REJECTED")


def main(argv=None):
    parser = FixedParser(prog="verify_ios_xcresult_summary.py", description=__doc__)
    parser.add_argument("--expected", required=True)
    parser.add_argument("--artifact", required=True)
    parser.add_argument("--plaintext")
    args = parser.parse_args(argv)
    require(sys.flags.isolated == 1)
    expected = bootstrap_json(bootstrap_read(args.expected, METADATA_LIMIT))
    protocol = load_protocol(expected)
    if args.plaintext:
        result = private_intake(args.artifact, args.plaintext, expected, protocol)
    else:
        receipt = public_intake(args.artifact, expected, protocol)
        result = {
            "schema": protocol.SCHEMA,
            "status": "UNSEALED_REJECTION_VERIFIED" if receipt["disposition"] == "REJECTED"
                      else "PUBLIC_CIPHERTEXT_VERIFIED",
            "privateIntakePerformed": False,
            "artifactSha256": expected["artifact"]["sha256"],
            "formalAcceptanceGranted": False,
        }
    print(json.dumps(result, sort_keys=True, separators=(",", ":")))
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except Exception:
        print("IOS_XCRESULT_INTAKE_REJECTED", file=sys.stderr)
        raise SystemExit(70)
