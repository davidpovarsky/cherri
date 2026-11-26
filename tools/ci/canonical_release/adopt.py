"""Validate original 56-gate receipts and adopt only the exact C3 report.

This 240-second phase invokes no Go and has no remote write token. It retains
55 local passes plus the original Linux R22 blocker without a native, release
or final179 grant. The sole-parent commit is exported as a a6bf prerequisite bundle.
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
spec = importlib.util.spec_from_file_location("canonical_release_hosted_adoption_protocol", P/"run.py")
protocol = importlib.util.module_from_spec(spec)
sys.modules[spec.name] = protocol
spec.loader.exec_module(protocol)
fixed = protocol.fixed
HEAD, TREE, BRANCH = protocol.HEAD, protocol.TREE, protocol.BRANCH
GENERATED = (protocol.TARGET,)
RAW_PINS = {
    "docs/apple-runtime/p10/p10-505-execution-evidence.json": "dd4532b8ff7d025cd09821979ce785d4ba4f2c4461877686d8d26b4c4779bf28",
    "docs/apple-runtime/p10/p10-505-parameter-contract-evidence.json": "74c5ed6ffb65594937aefcf248cef7f1d5ed47388fc57352c148f478e14f8eb5",
}
C1_GENERATOR_SHA256 = "99c33e84117cd9e048ef701aba24d1b95e75598479c50474e563cb8ead41e392"
C1_FINGERPRINT = "1dd5975e5e36e38eabb4d70eaa2b9e77760084c9fd26b01f7fe86af0b9ba705e"
C2_FINGERPRINT = protocol.DEPENDENCY_FINGERPRINT
PREIMAGE_SHA256 = "ae51a8d9657fc3dc12be440f0d88549b49d27b02c9f1e6f04998ab3718e0946d"
ORIGINAL_INTAKE_SHA256 = "8ac8bbe43ab1bd7449cf1e28b9ea4c0d8fd8de3b60b11b1b33bc63f0227df087"




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


def c1_fingerprint(owner):
    raw = owner.read(fixed.WORKTREE/"tools/p10r3/generate_action_contract_coverage.py")
    fixed.require(fixed.sha(raw) == C1_GENERATOR_SHA256, "ORIGINAL_C1_FINGERPRINT_PRODUCER")
    nodes = [n for n in ast.parse(raw).body if isinstance(n, ast.Assign) and
             any(isinstance(t, ast.Name) and t.id == "SOURCE_GLOBS" for t in n.targets)]
    fixed.require(len(nodes) == 1, "C1_SOURCE_GLOBS_IDENTITY")
    patterns = ast.literal_eval(nodes[0].value)
    names = sorted({p.relative_to(fixed.WORKTREE).as_posix() for pattern in patterns
                    for p in fixed.WORKTREE.glob(pattern) if p.is_file()})
    material = b""
    for name in names:
        owner.budget.check()
        material += (name+":"+fixed.sha(owner.read(fixed.WORKTREE/name).replace(b"\r\n", b"\n"))+"\n").encode()
    fixed.require(not set(protocol.GENERATED).intersection(names), "C2_PRODUCTS_EXCLUDED_FROM_C1")
    return {"fingerprint": fixed.sha(material), "fileCount": len(names), "patterns": patterns}


def verify_bundle(owner, bundle, commit):
    """A delta bundle carries one new head and requires the exact producer parent."""
    reference = "refs/heads/"+protocol.BRANCH
    owner.git("bundle", "verify", str(bundle))
    fixed.require(owner.git("bundle", "list-heads", str(bundle)).decode().strip().split() == [commit, reference],
                  "ONE_EXACT_BUNDLE_HEAD")
    data = owner.read(bundle)
    header = data.split(b"\n\n", 1)[0].decode("utf-8")
    prerequisites = [line[1:].split(" ", 1)[0] for line in header.splitlines() if line.startswith("-")]
    fixed.require(prerequisites == [protocol.HEAD] and 0 < len(data) <= 32*1024*1024,
                  "EXACT_SMALL_PREREQUISITE_BUNDLE")
    return {"path": bundle.name, "bytes": len(data), "sha256": fixed.sha(data),
            "prerequisite": protocol.HEAD, "head": commit, "ref": reference}


def execution_receipts(owner):
    output = fixed.EVIDENCE/"execution"
    raw = owner.read(output/"result.original.json")
    result = protocol.parse_json(raw)
    checks = []
    def need(condition, name):
        owner.budget.check()
        checks.append({"name": name, "passed": bool(condition)})
        fixed.require(condition, name)
    def parsed(path):
        return protocol.parse_json(owner.read(path))
    need(result["schema"] == "cherri-canonical-release-hosted-execution/1" and
         result["status"] == "PASS_LOCAL_WITH_EXTERNAL_BLOCKER" and result["failures"] == [],
         "ACTUAL_SUCCESSFUL_ORIGINAL_EXECUTION")
    need(result["source"] == HEAD and result["tree"] == TREE and result["sourceRole"] == "canonical-precursor" and
         result["controllerSource"] == fixed.CONTEXT["controllerSha"], "EXACT_PRODUCER_AND_CONTROLLER")
    need(result["sourceUnchanged"] is True and result["controllerUnchanged"] is True and
         result["dependenciesUnchanged"] is True, "EXACT_EXECUTION_GUARDS")
    need(result["wholeOwnerSeconds"] == 1200 and result["runnerMaximumSeconds"] == 900 and
         0 < result["elapsedSeconds"] < 1200 and result["nativeAcceptance"] is False and
         result["global179Acceptance"] is False and result["releaseReady"] is False, "ORIGINAL_BOUNDS_AND_SCOPE")
    need(result["ownerSHA256"] == fixed.sha(owner.read(P/"run.py")) == fixed.sha(owner.read(output/"run.py")) and
         result["sharedControllerSHA256"] == fixed.sha(owner.read(P/"hosted_source.py")) ==
         fixed.sha(owner.read(output/"hosted_source.py")) and result["helperSHA256"] == fixed.HELPER_SHA256 ==
         fixed.sha(owner.read(output/"owned_source_process.py")) and
         result["originalOracleOwnerSHA256"] == fixed.ORIGINAL_RELEASE_OWNER_SHA256, "ORIGINAL_OWNER_SOURCE_BINDING")
    before, after = [parsed(output/name) for name in ("source-before.original.json", "source-after.original.json")]
    need(before == after and (before["head"], before["tree"], before["branch"], before["status"]) ==
         (HEAD, TREE, BRANCH, ""), "COMPLETE_PHYSICAL_SOURCE_UNCHANGED")
    deps_before, deps_after = [parsed(output/name) for name in ("dependency-before.original.json", "dependency-after.original.json")]
    need(deps_before == deps_after and deps_before["fingerprint"] == C2_FINGERPRINT, "ORIGINAL_DEPENDENCY_CLOSURE")
    selected = protocol.selection(owner.read(output/"selection.original.json"))
    need(selected["head"] == HEAD and selected["tree"] == TREE, "FIXED_SELECTION")
    plan = parsed(output/"plan.original.json")
    commands = protocol.commands_for(output/"producer-output")
    need(plan["commands"] == commands and plan["wholeOwnerSeconds"] == 1200 and
         plan["childMaximumSeconds"] == [30, 900, 90] and tuple(plan["originalGateIds"]) == protocol.GATE_IDS and
         plan["dependencyFingerprint"] == C2_FINGERPRINT and plan["sourcePins"] == protocol.SOURCE_PINS,
         "ORIGINAL_THREE_COMMAND_PLAN")
    for path, digest in protocol.SOURCE_PINS.items():
        need(before["trackedPhysicalInputs"][path]["sha256"] == digest == fixed.sha(owner.read(fixed.WORKTREE/path)),
             "CURRENT_AND_EXECUTED_SOURCE_PIN:"+path)
    preflight = parsed(output/"preflight.original.json")
    need(preflight["admitted"] is True and preflight["admission"] == fixed.ADMISSION_FLOORS and
         preflight["liveFloor"] == fixed.LIVE_FLOORS and
         all(preflight["free"][k] >= v for k, v in fixed.ADMISSION_FLOORS.items()) and
         all(result["minimumFree"][k] >= v for k, v in fixed.LIVE_FLOORS.items()), "ACTUAL_UNCHANGED_CAPACITY_POLICY")
    processes = []
    for path in sorted((output/"commands").glob("*.process.original.json")):
        process = parsed(path)
        name = process["name"]
        need(re.fullmatch(r"[0-9]{3}-[a-z0-9-]+", name) is not None and
             path.name == name+".process.original.json", "ACTUAL_PROCESS_IDENTITY")
        need(process["status"] == "PASS" and process["exitCode"] == 0 and process["error"] is None and
             process["timedOut"] is False and process["cancelled"] is False and process["cancelSignal"] is None and
             process["wholeDeadlineExceeded"] is False, "ACTUAL_PROCESS_SUCCESS:"+name)
        cleanup = process["cleanup"]
        need(process["pid"] == process["pgid"] == process["sid"] and cleanup["parentReaped"] is True and
             cleanup["noLiveGroupMembers"] is True and cleanup["errors"] == [] and
             cleanup["after"]["state"] in ("ABSENT", "ZOMBIE_ONLY") and cleanup["after"]["live"] is False,
             "ACTUAL_PROCESS_CLEANUP:"+name)
        for channel in ("stdout", "stderr"):
            data = owner.read(output/"commands"/(name+"."+channel+".original"))
            need(process[channel] == {"complete": True, "sha256": fixed.sha(data), "bytes": len(data)},
                 "COMPLETE_ORIGINAL_STREAM:"+name+":"+channel)
        processes.append(process)
    required = result["processes"]
    need(len(required) == 3 and all(row in processes for row in required), "THREE_ORIGINAL_PROCESSES")
    for process, command, seconds in zip(required, commands, (30, 900, 90)):
        need(process["argv"] == command and process["commandSeconds"] == seconds and
             process["wholeOwnerSeconds"] == 1200, "ORIGINAL_CHILD_COMMAND_AND_BOUND")
    observers = [row for row in processes if row["name"].endswith("-concurrent-owner")]
    need(len(observers) == 1 and fixed.sha(owner.read(output/"admission-observer.original.py")) == fixed.OBSERVER_SHA256,
         "ONE_EXACT_PRIVILEGED_OBSERVER")
    observer = parsed(output/"commands"/(observers[0]["name"]+".stdout.original"))
    need(observer["admitted"] is True and observer["blockedPids"] == observer["errors"] == [] and
         observer["observerUid"] == 0 and observer["policy"] == "new-conservative-all-live-go-20261126",
         "STRICT_EXCLUSIVE_ADMISSION")
    prerequisite = owner.read(output/"prerequisite-result.original.json")
    need(prerequisite == owner.read(fixed.EVIDENCE/"prerequisite/result.original.json") and
         protocol.parse_json(prerequisite)["status"] == "PASS", "PREREQUISITE_ORIGINAL_JOIN")
    values = [owner.read(output/name) for name in protocol.PRODUCT_NAMES]
    inventory = protocol.definitions(owner.read(fixed.WORKTREE/protocol.RUNNER))
    review = parsed(fixed.WORKTREE/protocol.REVIEW)
    need(parsed(output/"gate-definitions.original.json") == inventory and
         owner.read(output/"protected-ios-review.original.json") == owner.read(fixed.WORKTREE/protocol.REVIEW),
         "EXACT_GATE_DEFINITIONS_AND_PROTECTED_REVIEW")
    observations = protocol.validate_products(*values, selected, inventory, review)
    need(observations == result["observations"], "COMPLETE_ORIGINAL_REPORT_VALIDATION")
    report, attestation = [protocol.parse_json(data) for data in values]
    need(attestation["command"] == " ".join(commands[1][2:]), "ORIGINAL_ATTESTATION_COMMAND")
    for name, data in zip(protocol.PRODUCT_NAMES, values):
        need(data == owner.read(output/"producer-output"/name) and result["products"][name] ==
             {"state": "RETAINED", "sha256": fixed.sha(data), "bytes": len(data)}, "EXACT_ORIGINAL_PRODUCT:"+name)
    console = owner.read(output/"commands"/(required[1]["name"]+".stdout.original")).decode()
    lines = re.findall(r"^\[(R[0-9]{2}|A[0-9]{2})\] (.*?) \.\.\. (PASS|FAIL|BLOCKED_[A-Z_]+) \(([0-9.]+)s\)$", console, re.M)
    need(len(lines) == 56 and tuple(row[0] for row in lines) == protocol.GATE_IDS, "ACTUAL_CONSOLE_GATE_INVENTORY")
    for actual, row in zip(lines, report["gates"]):
        need(actual[1] == row["name"] and actual[2] == row["status"] and float(actual[3]) == row["durationSeconds"],
             "ACTUAL_CONSOLE_RESULT:"+row["id"])
    need("Starting P10R4 Full Gate Execution (56 gates)" in console and "Passing:           55" in console and
         "Blocked External:  1" in console and "Failing:           0" in console, "ACTUAL_CONSOLE_SUMMARY")
    verify = owner.read(output/"commands"/(required[2]["name"]+".stdout.original")).decode()
    need("SUCCESS: All P10 artifact sourceCommit values verified as ancestors of HEAD ("+HEAD+")" in verify and
         C2_FINGERPRINT in verify, "ORIGINAL_COMMIT_AND_FINGERPRINT_VERIFIER")
    return {"schema": "cherri-canonical-release-hosted-intake/1", "status": "PASS_LOCAL_WITH_EXTERNAL_BLOCKER",
            "source": HEAD, "tree": TREE, "controllerSha": fixed.CONTEXT["controllerSha"],
            "ownerResultSHA256": fixed.sha(raw), "checkCount": len(checks), "checks": checks,
            "reportSHA256": fixed.sha(values[0]), "attestationSHA256": fixed.sha(values[1]),
            "summary": report["summary"], "verdict": report["verdict"],
            "actualCleanupStates": {row["name"]: row["cleanup"]["after"]["state"] for row in processes},
            "originalQualifiedIntakeSHA256": ORIGINAL_INTAKE_SHA256,
            "goRepeated": False, "producerReplayed": False, "nativeAcceptance": False,
            "global179Acceptance": False, "releaseReady": False}


def main():
    fixed.require(len(sys.argv) == 1, "FIXED_ADOPTION_ROUTE")
    with fixed.owned.WholeBudget(240, cleanup_seconds=8, final_reserve=25) as budget:
        output = fixed.EVIDENCE/"adoption"
        output.mkdir(mode=0o700)
        owner = fixed.SourceOwner({"worktree": str(fixed.WORKTREE)}, output, budget)
        failures, commit, tree, bundle_pin = [], None, None, None
        controller_before = controller_after = before = after = intake = None
        c1_before = c2_before = c1_after = c2_after = None
        fixed.owned.original(output/"hosted-runtime.original.json", fixed.runtime_identity())
        fixed.owned.original(output/"adopt.py", owner.read(P/"adopt.py"))
        try:
            controller_before = fixed.controller_snapshot(owner)
            fixed.owned.original(output/"controller-before.original.json", controller_before)
            before = owner.snapshot()
            fixed.owned.original(output/"source-before.original.json", before)
            fixed.require((before["head"], before["tree"], before["branch"], before["status"]) ==
                          (HEAD, TREE, BRANCH, "") and before == protocol.parse_json(owner.read(
                          fixed.EVIDENCE/"execution/source-after.original.json")), "NO_POST_EXECUTION_SOURCE_DRIFT")
            _, observed, errors = owner.observe_concurrent_owner()
            decision = protocol.parse_json(owner.read(observed))
            fixed.require(not owner.read(errors) and decision["admitted"] is True and decision["blockedPids"] == [],
                          "ADOPTION_EXCLUSIVE_GO_ADMISSION")
            resources = fixed.free()
            fixed.owned.original(output/"capacity.original.json", {"observed": resources,
                "admission": fixed.ADMISSION_FLOORS, "live": fixed.LIVE_FLOORS})
            fixed.require(all(resources[key] >= floor for key, floor in fixed.ADMISSION_FLOORS.items()),
                          "ADOPTION_CAPACITY_ADMISSION")
            intake = execution_receipts(owner)
            fixed.owned.original(output/"intake.original.json", intake)
            c1_before, c2_before = c1_fingerprint(owner), owner.dependencies()
            fixed.require(c1_before["fingerprint"] == C1_FINGERPRINT and c2_before["fingerprint"] == C2_FINGERPRINT,
                          "ACTUAL_C1_C2_INPUT_FINGERPRINTS")
            fixed.owned.original(output/"fingerprints-before.original.json", {"c1": c1_before, "c2": c2_before})
            report_raw = owner.read(fixed.EVIDENCE/"execution"/protocol.PRODUCT_NAMES[0])
            target = fixed.WORKTREE/protocol.TARGET
            info = target.lstat()
            fixed.require(stat.S_ISREG(info.st_mode) and info.st_uid == os.getuid() and not (info.st_mode & 0o111),
                          "OWNED_CANONICAL_REPORT")
            preimage = owner.read(target)
            fixed.require(fixed.sha(preimage) == PREIMAGE_SHA256 ==
                          before["trackedPhysicalInputs"][protocol.TARGET]["sha256"] and preimage != report_raw,
                          "EXACT_CANONICAL_PREIMAGE")
            fixed.owned.original(output/"canonical-report-preimage.original.json", preimage)
            fixed.owned.original(output/"canonical-report-successor.original.json", report_raw)
            temporary = target.with_name("."+target.name+".c3-q-adoption")
            with temporary.open("xb") as stream:
                stream.write(report_raw)
                stream.flush()
                os.fsync(stream.fileno())
            os.chmod(temporary, stat.S_IMODE(info.st_mode))
            os.replace(temporary, target)
            fixed.require(owner.read(target) == report_raw, "BYTE_IDENTICAL_ADOPTION")
            unstaged = snapshot_after(owner, before)
            fixed.owned.original(output/"source-adoption.original.json", unstaged)
            for path in (protocol.TARGET, "docs/apple-runtime/p10/p10-505-catalog-coverage-matrix.json",
                         "docs/apple-runtime/p10/p10-apple-cherri-crosswalk.json", "docs/apple-runtime/p10/baseline-state.json"):
                value = protocol.parse_json(owner.read(fixed.WORKTREE/path))
                fixed.require(value["inputDependencyFingerprint"] == C2_FINGERPRINT, "CANONICAL_DEPENDENCY:"+path)
                owner.git("merge-base", "--is-ancestor", value["sourceCommit"], HEAD)
            fixed.require(unstaged["status"].splitlines() == [" M "+protocol.TARGET], "EXACT_SINGLE_FILE_ADOPTION")
            owner.git("diff", "--check", "--", protocol.TARGET)
            owner.git("add", "--", protocol.TARGET)
            fixed.require(owner.git("diff", "--cached", "--name-only").decode().splitlines() == [protocol.TARGET],
                          "ONLY_C3_STAGED_PATH")
            owner.git("-c", "gc.auto=0", "-c", "maintenance.auto=false", "-c", "user.name=Cherri Completion Agent",
                      "-c", "user.email=cherri-completion@users.noreply.github.com", "commit", "-m",
                      "chore: adopt original 55-pass a6bf release report with R22 platform blocker [skip ci]")
            commit, tree = (owner.git("rev-parse", argument).decode().strip() for argument in ("HEAD", "HEAD^{tree}"))
            fixed.require(owner.git("rev-parse", "HEAD^").decode().strip() == HEAD and
                          owner.git("diff-tree", "--no-commit-id", "--name-only", "-r", "HEAD").decode().splitlines() ==
                          [protocol.TARGET], "EXACT_PRODUCER_PARENT_AND_SINGLE_REPORT")
            after = owner.snapshot()
            fixed.owned.original(output/"source-after.original.json", after)
            fixed.require((after["head"], after["tree"], after["branch"], after["status"]) ==
                          (commit, tree, BRANCH, "") and
                          after["trackedPhysicalInputs"] == unstaged["trackedPhysicalInputs"], "ADOPTED_BYTES_UNCHANGED")
            c1_after, c2_after = c1_fingerprint(owner), owner.dependencies()
            fixed.require(c1_before == c1_after and c2_before == c2_after, "ADOPTION_DEPENDENCIES_CHANGED")
            fixed.owned.original(output/"fingerprints-after.original.json", {"c1": c1_after, "c2": c2_after})
            bundle = output/"canonical-release.original.bundle"
            owner.git("bundle", "create", str(bundle), "refs/heads/"+BRANCH, "^"+HEAD)
            bundle_pin = verify_bundle(owner, bundle, commit)
            fixed.owned.original(output/"bundle.original.json", bundle_pin)
        except BaseException as error:
            failures.append({"stage": "adoption", "type": type(error).__name__, "message": str(error)})
        finally:
            with budget.finalizing():
                try:
                    controller_after = fixed.controller_snapshot(owner)
                    fixed.owned.original(output/"controller-after.original.json", controller_after)
                    fixed.require(controller_before == controller_after, "ADOPTION_CONTROLLER_CHANGED")
                    final = owner.snapshot(finalizing=True) if commit is not None else snapshot_after(owner, before)
                    fixed.owned.original(output/"source-final.original.json", final)
                    if after is not None:
                        fixed.require(final == after, "POST_BUNDLE_SOURCE_CHANGED")
                except BaseException as error:
                    failures.append({"stage": "post-source", "type": type(error).__name__, "message": str(error)})
                if budget.cancel_signal is not None or time.monotonic() >= budget.deadline:
                    failures.append({"stage": "owner", "type": "CANCELLED_OR_DEADLINE", "signal": budget.cancel_signal})
                passed = not failures and bundle_pin is not None
                report = {"schema": "cherri-canonical-release-hosted-byte-adoption/1", "status": "PASS" if passed else "FAIL",
                    "producerSource": HEAD, "producerTree": TREE, "controllerSha": fixed.CONTEXT["controllerSha"],
                    "ownerResultSHA256": intake["ownerResultSHA256"] if intake else None,
                    "commit": commit, "tree": tree, "branch": BRANCH, "bundle": bundle_pin,
                    "changedTrackedPaths": list(GENERATED), "wholeOwnerSeconds": 240,
                    "elapsedSeconds": time.monotonic()-budget.started, "minimumFree": owner.minimum,
                    "failures": failures, "goExecuted": False, "remotePushExecuted": False,
                    "nativeAcceptance": False, "global179Acceptance": False, "releaseReady": False,
                    "canonicalVerdict": "PARTIAL_WITH_EXTERNAL_BLOCKERS" if passed else None}
                with budget.defer():
                    pin = fixed.owned.original(output/"result.original.json", report)
    print(json.dumps({"path": str(output/"result.original.json"), "sha256": pin["sha256"],
                      "status": report["status"], "commit": commit}), flush=True)
    return 0 if passed else 1


if __name__ == "__main__":
    raise SystemExit(main())
