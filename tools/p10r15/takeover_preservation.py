#!/usr/bin/env python3
"""Preserve two real source worktrees while creating isolated recovery refs.

Original patches and unpublished file backups remain outside Git. Verification
is read-only and requires a separately selected, hash-pinned evidence inventory.
Fixture controls are not production takeover evidence.
"""
from __future__ import annotations
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import stat
import subprocess
import sys
import time

SCHEMA = "cherri-takeover-preservation/1"
SPEC_SCHEMA = "cherri-takeover-spec/1"
EXPECTED_SCHEMA = "cherri-takeover-expected/1"
SOURCE = "tools/p10r15/takeover_preservation.py"
ROLES = {"compiler": "davidpovarsky/cherri", "documentation": "davidpovarsky/cherrilang.org"}
ZERO = "0" * 40
MAX_FILE = 16 * 1024 * 1024
MAX_BACKUPS = 64 * 1024 * 1024
SCOPE = "Actual dual-repository takeover, private unpublished backups and create-only local recovery refs; no other acceptance or release credit"

class Rejected(ValueError):
    pass

def need(value, code):
    if not value:
        raise Rejected(code)

def digest(data):
    return hashlib.sha256(data).hexdigest()

def canonical(value):
    return (json.dumps(value, ensure_ascii=False, sort_keys=True, indent=2) + "\n").encode()

def read_json(path):
    def unique(items):
        result = {}
        for key, value in items:
            need(key not in result, "DUPLICATE_JSON_KEY")
            result[key] = value
        return result
    return json.loads(read_regular(path), object_pairs_hook=unique, parse_constant=lambda _: need(False, "NONFINITE_JSON"))

def read_regular(path, limit=MAX_FILE):
    path = Path(path)
    st = path.lstat()
    need(stat.S_ISREG(st.st_mode) and not path.is_symlink(), "NONREGULAR_OR_REDIRECTED_FILE")
    need(st.st_size <= limit, "FILE_SIZE_LIMIT")
    with path.open("rb") as stream:
        fs = os.fstat(stream.fileno())
        need((fs.st_dev, fs.st_ino, fs.st_size) == (st.st_dev, st.st_ino, st.st_size), "FILE_IDENTITY_CHANGED")
        data = stream.read(limit + 1)
    after = path.lstat()
    need(len(data) == st.st_size and (after.st_dev, after.st_ino, after.st_size, after.st_mtime_ns) ==
         (st.st_dev, st.st_ino, st.st_size, st.st_mtime_ns), "FILE_CHANGED")
    return data

def absolute(path):
    p = Path(path)
    need(p.is_absolute() and str(p) == os.path.normpath(str(p)) and p.resolve() == p, "REDIRECTED_PATH")
    return p

def relative(name):
    p = Path(name)
    need(name and not p.is_absolute() and p.as_posix() == name and ".." not in p.parts and
         ".git" not in p.parts and "\\" not in name, "UNSAFE_RELATIVE_PATH")
    return p

def pin(path):
    data = read_regular(path)
    return {"sha256": digest(data), "bytes": len(data)}

def ref_rows(data):
    result = {}
    for line in data.decode("utf-8").splitlines():
        oid, name = line.split(" ", 1)
        need(re.fullmatch(r"[0-9a-f]{40}", oid) and name.startswith("refs/") and name not in result,
             "INVALID_REF_INVENTORY")
        result[name] = oid
    return result

def git_command(root, args):
    return ["git", "--no-replace-objects", "-c", "core.hooksPath=/dev/null", "-c", "user.name=David Povarsky",
            "-c", "user.email=davidpovarsky@users.noreply.github.com", "-C", str(root), *args]

def invoke(root, args):
    # Local-only repository operations must not inherit another worktree/index,
    # namespace, object store, replacement, or injected Git configuration.
    env = {k: v for k, v in os.environ.items() if not k.startswith("GIT_")}
    env.update({"GIT_OPTIONAL_LOCKS": "0", "GIT_CONFIG_NOSYSTEM": "1",
                "GIT_CONFIG_SYSTEM": os.devnull, "GIT_CONFIG_GLOBAL": os.devnull})
    command = git_command(root, args)
    started = time.monotonic()
    result = subprocess.run(command, env=env, capture_output=True, timeout=30)
    need(len(result.stdout) <= MAX_FILE and len(result.stderr) <= 1024 * 1024, "GIT_OUTPUT_LIMIT")
    return result, {"command": command, "exitCode": result.returncode,
                    "elapsedSeconds": round(time.monotonic() - started, 6),
                    "stdoutSHA256": digest(result.stdout), "stderrSHA256": digest(result.stderr)}

