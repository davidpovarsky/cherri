#!/usr/bin/env python3
"""INACTIVE until root's explicit reviewed-intake handoff; no producer execution.

One new-source audit after all five original parents terminate. Reads/hashes the
same 38 manifest-bound inputs in place only after the explicit execution flag.
Never imports either historical auditor (they have R7-specific top-level reads).
"""
import collections
import datetime
import hashlib
import json
import os
from pathlib import Path
import re
import stat
import subprocess
import sys
import time

ROOT = Path("/mnt/development/projects/diagnostics/cherri-private-s4-20261123")
SOURCE_ROOT = Path("/mnt/development/projects/releases/cherri-p10r15-f01-compiler-source-20261123")
RUN_ROOT = ROOT / "s57b6d919-replay-01"
PROPOSAL_ROOT = ROOT / "proposal-s57b6d919-02"
SOURCE = "57b6d919757e1d0262993c6f11c6dc1cd17d6177"
TREE = "370d85f75d8fb5bba5f9c8afe438208d97a525c8"
SPEC_SHA = "474dcf801e6808fc7139db92224d7228ce6a75d651aa709ec7385fd71bb68eed"
MANIFEST_SHA = "766ac82ef7d92f50a894e50bc84d3fed9cc2d375f81ffe2771dbe8a5a6de5f39"
REPLAY_SHA = "2bffb8d5141862965f4ae9cc76bcd97561803cd68a469c506ee04674ef591561"
OWNER_SHA = "9487c34de113e89ff31841e9381c29687c4416b02dadb64e9c6fa993ff6f5053"
ORIGINAL_AUDITORS = {
    "audit-remaining-gate.py": "7b32b34cdbd558ab4c1be85effe203d35771eba675f49b9e73253603f81968c5",
    "audit-replay.py": "311d5ce382e446dd5ccd87df5e73d294defe88e243041914a247c27a71584d76",
}
TESTS = {
    "00-p12-par": ("TestP12_PAR_12_CorpusComplexParamsAndZeroStage3Gate", "p12_complex_params_test.go", "86172facc402af313872a2412e6dcd2d1dbd50f9f125d88a102acf21088db1ed"),
    "01-p13": ("TestP13_CorpusFidelityAndSourceSizeAudit", "p13_fidelity_promotion_test.go", "8163a5d80ef4ee23423fff2eef54599b5289c73e44337b02e006b2e47b9d6279"),
    "02-p10-measure": ("TestP10R10CorpusMeasure", "p10r10_corpus_test.go", "a6188a41ff23fd43f25308b8547391d9186d3c1b6ad92dbaea9ae514d865a761"),
    "03-p11-fresh38x2": ("TestP11_WIRE_01_Fresh38x2NativeActionWire", "p11_wire_fidelity_test.go", "c4ab56f1ee4f1eea58e4075da0965f525f85e8f1efc30452d2dc3703b34b8f67"),
    "04-p12-lifecycle": ("TestP12_BASE_02_LifecycleStagesAndReconcileCorpus", "p12_baseline_truth_test.go", "eb8b70716337cba945409df4d60edb23063c2998330bc15da8c1ec8e08c87658"),
}
MIN_FREE = 2 * 1024**3
CLAIMED = None


def require(value):
    if not value:
        raise ValueError("S4_INTAKE_REJECTED")


def sha(path):
    result = hashlib.sha256()
    with Path(path).open("rb") as handle:
        for chunk in iter(lambda: handle.read(1024 * 1024), b""):
            result.update(chunk)
    return result.hexdigest()


def data(path):
    require(stat.S_ISREG(Path(path).lstat().st_mode))
    return json.loads(Path(path).read_bytes())


def now():
    return datetime.datetime.now(datetime.timezone.utc).isoformat()


def put(path, value):
    raw = (json.dumps(value, indent=2) + "\n").encode()
    fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    with os.fdopen(fd, "wb") as handle:
        handle.write(raw)


def git(*args):
    # Source inspection only. No inherited routing overrides or replacement refs.
    env = {key: os.environ[key] for key in ("PATH", "HOME", "LANG", "LC_ALL") if key in os.environ}
    env.update({"GIT_NO_REPLACE_OBJECTS": "1", "GIT_OPTIONAL_LOCKS": "0", "GIT_TERMINAL_PROMPT": "0"})
    result = subprocess.run(
        ["git", "--no-replace-objects", "-C", str(SOURCE_ROOT), *args],
        stdin=subprocess.DEVNULL, capture_output=True, timeout=30, env=env,
    )
    require(result.returncode == 0)
    return result.stdout.decode().strip()


