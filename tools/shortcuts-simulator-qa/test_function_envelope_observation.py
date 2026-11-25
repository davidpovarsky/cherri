import base64
import unittest
import function_envelope_observation as p


def b64(text):
    return base64.b64encode(text.encode("utf-8")).decode("ascii")


# Independently declared public source rows, not generated from parser GOALS.
GOOD = """CHERRI_FUNCTION_ENVELOPE_PRESENCE_V1:
CASE=probeZero;KEY_COUNT=5;CALLER_KEY=1;ARGUMENTS_KEY=1;ARG_FLAG_KEY=0;ARG_VALUE_KEY=0;ARG_FLAG=UNOBSERVED;CALLER_FIELD_COUNT=0;CALLER_SAVED_COUNT=0;CALLER_FIELD_TYPE64=UNOBSERVED;CALLER_SAVED_TYPE64=UNOBSERVED;PARAMETER=NO_PARAMETER
CASE=probeDefault;KEY_COUNT=6;CALLER_KEY=1;ARGUMENTS_KEY=1;ARG_FLAG_KEY=1;ARG_VALUE_KEY=0;ARG_FLAG=0;CALLER_FIELD_COUNT=0;CALLER_SAVED_COUNT=0;CALLER_FIELD_TYPE64=UNOBSERVED;CALLER_SAVED_TYPE64=UNOBSERVED;PARAMETER=REVGQVVMVA==
CASE=probeOptional;KEY_COUNT=7;CALLER_KEY=1;ARGUMENTS_KEY=1;ARG_FLAG_KEY=1;ARG_VALUE_KEY=1;ARG_FLAG=1;CALLER_FIELD_COUNT=0;CALLER_SAVED_COUNT=0;CALLER_FIELD_TYPE64=UNOBSERVED;CALLER_SAVED_TYPE64=UNOBSERVED;PARAMETER=NONE"""


def mutate(case, **changes):
    lines = GOOD.split("\n")
    index = ("probeZero", "probeDefault", "probeOptional").index(case) + 1
    parts = lines[index].split(";")
    seen = set()
    for i, part in enumerate(parts):
        key, _ = part.split("=", 1)
        if key in changes:
            parts[i] = key + "=" + changes[key]
            seen.add(key)
    assert seen == set(changes)
    lines[index] = ";".join(parts)
    return "\n".join(lines)


