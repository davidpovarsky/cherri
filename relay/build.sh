#!/usr/bin/env bash
# Cherri Cloud Relay v2: compile only the validated Git blob loaded by prepare.py.
set -euo pipefail

: "$GITHUB_WORKSPACE"
: "$RUNNER_TEMP"
: "$CHERRI_REQUEST_ID"
: "$CHERRI_SOURCE_PATH"
: "$CHERRI_SKILL_REF"
: "$CHERRI_SKILL_SHA"
: "$CHERRI_COMPILER_SHA"
: "$CHERRI_DOCS_SHA"
cd "$GITHUB_WORKSPACE"
mkdir -p artifacts
trap 'rc=$?; if [ "$rc" -ne 0 ]; then printf "CHERRI_CLOUD_STATUS=FAIL\n" > artifacts/status.txt; fi' EXIT

export CHERRI_HOME="$RUNNER_TEMP/cherri-cloud-home"
unset CHERRI_BIN CHERRI_REF CHERRI_REPO CHERRI_DOCS_REF CHERRI_DOCS_REPO CHERRI_PREBUILT_URL CHERRI_OFFLINE || true
SKILL_HOME="$RUNNER_TEMP/cherri-relay-skill"
git fetch --no-tags --depth=1 origin "refs/heads/$CHERRI_SKILL_REF"
FETCHED_SHA=$(git rev-parse FETCH_HEAD)
if [ "$FETCHED_SHA" != "$CHERRI_SKILL_SHA" ]; then
    echo "Skill SHA moved during execution: $FETCHED_SHA" >&2
    exit 1
fi
git worktree add --detach "$SKILL_HOME" "$FETCHED_SHA" >/dev/null
export SKILL_DIR="$SKILL_HOME/skills/cherri-shortcuts"
export CHERRI_MANIFEST_FILE="$SKILL_DIR/compatibility-manifest.json"
python3 - "$CHERRI_MANIFEST_FILE" "$CHERRI_COMPILER_SHA" "$CHERRI_DOCS_SHA" <<'PY'
import json,sys
m=json.load(open(sys.argv[1],encoding="utf-8"))
assert m["compilerCommit"]==sys.argv[2], "pinned compiler changed since prepare"
assert m["docsCommit"]==sys.argv[3], "pinned docs changed since prepare"
PY

# actions/cache restores $CHERRI_HOME keyed by OS, architecture and immutable
# compiler/docs SHAs. The canonical setup.sh validates the cached binary and
# documentation fingerprints before use; a mismatch forces a fresh build.
sh "$SKILL_DIR/scripts/setup.sh"
sh "$SKILL_DIR/scripts/doctor.sh"
export CHERRI_BIN="$CHERRI_HOME/bin/cherri"
[ -x "$CHERRI_BIN" ] || { echo "Cherri compiler missing" >&2; exit 1; }
"$CHERRI_BIN" --capabilities-json > artifacts/capabilities.json

NAME="CherriCloud_$CHERRI_REQUEST_ID"
STAGING="$RUNNER_TEMP/cherri-relay-input"
mkdir -p "$STAGING"
cp "$CHERRI_SOURCE_PATH" "$STAGING/$NAME.cherri"
cp "$CHERRI_SOURCE_PATH" artifacts/source.cherri
"$CHERRI_BIN" check "$STAGING/$NAME.cherri" --json > artifacts/check.json
sh "$SKILL_DIR/scripts/build.sh" "$STAGING/$NAME.cherri" "artifacts/$NAME-unsigned.shortcut" --unsigned
[ -s "artifacts/$NAME-unsigned.shortcut" ] || { echo "Unsigned output missing" >&2; exit 1; }

if [ "$CHERRI_TARGET_MODE" = "sign" ] || [ "$CHERRI_TARGET_MODE" = "simulate" ]; then
    # Native macOS shortcuts sign cannot use iCloud on a headless CI runner.
    # Continue to use the canonical Cherri signing fallback (HubSign).
    if [ "$(uname -s)" != "Darwin" ]; then
        echo "Signing requires the macOS runner" >&2
        exit 1
    fi
    SIGN_PATH="$RUNNER_TEMP/cherri-no-native-icloud"
    mkdir -p "$SIGN_PATH"
    cat > "$SIGN_PATH/shortcuts" <<'WRAPPER'
#!/bin/sh
echo "Native iCloud signer unavailable in GitHub Actions; use canonical HubSign fallback" >&2
exit 2
WRAPPER
    chmod +x "$SIGN_PATH/shortcuts"
    export PATH="$SIGN_PATH:$PATH"
    sh "$SKILL_DIR/scripts/build.sh" "$STAGING/$NAME.cherri" "artifacts/$NAME.shortcut" --signed
    python3 - "$NAME" <<'PY'
import pathlib,sys
p=pathlib.Path("artifacts")/(sys.argv[1]+".shortcut")
if not p.is_file() or p.read_bytes()[:4] != b"AEA1":
    raise SystemExit("signed AEA1 file missing or invalid")
PY
fi

if [ "$CHERRI_TARGET_MODE" = "simulate" ]; then
    git fetch --no-tags --depth=1 origin refs/heads/cloud-cherri-requests
    BACKEND="$RUNNER_TEMP/cherri-relay-backend"
    git worktree add --detach "$BACKEND" FETCH_HEAD >/dev/null
    [ ! -e "$BACKEND/artifacts" ] || { echo "backend artifacts path occupied" >&2; exit 1; }
    ln -s "$GITHUB_WORKSPACE/artifacts" "$BACKEND/artifacts"
    (cd "$BACKEND" && bash scripts/cherri-cloud/simulate-request.sh "$GITHUB_WORKSPACE/artifacts/$NAME.shortcut" "$NAME")
fi

python3 - <<'PY'
import hashlib,json,os,pathlib
p=pathlib.Path("artifacts")
id=os.environ["CHERRI_REQUEST_ID"]
mode=os.environ["CHERRI_TARGET_MODE"]
manifest=json.load(open(os.environ["CHERRI_MANIFEST_FILE"],encoding="utf-8"))
src=(p/"source.cherri").read_bytes()
assert hashlib.sha256(src).hexdigest()==os.environ["CHERRI_SOURCE_SHA256"]
signed_name="CherriCloud_"+id+".shortcut"
result={
 "protocol":"cherri-cloud-relay-v2",
 "request_id":id,
 "mode":mode,
 "workflow_run_id":os.environ["GITHUB_RUN_ID"],
 "run_attempt":os.environ["GITHUB_RUN_ATTEMPT"],
 "workflow_sha":os.environ["GITHUB_SHA"],
 "source_sha256":hashlib.sha256(src).hexdigest(),
 "source_blob_sha":os.environ["CHERRI_SOURCE_BLOB_SHA"],
 "skill_commit":os.environ["CHERRI_SKILL_SHA"],
 "compiler_commit":manifest["compilerCommit"],
 "docs_commit":manifest["docsCommit"],
 "schema_fingerprint":manifest["schemaFingerprint"],
 "codec_fingerprint":manifest["codecFingerprint"],
 "signed":(p/signed_name).is_file(),
 "simulator_evidence_present":(p/"simulator-result.json").is_file(),
}
(p/"request-metadata.json").write_text(json.dumps(result,indent=2),encoding="utf-8")
print("CHERRI_CLOUD_REQUEST_VERIFIED="+id)
print("CHERRI_CLOUD_MODE="+mode)
print("CHERRI_CLOUD_COMPILE=PASS")
PY
printf "CHERRI_CLOUD_STATUS=PASS\n" > artifacts/status.txt
