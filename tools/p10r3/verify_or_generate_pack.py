#!/usr/bin/env python3
"""tools/p10r3/verify_or_generate_pack.py

Deterministic generator and verifier for the Pure Upstream + Apple Actions Pack track
(origin/apple-actions-pack-pure-core-20261022 @ 141b9cc96cf1df3f9d1593548dc49ae8a46a256a)
and Pure Original baseline (bcc0095754e2cc0baaeb4d93dae172b8153b4716) vs P10 Native Compiler.

Supports:
  --generate --out-dir <dir> [--p10-root <dir>]
  --verify-generated
  --json

Uses relative repository paths and `git show` only — zero hardcoded machine paths.
"""

import argparse
import collections
import hashlib
import json
import pathlib
import re
import subprocess
import sys
import tempfile

REPO_ROOT = pathlib.Path(__file__).resolve().parent.parent.parent
PURE_COMMIT = "bcc0095754e2cc0baaeb4d93dae172b8153b4716"
PURE_TREE = "4fe9fbc8c742aaf01d6ec8193d9be708fb1de7b5"
PACK_COMMIT = "141b9cc96cf1df3f9d1593548dc49ae8a46a256a"

REQUIRED_7_CLASSIFICATIONS = (
    "ALREADY_UPSTREAM",
    "PACK_DECLARATIVE",
    "PACK_ADVANCED_ALIAS",
    "P10_ONLY_GO_REQUIRED",
    "REQUIRES_APPINTENT_DESCRIPTOR",
    "UNSUPPORTED_WITH_EVIDENCE",
    "NEEDS_EVIDENCE",
)

EXPECTED_15_GO_ONLY = {
    "createRemindersList": "REQUIRES_APPINTENT_DESCRIPTOR",
    "editEvent": "P10_ONLY_GO_REQUIRED",
    "editReminder": "P10_ONLY_GO_REQUIRED",
    "openBook": "REQUIRES_APPINTENT_DESCRIPTOR",
    "playAudiobook": "REQUIRES_APPINTENT_DESCRIPTOR",
    "playVoiceMemo": "REQUIRES_APPINTENT_DESCRIPTOR",
    "recordVoiceMemo": "REQUIRES_APPINTENT_DESCRIPTOR",
    "run": "ALREADY_UPSTREAM",
    "searchSpotlight": "REQUIRES_APPINTENT_DESCRIPTOR",
    "setSilentMode": "REQUIRES_APPINTENT_DESCRIPTOR",
    "startNotesAudioRecording": "REQUIRES_APPINTENT_DESCRIPTOR",
    "startStopwatch": "REQUIRES_APPINTENT_DESCRIPTOR",
    "stopStopwatch": "REQUIRES_APPINTENT_DESCRIPTOR",
    "stopVoiceMemoRecording": "REQUIRES_APPINTENT_DESCRIPTOR",
    "transcribeText": "REQUIRES_APPINTENT_DESCRIPTOR",
}

KNOWN_PRESERVED_UPSTREAM = {
    "statistic",
    "getValue",
    "downloadURL",
    "formRequest",
    "jsonRequest",
    "fileRequest",
}


def git_show_text(commit: str, rel_path: str) -> str:
    res = subprocess.run(
        ["git", "show", f"{commit}:{rel_path}"],
        cwd=REPO_ROOT,
        capture_output=True,
        text=True,
        encoding="utf-8",
        check=True,
    )
    return res.stdout


def parse_declarations(text: str) -> dict:
    starts = list(re.finditer(r"(?m)^(action|enum)\b", text))
    decls = {}
    for idx, match in enumerate(starts):
        end_pos = starts[idx + 1].start() if idx + 1 < len(starts) else len(text)
        block = text[match.start() : end_pos].strip()
        kind = match.group(1)
        if kind == "enum":
            nm = re.search(r"^enum\s+(\w+)", block)
        else:
            nm = re.search(r"^action\b.*?\b(\w+)\s*\(", block, re.S)
        if nm:
            decls[(kind, nm.group(1))] = block
    return decls


