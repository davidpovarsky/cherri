#!/usr/bin/env bash
# Run a user-requested signed Shortcut in a clean iOS 27 simulator.
# This is an observation runner, not a claim that arbitrary UI workflows pass.
set -euo pipefail
SIGNED=$1
NAME=$2
[ -s "$SIGNED" ] || { echo "signed shortcut missing" >&2; exit 1; }
python3 - "$SIGNED" <<'PY'
import pathlib,sys
assert pathlib.Path(sys.argv[1]).read_bytes()[:4]==b"AEA1", "simulator requires signed AEA1 Shortcut"
PY
SIM_UDID=$(python3 - <<'PY'
import json,subprocess,sys
items=json.loads(subprocess.check_output(["xcrun","simctl","list","devices","available","-j"]))
for runtime,devs in items.get("devices",{}).items():
    if "iOS-27" in runtime:
        for d in devs:
            if d.get("isAvailable") and "iPhone" in d.get("name",""):
                print(d["udid"]);sys.exit(0)
raise SystemExit("No available iOS 27 iPhone simulator")
PY
)
echo "Cherri cloud simulator: $SIM_UDID"
xcrun simctl boot "$SIM_UDID" 2>/dev/null || true
xcrun simctl bootstatus "$SIM_UDID" -b
xcrun simctl launch "$SIM_UDID" com.apple.shortcuts
sleep 3
xcrun simctl io "$SIM_UDID" screenshot artifacts/simulator-before.png || true

# Reuse the repo's proven ImportHelper XCUITest app. Do not replace it with clicks.
PROJECT="tests/runtime_poc/ImportHelper"
(cd "$PROJECT" && xcodegen generate && xcodebuild build-for-testing -project ImportHelper.xcodeproj -scheme ImportHelper -destination "generic/platform=iOS Simulator" -derivedDataPath .build CODE_SIGN_IDENTITY="-")
RUNFILE=$(find "$PROJECT/.build" -name '*.xctestrun' -type f | head -n 1)
[ -f "$RUNFILE" ] || { echo "ImportHelper xctestrun missing" >&2; exit 1; }
run_helper() {
    local stage="$1"
    local logfile="artifacts/helper_$stage.log"
    local bundle="artifacts/helper_$stage.xcresult"
    rm -rf "$bundle"
    python3 - "$RUNFILE" "$stage" <<'PY'
import plistlib,sys
path,stage=sys.argv[1:]
with open(path,"rb") as f:data=plistlib.load(f)
seen=False
for key,target in data.items():
    if isinstance(target,dict) and ("ImportHelperUITests" in key or "Test" in key):
        env=target.get("EnvironmentVariables",{})
        env["CHERRI_STAGE"]=stage
        env.pop("CHERRI_MENU_TARGET",None)
        target["EnvironmentVariables"]=env
        seen=True
if not seen:raise SystemExit("Could not identify XCTest target environment")
with open(path,"wb") as f:plistlib.dump(data,f)
PY
    set -o pipefail
    xcodebuild test-without-building -xctestrun "$RUNFILE" -destination "id=$SIM_UDID" -maximum-test-execution-time-allowance 45 -resultBundlePath "$bundle" CODE_SIGN_IDENTITY="-" 2>&1 | tee "$logfile"
}
# Permissions/onboarding may be dismissed only through explicit helper stages.
run_helper onboarding
SIGNED_ABS=$(cd "$(dirname "$SIGNED")" && pwd)/$(basename "$SIGNED")
xcrun simctl openurl "$SIM_UDID" "file://$SIGNED_ABS"
sleep 4
run_helper import_cloud
sleep 3
xcrun simctl io "$SIM_UDID" screenshot artifacts/simulator-after-import.png || true

SENTINEL="CHERRI_CLOUD_BEFORE_$GITHUB_RUN_ID"
printf '%s' "$SENTINEL" | xcrun simctl pbcopy "$SIM_UDID"
URL=$(python3 - "$NAME" <<'PY'
import sys,urllib.parse
print("shortcuts://run-shortcut?name="+urllib.parse.quote(sys.argv[1],safe=""))
PY
)
xcrun simctl openurl "$SIM_UDID" "$URL"
RUNTIME_RESULT="$SENTINEL"
for i in $(seq 1 35); do
    RUNTIME_RESULT=$(xcrun simctl pbpaste "$SIM_UDID" 2>/dev/null || true)
    if [ "$RUNTIME_RESULT" != "$SENTINEL" ]; then break; fi
    if [ "$i" -eq 4 ] || [ "$i" -eq 11 ]; then
        run_helper run_permission || true
    fi
    sleep 1
done
printf '%s' "$RUNTIME_RESULT" > artifacts/simulator-clipboard.txt
xcrun simctl io "$SIM_UDID" screenshot artifacts/simulator-after-run.png || true
python3 - "$SENTINEL" "$RUNTIME_RESULT" "$SIM_UDID" <<'PY'
import json,sys
before,after,device=sys.argv[1:]
observed=after!=before
result={
 "status":"output_observed" if observed else "triggered_no_observable_output",
 "run_url_triggered":True,
 "clipboard_changed":observed,
 "simulator_udid":device,
 "runtime_tested":True,
 "shortcut_completed":None if not observed else "observable_output",
 "limitation":"This generic runner cannot prove completion of arbitrary non-clipboard/interactive workflows.",
}
with open("artifacts/simulator-result.json","w",encoding="utf-8") as f:json.dump(result,f,indent=2)
print("SIMULATOR_STATUS="+result["status"])
PY
