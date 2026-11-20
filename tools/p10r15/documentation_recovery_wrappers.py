#!/usr/bin/env python3
"""Execute genuine unpacked Skill documentation wrappers with exact input identities.

This collector does not publish, update pins, fetch repositories, or grant formal
acceptance. Its successful result covers only the explicitly recorded package,
compiler, docs checkout and original wrapper calls. Fresh canonical generation
and complete cross-repository snippet acceptance are separate producers.
"""
from __future__ import annotations

import argparse
import datetime
import hashlib
import json
import os
from pathlib import Path, PurePosixPath
import re
import shutil
import signal
import stat
import subprocess
import sys
import time

import package_skill

SOURCE_PATHS = (
    "tools/p10r15/documentation_recovery_wrappers.py",
    "tools/p10r15/test_documentation_recovery_wrappers.py",
    "tools/p10r15/package_skill.py",
)
DOC_SCOPES = ("fork-language", "language")
ACTION_QUERIES = ("show", "resizeImage", "round")
DOC_QUERIES = ("let ", "Optional", "native.action")
NO_MATCH = "CHERRI_DOC_WRAPPER_UNMATCHED_20261120_15D86E"
MAX_OBJECT = 4 * 1024 * 1024
MAX_TOTAL = 16 * 1024 * 1024
MAX_OUTPUT = 8 * 1024 * 1024
SHA = re.compile(r"[0-9a-f]{40}")
IDENTITY_FIELDS = (
    "languageVersion", "catalogSchemaVersion", "protocolVersion",
    "schemaFingerprint", "codecFingerprint", "macroFingerprint",
)


def digest(data):
    return hashlib.sha256(data).hexdigest()


def canonical(value):
    return (json.dumps(value, ensure_ascii=False, indent=2, sort_keys=True) + "\n").encode("utf-8")


def strict_json(data):
    def pairs(values):
        result = {}
        for key, value in values:
            if key in result:
                raise ValueError("duplicate JSON key: " + key)
            result[key] = value
        return result

    def constant(value):
        raise ValueError("non-finite JSON number: " + value)

    return json.loads(data.decode("utf-8", errors="strict"),
                      object_pairs_hook=pairs, parse_constant=constant)


def git(repo, *args, input_data=None):
    result = subprocess.run(
        ["git", "--no-replace-objects", "-C", str(repo), *args],
        input=input_data, stdout=subprocess.PIPE, stderr=subprocess.PIPE,
        timeout=60, env={**os.environ, "GIT_NO_REPLACE_OBJECTS": "1"})
    if result.returncode:
        raise ValueError("Git read failed: " + str(args[:2]) + ": " +
                         result.stderr.decode("utf-8", errors="replace")[-2000:])
    return result.stdout


def repository_state(repo, expected):
    if not SHA.fullmatch(expected):
        raise ValueError("source must be an immutable full commit")
    head = git(repo, "rev-parse", "HEAD").decode("ascii").strip()
    if head != expected or git(repo, "rev-parse", expected + "^{commit}").decode("ascii").strip() != expected:
        raise ValueError("repository HEAD differs from exact source")
    flags = git(repo, "ls-files", "-v", "-z")
    for row in flags.split(b"\0"):
        if row and (row[:1] == b"S" or row[:1].islower()):
            raise ValueError("unverified Git index flag")
    status = git(repo, "status", "--porcelain=v1", "--untracked-files=all")
    if status:
        raise ValueError("repository must be clean")
    return {"commit": head, "statusSha256": digest(status),
            "indexFlagsSha256": digest(flags)}


