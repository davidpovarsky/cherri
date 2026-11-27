"""Validate actual C2 originals, adopt seven bytes, and export a prerequisite bundle.

This phase has its own 240-second owner. It invokes no Go command and has no
write-capable token or remote push route. The adopted commit's sole parent is
the actual producer source, never the separate hosted controller commit.
"""
from pathlib import Path
import ast
import collections
import hashlib
import importlib.util
import json
import os
import pathlib
import re
import sys
import time

P = Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location("canonical_c2_hosted_adoption_protocol", P/"run.py")
protocol = importlib.util.module_from_spec(spec)
sys.modules[spec.name] = protocol
spec.loader.exec_module(protocol)
fixed = protocol.fixed
C1_GENERATOR_SHA256 = "99c33e84117cd9e048ef701aba24d1b95e75598479c50474e563cb8ead41e392"
C1_FINGERPRINT = "901ce789ace7104a0e1079ef71fc88ba117eee9bbb65ef38296252a55f084af0"
C2_FINGERPRINT = "c17891f7af525937fd1dff6c019c31997a92e778c7c167028f3aa6a6ac48d544"
ORIGINAL_INTAKE_SHA256 = "530f012492cd41fe7d8ebf89713ef6c83c6f64a87eaec67065f12340025cd942"


def c1_fingerprint(owner):
    raw = owner.read(fixed.WORKTREE/"tools/p10r3/generate_action_contract_coverage.py")
    fixed.require(fixed.sha(raw) == C1_GENERATOR_SHA256, "ORIGINAL_C1_FINGERPRINT_PRODUCER")
    nodes = [n for n in ast.parse(raw).body if isinstance(n, ast.Assign) and
             any(isinstance(t, ast.Name) and t.id == "SOURCE_GLOBS" for t in n.targets)]
    fixed.require(len(nodes) == 1, "C1_SOURCE_GLOBS_IDENTITY")
    patterns = ast.literal_eval(nodes[0].value)
    names = sorted({p.relative_to(fixed.WORKTREE).as_posix() for pattern in patterns
                    for p in fixed.WORKTREE.glob(pattern) if p.is_file()})
    material = b""
    for name in names:
        owner.budget.check()
        material += (name+":"+fixed.sha(owner.read(fixed.WORKTREE/name).replace(b"\r\n", b"\n"))+"\n").encode()
    fixed.require(not set(protocol.GENERATED).intersection(names), "C2_PRODUCTS_EXCLUDED_FROM_C1")
    return {"fingerprint": fixed.sha(material), "fileCount": len(names), "patterns": patterns}


def execution_receipts(owner):
    output = fixed.EVIDENCE/"execution"
    raw = owner.read(output/"result.original.json")
    result = json.loads(raw)
    fixed.require(result["status"] == "PASS" and result["source"] == protocol.HEAD and result["tree"] == protocol.TREE and
                  result["controllerSource"] == fixed.CONTEXT["controllerSha"] and result["controllerUnchanged"] and
                  result["sourceClosureUnchanged"] and result["dependenciesUnchanged"] and not result["failures"],
                  "ACTUAL_SUCCESSFUL_SOURCE_BOUND_EXECUTION")
    fixed.require(result["ownerSHA256"] == fixed.sha(owner.read(P/"run.py")) == fixed.sha(owner.read(output/"run.py")) and
                  result["sharedControllerSHA256"] == fixed.sha(owner.read(P/"hosted_source.py")) and
                  result["helperSHA256"] == fixed.HELPER_SHA256 and
                  result["originalOracleOwnerSHA256"] == fixed.ORIGINAL_C2_OWNER_SHA256, "ACTUAL_OWNER_PINS")
    fixed.require(len(result["processes"]) == 1 and tuple(result["declaredGeneratedPaths"]) == protocol.GENERATED,
                  "ONE_ORIGINAL_SEVEN_PRODUCT_PRODUCER")
    process = result["processes"][0]
    fixed.require(process["argv"] == protocol.COMMAND and process["status"] == "PASS" and process["exitCode"] == 0 and
                  not process["timedOut"] and not process["cancelled"] and not process["error"] and
                  process["cleanup"]["parentReaped"] and process["cleanup"]["noLiveGroupMembers"] and
                  process["cleanup"]["after"]["state"] == "ABSENT", "ACTUAL_C2_COMMAND_AND_CLEANUP")
    fixed.require(process["commandSeconds"] == process["wholeOwnerSeconds"] == result["wholeOwnerSeconds"] == 600 and
                  result["elapsedSeconds"] < 600 and not result["nativeAcceptance"] and not result["global179Acceptance"],
                  "ORIGINAL_BOUND_AND_CREDIT_SCOPE")
    fixed.require(re.fullmatch(r"[0-9]{3}-canonical-catalog-and-pack", process["name"]) is not None,
                  "CLOSED_PROCESS_RECEIPT_NAME")
    original = json.loads(owner.read(output/"commands"/(process["name"]+".process.original.json")))
    fixed.require(original == process, "EXACT_ORIGINAL_PROCESS_RECORD")
    for stream in ("stdout", "stderr"):
        data = owner.read(output/"commands"/(process["name"]+"."+stream+".original"))
        fixed.require(process[stream] == {"bytes": len(data), "sha256": fixed.sha(data), "complete": True},
                      "COMPLETE_ORIGINAL_STREAM")
    prerequisite = owner.read(output/"prerequisite-result.original.json")
    fixed.require(prerequisite == owner.read(fixed.EVIDENCE/"prerequisite/result.original.json") and
                  json.loads(prerequisite)["status"] == "PASS", "PREREQUISITE_ORIGINAL_JOIN")
    return result, fixed.sha(raw)


