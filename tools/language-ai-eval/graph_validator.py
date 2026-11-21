#!/usr/bin/env python3
"""Independent, bounded graph/dataflow oracle for the six AI01 task contracts.

This evaluates a deliberately small pure subset of emitted Shortcuts actions.
It does not execute Shortcuts, claim native coercion coverage, or use model
explanations as evidence. Unsupported source/graph forms stay unadjudicated.
"""

from dataclasses import dataclass, replace
from decimal import Decimal, InvalidOperation
import json
import math
import os
from pathlib import Path
import plistlib
import re
import subprocess
import sys
import time
import uuid

from evaluation_bundle import (BundleError, SCHEMA, canonical, claim_request,
                               digest, load_bundle, model_request, read_regular,
                               require, unique_json, utc_now, write_new)


PREFIX = "is.workflow.actions."
MAX_SOURCE = 64 * 1024
MAX_PLIST = 4 * 1024 * 1024
MAX_ACTIONS = 96
COMPILE_TIMEOUT = 35
ORIGINAL_LIMITS = {
    "intent.json": 1024 * 1024, "request.json": 1024 * 1024,
    "source.cherri": MAX_SOURCE, "compiler.stdout": MAX_PLIST,
    "compiler.stderr": MAX_PLIST, "output.shortcut": MAX_PLIST,
    "reply.json": 1024 * 1024,
}
META = {"UUID", "CustomOutputName"}
ACTION_FIELDS = {
    "gettext": {"WFTextActionText"},
    "number": {"WFNumberActionNumber"},
    "math": {"WFInput", "WFMathOperand", "WFMathOperation"},
    "list": {"WFItems"},
    "getitemfromlist": {"WFInput", "WFItemIndex", "WFItemSpecifier"},
    "setvariable": {"WFInput", "WFVariableName", "WFSerializationType"},
    "appendvariable": {"WFInput", "WFVariableName", "WFSerializationType"},
    "getvariable": {"WFVariable"},
    "nothing": set(),
    "showresult": {"Text"},
    "count": {"Input", "WFCountType"},
    "conditional": {"GroupingIdentifier", "WFControlFlowMode", "WFCondition",
                    "WFConditionalLegacyComparisonBehavior", "WFInput", "WFNumberValue"},
}
TOP_FIELDS = {"WFWorkflowActions", "WFWorkflowIcon", "WFQuickActionSurfaces",
              "WFWorkflowInputContentItemClasses", "WFWorkflowClientVersion",
              "WFWorkflowMinimumClientVersion", "WFWorkflowMinimumClientVersionString",
              "WFWorkflowImportQuestions", "WFWorkflowTypes",
              "WFWorkflowOutputContentItemClasses", "WFWorkflowHasShortcutInputVariables",
              "WFWorkflowHasOutputFallback", "WFWorkflowNoInputBehavior", "WFWorkflowTriggers"}


class Unsupported(BundleError):
    """The frozen oracle cannot adjudicate this shape; never score it as PASS."""


def supported(ok, code):
    if not ok:
        raise Unsupported(code)


def number(value):
    supported(type(value) in (int, float, str, Decimal), "NUMBER_KIND")
    if type(value) is float:
        supported(math.isfinite(value), "NUMBER_NONFINITE")
    if isinstance(value, str):
        supported(len(value) <= 128, "NUMBER_TEXT_LIMIT")
        match = re.fullmatch(r"-?(?:0|[1-9][0-9]*)(?:\.[0-9]+)?(?:[eE]([+-]?[0-9]+))?", value)
        supported(match is not None, "NUMBER_TEXT_GRAMMAR")
        if match.group(1) is not None:
            exponent = match.group(1)
            supported(len(exponent) <= 4 and -308 <= int(exponent) <= 308, "NUMBER_EXPONENT_RANGE")
    try:
        result = Decimal(str(value))
    except InvalidOperation as error:
        raise Unsupported("NUMBER_PARSE") from error
    supported(result.is_finite() and result.copy_abs() <= Decimal(2)**53, "NUMBER_EXACT_RANGE")
    return result


def number_text(value):
    return str(int(value)) if value == int(value) else format(value.normalize(), "f")


@dataclass(frozen=True)
class Value:
    kind: str
    data: object
    nodes: frozenset = frozenset()
    bindings: frozenset = frozenset()
    operations: tuple = ()
    selections: tuple = ()


