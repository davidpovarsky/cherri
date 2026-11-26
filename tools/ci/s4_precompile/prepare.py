"""Provision exact public S4 build prerequisites; no private input or test execution."""
from pathlib import Path, PurePosixPath
import hashlib
import importlib.util
import json
import os
import stat
import sys
import time
import zipfile

P = Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location("s4_precompile_hosted_prepare", P/"hosted_source.py")
fixed = importlib.util.module_from_spec(spec)
sys.modules[spec.name] = fixed
spec.loader.exec_module(fixed)
TOOLCHAIN_URL = "https://proxy.golang.org/golang.org/toolchain/@v/v0.0.1-go1.24.13.linux-arm64.zip"


def extract_toolchain(owner, archive):
    raw = owner.read(archive)
    fixed.require(len(raw) == fixed.TOOLCHAIN_ZIP_BYTES and fixed.sha(raw) == fixed.TOOLCHAIN_ZIP_SHA256,
                  "EXACT_TOOLCHAIN_ARCHIVE")
    destination = fixed.BASE/"toolchain"
    destination.mkdir(mode=0o700)
    rows, names, total = [], set(), 0
    prefix = PurePosixPath(fixed.TOOLCHAIN_PREFIX).parts
    with zipfile.ZipFile(archive) as source:
        entries = source.infolist()
        fixed.require(len(entries) == fixed.TOOLCHAIN_ENTRIES, "TOOLCHAIN_MEMBER_COUNT")
        for index, item in enumerate(entries):
            owner.budget.check()
            if index % 128 == 0:
                owner.monitor()
            relative = PurePosixPath(item.filename)
            mode = item.external_attr >> 16
            fixed.require(not relative.is_absolute() and relative.parts[:len(prefix)] == prefix and
                          len(relative.parts) > len(prefix) and
                          all(part not in ("", ".", "..") for part in relative.parts) and
                          item.filename not in names and not item.is_dir() and
                          stat.S_ISREG(mode) and mode & 0o7777 in (0o644, 0o755), "REGULAR_TOOLCHAIN_MEMBER")
            names.add(item.filename)
            total += item.file_size
            fixed.require(0 <= item.file_size <= 64*1024*1024 and total <= fixed.TOOLCHAIN_EXPANDED_BYTES,
                          "TOOLCHAIN_EXPANSION_BOUND")
            data = source.read(item)
            fixed.require(len(data) == item.file_size, "TOOLCHAIN_MEMBER_SIZE")
            target = destination/relative
            target.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
            with target.open("xb") as stream:
                stream.write(data)
            os.chmod(target, mode & 0o777)
            fixed.require(fixed.sha(owner.read(target)) == fixed.sha(data), "TOOLCHAIN_READBACK")
            rows.append({"path": item.filename, "bytes": len(data), "sha256": fixed.sha(data),
                         "mode": mode & 0o777})
    fixed.require(total == fixed.TOOLCHAIN_EXPANDED_BYTES and fixed.sha(owner.read(fixed.GO)) == fixed.GO_SHA256,
                  "EXACT_EXPANDED_TOOLCHAIN")
    fixed.owned.original(owner.out/"toolchain.original.json", {"url": TOOLCHAIN_URL,
        "archiveSHA256": fixed.sha(raw), "archiveBytes": len(raw), "memberCount": len(rows),
        "expandedBytes": total, "goSHA256": fixed.GO_SHA256, "files": rows})


