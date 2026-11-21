#!/usr/bin/env python3
"""Offline contract controls. No provider/counting request is made by this suite."""
import copy
import hashlib
import io
import json
from pathlib import Path
import tempfile
import types
import unittest
from unittest import mock

import anthropic_adapter as adapter
from runner import model_request


KEY = "mock_credential_for_offline_controls_only"
CODE = '  let title = "שלום 🍒"\nshow(title)\n\n'


def body(value):
    return json.dumps(value, ensure_ascii=False).encode("utf-8")


def message(code=CODE):
    return {
        "type": "message", "role": "assistant", "id": "msg_offline_control",
        "model": "claude-sonnet-5-5", "content": [{"type": "text", "text": code}],
        "stop_reason": "end_turn", "stop_sequence": None,
        "usage": {"input_tokens": 17, "output_tokens": 9,
                  "cache_creation_input_tokens": 0, "cache_read_input_tokens": 0},
    }


class FakeBundle:
    def __init__(self, root, request):
        self.root = root
        self.expected = copy.deepcopy(request)
        self.loaded = self.validated = self.claimed = 0
        self.manifest = {
            "implementation_sha": "c" * 40,
            "producer_sha256": {"anthropic_adapter.py": hashlib.sha256(
                Path(adapter.__file__).read_bytes()).hexdigest()},
            "provider": {
                "model": "claude-sonnet-5-5", "api_version": "2026-10-21",
                "settings": {"thinking": {"type": "disabled"}, "stream": False},
            },
            "budget": copy.deepcopy(request["budget"]),
            "http_timeout_seconds": {"count": 15, "message": 90},
            "process_timeout_seconds": 120,
        }

    def load_bundle(self, path, require_anchor=False):
        if path != self.root or require_anchor is not True:
            raise ValueError("unanchored bundle")
        self.loaded += 1
        return {"root": self.root, "manifest": self.manifest, "suite": {}, "parameters": {},
                "guides": {}, "anchor": {"verified": True}}

    def validate_model_request(self, request, bundle):
        if request != self.expected:
            raise ValueError("public request differs")
        self.validated += 1

    def claim_request(self, bundle, request, role):
        if role != "provider":
            raise ValueError("wrong claim role")
        self.claimed += 1
        path = self.root / "attempts" / request["request_id"] / role
        path.mkdir(parents=True, exist_ok=False)
        return path


