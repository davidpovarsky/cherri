"""Causal checks for the exact public data binding; no Apple runtime is modeled."""
import copy
import importlib.util
import json
from pathlib import Path
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]
SPEC = importlib.util.spec_from_file_location("verify_paginate_probe_data", ROOT/"tools/p10r15/verify_paginate_probe_data.py")
MODULE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(MODULE)


class PaginateProbeDataTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.root = Path(self.temporary.name)
        self.addCleanup(self.temporary.cleanup)
        manifest = ROOT/MODULE.MANIFEST
        self.path = self.root/MODULE.MANIFEST
        self.path.parent.mkdir(parents=True)
        self.path.write_bytes(manifest.read_bytes())
        self.manifest = json.loads(self.path.read_bytes())
        data = self.root/MODULE.DATA_DIRECTORY
        data.mkdir(parents=True)
        for filename in MODULE.DATA_FILES:
            (data/filename).write_bytes((ROOT/MODULE.DATA_DIRECTORY/filename).read_bytes())
        for case in self.manifest["cases"]:
            target = self.root/case["path"]
            target.write_bytes((ROOT/case["path"]).read_bytes())

    def write(self, manifest):
        self.path.write_text(json.dumps(manifest), encoding="utf-8")

    def good_fetch(self, url):
        page = next(p for p in self.manifest["pages"] if p["url"] == url)
        return 200, url, (self.root/MODULE.DATA_DIRECTORY/page["fileName"]).read_bytes()

    def test_exact_binding_and_data_bytes(self):
        report = MODULE.verify(self.root, self.good_fetch)
        self.assertEqual(report["status"], "PASS_PUBLIC_SYNTHETIC_DATA_BYTES_ONLY")
        self.assertEqual(len(report["pages"]), 6)
        self.assertEqual(len(report["fixtureSourceHashes"]), 5)
        self.assertEqual(report["nativeExecuted"], 0)
        self.assertNotIn("body", json.dumps(report).lower())
        expected_text = json.loads((self.root/MODULE.DATA_DIRECTORY/"one.json").read_bytes())["items"][0]["text"]
        ordered = (self.root/"tools/shortcuts-simulator-qa/fixtures/p15-paginate-probes/qa-paginate-ordered-v2.cherri").read_text(encoding="utf-8")
        self.assertIn(expected_text, ordered)
        self.assertNotIn("????", ordered)

    def test_manifest_pin_and_path_mutations_fail(self):
        mutations = [
            lambda m: m.update(dataCommit="main"),
            lambda m: m["pages"][0].update(url="https://example.invalid/private"),
            lambda m: m["pages"][0].update(sha256="0"*64),
            lambda m: m["pages"][0].update(fileName="../one.json"),
            lambda m: m["cases"][0].update(path="../private.cherri"),
            lambda m: m["cases"][0].update(sourceSha256="0"*64),
            lambda m: m["cases"].append(copy.deepcopy(m["cases"][0])),
            lambda m: m.update(syntheticOnly=False),
        ]
        for mutation in mutations:
            with self.subTest(mutation=mutations.index(mutation)):
                manifest = copy.deepcopy(self.manifest); mutation(manifest); self.write(manifest)
                with self.assertRaises((ValueError, OSError)):
                    MODULE.verify(self.root, self.good_fetch)
                self.write(self.manifest)
                self.assertEqual(MODULE.verify(self.root, self.good_fetch)["nativeExecuted"], 0)

    def test_remote_status_redirect_hash_and_size_mutations_fail(self):
        for mutation in ("status404", "redirect", "hash", "size", "timeout"):
            with self.subTest(mutation=mutation):
                def fetch(url):
                    if mutation == "timeout":
                        raise TimeoutError("synthetic timeout")
                    status, final_url, body = self.good_fetch(url)
                    if mutation == "status404": status = 404
                    if mutation == "redirect": final_url = "https://example.invalid/redirect"
                    if mutation == "hash": body += b" "
                    if mutation == "size": body = b"x"*65537
                    return status, final_url, body
                with self.assertRaises((ValueError, TimeoutError)):
                    MODULE.verify(self.root, fetch)
                self.assertEqual(MODULE.verify(self.root, self.good_fetch)["nativeExecuted"], 0)

    def test_fixture_url_cannot_drift_under_a_fresh_hash(self):
        case = self.manifest["cases"][0]
        path = self.root/case["path"]
        data = path.read_bytes().replace(self.manifest["dataCommit"].encode(), b"0"*40)
        path.write_bytes(data)
        case["sourceSha256"] = MODULE.sha(data)
        self.write(self.manifest)
        with self.assertRaisesRegex(ValueError, "PAGINATE_DATA_FIXTURE_URL_MISMATCH"):
            MODULE.verify(self.root, self.good_fetch)

    def test_selected_pipeline_requires_data_verifier_success(self):
        import os
        import subprocess
        from test_workflow_input_contract import bash_path
        bash = bash_path()
        if not bash:
            self.skipTest("bash unavailable")
        script = (ROOT/"scripts/ios27_shortcuts_simulator_qa.sh").read_text(encoding="utf-8")
        hook = script.split("# Verify only the explicitly selected paginate targets", 1)[1]
        hook = "# Verify only the explicitly selected paginate targets" + hook.split("# Phase 1:", 1)[0]
        for scenario in MODULE.SCENARIOS:
            sample = "qa-paginate-" + scenario
            for exit_code in (0, 17):
                with self.subTest(sample=sample, exit_code=exit_code):
                    program = 'python3() { printf "%s\\n" "$@"; return ' + str(exit_code) + '; }\n'
                    program += hook + '\nprintf "NATIVE_STAGE_REACHED\\n"\n'
                    result = subprocess.run([bash, "-s"], input=program,
                        env={**os.environ, "SAMPLE_ID": sample, "ROOT_DIR": "/reviewed", "ARTIFACT_DIR": "/evidence"},
                        text=True, capture_output=True, timeout=10)
                    self.assertEqual(result.returncode, exit_code, result.stderr)
                    self.assertIn("/reviewed/tools/p10r15/verify_paginate_probe_data.py", result.stdout)
                    self.assertIn("/evidence/runtime/paginate-data-verification.json", result.stdout)
                    self.assertEqual("NATIVE_STAGE_REACHED" in result.stdout, exit_code == 0)
        program = 'python3() { printf "UNEXPECTED_NETWORK_DEPENDENCY\\n"; return 17; }\n' + hook
        program += '\nprintf "NATIVE_STAGE_REACHED\\n"\n'
        result = subprocess.run([bash, "-s"], input=program,
            env={**os.environ, "SAMPLE_ID": "qa-export-unicode", "ROOT_DIR": "/reviewed", "ARTIFACT_DIR": "/evidence"},
            text=True, capture_output=True, timeout=10)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(result.stdout, "NATIVE_STAGE_REACHED\n")


if __name__ == "__main__":
    unittest.main()
