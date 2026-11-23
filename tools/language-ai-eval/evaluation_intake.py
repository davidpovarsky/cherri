#!/usr/bin/env python3
"""Recompute a sealed twelve-outcome evaluation from original host receipts.

The trusted Asana input MUST be a fresh authenticated connector/API reread,
collected independently of this bundle. This offline checker cannot authenticate
a caller-supplied JSON file as a server response. Its output states that boundary.
No original payload, seed, credential or provider request ID is published here.
"""

import argparse
import datetime as dt
from email.utils import parsedate_to_datetime
import json
import math
from pathlib import Path
import plistlib
import subprocess
import sys

from evaluation_bundle import (API_VERSION, MODEL, PROFILE, V2_PROFILE, SCHEMA,
    BundleError, canonical, digest, load_bundle, read_regular, request_sequence,
    require, reviewed_profile, unique_json, utc_time, validate_anchor, verify_outcome_seal, write_new)
from graph_validator import COMPILE_TIMEOUT, MAX_PLIST, semantic_checks, validation_request

EXPECTED_SYSTEM = (
    "Write Cherri source for the supplied language arm, guide, and task. "
    "Return only the complete source text, without Markdown fences or explanations. "
    "Use only the supplied language guide. Do not use tools."
)


def original(path, expected=None, limit=4*1024*1024, *, require_complete=True):
    raw = read_regular(path, limit)
    if expected is not None:
        require(isinstance(expected, dict) and expected.get("sha256") == digest(raw)
                and type(expected.get("bytes")) is int and expected.get("bytes") == len(raw)
                and type(expected.get("complete", True)) is bool
                and (not require_complete or expected.get("complete", True) is True),
                "ORIGINAL_HASH_OR_SIZE")
    return raw


def inventory(directory, receipt, *, require_complete=True):
    expected = receipt.get("originals")
    require(isinstance(expected, dict) and expected, "ORIGINAL_INVENTORY")
    for name, info in expected.items():
        require(isinstance(name, str) and Path(name).name == name and name not in (".", ".."), "ORIGINAL_PATH")
        original(directory / name, info, require_complete=require_complete)
    return expected


def usage_values(usage):
    require(isinstance(usage, dict), "USAGE_MISSING")
    result = {}
    for name in ("input_tokens", "output_tokens", "cache_creation_input_tokens", "cache_read_input_tokens"):
        value = usage.get(name, 0) if name.startswith("cache_") else usage.get(name)
        if name.startswith("cache_") and value is None:
            value = 0
        require(type(value) is int and value >= 0, "USAGE_TYPE")
        result[name] = value
    result["total_input_tokens"] = result["input_tokens"] + result["cache_creation_input_tokens"] + result["cache_read_input_tokens"]
    require(not any(key.endswith("_tokens") and key not in result for key in usage), "UNDECLARED_TOKEN_COMPONENT")
    if usage.get("cache_creation") is not None:
        creation = usage["cache_creation"]
        require(isinstance(creation, dict) and set(creation) <= {"ephemeral_1h_input_tokens", "ephemeral_5m_input_tokens"}
                and all(type(value) is int and value >= 0 for value in creation.values())
                and sum(creation.values()) == result["cache_creation_input_tokens"], "CACHE_COMPONENT_MISMATCH")
    details = usage.get("output_tokens_details")
    require(details is None or isinstance(details, dict) and set(details) <= {"thinking_tokens"}
            and type(details.get("thinking_tokens", 0)) is int and details.get("thinking_tokens", 0) == 0, "UNDECLARED_THINKING_USAGE")
    tools = usage.get("server_tool_use")
    require(tools is None or isinstance(tools, dict) and all(type(value) is int and value == 0 for value in tools.values()), "UNDECLARED_TOOL_USAGE")
    require(usage.get("iterations") in (None, []) and usage.get("fallback_credit") is None, "UNDECLARED_PROVIDER_ITERATIONS")
    return result


def ordered_times(*values):
    times = [utc_time(value) for value in values]
    require(all(a <= b for a, b in zip(times, times[1:])), "RECEIPT_CHRONOLOGY")


