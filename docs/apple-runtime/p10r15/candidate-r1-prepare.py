#!/usr/bin/env python3
"""Stage an inactive R1 tuple by copying the existing Remote Builder surfaces."""
import hashlib, json, pathlib, shutil

live = pathlib.Path("/home/chatgpt-agent/workspaces/cherri-remote")
stage = live / "jobs/p10r15-consumer-validation-r1-d00bc0b9"
candidate = stage / "remote-home"
source_sha = "d00bc0b9e9a4d8c460623de3d883680b94289e04"
docs_sha = "78d6cc8bc408504a5d9e7abfd1a7120f02a22c4d"
label = "p15-r1"
branch = "agent/cherri-p10r15-validation-r1-20261118"
def digest(p): return hashlib.sha256(pathlib.Path(p).read_bytes()).hexdigest()
expected = {
    "active.json": "ee9aac247c568fb743d4e505bcaaebe224bfca843d1091438701bd18273807d0",
    "bin/cherri-remote-runner.sh": "7ae689e9d494e7ddb9a7b9abfb5c5d96bb4321b069dc2847aec07271122cb954",
    "bin/cherri-remote-legacy.sh": "734507b7a981d7d4185fb3582bd9601d958db013dcd7b12b13201d0d3b0e0b4c",
    "bin/cherri-shortcut-lint.py": "51ffca98be2275f4015931e15d73251fff7f7041fb323c9b7aa6973eb57a454f",
}
for p, sha in expected.items():
    assert digest(live / p) == sha, "Unexpected concurrent live change: " + p
assert (live / "config/default-language-mode").read_text().strip() == "legacy"
assert not candidate.exists(), "Candidate directory already exists; inspect before modifying"
skill = stage / "skill"
shutil.copytree(stage / "compiler/skills/cherri-shortcuts", skill)
manifest_path = skill / "compatibility-manifest.json"
manifest = json.loads(manifest_path.read_text())
original_manifest = dict(manifest)
manifest["compilerCommit"] = source_sha
manifest["docsCommit"] = docs_sha
manifest_path.write_text(json.dumps(manifest, indent=2) + "\n")
for p in ("bin", "config", "jobs", "outputs", "tmp", f"source/{label}/skills", label):
    (candidate / p).mkdir(parents=True, exist_ok=True)
(candidate / f"source/{label}/skills/cherri-shortcuts").symlink_to(skill, target_is_directory=True)
(candidate / f"{label}/runtime").symlink_to(stage / "runtime", target_is_directory=True)
(candidate / "tools").symlink_to(live / "tools", target_is_directory=True)
(stage / "runtime/docs").symlink_to(stage / "docs", target_is_directory=True)
(stage / "runtime/src").symlink_to(stage / "compiler", target_is_directory=True)
(candidate / "config/default-language-mode").write_text("legacy\n")
runner_text = (live / "bin/cherri-remote-runner.sh").read_text()
anchor = 'sync) sync_release "$' + '{1:-}" ;;'
assert runner_text.count(anchor) == 1
runner_text = runner_text.replace(anchor, "sync) die 'R1 validation tuple is inactive; sync awaits the final release package' ;;")
runner = candidate / "bin/cherri-remote-runner.sh"
runner.write_text(runner_text)
runner.chmod(0o700)
validator = candidate / "bin/cherri-shortcut-lint.py"
shutil.copy2(live / "bin/cherri-shortcut-lint.py", validator)
legacy_text = (live / "bin/cherri-remote-legacy.sh").read_text()
legacy_text = legacy_text.replace(expected["bin/cherri-remote-runner.sh"], digest(runner))
legacy_text = legacy_text.replace("$BASE/p10/runtime", f"$BASE/{label}/runtime")
legacy_text = legacy_text.replace("x.get('label')!='p10'", f"x.get('label')!='{label}'")
legacy_text = legacy_text.replace("agent/cherri-p10-apple-actions-coverage", branch)
legacy_text = legacy_text.replace("$"+"{name}-p10-legacy-", "$"+"{name}-p15-r1-legacy-")
legacy_text = legacy_text.replace("P10", "R1")
legacy = candidate / "bin/cherri-remote-legacy.sh"
legacy.write_text(legacy_text)
legacy.chmod(0o700)
active = json.loads((live / "active.json").read_text())
active.update(label=label, compilerCommit=source_sha, docsCommit=docs_sha,
              schemaFingerprint=manifest["schemaFingerprint"], sourceBranch=branch,
              skillSourceHead=source_sha)
(candidate / "active.json").write_text(json.dumps(active, indent=2, sort_keys=True) + "\n")
receipt = {
    "status": "INACTIVE_R1_VALIDATION_ONLY",
    "sourceCommit": source_sha, "docsCommit": docs_sha,
    "candidateHome": str(candidate),
    "skillPackage": {
        "copiedFromSourceCommit": source_sha,
        "manifestOnlyOverrides": {k: {"before": original_manifest[k], "after": manifest[k]}
                                  for k in ("compilerCommit", "docsCommit")},
        "manifestSHA256": digest(manifest_path),
        "releasePackageCommit": None,
    },
    "liveHashesBefore": expected,
    "candidateScriptSHA256": {p: digest(candidate / p) for p in expected},
    "livePromotion": False,
    "syncPolicy": "Disabled in inactive validation copy; existing P10/P9 fallback cannot activate.",
}
receipt["liveHashesAfter"] = {p: digest(live / p) for p in expected}
assert receipt["liveHashesAfter"] == expected
(stage / "staging-receipt.json").write_text(json.dumps(receipt, indent=2, sort_keys=True) + "\n")
print(json.dumps({"status": receipt["status"], "candidateHome": str(candidate),
                  "liveUnchanged": True, "releasePackageCommit": None}))
