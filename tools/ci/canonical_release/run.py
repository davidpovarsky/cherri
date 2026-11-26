"""Run the original 56-gate protocol once on fixed a6bf within 1200 seconds.

The report validator retains the qualified declared-command implementation.
The original runner keeps its 30/900/90 command bounds, complete 56 inventory,
and Linux R22 blocker. Controller and producer are distinct source identities.
"""
from pathlib import Path
import ast
import hashlib
import importlib.util
import json
import math
import os
import re
import stat
import sys
import time

P = Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location("canonical_release_hosted_binding", P/"hosted_source.py")
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
RUNNER = "tools/p10r3/run_release_gates.py"
REVIEW = "docs/apple-runtime/p10/r23-protected-ios-review.json"
SOURCE_PINS = {
    RUNNER: "9cba82f54ec4fa6715fb8fe7b38c3fa238f96fd45d8ba12d5eb1cedf112bacf3",
    "tools/p10r3/protected_ios.py": "e1f6dbceddf6c55aeb0983c8d50155d621157c15776d89b98e02ae707f681b52",
    REVIEW: "b79bfae148919d0a13048326c265c3f2ee57a725883e56a56b8a040aab772a9b",
    "tools/p10r3/generate_catalog_and_pack_ledgers.py": "213153c9db35eb069c93e03acaa4f71f008f51bd078fcaf3ff11e2a95adbf418",
    "docs/apple-runtime/p10/p10-505-catalog-coverage-matrix.json": "bd82630b1a1343e673f2b5577003a65bc37c1b3c7df9186b892c5703a9b83824",
    "docs/apple-runtime/p10/p10-apple-cherri-crosswalk.json": "9797e70f40fcf711f674e91f8e192978fe6077f4a247775322604fa83e9d0437",
    "docs/apple-runtime/p10/baseline-state.json": "2fcad22c0450f673be78accd4e660c117e9a35d87ae053df2db5cd1c5f318581",
}
DEPENDENCY_FINGERPRINT = "c59d5139ccd51c4ceb11f9cddd878aaf26b07a991133bdea17c88607600c1d9e"
GATE_IDS = tuple(f"{prefix}{number:02d}" for prefix in ("R", "A") for number in range(1, 29))
PRODUCT_NAMES = ("p10r4-gate-results.original.json", "p10r4-ci-attestation.original.json")
TARGET = "docs/apple-runtime/p10/p10r3-gate-results.json"
GENERATED = (TARGET,)
PYTHON = "/usr/bin/python3"


def object_pairs(pairs):
    value = {}
    for key, item in pairs:
        fixed.require(key not in value, "RELEASE_DUPLICATE_JSON_KEY")
        value[key] = item
    return value


def parse_json(raw):
    fixed.require(type(raw) is bytes and 0 < len(raw) <= 4*1024*1024, "RELEASE_JSON_BOUND")
    return json.loads(raw, object_pairs_hook=object_pairs,
                      parse_constant=lambda value: (_ for _ in ()).throw(ValueError("NONFINITE_JSON")))


def same_json(left, right):
    return json.dumps(left, sort_keys=True, allow_nan=False) == json.dumps(right, sort_keys=True, allow_nan=False)


def selection(raw):
    value = parse_json(raw)
    fixed.require(type(value) is dict and set(value) == {"schema", "sourceRole", "head", "tree"}, "RELEASE_CLOSED_SELECTION")
    fixed.require(value["schema"] == "cherri-canonical-release-selection/1" and
                  value["sourceRole"] == "canonical-precursor", "RELEASE_SELECTION_SCOPE")
    fixed.require(all(type(value[key]) is str and re.fullmatch(r"[a-f0-9]{40}", value[key]) for key in ("head", "tree")),
                  "RELEASE_SOURCE_IDENTITY")
    return value


