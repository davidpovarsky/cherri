"""Publication transport controls; HTTP metadata/site execution is modeled here."""
import base64
import copy
from contextlib import contextmanager, redirect_stdout
from email.message import Message
from datetime import datetime, timedelta, timezone
import io
import json
import os
from pathlib import Path
import stat
import subprocess
import sys
import tempfile
import unittest
from unittest import mock
from urllib.request import HTTPHandler, HTTPSHandler, build_opener
from urllib.response import addinfourl
import warnings
import zipfile

import prepare_documentation_consumer_ci as GATE


class PublicationInputs(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix="cherri-d03-input-control-")
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.source = self.root / "source"
        self.source.mkdir()
        for args in (("init", "-q"), ("config", "user.name", "Synthetic D03 transport"),
                     ("config", "user.email", "synthetic@example.invalid"),
                     ("remote", "add", "origin", GATE.COMPILER_ORIGIN)):
            self.git(*args)
        self.pins = {"compilerRepo": GATE.COMPILER_ORIGIN,
                     "docsRepo": "https://github.com/" + GATE.DOCS_REPO + ".git",
                     "languageVersion": "2.0", "compilerCommit": "c" * 40,
                     "docsCommit": "d" * 40, "schemaFingerprint": "1" * 64,
                     "codecFingerprint": "2" * 64, "macroFingerprint": "3" * 64}
        target = self.source / "skills/cherri-shortcuts/compatibility-manifest.json"
        target.parent.mkdir(parents=True)
        target.write_bytes(self.encoded(self.pins))
        self.git("add", ".")
        self.git("commit", "-qm", "Synthetic immutable input")
        self.sha = self.git("rev-parse", "HEAD").strip()
        self.members = {name: self.encoded({"syntheticControl": name}) for name in GATE.MEMBERS}
        self.archive = self.zip(self.members.items())
        self.run_path = "actions/runs/123456"
        self.run = {"id": 123456, "head_sha": self.pins["docsCommit"], "status": "completed",
                    "conclusion": "success", "path": ".github/workflows/pages.yml", "event": "push",
                    "repository": {"id": 77, "full_name": GATE.DOCS_REPO},
                    "head_repository": {"id": 77, "full_name": GATE.DOCS_REPO},
                    "html_url": "https://github.com/" + GATE.DOCS_REPO + "/actions/runs/123456",
                    "run_attempt": 1, "run_started_at": "2026-11-19T10:00:00Z",
                    "updated_at": "2026-11-19T10:05:00Z"}
        self.jobs = {"total_count": 2, "jobs": [
            {"id": i, "name": name, "run_id": 123456, "head_sha": self.pins["docsCommit"],
             "status": "completed", "conclusion": "success"}
            for i, name in enumerate(("build", "deploy"), 1)]}
        self.artifact = {"id": 987, "name": GATE.ARTIFACT, "expired": False,
                         "size_in_bytes": len(self.archive), "digest": "sha256:" + GATE.digest(self.archive),
                         "expires_at": (datetime.now(timezone.utc) + timedelta(days=1)).isoformat().replace("+00:00", "Z"),
                         "created_at": "2026-11-19T10:04:00Z", "workflow_run": {
                             "id": 123456, "head_sha": self.pins["docsCommit"],
                             "repository_id": 77, "head_repository_id": 77}}
        self.artifacts = {"total_count": 1, "artifacts": [self.artifact]}
        self.fetches = []
        self.counter = 0

    @staticmethod
    def encoded(value):
        return (json.dumps(value, indent=2) + "\n").encode()

    @staticmethod
    def zip(members):
        stream = io.BytesIO()
        with warnings.catch_warnings():
            warnings.simplefilter("ignore", UserWarning)
            with zipfile.ZipFile(stream, "w", compression=zipfile.ZIP_DEFLATED) as archive:
                for name, content in members:
                    archive.writestr(name, content)
        return stream.getvalue()

    def git(self, *args):
        return subprocess.check_output(["git", "-C", str(self.source), *args], stderr=subprocess.PIPE).decode()

    def fetch(self, path):
        self.fetches.append(path)
        if path == self.run_path:
            return self.encoded(self.run)
        if path == self.run_path + "/attempts/1/jobs?per_page=100":
            return self.encoded(self.jobs)
        if path == self.run_path + "/artifacts?per_page=100":
            return self.encoded(self.artifacts)
        self.fail("unexpected metadata endpoint: " + path)

    def prepare(self, *, raw=None, fetch=None):
        self.counter += 1
        return GATE.prepare(self.source, self.sha, "123456",
                            base64.b64encode(self.archive if raw is None else raw).decode(),
                            self.root / ("result-" + str(self.counter)), fetch=fetch or self.fetch)

    def test_exact_public_bytes_retained_without_consumer_credit(self):
        result = self.prepare()
        verified = self.root / "result-1/verified"
        self.assertEqual((verified / "original-publication-evidence.zip").read_bytes(), self.archive)
        for name, raw in self.members.items():
            self.assertEqual((verified / name).read_bytes(), raw)
        self.assertEqual(result["originalArchiveSha256"], GATE.digest(self.archive))
        self.assertFalse(result["formalAcceptanceGranted"])
        self.assertFalse(result["liveCollectorExecuted"])
        self.assertEqual(self.git("status", "--porcelain"), "")
        self.assertEqual(self.fetches.count(self.run_path), 2)

    def test_real_source_revision_and_clean_checkout_required(self):
        with self.assertRaises(ValueError):
            GATE.prepare(self.source, "f" * 40, "123456", base64.b64encode(self.archive).decode(),
                         self.root / "wrong-revision", fetch=self.fetch)
        (self.source / "untracked.txt").write_text("uncommitted control")
        with self.assertRaisesRegex(ValueError, "Dirty checkout"):
            self.prepare()

    def test_deployment_and_artifact_identity_faults_rejected(self):
        originals = copy.deepcopy((self.run, self.jobs, self.artifacts))
        faults = {
            "wrong-source": lambda: self.run.update(head_sha="e" * 40),
            "wrong-workflow": lambda: self.run.update(path=".github/workflows/docs-smoke-test.yml"),
            "failed-run": lambda: self.run.update(conclusion="failure"),
            "missing-repository-id": lambda: self.run["repository"].pop("id"),
            "foreign-repository": lambda: self.run["repository"].update(full_name="other/docs"),
            "failed-deploy": lambda: self.jobs["jobs"][1].update(conclusion="failure"),
            "duplicate-build": lambda: self.jobs["jobs"][1].update(name="build"),
            "truncated-jobs": lambda: self.jobs.update(total_count=3),
            "expired-artifact": lambda: self.artifacts["artifacts"][0].update(expired=True),
            "expiration-passed": lambda: self.artifacts["artifacts"][0].update(expires_at="2000-01-01T00:00:00Z"),
            "missing-expiration": lambda: self.artifacts["artifacts"][0].update(expires_at=None),
            "different-digest": lambda: self.artifacts["artifacts"][0].update(digest="sha256:" + "0" * 64),
            "different-size": lambda: self.artifacts["artifacts"][0].update(size_in_bytes=1),
            "borrowed-run": lambda: self.artifacts["artifacts"][0]["workflow_run"].update(id=123457),
            "prior-attempt": lambda: self.artifacts["artifacts"][0].update(created_at="2026-11-19T09:59:59Z"),
            "missing-artifact": lambda: self.artifacts.update(total_count=0, artifacts=[]),
        }
        for name, fault in faults.items():
            self.run, self.jobs, self.artifacts = copy.deepcopy(originals)
            fault()
            with self.subTest(name=name), self.assertRaises(ValueError):
                self.prepare()
            self.assertFalse((self.root / ("result-" + str(self.counter)) / "verified").exists())

    def test_digest_failure_never_extracts_or_publishes_input(self):
        with self.assertRaisesRegex(ValueError, "D03_ARTIFACT_DIGEST_OR_SOURCE_BINDING"):
            self.prepare(raw=self.archive[:-1] + bytes([self.archive[-1] ^ 1]))
        self.assertFalse((self.root / "result-1/verified").exists())
        self.assertTrue((self.root / "result-1/quarantine/original-publication-evidence.zip").is_file())

    def test_exact_two_file_archive_rejects_unsafe_shapes(self):
        first, second = GATE.MEMBERS
        symbolic = zipfile.ZipInfo(first)
        symbolic.create_system = 3
        symbolic.external_attr = (stat.S_IFLNK | 0o777) << 16
        cases = {
            "extra": [(first, b"{}"), (second, b"{}"), ("extra.json", b"{}")],
            "duplicate": [(first, b"{}"), (first, b"{}")],
            "traversal": [("../" + first, b"{}"), (second, b"{}")],
            "backslash": [("nested\\" + first, b"{}"), (second, b"{}")],
            "symlink": [(symbolic, b"target"), (second, b"{}")],
            "oversized-member": [(first, b" " * (GATE.MAX_MEMBER_BYTES + 1)), (second, b"{}")],
            "not-json": [(first, b"not JSON"), (second, b"{}")],
            "duplicate-json-key": [(first, b'{"ok":true,"ok":false}'), (second, b"{}")],
        }
        for name, members in cases.items():
            with self.subTest(name=name), self.assertRaises(ValueError):
                GATE.exact_members(self.zip(members))

    def test_metadata_rerun_race_fails_before_verified_publication(self):
        count = 0
        def changed(path):
            nonlocal count
            if path == self.run_path:
                count += 1
                if count == 2:
                    value = copy.deepcopy(self.run)
                    value["run_attempt"] = 2
                    return self.encoded(value)
            return self.fetch(path)
        with self.assertRaisesRegex(ValueError, "D03_DEPLOYMENT_CHANGED_DURING_READ"):
            self.prepare(fetch=changed)
        self.assertFalse((self.root / "result-1/verified").exists())

    def test_input_bounds_and_noncanonical_encoding(self):
        for value in ("", "%%%", "YQ==\n", "A" * (4 * ((GATE.MAX_ZIP_BYTES + 2) // 3) + 1)):
            with self.subTest(value_length=len(value)), self.assertRaises(ValueError):
                GATE.decode_archive(value)



class PublicMetadataTransport(unittest.TestCase):
    endpoint = "actions/runs/123456"

    def setUp(self):
        env = mock.patch.dict(os.environ, {"CHERRI_D03_GITHUB_TOKEN": ""})
        env.start()
        self.addCleanup(env.stop)

    @contextmanager
    def transport(self, responder):
        requests = []

        def respond(handler, request):
            requests.append(request)
            code, values, body, url = responder(request)
            headers = Message()
            for key, value in values.items():
                headers[key] = value
            response = addinfourl(io.BytesIO(body), headers, url or request.full_url, code)
            response.msg = "Synthetic metadata response"
            return response

        class SyntheticHTTP(HTTPHandler):
            http_open = respond

        class SyntheticHTTPS(HTTPSHandler):
            https_open = respond

        def opener(*handlers):
            return build_opener(*handlers, SyntheticHTTP(), SyntheticHTTPS())

        with mock.patch.object(GATE, "build_opener", side_effect=opener):
            yield requests

    def test_existing_token_changes_anonymous_rejection_to_exact_public_bytes(self):
        original = b'{"original":"public metadata bytes"}\n'
        def server(request):
            if request.get_header("Authorization") == "Bearer synthetic-api-token":
                return 200, {}, original, None
            return 403, {"X-RateLimit-Remaining": "0"}, b"synthetic rejected body", None

        with self.transport(server) as requests:
            with self.assertRaisesRegex(GATE.MetadataHTTPError, "D03_METADATA_HTTP_403") as failed:
                GATE.public_metadata(self.endpoint)
            self.assertFalse(failed.exception.public_details["authenticated"])
            self.assertEqual(failed.exception.public_details["rateLimitRemaining"], 0)
            with mock.patch.dict(os.environ, {"CHERRI_D03_GITHUB_TOKEN": "synthetic-api-token"}):
                self.assertEqual(GATE.public_metadata(self.endpoint), original)
        self.assertEqual([r.full_url for r in requests], [GATE.API + self.endpoint] * 2)

    def test_only_fixed_public_api_endpoints_can_receive_a_token(self):
        invalid = ("https://other.invalid/run", "//other.invalid/run", "../other",
                   "actions/runs/0", "actions/runs/1?token=private",
                   "actions/runs/1/artifacts?per_page=100&redirect=other",
                   "actions/runs/1/attempts/0/jobs?per_page=100",
                   "actions/runs/1/%2e%2e/other")
        with mock.patch.dict(os.environ, {"CHERRI_D03_GITHUB_TOKEN": "synthetic-api-token"}), \
                mock.patch.object(GATE, "build_opener") as opener:
            for path in invalid:
                with self.subTest(path=path), self.assertRaisesRegex(ValueError, "D03_METADATA_ENDPOINT"):
                    GATE.public_metadata(path)
            opener.assert_not_called()

    def test_redirects_stop_before_a_second_request_or_token_forward(self):
        destinations = ("https://outside.invalid/private",
                        "http://api.github.com/private",
                        GATE.API + "actions/runs/999")
        for status in (301, 302, 303, 307, 308):
            for destination in destinations:
                with self.subTest(status=status, destination=destination), \
                        mock.patch.dict(os.environ, {"CHERRI_D03_GITHUB_TOKEN": "synthetic-api-token"}), \
                        self.transport(lambda request: (status, {"Location": destination},
                                                        b"synthetic redirect body", None)) as requests:
                    with self.assertRaisesRegex(GATE.MetadataHTTPError, "D03_METADATA_HTTP_" + str(status)):
                        GATE.public_metadata(self.endpoint)
                    self.assertEqual(len(requests), 1)
                    self.assertEqual(requests[0].full_url, GATE.API + self.endpoint)
                    self.assertEqual(requests[0].get_header("Authorization"), "Bearer synthetic-api-token")

    def test_http_errors_retain_status_endpoint_and_only_numeric_retry_metadata(self):
        for status in (401, 403, 404, 429, 503):
            with self.subTest(status=status), self.transport(lambda request: (
                    status, {"X-RateLimit-Remaining": "0", "X-RateLimit-Reset": "1795400000",
                             "Retry-After": "60", "X-Unreviewed": "synthetic-private-header"},
                    b"synthetic-private-response-body", None)):
                with self.assertRaises(GATE.MetadataHTTPError) as raised:
                    GATE.public_metadata(self.endpoint)
                self.assertEqual(raised.exception.public_details, {
                    "endpoint": self.endpoint, "status": status, "authenticated": False,
                    "rateLimitRemaining": 0, "rateLimitReset": 1795400000,
                    "retryAfterSeconds": 60})
                self.assertEqual(str(raised.exception), "D03_METADATA_HTTP_" + str(status))

    def test_unknown_retry_header_text_is_not_reported(self):
        with self.transport(lambda request: (
                403, {"X-RateLimit-Remaining": "synthetic-private-header",
                      "Retry-After": "9999999999999"}, b"private response", None)):
            with self.assertRaises(GATE.MetadataHTTPError) as raised:
                GATE.public_metadata(self.endpoint)
        self.assertEqual(raised.exception.public_details,
                         {"endpoint": self.endpoint, "status": 403, "authenticated": False})

    def test_response_identity_and_byte_limit_still_fail_closed(self):
        for body, url, code in ((b"{}", "https://outside.invalid", "D03_METADATA_HTTP_IDENTITY"),
                                (b"x" * (GATE.MAX_METADATA_BYTES + 1), None, "D03_METADATA_SIZE_LIMIT")):
            with self.subTest(code=code), self.transport(lambda request: (200, {}, body, url)):
                with self.assertRaisesRegex(ValueError, code):
                    GATE.public_metadata(self.endpoint)

    def test_main_prints_public_http_details_without_payload_or_token(self):
        failure = GATE.MetadataHTTPError(self.endpoint, 403, {"X-RateLimit-Remaining": "0"}, True)
        stream = io.StringIO()
        with mock.patch.object(GATE, "prepare", side_effect=failure), \
                mock.patch.object(sys, "argv", ["prepare", "--source", "/synthetic-source",
                                               "--implementation-sha", "a" * 40,
                                               "--output", "/synthetic-output"]), \
                mock.patch.dict(os.environ, {"CHERRI_D03_GITHUB_TOKEN": "synthetic-api-token",
                                             "CHERRI_D03_PUBLICATION_ZIP_BASE64": "synthetic-private-archive"}), \
                redirect_stdout(stream):
            self.assertEqual(GATE.main(), 1)
        result = json.loads(stream.getvalue())
        self.assertEqual(result["code"], "D03_METADATA_HTTP_403")
        self.assertEqual(result["http"], failure.public_details)
        self.assertFalse(result["formalAcceptanceGranted"])
        for private in ("synthetic-api-token", "synthetic-private-archive", "Authorization"):
            self.assertNotIn(private, stream.getvalue())

    def test_whitespace_token_fails_before_any_network_request(self):
        for token in (" leading", "trailing ", "line\nbreak", "line\rbreak"):
            with self.subTest(token=repr(token)), \
                    mock.patch.dict(os.environ, {"CHERRI_D03_GITHUB_TOKEN": token}), \
                    mock.patch.object(GATE, "build_opener") as opener:
                with self.assertRaisesRegex(ValueError, "D03_METADATA_TOKEN_FORMAT"):
                    GATE.public_metadata(self.endpoint)
                opener.assert_not_called()


if __name__ == "__main__":
    unittest.main()