def verify_original_intake(owner, execution):
    """Retain the qualified original seven-product intake's semantic checks."""
    E, W, J = fixed.EVIDENCE/"execution", fixed.WORKTREE, protocol.HEAD
    result, rawpins, c1pins = execution, protocol.RAW_PINS, protocol.C1_PINS
    paths = protocol.GENERATED
    names = tuple(Path(p).name for p in paths)
    checks = []
    def need(ok, name):
        owner.budget.check()
        checks.append({"check": name, "passed": bool(ok)})
        fixed.require(ok, name)
    def read(path):
        return owner.read(path)
    def sha(data):
        return fixed.sha(data)
    def parsed(path):
        return json.loads(read(path))
    for rel,digest in {**rawpins,**c1pins}.items():need(sha(read(W/rel))==digest,'fixed canonical input '+rel)
    raw=parsed(W/'docs/apple-runtime/p10/p10-505-execution-evidence.json');rawrows={r['name']:r for r in raw['entries']}
    matrix=parsed(E/'products'/names[0]);audit=parsed(E/'products'/names[1]);crosswalk=parsed(E/'products'/names[2]);baseline=parsed(E/'products'/names[6])
    dep=parsed(E/'dependency-before.original.json')
    need(dep==parsed(E/'dependency-after.original.json'),'unchanged actual dependency witnesses')
    material=b''
    for row in dep['files']:
     b=read(W/row['path'])
     need(len(b)==row['bytes'] and sha(b)==row['sha256'] and sha(b.replace(b'\r\n',b'\n'))==row['normalized_sha256'],'dependency bytes '+row['path'])
     material+=(row['path']+':'+row['normalized_sha256']+'\n').encode()
    need(sha(material)==dep['fingerprint'],'independent dependency fingerprint')
    for item in (matrix,crosswalk,baseline):
     need(item['sourceCommit']==J and item['inputDependencyFingerprint']==dep['fingerprint'] and item['schemaVersion']=='2.0','actual source/fingerprint identity')
    rows=matrix['entries'];row_names=[r['cherriName'] for r in rows]
    need(len(rows)==len(rawrows)==505 and len(set(row_names))==505 and set(row_names)==set(rawrows),'exact505 closed membership')
    for row in rows:
     rr=rawrows[row['cherriName']]
     need(row['shortcutIdentifier']==rr['shortcutIdentifier'] and row['compilerConstruct']==rr['compilerConstruct'],'raw identity '+row['cherriName'])
     for key in ('legacySupported','v2Supported','plistIdentifierVerified','parameterContractVerified','decompileSupported','structuralRoundTripPass','nativeSpecimenParityStatus','appleRuntimeExecutedStatus'):
      need(row[key]==rr[key],'raw '+key+' '+row['cherriName'])
     need(row['emittedParameterKeys']==sorted(set(row['emittedParameterKeys'])) and set(rr.get('emittedKeysSample') or []).issubset(row['emittedParameterKeys']),'actual raw sample contained in catalog union '+row['cherriName'])
    summary=matrix['summary']
    need(summary['callableActions']==sum(not r['compilerConstruct'] for r in rows)==497 and summary['compilerConstructs']==sum(r['compilerConstruct'] for r in rows)==8,'497 actions and8 constructs')
    need(summary['nativeSpecimenParityCount']==summary['appleRuntimeExecutedCount']==0,'zero new native count')
    need(summary['parameterDimensionSummaries']==raw['parameterDimensionSummaries'],'all original parameter dimensions')
    need(summary['decompileModeCounts']==dict(collections.Counter(r['decompileMode'] for r in rows)) and summary['evidenceTierCounts']==dict(collections.Counter(r['evidenceTier'] for r in rows)),'derived decompile/evidence aggregate')
    need(crosswalk['denominators']==baseline['counts'],'crosswalk/baseline denominator equality')
    denom=crosswalk['denominators']
    need(denom['uniqueIdentifiersInCherriCatalog']==len(set(r['shortcutIdentifier'] for r in rows))==390 and denom['uniqueCallableIdentifiersInCherriCatalog']==len(set(r['shortcutIdentifier'] for r in rows if not r['compilerConstruct']))==382,'390 identifiers/382 callable identifiers')
    old=parsed(E/'preimages'/names[1])
    need(audit['goOnlyActionsAudit']==old['goOnlyActionsAudit'] and audit['summary']==old['summary'],'all15 original classifications unchanged')
    need(len(audit['goOnlyActionsAudit'])==audit['summary']['totalGoOnlyEvaluated']==15,'exact15 audit membership')
    need(tuple(r['declaration'] for r in audit['preservedDeclarationsAudit'])==('statistic','calculate','getFileDetail','calculationOperation'),'exact4 preserved declaration inventory')
    for row in audit['preservedDeclarationsAudit']:
     for role,commit in (('upstreamSource','bcc0095754e2cc0baaeb4d93dae172b8153b4716'),('currentSource',J)):
      witness=row[role];source=read(E/'declaration-sources'/(role+'-'+pathlib.Path(witness['path']).name))
      need(witness['sourceCommit']==commit and sha(source)==witness['sha256'],'exact declaration source '+row['declaration']+' '+role)
      need(''.join(source.decode().splitlines(True)[witness['line']-1:]).startswith(witness['declaration']),'exact declaration line '+row['declaration']+' '+role)
      if role=='currentSource':need(source==read(W/witness['path']),'current physical declaration '+row['declaration'])
     if row['declaration']!='calculationOperation':
      sample=row['currentSample'];rr=rawrows[row['declaration']]
      need(sample=={'path':next(iter(rawpins)),'sha256':rawpins[next(iter(rawpins))],'entry':row['declaration'],'emittedKeysSample':rr['emittedKeysSample'],'scope':'SYNTHETIC_COMPILER_SAMPLE'},'original sample source '+row['declaration'])
     else:need('currentSample' not in row,'enum has no invented execution')
    for name in names[3:6]:
     md=read(E/'products'/name).decode()
     need('- **Source Commit**: `'+J+'`' in md and '- **Input Dependency Fingerprint**: `'+dep['fingerprint']+'`' in md,'markdown exact provenance '+name)
    md=read(E/'products'/names[3]).decode()
    table=md.split('## Complete 505 Catalog Verification Table\n',1)[1]
    actual=[x for x in table.splitlines() if re.match(r'^\| [0-9]+ \|',x)]
    expected=[]
    for i,r in enumerate(rows,1):
     expected.append('| '+str(i)+' | `'+r['cherriName']+'` | `'+r['entryKind']+'` | `'+r['shortcutIdentifier']+'` | `'+r['sourceFile']+':'+str(r['sourceLine'])+'` | `'+r['decompileMode']+'` | `'+('PASS' if r['structuralRoundTripPass'] else 'FAIL')+'` | `'+r['packSupportKind']+'` | `'+r['evidenceTier']+'` |')
    need(actual==expected,'all505 Markdown rows independent projection')
    before=parsed(E/'source-before.original.json');after=parsed(E/'source-after.original.json')
    need((before['head'],before['tree'],before['branch'])==(after['head'],after['tree'],after['branch']) and before['status']=='','original before/after identity')
    need(set(before['trackedPhysicalInputs'])==set(after['trackedPhysicalInputs']),'original full physical inventory closure')
    need(all(before['trackedPhysicalInputs'][n]==after['trackedPhysicalInputs'][n] for n in before['trackedPhysicalInputs'] if n not in paths),'all nonproduct inputs unchanged')
    need(all(x in {' M '+p for p in paths} for x in after['status'].splitlines()),'only seven allowed unstaged products')
    for rel,pin in result['products']['pins'].items():
     b=read(E/'products'/pathlib.Path(rel).name)
     need(sha(b)==pin['sha256'] and len(b)==pin['bytes'] and b==read(W/rel)==read(E/'observed-products'/pathlib.Path(rel).name),'three-way original product '+rel)
    return {"schema": "cherri-canonical-c2-hosted-intake/1", "status": "PASS", "checkCount": len(checks),
            "checks": checks, "products": result["products"]["pins"],
            "sourcePhysicalFiles": len(before["trackedPhysicalInputs"]),
            "semanticCounts": {"catalogEntries": 505, "callableActions": 497, "compilerConstructs": 8,
                               "uniqueNativeIdentifiers": 390, "uniqueCallableNativeIdentifiers": 382,
                               "goOnlyActions": 15, "preservedDeclarations": 4, "nativeSpecimenParity": 0,
                               "appleRuntimeExecuted": 0},
            "goRepeated": False, "canonicalProducerRepeated": False,
            "nativeAcceptance": False, "global179Acceptance": False}


