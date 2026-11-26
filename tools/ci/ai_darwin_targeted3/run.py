"""One fixed hosted Darwin replay of three original AI harness methods.

The original twelve local parent successes and both failed Mac invocations
remain separate evidence. This controller grants no full-suite/native credit.
"""
from pathlib import Path
import hashlib
import json
import os
import platform
import re
import signal
import stat
import sys
import time

sys.dont_write_bytecode = True
REPOSITORY = "davidpovarsky/cherri"
BRANCH = "probe/cherri-p10r15-ai-darwin-targeted3-20261126"
SOURCE = "baf6b0357983aa77e6db7cb6db3ad38fe79257a5"
SOURCE_TREE = "96903f28cf56097f7d4df860919d5babcc6754e9"
SOURCE_PARENT = "618423ffb40f43b2cc8179d4e5b00a01811bfa73"
HELPER_SOURCE = "7b1f85d9cea57773a9bb2a50abfb6056b4bce43b"
PRIOR_15_RESULT = "bcac6f4efca21332cfa67505caa381c6f43b6c38955e8ede3781fa1bf2971da9"
PRIOR_3_RESULT = "331d3f3468f4569dc2bb798db650a0b50a45e3409880282e1241a416d70bc10f"
BRIDGE_SHA = "e3da94806e711b0a79283aa7a310204f53fd20319241eb98b5bfa4142a424381"
SELECTION_SHA = 'c5ce28881dc20b45201bed432b61791cd24895e04422f9f031a79314b4f1ba6e'
SOURCE_PINS = {
    "tools/language-ai-eval/runner.py": (28047, "a68349c4075be6176515786fe4ec51f49f83bab350953f0677e5f772d5cced72"),
    "tools/language-ai-eval/test_runner.py": (31400, "6851e695ce91facf2500122dc8b035d34e2118f1416ad2b3172d75b9df679839"),
    "tools/language-ai-eval/test_process_group_cleanup.py": (6650, "bc189479ac962bde6d2900b74f2df2d51b578b614360c49e188ddd2ad492f8a4"),
    "tools/language-ai-eval/tasks.json": (2568, "fcdb35966a50e2bd7daaa1311a552bde46136e5824109d71d8bf8918db6b4cb5"),
}
HELPER_PINS = {'qa_terminal_receipts.py': (40602, 'cc7b8d3a6f3a8937d9603125f57800d195cac93ee0a50ef35c039c88215aa2be'), 'prepare_public_artifacts.py': (21184, 'a1c575af90a111dd1828825fae049276650a38e5eac411e3e1b49b1662ba1043'), 'storage_cross_run.py': (20494, '6f527c5e22bcd23e220f5d21d09c217abff8859fad16fd743b68183e4ced20c9'), 'xcode_destination_readiness.py': (22225, '4b973bd979e3db7b24edb18599846f8a76661bbf5ed55fc758ad753ba7e98079'), 'xcode_destination_diagnostic.py': (19880, 'adef5a99d7f7c41d297bbcbccabb12a229c07a7d326ecd4f97a76d52e81edf99'), 'simulator_session_recovery.py': (19628, '99f0cb92dc9ec32b2727f671f851053d53ae8e204b02d9405d0af85f188c230d'), 'json_body_probe.py': (38742, 'afaf38f499bdcd4bc56be3a5b7958b96fb65f8eaa7ea22550248a57f678b1520'), 'json_file_body_probe.py': (12641, '6ecc10c7765b24957b94365e57bf81e0958136f3ba9629bb7866c98ab868c24f'), 'retry_http_probe.py': (36163, '8731f6b1fffcb4c3a84af21f6d60e0969026c67611e90bbf2789ecfb8ce03121'), 'nested_list_runtime.py': (14611, '1d78bc44abbd62b14f7af4a3272270736f37eb3a2c86bc43fa37f0d0ea179d8a'), 'nested_list_probe.py': (32947, 'b21d24eecac940ab6decd9330015a49004ccdbbb7d21e5a3b42f1b15f878d925'), 'tagged_carrier_runtime.py': (15549, '34c13f516fe56ec1738860f6093a9af6772f309ec6938b68678aa1acbd3a0da3'), 'tagged_carrier_observation.py': (6360, '246225eb68cf80166e608186ff677afdd0ff80e3e4a39ddfc5777374852a6784'), 'tagged_carrier_graph.py': (24075, '69296dc7177b9ee8871410f9242fd5b50af56ef354193a47d247ae2d4cad58bf'), 'native_comparison_graph.py': (9506, 'b25c055f7718f2b47a4f7615388dbc153f66ec788c8732200644a82a54bb860d'), 'debug_label_preview.py': (15837, '3ffe6e701c0cf6523ca8a0a3e32ea73777e3b3b3b25e31514162c085960b66cb'), 'debug_label_acceptance.py': (38850, '6cdc2685482131f2193f401aa7f99f63492506b82341eb8ec0ddb64830f422e5'), 'project_prerequisite.py': (4717, '6c0bed088dc53e73af138221ce72a6faff0dfa0bc0279cc254ae622dd54bd843'), 'project_prerequisite_observation.py': (65318, '49bcb0e3a0837cae1dc9b31dbac6f53bfa462e5608e6907cff3a1b11832b2cc5')}
CONTROLLER_FILES = (
    ".github/workflows/p10r15-ai-darwin-targeted3.yml",
    "tools/ci/ai_darwin_targeted3/run.py",
    "tools/ci/ai_darwin_targeted3/bridge.py",
    "tools/ci/ai_darwin_targeted3/selection.json",
)
REQUIRED = (
    "test_runner.EvaluationHarnessTests.test_fake_subprocess_covers_both_arms_without_quality_score",
    "test_runner.EvaluationHarnessTests.test_hung_adapter_times_out",
    "test_runner.EvaluationHarnessTests.test_timeout_stops_a_spawned_worker",
)


