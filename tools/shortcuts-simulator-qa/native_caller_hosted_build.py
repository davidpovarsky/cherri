"""Fresh hosted build of the original fifteen-input standalone FN04 harness.

This is the hosted environment adaptation of the retained local build recipe.
It keeps the 780-second owner, 600-second Xcode child, all fifteen compiler-input
pins, complete before/after source checks, and original product verification.
"""
from pathlib import Path
import argparse
import hashlib
import json
import os
import shutil
import signal
import stat
import sys
import threading
import time

import native_caller_execution_context as execution

PACKET_SOURCE = "5b689c938c73420ef36b3f077764d30f8946f31a"
PROJECT_PINS = {"tools/shortcuts-simulator-qa/XcodeProject/project.yml":{"bytes":1074,"sha256":"51b5ef33721acedf3e67b42985f9c2ccca2051cefd5f48de5f08f989242ba88b","mode":"100644","gitBlob":"39a2633e519225b86e827089b8c78a205a158cf8"},"tools/shortcuts-simulator-qa/XcodeProject/Shared/DebugLabelPreviewPolicy.swift":{"bytes":9110,"sha256":"8409403164e989eeb85b70153ce63383bbc2fd06f47604083a05612c0766c1d1","mode":"100644","gitBlob":"ba2a2014d11121d7cd8551133ca960ca3a87da4c"},"tools/shortcuts-simulator-qa/XcodeProject/Shared/DifferentialImportTiming.swift":{"bytes":10794,"sha256":"3ce8143ff4b591a82ab26de94ba19a45a4e32731210c3a6fa23add4ecf13baea","mode":"100644","gitBlob":"b01d0278e418de3b0fc628781ec572dda74abe45"},"tools/shortcuts-simulator-qa/XcodeProject/Shared/LibraryForegroundGate.swift":{"bytes":2273,"sha256":"0ab1072c24facb41131a168725ffdc95720535a9d6ed374ce9a4494aaca34ba0","mode":"100644","gitBlob":"b4f6a59eb42bfe91ff3dd1eb7ef4701914b9cbcc"},"tools/shortcuts-simulator-qa/XcodeProject/Shared/CallbackPayloadDiagnostics.swift":{"bytes":1552,"sha256":"7a043ad54ec96621d7f3ec386ebbbdd39fec108e1d2ed2f288b9ecddc669a5f2","mode":"100644","gitBlob":"83b6c1bdb5a38cc56551a616dbd334f213e32806"},"tools/shortcuts-simulator-qa/XcodeProject/Shared/EvidenceGeometry.swift":{"bytes":1317,"sha256":"8fbeb078a3dea40c362757b7834620c88444e82c932f4dda4e4495e105338a30","mode":"100644","gitBlob":"f3f7d57bdd5f8bd7d8b6476e6f4a146efdffa071"},"tools/shortcuts-simulator-qa/XcodeProject/Shared/EditorForegroundRecovery.swift":{"bytes":3945,"sha256":"22c5f458643dac109afc34ee3357c528d9e3b34c6126f4b602e9394e750986a9","mode":"100644","gitBlob":"b40b91199128e4f34bcb7fb1548cda8f3bfddd52"},"tools/shortcuts-simulator-qa/XcodeProject/Shared/ImportSheetSelection.swift":{"bytes":20576,"sha256":"593006730531d81e17175f32edef0d83b37bf47d7bd572f49990766b6ddd3f8e","mode":"100644","gitBlob":"cabf958d996629df0311b150e52f0aa7423477a4"},"tools/shortcuts-simulator-qa/XcodeProject/Shared/PrimaryNoInputLaunch.swift":{"bytes":17244,"sha256":"ec9b15a08fd412fa5885e7118d94ff2d3b202aee7d4e32b375d304a4880e06d8","mode":"100644","gitBlob":"fe1269e3a4ca31e86a5ae8fe683143d6cb5bc1c1"},"tools/shortcuts-simulator-qa/XcodeProject/ReceiverApp/Info.plist":{"bytes":1792,"sha256":"855047fe836d4410c65ba55a6b6e1711dcce391eabf76fe92f650c8254256a3e","mode":"100644","gitBlob":"9ec2337fd56d55d1c125f21a00752ba091baf964"},"tools/shortcuts-simulator-qa/XcodeProject/ReceiverApp/ReceiverApp.swift":{"bytes":16556,"sha256":"c1249a6aa1a81746afb86cfde8dc2a0f162422ffe86716778eb7e72db24f0367","mode":"100644","gitBlob":"a350bbac81294a7186306a9fe39107f369a961a8"},"tools/shortcuts-simulator-qa/XcodeProject/UITests/QAImportedShortcutObserver.swift":{"bytes":27226,"sha256":"4827556366ec3049f9b397e19ea32b583ab7516a362e2b060e6495c16c41b91f","mode":"100644","gitBlob":"9f69fbad4ffcf511e9a3df49f0ecfd4552e3cb9b"},"tools/shortcuts-simulator-qa/XcodeProject/UITests/CherriDifferentialBatchUITests.swift":{"bytes":12038,"sha256":"0de6151a9fc82855dce209bbd66d751b664ab4e5d19f919bb1260a4c13599bc6","mode":"100644","gitBlob":"48a70d7c3928073b38cc799d36f7183938747c1e"},"tools/shortcuts-simulator-qa/XcodeProject/UITests/CherriSimulatorQAUITests.swift":{"bytes":85607,"sha256":"41b8e50d57ffee16bd7d60f5dc19d67f770ff5080f03c41cdd78fa289b25b274","mode":"100644","gitBlob":"ba3a4595a7b2713d2fc24cf47971f7ae1a45d01b"},"tools/shortcuts-simulator-qa/XcodeProject/UITests/DebugLabelPreviewObserver.swift":{"bytes":6647,"sha256":"e9b5ebeebc6b125483fee9ed9fb21f0d001d72886b5d5b4cf27865fa0f940a9b","mode":"100644","gitBlob":"fffbcf1020fd52568171a3e69bb9c3f1592e977a"}}
ADDITIONAL_ESTIMATE = 1024**3
OS_MARGIN = 512*1024**2
WHOLE_SECONDS = 780

