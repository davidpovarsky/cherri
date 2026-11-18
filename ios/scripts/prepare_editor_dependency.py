#!/usr/bin/env python3
"""Apply Cherri's bounded patch to the existing, immutable CodeEditorView pin."""
from __future__ import annotations

import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess


class PatchError(ValueError):
    pass


def sha256(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()


def git_value(checkout: Path, *args: str) -> str:
    result = subprocess.run(["git", "-C", str(checkout), *args], check=True,
                            text=True, capture_output=True)
    return result.stdout.strip()


def apply_patch(checkout: Path, manifest: dict, *, check_only: bool = False) -> dict:
    checkout = checkout.resolve()
    revision = manifest.get("revision", "")
    if manifest.get("schemaVersion") != 1 or not re.fullmatch(r"[0-9a-f]{40}", revision):
        raise PatchError("Unsupported patch manifest or revision")
    actual_revision = git_value(checkout, "rev-parse", "HEAD")
    if actual_revision != revision:
        raise PatchError(f"Editor revision mismatch: expected {revision}, got {actual_revision}")

    prepared = []
    seen = set()
    for entry in manifest.get("files", []):
        relative = entry.get("path", "")
        path = (checkout / relative).resolve()
        if (not relative.startswith("Sources/CodeEditorView/") or ".." in Path(relative).parts
                or path in seen or not path.is_relative_to(checkout)):
            raise PatchError("Unsafe or repeated patch path")
        seen.add(path)
        if git_value(checkout, "rev-parse", f"{revision}:{relative}") != entry["originalGitBlob"]:
            raise PatchError(f"Pinned Git blob mismatch: {relative}")
        before = path.read_bytes()
        actual_hash = sha256(before)
        if actual_hash == entry["patchedSha256"]:
            after = before
            status = "ALREADY_APPLIED"
        elif actual_hash == entry["originalSha256"]:
            source = before.decode("utf-8")
            for replacement in entry["replacements"]:
                old, new = replacement["old"], replacement["new"]
                if not old or source.count(old) != 1:
                    raise PatchError(f"Patch anchor is not unique: {relative}")
                source = source.replace(old, new, 1)
            after = source.encode("utf-8")
            if sha256(after) != entry["patchedSha256"]:
                raise PatchError(f"Patched source hash mismatch: {relative}")
            status = "APPLIED"
        else:
            raise PatchError(f"Unrecognized editor source hash: {relative}")
        prepared.append((path, before, after, status, entry))

    if not prepared:
        raise PatchError("Empty patch manifest")
    if check_only and any(before != after for _, before, after, _, _ in prepared):
        raise PatchError("Editor patch has not been applied")

    # Validate every file before changing any file. Each replacement is atomic;
    # rerunning after interruption accepts only original or exact patched bytes.
    for path, before, after, _, _ in prepared:
        if before == after:
            continue
        temporary = path.with_name(path.name + ".cherri-patch-tmp")
        try:
            with temporary.open("xb") as stream:
                stream.write(after)
                stream.flush()
                os.fsync(stream.fileno())
            os.replace(temporary, path)
        finally:
            temporary.unlink(missing_ok=True)
    return {
        "schemaVersion": 1, "patchId": manifest["patchId"], "revision": actual_revision,
        "status": "PASS", "checkOnly": check_only,
        "files": [{"path": entry["path"], "status": status,
                   "originalSha256": entry["originalSha256"], "patchedSha256": sha256(after)}
                  for _, _, after, status, entry in prepared],
    }


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--checkouts", type=Path)
    parser.add_argument("--checkout", type=Path)
    parser.add_argument("--check-only", action="store_true")
    parser.add_argument("--receipt", type=Path)
    args = parser.parse_args()
    ios = Path(__file__).resolve().parents[1]
    manifest = json.loads((ios / "DependencyPatches/CodeEditorView/generated-closers.json").read_text())
    project = (ios / "project.yml").read_text()
    if not re.search(r"^\s+revision:\s*" + re.escape(manifest["revision"]) + r"\s*$", project, re.M):
        raise PatchError("Project no longer pins the audited editor revision")
    checkout = args.checkout
    if checkout is None:
        checkouts = args.checkouts or ios / ".build/SourcePackages/checkouts"
        matches = [path for path in checkouts.iterdir()
                   if path.is_dir() and path.name.lower() == "codeeditorview"]
        if len(matches) != 1:
            raise PatchError("Resolve Swift packages first; exactly one pinned CodeEditorView checkout is required")
        checkout = matches[0]
    receipt = apply_patch(checkout, manifest, check_only=args.check_only)
    output = json.dumps(receipt, indent=2) + "\n"
    if args.receipt:
        args.receipt.parent.mkdir(parents=True, exist_ok=True)
        args.receipt.write_text(output, encoding="utf-8", newline="\n")
    print(output, end="")
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except (PatchError, OSError, subprocess.CalledProcessError) as error:
        raise SystemExit(f"Editor dependency verification failed: {error}")