def require(condition, message):
    if not condition:
        raise RuntimeError(message)


class InterruptionLatch:
    def __init__(self):
        self.signal = None

    def interrupted(self, sig, frame):
        if self.signal is None:
            self.signal = sig
            raise TimeoutError("HOSTED_OWNER_INTERRUPTED_%d" % sig)


def digest(raw):
    return hashlib.sha256(raw).hexdigest()


def original(path, value):
    raw = value if isinstance(value, bytes) else (json.dumps(value, sort_keys=True, indent=2) + "\n").encode()
    with path.open("xb") as stream:
        stream.write(raw)
    return {"path": path.name, "bytes": len(raw), "sha256": digest(raw)}


def validate_event(env, event):
    require(env.get("GITHUB_REPOSITORY") == REPOSITORY and env.get("GITHUB_EVENT_NAME") == "create", "FIXED_REPOSITORY_CREATE_EVENT")
    require(event.get("ref_type") == "branch" and event.get("ref") == BRANCH and
            event.get("repository", {}).get("full_name") == REPOSITORY, "EXACT_ACTIVATION_BRANCH")
    require(env.get("GITHUB_REF") == "refs/heads/" + BRANCH and env.get("GITHUB_RUN_ATTEMPT") == "1", "FIRST_EXACT_REF_ATTEMPT")
    require(re.fullmatch("[0-9a-f]{40}", env.get("GITHUB_SHA", "")) is not None and
            re.fullmatch("[1-9][0-9]*", env.get("GITHUB_RUN_ID", "")) is not None, "ACTUAL_CONTROLLER_AND_RUN")
    return {"repository": REPOSITORY, "event": "create", "ref": BRANCH,
            "controller": env["GITHUB_SHA"], "runId": int(env["GITHUB_RUN_ID"]), "attempt": 1}


def validate_selection(value):
    require(set(value) == {"schema", "prior15ResultSHA256", "prior3ResultSHA256", "originalFound", "originalPassed", "required", "singleFull15Green"}, "CLOSED_SELECTION")
    require(value["schema"] == "cherri-ai-original-three-replay-selection/1" and
            value["prior15ResultSHA256"] == PRIOR_15_RESULT and value["prior3ResultSHA256"] == PRIOR_3_RESULT and
            value["singleFull15Green"] is False, "ACTUAL_FAILED_ORIGIN_SCOPE")
    found, passed, required = value["originalFound"], value["originalPassed"], value["required"]
    require(all(isinstance(x, list) and all(isinstance(y, str) for y in x) for x in (found, passed, required)), "SELECTION_TYPES")
    require(len(found) == len(set(found)) == 15 and len(passed) == len(set(passed)) == 12 and
            set(passed) < set(found) and sorted(set(found) - set(passed)) == list(REQUIRED) == required, "EXACT_THREE_FAILED_ORIGINAL_METHODS")
    return required


