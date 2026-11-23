#!/usr/bin/env python3
"""Reviewed task-only identity launcher. No generation operation exists."""
import contextlib
import datetime
import hashlib
import http.client
import importlib.util
import io
import json
import os
from pathlib import Path
import re
import signal
import ssl
import subprocess
import sys
import time

SOURCE = Path("/mnt/development/projects/worktrees/cherri-p10r15-r23-20261123")
SOURCE_SHA = "dd6d1ed3eca4cb3a34fb24eaf5b5d7c715847426"
PROFILE = "fresh-six-exact-instances/v2"
MODEL = "claude-sonnet-5-5"
API_VERSION = "2026-10-21"
OUT = Path("/mnt/development/projects/evidence/cherri-p10r15-ai01-compat-20261123/model-identity-k9-attempt-01")
BODY_LIMIT = 65536
HTTP_SECONDS = 15
SAFE_ENV = {"PATH": "/usr/bin:/bin", "LANG": "C.UTF-8", "LC_ALL": "C.UTF-8"}
SUMMARY_KEYS = {"status", "httpStatus", "canonicalIdentityVerified", "receiptPath", "receiptSha256", "responseBytes", "responseSha256"}

def now():
    return datetime.datetime.now(datetime.timezone.utc).isoformat().replace("+00:00", "Z")

def sha(data):
    return hashlib.sha256(data).hexdigest()

def check(value, code):
    if not value:
        raise RuntimeError(code)

def secret(value):
    check(isinstance(value, str) and 16 <= len(value) <= 4096, "INVALID_CREDENTIAL")
    check(all(33 <= ord(c) <= 126 for c in value), "INVALID_CREDENTIAL")
    return value

def json_unique(data):
    def pairs(items):
        result = {}
        for k, v in items:
            check(k not in result, "DUPLICATE_JSON_KEY")
            result[k] = v
        return result
    def nonfinite(_):
        raise ValueError("NONFINITE_JSON")
    return json.loads(data, object_pairs_hook=pairs, parse_constant=nonfinite)

def reflected(data, credential):
    return any(part in data for part in (
        credential.encode("ascii"),
        json.dumps(credential, ensure_ascii=True)[1:-1].encode("ascii")))

def safe_summary(data, credential):
    check(isinstance(data, bytes) and len(data) <= 8192, "UNSAFE_SUMMARY")
    check(not reflected(data, credential), "UNSAFE_SUMMARY")
    obj = json_unique(data)
    check(isinstance(obj, dict) and set(obj) == SUMMARY_KEYS, "UNSAFE_SUMMARY")
    check(obj["status"] in ("MODEL_IDENTITY_VERIFIED", "MODEL_IDENTITY_FAILED"), "UNSAFE_SUMMARY")
    check(type(obj["canonicalIdentityVerified"]) is bool, "UNSAFE_SUMMARY")
    check(obj["httpStatus"] is None or type(obj["httpStatus"]) is int and 100 <= obj["httpStatus"] <= 599, "UNSAFE_SUMMARY")
    check(obj["receiptPath"] == str(OUT / "public-receipt.json"), "UNSAFE_SUMMARY")
    check(type(obj["responseBytes"]) is int and 0 <= obj["responseBytes"] <= BODY_LIMIT + 1, "UNSAFE_SUMMARY")
    check(all(isinstance(obj[k], str) and re.fullmatch(r"[0-9a-f]{64}", obj[k]) for k in ("receiptSha256", "responseSha256")), "UNSAFE_SUMMARY")
    return obj

def write_new(path, data):
    fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    with os.fdopen(fd, "wb") as stream:
        stream.write(data)

def encoded(value):
    return (json.dumps(value, sort_keys=True, indent=2, ensure_ascii=True) + "\n").encode("utf-8")