def definitions(raw):
    fixed.require(fixed.sha(raw) == SOURCE_PINS[RUNNER], "RELEASE_RUNNER_PIN")
    tree = ast.parse(raw)
    nodes = [node for node in tree.body if isinstance(node, ast.Assign) and
             any(isinstance(target, ast.Name) and target.id == "GATE_DEFINITIONS" for target in node.targets)]
    fixed.require(len(nodes) == 1, "RELEASE_DEFINITION_IDENTITY")
    value = ast.literal_eval(nodes[0].value)
    fixed.require(type(value) is dict and tuple(value) == GATE_IDS, "RELEASE_CLOSED_GATE_INVENTORY")
    fixed.require(sum(row.get("command", [None])[0] == "go" for row in value.values()) == 41 and
                  sum(row.get("command", [None])[0] == "python" for row in value.values()) == 13 and
                  value["R22"].get("isPlatformSimulatorCheck") is True and
                  value["R23"].get("protectedIOSReview") == REVIEW, "RELEASE_ORIGINAL_EXECUTION_ROLES")
    return value


def validate_products(report_raw, attestation_raw, selected, inventory, review):
    report, attestation = parse_json(report_raw), parse_json(attestation_raw)
    fixed.require(type(report) is dict and type(attestation) is dict, "RELEASE_PRODUCT_OBJECTS")
    fixed.require(report.get("milestone") == "P10R4" and report.get("sourceCommit") == selected["head"] and
                  report.get("inputDependencyFingerprint") == DEPENDENCY_FINGERPRINT, "RELEASE_ACTUAL_SOURCE")
    rows = report.get("gates")
    fixed.require(type(rows) is list and len(rows) == 56 and
                  all(type(row) is dict for row in rows) and tuple(row.get("id") for row in rows) == GATE_IDS,
                  "RELEASE_EXACT_ORDERED_56")
    for row in rows:
        expected = inventory[row["id"]]
        fixed.require(row.get("name") == expected["name"] and row.get("sourceFile") == expected["sourceFile"] and
                      row.get("tier") == expected["tier"], "RELEASE_GATE_SOURCE:"+row["id"])
        fixed.require(type(row.get("exitCode")) is int and row["exitCode"] == 0 and "error" not in row,
                      "RELEASE_GATE_PROCESS:"+row["id"])
        fixed.require(type(row.get("durationSeconds")) in (int, float) and
                      math.isfinite(row["durationSeconds"]) and row["durationSeconds"] >= 0 and
                      type(row.get("outputSha256")) is str and re.fullmatch(r"[a-f0-9]{64}", row["outputSha256"]),
                      "RELEASE_GATE_OBSERVATION:"+row["id"])
        if row["id"] == "R22":
            fixed.require(row.get("status") == row.get("blockerClass") == "BLOCKED_PLATFORM" and
                          row.get("blockerCategory") == "BLOCKED_EXTERNAL" and
                          row.get("physicalDeviceBlockerClass") == "BLOCKED_DEVICE" and
                          row.get("outputSha256") == fixed.sha(b"BLOCKED_PLATFORM_AND_BLOCKED_DEVICE") and
                          row.get("command") == "platform=linux (requires darwin + iOS 27 Simulator runtime; physical iPad tracked as BLOCKED_DEVICE)",
                          "RELEASE_NATIVE_BLOCKER_PRESERVED")
        else:
            fixed.require(row.get("status") == "PASS", "RELEASE_LOCAL_GATE_NOT_PASS:"+row["id"])
            if row["id"] != "R23":
                command = list(expected["command"])
                fixed.require(row.get("command") == " ".join(command), "RELEASE_ORIGINAL_COMMAND:"+row["id"])
    r23 = rows[22]
    expected_files = [{"path": row["path"], "baselineBlob": row["baseline"]["blob"],
                       "reviewedBlob": row["reviewed"]["blob"], "baselineTestsRetained": len(row["baseline"]["tests"]),
                       "reviewedTests": len(row["reviewed"]["tests"])} for row in review["files"]]
    fixed.require(r23.get("reviewedCommit") == review["reviewedCommit"] and
                  r23.get("reviewSha256") == SOURCE_PINS[REVIEW] and r23.get("protectedFiles") == expected_files and
                  r23.get("historicalBaselineResult") == "FAIL" and
                  r23.get("scope") == "Reviewed source preservation; iOS execution remains an independent gate",
                  "RELEASE_R23_REVIEW_AND_ORIGINAL_TESTS")
    summary = {"totalGates": 56, "passingGates": 55, "blockedExternalGates": 1, "failingGates": 0,
               "rGatesTotal": 28, "rGatesPassing": 27, "rGatesBlockedExternal": 1,
               "aGatesTotal": 28, "aGatesPassing": 28, "aGatesFailing": 0}
    fixed.require(report.get("summary") == summary and all(type(value) is int for value in report["summary"].values()) and
                  report.get("verdict") == "PARTIAL_WITH_EXTERNAL_BLOCKERS" and
                  report.get("legacyVerdict") == "P10R3_CORRECTIVE_PARTIAL_VERIFIED", "RELEASE_TRUTHFUL_LOCAL_VERDICT")
    keys = ("id", "status", "exitCode", "durationSeconds", "outputSha256")
    fixed.require(attestation.get("schemaVersion") == "1.0" and attestation.get("headCommit") == selected["head"] and
                  attestation.get("inputDependencyFingerprint") == DEPENDENCY_FINGERPRINT and
                  attestation.get("runId") == attestation.get("workflow") == "local" and
                  attestation.get("runnerOs") == "linux" and attestation.get("timestamp") == report.get("timestamp") and
                  same_json(attestation.get("summary"), summary) and attestation.get("verdict") == report["verdict"] and
                  same_json(attestation.get("gates"), [{key: row[key] for key in keys} for row in rows]),
                  "RELEASE_ATTESTATION_EXACT_JOIN")
    fixed.require(type(report.get("timestamp")) is str and report["timestamp"] and
                  attestation.get("toolVersions", {}).get("go") == "go version go1.24.13 linux/arm64",
                  "RELEASE_ACTUAL_TOOLCHAIN")
    return {"summary": summary, "verdict": report["verdict"], "nativeAcceptance": False,
            "global179Acceptance": False, "r23ReviewedCommit": r23["reviewedCommit"]}