def validate_test_receipt(value):
    expected = {"selection_complete", "found", "passed", "tests_run", "failures", "errors", "skipped", "expected_failures", "unexpected_successes"}
    require(isinstance(value, dict) and set(value) == expected, "ORIGINAL_BRIDGE_RECEIPT_FIELDS")
    require(value["selection_complete"] is True and value["found"] == list(REQUIRED) and
            value["passed"] == list(REQUIRED) and value["tests_run"] == 3, "EXACT_THREE_ACTUAL_PARENT_SUCCESSES")
    require(all(type(value[key]) is int and value[key] == 0 for key in
                ("failures", "errors", "skipped", "expected_failures", "unexpected_successes")), "ORIGINAL_NO_FAILURE_SKIP_OR_EXPECTED_FAILURE")


def stable_file(path, expected=None):
    before = path.lstat()
    require(stat.S_ISREG(before.st_mode) and before.st_size <= 128 * 1024 * 1024, "BOUNDED_REGULAR_SOURCE")
    sha256 = hashlib.sha256()
    git_blob = hashlib.sha1(("blob %d\0" % before.st_size).encode())
    raw = bytearray() if expected is not None else None
    with path.open("rb") as stream:
        while True:
            part = stream.read(262144)
            if not part:
                break
            sha256.update(part)
            git_blob.update(part)
            if raw is not None:
                raw.extend(part)
    after = path.lstat()
    require(all(getattr(before, key) == getattr(after, key) for key in
                ("st_dev", "st_ino", "st_mode", "st_uid", "st_gid", "st_size", "st_mtime_ns", "st_ctime_ns")), "SOURCE_CHANGED_DURING_READ")
    row = {"bytes": before.st_size, "sha256": sha256.hexdigest(), "blob": git_blob.hexdigest(),
           "mode": oct(stat.S_IMODE(before.st_mode)), "uid": before.st_uid, "gid": before.st_gid}
    if expected is not None:
        require((row["bytes"], row["sha256"]) == tuple(expected), "EXACT_PINNED_SOURCE")
        return row, bytes(raw)
    return row


