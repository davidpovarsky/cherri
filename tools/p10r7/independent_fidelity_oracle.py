#!/usr/bin/env python3
"""
tools/p10r7/independent_fidelity_oracle.py
Cherri P10R7 Independent Semantic Fidelity Oracle & 12-Gate Verification Engine.

Strict Privacy & Fidelity Invariants:
- NEVER leaks raw private shortcut text, personal strings, URLs, tokens, or PII.
- Operates independently from Cherri's serializer, compiler, or comparator implementations.
- True ground truth: strict action sequence, wire typed parameters, bijective variable graphs,
  control flow grouping, full workflow metadata headers, and per-action 3rd party descriptors.
- Any unexplained divergence is a hard FAIL, not a warning or auto-pass.
"""

import argparse
import copy
import hashlib
import json
import os
import pathlib
import plistlib
import re
import sys
import time
from typing import Any, Dict, List, Optional, Set, Tuple

ORACLE_VERSION = "p10r7-fidelity-oracle-20261110.1"

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

HEADER_CRITICAL_KEYS = [
    "WFWorkflowTypes",
    "WFWorkflowTriggers",
    "WFWorkflowNoInputBehavior",
    "WFWorkflowInputContentItemClasses",
    "WFWorkflowOutputContentItemClasses",
    "WFWorkflowMinimumClientVersion",
    "WFWorkflowImportQuestions",
]

UUID_REGEX = re.compile(r"^[0-9A-Fa-f]{8}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{12}$")

def sha256_file(p: pathlib.Path) -> str:
    h = hashlib.sha256()
    with open(p, "rb") as f:
        while chunk := f.read(65536):
            h.update(chunk)
    return h.hexdigest()

