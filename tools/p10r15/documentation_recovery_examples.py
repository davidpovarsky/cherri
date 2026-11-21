#!/usr/bin/env python3
"""Execute exact documentation examples from immutable compiler, Skill and docs sources.

This component proves actual structured checks and unsigned plist production.
It never substitutes for the packaged-wrapper collector, native execution, or
the formal RP46/BRG29 aggregator.
"""
import argparse
import hashlib
import datetime
import math
import signal
import json
import os
from pathlib import Path
import plistlib
import re
import subprocess
import time

COMPILER_ORIGIN = "https://github.com/davidpovarsky/cherri.git"
DOCS_ORIGIN = "https://github.com/davidpovarsky/cherrilang.org.git"
COMPILER_GUIDES = ("docs/language-v2/language-guide.md",)
DOCS_GUIDES = ("index.md", "fork-language/index.md", "fork-language/guide.md",
              "fork-language/migration.md", "language/functions.md",
              "language/variables-constants-globals.md")
SKILL_GUIDES = ("skills/cherri-shortcuts/SKILL.md",
                "skills/cherri-shortcuts/references/debug-logging.md",
                "skills/cherri-shortcuts/references/workflow.md",
                "skills/cherri-shortcuts/references/openminis.md")
TICK = chr(96)
FENCES = re.compile(r"^" + TICK*3 + r"(?P<kind>cherri(?:-[a-z]+)?)\s*\r?\n(?P<code>.*?)^" + TICK*3 + r"\s*$", re.M | re.S)
EXPECTATION = re.compile(r"<!--\s*cherri-test:\s*(.*?)\s*-->\s*$")
INLINE = re.compile(r"(?<!" + TICK + r")" + TICK + r"([^" + TICK + r"\n]+)" + TICK + r"(?!" + TICK + r")")
SOURCE_ROLES = {"compiler": COMPILER_ORIGIN, "consumer": COMPILER_ORIGIN, "docs": DOCS_ORIGIN}


def require(condition, reason):
    if not condition:
        raise ValueError(reason)


def digest(data):
    return hashlib.sha256(data).hexdigest()


def unique(pairs):
    value = {}
    for key, entry in pairs:
        require(key not in value, "DUPLICATE_JSON_KEY")
        value[key] = entry
    return value


def load_json(data):
    return json.loads(data, object_pairs_hook=unique,
                      parse_constant=lambda value: (_ for _ in ()).throw(ValueError("NONFINITE_JSON")))


def regular(path, maximum=4*1024*1024):
    require(path.is_file() and not path.is_symlink()
            and not getattr(path, "is_junction", lambda: False)(), "SOURCE_FILE_MISSING_OR_LINKED")
    require(path.stat().st_size <= maximum, "SOURCE_FILE_TOO_LARGE")
    return path.read_bytes()


def git(root, *arguments):
    return subprocess.check_output(["git", "--no-replace-objects", *arguments], cwd=root, timeout=30)


def checkout(root, commit, role):
    require(re.fullmatch(r"[0-9a-f]{40}", commit), "INVALID_SOURCE_COMMIT")
    require(git(root, "rev-parse", "HEAD").decode().strip() == commit, "SOURCE_HEAD_MISMATCH")
    require(not git(root, "status", "--porcelain", "--untracked-files=all").strip(), "SOURCE_NOT_CLEAN")
    require(git(root, "remote", "get-url", "origin").decode().strip() == SOURCE_ROLES[role],
            "SOURCE_ORIGIN_MISMATCH")


def bound_bytes(root, commit, path):
    raw = regular(root / path)
    original = git(root, "show", commit + ":" + path)
    require(raw == original, "SOURCE_BYTES_DIFFER:" + path)
    return raw


def completed(result):
    budget, elapsed = result.get("budgetSeconds"), result.get("elapsedSeconds")
    return (type(budget) in (int, float) and math.isfinite(budget) and budget > 0
            and type(elapsed) in (int, float) and math.isfinite(elapsed)
            and 0 <= elapsed <= budget and result.get("timedOut") is False
            and type(result.get("exitCode")) is int)


