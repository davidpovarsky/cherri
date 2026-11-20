import copy
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import time
import unittest
from unittest.mock import patch

import simulator_session_recovery as recovery
import prepare_public_artifacts as publication

UDID = '11111111-2222-3333-4444-555555555555'
SOURCE = 'a' * 40
RUN = '123456'
FIXTURE = 'qa-session-control-v2'
IDENTITY = {'schemaVersion': '1.0', 'compilerCommitSha': SOURCE, 'runId': RUN,
            'fixtureId': FIXTURE, 'deviceIdSha256': hashlib.sha256(UDID.encode()).hexdigest()}


class SessionRecoveryTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.scratch = self.root / 'scratch'
        self.scratch.mkdir()
        self.commands = []
        self.clock = 10.0
        self.clock_patch = patch.object(recovery.time, 'monotonic', lambda: self.clock)
        self.sleep_patch = patch.object(recovery.time, 'sleep', self.sleep)
        self.clock_patch.start()
        self.sleep_patch.start()
        self.addCleanup(self.clock_patch.stop)
        self.addCleanup(self.sleep_patch.stop)

    def sleep(self, value):
        self.clock += value

    def runner(self, args, allowance):
        self.commands.append((args, allowance))
        self.clock += .001
        if 'screenshot' in args:
            Path(args[-1]).write_bytes(b'X' * recovery.MIN_DISPLAY_BYTES)
        return {'result': 'PASS', 'exitCode': 0, 'elapsedMs': 1}

    def session(self, runner=None, budget=240):
        return recovery.Session(UDID, self.scratch, budget, runner or self.runner)

    def receipt(self, attempt):
        return {**IDENTITY, 'attempts': [{'attempt': 1, **attempt}]}

    def test_quick_probe_has_no_reboot_import_or_display_claim(self):
        result = self.session().recover()
        recovery.validate_receipt(self.receipt(result), SOURCE, RUN, FIXTURE)
        self.assertEqual(result['status'], 'PROBE_READY')
        self.assertEqual(result['springboardBytes'], 0)
        self.assertEqual(self.commands[0][0], ['xcrun', 'simctl', 'getenv', UDID, 'HOME'])
        self.assertEqual(len(self.commands), 1)

    def test_recovery_uses_owned_device_and_fresh_display_and_removes_raster(self):
        result = self.session().recover(force=True)
        recovery.validate_receipt(self.receipt(result))
        self.assertEqual(result['status'], 'RECOVERED')
        self.assertEqual([x['stage'] for x in result['commands']],
                         ['shutdown', 'boot', 'bootstatus', 'open', 'getenv', 'getenv', 'screenshot'])
        self.assertTrue(all(UDID in args for args, _ in self.commands))
        self.assertTrue(all(0 < allowance <= 120 for _, allowance in self.commands))
        self.assertEqual(list(self.scratch.iterdir()), [])

    def test_initial_probe_timeout_can_recover_with_independent_fresh_probes(self):
        def runner(args, allowance):
            if not self.commands:
                self.commands.append((args, allowance))
                self.clock += allowance
                return {'result': 'TIMEOUT', 'exitCode': None, 'elapsedMs': round(allowance * 1000)}
            return self.runner(args, allowance)
        result = self.session(runner).recover()
        recovery.validate_receipt(self.receipt(result))
        self.assertEqual(result['status'], 'RECOVERED')
        self.assertEqual(result['commands'][0]['result'], 'TIMEOUT')

    def test_recovery_command_timeout_stops_remaining_commands(self):
        def runner(args, allowance):
            if 'bootstatus' in args:
                self.commands.append((args, allowance))
                self.clock += allowance
                return {'result': 'TIMEOUT', 'exitCode': None, 'elapsedMs': round(allowance * 1000)}
            return self.runner(args, allowance)
        result = self.session(runner).recover(force=True)
        recovery.validate_receipt(self.receipt(result))
        self.assertEqual(result['status'], 'BLOCKED')
        self.assertEqual(result['reason'], 'RECOVERY_COMMAND_TIMEOUT')
        self.assertEqual(result['commands'][-1]['stage'], 'bootstatus')
        self.assertFalse(any(x['stage'] in {'open', 'screenshot'} for x in result['commands']))

    def test_deadline_rejects_late_zero_exit_and_never_starts_next_command(self):
        def runner(args, allowance):
            self.commands.append((args, allowance))
            self.clock += allowance + 12
            return {'result': 'PASS', 'exitCode': 0, 'elapsedMs': round((allowance + 12) * 1000)}
        result = self.session(runner, budget=.02).recover(force=True)
        recovery.validate_receipt(self.receipt(result))
        self.assertEqual(result['status'], 'BLOCKED')
        self.assertEqual(result['commands'][0]['result'], 'DEADLINE_EXHAUSTED')
        self.assertEqual(len(self.commands), 1)
        self.assertGreater(result['elapsedMs'], result['budgetMs'])

    def test_expired_allowance_does_not_invoke_runner(self):
        session = self.session(budget=.02)
        self.clock += .03
        self.assertFalse(session.probe())
        self.assertEqual(self.commands, [])

    def test_display_rejects_undersized_and_absent_raster_and_removes_all(self):
        def runner(args, allowance):
            result = self.runner(args, allowance)
            if 'screenshot' in args:
                Path(args[-1]).write_bytes(b'X' * (recovery.MIN_DISPLAY_BYTES - 1))
            return result
        ready, size = self.session(runner).display(20)
        self.assertFalse(ready)
        self.assertEqual(size, recovery.MIN_DISPLAY_BYTES - 1)
        self.assertEqual(list(self.scratch.iterdir()), [])

    def test_failed_old_raster_cannot_satisfy_later_success_without_new_file(self):
        screenshots = []
        def runner(args, allowance):
            self.commands.append((args, allowance))
            self.clock += .001
            if 'screenshot' in args:
                screenshots.append(args[-1])
                if len(screenshots) == 1:
                    Path(args[-1]).write_bytes(b'X' * recovery.MIN_DISPLAY_BYTES)
                    return {'result': 'EXIT_NONZERO', 'exitCode': 1, 'elapsedMs': 1}
            return {'result': 'PASS', 'exitCode': 0, 'elapsedMs': 1}
        ready, size = self.session(runner).display(20)
        self.assertFalse(ready)
        self.assertGreater(len(screenshots), 1)
        self.assertEqual(size, 0)
        self.assertEqual(list(self.scratch.iterdir()), [])

    def test_display_probe_failure_may_recover_only_with_new_success_pair(self):
        probes = []
        def runner(args, allowance):
            if 'getenv' in args:
                probes.append(args)
                if len(probes) == 2:
                    self.commands.append((args, allowance))
                    return {'result': 'EXIT_NONZERO', 'exitCode': 1, 'elapsedMs': 1}
            return self.runner(args, allowance)
        result = self.session(runner).recover(force=True)
        self.assertEqual(result['status'], 'RECOVERED')
        recovery.validate_receipt(self.receipt(result))

    def test_receipt_rejects_specific_false_readiness_and_unknown_fields(self):
        valid = self.receipt(self.session().recover(force=True))
        mutations = {
            'wrong source': lambda x: x.update(compilerCommitSha='b' * 40),
            'wrong run': lambda x: x.update(runId='another'),
            'wrong fixture': lambda x: x.update(fixtureId='another'),
            'private field': lambda x: x.update(rawLog='not allowed'),
            'false screenshot': lambda x: x['attempts'][0]['commands'][-1].update(exitCode=1),
            'missing probe': lambda x: x['attempts'][0]['commands'].pop(-2),
            'wrong recovery order': lambda x: x['attempts'][0]['commands'][0].update(stage='open'),
            'undersized display': lambda x: x['attempts'][0].update(springboardBytes=1),
            'late readiness': lambda x: x['attempts'][0].update(elapsedMs=240001),
            'wrong reason': lambda x: x['attempts'][0].update(reason='INITIAL_GETENV_READY'),
            'false timeout code': lambda x: x['attempts'][0]['commands'][2].update(result='TIMEOUT', exitCode=0),
            'continued after timeout': lambda x: x['attempts'][0]['commands'][2].update(result='TIMEOUT', exitCode=None),
        }
        for name, mutate in mutations.items():
            with self.subTest(name=name):
                changed = copy.deepcopy(valid)
                mutate(changed)
                with self.assertRaises(ValueError):
                    recovery.validate_receipt(changed, SOURCE, RUN, FIXTURE)

    def test_receipt_rejects_understated_time_and_success_after_stage_allowance(self):
        quick = self.receipt(self.session().recover())
        quick['attempts'][0]['commands'][0]['elapsedMs'] = 300000
        with self.assertRaisesRegex(ValueError, 'COMMAND_PASS_AFTER_LIMIT'):
            recovery.validate_receipt(quick)
        observed = self.receipt(self.session().recover(force=True))
        observed['attempts'][0]['elapsedMs'] = 1
        for command in observed['attempts'][0]['commands']:
            command['elapsedMs'] = 1000
        with self.assertRaisesRegex(ValueError, 'ATTEMPT_COMMAND_TIME_MISMATCH'):
            recovery.validate_receipt(observed)

    def test_history_keeps_failed_attempt_then_success_and_rejects_rebinding(self):
        def timeout(args, allowance):
            return {'result': 'TIMEOUT', 'exitCode': None, 'elapsedMs': 1}
        path = self.root / 'receipt.json'
        recovery.record_attempt(path, {k:v for k,v in IDENTITY.items() if k != 'schemaVersion'}, self.session(timeout).recover(force=True))
        before = path.read_bytes()
        result = recovery.record_attempt(path, {k:v for k,v in IDENTITY.items() if k != 'schemaVersion'}, self.session().recover())
        self.assertEqual([x['status'] for x in result['attempts']], ['BLOCKED', 'PROBE_READY'])
        self.assertEqual(result['attempts'][0], json.loads(before)['attempts'][0])
        retained = path.read_bytes()
        for key, value in [('runId','wrong'), ('deviceIdSha256','f'*64), ('compilerCommitSha','b'*40)]:
            with self.subTest(key=key), self.assertRaises(ValueError):
                recovery.record_attempt(path, {**IDENTITY, key:value}, self.session().recover())
            self.assertEqual(path.read_bytes(), retained)
        self.assertEqual(sorted(x.name for x in self.root.iterdir()), ['receipt.json', 'scratch'])

    def test_duplicate_history_and_existing_lock_fail_without_rewriting(self):
        path = self.root / 'receipt.json'
        data = json.dumps(self.receipt(self.session().recover()))
        data = data.replace('"runId": "123456"', '"runId": "wrong", "runId": "123456"')
        path.write_text(data)
        with self.assertRaisesRegex(ValueError, 'DUPLICATE_RECEIPT_KEY'):
            recovery.record_attempt(path, IDENTITY, self.session().recover())
        self.assertEqual(path.read_text(), data)
        lock = self.root / 'receipt.json.lock'
        lock.write_text('owned elsewhere')
        with self.assertRaises(FileExistsError):
            recovery.record_attempt(path, IDENTITY, self.session().recover())
        self.assertEqual(lock.read_text(), 'owned elsewhere')

    def test_publication_retains_exact_bounded_receipt_and_rejects_rebinding(self):
        raw = self.root / RUN
        (raw / 'runtime').mkdir(parents=True)
        (raw / 'compiler-manifest.json').write_text(json.dumps({'syntheticOnly':True,
            'compilerCommitSha':SOURCE, 'fixtures':[{'fixtureId':FIXTURE,
            'fileName':FIXTURE+'.cherri', 'sourceSha256':'1'*64}]}))
        (raw / 'summary.json').write_text('{"privacyGatePassed":true}')
        (raw / 'runtime/output-assertions.json').write_text(json.dumps({'targetFixtureId':FIXTURE}))
        path = raw / 'runtime/simulator-session-recovery.json'
        value = self.receipt(self.session().recover(force=True))
        path.write_text(json.dumps(value))
        trusted = {FIXTURE+'.cherri':'1'*64}
        publication.stage(raw, self.root/'public', trusted)
        self.assertEqual(path.read_bytes(), (self.root/'public/runtime/simulator-session-recovery.json').read_bytes())
        for key, wrong in [('compilerCommitSha','b'*40),('runId','wrong'),('fixtureId','wrong')]:
            with self.subTest(key=key):
                path.write_text(json.dumps({**value,key:wrong}))
                with self.assertRaisesRegex(publication.PublicationBlocked, 'INVALID_BOUNDED_SESSION_DIAGNOSTIC'):
                    publication.stage(raw, self.root/('rejected-'+key), trusted)
                self.assertFalse((self.root/('rejected-'+key)).exists())

    def test_invalid_device_and_symlink_scratch_are_rejected(self):
        with self.assertRaisesRegex(ValueError, 'DEVICE_ID_INVALID'):
            recovery.Session('booted', self.scratch)
        if os.name == 'posix':
            link = self.root/'link'
            link.symlink_to(self.scratch, target_is_directory=True)
            with self.assertRaisesRegex(ValueError, 'SCRATCH_INVALID'):
                recovery.Session(UDID, link)


