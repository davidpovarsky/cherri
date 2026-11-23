#!/usr/bin/env python3
"""Inactive, exact-K11 proposal. Default invocation launches nothing.

The original production CLI performs acceptance and both canonical AI intakes.
This adapter preserves process/source receipts; it grants no acceptance credit.
"""
import argparse
import collections
import datetime as dt
import hashlib
import json
import os
from pathlib import Path
import re
import signal
import stat
import subprocess
import sys
import time

PLAN = Path('/mnt/development/projects/evidence/cherri-p10r15-k11-local179-plan-20261123')
SPEC = PLAN / 'launch-spec.REVIEWED.json'
SOURCE = Path('/dev/shm/cherri-p10r15-runtime-20261123/cherri-k11-source')
OUTPUT = Path('/mnt/development/projects/evidence/cherri-p10r15-k11-local179-20261123/attempt-01')
SOURCE_SHA = '9b4135e705b08f765b7304e01212d3b9658806fb'
SOURCE_TREE = 'd3688f219f7454e80778cbcb3013967b6d339aad'
SOURCE_BRANCH = 'agent/cherri-p10r15-k11-verification-20261123'
ORIGIN = 'https://github.com/davidpovarsky/cherri.git'
DOCS = Path('/mnt/development/projects/worktrees/cherrilang-p10r15-d11-20261123')
DOCS_SHA = '365b08c5b4c71c6d22b591babb8d1ab852e5a3b9'
DOCS_TREE = '77f0e0f443434cce6ca2870884503f6624f0ed3f'
COMPILER_SOURCE = Path('/mnt/development/projects/releases/cherri-p10r15-r7-compiler-source-20261123')
COMPILER_SHA = 'ca316a3dee252456c4bec751594cf249c709b331'
COMPILER_TREE = 'df61b6ddf47137998c6053cfada292f0a6a17cca'
PAIR = Path('/mnt/development/projects/evidence/cherri-p10r15-r7-arm64-pair-20261123/attempt-02')
BINARIES = {
    'cherri': '42f6e42e7b6a8ea6f523a752fd2f06b96265e733fa00ad808190e062d1266301',
    'cherri-migrate': '050e33ad7b078118c0c9dea8ca32a0123926d4a975154244311a5a9f4fe64f91',
}
CONTRACTS = {
    'tests/language-v2/acceptance-contract.json': ('9fdfa8910b09824dc61395d8ddf4aa8fe8914619af0fdde59c63b67ab10b895e', 'cases', 94),
    'tests/language-v2/repair-regression-plan.json': ('eba070df0e1ff229a94e52d977a09c2a0c125a31b1a69caf32787d41600074f5', 'cases', 52),
    'tests/language-v2/backend-recovery-gates.json': ('e7fbb3ad177c5142935f71a46ca7e89b84822061bfc0919eec59708811ab7e7c', 'gates', 33),
}
WORKFLOWS = ('Build & Test', 'OpenMinis Skill', 'iOS Build', 'iOS 27 Shortcuts Runtime PoC')
TASKS = ('generate_unicode_literal', 'generate_sum', 'generate_list_selection',
         'edit_literal_only', 'edit_binding_reference', 'edit_arithmetic_operand')
COMMAND = ['/usr/bin/timeout', '--signal=TERM', '--kill-after=30s', '80m',
           '/usr/local/bin/go', 'run', './tools/language-acceptance', '--phase', 'local',
           '--out', str(OUTPUT / 'acceptance-report.json')]
UNIT = 'cherri-p10r15-local179-k11-20261123-attempt01.service'
BUDGETS = {'producerSeconds': 4800, 'producerTerminationGraceSeconds': 30,
           'outerTotalSeconds': 5400, 'outerRunSeconds': 5340,
           'outerStopSeconds': 30, 'outerStopReceiptSeconds': 30}


class GuardError(Exception):
    pass


def require(value, code):
    if not value:
        raise GuardError(code)


def now():
    return dt.datetime.now(dt.timezone.utc).isoformat().replace('+00:00', 'Z')


def sha(raw):
    return hashlib.sha256(raw).hexdigest()


