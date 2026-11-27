"""One fixed original C1 refresh on the actual published C0 successor, with exact generated-output scope.

The unchanged canonical producer owns its 1260/120/120-second children. This
owner bounds that producer to 1500 seconds and the entire observation to 1740.
Only the three declared tracked C1 products may differ after execution. All raw
C0 bytes and every other tracked physical source input remain independently
checked. No caller-supplied command, environment, source, or output is accepted.
"""
from pathlib import Path
import ast
import hashlib
import importlib.util
import json
import os
import re
import stat
import sys
import time

P = Path(__file__).resolve().parent
SHARED_SHA256 = "aea4df358ac64cc671cef07502a88f5fc14813f8413dc495f75281b8be3592e3"
HELPER_SHA256 = "4672474ddb270a5857914041ebb279f20431f87e9da173e6cd8bf83b3fb11c85"
for name, digest in (("hosted_source.py", SHARED_SHA256), ("owned_source_process.py", HELPER_SHA256)):
    if hashlib.sha256((P/name).read_bytes()).hexdigest() != digest:
        raise SystemExit("C1_OWNER_DEPENDENCY_CHANGED")
spec = importlib.util.spec_from_file_location("canonical_c1_fixed_owner", P/"hosted_source.py")
fixed = importlib.util.module_from_spec(spec)
sys.modules[spec.name] = fixed
spec.loader.exec_module(fixed)

WORKTREE, HEAD, TREE, BRANCH = fixed.WORKTREE, fixed.SOURCE, fixed.TREE, fixed.SOURCE_BRANCH
OUTPUT = fixed.EVIDENCE/"execution"
GENERATOR = "tools/p10r3/generate_action_contract_coverage.py"
GENERATOR_SHA256 = "99c33e84117cd9e048ef701aba24d1b95e75598479c50474e563cb8ead41e392"
CATALOG_GENERATOR = "tools/p10r3/generate_catalog_and_pack_ledgers.py"
CATALOG_GENERATOR_SHA256 = "213153c9db35eb069c93e03acaa4f71f008f51bd078fcaf3ff11e2a95adbf418"
TEST_FILE = "p10_catalog_execution_test.go"
TEST_FILE_SHA256 = "c2d717c3ebb3e24e0bec0ed0c3a23ca4b37203c4aab9d1455581c401b6c6c3ab"
GENERATED = ("docs/apple-runtime/p10r15/action-contract-execution.json",
             "docs/apple-runtime/p10r15/action-contract-coverage.json",
             "docs/apple-runtime/p10r15/action-contract-coverage.md")
LOCAL_LOG = "dist/p10r15/catalog-contract-execution.log"
RAW_PINS = fixed.RAW_PINS
TEST_COMMAND = ["go", "test", "-count=1", "-timeout=20m", "-run",
                "^TestP10_VerifyAndRecord505CatalogExecution$", "-v", "."]
COMMAND = ["/usr/bin/python3", "-B", GENERATOR, "--refresh"]


def validate_post_state(before, after):
    """Validate actual observed bytes; never substitute original output bytes."""
    fixed.require(tuple(before[k] for k in ("head", "tree", "branch", "status")) == (HEAD, TREE, BRANCH, ""), "C1_BEFORE_IDENTITY")
    fixed.require(tuple(after[k] for k in ("head", "tree", "branch")) == (HEAD, TREE, BRANCH), "C1_AFTER_IDENTITY")
    changes = after["status"].splitlines()
    fixed.require(len(changes) == len(set(changes)) and all(line in {" M "+p for p in GENERATED} for line in changes), "C1_UNDECLARED_OR_INDEX_CHANGE")
    old, current = before["trackedPhysicalInputs"], after["trackedPhysicalInputs"]
    fixed.require(set(old) == set(current), "C1_TRACKED_INVENTORY_CHANGED")
    fixed.require(set(GENERATED).issubset(old), "C1_MISSING_DECLARED_PRODUCT")
    for name, row in current.items():
        if name in GENERATED:
            fixed.require(row["mode"] == old[name]["mode"] == "100644" and 0 < row["bytes"] <= 8*1024*1024,
                          "C1_PRODUCT_TYPE_OR_BOUND")
        else:
            fixed.require(row == old[name], "C1_NONPRODUCT_SOURCE_CHANGED:"+name)
    for path, digest in RAW_PINS.items():
        fixed.require(old[path]["sha256"] == current[path]["sha256"] == digest, "C1_RAW_C0_CHANGED")
    return sorted(name for name in old if old[name] != current[name])


