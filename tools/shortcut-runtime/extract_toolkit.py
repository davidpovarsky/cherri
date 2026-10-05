#!/usr/bin/env python3
"""tools/shortcut-runtime/extract_toolkit.py

First-party Cherri extractor for Apple Shortcuts ToolKit SQLite registry.
Safely opens ~/Library/Shortcuts/ToolKit/Tools-prod.*.sqlite in read-only mode.
Produces:
  - toolkit-registry.json: Canonical normalized records for every tool in the registry
  - toolkit-apple-actions.json: Runnable Apple App Intent / Link actions
  - toolkit-names.json: Human-readable English names, descriptions, and enum cases
  - toolkit-summary.json: Full structural statistics and classification breakdown

Classifies tools into distinct semantic categories without flattening:
  - apple_link_runnable (visible Apple App Intent / Link actions)
  - apple_configuration_only (widget/control/focus configurations)
  - apple_link_hidden (visibilityFlags == 0 Apple tools)
  - apple_synthesized (synthesized system tools)
  - builtin_visible / builtin_hidden (WorkflowKit bundled actions)
  - apple_intent / third_party_intent (SiriKit / IntentActionProvider)
  - third_party_link (Third-party App Intents)
  - interchange_action (Interchange actions)
  - unknown_provider
"""

import argparse
import datetime
import glob
import json
import os
import pathlib
import re
import sqlite3
import sys

DEFAULT_DB_PATTERN = "~/Library/Shortcuts/ToolKit/Tools-prod.*.sqlite"

# LNValueType primitive protobuf tags
PRIMITIVES = {
    0x08: "any",
    0x10: "bool",
    0x18: "int",
    0x20: "double",
    0x30: "string",
    0x38: "date",
    0x40: "dateComponents",
    0x48: "url",
    0x58: "richText",
    0x80: "person",
    0x88: "file",
    0x90: "app",
    0xB0: "recurrence",
    0xC0: "measurement",
    0xC8: "location",
}

KIND_OF = {
    "string": "text",
    "richText": "text",
    "url": "text",
    "bool": "bool",
    "int": "number",
    "double": "number",
}

TYPE_KIND = {
    1: "primitive",
    2: "entity",
    3: "enum",
    4: "enum",
    6: "type",
}

CONFIGURATION_PROTOCOLS = {
    "widgetConfiguration",
    "controlConfiguration",
    "liveActivity",
    "focusConfiguration",
}


def sanitize_path_str(s):
    if not isinstance(s, str):
        return s
    return re.sub(r"/Users/[^/\s'\"]+", "~", s)


def primitive_name(blob):
    if not blob or not isinstance(blob, (bytes, bytearray)):
        return "unknown"
    if len(blob) >= 3 and blob[0] == 0x0A:
        return PRIMITIVES.get(blob[2], f"primitive_0x{blob[2]:02x}")
    return "unknown"


def read_bundle_and_name(blob):
    if not blob or not isinstance(blob, (bytes, bytearray)):
        return None, None
    try:
        i = 2 if len(blob) > 0 and blob[0] == 0x12 else 0
        if len(blob) > i + 1 and blob[i] == 0x0A:
            n = blob[i + 1]
            if len(blob) >= i + 2 + n:
                bundle = blob[i + 2:i + 2 + n].decode("utf-8", errors="replace")
                j = i + 2 + n
                if len(blob) > j + 1 and blob[j] == 0x12:
                    m = blob[j + 1]
                    if len(blob) >= j + 2 + m:
                        name = blob[j + 2:j + 2 + m].decode("utf-8", errors="replace")
                        return bundle, name
    except Exception:
        pass
    return None, None


def safe_parse_json(val):
    if not val:
        return {}
    if isinstance(val, dict):
        return val
    try:
        res = json.loads(val)
        return res if isinstance(res, dict) else {}
    except Exception:
        return {}


def get_table_columns(con, table_name):
    try:
        return {col[1] for col in con.execute(f'PRAGMA table_info("{table_name}")')}
    except Exception:
        return set()


