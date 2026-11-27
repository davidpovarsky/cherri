"""Bounded integrity checks for original macOS WorkflowKit action observations.

The stream validator binds complete original bytes, model/action UUIDs, native
classes, callback ordering and terminal state. It supplies no iOS, signed-import
or acceptance tier. Case-specific checks must still use their complete original
scenario oracle. A completion-time input is not a will-run input observation.
"""
from __future__ import annotations

import hashlib
import json
import math
import plistlib
import re

NIL = {"present": False, "nativeClass": None, "kind": "NIL"}
UUID = re.compile(r"[0-9A-F]{8}(?:-[0-9A-F]{4}){3}-[0-9A-F]{12}")
TEXT_SOURCE_SHA = "d13f780653b9c04d9951b9acf561dabbff1a69eb8e7a503119f772b9cc9c3b71"
TEXT_PLIST_SHA = "7389b4a423e5df2afd6ea139bf15088a13d3e5ff3ca1e26ccf566c1141547da8"
SCOPE = ("sourceProgramsChanged", "signedImportExercised",
         "iOSDeviceExercised", "formalAcceptanceGranted")


def require(value, code):
    if not value:
        raise ValueError(code)


def integer(value, minimum=0, maximum=64):
    return type(value) is int and minimum <= value <= maximum


def text(value, maximum=4096):
    return type(value) is str and len(value.encode("utf-16-le", errors="surrogatepass")) // 2 <= maximum


def native_value(value, depth=0, budget=None):
    """Validate captured native shape without converting it to a desired type."""
    if budget is None:
        budget = [0]
    budget[0] += 1
    require(depth <= 6 and budget[0] <= 1024, "NATIVE_VALUE_BOUND")
    require(type(value) is dict, "NATIVE_VALUE_OBJECT")
    kind = value.get("kind")
    if kind == "NIL":
        require(set(value) == {"present", "nativeClass", "kind"}
                and value["present"] is False and value["nativeClass"] is None,
                "ACTUAL_NIL_SHAPE")
        return
    base = {"present", "nativeClass", "kind"}
    require(value.get("present") is True and text(value.get("nativeClass"), 256)
            and len(value["nativeClass"]) > 0, "NATIVE_VALUE_CLASS")
    extra = {
        "NSNULL": set(),
        "UNOBSERVED": set(),
        "NSNUMBER": {"cfTypeId", "cfBooleanTypeId", "value"},
        "CFBOOLEAN": {"cfTypeId", "cfBooleanTypeId", "value"},
        "NSSTRING": {"value"},
        "NSATTRIBUTEDSTRING": {"plainString"},
        "WFNUMBERCONTENTITEM": {"number"},
        "WFCONTENTCOLLECTION": {"itemContainerClass", "cardinality", "items"},
        "NSARRAY": {"cardinality", "items"},
        "NSORDEREDSET": {"cardinality", "items"},
        "NSDICTIONARY": {"cardinality", "entries"},
        "WFSTRINGCONTENTITEM": {"representationSource", "representations"},
        "WFDICTIONARYCONTENTITEM": {"representationSource", "representations"},
        "WFCONTENTITEM": {"representationSource", "representations"},
        "WFOBJECTREPRESENTATION": {"observationSource", "object"},
        "WFOBJECTTYPE": {"declaredClassName"},
    }
    require(kind in extra and set(value) == base | extra[kind], "NATIVE_VALUE_FIELDS")
    if kind in ("NSNUMBER", "CFBOOLEAN"):
        require(integer(value["cfTypeId"], 1, 2**31) and
                integer(value["cfBooleanTypeId"], 1, 2**31), "ACTUAL_CF_TYPE_IDS")
        if kind == "CFBOOLEAN":
            require(value["cfTypeId"] == value["cfBooleanTypeId"]
                    and type(value["value"]) is bool, "ACTUAL_CF_BOOLEAN")
        else:
            require(value["cfTypeId"] != value["cfBooleanTypeId"]
                    and type(value["value"]) in (int, float)
                    and math.isfinite(value["value"]), "ACTUAL_CF_NUMBER")
    elif kind == "NSSTRING":
        require(text(value["value"]), "STORED_STRING_BOUND")
    elif kind in ("WFCONTENTCOLLECTION", "NSARRAY", "NSORDEREDSET"):
        require(integer(value["cardinality"]) and type(value["items"]) is list
                and len(value["items"]) == value["cardinality"], "ACTUAL_CARDINALITY")
        if kind == "WFCONTENTCOLLECTION":
            require(value["nativeClass"] == "WFContentCollection"
                    and text(value["itemContainerClass"], 256), "ACTUAL_COLLECTION_CLASS")
        for item in value["items"]:
            native_value(item, depth + 1, budget)
    elif kind == "NSDICTIONARY":
        require(integer(value["cardinality"]) and type(value["entries"]) is list
                and len(value["entries"]) == value["cardinality"], "ACTUAL_DICTIONARY_CARDINALITY")
        for entry in value["entries"]:
            require(type(entry) is dict and set(entry) == {"key", "value"}, "ACTUAL_DICTIONARY_ENTRY")
            native_value(entry["key"], depth + 1, budget)
            native_value(entry["value"], depth + 1, budget)
    elif kind in ("WFSTRINGCONTENTITEM", "WFDICTIONARYCONTENTITEM", "WFCONTENTITEM"):
        require(value["representationSource"] == "stored-representationsByType",
                "STORED_REPRESENTATIONS_ONLY")
        native_value(value["representations"], depth + 1, budget)
    elif kind == "WFOBJECTREPRESENTATION":
        require(value["nativeClass"] == "WFObjectRepresentation"
                and value["observationSource"] == "stored-object-getter",
                "STORED_OBJECT_GETTER")
        native_value(value["object"], depth + 1, budget)
    elif kind in ("NSATTRIBUTEDSTRING", "WFNUMBERCONTENTITEM", "WFOBJECTTYPE"):
        field = {"NSATTRIBUTEDSTRING": "plainString", "WFNUMBERCONTENTITEM": "number",
                 "WFOBJECTTYPE": "declaredClassName"}[kind]
        native_value(value[field], depth + 1, budget)


