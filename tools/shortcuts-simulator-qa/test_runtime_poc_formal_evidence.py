#!/usr/bin/env python3
"""Synthetic exporter controls; these are not Simulator acceptance evidence."""
import copy
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

import runtime_poc_formal_evidence as formal
from runtime_poc_publication import check_sources


def create_control_fixture(destination, source_repo, include_exporter=True):
    """Create a local Git/receipt fixture, never a real or published runtime run."""
    destination = Path(destination)
    repo = destination / "repo"
    root = destination / "artifacts"
    repo.mkdir(parents=True)
    root.mkdir()
    for name in formal.SOURCE_HASHES:
        relative = Path("tests/runtime_poc") / (name + ".cherri")
        path = repo / relative
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_bytes((Path(source_repo) / relative).read_bytes())
    if include_exporter:
        path = repo / "tools/shortcuts-simulator-qa/runtime_poc_formal_evidence.py"
        path.parent.mkdir(parents=True)
        path.write_bytes(Path(formal.__file__).read_bytes())
    hooks = destination / "empty-hooks"
    hooks.mkdir()
    subprocess.run(["git", "init", "-q", str(repo)], check=True, capture_output=True)
    subprocess.run(["git", "-C", str(repo), "config", "core.autocrlf", "false"], check=True)
    subprocess.run(["git", "-C", str(repo), "add", "."], check=True, capture_output=True)
    subprocess.run(["git", "-C", str(repo), "-c", "user.name=P15 fixture control",
                    "-c", "user.email=control@example.invalid", "-c", "core.hooksPath=" + str(hooks),
                    "commit", "-qm", "Synthetic formal-export control"], check=True, capture_output=True)
    commit = subprocess.check_output(["git", "-C", str(repo), "rev-parse", "HEAD"], text=True).strip()
    run_id, attempt = "900000001", "1"
    check_sources(repo, root, commit, run_id, attempt)
    values = {}
    for _, expected, _ in formal.FORMAL_SCENARIOS.values():
        values.update({key: value for key, value in expected.values()})
    clipboard = "CHERRI_IOS27_RUNTIME_OK\n" + "\n".join(key + "=" + value for key, value in values.items()) + "\n"
    for name in formal.FORMAL_FILES:
        if name.endswith(".cherri"):
            data = (repo / "tests/runtime_poc" / name).read_bytes()
        elif name.endswith(".shortcut"):
            # Header-only synthetic protocol fixture: no Apple validity claim.
            data = b"AEA1-SYNTHETIC-EXPORTER-UNIT-CONTROL"
        elif name == "clipboard-result.txt":
            data = clipboard.encode("utf-8")
        elif name.endswith(".json"):
            data = b'{"unitControl":true}\n'
        else:
            data = b"Synthetic exporter unit control\n"
        (root / name).write_bytes(data)
    artifacts = [{"path": formal.CI_PREFIX + name, "sha256": formal.sha((root / name).read_bytes()),
                  "run_id": run_id} for name in sorted(formal.FORMAL_FILES)]
    records = []
    for evidence_id, test in formal.RAW_IDENTITIES.items():
        record = {"evidence_id": evidence_id, "requirements": ["OLD_DIAGNOSTIC_NAMESPACE"],
                  "tier": "ios-runtime", "test_id": "ios27-runtime-poc:" + test,
                  "implementation_sha": commit, "result": "passed", "exit_code": 0,
                  "structural_pass": True, "runtime_pass": True,
                  "assertions": {"raw_control": True, "cases": ["Alpha", "Beta"]},
                  "artifacts": copy.deepcopy(artifacts), "detail": "Synthetic control, not a device run"}
        if evidence_id in formal.FORMAL_SCENARIOS:
            record["assertions"] = {key: golden for key, (_, golden)
                                    in formal.FORMAL_SCENARIOS[evidence_id][1].items()}
        records.append(record)
    (root / "runtime-diagnostics.json").write_text(json.dumps(records, indent=2) + "\n", encoding="utf-8")
    control = {"repo": str(repo), "root": str(root), "commit": commit,
               "run_id": run_id, "attempt": attempt, "synthetic_control_only": True}
    (destination / "control.json").write_text(json.dumps(control) + "\n", encoding="utf-8")
    return control


class FormalEvidenceTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory(prefix="cherri-formal-control-")
        self.addCleanup(self.temporary.cleanup)
        source = Path(os.environ.get("CHERRI_FORMAL_TEST_REPO", Path(__file__).resolve().parents[2]))
        self.control = create_control_fixture(self.temporary.name, source)
        self.root = Path(self.control["root"])
        self.path = self.root / "runtime-diagnostics.json"

    def records(self):
        return json.loads(self.path.read_text(encoding="utf-8"))

    def save(self, records):
        self.path.write_text(json.dumps(records, indent=2) + "\n", encoding="utf-8")

    def export(self):
        c = self.control
        return formal.export_records(c["repo"], c["root"], "runtime-diagnostics.json",
                                     c["commit"], c["run_id"], c["attempt"])

    def selected(self, records, suffix="variables"):
        return next(record for record in records if record["evidence_id"] == "ev-runtime-" + suffix)

    def refresh_artifact(self, records, name):
        digest = formal.sha((self.root / name).read_bytes())
        for record in records:
            for artifact in record["artifacts"]:
                if artifact["path"] == formal.CI_PREFIX + name:
                    artifact["sha256"] = digest

    def test_exact_scope_preserves_raw_schema_and_observations(self):
        original = self.path.read_bytes()
        records = self.export()
        self.assertEqual(self.path.read_bytes(), original)
        self.assertEqual(len(records), 4)
        self.assertEqual({identifier for record in records for identifier in record["requirements"]},
                         {"V01", "V03", "L02", "L04", "N01", "FN01", "FN02", "FN03"})
        for record in records:
            raw = next(value for value in self.records() if value["evidence_id"] == record["evidence_id"])
            for key in ("artifacts", "implementation_sha", "test_id", "result", "exit_code",
                        "runtime_pass", "structural_pass"):
                self.assertEqual(record[key], raw[key])
            self.assertEqual(record["assertions"]["diagnostics_sha256"], formal.sha(original))
            self.assertTrue(all(isinstance(value, str) for value in record["assertions"].values()))

    def test_selected_failure_is_retained_not_dropped_or_upgraded(self):
        raw = self.records()
        record = self.selected(raw)
        record.update(result="failed", exit_code=7, runtime_pass=False)
        self.save(raw)
        output = self.selected(self.export())
        self.assertEqual((output["result"], output["exit_code"], output["runtime_pass"]), ("failed", 7, False))
        self.assertEqual(len(self.export()), 4)

    def test_failed_missing_clipboard_values_stay_failed(self):
        raw = self.records()
        for record in raw:
            record.update(result="failed", exit_code=1, runtime_pass=False)
            if record["evidence_id"] in formal.FORMAL_SCENARIOS:
                record["assertions"] = {key: "" for key in record["assertions"]}
        (self.root / "clipboard-result.txt").write_bytes(b"BEFORE_RUNTIME_TEST")
        self.refresh_artifact(raw, "clipboard-result.txt")
        self.save(raw)
        self.assertTrue(all(record["result"] == "failed" for record in self.export()))

    def test_missing_duplicate_and_unknown_raw_identity_fail_closed(self):
        original = self.records()
        for fault in ("missing", "duplicate", "unknown", "wrong_test"):
            with self.subTest(fault=fault):
                records = copy.deepcopy(original)
                if fault == "missing":
                    records.pop()
                elif fault == "duplicate":
                    records[-1] = copy.deepcopy(records[0])
                elif fault == "unknown":
                    records[-1]["evidence_id"] = "ev-runtime-unreviewed"
                else:
                    records[-1]["test_id"] = "ios27-runtime-poc:unreviewed"
                self.save(records)
                with self.assertRaises(formal.PublicationBlocked):
                    self.export()

    def test_stale_source_run_and_artifact_digest_fail_closed(self):
        original = self.records()
        for fault in ("commit", "run", "digest", "missing_artifact", "duplicate_artifact"):
            with self.subTest(fault=fault):
                records = copy.deepcopy(original)
                record = self.selected(records)
                if fault == "commit":
                    record["implementation_sha"] = "f" * 40
                elif fault == "run":
                    record["artifacts"][0]["run_id"] = "900000002"
                elif fault == "digest":
                    record["artifacts"][0]["sha256"] = "f" * 64
                elif fault == "missing_artifact":
                    record["artifacts"].pop()
                else:
                    record["artifacts"][-1] = copy.deepcopy(record["artifacts"][0])
                self.save(records)
                with self.assertRaises(formal.PublicationBlocked):
                    self.export()

    def test_assertions_must_be_strings_bound_to_clipboard(self):
        original = self.records()
        for fault in ("wrong_type", "detached", "missing", "extra"):
            with self.subTest(fault=fault):
                records = copy.deepcopy(original)
                assertions = self.selected(records)["assertions"]
                if fault == "wrong_type":
                    assertions["num_captured"] = 1
                elif fault == "detached":
                    assertions["num_captured"] = "2"
                elif fault == "missing":
                    del assertions["num_captured"]
                else:
                    assertions["unreviewed"] = "pass"
                self.save(records)
                with self.assertRaises(formal.PublicationBlocked):
                    self.export()

    def test_success_cannot_bind_wrong_computed_value(self):
        records = self.records()
        self.selected(records)["assertions"]["num_captured"] = "2"
        path = self.root / "clipboard-result.txt"
        path.write_bytes(path.read_bytes().replace(b"NUM_CAPTURED=1", b"NUM_CAPTURED=2"))
        self.refresh_artifact(records, "clipboard-result.txt")
        self.save(records)
        with self.assertRaises(formal.PublicationBlocked):
            self.export()

    def test_contradictory_nonformal_menu_success_still_blocks_export(self):
        records = self.records()
        self.selected(records, "menu-structural")["runtime_pass"] = False
        self.save(records)
        with self.assertRaises(formal.PublicationBlocked):
            self.export()

    def test_source_receipt_and_reviewed_source_are_mandatory(self):
        path = self.root / formal.SOURCE_RECEIPT
        original = path.read_bytes()
        path.write_bytes(b"{}")
        with self.assertRaises(formal.PublicationBlocked):
            self.export()
        path.write_bytes(original)
        source = Path(self.control["repo"]) / "tests/runtime_poc/CherriRuntimePOC.cherri"
        source.write_bytes(source.read_bytes() + b"\n// changed source\n")
        with self.assertRaises(formal.PublicationBlocked):
            self.export()

    def test_old_revision_without_exporter_cannot_be_relabelled(self):
        with tempfile.TemporaryDirectory(prefix="cherri-old-formal-control-") as other:
            source = Path(os.environ.get("CHERRI_FORMAL_TEST_REPO", Path(__file__).resolve().parents[2]))
            c = create_control_fixture(other, source, include_exporter=False)
            with self.assertRaises(subprocess.CalledProcessError):
                formal.export_records(c["repo"], c["root"], "runtime-diagnostics.json",
                                      c["commit"], c["run_id"], c["attempt"])

    def command(self):
        c = self.control
        return [sys.executable, "-B", str(Path(formal.__file__)), "--repo", c["repo"],
                "--root", c["root"], "--commit", c["commit"], "--run-id", c["run_id"],
                "--attempt", c["attempt"]]

    def test_actual_cli_preserves_inputs_and_refuses_reuse(self):
        original = self.path.read_bytes()
        first = subprocess.run(self.command(), capture_output=True, text=True)
        self.assertEqual(first.returncode, 0, first.stderr)
        output = self.root / "runtime-evidence.json"
        published = output.read_bytes()
        self.assertEqual(json.loads(first.stdout)["formalRecords"], 4)
        self.assertEqual(self.path.read_bytes(), original)
        second = subprocess.run(self.command(), capture_output=True, text=True)
        self.assertEqual(second.returncode, 2)
        self.assertEqual(output.read_bytes(), published)
        self.assertEqual(self.path.read_bytes(), original)

    def test_actual_cli_invalid_input_creates_no_formal_receipt(self):
        records = self.records()
        records.pop()
        self.save(records)
        result = subprocess.run(self.command(), capture_output=True, text=True)
        self.assertEqual(result.returncode, 2)
        self.assertFalse((self.root / "runtime-evidence.json").exists())


if __name__ == "__main__":
    if len(sys.argv) == 5 and sys.argv[1] == "--fixture-only" and sys.argv[3] == "--source-repo":
        print(json.dumps(create_control_fixture(sys.argv[2], sys.argv[4])))
    else:
        unittest.main()
