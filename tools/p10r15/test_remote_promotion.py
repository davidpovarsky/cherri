"""Causal gate and filesystem transaction tests; no Apple/signing proof is claimed."""
import copy
import errno
from contextlib import ExitStack
import json
import os
from pathlib import Path
import shutil
import signal
import sys
import subprocess
import tempfile
import unittest
from unittest.mock import patch

import promote_remote_release as promotion
import remote_release_gates as gates
import stage_remote_release
import verify_remote_smoke

ARCHIVE = Path(os.environ.get("CHERRI_PROMOTION_TEST_ARCHIVE", str(
    Path(__file__).resolve().parents[2] / "docs/apple-runtime/p10r15/remote-release-d6872ba-20261119")))


def smoke_model(home, release, hashes, modes, default=False):
    """An explicitly synthetic unit-test model, never emitted as acceptance evidence."""
    rows = []
    for route in ("v2", "legacy"):
        for operation in ("status", "check", "unsigned", "signed"):
            artifacts = []
            if operation in ("unsigned", "signed"):
                ids = ["is.workflow.actions.gettext", "is.workflow.actions.showresult"]
                if route == "legacy":
                    ids.insert(1, "is.workflow.actions.setvariable")
                artifacts = [{"file": route + ".shortcut", "sha256": "1" * 64, "bytes": 100,
                              "magicHex": "41454131" if operation == "signed" else "3c3f786d",
                              "nativeActionIDs": ids, "nativeActions": len(ids),
                              "aea1EnvelopeObserved": operation == "signed", "syntheticSemanticsMatch": True}]
            rows.append({"route": route, "operation": operation, "verdict": "PASS", "exitCode": 0,
                         "stdoutSha256": gates.sha(b"unit stdout"), "stderrSha256": gates.sha(b""),
                         "stdoutBytes": 11, "stderrBytes": 0, "seconds": 0.1,
                         "diagnosticTail": "Synthetic unit model, not acceptance evidence",
                         "sourceUnchanged": True, "artifacts": artifacts})
    return {"formatVersion": 1, "scope": "ACTUAL_REMOTE_ROUTES_PUBLIC_SYNTHETIC_SIGNING", "tuple": release,
            "remoteHome": str(home), "usedDefaultHome": default, "passed": True, "fatalError": None,
            "privateCorpusUsed": False, "appleCryptographicValidation": "NOT_TESTED", "deviceExecution": "NOT_TESTED",
            "externalSigningRequested": True, "signingCommandAttempts": 2,
            "protectedUnchanged": True, "protectedHashesBefore": hashes, "protectedHashesAfter": hashes,
            "protectedModesBefore": modes, "protectedModesAfter": modes,
            "registeredLinkBefore": {"target": promotion.LINK_TARGET, "mode": 0o777},
            "registeredLinkAfter": {"target": promotion.LINK_TARGET, "mode": 0o777},
            "results": rows, "summary": {"checks": 8, "passed": 8, "failed": 0}}


