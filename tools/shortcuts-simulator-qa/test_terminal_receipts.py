import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys
import tempfile
import unittest

import prepare_public_artifacts as publication
import xcode_destination_readiness as destination

HERE = Path(__file__).resolve().parent
REPO = HERE.parents[1]
SCRIPT = REPO / "scripts/ios27_shortcuts_simulator_qa.sh"
UDID = "11111111-2222-3333-4444-555555555555"
SOURCE, RUN, FIXTURE = "a" * 40, "123456", "qa-terminal-control-v2"


def actual_summary_body():
    source = SCRIPT.read_text(encoding="utf-8")
    start = source.index("# Write summary.json and docs/report.md")
    block = source[start:]
    return block.split("<<'PY'\n", 1)[1].split("\nPY\n", 1)[0]


class OriginalRetentionTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.home = Path(self.temp.name)
        self.artifacts = self.home / RUN
        self.scratch = self.home / "scratch"
        self.repo = self.home / "repo"
        for path in (self.artifacts / "builds", self.artifacts / "runtime",
                     self.artifacts / "docs", self.artifacts / "tests",
                     self.scratch, self.repo / destination.PROJECT):
            path.mkdir(parents=True)
        self.clock = 10.0
        def runner(args, allowance, *, output=None):
            self.clock += 1
            if "-showdestinations" in args:
                output.write((f'Destinations compatible with the "{destination.SCHEME}" scheme:\n'
                    f'{{ platform:iOS Simulator, id:{UDID}, OS:27.1, name:OWNED_CONTROL }}\n').encode())
            elif output is not None:
                output.write(b"ACTUAL_PROCESS_CONTROL_OUTPUT\n")
            return {"result":"PASS", "exitCode":0, "elapsedMs":1000}
        _, self.receipt = destination.DestinationTestPhase(self.repo, self.scratch,
            self.artifacts, UDID, {"compilerCommitSha":SOURCE, "runId":RUN, "fixtureId":FIXTURE},
            runner=runner, now=lambda:self.clock).run()
        self.original = self.artifacts / self.receipt["queries"][0]["rawRelativePath"]
        self.original_bytes = self.original.read_bytes()
        self.write("compiler-manifest.json", {"syntheticOnly":True,"compilerCommitSha":SOURCE,
            "fixtures":[{"fixtureId":FIXTURE,"fileName":FIXTURE+".cherri","sourceSha256":"1"*64,
                         "unsignedPlistSha256":"2"*64,"buildProfile":"debug",
                         "runtimeTraceExpectation":"REQUIRED_NDJSON"}]})
        self.write("runtime/output-assertions.json", {"targetFixtureId":FIXTURE,
            "compilerCommitSha":SOURCE,"buildProfile":"debug","runtimeContractVerified":False})
        self.write("runtime/xcode-destination-readiness.json", self.receipt)
        self.trusted = {FIXTURE+".cherri":"1"*64}

    def write(self, name, value):
        path = self.artifacts / name
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(json.dumps(value), encoding="utf-8")

    def emit_actual_summary(self):
        gates = ["NOT_RUN"] * 13 + ["BLOCKED_DEVICE"]
        env = dict(os.environ, PYTHONPATH=str(HERE))
        result = subprocess.run([sys.executable, "-c", actual_summary_body(),
            str(self.artifacts), RUN, SOURCE, "true", *gates, UDID],
            cwd=REPO, env=env, capture_output=True, timeout=15)
        self.assertEqual(result.returncode, 0, result.stderr.decode(errors="replace"))
        return json.loads((self.artifacts / "summary.json").read_text(encoding="utf-8"))

    def test_actual_summary_and_publisher_keep_private_device_original(self):
        summary = self.emit_actual_summary()
        self.assertTrue(summary["privacyGatePassed"],
            "A private quarantined destination log must remain available to its strict validator.")
        self.assertFalse(summary["runtimeContractVerified"])
        output = self.home / "public"
        result = publication.stage(self.artifacts, output, self.trusted)
        self.assertTrue(result["publicationGatePassed"])
        self.assertEqual(self.original.read_bytes(), self.original_bytes)
        self.assertFalse((output / self.original.relative_to(self.artifacts)).exists())
        self.assertEqual((output / "runtime/xcode-destination-readiness.json").read_bytes(),
                         (self.artifacts / "runtime/xcode-destination-readiness.json").read_bytes())

    def test_actual_final_cleanup_preserves_strict_destination_original(self):
        bash = os.environ.get("CHERRI_TEST_BASH") or shutil.which("bash")
        if not bash:
            self.skipTest("Bash unavailable; native shell route remains unexecuted")
        source = SCRIPT.read_text(encoding="utf-8")
        lines = [line for line in source.splitlines()
                 if line.startswith("rm -rf ") and '"$ARTIFACT_DIR/tests/"' in line]
        self.assertEqual(len(lines), 1)
        env = dict(os.environ, ARTIFACT_DIR=self.artifacts.as_posix(),
                   QA_SCRATCH_DIR=self.scratch.as_posix(), RAW_SIM_LOG="", PHASE4_SIM_LOG="")
        done = subprocess.run([bash, "-c", lines[0]], env=env, capture_output=True, timeout=15)
        self.assertEqual(done.returncode, 0, done.stderr.decode(errors="replace"))
        destination.validate_receipt(self.receipt, SOURCE, RUN, FIXTURE,
                                     artifact_root=self.artifacts)
        self.assertEqual(self.original.read_bytes(), self.original_bytes)

    def test_actual_summary_still_blocks_public_device_and_secret_canaries(self):
        for name, text in (("device", UDID), ("secret", "CHERRI_PRIVATE_CANARY_PRIVATE_VALUE")):
            with self.subTest(name=name):
                self.write("environment.json", {"value":text})
                summary = self.emit_actual_summary()
                self.assertFalse(summary["privacyGatePassed"])




