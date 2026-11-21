#!/usr/bin/env python3
"""Single-attempt Claude transport for an anchored, reviewed Cherri evaluation.

This adapter never scores model quality. It sends only the approved public
request fields; the independent bundle/validator owns tasks and semantic oracles.
"""
import hashlib
import http.client
import importlib
import json
import os
from pathlib import Path
import re
import ssl
import stat
import sys
import time
from dataclasses import dataclass
from datetime import datetime, timezone
from email.utils import parsedate_to_datetime, format_datetime

from runner import SCHEMA, atomic_json

HOST = "api.anthropic.com"
MODEL = "claude-sonnet-5-5"
API_VERSION = "2026-10-21"
COUNT_PATH = "/v1/messages/count_tokens"
MESSAGE_PATH = "/v1/messages"
COUNT_TIMEOUT = 15
MESSAGE_TIMEOUT = 90
PROCESS_TIMEOUT = 120
MAX_BYTES = 1024 * 1024
RECEIPT_SCHEMA = "cherri-anthropic-provider/v1"
SYSTEM_PROMPT = (
    "Write Cherri source for the supplied language arm, guide, and task. "
    "Return only the complete source text, without Markdown fences or explanations. "
    "Use only the supplied language guide. Do not use tools."
)
REQUEST_FIELDS = {
    "schema", "request_id", "model", "budget", "arm", "task_id", "kind",
    "prompt", "starter", "guide", "seed", "suite_sha256",
}


def digest(raw):
    return hashlib.sha256(raw).hexdigest()


def encoded(value):
    return json.dumps(value, ensure_ascii=False, sort_keys=True, separators=(",", ":")).encode("utf-8")


def utc_now():
    return datetime.now(timezone.utc).isoformat()


class AdapterFailure(Exception):
    def __init__(self, kind, status="FAILED"):
        super().__init__(kind)
        self.kind = kind
        self.status = status


@dataclass
class HTTPResult:
    status: int | None
    request_id: str | None
    body: bytes
    complete: bool = True
    response_date: str | None = None


def safe_response_date(value):
    if not isinstance(value, str) or not re.fullmatch(
            r"[A-Z][a-z]{2}, [0-9]{2} [A-Z][a-z]{2} [0-9]{4} [0-9]{2}:[0-9]{2}:[0-9]{2} GMT", value):
        return None
    try:
        parsed = parsedate_to_datetime(value)
        return value if format_datetime(parsed, usegmt=True) == value else None
    except (ValueError, TypeError, OverflowError):
        return None


class TransportFailure(Exception):
    def __init__(self, kind, result):
        super().__init__(kind)
        self.kind = kind
        self.result = result


def post_json(path, body, credential, timeout):
    """Fixed HTTPS origin, one request, no redirect/fallback/retry mechanism."""
    if path not in (COUNT_PATH, MESSAGE_PATH):
        raise AdapterFailure("endpoint_not_allowed")
    connection = http.client.HTTPSConnection(
        HOST, timeout=timeout, context=ssl.create_default_context()
    )
    status = request_id = response_date = None
    chunks = []
    size = 0
    deadline = time.monotonic() + timeout
    try:
        connection.request("POST", path, body=body, headers={
            "x-api-key": credential,
            "anthropic-version": API_VERSION,
            "Content-Type": "application/json",
            "Accept": "application/json",
        })
        remaining = deadline - time.monotonic()
        if remaining <= 0:
            raise TimeoutError()
        if connection.sock is not None:
            connection.sock.settimeout(remaining)
        response = connection.getresponse()
        status = response.status
        request_id = response.getheader("request-id")
        response_date = safe_response_date(response.getheader("Date"))
        while True:
            remaining = deadline - time.monotonic()
            if remaining <= 0:
                raise TimeoutError()
            if connection.sock is not None:
                connection.sock.settimeout(remaining)
            chunk = response.read1(min(65536, MAX_BYTES + 1 - size))
            if not chunk:
                return HTTPResult(status, request_id, b"".join(chunks), True, response_date)
            chunks.append(chunk)
            size += len(chunk)
            if size > MAX_BYTES:
                raise TransportFailure("response_too_large",
                                       HTTPResult(status, request_id, b"".join(chunks), False, response_date))
    except TransportFailure:
        raise
    except KeyboardInterrupt:
        raise TransportFailure("cancelled", HTTPResult(status, request_id, b"".join(chunks), False)) from None
    except (TimeoutError, OSError, http.client.HTTPException) as error:
        kind = "transport_timeout" if isinstance(error, TimeoutError) else "transport_error"
        raise TransportFailure(kind, HTTPResult(status, request_id, b"".join(chunks), False)) from None
    finally:
        connection.close()