def reports_model():
    names = {"stage": "staging-receipt.json", "entrypoints": "candidate-entrypoint-partial.json",
             "faults": "candidate-signing-faults.json", "sourceIntegrity": "source-create.json",
             "runtimeIntegrity": "runtime-create.json", "unpackedIntegrity": "runtime-verify.json"}
    records = {key: json.loads((ARCHIVE / name).read_bytes()) for key, name in names.items()}
    # Historical files remain unchanged and are not new release evidence. This
    # in-memory model supplies the paired runtime and new causal cases solely to
    # exercise the current gate's rejection logic.
    stage = records["stage"]
    stage["tuple"]["migrationSHA256"] = "8" * 64
    records["entrypoints"]["tuple"] = copy.deepcopy(stage["tuple"])
    records["faults"]["provenance"]["installedTuple"] = copy.deepcopy(stage["tuple"])
    provenance = stage["binaryProvenance"]
    provenance["modulePath"] = "github.com/electrikmilk/cherri"
    provenance["migration"] = {
        "compilerCommit": stage["tuple"]["compilerCommit"], "goarch": "arm64", "goos": "linux",
        "sha256": stage["tuple"]["migrationSHA256"], "vcsModified": False,
        "modulePath": "github.com/electrikmilk/cherri/cmd/cherri-migrate"}
    records["sourceIntegrity"]["verifiedFiles"] = 23
    for key in ("runtimeIntegrity", "unpackedIntegrity"):
        records[key]["verifiedFiles"] = 29
        records[key]["runtime"] = copy.deepcopy(provenance)
    entry = records["entrypoints"]
    for row in entry["results"]:
        if row["check"] == "runner_existing_tests":
            row.update(verdict="PASS", exitCode=0, nativeArtifacts=[], seconds=0.1,
                       stdoutSha256=gates.sha(b"unit model pass"), stdoutBytes=15,
                       diagnosticTail="Synthetic unit model, not acceptance evidence")
    for name, diagnostic in (("reject_tuple_migrationSHA256", "IMMUTABLE_RELEASE_MISMATCH"),
                             ("reject_migration_drift", "IMMUTABLE_MIGRATION_MISMATCH")):
        entry["results"].append({"check": name, "kind": "CAUSAL_REJECTION", "verdict": "PASS",
                                 "exitCode": 1, "nativeArtifacts": [], "seconds": 0.1,
                                 "stdoutSha256": gates.sha(b""), "stdoutBytes": 0,
                                 "diagnosticTail": "Synthetic unit model: " + diagnostic})
    entry["summary"] = {"checks": 39, "passed": 39, "failed": 0, "notRun": 0}
    entry["activationGatePassed"] = True
    hashes = {name: stage["candidateHashes"][name] for name in gates.PROTECTED}
    modes = {name: 0o700 if name in promotion.SWITCH_ORDER[:2] else 0o600 for name in gates.PROTECTED}
    records["candidateSmoke"] = smoke_model(stage["candidateHome"], stage["tuple"], hashes, modes)
    return records


