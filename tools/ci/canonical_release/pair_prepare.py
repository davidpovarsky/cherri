"""Prepare the separately owned shallow Q source for the original pair route."""
from pathlib import Path
import hashlib
import importlib.util
import json
import os
import sys
import time

P = Path(__file__).resolve().parent
HOSTED_SOURCE_SHA256 = "4f0f26c7e3cd5be0d5104c9c4a8b1e695a315507a2c08175c2349cf9de19c21d"
if hashlib.sha256((P/"hosted_source.py").read_bytes()).hexdigest() != HOSTED_SOURCE_SHA256:
    raise SystemExit("PAIR_PREPARATION_BINDING_CHANGED")
_spec=importlib.util.spec_from_file_location("cherri_hosted_release_source",P/"hosted_source.py")
fixed=importlib.util.module_from_spec(_spec)
sys.modules[_spec.name]=fixed
_spec.loader.exec_module(fixed)

SOURCE = "44707350fb6e52dd0d947256bced105ac762a1ac"
TREE = "07db998051c5950be6326546b7e743d21beb8390"
ORIGIN = "https://github.com/davidpovarsky/cherri.git"
PAIR = fixed.BASE/"pair"
WORKTREE = PAIR/"src"
EVIDENCE = fixed.EVIDENCE/"pair-preparation"


def main():
    fixed.require(len(sys.argv) == 1, "PAIR_PREPARATION_FIXED_ROUTE")
    with fixed.owned.WholeBudget(180, cleanup_seconds=8, final_reserve=20) as budget:
        fixed.require(EVIDENCE.parent.resolve(strict=True) == EVIDENCE.parent and
                      not EVIDENCE.exists() and not EVIDENCE.is_symlink(),
                      "PAIR_PREPARATION_FRESH_EVIDENCE")
        EVIDENCE.mkdir(mode=0o700)
        fixed.owned.original(EVIDENCE/"owner.original.py", Path(__file__).read_bytes())
        owner = fixed.SourceOwner({"worktree": str(fixed.WORKTREE)}, EVIDENCE, budget)
        failures, source, controller = [], None, None
        try:
            fixed.require(fixed.SOURCE == SOURCE and fixed.TREE == TREE,
                          "PAIR_PREPARATION_EXACT_Q_CONTEXT")
            controller = fixed.controller_snapshot(owner)
            fixed.owned.original(EVIDENCE/"controller.original.json", controller)
            fixed.require(owner.git("rev-parse", "HEAD").decode().strip() == SOURCE and
                          owner.git("rev-parse", "HEAD^{tree}").decode().strip() == TREE and
                          not owner.git("status", "--porcelain=v1", "--untracked-files=all"),
                          "PAIR_PREPARATION_EXACT_PRECURSOR")
            fixed.require(not PAIR.exists() and not PAIR.is_symlink() and
                          PAIR.parent.resolve(strict=True) == PAIR.parent,
                          "PAIR_PREPARATION_FRESH_OUTPUT")
            PAIR.mkdir(mode=0o700)
            fixed.require(PAIR.stat().st_dev == fixed.WORKTREE.stat().st_dev,
                          "PAIR_SOURCE_CAPACITY_DEVICE_BINDING")
            commands = [
                ("pair-init", ["git", "init", str(WORKTREE)], 25),
                ("pair-origin", ["git", "-C", str(WORKTREE), "remote", "add", "origin", ORIGIN], 25),
                ("pair-fetch", ["git", "-C", str(WORKTREE), "-c", "credential.helper=",
                                "-c", "gc.auto=0", "-c", "maintenance.auto=false",
                                "fetch", "--no-tags", "--depth=1", "origin", SOURCE], 90),
                ("pair-detach", ["git", "-C", str(WORKTREE), "checkout", "--detach", SOURCE], 25),
            ]
            for name, command, seconds in commands:
                owner.command(name, command, seconds)
            source_evidence = EVIDENCE/"source-observation"
            source_evidence.mkdir(mode=0o700)
            source_owner = fixed.SourceOwner(
                {"worktree": str(WORKTREE), "sourceRole": "compiler-pair"},
                source_evidence, budget)
            source = source_owner.snapshot()
            fixed.require((source["head"], source["tree"], source["branch"], source["status"]) ==
                          (SOURCE, TREE, "", ""), "PAIR_PREPARATION_EXACT_SHALLOW_SOURCE")
            fixed.require(source_owner.read(WORKTREE/".git/shallow") == (SOURCE+"\n").encode(),
                          "PAIR_PREPARATION_EXACT_SHALLOW_BOUNDARY")
            fixed.require(source_owner.git("remote", "get-url", "origin").decode().strip() == ORIGIN,
                          "PAIR_PREPARATION_CANONICAL_ORIGIN")
            fixed.require(fixed.sha(source_owner.read(fixed.GO)) == fixed.GO_SHA256,
                          "PAIR_PREPARATION_TOOLCHAIN")
            source_pin = fixed.owned.original(EVIDENCE/"source.original.json", source)
            fixed.owned.original(EVIDENCE/"runtime-binding.original.json", fixed.runtime_identity())
        except BaseException as error:
            failures.append({"type": type(error).__name__, "message": str(error)})
        finally:
            with budget.finalizing():
                if budget.cancel_signal is not None or time.monotonic() >= budget.deadline:
                    failures.append({"type": "CANCELLED_OR_DEADLINE", "signal": budget.cancel_signal})
                passed = not failures and source is not None and controller is not None
                report = {
                    "schema": "cherri-hosted-pair-shallow-source-preparation/1",
                    "status": "PREPARED_EXACT_ORDINARY_SOURCE" if passed else "FAIL",
                    "source": SOURCE, "tree": TREE, "worktree": str(WORKTREE),
                    "pairDirectory": str(PAIR), "ordinaryCheckout": True, "shallow": True,
                    "sourceSnapshotSHA256": source_pin["sha256"] if passed else None,
                    "sourcePhysicalFiles": len(source["trackedPhysicalInputs"]) if passed else None,
                    "controller": fixed.CONTEXT["controllerSha"], "runId": fixed.CONTEXT["runId"],
                    "runAttempt": fixed.CONTEXT["runAttempt"], "originalGoSHA256": fixed.GO_SHA256,
                    "wholeOwnerSeconds": 180, "elapsedSeconds": time.monotonic()-budget.started,
                    "minimumFree": owner.minimum, "failures": failures,
                    "goExecuted": False, "nativeAcceptance": False, "finalTupleAcceptance": False,
                }
                with budget.defer():
                    pin = fixed.owned.original(EVIDENCE/"result.original.json", report)
    if passed:
        with Path(os.environ["GITHUB_OUTPUT"]).open("a", encoding="utf-8") as stream:
            stream.write("preparation_sha256=" + pin["sha256"] + "\n")
    print(json.dumps({"result": str(EVIDENCE/"result.original.json"), "sha256": pin["sha256"],
                      "status": report["status"]}), flush=True)
    return 0 if passed else 1


if __name__ == "__main__":
    raise SystemExit(main())
