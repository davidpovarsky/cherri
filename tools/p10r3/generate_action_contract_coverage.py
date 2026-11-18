#!/usr/bin/env python3
"""Generate P10R15 action contracts from the live canonical catalog and executed tests.

This is an evidence projection, not an action registry. The existing P10 catalog
test remains the compiler/decompiler runner; historical discovery and Pure/Pack
data remain explicitly historical. No private corpus is read by this tool.
"""
import argparse
import collections
import hashlib
import json
import pathlib
import subprocess
import sys

import generate_catalog_and_pack_ledgers as existing

ROOT = pathlib.Path(__file__).resolve().parents[2]
OUT = ROOT / "docs/apple-runtime/p10r15"
EXECUTION = ROOT / "docs/apple-runtime/p10/p10-505-execution-evidence.json"
CONTRACTS = OUT / "action-contract-coverage.json"
RECEIPT = OUT / "action-contract-execution.json"
TEST_COMMAND = ["go", "test", "-count=1", "-timeout=20m", "-run",
                "^TestP10_VerifyAndRecord505CatalogExecution$", "-v", "."]
DIMENSIONS = ("legacyNamedCompile", "v2NamedCompile", "nativeIdentifier",
              "parameterContract", "decompile", "structuralRoundtrip",
              "nativeSpecimen", "losslessImport", "hybridImport", "editIsolation",
              "signedEditor", "signedRuntime", "physicalIPad", "installedProvider")
SOURCE_GLOBS = ("*.go", "go.mod", "go.sum", "actions/*.cherri",
                "internal/**/*.go", "tests/**/*.cherri", "testdata/**/*",
                "tools/p10r3/*.py")


def digest(data):
    return hashlib.sha256(data).hexdigest()


def source_fingerprint(root=ROOT):
    files = set()
    for pattern in SOURCE_GLOBS:
        files.update(p for p in root.glob(pattern) if p.is_file())
    material = b""
    for p in sorted(files, key=lambda value: value.relative_to(root).as_posix()):
        material += (p.relative_to(root).as_posix() + ":" +
                     digest(p.read_bytes().replace(b"\r\n", b"\n")) + "\n").encode()
    return digest(material)


def run(args, timeout=120):
    result = subprocess.run(args, cwd=ROOT, capture_output=True, encoding="utf-8",
                            errors="replace", timeout=timeout)
    if result.returncode:
        raise RuntimeError("command failed (exit %d): %s\n%s" %
                           (result.returncode, " ".join(args),
                            (result.stdout + result.stderr)[-4000:]))
    return result.stdout


def read_json(path):
    return json.loads(path.read_text(encoding="utf-8"))


def write_json(path, value):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_bytes((json.dumps(value, ensure_ascii=False, indent=2) + "\n").encode("utf-8"))


def refresh_receipt():
    before = source_fingerprint()
    log = run(TEST_COMMAND, timeout=1260)
    if "PASS: TestP10_VerifyAndRecord505CatalogExecution" not in log:
        raise ValueError("CATALOG_TEST_NOT_EXECUTED")
    if source_fingerprint() != before:
        raise ValueError("SOURCE_CHANGED_DURING_EXECUTION")
    log_path = ROOT / "dist/p10r15/catalog-contract-execution.log"
    log_path.parent.mkdir(parents=True, exist_ok=True)
    log_path.write_bytes(log.encode("utf-8"))
    receipt = {
        "schemaVersion": 1,
        "sourceSha": run(["git", "rev-parse", "HEAD"]).strip(),
        "sourceFingerprint": before,
        "command": TEST_COMMAND,
        "exitCode": 0,
        "testExecuted": "TestP10_VerifyAndRecord505CatalogExecution",
        "executionEvidenceSha256": digest(EXECUTION.read_bytes()),
        "localLog": log_path.relative_to(ROOT).as_posix(),
        "localLogSha256": digest(log.encode()),
        "scope": "Synthetic compiler/decompiler cases, not Apple runtime or private corpus",
    }
    write_json(RECEIPT, receipt)
    return receipt