import copy
import signal
import time
from unittest.mock import patch

import qa_terminal_receipts as terminal
import simulator_session_recovery as recovery


class AcquisitionTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.home = Path(self.temp.name)
        self.root = self.home / RUN
        self.scratch = self.home / "scratch"
        for path in (self.root / "builds", self.root / "runtime", self.root / "tests", self.root / "docs",
                     self.scratch / "CherriSimulatorQA-Part1.xcresult",
                     self.scratch / "CherriSimulatorQA.xcresult"):
            path.mkdir(parents=True)
        self.container = self.home / UDID / "data" / "Containers" / "Data" / "Application" / "CONTROL"
        self.container.mkdir(parents=True)
        self.manifest = {"syntheticOnly":True, "compilerCommitSha":SOURCE, "fixtures":[{
            "fixtureId":FIXTURE,"fileName":FIXTURE+".cherri","sourceSha256":"1"*64,
            "unsignedPlistSha256":"2"*64,"buildProfile":"debug","runtimeTraceExpectation":"REQUIRED_NDJSON"}]}
        (self.root / "compiler-manifest.json").write_text(json.dumps(self.manifest), encoding="utf-8")
        self.expected = terminal.context(self.root, SOURCE, RUN, FIXTURE)
        self.gates = {key:"NOT_RUN" for key in terminal.GATES}
        self.gates["RUNNER_AVAILABLE"] = "PASS"
        self.gates["PHYSICAL_DEVICE_TESTED"] = "BLOCKED_DEVICE"
        self.clock = 10_000_000_000
        self.calls = []
        self.now = lambda:self.clock
        terminal.checkpoint(self.root,self.expected,"COMPILED",self.gates,
                            require_runtime=True,now_ns=self.now)

    def start(self, complete=True, elapsed_ms=100):
        self.clock += 1_000_000
        terminal.checkpoint(self.root,self.expected,"TEST02_STARTED",self.gates,
                            device=UDID,nonce="fresh-control-nonce",require_runtime=True,now_ns=self.now)
        self.clock += elapsed_ms * 1_000_000
        if complete:
            terminal.checkpoint(self.root,self.expected,"TEST02_COMPLETED",self.gates,
                                device=UDID,nonce="fresh-control-nonce",code=0,
                                require_runtime=True,now_ns=self.now)

    def runner(self, args, allowance, *, output=None, error_output=None):
        self.calls.append((list(args), allowance))
        self.clock += 1_000_000
        if "xcresulttool" in args:
            output.write(b'{"result":"Passed","passedTests":1}\n')
        else:
            output.write((str(self.container)+"\n").encode())
        error_output.write(b"INDEPENDENT_STDERR_CONTROL\n")
        return {"result":"PASS","exitCode":0,"elapsedMs":1}

    def collect(self, runner=None):
        return terminal.collect(self.root,self.expected,self.scratch,UDID,
                                runner=runner or self.runner,now_ns=self.now)

    def test_positive_uses_exact_commands_and_original_remaining_phase(self):
        self.start(elapsed_ms=899_995)
        code, value = self.collect()
        self.assertEqual(code,0)
        self.assertEqual(value["status"],"COLLECTED")
        self.assertEqual([x[1] for x in self.calls],[.005,.004,.003])
        self.assertEqual(self.calls[0][0],["xcrun","xcresulttool","get","test-results","summary","--path",
                                         str(self.scratch/"CherriSimulatorQA-Part1.xcresult")])
        self.assertEqual(self.calls[2][0],["xcrun","simctl","get_app_container",UDID,
                                         "org.cherrilang.qa.Receiver","data"])
        terminal.validate_inspection(value,self.root,self.expected)
        self.assertEqual(terminal.receiver_path(self.root,self.expected),str(self.container))
        for row in value["commands"]:
            self.assertNotEqual(row["stdout"]["sha256"],row["stderr"]["sha256"])
            self.assertEqual((self.root/row["stderr"]["path"]).read_bytes(),b"INDEPENDENT_STDERR_CONTROL\n")
        self.assertFalse(value["formalAcceptanceGranted"])

    def test_exhausted_budget_never_launches_and_does_not_create_working_summary(self):
        self.start(elapsed_ms=900_000)
        code,value = self.collect()
        self.assertEqual(code,70)
        self.assertEqual(self.calls,[])
        self.assertTrue(all(x["result"]=="DEADLINE_EXHAUSTED" and x["allowanceMs"]==0
                            for x in value["commands"]))
        self.assertFalse((self.root/"tests/xcresult-part1.json").exists())

    def test_late_zero_exit_is_not_parsed_or_promoted(self):
        self.start(elapsed_ms=899_990)
        def late(args, allowance, **streams):
            streams["output"].write(b'{"result":"Passed"}')
            self.clock += 11_000_000
            return {"result":"PASS","exitCode":0,"elapsedMs":11}
        code,value = self.collect(late)
        self.assertEqual(code,70)
        self.assertEqual(value["commands"][0]["result"],"DEADLINE_EXHAUSTED")
        self.assertEqual(value["commands"][0]["exitCode"],0)
        self.assertEqual(value["commands"][0]["parseStatus"],"NOT_READ")
        self.assertFalse((self.root/"tests/xcresult-part1.json").exists())

    def test_nonzero_exit_preserves_both_outputs_and_does_not_parse_success_text(self):
        self.start()
        def failed(args, allowance, **streams):
            self.clock += 2_000_000
            streams["output"].write(b'{"result":"Passed"}')
            streams["error_output"].write(b"OBSERVED_ERROR_7")
            return {"result":"EXIT_NONZERO","exitCode":7,"elapsedMs":2}
        _,value = self.collect(failed)
        self.assertTrue(all(x["exitCode"]==7 and x["parseStatus"]=="NOT_READ" for x in value["commands"]))
        self.assertEqual((self.root/"builds/post-test-xcresult-part1.stderr").read_bytes(),b"OBSERVED_ERROR_7")
        self.assertFalse((self.root/"tests/xcresult-part1.json").exists())

    def test_interrupt_preserves_partial_bytes_and_stops_without_another_query(self):
        self.start()
        def interrupted(args, allowance, **streams):
            self.calls.append(args)
            self.clock += 4_000_000
            streams["output"].write(b"ORIGINAL_PARTIAL_STDOUT")
            streams["error_output"].write(b"ORIGINAL_PARTIAL_STDERR")
            raise terminal.InspectionCancelled(int(signal.SIGTERM))
        code,value = self.collect(interrupted)
        self.assertEqual(code,128+int(signal.SIGTERM))
        self.assertEqual(len(self.calls),1)
        self.assertEqual(value["commands"][0]["result"],"CANCELLED")
        self.assertIsNone(value["commands"][0]["exitCode"])
        self.assertEqual(value["status"],"INCOMPLETE")
        self.assertEqual((self.root/"builds/post-test-xcresult-part1.stdout").read_bytes(),
                         b"ORIGINAL_PARTIAL_STDOUT")
        terminal.validate_inspection(value,self.root,self.expected)

    def test_unavailable_raw_original_remains_rejected(self):
        self.start()
        _,value = self.collect()
        raw = self.root/value["commands"][0]["stderr"]["path"]
        raw.unlink()
        with self.assertRaisesRegex(ValueError,"ORIGINAL_UNAVAILABLE"):
            terminal.validate_inspection(value,self.root,self.expected)

    def test_hash_path_context_time_and_success_forgery_are_rejected(self):
        self.start()
        _,good = self.collect()
        mutations = {
            "path":lambda d:d["commands"][0]["stdout"].update(path="../outside"),
            "digest":lambda d:d["commands"][0]["stdout"].update(sha256="0"*64),
            "size":lambda d:d["commands"][0]["stdout"].update(bytes=999),
            "source":lambda d:d.update(compilerCommitSha="b"*40),
            "run":lambda d:d.update(runId="wrong"),
            "fixture":lambda d:d.update(targetFixtureId="wrong"),
            "budget":lambda d:d.update(phaseBudgetMs=900001),
            "allowance":lambda d:d["commands"][0].update(allowanceMs=20001),
            "duration":lambda d:d["commands"][0].update(elapsedMs=800000),
            "clock":lambda d:d["commands"][1].update(phaseStartMs=0),
            "parse":lambda d:d["commands"][0].update(parseStatus="RECEIVER_CONTAINER"),
            "booleanExit":lambda d:d["commands"][0].update(exitCode=False),
            "formal":lambda d:d.update(formalAcceptanceGranted=True),
            "unknown":lambda d:d.update(unreviewed=True),
        }
        for name, mutate in mutations.items():
            with self.subTest(name=name):
                bad=copy.deepcopy(good);mutate(bad)
                with self.assertRaises(ValueError):
                    terminal.validate_inspection(bad,self.root,self.expected)

    def test_original_changed_to_same_size_cannot_be_rebound_to_pass(self):
        self.start()
        _,value=self.collect()
        row=value["commands"][0]
        path=self.root/row["stdout"]["path"]
        path.write_bytes(b"X"*path.stat().st_size)
        with self.assertRaisesRegex(ValueError,"ORIGINAL_BINDING_MISMATCH"):
            terminal.validate_inspection(value,self.root,self.expected)
        row["stdout"]=terminal.file_binding(self.root,row["stdout"]["path"])
        with self.assertRaisesRegex(ValueError,"INSPECTION_ORIGINAL_PARSE"):
            terminal.validate_inspection(value,self.root,self.expected)

    def test_stale_working_summary_and_repeated_collection_are_not_accepted(self):
        self.start()
        path=self.root/"tests/xcresult-part1.json"
        path.write_text('{"result":"Passed"}')
        with self.assertRaisesRegex(ValueError,"STALE_INSPECTION_OUTPUT"):
            self.collect()
        self.assertEqual(self.calls,[])
        path.unlink()
        self.collect()
        for p in (self.root/"tests").glob("xcresult-part*.json"):
            p.unlink()
        with self.assertRaisesRegex(ValueError,"INSPECTION_ALREADY_ATTEMPTED"):
            self.collect()
        self.assertEqual(len(self.calls),3)

    def test_exception_after_commands_cannot_enable_replay(self):
        self.start()
        original=terminal.write_new
        def fail_receipt(root,relative,value):
            if relative=="runtime/post-test-inspection.json":
                raise OSError("controlled receipt write failure")
            return original(root,relative,value)
        with patch.object(terminal,"write_new",fail_receipt):
            with self.assertRaises(OSError):
                self.collect()
        for p in (self.root/"tests").glob("xcresult-part*.json"):
            p.unlink()
        with self.assertRaisesRegex(ValueError,"INSPECTION_ALREADY_ATTEMPTED"):
            self.collect()
        self.assertEqual(len(self.calls),3)
        self.assertTrue((self.root/"builds/post-test-xcresult-part1.stdout").is_file())

    def test_failed_or_missing_summary_output_is_not_filled_with_pass(self):
        self.start()
        (self.scratch/"CherriSimulatorQA-Part1.xcresult").rmdir()
        def invalid(args,allowance,**streams):
            self.clock+=1_000_000
            streams["output"].write(b"not JSON")
            return {"result":"PASS","exitCode":0,"elapsedMs":1}
        _,value=self.collect(invalid)
        self.assertEqual(value["commands"][0]["result"],"MISSING_INPUT")
        self.assertEqual(value["commands"][1]["parseStatus"],"INVALID")
        self.assertEqual(value["status"],"INCOMPLETE")
        self.assertFalse((self.root/"tests/xcresult-part2.json").exists())

    def test_late_partial_finalizer_is_explicitly_incomplete_without_callback_or_trace(self):
        self.start(complete=False)
        self.clock+=901_000_000_000
        result=terminal.finalize(self.root,self.expected,"cancelled",now_ns=self.now)
        self.assertEqual(result["status"],"INCOMPLETE_SUMMARY_RETAINED")
        summary=json.loads((self.root/"summary.json").read_text())
        assertion=json.loads((self.root/"runtime/output-assertions.json").read_text())
        receipt=json.loads((self.root/"runtime/terminal-acquisition.json").read_text())
        self.assertTrue(receipt["phaseExpired"])
        self.assertEqual(receipt["lastAcquiredStage"],"TEST02_STARTED")
        self.assertIsNone(receipt["test02PhaseExitCode"])
        self.assertFalse(summary["runtimeContractVerified"])
        self.assertFalse(assertion["compiledShortcutCallbackVerified"])
        self.assertEqual(summary["gates"]["SHORTCUT_RUN_COMPLETED"],"NOT_RUN")
        self.assertEqual(summary["gates"]["RUNNER_AVAILABLE"],"PASS")
        self.assertFalse((self.root/"runtime/callback-events.json").exists())
        self.assertFalse((self.root/"runtime/runtime-trace.jsonl").exists())

    def test_interrupted_receipt_write_remains_private_without_public_partial_file(self):
        path=self.root/"summary.json"
        with patch.object(terminal.os,"fsync",side_effect=OSError("controlled fsync interruption")):
            with self.assertRaises(OSError):
                terminal.write_new(self.root,"summary.json",{"status":"INCOMPLETE_COLLECTION"})
        self.assertFalse(path.exists())
        retained=list((self.root/"builds").glob(".qa-receipt-*.tmp"))
        self.assertEqual(len(retained),1)
        self.assertIn(b"INCOMPLETE_COLLECTION",retained[0].read_bytes())

    def test_zero_test_phase_exit_does_not_establish_import_or_runtime(self):
        self.start()
        terminal.finalize(self.root,self.expected,"failure",now_ns=self.now)
        receipt=json.loads((self.root/"runtime/terminal-acquisition.json").read_text())
        self.assertEqual(receipt["test02PhaseExitCode"],0)
        self.assertFalse(receipt["runtimeAcceptanceGranted"])
        summary=json.loads((self.root/"summary.json").read_text())
        self.assertEqual(summary["gates"]["COMPILER_ARTIFACT_IMPORTED"],"NOT_RUN")
        self.assertFalse(summary["runtimeContractVerified"])

    def test_existing_completed_summary_is_byte_identical_after_late_finalizer(self):
        self.start()
        data=b'{"compilerCommitSha":"'+SOURCE.encode()+b'","runId":"'+RUN.encode()+b'","runtimeContractVerified":true,"control":"completed receipt"}\n'
        (self.root/"summary.json").write_bytes(data)
        self.clock+=2_000_000_000_000
        out=terminal.finalize(self.root,self.expected,"cancelled",now_ns=self.now)
        self.assertEqual(out["status"],"EXISTING_SUMMARY_RETAINED")
        self.assertEqual((self.root/"summary.json").read_bytes(),data)
        self.assertFalse((self.root/"runtime/output-assertions.json").exists())

    def test_completed_summary_race_wins_exclusive_write_without_overwrite(self):
        self.start()
        original=terminal.write_new
        data=json.dumps({"compilerCommitSha":SOURCE,"runId":RUN,"runtimeContractVerified":True}).encode()
        def race(root,relative,value):
            if relative=="summary.json":
                (Path(root)/relative).write_bytes(data)
            return original(root,relative,value)
        with patch.object(terminal,"write_new",race):
            out=terminal.finalize(self.root,self.expected,"cancelled",now_ns=self.now)
        self.assertEqual(out["status"],"EXISTING_SUMMARY_RETAINED")
        self.assertEqual((self.root/"summary.json").read_bytes(),data)

    def test_existing_callback_assertion_and_trace_bytes_are_never_rewritten(self):
        self.start()
        originals={
            "runtime/output-assertions.json":json.dumps({"compilerCommitSha":SOURCE,
                "targetFixtureId":FIXTURE,"runtimeContractVerified":True}).encode(),
            "runtime/callback-events.json":b'[{"observed":"PUBLIC_CONTROL"}]\n',
            "runtime/runtime-trace.jsonl":b'{"observed":"PUBLIC_TRACE_CONTROL"}\n',
        }
        for path,data in originals.items():(self.root/path).write_bytes(data)
        terminal.finalize(self.root,self.expected,"cancelled",now_ns=self.now)
        for path,data in originals.items():self.assertEqual((self.root/path).read_bytes(),data)
        self.assertFalse(json.loads((self.root/"summary.json").read_text())["runtimeContractVerified"])

    def test_partial_publication_requires_exact_checkpoint_and_original_bindings(self):
        self.start()
        self.collect()
        terminal.finalize(self.root,self.expected,"cancelled",now_ns=self.now)
        out=self.home/"public"
        result=publication.stage(self.root,out,{FIXTURE+".cherri":"1"*64})
        self.assertTrue(result["publicationGatePassed"])
        self.assertFalse((out/"builds").exists())
        for name in ("runtime/post-test-inspection.json","runtime/terminal-acquisition.json"):
            self.assertEqual((out/name).read_bytes(),(self.root/name).read_bytes())
        path=self.root/"builds/qa-acquisition-test02_started.json"
        path.write_bytes(path.read_bytes()+b" ")
        with self.assertRaisesRegex(publication.PublicationBlocked,"INVALID_BOUNDED_ACQUISITION_RECEIPT"):
            publication.stage(self.root,self.home/"bad-public",{FIXTURE+".cherri":"1"*64})

    def test_partial_publication_rejects_missing_checkpoint_or_assertion_hash(self):
        self.start()
        terminal.finalize(self.root,self.expected,"cancelled",now_ns=self.now)
        path=self.root/"runtime/output-assertions.json"
        original=path.read_bytes();path.write_bytes(original+b" ")
        with self.assertRaisesRegex(publication.PublicationBlocked,"INVALID_BOUNDED_ACQUISITION_RECEIPT"):
            publication.stage(self.root,self.home/"bad-public",{FIXTURE+".cherri":"1"*64})

    def test_checkpoint_source_order_duplicates_and_changed_requirement_are_rejected(self):
        with self.assertRaises(FileExistsError):
            terminal.checkpoint(self.root,self.expected,"COMPILED",self.gates,now_ns=self.now)
        with self.assertRaisesRegex(ValueError,"CHECKPOINT_ORDER"):
            terminal.checkpoint(self.root,self.expected,"TEST02_COMPLETED",self.gates,
                device=UDID,code=0,now_ns=self.now)
        self.start()
        records,_=terminal.read_checkpoints(self.root,self.expected)
        bad=copy.deepcopy(records[0]);bad["compilerCommitSha"]="b"*40
        with self.assertRaisesRegex(ValueError,"CHECKPOINT_CONTEXT"):
            terminal.check_checkpoint(bad,self.expected,"COMPILED")

    def test_actual_process_timeout_retains_partial_distinct_streams(self):
        # A real process waits beyond the remaining phase allowance. The test
        # clock is the same monotonic clock shifted to the original phase origin.
        self.start(elapsed_ms=898_000)
        anchor=time.monotonic_ns();base=self.clock
        now=lambda:base+time.monotonic_ns()-anchor
        def child(args, allowance, **streams):
            return recovery.run_process([sys.executable,"-c",
                "import sys,time; print('ORIGINAL_OUT',flush=True); print('ORIGINAL_ERR',file=sys.stderr,flush=True); time.sleep(3)"],
                allowance,**streams)
        code,value=terminal.collect(self.root,self.expected,self.scratch,UDID,runner=child,now_ns=now)
        self.assertEqual(code,70)
        self.assertEqual(value["commands"][0]["result"],"TIMEOUT")
        self.assertIn(b"ORIGINAL_OUT",(self.root/"builds/post-test-xcresult-part1.stdout").read_bytes())
        self.assertIn(b"ORIGINAL_ERR",(self.root/"builds/post-test-xcresult-part1.stderr").read_bytes())
        self.assertLess(value["phaseElapsedMs"],905_000)

    @unittest.skipUnless(os.name=="posix","actual SIGTERM/process-group control requires POSIX")
    def test_actual_mid_child_sigterm_keeps_partial_receipt_and_kills_descendant(self):
        # This runs on the native CI host; Windows runs the separate causal
        # cancellation and actual subprocess timeout controls above.
        for p in (self.root/"builds").glob("qa-acquisition-*.json"):p.unlink()
        clock=lambda:time.monotonic_ns()
        terminal.checkpoint(self.root,self.expected,"COMPILED",self.gates,now_ns=clock)
        terminal.checkpoint(self.root,self.expected,"TEST02_STARTED",self.gates,device=UDID,now_ns=clock)
        terminal.checkpoint(self.root,self.expected,"TEST02_COMPLETED",self.gates,device=UDID,code=0,now_ns=clock)
        binary=self.home/"bin";binary.mkdir()
        delayed=self.home/"forbidden-after-cancel"
        fake=binary/"xcrun"
        fake.write_text("#!"+sys.executable+"\nimport subprocess,sys,time\n"
            "print('CHILD_STARTED',flush=True)\n"
            "subprocess.Popen([sys.executable,'-c',"+repr("import time,pathlib; time.sleep(2); pathlib.Path("+repr(str(delayed))+").write_text('late')")+"])\n"
            "time.sleep(10)\n")
        fake.chmod(0o755)
        worker=("import json,signal,sys;import qa_terminal_receipts as q;"
                "signal.signal(signal.SIGTERM,q._cancel);"
                "code,value=q.collect(sys.argv[1],json.loads(sys.argv[2]),sys.argv[3],sys.argv[4]);"
                "sys.exit(code)")
        env=dict(os.environ,PYTHONPATH=str(HERE),PATH=str(binary)+os.pathsep+os.environ["PATH"])
        process=subprocess.Popen([sys.executable,"-c",worker,str(self.root),
            json.dumps(self.expected),str(self.scratch),UDID],env=env,stdout=subprocess.PIPE,stderr=subprocess.PIPE)
        try:
            raw=self.root/"builds/post-test-xcresult-part1.stdout"
            deadline=time.monotonic()+5
            while time.monotonic()<deadline and (not raw.exists() or b"CHILD_STARTED" not in raw.read_bytes()):
                time.sleep(.01)
            self.assertTrue(raw.exists() and b"CHILD_STARTED" in raw.read_bytes())
            process.send_signal(signal.SIGTERM)
            _,err=process.communicate(timeout=5)
            self.assertEqual(process.returncode,128+signal.SIGTERM,err.decode(errors="replace"))
            value=json.loads((self.root/"runtime/post-test-inspection.json").read_text())
            terminal.validate_inspection(value,self.root,self.expected)
            self.assertEqual(value["commands"][0]["result"],"CANCELLED")
            time.sleep(2.1)
            self.assertFalse(delayed.exists())
        finally:
            if process.poll() is None:process.kill();process.wait(timeout=3)