def main():
    fixed.require(len(sys.argv) == 1, "FIXED_PREREQUISITE_ROUTE")
    os.umask(0o077)
    with fixed.owned.WholeBudget(360, cleanup_seconds=8, final_reserve=25) as budget:
        fixed.BASE.mkdir(mode=0o700)
        fixed.EVIDENCE.mkdir(mode=0o700)
        output = fixed.EVIDENCE/"prerequisite"
        output.mkdir(mode=0o700)
        owner = fixed.SourceOwner({"worktree": str(fixed.WORKTREE)}, output, budget)
        failures, processes = [], []
        before = after = controller_before = controller_after = None
        fixed.owned.original(output/"hosted-runtime.original.json", fixed.runtime_identity())
        for name in ("prepare.py", "hosted_source.py", "owned_source_process.py"):
            fixed.owned.original(output/name, owner.read(P/name))
        try:
            controller_before = fixed.controller_snapshot(owner)
            fixed.owned.original(output/"controller-before.original.json", controller_before)
            before = owner.snapshot()
            fixed.owned.original(output/"source-before.original.json", before)
            fixed.require((before["head"], before["tree"], before["branch"], before["status"]) ==
                          (fixed.SOURCE, fixed.TREE, "", ""), "FRESH_DETACHED_PRODUCER")
            origin = owner.git("remote", "get-url", "origin").decode().strip()
            fixed.require(origin in ("https://github.com/davidpovarsky/cherri", "https://github.com/davidpovarsky/cherri.git"),
                          "PUBLIC_CREDENTIAL_FREE_ORIGIN")
            _, observed, errors = owner.observe_concurrent_owner()
            decision = json.loads(owner.read(observed))
            fixed.require(not owner.read(errors) and decision["admitted"] is True and decision["blockedPids"] == [],
                          "PREREQUISITE_EXCLUSIVE_GO_ADMISSION")
            resources = fixed.free()
            fixed.owned.original(output/"capacity.original.json", {"observed": resources,
                "admission": fixed.ADMISSION_FLOORS, "live": fixed.LIVE_FLOORS})
            fixed.require(all(resources[key] >= floor for key, floor in fixed.ADMISSION_FLOORS.items()),
                          "PREREQUISITE_CAPACITY_ADMISSION")
            owner.git("switch", "--no-track", "-c", fixed.SOURCE_BRANCH, fixed.SOURCE)
            for name in ("downloads", "module-cache", "build-cache", "tmp"):
                (fixed.BASE/name).mkdir(mode=0o700)
            archive = fixed.BASE/"downloads/toolchain.original.zip"
            row, _, _ = owner.command("public-toolchain-download", ["curl", "--fail", "--location", "--silent",
                "--show-error", "--max-time", "120", "--max-filesize", str(fixed.TOOLCHAIN_ZIP_BYTES),
                "--output", str(archive), TOOLCHAIN_URL], seconds=120)
            processes.append(row)
            extract_toolchain(owner, archive)
            row, stdout, _ = owner.command("pinned-go-version", [fixed.GO, "version"], seconds=10)
            processes.append(row)
            fixed.require(owner.read(stdout).strip() == b"go version go1.24.13 linux/arm64", "EXACT_GO_VERSION")
            # Only this prerequisite phase permits public dependency transport.
            # The original producer always receives GOPROXY=off/GOSUMDB=off.
            owner.env.update(GOPROXY="https://proxy.golang.org", GOSUMDB="sum.golang.org")
            try:
                row, _, _ = owner.command("original-module-download", [fixed.GO, "mod", "download", "-json"], seconds=120)
                processes.append(row)
                row, _, _ = owner.command("original-module-verify", [fixed.GO, "mod", "verify"], seconds=60)
                processes.append(row)
            finally:
                owner.env.update(GOPROXY="off", GOSUMDB="off")
        except BaseException as error:
            failures.append({"stage": "prerequisite", "type": type(error).__name__, "message": str(error)})
        finally:
            with budget.finalizing():
                try:
                    after = owner.snapshot(finalizing=True)
                    fixed.owned.original(output/"source-after.original.json", after)
                    fixed.require(before is not None and after["trackedPhysicalInputs"] == before["trackedPhysicalInputs"] and
                                  (after["head"], after["tree"], after["branch"], after["status"]) ==
                                  (fixed.SOURCE, fixed.TREE, fixed.SOURCE_BRANCH, ""), "PREREQUISITE_SOURCE_CHANGED")
                    controller_after = fixed.controller_snapshot(owner)
                    fixed.owned.original(output/"controller-after.original.json", controller_after)
                    fixed.require(controller_before == controller_after, "PREREQUISITE_CONTROLLER_CHANGED")
                except BaseException as error:
                    failures.append({"stage": "post-source", "type": type(error).__name__, "message": str(error)})
                if budget.cancel_signal is not None or time.monotonic() >= budget.deadline:
                    failures.append({"stage": "owner", "type": "CANCELLED_OR_DEADLINE", "signal": budget.cancel_signal})
                result = {"schema": "cherri-s4-precompile-hosted-prerequisite/1", "status": "PASS" if not failures else "FAIL",
                    "source": fixed.SOURCE, "tree": fixed.TREE, "controllerSha": fixed.CONTEXT["controllerSha"],
                    "processes": processes, "failures": failures, "minimumFree": owner.minimum,
                    "wholeOwnerSeconds": 360, "elapsedSeconds": time.monotonic()-budget.started,
                    "privateTestProducerExecuted": False, "goBuildOrTestExecuted": False,
                    "recordedSuccessfulGoCommands": [p["argv"] for p in processes if p["argv"][0] == str(fixed.GO)],
                    "sourceUnchanged": before is not None and after is not None and
                                       before["trackedPhysicalInputs"] == after["trackedPhysicalInputs"],
                    "controllerUnchanged": controller_before is not None and controller_before == controller_after}
                with budget.defer():
                    pin = fixed.owned.original(output/"result.original.json", result)
    print(json.dumps({"path": str(output/"result.original.json"), "sha256": pin["sha256"], "status": result["status"]}), flush=True)
    return 0 if result["status"] == "PASS" else 1