def unique_object(items):
    value = {}
    for key, item in items:
        require(key not in value, 'DUPLICATE_JSON_KEY')
        value[key] = item
    return value


def regular(path, limit=16 * 1024 * 1024):
    path = Path(path)
    before = path.lstat()
    require(stat.S_ISREG(before.st_mode) and before.st_size <= limit, 'REGULAR_INPUT_REQUIRED')
    with os.fdopen(os.open(path, os.O_RDONLY | os.O_NOFOLLOW), 'rb') as stream:
        opened = os.fstat(stream.fileno())
        require((opened.st_dev, opened.st_ino) == (before.st_dev, before.st_ino), 'INPUT_CHANGED')
        raw = stream.read(limit + 1)
    after = path.lstat()
    require(len(raw) == before.st_size and (before.st_dev, before.st_ino, before.st_size, before.st_mtime_ns) ==
            (after.st_dev, after.st_ino, after.st_size, after.st_mtime_ns), 'INPUT_CHANGED')
    return raw


def load_json(raw):
    return json.loads(raw, object_pairs_hook=unique_object)


def file_pin(path, expected, limit=16 * 1024 * 1024):
    require(isinstance(expected, str) and re.fullmatch(r'[0-9a-f]{64}', expected), 'HASH_PIN_REQUIRED')
    raw = regular(path, limit)
    require(sha(raw) == expected, 'HASH_PIN_MISMATCH')
    return raw


def path_pin(value):
    require(isinstance(value, str) and value.startswith('/') and os.path.normpath(value) == value, 'ABSOLUTE_CLEAN_PATH_REQUIRED')
    path = Path(value)
    require(path.resolve(strict=True) == path, 'SYMLINK_INPUT_PATH')
    return path


def write_new(path, value):
    raw = (json.dumps(value, indent=2, sort_keys=True) + '\n').encode()
    with Path(path).open('xb') as stream:
        stream.write(raw)
        stream.flush()
        os.fsync(stream.fileno())


def load_spec(expected_hash):
    raw = file_pin(SPEC, expected_hash)
    spec = load_json(raw)
    require(spec.get('launchEnabled') is True and spec.get('authorization') == 'ROOT_APPROVED_ONE_ORIGINAL_FULL179', 'LAUNCH_HELD')
    require(spec.get('source') == {'path': str(SOURCE), 'sha': SOURCE_SHA, 'tree': SOURCE_TREE,
                                 'branch': SOURCE_BRANCH, 'origin': ORIGIN}, 'SOURCE_SPEC_MISMATCH')
    require(spec.get('command') == COMMAND and spec.get('output') == str(OUTPUT) and spec.get('budgets') == BUDGETS, 'COMMAND_OR_BUDGET_CHANGED')
    require(spec.get('unit') == UNIT and spec.get('runnerUid') == 1001, 'SERVICE_IDENTITY_CHANGED')
    require(spec.get('publicFixtureSigning') == {'K05OriginalOnce': True, 'retryLimit': 0}, 'SIGNING_SCOPE_CHANGED')
    for name in ('supervise.py', 'launch_reviewed.py', 'source-witness.json'):
        file_pin(PLAN / name, spec['proposalFiles'][name])
    return spec


def git(root, *args):
    env = {'HOME': '/home/ubuntu', 'USER': 'ubuntu', 'LOGNAME': 'ubuntu',
           'PATH': '/usr/local/bin:/usr/bin:/bin', 'LANG': 'C.UTF-8',
           'GIT_TERMINAL_PROMPT': '0', 'GIT_OPTIONAL_LOCKS': '0', 'GIT_NO_REPLACE_OBJECTS': '1'}
    run = subprocess.run(['/usr/bin/git', '--no-replace-objects', '-C', str(root), *args],
                         capture_output=True, timeout=20, env=env)
    require(run.returncode == 0, 'GIT_READ_FAILED')
    return run.stdout.decode().strip()


