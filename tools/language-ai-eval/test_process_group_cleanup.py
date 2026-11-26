import importlib.util
from pathlib import Path
import subprocess
import types
import unittest
from unittest.mock import Mock, patch


spec = importlib.util.spec_from_file_location("cleanup_control_runner", Path(__file__).with_name("runner.py"))
runner = importlib.util.module_from_spec(spec)
spec.loader.exec_module(runner)


class ProcessGroupCleanupTests(unittest.TestCase):
    def observed(self, text, code=0):
        return subprocess.CompletedProcess(["synthetic-state-observer"], code, text)

    def test_waits_for_observed_group_exit(self):
        # The table fixtures represent an existing synthetic owned group.
        # Mock its kernel existence probe as well as its process-table state.
        # Other groups cannot block or satisfy the owned group's cancellation.
        replies = [self.observed("8 Ss\n42 R\n42 Z\n"), self.observed("8 Ss\n42 Z+\n")]
        with patch.object(runner.sys, "platform", "linux"), \
             patch.object(runner.os, "killpg", return_value=None), \
             patch.object(runner.subprocess, "run", side_effect=replies) as observe:
            runner.wait_for_posix_group_exit(42, runner.time.monotonic() + 5)
        self.assertEqual(observe.call_count, 2)
        for call in observe.call_args_list:
            self.assertEqual(call.args[0], ["ps", "-A", "-o", "pgid=,stat="])
            self.assertGreater(call.kwargs["timeout"], 0)
            self.assertLessEqual(call.kwargs["timeout"], 1)
            self.assertEqual(set(call.kwargs["env"]), {"PATH", "LC_ALL"})

    def test_darwin_owned_group_snapshot_avoids_unrelated_enumeration(self):
        # Actual Darwin observations retained for this regression showed both
        # whole-host forms timing out at 1s, while the owned-PGID form returned
        # valid rows within that same bound. Reproduce that interface condition
        # deterministically; it does not turn an unavailable snapshot into PASS.
        for group, remaining in ((42, 1.0), (107, 0.25)):
            with self.subTest(group=group, remaining=remaining):
                def observe(command, **options):
                    self.assertEqual(options, {
                        "stdout": subprocess.PIPE, "stderr": subprocess.DEVNULL,
                        "text": True, "encoding": "ascii", "errors": "strict",
                        "env": {"PATH": runner.os.defpath, "LC_ALL": "C"},
                        "timeout": remaining, "check": False})
                    if "-A" in command:
                        raise subprocess.TimeoutExpired(command, remaining)
                    self.assertEqual(command, ["ps", "-g", str(group), "-o", "pgid=", "-o", "stat="])
                    return self.observed(str(group) + " Z\n")
                with patch.object(runner.sys, "platform", "darwin"), \
                     patch.object(runner.os, "killpg", return_value=None), \
                     patch.object(runner.time, "monotonic", return_value=10.0), \
                     patch.object(runner.subprocess, "run", side_effect=observe) as snapshot:
                    runner.wait_for_posix_group_exit(group, 10.0 + remaining)
                snapshot.assert_called_once()

    def test_live_threads_under_zombie_leader_exhaust_the_bound(self):
        with patch.object(runner.os, "killpg", return_value=None), \
             patch.object(runner.time, "monotonic", side_effect=[0.0, 0.01, 5.0]), \
             patch.object(runner.subprocess, "run", return_value=self.observed("42 Zsl\n")) as observe:
            with self.assertRaisesRegex(ValueError, "cleanup deadline expired"):
                runner.wait_for_posix_group_exit(42, 5.0)
        self.assertEqual(observe.call_count, 1)

    def test_uncertain_state_cannot_certify_cleanup(self):
        for reply in (self.observed("", 0), self.observed("42 Z\n", 1),
                      self.observed("42 Z unexpected\n"), self.observed("42 Zgarbage\n"),
                      self.observed("PGID STAT\n")):
            with self.subTest(observation=reply.stdout, exit_code=reply.returncode):
                with patch.object(runner.os, "killpg", return_value=None), \
                     patch.object(runner.subprocess, "run", return_value=reply):
                    with self.assertRaises(ValueError):
                        runner.wait_for_posix_group_exit(42, runner.time.monotonic() + 5)
        with patch.object(runner.os, "killpg", return_value=None), \
             patch.object(runner.subprocess, "run", side_effect=subprocess.TimeoutExpired(["ps"], 1)):
            with self.assertRaisesRegex(ValueError, "could not be observed"):
                runner.wait_for_posix_group_exit(42, runner.time.monotonic() + 5)

    def exercise_timeout(self, cleanup_error=None):
        original = subprocess.TimeoutExpired(["synthetic-adapter"], 0.6)
        process = Mock(pid=42, returncode=-9)
        order = []
        def communicate(*args, **kwargs):
            order.append("communicate")
            if len(order) == 1:
                raise original
            self.assertGreater(kwargs["timeout"], 0)
            self.assertLessEqual(kwargs["timeout"], 5)
        process.communicate.side_effect = communicate
        def kill(group, signal):
            self.assertEqual(group, 42)
            order.append("kill")
        def quiescence(group, deadline):
            self.assertEqual(group, 42)
            self.assertEqual(order, ["communicate", "kill", "communicate"])
            order.append("quiescence")
            if cleanup_error:
                raise cleanup_error
        owned_os = types.SimpleNamespace(name="posix", killpg=kill)
        with patch.object(runner, "os", owned_os), \
             patch.object(runner.subprocess, "Popen", return_value=process), \
             patch.object(runner, "wait_for_posix_group_exit", side_effect=quiescence):
            with self.assertRaises(ValueError if cleanup_error else subprocess.TimeoutExpired) as raised:
                runner.run_process(["synthetic-adapter"], {}, 0.6)
        self.assertEqual(order[-1], "quiescence")
        return original, raised.exception

    def test_original_timeout_is_preserved_after_quiescence(self):
        original, raised = self.exercise_timeout()
        self.assertIs(raised, original)
        self.assertEqual(raised.timeout, 0.6)

    def test_uncertain_cleanup_is_not_a_quiescent_timeout(self):
        original, raised = self.exercise_timeout(ValueError("synthetic observer failure"))
        self.assertNotIsInstance(raised, subprocess.TimeoutExpired)
        self.assertIn("could not establish termination", str(raised))


if __name__ == "__main__":
    unittest.main()
