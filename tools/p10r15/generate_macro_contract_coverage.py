#!/usr/bin/env python3
"""Generate source-bound evidence for the eleven existing Asana macro children.

This is a requirements/evidence projection, not an action or macro registry.
Descriptors come from the real compiler. Synthetic CLI compiles remain distinct
from Apple runtime proof. No private shortcut corpus or signing service is used.
"""
import argparse
import collections
import copy
import datetime
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import plistlib
import re
import subprocess
import sys
import tempfile

sys.dont_write_bytecode = True
ROOT = Path(__file__).resolve().parents[2]
GENERATOR = Path(__file__).resolve()
MASTER = "1220544599390563"
NATIVE_PREFIX = "is.workflow.actions."
MARKER = "P15 ledger original \u05e9\u05dc\u05d5\u05dd \U0001f352"

# Requirements and evidence links only. Callable signatures are never defined here.
REQUIREMENTS = (
    ("require", "1220544599477512", "Typed require", ("require",),
     "Return the original typed object after semantic validation; failure stops with the supplied message.",
     "PARTIAL_IMPLEMENTATION", (),
     ("Full false/zero/null/empty/native-object matrix and native failure/cancel proof are not established by this ledger.",)),
    ("label", "1220544718902445", "debugLog labels", ("debugLog",),
     "Stable per-entry labels without replacing the original raw object or its downstream UUID.",
     "PARTIAL_IMPLEMENTATION", ("Native file/object identity and six-mode label acceptance",),
     ("Per-entry metadata and bounded source contracts are implemented; signed File-carrier byte identity still requires repair and acceptance.",)),
    ("export", "1220544665232213", "Local debugExport", ("debugExport",),
     "Local returned file; typed JSON/text or explicit opaque placeholders; safe escaping and default redaction.",
     "PARTIAL_IMPLEMENTATION", ("Apple returned-file/UTF-8/cardinality runtime evidence",),
     ("Local File export source supports default full redaction and explicit Text-only base64 payloads; structured types, dates/labels, selective redaction and native acceptance remain open. QA debugExportTrace is separate.",)),
    ("blocks", "1220544718901241", "Shared typed block facility", ("measure",),
     "Scoped typed block/IR, one evaluation, shadowing, value/void, nesting and explicit unavailable-dialect diagnostics.",
     "PARTIAL_IMPLEMENTATION", (),
     ("V2 and Legacy share the once-body/typed-result measure policy. General parameterized/void/early-error blocks and Legacy control-result declarations remain open.",)),
    ("measure", "1220544665232149", "Measure typed value", ("measure",),
     "Evaluate the body once, preserve its native result and remove only timing instrumentation in Release.",
     "PARTIAL_IMPLEMENTATION", ("Apple timing accuracy and error/cancel observation",),
     ("Legacy uses a final yield or value-producing call through existing structured tokens. Structural UUID/timer checks are not native elapsed-time, abort or cancellation proof.",)),
    ("collect", "1220544651247337", "Typed collection", (),
     "Use existing native Repeat Results without a second accumulator or standalone alias; account separately for typed input construction.",
     "EXISTING_V2_CONSTRUCT", ("Native empty/mixed/nested collection shape and performance observation",),
     ("No standalone collect alias is introduced. Compiled Repeat Results does not prove runtime flattening behavior.",)),
    ("cache", "1220544665232085", "Persistent TTL cache", ("cache",),
     "Explicit namespace/scope/TTL and honest serialization, invalidation, clock and permission behavior.",
     "IMPLEMENTATION_GAP_AND_NATIVE_PREREQUISITE", ("Stored Content cross-run/restart/global-local readback", "Physical iPad persistence/background proof"),
     ("Existing storage definitions are prerequisites, not a cache implementation; current storeValue exposes text.",)),
    ("retry", "1220544651247273", "Bounded retry", ("retry",),
     "Bounded attempts/backoff with actual transport/status/permission/cancel classification and idempotence safeguards.",
     "IMPLEMENTATION_GAP_AND_NATIVE_PREREQUISITE", ("Recoverable native error/status contract and signed synthetic endpoint probes",),
     ("No evidence establishes catchable transport failures; Alert plus Stop Shortcut is not exception handling.",)),
    ("paginate", "1220544665232021", "Bounded pagination", ("paginate",),
     "Explicit limits/cursor termination, typed aggregation and malformed/error/rate-limit/credential safeguards.",
     "BOUNDED_GET_IMPLEMENTATION", ("Signed synthetic multi-page HTTP/schema/error observation",),
     ("The shared bounded GET/body-cursor implementation has page/item limits and cycle guards. Response headers/status, transport recovery, redirect limits and generic retry remain outside its proven contract.",)),
    ("profiles", "1220544607463126", "Debug and Release", ("debugLog", "require", "measure"),
     "Compile-time diagnostic removal while retaining semantic require checks and original native producers.",
     "PARTIAL_IMPLEMENTATION", ("Native result equivalence and debug-mode acceptance",),
     ("The controlled compiler comparisons cover declared fixtures; they do not establish every diagnostic or runtime path.",)),
    ("qa", "1220544718902349", "Macro QA and consumers", (),
     "Generated evidence, canonical metadata/docs and coherent Skill/iOS/remote consumer acceptance.",
     "PARTIAL_EVIDENCE", ("Final pinned consumer tuple and signed/physical acceptance evidence",),
     ("This ledger builds and tests a compiler; it does not install a Skill, update a provider or execute on Apple.",)),
)


