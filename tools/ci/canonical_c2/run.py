"""Fixed original C2 generation, seven output paths and one 600-second owner.

The qualified process/source/admission helpers are unchanged. This route does
not build a release pair or grant native/final acceptance. Actual output bytes
are retained even on failure; every other tracked physical input stays exact.
"""
from pathlib import Path
import hashlib
import importlib.util
import json
import os
import stat
import sys
import time

P = Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location("canonical_c2_hosted_binding", P/"hosted_source.py")
fixed = importlib.util.module_from_spec(spec)
sys.modules[spec.name] = fixed
spec.loader.exec_module(fixed)
SHARED_SHA256 = fixed.sha((P/"hosted_source.py").read_bytes())
HELPER_SHA256 = fixed.HELPER_SHA256
WORKTREE = fixed.WORKTREE
HEAD = fixed.SOURCE
TREE = fixed.TREE
BRANCH = fixed.SOURCE_BRANCH
OUTPUT = fixed.EVIDENCE/"execution"
GENERATOR = "tools/p10r3/generate_catalog_and_pack_ledgers.py"
GENERATOR_SHA256 = "213153c9db35eb069c93e03acaa4f71f008f51bd078fcaf3ff11e2a95adbf418"
C1_SOURCE = "0f078655307b93f06dba594d834c37c5d689b450"
C1_PINS = {
    "docs/apple-runtime/p10r15/action-contract-execution.json": "2ae9236c4b43007fa5d8e1df95c8eba6f1de191b396dd9ca03ae09e079989b85",
    "docs/apple-runtime/p10r15/action-contract-coverage.json": "d2767777e4e8492e6e487150856e7e1cc11ca900229d33ea9ff5d7e2d3696c65",
    "docs/apple-runtime/p10r15/action-contract-coverage.md": "14ab613d6e8ea0cc286fe62f8127325852831683cb8e03a5ad61e1dd012036d5",
}
GENERATED = (
    "docs/apple-runtime/p10/p10-505-catalog-coverage-matrix.json",
    "docs/apple-runtime/p10/p10-pack-15-go-only-audit.json",
    "docs/apple-runtime/p10/p10-apple-cherri-crosswalk.json",
    "docs/apple-runtime/p10/catalog-supported-by-compiler.md",
    "docs/apple-runtime/p10/apple-discovered-not-supported.md",
    "docs/apple-runtime/p10/pure-pack-portability.md",
    "docs/apple-runtime/p10/baseline-state.json",
)
RAW_PINS = {
    "docs/apple-runtime/p10/p10-505-execution-evidence.json": "dd4532b8ff7d025cd09821979ce785d4ba4f2c4461877686d8d26b4c4779bf28",
    "docs/apple-runtime/p10/p10-505-parameter-contract-evidence.json": "74c5ed6ffb65594937aefcf248cef7f1d5ed47388fc57352c148f478e14f8eb5",
}
UPSTREAM = "bcc0095754e2cc0baaeb4d93dae172b8153b4716"
PACK = "141b9cc96cf1df3f9d1593548dc49ae8a46a256a"
DECLARATIONS = ("statistic", "calculate", "getFileDetail", "calculationOperation")
COMMAND = ["/usr/bin/python3", "-B", GENERATOR]

def validate_post_state(before, after):
    """Validate actual observed bytes; never substitute original output bytes."""
    fixed.require(tuple(before[k] for k in ("head", "tree", "branch", "status")) == (HEAD, TREE, BRANCH, ""), "C2_BEFORE_IDENTITY")
    fixed.require(tuple(after[k] for k in ("head", "tree", "branch")) == (HEAD, TREE, BRANCH), "C2_AFTER_IDENTITY")
    changes = after["status"].splitlines()
    fixed.require(len(changes) == len(set(changes)) and all(line in {" M "+p for p in GENERATED} for line in changes), "C2_UNDECLARED_OR_INDEX_CHANGE")
    old, current = before["trackedPhysicalInputs"], after["trackedPhysicalInputs"]
    fixed.require(set(old) == set(current), "C2_TRACKED_INVENTORY_CHANGED")
    fixed.require(set(GENERATED).issubset(old), "C2_MISSING_DECLARED_PRODUCT")
    for name, row in current.items():
        if name in GENERATED:
            fixed.require(row["mode"] == old[name]["mode"] == "100644" and 0 < row["bytes"] <= 8*1024*1024,
                          "C2_PRODUCT_TYPE_OR_BOUND")
        else:
            fixed.require(row == old[name], "C2_NONPRODUCT_SOURCE_CHANGED:"+name)
    for path, digest in RAW_PINS.items():
        fixed.require(old[path]["sha256"] == current[path]["sha256"] == digest, "C2_RAW_C0_CHANGED")
    return sorted(name for name in old if old[name] != current[name])

