#!/usr/bin/env python3
"""Verify the exact public synthetic data bytes used by signed paginate probes."""
import argparse
import hashlib
import json
from pathlib import Path
import re
import subprocess
import urllib.request

ROOT = Path(__file__).resolve().parents[2]
MANIFEST = "tools/shortcuts-simulator-qa/fixtures/p15-paginate-probes/manifest.json"
DATA_DIRECTORY = "docs/apple-runtime/p10r15/paginate-pages"
DATA_FILES = ("one.json", "two.json", "three.json", "empty.json", "cycle-one.json", "cycle-two.json")
SCENARIOS = ("ordered", "cycle", "page-cap", "item-cap", "empty")
RAW_PREFIX = "https://raw.githubusercontent.com/davidpovarsky/cherri/"


def sha(data):
    return hashlib.sha256(data).hexdigest()


def bound_manifest(root=ROOT):
    root = Path(root)
    manifest_path = root / MANIFEST
    manifest = json.loads(manifest_path.read_bytes())
    if manifest.get("syntheticOnly") is not True or manifest.get("schemaVersion") != 1:
        raise ValueError("PAGINATE_DATA_MANIFEST_INVALID")
    commit = manifest.get("dataCommit", "")
    if not isinstance(commit, str) or re.fullmatch(r"[0-9a-f]{40}", commit) is None:
        raise ValueError("PAGINATE_DATA_COMMIT_NOT_IMMUTABLE")
    base = RAW_PREFIX + commit + "/" + DATA_DIRECTORY + "/"
    pages = manifest.get("pages")
    if not isinstance(pages, list) or len(pages) != len(DATA_FILES):
        raise ValueError("PAGINATE_DATA_PAGE_SET_INVALID")
    if {p.get("fileName") for p in pages if isinstance(p, dict)} != set(DATA_FILES):
        raise ValueError("PAGINATE_DATA_PAGE_SET_INVALID")
    for page in pages:
        content = (root / DATA_DIRECTORY / page["fileName"]).read_bytes()
        if page.get("sha256") != sha(content):
            raise ValueError("PAGINATE_DATA_CHECKOUT_HASH_MISMATCH")
        if page.get("url") != base + page["fileName"]:
            raise ValueError("PAGINATE_DATA_URL_SCOPE_MISMATCH")
        json.loads(content)
    cases = manifest.get("cases")
    if not isinstance(cases, list) or len(cases) != len(SCENARIOS):
        raise ValueError("PAGINATE_DATA_CASE_SET_INVALID")
    expected = {"qa-paginate-" + scenario + "-v2" for scenario in SCENARIOS}
    if {c.get("id") for c in cases if isinstance(c, dict)} != expected:
        raise ValueError("PAGINATE_DATA_CASE_SET_INVALID")
    for case in cases:
        path = "tools/shortcuts-simulator-qa/fixtures/p15-paginate-probes/" + case["id"] + ".cherri"
        if case.get("path") != path:
            raise ValueError("PAGINATE_DATA_FIXTURE_PATH_MISMATCH")
        content = (root / path).read_bytes()
        if case.get("sourceSha256") != sha(content):
            raise ValueError("PAGINATE_DATA_FIXTURE_HASH_MISMATCH")
        calls = re.findall(r'paginate\("([^"]+)"', content.decode("utf-8"))
        if not calls or any(url not in {base + name for name in DATA_FILES} for url in calls):
            raise ValueError("PAGINATE_DATA_FIXTURE_URL_MISMATCH")
    return manifest, sha(manifest_path.read_bytes())


def fetch(url):
    request = urllib.request.Request(url, headers={"User-Agent": "Cherri-Synthetic-Paginate-QA/1"})
    with urllib.request.urlopen(request, timeout=20) as response:
        return response.status, response.geturl(), response.read(65537)


def verify(root=ROOT, fetcher=fetch):
    manifest, manifest_sha = bound_manifest(root)
    pages = []
    for page in manifest["pages"]:
        status, final_url, content = fetcher(page["url"])
        if status != 200:
            raise ValueError("PAGINATE_DATA_HTTP_STATUS")
        if final_url != page["url"]:
            raise ValueError("PAGINATE_DATA_REDIRECT_UNVERIFIED")
        if not isinstance(content, bytes) or len(content) > 65536:
            raise ValueError("PAGINATE_DATA_SIZE_INVALID")
        if sha(content) != page["sha256"]:
            raise ValueError("PAGINATE_DATA_REMOTE_HASH_MISMATCH")
        json.loads(content)
        pages.append({"fileName": page["fileName"], "sha256": sha(content), "bytes": len(content), "httpStatus": status})
    return {
        "schemaVersion": 1, "status": "PASS_PUBLIC_SYNTHETIC_DATA_BYTES_ONLY",
        "dataCommit": manifest["dataCommit"], "manifestSha256": manifest_sha,
        "pages": pages, "fixtureSourceHashes": {c["id"]: c["sourceSha256"] for c in manifest["cases"]},
        "nativeExecuted": 0,
    }


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--root", type=Path, default=ROOT)
    parser.add_argument("--report", type=Path, required=True)
    args = parser.parse_args()
    args.report.parent.mkdir(parents=True, exist_ok=True)
    args.report.write_text('{"status":"RUNNING_PUBLIC_DATA_VERIFICATION","nativeExecuted":0}\n', encoding="utf-8")
    try:
        report = verify(args.root)
        report["compilerSourceCommit"] = subprocess.check_output(
            ["git", "-C", str(args.root), "rev-parse", "HEAD"], text=True).strip()
    except Exception as error:
        # Deliberately retain only the exception category. URLs and response
        # bodies must not be included in an error receipt or callback.
        args.report.write_text(json.dumps({"status": "FAIL_PUBLIC_DATA_VERIFICATION",
            "exceptionType": type(error).__name__, "nativeExecuted": 0}) + "\n", encoding="utf-8")
        raise SystemExit("PAGINATE_PUBLIC_DATA_VERIFICATION_FAILED") from None
    args.report.write_text(json.dumps(report, indent=2) + "\n", encoding="utf-8")
    print(json.dumps({"status": report["status"], "dataCommit": report["dataCommit"], "verifiedPages": len(report["pages"])}))


if __name__ == "__main__":
    main()
