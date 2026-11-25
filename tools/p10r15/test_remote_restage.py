"""Real file controls for repeated immutable Remote Builder staging.

The input bytes are explicit synthetic fixtures. No compiler, signing endpoint
or installed Remote Builder is run by these component tests.
"""
import copy
import hashlib
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

import remote_stage_baseline as baseline
from remote_release_gates import PROTECTED, TOOL_PATHS


class SuccessorStageBaseline(unittest.TestCase):
    def setUp(self):
        temporary = tempfile.TemporaryDirectory(prefix="p15-successor-stage-")
        self.addCleanup(temporary.cleanup)
        self.root = Path(temporary.name)
        self.live = self.root / "live"
        self.tuple = {
            "label": "p15-synthetic-old",
            "compilerCommit": "1" * 40, "docsCommit": "2" * 40,
            "skillSourceHead": "3" * 40,
            "schemaFingerprint": "4" * 64, "codecFingerprint": "5" * 64,
            "macroFingerprint": "6" * 64, "compilerSHA256": "7" * 64,
            "migrationSHA256": "8" * 64, "sourceBranch": "agent/synthetic-old",
        }
        hashes = {}
        for name in sorted(set(PROTECTED) | TOOL_PATHS):
            path = self.live / name
            path.parent.mkdir(parents=True, exist_ok=True)
            raw = json.dumps(self.tuple).encode() if name == "active.json" else (
                "explicit synthetic original: " + name).encode()
            path.write_bytes(raw)
            hashes[name] = hashlib.sha256(raw).hexdigest()
        self.record = {
            "status": "STAGED_NOT_ACTIVATED", "livePromotion": False,
            "tuple": self.tuple, "candidateHashes": hashes,
            "externalToolchainDependencies": {name: hashes[name] for name in TOOL_PATHS},
        }
        self.path = self.root / "previous-stage.json"
        self.save()
        # Synthetic fixture content is intentionally not claimed to be the
        # production linter or production wrappers. Live K13 replay is separate.
        self.addCleanup(patch.stopall)
        patch.dict(baseline.ORIGINAL_LIVE_HASHES, {
            name: hashes[name] for name in baseline.ORIGINAL_LIVE_HASHES}).start()

    def save(self, record=None):
        self.path.write_text(json.dumps(self.record if record is None else record))
        self.sha = hashlib.sha256(self.path.read_bytes()).hexdigest()

    def load(self):
        return baseline.load_previous_stage(self.path, self.sha, self.live)

    def test_complete_current_release_is_read_only_and_binds_all_ten_files(self):
        before = {str(p): p.read_bytes() for p in self.root.rglob("*") if p.is_file()}
        hashes, witness = self.load()
        self.assertEqual(len(hashes), 10)
        self.assertEqual(hashes, self.record["candidateHashes"])
        self.assertEqual(witness["tuple"], self.tuple)
        self.assertTrue(witness["liveBytesMatch"])
        self.assertFalse(witness["acceptanceGranted"])
        self.assertEqual(before, {str(p): p.read_bytes() for p in self.root.rglob("*") if p.is_file()})

    def test_each_stale_live_file_is_rejected_without_restamping(self):
        for name in self.record["candidateHashes"]:
            with self.subTest(file=name):
                path = self.live / name
                original = path.read_bytes()
                path.write_bytes(original + b"\nchanged")
                with self.assertRaisesRegex(ValueError, "digest differs"):
                    self.load()
                self.assertEqual(path.read_bytes(), original + b"\nchanged")
                path.write_bytes(original)

    def test_changed_receipt_is_rejected_even_when_json_is_valid(self):
        self.path.write_text(self.path.read_text() + "\n")
        with self.assertRaisesRegex(ValueError, "digest differs"):
            self.load()

    def test_duplicate_receipt_key_is_rejected(self):
        self.path.write_text('{"status":"other",' + self.path.read_text()[1:])
        self.sha = hashlib.sha256(self.path.read_bytes()).hexdigest()
        with self.assertRaisesRegex(ValueError, "Duplicate JSON key"):
            self.load()

    def test_missing_and_extra_protected_paths_are_rejected(self):
        for mutate in ("missing", "extra"):
            with self.subTest(mutate=mutate):
                record = copy.deepcopy(self.record)
                if mutate == "missing":
                    record["candidateHashes"].pop("active.json")
                else:
                    record["candidateHashes"]["unrelated"] = "a" * 64
                self.save(record)
                with self.assertRaisesRegex(ValueError, "protected inventory"):
                    self.load()

    def test_active_tuple_mismatch_is_rejected_after_digest_matches(self):
        self.record["tuple"] = dict(self.tuple, compilerCommit="f" * 40)
        self.save()
        with self.assertRaisesRegex(ValueError, "active tuple differs"):
            self.load()

    def test_toolchain_binding_mismatch_is_rejected(self):
        self.record["externalToolchainDependencies"]["tools/go/VERSION"] = "a" * 64
        self.save()
        with self.assertRaisesRegex(ValueError, "toolchain binding"):
            self.load()

    def test_changed_validator_requires_template_review(self):
        baseline.ORIGINAL_LIVE_HASHES["bin/cherri-shortcut-lint.py"] = "a" * 64
        with self.assertRaisesRegex(ValueError, "validator requires"):
            self.load()

    def test_receipt_symlink_is_rejected(self):
        real = self.path.with_suffix(".original")
        self.path.rename(real)
        self.path.symlink_to(real)
        with self.assertRaisesRegex(ValueError, "regular original"):
            self.load()

    def test_live_file_symlink_is_rejected(self):
        path = self.live / "bin/cherri-remote-runner.sh"
        real = path.with_suffix(".original")
        path.rename(real)
        path.symlink_to(real)
        with self.assertRaisesRegex(ValueError, "regular original"):
            self.load()

    def test_predecessor_options_are_all_required(self):
        runner = self.live / "bin/cherri-remote-runner.sh"
        legacy = self.live / "bin/cherri-remote-legacy.sh"
        options = [self.path, self.sha, runner, legacy]
        for index in range(4):
            with self.subTest(missing=index):
                selected = list(options)
                selected[index] = None
                with self.assertRaisesRegex(ValueError, "requires its receipt"):
                    baseline.wrapper_templates(self.live, *selected)

    def test_initial_and_successor_routes_use_the_same_pinned_original_templates(self):
        runner = self.live / "bin/cherri-remote-runner.sh"
        legacy = self.live / "bin/cherri-remote-legacy.sh"
        initial = baseline.wrapper_templates(self.live, None, None, None, None)
        successor = baseline.wrapper_templates(self.live, self.path, self.sha, runner, legacy)
        self.assertIsNone(initial[1])
        self.assertEqual(initial[2], successor[2])
        self.assertEqual(initial[3], successor[3])
        self.assertEqual(successor[1]["tuple"], self.tuple)

    def test_changed_template_is_rejected_with_unchanged_live_files(self):
        runner = self.root / "runner.original"
        runner.write_bytes((self.live / "bin/cherri-remote-runner.sh").read_bytes() + b"changed")
        with self.assertRaisesRegex(ValueError, "digest differs"):
            baseline.wrapper_templates(
                self.live, self.path, self.sha, runner,
                self.live / "bin/cherri-remote-legacy.sh")