def repo_snapshot(root, head, tree, origin, branch=None, ordinary=False):
    require(root.is_dir() and not root.is_symlink(), 'SOURCE_DIRECTORY_REQUIRED')
    if ordinary:
        require((root / '.git').is_dir() and not (root / '.git').is_symlink(), 'ORDINARY_GIT_REQUIRED')
    common = Path(git(root, 'rev-parse', '--git-common-dir'))
    if not common.is_absolute():
        common = root / common
    require(not (common / 'objects/info/alternates').exists() and not (common / 'info/grafts').exists(), 'BORROWED_OR_REPLACED_GIT')
    require(not git(root, 'for-each-ref', '--format=%(refname)', 'refs/replace'), 'REPLACEMENT_REFS')
    result = {'head': git(root, 'rev-parse', 'HEAD'), 'tree': git(root, 'rev-parse', 'HEAD^{tree}'),
              'origin': git(root, 'remote', 'get-url', 'origin'),
              'branch': git(root, 'branch', '--show-current'),
              'statusSha256': sha(git(root, 'status', '--porcelain=v1', '--untracked-files=all').encode())}
    require(result['head'] == head and result['tree'] == tree and result['origin'] == origin and
            result['statusSha256'] == sha(b''), 'SOURCE_IDENTITY_OR_CLEANLINESS')
    if branch is not None:
        require(result['branch'] == branch and git(root, 'rev-parse', '--is-shallow-repository') == 'false', 'SOURCE_BRANCH_OR_HISTORY')
    return result


def source_snapshot(spec):
    value = {
        'implementation': repo_snapshot(SOURCE, SOURCE_SHA, SOURCE_TREE, ORIGIN, SOURCE_BRANCH, True),
        'documentation': repo_snapshot(DOCS, DOCS_SHA, DOCS_TREE, 'https://github.com/davidpovarsky/cherrilang.org.git'),
        'compilerSource': repo_snapshot(COMPILER_SOURCE, COMPILER_SHA, COMPILER_TREE, ORIGIN, ordinary=True),
        'binaryHashes': {}, 'sourceFiles': {}, 'contractIds': []}
    for name, expected in BINARIES.items():
        value['binaryHashes'][name] = sha(file_pin(PAIR / name, expected, 64 * 1024 * 1024))
    witness = load_json(file_pin(PLAN / 'source-witness.json', spec['proposalFiles']['source-witness.json']))
    require(witness['source'] == SOURCE_SHA and witness['tree'] == SOURCE_TREE, 'SOURCE_WITNESS_BINDING')
    for row in witness['files']:
        path = Path(row['path'])
        require(not path.is_absolute() and '..' not in path.parts, 'SOURCE_WITNESS_PATH')
        value['sourceFiles'][row['path']] = sha(file_pin(SOURCE / path, row['sha256']))
    for path, (expected, key, count) in CONTRACTS.items():
        contract = load_json(file_pin(SOURCE / path, expected))
        rows = contract[key]
        require(len(rows) == count, 'CONTRACT_COUNT_CHANGED')
        value['contractIds'].extend(x['id'] for x in rows)
    value['contractIds'].sort()
    require(len(value['contractIds']) == len(set(value['contractIds'])) == 179, 'CONTRACT_IDENTITY_CHANGED')
    return value


def utc(value):
    require(isinstance(value, str) and value.endswith('Z'), 'UTC_OBSERVATION_REQUIRED')
    return dt.datetime.fromisoformat(value[:-1] + '+00:00')