def source_guard():
    require(SOURCE_ROOT.resolve(strict=True) == SOURCE_ROOT)
    require((SOURCE_ROOT / ".git").is_dir() and not (SOURCE_ROOT / ".git").is_symlink())
    require(not (SOURCE_ROOT / ".git/objects/info/alternates").exists())
    current = {"source": git("rev-parse", "HEAD"), "tree": git("rev-parse", "HEAD^{tree}"),
               "clean": not git("status", "--porcelain"), "replaceRefsAbsent": not git("replace", "-l")}
    require(current["source"] == SOURCE and current["tree"] == TREE and
            current["clean"] and current["replaceRefsAbsent"] and
            Path(git("rev-parse", "--show-toplevel")) == SOURCE_ROOT)
    files = {}
    for _, name, expected in TESTS.values():
        require(sha(SOURCE_ROOT / name) == expected)
        original = subprocess.run(
            ["git", "--no-replace-objects", "-C", str(SOURCE_ROOT), "show", SOURCE + ":" + name],
            stdin=subprocess.DEVNULL, capture_output=True, check=True, timeout=30,
            env={"PATH": os.environ.get("PATH", "/usr/bin:/bin"), "GIT_NO_REPLACE_OBJECTS": "1",
                 "GIT_OPTIONAL_LOCKS": "0", "GIT_TERMINAL_PROMPT": "0"},
        ).stdout
        require(hashlib.sha256(original).hexdigest() == expected)
        files[name] = expected
    current["testSourceSHA256"] = files
    return current


def verify_spec():
    path = PROPOSAL_ROOT / "spec.execution-authorized.json"
    require(sha(path) == SPEC_SHA)
    spec = data(path)
    require(spec["executionApproved"] is True and spec["exclusiveGoOwnershipGranted"] is True)
    require(spec["source"] == SOURCE and spec["tree"] == TREE and spec["sourcePath"] == str(SOURCE_ROOT))
    require(spec["runDirectory"] == str(RUN_ROOT) and spec["retries"] == 0)
    require((spec["goBudgetSeconds"], spec["driverBudgetSeconds"], spec["fourParentOwnerBudgetSeconds"],
             spec["lifecycleOwnerBudgetSeconds"], spec["aggregateDeadlineSeconds"]) == (900, 930, 970, None, None))
    require(spec["privateManifestSHA256"] == MANIFEST_SHA and
            spec["privateInputCount"] == 38 and spec["privateInputBytes"] == 62980131 and
            spec["minimumFreeBytesForAdmission"] == MIN_FREE)
    require(spec["reviewedCodeSHA256"] == {"proposed_s4_owner.py": OWNER_SHA, "proposed_s4_replay.py": REPLAY_SHA})
    require(sha(PROPOSAL_ROOT / "proposed_s4_owner.py") == OWNER_SHA and
            sha(PROPOSAL_ROOT / "proposed_s4_replay.py") == REPLAY_SHA)
    for name, expected in ORIGINAL_AUDITORS.items():
        require(sha(ROOT / name) == expected)
    return spec


def verify_inputs_once():
    # Explicitly forbidden until the root-authorized --execute-reviewed-intake handoff.
    base = ROOT / "originals/r6-private-corpus-61614095-20261121"
    manifest_path = base / "private-input-manifest.json"
    require(sha(manifest_path) == MANIFEST_SHA)
    manifest = data(manifest_path)
    require(type(manifest) is list and len(manifest) == 38 and sum(row["bytes"] for row in manifest) == 62980131)
    names = [row["name"] for row in manifest]
    require(all(type(name) is str and Path(name).name == name and
                name not in ("", ".", "..") and "/" not in name and chr(92) not in name for name in names))
    require(len(set(names)) == 38)
    snapshot = base / "private-input"
    require({path.name for path in snapshot.iterdir()} == set(names))
    for row in manifest:
        path = snapshot / row["name"]
        before = path.lstat()
        require(stat.S_ISREG(before.st_mode) and before.st_size == row["bytes"] and sha(path) == row["sha256"])
        after = path.lstat()
        require((before.st_dev, before.st_ino, before.st_mode, before.st_size, before.st_mtime_ns) ==
                (after.st_dev, after.st_ino, after.st_mode, after.st_size, after.st_mtime_ns))
    require(sha(manifest_path) == MANIFEST_SHA)
    return manifest


