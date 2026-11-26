"""Compile the public Q test package once; execute no private test or corpus.

This is a new precompile phase, not one of the five historical go-test runs.
The original private execution bounds and strict auditor remain separate.
"""
from pathlib import Path
import ast
import hashlib
import importlib.util
import json
import os
import stat
import struct
import sys
import time

P = Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location('s4_precompile_binding', P / 'hosted_source.py')
fixed = importlib.util.module_from_spec(spec)
sys.modules[spec.name] = fixed
spec.loader.exec_module(fixed)

TESTS = (
    ('00-p12-par', 'TestP12_PAR_12_CorpusComplexParamsAndZeroStage3Gate', 'p12_complex_params_test.go',
     '86172facc402af313872a2412e6dcd2d1dbd50f9f125d88a102acf21088db1ed'),
    ('01-p13', 'TestP13_CorpusFidelityAndSourceSizeAudit', 'p13_fidelity_promotion_test.go',
     '8163a5d80ef4ee23423fff2eef54599b5289c73e44337b02e006b2e47b9d6279'),
    ('02-p10-measure', 'TestP10R10CorpusMeasure', 'p10r10_corpus_test.go',
     'a6188a41ff23fd43f25308b8547391d9186d3c1b6ad92dbaea9ae514d865a761'),
    ('03-p11-fresh38x2', 'TestP11_WIRE_01_Fresh38x2NativeActionWire', 'p11_wire_fidelity_test.go',
     'c4ab56f1ee4f1eea58e4075da0965f525f85e8f1efc30452d2dc3703b34b8f67'),
    ('04-p12-lifecycle', 'TestP12_BASE_02_LifecycleStagesAndReconcileCorpus', 'p12_baseline_truth_test.go',
     'eb8b70716337cba945409df4d60edb23063c2998330bc15da8c1ec8e08c87658'),
)
ORIGINAL_AUDITOR_SHA = 'f186bc74fb355aa0334ca60696e387a2a55deea0ab5ffb0230a58285bc66ce77'
ORIGINAL_OWNER_SHA = '8452dcf07565838816122a027c9800998b3b67ec6b917b5ac66fe8fd38625e81'
TEST2JSON_SHA = '45a0a116c4973aefff487a873f5067f4442889810bcffc79f9300b62e239a48d'
GO_TEST_SOURCE_SHA = 'be70c4d5f1576ca5563712cbdbe381095894aff08a13bdaff81b1b577f553c35'
GO_TESTFLAGS_SOURCE_SHA = '328a02ccdca36d750175bb017f13f08a5750d96ae2889b152a838cf9a62c91fc'
GO_TEST2JSON_SOURCE_SHA = '63c597b54f6be122114c8d4fcb5f80ffca02678780ba784ada5f4441a0ff1788'
PACKAGE = 'github.com/electrikmilk/cherri'
GO_ENV_KEYS = ('GOVERSION', 'GOOS', 'GOARCH', 'GOROOT', 'GOTOOLDIR', 'GOTOOLCHAIN',
               'GOMODCACHE', 'GOCACHE', 'GOTMPDIR', 'GOFLAGS', 'GOEXPERIMENT', 'GOENV',
               'CGO_ENABLED', 'CC', 'CGO_CFLAGS', 'CGO_CPPFLAGS', 'CGO_CXXFLAGS', 'CGO_LDFLAGS')


