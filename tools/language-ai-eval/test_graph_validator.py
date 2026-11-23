"""Synthetic oracle controls only; these never measure model or Apple quality."""
import copy
from decimal import Decimal
import json
import os
from pathlib import Path
import plistlib
import sys
import tempfile
import unittest
from unittest import mock

sys.path.insert(0, str(Path(__file__).resolve().parent))
import evaluation_bundle as bundle
import graph_validator as validator


def identifier(index):
    return f"abcdef{index:02x}-1234-4567-89ab-0123456789ab"


def action(kind, params):
    return {"WFWorkflowActionIdentifier": validator.PREFIX+kind,
            "WFWorkflowActionParameters": params}


def reference(index, name=None):
    ref = {"Type": "ActionOutput", "OutputUUID": identifier(index)}
    if name is not None:
        ref["OutputName"] = name
    return {"WFSerializationType": "WFTextTokenAttachment", "Value": ref}


def variable(name):
    return {"WFSerializationType": "WFTextTokenAttachment",
            "Value": {"Type": "Variable", "VariableName": name}}


def workflow(actions):
    # Round-trip through actual plist bytes. No JSON scalar normalization.
    return plistlib.loads(plistlib.dumps({"WFWorkflowActions": actions}, fmt=plistlib.FMT_BINARY))


def positive(task, parameters, arm):
    oracle = task["oracle"]
    if task["id"] in (bundle.TASK_IDS[1], bundle.TASK_IDS[5]):
        name = parameters["binding"]
        value = {"WFInput": str(parameters["left"]) if arm == "legacy" else parameters["left"],
                 "WFMathOperand": str(parameters["right"]) if arm == "legacy" else parameters["right"],
                 "WFMathOperation": "+", "UUID": identifier(1)}
        actions = [action("math", value)]
        if arm == "legacy":
            actions.append(action("setvariable", {"WFVariableName": name, "WFInput": reference(1)}))
            shown = variable(name)
        else:
            value["CustomOutputName"] = name
            shown = reference(1, name)
        actions.append(action("showresult", {"Text": shown}))
    elif "list" in oracle:
        items = oracle["list"]
        if arm == "legacy":
            name = "items_control"
            actions = [action("nothing", {}), action("setvariable", {"WFVariableName": name})]
            for index, text in enumerate(items, 1):
                actions += [action("gettext", {"WFTextActionText": text, "UUID": identifier(index)}),
                            action("appendvariable", {"WFVariableName": name, "WFInput": reference(index)})]
            selected_from = variable(name)
        else:
            actions = [action("list", {"WFItems": [{"WFItemType": 0, "WFValue": text} for text in items], "UUID": identifier(1)})]
            selected_from = reference(1)
        actions += [action("getitemfromlist", {"WFInput": selected_from, "WFItemIndex": 2,
                    "WFItemSpecifier": "Item At Index", "UUID": identifier(9)}),
                    action("showresult", {"Text": reference(9)})]
    elif "unchanged_binding" in oracle:
        name, value = next(iter(oracle["unchanged_binding"].items()))
        params = {"WFNumberActionNumber": str(value) if arm == "legacy" else value, "UUID": identifier(1)}
        actions = [action("number", params)]
        if arm == "legacy":
            actions.append(action("setvariable", {"WFVariableName": name, "WFInput": reference(1)}))
        else:
            params["CustomOutputName"] = name
        actions.append(action("showresult", {"Text": oracle["visible_text"]}))
    elif "requires_binding_reference" in oracle:
        name = oracle["requires_binding_reference"]
        params = {"WFTextActionText": oracle["visible_text"], "UUID": identifier(1)}
        actions = [action("gettext", params)]
        if arm == "legacy":
            actions.append(action("setvariable", {"WFVariableName": name, "WFInput": reference(1)}))
            shown = variable(name)
        else:
            params["CustomOutputName"] = name
            shown = reference(1, name)
        actions.append(action("showresult", {"Text": shown}))
    else:
        actions = [action("showresult", {"Text": oracle["visible_text"]})]
    return workflow(actions)