class EvidenceGates(unittest.TestCase):
    def validate(self, records):
        provenance = records["faults"]["provenance"]
        return gates.validate_reports(records, records["stage"]["candidateHome"],
                                      provenance["skillManifest"], provenance["skillBuildSha256"])

    def test_incomplete_actual_historical_report_rejected(self):
        records = reports_model()
        records["entrypoints"] = json.loads((ARCHIVE / "candidate-entrypoint-partial.json").read_bytes())
        with self.assertRaises(ValueError):
            self.validate(records)

    def test_complete_unit_model_and_causal_corruptions(self):
        baseline = reports_model()
        self.validate(baseline)
        mutations = {
            "missing_entry": lambda r: r["entrypoints"]["results"].pop(),
            "duplicate_entry": lambda r: r["entrypoints"]["results"].__setitem__(0, r["entrypoints"]["results"][1]),
            "entry_skip": lambda r: r["entrypoints"]["results"][0].update(verdict="NOT_RUN"),
            "summary_only_green": lambda r: r["entrypoints"]["results"][0].update(exitCode=1),
            "boolean_exit": lambda r: r["entrypoints"]["results"][0].update(exitCode=False),
            "boolean_summary": lambda r: r["entrypoints"]["summary"].update(failed=False),
            "stale_candidate": lambda r: r["entrypoints"]["candidateHashesAfter"].update({gates.PROTECTED[1]: "0" * 64}),
            "dirty_live": lambda r: r["entrypoints"].update(liveUnchanged=False),
            "missing_fault": lambda r: r["faults"]["results"].pop(),
            "duplicate_fault": lambda r: r["faults"]["results"].__setitem__(0, r["faults"]["results"][1]),
            "timeout_not_rejected": lambda r: next(x for x in r["faults"]["results"] if x["case"] == "timeout").update(exitCode=0),
            "artifact_on_negative": lambda r: r["faults"]["results"][1].update(artifacts=[{"bytes": 1}]),
            "missing_multipart": lambda r: r["faults"]["results"][1].update(multipartProtocolObserved=False),
            "stale_binary": lambda r: r["faults"]["provenance"].update(compilerSha256="0" * 64),
            "stale_wrapper": lambda r: r["faults"]["provenance"].update(remoteRunnerSha256="0" * 64),
            "missing_exact_preflight": lambda r: next(x for x in r["faults"]["results"] if x["surface"] == "remote_v2_copy" and x["case"] == "xml").update(exactPreflightBytesObserved=False),
            "wrong_uploaded_bytes": lambda r: next(x for x in r["faults"]["results"] if x["surface"] == "remote_legacy_copy" and x["case"] == "xml")["requestEvidence"][0].update(preflightSha256="0" * 64),
            "source_inventory_missing": lambda r: r["sourceIntegrity"].update(verifiedFiles=19),
            "old_source_inventory": lambda r: r["sourceIntegrity"].update(verifiedFiles=20),
            "old_runtime_inventory": lambda r: r["runtimeIntegrity"].update(verifiedFiles=23),
            "migration_tuple_missing": lambda r: r["stage"]["tuple"].pop("migrationSHA256"),
            "migration_provenance_missing": lambda r: r["stage"]["binaryProvenance"].pop("migration"),
            "migration_wrong_role": lambda r: r["stage"]["binaryProvenance"]["migration"].update(modulePath="github.com/electrikmilk/cherri"),
            "migration_wrong_source": lambda r: r["stage"]["binaryProvenance"]["migration"].update(compilerCommit="0" * 40),
            "migration_wrong_platform": lambda r: r["stage"]["binaryProvenance"]["migration"].update(goarch="amd64"),
            "migration_package_drift": lambda r: r["runtimeIntegrity"]["runtime"]["migration"].update(sha256="0" * 64),
            "unpack_missing": lambda r: r["unpackedIntegrity"].update(unpacked=False),
            "package_wrong_S": lambda r: r["runtimeIntegrity"].update(compilerCommit="0" * 40),
            "package_wrong_K": lambda r: r["sourceIntegrity"].update(skillCommit="0" * 40),
            "package_wrong_D": lambda r: r["sourceIntegrity"].update(docsCommit="0" * 40),
            "smoke_missing": lambda r: r["candidateSmoke"]["results"].pop(),
            "smoke_wrong_home": lambda r: r["candidateSmoke"].update(remoteHome="/another/home"),
            "smoke_fake_signed": lambda r: r["candidateSmoke"]["results"][3]["artifacts"][0].update(magicHex="3c3f786d"),
            "smoke_missing_attempt": lambda r: r["candidateSmoke"].update(signingCommandAttempts=1),
            "smoke_claims_apple": lambda r: r["candidateSmoke"].update(appleCryptographicValidation="PASS"),
            "smoke_mode_drift": lambda r: r["candidateSmoke"].update(protectedModesAfter={}),
            "missing_tool_binding": lambda r: r["stage"]["externalToolchainDependencies"].pop("tools/go/bin/go"),
            "boolean_version": lambda r: r["entrypoints"].update(formatVersion=True),
            "missing_execution_metadata": lambda r: r["entrypoints"]["results"][0].pop("stdoutSha256"),
            "wrong_native_graph": lambda r: next(x for x in r["entrypoints"]["results"] if x["check"] == "runner_compile_unsigned")["nativeArtifacts"][0].update(nativeActionIDs=["unrelated.action"]),
            "missing_causal_diagnostic": lambda r: next(x for x in r["entrypoints"]["results"] if x["check"].startswith("reject_tuple_")).update(diagnosticTail="unrelated failure"),
            "unsigned_blob_not_plist": lambda r: r["faults"]["results"][0].update(artifacts=[{"bytes": 1, "magicHex": "00"}]),
            "floating_package_count": lambda r: r["sourceIntegrity"].update(verifiedFiles=20.0),
            "smoke_no_execution_metadata": lambda r: r["candidateSmoke"]["results"][0].pop("stdoutBytes"),
            "smoke_wrong_native_graph": lambda r: r["candidateSmoke"]["results"][2]["artifacts"][0].update(nativeActions=99),
        }
        for name, mutate in mutations.items():
            with self.subTest(mutation=name):
                changed = copy.deepcopy(baseline)
                mutate(changed)
                with self.assertRaises((ValueError, KeyError)):
                    self.validate(changed)

    def test_duplicate_json_keys_rejected(self):
        with tempfile.TemporaryDirectory() as temporary:
            path = Path(temporary) / "receipt.json"
            path.write_text('{"passed": false, "passed": true}')
            with self.assertRaisesRegex(ValueError, "Duplicate JSON"):
                gates.read_json(path)

    def test_actual_manifest_files_missing_stale_duplicate_or_wrong_stage_rejected(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary); stage = root / "stage"; stage.mkdir()
            records = reports_model(); files = {}
            for key in gates.EVIDENCE_KEYS:
                path = stage / "staging-receipt.json" if key == "stage" else root / (key + ".json")
                path.write_bytes(b"synthetic archive unit fixture" if key.endswith("Archive") else json.dumps(records[key]).encode())
                files[key] = {"path": str(path), "sha256": gates.sha(path.read_bytes())}
            original = {"formatVersion": 1, "tuple": records["stage"]["tuple"], "files": files}
            path = root / "manifest.json"; path.write_text(json.dumps(original))
            gates.load_evidence(path, stage)
            for name, mutate in {
                "missing": lambda d: d["files"].pop("entrypoints"),
                "stale": lambda d: d["files"]["faults"].update(sha256="0" * 64),
                "duplicate": lambda d: d["files"].update(sourceArchive=d["files"]["runtimeArchive"]),
                "typed_version": lambda d: d.update(formatVersion=True),
                "tuple": lambda d: d["tuple"].update(compilerCommit="0" * 40),
                "wrong_stage": lambda d: d["files"].update(stage=d["files"]["entrypoints"]),
            }.items():
                with self.subTest(mutation=name):
                    changed = copy.deepcopy(original); mutate(changed); path.write_text(json.dumps(changed))
                    with self.assertRaises((ValueError, KeyError)):
                        gates.load_evidence(path, stage)


