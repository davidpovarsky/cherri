#!/usr/bin/env python3
"""Conservative AX/native-plist comparison with optional local OCR supplements.

Observed cards come only from AX container geometry and hierarchy. The expected
sequence is decoded independently from the actual compiled plist. Unexposed
identity, hidden parameters and ambiguous page overlap never become a PASS.
"""

import argparse
import hashlib
import json
import math
from collections import Counter, defaultdict
from pathlib import Path
import plistlib


VERSION = "1.5"
STRUCTURAL_KEYS = {"UUID", "OutputUUID", "GroupingIdentifier", "WFControlFlowMode",
                   "CustomOutputName", "OutputName"}
CHROME_ROLES = {"NavigationBar", "Toolbar", "TabBar", "Keyboard", "SearchField"}


def digest(value):
    return hashlib.sha256(value.encode("utf-8")).hexdigest()


def frame(node):
    values = node.get("frame", {})
    result = tuple(values.get(key, 0) for key in ("x", "y", "width", "height"))
    if any(not isinstance(v, (int, float)) or not math.isfinite(v) for v in result):
        return None
    return result


def contains(outer, inner):
    if not outer or not inner:
        return False
    x, y, w, h = outer
    ix, iy, iw, ih = inner
    return w > 0 and h > 0 and iw > 0 and ih > 0 and x <= ix + 1 and ix + iw <= x + w + 1 and y <= iy + 1 and iy + ih <= y + h + 1


def intersects(a, b):
    return bool(a and b and a[2] > 0 and a[3] > 0 and b[2] > 0 and b[3] > 0 and
                a[0] < b[0] + b[2] and b[0] < a[0] + a[2] and
                a[1] < b[1] + b[3] and b[1] < a[1] + a[3])


def readable(value):
    return isinstance(value, str) and bool(value.strip()) and not value.startswith("[REDACTED_")


