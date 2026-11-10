#!/usr/bin/env python3
"""tools/p10r3/verify_consumer_compatibility.py

P10R5 Part E — machine-verified consumer compatibility matrix.

Verifies that every consumer of the Cherri compiler is synchronized to the same
schema/codec contract, and writes:
  docs/apple-runtime/p10/consumer-compatibility-matrix.json
  docs/apple-runtime/p10/consumer-compatibility-matrix.md

Components:
  1. Compiler        (live `go run . --capabilities-json`)
  2. Documentation   (cherri docs/language-v2/actions-reference.md + cherrilang.org fork-language/actions.md
                      + cherrilang.org scripts/smoke_test_snippets.py PINNED_CHERRI_COMMIT)
  3. Skill           (skills/cherri-shortcuts/compatibility-manifest.json)
  4. Remote Builder  (consumes the Skill manifest compilerCommit; remote host not reachable from here)
  5. iOS Bridge      (ios_bridge.go / mobile_compiler.go use schema.DefaultRegistry())
  6. iOS UI          (CherriCompiler.swift decodes schemaFingerprint and calls CherriActionCatalog)
  7. Preview         (ios/WebPreview/package.json + package-lock.json pin of preview-shortcut)
  8. Pure+Pack       (141b9cc generator byte-parity via verify_or_generate_pack.py)

Statuses: PASS | FAIL | NOT_AVAILABLE_LOCALLY | BLOCKED_EXTERNAL.
Only PASS/FAIL are verdict-bearing; NOT_AVAILABLE_LOCALLY / BLOCKED_EXTERNAL are never counted as PASS.

Usage:
  python tools/p10r3/verify_consumer_compatibility.py               # verify + write matrix
  python tools/p10r3/verify_consumer_compatibility.py --check       # verify only (no write)
  python tools/p10r3/verify_consumer_compatibility.py --mutate-check  # prove 8 drift mutations fail closed (E8)
  Env: CHERRI_DOCS_DIR, PREVIEW_SHORTCUT_DIR (default: sibling checkouts), REQUIRE_ALL_CONSUMERS=1
"""

import argparse
import copy
import json
import os
import pathlib
import re
import subprocess
import sys

REPO_ROOT = pathlib.Path(__file__).resolve().parent.parent.parent
sys.path.insert(0, str(REPO_ROOT / "tools" / "p10r3"))

PACK_SHA = "141b9cc96cf1df3f9d1593548dc49ae8a46a256a"
PURE_SHA = "bcc0095754e2cc0baaeb4d93dae172b8153b4716"
OUT_DIR = REPO_ROOT / "docs" / "apple-runtime" / "p10"
HEX64 = re.compile(r"^[0-9a-f]{64}$")
HEX40 = re.compile(r"^[0-9a-f]{40}$")
FP_RE = re.compile(r"\*\*Schema Fingerprint:\*\*\s*`([0-9a-f]{64})`")

PASS, FAIL, NA, BLOCKED = "PASS", "FAIL", "NOT_AVAILABLE_LOCALLY", "BLOCKED_EXTERNAL"


def git(args, cwd=REPO_ROOT, check=False):
    return subprocess.run(["git"] + args, cwd=cwd, capture_output=True, text=True, encoding="utf-8", errors="replace", check=check)


def is_ancestor(sha, ref="HEAD", cwd=REPO_ROOT):
    if not sha or not HEX40.match(sha):
        return False
    if git(["cat-file", "-e", f"{sha}^{{commit}}"], cwd=cwd).returncode != 0:
        return False
    return git(["merge-base", "--is-ancestor", sha, ref], cwd=cwd).returncode == 0


def read_text(p):
    return pathlib.Path(p).read_text(encoding="utf-8") if pathlib.Path(p).is_file() else None


