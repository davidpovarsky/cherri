#!/usr/bin/env python3
"""Compile uniquely named Legacy fixtures, then bind existing signer receipts.

No plist is authored, repaired, renamed, normalized or signed here. The finalize
phase checks the actual Cherri signer record and exact bytes; AEA1 is a header
check, not an independent cryptographic verification or an Apple runtime pass.
"""
import argparse
import datetime
import hashlib
import json
from pathlib import Path
import plistlib
import re
import subprocess
import sys
import time

HERE = Path(__file__).resolve().parent
DEFAULT_MANIFEST = HERE.parents[1] / "tests/differential_30/manifest.json"


def require(condition, message):
    if not condition:
        raise ValueError(message)


def digest(data):
    return hashlib.sha256(data).hexdigest()


def read_json(path):
    def unique(pairs):
        result = {}
        for key, value in pairs:
            require(key not in result, "Duplicate JSON key: " + key)
            result[key] = value
        return result
    return json.loads(Path(path).read_text(encoding="utf-8"), object_pairs_hook=unique)


def write_json(path, value):
    temporary = path.with_suffix(path.suffix + ".tmp")
    temporary.write_text(json.dumps(value, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    temporary.replace(path)


def checked_bytes(path, expected):
    require(re.fullmatch(r"[0-9a-f]{64}", expected or ""), "Invalid SHA-256 for " + str(path))
    data = Path(path).read_bytes()
    require(digest(data) == expected, "Hash mismatch: " + str(path))
    return data


def contained(root, relative):
    path = (root / relative).resolve()
    require(path.is_relative_to(root.resolve()), "Path escapes artifact root: " + str(relative))
    return path


def suite_cases(manifest):
    cases = [case for case in manifest["fixtures"] if 1 <= case["number"] <= 37]
    require(sorted(case["number"] for case in cases) == list(range(1, 38)),
            "Expected exactly one fixture for each number 01 through 37")
    require(len({case["id"] for case in cases}) == 37, "Duplicate fixture id")
    for case in cases:
        runtime = case["runtime"]
        require(runtime.get("enabled") is True and runtime.get("external_side_effects") is False
                and runtime.get("terminal") == "output", "Ineligible native fixture: " + case["id"])
        require(case["expected"]["type"] == "text" and isinstance(case["expected"]["value"], str),
                "Expected output must be exact text: " + case["id"])
    return sorted(cases, key=lambda case: case["number"])


def renamed_source(data, old_name, new_name):
    old = b"#define name " + old_name.encode("utf-8")
    lines = data.splitlines(keepends=True)
    require(lines and lines[0].rstrip(b"\r\n") == old, "Expected leading name directive: " + old_name)
    require(len(re.findall(rb"(?m)^#define[ \t]+name[ \t]+", data)) == 1,
            "Ambiguous workflow name directives: " + old_name)
    return b"#define name " + new_name.encode("utf-8") + data[len(old):]


def compile_cases(options):
    compiler = options.compiler.resolve(strict=True)
    checked_bytes(compiler, options.compiler_sha256)
    require(re.fullmatch(r"[0-9a-f]{40}", options.compiler_commit), "Expected full compiler commit SHA")
    manifest_path = options.manifest.resolve(strict=True)
    manifest_bytes = manifest_path.read_bytes()
    manifest = read_json(manifest_path)
    cases = suite_cases(manifest)
    out = options.out.resolve()
    out.mkdir(parents=True, exist_ok=False)
    for directory in ("inputs", "sources", "artifacts", "logs"):
        (out / directory).mkdir()
    (out / "suite-manifest.json").write_bytes(manifest_bytes)
    flags = ["--legacy", "--skip-sign", "--no-ansi"]
    if options.derive_uuids:
        flags.append("--derive-uuids")
    receipt = {"schemaVersion": 1, "suiteId": manifest["suite_id"], "status": "COMPILING",
               "createdUTC": datetime.datetime.now(datetime.timezone.utc).isoformat(),
               "suiteManifestSHA256": digest(manifest_bytes), "generatorSHA256": digest(Path(__file__).read_bytes()),
               "compilerPath": str(compiler), "compilerSHA256": options.compiler_sha256,
               "declaredCompilerCommitSHA": options.compiler_commit, "compilerFlags": flags,
               "runtimeStatus": "NOT_RUN", "signingStatus": "NOT_SIGNED", "cases": []}
    for case in cases:
        name = f"CHERRI_QA_D30_L_{case['number']:02d}"
        old_name = case["shortcut_names"]["fork_legacy"]
        original = contained(manifest_path.parent, case["sources"]["fork_legacy"])
        original_bytes = original.read_bytes()
        staged_bytes = renamed_source(original_bytes, old_name, name)
        input_relative = f"inputs/{case['number']:02d}.cherri"
        source_relative = f"sources/{name}.cherri"
        unsigned_relative = f"artifacts/{name}.shortcut"
        log_relative = f"logs/{name}.log"
        (out / input_relative).write_bytes(original_bytes)
        (out / source_relative).write_bytes(staged_bytes)
        row = {"id": case["id"], "number": case["number"], "lane": "fork_legacy", "name": name,
               "originalName": old_name, "originalSource": str(original),
               "inputSource": input_relative, "inputSourceSHA256": digest(original_bytes),
               "source": source_relative, "sourceSHA256": digest(staged_bytes),
               "unsignedFile": unsigned_relative, "log": log_relative,
               "expected": case["expected"]["value"], "input": case.get("input")}
        command = [str(compiler), str(out / source_relative), *flags,
                   "--output=" + str(out / unsigned_relative)]
        row["command"] = command
        start = time.monotonic()
        try:
            result = subprocess.run(command, cwd=out / "sources", capture_output=True, timeout=options.timeout)
            log = result.stdout + b"\n--- STDERR ---\n" + result.stderr
            row["exit"] = result.returncode
            (out / log_relative).write_bytes(log)
            require(result.returncode == 0, "Compiler exited " + str(result.returncode))
            checked_bytes(out / source_relative, row["sourceSHA256"])
            unsigned = (out / unsigned_relative).read_bytes()
            document = plistlib.loads(unsigned)
            require(isinstance(document, dict) and isinstance(document.get("WFWorkflowActions"), list)
                    and document["WFWorkflowActions"], "Compiler did not emit a nonempty native workflow")
            row.update(status="COMPILED", unsignedSHA256=digest(unsigned), unsignedByteSize=len(unsigned),
                       nativeActionCount=len(document["WFWorkflowActions"]))
        except subprocess.TimeoutExpired as error:
            (out / log_relative).write_bytes((error.stdout or b"") + b"\n--- STDERR ---\n" + (error.stderr or b""))
            row.update(status="COMPILE_FAILED", error="Compiler timeout", exit=None)
        except (OSError, ValueError, plistlib.InvalidFileException) as error:
            row.update(status="COMPILE_FAILED", error=str(error))
        row["durationSeconds"] = round(time.monotonic() - start, 3)
        if (out / log_relative).is_file():
            row["logSHA256"] = digest((out / log_relative).read_bytes())
        receipt["cases"].append(row)
        write_json(out / "compilation-receipt.json", receipt)
        print(json.dumps({"id": row["id"], "name": name, "status": row["status"]}), flush=True)
    checked_bytes(compiler, options.compiler_sha256)
    receipt["status"] = "COMPILED" if all(row["status"] == "COMPILED" for row in receipt["cases"]) else "COMPILE_FAILED"
    write_json(out / "compilation-receipt.json", receipt)
    print(json.dumps({"compilationReceipt": str(out / "compilation-receipt.json"),
                      "sha256": digest((out / "compilation-receipt.json").read_bytes()), "status": receipt["status"]}))
    return 0 if receipt["status"] == "COMPILED" else 1


def finalize_manifest(options):
    out = options.out.resolve(strict=True)
    receipt_path = out / "compilation-receipt.json"
    checked_bytes(receipt_path, options.compilation_sha256)
    receipt = read_json(receipt_path)
    require(receipt["status"] == "COMPILED", "Compilation receipt is incomplete")
    checked_bytes(out / "suite-manifest.json", receipt["suiteManifestSHA256"])
    expected_cases = suite_cases(read_json(out / "suite-manifest.json"))
    require(len(receipt["cases"]) == 37, "Incomplete Legacy compilation case set")
    signed_root = options.signed_dir.resolve(strict=True)
    output = out / "native-manifest.json"
    require(not output.exists(), "Native manifest already exists; preserve prior evidence")
    cases = []
    for case, expected_case in zip(receipt["cases"], expected_cases):
        name = f"CHERRI_QA_D30_L_{expected_case['number']:02d}"
        require(case["id"] == expected_case["id"] and case["number"] == expected_case["number"]
                and case["name"] == name and case["lane"] == "fork_legacy" and case["status"] == "COMPILED",
                "Changed or reordered compilation case")
        require(case["expected"] == expected_case["expected"]["value"]
                and case.get("input") == expected_case.get("input"), "Changed runtime expectation")
        original = checked_bytes(contained(out, case["inputSource"]), case["inputSourceSHA256"])
        staged = checked_bytes(contained(out, case["source"]), case["sourceSHA256"])
        require(staged == renamed_source(original, expected_case["shortcut_names"]["fork_legacy"], name),
                "Source changed beyond its workflow name")
        checked_bytes(contained(out, case["log"]), case["logSHA256"])
        unsigned_path = contained(out, case["unsignedFile"])
        unsigned = checked_bytes(unsigned_path, case["unsignedSHA256"])
        signed_path = signed_root / (name + ".shortcut")
        signing_path = signed_root / (name + ".signing.json")
        signing = read_json(signing_path)
        require(signing["shortcutName"] == name and signing["signedFile"] == signed_path.name
                and signing["unsignedFile"] == unsigned_path.name, "Signer names do not match: " + name)
        require(signing["unsignedPlistSha256"] == case["unsignedSHA256"]
                and signing["unsignedByteSize"] == len(unsigned), "Signer input does not match compiler bytes: " + name)
        signed = checked_bytes(signed_path, signing["signedArtifactSha256"])
        require(signed[:4] == b"AEA1" and signing["headerMagic"] == "AEA1"
                and signing["signedByteSize"] == len(signed), "Signed header/size mismatch: " + name)
        require(signing["nativeActionCount"] == case["nativeActionCount"], "Signer action count changed: " + name)
        if signing.get("sourceSha256"):
            require(signing["sourceSha256"] == case["sourceSHA256"], "Signer source hash changed: " + name)
        if signing.get("compilerCommitSha"):
            require(signing["compilerCommitSha"] == receipt["declaredCompilerCommitSHA"], "Signer commit changed: " + name)
        row = {"id": case["id"], "lane": "fork_legacy", "name": name, "expected": case["expected"],
               "signedPath": str(signed_path), "signedSHA256": signing["signedArtifactSha256"],
               "unsignedPath": str(unsigned_path), "unsignedSHA256": case["unsignedSHA256"],
               "sourceSHA256": case["sourceSHA256"], "signingManifestSHA256": digest(signing_path.read_bytes()),
               "signingVerificationTier": signing["verificationTier"]}
        if case.get("input") is not None:
            row["input"] = case["input"]
        cases.append(row)
    manifest = {"schemaVersion": 1, "suiteId": receipt["suiteId"], "runtimeStatus": "NOT_RUN",
                "compilationReceiptSHA256": options.compilation_sha256,
                "declaredCompilerCommitSHA": receipt["declaredCompilerCommitSHA"],
                "compilerSHA256": receipt["compilerSHA256"], "cases": cases}
    write_json(output, manifest)
    print(json.dumps({"nativeManifest": str(output), "sha256": digest(output.read_bytes()), "cases": len(cases)}))
    return 0


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest="operation", required=True)
    compile_parser = commands.add_parser("compile", help="Compile the real final Legacy compiler; never sign or execute")
    compile_parser.add_argument("--compiler", type=Path, required=True)
    compile_parser.add_argument("--compiler-sha256", required=True)
    compile_parser.add_argument("--compiler-commit", required=True, help="Full source SHA from the separate binary build receipt")
    compile_parser.add_argument("--manifest", type=Path, default=DEFAULT_MANIFEST)
    compile_parser.add_argument("--out", type=Path, required=True)
    compile_parser.add_argument("--timeout", type=float, default=60)
    compile_parser.add_argument("--derive-uuids", action="store_true")
    finalize_parser = commands.add_parser("finalize", help="Bind existing Cherri *.signing.json receipts and signed bytes")
    finalize_parser.add_argument("--out", type=Path, required=True)
    finalize_parser.add_argument("--compilation-sha256", required=True)
    finalize_parser.add_argument("--signed-dir", type=Path, required=True)
    options = parser.parse_args()
    try:
        return compile_cases(options) if options.operation == "compile" else finalize_manifest(options)
    except (KeyError, TypeError, OSError, ValueError) as error:
        print("ERROR: " + str(error), file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