def clean_source():
    def git(*args):
        return subprocess.check_output(
            ["git", "-C", str(SOURCE), *args], env=SAFE_ENV,
            stderr=subprocess.DEVNULL, timeout=10)
    check(git("rev-parse", "HEAD").decode().strip() == SOURCE_SHA, "SOURCE_HEAD_MISMATCH")
    check(not git("status", "--porcelain"), "SOURCE_NOT_CLEAN")
    module_path = SOURCE / "tools/language-ai-eval/evaluation_bundle.py"
    check(module_path.read_bytes() == git("show", SOURCE_SHA + ":tools/language-ai-eval/evaluation_bundle.py"), "SOURCE_BYTES_MISMATCH")
    spec = importlib.util.spec_from_file_location("reviewed_evaluation_bundle", module_path)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    profile = module.reviewed_profile(PROFILE, require_compiler=True)
    check(module.MODEL == MODEL and module.API_VERSION == API_VERSION, "PROVIDER_PIN_MISMATCH")
    hashes = {}
    for name in module.V2_PRODUCER_FILES:
        path = "tools/language-ai-eval/" + name
        actual = module.read_regular(SOURCE / path)
        check(actual == git("show", SOURCE_SHA + ":" + path), "SOURCE_BYTES_MISMATCH")
        hashes[path] = sha(actual)
    check(len(hashes) == 7, "SOURCE_INVENTORY_MISMATCH")
    return module, hashes

def fetch_identity(credential):
    status, chunks, complete, error = None, [], False, None
    connection = http.client.HTTPSConnection("api.anthropic.com", 443, timeout=HTTP_SECONDS, context=ssl.create_default_context())
    old_handler = signal.getsignal(signal.SIGALRM)
    def deadline(_signum, _frame):
        raise TimeoutError("METADATA_DEADLINE")
    signal.signal(signal.SIGALRM, deadline)
    signal.setitimer(signal.ITIMER_REAL, HTTP_SECONDS)
    try:
        connection.request("GET", "/v1/models/" + MODEL, headers={
            "x-api-key": credential,
            "anthropic-version": API_VERSION,
            "accept": "application/json",
            "user-agent": "cherri-model-identity/v1"})
        response = connection.getresponse()
        status = response.status
        length = 0
        while length <= BODY_LIMIT:
            chunk = response.read1(min(8192, BODY_LIMIT + 1 - length))
            if not chunk:
                complete = True
                break
            chunks.append(chunk)
            length += len(chunk)
        if not complete:
            error = "RESPONSE_BODY_LIMIT"
    except TimeoutError:
        error = "TRANSPORT_TIMEOUT"
    except (OSError, http.client.HTTPException, ssl.SSLError):
        error = "TRANSPORT_OR_TLS_FAILURE"
    finally:
        signal.setitimer(signal.ITIMER_REAL, 0)
        signal.signal(signal.SIGALRM, old_handler)
        connection.close()
    return status, b"".join(chunks), complete, error