def gather_inputs():
    """Collect raw inputs from all consumer surfaces (mutations are applied to this dict)."""
    caps = json.loads(
        subprocess.run(["go", "run", ".", "--capabilities-json"], cwd=REPO_ROOT, capture_output=True, text=True, check=True).stdout
    )
    docs_dir = pathlib.Path(os.environ.get("CHERRI_DOCS_DIR", REPO_ROOT.parent / "cherrilang.org"))
    preview_dir = pathlib.Path(os.environ.get("PREVIEW_SHORTCUT_DIR", REPO_ROOT.parent / "preview-shortcut"))
    docs_available = (docs_dir / ".git").exists()
    preview_available = (preview_dir / ".git").exists()

    smoke = read_text(docs_dir / "scripts" / "smoke_test_snippets.py") if docs_available else None
    smoke_pin = None
    if smoke:
        m = re.search(r'PINNED_CHERRI_COMMIT\s*=\s*"([0-9a-f]{40})"', smoke)
        smoke_pin = m.group(1) if m else None

    pkg = json.loads(read_text(REPO_ROOT / "ios" / "WebPreview" / "package.json"))
    lock = json.loads(read_text(REPO_ROOT / "ios" / "WebPreview" / "package-lock.json"))
    pkg_spec = pkg.get("dependencies", {}).get("preview-shortcut", "")
    lock_resolved = lock.get("packages", {}).get("node_modules/preview-shortcut", {}).get("resolved", "")

    return {
        "headSha": git(["rev-parse", "HEAD"], check=True).stdout.strip(),
        "capabilities": caps,
        "cherriDocsReference": read_text(REPO_ROOT / "docs" / "language-v2" / "actions-reference.md") or "",
        "skillManifest": json.loads(read_text(REPO_ROOT / "skills" / "cherri-shortcuts" / "compatibility-manifest.json")),
        "docsAvailable": docs_available,
        "docsDir": str(docs_dir),
        "docsHead": git(["rev-parse", "HEAD"], cwd=docs_dir).stdout.strip() if docs_available else None,
        "docsSiteReference": read_text(docs_dir / "fork-language" / "actions.md") if docs_available else None,
        "docsSmokePin": smoke_pin,
        "iosBridgeSource": read_text(REPO_ROOT / "ios_bridge.go") or "",
        "mobileCompilerSource": read_text(REPO_ROOT / "mobile_compiler.go") or "",
        "iosCompilerSwift": read_text(REPO_ROOT / "ios" / "CherriApp" / "Compiler" / "CherriCompiler.swift") or "",
        "previewPackageSpec": pkg_spec,
        "previewLockResolved": lock_resolved,
        "previewAvailable": preview_available,
        "previewDir": str(preview_dir),
        "packSha": PACK_SHA,
    }