def extract_cards(nodes, coverage):
    """Return observed containers without consulting native count/order/values."""
    pages = defaultdict(list)
    issues = []
    canvas = frame({"frame": coverage.get("scrollContainerFrame") or {}})
    if not canvas or canvas[2] <= 0 or canvas[3] <= 0:
        return [], ["AX_CANVAS_NOT_IDENTIFIED"]
    for node in nodes:
        pages[node.get("sourceScreenIndex", 0)].append(node)
    by_page = []
    for page, page_nodes in sorted(pages.items()):
        indices = [n.get("nodeIndex") for n in page_nodes]
        if len(set(indices)) != len(indices) or None in indices:
            issues.append(f"DUPLICATE_OR_MISSING_NODE_INDEX_PAGE_{page}")
            continue
        indexed = {n["nodeIndex"]: n for n in page_nodes}
        children = defaultdict(list)
        for node in page_nodes:
            parent = node.get("parentIndex")
            if parent is not None and parent not in indexed:
                issues.append(f"ORPHAN_NODE_PAGE_{page}")
            children[parent].append(node["nodeIndex"])
        for siblings in children.values():
            siblings.sort(key=lambda idx: (indexed[idx].get("siblingOrder", idx), idx))

        def descendants(idx):
            result, seen, pending = [], {idx}, list(children[idx])
            while pending:
                current = pending.pop(0)
                if current in seen:
                    issues.append(f"CYCLIC_AX_TREE_PAGE_{page}")
                    break
                seen.add(current)
                result.append(indexed[current])
                pending.extend(children[current])
            return result

        def chrome_ancestor(node):
            seen = set()
            while node:
                idx = node["nodeIndex"]
                if idx in seen:
                    return True
                seen.add(idx)
                if node.get("elementTypeName") in CHROME_ROLES or node.get("identifier") == "all_shortcuts_library":
                    return True
                node = indexed.get(node.get("parentIndex"))
            return False

        def canvas_ancestor(node):
            seen = set()
            while node and node["nodeIndex"] not in seen:
                seen.add(node["nodeIndex"])
                if node.get("elementTypeName") in {"ScrollView", "CollectionView", "Table"}:
                    bounds = frame(node)
                    if bounds and all(abs(a - b) <= 1 for a, b in zip(bounds, canvas)):
                        return True
                node = indexed.get(node.get("parentIndex"))
            return False

        canvas_ids = {n["nodeIndex"] for n in page_nodes
                      if n.get("elementTypeName") in {"ScrollView", "CollectionView", "Table"}
                      and frame(n) and all(abs(a - b) <= 1 for a, b in zip(frame(n), canvas))}

        def single_other_chain(node):
            seen = set()
            while node and node["nodeIndex"] not in seen:
                seen.add(node["nodeIndex"])
                child_ids = children[node["nodeIndex"]]
                if len(child_ids) != 1 or indexed[child_ids[0]].get("elementTypeName") != "Other":
                    return node
                node = indexed[child_ids[0]]
            return None

        def canvas_group(node):
            # Actual iPadOS AX exposes action regions as unlabelled Other
            # siblings, not Cells. Recognize their header/body hierarchy only;
            # this is grouping evidence, never a native identifier or UUID.
            if node.get("elementTypeName") != "Other" or node.get("parentIndex") not in canvas_ids:
                return False
            branch = single_other_chain(node)
            if not branch:
                return False
            direct = [indexed[i] for i in children[branch["nodeIndex"]]]
            if not any(n.get("elementTypeName") == "Button" for n in direct):
                branch = single_other_chain(direct[0]) if direct and direct[0].get("elementTypeName") == "Other" else None
            if not branch or not contains(frame(node), frame(branch)):
                return False
            bounds, header = frame(node), frame(branch)
            if header[2] < bounds[2] * .45 or not 0 <= header[1] - bounds[1] <= max(2, bounds[2] * .05):
                return False
            direct_buttons = [indexed[i] for i in children[branch["nodeIndex"]]
                              if indexed[i].get("elementTypeName") == "Button"]
            title_nodes = [n for n in descendants(branch["nodeIndex"])
                           if n.get("elementTypeName") in {"StaticText", "TextView", "TextField"}]
            direct_button_ids = {n["nodeIndex"] for n in direct_buttons}
            # Number's title is an editable Button below the title Other chain;
            # it has no StaticText. Require that separate nested title branch,
            # not a title name taken from the expected native plan.
            nested_title_buttons = [n for n in descendants(branch["nodeIndex"])
                                    if n.get("elementTypeName") == "Button"
                                    and n["nodeIndex"] not in direct_button_ids
                                    and contains(header, frame(n))]
            return bool(direct_buttons and (title_nodes or nested_title_buttons or len(direct_buttons) >= 3))

        def nested_parameter_table(node):
            current, seen = indexed.get(node.get("parentIndex")), set()
            while current and current["nodeIndex"] not in seen:
                seen.add(current["nodeIndex"])
                if current["nodeIndex"] in canvas_ids:
                    return False
                if current.get("elementTypeName") == "Table":
                    return True
                current = indexed.get(current.get("parentIndex"))
            return False

        group_ids = {n["nodeIndex"] for n in page_nodes if canvas_group(n)}
        candidates = []
        for node in page_nodes:
            bounds = frame(node)
            role = node.get("elementTypeName")
            if not node.get("visible") or node.get("geometryValid") is False or chrome_ancestor(node) or not canvas_ancestor(node):
                continue
            if role != "Cell" and not node.get("actionContainerHint", False) and node["nodeIndex"] not in group_ids:
                continue
            if (role == "Cell" and nested_parameter_table(node)
                    and not (node.get("actionContainerHint") or node.get("nativeActionIdentifier") or node.get("nativeUUIDDigests"))):
                continue
            if not bounds or not intersects(canvas, bounds) or bounds[2] < canvas[2] * .45:
                continue
            if bounds[0] < canvas[0] - 1 or bounds[0] + bounds[2] > canvas[0] + canvas[2] + 1:
                continue
            nested = descendants(node["nodeIndex"])
            candidates.append((node, nested))
        cards = []
        for node, nested in candidates:
            # A nested parameter cell belongs to its outer action card. A
            # container wrapping multiple candidate cards is not itself a card.
            containing = [p for p, descendants_ in candidates if p is not node and node in descendants_]
            if containing:
                innermost_parent = min(containing, key=lambda p: frame(p)[2] * frame(p)[3])
                if innermost_parent.get("actionContainerHint") or innermost_parent["nodeIndex"] in group_ids:
                    continue
            inner_cards = [n for n, _ in candidates if n is not node and n in nested]
            if inner_cards and not node.get("actionContainerHint") and node["nodeIndex"] not in group_ids:
                continue
            visible = [n for n in [node] + nested if n.get("visible") and intersects(canvas, frame(n))]
            texts = []
            for n in visible:
                for key in ("label", "value"):
                    value = n.get(key)
                    if readable(value) and value not in texts:
                        texts.append(value)
            identities = sorted(set(node.get("nativeUUIDDigests", [])))
            native_ids = [node["nativeActionIdentifier"]] if node.get("nativeActionIdentifier") else []
            cards.append({
                "page": page, "nodeId": f"p{page}:n{node['nodeIndex']}",
                "containerRole": node.get("elementTypeName"), "frame": node.get("frame"),
                "containerEvidence": "AX_CANVAS_GROUP" if node["nodeIndex"] in group_ids else "AX_EXPLICIT_CONTAINER",
                "siblingOrder": node.get("siblingOrder"),
                "texts": texts, "nativeUUIDDigests": identities,
                "nativeIdentifiers": native_ids,
                "stableIdentifierDigest": node.get("identifierDigest"),
                "fullyVisible": contains(canvas, frame(node)),
                "evidence": [{"kind": "AX", "page": page, "nodeId": f"p{page}:n{node['nodeIndex']}"}],
            })
        cards.sort(key=lambda c: (c["frame"]["y"], c["frame"]["x"], c["nodeId"]))
        groups = [card for card in cards if card["containerEvidence"] == "AX_CANVAS_GROUP"]
        orders = [card.get("siblingOrder") for card in groups]
        if any(not isinstance(order, int) for order in orders) or any(a >= b for a, b in zip(orders, orders[1:])):
            issues.append(f"AX_GROUP_ORDER_CONFLICT_PAGE_{page}")
        for index, left in enumerate(groups):
            a = frame(left)
            for right in groups[index + 1:]:
                b = frame(right)
                if intersects(a, b) and min(a[1] + a[3], b[1] + b[3]) - max(a[1], b[1]) > 1:
                    issues.append(f"AX_GROUP_OVERLAP_PAGE_{page}")
        by_page.append(cards)

    observed = []
    for page_cards in by_page:
        for card in page_cards:
            stable = card.get("stableIdentifierDigest")
            same_page_count = sum(c.get("stableIdentifierDigest") == stable for c in page_cards)
            prior = [c for c in observed if stable and c.get("stableIdentifierDigest") == stable]
            if stable and same_page_count == 1 and len(prior) == 1 and prior[0]["page"] != card["page"]:
                previous = prior[0]
                if not card["nativeUUIDDigests"] or previous["nativeUUIDDigests"] != card["nativeUUIDDigests"] or previous["nativeIdentifiers"] != card["nativeIdentifiers"]:
                    issues.append("REUSED_AX_IDENTIFIER_WITH_DIFFERENT_IDENTITY")
                    observed.append(card)
                    continue
                previous["evidence"].extend(card["evidence"])
                previous["fullyVisible"] = previous["fullyVisible"] or card["fullyVisible"]
                previous["texts"] = list(dict.fromkeys(previous["texts"] + card["texts"]))
            else:
                if prior or (stable and same_page_count > 1):
                    issues.append("NONUNIQUE_AX_IDENTIFIER")
                observed.append(card)
    # Do not deduplicate equal labels: two equal adjacent actions are real
    # actions. Without stable identity, cross-page overlap is unresolved.
    if len(by_page) > 1 and any(not c.get("stableIdentifierDigest") for c in observed):
        issues.append("PAGE_OVERLAP_WITHOUT_STABLE_IDENTITY")
    return observed, sorted(set(issues))


