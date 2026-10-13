#!/usr/bin/env bash
# Cherri cloud: load live canonical Skill and install its pinned compatible compiler.
set -euo pipefail
test -n "$RUNNER_TEMP" && test -n "$GITHUB_WORKSPACE"
cd "$GITHUB_WORKSPACE"

SKILL_REF=$(python3 - <<'PY'
import json,re
c=json.load(open("cloud-cherri-requests/config.json",encoding="utf-8"))
ref=c["skill_ref"]
assert re.fullmatch(r"[A-Za-z0-9_./-]{2,150}",ref) and ".." not in ref and not ref.startswith("/"),"invalid skill ref"
assert c["repo"]=="davidpovarsky/cherri","unexpected repository"
print(ref)
PY
)
SKILL_CHECKOUT="$RUNNER_TEMP/cherri-live-skill"
export CHERRI_HOME="$RUNNER_TEMP/cherri-skill-home"
# Keep GitHub request branches isolated from the code and docs in the Skill's lock.
unset CHERRI_BIN CHERRI_REF CHERRI_REPO CHERRI_DOCS_REF CHERRI_DOCS_REPO CHERRI_PREBUILT_URL CHERRI_OFFLINE || true
git fetch --no-tags origin "refs/heads/$SKILL_REF:refs/remotes/origin/$SKILL_REF"
LIVE_SKILL_SHA=$(git rev-parse "refs/remotes/origin/$SKILL_REF")
git worktree add --detach "$SKILL_CHECKOUT" "$LIVE_SKILL_SHA" >/dev/null
export SKILL_DIR="$SKILL_CHECKOUT/skills/cherri-shortcuts"
export CHERRI_MANIFEST_FILE="$SKILL_DIR/compatibility-manifest.json"
[ -f "$CHERRI_MANIFEST_FILE" ] || { echo "canonical Cherri skill manifest missing" >&2; exit 1; }
python3 - "$CHERRI_MANIFEST_FILE" <<'PY'
import json,re,sys
m=json.load(open(sys.argv[1],encoding="utf-8"))
assert m["languageVersion"]=="2.0","unsupported Cherri languageVersion"
assert m["compilerRepo"]=="https://github.com/davidpovarsky/cherri.git","unexpected compiler repository"
for key in ("compilerCommit","docsCommit"):
    assert re.fullmatch(r"[0-9a-f]{40}",m[key]),f"invalid {key}"
for key in ("schemaFingerprint","codecFingerprint"):
    assert re.fullmatch(r"[0-9a-f]{64}",m[key]),f"invalid {key}"
print("PINNED_COMPILER_SHA="+m["compilerCommit"])
print("PINNED_DOCS_SHA="+m["docsCommit"])
PY
# The actual pinned installer compiles with Go on the runner (never in ChatGPT).
sh "$SKILL_DIR/scripts/setup.sh"
sh "$SKILL_DIR/scripts/doctor.sh"
export CHERRI_BIN="$CHERRI_HOME/bin/cherri"
[ -x "$CHERRI_BIN" ] || { echo "compiler binary missing" >&2; exit 1; }
export LIVE_SKILL_SHA
python3 - "$CHERRI_MANIFEST_FILE" "$CHERRI_BIN" <<'PY'
import json,subprocess,sys
m=json.load(open(sys.argv[1],encoding="utf-8"))
a=json.loads(subprocess.check_output([sys.argv[2],"--capabilities-json"],text=True))
for key in ("languageVersion","catalogSchemaVersion","schemaFingerprint","codecFingerprint"):
    if m.get(key)!=a.get(key):
        raise SystemExit(f"CAPABILITIES_MISMATCH {key}: {a.get(key)} != {m.get(key)}")
print("LIVE_SKILL_AND_COMPILER=VERIFIED")
PY
echo "LIVE_SKILL_SHA=$LIVE_SKILL_SHA"