def sha(raw):return hashlib.sha256(raw).hexdigest()
def require(value,code):
 if not value:raise ValueError(code)
def raw(path,value):
 path=Path(path)
 with path.open("xb") as f:f.write(value);f.flush();os.fsync(f.fileno())
 return {"bytes":len(value),"sha256":sha(value)}
def record(path,value):return raw(path,(json.dumps(value,ensure_ascii=False,sort_keys=True,indent=2,allow_nan=False)+"\n").encode())
class InterruptionLatch:
    """Keep the first cancellation while the existing helper drains its group."""
    def __init__(self):
        self.reason = None

    def interrupt(self, reason):
        if self.reason is not None:
            return
        self.reason = reason
        signal.setitimer(signal.ITIMER_REAL, 0)
        raise TimeoutError(reason)
def main(argv=None, *, parent_deadline=None, parent_interruption=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--execution-context", required=True)
    parser.add_argument("--device-record", required=True)
    args = parser.parse_args(argv)
    context = execution.validate_context(json.loads(Path(args.execution_context).read_bytes()))
    locations = execution.paths(context)
    SOURCE = context["sourceCommitSha"]
    W = locations["repo"]
    QA = W / "tools/shortcuts-simulator-qa"
    PROJECT = QA / "XcodeProject"
    GENERATED = PROJECT / "CherriSimulatorQA.xcodeproj"
    O = locations["buildRoot"]
    DD = locations["derivedData"]
    device_record = json.loads(Path(args.device_record).read_bytes())
    require(device_record["sourceCommit"] == SOURCE and
            device_record["runId"] == context["runId"], "HOSTED_BUILD_DEVICE_CONTEXT")
    DEVICE = device_record["udid"]
    os.environ["PATH"] = str(Path(sys.executable).parent) + os.pathsep + os.environ.get("PATH", "")
    require(sys.platform=="darwin" and sys.version_info >= (3,10),"DARWIN_SHARED_MONOTONIC_CLOCK")
    require(W.is_dir() and not W.is_symlink() and W.resolve()==W and W.stat().st_uid==os.getuid(),"OWNED_ORDINARY_SOURCE")
    require(not O.exists() and not DD.exists() and not GENERATED.exists(),"CREATE_ONLY_BUILD_OUTPUTS")
    O.mkdir(mode=0o700);(O/"processes").mkdir(mode=0o700)
    record(O/"compiled-inputs.expected.original.json",{"source":PACKET_SOURCE,"files":PROJECT_PINS})
    raw(O/"owner.original.py",Path(__file__).read_bytes())
    require(sha((QA/"project_prerequisite_observation.py").read_bytes())=="0d60de3707650e3f48f5023eeb92759abfaf0f5152cc96afa7c6d8a9a2059315","QUALIFIED_CAPTURE_PIN")
    require(sha((QA/"source_binding_observation.py").read_bytes())=="3c70e78347abeea9d84ff2c32b0f9c1b593bef0774f09d1fba9f9519d998f318","SOURCE_GUARD_PIN")
    sys.path.insert(0,str(QA))
    import project_prerequisite_observation as capture
    import source_binding_observation as binding
    started=time.monotonic();deadline=started+WHOLE_SECONDS
    if parent_deadline is not None:
     require(type(parent_deadline) in (int,float) and started<parent_deadline,"HOSTED_PARENT_DEADLINE")
     deadline=min(deadline,parent_deadline)
    latch=InterruptionLatch();stop=threading.Event();watch=None;minimum=None
    handlers={kind:signal.getsignal(kind) for kind in [signal.SIGALRM,signal.SIGTERM,signal.SIGINT]}
    capacity_reason={"low":False}
    require(parent_interruption is None or callable(parent_interruption),"PARENT_INTERRUPTION_SINK")
    def cancel(kind,frame):
     reason="BUILD_CAPACITY_EXHAUSTED" if capacity_reason["low"] else "BUILD_SIGNAL_"+str(kind)
     if latch.reason is None and parent_interruption is not None:parent_interruption(reason)
     latch.interrupt(reason)
    for kind in handlers:signal.signal(kind,cancel)
    signal.setitimer(signal.ITIMER_REAL,max(0.001,deadline-time.monotonic()))
    result={"schema":"cherri-native-caller-standalone-build/1","status":"FAIL","builtHarnessCommitSha":SOURCE,
    "executionLocation":"GITHUB_ACTIONS_MACOS","hostedContext":context,
    "builtHarnessTree":None,"compiledInputEquivalentSource":PACKET_SOURCE,"projectInputCount":len(PROJECT_PINS),
    "wholeOwnerSeconds":WHOLE_SECONDS,"buildSeconds":600,"nativeInvocation":False,"testsExecuted":False,"compilerRebuilt":False,
    "processes":[],"startedAt":binding.utc(),"capacityEstimate":{"additionalBytes":ADDITIONAL_ESTIMATE,"osMarginBytes":OS_MARGIN,
    "basis":"Fifteen current project inputs total217751 bytes; only Apple SDK dependencies; retained old standalone products4.75MB. OneGiB conservative new build/cache allowance, not a main-app estimate."}}
    before=None
    def capacity():
     nonlocal minimum
     v=os.statvfs("/System/Volumes/Data");free=v.f_bavail*v.f_frsize
     minimum=free if minimum is None else min(minimum,free)
     require(free>=OS_MARGIN,"BUILD_OS_MARGIN")
     return free
    def command(label,argv,seconds):
     capacity();folder=O/"processes"/label;folder.mkdir(mode=0o700)
     observed=details=None;last={}
     def journal(event,value):
      last.clear();last.update(value);record(folder/(event+".original.json"),value)
     try:
      with (folder/"stdout.original").open("xb") as out,(folder/"stderr.original").open("xb") as err:
       observed,details=capture.capture(argv,seconds,output=out,error_output=err,
        hard_deadline=min(deadline-10,time.monotonic()+seconds+5),journal=journal)
     finally:
      row={"label":label,"argv":argv,"observation":observed,"process":details or last,"firstInterruption":latch.reason}
      record(folder/"result.original.json",row);result["processes"].append(row)
     require(observed and observed["result"]=="PASS" and observed["exitCode"]==0,"BUILD_COMMAND_"+label)
     require(details["cleanupParentReaped"] is True and details["groupPresentAfterCleanup"] is False,"BUILD_COMMAND_GROUP")
     capacity();return (folder/"stdout.original").read_bytes()
    def preserve_generated():
     if not GENERATED.exists():return
     require(GENERATED.is_dir() and not GENERATED.is_symlink(),"GENERATED_PROJECT_KIND")
     inventory={}
     for p in sorted(GENERATED.rglob("*")):
      require(not p.is_symlink(),"GENERATED_PROJECT_SYMLINK")
      if p.is_file():
       data=p.read_bytes();require(len(data)<=2*1024**2 and len(inventory)<100,"GENERATED_PROJECT_BOUND")
       inventory[p.relative_to(GENERATED).as_posix()]={"bytes":len(data),"sha256":sha(data),"mode":stat.S_IMODE(p.stat().st_mode)}
     target=O/"generated-project.original.xcodeproj";os.rename(GENERATED,target)
     for name,pin in inventory.items():
      p=target/name;require(sha(p.read_bytes())==pin["sha256"] and stat.S_IMODE(p.stat().st_mode)==pin["mode"],"GENERATED_RELOCATION")
     record(O/"generated-project.original.json",inventory);result["generatedProjectRetained"]=True
    try:
     df=command("data-free",["/bin/df","-k","/System/Volumes/Data"],10).decode()
     result["dataFreeOriginal"]=df;result["freeBefore"]=capacity()
     require(result["freeBefore"]>=ADDITIONAL_ESTIMATE+OS_MARGIN,"BUILD_INCREMENTAL_ADMISSION")
     command("vm-stat",["/usr/bin/vm_stat"],10)
     version=command("xcode-version",["/usr/bin/xcodebuild","-version"],15).decode()
     require(version.splitlines()[0].startswith("Xcode 27"),"XCODE27");result["xcodeVersion"]=version
     runtimes=json.loads(command("available-runtimes",["/usr/bin/xcrun","simctl","list","runtimes","--json"],15))
     devices=json.loads(command("existing-devices",["/usr/bin/xcrun","simctl","list","devices","--json"],15))
     matches=[(rid,d) for rid,group in devices["devices"].items() for d in group if d.get("udid")==DEVICE]
     require(len(matches)==1,"EXACT_OWNED_DEVICE")
     rid,device=matches[0];runtime=[r for r in runtimes["runtimes"] if r.get("identifier")==rid]
     require(device.get("isAvailable") is True and len(runtime)==1 and runtime[0].get("isAvailable") is True and rid=="com.apple.CoreSimulator.SimRuntime.iOS-27-0","OWNED_IOS27_RUNTIME")
     require(device.get("deviceTypeIdentifier","").startswith("com.apple.CoreSimulator.SimDeviceType.iPhone-"),"OWNED_PHONE_PROFILE")
     result["device"]={"udid":DEVICE,"runtimeId":rid,"device":device,"runtime":runtime[0],"deviceMutation":False}
     critical=list(PROJECT_PINS)+["tools/shortcuts-simulator-qa/source_binding_observation.py","tools/shortcuts-simulator-qa/project_prerequisite_observation.py","tools/shortcuts-simulator-qa/function_envelope_observer.py","tools/shortcuts-simulator-qa/native_caller_hosted_build.py","tools/shortcuts-simulator-qa/native_caller_execution_context.py"]
     before=binding.source_inventory(W,SOURCE,O/"source-before",critical)
     require(before["completeTrackedSource"] is True,"COMPLETE_ORIGINAL_BUILD_SOURCE")
     result["builtHarnessTree"]=before["tree"]
     actual={k:v for k,v in before["files"].items() if k.startswith("tools/shortcuts-simulator-qa/XcodeProject/")}
     require(actual==PROJECT_PINS,"ALL_COMPILED_PROJECT_BYTES_MATCH_PACKET_SOURCE")
     result["sourceBeforeSHA256"]=sha((O/"source-before/snapshot.original.json").read_bytes())
     record(O/"compiled-inputs.actual.original.json",actual)
     def monitor():
      while not stop.wait(1):
       try:
        capacity()
       except BaseException:
        capacity_reason["low"]=True;os.kill(os.getpid(),signal.SIGTERM);return
     watch=threading.Thread(target=monitor,daemon=True);watch.start()
     DD.mkdir(mode=0o700)
     generator=shutil.which("xcodegen");require(generator is not None,"EXISTING_XCODEGEN")
     command("xcodegen",[generator,"generate","--spec",str(PROJECT/"project.yml")],30)
     command("xcodebuild",["/usr/bin/xcodebuild","build-for-testing","-project",str(GENERATED),"-scheme","CherriSimulatorQAReceiver",
      "-derivedDataPath",str(DD),"-destination","platform=iOS Simulator,id="+DEVICE,
      "CODE_SIGNING_ALLOWED=YES","CODE_SIGNING_REQUIRED=NO","CODE_SIGN_IDENTITY=-"],600)
     products=DD/"Build/Products";xct=list(products.glob("*.xctestrun"));require(len(xct)==1,"EXACT_BUILT_XCTESTRUN")
     receiver=products/"Debug-iphonesimulator/CherriSimulatorQAReceiver.app";runner=products/"Debug-iphonesimulator/CherriSimulatorQAUITests-Runner.app"
     for label,path in [("receiver",receiver),("runner",runner)]:
      command("codesign-verify-"+label,["/usr/bin/codesign","--verify","--deep","--strict",str(path)],20)
     import function_envelope_observer as native
     inventory=native.test_products_inventory(xct[0])
     result["productInventory"]=record(O/"build-products.original.json",inventory)
     result["receiverImage"]=record(O/"receiver-image.original.json",native.differential.receiver_image_inventory(receiver))
     result.update(receiverApp=str(receiver),xctestrun=str(xct[0]),productFiles=len(inventory["files"]),productBytes=inventory["bytes"])
     result["status"]="BUILD_VALIDATED"
    except BaseException as exc:
     result["error"]={"type":type(exc).__name__,"message":str(exc)[:1200]}
    finally:
     stop.set()
     if watch is not None:watch.join(timeout=2)
     try:preserve_generated()
     except BaseException as exc:result["generatedRetentionError"]={"type":type(exc).__name__,"message":str(exc)[:500]};result["status"]="FAIL"
     if before is not None and latch.reason is None:
      try:
       after=binding.source_inventory(W,SOURCE,O/"source-after",critical)
       require(after==before,"BUILD_SOURCE_CHANGED")
       result["sourceAfterSHA256"]=sha((O/"source-after/snapshot.original.json").read_bytes());result["sourceUnchanged"]=True
      except BaseException as exc:
       result["sourceAfterError"]={"type":type(exc).__name__,"message":str(exc)[:600]};result["status"]="FAIL"
     else:result["sourceUnchanged"]=False;result["status"]="FAIL"
     signal.setitimer(signal.ITIMER_REAL,0)
     for kind,handler in handlers.items():signal.signal(kind,handler)
     result.update(firstInterruption=latch.reason,minimumDataFreeBytes=minimum,freeAfter=shutil.disk_usage("/System/Volumes/Data").free,elapsedMs=int((time.monotonic()-started)*1000),completedAt=binding.utc())
     pin=record(O/"result.original.json",result)
     print(json.dumps({"status":result["status"],"receipt":str(O/"result.original.json"),**pin,"elapsedMs":result["elapsedMs"],"error":result.get("error"),"sourceAfterError":result.get("sourceAfterError"),"nativeInvocation":False}))

    return 0 if result['status'] == 'BUILD_VALIDATED' else 1


if __name__ == '__main__':
    raise SystemExit(main())