class OfflineProviderControls(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix="cherri_ai_adapter_offline_")
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.request = model_request(
            {"id": "generate_unicode_literal", "kind": "generation", "prompt": "Display a public literal.",
             "starter": {"legacy": "", "v2": ""}},
            "v2", "Public guide only", "claude-sonnet-5-5",
            {"max_input_tokens": 100, "max_output_tokens": 20}, 19, "a" * 64,
        )
        self.env = {"CHERRI_AI_EVAL_ALLOW_GENERATION": "1",
                    "CHERRI_AI_EVAL_BUNDLE": str(self.root),
                    "ANTHROPIC_API_KEY": " \t" + KEY + "\r\n"}
        self.api = FakeBundle(self.root, self.request)
        self.calls = []
        self.responses = [
            adapter.HTTPResult(200, "req_count_control", body({"input_tokens": 18})),
            adapter.HTTPResult(200, "req_generation_control", body(message())),
        ]
        forbidden = mock.patch.object(adapter.http.client, "HTTPSConnection",
                                      side_effect=AssertionError("unmocked network forbidden"))
        forbidden.start()
        self.addCleanup(forbidden.stop)

    @property
    def attempt(self):
        return self.root / "attempts" / self.request["request_id"] / "provider"

    def transport(self, path, raw, credential, timeout):
        self.calls.append({"path": path, "body": json.loads(raw), "credential": credential, "timeout": timeout})
        value = self.responses.pop(0)
        if isinstance(value, BaseException):
            raise value
        return value

    def run_case(self, request=None, raw=None):
        request = self.request if request is None else request
        raw = body(request) if raw is None else raw
        return adapter.run_adapter(request, raw, self.env, transport=self.transport, bundle_api=self.api)

    def read_receipt(self):
        return json.loads((self.attempt / "provider-receipt.json").read_bytes())

    def change_response(self, value):
        self.responses[1] = adapter.HTTPResult(200, "req_generation_control", body(value))

    def assert_failure(self, kind, calls=0, status="FAILED"):
        reply, code = self.run_case()
        self.assertNotEqual(code, 0)
        self.assertEqual(reply["error_type"], kind)
        self.assertEqual(reply["status"], status)
        self.assertEqual(len(self.calls), calls)
        self.assertNotIn("code", reply)
        if self.attempt.exists():
            receipt = self.read_receipt()
            self.assertEqual(receipt["status"], status)
            self.assertEqual(receipt["error_type"], kind)
            self.assertEqual(json.loads((self.attempt / "reply.json").read_bytes()), reply)
        return reply

    def test_exact_public_request_single_count_single_generation_and_unmodified_source(self):
        raw = body(self.request)
        reply, code = self.run_case(raw=raw)
        self.assertEqual(code, 0)
        self.assertEqual(reply["code"], CODE)
        self.assertIs(reply["synthetic"], False)
        self.assertEqual(reply["usage"], {"input_tokens": 17, "output_tokens": 9})
        self.assertEqual(reply["provider_request_id"], "req_generation_control")
        self.assertEqual([c["path"] for c in self.calls],
                         ["/v1/messages/count_tokens", "/v1/messages"])
        self.assertEqual([c["timeout"] for c in self.calls], [15, 90])
        self.assertEqual([c["credential"] for c in self.calls], [KEY, KEY])
        for call in self.calls:
            payload = call["body"]
            self.assertEqual(payload["model"], "claude-sonnet-5-5")
            self.assertEqual(payload["thinking"], {"type": "disabled"})
            self.assertEqual(len(payload["messages"]), 1)
            self.assertEqual(payload["messages"][0]["role"], "user")
            public = json.loads(payload["messages"][0]["content"])
            self.assertEqual(public, {k: self.request[k] for k in
                                     ("arm", "kind", "prompt", "starter", "guide")})
            self.assertEqual(payload["system"], adapter.SYSTEM_PROMPT)
        self.assertEqual(set(self.calls[0]["body"]), {"model", "system", "messages", "thinking"})
        self.assertEqual(set(self.calls[1]["body"]),
                         {"model", "system", "messages", "thinking", "max_tokens", "stream"})
        self.assertIs(self.calls[1]["body"]["stream"], False)
        self.assertEqual(self.calls[1]["body"]["max_tokens"], 20)
        receipt = self.read_receipt()
        self.assertEqual(receipt["status"], "GENERATED")
        self.assertEqual(receipt["expected_model"], receipt["actual_model"])
        self.assertEqual(receipt["provider_preflight_input_tokens"], 18)
        self.assertIs(receipt["preflight_is_actual_usage"], False)
        self.assertEqual(receipt["usage_counts"]["total_input_tokens"], 17)
        self.assertEqual(receipt["source_sha256"], hashlib.sha256(CODE.encode()).hexdigest())
        self.assertEqual(json.loads((self.attempt/"request.json").read_bytes()), self.request)
        self.assertEqual(json.loads((self.attempt/"reply.json").read_bytes()), reply)
        self.assertEqual((self.attempt/"adapter-request.original.json").read_bytes(), raw)
        self.assertEqual((self.attempt/"generated-source.original.cherri").read_bytes(), CODE.encode())
        expected = {"adapter-request.original.json", "request.json", "reply.json",
                    "count.request.original.json", "count.response.original.body",
                    "generation.request.original.json", "generation.response.original.body",
                    "generated-source.original.cherri"}
        self.assertEqual(set(receipt["originals"]), expected)
        for name, identity in receipt["originals"].items():
            retained = (self.attempt/name).read_bytes()
            self.assertEqual(identity, {"sha256": hashlib.sha256(retained).hexdigest(),
                                       "bytes": len(retained), "complete": True})
            self.assertNotIn(KEY.encode(), retained)
        self.assertNotIn(KEY, (self.attempt/"provider-receipt.json").read_text())

    def test_multiple_text_blocks_are_concatenated_without_source_repair(self):
        value = message()
        value["content"] = [{"type":"text","text":"  "}, {"type":"text","text":"\n~~~cherri\n"},
                            {"type":"text","text":'show("🍒")\n~~~  \n'}]
        self.change_response(value)
        reply, code = self.run_case()
        self.assertEqual(code, 0)
        self.assertEqual(reply["code"], '  \n~~~cherri\nshow("🍒")\n~~~  \n')

    def test_generation_authorization_required_before_bundle_or_http(self):
        del self.env["CHERRI_AI_EVAL_ALLOW_GENERATION"]
        reply, code = self.run_case()
        self.assertEqual((reply["error_type"], code), ("generation_not_authorized", 2))
        self.assertEqual(self.calls, [])
        self.assertEqual(self.api.loaded, 0)

    def test_missing_credential_cannot_count(self):
        del self.env["ANTHROPIC_API_KEY"]
        self.assert_failure("credential_unavailable")

    def test_interior_credential_whitespace_is_not_rewritten(self):
        self.env["ANTHROPIC_API_KEY"] = "mock key"
        self.assert_failure("invalid_credential")

    def test_bundle_missing_cannot_count(self):
        del self.env["CHERRI_AI_EVAL_BUNDLE"]
        self.assert_failure("bundle_unavailable")

    def test_unanchored_bundle_failure_is_safe_and_never_sent(self):
        self.api.load_bundle = mock.Mock(side_effect=ValueError(KEY))
        reply = self.assert_failure("configuration_or_io_error")
        self.assertNotIn(KEY, json.dumps(reply))

    def test_arbitrary_public_input_rejected_by_sealed_bundle_before_claim(self):
        self.request["prompt"] = "not the approved instance"
        self.assert_failure("configuration_or_io_error")
        self.assertEqual(self.api.claimed, 0)

    def test_unknown_request_field_is_not_forwarded(self):
        self.request["oracle"] = "not model input"
        self.assert_failure("invalid_request_fields")

    def test_wrong_requested_model_rejected_before_claim(self):
        self.request["model"] = "another-model"
        self.assert_failure("request_identity_mismatch")
        self.assertEqual(self.api.claimed, 0)

    def test_manifest_adapter_hash_mismatch_fails_before_http(self):
        self.api.manifest["producer_sha256"]["anthropic_adapter.py"] = "0" * 64
        self.assert_failure("adapter_source_mismatch")

    def test_manifest_provider_settings_cannot_enable_tools_or_thinking(self):
        self.api.manifest["provider"]["settings"]["tools"] = []
        self.assert_failure("provider_configuration_mismatch")

    def test_manifest_timeout_override_rejected(self):
        self.api.manifest["http_timeout_seconds"]["message"] = 91
        self.assert_failure("timeout_configuration_mismatch")

    def test_manifest_process_timeout_override_rejected(self):
        self.api.manifest["process_timeout_seconds"] = 121
        self.assert_failure("timeout_configuration_mismatch")

    def test_manifest_budget_override_rejected(self):
        self.api.manifest["budget"]["max_input_tokens"] = 101
        self.assert_failure("bundle_budget_mismatch")

    def test_preflight_input_budget_prevents_generation(self):
        self.responses[0] = adapter.HTTPResult(200, "req_count_control", body({"input_tokens": 101}))
        self.assert_failure("preflight_input_budget_exceeded", 1)
        self.assertIs(self.read_receipt()["calls"]["generation"]["attempted"], False)

    def test_preflight_boolean_is_not_a_token_count(self):
        self.responses[0] = adapter.HTTPResult(200, "req_count_control", body({"input_tokens": True}))
        self.assert_failure("invalid_preflight_count", 1)

    def test_count_http_error_preserved_without_retry(self):
        self.responses[0] = adapter.HTTPResult(429, "req_count_control",
                                               body({"type":"error","error":{"type":"rate_limit_error",
                                                                            "message":"local public control"}}))
        self.assert_failure("provider_http_error", 1)
        self.assertEqual(self.read_receipt()["calls"]["count"]["http_status"], 429)

    def test_redirect_is_failure_with_no_follow_or_generation(self):
        self.responses[0] = adapter.HTTPResult(302, "req_redirect", body({"type":"redirect"}))
        self.assert_failure("provider_http_error", 1)

    def test_missing_count_request_id_fails_before_generation(self):
        self.responses[0] = adapter.HTTPResult(200, None, body({"input_tokens": 18}))
        self.assert_failure("missing_provider_request_id", 1)

    def test_invalid_count_json_is_original_and_fails(self):
        self.responses[0] = adapter.HTTPResult(200, "req_count_control", b'{"input_tokens":18,')
        self.assert_failure("invalid_json", 1)
        self.assertEqual((self.attempt/"count.response.original.body").read_bytes(), b'{"input_tokens":18,')

    def test_duplicate_count_key_rejected(self):
        self.responses[0] = adapter.HTTPResult(200, "req_count_control", b'{"input_tokens":18,"input_tokens":1}')
        self.assert_failure("duplicate_json_key", 1)

    def test_generation_http_error_keeps_two_attempts_and_raw_body(self):
        self.responses[1] = adapter.HTTPResult(500, "req_generation_control",
                                               body({"error":{"type":"api_error","message":"public fault"}}))
        self.assert_failure("provider_http_error", 2)
        self.assertEqual(self.read_receipt()["calls"]["generation"]["http_status"], 500)

    def test_actual_usage_over_cap_is_rejected_after_preserving_source(self):
        value = message()
        value["usage"]["input_tokens"] = 101
        self.change_response(value)
        self.assert_failure("actual_usage_budget_exceeded", 2)
        self.assertEqual((self.attempt/"generated-source.original.cherri").read_bytes(), CODE.encode())
        self.assertEqual(self.read_receipt()["usage_counts"]["total_input_tokens"], 101)

    def test_output_usage_over_cap_is_rejected(self):
        value = message()
        value["usage"]["output_tokens"] = 21
        self.change_response(value)
        self.assert_failure("actual_usage_budget_exceeded", 2)

    def test_all_cache_components_counted_without_double_counting_breakdown(self):
        value = message()
        value["usage"].update(input_tokens=88, cache_creation_input_tokens=7, cache_read_input_tokens=6,
                              cache_creation={"ephemeral_1h_input_tokens":3,"ephemeral_5m_input_tokens":4})
        self.change_response(value)
        self.assert_failure("actual_usage_budget_exceeded", 2)
        self.assertEqual(self.read_receipt()["usage_counts"]["total_input_tokens"], 101)

    def test_unrequested_cache_usage_is_rejected_even_within_budget(self):
        value = message()
        value["usage"]["cache_read_input_tokens"] = 1
        self.change_response(value)
        self.assert_failure("unexpected_cache_usage", 2)

    def test_usage_boolean_is_rejected(self):
        value = message()
        value["usage"]["output_tokens"] = True
        self.change_response(value)
        self.assert_failure("invalid_usage", 2)

    def test_unknown_token_component_fails_closed(self):
        value = message()
        value["usage"]["other_input_tokens"] = 1
        self.change_response(value)
        self.assert_failure("unsupported_token_component", 2)

    def test_cache_subtotal_must_reconcile(self):
        value = message()
        value["usage"]["cache_creation"] = {"ephemeral_1h_input_tokens":1}
        self.change_response(value)
        self.assert_failure("cache_breakdown_mismatch", 2)

    def test_actual_model_mismatch_preserves_original(self):
        value = message()
        value["model"] = "unexpected-model"
        self.change_response(value)
        self.assert_failure("actual_model_mismatch", 2)
        self.assertEqual(self.read_receipt()["actual_model"], "unexpected-model")

    def test_refusal_preserves_real_usage_and_original_source(self):
        value = message("I decline this request.\n")
        value["stop_reason"] = "refusal"
        self.change_response(value)
        self.assert_failure("provider_refusal", 2, "REFUSED")
        self.assertEqual((self.attempt/"generated-source.original.cherri").read_bytes(),
                         b"I decline this request.\n")
        self.assertEqual(self.read_receipt()["usage_counts"]["output_tokens"], 9)

    def test_truncation_is_retained_not_repaired_or_retried(self):
        value = message('show("unfinished')
        value["stop_reason"] = "max_tokens"
        self.change_response(value)
        self.assert_failure("provider_truncation", 2, "TRUNCATED")
        self.assertEqual((self.attempt/"generated-source.original.cherri").read_bytes(), b'show("unfinished')

    def test_thinking_usage_rejected(self):
        value = message()
        value["usage"]["output_tokens_details"] = {"thinking_tokens":1}
        self.change_response(value)
        self.assert_failure("unexpected_thinking_usage", 2)

    def test_server_tool_usage_rejected(self):
        value = message()
        value["usage"]["server_tool_use"] = {"web_search_requests":1}
        self.change_response(value)
        self.assert_failure("unexpected_server_tool_usage", 2)

    def test_tool_call_content_does_not_become_source(self):
        value = message()
        value["content"].append({"type":"tool_use","id":"x","name":"tool","input":{}})
        self.change_response(value)
        self.assert_failure("missing_raw_source", 2)
        self.assertFalse((self.attempt/"generated-source.original.cherri").exists())

    def test_provider_iteration_rejected(self):
        value = message()
        value["usage"]["iterations"] = [{"type":"message","input_tokens":1,"output_tokens":1}]
        self.change_response(value)
        self.assert_failure("unexpected_provider_iteration", 2)

    def test_context_rewrite_is_rejected(self):
        value = message()
        value["context_management"] = {"applied_edits":[{"type":"clear_tool_uses"}]}
        self.change_response(value)
        self.assert_failure("unexpected_context_transformation", 2)

    def test_transport_timeout_retains_partial_generation_body_and_usage_unknown(self):
        self.responses[1] = adapter.TransportFailure(
            "transport_timeout", adapter.HTTPResult(200, "req_generation_control", b'{"partial":', False))
        self.assert_failure("transport_timeout", 2)
        receipt = self.read_receipt()
        self.assertIs(receipt["originals"]["generation.response.original.body"]["complete"], False)
        self.assertNotIn("usage_counts", receipt)

    def test_transport_exception_text_never_reaches_receipt_or_stdout(self):
        self.responses[0] = RuntimeError("secret header " + KEY)
        reply = self.assert_failure("transport_error", 1)
        self.assertNotIn(KEY, json.dumps(reply))
        self.assertNotIn(KEY, (self.attempt/"provider-receipt.json").read_text())

    def test_cancelled_claim_remains_final_and_cannot_retry(self):
        self.responses[0] = KeyboardInterrupt()
        reply, code = self.run_case()
        self.assertEqual((reply["status"], code), ("CANCELLED", 130))
        self.assertEqual(self.read_receipt()["status"], "CANCELLED")
        before = (self.attempt/"provider-receipt.json").read_bytes()
        repeated, exit_code = self.run_case()
        self.assertNotEqual(exit_code, 0)
        self.assertEqual(repeated["error_type"], "configuration_or_io_error")
        self.assertEqual(len(self.calls), 1)
        self.assertEqual((self.attempt/"provider-receipt.json").read_bytes(), before)

    def test_successful_claim_cannot_generate_a_second_time(self):
        self.assertEqual(self.run_case()[1], 0)
        snapshot = {p.name:p.read_bytes() for p in self.attempt.iterdir() if p.is_file()}
        repeated, code = self.run_case()
        self.assertNotEqual(code, 0)
        self.assertEqual(len(self.calls), 2)
        self.assertEqual(snapshot, {p.name:p.read_bytes() for p in self.attempt.iterdir() if p.is_file()})

    def test_reflected_credential_is_withheld_with_hash_and_never_printed(self):
        raw = body({"error":{"type":"api_error","message":KEY}})
        self.responses[0] = adapter.HTTPResult(500, "req_count_control", raw)
        reply = self.assert_failure("credential_reflected", 1)
        self.assertNotIn(KEY, json.dumps(reply))
        self.assertFalse((self.attempt/"count.response.original.body").exists())
        identity = self.read_receipt()["originals"]["count.response.original.body"]
        self.assertEqual(identity["sha256"], hashlib.sha256(raw).hexdigest())
        self.assertEqual(identity["withheld"], "credential_reflected")
        for path in self.attempt.iterdir():
            if path.is_file():
                self.assertNotIn(KEY.encode(), path.read_bytes())

    def test_server_http_date_retained_as_noncryptographic_receipt(self):
        self.responses[0].response_date = "Sat, 21 Nov 2026 22:00:00 GMT"
        self.responses[1].response_date = "Sat, 21 Nov 2026 22:00:01 GMT"
        self.assertEqual(self.run_case()[1], 0)
        calls = self.read_receipt()["calls"]
        self.assertEqual(calls["count"]["response_date"], "Sat, 21 Nov 2026 22:00:00 GMT")
        self.assertEqual(calls["generation"]["response_date"], "Sat, 21 Nov 2026 22:00:01 GMT")

    def test_malformed_server_date_is_not_serialized(self):
        self.responses[0].response_date = KEY
        self.assertEqual(self.run_case()[1], 0)
        self.assertIsNone(self.read_receipt()["calls"]["count"]["response_date"])
        self.assertNotIn(KEY, (self.attempt/"provider-receipt.json").read_text())

    def test_invalid_http_status_cannot_become_sensitive_receipt_metadata(self):
        self.responses[0] = adapter.HTTPResult(KEY, "req_count_control", b"{}")
        self.assert_failure("invalid_http_status", 1)
        self.assertIsNone(self.read_receipt()["calls"]["count"]["http_status"])

    def test_claimed_io_failure_reports_incomplete_instead_of_success(self):
        with mock.patch.object(adapter, "atomic_json", side_effect=OSError(KEY)):
            reply, code = self.run_case()
        self.assertNotEqual(code, 0)
        self.assertEqual(reply["error_type"], "receipt_write_failed")
        self.assertNotIn(KEY, json.dumps(reply))
        self.assertTrue(self.attempt.exists())
        self.assertEqual(self.calls, [])

    def test_raw_envelope_mismatch_rejected(self):
        raw = body({**self.request,"prompt":"other"})
        reply, code = self.run_case(raw=raw)
        self.assertEqual((reply["error_type"], code), ("raw_request_mismatch", 1))
        self.assertEqual(self.calls, [])

    def test_main_rejects_duplicate_input_key_without_network(self):
        out = io.StringIO()
        code = adapter.main(io.BytesIO(b'{"schema":1,"schema":2}'), out, {})
        self.assertEqual(code, 1)
        self.assertEqual(json.loads(out.getvalue())["error_type"], "duplicate_json_key")

    def test_main_rejects_nonfinite_input(self):
        out = io.StringIO()
        code = adapter.main(io.BytesIO(b'{"seed":NaN}'), out, {})
        self.assertEqual(code, 1)
        self.assertEqual(json.loads(out.getvalue())["error_type"], "nonfinite_json")

    def test_main_bounded_input(self):
        out = io.StringIO()
        code = adapter.main(io.BytesIO(b"x"*(adapter.MAX_BYTES+1)), out, {})
        self.assertEqual(code, 1)
        self.assertEqual(json.loads(out.getvalue())["error_type"], "request_too_large")