def extract_type_info(con, type_row_id, tables, types_cols):
    if "Types" not in tables:
        return {"kind": "unknown"}

    row = con.execute('SELECT id, kind FROM "Types" WHERE rowId=?', (type_row_id,)).fetchone()
    if not row:
        return {"kind": "unknown"}

    blob, kind = row
    k = TYPE_KIND.get(kind, f"kind_{kind}")
    if k == "primitive":
        return {"kind": "primitive", "primitive": primitive_name(blob)}

    bundle, name = read_bundle_and_name(blob)
    info = {"kind": k, "bundleIdentifier": bundle, "name": name}

    if k == "enum" and "EnumerationCases" in tables:
        cases_cols = get_table_columns(con, "EnumerationCases")
        if "typeId" in cases_cols and "id" in cases_cols and "title" in cases_cols:
            query = 'SELECT id, title FROM "EnumerationCases" WHERE typeId=? ORDER BY id'
            if "locale" in cases_cols:
                query = 'SELECT id, title FROM "EnumerationCases" WHERE typeId=? AND locale=\'en\' ORDER BY id'
            info["cases"] = [{"id": cid, "title": ctitle} for cid, ctitle in con.execute(query, (type_row_id,))]

    return info


def classify_tool(tool_record, is_apple):
    provider = tool_record.get("provider") or ""
    vis = tool_record.get("visibilityFlags", 0)
    hidden = vis == 0
    synthesized = tool_record.get("synthesized", False)
    configuration = tool_record.get("configuration", False)

    if provider == "WFBundledActionProvider":
        return "builtin_hidden" if hidden else "builtin_visible"
    elif provider == "WFLinkActionProvider":
        if not is_apple:
            return "third_party_link"
        if synthesized:
            return "apple_synthesized"
        if configuration:
            return "apple_configuration_only"
        if hidden:
            return "apple_link_hidden"
        return "apple_link_runnable"
    elif provider == "WFIntentActionProvider":
        return "apple_intent" if is_apple else "third_party_intent"
    elif provider == "WFInterchangeActionProvider":
        return "interchange_action"
    return "unknown_provider"