def validate_original_action_stream(source_bytes, plist_bytes, pilot, *,
                                    source_sha256, native_sha256):
    """Return source-bound events, without claiming a complete scenario result."""
    require(type(source_bytes) is bytes and
            hashlib.sha256(source_bytes).hexdigest() == source_sha256, "ORIGINAL_SOURCE")
    require(type(plist_bytes) is bytes and 0 < len(plist_bytes) <= 2 * 1024**2 and
            hashlib.sha256(plist_bytes).hexdigest() == native_sha256, "ORIGINAL_NATIVE_BYTES")
    original = plistlib.loads(plist_bytes)
    actions = original.get("WFWorkflowActions")
    require(type(actions) is list and 1 <= len(actions) <= 16, "ORIGINAL_ACTION_BOUND")
    require(type(pilot) is dict and pilot.get("schema") == "cherri-workflowkit-original-action-pilot/1"
            and pilot.get("status") == "ENGINE_CALLBACKS_RETAINED_VALIDATION_PENDING"
            and pilot.get("done") is True and type(pilot.get("engineRunInvocations")) is int
            and pilot["engineRunInvocations"] == 1, "ACTUAL_TERMINAL_PILOT")
    require(all(pilot.get(k) is False for k in SCOPE), "PILOT_SCOPE")
    require("error" not in pilot and "callbackError" not in pilot, "NO_OBSERVATION_ERROR")
    require(pilot.get("construction") ==
            "original-action-array-through-WFAction-factory; transient-WFWorkflow; no-library-import",
            "EXPLICIT_TRANSIENT_CONSTRUCTION")
    require(len(json.dumps(pilot, ensure_ascii=False).encode()) <= 1024**2, "PILOT_BYTES_BOUND")
    events = pilot.get("events")
    require(type(events) is list and 5 <= len(events) <= 64, "EVENT_BOUND")
    prior = -math.inf
    for index, row in enumerate(events):
        require(type(row) is dict and type(row.get("sequence")) is int
                and row["sequence"] == index, "CONTIGUOUS_EVENT_SEQUENCE")
        moment = row.get("uptime")
        require(type(moment) in (int, float) and math.isfinite(moment) and moment >= prior,
                "MONOTONIC_EVENT_TIME")
        prior = moment

    def one(kind):
        matches = [row for row in events if row.get("kind") == kind]
        require(len(matches) == 1, "EXACT_ONE_" + kind)
        return matches[0]

    model, run, will, final = [one(k) for k in
                              ("model-created", "engine-run-invoked",
                               "workflow-will-run", "workflow-finished")]
    models = model.get("actions")
    require(model.get("workflowClass") == "WFWorkflow" and type(models) is list
            and len(models) == len(actions), "COMPLETE_ORIGINAL_MODEL")
    absent = []
    seen_uuid = set()
    for index, (encoded, bound) in enumerate(zip(actions, models)):
        require(type(encoded) is dict and type(bound) is dict and
                set(bound) == {"index", "identifier", "actualClass", "actualUUID", "originalUUID"},
                "MODEL_IDENTITY_FIELDS")
        require(type(bound["index"]) is int and bound["index"] == index
                and bound["identifier"] == encoded.get("WFWorkflowActionIdentifier")
                and text(bound["actualClass"], 256) and bound["actualClass"], "ORIGINAL_ACTION_IDENTITY")
        require(type(bound["actualUUID"]) is str and UUID.fullmatch(bound["actualUUID"])
                and bound["actualUUID"] not in seen_uuid, "MODEL_RUNTIME_UUID")
        seen_uuid.add(bound["actualUUID"])
        parameters = encoded.get("WFWorkflowActionParameters", {})
        require(type(parameters) is dict, "ORIGINAL_PARAMETER_SHAPE")
        original_uuid = parameters.get("UUID")
        require(bound["originalUUID"] == original_uuid, "ORIGINAL_UUID_PRESENCE")
        if original_uuid is None:
            absent.append(index)
        else:
            require(bound["actualUUID"] == original_uuid, "ORIGINAL_ACTION_UUID")
    require(model.get("originalUUIDAbsentAtIndex") == absent, "ORIGINAL_UUID_ABSENCE")
    for original_key, observed_key in [
        ("WFWorkflowHasOutputFallback", "originalHasOutputFallback"),
        ("WFWorkflowHasShortcutInputVariables", "originalHasShortcutInputVariables"),
    ]:
        require(type(original.get(original_key)) is bool
                and model.get(observed_key) is original[original_key], "ORIGINAL_MODEL_FLAGS")
    completed = [row for row in events if row.get("kind") == "action-completed"]
    require(1 <= len(completed) <= 48, "ACTION_COMPLETION_BOUND")
    require(model["sequence"] < run["sequence"] < will["sequence"] <
            completed[0]["sequence"] <= completed[-1]["sequence"] < final["sequence"]
            and final["sequence"] == len(events) - 1, "ACTUAL_CALLBACK_ORDER")
    require(0 <= final["uptime"] - run["uptime"] <= 8, "ORIGINAL_ENGINE_DEADLINE")
    native_value(will.get("input"))
    for row in completed:
        require(integer(row.get("index"), 0, len(actions) - 1), "ACTION_MODEL_INDEX")
        bound = models[row["index"]]
        require(all(row.get(k) == bound[k] for k in
                    ("identifier", "actualClass", "actualUUID")), "ACTION_MODEL_BINDING")
        require(row.get("error") is None and row.get("inputObservationPhase") == "action-completion",
                "ACTION_COMPLETION_PHASE")
        for key in ("input", "output", "controllerPendingInput"):
            native_value(row.get(key))
    require(final.get("cancelled") is False and final.get("error") is None, "SUCCESSFUL_ENGINE_TERMINAL")
    native_value(final.get("output"))
    native_value(final.get("actionOutputsAtFinish"))
    return {"original": original, "models": models, "events": events,
            "completed": completed, "will": will, "final": final}