def merge_value(kind, data, values, node=None, binding=None, operation=None, selection=None):
    nodes, bindings, operations, selections = set(), set(), [], []
    for value in values:
        nodes.update(value.nodes)
        bindings.update(value.bindings)
        operations.extend(x for x in value.operations if x not in operations)
        selections.extend(x for x in value.selections if x not in selections)
    if node is not None:
        nodes.add(node)
    if binding is not None:
        bindings.add(binding)
    if operation is not None:
        operations.append(operation)
    if selection is not None:
        selections.append(selection)
    return Value(kind, data, frozenset(nodes), frozenset(bindings), tuple(operations), tuple(selections))


def render(value):
    if value.kind == "Text":
        return value.data
    if value.kind == "Number":
        return number_text(value.data)
    raise Unsupported("TEXT_RENDER_KIND")


def uuid_key(value):
    supported(isinstance(value, str) and re.fullmatch(
        r"[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}", value) is not None,
        "UUID_SHAPE")
    parsed = uuid.UUID(value)
    supported(parsed.int != 0, "UUID_ZERO")
    return str(parsed)


class Graph:
    def __init__(self):
        self.outputs = {}
        self.variables = {}
        self.binding_events = {}
        self.show = []
        self.previous = Value("Empty", None)
        self.action_count = 0
        self.neutral_nodes = set()
        self.groups = set()
        self.math_inputs = {}
        self.count_sources = {}

    def reference(self, ref):
        supported(isinstance(ref, dict), "REFERENCE_TYPE")
        kind = ref.get("Type")
        extra = set(ref) - {"Type", "OutputUUID", "OutputName", "VariableName", "Aggrandizements"}
        supported(not extra, "REFERENCE_FIELDS")
        if kind == "ActionOutput":
            supported("VariableName" not in ref, "REFERENCE_MIXED_IDENTITY")
            key = uuid_key(ref.get("OutputUUID"))
            supported(key in self.outputs, "REFERENCE_FORWARD_OR_MISSING")
            original, name, value = self.outputs[key]
            supported(original == ref["OutputUUID"], "REFERENCE_UUID_TEXT_MISMATCH")
            if "OutputName" in ref:
                supported(isinstance(ref["OutputName"], str), "OUTPUT_NAME_TYPE")
                if name is not None:
                    supported(ref["OutputName"] == name, "OUTPUT_NAME_MISMATCH")
        elif kind == "Variable":
            supported("OutputUUID" not in ref and "OutputName" not in ref, "REFERENCE_MIXED_IDENTITY")
            name = ref.get("VariableName")
            supported(isinstance(name, str) and name in self.variables, "VARIABLE_MISSING")
            value = self.variables[name]
        else:
            raise Unsupported("REFERENCE_KIND")
        # Text rendering of a single known scalar preserves its typed origin.
        # General coercions, properties and implicit external inputs are outside
        # these six contracts and must not become an unproved native claim.
        aggs = ref.get("Aggrandizements", [])
        supported(aggs in ([], [{"Type": "WFCoercionAggrandizement", "CoercionItemClass": "WFStringContentItem"}]),
                  "REFERENCE_AGGRANDIZEMENT")
        if aggs:
            supported(value.kind in ("Text", "Number"), "REFERENCE_TEXT_COERCION_KIND")
        return value

    def parameter(self, raw):
        if type(raw) is str:
            supported(len(raw.encode("utf-8")) <= MAX_SOURCE, "TEXT_LIMIT")
            return Value("Text", raw)
        if type(raw) in (int, float):
            return Value("Number", number(raw))
        supported(isinstance(raw, dict), "PARAMETER_KIND")
        supported(set(raw) == {"Value", "WFSerializationType"}, "PARAMETER_ENVELOPE")
        style = raw["WFSerializationType"]
        if style == "WFTextTokenAttachment":
            return self.reference(raw["Value"])
        supported(style == "WFTextTokenString", "PARAMETER_SERIALIZATION")
        value = raw["Value"]
        supported(isinstance(value, dict) and set(value) <= {"string", "attachmentsByRange"}, "TOKEN_STRING_FIELDS")
        text = value.get("string", "")
        attachments = value.get("attachmentsByRange", {})
        supported(isinstance(text, str) and isinstance(attachments, dict), "TOKEN_STRING_TYPES")
        units = text.encode("utf-16-le")
        refs = []
        for key, ref in attachments.items():
            match = re.fullmatch(r"\{(0|[1-9][0-9]*), 1\}", key)
            supported(match is not None, "TOKEN_RANGE_FORMAT")
            position = int(match.group(1)) * 2
            supported(position + 2 <= len(units) and units[position:position+2] == b"\xfc\xff", "TOKEN_RANGE_PLACEHOLDER")
            refs.append((position, self.reference(ref)))
        refs.sort(key=lambda item: item[0])
        supported(len({position for position, _ in refs}) == len(refs), "TOKEN_RANGE_OVERLAP")
        supported(sum(units[i:i+2] == b"\xfc\xff" for i in range(0, len(units), 2)) == len(refs), "TOKEN_UNBOUND_PLACEHOLDER")
        if text == "\ufffc" and len(refs) == 1:
            return refs[0][1]
        parts, last = [], 0
        for position, ref in refs:
            parts.extend([units[last:position].decode("utf-16-le"), render(ref)])
            last = position + 2
        parts.append(units[last:].decode("utf-16-le"))
        return merge_value("Text", "".join(parts), [ref for _, ref in refs])

    def bind(self, name, value):
        supported(isinstance(name, str) and 0 < len(name) <= 128, "BINDING_NAME")
        self.variables[name] = value
        self.binding_events.setdefault(name, []).append(value)

    def publish(self, params, value):
        name = params.get("CustomOutputName")
        if name is not None:
            supported(isinstance(name, str) and 0 < len(name) <= 128, "CUSTOM_OUTPUT_NAME")
            value = merge_value(value.kind, value.data, [value], binding=name)
            self.bind(name, value)
        if "UUID" in params:
            key = uuid_key(params["UUID"])
            supported(key not in self.outputs, "DUPLICATE_UUID")
            self.outputs[key] = (params["UUID"], name, value)
        return value

    def select(self, params, index):
        source = self.parameter(params.get("WFInput"))
        supported(source.kind == "List" and len(source.data) > 0, "SELECTION_LIST_KIND")
        specifier = params.get("WFItemSpecifier")
        if specifier == "Item At Index":
            raw_index = self.parameter(params.get("WFItemIndex"))
            supported(raw_index.kind in ("Text", "Number"), "SELECTION_INDEX_KIND")
            n = number(raw_index.data)
            supported(n == int(n), "SELECTION_NONINTEGER_INDEX")
            selected = int(n) - 1
        else:
            supported("WFItemIndex" not in params and specifier in ("First Item", "Last Item"), "SELECTION_SPECIFIER")
            selected = 0 if specifier == "First Item" else len(source.data) - 1
            raw_index = Value("Number", Decimal(selected+1))
        supported(0 <= selected < len(source.data), "SELECTION_OUT_OF_BOUNDS")
        item = source.data[selected]
        return merge_value(item.kind, item.data, [source, item, raw_index], index,
                           selection=(tuple(render(x) for x in source.data), selected, index))

    def index_guard(self, actions, index):
        """Adjudicate only the two nested numeric guards used by V2 indexing.

        The fixed structure and independent bounds/dataflow checks come from
        the indexing contract: 1 <= one_based_index <= count(the same list).
        This is not a general conditional or native predicate interpreter.
        """
        supported(index+9 <= len(actions), "INDEX_GUARD_TRUNCATED")
        kinds = ("conditional", "conditional", "getitemfromlist", "conditional", "nothing",
                 "conditional", "conditional", "nothing", "conditional")
        params = []
        for offset, kind in enumerate(kinds):
            action = actions[index+offset]
            supported(isinstance(action, dict) and set(action) <= {"WFWorkflowActionIdentifier", "WFWorkflowActionParameters"}
                      and action.get("WFWorkflowActionIdentifier") == PREFIX+kind, "INDEX_GUARD_ACTION_STRUCTURE")
            item = action.get("WFWorkflowActionParameters", {})
            supported(isinstance(item, dict) and set(item) <= ACTION_FIELDS[kind] | META, "INDEX_GUARD_ACTION_FIELDS")
            params.append(item)
        outer, inner = params[0].get("GroupingIdentifier"), params[1].get("GroupingIdentifier")
        keys = (uuid_key(outer), uuid_key(inner))
        supported(keys[0] != keys[1] and not self.groups.intersection(keys), "INDEX_GUARD_GROUP_REUSE")
        self.groups.update(keys)
        starts = []
        fields = {"GroupingIdentifier", "WFControlFlowMode", "WFCondition", "WFConditionalLegacyComparisonBehavior", "WFInput", "WFNumberValue"}
        for offset, boundary in ((0, 1.0), (1, 0.0)):
            item = params[offset]
            supported(set(item) == fields and type(item.get("WFControlFlowMode")) is int and item["WFControlFlowMode"] == 0
                      and type(item.get("WFCondition")) is int and item["WFCondition"] == 3
                      and item.get("WFConditionalLegacyComparisonBehavior") is True
                      and type(item.get("WFNumberValue")) is float and item["WFNumberValue"] == boundary,
                      "INDEX_GUARD_TYPED_PREDICATE")
            operand = item["WFInput"]
            supported(isinstance(operand, dict) and set(operand) == {"Type", "Variable"}
                      and operand["Type"] == "Variable", "INDEX_GUARD_INPUT_ENVELOPE")
            value = self.parameter(operand["Variable"])
            supported(value.kind == "Number", "INDEX_GUARD_NUMBER_INPUT")
            starts.append(value)
        for offset, group, mode in ((3, inner, 1), (5, inner, 2), (6, outer, 1), (8, outer, 2)):
            item = params[offset]
            expected = {"GroupingIdentifier", "WFControlFlowMode"} | (META if mode == 2 else set())
            supported(set(item) <= expected and item.get("GroupingIdentifier") == group
                      and type(item.get("WFControlFlowMode")) is int and item["WFControlFlowMode"] == mode,
                      "INDEX_GUARD_ENDPOINT")
            if mode == 2:
                supported("UUID" in item, "INDEX_GUARD_RESULT_UUID")
        supported(params[4] == {} and params[7] == {}, "INDEX_GUARD_EMPTY_BRANCH")
        selected_params = params[2]
        supported(set(selected_params) == {"WFInput", "WFItemIndex", "WFItemSpecifier", "UUID"}
                  and selected_params["WFItemSpecifier"] == "Item At Index"
                  and selected_params["WFItemIndex"] == params[0]["WFInput"]["Variable"], "INDEX_GUARD_SELECTION_IDENTITY")
        source = self.parameter(selected_params["WFInput"])
        supported(source.kind == "List", "INDEX_GUARD_SOURCE_LIST")
        one_based, difference = starts
        supported(one_based.data == int(one_based.data), "INDEX_GUARD_INTEGRAL_INDEX")
        supported(difference.operations and difference.operations[-1][3] in self.math_inputs, "INDEX_GUARD_DIFFERENCE_PRODUCER")
        op, count, compared_index = self.math_inputs[difference.operations[-1][3]]
        count_nodes = [node for node in count.nodes if node in self.count_sources]
        supported(op == "-" and len(count_nodes) == 1
                  and self.count_sources[count_nodes[0]] == source
                  and count.data == Decimal(len(source.data))
                  and compared_index == one_based, "INDEX_GUARD_CAUSAL_BOUNDS")
        active = one_based.data >= 1 and difference.data >= 0
        if active:
            value = self.select(selected_params, index+2)
            value = self.publish(selected_params, value)
        else:
            value = Value("Empty", None)
        # The output of the inner branch feeds the outer merge; no output from
        # an unexecuted get-item action is exposed as a live producer.
        value = merge_value(value.kind, value.data, [value, one_based, difference])
        value = replace(value, nodes=value.nodes | frozenset(range(index, index+9)))
        if one_based.data >= 1:
            value = self.publish(params[5], value)
        return value, params[8]

    def consume(self, workflow):
        supported(isinstance(workflow, dict) and set(workflow) <= TOP_FIELDS, "WORKFLOW_FIELDS")
        for key in ("WFWorkflowImportQuestions", "WFWorkflowTriggers", "WFQuickActionSurfaces"):
            supported(workflow.get(key) in (None, []), "WORKFLOW_EXTERNAL_BEHAVIOR")
        supported(workflow.get("WFWorkflowHasShortcutInputVariables", False) is False, "WORKFLOW_EXTERNAL_INPUT")
        supported(workflow.get("WFWorkflowHasOutputFallback", False) is False, "WORKFLOW_OUTPUT_FALLBACK")
        supported(workflow.get("WFWorkflowNoInputBehavior") in (None, {}), "WORKFLOW_INPUT_BEHAVIOR")
        actions = workflow.get("WFWorkflowActions")
        supported(isinstance(actions, list) and 0 < len(actions) <= MAX_ACTIONS, "ACTION_INVENTORY")
        # Check the complete original identity set, including untaken branches.
        declared = set()
        for action in actions:
            supported(isinstance(action, dict), "ACTION_ENVELOPE")
            parameters = action.get("WFWorkflowActionParameters", {})
            supported(isinstance(parameters, dict), "ACTION_PARAMETERS")
            if "UUID" in parameters:
                key = uuid_key(parameters["UUID"])
                supported(key not in declared, "DUPLICATE_UUID")
                declared.add(key)
        skip_until = 0
        for index, action in enumerate(actions):
            if index < skip_until:
                continue
            supported(isinstance(action, dict) and set(action) <= {"WFWorkflowActionIdentifier", "WFWorkflowActionParameters"}, "ACTION_ENVELOPE")
            identifier = action.get("WFWorkflowActionIdentifier")
            supported(isinstance(identifier, str) and identifier.startswith(PREFIX), "ACTION_IDENTIFIER")
            kind = identifier[len(PREFIX):]
            supported(kind in ACTION_FIELDS, "ACTION_OUTSIDE_SIX_TASK_SUBSET")
            params = action.get("WFWorkflowActionParameters", {})
            supported(isinstance(params, dict) and set(params) <= ACTION_FIELDS[kind] | META, "ACTION_PARAMETERS")
            if "WFSerializationType" in params:
                supported(params["WFSerializationType"] == "WFTextTokenAttachment", "VARIABLE_SERIALIZATION")
            name = params.get("CustomOutputName")
            supported(name is None or isinstance(name, str) and 0 < len(name) <= 128, "CUSTOM_OUTPUT_NAME")
            if kind == "nothing":
                value = Value("Empty", None, frozenset({index}))
                self.neutral_nodes.add(index)
            elif kind == "count":
                supported(params.get("WFCountType") == "Items", "COUNT_MODE")
                source = self.parameter(params.get("Input"))
                supported(source.kind == "List", "COUNT_LIST_KIND")
                self.count_sources[index] = source
                value = merge_value("Number", Decimal(len(source.data)), [source], index)
            elif kind == "conditional":
                value, params = self.index_guard(actions, index)
                name = params.get("CustomOutputName")
                skip_until = index+9
            elif kind == "gettext":
                value = self.parameter(params.get("WFTextActionText", ""))
                value = merge_value("Text", render(value), [value], index)
            elif kind == "number":
                raw = params.get("WFNumberActionNumber")
                value = self.parameter(raw)
                supported(value.kind in ("Text", "Number"), "NUMBER_ACTION_INPUT")
                value = merge_value("Number", number(value.data), [value], index)
            elif kind == "math":
                left = self.parameter(params.get("WFInput"))
                right = self.parameter(params.get("WFMathOperand"))
                supported(left.kind in ("Number", "Text") and right.kind in ("Number", "Text"), "MATH_INPUT_KIND")
                a, b = number(left.data), number(right.data)
                operation = params.get("WFMathOperation")
                supported(operation in ("+", "-", "×", "÷"), "MATH_OPERATION")
                supported(operation != "÷" or b != 0, "MATH_ZERO_DIVISION")
                result = {"+": lambda: a+b, "-": lambda: a-b, "×": lambda: a*b, "÷": lambda: a/b}[operation]()
                supported(result.is_finite() and abs(result) <= Decimal(2)**53, "MATH_RANGE")
                value = merge_value("Number", result, [left, right], index,
                                    operation=(operation, a, b, index))
                self.math_inputs[index] = (operation, left, right)
            elif kind == "list":
                items = params.get("WFItems")
                supported(isinstance(items, list) and 0 < len(items) <= 32, "LIST_ITEMS")
                values = []
                for item in items:
                    supported(isinstance(item, dict) and set(item) == {"WFItemType", "WFValue"}
                              and type(item["WFItemType"]) is int and item["WFItemType"] == 0, "LIST_ITEM_NATIVE_TYPE")
                    child = self.parameter(item["WFValue"])
                    supported(child.kind == "Text", "LIST_ITEM_VALUE_TYPE")
                    values.append(child)
                value = merge_value("List", tuple(values), values, index)
            elif kind in ("setvariable", "appendvariable"):
                variable = params.get("WFVariableName")
                supported(isinstance(variable, str), "VARIABLE_NAME")
                current = self.parameter(params["WFInput"]) if "WFInput" in params else self.previous
                if kind == "appendvariable":
                    supported(variable in self.variables, "APPEND_UNBOUND_VARIABLE")
                    old = self.variables[variable]
                    supported(old.kind in ("Empty", "List") and current.kind == "Text", "APPEND_LIST_KIND")
                    items = () if old.kind == "Empty" else old.data
                    value = merge_value("List", items + (current,), [old, current], index, variable)
                else:
                    value = merge_value(current.kind, current.data, [current], index, variable)
                self.bind(variable, value)
            elif kind == "getvariable":
                value = self.parameter(params.get("WFVariable"))
                value = merge_value(value.kind, value.data, [value], index)
            elif kind == "getitemfromlist":
                value = self.select(params, index)
            else:
                value = self.parameter(params.get("Text"))
                supported(value.kind in ("Text", "Number", "Empty"), "SHOW_VALUE_KIND")
                value = merge_value(value.kind, value.data, [value], index)
                self.show.append(value)
            if name is not None:
                value = merge_value(value.kind, value.data, [value], binding=name)
                if kind not in ("setvariable", "appendvariable") or name != params.get("WFVariableName"):
                    self.bind(name, value)
            if "UUID" in params:
                key = uuid_key(params["UUID"])
                supported(key not in self.outputs, "DUPLICATE_UUID")
                self.outputs[key] = (params["UUID"], name, value)
            self.previous = value
        self.action_count = len(actions)
        return self