def verify_bundle(owner, bundle, commit):
    """A delta bundle carries one new head and requires the exact producer parent."""
    reference = "refs/heads/"+protocol.BRANCH
    owner.git("bundle", "verify", str(bundle))
    fixed.require(owner.git("bundle", "list-heads", str(bundle)).decode().strip().split() == [commit, reference],
                  "ONE_EXACT_BUNDLE_HEAD")
    data = owner.read(bundle)
    header = data.split(b"\n\n", 1)[0].decode("utf-8")
    prerequisites = [line[1:].split(" ", 1)[0] for line in header.splitlines() if line.startswith("-")]
    fixed.require(prerequisites == [protocol.HEAD] and 0 < len(data) <= 32*1024*1024,
                  "EXACT_SMALL_PREREQUISITE_BUNDLE")
    return {"path": bundle.name, "bytes": len(data), "sha256": fixed.sha(data),
            "prerequisite": protocol.HEAD, "head": commit, "ref": reference}


def main():
    fixed.require(len(sys.argv) == 1, "FIXED_ADOPTION_ROUTE")
    with fixed.owned.WholeBudget(240, cleanup_seconds=8, final_reserve=25) as budget:
        output = fixed.EVIDENCE/"adoption"
        output.mkdir(mode=0o700)
        owner = fixed.SourceOwner({"worktree": str(fixed.WORKTREE)}, output, budget)
        failures, commit, tree, bundle_pin = [], None, None, None
        controller_before = controller_after = before = after = None
        original_result_sha = None
        fixed.owned.original(output/"hosted-runtime.original.json", fixed.runtime_identity())
        fixed.owned.original(output/"adopt.py", owner.read(P/"adopt.py"))
        try:
            controller_before = fixed.controller_snapshot(owner)
            fixed.owned.original(output/"controller-before.original.json", controller_before)
            execution, original_result_sha = execution_receipts(owner)
            original_before = json.loads(owner.read(fixed.EVIDENCE/"execution/source-before.original.json"))
            original_after = json.loads(owner.read(fixed.EVIDENCE/"execution/source-after.original.json"))
            before = protocol.snapshot_after(owner, original_before)
            fixed.owned.original(output/"source-before.original.json", before)
            fixed.require(before == original_after, "NO_POST_EXECUTION_SOURCE_DRIFT")
            _, observed, errors = owner.observe_concurrent_owner()
            decision = json.loads(owner.read(observed))
            fixed.require(not owner.read(errors) and decision["admitted"] is True and decision["blockedPids"] == [],
                          "ADOPTION_EXCLUSIVE_GO_ADMISSION")
            resources = fixed.free()
            fixed.owned.original(output/"capacity.original.json", {"observed": resources,
                "admission": fixed.ADMISSION_FLOORS, "live": fixed.LIVE_FLOORS})
            fixed.require(all(resources[key] >= floor for key, floor in fixed.ADMISSION_FLOORS.items()),
                          "ADOPTION_CAPACITY_ADMISSION")
            intake = verify_original_intake(owner, execution)
            fixed.owned.original(output/"intake.original.json", {**intake, "source": protocol.HEAD,
                "sourceTree": protocol.TREE, "controllerSha": fixed.CONTEXT["controllerSha"],
                "ownerResultSHA256": original_result_sha, "originalIntakeSourceSHA256": ORIGINAL_INTAKE_SHA256})
            c1_before, c2_before = c1_fingerprint(owner), owner.dependencies()
            fixed.require(c1_before["fingerprint"] == C1_FINGERPRINT and c2_before["fingerprint"] == C2_FINGERPRINT,
                          "ACTUAL_C1_C2_INPUT_FINGERPRINTS")
            fixed.owned.original(output/"fingerprints-before.original.json", {"c1": c1_before, "c2": c2_before})
            paths = sorted(protocol.GENERATED)
            fixed.require(sorted(before["status"].splitlines()) == sorted(" M "+path for path in paths),
                          "EXACT_SEVEN_BYTE_ADOPTION")
            owner.git("diff", "--check", "--", *paths)
            owner.git("add", "--", *paths)
            fixed.require(sorted(owner.git("diff", "--cached", "--name-only").decode().splitlines()) == paths,
                          "ONLY_SEVEN_STAGED_PATHS")
            owner.git("-c", "gc.auto=0", "-c", "maintenance.auto=false", "-c", "user.name=Cherri Completion Agent",
                      "-c", "user.email=cherri-completion@users.noreply.github.com", "commit", "-m",
                      "chore: adopt canonical C2 from exact producer fb1 [skip ci]")
            commit, tree = (owner.git("rev-parse", argument).decode().strip() for argument in ("HEAD", "HEAD^{tree}"))
            fixed.require(owner.git("rev-parse", "HEAD^").decode().strip() == protocol.HEAD and
                          sorted(owner.git("diff-tree", "--no-commit-id", "--name-only", "-r", "HEAD").decode().splitlines()) == paths,
                          "EXACT_PRODUCER_PARENT_AND_SEVEN_FILES")
            after = owner.snapshot()
            fixed.owned.original(output/"source-after.original.json", after)
            fixed.require((after["head"], after["tree"], after["branch"], after["status"]) ==
                          (commit, tree, protocol.BRANCH, "") and
                          after["trackedPhysicalInputs"] == before["trackedPhysicalInputs"], "ADOPTED_BYTES_UNCHANGED")
            c1_after, c2_after = c1_fingerprint(owner), owner.dependencies()
            fixed.require(c1_before == c1_after and c2_before == c2_after, "ADOPTION_DEPENDENCIES_CHANGED")
            fixed.owned.original(output/"fingerprints-after.original.json", {"c1": c1_after, "c2": c2_after})
            bundle = output/"canonical-c2.original.bundle"
            reference = "refs/heads/"+protocol.BRANCH
            owner.git("bundle", "create", str(bundle), reference, "^"+protocol.HEAD)
            bundle_pin = verify_bundle(owner, bundle, commit)
            fixed.owned.original(output/"bundle.original.json", bundle_pin)
        except BaseException as error:
            failures.append({"stage": "adoption", "type": type(error).__name__, "message": str(error)})
        finally:
            with budget.finalizing():
                try:
                    controller_after = fixed.controller_snapshot(owner)
                    fixed.owned.original(output/"controller-after.original.json", controller_after)
                    fixed.require(controller_before == controller_after, "ADOPTION_CONTROLLER_CHANGED")
                    final = owner.snapshot(finalizing=True) if commit is not None else protocol.snapshot_after(owner,
                        json.loads(owner.read(fixed.EVIDENCE/"execution/source-before.original.json")))
                    fixed.owned.original(output/"source-final.original.json", final)
                    if after is not None:
                        fixed.require(final == after, "POST_BUNDLE_SOURCE_CHANGED")
                except BaseException as error:
                    failures.append({"stage": "post-source", "type": type(error).__name__, "message": str(error)})
                if budget.cancel_signal is not None or time.monotonic() >= budget.deadline:
                    failures.append({"stage": "owner", "type": "CANCELLED_OR_DEADLINE", "signal": budget.cancel_signal})
                passed = not failures and bundle_pin is not None
                report = {"schema": "cherri-canonical-c2-hosted-byte-adoption/1", "status": "PASS" if passed else "FAIL",
                    "producerSource": protocol.HEAD, "producerTree": protocol.TREE,
                    "controllerSha": fixed.CONTEXT["controllerSha"], "ownerResultSHA256": original_result_sha,
                    "commit": commit, "tree": tree, "branch": protocol.BRANCH, "bundle": bundle_pin,
                    "changedTrackedPaths": list(protocol.GENERATED), "wholeOwnerSeconds": 240,
                    "elapsedSeconds": time.monotonic()-budget.started, "minimumFree": owner.minimum,
                    "failures": failures, "goExecuted": False, "remotePushExecuted": False,
                    "nativeAcceptance": False, "global179Acceptance": False}
                with budget.defer():
                    pin = fixed.owned.original(output/"result.original.json", report)
    print(json.dumps({"path": str(output/"result.original.json"), "sha256": pin["sha256"],
                      "status": report["status"], "commit": commit}), flush=True)
    return 0 if passed else 1


if __name__ == "__main__":
    raise SystemExit(main())