class PrivacyBoundaryTests(unittest.TestCase):
    def test_decoded_private_fields_are_blocked_but_raw_quarantine_is_untouched(self):
        with tempfile.TemporaryDirectory() as temp:
            root=Path(temp)
            (root/"builds").mkdir()
            raw=root/"builds/original.log"
            private=b"CHERRI_PRIVATE_CANARY "+UDID.encode()
            raw.write_bytes(private)
            cases=[
                '{"value":"'+UDID+'"}',
                '{"value":"'+"".join("\\u%04x"%ord(c)for c in UDID)+'"}',
                '{"value":"CHERRI_\\u0050RIVATE_CANARY"}',
                '{"value":"https:\\u002f\\u002fprivate-signing.invalid/opaque"}',
                '{"duplicate":"public","duplicate":"private"}',
            ]
            for text in cases:
                with self.subTest(text_hash=hashlib.sha256(text.encode()).hexdigest()):
                    (root/"environment.json").write_text(text,encoding="utf-8")
                    got=publication.scan_public_privacy(root,UDID,"https://private-signing.invalid/opaque")
                    self.assertFalse(got["passed"])
                    self.assertEqual(raw.read_bytes(),private)
            (root/"environment.json").write_text('{"status":"OBSERVED"}')
            self.assertTrue(publication.scan_public_privacy(root,UDID)["passed"])
            self.assertEqual(raw.read_bytes(),private)




