"""Exercise the real shell boundary with only bounded synthetic POSIX children."""
import json
import os
from pathlib import Path
import re
import signal
import subprocess
import sys
import tempfile
import time
import unittest

if os.name == "posix":
    import fcntl


REPO = Path(os.environ.get("CHERRI_QA_TEST_REPO", Path(__file__).resolve().parents[2]))
SHELL = Path(os.environ.get("CHERRI_QA_COMMAND_SHELL", REPO / "scripts/ios27_shortcuts_simulator_qa.sh"))
WORKER = r'''
import fcntl,json,os,pathlib,sys,time
p=pathlib.Path(sys.argv[1]); mode=sys.argv[2]
parent=os.getpid(); child=os.fork()
if child == 0:
    lock=(p/"owned.lock").open("wb"); fcntl.flock(lock,fcntl.LOCK_EX)
    (p/"ready.json").write_text(json.dumps({"parent":parent,"worker":os.getpid(),"group":os.getpgrp()}))
    os.close(1);os.close(2)
    end=time.monotonic()+12
    while time.monotonic()<end: time.sleep(.02)
    os._exit(0)
end=time.monotonic()+3
while not (p/"ready.json").is_file() and time.monotonic()<end: time.sleep(.01)
if mode == "exit": os._exit(0)
os.waitpid(child,0)
'''


