"""Counterfactual checks against complete retained native originals; no native replay."""
from pathlib import Path
import copy
import json
import os
import sys
import unittest

sys.path.insert(0, str(Path(__file__).resolve().parent))
import native_workflowkit_stored_representation_observation as observer

ROOT = Path(os.environ["CHERRI_WORKFLOWKIT_ORIGINAL_ROOT"])


class StoredRepresentationControls(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.source = (ROOT / "08/source.original.cherri").read_bytes()
        cls.plist = (ROOT / "08/unsigned.original.plist").read_bytes()
        cls.good = json.loads((ROOT / "08/pilot.original.json").read_bytes())
        cls.old = json.loads((ROOT / "07/pilot.original.json").read_bytes())

    def validate(self, value):
        return observer.validate_nothing_text_nothing(self.source, self.plist, value)

    def completed(self, value):
        return [r for r in value["events"] if r["kind"] == "action-completed"]

    def text_object(self, value):
        item = self.completed(value)[1]["output"]["items"][0]
        return item["representations"]["entries"][0]["value"]["items"][0]["object"]

    def test_actual_unobserved_then_stored_object_observed(self):
        with self.assertRaisesRegex(ValueError, "^STORED_STRING_DECLARATION$"):
            self.validate(self.old)
        result = self.validate(self.good)
        self.assertEqual(result["actualIntermediateText"], "M02_AFTER_NOTHING")
        self.assertFalse(result["formalAcceptanceGranted"])
        self.assertFalse(result["willRunActionInputsObserved"])

    def test_actual_value_and_type_cannot_be_replaced_by_declared_string_type(self):
        for mode in ("wrong-text", "number-object", "unobserved-object", "wrong-native-class"):
            with self.subTest(mode=mode):
                value = copy.deepcopy(self.good)
                target = self.text_object(value)
                if mode == "wrong-text":
                    target["value"] = "M02_BEFORE_NOTHING"
                elif mode == "wrong-native-class":
                    target["nativeClass"] = "NSDecimalNumber"
                elif mode == "number-object":
                    target.clear()
                    target.update(present=True, nativeClass="NSDecimalNumber", kind="NSNUMBER",
                                  cfTypeId=22, cfBooleanTypeId=21, value=37)
                else:
                    target.clear()
                    target.update(present=True, nativeClass="WFObjectRepresentation", kind="UNOBSERVED")
                with self.assertRaises(ValueError):
                    self.validate(value)

    def test_empty_collection_and_absent_terminal_remain_distinct(self):
        for mode in ("empty-to-nil", "nil-to-empty"):
            with self.subTest(mode=mode):
                value = copy.deepcopy(self.good)
                if mode == "empty-to-nil":
                    self.completed(value)[0]["output"] = copy.deepcopy(observer.NIL)
                else:
                    value["events"][-1]["output"] = copy.deepcopy(self.completed(value)[0]["output"])
                with self.assertRaises(ValueError):
                    self.validate(value)

    def test_original_action_uuid_order_and_completion_are_required(self):
        for mode in ("uuid", "order", "missing"):
            with self.subTest(mode=mode):
                value = copy.deepcopy(self.good)
                rows = self.completed(value)
                if mode == "uuid":
                    rows[1]["actualUUID"] = rows[0]["actualUUID"]
                elif mode == "order":
                    rows[1]["index"], rows[2]["index"] = rows[2]["index"], rows[1]["index"]
                else:
                    value["events"].remove(rows[1])
                    for i, row in enumerate(value["events"]):
                        row["sequence"] = i
                with self.assertRaises(ValueError):
                    self.validate(value)

    def test_source_or_full_native_bytes_cannot_be_substituted(self):
        for mode in ("source", "plist"):
            with self.subTest(mode=mode):
                with self.assertRaises(ValueError):
                    observer.validate_nothing_text_nothing(
                        self.source + (b"\n" if mode == "source" else b""),
                        self.plist + (b"\n" if mode == "plist" else b""), self.good)

    def test_engine_error_cancellation_and_expired_terminal_reject(self):
        for mode in ("error", "cancelled", "expired"):
            with self.subTest(mode=mode):
                value = copy.deepcopy(self.good)
                final = value["events"][-1]
                if mode == "error":
                    final["error"] = {"nativeClass": "NSError", "domain": "injected", "code": 1}
                elif mode == "cancelled":
                    final["cancelled"] = True
                else:
                    final["uptime"] += 9
                with self.assertRaises(ValueError):
                    self.validate(value)

    def test_ambiguous_presence_cardinality_or_scope_reject(self):
        for mode in ("presence-number", "nil-presence-number", "cardinality-bool", "ios-claim"):
            with self.subTest(mode=mode):
                value = copy.deepcopy(self.good)
                if mode == "presence-number":
                    self.text_object(value)["present"] = 1
                elif mode == "nil-presence-number":
                    value["events"][-1]["output"]["present"] = 0
                elif mode == "cardinality-bool":
                    self.completed(value)[1]["output"]["cardinality"] = True
                else:
                    value["iOSDeviceExercised"] = True
                with self.assertRaises(ValueError):
                    self.validate(value)


if __name__ == "__main__":
    observed = {"started": [], "passed": [], "subtests": []}

    class ObservedResult(unittest.TextTestResult):
        def startTest(self, test):
            observed["started"].append(test.id())
            super().startTest(test)

        def addSuccess(self, test):
            observed["passed"].append(test.id())
            super().addSuccess(test)

        def addSubTest(self, test, subtest, error):
            observed["subtests"].append({"test": test.id(), "id": str(subtest),
                                         "status": "PASS" if error is None else "FAIL"})
            super().addSubTest(test, subtest, error)

    suite = unittest.defaultTestLoader.loadTestsFromTestCase(StoredRepresentationControls)
    result = unittest.TextTestRunner(verbosity=2, resultclass=ObservedResult).run(suite)
    observed.update(testsRun=result.testsRun, failures=len(result.failures), errors=len(result.errors),
                    skipped=len(result.skipped), nativeExecution=False)
    output = Path(os.environ["CHERRI_WORKFLOWKIT_CONTROL_OUT"])
    with output.open("x") as stream:
        json.dump(observed, stream, sort_keys=True, indent=2)
        stream.write("\n")
    raise SystemExit(0 if result.wasSuccessful() else 1)