def snapshot_after(owner, before):
    """Strict physical snapshot permitting only exact C1 tracked output paths."""
    w = owner.worktree
    fixed.require(w.resolve(strict=True) == w and (w/".git").is_dir() and not (w/".git").is_symlink(), "ORDINARY_SOURCE_REQUIRED")
    fixed.require(not (w/".git/objects/info/alternates").exists() and not (w/".git/info/grafts").exists(), "NO_ALTERNATES_OR_GRAFTS")
    fixed.require(not owner.git("replace", "-l", finalizing=True), "NO_REPLACE_REFS")
    metadata = {}
    for key, args in (("head", ("rev-parse", "HEAD")), ("tree", ("rev-parse", "HEAD^{tree}")),
                      ("branch", ("branch", "--show-current")), ("status", ("status", "--porcelain=v1", "--untracked-files=all"))):
        raw = owner.git(*args, finalizing=True).decode()
        metadata[key] = raw if key == "status" else raw.strip()
    fixed.require(not owner.git("diff", "--cached", "--name-only", finalizing=True), "C1_INDEX_CHANGED")
    tracked = {}
    listing = owner.git("ls-tree", "-rz", "--full-tree", "HEAD", finalizing=True)
    for index, line in enumerate(listing.split(b"\0")):
        if not line:
            continue
        owner.budget.check()
        if index % 128 == 0:
            owner.monitor()
        left, name = line.split(b"\t", 1)
        mode, kind, blob = left.decode().split()
        name = name.decode()
        fixed.require(name not in tracked and kind == "blob" and mode in ("100644", "100755", "120000"), "C1_TRACKED_FILE_KIND")
        fixed.require(name in before["trackedPhysicalInputs"] and
                      (mode, blob) == tuple(before["trackedPhysicalInputs"][name][k] for k in ("mode", "blob")), "C1_GIT_TREE_CHANGED")
        path = w/name
        info = path.lstat()
        if mode == "120000":
            fixed.require(stat.S_ISLNK(info.st_mode), "TRACKED_LINK_KIND")
            raw = os.fsencode(os.readlink(path))
        else:
            fixed.require(stat.S_ISREG(info.st_mode) and bool(info.st_mode & 0o111) == (mode == "100755"), "TRACKED_FILE_MODE")
            raw = owner.read(path)
        observed_blob = hashlib.sha1(b"blob " + str(len(raw)).encode() + b"\0" + raw).hexdigest()
        if name not in GENERATED:
            fixed.require(observed_blob == blob, "C1_PHYSICAL_SOURCE_MISMATCH:"+name)
        tracked[name] = {"mode": mode, "blob": observed_blob, "bytes": len(raw), "sha256": fixed.sha(raw)}
    metadata["trackedPhysicalInputs"] = tracked
    validate_post_state(before, metadata)
    return metadata


def c1_fingerprint(owner):
    tree = ast.parse(owner.read(owner.worktree/GENERATOR))
    nodes = [n for n in tree.body if isinstance(n, ast.Assign) and
             any(isinstance(t, ast.Name) and t.id == "SOURCE_GLOBS" for t in n.targets)]
    fixed.require(len(nodes) == 1, "C1_SOURCE_GLOBS_IDENTITY")
    patterns = ast.literal_eval(nodes[0].value)
    names = sorted({p.relative_to(owner.worktree).as_posix() for pattern in patterns
                    for p in owner.worktree.glob(pattern) if p.is_file()})
    digest, rows = hashlib.sha256(), []
    for name in names:
        raw = owner.read(owner.worktree/name)
        normalized = fixed.sha(raw.replace(b"\r\n", b"\n"))
        digest.update((name+":"+normalized+"\n").encode())
        rows.append({"path": name, "bytes": len(raw), "sha256": fixed.sha(raw), "normalizedSHA256": normalized})
    fixed.require(not (set(names) & set(GENERATED)), "C1_PRODUCT_IN_SOURCE_FINGERPRINT")
    return {"fingerprint": digest.hexdigest(), "patterns": patterns, "files": rows}


