#!/usr/bin/env python3
"""Privately seal original RP39 diagnostics; never interpret or grant acceptance."""
import argparse
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import stat
import sys
import time
import zipfile


HERE = Path(__file__).resolve().parent


def load(name):
    spec = importlib.util.spec_from_file_location("_rp39_" + name, HERE / (name + ".py"))
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


base = load("collect_ios_xcresult_summary")
SCHEMA = "cherri-rp39-original-diagnostics/1"
FAILURE_SCHEMA = "cherri-rp39-source-failure-diagnostics/1"
WHOLE_SECONDS = 300
ENCRYPT_SECONDS = 20
LOGICAL_LIMIT = 512 * 1024 * 1024
ARCHIVE_LIMIT = 256 * 1024 * 1024
CIPHER_LIMIT = ARCHIVE_LIMIT + 65536
MEMBER_LIMIT = 10000
LIVE_FLOOR = 3 * 1024 * 1024 * 1024
METADATA_LIMIT = 4 * 1024 * 1024
OVERHEAD_BYTES = 16 * 1024 * 1024
PUBLIC = {"rp39-originals.zip.p7m", "receipt.json"}


def file_record(path, maximum=LOGICAL_LIMIT):
    before = path.lstat()
    base.require(stat.S_ISREG(before.st_mode) and before.st_size <= maximum,
                 "FILE_REJECTED")
    parents = base.ancestors(path)
    digest, size = hashlib.sha256(), 0
    with path.open("rb") as handle:
        base.require(base.file_identity(os.fstat(handle.fileno())) == base.file_identity(before),
                     "FILE_REJECTED")
        while True:
            raw = handle.read(1024 * 1024)
            if not raw:
                break
            digest.update(raw)
            size += len(raw)
            base.require(size <= maximum, "OUTPUT_LIMIT")
        base.require(base.file_identity(os.fstat(handle.fileno())) == base.file_identity(before),
                     "FILE_REJECTED")
    base.require(size == before.st_size and base.file_identity(path.lstat()) == base.file_identity(before)
                 and base.ancestors(path) == parents, "FILE_REJECTED")
    return {"bytes": size, "sha256": digest.hexdigest(), "mode": stat.S_IMODE(before.st_mode),
            "uid": before.st_uid, "gid": before.st_gid, "mtimeNs": before.st_mtime_ns}


def checked_before_source(repo, context):
    profile = load("record_workspace_presentation_source")
    directory = repo / "ios/.build/rp39-diagnostic-source"
    before_path = directory / "before.original.json"
    before_raw = base.read_regular(before_path, 65536)
    before = base.json_value(before_raw)
    base.exact_keys(before, {"schema", "context", "source", "methods", "scope", "nativeAcceptanceGranted", "phase"})
    base.exact_keys(before["source"], {"tree", "files"})
    expected_context = profile.context_from_environment(os.environ)
    base.require(expected_context["source"] == context["sourceSha"] and
                 before.get("context") == expected_context and before.get("schema") == profile.RP39_SCHEMA
                 and before.get("phase") == "before" and before.get("methods") == list(profile.RP39_METHODS)
                 and before.get("nativeAcceptanceGranted") is False
                 and before.get("scope") == "declared-rp39-diagnostic-source-only"
                 and base.HEX40.fullmatch(before["source"]["tree"]), "SOURCE_CHANGED")
    files = before["source"]["files"]
    base.require(set(files) == set(profile.RP39_SOURCE_FILES), "SOURCE_CHANGED")
    for name, recorded in files.items():
        raw = base.read_regular(repo / name, 1024 * 1024)
        info = (repo / name).lstat()
        blob = hashlib.sha1(b"blob " + str(len(raw)).encode() + b"\0" + raw).hexdigest()
        mode = "100755" if info.st_mode & 0o111 else "100644"
        base.require(recorded == {"gitBlob": blob, "sha256": base.sha(raw),
                                  "bytes": len(raw), "mode": mode}, "SOURCE_CHANGED")
    return before, before_raw, before_path