def sha(data):
    return hashlib.sha256(data).hexdigest()


def canonical(value):
    return json.dumps(value, ensure_ascii=False, sort_keys=True, separators=(",", ":")).encode("utf-8")


def read_json(path):
    return json.loads(path.read_text(encoding="utf-8"))


def write_json(path, value):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_bytes((json.dumps(value, ensure_ascii=False, indent=2) + "\n").encode("utf-8"))


def input_fingerprint(repo):
    # Reuse the existing catalog coverage source scope, including all root Go,
    # internal Go, actions and compiler test fixtures. Add this producer and the
    # canonical schema/docs generators and independent native metadata.
    tools_path = repo / "tools/p10r3"
    sys.path.insert(0, str(tools_path))
    try:
        spec = importlib.util.spec_from_file_location("cherri_action_contract_source", tools_path / "generate_action_contract_coverage.py")
        module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(module)
        base = module.source_fingerprint(repo)
    finally:
        sys.path.pop(0)
    extra = {}
    for pattern in ("tools/language-schema/*.go", "tools/language-docs/*.go"):
        for path in repo.glob(pattern):
            if path.is_file():
                extra[path.relative_to(repo).as_posix()] = sha(path.read_bytes().replace(b"\r\n", b"\n"))
    native = repo / "docs/apple-runtime/macos-27/builtin-actions.json"
    extra[native.relative_to(repo).as_posix()] = sha(native.read_bytes().replace(b"\r\n", b"\n"))
    extra["generator"] = sha(GENERATOR.read_bytes().replace(b"\r\n", b"\n"))
    return sha(canonical({"existingSourceFingerprint": base, "additionalInputs": extra}))


def run(args, repo, timeout=90, required=True, env=None):
    proc = subprocess.run([str(a) for a in args], cwd=repo, env=env,
                          text=True, encoding="utf-8", errors="replace",
                          stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=timeout)
    if required and proc.returncode:
        raise RuntimeError("COMMAND_FAILED: " + str(args[0]) + "\n" + (proc.stdout + proc.stderr)[-2500:])
    return proc


def action_id(action):
    return action.get("WFWorkflowActionIdentifier", "")


def params(action):
    return action.get("WFWorkflowActionParameters", {})


def named(actions, suffix):
    return [a for a in actions if action_id(a) == NATIVE_PREFIX + suffix]


def output_ref(value):
    if not isinstance(value, dict):
        return ""
    body = value.get("Value", value)
    if not isinstance(body, dict) or body.get("Type") != "ActionOutput" or body.get("Aggrandizements"):
        return ""
    return body.get("OutputUUID", "")


def final_consumer_index(actions, dialect):
    # These fixtures end with a getVariable expression and have no explicit
    # return. V2 emits exactly one bare Nothing for implicit void completion;
    # Legacy keeps the final consumer as its last action. Do not discard or
    # normalize any action: malformed/extra terminators must still fail.
    if dialect == "v2":
        if len(actions) < 2 or actions[-1] != {"WFWorkflowActionIdentifier": NATIVE_PREFIX + "nothing"}:
            return None
        index = len(actions) - 2
    elif dialect == "legacy":
        index = len(actions) - 1
    else:
        return None
    if index < 0 or action_id(actions[index]) != NATIVE_PREFIX + "getvariable":
        return None
    return index


def origin_matches(actions, producer, dialect):
    consumer_index = final_consumer_index(actions, dialect)
    if consumer_index is None:
        return False
    producers = {}
    for index, action in enumerate(actions):
        identity = params(action).get("UUID")
        if identity:
            if identity in producers:
                return False
            producers[identity] = (index, action)
    current = output_ref(params(actions[consumer_index]).get("WFVariable"))
    seen = set()
    while current:
        if current in seen or current not in producers:
            return False
        seen.add(current)
        index, action = producers[current]
        if index >= consumer_index:
            return False
        if current == producer:
            return bool(producer)
        if action_id(action) != NATIVE_PREFIX + "getvariable":
            return False
        consumer_index = index
        current = output_ref(params(action).get("WFVariable"))
    return False