def snapshot_after(owner, before):
    """Strict physical snapshot permitting only exact C2 tracked output paths."""
    w = owner.worktree
    fixed.require(w.resolve(strict=True) == w and (w/".git").is_dir() and not (w/".git").is_symlink(), "ORDINARY_SOURCE_REQUIRED")
    fixed.require(not (w/".git/objects/info/alternates").exists() and not (w/".git/info/grafts").exists(), "NO_ALTERNATES_OR_GRAFTS")
    fixed.require(not owner.git("replace", "-l", finalizing=True), "NO_REPLACE_REFS")
    metadata = {}
    for key, args in (("head", ("rev-parse", "HEAD")), ("tree", ("rev-parse", "HEAD^{tree}")),
                      ("branch", ("branch", "--show-current")), ("status", ("status", "--porcelain=v1", "--untracked-files=all"))):
        raw = owner.git(*args, finalizing=True).decode()
        metadata[key] = raw if key == "status" else raw.strip()
    fixed.require(not owner.git("diff", "--cached", "--name-only", finalizing=True), "C2_INDEX_CHANGED")
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
        fixed.require(name not in tracked and kind == "blob" and mode in ("100644", "100755", "120000"), "C2_TRACKED_FILE_KIND")
        fixed.require(name in before["trackedPhysicalInputs"] and
                      (mode, blob) == tuple(before["trackedPhysicalInputs"][name][k] for k in ("mode", "blob")), "C2_GIT_TREE_CHANGED")
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
            fixed.require(observed_blob == blob, "C2_PHYSICAL_SOURCE_MISMATCH:"+name)
        tracked[name] = {"mode": mode, "blob": observed_blob, "bytes": len(raw), "sha256": fixed.sha(raw)}
    metadata["trackedPhysicalInputs"] = tracked
    validate_post_state(before, metadata)
    return metadata

def retain_partial_products(owner):
    directory = owner.out/"observed-products"
    directory.mkdir(mode=0o700)
    inventory = {}
    for path in GENERATED:
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