def guarded_list(task, zero_based=1):
    def start(group, value, boundary):
        return action("conditional", {"GroupingIdentifier": group, "WFControlFlowMode": 0,
            "WFCondition": 3, "WFConditionalLegacyComparisonBehavior": True,
            "WFInput": {"Type": "Variable", "Variable": value}, "WFNumberValue": boundary})
    def endpoint(group, mode, output=None, name=None):
        params = {"GroupingIdentifier": group, "WFControlFlowMode": mode}
        if output is not None:
            params["UUID"] = identifier(output)
        if name is not None:
            params["CustomOutputName"] = name
        return action("conditional", params)
    outer, inner = identifier(20), identifier(21)
    return workflow([
        action("list", {"WFItems": [{"WFItemType": 0, "WFValue": text} for text in task["oracle"]["list"]],
                        "UUID": identifier(1), "CustomOutputName": "items"}),
        action("math", {"WFInput": zero_based, "WFMathOperand": 1.0, "WFMathOperation": "+", "UUID": identifier(2)}),
        action("count", {"Input": reference(1, "items"), "WFCountType": "Items", "UUID": identifier(3)}),
        action("math", {"WFInput": reference(3), "WFMathOperand": reference(2), "WFMathOperation": "-", "UUID": identifier(4)}),
        start(outer, reference(2), 1.0), start(inner, reference(4), 0.0),
        action("getitemfromlist", {"WFInput": reference(1, "items"), "WFItemIndex": reference(2),
                                    "WFItemSpecifier": "Item At Index", "UUID": identifier(5)}),
        endpoint(inner, 1), action("nothing", {}), endpoint(inner, 2, 6),
        endpoint(outer, 1), action("nothing", {}), endpoint(outer, 2, 7, "selected"),
        action("showresult", {"Text": reference(7, "selected")}), action("nothing", {})])


