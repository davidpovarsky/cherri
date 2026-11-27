"""First execution of the 23 original8333 parents unreached by the failed full suite.
Every selected child event and full output byte is retained; no native acceptance.
"""
from pathlib import Path
import base64
import hashlib
import importlib.util
import json
import os
import platform
import re
import shutil
import stat
import sys
import traceback

sys.dont_write_bytecode = True
SOURCE = "8333cde1d492f1b72faed5f3f5e92cf1d7f9d0c6"
TREE = "aafd356ec20d08a7e8bfc15a36071aeeb8d02d83"
PARENT = "55c892ee9ea71a38e49e2e6ce3a8c6215b2f3ea0"
BRANCH = "probe/cherri-p10r15-no-content-focused-8333cde-20261127"
HELPER_SHA = "4672474ddb270a5857914041ebb279f20431f87e9da173e6cd8bf83b3fb11c85"
PREFIX = "tools/ci/distinct_functions/"
CONTROLLER_FILES = [".github/workflows/p10r15-distinct-functions.yml", PREFIX+"run.py", PREFIX+"owned_source_process.py"]
PACKAGE = "github.com/electrikmilk/cherri"
EXPECTED_PACKAGES = (PACKAGE,)
NO_TEST_PACKAGES = frozenset()
REQUIRED_PARENTS = tuple(["TestBackendRecovery14ValueBlockReturnNative","TestBackendRecovery14ValueBlockReturnUnit","TestP15F04ValueLoopBoundaryNative","TestP15F04ValueLoopBoundaryUnit","TestP15M02CompleteGraphCausalControls","TestP15M02CompleteMainGraphs","TestP15M02ExactNativeWorkflowBoundary","TestP15M02ExplicitMainReturnBaseline","TestP15M02MainVoidFallthrough","TestP15M02MigratedEntryOutput","TestP15NoneBindingsClearPreviousOutput","TestP15NoneClearOracleRejectsCorruption","TestP15NoneClearingPreservesPresentEmptyValues","TestP15NoneReturnClearsPreviousOutput","TestP15OptionalCallResultPredicateEvaluatesOnce","TestP15OptionalNonePredicateOracleRejectsCorruption","TestP15OptionalNonePredicatesUsePresence","TestP15OptionalReturnCompleteness","TestP15RP23ValueBlockLoopNative","TestP15RP23ValueBlockLoopUnit","TestP15RP26CallerInputVoidNative","TestP15RP26PartialMarkerInputBoundaries","TestP15TaggedNoneResultRejectsStaleAndInventedValues"])
REGRESSION_COMMAND = ("go", "test", "-count=1", "-json", "-timeout=3m", ".", "-run", "^("+"|".join(REQUIRED_PARENTS)+")$")
SOURCE_PINS = {
    "internal/language/lower/lower.go": "4a28c73289d811282aac7be7bdea5d06f50e51486d951ea685f4064fe77c9eef",
    "internal/language/lower/main_output.go": "41721975525ff359d3880c06d796aff98663c1b91f42e050fcb8625f50350921",
    "p15_combined_value_blocks_test.go": "ed6f4ba43d546807e2ab9558163309a89cc30debd1f760611eb9a0bb0c64f112",
    "p15_function_optional_predicate_test.go": "3023245cabdc0088d962fc6dfae174c4a7a024e2df0697d83b0b71eb437c1f75",
    "p15_main_output_graph_test.go": "af6dfa89af76970505122e47ee09a16851d49de3559ec441507aae90355bf0d3",
    "p15_main_void_return_test.go": "bb493466658026ce3378ae2d442daca96e9d261468210c3c2c886b49c91cab0f",
    "p15_none_clear_boundary_test.go": "2312222a2495c673d6053182a19c35c719bbcb3404c7fb7b3631457bd8bef0f6",
    "p15_rp26_caller_input_void_test.go": "982b9e94e4870fd40b08f378ba3b4a8e48f195259b449d0efe6a391069d9e363"
}



def require(condition, message):
    if not condition:
        raise RuntimeError(message)


def sha(raw):
    return hashlib.sha256(raw).hexdigest()