def validate_product_values(values, fingerprint, raw):
    """Join actual outputs to fixed source and original raw compiler evidence."""
    fixed.require(set(values) == set(GENERATED), "C2_CLOSED_PRODUCT_INVENTORY")
    matrix, audit, crosswalk, baseline = (json.loads(values[GENERATED[i]]) for i in (0, 1, 2, 6))
    for item in (matrix, crosswalk, baseline):
        fixed.require(item["schemaVersion"] == "2.0" and item["sourceCommit"] == HEAD and
                      item["inputDependencyFingerprint"] == fingerprint, "C2_SOURCE_FINGERPRINT_BINDING")
    expected = {r["name"]: r for r in raw["entries"]}
    rows = matrix["entries"]
    fixed.require(len(expected) == len(rows) == matrix["totalCatalogEntries"] == 505 and
                  len({r["cherriName"] for r in rows}) == 505 and
                  {r["cherriName"] for r in rows} == set(expected), "C2_EXACT_505_MEMBERSHIP")
    summary = matrix["summary"]
    fixed.require(summary["callableActions"] == sum(not r["compilerConstruct"] for r in rows) == 497 and
                  summary["compilerConstructs"] == sum(r["compilerConstruct"] for r in rows) == 8,
                  "C2_497_ACTIONS_8_CONSTRUCTS")
    for row in rows:
        original = expected[row["cherriName"]]
        fixed.require(row["shortcutIdentifier"] == original["shortcutIdentifier"] and
                      row["compilerConstruct"] == original["compilerConstruct"], "C2_RAW_IDENTITY")
        for key in ("legacySupported", "v2Supported", "plistIdentifierVerified", "parameterContractVerified",
                    "decompileSupported", "structuralRoundTripPass", "nativeSpecimenParityStatus", "appleRuntimeExecutedStatus"):
            fixed.require(row[key] == original[key], "C2_RAW_RESULT:"+key)
    fixed.require(summary["nativeSpecimenParityCount"] == summary["appleRuntimeExecutedCount"] == 0,
                  "C2_NO_NEW_NATIVE_ACCEPTANCE")
    fixed.require(crosswalk["denominators"] == baseline["counts"] and
                  crosswalk["denominators"]["cherriCatalogTotalEntries"] == 505 and
                  crosswalk["denominators"]["cherriCallableActions"] == 497 and
                  crosswalk["denominators"]["cherriCompilerConstructs"] == 8, "C2_CROSSWALK_COUNT_JOIN")
    fixed.require(baseline["upstreamBaselineCommit"] == UPSTREAM and baseline["purePackCommit"] == PACK,
                  "C2_ORIGINAL_BASELINES")
    fixed.require(audit["summary"]["totalGoOnlyEvaluated"] == len(audit["goOnlyActionsAudit"]) == 15 and
                  len({r["actionName"] for r in audit["goOnlyActionsAudit"]}) == 15, "C2_ORIGINAL_15_ACTIONS")
    preserved = audit["preservedDeclarationsAudit"]
    fixed.require(tuple(r["declaration"] for r in preserved) == DECLARATIONS and
                  audit["summary"]["deliberatelyPreservedDeclarations"] == 4, "C2_EXACT_FOUR_DECLARATIONS")
    for item in preserved:
        fixed.require(item["upstreamSource"]["sourceCommit"] == UPSTREAM and
                      item["currentSource"]["sourceCommit"] == HEAD, "C2_DECLARATION_SOURCE_BINDING")
        if item["declaration"] == "calculationOperation":
            fixed.require("currentSample" not in item, "C2_ENUM_SOURCE_ONLY")
        else:
            sample = item["currentSample"]
            fixed.require(sample == {"path": next(iter(RAW_PINS)), "sha256": RAW_PINS[next(iter(RAW_PINS))],
                          "entry": item["declaration"], "emittedKeysSample": expected[item["declaration"]]["emittedKeysSample"],
                          "scope": "SYNTHETIC_COMPILER_SAMPLE"}, "C2_DECLARATION_RAW_SAMPLE_BINDING")
    for path in GENERATED[3:6]:
        text = values[path].decode("utf-8")
        fixed.require("- **Source Commit**: `"+HEAD+"`" in text and
                      "- **Input Dependency Fingerprint**: `"+fingerprint+"`" in text, "C2_MARKDOWN_SOURCE_BINDING")
    return {"catalogEntries": len(rows), "callableActions": 497, "compilerConstructs": 8,
            "goOnlyActions": 15, "preservedDeclarations": 4, "nativeSpecimenParityCount": 0,
            "appleRuntimeExecutedCount": 0, "preserved": preserved}


def verify_products(owner, fingerprint):
    directory = owner.out/"products"
    directory.mkdir(mode=0o700)
    values, pins = {}, {}
    for path in GENERATED:
        p = owner.worktree/path
        fixed.require(p.is_file() and not p.is_symlink(), "C2_PRODUCT_REGULAR_FILE")
        raw = owner.read(p)
        fixed.require(0 < len(raw) <= 8*1024*1024, "C2_PRODUCT_SIZE")
        raw.decode("utf-8")
        values[path] = raw
        pins[path] = fixed.owned.original(directory/p.name, raw)
    raw = json.loads(owner.read(owner.worktree/next(iter(RAW_PINS))))
    observations = validate_product_values(values, fingerprint, raw)
    source_dir = owner.out/"declaration-sources"
    source_dir.mkdir(mode=0o700)
    witnessed = {}
    for item in observations["preserved"]:
        expected_path = "actions/documents.cherri" if item["declaration"] == "getFileDetail" else "actions/math.cherri"
        for role in ("upstreamSource", "currentSource"):
            witness = item[role]
            fixed.require(witness["path"] == expected_path, "C2_DECLARATION_EXACT_PATH")
            identity = (witness["sourceCommit"], expected_path)
            if identity not in witnessed:
                data = owner.git("show", identity[0]+":"+identity[1])
                fixed.owned.original(source_dir/(role+"-"+Path(expected_path).name), data)
                witnessed[identity] = data
            data = witnessed[identity]
            lines = data.decode("utf-8").splitlines(True)
            line = witness["line"]
            fixed.require(fixed.sha(data) == witness["sha256"] and type(line) is int and 1 <= line <= len(lines),
                          "C2_DECLARATION_ACTUAL_GIT_BLOB")
            fixed.require(type(witness["declaration"]) is str and witness["declaration"] and
                          "".join(lines[line-1:]).startswith(witness["declaration"]), "C2_DECLARATION_LINE_BYTES")
    return {"pins": pins, "observations": observations, "actualSourceWitnesses": len(witnessed)}