def git_files(repo, commit, paths):
    if not SHA.fullmatch(commit):
        raise ValueError("blob source must be an immutable full commit")
    if git(repo, "rev-parse", commit + "^{commit}").decode("ascii").strip() != commit:
        raise ValueError("blob source commit resolution differs")
    listing = git(repo, "ls-tree", "-r", "-z", commit, "--", *paths)
    entries = []
    seen = set()
    for raw in listing.split(b"\0"):
        if not raw:
            continue
        metadata, name = raw.split(b"\t", 1)
        mode, kind, oid = metadata.decode("ascii").split()
        path = name.decode("utf-8", errors="strict")
        pure = PurePosixPath(path)
        if (kind != "blob" or mode not in ("100644", "100755") or
                pure.is_absolute() or ".." in pure.parts or path in seen):
            raise ValueError("unsupported immutable source entry")
        seen.add(path)
        entries.append((path, mode, oid))
    if not entries:
        raise ValueError("empty immutable source inventory")
    batch_input = "".join(oid + "\n" for _, _, oid in entries).encode("ascii")
    sizes = git(repo, "cat-file", "--batch-check", input_data=batch_input).splitlines()
    total = 0
    if len(sizes) != len(entries):
        raise ValueError("incomplete Git size preflight")
    for (_, _, oid), header in zip(entries, sizes):
        fields = header.split()
        if len(fields) != 3 or fields[:2] != [oid.encode(), b"blob"]:
            raise ValueError("unexpected Git object")
        size = int(fields[2])
        if size < 0 or size > MAX_OBJECT:
            raise ValueError("Git object exceeds byte budget")
        total += size + len(header) + 2
    if total > MAX_TOTAL:
        raise ValueError("combined Git sources exceed byte budget")
    raw = git(repo, "cat-file", "--batch", input_data=batch_input)
    if len(raw) > MAX_TOTAL:
        raise ValueError("Git batch exceeds byte budget")
    result, offset = {}, 0
    for path, mode, oid in entries:
        end = raw.find(b"\n", offset)
        if end < offset:
            raise ValueError("missing Git object header")
        fields = raw[offset:end].split()
        if len(fields) != 3 or fields[:2] != [oid.encode(), b"blob"]:
            raise ValueError("Git object identity changed")
        size = int(fields[2])
        data = raw[end + 1:end + 1 + size]
        offset = end + 1 + size
        if len(data) != size or raw[offset:offset + 1] != b"\n":
            raise ValueError("incomplete Git blob")
        offset += 1
        result[path] = (data, int(mode, 8), oid)
    if offset != len(raw):
        raise ValueError("extra Git object bytes")
    return result


def regular_tree(root):
    root = Path(root)
    if not root.is_dir() or root.is_symlink():
        raise ValueError("input directory is missing or linked")
    result = {}
    for directory, dirs, files in os.walk(root, followlinks=False):
        for name in dirs + files:
            path = Path(directory) / name
            info = path.lstat()
            if path.is_symlink() or getattr(info, "st_file_attributes", 0) & 0x400:
                raise ValueError("linked input entry: " + str(path))
            if name in files:
                if not stat.S_ISREG(info.st_mode):
                    raise ValueError("nonregular input entry")
                result[path.relative_to(root).as_posix()] = path
    return result


def verify_package(directory, expected):
    actual = regular_tree(directory)
    wanted = {name.removeprefix(package_skill.PREFIX): value for name, value in expected.items()}
    if set(actual) != set(wanted):
        raise ValueError("unpacked package inventory differs")
    rows = []
    for name, (data, mode) in sorted(wanted.items()):
        path = actual[name]
        if path.read_bytes() != data:
            raise ValueError("unpacked package bytes differ: " + name)
        if os.name != "nt" and stat.S_IMODE(path.stat().st_mode) != stat.S_IMODE(mode):
            raise ValueError("unpacked package mode differs: " + name)
        rows.append({"path": name, "bytes": len(data), "sha256": digest(data),
                     "gitMode": oct(mode), "posixModeVerified": os.name != "nt"})
    return rows


