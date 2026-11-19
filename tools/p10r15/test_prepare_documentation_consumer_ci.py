"""Publication transport controls; HTTP metadata/site execution is modeled here."""
import base64
import copy
from datetime import datetime, timedelta, timezone
import io
import json
from pathlib import Path
import stat
import subprocess
import tempfile
import unittest
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


if __name__ == "__main__":
    unittest.main()
