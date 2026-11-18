#!/usr/bin/env python3
"""Stage a synthetic-only, text-only publication set without touching originals.

Raster media, xcresult, archives and raw compiler inputs are quarantined by
policy regardless of OCR or byte-scan results. This is not pixel redaction.
"""

import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import tempfile


ALLOWED = {
    "summary.json", "environment.json", "compiler-manifest.json", "import-attempts.json",
    "runtime/instrumentation-vs-original.json", "runtime/output-assertions.json",
    "runtime/signing-provenance.json", "runtime/callback-events.json", "runtime/runtime-trace.jsonl",
    "tests/xcresult-summary.json", "editor/editor-action-coverage.json",
    "editor/screenshots-manifest.json", "editor/editor-actions.json",
    "editor/editor-comparison.json", "editor/editor-tree.json", "editor/editor-report.md",
    "editor/plist-vs-editor-comparison.json", "editor/handshake-status.json", "editor/ax-persistence-diagnostics.json", "docs/report.md",
}
RAW_SUFFIXES = {".png", ".jpg", ".jpeg", ".gif", ".heic", ".webp", ".pdf", ".zip", ".tar", ".gz",
                ".shortcut", ".plist", ".cherri", ".a", ".dylib", ".app", ".xcresult"}
CANARY = re.compile(r"(?i)(cherri-shortcut-audit-20261110|\b[0-3][0-9]\.plist\b|BEGIN [A-Z ]*PRIVATE KEY|"
                    r"ghp_[A-Za-z0-9_]+|github_pat_[A-Za-z0-9_]+|sk-[A-Za-z0-9]{16,}|"
                    r"Bearer\s+[A-Za-z0-9._\-]+|AppleID|iCloudToken|CHERRI_PRIVATE_CANARY|"
                    r"[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}|"
                    r"(?:access_token|api_key|password|secret|authorization)\s*[=:]\s*[^\s&\"<]+|"
                    r"(?:access_token|api_key|password|secret|authorization)[\"']\s*:\s*[\"'][^\"']+|"
                    r"[?&](?:token|key|auth|password|secret)=[^&\s]+)")
MAGIC = (b"\x89PNG\r\n\x1a\n", b"\xff\xd8\xff", b"GIF8", b"%PDF", b"PK\x03\x04", b"AEA1", b"bplist")


class PublicationBlocked(ValueError):
    pass


def sha(data):
    return hashlib.sha256(data).hexdigest()


def safe_path(path):
    value = path.as_posix()
    return value if re.fullmatch(r"[A-Za-z0-9_./-]{1,180}", value) and not CANARY.search(value) else "path-sha256:" + sha(value.encode())


def verify_synthetic_manifest(root, trusted_sources):
    manifest_path = root / "compiler-manifest.json"
    try:
        manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
    except (OSError, ValueError) as exc:
        raise PublicationBlocked("SYNTHETIC_PROVENANCE_MISSING") from exc
    if manifest.get("syntheticOnly") is not True:
        raise PublicationBlocked("SYNTHETIC_PROVENANCE_REQUIRED")
    fixtures = manifest.get("fixtures", [])
    if not fixtures or len({f.get("fixtureId") for f in fixtures}) != len(fixtures):
        raise PublicationBlocked("FIXTURE_MANIFEST_INVALID")
    for fixture in fixtures:
        file_name = fixture.get("fileName")
        if file_name not in trusted_sources or fixture.get("sourceSha256") != trusted_sources[file_name]:
            raise PublicationBlocked("FIXTURE_SOURCE_HASH_MISMATCH")
    summary_path = root / "summary.json"
    try:
        summary = json.loads(summary_path.read_text(encoding="utf-8"))
    except (OSError, ValueError) as exc:
        raise PublicationBlocked("SUMMARY_MISSING") from exc
    if summary.get("privacyGatePassed") is not True:
        raise PublicationBlocked("EXISTING_PRIVACY_GATE_FAILED")