def semantic_checks(workflow, task, parameters):
    graph = Graph().consume(workflow)
    oracle = task["oracle"]
    shown = graph.show[0] if len(graph.show) == 1 else None
    checks = {"compiles": True}
    expected_binding = oracle.get("requires_binding_reference")
    if parameters:
        expected_binding = parameters["binding"]
    for key, expected in oracle.items():
        passed = False
        if key == "visible_text":
            passed = shown is not None and shown.kind in ("Text", "Number") and render(shown) == expected
        elif key == "type":
            passed = shown is not None and shown.kind == expected
        elif key == "side_effects":
            passed = expected == ["show"] and len(graph.show) == 1
        elif key == "numeric_result":
            passed = shown is not None and shown.kind == "Number" and shown.data == number(expected)
        elif key == "requires_computation":
            passed = expected is True and shown is not None and bool(shown.operations)
        elif key == "requires_binding_reference":
            passed = shown is not None and expected in shown.bindings and expected in graph.binding_events
        elif key == "unchanged_binding":
            passed = True
            for name, value in expected.items():
                events = graph.binding_events.get(name, [])
                passed = passed and len(events) == 1 and events[0].kind == "Number" and events[0].data == number(value) and not events[0].operations
        elif key == "list":
            passed = shown is not None and any(items == tuple(expected) and selected == 1 for items, selected, _ in shown.selections)
        elif key == "selected_text":
            passed = shown is not None and shown.kind == "Text" and shown.data == expected and bool(shown.selections)
        else:
            raise Unsupported("ORACLE_KEY_OUTSIDE_SIX_TASK_CONTRACTS")
        checks["oracle."+key] = bool(passed)
    # Both arithmetic contracts require the specified operands, computation,
    # binding and final display to share one causal lineage. A disconnected
    # dummy addition or a precomputed literal cannot satisfy the contract.
    if parameters:
        causal = (shown is not None and expected_binding in shown.bindings
                  and any(op == "+" and (a, b) in ((number(parameters["left"]), number(parameters["right"])),
                                                   (number(parameters["right"]), number(parameters["left"])))
                          for op, a, b, _ in shown.operations))
        for key in ("numeric_result", "requires_computation", "requires_binding_reference"):
            if "oracle."+key in checks:
                checks["oracle."+key] = checks["oracle."+key] and causal
    # Every action was interpreted and classified above. Unused pure actions
    # cannot supply causal math/list/reference proof. For an edit, additional
    # disconnected work is outside this narrowly frozen edit oracle.
    consumed = set(shown.nodes) if shown else set()
    for name in oracle.get("unchanged_binding", {}):
        for value in graph.binding_events.get(name, []):
            consumed.update(value.nodes)
    complete = consumed | graph.neutral_nodes == set(range(graph.action_count))
    if task["kind"] == "edit" and not complete and all(checks.values()):
        raise Unsupported("EDIT_HAS_UNADJUDICATED_EXTRA_PURE_ACTIONS")
    return [{"name": name, "passed": checks[name]} for name in ["compiles"] + ["oracle."+key for key in sorted(oracle)]]