def index_unique(rows, key):
    indexed = {}
    for row in rows:
        name = row[key]
        if name in indexed:
            raise ValueError("DUPLICATE_ENTRY:" + name)
        indexed[name] = row
    return indexed


def validate_receipt(receipt, fingerprint, evidence_sha):
    if receipt.get("sourceFingerprint") != fingerprint:
        raise ValueError("STALE_SOURCE_EVIDENCE")
    if receipt.get("executionEvidenceSha256") != evidence_sha:
        raise ValueError("STALE_EXECUTION_EVIDENCE")
    if receipt.get("exitCode") != 0 or receipt.get("command") != TEST_COMMAND:
        raise ValueError("UNVERIFIED_EXECUTION_COMMAND")
    if receipt.get("testExecuted") != "TestP10_VerifyAndRecord505CatalogExecution":
        raise ValueError("CATALOG_TEST_NOT_EXECUTED")


def status(value):
    return "PASS" if value is True else "FAIL"


def build_contracts(catalog, execution, receipt, sources, historical):
    actions = index_unique(catalog["actions"], "name")
    observations = index_unique(execution["entries"], "name")
    if set(actions) != set(observations):
        raise ValueError("CATALOG_EVIDENCE_MEMBERSHIP_MISMATCH")
    old_rows = index_unique(historical.get("entries", []), "cherriName")
    rows = []
    for name, item in sorted(actions.items()):
        ev = observations[name]
        if item.get("shortcutIdentifier", "") != ev.get("shortcutIdentifier", ""):
            raise ValueError("NATIVE_IDENTIFIER_MISMATCH:" + name)
        construct = bool(item.get("compilerConstruct"))
        if construct != bool(ev.get("compilerConstruct")):
            raise ValueError("CONSTRUCT_KIND_MISMATCH:" + name)
        if name not in sources:
            raise ValueError("MISSING_CANONICAL_SOURCE:" + name)
        src_file, src_line = sources[name]
        old = old_rows.get(name, {})
        same_native_id = old.get("shortcutIdentifier") == item.get("shortcutIdentifier")
        dimensions = {
            "legacyNamedCompile": status(ev.get("legacySupported")),
            "v2NamedCompile": status(ev.get("v2Supported")),
            "nativeIdentifier": status(ev.get("plistIdentifierVerified")),
            "parameterContract": status(ev.get("parameterContractVerified")),
            "decompile": status(ev.get("decompileSupported")),
            "structuralRoundtrip": status(ev.get("structuralRoundTripPass")),
            "nativeSpecimen": ev.get("nativeSpecimenParityStatus", "NO_NATIVE_SPECIMEN"),
            "losslessImport": "NOT_SAMPLED_PER_CALLABLE",
            "hybridImport": "NOT_SAMPLED_PER_CALLABLE",
            "editIsolation": "NOT_SAMPLED_PER_CALLABLE",
            "signedEditor": "NOT_SAMPLED_PER_CALLABLE",
            "signedRuntime": "NOT_SAMPLED_PER_CALLABLE",
            "physicalIPad": "BLOCKED_DEVICE",
            "installedProvider": "NOT_CHECKED",
        }
        # The catalog runner cannot emit runtime proof. A modified status string
        # must not promote its compiler sample to an Apple-executed action.
        if ev.get("appleRuntimeExecutedStatus") == "APPLE_RUNTIME_EXECUTED":
            raise ValueError("RUNTIME_PROOF_REQUIRES_INDEPENDENT_RECEIPT:" + name)
        intent = item.get("appIntent")
        provider = {"kind": "COMPILER_CONSTRUCT" if construct else
                    ("APP_INTENT" if intent else "NATIVE_ACTION"),
                    "descriptor": intent, "installedVersion": "NOT_CHECKED"}
        parameters = []
        for p in item.get("parameters", []):
            parameters.append({
                **p,
                "required": not bool(p.get("optional") or "default" in p),
                "defaultEvidence": "DECLARED_BY_COMPILER" if "default" in p else "NOT_DECLARED",
                "enumEvidence": "DECLARED_BY_COMPILER_NOT_NATIVE_CONFIRMED" if p.get("enum") else "NOT_APPLICABLE",
                "nativeVariants": "NOT_SAMPLED_EXHAUSTIVELY",
            })
        compile_ok = all(dimensions[k] == "PASS" for k in DIMENSIONS[:6])
        row = {
            "callable": name,
            "nativeIdentifier": item.get("shortcutIdentifier", ""),
            "kind": provider["kind"],
            "status": "COMPILER_CONSTRUCT" if construct else ("PARTIALLY_VERIFIED" if compile_ok else "FAIL"),
            "category": item.get("category", "syntax" if construct else "UNCLASSIFIED"),
            "source": {"path": src_file, "line": src_line},
            "provider": provider,
            "platform": {k: item[k] for k in ("macOnly", "nonMacOnly", "minVersion", "maxVersion") if k in item},
            "platformEvidence": "DECLARED_CONSTRAINTS_ONLY",
            "parameters": parameters,
            "staticParameters": item.get("staticParameters", {}),
            "emittedKeysSample": ev.get("emittedKeysSample", []),
            "outputType": item.get("outputType", "NOT_DECLARED"),
            "evidenceTier": "STRUCTURAL_NATIVE_SYNTHETIC" if compile_ok else "FAILED_SYNTHETIC_CONTRACT",
            "dimensions": dimensions,
            "decompileMode": ev.get("decompileMode", "NOT_SAMPLED"),
            "nativeSnapshots": {"minimum": "NOT_CAPTURED_IN_EXISTING_RUNNER",
                                "full": "NOT_CAPTURED_IN_EXISTING_RUNNER"},
            "syntheticSourceHashes": {dialect: digest(ev.get(dialect + "Source", "").encode())
                                      for dialect in ("legacy", "v2")},
            "test": "p10_catalog_execution_test.go::TestP10_VerifyAndRecord505CatalogExecution[" + name + "]",
            "receipt": "action-contract-execution.json",
            "historicalPurePack": {
                "sourceSha": historical.get("sourceCommit", "UNKNOWN"),
                "classification": old.get("packSupportKind", "NOT_CLASSIFIED") if same_native_id else "STALE_IDENTIFIER",
                "status": "HISTORICAL_CLASSIFICATION_NOT_FRESH_INSTALLED_TEST",
            },
            "limitations": [
                "One synthetic compiler sample does not establish every optional/type/enum variant",
                "Native Apple specimen, editor, runtime and physical acceptance are independent",
            ],
        }
        rows.append(row)
    counts = {key: dict(sorted(collections.Counter(r["dimensions"][key] for r in rows).items()))
              for key in DIMENSIONS}
    native_ids = {r["nativeIdentifier"] for r in rows if r["kind"] != "COMPILER_CONSTRUCT"}
    return {
        "schemaVersion": 1,
        "sourceSha": receipt["sourceSha"],
        "sourceFingerprint": receipt["sourceFingerprint"],
        "catalogSha256": digest(json.dumps(catalog, sort_keys=True, ensure_ascii=False).encode()),
        "executionEvidenceSha256": receipt["executionEvidenceSha256"],
        "totalCatalogEntries": len(rows),
        "summary": {
            "callableEntries": sum(r["kind"] != "COMPILER_CONSTRUCT" for r in rows),
            "compilerConstructs": sum(r["kind"] == "COMPILER_CONSTRUCT" for r in rows),
            "uniqueCallableNativeIdentifiers": len(native_ids),
            "dimensions": counts,
        },
        "scope": "All current catalog entries; compiler-only macros remain outside this Apple-action denominator",
        "historicalDenominators": {
            "appleBuiltins": {"covered": 316, "discovered": 434, "status": "HISTORICAL_NOT_CURRENT_DISCOVERY"},
            "installedAppIntents": {"covered": 11, "discovered": 184, "status": "HISTORICAL_NOT_CURRENT_DISCOVERY"},
            "macOSExtraction": {"discovered": 2396, "eligible": 1527, "coveredPartial": 390,
                               "newNativeAction": 97, "missingAppIntentDescriptor": 1040,
                               "excludedDeferred": 869, "status": "HISTORICAL_NOT_CURRENT_DISCOVERY"},
            "relationship": "Different populations; do not add counts or infer missing current callable actions",
        },
        "entries": rows,
    }


