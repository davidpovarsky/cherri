"""Causal controls for explicit owned-device selection; no Apple command runs."""
import ast
import contextlib
import copy
import hashlib
import io
import json
from pathlib import Path
import tempfile
import unittest
from unittest import mock

import owned_simulator_selection as p

ROOT = Path(__file__).resolve().parents[2]
SOURCE = "a" * 40
RUN = "owned-selector-control"
UDID = "D631D98E-2271-4948-B465-8EDBA9BD4D68"
FOREIGN = "11111111-2222-3333-4444-555555555555"
RUNTIME = "com.apple.CoreSimulator.SimRuntime.iOS-27-0"
TYPE = "com.apple.CoreSimulator.SimDeviceType.iPhone-16-Pro"


def fixtures():
    record = {"schema": p.SCHEMA, "sourceCommit": SOURCE, "runId": RUN,
              "udid": UDID, "deviceName": "Cherri-P10R15-D03",
              "deviceTypeIdentifier": TYPE, "runtimeId": RUNTIME,
              "runtimeName": "iOS 27.0", "runtimeVersion": "27.0"}
    runtimes = {"runtimes": [{"identifier": RUNTIME, "name": "iOS 27.0",
                            "version": "27.0", "isAvailable": True}]}
    types = {"devicetypes": [{"identifier": TYPE, "name": "iPhone 16 Pro"}]}
    owned = {"udid": UDID, "name": record["deviceName"],
             "deviceTypeIdentifier": TYPE, "isAvailable": True}
    foreign = {**owned, "udid": FOREIGN, "name": "Other project simulator"}
    devices = {"devices": {RUNTIME: [foreign, owned]}}
    return record, runtimes, types, devices


def select(values):
    with mock.patch("subprocess.run", side_effect=AssertionError("No device mutation permitted")), \
         mock.patch("subprocess.Popen", side_effect=AssertionError("No child process permitted")):
        return p.select_owned(*values, SOURCE, RUN, "iphone-fallback")


