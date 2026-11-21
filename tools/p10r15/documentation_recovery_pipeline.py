#!/usr/bin/env python3
"""Run the real documentation producers against one immutable compiler/docs/Skill tuple.

This is orchestration of the canonical generators and existing strict collectors,
not a replacement catalog, parser, or an acceptance-status writer.
"""
from __future__ import annotations
import argparse
import json
from pathlib import Path
import sys
sys.dont_write_bytecode = True
import documentation_recovery_examples as examples
import documentation_recovery_wrappers as wrappers
import package_skill

def collect(mode, compiler_source, consumer_source, docs, compiler, implementation_sha, output, bash="sh"):
    source, consumer, docs, compiler, output = map(Path, (compiler_source, consumer_source, docs, compiler, output))
    wrappers.repository_state(consumer, implementation_sha)
    for name in ("documentation_recovery_pipeline.py", "documentation_recovery_examples.py",
                 "documentation_recovery_wrappers.py", "package_skill.py"):
        relative = "tools/p10r15/" + name
        examples.bound_bytes(consumer, implementation_sha, relative)
    if Path(__file__).resolve() != (consumer / "tools/p10r15/documentation_recovery_pipeline.py").resolve():
        raise ValueError("pipeline is not executing its declared immutable source")
    pins = examples.load_json(examples.bound_bytes(consumer, implementation_sha,
                              "skills/cherri-shortcuts/compatibility-manifest.json"))
    examples.checkout(source, pins["compilerCommit"], "compiler")
    examples.checkout(docs, pins["docsCommit"], "docs")
    output.mkdir(parents=True, exist_ok=False)
    report = {"schemaVersion": 1, "mode": mode, "implementationSHA": implementation_sha,
              "tuple": {"S": pins["compilerCommit"], "D": pins["docsCommit"], "K": implementation_sha},
              "status": "RUNNING", "formalAcceptanceGranted": False, "nativeStatus": "NOT_RUN"}
    try:
        if mode == "consumers":
            example_report = examples.collect(source, consumer, docs, compiler, implementation_sha, output / "examples")
            if example_report["summary"]["executed"] != 88 or example_report["summary"]["passed"] != 88 or example_report["summary"]["failed"] != 0:
                raise ValueError("complete documentation examples did not pass")
            expected, receipt = package_skill.expected_package(consumer, implementation_sha, "source")
            archive = output / "cherri-shortcuts-source.zip"
            package_skill.create_archive(archive, expected)
            package_skill.verify_archive(archive, expected, output / "unpacked")
            wrapper_report = wrappers.collect(consumer, implementation_sha,
                output / "unpacked/cherri-shortcuts", compiler, docs, output / "wrappers", bash=bash)
            if wrapper_report["status"] != "PASS_PACKAGED_DOCUMENTATION_WRAPPERS":
                raise ValueError("packaged documentation retrieval did not pass: " + wrapper_report.get("error", "unknown"))
            report["examples"] = example_report
            report["wrappers"] = wrapper_report
            report["package"] = {"files": len(expected), "sha256": examples.digest(archive.read_bytes()),
                                 "skillCommit": receipt["skillCommit"], "compilerCommit": receipt["compilerCommit"],
                                 "docsCommit": receipt["docsCommit"]}
        elif mode == "generation":
            identity = examples.compiler_identity(compiler, pins["compilerCommit"], pins, output)
            target = output / "actions-reference.md"
            for label, command in (
                ("generate-reference", ["go", "run", "./tools/language-docs", str(target)]),
                ("check-reference", ["go", "run", "./tools/language-docs", "--check", str(target)])):
                receipt, stdout, stderr = examples.process(output, label, command, source, budget=60)
                examples.require(examples.succeeded(receipt) and not stderr, "CANONICAL_GENERATION_PROCESS_FAILED:" + label)
            generated = examples.regular(target)
            original = examples.bound_bytes(source, pins["compilerCommit"], "docs/language-v2/actions-reference.md")
            published = examples.bound_bytes(docs, pins["docsCommit"], "fork-language/actions.md")
            if generated != original or generated != wrappers.reference_body(published):
                raise ValueError("fresh full canonical reference differs from compiler or pinned docs bytes")
            report["generation"] = {"bytes": len(generated), "sha256": examples.digest(generated),
                                    "compilerIdentity": identity, "matchesCompilerBlob": True,
                                    "matchesPinnedDocsBody": True}
        else:
            raise ValueError("unknown documentation producer mode")
        wrappers.repository_state(consumer, implementation_sha)
        examples.checkout(source, pins["compilerCommit"], "compiler")
        examples.checkout(docs, pins["docsCommit"], "docs")
        report["status"] = "PASS_DOCUMENTATION_RECOVERY"
        report["inputsUnchangedAfter"] = True
    except Exception as error:
        report["status"] = "FAIL_DOCUMENTATION_RECOVERY"
        report["error"] = str(error)
        raise
    finally:
        (output / "report.json").write_bytes(wrappers.canonical(report))
    return report

def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--mode", choices=("consumers", "generation"), required=True)
    parser.add_argument("--compiler-source", type=Path, required=True)
    parser.add_argument("--consumer-source", type=Path, required=True)
    parser.add_argument("--docs", type=Path, required=True)
    parser.add_argument("--compiler", type=Path, required=True)
    parser.add_argument("--implementation-sha", required=True)
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--bash", default="sh")
    args = parser.parse_args()
    try:
        report = collect(**vars(args))
    except Exception as error:
        print("[CHERRI_DOCUMENTATION_RECOVERY_FAILED] " + str(error), file=sys.stderr)
        return 1
    print(json.dumps({"status": report["status"], "mode": args.mode,
                      "report": str(args.output / "report.json")}, sort_keys=True))
    return 0

if __name__ == "__main__":
    raise SystemExit(main())
