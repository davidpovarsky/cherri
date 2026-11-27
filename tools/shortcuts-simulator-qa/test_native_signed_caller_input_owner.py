import copy
import json
from pathlib import Path
import tempfile
import unittest
from unittest import mock
import zipfile

import native_signed_caller_input_owner as signed

ROOT = Path(__file__).resolve().parents[2]
FIXTURE = ROOT / "tests/language-v2/fn04-exact-file-signing-originals.zip"
RETRY_FIXTURE = ROOT / "tests/language-v2/fn04-exact-file-signing-companion-retry-originals.zip"


def encoded(value):
    return (json.dumps(value, sort_keys=True, indent=2) + "\n").encode()


def fixture(case="nested_false", *, retry=False):
    with zipfile.ZipFile(RETRY_FIXTURE if retry else FIXTURE) as archive:
        names = archive.namelist()
        if len(names) != len(set(names)) or any(n.endswith("/") for n in names):
            raise AssertionError("closed original fixture")
        files = {name: archive.read(name) for name in names}
    raw = files.pop("intake.original.json")
    intake = signed.strict(raw)
    for name, pin in intake["files"].items():
        if pin != {"bytes": len(files[name]), "sha256": signed.sha(files[name])}:
            raise AssertionError("exact original fixture bytes")
    result = signed.strict(files["result.original.json"])
    packet = signed.strict(files["cases/" + case + "/packet.derived.json"])
    prefix = "cases/" + case + "/"
    checked = {"caseId": case, "name": packet["workflowName"], "token": packet["packetToken"],
        "source": files[prefix + "source.original.cherri"],
        "unsigned": files[prefix + "unsigned.original.plist"],
        "sourceSha256": packet["sourceSha256"], "unsignedSha256": packet["unsignedSha256"]}
    plan = {"compilerCommitSha": result["compilerCommitSha"], "caseId": case,
            "bundleIntakeSHA256": result["packetIntakeSHA256"]}
    return intake, files, plan, checked


def change_result(intake, files, callback):
    result = signed.strict(files["result.original.json"])
    callback(result)
    files["result.original.json"] = encoded(result)
    intake["originalResultSHA256"] = signed.sha(files["result.original.json"])
    return result


def write_bundle(root, intake, files):
    for name, raw in files.items():
        path = root / name
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_bytes(raw)
    raw = encoded(intake)
    (root / "intake.original.json").write_bytes(raw)
    return signed.sha(raw)


