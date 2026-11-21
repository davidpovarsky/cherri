"""Local observer/intake controls. These fabricated summaries are not Apple evidence."""
import base64
import copy
import json
from pathlib import Path
import tempfile
import unittest

import nested_list_probe as probe

ROOT = Path(__file__).resolve().parents[2]


def b64(value):
    return base64.b64encode(value.encode("utf-8")).decode("ascii")


def fields(fixture):
    value = dict(probe.GOALS[fixture])
    value.update(TEXT_TYPE64=b64("Text"), DICT_TYPE64=b64("Dictionary"))
    if fixture == probe.CONTROL:
        value.update(FIRST_TYPE64=b64("Dictionary"), LAST_TYPE64=b64("Dictionary"))
    else:
        # Deliberately arbitrary simulated types: matching counts must not imply
        # a real List type, even if the callback spells its type "List".
        value.update(FIRST_TYPE64=b64("List"), LAST_TYPE64=b64("List"))
    return value


def summary(fixture, values):
    return probe.PREFIXES[fixture] + ":".join(k + "=" + values[k] for k in probe.FIELDS[fixture])


class NestedListProbeControls(unittest.TestCase):
    def test_fixed_contracts_and_source_pins(self):
        for fixture in probe.FIXTURES:
            with self.subTest(fixture=fixture):
                value = probe.contract(ROOT, fixture)
                source = (ROOT / value["source_path"]).read_bytes()
                receipt = probe.verify_source(ROOT, fixture, source)
                self.assertFalse(receipt["nativeListTypeProved"])
                self.assertFalse(receipt["formalAcceptanceGranted"])
                self.assertTrue(source.endswith(b"return finalSummary\n"))
                with self.assertRaisesRegex(ValueError, "SOURCE_BYTES_DIFFER"):
                    probe.verify_source(ROOT, fixture, source + b"\n")

    def test_contract_rejects_goal_source_trace_and_promotion_changes(self):
        for fixture in probe.FIXTURES:
            value = probe.contract(ROOT, fixture)
            changes = [
                ("schema_version", True), ("source_sha256", "0" * 64),
                ("source_path", "elsewhere.cherri"), ("diagnostic_prefix", "SUCCESS:"),
                ("field_order", list(reversed(value["field_order"]))),
                ("trace_run_id", "other"), ("trace_label", "other"),
                ("native_list_type_proved", True), ("formal_requirements_proved", ["L01"]),
                ("runtime_status", "PASS"), ("goal_fields", {}),
            ]
            for key, bad in changes:
                with self.subTest(fixture=fixture, field=key), self.assertRaises(ValueError):
                    probe.validate_contract({**value, key: bad})
            with self.assertRaises(ValueError):
                probe.validate_contract({**value, "extra": True})

    def test_contract_byte_change_cannot_reconcile_itself(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            folder = root / "tools/shortcuts-simulator-qa/fixtures"
            folder.mkdir(parents=True)
            for fixture in probe.FIXTURES:
                value = probe.contract(ROOT, fixture)
                value["goal_fields"][next(iter(value["goal_fields"]))] = "999"
                (folder / (fixture + ".expected.json")).write_text(json.dumps(value), encoding="utf-8")
                with self.subTest(fixture=fixture), self.assertRaisesRegex(ValueError, "CONTRACT_BYTES"):
                    probe.contract(root, fixture)

    def test_duplicate_and_nonfinite_json_are_not_silently_normalized(self):
        for raw in (b'{"x":1,"x":2}', b'{"x":NaN}', b'{"x":Infinity}', b'{"x":-Infinity}'):
            with self.subTest(raw=raw), self.assertRaises(ValueError):
                probe._json(raw)
        self.assertFalse(probe._same({"x": False}, {"x": 0}))
        self.assertFalse(probe._same({"x": 0}, {"x": 0.0}))

    def test_complete_control_goals_are_separate_from_native_list_type(self):
        observed = probe.validate_summary(summary(probe.CONTROL, fields(probe.CONTROL)), probe.CONTROL)
        self.assertTrue(observed["controlGoalsMatched"])
        self.assertTrue(observed["dictionaryCarrierTypesMatched"])
        self.assertIsNone(observed["candidateShapeMatched"])
        self.assertFalse(observed["nativeListTypeProved"])
        self.assertFalse(observed["formalAcceptanceGranted"])

    def test_each_control_goal_contradiction_is_retained(self):
        original = fields(probe.CONTROL)
        for key in probe.GOALS[probe.CONTROL]:
            changed = dict(original)
            changed[key] = b64("WRONG") if key.endswith("64") else "999"
            with self.subTest(field=key):
                result = probe.validate_summary(summary(probe.CONTROL, changed), probe.CONTROL)
                self.assertFalse(result["controlGoalsMatched"])
                self.assertFalse(result["goalChecks"][key])
                self.assertEqual(result["actualFields"], changed)
                self.assertFalse(result["formalAcceptanceGranted"])

    def test_localized_type_witnesses_are_compared_without_an_english_oracle(self):
        value = fields(probe.CONTROL)
        value.update(TEXT_TYPE64=b64("טקסט"), DICT_TYPE64=b64("מילון"),
                     FIRST_TYPE64=b64("מילון"), LAST_TYPE64=b64("מילון"))
        result = probe.validate_summary(summary(probe.CONTROL, value), probe.CONTROL)
        self.assertTrue(result["controlGoalsMatched"])
        self.assertEqual(result["decodedBase64"]["DICT_TYPE64"], "מילון")
        for key in ("TEXT_TYPE64", "FIRST_TYPE64", "LAST_TYPE64"):
            changed = {**value, key: b64("מילון") if key == "TEXT_TYPE64" else b64("טקסט")}
            with self.subTest(key=key):
                self.assertFalse(probe.validate_summary(summary(probe.CONTROL, changed), probe.CONTROL)["controlGoalsMatched"])
        for key in ("TEXT_TYPE64", "DICT_TYPE64"):
            with self.subTest(empty=key):
                self.assertFalse(probe.validate_summary(summary(probe.CONTROL, {**value, key: ""}), probe.CONTROL)["controlGoalsMatched"])

    def test_candidate_shape_match_does_not_promote_any_runtime_type(self):
        for type_name in ("List", "Dictionary", "Text", "", "Unsupported type"):
            value = fields(probe.CANDIDATE)
            value["FIRST_TYPE64"] = value["LAST_TYPE64"] = b64(type_name)
            result = probe.validate_summary(summary(probe.CANDIDATE, value), probe.CANDIDATE)
            self.assertTrue(result["candidateShapeMatched"])
            self.assertIsNone(result["controlGoalsMatched"])
            self.assertIsNone(result["dictionaryCarrierTypesMatched"])
            self.assertFalse(result["nativeListTypeProved"])
            self.assertFalse(result["formalAcceptanceGranted"])

    def test_flattened_candidate_is_preserved_and_fails_the_shape_goal(self):
        value = fields(probe.CANDIDATE)
        value.update(FIRST_COUNT="1", LAST_COUNT="1", FIRST_LAST64=b64("INNER_A"),
                     LAST_FIRST64=b64("INNER_B"), LAST_LAST64=b64("INNER_B"),
                     FIRST_TYPE64=b64("Text"), LAST_TYPE64=b64("Text"))
        result = probe.validate_summary(summary(probe.CANDIDATE, value), probe.CANDIDATE)
        self.assertEqual(result["actualFields"], value)
        self.assertFalse(result["candidateShapeMatched"])
        self.assertFalse(result["nativeListTypeProved"])

    def test_dropped_candidate_rows_retain_unobserved_branch(self):
        value = fields(probe.CANDIDATE)
        value.update(OUTER_COUNT="0", OBSERVED="N", FIRST_COUNT="-1", LAST_COUNT="-1")
        for key in ("FIRST_FIRST64", "FIRST_LAST64", "LAST_FIRST64", "LAST_LAST64", "FIRST_TYPE64", "LAST_TYPE64"):
            value[key] = "UNOBSERVED"
        result = probe.validate_summary(summary(probe.CANDIDATE, value), probe.CANDIDATE)
        self.assertFalse(result["candidateShapeMatched"])
        self.assertEqual(result["actualFields"], value)
        self.assertNotIn("FIRST_TYPE64", result["decodedBase64"])

    def test_branch_inconsistent_observation_is_rejected(self):
        value = fields(probe.CANDIDATE)
        for key, bad in (("OBSERVED", "N"), ("OUTER_COUNT", "0"), ("FIRST_COUNT", "-1"),
                         ("FIRST_FIRST64", "UNOBSERVED"), ("FIRST_TYPE64", "UNOBSERVED"),
                         ("LAST_FIRST64", b64("UNREAD"))):
            with self.subTest(key=key), self.assertRaises(ValueError):
                probe.validate_summary(summary(probe.CANDIDATE, {**value, key: bad}), probe.CANDIDATE)

    def test_field_order_duplicates_missing_and_unreviewed_fields_fail(self):
        for fixture in probe.FIXTURES:
            good = summary(fixture, fields(fixture))
            tail = good[len(probe.PREFIXES[fixture]):].split(":")
            bad = [
                good + ":EXTRA=1", good + ":" + tail[0],
                probe.PREFIXES[fixture] + ":".join(tail[:-1]),
                probe.PREFIXES[fixture] + ":".join(reversed(tail)),
                good.replace(probe.PREFIXES[fixture], "UNREVIEWED:", 1),
                good + "\nforged trace", good + "\r", "A" * 8193,
            ]
            for changed in bad:
                with self.subTest(fixture=fixture, changed=changed[:30]), self.assertRaises(ValueError):
                    probe.validate_summary(changed, fixture)

    def test_base64_rejects_noncanonical_bytes_utf8_and_size(self):
        value = fields(probe.CONTROL)
        invalid = ("VGV4dA=", "VGV4dA===", "VGV4dA==\x00", "/w==", b64("X" * 129), "*", "VGV4dA== ")
        for encoded in invalid:
            with self.subTest(encoded=encoded[:30]), self.assertRaises(ValueError):
                probe.validate_summary(summary(probe.CONTROL, {**value, "TEXT_TYPE64": encoded}), probe.CONTROL)
        for encoded in (b64("X" * 257), "TEVGVA==\t"):
            with self.subTest(item=encoded[:30]), self.assertRaises(ValueError):
                probe.validate_summary(summary(probe.CONTROL, {**value, "FLAT_FIRST64": encoded}), probe.CONTROL)

    def test_count_strings_have_closed_finite_decimal_grammar(self):
        value = fields(probe.CONTROL)
        for bad in ("", "-1", "+2", "2.0", "02", "NaN", "Infinity", "10000", "2e0", " 2", "2 "):
            with self.subTest(bad=bad), self.assertRaises(ValueError):
                probe.validate_summary(summary(probe.CONTROL, {**value, "FLAT_COUNT": bad}), probe.CONTROL)

    def test_receipt_inventories_never_claim_native_or_formal_success(self):
        for fixture, count, mutations in ((probe.CONTROL, 56, 28), (probe.CANDIDATE, 78, 33)):
            with self.subTest(fixture=fixture):
                graph = probe.graph_receipt(fixture)
                negative = probe.mutation_receipt(fixture)
                self.assertEqual(graph["nativeActionCount"], count)
                self.assertEqual(len(negative["rejected"]), mutations)
                self.assertEqual(len(set(negative["rejected"])), mutations)
                for result in (graph, negative):
                    self.assertEqual(result["appleRuntime"], "NOT_RUN")
                    self.assertFalse(result["nativeListTypeProved"])
                    self.assertFalse(result["formalAcceptanceGranted"])
        for method in (probe.graph_receipt, probe.mutation_receipt):
            with self.assertRaises(ValueError):
                method("qa-unreviewed-v2")


if __name__ == "__main__":
    unittest.main(verbosity=2)