def scalar_parameters(params):
    visible, hidden = [], []
    for key, value in params.items():
        if key in STRUCTURAL_KEYS:
            continue
        if isinstance(value, str) and value:
            visible.append({"key": key, "value": value, "nativeType": "string"})
        elif isinstance(value, dict) and value.get("WFSerializationType") == "WFTextTokenString":
            text_value = value.get("Value", {}).get("string")
            if text_value:
                visible.append({"key": key, "value": text_value, "nativeType": "WFTextTokenString"})
            hidden.append({"key": key, "status": "HIDDEN", "reason": "token attachment identity is not proven by visible text"})
        else:
            hidden.append({"key": key, "status": "AX_UNEXPOSED", "reason": "native type/value requires explicit UI evidence"})
    return visible, hidden


def inspect(document, nodes, coverage, fixture, run_id, source_sha, node_error=None):
    native = document.get("WFWorkflowActions", [])
    cards, issues = extract_cards(nodes, coverage)
    expected_uuids = [a.get("WFWorkflowActionParameters", {}).get("UUID") for a in native]
    expected_uuids = [str(value).lower() for value in expected_uuids if value]
    if len(set(expected_uuids)) != len(expected_uuids):
        issues.append("DUPLICATE_EXPECTED_NATIVE_UUID")
    if node_error:
        issues.append(node_error)
    if not nodes:
        issues.append("AX_TREE_MISSING_OR_EMPTY")
    if not coverage.get("editorOpened") or not coverage.get("editorIdentityVerified") or coverage.get("expectedShortcutName") != fixture:
        issues.append("EDITOR_IDENTITY_NOT_VERIFIED")
    if coverage.get("runId") != run_id:
        issues.append("AX_RUN_ID_UNVERIFIED")
    bottom = coverage.get("bottomReachedEvidence", "")
    if not bottom or bottom == "MAX_PAGES_REACHED":
        issues.append("EDITOR_BOTTOM_UNVERIFIED")
    if len(native) != len(cards):
        issues.append("ACTION_COUNT_MISMATCH")

    rows = []
    for index, action in enumerate(native):
        params = action.get("WFWorkflowActionParameters", {})
        uuid = params.get("UUID")
        values, hidden = scalar_parameters(params)
        card = cards[index] if index < len(cards) else None
        row = {"sourceIndex": index, "nativeIdentifier": action.get("WFWorkflowActionIdentifier"),
               "nativeUUIDDigest": digest(uuid.lower()) if isinstance(uuid, str) else None,
               "groupingIdentifierDigest": digest(str(params["GroupingIdentifier"]).lower()) if params.get("GroupingIdentifier") else None,
               "controlFlowMode": params.get("WFControlFlowMode"), "visibleParameters": [],
               "hiddenOrUnobserved": hidden, "evidence": [], "status": "NOT_FOUND"}
        if card:
            row["evidence"] = card["evidence"]
            row["observedTexts"] = card["texts"]
            uuid_seen = row["nativeUUIDDigest"] and row["nativeUUIDDigest"] in card["nativeUUIDDigests"]
            id_seen = row["nativeIdentifier"] in card["nativeIdentifiers"]
            identity_conflict = bool((card["nativeUUIDDigests"] and not uuid_seen) or (card["nativeIdentifiers"] and not id_seen))
            for parameter in values:
                seen = parameter["value"] in card["texts"]
                bound = bool((uuid_seen or id_seen) and not identity_conflict)
                parameter_status = "MATCH" if seen and bound else "AX_UNEXPOSED" if seen else "HIDDEN"
                row["visibleParameters"].append({**parameter, "status": parameter_status,
                    "observedTextPresent": seen, "evidence": card["evidence"] if seen else []})
            all_values_seen = all(p["status"] == "MATCH" for p in row["visibleParameters"])
            if identity_conflict:
                row["status"] = "MISMATCH"
            elif not (uuid_seen or id_seen):
                row["status"] = "AX_UNEXPOSED"
            elif not card["fullyVisible"]:
                row["status"] = "HIDDEN"
            elif not all_values_seen or hidden:
                row["status"] = "NEEDS_REVIEW"
            else:
                row["status"] = "MATCH"
            row["nativeIdentityEvidence"] = "UUID" if uuid_seen else "NATIVE_IDENTIFIER" if id_seen else "AX_UNEXPOSED"
        rows.append(row)
    counts = Counter(row["status"] for row in rows)
    issues = sorted(set(issues))
    complete = bool(native) and not issues and counts["MATCH"] == len(native)
    common = {"schemaVersion": VERSION, "fixtureId": fixture, "runId": run_id, "compilerSha": source_sha,
              "expectedNativeActionCount": len(native), "observedCardCount": len(cards),
              "observedCardCountScope": "PAGE_OBSERVATIONS_WITH_UNRESOLVED_IDENTITY" if any(
                  issue in issues for issue in ("PAGE_OVERLAP_WITHOUT_STABLE_IDENTITY",
                    "REUSED_AX_IDENTIFIER_WITH_DIFFERENT_IDENTITY", "NONUNIQUE_AX_IDENTIFIER")) else "AX_CONTAINERS",
              "status": "PASS" if complete else "NEEDS_REVIEW", "issues": issues,
              "proofTier": "AX_NATIVE_COMPARISON", "ocrUsed": False}
    comparison = {**common, "actions": rows, "summary": {"matched": counts["MATCH"],
        "unmatchedExpected": len(native) - counts["MATCH"], "statusCounts": dict(counts),
        "ambiguousObserved": len(cards) if issues else 0,
        "falsePassCount": None, "falsePassNote": "Measured only by separate negative-control test execution"}}
    observed = {**common, "cards": cards,
                "legacyHeuristicLabelCount": len(coverage.get("detectedActionCards", [])),
                "legacyHeuristicIsCardCount": False}
    return observed, comparison


