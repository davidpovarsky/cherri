import copy
import hashlib
import json
import pathlib
import plistlib
import unittest
import uuid
import xml.etree.ElementTree as ET

import function_envelope_graph as g


ROOT = pathlib.Path(__file__).resolve().parent
SOURCE = ROOT / "fixtures/qa-function-envelope-presence-v2.cherri"
CONTRACT = ROOT / "fixtures/qa-function-envelope-presence-v2.expected.json"
NATIVE = ROOT / "testdata/function-envelope-presence-compiled.original.plist"
NATIVE_SHA256 = "54889840e546acc60ffe05c8399e1d4bd54f0dc846ec3fd520b4fffc0aa6c7d0"


def params(actions, index):
    return actions[index].get("WFWorkflowActionParameters", {})


def dictionary_field(actions, index, key):
    rows = params(actions, index)["WFItems"]["Value"]["WFDictionaryFieldValueItems"]
    matches = [row for row in rows if row["WFKey"]["Value"]["string"] == key]
    assert len(matches) == 1
    return matches[0]


def edge(actions, index):
    return {"WFSerializationType": "WFTextTokenAttachment",
            "Value": {"Type": "ActionOutput", "OutputUUID": params(actions, index)["UUID"]}}


class FunctionPresenceGraphControls(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.native = NATIVE.read_bytes()
        assert hashlib.sha256(cls.native).hexdigest() == NATIVE_SHA256
        cls.actions = plistlib.loads(cls.native)["WFWorkflowActions"]
        cls.expected = g.expected_program()
        assert len(cls.actions) == len(cls.expected.actions) == 379

    def rejected(self, mutate):
        actions = copy.deepcopy(self.actions)
        mutate(actions)
        self.assertTrue(json.dumps(actions, sort_keys=True, allow_nan=False) !=\
                        json.dumps(self.actions, sort_keys=True, allow_nan=False),\
                        "mutation must alter typed original bytes")
        with self.assertRaises(ValueError):
            g.validate_actions(actions)

    def test_original_full_graph_has_no_runtime_authority(self):
        result = g.validate_compiled(SOURCE.read_bytes(), CONTRACT.read_bytes(), self.native)
        self.assertEqual(result["actionCount"], 379)
        self.assertEqual(result["exactKeyComparisons"], 12)
        self.assertEqual(result["functionCases"], 3)
        self.assertTrue(result["sourceAuthenticated"])
        self.assertTrue(result["knownExistingIncompleteDispatcherPreserved"])
        for key in ("completeEnvelopeAdmissionVerified", "noInputLaunchVerified",
                    "appleExecutionAuthenticated", "formalAcceptanceGranted"):
            self.assertIs(result[key], False)

    def test_every_action_identity_is_causal(self):
        for index in range(379):
            with self.subTest(action=index):
                self.rejected(lambda actions, i=index: actions[i].__setitem__(
                    "WFWorkflowActionIdentifier", "is.workflow.actions.unreviewed"))

    def test_missing_extra_and_reordered_actions_fail(self):
        self.rejected(lambda a: a.pop())
        self.rejected(lambda a: a.append(copy.deepcopy(a[-1])))
        self.rejected(lambda a: a.__setitem__(slice(100, 102), list(reversed(a[100:102]))))
        self.rejected(lambda a: a[0].__setitem__("UnreviewedTopLevelKey", True))
        self.rejected(lambda a: params(a, 0).__setitem__("UnreviewedParameter", True))

    def test_all_key_comparisons_preserve_exact_operands_and_polarity(self):
        for c in self.expected.comparisons:
            changes = {
                "quote_find": lambda a: params(a, c["quote"]).__setitem__("WFReplaceTextFind", ""),
                "quote_escape": lambda a: params(a, c["quote"]).__setitem__("WFReplaceTextReplace", ""),
                "quote_regex": lambda a: params(a, c["quote"]).__setitem__("WFReplaceTextRegularExpression", True),
                "case_insensitive": lambda a: params(a, c["match"]).__setitem__("WFMatchTextCaseSensitive", False),
                "unanchored": lambda a: params(a, c["match"])["WFMatchTextPattern"]["Value"].__setitem__("string", "\ufffc"),
                "wrong_left": lambda a: params(a, c["start"])["WFTextActionText"]["Value"]["attachmentsByRange"]["{1, 1}"].__setitem__("OutputUUID", params(a, 0)["UUID"]),
                "wrong_right": lambda a: params(a, c["start"] + 1).__setitem__("WFTextActionText", "~OTHER_KEY"),
                "wrong_count_edge": lambda a: params(a, c["count"]).__setitem__("Input", edge(a, c["start"])),
                "count_two": lambda a: params(a, c["count"] + 1).__setitem__("WFNumberValue", 2.0),
                "wrong_true": lambda a: params(a, c["count"] + 2).__setitem__("WFNumberActionNumber", 0.0),
                "boolean_instead_of_number": lambda a: params(a, c["count"] + 2).__setitem__("WFNumberActionNumber", True),
                "outer_polarity": lambda a: params(a, c["guard"]).__setitem__("WFCondition", 5),
            }
            for name, mutate in changes.items():
                with self.subTest(key=c["right"], guard=c["guard"], mutation=name):
                    self.rejected(mutate)

    def test_raw_and_restored_caller_observations_are_separate(self):
        for case in ("probeZero", "probeDefault", "probeOptional"):
            r = self.expected.roles[case]
            changes = {
                "raw_uses_restored": lambda a: params(a, r["envelope"]).__setitem__("WFInput", g.variable("_cherri_caller_input")),
                "key_source": lambda a: params(a, r["keys"]).__setitem__("WFInput", edge(a, 4)),
                "wrong_raw_key": lambda a: params(a, r["rawCaller"]).__setitem__("WFDictionaryKey", "arguments"),
                "raw_without_presence": lambda a: params(a, r["callerGuard"]).__setitem__("WFCondition", 5),
                "raw_count_from_keys": lambda a: params(a, r["rawCount"]).__setitem__("Input", edge(a, r["keys"])),
                "raw_type_from_keys": lambda a: params(a, r["rawType"]).__setitem__("WFInput", edge(a, r["keys"])),
                "saved_count_raw_extension": lambda a: params(a, r["savedCount"]).__setitem__("Input", g.attachment({"Type": "ExtensionInput"})),
                "saved_type_raw_extension": lambda a: params(a, r["savedType"]).__setitem__("WFInput", g.attachment({"Type": "ExtensionInput"})),
                "key_count_wrong_source": lambda a: params(a, r["keyCount"]).__setitem__("Input", edge(a, r["rawCount"])),
                "text_encoded_with_linebreaks": lambda a: params(a, r["rawType"] + 1).__setitem__("WFBase64LineBreakMode", "64 Characters"),
            }
            for name, mutate in changes.items():
                with self.subTest(case=case, mutation=name):
                    self.rejected(mutate)

    def test_row_fields_are_bound_to_their_actual_producers(self):
        for case in ("probeZero", "probeDefault", "probeOptional"):
            r = self.expected.roles[case]
            references = params(self.actions, r["semantic"])["WFTextActionText"]["Value"]["attachmentsByRange"]
            for position in references:
                with self.subTest(case=case, position=position):
                    self.rejected(lambda a, position=position: params(a, r["semantic"])[
                        "WFTextActionText"]["Value"]["attachmentsByRange"].__setitem__(
                            position, {"Type": "Variable", "VariableName": "FOREIGN"}))

    def test_legal_sender_presence_and_self_target_are_not_truthiness(self):
        for case in ("probeZero", "probeDefault", "probeOptional"):
            sender = self.expected.roles["sender_" + case]
            payload, called = sender["payload"], sender["call"]
            changes = {
                "caller_alias": lambda a: dictionary_field(a, payload, "_cherri_caller_input").__setitem__("WFValue", g.variable("_cherri_is_fn")),
                "magic": lambda a: dictionary_field(a, payload, "_cherri_magic").__setitem__("WFValue", g.text_state("WRONG")),
                "marker": lambda a: dictionary_field(a, payload, "cherri_functions").__setitem__("WFValue", g.text_state("0")),
                "function": lambda a: dictionary_field(a, payload, "function").__setitem__("WFValue", g.text_state("unknown")),
                "caller_omitted": lambda a: params(a, payload)["WFItems"]["Value"]["WFDictionaryFieldValueItems"].pop(0),
                "target_not_self": lambda a: params(a, called)["WFWorkflow"].__setitem__("isSelf", False),
                "target_name": lambda a: params(a, called).__setitem__("WFWorkflowName", "Another"),
                "selector_name": lambda a: params(a, called)["WFWorkflow"].__setitem__("workflowName", "Another"),
                "wrong_payload_edge": lambda a: params(a, called).__setitem__("WFInput", edge(a, 0)),
            }
            if case == "probeZero":
                changes["zero_mirror_has_value"] = lambda a: dictionary_field(a, payload, "arguments")["WFValue"].__setitem__("Value", [])
            else:
                changes["presence_flag"] = lambda a: dictionary_field(a, payload, "has_arg_0").__setitem__("WFValue", g.text_state("1" if case == "probeDefault" else "0"))
            for name, mutate in changes.items():
                with self.subTest(case=case, mutation=name):
                    self.rejected(mutate)
        carrier = self.expected.roles["suppliedNone"]
        self.rejected(lambda a: dictionary_field(a, carrier, "has_value").__setitem__("WFValue", g.text_state("1")))
        self.rejected(lambda a: dictionary_field(a, carrier, "_cherri_optional_argument_v1").__setitem__("WFValue", g.text_state("WRONG")))
        self.rejected(lambda a: params(a, carrier)["WFItems"]["Value"]["WFDictionaryFieldValueItems"].append(
            {"WFKey": g.text_state("value"), "WFItemType": 0, "WFValue": g.text_state("")}))

    def test_default_and_optional_decoder_paths_remain_exact(self):
        # These source-authored literals and decoder operations occur in each
        # selected function before its independently guarded diagnostic body.
        defaults = [i for i, a in enumerate(self.actions)
                    if params(self.actions, i).get("WFTextActionText") == "DEFAULT"]
        self.assertEqual(len(defaults), 1)
        self.rejected(lambda a: params(a, defaults[0]).__setitem__("WFTextActionText", ""))
        optional_guards = [i for i, a in enumerate(self.actions)
                           if a["WFWorkflowActionIdentifier"] == g.ACTION_PREFIX + "conditional"
                           and params(self.actions, i).get("WFCondition") == 5
                           and params(self.actions, i).get("WFConditionalActionString") == "0"]
        self.assertEqual(len(optional_guards), 1)
        self.rejected(lambda a: params(a, optional_guards[0]).__setitem__("WFCondition", 4))

    def test_trace_identity_and_final_order_are_exact(self):
        for stage in ("before", "after"):
            index = self.expected.roles["trace_" + stage]
            for old, new in (('"label":"function-envelope-presence"', '"label":"foreign"'),
                             ('"stage":"' + stage + '"', '"stage":"other"'),
                             ('"stepSequence":' + str(index), '"stepSequence":0')):
                with self.subTest(stage=stage, field=old):
                    self.rejected(lambda a: params(a, index)["WFTextActionText"]["Value"].__setitem__(
                        "string", params(a, index)["WFTextActionText"]["Value"]["string"].replace(old, new)))
            self.rejected(lambda a: params(a, index + 1).__setitem__("WFVariableName", "FOREIGN_TRACE"))
        index = self.expected.roles["traceEnvelope"]
        self.rejected(lambda a: params(a, index)["WFTextActionText"]["Value"].__setitem__(
            "string", params(a, index)["WFTextActionText"]["Value"]["string"].replace(
                '"runId":"qa-function-envelope-presence-v2"', '"runId":"foreign"')))
        final = self.expected.roles["final"]
        self.rejected(lambda a: params(a, final)["WFTextActionText"]["Value"]["attachmentsByRange"].__setitem__(
            "{0, 1}", params(a, final)["WFTextActionText"]["Value"]["attachmentsByRange"]["{2, 1}"]))

    def test_uuid_edge_control_and_type_corruptions_are_rejected(self):
        raw = self.expected.roles["probeZero"]["rawCaller"]
        changes = {
            "duplicate_uuid": lambda a: params(a, raw).__setitem__("UUID", params(a, 0)["UUID"]),
            "unknown_edge": lambda a: params(a, raw)["WFInput"]["Value"].__setitem__("OutputUUID", "FFFFFFFF-FFFF-FFFF-FFFF-FFFFFFFFFFFF"),
            "future_edge": lambda a: params(a, raw)["WFInput"]["Value"].__setitem__("OutputUUID", params(a, self.expected.roles["final"])["UUID"]),
            "bad_uuid_case": lambda a: params(a, raw).__setitem__("UUID", params(a, raw)["UUID"].lower()),
            "non_string_display": lambda a: params(a, raw).__setitem__("CustomOutputName", 1),
            "foreign_reference_property": lambda a: params(a, raw)["WFInput"]["Value"].__setitem__("Extra", True),
            "numeric_boolean": lambda a: params(a, 100).__setitem__("WFNumberValue", True),
            "control_mode_boolean": lambda a: params(a, 100).__setitem__("WFControlFlowMode", False),
            "wrong_loop_kind": lambda a: a[33].__setitem__("WFWorkflowActionIdentifier", g.ACTION_PREFIX + "conditional"),
            "wrong_end_group": lambda a: params(a, 95).__setitem__("GroupingIdentifier", params(a, 100)["GroupingIdentifier"]),
        }
        for name, mutate in changes.items():
            with self.subTest(name=name):
                self.rejected(mutate)

    def test_only_alpha_identity_and_display_names_may_change(self):
        actions = copy.deepcopy(self.actions)
        identifiers = set()
        for action in actions:
            p = action.get("WFWorkflowActionParameters", {})
            identifiers.update(p[k] for k in ("UUID", "GroupingIdentifier") if k in p)
        remap = {old: str(uuid.uuid5(uuid.NAMESPACE_URL, "function-observer/" + old)).upper()
                 for old in identifiers}

        def change(value):
            if isinstance(value, list):
                return [change(item) for item in value]
            if not isinstance(value, dict):
                return value
            result = {}
            for key, item in value.items():
                if key in ("UUID", "GroupingIdentifier", "OutputUUID") and isinstance(item, str):
                    result[key] = remap[item]
                elif key in ("CustomOutputName", "OutputName"):
                    result[key] = "display-only"
                else:
                    result[key] = change(item)
            return result

        changed = change(actions)
        self.assertNotEqual(changed, self.actions)
        self.assertEqual(g.validate_actions(changed)["actionCount"], 379)

    def test_source_contract_and_original_xml_boundaries(self):
        source, contract = SOURCE.read_bytes(), CONTRACT.read_bytes()
        for wrong_source, wrong_contract, native in (
                (source + b"\n", contract, self.native),
                (source, contract + b"\n", self.native),
                (source, contract, self.native + b"junk"),
                (source, contract, self.native[:-12]),
                (source, contract, b""),
                (source, contract, b"x" * (1024 * 1024 + 1))):
            with self.subTest(source=len(wrong_source), contract=len(wrong_contract), native=len(native)):
                with self.assertRaises(ValueError):
                    g.validate_compiled(wrong_source, wrong_contract, native)
        xml = ET.fromstring(self.native)
        dictionary = xml.find("dict")
        children = list(dictionary)
        dictionary.append(copy.deepcopy(children[0]))
        dictionary.append(copy.deepcopy(children[1]))
        with self.assertRaisesRegex(ValueError, "XML_DUPLICATE_KEY"):
            g.validate_compiled(source, contract, ET.tostring(xml))


if __name__ == "__main__":
    unittest.main()