def ai_inputs(spec):
    ai = spec['ai']
    require(ai.get('rootReviewedCompletedOriginals') is True, 'LIVE_AI_COMPLETION_NOT_REVIEWED')
    bundle = path_pin(ai['bundlePath'])
    require(bundle.is_dir(), 'BUNDLE_DIRECTORY_REQUIRED')
    manifest = load_json(file_pin(bundle / 'manifest.json', ai['manifestSha256']))
    report = load_json(file_pin(bundle / 'report.json', ai['reportSha256']))
    require(manifest.get('implementation_sha') == SOURCE_SHA and manifest.get('expected_outcomes') == 12 and
            manifest.get('retry_limit') == 0, 'SEALED_AI_SOURCE_OR_INVENTORY')
    require(report.get('mode') == 'external' and report.get('status') == 'EVALUATION_COMPLETED' and
            report.get('ai_quality_measured') is True, 'AI_OUTCOMES_INCOMPLETE')
    rows = report.get('records')
    require(isinstance(rows, list) and len(rows) == 12 and
            {(x.get('task_id'), x.get('arm')) for x in rows} == {(task, arm) for task in TASKS for arm in ('legacy', 'v2')} and
            all(x.get('status') in ('MEASURED', 'MEASURED_PROVIDER_FAILURE') for x in rows), 'AI_OUTCOME_INVENTORY')
    trusted = []
    for label in ('before', 'after'):
        entry = ai[label]
        path = path_pin(entry['path'])
        require(not path.is_relative_to(bundle), 'ASANA_ORIGINAL_INSIDE_BUNDLE')
        file_pin(path, entry['sha256'], 4 * 1024 * 1024)
        utc(entry['observedUTC'])
        trusted.append(path.stat())
    require((trusted[0].st_dev, trusted[0].st_ino) != (trusted[1].st_dev, trusted[1].st_ino) and
            utc(ai['before']['observedUTC']) < utc(ai['after']['observedUTC']), 'ASANA_ORIGINALS_NOT_INDEPENDENT')
    require(re.fullmatch(r'[0-9]{10,30}', str(ai['anchorStoryId'])) and
            re.fullmatch(r'[0-9a-f]{64}', str(ai['anchorTextSha256'])), 'ASANA_ANCHOR_PIN_REQUIRED')
    completion = ai['completionReceipt']
    file_pin(path_pin(completion['path']), completion['sha256'])
    return {'manifestSha256': ai['manifestSha256'], 'reportSha256': ai['reportSha256'],
            'reportedCompletedOutcomes': 12, 'beforeSha256': ai['before']['sha256'],
            'afterSha256': ai['after']['sha256'], 'canonicalAcceptancePerformedBySupervisor': False}


def ci_inputs(spec):
    bindings = spec['workflowBindings']
    require(isinstance(bindings, list) and len(bindings) == 4 and
            {x['workflowName'] for x in bindings} == set(WORKFLOWS), 'FOUR_WORKFLOW_BINDINGS_REQUIRED')
    result = []
    for row in bindings:
        raw = file_pin(path_pin(row['originalPath']), row['sha256'])
        run = load_json(raw)
        require(run.get('repository', {}).get('full_name') == 'davidpovarsky/cherri' and
                run.get('name') == row['workflowName'] and str(run.get('id')) == str(row['runId']) and
                run.get('head_sha') == SOURCE_SHA and run.get('head_branch') == SOURCE_BRANCH and
                run.get('status') == row['status'] == 'completed' and
                run.get('conclusion') == row['conclusion'] and bool(row['conclusion']) and
                run.get('event') == row['event'] and bool(row['event']) and
                type(row['runAttempt']) is int and row['runAttempt'] >= 1 and
                run.get('run_attempt') == row['runAttempt'], 'WORKFLOW_ORIGINAL_BINDING')
        result.append({key: row[key] for key in ('workflowName', 'runId', 'runAttempt', 'event', 'status', 'conclusion', 'sha256')})
    return result


def capacity():
    values = {}
    for name, path in (('development', '/mnt/development'), ('tmpfs', '/dev/shm'), ('root', '/')):
        info = os.statvfs(path)
        values[name + 'AvailableBytes'] = info.f_bavail * info.f_frsize
    for line in Path('/proc/meminfo').read_text().splitlines():
        if line.startswith(('MemTotal:', 'MemAvailable:')):
            key, number, _ = line.split()
            values[key.rstrip(':') + 'Bytes'] = int(number) * 1024
    return values


