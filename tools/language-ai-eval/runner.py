#!/usr/bin/env python3
"""Equal-budget generation/edit evaluation orchestration; offline by default.

Adapters are explicit executable paths, speak JSON on stdin/stdout, and own
provider integration. Fake controls never generate AI-quality measurements.
"""

import argparse
import hashlib
import json
import math
import os
from pathlib import Path
import random
import re
import signal
import subprocess
import sys
import tempfile
import time


SCHEMA = "cherri-language-ai-eval/v1"
ARMS = ("legacy", "v2")
MAX_ADAPTER_BYTES = 1024 * 1024


def digest(data):
    return hashlib.sha256(data).hexdigest()


def regular_file(path):
    path = Path(path)
    if not path.is_file() or path.is_symlink():
        raise ValueError("expected a regular, non-symlink input file")
    return path.read_bytes()


def load_suite(path):
    raw = regular_file(path)
    suite = json.loads(raw)
    if not isinstance(suite, dict) or suite.get("schema") != SCHEMA or not isinstance(suite.get("tasks"), list):
        raise ValueError("invalid task-suite schema")
    tasks = suite["tasks"]
    if any(not isinstance(task, dict) for task in tasks):
        raise ValueError("each task must be an object")
    ids = [task.get("id") for task in tasks]
    if not tasks or any(not isinstance(i, str) or not i for i in ids) or len(set(ids)) != len(ids):
        raise ValueError("task IDs must be nonempty and unique")
    kinds = set()
    for task in tasks:
        kinds.add(task.get("kind"))
        if task.get("kind") not in ("generation", "edit") or not task.get("prompt"):
            raise ValueError("each task needs a generation/edit kind and prompt")
        if not isinstance(task.get("starter"), dict) or set(task["starter"]) != set(ARMS):
            raise ValueError("each task must define both arm inputs")
        if not isinstance(task.get("oracle"), dict) or not task["oracle"]:
            raise ValueError("each task needs a separate semantic oracle")
        if any(not re.fullmatch(r"[a-z][a-z0-9_]{0,63}", key) for key in task["oracle"]):
            raise ValueError("oracle keys must be bounded public check identifiers")
        if not isinstance(task["prompt"], str) or any(not isinstance(value, str) for value in task["starter"].values()):
            raise ValueError("model input fields must be strings")
    if kinds != {"generation", "edit"}:
        raise ValueError("both generation and edit tasks are required")
    return tasks, digest(raw)


def atomic_json(path, value):
    path = Path(path)
    path.parent.mkdir(parents=True, exist_ok=True)
    fd, temporary = tempfile.mkstemp(prefix=".ai-eval-", dir=path.parent)
    try:
        with os.fdopen(fd, "w", encoding="utf-8", newline="\n") as stream:
            json.dump(value, stream, ensure_ascii=False, indent=2)
            stream.write("\n")
            stream.flush()
            os.fsync(stream.fileno())
        os.replace(temporary, path)
    finally:
        if os.path.exists(temporary):
            os.unlink(temporary)