def collection(value, expected_count):
    require(value.get("kind") == "WFCONTENTCOLLECTION"
            and value.get("cardinality") == expected_count, "EXACT_CONTENT_COLLECTION")
    return value["items"]


def stored_text(item):
    require(item.get("kind") == "WFSTRINGCONTENTITEM"
            and item.get("nativeClass") == "WFStringContentItem", "ACTUAL_STRING_CONTENT_ITEM")
    representations = item["representations"]
    require(representations.get("kind") == "NSDICTIONARY"
            and representations.get("cardinality") == 1, "SINGLE_STORED_STRING_REPRESENTATION")
    entry = representations["entries"][0]
    key = entry["key"]
    require(key.get("kind") == "WFOBJECTTYPE" and key.get("nativeClass") == "WFObjectType"
            and key.get("declaredClassName", {}).get("kind") == "NSSTRING"
            and key["declaredClassName"]["value"] == "NSString", "STORED_STRING_DECLARATION")
    values = entry["value"]
    require(values.get("kind") == "NSARRAY" and values.get("cardinality") == 1, "ONE_STORED_OBJECT")
    representation = values["items"][0]
    require(representation.get("kind") == "WFOBJECTREPRESENTATION"
            and representation.get("observationSource") == "stored-object-getter",
            "ACTUAL_STORED_OBJECT_GETTER")
    observed = representation["object"]
    require(observed.get("kind") == "NSSTRING"
            and observed.get("nativeClass") == "__NSCFString", "ACTUAL_STORED_STRING_TYPE")
    return observed["value"]


