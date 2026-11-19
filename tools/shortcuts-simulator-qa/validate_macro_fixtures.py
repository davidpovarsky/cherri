"""Execute the real fixture-compilation phase using the actual local Cherri CLI.

This validator never invokes signing or a simulator. Negative cases alter an
in-memory copy of the production phase and write only ignored artifacts.
"""
from pathlib import Path
import copy
import hashlib
import plistlib
import json
import shutil
import subprocess
import sys
import tempfile

from prepare_public_artifacts import PublicationBlocked, stage, trusted_fixture_sources
from fixture_compile_contract import EXPECTED_REJECTIONS
from export_unicode_diagnostic import FIXTURE_ID as UNICODE_ID, validate_export_unicode_graph

repo = Path(sys.argv[1]).resolve()
binary = Path(sys.argv[2]).resolve()
output = Path(sys.argv[3]).resolve() if len(sys.argv) > 3 else repo / 'artifacts/p15-macro-harness-validation'
output.mkdir(parents=True, exist_ok=True)
script_path = repo / 'scripts/ios27_shortcuts_simulator_qa.sh'
script = script_path.read_text(encoding='utf-8')
start = script.index('python3 - "$ROOT_DIR" "$ARTIFACT_DIR" "$SOURCE_SHA"')
start = script.index("<<'PY'\n", start) + len("<<'PY'\n")
end = script.index('\nPY\n', start)
program = script[start:end]
sha = subprocess.check_output(['git', '-C', str(repo), 'rev-parse', 'HEAD'], text=True).strip()
source_dirty = bool(subprocess.check_output(['git', '-C', str(repo), 'status', '--porcelain'], text=True).strip())

def execute(name, code):
    directory = output / name
    (directory / 'builds').mkdir(parents=True, exist_ok=True)
    (directory / 'runtime').mkdir(parents=True, exist_ok=True)
    # Preserve the actual compiler executable mode on POSIX hosts.
    shutil.copy2(binary, directory / 'builds/cherri')
    return subprocess.run([sys.executable, '-c', code, str(repo), str(directory), sha],
                          capture_output=True, text=True, timeout=100), directory

positive, directory = execute('positive', program)
if positive.returncode != 0:
    print(positive.stdout)
    print(positive.stderr)
    raise SystemExit(positive.returncode)
manifest = json.loads((directory / 'compiler-manifest.json').read_text())
rejected = {entry["fixtureId"]: entry for entry in manifest["fixtureRejections"]}
assert set(rejected) == set(EXPECTED_REJECTIONS)
assert manifest["compileAttempts"] == len(manifest["fixtures"]) + len(rejected)
assert manifest["compiledArtifactCount"] == len(manifest["fixtures"])
assert manifest["expectedDiagnosticRejectionCount"] == len(rejected)
assert not set(rejected).intersection(entry["fixtureId"] for entry in manifest["fixtures"])
for fid, entry in rejected.items():
    assert entry["status"] == "BLOCKED_PLATFORM" and entry["appleRuntime"] == "NOT_RUN", fid
    assert entry["compileOutcome"] == "REJECTED_UNSUPPORTED_VALUE_SHAPE", fid
    assert entry["diagnostic"]["code"] == "E_TARGET_VALUE_SHAPE", fid
    assert entry["compileExitCode"] != 0 and entry["checkExitCode"] != 0, fid
    assert entry["nativeArtifactEmitted"] is False, fid
    assert not (directory / "builds" / (fid + ".shortcut")).exists(), fid
    source = repo / "tools/shortcuts-simulator-qa/fixtures" / entry["fileName"]
    assert entry["sourceSha256"] == hashlib.sha256(source.read_bytes()).hexdigest(), fid
