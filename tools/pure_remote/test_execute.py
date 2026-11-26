"""Actual owned POSIX process controls; no compiler/native-runtime credit."""
import json
import os
from pathlib import Path
import signal
import subprocess
import sys
import tempfile
import time
import unittest

import service

CHILD = r"""
import json, os, pathlib, signal, subprocess, sys, time
mode, directory = sys.argv[1:]
root = pathlib.Path(directory)
def pin(name):
    raw = pathlib.Path('/proc/%d/stat' % os.getpid()).read_text()
    fields = raw[raw.rfind(')') + 2:].split()
    pending = root / (name + '.pending')
    pending.write_text(json.dumps({
        'pid': os.getpid(), 'pgid': os.getpgrp(), 'sid': os.getsid(0),
        'start': fields[19]}))
    pending.replace(root / (name + '.json'))
if mode == 'grandchild':
    signal.signal(signal.SIGTERM, signal.SIG_IGN)
    pin('grandchild')
    time.sleep(30)
elif mode in ('descendant_timeout', 'descendant_success'):
    subprocess.Popen([sys.executable, __file__, 'grandchild', directory])
    pin('leader')
    deadline = time.monotonic() + 8
    while not (root / 'grandchild.json').exists():
        if time.monotonic() > deadline:
            raise RuntimeError('grandchild did not become ready')
        time.sleep(.01)
    if mode == 'descendant_success':
        sys.exit(0)
    time.sleep(30)
elif mode == 'direct_stubborn':
    signal.signal(signal.SIGTERM, signal.SIG_IGN)
    pin('leader')
    time.sleep(30)
elif mode == 'direct':
    pin('leader')
    time.sleep(30)
elif mode == 'success':
    pin('leader')
    print('actual child stdout', flush=True)
    print('actual child stderr', file=sys.stderr, flush=True)
elif mode == 'nonzero':
    pin('leader')
    sys.exit(7)
else:
    raise RuntimeError('unknown child mode')
"""

DRIVER = r"""
import json, pathlib, sys
sys.path.insert(0, sys.argv[1])
import service
root = pathlib.Path(sys.argv[2])
try:
    result = service.execute(
        [sys.executable, str(root / 'child.py'), sys.argv[3], str(root)],
        root, root / 'streams', timeout=float(sys.argv[4]))
    (root / 'driver-result.original.json').write_text(json.dumps(result))
except BaseException as error:
    (root / 'driver-error.original.json').write_text(json.dumps({
        'type': type(error).__name__, 'message': str(error)}))
    raise
"""


def identity(pid):
    try:
        raw = Path('/proc/%d/stat' % pid).read_text()
    except FileNotFoundError:
        return None
    fields = raw[raw.rfind(')') + 2:].split()
    return {'pid': pid, 'state': fields[0], 'pgid': int(fields[2]),
            'sid': int(fields[3]), 'start': fields[19]}


