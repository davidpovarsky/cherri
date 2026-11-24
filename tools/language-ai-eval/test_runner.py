import copy
import importlib.util
import json
import os
from pathlib import Path
import signal
import subprocess
import sys
import tempfile
import time
import unittest
from unittest.mock import patch


ROOT = Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location("language_ai_eval_runner", ROOT / "runner.py")
runner = importlib.util.module_from_spec(spec)
spec.loader.exec_module(runner)


class EvaluationHarnessTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.guide = self.root / "guide.md"
        self.guide.write_text("Synthetic control context only", encoding="utf-8")
        self.output = self.root / "report.json"

    def cli(self, *args):
        result = subprocess.run([sys.executable, str(ROOT / "runner.py"), "--output", str(self.output), *args],
                                capture_output=True, text=True, timeout=15)
        report = json.loads(self.output.read_text()) if self.output.exists() else None
        return result, report

    def test_absent_external_configuration_is_not_model_quality(self):
        result, report = self.cli()
        self.assertEqual(result.returncode, 2)
        self.assertEqual(report["status"], "AI_EVAL_NOT_RUN")
        self.assertFalse(report["ai_quality_measured"])
        self.assertEqual(report["records"], [])
        self.assertNotIn("measured_results", report)

    def test_fake_subprocess_covers_both_arms_without_quality_score(self):
        result, report = self.cli("--fake", "--model", "control-model", "--legacy-guide", str(self.guide), "--v2-guide", str(self.guide))
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(report["status"], "HARNESS_CONTROL_PASS")
        self.assertFalse(report["ai_quality_measured"])
        self.assertNotIn("measured_results", report)
        tasks, suite_sha = runner.load_suite(ROOT / "tasks.json")
        self.assertEqual({(r["task_id"], r["arm"]) for r in report["records"]},
                         {(t["id"], a) for t in tasks for a in runner.ARMS})
        self.assertTrue(all(r["process"]["exit_code"] == 0 and r["process"]["synthetic"] for r in report["records"]))
        self.assertTrue(all(r["validator_process"]["exit_code"] == 0 and r["validator_process"]["synthetic"] for r in report["records"]))
        for record in report["records"]:
            task = next(t for t in tasks if t["id"] == record["task_id"])
            self.assertEqual({c["name"] for c in record["control_checks"]}, set(runner.required_checks(task)))
            self.assertNotIn("passed", record)
            request = runner.model_request(task, record["arm"], self.guide.read_text(),
                "control-model", report["budget"], 0, suite_sha)
            source = 'show("HARNESS_CONTROL_ONLY")\n'
            generation = {"schema": runner.SCHEMA, "request_id": request["request_id"],
                "model": "control-model", "synthetic": True, "code": source,
                "usage": {"input_tokens": 1, "output_tokens": 1}}
            validation = {"schema": runner.SCHEMA, "request_id": request["request_id"],
                "task_id": task["id"], "arm": record["arm"], "source_sha256": runner.digest(source.encode())}
            validation.update(synthetic=True, checks=[{"name": name, "passed": True}
                for name in runner.required_checks(task)])
            for key, reply in (("process", generation), ("validator_process", validation)):
                raw = (json.dumps(reply) + os.linesep).encode()
                self.assertEqual(record[key]["stdout_sha256"], runner.digest(raw))
                self.assertEqual(record[key]["stdout_total_bytes"], len(raw))

    def test_duplicate_task_and_missing_arm_fail_closed(self):
        original = json.loads((ROOT / "tasks.json").read_text(encoding="utf-8"))
        mutations = [lambda d: d["tasks"].append(copy.deepcopy(d["tasks"][0])),
                     lambda d: d["tasks"][0]["starter"].pop("v2"),
                     lambda d: d["tasks"][0].pop("oracle")]
        for index, mutate in enumerate(mutations):
            with self.subTest(index=index):
                changed = copy.deepcopy(original)
                mutate(changed)
                suite = self.root / (str(index) + ".json")
                suite.write_text(json.dumps(changed), encoding="utf-8")
                with self.assertRaises(ValueError):
                    runner.load_suite(suite)

    def test_identity_model_and_actual_usage_are_enforced(self):
        request = {"request_id": "request-a", "model": "same-model", "budget": {"max_input_tokens": 10, "max_output_tokens": 4}}
        valid = {"schema": runner.SCHEMA, "request_id": "request-a", "model": "same-model", "code": "show(1)",
                 "usage": {"input_tokens": 8, "output_tokens": 4}}
        self.assertEqual(runner.validate_generation(valid, request)[0], "show(1)")
        valid["usage"]["credential"] = "SECRET_CANARY"
        self.assertNotIn("SECRET_CANARY", json.dumps(runner.validate_generation(valid, request)[1]))
        changes = [lambda d: d.update(request_id="different"), lambda d: d.update(model="different"),
                   lambda d: d["usage"].update(output_tokens=5), lambda d: d["usage"].update(input_tokens=11),
                   lambda d: d["usage"].update(output_tokens=True), lambda d: d.update(code=""),
                   lambda d: d.pop("usage")]
        for index, change in enumerate(changes):
            with self.subTest(index=index):
                reply = copy.deepcopy(valid)
                change(reply)
                with self.assertRaises(ValueError):
                    runner.validate_generation(reply, request)

    def test_validator_requires_exact_source_identity_and_oracle_coverage(self):
        request = {"schema": runner.SCHEMA, "request_id": "request-a", "task_id": "task", "arm": "v2",
                   "source_sha256": "source-hash", "required_checks": ["compiles", "oracle.visible_text"]}
        valid = {key: request[key] for key in ("schema", "request_id", "task_id", "arm", "source_sha256")}
        valid.update(synthetic=False, checks=[{"name": name, "passed": True, "diagnostic": "SECRET_CANARY"}
                                              for name in request["required_checks"]])
        clean = runner.validate_semantics(valid, request, synthetic=False)
        self.assertNotIn("SECRET_CANARY", json.dumps(clean))
        changes = [lambda d: d.update(request_id="other"), lambda d: d.update(task_id="other"),
                   lambda d: d.update(arm="legacy"), lambda d: d.update(source_sha256="other"),
                   lambda d: d.update(synthetic=True), lambda d: d["checks"].pop(),
                   lambda d: d["checks"].append(copy.deepcopy(d["checks"][0])),
                   lambda d: d["checks"][0].update(name="SECRET_CANARY"),
                   lambda d: d["checks"][0].update(passed=1)]
        for index, change in enumerate(changes):
            with self.subTest(index=index):
                reply = copy.deepcopy(valid)
                change(reply)
                with self.assertRaises(ValueError):
                    runner.validate_semantics(reply, request, synthetic=False)
        valid["checks"][1]["passed"] = False
        self.assertFalse(all(c["passed"] for c in runner.validate_semantics(valid, request, synthetic=False)))

    def test_malformed_process_json_and_oversized_stdout_fail_closed(self):
        for script_text in ["print('not-json')", "print('x' * (1024 * 1024 + 1))"]:
            script = self.root / "bad-json.py"
            script.write_text(script_text, encoding="utf-8")
            with self.assertRaises(ValueError):
                runner.run_process([sys.executable, str(script)], {}, 2)

    def test_response_types_are_rejected_without_attribute_errors(self):
        for reply in [None, [], "bad"]:
            with self.subTest(reply=reply), self.assertRaises(ValueError):
                runner.validate_generation(reply, {})
            with self.subTest(reply=reply), self.assertRaises(ValueError):
                runner.validate_semantics(reply, {}, synthetic=False)

    def test_nonzero_secret_stderr_is_never_in_error(self):
        script = self.root / "fault.py"
        script.write_text('import sys\nprint("SECRET_CANARY", file=sys.stderr)\nsys.exit(7)\n', encoding="utf-8")
        with self.assertRaises(ValueError) as error:
            runner.run_process([sys.executable, str(script)], {}, 2)
        self.assertNotIn("SECRET_CANARY", str(error.exception))

    def test_hung_adapter_times_out(self):
        script = self.root / "hang.py"
        script.write_text("import time\ntime.sleep(2)\n", encoding="utf-8")
        with self.assertRaises(subprocess.TimeoutExpired):
            runner.run_process([sys.executable, str(script)], {}, 0.05)
        if os.name == "nt":
            return
        # Synthetic observer controls do not replace the actual timed-out
        # process above or the descendant lock/effect case below.
        with self.subTest(observer="owned_group_absent"):
            with patch.object(runner.os, "killpg", side_effect=ProcessLookupError) as probe, \
                    patch.object(runner.subprocess, "run") as table, \
                    patch.object(runner.time, "monotonic", return_value=10.0):
                runner.wait_for_posix_group_exit(101, 11.0)
                probe.assert_called_once_with(101, 0)
                table.assert_not_called()
        with self.subTest(observer="permission_failure_is_not_absence"):
            with patch.object(runner.os, "killpg", side_effect=PermissionError), \
                    patch.object(runner.subprocess, "run") as table, \
                    patch.object(runner.time, "monotonic", return_value=10.0):
                with self.assertRaisesRegex(ValueError, "could not be observed"):
                    runner.wait_for_posix_group_exit(101, 11.0)
                table.assert_not_called()
        with self.subTest(observer="absence_after_deadline_is_not_success"):
            with patch.object(runner.os, "killpg", side_effect=ProcessLookupError), \
                    patch.object(runner.subprocess, "run") as table, \
                    patch.object(runner.time, "monotonic", side_effect=[10.0, 12.0]):
                with self.assertRaisesRegex(ValueError, "deadline expired"):
                    runner.wait_for_posix_group_exit(101, 11.0)
                table.assert_not_called()
        with self.subTest(observer="live_group_still_requires_observation"):
            with patch.object(runner.os, "killpg", return_value=None), \
                    patch.object(runner.subprocess, "run", side_effect=subprocess.TimeoutExpired(["ps"], 1.0)), \
                    patch.object(runner.time, "monotonic", return_value=10.0):
                with self.assertRaisesRegex(ValueError, "could not be observed"):
                    runner.wait_for_posix_group_exit(101, 11.0)
        for state in ("S", "Zl"):
            with self.subTest(observer="live_or_threaded_zombie_must_not_return", state=state):
                with patch.object(runner.os, "killpg", side_effect=[None, ProcessLookupError]) as probe, \
                        patch.object(runner.subprocess, "run", return_value=subprocess.CompletedProcess(["ps"], 0, "101 " + state + "\n202 Z\n")) as table, \
                        patch.object(runner.time, "monotonic", return_value=10.0):
                    runner.wait_for_posix_group_exit(101, 11.0)
                    self.assertEqual(probe.call_count, 2)
                    table.assert_called_once()
                    self.assertEqual(table.call_args.kwargs["timeout"], 1.0)

    def test_timeout_stops_a_spawned_worker(self):
        script = self.root / "parent.py"
        marker = self.root / "worker-finished"
        started = self.root / "worker-started"
        release = self.root / "worker-release-after-return"
        expired = self.root / "worker-expired"
        lock_path = self.root / "worker.lock"
        lock_path.write_bytes(b"x")
        # Readiness comes from the actual descendant holding an OS lock. Its
        # effect gate is released only after run_process has returned, so work
        # during the existing cancellation allowance is not called a survivor.
        # The bounded worker self-expiry is always an explicit test failure.
        worker = ("import os,time\n"
            "handle=open(" + repr(str(lock_path)) + ",'r+b')\n"
            "if os.name=='nt':\n import msvcrt\n msvcrt.locking(handle.fileno(),msvcrt.LK_NBLCK,1)\n"
            "else:\n import fcntl\n fcntl.flock(handle,fcntl.LOCK_EX|fcntl.LOCK_NB)\n"
            "temporary=" + repr(str(started)+".tmp") + "\n"
            "with open(temporary,'w') as stream: stream.write(str(os.getpid()))\n"
            "os.replace(temporary," + repr(str(started)) + ")\n"
            "deadline=time.monotonic()+12\n"
            "while not os.path.exists(" + repr(str(release)) + "):\n"
            " if time.monotonic()>=deadline:\n"
            "  with open(" + repr(str(expired)) + ",'w') as stream: stream.write('worker self-expired')\n"
            "  raise SystemExit(19)\n"
            " time.sleep(0.01)\n"
            "with open(" + repr(str(marker)) + ",'w') as stream: stream.write('escaped after return')\n"
            "handle.close()\n")
        script.write_text("import subprocess,sys\nchild=subprocess.Popen([sys.executable, '-c', "
                          + repr(worker) + "])\nraise SystemExit(child.wait())\n", encoding="utf-8")

        def lock_available():
            with lock_path.open("r+b") as stream:
                try:
                    if os.name == "nt":
                        import msvcrt
                        msvcrt.locking(stream.fileno(), msvcrt.LK_NBLCK, 1)
                        stream.seek(0)
                        msvcrt.locking(stream.fileno(), msvcrt.LK_UNLCK, 1)
                    else:
                        import fcntl
                        fcntl.flock(stream, fcntl.LOCK_EX | fcntl.LOCK_NB)
                        fcntl.flock(stream, fcntl.LOCK_UN)
                    return True
                except OSError:
                    return False

        def worker_process_state(pid):
            if os.name == "nt":
                # Windows os.kill(pid, 0) is not a read-only liveness probe.
                import ctypes
                from ctypes import wintypes
                kernel = ctypes.WinDLL("kernel32", use_last_error=True)
                kernel.OpenProcess.argtypes = (wintypes.DWORD, wintypes.BOOL, wintypes.DWORD)
                kernel.OpenProcess.restype = wintypes.HANDLE
                kernel.GetExitCodeProcess.argtypes = (wintypes.HANDLE, ctypes.POINTER(wintypes.DWORD))
                kernel.GetExitCodeProcess.restype = wintypes.BOOL
                kernel.CloseHandle.argtypes = (wintypes.HANDLE,)
                kernel.CloseHandle.restype = wintypes.BOOL
                handle = kernel.OpenProcess(0x1000, False, pid)
                if not handle:
                    error = ctypes.get_last_error()
                    if error in (87, 1168):
                        return "absent"
                    raise OSError(error, "cannot observe owned worker process")
                try:
                    code = wintypes.DWORD()
                    if not kernel.GetExitCodeProcess(handle, ctypes.byref(code)):
                        raise OSError(ctypes.get_last_error(), "cannot read owned worker exit code")
                    return "running" if code.value == 259 else "exited"
                finally:
                    kernel.CloseHandle(handle)
            # An exited POSIX grandchild can remain a zombie until reaped.
            # Distinguish it from an executing descendant without signalling it.
            observed = subprocess.run(["ps", "-p", str(pid), "-o", "stat="],
                                      capture_output=True, text=True, timeout=1, check=False)
            state = observed.stdout.strip()
            if observed.returncode == 1 and not state:
                return "absent"
            if observed.returncode != 0 or not state:
                raise AssertionError("cannot observe owned worker process state")
            return "zombie" if state.startswith("Z") else "running"

        self.assertTrue(lock_available(), "worker lock must be free before the actual descendant starts")
        actual_popen = subprocess.Popen
        owned = []
        readiness = {"observed": False, "locked": False, "worker_pid": None}
        command = [sys.executable, str(script)]

        def started_process(argv, *args, **kwargs):
            process = actual_popen(argv, *args, **kwargs)
            if argv == command:
                owned.append(process)
                deadline = time.monotonic() + 2.0
                while not started.exists() and process.poll() is None and time.monotonic() < deadline:
                    time.sleep(0.01)
                if started.exists():
                    readiness.update(observed=True, locked=not lock_available(),
                                     worker_pid=int(started.read_text()))
                # Fail while the owned parent still waits for the child. The
                # finally block cleans the tree even without a received PID.
                self.assertTrue(readiness["observed"], "actual worker readiness was not established within bounded setup")
                self.assertTrue(readiness["locked"], "readiness must come from the live worker holding its OS lock")
                self.assertIs(type(readiness["worker_pid"]), int)
                self.assertGreater(readiness["worker_pid"], 0)
                self.assertNotIn(readiness["worker_pid"], (os.getpid(), process.pid))
            return process

        try:
            with patch.object(runner.subprocess, "Popen", side_effect=started_process):
                with self.assertRaises(subprocess.TimeoutExpired) as failure:
                    runner.run_process(command, {}, 0.6)
            self.assertGreater(failure.exception.timeout, 0)
            self.assertLessEqual(failure.exception.timeout, 0.6)
            # Capture return-time state before releasing the worker. Assertions
            # follow the observation window so a real survivor can demonstrate
            # the forbidden effect in the parent-only cancellation contrary.
            returned = {"monotonic_ns": time.monotonic_ns(), "parent_returncode": owned[0].poll(),
                        "worker_pid": readiness["worker_pid"], "lock_released": lock_available(),
                        "worker_state": worker_process_state(readiness["worker_pid"]),
                        "expired": expired.exists(), "effect_before_gate": marker.exists()}
            release.write_text("release only after run_process returned", encoding="utf-8")
            time.sleep(0.9)
            observed = {"readiness": dict(readiness), "at_return": returned,
                        "effect_after_gate": marker.exists(), "expired_after_gate": expired.exists(),
                        "lock_released_after_wait": lock_available()}
            self._timeout_worker_observation = observed
            self.assertIsNotNone(returned["parent_returncode"], f"timed-out parent remains active: {observed}")
            self.assertTrue(returned["lock_released"], f"timed-out descendant retained its live-process lock at return: {observed}")
            self.assertIn(returned["worker_state"], ("absent", "exited", "zombie"), f"timed-out descendant still executes: {observed}")
            self.assertFalse(returned["expired"] or observed["expired_after_gate"], f"worker self-expiry cannot prove cancellation: {observed}")
            self.assertFalse(returned["effect_before_gate"] or observed["effect_after_gate"], f"timed-out worker produced a post-return effect: {observed}")
            self.assertTrue(observed["lock_released_after_wait"], f"worker lock was reacquired after return: {observed}")
        finally:
            # Also clean the actual owned tree if bounded readiness/setup fails.
            # This cleanup cannot turn a missing precondition into a passing test.
            for process in owned:
                if process.poll() is None:
                    if os.name == "nt":
                        subprocess.run(["taskkill", "/PID", str(process.pid), "/T", "/F"],
                                       stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, timeout=5, check=False)
                        process.kill()
                    else:
                        os.killpg(process.pid, signal.SIGKILL)
                    process.wait(timeout=5)
            if readiness["worker_pid"] is not None and not lock_available():
                os.kill(readiness["worker_pid"], signal.SIGTERM)
                deadline = time.monotonic() + 1.0
                while not lock_available() and time.monotonic() < deadline:
                    time.sleep(0.01)
                self.assertTrue(lock_available(), "owned worker cleanup did not release its process lock")

    def test_malformed_suite_replaces_stale_success_with_failure(self):
        for malformed in [[], {"schema": runner.SCHEMA, "tasks": [[]]}]:
            suite = self.root / "malformed.json"
            suite.write_text(json.dumps(malformed), encoding="utf-8")
            self.output.write_text('{"status":"SYNTHETIC_STALE_SUCCESS","ai_quality_measured":false}', encoding="utf-8")
            result, report = self.cli("--suite", str(suite))
            self.assertEqual(result.returncode, 1)
            self.assertEqual(report["status"], "HARNESS_FAILED")
            self.assertFalse(report["ai_quality_measured"])

    def test_nonfinite_timeout_replaces_stale_receipt_without_starting(self):
        for timeout in ["inf", "nan", "-inf", "3601"]:
            self.output.write_text('{"status":"SYNTHETIC_STALE_SUCCESS","ai_quality_measured":false}', encoding="utf-8")
            result, report = self.cli("--timeout=" + timeout)
            self.assertEqual(result.returncode, 1)
            self.assertEqual(report["status"], "HARNESS_FAILED")

    def test_request_identity_binds_all_model_inputs_and_excludes_oracle(self):
        tasks, suite_sha = runner.load_suite(ROOT / "tasks.json")
        budget = {"max_input_tokens": 30, "max_output_tokens": 10}
        base = runner.model_request(tasks[0], "v2", "guide-a", "pinned", budget, 0, suite_sha)
        self.assertNotIn("oracle", base)
        variants = [runner.model_request(tasks[0], "v2", "guide-b", "pinned", budget, 0, suite_sha),
                    runner.model_request(tasks[0], "v2", "guide-a", "pinned", {**budget, "max_output_tokens": 11}, 0, suite_sha),
                    runner.model_request(tasks[0], "v2", "guide-a", "different", budget, 0, suite_sha),
                    runner.model_request(tasks[0], "legacy", "guide-a", "pinned", budget, 0, suite_sha)]
        self.assertTrue(all(request["request_id"] != base["request_id"] for request in variants))

    def test_output_cannot_overwrite_a_task_suite(self):
        suite = self.root / "suite.json"
        before = (ROOT / "tasks.json").read_bytes()
        suite.write_bytes(before)
        result = subprocess.run([sys.executable, str(ROOT / "runner.py"), "--suite", str(suite), "--output", str(suite)],
                                capture_output=True, text=True, timeout=5)
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(suite.read_bytes(), before)

    def test_external_configuration_requires_explicit_execution_flag(self):
        adapter = self.root / "must-not-run.py"
        marker = self.root / "unexpected-call"
        adapter.write_text("from pathlib import Path\nPath(" + repr(str(marker)) + ").write_text('bad')\n", encoding="utf-8")
        result, report = self.cli("--adapter", str(adapter), "--validator", str(adapter), "--model", "pinned")
        self.assertEqual(result.returncode, 1)
        self.assertFalse(marker.exists())
        self.assertFalse(report["ai_quality_measured"])


if __name__ == "__main__":
    unittest.main()