def render_markdown(report):
    lines = [
        "# P10R15 canonical action contract coverage", "",
        "Generated by tools/p10r3/generate_action_contract_coverage.py from the live catalog.",
        "Every PASS below is bounded to the synthetic compiler contract runner.",
        "No row claims signed runtime, installed-provider or physical iPad success.", "",
        "- Source SHA: `" + report["sourceSha"] + "`",
        "- Source fingerprint: `" + report["sourceFingerprint"] + "`",
        "- Catalog entries: " + str(report["totalCatalogEntries"]),
        "- Unique callable native identifiers: " + str(report["summary"]["uniqueCallableNativeIdentifiers"]), "",
        "Importing 38 complete native workflows losslessly is a separate gate; it cannot validate all named callable variants.",
        "Absence of a platform constraint means undeclared availability, not universal availability.", "",
        "| Callable | Native identifier | Legacy | V2 | Roundtrip | Native specimen | Release status |",
        "|---|---|---|---|---|---|---|",
    ]
    for row in report["entries"]:
        d = row["dimensions"]
        lines.append("| " + " | ".join([row["callable"], row["nativeIdentifier"],
                     d["legacyNamedCompile"], d["v2NamedCompile"],
                     d["structuralRoundtrip"], d["nativeSpecimen"], row["status"]]) + " |")
    return "\n".join(lines) + "\n"


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    mode = parser.add_mutually_exclusive_group(required=True)
    mode.add_argument("--refresh", action="store_true", help="execute current catalog runner and regenerate")
    mode.add_argument("--verify", action="store_true", help="reject stale or incomplete committed evidence")
    args = parser.parse_args()
    receipt = refresh_receipt() if args.refresh else read_json(RECEIPT)
    validate_receipt(receipt, source_fingerprint(), digest(EXECUTION.read_bytes()))
    catalog = json.loads(run(["go", "run", ".", "--actions-json", "--no-ansi"]))
    report = build_contracts(catalog, read_json(EXECUTION), receipt,
                             existing.locate_exact_action_sources(),
                             read_json(ROOT / "docs/apple-runtime/p10/p10-505-catalog-coverage-matrix.json"))
    md = render_markdown(report)
    if args.verify:
        if read_json(CONTRACTS) != report:
            raise ValueError("STALE_OR_MUTATED_CONTRACT_LEDGER")
        if (OUT / "action-contract-coverage.md").read_text(encoding="utf-8") != md:
            raise ValueError("STALE_CONTRACT_DOCUMENTATION")
    else:
        write_json(CONTRACTS, report)
        (OUT / "action-contract-coverage.md").write_bytes(md.encode("utf-8"))
    print(json.dumps({"status": "PASS", "entries": len(report["entries"]),
                      "uniqueCallableNativeIdentifiers": report["summary"]["uniqueCallableNativeIdentifiers"],
                      "sourceSha": report["sourceSha"],
                      "scope": report["scope"]}))


if __name__ == "__main__":
    try:
        main()
    except (RuntimeError, ValueError, OSError, subprocess.TimeoutExpired) as exc:
        print(str(exc), file=sys.stderr)
        sys.exit(1)
