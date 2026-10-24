#!/usr/bin/env python3
"""Authenticated Kilo smoke test. Never print or persist credentials or raw responses."""
import json
import os
import pathlib
import sys
import urllib.error
import urllib.request

ROOT = "https://ai-gateway.kilo.ai/api/v1"
MODEL = os.environ.get("KILO_MODEL", "nvidia/nemotron-3-super-120b-a12b:free")
KEY = os.environ.get("KILO_API_KEY", "")
OUT = pathlib.Path("artifacts/lumen-api-result.json")
OUT.parent.mkdir(parents=True, exist_ok=True)
result = {"model": MODEL, "catalog": False, "http_status": None, "has_assistant_text": False, "passed": False}

def finish(reason):
    result["reason"] = reason
    OUT.write_text(json.dumps(result, indent=2) + "\n")
    print("LUMEN_KILO_SMOKE=" + ("PASS" if result["passed"] else "FAIL"))
    print("reason=" + reason)
    print("http_status=" + str(result["http_status"]))
    sys.exit(0 if result["passed"] else 1)

if not KEY:
    finish("missing_repository_secret_LUMEN_KILO_API_KEY")
try:
    with urllib.request.urlopen(ROOT + "/models", timeout=25) as resp:
        models = json.load(resp).get("data", [])
    result["catalog"] = any(isinstance(m, dict) and m.get("id") == MODEL for m in models)
except (urllib.error.URLError, ValueError, KeyError):
    finish("model_catalog_unavailable")
if not result["catalog"]:
    finish("model_not_in_catalog")
body = json.dumps({"model": MODEL, "messages": [{"role": "user", "content": "Reply with the single word PONG."}], "max_tokens": 64, "stream": False}).encode()
req = urllib.request.Request(ROOT + "/chat/completions", body, {"Content-Type": "application/json", "Authorization": "Bearer " + KEY}, method="POST")
try:
    with urllib.request.urlopen(req, timeout=90) as resp:
        result["http_status"] = resp.status
        response = json.load(resp)
except urllib.error.HTTPError as exc:
    result["http_status"] = exc.code
    # Deliberately do not expose the provider response body, which may echo credentials.
    finish("provider_http_error")
except (urllib.error.URLError, ValueError):
    finish("provider_network_or_json_error")
choices = response.get("choices") or []
content = (choices[0].get("message") or {}).get("content") if choices else None
result["has_assistant_text"] = isinstance(content, str) and bool(content.strip())
result["passed"] = result["has_assistant_text"]
finish("assistant_text_received" if result["passed"] else "missing_assistant_text")