class Recorder:
    def __init__(self, output):
        self.output, self.commands = output, []
    def __call__(self, role, phase, name, root, args):
        result, process = invoke(root, args)
        identity = role + "-" + phase + "-" + name
        need(identity not in [row["id"] for row in self.commands], "DUPLICATE_COMMAND")
        process["id"] = identity
        for suffix, data in ((".stdout", result.stdout), (".stderr", result.stderr), (".process.json", canonical(process))):
            path = self.output / "commands" / (identity + suffix)
            path.parent.mkdir(exist_ok=True)
            path.write_bytes(data)
        self.commands.append(process)
        need(result.returncode == 0, "GIT_COMMAND_FAILED:" + identity)
        return result.stdout

def live_reader(role, phase, name, root, args):
    result, _ = invoke(root, args)
    need(result.returncode == 0, "CURRENT_GIT_QUERY_FAILED")
    return result.stdout

def snapshot(repo, phase, capture):
    root, role = absolute(repo["root"]), repo["role"]
    def query(name, *args):
        return capture(role, phase, name, root, list(args))
    identity = query("identity", "rev-parse", "--path-format=absolute", "--show-toplevel", "--absolute-git-dir", "--git-common-dir")
    locations = identity.decode().splitlines()
    need(len(locations) == 3 and absolute(locations[0]) == root, "ACTUAL_REPOSITORY_IDENTITY")
    gitdir, common = absolute(locations[1]), absolute(locations[2])
    need(gitdir.is_dir() and common.is_dir(), "ACTUAL_GIT_DIRECTORY")
    for forbidden in ("objects/info/alternates", "info/grafts"):
        need(not (common / forbidden).exists() and not (common / forbidden).is_symlink(), "REDIRECTED_OBJECT_HISTORY")
    head = query("head", "rev-parse", "HEAD").decode().strip()
    tree = query("tree", "rev-parse", "HEAD^{tree}").decode().strip()
    branch = query("branch", "symbolic-ref", "HEAD").decode().strip()
    origin = query("origin", "remote", "get-url", "origin").decode().strip()
    need(origin in ("https://github.com/" + repo["repository"],
                    "https://github.com/" + repo["repository"] + ".git",
                    "git@github.com:" + repo["repository"] + ".git"), "DIFFERENT_ORIGIN")
    refs = ref_rows(query("refs", "for-each-ref", "--format=%(objectname) %(refname)"))
    need(not any(name.startswith("refs/replace/") for name in refs), "REPLACEMENT_REFS")
    status = query("status", "status", "--porcelain=v1", "-z", "--untracked-files=all")
    tracked = query("tracked", "ls-files", "-z").decode().split("\0")
    untracked = query("untracked", "ls-files", "--others", "--exclude-standard", "-z").decode().split("\0")
    workdiff = query("workdiff", "diff", "--no-ext-diff", "--no-textconv", "--binary", "--no-renames", "--")
    indexdiff = query("indexdiff", "diff", "--cached", "--no-ext-diff", "--no-textconv", "--binary", "--no-renames", "--")
    changed = []
    for name, args in (
        ("changed-work", ["diff", "--no-ext-diff", "--no-textconv", "--no-renames", "--name-only", "-z", "--"]),
        ("changed-index", ["diff", "--cached", "--no-ext-diff", "--no-textconv", "--no-renames", "--name-only", "-z", "--"]),
    ):
        changed += query(name, *args).decode().split("\0")
    index_flags = query("index-flags", "ls-files", "-v", "-z")
    need(all(item.startswith(b"H ") for item in index_flags.split(b"\0") if item), "HIDDEN_OR_UNMERGED_INDEX_ENTRY")
    tracked, untracked = sorted(set(filter(None, tracked))), sorted(set(filter(None, untracked)))
    need(sorted(item[2:].decode() for item in index_flags.split(b"\0") if item) == tracked, "INDEX_FLAG_INVENTORY")
    need(not (set(tracked) & set(untracked)) and len(tracked) + len(untracked) <= 20000, "SOURCE_INVENTORY")
    changed = sorted(set(filter(None, changed)))
    need(len(set(tracked + untracked + changed)) <= 20000, "SOURCE_INVENTORY")
    files = {}
    for name in sorted(set(tracked + untracked + changed)):
        path = root / relative(name)
        if not path.exists() and not path.is_symlink():
            need(name in tracked or name in changed, "MISSING_UNTRACKED_FILE")
            files[name] = {"missing": True}
            continue
        need(not path.is_symlink(), "NONREGULAR_OR_REDIRECTED_FILE")
        absolute(path)
        data = read_regular(path)
        st = path.stat()
        files[name] = {"sha256": digest(data), "bytes": len(data), "mode": stat.S_IMODE(st.st_mode)}
    need(head == repo["head"] and branch == repo["branch"] and refs.get(branch) == head, "SOURCE_IDENTITY")
    return {"head": head, "tree": tree, "branch": branch, "origin": origin, "refs": refs,
            "repositoryIdentitySHA256": digest(identity), "indexFlagsSHA256": digest(index_flags),
            "statusSHA256": digest(status), "workDiffSHA256": digest(workdiff),
            "indexDiffSHA256": digest(indexdiff), "tracked": tracked, "untracked": untracked,
            "changed": sorted(set(filter(None, changed))), "files": files}

