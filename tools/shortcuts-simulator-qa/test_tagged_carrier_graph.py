"""Causal graph controls against an actual public compiler artifact.

The fixture was produced once by compiler cb23a754. These tests mutate its real
bytes in memory; they never execute Shortcuts or create native acceptance credit.
"""
import copy
import hashlib
import json
import pathlib
import plistlib
import unittest
import uuid

import tagged_carrier_graph as graph

DIRECTORY = pathlib.Path(__file__).parent
FIXTURES = DIRECTORY / "fixtures"
ORIGINAL = FIXTURES / "source-graphs/qa-tagged-value-carrier-v2.compiler-cb23a754.plist"
ORIGINAL_SHA256 = "dabf4b0869396585e315a0fbc397945fcdc1efc7defbf64763b274e75b85c2ea"


def p(actions, index):
    return actions[index]["WFWorkflowActionParameters"]


def output_id(actions, index):
    return p(actions, index)["UUID"]


def edge_to(actions, target):
    return {"WFSerializationType": "WFTextTokenAttachment",
            "Value": {"Type": "ActionOutput", "OutputUUID": output_id(actions, target)}}


def original_actions():
    raw = ORIGINAL.read_bytes()
    if hashlib.sha256(raw).hexdigest() != ORIGINAL_SHA256:
        raise AssertionError("actual compiler fixture changed")
    return plistlib.loads(raw)["WFWorkflowActions"]


def changed_at(path, value):
    def mutate(actions):
        target = actions
        for key in path[:-1]:
            target = target[key]
        target[path[-1]] = value
    return mutate


def set_parameter(index, key, value):
    return changed_at([index, "WFWorkflowActionParameters", key], value)


def redirect(index, key, target, conditional=False):
    def mutate(actions):
        edge = edge_to(actions, target)
        p(actions, index)[key] = {"Type": "Variable", "Variable": edge} if conditional else edge
    return mutate


def cell_value(index, row, value):
    return changed_at([index, "WFWorkflowActionParameters", "WFItems", "Value",
                       "WFDictionaryFieldValueItems", row, "WFValue", "Value", "string"], value)