# BEGIN EXACT FOUR-PARENT STRICT CORE (original lines19-50 and65-66).
def four_parent_core(mode, test, out, manifest, binding):
    raw=(out/"stdout.jsonl").read_bytes();rows=[];parse_errors=0
    for line in raw.splitlines():
     if not line.strip():continue
     try:rows.append(json.loads(line))
     except (ValueError,UnicodeDecodeError):parse_errors+=1
    starts=collections.Counter((x.get("Package"),x["Test"]) for x in rows if x.get("Action")=="run" and x.get("Test"))
    ends=collections.Counter((x.get("Package"),x["Test"]) for x in rows if x.get("Action") in ["pass","fail","skip"] and x.get("Test"))
    counts=collections.Counter(x["Action"] for x in rows if x.get("Action") in ["pass","fail","skip"] and x.get("Test"))
    own=[x for x in rows if x.get("Test")==test];parent_runs=sum(x.get("Action")=="run" for x in own);parent_passes=[x for x in own if x.get("Action")=="pass"]
    packages=[{"package":x.get("Package"),"action":x["Action"],"elapsed":x.get("Elapsed")} for x in rows if x.get("Action") in ["pass","fail","skip"] and not x.get("Test")]
    outputs=[x.get("Output","") for x in rows];text="".join(outputs)
    patterns={
    "00-p12-par":r"P12_PAR_12 corpus verified: files=(\d+) actions=(\d+) natural=(\d+) fallback=(\d+) invalid=(\d+) demotions=(\d+) thirdParty=(\d+)",
    "01-p13":r"P10R13 Corpus Summary: total=(\d+) natural=(\d+) \([^\n)]*\) fallback=(\d+) invalid=(\d+) demoted=(\d+)",
    "02-p10-measure":r"TOTAL actions=(\d+) natural=(\d+) \([^\n)]*\) reasons=",
    "03-p11-fresh38x2":r"P11_WIRE_01 checked (\d+) files, (\d+) total actions, (\d+) NATURAL_VERIFIED"}
    matches=list(re.finditer(patterns[mode],text));summary={};summary_ok=False;per_input_count=None
    if len(matches)==1:
     nums=[int(x) for x in matches[0].groups()]
     if mode=="00-p12-par":
      summary=dict(zip(["files","actions","natural","fallback","invalid","demotions","thirdParty"],nums))
      summary_ok=nums[:6]==[38,27652,23353,4298,1,0] and nums[6]>0
     elif mode=="01-p13":
      summary=dict(zip(["actions","natural","fallback","invalid","demotions"],nums));summary_ok=nums==[27652,23353,4298,1,0]
     elif mode=="02-p10-measure":
      summary=dict(zip(["actions","natural"],nums))
      per=list(re.finditer(r"(?m)^(\d{3}) actions=\d+ natural=\d+ \([0-9.]+%\) OK \(\d+ms\)\s*$",text))
      per_input_count=len(per)
      summary_ok=nums==[27652,23353] and len(per)==38 and sorted(x[1]+".plist" for x in per)==sorted(x["name"] for x in manifest)
     elif mode=="03-p11-fresh38x2":
      summary=dict(zip(["files","actions","natural"],nums));summary_ok=nums==[38,27652,23353]
    events_ok=(parent_runs==len(parent_passes)==1 and not parse_errors and len(starts)==1 and set(starts)==set(ends) and all(n==1 for n in starts.values()) and all(n==1 for n in ends.values()) and counts.get("pass",0)==1 and not counts.get("fail",0) and not counts.get("skip",0) and len(packages)==1 and packages[0]["action"]=="pass")
    go_timeout="panic: test timed out after" in text
    not_run=bool(re.search(r"(?m)^\s*"+re.escape(binding["file"])+r":\d+: NOT_RUN_PRIVATE_CORPUS: set CHERRI_P10R10_CORPUS=1",text))
    return {
        "eventCount": len(rows), "parseErrorCount": parse_errors, "startedCaseCount": len(starts),
        "terminalCounts": dict(counts), "incompleteCaseCount": len(set(starts)-set(ends)),
        "orphanTerminalCount": len(set(ends)-set(starts)), "packageTerminals": packages,
        "parentRunCount": parent_runs, "parentPassCount": len(parent_passes),
        "parentElapsedSeconds": parent_passes[0].get("Elapsed") if parent_passes else None,
        "summaryCount": len(matches), "summary": summary, "perInputSummaryCount": per_input_count,
        "summaryVerified": summary_ok, "eventsVerified": events_ok,
        "gatedNotRun": not_run, "goTimedOut": go_timeout,
    }