def load_spec(path, expected_sha):
    need(re.fullmatch(r"[0-9a-f]{64}", expected_sha) is not None, "SPEC_PIN")
    raw = read_regular(path)
    need(digest(raw) == expected_sha, "SPEC_HASH")
    value = read_json(path)
    need(set(value) == {"schema", "implementationSHA", "repositories"} and value["schema"] == SPEC_SCHEMA and
         re.fullmatch(r"[0-9a-f]{40}", value["implementationSHA"]) and
         isinstance(value["repositories"], list) and len(value["repositories"]) == 2, "SPEC_SCHEMA")
    need({r.get("role") for r in value["repositories"]} == set(ROLES), "REPOSITORY_ROLES")
    roots = []
    for repo in value["repositories"]:
        need(set(repo) == {"role", "repository", "root", "head", "branch", "recoveryRef"}, "REPOSITORY_FIELDS")
        need(repo["repository"] == ROLES[repo["role"]] and re.fullmatch(r"[0-9a-f]{40}", repo["head"]) and
             re.fullmatch(r"refs/heads/[A-Za-z0-9_./-]+", repo["branch"]) and
             re.fullmatch(r"refs/heads/agent/[a-z0-9][a-z0-9/_-]{1,140}", repo["recoveryRef"]) and
             repo["branch"] != repo["recoveryRef"], "REPOSITORY_IDENTITY")
        roots.append(absolute(repo["root"]))
    need(roots[0] != roots[1] and roots[0] not in roots[1].parents and roots[1] not in roots[0].parents,
         "OVERLAPPING_REPOSITORIES")
    compiler = next(r for r in value["repositories"] if r["role"] == "compiler")
    need(compiler["head"] == value["implementationSHA"], "DIFFERENT_IMPLEMENTATION")
    result, _ = invoke(compiler["root"], ["show", value["implementationSHA"] + ":" + SOURCE])
    need(result.returncode == 0 and result.stdout == Path(__file__).read_bytes(), "PRODUCER_SOURCE_MISMATCH")
    return value

