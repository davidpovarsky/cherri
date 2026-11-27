"""Source-bound Docs annotation causal controls and reusable actual-original test binaries."""
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
SOURCE = "cf42869ab558c0b84cd5bfc599052afb86e48d15"
TREE = "72b22b03de478442d7f211da255961fd64c8ac5c"
PARENT = "5f184cd60d26e9e683c87e4acd231912231493c1"
REGRESSION = PARENT
REGRESSION_TREE = "6d0ec992bb88e9d72b5e141d4405c6730221d103"
REGRESSION_PARENT = "e3fec8326106be7eca5db0c90bc3be56f578cd77"
BRANCH = "probe/cherri-p10r15-docs-provider-annotation-20261127"
HELPER_SHA = "4672474ddb270a5857914041ebb279f20431f87e9da173e6cd8bf83b3fb11c85"
PREFIX = "tools/ci/distinct_functions/"
CONTROLLER_FILES = [".github/workflows/p10r15-distinct-functions.yml", PREFIX+"run.py", PREFIX+"owned_source_process.py"]
PACKAGE = "github.com/electrikmilk/cherri/tools/language-acceptance"
ORIGINAL_PARENT = "TestDocsProcessPublicationControls"
ANNOTATION_PARENT = "TestDocsProcessPublicationSentinelAnnotations"
ORIGINAL_CHILDREN = ["positive-original-format","wrong-docs-source","wrong-compiler-source","force-argument","second-ref","wrong-repository-directory","failed-push","reversed-push-time","missing-provider-original","provider-contradiction","provider-content-tamper","preflight-original-missing","remote-source-mismatch","remote-tree-mismatch","post-witness-before-push","post-wrong-source","post-command-missing","post-command-timed-out","post-ancestry-failed","post-ancestry-reversed","dirty-post-status","hidden-index","unrepinned-stream-tamper","duplicate-stream"]
ANNOTATION_CHILDREN = ["actual-continuity-shape","duplicate-publication-result","second-conflicting-publication-result","structured-text-conflict","arbitrary-extra-text","duplicate-continuity-annotations","annotation-before-result","annotation-without-result","nontext-annotation","annotation-extra-field","annotation-embedded-result","annotation-altered-directive","annotation-overlong-context","annotation-negative-revision","annotated-force-command","annotated-second-ref","annotated-ref-mismatch","annotated-failed-push","annotated-provider-failure"]
REQUIRED_EVENTS = frozenset([ORIGINAL_PARENT, ANNOTATION_PARENT] +
    [ORIGINAL_PARENT + "/" + n for n in ORIGINAL_CHILDREN] +
    [ANNOTATION_PARENT + "/" + n for n in ANNOTATION_CHILDREN])
REGRESSION_EVENTS = frozenset([ANNOTATION_PARENT] + [ANNOTATION_PARENT + "/" + n for n in ANNOTATION_CHILDREN])
RED_COMMAND = ("go", "test", "-json", "-count=1", "-timeout=3m",
               "-run", "^" + ANNOTATION_PARENT + "$", "./tools/language-acceptance")
TEST_COMMAND = ("go", "test", "-json", "-count=1", "-timeout=3m",
                "-run", "^(" + ORIGINAL_PARENT + "|" + ANNOTATION_PARENT + ")$", "./tools/language-acceptance")
