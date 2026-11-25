"""Actual Git and publication controls for one fixed synthetic engineering probe.

The retained unsigned graph is real; signatures/callbacks below are authored
controls and never Apple execution evidence. No Go/native build is launched.
"""
import copy
import json
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import unittest

import tagged_carrier_public_originals as retention
import tagged_carrier_runtime as runtime
import prepare_public_artifacts as publication
import qa_terminal_receipts as terminal
import test_tagged_carrier_runtime as controls

ROOT = controls.ROOT
QA = controls.QA
FIXTURE = controls.FIXTURE
RUN = controls.RUN
ATTEMPT = "1"


def git(repo, *args):
    process = subprocess.run(["git", "-c", "user.name=Cherri synthetic control",
                              "-c", "user.email=synthetic@example.invalid", *args],
                             cwd=repo, capture_output=True, timeout=15)
    if process.returncode:
        raise AssertionError(process.stderr.decode("utf-8", errors="replace"))
    return process.stdout.decode("utf-8").strip()


class TaggedPublicOriginalControls(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.repo_temp = tempfile.TemporaryDirectory(prefix="tagged-retention-git-")
        cls.addClassCleanup(cls.repo_temp.cleanup)
        cls.repo = Path(cls.repo_temp.name).resolve()
        for name in ("tools/shortcuts-simulator-qa/fixtures/" + FIXTURE + ".cherri",
                     "tools/shortcuts-simulator-qa/fixtures/" + FIXTURE + ".expected.json",
                     runtime.ENROLLMENT):
            target = cls.repo / name
            target.parent.mkdir(parents=True, exist_ok=True)
            target.write_bytes((ROOT / name).read_bytes())
        git(cls.repo, "init", "-q")
        git(cls.repo, "add", ".")
        git(cls.repo, "commit", "-qm", "Pin synthetic publication control inputs")
        cls.commit = git(cls.repo, "rev-parse", "HEAD")

    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory(prefix="tagged-retention-synthetic-")
        self.addCleanup(self.temporary.cleanup)
        self.parent = Path(self.temporary.name).resolve()
        self.sample = controls.SyntheticTaggedRun(self.parent / RUN)
        self.sample.manifest["compilerCommitSha"] = self.commit
        self.sample.signing["compilerCommitSha"] = self.commit
        self.sample.signing["targetProvenance"]["compilerCommitSha"] = self.commit
        self.sample.write("compiler-manifest.json", self.sample.manifest)
        self.sample.write("runtime/signing-provenance.json", self.sample.signing)
        self.sample.write("summary.json", {"privacyGatePassed": True, "runId": RUN,
                                          "compilerCommitSha": self.commit})
        self.sequence = 0

    def destination(self, name):
        self.sequence += 1
        return self.parent / (name + "-" + str(self.sequence))

    def publish(self, diagnostic=True):
        if diagnostic:
            oracle = runtime.evaluate(ROOT, self.sample.root, FIXTURE, self.commit, controls.NONCE, "")
            if oracle["runtimeObservationVerified"]:
                (self.sample.root / "runtime/runtime-trace.jsonl").write_bytes(runtime.trace_bytes(oracle["traceRecords"]))
            assertions = {"targetFixtureId": FIXTURE, "expectedRunId": controls.NONCE,
                          "shortcutRunCompletedGate": oracle["runGate"],
                          "traceCapturedGate": oracle["traceGate"]}
            runtime.apply_assertions(assertions, oracle, ROOT, self.sample.root, self.commit, "")
            self.sample.write("runtime/output-assertions.json", assertions)
        public = self.destination("public")
        publication.stage(self.sample.root, public, publication.trusted_fixture_sources(QA / "fixtures"))
        return public

    def stage(self, public, **changes):
        args = {"repo": self.repo, "root": self.sample.root, "public_root": public,
                "output": self.destination("bundle"), "fixture": FIXTURE, "commit": self.commit,
                "run_id": RUN, "attempt": ATTEMPT}
        args.update(changes)
        value = retention.stage(**args)
        return args["output"], value

    def collect(self, bundle, public, **changes):
        args = {"repo": self.repo, "bundle": bundle, "public_root": public,
                "output": self.destination("joined"), "fixture": FIXTURE, "commit": self.commit,
                "run_id": RUN, "attempt": ATTEMPT}
        args.update(changes)
        value = retention.collect(**args)
        return args["output"], value

    def clone(self, source, label):
        target = self.destination(label)
        shutil.copytree(source, target)
        return target

    def rebind_public(self, public, name, raw):
        (public / name).write_bytes(raw)
        path = public / "publication-manifest.json"
        manifest = json.loads(path.read_bytes())
        rows = [row for row in manifest["published"] if row["path"] == name]
        self.assertEqual(len(rows), 1)
        rows[0].update(retention.pin(name, raw))
        path.write_bytes(runtime.canonical(manifest))

    def test_actual_stage_collect_preserves_three_originals_and_text_quarantine(self):
        public = self.publish()
        names = retention.original_paths(FIXTURE)
        for name in names:
            self.assertFalse((public / name).exists())
        manifest = json.loads((public / "publication-manifest.json").read_bytes())
        self.assertTrue(set(names).issubset({entry["path"] for entry in manifest["quarantined"]}))
        bundle, receipt = self.stage(public)
        files = {p.relative_to(bundle).as_posix() for p in bundle.rglob("*") if p.is_file()}
        self.assertEqual(files, set(names) | {retention.RECEIPT})
        joined, result = self.collect(bundle, public)
        for name in names:
            self.assertEqual((joined / name).read_bytes(), (self.sample.root / name).read_bytes())
            self.assertEqual((bundle / name).read_bytes(), (self.sample.root / name).read_bytes())
        self.assertEqual(result["originalsJoined"], 3)
        self.assertTrue(result["runtimeObservationVerified"])
        self.assertTrue(result["nativeCarrierGoalsMatched"])
        self.assertEqual(result["diagnosticStatus"], "VALIDATED_ORIGINAL")
        for value in (receipt, result):
            self.assertIs(value["formalAcceptanceGranted"], False)
            self.assertIs(value["appleExecutionAuthenticated"], False)

    def test_contrary_observation_is_byte_preserved_without_semantic_credit(self):
        self.sample.fields["FIRST_FIRST64"] = controls.b64("CONTRARY")
        self.sample.event["resultSummary"] = self.sample.summary()
        self.sample.write("runtime/callback-events.json", [self.sample.event])
        public = self.publish()
        bundle, _ = self.stage(public)
        joined, result = self.collect(bundle, public)
        self.assertTrue(result["runtimeObservationVerified"])
        self.assertFalse(result["nativeCarrierGoalsMatched"])
        self.assertEqual((joined / runtime.DIAGNOSTIC).read_bytes(), (public / runtime.DIAGNOSTIC).read_bytes())
        self.assertFalse(result["semanticContractVerified"])

    def test_absent_or_failed_diagnostic_does_not_create_native_success(self):
        for diagnostic in (False, True):
            with self.subTest(diagnostic=diagnostic):
                if diagnostic:
                    self.sample.write("runtime/callback-events.json", [{**self.sample.event, "status": "CANCELLED"}])
                public = self.publish(diagnostic)
                bundle, _ = self.stage(public)
                _, result = self.collect(bundle, public)
                self.assertFalse(result["runtimeObservationVerified"])
                self.assertFalse(result["nativeCarrierGoalsMatched"])
                self.assertFalse(result["formalAcceptanceGranted"])

    def test_source_run_attempt_and_old_nested_identity_relabel_are_rejected(self):
        public = self.publish()
        for changes in ({"fixture": "qa-nested-list-candidate-v2"}, {"commit": "b" * 40},
                        {"run_id": "0"}, {"attempt": "0"}, {"attempt": "../1"}):
            with self.subTest(changes=changes), self.assertRaises(ValueError):
                self.stage(public, **changes)

    def test_fixed_source_contract_and_enrollment_drift_are_rejected(self):
        public = self.publish()
        for name in ("tools/shortcuts-simulator-qa/fixtures/" + FIXTURE + ".cherri",
                     "tools/shortcuts-simulator-qa/fixtures/" + FIXTURE + ".expected.json",
                     runtime.ENROLLMENT):
            with self.subTest(path=name):
                root = self.clone(self.repo, "drift")
                p = root / name
                p.write_bytes(p.read_bytes() + b" ")
                with self.assertRaises(ValueError):
                    self.stage(public, repo=root)

    def test_original_source_signature_missing_files_and_symlinks_are_rejected(self):
        public = self.publish()
        for name in retention.original_paths(FIXTURE):
            for mutation in ("changed", "missing", "symlink"):
                with self.subTest(path=name, mutation=mutation):
                    root = self.clone(self.sample.root, "mutated")
                    p = root / name
                    raw = p.read_bytes()
                    if mutation == "changed":
                        p.write_bytes(raw + b"changed")
                    else:
                        p.unlink()
                        if mutation == "symlink":
                            target = self.destination("outside"); target.write_bytes(raw); p.symlink_to(target)
                    with self.assertRaises(ValueError):
                        self.stage(public, root=root)

    def test_secret_magic_and_undeclared_public_files_cannot_be_promoted(self):
        public = self.publish()
        for mutation in ("secret", "raster", "extra"):
            with self.subTest(mutation=mutation):
                changed = self.clone(public, "unsafe-public")
                if mutation == "extra":
                    (changed / "unlisted.txt").write_text("unlisted")
                else:
                    raw = (b'{"value":"CHERRI_PRIVATE_CANARY"}' if mutation == "secret" else b"\x89PNG\r\n\x1a\n")
                    self.rebind_public(changed, "summary.json", raw)
                with self.assertRaises(ValueError):
                    self.stage(changed)

    def test_bundle_receipt_cannot_change_identity_or_grant_credit(self):
        public = self.publish()
        bundle, receipt = self.stage(public)
        for key, value in (("implementationSha", "b" * 40), ("runId", "999"),
                           ("runAttempt", "2"), ("fixtureId", "qa-nested-list-candidate-v2"),
                           ("formalAcceptanceGranted", True), ("appleExecutionAuthenticated", True),
                           ("enrollmentSha256", "0" * 64)):
            with self.subTest(field=key):
                changed = self.clone(bundle, "wrong-receipt")
                (changed / retention.RECEIPT).write_bytes(runtime.canonical({**receipt, key: value}))
                with self.assertRaises(ValueError):
                    self.collect(changed, public)

    def test_bundle_inventory_and_duplicate_json_are_rejected(self):
        public = self.publish()
        bundle, _ = self.stage(public)
        for mutation in ("extra", "duplicate", "changed"):
            with self.subTest(mutation=mutation):
                changed = self.clone(bundle, "wrong-bundle")
                if mutation == "extra":
                    (changed / "not-reviewed.txt").write_text("extra")
                elif mutation == "duplicate":
                    (changed / retention.RECEIPT).write_bytes(b'{"runId":"1","runId":"2"}')
                else:
                    p = changed / retention.original_paths(FIXTURE)[0]; p.write_bytes(p.read_bytes() + b" ")
                with self.assertRaises(ValueError):
                    self.collect(changed, public)

    def test_public_diagnostic_and_run_nonce_are_recomputed(self):
        public = self.publish()
        bundle, _ = self.stage(public)
        for mutation in ("goals", "trace", "nonce"):
            with self.subTest(mutation=mutation):
                changed = self.clone(public, "wrong-observation")
                if mutation == "goals":
                    name = runtime.DIAGNOSTIC
                    document = json.loads((changed / name).read_bytes())
                    document["observation"]["actualFields"]["FIRST_FIRST64"] = controls.b64("REWRITTEN")
                    self.rebind_public(changed, name, runtime.canonical(document))
                elif mutation == "trace":
                    self.rebind_public(changed, "runtime/runtime-trace.jsonl", b'{"fabricated":true}\n')
                else:
                    name = "runtime/output-assertions.json"
                    assertions = json.loads((changed / name).read_bytes())
                    assertions["expectedRunId"] = "stale"
                    self.rebind_public(changed, name, runtime.canonical(assertions))
                with self.assertRaises(ValueError):
                    self.collect(bundle, changed)

    def test_existing_or_nested_outputs_are_never_overwritten(self):
        public = self.publish()
        output = self.destination("existing"); output.mkdir(); (output / "owned").write_text("preserve")
        with self.assertRaises(ValueError):
            self.stage(public, output=output)
        self.assertEqual((output / "owned").read_text(), "preserve")
        with self.assertRaises(ValueError):
            self.stage(public, output=self.sample.root / "nested-output")
        self.assertFalse((self.sample.root / "nested-output").exists())

    def test_actual_checkout_guard_binds_added_runtime_and_publication_sources(self):
        temporary = self.destination("source-guard"); temporary.mkdir()
        for name in terminal.SOURCE_FILES:
            target = temporary / name; target.parent.mkdir(parents=True, exist_ok=True)
            target.write_bytes((ROOT / name).read_bytes())
        git(temporary, "init", "-q")
        git(temporary, "add", ".")
        git(temporary, "commit", "-qm", "Pin synthetic native owner inventory")
        commit = git(temporary, "rev-parse", "HEAD")
        program = ("import sys;sys.path.insert(0,sys.argv[1]+'/tools/shortcuts-simulator-qa');"
                   "import qa_terminal_receipts as q;q.verify_checkout(sys.argv[1],sys.argv[2])")
        def verify():
            return subprocess.run([sys.executable, "-B", "-c", program, str(temporary), commit],
                                  capture_output=True, timeout=15)
        self.assertEqual(verify().returncode, 0)
        for name in ("tagged_carrier_runtime.py", "tagged_carrier_graph.py",
                     "tagged_carrier_observation.py", "tagged_carrier_public_originals.py",
                     "native_comparison_graph.py"):
            with self.subTest(tampered=name):
                target = temporary / "tools/shortcuts-simulator-qa" / name
                raw = target.read_bytes()
                target.write_bytes(raw + b"\n# synthetic tamper\n")
                p = verify()
                self.assertNotEqual(p.returncode, 0)
                self.assertIn(b"ACQUISITION_SOURCE_BYTES", p.stderr)
                target.write_bytes(raw)


if __name__ == "__main__":
    unittest.main()