@unittest.skipUnless(os.name == "posix", "requires owned POSIX groups")
class OwnedShellCommandTests(unittest.TestCase):
    def setUp(self):
        retained = os.environ.get("CHERRI_OWNED_COMMAND_TEST_ORIGINALS")
        if retained:
            self.root = Path(retained) / self._testMethodName
            self.root.mkdir(parents=True, mode=0o700, exist_ok=False)
        else:
            temporary = tempfile.TemporaryDirectory(prefix="cherri-owned-command-")
            self.addCleanup(temporary.cleanup)
            self.root = Path(temporary.name)
        (self.root / "builds").mkdir(mode=0o700)
        source = SHELL.read_text()
        matches = re.findall(r"    run_cmd_timeout\(\) \{\n.*?\nPY\n    \}", source, re.S)
        self.assertEqual(len(matches), 1)
        self.driver = self.root / "driver.original.sh"
        self.driver.write_text(matches[0] + '\nrun_cmd_timeout "$@"\n')
        self.processes, self.groups = [], []
        self.addCleanup(self.cleanup_owned)

    def cleanup_owned(self):
        for group in self.groups + [process.pid for process in self.processes]:
            self.assertNotEqual(group, os.getpgrp())
            try:
                os.killpg(group, signal.SIGKILL)
            except ProcessLookupError:
                pass
        for process in self.processes:
            out, err = process.communicate(timeout=5)
            (self.root / (str(process.pid) + ".stdout.original")).write_bytes(out)
            (self.root / (str(process.pid) + ".stderr.original")).write_bytes(err)

    def start(self, timeout, code, *args):
        env = dict(os.environ, ROOT_DIR=str(REPO), ARTIFACT_DIR=str(self.root))
        process = subprocess.Popen(["bash", str(self.driver), str(timeout), sys.executable,
            "-I", "-B", "-c", code, *args], stdout=subprocess.PIPE, stderr=subprocess.PIPE,
            start_new_session=True, env=env)
        self.processes.append(process)
        return process

    def ready(self):
        deadline = time.monotonic() + 5
        while time.monotonic() < deadline:
            path = self.root / "ready.json"
            if path.is_file():
                try:
                    record = json.loads(path.read_text())
                except json.JSONDecodeError:
                    time.sleep(.01)
                    continue
                self.assertEqual(record["group"], record["parent"])
                self.assertNotEqual(record["group"], os.getpgrp())
                self.groups.append(record["group"])
                return record
            time.sleep(.01)
        self.fail("owned synthetic child did not reach readiness")

    def lock_released(self):
        with (self.root / "owned.lock").open("rb") as stream:
            try:
                fcntl.flock(stream, fcntl.LOCK_EX | fcntl.LOCK_NB)
                fcntl.flock(stream, fcntl.LOCK_UN)
                return True
            except BlockingIOError:
                return False

    def await_release(self):
        deadline = time.monotonic() + 3
        while time.monotonic() < deadline:
            if self.lock_released():
                return True
            time.sleep(.01)
        return False

    def result(self):
        deadline = time.monotonic() + 3
        while time.monotonic() < deadline:
            paths = list((self.root / "builds").glob("owned-command-*/result.original.json"))
            self.assertLessEqual(len(paths), 1)
            if paths:
                try:
                    return json.loads(paths[0].read_text())
                except json.JSONDecodeError:
                    pass
            time.sleep(.01)
        self.fail("owned command did not retain its terminal process receipt")

    def test_cancellation_releases_actual_owned_descendant(self):
        process = self.start(10, WORKER, str(self.root), "wait")
        ready = self.ready()
        self.assertFalse(self.lock_released())
        os.killpg(process.pid, signal.SIGTERM)
        process.wait(timeout=5)
        released = self.await_release()
        (self.root / "cancellation-observed.original.json").write_text(json.dumps({
            "ready": ready, "wrapperExit": process.returncode, "lockReleased": released}))
        self.assertTrue(released, "cancelled wrapper left its actual owned worker alive")
        result = self.result()
        self.assertEqual(result["interruptSignal"], signal.SIGTERM)
        self.assertEqual(result["failure"], "INTERRUPTED")
        self.assertTrue(result["process"]["cleanupParentReaped"])
        self.assertEqual(result["process"]["pid"], ready["parent"])
        self.assertNotEqual(result["exitCode"], 0)

    def test_zero_parent_cannot_authenticate_live_descendant(self):
        process = self.start(5, WORKER, str(self.root), "exit")
        ready = self.ready()
        process.wait(timeout=7)
        (self.root / "zero-parent-observed.original.json").write_text(json.dumps({
            "ready": ready, "wrapperExit": process.returncode, "lockReleased": self.lock_released()}))
        self.assertNotEqual(process.returncode, 0, "zero parent hid its surviving owned worker")
        self.assertTrue(self.await_release())
        result = self.result()
        self.assertEqual(result["observation"]["result"], "OWNED_GROUP_REMAINED")
        self.assertEqual(result["process"]["pid"], ready["parent"])
        self.assertTrue(result["process"]["cleanupParentReaped"])

    def test_timeout_retains_original_bound_and_kills_owned_group(self):
        process = self.start(1, "import time;time.sleep(12)")
        out, err = process.communicate(timeout=7)
        self.assertEqual(process.returncode, 124)
        self.assertEqual(out, b"")
        self.assertIn(b"command timed out after 1s:", err)
        result = self.result()
        self.assertEqual(result["timeoutSeconds"], 1)
        self.assertEqual(result["cleanupEnvelopeSeconds"], 5)
        self.assertTrue(result["cleanupComplete"])

    def test_large_live_streams_and_literal_arguments_are_lossless(self):
        literal = "space ' quote $() עברית"
        code = "import os,sys;os.write(1,sys.argv[1].encode()+b'\\n'+b'x'*3145728);os.write(2,b'EXACT-ERR')"
        process = self.start(5, code, literal)
        out, err = process.communicate(timeout=8)
        self.assertEqual(process.returncode, 0)
        self.assertEqual(out, literal.encode() + b"\n" + b"x" * 3145728)
        self.assertEqual(err, b"EXACT-ERR")
        result = self.result()
        self.assertEqual(result["argv"][-1], literal)
        self.assertEqual(result["process"]["stdout"]["bytes"], 0)
        self.assertEqual(result["process"]["stderr"]["bytes"], 0)
        self.assertTrue(result["cleanupComplete"])

    def test_nonzero_exit_and_original_stdio_are_preserved(self):
        process = self.start(5, "import os;os.write(1,b'OUT');os.write(2,b'ERR');raise SystemExit(37)")
        out, err = process.communicate(timeout=8)
        self.assertEqual((process.returncode, out, err), (37, b"OUT", b"ERR"))
        result = self.result()
        self.assertEqual(result["observation"]["result"], "EXIT_NONZERO")
        self.assertEqual(result["observation"]["exitCode"], 37)
        self.assertTrue(result["cleanupComplete"])


if __name__ == "__main__":
    unittest.main()