def verify_docs(directory, commit):
    state = repository_state(directory, commit)
    source = git_files(directory, commit, DOC_SCOPES)
    actual = {}
    for scope in DOC_SCOPES:
        for name, path in regular_tree(Path(directory) / scope).items():
            actual[scope + "/" + name] = path
    if set(source) != set(actual):
        raise ValueError("docs scopes contain missing or extra files")
    rows = []
    for name, (data, mode, oid) in sorted(source.items()):
        if actual[name].read_bytes() != data:
            raise ValueError("docs bytes differ from pinned Git blob: " + name)
        rows.append({"path": name, "gitBlob": oid, "bytes": len(data),
                     "sha256": digest(data), "gitMode": oct(mode)})
    return state, source, rows


def validate_capabilities(value, manifest):
    if not isinstance(value, dict):
        raise ValueError("capabilities must be an object")
    for field in IDENTITY_FIELDS:
        expected = manifest.get(field, "1.0" if field == "protocolVersion" else None)
        if not isinstance(expected, str) or value.get(field) != expected:
            raise ValueError("capability identity mismatch: " + field)
    if type(value.get("actionCount")) is not int or value["actionCount"] <= 0:
        raise ValueError("invalid action count")


def validate_catalog(value, capabilities):
    if (not isinstance(value, dict) or value.get("ok") is not True or
            str(value.get("version")) != "2" or
            type(value.get("count")) is not int or
            not isinstance(value.get("actions"), list)):
        raise ValueError("invalid complete catalog response")
    actions = value["actions"]
    names = [action.get("name") for action in actions if isinstance(action, dict)]
    if (len(actions) != value["count"] or len(actions) != capabilities["actionCount"] or
            len(names) != len(actions) or any(not isinstance(name, str) or not name for name in names) or
            len(set(names)) != len(names)):
        raise ValueError("catalog inventory differs")
    return {action["name"]: action for action in actions}


def reference_body(raw):
    if not raw.startswith(b"---\n"):
        raise ValueError("pinned docs reference lacks reviewed front matter")
    end = raw.find(b"\n---\n", 4)
    if end < 0 or raw[end + 5:end + 6] != b"\n":
        raise ValueError("pinned docs reference front matter is malformed")
    return raw[end + 6:]


def verify_reference(docs_raw, compiler_raw, capabilities, catalog):
    body = reference_body(docs_raw)
    if body != compiler_raw:
        raise ValueError("complete generated reference differs across source pins")
    text = body.decode("utf-8", errors="strict")
    for label, field in (("Schema Fingerprint", "schemaFingerprint"),
                         ("Compiler Macro Fingerprint", "macroFingerprint")):
        found = re.findall(r"\*\*" + label + r":\*\* \x60([0-9a-f]{64})\x60", text)
        if found != [capabilities[field]]:
            raise ValueError("reference fingerprint differs: " + field)
    count = re.findall(r"\*\*Total Actions:\*\* ([0-9]+)", text)
    if count != [str(capabilities["actionCount"])]:
        raise ValueError("reference action count differs")
    if text.count("## Compiler macros\n") != 1:
        raise ValueError("reference macro boundary is ambiguous")
    native = text.split("## Compiler macros\n", 1)[0]
    names = re.findall(r"^### \x60([^\x60]+)\x60$", native, re.MULTILINE)
    if len(names) != len(set(names)) or set(names) != set(catalog):
        raise ValueError("full reference callable inventory differs")
    return {"bytes": len(body), "sha256": digest(body), "nativeActions": len(names),
            "comparison": "Exact full committed generated reference body at compiler S equals pinned docs D; fresh generation is a separate producer."}


