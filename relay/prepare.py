#!/usr/bin/env python3
"""Load one Cherri Cloud request from the fixed, unmerged relay PR and Git blob.

No new branch or commit is created when requests are submitted. The GitHub
Actions job is re-run by job ID after the PR body is updated. All inputs are
untrusted until validated; no source code is printed to logs.
"""
import base64
import hashlib
import json
import os
import pathlib
import re
import sys
import urllib.error
import urllib.parse
import urllib.request

REPO = "davidpovarsky/cherri"
OWNER = "davidpovarsky"
RELAY_BRANCH = "cloud-cherri-relay"
PROTOCOL = "cherri-cloud-relay-v2"
MAX_BYTES = 262144


def api(path):
    url = "https://api.github.com/" + path.lstrip("/")
    req = urllib.request.Request(url, headers={
        "Accept": "application/vnd.github+json",
        "Authorization": "Bearer " + os.environ["GH_TOKEN"],
        "X-GitHub-Api-Version": "2022-11-28",
        "User-Agent": "cherri-cloud-relay"
    })
    with urllib.request.urlopen(req, timeout=30) as res:
        return json.load(res)


def blob_contents(path):
    content = api("repos/" + REPO + "/contents/" + path)
    if content.get("encoding") != "base64":
        raise RuntimeError("unexpected GitHub content encoding")
    return base64.b64decode(content["content"])


def output(**items):
    with open(os.environ["GITHUB_OUTPUT"], "a", encoding="utf-8") as f:
        for key, value in items.items():
            print(key + "=" + str(value), file=f)


def env(**items):
    with open(os.environ["GITHUB_ENV"], "a", encoding="utf-8") as f:
        for key, value in items.items():
            print(key + "=" + str(value), file=f)


def valid_hex(s, size):
    return isinstance(s, str) and bool(re.fullmatch("[0-9a-f]{" + str(size) + "}", s))


def main():
    target_mode = os.environ["CHERRI_TARGET_MODE"]
    if target_mode not in ("compile", "sign", "simulate"):
        raise RuntimeError("unsupported matrix mode")
    prs = api("repos/" + REPO + "/pulls?state=open&head=davidpovarsky%3A" + RELAY_BRANCH + "&per_page=30")
    prs = [p for p in prs if p["head"]["ref"] == RELAY_BRANCH and p["base"]["ref"] == "main"]
    if not prs:
        print("CHERRI_RELAY=READY: fixed PR not created yet")
        output(ready="false")
        return
    if len(prs) != 1:
        raise RuntimeError("expected one and only one relay pull request")
    pr = prs[0]
    if (pr["user"]["login"] != OWNER or pr["head"]["repo"]["full_name"] != REPO
            or pr["base"]["repo"]["full_name"] != REPO or not pr["draft"]):
        raise RuntimeError("relay PR provenance mismatch")
    data = json.loads(pr["body"] or "{}")
    if data.get("protocol") != PROTOCOL:
        raise RuntimeError("relay PR protocol mismatch")
    if data.get("ready") is False:
        print("CHERRI_RELAY=READY: no source submitted")
        output(ready="false")
        return
    if data.get("mode") != target_mode:
        print("CHERRI_RELAY=READY: request targeted another job")
        output(ready="false")
        return
    rid = data.get("request_id")
    oid = data.get("source_blob_sha")
    if not isinstance(rid, str) or not re.fullmatch(r"r[0-9]{8}-[a-z0-9]{8,32}", rid):
        raise RuntimeError("invalid request ID")
    if not valid_hex(oid, 40):
        raise RuntimeError("invalid Git blob SHA")
    blob = api("repos/" + REPO + "/git/blobs/" + oid)
    if blob.get("encoding") != "base64" or blob.get("sha") != oid:
        raise RuntimeError("Git blob encoding/SHA mismatch")
    data_bytes = base64.b64decode(blob["content"])
    if not data_bytes or len(data_bytes) > MAX_BYTES:
        raise RuntimeError("request source empty or over 256 KiB")
    git_oid = hashlib.sha1(b"blob " + str(len(data_bytes)).encode() + b"\x00" + data_bytes).hexdigest()
    if git_oid != oid:
        raise RuntimeError("Git blob content hash mismatch")
    source_sha256 = hashlib.sha256(data_bytes).hexdigest()
    if source_sha256 != data.get("source_sha256"):
        raise RuntimeError("source SHA-256 mismatch")
    source_text = data_bytes.decode("utf-8")
    if "\x00" in source_text:
        raise RuntimeError("source contains a NUL byte")
    source_path = pathlib.Path(os.environ["RUNNER_TEMP"]) / "cherri-cloud-source.cherri"
    source_path.write_bytes(data_bytes)

    cfg = json.loads(blob_contents("cloud-cherri-requests/config.json?ref=cloud-cherri-requests"))
    skill_ref = cfg["skill_ref"]
    if (cfg["repo"] != REPO or not isinstance(skill_ref, str)
            or not re.fullmatch(r"[A-Za-z0-9_./-]{3,150}", skill_ref)
            or ".." in skill_ref or skill_ref.startswith("/")):
        raise RuntimeError("untrusted Skill configuration")
    commit = api("repos/" + REPO + "/commits/" + urllib.parse.quote(skill_ref, safe=""))
    skill_sha = commit["sha"]
    manifest_ref = urllib.parse.quote(skill_sha, safe="")
    manifest = json.loads(blob_contents("skills/cherri-shortcuts/compatibility-manifest.json?ref=" + manifest_ref))
    compiler_sha = manifest.get("compilerCommit")
    docs_sha = manifest.get("docsCommit")
    if manifest.get("languageVersion") != "2.0" or not valid_hex(compiler_sha, 40) or not valid_hex(docs_sha, 40):
        raise RuntimeError("invalid pinned Skill compiler/docs contract")
    if not valid_hex(manifest.get("schemaFingerprint"), 64) or not valid_hex(manifest.get("codecFingerprint"), 64):
        raise RuntimeError("invalid Skill fingerprints")
    output(ready="true", request_id=rid, compiler_sha=compiler_sha, docs_sha=docs_sha)
    env(CHERRI_REQUEST_ID=rid, CHERRI_TARGET_MODE=target_mode,
        CHERRI_SOURCE_PATH=str(source_path), CHERRI_SOURCE_SHA256=source_sha256,
        CHERRI_SOURCE_BLOB_SHA=oid, CHERRI_SKILL_REF=skill_ref,
        CHERRI_SKILL_SHA=skill_sha, CHERRI_COMPILER_SHA=compiler_sha,
        CHERRI_DOCS_SHA=docs_sha)
    print("CHERRI_RELAY=ACCEPTED request=" + rid + " mode=" + target_mode)
    print("LIVE_SKILL_COMMIT=" + skill_sha)
    print("PINNED_COMPILER_COMMIT=" + compiler_sha)


if __name__ == "__main__":
    try:
        main()
    except (KeyError, ValueError, UnicodeError, RuntimeError, urllib.error.HTTPError) as error:
        print("CHERRI_RELAY=REJECTED: " + str(error), file=sys.stderr)
        sys.exit(1)
