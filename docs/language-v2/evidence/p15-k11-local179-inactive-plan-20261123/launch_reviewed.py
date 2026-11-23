#!/usr/bin/env python3
"""Root-only, create-once transient launch proposal; default performs no action."""
import argparse
import json
import os
from pathlib import Path
import subprocess
import sys

from supervise import PLAN, SPEC, SOURCE, OUTPUT, UNIT, load_spec, now, write_new, require


def command(spec_sha256):
    supervisor = str(PLAN / 'supervise.py')
    stop = ' '.join(['/usr/bin/timeout', '--signal=TERM', '--kill-after=1s', '29s',
                     '/usr/bin/python3', '-B', supervisor, 'host-stop', '--spec-sha256', spec_sha256])
    return ['/usr/bin/systemd-run', '--no-block', '--unit=' + UNIT, '--service-type=exec',
        '--property=User=ubuntu', '--property=Group=ubuntu', '--property=WorkingDirectory=' + str(SOURCE),
        '--setenv=HOME=/home/ubuntu', '--setenv=USER=ubuntu',
        '--setenv=LOGNAME=ubuntu', '--property=UMask=0077',
        '--property=RuntimeMaxSec=5340s', '--property=TimeoutStopSec=30s',
        '--property=RuntimeRandomizedExtraSec=0', '--property=TimeoutStartSec=30s',
        '--property=KillMode=control-group', '--property=KillSignal=SIGTERM',
        '--property=FinalKillSignal=SIGKILL', '--property=Restart=no',
        '--property=StandardOutput=append:' + str(OUTPUT / 'service.stdout.original'),
        '--property=StandardError=append:' + str(OUTPUT / 'service.stderr.original'),
        '--property=ExecStopPost=' + stop,
        '/usr/bin/python3', '-B', supervisor, 'run-reviewed', '--spec-sha256', spec_sha256]


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--launch-reviewed', action='store_true')
    parser.add_argument('--spec-sha256')
    args = parser.parse_args()
    if not args.launch_reviewed:
        print('HELD: no service, producer, or attempt directory created.')
        return 0
    require(os.getuid() == 0, 'ROOT_SYSTEM_SERVICE_DISPATCH_REQUIRED')
    require(args.spec_sha256 is not None, 'EXPLICIT_SPEC_HASH_REQUIRED')
    load_spec(args.spec_sha256)
    argv = command(args.spec_sha256)
    require(not OUTPUT.exists() and not OUTPUT.parent.exists(), 'EXCLUSIVE_NEW_OUTPUT_PARENT_REQUIRED')
    # An atomic claim is never removed on failure or transport ambiguity.
    # Root reconciles this receipt and the exact unit; there is no retry path.
    claim = PLAN / 'dispatch-claim.original.json'
    write_new(claim, {'createdAt': now(), 'specSha256': args.spec_sha256, 'unit': UNIT, 'command': argv})
    OUTPUT.mkdir(parents=True, mode=0o700, exist_ok=False)
    os.chown(OUTPUT.parent, 1001, 1001)
    os.chmod(OUTPUT.parent, 0o700)
    os.chown(OUTPUT, 1001, 1001)
    os.chmod(OUTPUT, 0o700)
    # Precreate only these fresh service streams for the ubuntu worker and its
    # post-stop inventory; O_EXCL also rejects any unexpected existing entry.
    for name in ('service.stdout.original', 'service.stderr.original'):
        fd = os.open(OUTPUT / name, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
        try:
            os.fchown(fd, 1001, 1001)
            os.fchmod(fd, 0o600)
            os.fsync(fd)
        finally:
            os.close(fd)
    with (OUTPUT / 'dispatch.stdout.original').open('xb') as stdout, (OUTPUT / 'dispatch.stderr.original').open('xb') as stderr:
        run = subprocess.run(argv, stdout=stdout, stderr=stderr, timeout=30)
    for name in ('dispatch.stdout.original', 'dispatch.stderr.original'):
        os.chown(OUTPUT / name, 1001, 1001)
    result = {'recordedAt': now(), 'unit': UNIT, 'systemdRunExitCode': run.returncode,
              'specSha256': args.spec_sha256, 'producerOutcome': 'NOT_INFERRED_FROM_DISPATCH', 'automaticRetryAllowed': False}
    write_new(PLAN / 'dispatch-result.original.json', result)
    print(json.dumps(result, sort_keys=True))
    return run.returncode


if __name__ == '__main__':
    raise SystemExit(main())