class Recorder:
    def __init__(self, directory):
        self.directory = Path(directory)
        self.directory.mkdir(parents=True, exist_ok=False)
        self.rows = []

    def run(self, label, command, cwd, env, timeout=60):
        if not re.fullmatch(r"[a-z0-9][a-z0-9-]*", label) or any(r["id"] == label for r in self.rows):
            raise ValueError("duplicate or invalid process label")
        out = self.directory / (label + ".stdout")
        err = self.directory / (label + ".stderr")
        started = datetime.datetime.now(datetime.timezone.utc).isoformat()
        start = time.monotonic()
        timed_out = False
        row = {"id": label, "command": [str(x) for x in command], "cwd": str(cwd),
               "startedUTC": started, "timeoutSeconds": timeout}
        try:
            with out.open("xb") as stdout, err.open("xb") as stderr:
                process = subprocess.Popen(command, cwd=cwd, env=env, stdout=stdout,
                                           stderr=stderr, stdin=subprocess.DEVNULL,
                                           start_new_session=os.name != "nt")
                try:
                    code = process.wait(timeout=timeout)
                except subprocess.TimeoutExpired:
                    timed_out = True
                    if os.name == "nt":
                        subprocess.run(["taskkill", "/PID", str(process.pid), "/T", "/F"],
                                       stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, timeout=20)
                    else:
                        os.killpg(process.pid, signal.SIGKILL)
                    code = process.wait(timeout=20)
            row.update({"exitCode": code, "timedOut": timed_out})
        except OSError as error:
            row.update({"exitCode": None, "timedOut": timed_out, "launchError": str(error)})
        finally:
            row["elapsedSeconds"] = time.monotonic() - start
            row["endedUTC"] = datetime.datetime.now(datetime.timezone.utc).isoformat()
            for key, path in (("stdout", out), ("stderr", err)):
                data = path.read_bytes() if path.exists() else b""
                row[key] = {"path": path.name, "bytes": len(data), "sha256": digest(data)}
            self.rows.append(row)
            (self.directory / (label + ".process.json")).write_bytes(canonical(row))
        if max(row["stdout"]["bytes"], row["stderr"]["bytes"]) > MAX_OUTPUT:
            raise ValueError("process output exceeds reviewed byte budget")
        return row, out.read_bytes(), err.read_bytes()


def succeeded(result, code=0):
    row, stdout, stderr = result
    if row["timedOut"] or row["exitCode"] != code:
        raise ValueError("unexpected process outcome: " + row["id"])
    return stdout, stderr


def wrapper_environment(home, compiler, docs):
    result = {key: value for key, value in os.environ.items()
              if not key.startswith(("CHERRI_", "GIT_")) and key not in ("BASH_ENV", "ENV", "CDPATH")}
    result.update({"CHERRI_HOME": Path(home).as_posix(),
                   "CHERRI_BIN": Path(compiler).as_posix(),
                   "CHERRI_DOCS_DIR": Path(docs).as_posix(),
                   "GIT_NO_REPLACE_OBJECTS": "1", "LC_ALL": "C"})
    return result


def verify_search(raw, docs, scope, query, source):
    expression = re.compile(query.encode("utf-8"), re.IGNORECASE)
    expected = set()
    for name, (data, _, _) in source.items():
        if not name.startswith(scope + "/") or Path(name).suffix not in (".md", ".html", ".txt") or b"\0" in data:
            continue
        for line, content in enumerate(data.splitlines(), 1):
            if expression.search(content):
                expected.add((name, line, content))
    found = set()
    for raw_line in raw.splitlines():
        match = re.fullmatch(rb"(.+?):([1-9][0-9]*):(.*)", raw_line)
        if not match:
            raise ValueError("malformed documentation search row")
        path = Path(match[1].decode("utf-8", errors="strict"))
        try:
            relative = path.resolve().relative_to(Path(docs).resolve()).as_posix()
        except ValueError as error:
            raise ValueError("search escaped pinned docs scope") from error
        item = (relative, int(match[2]), match[3])
        if item not in expected or item in found:
            raise ValueError("search row differs from original pinned source line")
        found.add(item)
    if len(found) != min(len(expected), 80):
        raise ValueError("search result coverage differs")
    return {"matches": len(found), "fullSourceMatches": len(expected),
            "scope": scope, "query": query, "headLimit": 80}