def evaluate(inp, run_pack_generator=True):
    head = inp["headSha"]
    caps = inp["capabilities"]
    schema_fp = caps.get("schemaFingerprint", "")
    codec_fp = caps.get("codecFingerprint", "")
    rows = []

    def row(component, source_commit, tested_against, checks, status=None, notes=""):
        failures = [msg for ok, msg in checks if not ok]
        if failures:
            status = FAIL
        elif status is None:
            status = PASS
        rows.append(
            {
                "component": component,
                "sourceCommit": source_commit,
                "schemaFingerprint": schema_fp,
                "codecFingerprint": codec_fp,
                "testedAgainst": tested_against,
                "status": status,
                "checks": [{"ok": ok, "check": msg} for ok, msg in checks],
                "failures": failures,
                "notes": notes,
            }
        )

    # 1. Compiler
    row(
        "Compiler",
        head,
        "live `go run . --capabilities-json`",
        [
            (bool(HEX64.match(schema_fp)), "schemaFingerprint is 64-hex"),
            (bool(HEX64.match(codec_fp)), "codecFingerprint is 64-hex"),
            (schema_fp != codec_fp, "codecFingerprint independent of schemaFingerprint"),
            (caps.get("actionCount") == 505, f"actionCount == 505 (got {caps.get('actionCount')})"),
            (caps.get("languageVersion") == "2.0", "languageVersion == 2.0"),
        ],
    )

    # 2. Documentation
    m = FP_RE.search(inp["cherriDocsReference"])
    doc_checks = [(bool(m) and m.group(1) == schema_fp, "cherri docs/language-v2/actions-reference.md fingerprint == compiler")]
    if inp["docsAvailable"]:
        ms = FP_RE.search(inp["docsSiteReference"] or "")
        doc_checks += [
            (bool(ms) and ms.group(1) == schema_fp,
             f"cherrilang.org fork-language/actions.md fingerprint == compiler (got {ms.group(1) if ms else 'none'})"),
            (is_ancestor(inp["docsSmokePin"], head),
             f"smoke_test_snippets.py PINNED_CHERRI_COMMIT ({inp['docsSmokePin']}) is an ancestor of compiler HEAD"),
        ]
        row("Documentation", inp["docsHead"], f"compiler {head[:12]} schema {schema_fp[:12]}", doc_checks)
    else:
        row("Documentation", None, "cherri in-repo reference only", doc_checks, status=NA,
            notes=f"cherrilang.org checkout not present at {inp['docsDir']}; site copy and smoke pin not verified.")

    # 3. Skill
    sm = inp["skillManifest"]
    skill_checks = [
        (sm.get("schemaFingerprint") == schema_fp, f"skill schemaFingerprint == compiler (got {sm.get('schemaFingerprint')})"),
        (sm.get("codecFingerprint") == codec_fp, f"skill codecFingerprint == compiler (got {sm.get('codecFingerprint')})"),
        (sm.get("languageVersion") == caps.get("languageVersion"), "skill languageVersion == compiler"),
        (sm.get("catalogSchemaVersion") == caps.get("catalogSchemaVersion"), "skill catalogSchemaVersion == compiler"),
        (is_ancestor(sm.get("compilerCommit"), head), f"skill compilerCommit ({sm.get('compilerCommit')}) is an ancestor of HEAD"),
    ]
    if inp["docsAvailable"]:
        skill_checks.append(
            (sm.get("docsCommit") == inp["docsHead"], f"skill docsCommit ({sm.get('docsCommit')}) == docs HEAD ({inp['docsHead']})")
        )
    row("Skill", sm.get("compilerCommit"), f"docs {str(sm.get('docsCommit'))[:12]}", skill_checks)

    # 4. Remote Builder — consumes the skill tuple; the remote Linux ARM64 host is not reachable from this environment.
    row(
        "Remote Builder",
        sm.get("compilerCommit"),
        "skill compatibility-manifest.json tuple",
        [(is_ancestor(sm.get("compilerCommit"), head), "builder input compilerCommit is an ancestor of HEAD")],
        status=BLOCKED,
        notes="Remote Linux ARM64 builder host is external; only its input tuple is verified locally. Not counted as PASS.",
    )

    # 5. iOS Bridge
    bridge = inp["iosBridgeSource"]
    mobile = inp["mobileCompilerSource"]
    row(
        "iOS Bridge",
        head,
        "schema.DefaultRegistry() (same registry as compiler)",
        [
            ("//export CherriActionCatalog" in bridge, "ios_bridge.go exports CherriActionCatalog"),
            ("SchemaFingerprint: fp" in bridge, "ios_bridge.go returns registry-derived SchemaFingerprint"),
            ("schema.DefaultRegistry()" in mobile, "mobile_compiler.go consumes schema.DefaultRegistry()"),
        ],
    )

    # 6. iOS UI
    swift = inp["iosCompilerSwift"]
    hard_fp = re.findall(r'"([0-9a-f]{64})"', swift)
    row(
        "iOS UI",
        head,
        "iOS Bridge C ABI",
        [
            ("CherriActionCatalog()" in swift, "CherriCompiler.swift calls CherriActionCatalog()"),
            ("schemaFingerprint" in swift, "CherriCompiler.swift decodes schemaFingerprint from bridge"),
            (not hard_fp, f"no hardcoded fingerprint literals in CherriCompiler.swift ({len(hard_fp)} found)"),
        ],
        notes="Swift build/runtime verification requires macOS/Xcode (macOS CI job); static contract checked here.",
    )

    # 7. Preview
    spec_m = re.search(r"#([0-9a-f]{40})$", inp["previewPackageSpec"])
    lock_m = re.search(r"#([0-9a-f]{40})$", inp["previewLockResolved"])
    pin = spec_m.group(1) if spec_m else None
    prev_checks = [
        (bool(pin), f"package.json pins preview-shortcut to a 40-hex commit ({inp['previewPackageSpec']})"),
        (bool(lock_m) and lock_m.group(1) == pin, "package-lock.json resolved commit == package.json pin"),
    ]
    if inp["previewAvailable"]:
        pdir = inp["previewDir"]
        prev_checks.append(
            (git(["cat-file", "-e", f"{pin}^{{commit}}"], cwd=pdir).returncode == 0 if pin else False,
             "pinned preview commit exists in preview-shortcut repository")
        )
    row("Preview", pin, "ios/WebPreview lock + generic catalog metadata renderer", prev_checks,
        notes="" if inp["previewAvailable"] else "preview-shortcut checkout not present; commit existence not verified.")

    # 8. Pure+Pack
    pack_checks = [
        (inp["packSha"] == PACK_SHA, f"Pure+Pack commit == {PACK_SHA}"),
        (git(["cat-file", "-e", f"{inp['packSha']}^{{commit}}"]).returncode == 0, "Pure+Pack commit object present"),
        (git(["cat-file", "-e", f"{PURE_SHA}^{{commit}}"]).returncode == 0, "Pure Original commit object present (read-only)"),
    ]
    if run_pack_generator:
        import verify_or_generate_pack as pack_ver
        errs = pack_ver.verify_generated_pack(p10_root=REPO_ROOT)
        pack_checks.append((errs == [], f"deterministic generator byte-parity with 141b9cc ({len(errs)} drift errors)"))
    row("Pure+Pack", inp["packSha"], f"P10 catalog @ {head[:12]}", pack_checks)

    return rows


