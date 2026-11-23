#!/usr/bin/env python3
"""Verify actual staged Remote Builder entrypoints and causal integrity failures.

Uses only synthetic unsigned sources. Mutations are confined to the inactive
candidate, restored after each check, and never applied to shared Go tools.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import plistlib
import signal
import subprocess
import tempfile
import time


def digest(path):
    return hashlib.sha256(Path(path).read_bytes()).hexdigest()


def execute_owned(command, env, cwd, timeout):
    """Wait for or terminate the entire owned process group before returning."""
    process = subprocess.Popen([str(part) for part in command], env=env, cwd=cwd,
                               text=True, errors="replace", stdout=subprocess.PIPE,
                               stderr=subprocess.PIPE, start_new_session=True)
    try:
        stdout, stderr = process.communicate(timeout=timeout)
        return process.returncode, stdout, stderr
    except subprocess.TimeoutExpired:
        try:
            os.killpg(process.pid, signal.SIGTERM)
        except ProcessLookupError:
            pass
        try:
            stdout, stderr = process.communicate(timeout=3)
        except subprocess.TimeoutExpired:
            try:
                os.killpg(process.pid, signal.SIGKILL)
            except ProcessLookupError:
                pass
            stdout, stderr = process.communicate(timeout=3)
        # A parent may exit on TERM while a descendant ignores it and has closed
        # the inherited pipes. Kill any remaining member before state restoration.
        try:
            os.killpg(process.pid, signal.SIGKILL)
        except ProcessLookupError:
            pass
        return None, stdout, stderr + "\nAUDIT_TIMEOUT_NOT_A_PASS"


def synthetic_workflow_matches(actions, dialect):
    """Independent exact producer/consumer oracle for these two tiny fixtures."""
    if dialect == "v2":
        if not actions:
            return False
        terminal = actions[-1]
        if (not isinstance(terminal, dict)
                or terminal.get("WFWorkflowActionIdentifier") != "is.workflow.actions.nothing"
                or terminal.get("WFWorkflowActionParameters", {}) != {}):
            return False
        # V2 main-void lowering emits one parameter-free terminal Nothing.
        # Keep every producer/consumer action and the caller's input unchanged.
        actions = actions[:-1]
    expected_ids = ["is.workflow.actions.gettext"]
    if dialect == "legacy":
        expected_ids.append("is.workflow.actions.setvariable")
    expected_ids.append("is.workflow.actions.showresult")
    if [a.get("WFWorkflowActionIdentifier") for a in actions] != expected_ids:
        return False
    producer = actions[0].get("WFWorkflowActionParameters", {})
    expected_text = "P10R15 staged " + ("Legacy" if dialect == "legacy" else "V2") + " verification"
    uuid = producer.get("UUID")
    if producer.get("WFTextActionText") != expected_text or not isinstance(uuid, str) or not uuid:
        return False
    display = actions[-1].get("WFWorkflowActionParameters", {}).get("Text", {})
    if display.get("WFSerializationType") != "WFTextTokenString":
        return False
    value = display.get("Value", {})
    if value.get("string") != "\ufffc":
        return False
    attachments = value.get("attachmentsByRange", {})
    if set(attachments) != {"{0, 1}"}:
        return False
    actual = attachments["{0, 1}"]
    if dialect == "legacy":
        assignment = actions[1].get("WFWorkflowActionParameters", {})
        source = assignment.get("WFInput", {})
        if assignment.get("WFVariableName") != "message":
            return False
        if source.get("WFSerializationType") != "WFTextTokenAttachment":
            return False
        reference = source.get("Value", {})
        return (reference.get("Type") == "ActionOutput" and reference.get("OutputUUID") == uuid
                and actual == {"Type": "Variable", "VariableName": "message"})
    return actual == {"Type": "ActionOutput", "OutputUUID": uuid, "OutputName": "message"}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--stage", type=Path, required=True)
    parser.add_argument("--live-home", type=Path, required=True)
    parser.add_argument("--report", type=Path, required=True)
    parser.add_argument("--skip-go-tests", action="store_true")
    args = parser.parse_args()
    stage, live = args.stage.resolve(), args.live_home.resolve()
    receipt = json.loads((stage / "staging-receipt.json").read_text())
    candidate = (stage / "remote-home").resolve()
    if candidate != Path(receipt["candidateHome"]).resolve():
        parser.error("staging receipt belongs to another candidate")
    if live == candidate or live.is_relative_to(candidate) or candidate.is_relative_to(live):
        parser.error("candidate must be outside live home")
    if args.report.exists():
        parser.error("existing evidence must not be overwritten")
    release = receipt["tuple"]
    label = release["label"]
    runtime = candidate / label / "runtime"
    skill = candidate / "source" / label / "skills/cherri-shortcuts"
    before = {name: digest(live / name) for name in receipt["liveHashesBefore"]}
    if before != receipt["liveHashesBefore"]:
        parser.error("live state differs from the staged receipt")
    live_link = os.readlink(live / "bin/cherri-remote-builder")
    initial_candidate = {name: digest(candidate / name) for name in receipt["candidateHashes"]}
    if initial_candidate != receipt["candidateHashes"]:
        parser.error("candidate differs from its staging receipt")
    env = os.environ.copy()
    for key in ("CHERRI_BIN", "CHERRI_MIGRATE_BIN", "CHERRI_HOME", "CHERRI_DOCS_DIR", "CHERRI_REF",
                "CHERRI_DOCS_REF", "SKILL_DIR", "CHERRI_REPO", "CHERRI_DOCS_REPO"):
        env.pop(key, None)
    env.update(CHERRI_REMOTE_HOME=str(candidate), CHERRI_OFFLINE="1",
               GOMAXPROCS="2", GOFLAGS="-p=1")
    runner = candidate / "bin/cherri-remote-runner.sh"
    legacy_runner = candidate / "bin/cherri-remote-legacy.sh"
    builder = candidate / "bin/cherri-remote-builder"
    rows = []
    fatal = None

    def execute(name, command, expected=None, timeout=60):
        artifacts_before = {str(p): digest(p) for p in (candidate / "outputs").glob("*.shortcut")}
        start = time.monotonic()
        rc, stdout, stderr = execute_owned(command, env, candidate, timeout)
        artifacts_after = {str(p): digest(p) for p in (candidate / "outputs").glob("*.shortcut")}
        emitted = [Path(path) for path in artifacts_after if path not in artifacts_before]
        native = []
        if expected:
            ok = rc not in (None, 0) and expected in stdout + stderr and artifacts_after == artifacts_before
        else:
            ok = rc == 0 and bool(stdout.strip() or stderr.strip())
            if name.endswith("compile_unsigned"):
                for path in emitted:
                    try:
                        actions = plistlib.loads(path.read_bytes()).get("WFWorkflowActions", [])
                        dialect = "v2" if name == "runner_compile_unsigned" else "legacy"
                        native.append({"name": path.name, "sha256": digest(path), "actions": len(actions),
                                       "syntheticSemanticsMatch": synthetic_workflow_matches(actions, dialect),
                                       "nativeActionIDs": sorted({a["WFWorkflowActionIdentifier"] for a in actions})})
                    except Exception:
                        ok = False
                ok = ok and len(native) == 1 and native[0]["actions"] > 0
                ok = ok and native[0]["syntheticSemanticsMatch"]
                ok = ok and all(artifacts_after.get(path) == sha for path, sha in artifacts_before.items())
            elif name == "skill_full_catalog":
                try:
                    ok = ok and bool(json.loads(stdout))
                except (ValueError, TypeError):
                    ok = False
            elif name == "runner_docs":
                ok = ok and "actions.md" in stdout
            elif name.startswith("immutable_sync"):
                ok = ok and "IMMUTABLE_RELEASE_VERIFIED" in stdout + stderr
                try:
                    ok = ok and json.loads(stdout) == release
                except ValueError:
                    ok = False
            elif name in ("runner_status", "restored_runner_status"):
                try:
                    ok = ok and json.loads(stdout) == release
                except ValueError:
                    ok = False
            elif name in ("legacy_status", "registered_status", "restored_registered_status"):
                expected_status = {"CHERRI_REMOTE_LANGUAGE_MODE": "legacy",
                            "CHERRI_REMOTE_SOURCE_BRANCH": release["sourceBranch"]}
                expected_status.update({key: release[key] for key in
                                 ("compilerCommit", "docsCommit", "schemaFingerprint", "sourceBranch")})
                lines = [line.split("=", 1) for line in stdout.splitlines() if line.strip()]
                ok = ok and len(lines) == len(expected_status) and all(len(pair) == 2 for pair in lines)
                ok = ok and dict(lines) == expected_status
        row = {"check": name, "kind": "CAUSAL_REJECTION" if expected else "ENTRYPOINT",
               "exitCode": rc, "seconds": round(time.monotonic() - start, 3),
               "verdict": "PASS" if ok else "FAIL", "nativeArtifacts": native,
               "stdoutSha256": hashlib.sha256(stdout.encode()).hexdigest(),
               "stdoutBytes": len(stdout.encode()),
               "diagnosticTail": (stdout + stderr)[-1800:]}
        rows.append(row)
        print(json.dumps({key: row[key] for key in ("check", "exitCode", "verdict", "seconds")}), flush=True)
        return ok

    def file_mutation(name, path, replacement, command, diagnostic):
        original = path.read_bytes()
        try:
            path.write_bytes(replacement)
            execute(name, command, diagnostic)
        finally:
            path.write_bytes(original)
        if path.read_bytes() != original:
            raise RuntimeError("candidate restoration failed: " + name)

    try:
        v2 = candidate / "jobs/release-audit-v2.cherri"
        legacy = candidate / "jobs/release-audit-legacy.cherri"
        for path in (v2, legacy):
            if path.exists():
                raise ValueError("synthetic audit source already exists; use a fresh stage")
        v2.write_text('let message = "P10R15 staged V2 verification"\nshow(message)\n')
        legacy.write_text('@message = "P10R15 staged Legacy verification"\nshow("{@message}")\n')
        skill_env = env.copy()
        skill_env.update(CHERRI_BIN=str(runtime / "bin/cherri"),
                         CHERRI_MIGRATE_BIN=str(runtime / "bin/cherri-migrate"), CHERRI_HOME=str(runtime),
                         CHERRI_DOCS_DIR=str(runtime / "docs"), SKILL_DIR=str(skill))
        cases = [
            ("runner_status", [runner, "status"]),
            ("legacy_status", [legacy_runner, "status"]),
            ("registered_status", [builder, "status"]),
            ("runner_check", [runner, "check", v2]),
            ("legacy_check", [legacy_runner, "check", legacy]),
            ("registered_check", [builder, "check", legacy]),
            ("runner_compile_unsigned", [runner, "compile", v2, "--unsigned"]),
            ("legacy_compile_unsigned", [legacy_runner, "compile", legacy, "--unsigned"]),
            ("registered_compile_unsigned", [builder, "compile", legacy, "--unsigned"]),
            ("runner_catalog", [runner, "catalog", "show"]),
            ("runner_actions", [runner, "actions", "show"]),
            ("runner_docs", [runner, "docs", "show"]),
            ("immutable_sync", [runner, "sync"]),
            ("immutable_sync_p15", [runner, "sync", "--require-p15"]),
            ("immutable_sync_p10_compatibility", [runner, "sync", "--require-p10"]),
        ]
        for name, command in cases:
            execute(name, command)
        saved_env = env
        env = skill_env
        try:
            execute("skill_full_catalog", ["sh", skill / "scripts/catalog.sh"])
        finally:
            env = saved_env
        if args.skip_go_tests:
            rows.append({"check": "runner_existing_tests", "kind": "ENTRYPOINT", "verdict": "NOT_RUN",
                         "reason": "Explicit --skip-go-tests; this report cannot satisfy activation"})
        else:
            execute("runner_existing_tests", [runner, "test"], timeout=600)
        active_path = candidate / "active.json"
        for field in release:
            changed = dict(release)
            changed[field] = "p15-causal-mismatch" if field == "label" else "0" * len(str(release[field]))
            if changed[field] == release[field]:
                raise ValueError("causal mutation did not change " + field)
            file_mutation("reject_tuple_" + field, active_path,
                          (json.dumps(changed) + "\n").encode(), [runner, "status"],
                          "IMMUTABLE_RELEASE_MISMATCH")
        for kind, repo in (("skill", candidate / "source" / label),
                           ("compiler", runtime / "src"), ("docs", runtime / "docs")):
            marker = repo / "p15_remote_causal_control.untracked"
            if marker.exists():
                raise ValueError("candidate marker already exists")
            try:
                marker.write_text("Synthetic integrity control; never part of a release.\n")
                tracked = subprocess.check_output(["git", "ls-files", "--others", "--exclude-standard",
                                                   "--", marker.name], cwd=repo, text=True).strip()
                if tracked != marker.name:
                    raise ValueError("causal marker is ignored, so the control would be invalid")
                execute("reject_dirty_" + kind, [runner, "status"], "IMMUTABLE_SOURCE_MISMATCH")
            finally:
                marker.unlink(missing_ok=True)
        binary = runtime / "bin/cherri"
        file_mutation("reject_binary_drift", binary, binary.read_bytes() + b"\nP15_CAUSAL_CONTROL\n",
                      [runner, "status"], "IMMUTABLE_COMPILER_MISMATCH")
        migration = runtime / "bin/cherri-migrate"
        file_mutation("reject_migration_drift", migration, migration.read_bytes() + b"\nP15_CAUSAL_CONTROL\n",
                      [runner, "status"], "IMMUTABLE_MIGRATION_MISMATCH")
        tools_link = candidate / "tools"
        original_link = os.readlink(tools_link)
        with tempfile.TemporaryDirectory(prefix="p15-tool-causal-", dir=stage) as temp:
            alternate = Path(temp) / "tools"
            for relative in receipt["externalToolchainDependencies"]:
                destination = alternate / Path(relative).relative_to("tools")
                destination.parent.mkdir(parents=True, exist_ok=True)
                if relative == "tools/go/VERSION":
                    destination.write_bytes((live / relative).read_bytes() + b"\nP15_CAUSAL_CONTROL\n")
                else:
                    destination.symlink_to(live / relative)
            try:
                tools_link.unlink()
                tools_link.symlink_to(alternate, target_is_directory=True)
                execute("reject_toolchain_dependency_drift", [runner, "status"], "TOOLCHAIN_DEPENDENCY_MISMATCH")
            finally:
                tools_link.unlink(missing_ok=True)
                tools_link.symlink_to(original_link, target_is_directory=True)
        mode = candidate / "config/default-language-mode"
        file_mutation("reject_implicit_language_switch", mode, b"v2\n", [builder, "status"],
                      "default language mode is not Legacy")
        validator = candidate / "bin/cherri-shortcut-lint.py"
        for name, command in (("legacy", [builder, "check", legacy]),
                              ("v2", [runner, "compile", v2, "--unsigned"])):
            file_mutation("reject_validator_drift_" + name, validator,
                          validator.read_bytes() + b"\n# synthetic integrity control\n",
                          command, "checksum")
        file_mutation("reject_registered_runner_drift", runner,
                      runner.read_bytes() + b"\n# synthetic integrity control\n",
                      [builder, "status"], "runner checksum changed")
        execute("restored_registered_status", [builder, "status"])
        execute("restored_runner_status", [runner, "status"])
    except Exception as exc:
        fatal = type(exc).__name__ + ": " + str(exc)
    finally:
        after = {name: digest(live / name) for name in before}
        final_candidate = {name: digest(candidate / name) for name in initial_candidate}
        unchanged = before == after and os.readlink(live / "bin/cherri-remote-builder") == live_link
        restored = initial_candidate == final_candidate
        failures = sum(row["verdict"] == "FAIL" for row in rows)
        not_run = sum(row["verdict"] == "NOT_RUN" for row in rows)
        report = {"formatVersion": 1, "scope": "INACTIVE_CANDIDATE_REAL_ENTRYPOINTS_AND_CAUSAL_INTEGRITY",
                  "observedAtUtc": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
                  "tuple": release, "privateCorpusUsed": False, "livePromotion": False,
                  "liveHashesBefore": before, "liveHashesAfter": after, "liveUnchanged": unchanged,
                  "candidateHashesBefore": initial_candidate, "candidateHashesAfter": final_candidate,
                  "candidateRestored": restored, "results": rows, "fatalError": fatal,
                  "summary": {"checks": len(rows), "passed": sum(row["verdict"] == "PASS" for row in rows),
                              "failed": failures, "notRun": not_run},
                  "activationGatePassed": not failures and not not_run and not fatal and unchanged and restored}
        args.report.parent.mkdir(parents=True, exist_ok=True)
        args.report.write_text(json.dumps(report, indent=2) + "\n")
        print(json.dumps({"summary": report["summary"], "activationGatePassed": report["activationGatePassed"],
                          "fatalError": fatal}), flush=True)
    return 0 if report["activationGatePassed"] else 1


if __name__ == "__main__":
    raise SystemExit(main())