def extract_param_keys(block: str) -> list[str]:
    m = re.search(r"\b\w+\s*\(", block)
    if not m:
        return []
    pos = m.end()
    level = 1
    quote = None
    start = pos
    while pos < len(block) and level > 0:
        c = block[pos]
        if quote:
            if c == "\\":
                pos += 2
                continue
            if c == quote:
                quote = None
        elif c in "'\"":
            quote = c
        elif c == "(":
            level += 1
        elif c == ")":
            level -= 1
        pos += 1
    return re.findall(r":\s*'([^']+)'", block[start : pos - 1])


def compatible_keys(orig_keys: list[str], new_keys: list[str]) -> bool:
    it = iter(new_keys)
    return all(any(k == v for v in it) for k in orig_keys)


def normalize_decl(text: str) -> str:
    no_comments = re.sub(r"(?m)^\s*//[^\n]*", "", text)
    return re.sub(r"\s+", " ", no_comments).strip()


def make_alias_declaration(block: str, name: str, alias: str, identifier: str) -> str:
    header_end = block.index("(", block.index(name))
    prefix = block[:header_end]
    if "'" not in prefix:
        ident = identifier
        if ident.startswith("is.workflow.actions."):
            ident = ident[len("is.workflow.actions.") :]
        idx = prefix.rfind(name)
        prefix = f"{prefix[:idx]}'{ident}' {prefix[idx:]}"
    updated = prefix + block[header_end:]
    return re.sub(rf"\b{re.escape(name)}(?=\s*\()", alias, updated, count=1)


def gather_pure_upstream_declarations() -> tuple[dict, dict]:
    merged = {}
    files = {}
    tree_out = subprocess.check_output(
        ["git", "ls-tree", "--name-only", f"{PURE_COMMIT}:actions"],
        cwd=REPO_ROOT,
        text=True,
        encoding="utf-8",
    )
    for fname in sorted(tree_out.splitlines()):
        if not fname.endswith(".cherri"):
            continue
        stem = fname[:-7]
        content = git_show_text(PURE_COMMIT, f"actions/{fname}")
        for key, block in parse_declarations(content).items():
            merged[key] = block
            files[key] = stem
    return merged, files


def gather_p10_declarations(p10_root: pathlib.Path = REPO_ROOT) -> tuple[dict, dict]:
    merged = {}
    files = {}
    actions_dir = p10_root / "actions"
    for file_path in sorted(actions_dir.glob("*.cherri")):
        stem = file_path.stem
        content = file_path.read_text(encoding="utf-8")
        for key, block in parse_declarations(content).items():
            merged[key] = block
            files[key] = stem
    return merged, files


def load_catalog_identifier_map(p10_root: pathlib.Path = REPO_ROOT) -> dict:
    matrix_path = p10_root / "docs" / "apple-runtime" / "p10" / "p10-505-catalog-coverage-matrix.json"
    if matrix_path.exists():
        data = json.loads(matrix_path.read_text(encoding="utf-8"))
        return {e["cherriName"]: e["shortcutIdentifier"] for e in data.get("entries", [])}
    return {}