def source_receipts(repo, context):
    before, before_raw, before_path = checked_before_source(repo, context)
    after_path = before_path.with_name("after.original.json")
    after_raw = base.read_regular(after_path, 65536)
    after = base.json_value(after_raw)
    base.require(after == dict(before, phase="after", beforeSHA256=base.sha(before_raw)), "SOURCE_CHANGED")
    return {"before": base.digest_record(before_raw), "after": base.digest_record(after_raw),
            "source": before["source"], "paths": (before_path, after_path)}


def failed_source_receipts(repo, context):
    """Authenticate available originals without claiming the missing source closure."""
    profile = load("record_workspace_presentation_source")
    before, before_raw, before_path = checked_before_source(repo, context)
    after_path = before_path.with_name("after.original.json")
    base.require(not after_path.exists() and not after_path.is_symlink(), "SOURCE_CHANGED")
    failure_path = before_path.with_name("after-failure.original.json")
    raw = base.read_regular(failure_path, 256 * 1024)
    value = base.json_value(raw)
    base.exact_keys(value, {"schema", "context", "phase", "sourceVerification",
                            "formalAcceptanceGranted", "beforeSHA256", "exception"})
    base.require(value["schema"] == profile.FAILURE_SCHEMA and value["context"] == before["context"]
                 and value["phase"] == "after" and value["sourceVerification"] == "FAILED"
                 and value["formalAcceptanceGranted"] is False
                 and value["beforeSHA256"] == base.sha(before_raw), "SOURCE_CHANGED")
    error = value["exception"]
    base.exact_keys(error, {"type", "message", "messageTruncated", "argv", "timeoutSeconds",
                            "returnCode", "stdout", "stderr"})
    base.require(type(error["type"]) is str and error["type"].isidentifier()
                 and len(error["type"]) <= 64 and type(error["message"]) is str
                 and len(error["message"]) <= 4096 and type(error["messageTruncated"]) is bool,
                 "RESULT_INVALID")
    argv = error["argv"]
    base.require(argv is None or (type(argv) is list and len(argv) <= 80 and
                 all(type(item) is str and len(item.encode()) <= 4096 for item in argv)), "RESULT_INVALID")
    base.require(error["timeoutSeconds"] is None or (type(error["timeoutSeconds"]) in (int, float)
                 and 0 < error["timeoutSeconds"] <= 10), "RESULT_INVALID")
    base.require(error["returnCode"] is None or base.integer(error["returnCode"], -255, 255),
                 "RESULT_INVALID")
    for name in ("stdout", "stderr"):
        stream = error[name]
        if stream is not None:
            base.exact_keys(stream, {"encoding", "bytes", "data"})
            base.require(stream["encoding"] == "hex" and base.integer(stream["bytes"], 0, 32768)
                         and type(stream["data"]) is str and len(stream["data"]) == 2 * stream["bytes"]
                         and bytes.fromhex(stream["data"]).hex() == stream["data"], "RESULT_INVALID")
    return {"before": base.digest_record(before_raw), "after": None,
            "failure": base.digest_record(raw), "source": before["source"],
            "paths": (before_path, failure_path), "pathPhases": ("before", "after-failure")}


