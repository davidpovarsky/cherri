"""Causal original-byte join controls; all Receiver/Apple data here are synthetic.

The input compiler export and unsigned graph are the retained public originals.
The test signature, process and Receiver receipts are explicit synthetic doubles.
No control invokes simctl, signs/imports/runs a Shortcut, or grants native credit.
"""
import contextlib
import copy
import io
import json
from pathlib import Path
import unittest
from unittest import mock

import function_envelope_r2_launch as launch
import function_envelope_r2_runtime as runtime
from nested_list_runtime import trace_bytes
from test_function_envelope_r2_launch import SyntheticRun
from test_function_envelope_observation import mutate as summary_mutation


class FunctionR2OriginalJoinTests(unittest.TestCase):
    def setUp(self):
        self.fixture = SyntheticRun()
        self.addCleanup(self.fixture.close)
        self.fixture.ready()

    @contextlib.contextmanager
    def original_artifacts(self):
        root = self.fixture.root
        before = {p.relative_to(root): p.read_bytes() for p in root.rglob("*") if p.is_file()}
        try:
            yield
        finally:
            for p in root.rglob("*"):
                if p.is_file() and p.relative_to(root) not in before:
                    p.unlink()
            for relative, raw in before.items():
                p = root / relative
                p.parent.mkdir(parents=True, exist_ok=True)
                if p.is_symlink():
                    p.unlink()
                p.write_bytes(raw)

    def edit(self, relative, mutate):
        path = self.fixture.root / relative
        old = path.read_bytes()
        value = json.loads(old)
        mutate(value)
        self.fixture.json_write(path, value)
        self.assertNotEqual(path.read_bytes(), old, "causal control did not alter its declared input")
        return value

    def assert_rejected(self, reason):
        value = self.fixture.evaluate()
        self.assertFalse(value["runtimeObservationVerified"], value)
        self.assertFalse(value["declaredNoInputGoalsMatched"], value)
        self.assertIn(reason, value["reason"])
        for field in ("semanticContractVerified", "importTransportAuthenticated",
                      "appleExecutionAuthenticated", "formalAcceptanceGranted"):
            self.assertIs(value[field], False)
        return value

    def refresh_process_hashes(self):
        root = self.fixture.root / launch.RELATIVE
        result = json.loads((root / "result.original.json").read_bytes())
        for name in ("process.original.json", "process-started.original.json", "process-finished.original.json"):
            raw = (root / name).read_bytes()
            result[name] = {"bytes": len(raw), "sha256": launch.sha(raw)}
        self.fixture.json_write(root / "result.original.json", result)

    def edit_process(self, mutate, *, finish=True):
        value = self.edit(launch.RELATIVE + "/process.original.json", mutate)
        if finish:
            self.fixture.json_write(self.fixture.root / launch.RELATIVE / "process-finished.original.json",
                                    value["details"])
        self.refresh_process_hashes()

    def test_exact_join_keeps_authorities_and_source_identities_separate(self):
        value = self.fixture.evaluate()
        self.assertTrue(value["runtimeObservationVerified"], value)
        self.assertTrue(value["declaredNoInputGoalsMatched"], value)
        self.assertTrue(value["oneNoInputLaunchContractVerified"], value)
        self.assertEqual(value["compilerCommitSha"], runtime.COMPILER_SOURCE)
        self.assertEqual(value["harnessCommitSha"], self.fixture.commit)
        self.assertNotEqual(value["compilerCommitSha"], value["harnessCommitSha"])
        self.assertEqual(value["launchBinding"]["receiverPID"], 4242)
        self.assertEqual(len(value["inputBindings"]), 7)
        self.assertEqual(len(value["traceRecords"]), 4)
        for field in ("semanticContractVerified", "importTransportAuthenticated",
                      "appleExecutionAuthenticated", "formalAcceptanceGranted"):
            self.assertIs(value[field], False)
        for field in ("launchInputAuthenticated", "sourceAuthenticated",
                      "appleExecutionAuthenticated", "formalAcceptanceGranted"):
            self.assertIs(value["observation"][field], False)

    def test_contrary_observations_are_preserved_without_no_input_goal_credit(self):
        cases = (
            ("missing_caller", summary_mutation("probeZero", CALLER_KEY="0",
                CALLER_FIELD_COUNT="UNOBSERVED", CALLER_FIELD_TYPE64="UNOBSERVED")),
            ("nonempty_caller", summary_mutation("probeZero", CALLER_FIELD_COUNT="1",
                CALLER_FIELD_TYPE64="VGV4dA==", CALLER_SAVED_COUNT="1",
                CALLER_SAVED_TYPE64="VGV4dA==")),
            ("empty_default", summary_mutation("probeDefault", PARAMETER="")),
            ("present_optional", summary_mutation("probeOptional", PARAMETER="MA==")),
        )
        for name, summary in cases:
            with self.subTest(name=name), self.original_artifacts():
                self.edit("runtime/callback-events.json", lambda rows: rows[0].update(resultSummary=summary))
                value = self.fixture.evaluate()
                self.assertTrue(value["runtimeObservationVerified"], value)
                self.assertFalse(value["declaredNoInputGoalsMatched"], value)
                self.assertIs(value["formalAcceptanceGranted"], False)
                if name == "missing_caller":
                    fields = value["observation"]["caseObservations"]["probeZero"]["actualFields"]
                    self.assertEqual(fields["CALLER_FIELD_COUNT"], "UNOBSERVED")

    def test_retained_compiler_signed_manifest_and_exact_source_causal_guards(self):
        cases = (
            ("compiler_identity", "compiler-manifest.json",
             lambda v: v.update(compilerCommitSha=self.fixture.commit), "MANIFEST_IDENTITY"),
            ("harness_identity", "compiler-manifest.json",
             lambda v: v.update(harnessCommitSha=runtime.COMPILER_SOURCE), "MANIFEST_IDENTITY"),
            ("origin", "compiler-manifest.json",
             lambda v: v.update(compilerArtifactOrigin="rebuilt"), "MANIFEST_IDENTITY"),
            ("duplicate_fixture", "compiler-manifest.json",
             lambda v: v["fixtures"].append(copy.deepcopy(v["fixtures"][0])), "MANIFEST_EXACT_FIXTURE"),
            ("wrong_fixture", "compiler-manifest.json",
             lambda v: v["fixtures"][0].update(fixtureId="qa-nested-list-control-v2"), "MANIFEST_EXACT_FIXTURE"),
            ("wrong_profile", "compiler-manifest.json",
             lambda v: v["fixtures"][0].update(buildProfile="release"), "PREPARED_buildProfile"),
            ("bool_action_count", "compiler-manifest.json",
             lambda v: v["fixtures"][0].update(nativeActionCount=True), "PREPARED_nativeActionCount"),
            ("wrong_graph_proof", "compiler-manifest.json",
             lambda v: v["fixtures"][0]["functionEnvelopeR2Probe"].update(firstAndOnlySemanticInvocation=False),
             "PREPARED_functionEnvelopeR2Probe"),
            ("wrong_sign_compiler", "runtime/signing-provenance.json",
             lambda v: v.update(compilerCommitSha=self.fixture.commit), "SIGNING_SUMMARY"),
            ("wrong_sign_hash", "runtime/signing-provenance.json",
             lambda v: v.update(signedArtifactSha256="0"*64), "SIGNING_SUMMARY"),
            ("signed_bool_size", "runtime/signing-provenance.json",
             lambda v: v.update(signedByteSize=True), "SIGNING_SUMMARY"),
            ("wrong_signed_source", "runtime/signing-provenance.json",
             lambda v: v["targetProvenance"].update(sourceSha256="0"*64), "SIGNED_sourceSha256"),
            ("wrong_signed_unsigned", "runtime/signing-provenance.json",
             lambda v: v["targetProvenance"].update(unsignedPlistSha256="0"*64), "SIGNED_unsignedPlistSha256"),
            ("wrong_signed_count", "runtime/signing-provenance.json",
             lambda v: v["targetProvenance"].update(nativeActionCount=378), "SIGNED_nativeActionCount"),
        )
        for name, relative, change, code in cases:
            with self.subTest(name=name), self.original_artifacts():
                self.edit(relative, change)
                self.assert_rejected(code)
        for relative, suffix, code in (
            ("builds/function-envelope-r2-compiler-export.original.json", b"\n", "COMPILER_EXPORT_BYTES"),
            ("builds/" + launch.WORKFLOW + ".shortcut", b"\n", "RETAINED_COMPILER_ARTIFACT"),
            ("builds/" + launch.WORKFLOW + ".cherri", b"\n", "RETAINED_COMPILER_ARTIFACT"),
            ("builds/signed/" + launch.WORKFLOW + ".shortcut", b"x", "SIGNING_SUMMARY"),
        ):
            with self.subTest(relative=relative), self.original_artifacts():
                p = self.fixture.root / relative
                p.write_bytes(p.read_bytes() + suffix)
                self.assert_rejected(code)

    def test_claim_result_and_one_invocation_causal_guards(self):
        cases = (
            ("retry", "attempt.claim.original.json", lambda v: v.update(automaticRetry=True), "LAUNCH_automaticRetry"),
            ("fallback", "attempt.claim.original.json", lambda v: v.update(fallbackAllowed=True), "LAUNCH_fallbackAllowed"),
            ("second_attempt", "attempt.claim.original.json", lambda v: v.update(attemptOrdinal=2), "LAUNCH_CLAIM"),
            ("count_bool", "attempt.claim.original.json", lambda v: v.update(semanticAttemptCount=True), "LAUNCH_semanticAttemptCount"),
            ("child_budget", "attempt.claim.original.json", lambda v: v.update(processTimeoutSeconds=26), "LAUNCH_CLAIM"),
            ("owner_budget", "attempt.claim.original.json", lambda v: v.update(wholeLaunchOwnerSeconds=36), "LAUNCH_CLAIM"),
            ("claim_source", "attempt.claim.original.json", lambda v: v.update(harnessCommitSha=runtime.COMPILER_SOURCE), "LAUNCH_harnessCommitSha"),
            ("claim_input", "attempt.claim.original.json", lambda v: v["command"].append("input=text"), "LAUNCH_command"),
            ("not_pass", "result.original.json", lambda v: v.update(status="FAIL"), "LAUNCH_RESULT"),
            ("source_changed", "result.original.json", lambda v: v.update(sourceUnchanged=False), "LAUNCH_RESULT"),
            ("over_budget", "result.original.json", lambda v: v.update(elapsedSeconds=35), "LAUNCH_RESULT"),
            ("negative_elapsed", "result.original.json", lambda v: v.update(elapsedSeconds=-1), "LAUNCH_RESULT"),
            ("bool_elapsed", "result.original.json", lambda v: v.update(elapsedSeconds=True), "LAUNCH_RESULT"),
            ("fake_native", "result.original.json", lambda v: v.update(nativeExecutionAuthenticated=True), "LAUNCH_RESULT"),
            ("fake_formal", "result.original.json", lambda v: v.update(formalAcceptanceGranted=True), "LAUNCH_RESULT"),
            ("result_pid", "result.original.json", lambda v: v.update(receiverPID=4243), "LAUNCH_RECEIVER_PID"),
            ("result_compiler", "result.original.json", lambda v: v.update(compilerCommitSha=self.fixture.commit), "LAUNCH_compilerCommitSha"),
        )
        for name, filename, change, code in cases:
            with self.subTest(name=name), self.original_artifacts():
                self.edit(launch.RELATIVE + "/" + filename, change)
                self.assert_rejected(code)

    def test_actual_process_stream_journal_and_cleanup_causal_guards(self):
        cases = (
            ("exit", lambda v: v["observation"].update(exitCode=7), "LAUNCH_PROCESS"),
            ("bool_exit", lambda v: v["observation"].update(exitCode=False), "LAUNCH_PROCESS"),
            ("result", lambda v: v["observation"].update(result="TIMEOUT"), "LAUNCH_PROCESS"),
            ("not_reaped", lambda v: v["details"].update(cleanupParentReaped=False), "LAUNCH_PROCESS"),
            ("group_remaining", lambda v: v["details"].update(groupPresentAfterCleanup=True), "LAUNCH_PROCESS"),
            ("command_input", lambda v: v["command"].append("text=wrong"), "LAUNCH_COMMAND"),
            ("stdout_eof", lambda v: v["details"]["stdout"].update(eof=False), "LAUNCH_STREAM"),
            ("stderr_overflow", lambda v: v["details"]["stderr"].update(overflow=True), "LAUNCH_STREAM"),
            ("stdout_hash", lambda v: v["details"]["stdout"].update(sha256="0"*64), "LAUNCH_STREAM"),
            ("stderr_bool_count", lambda v: v["details"]["stderr"].update(bytes=False), "LAUNCH_STREAM"),
            ("pid_mismatch", lambda v: v["details"].update(pid=3132), "LAUNCH_JOURNAL"),
            ("time_reversal", lambda v: v["details"].update(finishedAt="2000-01-01T00:00:00+00:00"), "LAUNCH_PROCESS_TIME"),
        )
        for name, change, code in cases:
            with self.subTest(name=name), self.original_artifacts():
                self.edit_process(change)
                self.assert_rejected(code)
        for filename in ("process-started.original.json", "process-finished.original.json"):
            with self.subTest(missing=filename), self.original_artifacts():
                (self.fixture.root / launch.RELATIVE / filename).unlink()
                self.assert_rejected("FUNCTION_LAUNCH_ORIGINAL_FILE")
        with self.original_artifacts():
            p = self.fixture.root / launch.RELATIVE / "stdout.original"
            p.write_bytes(b"org.cherrilang.qa.Receiver: 4243\n")
            self.assert_rejected("LAUNCH_STREAM")

    def test_physical_source_witness_cannot_be_rebound_to_a_forged_git_blob(self):
        p = self.fixture.root / launch.RELATIVE
        before = json.loads((p / "source.before.original.json").read_bytes())
        before["files"][0]["sha256"] = "0" * 64
        for name in ("source.before.original.json", "source.after.original.json"):
            self.fixture.json_write(p / name, before)
        context = self.edit(launch.RELATIVE + "/prepared.original.json", lambda v: v.update(
            sourceSnapshotSHA256=launch.sha((p / "source.before.original.json").read_bytes())))
        context_sha = launch.sha((p / "prepared.original.json").read_bytes())
        self.edit(launch.RELATIVE + "/attempt.claim.original.json", lambda v: v.update(contextSHA256=context_sha))
        self.edit(launch.RELATIVE + "/result.original.json", lambda v: v.update(contextSHA256=context_sha))
        self.assert_rejected("LAUNCH_SOURCE_GIT")

    def test_receiver_owner_and_every_declared_identity_are_causal(self):
        mutations = (
            ("schema", "other"), ("mode", "fallback"), ("attemptOrdinal", 2),
            ("processID", 4243), ("contractSHA256", "0"*64), ("contractByteCount", 1),
            ("source", "argv"), ("attemptConsumed", False), ("requestedURLUTF8Bytes", 1),
            ("requestedURLSHA256", "0"*64), ("requestedURLOriginalRetained", False),
            ("contractOriginalRetained", False), ("retentionFailure", True),
            ("requestedURL", "shortcuts://run-shortcut?name=other&input=text"),
            ("requestedURLUTF8Base64", "eA=="), ("fixtureId", "other"), ("workflowName", "other"),
            ("runNonce", "other"), ("primaryURL", "shortcuts://run-shortcut?name=other"),
            ("primaryURLUTF8Base64", "eA=="), ("primaryURLUTF8Bytes", True),
            ("primaryURLSHA256", "0"*64),
        )
        for filename in ("001-intent.original.json", "002-completion.original.json"):
            for field, bad in mutations:
                with self.subTest(file=filename, field=field), self.original_artifacts():
                    self.edit(launch.TRIGGERS + "/" + filename, lambda v, f=field, b=bad: v.update({f:b}))
                    self.fixture.refresh_trigger_inventory()
                    self.assert_rejected("TRIGGER_" + field)
        for name, change, code in (
            ("completion_false", lambda v: v.update(success=False), "TRIGGER_success"),
            ("wrong_sequence", lambda v: v.update(sequence=3), "TRIGGER_sequence"),
            ("bool_sequence", lambda v: v.update(sequence=True), "TRIGGER_sequence"),
            ("wrong_event", lambda v: v.update(event="intent"), "TRIGGER_event"),
            ("extra_field", lambda v: v.update(extra="x"), "TRIGGER_FIELDS"),
            ("bad_token", lambda v: v.update(invocationToken="not-uuid"), "TRIGGER_TOKEN"),
            ("other_token", lambda v: v.update(invocationToken="12345678-1234-4234-8234-1234567890AC"), "TRIGGER_TOKEN_PAIR"),
            ("old_time", lambda v: v.update(timestampISO8601="2000-01-01T00:00:00Z"), "TRIGGER_TIME_ORDER"),
            ("future_time", lambda v: v.update(timestampISO8601="2999-01-01T00:00:00Z"), "TRIGGER_TIME_ORDER"),
        ):
            with self.subTest(name=name), self.original_artifacts():
                self.edit(launch.TRIGGERS + "/002-completion.original.json", change)
                self.fixture.refresh_trigger_inventory()
                self.assert_rejected(code)
        with self.original_artifacts():
            self.edit(launch.TRIGGERS + "/owner.original.json", lambda v: v.update(processID=4243))
            self.fixture.refresh_trigger_inventory()
            self.assert_rejected("TRIGGER_OWNER")

    def test_trigger_closed_world_missing_original_and_retention_failures(self):
        for filename in ("001-intent.original.json", "002-completion.original.json",
                         "contract.original.json", "owner.original.json"):
            with self.subTest(missing=filename), self.original_artifacts():
                (self.fixture.root / launch.TRIGGERS / filename).unlink()
                self.fixture.refresh_trigger_inventory()
                self.assert_rejected("TRIGGER_EXACT_ORIGINALS")
        with self.original_artifacts():
            launch.record(self.fixture.root / launch.TRIGGERS / "003-rejection.original.json",
                          {"reason": "REPEATED_ATTEMPT"})
            self.fixture.refresh_trigger_inventory()
            self.assert_rejected("TRIGGER_EXACT_ORIGINALS")
        with self.original_artifacts():
            self.edit("runtime/function-envelope-r2-trigger-acquisition.original.json",
                      lambda v: v.update(byteIdentical=False))
            self.assert_rejected("TRIGGER_ACQUISITION")
        with self.original_artifacts():
            p = self.fixture.root / launch.TRIGGERS / "contract.original.json"
            p.write_bytes(p.read_bytes() + b"\n")
            self.fixture.refresh_trigger_inventory()
            self.assert_rejected("TRIGGER_CONTRACT")

    def test_callback_unique_nonce_origin_status_and_trace_causal_guards(self):
        mutations = (
            ("duplicate", lambda v: v.append(copy.deepcopy(v[0])), "CALLBACK_UNIQUE_NONCE"),
            ("nonce", lambda v: v[0].update(runId="other"), "CALLBACK_UNIQUE_NONCE"),
            ("selftest", lambda v: v[0].update(receiverSelfTest=True), "CALLBACK_UNIQUE_NONCE"),
            ("selftest_missing", lambda v: v[0].pop("receiverSelfTest"), "CALLBACK_ORIGIN"),
            ("event_source", lambda v: v[0].update(eventSource="receiver-self-test"), "CALLBACK_ORIGIN"),
            ("fixture", lambda v: v[0].update(fixtureId="other"), "CALLBACK_ORIGIN"),
            ("workflow", lambda v: v[0].update(expectedShortcutName="other"), "CALLBACK_ORIGIN"),
            ("error", lambda v: v[0].update(status="ERROR"), "CALLBACK_SUCCESS"),
            ("cancelled", lambda v: v[0].update(status="CANCELLED"), "CALLBACK_SUCCESS"),
            ("summary", lambda v: v[0].update(resultSummary="wrong"), "FUNCTION_ENVELOPE_SUMMARY_LINES"),
            ("trace_missing", lambda v: v[0].update(tracePayload=""), "TRACE_SIZE"),
            ("trace_extra", lambda v: v[0].update(tracePayload=v[0]["tracePayload"]+"\n{}"), "TRACE_FOUR_EVENTS"),
        )
        for name, change, code in mutations:
            with self.subTest(name=name), self.original_artifacts():
                self.edit("runtime/callback-events.json", change)
                self.assert_rejected(code)
        trace_changes = (
            ("collector", 0, "runId", "other", "TRACE_COLLECTOR"),
            ("before_sequence", 1, "stepSequence", 361, "TRACE_INLINE"),
            ("before_action", 1, "actionIdentifier", "is.workflow.actions.runworkflow", "TRACE_INLINE"),
            ("after_sequence", 2, "stepSequence", 369, "TRACE_INLINE"),
            ("after_action", 2, "actionIdentifier", "is.workflow.actions.setvariable", "TRACE_INLINE"),
            ("missing_stamp", 2, "timestamp", "", "TRACE_TIMESTAMP"),
            ("attachment_stamp", 2, "timestamp", "\ufffc", "TRACE_TIMESTAMP"),
            ("label", 2, "label", "other", "TRACE_INLINE"),
        )
        for name, index, field, bad, code in trace_changes:
            with self.subTest(name=name), self.original_artifacts():
                def edit_trace(events):
                    rows = [json.loads(line) for line in events[0]["tracePayload"].splitlines()]
                    rows[index][field] = bad
                    events[0]["tracePayload"] = "\n".join(launch.canonical(row).decode() for row in rows)
                self.edit("runtime/callback-events.json", edit_trace)
                self.assert_rejected(code)


    def test_r2_preserves_historical_failed_export_and_grammar_scope(self):
        root = self.fixture.root
        raw = (root / "builds/function-envelope-r2-compiler-export.original.json").read_bytes()
        original = runtime.validate_compiler_export(raw, self.fixture.source, self.fixture.unsigned, self.fixture.compiler)
        self.assertEqual(original["ownerStatusPreserved"], "FAILED")
        self.assertEqual(original["fullGraphValidation"], "PENDING_SEPARATE_QUALIFICATION")
        value = self.fixture.evaluate()
        self.assertTrue(value["runtimeObservationVerified"], value)
        self.assertEqual(value["observation"]["fixtureId"], launch.FIXTURE)
        self.assertEqual(value["observation"]["grammarFixtureId"], "qa-function-envelope-presence-v2")
        self.assertTrue(value["observation"]["grammarOnlyReuse"])
        for field in ("semanticContractVerified", "importTransportAuthenticated", "appleExecutionAuthenticated", "formalAcceptanceGranted"):
            self.assertIs(value[field], False)
        for field in ("ownerStatusPreserved", "fullGraphValidation"):
            with self.subTest(field=field), self.original_artifacts():
                self.edit("builds/function-envelope-r2-compiler-export.original.json", lambda v, field=field: v.update({field:"PASS"}))
                self.assert_rejected("COMPILER_EXPORT_BYTES")

    def test_historical_packet_cannot_receive_r2_identity(self):
        here = Path(__file__).resolve().parent
        historical = (
            ("builds/function-envelope-r2-compiler-export.original.json", "testdata/function-envelope-compiler-export.original.json", "COMPILER_EXPORT_BYTES"),
            ("builds/" + launch.WORKFLOW + ".shortcut", "testdata/function-envelope-presence-compiled.original.plist", "RETAINED_COMPILER_ARTIFACT"),
            ("builds/" + launch.WORKFLOW + ".cherri", "fixtures/qa-function-envelope-presence-v2.cherri", "RETAINED_COMPILER_ARTIFACT"),
        )
        for target, source, code in historical:
            with self.subTest(source=source), self.original_artifacts():
                (self.fixture.root / target).write_bytes((here / source).read_bytes())
                self.assert_rejected(code)


if __name__ == "__main__":
    unittest.main()