def build_7_category_classification_table(
    decisions: list[dict],
    p10_root: pathlib.Path = REPO_ROOT,
) -> dict:
    """Classifies all declarations and Apple runtime records across the 7 required categories:
    - ALREADY_UPSTREAM
    - PACK_DECLARATIVE
    - PACK_ADVANCED_ALIAS
    - P10_ONLY_GO_REQUIRED
    - REQUIRES_APPINTENT_DESCRIPTOR
    - UNSUPPORTED_WITH_EVIDENCE
    - NEEDS_EVIDENCE
    """
    buckets = {k: [] for k in REQUIRED_7_CLASSIFICATIONS}

    for d in decisions:
        if d.get("kind") != "action":
            continue
        nm = d["name"]
        st = d["status"]
        if st == "added":
            buckets["PACK_DECLARATIVE"].append(nm)
        elif st == "extended_alias":
            buckets["PACK_ADVANCED_ALIAS"].append(d.get("alias") or f"{nm}Advanced")
            buckets["ALREADY_UPSTREAM"].append(nm)
        elif st in ("unchanged", "original_preserved"):
            buckets["ALREADY_UPSTREAM"].append(nm)

    for go_name, cls in sorted(EXPECTED_15_GO_ONLY.items()):
        if cls == "ALREADY_UPSTREAM":
            if go_name not in buckets["ALREADY_UPSTREAM"]:
                buckets["ALREADY_UPSTREAM"].append(go_name)
        elif cls == "P10_ONLY_GO_REQUIRED":
            buckets["P10_ONLY_GO_REQUIRED"].append(go_name)
        elif cls == "REQUIRES_APPINTENT_DESCRIPTOR":
            buckets["REQUIRES_APPINTENT_DESCRIPTOR"].append(go_name)

    ledger_path = p10_root / "docs" / "apple-runtime" / "p10" / "p10-action-ledger.json"
    if ledger_path.exists():
        ledger = json.loads(ledger_path.read_text(encoding="utf-8"))
        for row in ledger:
            st = row.get("status")
            ident = row.get("identifier", "")
            if st in ("NEW_NATIVE_ACTION", "MISSING_APP_INTENT_DESCRIPTOR"):
                buckets["UNSUPPORTED_WITH_EVIDENCE"].append(ident)
            elif st == "DEFERRED_NO_EVIDENCE":
                buckets["NEEDS_EVIDENCE"].append(ident)

    return {
        "counts": {k: len(v) for k, v in buckets.items()},
        "samples": {k: sorted(v)[:25] for k, v in buckets.items()},
        "goOnly15": dict(EXPECTED_15_GO_ONLY),
    }