def safe_source(source):
    supported(isinstance(source, str) and 0 < len(source.encode("utf-8")) <= MAX_SOURCE, "SOURCE_SIZE")
    supported(re.search(r"#\s*(?:include|embed)\b", source) is None, "SOURCE_FILE_DIRECTIVE")
    # This is a conservative grammar for these six tasks, before the compiler
    # may open anything. It is not a second semantic oracle or an OS sandbox.
    # In particular, unknown syntax cannot hide calls behind comments, member
    # access, parenthesized callees, aliases, directives or unterminated strings.
    token = re.compile(r'''(?sx)
        (?P<space>[\ \t\r]+)|(?P<comment>//[^\n]*|/\*.*?\*/)|
        (?P<newline>\n)|(?P<string>"(?:\\.|[^"\\])*"|'(?:\\'|\\(?!')|[^'\\])*')|
        (?P<number>(?:[0-9]+(?:\.[0-9]*)?|\.[0-9]+)(?:[eE][+-]?[0-9]+)?)|
        (?P<word>[A-Za-z_][A-Za-z_0-9]*)|(?P<symbol>[@=():,;\[\]+*/<>-])
    ''')
    tokens = []
    position = 0
    while position < len(source):
        matched = token.match(source, position)
        supported(matched is not None, "SOURCE_TOKEN_OUTSIDE_SIX_TASK_SUBSET")
        position = matched.end()
        if matched.lastgroup not in {"space", "comment"}:
            tokens.append((matched.lastgroup, matched.group()))
            supported(len(tokens) <= 8192, "SOURCE_TOKEN_LIMIT")
    forbidden = {"import", "include", "native", "action", "macro", "asset", "embed", "func", "function", "extension"}
    supported(not any(kind == "word" and value in forbidden for kind, value in tokens), "SOURCE_EXTERNAL_OR_EXTENSION")
    index = 0
    def peek(value=None):
        return index < len(tokens) and (value is None or tokens[index][1] == value)
    def take(value=None):
        nonlocal index
        supported(peek(value), "SOURCE_GRAMMAR_OUTSIDE_SIX_TASK_SUBSET")
        result = tokens[index]
        index += 1
        return result
    def newlines():
        nonlocal index
        while index < len(tokens) and tokens[index][0] == "newline":
            index += 1
    def expression(depth=0):
        supported(depth <= 32, "SOURCE_EXPRESSION_DEPTH")
        if peek("+") or peek("-"):
            take()
            expression(depth+1)
            return
        if peek("("):
            take("("); newlines(); expression(depth+1); newlines(); take(")")
        elif peek("["):
            take("["); newlines()
            if not peek("]"):
                expression(depth+1); newlines()
                while peek(","):
                    take(","); newlines(); expression(depth+1); newlines()
            take("]")
        else:
            legacy_variable = bool(peek("@"))
            if legacy_variable:
                take("@")
            kind, value = take()
            supported(kind in ({"word"} if legacy_variable else {"word", "number", "string"}), "SOURCE_EXPRESSION_ATOM")
            if kind == "word" and peek("("):
                supported(not legacy_variable and value in {"show", "getListItem", "getFirstItem", "getLastItem"},
                          "SOURCE_CALL_OUTSIDE_SIX_TASK_SUBSET")
                take("("); newlines()
                if not peek(")"):
                    expression(depth+1); newlines()
                    while peek(","):
                        take(","); newlines(); expression(depth+1); newlines()
                take(")")
        while peek("["):
            take("["); newlines(); expression(depth+1); newlines(); take("]")
        if peek() and tokens[index][1] in {"+", "-", "*", "/"}:
            take(); expression(depth+1)
    statements = 0
    while index < len(tokens):
        newlines()
        if index == len(tokens):
            break
        statements += 1
        supported(statements <= 96, "SOURCE_STATEMENT_LIMIT")
        if peek("let") or peek("var") or peek("const") or peek("@"):
            take()
            supported(take()[0] == "word", "SOURCE_BINDING_NAME")
            if peek(":"):
                take(":")
                kind, value = take()
                supported(kind == "word" and value in {"Text", "Number", "Integer", "Float", "Bool", "List"}, "SOURCE_TYPE_OUTSIDE_SUBSET")
                if value == "List" and peek("<"):
                    take("<"); supported(take()[1] in {"Text", "Number", "Integer", "Float", "Bool"}, "SOURCE_TYPE_OUTSIDE_SUBSET"); take(">")
            take("="); expression()
        elif index+1 < len(tokens) and tokens[index][0] == "word" and tokens[index+1][1] == "=":
            take(); take("="); expression()
        else:
            expression()
        supported(not peek() or tokens[index][0] == "newline" or peek(";"), "SOURCE_STATEMENT_TERMINATOR")
        if peek(";"):
            take(";")
    supported(statements > 0, "SOURCE_EMPTY_PROGRAM")