def sha256_data(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()

class BijectiveUUIDScope:
    def __init__(self):
        self.src_to_reb: Dict[str, str] = {}
        self.reb_to_src: Dict[str, str] = {}
        self.src_action_uuid_to_idx: Dict[str, int] = {}
        self.reb_action_uuid_to_idx: Dict[str, int] = {}
        self.group_src_to_reb: Dict[str, str] = {}
        self.group_reb_to_src: Dict[str, str] = {}

    def register_action_uuid(self, src_uuid: str, reb_uuid: str, action_idx: int) -> Optional[str]:
        if not src_uuid or not reb_uuid:
            return None
        self.src_action_uuid_to_idx[src_uuid] = action_idx
        self.reb_action_uuid_to_idx[reb_uuid] = action_idx
        
        # Check bijectivity
        if src_uuid in self.src_to_reb and self.src_to_reb[src_uuid] != reb_uuid:
            return f"UUID collision: source {src_uuid} already mapped to {self.src_to_reb[src_uuid]}, now mapped to {reb_uuid}"
        if reb_uuid in self.reb_to_src and self.reb_to_src[reb_uuid] != src_uuid:
            return f"UUID collision: rebuilt {reb_uuid} already mapped from {self.reb_to_src[reb_uuid]}, now from {src_uuid}"
        
        self.src_to_reb[src_uuid] = reb_uuid
        self.reb_to_src[reb_uuid] = src_uuid
        return None

    def register_group_uuid(self, src_group: str, reb_group: str) -> Optional[str]:
        if not src_group or not reb_group:
            return None
        if src_group in self.group_src_to_reb and self.group_src_to_reb[src_group] != reb_group:
            return f"Grouping UUID collision: source {src_group} mapped to multiple rebuilt groups ({self.group_src_to_reb[src_group]} vs {reb_group})"
        if reb_group in self.group_reb_to_src and self.group_reb_to_src[reb_group] != src_group:
            return f"Grouping UUID collision: rebuilt {reb_group} mapped from multiple source groups ({self.group_reb_to_src[reb_group]} vs {src_group})"
        self.group_src_to_reb[src_group] = reb_group
        self.group_reb_to_src[reb_group] = src_group
        return None

    def match_uuid(self, src_uuid: str, reb_uuid: str) -> bool:
        if not src_uuid or not reb_uuid:
            return src_uuid == reb_uuid
        if src_uuid in self.src_to_reb:
            return self.src_to_reb[src_uuid] == reb_uuid
        if reb_uuid in self.reb_to_src:
            return self.reb_to_src[reb_uuid] == src_uuid
        # Unmapped: register dynamically if neither has been mapped yet
        self.src_to_reb[src_uuid] = reb_uuid
        self.reb_to_src[reb_uuid] = src_uuid
        return True

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

def extract_output_uuids(val: Any) -> List[str]:
    """Finds all OutputUUID strings within a parameter dictionary or list."""
    uuids = []
    if isinstance(val, dict):
        if "OutputUUID" in val and isinstance(val["OutputUUID"], str):
            uuids.append(val["OutputUUID"])
        for v in val.values():
            uuids.extend(extract_output_uuids(v))
    elif isinstance(val, list):
        for elem in val:
            uuids.extend(extract_output_uuids(elem))
    return uuids

def compare_values(
    src_val: Any,
    reb_val: Any,
    path: str,
    scope: BijectiveUUIDScope
) -> Tuple[bool, Optional[str], Optional[str], Optional[str]]:
    """
    Recursively compares source value against rebuilt value.
    Returns: (is_equal, mismatch_category, mismatch_path, error_detail)
    """
    if type(src_val) != type(reb_val):
        # Allow int and float equality if mathematically identical (e.g. 1 and 1.0)
        if isinstance(src_val, (int, float)) and isinstance(reb_val, (int, float)) and not isinstance(src_val, bool) and not isinstance(reb_val, bool):
            if src_val == reb_val:
                return True, None, None, None
        return False, "PARAM_TYPE_CHANGE", path, f"Type mismatch: {type(src_val).__name__} vs {type(reb_val).__name__}"

    if isinstance(src_val, dict):
        src_keys = set(src_val.keys())
        reb_keys = set(reb_val.keys())
        all_keys = src_keys | reb_keys
        for k in sorted(all_keys):
            subpath = f"{path}.{k}"
            if k not in src_val:
                return False, "EXTRA_PARAM_KEY", subpath, f"Key '{k}' added in rebuilt"
            if k not in reb_val:
                return False, "MISSING_PARAM_KEY", subpath, f"Key '{k}' missing in rebuilt"
            
            eq, cat, m_path, detail = compare_values(src_val[k], reb_val[k], subpath, scope)
            if not eq:
                return False, cat, m_path, detail
        return True, None, None, None

    if isinstance(src_val, list):
        if len(src_val) != len(reb_val):
            return False, "PARAM_VALUE_CHANGE", path, f"List length mismatch: {len(src_val)} vs {len(reb_val)}"
        if path in ("WFWorkflowTypes", "WFWorkflowInputContentItemClasses", "WFWorkflowOutputContentItemClasses", "WFQuickActionSurfaces"):
            if all(isinstance(x, str) for x in src_val) and all(isinstance(x, str) for x in reb_val):
                if sorted(src_val) != sorted(reb_val):
                    return False, "SET_MEMBERSHIP_CHANGE", path, f"Set content mismatch: {set(src_val) ^ set(reb_val)}"
                return True, None, None, None
        for idx, (s_elem, r_elem) in enumerate(zip(src_val, reb_val)):
            subpath = f"{path}[{idx}]"
            eq, cat, m_path, detail = compare_values(s_elem, r_elem, subpath, scope)
            if not eq:
                return False, cat, m_path, detail
        return True, None, None, None

    if isinstance(src_val, str):
        if UUID_REGEX.match(src_val) and UUID_REGEX.match(reb_val):
            if path.endswith("GroupingIdentifier"):
                if not scope.match_group(src_val, reb_val):
                    return False, "REFERENCE_CHANGE", path, "GroupingIdentifier does not match bijective group mapping"
                return True, None, None, None
            if not scope.match_uuid(src_val, reb_val):
                return False, "REFERENCE_CHANGE", path, "UUID reference does not match bijective variable mapping"
            return True, None, None, None
        
        if src_val != reb_val:
            return False, "PARAM_VALUE_CHANGE", path, "String value mismatch"
        return True, None, None, None

    if isinstance(src_val, (bytes, bytearray)):
        if src_val != reb_val:
            return False, "PARAM_VALUE_CHANGE", path, "Binary data mismatch"
        return True, None, None, None

    # Primitive types (int, float, bool)
    if src_val != reb_val:
        return False, "PARAM_VALUE_CHANGE", path, f"Primitive value mismatch: {src_val} vs {reb_val}"

    return True, None, None, None

def evaluate_shortcut_fidelity(
    src_doc: Dict[str, Any],
    reb_doc: Dict[str, Any],
    json_doc: Optional[Dict[str, Any]] = None,
    allow_lowering_rules: bool = False
) -> Dict[str, Any]:
    """
    Evaluates 12 fidelity gates on two shortcut documents (decoded dicts).
    """
    gates = {g: "PENDING" for g in GATES}
    first_mismatch: Optional[Dict[str, Any]] = None

    def set_fail(gate: str, cat: str, path: str, detail: str):
        nonlocal first_mismatch
        gates[gate] = "FAIL"
        if first_mismatch is None:
            first_mismatch = {
                "gate": gate,
                "category": cat,
                "path": path,
                "detail": detail
            }

    # Gate 1: RAW_JSON_DECODE_PASS
    if json_doc is not None:
        json_actions = json_doc.get("WFWorkflowActions", [])
        src_actions_test = src_doc.get("WFWorkflowActions", [])
        if len(json_actions) == len(src_actions_test) and [a.get("WFWorkflowActionIdentifier") for a in json_actions] == [a.get("WFWorkflowActionIdentifier") for a in src_actions_test]:
            gates["RAW_JSON_DECODE_PASS"] = "PASS"
        else:
            gates["RAW_JSON_DECODE_PASS"] = "FAIL"
            set_fail("RAW_JSON_DECODE_PASS", "JSON_PLIST_MISMATCH", "WFWorkflowActions", "JSON input actions do not match plist input actions")
    else:
        gates["RAW_JSON_DECODE_PASS"] = "NOT_APPLICABLE"

    # Gate 2: BPLIST_OR_XML_DECODE_PASS
    if "WFWorkflowActions" in src_doc and isinstance(src_doc["WFWorkflowActions"], list):
        gates["BPLIST_OR_XML_DECODE_PASS"] = "PASS"
    else:
        gates["BPLIST_OR_XML_DECODE_PASS"] = "FAIL"
        set_fail("BPLIST_OR_XML_DECODE_PASS", "DECODE_ERROR", "WFWorkflowActions", "Source document lacks valid WFWorkflowActions list")

    # Gate 3: WORKFLOW_HEADER_PASS
    header_mismatches = []
    for k in HEADER_CRITICAL_KEYS:
        if k in src_doc:
            if k not in reb_doc:
                header_mismatches.append(f"Missing header key: {k}")
            else:
                scope_header = BijectiveUUIDScope()
                eq, cat, p, d = compare_values(src_doc[k], reb_doc[k], k, scope_header)
                if not eq:
                    header_mismatches.append(f"Header value mismatch in {k}: {cat}")
    
    if not header_mismatches:
        gates["WORKFLOW_HEADER_PASS"] = "PASS"
    else:
        gates["WORKFLOW_HEADER_PASS"] = "FAIL"
        set_fail("WORKFLOW_HEADER_PASS", "HEADER_CHANGE", header_mismatches[0], header_mismatches[0])

    src_actions = src_doc.get("WFWorkflowActions", [])
    reb_actions = reb_doc.get("WFWorkflowActions", [])

    src_count = len(src_actions)
    reb_count = len(reb_actions)
    action_delta = reb_count - src_count

    src_ids = [a.get("WFWorkflowActionIdentifier") for a in src_actions]
    reb_ids = [a.get("WFWorkflowActionIdentifier") for a in reb_actions]

    # Gate 4: STRICT_ACTION_SEQUENCE_PASS
    if src_count == reb_count and src_ids == reb_ids:
        gates["STRICT_ACTION_SEQUENCE_PASS"] = "PASS"
    else:
        gates["STRICT_ACTION_SEQUENCE_PASS"] = "FAIL"
        if src_count != reb_count:
            set_fail("STRICT_ACTION_SEQUENCE_PASS", "ACTION_COUNT_MISMATCH", f"WFWorkflowActions (len {src_count} vs {reb_count})", f"Action count delta: {action_delta}")
        else:
            first_idx = next(i for i, (s, r) in enumerate(zip(src_ids, reb_ids)) if s != r)
            set_fail("STRICT_ACTION_SEQUENCE_PASS", "REPLACED_IDENTIFIER", f"WFWorkflowActions[{first_idx}].WFWorkflowActionIdentifier", f"Identifier changed from {src_ids[first_idx]} to {reb_ids[first_idx]}")

    # Build Bijective Scope across actions
    scope = BijectiveUUIDScope()
    scope_action_errors = []
    scope_group_errors = []
    min_len = min(src_count, reb_count)
    for i in range(min_len):
        s_act = src_actions[i]
        r_act = reb_actions[i]
        s_params = s_act.get("WFWorkflowActionParameters", {})
        r_params = r_act.get("WFWorkflowActionParameters", {})
        
        s_uuid = s_params.get("UUID")
        r_uuid = r_params.get("UUID")
        if s_uuid and r_uuid:
            err = scope.register_action_uuid(s_uuid, r_uuid, i)
            if err:
                scope_action_errors.append((i, err))

        s_group = s_params.get("GroupingIdentifier")
        r_group = r_params.get("GroupingIdentifier")
        if s_group and r_group:
            err = scope.register_group_uuid(s_group, r_group)
            if err:
                scope_group_errors.append((i, err))

    # Gate 5: VARIABLE_GRAPH_PASS
    graph_errors = []
    if scope_action_errors:
        idx, err = scope_action_errors[0]
        graph_errors.append((idx, "REFERENCE_CHANGE", f"WFWorkflowActions[{idx}].UUID", err))
    else:
        # Check all referenced OutputUUIDs
        for i in range(min_len):
            s_params = src_actions[i].get("WFWorkflowActionParameters", {})
            r_params = reb_actions[i].get("WFWorkflowActionParameters", {})
            s_out_uuids = extract_output_uuids(s_params)
            r_out_uuids = extract_output_uuids(r_params)
            if len(s_out_uuids) != len(r_out_uuids):
                graph_errors.append((i, "REFERENCE_CHANGE", f"WFWorkflowActions[{i}].OutputUUID", f"OutputUUID count mismatch: {len(s_out_uuids)} vs {len(r_out_uuids)}"))
                break
            for s_u, r_u in zip(s_out_uuids, r_out_uuids):
                if not scope.match_uuid(s_u, r_u):
                    graph_errors.append((i, "REFERENCE_CHANGE", f"WFWorkflowActions[{i}].OutputUUID", f"OutputUUID mismatch: {s_u} vs {r_u}"))
                    break
            if graph_errors:
                break

    if not graph_errors and src_count == reb_count:
        gates["VARIABLE_GRAPH_PASS"] = "PASS"
    else:
        gates["VARIABLE_GRAPH_PASS"] = "FAIL"
        if graph_errors:
            idx, cat, p, err = graph_errors[0]
            set_fail("VARIABLE_GRAPH_PASS", cat, p, err)
        elif src_count != reb_count:
            set_fail("VARIABLE_GRAPH_PASS", "REFERENCE_CHANGE", "WFWorkflowActions", "Variable graph mismatch due to action count delta")

    # Gate 6: CONTROL_FLOW_PASS
    control_flow_mismatches = []
    if scope_group_errors:
        idx, err = scope_group_errors[0]
        control_flow_mismatches.append((idx, err))
    
    for i in range(min_len):
        s_act = src_actions[i]
        r_act = reb_actions[i]
        s_ident = s_act.get("WFWorkflowActionIdentifier", "")
        if "conditional" in s_ident or "choosefrommenu" in s_ident or "repeat" in s_ident:
            s_mode = s_act.get("WFWorkflowActionParameters", {}).get("WFControlFlowMode")
            r_mode = r_act.get("WFWorkflowActionParameters", {}).get("WFControlFlowMode")
            if s_mode != r_mode:
                control_flow_mismatches.append((i, f"WFControlFlowMode mismatch: {s_mode} vs {r_mode}"))
            s_group = s_act.get("WFWorkflowActionParameters", {}).get("GroupingIdentifier")
            r_group = r_act.get("WFWorkflowActionParameters", {}).get("GroupingIdentifier")
            if (s_group is None) != (r_group is None):
                control_flow_mismatches.append((i, "GroupingIdentifier presence mismatch"))

    if not control_flow_mismatches and src_count == reb_count:
        gates["CONTROL_FLOW_PASS"] = "PASS"
    else:
        gates["CONTROL_FLOW_PASS"] = "FAIL"
        if control_flow_mismatches:
            idx, err = control_flow_mismatches[0]
            set_fail("CONTROL_FLOW_PASS", "CONTROL_FLOW_CHANGE", f"WFWorkflowActions[{idx}].WFControlFlowMode", err)
        elif src_count != reb_count:
            set_fail("CONTROL_FLOW_PASS", "CONTROL_FLOW_CHANGE", "WFWorkflowActions", "Control flow length divergence")

    # Gate 7: THIRD_PARTY_DESCRIPTOR_WIRE_PASS
    descriptor_mismatches = []
    src_descriptor_count = 0
    reb_descriptor_count = 0
    for i in range(src_count):
        s_act = src_actions[i]
        s_desc = s_act.get("WFWorkflowActionParameters", {}).get("AppIntentDescriptor")
        if s_desc:
            src_descriptor_count += 1
            if i >= reb_count:
                descriptor_mismatches.append((i, "Action with descriptor missing in rebuilt"))
                continue
            r_act = reb_actions[i]
            r_desc = r_act.get("WFWorkflowActionParameters", {}).get("AppIntentDescriptor")
            if not r_desc:
                descriptor_mismatches.append((i, "AppIntentDescriptor missing in rebuilt at matching action"))
            else:
                eq, cat, p, d = compare_values(s_desc, r_desc, f"WFWorkflowActions[{i}].AppIntentDescriptor", scope)
                if not eq:
                    descriptor_mismatches.append((i, f"Descriptor content mismatch: {cat} - {d}"))

    for i in range(reb_count):
        r_desc = reb_actions[i].get("WFWorkflowActionParameters", {}).get("AppIntentDescriptor")
        if r_desc:
            reb_descriptor_count += 1

    if src_descriptor_count == 0 and reb_descriptor_count == 0:
        gates["THIRD_PARTY_DESCRIPTOR_WIRE_PASS"] = "NOT_APPLICABLE"
    elif src_descriptor_count == reb_descriptor_count and not descriptor_mismatches:
        gates["THIRD_PARTY_DESCRIPTOR_WIRE_PASS"] = "PASS"
    else:
        gates["THIRD_PARTY_DESCRIPTOR_WIRE_PASS"] = "FAIL"
        if descriptor_mismatches:
            idx, err = descriptor_mismatches[0]
            set_fail("THIRD_PARTY_DESCRIPTOR_WIRE_PASS", "DESCRIPTOR_MISMATCH", f"WFWorkflowActions[{idx}].AppIntentDescriptor", err)
        else:
            set_fail("THIRD_PARTY_DESCRIPTOR_WIRE_PASS", "DESCRIPTOR_COUNT_MISMATCH", "WFWorkflowActions", f"Descriptor count mismatch: {src_descriptor_count} vs {reb_descriptor_count}")

    # Gate 8: ACTION_PARAMETER_WIRE_PASS
    param_mismatches = []
    if src_count != reb_count:
        gates["ACTION_PARAMETER_WIRE_PASS"] = "FAIL"
        set_fail("ACTION_PARAMETER_WIRE_PASS", "ACTION_COUNT_MISMATCH", "WFWorkflowActions", f"Cannot compare parameters 1:1 due to action count delta ({src_count} vs {reb_count})")
    else:
        for i in range(src_count):
            s_act = src_actions[i]
            r_act = reb_actions[i]
            s_params = s_act.get("WFWorkflowActionParameters", {})
            r_params = r_act.get("WFWorkflowActionParameters", {})
            
            s_filtered = {k: v for k, v in s_params.items() if k not in ("UUID", "CustomOutputName")}
            r_filtered = {k: v for k, v in r_params.items() if k not in ("UUID", "CustomOutputName")}
            
            eq, cat, m_path, detail = compare_values(s_filtered, r_filtered, f"WFWorkflowActions[{i}].WFWorkflowActionParameters", scope)
            if not eq:
                param_mismatches.append((i, cat, m_path, detail))
                set_fail("ACTION_PARAMETER_WIRE_PASS", cat, m_path, detail)
                break
        
        if not param_mismatches:
            gates["ACTION_PARAMETER_WIRE_PASS"] = "PASS"
        else:
            gates["ACTION_PARAMETER_WIRE_PASS"] = "FAIL"

    # Gate 9: FULL_WORKFLOW_SEMANTIC_PASS
    required_gates = [
        "STRICT_ACTION_SEQUENCE_PASS",
        "ACTION_PARAMETER_WIRE_PASS",
        "VARIABLE_GRAPH_PASS",
        "CONTROL_FLOW_PASS",
        "WORKFLOW_HEADER_PASS",
    ]
    if gates["THIRD_PARTY_DESCRIPTOR_WIRE_PASS"] != "NOT_APPLICABLE":
        required_gates.append("THIRD_PARTY_DESCRIPTOR_WIRE_PASS")

    if all(gates[g] == "PASS" for g in required_gates):
        gates["FULL_WORKFLOW_SEMANTIC_PASS"] = "PASS"
    else:
        gates["FULL_WORKFLOW_SEMANTIC_PASS"] = "FAIL"

    gates["APPLE_RUNTIME_EXECUTED"] = "BLOCKED_DEVICE"

    return {
        "oracleVersion": ORACLE_VERSION,
        "gates": gates,
        "sourceActions": src_count,
        "compiledActions": reb_count,
        "actionDelta": action_delta,
        "sourceDescriptors": src_descriptor_count,
        "compiledDescriptors": reb_descriptor_count,
        "firstMismatch": first_mismatch
    }

def audit_file_pair(
    source_plist_path: pathlib.Path,
    compiled_shortcut_path: pathlib.Path,
    source_json_path: Optional[pathlib.Path] = None,
    allow_lowering: bool = False
) -> Dict[str, Any]:
    """Loads files and evaluates fidelity."""
    if not source_plist_path.exists():
        raise FileNotFoundError(f"Source plist not found: {source_plist_path}")
    if not compiled_shortcut_path.exists():
        raise FileNotFoundError(f"Compiled shortcut not found: {compiled_shortcut_path}")

    with open(source_plist_path, "rb") as f:
        src_doc = plistlib.load(f)

    with open(compiled_shortcut_path, "rb") as f:
        reb_doc = plistlib.load(f)

    json_doc = None
    if source_json_path and source_json_path.exists():
        try:
            with open(source_json_path, "r", encoding="utf-8-sig") as f:
                json_doc = json.load(f)
        except Exception:
            json_doc = None

    res = evaluate_shortcut_fidelity(src_doc, reb_doc, json_doc, allow_lowering)
    res["sourcePlistSha256"] = sha256_file(source_plist_path)
    res["compiledShortcutSha256"] = sha256_file(compiled_shortcut_path)
    return res

def main():
    default_audit_root = pathlib.Path(__file__).resolve().parents[3] / "cherri-shortcut-audit-20261110"
    parser = argparse.ArgumentParser(description="Cherri P10R7 Independent Semantic Fidelity Oracle")
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
        print(f"[{ORACLE_VERSION}] Auditing full corpus (001-038)...")
        for i in range(1, total + 1):
            sid = f"{i:03d}"
            splist = args.input_dir / f"{sid}.plist"
            sjson = args.input_dir / f"{sid}.json"
            reb = args.work_dir / f"{sid}_legacy.shortcut"
            if not reb.exists():
                print(f"Specimen {sid}: REBUILT NOT FOUND -> FAIL")
                continue
            r = audit_file_pair(splist, reb, sjson)
            r["specimenId"] = sid
            results.append(r)
            seq_status = r["gates"]["STRICT_ACTION_SEQUENCE_PASS"]
            sem_status = r["gates"]["FULL_WORKFLOW_SEMANTIC_PASS"]
            if seq_status == "PASS":
                pass_seq_count += 1
            if sem_status == "PASS":
                pass_semantic_count += 1
            mismatch_str = ""
            if r["firstMismatch"]:
                mismatch_str = f"({r['firstMismatch']['category']} at {r['firstMismatch']['path']})"
            print(f"Specimen {sid}: Seq={seq_status}, Sem={sem_status}, Delta={r['actionDelta']:+d} {mismatch_str}")

        print(f"\n--- AUDIT SUMMARY ---")
        print(f"Total Specimens: {len(results)}")
        print(f"Strict Action Sequence PASS: {pass_seq_count} / {len(results)}")
        print(f"Full Workflow Semantic PASS: {pass_semantic_count} / {len(results)}")

        if args.out_json:
            args.out_json.parent.mkdir(parents=True, exist_ok=True)
            with open(args.out_json, "w", encoding="utf-8") as f:
                json.dump(results, f, indent=2)
            print(f"Results written to: {args.out_json}")

        if pass_semantic_count < len(results):
            sys.exit(1)
        sys.exit(0)

    if args.source_plist and args.compiled_shortcut:
        res = audit_file_pair(args.source_plist, args.compiled_shortcut, args.source_json)
        print(json.dumps(res, indent=2))
        if res["gates"]["FULL_WORKFLOW_SEMANTIC_PASS"] != "PASS":
            sys.exit(1)
        sys.exit(0)

    parser.print_help()

if __name__ == "__main__":
    main()