def structural_checks(kind, profile, actions, dialect):
    checks = {}
    if kind == "require":
        originals = [a for a in named(actions, "gettext") if params(a).get("WFTextActionText") == MARKER]
        checks["oneOriginalProducer"] = len(originals) == 1
        checks["originalUUIDPassthrough"] = len(originals) == 1 and origin_matches(actions, params(originals[0]).get("UUID"), dialect)
        checks["canonicalGetType"] = len(named(actions, "getitemtype")) >= 2 and not named(actions, "gettypeaction")
        checks["semanticGuardRetained"] = all(named(actions, suffix) for suffix in ("conditional", "alert", "exit", "count"))
        checks["diagnosticProfile"] = bool(named(actions, "appendvariable")) if profile == "debug" else not named(actions, "appendvariable")
    elif kind == "measure":
        producers = named(actions, "dictionary")
        checks["oneTypedBodyProducer"] = len(producers) == 1
        checks["originalUUIDPassthrough"] = len(producers) == 1 and origin_matches(actions, params(producers[0]).get("UUID"), dialect)
        checks["noTextNumberCoercion"] = not named(actions, "gettext") and not named(actions, "number")
        checks["timerProfile"] = len(named(actions, "gettimebetweendates")) == (1 if profile == "debug" else 0)
        checks["releaseNoDiagnosticAppend"] = profile != "release" or not named(actions, "appendvariable")
    elif kind == "collect":
        loops = named(actions, "repeat.each")
        checks["pairedRepeatRegion"] = (len(loops) == 2 and
            params(loops[0]).get("GroupingIdentifier") == params(loops[1]).get("GroupingIdentifier") and
            str(params(loops[0]).get("WFControlFlowMode")) == "0" and str(params(loops[1]).get("WFControlFlowMode")) == "2")
        checks["consumerUsesRepeatResults"] = len(loops) == 2 and origin_matches(actions, params(loops[1]).get("UUID"), dialect)
        checks["noAccumulator"] = not named(actions, "appendvariable")
    elif kind == "paginate":
        requests = named(actions, "downloadurl")
        loops = [a for a in named(actions, "repeat.count") if str(params(a).get("WFControlFlowMode")) == "0"]
        checks["oneGETNode"] = len(requests) == 1 and params(requests[0]).get("WFHTTPMethod") == "GET"
        checks["explicitPageBudget"] = len(loops) == 1 and str(params(loops[0]).get("WFRepeatCount")) == "3"
        checks["nativeCycleHash"] = bool(named(actions, "hash"))
        checks["typedDictionaryWitness"] = bool(named(actions, "getitemtype")) and bool(named(actions, "getvalueforkey"))
        checks["semanticFailureStops"] = bool(named(actions, "alert")) and bool(named(actions, "exit"))
    elif kind == "paginate-zero":
        checks["zeroBudgetNoGET"] = not named(actions, "downloadurl")
        checks["zeroBudgetNoLoop"] = not named(actions, "repeat.count") and not named(actions, "repeat.each")
        checks["explicitEmptyResult"] = any(params(a).get("WFItems") == [] for a in named(actions, "list"))
    return {key: bool(value) for key, value in checks.items()}


def fixture_specs():
    result = []
    for dialect in ("v2", "legacy"):
        prefix = "let" if dialect == "v2" else "const"
        arguments = 'original, type: "text", nonEmpty: true, message: "P15 semantic guard"' if dialect == "v2" else 'original, "text", true, "P15 semantic guard"'
        source = (prefix + " original = text(" + json.dumps(MARKER, ensure_ascii=False) + ")\n"
                  "debugLog()\n" + prefix + " checked = require(" + arguments + ")\ngetVariable(checked)\n")
        for profile in ("debug", "release"):
            result.append({"id": "require_" + dialect + "_" + profile, "kind": "require",
                           "dialect": dialect, "profile": profile, "source": source})
    measure = ('let result = measure("P15 typed \u05e9\u05dc\u05d5\u05dd") {\n'
               '    let payload = {"flag": false, "nested": [1, {"name": "typed"}]}\n'
               '    payload\n}\ngetVariable(result)\n')
    for dialect in ("v2", "legacy"):
        body = measure if dialect == "v2" else measure.replace("let ", "const ").replace("    payload\n", "    yield payload\n")
        for profile in ("debug", "release"):
            result.append({"id": "measure_" + dialect + "_" + profile, "kind": "measure", "dialect": dialect,
                           "profile": profile, "source": body})
    for size in (0, 1, 100):
        source = "let result = for item in [" + ",".join(str(i) for i in range(size)) + "] { yield item }\ngetVariable(result)\n"
        result.append({"id": "collect_v2_" + str(size), "kind": "collect", "dialect": "v2",
                       "profile": "debug", "source": source})
    result.append({"id": "collect_v2_dictionary", "kind": "collect", "dialect": "v2",
        "profile": "debug", "source": 'let result = for item in [1, 2] { yield {"name": "typed", "flag": false} }\ngetVariable(result)\n'})
    result.extend((
        {"id": "measure_legacy_empty_rejected", "kind": "negative", "dialect": "legacy", "profile": "debug",
         "source": 'measure("empty") {}\n', "diagnostic": "E_BLOCK_VALUE_REQUIRED"},
        {"id": "measure_legacy_control_value_rejected", "kind": "negative", "dialect": "legacy", "profile": "debug",
         "source": 'measure("control") {\nconst value = if 1 == 1 {\ntext("body")\n}\nyield value\n}\n', "diagnostic": "E_BLOCK_LEGACY_CONTROL_VALUE"},
        {"id": "measure_v2_empty_rejected", "kind": "negative", "dialect": "v2", "profile": "debug",
         "source": 'let result = measure("empty") {}\n', "diagnostic": "E_BLOCK_VALUE_REQUIRED"},
    ))
    for dialect in ("v2", "legacy"):
        for kind, pages in (("paginate", 3), ("paginate-zero", 0)):
            result.append({"id": kind + "_" + dialect, "kind": kind, "dialect": dialect,
                           "profile": "debug", "source": 'paginate("https://example.invalid/p15/entry.json", "items", "next", ' + str(pages) + ', 6)\n'})
    return result


