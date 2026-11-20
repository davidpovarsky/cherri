#!/usr/bin/env python3
"""Audit actual CLI/wrapper rejection of loopback signing faults.

Only synthetic source and locally served bytes are used. Wrapper copies change
only the signer endpoint. An explicit Skill compiler input may select the
manifest-pinned binary separately from the direct CLI input; no pins are rewritten.
Live files and Pure upstream remain untouched.
The report is protocol/exit/artifact evidence, never Apple signature evidence.
"""
import argparse
import base64
import email.policy
from email.parser import BytesParser
import hashlib
import http.server
import json
import os
from pathlib import Path
import plistlib
import re
import shutil
import subprocess
import tempfile
import threading
import time


def digest(path):
    return hashlib.sha256(Path(path).read_bytes()).hexdigest()


def cases():
    fake_archive = b"AEA1" + bytes(range(1, 29))
    envelope = {"success": True, "file": {
        "format": "aea", "content_base64": base64.b64encode(fake_archive).decode(),
        "signed_sha256": "0" * 64}}
    return [
        ("xml", 200, "application/x-plist", b"<?xml version='1.0'?><plist><dict/></plist>"),
        ("html", 200, "text/html", b"<html>synthetic signer error</html>"),
        ("json_error", 200, "application/json", b'{"success":false,"error":{"message":"synthetic fault","code":"TEST"}}'),
        ("non_200", 503, "application/json", b'{"message":"synthetic unavailable"}'),
        ("empty", 200, "application/octet-stream", b""),
        ("corrupt_json", 200, "application/json", b"{broken"),
        ("corrupt_base64", 200, "application/json", b'{"success":true,"file":{"content_base64":"%%%"}}'),
        ("digest_mismatch", 200, "application/json", json.dumps(envelope).encode()),
        ("header_only", 200, "application/octet-stream", b"AEA1"),
        ("zero_archive", 200, "application/octet-stream", b"AEA1" + b"\0" * 28),
        ("timeout", 200, "application/octet-stream", b""),
    ]


