"""Real loopback and causal evidence tests; these are not Apple runtime proof."""
import copy
import http.client
import json
from pathlib import Path
import socket
import tempfile
import threading
import unittest

import retry_http_probe as probe
from prepare_public_artifacts import PublicationBlocked, stage


HERE = Path(__file__).resolve().parent
COMMIT = "85b6c352d1e0f9f272cfc6f33c10d0becd29eee6"


class RetryHTTPProbeTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.base = Path(self.temp.name)
        self.servers = []

    def tearDown(self):
        for server, thread in self.servers:
            server.stop()
            thread.join(timeout=4)
            self.assertFalse(thread.is_alive(), "finite loopback server did not stop")
        self.temp.cleanup()

    def start(self, case="control", lifetime=30):
        fid = "qa-retry-" + case + "-v2"
        nonce = fid + "-unit-" + "1" * 32
        root = self.base / (case + "-" + str(len(self.servers)))
        server = probe.ProbeServer(root / "runtime", fid, nonce, COMMIT, lifetime)
        thread = threading.Thread(target=server.serve, daemon=True)
        self.servers.append((server, thread))
        thread.start()
        # Await the same service-loop acknowledgement used by the native runner.
        probe.ready_input(server.ready_path, fid, nonce, COMMIT, timeout=3)
        return root, server, thread, fid, nonce

    def request(self, server, path, method="GET"):
        client = http.client.HTTPConnection("127.0.0.1", server.server.server_port, timeout=3)
        try:
            client.request(method, path)
            response = client.getresponse()
            return response.status, response.read(), dict(response.getheaders())
        finally:
            client.close()

    def finish(self, server, thread):
        server.stop()
        thread.join(timeout=4)
        self.assertFalse(thread.is_alive())
        return json.loads(server.receipt_path.read_text(encoding="utf-8"))

    def seed(self, case="control", continuation=True, callback_status="SUCCESS", runtime=True):
        root, server, thread, fid, nonce = self.start(case)
        path = "/" + nonce + "/" + case
        if case == "transport":
            with self.assertRaises(http.client.RemoteDisconnected):
                self.request(server, path)
        else:
            self.request(server, path)
        if continuation:
            self.request(server, "/" + nonce + "/continued")
        self.finish(server, thread)
        fixtures = self.base / "fixtures"
        fixtures.mkdir(exist_ok=True)
        source = (HERE / "fixtures" / (fid + ".cherri")).read_bytes()
        (fixtures / (fid + ".cherri")).write_bytes(source)
        row = {"fixtureId": fid, "fileName": fid + ".cherri",
               "sourceSha256": probe.sha(source), "unsignedPlistSha256": "b" * 64,
               "buildProfile": "debug", "runtimeTraceExpectation": "REQUIRED_NDJSON",
               "httpProbeGraph": {"verified": True, "httpActionCount": 2, "method": "GET",
                                  "retryActionCount": 0, "nativeErrorCatchAdded": False}}
        manifest = {"syntheticOnly": True, "compilerCommitSha": COMMIT, "fixtures": [row]}
        assertion = {"targetFixtureId": fid, "expectedRunId": nonce, "compilerCommitSha": COMMIT,
                     "compiledSourceSha256": row["sourceSha256"], "unsignedPlistSha256": row["unsignedPlistSha256"],
                     "buildProfile": "debug", "traceExpectation": "REQUIRED_NDJSON",
                     "expectedSemanticOutput": probe.expected_marker(fid),
                     "observedCorrelatedCallbackStatus": callback_status,
                     "runtimeContractVerified": runtime, "shortcutRunCompletedGate": "PASS" if runtime else "FAIL_RUNTIME",
                     "traceCapturedGate": "PASS" if runtime else "FAIL_RUNTIME"}
        callback = {"receiverSelfTest": False, "eventSource": "shortcuts-x-callback", "runId": nonce,
                    "fixtureId": fid, "expectedShortcutName": fid, "status": callback_status,
                    "resultSummary": probe.expected_marker(fid) if callback_status == "SUCCESS" else None,
                    "errorCode": "1" if callback_status == "ERROR" else None}
        probe.atomic_json(root / "compiler-manifest.json", manifest)
        probe.atomic_json(root / "runtime/output-assertions.json", assertion)
        probe.atomic_json(root / "runtime/callback-events.json", [callback])
        return root, fixtures, fid, nonce, manifest, assertion, callback

    def diagnose(self, data):
        return probe.diagnose(*data[:4], COMMIT)

    def test_server_ready_file_requires_active_service_loop(self):
        fid = "qa-retry-control-v2"
        nonce = fid + "-ready-" + "2" * 32
        server = probe.ProbeServer(self.base / "active-ready", fid, nonce, COMMIT, lifetime=5)
        self.addCleanup(server.server.server_close)
        self.assertFalse(server.ready_path.exists(), "constructed listener is not an acknowledged running service")
        thread = threading.Thread(target=server.serve, daemon=True)
        self.servers.append((server, thread))
        thread.start()
        value = probe.ready_input(server.ready_path, fid, nonce, COMMIT, timeout=2)
        self.assertEqual(value, str(server.server.server_port) + "/" + nonce)
        self.assertEqual(self.request(server, "/" + nonce + "/control")[:2], (200, probe.BODY))
        receipt = self.finish(server, thread)
        self.assertEqual(len(probe.validate_receipt(receipt, fid, nonce, COMMIT)), 1)

    def test_server_ready_lifetime_starts_when_service_runs(self):
        import time
        fid = "qa-retry-control-v2"
        nonce = fid + "-delayed-" + "3" * 32
        server = probe.ProbeServer(self.base / "delayed-ready", fid, nonce, COMMIT, lifetime=1)
        self.addCleanup(server.server.server_close)
        # A constructor/startup delay must not publish a stale ready file for a
        # server whose entire active lifetime elapsed before serving any request.
        time.sleep(1.05)
        thread = threading.Thread(target=server.serve, daemon=True)
        self.servers.append((server, thread))
        thread.start()
        probe.ready_input(server.ready_path, fid, nonce, COMMIT, timeout=2)
        self.assertEqual(self.request(server, "/" + nonce + "/control")[:2], (200, probe.BODY))
        receipt = self.finish(server, thread)
        self.assertEqual(receipt["serverState"], "STOPPED")
        self.assertEqual(len(probe.validate_receipt(receipt, fid, nonce, COMMIT)), 1)

    def test_real_http_responses_keep_body_constant_across_statuses(self):
        for case, status in probe.CASES.items():
            if status is None:
                continue
            with self.subTest(case=case):
                root, server, thread, fid, nonce = self.start(case)
                input_text = probe.ready_input(server.ready_path, fid, nonce, COMMIT)
                self.assertEqual(input_text, str(server.server.server_port) + "/" + nonce)
                actual_status, body, headers = self.request(server, "/" + nonce + "/" + case)
                self.assertEqual(actual_status, status)
                self.assertEqual(body, b"P15_HTTP_BODY_OK")
                self.assertEqual(headers["Content-Type"], "text/plain; charset=utf-8")
                self.assertEqual(self.request(server, "/" + nonce + "/continued")[:2],
                                 (200, b"P15_HTTP_CONTINUED_OK"))
                receipt = self.finish(server, thread)
                events = probe.validate_receipt(receipt, fid, nonce, COMMIT)
                self.assertEqual([e["route"] for e in events], ["primary", "continued"])
                self.assertEqual([e["status"] for e in events], [status, 200])

    def test_transport_closes_without_sending_an_http_status(self):
        root, server, thread, fid, nonce = self.start("transport")
        with self.assertRaises(http.client.RemoteDisconnected):
            self.request(server, "/" + nonce + "/transport")
        receipt = self.finish(server, thread)
        event, = probe.validate_receipt(receipt, fid, nonce, COMMIT)
        self.assertEqual(event["responseKind"], "CONNECTION_CLOSED")
        self.assertIsNone(event["status"])
        self.assertIsNone(event["bodySha256"])
        self.assertTrue(event["responseWriteCompleted"])

    def test_stop_file_closes_socket_and_prior_evidence_is_not_replaced(self):
        root, server, thread, fid, nonce = self.start()
        (root / "runtime/http-probe.stop").touch()
        thread.join(timeout=4)
        self.assertFalse(thread.is_alive())
        receipt = json.loads(server.receipt_path.read_text(encoding="utf-8"))
        self.assertEqual(receipt["serverState"], "STOPPED")
        with socket.socket() as client:
            client.settimeout(1)
            self.assertNotEqual(client.connect_ex(("127.0.0.1", server.server.server_port)), 0)
        with self.assertRaisesRegex(probe.ProbeError, "PRIOR_PROBE"):
            probe.ProbeServer(root / "runtime", fid, nonce, COMMIT)
        self.assertEqual(json.loads(server.receipt_path.read_text(encoding="utf-8")), receipt)

    def test_finite_expiry_is_not_a_cleanly_stopped_receipt(self):
        root, server, thread, fid, nonce = self.start(lifetime=1)
        thread.join(timeout=3)
        self.assertFalse(thread.is_alive())
        receipt = json.loads(server.receipt_path.read_text(encoding="utf-8"))
        self.assertEqual(receipt["serverState"], "EXPIRED")
        with self.assertRaisesRegex(probe.ProbeError, "SERVER_NOT_STOPPED"):
            probe.validate_receipt(receipt, fid, nonce, COMMIT)

    def test_unreviewed_requests_leave_no_path_or_payload_in_receipt(self):
        for path, method in (("/wrong-nonce/private-text", "GET"), ("/unreviewed", "POST"), ("/unreviewed", "HEAD")):
            with self.subTest(method=method):
                root, server, thread, fid, nonce = self.start()
                self.assertEqual(self.request(server, path, method)[0], 404)
                receipt = self.finish(server, thread)
                self.assertEqual(receipt["rejectedRequestCount"], 1)
                self.assertEqual(receipt["events"], [])
                self.assertNotIn(path, server.receipt_path.read_text(encoding="utf-8"))
                with self.assertRaisesRegex(probe.ProbeError, "UNREVIEWED_REQUEST"):
                    probe.validate_receipt(receipt, fid, nonce, COMMIT)

    def test_ready_endpoint_rejects_wrong_fixture_nonce_host_and_port(self):
        root, server, thread, fid, nonce = self.start()
        original = json.loads(server.ready_path.read_text(encoding="utf-8"))
        for key, value in (("fixtureId", "qa-private"), ("runNonce", nonce + "0"), ("compilerSha", "0" * 40),
                           ("host", "example.invalid"), ("port", True), ("port", 0), ("port", 65536)):
            with self.subTest(key=key, value=value):
                probe.atomic_json(server.ready_path, {**original, key: value})
                with self.assertRaises(probe.ProbeError):
                    probe.ready_input(server.ready_path, fid, nonce, COMMIT, timeout=0)

    def test_request_bound_is_enforced_and_extra_request_prevents_claim(self):
        root, server, thread, fid, nonce = self.start()
        for _ in range(probe.MAX_REQUESTS):
            self.assertEqual(self.request(server, "/" + nonce + "/control")[0], 200)
        self.assertEqual(self.request(server, "/" + nonce + "/control")[0], 404)
        receipt = self.finish(server, thread)
        self.assertEqual(len(receipt["events"]), probe.MAX_REQUESTS)
        with self.assertRaises(probe.ProbeError):
            probe.validate_receipt(receipt, fid, nonce, COMMIT)

    def test_control_requires_strict_output_trace_and_continuation(self):
        data = self.seed()
        report = self.diagnose(data)
        self.assertEqual(report["diagnosticStatus"], "CONTROL_VERIFIED")
        self.assertTrue(report["ordinaryRuntimeContractVerified"])
        self.assertEqual(report["nativeCatchability"], "NOT_PROVEN")
        self.assertFalse(report["retryImplemented"])
        bad = copy.deepcopy(data[5])
        bad["runtimeContractVerified"] = False
        bad["traceCapturedGate"] = "FAIL_RUNTIME"
        probe.atomic_json(data[0] / "runtime/output-assertions.json", bad)
        report = self.diagnose(data)
        self.assertEqual(report["diagnosticStatus"], "CONTINUATION_OBSERVED_OUTPUT_UNVERIFIED")
        self.assertFalse(report["diagnosticObservationComplete"])

    def test_error_observation_never_promotes_ordinary_runtime(self):
        for case in ("http500", "http404", "http429", "transport"):
            with self.subTest(case=case):
                data = self.seed(case, continuation=False, callback_status="ERROR", runtime=False)
                report = self.diagnose(data)
                self.assertEqual(report["diagnosticStatus"], "TERMINATED_WITHOUT_CONTINUATION_RECEIPT")
                self.assertTrue(report["diagnosticObservationComplete"])
                self.assertFalse(report["ordinaryRuntimeContractVerified"])
                self.assertEqual(report["sameRequestStatusValue"], "NOT_PROVEN")
                self.assertEqual(report["numericErrorCode"], "1")
                bad = {**data[5], "runtimeContractVerified": True, "shortcutRunCompletedGate": "PASS"}
                probe.atomic_json(data[0] / "runtime/output-assertions.json", bad)
                with self.assertRaisesRegex(probe.ProbeError, "ERROR_CALLBACK_CANNOT_PASS"):
                    self.diagnose(data)
        control = self.seed("control", continuation=False, callback_status="ERROR", runtime=False)
        self.assertFalse(self.diagnose(control)["diagnosticObservationComplete"])

    def test_http_status_response_continuation_is_not_status_value_or_catchability(self):
        data = self.seed("http500")
        report = self.diagnose(data)
        self.assertEqual(report["diagnosticStatus"], "HTTP_RESPONSE_CONTINUED")
        self.assertEqual(report["events"][0]["status"], 500)
        self.assertTrue(report["ordinaryRuntimeContractVerified"])
        self.assertEqual(report["nativeCatchability"], "NOT_PROVEN")
        self.assertEqual(report["sameRequestStatusValue"], "NOT_PROVEN")

    def test_source_nonce_profile_and_callback_tampering_fail_closed(self):
        data = self.seed()
        root, fixtures, fid, nonce, manifest, assertion, callback = data
        for key, value in (("sourceSha256", "f" * 64), ("unsignedPlistSha256", "f" * 64),
                           ("buildProfile", "release"), ("runtimeTraceExpectation", "ABSENT_RELEASE"),
                           ("httpProbeGraph", {"verified": True})):
            changed = copy.deepcopy(manifest)
            changed["fixtures"][0][key] = value
            probe.atomic_json(root / "compiler-manifest.json", changed)
            with self.subTest(manifest=key), self.assertRaises(probe.ProbeError):
                self.diagnose(data)
        probe.atomic_json(root / "compiler-manifest.json", manifest)
        for key, value in (("expectedRunId", nonce + "0"), ("compilerCommitSha", "f" * 40),
                           ("compiledSourceSha256", "f" * 64), ("expectedSemanticOutput", "P15_PRIVATE")):
            probe.atomic_json(root / "runtime/output-assertions.json", {**assertion, key: value})
            with self.subTest(assertion=key), self.assertRaises(probe.ProbeError):
                self.diagnose(data)
        probe.atomic_json(root / "runtime/output-assertions.json", assertion)
        for events in ([{**callback, "runId": "stale"}], [callback, callback],
                       [callback, {**callback, "fixtureId": "qa-other"}],
                       [{**callback, "receiverSelfTest": True}], [{**callback, "eventSource": "receiver-self-test"}]):
            probe.atomic_json(root / "runtime/callback-events.json", events)
            with self.subTest(callback=events), self.assertRaises(probe.ProbeError):
                self.diagnose(data)

    def test_receipt_mutations_cannot_substitute_http_status_or_prove_continuation(self):
        data = self.seed()
        receipt_path = data[0] / "runtime/http-probe-receipt.json"
        receipt = json.loads(receipt_path.read_text(encoding="utf-8"))
        for event_key, value in (("route", "other"), ("method", "POST"), ("status", 500),
                                 ("bodySha256", "0" * 64), ("sequence", True),
                                 ("elapsedMilliseconds", -1), ("responseWriteCompleted", "true")):
            changed = copy.deepcopy(receipt)
            changed["events"][0][event_key] = value
            probe.atomic_json(receipt_path, changed)
            with self.subTest(key=event_key), self.assertRaises(probe.ProbeError):
                self.diagnose(data)
        changed = copy.deepcopy(receipt)
        changed["events"][1]["route"] = "primary"
        changed["events"][1]["bodySha256"] = probe.sha(probe.BODY)
        probe.atomic_json(receipt_path, changed)
        report = self.diagnose(data)
        self.assertEqual(report["diagnosticStatus"], "MULTIPLE_REQUESTS_OBSERVED")
        self.assertFalse(report["diagnosticObservationComplete"])

    def test_publication_allows_only_bounded_diagnostic_not_server_raw_data(self):
        data = self.seed()
        root = data[0]
        self.diagnose(data)
        probe.atomic_json(root / "summary.json", {"privacyGatePassed": True})
        (root / "runtime/http-probe-server.log").write_text("local-only server log", encoding="utf-8")
        trusted = {data[2] + ".cherri": data[4]["fixtures"][0]["sourceSha256"]}
        public = self.base / "public"
        result = stage(root, public, trusted)
        self.assertTrue(result["publicationGatePassed"])
        self.assertTrue((public / "runtime/http-probe-diagnostic.json").is_file())
        for name in ("http-probe-ready.json", "http-probe-receipt.json", "http-probe-server.log"):
            self.assertFalse((public / "runtime" / name).exists())
        report = json.loads((root / "runtime/http-probe-diagnostic.json").read_text(encoding="utf-8"))
        for key, value in (("arbitraryHeader", "unexpected data"), ("runNonce", data[3][:-1] + "2"),
                           ("sourceSha256", "f" * 64), ("diagnosticStatus", "HTTP_RESPONSE_CONTINUED")):
            changed = {**report, key: value}
            probe.atomic_json(root / "runtime/http-probe-diagnostic.json", changed)
            with self.subTest(key=key), self.assertRaises(PublicationBlocked):
                stage(root, self.base / ("rejected-" + key), trusted)


class RetryNativeGraphTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.specimen = json.loads((HERE / "testdata/retry-control-compiled-graph.json").read_text(encoding="utf-8"))

    def test_actual_compiled_graph_has_one_primary_and_one_observer(self):
        specimen = self.specimen
        self.assertEqual(specimen["scope"], "ACTUAL_CHERRI_COMPILED_GRAPH_NOT_APPLE_RUNTIME_PROOF")
        self.assertEqual(specimen["sourceSha256"], probe.sha((HERE / "fixtures/qa-retry-control-v2.cherri").read_bytes()))
        graph = probe.validate_fixture_graph(specimen["fixtureId"], specimen["actions"])
        self.assertEqual((graph["primaryIndex"], graph["continuationIndex"], graph["summaryIndex"]), (5, 6, 7))
        self.assertEqual(graph["retryActionCount"], 0)
        self.assertIsNone(graph["continuationUUIDDigest"], "unused observer does not have a fabricated native UUID")

    def test_native_http_mutations_are_rejected(self):
        original = self.specimen["actions"]
        mutations = []
        def mutate(label, fn):
            actions = copy.deepcopy(original)
            fn(actions)
            mutations.append((label, actions))
        mutate("duplicate request", lambda a: a.append(copy.deepcopy(a[5])))
        mutate("POST", lambda a: a[5]["WFWorkflowActionParameters"].update(WFHTTPMethod="POST"))
        mutate("headers", lambda a: a[5]["WFWorkflowActionParameters"].update(WFHTTPHeaders={"synthetic-header": "unexpected"}))
        mutate("unknown HTTP parameter", lambda a: a[5]["WFWorkflowActionParameters"].update(WFUnexpectedOption=True))
        mutate("external URL", lambda a: a[5]["WFWorkflowActionParameters"]["WFURL"]["Value"].update(string="http://example.invalid/\ufffc/control"))
        mutate("wrong observer route", lambda a: a[6]["WFWorkflowActionParameters"]["WFURL"]["Value"].update(string="http://127.0.0.1:\ufffc/control"))
        mutate("wrong endpoint attachment", lambda a: a[5]["WFWorkflowActionParameters"]["WFURL"]["Value"].update(attachmentsByRange={"{17, 1}": {"Type": "Variable", "VariableName": "private"}}))
        mutate("wrong input range", lambda a: a[5]["WFWorkflowActionParameters"]["WFURL"]["Value"].update(attachmentsByRange={"{18, 1}": {"Type": "ExtensionInput"}}))
        mutate("missing producer", lambda a: a[5]["WFWorkflowActionParameters"].pop("UUID"))
        mutate("wrong response binding", lambda a: next(iter(a[7]["WFWorkflowActionParameters"]["WFTextActionText"]["Value"]["attachmentsByRange"].values())).update(OutputUUID="wrong-uuid"))
        mutate("summary before observer", lambda a: a.__setitem__(slice(6, 8), [a[7], a[6]]))
        mutate("retry control flow", lambda a: a.append({"WFWorkflowActionIdentifier": "is.workflow.actions.repeat.count"}))
        for label, actions in mutations:
            with self.subTest(label=label), self.assertRaises(probe.ProbeError):
                probe.validate_fixture_graph("qa-retry-control-v2", actions)


if __name__ == "__main__":
    unittest.main()