def compile_case(spec, binary, repo, work):
    folder = work / spec["id"]
    folder.mkdir()
    source = folder / "source.cherri"
    target = folder / "output.shortcut"
    source.write_bytes(spec["source"].encode("utf-8"))
    args = [binary, "compile", source, "--output=" + str(target), "--skip-sign",
            "--mode=" + spec["dialect"], "--build-profile=" + spec["profile"], "--load-all-actions", "--no-ansi"]
    process = run(args, repo, required=False)
    log = (process.stdout + process.stderr).replace(str(work), "<synthetic-fixtures>")
    row = {"id": spec["id"], "kind": spec["kind"], "dialect": spec["dialect"], "profile": spec["profile"],
           "source": spec["source"], "sourceSHA256": sha(spec["source"].encode("utf-8")),
           "command": ["<freshly-built-compiler>"] + [str(a).replace(str(work), "<synthetic-fixtures>") for a in args[1:]],
           "exitCode": process.returncode, "diagnosticTail": log[-1200:],
           "signedRuntime": "NOT_EXECUTED", "physicalDevice": "NOT_EXECUTED"}
    if spec["kind"] == "negative":
        row["expectedDiagnostic"] = spec["diagnostic"]
        row["noArtifact"] = not target.exists()
        row["verdict"] = "PASS" if process.returncode != 0 and spec["diagnostic"] in log and not target.exists() else "FAIL"
        return row, None
    if process.returncode != 0 or not target.exists():
        row["verdict"] = "FAIL"
        row["artifact"] = "MISSING"
        return row, None
    native = plistlib.loads(target.read_bytes())
    actions = native.get("WFWorkflowActions")
    if not isinstance(actions, list):
        raise ValueError("MISSING_NATIVE_ACTIONS:" + spec["id"])
    checks = structural_checks(spec["kind"], spec["profile"], actions, spec["dialect"])
    row.update(unsignedSHA256=sha(target.read_bytes()), nativeActionCount=len(actions),
               nativeActionIDs=dict(sorted(collections.Counter(action_id(a) for a in actions).items())),
               structuralChecks=checks)
    imported = folder / "imported.cherri"
    import_run = run([binary, "import", target, "--output=" + str(imported),
                      "--mode=v2", "--fidelity=lossless", "--no-ansi"], repo, required=False)
    rebuilt = folder / "rebuilt.shortcut"
    recompile_exit = None
    exact = False
    if import_run.returncode == 0 and imported.exists():
        compiled = run([binary, "compile", imported, "--output=" + str(rebuilt),
                        "--skip-sign", "--mode=v2", "--build-profile=debug", "--no-ansi"], repo, required=False)
        recompile_exit = compiled.returncode
        if compiled.returncode == 0 and rebuilt.exists():
            exact = plistlib.loads(rebuilt.read_bytes()).get("WFWorkflowActions") == actions
    row["decompile"] = {"mode": "V2_LOSSLESS_EXPANDED_NATIVE", "importExitCode": import_run.returncode,
                        "recompileExitCode": recompile_exit, "nativeActionsExact": exact,
                        "macroSourceReconstruction": "NOT_CLAIMED"}
    row["verdict"] = "PASS" if all(checks.values()) and exact else "FAIL"
    return row, actions


MUTATION_SCENARIOS = (
        ("require_v2_debug", "wrong_type_action", "canonicalGetType"),
        ("require_v2_release", "lost_output_uuid", "originalUUIDPassthrough"),
        ("measure_v2_debug", "duplicate_body", "oneTypedBodyProducer"),
        ("measure_v2_release", "retained_release_timer", "timerProfile"),
        ("measure_legacy_debug", "scoped_alias_cycle", "originalUUIDPassthrough"),
        ("measure_legacy_release", "scoped_result_coercion", "originalUUIDPassthrough"),
        ("collect_v2_1", "wrong_repeat_result", "consumerUsesRepeatResults"),
        ("paginate_v2", "page_budget_changed", "explicitPageBudget"),
        ("require_v2_release", "extra_void_terminator", "originalUUIDPassthrough"),
        ("require_v2_release", "malformed_void_terminator", "originalUUIDPassthrough"),
        ("require_v2_release", "missing_void_terminator", "originalUUIDPassthrough"),
        ("measure_v2_release", "wrong_final_consumer", "originalUUIDPassthrough"),
        ("require_v2_release", "original_uuid_replaced", "originalUUIDPassthrough"),
        ("require_legacy_release", "legacy_void_terminator", "originalUUIDPassthrough"),
)


