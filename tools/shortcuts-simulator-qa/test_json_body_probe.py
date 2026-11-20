import base64
import copy
import http.client
import json
import os
from pathlib import Path
import re
import subprocess
import tempfile
import threading
import unittest

import json_body_probe as probe

REPO = Path(__file__).resolve().parents[2]
SOURCE = "a" * 40
RUN = "123456"


class JSONBodyProbeTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)

    def case(self, name="control"):
        fixture = "qa-json-body-" + name + "-v2"
        nonce = fixture + "-" + RUN + "-" + "b" * 32
        return fixture, nonce, probe.contract(REPO, fixture)

    def start(self, name="control"):
        fixture, nonce, value = self.case(name)
        root = self.root / (name + "-" + str(len(list(self.root.iterdir()))))
        server = probe.BodyServer(root, REPO, fixture, nonce, SOURCE, lifetime=5)
        thread = threading.Thread(target=server.serve)
        thread.start()
        endpoint = probe.ready_input(root, fixture, nonce, SOURCE, timeout=2)
        self.assertEqual(endpoint, str(server.server.server_port) + "/" + nonce)
        def stop():
            server.stop()
            thread.join(3)
            self.assertFalse(thread.is_alive())
        self.addCleanup(stop)
        return root, server, thread, fixture, nonce, value

    def request(self, server, nonce, case, data, method="POST", path=None, headers=None):
        connection = http.client.HTTPConnection("127.0.0.1", server.server.server_port, timeout=2)
        try:
            connection.request(method, path or ("/" + nonce + "/" + case), body=data,
                               headers=headers or {"Content-Type": "application/json"})
            response = connection.getresponse()
            return response.status, response.read()
        finally:
            connection.close()

    def transaction(self, name="control", candidate="ABSENT"):
        root, server, thread, fixture, nonce, value = self.start(name)
        body = copy.deepcopy(value["expected_control_json"])
        if name != "control" and candidate != "ABSENT":
            body["candidate_null"] = candidate
        raw = json.dumps(body, ensure_ascii=False, separators=(",", ":")).encode()
        self.assertEqual(self.request(server, nonce, name, raw), (200, probe.BODY))
        self.assertEqual(self.request(server, nonce, name, None, method="GET", path="/" + nonce + "/continued"),
                         (200, probe.CONTINUED))
        server.stop()
        thread.join(3)
        self.assertFalse(thread.is_alive())
        receipt = probe.read_json((root / "runtime/http-probe-receipt.json").read_bytes())
        probe.validate_receipt(receipt, value, fixture, nonce, SOURCE)
        return root, fixture, nonce, value, receipt, raw

    def test_fixed_sources_are_pinned_and_diagnostic_only(self):
        for fixture in probe.FIXTURES:
            source = (REPO / "tools/shortcuts-simulator-qa/fixtures" / (fixture + ".cherri")).read_bytes()
            report = probe.verify_source(REPO, fixture, source)
            self.assertFalse(report["formalAcceptanceGranted"])
            self.assertEqual(report["candidateNullSemantics"], "NOT_ASSUMED")
            with self.assertRaisesRegex(ValueError, "SOURCE_BYTES_DIFFER"):
                probe.verify_source(REPO, fixture, source + b"\n")
        self.assertIsNone(probe.verify_source(REPO, "private-same-name", b"private"))
        with self.assertRaises(ValueError):
            probe.identity("private-same-name", "nonce", SOURCE)

    def test_actual_loopback_post_preserves_body_bytes_and_separate_continuation(self):
        _, fixture, nonce, value, receipt, raw = self.transaction()
        self.assertEqual([x["method"] for x in receipt["events"]], ["POST", "GET"])
        observed = receipt["events"][0]["requestBody"]
        self.assertEqual(base64.b64decode(observed["publicBodyBase64"]), raw)
        self.assertEqual(observed["sha256"], probe.sha(raw))
        self.assertTrue(observed["controlsExact"])
        self.assertTrue(observed["missingKeyAbsent"])
        self.assertEqual(observed["candidateKind"], "NOT_AUTHORED")
        self.assertEqual(probe.observation_status(receipt, True), ("CONTROLLED_BODY_OBSERVED", True))
        with self.assertRaisesRegex(ValueError, "BODY_RECEIPT_IDENTITY"):
            probe.validate_receipt(receipt, value, fixture, nonce, "c" * 40)

    def test_null_absent_empty_false_and_zero_are_distinct_actual_http_observations(self):
        for item, kind, null in [(None, "NULL", True), ("ABSENT", "ABSENT", False),
                                 ("", "EMPTY_TEXT", False), (False, "BOOLEAN_FALSE", False), (0, "NUMBER_ZERO", False)]:
            with self.subTest(kind=kind):
                _, _, _, _, receipt, _ = self.transaction("null-candidate", item)
                body = receipt["events"][0]["requestBody"]
                self.assertEqual(body["candidateKind"], kind)
                self.assertEqual(body["candidateNullObserved"], null)
                self.assertTrue(body["controlsExact"])
        # These local HTTP clients are controls, not Shortcuts execution evidence.

    def test_wrong_boolean_zero_nested_shape_or_missing_key_cannot_pass_or_publish(self):
        _, _, value = self.case("null-candidate")
        mutations = {
            "false as zero": lambda x: x.update(stream=0),
            "zero as false": lambda x: x.update(zero=False),
            "flattened matrix": lambda x: x["nested"].update(matrix=[1, 2, 3, 4]),
            "lost inner value": lambda x: x["nested"].update(matrix=[[1], [3, 4]]),
            "missing becomes null": lambda x: x.update(missing_val=None),
            "empty becomes null": lambda x: x.update(empty_val=None),
            "extra private key": lambda x: x.update(private_field="do-not-publish"),
        }
        for name, mutate in mutations.items():
            with self.subTest(name=name):
                body = copy.deepcopy(value["expected_control_json"])
                body["candidate_null"] = None
                mutate(body)
                report = probe.inspect_body(json.dumps(body).encode(), value)
                self.assertFalse(report["controlsExact"])
                self.assertFalse(report["candidateNullObserved"])
                self.assertIsNone(report["publicBodyBase64"])
                self.assertNotIn("do-not-publish", json.dumps(report))

    def test_unknown_candidate_string_and_malformed_json_are_quarantined(self):
        _, _, value = self.case("null-candidate")
        body = {**value["expected_control_json"], "candidate_null": "PRIVATE_CANDIDATE"}
        report = probe.inspect_body(json.dumps(body).encode(), value)
        self.assertEqual(report["candidateKind"], "OTHER_REDACTED")
        self.assertIsNone(report["publicBodyBase64"])
        self.assertNotIn("PRIVATE_CANDIDATE", json.dumps(report))
        for raw in (b'{"duplicate":1,"duplicate":2}', b'{"x":NaN}', b'\xff', b'not-json'):
            with self.subTest(raw=raw):
                report = probe.inspect_body(raw, value)
                self.assertFalse(report["jsonParsed"])
                self.assertFalse(report["controlsExact"])
                self.assertIsNone(report["publicBodyBase64"])

    def test_actual_foreign_route_method_content_type_and_oversized_body_reject(self):
        for label in ("foreign-route", "method", "content-type", "size"):
            with self.subTest(label=label):
                _, server, thread, _, nonce, value = self.start()
                raw = json.dumps(value["expected_control_json"]).encode()
                kwargs = {}
                if label == "foreign-route": kwargs["path"] = "/foreign-nonce/control"
                if label == "method": kwargs["method"] = "PUT"
                if label == "content-type": kwargs["headers"] = {"Content-Type": "text/plain"}
                if label == "size": raw = b"X" * (probe.MAX_BODY + 1)
                self.assertEqual(self.request(server, nonce, "control", raw, **kwargs)[0], 404)
                server.stop(); thread.join(3)
                self.assertEqual(server.receipt["rejectedRequestCount"], 1)
                self.assertEqual(server.receipt["events"], [])

    def test_stopped_server_cannot_be_reused_as_ready(self):
        root, fixture, nonce, _, _, _ = self.transaction()
        with self.assertRaisesRegex(ValueError, "SERVER_NOT_READY"):
            probe.ready_input(root, fixture, nonce, SOURCE, timeout=.01)
        with self.assertRaisesRegex(ValueError, "PRIOR_PROBE_EVIDENCE_EXISTS"):
            probe.BodyServer(root, REPO, fixture, nonce, SOURCE)

    def test_receipt_corruptions_cannot_create_an_observation(self):
        _, fixture, nonce, value, original, _ = self.transaction("null-candidate", None)
        mutations = {
            "wrong source": lambda x: x.update(compilerSha="d" * 40),
            "wrong fixture": lambda x: x.update(fixtureId="qa-json-body-control-v2"),
            "wrong nonce": lambda x: x.update(runNonce=nonce.replace("b", "c")),
            "other host": lambda x: x.update(boundHost="0.0.0.0"),
            "negative time": lambda x: x["events"][0].update(elapsedMilliseconds=-1),
            "hash mismatch": lambda x: x["events"][0]["requestBody"].update(sha256="0" * 64),
            "null relabeled": lambda x: x["events"][0]["requestBody"].update(candidateKind="EMPTY_TEXT"),
            "missing proof": lambda x: x["events"][0]["requestBody"].update(publicBodyBase64=None),
            "private extra receipt": lambda x: x.update(rawPrivate="no"),
            "body on GET": lambda x: x["events"][1].update(requestBody={}),
        }
        for name, mutate in mutations.items():
            with self.subTest(name=name):
                changed = copy.deepcopy(original); mutate(changed)
                with self.assertRaises(ValueError):
                    probe.validate_receipt(changed, value, fixture, nonce, SOURCE)
        late = copy.deepcopy(original)
        for event in late["events"]: event["elapsedMilliseconds"] = late["lifetimeSeconds"] * 1000 + 1
        probe.validate_receipt(late, value, fixture, nonce, SOURCE)
        self.assertFalse(probe.observation_status(late, True)[1])
        for mutate in (lambda x: x.update(serverState="EXPIRED"), lambda x: x.update(rejectedRequestCount=1),
                       lambda x: x["events"][0].update(responseWriteCompleted=False), lambda x: x["events"].pop()):
            changed = copy.deepcopy(original); mutate(changed)
            self.assertFalse(probe.observation_status(changed, True)[1])
        self.assertFalse(probe.observation_status(original, False)[1])

    def diagnostic_control(self):
        root, fixture, nonce, value, receipt, _ = self.transaction("null-candidate", None)
        source = (REPO / "tools/shortcuts-simulator-qa/fixtures" / (fixture + ".cherri")).read_bytes()
        manifest = {"syntheticOnly": True, "compilerCommitSha": SOURCE, "fixtures": [{"fixtureId": fixture,
            "fileName": fixture + ".cherri",
            "sourceSha256": probe.FIXTURES[fixture][1], "unsignedPlistSha256": "1" * 64,
            "jsonBodySource": probe.verify_source(REPO, fixture, source),
            "jsonBodyGraph": probe.graph_receipt(value), "jsonBodyCausalMutations": probe.mutation_receipt()}]}
        assertions = {"targetFixtureId": fixture, "expectedRunId": nonce, "compilerCommitSha": SOURCE,
            "compiledSourceSha256": probe.FIXTURES[fixture][1], "unsignedPlistSha256": "1" * 64,
            "expectedSemanticOutput": value["expected_callback_prefix"] + probe.BODY.decode(),
            "runtimeContractVerified": True, "compiledShortcutCallbackVerified": True}
        # A constructed core callback is a unit control only. It is never native evidence.
        callback = {"fixtureId": fixture, "runId": nonce, "expectedShortcutName": fixture,
            "eventSource": "shortcuts-x-callback", "host": "success", "status": "SUCCESS",
            "resultSummary": assertions["expectedSemanticOutput"] + "\nLOCAL_TEST_TRACE_CONTROL"}
        callback_bytes = json.dumps([callback]).encode()
        (root / "compiler-manifest.json").write_text(json.dumps(manifest))
        (root / "runtime/output-assertions.json").write_text(json.dumps(assertions))
        (root / "runtime/callback-events.json").write_bytes(callback_bytes)
        report = probe.diagnose(root, REPO, fixture, nonce, SOURCE)
        return root, fixture, nonce, value, receipt, report, manifest, assertions, callback, callback_bytes

    def test_diagnose_binds_original_request_callback_source_and_failure_truth(self):
        root, fixture, nonce, value, receipt, report, manifest, assertions, callback, callback_bytes = self.diagnostic_control()
        self.assertTrue(report["controlledBodyVerified"])
        self.assertTrue(report["candidateNullObserved"])
        self.assertFalse(report["formalAcceptanceGranted"])
        for field, wrong in (("compilerSha", "e" * 40), ("callbackEventsSha256", "0" * 64),
                             ("unsignedPlistSha256", "f" * 64), ("formalAcceptanceGranted", True),
                             ("candidateNullObserved", False), ("scope", "FORMAL_ACCEPTANCE")):
            with self.subTest(field=field), self.assertRaises(ValueError):
                probe.validate_public_diagnostic({**report, field: wrong}, manifest, assertions, callback_bytes, REPO)
        for field in ("jsonBodySource", "jsonBodyGraph", "jsonBodyCausalMutations"):
            with self.subTest(missing=field):
                changed_manifest = copy.deepcopy(manifest)
                changed_manifest["fixtures"][0].pop(field)
                with self.assertRaises(ValueError):
                    probe.validate_public_diagnostic(report, changed_manifest, assertions, callback_bytes, REPO)
        assertions["runtimeContractVerified"] = False
        assertions["compiledShortcutCallbackVerified"] = False
        callback.update(host="error", status="ERROR")
        callback.pop("resultSummary")
        (root / "runtime/output-assertions.json").write_text(json.dumps(assertions))
        (root / "runtime/callback-events.json").write_text(json.dumps([callback]))
        failed = probe.diagnose(root, REPO, fixture, nonce, SOURCE)
        self.assertEqual(failed["observedCallbackStatus"], "ERROR")
        self.assertFalse(failed["controlledBodyVerified"])
        self.assertFalse(failed["candidateNullObserved"])
        self.assertEqual(failed["receipt"], receipt)

    def test_callback_origin_and_exact_result_corruptions_do_not_pass(self):
        _, _, _, _, _, report, manifest, assertions, callback, _ = self.diagnostic_control()
        for field, wrong in (("fixtureId", "another"), ("runId", "stale"),
                             ("receiverSelfTest", True), ("expectedShortcutName", "another"),
                             ("eventSource", "self-test"), ("host", "error"),
                             ("status", "ERROR"), ("resultSummary", "success")):
            with self.subTest(field=field):
                changed = {**callback, field: wrong}
                raw = json.dumps([changed]).encode()
                changed_report = {**report, "callbackEventsSha256": probe.sha(raw)}
                with self.assertRaises(ValueError):
                    probe.validate_public_diagnostic(changed_report, manifest, assertions, raw, REPO)
        for callbacks in ([callback, callback], [1], [], {"not": "callbacks"}):
            raw = json.dumps(callbacks).encode()
            with self.assertRaises(ValueError):
                probe.validate_public_diagnostic({**report, "callbackEventsSha256": probe.sha(raw)},
                                                 manifest, assertions, raw, REPO)

    def test_actual_publisher_preserves_original_public_bytes_and_quarantines_raw_receipt(self):
        from prepare_public_artifacts import stage, trusted_fixture_sources
        root, _, _, _, _, _, _, _, _, _ = self.diagnostic_control()
        (root / "summary.json").write_text(json.dumps({"privacyGatePassed": True}))
        original = (root / "runtime/json-body-diagnostic.json").read_bytes()
        raw_receipt = (root / "runtime/http-probe-receipt.json").read_bytes()
        output = self.root / "published"
        publication = stage(root, output, trusted_fixture_sources(REPO / "tools/shortcuts-simulator-qa/fixtures"))
        self.assertEqual((output / "runtime/json-body-diagnostic.json").read_bytes(), original)
        self.assertEqual((root / "runtime/http-probe-receipt.json").read_bytes(), raw_receipt)
        self.assertFalse((output / "runtime/http-probe-receipt.json").exists())
        self.assertTrue(any(x["path"] == "runtime/http-probe-receipt.json" for x in publication["quarantined"]))
        self.assertTrue(all(probe.sha((output / x["path"]).read_bytes()) == x["sha256"] for x in publication["published"]))

    def test_actual_publisher_rejects_body_proof_or_callback_tampering(self):
        from prepare_public_artifacts import PublicationBlocked, stage, trusted_fixture_sources
        root, _, _, _, _, report, _, _, _, callback_bytes = self.diagnostic_control()
        (root / "summary.json").write_text(json.dumps({"privacyGatePassed": True}))
        trusted = trusted_fixture_sources(REPO / "tools/shortcuts-simulator-qa/fixtures")
        for label in ("body", "callback", "promoted", "late"):
            with self.subTest(label=label):
                changed = copy.deepcopy(report)
                (root / "runtime/callback-events.json").write_bytes(callback_bytes)
                if label == "body":
                    changed["receipt"]["events"][0]["requestBody"]["publicBodyBase64"] = base64.b64encode(b'{"private":"NEVER_PUBLIC"}').decode()
                elif label == "callback":
                    (root / "runtime/callback-events.json").write_bytes(b"[]")
                elif label == "promoted":
                    changed["formalAcceptanceGranted"] = True
                else:
                    changed["receipt"]["events"][0]["elapsedMilliseconds"] = 300000
                (root / "runtime/json-body-diagnostic.json").write_text(json.dumps(changed))
                output = self.root / ("blocked-" + label)
                with self.assertRaisesRegex(PublicationBlocked, "INVALID_BOUNDED_JSON_BODY_DIAGNOSTIC"):
                    stage(root, output, trusted)
                self.assertFalse(output.exists())

    def test_shell_helper_route_is_closed_and_original_retry_route_is_preserved(self):
        shell = (REPO / "scripts/ios27_shortcuts_simulator_qa.sh").read_text()
        function = re.search(r"(?ms)^http_probe\(\) \{\n.*?^\}", shell).group()
        # Execute the real dispatcher, replacing only python3 with an argv
        # recorder. This control never runs the simulator or a fixture.
        script = 'ROOT_DIR=/reviewed\npython3() { printf "%s\\n" "$@"; }\n' + function + '\nhttp_probe input --root /owned\n'
        bash = os.environ.get("CHERRI_TEST_BASH", "bash")
        for fixture in (*probe.FIXTURES, "qa-retry-control-v2", "qa-retry-http500-v2", "private"):
            result = subprocess.run([bash, "-c", script], capture_output=True, text=True,
                                    env={**os.environ, "TARGET_FIXTURE_ID": fixture}, timeout=10)
            if fixture == "private":
                self.assertEqual(result.returncode, 2)
                self.assertEqual(result.stdout, "")
            else:
                self.assertEqual(result.returncode, 0, result.stderr)
                selected = "json_body_probe.py" if fixture in probe.FIXTURES else "retry_http_probe.py"
                self.assertEqual(result.stdout.splitlines(), ["/reviewed/tools/shortcuts-simulator-qa/" + selected, "input", "--root", "/owned"])


if __name__ == "__main__":
    unittest.main()
