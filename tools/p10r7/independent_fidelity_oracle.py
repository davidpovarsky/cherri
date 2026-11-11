#!/usr/bin/env python3
"""
tools/p10r7/independent_fidelity_oracle.py
Cherri independent fidelity oracle (P10R8 hardening of the P10R7 oracle).

Strict Privacy & Fidelity Invariants:
- NEVER leaks raw private shortcut text, personal strings, URLs, tokens, or PII.
  Mismatch details contain only categories, type names and field paths.
- Operates independently from Cherri's serializer, compiler, or comparator implementations
  (pure plistlib/json; nothing from the Go code base is imported or trusted).
- Claims are separated:
    SOURCE_CONTAINER_EQUIVALENCE  decoded source JSON tree == decoded source plist tree
    STRICT_NATIVE_WIRE_FIDELITY   full typed tree equality modulo an audited bijective
                                  UUID remapping (UUID / OutputUUID / GroupingIdentifier)
    BEHAVIORAL_SEMANTIC_EQUIVALENCE  not claimed here (no semantic exemptions exist)
    APPLE_RUNTIME_EXECUTED        always BLOCKED_DEVICE without device evidence
- Any unexplained divergence is a hard FAIL, not a warning or auto-pass.

The 13-gate contract (GATES) is unchanged. P10R8 adds backward-compatible fields:
  extendedGates, gateMismatches (first mismatch per gate), gateMismatchCounts,
  mismatchCategoryCounts, sourceValidity.
"""

import argparse
import difflib
import hashlib
import json
import pathlib
import plistlib
import re
import sys
from typing import Any, Dict, List, Optional, Tuple

ORACLE_VERSION = "p10r8-fidelity-oracle-20261111.1"

GATES = [
    "RAW_JSON_DECODE_PASS",
    "BPLIST_OR_XML_DECODE_PASS",
    "DECOMPILE_PASS",
    "LEGACY_COMPILE_PASS",
    "V2_COMPILE_PASS",
    "STRICT_ACTION_SEQUENCE_PASS",
    "ACTION_PARAMETER_WIRE_PASS",
    "VARIABLE_GRAPH_PASS",
    "CONTROL_FLOW_PASS",
    "WORKFLOW_HEADER_PASS",
    "THIRD_PARTY_DESCRIPTOR_WIRE_PASS",
    "FULL_WORKFLOW_SEMANTIC_PASS",
    "APPLE_RUNTIME_EXECUTED",
]

# Backward-compatible additions (kept out of GATES so the 13-gate consumer contract holds).
EXTENDED_GATES = [
    "SOURCE_CONTAINER_EQUIVALENCE_PASS",
    "STRICT_NATIVE_WIRE_FIDELITY_PASS",
]

# Retained for consumers that import it. P10R8 compares ALL top-level keys; this list is
# no longer a filter.
HEADER_CRITICAL_KEYS = [
    "WFWorkflowTypes",
    "WFWorkflowTriggers",
    "WFWorkflowNoInputBehavior",
    "WFWorkflowInputContentItemClasses",
    "WFWorkflowOutputContentItemClasses",
    "WFWorkflowMinimumClientVersion",
    "WFWorkflowImportQuestions",
]

# Documented host-volatile exceptions: top-level keys proven to change per host run.
# Intentionally EMPTY: no field has been proven volatile. Adding one requires evidence
# and an adversarial test.
HOST_VOLATILE_HEADER_KEYS: frozenset = frozenset()

# Top-level array keys whose element order is not significant (documented set semantics).
UNORDERED_SET_PATHS = (
    "WFWorkflowTypes",
    "WFWorkflowInputContentItemClasses",
    "WFWorkflowOutputContentItemClasses",
    "WFQuickActionSurfaces",
)

# Structural paths (dictionary keys) that carry producer/reference identity. Only these are
# alpha-renamed through the bijective scope; every other string (even UUID-shaped) is exact.
PRODUCER_KEY = "UUID"
REFERENCE_KEY = "OutputUUID"
GROUP_KEY = "GroupingIdentifier"

DESCRIPTOR_KEYS = ("AppIntentDescriptor", "IntentAppDefinition")
CONTROL_FLOW_MODE_KEY = "WFControlFlowMode"
MAX_DIFFS = 400

UUID_REGEX = re.compile(r"^[0-9A-Fa-f]{8}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{12}$")
_RANGE_KEY = re.compile(r"\{\s*\d+\s*,\s*\d+\s*\}")


def sha256_file(p: pathlib.Path) -> str:
    h = hashlib.sha256()
    with open(p, "rb") as f:
        while chunk := f.read(65536):
            h.update(chunk)
    return h.hexdigest()