def run_process(command, request, timeout, originals=None):
    # No shell interpolation. stderr can contain provider credentials, so it is
    # hashed and counted, never included in the distributable receipt.
    with tempfile.TemporaryFile() as stdout, tempfile.TemporaryFile() as stderr:
        options = {"start_new_session": True} if os.name != "nt" else {"creationflags": subprocess.CREATE_NEW_PROCESS_GROUP}
        result = subprocess.Popen(command, stdin=subprocess.PIPE, stdout=stdout, stderr=stderr, **options)
        timed_out = None
        try:
            result.communicate(json.dumps(request).encode(), timeout=timeout)
        except subprocess.TimeoutExpired as error:
            if os.name == "nt":
                try:
                    subprocess.run(["taskkill", "/PID", str(result.pid), "/T", "/F"],
                                   stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, timeout=5, check=False)
                finally:
                    result.kill()
            else:
                os.killpg(result.pid, signal.SIGKILL)
            result.communicate()
            timed_out = error
        stdout.seek(0)
        output = stdout.read(MAX_ADAPTER_BYTES + 1)
        stderr.seek(0)
        diagnostic = stderr.read(MAX_ADAPTER_BYTES + 1)
        stdout.seek(0, 2)
        output_size = stdout.tell()
        stderr.seek(0, 2)
        diagnostic_size = stderr.tell()
    receipt = {"exit_code": result.returncode, "stderr_sha256": digest(diagnostic),
               "stderr_captured_bytes": len(diagnostic), "stdout_sha256": digest(output),
               "stdout_captured_bytes": len(output), "stdout_total_bytes": output_size,
               "stderr_total_bytes": diagnostic_size, "timed_out": timed_out is not None}
    if originals is not None:
        originals = Path(originals)
        originals.mkdir(parents=True, exist_ok=False)
        (originals / "request.json").write_bytes(json.dumps(request, sort_keys=True).encode())
        (originals / "stdout.captured").write_bytes(output)
        (originals / "stderr.captured").write_bytes(diagnostic)
        atomic_json(originals / "process.json", receipt)
    if timed_out is not None:
        raise timed_out
    if result.returncode != 0:
        raise ValueError("adapter process failed; stderr is intentionally not published")
    if len(output) > MAX_ADAPTER_BYTES:
        raise ValueError("adapter response exceeded the byte limit")
    return json.loads(output), receipt


def executable_command(path):
    return [sys.executable, str(path)] if str(path).endswith(".py") else [str(path)]


def validate_generation(reply, request):
    if not isinstance(reply, dict):
        raise ValueError("adapter response must be an object")
    if reply.get("schema") != SCHEMA or reply.get("request_id") != request["request_id"]:
        raise ValueError("adapter response identity mismatch")
    if reply.get("model") != request["model"]:
        raise ValueError("adapter changed the pinned model")
    code = reply.get("code")
    usage = reply.get("usage", {})
    if not isinstance(code, str) or not code.strip():
        raise ValueError("adapter returned no source code")
    if not isinstance(usage, dict):
        raise ValueError("adapter usage must be an object")
    for field in ("input_tokens", "output_tokens"):
        actual = usage.get(field)
        if type(actual) is not int or not 0 <= actual <= request["budget"]["max_" + field]:
            raise ValueError("adapter usage is missing, invalid, or over budget")
    return code, {field: usage[field] for field in ("input_tokens", "output_tokens")}


def required_checks(task):
    return ["compiles"] + ["oracle." + key for key in sorted(task["oracle"])]


def validate_semantics(reply, request, synthetic):
    if not isinstance(reply, dict):
        raise ValueError("validator response must be an object")
    for field in ("schema", "request_id", "task_id", "arm", "source_sha256"):
        if reply.get(field) != request[field]:
            raise ValueError("validator response identity mismatch")
    if reply.get("synthetic") is not synthetic:
        raise ValueError("validator evidence mode mismatch")
    checks = reply.get("checks")
    if not isinstance(checks, list) or not checks:
        raise ValueError("validator returned no checks")
    names = []
    clean = []
    for check in checks:
        if not isinstance(check, dict) or type(check.get("passed")) is not bool:
            raise ValueError("invalid validator check")
        name = check.get("name")
        if name not in request["required_checks"]:
            raise ValueError("validator returned an unknown check")
        names.append(name)
        clean.append({"name": name, "passed": check["passed"]})
    if len(set(names)) != len(names) or set(names) != set(request["required_checks"]):
        raise ValueError("validator did not cover every required oracle check exactly once")
    return clean


def model_request(task, arm, guide, model, budget, seed, suite_sha):
    request = {"schema": SCHEMA, "model": model, "budget": budget,
               "arm": arm, "task_id": task["id"], "kind": task["kind"],
               "prompt": task["prompt"], "starter": task["starter"][arm],
               "guide": guide, "seed": seed, "suite_sha256": suite_sha}
    request["request_id"] = digest(json.dumps(request, sort_keys=True).encode())
    return request