class TransactionFixture:
    def __init__(self, hook=None):
        self.temporary = tempfile.TemporaryDirectory(prefix="p15-transaction-test-")
        self.root = Path(self.temporary.name)
        self.live = self.root / "user/workspaces/cherri-remote"
        self.stage = self.root / "stage"
        self.candidate = self.stage / "remote-home"
        self.journal = self.root / "journal"
        self.release = {"label": "p15-unit-fixture", "sourceBranch": "agent/unit-fixture",
                        "compilerCommit": "1" * 40, "docsCommit": "2" * 40, "skillSourceHead": "3" * 40,
                        "schemaFingerprint": "4" * 64, "codecFingerprint": "5" * 64,
                        "macroFingerprint": "6" * 64, "compilerSHA256": "7" * 64,
                        "migrationSHA256": "8" * 64}
        self.old = {**self.release, "label": "p10", "sourceBranch": "agent/old-unit-fixture"}
        for home, release in ((self.live, self.old), (self.candidate, self.release)):
            for directory in ("bin", "config", "source", "jobs", "outputs", "tmp", "tools/go"):
                (home / directory).mkdir(parents=True, exist_ok=True)
            (home / "active.json").write_text(json.dumps(release))
            (home / "active.json").chmod(0o600)
            (home / "config/default-language-mode").write_bytes(b"legacy\n")
            (home / "config/default-language-mode").chmod(0o600)
            (home / "tools/go/VERSION").write_bytes(b"toolchain retained\n")
            (home / "tools/go/VERSION").chmod(0o600)
            (home / "bin/cherri-shortcut-lint.py").write_bytes(b"# unchanged validator\n")
            (home / "bin/cherri-shortcut-lint.py").chmod(0o600)
            script = ("#!/usr/bin/env python3\nimport json,pathlib,sys\n"
                      "home=pathlib.Path(__file__).resolve().parents[1]\n"
                      "x=json.loads((home/'active.json').read_text())\n"
                      "assert x['label']==" + repr(release["label"]) + "\n")
            runner = home / promotion.PROTECTED[1]
            runner.write_text(script + "print(json.dumps(x))\n"); runner.chmod(0o700)
            legacy = home / promotion.PROTECTED[2]
            legacy.write_text(script + "import hashlib\nassert hashlib.sha256((home/'bin/cherri-remote-runner.sh').read_bytes()).hexdigest()=="
                + repr(gates.sha(runner.read_bytes())) + "\nprint('CHERRI_REMOTE_LANGUAGE_MODE=legacy')\n"
                "print('CHERRI_REMOTE_SOURCE_BRANCH='+x['sourceBranch'])\n"
                "for k in ('compilerCommit','docsCommit','schemaFingerprint','sourceBranch'): print(k+'='+x[k])\n")
            legacy.chmod(0o700)
            (home / promotion.LINK).symlink_to(promotion.LINK_TARGET)
        for relative in ("p10/runtime/keep", "p9/runtime/keep", "pure-upstream/keep", "jobs/user-job", "outputs/user-output"):
            path = self.live / relative; path.parent.mkdir(parents=True, exist_ok=True); path.write_bytes(b"unrelated preserved")
        label = self.release["label"]
        for path in (self.candidate / "source" / label, self.candidate / label / "runtime"):
            path.mkdir(parents=True); (path / "immutable-data").write_bytes(b"new immutable tree")
        protected = (*promotion.PROTECTED, "tools/go/VERSION")
        report = {"tuple": self.release, "liveHashesBefore": {p: gates.sha((self.live / p).read_bytes()) for p in protected},
                  "candidateHashes": {p: gates.sha((self.candidate / p).read_bytes()) for p in protected}}
        self.stack = ExitStack()
        self.stack.enter_context(patch.dict(os.environ, HOME=str(self.root / "user")))
        self.stack.enter_context(patch.object(promotion, "preflight", return_value=({"files": {}}, {}, {"stage": report})))
        self.stack.enter_context(patch.object(promotion, "verify_runtime"))
        self.stack.enter_context(patch.object(promotion, "probe", side_effect=self.smoke))
        self.tx = promotion.Promotion.prepare(self.stage, self.live, self.journal, self.root / "unused-manifest", hook)

    def smoke(self, home, release, report, default):
        hashes = {name: gates.sha((home / name).read_bytes()) for name in promotion.PROTECTED}
        modes = {name: (home / name).stat().st_mode & 0o7777 for name in promotion.PROTECTED}
        value = smoke_model(home, release, hashes, modes, default)
        Path(report).write_text(json.dumps(value))
        return value

    def assert_old(self, test):
        for name, record in self.tx.document["before"].items():
            test.assertEqual(promotion.regular_state(self.live / name), record)
        test.assertEqual(promotion.link_state(self.live / promotion.LINK), self.tx.document["registeredLink"])
        for relative in ("p10/runtime/keep", "p9/runtime/keep", "pure-upstream/keep", "jobs/user-job", "outputs/user-output"):
            test.assertEqual((self.live / relative).read_bytes(), b"unrelated preserved")

    def close(self):
        self.stack.close(); self.temporary.cleanup()