class RealProcessTimeoutTests(unittest.TestCase):
    def test_actual_process_exit_and_launch_failure(self):
        self.assertEqual(recovery.run_process([sys.executable,'-c','pass'], 2)['result'], 'PASS')
        result = recovery.run_process([sys.executable,'-c','raise SystemExit(7)'], 2)
        self.assertEqual((result['result'], result['exitCode']), ('EXIT_NONZERO',7))
        self.assertEqual(recovery.run_process([str(Path(tempfile.gettempdir())/'cherri-no-such-executable-session-control')], 1)['result'], 'LAUNCH_FAILED')

    def test_actual_child_timeout_is_recorded_and_bounded(self):
        started = time.monotonic()
        result = recovery.run_process([sys.executable,'-c','import time; time.sleep(30)'], .08)
        self.assertEqual(result['result'], 'TIMEOUT')
        self.assertIsNone(result['exitCode'])
        self.assertGreaterEqual(result['elapsedMs'], 70)
        self.assertLess(time.monotonic()-started, 3)

    @unittest.skipUnless(os.name == 'posix', 'POSIX process groups are the macOS runtime path')
    def test_actual_timeout_kills_descendant_before_delayed_write(self):
        with tempfile.TemporaryDirectory() as directory:
            marker = Path(directory)/'child-ran'
            child = 'import time; from pathlib import Path; time.sleep(.6); Path('+repr(str(marker))+').write_text("unexpected")'
            parent = 'import subprocess,sys,time; subprocess.Popen([sys.executable,"-c",'+repr(child)+']); time.sleep(30)'
            result = recovery.run_process([sys.executable,'-c',parent], .15)
            self.assertEqual(result['result'], 'TIMEOUT')
            time.sleep(.7)
            self.assertFalse(marker.exists())

    @unittest.skipUnless(os.name == 'posix' and shutil.which('bash'), 'actual bash wrapper control runs on POSIX')
    def test_actual_shell_wrapper_passes_identity_and_propagates_failure(self):
        repo = Path(__file__).resolve().parents[2]
        shell = (repo/'scripts/ios27_shortcuts_simulator_qa.sh').read_text()
        body = shell[shell.index('simctl_getenv_ok() {'):shell.index('simctl_openurl_resilient() {')]
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            bins = root/'bin'; bins.mkdir()
            scratch = root/'scratch'; scratch.mkdir()
            artifact = root/RUN; (artifact/'runtime').mkdir(parents=True)
            xcrun = bins/'xcrun'
            xcrun.write_text('#!'+sys.executable+'\nraise SystemExit(0)\n')
            xcrun.chmod(0o700)
            env = {**os.environ, 'PATH':str(bins)+os.pathsep+os.environ['PATH'],
                'ROOT_DIR':str(repo),'QA_SCRATCH_DIR':str(scratch),'ARTIFACT_DIR':str(artifact),
                'SOURCE_SHA':SOURCE,'RUN_ID':RUN,'TARGET_FIXTURE_ID':FIXTURE}
            command = body+'\nensure_sim_session_ready '+UDID+' 0\n'
            result = subprocess.run(['bash','-c',command],env=env,capture_output=True,text=True,timeout=5)
            self.assertEqual(result.returncode,0,result.stderr)
            receipt = recovery.validate_receipt(json.loads((artifact/'runtime/simulator-session-recovery.json').read_text()),SOURCE,RUN,FIXTURE)
            self.assertEqual(receipt['attempts'][0]['status'],'PROBE_READY')
            bad = subprocess.run(['bash','-c',body+'\nensure_sim_session_ready bad-device 0\n'],env=env,capture_output=True,text=True,timeout=5)
            self.assertEqual(bad.returncode,2)
            self.assertEqual(len(json.loads((artifact/'runtime/simulator-session-recovery.json').read_text())['attempts']),1)


if __name__ == '__main__':
    unittest.main()
