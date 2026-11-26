"""Provision exact public prerequisites; never execute the 56-gate producer here."""
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
spec = importlib.util.spec_from_file_location("canonical_release_hosted_prepare", P/"hosted_source.py")
fixed = importlib.util.module_from_spec(spec)
sys.modules[spec.name] = fixed
spec.loader.exec_module(fixed)
TOOLCHAIN_URL = "https://proxy.golang.org/golang.org/toolchain/@v/v0.0.1-go1.24.13.linux-arm64.zip"
UPSTREAM = "bcc0095754e2cc0baaeb4d93dae172b8153b4716"
PACK = "141b9cc96cf1df3f9d1593548dc49ae8a46a256a"


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
            row, _, _ = owner.command("fixed-public-history", ["git", "--no-replace-objects", "-c", "credential.helper=",
                "fetch", "--no-tags", "origin", UPSTREAM, PACK], seconds=60)
            processes.append(row)
            for commit in (UPSTREAM, PACK):
                fixed.require(owner.git("rev-parse", commit+"^{commit}").decode().strip() == commit,
                              "EXACT_HISTORICAL_COMMIT")
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
                result = {"schema": "cherri-canonical-release-hosted-prerequisite/1", "status": "PASS" if not failures else "FAIL",
                    "source": fixed.SOURCE, "tree": fixed.TREE, "controllerSha": fixed.CONTEXT["controllerSha"],
                    "processes": processes, "failures": failures, "minimumFree": owner.minimum,
                    "wholeOwnerSeconds": 360, "elapsedSeconds": time.monotonic()-budget.started,
                    "canonicalProducerExecuted": False, "goBuildOrTestExecuted": False,
                    "recordedSuccessfulGoCommands": [p["argv"] for p in processes if p["argv"][0] == str(fixed.GO)],
                    "sourceUnchanged": before is not None and after is not None and
                                       before["trackedPhysicalInputs"] == after["trackedPhysicalInputs"],
                    "controllerUnchanged": controller_before is not None and controller_before == controller_after}
                with budget.defer():
                    pin = fixed.owned.original(output/"result.original.json", result)
    print(json.dumps({"path": str(output/"result.original.json"), "sha256": pin["sha256"], "status": result["status"]}), flush=True)
    return 0 if result["status"] == "PASS" else 1


def retain_pair_products(budget):
    """Copy only the successful original pair's seven fixed deliverable files."""
    result_path = fixed.EVIDENCE/"pair/result.original.json"
    if not result_path.is_file():
        return {"status": "NOT_PRODUCED", "files": []}
    result_raw = result_path.read_bytes()
    result = json.loads(result_raw)
    if result.get("status") != "PASS_CANDIDATE_COMPILER_PAIR":
        return {"status": "ORIGINAL_PAIR_NOT_PASS", "files": [],
                "originalResultSHA256": fixed.sha(result_raw), "originalStatus": result.get("status")}
    fixed.require(result["source"] == fixed.SOURCE and result["tree"] == fixed.TREE and
                  result["sourceUnchanged"] is True and result["failures"] == [] and
                  result["ordinaryCheckout"] is True and result["shallow"] is True and
                  set(result["binaries"]) == {"cherri", "cherri-migrate"}, "PAIR_EXPORT_EXACT_SUCCESSFUL_SOURCE")
    pair = fixed.BASE/"pair"
    fixed.require(pair.is_dir() and not pair.is_symlink() and pair.resolve(strict=True) == pair,
                  "PAIR_EXPORT_FIXED_SOURCE_DIRECTORY")
    destination = fixed.EVIDENCE/"pair/products"
    destination.mkdir(mode=0o700)
    expected = {}
    for name, pin in result["binaries"].items():
        fixed.require(pin["source"] == fixed.SOURCE and pin["path"] == str(pair/name),
                      "PAIR_EXPORT_ORIGINAL_BINARY_PATH")
        expected[name] = None
        expected[name+".commit"] = (fixed.SOURCE+"\n").encode()
        expected[name+".sha256"] = (pin["sha256"]+"\n").encode()
    receipt = {"formatVersion": 1, "compilerRepository": "https://github.com/davidpovarsky/cherri.git",
               "compilerCommit": fixed.SOURCE, "binarySha256": result["binaries"]["cherri"]["sha256"],
               "vcsModified": False}
    fixed.require(result["cacheReceipt"] == receipt, "PAIR_EXPORT_ORIGINAL_CACHE_RECEIPT")
    expected["compiler-receipt.json"] = (json.dumps(receipt, sort_keys=True, indent=2, allow_nan=False)+"\n").encode()
    fixed.require(set(expected) == {"cherri", "cherri-migrate", "cherri.commit", "cherri.sha256",
                  "cherri-migrate.commit", "cherri-migrate.sha256", "compiler-receipt.json"},
                  "PAIR_EXPORT_CLOSED_SEVEN_FILE_INVENTORY")
    rows = []
    for name, expected_bytes in sorted(expected.items()):
        budget.check()
        source = pair/name
        info = source.lstat()
        fixed.require(stat.S_ISREG(info.st_mode) and not source.is_symlink() and
                      info.st_uid == os.getuid() and 0 < info.st_size <= 64*1024*1024,
                      "PAIR_EXPORT_REGULAR_BOUNDED_OWNED_FILE")
        data = source.read_bytes()
        after = source.lstat()
        fixed.require((info.st_dev, info.st_ino, info.st_size, info.st_mtime_ns, info.st_mode, info.st_uid, info.st_gid) ==
                      (after.st_dev, after.st_ino, after.st_size, after.st_mtime_ns, after.st_mode, after.st_uid, after.st_gid),
                      "PAIR_EXPORT_STABLE_ORIGINAL")
        if expected_bytes is None:
            pin = result["binaries"][name]
            fixed.require(len(data) == pin["bytes"] and fixed.sha(data) == pin["sha256"] and
                          oct(stat.S_IMODE(info.st_mode)) == pin["mode"] and
                          info.st_uid == pin["uid"] and info.st_gid == pin["gid"],
                          "PAIR_EXPORT_EXACT_ORIGINAL_BINARY")
        else:
            fixed.require(data == expected_bytes and stat.S_IMODE(info.st_mode) == 0o644,
                          "PAIR_EXPORT_EXACT_ORIGINAL_SIDECAR")
        pin = fixed.owned.original(destination/name, data)
        fixed.require((destination/name).read_bytes() == data, "PAIR_EXPORT_BYTE_IDENTICAL_READBACK")
        rows.append({"path": name, **pin, "originalMode": stat.S_IMODE(info.st_mode),
                     "originalUid": info.st_uid, "originalGid": info.st_gid})
    report = {"schema": "cherri-hosted-pair-seven-original-product-transport/1", "status": "PASS",
              "source": fixed.SOURCE, "tree": fixed.TREE, "originalResultSHA256": fixed.sha(result_raw),
              "files": rows, "compilerRebuilt": False, "installed": False,
              "nativeAcceptance": False, "finalTupleAcceptance": False}
    fixed.owned.original(fixed.EVIDENCE/"pair/product-transport.original.json", report)
    return report


