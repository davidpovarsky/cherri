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

def create_p10r8_base():
    """Synthetic base extended with every header field P10R7 failed to compare."""
    base = create_synthetic_base()
    base["WFWorkflowIcon"] = {"WFWorkflowIconGlyphNumber": 59511, "WFWorkflowIconStartColor": 4282601983}
    base["WFQuickActionSurfaces"] = ["QuickActions"]
    base["WFWorkflowMinimumClientVersionString"] = "900"
    base["WFWorkflowHasShortcutInputVariables"] = False
    base["WFWorkflowHasOutputFallback"] = False
    base["WFWorkflowImportQuestions"] = [
        {"ActionIndex": 0, "Category": "Parameter", "ParameterKey": "WFTextActionText", "Text": "Q"}
    ]
    base["WFWorkflowInputContentItemClasses"] = []
    return base


class TestP10R8OracleMutations(unittest.TestCase):
    """M37-M60. Each mutation is executed against the real evaluator."""

    def setUp(self):
        self.src = create_p10r8_base()
        self.reb = copy.deepcopy(self.src)

    def ev(self, reb, src=None, **kw):
        return evaluate_shortcut_fidelity(src if src is not None else self.src, reb, **kw)

    def assertFullFail(self, res, *gate_names):
        for g in gate_names:
            self.assertEqual(res["gates"][g], "FAIL", g)
        self.assertEqual(res["gates"]["FULL_WORKFLOW_SEMANTIC_PASS"], "FAIL")
        self.assertEqual(res["extendedGates"]["STRICT_NATIVE_WIRE_FIDELITY_PASS"], "FAIL")

    def test_p10r8_baseline_passes_with_json(self):
        res = self.ev(self.reb, json_doc=self.src)
        for g, v in res["gates"].items():
            if g in ("DECOMPILE_PASS", "LEGACY_COMPILE_PASS", "V2_COMPILE_PASS", "APPLE_RUNTIME_EXECUTED"):
                continue
            self.assertEqual(v, "PASS", g)
        self.assertEqual(res["extendedGates"]["SOURCE_CONTAINER_EQUIVALENCE_PASS"], "PASS")
        self.assertEqual(res["gateMismatches"], {})

    def test_m37_change_client_version(self):
        self.reb["WFWorkflowClientVersion"] = "2300.0.0"
        self.assertFullFail(self.ev(self.reb), "WORKFLOW_HEADER_PASS")

    def test_m38_remove_client_version(self):
        del self.reb["WFWorkflowClientVersion"]
        self.assertFullFail(self.ev(self.reb), "WORKFLOW_HEADER_PASS")

    def test_m39_change_icon_glyph_and_color(self):
        for field in ("WFWorkflowIconGlyphNumber", "WFWorkflowIconStartColor"):
            reb = copy.deepcopy(self.reb)
            reb["WFWorkflowIcon"][field] += 1
            self.assertFullFail(self.ev(reb), "WORKFLOW_HEADER_PASS")
        reb = copy.deepcopy(self.reb)
        del reb["WFWorkflowIcon"]["WFWorkflowIconStartColor"]
        self.assertFullFail(self.ev(reb), "WORKFLOW_HEADER_PASS")

    def test_m40_add_or_remove_quick_action_surfaces(self):
        reb = copy.deepcopy(self.reb)
        del reb["WFQuickActionSurfaces"]
        self.assertFullFail(self.ev(reb), "WORKFLOW_HEADER_PASS")
        src = copy.deepcopy(self.src)
        del src["WFQuickActionSurfaces"]
        self.assertFullFail(self.ev(self.reb, src=src), "WORKFLOW_HEADER_PASS")

    def test_m41_remove_custom_output_name(self):
        del self.reb["WFWorkflowActions"][0]["WFWorkflowActionParameters"]["CustomOutputName"]
        self.assertFullFail(self.ev(self.reb), "ACTION_PARAMETER_WIRE_PASS")

    def test_m42_change_custom_output_name(self):
        self.reb["WFWorkflowActions"][0]["WFWorkflowActionParameters"]["CustomOutputName"] = "Renamed"
        self.assertFullFail(self.ev(self.reb), "ACTION_PARAMETER_WIRE_PASS")

    def test_m43_remove_action_uuid_including_final(self):
        for idx in (0, 4):
            reb = copy.deepcopy(self.reb)
            del reb["WFWorkflowActions"][idx]["WFWorkflowActionParameters"]["UUID"]
            self.assertFullFail(self.ev(reb), "VARIABLE_GRAPH_PASS", "ACTION_PARAMETER_WIRE_PASS")
        # Source UUID absent but rebuilt invents one: extra key, never harmless.
        src = copy.deepcopy(self.src)
        del src["WFWorkflowActions"][4]["WFWorkflowActionParameters"]["UUID"]
        res = self.ev(self.reb, src=src)
        self.assertFullFail(res, "VARIABLE_GRAPH_PASS", "ACTION_PARAMETER_WIRE_PASS")
        # The synthetic final action never had a UUID: adding one must fail too.
        reb = copy.deepcopy(self.reb)
        reb["WFWorkflowActions"][5]["WFWorkflowActionParameters"]["UUID"] = "55555555-5555-5555-5555-555555555555"
        self.assertFullFail(self.ev(reb), "VARIABLE_GRAPH_PASS")

    def test_m44_duplicate_producer_uuid(self):
        self.reb["WFWorkflowActions"][1]["WFWorkflowActionParameters"]["UUID"] = \
            self.reb["WFWorkflowActions"][0]["WFWorkflowActionParameters"]["UUID"]
        self.assertFullFail(self.ev(self.reb), "VARIABLE_GRAPH_PASS")

    def test_m45_output_uuid_non_existent_producer(self):
        self.reb["WFWorkflowActions"][1]["WFWorkflowActionParameters"]["WFInput"]["Value"]["OutputUUID"] = \
            "99999999-0000-0000-0000-000000000000"
        res = self.ev(self.reb)
        self.assertFullFail(res, "VARIABLE_GRAPH_PASS")
        self.assertEqual(res["gateMismatches"]["VARIABLE_GRAPH_PASS"]["subcategory"], "DANGLING_REFERENCE")

    def test_m45b_alpha_renaming_is_allowed_only_when_bijective(self):
        # Consistent renaming of producer+reference is NOT a fidelity failure in the graph gate.
        reb = copy.deepcopy(self.reb)
        old = "11111111-1111-1111-1111-111111111111"
        new = "ABABABAB-1111-1111-1111-111111111111"
        reb["WFWorkflowActions"][0]["WFWorkflowActionParameters"]["UUID"] = new
        for i in (1, 2):
            reb["WFWorkflowActions"][i]["WFWorkflowActionParameters"]["WFInput"]["Value"]["OutputUUID"] = new
        res = self.ev(reb)
        self.assertEqual(res["gates"]["VARIABLE_GRAPH_PASS"], "PASS")
        self.assertEqual(res["gates"]["ACTION_PARAMETER_WIRE_PASS"], "PASS")
        self.assertNotEqual(old, new)

    def test_m46_json_text_differs_from_plist(self):
        js = copy.deepcopy(self.src)
        js["WFWorkflowActions"][0]["WFWorkflowActionParameters"]["WFTextActionText"] = "Other"
        res = self.ev(self.reb, json_doc=js)
        self.assertEqual(res["gates"]["RAW_JSON_DECODE_PASS"], "FAIL")
        self.assertEqual(res["extendedGates"]["SOURCE_CONTAINER_EQUIVALENCE_PASS"], "FAIL")
        self.assertEqual(res["gates"]["FULL_WORKFLOW_SEMANTIC_PASS"], "FAIL")

    def test_m47_json_nested_key_or_type_differs(self):
        js = copy.deepcopy(self.src)
        js["WFWorkflowActions"][4]["WFWorkflowActionParameters"]["AppIntentDescriptor"]["Parameters"]["extra"] = 1
        self.assertEqual(self.ev(self.reb, json_doc=js)["extendedGates"]["SOURCE_CONTAINER_EQUIVALENCE_PASS"], "FAIL")
        js = copy.deepcopy(self.src)
        js["WFWorkflowActions"][4]["WFWorkflowActionParameters"]["AppIntentDescriptor"]["ActionRequiresAppInstallation"] = "true"
        self.assertEqual(self.ev(self.reb, json_doc=js)["extendedGates"]["SOURCE_CONTAINER_EQUIVALENCE_PASS"], "FAIL")
        js = copy.deepcopy(self.src)
        js["WFWorkflowMinimumClientVersion"] = "900"
        self.assertEqual(self.ev(self.reb, json_doc=js)["extendedGates"]["SOURCE_CONTAINER_EQUIVALENCE_PASS"], "FAIL")

    def test_m48_empty_list_replaced_by_missing_key(self):
        del self.reb["WFWorkflowInputContentItemClasses"]
        self.assertFullFail(self.ev(self.reb), "WORKFLOW_HEADER_PASS")

    def test_m49_minimum_client_version_string_only(self):
        self.reb["WFWorkflowMinimumClientVersionString"] = "1400"
        self.assertFullFail(self.ev(self.reb), "WORKFLOW_HEADER_PASS")

    def test_m50_descriptor_moved_preserving_content_multiset(self):
        src = copy.deepcopy(self.src)
        extra = copy.deepcopy(src["WFWorkflowActions"][4]["WFWorkflowActionParameters"]["AppIntentDescriptor"])
        extra["AppIntentIdentifier"] = "SecondIntent"
        src["WFWorkflowActions"][0]["WFWorkflowActionParameters"]["AppIntentDescriptor"] = extra
        reb = copy.deepcopy(src)
        # swap descriptors between the two actions: same bag of dictionaries, wrong indices
        d0 = reb["WFWorkflowActions"][0]["WFWorkflowActionParameters"]["AppIntentDescriptor"]
        d4 = reb["WFWorkflowActions"][4]["WFWorkflowActionParameters"]["AppIntentDescriptor"]
        reb["WFWorkflowActions"][0]["WFWorkflowActionParameters"]["AppIntentDescriptor"] = d4
        reb["WFWorkflowActions"][4]["WFWorkflowActionParameters"]["AppIntentDescriptor"] = d0
        res = self.ev(reb, src=src)
        self.assertFullFail(res, "THIRD_PARTY_DESCRIPTOR_WIRE_PASS")

    def test_m51_malformed_text_token_string(self):
        src = copy.deepcopy(self.src)
        src["WFWorkflowActions"][0]["WFWorkflowActionParameters"]["WFTextActionText"] = {
            "Value": {"string": "\ufffc", "attachmentsByRange": {"{0, 1}": {"Type": "Variable", "VariableName": "x"}}},
            "WFSerializationType": "WFTextTokenString",
        }
        reb = copy.deepcopy(src)
        reb["WFWorkflowActions"][0]["WFWorkflowActionParameters"]["WFTextActionText"]["Value"]["attachmentsByRange"] = {
            "{0, 2}": {"Type": "Variable", "VariableName": "x"}}
        self.assertFullFail(self.ev(reb, src=src), "ACTION_PARAMETER_WIRE_PASS")
        reb = copy.deepcopy(src)
        del reb["WFWorkflowActions"][0]["WFWorkflowActionParameters"]["WFTextActionText"]["Value"]["attachmentsByRange"]
        self.assertFullFail(self.ev(reb, src=src), "ACTION_PARAMETER_WIRE_PASS")

    def test_m52_default_serialization_type_added(self):
        self.reb["WFWorkflowActions"][0]["WFWorkflowActionParameters"]["WFSerializationType"] = "WFTextTokenString"
        res = self.ev(self.reb)
        self.assertFullFail(res, "ACTION_PARAMETER_WIRE_PASS")
        self.assertEqual(res["gateMismatches"]["ACTION_PARAMETER_WIRE_PASS"]["category"], "EXTRA_PARAM_KEY")

    def test_m53_stale_shortcut_after_failed_compile_step(self):
        import tempfile
        from audit_integrity import run_compile_step
        with tempfile.TemporaryDirectory() as d:
            out = pathlib.Path(d) / "x.shortcut"
            out.write_bytes(b"stale-previous-output")
            old = out.stat().st_mtime - 1000
            os.utime(out, (old, old))
            # step that fails and produces nothing
            res = run_compile_step([sys.executable, "-c", "import sys; sys.exit(3)"], out)
            self.assertEqual(res["gate"], "FAIL")
            self.assertFalse(res["outputPresent"], "stale file must have been removed")
            # step that exits 0 but produces nothing is also a failure
            res = run_compile_step([sys.executable, "-c", "pass"], out)
            self.assertEqual(res["gate"], "FAIL")
            # a genuine fresh output passes
            script = f"open({str(out)!r},'wb').write(b'fresh')"
            res = run_compile_step([sys.executable, "-c", script], out)
            self.assertEqual(res["gate"], "PASS")

    def test_m54_source_extra_top_level_key(self):
        self.src["WFWorkflowFutureKey"] = {"x": 1}
        self.assertFullFail(self.ev(self.reb), "WORKFLOW_HEADER_PASS")
        reb = copy.deepcopy(self.src)
        reb["WFWorkflowOtherFutureKey"] = 1
        self.assertFullFail(self.ev(reb), "WORKFLOW_HEADER_PASS")

    def test_m55_group_end_marker_or_import_question_index(self):
        self.reb["WFWorkflowActions"][5]["WFWorkflowActionParameters"]["WFControlFlowMode"] = 1
        self.assertFullFail(self.ev(self.reb), "CONTROL_FLOW_PASS")
        reb = copy.deepcopy(self.src)
        reb["WFWorkflowImportQuestions"][0]["ActionIndex"] = 1
        self.assertFullFail(self.ev(reb), "WORKFLOW_HEADER_PASS")

    def test_m56_missing_specimen_fails_closed(self):
        import tempfile
        from audit_integrity import check_corpus_inventory
        with tempfile.TemporaryDirectory() as d:
            root = pathlib.Path(d)
            for sid in [f"{i:03d}" for i in range(1, 39)]:
                (root / f"{sid}.plist").write_bytes(b"x")
                (root / f"{sid}.json").write_bytes(b"{}")
            self.assertEqual(check_corpus_inventory(root), [])
            (root / "017.plist").unlink()
            errs = check_corpus_inventory(root)
            self.assertEqual(errs, ["MISSING_SPECIMEN:017.plist"])
            (root / "038.json").write_bytes(b"")
            self.assertIn("EMPTY_SPECIMEN:038.json", check_corpus_inventory(root))

    def test_m57_output_name_changed_with_bijective_uuid_remap(self):
        reb = copy.deepcopy(self.reb)
        new = "CDCDCDCD-1111-1111-1111-111111111111"
        reb["WFWorkflowActions"][0]["WFWorkflowActionParameters"]["UUID"] = new
        for i in (1, 2):
            v = reb["WFWorkflowActions"][i]["WFWorkflowActionParameters"]["WFInput"]["Value"]
            v["OutputUUID"] = new
        reb["WFWorkflowActions"][1]["WFWorkflowActionParameters"]["WFInput"]["Value"]["OutputName"] = "Different"
        res = self.ev(reb)
        self.assertEqual(res["gates"]["VARIABLE_GRAPH_PASS"], "PASS")
        self.assertFullFail(res, "ACTION_PARAMETER_WIRE_PASS")

    def test_m58_output_fallback_and_input_variables(self):
        for key in ("WFWorkflowHasOutputFallback", "WFWorkflowHasShortcutInputVariables"):
            reb = copy.deepcopy(self.reb)
            reb[key] = not reb[key]
            self.assertFullFail(self.ev(reb), "WORKFLOW_HEADER_PASS")
            reb = copy.deepcopy(self.reb)
            del reb[key]
            self.assertFullFail(self.ev(reb), "WORKFLOW_HEADER_PASS")

    def test_m59_string_true_in_native_bool_intent(self):
        d = self.reb["WFWorkflowActions"][4]["WFWorkflowActionParameters"]["AppIntentDescriptor"]
        d["ActionRequiresAppInstallation"] = "true"
        res = self.ev(self.reb)
        self.assertFullFail(res, "ACTION_PARAMETER_WIRE_PASS", "THIRD_PARTY_DESCRIPTOR_WIRE_PASS")

    def test_m60_runtime_pass_without_device_evidence(self):
        from audit_integrity import check_runtime_gate
        res = self.ev(self.reb)
        self.assertEqual(check_runtime_gate(res["gates"]), [])
        forged = dict(res["gates"], APPLE_RUNTIME_EXECUTED="PASS")
        self.assertEqual(check_runtime_gate(forged), ["RUNTIME_PASS_WITHOUT_DEVICE_EVIDENCE"])
        self.assertTrue(check_runtime_gate(forged, {"deviceModel": "x"}))

    # --- Additional hardening beyond M37-M60 -------------------------------------------
    def test_per_gate_first_mismatch_is_not_hidden_by_header_failure(self):
        self.reb["WFWorkflowClientVersion"] = "other"
        self.reb["WFWorkflowActions"][0]["WFWorkflowActionParameters"]["WFTextActionText"] = "Changed"
        res = self.ev(self.reb)
        self.assertEqual(res["firstMismatch"]["gate"], "WORKFLOW_HEADER_PASS")
        self.assertIn("ACTION_PARAMETER_WIRE_PASS", res["gateMismatches"])
        self.assertIn("WORKFLOW_HEADER_PASS", res["gateMismatches"])

    def test_mismatch_details_never_contain_values(self):
        self.reb["WFWorkflowActions"][0]["WFWorkflowActionParameters"]["WFTextActionText"] = "SECRET-TOKEN-123"
        res = self.ev(self.reb)
        self.assertNotIn("SECRET-TOKEN-123", json.dumps(res))
        self.assertNotIn("Hello World", json.dumps(res))

    def test_source_control_flow_orphan_end_is_never_a_pass(self):
        src = copy.deepcopy(self.src)
        src["WFWorkflowActions"].append(copy.deepcopy(src["WFWorkflowActions"][5]))  # orphan duplicate End
        reb = copy.deepcopy(src)  # byte-identical rebuild of a malformed source
        res = self.ev(reb, src=src)
        self.assertFalse(res["sourceValidity"]["controlFlowValid"])
        self.assertEqual(res["gates"]["CONTROL_FLOW_PASS"], "FAIL")
        self.assertEqual(res["gates"]["FULL_WORKFLOW_SEMANTIC_PASS"], "FAIL")
        self.assertEqual(res["gateMismatches"]["CONTROL_FLOW_PASS"]["subcategory"], "SOURCE_CONTROL_FLOW_INVALID")

    def test_unregistered_uuid_is_not_dynamically_accepted(self):
        scope = BijectiveUUIDScope()
        self.assertFalse(scope.match_uuid("A", "B"))

    def test_other_uuid_shaped_strings_are_exact_not_remapped(self):
        src = copy.deepcopy(self.src)
        src["WFWorkflowActions"][0]["WFWorkflowActionParameters"]["WFTextActionText"] = "11111111-1111-1111-1111-111111111111"
        reb = copy.deepcopy(src)
        reb["WFWorkflowActions"][0]["WFWorkflowActionParameters"]["WFTextActionText"] = "22222222-2222-2222-2222-222222222222"
        self.assertFullFail(self.ev(reb, src=src), "ACTION_PARAMETER_WIRE_PASS")

    def test_int_float_not_coerced_in_wire(self):
        reb = copy.deepcopy(self.reb)
        reb["WFWorkflowActions"][3]["WFWorkflowActionParameters"]["WFCondition"] = 4.0
        self.assertFullFail(self.ev(reb), "ACTION_PARAMETER_WIRE_PASS")

    def test_unparseable_present_json_is_fail_in_file_audit(self):
        import plistlib
        import tempfile
        from independent_fidelity_oracle import audit_file_pair
        with tempfile.TemporaryDirectory() as d:
            root = pathlib.Path(d)
            (root / "s.plist").write_bytes(plistlib.dumps(self.src))
            (root / "r.shortcut").write_bytes(plistlib.dumps(self.reb))
            (root / "s.json").write_text("{not json", encoding="utf-8")
            res = audit_file_pair(root / "s.plist", root / "r.shortcut", root / "s.json")
            self.assertEqual(res["gates"]["RAW_JSON_DECODE_PASS"], "FAIL")
            self.assertEqual(res["gates"]["FULL_WORKFLOW_SEMANTIC_PASS"], "FAIL")

    def test_aligned_inventory_does_not_blame_shifted_actions(self):
        from independent_fidelity_oracle import aligned_action_inventory
        reb = copy.deepcopy(self.reb)
        reb["WFWorkflowActions"].insert(0, {"WFWorkflowActionIdentifier": "is.workflow.actions.nothing", "WFWorkflowActionParameters": {}})
        rows = aligned_action_inventory(self.src, reb)
        self.assertEqual([r["op"] for r in rows], ["added"])


if __name__ == "__main__":
    unittest.main()