def check_mutations(outputs):
    rows = []
    specs = {s["id"]: s for s in fixture_specs()}
    for case_id, mutation, expected in MUTATION_SCENARIOS:
        actual = outputs.get(case_id)
        if actual is None:
            rows.append({"case": case_id, "mutation": mutation, "verdict": "FAIL", "reason": "positive compile missing"})
            continue
        changed = copy.deepcopy(actual)
        spec = specs[case_id]
        consumer = final_consumer_index(changed, spec["dialect"])
        if mutation == "wrong_type_action":
            named(changed, "getitemtype")[0]["WFWorkflowActionIdentifier"] = NATIVE_PREFIX + "gettypeaction"
        elif mutation in ("lost_output_uuid", "wrong_repeat_result"):
            params(changed[consumer])["WFVariable"] = {"Value": {"Type": "ActionOutput", "OutputUUID": "missing-producer"}, "WFSerializationType": "WFTextTokenAttachment"}
        elif mutation == "duplicate_body":
            changed.insert(0, copy.deepcopy(named(changed, "dictionary")[0]))
        elif mutation == "scoped_alias_cycle":
            alias = named(changed, "getvariable")[-2]
            params(alias)["WFVariable"] = {"Value": {"Type": "ActionOutput", "OutputUUID": params(alias)["UUID"]}, "WFSerializationType": "WFTextTokenAttachment"}
        elif mutation == "scoped_result_coercion":
            params(changed[consumer])["WFVariable"]["Value"]["Aggrandizements"] = [{"Type": "WFCoercionVariableAggrandizement", "CoercionItemClass": "WFStringContentItem"}]
        elif mutation in ("extra_void_terminator", "legacy_void_terminator"):
            changed.append({"WFWorkflowActionIdentifier": NATIVE_PREFIX + "nothing"})
        elif mutation == "malformed_void_terminator":
            changed[-1]["WFWorkflowActionParameters"] = {"WFVariable": "unexpected payload"}
        elif mutation == "missing_void_terminator":
            changed.pop()
        elif mutation == "wrong_final_consumer":
            changed[consumer]["WFWorkflowActionIdentifier"] = NATIVE_PREFIX + "count"
        elif mutation == "original_uuid_replaced":
            original = next(a for a in named(changed, "gettext") if params(a).get("WFTextActionText") == MARKER)
            params(original)["UUID"] = "replaced-original-producer"
        elif mutation == "page_budget_changed":
            next(a for a in named(changed, "repeat.count") if str(params(a).get("WFControlFlowMode")) == "0")["WFWorkflowActionParameters"]["WFRepeatCount"] = 4
        else:
            changed.insert(consumer, {"WFWorkflowActionIdentifier": NATIVE_PREFIX + "gettimebetweendates", "WFWorkflowActionParameters": {}})
        original = structural_checks(spec["kind"], spec["profile"], actual, spec["dialect"])
        altered = structural_checks(spec["kind"], spec["profile"], changed, spec["dialect"])
        ok = all(original.values()) and original[expected] and not altered[expected]
        rows.append({"case": case_id, "mutation": mutation, "intendedCheck": expected,
                     "positiveControl": all(original.values()), "faultRejected": not altered[expected],
                     "verdict": "PASS" if ok else "FAIL"})
    return rows


def observations_digest(report):
    return sha(canonical({k: report[k] for k in ("compilerMacros", "canonicalDocs", "nativeMetadata",
                                                "compiles", "causalMutations", "crossCaseChecks", "requirements")}))


def build_rows(macros, cases):
    names = {m["callableName"]: m for m in macros}
    rows = []
    links = {
        "require": ["require_v2_debug", "require_v2_release", "require_legacy_debug", "require_legacy_release"],
        "blocks": ["measure_v2_debug", "measure_v2_release", "measure_legacy_debug", "measure_legacy_release", "measure_legacy_empty_rejected", "measure_legacy_control_value_rejected", "measure_v2_empty_rejected"],
        "measure": ["measure_v2_debug", "measure_v2_release", "measure_legacy_debug", "measure_legacy_release"],
        "collect": ["collect_v2_0", "collect_v2_1", "collect_v2_100", "collect_v2_dictionary"],
        "paginate": ["paginate_v2", "paginate_legacy", "paginate-zero_v2", "paginate-zero_legacy"],
        "profiles": ["require_v2_debug", "require_v2_release", "require_legacy_debug", "require_legacy_release", "measure_v2_debug", "measure_v2_release", "measure_legacy_debug", "measure_legacy_release"],
        "qa": [c["id"] for c in cases],
    }
    for key, task, title, wanted, requirement, state, external, limitations in REQUIREMENTS:
        descriptors = [name for name in wanted if name in names]
        if key == "label":
            exists = any(p.get("id") == "label" for p in names.get("debugLog", {}).get("parameters", []))
            contract = "LABEL_PARAMETER_PRESENT_REQUIRES_NEW_ACCEPTANCE" if exists else "LABEL_PARAMETER_ABSENT"
            if exists:
                state = "DECLARED_BUT_NOT_EXERCISED"
        elif key == "collect":
            contract = "EXISTING_FOR_YIELD_CONSTRUCT_NO_COLLECT_ALIAS"
        elif key in ("profiles", "qa"):
            contract = "CROSS_CUTTING_COMPILER_CONTRACT"
        else:
            contract = "DESCRIPTOR_PRESENT" if descriptors else "NO_CANONICAL_DESCRIPTOR"
            if descriptors and state.startswith("IMPLEMENTATION_GAP"):
                state = "DECLARED_BUT_NOT_EXERCISED"
        rows.append({"feature": key, "asanaTask": task, "name": title, "acceptanceRequirement": requirement,
                     "status": "OPEN", "implementationState": state, "canonicalDescriptors": descriptors,
                     "canonicalContractObservation": contract, "executedCases": links.get(key, []),
                     "externalPrerequisites": list(external), "limitations": list(limitations),
                     "nativeSourceExemplar": "NO_INDEPENDENT_APPLE_AUTHORED_MACRO_SPECIMEN",
                     "signedSimulator": "NOT_ESTABLISHED_BY_THIS_LEDGER",
                     "physicalDevice": "BLOCKED_DEVICE",
                     "closure": "NOT_PROVEN_BY_STRUCTURAL_EVIDENCE"})
    return rows


