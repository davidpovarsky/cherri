#!/usr/bin/env python3
"""Generate source-bound evidence for the eleven existing Asana macro children.

This is a requirements/evidence projection, not an action or macro registry.
Descriptors come from the real compiler. Synthetic CLI compiles remain distinct
from Apple runtime proof. No private shortcut corpus or signing service is used.
"""
import argparse
import collections
import copy
import datetime
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import plistlib
import re
import subprocess
import sys
import tempfile

sys.dont_write_bytecode = True
ROOT = Path(__file__).resolve().parents[2]
GENERATOR = Path(__file__).resolve()
MASTER = "1220544599390563"
NATIVE_PREFIX = "is.workflow.actions."
MARKER = "P15 ledger original \u05e9\u05dc\u05d5\u05dd \U0001f352"
LABEL_HELPER = "tools/shortcuts-simulator-qa/debug_label_acceptance.py"
# Reviewed fixture/control inventory, not callable signatures or an action registry.
LABEL_ACTION_COUNTS = {"plain": 171, "dated": 177, "typed": 203,
                       "dated-typed": 212, "text": 204, "dated-text": 212}
LABEL_BASE_MUTATIONS = (
    "raw_object_redirected", "original_output_name_lost", "boxed_raw_metadata",
    "metadata_label_replaced", "cardinality_from_other_object", "range_subtraction_reversed",
    "range_start_zero", "metadata_append_uses_raw_object", "restore_previous_output_redirected",
    "clear_has_input", "metadata_clear_has_input", "constant_success_text", "raw_list_item_shifted",
    "file_bytes_borrow_source", "file_name_borrow_source", "empty_label_key_observation_changed",
    "wrong_fixed_date", "arbitrary_unowned_nothing", "network_effect",
)
LABEL_DATE_MUTATIONS = ("late_creation_date_capture", "different_date_property")
LABEL_PREVIEW_MUTATIONS = (
    "outer_item_not_snapshotted", "outer_index_not_snapshotted", "range_first_reads_last",
    "range_comparison_inverted", "metadata_loop_owner_changed", "preview_row_name_is_constant",
    "preview_reads_raw_buffer_directly",
)
LABEL_LEGACY_MUTATIONS = ("authored_legacy_list_clear_has_input", "authored_legacy_list_order_changed",
                          "authored_legacy_list_append_redirected")

# Requirements and evidence links only. Callable signatures are never defined here.
REQUIREMENTS = (
    ("require", "1220544599477512", "Typed require", ("require",),
     "Return the original typed object after semantic validation; failure stops with the supplied message.",
     "PARTIAL_IMPLEMENTATION", (),
     ("Full false/zero/null/empty/native-object matrix and native failure/cancel proof are not established by this ledger.",)),
    ("label", "1220544718902445", "debugLog labels", ("debugLog",),
     "Stable per-entry labels without replacing the original raw object or its downstream UUID.",
     "PARTIAL_IMPLEMENTATION", ("Native file/object identity and six-mode label acceptance",),
     ("Per-entry metadata and bounded source contracts are implemented; signed File-carrier byte identity still requires repair and acceptance.",)),
    ("export", "1220544665232213", "Local debugExport", ("debugExport",),
     "Local returned file; typed JSON/text or explicit opaque placeholders; safe escaping and default redaction.",
     "PARTIAL_IMPLEMENTATION", ("Apple returned-file/UTF-8/cardinality runtime evidence",),
     ("Local export source preserves full default redaction. Explicit disclosure supports Text, native Number display text, ISO 8601 Dates and bounded Dictionary field diagnostics; nonempty literal redactKeys filters fields before value access. Source Bool/null/List boundaries, nested containers, native Date/label behavior and returned File acceptance remain open. Separate feature tests do not grant runtime credit to this ledger; QA debugExportTrace is separate.",)),
    ("blocks", "1220544718901241", "Shared typed block facility", ("measure",),
     "Scoped typed block/IR, one evaluation, shadowing, value/void, nesting and explicit unavailable-dialect diagnostics.",
     "PARTIAL_IMPLEMENTATION", (),
     ("V2 and Legacy share once-only typed-value and Void measure bodies with explicit empty-result boundaries. General parameterized blocks, control-result declarations and full early-error/cancellation semantics remain open.",)),
    ("measure", "1220544665232149", "Measure typed value", ("measure",),
     "Evaluate the body once, preserve its native result and remove only timing instrumentation in Release.",
     "PARTIAL_IMPLEMENTATION", ("Apple timing accuracy and error/cancel observation",),
     ("Legacy uses a final yield or value-producing call through existing structured tokens. Structural UUID/timer checks are not native elapsed-time, abort or cancellation proof.",)),
    ("collect", "1220544651247337", "Typed collection", (),
     "Use existing native Repeat Results without a second accumulator or standalone alias; account separately for typed input construction.",
     "EXISTING_V2_CONSTRUCT", ("Native empty/mixed/nested collection shape and performance observation",),
     ("No standalone collect alias is introduced. Compiled Repeat Results does not prove runtime flattening behavior.",)),
    ("cache", "1220544665232085", "Persistent TTL cache", ("cache",),
     "Explicit namespace/scope/TTL and honest serialization, invalidation, clock and permission behavior.",
     "IMPLEMENTATION_GAP_AND_NATIVE_PREREQUISITE", ("Stored Content cross-run/restart/global-local readback", "Physical iPad persistence/background proof"),
     ("Shared source supports Text/Number/Bool and guarded Date epoch-offset archives. Dictionary/List/Optional/JsonNull/File persistence remains unsupported. This ledger does not exercise persistence, payload round trips, expiry, invalidation or native error behavior; invalid numeric Date archives may abort in native Adjust Date before reconstruction guards.",)),
    ("retry", "1220544651247273", "Bounded retry", ("retry",),
     "Bounded attempts/backoff with actual transport/status/permission/cancel classification and idempotence safeguards.",
     "IMPLEMENTATION_GAP_AND_NATIVE_PREREQUISITE", ("Recoverable native error/status contract and signed synthetic endpoint probes",),
     ("No evidence establishes catchable transport failures; Alert plus Stop Shortcut is not exception handling.",)),
    ("paginate", "1220544665232021", "Bounded pagination", ("paginate",),
     "Explicit limits/cursor termination, typed aggregation and malformed/error/rate-limit/credential safeguards.",
     "BOUNDED_GET_IMPLEMENTATION", ("Signed synthetic multi-page HTTP/schema/error observation",),
     ("The shared bounded GET implementation has page/item limits, cycle guards, page callbacks and literal Dictionary/Text/Number/Bool item selection with optional key projection. These ledger cases exercise the default Dictionary route only; separate typed-selection tests do not establish native list shape. Response headers/status, transport recovery, redirect limits and generic retry remain outside its proven contract.",)),
    ("profiles", "1220544607463126", "Debug and Release", ("debugLog", "require", "measure"),
     "Compile-time diagnostic removal while retaining semantic require checks and original native producers.",
     "PARTIAL_IMPLEMENTATION", ("Native result equivalence and debug-mode acceptance",),
     ("The controlled compiler comparisons cover declared fixtures; they do not establish every diagnostic or runtime path.",)),
    ("qa", "1220544718902349", "Macro QA and consumers", (),
     "Generated evidence, canonical metadata/docs and coherent Skill/iOS/remote consumer acceptance.",
     "PARTIAL_EVIDENCE", ("Final pinned consumer tuple and signed/physical acceptance evidence",),
     ("This ledger builds and tests a compiler; it does not install a Skill, update a provider or execute on Apple.",)),
)


def sha(data):
    return hashlib.sha256(data).hexdigest()


def canonical(value):
    return json.dumps(value, ensure_ascii=False, sort_keys=True, separators=(",", ":")).encode("utf-8")


def read_json(path):
    return json.loads(path.read_text(encoding="utf-8"))


def write_json(path, value):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_bytes((json.dumps(value, ensure_ascii=False, indent=2) + "\n").encode("utf-8"))