def verify_products(owner, fingerprint, stdout):
    products = owner.out/"products"
    products.mkdir(mode=0o700)
    values, pins = {}, {}
    for path in (*GENERATED, LOCAL_LOG):
        p = owner.worktree/path
        fixed.require(p.is_file() and not p.is_symlink(), "C1_PRODUCT_REGULAR_FILE")
        raw = owner.read(p)
        fixed.require(0 < len(raw) <= 8*1024*1024, "C1_PRODUCT_SIZE")
        raw.decode("utf-8")
        pins[path] = fixed.owned.original(products/p.name, raw)
        values[path] = raw
    receipt = json.loads(values[GENERATED[0]])
    report = json.loads(values[GENERATED[1]])
    observed = json.loads(stdout)
    fixed.require(receipt["sourceSha"] == report["sourceSha"] == observed["sourceSha"] == HEAD, "C1_PRODUCT_SOURCE")
    fixed.require(receipt["sourceFingerprint"] == report["sourceFingerprint"] == fingerprint, "C1_PRODUCT_FINGERPRINT")
    raw_digest = RAW_PINS["docs/apple-runtime/p10/p10-505-execution-evidence.json"]
    fixed.require(receipt["executionEvidenceSha256"] == report["executionEvidenceSha256"] == raw_digest, "C1_RECEIPT_RAW_BINDING")
    fixed.require(receipt["command"] == TEST_COMMAND and receipt["exitCode"] == 0 and
                  receipt["testExecuted"] == "TestP10_VerifyAndRecord505CatalogExecution", "C1_EXACT_TEST_RECEIPT")
    fixed.require(receipt["localLog"] == LOCAL_LOG and receipt["localLogSha256"] == pins[LOCAL_LOG]["sha256"], "C1_ORIGINAL_LOG_BINDING")
    log = values[LOCAL_LOG].decode()
    runs = re.findall(r"(?m)^=== RUN   (\S+)\s*$", log)
    passed = re.findall(r"(?m)^--- PASS: (\S+) \([0-9.]+s\)\s*$", log)
    fixed.require(runs == passed == ["TestP10_VerifyAndRecord505CatalogExecution"] and
                  re.search(r"(?m)^(?:--- (?:FAIL|SKIP):|FAIL\b)", log) is None, "C1_EXACT_ORIGINAL_TEST_TERMINAL")
    fixed.require(observed["status"] == "PASS" and observed["entries"] == report["totalCatalogEntries"] == len(report["entries"]) == 505,
                  "C1_COMPLETE_ENTRY_COUNT")
    fixed.require(report["summary"]["callableEntries"] == 497 and report["summary"]["compilerConstructs"] == 8, "C1_COMPLETE_DENOMINATOR")
    return {"products": pins, "canonicalStdout": observed, "actualRun": runs, "actualPass": passed}


def retain_partial_products(owner):
    directory = owner.out/"observed-products"
    directory.mkdir(mode=0o700)
    inventory = {}
    for path in (*GENERATED, LOCAL_LOG):
        p = owner.worktree/path
        if not p.exists() and not p.is_symlink():
            inventory[path] = {"state": "ABSENT"}
            continue
        info = p.lstat()
        if not stat.S_ISREG(info.st_mode) or info.st_size > 8*1024*1024:
            inventory[path] = {"state": "UNRETAINED_INVALID_TYPE_OR_SIZE", "mode": info.st_mode, "bytes": info.st_size}
            continue
        inventory[path] = {"state": "RETAINED", **fixed.owned.original(directory/p.name, owner.read(p))}
    fixed.owned.original(directory/"inventory.original.json", inventory)