def validate_nothing_text_nothing(source_bytes, plist_bytes, pilot):
    bound = validate_original_action_stream(source_bytes, plist_bytes, pilot,
                                           source_sha256=TEXT_SOURCE_SHA,
                                           native_sha256=TEXT_PLIST_SHA)
    expected_identifiers = ["is.workflow.actions.nothing",
                            "is.workflow.actions.gettext", "is.workflow.actions.nothing"]
    expected_classes = ["WFNothingAction", "WFTextAction", "WFNothingAction"]
    require([row["identifier"] for row in bound["models"]] == expected_identifiers
            and [row["actualClass"] for row in bound["models"]] == expected_classes,
            "EXACT_ORIGINAL_THREE_ACTIONS")
    completed = bound["completed"]
    require([row["index"] for row in completed] == [0, 1, 2], "EXACT_THREE_COMPLETION_ORDER")
    require(bound["will"]["input"] == NIL and all(
        row["input"] == NIL and row["controllerPendingInput"] == NIL for row in completed),
        "ACTUAL_COMPLETION_INPUT_ABSENCE")
    collection(completed[0]["output"], 0)
    actual_text = stored_text(collection(completed[1]["output"], 1)[0])
    require(actual_text == "M02_AFTER_NOTHING", "ORIGINAL_INTERMEDIATE_TEXT")
    collection(completed[2]["output"], 0)
    require(bound["final"]["output"] == NIL, "ACTUAL_VOID_DISTINCT_FROM_EMPTY")
    require(bound["final"]["actionOutputsAtFinish"] == NIL, "ACTUAL_FINISHED_OUTPUT_TABLE_ABSENT")
    return {
        "schema": "cherri-workflowkit-stored-representation-validation/1",
        "status": "ACTUAL_ORIGINAL_MAC_ENGINE_TEXT_OBSERVATION_VALIDATED",
        "sourceSHA256": TEXT_SOURCE_SHA, "nativeSHA256": TEXT_PLIST_SHA,
        "completedOriginalActions": 3, "actionOrder": [0, 1, 2],
        "actualIntermediateText": actual_text, "actualTextContentClass": "WFStringContentItem",
        "valueObservedThrough": "stored-WFObjectRepresentation.object",
        "nothingOutputCollectionCardinalities": [0, 0], "finalOutputPresent": False,
        "completionTimeInputObservedAbsent": True, "willRunActionInputsObserved": False,
        "nativeExecutionRepeated": False, "signedImportExercised": False,
        "iOSDeviceExercised": False, "completeM02CohortQualified": False,
        "formalAcceptanceGranted": False,
    }