def validation_request(request, bundle):
    require(isinstance(request, dict), "VALIDATION_REQUEST_TYPE")
    tasks = {task["id"]: task for task in bundle["suite"]["tasks"]}
    require(request.get("task_id") in tasks and request.get("arm") in ("legacy", "v2"), "VALIDATION_CASE")
    task = tasks[request["task_id"]]
    public = model_request(task, request["arm"], bundle)
    require(set(request) == {"schema", "request_id", "task_id", "arm", "source", "source_sha256", "oracle", "required_checks"}, "VALIDATION_FIELDS")
    require(request["schema"] == SCHEMA and request["request_id"] == public["request_id"], "VALIDATION_IDENTITY")
    require(request["oracle"] == task["oracle"] and request["required_checks"] == ["compiles"] + ["oracle."+key for key in sorted(task["oracle"])], "VALIDATION_ORACLE_CHANGED")
    require(isinstance(request["source"], str) and digest(request["source"].encode()) == request["source_sha256"], "VALIDATION_SOURCE_HASH")
    return task


def compiler_environment(directory):
    # The compiler gets operating-system runtime paths/locale only. Provider
    # credentials and mutable Cherri feature overrides are not inherited.
    allowed = {"SystemRoot", "SYSTEMROOT", "WINDIR", "LANG", "LC_ALL", "LC_CTYPE"}
    result = {key: value for key, value in os.environ.items() if key in allowed}
    scratch = Path(directory) / "compiler-temp"
    scratch.mkdir(exist_ok=False)
    result.update(TEMP=str(scratch), TMP=str(scratch))
    return result


