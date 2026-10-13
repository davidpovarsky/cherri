#!/usr/bin/env bash
# Execute one isolated Cherri request in GitHub Actions, using the live canonical Skill.
set -euo pipefail
cd "$GITHUB_WORKSPACE"
mkdir -p artifacts
trap 'status=$?; if [ "$status" -ne 0 ]; then echo "CHERRI_CLOUD_STATUS=FAIL" > artifacts/status.txt; fi' EXIT
REF="$GITHUB_REF_NAME"
PREFIX="cloud-cherri-request/"
case "$REF" in "$PREFIX"*) ;; *) echo "Invalid request branch" >&2; exit 2;; esac
KEY=$(printf '%s' "$REF" | sed 's#^cloud-cherri-request/##')
if ! [[ "$KEY" =~ ^(compile|sign|simulate)-[a-z0-9-]{6,60}$ ]]; then
    echo "Request key/mode invalid" >&2; exit 2
fi
MODE=$(printf '%s' "$KEY" | cut -d - -f1)
SOURCE_PATH="cloud-requests/$KEY/source.cherri"
[ -s "$SOURCE_PATH" ] || { echo "source.cherri missing" >&2; exit 2; }
if [ "$(wc -c < "$SOURCE_PATH")" -gt 262144 ]; then
    echo "Source exceeds 256 KiB" >&2; exit 2
fi
# Do not log source text; public request branches are for nonsensitive input only.
python3 - "$SOURCE_PATH" <<'PY'
import pathlib,re,sys
data=pathlib.Path(sys.argv[1]).read_text("utf-8")
blocked=[
  r"-----BEGIN (?:RSA |EC |OPENSSH )?PRIVATE KEY-----",
  r"github_pat_[A-Za-z0-9_]{10,}",
  r"ghp_[A-Za-z0-9]{10,}",
  r"sk-proj-[A-Za-z0-9_-]{12,}",
  r"(?i)Authorization\s*:\s*Bearer\s+\S+",
]
if any(re.search(p,data) for p in blocked):
    raise SystemExit("Potential credential detected in source; refusing public GitHub request")
PY
source scripts/cherri-cloud/bootstrap.sh
BIN="$CHERRI_BIN"
BUILD_NAME="CherriCloud_$(printf '%s' "$KEY" | tr '-' '_')"
STAGING="$RUNNER_TEMP/cherri-cloud-stage"
mkdir -p "$STAGING"
cp "$SOURCE_PATH" "$STAGING/$BUILD_NAME.cherri"
cp "$SOURCE_PATH" artifacts/source.cherri
"$BIN" --capabilities-json > artifacts/capabilities.json
echo "Validating source with canonical v2 CLI..."
"$BIN" check "$STAGING/$BUILD_NAME.cherri" --json > artifacts/check.json
echo "Compiling unsigned with canonical Skill build.sh..."
sh "$SKILL_DIR/scripts/build.sh" "$STAGING/$BUILD_NAME.cherri" "artifacts/$BUILD_NAME-unsigned.shortcut" --unsigned
[ -s "artifacts/$BUILD_NAME-unsigned.shortcut" ] || { echo "Unsigned output missing" >&2; exit 1; }
python3 - "$KEY" "$MODE" "$LIVE_SKILL_SHA" "$CHERRI_MANIFEST_FILE" "$BUILD_NAME" <<'PY'
import json,os,sys
key,mode,skill_sha,manifest_path,name=sys.argv[1:]
m=json.load(open(manifest_path,encoding="utf-8"))
data={"request_key":key,"mode":mode,"request_commit":os.environ["GITHUB_SHA"],"skill_commit":skill_sha,"compiler_commit":m["compilerCommit"],"docs_commit":m["docsCommit"],"schema_fingerprint":m["schemaFingerprint"],"codec_fingerprint":m["codecFingerprint"],"shortcut_name":name,"signed":False,"simulator":{"status":"not_requested"}}
with open("artifacts/request-metadata.json","w",encoding="utf-8") as f:json.dump(data,f,indent=2)
PY
if [ "$MODE" = "sign" ] || [ "$MODE" = "simulate" ]; then
    echo "Signing via canonical Skill (remote signing service may receive plist)..."
    sh "$SKILL_DIR/scripts/build.sh" "$STAGING/$BUILD_NAME.cherri" "artifacts/$BUILD_NAME.shortcut" --signed
    python3 - "$BUILD_NAME" <<'PY'
import pathlib,sys
p=pathlib.Path("artifacts")/(sys.argv[1]+".shortcut")
if not p.exists() or p.read_bytes()[:4]!=b"AEA1": raise SystemExit("Signed Shortcut missing or invalid AEA1 header")
PY
    python3 - <<'PY'
import json
p="artifacts/request-metadata.json"
m=json.load(open(p,encoding="utf-8"));m["signed"]=True
with open(p,"w",encoding="utf-8") as f:json.dump(m,f,indent=2)
PY
fi
if [ "$MODE" = "simulate" ]; then
    bash scripts/cherri-cloud/simulate-request.sh "artifacts/$BUILD_NAME.shortcut" "$BUILD_NAME"
fi
echo "CHERRI_CLOUD_STATUS=PASS" > artifacts/status.txt
echo "CHERRI_CLOUD_REQUEST_KEY=$KEY"
echo "CHERRI_CLOUD_MODE=$MODE"