def fault_inventory():
    r = graph.expected_program().roles
    first, empty, mapping, absent = (r[k] for k in ("first", "empty", "map", "absent"))
    equality = r["typeEquality"]
    faults = {}
    def add(name, mutation):
        if name in faults:
            raise AssertionError("duplicate fault name")
        faults[name] = mutation
    def swap(left, right):
        def mutate(actions):
            actions[left], actions[right] = actions[right], actions[left]
        return mutate

    add("missing_action", lambda a: a.pop(0))
    add("extra_action", lambda a: a.append({"WFWorkflowActionIdentifier": "is.workflow.actions.nothing"}))
    add("reordered_source_numbers", swap(5, 6))
    add("unexpected_action_parameter", set_parameter(r["originalListType"], "WFHiddenInput", "extra"))
    add("unexpected_action_outer_key", lambda a: a[8].update(WFUnknownActionState=True))
    add("missing_registered_nothing_parameter_map",
        lambda a: a[1].pop("WFWorkflowActionParameters"))
    add("extra_parameterless_nothing_parameter_map",
        lambda a: a[27].update(WFWorkflowActionParameters={}))
    add("duplicate_producer_uuid", lambda a: p(a, 8).update(UUID=output_id(a, 7)))
    add("unknown_producer_uuid", set_parameter(8, "WFInput", {
        "WFSerializationType": "WFTextTokenAttachment",
        "Value": {"Type": "ActionOutput", "OutputUUID": "00000000-0000-4000-8000-000000000000"}}))
    add("non_string_producer_uuid", set_parameter(8, "WFInput", {
        "WFSerializationType": "WFTextTokenAttachment",
        "Value": {"Type": "ActionOutput", "OutputUUID": []}}))
    add("forward_producer_edge", redirect(8, "WFInput", r["originalMap"]))
    add("self_producer_edge", redirect(8, "WFInput", 8))
    add("original_list_value", set_parameter(5, "WFNumberActionNumber", 12))
    add("original_number_changed_to_boolean", set_parameter(5, "WFNumberActionNumber", True))
    add("original_list_reversed", lambda a: p(a, r["originalList"])["WFItems"].reverse())
    add("list_cell_payload_redirected_to_text", lambda a:
        p(a, r["firstCell"])["WFItems"]["Value"]["WFDictionaryFieldValueItems"][3].update(
            WFValue=edge_to(a, 10)))
    add("list_cell_wrong_kind", cell_value(r["firstCell"], 2, "text"))
    add("list_cell_wrong_marker", cell_value(r["firstCell"], 0, "2"))
    add("empty_list_cell_marked_absent", cell_value(r["emptyCell"], 1, "0"))
    add("empty_list_cell_changed_to_text", changed_at([
        r["emptyCell"], "WFWorkflowActionParameters", "WFItems", "Value",
        "WFDictionaryFieldValueItems", 3, "WFItemType"], 0))
    add("empty_list_payload_not_empty", changed_at([
        r["emptyCell"], "WFWorkflowActionParameters", "WFItems", "Value",
        "WFDictionaryFieldValueItems", 3, "WFValue", "Value"], [""]))
    add("absent_map_cell_marked_present", cell_value(r["absentCell"], 1, "1"))
    add("absent_map_cell_wrong_kind", cell_value(r["absentCell"], 2, "map"))
    add("absent_map_cell_injected_payload", lambda a:
        p(a, r["absentCell"])["WFItems"]["Value"]["WFDictionaryFieldValueItems"].append(
            {"WFItemType": 0, "WFKey": graph.text_state("value"),
             "WFValue": graph.text_state("invented")}))
    add("map_cell_payload_is_original_list", lambda a:
        p(a, r["mapCell"])["WFItems"]["Value"]["WFDictionaryFieldValueItems"][3].update(
            WFValue=edge_to(a, r["originalList"])))
    add("original_map_value", cell_value(r["originalMap"], 0, "CHANGED"))
    for label, role in (("first", first), ("empty", empty), ("map", mapping), ("absent", absent)):
        add(label + "_ordinal_changed", set_parameter(role["oneBased"], "WFInput", 3))
        add(label + "_lower_bound_polarity", set_parameter(role["lower"], "WFCondition", 2))
        add(label + "_upper_bound_threshold", set_parameter(role["upper"], "WFNumberValue", -1.0))
        add(label + "_presence_key", set_parameter(role["has"], "WFDictionaryKey", "kind"))
        add(label + "_payload_key", set_parameter(role["payload"], "WFDictionaryKey", "has_value"))
        add(label + "_presence_polarity", set_parameter(role["present"], "WFConditionalActionString", "0"))
    add("count_source_is_scalar", redirect(first["count"], "Input", 5))
    add("difference_operands_swapped", lambda a: p(a, first["difference"]).update(
        WFInput=edge_to(a, first["oneBased"]), WFMathOperand=edge_to(a, first["count"])))
    add("decoded_value_store_uses_original_list", redirect(first["decoded"] + 2, "WFInput", r["originalList"]))
    add("decoded_presence_store_uses_payload", redirect(first["decoded"] + 1, "WFInput", first["payload"]))
    add("decoded_value_variable_wrong_slot", lambda a:
        p(a, first["result"]).update(WFVariable=copy.deepcopy(p(a, empty["result"])["WFVariable"])))
    add("list_type_reads_original_instead_of_decoded", redirect(r["decodedListType"], "WFInput", r["originalList"]))
    add("map_type_reads_original_instead_of_decoded", redirect(r["decodedMapType"], "WFInput", r["originalMap"]))
    add("text_type_reads_list", redirect(r["textType"], "WFInput", r["originalList"]))
    add("decoded_type_encoding_reads_original", redirect(r["decodedMapType"] + 1, "WFInput", r["originalMapType"]))
    add("type_encoding_mode", set_parameter(r["decodedMapType"] + 1, "WFEncodeMode", "Decode"))
    add("type_encoding_line_break", set_parameter(r["decodedMapType"] + 1, "WFBase64LineBreakMode", "64 Characters"))
    add("unowned_value_slot", set_parameter(first["result"], "WFVariable", graph.variable("untrusted")))
    add("crossed_control_groups", lambda a:
        p(a, first["upper"]).update(GroupingIdentifier=p(a, first["lower"])["GroupingIdentifier"]))
    add("control_mode_boolean", set_parameter(first["upper"], "WFControlFlowMode", False))
    add("map_dictionary_read_before_type_guard", swap(r["mapKeys"], equality["guard"]))
    add("map_guard_ends_before_read", swap(r["mapTypeGuardEnd"], r["mapBeta"]))
    for role_name in ("mapKeys", "mapAlpha", "mapBeta"):
        add(role_name + "_uses_original_map", redirect(r[role_name], "WFInput", r["originalMap"]))
    add("map_keys_changed_to_values", set_parameter(r["mapKeys"], "WFGetDictionaryValueType", "All Values"))
    add("map_alpha_key_changed", set_parameter(r["mapAlpha"], "WFDictionaryKey", "beta"))
    add("comparison_wrong_escape_find", set_parameter(equality["quote"], "WFReplaceTextFind", r"\Q"))
    add("comparison_wrong_escape_replace", set_parameter(equality["quote"], "WFReplaceTextReplace", r"\E"))
    add("comparison_regex_replace_enabled", set_parameter(equality["quote"], "WFReplaceTextRegularExpression", True))
    add("comparison_case_insensitive", set_parameter(equality["match"], "WFMatchTextCaseSensitive", False))
    add("comparison_unanchored_pattern", lambda a:
        p(a, equality["match"])["WFMatchTextPattern"]["Value"].update(string="\ufffc"))
    add("comparison_wrong_cardinality", set_parameter(equality["compare"], "WFNumberValue", 2.0))
    add("comparison_true_false_reversed", set_parameter(equality["compare"] + 1, "WFNumberActionNumber", 0.0))
    add("comparison_guard_inverted", set_parameter(equality["guard"], "WFCondition", 5))
    add("comparison_left_source_spoof", lambda a:
        p(a, equality["start"])["WFTextActionText"]["Value"]["attachmentsByRange"]["{1, 1}"].update(
            OutputUUID=output_id(a, r["originalMapType"])))
    add("semantic_summary_is_constant_success", set_parameter(r["semantic"], "WFTextActionText", "PASS"))
    add("semantic_summary_missing_field", lambda a:
        p(a, r["semantic"])["WFTextActionText"]["Value"].update(string="CHERRI_TAGGED_VALUE_CARRIER_V1:"))
    add("semantic_summary_type_from_wrong_producer", lambda a:
        p(a, r["semantic"])["WFTextActionText"]["Value"]["attachmentsByRange"]["{157, 1}"].update(
            OutputUUID=output_id(a, 12)))
    add("semantic_summary_value_from_wrong_slot", lambda a:
        p(a, r["semantic"])["WFTextActionText"]["Value"]["attachmentsByRange"]["{179, 1}"].update(
            VariableName="originalType64"))
    add("before_trace_stage", lambda a:
        p(a, r["trace_before"])["WFTextActionText"]["Value"].update(
            string=p(a, r["trace_before"])["WFTextActionText"]["Value"]["string"].replace('"before"', '"after"')))
    add("trace_fixture_identity", lambda a:
        p(a, r["traceEnvelope"])["WFTextActionText"]["Value"].update(
            string=p(a, r["traceEnvelope"])["WFTextActionText"]["Value"]["string"].replace(
                "qa-tagged-value-carrier-v2", "qa-nested-list-candidate-v2")))
    add("final_summary_omits_trace", lambda a:
        p(a, r["final"]).update(WFTextActionText=graph.text_state("\ufffc", [edge_to(a, r["semantic"])])))
    add("return_is_original_list", lambda a:
        p(a, 224)["WFOutput"]["Value"]["attachmentsByRange"]["{0, 1}"].update(
            OutputUUID=output_id(a, r["originalList"])))
    return faults


