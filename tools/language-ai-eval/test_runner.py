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

    def test_timeout_stops_a_spawned_worker(self):
        script = self.root / "parent.py"
        marker = self.root / "worker-finished"
        started = self.root / "worker-started"
        release = self.root / "worker-release"
        lock_path = self.root / "worker.lock"
        lock_path.write_bytes(b"x")
        # Only this actual descendant writes readiness, after taking an OS lock.
        # Its completion clock starts at release, after bounded setup and before
        # the unchanged 0.6s communicate/cancellation allowance starts.
        worker = ("import os,time\n"
            "handle=open(" + repr(str(lock_path)) + ",'r+b')\n"
            "if os.name=='nt':\n import msvcrt\n msvcrt.locking(handle.fileno(),msvcrt.LK_NBLCK,1)\n"
            "else:\n import fcntl\n fcntl.flock(handle,fcntl.LOCK_EX|fcntl.LOCK_NB)\n"
            "temporary=" + repr(str(started)+".tmp") + "\n"
            "with open(temporary,'w') as stream: stream.write(str(os.getpid()))\n"
            "os.replace(temporary," + repr(str(started)) + ")\n"
            "deadline=time.monotonic()+4\n"
            "while not os.path.exists(" + repr(str(release)) + "):\n"
            " if time.monotonic()>=deadline: raise SystemExit(19)\n time.sleep(0.01)\n"
            "time.sleep(1.2)\n"
            "with open(" + repr(str(marker)) + ",'w') as stream: stream.write('escaped')\n"
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
                # Fail while the actual owned parent still waits for its child.
                # The finally block cleans that tree even when no child PID was
                # received; an absent precondition never enters the timeout test.
                self.assertTrue(readiness["observed"], "actual worker readiness was not established within bounded setup")
                self.assertTrue(readiness["locked"], "readiness must come from the live worker holding its OS lock")
                self.assertIs(type(readiness["worker_pid"]), int)
                self.assertGreater(readiness["worker_pid"], 0)
                self.assertNotIn(readiness["worker_pid"], (os.getpid(), process.pid))
                release.write_text("release")
            return process
        try:
            with patch.object(runner.subprocess, "Popen", side_effect=started_process):
                with self.assertRaises(subprocess.TimeoutExpired) as failure:
                    runner.run_process(command, {}, 0.6)
            # communicate may report its remaining sub-wait allowance.
            self.assertGreater(failure.exception.timeout, 0)
            self.assertLessEqual(failure.exception.timeout, 0.6)
            self.assertTrue(readiness["observed"], "actual worker readiness was not established within bounded setup")
            self.assertTrue(readiness["locked"], "readiness must come from the live worker holding its OS lock")
            self.assertNotIn(readiness["worker_pid"], (os.getpid(), owned[0].pid))
            self.assertIsNotNone(owned[0].poll(), "timed-out parent remains active")
            self.assertTrue(lock_available(), "timed-out descendant still holds its live-process lock")
            time.sleep(0.9)
            self.assertFalse(marker.exists(), "timed-out worker survived cancellation")
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