def main():
    require(len(sys.argv) == 1 and platform.system() == "Darwin", "FIXED_NO_ARGUMENT_DARWIN_ROUTE")
    require(SELECTION_SHA is not None and len(HELPER_PINS) == 19, "PREPARATION_COMPLETE")
    environment = dict(os.environ)
    identity = validate_event(environment, json.loads(Path(environment["GITHUB_EVENT_PATH"]).read_bytes()))
    workspace = Path(environment["GITHUB_WORKSPACE"]).resolve(strict=True)
    controller, source, helpers_repo = (workspace / name for name in ("controller", "source", "capture-helpers"))
    require(Path(__file__).resolve() == controller / CONTROLLER_FILES[1], "EXACT_CONTROLLER_ENTRY")
    for directory in (controller, source, helpers_repo):
        require(directory.resolve(strict=True) == directory and (directory / ".git").is_dir(), "SEPARATE_ORDINARY_CHECKOUTS")
    output = Path(environment["RUNNER_TEMP"]).resolve(strict=True) / "cherri-ai-darwin-targeted3-20261126"
    os.umask(0o077)
    output.mkdir(mode=0o700)
    commands = output / "commands"
    commands.mkdir(mode=0o700)
    temporary = output / "temporary"
    temporary.mkdir(mode=0o700)
    helper_root = helpers_repo / "tools/shortcuts-simulator-qa"
    helpers_before = {name: stable_file(helper_root / name, expected)[0] for name, expected in HELPER_PINS.items()}
    sys.path.insert(0, str(helper_root))
    from project_prerequisite_observation import capture
    imported = {Path(module.__file__).name for module in tuple(sys.modules.values())
                if getattr(module, "__file__", None) and Path(module.__file__).parent == helper_root}
    require(imported <= set(HELPER_PINS), "CLOSED_QUALIFIED_HELPER_IMPORTS")
    bridge_path = controller / CONTROLLER_FILES[2]
    bridge = bridge_path.read_bytes()
    require(digest(bridge) == BRIDGE_SHA, "EXACT_TARGETED_ORIGINAL_BRIDGE")
    selection_raw = (controller / CONTROLLER_FILES[3]).read_bytes()
    require(digest(selection_raw) == SELECTION_SHA, "PINNED_PRIOR_SELECTION")
    selection = json.loads(selection_raw)
    required = validate_selection(selection)
    keep = ("PATH", "HOME", "LANG", "LC_ALL", "LC_CTYPE", "TZ")
    os.environ.clear()
    os.environ.update({key: environment[key] for key in keep if key in environment})
    os.environ.update(PYTHONDONTWRITEBYTECODE="1", TMPDIR=str(temporary),
                      GIT_OPTIONAL_LOCKS="0", GIT_NO_REPLACE_OBJECTS="1", GIT_TERMINAL_PROMPT="0")
    os.chdir(source)
    started = time.monotonic()
    hard_end = started + 300
    interruption = InterruptionLatch()
    for sig in (signal.SIGALRM, signal.SIGINT, signal.SIGTERM):
        signal.signal(sig, interruption.interrupted)
    signal.setitimer(signal.ITIMER_REAL, 300)
    processes = []
    def run(label, argv, seconds, hard_deadline=None, must_pass=True):
        name = "%03d-%s" % (len(processes) + 1, label)
        prefix = commands / name
        def journal(event, detail):
            original(prefix.with_name(name + "." + event + ".original.json"), detail)
        with prefix.with_name(name + ".stdout.original").open("xb") as stdout, prefix.with_name(name + ".stderr.original").open("xb") as stderr:
            observation, details = capture(argv, seconds, output=stdout, error_output=stderr,
                                           hard_deadline=min(hard_end, hard_deadline or hard_end), journal=journal)
        row = {"name": name, "argv": argv, "seconds": seconds, "observation": observation, "details": details}
        original(prefix.with_name(name + ".process.original.json"), row)
        processes.append(row)
        clean = details["cleanupParentReaped"] is True and details["groupPresentAfterCleanup"] is False
        require(clean, "ACTUAL_OWNED_PROCESS_CLEANUP")
        if must_pass:
            require(observation["result"] == "PASS" and observation["exitCode"] == 0, "ACTUAL_COMMAND_SUCCESS:" + label)
        return row, prefix.with_name(name + ".stdout.original").read_bytes()
    def git(directory, label, *argv):
        return run(label, ["git", "-C", str(directory), *argv], 10)[1]
    def snapshot(directory, expected_sha, expected_tree=None, selected=None):
        head, tree = git(directory, "head-tree", "rev-parse", "HEAD", "HEAD^{tree}").decode().splitlines()
        require(head == expected_sha and (expected_tree is None or tree == expected_tree), "ACTUAL_EXACT_SOURCE_IDENTITY")
        require(git(directory, "status", "status", "--porcelain=v1", "--untracked-files=all") == b"", "ACTUAL_CLEAN_SOURCE")
        entries = git(directory, "tree", "ls-tree", "-rz", "--full-tree", "HEAD", *(["--", *selected] if selected else []))
        files = {}
        for item in entries.split(b"\0"):
            if not item:
                continue
            metadata, raw_name = item.split(b"\t", 1)
            mode, kind, blob = metadata.decode().split(" ")
            name = raw_name.decode("utf-8")
            require(mode in ("100644", "100755") and kind == "blob" and
                    not name.startswith("/") and all(part not in ("", ".", "..", ".git") for part in name.split("/")) and name not in files,
                    "REGULAR_UNIQUE_GIT_SOURCE_PATH")
            row = stable_file(directory / name)
            require(row["blob"] == blob, "ACTUAL_PHYSICAL_GIT_BLOB")
            require((int(row["mode"], 8) & 0o111) == (int(mode, 8) & 0o111),
                    "ACTUAL_PHYSICAL_GIT_EXECUTABLE_BITS")
            files[name] = {**row, "gitMode": mode}
        require(files and (selected is None or set(files) == set(selected)), "COMPLETE_SELECTED_SOURCE")
        return {"head": head, "tree": tree, "files": files}
    before = after = controller_before = controller_after = None
    observation = receipt = None
    failures = []
    result = {"schema": "cherri-hosted-ai-original-three/1", "status": "STARTED", **identity,
              "source": SOURCE, "tree": SOURCE_TREE, "sourceParent": SOURCE_PARENT, "helperSource": HELPER_SOURCE,
              "required": list(REQUIRED), "prior15ResultSHA256": PRIOR_15_RESULT, "prior3ResultSHA256": PRIOR_3_RESULT,
              "priorTwelveSuccessesPreserved": True, "singleFull15Green": False, "fullSuiteAcceptance": False,
              "providerInvoked": False, "goExecuted": False, "nativeInvoked": False,
              "childSeconds": 120, "childWithCleanupSeconds": 125, "wholeOwnerSeconds": 300,
              "runnerLabel": "macos-latest", "pythonSetup": "3.x", "host": {"system": platform.system(),
              "release": platform.release(), "machine": platform.machine(), "python": platform.python_version()},
              "processes": processes, "failures": failures}
    try:
        original(output / "selection.original.json", selection_raw)
        original(output / "bridge.original.py", bridge)
        original(output / "helper-source-before.original.json", helpers_before)
        require(git(helpers_repo, "helper-head", "rev-parse", "HEAD").decode().strip() == HELPER_SOURCE, "EXACT_QUALIFIED_HELPER_COMMIT")
        controller_before = snapshot(controller, identity["controller"], selected=CONTROLLER_FILES)
        original(output / "controller-before.original.json", controller_before)
        retained_controller = output / "controller-source"
        retained_controller.mkdir(mode=0o700)
        for path in CONTROLLER_FILES:
            raw = (controller / path).read_bytes()
            require(digest(raw) == controller_before["files"][path]["sha256"], "RETAINED_EXECUTED_CONTROLLER_BYTES")
            original(retained_controller / Path(path).name, raw)
        retained_helpers = output / "helper-source"
        retained_helpers.mkdir(mode=0o700)
        for name, expected in HELPER_PINS.items():
            original(retained_helpers / name, stable_file(helper_root / name, expected)[1])
        before = snapshot(source, SOURCE, SOURCE_TREE)
        original(output / "source-before.original.json", before)
        retained = output / "source"
        retained.mkdir(mode=0o700)
        for path, expected in SOURCE_PINS.items():
            _, raw = stable_file(source / path, expected)
            original(retained / Path(path).name, raw)
        command = [sys.executable, "-I", "-B", "-c", bridge.decode(), str(source / "tools/language-ai-eval"), json.dumps(required)]
        original(output / "command.original.json", {"argv": command, "childSeconds": 120, "cleanupHardSeconds": 125,
                 "sameOriginalMethodBoundsSeconds": [1, 2, 15], "wholeOwnerSeconds": 300,
                 "environmentKeys": sorted(os.environ), "source": SOURCE})
        require(time.monotonic() + 150 < hard_end, "RESERVE_ORIGINAL_120_PLUS_CLEANUP_AND_SOURCE_AFTER")
        observation, stdout = run("original-three", command, 120, time.monotonic() + 125, must_pass=False)
        receipt = json.loads(stdout)
        validate_test_receipt(receipt)
        require(observation["observation"]["result"] == "PASS" and observation["observation"]["exitCode"] == 0, "ACTUAL_ORIGINAL_THREE_SUCCESS")
    except BaseException as error:
        failures.append({"stage": "execution", "type": type(error).__name__, "message": str(error)})
    finally:
        try:
            require(interruption.signal is None, "NO_NEW_POSTFLIGHT_CHILD_AFTER_INTERRUPTION")
            if before is not None:
                after = snapshot(source, SOURCE, SOURCE_TREE)
                original(output / "source-after.original.json", after)
                require(after == before, "SOURCE_UNCHANGED")
            if controller_before is not None:
                controller_after = snapshot(controller, identity["controller"], selected=CONTROLLER_FILES)
                original(output / "controller-after.original.json", controller_after)
                require(controller_after == controller_before, "CONTROLLER_UNCHANGED")
            helpers_after = {name: stable_file(helper_root / name, expected)[0] for name, expected in HELPER_PINS.items()}
            original(output / "helper-source-after.original.json", helpers_after)
            require(helpers_before == helpers_after, "QUALIFIED_HELPERS_UNCHANGED")
        except BaseException as error:
            failures.append({"stage": "source-after", "type": type(error).__name__, "message": str(error)})
        result.update(status="PASS_THREE_ORIGINAL_METHODS" if not failures else "FAIL", originalTestReceipt=receipt,
                      interruptionSignal=interruption.signal,
                      sourceUnchanged=before is not None and after == before,
                      controllerUnchanged=controller_before is not None and controller_after == controller_before,
                      elapsedSeconds=time.monotonic() - started)
        original(output / "result.original.json", result)
        signal.setitimer(signal.ITIMER_REAL, 0)
    print(json.dumps({"status": result["status"], "source": SOURCE, "controller": identity["controller"],
                      "required": list(REQUIRED), "singleFull15Green": False}), flush=True)
    return 0 if not failures else 1


if __name__ == "__main__":
    raise SystemExit(main())