def produce(spec_path, spec_sha, output):
    spec = load_spec(spec_path, spec_sha)
    output = absolute(output)
    need(not output.exists() and all(Path(r["root"]) not in output.parents and output not in Path(r["root"]).parents
                                    for r in spec["repositories"]), "OUTPUT_OVERLAP_OR_EXISTS")
    output.mkdir(parents=True, mode=0o700)
    (output / "spec.original.json").write_bytes(read_regular(spec_path))
    recorder = Recorder(output)
    receipt = {"schema": SCHEMA, "status": "FAILED", "implementationSHA": spec["implementationSHA"],
               "specSHA256": spec_sha, "producerSHA256": digest(Path(__file__).read_bytes()),
               "repositories": [], "commands": [], "closureComplete": False,
               "scope": SCOPE}
    started = time.monotonic()
    try:
        backup_bytes = 0
        for repo in spec["repositories"]:
            before = snapshot(repo, "before", recorder)
            need(repo["recoveryRef"] not in before["refs"], "RECOVERY_REF_ALREADY_EXISTS")
            backups = {}
            for name in sorted(set(before["changed"] + before["untracked"])):
                data = before["files"][name]
                if data.get("missing"):
                    continue
                original = read_regular(Path(repo["root"]) / name)
                need(digest(original) == data["sha256"], "UNPUBLISHED_FILE_CHANGED")
                backup_bytes += len(original)
                need(backup_bytes <= MAX_BACKUPS, "BACKUP_TOTAL_LIMIT")
                location = repo["role"] + "/backups/" + digest(name.encode()) + ".bin"
                target = output / location
                target.parent.mkdir(parents=True, exist_ok=True)
                target.write_bytes(original)
                backups[name] = {"file": location, **pin(target)}
            receipt["repositories"].append({"role": repo["role"], "before": before, "backups": backups})
        # All preflights/backups finish before either create-only ref operation.
        for repo in spec["repositories"]:
            recorder(repo["role"], "create", "ref", repo["root"],
                     ["update-ref", repo["recoveryRef"], repo["head"], ZERO])
        for repo, row in zip(spec["repositories"], receipt["repositories"]):
            after = snapshot(repo, "after", recorder)
            wanted = copy_snapshot(row["before"])
            wanted["refs"][repo["recoveryRef"]] = repo["head"]
            need(after == wanted, "SOURCE_OR_UNRELATED_REF_CHANGED")
            row["after"] = after
        receipt["status"] = "PASSED"
    except Exception as error:
        receipt["failure"] = str(error) if isinstance(error, Rejected) else type(error).__name__
        raise
    finally:
        receipt["commands"] = recorder.commands
        receipt["elapsedSeconds"] = round(time.monotonic() - started, 6)
        (output / "takeover.original.json").write_bytes(canonical(receipt))
    return receipt

def copy_snapshot(value):
    return json.loads(json.dumps(value))

def selection(output, target):
    """Select completed originals separately; this never executes the producer."""
    output, target = absolute(output), absolute(target)
    need(not target.exists() and target.parent != output and output not in target.parents, "SELECTION_LOCATION")
    report = read_json(output / "takeover.original.json")
    need(report["status"] == "PASSED", "UNSUCCESSFUL_PRODUCER")
    files = {}
    for path in sorted(output.rglob("*")):
        if path.is_file():
            files[path.relative_to(output).as_posix()] = {"path": str(path), **pin(path)}
        else:
            need(path.is_dir() and not path.is_symlink(), "REDIRECTED_ORIGINAL")
    expected = {"schema": EXPECTED_SCHEMA, "implementationSHA": report["implementationSHA"], "files": files}
    target.parent.mkdir(parents=True, exist_ok=True)
    target.write_bytes(canonical(expected))
    return expected

