"""Execute the real fixture-compilation phase using the actual local Cherri CLI.

This validator never invokes signing or a simulator. Negative cases alter an
in-memory copy of the production phase and write only ignored artifacts.
"""
from pathlib import Path
import hashlib
import json
import shutil
import subprocess
import sys
import tempfile

from prepare_public_artifacts import PublicationBlocked, stage, trusted_fixture_sources

repo = Path(sys.argv[1]).resolve()
binary = Path(sys.argv[2]).resolve()
output = repo / 'artifacts/p15-macro-harness-validation'
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
    shutil.copyfile(binary, directory / 'builds/cherri')
    return subprocess.run([sys.executable, '-c', code, str(repo), str(directory), sha],
                          capture_output=True, text=True, timeout=100), directory

positive, directory = execute('positive', program)
if positive.returncode != 0:
    print(positive.stdout)
    print(positive.stderr)
    raise SystemExit(positive.returncode)
manifest = json.loads((directory / 'compiler-manifest.json').read_text())
new_probes = {
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
    'actualCLIFixturesCompiled': len(manifest['fixtures']),
    'selectedFixtures': selected, 'newExplicitSignedQASelections': len(new_probes),
    'causalCompileMutations': negatives, 'causalPublicationMutations': publication_negatives,
    'publication': {'publishedTextFiles': len(publication['published']),
                    'quarantinedRawFiles': len(publication['quarantined']),
                    'rasterPublicationAllowed': False},
    'appleRuntime': 'NOT_RUN', 'signing': 'NOT_RUN',
}
(output / 'validation.json').write_text(json.dumps(report, indent=2) + '\n', encoding='utf-8')
print(json.dumps({k: report[k] for k in ('status', 'actualCLIFixturesCompiled', 'newExplicitSignedQASelections',
    'causalCompileMutations', 'causalPublicationMutations', 'publication', 'appleRuntime', 'signing')}, indent=2))