def label_helper(repo):
    spec = importlib.util.spec_from_file_location("cherri_macro_label_controls", repo / LABEL_HELPER)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    expected = {"qa-debug-label-" + mode + "-" + dialect
                for mode in LABEL_ACTION_COUNTS for dialect in ("v2", "legacy")}
    if set(module.MODES) != set(LABEL_ACTION_COUNTS) or set(module.SOURCE_HASHES) != expected:
        raise ValueError("MACRO_LABEL_HELPER_INVENTORY_MISMATCH")
    return module


def label_source_inputs(repo, helper):
    # Exact bytes: the helper itself and its twelve independently pinned sources.
    inputs = {LABEL_HELPER: sha((repo / LABEL_HELPER).read_bytes())}
    for fixture, expected in helper.SOURCE_HASHES.items():
        path = "tools/shortcuts-simulator-qa/fixtures/" + fixture + ".cherri"
        observed = sha((repo / path).read_bytes())
        if observed != expected:
            raise ValueError("MACRO_LABEL_REVIEWED_SOURCE_CHANGED")
        inputs[path] = observed
    return inputs


def verify_label_controls(report, repo):
    controls = report.get("labelControls")
    if not isinstance(controls, dict):
        raise ValueError("MACRO_LABEL_COMPONENT_MISSING")
    if set(controls) != {"schemaVersion", "implementationSha", "cleanCheckout", "compilerSha256",
                         "helperSha256", "fixtures", "actualCompileCount", "rejectedMutationCount",
                         "nativeExecution", "formalAcceptanceGranted", "sourceInputs"} or controls["schemaVersion"] != "1":
        raise ValueError("MACRO_LABEL_COMPONENT_SCHEMA_MISMATCH")
    helper = label_helper(repo)
    inputs = label_source_inputs(repo, helper)
    if (controls["implementationSha"] != report["sourceSha"] or
        type(report["worktreeDirtyBeforeExecution"]) is not bool or
        controls["cleanCheckout"] is not (not report["worktreeDirtyBeforeExecution"])):
        raise ValueError("MACRO_LABEL_SOURCE_JOIN_MISMATCH")
    if controls["compilerSha256"] != report["compilerSHA256"] or not re.fullmatch(r"[0-9a-f]{64}", controls["compilerSha256"]):
        raise ValueError("MACRO_LABEL_COMPILER_JOIN_MISMATCH")
    if controls["helperSha256"] != inputs[LABEL_HELPER]:
        raise ValueError("MACRO_LABEL_HELPER_JOIN_MISMATCH")
    if controls["sourceInputs"] != inputs:
        raise ValueError("MACRO_LABEL_SOURCE_INPUT_MISMATCH")
    if controls["nativeExecution"] != "NOT_RUN" or controls["formalAcceptanceGranted"] is not False:
        raise ValueError("MACRO_LABEL_SCOPE_MISMATCH")
    fixtures = controls["fixtures"]
    if (not isinstance(fixtures, list) or any(not isinstance(r, dict) for r in fixtures) or
        [r.get("fixtureId") for r in fixtures] != list(helper.SOURCE_HASHES)):
        raise ValueError("MACRO_LABEL_CASE_INVENTORY_MISMATCH")
    total_mutations = 0
    for row in fixtures:
        if set(row) != {"fixtureId", "source", "unsignedPlistSha256", "nativeActionCount", "graph", "causalMutations"}:
            raise ValueError("MACRO_LABEL_CASE_SCHEMA_MISMATCH")
        fixture = row["fixtureId"]
        mode = helper.fixture_mode(fixture)
        source = {"sourceSha256": helper.SOURCE_HASHES[fixture], "mode": mode,
                  "syntheticOnly": True, "nativeExecution": "NOT_RUN", "formalAcceptanceGranted": False}
        if canonical(row["source"]) != canonical(source):
            raise ValueError("MACRO_LABEL_SOURCE_MISMATCH")
        if not re.fullmatch(r"[0-9a-f]{64}", row["unsignedPlistSha256"]):
            raise ValueError("MACRO_LABEL_UNSIGNED_HASH_MISSING")
        count = LABEL_ACTION_COUNTS[mode] + (7 if fixture.endswith("-legacy") else 0)
        graph = row["graph"]
        expected_graph = {"mode": mode, "sourceSha256": helper.SOURCE_HASHES[fixture], "labels": 6,
                          "rawBufferExpectedItems": 7, "metadataContainsRawObjects": False,
                          "originalUUIDAndOutputNamesPreserved": True, "nativeValueAndCardinality": "NOT_RUN",
                          "datePrecision": "Seconds", "scopedToReviewedSource": True,
                          "arbitraryUserActionClassification": False}
        if (type(row["nativeActionCount"]) is not int or row["nativeActionCount"] != count or
            not isinstance(graph, dict) or
            canonical({k: v for k, v in graph.items() if k != "instrumentation"}) != canonical(expected_graph)):
            raise ValueError("MACRO_LABEL_GRAPH_CONTRACT_MISMATCH")
        partition = graph.get("instrumentation", {})
        if (not isinstance(partition, dict) or set(partition) != {"inlineIndices", "collectorIndices", "sourceIndices"} or
            any(not isinstance(indices, list) or not indices or
                any(type(i) is not int for i in indices) or indices != sorted(set(indices))
                for indices in partition.values()) or
            sorted(i for indices in partition.values() for i in indices) != list(range(count))):
            raise ValueError("MACRO_LABEL_GRAPH_CONTRACT_MISMATCH")
        dates, typed, as_text = helper.MODES[mode]
        mutations = (LABEL_BASE_MUTATIONS + (LABEL_DATE_MUTATIONS if dates else ()) +
                     (LABEL_PREVIEW_MUTATIONS if typed or as_text else ()) +
                     (LABEL_LEGACY_MUTATIONS if fixture.endswith("-legacy") else ()))
        expected_mutations = {"rejected": list(mutations), "count": len(mutations), "nativeExecution": "NOT_RUN"}
        if canonical(row["causalMutations"]) != canonical(expected_mutations):
            raise ValueError("MACRO_LABEL_MUTATION_INVENTORY_MISMATCH")
        total_mutations += len(mutations)
    if (type(controls["actualCompileCount"]) is not int or controls["actualCompileCount"] != 12 or
        type(controls["rejectedMutationCount"]) is not int or controls["rejectedMutationCount"] != total_mutations or
        total_mutations != 314):
        raise ValueError("MACRO_LABEL_EXECUTION_COUNT_MISMATCH")


def input_fingerprint(repo):
    # Reuse the existing catalog coverage source scope, including all root Go,
    # internal Go, actions and compiler test fixtures. Add this producer and the
    # canonical schema/docs generators and independent native metadata.
    tools_path = repo / "tools/p10r3"
    sys.path.insert(0, str(tools_path))
    try:
        spec = importlib.util.spec_from_file_location("cherri_action_contract_source", tools_path / "generate_action_contract_coverage.py")
        module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(module)
        base = module.source_fingerprint(repo)
    finally:
        sys.path.pop(0)
    extra = {}
    for pattern in ("tools/language-schema/*.go", "tools/language-docs/*.go"):
        for path in repo.glob(pattern):
            if path.is_file():
                extra[path.relative_to(repo).as_posix()] = sha(path.read_bytes().replace(b"\r\n", b"\n"))
    native = repo / "docs/apple-runtime/macos-27/builtin-actions.json"
    extra[native.relative_to(repo).as_posix()] = sha(native.read_bytes().replace(b"\r\n", b"\n"))
    extra["generator"] = sha(GENERATOR.read_bytes().replace(b"\r\n", b"\n"))
    extra.update(label_source_inputs(repo, label_helper(repo)))
    return sha(canonical({"existingSourceFingerprint": base, "additionalInputs": extra}))


