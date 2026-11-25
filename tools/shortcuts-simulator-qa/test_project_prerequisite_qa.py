"""Offline QA-join controls; no Apple, Homebrew, compiler or native execution.

Actual owned Git/source/context, original prepare, stage processes and encryption
are used. Platform/environment identity is explicitly synthetic. Timeout and
overflow injections shorten only the test transport; production 180/60/240/310
constants, the adapter's requested allowances and all original controls remain.
"""
import ast
import copy
import hashlib
import json
import os
from pathlib import Path
import plistlib
import signal
import shlex
import shutil
import subprocess
import sys
import tempfile
import time
import unittest
from unittest import mock
import zipfile

import project_prerequisite_observation as subject

HERE = Path(__file__).resolve().parent
REPO = HERE.parents[1]
RUN = "123456"
FIXTURE = "qa-project-observation-control-v2"
GENERATOR_ERROR = b"ORIGINAL_GENERATION_FAILURE_PRIVATE"
DRIVER = r'''
import json,os,sys,time
from pathlib import Path
cfg=json.loads(Path(sys.argv[1]).read_text())
sys.path.insert(0,str(Path(cfg["repo"])/"tools/shortcuts-simulator-qa"))
import project_prerequisite_observation as subject
subject.sys.platform="darwin"  # offline identity adapter, never Apple evidence
real_capture=subject.capture
real_prepare=subject.prerequisite.prepare
real_encrypt=subject.encrypt_archive
real_archive=subject.make_archive
calls=[]
requested=[]
def environment(shell,deadline):
    supplied=Path(shell); physical=supplied.resolve(strict=True)
    raw,identity=subject.bound_read(physical,32*1024*1024)
    return {"expected":subject.EXPECTED_ENVIRONMENT,"actual":subject.EXPECTED_ENVIRONMENT,
        "matched":True,"commands":[],"syntheticControl":True,
        "bash":{"supplied":shell,"physical":str(physical),"sha256":subject.sha(raw),
            "bytes":len(raw),"suppliedIdentity":list(subject.file_identity(supplied.lstat())),
            "physicalIdentity":identity},"python":{"executable":sys.executable,"version":sys.version}}
subject.environment_binding=environment
def capture(command,allowance,**kw):
    if command==["git","--no-replace-objects","rev-parse","HEAD"] and cfg["mode"]=="source_capture_exception":
        kw["output"].write(b"ORIGINAL_SOURCE_CAPTURE_PARTIAL_PRIVATE")
        raise OSError("PRIVATE_EXCEPTION_TEXT_MUST_NOT_ESCAPE")
    if command==[cfg["bash"],"-c",subject.SCRIPTS[0][1]] and cfg["mode"]=="install_timeout":
        requested.append({"stage":"install","allowance":allowance,"injected":0.05})
        return real_capture(command,min(allowance,0.05),**kw)
    if command==[cfg["bash"],"-c",subject.SCRIPTS[1][1]] and cfg["mode"]=="overflow":
        requested.append({"stage":"generate","allowance":allowance,"stream_limit":64})
        return real_capture(command,allowance,stream_limit=64,**kw)
    return real_capture(command,allowance,**kw)
subject.capture=capture
def prepare(*args,**kw):
    calls.append({"context":args[2],"root":str(args[1])})
    code,receipt=real_prepare(*args,**kw)
    if cfg["mode"]=="late":
        assert code==0 and receipt["status"]=="PREPARED"
        receipt["elapsedMs"]=240000
        (Path(args[1])/"builds/project-prerequisite/receipt.json").write_text(json.dumps(receipt,sort_keys=True,indent=2)+"\n")
    return code,receipt
subject.prerequisite.prepare=prepare
def encrypt(*args,**kw):
    if cfg["mode"]=="recipient_changed":
        (Path(cfg["repo"])/subject.RECIPIENT).write_bytes(b"CHANGED_PUBLIC_RECIPIENT")
    result=real_encrypt(*args,**kw)
    if cfg["mode"]=="context_changed_at_seal":
        path=Path(cfg["qa_root"])/"compiler-manifest.json"
        path.write_bytes(path.read_bytes()+b" ")
    if cfg["mode"]=="source_changed_at_seal":
        path=Path(cfg["observation"])/"private/source-head.stdout"
        path.write_bytes(path.read_bytes()+b"CHANGED")
    return result
subject.encrypt_archive=encrypt
def archive(private,*args,**kw):
    if cfg["mode"]=="raw_changed":
        (private/"builds/project-prerequisite/generate.stdout").write_bytes(b"CHANGED_ORIGINAL")
    if cfg["mode"]=="manifest_copy_changed":
        (private/subject.QA_MANIFEST).write_bytes(b'{"changed":true}')
    if cfg["mode"]=="source_raw_changed":
        (private/"source-head.stdout").write_bytes(b"CHANGED_ORIGINAL")
    if cfg["mode"]=="source_process_retyped":
        path=private/"source-head.process.json"
        value=json.loads(path.read_bytes());value["captureReturned"]=1
        path.write_bytes(subject.canonical(value))
    if cfg["mode"]=="source_original_replaced":
        path=private/"source-head.stdout";raw=path.read_bytes()
        path.unlink();path.write_bytes(raw)
    if cfg["mode"]=="source_original_deleted":
        (private/"source-head.stderr").unlink()
    if cfg["mode"]=="source_journal_changed":
        path=private/subject.SOURCE_JOURNAL
        value=json.loads(path.read_bytes());value["sourceMatched"]=1
        path.write_bytes(subject.canonical(value))
    return real_archive(private,*args,**kw)
subject.make_archive=archive
result={"prepare_calls":calls,"requested_fault_allowances":requested,
        "synthetic_environment":True,"native_execution":False}
os.chdir(cfg["repo"])
try:
    sys.argv=["project_prerequisite_observation.py","--repo",cfg["repo"],
        "--root",cfg["observation"],"--source-sha",cfg["source"],"--run-id",cfg["run"],
        "--attempt","1","--bash",cfg["bash"],"--github-output",cfg["github_output"],
        "--qa-root",cfg["qa_root"],"--fixture-id",cfg["fixture"]]
    code=subject.main()  # retain the actual whole-envelope signal/finally path
    result["returncode"]=code
    if code==0:
        Path(cfg["continuation"]).write_bytes(b"SYNTHETIC_XCODE_CONTINUATION_REACHED")
except BaseException as error:
    result.update(returncode=70,exception_type=type(error).__name__,exception=str(error))
Path(cfg["result"]).write_text(json.dumps(result,sort_keys=True,indent=2)+"\n")
raise SystemExit(result["returncode"])
'''