def succeeded(result):
    return completed(result) and result["exitCode"] == 0


def stop_process_tree(child):
    if child.poll() is None:
        if os.name == "nt":
            subprocess.run(["taskkill", "/PID", str(child.pid), "/T", "/F"],
                           stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL,
                           stdin=subprocess.DEVNULL, timeout=10, check=False)
        else:
            try:
                os.killpg(child.pid, signal.SIGKILL)
            except ProcessLookupError:
                pass
    try:
        child.wait(timeout=5)
    except subprocess.TimeoutExpired:
        child.kill()
        child.wait(timeout=5)


def process(output, name, command, cwd, budget=30):
    require(re.fullmatch(r"[a-z0-9_-]+", name), "PROCESS_NAME")
    require(type(budget) in (int, float) and math.isfinite(budget) and budget > 0,
            "PROCESS_BUDGET")
    stdout, stderr = output/(name+".stdout"), output/(name+".stderr")
    require(not stdout.exists() and not stderr.exists(), "PROCESS_OUTPUT_EXISTS")
    started_utc = datetime.datetime.now(datetime.timezone.utc).isoformat()
    started = time.monotonic()
    deadline = started + budget
    child, code, timed_out, launch_error = None, None, False, None
    with stdout.open("xb") as out, stderr.open("xb") as err:
        try:
            options = ({"creationflags": subprocess.CREATE_NEW_PROCESS_GROUP} if os.name == "nt"
                       else {"start_new_session": True})
            child = subprocess.Popen(command, cwd=cwd, stdin=subprocess.DEVNULL,
                                     stdout=out, stderr=err, **options)
            remaining = deadline-time.monotonic()
            if remaining <= 0:
                timed_out = True
                stop_process_tree(child)
            else:
                try:
                    child.wait(timeout=remaining)
                except subprocess.TimeoutExpired:
                    timed_out = True
                    stop_process_tree(child)
            code = child.returncode
            if time.monotonic() > deadline:
                timed_out = True
        except OSError as error:
            launch_error = str(error)
        except BaseException:
            if child is not None:
                stop_process_tree(child)
            raise
    elapsed = time.monotonic()-started
    result = {"command": list(command), "cwd": str(cwd), "exitCode": code,
              "timedOut": timed_out, "budgetSeconds": budget,
              "elapsedSeconds": elapsed, "startedUTC": started_utc,
              "endedUTC": datetime.datetime.now(datetime.timezone.utc).isoformat(),
              "launchError": launch_error,
              "stdout": {"path": stdout.name, "bytes": stdout.stat().st_size,
                         "sha256": digest(stdout.read_bytes())},
              "stderr": {"path": stderr.name, "bytes": stderr.stat().st_size,
                         "sha256": digest(stderr.read_bytes())}}
    (output/(name+".process.json")).write_bytes(
        (json.dumps(result, indent=2)+"\n").encode("utf-8"))
    return result, stdout.read_bytes(), stderr.read_bytes()


def documentation_blocks(text):
    blocks, active, offset = [], None, 0
    for line_number, line in enumerate(text.splitlines(keepends=True), 1):
        raw = line.rstrip("\r\n")
        opener = re.fullmatch(r" {0,3}([" + TICK + r"~]{3,})([^\r\n]*)", raw)
        if active is None:
            if opener and len(set(opener[1])) == 1:
                info = opener[2].strip()
                active = {"start": offset, "codeStart": offset+len(line),
                          "kind": info, "delimiter": opener[1],
                          "lineStart": line_number+1}
        else:
            delimiter = active["delimiter"]
            close = re.fullmatch(r" {0,3}" + re.escape(delimiter[0]) +
                                 "{"+str(len(delimiter)) + r",}\s*", raw)
            if close:
                active.update(end=offset+len(line), code=text[active["codeStart"]:offset])
                blocks.append(active)
                active = None
        offset += len(line)
    require(active is None, "UNCLOSED_DOCUMENTATION_FENCE")
    return blocks