def run(args, repo, timeout=90, required=True, env=None):
    proc = subprocess.run([str(a) for a in args], cwd=repo, env=env,
                          text=True, encoding="utf-8", errors="replace",
                          stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=timeout)
    if required and proc.returncode:
        raise RuntimeError("COMMAND_FAILED: " + str(args[0]) + "\n" + (proc.stdout + proc.stderr)[-2500:])
    return proc


def action_id(action):
    return action.get("WFWorkflowActionIdentifier", "")


def params(action):
    return action.get("WFWorkflowActionParameters", {})


def named(actions, suffix):
    return [a for a in actions if action_id(a) == NATIVE_PREFIX + suffix]


def output_ref(value):
    if not isinstance(value, dict):
        return ""
    body = value.get("Value", value)
    if not isinstance(body, dict) or body.get("Type") != "ActionOutput" or body.get("Aggrandizements"):
        return ""
    return body.get("OutputUUID", "")


def final_consumer_index(actions, dialect):
    # These fixtures end with a getVariable expression and have no explicit
    # return. V2 emits exactly one bare Nothing for implicit void completion;
    # Legacy keeps the final consumer as its last action. Do not discard or
    # normalize any action: malformed/extra terminators must still fail.
    if dialect == "v2":
        if len(actions) < 2 or actions[-1] != {"WFWorkflowActionIdentifier": NATIVE_PREFIX + "nothing"}:
            return None
        index = len(actions) - 2
    elif dialect == "legacy":
        index = len(actions) - 1
    else:
        return None
    if index < 0 or action_id(actions[index]) != NATIVE_PREFIX + "getvariable":
        return None
    return index


def origin_matches(actions, producer, dialect):
    consumer_index = final_consumer_index(actions, dialect)
    if consumer_index is None:
        return False
    producers = {}
    for index, action in enumerate(actions):
        identity = params(action).get("UUID")
        if identity:
            if identity in producers:
                return False
            producers[identity] = (index, action)
    current = output_ref(params(actions[consumer_index]).get("WFVariable"))
    seen = set()
    while current:
        if current in seen or current not in producers:
            return False
        seen.add(current)
        index, action = producers[current]
        if index >= consumer_index:
            return False
        if current == producer:
            return bool(producer)
        if action_id(action) != NATIVE_PREFIX + "getvariable":
            return False
        consumer_index = index
        current = output_ref(params(action).get("WFVariable"))
    return False



def empty_measure_void_graph_matches(actions, profile, dialect):
    # Independent complete empty-body contract from EmitScopedMeasureVoid and
    # its canonical Previous boundary: capture native input, start the timer,
    # restore the owned input, then elapsed/log/forward and parameter-free Nothing.
    # Bind only opaque identities; retain every action, field and reference.
    if profile != "debug" or dialect not in ("legacy", "v2") or type(actions) is not list or len(actions) != 7:
        return False
    if any(type(a) is not dict for a in actions) or any(type(a.get("WFWorkflowActionParameters")) is not dict for a in actions[:6]):
        return False
    input_name = params(actions[0]).get("WFVariableName")
    timer = params(actions[1]).get("WFVariableName")
    restored = params(actions[2]).get("UUID")
    elapsed = params(actions[3]).get("UUID")
    forwarded = params(actions[5]).get("UUID")
    uuid = r"[0-9A-Fa-f]{8}-(?:[0-9A-Fa-f]{4}-){3}[0-9A-Fa-f]{12}"
    identities = (restored, elapsed, forwarded)
    if (type(input_name) is not str or re.fullmatch(r"__cherri_block_[0-9A-Fa-f]{32}_input", input_name) is None or
        type(timer) is not str or re.fullmatch(r"__cherri_debug_timer_[0-9a-f]{16}", timer) is None or
        any(type(value) is not str or re.fullmatch(uuid, value) is None for value in identities) or
        len({value.lower() for value in identities}) != len(identities)):
        return False
    def attachment(value):
        return {"WFSerializationType": "WFTextTokenAttachment", "Value": value}
    def date_string(value):
        return {"WFSerializationType": "WFTextTokenString", "Value": {
            "string": "\ufffc", "attachmentsByRange": {"{0, 1}": value}}}
    def action(suffix, parameters):
        return {"WFWorkflowActionIdentifier": NATIVE_PREFIX + suffix, "WFWorkflowActionParameters": parameters}
    elapsed_ref = {"Type": "ActionOutput", "OutputUUID": elapsed, "OutputName": "Time Between Dates"}
    expected = [
        action("setvariable", {"WFVariableName": input_name}),
        action("setvariable", {"WFVariableName": timer, "WFInput": attachment({"Type": "CurrentDate"})}),
        action("getvariable", {"UUID": restored, "WFVariable": attachment({"Type": "Variable", "VariableName": input_name})}),
        action("gettimebetweendates", {"UUID": elapsed, "WFTimeUntilUnit": "Seconds",
            "WFTimeUntilFromDate": date_string({"Type": "Variable", "VariableName": timer}),
            "WFInput": date_string({"Type": "CurrentDate"})}),
        action("appendvariable", {"WFVariableName": "__cherri_debug_log", "WFInput": attachment(elapsed_ref)}),
        action("getvariable", {"UUID": forwarded, "WFVariable": attachment(elapsed_ref)}),
        {"WFWorkflowActionIdentifier": NATIVE_PREFIX + "nothing"},
    ]
    return actions == expected


def negative_result_matches(spec, exit_code, log, artifact_exists):
    if exit_code == 0 or artifact_exists or spec["diagnostic"] not in log:
        return False
    if "diagnosticExact" in spec:
        return type(exit_code) is int and exit_code == 1 and log == spec["diagnosticExact"]
    return True


def structural_checks(kind, profile, actions, dialect):
    checks = {}
    if kind == "require":
        originals = [a for a in named(actions, "gettext") if params(a).get("WFTextActionText") == MARKER]
        checks["oneOriginalProducer"] = len(originals) == 1
        checks["originalUUIDPassthrough"] = len(originals) == 1 and origin_matches(actions, params(originals[0]).get("UUID"), dialect)
        checks["canonicalGetType"] = len(named(actions, "getitemtype")) >= 2 and not named(actions, "gettypeaction")
        checks["semanticGuardRetained"] = all(named(actions, suffix) for suffix in ("conditional", "alert", "exit", "count"))
        checks["diagnosticProfile"] = bool(named(actions, "appendvariable")) if profile == "debug" else not named(actions, "appendvariable")
    elif kind == "measure":
        producers = named(actions, "dictionary")
        checks["oneTypedBodyProducer"] = len(producers) == 1
        checks["originalUUIDPassthrough"] = len(producers) == 1 and origin_matches(actions, params(producers[0]).get("UUID"), dialect)
        checks["noTextNumberCoercion"] = not named(actions, "gettext") and not named(actions, "number")
        checks["timerProfile"] = len(named(actions, "gettimebetweendates")) == (1 if profile == "debug" else 0)
        checks["releaseNoDiagnosticAppend"] = profile != "release" or not named(actions, "appendvariable")
    elif kind == "measure-void":
        checks["emptyVoidWholeGraph"] = empty_measure_void_graph_matches(actions, profile, dialect)
    elif kind == "collect":
        loops = named(actions, "repeat.each")
        checks["pairedRepeatRegion"] = (len(loops) == 2 and
            params(loops[0]).get("GroupingIdentifier") == params(loops[1]).get("GroupingIdentifier") and
            str(params(loops[0]).get("WFControlFlowMode")) == "0" and str(params(loops[1]).get("WFControlFlowMode")) == "2")
        checks["consumerUsesRepeatResults"] = len(loops) == 2 and origin_matches(actions, params(loops[1]).get("UUID"), dialect)
        checks["noAccumulator"] = not named(actions, "appendvariable")
    elif kind == "paginate":
        requests = named(actions, "downloadurl")
        loops = [a for a in named(actions, "repeat.count") if str(params(a).get("WFControlFlowMode")) == "0"]
        checks["oneGETNode"] = len(requests) == 1 and params(requests[0]).get("WFHTTPMethod") == "GET"
        checks["explicitPageBudget"] = len(loops) == 1 and str(params(loops[0]).get("WFRepeatCount")) == "3"
        checks["nativeCycleHash"] = bool(named(actions, "hash"))
        checks["typedDictionaryWitness"] = bool(named(actions, "getitemtype")) and bool(named(actions, "getvalueforkey"))
        checks["semanticFailureStops"] = bool(named(actions, "alert")) and bool(named(actions, "exit"))
    elif kind == "paginate-zero":
        checks["zeroBudgetNoGET"] = not named(actions, "downloadurl")
        checks["zeroBudgetNoLoop"] = not named(actions, "repeat.count") and not named(actions, "repeat.each")
        checks["explicitEmptyResult"] = any(params(a).get("WFItems") == [] for a in named(actions, "list"))
    return {key: bool(value) for key, value in checks.items()}