def stage(root, output, trusted_sources):
    root, output = Path(root).resolve(), Path(output).absolute()
    if not root.is_dir() or output.exists() or root == output or root in output.parents:
        raise PublicationBlocked("OUTPUT_MUST_BE_NEW_AND_OUTSIDE_RAW_ROOT")
    verify_synthetic_manifest(root, trusted_sources)
    approved, quarantined, failures = [], [], []
    for path in sorted(root.rglob("*")):
        rel = path.relative_to(root)
        name = rel.as_posix()
        if path.is_symlink():
            failures.append({"path": safe_path(rel), "reason": "SYMLINK_NOT_ALLOWED"})
            continue
        if not path.is_file():
            continue
        data = path.read_bytes()
        entry = {"path": safe_path(rel), "sha256": sha(data), "bytes": len(data)}
        raw = any(p.suffix.lower() in RAW_SUFFIXES for p in [path] + list(path.relative_to(root).parents))
        if raw or data.startswith(MAGIC):
            quarantined.append({**entry, "reason": "RASTER_ARCHIVE_OR_RAW_INPUT_NOT_PUBLIC"})
            continue
        if name not in ALLOWED:
            quarantined.append({**entry, "reason": "PATH_NOT_EXPLICITLY_ALLOWLISTED"})
            continue
        if len(data) > 10 * 1024 * 1024:
            failures.append({**entry, "reason": "TEXT_SIZE_LIMIT"})
            continue
        try:
            text = data.decode("utf-8")
            if "\x00" in text:
                raise ValueError("binary")
            if name.endswith(".json"):
                json.loads(text)
            elif name.endswith(".jsonl"):
                for line in text.splitlines():
                    if line.strip():
                        json.loads(line)
        except (UnicodeError, ValueError):
            failures.append({**entry, "reason": "INVALID_TEXT_OR_STRUCTURED_DATA"})
            continue
        if CANARY.search(text):
            # Never echo the matching secret, its surrounding content or an
            # unsafe filename into stdout or the published manifest.
            failures.append({**entry, "reason": "SECRET_OR_PRIVATE_CANARY"})
            continue
        approved.append((data, rel, entry))
    if failures:
        raise PublicationBlocked("PUBLICATION_BLOCKED:" + json.dumps(failures, separators=(",", ":")))
    output.parent.mkdir(parents=True, exist_ok=True)
    temporary = Path(tempfile.mkdtemp(prefix=".cherri-public-", dir=output.parent))
    try:
        for data, rel, _ in approved:
            destination = temporary / rel
            destination.parent.mkdir(parents=True, exist_ok=True)
            destination.write_bytes(data)
        manifest = {"schemaVersion": "1.5", "syntheticOnly": True, "publicationGatePassed": True,
                    "pixelRedactionClaimed": False, "rasterPublicationAllowed": False,
                    "policy": "EXPLICIT_SYNTHETIC_TEXT_PATHS_ONLY", "published": [e for _, _, e in approved],
                    "quarantined": quarantined,
                    "limitations": "No arbitrary private-input or pixel sanitization guarantee; all raster/archives/raw files stay outside publication."}
        (temporary / "publication-manifest.json").write_text(json.dumps(manifest, indent=2) + "\n", encoding="utf-8")
        temporary.rename(output)
    finally:
        if temporary.exists():
            shutil.rmtree(temporary)
    return manifest


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--root", required=True)
    parser.add_argument("--output", required=True)
    parser.add_argument("--fixtures-dir", required=True)
    args = parser.parse_args()
    fixtures = Path(args.fixtures_dir)
    trusted = {p.name: sha(p.read_bytes()) for p in fixtures.glob("qa-*.cherri") if p.is_file() and not p.is_symlink()}
    try:
        manifest = stage(args.root, args.output, trusted)
    except (OSError, PublicationBlocked) as exc:
        print(str(exc))
        return 1
    if os.environ.get("GITHUB_OUTPUT"):
        with open(os.environ["GITHUB_OUTPUT"], "a", encoding="utf-8") as handle:
            handle.write("passed=true\n")
    print(json.dumps({"publicationGatePassed": True, "publishedFiles": len(manifest["published"]),
                      "quarantinedFiles": len(manifest["quarantined"]), "rasterPublicationAllowed": False}))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