def refresh(repo):
    fingerprint = input_fingerprint(repo)
    source_sha = run(["git", "rev-parse", "HEAD"], repo).stdout.strip()
    dirty = bool(run(["git", "status", "--porcelain"], repo).stdout.strip())
    with tempfile.TemporaryDirectory(prefix="cherri-macro-contract-") as directory:
        work = Path(directory)
        binary = work / ("cherri.exe" if os.name == "nt" else "cherri")
        env = os.environ.copy()
        env["CGO_ENABLED"] = "0"
        build_command = ["go", "build", "-trimpath", "-o", str(binary), "."]
        run(build_command, repo, timeout=300, env=env)
        caps = json.loads(run([binary, "--capabilities-json"], repo).stdout)
        macros = caps.get("compilerMacros")
        if not isinstance(macros, list) or not macros or not re.fullmatch(r"[0-9a-f]{64}", caps.get("macroFingerprint", "")):
            raise ValueError("CANONICAL_MACRO_METADATA_MISSING")
        names = [m["callableName"] for m in macros]
        if names != sorted(set(names)):
            raise ValueError("MACRO_DESCRIPTORS_NOT_UNIQUE_SORTED")
        docs = work / "actions-reference.md"
        run(["go", "run", "./tools/language-docs", docs], repo, timeout=300, env=env)
        docs_text = docs.read_text(encoding="utf-8")
        tick = chr(96)
        expected_header = "**Compiler Macro Fingerprint:** " + tick + caps["macroFingerprint"] + tick
        docs_ok = expected_header in docs_text and "## Compiler macros" in docs_text
        native_path = repo / "docs/apple-runtime/macos-27/builtin-actions.json"
        meta = read_json(native_path)["is.workflow.actions.getitemtype"]
        native_ok = (meta.get("ActionClass") == "WFGetItemTypeAction" and
                     meta.get("Output", {}).get("Types") == ["NSString"] and
                     meta.get("Input", {}).get("ParameterKey") == "WFInput")
        compiles, outputs = [], {}
        for spec in fixture_specs():
            row, actions = compile_case(spec, binary, repo, work)
            compiles.append(row)
            outputs[spec["id"]] = actions
            print(json.dumps({"case": row["id"], "verdict": row["verdict"]}), flush=True)
        mutations = check_mutations(outputs)
        by_id = {c["id"]: c for c in compiles}
        totals, loop_counts, numeric_inputs = [], [], []
        for size in (0, 1, 100):
            graph = outputs["collect_v2_" + str(size)] or []
            boundaries = [i for i, action in enumerate(graph) if action_id(action) == NATIVE_PREFIX + "repeat.each"]
            totals.append(len(graph))
            loop_counts.append(boundaries[1] - boundaries[0] + 1 if len(boundaries) == 2 else None)
            numeric_inputs.append(len(named(graph, "number")))
        profile_counts = {}
        for dialect in ("legacy", "v2"):
            debug = by_id["require_" + dialect + "_debug"].get("nativeActionIDs", {})
            release = by_id["require_" + dialect + "_release"].get("nativeActionIDs", {})
            profile_counts[dialect] = all(release.get(NATIVE_PREFIX + name, 0) == debug.get(NATIVE_PREFIX + name, 0) > 0
                                          for name in ("conditional", "alert", "exit", "getitemtype", "count"))
        cross = {"constantCollectionLoopActionCount": None not in loop_counts and len(set(loop_counts)) == 1 and loop_counts[0] > 0,
                 "collectionLoopActionCounts": loop_counts, "collectionTotalActionCounts": totals,
                 "numericInputActionCounts": numeric_inputs, "typedNumericInputCountsMatch": numeric_inputs == [0, 1, 100],
                 "requireGuardCountsDebugRelease": profile_counts}
        requirements = build_rows(macros, compiles)
        report = {"formatVersion": 1, "observedAtUtc": datetime.datetime.now(datetime.timezone.utc).isoformat(),
                  "masterAsanaTask": MASTER, "sourceSha": source_sha, "worktreeDirtyBeforeExecution": dirty,
                  "sourceFingerprint": fingerprint, "generatorSHA256": sha(GENERATOR.read_bytes().replace(b"\r\n", b"\n")),
                  "buildCommand": ["go", "build", "-trimpath", "-o", "<temporary-compiler>", "."],
                  "goVersion": run(["go", "version"], repo).stdout.strip(),
                  "compilerSHA256": sha(binary.read_bytes()), "languageVersion": caps.get("languageVersion"),
                  "schemaFingerprint": caps.get("schemaFingerprint"), "macroFingerprint": caps["macroFingerprint"],
                  "nativeCatalogCount": caps.get("actionCount"), "compilerMacroCount": len(macros),
                  "compilerMacros": macros, "scope": "SYNTHETIC_CLI_STRUCTURAL_AND_LOSSLESS_NATIVE_ONLY",
                  "canonicalDocs": {"generatedFromThisSource": True, "macroFingerprintMatches": docs_ok, "sha256": sha(docs.read_bytes())},
                  "nativeMetadata": {"source": native_path.relative_to(repo).as_posix(),
                                     "sha256": sha(native_path.read_bytes().replace(b"\r\n", b"\n")),
                                     "canonicalTypeWitness": "is.workflow.actions.getitemtype", "contractMatches": native_ok},
                  "compiles": compiles, "causalMutations": mutations, "crossCaseChecks": cross,
                  "requirements": requirements,
                  "privacy": {"privateCorpusRead": False, "signerUsed": False, "networkFixtureExecuted": False, "liveConsumerChanged": False}}
    if input_fingerprint(repo) != fingerprint:
        raise ValueError("SOURCE_CHANGED_DURING_EXECUTION")
    passed = all(c["verdict"] == "PASS" for c in compiles) and all(c["verdict"] == "PASS" for c in mutations)
    passed = passed and docs_ok and native_ok and cross["constantCollectionLoopActionCount"] and cross["typedNumericInputCountsMatch"] and all(profile_counts.values())
    report["summary"] = {"asanaChildren": len(requirements), "open": len(requirements), "closed": 0,
                         "compilerMacros": len(macros), "compileChecks": len(compiles),
                         "compilePassed": sum(c["verdict"] == "PASS" for c in compiles),
                         "causalMutationChecks": len(mutations), "structuralExecutionVerdict": "PASS" if passed else "FAIL",
                         "releaseCompletion": "OPEN"}
    report["observationsSHA256"] = observations_digest(report)
    return report