class FixedTransportControls(unittest.TestCase):
    def fake_connection(self, status=200, chunks=None, request_id="req_transport"):
        chunks = [b"{}"] if chunks is None else chunks
        response = types.SimpleNamespace(status=status)
        response.getheader = mock.Mock(side_effect=lambda name: request_id if name == "request-id" else
                                             "Sat, 21 Nov 2026 22:00:00 GMT" if name == "Date" else None)
        response.read1 = mock.Mock(side_effect=[*chunks, b""])
        connection = types.SimpleNamespace(
            sock=types.SimpleNamespace(settimeout=mock.Mock()),
            request=mock.Mock(), getresponse=mock.Mock(return_value=response), close=mock.Mock())
        return connection, response

    def test_https_origin_headers_single_request_no_redirect_follow(self):
        connection, response = self.fake_connection(status=302)
        with mock.patch.object(adapter.http.client, "HTTPSConnection", return_value=connection) as constructor:
            result = adapter.post_json("/v1/messages", b"{}", KEY, 90)
        self.assertEqual(constructor.call_count, 1)
        self.assertEqual(constructor.call_args.args, ("api.anthropic.com",))
        self.assertEqual(constructor.call_args.kwargs["timeout"], 90)
        self.assertEqual(result.status, 302)
        self.assertEqual(result.response_date, "Sat, 21 Nov 2026 22:00:00 GMT")
        connection.request.assert_called_once()
        args, kwargs = connection.request.call_args
        self.assertEqual(args, ("POST", "/v1/messages"))
        self.assertEqual(kwargs["headers"],
                         {"x-api-key":KEY, "anthropic-version":"2026-10-21",
                          "Content-Type":"application/json","Accept":"application/json"})
        connection.close.assert_called_once()

    def test_other_endpoint_rejected_before_connection(self):
        with mock.patch.object(adapter.http.client, "HTTPSConnection") as constructor:
            with self.assertRaisesRegex(adapter.AdapterFailure, "endpoint_not_allowed"):
                adapter.post_json("https://other.invalid/v1/messages", b"{}", KEY, 90)
            constructor.assert_not_called()

    def test_transport_bounds_original_body_and_reports_incomplete(self):
        connection, response = self.fake_connection(chunks=[b"abcde"])
        with mock.patch.object(adapter, "MAX_BYTES", 4), \
             mock.patch.object(adapter.http.client, "HTTPSConnection", return_value=connection):
            with self.assertRaises(adapter.TransportFailure) as failure:
                adapter.post_json("/v1/messages", b"{}", KEY, 90)
        self.assertEqual(failure.exception.kind, "response_too_large")
        self.assertEqual(failure.exception.result.body, b"abcde")
        self.assertIs(failure.exception.result.complete, False)
        self.assertEqual(response.read1.call_args.args, (5,))
        connection.close.assert_called_once()

    def test_late_eof_cannot_be_a_successful_http_response(self):
        connection, response = self.fake_connection(chunks=[])
        with mock.patch.object(adapter.http.client, "HTTPSConnection", return_value=connection), \
             mock.patch.object(adapter.time, "monotonic", side_effect=[0, 0, 0, 16]):
            with self.assertRaises(adapter.TransportFailure) as failure:
                adapter.post_json("/v1/messages/count_tokens", b"{}", KEY, 15)
        self.assertEqual(failure.exception.kind, "transport_timeout")
        self.assertIs(failure.exception.result.complete, False)
        self.assertEqual(failure.exception.result.status, 200)
        connection.request.assert_called_once()
        connection.close.assert_called_once()

    def test_late_body_is_preserved_as_incomplete(self):
        connection, response = self.fake_connection(chunks=[b"{}"])
        with mock.patch.object(adapter.http.client, "HTTPSConnection", return_value=connection), \
             mock.patch.object(adapter.time, "monotonic", side_effect=[0, 0, 0, 91]):
            with self.assertRaises(adapter.TransportFailure) as failure:
                adapter.post_json("/v1/messages", b"{}", KEY, 90)
        self.assertEqual(failure.exception.kind, "transport_timeout")
        self.assertEqual(failure.exception.result.body, b"{}")
        self.assertIs(failure.exception.result.complete, False)

    def test_transport_exception_message_is_discarded(self):
        connection, response = self.fake_connection()
        connection.request.side_effect = OSError(KEY)
        with mock.patch.object(adapter.http.client, "HTTPSConnection", return_value=connection):
            with self.assertRaises(adapter.TransportFailure) as failure:
                adapter.post_json("/v1/messages/count_tokens", b"{}", KEY, 15)
        self.assertEqual(str(failure.exception), "transport_error")
        self.assertEqual(failure.exception.result.body, b"")
        connection.request.assert_called_once()
        connection.close.assert_called_once()


if __name__ == "__main__":
    unittest.main()
