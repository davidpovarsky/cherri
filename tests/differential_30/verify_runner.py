#!/usr/bin/env python3
"""Adversarial checks on actual compiled fixture trees, not authored plists."""
import argparse
import copy
import importlib.util
import json
import pathlib
import plistlib
import uuid

spec = importlib.util.spec_from_file_location("differential_run", pathlib.Path(__file__).with_name("run.py"))
run = importlib.util.module_from_spec(spec)
spec.loader.exec_module(run)


def renamed_fixture(source):
    """Rename identities in a clean compiler-produced positive control only."""
    renamed = copy.deepcopy(source)
    mapping = {}
    for action in renamed["WFWorkflowActions"]:
        params = action.get("WFWorkflowActionParameters", {})
        for key in ("UUID", "GroupingIdentifier"):
            if key in params:
                mapping.setdefault(params[key], str(uuid.uuid4()))

    def walk(node):
        if isinstance(node, dict):
            for key, value in node.items():
                if key in run.IDENTITY_KEYS and isinstance(value, str) and value in mapping:
                    node[key] = mapping[value]
                else:
                    walk(value)
        elif isinstance(node, list):
            for value in node:
                walk(value)
    walk(renamed)
    return renamed, mapping


def action_output_tokens(node, path=()):
    """Test-side traversal independent of the runner's native-path classifier."""
    if isinstance(node, dict):
        if node.get("Type") == "ActionOutput" and isinstance(node.get("OutputUUID"), str):
            yield path, node
        for key, value in node.items():
            yield from action_output_tokens(value, path + (key,))
    elif isinstance(node, list):
        for index, value in enumerate(node):
            yield from action_output_tokens(value, path + (index,))