def verify(expected_path, expected_sha, implementation):
    need(re.fullmatch(r"[0-9a-f]{64}", expected_sha) is not None and
         digest(read_regular(expected_path)) == expected_sha, "EXPECTED_CONTEXT_HASH")
    expected = read_json(expected_path)
    need(set(expected) == {"schema", "implementationSHA", "files"} and expected["schema"] == EXPECTED_SCHEMA and
         expected["implementationSHA"] == implementation and isinstance(expected["files"], dict), "EXPECTED_CONTEXT_SCOPE")
    files = expected["files"]
    need("spec.original.json" in files and "takeover.original.json" in files and len(files) <= 2000, "ORIGINAL_INVENTORY")
    total = 0
    for name, row in files.items():
        relative(name)
        need(set(row) == {"path", "sha256", "bytes"} and isinstance(row["bytes"], int) and
             not isinstance(row["bytes"], bool) and row["bytes"] >= 0 and
             re.fullmatch(r"[0-9a-f]{64}", row["sha256"]), "ORIGINAL_PIN")
        absolute(row["path"])
        need(pin(row["path"]) == {k: row[k] for k in ("sha256", "bytes")}, "ORIGINAL_CHANGED")
        total += row["bytes"]
    need(total <= MAX_BACKUPS + 16 * 1024 * 1024, "ORIGINAL_TOTAL_LIMIT")
    spec_pin, report_pin = files["spec.original.json"], files["takeover.original.json"]
    spec = load_spec(spec_pin["path"], spec_pin["sha256"])
    report = read_json(report_pin["path"])
    need(set(report) == {"schema", "status", "implementationSHA", "specSHA256", "producerSHA256", "repositories", "commands", "closureComplete", "scope", "elapsedSeconds"}, "OBSERVATION_FIELDS")
    need(report.get("schema") == SCHEMA and report.get("status") == "PASSED" and
         report.get("implementationSHA") == implementation and report.get("specSHA256") == spec_pin["sha256"] and
         report.get("producerSHA256") == digest(Path(__file__).read_bytes()) and
         report.get("closureComplete") is False and "failure" not in report and
         type(report.get("elapsedSeconds")) in (int, float) and 0 < report["elapsedSeconds"] <= 300, "PRODUCER_OBSERVATION")
    need(report["scope"] == SCOPE, "PRODUCER_SCOPE")
    need(len(report["repositories"]) == 2 and len(report["commands"]) == 58, "COMPLETE_EXECUTION_INVENTORY")
    used = {"spec.original.json", "takeover.original.json"}
    for repo, row in zip(spec["repositories"], report["repositories"]):
        need(row["role"] == repo["role"] and row["before"]["head"] == repo["head"] and
             row["before"]["branch"] == repo["branch"] and repo["recoveryRef"] not in row["before"]["refs"], "STARTING_IDENTITIES")
        wanted = copy_snapshot(row["before"])
        wanted["refs"][repo["recoveryRef"]] = repo["head"]
        need(row["after"] == wanted, "PROTECTED_SOURCE_OR_REFS")
        need(snapshot(repo, "verify", live_reader) == row["after"], "CURRENT_SOURCE_OR_REFS_CHANGED")
        required_backups = {name for name in set(row["before"]["changed"] + row["before"]["untracked"])
                            if not row["before"]["files"][name].get("missing")}
        need(set(row["backups"]) == required_backups, "MISSING_OR_EXTRA_UNPUBLISHED_BACKUP")
        for name, backup in row["backups"].items():
            original = row["before"]["files"][name]
            need(backup["file"] in files and backup["sha256"] == original["sha256"] and
                 backup["bytes"] == original["bytes"] and
                 files[backup["file"]]["sha256"] == backup["sha256"] and
                 files[backup["file"]]["bytes"] == backup["bytes"], "BACKUP_IDENTITY")
            used.add(backup["file"])
    expected_commands = original_commands(spec)
    need([c["id"] for c in report["commands"]] == list(expected_commands), "COMPLETE_ORIGINAL_COMMAND_ORDER")
    identifiers = set()
    for command in report["commands"]:
        identity = command["id"]
        need(identity not in identifiers and re.fullmatch(r"(compiler|documentation)-(before|after|create)-[a-z-]+", identity),
             "COMMAND_IDENTITY")
        identifiers.add(identity)
        need(command["command"] == expected_commands[identity], "EXACT_ORIGINAL_COMMAND")
        need(set(command) == {"command", "exitCode", "elapsedSeconds", "stdoutSHA256", "stderrSHA256", "id"}, "PROCESS_FIELDS")
        need(type(command["exitCode"]) is int and command["exitCode"] == 0 and type(command["elapsedSeconds"]) in (int, float) and
             0 <= command["elapsedSeconds"] <= 30.5, "COMMAND_OUTCOME")
        for suffix, field in ((".stdout", "stdoutSHA256"), (".stderr", "stderrSHA256")):
            name = "commands/" + identity + suffix
            need(name in files and files[name]["sha256"] == command[field], "PROCESS_STREAM_BINDING")
            used.add(name)
        name = "commands/" + identity + ".process.json"
        need(name in files and read_json(files[name]["path"]) == command, "PROCESS_RECEIPT_BINDING")
        used.add(name)
    for repo in spec["repositories"]:
        found = [c for c in report["commands"] if c["id"] == repo["role"] + "-create-ref"]
        need(len(found) == 1 and found[0]["command"] == git_command(repo["root"],
             ["update-ref", repo["recoveryRef"], repo["head"], ZERO]), "CREATE_ONLY_BRANCH_COMMAND")
    for repo, row in zip(spec["repositories"], report["repositories"]):
        for phase in ("before", "after"):
            verify_snapshot_streams(repo, phase, row[phase], files)
    need(used == set(files), "UNSELECTED_OR_EXTRA_ORIGINAL")
    return {"schema": SCHEMA, "status": "PASSED", "implementationSHA": implementation,
            "expectedSHA256": expected_sha, "receiptSHA256": report_pin["sha256"],
            "repositories": 2, "commands": len(report["commands"]),
            "unpublishedBackups": sum(len(r["backups"]) for r in report["repositories"]),
            "scope": report["scope"], "closureComplete": False}

