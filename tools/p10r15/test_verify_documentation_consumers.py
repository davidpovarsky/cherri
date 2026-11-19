"""Collector controls use actual pinned shell scripts and explicitly modeled HTTP.

These tests do not publish a website, install an Apple app, or produce a
production acceptance record. Temporary Git repositories have synthetic data.
"""
import copy
import importlib.util
import json
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest


HERE = Path(__file__).resolve().parent
SPEC = importlib.util.spec_from_file_location("documentation_consumers", HERE / "verify_documentation_consumers.py")
GATE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(GATE)


class DocumentationCollectorTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory(prefix="cherri-d03-collector-model-")
        self.root = Path(self.temporary.name)
        self.source = self.root / "source"
        self.docs = self.root / "docs"
        self.skill = self.root / "installed-skill"
        for directory, origin in ((self.source, GATE.COMPILER_ORIGIN), (self.docs, GATE.DOCS_ORIGIN)):
            directory.mkdir()
            self.git(directory, "init", "-q")
            self.git(directory, "config", "core.autocrlf", "false")
            self.git(directory, "config", "user.name", "Synthetic D03 verifier test")
            self.git(directory, "config", "user.email", "synthetic@example.invalid")
            self.git(directory, "remote", "add", "origin", origin)
        for name in GATE.SOURCE_PATHS:
            target = self.docs / name
            target.parent.mkdir(exist_ok=True)
            target.write_text("# Current documentation\nshow(\"synthetic public control\")\n")
        (self.docs / "language").mkdir()
        (self.docs / "language/raw-actions.md").write_text("# Historical reference\naction rawAction\n")
        self.docs_commit = self.commit(self.docs)
        self.pins = {"skillVersion": "2.0.0", "languageVersion": "2.0", "catalogSchemaVersion": "3.0",
                     "compilerRepo": GATE.COMPILER_ORIGIN, "docsRepo": GATE.DOCS_ORIGIN,
                     "compilerCommit": "a" * 40, "docsCommit": self.docs_commit,
                     "schemaFingerprint": "1" * 64, "codecFingerprint": "2" * 64, "macroFingerprint": "3" * 64}
        original = HERE.parents[1] / "skills/cherri-shortcuts"
        for name in ("scripts/search-docs.sh", "scripts/common.sh"):
            target = self.source / "skills/cherri-shortcuts" / name
            target.parent.mkdir(parents=True, exist_ok=True)
            target.write_bytes((original / name).read_bytes())
        (self.source / "skills/cherri-shortcuts/compatibility-manifest.json").write_text(
            json.dumps(self.pins, indent=2) + "\n")
        self.implementation = self.commit(self.source)
        shutil.copytree(self.source / "skills/cherri-shortcuts", self.skill)
        self.bodies = {route: ("Synthetic fixture for " + route).encode() for route, _ in GATE.ROUTES}
        self.bodies["fork-language/actions"] = b"<html>Current generated show action</html>"
        self.bodies["assets/js/search-data.json"] = self.encode({
            "0": {"doc": "Actions", "title": "show", "content": "Current V2 show",
                  "url": "/cherrilang.org/fork-language/actions#show",
                  "relUrl": "/fork-language/actions#show"},
            "1": {"doc": "Guide", "title": "Current guide", "content": "Current V2 guide",
                  "url": "/cherrilang.org/fork-language/guide", "relUrl": "/fork-language/guide"},
        })
        self.build = {
            "formatVersion": 2, "baseUrl": "/cherrilang.org", "publicationOrigin": "https://davidpovarsky.github.io",
            "currentSearchRoutes": ["/cherrilang.org", "/cherrilang.org/fork-language",
                                    "/cherrilang.org/fork-language/actions", "/cherrilang.org/fork-language/guide",
                                    "/cherrilang.org/fork-language/migration"],
            "verifiedLocalLinks": 5, "historicalSearchEntries": 0,
            "docsCommit": self.docs_commit, "compilerCommit": self.pins["compilerCommit"],
            "sourceTreeClean": True, "sourceReferenceSha256": GATE.digest((self.docs / "fork-language/actions.md").read_bytes()),
            "renderedActionPage": "fork-language/actions.html",
            "renderedActionPageSha256": GATE.digest(self.bodies["fork-language/actions"]),
            "searchIndexSha256": GATE.digest(self.bodies["assets/js/search-data.json"]),
            "schemaFingerprints": [self.pins["schemaFingerprint"], self.pins["macroFingerprint"]],
            "nativeActionCount": 1, "renderedReferenceSignatures": 1,
            "searchEntries": 2, "actionSearchEntries": 1, "sitePublished": False,
        }
        diagnostics = (
            "Default search includes a historical route: /language/obsolete",
            "Search result URL does not match its project-base route",
            "Search client does not load and use the verified project URLs",
            "Home page lacks its unique current-guide entry point",
            "Built site is missing a unique page: /assets/missing-control.png",
            "rendered action page lost canonical metadata: fingerprints=[]",
        )
        self.controls = {"formatVersion": 1, "docsCommit": self.docs_commit,
                         "compilerCommit": self.pins["compilerCommit"],
                         "passed": 6, "failed": 0, "skipped": 0, "artifactsRestoredExactly": True,
                         "controls": [{"name": name, "rejected": True, "diagnostic": diagnostic}
                                      for name, diagnostic in zip(GATE.CONTROL_NAMES, diagnostics)]}
        self.deployment = {"headSha": self.docs_commit, "conclusion": "success", "status": "completed",
                           "url": "https://github.com/davidpovarsky/cherrilang.org/actions/runs/123456",
                           "jobs": [{"name": name, "conclusion": "success", "status": "completed"}
                                    for name in ("build", "deploy")]}
        self.build_path, self.controls_path, self.deploy_path = [
            self.root / name for name in ("build.json", "controls.json", "deployment.json")]
        self.write_metadata()

    def tearDown(self):
        self.temporary.cleanup()

    @staticmethod
    def encode(value):
        return (json.dumps(value, indent=2) + "\n").encode()

    @staticmethod
    def git(root, *args):
        return subprocess.check_output(["git", "-C", str(root), *args], stderr=subprocess.PIPE).decode().strip()

    def commit(self, root):
        self.git(root, "add", ".")
        self.git(root, "commit", "-q", "-m", "Synthetic D03 fixture")
        return self.git(root, "rev-parse", "HEAD")

    def write_metadata(self):
        for path, value in ((self.build_path, self.build), (self.controls_path, self.controls),
                            (self.deploy_path, self.deployment)):
            path.write_bytes(self.encode(value))

    def fetch(self, route):
        return self.bodies[route], {"Content-Type": "application/json" if route.endswith(".json") else "application/javascript" if route.endswith(".js") else "text/html"}

    def collect(self, suffix="proof", execute=GATE.execute_owned):
        return GATE.collect(self.source, self.skill, self.docs, self.implementation,
                            self.build_path, self.controls_path, self.deploy_path, self.root / suffix,
                            fetch=self.fetch, execute=execute)

    def test_actual_installed_shell_matrix_with_explicitly_modeled_http(self):
        report, record = self.collect()
        self.assertEqual(len(report["searchCases"]), 8)
        self.assertEqual(len(report["httpObservations"]), 7)
        self.assertEqual(len(record["artifacts"]), 34)
        self.assertEqual(record["requirements"], ["D03"])
        self.assertEqual(record["tier"], "docs")
        self.assertEqual(report["before"], report["after"])
        self.assertEqual(self.git(self.source, "status", "--porcelain"), "")
        self.assertEqual(self.git(self.docs, "status", "--porcelain"), "")
        self.assertNotIn("appIntegration", record["assertions"])

    def test_installed_wrapper_drift_is_rejected_before_execution(self):
        path = self.skill / "scripts/search-docs.sh"
        path.write_bytes(path.read_bytes() + b"\n# drift\n")
        with self.assertRaisesRegex(ValueError, "Installed Skill bytes differ"):
            self.collect()

    def test_repository_copy_cannot_claim_installed_consumer(self):
        self.skill = self.source / "skills/cherri-shortcuts"
        with self.assertRaisesRegex(ValueError, "installed/packaged Skill"):
            self.collect()

    def test_dirty_and_wrong_origin_docs_are_rejected(self):
        (self.docs / "untracked.txt").write_text("control")
        with self.assertRaisesRegex(ValueError, "Dirty checkout"):
            self.collect("dirty")
        (self.docs / "untracked.txt").unlink()
        self.git(self.docs, "remote", "set-url", "origin", "https://example.invalid/docs.git")
        with self.assertRaisesRegex(ValueError, "origin mismatch"):
            self.collect("wrong-origin")

    def test_actual_error_controls_cannot_be_changed_to_success(self):
        for name, args in (("invalid-regex", ["["]), ("missing-current", [GATE.MISSING_QUERY])):
            def corrupted(command, env, cwd, target=args):
                if command[2:] == target:
                    return 0, b"", b"", False
                return GATE.execute_owned(command, env, cwd)
            with self.subTest(name=name), self.assertRaises(ValueError):
                self.collect("false-pass-" + name, corrupted)

    def test_timeout_and_historical_result_are_rejected(self):
        def timeout(command, env, cwd):
            return 0, b"", b"", True
        with self.assertRaisesRegex(ValueError, "timed out"):
            self.collect("timeout", timeout)

        def historical(command, env, cwd):
            if command[2:] == ["show"]:
                return 0, (str(self.docs / "language/old.md") + ":1:show old\n").encode(), b"", False
            return GATE.execute_owned(command, env, cwd)
        with self.assertRaisesRegex(ValueError, "escape"):
            self.collect("historical", historical)

    def test_live_bytes_must_match_the_pinned_deployment_build(self):
        self.bodies["fork-language/actions"] += b"changed"
        with self.assertRaisesRegex(ValueError, "Live action page differs"):
            self.collect()

    def test_historical_and_unbased_search_remain_rejected_when_hashes_are_recomputed(self):
        original = copy.deepcopy(self.bodies)
        for name, update in (
                ("historical", {"url": "/cherrilang.org/language/old", "relUrl": "/language/old"}),
                ("unbased", {"url": "/fork-language/actions#show"})):
            with self.subTest(name=name):
                self.bodies = copy.deepcopy(original)
                data = json.loads(self.bodies["assets/js/search-data.json"])
                data["0"].update(update)
                self.bodies["assets/js/search-data.json"] = self.encode(data)
                self.build["searchIndexSha256"] = GATE.digest(self.bodies["assets/js/search-data.json"])
                self.write_metadata()
                with self.assertRaisesRegex(ValueError, "Historical|Unbased"):
                    self.collect(name)

    def test_failed_stale_duplicate_or_incomplete_publication_is_rejected(self):
        original = copy.deepcopy(self.deployment)
        for name, change in (
                ("failed", lambda d: d.update(conclusion="failure")),
                ("stale", lambda d: d.update(headSha="b" * 40)),
                ("missing", lambda d: d.update(jobs=d["jobs"][:1])),
                ("duplicate", lambda d: d["jobs"].append(copy.deepcopy(d["jobs"][0])))):
            self.deployment = copy.deepcopy(original)
            change(self.deployment)
            self.write_metadata()
            with self.subTest(name=name), self.assertRaises(ValueError):
                self.collect(name)

    def test_site_controls_require_all_independent_actual_diagnostics(self):
        original = copy.deepcopy(self.controls)
        for name, change in (
                ("missing", lambda d: d["controls"].pop()),
                ("duplicate", lambda d: d["controls"].__setitem__(1, copy.deepcopy(d["controls"][0]))),
                ("empty", lambda d: d["controls"][0].update(diagnostic="")),
                ("not-restored", lambda d: d.update(artifactsRestoredExactly=False))):
            self.controls = copy.deepcopy(original)
            change(self.controls)
            self.write_metadata()
            with self.subTest(name=name), self.assertRaises(ValueError):
                self.collect(name)

    def test_duplicate_json_and_nonempty_output_fail_closed(self):
        with self.assertRaisesRegex(ValueError, "Duplicate JSON"):
            GATE.load_json(b'{"passed":6,"passed":0}')
        with self.assertRaisesRegex(ValueError, "Nonfinite JSON"):
            GATE.load_json(b'{"duration":NaN}')
        output = self.root / "occupied"
        output.mkdir()
        (output / "keep.txt").write_text("unchanged")
        with self.assertRaisesRegex(ValueError, "not empty"):
            self.collect("occupied")
        self.assertEqual((output / "keep.txt").read_text(), "unchanged")


if __name__ == "__main__":
    unittest.main()