def main():
    fixed.require(len(sys.argv) == 1, "C2_FIXED_NO_ARGUMENT_ROUTE")
    with fixed.owned.WholeBudget(600, cleanup_seconds=8, final_reserve=20) as budget:
        fixed.require(OUTPUT.parent.resolve(strict=True) == OUTPUT.parent and not OUTPUT.exists(), "C2_FRESH_OUTPUT")
        OUTPUT.mkdir(mode=0o750)
        owner = fixed.SourceOwner({"worktree": str(WORKTREE)}, OUTPUT, budget)
        before = after = dependency_before = dependency_after = None
        controller_before = controller_after = None
        processes, failures, products = [], [], None
        plan = {"schema": "cherri-canonical-c2-selection/1", "source": HEAD, "tree": TREE, "branch": BRANCH,
                "worktree": str(WORKTREE), "command": COMMAND, "commandMaximumSeconds": 600,
                "wholeOwnerSeconds": 600, "cleanupAndPostSourceIncluded": True, "rawPins": RAW_PINS,
                "generatedPaths": list(GENERATED), "generatorSHA256": GENERATOR_SHA256,
                "c1ProducerSource": C1_SOURCE, "c1Pins": C1_PINS,
                "nativeAcceptance": False, "global179Acceptance": False}
        fixed.owned.original(OUTPUT/"selection.original.json", plan)
        fixed.owned.original(OUTPUT/"hosted-runtime.original.json", fixed.runtime_identity())
        for name in ("hosted_source.py", "owned_source_process.py", "run.py"):
            fixed.owned.original(OUTPUT/name, (P/name).read_bytes())
        try:
            prerequisite_raw = owner.read(fixed.EVIDENCE/"prerequisite/result.original.json")
            prerequisite = json.loads(prerequisite_raw)
            fixed.require(prerequisite.get("status") == "PASS" and prerequisite.get("source") == HEAD and
                          prerequisite.get("controllerSha") == fixed.CONTEXT["controllerSha"], "HOSTED_PREREQUISITES")
            fixed.owned.original(OUTPUT/"prerequisite-result.original.json", prerequisite_raw)
            controller_before = fixed.controller_snapshot(owner)
            fixed.owned.original(OUTPUT/"controller-before.original.json", controller_before)
            before = owner.snapshot()
            fixed.owned.original(OUTPUT/"source-before.original.json", before)
            fixed.require(tuple(before[k] for k in ("head", "tree", "branch", "status")) == (HEAD, TREE, BRANCH, ""), "C2_CLEAN_EXACT_SOURCE")
            remote = owner.git("-c", "credential.helper=", "ls-remote", "--heads", "origin", "refs/heads/"+BRANCH)
            fixed.owned.original(OUTPUT/"remote-ref.original.txt", remote)
            fixed.require(remote.decode().strip().split() == [HEAD, "refs/heads/"+BRANCH], "C2_PUBLISHED_SOURCE")
            for path, digest in {GENERATOR: GENERATOR_SHA256, **RAW_PINS, **C1_PINS}.items():
                fixed.require(fixed.sha(owner.read(WORKTREE/path)) == digest, "C2_INPUT_PIN:"+path)
            owner.git("merge-base", "--is-ancestor", C1_SOURCE, HEAD)
            fixed.require(fixed.sha(owner.read(fixed.GO)) == fixed.GO_SHA256, "C2_GO_BINARY_PIN")
            preimages = OUTPUT/"preimages"
            preimages.mkdir(mode=0o700)
            for path in GENERATED:
                fixed.owned.original(preimages/Path(path).name, owner.read(WORKTREE/path))
            dependency_before = owner.dependencies()
            fixed.require(not (set(GENERATED) & {x["path"] for x in dependency_before["files"]}), "C2_OUTPUT_IN_DEPENDENCY_INPUTS")
            fixed.owned.original(OUTPUT/"dependency-before.original.json", dependency_before)
            _, observer_stdout, observer_stderr = owner.observe_concurrent_owner()
            decision = json.loads(owner.read(observer_stdout))
            fixed.require(not owner.read(observer_stderr) and decision.get("admitted") is True and decision.get("blockedPids") == [], "C2_EXCLUSIVE_GO_ADMISSION")
            floors = {"root": 939524096, "development": 1073741824, "shm": 402653184, "memoryAvailable": 3221225472}
            observed_free = fixed.free()
            admitted = all(observed_free[k] >= v for k, v in floors.items())
            fixed.owned.original(OUTPUT/"preflight.original.json", {"free": observed_free, "admission": floors, "liveFloor": fixed.LIVE_FLOORS, "admitted": admitted})
            fixed.require(admitted, "C2_CAPACITY_ADMISSION")
            row, _, _ = owner.command("canonical-catalog-and-pack", COMMAND, 600, require_zero=False)
            processes.append(row)
            fixed.owned.require_success(row)
            products = verify_products(owner, dependency_before["fingerprint"])
        except BaseException as error:
            failures.append({"stage": "execution", "type": type(error).__name__, "message": str(error)})
        finally:
            with budget.finalizing():
                try:
                    retain_partial_products(owner)
                except BaseException as error:
                    failures.append({"stage": "product-retention", "type": type(error).__name__, "message": str(error)})
                try:
                    controller_after = fixed.controller_snapshot(owner)
                    fixed.owned.original(OUTPUT/"controller-after.original.json", controller_after)
                    fixed.require(controller_before == controller_after, "CONTROLLER_SOURCE_CHANGED")
                    if before is not None:
                        after = snapshot_after(owner, before)
                        fixed.owned.original(OUTPUT/"source-after.original.json", after)
                    if dependency_before is not None:
                        dependency_after = owner.dependencies()
                        fixed.owned.original(OUTPUT/"dependency-after.original.json", dependency_after)
                        fixed.require(dependency_before == dependency_after, "C2_DEPENDENCY_CHANGED")
                except BaseException as error:
                    failures.append({"stage": "post-source", "type": type(error).__name__, "message": str(error)})
                if budget.cancel_signal is not None or time.monotonic() >= budget.deadline:
                    failures.append({"stage": "owner", "type": "CANCELLED_OR_DEADLINE", "signal": budget.cancel_signal})
                passed = not failures and len(processes) == 1 and products is not None and before is not None and after is not None
                report = {"schema": "cherri-canonical-c2-owned-execution/1", "executionHost": "github-hosted-linux-arm64",
                          "controllerSource": fixed.CONTEXT["controllerSha"], "controllerUnchanged": controller_before is not None and controller_before == controller_after, "source": HEAD, "tree": TREE,
                          "status": "PASS" if passed else "FAIL" if processes else "NOT_RUN", "failures": failures,
                          "processes": processes, "products": products, "minimumFree": owner.minimum,
                          "wholeOwnerSeconds": 600, "elapsedSeconds": time.monotonic()-budget.started,
                          "declaredGeneratedPaths": list(GENERATED), "sourceClosureUnchanged": before is not None and after is not None and not any(f["stage"] == "post-source" for f in failures),
                          "dependenciesUnchanged": dependency_before is not None and dependency_before == dependency_after,
                          "nativeAcceptance": False, "global179Acceptance": False,
                          "originalOracleOwnerSHA256": fixed.ORIGINAL_C2_OWNER_SHA256,
                          "ownerSHA256": fixed.sha(Path(__file__).read_bytes()), "sharedControllerSHA256": SHARED_SHA256, "helperSHA256": HELPER_SHA256}
                with budget.defer():
                    pin = fixed.owned.original(OUTPUT/"result.original.json", report)
    print(json.dumps({"result": str(OUTPUT/"result.original.json"), "sha256": pin["sha256"], "status": report["status"]}), flush=True)
    return 0 if passed else 1


if __name__ == "__main__":
    raise SystemExit(main())