def environment(spec):
    env = {'HOME': '/home/ubuntu', 'USER': 'ubuntu', 'LOGNAME': 'ubuntu',
           'PATH': '/usr/local/bin:/usr/local/go/bin:/usr/bin:/bin:/usr/sbin:/sbin',
           'LANG': 'C.UTF-8', 'LC_ALL': 'C.UTF-8', 'GOTOOLCHAIN': 'go1.24.13',
           'GOMAXPROCS': '2', 'GOFLAGS': '-p=2',
           'GOMODCACHE': '/mnt/development/tooling/cherri-go/mod',
           'GOCACHE': '/dev/shm/cherri-p10r15-runtime-20261123/go-build-cache',
           'GOTMPDIR': '/dev/shm/cherri-p10r15-runtime-20261123/go-tmp-ubuntu',
           'TMPDIR': '/dev/shm/cherri-p10r15-runtime-20261123/go-tmp-ubuntu',
           'PYTHONDONTWRITEBYTECODE': '1', 'CHERRI_ACCEPTANCE_PYTHON': '/usr/bin/python3',
           'GIT_OPTIONAL_LOCKS': '0', 'GIT_NO_REPLACE_OBJECTS': '1', 'GIT_TERMINAL_PROMPT': '0',
           'GH_PROMPT_DISABLED': '1',
           'CHERRI_DOCS_DIR': str(DOCS), 'CHERRI_D03_DOCS_REPO': str(DOCS),
           'CHERRI_DOCUMENTATION_SOURCE_ROOT': str(COMPILER_SOURCE),
           'CHERRI_DOCUMENTATION_COMPILER': str(PAIR / 'cherri'),
           'CHERRI_LOCAL_ACCEPTANCE_OUT': str(OUTPUT), 'CHERRI_K05_EVIDENCE_DIR': str(OUTPUT / 'k05'),
           'CHERRI_K05_LIVE_SIGNING': '1'}
    ai = spec['ai']
    env.update(CHERRI_AI_EVALUATION_BUNDLE=ai['bundlePath'], CHERRI_AI_EVALUATION_SOURCE_ROOT=str(SOURCE),
               CHERRI_AI_EVALUATION_ANCHOR_STORY_ID=str(ai['anchorStoryId']),
               CHERRI_AI_EVALUATION_ANCHOR_TEXT_SHA256=ai['anchorTextSha256'])
    for label in ('before', 'after'):
        prefix = 'CHERRI_AI_EVALUATION_ASANA_' + label.upper()
        env[prefix] = ai[label]['path']
        env[prefix + '_SHA256'] = ai[label]['sha256']
        env[prefix + '_OBSERVED_UTC'] = ai[label]['observedUTC']
    # No inherited provider credentials, generation authorization, private-corpus
    # opt-in, alternate signing endpoint, or unrelated CHERRI variables.
    # gh uses the existing ubuntu HOME authentication; no credential is copied.
    return env


def preflight(spec):
    require(os.getuid() == 1001 and os.getgid() == 1001 and os.path.expanduser('~') == '/home/ubuntu', 'REAL_UBUNTU_IDENTITY_REQUIRED')
    require(any(line.rsplit('/', 1)[-1] == UNIT for line in Path('/proc/self/cgroup').read_text().splitlines()),
            'EXPECTED_HOST_SERVICE_REQUIRED')
    values = capacity()
    review = spec['capacityReview']
    for key, available in (('minimumPersistentFreeBytes', 'developmentAvailableBytes'),
                           ('minimumTmpfsFreeBytes', 'tmpfsAvailableBytes'),
                           ('minimumMemAvailableBytes', 'MemAvailableBytes')):
        require(type(review[key]) is int and review[key] > 0 and values[available] >= review[key], 'CAPACITY_REVIEW_REQUIRED')
    return {'recordedAt': now(), 'capacity': values, 'sources': source_snapshot(spec),
            'aiReadiness': ai_inputs(spec), 'workflowOriginals': ci_inputs(spec)}