# END EXACT FOUR-PARENT STRICT CORE.


# BEGIN EXACT LIFECYCLE STRICT CORE (original lines15-35 and45-46).
def lifecycle_core(out):
    raw=(out/"stdout.jsonl").read_bytes()
    rows=[];parse_errors=0
    for line in raw.splitlines():
     if not line.strip():continue
     try:rows.append(json.loads(line))
     except (ValueError,UnicodeDecodeError):parse_errors+=1
    test="TestP12_BASE_02_LifecycleStagesAndReconcileCorpus"
    starts=collections.Counter((x.get("Package"),x["Test"]) for x in rows if x.get("Action")=="run" and x.get("Test"))
    ends=collections.Counter((x.get("Package"),x["Test"]) for x in rows if x.get("Action") in ["pass","fail","skip"] and x.get("Test"))
    parent_runs=[x for x in rows if x.get("Action")=="run" and x.get("Test")==test]
    parent_passes=[x for x in rows if x.get("Action")=="pass" and x.get("Test")==test]
    counts=collections.Counter(x.get("Action") for x in rows if x.get("Action") in ["pass","fail","skip"] and x.get("Test"))
    package_ends=[{"package":x.get("Package"),"action":x["Action"],"elapsed":x.get("Elapsed")} for x in rows if x.get("Action") in ["pass","fail","skip"] and not x.get("Test")]
    outputs=[x.get("Output","") for x in rows]
    matches=[]
    pattern=re.compile(r"P12_BASE_02 corpus reconciliation: files=(\d+) freshNat=(\d+) seqNat=(\d+) diffs=\[([^\]]*)\]")
    for line in outputs:
     for m in pattern.finditer(line):matches.append({"files":int(m[1]),"freshNatural":int(m[2]),"sequentialNatural":int(m[3]),"diffsEmpty":not m[4].strip()})
    demotion_summaries=sum("P12_BASE_02 demotionStages=" in line and "demotionActionReasons=" in line for line in outputs)
    not_run=any("P12_BASE_02 corpus: NOT_RUN" in line for line in outputs)
    timed_out=any("panic: test timed out after" in line for line in outputs)
    corpus_complete=len(matches)==1 and matches[0]["files"]==38 and matches[0]["freshNatural"]==matches[0]["sequentialNatural"] and matches[0]["diffsEmpty"] and demotion_summaries==1 and not not_run
    events_ok=(len(parent_runs)==len(parent_passes)==1 and not parse_errors and set(starts)==set(ends) and all(n==1 for n in starts.values()) and all(n==1 for n in ends.values()) and not counts.get("fail",0) and not counts.get("skip",0) and len(package_ends)==1 and package_ends[0]["action"]=="pass")
    return {
        "eventCount": len(rows), "parseErrorCount": parse_errors, "startedCaseCount": len(starts),
        "terminalCounts": dict(counts), "incompleteCaseCount": len(set(starts)-set(ends)),
        "orphanTerminalCount": len(set(ends)-set(starts)), "packageTerminals": package_ends,
        "parentRunCount": len(parent_runs), "parentPassCount": len(parent_passes),
        "parentElapsedSeconds": parent_passes[0].get("Elapsed") if parent_passes else None,
        "corpusReconciliation": matches, "demotionSummaryCount": demotion_summaries,
        "summaryVerified": corpus_complete, "eventsVerified": events_ok,
        "gatedNotRun": not_run, "goTimedOut": timed_out,
    }
# END EXACT LIFECYCLE STRICT CORE.