def error_kinds(document):
    return {error["kind"] for error in run.graph_audit(document)["errors"]}


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("receipt_dir", type=pathlib.Path)
    parser.add_argument("--out", type=pathlib.Path, required=True)
    args = parser.parse_args()
    receipt = json.loads((args.receipt_dir / "results.json").read_text())
    rows = {(row["fixture_id"], row["lane"]): row for row in receipt["compilations"]}
    checks = []

    def document(ident, lane="fork_v2"):
        return plistlib.loads(pathlib.Path(rows[(ident, lane)]["artifact"]).read_bytes())

    def check(name, condition, **evidence):
        checks.append({"id": name, "status": "PASS" if condition else "FAIL", **evidence})

    source = document("02_case_variants")
    check("exact_self_equality", not run.exact_diffs(source, source, set()))
    renamed, mapping = renamed_fixture(source)
    scope = run.identity_scope(source, renamed)
    check("bijective_uuid_rename_keeps_graph", scope is not None and run.graph_audit(renamed)["valid"]
          and not run.exact_diffs(source, renamed, set(), scope)
          and bool(run.exact_diffs(source, renamed, set())))

    broken = copy.deepcopy(source)
    output = broken["WFWorkflowActions"][-1]
    refs = run.oracle.collect_references(output)
    check("reference_fixture_precondition", bool(refs))

    def break_first_ref(node):
        if isinstance(node, dict):
            if "OutputUUID" in node:
                node["OutputUUID"] = str(uuid.uuid4())
                return True
            return any(break_first_ref(value) for value in node.values())
        if isinstance(node, list):
            return any(break_first_ref(value) for value in node)
        return False
    break_first_ref(output)
    check("dangling_reference_rejected", not run.graph_audit(broken)["valid"]
          and bool(run.exact_diffs(source, broken, set(), run.identity_scope(source, broken))))

    collided = copy.deepcopy(source)
    first = collided["WFWorkflowActions"][0]["WFWorkflowActionParameters"]["UUID"]
    collided["WFWorkflowActions"][1]["WFWorkflowActionParameters"]["UUID"] = first
    check("uuid_collision_rejected", not run.graph_audit(collided)["valid"]
          and run.identity_scope(source, collided) is None)

    numeric = document("15_numeric_list_statistics")
    numeric_changed = copy.deepcopy(numeric)
    number_action = next(a for a in numeric_changed["WFWorkflowActions"]
                         if a["WFWorkflowActionIdentifier"] == "is.workflow.actions.number")
    number_action["WFWorkflowActionParameters"]["WFNumberActionNumber"] = "3"
    check("integer_text_change_retained", any(d["kind"] == "type_change"
          for d in run.exact_diffs(numeric, numeric_changed, set())))
    check("bool_integer_change_retained", run.exact_diffs(True, 1, set())[0]["kind"] == "type_change")
    check("integer_real_change_retained", run.exact_diffs(1, 1.0, set())[0]["kind"] == "type_change")

    metadata = copy.deepcopy(source)
    del metadata["WFWorkflowClientVersion"]
    diff = run.exact_diffs(source, metadata, set())
    check("metadata_missing_not_normalized", bool(diff)
          and all(d["category"] == "workflow_metadata" for d in diff))

    display = copy.deepcopy(source)
    display["WFWorkflowActions"][0]["WFWorkflowActionParameters"]["CustomOutputName"] = "Different name"
    diff = run.exact_diffs(source, display, set())
    check("display_difference_retained", bool(diff)
          and any(d["category"] == "display_candidate" for d in diff))

    original = document("04_regex_replace", "original")
    omitted = copy.deepcopy(original)
    omitted["WFWorkflowActions"][0]["WFWorkflowActionParameters"].pop("WFReplaceTextCaseSensitive")
    diff = run.exact_diffs(original, omitted, {(0, "WFReplaceTextCaseSensitive")})
    check("default_omission_retained", bool(diff)
          and diff[0]["category"] == "catalog_static_or_default_candidate")

    shortened = copy.deepcopy(source)
    shortened["WFWorkflowActions"].pop(0)
    check("action_removal_retained", bool(run.exact_diffs(source, shortened, set())))

    literal = copy.deepcopy(source)
    literal["WFWorkflowActions"][0]["WFWorkflowActionParameters"]["text"] = first
    literal2 = copy.deepcopy(literal)
    literal2["WFWorkflowActions"][0]["WFWorkflowActionParameters"]["text"] = str(uuid.uuid4())
    check("uuid_shaped_literal_remains_exact", bool(run.exact_diffs(
        literal, literal2, set(), run.identity_scope(literal, literal2))))

    nested = document("25_nested_repeat")
    check("native_nested_control_fixture_valid", run.graph_audit(nested)["valid"])
    nested_broken = copy.deepcopy(nested)
    controls = [a for a in nested_broken["WFWorkflowActions"]
                if a.get("WFWorkflowActionParameters", {}).get("WFControlFlowMode") == 0]
    controls[1]["WFWorkflowActionParameters"]["GroupingIdentifier"] = controls[0]["WFWorkflowActionParameters"]["GroupingIdentifier"]
    check("nested_group_collision_rejected", not run.graph_audit(nested_broken)["valid"])
    missing_group = copy.deepcopy(nested)
    next(a for a in missing_group["WFWorkflowActions"]
         if "WFControlFlowMode" in a.get("WFWorkflowActionParameters", {}))["WFWorkflowActionParameters"].pop("GroupingIdentifier")
    check("missing_control_group_rejected", not run.graph_audit(missing_group)["valid"])
    # A native-looking leaf name is not sufficient to authorize alpha.
    # Literal payload fields are inserted only after the clean positive-control
    # rename, so the test never accidentally renames its own authored literals.
    for key in ("UUID", "OutputUUID", "GroupingIdentifier"):
        left = copy.deepcopy(source)
        right = copy.deepcopy(renamed)
        left_value = first if key != "GroupingIdentifier" else "red"
        right_value = mapping[first] if key != "GroupingIdentifier" else "blue"
        left["WFWorkflowActions"][0]["WFWorkflowActionParameters"]["AuthoredPayload"] = {key: left_value}
        right["WFWorkflowActions"][0]["WFWorkflowActionParameters"]["AuthoredPayload"] = {key: right_value}
        scoped = run.identity_scope(left, right)
        differences = run.exact_diffs(left, right, set(), scoped)
        path = ["WFWorkflowActions", 0, "WFWorkflowActionParameters", "AuthoredPayload", key]
        check("authored_" + key + "_field_remains_exact", scoped is not None
              and any(d["path"] == path for d in differences),
              retained_paths=[d["path"] for d in differences])

    top_group = copy.deepcopy(source)
    top_group_renamed = copy.deepcopy(source)
    top_group["WFWorkflowActions"][0]["WFWorkflowActionParameters"]["GroupingIdentifier"] = "red"
    top_group_renamed["WFWorkflowActions"][0]["WFWorkflowActionParameters"]["GroupingIdentifier"] = "blue"
    check("noncontrol_group_field_is_literal", bool(run.exact_diffs(top_group, top_group_renamed, set(),
          run.identity_scope(top_group, top_group_renamed))))

    header = copy.deepcopy(source)
    header2 = copy.deepcopy(renamed)
    header["AuthoredMetadata"] = {"UUID": first, "GroupingIdentifier": "red"}
    header2["AuthoredMetadata"] = {"UUID": mapping[first], "GroupingIdentifier": "blue"}
    differences = run.exact_diffs(header, header2, set(), run.identity_scope(header, header2))
    check("workflow_metadata_identity_names_remain_exact",
          len([d for d in differences if d["path"][0] == "AuthoredMetadata"]) == 2)

    lookalike = copy.deepcopy(source)
    lookalike2 = copy.deepcopy(renamed)
    lookalike["WFWorkflowActions"][0]["WFWorkflowActionParameters"]["AuthoredPayload"] = {
        "Type": "ActionOutput", "OutputUUID": first}
    lookalike2["WFWorkflowActions"][0]["WFWorkflowActionParameters"]["AuthoredPayload"] = {
        "Type": "ActionOutput", "OutputUUID": mapping[first]}
    check("unwrapped_actionoutput_lookalike_remains_literal", bool(run.exact_diffs(
          lookalike, lookalike2, set(), run.identity_scope(lookalike, lookalike2))))

    literal_reference = copy.deepcopy(source)
    literal_reference["WFWorkflowActions"][0]["WFWorkflowActionParameters"]["AuthoredPayload"] = {
        "OutputUUID": "user-authored-literal"}
    check("literal_outputuuid_does_not_create_graph_edge", run.graph_audit(literal_reference)["valid"]
          and len(run.graph_audit(literal_reference)["references"]) == len(run.graph_audit(source)["references"]))

    # A real typed reference must preserve its producer, even if a different
    # preceding producer would also make the mutated graph well-formed.
    retargeted = copy.deepcopy(source)
    _, token = next(action_output_tokens(retargeted))
    original_target = token["OutputUUID"]
    token["OutputUUID"] = next(a["WFWorkflowActionParameters"]["UUID"]
                              for a in retargeted["WFWorkflowActions"]
                              if a.get("WFWorkflowActionParameters", {}).get("UUID") not in (None, original_target))
    check("retarget_to_existing_producer_remains_different", run.graph_audit(retargeted)["valid"]
          and bool(run.exact_diffs(source, retargeted, set(), run.identity_scope(source, retargeted))))

    self_reference = copy.deepcopy(source)
    path, token = next(action_output_tokens(self_reference))
    consumer = path[1]
    parameters = self_reference["WFWorkflowActions"][consumer]["WFWorkflowActionParameters"]
    parameters.setdefault("UUID", str(uuid.uuid4()))
    token["OutputUUID"] = parameters["UUID"]
    kinds = error_kinds(self_reference)
    check("self_reference_rejected", "self_reference" in kinds
          and run.identity_scope(self_reference, self_reference) is None, graph_error_kinds=sorted(kinds))

    forward_reference = copy.deepcopy(source)
    _, token = next(action_output_tokens(forward_reference))
    producer = next(i for i, action in enumerate(forward_reference["WFWorkflowActions"])
                    if action.get("WFWorkflowActionParameters", {}).get("UUID") == token["OutputUUID"])
    moved = forward_reference["WFWorkflowActions"].pop(producer)
    forward_reference["WFWorkflowActions"].append(moved)
    kinds = error_kinds(forward_reference)
    check("forward_reference_rejected", "forward_reference" in kinds
          and run.identity_scope(forward_reference, forward_reference) is None, graph_error_kinds=sorted(kinds))

    malformed = copy.deepcopy(source)
    _, token = next(action_output_tokens(malformed))
    token["Type"] = "Variable"
    check("malformed_native_reference_type_rejected", "invalid_reference_type" in error_kinds(malformed)
          and run.identity_scope(malformed, malformed) is None)

    null_producer = copy.deepcopy(source)
    null_producer["WFWorkflowActions"][0]["WFWorkflowActionParameters"]["UUID"] = None
    check("null_native_producer_rejected", "invalid_producer" in error_kinds(null_producer)
          and run.identity_scope(null_producer, null_producer) is None)

    sequential = document("22_text_predicates")
    starts = [a for a in sequential["WFWorkflowActions"]
              if a.get("WFWorkflowActionParameters", {}).get("WFControlFlowMode") == 0]
    check("sequential_group_fixture_precondition", len(starts) >= 2 and run.graph_audit(sequential)["valid"])
    first_group = starts[0]["WFWorkflowActionParameters"]["GroupingIdentifier"]
    second_group = starts[1]["WFWorkflowActionParameters"]["GroupingIdentifier"]
    reused = copy.deepcopy(sequential)
    for action in reused["WFWorkflowActions"]:
        params = action.get("WFWorkflowActionParameters", {})
        if params.get("GroupingIdentifier") == second_group:
            params["GroupingIdentifier"] = first_group
    kinds = error_kinds(reused)
    check("group_reuse_after_closure_rejected", "duplicate_group_start" in kinds
          and run.identity_scope(reused, reused) is None, graph_error_kinds=sorted(kinds))

    wrong_end = copy.deepcopy(nested)
    next(a for a in wrong_end["WFWorkflowActions"]
         if a.get("WFWorkflowActionParameters", {}).get("WFControlFlowMode") == 2
         )["WFWorkflowActionIdentifier"] = "is.workflow.actions.conditional"
    kinds = error_kinds(wrong_end)
    check("control_end_action_identifier_mismatch_rejected", "control_action_mismatch" in kinds
          and run.identity_scope(wrong_end, wrong_end) is None, graph_error_kinds=sorted(kinds))

    wrong_middle = document("20_text_branch")
    next(a for a in wrong_middle["WFWorkflowActions"]
         if a.get("WFWorkflowActionParameters", {}).get("WFControlFlowMode") == 1
         )["WFWorkflowActionIdentifier"] = "is.workflow.actions.choosefrommenu"
    kinds = error_kinds(wrong_middle)
    check("control_middle_action_identifier_mismatch_rejected", "control_action_mismatch" in kinds
          and run.identity_scope(wrong_middle, wrong_middle) is None, graph_error_kinds=sorted(kinds))

    bool_mode = copy.deepcopy(nested)
    next(a for a in bool_mode["WFWorkflowActions"]
         if a.get("WFWorkflowActionParameters", {}).get("WFControlFlowMode") == 0
         )["WFWorkflowActionParameters"]["WFControlFlowMode"] = False
    check("bool_control_mode_rejected", "unknown_control_mode" in error_kinds(bool_mode))

    renamed_nested, _ = renamed_fixture(nested)
    scope = run.identity_scope(nested, renamed_nested)
    check("native_nested_group_rename_keeps_graph", scope is not None
          and not run.exact_diffs(nested, renamed_nested, set(), scope))

    quantity = document("39_timer_dynamic_qty")
    renamed_quantity, _ = renamed_fixture(quantity)
    scope = run.identity_scope(quantity, renamed_quantity)
    check("native_bare_quantity_reference_rename_keeps_graph", scope is not None
          and not run.exact_diffs(quantity, renamed_quantity, set(), scope)
          and any(ref["path"].endswith("Magnitude.OutputUUID") for ref in run.graph_audit(quantity)["references"]))

    check("native_repeat_and_foreach_tokens_remain_valid",
          all(run.graph_audit(document(ident))["valid"] for ident in
              ("23_repeat_sum", "24_foreach_text", "25_nested_repeat", "26_nested_foreach", "27_foreach_filter")))
    result = {"status": "PASS" if all(c["status"] == "PASS" for c in checks) else "FAIL", "checks": checks, "source_receipt": str(args.receipt_dir),
              "method": "Mutations of compiler-produced fixture trees only; no Shortcut plist generation."}
    run.write_json(args.out, result)
    print(json.dumps({"status": result["status"], "checks": len(checks), "receipt": str(args.out)}))
    return 0 if result["status"] == "PASS" else 1


if __name__ == "__main__":
    raise SystemExit(main())
