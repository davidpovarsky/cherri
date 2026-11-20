"""Synthetic protocol controls for the real source-bound auditor, not native results."""
import copy
import json
from pathlib import Path
import sys
import subprocess
import tempfile
import unittest
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(ROOT / "tools/p10r15"))
import audit_native_continuation as audit
import test_runtime_callback_contract as callbacks
import test_storage_cross_run as storage_tests
import storage_cross_run as storage

COMMIT = subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=ROOT, text=True).strip()
RUN_ID = 41  # Deliberately synthetic. Never written into native evidence.
FID = audit.FIXTURES[3]


class NativeContinuationAuditTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        callbacks.CallbackContractTests.setUpClass()
        cls.oracle = callbacks.CallbackContractTests()
        cls.program = audit.oracle_program(audit.source_bytes(COMMIT, audit.SHELL).decode())
        cls.case = next(c for c in json.loads(audit.source_bytes(COMMIT, audit.MANIFEST))["cases"] if c["id"] == FID)

    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        (self.root / "runtime").mkdir()
        self.nonce = FID + "-" + str(RUN_ID) + "-" + "1" * 32
        self.compiler = self.oracle.manifest(FID)
        self.compiler["compilerCommitSha"] = COMMIT
        self.entry = self.compiler["fixtures"][0]
        self.entry.update(fileName=FID + ".cherri", sourceSha256=self.case["sourceSha256"],
                          nativeActionCount=1, actionIdentifiers=["is.workflow.actions.number"])
        # Synthetic signing metadata tests cross-field binding only.
        target = dict(compilerCommitSha=COMMIT, shortcutName=FID,
            sourceSha256=self.case["sourceSha256"], unsignedPlistSha256="c" * 64,
            nativeActionCount=1, signedArtifactSha256="d" * 64, signedByteSize=1234, headerMagic="AEA1")
        self.signing = dict(compilerCommitSha=COMMIT, targetProvenance=target,
                            signedArtifactSha256="d" * 64, signedByteSize=1234)
        self.event = self.oracle.event()
        self.event.update(fixtureId=FID, expectedShortcutName=FID, runId=self.nonce,
                          resultSummary=self.case["expectedSummary"])
        self.event["tracePayload"] = self.event["tracePayload"].replace("qa-unit", FID)
        self.summary = dict(compilerCommitSha=COMMIT, runId=str(RUN_ID), requireRuntime=True,
            targetFixtureId=FID, privacyGatePassed=True, buildProfile="debug",
            traceExpectation="REQUIRED_NDJSON", runtimeContractVerified=True)
        self.assertions = dict(compilerCommitSha=COMMIT, targetFixtureId=FID,
            compiledSourceSha256=self.case["sourceSha256"], unsignedPlistSha256="c" * 64,
            buildProfile="debug", traceExpectation="REQUIRED_NDJSON", releaseBuildVerified=False,
            expectedRunId=self.nonce, expectedSemanticOutput=self.case["expectedSummary"],
            runtimeContractVerified=True)
        self.metadata = dict(id=RUN_ID, head_sha=COMMIT, event="workflow_dispatch",
            path=".github/workflows/cherri-ios27-shortcuts-simulator-qa.yml",
            status="completed", conclusion="success",
            html_url="https://github.com/davidpovarsky/cherri/actions/runs/" + str(RUN_ID))

    def publish_control(self):
        oracle, trace = audit.replay(self.program, [self.event], self.compiler, COMMIT,
                                    FID, self.nonce, self.case["expectedSummary"])
        for path, value in (
            ("summary.json", self.summary), ("compiler-manifest.json", self.compiler),
            ("runtime/output-assertions.json", self.assertions),
            ("runtime/signing-provenance.json", self.signing),
            ("runtime/callback-events.json", [self.event]), ("environment.json", {"syntheticUnitControl": True}),
        ):
            (self.root / path).write_text(json.dumps(value), encoding="utf-8")
        (self.root / "runtime/runtime-trace.jsonl").write_bytes(trace)
        files = [{"path": p.relative_to(self.root).as_posix(), "bytes": p.stat().st_size,
                  "sha256": audit.digest(p.read_bytes())} for p in self.root.rglob("*")
                 if p.is_file() and p.name != "publication-manifest.json"]
        publication = dict(syntheticOnly=True, publicationGatePassed=True,
                           pixelRedactionClaimed=False, rasterPublicationAllowed=False, published=files)
        (self.root / "publication-manifest.json").write_text(json.dumps(publication), encoding="utf-8")
        return publication

    def run_audit(self):
        return audit.audit(self.root, COMMIT, RUN_ID, FID, self.metadata)

    def test_synthetic_protocol_control_replays_actual_oracle_and_eight_causal_rejections(self):
        self.publish_control()
        report = self.run_audit()
        self.assertTrue(report["runtimeContractVerified"])
        self.assertEqual(len(report["causalOracleControls"]["rejected"]), 8)
        self.assertEqual(report["observedNumericFlags"], report["expectedNumericFlags"])

    def test_compiler_rejection_cannot_be_promoted_to_native_execution(self):
        self.compiler["fixtureRejections"] = [{"fixtureId": FID, "status": "BLOCKED_PLATFORM",
                                              "appleRuntime": "NOT_RUN", "nativeArtifactEmitted": False}]
        self.publish_control()
        with self.assertRaisesRegex(ValueError, "compiler-rejected fixture cannot establish native execution"):
            self.run_audit()

    def test_unrelated_preserved_rejections_do_not_promote_or_block_executed_target(self):
        self.compiler["fixtureRejections"] = [{"fixtureId": "qa-f04-yield-list-v2",
                                              "status": "BLOCKED_PLATFORM", "appleRuntime": "NOT_RUN"}]
        self.publish_control()
        report = self.run_audit()
        self.assertEqual(report["fixtureId"], FID)
        self.assertTrue(report["runtimeContractVerified"])

    def test_observed_wrong_number_flags_remain_semantic_failure(self):
        self.event["resultSummary"] = self.case["expectedSummary"].replace("DNE=N", "DNE=Y")
        self.summary["runtimeContractVerified"] = self.assertions["runtimeContractVerified"] = False
        self.publish_control()
        report = self.run_audit()
        self.assertEqual(report["status"], "FAIL_RUNTIME_PRESERVED")
        self.assertEqual(report["observedCallback"]["status"], "SUCCESS")
        self.assertEqual(report["observedNumericFlags"]["DNE"], "Y")
        self.assertEqual(report["causalOracleControls"]["status"], "NOT_RUN_BASELINE_NOT_PASS")
        self.assertEqual(report["causalOracleControls"]["rejected"], [])

    def test_summary_cannot_promote_wrong_output_or_missing_trace(self):
        for change in ({"resultSummary": "CHERRI_QA_WRONG"}, {"tracePayload": ""}, {"status": "ERROR"}):
            with self.subTest(change=change):
                original = copy.deepcopy(self.event)
                self.event.update(change)
                self.publish_control()
                with self.assertRaisesRegex(ValueError, "runtime success disagrees"):
                    self.run_audit()
                self.event = original

    def test_exact_run_source_and_signing_binding_faults_are_rejected(self):
        for obj, key, value in (
            (self.metadata, "head_sha", "f" * 40), (self.metadata, "status", "in_progress"),
            (self.metadata, "path", ".github/workflows/unrelated.yml"),
            (self.assertions, "compiledSourceSha256", "e" * 64),
            (self.signing["targetProvenance"], "unsignedPlistSha256", "e" * 64),
            (self.signing["targetProvenance"], "signedArtifactSha256", "e" * 64),
            (self.assertions, "expectedSemanticOutput", "CHERRI_QA_WRONG"),
            (self.assertions, "expectedRunId", FID + "-40-" + "1" * 32),
        ):
            with self.subTest(key=key):
                original = obj[key]
                obj[key] = value
                self.publish_control()
                with self.assertRaises(ValueError):
                    self.run_audit()
                obj[key] = original

    def test_publication_hash_inventory_and_privacy_faults_are_rejected(self):
        allowed = audit.constant(audit.source_bytes(COMMIT, audit.PUBLISHER), "ALLOWED")
        for mutation in ("hash", "duplicate", "extra_raw", "path_escape", "raster_claim"):
            with self.subTest(mutation=mutation):
                publication = self.publish_control()
                if mutation == "hash":
                    publication["published"][0]["sha256"] = "f" * 64
                elif mutation == "duplicate":
                    publication["published"].append(copy.deepcopy(publication["published"][0]))
                elif mutation == "extra_raw":
                    (self.root / "unlisted.shortcut").write_bytes(b"AEA1")
                elif mutation == "path_escape":
                    publication["published"][0]["path"] = "../outside.json"
                else:
                    publication["rasterPublicationAllowed"] = True
                with self.assertRaises(ValueError):
                    audit.publication_files(self.root, publication, allowed)
                (self.root / "unlisted.shortcut").unlink(missing_ok=True)

    def test_actual_audit_cli_distinguishes_protocol_success_runtime_failure_and_bad_evidence(self):
        with tempfile.TemporaryDirectory(prefix="cherri-audit-cli-") as outside:
            directory = Path(outside)
            for expected_exit in (0, 2, 3):
                with self.subTest(expected_exit=expected_exit):
                    if expected_exit == 2:
                        self.event["resultSummary"] = self.case["expectedSummary"].replace("DNE=N", "DNE=Y")
                        self.summary["runtimeContractVerified"] = self.assertions["runtimeContractVerified"] = False
                    if expected_exit == 3:
                        self.metadata["head_sha"] = "f" * 40
                    self.publish_control()
                    metadata = directory / "metadata.json"
                    metadata.write_text(json.dumps(self.metadata), encoding="utf-8")
                    output = directory / "report.json"
                    completed = subprocess.run([sys.executable, "-X", "utf8",
                        str(ROOT / "tools/p10r15/audit_native_continuation.py"),
                        "--artifact", str(self.root), "--commit", COMMIT, "--run-id", str(RUN_ID),
                        "--fixture", FID, "--run-metadata", str(metadata), "--output", str(output)],
                        capture_output=True, text=True, encoding="utf-8", timeout=20)
                    self.assertEqual(completed.returncode, expected_exit, completed.stderr)
                    result = json.loads(output.read_bytes())
                    self.assertEqual(result["runtimeContractVerified"], expected_exit == 0)
                    if expected_exit == 3:
                        self.assertEqual(result["status"], "FAIL_EVIDENCE_VALIDATION")

    def test_numeric_parser_preserves_all_nine_ordered_flags(self):
        expected = self.case["expectedSummary"]
        self.assertEqual(list(audit.numeric_flags(expected)), list(audit.FIELDS))
        for text in (None, expected + ":DNE=N", expected.replace("DNE=N:", ""),
                     expected.replace("DNE=N", "DNE=X"), expected.replace("DEQ=Y:DNE=N", "DNE=N:DEQ=Y")):
            self.assertIsNone(audit.numeric_flags(text))


    def test_reviewed_selector_contracts_bind_exact_source_and_expected_literals(self):
        expected = {
            "qa-optional-forwarding-diagnostic-v2": "CHERRI_OPTIONAL_FORWARDING_DIAGNOSTIC_V1:NONE=NONE:EMPTY_TEXT=TEXT[]:FALSE=BOOL[false]:ZERO=NUMBER[0]:EMPTY_LIST=LIST[0]:EMPTY_MAP=MAP[0]:LIST=LIST[2|A|B]:MAP=MAP[1|VALUE|Dictionary]:SNAPSHOT=NONE:WRITTEN=TEXT[]:DEFAULT=TEXT[DEFAULT]",
            "qa-collect-empty-split-v2": "CHERRI_QA_COLLECT_EMPTY_SPLIT:direct=0:repeat=0:cleared=0:emptyText=1",
            "qa-collect-counts-v2": "CHERRI_QA_COLLECT_COUNTS:empty=0:single=1:one:many=100:0:99",
            "qa-nil-clear-v2": "CHERRI_QA_NIL_CLEAR:binding=0:assignment=0:control=0:present=1",
            "qa-file-name-binding-v2": "CHERRI_QA_FILE_NAME_BINDING:L=ok:V=ok:I=ok:LT=ok:VT=ok:IT=ok",
            "qa-number-coercion-control-v2": "CHERRI_QA_NUMBER_COERCION_RESULT:DEQ=Y:DNE=N:REQ=Y:RNE=N:BEQ=Y:BNE=N:ONE_NE_ZERO=Y:ZERO_NE_ONE=Y:ONE_EQ_ONE=Y",
        }
        for fid, output in expected.items():
            with self.subTest(fid=fid):
                case = audit.reviewed_case(COMMIT, fid)
                self.assertEqual(case["expectedSummary"], output)
                self.assertEqual(case["sourceSha256"], audit.digest(audit.source_bytes(COMMIT, case["path"])))
        with self.assertRaisesRegex(ValueError, "unreviewed"):
            audit.reviewed_case(COMMIT, "private-unreviewed-fixture")

    def test_reviewed_export_paths_bind_nested_source_and_reject_substitution(self):
        for name in ("unicode", "empty", "thousand", "redacted", "unsupported-file", "text-file"):
            fid = "qa-export-" + name + "-v2"
            with self.subTest(fid=fid):
                case = audit.reviewed_case(COMMIT, fid)
                path = "tools/shortcuts-simulator-qa/fixtures/p15-export-probes/" + fid + ".cherri"
                self.assertEqual(case["path"], path)
                self.assertEqual(case["sourceSha256"], audit.digest(audit.source_bytes(COMMIT, path)))
                expected = "CHERRI_QA_EXPORT_PROBE_RESULT:P15_EXPORT_" + name.replace("-", "_").upper() + "_OK"
                self.assertEqual(case["expectedSummary"], expected)
        fid = "qa-export-unicode-v2"
        original = audit.source_bytes
        shell = original(COMMIT, audit.SHELL).decode("utf-8")
        changes = {
            "wrong_selector_path": shell.replace('TARGET_SOURCE_RELATIVE="p15-export-probes/' + fid + '.cherri"',
                                                   'TARGET_SOURCE_RELATIVE="' + fid + '.cherri"'),
            "wrong_compiler_path": shell.replace('("' + fid + '", "p15-export-probes/' + fid + '.cherri", "v2")',
                                                   '("' + fid + '", "' + fid + '.cherri", "v2")'),
        }
        for name, changed in changes.items():
            with self.subTest(fault=name):
                self.assertNotEqual(changed, shell)
                def read(commit, path):
                    return changed.encode("utf-8") if path == audit.SHELL else original(commit, path)
                with patch.object(audit, "source_bytes", side_effect=read):
                    with self.assertRaises(ValueError):
                        audit.reviewed_case(COMMIT, fid)

    def test_source_selector_identity_and_literal_faults_are_rejected(self):
        fid = "qa-number-coercion-control-v2"
        original = audit.source_bytes
        shell = original(COMMIT, audit.SHELL).decode("utf-8")
        label = "  qa-number-coercion-control)\n"
        block = shell[shell.index(label):shell.index("    ;;", shell.index(label)) + len("    ;;")]
        changes = {
            "duplicate_selector": shell.replace(block, block + "\n" + block),
            "wrong_fixture": shell.replace('TARGET_FIXTURE_ID="' + fid + '"', 'TARGET_FIXTURE_ID="qa-other"'),
            "wrong_shortcut": shell.replace('EXPECTED_SHORTCUT_NAME="' + fid + '"', 'EXPECTED_SHORTCUT_NAME="qa-other"'),
            "dynamic_expected": shell.replace("CHERRI_QA_NUMBER_COERCION_RESULT:DEQ=Y", "CHERRI_QA_NUMBER_COERCION_RESULT:$UNTRUSTED"),
            "wrong_compile_file": shell.replace('"' + fid + '.cherri"', '"qa-other.cherri"'),
        }
        for name, changed in changes.items():
            with self.subTest(name=name):
                def read(commit, path):
                    return changed.encode("utf-8") if path == audit.SHELL else original(commit, path)
                with patch.object(audit, "source_bytes", side_effect=read):
                    with self.assertRaises(ValueError):
                        audit.reviewed_case(COMMIT, fid)

    def test_new_numeric_selector_uses_unchanged_strict_oracle(self):
        # Pure synthetic protocol metadata, deliberately not an Apple receipt.
        fid = "qa-number-coercion-control-v2"
        with patch(__name__ + ".FID", fid):
            self.case = audit.reviewed_case(COMMIT, fid)
            self.setUp()
            self.publish_control()
            report = self.run_audit()
            self.assertTrue(report["runtimeContractVerified"])
            self.assertEqual(report["expectedNumericFlags"]["RNE"], "N")
            self.event["resultSummary"] = self.case["expectedSummary"].replace("RNE=N", "RNE=Y")
            self.summary["runtimeContractVerified"] = self.assertions["runtimeContractVerified"] = False
            self.publish_control()
            report = self.run_audit()
            self.assertEqual(report["status"], "FAIL_RUNTIME_PRESERVED")
            self.assertEqual(report["observedNumericFlags"]["RNE"], "Y")


    def test_http_type_requires_exactly_one_executed_literal_branch(self):
        def records(labels):
            return b"\n".join(json.dumps({"kind": "INLINE_DEBUG_ACTION", "label": label}).encode() for label in labels)
        self.assertEqual(audit.http_type(records(["http-json-raw-type-text"])), "Text")
        self.assertEqual(audit.http_type(records(["http-json-raw-type-dictionary"])), "Dictionary")
        for labels in ([], ["http-json-raw-type-text"] * 2, ["http-json-raw-type-private"]):
            self.assertEqual(audit.http_type(records(labels)), "UNOBSERVED_OR_AMBIGUOUS")

    def test_storage_public_phase_report_is_recomputed_from_actual_oracle(self):
        storage_tests.StorageCrossRunContractTests.setUpClass()
        control = storage_tests.StorageCrossRunContractTests()
        control.setUp()
        self.addCleanup(control.doCleanups)
        control.successful_dispatch()
        diagnostic = storage.build_report(control.root)
        storage.write_json(control.root / "runtime/storage-cross-run-diagnostic.json", diagnostic)
        source = control.manifest["fixtures"][0]
        events = storage.read_json(control.root / "runtime/callback-events.json")
        with patch.object(audit, "source_bytes", return_value=(ROOT / audit.STORAGE).read_bytes()):
            report = audit.phase_replays(control.root, diagnostic, self.oracle.program,
                                         control.manifest, "a" * 40, source, events)
            self.assertTrue(report["crossRunTextVerified"])
            self.assertFalse(report["rawLaunchAndKeyIndependentlyInspected"])
            faults = [
                ("reader_success", lambda d: d["reader"].update(verified=False)),
                ("replayed_nonce", lambda d: d["reader"].update(runId=d["writer"]["runId"])),
                ("writer_only", lambda d: d.update(readerDispatchedAfterVerifiedWriter=False)),
                ("changed_source", lambda d: d.update(sourceSha256="e" * 64)),
                ("ttl_claim", lambda d: d.update(ttlProven=True)),
            ]
            for name, mutate in faults:
                with self.subTest(name=name):
                    changed = copy.deepcopy(diagnostic)
                    mutate(changed)
                    with self.assertRaises(ValueError):
                        audit.phase_replays(control.root, changed, self.oracle.program,
                                            control.manifest, "a" * 40, source, events)


if __name__ == "__main__":
    unittest.main()