def report_summary(expected_ids):
    path = OUTPUT / 'acceptance-report.json'
    if not path.exists():
        return {'present': False, 'integrityVerified': False}
    raw = regular(path, 32 * 1024 * 1024)
    report = load_json(raw)
    rows = report.get('results', [])
    ids = [x.get('id') for x in rows]
    statuses = dict(collections.Counter(x.get('status', 'MISSING_STATUS') for x in rows))
    exact = len(rows) == len(set(ids)) == 179 and sorted(ids) == expected_ids
    passed = sum(x.get('passed') is True for x in rows)
    failed = statuses.get('FAILED', 0)
    counters = (report.get('total_cases') == 179 and report.get('passed_cases') == passed and
                report.get('failed_cases') == failed and report.get('skipped_cases') == 179 - passed - failed)
    companions = {name: (OUTPUT / name).is_file() for name in
                  ('acceptance-local-evidence.json', 'acceptance-local-observations.json',
                   'acceptance-execution-audit.json', 'local-runner-evidence.json', 'requirements-evidence-map.json')}
    return {'present': True, 'sha256': sha(raw), 'bytes': len(raw), 'uniqueIds': len(set(ids)),
            'rowCount': len(rows), 'statusCounts': statuses, 'originalClosureComplete': report.get('closure_complete'),
            'originalPassedCases': report.get('passed_cases'), 'originalFailedCases': report.get('failed_cases'),
            'originalSkippedCases': report.get('skipped_cases'), 'originalPhase': report.get('phase'),
            'exact179Ids': exact, 'declaredCountersMatch': counters, 'companionArtifactsPresent': companions,
            'integrityVerified': exact and counters and report.get('phase') == 'local' and all(companions.values()),
            'formalAcceptanceGrantedBySupervisor': False}


def run(spec_hash):
    spec = load_spec(spec_hash)
    require(OUTPUT.is_dir() and not (OUTPUT / 'producer.start.original.json').exists(), 'ATTEMPT_NOT_FRESH')
    require(not any(OUTPUT.glob('acceptance-*.json')) and not (OUTPUT / 'k05').exists(), 'PRODUCER_OUTPUT_NOT_FRESH')
    start = {'recordedAt': now(), 'pid': os.getpid(), 'specSha256': spec_hash,
             'sourceSha': SOURCE_SHA, 'command': COMMAND, 'budgets': BUDGETS,
             'status': 'PREFLIGHT_ONLY', 'producerStarted': False}
    write_new(OUTPUT / 'supervisor.start.original.json', start)
    receipt = dict(start)
    timer = time.monotonic()
    process = None
    signals = []

    def interrupt(signum, _frame):
        signals.append(signum)
        if process is not None and process.poll() is None:
            try:
                os.killpg(process.pid, signal.SIGTERM)
            except ProcessLookupError:
                pass

    signal.signal(signal.SIGTERM, interrupt)
    signal.signal(signal.SIGINT, interrupt)
    try:
        before = preflight(spec)
        write_new(OUTPUT / 'preflight.original.json', before)
        env = environment(spec)
        version = subprocess.run(['/usr/local/bin/go', 'version'], env=env, cwd=SOURCE,
                                 capture_output=True, text=True, timeout=20)
        require(version.returncode == 0 and version.stdout.strip() == 'go version go1.24.13 linux/arm64', 'ORIGINAL_GO_TOOLCHAIN_REQUIRED')
        require(not signals, 'HOST_STOP_BEFORE_PRODUCER')
        require(time.monotonic() - timer + BUDGETS['producerSeconds'] +
                BUDGETS['producerTerminationGraceSeconds'] + 30 < BUDGETS['outerRunSeconds'],
                'INSUFFICIENT_OUTER_TIME_FOR_UNCHANGED_PRODUCER_BUDGET')
        with (OUTPUT / 'producer.stdout.original').open('xb') as stdout, (OUTPUT / 'producer.stderr.original').open('xb') as stderr:
            process = subprocess.Popen(COMMAND, cwd=SOURCE, env=env, stdout=stdout, stderr=stderr, start_new_session=True)
            receipt.update(producerStarted=True, producerPid=process.pid, producerStartedAt=now())
            write_new(OUTPUT / 'producer.start.original.json', {**receipt, 'toolchain': version.stdout.strip()})
            exit_code = process.wait()
        receipt.update(producerExitCode=exit_code, producerFinishedAt=now(), status='PRODUCER_TERMINAL',
                       gnuTimeoutExit124=exit_code == 124)
        try:
            after = source_snapshot(spec)
            receipt['sourcesUnchanged'] = after == before['sources']
            write_new(OUTPUT / 'source-after.original.json', after)
        except (GuardError, OSError, ValueError, subprocess.SubprocessError) as error:
            receipt.update(sourcesUnchanged=False, sourceAfterError=error.args[0] if isinstance(error, GuardError) else type(error).__name__)
        try:
            receipt['report'] = report_summary(before['sources']['contractIds'])
        except (GuardError, OSError, ValueError, TypeError, KeyError) as error:
            receipt['report'] = {'integrityVerified': False, 'error': error.args[0] if isinstance(error, GuardError) else type(error).__name__}
    except (GuardError, OSError, ValueError, TypeError, KeyError, subprocess.SubprocessError) as error:
        receipt.update(status='SUPERVISOR_ERROR', error=error.args[0] if isinstance(error, GuardError) else type(error).__name__)
    finally:
        if process is not None and process.poll() is None:
            receipt['ownedProcessCleanupRequired'] = True
            try:
                os.killpg(process.pid, signal.SIGTERM)
                process.wait(timeout=30)
            except subprocess.TimeoutExpired:
                os.killpg(process.pid, signal.SIGKILL)
                process.wait(timeout=5)
            except ProcessLookupError:
                pass
            receipt['producerExitCode'] = process.poll()
        receipt.update(finishedAt=now(), elapsedSeconds=round(time.monotonic() - timer, 6),
                       hostSignalsObserved=signals, capacityAfter=capacity(), formalAcceptanceGranted=False)
        write_new(OUTPUT / 'producer.result.original.json', receipt)
    print(json.dumps({'status': receipt['status'], 'producerStarted': receipt['producerStarted'],
                      'producerExitCode': receipt.get('producerExitCode'), 'sourceSha': SOURCE_SHA,
                      'formalAcceptanceGranted': False}, sort_keys=True))
    if receipt['status'] != 'PRODUCER_TERMINAL' or not receipt.get('sourcesUnchanged') or not receipt.get('report', {}).get('integrityVerified'):
        return 1
    return receipt['producerExitCode'] if 0 <= receipt['producerExitCode'] <= 255 else 1