def execute(cmd, env, cwd):
    start = time.monotonic()
    try:
        result = subprocess.run(cmd, env=env, cwd=cwd, capture_output=True,
                                text=True, errors="replace", timeout=55)
        return result.returncode, (result.stdout + result.stderr)[-1800:], round(time.monotonic() - start, 3)
    except subprocess.TimeoutExpired:
        return None, "AUDIT_HARNESS_TIMEOUT (not a compiler timeout pass)", round(time.monotonic() - start, 3)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--compiler", required=True, type=Path)
    parser.add_argument("--skill", type=Path)
    parser.add_argument("--skill-compiler", type=Path,
                        help="actual manifest-pinned binary for Skill copies; defaults to --compiler")
    parser.add_argument("--remote-home", type=Path)
    parser.add_argument("--include-timeout", action="store_true")
    parser.add_argument("--report", required=True, type=Path)
    opts = parser.parse_args()
    opts.compiler = opts.compiler.resolve()
    if opts.skill_compiler and not opts.skill:
        parser.error("--skill-compiler requires --skill")
    surfaces = ["cli_v2", "cli_legacy"]
    if opts.skill:
        opts.skill = opts.skill.resolve()
        opts.skill_compiler = (opts.skill_compiler or opts.compiler).resolve()
        surfaces.append("skill_copy")
    if opts.remote_home:
        opts.remote_home = opts.remote_home.resolve()
        surfaces.extend(["remote_legacy_copy", "remote_v2_copy"])
    caps = subprocess.run([str(opts.compiler), "--capabilities-json", "--no-ansi"],
                          capture_output=True, text=True, check=True, timeout=55)
    provenance = {"compilerSha256": digest(opts.compiler), "capabilities": json.loads(caps.stdout)}
    if opts.skill:
        skill_caps = subprocess.run([str(opts.skill_compiler), "--capabilities-json", "--no-ansi"],
                                    capture_output=True, text=True, check=True, timeout=55)
        provenance["skillCompilerSha256"] = digest(opts.skill_compiler)
        provenance["skillCompilerCapabilities"] = json.loads(skill_caps.stdout)
        provenance["skillManifest"] = json.loads((opts.skill / "compatibility-manifest.json").read_text())
        provenance["skillBuildSha256"] = digest(opts.skill / "scripts/build.sh")
    if opts.remote_home:
        provenance["installedTuple"] = json.loads((opts.remote_home / "active.json").read_text())
        remote_label = provenance["installedTuple"].get("label")
        if not isinstance(remote_label, str) or not re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9._-]*", remote_label):
            parser.error("remote active label must be one safe path component")
        provenance["remoteLegacySha256"] = digest(opts.remote_home / "bin/cherri-remote-legacy.sh")
        provenance["remoteRunnerSha256"] = digest(opts.remote_home / "bin/cherri-remote-runner.sh")

    rows = []
    with tempfile.TemporaryDirectory(prefix="p10r15-signing-audit-") as scratch:
        root = Path(scratch)
        for surface in surfaces:
            for name, status, content_type, body in [("unsigned_control", 200, "", b"")] + cases():
                if name == "timeout" and (not opts.include_timeout or surface in ["cli_legacy", "skill_copy"]):
                    continue
                work = root / surface / name
                work.mkdir(parents=True)
                calls = []

                class FaultHandler(http.server.BaseHTTPRequestHandler):
                    def do_POST(self):
                        size = int(self.headers.get("Content-Length", "0"))
                        payload = self.rfile.read(size)
                        request = {"method": "POST", "requestBytes": size,
                                   "contentType": self.headers.get("Content-Type", "")}
                        if surface.startswith("remote_"):
                            message = BytesParser(policy=email.policy.default).parsebytes(
                                ("Content-Type: " + request["contentType"] +
                                 "\r\nMIME-Version: 1.0\r\n\r\n").encode() + payload)
                            uploads = [part.get_payload(decode=True) for part in message.iter_parts()
                                       if part.get_param("name", header="content-disposition") == "file"]
                            preflights = list((work / "tmp").glob("*.shortcut"))
                            exact = len(uploads) == 1 and len(preflights) == 1
                            if exact:
                                request["uploadedPlistSha256"] = hashlib.sha256(uploads[0]).hexdigest()
                                request["preflightSha256"] = digest(preflights[0])
                                exact = request["uploadedPlistSha256"] == request["preflightSha256"]
                            request["exactPreflightBytes"] = exact
                        calls.append(request)
                        if name == "timeout":
                            time.sleep(48)
                        try:
                            self.send_response(status)
                            self.send_header("Content-Type", content_type)
                            self.send_header("Content-Length", str(len(body)))
                            self.end_headers()
                            self.wfile.write(body)
                        except (BrokenPipeError, ConnectionResetError):
                            pass

                    def log_message(self, *args):
                        pass

                server = http.server.ThreadingHTTPServer(("127.0.0.1", 0), FaultHandler)
                server.daemon_threads = True
                thread = threading.Thread(target=server.serve_forever, daemon=True)
                thread.start()
                # A path marker selects the existing multipart adapter without
                # putting another '=' inside a CLI --flag=value argument.
                endpoint = "http://127.0.0.1:%s/shortcuty.app/sign" % server.server_port
                env = os.environ.copy()
                for key in ["CHERRI_BIN", "CHERRI_HOME", "CHERRI_DOCS_DIR", "CHERRI_REF", "CHERRI_DOCS_REF", "SKILL_DIR"]:
                    env.pop(key, None)
                env["CHERRI_OFFLINE"] = "1"
                source_dir = work
                output_dir = work
                if surface.startswith("remote_"):
                    source_dir = work / "jobs"
                    output_dir = work / "outputs"
                    for path in [source_dir, output_dir, work / "bin", work / "config", work / "tmp", work / "source", work / remote_label]:
                        path.mkdir(parents=True, exist_ok=True)
                    for name_in_bin in ["cherri-remote-runner.sh", "cherri-shortcut-lint.py"]:
                        shutil.copy2(opts.remote_home / "bin" / name_in_bin, work / "bin" / name_in_bin)
                    shutil.copy2(opts.remote_home / "active.json", work / "active.json")
                    shutil.copy2(opts.remote_home / "config/default-language-mode", work / "config/default-language-mode")
                    (work / "source" / remote_label).symlink_to(opts.remote_home / "source" / remote_label, target_is_directory=True)
                    (work / remote_label / "runtime").symlink_to(opts.remote_home / remote_label / "runtime", target_is_directory=True)
                    # Preserve the observed, checksum-verified Go dependency of
                    # immutable release runners; do not replace their verifier.
                    if (opts.remote_home / "tools").is_dir():
                        (work / "tools").symlink_to(opts.remote_home / "tools", target_is_directory=True)
                    wrapper_name = ("cherri-remote-runner.sh" if surface == "remote_v2_copy"
                                    else "cherri-remote-legacy.sh")
                    wrapper = (opts.remote_home / "bin" / wrapper_name).read_text()
                    assert wrapper.count("--hubsign") == 1, "unexpected wrapper signing invocation"
                    wrapper_path = work / "bin" / wrapper_name
                    wrapper_path.write_text(wrapper.replace("--hubsign", "--signing-server=" + endpoint))
                    wrapper_path.chmod(0o700)
                    env["CHERRI_REMOTE_HOME"] = str(work)
                source = source_dir / "fixture.cherri"
                source.write_text('show("P10R15 isolated signing audit")\n')
                target = output_dir / "result.shortcut"
                if surface.startswith("cli_"):
                    cmd = [str(opts.compiler), str(source), "--output=" + str(target), "--no-ansi"]
                    if surface == "cli_legacy":
                        cmd += ["--legacy", "--load-all-actions"]
                    cmd += ["--skip-sign"] if name == "unsigned_control" else ["--signing-server=" + endpoint, "--share=anyone"]
                elif surface == "skill_copy":
                    skill = work / "skill"
                    (skill / "scripts").mkdir(parents=True)
                    for path in [opts.skill / "compatibility-manifest.json",
                                 opts.skill / "scripts/common.sh", opts.skill / "scripts/json-object.awk"]:
                        shutil.copy2(path, skill / path.relative_to(opts.skill))
                    wrapper = (opts.skill / "scripts/build.sh").read_text()
                    assert wrapper.count("--hubsign") == 1, "unexpected Skill signing invocation"
                    (skill / "scripts/build.sh").write_text(wrapper.replace("--hubsign", "--signing-server=" + endpoint))
                    env["CHERRI_BIN"] = str(opts.skill_compiler)
                    env["CHERRI_HOME"] = str(work / "home")
                    env["SKILL_DIR"] = str(skill)
                    cmd = ["sh", str(skill / "scripts/build.sh"), str(source), str(target), "--unsigned" if name == "unsigned_control" else "--signed"]
                else:
                    cmd = [str(wrapper_path), "compile", str(source), "--unsigned" if name == "unsigned_control" else "--signed"]
                rc, output, seconds = execute(cmd, env, work)
                server.shutdown()
                server.server_close()
                if surface.startswith("remote_"):
                    artifacts = sorted(output_dir.glob("*.shortcut"))
                elif name == "unsigned_control" and surface.startswith("cli_"):
                    artifacts = sorted(work.glob("*_unsigned.shortcut"))
                    if target.exists():
                        artifacts.append(target)
                else:
                    artifacts = [target] if target.exists() else []
                summaries = [{"bytes": path.stat().st_size, "magicHex": path.read_bytes()[:4].hex(), "sha256": digest(path)} for path in artifacts]
                control_ok = False
                if name == "unsigned_control" and rc == 0 and artifacts:
                    control_ok = all(bool(plistlib.loads(path.read_bytes()).get("WFWorkflowActions")) for path in artifacts)
                multipart = bool(calls) and all(c["contentType"].startswith("multipart/form-data;") for c in calls)
                exact_preflight = (not surface.startswith("remote_") or
                                   bool(calls) and all(c.get("exactPreflightBytes") is True for c in calls))
                diagnostic_ok = name != "digest_mismatch" or "signed_sha256" in output
                verdict = "CONTROL_PASS" if control_ok else (
                    "REJECTED_NONZERO_NO_ARTIFACT" if name != "unsigned_control" and rc not in [0, None]
                    and not artifacts and multipart and diagnostic_ok and exact_preflight else "FAIL")
                row = {"surface": surface, "case": name, "exitCode": rc, "seconds": seconds,
                       "localRequests": len(calls), "multipartProtocolObserved": multipart,
                       "exactPreflightBytesObserved": exact_preflight if surface.startswith("remote_") and calls else None,
                       "requestEvidence": calls,
                       "artifacts": summaries, "verdict": verdict,
                       "diagnosticTail": output.replace(str(root), "<isolated-audit>")}
                if surface.startswith("cli_"):
                    row["compilerInput"] = "direct_cli"
                    row["compilerSha256"] = provenance["compilerSha256"]
                elif surface == "skill_copy":
                    row["compilerInput"] = "skill_copy"
                    row["compilerSha256"] = provenance["skillCompilerSha256"]
                rows.append(row)
                print(json.dumps({k:row[k] for k in ["surface", "case", "exitCode", "localRequests", "verdict"]}), flush=True)
    assert digest(opts.compiler) == provenance["compilerSha256"], "direct CLI binary changed during audit"
    if opts.skill:
        assert digest(opts.skill_compiler) == provenance["skillCompilerSha256"], "Skill compiler changed during audit"
    report = {"formatVersion": 1, "observedAtUtc": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
              "scope": "ACTUAL_CLI_AND_ENDPOINT_INSTRUMENTED_WRAPPER_COPIES_LOOPBACK_ONLY",
              "appleCryptographicValidation": "NOT_TESTED", "privateCorpusUsed": False,
              "provenance": provenance, "results": rows,
              "summary": {"checks": len(rows), "controlsPassed": sum(x["verdict"] == "CONTROL_PASS" for x in rows),
                          "negativesRejected": sum(x["verdict"] == "REJECTED_NONZERO_NO_ARTIFACT" for x in rows),
                          "failures": sum(x["verdict"] == "FAIL" for x in rows)}}
    opts.report.parent.mkdir(parents=True, exist_ok=True)
    opts.report.write_text(json.dumps(report, indent=2) + "\n")
    print(json.dumps(report["summary"]))
    return 1 if report["summary"]["failures"] else 0


if __name__ == "__main__":
    raise SystemExit(main())
