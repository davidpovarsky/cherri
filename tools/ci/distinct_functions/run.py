"""37 full original numeric/index/Unicode fixtures; no Apple acceptance.
Adapted from the qualified canonical source controller 53b4ba5/run45918200717.
Oracle attempt01 remains NOT_RUN due its original Development volume floor.
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
SOURCE = "8f0e1561754fe71fa3ccacd6cb6541ee9dbbd1c3"
TREE = "b31097edee52725ca6049014969a6320b8c9ad1e"
PARENT = "4d957978180d7c9a15db0a9ce2be886efbf545ed"
BRANCH = "probe/cherri-p10r15-native-numeric-8f0e1-20261127"
HELPER_SHA = "4672474ddb270a5857914041ebb279f20431f87e9da173e6cd8bf83b3fb11c85"
PREFIX = "tools/ci/distinct_functions/"
CONTROLLER_FILES = [".github/workflows/p10r15-distinct-functions.yml", PREFIX+"run.py", PREFIX+"owned_source_process.py"]
PACKAGE = "github.com/electrikmilk/cherri"
EXPORT_PARENT = "TestP15NativeNumericContractFixtureExport"
EXPORT_COMMAND = tuple(["go","test","-json","-count=1","-timeout=3m","-run","^(TestP15RP15CompoundOnceUnit|TestP15RP15CompoundOnceNative|TestP15RP17ComparisonsArithmeticUnit|TestP15RP17ComparisonsArithmeticNative|TestP15RP19LiteralUnicodeUnit|TestP15RP19LiteralUnicodeNativeRoundTrip|TestBackendRecovery08InlineCodecUnit|TestBackendRecovery08InlineCodecNativeRoundTrip|TestP15RP21SafeIndexUnit|TestP15RP21SafeIndexNative|TestP15BRG16NumericBoundsUnit|TestP15BRG16NumericBoundsNative|TestP15NativeNumericContractFixtureExport)$","."])
PARSER_PATH = "tools/shortcuts-simulator-qa/native_numeric_contract_fixtures.py"
PARSER_SHA = "1c4317df2b6935d924757915fa59462e193bbae776f674993fdb58b8a8828667"
EXPORTER_PATH = "p15_native_numeric_contract_export_test.go"
EXPORTER_SHA = "246d8dae95160baad1dc331aaf712d33bf56d9ddf10af6e4c333da54d1f90eb9"
SHARED_PATH = "tools/shortcuts-simulator-qa/native_go_fixture_stream.py"
SHARED_SHA = "71b843d5272ff0bed52b2ad89674959c55e041d7b2c6b0e1ad72fe65071e02d2"
CONTROLS_PATH = "tools/shortcuts-simulator-qa/test_native_numeric_contract_fixtures.py"
CONTROLS_SHA = "d2ebb519dee578cfc8d98d89f5e8f6251f7d10e860ef4151c85dd8aabcf56e45"



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
    output = Path(environ["RUNNER_TEMP"]).resolve(strict=True)/"cherri-native-numeric-export-8f0e1"
    output.mkdir(mode=0o700)
    commands, fixture_output = output/"commands", output/"fixtures"
    commands.mkdir(mode=0o700)
    fixture_output.mkdir(mode=0o700)
    temporary = output.parent/"cherri-native-numeric-export-tmp-8f0e1"
    temporary.mkdir(mode=0o700)
    env = {key: environ[key] for key in ("PATH", "HOME", "LANG", "LC_ALL", "LC_CTYPE", "TZ", "GOCACHE", "GOMODCACHE") if key in environ}
    env.update(PYTHONDONTWRITEBYTECODE="1", GOMAXPROCS="1", GOFLAGS="-p=1 -mod=readonly", GOMEMLIMIT="1GiB",
               GOTOOLCHAIN="local", GOENV="off", GOWORK="off", GOTELEMETRY="off",
               GIT_NO_REPLACE_OBJECTS="1", GIT_OPTIONAL_LOCKS="0", GIT_TERMINAL_PROMPT="0",
               TMPDIR=str(temporary), GOTMPDIR=str(temporary), CGO_ENABLED="1")
    processes = []
    minimum = {"disk": shutil.disk_usage(output).free, "memory": 1 << 62}
    result = {"schema":"cherri-native-numeric-export-qualification/1", "status":"FAILED",
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
            require(len(before["files"])==7902,"EXPECTED_SOURCE_FILE_COUNT")
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
            require(before["files"][PARSER_PATH]["sha256"]==PARSER_SHA and
                    before["files"][EXPORTER_PATH]["sha256"]==EXPORTER_SHA,"EXACT_EXPORT_SOURCE")
            require(before["files"][SHARED_PATH]["sha256"]==SHARED_SHA and
                    before["files"][CONTROLS_PATH]["sha256"]==CONTROLS_SHA,"EXACT_SHARED_PARSER_AND_CONTROLS")
            sys.path.insert(0,str(source/"tools/shortcuts-simulator-qa"))
            parser_spec=importlib.util.spec_from_file_location("original_native_numeric_contract_fixtures",source/PARSER_PATH)
            parser=importlib.util.module_from_spec(parser_spec)
            sys.modules[parser_spec.name]=parser
            parser_spec.loader.exec_module(parser)
            require(tuple(parser.EXPORT_COMMAND)==EXPORT_COMMAND and parser.PACKAGE==PACKAGE and
                    len(parser.CASES)==37 and len(parser.REQUIRED_EVENTS)==729,"FIXED_EXPORT_PROTOCOL")
            require({key:len(value) for key,value in parser.ORIGINAL_EVENTS.items()}==
                    {"RP15":80,"RP17":57,"RP21":234,"BRG16":213,"RP19":46,"BRG08":61},
                    "ALL_691_ORIGINAL_EVENTS")
            # The original test retains its internal 180-second deadline. The
            # separate 600-second command also includes the hosted Go compilation.
            actual=run("original-canonical-fixture-export",list(EXPORT_COMMAND),600)
            process=processes[-1]
            require(process["stdout"]["complete"] and process["stderr"]["complete"] and
                    process["stdout"]["sha256"]==sha(actual),"COMPLETE_ORIGINAL_EXPORT_STREAMS")
            summary=validate_go(actual,[PACKAGE],[(PACKAGE,name) for name in sorted(parser.REQUIRED_EVENTS)])
            fixtures=parser.parse_exports(actual,expected_sha256=process["stdout"]["sha256"])
            require(len(fixtures)==37 and summary["namedPasses"]==729,"EXACT_37_CASES_729_EVENTS")
            summary["requiredNamedEvents"]=sorted(parser.REQUIRED_EVENTS)
            result["goTests"].append(summary)
            manifest=[]
            for index,(case,fixture) in enumerate(zip(parser.CASES,fixtures),1):
                budget.check()
                directory=fixture_output/("%03d-%s"%(index,case.requirement))
                directory.mkdir(mode=0o700)
                source_bytes=base64.b64decode(fixture["sourceBase64"],validate=True)
                native_bytes=base64.b64decode(fixture["unsignedBase64"],validate=True)
                source_receipt=owned.original(directory/"source.original.cherri",source_bytes)
                native_receipt=owned.original(directory/"unsigned.original.plist",native_bytes)
                export_receipt=owned.original(directory/"export.derived.json",fixture)
                require(source_receipt["sha256"]==fixture["sourceSha256"] and
                        native_receipt["sha256"]==fixture["unsignedSha256"],"EXPORTED_BYTE_BINDINGS")
                manifest.append({"testId":case.export_test,"requirement":case.requirement,"caseId":case.name,
                    "directory":directory.relative_to(output).as_posix(),"source":source_receipt,
                    "unsigned":native_receipt,"derivedExport":export_receipt,
                    "originalGoStreamSHA256":process["stdout"]["sha256"],
                    "nativeExecuted":False,"formalAcceptanceGranted":False})
            manifest_receipt=owned.original(output/"fixture-manifest.derived.json",{
                "schema":"cherri-original-numeric-native-fixture-manifest/1","source":SOURCE,"tree":TREE,
                "runId":result["runId"],"attempt":1,"controller":controller_sha,
                "originalCommand":list(EXPORT_COMMAND),"originalGoStreamSHA256":sha(actual),
                "fixtures":manifest,"nativeExecuted":False,"formalAcceptanceGranted":False})
            result["fixtureManifest"]=manifest_receipt
            result["fixtureCount"]=len(manifest)
            result["exportSourcePins"]={PARSER_PATH:PARSER_SHA,EXPORTER_PATH:EXPORTER_SHA,SHARED_PATH:SHARED_SHA,CONTROLS_PATH:CONTROLS_SHA}
            result["status"]="PASS_COMPILED_FIXTURE_EXPORTS"
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
                              "fixtureCount":result.get("fixtureCount"),"error":result.get("error"),
                              "closingError":result.get("closingError")}))
    return 0 if result["status"]=="PASS_COMPILED_FIXTURE_EXPORTS" else 1


if __name__ == "__main__":
    raise SystemExit(main())