def commands_for(product_dir):
    report_path, attestation_path = [product_dir/name for name in PRODUCT_NAMES]
    return [[PYTHON, "-B", RUNNER, "--verify-registry"],
            [PYTHON, "-B", RUNNER, "--run-all", "--out-results", str(report_path),
             "--ci-attestation", str(attestation_path)],
            [PYTHON, "-B", RUNNER, "--verify-commit", "--out-results", str(report_path)]]


def main():
    fixed.require(len(sys.argv) == 1 and sys.platform == "linux", "RELEASE_FIXED_LINUX_NO_ARGUMENT_ROUTE")
    sys.dont_write_bytecode = True
    with fixed.owned.WholeBudget(1200, cleanup_seconds=8, final_reserve=30) as budget:
        fixed.require(OUTPUT.parent.resolve(strict=True) == OUTPUT.parent and
                      not OUTPUT.exists() and not OUTPUT.is_symlink(), "RELEASE_FRESH_OUTPUT")
        OUTPUT.mkdir(mode=0o700)
        product_dir = OUTPUT/"producer-output"
        product_dir.mkdir(mode=0o700)
        owner = fixed.SourceOwner({"worktree": str(WORKTREE)}, OUTPUT, budget)
        selected = selection(json.dumps({"schema": "cherri-canonical-release-selection/1",
            "sourceRole": "canonical-precursor", "head": HEAD, "tree": TREE}).encode())
        report_path, attestation_path = [product_dir/name for name in PRODUCT_NAMES]
        commands = commands_for(product_dir)
        before = after = dependency_before = dependency_after = observations = None
        controller_before = controller_after = None
        processes, failures, products = [], [], {}
        fixed.owned.original(OUTPUT/"selection.original.json", selected)
        fixed.owned.original(OUTPUT/"hosted-runtime.original.json", fixed.runtime_identity())
        fixed.owned.original(OUTPUT/"plan.original.json", {"commands": commands,
            "childMaximumSeconds": [30, 900, 90], "wholeOwnerSeconds": 1200,
            "budgetOrigin": "Qualified original owner and original inner commands unchanged",
            "sourcePins": SOURCE_PINS, "dependencyFingerprint": DEPENDENCY_FINGERPRINT,
            "originalGateIds": GATE_IDS, "nativeAcceptance": False, "global179Acceptance": False})
        for name in ("hosted_source.py", "owned_source_process.py", "run.py"):
            fixed.owned.original(OUTPUT/name, owner.read(P/name))
        try:
            controller_before = fixed.controller_snapshot(owner)
            fixed.owned.original(OUTPUT/"controller-before.original.json", controller_before)
            prerequisite_raw = owner.read(fixed.EVIDENCE/"prerequisite/result.original.json")
            prerequisite = parse_json(prerequisite_raw)
            fixed.require(prerequisite["status"] == "PASS" and prerequisite["source"] == HEAD and
                          prerequisite["tree"] == TREE and prerequisite["controllerSha"] == fixed.CONTEXT["controllerSha"]
                          and prerequisite["sourceUnchanged"] and prerequisite["controllerUnchanged"],
                          "RELEASE_EXACT_SUCCESSFUL_PREREQUISITE")
            fixed.owned.original(OUTPUT/"prerequisite-result.original.json", prerequisite_raw)
            before = owner.snapshot()
            fixed.owned.original(OUTPUT/"source-before.original.json", before)
            fixed.require(tuple(before[key] for key in ("head", "tree", "branch", "status")) ==
                          (HEAD, TREE, BRANCH, ""), "RELEASE_CLEAN_EXACT_SOURCE")
            fixed.require(before == parse_json(owner.read(fixed.EVIDENCE/"prerequisite/source-after.original.json")),
                          "RELEASE_PREREQUISITE_SOURCE_CONTINUITY")
            remote = owner.git("-c", "credential.helper=", "ls-remote", "--heads", "origin", "refs/heads/"+BRANCH)
            fixed.owned.original(OUTPUT/"remote-ref.original.txt", remote)
            fixed.require(remote.decode().strip().split() == [HEAD, "refs/heads/"+BRANCH],
                          "RELEASE_PUBLISHED_SOURCE")
            for path, digest in SOURCE_PINS.items():
                fixed.require(fixed.sha(owner.read(WORKTREE/path)) == digest, "RELEASE_SOURCE_PIN:"+path)
            inventory = definitions(owner.read(WORKTREE/RUNNER))
            review = parse_json(owner.read(WORKTREE/REVIEW))
            fixed.owned.original(OUTPUT/"gate-definitions.original.json", inventory)
            fixed.owned.original(OUTPUT/"protected-ios-review.original.json", owner.read(WORKTREE/REVIEW))
            dependency_before = owner.dependencies()
            fixed.owned.original(OUTPUT/"dependency-before.original.json", dependency_before)
            fixed.require(dependency_before["fingerprint"] == DEPENDENCY_FINGERPRINT,
                          "RELEASE_C2_FINGERPRINT")
            fixed.require(fixed.sha(owner.read(fixed.GO)) == fixed.GO_SHA256, "RELEASE_GO_BINARY_PIN")
            _, observer_stdout, observer_stderr = owner.observe_concurrent_owner()
            decision = parse_json(owner.read(observer_stdout))
            fixed.require(not owner.read(observer_stderr) and decision.get("admitted") is True and
                          decision.get("blockedPids") == [], "RELEASE_EXCLUSIVE_GO_ADMISSION")
            actual_free = fixed.free()
            admitted = all(actual_free[key] >= value for key, value in fixed.ADMISSION_FLOORS.items())
            fixed.owned.original(OUTPUT/"preflight.original.json", {"free": actual_free,
                "admission": fixed.ADMISSION_FLOORS, "liveFloor": fixed.LIVE_FLOORS, "admitted": admitted})
            fixed.require(admitted, "RELEASE_CAPACITY_ADMISSION")
            for label, command, seconds in zip(("release-registry", "release56", "release-commit-verification"),
                                                commands, (30, 900, 90)):
                row, _, _ = owner.command(label, command, seconds, require_zero=False)
                processes.append(row)
                fixed.owned.require_success(row)
            values = [owner.read(path) for path in (report_path, attestation_path)]
            observations = validate_products(*values, selected, inventory, review)
            fixed.require(parse_json(values[1]).get("command") == " ".join(commands[1][2:]),
                          "RELEASE_EFFECTIVE_COMMAND")
        except BaseException as error:
            failures.append({"stage": "execution", "type": type(error).__name__, "message": str(error)})
        finally:
            with budget.finalizing():
                try:
                    for path in (report_path, attestation_path):
                        if not path.exists() and not path.is_symlink():
                            products[path.name] = {"state": "ABSENT"}
                            continue
                        info = path.lstat()
                        fixed.require(stat.S_ISREG(info.st_mode) and 0 < info.st_size <= 4*1024*1024,
                                      "RELEASE_PRODUCT_TYPE_BOUND")
                        products[path.name] = {"state": "RETAINED",
                            **fixed.owned.original(OUTPUT/path.name, owner.read(path))}
                    fixed.owned.original(OUTPUT/"products.original.json", products)
                except BaseException as error:
                    failures.append({"stage": "product-retention", "type": type(error).__name__, "message": str(error)})
                try:
                    if before is not None:
                        after = owner.snapshot(finalizing=True)
                        fixed.owned.original(OUTPUT/"source-after.original.json", after)
                        fixed.require(before == after, "RELEASE_SOURCE_CHANGED")
                    if dependency_before is not None:
                        dependency_after = owner.dependencies()
                        fixed.owned.original(OUTPUT/"dependency-after.original.json", dependency_after)
                        fixed.require(dependency_before == dependency_after, "RELEASE_DEPENDENCY_CHANGED")
                    controller_after = fixed.controller_snapshot(owner)
                    fixed.owned.original(OUTPUT/"controller-after.original.json", controller_after)
                    fixed.require(controller_before == controller_after, "RELEASE_CONTROLLER_CHANGED")
                except BaseException as error:
                    failures.append({"stage": "post-source", "type": type(error).__name__, "message": str(error)})
                if budget.cancel_signal is not None or time.monotonic() >= budget.deadline:
                    failures.append({"stage": "owner", "type": "CANCELLED_OR_DEADLINE", "signal": budget.cancel_signal})
                passed = not failures and len(processes) == 3 and observations is not None and before is not None and after is not None
                result = {"schema": "cherri-canonical-release-hosted-execution/1", "source": HEAD, "tree": TREE,
                    "controllerSource": fixed.CONTEXT["controllerSha"], "sourceRole": selected["sourceRole"],
                    "status": "PASS_LOCAL_WITH_EXTERNAL_BLOCKER" if passed else "FAIL" if processes else "NOT_RUN",
                    "failures": failures, "processes": processes, "products": products, "observations": observations,
                    "wholeOwnerSeconds": 1200, "runnerMaximumSeconds": 900,
                    "elapsedSeconds": time.monotonic()-budget.started, "minimumFree": owner.minimum,
                    "sourceUnchanged": before is not None and before == after,
                    "controllerUnchanged": controller_before is not None and controller_before == controller_after,
                    "dependenciesUnchanged": dependency_before is not None and dependency_before == dependency_after,
                    "nativeAcceptance": False, "global179Acceptance": False, "releaseReady": False,
                    "ownerSHA256": fixed.sha(Path(__file__).read_bytes()), "sharedControllerSHA256": SHARED_SHA256,
                    "helperSHA256": HELPER_SHA256, "originalOracleOwnerSHA256": fixed.ORIGINAL_RELEASE_OWNER_SHA256}
                with budget.defer():
                    pin = fixed.owned.original(OUTPUT/"result.original.json", result)
    print(json.dumps({"result": str(OUTPUT/"result.original.json"), "sha256": pin["sha256"],
                      "status": result["status"]}), flush=True)
    return 0 if passed else 1


if __name__ == "__main__":
    raise SystemExit(main())
