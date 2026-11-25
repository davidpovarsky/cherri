"""Verify reviewed iOS evolution without weakening the original R23 freeze.

The historical baseline remains immutable. A review pins an exact descendant
commit and both protected blobs, with the baseline test inventory preserved.
Subsequent committed, staged, unstaged or untracked changes still fail closed.
This checks source preservation, not the execution or success of iOS tests.
"""

import hashlib
import json
import pathlib
import re
import subprocess
import time


def test_methods(raw):
    return sorted(set(re.findall(
        rb"(?m)^\s*(?:(?:public|internal|private|fileprivate|override|final|static|class)\s+)*"
        rb"func\s+(test[A-Za-z0-9_]*)\s*\(", raw)))


def execute_reviewed_gate(root, gate_id, definition):
    started = time.monotonic()
    observations = []
    result = {
        "id": gate_id, "name": definition["name"], "tier": definition["tier"],
        "sourceFile": definition["sourceFile"], "status": "FAIL", "exitCode": 1,
    }

    def git(*args):
        command = ["git", "--no-replace-objects", *args]
        process = subprocess.run(command, cwd=root, capture_output=True, timeout=30)
        observations.append({
            "command": command, "exitCode": process.returncode,
            "stdoutSha256": hashlib.sha256(process.stdout).hexdigest(),
            "stderrSha256": hashlib.sha256(process.stderr).hexdigest(),
        })
        if process.returncode:
            raise ValueError("Git verification failed: " + " ".join(command))
        return process.stdout

    def snapshot(revision, path):
        entry = git("ls-tree", "-z", revision, "--", path)
        if not entry:
            return {"blob": None, "sha256": None, "tests": []}
        fields, actual_path = entry.rstrip(b"\0").split(b"\t", 1)
        mode, kind, blob = fields.decode().split()
        if actual_path.decode() != path or mode not in ("100644", "100755") or kind != "blob":
            raise ValueError("Protected file is not a regular Git blob: " + path)
        raw = git("cat-file", "blob", blob)
        return {
            "blob": blob, "sha256": hashlib.sha256(raw).hexdigest(),
            "tests": [name.decode() for name in test_methods(raw)],
        }

    try:
        protected = definition["checkProtectedIOSFiles"]
        base = definition["upstreamBase"]
        review_path = definition["protectedIOSReview"]
        relative = pathlib.PurePosixPath(review_path)
        if relative.is_absolute() or ".." in relative.parts:
            raise ValueError("Review must be a repository-relative file")
        review_raw = (root / review_path).read_bytes()
        review = json.loads(review_raw)
        if review.get("schemaVersion") != 1 or review.get("baseline") != base:
            raise ValueError("Review does not match the immutable R23 baseline")
        reviewed = review.get("reviewedCommit", "")
        if not re.fullmatch(r"[0-9a-f]{40}", reviewed):
            raise ValueError("Review must pin a full commit")
        rows = review.get("files", [])
        if (not isinstance(rows, list) or len(rows) != len(protected)
                or {row.get("path") for row in rows} != set(protected)):
            raise ValueError("Review must cover exactly both protected paths")
        if not review.get("reason") or not review.get("reviewEvidence"):
            raise ValueError("Review reason and evidence are required")
        git("merge-base", "--is-ancestor", base, reviewed)
        git("merge-base", "--is-ancestor", reviewed, "HEAD")
        historical = git("diff", base + "..HEAD", "--", *protected)
        result["historicalBaselineResult"] = "FAIL" if historical.strip() else "PASS"
        result["historicalDiffSha256"] = hashlib.sha256(historical).hexdigest()
        changed = git("status", "--porcelain", "--", *protected, review_path)
        if changed.strip():
            raise ValueError("Protected files or review contain uncommitted changes")
        # Require the on-disk review to be precisely the committed object.
        if git("show", "HEAD:" + review_path) != review_raw:
            raise ValueError("Review differs from its committed Git object")
        file_results = []
        for row in rows:
            path = row["path"]
            baseline = snapshot(base, path)
            approved = snapshot(reviewed, path)
            current = snapshot("HEAD", path)
            if row.get("baseline") != baseline or row.get("reviewed") != approved:
                raise ValueError("Review blob, digest or test inventory mismatch: " + path)
            missing = sorted(set(baseline["tests"]) - set(approved["tests"]))
            if missing:
                raise ValueError("Review removes original XCTest methods: " + ", ".join(missing))
            if baseline["blob"] is not None and approved["blob"] is None:
                raise ValueError("Review removes a protected file: " + path)
            if current != approved:
                raise ValueError("Unreviewed committed protected-file change: " + path)
            file_results.append({
                "path": path, "baselineBlob": baseline["blob"],
                "reviewedBlob": approved["blob"],
                "baselineTestsRetained": len(baseline["tests"]),
                "reviewedTests": len(approved["tests"]),
            })
        result.update({
            "status": "PASS", "exitCode": 0, "reviewedCommit": reviewed,
            "reviewSha256": hashlib.sha256(review_raw).hexdigest(),
            "protectedFiles": file_results,
            "scope": "Reviewed source preservation; iOS execution remains an independent gate",
        })
    except (ValueError, OSError, KeyError, TypeError, subprocess.TimeoutExpired) as exc:
        result["error"] = str(exc)
    result["durationSeconds"] = round(time.monotonic() - started, 3)
    result["command"] = "reviewed R23 baseline..HEAD, pinned blobs and working tree"
    result["observations"] = observations
    result["outputSha256"] = hashlib.sha256(
        json.dumps(result, sort_keys=True).encode()
    ).hexdigest()
    return result