def markdown(report):
    lines = ["# Macro contract coverage", "", "All eleven existing Asana children remain OPEN.", "",
             "This generated ledger records actual synthetic CLI compilation, native graph checks and exact lossless native-action roundtrip. It does not establish signed runtime, physical-device or installed-consumer acceptance.", "",
             "- Compiler source: " + report["sourceSha"],
             "- Source fingerprint: " + report["sourceFingerprint"],
             "- Macro fingerprint: " + report["macroFingerprint"],
             "- Canonical compiler macros: " + str(report["compilerMacroCount"]),
             "- Structural execution: " + report["summary"]["structuralExecutionVerdict"],
             "- Compile checks: " + str(report["summary"]["compilePassed"]) + "/" + str(report["summary"]["compileChecks"]),
             "- Causal mutation checks: " + str(report["summary"]["causalMutationChecks"]), "",
             "| Requirement | Asana child | Implementation observation | Executed cases | Status |",
             "|---|---|---|---:|---|"]
    for row in report["requirements"]:
        lines.append("| " + row["name"] + " | " + row["asanaTask"] + " | " + row["implementationState"] + " | " + str(len(row["executedCases"])) + " | OPEN |")
    lines.extend(["", "## Actual controlled CLI observations", "",
                  "| Case | Dialect/profile | Native actions | Exact lossless action roundtrip | Result |",
                  "|---|---|---:|---|---|"])
    for row in report["compiles"]:
        roundtrip = row.get("decompile", {}).get("nativeActionsExact")
        detail = str(roundtrip) if roundtrip is not None else "expected unavailable/invalid diagnostic"
        lines.append("| " + row["id"] + " | " + row["dialect"] + "/" + row["profile"] + " | " + str(row.get("nativeActionCount", "-")) + " | " + detail + " | " + row["verdict"] + " |")
    lines.extend(["", "## Limits and next prerequisites", ""])
    for row in report["requirements"]:
        lines.append("- " + row["name"] + ": " + " ".join(row["limitations"]) +
                     (" External prerequisites: " + "; ".join(row["externalPrerequisites"]) + "." if row["externalPrerequisites"] else ""))
    lines.extend(["", "Standalone collect is deliberately not added. Existing for/yield evidence belongs to the typed collection requirement; it is not counted as a new canonical macro. Numeric input construction precedes the constant-size native loop and is counted separately. Cache and retry have both missing source implementation and native prerequisites. Label and bounded GET pagination have implementations with open native acceptance. Local export has a bounded Text-only/default-redacted source subset; its native acceptance and remaining typed serialization contract stay open.", "",
                  "Reproduce with python3 tools/p10r15/generate_macro_contract_coverage.py --refresh, then --verify. Every refresh builds the source and runs the actual CLI. Verification rejects source/producer drift and edited or incomplete evidence. The source SHA is the compiled revision before committing evidence, avoiding a self-referential commit pin.", ""])
    return "\n".join(lines)


