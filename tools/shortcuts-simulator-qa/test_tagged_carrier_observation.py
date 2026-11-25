import base64
import unittest
import tagged_carrier_observation as p

def b64(text):
    return base64.b64encode(text.encode()).decode()

def values():
    result = dict(p.GOALS)
    result.update(ORIGINAL_LIST_TYPE64=b64("Number"), DECODED_LIST_TYPE64=b64("Number"),
                  TEXT_TYPE64=b64("Text"), ORIGINAL_MAP_TYPE64=b64("Dictionary"),
                  DECODED_MAP_TYPE64=b64("Dictionary"))
    return result

def line(data):
    return p.PREFIX + ":".join(k + "=" + data[k] for k in p.FIELDS)

class ObservationControls(unittest.TestCase):
    def test_exact_public_observation(self):
        value = p.validate_summary(line(values()))
        self.assertTrue(value["semanticGoalsMatched"])
        self.assertFalse(value["formalAcceptanceGranted"])
        self.assertFalse(value["sourceAuthenticated"])
        self.assertFalse(value["appleExecutionAuthenticated"])

    def test_each_literal_goal_is_causal(self):
        for key in p.GOALS:
            with self.subTest(key=key):
                v = values()
                if key == "FIRST_PRESENT":
                    v.update(FIRST_PRESENT="N", FIRST_COUNT="-1", FIRST_FIRST64="UNOBSERVED",
                             FIRST_LAST64="UNOBSERVED", DECODED_LIST_TYPE64="UNOBSERVED")
                elif key == "EMPTY_PRESENT":
                    v.update(EMPTY_PRESENT="N", EMPTY_COUNT="-1")
                elif key == "MAP_PRESENT":
                    v.update(MAP_PRESENT="N", DECODED_MAP_TYPE64="UNOBSERVED", MAP_KEYS="-1",
                             MAP_ALPHA64="UNOBSERVED", MAP_BETA64="UNOBSERVED")
                elif key == "MAP_ABSENT":
                    v[key] = "N"
                elif key.endswith("64"):
                    v[key] = b64("WRONG")
                else:
                    v[key] = "3"
                self.assertFalse(p.validate_summary(line(v))["semanticGoalsMatched"])

    def test_list_type_cannot_be_rendered_text(self):
        v=values();v["DECODED_LIST_TYPE64"]=v["TEXT_TYPE64"]
        self.assertFalse(p.validate_summary(line(v))["semanticGoalsMatched"])

    def test_map_type_mismatch_is_retained(self):
        v=values();v.update(DECODED_MAP_TYPE64=v["TEXT_TYPE64"],MAP_KEYS="-1",
                           MAP_ALPHA64="UNOBSERVED",MAP_BETA64="UNOBSERVED")
        self.assertFalse(p.validate_summary(line(v))["semanticGoalsMatched"])

    def test_self_agreeing_text_witnesses_fail(self):
        for kind in ("LIST","MAP"):
            with self.subTest(kind=kind):
                v=values()
                v["ORIGINAL_"+kind+"_TYPE64"]=v["TEXT_TYPE64"]
                v["DECODED_"+kind+"_TYPE64"]=v["TEXT_TYPE64"]
                self.assertFalse(p.validate_summary(line(v))["semanticGoalsMatched"])

    def test_absent_empty_list_is_not_present_empty(self):
        v=values();v.update(EMPTY_PRESENT="N",EMPTY_COUNT="-1")
        r=p.validate_summary(line(v))
        self.assertFalse(r["goalChecks"]["EMPTY_PRESENT"])
        self.assertFalse(r["goalChecks"]["EMPTY_COUNT"])

    def test_bad_transport_is_rejected(self):
        good=line(values())
        cases={
            "wrong_prefix":good.replace(p.PREFIX,"CHERRI_NESTED_LIST_CONTROL_V1:",1),
            "extra":good+":EXTRA=1", "missing":good.rsplit(":",1)[0],
            "duplicate":good.replace("FIRST_PRESENT=Y","OUTER_COUNT=2"),
            "reordered":good.replace("OUTER_COUNT=2:FIRST_PRESENT=Y","FIRST_PRESENT=Y:OUTER_COUNT=2"),
            "newline":good+"\n", "non_ascii":good+"א",
        }
        for name,replacement in (("bad_base64","!"),("noncanonical","MTE"),("invalid_utf8","/w==")):
            cases[name]=good.replace("FIRST_FIRST64=MTE=","FIRST_FIRST64="+replacement)
        for name,changes in {
            "missing_witness":{"TEXT_TYPE64":"UNOBSERVED"},
            "empty_type":{"TEXT_TYPE64":""},
            "bad_flag":{"FIRST_PRESENT":"TRUE"},
            "negative_outer":{"OUTER_COUNT":"-1"},
            "leading_zero":{"MAP_KEYS":"02"},
            "absent_has_values":{"FIRST_PRESENT":"N"},
            "present_missing_values":{"FIRST_FIRST64":"UNOBSERVED"},
            "wrong_map_reads_keys":{"DECODED_MAP_TYPE64":b64("Text")},
            "missing_map_keys":{"MAP_KEYS":"-1"},
            "empty_present_without_slot":{"OUTER_COUNT":"1"},
        }.items():
            v=values();v.update(changes);cases[name]=line(v)
        for name,bad in cases.items():
            with self.subTest(name=name):
                with self.assertRaises(ValueError):
                    p.validate_summary(bad)


    def test_all_five_type_witnesses_require_nonempty_canonical_utf8(self):
        for key in p.TYPE_WITNESSES:
            for bad in ("", "!", "/w==", "VGV4dA", "VGV4dA==="):
                with self.subTest(key=key, bad=bad):
                    v=values();v[key]=bad
                    with self.assertRaises(ValueError):
                        p.validate_summary(line(v))

    def test_unobserved_decoded_type_is_never_a_type_witness(self):
        changes=(
            dict(FIRST_PRESENT="N",FIRST_COUNT="-1",FIRST_FIRST64="UNOBSERVED",
                 FIRST_LAST64="UNOBSERVED",DECODED_LIST_TYPE64="UNOBSERVED"),
            dict(MAP_PRESENT="N",DECODED_MAP_TYPE64="UNOBSERVED",MAP_KEYS="-1",
                 MAP_ALPHA64="UNOBSERVED",MAP_BETA64="UNOBSERVED"),
        )
        for change in changes:
            with self.subTest(change=change):
                v=values();v.update(change)
                r=p.validate_summary(line(v))
                self.assertFalse(r["typeWitnessesComplete"])
                self.assertFalse(any(r["typeRelations"].values()))
                self.assertFalse(r["semanticGoalsMatched"])

    def test_contract_exactly_binds_parser_prefix_fields_and_source(self):
        import pathlib,json,hashlib
        directory=pathlib.Path(__file__).parent
        c=json.loads((directory/"fixtures"/"qa-tagged-value-carrier-v2.expected.json").read_bytes())
        self.assertEqual(c["diagnostic_prefix"],p.PREFIX)
        self.assertEqual(c["field_order"],list(p.FIELDS))
        self.assertEqual(c["literal_goals"],p.GOALS)
        self.assertEqual(c["type_witness_fields"],list(p.TYPE_WITNESSES))
        self.assertEqual(c["source_sha256"],hashlib.sha256(
            (directory/"fixtures"/"qa-tagged-value-carrier-v2.cherri").read_bytes()).hexdigest())

if __name__ == "__main__":
    unittest.main()
