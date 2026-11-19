#!/usr/bin/env python3
"""Exercise actual immutable Remote Builder routes with public synthetic inputs.

The signed checks intentionally contact the configured canonical signing service.
AEA1 plus successful compiler validation is a transport/envelope observation;
this tool does not claim independent Apple cryptographic or device validation.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import plistlib
import stat
import time
import uuid

from verify_remote_release import digest, execute_owned, synthetic_workflow_matches


def scrubbed_env(home, use_default_home):
    env = os.environ.copy()
    for key in ("CHERRI_BIN", "CHERRI_HOME", "CHERRI_DOCS_DIR", "CHERRI_REF",
                "CHERRI_DOCS_REF", "SKILL_DIR", "CHERRI_REPO", "CHERRI_DOCS_REPO",
                "CHERRI_REMOTE_HOME"):
        env.pop(key, None)
    if use_default_home:
        if home != (Path(env["HOME"]) / "workspaces/cherri-remote").resolve():
            raise ValueError("Requested home is not the actual default Remote Builder home")
    else:
        env["CHERRI_REMOTE_HOME"] = str(home)
    env.update(CHERRI_OFFLINE="1", PYTHONDONTWRITEBYTECODE="1")
    return env


def status_matches(stdout, release, dialect):
    if dialect == "v2":
        try:
            return json.loads(stdout) == release
        except ValueError:
            return False
    expected = {"CHERRI_REMOTE_LANGUAGE_MODE": "legacy",
                "CHERRI_REMOTE_SOURCE_BRANCH": release["sourceBranch"]}
    expected.update({key: release[key] for key in
                     ("compilerCommit", "docsCommit", "schemaFingerprint", "sourceBranch")})
    pairs = [line.split("=", 1) for line in stdout.splitlines() if line.strip()]
    return (len(pairs) == len(expected) and all(len(pair) == 2 for pair in pairs)
            and dict(pairs) == expected)


def probe(home, release, report_path, use_default_home=False):
    home, report_path = Path(home).resolve(), Path(report_path).resolve()
    if report_path.exists():
        raise ValueError("Existing smoke evidence must not be overwritten")
    if json.loads((home / "active.json").read_text()) != release:
        raise ValueError("Home does not contain the expected immutable release")
    registered = home / "bin/cherri-remote-builder"
    if not registered.is_symlink() or os.readlink(registered) != "cherri-remote-legacy.sh":
        raise ValueError("Actual registered Legacy route changed")
    protected = ("active.json", "bin/cherri-remote-runner.sh", "bin/cherri-remote-legacy.sh",
                 "bin/cherri-shortcut-lint.py", "config/default-language-mode")
    before = {name: digest(home / name) for name in protected}
    modes_before = {name: stat.S_IMODE((home / name).lstat().st_mode) for name in protected}
    link_before = {"target": os.readlink(registered), "mode": stat.S_IMODE(registered.lstat().st_mode)}
    env = scrubbed_env(home, use_default_home)
    prefix = "p15-release-smoke-" + uuid.uuid4().hex
    rows, sources = [], []
    fatal = None
    signing_attempts = 0
    try:
        for dialect, route in (("v2", home / "bin/cherri-remote-runner.sh"),
                               ("legacy", registered)):
            source = home / "jobs" / (prefix + "-" + dialect + ".cherri")
            content = ('let message = "P10R15 staged V2 verification"\nshow(message)\n'
                       if dialect == "v2" else
                       '@message = "P10R15 staged Legacy verification"\nshow("{@message}")\n')
            with source.open("x", encoding="utf-8", newline="\n") as output:
                output.write(content)
            source_hash = digest(source)
            sources.append({"file": source.name, "sha256": source_hash, "synthetic": True})
            for operation in ("status", "check", "unsigned", "signed"):
                prior = {str(p): digest(p) for p in (home / "outputs").glob(source.stem + "-*.shortcut")}
                command = [route, operation] if operation == "status" else [route, "check", source]
                if operation in ("unsigned", "signed"):
                    command = [route, "compile", source, "--" + operation]
                start = time.monotonic()
                if operation == "signed":
                    signing_attempts += 1
                rc, stdout, stderr = execute_owned(command, env, home, 75)
                after = {str(p): digest(p) for p in (home / "outputs").glob(source.stem + "-*.shortcut")}
                emitted = [Path(p) for p in after if p not in prior]
                stable = all(after.get(p) == sha for p, sha in prior.items()) and digest(source) == source_hash
                ok = rc == 0 and stable
                artifacts = []
                if operation in ("status", "check"):
                    ok = ok and after == prior
                    if operation == "status":
                        ok = ok and status_matches(stdout, release, dialect)
                else:
                    ok = ok and len(emitted) == 1
                    for path in emitted:
                        raw = path.read_bytes()
                        item = {"file": path.name, "sha256": digest(path), "bytes": len(raw),
                                "magicHex": raw[:4].hex()}
                        if operation == "unsigned":
                            try:
                                actions = plistlib.loads(raw).get("WFWorkflowActions", [])
                                item["nativeActions"] = len(actions)
                                item["nativeActionIDs"] = [action.get("WFWorkflowActionIdentifier") for action in actions]
                                item["syntheticSemanticsMatch"] = synthetic_workflow_matches(actions, dialect)
                                ok = ok and item["syntheticSemanticsMatch"]
                            except Exception:
                                ok = False
                        else:
                            item["aea1EnvelopeObserved"] = len(raw) > 12 and raw[:4] == b"AEA1"
                            ok = ok and item["aea1EnvelopeObserved"]
                        artifacts.append(item)
                row = {"route": dialect, "operation": operation, "exitCode": rc,
                       "seconds": round(time.monotonic() - start, 3),
                       "verdict": "PASS" if ok else "FAIL", "artifacts": artifacts,
                       "sourceUnchanged": digest(source) == source_hash,
                       "stdoutSha256": hashlib.sha256(stdout.encode()).hexdigest(),
                       "stdoutBytes": len(stdout.encode()),
                       "stderrSha256": hashlib.sha256(stderr.encode()).hexdigest(),
                       "stderrBytes": len(stderr.encode()),
                       "diagnosticTail": (stdout + stderr)[-2000:]}
                rows.append(row)
                print(json.dumps({key: row[key] for key in
                                  ("route", "operation", "exitCode", "verdict")}), flush=True)
                if not ok:
                    raise RuntimeError("Actual route failed: " + dialect + "/" + operation)
    except Exception as exc:
        fatal = type(exc).__name__ + ": " + str(exc)
    after_hashes, modes_after, link_after = {}, {}, None
    try:
        after_hashes = {name: digest(home / name) for name in protected}
        modes_after = {name: stat.S_IMODE((home / name).lstat().st_mode) for name in protected}
        link_after = {"target": os.readlink(registered), "mode": stat.S_IMODE(registered.lstat().st_mode)}
    except OSError as error:
        fatal = (fatal + "; " if fatal else "") + "protected-state: " + str(error)
    unchanged = before == after_hashes and modes_before == modes_after and link_before == link_after
    report = {"formatVersion": 1, "scope": "ACTUAL_REMOTE_ROUTES_PUBLIC_SYNTHETIC_SIGNING",
              "observedAtUtc": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
              "tuple": release, "remoteHome": str(home), "usedDefaultHome": use_default_home,
              "privateCorpusUsed": False, "externalSigningRequested": signing_attempts > 0,
              "signingCommandAttempts": signing_attempts,
              "appleCryptographicValidation": "NOT_TESTED",
              "deviceExecution": "NOT_TESTED", "sources": sources, "results": rows,
              "protectedHashesBefore": before, "protectedHashesAfter": after_hashes,
              "protectedModesBefore": modes_before, "protectedModesAfter": modes_after,
              "registeredLinkBefore": link_before, "registeredLinkAfter": link_after,
              "protectedUnchanged": unchanged, "fatalError": fatal,
              "summary": {"checks": len(rows), "passed": sum(r["verdict"] == "PASS" for r in rows),
                          "failed": sum(r["verdict"] == "FAIL" for r in rows)},
              "passed": len(rows) == 8 and all(r["verdict"] == "PASS" for r in rows)
                        and unchanged and not fatal}
    report_path.parent.mkdir(parents=True, exist_ok=True)
    with report_path.open("x", encoding="utf-8", newline="\n") as output:
        json.dump(report, output, indent=2, sort_keys=True)
        output.write("\n")
    return report


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--remote-home", required=True, type=Path)
    parser.add_argument("--report", required=True, type=Path)
    parser.add_argument("--use-default-home", action="store_true")
    args = parser.parse_args()
    release = json.loads((args.remote_home / "active.json").read_text())
    report = probe(args.remote_home, release, args.report, args.use_default_home)
    print(json.dumps({"passed": report["passed"], "summary": report["summary"],
                      "fatalError": report["fatalError"]}))
    return 0 if report["passed"] else 1


if __name__ == "__main__":
    raise SystemExit(main())