def member_inventory(repo, root, receipts, directories=None):
    members = {}
    directories = {} if directories is None else directories

    def add(name, path):
        base.require(name not in members and len(members) < MEMBER_LIMIT and
                     not name.startswith("/") and ".." not in Path(name).parts
                     and len(name.encode("utf-8")) <= 1024, "RESULT_INVALID")
        members[name] = {"path": path, "original": file_record(path)}
        base.require(sum(row["original"]["bytes"] for row in members.values()) <= LOGICAL_LIMIT,
                     "OUTPUT_LIMIT")

    def tree(prefix, directory):
        base.require(directory.is_dir() and not directory.is_symlink(), "RESULT_INVALID")
        for current, dirs, files in os.walk(directory, followlinks=False):
            current = Path(current)
            directory_name = prefix + "/" + current.relative_to(directory).as_posix()
            info = current.lstat()
            base.require(stat.S_ISDIR(info.st_mode) and not current.is_symlink()
                         and len(directories) < MEMBER_LIMIT, "RESULT_INVALID")
            directories[directory_name] = {"mode": stat.S_IMODE(info.st_mode), "uid": info.st_uid,
                                           "gid": info.st_gid, "mtimeNs": info.st_mtime_ns}
            for name in dirs:
                base.require(not (current / name).is_symlink(), "RESULT_INVALID")
            for name in sorted(files):
                path = current / name
                add(prefix + "/" + path.relative_to(directory).as_posix(), path)

    for phase, path in zip(receipts.get("pathPhases", ("before", "after")), receipts["paths"]):
        add("sources/" + phase + ".original.json", path)
    for name in ("xcodetest.log", "xcodetest.exitcode"):
        path = repo / "ios" / name
        if path.exists() or path.is_symlink():
            add(name, path)
    bundle = root / "tests.xcresult"
    if bundle.exists() or bundle.is_symlink():
        tree("tests.xcresult", bundle)
    for phase in ("prepare", "collect"):
        directory = root.parent / (root.name + "-git-" + phase)
        if directory.exists() or directory.is_symlink():
            tree("source-reads/" + phase, directory)
    for name, relative in (("summary/receipt.json", "public/receipt.json"),
                           ("summary/original.zip", "private/ios-xcresult-summary.zip")):
        path = root / relative
        if path.exists() or path.is_symlink():
            add(name, path)
    return members


def make_archive(archive, members, identity, *, guard=lambda: None, manifest_schema=SCHEMA):
    base.require(manifest_schema in (SCHEMA, FAILURE_SCHEMA, "cherri-native-original-diagnostics/1"), "SCHEMA_REJECTED")
    manifest = {"schema": manifest_schema, "identity": identity,
                "members": [{"name": name, **row["original"]} for name, row in sorted(members.items())]}
    manifest_raw = base.canonical(manifest)
    base.require(len(manifest_raw) <= METADATA_LIMIT, "OUTPUT_LIMIT")
    with zipfile.ZipFile(archive, "x", compression=zipfile.ZIP_DEFLATED, compresslevel=1) as output:
        for name, row in sorted(members.items()):
            guard()
            path = row["path"]
            original = row["original"]
            base.require(file_record(path) == original, "RESULT_INVALID")
            info = zipfile.ZipInfo(name)
            info.compress_type = zipfile.ZIP_DEFLATED
            info.external_attr = (stat.S_IFREG | original["mode"]) << 16
            digest, size = hashlib.sha256(), 0
            with path.open("rb") as source, output.open(info, "w") as target:
                while True:
                    raw = source.read(1024 * 1024)
                    if not raw:
                        break
                    guard()
                    target.write(raw)
                    digest.update(raw)
                    size += len(raw)
                    base.require(size <= original["bytes"] and archive.stat().st_size <= ARCHIVE_LIMIT,
                                 "OUTPUT_LIMIT")
            base.require(size == original["bytes"] and digest.hexdigest() == original["sha256"]
                         and file_record(path) == original, "RESULT_INVALID")
        output.writestr("manifest.original.json", manifest_raw)
    base.require(archive.stat().st_size <= ARCHIVE_LIMIT, "OUTPUT_LIMIT")
    with zipfile.ZipFile(archive, "r") as check:
        base.require(set(check.namelist()) == set(members) | {"manifest.original.json"}
                     and len(check.namelist()) == len(members) + 1, "RESULT_INVALID")
        for name, row in members.items():
            guard()
            digest, size = hashlib.sha256(), 0
            with check.open(name) as source:
                while True:
                    raw = source.read(1024 * 1024)
                    if not raw:
                        break
                    digest.update(raw)
                    size += len(raw)
            base.require(size == row["original"]["bytes"] and digest.hexdigest() == row["original"]["sha256"],
                         "RESULT_INVALID")
        base.require(check.read("manifest.original.json") == manifest_raw, "RESULT_INVALID")
    return file_record(archive, ARCHIVE_LIMIT), manifest