class OwnedSimulatorSelectionTests(unittest.TestCase):
    def test_exact_owned_device_is_selected_even_when_foreign_device_is_first(self):
        values = fixtures()
        before = copy.deepcopy(values)
        selection = select(values)
        self.assertEqual(selection["udid"], UDID)
        self.assertEqual(selection["deviceName"], "Cherri-P10R15-D03")
        self.assertEqual(selection["selectionMode"], "EXACT_OWNED_EXISTING_DEVICE")
        self.assertEqual(values, before)

    def test_missing_owned_device_cannot_fall_back_to_foreign(self):
        values = fixtures()
        values[3]["devices"][RUNTIME] = values[3]["devices"][RUNTIME][:1]
        with self.assertRaisesRegex(ValueError, "OWNED_DEVICE_MISSING_OR_AMBIGUOUS"):
            select(values)

    def test_record_identity_mismatch_is_rejected(self):
        changes = {"sourceCommit": "b" * 40, "runId": "another-run", "schema": "foreign",
                   "udid": "not-a-uuid", "deviceName": "Other project simulator",
                   "deviceTypeIdentifier": TYPE + "-Other", "runtimeId": RUNTIME + "-Other",
                   "runtimeName": "iOS 27.2", "runtimeVersion": "27.2"}
        for field, value in changes.items():
            with self.subTest(field=field):
                values = fixtures()
                values[0][field] = value
                with self.assertRaises(ValueError):
                    select(values)

    def test_missing_extra_or_wrong_typed_owner_fields_are_rejected(self):
        for field in sorted(p.RECORD_FIELDS):
            for operation in ("missing", "wrong_type", "empty"):
                with self.subTest(field=field, operation=operation):
                    values = fixtures()
                    if operation == "missing":
                        del values[0][field]
                    else:
                        values[0][field] = None if operation == "wrong_type" else ""
                    with self.assertRaises(ValueError):
                        select(values)
        values = fixtures()
        values[0]["allowFallback"] = True
        with self.assertRaises(ValueError):
            select(values)

    def test_unavailable_or_ambiguous_actual_inventory_never_selects(self):
        for kind in ("device_missing_availability", "device_unavailable", "runtime_unavailable",
                     "duplicate_device", "duplicate_device_other_runtime", "duplicate_runtime",
                     "duplicate_type", "missing_type", "wrong_runtime_group", "wrong_name",
                     "wrong_type", "wrong_profile", "not_ios", "not_27"):
            with self.subTest(kind=kind):
                values = fixtures()
                record, runtimes, types, devices = values
                owned = devices["devices"][RUNTIME][1]
                if kind == "device_missing_availability": del owned["isAvailable"]
                elif kind == "device_unavailable": owned["isAvailable"] = False
                elif kind == "runtime_unavailable": runtimes["runtimes"][0]["isAvailable"] = False
                elif kind == "duplicate_device": devices["devices"][RUNTIME].append(dict(owned))
                elif kind == "duplicate_device_other_runtime": devices["devices"]["foreign"] = [dict(owned)]
                elif kind == "duplicate_runtime": runtimes["runtimes"].append(dict(runtimes["runtimes"][0]))
                elif kind == "duplicate_type": types["devicetypes"].append(dict(types["devicetypes"][0]))
                elif kind == "missing_type": types["devicetypes"] = []
                elif kind == "wrong_runtime_group": devices["devices"] = {"foreign": [owned]}
                elif kind == "wrong_name": owned["name"] = "Other"
                elif kind == "wrong_type": owned["deviceTypeIdentifier"] = "Other"
                elif kind == "wrong_profile": types["devicetypes"][0]["name"] = "iPad Pro"
                elif kind == "not_ios":
                    record["runtimeId"] = "com.apple.CoreSimulator.SimRuntime.tvOS-27-0"
                    runtimes["runtimes"][0]["identifier"] = record["runtimeId"]
                    devices["devices"] = {record["runtimeId"]: [owned]}
                elif kind == "not_27":
                    record["runtimeVersion"] = runtimes["runtimes"][0]["version"] = "18.6"
                with self.assertRaises(ValueError):
                    select(values)

    def test_wrong_or_malformed_profile_cannot_select(self):
        for value in ("", "ipad-default", "foreign"):
            with self.subTest(profile=value), self.assertRaises(ValueError):
                p.select_owned(*fixtures(), SOURCE, RUN, value)

    def test_invalid_inventory_shapes_are_rejected(self):
        for index, bad in ((1, []), (1, {"runtimes": [None]}),
                           (2, {"devicetypes": [None]}), (3, {"devices": {RUNTIME: None}}),
                           (3, {"devices": {RUNTIME: [None]}})):
            with self.subTest(index=index, value=bad):
                values = list(fixtures())
                values[index] = bad
                with self.assertRaises(ValueError):
                    select(values)

    def test_duplicate_keys_and_oversized_records_are_rejected(self):
        with self.assertRaisesRegex(ValueError, "DUPLICATE_KEY"):
            p.unique_json(b'{"udid":"one","udid":"two"}')
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            path = root / "record.json"
            path.write_text(" " * 65537)
            with self.assertRaisesRegex(ValueError, "INPUT_TOO_LARGE"):
                p.read_input(path, 65536, {}, "owner")

    def test_symlink_input_is_rejected(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            path = root / "record.json"
            path.write_text("{}")
            alias = root / "alias.json"
            alias.symlink_to(path)
            with self.assertRaisesRegex(ValueError, "INPUT_NOT_REGULAR"):
                p.read_input(alias, 65536, {}, "owner")

    def run_cli(self, *, missing=False):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            record, runtimes, types, devices = fixtures()
            if missing:
                devices["devices"][RUNTIME] = devices["devices"][RUNTIME][:1]
            for name, value in (("record.json", record), ("sim-runtimes.json", runtimes),
                                ("sim-devicetypes.json", types), ("sim-devices.json", devices)):
                (root / name).write_text(json.dumps(value))
            output = root / "selection.json"
            args = ["--record", str(root / "record.json"), "--inventory-root", str(root),
                    "--output", str(output), "--source-sha", SOURCE, "--run-id", RUN,
                    "--profile", "iphone-fallback"]
            stdout, stderr = io.StringIO(), io.StringIO()
            with contextlib.redirect_stdout(stdout), contextlib.redirect_stderr(stderr):
                code = p.main(args)
            receipt = json.loads(output.read_text())
            with self.assertRaises(FileExistsError), contextlib.redirect_stdout(io.StringIO()):
                p.main(args)
            return code, stdout.getvalue(), stderr.getvalue(), receipt

    def test_cli_preserves_success_source_and_all_input_bindings(self):
        code, stdout, stderr, receipt = self.run_cli()
        self.assertEqual(code, 0)
        self.assertEqual(json.loads(stdout)["udid"], UDID)
        self.assertEqual(stderr, "")
        self.assertEqual(receipt["status"], "SELECTED")
        self.assertEqual(receipt["sourceCommit"], SOURCE)
        self.assertEqual(set(receipt["inputBindings"]), {"ownerRecord", "runtimes", "deviceTypes", "devices"})
        self.assertFalse(receipt["nativeExecution"])
        self.assertFalse(receipt["fallbackAttempted"])
        self.assertFalse(receipt["deviceMutationAttempted"])

    def test_cli_retains_rejection_before_any_boot_and_emits_no_device(self):
        code, stdout, stderr, receipt = self.run_cli(missing=True)
        self.assertEqual(code, 2)
        self.assertEqual(stdout, "")
        self.assertIn("OWNED_DEVICE_MISSING_OR_AMBIGUOUS", stderr)
        self.assertEqual(receipt["status"], "REJECTED")
        self.assertEqual(len(receipt["inputBindings"]), 4)
        self.assertNotIn("selection", receipt)
        self.assertFalse(receipt["deviceMutationAttempted"])

    def test_default_selector_bytes_and_source_witnesses_are_preserved(self):
        shell = (ROOT / "scripts/ios27_shortcuts_simulator_qa.sh").read_text()
        start = shell.index('SIM_SELECTION="$(python3 - "$ARTIFACT_DIR" "$DEVICE_PROFILE"')
        end = shell.index('\n)"', start) + len('\n)"')
        self.assertEqual(hashlib.sha256(shell[start:end].encode()).hexdigest(), "d38ca1db43df85d5f3bf18bc3226caa898657ed350227707c05670462d0b45d5")
        self.assertIn('if [ "\u0024{QA_OWNED_SIMULATOR_SELECTION+x}" = x ]; then', shell)
        self.assertIn('--record "\u0024QA_OWNED_SIMULATOR_SELECTION"', shell)
        self.assertLess(shell.index("owned_simulator_selection.py"), shell.index('xcrun simctl boot "\u0024UDID"'))
        tree = ast.parse((ROOT / "tools/shortcuts-simulator-qa/qa_terminal_receipts.py").read_text())
        values = next(ast.literal_eval(n.value) for n in tree.body
                      if isinstance(n, ast.Assign) and any(isinstance(t, ast.Name) and t.id == "SOURCE_FILES" for t in n.targets))
        for path in ("tools/shortcuts-simulator-qa/owned_simulator_selection.py",
                     "tools/shortcuts-simulator-qa/test_owned_simulator_selection.py"):
            self.assertIn(path, values)


if __name__ == "__main__":
    unittest.main()