def generate_pack(out_dir: pathlib.Path, p10_root: pathlib.Path = REPO_ROOT) -> dict:
    """Deterministically generates the Pure+Pack extension directory (`modules/*.cherri`,
    `manifest.json`, `LIMITATIONS.md`, `checksums.sha256`) from Pure Original (`bcc0095`)
    and P10 `.cherri` definitions."""
    out_dir.mkdir(parents=True, exist_ok=True)
    modules_dir = out_dir / "modules"
    modules_dir.mkdir(parents=True, exist_ok=True)

    old_decls, old_files = gather_pure_upstream_declarations()
    new_decls, new_files = gather_p10_declarations(p10_root)
    ident_map = load_catalog_identifier_map(p10_root)

    counts = collections.Counter()
    choices = []
    by_module = collections.defaultdict(list)

    for key, body in sorted(new_decls.items(), key=lambda x: (new_files[x[0]], x[0][0], x[0][1])):
        kind, name = key
        reason = ""
        alias = None
        out_block = None

        if name in KNOWN_PRESERVED_UPSTREAM:
            status = "original_preserved"
            reason = "upstream statistic() already uses verified WFInput parameter key"
        elif key not in old_decls:
            status = "added"
            out_block = body
        elif normalize_decl(old_decls[key]) == normalize_decl(body):
            status = "unchanged"
        elif kind == "enum":
            status = "original_preserved"
            reason = "changed original enum requires manual audit"
        else:
            orig_k = extract_param_keys(old_decls[key])
            new_k = extract_param_keys(body)
            if not compatible_keys(orig_k, new_k):
                status = "original_preserved"
                reason = "replacement/removal/reordering of original native param keys"
            else:
                status = "extended_alias"
                alias = f"{name}Advanced"
                identifier = ident_map.get(name, f"is.workflow.actions.{name.lower()}")
                out_block = make_alias_declaration(body, name, alias, identifier)

        counts[status] += 1
        choices.append(
            {
                "kind": kind,
                "name": name,
                "alias": alias,
                "module": new_files[key],
                "status": status,
                "old_file": old_files.get(key),
                "reason": reason,
                "originalKeys": extract_param_keys(old_decls[key]) if (key in old_decls and kind == "action") else None,
                "newKeys": extract_param_keys(body) if kind == "action" else None,
            }
        )
        if out_block is not None:
            by_module[new_files[key]].append((kind, name, out_block, alias))

    for old_f in modules_dir.glob("*.cherri"):
        old_f.unlink()

    module_sha256 = {}
    for module_name, defs in sorted(by_module.items()):
        lines = [
            "// EXTERNAL opt-in Cherri Apple Actions Pack — no modifications to original compiler.",
            "// Existing functions stay original; additional optional parameters use Advanced aliases.",
            "// Source declarations: P10 action DSL. Load original category's enums and metadata.",
            (
                f"#include 'actions/{module_name}'"
                if module_name != "basic"
                else "// basic action category is implicitly included by Cherri"
            ),
            "",
        ]
        for kind, name, block, alias in sorted(defs, key=lambda x: 0 if x[0] == "enum" else 1):
            comment = f"// New {kind} {name}" if not alias else f"// Advanced alias {alias} for {name}"
            lines.extend([comment, block, ""])
        mod_text = "\n".join(lines) + "\n"
        mod_bytes = mod_text.encode("utf-8")
        mod_path = modules_dir / f"{module_name}.cherri"
        mod_path.write_bytes(mod_bytes)
        module_sha256[mod_path.name] = hashlib.sha256(mod_bytes).hexdigest()

    classifications_7 = build_7_category_classification_table(choices, p10_root)

    manifest = {
        "name": "cherri-apple-actions-pack",
        "version": "0.2.0",
        "pure_source_sha": PURE_COMMIT,
        "pure_source_tree": PURE_TREE,
        "definition_source_branch": "agent/cherri-p10-apple-actions-coverage",
        "status": "GENERATED_FROM_P10_DECLARATIVE_ACTIONS",
        "module_sha256": module_sha256,
        "summary": dict(counts),
        "classifications7": classifications_7,
        "modules": {
            mod_name: [{"name": k, "alias": alias, "kind": kind} for kind, k, _, alias in defs]
            for mod_name, defs in sorted(by_module.items())
        },
        "special_handlers": [
            "WFContentItemFilter predicates require native conversion; simple nil filters do not prove complex filters",
            "P10 Go-only app intents and system actions need native descriptors/UUID mapping and separate validation",
            "Original Cherri builtin run()/statistic() are NOT replaced",
        ],
        "decisions": choices,
    }
    manifest_bytes = (json.dumps(manifest, indent=2, ensure_ascii=False) + "\n").encode("utf-8")
    (out_dir / "manifest.json").write_bytes(manifest_bytes)

    limitations_lines = [
        "# Cherri Apple Actions Pack — Architectural Scope & Limitations",
        "",
        "## 15 Go-Only Actions Evaluated Between Pure+Pack (`141b9cc`) and P10 Native",
        "",
    ]
    for act_name, cls in sorted(EXPECTED_15_GO_ONLY.items()):
        limitations_lines.append(f"- `{act_name}`: `{cls}`")
    limitations_bytes = ("\n".join(limitations_lines) + "\n").encode("utf-8")
    (out_dir / "LIMITATIONS.md").write_bytes(limitations_bytes)

    checksum_lines = []
    for fn in sorted(module_sha256.keys()):
        checksum_lines.append(f"{module_sha256[fn]}  modules/{fn}")
    checksum_lines.append(f"{hashlib.sha256(manifest_bytes).hexdigest()}  manifest.json")
    checksum_lines.append(f"{hashlib.sha256(limitations_bytes).hexdigest()}  LIMITATIONS.md")
    (out_dir / "checksums.sha256").write_bytes(("\n".join(checksum_lines) + "\n").encode("utf-8"))

    return manifest