def fixture_specs():
    result = []
    for dialect in ("v2", "legacy"):
        prefix = "let" if dialect == "v2" else "const"
        arguments = 'original, type: "text", nonEmpty: true, message: "P15 semantic guard"' if dialect == "v2" else 'original, "text", true, "P15 semantic guard"'
        source = (prefix + " original = text(" + json.dumps(MARKER, ensure_ascii=False) + ")\n"
                  "debugLog()\n" + prefix + " checked = require(" + arguments + ")\ngetVariable(checked)\n")
        for profile in ("debug", "release"):
            result.append({"id": "require_" + dialect + "_" + profile, "kind": "require",
                           "dialect": dialect, "profile": profile, "source": source})
    measure = ('let result = measure("P15 typed \u05e9\u05dc\u05d5\u05dd") {\n'
               '    let payload = {"flag": false, "nested": [1, {"name": "typed"}]}\n'
               '    payload\n}\ngetVariable(result)\n')
    for dialect in ("v2", "legacy"):
        body = measure if dialect == "v2" else measure.replace("let ", "const ").replace("    payload\n", "    yield payload\n")
        for profile in ("debug", "release"):
            result.append({"id": "measure_" + dialect + "_" + profile, "kind": "measure", "dialect": dialect,
                           "profile": profile, "source": body})
    for size in (0, 1, 100):
        source = "let result = for item in [" + ",".join(str(i) for i in range(size)) + "] { yield item }\ngetVariable(result)\n"
        result.append({"id": "collect_v2_" + str(size), "kind": "collect", "dialect": "v2",
                       "profile": "debug", "source": source})
    result.append({"id": "collect_v2_dictionary", "kind": "collect", "dialect": "v2",
        "profile": "debug", "source": 'let result = for item in [1, 2] { yield {"name": "typed", "flag": false} }\ngetVariable(result)\n'})
    result.extend((
        # Preserve the historical ID and exact source. The original name records
        # an obsolete expectation; this source is a legal Void statement.
        {"id": "measure_legacy_empty_rejected", "kind": "measure-void", "dialect": "legacy", "profile": "debug",
         "source": 'measure("empty") {}\n',
         "historicalIDNote": "Original rejected suffix retained; exact source is now checked as a legal Void statement."},
        {"id": "measure_legacy_control_value_rejected", "kind": "negative", "dialect": "legacy", "profile": "debug",
         "source": 'measure("control") {\nconst value = if 1 == 1 {\ntext("body")\n}\nyield value\n}\n', "diagnostic": "E_BLOCK_LEGACY_CONTROL_VALUE"},
        {"id": "measure_v2_empty_rejected", "kind": "negative", "dialect": "v2", "profile": "debug",
         "source": 'let result = measure("empty") {}\n', "diagnostic": "E_BLOCK_VOID_VALUE",
         "diagnosticExact": "Compilation error: semantic error [E_BLOCK_VOID_VALUE]: a void scoped block cannot initialize a binding; end the block with an explicit value at <synthetic-fixtures>/measure_v2_empty_rejected/source.cherri:1:14-1:33\n"},
        {"id": "measure_v2_empty_statement", "kind": "measure-void", "dialect": "v2", "profile": "debug",
         "source": 'measure("empty") {}\n'},
        {"id": "measure_legacy_empty_binding_rejected", "kind": "negative", "dialect": "legacy", "profile": "debug",
         "source": 'const result = measure("empty") {\n}\n', "diagnostic": "E_BLOCK_VOID_VALUE",
         "diagnosticExact": "\nError: E_BLOCK_VOID_VALUE: a Void scoped body has no value to bind or consume\n\n"},
    ))
    for dialect in ("v2", "legacy"):
        for kind, pages in (("paginate", 3), ("paginate-zero", 0)):
            result.append({"id": kind + "_" + dialect, "kind": kind, "dialect": dialect,
                           "profile": "debug", "source": 'paginate("https://example.invalid/p15/entry.json", "items", "next", ' + str(pages) + ', 6)\n'})
    return result


def compile_case(spec, binary, repo, work):
    folder = work / spec["id"]
    folder.mkdir()
    source = folder / "source.cherri"
    target = folder / "output.shortcut"
    source.write_bytes(spec["source"].encode("utf-8"))
    args = [binary, "compile", source, "--output=" + str(target), "--skip-sign",
            "--mode=" + spec["dialect"], "--build-profile=" + spec["profile"], "--load-all-actions", "--no-ansi"]
    process = run(args, repo, required=False)
    log = (process.stdout + process.stderr).replace(str(work), "<synthetic-fixtures>")
    row = {"id": spec["id"], "kind": spec["kind"], "dialect": spec["dialect"], "profile": spec["profile"],
           "source": spec["source"], "sourceSHA256": sha(spec["source"].encode("utf-8")),
           "command": ["<freshly-built-compiler>"] + [str(a).replace(str(work), "<synthetic-fixtures>") for a in args[1:]],
           "exitCode": process.returncode, "diagnosticTail": log[-1200:],
           "signedRuntime": "NOT_EXECUTED", "physicalDevice": "NOT_EXECUTED"}
    if "historicalIDNote" in spec:
        row["historicalIDNote"] = spec["historicalIDNote"]
    if spec["kind"] == "negative":
        row["expectedDiagnostic"] = spec["diagnostic"]
        if "diagnosticExact" in spec:
            row["expectedDiagnosticExact"] = spec["diagnosticExact"]
        row["noArtifact"] = not target.exists()
        row["verdict"] = "PASS" if negative_result_matches(spec, process.returncode, log, target.exists()) else "FAIL"
        return row, None
    if process.returncode != 0 or not target.exists():
        row["verdict"] = "FAIL"
        row["artifact"] = "MISSING"
        return row, None
    native = plistlib.loads(target.read_bytes())
    actions = native.get("WFWorkflowActions")
    if not isinstance(actions, list):
        raise ValueError("MISSING_NATIVE_ACTIONS:" + spec["id"])
    checks = structural_checks(spec["kind"], spec["profile"], actions, spec["dialect"])
    row.update(unsignedSHA256=sha(target.read_bytes()), nativeActionCount=len(actions),
               nativeActionIDs=dict(sorted(collections.Counter(action_id(a) for a in actions).items())),
               structuralChecks=checks)
    imported = folder / "imported.cherri"
    import_run = run([binary, "import", target, "--output=" + str(imported),
                      "--mode=v2", "--fidelity=lossless", "--no-ansi"], repo, required=False)
    rebuilt = folder / "rebuilt.shortcut"
    recompile_exit = None
    exact = False
    if import_run.returncode == 0 and imported.exists():
        compiled = run([binary, "compile", imported, "--output=" + str(rebuilt),
                        "--skip-sign", "--mode=v2", "--build-profile=debug", "--no-ansi"], repo, required=False)
        recompile_exit = compiled.returncode
        if compiled.returncode == 0 and rebuilt.exists():
            exact = plistlib.loads(rebuilt.read_bytes()).get("WFWorkflowActions") == actions
    row["decompile"] = {"mode": "V2_LOSSLESS_EXPANDED_NATIVE", "importExitCode": import_run.returncode,
                        "recompileExitCode": recompile_exit, "nativeActionsExact": exact,
                        "macroSourceReconstruction": "NOT_CLAIMED"}
    row["verdict"] = "PASS" if all(checks.values()) and exact else "FAIL"
    return row, actions