TEST_PATH = "tools/language-acceptance/docs_publication_annotation_test.go"
TEST_SHA = "98b5e1b405a09e355ec5e435e217f414a29186d2345a5444aa56ca771f838d7d"
ORIGINAL_TEST_PATH = "tools/language-acceptance/docs_publication_execution_test.go"
ORIGINAL_TEST_SHA = "4355e653436d7d74ae571a33057d06fbca478e6109491235873d0199f0e11e26"
VALIDATOR_PATH = "tools/language-acceptance/docs_publication_execution.go"
VALIDATOR_SHA = "58d3d9f9135f9fd4488b7a31fd4872f75652547136b1d3eefe64fe41f96d6378"
OLD_VALIDATOR_SHA = "e2a78a5192228b6c23dddec17a215c65bda334f91efd652cc68445ad86c0484e"



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
    controller, source, regression = workspace/"controller", workspace/"source", workspace/"regression"
    for directory in (controller, source, regression):
        require(directory.resolve(strict=True) == directory and (directory/".git").is_dir() and
                not (directory/".git/objects/info/alternates").exists() and
                not (directory/".git/info/grafts").exists(), "ORDINARY_SEPARATE_SOURCE")
    helper = controller/(PREFIX+"owned_source_process.py")
    require(sha(helper.read_bytes()) == HELPER_SHA, "UNCHANGED_QUALIFIED_PROCESS_OWNER")
    spec = importlib.util.spec_from_file_location("distinct_function_owned_process", helper)
    owned = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(owned)
    output = Path(environ["RUNNER_TEMP"]).resolve(strict=True)/"cherri-docs-provider-annotation-cf42869"
    output.mkdir(mode=0o700)
    commands = output/"commands"
    commands.mkdir(mode=0o700)
    temporary = output.parent/"cherri-docs-provider-annotation-tmp-cf42869"
    temporary.mkdir(mode=0o700)
    env = {key: environ[key] for key in ("PATH", "HOME", "LANG", "LC_ALL", "LC_CTYPE", "TZ", "GOCACHE", "GOMODCACHE") if key in environ}
    env.update(PYTHONDONTWRITEBYTECODE="1", GOMAXPROCS="2", GOFLAGS="-p=2 -mod=readonly",
               GOTOOLCHAIN="local", GOENV="off", GOWORK="off", GOTELEMETRY="off",
               GIT_NO_REPLACE_OBJECTS="1", GIT_OPTIONAL_LOCKS="0", GIT_TERMINAL_PROMPT="0",
               TMPDIR=str(temporary), GOTMPDIR=str(temporary), CGO_ENABLED="1")
    processes = []
    minimum = {"disk": shutil.disk_usage(output).free, "memory": 1 << 62}
    result = {"schema":"cherri-docs-provider-annotation-qualification/1", "status":"FAILED",
              "source":SOURCE, "tree":TREE, "sourceParent":PARENT, "controller":controller_sha,
              "runId":int(environ["GITHUB_RUN_ID"]), "attempt":1, "event":"create", "branch":BRANCH,
              "host":{"system":platform.system(), "machine":platform.machine(), "release":platform.release(),
                      "python":platform.python_version()}, "runner":"ubuntu-24.04-arm",
              "wholeOwnerSeconds":1200, "goTestInternalSeconds":180,
              "compilationInclusiveChildSeconds":600, "processes":processes, "goTests":[],
              "formalAcceptanceGranted":False, "rawNativeTypesObserved":False,
              "full179Executed":False, "nativeAppleExecuted":False, "providerInvoked":False,
              "privateCorpusPresent":False, "releasePromoted":False, "startedAt":owned.utc()}
    owned.original(output/"github-event.original.json", Path(environ["GITHUB_EVENT_PATH"]).read_bytes())
    with owned.WholeBudget(1200, cleanup_seconds=8, final_reserve=35) as budget:
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
        def run_regression(name, argv, expected_exit, seconds=600):
            actual_name = "%03d-%s"%(len(processes)+1,name)
            row = owned.run_owned(argv, cwd=regression, env=env,
                                  output=commands, name=actual_name, budget=budget,
                                  seconds=seconds, monitor=monitor)
            processes.append(row)
            require(expected_exit in (0,1) and row["exitCode"] == expected_exit and
                    row["status"] == ("PASS" if expected_exit == 0 else "EXIT_NONZERO") and
                    not row["timedOut"] and not row["cancelled"] and not row["wholeDeadlineExceeded"] and
                    row["error"] is None and row["cleanup"]["parentReaped"] and
                    row["cleanup"]["after"]["state"] == "ABSENT" and
                    row["stdout"]["complete"] and row["stderr"]["complete"],
                    "EXACT_OWNED_REGRESSION_OUTCOME")
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
        before=regression_before=controller_before=None
        try:
            monitor()
            before=snapshot(source,SOURCE,TREE)
            require(len(before["files"])==7892,"EXPECTED_SOURCE_FILE_COUNT")
            regression_before=snapshot(regression,REGRESSION,REGRESSION_TREE)
            require(len(regression_before["files"])==7892,"EXPECTED_REGRESSION_FILE_COUNT")
            require(git(regression,"regression-parent","rev-parse","HEAD^").decode().strip()==REGRESSION_PARENT,"REGRESSION_PARENT")
            changed=[p for p in before["files"] if before["files"][p]!=regression_before["files"].get(p)]
            require(changed==[VALIDATOR_PATH] and set(before["files"])==set(regression_before["files"]),"ONLY_PRODUCTION_PARSER_DIFFERS")
            controller_before=snapshot(controller,controller_sha,selected=CONTROLLER_FILES)
            require(git(source,"source-parent","rev-parse","HEAD^").decode().strip()==PARENT,"SOURCE_PARENT")
            owned.original(output/"source-before.original.json",before)
            owned.original(output/"regression-before.original.json",regression_before)
            owned.original(output/"controller-before.original.json",controller_before)
            for path in CONTROLLER_FILES:
                target=output/"controller-originals"/path
                target.parent.mkdir(parents=True,exist_ok=True)
                owned.original(target,(controller/path).read_bytes())
            version=run("go-version",["go","version"]).decode().strip()
            require(version=="go version go1.24.13 linux/arm64","PINNED_GO_TOOLCHAIN")
            result["goVersion"]=version
            result["goEnvironment"]=json.loads(run("go-environment",["go","env","-json","GOOS","GOARCH","CGO_ENABLED","GOVERSION","GOFLAGS","GOTOOLCHAIN","GOMOD","GOCACHE","GOMODCACHE"]))
            for snapshot_value in (before,regression_before):
                require(snapshot_value["files"][TEST_PATH]["sha256"] == TEST_SHA and
                        snapshot_value["files"][ORIGINAL_TEST_PATH]["sha256"] == ORIGINAL_TEST_SHA,
                        "EXACT_NEW_AND_UNCHANGED_ORIGINAL_CONTROLS")
            require(before["files"][VALIDATOR_PATH]["sha256"] == VALIDATOR_SHA and
                    regression_before["files"][VALIDATOR_PATH]["sha256"] == OLD_VALIDATOR_SHA,
                    "EXACT_OLD_AND_NEW_PRODUCTION_PARSER")
            require(run("gofmt-check",["gofmt","-l",str(source/VALIDATOR_PATH),str(source/TEST_PATH)])==b"",
                    "FROZEN_GO_FILES_FORMATTED")
            old_raw=run_regression("old-source-annotation-causal",list(RED_COMMAND),1)
            events=[json.loads(line) for line in old_raw.decode().splitlines() if line.strip()]
            require(events and not any(e.get("Action")=="skip" for e in events),"NO_SKIPPED_CAUSAL_EVENT")
            runs=[e["Test"] for e in events if e.get("Action")=="run" and e.get("Test")]
            passes=[e["Test"] for e in events if e.get("Action")=="pass" and e.get("Test")]
            failures=[e["Test"] for e in events if e.get("Action")=="fail" and e.get("Test")]
            rejected=ANNOTATION_PARENT+"/actual-continuity-shape"
            require(all(e.get("Package")==PACKAGE for e in events) and
                    len(runs)==len(set(runs)) and set(runs)==REGRESSION_EVENTS and
                    sorted(failures)==sorted([ANNOTATION_PARENT,rejected]) and
                    len(passes)==len(set(passes)) and set(passes)==REGRESSION_EVENTS-{ANNOTATION_PARENT,rejected},
                    "EXACT_CAUSAL_OLD_SOURCE_FAILURE_ONLY")
            require([e["Action"] for e in events if not e.get("Test") and e.get("Action") in ("pass","fail","skip")]==["fail"],
                    "EXACT_OLD_PACKAGE_FAILURE")
            reason="unexpected annotated original-format control result: docs original process publication provider response content is ambiguous"
            require(sum(reason in e.get("Output","") for e in events if e.get("Test")==rejected)==1,
                    "ORIGINAL_REJECTION_CAUSE")
            result["oldSourceCausal"]={"source":REGRESSION,"tree":REGRESSION_TREE,"command":list(RED_COMMAND),
                "namedRuns":len(runs),"namedPasses":len(passes),"namedFailures":failures,
                "originalStreamSHA256":sha(old_raw),"expectedExitCode":1,"cause":reason}
            actual=run("current-source-original-controls",list(TEST_COMMAND),600)
            summary=validate_go(actual,[PACKAGE],[(PACKAGE,n) for n in sorted(REQUIRED_EVENTS)])
            current_events=[json.loads(line) for line in actual.decode().splitlines() if line.strip()]
            current_runs=[e["Test"] for e in current_events if e.get("Action")=="run" and e.get("Test")]
            require(summary["namedPasses"]==45 and len(current_runs)==45 and
                    len(set(current_runs))==45 and set(current_runs)==REQUIRED_EVENTS,
                    "EXACT_24_ORIGINAL_PLUS_19_ANNOTATION_CONTROLS_AND_PARENTS")
            summary["requiredNamedEvents"]=sorted(REQUIRED_EVENTS)
            result["goTests"].append(summary)
            products=output/"public-test-binaries";products.mkdir(mode=0o700)
            result["products"]={}
            for label,directory,revision in (("regression",regression,REGRESSION),("current",source,SOURCE)):
                target=products/(label+"-language-acceptance.test")
                build=["go","test","-c","-buildvcs=true","-o",str(target),"./tools/language-acceptance"]
                if label=="regression":
                    run_regression(label+"-actual-original-test-binary",build,0,600)
                else:
                    run(label+"-actual-original-test-binary",build,600)
                info=run(label+"-test-binary-build-info",["go","version","-m",str(target)]).decode()
                require("go1.24.13" in info and "vcs.revision="+revision in info and
                        "vcs.modified=false" in info and "GOARCH=arm64" in info and "GOOS=linux" in info,
                        "SOURCE_BOUND_ACTUAL_ORIGINAL_TEST_BINARY")
                observed=stable(target)
                require(observed["mode"]&0o111,"EXECUTABLE_TEST_BINARY")
                owned.original(products/(label+"-build-info.original.txt"),info.encode())
                result["products"][label]={"source":revision,"path":str(target),"file":observed,
                    "buildCommand":build,"buildInfoSHA256":sha(info.encode()),"actualOriginalPublicationExecuted":False}
            result["originalCommand"]=list(TEST_COMMAND)
            result["sourcePins"]={TEST_PATH:TEST_SHA,ORIGINAL_TEST_PATH:ORIGINAL_TEST_SHA,VALIDATOR_PATH:VALIDATOR_SHA}
            result["regression"]={"source":REGRESSION,"tree":REGRESSION_TREE,"parent":REGRESSION_PARENT}
            result["scope"]="Strict original Docs publication parser controls plus one causal annotation repair. Real retained publication verification remains a separate private Oracle opt-in test; no publication replay or final179 acceptance."
            result["status"]="PASS_DOCS_PROVIDER_ANNOTATION_CAUSAL_CONTROLS"
        except BaseException as error:
            result["error"]={"type":type(error).__name__,"message":str(error),"traceback":traceback.format_exc()}
        finally:
            try:
                with budget.finalizing():
                    after=snapshot(source,SOURCE,TREE,finalizing=True)
                    owned.original(output/"source-after.original.json",after)
                    regression_after=snapshot(regression,REGRESSION,REGRESSION_TREE,finalizing=True)
                    owned.original(output/"regression-after.original.json",regression_after)
                    require(regression_before==regression_after,"REGRESSION_SOURCE_CHANGED")
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
                              "goTests":result["goTests"],"error":result.get("error"),
                              "closingError":result.get("closingError")}))
    return 0 if result["status"]=="PASS_DOCS_PROVIDER_ANNOTATION_CAUSAL_CONTROLS" else 1


if __name__ == "__main__":
    raise SystemExit(main())