def main():
    parser = argparse.ArgumentParser()
    for name in ("plist", "tree", "coverage", "fixture", "run-id", "source-sha", "out-dir"):
        parser.add_argument("--" + name, required=True)
    parser.add_argument("--ocr-json", action="append", default=[], help="Private local OCR observation file; repeat per page")
    parser.add_argument("--screenshots-manifest", help="Capture-time image hashes and viewport sizes; required with OCR")
    args = parser.parse_args()
    document = plistlib.loads(Path(args.plist).read_bytes())
    coverage = json.loads(Path(args.coverage).read_text(encoding="utf-8")) if Path(args.coverage).exists() else {}
    node_error = None
    try:
        nodes = json.loads(Path(args.tree).read_text(encoding="utf-8"))
        if not isinstance(nodes, list):
            raise ValueError("tree must be a list")
    except (OSError, ValueError):
        nodes, node_error = [], "AX_TREE_UNREADABLE"
    observed, comparison = inspect(document, nodes, coverage, args.fixture, args.run_id, args.source_sha, node_error)
    if args.ocr_json:
        from editor_ocr import OCRError, supplement
        try:
            if not args.screenshots_manifest:
                raise OCRError("SCREENSHOT_MANIFEST_REQUIRED")
            ocr_documents = [json.loads(Path(p).read_text(encoding="utf-8")) for p in args.ocr_json]
            screenshot_manifest = json.loads(Path(args.screenshots_manifest).read_text(encoding="utf-8"))
            observed, comparison = supplement(observed, comparison, ocr_documents, screenshot_manifest)
        except (OCRError, OSError, ValueError, TypeError, KeyError):
            for result in (observed, comparison):
                result["status"] = "NEEDS_REVIEW"
                result["issues"] = sorted(set(result["issues"] + ["OCR_SUPPLEMENT_REJECTED"]))
    out = Path(args.out_dir)
    out.mkdir(parents=True, exist_ok=True)
    for filename, data in (("editor-actions.json", observed), ("editor-comparison.json", comparison)):
        (out / filename).write_text(json.dumps(data, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    lines = ["# Editor Inspector", "", f"Status: `{comparison['status']}`", "",
             f"Expected native actions: {comparison['expectedNativeActionCount']}; observed AX containers: {comparison['observedCardCount']}.",
             "", "This compares independent compiled plist data with AX observations. Hidden values and missing identity remain unverified.",
             "Historical detectedActionCards counts distinct UI strings, including toolbar controls and parameters; it is not a native action count.", "",
             "| Native index | Identifier | Status |", "|---|---|---|"]
    lines.extend(f"| {r['sourceIndex']} | {r['nativeIdentifier']} | {r['status']} |" for r in comparison["actions"])
    if comparison.get("ocrUsed"):
        lines.extend(["", "## Optional local OCR", "",
                      f"Displayed-text observations: {comparison['ocrSupplement']['observationCount']}. No AX status was promoted.",
                      "OCR cannot prove native identity, hidden parameters, object types or exact plist equivalence. Raw OCR stays local."])
    lines.extend(["", "## Limitations", ""] + ["- " + i for i in comparison["issues"]])
    (out / "editor-report.md").write_text("\n".join(lines) + "\n", encoding="utf-8")
    print(json.dumps({"status": comparison["status"], "expected": len(comparison["actions"]), "observed": len(observed["cards"]), "issues": comparison["issues"]}))
    return 0 if comparison["status"] == "PASS" else 2


if __name__ == "__main__":
    raise SystemExit(main())