MUTATION_SCENARIOS = (
        ("require_v2_debug", "wrong_type_action", "canonicalGetType"),
        ("require_v2_release", "lost_output_uuid", "originalUUIDPassthrough"),
        ("measure_v2_debug", "duplicate_body", "oneTypedBodyProducer"),
        ("measure_v2_release", "retained_release_timer", "timerProfile"),
        ("measure_legacy_debug", "scoped_alias_cycle", "originalUUIDPassthrough"),
        ("measure_legacy_release", "scoped_result_coercion", "originalUUIDPassthrough"),
        ("collect_v2_1", "wrong_repeat_result", "consumerUsesRepeatResults"),
        ("paginate_v2", "page_budget_changed", "explicitPageBudget"),
        ("require_v2_release", "extra_void_terminator", "originalUUIDPassthrough"),
        ("require_v2_release", "malformed_void_terminator", "originalUUIDPassthrough"),
        ("require_v2_release", "missing_void_terminator", "originalUUIDPassthrough"),
        ("measure_v2_release", "wrong_final_consumer", "originalUUIDPassthrough"),
        ("require_v2_release", "original_uuid_replaced", "originalUUIDPassthrough"),
        ("require_legacy_release", "legacy_void_terminator", "originalUUIDPassthrough"),
) + tuple((case, mutation, "emptyVoidWholeGraph")
    for case in ("measure_legacy_empty_rejected", "measure_v2_empty_statement")
    for mutation in (
        "void_wrong_start_clock", "void_wrong_stop_origin", "void_wrong_stop_clock",
        "void_wrong_elapsed_log", "void_wrong_elapsed_forward", "void_elapsed_coercion",
        "void_wrong_unit", "void_missing_boundary", "void_boundary_payload",
        "void_extra_value", "void_duplicate_elapsed_id", "void_extra_action",
    )) + tuple((case, mutation, "emptyVoidWholeGraph")
    for case in ("measure_legacy_empty_rejected", "measure_v2_empty_statement")
    for mutation in (
        "void_input_capture_has_input", "void_input_capture_order", "void_missing_input_capture",
        "void_input_restore_wrong_slot", "void_input_restore_coercion", "void_input_restore_order",
        "void_input_restore_missing_id", "void_input_restore_duplicate_id", "void_missing_input_restore",
    ))


def check_mutations(outputs):
    rows = []
    specs = {s["id"]: s for s in fixture_specs()}
    for case_id, mutation, expected in MUTATION_SCENARIOS:
        actual = outputs.get(case_id)
        if actual is None:
            rows.append({"case": case_id, "mutation": mutation, "verdict": "FAIL", "reason": "positive compile missing"})
            continue
        spec = specs[case_id]
        original = structural_checks(spec["kind"], spec["profile"], actual, spec["dialect"])
        if not all(original.values()) or not original.get(expected, False):
            rows.append({"case": case_id, "mutation": mutation, "intendedCheck": expected,
                         "positiveControl": False, "faultRejected": False, "verdict": "FAIL",
                         "reason": "positive graph contract failed before mutation"})
            continue
        changed = copy.deepcopy(actual)
        consumer = final_consumer_index(changed, spec["dialect"])
        if spec["kind"] == "measure-void":
            # The complete positive oracle above establishes these exact roles
            # and boundaries before any causal corruption is attempted.
            captured, timer, restored, elapsed, logged, forwarded, boundary = changed
        if mutation == "void_input_capture_has_input":
            params(captured)["WFInput"] = {"WFSerializationType": "WFTextTokenAttachment", "Value": {"Type": "Clipboard"}}
        elif mutation == "void_input_capture_order":
            changed[0], changed[1] = timer, captured
        elif mutation == "void_missing_input_capture":
            changed.remove(captured)
        elif mutation == "void_input_restore_wrong_slot":
            params(restored)["WFVariable"]["Value"]["VariableName"] += "_unowned"
        elif mutation == "void_input_restore_coercion":
            params(restored)["WFVariable"]["Value"]["Aggrandizements"] = [{"Type": "WFCoercionVariableAggrandizement", "CoercionItemClass": "WFStringContentItem"}]
        elif mutation == "void_input_restore_order":
            changed[2], changed[3] = elapsed, restored
        elif mutation == "void_input_restore_missing_id":
            del params(restored)["UUID"]
        elif mutation == "void_input_restore_duplicate_id":
            params(restored)["UUID"] = params(elapsed)["UUID"]
        elif mutation == "void_missing_input_restore":
            changed.remove(restored)
        elif mutation == "void_wrong_start_clock":
            params(timer)["WFInput"]["Value"]["Type"] = "Clipboard"
        elif mutation == "void_wrong_stop_origin":
            params(elapsed)["WFTimeUntilFromDate"]["Value"]["attachmentsByRange"]["{0, 1}"]["VariableName"] += "_unowned"
        elif mutation == "void_wrong_stop_clock":
            params(elapsed)["WFInput"]["Value"]["attachmentsByRange"]["{0, 1}"]["Type"] = "Clipboard"
        elif mutation == "void_wrong_elapsed_log":
            params(logged)["WFInput"]["Value"]["OutputUUID"] = params(forwarded)["UUID"]
        elif mutation == "void_wrong_elapsed_forward":
            params(forwarded)["WFVariable"]["Value"]["OutputUUID"] = params(forwarded)["UUID"]
        elif mutation == "void_elapsed_coercion":
            params(forwarded)["WFVariable"]["Value"]["Aggrandizements"] = [{"Type": "WFCoercionVariableAggrandizement", "CoercionItemClass": "WFStringContentItem"}]
        elif mutation == "void_wrong_unit":
            params(elapsed)["WFTimeUntilUnit"] = "Minutes"
        elif mutation == "void_missing_boundary":
            changed.remove(boundary)
        elif mutation == "void_boundary_payload":
            boundary["WFWorkflowActionParameters"] = {"WFInput": "invented value"}
        elif mutation == "void_extra_value":
            changed.insert(changed.index(boundary), {"WFWorkflowActionIdentifier": NATIVE_PREFIX + "number", "WFWorkflowActionParameters": {"WFNumberActionNumber": 0}})
        elif mutation == "void_duplicate_elapsed_id":
            params(forwarded)["UUID"] = params(elapsed)["UUID"]
        elif mutation == "void_extra_action":
            changed.append({"WFWorkflowActionIdentifier": NATIVE_PREFIX + "nothing"})
        elif mutation == "wrong_type_action":
            named(changed, "getitemtype")[0]["WFWorkflowActionIdentifier"] = NATIVE_PREFIX + "gettypeaction"
        elif mutation in ("lost_output_uuid", "wrong_repeat_result"):
            params(changed[consumer])["WFVariable"] = {"Value": {"Type": "ActionOutput", "OutputUUID": "missing-producer"}, "WFSerializationType": "WFTextTokenAttachment"}
        elif mutation == "duplicate_body":
            changed.insert(0, copy.deepcopy(named(changed, "dictionary")[0]))
        elif mutation == "scoped_alias_cycle":
            alias = named(changed, "getvariable")[-2]
            params(alias)["WFVariable"] = {"Value": {"Type": "ActionOutput", "OutputUUID": params(alias)["UUID"]}, "WFSerializationType": "WFTextTokenAttachment"}
        elif mutation == "scoped_result_coercion":
            params(changed[consumer])["WFVariable"]["Value"]["Aggrandizements"] = [{"Type": "WFCoercionVariableAggrandizement", "CoercionItemClass": "WFStringContentItem"}]
        elif mutation in ("extra_void_terminator", "legacy_void_terminator"):
            changed.append({"WFWorkflowActionIdentifier": NATIVE_PREFIX + "nothing"})
        elif mutation == "malformed_void_terminator":
            changed[-1]["WFWorkflowActionParameters"] = {"WFVariable": "unexpected payload"}
        elif mutation == "missing_void_terminator":
            changed.pop()
        elif mutation == "wrong_final_consumer":
            changed[consumer]["WFWorkflowActionIdentifier"] = NATIVE_PREFIX + "count"
        elif mutation == "original_uuid_replaced":
            original_producer = next(a for a in named(changed, "gettext") if params(a).get("WFTextActionText") == MARKER)
            params(original_producer)["UUID"] = "replaced-original-producer"
        elif mutation == "page_budget_changed":
            next(a for a in named(changed, "repeat.count") if str(params(a).get("WFControlFlowMode")) == "0")["WFWorkflowActionParameters"]["WFRepeatCount"] = 4
        else:
            changed.insert(consumer, {"WFWorkflowActionIdentifier": NATIVE_PREFIX + "gettimebetweendates", "WFWorkflowActionParameters": {}})
        altered = structural_checks(spec["kind"], spec["profile"], changed, spec["dialect"])
        ok = all(original.values()) and original[expected] and not altered[expected]
        rows.append({"case": case_id, "mutation": mutation, "intendedCheck": expected,
                     "positiveControl": all(original.values()), "faultRejected": not altered[expected],
                     "verdict": "PASS" if ok else "FAIL"})
    return rows