def verified_owner(mode, test, receipt):
    if mode == "04-p12-lifecycle":
        require(not (RUN_ROOT / "parent-owners" / mode).exists())
        return {"route": "ORIGINAL_DIRECT_930", "ownerBudgetSeconds": None,
                "ownerReceiptSHA256": None, "ownerVerified": True}
    folder = RUN_ROOT / "parent-owners" / mode
    owner = data(folder / "receipt.json")
    started = data(folder / "started.json")
    require(all(owner.get(key) == value for key, value in started.items()))
    require(owner["source"] == SOURCE and owner["tree"] == TREE and owner["specSHA256"] == SPEC_SHA)
    require(owner["scope"] == "ONE_PARENT_ONLY" and owner["mode"] == mode and owner["test"] == test)
    require((owner["outerOwnerBudgetSeconds"], owner["goBudgetSeconds"], owner["driverBudgetSeconds"]) == (970, 900, 930))
    require(owner["lifecycleSelected"] is False and owner["exitCode"] == 0 and owner["timedOut"] is False)
    require(owner["originalReceiptExists"] is True and 0 < owner["durationSeconds"] < 970)
    require(owner["minimumFreeBytesForAdmission"] == MIN_FREE and
            owner["capacityBefore"]["diskFreeBytes"] >= MIN_FREE)
    require(sha(folder / "driver.stdout.bin") == owner["driverStdoutSHA256"] and
            sha(folder / "driver.stderr.bin") == owner["driverStderrSHA256"])
    # Bind the actual owner's child output to the independently retained producer receipt.
    require(json.loads((folder / "driver.stdout.bin").read_bytes()) == receipt)
    return {"route": "ORIGINAL_FOUR_PARENT_970_OWNER", "ownerBudgetSeconds": 970,
            "ownerReceiptSHA256": sha(folder / "receipt.json"),
            "ownerStartedSHA256": sha(folder / "started.json"),
            "ownerDurationSeconds": owner["durationSeconds"],
            "ownerExitCode": owner["exitCode"], "ownerTimedOut": owner["timedOut"],
            "driverStdoutSHA256": owner["driverStdoutSHA256"],
            "driverStderrSHA256": owner["driverStderrSHA256"], "ownerVerified": True}


def audit_parent(mode, manifest):
    test, filename, source_sha = TESTS[mode]
    out = RUN_ROOT / mode
    receipt = data(out / "receipt.json")
    started = data(out / "started.json")
    require(all(receipt.get(key) == value for key, value in started.items()))
    command = ["go", "test", "-count=1", "-json", "-timeout=15m", "-run", "^" + test + "$", "."]
    require(receipt["source"] == SOURCE and receipt["tree"] == TREE and receipt["specSHA256"] == SPEC_SHA)
    require(receipt["command"] == command and receipt["test"] == test and receipt["outerGuardSeconds"] == 930)
    require(receipt["compileOnly"] is False and receipt["inputManifestSHA256"] == MANIFEST_SHA)
    require(receipt["sourceCleanBefore"] is True and receipt["sourceCleanAfter"] is True and
            receipt["headAfter"] == SOURCE and receipt["treeAfter"] == TREE)
    require(receipt["capacityFreeBefore"] >= MIN_FREE and receipt["inputsUnchangedAfter"] is True)
    require(receipt["privateOriginalsPublished"] is False and
            receipt["independentS4IntakeRequired"] is True and receipt["aggregateAcceptanceGranted"] is False)
    require(sha(out / "stdout.jsonl") == receipt["stdoutSHA256"] and
            sha(out / "stderr.bin") == receipt["stderrSHA256"] and
            sha(out / "submitted-driver.py") == REPLAY_SHA)
    require(receipt["exitCode"] == 0 and receipt["timedOut"] is False and
            0 < receipt["durationSeconds"] < 930 and receipt["gatePassed"] is True)
    core = lifecycle_core(out) if mode == "04-p12-lifecycle" else four_parent_core(
        mode, test, out, manifest, {"file": filename})
    owner = verified_owner(mode, test, receipt)
    accepted = bool(core["eventsVerified"] and core["summaryVerified"] and
                    not core["gatedNotRun"] and not core["goTimedOut"])
    return {
        "mode": mode, "test": test, "source": SOURCE, "tree": TREE,
        "testSourceSHA256": source_sha, "command": command, "goTimeoutSeconds": 900,
        "driverTimeoutSeconds": 930, "receiptSHA256": sha(out / "receipt.json"),
        "startedSHA256": sha(out / "started.json"), "submittedDriverSHA256": REPLAY_SHA,
        "stdoutSHA256": receipt["stdoutSHA256"], "stderrSHA256": receipt["stderrSHA256"],
        "startedUTC": receipt["startedUTC"], "finishedUTC": receipt["finishedUTC"],
        "driverDurationSeconds": receipt["durationSeconds"], "exitCode": receipt["exitCode"],
        "supervisorTimedOut": receipt["timedOut"], "driverGatePassed": receipt["gatePassed"],
        "currentStreamsVerified": True, "exactContractVerified": True,
        "sourceAndInputProducerGuardsVerified": True,
        **core, **owner, "newSourceParentAccepted": accepted,
    }