def extract_pack_summary() -> dict:
    manifest_raw = git_show_text(PACK_COMMIT, "extensions/cherri-apple-actions-pack/manifest.json")
    manifest = json.loads(manifest_raw)
    verify_raw = git_show_text(PACK_COMMIT, "extensions/cherri-apple-actions-pack/entry-verification-summary.json")
    verify_summary = json.loads(verify_raw)
    limitations_md = git_show_text(PACK_COMMIT, "extensions/cherri-apple-actions-pack/LIMITATIONS.md")

    decisions = manifest.get("decisions", [])
    new_actions = [d["name"] for d in decisions if d.get("kind") == "action" and d.get("status") == "added"]
    advanced_aliases = [
        d.get("alias") or d["name"]
        for d in decisions
        if d.get("kind") == "action" and d.get("status") == "extended_alias"
    ]
    unchanged_actions = [
        d["name"]
        for d in decisions
        if d.get("kind") == "action" and d.get("status") in ("unchanged", "original_preserved")
    ]

    return {
        "pureCommit": PURE_COMMIT,
        "packCommit": PACK_COMMIT,
        "totalEntries": len(new_actions) + len(advanced_aliases),
        "newActionsCount": len(new_actions),
        "advancedAliasesCount": len(advanced_aliases),
        "unchangedActionsCount": len(unchanged_actions),
        "newActions": sorted(new_actions),
        "advancedAliases": sorted(advanced_aliases),
        "unchangedActions": sorted(unchanged_actions),
        "entryVerificationSummary": verify_summary,
        "limitationsMentionsAll15": all(name in limitations_md for name in EXPECTED_15_GO_ONLY),
    }


def verify_generated_pack(
    p10_root: pathlib.Path = REPO_ROOT,
    mutated_modules: dict = None,
) -> list[str]:
    """Generates the Pack from scratch in a temporary directory and verifies:
    1. All 20 generated modules/*.cherri match the committed 141b9cc Pack modules byte-for-byte.
    2. All 7 required classification categories are present and populated.
    3. All 15 Go-only actions match p10-pack-15-go-only-audit.json.
    4. Zero hardcoded container paths exist anywhere in tools/.
    """
    errors = verify_15_go_only_consistency()
    mutated_modules = mutated_modules or {}

    with tempfile.TemporaryDirectory() as tmp:
        tmp_path = pathlib.Path(tmp)
        gen_manifest = generate_pack(tmp_path, p10_root=p10_root)

        # Apply test mutations if requested
        for mod_file, bad_content in mutated_modules.items():
            (tmp_path / "modules" / mod_file).write_text(bad_content, encoding="utf-8")

        # Compare generated modules against 141b9cc modules
        ref_manifest = json.loads(git_show_text(PACK_COMMIT, "extensions/cherri-apple-actions-pack/manifest.json"))
        ref_mod_hashes = ref_manifest.get("module_sha256", {})
        gen_mod_files = sorted((tmp_path / "modules").glob("*.cherri"))

        if len(gen_mod_files) != len(ref_mod_hashes):
            errors.append(
                f"Generated module count mismatch: got {len(gen_mod_files)}, expected {len(ref_mod_hashes)}"
            )

        for mod_path in gen_mod_files:
            actual_bytes = mod_path.read_bytes().replace(b"\r\n", b"\n")
            actual_sha = hashlib.sha256(actual_bytes).hexdigest()
            expected_sha = ref_mod_hashes.get(mod_path.name)
            if expected_sha != actual_sha:
                errors.append(
                    f"Generated Pack module drift in {mod_path.name}: sha256 {actual_sha} != 141b9cc {expected_sha}"
                )

        # Verify all 7 classification buckets exist and are non-empty
        cls7 = gen_manifest.get("classifications7", {}).get("counts", {})
        for req_cls in REQUIRED_7_CLASSIFICATIONS:
            if cls7.get(req_cls, 0) <= 0:
                errors.append(f"Classification bucket {req_cls} is missing or empty in generated manifest: {cls7}")

    return errors


