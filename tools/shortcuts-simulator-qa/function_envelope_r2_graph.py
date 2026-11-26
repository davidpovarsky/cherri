"""Independent authored R2 graph for the versioned function diagnostic.

The fixed historical expected program is authored code, not an observed plist.
R2 changes only its empty argument array and the self-call/trace identities.
The historical validator and b82 artifact remain unchanged and independently
testable. This diagnostic still preserves the incomplete existing dispatcher.
"""
import argparse
import hashlib
import json
import pathlib
import plistlib
import xml.etree.ElementTree as ET
from xml.parsers.expat import ExpatError

import function_envelope_graph as historical
from native_comparison_graph import verify_equality_region
from tagged_carrier_graph import (
    ACTION_PREFIX, attachment, variable, text_state, difference,
)

FIXTURE = "qa-function-envelope-presence-v2-r2"
WORKFLOW = "CherriFunctionEnvelopePresenceProbeR2"
SOURCE_SHA256 = "8c044ebef8e0ecd1481a99eb030ef843de2a9ddc4b4fb129545501d1bdb28be1"
CONTRACT_SHA256 = "f27cc210f0656118d650d53a05f14d7ae2843e600c78b84a9b0e7823e73673de"
PREDECESSOR_UNSIGNED_SHA256 = "54889840e546acc60ffe05c8399e1d4bd54f0dc846ec3fd520b4fffc0aa6c7d0"


def require(ok, reason):
    if not ok:
        raise ValueError("FUNCTION_PRESENCE_R2_GRAPH_" + reason)


def expected_program():
    # This returns a fresh, source-authored program. Never read or transform an
    # observed workflow to manufacture an expected graph.
    program = historical.expected_program()
    sender = program.roles["sender_probeZero"]
    rows = program.actions[sender["payload"]]["WFWorkflowActionParameters"][
        "WFItems"]["Value"]["WFDictionaryFieldValueItems"]
    array_rows = [row for row in rows
                  if row["WFKey"] == text_state("arguments")]
    require(len(array_rows) == 1, "AUTHORED_EMPTY_ARGUMENT_ROW")
    row = array_rows[0]
    require(row["WFItemType"] == 2 and row["WFValue"] == {
        "WFSerializationType": "WFArrayParameterState"},
        "AUTHORED_PREDECESSOR_EMPTY_SHAPE")
    row["WFValue"] = {"WFSerializationType": "WFArrayParameterState", "Value": []}

    for case in ("probeZero", "probeDefault", "probeOptional"):
        call = program.roles["sender_" + case]["call"]
        parameters = program.actions[call]["WFWorkflowActionParameters"]
        require(parameters["WFWorkflowName"] == historical.WORKFLOW
                and parameters["WFWorkflow"] == {
                    "isSelf": True, "workflowName": historical.WORKFLOW},
                "AUTHORED_PREDECESSOR_TARGET")
        parameters["WFWorkflowName"] = WORKFLOW
        parameters["WFWorkflow"] = {"isSelf": True, "workflowName": WORKFLOW}

    # Rebuild UTF-16 attachment ranges from the authored text after extending
    # runId. The actual graph never receives a matching rewrite or normalization.
    trace = program.roles["traceEnvelope"]
    parameters = program.actions[trace]["WFWorkflowActionParameters"]
    value = parameters["WFTextActionText"]["Value"]
    old = '"runId":"' + historical.FIXTURE + '"'
    require(value["string"].count(old) == 2
            and len(value["attachmentsByRange"]) == 1,
            "AUTHORED_PREDECESSOR_TRACE")
    references = [attachment(ref) for ref in value["attachmentsByRange"].values()]
    parameters["WFTextActionText"] = text_state(
        value["string"].replace(old, '"runId":"' + FIXTURE + '"'), references)
    require(len(program.actions) == 379 and len(program.comparisons) == 12,
            "AUTHORED_INVENTORY")
    return program


def validate_actions(actions):
    expected = expected_program()
    actual = historical.normalized_actions(actions, expected)
    mismatch = difference(actual, expected.actions)
    require(mismatch is None, "FULL_ACTION_CONTRACT:" + str(mismatch))
    for comparison in expected.comparisons:
        verify_equality_region(
            actions, comparison["start"], comparison["guard"],
            actions[comparison["left"]], right=comparison["right"], require=require)
    return {
        "status": "ENGINEERING_SOURCE_GRAPH_VERIFIED",
        "fixtureId": FIXTURE,
        "workflowName": WORKFLOW,
        "actionCount": len(actions),
        "exactKeyComparisons": 12,
        "functionCases": 3,
        "normalization": ["producer/control UUID alpha identity",
                          "action/output display names"],
        "roles": expected.roles,
        "explicitEmptyArgumentValueVerified": True,
        "versionedSelfCallsVerified": 3,
        "versionedCollectorRunIdsVerified": 2,
        "knownExistingIncompleteDispatcherPreserved": True,
        "completeEnvelopeAdmissionVerified": False,
        "noInputLaunchVerified": False,
        "appleExecutionAuthenticated": False,
        "formalAcceptanceGranted": False,
        "predecessorUnsignedSha256": PREDECESSOR_UNSIGNED_SHA256,
        "predecessorCurrentCompilerCredit": False,
    }


def validate_compiled(source, contract, native):
    require(isinstance(source, bytes)
            and hashlib.sha256(source).hexdigest() == SOURCE_SHA256, "SOURCE_SHA256")
    require(isinstance(contract, bytes)
            and hashlib.sha256(contract).hexdigest() == CONTRACT_SHA256, "CONTRACT_SHA256")
    require(isinstance(native, bytes) and 0 < len(native) <= 1024 * 1024, "NATIVE_BYTES")
    try:
        xml = ET.fromstring(native)
        require(xml.tag == "plist" and xml.attrib == {"version": "1.0"}, "XML_PLIST")
        for dictionary in xml.iter("dict"):
            children = list(dictionary)
            require(len(children) % 2 == 0, "XML_DICTIONARY")
            keys = []
            for index in range(0, len(children), 2):
                require(children[index].tag == "key", "XML_KEY")
                keys.append(children[index].text or "")
            require(len(keys) == len(set(keys)), "XML_DUPLICATE_KEY")
        document = plistlib.loads(native)
    except (ValueError, TypeError, plistlib.InvalidFileException,
            ExpatError, ET.ParseError) as exc:
        raise ValueError("FUNCTION_PRESENCE_R2_GRAPH_NATIVE_PLIST:" + str(exc)) from exc
    require(isinstance(document, dict), "WORKFLOW")
    result = validate_actions(document.get("WFWorkflowActions"))
    result.update(
        sourceAuthenticated=True, sourceSha256=SOURCE_SHA256,
        expectedContractSha256=CONTRACT_SHA256,
        unsignedSha256=hashlib.sha256(native).hexdigest(), unsignedBytes=len(native))
    return result


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--source", required=True)
    parser.add_argument("--contract", required=True)
    parser.add_argument("--native", required=True)
    parser.add_argument("--out", required=True)
    args = parser.parse_args()
    result = validate_compiled(
        pathlib.Path(args.source).read_bytes(),
        pathlib.Path(args.contract).read_bytes(),
        pathlib.Path(args.native).read_bytes())
    with pathlib.Path(args.out).open("x", encoding="utf-8") as output:
        output.write(json.dumps(result, indent=2) + "\n")


if __name__ == "__main__":
    main()