def verify_provider(directory, request, bundle):
    receipt = unique_json(original(directory / "provider-receipt.json"))
    require(receipt.get("schema") == "cherri-anthropic-provider/v1", "PROVIDER_RECEIPT_SCHEMA")
    profile = bundle["manifest"].get("profile")
    selected = reviewed_profile(profile)
    require(bundle["manifest"]["provider"] == selected["provider"], "PROVIDER_SETTINGS")
    require("evaluation_profile" not in receipt if profile == PROFILE else
            receipt.get("evaluation_profile") == profile, "PROVIDER_PROFILE")
    for key in ("request_id", "task_id", "arm"):
        require(receipt.get(key) == request[key], "PROVIDER_RECEIPT_IDENTITY")
    require(receipt.get("implementation_sha") == bundle["manifest"]["implementation_sha"]
            and receipt.get("adapter_sha256") == bundle["manifest"]["producer_sha256"]["anthropic_adapter.py"], "PROVIDER_SOURCE_IDENTITY")
    require(receipt.get("expected_model") == MODEL and receipt.get("api_version") == API_VERSION, "PROVIDER_MODEL_IDENTITY")
    provider_status = receipt.get("status")
    completed = provider_status in {"GENERATED", "REFUSED", "TRUNCATED"}
    # A transport timeout may retain a valid, hash-bound prefix of a response.
    # Such bytes remain incomplete evidence and must not erase the attempt.
    originals = inventory(directory, receipt, require_complete=completed)
    require(unique_json(original(directory / "request.json")) == request, "PROVIDER_APPROVED_REQUEST_MISMATCH")
    require(unique_json(original(directory / "adapter-request.original.json")) == request, "PROVIDER_STDIN_MISMATCH")
    intent = unique_json(original(directory / "intent.json"))
    require(intent.get("request_id") == request["request_id"] and intent.get("stage") == "provider"
            and intent.get("manifest_sha256") == digest(canonical(bundle["manifest"]))
            and intent.get("anchor") == bundle["anchor"], "PROVIDER_INTENT")
    ordered_times(bundle["anchor"]["created_at"], intent["started_at"], receipt["started_at"])
    started_at = receipt["started_at"]
    finished_at = receipt.get("finished_at")
    if finished_at is not None:
        ordered_times(started_at, finished_at)
    # Non-generated outcomes remain in the denominator. A STARTED/forced-kill
    # receipt is incomplete evidence, never a successful generation.
    if not completed:
        require(receipt.get("status") in {"STARTED", "COUNT_STARTED", "GENERATION_STARTED", "REFUSED", "TRUNCATED", "CANCELLED", "FAILED"}, "PROVIDER_STATUS")
        return {"status": "PROVIDER_ERROR", "provider_status": receipt["status"],
                "started_at": started_at, "finished_at": finished_at,
                "receipt_sha256": digest(canonical(receipt))}
    stop_reason = {"GENERATED": "end_turn", "REFUSED": "refusal", "TRUNCATED": "max_tokens"}[provider_status]
    require(receipt.get("actual_model") == MODEL and receipt.get("stop_reason") == stop_reason, "ACTUAL_MODEL_OR_COMPLETION")
    ordered_times(receipt["started_at"], receipt["finished_at"])
    calls = receipt.get("calls", {})
    require(set(calls) == {"count", "generation"}, "PROVIDER_CALL_INVENTORY")
    public_payload = {key: request[key] for key in ("arm", "kind", "prompt", "starter", "guide")}
    bodies = {}
    for name in ("count", "generation"):
        call = calls[name]
        require(call.get("attempted") is True and call.get("http_status") == 200
                and call.get("response_complete") is True, "PROVIDER_CALL_INCOMPLETE")
        ordered_times(receipt["started_at"], call["started_at"], call["finished_at"], receipt["finished_at"])
        raw_request = original(directory / (name+".request.original.json"), originals.get(name+".request.original.json"))
        require(digest(raw_request) == call.get("request_sha256"), "PROVIDER_HTTP_REQUEST_HASH")
        body = unique_json(raw_request)
        fields = {"model", "system", "messages", "thinking"} | ({"max_tokens", "stream"} if name == "generation" else set())
        if profile == V2_PROFILE:
            fields.add("output_config")
        require(set(body) == fields and body.get("model") == MODEL
                and body.get("thinking") == selected["provider"]["settings"]["thinking"], "PROVIDER_HTTP_FIELDS")
        if profile == V2_PROFILE:
            require(body.get("output_config") == selected["provider"]["settings"]["output_config"], "PROVIDER_HTTP_FIELDS")
        require(body.get("system") == EXPECTED_SYSTEM
                and receipt.get("system_sha256") == digest(EXPECTED_SYSTEM.encode()), "PROVIDER_SYSTEM_PROMPT")
        messages = body.get("messages")
        require(isinstance(messages, list) and len(messages) == 1 and set(messages[0]) == {"role", "content"}
                and messages[0]["role"] == "user", "PROVIDER_ISOLATED_MESSAGE")
        content = messages[0]["content"]
        require(isinstance(content, str), "PROVIDER_PUBLIC_TEXT_ENVELOPE")
        require(unique_json(content) == public_payload, "PROVIDER_ORACLE_OR_PRIVATE_INPUT_LEAK")
        if name == "generation":
            require(body.get("max_tokens") == bundle["manifest"]["budget"]["max_output_tokens"]
                    and body.get("stream") is False, "PROVIDER_OUTPUT_CAP")
        require(isinstance(call.get("response_date"), str) and call["response_date"], "PROVIDER_SERVER_DATE_MISSING")
        if call.get("response_date"):
            try:
                server = parsedate_to_datetime(call["response_date"])
            except (TypeError, ValueError) as error:
                raise BundleError("PROVIDER_SERVER_DATE") from error
            require(server.tzinfo is not None and server + dt.timedelta(seconds=1) > utc_time(bundle["anchor"]["created_at"]), "PROVIDER_PRECEDES_ANCHOR")
        bodies[name] = body
    require({k: v for k, v in bodies["generation"].items() if k not in ("max_tokens", "stream")} == bodies["count"], "COUNT_GENERATION_INPUT_CHANGED")
    ordered_times(calls["count"]["finished_at"], calls["generation"]["started_at"])
    count = unique_json(original(directory / "count.response.original.body", originals.get("count.response.original.body")))
    require(type(count.get("input_tokens")) is int and 0 <= count["input_tokens"] <= bundle["manifest"]["budget"]["max_input_tokens"], "PREFLIGHT_CAP")
    require(receipt.get("provider_preflight_input_tokens") == count["input_tokens"] and receipt.get("preflight_is_actual_usage") is False, "PREFLIGHT_IS_NOT_ACTUAL_USAGE")
    generated = unique_json(original(directory / "generation.response.original.body", originals.get("generation.response.original.body")))
    require(generated.get("model") == MODEL and generated.get("role") == "assistant"
            and generated.get("stop_reason") == stop_reason and generated.get("type") == "message", "PROVIDER_ORIGINAL_RESPONSE_IDENTITY")
    require(generated.get("stop_sequence") is None and generated.get("container") is None
            and generated.get("input_transformations") in (None, [])
            and generated.get("context_management") in (None, {"applied_edits": []}), "UNDECLARED_PROVIDER_STATE")
    content = generated.get("content")
    require(isinstance(content, list) and all(isinstance(block, dict) and block.get("type") == "text"
            and isinstance(block.get("text"), str) for block in content), "PROVIDER_UNADJUDICABLE_CONTENT_BLOCK")
    plain = isinstance(content, list) and content and all(isinstance(block, dict) and block.get("type") == "text"
            and isinstance(block.get("text"), str) for block in content)
    code = "".join(block["text"] for block in content) if plain else None
    if provider_status == "GENERATED":
        require(code is not None and code.strip(), "PROVIDER_ORIGINAL_CONTENT")
    if code is not None:
        require(original(directory / "generated-source.original.cherri") == code.encode()
                and receipt.get("source_sha256") == digest(code.encode()), "PROVIDER_SOURCE_CHANGED")
    else:
        require("source_sha256" not in receipt and not (directory / "generated-source.original.cherri").exists(), "UNOBSERVED_PROVIDER_SOURCE")
    actual_usage = usage_values(generated.get("usage"))
    require(actual_usage == receipt.get("usage_counts") and generated["usage"] == receipt.get("usage_original"), "USAGE_ORIGINAL_MISMATCH")
    require(actual_usage["total_input_tokens"] <= bundle["manifest"]["budget"]["max_input_tokens"]
            and actual_usage["output_tokens"] <= bundle["manifest"]["budget"]["max_output_tokens"], "ACTUAL_TOKEN_CAP")
    require(actual_usage["cache_creation_input_tokens"] == 0 and actual_usage["cache_read_input_tokens"] == 0, "UNDECLARED_CACHE_USAGE")
    require(receipt.get("provider_message_id") == generated.get("id"), "PROVIDER_MESSAGE_ID")
    request_id = calls["generation"].get("provider_request_id")
    require(isinstance(request_id, str) and request_id and isinstance(generated.get("id"), str) and generated["id"], "PROVIDER_IDS_MISSING")
    reply = unique_json(original(directory / "reply.json"))
    if provider_status in {"REFUSED", "TRUNCATED"}:
        require(reply.get("schema") == SCHEMA and reply.get("request_id") == request["request_id"]
                and reply.get("model") == MODEL and reply.get("synthetic") is False
                and reply.get("status") == provider_status and "code" not in reply, "PROVIDER_FAILURE_REPLY_IDENTITY")
        result = {"status": "MEASURED_PROVIDER_FAILURE", "provider_status": provider_status,
                  "usage": {"input_tokens": actual_usage["total_input_tokens"], "output_tokens": actual_usage["output_tokens"]},
                  "provider_request_id": request_id, "passed": False,
                  "started_at": started_at, "finished_at": receipt["finished_at"],
                  "receipt_sha256": digest(canonical(receipt))}
        if code is not None:
            result["source_sha256"] = digest(code.encode())
        return result
    require(reply.get("schema") == SCHEMA and reply.get("request_id") == request["request_id"]
            and reply.get("model") == MODEL and reply.get("code") == code and reply.get("synthetic") is False
            and reply.get("provider_request_id") == request_id
            and reply.get("usage") == {"input_tokens": actual_usage["total_input_tokens"], "output_tokens": actual_usage["output_tokens"]}, "PROVIDER_REPLY_NOT_ORIGINAL")
    return {"status": "GENERATED", "code": code, "usage": reply["usage"], "provider_request_id": request_id,
            "started_at": started_at, "finished_at": receipt["finished_at"],
            "receipt_sha256": digest(canonical(receipt))}