class InstalledImmutableBaseline(unittest.TestCase):
    # Share fixture setup without rerunning the unchanged inherited test cases.
    setUp = SuccessorStageBaseline.setUp
    save = SuccessorStageBaseline.save
    def test_installed_mode_reads_current_wrappers_without_historical_templates(self):
        baseline.ORIGINAL_LIVE_HASHES["bin/cherri-remote-runner.sh"] = "0" * 64
        baseline.ORIGINAL_LIVE_HASHES["bin/cherri-remote-legacy.sh"] = "1" * 64
        hashes, witness, values, records = baseline.wrapper_templates(
            self.live, self.path, self.sha, None, None, installed=True)
        self.assertEqual(hashes, self.record["candidateHashes"])
        self.assertFalse(witness["acceptanceGranted"])
        for role in ("runner", "legacy"):
            relative = "bin/cherri-remote-" + role + ".sh"
            self.assertEqual(values[role], (self.live / relative).read_text())
            self.assertEqual(records[role], {
                "path": str((self.live / relative).resolve()),
                "sha256": hashes[relative], "kind": "CURRENT_INSTALLED_IMMUTABLE",
            })

    def test_installed_mode_rejects_partial_or_mixed_template_selection(self):
        for receipt, sha, runner, legacy in (
            (None, self.sha, None, None), (self.path, None, None, None),
            (self.path, self.sha, self.live / "runner", None),
            (self.path, self.sha, None, self.live / "legacy"),
            (self.path, self.sha, self.live / "runner", self.live / "legacy"),
        ):
            with self.subTest(receipt=receipt, sha=sha, runner=runner, legacy=legacy):
                with self.assertRaisesRegex(ValueError, "no historical wrapper"):
                    baseline.wrapper_templates(
                        self.live, receipt, sha, runner, legacy, installed=True)


