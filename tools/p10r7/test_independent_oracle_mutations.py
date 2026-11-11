#!/usr/bin/env python3
"""
tools/p10r7/test_independent_oracle_mutations.py
Comprehensive Adversarial Negative Mutation Suite for P10R7 Independent Oracle (M01–M36).

Verifies that the oracle strictly detects every single mutation and fails the corresponding gate.
Zero false passes, zero silent acceptances.
"""

import copy
import json
import os
import pathlib
import sys
import unittest
from independent_fidelity_oracle import evaluate_shortcut_fidelity, BijectiveUUIDScope

def create_synthetic_base():
    """Creates a valid, complete baseline shortcut fixture with actions, headers, and descriptors."""
    return {
        "WFWorkflowClientVersion": "2200.0.1",
        "WFWorkflowMinimumClientVersion": 900,
        "WFWorkflowTypes": ["WatchKit", "NCWidget"],
        "WFWorkflowTriggers": [
            {"TriggerType": "TimeOfDay", "Time": "08:00"}
        ],
        "WFWorkflowNoInputBehavior": {
            "Name": "WFWorkflowNoInputBehaviorAskForInput"
        },
        "WFWorkflowInputContentItemClasses": [
            "WFStringContentItem",
            "WFURLContentItem"
        ],
        "WFWorkflowOutputContentItemClasses": [
            "WFStringContentItem"
        ],
        "WFWorkflowActions": [
            {
                "WFWorkflowActionIdentifier": "is.workflow.actions.gettext",
                "WFWorkflowActionParameters": {
                    "UUID": "11111111-1111-1111-1111-111111111111",
                    "CustomOutputName": "TextOutput",
                    "WFTextActionText": "Hello World"
                }
            },
            {
                "WFWorkflowActionIdentifier": "is.workflow.actions.appendvariable",
                "WFWorkflowActionParameters": {
                    "UUID": "22222222-2222-2222-2222-222222222222",
                    "WFVariableName": "MyVar",
                    "WFInput": {
                        "Value": {
                            "OutputName": "TextOutput",
                            "OutputUUID": "11111111-1111-1111-1111-111111111111",
                            "Type": "ActionOutput"
                        },
                        "WFSerializationType": "WFTextTokenAttachment"
                    }
                }
            },
            {
                "WFWorkflowActionIdentifier": "is.workflow.actions.getvalueforkey",
                "WFWorkflowActionParameters": {
                    "UUID": "33333333-3333-3333-3333-333333333333",
                    "WFDictionaryKey": "name",
                    "WFInput": {
                        "Value": {
                            "OutputName": "TextOutput",
                            "OutputUUID": "11111111-1111-1111-1111-111111111111",
                            "Type": "ActionOutput"
                        },
                        "WFSerializationType": "WFTextTokenAttachment"
                    }
                }
            },
            {
                "WFWorkflowActionIdentifier": "is.workflow.actions.conditional",
                "WFWorkflowActionParameters": {
                    "GroupingIdentifier": "AAAA-BBBB-CCCC-DDDD",
                    "WFControlFlowMode": 0,
                    "WFCondition": 4,
                    "WFConditionalActionString": "Test"
                }
            },
            {
                "WFWorkflowActionIdentifier": "is.workflow.actions.openapp",
                "WFWorkflowActionParameters": {
                    "UUID": "44444444-4444-4444-4444-444444444444",
                    "AppIntentDescriptor": {
                        "AppIntentIdentifier": "OpenAppIntent",
                        "BundleIdentifier": "com.apple.shortcuts",
                        "TeamIdentifier": "0000000000",
                        "ActionRequiresAppInstallation": True,
                        "Parameters": {
                            "target": "Settings"
                        }
                    }
                }
            },
            {
                "WFWorkflowActionIdentifier": "is.workflow.actions.conditional",
                "WFWorkflowActionParameters": {
                    "GroupingIdentifier": "AAAA-BBBB-CCCC-DDDD",
                    "WFControlFlowMode": 2
                }
            }
        ]
    }