def verify_validator(directory, request, provider, bundle):
    receipt = unique_json(original(directory / "validator-receipt.json"))
    require(receipt.get("schema") == "cherri-ai-graph-validation/v1" and receipt.get("synthetic") is False
            and receipt.get("apple_runtime_proved") is False and receipt.get("semantic_scope") == "six-task-emitted-pure-graph", "VALIDATOR_SCOPE")
    for key in ("request_id", "task_id", "arm"):
        require(receipt.get(key) == request[key], "VALIDATOR_IDENTITY")
    require(receipt.get("compiler") == bundle["manifest"]["compiler"], "VALIDATOR_COMPILER_IDENTITY")
    profile = bundle["manifest"].get("profile")
    selected = reviewed_profile(profile)
    require("evaluation_profile" not in receipt if profile == PROFILE else
            receipt.get("evaluation_profile") == profile, "VALIDATOR_PROFILE")
    require(receipt.get("compiler_environment_profile") == selected["compiler_environment_profile"], "VALIDATOR_ENVIRONMENT_PROFILE")
    require(receipt.get("originals_complete", True) is True, "VALIDATOR_ORIGINALS_INCOMPLETE")
    inventory(directory, receipt)
    validated = unique_json(original(directory / "request.json"))
    task = validation_request(validated, bundle)
    source = original(directory / "source.cherri")
    require(source == provider["code"].encode() and validated["source"] == provider["code"]
            and receipt.get("source_sha256") == digest(source), "VALIDATOR_SOURCE_MISMATCH")
    ordered_times(provider["finished_at"], receipt["started_at"], receipt["finished_at"])
    if receipt.get("status") == "UNADJUDICATED":
        return {"status": "VALIDATOR_UNADJUDICATED", "finished_at": receipt["finished_at"],
                "receipt_sha256": digest(canonical(receipt))}
    command = [str(bundle["root"] / "compiler.exe"), "compile", str(directory / "source.cherri"),
               "--mode="+request["arm"], "--output", str(directory / "output.shortcut"), "--skip-sign"]
    elapsed = receipt.get("compile_elapsed_seconds")
    require(receipt.get("command") == command and type(elapsed) in (int, float)
            and math.isfinite(elapsed) and 0 <= elapsed <= COMPILE_TIMEOUT + 1, "VALIDATOR_COMMAND_OR_BUDGET")
    if receipt.get("status") == "COMPILE_REJECTED":
        require(type(receipt.get("compile_returncode")) is int and receipt["compile_returncode"] != 0, "COMPILE_REJECTION_RECEIPT")
        checks = [{"name": name, "passed": False} for name in validated["required_checks"]]
    else:
        require(receipt.get("status") == "ADJUDICATED" and type(receipt.get("compile_returncode")) is int
                and receipt["compile_returncode"] == 0, "VALIDATOR_STATUS")
        raw = original(directory / "output.shortcut", limit=MAX_PLIST)
        require(digest(raw) == receipt.get("graph_sha256"), "GRAPH_HASH")
        checks = semantic_checks(plistlib.loads(raw), task, bundle["parameters"].get(task["id"], {}))
    reply = unique_json(original(directory / "reply.json"))
    expected = {key: validated[key] for key in ("schema", "request_id", "task_id", "arm", "source_sha256")}
    expected.update(synthetic=False, checks=checks)
    require(reply == expected and digest(canonical(reply)) == receipt.get("reply_sha256"), "VALIDATOR_CHECKS_NOT_RECOMPUTED")
    return {"status": "MEASURED", "checks": checks, "passed": all(item["passed"] for item in checks),
            "finished_at": receipt["finished_at"], "receipt_sha256": digest(canonical(receipt))}