def observations_digest(report):
    observations = {k: report[k] for k in ("compilerMacros", "canonicalDocs", "nativeMetadata",
                                         "compiles", "causalMutations", "crossCaseChecks", "requirements")}
    observations["labelControls"] = report.get("labelControls")
    return sha(canonical(observations))


def build_rows(macros, cases, label_controls=None):
    names = {m["callableName"]: m for m in macros}
    rows = []
    label_cases = [r["fixtureId"] for r in label_controls["fixtures"]] if label_controls is not None else []
    links = {
        "require": ["require_v2_debug", "require_v2_release", "require_legacy_debug", "require_legacy_release"],
        "label": label_cases,
        "blocks": ["measure_v2_debug", "measure_v2_release", "measure_legacy_debug", "measure_legacy_release", "measure_legacy_empty_rejected", "measure_legacy_control_value_rejected", "measure_v2_empty_rejected", "measure_v2_empty_statement", "measure_legacy_empty_binding_rejected"],
        "measure": ["measure_v2_debug", "measure_v2_release", "measure_legacy_debug", "measure_legacy_release", "measure_legacy_empty_rejected", "measure_v2_empty_rejected", "measure_v2_empty_statement", "measure_legacy_empty_binding_rejected"],
        "collect": ["collect_v2_0", "collect_v2_1", "collect_v2_100", "collect_v2_dictionary"],
        "paginate": ["paginate_v2", "paginate_legacy", "paginate-zero_v2", "paginate-zero_legacy"],
        "profiles": ["require_v2_debug", "require_v2_release", "require_legacy_debug", "require_legacy_release", "measure_v2_debug", "measure_v2_release", "measure_legacy_debug", "measure_legacy_release"],
        "qa": [c["id"] for c in cases] + label_cases,
    }
    for key, task, title, wanted, requirement, state, external, limitations in REQUIREMENTS:
        descriptors = [name for name in wanted if name in names]
        if key == "label":
            exists = any(p.get("id") == "label" for p in names.get("debugLog", {}).get("parameters", []))
            contract = "LABEL_PARAMETER_PRESENT_REQUIRES_NEW_ACCEPTANCE" if exists else "LABEL_PARAMETER_ABSENT"
            if exists and label_cases:
                contract = "LABEL_PARAMETER_PRESENT_WITH_SOURCE_BOUND_CLI_CONTROLS"
            elif exists:
                state = "DECLARED_BUT_NOT_EXERCISED"
        elif key == "collect":
            contract = "EXISTING_FOR_YIELD_CONSTRUCT_NO_COLLECT_ALIAS"
        elif key in ("profiles", "qa"):
            contract = "CROSS_CUTTING_COMPILER_CONTRACT"
        else:
            contract = "DESCRIPTOR_PRESENT" if descriptors else "NO_CANONICAL_DESCRIPTOR"
            if descriptors and state.startswith("IMPLEMENTATION_GAP"):
                state = "DECLARED_BUT_NOT_EXERCISED"
        rows.append({"feature": key, "asanaTask": task, "name": title, "acceptanceRequirement": requirement,
                     "status": "OPEN", "implementationState": state, "canonicalDescriptors": descriptors,
                     "canonicalContractObservation": contract, "executedCases": links.get(key, []),
                     "externalPrerequisites": list(external), "limitations": list(limitations),
                     "nativeSourceExemplar": "NO_INDEPENDENT_APPLE_AUTHORED_MACRO_SPECIMEN",
                     "signedSimulator": "NOT_ESTABLISHED_BY_THIS_LEDGER",
                     "physicalDevice": "BLOCKED_DEVICE",
                     "closure": "NOT_PROVEN_BY_STRUCTURAL_EVIDENCE"})
    return rows


def retain_compiler(binary, destination, expected_sha):
    data = binary.read_bytes()
    if sha(data) != expected_sha:
        raise ValueError("MACRO_RETAINED_COMPILER_SOURCE_CHANGED")
    destination.parent.mkdir(parents=True, exist_ok=True)
    with destination.open("xb") as output:
        output.write(data)
    if sha(destination.read_bytes()) != expected_sha:
        raise ValueError("MACRO_RETAINED_COMPILER_COPY_CHANGED")