def identity():
    credential = secret(os.environ.get("ANTHROPIC_API_KEY"))
    check(os.getuid() == 1001, "WRONG_EXECUTION_USER")
    check(os.environ.get("CHERRI_AI_EVAL_ALLOW_GENERATION") == "0", "GENERATION_GUARD_MISSING")
    module, source_hashes = clean_source()
    OUT.mkdir(mode=0o700, parents=False, exist_ok=False)
    started = now()
    intent = {"schema": "cherri-model-identity-intent/v1", "operation": "PROVIDER_MODEL_IDENTITY_ONLY",
              "startedAtUtc": started, "method": "GET", "origin": "https://api.anthropic.com",
              "path": "/v1/models/" + MODEL, "apiVersion": API_VERSION, "requestedModelId": MODEL,
              "evaluationProfile": PROFILE, "implementationSource": SOURCE_SHA,
              "producerSha256": source_hashes, "helperSha256": sha(Path(__file__).read_bytes()),
              "httpDeadlineSeconds": HTTP_SECONDS, "responseByteLimit": BODY_LIMIT,
              "attempts": 1, "retries": 0, "followRedirects": False, "generationRequests": 0,
              "credentialPersistence": False}
    write_new(OUT / "request-intent.original.json", encoded(intent))
    status, body, complete, transport_error = fetch_identity(credential)
    finished = now()
    receipt = {"schema": "cherri-model-identity-receipt/v1",
               "operation": "PROVIDER_MODEL_IDENTITY_ONLY", "startedAtUtc": started,
               "finishedAtUtc": finished, "requestedModelId": MODEL, "apiVersion": API_VERSION,
               "httpStatus": status, "transportErrorType": transport_error, "parseErrorType": None,
               "canonicalIdentityVerified": False, "returnedModel": {},
               "responseSha256": sha(body), "responseBytes": len(body), "responseComplete": complete,
               "originalResponseRetained": False, "implementationSource": SOURCE_SHA,
               "evaluationProfile": PROFILE, "requestIntentSha256": sha(encoded(intent)),
               "generationRequests": 0, "attempts": 1, "retries": 0}
    if reflected(body, credential):
        receipt["parseErrorType"] = "CREDENTIAL_REFLECTION_BODY_WITHHELD"
    else:
        write_new(OUT / "response-body.original.bin", body)
        receipt["originalResponseRetained"] = True
        try:
            parsed = json_unique(body)
            check(isinstance(parsed, dict), "INVALID_MODEL_RESPONSE")
            model = {k: parsed.get(k) for k in ("id", "created_at", "lifecycle")}
            check(all(isinstance(v, str) for v in model.values()), "INVALID_MODEL_RESPONSE")
            receipt["returnedModel"] = model
            receipt["canonicalIdentityVerified"] = bool(status == 200 and complete and transport_error is None and model["id"] == MODEL and model["lifecycle"] == "active")
            module.validate_model_metadata(receipt)
        except Exception:
            receipt["canonicalIdentityVerified"] = False
            receipt["parseErrorType"] = "MODEL_METADATA_VALIDATION_FAILED"
    try:
        _, after_hashes = clean_source()
        check(after_hashes == source_hashes, "SOURCE_CHANGED_DURING_IDENTITY")
        receipt["sourceStable"] = True
    except Exception:
        receipt["sourceStable"] = False
        receipt["canonicalIdentityVerified"] = False
        receipt["transportErrorType"] = "SOURCE_CHANGED_DURING_IDENTITY"
    raw = encoded(receipt)
    check(not reflected(raw, credential), "RECEIPT_REFLECTION")
    write_new(OUT / "public-receipt.json", raw)
    result = {"status": "MODEL_IDENTITY_VERIFIED" if receipt["canonicalIdentityVerified"] else "MODEL_IDENTITY_FAILED",
              "httpStatus": status, "canonicalIdentityVerified": receipt["canonicalIdentityVerified"],
              "receiptPath": str(OUT / "public-receipt.json"), "receiptSha256": sha(raw),
              "responseBytes": len(body), "responseSha256": sha(body)}
    print(json.dumps(result, sort_keys=True))
    return 0 if receipt["canonicalIdentityVerified"] else 2

def bridge():
    raw = sys.stdin.buffer.read(8193)
    check(len(raw) <= 8192, "CREDENTIAL_ENVELOPE_TOO_LARGE")
    envelope = json_unique(raw)
    check(isinstance(envelope, dict) and set(envelope) == {"ANTHROPIC_API_KEY"}, "INVALID_CREDENTIAL_ENVELOPE")
    credential = secret(envelope["ANTHROPIC_API_KEY"])
    check(os.getuid() == 1001, "WRONG_EXECUTION_USER")
    child_env = dict(SAFE_ENV, ANTHROPIC_API_KEY=credential, CHERRI_AI_EVAL_ALLOW_GENERATION="0")
    helper_path = str(Path(__file__).resolve())
    helper_hash = sha(Path(helper_path).read_bytes())
    bootstrap = "import hashlib,pathlib,sys;p=pathlib.Path(sys.argv[1]);b=p.read_bytes();assert hashlib.sha256(b).hexdigest()==sys.argv[2];sys.argv=[str(p),'--identity'];exec(compile(b,str(p),'exec'),{'__name__':'__main__','__file__':str(p)})"
    completed = subprocess.run(
        [sys.executable, "-I", "-B", "-c", bootstrap, helper_path, helper_hash],
        stdin=subprocess.DEVNULL, stdout=subprocess.PIPE, stderr=subprocess.PIPE,
        env=child_env, timeout=75, check=False)
    result = safe_summary(completed.stdout, credential)
    check(not reflected(completed.stderr, credential), "CHILD_REFLECTION_SUPPRESSED")
    check(completed.returncode in (0, 2), "CHILD_EXECUTION_FAILED")
    print(json.dumps(result, sort_keys=True))
    return completed.returncode

def main():
    check(len(sys.argv) == 2 and sys.argv[1] in ("--bridge", "--identity"), "INVALID_OPERATION")
    return bridge() if sys.argv[1] == "--bridge" else identity()

if __name__ == "__main__":
    sys.dont_write_bytecode = True
    try:
        raise SystemExit(main())
    except Exception:
        print('{"status":"METADATA_LAUNCH_FAILED","detailsWithheld":true}')
        raise SystemExit(3) from None