def validate_go(raw, packages, required):
    events = [json.loads(line) for line in raw.decode("utf-8").splitlines() if line.strip()]
    require(events and all(type(e) is dict for e in events), "ORIGINAL_GO_EVENTS")
    require(not any(e.get("Action") in ("fail", "skip") for e in events), "FAILED_OR_SKIPPED_GO_EVENT")
    passed_packages = [e["Package"] for e in events if e.get("Action") == "pass" and not e.get("Test")]
    require(sorted(passed_packages) == sorted(packages), "EXACT_COMPLETE_PACKAGE_TERMINALS")
    passed = [(e["Package"], e["Test"]) for e in events if e.get("Action") == "pass" and e.get("Test")]
    require(len(passed) == len(set(passed)), "DUPLICATE_NAMED_TERMINAL")
    for package, name in required:
        require(passed.count((package, name)) == 1, "MISSING_REQUIRED_PARENT:"+name)
    return {"packages": passed_packages, "namedPasses": len(passed), "requiredParents": required,
            "failed": 0, "skipped": 0, "originalStreamSHA256": sha(raw)}

def inspect_regression_stream(path, packages, no_test_packages, required, tick=lambda: None):
    """Read the entire authentic Go 1.24 JSONL without filtering or truncation.

    Ordinary original test skips remain visible. Required affected parents must
    pass; a package with no test files is distinct from a skipped named test.
    """
    path = Path(path)
    before = path.lstat()
    require(stat.S_ISREG(before.st_mode), "REGULAR_ORIGINAL_GO_STREAM")
    allowed = set(packages)
    require(len(allowed) == len(packages) and set(no_test_packages) <= allowed,
            "CLOSED_ORIGINAL_PACKAGE_SET")
    require(len(required) == len(set(required)) and all(p in allowed for p, _ in required),
            "CLOSED_AFFECTED_PARENT_SET")
    starts, package_terminals, named, builds = {}, {}, {}, {}
    output_events = 0
    output_bytes = 0
    digest = hashlib.sha256()
    size = lines = 0
    def unique(pairs):
        result = {}
        for key, value in pairs:
            require(key not in result, "DUPLICATE_JSON_KEY")
            result[key] = value
        return result
    with path.open("rb") as stream:
        for raw in stream:
            tick()
            size += len(raw)
            digest.update(raw)
            require(raw.endswith(b"\n") and raw.strip(), "COMPLETE_NONEMPTY_JSON_LINE")
            event = json.loads(raw.decode("utf-8"), object_pairs_hook=unique)
            lines += 1
            require(type(event) is dict and type(event.get("Action")) is str, "GO_EVENT_OBJECT")
            action = event["Action"]
            if action in ("build-output", "build-fail"):
                identity = event.get("ImportPath")
                require(type(identity) is str and identity and not event.get("Test"),
                        "GO_124_BUILD_IDENTITY")
                state = builds.setdefault(identity, {"outputEvents": 0, "failed": False})
                if action == "build-output":
                    require(type(event.get("Output")) is str, "ORIGINAL_BUILD_OUTPUT")
                    state["outputEvents"] += 1
                else:
                    require(not state["failed"], "DUPLICATE_BUILD_FAILURE")
                    state["failed"] = True
                continue
            require(action in ("start", "run", "pause", "cont", "pass", "fail", "skip", "output"),
                    "KNOWN_TEST_EVENT")
            package = event.get("Package")
            require(package in allowed, "UNEXPECTED_TEST_PACKAGE")
            name = event.get("Test")
            require(name is None or (type(name) is str and bool(name)), "NAMED_TEST_IDENTITY")
            if "Elapsed" in event:
                value = event["Elapsed"]
                require(type(value) in (int, float) and value == value and
                        value >= 0 and value < float("inf"), "FINITE_EVENT_ELAPSED")
            if action == "output":
                require(type(event.get("Output")) is str, "ORIGINAL_TEST_OUTPUT")
                output_events += 1
                output_bytes += len(event["Output"].encode("utf-8"))
                continue
            if action == "start":
                require(name is None and package not in starts and package not in package_terminals,
                        "ONE_PACKAGE_START")
                starts[package] = lines
                continue
            require(package in starts and package not in package_terminals,
                    "PACKAGE_LIFETIME")
            if name is None:
                require(action in ("pass", "fail", "skip"), "PACKAGE_TERMINAL_ACTION")
                package_terminals[package] = action
                continue
            key = (package, name)
            if action == "run":
                require(key not in named, "ONE_NAMED_START")
                named[key] = {"runLine": lines, "terminal": None, "paused": False}
            elif action in ("pause", "cont"):
                require(key in named and named[key]["terminal"] is None, "RUNNING_NAMED_EVENT")
                require(named[key]["paused"] is (action == "cont"), "PAUSE_CONT_ORDER")
                named[key]["paused"] = action == "pause"
            else:
                require(action in ("pass", "fail", "skip") and key in named and
                        named[key]["terminal"] is None, "ONE_NAMED_TERMINAL")
                named[key]["terminal"] = action
                named[key]["terminalLine"] = lines
    after = path.lstat()
    require(all(getattr(before, key) == getattr(after, key) for key in
                ("st_dev", "st_ino", "st_mode", "st_uid", "st_gid", "st_size", "st_mtime_ns", "st_ctime_ns")),
            "UNCHANGED_COMPLETE_ORIGINAL_STREAM")
    require(lines > 0 and size == before.st_size, "ENTIRE_ORIGINAL_STREAM_READ")
    missing_packages = sorted(allowed - set(package_terminals))
    invalid_packages = {p: status for p, status in package_terminals.items()
                        if status != "pass" and not (status == "skip" and p in no_test_packages)}
    incomplete = sorted(key for key, value in named.items() if value["terminal"] is None)
    failed = sorted(key for key, value in named.items() if value["terminal"] == "fail")
    skipped = sorted(key for key, value in named.items() if value["terminal"] == "skip")
    mandatory_missing = sorted(key for key in required
                               if key not in named or named[key]["terminal"] != "pass")
    build_failures = sorted(key for key, value in builds.items() if value["failed"])
    complete = (set(starts) == allowed and not missing_packages and not invalid_packages and
                not incomplete and not failed and not mandatory_missing and not build_failures)
    return {"schema": "cherri-complete-go-regression-stream/1", "passed": complete,
            "packageTerminals": package_terminals, "expectedPackages": sorted(allowed),
            "noTestFilePackages": sorted(no_test_packages), "missingPackages": missing_packages,
            "nonpassingPackages": invalid_packages, "requiredParents": list(required),
            "requiredParentsNotPassed": mandatory_missing, "namedRuns": len(named),
            "namedPasses": sum(v["terminal"] == "pass" for v in named.values()),
            "namedFailures": failed, "namedSkips": skipped, "incompleteNamedTests": incomplete,
            "buildEvents": builds, "buildFailures": build_failures,
            "eventCount": lines, "testOutputEvents": output_events,
            "testOutputUTF8Bytes": output_bytes, "originalStreamBytes": size,
            "originalStreamSHA256": digest.hexdigest(),
            "namedTerminals": [{"package": p, "test": n, **value}
                               for (p, n), value in sorted(named.items())]}