def refresh(repo, compiler_output=None):
    fingerprint = input_fingerprint(repo)
    source_sha = run(["git", "rev-parse", "HEAD"], repo).stdout.strip()
    dirty = bool(run(["git", "status", "--porcelain"], repo).stdout.strip())
    with tempfile.TemporaryDirectory(prefix="cherri-macro-contract-") as directory:
        work = Path(directory)
        binary = work / ("cherri.exe" if os.name == "nt" else "cherri")
        env = os.environ.copy()
        env["CGO_ENABLED"] = "0"
        build_command = ["go", "build", "-trimpath", "-o", str(binary), "."]
        run(build_command, repo, timeout=300, env=env)
        compiler_sha = sha(binary.read_bytes())
        caps = json.loads(run([binary, "--capabilities-json"], repo).stdout)
        macros = caps.get("compilerMacros")
        if not isinstance(macros, list) or not macros or not re.fullmatch(r"[0-9a-f]{64}", caps.get("macroFingerprint", "")):
            raise ValueError("CANONICAL_MACRO_METADATA_MISSING")
        names = [m["callableName"] for m in macros]
        if names != sorted(set(names)):
            raise ValueError("MACRO_DESCRIPTORS_NOT_UNIQUE_SORTED")
        docs = work / "actions-reference.md"
        run(["go", "run", "./tools/language-docs", docs], repo, timeout=300, env=env)
        docs_text = docs.read_text(encoding="utf-8")
        tick = chr(96)
        expected_header = "**Compiler Macro Fingerprint:** " + tick + caps["macroFingerprint"] + tick
        docs_ok = expected_header in docs_text and "## Compiler macros" in docs_text
        native_path = repo / "docs/apple-runtime/macos-27/builtin-actions.json"
        meta = read_json(native_path)["is.workflow.actions.getitemtype"]
        native_ok = (meta.get("ActionClass") == "WFGetItemTypeAction" and
                     meta.get("Output", {}).get("Types") == ["NSString"] and
                     meta.get("Input", {}).get("ParameterKey") == "WFInput")
        compiles, outputs = [], {}
        for spec in fixture_specs():
            row, actions = compile_case(spec, binary, repo, work)
            compiles.append(row)
            outputs[spec["id"]] = actions
            print(json.dumps({"case": row["id"], "verdict": row["verdict"]}), flush=True)
        mutations = check_mutations(outputs)
        by_id = {c["id"]: c for c in compiles}
        totals, loop_counts, numeric_inputs = [], [], []
        for size in (0, 1, 100):
            graph = outputs["collect_v2_" + str(size)] or []
            boundaries = [i for i, action in enumerate(graph) if action_id(action) == NATIVE_PREFIX + "repeat.each"]
            totals.append(len(graph))
            loop_counts.append(boundaries[1] - boundaries[0] + 1 if len(boundaries) == 2 else None)
            numeric_inputs.append(len(named(graph, "number")))
        profile_counts = {}
        for dialect in ("legacy", "v2"):
            debug = by_id["require_" + dialect + "_debug"].get("nativeActionIDs", {})
            release = by_id["require_" + dialect + "_release"].get("nativeActionIDs", {})
            profile_counts[dialect] = all(release.get(NATIVE_PREFIX + name, 0) == debug.get(NATIVE_PREFIX + name, 0) > 0
                                          for name in ("conditional", "alert", "exit", "getitemtype", "count"))
        cross = {"constantCollectionLoopActionCount": None not in loop_counts and len(set(loop_counts)) == 1 and loop_counts[0] > 0,
                 "collectionLoopActionCounts": loop_counts, "collectionTotalActionCounts": totals,
                 "numericInputActionCounts": numeric_inputs, "typedNumericInputCountsMatch": numeric_inputs == [0, 1, 100],
                 "requireGuardCountsDebugRelease": profile_counts}
        # Reuse the owning fresh compiler and the existing twelve-source producer.
        # Its sixty-second CLI bounds, real graph checks and 314 causal mutations
        # are unchanged. These cases are separate from the original 19/14 ledger.
        helper = label_helper(repo)
        label_controls = helper.compile_controls(repo, binary)
        label_controls["sourceInputs"] = label_source_inputs(repo, helper)
        if sha(binary.read_bytes()) != compiler_sha:
            raise ValueError("MACRO_LABEL_COMPILER_CHANGED_DURING_EXECUTION")
        print(json.dumps({"component": "label", "actualCompileCount": label_controls["actualCompileCount"],
                          "rejectedMutationCount": label_controls["rejectedMutationCount"],
                          "nativeExecution": label_controls["nativeExecution"]}), flush=True)
        requirements = build_rows(macros, compiles, label_controls)
        report = {"formatVersion": 1, "observedAtUtc": datetime.datetime.now(datetime.timezone.utc).isoformat(),
                  "masterAsanaTask": MASTER, "sourceSha": source_sha, "worktreeDirtyBeforeExecution": dirty,
                  "sourceFingerprint": fingerprint, "generatorSHA256": sha(GENERATOR.read_bytes().replace(b"\r\n", b"\n")),
                  "buildCommand": ["go", "build", "-trimpath", "-o", "<temporary-compiler>", "."],
                  "goVersion": run(["go", "version"], repo).stdout.strip(),
                  "compilerSHA256": compiler_sha, "languageVersion": caps.get("languageVersion"),
                  "schemaFingerprint": caps.get("schemaFingerprint"), "macroFingerprint": caps["macroFingerprint"],
                  "nativeCatalogCount": caps.get("actionCount"), "compilerMacroCount": len(macros),
                  "compilerMacros": macros, "scope": "SYNTHETIC_CLI_STRUCTURAL_AND_LOSSLESS_NATIVE_ONLY",
                  "canonicalDocs": {"generatedFromThisSource": True, "macroFingerprintMatches": docs_ok, "sha256": sha(docs.read_bytes())},
                  "nativeMetadata": {"source": native_path.relative_to(repo).as_posix(),
                                     "sha256": sha(native_path.read_bytes().replace(b"\r\n", b"\n")),
                                     "canonicalTypeWitness": "is.workflow.actions.getitemtype", "contractMatches": native_ok},
                  "compiles": compiles, "causalMutations": mutations, "crossCaseChecks": cross,
                  "labelControls": label_controls,
                  "requirements": requirements,
                  "privacy": {"privateCorpusRead": False, "signerUsed": False, "networkFixtureExecuted": False, "liveConsumerChanged": False}}
        if compiler_output is not None:
            retain_compiler(binary, compiler_output, compiler_sha)
    if input_fingerprint(repo) != fingerprint:
        raise ValueError("SOURCE_CHANGED_DURING_EXECUTION")
    passed = all(c["verdict"] == "PASS" for c in compiles) and all(c["verdict"] == "PASS" for c in mutations)
    passed = passed and docs_ok and native_ok and cross["constantCollectionLoopActionCount"] and cross["typedNumericInputCountsMatch"] and all(profile_counts.values())
    report["summary"] = {"asanaChildren": len(requirements), "open": len(requirements), "closed": 0,
                         "compilerMacros": len(macros), "compileChecks": len(compiles),
                         "compilePassed": sum(c["verdict"] == "PASS" for c in compiles),
                         "causalMutationChecks": len(mutations), "structuralExecutionVerdict": "PASS" if passed else "FAIL",
                         "labelCompileChecks": label_controls["actualCompileCount"],
                         "labelMutationChecks": label_controls["rejectedMutationCount"],
                         "releaseCompletion": "OPEN"}
    report["observationsSHA256"] = observations_digest(report)
    return report


def markdown(report):
    lines = ["# Macro contract coverage", "", "All eleven existing Asana children remain OPEN.", "",
             "This generated ledger records actual synthetic CLI compilation, native graph checks and exact lossless native-action roundtrip. It does not establish signed runtime, physical-device or installed-consumer acceptance.", "",
             "- Compiler source: " + report["sourceSha"],
             "- Source fingerprint: " + report["sourceFingerprint"],
             "- Macro fingerprint: " + report["macroFingerprint"],
             "- Canonical compiler macros: " + str(report["compilerMacroCount"]),
             "- Structural execution: " + report["summary"]["structuralExecutionVerdict"],
             "- Compile checks: " + str(report["summary"]["compilePassed"]) + "/" + str(report["summary"]["compileChecks"]),
             "- Causal mutation checks: " + str(report["summary"]["causalMutationChecks"]),
             "- Separate label compile checks: " + str(report["summary"]["labelCompileChecks"]),
             "- Separate label causal mutation checks: " + str(report["summary"]["labelMutationChecks"]), "",
             "| Requirement | Asana child | Implementation observation | Executed cases | Status |",
             "|---|---|---|---:|---|"]
    for row in report["requirements"]:
        lines.append("| " + row["name"] + " | " + row["asanaTask"] + " | " + row["implementationState"] + " | " + str(len(row["executedCases"])) + " | OPEN |")
    lines.extend(["", "## Actual controlled CLI observations", "",
                  "| Case | Dialect/profile | Native actions | Exact lossless action roundtrip | Result |",
                  "|---|---|---:|---|---|"])
    for row in report["compiles"]:
        roundtrip = row.get("decompile", {}).get("nativeActionsExact")
        detail = str(roundtrip) if roundtrip is not None else "expected unavailable/invalid diagnostic"
        lines.append("| " + row["id"] + " | " + row["dialect"] + "/" + row["profile"] + " | " + str(row.get("nativeActionCount", "-")) + " | " + detail + " | " + row["verdict"] + " |")
    for row in report["compiles"]:
        if "historicalIDNote" in row:
            lines.extend(["", row["id"] + ": " + row["historicalIDNote"]])
    lines.extend(["", "## Separate label CLI component", "",
                  "These twelve reviewed fixtures use the same freshly built compiler. Their existing helper verifies real graphs and 314 causal mutations. They are separate from the " + str(report["summary"]["compileChecks"]) + " cases and " + str(report["summary"]["causalMutationChecks"]) + " mutations above; no label roundtrip, signed execution or native value/cardinality result is claimed. Native execution remains NOT_RUN and formal acceptance remains false.", "",
                  "| Reviewed fixture | Native actions in unsigned graph | Rejected causal mutations | Native execution |",
                  "|---|---:|---:|---|"])
    for row in report["labelControls"]["fixtures"]:
        lines.append("| " + row["fixtureId"] + " | " + str(row["nativeActionCount"]) + " | " +
                     str(row["causalMutations"]["count"]) + " | NOT_RUN |")
    lines.extend(["", "## Limits and next prerequisites", ""])
    for row in report["requirements"]:
        lines.append("- " + row["name"] + ": " + " ".join(row["limitations"]) +
                     (" External prerequisites: " + "; ".join(row["externalPrerequisites"]) + "." if row["externalPrerequisites"] else ""))
    lines.extend(["", "Standalone collect is deliberately not added. Existing for/yield evidence belongs to the typed collection requirement; it is not counted as a new canonical macro. Numeric input construction precedes the constant-size native loop and is counted separately. Cache and retry are outside this ledger's executed fixtures; their source and native acceptance require separately scoped evidence. Label and bounded GET pagination have implementations with open native acceptance. Local export has a bounded Text-only/default-redacted source subset; its native acceptance and remaining typed serialization contract stay open.", "",
                  "Reproduce with python3 tools/p10r15/generate_macro_contract_coverage.py --refresh, then --verify. Every refresh builds the source and runs the actual CLI. Verification rejects source/producer drift and edited or incomplete evidence. The source SHA is the compiled revision before committing evidence, avoiding a self-referential commit pin.", ""])
    return "\n".join(lines)