def evaluate_request(request, bundle):
    task = validation_request(request, bundle)
    directory = claim_request(bundle, request, "validator")
    write_new(directory / "request.json", canonical(request))
    write_new(directory / "source.cherri", request["source"].encode())
    receipt = {"schema": "cherri-ai-graph-validation/v1", "request_id": request["request_id"],
               "task_id": task["id"], "arm": request["arm"], "source_sha256": request["source_sha256"],
               "compiler": bundle["manifest"]["compiler"], "started_at": utc_now(),
               "compiler_environment_profile": "isolated-defaults/v1",
               "synthetic": False, "apple_runtime_proved": False, "semantic_scope": "six-task-emitted-pure-graph"}
    try:
        safe_source(request["source"])
        binary = bundle["root"] / "compiler.exe"
        require(digest(read_regular(binary, 512 * 1024 * 1024)) == bundle["manifest"]["compiler"]["binary_sha256"], "COMPILER_BYTES_CHANGED")
        command = [str(binary), "compile", str(directory / "source.cherri"), "--mode="+request["arm"],
                   "--output", str(directory / "output.shortcut"), "--skip-sign"]
        receipt["command"] = command
        started = time.monotonic()
        with (directory / "compiler.stdout").open("xb") as stdout, (directory / "compiler.stderr").open("xb") as stderr:
            process = subprocess.run(command, cwd=directory, stdout=stdout, stderr=stderr,
                                     timeout=COMPILE_TIMEOUT, check=False, env=compiler_environment(directory))
        receipt.update(compile_returncode=process.returncode, compile_elapsed_seconds=round(time.monotonic()-started, 6))
        if process.returncode != 0:
            checks = [{"name": name, "passed": False} for name in request["required_checks"]]
            receipt["status"] = "COMPILE_REJECTED"
        else:
            raw = read_regular(directory / "output.shortcut", MAX_PLIST)
            receipt["graph_sha256"] = digest(raw)
            workflow = plistlib.loads(raw)
            checks = semantic_checks(workflow, task, bundle["parameters"].get(task["id"], {}))
            receipt["status"] = "ADJUDICATED"
        require(digest(read_regular(binary, 512 * 1024 * 1024)) == bundle["manifest"]["compiler"]["binary_sha256"], "COMPILER_CHANGED_DURING_VALIDATION")
        reply = {key: request[key] for key in ("schema", "request_id", "task_id", "arm", "source_sha256")}
        reply.update(synthetic=False, checks=checks)
        write_new(directory / "reply.json", canonical(reply))
        receipt["reply_sha256"] = digest(canonical(reply))
        return reply
    except (OSError, ValueError, subprocess.SubprocessError, plistlib.InvalidFileException) as error:
        receipt.update(status="UNADJUDICATED", error_type=type(error).__name__)
        if isinstance(error, BundleError):
            receipt["error_code"] = str(error)
        raise
    finally:
        originals, errors = {}, {}
        for name, limit in ORIGINAL_LIMITS.items():
            path = directory / name
            if not path.exists() and not path.is_symlink():
                continue
            try:
                raw = read_regular(path, limit)
                originals[name] = {"sha256": digest(raw), "bytes": len(raw), "complete": True}
            except (OSError, BundleError) as error:
                errors[name] = {"error_type": type(error).__name__}
                if isinstance(error, BundleError):
                    errors[name]["error_code"] = str(error)
        receipt.update(originals=originals, originals_complete=not errors, finished_at=utc_now())
        if errors:
            # Preserve oversized/changed originals in place. Their complete
            # bytes were not safely verified and cannot support adjudication.
            receipt.update(status="UNADJUDICATED", original_errors=errors,
                           error_code="VALIDATOR_ORIGINALS_INCOMPLETE")
        write_new(directory / "validator-receipt.json", canonical(receipt))
        if errors:
            raise BundleError("VALIDATOR_ORIGINALS_INCOMPLETE")


def main():
    try:
        raw = sys.stdin.buffer.read(1024*1024+1)
        require(len(raw) <= 1024*1024, "VALIDATION_REQUEST_LIMIT")
        bundle = load_bundle(os.environ.get("CHERRI_AI_EVAL_BUNDLE", ""))
        print(json.dumps(evaluate_request(unique_json(raw), bundle), ensure_ascii=False))
        return 0
    except (OSError, ValueError, subprocess.SubprocessError, plistlib.InvalidFileException):
        print("GRAPH_VALIDATION_UNADJUDICATED", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