def main():
    global CLAIMED
    if sys.argv[1:] != ["--execute-reviewed-intake"]:
        print("S4_INTAKE_INACTIVE", file=sys.stderr)
        return 64
    require(os.getuid() == 1001)
    # A single complete audit; never watch or poll an unfinished producer.
    require(all((RUN_ROOT / mode / "receipt.json").is_file() for mode in TESTS))
    verify_spec()
    before = source_guard()
    target = RUN_ROOT / "independent-intake-01"
    target.mkdir(mode=0o700, exist_ok=False)
    CLAIMED = target
    began = time.monotonic()
    put(target / "claim.original.json", {
        "schema": "cherri-private-s4-independent-intake-claim/2", "source": SOURCE, "tree": TREE,
        "startedUTC": now(), "pid": os.getpid(), "auditorSHA256": sha(Path(__file__)),
        "producerExecutionCount": 0, "reviewedExecutionFlag": True,
    })
    manifest = verify_inputs_once()
    parents = [audit_parent(mode, manifest) for mode in TESTS]
    after = source_guard()
    require(after == before)
    accepted = all(row["newSourceParentAccepted"] for row in parents)
    result = {
        "schema": "cherri-private-s4-new-source-independent-intake/2",
        "source": SOURCE, "tree": TREE, "auditedUTC": now(),
        "auditDurationSeconds": round(time.monotonic() - began, 6),
        "auditorSHA256": sha(Path(__file__)), "originalAuditorsSHA256": ORIGINAL_AUDITORS,
        "approvedSpecSHA256": SPEC_SHA, "reviewedReplaySHA256": REPLAY_SHA, "reviewedOwnerSHA256": OWNER_SHA,
        "configuredGoToolchain": "go1.24.13", "configuredToolchainEvidence": "Exact reviewed replay source",
        "independentlyInvokedGoVersion": False,
        "goTimeoutSeconds": 900, "driverTimeoutSeconds": 930,
        "fourParentOwnerTimeoutSeconds": 970, "lifecycleOwnerTimeoutSeconds": None,
        "privateInputCount": 38, "privateInputBytes": 62980131, "expectedPrivateActionCount": 27652,
        "privateInputManifestSHA256": MANIFEST_SHA, "exactInventoryAndHashesVerified": True,
        "privateInputSemanticContentParsed": False, "privateNamesPublished": False,
        "inputHashPassCount": 1, "sourceGuardBefore": before, "sourceGuardAfter": after,
        "historicalSummaryExpectationsPreserved": True, "strictEventControlsPreserved": True,
        "r7ExecutionCreditReused": False, "compilePreparationReused": False,
        "compileOnlyReceiptRequired": False,
        "compilationEvidence": "Five actual go test executions at clean S; no separate precompile credit",
        "historicalArchiveReverified": False,
        "historicalArchiveNote": "Prior preservation receipts remain historical; this intake does not reuse their source or acceptance.",
        "parentCount": len(parents), "parents": parents,
        "allFiveNewSourceParentsAccepted": accepted, "producerExecutionsByThisAuditor": 0,
        "aggregateFormal179AcceptanceGranted": False,
    }
    put(target / "result.original.json", result)
    print(json.dumps({
        "schema": result["schema"], "source": SOURCE, "tree": TREE, "parentCount": len(parents),
        "allFiveNewSourceParentsAccepted": accepted,
        "resultSHA256": sha(target / "result.original.json"),
        "privateInputCount": 38, "privateInputBytes": 62980131,
        "privateInputManifestSHA256": MANIFEST_SHA,
        "producerExecutionsByThisAuditor": 0, "privateNamesPublished": False,
    }, sort_keys=True))
    return 0 if accepted else 1


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except Exception:
        if CLAIMED is not None:
            put(CLAIMED / "rejection.original.json", {
                "schema": "cherri-private-s4-independent-intake-rejection/2",
                "source": SOURCE, "tree": TREE, "utc": now(), "status": "S4_INTAKE_REJECTED",
                "producerExecutionsByThisAuditor": 0, "privateValuesPublished": False,
            })
        print("S4_INTAKE_REJECTED", file=sys.stderr)
        raise SystemExit(1)