def fence_examples(text, role, path):
    cases, used_annotations = [], set()
    blocks = documentation_blocks(text)
    for block in blocks:
        kind = block["kind"]
        if not kind.startswith("cherri"):
            continue
        require(re.fullmatch(r"cherri(?:-[a-z]+)?", kind), "UNSUPPORTED_DOCUMENTATION_FENCE")
        code = block["code"]
        stripped = code.strip()
        require(bool(stripped), "EMPTY_DOCUMENTATION_EXAMPLE")
        prefix, errors, mode = "", [], "v2"
        annotation = EXPECTATION.search(text[:block["start"]])
        if annotation:
            fields = annotation[1].split()
            require(len(fields) >= 2 and fields[0] == "diagnostic"
                    and all(re.fullmatch(r"E_[A-Z0-9_]+", value) for value in fields[1:])
                    and len(fields[1:]) == len(set(fields[1:])),
                    "INVALID_DIAGNOSTIC_ANNOTATION")
            errors = fields[1:]
            used_annotations.add(annotation.start())
        if kind == "cherri-legacy":
            require(not errors, "LEGACY_ANNOTATION_AMBIGUOUS")
            mode = "legacy"
            if path == "fork-language/migration.md" and stripped == "resizeImage(@photo, 800, 600)":
                prefix = "@photo = selectPhotos()\n"
            elif path == "fork-language/migration.md" and stripped == "@first = getListItem(@list, 1) // 1-based action index":
                prefix = '@list = ["first", "second"]\n'
        else:
            require(kind == "cherri", "UNIMPLEMENTED_CONTEXTUAL_FENCE:" + kind)
        suffix = "" if code.endswith("\n") else "\n"
        cases.append({"repository": role, "path": path, "ordinal": len(cases)+1,
                      "location": "fence", "lineStart": block["lineStart"], "mode": mode,
                      "classification": "intended-error" if errors else
                          ("historical-context" if mode == "legacy" else "runnable"),
                      "originalSnippet": code, "originalSnippetSHA256": digest(code.encode("utf-8")),
                      "contextPrefix": prefix, "contextSuffix": suffix,
                      "expectedErrors": errors, "source": prefix+code+suffix})
    for annotation in re.finditer(r"<!--\s*cherri-test:.*?-->", text, re.S):
        if any(block["start"] <= annotation.start() < block["end"] for block in blocks):
            continue
        require(annotation.start() in used_annotations, "ORPHAN_DOCUMENTATION_EXPECTATION")
    return cases


def skill_examples(text, path):
    # Keep every inline token in the inventory. Only complete calls and the
    # explicit expression contexts below are compiler examples.
    body = list(text)
    for block in documentation_blocks(text):
        body[block["start"]:block["end"]] = ["\n" if c == "\n" else " " for c in text[block["start"]:block["end"]]]
    body = "".join(body)
    cases, inventory = [], []
    for ordinal, match in enumerate(INLINE.finditer(body), 1):
        code = match[1]
        heading = body[:match.start()].rsplit("\n## ", 1)[-1].splitlines()[0]
        row = {"path": path, "ordinal": ordinal, "heading": heading,
               "source": code, "sha256": digest(code.encode()), "classification": "reference-token"}
        inventory.append(row)
        prefix, suffix, mode = 'text("documentation-context")\n', "\n", "v2"
        if code == 'f"Hello {name}"':
            prefix = 'let name = "documentation-context"\nlet result = '
        elif code == "list[0]":
            prefix = 'let list = ["first", "second"]\nlet result = '
        elif code == "resizeImage(photo, width: 800)":
            prefix = "let photo = selectPhotos()\n"
        elif re.fullmatch(r"debug(?:Log|LogPrevious|TraceEvent|ExportTrace)\(.*\)", code):
            if heading == "Legacy calls":
                mode = "legacy"
        elif "(" in code and ("..." in code or "{...}" in code):
            row["classification"] = "placeholder-reference"
            continue
        else:
            continue
        variants = [code]
        if '"before|after|error|end"' in code:
            variants = [code.replace('"before|after|error|end"', json.dumps(stage))
                        for stage in ("before", "after", "error", "end")]
        row["classification"] = "contextual-example"
        for index, variant in enumerate(variants):
            cases.append({"repository": "consumer", "path": path, "ordinal": ordinal,
                          "location": "inline", "heading": heading, "mode": mode,
                          "classification": "historical-context" if mode == "legacy" else "contextual",
                          "originalSnippet": code, "originalSnippetSHA256": digest(code.encode()),
                          "contextPrefix": prefix, "contextSuffix": suffix,
                          "expandedDocumentedAlternative": variant if variant != code else None,
                          "variant": index, "expectedErrors": [], "source": prefix+variant+suffix})
    return cases, inventory