def verify(report, repo):
    if report.get("sourceFingerprint") != input_fingerprint(repo):
        raise ValueError("STALE_MACRO_SOURCE_EVIDENCE")
    if report.get("generatorSHA256") != sha(GENERATOR.read_bytes().replace(b"\r\n", b"\n")):
        raise ValueError("STALE_MACRO_PRODUCER")
    if report.get("observationsSHA256") != observations_digest(report):
        raise ValueError("EDITED_MACRO_OBSERVATIONS")
    if {r["asanaTask"] for r in report["requirements"]} != {r[1] for r in REQUIREMENTS} or len(report["requirements"]) != 11:
        raise ValueError("MISSING_OR_DUPLICATE_ASANA_CHILD")
    if any(r["status"] != "OPEN" for r in report["requirements"]):
        raise ValueError("STRUCTURAL_EVIDENCE_CANNOT_CLOSE_CHILD")
    if {c["id"] for c in report["compiles"]} != {c["id"] for c in fixture_specs()}:
        raise ValueError("MISSING_CONTROLLED_COMPILE")
    if report["summary"]["structuralExecutionVerdict"] != "PASS":
        raise ValueError("MACRO_EXECUTION_FAILED")
    if any(c["verdict"] != "PASS" for c in report["compiles"] + report["causalMutations"]):
        raise ValueError("MACRO_CASE_FAILED")
    if (len(report["compiles"]) != len(fixture_specs()) or
        tuple((m.get("case"), m.get("mutation"), m.get("intendedCheck")) for m in report["causalMutations"]) != MUTATION_SCENARIOS):
        raise ValueError("MACRO_EXECUTION_COUNT_MISMATCH")
    if any(not m.get("positiveControl") or not m.get("faultRejected") for m in report["causalMutations"]):
        raise ValueError("MACRO_MUTATION_NOT_CAUSAL")
    specs = {spec["id"]: spec for spec in fixture_specs()}
    for case in report["compiles"]:
        spec = specs[case["id"]]
        if any(case[field] != spec[field] for field in ("id", "kind", "dialect", "profile", "source")):
            raise ValueError("MACRO_FIXTURE_IDENTITY_MISMATCH")
        if case["sourceSHA256"] != sha(case["source"].encode("utf-8")):
            raise ValueError("MACRO_FIXTURE_HASH_MISMATCH")
        if case["kind"] == "negative":
            if (case["expectedDiagnostic"] != spec["diagnostic"] or
                case.get("expectedDiagnosticExact") != spec.get("diagnosticExact") or
                not negative_result_matches(spec, case["exitCode"], case["diagnosticTail"], not case["noArtifact"])):
                raise ValueError("MACRO_NEGATIVE_NOT_CAUSAL")
            continue
        if case["exitCode"] != 0 or not case["structuralChecks"] or not all(case["structuralChecks"].values()):
            raise ValueError("MACRO_STRUCTURAL_CHECK_FAILED")
        if case["nativeActionCount"] != sum(case["nativeActionIDs"].values()):
            raise ValueError("MACRO_NATIVE_ACTION_COUNT_MISMATCH")
        decompile = case["decompile"]
        if decompile["importExitCode"] != 0 or decompile["recompileExitCode"] != 0 or not decompile["nativeActionsExact"]:
            raise ValueError("MACRO_LOSSLESS_ROUNDTRIP_FAILED")
    if not report["canonicalDocs"]["macroFingerprintMatches"] or not report["nativeMetadata"]["contractMatches"]:
        raise ValueError("MACRO_CANONICAL_CONTRACT_FAILED")
    cross = report["crossCaseChecks"]
    if not cross["constantCollectionLoopActionCount"] or not cross["typedNumericInputCountsMatch"] or not all(cross["requireGuardCountsDebugRelease"].values()):
        raise ValueError("MACRO_CROSS_CASE_CHECK_FAILED")
    names = [m["callableName"] for m in report["compilerMacros"]]
    if report["compilerMacroCount"] != len(names) or names != sorted(set(names)):
        raise ValueError("MACRO_DESCRIPTOR_COUNT_MISMATCH")
    summary = report["summary"]
    if summary["open"] != 11 or summary["closed"] != 0 or summary["compileChecks"] != len(report["compiles"]) or summary["compilePassed"] != len(report["compiles"]) or summary["causalMutationChecks"] != len(MUTATION_SCENARIOS):
        raise ValueError("MACRO_SUMMARY_MISMATCH")
    verify_label_controls(report, repo)
    expected_rows = build_rows(report["compilerMacros"], report["compiles"], report["labelControls"])
    if canonical(report["requirements"]) != canonical(expected_rows):
        raise ValueError("MACRO_LABEL_REQUIREMENT_LINK_MISMATCH")
    if summary.get("labelCompileChecks") != 12 or summary.get("labelMutationChecks") != 314:
        raise ValueError("MACRO_LABEL_SUMMARY_MISMATCH")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--repo", type=Path, default=ROOT)
    parser.add_argument("--out-dir", type=Path)
    parser.add_argument("--retain-compiler", type=Path, help="retain the already-tested exact compiler at a fresh explicit path")
    mode = parser.add_mutually_exclusive_group(required=True)
    mode.add_argument("--refresh", action="store_true")
    mode.add_argument("--verify", action="store_true")
    opts = parser.parse_args()
    if opts.retain_compiler is not None:
        if not opts.refresh:
            parser.error("--retain-compiler requires --refresh")
        if opts.retain_compiler.exists() or opts.retain_compiler.is_symlink():
            parser.error("retained compiler destination must be absent")
    repo = opts.repo.resolve()
    out = opts.out_dir.resolve() if opts.out_dir else repo / "docs/apple-runtime/p10r15"
    json_path = out / "macro-contract-coverage.json"
    md_path = out / "macro-contract-coverage.md"
    if opts.refresh:
        report = refresh(repo, opts.retain_compiler)
        write_json(json_path, report)
        md_path.write_bytes(markdown(report).encode("utf-8"))
    else:
        report = read_json(json_path)
    verify(report, repo)
    if md_path.read_bytes() != markdown(report).encode("utf-8"):
        raise ValueError("STALE_MACRO_MARKDOWN")
    print(json.dumps(report["summary"]), flush=True)


if __name__ == "__main__":
    try:
        main()
    except (ValueError, RuntimeError, OSError, subprocess.TimeoutExpired) as exc:
        print(str(exc), file=sys.stderr)
        raise SystemExit(1)