@unittest.skipUnless(os.name == "posix", "actual process-group controls require POSIX")
class QAIntegrationTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory(prefix="cherri-prerequisite-qa-offline-")
        self.addCleanup(self.temporary.cleanup)
        self.home = Path(self.temporary.name).resolve()
        self.case_count = 0
        self.git_executable = str(Path(shutil.which("git")).resolve())
        self.bash = str(Path(shutil.which("bash")).resolve())

    def git(self, repo, *args):
        run = subprocess.run([self.git_executable, *args], cwd=repo,
            capture_output=True, timeout=10, check=False)
        self.assertEqual(run.returncode, 0, run.stderr.decode(errors="replace"))
        return run.stdout.decode().strip()

    def case(self, mode="healthy"):
        self.case_count += 1
        base = self.home / str(self.case_count)
        repo, bins = base / "repo", base / "bin"
        repo.mkdir(parents=True)
        bins.mkdir()
        for name in subject.QA_SOURCE_FILES:
            target = repo / name
            target.parent.mkdir(parents=True, exist_ok=True)
            shutil.copyfile(REPO / name, target)
        # Each independent fixture is committed before its manifest is pinned.
        # Ignoring generated output does not bypass prepare's actual stale check.
        (repo / ".gitignore").write_text("artifacts/\n**/CherriSimulatorQA.xcodeproj/\n")
        project = repo / subject.prerequisite.PROJECT
        info = project / "ReceiverApp/Info.plist"
        info.write_bytes(plistlib.dumps({"CFBundleURLTypes": [{"CFBundleURLSchemes": ["cherriqa"]}]}))
        fixture_raw = b"// fixed public offline context fixture\n"
        (repo / subject.QA / "fixtures" / (FIXTURE + ".cherri")).write_bytes(fixture_raw)
        self.git(repo, "init", "-q")
        self.git(repo, "config", "core.autocrlf", "false")
        self.git(repo, "add", ".")
        self.git(repo, "-c", "user.name=Cherri_QA", "-c", "user.email=qa@example.invalid",
                 "-c", "commit.gpgsign=false", "commit", "-qm", "owned QA integration control")
        source = self.git(repo, "--no-replace-objects", "rev-parse", "HEAD")
        qa_root = repo / "artifacts/cherri-ios27-simulator-qa" / RUN
        qa_root.mkdir(parents=True)
        manifest = {"syntheticOnly": True, "compilerCommitSha": source, "fixtures": [{
            "fixtureId": FIXTURE, "fileName": FIXTURE + ".cherri",
            "sourceSha256": hashlib.sha256(fixture_raw).hexdigest(),
            "unsignedPlistSha256": "2" * 64, "buildProfile": "debug",
            "runtimeTraceExpectation": "REQUIRED_NDJSON"}]}
        (qa_root / "compiler-manifest.json").write_text(json.dumps(manifest))
        runner_temp = base / "runner-temp"
        runner_temp.mkdir()
        cfg = {"repo": str(repo), "qa_root": str(qa_root), "source": source,
               "run": RUN, "fixture": FIXTURE, "bash": self.bash, "mode": mode,
               "observation": str(runner_temp / ("cherri-qa-project-prerequisite-" + RUN + "-1")),
               "github_output": str(base / "github-output"), "result": str(base / "driver-result.json"),
               "continuation": str(base / "synthetic-xcode-continuation")}
        wrapper = bins / "git"
        wrapper.write_text("#!" + self.bash + "\nexec " + shlex.quote(self.git_executable) + ' "$@"\n')
        wrapper.chmod(0o700)
        generator = (
            "import os,sys,time;from pathlib import Path;mode=os.environ['CONTROL_MODE'];"
            "\nif mode=='install_timeout':time.sleep(2)"
            "\nif mode in ('generation_failure','forged_summary'):"
            "\n os.write(2,b'ORIGINAL_GENERATION_FAILURE_PRIVATE');sys.exit(11)"
            "\nif mode=='overflow':os.write(1,b'x'*128);sys.exit(0)"
            "\np=Path('CherriSimulatorQA.xcodeproj');p.mkdir();"
            "(p/'project.pbxproj').write_bytes(b'OWNED_PUBLIC_PROJECT_BYTES');"
            "os.write(1,b'ORIGINAL_GENERATION_OUTPUT')"
            "\nif mode=='source_changed':"
            "\n p=Path('project.yml');p.write_bytes(p.read_bytes()+b'\\n# changed')"
        )
        program = bins / ("brew" if mode == "install_timeout" else "xcodegen")
        program.write_text("#!" + self.bash + "\nexec " + shlex.quote(sys.executable) +
                           " -c " + shlex.quote(generator) + "\n")
        program.chmod(0o700)
        env = dict(os.environ, PATH=str(bins), PYTHONDONTWRITEBYTECODE="1",
                   GIT_NO_REPLACE_OBJECTS="1", RUNNER_TEMP=str(runner_temp), RUNNER_OS="macOS",
                   GITHUB_SHA=source, GITHUB_RUN_ID=RUN, GITHUB_RUN_ATTEMPT="1",
                   GITHUB_REPOSITORY="davidpovarsky/cherri", GITHUB_EVENT_NAME="workflow_dispatch",
                   CONTROL_MODE=mode)
        return cfg, env

    def run_case(self, cfg, env):
        path = Path(cfg["result"]).parent / "driver-input.json"
        path.write_text(json.dumps(cfg))
        command = [sys.executable, "-B", "-c", DRIVER, str(path)]
        process = subprocess.Popen(command, cwd=cfg["repo"], env=env,
            stdout=subprocess.PIPE, stderr=subprocess.PIPE, start_new_session=True)
        try:
            stdout, stderr = process.communicate(timeout=15)
        except subprocess.TimeoutExpired:
            # The real main SIGTERM handler unwinds its current capture and
            # cleans that owned stage group before this test cleans its driver.
            try: os.killpg(process.pid, signal.SIGTERM)
            except ProcessLookupError: pass
            try:
                process.communicate(timeout=2)
            except subprocess.TimeoutExpired:
                try: os.killpg(process.pid, signal.SIGKILL)
                except ProcessLookupError: pass
                process.communicate(timeout=2)
            raise
        finally:
            if process.poll() is None:
                try: os.killpg(process.pid, signal.SIGKILL)
                except ProcessLookupError: pass
                process.wait(timeout=2)
        run = subprocess.CompletedProcess(command, process.returncode, stdout, stderr)
        self.assertTrue(Path(cfg["result"]).is_file(), run.stderr.decode(errors="replace"))
        result = json.loads(Path(cfg["result"]).read_bytes())
        self.assertEqual(run.returncode, result["returncode"])
        self.assertFalse(result["native_execution"])
        return run, result

    def receipt(self, cfg):
        return json.loads((Path(cfg["observation"]) / "private/builds/project-prerequisite/receipt.json").read_bytes())

    def assert_stopped(self, cfg, result):
        self.assertNotEqual(result["returncode"], 0)
        self.assertFalse(Path(cfg["continuation"]).exists())
        self.assertLessEqual(len(result["prepare_calls"]), 1)

    def source_packet(self, cfg, result, code, checkpoint):
        self.assert_stopped(cfg, result)
        self.assertEqual(result["prepare_calls"], [])
        root = Path(cfg["observation"])
        summary = subject.read_json(root / "public/summary.json")
        subject.validate_public_summary(summary)
        self.assertEqual((summary["reason"], summary["prerequisiteStatus"]),
                         ("SOURCE_REJECTED", "NOT_STARTED"))
        self.assertFalse(summary["sourceMatchedBefore"] or summary["sourceMatchedAfter"])
        journal = subject.read_json(root / "private" / subject.SOURCE_JOURNAL)
        self.assertEqual((journal["rejectionCode"], journal["checkpoint"]), (code, checkpoint))
        self.assertIs(journal["sourceMatched"], False)
        self.assertEqual(journal["sourceSha"], cfg["source"])
        self.assertEqual(journal["sourcePaths"], list(subject.QA_SOURCE_FILES))
        self.assertEqual([row["phase"] for row in journal["commands"]],
                         list(subject.SOURCE_PHASES[:len(journal["commands"])]))
        with zipfile.ZipFile(root / "private/project-prerequisite.zip") as archive:
            manifest = subject.parse_json(archive.read("manifest.json"))
            self.assertEqual(manifest["members"], summary["members"])
            for member in summary["members"]:
                raw = archive.read(member["name"])
                self.assertEqual((len(raw), subject.sha(raw)), (member["bytes"], member["sha256"]))
                self.assertEqual(raw, (root / "private" / member["name"]).read_bytes())
        public = subject.canonical(summary)
        for private in (str(root).encode(), str(cfg["repo"]).encode(), code.encode(),
                        b"PRIVATE_SOURCE", b"PRIVATE_EXCEPTION", b"hardDeadlineMonotonic"):
            self.assertNotIn(private, public)
        self.assertEqual({p.name for p in (root / "public").iterdir()}, subject.PUBLIC_FILES)
        self.assertEqual(Path(cfg["github_output"]).read_text(), "prerequisite_publication_ready=true\n")
        return summary, journal

    def test_dirty_source_keeps_original_git_output_without_preparing(self):
        cfg, env = self.case()
        canary = "PRIVATE_SOURCE_STATUS_CANARY"
        (Path(cfg["repo"]) / canary).write_bytes(b"owned synthetic source control")
        run, result = self.run_case(cfg, env)
        summary, journal = self.source_packet(cfg, result, "SOURCE_DIRTY_BEFORE", "CLEAN_STATUS")
        self.assertEqual(len(journal["commands"]), 2)
        raw = (Path(cfg["observation"]) / "private/source-status.stdout").read_bytes()
        self.assertEqual(raw, ("?? " + canary + "\n").encode())
        self.assertEqual(journal["commands"][-1]["commandReceipt"]["result"], "PASS")
        self.assertIs(summary["captureComplete"], True)
        self.assertNotIn(canary.encode(), run.stdout + run.stderr)
        self.assertEqual(len(summary["members"]), 10)

    def test_source_head_tree_bytes_and_file_type_rejections_keep_the_boundary(self):
        for fault, code, checkpoint in (
            ("head", "SOURCE_HEAD", "HEAD_VALUE"),
            ("tree", "SOURCE_INVENTORY", "TREE_INVENTORY"),
            ("bytes", "SOURCE_BYTES", "FILE_BYTES"),
            ("symlink", "FILE_NOT_REGULAR", "FILE_READ")):
            with self.subTest(fault=fault):
                cfg, env = self.case()
                repo = Path(cfg["repo"])
                name = subject.QA + "XcodeProject/project.yml"
                if fault == "head":
                    cfg["source"] = env["GITHUB_SHA"] = "b" * 40
                elif fault == "tree":
                    self.git(repo, "rm", subject.QA + "test_project_prerequisite_qa.py")
                    self.git(repo, "-c", "user.name=Cherri_QA", "-c", "user.email=qa@example.invalid",
                             "-c", "commit.gpgsign=false", "commit", "-qm", "owned missing inventory control")
                    cfg["source"] = env["GITHUB_SHA"] = self.git(repo, "rev-parse", "HEAD")
                else:
                    # Real Git's index can hide a local mutation from status;
                    # the independent Git blob and regular-file checks must hold.
                    self.git(repo, "update-index", "--assume-unchanged", name)
                    if fault == "bytes":
                        (repo / name).write_bytes((repo / name).read_bytes() + b"\n# owned mutation\n")
                    else:
                        (repo / name).unlink()
                        (repo / name).symlink_to("ReceiverApp/Info.plist")
                    self.assertEqual(self.git(repo, "status", "--porcelain=v1"), "")
                _, result = self.run_case(cfg, env)
                _, journal = self.source_packet(cfg, result, code, checkpoint)
                if fault in {"bytes", "symlink"}:
                    self.assertEqual(journal["fileIndex"], list(subject.QA_SOURCE_FILES).index(name))
                else:
                    self.assertIsNone(journal["fileIndex"])

    def test_failed_git_processes_keep_bounded_raw_streams_and_actual_receipts(self):
        for fault, observed, complete in (("nonzero", "EXIT_NONZERO", True),
                ("stderr", "PASS", True), ("timeout", "TIMEOUT", None),
                ("overflow", "OUTPUT_OVERFLOW", False)):
            with self.subTest(fault=fault):
                cfg, env = self.case()
                wrapper = Path(env["PATH"]) / "git"
                program = (
                    "import os,sys,time\n"
                    "if sys.argv[1:]==['--no-replace-objects','rev-parse','HEAD']:\n"
                    f" fault={fault!r}\n"
                    " if fault=='nonzero':os.write(1,b'PRIVATE_SOURCE_PARTIAL');os.write(2,b'PRIVATE_SOURCE_ERROR');sys.exit(17)\n"
                    f" if fault=='stderr':os.write(1,{(cfg['source'] + chr(10)).encode()!r});os.write(2,b'PRIVATE_SOURCE_WARNING');sys.exit(0)\n"
                    " if fault=='timeout':os.write(1,b'PRIVATE_SOURCE_TIMEOUT');time.sleep(6)\n"
                    " if fault=='overflow':os.write(1,b'PRIVATE_SOURCE_OVERFLOW'+b'x'*65537);sys.exit(0)\n"
                    f"os.execv({self.git_executable!r},[{self.git_executable!r},*sys.argv[1:]])\n"
                )
                wrapper.write_text("#!" + sys.executable + "\n" + program)
                run, result = self.run_case(cfg, env)
                summary, journal = self.source_packet(cfg, result, "IDENTITY_COMMAND_FAILED", "HEAD_COMMAND")
                row = journal["commands"][0]
                self.assertEqual(len(journal["commands"]), 1)
                self.assertEqual(row["commandReceipt"]["result"], observed)
                self.assertEqual(row["maximumSeconds"], 3)
                self.assertEqual(row["allowanceSeconds"], 3)
                self.assertEqual(row["argv"], ["git", "--no-replace-objects", "rev-parse", "HEAD"])
                self.assertEqual(row["cwd"], cfg["repo"])
                if complete is not None:
                    self.assertIs(summary["captureComplete"], complete)
                else:
                    # Cancellation may drain both pipes fully. Preserve that
                    # observation without promoting the timed-out source check.
                    self.assertEqual(summary["captureComplete"], all(
                        stream["complete"] for stream in row["streams"].values()))
                self.assertNotIn(b"PRIVATE_SOURCE", run.stdout + run.stderr)
                for stream in ("stdout", "stderr"):
                    raw = (Path(cfg["observation"]) / ("private/source-head." + stream)).read_bytes()
                    captured = row["commandReceipt"]["capture"][stream]
                    self.assertEqual((captured["bytes"], captured["sha256"]), (len(raw), subject.sha(raw)))
                    self.assertLessEqual(len(raw), subject.METADATA_LIMIT + 1)
                if fault == "nonzero":
                    self.assertEqual(row["commandReceipt"]["exitCode"], 17)
                if fault == "overflow":
                    self.assertEqual(row["streams"]["stdout"]["bytes"], subject.METADATA_LIMIT + 1)
                    self.assertTrue(row["commandReceipt"]["capture"]["stdout"]["overflow"])

    def test_interrupted_capture_retains_partial_bytes_without_inventing_a_result(self):
        cfg, env = self.case("source_capture_exception")
        run, result = self.run_case(cfg, env)
        summary, journal = self.source_packet(cfg, result, "OS_ERROR", "HEAD_COMMAND")
        row = journal["commands"][0]
        self.assertIs(row["captureReturned"], False)
        self.assertIsNone(row["commandReceipt"])
        self.assertFalse(summary["captureComplete"])
        self.assertFalse(any(value["complete"] for value in row["streams"].values()))
        raw = (Path(cfg["observation"]) / "private/source-head.stdout").read_bytes()
        self.assertEqual(raw, b"ORIGINAL_SOURCE_CAPTURE_PARTIAL_PRIVATE")
        self.assertNotIn(b"PRIVATE_EXCEPTION", run.stdout + run.stderr)

    def test_source_original_tampering_blocks_archive_or_publication(self):
        for mode in ("source_raw_changed", "source_process_retyped", "source_original_replaced",
                     "source_original_deleted", "source_journal_changed", "source_changed_at_seal"):
            with self.subTest(mode=mode):
                cfg, env = self.case(mode)
                _, result = self.run_case(cfg, env)
                self.assert_stopped(cfg, result)
                self.assertEqual(len(result["prepare_calls"]), 1)
                self.assertFalse(Path(cfg["github_output"]).exists())
                self.assertFalse((Path(cfg["observation"]) / "public/summary.json").exists())
                if mode != "source_original_deleted":
                    expected = "SOURCE_ORIGINAL_CHANGED" if mode == "source_changed_at_seal" else "ORIGINAL_CHANGED_BEFORE_ARCHIVE"
                    self.assertEqual(result["exception"], expected)

    def test_journal_cannot_change_command_order_directory_or_original_allowance(self):
        cfg, _ = self.case()
        repo, private = Path(cfg["repo"]), self.home / "journal-router"
        private.mkdir()
        deadline = time.monotonic() + 30
        args = ["git", "--no-replace-objects", "rev-parse", "HEAD"]
        before = Path.cwd()
        try:
            os.chdir(repo)
            for command, maximum, cwd in ((args + ["extra"], 3, repo),
                    (args, 4, repo), (args, 3.0, repo), (args, 3, self.home),
                    (["git", "status"], 3, repo)):
                with self.subTest(command=command, maximum=maximum, cwd=str(cwd)):
                    os.chdir(cwd)
                    journal = subject.SourcePreflight(private, repo, cfg["source"], deadline)
                    with mock.patch.object(subject, "capture") as capture:
                        with self.assertRaises(subject.Rejected):
                            subject.small_command(command, deadline, maximum, journal=journal)
                        capture.assert_not_called()
            os.chdir(repo)
            journal = subject.SourcePreflight(private, repo, cfg["source"], deadline)
            sampled = time.monotonic()
            with self.assertRaisesRegex(subject.Rejected, "SOURCE_COMMAND_ALLOWANCE"):
                journal.begin(args, deadline, 3, 2, sampled)
            subject.small_command(args, deadline, 3, journal=journal)
            with mock.patch.object(subject, "capture") as capture:
                with self.assertRaisesRegex(subject.Rejected, "SOURCE_COMMAND_CHANGED"):
                    subject.small_command(args, deadline, 3, journal=journal)
                capture.assert_not_called()
            path = private / "source-head.process.json"
            raw = path.read_bytes()
            path.unlink()
            path.write_bytes(raw)
            with self.assertRaisesRegex(subject.Rejected, "SOURCE_JOURNAL_CHANGED"):
                journal.finish(subject.Rejected("SOURCE_HEAD"))
        finally:
            os.chdir(before)

    def test_source_member_prefix_and_capture_completeness_cannot_be_forged(self):
        cfg, env = self.case()
        (Path(cfg["repo"]) / "PRIVATE_SOURCE_STATUS_CANARY").write_bytes(b"owned")
        _, result = self.run_case(cfg, env)
        summary, _ = self.source_packet(cfg, result, "SOURCE_DIRTY_BEFORE", "CLEAN_STATUS")
        for fault in ("missing_journal", "missing_stream", "nonprefix", "arbitrary", "matched", "complete", "type"):
            with self.subTest(fault=fault):
                broken = copy.deepcopy(summary)
                if fault == "missing_journal":
                    broken["members"] = [row for row in broken["members"] if row["name"] != subject.SOURCE_JOURNAL]
                if fault == "missing_stream":
                    broken["members"] = [row for row in broken["members"] if row["name"] != "source-head.stderr"]
                if fault == "nonprefix":
                    for row in broken["members"]:
                        row["name"] = row["name"].replace("source-head.", "source-tree.")
                if fault == "arbitrary": broken["members"][0]["name"] = "private-canary.stdout"
                if fault == "matched": broken["sourceMatchedBefore"] = True
                if fault == "complete": broken["captureComplete"] = False
                if fault == "type": broken["members"][-1]["complete"] = 1
                with self.assertRaises(subject.Rejected):
                    subject.validate_public_summary(broken)

    def test_observed_prepare_runs_once_and_keeps_original_fixture_context(self):
        cfg, env = self.case()
        run, result = self.run_case(cfg, env)
        self.assertEqual(result["returncode"], 0, run.stderr.decode(errors="replace") + str(result))
        self.assertEqual(len(result["prepare_calls"]), 1)
        self.assertTrue(Path(cfg["continuation"]).is_file())
        receipt = self.receipt(cfg)
        expected = subject.prerequisite.terminal.context(cfg["qa_root"], cfg["source"], RUN, FIXTURE)
        self.assertEqual(result["prepare_calls"][0]["context"], expected)
        self.assertEqual({key: receipt[key] for key in expected}, expected)
        self.assertEqual(receipt["status"], "PREPARED")
        self.assertEqual([row["stage"] for row in receipt["commands"]], ["install", "generate"])
        self.assertEqual([row["allowanceMs"] for row in receipt["commands"]], [180000, 60000])
        self.assertLess(receipt["elapsedMs"], 240000)
        generated = Path(cfg["repo"]) / subject.prerequisite.PROJECT / subject.prerequisite.GENERATED
        self.assertEqual(generated.read_bytes(), b"OWNED_PUBLIC_PROJECT_BYTES")
        self.assertEqual(receipt["projectSha256"], subject.sha(generated.read_bytes()))
        summary = subject.parse_json((Path(cfg["observation"]) / "public/summary.json").read_bytes())
        subject.validate_public_summary(summary)
        self.assertEqual(summary["schema"], subject.QA_SCHEMA)
        self.assertEqual(summary["qaContextSha256"], subject.sha(subject.canonical(expected)))
        self.assertTrue(summary["qaContextMatchedBefore"] and summary["qaContextMatchedAfter"])
        self.assertEqual(len(summary["members"]), 23)
        original_manifest = (Path(cfg["qa_root"]) / "compiler-manifest.json").read_bytes()
        manifest_member = next(row for row in summary["members"] if row["name"] == subject.QA_MANIFEST)
        self.assertEqual(manifest_member["sha256"], expected["compilerManifestSha256"])
        self.assertEqual(manifest_member["bytes"], len(original_manifest))
        with zipfile.ZipFile(Path(cfg["observation"]) / "private/project-prerequisite.zip") as archive:
            self.assertEqual(len(archive.namelist()), 24)
            self.assertEqual(archive.read(subject.QA_MANIFEST), original_manifest)
            self.assertEqual(json.loads(archive.read("manifest.json"))["schema"], subject.QA_SCHEMA + "/originals")
        self.assertEqual({p.name for p in (Path(cfg["observation"]) / "public").iterdir()},
                         {"project-prerequisite.zip.p7m", "summary.json"})
        self.assertEqual(Path(cfg["github_output"]).read_text(), "prerequisite_publication_ready=true\n")
        self.assertFalse((Path(cfg["qa_root"]) / "builds/project-prerequisite").exists())

    def test_failed_timeout_overflow_and_late_prepare_never_continue(self):
        for mode, expected in (("generation_failure", "EXIT_NONZERO"),
                               ("install_timeout", "TIMEOUT"), ("overflow", "OUTPUT_OVERFLOW"),
                               ("late", "LATE_OR_UNBOUND_PREPARED")):
            with self.subTest(mode=mode):
                cfg, env = self.case(mode)
                _, result = self.run_case(cfg, env)
                self.assert_stopped(cfg, result)
                self.assertEqual(len(result["prepare_calls"]), 1)
                receipt = self.receipt(cfg)
                if mode == "late":
                    self.assertEqual(result["exception"], expected)
                    self.assertFalse(Path(cfg["github_output"]).exists())
                else:
                    self.assertEqual(receipt["status"], "FAILED")
                    self.assertEqual(receipt["commands"][-1]["result"], expected)
                    self.assertTrue((Path(cfg["observation"]) / "public/project-prerequisite.zip.p7m").is_file())
                if mode == "generation_failure":
                    raw = Path(cfg["observation"]) / "private/builds/project-prerequisite/generate.stderr"
                    self.assertEqual(raw.read_bytes(), GENERATOR_ERROR)
                if mode == "install_timeout":
                    self.assertEqual([row["stage"] for row in receipt["commands"]], ["install"])
                    self.assertEqual(result["requested_fault_allowances"][0]["allowance"], 180)
                    self.assertFalse((Path(cfg["observation"]) / "private/generate.intent.json").exists())
        self.assertEqual(subject.prerequisite.STAGE_SECONDS, {"install": 180, "generate": 60})
        self.assertEqual((subject.prerequisite.BUDGET_SECONDS, subject.OUTER_SECONDS), (240, 310))

    def test_stale_project_or_untrusted_context_cannot_supply_a_skip(self):
        cfg, env = self.case()
        stale = Path(cfg["repo"]) / subject.prerequisite.PROJECT / subject.prerequisite.GENERATED
        stale.parent.mkdir()
        stale.write_bytes(b"STALE_PROJECT")
        _, result = self.run_case(cfg, env)
        self.assert_stopped(cfg, result)
        self.assertEqual(len(result["prepare_calls"]), 1)
        self.assertEqual(stale.read_bytes(), b"STALE_PROJECT")
        cfg, env = self.case("forged_summary")
        (Path(cfg["qa_root"]) / "summary.json").write_text('{"prerequisiteStatus":"PREPARED"}')
        _, result = self.run_case(cfg, env)
        self.assert_stopped(cfg, result)
        self.assertEqual(len(result["prepare_calls"]), 1)
        self.assertEqual(self.receipt(cfg)["commands"][-1]["exitCode"], 11)
        cfg, _ = self.case()
        original = Path(cfg["qa_root"]) / "compiler-manifest.json"
        raw = original.read_bytes()
        for fault in ("source", "fixture", "profile", "synthetic"):
            with self.subTest(context_fault=fault):
                value = json.loads(raw)
                if fault == "source": value["compilerCommitSha"] = "b" * 40
                if fault == "fixture": value["fixtures"][0]["fixtureId"] = "another-fixture"
                if fault == "profile": value["fixtures"][0]["buildProfile"] = "unproved"
                if fault == "synthetic": value["syntheticOnly"] = False
                original.write_text(json.dumps(value))
                with self.assertRaises(ValueError):
                    subject.qa_context_binding(cfg["repo"], cfg["qa_root"], cfg["source"], RUN, FIXTURE)
        original.write_bytes(raw)
        with self.assertRaisesRegex(subject.Rejected, "QA_ARTIFACT_ROOT"):
            subject.qa_context_binding(cfg["repo"], cfg["qa_root"], cfg["source"], "999999", FIXTURE)

    def test_post_prepare_context_and_original_mutation_rejects_publication(self):
        for mode in ("context_changed_at_seal", "raw_changed", "manifest_copy_changed",
                     "recipient_changed", "source_changed"):
            with self.subTest(mode=mode):
                cfg, env = self.case(mode)
                _, result = self.run_case(cfg, env)
                self.assert_stopped(cfg, result)
                self.assertEqual(len(result["prepare_calls"]), 1)
                self.assertFalse(Path(cfg["github_output"]).exists())
                self.assertFalse((Path(cfg["observation"]) / "public/summary.json").exists())
                self.assertTrue((Path(cfg["observation"]) / "private/builds/project-prerequisite/receipt.json").is_file())
                expected = {"context_changed_at_seal": "QA_CONTEXT_CHANGED_AT_SEAL",
                            "raw_changed": "ORIGINAL_CHANGED_BEFORE_ARCHIVE",
                            "manifest_copy_changed": "ORIGINAL_CHANGED_BEFORE_ARCHIVE",
                            "recipient_changed": "RECIPIENT_CHANGED",
                            "source_changed": "LATE_OR_UNBOUND_PREPARED"}[mode]
                self.assertEqual(result["exception"], expected)

    def test_actual_shell_routes_once_and_has_no_failure_fallback(self):
        script = (REPO / "scripts/ios27_shortcuts_simulator_qa.sh").read_text()
        start = script.index('    echo "=== Phase 3 & 4: Running XCTest UI Automation & Callback Receiver ==="')
        block = script[start:script.index('    # A GitHub rerun reuses RUN_ID.', start)]
        for mode, code in (("true", 0), ("true", 70), ("false", 0), ("invalid", 0)):
            with self.subTest(mode=mode, returncode=code):
                case = self.home / (mode + str(code)); case.mkdir()
                calls, reached = case / "calls", case / "continuation"
                program = case / "python3"
                program.write_text("#!" + self.bash + '\nprintf "%s\\n" "$1" >> "$CONTROL_CALLS"\nexit "$CONTROL_EXIT"\n')
                program.chmod(0o700)
                env = dict(os.environ, PATH=str(case), ROOT_DIR="/owned-source", ARTIFACT_DIR="/owned-artifacts",
                    RUN_ID=RUN, SOURCE_SHA="a" * 40, TARGET_FIXTURE_ID=FIXTURE,
                    RUNNER_TEMP="/owned-runner-temp", GITHUB_RUN_ATTEMPT="1", GITHUB_OUTPUT="/owned-github-output",
                    QA_OBSERVE_PROJECT_PREREQUISITE=mode, CONTROL_CALLS=str(calls), CONTROL_EXIT=str(code),
                    CONTROL_CONTINUATION=str(reached))
                run = subprocess.run([self.bash, "-c", block + '\nprintf REACHED > "$CONTROL_CONTINUATION"\n'],
                    env=env, capture_output=True, timeout=5, check=False)
                observed = calls.read_text().splitlines() if calls.exists() else []
                if mode == "invalid":
                    self.assertNotEqual(run.returncode, 0); self.assertEqual(observed, [])
                else:
                    wanted = "project_prerequisite_observation.py" if mode == "true" else "project_prerequisite.py"
                    self.assertEqual(observed, ["/owned-source/tools/shortcuts-simulator-qa/" + wanted])
                    self.assertEqual(run.returncode == 0, code == 0)
                self.assertEqual(reached.exists(), mode != "invalid" and code == 0)

    def test_qa_scope_and_exact_workflow_upload_are_separate_from_standalone(self):
        import yaml
        original = __import__("test_project_prerequisite_observation").ObservationTests
        empty = {"bytes": 0, "sha256": subject.sha(b"")}
        members = [{"name": name, **empty, "complete": True}
                   for name in ("source.json", "identity.json", "diagnostic.json")]
        # Use the unchanged v1 shape, then explicitly construct the v3 scope.
        summary = original.summary(self, members, empty, empty)
        subject.validate_public_summary(summary)
        legacy = copy.deepcopy(summary)
        summary["schema"] = subject.QA_SCHEMA
        for name in ("simulatorStarted", "nativeTestStarted", "compilerStarted", "modelRequestStarted"):
            del summary[name]
        summary.update(executionScope=subject.QA_SCOPE, qaContextSha256=None,
            qaContextMatchedBefore=False, qaContextMatchedAfter=False,
            sourceMatchedBefore=False, sourceMatchedAfter=False, **subject.execution_fields(qa=True))
        summary["members"].append({"name": subject.SOURCE_JOURNAL, **empty, "complete": True})
        subject.validate_public_summary(summary)
        for fault in ("global_false", "assessed_prior", "native_action", "context_boolean", "prepared_without_context"):
            with self.subTest(scope_fault=fault):
                broken = copy.deepcopy(summary)
                if fault == "global_false": broken["simulatorStarted"] = False
                if fault == "assessed_prior": broken["priorWorkflowPhasesAssessed"] = True
                if fault == "native_action": broken["collectorActionsStarted"]["nativeTest"] = True
                if fault == "context_boolean": broken["qaContextMatchedBefore"] = 0
                if fault == "prepared_without_context": broken["prerequisiteStatus"] = "PREPARED"
                with self.assertRaises(subject.Rejected): subject.validate_public_summary(broken)
        subject.validate_public_summary(legacy)
        workflow = yaml.safe_load((REPO / subject.WORKFLOW).read_text())
        steps = workflow["jobs"]["simulator-qa"]["steps"]
        native = next(step for step in steps if step.get("id") == "native_qa")
        self.assertEqual(native["env"]["QA_OBSERVE_PROJECT_PREREQUISITE"], "true")
        upload = next(step for step in steps if step["name"] == "Upload only sealed project prerequisite diagnostics")
        self.assertEqual(upload["if"], "always() && steps.native_qa.outputs.prerequisite_publication_ready == 'true'")
        prefix = "${{ runner.temp }}/cherri-qa-project-prerequisite-${{ github.run_id }}-${{ github.run_attempt }}/public/"
        self.assertEqual(upload["with"]["path"].splitlines(),
                         [prefix + "project-prerequisite.zip.p7m", prefix + "summary.json"])
        self.assertNotIn("qa-scratch", upload["with"]["path"])
        self.assertEqual(upload["with"]["if-no-files-found"], "error")
        prior = ast.parse((HERE / "test_project_prerequisite.py").read_bytes())
        observer = ast.parse((HERE / "test_project_prerequisite_observation.py").read_bytes())
        count = lambda tree: sum(isinstance(node, ast.FunctionDef) and node.name.startswith("test_") for node in ast.walk(tree))
        self.assertEqual((count(prior), count(observer)), (8, 33))


if __name__ == "__main__":
    unittest.main()
