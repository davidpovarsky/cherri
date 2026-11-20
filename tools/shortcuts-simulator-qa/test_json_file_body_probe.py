"""Actual local loopback controls; no test callback is native evidence."""
import base64
import copy
import json
import os
from pathlib import Path
import subprocess
import sys
import unittest

import json_body_probe as probe
import json_file_body_probe as file_probe
import test_json_body_probe as original

REPO = Path(__file__).resolve().parents[2]
SOURCE = original.SOURCE
FIXTURE = file_probe.FIXTURE


class JSONFileBodyProbeTests(unittest.TestCase):
    setUp = original.JSONBodyProbeTests.setUp
    case = original.JSONBodyProbeTests.case
    start = original.JSONBodyProbeTests.start
    request = original.JSONBodyProbeTests.request

    def transaction(self, raw=None):
        root, server, thread, fixture, nonce, value = self.start("file-utf8")
        raw = value["expected_raw_utf8"].encode("utf-8") if raw is None else raw
        self.assertEqual(self.request(server, nonce, "file-utf8", raw), (200, probe.BODY))
        self.assertEqual(self.request(server, nonce, "file-utf8", None, method="GET",
                                     path="/" + nonce + "/continued"), (200, probe.CONTINUED))
        server.stop()
        thread.join(3)
        self.assertFalse(thread.is_alive())
        receipt = probe.read_json((root / "runtime/http-probe-receipt.json").read_bytes())
        probe.validate_receipt(receipt, value, fixture, nonce, SOURCE)
        return root, fixture, nonce, value, receipt, raw

    def diagnostic_control(self, raw=None):
        root, fixture, nonce, value, receipt, raw = self.transaction(raw)
        source = (REPO / "tools/shortcuts-simulator-qa/fixtures" / (fixture + ".cherri")).read_bytes()
        manifest = {"syntheticOnly": True, "compilerCommitSha": SOURCE, "fixtures": [{
            "fixtureId": fixture, "fileName": fixture + ".cherri",
            "sourceSha256": probe.FIXTURES[fixture][1], "unsignedPlistSha256": "1" * 64,
            "jsonBodySource": probe.verify_source(REPO, fixture, source),
            "jsonBodyGraph": probe.graph_receipt(value),
            "jsonBodyCausalMutations": probe.mutation_receipt(fixture)}]}
        assertions = {"targetFixtureId": fixture, "expectedRunId": nonce, "compilerCommitSha": SOURCE,
            "compiledSourceSha256": probe.FIXTURES[fixture][1], "unsignedPlistSha256": "1" * 64,
            "expectedSemanticOutput": value["expected_callback_prefix"] + probe.BODY.decode(),
            "runtimeContractVerified": True, "compiledShortcutCallbackVerified": True}
        # This fabricated callback is only a local validation control. Actual
        # native receipts require the existing authenticated QA acquisition.
        callback = {"fixtureId": fixture, "runId": nonce, "expectedShortcutName": fixture,
            "eventSource": "shortcuts-x-callback", "host": "success", "status": "SUCCESS",
            "resultSummary": assertions["expectedSemanticOutput"] + "\nLOCAL_TEST_TRACE_CONTROL"}
        callback_bytes = json.dumps([callback]).encode()
        (root / "compiler-manifest.json").write_bytes(json.dumps(manifest).encode())
        (root / "runtime/output-assertions.json").write_bytes(json.dumps(assertions).encode())
        (root / "runtime/callback-events.json").write_bytes(callback_bytes)
        report = probe.diagnose(root, REPO, fixture, nonce, SOURCE)
        return root, fixture, nonce, value, receipt, report, manifest, assertions, callback, callback_bytes

    def test_registered_file_body_source_and_exact_contract_are_pinned(self):
        value = probe.contract(REPO, FIXTURE)
        raw = file_probe.validate_contract(value)
        self.assertEqual(len(raw), 222)
        self.assertEqual(probe.sha(raw), "310514cc82c5926d5b2f63ff61f23e4d41967f60e83cfd6a60db058656f3bdac")
        source = (REPO / "tools/shortcuts-simulator-qa/fixtures" / (FIXTURE + ".cherri")).read_bytes()
        self.assertIn(b"let response = fileRequest(", source)
        self.assertIn(b"body: bodyText", source)
        self.assertNotIn(b"native.action(", source)
        self.assertEqual(probe.verify_source(REPO, FIXTURE, source)["appleRuntime"], "NOT_RUN")
        self.assertFalse(probe.graph_receipt(value)["formalAcceptanceGranted"])
        self.assertEqual(probe.graph_receipt(value)["nativeActionCount"], 17)
        self.assertEqual(probe.mutation_receipt(FIXTURE)["rejected"], list(file_probe.CAUSAL_CONTROLS))
        self.assertEqual(len(file_probe.CAUSAL_CONTROLS), 31)
        with self.assertRaisesRegex(ValueError, "SOURCE_BYTES_DIFFER"):
            probe.verify_source(REPO, FIXTURE, source + b"\n")

    def test_contract_rejects_boolean_number_and_literal_substitution(self):
        value = probe.contract(REPO, FIXTURE)
        for field, bad in (("authored_body_type", "JSON"), ("authored_api", "native.action"),
                           ("expected_content_type", "text/plain"),
                           ("expected_raw_utf8_sha256", "0" * 64)):
            with self.subTest(field=field), self.assertRaises(ValueError):
                file_probe.validate_contract({**value, field: bad})
        for key, replacement in (("stream", 0), ("zero", False), ("candidate_null", "")):
            with self.subTest(key=key), self.assertRaises(ValueError):
                changed = copy.deepcopy(value)
                changed["expected_json"][key] = replacement
                file_probe.validate_contract(changed)

    def test_actual_loopback_exact_utf8_has_null_missing_false_zero_and_nested_distinctions(self):
        _, _, _, _, receipt, report, _, _, _, _ = self.diagnostic_control()
        observed = receipt["events"][0]["requestBody"]
        self.assertEqual(observed["candidateKind"], "NULL")
        self.assertTrue(observed["candidateNullObserved"])
        self.assertTrue(observed["missingKeyAbsent"])
        body = probe.read_json(base64.b64decode(observed["publicBodyBase64"], validate=True))
        self.assertIsNone(body["candidate_null"])
        self.assertNotIn("missing_val", body)
        self.assertIs(body["stream"], False)
        self.assertIs(type(body["zero"]), int)
        self.assertEqual(body["zero"], 0)
        self.assertEqual(body["empty_val"], "")
        self.assertEqual(body["nested"]["matrix"], [[1, 2], [3, 4]])
        self.assertTrue(report["rawUtf8BodyVerified"])
        self.assertFalse(report["formalAcceptanceGranted"])

    def test_other_null_kinds_and_reformatted_json_preserve_observation_but_not_exact_transport(self):
        value = probe.contract(REPO, FIXTURE)
        for label, item in (("absent", "ABSENT"), ("empty", ""), ("false", False), ("zero", 0),
                            ("same_semantics_different_bytes", None)):
            with self.subTest(label=label):
                body = copy.deepcopy(value["expected_control_json"])
                if item != "ABSENT":
                    body["candidate_null"] = item
                # Deliberately different byte order/spacing for the null control.
                raw = json.dumps(body, indent=2).encode()
                _, _, _, _, _, report, _, _, _, _ = self.diagnostic_control(raw)
                self.assertTrue(report["controlledBodyVerified"])
                self.assertFalse(report["rawUtf8BodyVerified"])
                self.assertEqual(report["candidateNullObserved"], item is None)

    def test_raw_flag_cannot_override_source_callback_graph_or_bytes(self):
        _, _, _, _, _, report, manifest, assertions, callback, callback_bytes = self.diagnostic_control()
        mutations = {
            "forged raw flag": lambda r, m, a: r.update(rawUtf8BodyVerified=False),
            "missing raw flag": lambda r, m, a: r.pop("rawUtf8BodyVerified"),
            "borrowed graph": lambda r, m, a: m["fixtures"][0].update(jsonBodyGraph={"status": "PASS"}),
            "old mutation receipt": lambda r, m, a: m["fixtures"][0].update(jsonBodyCausalMutations=probe.mutation_receipt()),
            "missing mutation receipt": lambda r, m, a: m["fixtures"][0].pop("jsonBodyCausalMutations"),
            "borrowed source": lambda r, m, a: r.update(sourceSha256="0" * 64),
            "borrowed unsigned bytes": lambda r, m, a: a.update(unsignedPlistSha256="2" * 64),
            "missing callback proof": lambda r, m, a: a.update(compiledShortcutCallbackVerified=False),
            "changed raw bytes": lambda r, m, a: r["receipt"]["events"][0]["requestBody"].update(
                publicBodyBase64=base64.b64encode(b'{"candidate_null":null}').decode()),
            "promoted": lambda r, m, a: r.update(formalAcceptanceGranted=True),
        }
        for name, mutate in mutations.items():
            with self.subTest(name=name):
                r, m, a = copy.deepcopy((report, manifest, assertions))
                mutate(r, m, a)
                with self.assertRaises(ValueError):
                    probe.validate_public_diagnostic(r, m, a, callback_bytes, REPO)
        for field, wrong in (("runId", "stale"), ("eventSource", "self-test"),
                             ("expectedShortcutName", "private"), ("status", "ERROR"),
                             ("resultSummary", "unrelated success")):
            with self.subTest(callback=field):
                changed = {**callback, field: wrong}
                raw = json.dumps([changed]).encode()
                with self.assertRaises(ValueError):
                    probe.validate_public_diagnostic({**report, "callbackEventsSha256": probe.sha(raw)},
                                                     manifest, assertions, raw, REPO)

    def test_actual_failed_callback_retains_original_body_without_transport_success(self):
        root, fixture, nonce, _, receipt, _, _, assertions, callback, _ = self.diagnostic_control()
        assertions.update(runtimeContractVerified=False, compiledShortcutCallbackVerified=False)
        callback.update(status="ERROR", host="error")
        callback.pop("resultSummary")
        (root / "runtime/output-assertions.json").write_bytes(json.dumps(assertions).encode())
        (root / "runtime/callback-events.json").write_bytes(json.dumps([callback]).encode())
        report = probe.diagnose(root, REPO, fixture, nonce, SOURCE)
        self.assertFalse(report["rawUtf8BodyVerified"])
        self.assertFalse(report["controlledBodyVerified"])
        self.assertEqual(report["receipt"], receipt)
        self.assertEqual(report["observedCallbackStatus"], "ERROR")

    def test_actual_cli_requires_exact_file_bytes_without_changing_old_dictionary_goal(self):
        value = probe.contract(REPO, FIXTURE)
        for raw, expected in ((None, 0), (json.dumps(value["expected_json"], indent=2).encode(), 2)):
            with self.subTest(expected_exit=expected):
                root, fixture, nonce, _, _, _, _, _, _, _ = self.diagnostic_control(raw)
                process = subprocess.run([sys.executable, str(REPO / "tools/shortcuts-simulator-qa/json_body_probe.py"),
                    "diagnose", "--root", str(root), "--fixture", fixture, "--nonce", nonce, "--commit", SOURCE],
                    capture_output=True, text=True, timeout=10)
                self.assertEqual(process.returncode, expected, process.stdout + process.stderr)
        # The existing suite exercises the original candidate-absent observation.

    def test_actual_publisher_keeps_exact_original_bytes_and_rejects_forged_transport(self):
        from prepare_public_artifacts import PublicationBlocked, stage, trusted_fixture_sources
        root, _, _, _, _, report, _, _, _, _ = self.diagnostic_control()
        (root / "summary.json").write_bytes(b'{"privacyGatePassed":true}')
        trusted = trusted_fixture_sources(REPO / "tools/shortcuts-simulator-qa/fixtures")
        original_bytes = (root / "runtime/json-body-diagnostic.json").read_bytes()
        raw_receipt = (root / "runtime/http-probe-receipt.json").read_bytes()
        output = self.root / "published"
        publication = stage(root, output, trusted)
        self.assertEqual((output / "runtime/json-body-diagnostic.json").read_bytes(), original_bytes)
        self.assertEqual((root / "runtime/http-probe-receipt.json").read_bytes(), raw_receipt)
        self.assertFalse((output / "runtime/http-probe-receipt.json").exists())
        self.assertTrue(any(x["path"] == "runtime/http-probe-receipt.json" for x in publication["quarantined"]))
        for field, bad in (("rawUtf8BodyVerified", False), ("formalAcceptanceGranted", True)):
            with self.subTest(field=field):
                changed = {**report, field: bad}
                (root / "runtime/json-body-diagnostic.json").write_bytes(json.dumps(changed).encode())
                destination = self.root / ("blocked-" + field)
                with self.assertRaisesRegex(PublicationBlocked, "INVALID_BOUNDED_JSON_BODY_DIAGNOSTIC"):
                    stage(root, destination, trusted)
                self.assertFalse(destination.exists())


if __name__ == "__main__":
    unittest.main()
