"""Transport controls for exactly two public fixtures; no Apple evidence.

Reuse the existing actual-compiler fixture builder and canonical diagnostic
oracle. The AEA1 bytes and callback events remain explicitly synthetic and
cannot establish a real signature, native runtime or formal acceptance.
"""
import copy
import json
from pathlib import Path
import plistlib
import shutil
import unittest
from unittest import mock

import nested_list_probe as probe
import nested_list_public_originals as retention
import nested_list_runtime as runtime
import prepare_public_artifacts as publication
import test_nested_list_runtime as runtime_controls

ROOT = Path(__file__).resolve().parents[2]
RUN = "1234"
ATTEMPT = "1"


class NestedPublicOriginalControls(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        # One compiler build for this family. Do not inherit/repeat the existing
        # runtime test methods; reuse only their original fixture preparation.
        runtime_controls.NestedRuntimeControls.setUpClass.__func__(cls)
        cls.commit = retention.git_bytes(ROOT, "rev-parse", "HEAD").decode().strip()

    write = runtime_controls.NestedRuntimeControls.write

    def setUp(self):
        self.sequence = 0
        runtime_controls.NestedRuntimeControls.setUp(self)

    def initialize(self, fixture):
        runtime_controls.NestedRuntimeControls.initialize(self, fixture)
        self.manifest["compilerCommitSha"] = self.commit
        self.signing["compilerCommitSha"] = self.commit
        self.signing["targetProvenance"]["compilerCommitSha"] = self.commit
        self.nonce = fixture + "-" + RUN + "-synthetic-control"
        self.event["runId"] = self.nonce
        self.write("compiler-manifest.json", self.manifest)
        self.write("runtime/signing-provenance.json", self.signing)
        self.write("runtime/callback-events.json", [self.event])
        self.write("summary.json", {"privacyGatePassed": True, "runId": RUN,
                                    "compilerCommitSha": self.commit})
        for name in ("runtime/nested-list-observation.json", "runtime/output-assertions.json"):
            (self.root / name).unlink(missing_ok=True)

    def destination(self, label):
        self.sequence += 1
        return self.root.parent / (label + "-" + str(self.sequence))

    def publish(self, diagnostic=True):
        if diagnostic:
            oracle = runtime.evaluate(ROOT, self.root, self.fixture, self.commit, self.nonce, "")
            if oracle["runtimeObservationVerified"]:
                (self.root / "runtime/runtime-trace.jsonl").write_bytes(
                    runtime.trace_bytes(oracle["traceRecords"]))
            assertions = {"targetFixtureId": self.fixture, "expectedRunId": self.nonce,
                          "shortcutRunCompletedGate": oracle["runGate"],
                          "traceCapturedGate": oracle["traceGate"]}
            runtime.apply_assertions(assertions, oracle, ROOT, self.root, self.commit, "")
            self.write("runtime/output-assertions.json", assertions)
        public = self.destination("public")
        publication.stage(self.root, public, publication.trusted_fixture_sources(
            ROOT / "tools/shortcuts-simulator-qa/fixtures"))
        return public

    def stage(self, public, root=None, **changes):
        arguments = {"repo": ROOT, "root": root or self.root, "public_root": public,
                     "output": self.destination("bundle"), "fixture": self.fixture,
                     "commit": self.commit, "run_id": RUN, "attempt": ATTEMPT}
        arguments.update(changes)
        result = retention.stage(**arguments)
        return arguments["output"], result

    def collect(self, bundle, public, **changes):
        arguments = {"repo": ROOT, "bundle": bundle, "public_root": public,
                     "output": self.destination("collected"), "fixture": self.fixture,
                     "commit": self.commit, "run_id": RUN, "attempt": ATTEMPT}
        arguments.update(changes)
        result = retention.collect(**arguments)
        return arguments["output"], result

    def copy(self, source, label):
        destination = self.destination(label)
        shutil.copytree(source, destination)
        return destination

    def rebind_public(self, root, name, raw):
        # Deliberately reconcile the transport hash to prove that a matching
        # self-authored manifest cannot override privacy/provenance validators.
        (root / name).write_bytes(raw)
        path = root / "publication-manifest.json"
        manifest = json.loads(path.read_bytes())
        for row in manifest["published"]:
            if row["path"] == name:
                row.update(retention.pin(name, raw))
                break
        else:
            raise AssertionError("control must change an existing published file")
        path.write_bytes(runtime.canonical(manifest))

    def test_roundtrip_both_fixtures_preserves_all_original_bytes(self):
        for fixture in probe.FIXTURES:
            with self.subTest(fixture=fixture):
                self.initialize(fixture)
                public = self.publish()
                published = {p.relative_to(public).as_posix(): p.read_bytes()
                             for p in public.rglob("*") if p.is_file()}
                names = retention.original_paths(fixture)
                for name in names:
                    self.assertFalse((public / name).exists())
                manifest = json.loads(published["publication-manifest.json"])
                self.assertTrue(set(names).issubset({row["path"] for row in manifest["quarantined"]}))
                bundle, receipt = self.stage(public)
                files = {p.relative_to(bundle).as_posix() for p in bundle.rglob("*") if p.is_file()}
                self.assertEqual(files, set(names) | {retention.RECEIPT})
                self.assertEqual(receipt["artifactName"], "cherri-nested-list-public-originals-1234-1")
                self.assertFalse(receipt["runtimeAcceptanceGranted"])
                collected, result = self.collect(bundle, public)
                for name, raw in published.items():
                    self.assertEqual((collected / name).read_bytes(), raw)
                for name in names:
                    original = (self.root / name).read_bytes()
                    self.assertEqual((bundle / name).read_bytes(), original)
                    self.assertEqual((collected / name).read_bytes(), original)
                self.assertEqual(result["diagnosticStatus"], "VALIDATED_ORIGINAL")
                self.assertTrue(result["runtimeObservationVerified"])  # synthetic control only
                self.assertEqual(result["originalsJoined"], 3)
                for key in ("semanticContractVerified", "nativeListTypeProved", "formalAcceptanceGranted"):
                    self.assertIs(result[key], False)

    def test_contrary_values_remain_contrary_without_acceptance(self):
        self.fields["FLAT_COUNT"] = "999"
        self.event["resultSummary"] = runtime_controls.observer_controls.summary(self.fixture, self.fields)
        self.write("runtime/callback-events.json", [self.event])
        public = self.publish()
        bundle, _ = self.stage(public)
        collected, result = self.collect(bundle, public)
        original = (public / "runtime/nested-list-observation.json").read_bytes()
        self.assertEqual((collected / "runtime/nested-list-observation.json").read_bytes(), original)
        diagnostic = json.loads(original)
        self.assertEqual(diagnostic["observation"]["actualFields"]["FLAT_COUNT"], "999")
        self.assertFalse(diagnostic["observation"]["goalChecks"]["FLAT_COUNT"])
        self.assertFalse(result["formalAcceptanceGranted"])

    def test_absent_and_failed_diagnostics_cannot_create_runtime_success(self):
        for diagnostic in (False, True):
            with self.subTest(diagnostic_present=diagnostic):
                self.initialize(probe.CONTROL)
                (self.root / "runtime/callback-events.json").unlink()
                public = self.publish(diagnostic=diagnostic)
                bundle, _ = self.stage(public)
                collected, result = self.collect(bundle, public)
                self.assertFalse(result["runtimeObservationVerified"])
                self.assertFalse(result["formalAcceptanceGranted"])
                self.assertEqual(result["diagnosticStatus"],
                                 "VALIDATED_ORIGINAL" if diagnostic else "NOT_PRESENT")
                self.assertFalse((collected / "runtime/runtime-trace.jsonl").exists())

    def test_unreviewed_fixture_source_run_and_attempt_are_rejected(self):
        public = self.publish()
        self.stage(public)
        for changes in ({"fixture": "qa-other-v2"}, {"commit": "b" * 40},
                        {"run_id": "0"}, {"attempt": "0"}, {"run_id": "9999"},
                        {"attempt": "not-numeric"}):
            with self.subTest(changes=changes), self.assertRaises((ValueError, OSError)):
                self.stage(public, **changes)

    def test_fixed_source_and_contract_drift_are_rejected(self):
        # Actual fixture bytes change; only local Git metadata is a synthetic
        # stub. The real fixed source/contract hash guard remains unchanged.
        checkout = self.destination("fixed-context")
        originals = {}
        for suffix in (".cherri", ".expected.json"):
            name = "tools/shortcuts-simulator-qa/fixtures/" + self.fixture + suffix
            raw = (ROOT / name).read_bytes()
            destination = checkout / name
            destination.parent.mkdir(parents=True, exist_ok=True)
            destination.write_bytes(raw)
            originals[name] = raw
        def metadata(repo, *args):
            self.assertEqual(repo, checkout)
            if args == ("rev-parse", "HEAD"):
                return (self.commit + "\n").encode()
            self.assertEqual(args[0], "show")
            return originals[args[1].split(":", 1)[1]]
        with mock.patch.object(retention, "git_bytes", side_effect=metadata):
            retention.verify_context(checkout, self.fixture, self.commit, RUN, ATTEMPT)
            for name, raw in originals.items():
                (checkout / name).write_bytes(raw + b"changed")
                with self.subTest(name=name), self.assertRaisesRegex(ValueError, "FIXED_SOURCE_BYTES"):
                    retention.verify_context(checkout, self.fixture, self.commit, RUN, ATTEMPT)
                (checkout / name).write_bytes(raw)

    def test_original_source_signature_and_missing_file_are_rejected(self):
        public = self.publish()
        self.stage(public)
        names = retention.original_paths(self.fixture)
        for index, change in ((0, "append"), (2, "append"), (2, "header"), (1, "missing"), (0, "compiler")):
            changed = self.copy(self.root, "raw")
            path = changed / names[index]
            if change == "compiler":
                path = changed / "compiler-manifest.json"
                path.write_bytes(path.read_bytes() + b" ")
            elif change == "missing":
                path.unlink()
            elif change == "header":
                path.write_bytes(b"BAD!" + path.read_bytes()[4:])
            else:
                path.write_bytes(path.read_bytes() + b"changed")
            with self.subTest(index=index, change=change), self.assertRaises((ValueError, OSError)):
                self.stage(public, root=changed)

    def test_same_hash_file_and_directory_symlinks_are_rejected(self):
        public = self.publish()
        self.stage(public)
        for directory in (False, True):
            changed = self.copy(self.root, "linked-raw")
            name = "builds" if directory else retention.original_paths(self.fixture)[0]
            path = changed / name
            held = changed / ("held-directory" if directory else "held-file")
            path.rename(held)
            path.symlink_to(held, target_is_directory=directory)
            with self.subTest(directory=directory), self.assertRaises(ValueError):
                self.stage(public, root=changed)

    def test_original_size_limits_fail_before_publication(self):
        public = self.publish()
        self.stage(public)
        limits = (retention.MAX_SOURCE, runtime.MAX_PLIST, 10 * 1024 * 1024)
        for name, limit in zip(retention.original_paths(self.fixture), limits):
            changed = self.copy(self.root, "oversize-raw")
            with (changed / name).open("r+b") as stream:
                stream.truncate(limit + 1)
            with self.subTest(name=name), self.assertRaises(ValueError):
                self.stage(public, root=changed)

    def test_reconciled_unsigned_hashes_do_not_override_existing_graph_oracle(self):
        public = self.publish()
        self.stage(public)
        changed = plistlib.loads(self.compiled[self.fixture][1])
        changed["WFWorkflowActions"][-1]["WFWorkflowActionParameters"]["WFOutput"] = "SUCCESS"
        unsigned = plistlib.dumps(changed)
        (self.root / retention.original_paths(self.fixture)[1]).write_bytes(unsigned)
        manifest = copy.deepcopy(self.manifest)
        manifest["fixtures"][0]["unsignedPlistSha256"] = probe.sha(unsigned)
        manifest["fixtures"][0]["nestedListProbe"]["unsignedPlistSha256"] = probe.sha(unsigned)
        self.write("compiler-manifest.json", manifest)
        signing = copy.deepcopy(self.signing)
        signing["targetProvenance"].update(unsignedPlistSha256=probe.sha(unsigned),
                                            unsignedByteSize=len(unsigned))
        self.write("runtime/signing-provenance.json", signing)
        with self.assertRaisesRegex(ValueError, "ACTION_55_CHANGED"):
            self.stage(public)

    def test_public_gate_inventory_magic_and_secret_rejections(self):
        public = self.publish()
        self.stage(public)
        for mutation in ("run", "signing", "extra", "magic", "escaped-secret", "gate", "pin"):
            changed = self.copy(public, "changed-public")
            if mutation == "extra":
                (changed / "private.png").write_bytes(b"\x89PNG\r\n\x1a\nSYNTHETIC")
            elif mutation in ("gate", "pin"):
                path = changed / "publication-manifest.json"
                value = json.loads(path.read_bytes())
                if mutation == "gate":
                    value["publicationGatePassed"] = False
                else:
                    value["published"][0]["bytes"] += 1
                path.write_bytes(runtime.canonical(value))
            else:
                name = "runtime/signing-provenance.json" if mutation == "signing" else "summary.json"
                value = json.loads((changed / name).read_bytes())
                if mutation == "run":
                    value["runId"] = "9999"
                elif mutation == "signing":
                    value["targetProvenance"]["sourceSha256"] = "0" * 64
                raw = (b"AEA1SYNTHETIC_NOT_ALLOWED_AS_PUBLIC_TEXT" if mutation == "magic"
                       else br'{"note":"\u0043HERRI_PRIVATE_CANARY"}' if mutation == "escaped-secret"
                       else runtime.canonical(value))
                self.rebind_public(changed, name, raw)
            with self.subTest(mutation=mutation), self.assertRaises(ValueError):
                self.stage(changed)

    def test_bundle_receipt_cannot_reassign_run_source_fixture_or_claim_credit(self):
        public = self.publish()
        bundle, receipt = self.stage(public)
        self.collect(bundle, public)
        mutations = (
            ("runAttempt", "2"), ("runId", "9999"), ("implementationSha", "b" * 40),
            ("fixtureId", probe.CANDIDATE), ("sourceSha256", "0" * 64),
            ("runtimeAcceptanceGranted", True), ("formalAcceptanceGranted", True),
            ("nativeListTypeProved", True), ("unreviewedExtra", "value"),
        )
        for key, value in mutations:
            changed = self.copy(bundle, "forged-bundle")
            forged = copy.deepcopy(receipt)
            forged[key] = value
            (changed / retention.RECEIPT).write_bytes(runtime.canonical(forged))
            with self.subTest(key=key), self.assertRaisesRegex(ValueError, "BUNDLE_RECEIPT_JOIN"):
                self.collect(changed, public)

    def test_bundle_extra_tampered_and_duplicate_json_inputs_are_rejected(self):
        public = self.publish()
        bundle, _ = self.stage(public)
        self.collect(bundle, public)
        for mutation in ("extra", "raw", "duplicate", "symlink", "pin"):
            changed = self.copy(bundle, "tampered-bundle")
            if mutation == "extra":
                (changed / "private.cherri").write_bytes(b"CHERRI_PRIVATE_CANARY")
            elif mutation == "raw":
                path = changed / retention.original_paths(self.fixture)[2]
                path.write_bytes(path.read_bytes() + b"changed")
            elif mutation == "duplicate":
                path = changed / retention.RECEIPT
                path.write_bytes(b'{"runId":"1234","runId":"9999"}')
            elif mutation == "symlink":
                path = changed / retention.original_paths(self.fixture)[0]
                path.unlink()
                path.symlink_to(bundle / retention.original_paths(self.fixture)[0])
            else:
                path = changed / retention.RECEIPT
                value = json.loads(path.read_bytes())
                value["originals"][0]["bytes"] += 1
                path.write_bytes(runtime.canonical(value))
            with self.subTest(mutation=mutation), self.assertRaises((ValueError, OSError)):
                self.collect(changed, public)

    def test_collector_rechecks_diagnostic_and_run_nonce(self):
        public = self.publish()
        bundle, _ = self.stage(public)
        self.collect(bundle, public)
        for mutation in ("claim", "callback"):
            changed = self.copy(public, "forged-diagnostic")
            name = ("runtime/nested-list-observation.json" if mutation == "claim"
                    else "runtime/callback-events.json")
            value = json.loads((changed / name).read_bytes())
            if mutation == "claim":
                value["formalAcceptanceGranted"] = True
            else:
                value[0]["status"] = "ERROR"
            self.rebind_public(changed, name, runtime.canonical(value))
            changed_bundle, _ = self.stage(changed)
            with self.subTest(mutation=mutation), self.assertRaisesRegex(ValueError, "PUBLIC_RECOMPUTATION"):
                self.collect(changed_bundle, changed)
        self.nonce = self.fixture + "-9999-synthetic-control"
        self.event["runId"] = self.nonce
        self.write("runtime/callback-events.json", [self.event])
        wrong_run_public = self.publish()
        wrong_run_bundle, _ = self.stage(wrong_run_public)
        with self.assertRaisesRegex(ValueError, "DIAGNOSTIC_RUN_NONCE"):
            self.collect(wrong_run_bundle, wrong_run_public)

    def test_existing_or_nested_outputs_are_never_overwritten(self):
        public = self.publish()
        bundle, _ = self.stage(public)
        existing = self.destination("existing")
        existing.mkdir()
        marker = existing / "unchanged"
        marker.write_bytes(b"ORIGINAL")
        for output in (existing, self.root / "child", public / "child"):
            with self.subTest(stage=str(output)), self.assertRaises(ValueError):
                self.stage(public, output=output)
        for output in (existing, bundle / "child", public / "child"):
            with self.subTest(collect=str(output)), self.assertRaises(ValueError):
                self.collect(bundle, public, output=output)
        self.assertEqual(marker.read_bytes(), b"ORIGINAL")
        self.assertFalse((bundle / "child").exists())
        self.assertFalse((public / "child").exists())


if __name__ == "__main__":
    unittest.main()
