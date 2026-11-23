"""Synthetic transport/receipt tamper controls. No model or compiler executes."""
import copy
import datetime as dt
from email.utils import format_datetime
import json
from pathlib import Path
import plistlib
import shutil
import tempfile
import unittest
from unittest import mock

import anthropic_adapter as adapter
import evaluation_bundle as eb
import evaluation_intake as intake
import graph_validator as gv
from test_graph_validator import positive


def put(path, value):
    path.write_bytes(eb.canonical(value))


class FixtureAPI:
    def __init__(self, bundle):
        self.bundle = bundle

    def load_bundle(self, path, require_anchor=True):
        if path != self.bundle["root"] or require_anchor is not True:
            raise ValueError("synthetic bundle path mismatch")
        return self.bundle

    def validate_model_request(self, request, bundle):
        eb.validate_model_request(request, bundle)

    def claim_request(self, bundle, request, role):
        return eb.claim_request(bundle, request, role)


class IntakeTamperControls(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        # These are only offline test fixtures. No real evaluation/signing key,
        # provider payload, verified result or production filesystem check is
        # cached. The sequential unittest class has one isolated fixed path so
        # original compiler argv and signed file inventories remain exact.
        cls.fixture_temp = tempfile.TemporaryDirectory(prefix="cherri_ai_intake_control_")
        cls.addClassCleanup(cls.fixture_temp.cleanup)
        cls.fixture_parent = Path(cls.fixture_temp.name)
        cls.key_material = None
        cls.manifest_material = None
        cls.complete_snapshot = None
        cls.complete_snapshot_digest = None

    @classmethod
    def tearDownClass(cls):
        cls.key_material = cls.manifest_material = cls.complete_snapshot = None
        cls.complete_snapshot_digest = None

    def snapshot(self):
        self.assertEqual(self.root, type(self).fixture_parent / "synthetic-bundle")
        self.assertFalse(self.root.is_symlink())
        paths = sorted(self.root.rglob("*"))
        self.assertTrue(all(not path.is_symlink() for path in paths))
        return tuple((path.relative_to(self.root).as_posix(), path.read_bytes())
                     for path in paths if path.is_file())

    @staticmethod
    def snapshot_digest(snapshot):
        return eb.digest(eb.canonical([
            {"path": name, "bytes": len(raw), "sha256": eb.digest(raw)}
            for name, raw in snapshot]))

    def restore(self, snapshot):
        self.assertIs(type(snapshot), tuple)
        self.assertEqual(self.root, type(self).fixture_parent / "synthetic-bundle")
        self.assertFalse(self.root.is_symlink())
        if self.root.exists():
            shutil.rmtree(self.root)
        self.root.mkdir()
        for name, raw in snapshot:
            relative = Path(name)
            self.assertFalse(relative.is_absolute())
            self.assertNotIn("..", relative.parts)
            self.assertIs(type(raw), bytes)
            path = self.root / relative
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_bytes(raw)
        # This independent byte comparison also rejects residual files from a
        # preceding mutation. The actual intake still rereads every original.
        self.assertEqual(self.snapshot(), snapshot)

    def fixture_time(self, request, offset=0):
        index = next(index for index, item in enumerate(self.requests)
                     if item["request_id"] == request["request_id"])
        return dt.datetime(2026, 11, 21, 20, tzinfo=dt.timezone.utc) + dt.timedelta(seconds=10*index+offset)

    def fixture_timestamp(self, request, offset=0):
        return self.fixture_time(request, offset).isoformat().replace("+00:00", "Z")

    def seal_record(self, request, record):
        with mock.patch.object(eb, "utc_now", return_value=self.fixture_timestamp(request, 3)):
            return eb.seal_outcome(self.bundle, record)

    def setUp(self):
        self.parent = type(self).fixture_parent
        self.root = self.parent / "synthetic-bundle"
        self.assertFalse(self.root.is_symlink())
        if self.root.exists():
            shutil.rmtree(self.root)
        self.root.mkdir()
        (self.root / "private").mkdir()
        suite, parameters = eb.generate_suite(bytes(range(32)))
        manifest = {"schema": eb.BUNDLE_SCHEMA, "profile": eb.PROFILE, "implementation_sha": "c"*40,
            "created_at": "2026-11-21T19:00:00Z", "provider": eb.PROVIDER,
            "producer_sha256": {name: eb.digest(Path(eb.__file__).with_name(name).read_bytes()) for name in eb.PRODUCER_FILES},
            "budget": {"max_input_tokens": 12000, "max_output_tokens": 2000}, "http_timeout_seconds": eb.HTTP_TIMEOUTS,
            "process_timeout_seconds": 120, "order_seed": 31, "suite_sha256": eb.digest(eb.canonical(suite)),
            "oracle_sha256": eb.digest(eb.canonical({t["id"]: t["oracle"] for t in suite["tasks"]})),
            "seed_commitment_sha256": eb.digest(bytes(range(32))),
            "compiler": {"source_sha": eb.COMPILER_SOURCE, "binary_sha256": eb.COMPILER_SHA256}}
        owner = type(self)
        if owner.key_material is None:
            eb.crypto("genpkey", "-algorithm", "ED25519", "-out", self.root / "private" / "signing-key.pem")
            eb.crypto("pkey", "-in", self.root / "private" / "signing-key.pem", "-pubout", "-out", self.root / "public-key.pem")
            owner.key_material = tuple((name, (self.root / name).read_bytes())
                for name in ("private/signing-key.pem", "public-key.pem"))
        else:
            for name, raw in owner.key_material:
                (self.root / name).write_bytes(raw)
        manifest["public_key_sha256"] = eb.digest((self.root / "public-key.pem").read_bytes())
        put(self.root / "manifest.json", manifest)
        if owner.manifest_material is None:
            eb.crypto("pkeyutl", "-sign", "-rawin", "-inkey", self.root / "private" / "signing-key.pem",
                      "-in", self.root / "manifest.json", "-out", self.root / "signature.bin")
            owner.manifest_material = ((self.root / "manifest.json").read_bytes(),
                                       (self.root / "signature.bin").read_bytes())
        else:
            self.assertEqual((self.root / "manifest.json").read_bytes(), owner.manifest_material[0])
            (self.root / "signature.bin").write_bytes(owner.manifest_material[1])
        self.bundle = {"root": self.root, "manifest": manifest, "suite": suite, "parameters": parameters,
                       "guides": {"legacy": "synthetic legacy guide", "v2": "synthetic v2 guide"}, "anchor": {}}
        self.story = {"data": {"gid": eb.ASANA_TASK, "comments": [{"gid": "1220000000000001", "resource_subtype": "comment_added",
                      "created_at": "2026-11-21T19:01:00Z", "text": eb.anchor_text(self.bundle)}]}}
        self.bundle["anchor"] = eb.validate_anchor(self.story, self.bundle)
        self.trusted = self.parent / "synthetic-authenticated-response-control.json"
        put(self.trusted, self.story)
        self.api = FixtureAPI(self.bundle)
        self.requests = eb.request_sequence(self.bundle)
        self.no_network = mock.patch.object(adapter.http.client, "HTTPSConnection", side_effect=AssertionError("unmocked network forbidden"))
        self.no_network.start()
        self.addCleanup(self.no_network.stop)

    def provider(self, request, stop_reason="end_turn", *, content=None, partial_generation=False):
        code = 'show("synthetic control source")\n'
        counter = []
        def transport(path, raw, credential, timeout):
            counter.append(path)
            if path == adapter.COUNT_PATH:
                response = {"input_tokens": 18}
                request_id = "req_count_"+request["request_id"]
            else:
                response = {"id": "msg_"+request["request_id"], "model": eb.MODEL,
                    "type": "message", "role": "assistant", "stop_reason": stop_reason, "stop_sequence": None,
                    "content": [{"type": "text", "text": code}] if content is None else content,
                    "usage": {"input_tokens": 17, "output_tokens": 9, "cache_creation_input_tokens": 0, "cache_read_input_tokens": 0}}
                request_id = "req_generation_"+request["request_id"]
            if partial_generation and path == adapter.MESSAGE_PATH:
                return adapter.HTTPResult(200, request_id, b'{"type":"message",', False,
                                          format_datetime(self.fixture_time(request), usegmt=True))
            return adapter.HTTPResult(200, request_id, eb.canonical(response), True,
                                      format_datetime(self.fixture_time(request), usegmt=True))
        with mock.patch.object(adapter, "utc_now", return_value=self.fixture_timestamp(request)), \
                mock.patch.object(eb, "utc_now", return_value=self.fixture_timestamp(request)):
            reply, status = adapter.run_adapter(request, eb.canonical(request),
                {"ANTHROPIC_API_KEY": "mock_credential_for_local_test_only", "CHERRI_AI_EVAL_ALLOW_GENERATION": "1",
                 "CHERRI_AI_EVAL_BUNDLE": str(self.root)}, transport=transport, bundle_api=self.api)
        self.assertEqual(status, 0 if stop_reason == "end_turn" and not partial_generation else 1, reply)
        self.assertEqual(counter, [adapter.COUNT_PATH, adapter.MESSAGE_PATH])
        return reply

    def validator(self, request, reply):
        task = next(t for t in self.bundle["suite"]["tasks"] if t["id"] == request["task_id"])
        with mock.patch.object(eb, "utc_now", return_value=self.fixture_timestamp(request, 1)):
            directory = eb.claim_request(self.bundle, request, "validator")
        code = reply["code"].encode()
        validated = {key: request[key] for key in ("schema", "request_id", "task_id", "arm")}
        validated.update(source=reply["code"], source_sha256=eb.digest(code), oracle=task["oracle"],
                         required_checks=["compiles"]+["oracle."+key for key in sorted(task["oracle"])])
        put(directory / "request.json", validated)
        (directory / "source.cherri").write_bytes(code)
        graph = positive(task, self.bundle["parameters"].get(task["id"], {}), request["arm"])
        raw = plistlib.dumps(graph, fmt=plistlib.FMT_BINARY)
        (directory / "output.shortcut").write_bytes(raw)
        (directory / "compiler.stdout").write_bytes(b"synthetic subprocess control; compiler not executed\n")
        (directory / "compiler.stderr").write_bytes(b"")
        checks = gv.semantic_checks(graph, task, self.bundle["parameters"].get(task["id"], {}))
        answer = {key: validated[key] for key in ("schema", "request_id", "task_id", "arm", "source_sha256")}
        answer.update(synthetic=False, checks=checks)
        put(directory / "reply.json", answer)
        now = self.fixture_timestamp(request, 1)
        receipt = {"schema": "cherri-ai-graph-validation/v1", "request_id": request["request_id"],
            "task_id": task["id"], "arm": request["arm"], "source_sha256": eb.digest(code),
            "compiler": self.bundle["manifest"]["compiler"], "started_at": now, "finished_at": now,
            "compiler_environment_profile": eb.reviewed_profile(self.bundle["manifest"]["profile"])["compiler_environment_profile"],
            "synthetic": False, "apple_runtime_proved": False, "semantic_scope": "six-task-emitted-pure-graph",
            "command": [str(self.root / "compiler.exe"), "compile", str(directory / "source.cherri"), "--mode="+request["arm"],
                        "--output", str(directory / "output.shortcut"), "--skip-sign"],
            "compile_returncode": 0, "compile_elapsed_seconds": 0.001, "status": "ADJUDICATED",
            "graph_sha256": eb.digest(raw), "reply_sha256": eb.digest(eb.canonical(answer)),
            "originals": {p.name: {"sha256": eb.digest(p.read_bytes()), "bytes": p.stat().st_size} for p in directory.iterdir()}}
        if self.bundle["manifest"]["profile"] == eb.V2_PROFILE:
            receipt["evaluation_profile"] = eb.V2_PROFILE
        put(directory / "validator-receipt.json", receipt)
        return answer

    def outcome(self, request, *, refusal=False, unfinished=False):
        if unfinished:
            self.provider(request, partial_generation=True)
            path = self.root / "attempts" / request["request_id"] / "provider" / "provider-receipt.json"
            receipt = eb.unique_json(path.read_bytes())
            receipt.update(status="GENERATION_STARTED", finished_at=None)
            put(path, receipt)
            record = {"task_id": request["task_id"], "arm": request["arm"], "request_id": request["request_id"],
                      "status": "PROVIDER_ERROR", "finished_at": self.fixture_timestamp(request, 2)}
        elif refusal:
            self.provider(request, "refusal")
            provider = intake.verify_provider(self.root / "attempts" / request["request_id"] / "provider", request, self.bundle)
            record = {"task_id": request["task_id"], "arm": request["arm"], "request_id": request["request_id"],
                "status": "MEASURED_PROVIDER_FAILURE", "provider_status": "REFUSED", "usage": provider["usage"], "passed": False,
                "provider_request_id_sha256": eb.digest(provider["provider_request_id"].encode()),
                "finished_at": self.fixture_timestamp(request, 2), "source_sha256": provider["source_sha256"]}
        else:
            reply = self.provider(request)
            validation = self.validator(request, reply)
            record = {"task_id": request["task_id"], "arm": request["arm"], "request_id": request["request_id"],
                "status": "MEASURED", "source_sha256": eb.digest(reply["code"].encode()), "usage": reply["usage"],
                "provider_request_id_sha256": eb.digest(reply["provider_request_id"].encode()),
                "checks": validation["checks"], "passed": True, "finished_at": self.fixture_timestamp(request, 2)}
        record.update(self.seal_record(request, record))
        return record

    def write_report(self, records, *, unfinished=False):
        report = {"schema": eb.SCHEMA, "mode": "external", "model": eb.MODEL,
                  "manifest_sha256": eb.digest(eb.canonical(self.bundle["manifest"])),
                  "suite_sha256": self.bundle["manifest"]["suite_sha256"], "budget": self.bundle["manifest"]["budget"],
                  "seed": self.bundle["manifest"]["order_seed"], "records": records,
                  "status": "EVALUATION_INCOMPLETE" if unfinished else "EVALUATION_COMPLETED",
                  "ai_quality_measured": not unfinished}
        if not unfinished:
            report["measured_results"] = {arm: {"passed": sum(r["passed"] for r in records if r["arm"] == arm), "total": 6} for arm in ("legacy", "v2")}
        put(self.root / "report.json", report)
        return report

    def complete(self, first_refusal=False, first_unfinished=False):
        self.assertFalse(first_refusal and first_unfinished)
        owner = type(self)
        if owner.complete_snapshot is None:
            # Construct all twelve originals and their actual Ed25519 seals once
            # per isolated class, even when a variant is the first selected test.
            self.write_report([self.outcome(request) for request in self.requests])
            owner.complete_snapshot = self.snapshot()
            owner.complete_snapshot_digest = self.snapshot_digest(owner.complete_snapshot)
        self.assertEqual(self.snapshot_digest(owner.complete_snapshot), owner.complete_snapshot_digest)
        self.restore(owner.complete_snapshot)
        report = eb.unique_json((self.root / "report.json").read_bytes())
        self.assertEqual([record["request_id"] for record in report["records"]],
                         [request["request_id"] for request in self.requests])
        if first_refusal or first_unfinished:
            request = self.requests[0]
            identity = request["request_id"]
            # Only synthetic originals in this freshly restored test fixture
            # are replaced. Production never deletes or retries an attempt.
            shutil.rmtree(self.root / "attempts" / identity)
            for suffix in (".json", ".sig"):
                (self.root / "outcome-seals" / (identity+suffix)).unlink()
            report["records"][0] = self.outcome(request,
                refusal=first_refusal, unfinished=first_unfinished)
            report = self.write_report(report["records"], unfinished=first_unfinished)
        self.assertEqual(self.snapshot_digest(owner.complete_snapshot), owner.complete_snapshot_digest)
        return report

    def run_intake(self):
        def git_bytes(command, **kwargs):
            name = command[-1].rsplit("/", 1)[1]
            return Path(eb.__file__).with_name(name).read_bytes()
        with mock.patch.object(intake, "load_bundle", return_value=self.bundle), \
                mock.patch.object(intake.subprocess, "check_output", side_effect=git_bytes):
            return intake.intake(self.root, self.trusted)

    def test_mock_provider_and_graph_receipts_join_all_twelve_outcomes(self):
        self.complete()
        # Deliberately dirty the restored working copy. Its immutable byte
        # snapshot must survive, and a fresh restore must remove both mutations
        # before the unchanged full twelve-row intake reads the originals.
        owner = type(self)
        before = owner.complete_snapshot_digest
        first = self.root / "attempts" / self.requests[0]["request_id"] / "provider" / "provider-receipt.json"
        first.write_bytes(b"SYNTHETIC_MUTATION_MUST_NOT_LEAK_TO_ANOTHER_CONTROL")
        unexpected = self.root / "unexpected-from-prior-test"
        unexpected.write_bytes(b"SYNTHETIC_EXTRA_FILE")
        self.restore(owner.complete_snapshot)
        self.assertFalse(unexpected.exists())
        self.assertEqual(self.snapshot_digest(owner.complete_snapshot), before)
        result = self.run_intake()
        self.assertEqual(len(result["records"]), 12)
        self.assertEqual(result["status"], "VERIFIED_MEASUREMENT")
        self.assertFalse(result["formal_acceptance_granted"])
        self.assertFalse(result["apple_runtime_proved"])
        self.assertFalse(result["unseen_pattern_provenance_proved"])
        starts, finishes = [], []
        for request in self.requests:
            directory = self.root / "attempts" / request["request_id"]
            provider = eb.unique_json((directory / "provider" / "provider-receipt.json").read_bytes())
            validator = eb.unique_json((directory / "validator" / "validator-receipt.json").read_bytes())
            starts.append(eb.utc_time(provider["started_at"]))
            finishes.extend(eb.utc_time(value) for value in (provider["finished_at"], validator["finished_at"]))
        self.assertEqual(result["execution_interval"], {
            "started_utc": min(starts).isoformat().replace("+00:00", "Z"),
            "finished_utc": max(finishes).isoformat().replace("+00:00", "Z")})

    def test_unfinished_provider_has_no_invented_interval_end_or_quality_measurement(self):
        self.complete(first_unfinished=True)
        result = self.run_intake()
        self.assertEqual(result["status"], "VERIFIED_INCOMPLETE_OUTCOMES")
        self.assertFalse(result["ai_quality_measured"])
        self.assertIsNone(result["execution_interval"]["finished_utc"])
        self.assertIsNotNone(eb.utc_time(result["execution_interval"]["started_utc"]))
        self.assertEqual(len(result["records"]), 12)
        self.assertNotIn("measured_results", result)

    def test_provider_identity_usage_public_context_and_chronology_are_recomputed(self):
        request = self.requests[0]
        self.provider(request)
        directory = self.root / "attempts" / request["request_id"] / "provider"
        self.assertEqual(intake.verify_provider(directory, request, self.bundle)["status"], "GENERATED")
        path = directory / "provider-receipt.json"
        original = json.loads(path.read_bytes())
        mutations = [lambda x: x.update(actual_model="other"),
                     lambda x: x["usage_counts"].update(input_tokens=18),
                     lambda x: x["calls"]["generation"].update(started_at="2026-11-21T18:00:00Z"),
                     lambda x: x["calls"]["generation"].update(response_date="Sat, 21 Nov 2026 18:00:00 GMT")]
        for change in mutations:
            value = copy.deepcopy(original)
            change(value)
            put(path, value)
            with self.assertRaises(eb.BundleError):
                intake.verify_provider(directory, request, self.bundle)
        put(path, original)
        payload_path = directory / "generation.request.original.json"
        payload = json.loads(payload_path.read_bytes())
        public = json.loads(payload["messages"][0]["content"])
        public["oracle"] = {"forbidden": True}
        payload["messages"][0]["content"] = json.dumps(public)
        raw = eb.canonical(payload)
        payload_path.write_bytes(raw)
        original["originals"][payload_path.name] = {"sha256": eb.digest(raw), "bytes": len(raw), "complete": True}
        original["calls"]["generation"]["request_sha256"] = eb.digest(raw)
        put(path, original)
        with self.assertRaisesRegex(eb.BundleError, "PROVIDER_ORACLE_OR_PRIVATE_INPUT_LEAK"):
            intake.verify_provider(directory, request, self.bundle)

    def test_hash_reconciled_graph_change_fails_independent_oracle(self):
        request = next(r for r in self.requests if r["task_id"] == eb.TASK_IDS[0])
        reply = self.provider(request)
        self.validator(request, reply)
        parent = self.root / "attempts" / request["request_id"]
        provider = intake.verify_provider(parent / "provider", request, self.bundle)
        self.assertEqual(intake.verify_validator(parent / "validator", request, provider, self.bundle)["status"], "MEASURED")
        directory = parent / "validator"
        graph = plistlib.loads((directory / "output.shortcut").read_bytes())
        graph["WFWorkflowActions"][-1]["WFWorkflowActionParameters"]["Text"] = "wrong literal"
        raw = plistlib.dumps(graph, fmt=plistlib.FMT_BINARY)
        (directory / "output.shortcut").write_bytes(raw)
        receipt = json.loads((directory / "validator-receipt.json").read_bytes())
        receipt["graph_sha256"] = eb.digest(raw)
        receipt["originals"]["output.shortcut"] = {"sha256": eb.digest(raw), "bytes": len(raw)}
        put(directory / "validator-receipt.json", receipt)
        with self.assertRaisesRegex(eb.BundleError, "VALIDATOR_CHECKS_NOT_RECOMPUTED"):
            intake.verify_validator(directory, request, provider, self.bundle)

    def test_validator_original_inventory_and_numeric_receipt_types_are_required(self):
        request = self.requests[0]
        reply = self.provider(request)
        self.validator(request, reply)
        parent = self.root / "attempts" / request["request_id"]
        provider = intake.verify_provider(parent / "provider", request, self.bundle)
        path = parent / "validator" / "validator-receipt.json"
        original = eb.unique_json(path.read_bytes())
        for fields, expected in (
                ({"originals_complete": False}, "VALIDATOR_ORIGINALS_INCOMPLETE"),
                ({"compile_returncode": False}, "VALIDATOR_STATUS"),
                ({"compile_elapsed_seconds": True}, "VALIDATOR_COMMAND_OR_BUDGET")):
            with self.subTest(fields=fields):
                put(path, {**original, **fields})
                with self.assertRaisesRegex(eb.BundleError, expected):
                    intake.verify_validator(parent / "validator", request, provider, self.bundle)
        overflowing = eb.canonical({**original, "compile_elapsed_seconds": "CONTROL_OVERFLOW"})
        path.write_bytes(overflowing.replace(b'"CONTROL_OVERFLOW"', b"1e999"))
        with self.assertRaisesRegex(eb.BundleError, "VALIDATOR_COMMAND_OR_BUDGET"):
            intake.verify_validator(parent / "validator", request, provider, self.bundle)
        put(path, original)
        self.assertEqual(intake.verify_validator(parent / "validator", request, provider, self.bundle)["status"], "MEASURED")

    def test_signed_original_inventory_rejects_reconciled_receipt_tampering(self):
        request = self.requests[0]
        self.provider(request)
        record = {"request_id": request["request_id"], "status": "PROVIDER_ERROR", "finished_at": eb.utc_now()}
        record.update(eb.seal_outcome(self.bundle, record))
        eb.verify_outcome_seal(self.bundle, record)
        directory = self.root / "attempts" / request["request_id"] / "provider"
        source = directory / "generated-source.original.cherri"
        source.write_text("modified source")
        receipt = json.loads((directory / "provider-receipt.json").read_bytes())
        receipt["originals"][source.name] = {"sha256": eb.digest(source.read_bytes()), "bytes": source.stat().st_size, "complete": True}
        receipt["source_sha256"] = eb.digest(source.read_bytes())
        put(directory / "provider-receipt.json", receipt)
        with self.assertRaisesRegex(eb.BundleError, "SEALED_OUTCOME_ORIGINAL_CHANGED"):
            eb.verify_outcome_seal(self.bundle, record)

    def test_changed_signed_record_and_repeated_outcome_seal_reject(self):
        request = self.requests[0]
        self.provider(request)
        record = {"request_id": request["request_id"], "status": "PROVIDER_ERROR", "finished_at": eb.utc_now()}
        record.update(eb.seal_outcome(self.bundle, record))
        altered = {**record, "status": "MEASURED"}
        with self.assertRaisesRegex(eb.BundleError, "OUTCOME_RECORD_CHANGED"):
            eb.verify_outcome_seal(self.bundle, altered)
        with self.assertRaises(FileExistsError):
            eb.seal_outcome(self.bundle, record)

    def test_budget_components_reject_booleans_extra_tokens_and_hidden_tools(self):
        valid = {"input_tokens": 3, "output_tokens": 2, "cache_creation_input_tokens": 1, "cache_read_input_tokens": 4}
        self.assertEqual(intake.usage_values(valid)["total_input_tokens"], 8)
        for changed in ({**valid, "output_tokens": True}, {**valid, "other_tokens": 1},
                        {**valid, "server_tool_use": {"web_search_requests": 1}},
                        {**valid, "output_tokens_details": {"thinking_tokens": 1}}):
            with self.assertRaises(eb.BundleError):
                intake.usage_values(changed)

    def test_complete_refusal_and_truncation_are_observed_failures_without_validator_claims(self):
        for request, stop in zip(self.requests, ("refusal", "max_tokens")):
            self.provider(request, stop)
            directory = self.root / "attempts" / request["request_id"]
            outcome = intake.verify_provider(directory / "provider", request, self.bundle)
            self.assertEqual(outcome["status"], "MEASURED_PROVIDER_FAILURE")
            self.assertFalse(outcome["passed"])
            self.assertEqual(outcome["usage"], {"input_tokens": 17, "output_tokens": 9})
            self.assertNotIn("checks", outcome)
            self.assertFalse((directory / "validator").exists())

    def test_one_complete_refusal_keeps_six_per_arm_denominator_and_zero_task_success(self):
        self.complete(first_refusal=True)
        result = self.run_intake()
        self.assertEqual(result["status"], "VERIFIED_MEASUREMENT")
        self.assertEqual(sum(r["passed"] for r in result["records"]), 11)
        self.assertEqual(sum(r["status"] == "MEASURED_PROVIDER_FAILURE" for r in result["records"]), 1)
        self.assertTrue(all(value["total"] == 6 for value in result["measured_results"].values()))
        failure = next(r for r in result["records"] if r["status"] == "MEASURED_PROVIDER_FAILURE")
        self.assertNotIn("checks", failure)

    def test_partial_adapter_response_retains_a_hash_bound_provider_failure(self):
        request = self.requests[0]
        self.provider(request, partial_generation=True)
        directory = self.root / "attempts" / request["request_id"] / "provider"
        receipt = eb.unique_json((directory / "provider-receipt.json").read_bytes())
        self.assertFalse(receipt["originals"]["generation.response.original.body"]["complete"])
        self.assertEqual(intake.verify_provider(directory, request, self.bundle)["status"], "PROVIDER_ERROR")
        invalid = {**receipt, "finished_at": "2026-11-21T18:00:00Z"}
        put(directory / "provider-receipt.json", invalid)
        with self.assertRaisesRegex(eb.BundleError, "RECEIPT_CHRONOLOGY"):
            intake.verify_provider(directory, request, self.bundle)
        put(directory / "provider-receipt.json", receipt)
        (directory / "generation.response.original.body").write_bytes(b"changed partial bytes")
        with self.assertRaisesRegex(eb.BundleError, "ORIGINAL_HASH_OR_SIZE"):
            intake.verify_provider(directory, request, self.bundle)

    def test_refusal_and_truncation_with_forbidden_blocks_are_not_measured(self):
        for request, (stop, block) in zip(self.requests, (
                ("refusal", {"type": "thinking", "thinking": "not permitted"}),
                ("max_tokens", {"type": "tool_use", "id": "synthetic", "name": "forbidden", "input": {}}))):
            self.provider(request, stop, content=[block])
            directory = self.root / "attempts" / request["request_id"] / "provider"
            with self.assertRaisesRegex(eb.BundleError, "PROVIDER_UNADJUDICABLE_CONTENT_BLOCK"):
                intake.verify_provider(directory, request, self.bundle)

    def use_v2_intake_control(self):
        pin = mock.patch.object(eb, "V2_COMPILER_SHA256", "f"*64)
        pin.start()
        self.addCleanup(pin.stop)
        selected = eb.reviewed_profile(eb.V2_PROFILE, require_compiler=True)
        self.bundle["manifest"] = copy.deepcopy(self.bundle["manifest"])
        self.bundle["manifest"].update(profile=eb.V2_PROFILE, provider=selected["provider"],
                                       producer_sha256={name: eb.digest(Path(eb.__file__).with_name(name).read_bytes())
                                                        for name in selected["producer_files"]},
                                       compiler=selected["compiler"],
                                       compiler_environment_profile=selected["compiler_environment_profile"])
        put(self.root/"manifest.json", self.bundle["manifest"])
        eb.crypto("pkeyutl", "-sign", "-rawin", "-inkey", self.root/"private"/"signing-key.pem",
                  "-in", self.root/"manifest.json", "-out", self.root/"signature.bin")
        self.story["data"]["comments"][0]["text"] = eb.anchor_text(self.bundle)
        self.bundle["anchor"] = eb.validate_anchor(self.story, self.bundle)
        put(self.trusted, self.story)

    def test_v2_mock_receipts_join_twelve_outcomes_without_changing_the_denominator(self):
        self.use_v2_intake_control()
        records = [self.outcome(request) for request in self.requests]
        self.write_report(records)
        result = self.run_intake()
        self.assertEqual(result["status"], "VERIFIED_MEASUREMENT")
        self.assertEqual(result["expected_outcomes"], 12)
        self.assertEqual(len(result["records"]), 12)
        self.assertEqual(result["compiler"], self.bundle["manifest"]["compiler"])
        self.assertEqual({arm: sum(r["arm"] == arm for r in result["records"]) for arm in ("legacy", "v2")},
                         {"legacy": 6, "v2": 6})

    def test_v2_intake_rejects_hash_reconciled_wire_drift(self):
        self.use_v2_intake_control()
        request = self.requests[0]
        self.provider(request)
        directory = self.root/"attempts"/request["request_id"]/"provider"
        receipt_path = directory/"provider-receipt.json"
        baseline_receipt = eb.unique_json(receipt_path.read_bytes())
        wire_path = directory/"generation.request.original.json"
        baseline_wire = eb.unique_json(wire_path.read_bytes())
        mutations = [
            ("disabled", lambda b: b.update(thinking={"type": "disabled"})),
            ("adaptive", lambda b: b.update(thinking={"type": "adaptive"})),
            ("tools", lambda b: b.update(tools=[])),
            ("model", lambda b: b.update(model="other-model")),
            ("effort", lambda b: b["output_config"].update(effort="max")),
            ("missing effort", lambda b: b.pop("output_config")),
            ("output cap", lambda b: b.update(max_tokens=2001)),
            ("stream", lambda b: b.update(stream=True))]
        for name, change in mutations:
            with self.subTest(mutation=name):
                wire = copy.deepcopy(baseline_wire)
                change(wire)
                raw = eb.canonical(wire)
                wire_path.write_bytes(raw)
                receipt = copy.deepcopy(baseline_receipt)
                receipt["calls"]["generation"]["request_sha256"] = eb.digest(raw)
                receipt["originals"][wire_path.name].update(sha256=eb.digest(raw), bytes=len(raw))
                put(receipt_path, receipt)
                with self.assertRaisesRegex(eb.BundleError, "PROVIDER_HTTP_FIELDS|PROVIDER_OUTPUT_CAP"):
                    intake.verify_provider(directory, request, self.bundle)

    def test_v1_receipt_cannot_claim_v2_and_v1_seals_cannot_be_restamped(self):
        request = self.requests[0]
        record = self.outcome(request)
        directory = self.root/"attempts"/request["request_id"]/"provider"
        path = directory/"provider-receipt.json"
        receipt = eb.unique_json(path.read_bytes())
        receipt["evaluation_profile"] = eb.V2_PROFILE
        put(path, receipt)
        with self.assertRaisesRegex(eb.BundleError, "PROVIDER_PROFILE"):
            intake.verify_provider(directory, request, self.bundle)
        self.use_v2_intake_control()
        with self.assertRaisesRegex(eb.BundleError, "OUTCOME_SEAL_IDENTITY"):
            eb.verify_outcome_seal(self.bundle, record)

    def test_v2_provider_and_validator_require_explicit_profile_receipts(self):
        self.use_v2_intake_control()
        request = self.requests[0]
        reply = self.provider(request)
        provider_dir = self.root/"attempts"/request["request_id"]/"provider"
        provider = intake.verify_provider(provider_dir, request, self.bundle)
        provider_path = provider_dir/"provider-receipt.json"
        provider_receipt = eb.unique_json(provider_path.read_bytes())
        provider_receipt.pop("evaluation_profile")
        put(provider_path, provider_receipt)
        with self.assertRaisesRegex(eb.BundleError, "PROVIDER_PROFILE"):
            intake.verify_provider(provider_dir, request, self.bundle)
        self.validator(request, reply)
        directory = self.root/"attempts"/request["request_id"]/"validator"
        path = directory/"validator-receipt.json"
        receipt = eb.unique_json(path.read_bytes())
        receipt.pop("evaluation_profile")
        put(path, receipt)
        with self.assertRaisesRegex(eb.BundleError, "VALIDATOR_PROFILE"):
            intake.verify_validator(directory, request, provider, self.bundle)

    def test_v2_canonical_intake_requires_the_reviewed_protocol_git_bytes(self):
        self.use_v2_intake_control()
        def changed_protocol_git(command, **kwargs):
            name = command[-1].rsplit("/", 1)[1]
            raw = Path(eb.__file__).with_name(name).read_bytes()
            return raw + b"synthetic Git document drift\n" if name == "fresh-instance-protocol-v2.md" else raw
        with mock.patch.object(intake, "load_bundle", return_value=self.bundle), \
             mock.patch.object(intake.subprocess, "check_output", side_effect=changed_protocol_git):
            with self.assertRaisesRegex(eb.BundleError, "FROZEN_GIT_SOURCE_BINDING"):
                intake.intake(self.root, self.trusted)




if __name__ == "__main__":
    unittest.main()