def extract_toolkit_registry(db_path):
    """Opens ToolKit SQLite database read-only and extracts complete normalized registry."""
    if not db_path or not os.path.exists(db_path):
        raise FileNotFoundError(f"ToolKit database not found: {db_path}")

    # Connect strictly in read-only immutable mode
    resolved_path = pathlib.Path(db_path).resolve().as_posix()
    con = sqlite3.connect(f"file:{resolved_path}?mode=ro&immutable=1", uri=True)

    try:
        tables = {x[0] for x in con.execute("SELECT name FROM sqlite_master WHERE type='table'")}

        # 1. Metadata
        meta = {}
        if "Metadata" in tables:
            try:
                for k, v in con.execute('SELECT key, value FROM "Metadata"'):
                    if isinstance(v, bytes):
                        try:
                            v = v.decode("utf-8")
                        except Exception:
                            v = v.hex()
                    meta[str(k)] = v
            except Exception:
                pass

        version_info = safe_parse_json(meta.get("VersionKey"))
        ls_info = safe_parse_json(meta.get("LaunchServicesDatabaseVersionKey"))

        provenance = {
            "schemaVersion": "2.0.0",
            "dbFilename": os.path.basename(db_path),
            "osVersion": meta.get("OSVersion"),
            "indexerSource": meta.get("IndexerSource"),
            "toolkitVersion": version_info.get("uuid"),
            "launchServicesSequence": ls_info.get("sequenceNumber"),
            "extractedAt": datetime.datetime.now(datetime.timezone.utc).isoformat(),
        }

        # 2. Containers (ContainerMetadata + ContainerMetadataLocalizations)
        containers = {}
        if "ContainerMetadata" in tables:
            cm_cols = get_table_columns(con, "ContainerMetadata")
            sel_cols = ["rowId", "id"]
            if "teamId" in cm_cols:
                sel_cols.append("teamId")
            else:
                sel_cols.append("NULL")
            if "origin" in cm_cols:
                sel_cols.append("origin")
            else:
                sel_cols.append("NULL")
            if "containerType" in cm_cols:
                sel_cols.append("containerType")
            else:
                sel_cols.append("NULL")

            query = f'SELECT {", ".join(sel_cols)} FROM "ContainerMetadata"'
            for row in con.execute(query):
                row_id, cid, team, origin, ctype = row
                cname = None
                if "ContainerMetadataLocalizations" in tables:
                    cml_cols = get_table_columns(con, "ContainerMetadataLocalizations")
                    if "containerId" in cml_cols and "name" in cml_cols:
                        loc_q = 'SELECT name FROM "ContainerMetadataLocalizations" WHERE containerId=?'
                        if "locale" in cml_cols:
                            loc_q += ' AND locale=\'en\''
                        n_res = con.execute(loc_q, (row_id,)).fetchone()
                        if n_res:
                            cname = n_res[0]

                bundle_id = str(cid or "")
                team_id = str(team) if team else None
                is_apple = bundle_id.startswith("com.apple.") and team_id in (None, "0000000000", "")

                containers[row_id] = {
                    "bundleIdentifier": bundle_id,
                    "teamId": team_id,
                    "name": cname,
                    "containerType": ctype,
                    "origin": origin,
                    "isApple": is_apple,
                }

        # 3. Type display representations
        type_display = {}
        types_cols = get_table_columns(con, "Types")
        if "Types" in tables and "TypeDisplayRepresentations" in tables:
            tdr_cols = get_table_columns(con, "TypeDisplayRepresentations")
            for row_id, blob in con.execute('SELECT rowId, id FROM "Types"'):
                bundle, tname = read_bundle_and_name(blob)
                if bundle and tname:
                    q = 'SELECT name FROM "TypeDisplayRepresentations" WHERE typeId=?'
                    if "locale" in tdr_cols:
                        q += ' AND locale=\'en\''
                    q += ' AND name IS NOT NULL AND name != \'\' LIMIT 1'
                    name_res = con.execute(q, (row_id,)).fetchone()
                    if name_res and name_res[0]:
                        type_display[f"{bundle}.{tname}"] = name_res[0]

        # 4. Extract Tools
        tools = []
        names_dict = {}
        classification_counts = {}
        provider_counts = {}

        if "Tools" in tables:
            tools_cols = get_table_columns(con, "Tools")
            t_sel = ["rowId", "id"]
            t_sel.append("toolType" if "toolType" in tools_cols else "0")
            t_sel.append("flags" if "flags" in tools_cols else "0")
            t_sel.append("visibilityFlags" if "visibilityFlags" in tools_cols else "0")
            t_sel.append("sourceActionProvider" if "sourceActionProvider" in tools_cols else "'unknown'")
            t_sel.append("pythonName" if "pythonName" in tools_cols else "NULL")
            t_sel.append("sourceContainerId" if "sourceContainerId" in tools_cols else "NULL")
            t_sel.append("attributionContainerId" if "attributionContainerId" in tools_cols else "NULL")

            t_query = f'SELECT {", ".join(t_sel)} FROM "Tools" ORDER BY id'

            for row in con.execute(t_query):
                row_id, ident, tool_type, flags, vis, provider, python_name, container_id, attr_id = row

                # Tool Localizations
                loc_name, loc_out, loc_desc, loc_res, loc_note = None, None, None, None, None
                if "ToolLocalizations" in tables:
                    tl_cols = get_table_columns(con, "ToolLocalizations")
                    loc_q = 'SELECT name, outputResultName, descriptionSummary, descriptionResult, descriptionNote FROM "ToolLocalizations" WHERE toolId=?'
                    if "locale" in tl_cols:
                        loc_q += ' AND locale=\'en\''
                    loc_res_row = con.execute(loc_q, (row_id,)).fetchone()
                    if loc_res_row:
                        loc_name, loc_out, loc_desc, loc_res, loc_note = loc_res_row

                # Parameters
                params = []
                if "Parameters" in tables:
                    p_cols = get_table_columns(con, "Parameters")
                    if "key" in p_cols and "toolId" in p_cols:
                        p_sel = ["key"]
                        p_sel.append("sortOrder" if "sortOrder" in p_cols else "rowId")
                        p_sel.append("flags" if "flags" in p_cols else "0")
                        order_clause = "ORDER BY sortOrder" if "sortOrder" in p_cols else "ORDER BY rowId"
                        p_query = f'SELECT {", ".join(p_sel)} FROM "Parameters" WHERE toolId=? {order_clause}'

                        for p_key, p_order, p_flags in con.execute(p_query, (row_id,)):
                            ploc_name, ploc_desc, ploc_true, ploc_false = None, None, None, None
                            if "ParameterLocalizations" in tables:
                                pl_cols = get_table_columns(con, "ParameterLocalizations")
                                pl_q = 'SELECT name, description, trueString, falseString FROM "ParameterLocalizations" WHERE toolId=? AND key=?'
                                if "locale" in pl_cols:
                                    pl_q += ' AND locale=\'en\''
                                pl_res = con.execute(pl_q, (row_id, p_key)).fetchone()
                                if pl_res:
                                    ploc_name, ploc_desc, ploc_true, ploc_false = pl_res

                            # Parameter Types
                            param_types = []
                            if "ToolParameterTypes" in tables:
                                tpt_cols = get_table_columns(con, "ToolParameterTypes")
                                if "toolId" in tpt_cols and "key" in tpt_cols and "typeId" in tpt_cols:
                                    for (tid,) in con.execute('SELECT typeId FROM "ToolParameterTypes" WHERE toolId=? AND key=?', (row_id, p_key)):
                                        param_types.append(extract_type_info(con, tid, tables, types_cols))

                            primary_type = param_types[0] if param_types else {"kind": "unknown"}
                            if ploc_true is not None and primary_type.get("kind") == "primitive" and primary_type.get("primitive") != "bool":
                                primary_type = {"kind": "primitive", "primitive": "bool"}

                            p_entry = {
                                "key": p_key,
                                "name": ploc_name,
                                "description": ploc_desc,
                                "kind": KIND_OF.get(primary_type.get("primitive"), "string" if primary_type.get("kind") == "enum" else "any"),
                                "type": primary_type,
                                "flags": p_flags,
                                "order": p_order,
                            }
                            if ploc_true is not None or ploc_false is not None:
                                p_entry["booleanLabels"] = {"true": ploc_true, "false": ploc_false}
                            if len(param_types) > 1:
                                p_entry["alternativeTypes"] = param_types[1:]

                            params.append(p_entry)

                # Output types
                outputs = []
                if "ToolOutputTypes" in tables:
                    tot_cols = get_table_columns(con, "ToolOutputTypes")
                    if "toolId" in tot_cols and "typeIdentifier" in tot_cols:
                        outputs = [t for (t,) in con.execute('SELECT typeIdentifier FROM "ToolOutputTypes" WHERE toolId=?', (row_id,))]

                output_name = loc_out or next((type_display[t] for t in outputs if t in type_display), None)

                # Keywords
                keywords = []
                if "SearchKeywords" in tables:
                    sk_cols = get_table_columns(con, "SearchKeywords")
                    if "toolId" in sk_cols and "keyword" in sk_cols:
                        sk_q = 'SELECT keyword FROM "SearchKeywords" WHERE toolId=?'
                        if "locale" in sk_cols:
                            sk_q += ' AND locale=\'en\''
                        if "order" in sk_cols:
                            sk_q += ' ORDER BY "order"'
                        keywords = [k for (k,) in con.execute(sk_q, (row_id,))]

                # Link Action Identifiers
                link_ids = []
                if "LinkActionIdentifiers" in tables:
                    lai_cols = get_table_columns(con, "LinkActionIdentifiers")
                    if "toolId" in lai_cols and "identifier" in lai_cols:
                        link_ids = [k for (k,) in con.execute('SELECT identifier FROM "LinkActionIdentifiers" WHERE toolId=?', (row_id,))]

                # System Protocols
                protocols = []
                if "SystemToolProtocols" in tables:
                    stp_cols = get_table_columns(con, "SystemToolProtocols")
                    if "toolId" in stp_cols and "identifier" in stp_cols:
                        protocols = sorted({p for (p,) in con.execute('SELECT identifier FROM "SystemToolProtocols" WHERE toolId=?', (row_id,))})

                container = containers.get(container_id, {})
                attr_container = containers.get(attr_id) if attr_id and attr_id != container_id else None
                is_apple = container.get("isApple", False)
                configuration_only = bool(set(protocols) & CONFIGURATION_PROTOCOLS)
                synthesized = "synthesizedTool" in protocols

                tool_rec = {
                    "identifier": ident,
                    "key": python_name,
                    "toolType": tool_type,
                    "provider": provider,
                    "visibilityFlags": vis,
                    "flags": flags,
                    "hidden": vis == 0,
                    "configuration": configuration_only,
                    "synthesized": synthesized,
                    "protocols": protocols,
                    "app": container,
                    "attribution": attr_container,
                    "name": loc_name,
                    "outputName": output_name,
                    "outputNameSource": "resultLabel" if loc_out else ("outputType" if output_name else None),
                    "description": loc_desc,
                    "descriptionResult": loc_res,
                    "descriptionNote": loc_note,
                    "outputTypes": outputs,
                    "outputTypeNames": [type_display.get(t) for t in outputs if t in type_display],
                    "keywords": keywords,
                    "appIntentIdentifier": link_ids[0] if link_ids else None,
                    "parameters": params,
                }

                category = classify_tool(tool_rec, is_apple)
                tool_rec["classification"] = category
                classification_counts[category] = classification_counts.get(category, 0) + 1
                provider_counts[provider] = provider_counts.get(provider, 0) + 1

                tools.append(tool_rec)

                if loc_name:
                    enums = {p["key"]: [c["id"] for c in p["type"].get("cases", [])] for p in params if p["type"].get("kind") == "enum" and p["type"].get("cases")}
                    labels = {p["key"]: p["name"] for p in params if p.get("name")}
                    names_dict[ident] = {
                        "name": loc_name,
                        "description": loc_desc,
                        "outputName": loc_out,
                        "enumCases": enums,
                        "labels": labels,
                        "classification": category,
                    }

        # 5. Counts and Summary
        table_counts = {}
        for t in ["Tools", "Parameters", "ToolParameterTypes", "ToolOutputTypes", "EnumerationCases"]:
            if t in tables:
                table_counts[t] = con.execute(f'SELECT count(*) FROM "{t}"').fetchone()[0]
            else:
                table_counts[t] = 0

        vis_count = sum(1 for t in tools if not t["hidden"])
        apple_containers_count = sum(1 for c in containers.values() if c.get("isApple"))

        summary = {
            "schemaVersion": "2.0.0",
            "toolkitDb": os.path.basename(db_path),
            "totalTools": len(tools),
            "uniqueIdentifiers": len({t["identifier"] for t in tools}),
            "duplicateIdentifiers": sorted([t_id for t_id in {t["identifier"] for t in tools} if sum(1 for x in tools if x["identifier"] == t_id) > 1]),
            "visible": vis_count,
            "hidden": len(tools) - vis_count,
            "containers": len(containers),
            "appleContainers": apple_containers_count,
            "thirdPartyContainers": len(containers) - apple_containers_count,
            "providers": provider_counts,
            "classifications": classification_counts,
            "tableCounts": table_counts,
            "provenance": provenance,
            "failures": [],
        }

        # Filter Apple Runnable actions
        apple_runnable = {t["identifier"]: t for t in tools if t["classification"] == "apple_link_runnable"}

        registry_payload = {
            "provenance": provenance,
            "tools": tools,
        }

        apple_payload = {
            "provenance": provenance,
            "actions": apple_runnable,
        }

        return registry_payload, apple_payload, names_dict, summary

    finally:
        con.close()