class FunctionEnvelopeObservationControls(unittest.TestCase):
    def test_exact_three_rows_remain_unauthenticated(self):
        result = p.validate_summary(GOOD)
        self.assertTrue(result["declaredNoInputGoalsMatched"])
        self.assertEqual(list(result["caseObservations"]),
                         ["probeZero", "probeDefault", "probeOptional"])
        for flag in ("launchInputAuthenticated", "sourceAuthenticated",
                     "appleExecutionAuthenticated", "formalAcceptanceGranted"):
            self.assertIs(result[flag], False)
        for case in result["caseObservations"].values():
            self.assertFalse(case["observedCallerTypesBothPresent"])
            self.assertFalse(case["observedCallerTypesEqual"])

    def test_all_thirty_three_declared_goals_are_causal(self):
        for case, goals in p.GOALS.items():
            for field in goals:
                with self.subTest(case=case, field=field):
                    if field == "CALLER_KEY":
                        changes = dict(CALLER_KEY="0", CALLER_FIELD_COUNT="UNOBSERVED",
                                       CALLER_FIELD_TYPE64="UNOBSERVED")
                    elif field in ("CALLER_FIELD_COUNT", "CALLER_FIELD_TYPE64"):
                        changes = dict(CALLER_FIELD_COUNT="1", CALLER_FIELD_TYPE64=b64("Text"))
                    elif field in ("CALLER_SAVED_COUNT", "CALLER_SAVED_TYPE64"):
                        changes = dict(CALLER_SAVED_COUNT="1", CALLER_SAVED_TYPE64=b64("Text"))
                    elif field == "ARG_FLAG_KEY":
                        if goals[field] == "0":
                            changes = dict(ARG_FLAG_KEY="1", ARG_FLAG="0")
                        else:
                            changes = dict(ARG_FLAG_KEY="0", ARG_FLAG="UNOBSERVED")
                    elif field == "ARG_FLAG":
                        changes = dict(ARG_FLAG_KEY="1", ARG_FLAG="WRONG")
                    elif field == "PARAMETER":
                        if case == "probeZero":
                            with self.assertRaisesRegex(ValueError, "ZERO_PARAMETER"):
                                p.validate_summary(mutate(case, PARAMETER="WRONG"))
                            continue
                        changes = dict(PARAMETER=b64("WRONG"))
                    elif field == "KEY_COUNT":
                        changes = {field: "4"}
                    else:
                        changes = {field: "0" if goals[field] == "1" else "1"}
                    result = p.validate_summary(mutate(case, **changes))
                    self.assertFalse(result["declaredNoInputGoalsMatched"])
                    self.assertFalse(result["caseNoInputGoalsMatched"][case])

    def test_absent_caller_is_preserved_not_rewritten_to_empty(self):
        for case in p.CASES:
            with self.subTest(case=case):
                result = p.validate_summary(mutate(
                    case, CALLER_KEY="0", CALLER_FIELD_COUNT="UNOBSERVED",
                    CALLER_FIELD_TYPE64="UNOBSERVED"))
                actual = result["caseObservations"][case]["actualFields"]
                self.assertEqual(actual["CALLER_FIELD_COUNT"], "UNOBSERVED")
                self.assertFalse(result["declaredNoInputGoalsMatched"])

    def test_supplied_text_is_observed_but_not_a_no_input_pass(self):
        result = p.validate_summary(mutate(
            "probeZero", CALLER_FIELD_COUNT="1", CALLER_SAVED_COUNT="1",
            CALLER_FIELD_TYPE64=b64("Text"), CALLER_SAVED_TYPE64=b64("Text")))
        case = result["caseObservations"]["probeZero"]
        self.assertTrue(case["observedCallerTypesBothPresent"])
        self.assertTrue(case["observedCallerTypesEqual"])
        self.assertFalse(result["declaredNoInputGoalsMatched"])
        self.assertFalse(result["launchInputAuthenticated"])

    def test_mismatched_real_types_are_retained(self):
        result = p.validate_summary(mutate(
            "probeZero", CALLER_FIELD_COUNT="1", CALLER_SAVED_COUNT="1",
            CALLER_FIELD_TYPE64=b64("Dictionary"), CALLER_SAVED_TYPE64=b64("Text")))
        self.assertFalse(result["caseObservations"]["probeZero"]["observedCallerTypesEqual"])

    def test_empty_default_does_not_become_omission(self):
        result = p.validate_summary(mutate("probeDefault", PARAMETER=""))
        self.assertEqual(result["caseObservations"]["probeDefault"]["decodedBase64"]["PARAMETER"], "")
        self.assertFalse(result["declaredNoInputGoalsMatched"])

    def test_explicit_optional_value_does_not_become_none(self):
        for value in ("", "DEFAULT", "false", "0"):
            with self.subTest(value=value):
                result = p.validate_summary(mutate("probeOptional", PARAMETER=b64(value)))
                self.assertEqual(result["caseObservations"]["probeOptional"]["decodedBase64"]["PARAMETER"], value)
                self.assertFalse(result["declaredNoInputGoalsMatched"])

    def test_wrong_literal_flag_is_a_negative_observation(self):
        for value in ("", "2", "false", "UNOBSERVED"):
            with self.subTest(value=value):
                result = p.validate_summary(mutate("probeDefault", ARG_FLAG=value))
                self.assertFalse(result["declaredNoInputGoalsMatched"])

    def test_structure_and_row_order_are_closed(self):
        lines = GOOD.split("\n")
        bad = {
            "prefix": GOOD.replace("_V1:", "_V2:", 1),
            "suffix": GOOD + "\n",
            "crlf": GOOD.replace("\n", "\r\n"),
            "missing_case": "\n".join(lines[:-1]),
            "extra_case": GOOD + "\n" + lines[1],
            "reordered_case": "\n".join([lines[0], lines[2], lines[1], lines[3]]),
            "duplicate_case": "\n".join([lines[0], lines[1], lines[1], lines[3]]),
            "extra_field": GOOD.replace(";KEY_COUNT=5", ";EXTRA=1;KEY_COUNT=5", 1),
            "duplicate_field": GOOD.replace(";KEY_COUNT=5", ";CASE=probeZero", 1),
            "reordered_field": GOOD.replace("KEY_COUNT=5;CALLER_KEY=1", "CALLER_KEY=1;KEY_COUNT=5", 1),
            "missing_field": GOOD.replace(";KEY_COUNT=5", "", 1),
            "unknown_case": GOOD.replace("probeZero", "foreign", 1),
            "non_ascii": GOOD + "א",
            "oversized": GOOD + "x" * 4096,
        }
        for name, value in bad.items():
            with self.subTest(name=name), self.assertRaises(ValueError):
                p.validate_summary(value)
        for value in (None, b"ASCII", [], 4):
            with self.subTest(value=repr(value)), self.assertRaises(ValueError):
                p.validate_summary(value)

    def test_canonical_counts_and_flags_are_required(self):
        for field in ("KEY_COUNT", "CALLER_FIELD_COUNT", "CALLER_SAVED_COUNT"):
            for value in ("-1", "+1", "01", "1.0", "1e0", "10000", ""):
                with self.subTest(field=field, value=value), self.assertRaises(ValueError):
                    p.validate_summary(mutate("probeZero", **{field: value}))
        for field in p.FLAGS:
            for value in ("Y", "true", "2", "01", ""):
                with self.subTest(field=field, value=value), self.assertRaises(ValueError):
                    p.validate_summary(mutate("probeZero", **{field: value}))

    def test_fixed_source_absence_guards_are_checked(self):
        cases = [
            dict(CALLER_KEY="0"), dict(CALLER_FIELD_TYPE64=b64("Text")),
            dict(CALLER_FIELD_COUNT="1"), dict(CALLER_SAVED_TYPE64=b64("Text")),
            dict(CALLER_SAVED_COUNT="1"), dict(ARG_FLAG="0"),
        ]
        for changes in cases:
            with self.subTest(changes=changes), self.assertRaises(ValueError):
                p.validate_summary(mutate("probeZero", **changes))

    def test_base64_is_canonical_bounded_and_utf8(self):
        for value in ("!", "REVGQVVMVA", "/w==", "Zh==", b64("x" * 257)):
            with self.subTest(value=value[:20]), self.assertRaises(ValueError):
                p.validate_summary(mutate("probeDefault", PARAMETER=value))
        for value in ("", "UNOBSERVED", b64("UNOBSERVED"), "/w==", "VA===", b64("x" * 129)):
            with self.subTest(value=value[:20]), self.assertRaises(ValueError):
                p.validate_summary(mutate("probeZero", CALLER_FIELD_COUNT="1", CALLER_FIELD_TYPE64=value))

    def test_flag_text_cannot_escape_its_bounded_field(self):
        for value in ("x" * 65, "\t", "\x00", "0;EXTRA=1"):
            with self.subTest(value=repr(value)), self.assertRaises(ValueError):
                p.validate_summary(mutate("probeDefault", ARG_FLAG=value))

    def test_no_input_claim_requires_each_original_case(self):
        for case in p.CASES:
            with self.subTest(case=case):
                result = p.validate_summary(mutate(case, ARGUMENTS_KEY="0"))
                self.assertFalse(result["declaredNoInputGoalsMatched"])
                self.assertEqual(sum(result["caseNoInputGoalsMatched"].values()), 2)


if __name__ == "__main__":
    unittest.main()