def seal():
    """Transport only public precompile originals and an actually verified binary."""
    fixed.require(len(sys.argv) == 2 and sys.argv[1] == "--seal", "FIXED_PUBLIC_SEAL_ROUTE")
    os.umask(0o077)
    with fixed.owned.WholeBudget(120, cleanup_seconds=8, final_reserve=10) as budget:
        fixed.EVIDENCE.mkdir(mode=0o700, parents=True, exist_ok=True)
        output = fixed.EVIDENCE/"seal"
        output.mkdir(mode=0o700)
        fixed.owned.original(output/"runtime-binding.original.json", fixed.runtime_identity())
        result_path = fixed.EVIDENCE/"precompile/result.original.json"
        delivery = {"status": "NO_SUCCESSFUL_BINARY", "files": [], "privateInputsTransferred": False}
        if result_path.is_file():
            original = result_path.read_bytes()
            result = json.loads(original)
            delivery["precompileResultSHA256"] = fixed.sha(original)
            delivery["precompileStatus"] = result.get("status")
            if result.get("status") == "PASS_PUBLIC_PRECOMPILE_ONLY":
                fixed.require(result["source"] == fixed.SOURCE and result["tree"] == fixed.TREE and
                              result["controller"] == fixed.CONTEXT["controllerSha"] and
                              result["compileOnly"] is True and result["testMethodsExecuted"] == 0 and
                              result["privateCorpusTransferred"] is False and result["sourceUnchanged"] is True and
                              result["controllerUnchanged"] is True and result["failures"] == [],
                              "EXACT_SUCCESSFUL_PUBLIC_BUILD")
                pin = result["binary"]
                binary = fixed.BASE/"products/cherri.test"
                fixed.require(pin["path"] == str(binary) and binary.resolve(strict=True) == binary,
                              "EXACT_BUILT_BINARY_PATH")
                info = binary.lstat()
                fixed.require(stat.S_ISREG(info.st_mode) and info.st_uid == os.getuid() and
                              bool(info.st_mode & 0o111) and 0 < info.st_size <= 128*1024*1024,
                              "REGULAR_OWNED_EXECUTABLE")
                data = binary.read_bytes()
                after = binary.lstat()
                keys = ("st_dev", "st_ino", "st_mode", "st_uid", "st_gid", "st_size", "st_mtime_ns", "st_ctime_ns")
                fixed.require(all(getattr(info, k) == getattr(after, k) for k in keys), "STABLE_EXPORT_SOURCE")
                fixed.require(len(data) == pin["bytes"] and fixed.sha(data) == pin["sha256"] and
                              oct(stat.S_IMODE(info.st_mode)) == pin["mode"] and
                              info.st_uid == pin["uid"] and info.st_gid == pin["gid"], "EXACT_SUCCESSFUL_BINARY")
                destination = fixed.EVIDENCE/"products"
                destination.mkdir(mode=0o700)
                target = destination/"cherri.test"
                with target.open("xb") as stream:
                    stream.write(data)
                target.chmod(stat.S_IMODE(info.st_mode))
                fixed.require(target.read_bytes() == data, "BINARY_EXPORT_READBACK")
                delivery.update(status="EXACT_PUBLIC_TEST_BINARY_RETAINED", files=[{
                    "path": "products/cherri.test", "bytes": len(data), "sha256": fixed.sha(data),
                    "originalMode": pin["mode"], "originalUid": pin["uid"], "originalGid": pin["gid"],
                    "source": fixed.SOURCE, "tree": fixed.TREE}])
        fixed.owned.original(output/"binary-transport.original.json", delivery)
        rows, total = [], 0
        for path in sorted(fixed.EVIDENCE.rglob("*")):
            budget.check()
            relative = path.relative_to(fixed.EVIDENCE)
            fixed.require(relative.parts[0] in ("prerequisite", "precompile", "products", "seal") and
                          ".git" not in relative.parts, "CLOSED_PUBLIC_EVIDENCE_NAMESPACE")
            info = path.lstat()
            fixed.require(not path.is_symlink(), "NO_EVIDENCE_SYMLINK")
            if stat.S_ISDIR(info.st_mode):
                continue
            fixed.require(stat.S_ISREG(info.st_mode) and info.st_uid == os.getuid() and
                          info.st_size <= 128*1024*1024, "OWNED_REGULAR_EVIDENCE_FILE")
            raw = path.read_bytes()
            total += len(raw)
            fixed.require(len(rows) < 2048 and total <= 512*1024*1024, "FINITE_EVIDENCE_BOUNDS")
            rows.append({"path": relative.as_posix(), "bytes": len(raw), "sha256": fixed.sha(raw),
                         "mode": oct(stat.S_IMODE(info.st_mode))})
        manifest = {"schema": "cherri-s4-public-originals-manifest/1", "source": fixed.SOURCE,
                    "tree": fixed.TREE, "controller": fixed.CONTEXT["controllerSha"],
                    "files": rows, "totalBytes": total, "privateInputsTransferred": False}
        manifest_pin = fixed.owned.original(output/"manifest.original.json", manifest)
        receipt = {"schema": "cherri-s4-public-originals-seal/1", "status": "SEALED_PUBLIC_ORIGINALS",
                   "source": fixed.SOURCE, "tree": fixed.TREE, "controller": fixed.CONTEXT["controllerSha"],
                   "manifestSHA256": manifest_pin["sha256"], "fileCount": len(rows), "totalBytes": total,
                   "binaryDeliveryStatus": delivery["status"], "privateInputsTransferred": False,
                   "testMethodsExecutedBySeal": 0, "nativeAcceptance": False, "formal179Acceptance": False}
        pin = fixed.owned.original(output/"result.original.json", receipt)
    print(json.dumps({"path": str(output/"result.original.json"), "sha256": pin["sha256"], **receipt}), flush=True)
    return 0


if __name__ == "__main__":
    raise SystemExit(seal() if sys.argv[1:] == ["--seal"] else main())