def _unique_pairs(pairs):
    value = {}
    for key, item in pairs:
        if key in value:
            raise AdapterFailure("duplicate_json_key")
        value[key] = item
    return value


def strict_json(raw):
    def invalid_constant(_):
        raise AdapterFailure("nonfinite_json")
    try:
        return json.loads(raw.decode("utf-8"), object_pairs_hook=_unique_pairs,
                          parse_constant=invalid_constant)
    except (UnicodeError, ValueError, TypeError):
        raise AdapterFailure("invalid_json") from None


def check(condition, kind):
    if not condition:
        raise AdapterFailure(kind)


def integer(value, kind, minimum=0):
    check(type(value) is int and value >= minimum, kind)
    return value


def basic_request(request):
    check(isinstance(request, dict) and set(request) == REQUEST_FIELDS, "invalid_request_fields")
    check(request["schema"] == SCHEMA and request["model"] == MODEL, "request_identity_mismatch")
    check(isinstance(request["request_id"], str) and
          re.fullmatch(r"[0-9a-f]{64}", request["request_id"]), "invalid_request_id")
    check(isinstance(request["suite_sha256"], str) and
          re.fullmatch(r"[0-9a-f]{64}", request["suite_sha256"]), "invalid_suite_hash")
    check(request["arm"] in ("legacy", "v2") and
          request["kind"] in ("generation", "edit"), "invalid_task_identity")
    check(isinstance(request["task_id"], str) and
          re.fullmatch(r"[a-z][a-z0-9_]{0,63}", request["task_id"]), "invalid_task_identity")
    check(all(isinstance(request[key], str) for key in ("prompt", "starter", "guide")),
          "invalid_public_input")
    check(bool(request["prompt"]) and bool(request["guide"]), "empty_public_input")
    integer(request["seed"], "invalid_seed")
    budget = request["budget"]
    check(isinstance(budget, dict) and set(budget) == {"max_input_tokens", "max_output_tokens"},
          "invalid_budget")
    for value in budget.values():
        integer(value, "invalid_budget", 1)


def provider_payload(request):
    # Explicit allowlist: seed, suite, private parameters and oracles never enter
    # the provider's context. This exact transformation is source-bound.
    public = {key: request[key] for key in ("arm", "kind", "prompt", "starter", "guide")}
    return {
        "model": MODEL,
        "system": SYSTEM_PROMPT,
        "messages": [{"role": "user", "content": encoded(public).decode("utf-8")}],
        "thinking": {"type": "disabled"},
    }


def actual_usage(value):
    check(isinstance(value, dict), "missing_usage")
    counts = {
        "input_tokens": integer(value.get("input_tokens"), "invalid_usage"),
        "output_tokens": integer(value.get("output_tokens"), "invalid_usage"),
    }
    for name in ("cache_creation_input_tokens", "cache_read_input_tokens"):
        item = value.get(name)
        counts[name] = 0 if item is None else integer(item, "invalid_cache_usage")
    creation = value.get("cache_creation")
    if creation is not None:
        check(isinstance(creation, dict) and set(creation) <= {
            "ephemeral_1h_input_tokens", "ephemeral_5m_input_tokens"}, "invalid_cache_breakdown")
        subtotal = sum(integer(item, "invalid_cache_breakdown") for item in creation.values())
        check(subtotal == counts["cache_creation_input_tokens"], "cache_breakdown_mismatch")
    for key in value:
        if key.endswith("_tokens") and key not in counts:
            raise AdapterFailure("unsupported_token_component")
    counts["total_input_tokens"] = (
        counts["input_tokens"] + counts["cache_creation_input_tokens"] + counts["cache_read_input_tokens"]
    )
    return counts


def safe_provider_id(value, prefix, credential):
    if not isinstance(value, str) or credential in value:
        return None
    return value if re.fullmatch(prefix + r"_[A-Za-z0-9_-]{1,192}", value) else None