def verdict(rows):
    if any(r["status"] == FAIL for r in rows):
        return "INCOMPATIBLE"
    if any(r["status"] in (NA, BLOCKED) for r in rows):
        return "COMPATIBLE_WITH_EXTERNAL_BLOCKERS"
    return "COMPATIBLE"


MUTATIONS = {
    # E8: 8 drift scenarios that MUST be detected
    "e8_1_skill_schema_fingerprint_drift": lambda i: i["skillManifest"].__setitem__("schemaFingerprint", "0" * 64),
    "e8_2_skill_codec_fingerprint_drift": lambda i: i["skillManifest"].__setitem__("codecFingerprint", "1" * 64),
    "e8_3_docs_reference_stale_fingerprint": lambda i: i.__setitem__(
        "cherriDocsReference", FP_RE.sub("**Schema Fingerprint:** `" + "2" * 64 + "`", i["cherriDocsReference"])),
    "e8_4_skill_compiler_commit_not_ancestor": lambda i: i["skillManifest"].__setitem__("compilerCommit", "deadbeef" * 5),
    "e8_5_ios_bridge_bypasses_registry": lambda i: i.__setitem__(
        "mobileCompilerSource", i["mobileCompilerSource"].replace("schema.DefaultRegistry()", "localHandMaintainedRegistry()")),
    "e8_6_ios_ui_hardcoded_fingerprint": lambda i: i.__setitem__(
        "iosCompilerSwift", i["iosCompilerSwift"] + '\nlet pinnedFP = "' + "3" * 64 + '"\n'),
    "e8_7_preview_lock_pin_mismatch": lambda i: i.__setitem__(
        "previewLockResolved", re.sub(r"#[0-9a-f]{40}$", "#" + "4" * 40, i["previewLockResolved"])),
    "e8_8_compiler_action_count_drift": lambda i: i["capabilities"].__setitem__("actionCount", 504),
}