def encrypt_archive(private, stage, archive, archive_record, recipient, helper, deadline):
    base.directory_identity(stage)
    base.require(file_record(archive, ARCHIVE_LIMIT) == archive_record, "ENCRYPTION_REJECTED")
    recipient_before = base.read_regular(recipient, base.METADATA_LIMIT)
    base.require(base.sha(recipient_before) == base.RECIPIENT_SHA256, "ENCRYPTION_REJECTED")
    cipher = stage / "rp39-originals.zip.p7m"
    base.require(not cipher.exists() and not cipher.is_symlink(), "ENCRYPTION_REJECTED")
    argv = ["/usr/bin/openssl", "smime", "-encrypt", "-binary", "-aes256", "-in", str(archive),
            "-out", str(cipher), "-outform", "DER", str(recipient)]
    base.require(time.monotonic() < deadline - ENCRYPT_SECONDS, "DIAGNOSTIC_DEADLINE")
    with (private / "encryption.stdout.original").open("xb") as stdout, \
            (private / "encryption.stderr.original").open("xb") as stderr:
        observed, detail = helper.capture(argv, ENCRYPT_SECONDS, output=stdout, error_output=stderr,
            hard_deadline=min(deadline, time.monotonic() + ENCRYPT_SECONDS), stream_limit=base.METADATA_LIMIT,
            journal=lambda phase, value: base.write_json(private / ("encryption-" + phase + ".original.json"), value))
    base.write_json(private / "encryption.original.json", {"argv": argv, "observed": observed, "capture": detail})
    base.require(observed["result"] == "PASS" and detail["cleanupParentReaped"] is True
                 and detail["groupAfterCleanup"]["state"] == "ABSENT" and
                 not base.read_regular(private / "encryption.stdout.original", base.METADATA_LIMIT) and
                 not base.read_regular(private / "encryption.stderr.original", base.METADATA_LIMIT), "ENCRYPTION_REJECTED")
    base.require(base.read_regular(recipient, base.METADATA_LIMIT) == recipient_before and
                 file_record(archive, ARCHIVE_LIMIT) == archive_record, "ENCRYPTION_REJECTED")
    cipher_record = file_record(cipher, CIPHER_LIMIT)
    with cipher.open("rb") as handle:
        base.require(handle.read(1) == b"\x30", "ENCRYPTION_REJECTED")
    return cipher_record, {"observed": observed, "capture": detail}