class TransactionBoundaries(unittest.TestCase):
    def fixture(self, hook=None):
        fixture = TransactionFixture(hook)
        self.addCleanup(fixture.close)
        return fixture

    def test_actual_files_cycle_rollback_reapply_and_manual_rollback(self):
        fixture = self.fixture()
        fixture.tx.cycle()
        for name, record in fixture.tx.document["after"].items():
            self.assertEqual(promotion.regular_state(fixture.live / name), record)
        events = [json.loads(line) for line in (fixture.journal / "events.jsonl").read_text().splitlines()]
        self.assertEqual(events[-1]["event"], "COMPLETED")
        self.assertEqual(sum(row["event"] == "SMOKE_PASSED" for row in events), 2)
        self.assertTrue(any(row["event"] == "ROLLED_BACK" for row in events))
        recovered = promotion.Promotion.load(fixture.journal, fixture.live)
        recovered.rollback("manual-rollback-status")
        fixture.assert_old(self)
        self.assertTrue((fixture.live / fixture.release["label"] / "runtime/immutable-data").exists())
        self.assertTrue((fixture.live / "source" / fixture.release["label"] / "immutable-data").exists())

    def test_every_observed_write_boundary_restores_on_error_and_sigterm(self):
        points = []
        fixture = self.fixture(points.append); fixture.tx.cycle()
        self.assertGreaterEqual(len(points), 50)
        for interrupt in (False, True):
            for point in points:
                with self.subTest(point=point, sigterm=interrupt):
                    fired = False
                    def fail_here(actual):
                        nonlocal fired
                        if actual == point and not fired:
                            fired = True
                            if interrupt:
                                os.kill(os.getpid(), signal.SIGTERM)
                            raise RuntimeError("causal mutation boundary")
                    subject = TransactionFixture(fail_here)
                    try:
                        with promotion.interruption_handlers(), self.assertRaises((RuntimeError, InterruptedError)):
                            subject.tx.cycle()
                        self.assertTrue(fired)
                        subject.assert_old(self)
                        self.assertNotIn('"event": "COMPLETED"', (subject.journal / "events.jsonl").read_text())
                    finally:
                        subject.close()

    def test_hard_interruption_journal_recovers_each_replaced_file(self):
        for phase in ("activate", "rollback", "reapply"):
            for name in (promotion.PROTECTED[1], promotion.PROTECTED[2], promotion.PROTECTED[0]):
                with self.subTest(phase=phase, path=name):
                    point = phase + ":" + name + ":after_replace"
                    fixture = self.fixture(lambda actual: os._exit(91) if actual == point else None)
                    pid = os.fork()
                    if pid == 0:
                        fixture.tx.cycle(); os._exit(92)
                    _, status = os.waitpid(pid, 0)
                    self.assertEqual(os.waitstatus_to_exitcode(status), 91)
                    promotion.Promotion.load(fixture.journal, fixture.live).rollback("recovered-" + phase)
                    fixture.assert_old(self)

    def test_stale_concurrent_file_is_not_overwritten(self):
        fixture = self.fixture()
        changed = False
        def drift(point):
            nonlocal changed
            if point == "activate:" + promotion.PROTECTED[1] + ":before_replace" and not changed:
                changed = True
                (fixture.live / promotion.PROTECTED[1]).write_bytes(b"concurrent foreign edit")
        fixture.tx.hook = drift
        with self.assertRaisesRegex(RuntimeError, "journal recovery"):
            fixture.tx.cycle()
        self.assertEqual((fixture.live / promotion.PROTECTED[1]).read_bytes(), b"concurrent foreign edit")
        self.assertIn('"event": "ROLLBACK_FAILED"', (fixture.journal / "events.jsonl").read_text())

    def test_no_replace_directory_keeps_concurrent_destination(self):
        fixture = self.fixture()
        path = fixture.live / "source" / fixture.release["label"]
        def occupy(point):
            if point == "install:before_move:source/" + fixture.release["label"]:
                path.mkdir(); (path / "foreign").write_bytes(b"do not overwrite")
        fixture.tx.hook = occupy
        with self.assertRaises(OSError):
            fixture.tx.cycle()
        self.assertEqual((path / "foreign").read_bytes(), b"do not overwrite")
        fixture.assert_old(self)

    def test_exclusive_lock_and_modified_journal_rejected(self):
        fixture = self.fixture()
        with promotion.deployment_lock(fixture.live):
            with self.assertRaisesRegex(ValueError, "Another deployment"):
                with promotion.deployment_lock(fixture.live):
                    self.fail("concurrent lock accepted")
        (fixture.journal / "before/0").write_bytes(b"forged backup")
        with self.assertRaisesRegex(ValueError, "snapshot was modified"):
            promotion.Promotion.load(fixture.journal, fixture.live)

    def test_event_write_failure_after_switch_still_restores_files(self):
        fixture = self.fixture()
        broken = False
        original_open = os.open
        def hook(point):
            nonlocal broken
            if point == "activate:" + promotion.PROTECTED[1] + ":after_replace":
                broken = True
        def failing_open(path, *args, **kwargs):
            if broken and str(path).endswith("events.jsonl"):
                raise OSError(errno.ENOSPC, "causal journal-full failure")
            return original_open(path, *args, **kwargs)
        fixture.tx.hook = hook
        with patch.object(os, "open", side_effect=failing_open), self.assertRaises(OSError):
            fixture.tx.cycle()
        fixture.assert_old(self)
        self.assertIsNotNone(fixture.tx.journal_error)

    def test_mixed_file_states_never_validate_the_wrong_tuple(self):
        fixture = self.fixture(); observations = []
        def inspect(point):
            if point.endswith(":after_replace"):
                active = json.loads((fixture.live / "active.json").read_bytes())
                codes = []
                for dialect, route in (("v2", fixture.live / promotion.PROTECTED[1]),
                                       ("legacy", fixture.live / promotion.LINK)):
                    rc, stdout, _ = promotion.execute_owned([route, "status"],
                        verify_remote_smoke.scrubbed_env(fixture.live, True), fixture.live, 5)
                    codes.append(rc)
                    if rc == 0:
                        self.assertTrue(verify_remote_smoke.status_matches(stdout, active, dialect))
                observations.append((point, codes))
        fixture.tx.hook = inspect; fixture.tx.cycle()
        self.assertTrue(any(codes == [1, 1] for _, codes in observations))

    def test_manual_cli_reapply_logging_failure_restores_and_returns_nonzero(self):
        fixture = self.fixture()
        fixture.tx.install_directories()
        pid = os.fork()
        if pid == 0:
            broken = False
            original_open, original_replace = os.open, os.replace
            def replace(source, destination):
                nonlocal broken
                original_replace(source, destination)
                if Path(destination) == fixture.live / promotion.PROTECTED[1]:
                    broken = True
            def failing_open(path, *args, **kwargs):
                if broken and str(path).endswith("events.jsonl"):
                    raise OSError(errno.ENOSPC, "causal manual CLI journal failure")
                return original_open(path, *args, **kwargs)
            with patch.object(os, "replace", side_effect=replace), patch.object(os, "open", side_effect=failing_open), \
                 patch.object(sys, "argv", ["promote_remote_release.py", "reapply", "--live-home", str(fixture.live),
                                            "--journal", str(fixture.journal)]):
                os._exit(promotion.main())
        _, status = os.waitpid(pid, 0)
        self.assertEqual(os.waitstatus_to_exitcode(status), 1)
        fixture.assert_old(self)

    def test_canonical_clone_survives_source_removal_without_alternates(self):
        with tempfile.TemporaryDirectory() as temporary:
            source, destination = Path(temporary) / "source", Path(temporary) / "release"
            source.mkdir()
            subprocess.run(["git", "init", "-q", source], check=True)
            (source / "known.txt").write_bytes(b"immutable source")
            subprocess.run(["git", "-C", source, "add", "known.txt"], check=True)
            subprocess.run(["git", "-C", source, "-c", "user.name=Unit test", "-c", "user.email=unit@example.invalid",
                            "commit", "-qm", "synthetic clone fixture"], check=True)
            revision = promotion.git(source, "rev-parse", "HEAD")
            stage_remote_release.checked_clone(source, destination, revision, promotion.CANONICAL)
            shutil.rmtree(source)
            promotion.verify_checkout(destination, revision, promotion.CANONICAL)
            self.assertEqual((destination / "known.txt").read_bytes(), b"immutable source")


class SmokeFailureEvidence(unittest.TestCase):
    def test_early_failure_does_not_claim_signing_and_retains_missing_file_error(self):
        fixture = TransactionFixture()
        try:
            def missing(*args):
                (fixture.live / promotion.PROTECTED[3]).unlink()
                return 1, "", "causal early failure"
            output = fixture.root / "failed-smoke.json"
            with patch.object(verify_remote_smoke, "execute_owned", side_effect=missing):
                report = verify_remote_smoke.probe(fixture.live, fixture.old, output, True)
            self.assertFalse(report["passed"])
            self.assertFalse(report["externalSigningRequested"])
            self.assertEqual(report["signingCommandAttempts"], 0)
            self.assertIn("protected-state", report["fatalError"])
            self.assertTrue(output.exists())
        finally:
            fixture.close()


if __name__ == "__main__":
    unittest.main(verbosity=2)