def compiler_identity(binary, expected, pins, output, go="go"):
    require(re.fullmatch(r"[0-9a-f]{40}", expected), "INVALID_COMPILER_COMMIT")
    before = regular(binary, 128*1024*1024)
    info, metadata, info_stderr = process(output, "compiler-build-info", [str(go), "version", "-m", str(binary)], output)
    require(succeeded(info) and not info_stderr, "COMPILER_METADATA_FAILED")
    text = metadata.decode("utf-8", errors="strict")
    paths = re.findall(r"^\s*path\s+(\S+)\s*$", text, re.M)
    require(paths == ["github.com/electrikmilk/cherri"], "COMPILER_ROLE")
    build = {}
    for line in re.findall(r"^\s*build\s+(.+?)\s*$", text, re.M):
        key, separator, value = line.partition("=")
        require(separator and key not in build, "COMPILER_BUILD_INFO_AMBIGUOUS")
        build[key] = value
    require(build.get("vcs") == "git" and build.get("vcs.revision") == expected
            and build.get("vcs.modified") == "false", "COMPILER_VCS_IDENTITY")
    require(all(isinstance(build.get(k), str) and re.fullmatch(r"[a-z0-9]+", build[k])
                for k in ("GOOS", "GOARCH")), "COMPILER_PLATFORM")
    cap_process, raw, cap_stderr = process(output, "compiler-capabilities", [str(binary), "--capabilities-json"], output)
    require(succeeded(cap_process) and not cap_stderr, "CAPABILITIES_FAILED")
    caps = load_json(raw)
    require(type(caps) is dict and caps.get("protocolVersion") == "1.0", "CAPABILITIES_PROTOCOL")
    for key in ("languageVersion", "catalogSchemaVersion",
                "schemaFingerprint", "codecFingerprint", "macroFingerprint"):
        require(type(pins.get(key)) is str and type(caps.get(key)) is str
                and caps[key] == pins[key], "CAPABILITIES_PIN:" + key)
        if key.endswith("Fingerprint"):
            require(re.fullmatch(r"[0-9a-f]{64}", caps[key]), "CAPABILITIES_DIGEST:" + key)
    if "compilerCommit" in caps:
        require(caps["compilerCommit"] == expected, "CAPABILITIES_SOURCE")
    require(regular(binary, 128*1024*1024) == before, "COMPILER_BYTES_CHANGED")
    return {"path": str(binary), "sha256": digest(before), "bytes": len(before),
            "commit": expected, "role": paths[0], "goos": build["GOOS"], "goarch": build["GOARCH"],
            "protocolVersion": caps["protocolVersion"], "buildInfo": info,
            "capabilitiesProcess": cap_process}


def diagnostic_errors(raw):
    diagnostic = load_json(raw)
    require(type(diagnostic) is dict and type(diagnostic.get("valid")) is bool, "CHECK_SCHEMA")
    require("diagnostics" in diagnostic, "DIAGNOSTICS_SCHEMA")
    values = diagnostic["diagnostics"]
    # protocol.DocumentAnalysis serializes an empty nil Go slice as JSON null.
    # Only the explicit valid/no-errors response may use that exact wire value.
    if values is None:
        require(diagnostic["valid"] is True, "NULL_INVALID_DIAGNOSTICS")
        values = []
    require(type(values) is list, "DIAGNOSTICS_SCHEMA")
    for item in values:
        require(type(item) is dict and item.get("severity") in ("ERROR", "WARNING", "INFO", "HINT")
                and type(item.get("code")) is str and re.fullmatch(r"[A-Z][A-Z0-9_]+", item["code"])
                and type(item.get("message")) is str, "DIAGNOSTIC_ENTRY_SCHEMA")
    return diagnostic, [item["code"] for item in values if item["severity"] == "ERROR"]