new_probes = {
    'qa-export-unicode-diagnostic-v2': '',
    'qa-nil-clear-v2': '',
    'qa-label-file-carrier-v2': 'p15-label-probes',
    'qa-label-raw-empty-v2': 'p15-label-probes',
    'qa-label-raw-grouping-v2': 'p15-label-probes',
    'qa-export-unicode-v2': 'p15-export-probes',
    'qa-export-empty-v2': 'p15-export-probes',
    'qa-export-thousand-v2': 'p15-export-probes',
    'qa-export-redacted-v2': 'p15-export-probes',
    'qa-export-unsupported-file-v2': 'p15-export-probes',
    'qa-export-text-file-v2': 'p15-export-probes',
    'qa-paginate-ordered-v2': 'p15-paginate-probes',
    'qa-paginate-cycle-v2': 'p15-paginate-probes',
    'qa-paginate-page-cap-v2': 'p15-paginate-probes',
    'qa-paginate-item-cap-v2': 'p15-paginate-probes',
    'qa-paginate-empty-v2': 'p15-paginate-probes',
    'qa-storage-text-same-run-v2': '',
    'qa-storage-scope-v2': '',
    'qa-storage-delete-v2': '',
    'qa-storage-text-cross-run-v2': '',
    'qa-http-json-control-v2': '',
    'qa-http-json-fetch-v2': '',
    'qa-number-comparison-diag-v2': '',
    'qa-number-coercion-control-v2': '',
}
selected_ids = {'qa-require-v2', 'qa-measure-v2', 'qa-measure-release-v2', *new_probes}
selected = {f['fixtureId']: f for f in manifest['fixtures'] if f['fixtureId'] in selected_ids}
assert set(selected) == selected_ids
assert selected['qa-measure-release-v2']['buildProfile'] == 'release'
assert selected['qa-measure-release-v2']['releaseDiagnosticActionsAbsent'] is True
assert selected['qa-measure-release-v2']['releaseBodyShapeVerified'] is True
for fid, folder in new_probes.items():
    entry = selected[fid]
    expected_file = f'{folder}/{fid}.cherri' if folder else f'{fid}.cherri'
    assert entry['fileName'] == expected_file, fid
    assert entry['dialect'] == 'v2' and entry['buildProfile'] == 'debug', fid
    assert entry['runtimeTraceExpectation'] == 'REQUIRED_NDJSON', fid
    assert entry['releaseDiagnosticActionsAbsent'] is False, fid
    assert entry['releaseBodyShapeVerified'] is False, fid
    source = repo / 'tools/shortcuts-simulator-qa/fixtures' / entry['fileName']
    assert entry['sourceSha256'] == hashlib.sha256(source.read_bytes()).hexdigest(), fid
    compiled = directory / 'builds' / (fid + '.shortcut')
    assert entry['unsignedPlistSha256'] == hashlib.sha256(compiled.read_bytes()).hexdigest(), fid
    assert entry['nativeActionCount'] > 0, fid

# Mutate the actual emitted diagnostic graph. This does not simulate Apple:
# it establishes that a wrong source edge, forged flag or missing output cap
# cannot be signed as this reviewed diagnostic.
unicode_entry = selected[UNICODE_ID]
unicode_graph = plistlib.loads((directory / "builds" / (UNICODE_ID + ".shortcut")).read_bytes())["WFWorkflowActions"]
unicode_result = validate_export_unicode_graph(UNICODE_ID, unicode_graph)
assert unicode_entry["exportUnicodeDiagnosticGraph"] == unicode_result
unicode_mutations = {}

def graph_params(actions, name):
    matches = [a["WFWorkflowActionParameters"] for a in actions
               if a.get("WFWorkflowActionParameters", {}).get("CustomOutputName") == name]
    assert len(matches) == 1, name
    return matches[0]

def graph_write_indexes(actions, variable):
    return [i for i, a in enumerate(actions) if a.get("WFWorkflowActionIdentifier") == "is.workflow.actions.setvariable"
            and a.get("WFWorkflowActionParameters", {}).get("WFVariableName") == variable]

