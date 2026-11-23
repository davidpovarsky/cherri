#!/usr/bin/env python3
"""PROPOSAL ONLY until reviewed. Read one Windows Process variable at execution."""
import hashlib
import json
import os
from pathlib import Path
import re
import shlex
import subprocess
import sys

HELPER = "/mnt/development/projects/evidence/cherri-p10r15-ai01-compat-20261123/credential-launcher-proposal/oracle-model-identity.py"
HELPER_SHA256 = "267d3a84c61d201e7556cd389f70d08d36c83e8f3eb54cffc51194c1a9bb8fbd"
RECEIPT_PATH = "/mnt/development/projects/evidence/cherri-p10r15-ai01-compat-20261123/model-identity-k9-attempt-01/public-receipt.json"
KEYS = {"status", "httpStatus", "canonicalIdentityVerified", "receiptPath", "receiptSha256", "responseBytes", "responseSha256"}

def check(value):
    if not value:
        raise RuntimeError("METADATA_LAUNCH_REFUSED")

def command():
    bootstrap = "import hashlib,pathlib,sys;p=pathlib.Path(sys.argv[1]);b=p.read_bytes();assert hashlib.sha256(b).hexdigest()==sys.argv[2];sys.argv=[str(p),'--bridge'];exec(compile(b,str(p),'exec'),{'__name__':'__main__','__file__':str(p)})"
    remote = shlex.join(["python3", "-I", "-B", "-c", bootstrap, HELPER, HELPER_SHA256])
    return ["ssh", "-T", "-o", "BatchMode=yes", "-o", "ConnectTimeout=12",
            "-o", "LogLevel=ERROR", "-o", "ForwardAgent=no", "-o", "PermitLocalCommand=no",
            "torah-dev", remote]

def summary(raw, credential):
    check(isinstance(raw, bytes) and len(raw) <= 8192)
    check(credential.encode("ascii") not in raw and json.dumps(credential)[1:-1].encode("ascii") not in raw)
    obj = json.loads(raw)
    check(isinstance(obj, dict) and set(obj) == KEYS)
    check(obj["status"] in ("MODEL_IDENTITY_VERIFIED", "MODEL_IDENTITY_FAILED"))
    check(type(obj["canonicalIdentityVerified"]) is bool)
    check(obj["httpStatus"] is None or type(obj["httpStatus"]) is int and 100 <= obj["httpStatus"] <= 599)
    check(obj["receiptPath"] == RECEIPT_PATH)
    check(type(obj["responseBytes"]) is int and 0 <= obj["responseBytes"] <= 65537)
    check(all(isinstance(obj[k], str) and re.fullmatch(r"[0-9a-f]{64}", obj[k]) for k in ("receiptSha256", "responseSha256")))
    return obj

def launch(credential, process_env):
    check(isinstance(credential, str) and 16 <= len(credential) <= 4096)
    check(all(33 <= ord(c) <= 126 for c in credential))
    # Only ordinary Windows/SSH path discovery variables reach the local SSH child.
    # No provider credential is inherited by SSH; its only provider-key channel is stdin.
    ssh_env = {name: process_env[name] for name in ("SystemRoot", "WINDIR", "SystemDrive", "COMSPEC", "PATHEXT", "USERPROFILE", "HOMEDRIVE", "HOMEPATH", "PATH", "TEMP", "TMP", "APPDATA", "LOCALAPPDATA", "ProgramData", "ALLUSERSPROFILE", "ProgramFiles", "ProgramFiles(x86)") if name in process_env}
    payload = json.dumps({"ANTHROPIC_API_KEY": credential}, separators=(",", ":")).encode("ascii")
    result = subprocess.run(command(), input=payload, stdout=subprocess.PIPE,
                            stderr=subprocess.PIPE, env=ssh_env, timeout=100, check=False)
    value = summary(result.stdout, credential)
    check(credential.encode("ascii") not in result.stderr)
    check(result.returncode in (0, 2))
    print(json.dumps(value, sort_keys=True))
    return result.returncode

def main():
    check(os.name == "nt")
    # Reads only the named Process environment value. No config, vault or user/machine lookup.
    credential = os.environ.get("ANTHROPIC_API_KEY")
    # Normalize observed surrounding whitespace only in memory; never modify the stored variable.
    return launch(credential.strip() if isinstance(credential, str) else credential, os.environ)

if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except Exception:
        print('{"status":"METADATA_LAUNCH_FAILED","detailsWithheld":true}')
        raise SystemExit(3) from None