def inspect_sqlite_database(db_path, dump_exit_code=0):
    """Backwards-compatible summary inspection of the ToolKit database."""
    if not db_path or not os.path.exists(db_path):
        return {
            "toolkitDb": os.path.basename(db_path) if db_path else None,
            "totalTools": 0,
            "failures": ["ToolKit DB not found"]
        }
    try:
        _, _, _, summary = extract_toolkit_registry(db_path)
        if dump_exit_code != 0:
            summary["failures"].append(f"dump-toolkit-registry exit {dump_exit_code}")
        for k, v in summary.get("tableCounts", {}).items():
            summary[k] = v
        return summary
    except Exception as e:
        return {
            "toolkitDb": os.path.basename(db_path) if db_path else None,
            "totalTools": 0,
            "failures": [str(e)]
        }


def find_default_toolkit_db():
    pattern = os.path.expanduser(DEFAULT_DB_PATTERN)
    files = sorted(glob.glob(pattern))
    return files[-1] if files else None


def main():
    parser = argparse.ArgumentParser(description="First-party Cherri ToolKit registry extractor")
    parser.add_argument("--data-dir", default="out/data", help="Output data directory")
    parser.add_argument("--logs-dir", default="out/logs", help="Logs directory")
    parser.add_argument("--db-path", help="Explicit path to Tools-prod.*.sqlite")
    parser.add_argument("--shortcutkit-dir", default="out/shortcutkit", help="ShortcutKit repo directory (for reference only)")
    parser.add_argument("--skip-dump", action="store_true", help="Deprecated flag maintained for backwards compatibility")
    args = parser.parse_args()

    data_dir = pathlib.Path(args.data_dir)
    data_dir.mkdir(parents=True, exist_ok=True)

    db_file = args.db_path or find_default_toolkit_db()
    if not db_file:
        print(f"Error: No ToolKit DB found matching {DEFAULT_DB_PATTERN}", file=sys.stderr)
        summary = {
            "schemaVersion": "2.0.0",
            "toolkitDb": None,
            "totalTools": 0,
            "failures": ["ToolKit database not found on runner"],
        }
        (data_dir / "toolkit-summary.json").write_text(json.dumps(summary, indent=2, sort_keys=True), encoding="utf-8")
        sys.exit(1)

    print(f"Extracting ToolKit registry from {os.path.basename(db_file)}...")
    registry, apple_actions, names, summary = extract_toolkit_registry(db_file)

    # Write all artifacts deterministically
    (data_dir / "toolkit-registry.json").write_text(json.dumps(registry, indent=2, sort_keys=True), encoding="utf-8")
    (data_dir / "toolkit-apple-actions.json").write_text(json.dumps(apple_actions, indent=2, sort_keys=True), encoding="utf-8")
    (data_dir / "toolkit-names.json").write_text(json.dumps(names, indent=2, sort_keys=True), encoding="utf-8")
    (data_dir / "toolkit-summary.json").write_text(json.dumps(summary, indent=2, sort_keys=True), encoding="utf-8")

    print(f"Extraction complete:")
    print(f"  Total tools: {summary['totalTools']}")
    print(f"  Apple Link runnable: {summary['classifications'].get('apple_link_runnable', 0)}")
    print(f"  Providers: {', '.join(f'{k}: {v}' for k, v in sorted(summary['providers'].items()))}")
    print(f"  Classifications: {', '.join(f'{k}: {v}' for k, v in sorted(summary['classifications'].items()))}")


if __name__ == "__main__":
    main()