def evaluate(args):
    tasks, suite_sha = load_suite(args.suite)
    report = {"schema": SCHEMA, "suite_sha256": suite_sha,
              "mode": "fake-control" if args.fake else "external",
              "ai_quality_measured": False, "records": [],
              "budget": {"max_input_tokens": args.input_tokens,
                         "max_output_tokens": args.output_tokens},
              "model": args.model, "seed": args.seed}
    args.active_report = report
    if not args.fake and (not args.adapter or not args.validator or not args.model):
        report.update(status="AI_EVAL_NOT_RUN", reason="external_adapter_not_configured",
                      next_step="Configure explicit model adapter, independent validator, model ID, and both documentation snapshots; rerun with --allow-external.")
        return report, 2
    if not args.fake and not args.allow_external:
        raise ValueError("external execution requires the explicit --allow-external switch")
    bundle = None
    if getattr(args, "sealed_bundle", None):
        from evaluation_bundle import load_bundle, require, write_new, canonical, utc_now
        bundle = load_bundle(args.sealed_bundle)
        frozen = bundle["manifest"]
        require(not args.fake and args.model == frozen["provider"]["model"]
                and report["budget"] == frozen["budget"] and args.seed == frozen["order_seed"]
                and args.timeout == frozen["process_timeout_seconds"], "RUNNER_CONFIG_NOT_SEALED")
        require(args.suite.resolve() == bundle["root"] / "suite.json"
                and args.output.resolve() == bundle["root"] / "report.json", "RUNNER_PATH_NOT_SEALED")
        require(Path(args.adapter).resolve() == Path(__file__).with_name("anthropic_adapter.py").resolve()
                and Path(args.validator).resolve() == Path(__file__).with_name("graph_validator.py").resolve(), "RUNNER_PRODUCER_NOT_SEALED")
        require(os.environ.get("CHERRI_AI_EVAL_BUNDLE") == str(bundle["root"]), "RUNNER_BUNDLE_ENVIRONMENT")
        for arm in ARMS:
            require(Path(getattr(args, arm+"_guide")).resolve() == bundle["root"] / (arm+"-guide.md"), "RUNNER_GUIDE_NOT_SEALED")
        write_new(bundle["root"] / "runner-intent.json", canonical({"started_at": utc_now(),
                  "manifest_sha256": digest(canonical(frozen)), "anchor": bundle["anchor"],
                  "expected_outcomes": 12, "retry_limit": 0}))
        report.update(manifest_sha256=digest(canonical(frozen)), anchor=bundle["anchor"],
                      provenance_scope=frozen["provenance_scope"],
                      unseen_pattern_provenance_proved=False, apple_runtime_proved=False)
    guides = {arm: regular_file(getattr(args, arm + "_guide")).decode("utf-8")
              for arm in ARMS}
    report["guide_sha256"] = {arm: digest(text.encode()) for arm, text in guides.items()}
    if not args.fake:
        report["adapter_sha256"] = digest(regular_file(args.adapter))
        report["validator_sha256"] = digest(regular_file(args.validator))
    sequence = [(task, arm) for task in tasks for arm in ARMS]
    random.Random(args.seed).shuffle(sequence)
    report["records"] = [{"task_id": task["id"], "arm": arm,
                           "request_id": model_request(task, arm, guides[arm], args.model, report["budget"], args.seed, suite_sha)["request_id"],
                           "status": "NOT_STARTED"} for task, arm in sequence]
    report["status"] = "IN_PROGRESS"
    atomic_json(args.output, report)
    for position, (task, arm) in enumerate(sequence):
        request = model_request(task, arm, guides[arm], args.model, report["budget"], args.seed, suite_sha)
        identity = request["request_id"]
        started = time.monotonic()
        record = report["records"][position]
        if bundle:
            try:
                from evaluation_bundle import load_bundle, validate_model_request
                bundle = load_bundle(args.sealed_bundle)
                validate_model_request(request, bundle)
            except (OSError, ValueError, TypeError, subprocess.SubprocessError) as error:
                record.update(status="PRECONDITION_REJECTED", error_type=type(error).__name__)
                report.update(status="EVIDENCE_CHANGED", ai_quality_measured=False)
                atomic_json(args.output, report)
                return report, 1
        record.update(status="STARTED", request_sha256=digest(json.dumps(request, sort_keys=True).encode()))
        if bundle:
            from evaluation_bundle import utc_now
            record["started_at"] = utc_now()
        atomic_json(args.output, report)
        try:
            record.update(evaluate_case(args, task, arm, request, bundle))
        except (OSError, ValueError, TypeError, subprocess.TimeoutExpired) as error:
            if args.fake:
                raise
            # Preserve each failure as one outcome; do not retry or discard it.
            # The external adapter/validator keep their own original receipts.
            provider_done = bundle and (bundle["root"] / "attempts" / identity / "provider" / "reply.json").is_file()
            record.update(status="VALIDATOR_UNADJUDICATED" if provider_done else "PROVIDER_ERROR",
                          error_type=type(error).__name__)
            if bundle:
                try:
                    from evaluation_intake import verify_provider
                    outcome = verify_provider(bundle["root"] / "attempts" / identity / "provider", request, bundle)
                    if outcome["status"] == "MEASURED_PROVIDER_FAILURE":
                        record.update(status="MEASURED_PROVIDER_FAILURE", provider_status=outcome["provider_status"],
                                      usage=outcome["usage"], passed=False,
                                      provider_request_id_sha256=digest(outcome["provider_request_id"].encode()))
                        if "source_sha256" in outcome:
                            record["source_sha256"] = outcome["source_sha256"]
                    elif outcome["status"] != "GENERATED":
                        record["status"] = "PROVIDER_ERROR"
                except (OSError, ValueError, TypeError, KeyError):
                    record["status"] = "PROVIDER_ERROR"
        finally:
            record["duration_seconds"] = round(time.monotonic()-started, 6)
            if bundle:
                from evaluation_bundle import utc_now, seal_outcome
                record["finished_at"] = utc_now()
                record.update(seal_outcome(bundle, record))
            atomic_json(args.output, report)
    expected = {(task["id"], arm) for task in tasks for arm in ARMS}
    observed = {(r["task_id"], r["arm"]) for r in report["records"]}
    if expected != observed or len(report["records"]) != len(expected):
        raise ValueError("incomplete or duplicate task/arm coverage")
    if args.fake:
        report["status"] = "HARNESS_CONTROL_PASS"
    else:
        if digest(regular_file(args.adapter)) != report["adapter_sha256"] or digest(regular_file(args.validator)) != report["validator_sha256"]:
            report.update(status="EVIDENCE_CHANGED", ai_quality_measured=False)
            return report, 1
        if bundle:
            from evaluation_bundle import load_bundle
            load_bundle(args.sealed_bundle)
        complete = all(record["status"] in {"MEASURED", "MEASURED_PROVIDER_FAILURE"} for record in report["records"])
        report["status"] = "EVALUATION_COMPLETED" if complete else "EVALUATION_INCOMPLETE"
        report["ai_quality_measured"] = complete
        report["outcome_accounting"] = {arm: {"total": len(tasks),
            **{status: sum(r["arm"] == arm and r["status"] == status for r in report["records"])
               for status in ("MEASURED", "MEASURED_PROVIDER_FAILURE", "PROVIDER_ERROR", "VALIDATOR_UNADJUDICATED", "NOT_STARTED")}}
            for arm in ARMS}
        if complete:
            report["measured_results"] = {arm: {"passed": sum(r["passed"] for r in report["records"] if r["arm"] == arm),
                                                 "total": len(tasks)} for arm in ARMS}
        else:
            return report, 1
    return report, 0