class PureExecuteLifecycleTests(unittest.TestCase):
    def setUp(self):
        self.temporary = None
        evidence = os.environ.get('PURE_EXECUTE_CONTROL_OUT')
        if evidence:
            self.root = Path(evidence) / self._testMethodName
            self.root.mkdir()
        else:
            self.temporary = tempfile.TemporaryDirectory()
            self.root = Path(self.temporary.name)
        (self.root / 'streams').mkdir()
        (self.root / 'child.py').write_text(CHILD)
        self.wrapper = None
        self.foreign = None
        self.observations = {}

    def ready(self, name, limit=8):
        path = self.root / (name + '.json')
        deadline = time.monotonic() + limit
        while not path.exists():
            if self.wrapper is not None and self.wrapper.poll() is not None:
                self.fail('setup: wrapper exited before ' + name + ' became ready')
            if time.monotonic() >= deadline:
                self.fail('setup: bounded child readiness timed out')
            time.sleep(.01)
        value = json.loads(path.read_text())
        current = identity(value['pid'])
        self.assertIsNotNone(current, 'setup: child identity disappeared')
        self.assertEqual(current['start'], value['start'])
        return value

    def start(self, mode, timeout=1):
        stdout = (self.root / 'wrapper-stdout.original.bin').open('xb')
        stderr = (self.root / 'wrapper-stderr.original.bin').open('xb')
        self.addCleanup(stdout.close)
        self.addCleanup(stderr.close)
        self.wrapper = subprocess.Popen(
            [sys.executable, '-B', '-c', DRIVER, str(Path(service.__file__).parent),
             str(self.root), mode, str(timeout)],
            stdin=subprocess.DEVNULL, stdout=stdout, stderr=stderr,
            start_new_session=True)
        self.wrapper_pin = identity(self.wrapper.pid)
        return self.wrapper

    def wait_wrapper(self):
        code = self.wrapper.wait(timeout=14)
        self.observations['wrapperExitCode'] = code
        p = self.root / 'driver-result.original.json'
        return json.loads(p.read_text()) if p.exists() else None

    def alive(self, name):
        p = self.root / (name + '.json')
        self.assertTrue(p.exists(), 'setup: missing child identity ' + name)
        original = json.loads(p.read_text())
        actual = identity(original['pid'])
        alive = actual is not None and actual['start'] == original['start'] and actual['state'] != 'Z'
        self.observations.setdefault('beforeControlCleanup', {})[name] = {
            'original': original, 'observed': actual, 'alive': alive}
        return alive

    def assert_child_gone(self, name):
        self.assertFalse(self.alive(name), 'owned ' + name + ' remained alive after execute returned/cancelled')

    def signal_wrapper(self, sig):
        current = identity(self.wrapper.pid)
        self.assertIsNotNone(current, 'setup: wrapper already exited')
        self.assertEqual(current['start'], self.wrapper_pin['start'])
        os.kill(self.wrapper.pid, sig)

    def tearDown(self):
        # These controls clean only newly-created, identity-bound sessions, even on RED.
        cleanup = []
        for name in ('leader', 'grandchild'):
            p = self.root / (name + '.json')
            if not p.exists():
                continue
            pin = json.loads(p.read_text())
            current = identity(pin['pid'])
            if current is not None and current['start'] == pin['start'] and current['state'] != 'Z':
                self.assertEqual(current['pgid'], pin['pgid'])
                self.assertEqual(current['sid'], pin['sid'])
                self.assertEqual(pin['pgid'], pin['sid'])
                os.killpg(pin['pgid'], signal.SIGKILL)
                cleanup.append({'name': name, 'pgid': pin['pgid']})
        if self.wrapper is not None:
            if self.wrapper.poll() is None:
                current = identity(self.wrapper.pid)
                if current is not None and current['start'] == self.wrapper_pin['start']:
                    os.killpg(self.wrapper.pid, signal.SIGKILL)
            self.wrapper.wait(timeout=5)
        if self.foreign is not None:
            current = identity(self.foreign.pid)
            if current is not None and current['start'] == self.foreign_pin['start']:
                os.killpg(self.foreign.pid, signal.SIGKILL)
            self.foreign.wait(timeout=5)
        self.observations['testSafetyCleanup'] = cleanup
        (self.root / 'control-observations.original.json').write_text(json.dumps(self.observations, indent=2))
        if self.temporary is not None:
            self.temporary.cleanup()

    def test_success_retains_actual_exit_and_streams(self):
        self.start('success')
        result = self.wait_wrapper()
        self.assertEqual(result['exitCode'], 0)
        self.assertFalse(result['timedOut'])
        self.assert_child_gone('leader')
        self.assertEqual((self.root / 'streams/stdout.original.bin').read_bytes(), b'actual child stdout\n')
        self.assertEqual((self.root / 'streams/stderr.original.bin').read_bytes(), b'actual child stderr\n')

    def test_nonzero_exit_is_not_success(self):
        self.start('nonzero')
        result = self.wait_wrapper()
        self.assertEqual(result['exitCode'], 7)
        self.assertFalse(result['timedOut'])
        self.assert_child_gone('leader')

    def test_timeout_direct_preserves_unrelated_owned_control(self):
        self.foreign = subprocess.Popen(
            [sys.executable, '-c', 'import time; time.sleep(30)'],
            stdin=subprocess.DEVNULL, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL,
            start_new_session=True)
        self.foreign_pin = identity(self.foreign.pid)
        self.start('direct')
        self.ready('leader')
        result = self.wait_wrapper()
        self.assertTrue(result['timedOut'])
        self.assert_child_gone('leader')
        observed = identity(self.foreign.pid)
        self.assertEqual(observed['start'], self.foreign_pin['start'])
        self.assertNotEqual(observed['state'], 'Z')
        self.observations['unrelatedGroupPreserved'] = observed

    def test_timeout_kills_term_ignoring_direct_child(self):
        self.start('direct_stubborn')
        self.ready('leader')
        result = self.wait_wrapper()
        self.assertTrue(result['timedOut'])
        self.assert_child_gone('leader')

    def test_timeout_kills_descendant_after_parent_exits(self):
        self.start('descendant_timeout')
        self.ready('leader')
        self.ready('grandchild')
        result = self.wait_wrapper()
        self.assertTrue(result['timedOut'])
        self.assert_child_gone('leader')
        self.assert_child_gone('grandchild')

    def test_success_kills_lingering_descendant(self):
        self.start('descendant_success')
        result = self.wait_wrapper()
        self.assertEqual(result['exitCode'], 0)
        self.assertFalse(result['timedOut'])
        self.assert_child_gone('leader')
        self.assert_child_gone('grandchild')

    def test_sigterm_cleans_owned_group(self):
        self.start('direct', timeout=30)
        self.ready('leader')
        self.signal_wrapper(signal.SIGTERM)
        result = self.wait_wrapper()
        self.assert_child_gone('leader')
        self.assertIsNotNone(result, 'cancelled invocation must retain its process receipt')
        self.assertTrue(result['cancelled'])
        self.assertEqual(result['cancelSignal'], signal.SIGTERM)

    def test_sigint_cleans_owned_group(self):
        self.start('direct', timeout=30)
        self.ready('leader')
        self.signal_wrapper(signal.SIGINT)
        result = self.wait_wrapper()
        self.assert_child_gone('leader')
        self.assertIsNotNone(result, 'cancelled invocation must retain its process receipt')
        self.assertTrue(result['cancelled'])
        self.assertEqual(result['cancelSignal'], signal.SIGINT)


if __name__ == '__main__':
    unittest.main()