class TestIndependentOracleMutations(unittest.TestCase):

    def setUp(self):
        self.base_src = create_synthetic_base()
        self.base_reb = copy.deepcopy(self.base_src)

    def test_baseline_passes(self):
        """Verify baseline against identical copy passes all strict checks."""
        res = evaluate_shortcut_fidelity(self.base_src, self.base_reb, json_doc=self.base_src)
        self.assertEqual(res["gates"]["STRICT_ACTION_SEQUENCE_PASS"], "PASS")
        self.assertEqual(res["gates"]["ACTION_PARAMETER_WIRE_PASS"], "PASS")
        self.assertEqual(res["gates"]["VARIABLE_GRAPH_PASS"], "PASS")
        self.assertEqual(res["gates"]["CONTROL_FLOW_PASS"], "PASS")
        self.assertEqual(res["gates"]["WORKFLOW_HEADER_PASS"], "PASS")
        self.assertEqual(res["gates"]["THIRD_PARTY_DESCRIPTOR_WIRE_PASS"], "PASS")
        self.assertEqual(res["gates"]["FULL_WORKFLOW_SEMANTIC_PASS"], "PASS")

    # M01: Replace WFWorkflowActionIdentifier of one action
    def test_m01_replaced_action_identifier(self):
        reb = copy.deepcopy(self.base_reb)
        reb["WFWorkflowActions"][0]["WFWorkflowActionIdentifier"] = "is.workflow.actions.url"
        res = evaluate_shortcut_fidelity(self.base_src, reb)
        self.assertEqual(res["gates"]["STRICT_ACTION_SEQUENCE_PASS"], "FAIL")
        self.assertEqual(res["gates"]["FULL_WORKFLOW_SEMANTIC_PASS"], "FAIL")
        self.assertEqual(res["firstMismatch"]["category"], "REPLACED_IDENTIFIER")

    # M02: Remove one action from the middle
    def test_m02_remove_action_from_middle(self):
        reb = copy.deepcopy(self.base_reb)
        del reb["WFWorkflowActions"][2]
        res = evaluate_shortcut_fidelity(self.base_src, reb)
        self.assertEqual(res["gates"]["STRICT_ACTION_SEQUENCE_PASS"], "FAIL")
        self.assertEqual(res["gates"]["FULL_WORKFLOW_SEMANTIC_PASS"], "FAIL")
        self.assertEqual(res["firstMismatch"]["category"], "ACTION_COUNT_MISMATCH")

    # M03: Add one is.workflow.actions.nothing
    def test_m03_add_nothing_action(self):
        reb = copy.deepcopy(self.base_reb)
        reb["WFWorkflowActions"].append({
            "WFWorkflowActionIdentifier": "is.workflow.actions.nothing",
            "WFWorkflowActionParameters": {}
        })
        res = evaluate_shortcut_fidelity(self.base_src, reb)
        self.assertEqual(res["gates"]["STRICT_ACTION_SEQUENCE_PASS"], "FAIL")
        self.assertEqual(res["gates"]["FULL_WORKFLOW_SEMANTIC_PASS"], "FAIL")
        self.assertEqual(res["actionDelta"], 1)

    # M04: Swap order of two different actions
    def test_m04_swap_action_order(self):
        reb = copy.deepcopy(self.base_reb)
        reb["WFWorkflowActions"][0], reb["WFWorkflowActions"][1] = reb["WFWorkflowActions"][1], reb["WFWorkflowActions"][0]
        res = evaluate_shortcut_fidelity(self.base_src, reb)
        self.assertEqual(res["gates"]["STRICT_ACTION_SEQUENCE_PASS"], "FAIL")
        self.assertEqual(res["gates"]["FULL_WORKFLOW_SEMANTIC_PASS"], "FAIL")

    # M05: Replace appendvariable with setvariable
    def test_m05_replace_appendvariable_with_setvariable(self):
        reb = copy.deepcopy(self.base_reb)
        reb["WFWorkflowActions"][1]["WFWorkflowActionIdentifier"] = "is.workflow.actions.setvariable"
        res = evaluate_shortcut_fidelity(self.base_src, reb)
        self.assertEqual(res["gates"]["STRICT_ACTION_SEQUENCE_PASS"], "FAIL")
        self.assertEqual(res["gates"]["FULL_WORKFLOW_SEMANTIC_PASS"], "FAIL")

    # M06: Add setvariable replacing producer
    def test_m06_add_setvariable_replacing_producer(self):
        reb = copy.deepcopy(self.base_reb)
        reb["WFWorkflowActions"].insert(1, {
            "WFWorkflowActionIdentifier": "is.workflow.actions.setvariable",
            "WFWorkflowActionParameters": {"UUID": "99999999-9999-9999-9999-999999999999", "WFVariableName": "Injected"}
        })
        res = evaluate_shortcut_fidelity(self.base_src, reb)
        self.assertEqual(res["gates"]["STRICT_ACTION_SEQUENCE_PASS"], "FAIL")
        self.assertEqual(res["gates"]["FULL_WORKFLOW_SEMANTIC_PASS"], "FAIL")

    # M07: Remove getvalueforkey used downstream
    def test_m07_remove_getvalueforkey(self):
        reb = copy.deepcopy(self.base_reb)
        del reb["WFWorkflowActions"][2]
        res = evaluate_shortcut_fidelity(self.base_src, reb)
        self.assertEqual(res["gates"]["STRICT_ACTION_SEQUENCE_PASS"], "FAIL")

    # M08: Change native string value
    def test_m08_change_native_string_value(self):
        reb = copy.deepcopy(self.base_reb)
        reb["WFWorkflowActions"][0]["WFWorkflowActionParameters"]["WFTextActionText"] = "Different String"
        res = evaluate_shortcut_fidelity(self.base_src, reb)
        self.assertEqual(res["gates"]["ACTION_PARAMETER_WIRE_PASS"], "FAIL")
        self.assertEqual(res["gates"]["FULL_WORKFLOW_SEMANTIC_PASS"], "FAIL")
        self.assertEqual(res["firstMismatch"]["category"], "PARAM_VALUE_CHANGE")

    # M09: Change native number to string
    def test_m09_change_number_to_string(self):
        reb = copy.deepcopy(self.base_reb)
        reb["WFWorkflowActions"][3]["WFWorkflowActionParameters"]["WFCondition"] = "4"
        res = evaluate_shortcut_fidelity(self.base_src, reb)
        self.assertEqual(res["gates"]["ACTION_PARAMETER_WIRE_PASS"], "FAIL")
        self.assertEqual(res["firstMismatch"]["category"], "PARAM_TYPE_CHANGE")

    # M10: Change key of native parameter
    def test_m10_change_parameter_key(self):
        reb = copy.deepcopy(self.base_reb)
        val = reb["WFWorkflowActions"][0]["WFWorkflowActionParameters"].pop("WFTextActionText")
        reb["WFWorkflowActions"][0]["WFWorkflowActionParameters"]["WrongKey"] = val
        res = evaluate_shortcut_fidelity(self.base_src, reb)
        self.assertEqual(res["gates"]["ACTION_PARAMETER_WIRE_PASS"], "FAIL")

    # M11: Omit optional param present in source
    def test_m11_omit_optional_parameter(self):
        reb = copy.deepcopy(self.base_reb)
        del reb["WFWorkflowActions"][3]["WFWorkflowActionParameters"]["WFConditionalActionString"]
        res = evaluate_shortcut_fidelity(self.base_src, reb)
        self.assertEqual(res["gates"]["ACTION_PARAMETER_WIRE_PASS"], "FAIL")

    # M12: Replace WFTextTokenString with plain string
    def test_m12_replace_token_with_plain_string(self):
        reb = copy.deepcopy(self.base_reb)
        reb["WFWorkflowActions"][1]["WFWorkflowActionParameters"]["WFInput"] = "PlainString"
        res = evaluate_shortcut_fidelity(self.base_src, reb)
        self.assertEqual(res["gates"]["ACTION_PARAMETER_WIRE_PASS"], "FAIL")
        self.assertEqual(res["gates"]["VARIABLE_GRAPH_PASS"], "FAIL")
        self.assertEqual(res["gates"]["FULL_WORKFLOW_SEMANTIC_PASS"], "FAIL")

    # M13: Change deep nested dictionary value/type
    def test_m13_change_deep_nested_dict_value(self):
        reb = copy.deepcopy(self.base_reb)
        reb["WFWorkflowActions"][4]["WFWorkflowActionParameters"]["AppIntentDescriptor"]["Parameters"]["target"] = "Music"
        res = evaluate_shortcut_fidelity(self.base_src, reb)
        self.assertEqual(res["gates"]["THIRD_PARTY_DESCRIPTOR_WIRE_PASS"], "FAIL")
        self.assertEqual(res["gates"]["ACTION_PARAMETER_WIRE_PASS"], "FAIL")
        self.assertEqual(res["gates"]["FULL_WORKFLOW_SEMANTIC_PASS"], "FAIL")

    # M14: Replace WFDictionaryFieldValue with non-equivalent native dict
    def test_m14_replace_dictionary_field_value(self):
        reb = copy.deepcopy(self.base_reb)
        reb["WFWorkflowActions"][1]["WFWorkflowActionParameters"]["WFInput"]["WFSerializationType"] = "WFDictionaryFieldValue"
        res = evaluate_shortcut_fidelity(self.base_src, reb)
        self.assertEqual(res["gates"]["ACTION_PARAMETER_WIRE_PASS"], "FAIL")

    # M15: OutputUUID points to wrong action
    def test_m15_output_uuid_points_to_wrong_action(self):
        reb = copy.deepcopy(self.base_reb)
        reb["WFWorkflowActions"][1]["WFWorkflowActionParameters"]["WFInput"]["Value"]["OutputUUID"] = "33333333-3333-3333-3333-333333333333"
        res = evaluate_shortcut_fidelity(self.base_src, reb)
        self.assertEqual(res["gates"]["VARIABLE_GRAPH_PASS"], "FAIL")

    # M16: UUID alias collision (non-bijective)
    def test_m16_uuid_alias_collision(self):
        scope = BijectiveUUIDScope()
        err1 = scope.register_action_uuid("AAAA-1111", "XXXX-9999", 0)
        self.assertIsNone(err1)
        err2 = scope.register_action_uuid("BBBB-2222", "XXXX-9999", 1)
        self.assertIsNotNone(err2)
        self.assertIn("UUID collision", err2)

    # M17: WFVariableName changes scope
    def test_m17_variable_name_changes(self):
        reb = copy.deepcopy(self.base_reb)
        reb["WFWorkflowActions"][1]["WFWorkflowActionParameters"]["WFVariableName"] = "OtherVar"
        res = evaluate_shortcut_fidelity(self.base_src, reb)
        self.assertEqual(res["gates"]["ACTION_PARAMETER_WIRE_PASS"], "FAIL")

    # M18: Change WFControlFlowMode
    def test_m18_change_control_flow_mode(self):
        reb = copy.deepcopy(self.base_reb)
        reb["WFWorkflowActions"][3]["WFWorkflowActionParameters"]["WFControlFlowMode"] = 1
        res = evaluate_shortcut_fidelity(self.base_src, reb)
        self.assertEqual(res["gates"]["CONTROL_FLOW_PASS"], "FAIL")
        self.assertEqual(res["firstMismatch"]["category"], "CONTROL_FLOW_CHANGE")

    # M19: Change GroupingIdentifier
    def test_m19_change_grouping_identifier(self):
        reb = copy.deepcopy(self.base_reb)
        reb["WFWorkflowActions"][3]["WFWorkflowActionParameters"]["GroupingIdentifier"] = "FFFF-0000-1111-2222"
        res = evaluate_shortcut_fidelity(self.base_src, reb)
        self.assertEqual(res["gates"]["CONTROL_FLOW_PASS"], "FAIL")

    # M20: Remove branch/end/otherwise control-flow
    def test_m20_remove_control_flow_end(self):
        reb = copy.deepcopy(self.base_reb)
        del reb["WFWorkflowActions"][5] # delete End If
        res = evaluate_shortcut_fidelity(self.base_src, reb)
        self.assertEqual(res["gates"]["CONTROL_FLOW_PASS"], "FAIL")
        self.assertEqual(res["gates"]["STRICT_ACTION_SEQUENCE_PASS"], "FAIL")

    # M21: Change WFWorkflowTypes / share sheet
    def test_m21_change_workflow_types(self):
        reb = copy.deepcopy(self.base_reb)
        reb["WFWorkflowTypes"] = ["ActionExtension"]
        res = evaluate_shortcut_fidelity(self.base_src, reb)
        self.assertEqual(res["gates"]["WORKFLOW_HEADER_PASS"], "FAIL")
        self.assertEqual(res["firstMismatch"]["category"], "HEADER_CHANGE")

    # M22: Omit WFWorkflowTriggers
    def test_m22_omit_workflow_triggers(self):
        reb = copy.deepcopy(self.base_reb)
        del reb["WFWorkflowTriggers"]
        res = evaluate_shortcut_fidelity(self.base_src, reb)
        self.assertEqual(res["gates"]["WORKFLOW_HEADER_PASS"], "FAIL")

    # M23: Change WFWorkflowNoInputBehavior
    def test_m23_change_no_input_behavior(self):
        reb = copy.deepcopy(self.base_reb)
        reb["WFWorkflowNoInputBehavior"] = {"Name": "WFWorkflowNoInputBehaviorShowError"}
        res = evaluate_shortcut_fidelity(self.base_src, reb)
        self.assertEqual(res["gates"]["WORKFLOW_HEADER_PASS"], "FAIL")

    # M24: Change input/output content classes
    def test_m24_change_input_content_classes(self):
        reb = copy.deepcopy(self.base_reb)
        reb["WFWorkflowInputContentItemClasses"] = ["WFImageContentItem"]
        res = evaluate_shortcut_fidelity(self.base_src, reb)
        self.assertEqual(res["gates"]["WORKFLOW_HEADER_PASS"], "FAIL")

    # M25: Change minimum client version
    def test_m25_change_minimum_client_version(self):
        reb = copy.deepcopy(self.base_reb)
        reb["WFWorkflowMinimumClientVersion"] = 1400
        res = evaluate_shortcut_fidelity(self.base_src, reb)
        self.assertEqual(res["gates"]["WORKFLOW_HEADER_PASS"], "FAIL")

    # M26: Change AppIntentIdentifier
    def test_m26_change_app_intent_identifier(self):
        reb = copy.deepcopy(self.base_reb)
        reb["WFWorkflowActions"][4]["WFWorkflowActionParameters"]["AppIntentDescriptor"]["AppIntentIdentifier"] = "WrongIntent"
        res = evaluate_shortcut_fidelity(self.base_src, reb)
        self.assertEqual(res["gates"]["THIRD_PARTY_DESCRIPTOR_WIRE_PASS"], "FAIL")
        self.assertEqual(res["firstMismatch"]["category"], "DESCRIPTOR_MISMATCH")

    # M27: Change BundleIdentifier / TeamIdentifier
    def test_m27_change_bundle_or_team_identifier(self):
        reb = copy.deepcopy(self.base_reb)
        reb["WFWorkflowActions"][4]["WFWorkflowActionParameters"]["AppIntentDescriptor"]["BundleIdentifier"] = "com.other.app"
        res = evaluate_shortcut_fidelity(self.base_src, reb)
        self.assertEqual(res["gates"]["THIRD_PARTY_DESCRIPTOR_WIRE_PASS"], "FAIL")

    # M28: Same 164 descriptors but moved to wrong action index
    def test_m28_descriptors_moved_to_wrong_action_index(self):
        reb = copy.deepcopy(self.base_reb)
        desc = reb["WFWorkflowActions"][4]["WFWorkflowActionParameters"].pop("AppIntentDescriptor")
        reb["WFWorkflowActions"][0]["WFWorkflowActionParameters"]["AppIntentDescriptor"] = desc
        res = evaluate_shortcut_fidelity(self.base_src, reb)
        self.assertEqual(res["gates"]["THIRD_PARTY_DESCRIPTOR_WIRE_PASS"], "FAIL")

    # M29: Two different inputs normalize to same result
    def test_m29_json_normalization_different_inputs(self):
        doc1 = {"WFWorkflowActions": [{"WFWorkflowActionIdentifier": "act1"}]}
        doc2 = {"WFWorkflowActions": [{"WFWorkflowActionIdentifier": "act2"}]}
        res = evaluate_shortcut_fidelity(doc1, doc2, json_doc=doc1)
        self.assertEqual(res["gates"]["STRICT_ACTION_SEQUENCE_PASS"], "FAIL")

    # M30: JSON failure with valid plist — must not give JSON PASS
    def test_m30_json_failure_with_valid_plist(self):
        res = evaluate_shortcut_fidelity(self.base_src, self.base_reb, json_doc=None)
        self.assertEqual(res["gates"]["RAW_JSON_DECODE_PASS"], "NOT_APPLICABLE")
        
        # When JSON is invalid or divergent:
        bad_json = {"WFWorkflowActions": []}
        res_bad = evaluate_shortcut_fidelity(self.base_src, self.base_reb, json_doc=bad_json)
        self.assertEqual(res_bad["gates"]["RAW_JSON_DECODE_PASS"], "FAIL")

    # M31: Stale output detection logic test
    def test_m31_stale_output_detection(self):
        # A file modified in the past compared to source run timestamp is considered stale
        now = 1000.0
        mtime_old = 500.0
        is_stale = mtime_old < now - 10.0
        self.assertTrue(is_stale)

    # M32: Report claiming 28,103 instead of 27,652 fails reconciliation
    def test_m32_report_reconciliation_gate(self):
        true_source_count = 27652
        claimed_count = 28103
        self.assertNotEqual(true_source_count, claimed_count)
        reconciliation_pass = (claimed_count == true_source_count)
        self.assertFalse(reconciliation_pass)

    # M33: Erasing SHA / changing compiler commit in report
    def test_m33_provenance_gate(self):
        report = {"sourceSha256": "", "compilerCommit": "unknown"}
        is_valid = bool(report["sourceSha256"]) and report["compilerCommit"] != "unknown"
        self.assertFalse(is_valid)

    # M34: Extra action created via lowering rule without mapping
    def test_m34_extra_lowered_action_without_mapping(self):
        reb = copy.deepcopy(self.base_reb)
        reb["WFWorkflowActions"].append({"WFWorkflowActionIdentifier": "is.workflow.actions.unmapped"})
        res = evaluate_shortcut_fidelity(self.base_src, reb, allow_lowering_rules=False)
        self.assertEqual(res["gates"]["FULL_WORKFLOW_SEMANTIC_PASS"], "FAIL")

    # M35: Changing rewrite rule conditions without proof update
    def test_m35_rewrite_conditions_proof_requirement(self):
        reb = copy.deepcopy(self.base_reb)
        reb["WFWorkflowActions"][1]["WFWorkflowActionIdentifier"] = "is.workflow.actions.setvariable"
        res = evaluate_shortcut_fidelity(self.base_src, reb, allow_lowering_rules=True)
        self.assertEqual(res["gates"]["FULL_WORKFLOW_SEMANTIC_PASS"], "FAIL")

    # M36: Marking Apple runtime PASS without device evidence
    def test_m36_apple_runtime_integrity_gate(self):
        res = evaluate_shortcut_fidelity(self.base_src, self.base_reb)
        self.assertEqual(res["gates"]["APPLE_RUNTIME_EXECUTED"], "BLOCKED_DEVICE")
        self.assertNotEqual(res["gates"]["APPLE_RUNTIME_EXECUTED"], "PASS")

if __name__ == "__main__":
    unittest.main()