def binary_identity(owner, path):
    path = Path(path)
    info = path.lstat()
    fixed.require(stat.S_ISREG(info.st_mode) and not path.is_symlink() and
                  info.st_uid == os.getuid() and bool(info.st_mode & 0o111) and
                  0 < info.st_size <= 128 * 1024 * 1024, 'OWNED_REGULAR_TEST_BINARY')
    raw = owner.read(path)
    fixed.require(raw[:6] == b'\x7fELF\x02\x01' and len(raw) >= 64 and
                  struct.unpack_from('<H', raw, 18)[0] == 183, 'ELF64_LITTLE_ENDIAN_ARM64')
    after = path.lstat()
    keys = ('st_dev', 'st_ino', 'st_mode', 'st_uid', 'st_gid', 'st_size', 'st_mtime_ns', 'st_ctime_ns')
    fixed.require(all(getattr(info, k) == getattr(after, k) for k in keys), 'STABLE_TEST_BINARY')
    return {'path': str(path), 'bytes': len(raw), 'sha256': fixed.sha(raw),
            'mode': oct(stat.S_IMODE(info.st_mode)), 'uid': info.st_uid, 'gid': info.st_gid,
            'elfClass': 64, 'elfByteOrder': 'little', 'elfMachine': 183}


def main():
    fixed.require(len(sys.argv) == 1, 'FIXED_PUBLIC_PRECOMPILE_ROUTE')
    os.umask(0o077)
    out = fixed.EVIDENCE / 'precompile'
    out.mkdir(mode=0o700)
    products = fixed.BASE / 'products'
    products.mkdir(mode=0o700)
    binary = products / 'cherri.test'
    result = {'schema': 'cherri-public-s4-precompile/1', 'status': 'STARTED',
              'source': fixed.SOURCE, 'tree': fixed.TREE, 'controller': fixed.CONTEXT['controllerSha'],
              'package': PACKAGE, 'originalFiveTests': TESTS,
              'compileOnly': True, 'testMethodsExecuted': 0, 'privateInputsPresent': False,
              'privateCorpusTransferred': False, 'literalOriginalGoTestInvocation': False,
              'oracleGoExecuted': False, 'nativeAcceptance': False, 'formal179Acceptance': False,
              'originalAuditorSHA256': ORIGINAL_AUDITOR_SHA, 'originalSourceOwnerSHA256': ORIGINAL_OWNER_SHA,
              'wholeOwnerSeconds': 970, 'precompileChildSeconds': 900,
              'futurePrivateBounds': {'testSeconds': 900, 'driverSeconds': 930, 'wholeOwnerSeconds': 970},
              'historicalEffectiveCGORecorded': False, 'declaredCGOPolicy': 'UNSET',
              'buildFormDifference': 'go test -c retains DWARF; ordinary go test omits it when no retained binary is requested. No trimpath/ldflags/race/coverage/tag override is added.',
              'failures': [], 'processes': []}
    before = after = controller_before = controller_after = None
    with fixed.owned.WholeBudget(970, cleanup_seconds=8, final_reserve=35) as budget:
        owner = fixed.SourceOwner({'worktree': str(fixed.WORKTREE), 'sourceRole': 's4-precompile'}, out, budget)
        fixed.owned.original(out / 'runtime-binding.original.json', fixed.runtime_identity())
        for name in ('build.py', 'hosted_source.py', 'owned_source_process.py'):
            fixed.owned.original(out / name, owner.read(P / name))
        try:
            prerequisite = json.loads(owner.read(fixed.EVIDENCE / 'prerequisite/result.original.json'))
            fixed.require(prerequisite['status'] == 'PASS' and prerequisite['source'] == fixed.SOURCE and
                          prerequisite['tree'] == fixed.TREE and prerequisite['sourceUnchanged'] is True and
                          prerequisite['controllerUnchanged'] is True and prerequisite['goBuildOrTestExecuted'] is False,
                          'SUCCESSFUL_EXACT_PUBLIC_PREREQUISITES')
            fixed.require(not any(k.startswith('CHERRI_') for k in owner.env) and
                          all(k not in owner.env for k in ('CGO_ENABLED', 'GOOS', 'GOARCH', 'GOEXPERIMENT', 'GOENV')),
                          'ORIGINAL_UNSET_POLICY_AND_NO_PRIVATE_OPT_IN')
            fixed.require(owner.env['GOFLAGS'] == '-p=2 -mod=readonly' and owner.env['GOMAXPROCS'] == '2' and
                          owner.env['GOTOOLCHAIN'] == 'go1.24.13' and owner.env['GOPROXY'] == owner.env['GOSUMDB'] == 'off',
                          'ORIGINAL_DECLARED_BUILD_POLICY')
            before = owner.snapshot()
            fixed.owned.original(out / 'source-before.original.json', before)
            fixed.require((before['head'], before['tree'], before['branch'], before['status']) ==
                          (fixed.SOURCE, fixed.TREE, fixed.SOURCE_BRANCH, ''), 'CLEAN_EXACT_Q_SOURCE')
            controller_before = fixed.controller_snapshot(owner)
            fixed.owned.original(out / 'controller-before.original.json', controller_before)
            for mode, name, relative, digest in TESTS:
                data = owner.read(fixed.WORKTREE / relative)
                fixed.require(fixed.sha(data) == digest and ('func ' + name + '(').encode() in data,
                              'EXACT_ORIGINAL_TEST_FILE:' + mode)
            auditor = owner.read(fixed.WORKTREE / 'docs/language-v2/evidence/p15-s4-new-source-20261123/auditor-source.original.py')
            fixed.require(fixed.sha(auditor) == ORIGINAL_AUDITOR_SHA, 'UNMODIFIED_ORIGINAL_STRICT_AUDITOR')
            fixed.owned.original(out / 'auditor-source.original.py', auditor)
            fixed.require(fixed.sha(owner.read(fixed.GO)) == fixed.GO_SHA256, 'ACTUAL_PINNED_GO_BINARY')
            goroot = fixed.GO.parent.parent
            tool = goroot / 'pkg/tool/linux_arm64/test2json'
            fixed.require(fixed.sha(owner.read(tool)) == TEST2JSON_SHA, 'PINNED_STANDALONE_TEST2JSON')
            for name, digest in (('test.go', GO_TEST_SOURCE_SHA), ('testflag.go', GO_TESTFLAGS_SOURCE_SHA)):
                fixed.require(fixed.sha(owner.read(goroot / 'src/cmd/go/internal/test' / name)) == digest, 'PINNED_GO_TEST_RUNTIME_SOURCE')
            fixed.require(fixed.sha(owner.read(goroot / 'src/cmd/test2json/main.go')) == GO_TEST2JSON_SOURCE_SHA, 'PINNED_CONVERTER_SOURCE')
            _, admission_path, errors = owner.observe_concurrent_owner()
            admission = json.loads(owner.read(admission_path))
            fixed.require(not owner.read(errors) and admission['admitted'] is True and admission['blockedPids'] == [], 'EXCLUSIVE_HOSTED_BUILD_ADMISSION')
            observed = fixed.free()
            fixed.require(all(observed[k] >= v for k, v in fixed.ADMISSION_FLOORS.items()), 'ORIGINAL_S4_CAPACITY_FLOORS')
            fixed.owned.original(out / 'capacity.original.json', {'observed': observed, 'admission': fixed.ADMISSION_FLOORS, 'live': fixed.LIVE_FLOORS})
            row, stdout, _ = owner.command('effective-go-environment', [fixed.GO, 'env', '-json', *GO_ENV_KEYS], seconds=30)
            result['processes'].append(row)
            environment = json.loads(owner.read(stdout))
            fixed.require((environment['GOVERSION'], environment['GOOS'], environment['GOARCH']) ==
                          ('go1.24.13', 'linux', 'arm64') and environment['CGO_ENABLED'] in ('0', '1') and
                          environment['GOFLAGS'] == '-p=2 -mod=readonly', 'ACTUAL_NATIVE_BUILD_SETTINGS')
            fixed.owned.original(out / 'go-env.original.json', owner.read(stdout))
            result['effectiveBuildSettings'] = environment
            result['buildHostLibc'] = os.confstr('CS_GNU_LIBC_VERSION')
            args = [str(fixed.GO), 'test', '-c', '-o', str(binary), '.']
            intent = {'schema': 'cherri-public-s4-precompile-intent/1', 'argv': args,
                      'package': PACKAGE, 'source': fixed.SOURCE, 'tree': fixed.TREE,
                      'declaredEnvironment': owner.env, 'precompileChildSeconds': 900, 'wholeOwnerSeconds': 970,
                      'testsExecuted': 0, 'privateInputsTransferred': False,
                      'defaultVetRetained': True, 'futureOracleEffectiveTestFlags':
                      ['-test.paniconexit0', '-test.v=test2json', '-test.count=1', '-test.timeout=15m', '-test.run=^<one exact original parent>$']}
            fixed.owned.original(out / 'intent.original.json', intent)
            row, _, _ = owner.command('compile-original-public-test-package', args, seconds=900, require_zero=False)
            result['processes'].append(row)
            fixed.owned.require_success(row)
            result['binary'] = binary_identity(owner, binary)
            row, stdout, _ = owner.command('actual-test-binary-build-info', [fixed.GO, 'version', '-m', binary], seconds=10)
            result['processes'].append(row)
            build_info = owner.read(stdout)
            fixed.require(b'go1.24.13' in build_info, 'ACTUAL_TEST_BINARY_TOOLCHAIN_VERSION')
            fixed.owned.original(out / 'binary-build-info.original.txt', build_info)
            row, stdout, _ = owner.command('actual-test-binary-build-id', [fixed.GO, 'tool', 'buildid', binary], seconds=10)
            result['processes'].append(row)
            build_id = owner.read(stdout)
            fixed.require(0 < len(build_id.strip()) <= 1024, 'ACTUAL_TEST_BINARY_BUILD_ID')
            fixed.owned.original(out / 'binary-build-id.original.txt', build_id)
            result['test2json'] = {'relativePath': 'pkg/tool/linux_arm64/test2json', 'sha256': TEST2JSON_SHA,
                                   'bytes': tool.stat().st_size, 'sourceSHA256': GO_TEST2JSON_SOURCE_SHA}
            result['status'] = 'PASS_PUBLIC_PRECOMPILE_ONLY'
        except BaseException as error:
            result['failures'].append({'stage': 'precompile', 'type': type(error).__name__, 'message': str(error)})
        finally:
            with budget.finalizing():
                try:
                    after = owner.snapshot(finalizing=True)
                    fixed.owned.original(out / 'source-after.original.json', after)
                    fixed.require(before == after and before is not None, 'SOURCE_UNCHANGED_AFTER_PRECOMPILE')
                    controller_after = fixed.controller_snapshot(owner)
                    fixed.owned.original(out / 'controller-after.original.json', controller_after)
                    fixed.require(controller_before == controller_after and controller_before is not None, 'CONTROLLER_UNCHANGED')
                    if 'binary' in result:
                        fixed.require(binary_identity(owner, binary) == result['binary'], 'BINARY_UNCHANGED_AT_CLOSING_GUARD')
                except BaseException as error:
                    result['failures'].append({'stage': 'post-source', 'type': type(error).__name__, 'message': str(error)})
                if budget.cancel_signal is not None or time.monotonic() >= budget.deadline:
                    result['failures'].append({'stage': 'owner', 'type': 'CANCELLED_OR_DEADLINE', 'signal': budget.cancel_signal})
                result['sourceUnchanged'] = before is not None and before == after
                result['controllerUnchanged'] = controller_before is not None and controller_before == controller_after
                result['minimumFree'] = owner.minimum
                result['elapsedSeconds'] = time.monotonic() - budget.started
                if result['failures'] or result['status'] != 'PASS_PUBLIC_PRECOMPILE_ONLY':
                    result['status'] = 'FAIL'
                with budget.defer():
                    pin = fixed.owned.original(out / 'result.original.json', result)
    print(json.dumps({'path': str(out / 'result.original.json'), 'sha256': pin['sha256'], 'status': result['status']}), flush=True)
    return 0 if result['status'] == 'PASS_PUBLIC_PRECOMPILE_ONLY' else 1


if __name__ == '__main__':
    raise SystemExit(main())