def main():
    fixed.require(len(sys.argv) == 1, "C1_FIXED_NO_ARGUMENT_ROUTE")
    with fixed.owned.WholeBudget(1740, cleanup_seconds=8, final_reserve=20) as budget:
        fixed.require(OUTPUT.parent.resolve(strict=True) == OUTPUT.parent and not OUTPUT.exists(), "C1_FRESH_OUTPUT")
        OUTPUT.mkdir(mode=0o750)
        owner = fixed.SourceOwner({"worktree": str(WORKTREE)}, OUTPUT, budget)
        before = after = dependency_before = dependency_after = fingerprint_before = fingerprint_after = None
        controller_before = controller_after = None
        processes, failures, products = [], [], None
        plan = {"schema": "cherri-canonical-c1-selection/1", "source": HEAD, "tree": TREE, "branch": BRANCH,
                "worktree": str(WORKTREE), "command": COMMAND, "producerSeconds": 1500, "wholeOwnerSeconds": 1740,
                "canonicalChildSeconds": [1260, 120, 120], "canonicalGoTimeout": "20m", "rawPins": RAW_PINS,
                "generatedPaths": list(GENERATED), "ignoredLocalLog": LOCAL_LOG,
                "generatorSHA256": GENERATOR_SHA256, "catalogGeneratorSHA256": CATALOG_GENERATOR_SHA256,
                "testFileSHA256": TEST_FILE_SHA256, "nativeAcceptance": False, "global179Acceptance": False,
                "controllerSha": fixed.CONTEXT["controllerSha"], "sourceRole": "canonical-precursor",
                "originalC0HandoffSHA256": fixed.C0_HANDOFF_SHA256}
        fixed.owned.original(OUTPUT/"selection.original.json", plan)
        fixed.owned.original(OUTPUT/"hosted-runtime.original.json", fixed.runtime_identity())
        for name in ("hosted_source.py", "owned_source_process.py", "run.py"):
            fixed.owned.original(OUTPUT/name, (P/name).read_bytes())
        try:
            controller_before = fixed.controller_snapshot(owner)
            fixed.owned.original(OUTPUT/"controller-before.original.json", controller_before)
            prerequisite_raw = owner.read(fixed.EVIDENCE/"prerequisite/result.original.json")
            prerequisite = json.loads(prerequisite_raw)
            fixed.require(prerequisite["status"] == "PASS" and prerequisite["failures"] == [] and
                          prerequisite["source"] == HEAD and prerequisite["tree"] == TREE and
                          prerequisite["controllerSha"] == fixed.CONTEXT["controllerSha"] and
                          prerequisite["sourceUnchanged"] is True and prerequisite["controllerUnchanged"] is True,
                          "C1_EXACT_SUCCESSFUL_PREREQUISITE")
            fixed.owned.original(OUTPUT/"prerequisite-result.original.json", prerequisite_raw)
            before = owner.snapshot()
            fixed.owned.original(OUTPUT/"source-before.original.json", before)
            fixed.require(tuple(before[k] for k in ("head", "tree", "branch", "status")) == (HEAD, TREE, BRANCH, ""), "C1_CLEAN_EXACT_SOURCE")
            remote = owner.git("-c", "credential.helper=", "ls-remote", "--heads", "origin", "refs/heads/"+BRANCH)
            fixed.owned.original(OUTPUT/"remote-ref.original.txt", remote)
            fixed.require(remote.decode().strip().split() == [HEAD, "refs/heads/"+BRANCH], "C1_PUBLISHED_SOURCE")
            for path, digest in {GENERATOR: GENERATOR_SHA256, CATALOG_GENERATOR: CATALOG_GENERATOR_SHA256,
                                 TEST_FILE: TEST_FILE_SHA256, **RAW_PINS}.items():
                fixed.require(fixed.sha(owner.read(WORKTREE/path)) == digest, "C1_INPUT_PIN:"+path)
            fixed.require(fixed.sha(owner.read(fixed.GO)) == fixed.GO_SHA256, "C1_GO_BINARY_PIN")
            preimages = OUTPUT/"preimages"
            preimages.mkdir(mode=0o700)
            for path in GENERATED:
                fixed.owned.original(preimages/Path(path).name, owner.read(WORKTREE/path))
            log_path = WORKTREE/LOCAL_LOG
            if log_path.exists() or log_path.is_symlink():
                fixed.require(log_path.is_file() and not log_path.is_symlink(), "C1_LOG_PREIMAGE_TYPE")
                fixed.owned.original(preimages/"catalog-contract-execution.log", owner.read(log_path))
            dependency_before, fingerprint_before = owner.dependencies(), c1_fingerprint(owner)
            fixed.owned.original(OUTPUT/"dependency-before.original.json", dependency_before)
            fixed.owned.original(OUTPUT/"c1-fingerprint-before.original.json", fingerprint_before)
            _, observer_stdout, observer_stderr = owner.observe_concurrent_owner()
            decision = json.loads(owner.read(observer_stdout))
            fixed.require(not owner.read(observer_stderr) and decision.get("admitted") is True and decision.get("blockedPids") == [], "C1_EXCLUSIVE_GO_ADMISSION")
            floors = {"root": 939524096, "development": 1073741824, "shm": 402653184, "memoryAvailable": 3221225472}
            observed_free = fixed.free()
            admitted = all(observed_free[k] >= v for k, v in floors.items())
            fixed.owned.original(OUTPUT/"preflight.original.json", {"free": observed_free, "admission": floors, "liveFloor": fixed.LIVE_FLOORS, "admitted": admitted})
            fixed.require(admitted, "C1_CAPACITY_ADMISSION")
            row, stdout, _ = owner.command("canonical-action-contract", COMMAND, 1500, require_zero=False)
            processes.append(row)
            fixed.owned.require_success(row)
            products = verify_products(owner, fingerprint_before["fingerprint"], owner.read(stdout))
        except BaseException as error:
            failures.append({"stage": "execution", "type": type(error).__name__, "message": str(error)})
        finally:
            with budget.finalizing():
                try:
                    retain_partial_products(owner)
                except BaseException as error:
                    failures.append({"stage": "product-retention", "type": type(error).__name__, "message": str(error)})
                try:
                    if before is not None:
                        after = snapshot_after(owner, before)
                        fixed.owned.original(OUTPUT/"source-after.original.json", after)
                    if dependency_before is not None:
                        dependency_after, fingerprint_after = owner.dependencies(), c1_fingerprint(owner)
                        fixed.owned.original(OUTPUT/"dependency-after.original.json", dependency_after)
                        fixed.owned.original(OUTPUT/"c1-fingerprint-after.original.json", fingerprint_after)
                        fixed.require(dependency_before == dependency_after and fingerprint_before == fingerprint_after, "C1_DEPENDENCY_CHANGED")
                    controller_after = fixed.controller_snapshot(owner)
                    fixed.owned.original(OUTPUT/"controller-after.original.json", controller_after)
                    fixed.require(controller_before == controller_after, "C1_CONTROLLER_CHANGED")
                except BaseException as error:
                    failures.append({"stage": "post-source", "type": type(error).__name__, "message": str(error)})
                if budget.cancel_signal is not None or time.monotonic() >= budget.deadline:
                    failures.append({"stage": "owner", "type": "CANCELLED_OR_DEADLINE", "signal": budget.cancel_signal})
                passed = not failures and len(processes) == 1 and products is not None and before is not None and after is not None
                report = {"schema": "cherri-canonical-c1-owned-execution/1", "source": HEAD, "tree": TREE,
                          "status": "PASS" if passed else "FAIL" if processes else "NOT_RUN", "failures": failures,
                          "processes": processes, "products": products, "minimumFree": owner.minimum,
                          "wholeOwnerSeconds": 1740, "elapsedSeconds": time.monotonic()-budget.started,
                          "declaredGeneratedPaths": list(GENERATED), "sourceClosureUnchanged": before is not None and after is not None and not any(f["stage"] == "post-source" for f in failures),
                          "dependenciesUnchanged": dependency_before is not None and dependency_before == dependency_after,
                          "nativeAcceptance": False, "global179Acceptance": False,
                          "ownerSHA256": fixed.sha(Path(__file__).read_bytes()), "sharedControllerSHA256": SHARED_SHA256, "helperSHA256": HELPER_SHA256,
                          "controllerSource": fixed.CONTEXT["controllerSha"], "sourceRole": "canonical-precursor",
                          "controllerUnchanged": controller_before is not None and controller_before == controller_after,
                          "originalOracleOwnerSHA256": fixed.ORIGINAL_C1_OWNER_SHA256,
                          "originalC0HandoffSHA256": fixed.C0_HANDOFF_SHA256}
                with budget.defer():
                    pin = fixed.owned.original(OUTPUT/"result.original.json", report)
    print(json.dumps({"result": str(OUTPUT/"result.original.json"), "sha256": pin["sha256"], "status": report["status"]}), flush=True)
    return 0 if passed else 1


if __name__ == "__main__":
    raise SystemExit(main())