def execute_example(binary, case, directory):
    directory.mkdir()
    source = directory/"snippet.cherri"
    source.write_bytes(case["source"].encode("utf-8"))
    case = dict(case, sourceSHA256=digest(source.read_bytes()), artifact=None)
    failures = []
    if case["mode"] == "v2":
        checked, raw, _ = process(directory, "check",
                                 [str(binary), "check", str(source), "--json", "--no-ansi"], directory)
        case["check"] = checked
        try:
            require(completed(checked), "CHECK_DEADLINE_OR_PROCESS")
            diagnostic, errors = diagnostic_errors(raw)
            case["actualErrors"] = errors
            if case["expectedErrors"]:
                require(checked["exitCode"] != 0 and diagnostic["valid"] is False
                        and errors == case["expectedErrors"], "INTENDED_ERROR_MISMATCH")
            else:
                require(checked["exitCode"] == 0 and diagnostic["valid"] is True and not errors,
                        "RUNNABLE_CHECK_FAILED")
        except (ValueError, KeyError, TypeError, UnicodeError) as error:
            failures.append("check:" + str(error))
    command = [str(binary), str(source), "--skip-sign", "--no-ansi"]
    if case["mode"] == "legacy":
        command = [str(binary), "compile", str(source), "--mode=legacy",
                   "--load-all-actions", "--skip-sign", "--no-ansi"]
    compiled, compile_stdout, compile_stderr = process(directory, "compile", command, directory)
    case["compile"] = compiled
    artifact = directory/"snippet_unsigned.shortcut"
    try:
        require(completed(compiled), "COMPILE_DEADLINE_OR_PROCESS")
        if case["expectedErrors"]:
            require(compiled["exitCode"] != 0 and not artifact.exists(),
                    "INVALID_EXAMPLE_EMITTED_OUTPUT")
            codes = re.findall(r"\bE_[A-Z0-9_]+\b",
                               (compile_stdout+b"\n"+compile_stderr).decode("utf-8", errors="strict"))
            case["compileErrors"] = list(dict.fromkeys(codes))
            require(case["compileErrors"] == case["expectedErrors"], "INTENDED_COMPILE_ERROR_MISMATCH")
        else:
            require(compiled["exitCode"] == 0, "RUNNABLE_COMPILE_FAILED")
            raw = regular(artifact, 16*1024*1024)
            workflow = plistlib.loads(raw)
            require(type(workflow) is dict and type(workflow.get("WFWorkflowActions")) is list,
                    "NOT_NATIVE_WORKFLOW")
            require(all(type(row) is dict and type(row.get("WFWorkflowActionIdentifier")) is str
                        and row["WFWorkflowActionIdentifier"] for row in workflow["WFWorkflowActions"]),
                    "NATIVE_ACTION_SCHEMA")
            case["artifact"] = {"path": artifact.name, "bytes": len(raw),
                                "sha256": digest(raw), "actionCount": len(workflow["WFWorkflowActions"])}
    except (ValueError, OSError, UnicodeError, plistlib.InvalidFileException) as error:
        failures.append("compile:" + str(error))
    case.update(passed=not failures, failures=failures)
    (directory/"example.json").write_bytes(
        (json.dumps(case, indent=2, ensure_ascii=False)+"\n").encode("utf-8"))
    return case