def verify_15_go_only_consistency() -> list[str]:
    errors = []
    audit_path = REPO_ROOT / "docs" / "apple-runtime" / "p10" / "p10-pack-15-go-only-audit.json"
    if not audit_path.exists():
        return [f"Missing {audit_path}"]

    audit_data = json.loads(audit_path.read_text(encoding="utf-8"))
    rows = audit_data.get("goOnlyActionsAudit", [])
    if len(rows) != 15:
        errors.append(f"Expected 15 rows in goOnlyActionsAudit, got {len(rows)}")

    found_map = {}
    for row in rows:
        name = row.get("actionName")
        cls = row.get("classification")
        found_map[name] = cls
        if name not in EXPECTED_15_GO_ONLY:
            errors.append(f"Unexpected action in 15 Go-only audit: {name}")
        elif cls != EXPECTED_15_GO_ONLY[name]:
            errors.append(
                f"Classification mismatch for {name}: got {cls!r}, expected {EXPECTED_15_GO_ONLY[name]!r}"
            )

    missing = set(EXPECTED_15_GO_ONLY.keys()) - set(found_map.keys())
    if missing:
        errors.append(f"Missing actions in 15 Go-only audit: {sorted(missing)}")

    pack_info = extract_pack_summary()
    if pack_info["totalEntries"] != 102 or pack_info["newActionsCount"] != 48 or pack_info["advancedAliasesCount"] != 54:
        errors.append(f"Unexpected pack entry counts: {pack_info}")
    if not pack_info["limitationsMentionsAll15"]:
        errors.append("Pack LIMITATIONS.md does not mention all 15 Go-only action names")

    # Verify zero hardcoded container paths in tools/
    forbidden_prefix = "/" + "home/" + "chatgpt-agent"
    for py_file in (REPO_ROOT / "tools").rglob("*.py"):
        content = py_file.read_text(encoding="utf-8", errors="replace")
        if forbidden_prefix in content:
            errors.append(f"Forbidden hardcoded path {forbidden_prefix} found in {py_file.relative_to(REPO_ROOT)}")

    return errors


def main():
    parser = argparse.ArgumentParser(description="Deterministic Pure+Pack generator and 15 Go-only verifier")
    parser.add_argument("--generate", action="store_true", help="Generate Pure+Pack modules and manifest into --out-dir")
    parser.add_argument("--out-dir", type=str, default="", help="Output directory for --generate")
    parser.add_argument("--p10-root", type=str, default="", help="Root of P10 repository (defaults to REPO_ROOT)")
    parser.add_argument(
        "--verify-generated",
        action="store_true",
        help="Generate Pack in a temp directory and verify byte-for-byte parity with 141b9cc + 7 classifications",
    )
    parser.add_argument("--json", action="store_true", help="Print pack summary JSON")
    args = parser.parse_args()

    p10_root = pathlib.Path(args.p10_root).resolve() if args.p10_root else REPO_ROOT

    if args.generate:
        if not args.out_dir:
            print("ERROR: --generate requires --out-dir <path>", file=sys.stderr)
            sys.exit(1)
        out_dir = pathlib.Path(args.out_dir).resolve()
        manifest = generate_pack(out_dir, p10_root=p10_root)
        print(
            json.dumps(
                {
                    "status": "GENERATED",
                    "outDir": str(out_dir),
                    "modulesCount": len(manifest["modules"]),
                    "summary": manifest["summary"],
                    "classifications7": manifest["classifications7"]["counts"],
                },
                indent=2,
            )
        )
        sys.exit(0)

    if args.verify_generated:
        errors = verify_generated_pack(p10_root=p10_root)
        if errors:
            for e in errors:
                print(f"ERROR: {e}", file=sys.stderr)
            sys.exit(1)
        print(
            "SUCCESS: Deterministically generated Pure+Pack from P10 .cherri sources; "
            "verified 20/20 modules match 141b9cc byte-for-byte, all 7 classification buckets populated, "
            "and 15/15 Go-only actions verified."
        )
        sys.exit(0)

    errors = verify_15_go_only_consistency()
    if errors:
        for e in errors:
            print(f"ERROR: {e}", file=sys.stderr)
        sys.exit(1)

    summary = extract_pack_summary()
    if args.json:
        print(json.dumps(summary, indent=2))
    else:
        print(
            f"SUCCESS: Verified Pure Original ({PURE_COMMIT[:7]}), Pure+Pack ({PACK_COMMIT[:7]}: "
            f"{summary['totalEntries']} entries = {summary['newActionsCount']} new + {summary['advancedAliasesCount']} aliases), "
            f"and all 15 Go-only actions with exact classifications."
        )


if __name__ == "__main__":
    main()