for name in ("changed-unicode-literal", "wrong-log-producer", "bypassed-file-materialization",
             "wrong-payload-key", "restored-decode-bypasses-payload", "literal-control-uses-payload",
             "byte-witness-aliases-original", "byte-witness-line-breaks", "witness-cap-raised",
             "witness-cap-wrong-operator", "witness-overwritten-after-cap", "inequality-became-equality",
             "flag-forged-positive", "summary-bypasses-bounded-variable", "output-replaced-with-positive"):
    damaged = copy.deepcopy(unicode_graph)
    def p(value): return graph_params(damaged, value)
    def output_ref(value):
        return {"WFSerializationType": "WFTextTokenAttachment",
                "Value": {"Type": "ActionOutput", "OutputUUID": p(value)["UUID"], "OutputName": value}}
    if name == "changed-unicode-literal":
        p("original")["WFTextActionText"] += "wrong"
    elif name == "wrong-log-producer":
        oi = next(i for i, a in enumerate(damaged) if a.get("WFWorkflowActionParameters", {}).get("CustomOutputName") == "original")
        damaged[oi + 1]["WFWorkflowActionParameters"]["WFInput"] = output_ref("literalText")
    elif name == "bypassed-file-materialization":
        p("artifact")["WFInput"] = output_ref("original")
    elif name == "wrong-payload-key":
        p("payloadValue")["WFDictionaryKey"] = "status"
    elif name == "restored-decode-bypasses-payload":
        p("restoredFile")["WFInput"] = output_ref("original")
    elif name == "literal-control-uses-payload":
        p("literalFile")["WFInput"] = output_ref("payloadValue")
    elif name == "byte-witness-aliases-original":
        p("restored64Raw")["WFInput"] = output_ref("original")
    elif name == "byte-witness-line-breaks":
        p("original64Raw")["WFBase64LineBreakMode"] = "Every 64 Characters"
    elif name in ("witness-cap-raised", "witness-cap-wrong-operator"):
        first = graph_write_indexes(damaged, "original64")[0]
        cp = damaged[first + 1]["WFWorkflowActionParameters"]
        cp["WFNumberValue" if name == "witness-cap-raised" else "WFCondition"] = 999999 if name == "witness-cap-raised" else 4
    elif name == "witness-overwritten-after-cap":
        second = graph_write_indexes(damaged, "original64")[1]
        damaged.insert(second + 2, copy.deepcopy(damaged[second]))
    elif name == "inequality-became-equality":
        first = graph_write_indexes(damaged, "restoredNe")[0]
        damaged[first + 1]["WFWorkflowActionParameters"]["WFCondition"] = 4
    elif name == "flag-forged-positive":
        first, second = graph_write_indexes(damaged, "restoredEq")
        damaged[first]["WFWorkflowActionParameters"]["WFInput"] = copy.deepcopy(damaged[second]["WFWorkflowActionParameters"]["WFInput"])
    elif name == "summary-bypasses-bounded-variable":
        atts = p("finalSummary")["WFTextActionText"]["Value"]["attachmentsByRange"]
        k = next(k for k, v in atts.items() if v.get("VariableName") == "original64")
        atts[k] = output_ref("original64Raw")["Value"]
    else:
        damaged[-1]["WFWorkflowActionParameters"]["WFOutput"] = "CHERRI_QA_EXPORT_UNICODE_DIAG:TYPE=Y"
    try:
        validate_export_unicode_graph(UNICODE_ID, damaged)
    except ValueError as exc:
        assert str(exc).startswith("EXPORT_UNICODE_DIAGNOSTIC_GRAPH_MISMATCH:"), str(exc)
        unicode_mutations[name] = {"expectedRejectionVerified": True, "diagnostic": str(exc)}
    else:
        raise AssertionError(name + ": damaged diagnostic graph incorrectly accepted")

release_only = program.replace('for fid, fname, dialect in fixture_specs:',
                               'for fid, fname, dialect in [spec for spec in fixture_specs if spec[0] == "qa-measure-release-v2"]:')
ignored_profile = release_only.replace(', f"--build-profile={profile}"', '')
missing_body = release_only.replace('    shutil.copyfile(fpath, build_src)\n',
    '    shutil.copyfile(fpath, build_src)\n'
    '    with open(build_src, encoding="utf-8") as body_file: body = body_file.read()\n'
    '    with open(build_src, "w", encoding="utf-8") as body_file: body_file.write(body.replace("wait(1)", "comment(\\"CHERRI_QA_BODY_REMOVED\\")"))\n')