def collect(repo, source_commit, package_dir, compiler, docs_dir, evidence_dir, go="go", bash="sh"):
    repo, package_dir, compiler, docs_dir = (Path(p).absolute() for p in (repo, package_dir, compiler, docs_dir))
    evidence = Path(evidence_dir).absolute()
    evidence.mkdir(parents=True, exist_ok=False)
    recorder = Recorder(evidence / "calls")
    report = {"schemaVersion": 1, "status": "RUNNING", "sourceCommit": source_commit,
              "componentTupleOnly": True, "formalAcceptanceGranted": False,
              "requirementsClosed": [], "calls": recorder.rows, "checks": {}}
    work = evidence / "work"
    work.mkdir()
    before, package_before, docs_before, binary_hash = None, None, None, None
    try:
        before = repository_state(repo, source_commit)
        source_files = git_files(repo, source_commit, SOURCE_PATHS)
        if set(source_files) != set(SOURCE_PATHS):
            raise ValueError("collector source inventory incomplete")
        for name, (data, _, _) in source_files.items():
            if (repo / name).read_bytes() != data:
                raise ValueError("executed collector source differs from immutable source")
        if Path(__file__).resolve() != (repo / SOURCE_PATHS[0]).resolve():
            raise ValueError("collector was not executed from the declared repository")
        report["collectorSources"] = [
            {"path": name, "gitBlob": oid, "bytes": len(data), "sha256": digest(data)}
            for name, (data, _, oid) in sorted(source_files.items())]
        expected, package_receipt = package_skill.expected_package(repo, source_commit, "source")
        package_before = verify_package(package_dir, expected)
        manifest = strict_json(expected[package_skill.PREFIX + "compatibility-manifest.json"][0])
        report["tuple"] = {"K": source_commit, "S": manifest["compilerCommit"], "D": manifest["docsCommit"]}
        report["packageManifest"] = package_receipt
        report["packageFiles"] = package_before
        docs_state, docs_source, docs_before = verify_docs(docs_dir, manifest["docsCommit"])
        report["docsStateBefore"] = docs_state
        report["docsFiles"] = docs_before
        succeeded(recorder.run("compiler-input-build-info", [go, "version", "-m", str(compiler)], work, wrapper_environment(work / "unused-home", compiler, docs_dir)))
        binary, identity = package_skill.inspect_binary(compiler, manifest["compilerCommit"], go)
        binary_hash = digest(binary)
        report["compiler"] = {**identity, "originalPath": str(compiler.resolve()), "bytes": len(binary)}
        actual_binary = work / ("cherri.exe" if identity["goos"] == "windows" else "cherri")
        actual_binary.write_bytes(binary)
        actual_binary.chmod(0o755)
        home = work / "home"
        home.mkdir()
        env = wrapper_environment(home, actual_binary, docs_dir)
        report["selectedEnvironment"] = {key: env[key] for key in
                                         ("CHERRI_HOME", "CHERRI_BIN", "CHERRI_DOCS_DIR", "GIT_NO_REPLACE_OBJECTS", "LC_ALL")}
        succeeded(recorder.run("compiler-build-info", [go, "version", "-m", str(actual_binary)], work, env))
        cap_raw, cap_error = succeeded(recorder.run("compiler-capabilities", [str(actual_binary), "--capabilities-json"], work, env))
        if cap_error:
            raise ValueError("capabilities wrote unexpected stderr")
        capabilities = strict_json(cap_raw)
        validate_capabilities(capabilities, manifest)
        report["capabilities"] = {key: capabilities[key] for key in IDENTITY_FIELDS + ("actionCount",)}
        catalog_raw, catalog_error = succeeded(recorder.run("compiler-catalog", [str(actual_binary), "--actions-json"], work, env))
        if catalog_error:
            raise ValueError("catalog wrote unexpected stderr")
        catalog = validate_catalog(strict_json(catalog_raw), capabilities)
        reference_files = git_files(repo, manifest["compilerCommit"], ("docs/language-v2/actions-reference.md",))
        reference, _, reference_oid = reference_files["docs/language-v2/actions-reference.md"]
        report["generatedReference"] = verify_reference(docs_source["fork-language/actions.md"][0], reference, capabilities, catalog)
        report["generatedReference"].update({"compilerGitBlob": reference_oid, "compilerCommit": manifest["compilerCommit"]})
        script = package_dir / "scripts"
        for name in ACTION_QUERIES:
            if name not in catalog:
                raise ValueError("required action is absent")
            direct, error = succeeded(recorder.run("direct-action-" + name.lower(),
                                                  [str(actual_binary), "--action=" + name, "--no-ansi"], work, env))
            wrapped, wrapped_error = succeeded(recorder.run("wrapper-action-" + name.lower(),
                                                            [bash, (script / "action.sh").as_posix(), name], work, env))
            if error or wrapped_error or wrapped != direct.rstrip(b"\n") + b"\n" or (name + "(").encode() not in wrapped:
                raise ValueError("action wrapper differs from actual compiler: " + name)
            for parameter in catalog[name].get("parameters", []):
                if parameter["name"].encode() not in wrapped:
                    raise ValueError("action query omits actual parameter")
                for choice in parameter.get("enumValues", []):
                    if ("'" + choice + "'").encode() not in wrapped:
                        raise ValueError("action query omits actual enum choice")
            report["checks"]["action-" + name] = True
        wrapped_catalog, error = succeeded(recorder.run("wrapper-catalog", [bash, (script / "catalog.sh").as_posix()], work, env))
        if error or wrapped_catalog != catalog_raw:
            raise ValueError("wrapper catalog differs from complete compiler response")
        needle = '"name": "resizeImage"'
        filtered, error = succeeded(recorder.run("wrapper-catalog-filter", [bash, (script / "catalog.sh").as_posix(), needle], work, env))
        expected_filter = b"".join(line + b"\n" for line in catalog_raw.replace(b",", b"\n").splitlines() if needle.encode() in line)
        if error or not expected_filter or filtered != expected_filter:
            raise ValueError("filtered catalog differs from original full response")
        report["checks"]["complete-catalog"] = True
        report["checks"]["filtered-catalog"] = True
        for index, query in enumerate(DOC_QUERIES):
            data, error = succeeded(recorder.run("docs-active-" + str(index), [bash, (script / "search-docs.sh").as_posix(), query], work, env))
            if error:
                raise ValueError("active docs query wrote stderr")
            observation = verify_search(data, docs_dir, "fork-language", query, docs_source)
            if observation["matches"] == 0:
                raise ValueError("required active docs query matched no source")
            report["checks"]["docs-active-" + str(index)] = observation
        data, error = succeeded(recorder.run("docs-legacy-explicit", [bash, (script / "search-docs.sh").as_posix(), "--legacy-reference", "repeat"], work, env))
        if error:
            raise ValueError("explicit Legacy query wrote stderr")
        observation = verify_search(data, docs_dir, "language", "repeat", docs_source)
        if observation["matches"] == 0:
            raise ValueError("explicit Legacy query matched no original source")
        report["checks"]["explicit-legacy"] = observation
        for data, _, _ in docs_source.values():
            if NO_MATCH.encode().lower() in data.lower():
                raise ValueError("no-match control collides with actual docs")
        data, error = succeeded(recorder.run("docs-no-match", [bash, (script / "search-docs.sh").as_posix(), NO_MATCH], work, env))
        if data or error:
            raise ValueError("no-match query must return empty success")
        data, error = succeeded(recorder.run("docs-invalid-regex", [bash, (script / "search-docs.sh").as_posix(), "["], work, env), 2)
        if data or not error or b"grep:" not in error:
            raise ValueError("invalid-regex control failed for an unrelated reason")
        missing = work / "missing-current"
        missing.mkdir()
        for name, (data, _, _) in docs_source.items():
            if name.startswith("language/"):
                path = missing / name
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_bytes(data)
        missing_env = wrapper_environment(home, actual_binary, missing)
        data, error = succeeded(recorder.run("missing-current-legacy-control", [bash, (script / "search-docs.sh").as_posix(), "--legacy-reference", "repeat"], work, missing_env))
        if error or verify_search(data, missing, "language", "repeat", docs_source)["matches"] == 0:
            raise ValueError("missing-current control lacks readable original Legacy bytes")
        data, error = succeeded(recorder.run("docs-missing-current", [bash, (script / "search-docs.sh").as_posix(), "repeat"], work, missing_env), 1)
        if data or not error.startswith(b"Active V2 documentation directory is missing or linked: "):
            raise ValueError("missing current guide silently fell back or failed for an unrelated reason")
        report["checks"].update({"no-match-empty-success": True, "invalid-regex-propagates": True,
                                 "missing-current-does-not-use-readable-legacy": True})
        if digest(actual_binary.read_bytes()) != binary_hash:
            raise ValueError("executed compiler copy changed")
        report["status"] = "PASS_PACKAGED_DOCUMENTATION_WRAPPERS"
    except (ValueError, OSError, KeyError, TypeError, UnicodeError, subprocess.SubprocessError) as error:
        report["status"] = "FAIL_PACKAGED_DOCUMENTATION_WRAPPERS"
        report["error"] = str(error)
    finally:
        try:
            if before is not None and repository_state(repo, source_commit) != before:
                raise ValueError("source identity changed during wrapper collection")
            if package_before is not None and verify_package(package_dir, expected) != package_before:
                raise ValueError("package changed during wrapper collection")
            if docs_before is not None:
                final_state, _, final_docs = verify_docs(docs_dir, manifest["docsCommit"])
                if final_state != docs_state or final_docs != docs_before:
                    raise ValueError("docs changed during wrapper collection")
            if binary_hash is not None and digest(compiler.read_bytes()) != binary_hash:
                raise ValueError("original compiler changed during wrapper collection")
            report["inputsUnchangedAfter"] = (True if all(value is not None for value in (before, package_before, docs_before, binary_hash)) else None)
        except (ValueError, OSError, KeyError, UnicodeError, subprocess.SubprocessError) as error:
            report["status"] = "FAIL_PACKAGED_DOCUMENTATION_WRAPPERS"
            report["postflightError"] = str(error)
            report["inputsUnchangedAfter"] = False
        report["calls"] = recorder.rows
        report["completedUTC"] = datetime.datetime.now(datetime.timezone.utc).isoformat()
        report["scope"] = ("Actual original unpacked wrappers, pinned public source/docs bytes and compiler metadata. "
                           "The full committed generated reference is compared; fresh generation, snippet semantics, "
                           "native execution, release promotion and final acceptance remain separate.")
        (evidence / "report.json").write_bytes(canonical(report))
    return report


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--repo", type=Path, required=True)
    parser.add_argument("--source-commit", required=True)
    parser.add_argument("--package-dir", type=Path, required=True)
    parser.add_argument("--compiler", type=Path, required=True)
    parser.add_argument("--docs-dir", type=Path, required=True)
    parser.add_argument("--evidence-dir", type=Path, required=True)
    parser.add_argument("--go", default="go")
    parser.add_argument("--bash", default="sh")
    args = parser.parse_args()
    try:
        report = collect(**vars(args))
    except (ValueError, OSError, subprocess.SubprocessError) as error:
        print("[CHERRI_DOC_WRAPPER_COLLECTION_FAILED] " + str(error), file=sys.stderr)
        return 1
    print(json.dumps({"status": report["status"], "sourceCommit": report["sourceCommit"],
                      "tuple": report.get("tuple"), "calls": len(report["calls"]),
                      "formalAcceptanceGranted": False}, sort_keys=True))
    return 0 if report["status"] == "PASS_PACKAGED_DOCUMENTATION_WRAPPERS" else 1


if __name__ == "__main__":
    raise SystemExit(main())
