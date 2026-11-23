#!/usr/bin/env python3
"""Bind an original public Pages receipt ZIP to the exact pinned deployment.

Only public metadata is fetched. The caller transfers the original authorized
artifact bytes; this does not need a cross-repository download credential.
No site is built/deployed, no pin is changed, and no D03 pass is emitted here.
"""
import argparse
import base64
from datetime import datetime, timezone
import io
import json
import os
from pathlib import Path
import re
import stat
import subprocess
import zipfile
from urllib.error import HTTPError
from urllib.request import HTTPRedirectHandler, Request, build_opener

from verify_documentation_consumers import (
    COMPILER_ORIGIN, digest, load_json, regular_bytes, require,
    validate_pins, verify_checkout,
)

DOCS_REPO = "davidpovarsky/cherrilang.org"
API = "https://api.github.com/repos/" + DOCS_REPO + "/"
ARTIFACT = "published-docs-build-evidence"
MEMBERS = ("cherri-docs-site-evidence.json", "cherri-docs-site-controls.json")
MAX_ZIP_BYTES = 32768
MAX_MEMBER_BYTES = 262144
MAX_METADATA_BYTES = 1048576


class MetadataHTTPError(ValueError):
    """Expose only the fixed public endpoint, status and numeric retry metadata."""

    def __init__(self, relative, status, headers, authenticated):
        super().__init__("D03_METADATA_HTTP_" + str(status))
        self.public_details = {"endpoint": relative, "status": status,
                               "authenticated": authenticated}
        for header, field in (("X-RateLimit-Remaining", "rateLimitRemaining"),
                              ("X-RateLimit-Reset", "rateLimitReset"),
                              ("Retry-After", "retryAfterSeconds")):
            value = headers.get(header, "")
            if re.fullmatch(r"[0-9]{1,12}", value):
                self.public_details[field] = int(value)


class NoMetadataRedirect(HTTPRedirectHandler):
    def redirect_request(self, request, response, code, message, headers, new_url):
        # Exact URL identity is required, so a redirect has no valid use here.
        # Reject it before urllib can forward an Authorization header.
        return None


def public_metadata(relative):
    require(isinstance(relative, str) and re.fullmatch(
        r"actions/runs/[1-9][0-9]{0,19}"
        r"(?:/attempts/[1-9][0-9]{0,19}/jobs\?per_page=100|/artifacts\?per_page=100)?",
        relative), "D03_METADATA_ENDPOINT")
    url = API + relative
    headers = {"Accept": "application/vnd.github+json",
               "User-Agent": "Cherri-D03-public-receipt-verifier/1",
               "Cache-Control": "no-cache"}
    token = os.environ.get("CHERRI_D03_GITHUB_TOKEN", "")
    require(not token or not re.search(r"\s", token), "D03_METADATA_TOKEN_FORMAT")
    if token:
        headers["Authorization"] = "Bearer " + token
    request = Request(url, headers=headers)
    try:
        with build_opener(NoMetadataRedirect()).open(request, timeout=30) as response:
            require(response.status == 200 and response.url == url, "D03_METADATA_HTTP_IDENTITY")
            data = response.read(MAX_METADATA_BYTES + 1)
            require(len(data) <= MAX_METADATA_BYTES, "D03_METADATA_SIZE_LIMIT")
            return data
    except HTTPError as error:
        try:
            require(error.url == url, "D03_METADATA_HTTP_IDENTITY")
            failure = MetadataHTTPError(relative, error.code, error.headers, bool(token))
        finally:
            error.close()
        raise failure from None


def timestamp(value):
    require(isinstance(value, str) and value.endswith("Z"), "D03_METADATA_TIMESTAMP")
    return datetime.fromisoformat(value[:-1] + "+00:00")