READ_QUERIES = {
    "identity": ["rev-parse", "--path-format=absolute", "--show-toplevel", "--absolute-git-dir", "--git-common-dir"],
    "head": ["rev-parse", "HEAD"],
    "tree": ["rev-parse", "HEAD^{tree}"],
    "branch": ["symbolic-ref", "HEAD"],
    "origin": ["remote", "get-url", "origin"],
    "refs": ["for-each-ref", "--format=%(objectname) %(refname)"],
    "status": ["status", "--porcelain=v1", "-z", "--untracked-files=all"],
    "tracked": ["ls-files", "-z"],
    "untracked": ["ls-files", "--others", "--exclude-standard", "-z"],
    "workdiff": ["diff", "--no-ext-diff", "--no-textconv", "--binary", "--no-renames", "--"],
    "indexdiff": ["diff", "--cached", "--no-ext-diff", "--no-textconv", "--binary", "--no-renames", "--"],
    "changed-work": ["diff", "--no-ext-diff", "--no-textconv", "--no-renames", "--name-only", "-z", "--"],
    "changed-index": ["diff", "--cached", "--no-ext-diff", "--no-textconv", "--no-renames", "--name-only", "-z", "--"],
    "index-flags": ["ls-files", "-v", "-z"],
}

def original_commands(spec):
    expected = {}
    for phase in ("before", "create", "after"):
        for repo in spec["repositories"]:
            commands = {"ref": ["update-ref", repo["recoveryRef"], repo["head"], ZERO]} if phase == "create" else READ_QUERIES
            for name, arguments in commands.items():
                expected[repo["role"] + "-" + phase + "-" + name] = git_command(repo["root"], arguments)
    return expected

def verify_snapshot_streams(repo, phase, snapshot, files):
    def raw(name):
        return read_regular(files["commands/" + repo["role"] + "-" + phase + "-" + name + ".stdout"]["path"])
    need(digest(raw("identity")) == snapshot["repositoryIdentitySHA256"] and
         digest(raw("index-flags")) == snapshot["indexFlagsSHA256"], "SNAPSHOT_REPOSITORY_AND_INDEX_IDENTITY")
    for key in ("head", "tree", "branch", "origin"):
        need(raw(key).decode().strip() == snapshot[key], "SNAPSHOT_COMMAND_IDENTITY")
    need(ref_rows(raw("refs")) == snapshot["refs"], "SNAPSHOT_REF_STREAM")
    for query, field in (("status", "statusSHA256"), ("workdiff", "workDiffSHA256"), ("indexdiff", "indexDiffSHA256")):
        need(digest(raw(query)) == snapshot[field], "SNAPSHOT_PATCH_STREAM")
    for name in ("tracked", "untracked"):
        need(sorted(set(filter(None, raw(name).decode().split("\0")))) == snapshot[name], "SNAPSHOT_FILE_INVENTORY")
    changed = sorted(set(filter(None, (raw("changed-work") + raw("changed-index")).decode().split("\0"))))
    need(changed == snapshot["changed"], "SNAPSHOT_CHANGED_INVENTORY")

def main():
    parser = argparse.ArgumentParser(description=__doc__)
    sub = parser.add_subparsers(dest="mode", required=True)
    make = sub.add_parser("produce")
    make.add_argument("--spec", required=True); make.add_argument("--spec-sha256", required=True); make.add_argument("--output", required=True)
    select = sub.add_parser("select")
    select.add_argument("--output", required=True); select.add_argument("--expected", required=True)
    check = sub.add_parser("verify")
    check.add_argument("--expected", required=True); check.add_argument("--expected-sha256", required=True); check.add_argument("--implementation-sha", required=True)
    args = parser.parse_args()
    if args.mode == "produce":
        result = produce(args.spec, args.spec_sha256, args.output)
        print(json.dumps({"status": result["status"], "implementationSHA": result["implementationSHA"]}))
    elif args.mode == "select":
        result = selection(args.output, args.expected)
        print(json.dumps({"status": "SELECTED", "files": len(result["files"]), "expectedSHA256": digest(read_regular(args.expected))}))
    else:
        print(json.dumps(verify(args.expected, args.expected_sha256, args.implementation_sha)))
if __name__ == "__main__":
    try:
        main()
    except (Rejected, OSError, ValueError, KeyError, TypeError, IndexError, subprocess.SubprocessError) as error:
        print(json.dumps({"status": "REJECTED", "error": str(error) if isinstance(error, Rejected) else type(error).__name__}), file=sys.stderr)
        raise SystemExit(1)
