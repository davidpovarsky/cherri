#!/usr/bin/env python3
import hashlib, json, os, pathlib, plistlib, subprocess, time

live = pathlib.Path("/home/chatgpt-agent/workspaces/cherri-remote")
stage = live / "jobs/p10r15-consumer-validation-r1-d00bc0b9"
candidate = stage / "remote-home"
def digest(p): return hashlib.sha256(pathlib.Path(p).read_bytes()).hexdigest()
receipt = json.loads((stage / "staging-receipt.json").read_text())
before = {p: digest(live / p) for p in receipt["liveHashesBefore"]}
assert before == receipt["liveHashesBefore"]
v2 = candidate / "jobs/surface-v2.cherri"
legacy = candidate / "jobs/surface-legacy.cherri"
v2.write_text('let message = "P10R15 inactive V2 surface"\nshow(message)\n')
legacy.write_text('@message = "P10R15 inactive Legacy surface"\nshow("{@message}")\n')
env = os.environ.copy()
env["CHERRI_REMOTE_HOME"] = str(candidate)
env["CHERRI_BIN"] = str(stage / "runtime/bin/cherri")
env["CHERRI_HOME"] = str(stage / "runtime")
env["CHERRI_DOCS_DIR"] = str(stage / "docs")
env["SKILL_DIR"] = str(stage / "skill")
runner = str(candidate / "bin/cherri-remote-runner.sh")
legacy_runner = str(candidate / "bin/cherri-remote-legacy.sh")
cases = [
    ("runner_status", runner, ["status"]),
    ("legacy_status", legacy_runner, ["status"]),
    ("runner_check", runner, ["check", str(v2)]),
    ("legacy_check", legacy_runner, ["check", str(legacy)]),
    ("runner_compile_unsigned", runner, ["compile", str(v2), "--unsigned"]),
    ("legacy_compile_unsigned", legacy_runner, ["compile", str(legacy), "--unsigned"]),
    ("runner_catalog", runner, ["catalog", "show"]),
    ("skill_full_catalog", str(stage / "skill/scripts/catalog.sh"), []),
    ("runner_actions", runner, ["actions", "show"]),
    ("runner_docs", runner, ["docs", "show"]),
    ("sync_validation_guard", runner, ["sync"]),
    ("runner_existing_tests", runner, ["test"]),
]
rows = []
for name, entry, arguments in cases:
    previous = {str(p) for p in (candidate / "outputs").glob("*.shortcut")}
    start = time.monotonic()
    try:
        result = subprocess.run(["bash", entry] + arguments, env=env,
            text=True, capture_output=True, timeout=300 if name == "runner_existing_tests" else 30)
        rc, output, err = result.returncode, result.stdout, result.stderr
    except subprocess.TimeoutExpired as exc:
        rc, output, err = None, "", "CANDIDATE_SURFACE_TIMEOUT"
    ok = rc == 0 and bool(output.strip() or err.strip())
    native = []
    if name == "sync_validation_guard":
        ok = rc not in [None, 0] and "R1 validation tuple is inactive" in err
    elif name.endswith("compile_unsigned"):
        emitted = [p for p in (candidate / "outputs").glob("*.shortcut") if str(p) not in previous]
        for p in emitted:
            parsed = plistlib.loads(p.read_bytes())
            actions = parsed.get("WFWorkflowActions", [])
            native.append({"file": p.name, "sha256": digest(p), "actions": len(actions),
                           "nativeActionIDs": sorted({a["WFWorkflowActionIdentifier"] for a in actions})})
        ok = ok and len(native) == 1 and native[0]["actions"] > 0 and "is.workflow.actions.showresult" in native[0]["nativeActionIDs"]
    elif name == "runner_catalog":
        # Filtered catalog is intentionally grep fragments, not a JSON document.
        ok = ok and "show" in output
    elif name == "skill_full_catalog":
        try: ok = ok and bool(json.loads(output))
        except Exception: ok = False
    elif name == "runner_docs":
        ok = ok and "actions.md" in output
    row = {"check": name, "exitCode": rc, "seconds": round(time.monotonic()-start,3),
           "verdict": "PASS" if ok else "FAIL", "nativeArtifacts": native,
           "stdoutSha256": hashlib.sha256(output.encode()).hexdigest(),
           "stdoutBytes": len(output.encode()),
           "diagnosticTail": (output + err)[-1200:]}
    rows.append(row)
    print(json.dumps({k: row[k] for k in ("check","exitCode","verdict","seconds")}), flush=True)
after = {p: digest(live / p) for p in before}
report = {"formatVersion":1, "scope":"INACTIVE_CANDIDATE_REAL_ENTRYPOINTS_SYNTHETIC_UNSIGNED",
    "observedAtUtc":time.strftime("%Y-%m-%dT%H:%M:%SZ",time.gmtime()),
    "compilerCommit":receipt["sourceCommit"],"docsCommit":receipt["docsCommit"],
    "compilerSHA256":digest(stage / "runtime/bin/cherri"),
    "livePromotion":False,"liveHashesBefore":before,"liveHashesAfter":after,
    "liveUnchanged":before==after,"results":rows,
    "pilotCorrection":"The first harness incorrectly parsed filtered catalog grep output as full JSON. The rerun checks filtered output and independently parses the unfiltered catalog.",
    "summary":{"checks":len(rows),"passed":sum(r["verdict"]=="PASS" for r in rows),
               "failed":sum(r["verdict"]!="PASS" for r in rows)}}
(stage / "candidate-surface-audit.json").write_text(json.dumps(report,indent=2)+"\n")
print(json.dumps(report["summary"]),flush=True)
raise SystemExit(0 if report["summary"]["failed"]==0 and before==after else 1)