def sha256_data(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()


_SAFE_COMPONENT = re.compile(r"^[A-Za-z0-9_\-]{1,48}(\[\d+\])*$")


def redact_path(path: str) -> str:
    """Make a field path safe to publish: no text-range keys, no free-form dictionary keys.

    Components that are not plain identifier-like names (possible user-authored dictionary keys)
    are replaced with <k>. Action indices and structural key names are preserved."""
    path = _RANGE_KEY.sub("{r}", path)
    parts = []
    for comp in path.split("."):
        if comp == "" or comp == "{r}" or _SAFE_COMPONENT.match(comp) or comp.startswith("{r}"):
            parts.append(comp)
        else:
            parts.append("<k>")
    return ".".join(parts)


class BijectiveUUIDScope:
    """Audited one-to-one producer/group identity mapping between source and rebuilt."""

    def __init__(self):
        self.src_to_reb: Dict[str, str] = {}
        self.reb_to_src: Dict[str, str] = {}
        self.src_action_uuid_to_idx: Dict[str, int] = {}
        self.reb_action_uuid_to_idx: Dict[str, int] = {}
        self.group_src_to_reb: Dict[str, str] = {}
        self.group_reb_to_src: Dict[str, str] = {}
        self.allow_preserved_dangling: bool = False

    def register_action_uuid(self, src_uuid: str, reb_uuid: str, action_idx: int) -> Optional[str]:
        if not src_uuid or not reb_uuid:
            return None
        self.src_action_uuid_to_idx[src_uuid] = action_idx
        self.reb_action_uuid_to_idx[reb_uuid] = action_idx

        if src_uuid in self.src_to_reb and self.src_to_reb[src_uuid] != reb_uuid:
            return "UUID collision: one source producer mapped to multiple rebuilt producers"
        if reb_uuid in self.reb_to_src and self.reb_to_src[reb_uuid] != src_uuid:
            return "UUID collision: one rebuilt producer mapped from multiple source producers"

        self.src_to_reb[src_uuid] = reb_uuid
        self.reb_to_src[reb_uuid] = src_uuid
        return None

    def register_group_uuid(self, src_group: str, reb_group: str) -> Optional[str]:
        if not src_group or not reb_group:
            return None
        if src_group in self.group_src_to_reb and self.group_src_to_reb[src_group] != reb_group:
            return "Grouping collision: one source group mapped to multiple rebuilt groups"
        if reb_group in self.group_reb_to_src and self.group_reb_to_src[reb_group] != src_group:
            return "Grouping collision: one rebuilt group mapped from multiple source groups"
        self.group_src_to_reb[src_group] = reb_group
        self.group_reb_to_src[reb_group] = src_group
        return None

    def match_uuid(self, src_uuid: str, reb_uuid: str) -> bool:
        """Fail-closed: only producers registered by the graph pass can be matched.

        Unknown identities are never registered on the fly (P10R7 did, which let
        unverified references pass)."""
        if not src_uuid or not reb_uuid:
            return src_uuid == reb_uuid
        return self.src_to_reb.get(src_uuid) == reb_uuid and self.reb_to_src.get(reb_uuid) == src_uuid

    def match_reference_uuid(self, src_uuid: str, reb_uuid: str) -> bool:
        """Match an OutputUUID reference against producer mapping, allowing only exact
        reproduction of pre-existing dangling source references after graph verification."""
        if self.match_uuid(src_uuid, reb_uuid):
            return True
        if (
            self.allow_preserved_dangling
            and src_uuid == reb_uuid
            and src_uuid not in self.src_action_uuid_to_idx
            and reb_uuid not in self.reb_action_uuid_to_idx
        ):
            return True
        return False

    def match_group(self, src_group: str, reb_group: str) -> bool:
        if not src_group or not reb_group:
            return src_group == reb_group
        if src_group in self.group_src_to_reb:
            return self.group_src_to_reb[src_group] == reb_group
        if reb_group in self.group_reb_to_src:
            return self.group_reb_to_src[reb_group] == src_group
        self.group_src_to_reb[src_group] = reb_group
        self.group_reb_to_src[reb_group] = src_group
        return True


# ---------------------------------------------------------------------------
# Tree walking / comparison
# ---------------------------------------------------------------------------

def collect_references(val: Any, path: str = "") -> List[Tuple[str, str]]:
    """All (path, uuid) of OutputUUID reference sites, in deterministic traversal order."""
    out: List[Tuple[str, str]] = []
    if isinstance(val, dict):
        for k in sorted(val.keys(), key=str):
            sub = f"{path}.{k}" if path else str(k)
            v = val[k]
            if k == REFERENCE_KEY and isinstance(v, str):
                out.append((sub, v))
            else:
                out.extend(collect_references(v, sub))
    elif isinstance(val, list):
        for i, e in enumerate(val):
            out.extend(collect_references(e, f"{path}[{i}]"))
    return out


def extract_output_uuids(val: Any) -> List[str]:
    """Compat helper: OutputUUID strings in a parameter tree."""
    return [u for _, u in collect_references(val)]


def _tname(v: Any) -> str:
    return type(v).__name__


def diff_values(
    src_val: Any,
    reb_val: Any,
    path: str,
    scope: BijectiveUUIDScope,
    out: List[Dict[str, str]],
    key: Optional[str] = None,
    json_numbers: bool = False,
) -> None:
    """Collect ALL typed differences (bounded by MAX_DIFFS). Details never contain values."""
    if len(out) >= MAX_DIFFS:
        return

    def add(cat: str, detail: str):
        out.append({"category": cat, "path": redact_path(path), "detail": detail})

    if type(src_val) != type(reb_val):
        if json_numbers and isinstance(src_val, (int, float)) and isinstance(reb_val, (int, float)) \
                and not isinstance(src_val, bool) and not isinstance(reb_val, bool):
            if src_val == reb_val:
                return
        add("PARAM_TYPE_CHANGE", f"Type mismatch: {_tname(src_val)} vs {_tname(reb_val)}")
        return

    if isinstance(src_val, dict):
        for k in sorted(set(src_val) | set(reb_val), key=str):
            subpath = f"{path}.{k}" if path else str(k)
            if k not in src_val:
                out.append({"category": "EXTRA_PARAM_KEY", "path": redact_path(subpath), "detail": "Key added in rebuilt"})
            elif k not in reb_val:
                out.append({"category": "MISSING_PARAM_KEY", "path": redact_path(subpath), "detail": "Key missing in rebuilt"})
            else:
                diff_values(src_val[k], reb_val[k], subpath, scope, out, key=str(k), json_numbers=json_numbers)
            if len(out) >= MAX_DIFFS:
                return
        return

    if isinstance(src_val, list):
        if len(src_val) != len(reb_val):
            add("PARAM_VALUE_CHANGE", f"List length mismatch: {len(src_val)} vs {len(reb_val)}")
            return
        if path in UNORDERED_SET_PATHS and all(isinstance(x, str) for x in src_val + reb_val):
            if sorted(src_val) != sorted(reb_val):
                add("SET_MEMBERSHIP_CHANGE", "Set membership mismatch")
            return
        for idx, (s_elem, r_elem) in enumerate(zip(src_val, reb_val)):
            diff_values(s_elem, r_elem, f"{path}[{idx}]", scope, out, key=None, json_numbers=json_numbers)
        return

    if isinstance(src_val, str):
        if key == GROUP_KEY:
            if not scope.match_group(src_val, reb_val):
                add("REFERENCE_CHANGE", "GroupingIdentifier does not match bijective group mapping")
            return
        if key == PRODUCER_KEY and not json_numbers:
            if not scope.match_uuid(src_val, reb_val):
                add("REFERENCE_CHANGE", f"{key} does not match bijective producer mapping")
            return
        if key == REFERENCE_KEY and not json_numbers:
            if not scope.match_reference_uuid(src_val, reb_val):
                add("REFERENCE_CHANGE", f"{key} does not match bijective producer mapping")
            return
        if src_val != reb_val:
            add("PARAM_VALUE_CHANGE", "String value mismatch")
        return

    if isinstance(src_val, (bytes, bytearray)):
        if src_val != reb_val:
            add("PARAM_VALUE_CHANGE", "Binary data mismatch")
        return

    if src_val != reb_val:
        add("PARAM_VALUE_CHANGE", f"{_tname(src_val)} value mismatch")


def compare_values(
    src_val: Any,
    reb_val: Any,
    path: str,
    scope: BijectiveUUIDScope,
    key: Optional[str] = None,
) -> Tuple[bool, Optional[str], Optional[str], Optional[str]]:
    """Back-compat wrapper: (is_equal, category, path, detail) of the first difference."""
    diffs: List[Dict[str, str]] = []
    diff_values(src_val, reb_val, path, scope, diffs, key=key)
    if not diffs:
        return True, None, None, None
    d = diffs[0]
    return False, d["category"], d["path"], d["detail"]


def validate_control_flow(actions: List[Any]) -> Optional[Dict[str, Any]]:
    """Stack/group validator. Returns first violation or None when the flow is well formed.

    Modes: 0 = start, 1 = middle (otherwise/menu item), 2 = end."""
    open_groups: Dict[str, int] = {}
    for i, act in enumerate(actions):
        params = act.get("WFWorkflowActionParameters") if isinstance(act, dict) else None
        if not isinstance(params, dict) or CONTROL_FLOW_MODE_KEY not in params:
            continue
        mode = params.get(CONTROL_FLOW_MODE_KEY)
        group = params.get(GROUP_KEY)
        if not isinstance(group, str) or not group:
            return {"index": i, "category": "CONTROL_FLOW_NO_GROUP"}
        if mode == 0:
            if group in open_groups:
                return {"index": i, "category": "CONTROL_FLOW_DUPLICATE_START"}
            open_groups[group] = i
        elif mode == 1:
            if group not in open_groups:
                return {"index": i, "category": "CONTROL_FLOW_ORPHAN_MIDDLE"}
        elif mode == 2:
            if group not in open_groups:
                return {"index": i, "category": "CONTROL_FLOW_ORPHAN_END"}
            del open_groups[group]
        else:
            return {"index": i, "category": "CONTROL_FLOW_UNKNOWN_MODE"}
    if open_groups:
        return {"index": min(open_groups.values()), "category": "CONTROL_FLOW_UNCLOSED_START"}
    return None


def _params(act: Any) -> Dict[str, Any]:
    if isinstance(act, dict) and isinstance(act.get("WFWorkflowActionParameters"), dict):
        return act["WFWorkflowActionParameters"]
    return {}


def _ident(act: Any) -> Any:
    return act.get("WFWorkflowActionIdentifier") if isinstance(act, dict) else None


def evaluate_shortcut_fidelity(
    src_doc: Dict[str, Any],
    reb_doc: Dict[str, Any],
    json_doc: Optional[Dict[str, Any]] = None,
    allow_lowering_rules: bool = False,
) -> Dict[str, Any]:
    """Evaluate the 13 contract gates + extended gates on two decoded shortcut documents.

    `allow_lowering_rules` is accepted for API compatibility and deliberately IGNORED: no
    lowering rule can turn a wire difference into a PASS (see M34/M35)."""
    gates = {g: "PENDING" for g in GATES}
    first_mismatch: Optional[Dict[str, Any]] = None
    gate_mismatches: Dict[str, Dict[str, Any]] = {}
    gate_counts: Dict[str, int] = {}
    category_counts: Dict[str, int] = {}

    def record(gate: str, cat: str, path: str, detail: str, sub: Optional[str] = None):
        nonlocal first_mismatch
        gates[gate] = "FAIL"
        gate_counts[gate] = gate_counts.get(gate, 0) + 1
        key = sub or cat
        category_counts[key] = category_counts.get(key, 0) + 1
        entry = {"gate": gate, "category": cat, "path": redact_path(path), "detail": detail}
        if sub:
            entry["subcategory"] = sub
        if gate not in gate_mismatches:
            gate_mismatches[gate] = entry
        if first_mismatch is None:
            first_mismatch = entry

    if not isinstance(src_doc, dict) or not isinstance(reb_doc, dict):
        raise TypeError("source and rebuilt documents must be decoded dictionaries")

    # Gate 1: RAW_JSON_DECODE_PASS == SOURCE_CONTAINER_EQUIVALENCE (full decoded tree)
    extended = {g: "PENDING" for g in EXTENDED_GATES}
    if json_doc is not None:
        container_diffs: List[Dict[str, str]] = []
        diff_values(json_doc, src_doc, "", BijectiveUUIDScope(), container_diffs, json_numbers=True)
        if container_diffs:
            for d in container_diffs[:1]:
                record("RAW_JSON_DECODE_PASS", "JSON_PLIST_MISMATCH", d["path"] or "<root>", d["detail"], d["category"])
            gate_counts["RAW_JSON_DECODE_PASS"] = len(container_diffs)
            extended["SOURCE_CONTAINER_EQUIVALENCE_PASS"] = "FAIL"
        else:
            gates["RAW_JSON_DECODE_PASS"] = "PASS"
            extended["SOURCE_CONTAINER_EQUIVALENCE_PASS"] = "PASS"
    else:
        gates["RAW_JSON_DECODE_PASS"] = "NOT_APPLICABLE"
        extended["SOURCE_CONTAINER_EQUIVALENCE_PASS"] = "NOT_APPLICABLE"

    # Gate 2: BPLIST_OR_XML_DECODE_PASS
    if isinstance(src_doc.get("WFWorkflowActions"), list):
        gates["BPLIST_OR_XML_DECODE_PASS"] = "PASS"
    else:
        record("BPLIST_OR_XML_DECODE_PASS", "DECODE_ERROR", "WFWorkflowActions", "Source document lacks valid WFWorkflowActions list")

    # Gate 3: WORKFLOW_HEADER_PASS - every top-level key except WFWorkflowActions
    header_diffs: List[Dict[str, str]] = []
    src_header = {k: v for k, v in src_doc.items() if k != "WFWorkflowActions" and k not in HOST_VOLATILE_HEADER_KEYS}
    reb_header = {k: v for k, v in reb_doc.items() if k != "WFWorkflowActions" and k not in HOST_VOLATILE_HEADER_KEYS}
    diff_values(src_header, reb_header, "", BijectiveUUIDScope(), header_diffs)
    if header_diffs:
        for d in header_diffs:
            record("WORKFLOW_HEADER_PASS", "HEADER_CHANGE", d["path"], d["detail"], d["category"])
    else:
        gates["WORKFLOW_HEADER_PASS"] = "PASS"

    src_actions = src_doc.get("WFWorkflowActions", []) if isinstance(src_doc.get("WFWorkflowActions"), list) else []
    reb_actions = reb_doc.get("WFWorkflowActions", []) if isinstance(reb_doc.get("WFWorkflowActions"), list) else []
    src_count = len(src_actions)
    reb_count = len(reb_actions)
    action_delta = reb_count - src_count
    src_ids = [_ident(a) for a in src_actions]
    reb_ids = [_ident(a) for a in reb_actions]
    min_len = min(src_count, reb_count)

    # Gate 4: STRICT_ACTION_SEQUENCE_PASS
    if src_count == reb_count and src_ids == reb_ids:
        gates["STRICT_ACTION_SEQUENCE_PASS"] = "PASS"
    elif src_count != reb_count:
        record("STRICT_ACTION_SEQUENCE_PASS", "ACTION_COUNT_MISMATCH", f"WFWorkflowActions (len {src_count} vs {reb_count})", f"Action count delta: {action_delta}")
    else:
        first_idx = next(i for i, (s, r) in enumerate(zip(src_ids, reb_ids)) if s != r)
        record("STRICT_ACTION_SEQUENCE_PASS", "REPLACED_IDENTIFIER", f"WFWorkflowActions[{first_idx}].WFWorkflowActionIdentifier", "Action identifier changed at index")

    # Gate 5: VARIABLE_GRAPH_PASS - producer presence, uniqueness, bijection, reference edges
    scope = BijectiveUUIDScope()
    src_producers: Dict[str, int] = {}
    reb_producers: Dict[str, int] = {}
    graph_failed = False
    dup_counts = {"source": 0, "rebuilt": 0}

    def graph_fail(cat: str, path: str, detail: str):
        nonlocal graph_failed
        graph_failed = True
        record("VARIABLE_GRAPH_PASS", "REFERENCE_CHANGE", path, detail, cat)

    for side, acts, prod in (("source", src_actions, src_producers), ("rebuilt", reb_actions, reb_producers)):
        for i, act in enumerate(acts):
            u = _params(act).get(PRODUCER_KEY)
            if isinstance(u, str) and u:
                if u in prod:
                    # A duplicate producer is only tolerated when the OTHER side reproduces the exact
                    # same duplicate pattern; the bijection below rejects any asymmetric duplicate
                    # (M44). Source duplicates are surfaced in sourceValidity, never hidden.
                    dup_counts[side] += 1
                else:
                    prod[u] = i
            elif PRODUCER_KEY in _params(act):
                graph_fail("INVALID_PRODUCER", f"WFWorkflowActions[{i}].UUID", f"Non-string or empty producer UUID in {side}")

    if src_count != reb_count:
        graph_fail("GRAPH_LENGTH_MISMATCH", "WFWorkflowActions", "Variable graph mismatch due to action count delta")
    for i in range(min_len):
        s_has = PRODUCER_KEY in _params(src_actions[i])
        r_has = PRODUCER_KEY in _params(reb_actions[i])
        if s_has != r_has:
            graph_fail("UUID_PRESENCE_CHANGE", f"WFWorkflowActions[{i}].UUID", "Producer UUID presence differs from source")
            continue
        s_u = _params(src_actions[i]).get(PRODUCER_KEY)
        r_u = _params(reb_actions[i]).get(PRODUCER_KEY)
        if isinstance(s_u, str) and isinstance(r_u, str) and s_u and r_u:
            err = scope.register_action_uuid(s_u, r_u, i)
            if err:
                graph_fail("UUID_COLLISION", f"WFWorkflowActions[{i}].UUID", err)

    source_dangling = 0
    if src_count == reb_count:
        for i in range(min_len):
            s_refs = collect_references(_params(src_actions[i]))
            r_refs = collect_references(_params(reb_actions[i]))
            if len(s_refs) != len(r_refs):
                graph_fail("EDGE_COUNT_CHANGE", f"WFWorkflowActions[{i}].OutputUUID", f"Reference edge count mismatch: {len(s_refs)} vs {len(r_refs)}")
                continue
            for (s_path, s_u), (r_path, r_u) in zip(s_refs, r_refs):
                if s_path != r_path:
                    graph_fail("EDGE_PLACEMENT_CHANGE", f"WFWorkflowActions[{i}].{s_path}", "Reference moved to a different parameter path")
                    continue
                if s_u not in src_producers:
                    # Source itself dangles: the rebuilt literal must be identical and also dangle.
                    source_dangling += 1
                    if r_u != s_u or r_u in reb_producers:
                        graph_fail("LOST_DANGLING_REFERENCE", f"WFWorkflowActions[{i}].{s_path}", "Source dangling reference not reproduced exactly")
                    continue
                if r_u not in reb_producers:
                    graph_fail("DANGLING_REFERENCE", f"WFWorkflowActions[{i}].{r_path}", "Rebuilt reference points to a non-existent producer")
                elif not scope.match_uuid(s_u, r_u):
                    graph_fail("EDGE_CHANGE", f"WFWorkflowActions[{i}].{r_path}", "Reference edge does not map to the corresponding producer")
    if not graph_failed:
        gates["VARIABLE_GRAPH_PASS"] = "PASS"
    if src_count == reb_count:
        scope.allow_preserved_dangling = True

    # Gate 6: CONTROL_FLOW_PASS
    src_cf_violation = validate_control_flow(src_actions)
    reb_cf_violation = validate_control_flow(reb_actions)
    source_validity = {
        "controlFlowValid": src_cf_violation is None,
        "controlFlowViolation": (
            {"index": src_cf_violation["index"], "category": src_cf_violation["category"]} if src_cf_violation else None
        ),
        "danglingReferenceCount": source_dangling,
        "duplicateProducerUuidCount": dup_counts["source"],
    }
    cf_failed = False

    def cf_fail(cat: str, path: str, detail: str):
        nonlocal cf_failed
        cf_failed = True
        record("CONTROL_FLOW_PASS", "CONTROL_FLOW_CHANGE", path, detail, cat)

    if src_cf_violation:
        cf_fail("SOURCE_CONTROL_FLOW_INVALID", f"WFWorkflowActions[{src_cf_violation['index']}].{CONTROL_FLOW_MODE_KEY}",
                f"Source control flow is malformed ({src_cf_violation['category']}); never a PASS")
    if reb_cf_violation and not src_cf_violation:
        cf_fail("REBUILT_CONTROL_FLOW_INVALID", f"WFWorkflowActions[{reb_cf_violation['index']}].{CONTROL_FLOW_MODE_KEY}",
                f"Rebuilt control flow is malformed ({reb_cf_violation['category']})")
    cf_scope = BijectiveUUIDScope()
    for i in range(min_len):
        sp = _params(src_actions[i])
        rp = _params(reb_actions[i])
        if CONTROL_FLOW_MODE_KEY not in sp and CONTROL_FLOW_MODE_KEY not in rp and GROUP_KEY not in sp and GROUP_KEY not in rp:
            continue
        path = f"WFWorkflowActions[{i}]"
        if (CONTROL_FLOW_MODE_KEY in sp) != (CONTROL_FLOW_MODE_KEY in rp) or sp.get(CONTROL_FLOW_MODE_KEY) != rp.get(CONTROL_FLOW_MODE_KEY) \
                or type(sp.get(CONTROL_FLOW_MODE_KEY)) != type(rp.get(CONTROL_FLOW_MODE_KEY)):
            cf_fail("CONTROL_FLOW_MODE_CHANGE", f"{path}.{CONTROL_FLOW_MODE_KEY}", "WFControlFlowMode differs")
        if (GROUP_KEY in sp) != (GROUP_KEY in rp):
            cf_fail("GROUP_PRESENCE_CHANGE", f"{path}.{GROUP_KEY}", "GroupingIdentifier presence differs")
        elif GROUP_KEY in sp and isinstance(sp[GROUP_KEY], str) and isinstance(rp[GROUP_KEY], str):
            if not cf_scope.match_group(sp[GROUP_KEY], rp[GROUP_KEY]):
                cf_fail("GROUP_MAPPING_CHANGE", f"{path}.{GROUP_KEY}", "GroupingIdentifier does not follow bijective group mapping")
        elif GROUP_KEY in sp and type(sp[GROUP_KEY]) != type(rp[GROUP_KEY]):
            cf_fail("GROUP_TYPE_CHANGE", f"{path}.{GROUP_KEY}", "GroupingIdentifier type differs")
    if src_count != reb_count:
        cf_fail("CONTROL_FLOW_LENGTH_DIVERGENCE", "WFWorkflowActions", "Control flow length divergence")
    if not cf_failed:
        gates["CONTROL_FLOW_PASS"] = "PASS"
    # A GroupingIdentifier mapping must be bijective across the whole flow, shared with the wire pass.
    scope.group_src_to_reb.update(cf_scope.group_src_to_reb)
    scope.group_reb_to_src.update(cf_scope.group_reb_to_src)

    # Gate 7: THIRD_PARTY_DESCRIPTOR_WIRE_PASS - INDEXED, never a content multiset
    src_desc_count = 0
    reb_desc_count = 0
    desc_failed = False
    for i in range(max(src_count, reb_count)):
        sp = _params(src_actions[i]) if i < src_count else {}
        rp = _params(reb_actions[i]) if i < reb_count else {}
        for dk in DESCRIPTOR_KEYS:
            if dk in sp:
                src_desc_count += 1
            if dk in rp:
                reb_desc_count += 1
            if dk not in sp and dk not in rp:
                continue
            path = f"WFWorkflowActions[{i}].{dk}"
            if dk in sp and dk not in rp:
                desc_failed = True
                record("THIRD_PARTY_DESCRIPTOR_WIRE_PASS", "DESCRIPTOR_MISMATCH", path, f"{dk} missing in rebuilt at the same action index")
            elif dk in rp and dk not in sp:
                desc_failed = True
                record("THIRD_PARTY_DESCRIPTOR_WIRE_PASS", "DESCRIPTOR_MISMATCH", path, f"{dk} added in rebuilt at an index where source has none")
            else:
                dd: List[Dict[str, str]] = []
                diff_values(sp[dk], rp[dk], path, scope, dd, key=dk)
                for d in dd:
                    desc_failed = True
                    record("THIRD_PARTY_DESCRIPTOR_WIRE_PASS", "DESCRIPTOR_MISMATCH", d["path"], f"Descriptor content mismatch: {d['category']} - {d['detail']}", d["category"])
    if src_desc_count == 0 and reb_desc_count == 0:
        gates["THIRD_PARTY_DESCRIPTOR_WIRE_PASS"] = "NOT_APPLICABLE"
    elif not desc_failed and src_desc_count == reb_desc_count:
        gates["THIRD_PARTY_DESCRIPTOR_WIRE_PASS"] = "PASS"
    elif not desc_failed:
        record("THIRD_PARTY_DESCRIPTOR_WIRE_PASS", "DESCRIPTOR_MISMATCH", "WFWorkflowActions", f"Descriptor count mismatch: {src_desc_count} vs {reb_desc_count}")

    # Gate 8: ACTION_PARAMETER_WIRE_PASS - full typed parameter tree INCLUDING UUID / CustomOutputName
    if src_count != reb_count:
        record("ACTION_PARAMETER_WIRE_PASS", "ACTION_COUNT_MISMATCH", "WFWorkflowActions", f"Cannot compare parameters 1:1 due to action count delta ({src_count} vs {reb_count})")
    else:
        wire_failed = False
        for i in range(src_count):
            s_act, r_act = src_actions[i], reb_actions[i]
            base = f"WFWorkflowActions[{i}]"
            if isinstance(s_act, dict) and isinstance(r_act, dict):
                extra_keys = (set(s_act) ^ set(r_act)) - {"WFWorkflowActionIdentifier", "WFWorkflowActionParameters"}
                for k in sorted(extra_keys, key=str):
                    wire_failed = True
                    cat = "EXTRA_PARAM_KEY" if k in r_act else "MISSING_PARAM_KEY"
                    record("ACTION_PARAMETER_WIRE_PASS", cat, f"{base}.{k}", "Action-level key presence differs", cat)
                if ("WFWorkflowActionParameters" in s_act) != ("WFWorkflowActionParameters" in r_act):
                    wire_failed = True
                    record("ACTION_PARAMETER_WIRE_PASS", "PARAM_CONTAINER_PRESENCE", f"{base}.WFWorkflowActionParameters", "Parameter container presence differs")
            wd: List[Dict[str, str]] = []
            diff_values(_params(s_act), _params(r_act), f"{base}.WFWorkflowActionParameters", scope, wd)
            for d in wd:
                wire_failed = True
                record("ACTION_PARAMETER_WIRE_PASS", d["category"], d["path"], d["detail"])
        if not wire_failed:
            gates["ACTION_PARAMETER_WIRE_PASS"] = "PASS"

    # Gate 9: FULL_WORKFLOW_SEMANTIC_PASS == STRICT_NATIVE_WIRE_FIDELITY (no semantic exemptions)
    required = [
        "STRICT_ACTION_SEQUENCE_PASS",
        "ACTION_PARAMETER_WIRE_PASS",
        "VARIABLE_GRAPH_PASS",
        "CONTROL_FLOW_PASS",
        "WORKFLOW_HEADER_PASS",
        "BPLIST_OR_XML_DECODE_PASS",
    ]
    if gates["THIRD_PARTY_DESCRIPTOR_WIRE_PASS"] != "NOT_APPLICABLE":
        required.append("THIRD_PARTY_DESCRIPTOR_WIRE_PASS")
    if gates["RAW_JSON_DECODE_PASS"] != "NOT_APPLICABLE":
        required.append("RAW_JSON_DECODE_PASS")
    strict_ok = all(gates[g] == "PASS" for g in required)
    gates["FULL_WORKFLOW_SEMANTIC_PASS"] = "PASS" if strict_ok else "FAIL"
    extended["STRICT_NATIVE_WIRE_FIDELITY_PASS"] = "PASS" if strict_ok else "FAIL"

    # Never inferred from compiler/CI evidence.
    gates["APPLE_RUNTIME_EXECUTED"] = "BLOCKED_DEVICE"

    return {
        "oracleVersion": ORACLE_VERSION,
        "gates": gates,
        "extendedGates": extended,
        "sourceActions": src_count,
        "compiledActions": reb_count,
        "actionDelta": action_delta,
        "sourceDescriptors": src_desc_count,
        "compiledDescriptors": reb_desc_count,
        "firstMismatch": first_mismatch,
        "gateMismatches": gate_mismatches,
        "gateMismatchCounts": gate_counts,
        "mismatchCategoryCounts": category_counts,
        "sourceValidity": source_validity,
    }


def aligned_action_inventory(src_doc: Dict[str, Any], reb_doc: Dict[str, Any]) -> List[Dict[str, Any]]:
    """Privacy-safe forensic rows over a SEQUENCE ALIGNMENT of action identifiers.

    Index-shifted comparisons make unrelated actions look mutated; this aligns first and
    reports only key names, type names and categories (never values)."""
    sa = src_doc.get("WFWorkflowActions", [])
    ra = reb_doc.get("WFWorkflowActions", [])
    sm = difflib.SequenceMatcher(a=[_ident(a) for a in sa], b=[_ident(a) for a in ra], autojunk=False)
    rows: List[Dict[str, Any]] = []
    ops = sm.get_opcodes()
    scope = BijectiveUUIDScope()
    for tag, i1, i2, j1, j2 in ops:
        if tag == "equal":
            for off in range(i2 - i1):
                su = _params(sa[i1 + off]).get(PRODUCER_KEY)
                ru = _params(ra[j1 + off]).get(PRODUCER_KEY)
                if isinstance(su, str) and isinstance(ru, str):
                    scope.register_action_uuid(su, ru, i1 + off)
    for tag, i1, i2, j1, j2 in ops:
        if tag == "equal":
            for off in range(i2 - i1):
                si, ri = i1 + off, j1 + off
                diffs: List[Dict[str, str]] = []
                diff_values(_params(sa[si]), _params(ra[ri]), "", scope, diffs)
                if diffs:
                    rows.append({
                        "sourceIndex": si, "rebuiltIndex": ri, "identifier": _ident(sa[si]), "op": "aligned",
                        "diffs": [{"category": d["category"], "path": d["path"]} for d in diffs],
                    })
        else:
            for si in range(i1, i2):
                rows.append({"sourceIndex": si, "rebuiltIndex": None, "identifier": _ident(sa[si]), "op": "removed"})
            for ri in range(j1, j2):
                rows.append({"sourceIndex": None, "rebuiltIndex": ri, "identifier": _ident(ra[ri]), "op": "added"})
    return rows


def load_json_doc(path: pathlib.Path) -> Optional[Dict[str, Any]]:
    try:
        with open(path, "r", encoding="utf-8-sig") as f:
            return json.load(f)
    except Exception:
        return None


def audit_file_pair(
    source_plist_path: pathlib.Path,
    compiled_shortcut_path: pathlib.Path,
    source_json_path: Optional[pathlib.Path] = None,
    allow_lowering: bool = False,
) -> Dict[str, Any]:
    """Loads files and evaluates fidelity. A present-but-unparseable JSON is a FAIL, not N/A."""
    if not source_plist_path.exists():
        raise FileNotFoundError(f"Source plist not found: {source_plist_path}")
    if not compiled_shortcut_path.exists():
        raise FileNotFoundError(f"Compiled shortcut not found: {compiled_shortcut_path}")

    with open(source_plist_path, "rb") as f:
        src_doc = plistlib.load(f)
    with open(compiled_shortcut_path, "rb") as f:
        reb_doc = plistlib.load(f)

    json_doc = None
    json_unparseable = False
    if source_json_path and source_json_path.exists():
        json_doc = load_json_doc(source_json_path)
        if json_doc is None:
            json_unparseable = True
            json_doc = {}  # forces a full-tree mismatch instead of NOT_APPLICABLE

    res = evaluate_shortcut_fidelity(src_doc, reb_doc, json_doc, allow_lowering)
    if json_unparseable:
        res["gates"]["RAW_JSON_DECODE_PASS"] = "FAIL"
    res["sourcePlistSha256"] = sha256_file(source_plist_path)
    res["compiledShortcutSha256"] = sha256_file(compiled_shortcut_path)
    return res


def main():
    default_audit_root = pathlib.Path(__file__).resolve().parents[3] / "cherri-shortcut-audit-20261110"
    parser = argparse.ArgumentParser(description="Cherri Independent Fidelity Oracle")
    parser.add_argument("--source-plist", type=pathlib.Path, help="Path to source plist")
    parser.add_argument("--compiled-shortcut", type=pathlib.Path, help="Path to compiled shortcut")
    parser.add_argument("--source-json", type=pathlib.Path, help="Optional path to source JSON")
    parser.add_argument("--audit-corpus", action="store_true", help="Audit the entire 38-shortcut corpus")
    parser.add_argument("--input-dir", type=pathlib.Path, default=default_audit_root / "input")
    parser.add_argument("--work-dir", type=pathlib.Path, default=default_audit_root / "work")
    parser.add_argument("--out-json", type=pathlib.Path, help="Optional output JSON path for results")
    args = parser.parse_args()

    if args.audit_corpus:
        results = []
        pass_seq_count = 0
        pass_semantic_count = 0
        total = 38
        missing = 0
        print(f"[{ORACLE_VERSION}] Auditing full corpus (001-038)...")
        for i in range(1, total + 1):
            sid = f"{i:03d}"
            splist = args.input_dir / f"{sid}.plist"
            sjson = args.input_dir / f"{sid}.json"
            reb = args.work_dir / f"{sid}_legacy.shortcut"
            if not splist.exists() or not reb.exists():
                print(f"Specimen {sid}: INPUT OR REBUILT NOT FOUND -> FAIL")
                missing += 1
                continue
            r = audit_file_pair(splist, reb, sjson)
            r["specimenId"] = sid
            results.append(r)
            seq_status = r["gates"]["STRICT_ACTION_SEQUENCE_PASS"]
            sem_status = r["gates"]["FULL_WORKFLOW_SEMANTIC_PASS"]
            pass_seq_count += seq_status == "PASS"
            pass_semantic_count += sem_status == "PASS"
            mismatch_str = ""
            if r["firstMismatch"]:
                mismatch_str = f"({r['firstMismatch']['category']} at {r['firstMismatch']['path']})"
            print(f"Specimen {sid}: Seq={seq_status}, Sem={sem_status}, Delta={r['actionDelta']:+d} {mismatch_str}")

        print("\n--- AUDIT SUMMARY ---")
        print(f"Total Specimens: {len(results)} (missing: {missing})")
        print(f"Strict Action Sequence PASS: {pass_seq_count} / {total}")
        print(f"Full Workflow Semantic PASS: {pass_semantic_count} / {total}")

        if args.out_json:
            args.out_json.parent.mkdir(parents=True, exist_ok=True)
            with open(args.out_json, "w", encoding="utf-8") as f:
                json.dump(results, f, indent=2)
            print(f"Results written to: {args.out_json}")

        # Missing specimens are failures, never silently skipped.
        sys.exit(1 if (missing or pass_semantic_count < total) else 0)

    if args.source_plist and args.compiled_shortcut:
        res = audit_file_pair(args.source_plist, args.compiled_shortcut, args.source_json)
        print(json.dumps(res, indent=2))
        sys.exit(0 if res["gates"]["FULL_WORKFLOW_SEMANTIC_PASS"] == "PASS" else 1)

    parser.print_help()


if __name__ == "__main__":
    main()