def collect(compiler_source, consumer_source, docs, binary, implementation, output):
    compiler_source, consumer_source, docs, binary, output = (
        Path(value).absolute() for value in (compiler_source, consumer_source, docs, binary, output))
    require(not output.exists(), "OUTPUT_EXISTS")
    checkout(consumer_source, implementation, "consumer")
    manifest_path = "skills/cherri-shortcuts/compatibility-manifest.json"
    manifest_raw = bound_bytes(consumer_source, implementation, manifest_path)
    pins = load_json(manifest_raw)
    require(pins.get("compilerRepo") == COMPILER_ORIGIN and pins.get("docsRepo") == DOCS_ORIGIN, "PIN_ORIGINS")
    checkout(compiler_source, pins["compilerCommit"], "compiler")
    checkout(docs, pins["docsCommit"], "docs")
    output.mkdir(parents=True)
    identity = compiler_identity(binary, pins["compilerCommit"], pins, output)
    sources, cases, inline_inventory = [], [], []
    sets = ((compiler_source, pins["compilerCommit"], "compiler", COMPILER_GUIDES),
            (docs, pins["docsCommit"], "docs", DOCS_GUIDES),
            (consumer_source, implementation, "consumer", SKILL_GUIDES))
    for root, commit, role, paths in sets:
        for path in paths:
            raw = bound_bytes(root, commit, path)
            sources.append({"repository": role, "commit": commit, "path": path,
                            "bytes": len(raw), "sha256": digest(raw)})
            text = raw.decode("utf-8")
            cases += fence_examples(text, role, path)
            if role == "consumer":
                examples, tokens = skill_examples(text, path)
                cases += examples
                inline_inventory += tokens
    require(any(c["repository"] == "compiler" for c in cases)
            and any(c["repository"] == "docs" for c in cases)
            and any(c["repository"] == "consumer" for c in cases), "MISSING_REPOSITORY_EXAMPLES")
    require(any(c["expectedErrors"] for c in cases)
            and any(c["mode"] == "legacy" for c in cases)
            and any(c["classification"] == "contextual" for c in cases), "MISSING_REQUIRED_EXAMPLE_CLASS")
    results = [execute_example(binary, case, output/("example-%04d"%index))
               for index, case in enumerate(cases)]
    for root, commit, role, paths in sets:
        checkout(root, commit, role)
        for path in paths:
            expected = next(x for x in sources if x["repository"] == role and x["path"] == path)
            require(digest(bound_bytes(root, commit, path)) == expected["sha256"], "SOURCE_CHANGED_DURING_RUN")
    report = {"formatVersion": 1, "implementationSha": implementation,
              "compilerCommit": pins["compilerCommit"], "docsCommit": pins["docsCommit"],
              "manifestSha256": digest(manifest_raw), "compiler": identity, "sources": sources,
              "examples": results, "inlineInventory": inline_inventory,
              "summary": {"executed": len(results), "passed": sum(x["passed"] for x in results),
                          "failed": sum(not x["passed"] for x in results),
                          "intendedErrorExamples": sum(bool(x["expectedErrors"]) for x in results),
                          "explicitLegacyExamples": sum(x["mode"] == "legacy" for x in results),
                          "contextualExamples": sum(x["classification"] == "contextual" for x in results)},
              "scope": "ACTUAL_DOCS_AND_CONTEXTUAL_SKILL_CHECKS_AND_UNSIGNED_COMPILATION",
              "generatedSignaturesVerified": False, "packagedWrappersVerified": False,
              "nativeExecution": "NOT_RUN", "formalAcceptanceGranted": False}
    (output/"documentation-examples.json").write_text(json.dumps(report, indent=2, ensure_ascii=False)+"\n", encoding="utf-8")
    return report


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ("compiler-source", "consumer-source", "docs", "compiler", "output"):
        parser.add_argument("--"+name, required=True, type=Path)
    parser.add_argument("--implementation-sha", required=True)
    args = parser.parse_args()
    report = collect(args.compiler_source, args.consumer_source, args.docs, args.compiler,
                     args.implementation_sha, args.output)
    print(json.dumps({"summary": report["summary"], "formalAcceptanceGranted": False}))
    return 0 if report["summary"]["failed"] == 0 else 1


if __name__ == "__main__":
    raise SystemExit(main())