class SignedCallerOriginalTests(unittest.TestCase):
    def test_actual_two_signed_cases_keep_aggregate_failure_and_structural_tier(self):
        for case in ("nested_false", "nested_true"):
            with self.subTest(case=case):
                intake, files, plan, checked = fixture(case)
                raw, receipt = signed.validate_originals(intake, files, plan, checked)
                self.assertEqual(raw, files["cases/" + case + "/signed.original.shortcut"])
                self.assertEqual(receipt["originalOverallStatus"], "FAIL")
                self.assertEqual(receipt["failedCases"], ["without_functions"])
                self.assertEqual(receipt["verificationTier"], "AEA1_HEADER_PASS")
                for field in ("nativeExecuted", "appleSignatureValidated", "formalAcceptanceGranted"):
                    self.assertIs(receipt[field], False)

    def test_separate_actual_companion_retry_retains_prior_502_and_case_scope(self):
        intake, files, plan, checked = fixture("without_functions", retry=True)
        raw, receipt = signed.validate_originals(intake, files, plan, checked)
        self.assertEqual(raw, files["cases/without_functions/signed.original.shortcut"])
        self.assertEqual(receipt["successfulCases"], ["without_functions"])
        self.assertEqual(receipt["previousResultSHA256"], signed.PREVIOUS_RESULT_SHA256)
        self.assertEqual(signed.strict(files["previous-result.original.json"])["status"], "FAIL")
        self.assertIn(b"502 Bad Gateway", files["previous-failed-signing.stderr.original"])
        for field in ("nativeExecuted", "appleSignatureValidated", "formalAcceptanceGranted"):
            self.assertIs(receipt[field], False)
        for name in sorted(signed.RETRY_FILES):
            with self.subTest(changed_prior=name):
                changed = dict(files)
                changed[name] = b"changed"
                with self.assertRaisesRegex(ValueError, "ONE_COMPANION_RETRY_PRESERVES_ORIGINAL_502"):
                    signed.validate_originals(intake, changed, plan, checked)
        with self.assertRaisesRegex(ValueError, "SELECTED_PACKET_NOT_SIGNED"):
            signed.validate_originals(intake, files, dict(plan, caseId="nested_false"),
                                     dict(checked, caseId="nested_false"))

    def test_actual_failed_companion_is_not_admitted(self):
        intake, files, plan, checked = fixture("without_functions")
        self.assertIn(b"502 Bad Gateway", files["commands/sign-without_functions.stderr.original"])
        with self.assertRaisesRegex(ValueError, "SELECTED_PACKET_NOT_SIGNED"):
            signed.validate_originals(intake, files, plan, checked)

    def test_signer_packet_input_and_authority_corruptions_are_rejected(self):
        mutations = {
            "compiler-source": lambda i, f, p, c: p.update(compilerCommitSha=signed.SIGNER_SOURCE),
            "source": lambda i, f, p, c: f.__setitem__("cases/nested_false/source.original.cherri", b"changed"),
            "unsigned": lambda i, f, p, c: f.__setitem__("cases/nested_false/unsigned.original.plist", b"changed"),
            "workflow-name": lambda i, f, p, c: c.update(name="DifferentWorkflow"),
            "owner": lambda i, f, p, c: f.__setitem__("owner.original.py", b"unqualified"),
            "build-info": lambda i, f, p, c: f.__setitem__("signer-build-info.original.txt", b"other binary"),
            "build-authority": lambda i, f, p, c: f.__setitem__("signer-build-authority.original.json", b"{}"),
            "signer-replaced": lambda i, f, p, c: f.__setitem__("signer-after.original.json", b"{}"),
            "closing-failed": lambda i, f, p, c: change_result(i, f, lambda r: r.update(sourceAndSignerUnchanged=False)),
            "fallback": lambda i, f, p, c: change_result(i, f, lambda r: r.update(providerFallback=True)),
            "recompiled": lambda i, f, p, c: change_result(i, f, lambda r: r.update(compiled=True)),
            "extra-input": lambda i, f, p, c: f.__setitem__("unexpected.original", b"unbound"),
        }
        for name, mutation in mutations.items():
            with self.subTest(mutation=name):
                intake, files, plan, checked = fixture()
                mutation(intake, files, plan, checked)
                with self.assertRaises(ValueError):
                    signed.validate_originals(intake, files, plan, checked)

    def test_original_process_join_rejects_wrong_route_bounds_streams_and_cleanup(self):
        def altered(intake, files, fn, index=0, fix_start=False):
            result = signed.strict(files["result.original.json"])
            process = result["processes"][index]
            fn(process)
            prefix = "commands/" + process["name"] + "."
            files[prefix + "process.original.json"] = encoded(process)
            if fix_start:
                start = signed.strict(files[prefix + "start.original.json"])
                for key in ("argv", "name", "pid", "pgid", "sid", "startedAt", "commandSeconds", "wholeOwnerSeconds"):
                    start[key] = process[key]
                files[prefix + "start.original.json"] = encoded(start)
            files["result.original.json"] = encoded(result)
            intake["originalResultSHA256"] = signed.sha(files["result.original.json"])
        mutations = {
            "different-service": lambda p: p["argv"].__setitem__(10, "https://example.invalid/sign"),
            "bound": lambda p: p.update(commandSeconds=121),
            "missing-eof": lambda p: p["stdout"].update(complete=False),
            "different-stream": lambda p: p["stdout"].update(sha256="0" * 64),
            "unreaped": lambda p: p["cleanup"].update(parentReaped=False),
            "unknown-group": lambda p: p["cleanup"]["after"].update(state="UNOBSERVABLE"),
            "cleanup-error": lambda p: p["cleanup"].update(errors=["failed"]),
            "timeout": lambda p: p.update(timedOut=True),
            "cancellation": lambda p: p.update(cancelled=True),
            "no-real-exit": lambda p: p.update(exitCode=None),
            "boolean-pid": lambda p: p.update(pid=True, pgid=True, sid=True),
        }
        for name, mutation in mutations.items():
            with self.subTest(mutation=name):
                intake, files, plan, checked = fixture()
                altered(intake, files, mutation, fix_start=True)
                with self.assertRaises(ValueError):
                    signed.validate_originals(intake, files, plan, checked)
        intake, files, plan, checked = fixture()
        first = signed.strict(files["result.original.json"])["processes"][0]["pid"]
        altered(intake, files, lambda p: p.update(pid=first, pgid=first, sid=first),
                index=1, fix_start=True)
        with self.assertRaisesRegex(ValueError, "DISTINCT_ORIGINAL_SIGNER_PROCESS"):
            signed.validate_originals(intake, files, plan, checked)
        intake, files, plan, checked = fixture()
        start = signed.strict(files["commands/sign-nested_false.start.original.json"])
        start["ownershipVerified"] = False
        files["commands/sign-nested_false.start.original.json"] = encoded(start)
        with self.assertRaisesRegex(ValueError, "OWNED_SIGNER_PROCESS_START"):
            signed.validate_originals(intake, files, plan, checked)

    def test_signed_manifest_and_native_overclaims_are_rejected(self):
        for field, value in (("compilerCommitSha", signed.SIGNER_SOURCE),
                ("verificationTier", "APPLE_SIGNATURE_VALIDATED"), ("nativeActionCount", 4),
                ("sourceSha256", "0" * 64), ("signedArtifactSha256", "0" * 64)):
            with self.subTest(field=field):
                intake, files, plan, checked = fixture()
                path = "cases/nested_false/provenance.original.json"
                manifest = signed.strict(files[path])
                manifest[field] = value
                files[path] = encoded(manifest)
                change_result(intake, files, lambda r: r["cases"][0].update(
                    manifest={"bytes": len(files[path]), "sha256": signed.sha(files[path])}))
                with self.assertRaisesRegex(ValueError, "EXACT_SIGNING_MANIFEST"):
                    signed.validate_originals(intake, files, plan, checked)
        for field in ("nativeExecuted", "appleSignatureValidated", "formalAcceptanceGranted"):
            with self.subTest(overclaim=field):
                intake, files, plan, checked = fixture()
                change_result(intake, files, lambda r: r.update({field: True}))
                with self.assertRaisesRegex(ValueError, "SIGNING_TIER_OVERCLAIM"):
                    signed.validate_originals(intake, files, plan, checked)
        intake, files, plan, checked = fixture()
        change_result(intake, files, lambda r: r.update(
            status="PASS_THREE_EXACT_UNSIGNED_PACKETS_SIGNED_STRUCTURAL_ONLY"))
        intake["originalOverallStatus"] = "PASS_THREE_EXACT_UNSIGNED_PACKETS_SIGNED_STRUCTURAL_ONLY"
        with self.assertRaisesRegex(ValueError, "PRESERVE_AGGREGATE_SIGNING_FAILURE"):
            signed.validate_originals(intake, files, plan, checked)

    def test_changed_bytes_and_directory_redirect_cannot_reuse_an_intake_pin(self):
        intake, files, plan, checked = fixture()
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary) / "bundle"
            root.mkdir()
            pin = write_bundle(root, intake, files)
            selected = root / "cases/nested_false/signed.original.shortcut"
            selected.write_bytes(b"AEA1changed")
            with self.assertRaisesRegex(ValueError, "ORIGINAL_BYTES_CHANGED"):
                signed.load_bundle(root, pin)
            selected.write_bytes(files["cases/nested_false/signed.original.shortcut"])
            moved = Path(temporary) / "moved"
            (root / "cases").rename(moved)
            (root / "cases").symlink_to(moved, target_is_directory=True)
            with self.assertRaisesRegex(ValueError, "SIGNING_DIRECTORY_REDIRECT"):
                signed.load_bundle(root, pin)

    def test_closed_selection_binds_exact_plan_and_disjoint_namespace(self):
        plan = {"repo": "/Users/m1/source", "root": "/Users/m1/native",
                "buildRoot": "/Users/m1/build", "bundleRoot": "/Users/m1/packets",
                "runId": "fresh-one", "runNonce": "1" * 32}
        selection = {"schema": signed.SELECTION_SCHEMA,
                "planSHA256": signed.sha(signed.base.library.canonical(plan) + b"\n"),
                "bundleRoot": "/Users/m1/signatures", "bundleIntakeSHA256": "a" * 64}
        self.assertEqual(signed.validate_selection(selection, plan), selection)
        for change in ({"runId": "another"}, {"runNonce": "2" * 32}):
            with self.subTest(plan_change=change):
                with self.assertRaisesRegex(ValueError, "EXACT_NATIVE_PLAN_SELECTION"):
                    signed.validate_selection(selection, dict(plan, **change))
        for change in ({"extra": True}, {"bundleRoot": "/tmp/other"},
                {"bundleRoot": "/Users/m1/packets/nested"}, {"bundleRoot": "/Users/m1//signatures"},
                {"bundleRoot": "/Users/m1/../signatures"}, {"bundleIntakeSHA256": "BAD"}):
            with self.subTest(selection_change=change):
                with self.assertRaises(ValueError):
                    signed.validate_selection(dict(selection, **change), plan)

    def test_signature_step_retains_exact_originals_without_any_process_or_resigning(self):
        intake, files, plan, checked = fixture()
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            bundle = root / "bundle"; bundle.mkdir()
            pin = write_bundle(bundle, intake, files)
            folder = root / "runtime"; folder.mkdir()
            owner = signed.SignedNativeCallerObservation.__new__(signed.SignedNativeCallerObservation)
            owner.plan = plan
            owner.folder = folder
            owner.summary = {}
            owner.signing_selection = {"bundleRoot": str(bundle), "bundleIntakeSHA256": pin}
            owner.capture = mock.Mock(side_effect=AssertionError("no second signing process"))
            with mock.patch.object(signed.base, "bind_packet_bundle",
                    return_value=({"actual": "packet"}, {}, checked, {"intakeSHA256": plan["bundleIntakeSHA256"]})):
                owner.sign_original()
            owner.capture.assert_not_called()
            self.assertEqual(owner.signed.read_bytes(), files["cases/nested_false/signed.original.shortcut"])
            self.assertEqual(owner.unsigned.read_bytes(), checked["unsigned"])
            self.assertEqual(owner.summary["signingOriginals"]["files"], intake["files"])
            retained = signed.load_bundle(folder / "exact-file-signing-originals", pin)
            self.assertEqual(retained[2], files)
            self.assertIs(owner.summary["signingOriginals"]["binding"]["signingRepeated"], False)


if __name__ == "__main__":
    unittest.main()