def plain_text(response):
    content = response.get("content")
    if not isinstance(content, list) or not content:
        return None
    if any(not isinstance(block, dict) or block.get("type") != "text" or
           not isinstance(block.get("text"), str) for block in content):
        return None
    return "".join(block["text"] for block in content)


def run_adapter(request, raw_request, env, *, transport=None, bundle_api=None):
    """Return one protocol reply and exit code; tests inject only local fakes."""
    transport = post_json if transport is None else transport
    receipt = attempt = None
    credential = ""
    identity = request.get("request_id") if isinstance(request, dict) else None
    identity = identity if isinstance(identity, str) and re.fullmatch(r"[0-9a-f]{64}", identity) else None

    def persist():
        # Provider diagnostics/headers/credentials are never serialized here.
        check(credential.encode() not in encoded(receipt), "credential_reflected")
        atomic_json(attempt / "provider-receipt.json", receipt)

    def original(name, raw, *, complete=True):
        record = {"sha256": digest(raw), "bytes": len(raw), "complete": complete}
        receipt["originals"][name] = record
        if credential.encode() in raw:
            record["withheld"] = "credential_reflected"
            raise AdapterFailure("credential_reflected")
        check(len(raw) <= MAX_BYTES + 1, "artifact_too_large")
        descriptor = os.open(attempt / name, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
        with os.fdopen(descriptor, "wb") as stream:
            stream.write(raw)

    def exchange(stage, path, payload, timeout):
        body = encoded(payload)
        original(stage + ".request.original.json", body)
        row = receipt["calls"][stage]
        row.update(attempted=True, started_at=utc_now(), request_sha256=digest(body))
        receipt["status"] = stage.upper() + "_STARTED"
        persist()
        failure = None
        try:
            result = transport(path, body, credential, timeout)
        except TransportFailure as error:
            result = error.result
            failure = error.kind
        except KeyboardInterrupt:
            result = HTTPResult(None, None, b"", False)
            failure = "cancelled"
        except Exception:
            # Never retain an exception message: HTTP libraries can include
            # credential-bearing headers or URLs in their diagnostics.
            result = HTTPResult(None, None, b"", False)
            failure = "transport_error"
        check(isinstance(result, HTTPResult) and isinstance(result.body, bytes), "invalid_transport_result")
        safe_status = result.status if type(result.status) is int and 100 <= result.status <= 599 else None
        row.update(finished_at=utc_now(), http_status=safe_status,
                   provider_request_id=safe_provider_id(result.request_id, "req", credential),
                   response_complete=result.complete is True,
                   response_date=safe_response_date(result.response_date))
        original(stage + ".response.original.body", result.body, complete=result.complete)
        if failure:
            row["error_type"] = failure
            raise AdapterFailure(failure, "CANCELLED" if failure == "cancelled" else "FAILED")
        check(type(result.status) is int and 100 <= result.status <= 599, "invalid_http_status")
        check(result.complete is True and len(result.body) <= MAX_BYTES, "incomplete_http_response")
        parsed = strict_json(result.body)
        check(isinstance(parsed, dict), "invalid_provider_response")
        if result.status != 200:
            error = parsed.get("error")
            kind = error.get("type") if isinstance(error, dict) else None
            row["error_type"] = kind if isinstance(kind, str) and re.fullmatch(r"[a-z_]{1,64}", kind) else "provider_http_error"
            raise AdapterFailure("provider_http_error")
        check(row["provider_request_id"] is not None, "missing_provider_request_id")
        return parsed

    try:
        check(env.get("CHERRI_AI_EVAL_ALLOW_GENERATION") == "1", "generation_not_authorized")
        basic_request(request)
        check(isinstance(raw_request, bytes) and len(raw_request) <= MAX_BYTES, "request_too_large")
        check(strict_json(raw_request) == request, "raw_request_mismatch")
        raw_key = env.get("ANTHROPIC_API_KEY")
        check(isinstance(raw_key, str), "credential_unavailable")
        credential = raw_key.strip()
        check(bool(credential) and len(credential) <= 4096 and
              all(33 <= ord(c) <= 126 for c in credential), "invalid_credential")
        bundle_path = env.get("CHERRI_AI_EVAL_BUNDLE")
        check(isinstance(bundle_path, str) and Path(bundle_path).is_absolute(), "bundle_unavailable")
        bundle_api = importlib.import_module("evaluation_bundle") if bundle_api is None else bundle_api
        bundle = bundle_api.load_bundle(Path(bundle_path), require_anchor=True)
        bundle_api.validate_model_request(request, bundle)
        manifest = bundle["manifest"]
        check(manifest["provider"] == {
            "model": MODEL, "api_version": API_VERSION,
            "settings": {"thinking": {"type": "disabled"}, "stream": False},
        } and manifest["provider"]["settings"]["stream"] is False, "provider_configuration_mismatch")
        check(manifest["budget"] == request["budget"], "bundle_budget_mismatch")
        check(manifest["http_timeout_seconds"] == {"count": COUNT_TIMEOUT, "message": MESSAGE_TIMEOUT}
              and all(type(v) is int for v in manifest["http_timeout_seconds"].values())
              and type(manifest["process_timeout_seconds"]) is int
              and manifest["process_timeout_seconds"] == PROCESS_TIMEOUT, "timeout_configuration_mismatch")
        implementation = manifest["implementation_sha"]
        check(isinstance(implementation, str) and re.fullmatch(r"[0-9a-f]{40}", implementation),
              "invalid_implementation_sha")
        source_file = Path(__file__)
        check(not source_file.is_symlink() and not getattr(source_file, "is_junction", lambda: False)()
              and stat.S_ISREG(source_file.lstat().st_mode), "invalid_adapter_source")
        adapter_sha = digest(source_file.read_bytes())
        check(manifest["producer_sha256"][source_file.name] == adapter_sha, "adapter_source_mismatch")
        attempt = Path(bundle_api.claim_request(bundle, request, "provider"))
        root = Path(bundle["root"])
        expected = root / "attempts" / request["request_id"] / "provider"
        check(root.is_absolute() and attempt == expected and attempt.is_dir(), "invalid_claim_directory")
        check(all(not p.is_symlink() and not getattr(p, "is_junction", lambda: False)()
                  for p in (root, root/"attempts", expected.parent, expected)), "linked_claim_directory")
        receipt = {
            "schema": RECEIPT_SCHEMA, "request_id": request["request_id"],
            "task_id": request["task_id"], "arm": request["arm"],
            "implementation_sha": implementation, "adapter_sha256": adapter_sha,
            "expected_model": MODEL, "actual_model": None, "api_version": API_VERSION,
            "started_at": utc_now(), "finished_at": None, "status": "STARTED",
            "system_sha256": digest(SYSTEM_PROMPT.encode()),
            "request_sha256": digest(raw_request), "budget": request["budget"],
            "http_timeout_seconds": {"count": COUNT_TIMEOUT, "message": MESSAGE_TIMEOUT},
            "process_timeout_seconds": PROCESS_TIMEOUT,
            "calls": {"count": {"attempted": False}, "generation": {"attempted": False}},
            "originals": {},
        }
        check(not (attempt/"provider-receipt.json").exists(), "occupied_receipt")
        original("adapter-request.original.json", raw_request)
        original("request.json", encoded(request))
        persist()
        payload = provider_payload(request)
        count = exchange("count", COUNT_PATH, payload, COUNT_TIMEOUT)
        preflight = integer(count.get("input_tokens"), "invalid_preflight_count", 1)
        receipt["provider_preflight_input_tokens"] = preflight
        receipt["preflight_is_actual_usage"] = False
        check(preflight <= request["budget"]["max_input_tokens"], "preflight_input_budget_exceeded")
        payload = {**payload, "max_tokens": request["budget"]["max_output_tokens"], "stream": False}
        response = exchange("generation", MESSAGE_PATH, payload, MESSAGE_TIMEOUT)
        receipt["actual_model"] = response.get("model") if isinstance(response.get("model"), str) else None
        receipt["provider_message_id"] = safe_provider_id(response.get("id"), "msg", credential)
        receipt["stop_reason"] = response.get("stop_reason")
        receipt["usage_original"] = response.get("usage")
        code = plain_text(response)
        if code is not None:
            original("generated-source.original.cherri", code.encode("utf-8"))
            receipt["source_sha256"] = digest(code.encode("utf-8"))
        usage = actual_usage(response.get("usage"))
        receipt["usage_counts"] = usage
        check(usage["total_input_tokens"] <= request["budget"]["max_input_tokens"]
              and usage["output_tokens"] <= request["budget"]["max_output_tokens"], "actual_usage_budget_exceeded")
        check(response.get("type") == "message" and response.get("role") == "assistant"
              and receipt["provider_message_id"] is not None, "invalid_message_identity")
        check(response.get("model") == MODEL, "actual_model_mismatch")
        stop = response.get("stop_reason")
        if stop == "refusal":
            raise AdapterFailure("provider_refusal", "REFUSED")
        if stop == "max_tokens":
            raise AdapterFailure("provider_truncation", "TRUNCATED")
        check(stop == "end_turn" and response.get("stop_sequence") is None, "unexpected_stop_reason")
        check(usage["cache_creation_input_tokens"] == 0 and usage["cache_read_input_tokens"] == 0,
              "unexpected_cache_usage")
        details = response["usage"].get("output_tokens_details")
        if details is not None:
            check(isinstance(details, dict) and set(details) <= {"thinking_tokens"}
                  and integer(details.get("thinking_tokens", 0), "invalid_thinking_usage") == 0,
                  "unexpected_thinking_usage")
        tools = response["usage"].get("server_tool_use")
        if tools is not None:
            check(isinstance(tools, dict) and all(type(v) is int and v == 0 for v in tools.values()),
                  "unexpected_server_tool_usage")
        check(response["usage"].get("iterations") in (None, []) and
              response["usage"].get("fallback_credit") is None, "unexpected_provider_iteration")
        check(response.get("container") is None and response.get("input_transformations") in (None, []),
              "unexpected_provider_state")
        context = response.get("context_management")
        check(context is None or context == {"applied_edits": []}, "unexpected_context_transformation")
        check(code is not None and bool(code.strip()), "missing_raw_source")
        check(digest(source_file.read_bytes()) == adapter_sha, "adapter_changed_during_request")
        reply = {
            "schema": SCHEMA, "request_id": request["request_id"], "model": response["model"],
            "synthetic": False, "provider_request_id": receipt["calls"]["generation"]["provider_request_id"],
            "code": code, "usage": {"input_tokens": usage["total_input_tokens"],
                                  "output_tokens": usage["output_tokens"]},
        }
        original("reply.json", encoded(reply))
        receipt.update(status="GENERATED", finished_at=utc_now())
        persist()
        return reply, 0
    except AdapterFailure as error:
        failure = error
    except KeyboardInterrupt:
        failure = AdapterFailure("cancelled", "CANCELLED")
    except Exception:
        failure = AdapterFailure("configuration_or_io_error")
    reply = {"schema": SCHEMA, "request_id": identity, "model": MODEL, "synthetic": False,
             "status": failure.status, "error_type": failure.kind}
    if receipt is not None:
        receipt.update(status=failure.status, error_type=failure.kind, finished_at=utc_now())
        try:
            if not (attempt / "reply.json").exists():
                original("reply.json", encoded(reply))
            persist()
        except Exception:
            # A prior STARTED receipt/claimed directory remains incomplete. Do
            # not print filesystem errors or fabricate a completed receipt.
            failure = AdapterFailure("receipt_write_failed")
            reply.update(status=failure.status, error_type=failure.kind)
    return reply, (
                130 if failure.kind == "cancelled" else 2 if failure.kind == "generation_not_authorized" else 1
            )


def main(stdin=None, stdout=None, env=None):
    stdin = sys.stdin.buffer if stdin is None else stdin
    stdout = sys.stdout if stdout is None else stdout
    env = os.environ if env is None else env
    try:
        raw = stdin.read(MAX_BYTES + 1)
        check(isinstance(raw, bytes) and len(raw) <= MAX_BYTES, "request_too_large")
        request = strict_json(raw)
        reply, code = run_adapter(request, raw, env)
    except AdapterFailure as error:
        reply = {"schema": SCHEMA, "status": error.status, "synthetic": False, "error_type": error.kind}
        code = 1
    except Exception:
        reply = {"schema": SCHEMA, "status": "FAILED", "synthetic": False, "error_type": "input_error"}
        code = 1
    # JSON escaping preserves all original source characters on Windows pipes.
    stdout.write(json.dumps(reply, ensure_ascii=True) + "\n")
    return code


if __name__ == "__main__":
    raise SystemExit(main())