class InstalledImmutableRenderer(unittest.TestCase):
    def setUp(self):
        from stage_remote_release import immutable_release_reader
        from remote_immutable_successor import render_immutable_successor
        self.reader = immutable_release_reader
        self.renderer = render_immutable_successor
        directory = Path(__file__).parent / "fixtures" / "remote-immutable-k18-20261125"
        self.fixture = json.loads((directory / "current-capture-fixture.json").read_text())
        self.templates = {role: (directory / (role + ".sh")).read_text()
                          for role in ("runner", "legacy")}
        self.protected = self.fixture["protectedHashes"]
        self.previous = self.fixture["tuple"]
        self.tools = self.fixture["toolHashes"]
        self.next = {
            key: ("a" * len(value) if key not in ("label", "sourceBranch") else value)
            for key, value in self.previous.items()
        }
        self.next.update(label="p15-source-bound-prototype",
                         sourceBranch="agent/cherri-prototype-only")

    def render(self, templates=None, protected=None, previous=None, next_tuple=None,
               tools=None):
        return self.renderer(
            self.templates if templates is None else templates,
            self.protected if protected is None else protected,
            self.previous if previous is None else previous,
            self.next if next_tuple is None else next_tuple,
            self.tools if tools is None else tools, self.reader)

    def test_new_capture_has_exact_current_wrapper_hashes(self):
        self.assertEqual(self.fixture["kind"], "DERIVED_FROM_CURRENT_INSTALLED_CAPTURE")
        self.assertFalse(self.fixture["historicalAcceptance"])
        for role, text in self.templates.items():
            self.assertEqual(hashlib.sha256(text.encode()).hexdigest(),
                             self.protected["bin/cherri-remote-" + role + ".sh"])

    def test_same_tuple_preserves_both_actual_wrappers_byte_for_byte(self):
        self.assertEqual(self.render(next_tuple=self.previous), self.templates)

    def test_new_tuple_changes_only_one_reader_line_and_six_legacy_bindings(self):
        import ast
        actual = self.render()
        before = self.templates["runner"].splitlines(True)
        after = actual["runner"].splitlines(True)
        self.assertEqual(len(before), len(after))
        differences = [i for i, (a, b) in enumerate(zip(before, after)) if a != b]
        self.assertEqual(len(differences), 1)
        self.assertTrue(before[differences[0]].startswith("expected="))
        self.assertEqual(ast.literal_eval(after[differences[0]][len("expected="):]), self.next)
        before = self.templates["legacy"].splitlines(True)
        after = actual["legacy"].splitlines(True)
        self.assertEqual(len(before), len(after))
        differences = [i for i, (a, b) in enumerate(zip(before, after)) if a != b]
        self.assertEqual(len(differences), 6)
        prefixes = ("COMPILER=", "EXPECTED_RUNNER=", "if x.get('label')!=",
                    "   printf 'CHERRI_REMOTE_SOURCE_BRANCH=", "out=", "printf 'CHERRI_BRANCH=")
        self.assertEqual({next(p for p in prefixes if before[i].startswith(p))
                          for i in differences}, set(prefixes))
        self.assertIn('COMPILER="$BASE/' + self.next["label"] + '/runtime/bin/cherri"\n', after)
        self.assertIn('EXPECTED_RUNNER="' +
                      hashlib.sha256(actual["runner"].encode()).hexdigest() + '"\n', after)
        self.assertIn("if x.get('label')!='" + self.next["label"] +
                      "' or x.get('sourceBranch')!='" + self.next["sourceBranch"] + "':\n", after)
        for role in actual:
            self.assertIn(' sign "$preflight"', actual[role])
            self.assertIn('--source-file="$', actual[role])

    def test_actual_rendered_shell_syntax(self):
        import subprocess
        for role, text in self.render().items():
            with self.subTest(role=role):
                result = subprocess.run(["bash", "-n", "-s"], input=text, text=True,
                                        capture_output=True, timeout=5)
                self.assertEqual(result.returncode, 0, result.stderr)

    def test_changed_wrapper_bytes_or_pins_are_rejected(self):
        for role in self.templates:
            with self.subTest(role=role, fault="bytes"):
                changed = dict(self.templates)
                changed[role] += "\n# unexpected content\n"
                with self.assertRaisesRegex(ValueError, "digest differs"):
                    self.render(templates=changed)
            with self.subTest(role=role, fault="pin"):
                changed = dict(self.protected)
                changed["bin/cherri-remote-" + role + ".sh"] = "0" * 64
                with self.assertRaisesRegex(ValueError, "digest differs"):
                    self.render(protected=changed)

    def test_every_prior_tuple_field_remains_bound_to_the_actual_reader(self):
        for key, value in self.previous.items():
            changed = dict(self.previous)
            changed[key] = ("p15-other" if key == "label" else
                            "agent/other" if key == "sourceBranch" else "b" * len(value))
            with self.subTest(field=key):
                with self.assertRaisesRegex(ValueError, "reader"):
                    self.render(previous=changed)

    def test_missing_or_duplicate_reader_is_rejected_after_hash_binding(self):
        reader = self.reader(self.previous, self.tools)
        for fault in ("missing", "duplicate"):
            changed = dict(self.templates)
            changed["runner"] = (changed["runner"].replace(reader, "") if fault == "missing"
                                 else changed["runner"] + "\n" + reader + "\n")
            pins = dict(self.protected)
            pins["bin/cherri-remote-runner.sh"] = hashlib.sha256(changed["runner"].encode()).hexdigest()
            with self.subTest(fault=fault):
                with self.assertRaisesRegex(ValueError, "reader"):
                    self.render(templates=changed, protected=pins)

    def test_each_legacy_binding_must_be_one_complete_line(self):
        prefixes = ("COMPILER=", "EXPECTED_RUNNER=", "if x.get('label')!=",
                    "   printf 'CHERRI_REMOTE_SOURCE_BRANCH=", "out=", "printf 'CHERRI_BRANCH=")
        for prefix in prefixes:
            original = next(line for line in self.templates["legacy"].splitlines(True)
                            if line.startswith(prefix))
            for fault in ("missing", "duplicate"):
                changed = dict(self.templates)
                changed["legacy"] = changed["legacy"].replace(
                    original, "" if fault == "missing" else original + original, 1)
                pins = dict(self.protected)
                pins["bin/cherri-remote-legacy.sh"] = hashlib.sha256(changed["legacy"].encode()).hexdigest()
                with self.subTest(binding=prefix, fault=fault):
                    with self.assertRaisesRegex(ValueError, "Legacy binding"):
                        self.render(templates=changed, protected=pins)

    def test_comments_quoting_old_bindings_are_preserved_and_never_replaced(self):
        actual = self.render()
        pairs = [(old, new) for old, new in zip(
            self.templates["legacy"].splitlines(True), actual["legacy"].splitlines(True))
                 if old != new]
        self.assertEqual(len(pairs), 6)
        for old, new in pairs:
            comment = "# unchanged quoted source: " + old
            changed = dict(self.templates)
            changed["legacy"] = changed["legacy"].replace(old, comment + old, 1)
            pins = dict(self.protected)
            pins["bin/cherri-remote-legacy.sh"] = hashlib.sha256(changed["legacy"].encode()).hexdigest()
            with self.subTest(binding=old):
                rendered = self.render(templates=changed, protected=pins)["legacy"]
                self.assertIn(comment, rendered)
                self.assertIn(new, rendered.splitlines(True))

    def test_invalid_next_tuple_cannot_inject_shell_or_python(self):
        for key, value in (("label", "p15-bad;command"),
                           ("sourceBranch", "agent/bad\naction"),
                           ("compilerCommit", "x" * 40),
                           ("compilerSHA256", "f" * 63),
                           ("skillSourceHead", 1)):
            changed = dict(self.next)
            changed[key] = value
            with self.subTest(field=key):
                with self.assertRaises(ValueError):
                    self.render(next_tuple=changed)

    def test_toolchain_inventory_and_hashes_remain_exact(self):
        missing = dict(self.tools)
        missing.pop(next(iter(missing)))
        extra = dict(self.tools, unexpected="a" * 64)
        changed = dict(self.tools)
        changed["tools/go/VERSION"] = "a" * 64
        for name, tools in (("missing", missing), ("extra", extra), ("changed", changed)):
            with self.subTest(fault=name):
                with self.assertRaisesRegex(ValueError, "toolchain binding"):
                    self.render(tools=tools)

if __name__ == "__main__":
    unittest.main()
