"""Execute the real fixture-compilation phase using the actual local Cherri CLI.

This validator never invokes signing or a simulator. Negative cases alter an
in-memory copy of the production phase and write only ignored artifacts.
"""
from pathlib import Path
import json
import shutil
import subprocess
import sys

repo = Path(sys.argv[1]).resolve()
binary = Path(sys.argv[2]).resolve()
output = repo / 'artifacts/p15-macro-harness-validation'
output.mkdir(parents=True, exist_ok=True)
script = (repo / 'scripts/ios27_shortcuts_simulator_qa.sh').read_text(encoding='utf-8')
start = script.index('python3 - "$ROOT_DIR" "$ARTIFACT_DIR" "$SOURCE_SHA"')
start = script.index("<<'PY'\n", start) + len("<<'PY'\n")
end = script.index('\nPY\n', start)
program = script[start:end]
sha = subprocess.check_output(['git', '-C', str(repo), 'rev-parse', 'HEAD'], text=True).strip()

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
selected = {f['fixtureId']: f for f in manifest['fixtures'] if f['fixtureId'] in (
    'qa-require-v2', 'qa-measure-v2', 'qa-measure-release-v2')}
assert len(selected) == 3
assert selected['qa-measure-release-v2']['buildProfile'] == 'release'
assert selected['qa-measure-release-v2']['releaseDiagnosticActionsAbsent'] is True
assert selected['qa-measure-release-v2']['releaseBodyShapeVerified'] is True

release_only = program.replace('for fid, fname, dialect in fixture_specs:',
                               'for fid, fname, dialect in [spec for spec in fixture_specs if spec[0] == "qa-measure-release-v2"]:')
ignored_profile = release_only.replace(', f"--build-profile={profile}"', '')
missing_body = release_only.replace('    shutil.copyfile(fpath, build_src)\n',
    '    shutil.copyfile(fpath, build_src)\n'
    '    with open(build_src, encoding="utf-8") as body_file: body = body_file.read()\n'
    '    with open(build_src, "w", encoding="utf-8") as body_file: body_file.write(body.replace("wait(1)", "comment(\\\"CHERRI_QA_BODY_REMOVED\\\")"))\n')
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

report = {
    'schemaVersion': '1.0', 'compilerCommitSha': sha,
    'actualCLIFixturesCompiled': len(manifest['fixtures']),
    'selectedFixtures': selected, 'causalCompileMutations': negatives,
    'appleRuntime': 'NOT_RUN', 'signing': 'NOT_RUN',
}
(output / 'validation.json').write_text(json.dumps(report, indent=2) + '\n', encoding='utf-8')
print(json.dumps(report, indent=2))