def verify(report, repo):
    if report.get("sourceFingerprint") != input_fingerprint(repo):
        raise ValueError("STALE_MACRO_SOURCE_EVIDENCE")
    if report.get("generatorSHA256") != sha(GENERATOR.read_bytes().replace(b"\r\n", b"\n")):
        raise ValueError("STALE_MACRO_PRODUCER")
    if report.get("observationsSHA256") != observations_digest(report):
        raise ValueError("EDITED_MACRO_OBSERVATIONS")
    if {r["asanaTask"] for r in report["requirements"]} != {r[1] for r in REQUIREMENTS} or len(report["requirements"]) != 11:
        raise ValueError("MISSING_OR_DUPLICATE_ASANA_CHILD")
    if any(r["status"] != "OPEN" for r in report["requirements"]):
        raise ValueError("STRUCTURAL_EVIDENCE_CANNOT_CLOSE_CHILD")
    if {c["id"] for c in report["compiles"]} != {c["id"] for c in fixture_specs()}:
        raise ValueError("MISSING_CONTROLLED_COMPILE")
    if report["summary"]["structuralExecutionVerdict"] != "PASS":
        raise ValueError("MACRO_EXECUTION_FAILED")
    if any(c["verdict"] != "PASS" for c in report["compiles"] + report["causalMutations"]):
        raise ValueError("MACRO_CASE_FAILED")
    if (len(report["compiles"]) != len(fixture_specs()) or
        tuple((m.get("case"), m.get("mutation"), m.get("intendedCheck")) for m in report["causalMutations"]) != MUTATION_SCENARIOS):
        raise ValueError("MACRO_EXECUTION_COUNT_MISMATCH")
    if any(not m.get("positiveControl") or not m.get("faultRejected") for m in report["causalMutations"]):
        raise ValueError("MACRO_MUTATION_NOT_CAUSAL")
    for case in report["compiles"]:
        if case["sourceSHA256"] != sha(case["source"].encode("utf-8")):
            raise ValueError("MACRO_FIXTURE_HASH_MISMATCH")
        if case["kind"] == "negative":
            if case["exitCode"] == 0 or not case["noArtifact"] or case["expectedDiagnostic"] not in case["diagnosticTail"]:
                raise ValueError("MACRO_NEGATIVE_NOT_CAUSAL")
            continue
        if case["exitCode"] != 0 or not case["structuralChecks"] or not all(case["structuralChecks"].values()):
            raise ValueError("MACRO_STRUCTURAL_CHECK_FAILED")
        if case["nativeActionCount"] != sum(case["nativeActionIDs"].values()):
            raise ValueError("MACRO_NATIVE_ACTION_COUNT_MISMATCH")
        decompile = case["decompile"]
        if decompile["importExitCode"] != 0 or decompile["recompileExitCode"] != 0 or not decompile["nativeActionsExact"]:
            raise ValueError("MACRO_LOSSLESS_ROUNDTRIP_FAILED")
    if not report["canonicalDocs"]["macroFingerprintMatches"] or not report["nativeMetadata"]["contractMatches"]:
        raise ValueError("MACRO_CANONICAL_CONTRACT_FAILED")
    cross = report["crossCaseChecks"]
    if not cross["constantCollectionLoopActionCount"] or not cross["typedNumericInputCountsMatch"] or not all(cross["requireGuardCountsDebugRelease"].values()):
        raise ValueError("MACRO_CROSS_CASE_CHECK_FAILED")
    names = [m["callableName"] for m in report["compilerMacros"]]
    if report["compilerMacroCount"] != len(names) or names != sorted(set(names)):
        raise ValueError("MACRO_DESCRIPTOR_COUNT_MISMATCH")
    summary = report["summary"]
    if summary["open"] != 11 or summary["closed"] != 0 or summary["compileChecks"] != len(report["compiles"]) or summary["compilePassed"] != len(report["compiles"]) or summary["causalMutationChecks"] != len(MUTATION_SCENARIOS):
        raise ValueError("MACRO_SUMMARY_MISMATCH")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--repo", type=Path, default=ROOT)
    parser.add_argument("--out-dir", type=Path)
    mode = parser.add_mutually_exclusive_group(required=True)
    mode.add_argument("--refresh", action="store_true")
    mode.add_argument("--verify", action="store_true")
    opts = parser.parse_args()
    repo = opts.repo.resolve()
    out = opts.out_dir.resolve() if opts.out_dir else repo / "docs/apple-runtime/p10r15"
    json_path = out / "macro-contract-coverage.json"
    md_path = out / "macro-contract-coverage.md"
    if opts.refresh:
        report = refresh(repo)
        write_json(json_path, report)
        md_path.write_bytes(markdown(report).encode("utf-8"))
    else:
        report = read_json(json_path)
    verify(report, repo)
    if md_path.read_bytes() != markdown(report).encode("utf-8"):
        raise ValueError("STALE_MACRO_MARKDOWN")
    print(json.dumps(report["summary"]), flush=True)


if __name__ == "__main__":
    try:
        main()
    except (ValueError, RuntimeError, OSError, subprocess.TimeoutExpired) as exc:
        print(str(exc), file=sys.stderr)
        raise SystemExit(1)
