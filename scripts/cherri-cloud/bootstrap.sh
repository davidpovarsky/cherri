#!/usr/bin/env bash
# Source this file from a GitHub Actions Cherri cloud request job.
set -euo pipefail

: "${RUNNER_TEMP:?RUNNER_TEMP is required}"
: "${GITHUB_WORKSPACE:?GITHUB_WORKSPACE is required}"
CHERRI_SKILL_BRANCH="agent/shared-shortcuts-engine-v2-round3-closure"
SKILL_CHECKOUT="$RUNNER_TEMP/cherri-cloud-live-skill"
ENGINE_CHECKOUT="$RUNNER_TEMP/cherri-cloud-engine"
CHERRI_BIN="$RUNNER_TEMP/cherri-cloud-bin/cherri"
export CHERRI_BIN
mkdir -p "$RUNNER_TEMP/cherri-cloud-bin"

# The installed plugin never pins its own copy of the Skill. Fetch the
# authoritative current branch and consume its immutable compatibility lock.
git fetch --no-tags origin "refs/heads/$CHERRI_SKILL_BRANCH:refs/remotes/origin/$CHERRI_SKILL_BRANCH"
LIVE_SKILL_SHA=$(git rev-parse "refs/remotes/origin/$CHERRI_SKILL_BRANCH")
git worktree add --detach "$SKILL_CHECKOUT" "$LIVE_SKILL_SHA" >/dev/null
SKILL_DIR="$SKILL_CHECKOUT/skills/cherri-shortcuts"
CHERRI_MANIFEST_FILE="$SKILL_DIR/compatibility-manifest.json"
export SKILL_DIR CHERRI_MANIFEST_FILE LIVE_SKILL_SHA

PINNED_COMPILER_SHA=$(python3 - "$CHERRI_MANIFEST_FILE" <<'PY'
import json,re,sys
m=json.load(open(sys.argv[1],encoding="utf-8"))
sha=m["compilerCommit"]
assert re.fullmatch(r"[0-9a-f]{40}",sha), "invalid immutable compiler commit"
assert m["languageVersion"]=="2.0", "unexpected Cherri language major version"
for key in ("schemaFingerprint","codecFingerprint"):
    assert re.fullmatch(r"[0-9a-f]{64}",m[key]), f"invalid {key}"
assert m["compilerRepo"]=="https://github.com/davidpovarsky/cherri.git", "unexpected compiler repository"
print(sha)
PY
)
export PINNED_COMPILER_SHA
git cat-file -e "$PINNED_COMPILER_SHA^{commit}" || git fetch --no-tags origin "$PINNED_COMPILER_SHA"
git worktree add --detach "$ENGINE_CHECKOUT" "$PINNED_COMPILER_SHA" >/dev/null
(cd "$ENGINE_CHECKOUT" && go build -o "$CHERRI_BIN" .)
CHERRI_HOME="$RUNNER_TEMP/cherri-cloud-home"
export CHERRI_HOME
# Use the same validation as the canonical skill.
sh "$SKILL_DIR/scripts/doctor.sh"
python3 - "$CHERRI_MANIFEST_FILE" "$CHERRI_BIN" <<'PY'
import json,subprocess,sys
m=json.load(open(sys.argv[1],encoding="utf-8"))
a=json.loads(subprocess.check_output([sys.argv[2],"--capabilities-json"],text=True))
for key in ("languageVersion","catalogSchemaVersion","schemaFingerprint","codecFingerprint"):
    if m.get(key)!=a.get(key): raise SystemExit(f"CAPABILITIES_MISMATCH {key}: {a.get(key)} != {m.get(key)}")
print("LIVE_SKILL_AND_COMPILER=VERIFIED")
PY
echo "LIVE_SKILL_SHA=$LIVE_SKILL_SHA"
echo "PINNED_COMPILER_SHA=$PINNED_COMPILER_SHA"