def seal():
    fixed.require(sys.argv == [sys.argv[0], "--seal"], "FIXED_SEAL_ROUTE")
    with fixed.owned.WholeBudget(60, cleanup_seconds=8, final_reserve=5) as budget:
        fixed.require(fixed.EVIDENCE.is_dir() and not fixed.EVIDENCE.is_symlink(), "EXISTING_EVIDENCE_DIRECTORY")
        try:
            pair_transport = retain_pair_products(budget)
        except BaseException as error:
            pair_transport = {"status": "FAIL", "type": type(error).__name__, "message": str(error)}
            fixed.owned.original(fixed.EVIDENCE/"pair-product-transport-failure.original.json", pair_transport)
        rows, total = [], 0
        for path in sorted(fixed.EVIDENCE.rglob("*")):
            budget.check()
            info = path.lstat()
            if stat.S_ISDIR(info.st_mode):
                continue
            relative = path.relative_to(fixed.EVIDENCE).as_posix()
            fixed.require(stat.S_ISREG(info.st_mode) and not path.is_symlink() and
                          ".git" not in path.relative_to(fixed.EVIDENCE).parts and
                          relative not in ("artifact-manifest.original.json", "artifact-summary.original.json"), "CLOSED_ARTIFACT_MEMBER")
            fixed.require(len(rows) < 2048 and info.st_size <= 64*1024*1024 and
                          total + info.st_size <= 256*1024*1024, "ARTIFACT_RETENTION_BOUND")
            data = path.read_bytes()
            total += len(data)
            fixed.require(len(data) == info.st_size and path.lstat().st_mtime_ns == info.st_mtime_ns,
                          "ARTIFACT_CHANGED_DURING_SEAL")
            rows.append({"path": relative, "bytes": len(data), "sha256": fixed.sha(data)})
        fixed.owned.original(fixed.EVIDENCE/"artifact-manifest.original.json", {"schema": "cherri-canonical-release-hosted-artifacts/1",
            "controllerSha": fixed.CONTEXT["controllerSha"], "source": fixed.SOURCE, "runId": fixed.CONTEXT["runId"],
            "runAttempt": 1, "files": rows, "totalBytes": total, "tokensOrGitConfigurationRetained": False})
        states = {}
        for phase in ("prerequisite", "execution", "pair-preparation", "pair", "adoption"):
            path = fixed.EVIDENCE/phase/"result.original.json"
            states[phase] = {"status": "NOT_PRODUCED"} if not path.is_file() else {
                "status": json.loads(path.read_bytes())["status"], "sha256": fixed.sha(path.read_bytes())}
        fixed.owned.original(fixed.EVIDENCE/"artifact-summary.original.json", {"phases": states,
            "source": fixed.SOURCE, "controllerSha": fixed.CONTEXT["controllerSha"],
            "pairProductTransport": pair_transport,
            "nativeAcceptance": False, "global179Acceptance": False})
    print(json.dumps({"artifactFiles": len(rows), "artifactBytes": total, "phases": states}), flush=True)
    return 1 if pair_transport["status"] == "FAIL" else 0


if __name__ == "__main__":
    raise SystemExit(seal() if sys.argv[1:] == ["--seal"] else main())
