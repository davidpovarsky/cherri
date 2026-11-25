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

    def check(name, condition):
        checks.append({"id": name, "status": "PASS" if condition else "FAIL"})
        if not condition:
            raise AssertionError(name)

    source = document("02_case_variants")
    check("exact_self_equality", not run.exact_diffs(source, source, set()))
    renamed = copy.deepcopy(source)
    mapping = {}
    for action in renamed["WFWorkflowActions"]:
        params = action.get("WFWorkflowActionParameters", {})
        for key in ("UUID", "GroupingIdentifier"):
            if key in params:
                mapping.setdefault(params[key], str(uuid.uuid4()))

    def rename(node):
        if isinstance(node, dict):
            for key, value in node.items():
                if key in run.IDENTITY_KEYS and value in mapping:
                    node[key] = mapping[value]
                else:
                    rename(value)
        elif isinstance(node, list):
            for value in node:
                rename(value)
    rename(renamed)
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
    result = {"status": "PASS", "checks": checks, "source_receipt": str(args.receipt_dir),
              "method": "Mutations of compiler-produced fixture trees only; no Shortcut plist generation."}
    run.write_json(args.out, result)
    print(json.dumps({"status": result["status"], "checks": len(checks), "receipt": str(args.out)}))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
