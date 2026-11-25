#!/usr/bin/env python3
"""Read-only Q41 wire oracle from captured Apple states and source binding roles."""
import argparse
import copy
import hashlib
import json
from pathlib import Path
import plistlib

HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[2]
STATE_FILE = ROOT / "docs/apple-runtime/macos-27/encoding-table.json"
SLOTS = [
    ("parts", "is.workflow.actions.text.split", "text", "Variable", "sample"),
    ("parts", "is.workflow.actions.text.split", "WFTextCustomSeparator", "ActionOutput", "separator"),
    ("upper", "is.workflow.actions.text.changecase", "text", "ActionOutput", "first"),
    ("lower", "is.workflow.actions.text.changecase", "text", "ActionOutput", "last"),
    ("title", "is.workflow.actions.text.changecase", "text", "ActionOutput", "first"),
    ("joined", "is.workflow.actions.text.combine", "WFTextCustomSeparator", "ActionOutput", "glue"),
    ("urls", "is.workflow.actions.detect.link", "WFInput", "Variable", "linkText"),
    ("host", "is.workflow.actions.geturlcomponent", "WFURL", "ActionOutput", "firstURL"),
]
PRODUCERS = {"separator": "is.workflow.actions.gettext", "glue": "is.workflow.actions.gettext",
             "first": "is.workflow.actions.gettext", "last": "is.workflow.actions.gettext",
             "firstURL": "is.workflow.actions.gettext", "parts": "is.workflow.actions.text.split",
             "firstItem": "is.workflow.actions.getitemfromlist", "lastItem": "is.workflow.actions.getitemfromlist",
             "firstURLItem": "is.workflow.actions.getitemfromlist"}
INITIALIZERS = {"sample": "alpha,BETA", "linkText": "https://example.org/path"}


def sha(path):
    return hashlib.sha256(Path(path).read_bytes()).hexdigest()


def unique_output(actions, name):
    matches = [(i, a) for i, a in enumerate(actions)
               if a["WFWorkflowActionParameters"].get("CustomOutputName") == name]
    if len(matches) != 1:
        raise ValueError("Source output binding is not unique: " + name)
    return matches[0]


def expected_reference(actions, consumer, kind, name, states, state):
    sample = "variable:MyVar" if kind == "Variable" else "variable:ActionOutput"
    expected = copy.deepcopy(states[state][sample])
    body = expected["Value"]
    if state == "WFVariableStringParameterState":
        if expected["WFSerializationType"] != "WFTextTokenString" or body["string"] != "\ufffc":
            raise ValueError("Captured Apple text state shape changed")
        token = body["attachmentsByRange"]["{0, 1}"]
    else:
        if expected["WFSerializationType"] != "WFTextTokenAttachment":
            raise ValueError("Captured Apple variable state shape changed")
        token = body
    if kind == "Variable":
        stores = [(i, a) for i, a in enumerate(actions)
                  if a["WFWorkflowActionIdentifier"] == "is.workflow.actions.setvariable"
                  and a["WFWorkflowActionParameters"].get("WFVariableName") == name]
        if len(stores) != 1 or stores[0][0] >= consumer:
            raise ValueError("Mutable source store missing or late: " + name)
        position, store = stores[0]
        source_uuid = store["WFWorkflowActionParameters"]["WFInput"]["Value"]["OutputUUID"]
        initial = [(i, a) for i, a in enumerate(actions)
                   if a["WFWorkflowActionParameters"].get("UUID") == source_uuid]
        if (len(initial) != 1 or initial[0][0] >= position
                or initial[0][1]["WFWorkflowActionIdentifier"] != "is.workflow.actions.gettext"
                or initial[0][1]["WFWorkflowActionParameters"].get("WFTextActionText") != INITIALIZERS[name]):
            raise ValueError("Mutable initializer/source edge changed: " + name)
        token["VariableName"] = name
    else:
        position, producer = unique_output(actions, name)
        if position >= consumer or producer["WFWorkflowActionIdentifier"] != PRODUCERS[name]:
            raise ValueError("Producer role or evaluation order changed: " + name)
        params = producer["WFWorkflowActionParameters"]
        if name in ("separator", "glue") and params.get("WFTextActionText") != ("," if name == "separator" else "::"):
            raise ValueError("Runtime separator/glue literal changed")
        if name in ("first", "last", "firstURL"):
            item = {"first": "firstItem", "last": "lastItem", "firstURL": "firstURLItem"}[name]
            nested = expected_reference(actions, position, "ActionOutput", item, states, "WFVariableStringParameterState")
            if params.get("WFTextActionText") != nested:
                raise ValueError("Explicit Text materialization lost its list-item reference: " + name)
        token["OutputUUID"] = params["UUID"]
        token["OutputName"] = name
    return expected


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("run", type=Path)
    parser.add_argument("--out", required=True, type=Path)
    args = parser.parse_args()
    receipt = json.loads((args.run / "results.json").read_text())
    states = json.loads(STATE_FILE.read_text())["stateClasses"]
    lanes = []
    for row in receipt["compilations"]:
        checks = []
        lane = {"lane": row["lane"], "artifact": row["artifact"], "artifactSHA256": row.get("artifact_sha256"),
                "checks": checks, "graphValid": row.get("graph", {}).get("valid", False)}
        try:
            if row["status"] != "COMPILED" or not lane["graphValid"] or sha(row["artifact"]) != row["artifact_sha256"]:
                raise ValueError("Compilation, graph or artifact hash is not valid")
            actions = plistlib.loads(Path(row["artifact"]).read_bytes())["WFWorkflowActions"]
            for output, identifier, key, kind, source in SLOTS:
                consumer, action = unique_output(actions, output)
                if action["WFWorkflowActionIdentifier"] != identifier:
                    raise ValueError("Consumer role changed: " + output)
                want = expected_reference(actions, consumer, kind, source, states, "WFVariableStringParameterState")
                actual = action["WFWorkflowActionParameters"].get(key)
                checks.append({"output": output, "key": key, "source": source, "referenceKind": kind,
                               "actual": actual, "expected": want, "passed": actual == want})
            consumer, action = unique_output(actions, "joined")
            want = expected_reference(actions, consumer, "ActionOutput", "parts", states, "WFVariableParameterState")
            actual = action["WFWorkflowActionParameters"].get("text")
            checks.append({"output": "joined", "key": "text", "source": "parts", "role": "list-input-attachment-control",
                           "actual": actual, "expected": want, "passed": actual == want})
        except (KeyError, TypeError, ValueError, OSError) as error:
            lane["error"] = str(error)
        lane["passed"] = len(checks) == 9 and all(x["passed"] for x in checks) and "error" not in lane
        lanes.append(lane)
    result = {"schemaVersion": 1, "compilerReceiptSHA256": sha(args.run / "results.json"),
              "nativeStatePath": str(STATE_FILE), "nativeStateSHA256": sha(STATE_FILE),
              "oracleSHA256": sha(__file__), "manifestSHA256": receipt["manifest_sha256"],
              "scope": "Captured Apple native text envelopes plus source producer/variable identity; primary join list remains an attachment",
              "lanes": lanes, "passed": bool(lanes) and all(x["passed"] for x in lanes), "runtimeStatus": "NOT_RUN"}
    args.out.write_text(json.dumps(result, indent=2) + "\n")
    print(json.dumps({"passed": result["passed"], "lanes": [{"lane": x["lane"], "passed": x["passed"],
          "failedSlots": sum(not c["passed"] for c in x["checks"]), "error": x.get("error")} for x in lanes]}))
    return 0 if result["passed"] else 1


if __name__ == "__main__":
    raise SystemExit(main())