def seal(repo, context, root, output, *, started, failure_only=False):
    profile = load("record_workspace_presentation_source")
    deadline = started + WHOLE_SECONDS
    base.require(type(failure_only) is bool, "RESULT_INVALID")
    read_receipts = failed_source_receipts if failure_only else source_receipts
    receipts = read_receipts(repo, context)
    helper = base.load_primitives(repo, None)
    output.mkdir(mode=0o700)
    private = output / "private"
    private.mkdir(mode=0o700)
    directories = {}
    members = member_inventory(repo, root, receipts, directories)
    logical = sum(item["original"]["bytes"] for item in members.values())
    space = os.statvfs(output)
    base.require(space.f_bavail * space.f_frsize >= LIVE_FLOOR + 2 * logical + OVERHEAD_BYTES,
                 "OUTPUT_LIMIT")
    native = base.native_exit(base.read_regular(repo / "ios/xcodetest.exitcode", 1024)) if "xcodetest.exitcode" in members else None
    complete = False
    if "summary/receipt.json" in members:
        summary = base.json_value(base.read_regular(root / "public/receipt.json", base.METADATA_LIMIT))
        base.validate_receipt(summary)
        base.require(summary["context"] == context and summary["nativeExitCode"] == native,
                     "RESULT_INVALID")
        if summary["disposition"] == "SEALED_COMPLETE":
            base.require("summary/original.zip" in members and "tests.xcresult/Info.plist" in members
                         and members["tests.xcresult/Info.plist"]["original"]["bytes"] > 0,
                         "RESULT_INVALID")
            base.require({key: members["summary/original.zip"]["original"][key] for key in ("bytes", "sha256")}
                         == summary["plaintextArchive"], "RESULT_INVALID")
            complete = True
    def guard():
        current = os.statvfs(output)
        base.require(current.f_bavail * current.f_frsize >= LIVE_FLOOR and time.monotonic() < deadline,
                     "DIAGNOSTIC_DEADLINE")

    identity = {"context": context, "directories": directories, "sourceReceipts": {key: receipts[key] for key in ("before", "after", "source")},
                "methods": list(profile.RP39_METHODS), "nativeExitCode": native,
                "finishedXCResultPresent": complete, "formalAcceptanceGranted": False}
    if failure_only:
        identity.update(disposition="SOURCE_AFTER_FAILED", sourceClosureVerified=False)
        identity["sourceReceipts"]["failure"] = receipts["failure"]
    archive = private / "rp39-originals.zip"
    schema = FAILURE_SCHEMA if failure_only else SCHEMA
    archive_record, manifest = make_archive(archive, members, identity, guard=guard, manifest_schema=schema)
    stage = private / "public-stage"
    stage.mkdir(mode=0o700)
    cipher_record, encryption = encrypt_archive(private, stage, archive, archive_record,
        repo / base.RECIPIENT, helper, deadline)
    guard()
    after_directories = {}
    base.require(read_receipts(repo, context) == receipts and
                 member_inventory(repo, root, receipts, after_directories) == members and
                 after_directories == directories, "SOURCE_CHANGED")
    receipt = {"schema": schema, "context": context, "methods": list(profile.RP39_METHODS),
               "finishedXCResultPresent": complete, "nativeExitCode": native,
               "formalAcceptanceGranted": False, "memberCount": len(members) + 1,
               "encryptionProcess": encryption,
               "manifest": base.digest_record(base.canonical(manifest)),
               "plaintextArchive": {key: archive_record[key] for key in ("bytes", "sha256")},
               "ciphertext": {key: cipher_record[key] for key in ("bytes", "sha256")},
               "recipientSha256": base.RECIPIENT_SHA256,
               "sourceReceipts": {key: receipts[key] for key in ("before", "after")},
               "elapsedMs": round((time.monotonic() - started) * 1000),
               "budgets": {"wholeSeconds": WHOLE_SECONDS, "encryptionSeconds": ENCRYPT_SECONDS,
                           "logicalBytes": LOGICAL_LIMIT, "archiveBytes": ARCHIVE_LIMIT,
                           "members": MEMBER_LIMIT, "liveFloorBytes": LIVE_FLOOR,
                           "overheadBytes": OVERHEAD_BYTES}}
    if failure_only:
        receipt.update(disposition="SOURCE_AFTER_FAILED", sourceClosureVerified=False)
        receipt["sourceReceipts"]["failure"] = receipts["failure"]
    base.write_json(stage / "receipt.json", receipt)
    base.require({path.name for path in stage.iterdir()} == PUBLIC and time.monotonic() < deadline,
                 "DIAGNOSTIC_DEADLINE")
    stage.rename(output / "public")
    return receipt


def main():
    parser = base.FixedParser()
    parser.add_argument("--repo", required=True)
    parser.add_argument("--retain-source-failure", action="store_true")
    args = parser.parse_args()
    base.require(sys.flags.isolated == 1 and os.environ.get("RP39_DIAGNOSTIC_ONLY") == "true",
                 "CONTEXT_REJECTED")
    context = base.ci_context()
    base.require(context["event"] == "workflow_dispatch", "CONTEXT_REJECTED")
    repo = Path(args.repo).resolve(strict=True)
    base.require(repo == Path(os.environ["GITHUB_WORKSPACE"]).resolve(strict=True), "SOURCE_CHANGED")
    root = base.owned_root(context)
    output = root.parent / (root.name + "-rp39")
    after = repo / "ios/.build/rp39-diagnostic-source/after.original.json"
    failure_only = args.retain_source_failure and not after.exists() and not after.is_symlink()
    with base.envelope(WHOLE_SECONDS):
        value = seal(repo, context, root, output, started=time.monotonic(), failure_only=failure_only)
    if failure_only:
        print("RP39_DIAGNOSTIC_RETAINED_SOURCE_FAILURE")
        return 70
    print("RP39_DIAGNOSTIC_SEALED_FINISHED" if value["finishedXCResultPresent"] else "RP39_DIAGNOSTIC_SEALED_NO_FINISHED_RESULT")
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except (Exception, base.EnvelopeExpired):
        print("RP39_DIAGNOSTIC_REJECTED", file=sys.stderr)
        raise SystemExit(70)