FAULTS = fault_inventory()


class CarrierGraphControls(unittest.TestCase):
    def test_exact_actual_compiler_fixture(self):
        source = (FIXTURES / "qa-tagged-value-carrier-v2.cherri").read_bytes()
        contract = (FIXTURES / "qa-tagged-value-carrier-v2.expected.json").read_bytes()
        native = ORIGINAL.read_bytes()
        manifest = json.loads(ORIGINAL.with_suffix(".json").read_bytes())
        self.assertEqual(hashlib.sha256(native).hexdigest(), ORIGINAL_SHA256)
        self.assertEqual(manifest["unsignedSha256"], ORIGINAL_SHA256)
        self.assertEqual(manifest["compilerSource"], "cb23a75446f1fdb13da9d10fe7ffe9d6903b47f7")
        self.assertFalse(manifest["nativeExecuted"])
        result = graph.validate_compiled(source, contract, native)
        self.assertEqual(result["actionCount"], 225)
        self.assertTrue(result["sourceAuthenticated"])
        self.assertFalse(result["nativeCarrierPreservationVerified"])
        self.assertFalse(result["appleExecutionAuthenticated"])
        self.assertFalse(result["formalAcceptanceGranted"])

    def test_semantic_identity_alpha_renaming(self):
        actions = original_actions()
        identities = set()
        def inventory(value):
            if isinstance(value, dict):
                for key, item in value.items():
                    if key in ("UUID", "OutputUUID", "GroupingIdentifier"):
                        identities.add(item)
                    inventory(item)
            elif isinstance(value, list):
                for item in value:
                    inventory(item)
        inventory(actions)
        renamed = {value: str(uuid.uuid5(uuid.NAMESPACE_URL, "tagged-test-" + value)).upper()
                   for value in identities}
        slots = {}
        for action in actions:
            name = graph.parameters(action).get("WFVariableName", "")
            if name.startswith("__cherri_scope_"):
                slots[name] = name.rsplit("_", 1)[0] + "_" + str(
                    uuid.uuid5(uuid.NAMESPACE_URL, "slot-" + name)).upper()
        def rename(value):
            if isinstance(value, dict):
                result = {}
                for key, item in value.items():
                    if key in ("UUID", "OutputUUID", "GroupingIdentifier"):
                        result[key] = renamed[item]
                    elif key in ("WFVariableName", "VariableName") and item in slots:
                        result[key] = slots[item]
                    elif key in ("CustomOutputName", "OutputName"):
                        result[key] = "Display label only"
                    else:
                        result[key] = rename(item)
                return result
            if isinstance(value, list):
                return [rename(item) for item in value]
            return value
        result = graph.validate_actions(rename(actions))
        self.assertEqual(result["actionCount"], 225)
        self.assertFalse(result["nativeCarrierPreservationVerified"])

    def test_changed_source_is_rejected(self):
        with self.assertRaisesRegex(ValueError, "SOURCE_SHA256"):
            graph.validate_compiled(
                (FIXTURES / "qa-tagged-value-carrier-v2.cherri").read_bytes() + b"\n",
                (FIXTURES / "qa-tagged-value-carrier-v2.expected.json").read_bytes(),
                ORIGINAL.read_bytes())

    def test_changed_contract_is_rejected(self):
        with self.assertRaisesRegex(ValueError, "CONTRACT_SHA256"):
            graph.validate_compiled(
                (FIXTURES / "qa-tagged-value-carrier-v2.cherri").read_bytes(),
                (FIXTURES / "qa-tagged-value-carrier-v2.expected.json").read_bytes() + b"\n",
                ORIGINAL.read_bytes())


def make_fault_test(name, mutation):
    def test(self):
        actions = original_actions()
        before = copy.deepcopy(actions)
        mutation(actions)
        self.assertNotEqual(
            hashlib.sha256(plistlib.dumps(actions, fmt=plistlib.FMT_BINARY)).hexdigest(),
            hashlib.sha256(plistlib.dumps(before, fmt=plistlib.FMT_BINARY)).hexdigest(),
            "fault must modify typed native bytes")
        with self.assertRaisesRegex(ValueError, "TAGGED_CARRIER_GRAPH_"):
            graph.validate_actions(actions)
    test.__name__ = "test_reject_" + name
    return test


for fault_name, mutation in FAULTS.items():
    setattr(CarrierGraphControls, "test_reject_" + fault_name,
            make_fault_test(fault_name, mutation))


if __name__ == "__main__":
    unittest.main()