duplicated_body = release_only.replace('    shutil.copyfile(fpath, build_src)\n',
    '    shutil.copyfile(fpath, build_src)\n'
    '    with open(build_src, encoding="utf-8") as body_file: body = body_file.read()\n'
    '    with open(build_src, "w", encoding="utf-8") as body_file: body_file.write(body.replace("wait(1)", "wait(1); wait(1)"))\n')
negatives = {}
for name, code in (('ignored-release-profile', ignored_profile), ('missing-body', missing_body), ('duplicated-body', duplicated_body)):
    result, _ = execute(name, code)
    assert result.returncode != 0, f'{name}: negative mutation incorrectly accepted'
    assert 'Release measure fixture retained diagnostics or lost/duplicated its native body' in result.stderr, result.stderr
    negatives[name] = {'exitCode': result.returncode, 'expectedRejectionVerified': True}

# Execute the preserved F04 compile/check paths with damaged success evidence.
# An unrelated error, success exit or stale artifact must never count as the
# required fail-closed diagnostic.
f04_only = program.replace("for fid, fname, dialect in fixture_specs:",
    'for fid, fname, dialect in [spec for spec in fixture_specs if spec[0] == "qa-f04-yield-list-v2"]:')
fault_anchor = "        rejection = verify_expected_rejection("
rejection_negatives = {}
for name, fault in (
    ("wrong-diagnostic", '        checked.stdout = checked.stdout.replace("E_TARGET_VALUE_SHAPE", "E_ARGUMENT_TYPE")\n'),
    ("compile-exit-success", '        proc.returncode = 0\n'),
    ("unexpected-native-artifact", '        with open(out_shortcut, "wb") as stale: stale.write(b"unexpected-native-output")\n'),
    ("extra-unrelated-error", '        diagnostic_result = json.loads(checked.stdout)\n'
                            '        diagnostic_result["diagnostics"].append({"severity":"ERROR","code":"E_UNKNOWN_NAME"})\n'
                            '        checked.stdout = json.dumps(diagnostic_result)\n'),
):
    result, _ = execute("f04-" + name, f04_only.replace(fault_anchor, fault + fault_anchor))
    assert result.returncode != 0 and "EXPECTED_FIXTURE_REJECTION_MISMATCH" in result.stderr, (name, result.stderr)
    rejection_negatives[name] = {"exitCode": result.returncode, "expectedRejectionVerified": True}

# Exercise the actual selected-target gate before any signing/Simulator work.
target_marker = 'python3 - "$ARTIFACT_DIR/compiler-manifest.json" "$TARGET_FIXTURE_ID"'
target_start = script.index("<<'PY'\n", script.index(target_marker)) + len("<<'PY'\n")
target_end = script.index("\nPY\n", target_start)
target_gate = script[target_start:target_end]
target_results = {}
for fid, expected_code in [(fid, 3) for fid in EXPECTED_REJECTIONS] + [(UNICODE_ID, 0), ("qa-export-unicode-v2", 0), ("qa-storage-text-cross-run-v2", 0)]:
    result = subprocess.run([sys.executable, "-c", target_gate, str(directory / "compiler-manifest.json"), fid],
                            capture_output=True, text=True, timeout=10)
    assert result.returncode == expected_code, (fid, result.stderr)
    if expected_code:
        assert "BLOCKED_PLATFORM" in result.stderr and "E_TARGET_VALUE_SHAPE" in result.stderr
    target_results[fid] = {"exitCode": result.returncode, "signing": "NOT_RUN", "appleRuntime": "NOT_RUN"}

# Use actual Phase 0 source hashes against reviewed checkout files. This
# synthetic local producer supplies the publication precondition, not a claim
# that an Apple run's privacy gate has passed.
(directory / 'summary.json').write_text(json.dumps({
    'privacyGatePassed': True, 'evidenceScope': 'SYNTHETIC_LOCAL_COMPILATION_ONLY',
    'appleRuntime': 'NOT_RUN', 'signing': 'NOT_RUN',
}), encoding='utf-8')
trusted = trusted_fixture_sources(repo / 'tools/shortcuts-simulator-qa/fixtures')
# Every invocation uses a new destination. Never overwrite older evidence.
publication_parent = Path(tempfile.mkdtemp(prefix='publication-', dir=output))
publication = stage(directory, publication_parent / 'public', trusted)
assert publication['publicationGatePassed'] is True
assert publication['rasterPublicationAllowed'] is False
assert not list((publication_parent / 'public').rglob('*.cherri'))
assert not list((publication_parent / 'public').rglob('*.shortcut'))