def host_stop(spec_hash):
    # Runs only as ExecStopPost after systemd has stopped remaining service
    # processes. It never launches a producer or performs another acceptance.
    require(os.getuid() == 1001, 'UBUNTU_STOP_RECEIPT_REQUIRED')
    file_pin(SPEC, spec_hash)
    rows = []
    for path in sorted(OUTPUT.rglob('*')):
        if path.is_symlink():
            rows.append({'path': str(path.relative_to(OUTPUT)), 'type': 'symlink', 'hashed': False})
        elif path.is_file() and path not in (OUTPUT / 'host-stop.original.json', OUTPUT / 'artifacts-final.original.json'):
            digest = hashlib.sha256()
            with path.open('rb') as stream:
                while block := stream.read(1024 * 1024):
                    digest.update(block)
            rows.append({'path': str(path.relative_to(OUTPUT)), 'bytes': path.stat().st_size, 'sha256': digest.hexdigest()})
    write_new(OUTPUT / 'artifacts-final.original.json', {'recordedAt': now(), 'members': rows})
    write_new(OUTPUT / 'host-stop.original.json', {'recordedAt': now(), 'sourceSha': SOURCE_SHA,
        'specSha256': spec_hash, 'serviceResult': os.environ.get('SERVICE_RESULT'),
        'exitCodeKind': os.environ.get('EXIT_CODE'), 'exitStatus': os.environ.get('EXIT_STATUS'),
        'capacity': capacity(), 'memberCount': len(rows), 'formalAcceptanceGranted': False})
    return 0


if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('mode', choices=('held', 'run-reviewed', 'host-stop'), nargs='?', default='held')
    parser.add_argument('--spec-sha256')
    args = parser.parse_args()
    if args.mode == 'held':
        print('HELD: no producer or service launched; explicit reviewed specification required.')
        raise SystemExit(0)
    try:
        require(args.spec_sha256 is not None, 'EXPLICIT_SPEC_HASH_REQUIRED')
        raise SystemExit(run(args.spec_sha256) if args.mode == 'run-reviewed' else host_stop(args.spec_sha256))
    except (GuardError, OSError, ValueError, TypeError, KeyError) as error:
        print(json.dumps({'status': 'HELD_OR_REJECTED', 'error': error.args[0] if isinstance(error, GuardError) else type(error).__name__}))
        raise SystemExit(1)
