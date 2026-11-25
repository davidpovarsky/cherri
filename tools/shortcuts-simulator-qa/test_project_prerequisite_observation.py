"""Offline controls only: no Homebrew, Apple command, simulator or provider."""
import contextlib
import errno
import copy
import hashlib
import io
import json
import os
from pathlib import Path
import plistlib
import shutil
import shlex
import stat
import subprocess
import sys
import tempfile
import time
import unittest
from unittest import mock
import zipfile

import project_prerequisite_observation as subject

HERE = Path(__file__).resolve().parent
CONTEXT = subject.validate_context("a" * 40, "123456789", "1")


@contextlib.contextmanager
def cwd(path):
    old = Path.cwd()
    os.chdir(path)
    try:
        yield
    finally:
        os.chdir(old)


class ObservationTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory(prefix="cherri-prerequisite-offline-")
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name).resolve()
        self.private, self.public = self.root / "private", self.root / "public"
        self.private.mkdir()
        self.public.mkdir()
        self.project_repo = self.root / "repo"
        self.project = self.project_repo / subject.prerequisite.PROJECT
        self.project.mkdir(parents=True)
        (self.project / "ReceiverApp").mkdir()
        (self.project / "ReceiverApp/Info.plist").write_bytes(plistlib.dumps({
            "CFBundleURLTypes": [{"CFBundleURLSchemes": ["cherriqa"]}]}))
        (self.private / "builds").mkdir()
        self.bash = str(Path(shutil.which("bash")).resolve())
        raw, identity = subject.bound_read(self.bash, 32 * 1024 * 1024)
        self.shell_observation = {"physical": self.bash,
            "physicalIdentity": identity,
            "suppliedIdentity": list(subject.file_identity(Path(self.bash).lstat())),
            "sha256": subject.sha(raw)}

    def actual_capture(self, code, allowance=2, **kw):
        out, err = io.BytesIO(), io.BytesIO()
        result, detail = subject.capture([sys.executable, "-c", code], allowance,
            output=out, error_output=err, hard_deadline=time.monotonic() + allowance + 2, **kw)
        self.assertLessEqual(detail["stdout"]["bytes"], subject.STREAM_LIMIT + 1)
        self.assertLessEqual(detail["stderr"]["bytes"], subject.STREAM_LIMIT + 1)
        self.assertEqual(detail["stdout"]["sha256"], subject.sha(out.getvalue()))
        self.assertEqual(detail["stderr"]["sha256"], subject.sha(err.getvalue()))
        return result, detail, out.getvalue(), err.getvalue()

    def metadata(self, canary="PRIVATE-CANARY"):
        values = {"source.json": {"source": "a" * 40},
                  "identity.json": {"privateObservation": canary},
                  "diagnostic.json": {"scope": "offline"}}
        for name, value in values.items():
            subject.write_json(self.private / name, value)
        expected = subject.validate_private(self.private, values, [], None)
        return values, expected

    def summary(self, members, archive, cipher):
        return {"schema": subject.SCHEMA, **CONTEXT, "sourceMatchedBefore": True,
            "sourceMatchedAfter": False, "environmentMatched": False,
            "reason": "ENVIRONMENT_CHANGED", "prerequisiteStatus": "NOT_STARTED",
            "captureComplete": True,
            "stages": [{"stage": name, "status": "NOT_STARTED"} for name, _ in subject.SCRIPTS],
            "members": members, "plaintextArchive": archive, "ciphertext": cipher,
            "recipientSha256": subject.RECIPIENT_SHA256, "elapsedMs": 10,
            "budgetsSeconds": {"install": 180, "generate": 60, "aggregate": 240,
                              "preflight": 30, "encryption": 20, "outer": 310},
            "simulatorStarted": False, "nativeTestStarted": False,
            "compilerStarted": False, "modelRequestStarted": False}

    def adapter(self):
        clock = subject.MonotonicClock()
        clock()
        (self.private / "builds/project-prerequisite").mkdir(exist_ok=True)
        return subject.StageAdapter(self.project_repo, self.private, self.bash, clock,
                                    time.monotonic() + 310, self.shell_observation)

    def handles(self, stage):
        return ((self.private / "builds/project-prerequisite" / (stage + ".stdout")).open("xb"),
                (self.private / "builds/project-prerequisite" / (stage + ".stderr")).open("xb"))

    def owned_worker_state(self, pid):
        query = subprocess.run(["ps", "-p", str(pid), "-o", "stat="],
                               capture_output=True, timeout=1, check=False)
        state = query.stdout.strip()
        self.assertIn(query.returncode, (0, 1))
        return "ABSENT" if query.returncode == 1 and not state else \
            "ZOMBIE" if state.startswith(b"Z") else "RUNNING"

    def test_original_prerequisite_and_all_budgets_are_unchanged(self):
        self.assertEqual(subject.prerequisite.BUDGET_SECONDS, 240)
        self.assertEqual(subject.prerequisite.STAGE_SECONDS, {"install": 180, "generate": 60})
        self.assertEqual(subject.OUTER_SECONDS, 310)
        self.assertEqual(subject.PREFLIGHT_SECONDS, 30)
        self.assertEqual(subject.ENCRYPT_SECONDS, 20)
        self.assertEqual(subject.STREAM_LIMIT, 2 * 1024 * 1024)
        self.assertEqual(subject.ARCHIVE_LIMIT, 9 * 1024 * 1024)
        self.assertEqual(subject.METADATA_LIMIT, 64 * 1024)
        self.assertEqual(subject.sha(Path(subject.prerequisite.__file__).read_bytes()),
                         "6c0bed088dc53e73af138221ce72a6faff0dfa0bc0279cc254ae622dd54bd843")

    def test_rejection_location_does_not_publish_private_exception_fields(self):
        out = io.StringIO()
        for error in (subject.Rejected("PRIVATE_TOKEN_PATH_SERVICE_OUTPUT"),
                      OSError("PRIVATE_TOKEN_PATH_SERVICE_OUTPUT")):
            with contextlib.redirect_stdout(out):
                subject.report_rejection(error, "COLLECTOR", subject.__file__)
        self.assertNotIn("PRIVATE_TOKEN", out.getvalue())
        rows = [json.loads(line) for line in out.getvalue().splitlines()]
        self.assertEqual([row["kind"] for row in rows], ["REJECTED", "OS_ERROR"])
        for row in rows:
            self.assertEqual(set(row), {"diagnostic", "phase", "sourceLine", "kind"})
            self.assertIsNone(row["sourceLine"])
            self.assertEqual(row["phase"], "COLLECTOR")

    @unittest.skipUnless(os.name == "posix", "actual cancellation requires POSIX")
    def test_actual_signal_before_popen_returns_reaps_owned_capture_child(self):
        original = subject.subprocess.Popen
        children = []
        def cancelled(kind, frame):
            raise subject.ObservationExpired("DIAGNOSTIC_DEADLINE_OR_INTERRUPTION")
        previous = subject.signal.signal(subject.signal.SIGTERM, cancelled)
        def interrupt_before_return(*args, **kwargs):
            child = original(*args, **kwargs)
            children.append(child)
            os.kill(os.getpid(), subject.signal.SIGTERM)
            return child
        try:
            with mock.patch.object(subject.subprocess, "Popen", interrupt_before_return):
                with self.assertRaises(subject.ObservationExpired):
                    self.actual_capture("import time;time.sleep(10)")
            self.assertEqual(len(children), 1)
            self.assertIsNotNone(children[0].poll(), "owned capture child survived cancellation")
            self.assertIs(subject.signal.getsignal(subject.signal.SIGTERM), cancelled)
        finally:
            subject.signal.signal(subject.signal.SIGTERM, previous)
            for child in children:
                if child.poll() is None:
                    os.killpg(child.pid, subject.signal.SIGKILL)
                    child.wait(timeout=2)

    def test_actual_two_stream_capture_retains_exact_originals(self):
        result, detail, out, err = self.actual_capture(
            "import os;os.write(1,b'ORIGINAL_STDOUT\\x00\\xff');os.write(2,b'ORIGINAL_STDERR\\n')")
        self.assertEqual(result["result"], "PASS")
        self.assertEqual(result["exitCode"], 0)
        self.assertEqual(out, b"ORIGINAL_STDOUT\x00\xff")
        self.assertEqual(err, b"ORIGINAL_STDERR\n")
        self.assertEqual(detail["stdin"], "DEVNULL")
        self.assertTrue(detail["stdout"]["eof"] and detail["stderr"]["eof"])
        self.assertTrue(detail["cleanupParentReaped"])
        self.assertFalse(detail["groupPresentAfterCleanup"])

    def test_parent_exit_does_not_release_a_descendant_held_pipe(self):
        worker = "import os,time;os.write(1,('CHILD:'+str(os.getpid())+'\\n').encode());time.sleep(12)"
        code = "import subprocess,sys;subprocess.Popen([sys.executable,'-c'," + repr(worker) + "])"
        started = time.monotonic()
        result, detail, out, _ = self.actual_capture(code, 0.5)
        self.assertLess(time.monotonic() - started, 2.5)
        self.assertEqual(result["result"], "TIMEOUT")
        self.assertEqual(detail["returnCodeObserved"], 0)
        self.assertTrue(detail["cleanupGroupSignalSent"])
        pid = int(out.split(b"CHILD:")[1].splitlines()[0])
        self.assertNotEqual(pid, detail["pid"])
        self.assertIn(self.owned_worker_state(pid), ("ABSENT", "ZOMBIE"))

    def test_group_probe_classifies_absence_and_unknown_separately(self):
        for error, state, present in (
            (None, "PRESENT", True),
            (ProcessLookupError(errno.ESRCH, "synthetic missing group"), "ABSENT", False),
            (PermissionError(errno.EPERM, "synthetic denied query"), "PERMISSION_DENIED", None),
            (OSError(errno.EIO, "synthetic query error"), "QUERY_FAILED", None),
        ):
            with self.subTest(state=state), mock.patch.object(subject.os, "killpg", side_effect=error):
                observation = subject.group_observation(123)
                self.assertEqual(observation["state"], state)
                self.assertIs(observation["present"], present)
                self.assertEqual(observation["errno"], error.errno if present is None else None)

    def test_permission_denied_probe_cannot_promote_zero_exit(self):
        actual_killpg = os.killpg
        for error, state in (
            (PermissionError(errno.EPERM, "synthetic denied query"), "PERMISSION_DENIED"),
            (OSError(errno.EIO, "synthetic query error"), "QUERY_FAILED"),
        ):
            def query_denied(pid, sig):
                if sig == 0:
                    raise error
                return actual_killpg(pid, sig)
            with self.subTest(state=state), mock.patch.object(subject.os, "killpg", side_effect=query_denied):
                result, detail, out, err = self.actual_capture("import os;os.write(1,b'EXACT-OUT');os.write(2,b'EXACT-ERR')")
            self.assertEqual(result["result"], "CAPTURE_INCOMPLETE")
            self.assertEqual(result["exitCode"], 0)
            self.assertEqual((out, err), (b"EXACT-OUT", b"EXACT-ERR"))
            self.assertIsNone(detail["groupPresentAfterCleanup"])
            self.assertEqual(detail["groupBeforeCleanup"]["state"], state)
            self.assertEqual(detail["groupAfterCleanup"]["state"], state)
            self.assertTrue(detail["stdout"]["eof"] and detail["stderr"]["eof"])
            self.assertTrue(detail["cleanupParentReaped"])
            self.assertIn("finishedAt", detail)

    def test_permission_denied_probe_preserves_held_pipe_timeout(self):
        actual_killpg = os.killpg
        def query_denied(pid, sig):
            if sig == 0:
                raise PermissionError(errno.EPERM, "synthetic denied post-cleanup query")
            return actual_killpg(pid, sig)
        worker = "import os,time;os.write(1,('CHILD:'+str(os.getpid())+'\\n').encode());time.sleep(12)"
        code = "import subprocess,sys;subprocess.Popen([sys.executable,'-c'," + repr(worker) + "])"
        started = time.monotonic()
        with mock.patch.object(subject.os, "killpg", side_effect=query_denied):
            result, detail, out, _ = self.actual_capture(code, 0.5)
        self.assertLess(time.monotonic() - started, 2.5)
        self.assertEqual(result["result"], "TIMEOUT")
        self.assertEqual(detail["returnCodeObserved"], 0)
        self.assertTrue(detail["cleanupGroupSignalSent"])
        self.assertEqual(detail["cleanupGroupSignal"]["state"], "SENT")
        self.assertIsNone(detail["groupPresentAfterCleanup"])
        self.assertEqual(detail["groupAfterCleanup"]["state"], "PERMISSION_DENIED")
        pid = int(out.split(b"CHILD:")[1].splitlines()[0])
        self.assertNotEqual(pid, detail["pid"])
        self.assertIn(self.owned_worker_state(pid), ("ABSENT", "ZOMBIE"))

    def test_permission_denied_signal_keeps_cleanup_incomplete(self):
        actual_killpg = os.killpg
        owned = []
        def signal_denied(pid, sig):
            if sig == subject.signal.SIGKILL:
                owned.append(pid)
                raise PermissionError(errno.EPERM, "synthetic denied cleanup signal")
            return actual_killpg(pid, sig)
        worker = "import os,time;os.write(1,('CHILD:'+str(os.getpid())+'\\n').encode());time.sleep(12)"
        code = "import subprocess,sys;subprocess.Popen([sys.executable,'-c'," + repr(worker) + "])"
        pid = None
        try:
            started = time.monotonic()
            with mock.patch.object(subject.os, "killpg", side_effect=signal_denied):
                result, detail, out, _ = self.actual_capture(code, 0.5)
            self.assertLess(time.monotonic() - started, 2.5)
            self.assertEqual(result["result"], "TIMEOUT")
            self.assertEqual(detail["returnCodeObserved"], 0)
            self.assertFalse(detail["cleanupGroupSignalSent"])
            self.assertEqual(detail["cleanupGroupSignal"]["state"], "PERMISSION_DENIED")
            self.assertIsNot(detail["groupPresentAfterCleanup"], False)
            pid = int(out.split(b"CHILD:")[1].splitlines()[0])
            self.assertEqual(self.owned_worker_state(pid), "RUNNING")
            self.assertIn("finishedAt", detail)
        finally:
            # The contrary deliberately denied the production signal. Only this
            # observed owned group is cleaned with the captured real operation.
            for group in owned:
                try:
                    actual_killpg(group, subject.signal.SIGKILL)
                except ProcessLookupError:
                    pass
            if pid is not None:
                deadline = time.monotonic() + 1
                while self.owned_worker_state(pid) == "RUNNING" and time.monotonic() < deadline:
                    time.sleep(0.01)
                self.assertIn(self.owned_worker_state(pid), ("ABSENT", "ZOMBIE"))

    def test_stdout_overflow_keeps_only_the_bounded_prefix(self):
        result, detail, out, _ = self.actual_capture("import os;os.write(1,b'x'*10000)", stream_limit=64)
        self.assertEqual(result["result"], "OUTPUT_OVERFLOW")
        self.assertEqual(out, b"x" * 65)
        self.assertTrue(detail["stdout"]["overflow"])

    def test_stderr_overflow_is_independently_bounded(self):
        result, detail, out, err = self.actual_capture("import os;os.write(2,b'y'*10000)", stream_limit=64)
        self.assertEqual(result["result"], "OUTPUT_OVERFLOW")
        self.assertEqual(out, b"")
        self.assertEqual(err, b"y" * 65)
        self.assertTrue(detail["stderr"]["overflow"])

    def test_expired_allowance_never_launches_and_boolean_is_rejected(self):
        with mock.patch.object(subject.subprocess, "Popen") as launch:
            result, _, _, _ = self.actual_capture("raise SystemExit(0)", 0)
            self.assertEqual(result["result"], "DEADLINE_EXHAUSTED")
            launch.assert_not_called()
            with self.assertRaisesRegex(subject.Rejected, "ALLOWANCE_TYPE"):
                self.actual_capture("raise SystemExit(0)", True)
            launch.assert_not_called()

    def test_late_zero_exit_is_not_promoted(self):
        # Actual process outputs remain real. Force only the observed final
        # monotonic sample later; no delayed process is called a successful one.
        actual = subject.time.monotonic
        calls = {"final": False}
        real_wait = subject.subprocess.Popen.wait
        def waited(proc, *a, **kw):
            result = real_wait(proc, *a, **kw)
            calls["final"] = True
            return result
        def now():
            return actual() + (3 if calls["final"] else 0)
        with mock.patch.object(subject.subprocess.Popen, "wait", waited), \
             mock.patch.object(subject.time, "monotonic", now):
            result, detail, _, _ = self.actual_capture("pass", 2)
        self.assertEqual(detail["returnCodeObserved"], 0)
        self.assertEqual(result["result"], "DEADLINE_EXHAUSTED")

    def test_exact_command_cwd_and_handle_router_rejects_substitution(self):
        adapter = self.adapter()
        with contextlib.ExitStack() as stack:
            out, err = [stack.enter_context(v) for v in self.handles("install")]
            wrong = stack.enter_context((self.private / "wrong").open("xb"))
            valid = [self.bash, "-c", subject.SCRIPTS[0][1]]
            with mock.patch.object(subject, "capture") as launch:
                with cwd(self.project):
                    for command, output, error in [
                        ([self.bash, "-c", "printf substituted"], out, err),
                        ([self.bash, "-c", subject.SCRIPTS[1][1]], out, err),
                        (valid, wrong, err), (valid, out, out)]:
                        with self.subTest(command=command[-1], output=str(output.name)):
                            with self.assertRaises(subject.Rejected):
                                adapter(command, 180, output=output, error_output=error)
                with self.assertRaisesRegex(subject.Rejected, "STAGE_CWD"):
                    adapter(valid, 180, output=out, error_output=err)
                launch.assert_not_called()

    def test_repeated_stage_and_allowance_inflation_are_rejected(self):
        adapter = self.adapter()
        with contextlib.ExitStack() as stack, cwd(self.project):
            out, err = [stack.enter_context(v) for v in self.handles("install")]
            with mock.patch.object(subject, "capture") as launch:
                for value in (181, True, float("nan"), float("inf"), 179):
                    with self.subTest(allowance=str(value)):
                        with self.assertRaisesRegex(subject.Rejected, "ALLOWANCE_CHANGED"):
                            adapter([self.bash, "-c", subject.SCRIPTS[0][1]], value,
                                    output=out, error_output=err)
                adapter.records = [{"stage": "install"}]
                with self.assertRaisesRegex(subject.Rejected, "UNAPPROVED_COMMAND_OR_ORDER"):
                    adapter([self.bash, "-c", subject.SCRIPTS[0][1]], 60,
                            output=out, error_output=err)
                adapter.records.append({"stage": "generate"})
                with self.assertRaisesRegex(subject.Rejected, "REPEATED_STAGE"):
                    adapter([self.bash, "-c", "anything"], 60, output=out, error_output=err)
                launch.assert_not_called()

    def test_shell_replacement_rejected_before_stage(self):
        shell = self.root / "bash"
        shell.write_bytes(b"same shell bytes")
        shell.chmod(0o700)
        raw, identity = subject.bound_read(shell, 1024)
        observation = {"physical": str(shell), "physicalIdentity": identity,
                       "suppliedIdentity": identity, "sha256": subject.sha(raw)}
        subject.verify_shell(str(shell), observation)
        replacement = self.root / "replacement"
        replacement.write_bytes(raw)
        replacement.chmod(0o700)
        replacement.replace(shell)
        with self.assertRaisesRegex(subject.Rejected, "BASH_CHANGED"):
            subject.verify_shell(str(shell), observation)

    def test_original_prepare_stops_after_actual_install_timeout(self):
        def run(command, allowance, *, output, error_output):
            return subject.capture([sys.executable, "-c", "import time;time.sleep(12)"],
                0.05, output=output, error_output=error_output,
                hard_deadline=time.monotonic() + 2)[0]
        code, receipt = subject.prerequisite.prepare(self.project_repo, self.private, CONTEXT,
                                                     self.bash, runner=run)
        self.assertEqual(code, 70)
        self.assertEqual(receipt["status"], "FAILED")
        self.assertEqual([x["stage"] for x in receipt["commands"]], ["install"])
        self.assertEqual(receipt["commands"][0]["result"], "TIMEOUT")
        self.assertFalse((self.private / "builds/project-prerequisite/generate.stdout").exists())

    def test_original_prepare_keeps_actual_generation_nonzero(self):
        counter = []
        def run(command, allowance, *, output, error_output):
            counter.append(command)
            code = "pass" if len(counter) == 1 else "import os;os.write(2,b'GENERATION_ORIGINAL');raise SystemExit(11)"
            return subject.capture([sys.executable, "-c", code], 2, output=output,
                                   error_output=error_output, hard_deadline=time.monotonic() + 3)[0]
        code, receipt = subject.prerequisite.prepare(self.project_repo, self.private, CONTEXT,
                                                     self.bash, runner=run)
        self.assertEqual(code, 70)
        self.assertEqual(receipt["commands"][-1]["exitCode"], 11)
        self.assertEqual((self.private / "builds/project-prerequisite/generate.stderr").read_bytes(),
                         b"GENERATION_ORIGINAL")

    def test_original_aggregate_exhaustion_cannot_start_generation(self):
        now = [0.0]
        def run(command, allowance, *, output, error_output):
            self.assertEqual(allowance, 180)
            now[0] = 240.0
            return {"result": "PASS", "exitCode": 0, "elapsedMs": 240000}
        code, receipt = subject.prerequisite.prepare(self.project_repo, self.private, CONTEXT,
                                                    self.bash, runner=run, now=lambda: now[0])
        self.assertEqual(code, 70)
        self.assertEqual(len(receipt["commands"]), 1)
        self.assertEqual(receipt["elapsedMs"], 240000)

    def test_interrupted_stage_cleanup_never_marks_complete(self):
        actual_read = subject.os.read
        once, selected = [False], [False]
        actual_selector = subject.selectors.DefaultSelector
        def selector():
            actual = actual_selector()
            wrapper = mock.Mock(wraps=actual)
            def select(*a, **kw):
                events = actual.select(*a, **kw)
                if events:
                    selected[0] = True
                return events
            wrapper.select.side_effect = select
            return wrapper
        def failed(fd, count):
            if selected[0] and not once[0]:
                once[0] = True
                raise OSError("synthetic drain failure")
            return actual_read(fd, count)
        real_kill = subject.kill_group
        killed = []
        def killing(process):
            killed.append(process.pid)
            return real_kill(process)
        with mock.patch.object(subject.os, "read", failed), \
             mock.patch.object(subject, "kill_group", killing), \
             mock.patch.object(subject.selectors, "DefaultSelector", selector):
            with self.assertRaises(OSError):
                self.actual_capture("import os,time;os.write(1,b'ready');time.sleep(12)")
        self.assertEqual(len(killed), 1)
        self.assertIn(self.owned_worker_state(killed[0]), ("ABSENT", "ZOMBIE"))

    def test_same_byte_replacement_during_read_is_rejected(self):
        path = self.root / "original"
        path.write_bytes(b"actual bytes")
        real_open = Path.open
        def replacing(p, *a, **kw):
            result = real_open(p, *a, **kw)
            if p == path and a and a[0] == "rb":
                other = self.root / "same-byte-replacement"
                other.write_bytes(b"actual bytes")
                other.replace(path)
            return result
        with mock.patch.object(Path, "open", replacing):
            with self.assertRaisesRegex(subject.Rejected, "FILE_CHANGED"):
                subject.read_regular(path, 64)

    def test_real_alias_after_physical_read_is_rejected(self):
        actual = self.root / "actual"
        actual.mkdir()
        path = actual / "data"
        path.write_bytes(b"bound")
        self.assertEqual(subject.read_regular(path, 64), b"bound")
        link = self.root / "alias"
        link.symlink_to(actual, target_is_directory=True)
        with self.assertRaisesRegex(subject.Rejected, "ANCESTOR_NOT_DIRECTORY"):
            subject.read_regular(link / "data", 64)

    def test_invalid_metadata_is_rejected_at_read_and_write(self):
        for raw in (b'{"a":1,"a":2}', b'{"a":NaN}', b'{"a":Infinity}', b'{"a":-Infinity}', b'{"a":1e999}'):
            with self.subTest(raw=raw):
                path = self.root / "bad.json"
                path.write_bytes(raw)
                with self.assertRaises(subject.Rejected):
                    subject.read_json(path)
        with self.assertRaises(ValueError):
            subject.write_json(self.root / "nan.json", {"bad": float("nan")})
        with self.assertRaisesRegex(subject.Rejected, "METADATA_LIMIT"):
            subject.write_json(self.root / "large.json", {"large": "x" * subject.METADATA_LIMIT})
        valid = self.summary([], {"bytes": 1, "sha256": "b" * 64}, {"bytes": 1, "sha256": "c" * 64})
        valid["elapsedMs"] = True
        with self.assertRaisesRegex(subject.Rejected, "PUBLIC_ELAPSED"):
            subject.validate_public_summary(valid)
        _, expected = self.metadata()
        _, archive_binding, members = subject.make_archive(self.private, CONTEXT, [], expected)
        valid = self.summary(members, archive_binding, {"bytes": 1, "sha256": "c" * 64})
        subject.validate_public_summary(valid)
        valid["budgetsSeconds"]["install"] = 180.0
        with self.assertRaisesRegex(subject.Rejected, "PUBLIC_BUDGET"):
            subject.validate_public_summary(valid)

    def test_missing_or_changed_private_member_prevents_archive(self):
        metadata, expected = self.metadata()
        (self.private / "identity.json").write_bytes(b'{"changed":true}\n')
        with self.assertRaisesRegex(subject.Rejected, "ORIGINAL_CHANGED_BEFORE_ARCHIVE"):
            subject.make_archive(self.private, CONTEXT, [], expected)
        (self.private / "identity.json").write_bytes(subject.canonical(metadata["identity.json"]))
        for expected_number, changed_scalar in ((0, False), (1, True), (1, 1.0)):
            with self.subTest(expected=expected_number, changed=changed_scalar,
                              changed_type=type(changed_scalar).__name__):
                metadata["diagnostic.json"] = {"counter": expected_number}
                path = self.private / "diagnostic.json"
                path.write_bytes(subject.canonical(metadata["diagnostic.json"]))
                subject.validate_private(self.private, metadata, [], None)
                path.write_bytes(subject.canonical({"counter": changed_scalar}))
                with self.assertRaisesRegex(subject.Rejected, "METADATA_CHANGED"):
                    subject.validate_private(self.private, metadata, [], None)
        (self.private / "diagnostic.json").write_bytes(subject.canonical(metadata["diagnostic.json"]))
        (self.private / "unaccounted.stdout").write_bytes(b"retained")
        with self.assertRaisesRegex(subject.Rejected, "UNACCOUNTED_PRIVATE_MEMBER"):
            subject.validate_private(self.private, metadata, [], None)

    def test_archive_limit_rejects_before_plaintext_creation(self):
        _, expected = self.metadata()
        with mock.patch.object(subject, "ARCHIVE_LIMIT", 10):
            with self.assertRaisesRegex(subject.Rejected, "ARCHIVE_LIMIT"):
                subject.make_archive(self.private, CONTEXT, [], expected)
        self.assertFalse((self.private / "project-prerequisite.zip").exists())

    def test_wrong_recipient_cannot_start_encryption(self):
        _, expected = self.metadata()
        archive, archive_binding, _ = subject.make_archive(self.private, CONTEXT, [], expected)
        recipient = self.root / "wrong.pem"
        recipient.write_bytes(b"wrong")
        with mock.patch.object(subject, "capture") as launch:
            with self.assertRaisesRegex(subject.Rejected, "RECIPIENT_CHANGED"):
                subject.encrypt_archive(self.private, self.public, archive, archive_binding,
                                        recipient, time.monotonic() + 20)
            launch.assert_not_called()

    def test_failed_and_late_encryption_never_publish(self):
        _, expected = self.metadata()
        archive, archive_binding, _ = subject.make_archive(self.private, CONTEXT, [], expected)
        recipient = self.root / "test-recipient"
        recipient.write_bytes(b"explicit synthetic recipient")
        for result in ("EXIT_NONZERO", "TIMEOUT", "DEADLINE_EXHAUSTED"):
            with self.subTest(result=result):
                for name in ("encryption.stdout", "encryption.stderr", "encryption.process.json"):
                    (self.private / name).unlink(missing_ok=True)
                with mock.patch.object(subject, "RECIPIENT_SHA256", subject.sha(recipient.read_bytes())), \
                     mock.patch.object(subject, "capture", return_value=(
                         {"result": result, "exitCode": 0 if result == "DEADLINE_EXHAUSTED" else None,
                          "elapsedMs": 20}, {})):
                    with self.assertRaisesRegex(subject.Rejected, "ENCRYPTION_FAILED"):
                        subject.encrypt_archive(self.private, self.public, archive, archive_binding,
                                                recipient, time.monotonic() + 20)
                self.assertFalse((self.public / "summary.json").exists())

    def test_ciphertext_and_extra_public_member_are_rejected(self):
        _, expected = self.metadata()
        _, archive_binding, members = subject.make_archive(self.private, CONTEXT, [], expected)
        cipher = self.public / "project-prerequisite.zip.p7m"
        cipher.write_bytes(b"\x30synthetic")
        summary = self.summary(members, archive_binding,
                               {"bytes": cipher.stat().st_size, "sha256": subject.sha(cipher.read_bytes())})
        subject.write_json(self.public / "summary.json", summary)
        subject.verify_public(self.public, summary)
        cipher.write_bytes(b"\x30tampered")
        with self.assertRaisesRegex(subject.Rejected, "PUBLIC_CIPHERTEXT_CHANGED"):
            subject.verify_public(self.public, summary)
        cipher.write_bytes(b"\x30synthetic")
        (self.public / "plain.zip").write_bytes(b"forbidden")
        with self.assertRaisesRegex(subject.Rejected, "PUBLICATION_MEMBER"):
            subject.verify_public(self.public, summary)

    def test_public_summary_cannot_carry_arbitrary_private_fields(self):
        _, expected = self.metadata()
        _, archive_binding, members = subject.make_archive(self.private, CONTEXT, [], expected)
        summary = self.summary(members, archive_binding, {"bytes": 1, "sha256": "b" * 64})
        subject.validate_public_summary(summary)
        for mutate in (
            lambda x: x.update(rawError="/private/token?secret=canary"),
            lambda x: x.update(formalAcceptanceGranted=True),
            lambda x: x["members"][0].update(name="/private/path"),
            lambda x: x["stages"][0].update(exitCode=False),
        ):
            item = copy.deepcopy(summary)
            mutate(item)
            with self.assertRaises(subject.Rejected):
                subject.validate_public_summary(item)

    def test_actual_synthetic_crypto_roundtrip_keeps_canary_private(self):
        # This real offline roundtrip uses an ephemeral test recipient. The
        # production certificate digest is independently pinned, never relaxed
        # in the dispatched collector; no production private key is available here.
        canary = "https://private.invalid/path?secret=PREREQUISITE_CANARY"
        _, expected = self.metadata(canary)
        archive, archive_binding, members = subject.make_archive(self.private, CONTEXT, [], expected)
        key, cert = self.root / "offline.key", self.root / "offline.crt"
        create = subprocess.run(["/usr/bin/openssl", "req", "-x509", "-newkey", "rsa:2048",
            "-nodes", "-keyout", str(key), "-out", str(cert), "-days", "1",
            "-subj", "/CN=CherriOfflineControl"], stdin=subprocess.DEVNULL,
            capture_output=True, timeout=10, check=False)
        self.assertEqual(create.returncode, 0)
        with mock.patch.object(subject, "RECIPIENT_SHA256", subject.sha(cert.read_bytes())):
            cipher = subject.encrypt_archive(self.private, self.public, archive, archive_binding,
                                             cert, time.monotonic() + 20)
            summary = self.summary(members, archive_binding, cipher)
            subject.write_json(self.public / "summary.json", summary)
            subject.verify_public(self.public, summary)
        decoded = self.root / "decoded.zip"
        dec = subprocess.run(["/usr/bin/openssl", "smime", "-decrypt", "-binary", "-inform", "DER",
            "-in", str(self.public / "project-prerequisite.zip.p7m"),
            "-recip", str(cert), "-inkey", str(key), "-out", str(decoded)],
            stdin=subprocess.DEVNULL, capture_output=True, timeout=10, check=False)
        self.assertEqual(dec.returncode, 0)
        self.assertEqual(decoded.read_bytes(), archive.read_bytes())
        with zipfile.ZipFile(decoded) as sealed:
            self.assertIn(canary.encode(), sealed.read("identity.json"))
        self.assertNotIn(canary.encode(), (self.public / "summary.json").read_bytes())
        self.assertNotIn(canary.encode(), (self.public / "project-prerequisite.zip.p7m").read_bytes())
        self.assertEqual(set(p.name for p in self.public.iterdir()), subject.PUBLIC_FILES)

    def test_actual_fixed_stage_adapter_preserves_prepared_receipt_and_output(self):
        bins = self.root / "owned-bin"
        bins.mkdir()
        generator = self.root / "owned-generator.py"
        generator.write_text(
            "from pathlib import Path\n"
            "p=Path('CherriSimulatorQA.xcodeproj');p.mkdir()\n"
            "(p/'project.pbxproj').write_bytes(b'PUBLIC_SYNTHETIC_PROJECT')\n"
            "print('ORIGINAL_GENERATION_OUTPUT')\n")
        executable = bins / "xcodegen"
        executable.write_text("#!" + self.bash + "\nexec " + shlex.quote(sys.executable) +
                              " " + shlex.quote(str(generator)) + "\n")
        executable.chmod(0o700)
        clock = subject.MonotonicClock()
        adapter = subject.StageAdapter(self.project_repo, self.private, self.bash, clock,
                                       time.monotonic() + 310, self.shell_observation)
        with mock.patch.dict(os.environ, {"PATH": str(bins)}):
            self.assertEqual(shutil.which("xcodegen"), str(executable))
            code, receipt = subject.prerequisite.prepare(self.project_repo, self.private,
                CONTEXT, self.bash, runner=adapter, now=clock)
        self.assertEqual(code, 0)
        self.assertEqual(receipt["status"], "PREPARED")
        self.assertEqual(receipt["projectSha256"], subject.sha(b"PUBLIC_SYNTHETIC_PROJECT"))
        self.assertEqual([x["stage"] for x in adapter.records], ["install", "generate"])
        stages = subject.stage_summary(self.private, adapter.records)
        self.assertEqual([x["status"] for x in stages], ["PASS", "PASS"])
        self.assertTrue(all(stream["complete"] for row in stages for stream in row["streams"].values()))
        metadata = {"source.json": {"control": "offline"},
                    "identity.json": {"control": "offline"},
                    "diagnostic.json": {"control": "offline"}}
        for name, value in metadata.items():
            subject.write_json(self.private / name, value)
        expected = subject.validate_private(self.private, metadata, adapter.records, receipt)
        archive, _, members = subject.make_archive(self.private, CONTEXT, adapter.records, expected)
        self.assertEqual(len(members), 12)
        with zipfile.ZipFile(archive) as z:
            self.assertEqual(z.read("builds/project-prerequisite/receipt.json"),
                             (self.private / "builds/project-prerequisite/receipt.json").read_bytes())
        process_path = self.private / "install.process.json"
        original_process = process_path.read_bytes()
        altered = json.loads(original_process)
        self.assertEqual(type(altered["exitCode"]), int)
        self.assertEqual(altered["exitCode"], 0)
        altered["exitCode"] = False
        process_path.write_bytes(subject.canonical(altered))
        with self.assertRaisesRegex(subject.Rejected, "PROCESS_RECEIPT_CHANGED"):
            subject.stage_summary(self.private, adapter.records)
        process_path.write_bytes(original_process)
        output = self.private / "builds/project-prerequisite/generate.stdout"
        output.write_bytes(b"ORIGINAL_GENERATION_OUTPUT\n")
        with self.assertRaisesRegex(subject.Rejected, "STREAM_IDENTITY_OR_BYTES_CHANGED"):
            subject.stage_summary(self.private, adapter.records)

    def test_source_binding_uses_actual_git_objects_and_rejects_mutation(self):
        repo = self.root / "source-repo"
        repo.mkdir()
        certificate = HERE / "xcode_environment_inventory_recipient.pem"
        for name in subject.SOURCE_FILES:
            path = repo / name
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_bytes(certificate.read_bytes() if name == subject.RECIPIENT
                             else ("OWNED_OFFLINE_WITNESS:" + name).encode())
        def git(*args):
            result = subprocess.run(["git", *args], cwd=repo, capture_output=True,
                                    timeout=5, check=False)
            self.assertEqual(result.returncode, 0, result.stderr.decode(errors="replace"))
            return result.stdout
        git("init", "-q")
        git("add", "--", ".")
        git("-c", "user.name=Cherri_QA", "-c", "user.email=qa@example.invalid",
            "-c", "commit.gpgsign=false", "commit", "-qm", "owned immutable source control")
        source = git("rev-parse", "HEAD").decode().strip()
        with cwd(repo):
            before = subject.source_binding(repo, source, time.monotonic() + 15, clean=True)
            self.assertEqual(len(before["files"]), len(subject.SOURCE_FILES))
            subject.confirm_sources(repo, before)
            spec = repo / subject.QA / "XcodeProject/project.yml"
            spec.write_bytes(spec.read_bytes() + b"CHANGED")
            with self.assertRaisesRegex(subject.Rejected, "SOURCE_BYTES"):
                subject.source_binding(repo, source, time.monotonic() + 15, clean=False)
            with self.assertRaisesRegex(subject.Rejected, "SOURCE_CHANGED_AT_SEAL"):
                subject.confirm_sources(repo, before)

    def test_environment_version_or_selected_developer_change_is_explicit(self):
        commands = []
        def command(argv, deadline, maximum):
            commands.append(argv)
            if argv == ["/usr/bin/sw_vers"]:
                return b"ProductName:\tmacOS\nProductVersion:\t27.0.1\nBuildVersion:\t26A434\n", {}
            if argv == ["/usr/bin/xcode-select", "-p"]:
                return b"/Applications/Xcode_27.1.app/Contents/Developer\n", {}
            if argv == ["/usr/bin/xcodebuild", "-version"]:
                return b"Xcode 27.1\nBuild version 27A9275\n", {}
            self.fail("unexpected command")
        with mock.patch.object(subject, "small_command", side_effect=command):
            observation = subject.environment_binding(self.bash, time.monotonic() + 30)
        self.assertTrue(observation["matched"])
        self.assertEqual(len(commands), 3)
        def changed(argv, deadline, maximum):
            if argv == ["/usr/bin/xcode-select", "-p"]:
                return b"/Applications/Foreign.app/Contents/Developer\n", {}
            return command(argv, deadline, maximum)
        with mock.patch.object(subject, "small_command", side_effect=changed):
            self.assertFalse(subject.environment_binding(self.bash, time.monotonic() + 30)["matched"])


if __name__ == "__main__":
    unittest.main()