def write_outputs(rows, inp):
    v = verdict(rows)
    doc = {
        "schemaVersion": "1.0",
        "generator": "tools/p10r3/verify_consumer_compatibility.py",
        "compilerHead": inp["headSha"],
        "schemaFingerprint": inp["capabilities"].get("schemaFingerprint"),
        "codecFingerprint": inp["capabilities"].get("codecFingerprint"),
        "verdict": v,
        "statusCounts": {s: sum(1 for r in rows if r["status"] == s) for s in (PASS, FAIL, NA, BLOCKED)},
        "components": rows,
    }
    (OUT_DIR / "consumer-compatibility-matrix.json").write_text(json.dumps(doc, indent=2) + "\n", encoding="utf-8")
    lines = [
        "# P10R5 — Consumer Compatibility Matrix",
        "",
        f"- **Generator**: `tools/p10r3/verify_consumer_compatibility.py`",
        f"- **Compiler HEAD at generation**: `{inp['headSha']}`",
        f"- **Verdict**: `{v}`",
        "",
        "> `NOT_AVAILABLE_LOCALLY` and `BLOCKED_EXTERNAL` are never counted as `PASS`.",
        "",
        "| Component | Source Commit | Schema Fingerprint | Codec Fingerprint | Tested Against | Status |",
        "| :--- | :--- | :--- | :--- | :--- | :--- |",
    ]
    for r in rows:
        sc = f"`{r['sourceCommit'][:12]}`" if r["sourceCommit"] else "—"
        lines.append(
            f"| {r['component']} | {sc} | `{r['schemaFingerprint'][:12]}` | `{r['codecFingerprint'][:12]}` | {r['testedAgainst']} | `{r['status']}` |"
        )
    lines += ["", "## Check Details", ""]
    for r in rows:
        lines.append(f"### {r['component']} — `{r['status']}`")
        for c in r["checks"]:
            lines.append(f"- [{'x' if c['ok'] else ' '}] {c['check']}")
        if r["notes"]:
            lines.append(f"- _Note_: {r['notes']}")
        lines.append("")
    (OUT_DIR / "consumer-compatibility-matrix.md").write_text("\n".join(lines), encoding="utf-8")
    return v


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--check", action="store_true", help="verify only, do not write artifacts")
    ap.add_argument("--mutate-check", action="store_true", help="prove the 8 E8 drift mutations fail closed")
    args = ap.parse_args()

    base = gather_inputs()

    if args.mutate_check:
        failed_to_detect = []
        for name, mut in MUTATIONS.items():
            inp = copy.deepcopy(base)
            mut(inp)
            rows = evaluate(inp, run_pack_generator=False)
            if verdict(rows) != "INCOMPATIBLE":
                failed_to_detect.append(name)
            else:
                bad = [r["component"] for r in rows if r["status"] == FAIL]
                print(f"  [DETECTED] {name} -> FAIL in {bad}")
        if failed_to_detect:
            print(f"ERROR: mutations not detected: {failed_to_detect}", file=sys.stderr)
            sys.exit(1)
        print(f"SUCCESS: All {len(MUTATIONS)} consumer drift mutations correctly failed closed.")
        sys.exit(0)

    rows = evaluate(base)
    v = write_outputs(rows, base) if not args.check else verdict(rows)
    for r in rows:
        print(f"{r['component']:<15} {r['status']:<22} {'; '.join(r['failures'])}")
    print(f"VERDICT: {v}")
    require_all = os.environ.get("REQUIRE_ALL_CONSUMERS") == "1"
    if v == "INCOMPATIBLE" or (require_all and v != "COMPATIBLE"):
        sys.exit(1)
    sys.exit(0)


if __name__ == "__main__":
    main()