def intake(bundle_path, trusted_asana_response):
    bundle = load_bundle(bundle_path)
    # This argument is deliberately outside the bundle. The operator obtains it
    # via an authenticated Asana read immediately before invoking this checker.
    trusted_path = Path(trusted_asana_response).resolve()
    require(not trusted_path.is_relative_to(bundle["root"].resolve()), "TRUSTED_ANCHOR_MUST_BE_INDEPENDENT")
    trusted_raw = original(trusted_path)
    require(validate_anchor(unique_json(trusted_raw), bundle) == bundle["anchor"], "LIVE_ANCHOR_CHANGED")
    root = Path(__file__).resolve().parents[2]
    for name in reviewed_profile(bundle["manifest"]["profile"])["producer_files"]:
        data = subprocess.check_output(["git", "--no-replace-objects", "show",
              bundle["manifest"]["implementation_sha"]+":tools/language-ai-eval/"+name], cwd=root, timeout=15)
        require(digest(data) == bundle["manifest"]["producer_sha256"][name], "FROZEN_GIT_SOURCE_BINDING")
    report = unique_json(original(bundle["root"] / "report.json"))
    require(report.get("schema") == SCHEMA and report.get("mode") == "external"
            and report.get("model") == MODEL and report.get("manifest_sha256") == digest(canonical(bundle["manifest"]))
            and report.get("suite_sha256") == bundle["manifest"]["suite_sha256"]
            and report.get("budget") == bundle["manifest"]["budget"] and report.get("seed") == bundle["manifest"]["order_seed"], "RUNNER_REPORT_IDENTITY")
    sequence = request_sequence(bundle)
    records = report.get("records")
    require(isinstance(records, list) and len(records) == 12, "TWELVE_OUTCOME_INVENTORY")
    expected_ids = [request["request_id"] for request in sequence]
    require([record.get("request_id") for record in records] == expected_ids, "OUTCOME_ORDER_OR_IDENTITY")
    attempts = bundle["root"] / "attempts"
    require(attempts.is_dir() and {path.name for path in attempts.iterdir()} <= set(expected_ids), "UNDECLARED_OR_RETRIED_REQUEST")
    results = []
    provider_ids = set()
    provider_starts = []
    evidence_finishes = []
    all_started_providers_finished = True
    for request, record in zip(sequence, records):
        verify_outcome_seal(bundle, record)
        require(record.get("task_id") == request["task_id"] and record.get("arm") == request["arm"], "OUTCOME_CASE_IDENTITY")
        directory = attempts / request["request_id"]
        require(directory.is_dir() and (directory / "provider").is_dir(), "MISSING_STARTED_OUTCOME")
        provider = verify_provider(directory / "provider", request, bundle)
        provider_starts.append(utc_time(provider["started_at"]))
        if provider["finished_at"] is None:
            all_started_providers_finished = False
        else:
            evidence_finishes.append(utc_time(provider["finished_at"]))
        result = {"task_id": request["task_id"], "arm": request["arm"], "request_id": request["request_id"],
                  "status": provider["status"], "provider_receipt_sha256": provider["receipt_sha256"]}
        if provider["status"] == "GENERATED":
            require(provider["provider_request_id"] not in provider_ids, "REUSED_PROVIDER_REQUEST_ID")
            provider_ids.add(provider["provider_request_id"])
            validated = verify_validator(directory / "validator", request, provider, bundle)
            evidence_finishes.append(utc_time(validated["finished_at"]))
            result.update(status=validated["status"], validator_receipt_sha256=validated["receipt_sha256"],
                          source_sha256=digest(provider["code"].encode()), usage=provider["usage"],
                          provider_request_id_sha256=digest(provider["provider_request_id"].encode()))
            if validated["status"] == "MEASURED":
                require(record.get("checks") == validated["checks"] and record.get("passed") is validated["passed"]
                        and record.get("source_sha256") == result["source_sha256"] and record.get("usage") == result["usage"]
                        and record.get("provider_request_id_sha256") == result["provider_request_id_sha256"], "RUNNER_MEASUREMENT_CHANGED")
                result.update(checks=validated["checks"], passed=validated["passed"])
        elif provider["status"] == "MEASURED_PROVIDER_FAILURE":
            require(not (directory / "validator").exists(), "PROVIDER_FAILURE_CANNOT_BORROW_VALIDATION")
            require(provider["provider_request_id"] not in provider_ids, "REUSED_PROVIDER_REQUEST_ID")
            provider_ids.add(provider["provider_request_id"])
            result.update(provider_status=provider["provider_status"], usage=provider["usage"], passed=False,
                          provider_request_id_sha256=digest(provider["provider_request_id"].encode()))
            if "source_sha256" in provider:
                result["source_sha256"] = provider["source_sha256"]
            require(record.get("provider_status") == result["provider_status"] and record.get("usage") == result["usage"]
                    and record.get("passed") is False and "checks" not in record
                    and record.get("provider_request_id_sha256") == result["provider_request_id_sha256"], "PROVIDER_FAILURE_OUTCOME_CHANGED")
        require(record.get("status") == result["status"], "OUTCOME_STATUS_MISMATCH")
        results.append(result)
    complete = all(result["status"] in {"MEASURED", "MEASURED_PROVIDER_FAILURE"} for result in results)
    require(report.get("ai_quality_measured") is complete
            and report.get("status") == ("EVALUATION_COMPLETED" if complete else "EVALUATION_INCOMPLETE"), "QUALITY_COMPLETION_CLAIM")
    summary = {"schema": "cherri-ai-evaluation-intake/v1", "status": "VERIFIED_MEASUREMENT" if complete else "VERIFIED_INCOMPLETE_OUTCOMES",
        "manifest_sha256": digest(canonical(bundle["manifest"])), "implementation_sha": bundle["manifest"]["implementation_sha"],
        "compiler": bundle["manifest"]["compiler"], "suite_sha256": bundle["manifest"]["suite_sha256"],
        "trusted_asana_response_sha256": digest(trusted_raw), "anchor": bundle["anchor"],
        # Original receipt times bound all validated count/generation calls and
        # any validator execution. An unfinished attempt has no claimed end.
        "execution_interval": {
            "started_utc": min(provider_starts).isoformat().replace("+00:00", "Z"),
            "finished_utc": max(evidence_finishes).isoformat().replace("+00:00", "Z")
                if all_started_providers_finished else None},
        "asana_authentication_boundary": "independent authenticated connector read supplied by operator",
        "ai_quality_measured": complete, "expected_outcomes": 12, "records": results,
        "provenance_scope": "fresh exact instances of six existing public patterns",
        "unseen_pattern_provenance_proved": False, "apple_runtime_proved": False,
        "formal_acceptance_granted": False}
    if complete:
        measured = {arm: {"passed": sum(r["passed"] for r in results if r["arm"] == arm), "total": 6} for arm in ("legacy", "v2")}
        require(report.get("measured_results") == measured, "MEASURED_DENOMINATOR_OR_SCORE")
        summary["measured_results"] = measured
    return summary


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--bundle", type=Path, required=True)
    parser.add_argument("--trusted-asana-response", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    try:
        result = intake(args.bundle, args.trusted_asana_response)
        write_new(args.output, canonical(result))
        print(json.dumps({"status": result["status"], "ai_quality_measured": result["ai_quality_measured"]}))
        return 0
    except (OSError, ValueError, KeyError, TypeError, subprocess.SubprocessError, plistlib.InvalidFileException):
        print("EVALUATION_INTAKE_REJECTED", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