class SixTaskGraphControls(unittest.TestCase):
    def setUp(self):
        self.suite, self.parameters = bundle.generate_suite(bytes(range(32)))
        self.tasks = {t["id"]: t for t in self.suite["tasks"]}

    def checks(self, graph, task_id):
        return {c["name"]: c["passed"] for c in validator.semantic_checks(
            graph, self.tasks[task_id], self.parameters.get(task_id, {}))}

    def graph(self, index, arm="v2"):
        task = self.suite["tasks"][index]
        return positive(task, self.parameters.get(task["id"], {}), arm)

    def inventory_attempt(self, root):
        directory = root / "attempt"
        directory.mkdir()
        binary = root / "compiler.exe"
        binary.write_bytes(b"synthetic compiler bytes; never executed")
        task = self.tasks[bundle.TASK_IDS[0]]
        source = 'show("synthetic source")'
        request = {"schema": bundle.SCHEMA, "request_id": "a"*64, "task_id": task["id"],
                   "arm": "v2", "source": source, "source_sha256": bundle.digest(source.encode()),
                   "required_checks": ["compiles"]+["oracle."+key for key in sorted(task["oracle"])]}
        fake_bundle = {"root": root, "parameters": self.parameters,
                       "manifest": {"compiler": {"source_sha": "b"*40,
                       "binary_sha256": bundle.digest(binary.read_bytes())}}}
        return directory, task, request, fake_bundle

    def test_receipt_inventory_never_uses_unbounded_reads(self):
        with tempfile.TemporaryDirectory() as temporary:
            directory, task, request, fake_bundle = self.inventory_attempt(Path(temporary))
            graph = plistlib.dumps(self.graph(0), fmt=plistlib.FMT_BINARY)
            def compile_control(command, **kwargs):
                (directory / "output.shortcut").write_bytes(graph)
                return validator.subprocess.CompletedProcess(command, 0)
            with mock.patch.object(validator, "validation_request", return_value=task), \
                 mock.patch.object(validator, "claim_request", return_value=directory), \
                 mock.patch.object(validator.subprocess, "run", side_effect=compile_control), \
                 mock.patch.object(Path, "read_bytes", side_effect=AssertionError("unbounded original read forbidden")):
                reply = validator.evaluate_request(request, fake_bundle)
            self.assertTrue(all(check["passed"] for check in reply["checks"]))
            receipt = bundle.unique_json((directory / "validator-receipt.json").read_bytes())
            self.assertIs(receipt["originals_complete"], True)
            for name, original in receipt["originals"].items():
                raw = bundle.read_regular(directory / name)
                self.assertEqual(original, {"sha256": bundle.digest(raw), "bytes": len(raw), "complete": True})

    def test_oversized_original_remains_unadjudicated_and_is_not_read_or_deleted(self):
        with tempfile.TemporaryDirectory() as temporary:
            directory, task, request, fake_bundle = self.inventory_attempt(Path(temporary))
            def compile_control(command, **kwargs):
                with (directory / "output.shortcut").open("wb") as stream:
                    stream.truncate(validator.MAX_PLIST + 1)
                return validator.subprocess.CompletedProcess(command, 0)
            with mock.patch.object(validator, "validation_request", return_value=task), \
                 mock.patch.object(validator, "claim_request", return_value=directory), \
                 mock.patch.object(validator.subprocess, "run", side_effect=compile_control), \
                 mock.patch.object(Path, "read_bytes", side_effect=AssertionError("unbounded original read forbidden")):
                with self.assertRaisesRegex(bundle.BundleError, "VALIDATOR_ORIGINALS_INCOMPLETE"):
                    validator.evaluate_request(request, fake_bundle)
            self.assertEqual((directory / "output.shortcut").stat().st_size, validator.MAX_PLIST + 1)
            receipt = bundle.unique_json((directory / "validator-receipt.json").read_bytes())
            self.assertEqual(receipt["status"], "UNADJUDICATED")
            self.assertIs(receipt["originals_complete"], False)
            self.assertEqual(receipt["original_errors"]["output.shortcut"]["error_code"], "INPUT_SIZE_LIMIT")
            self.assertNotIn("output.shortcut", receipt["originals"])

    def test_all_six_contracts_in_both_graph_encodings(self):
        for arm in ("legacy", "v2"):
            for index, task in enumerate(self.suite["tasks"]):
                with self.subTest(arm=arm, task=task["id"]):
                    checks = self.checks(self.graph(index, arm), task["id"])
                    self.assertEqual(set(checks), {"compiles"} | {"oracle."+key for key in task["oracle"]})
                    self.assertTrue(all(checks.values()), checks)

    def test_numeric_literal_cannot_replace_required_computation(self):
        for index in (1, 5):
            task = self.suite["tasks"][index]
            name = self.parameters[task["id"]]["binding"]
            graph = workflow([action("number", {"WFNumberActionNumber": task["oracle"]["numeric_result"],
                             "CustomOutputName": name, "UUID": identifier(1)}),
                              action("showresult", {"Text": reference(1, name)})])
            self.assertFalse(self.checks(graph, task["id"])["oracle.numeric_result"])

    def test_unused_addition_does_not_supply_display_lineage(self):
        graph = self.graph(1)
        task = self.suite["tasks"][1]
        graph["WFWorkflowActions"][-1]["WFWorkflowActionParameters"]["Text"] = str(task["oracle"]["numeric_result"])
        checks = self.checks(graph, task["id"])
        self.assertFalse(checks["oracle.numeric_result"])
        self.assertFalse(checks["oracle.requires_computation"])
        self.assertTrue(checks["oracle.side_effects"])

    def test_wrong_operands_with_same_sum_are_rejected(self):
        graph = self.graph(1)
        params = graph["WFWorkflowActions"][0]["WFWorkflowActionParameters"]
        params["WFInput"] += 1
        params["WFMathOperand"] -= 1
        self.assertFalse(self.checks(graph, bundle.TASK_IDS[1])["oracle.numeric_result"])

    def test_binding_reference_cannot_be_hardcoded_duplicate(self):
        for arm in ("legacy", "v2"):
            graph = self.graph(4, arm)
            graph["WFWorkflowActions"][-1]["WFWorkflowActionParameters"]["Text"] = self.suite["tasks"][4]["oracle"]["visible_text"]
            checks = self.checks(graph, bundle.TASK_IDS[4])
            self.assertTrue(checks["oracle.visible_text"])
            self.assertFalse(checks["oracle.requires_binding_reference"])

    def test_retained_number_binding_must_exist_once_and_be_unchanged(self):
        graph = self.graph(3)
        params = graph["WFWorkflowActions"][0]["WFWorkflowActionParameters"]
        params["WFNumberActionNumber"] += 1
        self.assertFalse(self.checks(graph, bundle.TASK_IDS[3])["oracle.unchanged_binding"])
        graph = self.graph(3)
        duplicate = copy.deepcopy(graph["WFWorkflowActions"][0])
        duplicate["WFWorkflowActionParameters"]["UUID"] = identifier(2)
        graph["WFWorkflowActions"].insert(1, duplicate)
        self.assertFalse(self.checks(graph, bundle.TASK_IDS[3])["oracle.unchanged_binding"])

    def test_list_order_and_middle_selection_are_independent_obligations(self):
        graph = self.graph(2)
        items = graph["WFWorkflowActions"][0]["WFWorkflowActionParameters"]["WFItems"]
        items[0], items[2] = items[2], items[0]
        checks = self.checks(graph, bundle.TASK_IDS[2])
        self.assertTrue(checks["oracle.selected_text"])
        self.assertFalse(checks["oracle.list"])
        graph = self.graph(2)
        graph["WFWorkflowActions"][1]["WFWorkflowActionParameters"]["WFItemIndex"] = 1
        self.assertFalse(self.checks(graph, bundle.TASK_IDS[2])["oracle.selected_text"])

    def test_unused_list_plus_literal_middle_is_not_selection(self):
        graph = self.graph(2)
        graph["WFWorkflowActions"][-1]["WFWorkflowActionParameters"]["Text"] = self.suite["tasks"][2]["oracle"]["selected_text"]
        checks = self.checks(graph, bundle.TASK_IDS[2])
        self.assertFalse(checks["oracle.list"])
        self.assertFalse(checks["oracle.selected_text"])

    def test_extra_show_is_a_real_effect_failure(self):
        graph = self.graph(0)
        graph["WFWorkflowActions"].append(copy.deepcopy(graph["WFWorkflowActions"][0]))
        self.assertFalse(self.checks(graph, bundle.TASK_IDS[0])["oracle.side_effects"])

    def test_same_numeric_boolean_and_list_control_type_are_not_interchangeable(self):
        graph = self.graph(1)
        graph["WFWorkflowActions"][0]["WFWorkflowActionParameters"]["WFInput"] = True
        with self.assertRaisesRegex(validator.Unsupported, "PARAMETER_KIND"):
            self.checks(graph, bundle.TASK_IDS[1])
        for wrong in (False, 0.0):
            graph = self.graph(2)
            graph["WFWorkflowActions"][0]["WFWorkflowActionParameters"]["WFItems"][0]["WFItemType"] = wrong
            with self.assertRaisesRegex(validator.Unsupported, "LIST_ITEM_NATIVE_TYPE"):
                self.checks(graph, bundle.TASK_IDS[2])

    def test_forward_missing_zero_and_casefold_duplicate_uuid_rejected(self):
        graph = self.graph(1)
        graph["WFWorkflowActions"][-1]["WFWorkflowActionParameters"]["Text"]["Value"]["OutputUUID"] = identifier(2)
        with self.assertRaisesRegex(validator.Unsupported, "REFERENCE_FORWARD_OR_MISSING"):
            self.checks(graph, bundle.TASK_IDS[1])
        for raw in ("00000000-0000-0000-0000-000000000000", "abcdef01"):
            graph = self.graph(1)
            graph["WFWorkflowActions"][0]["WFWorkflowActionParameters"]["UUID"] = raw
            with self.assertRaises(validator.Unsupported):
                self.checks(graph, bundle.TASK_IDS[1])
        graph = self.graph(1)
        duplicate = copy.deepcopy(graph["WFWorkflowActions"][0])
        duplicate["WFWorkflowActionParameters"]["UUID"] = identifier(1).upper()
        graph["WFWorkflowActions"].insert(1, duplicate)
        with self.assertRaisesRegex(validator.Unsupported, "DUPLICATE_UUID"):
            self.checks(graph, bundle.TASK_IDS[1])

    def test_reference_name_and_external_inputs_fail_closed(self):
        graph = self.graph(4)
        graph["WFWorkflowActions"][-1]["WFWorkflowActionParameters"]["Text"]["Value"]["OutputName"] = "other"
        with self.assertRaisesRegex(validator.Unsupported, "OUTPUT_NAME_MISMATCH"):
            self.checks(graph, bundle.TASK_IDS[4])
        graph = self.graph(0)
        graph["WFWorkflowHasShortcutInputVariables"] = True
        with self.assertRaisesRegex(validator.Unsupported, "WORKFLOW_EXTERNAL_INPUT"):
            self.checks(graph, bundle.TASK_IDS[0])

    def test_utf16_attachment_ranges_are_not_codepoint_offsets(self):
        graph = validator.Graph()
        graph.outputs[identifier(1)] = (identifier(1), None, validator.Value("Text", "value"))
        raw = {"WFSerializationType": "WFTextTokenString", "Value": {
               "string": "🍒\ufffc", "attachmentsByRange": {"{2, 1}": reference(1)["Value"]}}}
        self.assertEqual(graph.parameter(raw).data, "🍒value")
        raw["Value"]["attachmentsByRange"] = {"{1, 1}": reference(1)["Value"]}
        with self.assertRaisesRegex(validator.Unsupported, "TOKEN_RANGE_PLACEHOLDER"):
            graph.parameter(raw)

    def test_unsupported_native_effect_is_unadjudicated(self):
        graph = self.graph(0)
        graph["WFWorkflowActions"].insert(0, action("downloadurl", {"WFURL": "https://example.invalid"}))
        with self.assertRaisesRegex(validator.Unsupported, "ACTION_OUTSIDE_SIX_TASK_SUBSET"):
            self.checks(graph, bundle.TASK_IDS[0])

    def test_source_cannot_request_files_native_escapes_or_arbitrary_calls(self):
        for source in ('#include "private.cherri"\nshow(1)', 'native.workflow "private.shortcut"',
                       'show(readFile("secret"))', 'native.action("example", {})'):
            with self.subTest(source=source), self.assertRaises(validator.Unsupported):
                validator.safe_source(source)
        validator.safe_source('let title = "literal braces {text}"\nshow(title)')

    def test_source_subset_cannot_hide_callees_or_unclosed_raw_literals(self):
        rejected = (
            'let value = (embedFile)("private.txt")\nshow(value)',
            'let functionAlias = embedFile\nshow(functionAlias("private.txt"))',
            'let x = embedFile /* comment */ ("private.txt")\nshow(x)',
            'show(native.workflow("private.shortcut"))',
            'show("unterminated)',
            "show('unterminated\\')",
            'let x = arbitrary.member("private.txt")\nshow(x)',
            'show("#include private")',
            'show("#embed private")',
        )
        for source in rejected:
            with self.subTest(source=source), self.assertRaises(validator.Unsupported):
                validator.safe_source(source)
        validator.safe_source('let title: Text = "literal embedFile(private)"\nshow(title)')
        validator.safe_source('let items: List<Text> = ["a", "b", "c"]\nshow(items[1])')
        validator.safe_source('@items = ["a", "b", "c"]\n@selected = getListItem(@items, 2)\nshow(@selected)')

    def test_rejected_external_source_is_retained_before_any_compiler_read_or_process(self):
        for source in ('#include "private.cherri"\nshow(1)', 'show(embedFile("private.txt"))',
                       'import "private.cherri" as private\nshow(1)'):
            with self.subTest(source=source), tempfile.TemporaryDirectory() as temporary:
                root = Path(temporary)
                request = {"request_id": "a"*64, "task_id": bundle.TASK_IDS[0], "arm": "legacy", "source": source,
                           "source_sha256": bundle.digest(source.encode())}
                fake_bundle = {"root": root, "manifest": {"compiler": {"source_sha": "b"*40, "binary_sha256": "c"*64}}}
                def no_compiler_read(path, limit=validator.MAX_PLIST):
                    if Path(path) == root / "compiler.exe":
                        raise AssertionError("compiler read forbidden")
                    return bundle.read_regular(path, limit)
                with mock.patch.object(validator, "validation_request", return_value=self.tasks[bundle.TASK_IDS[0]]), \
                     mock.patch.object(validator, "claim_request", return_value=root), \
                     mock.patch.object(validator, "read_regular", side_effect=no_compiler_read), \
                     mock.patch.object(validator.subprocess, "run", side_effect=AssertionError("compiler invocation forbidden")) as process:
                    with self.assertRaises(validator.Unsupported):
                        validator.evaluate_request(request, fake_bundle)
                process.assert_not_called()
                self.assertEqual((root / "source.cherri").read_text(), source)
                receipt = bundle.unique_json((root / "validator-receipt.json").read_bytes())
                self.assertEqual(receipt["status"], "UNADJUDICATED")
                self.assertNotIn("command", receipt)

    def test_compiler_environment_has_no_credentials_home_paths_or_feature_overrides(self):
        with tempfile.TemporaryDirectory() as temporary, mock.patch.dict(os.environ, {
                "ANTHROPIC_API_KEY": "synthetic-credential", "CHERRI_P10R10_CORPUS_DIR": "private-corpus",
                "HOME": "private-home", "USERPROFILE": "private-profile", "PATH": "private-search-path",
                "TEMP": "private-temp", "SystemRoot": "public-os-root", "LANG": "C.UTF-8"}, clear=True):
            result = validator.compiler_environment(Path(temporary))
            self.assertEqual({key.upper() for key in result}, {"SYSTEMROOT", "LANG", "TEMP", "TMP"})
            self.assertEqual(result["TEMP"], str(Path(temporary) / "compiler-temp"))
            self.assertEqual(result["TMP"], result["TEMP"])
            self.assertNotIn("private", json.dumps(result))

    def test_plist_scalar_types_are_preserved(self):
        raw = plistlib.dumps({"i": 0, "r": 0.0, "b": False}, fmt=plistlib.FMT_BINARY)
        values = plistlib.loads(raw)
        self.assertIs(type(values["i"]), int)
        self.assertIs(type(values["r"]), float)
        self.assertIs(type(values["b"]), bool)

    def test_v2_index_guard_has_causal_count_bounds_and_selected_value(self):
        task = self.suite["tasks"][2]
        self.assertTrue(all(self.checks(guarded_list(task), task["id"]).values()))
        for index in (-1, 3):
            checks = self.checks(guarded_list(task, index), task["id"])
            self.assertFalse(checks["oracle.list"])
            self.assertFalse(checks["oracle.selected_text"])
            self.assertTrue(checks["oracle.side_effects"])

    def test_v2_index_guard_typed_integer_controls_and_real_rhs_are_exact(self):
        task = self.suite["tasks"][2]
        for key, value in (("WFCondition", 3.0), ("WFControlFlowMode", False),
                           ("WFNumberValue", 1), ("WFConditionalLegacyComparisonBehavior", 1)):
            graph = guarded_list(task)
            graph["WFWorkflowActions"][4]["WFWorkflowActionParameters"][key] = value
            with self.subTest(key=key), self.assertRaisesRegex(validator.Unsupported, "INDEX_GUARD_TYPED_PREDICATE"):
                self.checks(graph, task["id"])

    def test_v2_index_guard_cannot_borrow_unrelated_numeric_operand(self):
        task = self.suite["tasks"][2]
        graph = guarded_list(task)
        graph["WFWorkflowActions"][3]["WFWorkflowActionParameters"]["WFMathOperand"] = 2
        with self.assertRaisesRegex(validator.Unsupported, "INDEX_GUARD_CAUSAL_BOUNDS"):
            self.checks(graph, task["id"])
        graph = guarded_list(task)
        graph["WFWorkflowActions"][6]["WFWorkflowActionParameters"]["WFItemIndex"] = 2
        with self.assertRaisesRegex(validator.Unsupported, "INDEX_GUARD_SELECTION_IDENTITY"):
            self.checks(graph, task["id"])

    def test_v2_index_guard_rejects_casefold_group_alias_and_wrong_endpoint(self):
        task = self.suite["tasks"][2]
        graph = guarded_list(task)
        for index in (1+4, 3+4, 5+4):
            graph["WFWorkflowActions"][index]["WFWorkflowActionParameters"]["GroupingIdentifier"] = identifier(20).upper()
        with self.assertRaisesRegex(validator.Unsupported, "INDEX_GUARD_GROUP_REUSE"):
            self.checks(graph, task["id"])
        graph = guarded_list(task)
        graph["WFWorkflowActions"][9]["WFWorkflowActionParameters"]["GroupingIdentifier"] = identifier(22)
        with self.assertRaisesRegex(validator.Unsupported, "INDEX_GUARD_ENDPOINT"):
            self.checks(graph, task["id"])

    def test_v2_index_guard_inspects_untaken_branch_and_complete_identity_set(self):
        task = self.suite["tasks"][2]
        graph = guarded_list(task)
        graph["WFWorkflowActions"][8] = action("showresult", {"Text": "hidden effect"})
        with self.assertRaisesRegex(validator.Unsupported, "INDEX_GUARD_ACTION_STRUCTURE"):
            self.checks(graph, task["id"])
        graph = guarded_list(task)
        graph["WFWorkflowActions"][9]["WFWorkflowActionParameters"]["UUID"] = identifier(1).upper()
        with self.assertRaisesRegex(validator.Unsupported, "DUPLICATE_UUID"):
            self.checks(graph, task["id"])

    def test_v2_final_nothing_is_neutral_but_an_extra_effect_is_not(self):
        for index in (3, 4, 5):
            task = self.suite["tasks"][index]
            graph = self.graph(index)
            graph["WFWorkflowActions"].append(action("nothing", {}))
            self.assertTrue(all(self.checks(graph, task["id"]).values()))
            graph["WFWorkflowActions"].append(action("showresult", {"Text": "extra"}))
            self.assertFalse(self.checks(graph, task["id"])["oracle.side_effects"])

    def test_malformed_legacy_exponent_is_not_accepted_as_a_number(self):
        for text in ("2.29323196e", "6.5292533e", "2.29323196e+", "2.29323196e-",
                     "1e+-2", "1e2junk", " 1e2", "+1e2", "NaN", "Infinity"):
            with self.assertRaisesRegex(validator.Unsupported, "NUMBER_TEXT_GRAMMAR"):
                validator.number(text)

    def test_complete_legacy_scientific_operands_preserve_exact_values(self):
        for text, expected in (("2.29323196e+08", 229323196), ("6.5292533e+07", 65292533),
                               ("-1.25E+02", -125), ("1.25e-02", Decimal("0.0125"))):
            with self.subTest(text=text):
                self.assertEqual(validator.number(text), Decimal(expected))
        for text in ("9.007199254740993e+15", "-9.007199254740993e+15"):
            with self.assertRaisesRegex(validator.Unsupported, "NUMBER_EXACT_RANGE"):
                validator.number(text)
        for text in ("1e309", "1e-309", "1e999999"):
            with self.assertRaisesRegex(validator.Unsupported, "NUMBER_EXPONENT_RANGE"):
                validator.number(text)
        with self.assertRaisesRegex(validator.Unsupported, "NUMBER_TEXT_LIMIT"):
            validator.number("1"*129)

    def test_v2_compiler_environment_has_attempt_local_tmpdir_without_inherited_secrets(self):
        with tempfile.TemporaryDirectory() as temporary:
            directory = Path(temporary)
            with mock.patch.dict(validator.os.environ, {
                    "ANTHROPIC_API_KEY": "synthetic-not-a-credential", "TMPDIR": "outside",
                    "PATH": "outside", "HOME": "outside", "LANG": "C"}, clear=True):
                environment = validator.compiler_environment(directory, bundle.V2_PROFILE)
            self.assertEqual(set(environment), {"LANG", "TEMP", "TMP", "TMPDIR"})
            self.assertEqual(environment["TMPDIR"], str(directory/"compiler-temp"))
            self.assertEqual(environment["TEMP"], environment["TMPDIR"])
            self.assertEqual(environment["TMP"], environment["TMPDIR"])

    def test_v2_native_validation_receipt_uses_selected_profile_and_isolated_environment(self):
        with tempfile.TemporaryDirectory() as temporary:
            directory, task, request, fake_bundle = self.inventory_attempt(Path(temporary))
            fake_bundle["manifest"]["profile"] = bundle.V2_PROFILE
            graph = plistlib.dumps(self.graph(0), fmt=plistlib.FMT_BINARY)
            calls = []
            def compile_control(command, **kwargs):
                calls.append((command, kwargs))
                self.assertEqual(kwargs["env"]["TMPDIR"], str(directory/"compiler-temp"))
                self.assertNotIn("ANTHROPIC_API_KEY", kwargs["env"])
                (directory/"output.shortcut").write_bytes(graph)
                return validator.subprocess.CompletedProcess(command, 0)
            with mock.patch.object(validator, "validation_request", return_value=task), \
                 mock.patch.object(validator, "claim_request", return_value=directory), \
                 mock.patch.object(validator.subprocess, "run", side_effect=compile_control):
                reply = validator.evaluate_request(request, fake_bundle)
            self.assertTrue(all(check["passed"] for check in reply["checks"]))
            self.assertEqual(len(calls), 1)
            receipt = json.loads((directory/"validator-receipt.json").read_bytes())
            self.assertEqual(receipt["evaluation_profile"], bundle.V2_PROFILE)
            self.assertEqual(receipt["compiler_environment_profile"], "isolated-defaults-linux/v2")



if __name__ == "__main__":
    unittest.main()