def decode_archive(encoded):
    require(isinstance(encoded, str) and 0 < len(encoded) <= 4 * ((MAX_ZIP_BYTES + 2) // 3),
            "D03_ARCHIVE_INPUT_BOUND")
    try:
        raw = base64.b64decode(encoded, validate=True)
    except (ValueError, TypeError) as error:
        raise ValueError("D03_ARCHIVE_BASE64_INVALID") from error
    require(0 < len(raw) <= MAX_ZIP_BYTES and base64.b64encode(raw).decode("ascii") == encoded,
            "D03_ARCHIVE_ENCODING_OR_SIZE")
    return raw


def exact_members(raw):
    try:
        with zipfile.ZipFile(io.BytesIO(raw)) as archive:
            rows = archive.infolist()
            require(len(rows) == 2 and {row.filename for row in rows} == set(MEMBERS),
                    "D03_ARCHIVE_MEMBER_SET")
            result = {}
            for row in rows:
                require(not row.is_dir() and stat.S_IFMT(row.external_attr >> 16) in (0, stat.S_IFREG)
                        and row.compress_type in (zipfile.ZIP_STORED, zipfile.ZIP_DEFLATED)
                        and not row.flag_bits & 1 and 0 < row.file_size <= MAX_MEMBER_BYTES,
                        "D03_ARCHIVE_UNSAFE_MEMBER")
                data = archive.read(row)
                require(len(data) == row.file_size, "D03_ARCHIVE_MEMBER_SIZE")
                load_json(data)
                result[row.filename] = data
            return result
    except (zipfile.BadZipFile, RuntimeError, NotImplementedError) as error:
        raise ValueError("D03_ARCHIVE_INVALID") from error


def write_json(path, value):
    path.write_bytes((json.dumps(value, indent=2) + "\n").encode("utf-8"))


def prepare(source, implementation, deployment_run, encoded, output, *, fetch=public_metadata):
    require(re.fullmatch(r"[0-9a-f]{40}", implementation or ""), "D03_IMPLEMENTATION_SHA")
    require(re.fullmatch(r"[1-9][0-9]{0,19}", deployment_run or ""), "D03_DEPLOYMENT_RUN_ID")
    source, output = Path(source).resolve(), Path(output).absolute()
    require(not output.exists() and not output.is_symlink() and output == output.resolve()
            and source not in output.resolve().parents,
            "D03_NEW_OUTPUT_OUTSIDE_SOURCE_REQUIRED")
    verify_checkout(source, implementation, COMPILER_ORIGIN)
    pins = load_json(regular_bytes(source / "skills/cherri-shortcuts/compatibility-manifest.json"))
    validate_pins(pins)
    raw = decode_archive(encoded)
    output.mkdir(parents=True)
    quarantine = output / "quarantine"
    quarantine.mkdir()
    # Unverified input is retained locally and is never in the upload path.
    (quarantine / "original-publication-evidence.zip").write_bytes(raw)
    saved = {}

    def observe(relative, filename):
        data = fetch(relative)
        require(isinstance(data, bytes) and 0 < len(data) <= MAX_METADATA_BYTES,
                "D03_METADATA_SIZE_LIMIT")
        value = load_json(data)
        saved[filename] = data
        return value

    run_id = int(deployment_run)
    run_path = "actions/runs/" + deployment_run
    run = observe(run_path, "github-deployment-run.json")
    require(run.get("id") == run_id and run.get("head_sha") == pins["docsCommit"]
            and run.get("status") == "completed" and run.get("conclusion") == "success"
            and run.get("path") == ".github/workflows/pages.yml"
            and run.get("event") in ("push", "workflow_dispatch")
            and run.get("repository", {}).get("full_name") == DOCS_REPO
            and run.get("head_repository", {}).get("full_name") == DOCS_REPO
            and type(run["repository"].get("id")) is int and run["repository"]["id"] > 0
            and type(run["head_repository"].get("id")) is int and run["head_repository"]["id"] > 0
            and run.get("html_url") == "https://github.com/" + DOCS_REPO + "/actions/runs/" + deployment_run
            and type(run.get("run_attempt")) is int and run["run_attempt"] > 0,
            "D03_DEPLOYMENT_RUN_IDENTITY")
    attempt = run["run_attempt"]
    jobs_data = observe(run_path + "/attempts/" + str(attempt) + "/jobs?per_page=100",
                        "github-deployment-jobs.json")
    jobs = jobs_data.get("jobs", [])
    require(isinstance(jobs, list) and type(jobs_data.get("total_count")) is int
            and jobs_data["total_count"] == len(jobs) and 0 < len(jobs) <= 100
            and all(isinstance(job, dict) and job.get("run_id") == run_id
                    and job.get("head_sha") == pins["docsCommit"] for job in jobs),
            "D03_DEPLOYMENT_JOBS_INCOMPLETE")
    for name in ("build", "deploy"):
        matches = [job for job in jobs if isinstance(job, dict) and job.get("name") == name]
        require(len(matches) == 1 and matches[0].get("run_id") == run_id
                and matches[0].get("head_sha") == pins["docsCommit"]
                and matches[0].get("status") == "completed" and matches[0].get("conclusion") == "success",
                "D03_DEPLOYMENT_JOB_NOT_PASSED")
    artifact_data = observe(run_path + "/artifacts?per_page=100", "github-deployment-artifacts.json")
    artifacts = artifact_data.get("artifacts", [])
    require(isinstance(artifacts, list) and type(artifact_data.get("total_count")) is int
            and artifact_data["total_count"] == len(artifacts) and len(artifacts) <= 100,
            "D03_DEPLOYMENT_ARTIFACTS_INCOMPLETE")
    matching = [row for row in artifacts if isinstance(row, dict) and row.get("name") == ARTIFACT]
    require(len(matching) == 1, "D03_DEPLOYMENT_ARTIFACT_NOT_UNIQUE")
    artifact = matching[0]
    binding = artifact.get("workflow_run", {})
    require(type(artifact.get("id")) is int and artifact["id"] > 0
            and artifact.get("expired") is False and type(artifact.get("size_in_bytes")) is int
            and artifact["size_in_bytes"] == len(raw) and artifact.get("digest") == "sha256:" + digest(raw)
            and binding.get("id") == run_id and binding.get("head_sha") == pins["docsCommit"]
            and binding.get("repository_id") == run["repository"].get("id")
            and binding.get("head_repository_id") == run["head_repository"].get("id"),
            "D03_ARTIFACT_DIGEST_OR_SOURCE_BINDING")
    require(timestamp(run["run_started_at"]) <= timestamp(artifact["created_at"]) <= timestamp(run["updated_at"]),
            "D03_ARTIFACT_FROM_DIFFERENT_ATTEMPT")
    require(timestamp(artifact["expires_at"]) > datetime.now(timezone.utc),
            "D03_ARTIFACT_EXPIRATION_PASSED")
    members = exact_members(raw)
    refreshed = observe(run_path, "github-deployment-run-confirmed.json")
    require(all(refreshed.get(key) == run.get(key) for key in
                ("id", "head_sha", "status", "conclusion", "run_attempt", "updated_at")),
            "D03_DEPLOYMENT_CHANGED_DURING_READ")
    verify_checkout(source, implementation, COMPILER_ORIGIN)
    verified = output / "verified"
    verified.mkdir()
    for name, data in {**saved, **members, "original-publication-evidence.zip": raw}.items():
        (verified / name).write_bytes(data)
    # This is an adapter of observed API fields; original metadata is retained.
    deployment = {"headSha": run["head_sha"], "conclusion": run["conclusion"],
                  "status": run["status"], "url": run["html_url"],
                  "jobs": [{key: job.get(key) for key in ("name", "conclusion", "status")}
                           for job in jobs]}
    write_json(verified / "documentation-site-deployment.json", deployment)
    receipt = {"status": "PASS_PUBLICATION_INPUT_BINDING_ONLY", "implementationSha": implementation,
               "docsCommit": pins["docsCommit"], "consumerCompilerPin": pins["compilerCommit"],
               "deploymentRunId": deployment_run, "deploymentAttempt": attempt,
               "artifactId": artifact["id"], "originalArchiveSha256": digest(raw),
               "originalArchiveBytes": len(raw),
               "members": {name: digest(data) for name, data in members.items()},
               "metadata": {name: digest(data) for name, data in saved.items()},
               "liveCollectorExecuted": False, "appIntegrationExecuted": False,
               "formalAcceptanceGranted": False}
    write_json(verified / "publication-input-receipt.json", receipt)
    return receipt


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--source", type=Path, required=True)
    parser.add_argument("--implementation-sha", required=True)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    try:
        receipt = prepare(args.source, args.implementation_sha,
                          os.environ.get("CHERRI_D03_DEPLOYMENT_RUN_ID", ""),
                          os.environ.get("CHERRI_D03_PUBLICATION_ZIP_BASE64", ""), args.output)
    except (ValueError, OSError, KeyError, TypeError, subprocess.SubprocessError) as error:
        # Never echo a supplied ZIP, response body, credentials or unknown path.
        code = str(error) if isinstance(error, ValueError) and str(error).startswith("D03_") else type(error).__name__
        failure = {"status": "D03_PREPARATION_BLOCKED", "code": code,
                   "formalAcceptanceGranted": False}
        if isinstance(error, MetadataHTTPError):
            failure["http"] = error.public_details
        print(json.dumps(failure))
        return 1
    print(json.dumps(receipt))
    if os.environ.get("GITHUB_OUTPUT"):
        with open(os.environ["GITHUB_OUTPUT"], "a", encoding="utf-8") as stream:
            stream.write("docs_sha=" + receipt["docsCommit"] + "\n")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