def evaluate_case(args, task, arm, request, bundle):
        identity = request["request_id"]
        original = (bundle["root"] / "runner-processes" / identity) if bundle else None
        if args.fake:
            reply, process = run_process([sys.executable, __file__, "--fake-adapter"], request, args.timeout)
            if not isinstance(reply, dict) or reply.get("synthetic") is not True:
                raise ValueError("fake adapter evidence mode mismatch")
            process["synthetic"] = True
        else:
            reply, process = run_process(executable_command(args.adapter), request, args.timeout,
                                         original / "provider" if original else None)
            if not isinstance(reply, dict) or reply.get("synthetic") is not False or not isinstance(reply.get("provider_request_id"), str) or not reply["provider_request_id"].strip():
                raise ValueError("external adapter must identify an actual provider request")
        code, usage = validate_generation(reply, request)
        record = {"task_id": task["id"], "arm": arm, "request_id": identity,
                  "request_sha256": digest(json.dumps(request, sort_keys=True).encode()),
                  "source_sha256": digest(code.encode()), "usage": usage,
                  "process": process}
        validation_request = {"schema": SCHEMA, "request_id": identity,
                              "task_id": task["id"], "arm": arm,
                              "source": code, "source_sha256": digest(code.encode()),
                              "oracle": task["oracle"], "required_checks": required_checks(task)}
        command = [sys.executable, __file__, "--fake-validator"] if args.fake else executable_command(args.validator)
        validation, validation_process = run_process(command, validation_request, args.timeout,
                                                     original / "validator" if original else None)
        checks = validate_semantics(validation, validation_request, synthetic=args.fake)
        if args.fake:
            validation_process["synthetic"] = True
            record.update(status="HARNESS_CONTROL_ONLY", control_checks=checks,
                          validator_process=validation_process)
        else:
            record.update(status="MEASURED", checks=checks, validator_process=validation_process,
                          provider_request_id_sha256=digest(reply["provider_request_id"].encode()),
                          passed=all(c["passed"] for c in checks))
        return record


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--suite", type=Path, default=Path(__file__).with_name("tasks.json"))
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--legacy-guide", type=Path)
    parser.add_argument("--v2-guide", type=Path)
    parser.add_argument("--adapter")
    parser.add_argument("--validator")
    parser.add_argument("--model", default="")
    parser.add_argument("--fake", action="store_true")
    parser.add_argument("--allow-external", action="store_true")
    parser.add_argument("--input-tokens", type=int, default=12000)
    parser.add_argument("--output-tokens", type=int, default=2000)
    parser.add_argument("--timeout", type=float, default=120)
    parser.add_argument("--seed", type=int, default=0)
    parser.add_argument("--sealed-bundle", type=Path)
    args = parser.parse_args(argv)
    output = os.path.normcase(str(args.output.resolve()))
    inputs = (Path(__file__), args.suite, args.legacy_guide, args.v2_guide, args.adapter, args.validator)
    if args.output.is_symlink() or any(p and output == os.path.normcase(str(Path(p).resolve())) for p in inputs):
        print("Output cannot replace an input or symlink.", file=sys.stderr)
        return 1
    try:
        if not math.isfinite(args.timeout) or min(args.input_tokens, args.output_tokens, args.timeout) <= 0 or args.timeout > 3600:
            raise ValueError("budgets and timeout must be positive")
        report, code = evaluate(args)
    except (OSError, ValueError, TypeError, subprocess.SubprocessError) as error:
        # Keep already-started outcomes even when final integrity validation
        # fails. A stale report from a previous invocation is never loaded.
        report = getattr(args, "active_report", {"schema": SCHEMA})
        report.update(status="HARNESS_FAILED", ai_quality_measured=False,
                      error_type=type(error).__name__, reason="invalid configuration, input, or adapter evidence; no score produced")
        report.pop("measured_results", None)
        code = 1
    atomic_json(args.output, report)
    print(json.dumps({"status": report["status"], "ai_quality_measured": report["ai_quality_measured"]}))
    return code


if __name__ == "__main__":
    if sys.argv[1:] == ["--fake-adapter"]:
        request = json.load(sys.stdin)
        print(json.dumps({"schema": SCHEMA, "request_id": request["request_id"],
                          "model": request["model"], "synthetic": True,
                          "code": 'show("HARNESS_CONTROL_ONLY")\n',
                          "usage": {"input_tokens": 1, "output_tokens": 1}}))
    elif sys.argv[1:] == ["--fake-validator"]:
        request = json.load(sys.stdin)
        response = {key: request[key] for key in ("schema", "request_id", "task_id", "arm", "source_sha256")}
        response.update(synthetic=True, checks=[{"name": name, "passed": True} for name in request["required_checks"]])
        print(json.dumps(response))
    else:
        raise SystemExit(main())