def main():
    require(len(sys.argv) == 1 and platform.system() == "Linux" and platform.machine() in ("aarch64", "arm64"),
            "FIXED_NATIVE_LINUX_ARM64_ROUTE")
    environ = dict(os.environ)
    event = json.loads(Path(environ["GITHUB_EVENT_PATH"]).read_bytes())
    require(environ["GITHUB_REPOSITORY"] == "davidpovarsky/cherri" and
            environ["GITHUB_EVENT_NAME"] == "create" and event.get("ref_type") == "branch" and
            event.get("ref") == BRANCH and environ["GITHUB_REF"] == "refs/heads/"+BRANCH and
            environ["GITHUB_RUN_ATTEMPT"] == "1", "FIRST_EXACT_CREATE_EVENT")
    controller_sha = environ["GITHUB_SHA"]
    require(re.fullmatch("[0-9a-f]{40}", controller_sha) is not None, "CONTROLLER_SHA")
    workspace = Path(environ["GITHUB_WORKSPACE"]).resolve(strict=True)
    controller, source = workspace/"controller", workspace/"source"
    for directory in (controller, source):
        require(directory.resolve(strict=True) == directory and (directory/".git").is_dir() and
                not (directory/".git/objects/info/alternates").exists() and
                not (directory/".git/info/grafts").exists(), "ORDINARY_SEPARATE_SOURCE")
    helper = controller/(PREFIX+"owned_source_process.py")
    require(sha(helper.read_bytes()) == HELPER_SHA, "UNCHANGED_QUALIFIED_PROCESS_OWNER")
    spec = importlib.util.spec_from_file_location("distinct_function_owned_process", helper)
    owned = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(owned)
    output = Path(environ["RUNNER_TEMP"]).resolve(strict=True)/"cherri-no-content-focused-8333"
    output.mkdir(mode=0o700)
    commands = output/"commands"
    commands.mkdir(mode=0o700)
    temporary = output.parent/"cherri-no-content-focused-tmp-8333"
    temporary.mkdir(mode=0o700)
    env = {key: environ[key] for key in ("PATH", "HOME", "LANG", "LC_ALL", "LC_CTYPE", "TZ", "GOCACHE", "GOMODCACHE") if key in environ}
    env.update(PYTHONDONTWRITEBYTECODE="1", GOMAXPROCS="1", GOFLAGS="-p=1 -mod=readonly", GOMEMLIMIT="1GiB",
               GOTOOLCHAIN="local", GOENV="off", GOWORK="off", GOTELEMETRY="off",
               GIT_NO_REPLACE_OBJECTS="1", GIT_OPTIONAL_LOCKS="0", GIT_TERMINAL_PROMPT="0",
               TMPDIR=str(temporary), GOTMPDIR=str(temporary), CGO_ENABLED="1")
    processes = []
    minimum = {"disk": shutil.disk_usage(output).free, "memory": 1 << 62}
    result = {"schema":"cherri-no-content-focused-original-parents/1", "status":"FAILED",
              "source":SOURCE, "tree":TREE, "sourceParent":PARENT, "controller":controller_sha,
              "runId":int(environ["GITHUB_RUN_ID"]), "attempt":1, "event":"create", "branch":BRANCH,
              "host":{"system":platform.system(), "machine":platform.machine(), "release":platform.release(),
                      "python":platform.python_version()}, "runner":"ubuntu-24.04-arm",
              "wholeOwnerSeconds":900, "goTestInternalSeconds":180,
              "compilationInclusiveChildSeconds":600, "processes":processes, "goTests":[],
              "formalAcceptanceGranted":False, "rawNativeTypesObserved":False,
              "full179Executed":False, "nativeAppleExecuted":False, "providerInvoked":False,
              "privateCorpusPresent":False, "releasePromoted":False, "startedAt":owned.utc()}
    owned.original(output/"github-event.original.json", Path(environ["GITHUB_EVENT_PATH"]).read_bytes())
    with owned.WholeBudget(900, cleanup_seconds=8, final_reserve=60) as budget:
        def monitor():
            mem = {k:int(v.strip().split()[0])*1024 for k,v in
                   (line.split(":",1) for line in Path("/proc/meminfo").read_text().splitlines())}
            observed = {"disk":shutil.disk_usage(output).free,"memory":mem["MemAvailable"]}
            for key in minimum:
                minimum[key] = min(minimum[key],observed[key])
            require(observed["disk"] >= 2*1024**3 and observed["memory"] >= 1024**3, "HOSTED_LIVE_CAPACITY")
        def run(name, argv, seconds=20, finalizing=False, override=None):
            actual_name = "%03d-%s"%(len(processes)+1,name)
            row = owned.run_owned(argv, cwd=source, env={**env, **(override or {})},
                                  output=commands, name=actual_name, budget=budget,
                                  seconds=seconds, monitor=monitor,
                                  post_reserve=2 if finalizing else None)
            processes.append(row)
            owned.require_success(row)
            require(row["cleanup"]["parentReaped"] and row["cleanup"]["after"]["state"] == "ABSENT",
                    "OWNED_GROUP_REAPED_AND_ABSENT")
            return (commands/(actual_name+".stdout.original")).read_bytes()
        def git(directory,name,*args,finalizing=False):
            return run(name,["git","--no-replace-objects","-C",str(directory),*args],20,finalizing)
        def stable(path):
            before=path.lstat()
            require(stat.S_ISREG(before.st_mode) and before.st_size <= 128*1024**2, "REGULAR_BOUNDED_INPUT")
            digest=hashlib.sha256()
            blob=hashlib.sha1(("blob %d\0"%before.st_size).encode())
            with path.open("rb") as stream:
                for chunk in iter(lambda:stream.read(1024**2),b""):
                    budget.check()
                    digest.update(chunk);blob.update(chunk)
            after=path.lstat()
            require(all(getattr(before,k)==getattr(after,k) for k in
                        ("st_dev","st_ino","st_mode","st_uid","st_gid","st_size","st_mtime_ns","st_ctime_ns")),
                    "FILE_CHANGED_DURING_READ")
            return {"bytes":before.st_size,"sha256":digest.hexdigest(),"blob":blob.hexdigest(),
                    "mode":stat.S_IMODE(before.st_mode)}
        def snapshot(directory,head,tree=None,selected=None,finalizing=False):
            require(not git(directory,"replace","replace","-l",finalizing=finalizing), "NO_REPLACE_REFS")
            actual=git(directory,"identity","rev-parse","HEAD","HEAD^{tree}",finalizing=finalizing).decode().splitlines()
            require(actual[0]==head and (tree is None or actual[1]==tree), "EXACT_SOURCE_IDENTITY")
            require(not git(directory,"status","status","--porcelain=v1","--untracked-files=all",finalizing=finalizing),
                    "CLEAN_SOURCE")
            entries=git(directory,"tree","ls-tree","-rz","--full-tree","HEAD",
                        *(["--",*selected] if selected else []),finalizing=finalizing)
            files={}
            for entry in entries.split(b"\0"):
                if not entry:continue
                metadata,path=entry.split(b"\t",1);mode,kind,blob=metadata.decode().split()
                path=path.decode()
                require(mode in ("100644","100755") and kind=="blob" and
                        not path.startswith("/") and all(part not in ("",".","..",".git") for part in path.split("/"))
                        and path not in files,"CLOSED_TRACKED_PATH")
                observed=stable(directory/path)
                require(observed["blob"]==blob and (observed["mode"]&0o111)==(int(mode,8)&0o111),
                        "PHYSICAL_SOURCE_MATCHES_GIT")
                files[path]=observed
            require(files and (selected is None or set(files)==set(selected)),"COMPLETE_INPUT_INVENTORY")
            return {"source":actual[0],"tree":actual[1],"files":files}
        before=controller_before=None
        try:
            monitor()
            before=snapshot(source,SOURCE,TREE)
            require(len(before["files"])==7909,"EXPECTED_SOURCE_FILE_COUNT")
            controller_before=snapshot(controller,controller_sha,selected=CONTROLLER_FILES)
            require(git(source,"source-parent","rev-parse","HEAD^").decode().strip()==PARENT,"SOURCE_PARENT")
            owned.original(output/"source-before.original.json",before)
            owned.original(output/"controller-before.original.json",controller_before)
            for path in CONTROLLER_FILES:
                target=output/"controller-originals"/path
                target.parent.mkdir(parents=True,exist_ok=True)
                owned.original(target,(controller/path).read_bytes())
            version=run("go-version",["go","version"]).decode().strip()
            require(version=="go version go1.24.13 linux/arm64","PINNED_GO_TOOLCHAIN")
            result["goVersion"]=version
            result["goEnvironment"]=json.loads(run("go-environment",["go","env","-json","GOOS","GOARCH","CGO_ENABLED","GOVERSION","GOFLAGS","GOTOOLCHAIN","GOMOD","GOCACHE","GOMODCACHE"]))
            require(git(controller,"controller-parent","rev-parse","HEAD^").decode().strip()==SOURCE,
                    "CONTROLLER_IS_EXACT_SOURCE_CHILD")
            for path, digest in SOURCE_PINS.items():
                require(before["files"][path]["sha256"]==digest, "EXACT_REPAIR_AND_ORACLE_SOURCE:"+path)
            observed_parents=[]
            for path in SOURCE_PINS:
                if path.endswith("_test.go"):
                    observed_parents.extend(re.findall(r"(?m)^func (Test[A-Za-z0-9_]+)\(",
                                                      (source/path).read_text()))
            require(sorted(observed_parents)==list(REQUIRED_PARENTS) and len(observed_parents)==23,
                    "ALL_23_ORIGINAL_AFFECTED_PARENTS")
            require(EXPECTED_PACKAGES==(PACKAGE,) and not NO_TEST_PACKAGES,
                    "ONE_SELECTED_ORIGINAL_ROOT_PACKAGE")
            # The original affected parents retain their180-second test deadline.
            # The600-second child includes actual hosted compilation; the unchanged
            # owner writes complete uncapped streams to exclusive files.
            actual_name="%03d-original-23-affected-parents"%(len(processes)+1)
            process=owned.run_owned(list(REGRESSION_COMMAND),cwd=source,env=env,output=commands,
                                    name=actual_name,budget=budget,seconds=600,monitor=monitor)
            processes.append(process)
            result["selectedOriginalParentsExecuted"]=process["pid"] is not None
            result["fullRootAndInternalSuiteExecuted"]=False
            result["priorFullSuiteRegressionPassed"]=False
            result["originalGoCommand"]=list(REGRESSION_COMMAND)
            result["originalGoProcessName"]=actual_name
            result["originalGoStreams"]={key:process.get(key) for key in ("stdout","stderr")}
            require(process["cleanup"]["parentReaped"] and
                    process["cleanup"]["after"]["state"]=="ABSENT", "OWNED_GO_GROUP_CLOSED")
            require(process["stdout"]["complete"] and process["stderr"]["complete"],
                    "COMPLETE_UNCAPPED_ORIGINAL_GO_STREAMS")
            def intake_tick():
                budget.check()
            summary=inspect_regression_stream(commands/(actual_name+".stdout.original"),
                    list(EXPECTED_PACKAGES),NO_TEST_PACKAGES,
                    [(PACKAGE,name) for name in REQUIRED_PARENTS],tick=intake_tick)
            require(summary["originalStreamBytes"]==process["stdout"]["bytes"] and
                    summary["originalStreamSHA256"]==process["stdout"]["sha256"],
                    "ENTIRE_ORIGINAL_GO_STREAM_RECONCILED")
            result["goTests"].append(summary)
            result["goEventIndex"]=owned.original(output/"go-event-index.derived.json",summary)
            result["repairSourcePins"]=SOURCE_PINS
            owned.require_success(process)
            require(summary["passed"],"SELECTED_ORIGINAL_PARENTS_FAILED")
            require(not summary["namedSkips"],"NO_SELECTED_ORIGINAL_SKIP")
            require({row["test"].split("/",1)[0] for row in summary["namedTerminals"]}==
                    set(REQUIRED_PARENTS),"EXACT_23_PARENT_FAMILIES")
            result["status"]="PASS_23_ORIGINAL_AFFECTED_PARENTS"

        except BaseException as error:
            result["error"]={"type":type(error).__name__,"message":str(error),"traceback":traceback.format_exc()}
        finally:
            try:
                with budget.finalizing():
                    after=snapshot(source,SOURCE,TREE,finalizing=True)
                    owned.original(output/"source-after.original.json",after)
                    controller_after=snapshot(controller,controller_sha,selected=CONTROLLER_FILES,finalizing=True)
                    owned.original(output/"controller-after.original.json",controller_after)
                    require(before==after and controller_before==controller_after,"SOURCE_OR_CONTROLLER_CHANGED")
                    require(sha(helper.read_bytes())==HELPER_SHA,"HELPER_CHANGED")
                    result["sourceUnchanged"]=True
            except BaseException as error:
                result["closingError"]={"type":type(error).__name__,"message":str(error)}
                result["status"]="FAILED"
            result["minimumCapacity"]=minimum
            result["finishedAt"]=owned.utc()
            receipt=owned.original(output/"result.original.json",result)
            print(json.dumps({"path":str(output/"result.original.json"),"sha256":receipt["sha256"],
                              "status":result["status"],"source":SOURCE,
                              "namedPasses":[x["namedPasses"] for x in result["goTests"]],
                              "streamBytes":result.get("originalGoStreams"),"error":result.get("error"),
                              "closingError":result.get("closingError")}))
    return 0 if result["status"]=="PASS_23_ORIGINAL_AFFECTED_PARENTS" else 1


if __name__ == "__main__":
    raise SystemExit(main())
