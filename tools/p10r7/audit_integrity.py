#!/usr/bin/env python3
"""
tools/p10r7/audit_integrity.py
Harness-integrity primitives for the 38-specimen audit (P10R8).

These are small, independently testable functions used by run_38_audit.py. They exist so the
anti-false-positive claims (stale output, missing specimens, forged runtime PASS) are
exercised by real code, not by hand-written truth tables.
"""

import os
import pathlib
import subprocess
import time
from typing import Any, Dict, List, Optional

EXPECTED_SPECIMEN_IDS = [f"{i:03d}" for i in range(1, 39)]
RUNTIME_EVIDENCE_REQUIRED_FIELDS = ("deviceModel", "osVersion", "signedInstallRecorded", "runLogSha256", "executedAt")


def check_corpus_inventory(input_dir: pathlib.Path, expected_ids: Optional[List[str]] = None) -> List[str]:
    """Fail-closed inventory check: every expected id needs a non-empty .plist and .json."""
    errors: List[str] = []
    for sid in expected_ids or EXPECTED_SPECIMEN_IDS:
        for ext in ("plist", "json"):
            p = input_dir / f"{sid}.{ext}"
            if not p.is_file():
                errors.append(f"MISSING_SPECIMEN:{sid}.{ext}")
            elif p.stat().st_size == 0:
                errors.append(f"EMPTY_SPECIMEN:{sid}.{ext}")
    return errors


def run_compile_step(cmd: List[str], out_path: pathlib.Path, timeout: int = 60) -> Dict[str, Any]:
    """Run one CLI step against a FRESH output path.

    Any pre-existing file at out_path is removed first, so a stale artifact from an earlier
    run can never be mistaken for the result of this step. The step only counts as passing if
    the process exits 0 AND this invocation produced a non-empty output."""
    try:
        if out_path.exists():
            out_path.unlink()
    except OSError as e:
        return {"gate": "FAIL", "rc": -3, "outputPresent": False, "error": f"CANNOT_CLEAR_STALE_OUTPUT:{type(e).__name__}"}
    t0 = time.time()
    try:
        res = subprocess.run(cmd, stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=timeout, text=True,
                             encoding="utf-8", errors="replace")
        rc, out, err = res.returncode, res.stdout, res.stderr
    except subprocess.TimeoutExpired:
        rc, out, err = -1, "", "TIMEOUT"
    except Exception as e:  # oracle/harness exception is an ERROR, never a pass
        rc, out, err = -2, "", f"EXCEPTION:{type(e).__name__}"
    present = out_path.is_file() and out_path.stat().st_size > 0
    fresh = present and out_path.stat().st_mtime >= t0 - 1.0
    return {
        "gate": "PASS" if (rc == 0 and fresh) else "FAIL",
        "rc": rc,
        "outputPresent": present,
        "outputFresh": fresh,
        "seconds": round(time.time() - t0, 3),
        "stdoutTail": out[-300:] if rc != 0 else "",
        "stderrTail": err[-300:] if rc != 0 else "",
    }


def check_runtime_gate(gates: Dict[str, str], device_evidence: Optional[Dict[str, Any]] = None) -> List[str]:
    """APPLE_RUNTIME_EXECUTED may only be PASS with complete physical-device evidence."""
    errors: List[str] = []
    state = gates.get("APPLE_RUNTIME_EXECUTED")
    if state == "PASS":
        if not isinstance(device_evidence, dict):
            errors.append("RUNTIME_PASS_WITHOUT_DEVICE_EVIDENCE")
        else:
            for f in RUNTIME_EVIDENCE_REQUIRED_FIELDS:
                if not device_evidence.get(f):
                    errors.append(f"RUNTIME_EVIDENCE_FIELD_MISSING:{f}")
    elif state != "BLOCKED_DEVICE":
        errors.append(f"RUNTIME_STATE_UNEXPECTED:{state}")
    return errors