publication_negatives = {}
base_entry = selected['qa-export-unicode-v2']
for name, mutation, diagnostic in [
    ('nested-basename-flattened', {'fileName': 'qa-export-unicode-v2.cherri'}, 'FIXTURE_SOURCE_HASH_MISMATCH'),
    ('unreviewed-same-basename', {'fileName': 'private/qa-export-unicode-v2.cherri'}, 'FIXTURE_SOURCE_HASH_MISMATCH'),
    ('nested-path-traversal', {'fileName': 'p15-export-probes/../qa-export-unicode-v2.cherri'}, 'FIXTURE_SOURCE_HASH_MISMATCH'),
    ('stale-source-hash', {'sourceSha256': '0' * 64}, 'FIXTURE_SOURCE_HASH_MISMATCH'),
    ('source-identity-reassigned', {'fixtureId': 'qa-label-file-carrier-v2'}, 'FIXTURE_SOURCE_ID_MISMATCH'),
]:
    raw = publication_parent / name
    raw.mkdir()
    (raw / 'compiler-manifest.json').write_text(json.dumps({
        'syntheticOnly': True, 'fixtures': [{**base_entry, **mutation}],
    }), encoding='utf-8')
    (raw / 'summary.json').write_text('{"privacyGatePassed":true}', encoding='utf-8')
    try:
        stage(raw, publication_parent / (name + '-public'), trusted)
    except PublicationBlocked as exc:
        assert diagnostic == str(exc), (name, str(exc))
    else:
        raise AssertionError(name + ': publication mutation incorrectly accepted')
    publication_negatives[name] = {'expectedRejectionVerified': True, 'diagnostic': diagnostic}

report = {
    'schemaVersion': '1.1', 'status': 'PASS_LOCAL_COMPILATION_AND_GATES_ONLY',
    'compilerCommitSha': sha, 'workingTreeDirty': source_dirty,
    'orchestratorSha256': hashlib.sha256(script_path.read_bytes()).hexdigest(),
    'validatorSha256': hashlib.sha256(Path(__file__).read_bytes()).hexdigest(),
    'compilerBinarySha256': hashlib.sha256(binary.read_bytes()).hexdigest(),
    'actualCLIFixturesAttempted': manifest['compileAttempts'],
    'actualCLIFixturesCompiled': len(manifest['fixtures']),
    'actualCLIExpectedDiagnosticRejections': len(rejected),
    'selectedFixtures': selected, 'newExplicitSignedQASelections': len(new_probes),
    'unicodeDiagnosticGraph': unicode_result, 'causalUnicodeGraphMutations': unicode_mutations,
    'causalCompileMutations': negatives, 'causalPublicationMutations': publication_negatives,
    'preservedUnsupportedFixtures': rejected, 'causalRejectionMutations': rejection_negatives,
    'selectedTargetCompileGates': target_results,
    'publication': {'publishedTextFiles': len(publication['published']),
                    'quarantinedRawFiles': len(publication['quarantined']),
                    'rasterPublicationAllowed': False},
    'appleRuntime': 'NOT_RUN', 'signing': 'NOT_RUN',
}
(output / 'validation.json').write_text(json.dumps(report, indent=2) + '\n', encoding='utf-8')
print(json.dumps({k: report[k] for k in ('status', 'actualCLIFixturesAttempted', 'actualCLIFixturesCompiled', 'actualCLIExpectedDiagnosticRejections', 'newExplicitSignedQASelections',
    'unicodeDiagnosticGraph', 'causalUnicodeGraphMutations', 'causalCompileMutations', 'causalPublicationMutations', 'causalRejectionMutations', 'selectedTargetCompileGates', 'publication', 'appleRuntime', 'signing')}, indent=2))