class SourceAndShellTests(unittest.TestCase):
    def setUp(self):
        self.temp=tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.home=Path(self.temp.name)
        self.repo=self.home/"repo"
        self.helper_dir=self.repo/"tools/shortcuts-simulator-qa"
        (self.helper_dir/"fixtures").mkdir(parents=True)
        self.helper=self.helper_dir/"qa_terminal_receipts.py"
        for relative in terminal.SOURCE_FILES:
            destination_path=self.repo/relative
            destination_path.parent.mkdir(parents=True,exist_ok=True)
            shutil.copyfile(REPO/relative,destination_path)
        self.fixture_bytes=b"// fixed public source-control fixture\n"
        (self.helper_dir/"fixtures"/(FIXTURE+".cherri")).write_bytes(self.fixture_bytes)
        self.git("init","-q")
        self.git("config","core.autocrlf","false")
        self.git("config","core.eol","lf")
        self.git("add","tools","scripts",".github")
        self.git("-c","user.name=Cherri_QA","-c","user.email=qa@example.invalid",
                 "-c","commit.gpgsign=false","commit","-qm","owned acquisition source control")
        self.source=self.git("--no-replace-objects","rev-parse","HEAD").strip()
        self.root=self.repo/"artifacts"/RUN
        for name in ("builds","runtime","tests","docs"):
            (self.root/name).mkdir(parents=True)
        self.manifest={"syntheticOnly":True,"compilerCommitSha":self.source,"fixtures":[{
            "fixtureId":FIXTURE,"fileName":FIXTURE+".cherri",
            "sourceSha256":hashlib.sha256(self.fixture_bytes).hexdigest(),
            "unsignedPlistSha256":"2"*64,"buildProfile":"debug","runtimeTraceExpectation":"REQUIRED_NDJSON"}]}
        (self.root/"compiler-manifest.json").write_text(json.dumps(self.manifest))
        self.gates=["NOT_RUN"]*13+["BLOCKED_DEVICE"]

    def git(self,*args):
        result=subprocess.run(["git",*args],cwd=self.repo,capture_output=True,timeout=20)
        self.assertEqual(result.returncode,0,result.stderr.decode(errors="replace"))
        return result.stdout.decode()

    def command(self,helper=None):
        return [sys.executable,str(helper or self.helper),"checkpoint","--repo",str(self.repo),
                "--root",str(self.root),"--source-sha",self.source,"--run-id",RUN,
                "--fixture-id",FIXTURE,"--stage","COMPILED","--require-runtime","true",
                "--gates",*self.gates]

    def test_actual_cli_uses_exact_owned_checkout_and_source_fixture_bytes(self):
        done=subprocess.run(self.command(),capture_output=True,timeout=30)
        self.assertEqual(done.returncode,0,done.stderr.decode(errors="replace"))
        expected=terminal.context(self.root,self.source,RUN,FIXTURE)
        records,_=terminal.read_checkpoints(self.root,expected)
        self.assertEqual(records[0]["compiledSourceSha256"],hashlib.sha256(self.fixture_bytes).hexdigest())
        self.assertTrue(records[0]["requireRuntime"])

    def test_actual_cli_rejects_dirty_acquisition_source(self):
        self.helper.write_bytes(self.helper.read_bytes()+b"\n# changed acquisition source control\n")
        done=subprocess.run(self.command(),capture_output=True,timeout=30)
        self.assertNotEqual(done.returncode,0)
        self.assertFalse((self.root/"builds/qa-acquisition-compiled.json").exists())

    def test_actual_cli_rejects_borrowing_a_different_checkout_identity(self):
        done=subprocess.run(self.command(HERE/"qa_terminal_receipts.py"),capture_output=True,timeout=30)
        self.assertNotEqual(done.returncode,0)
        self.assertFalse((self.root/"builds/qa-acquisition-compiled.json").exists())

    def test_actual_git_replacement_cannot_rebind_changed_acquisition_code(self):
        self.helper.write_bytes(self.helper.read_bytes()+b"\n# replacement-object acquisition control\n")
        self.git("add","tools")
        self.git("-c","user.name=Cherri_QA","-c","user.email=qa@example.invalid",
                 "-c","commit.gpgsign=false","commit","-qm","replacement-object control")
        replacement=self.git("--no-replace-objects","rev-parse","HEAD").strip()
        self.git("update-ref","HEAD",self.source)
        self.git("replace",self.source,replacement)
        done=subprocess.run(self.command(),capture_output=True,timeout=30)
        self.assertNotEqual(done.returncode,0)
        self.assertFalse((self.root/"builds/qa-acquisition-compiled.json").exists())

    def test_actual_shell_checkpoint_wrapper_preserves_phase_exit_and_requirement(self):
        bash=os.environ.get("CHERRI_TEST_BASH")or shutil.which("bash")
        if not bash:self.skipTest("Bash unavailable")
        shell=SCRIPT.read_text(encoding="utf-8")
        function=shell[shell.index("qa_acquisition_checkpoint() {"):shell.index("# Phase 0: Build")]
        bins=self.home/"bin";bins.mkdir()
        python=bins/"python3"
        python.write_text("#!/usr/bin/env bash\nexec "+ "'" + sys.executable.replace("\\","/").replace("'","'\"'\"'") + "' \"$@\"\n")
        python.chmod(0o755)
        env=dict(os.environ,PATH=str(bins)+os.pathsep+os.environ["PATH"],
                 ROOT_DIR=self.repo.as_posix(),ARTIFACT_DIR=self.root.as_posix(),
                 SOURCE_SHA=self.source,RUN_ID=RUN,TARGET_FIXTURE_ID=FIXTURE,
                 REQUIRE_RUNTIME="true",UDID=UDID,EXEC_RUN_NONCE="fresh-control-nonce")
        env.update({"GATE_"+name:value for name,value in zip(terminal.GATES,self.gates)})
        snippet=function+"\nqa_acquisition_checkpoint COMPILED && qa_acquisition_checkpoint TEST02_STARTED && qa_acquisition_checkpoint TEST02_COMPLETED 7\n"
        done=subprocess.run([bash,"-c",snippet],env=env,capture_output=True,timeout=45)
        self.assertEqual(done.returncode,0,done.stderr.decode(errors="replace"))
        records,_=terminal.read_checkpoints(self.root,terminal.context(self.root,self.source,RUN,FIXTURE))
        self.assertEqual([x["stage"]for x in records],list(terminal.STAGES))
        self.assertEqual(records[-1]["test02PhaseExitCode"],7)
        self.assertEqual(records[1]["testPhaseBudgetMs"],int(destination.PHASE_SECONDS*1000))
        self.assertLessEqual(records[1]["monotonicNs"],records[2]["monotonicNs"])
        self.assertTrue(all(x["requireRuntime"]for x in records))


if __name__ == "__main__":
    unittest.main()
