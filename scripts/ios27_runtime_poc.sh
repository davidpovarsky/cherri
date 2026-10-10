#!/usr/bin/env bash
set -e

mkdir -p artifacts
LOG_PID=""
HTTP_PID=""
SIM_UDID=""

cleanup() {
    echo "=== CLEANUP ==="
    if [ -n "$HTTP_PID" ]; then
        kill "$HTTP_PID" 2>/dev/null || true
    fi
    if [ -n "$LOG_PID" ]; then
        kill "$LOG_PID" 2>/dev/null || true
    fi
}
trap cleanup EXIT

take_failure_screenshot() {
    if [ -n "$SIM_UDID" ]; then
        echo "Capturing failure screenshot..."
        xcrun simctl io "$SIM_UDID" screenshot artifacts/failure.png 2>/dev/null || true
    fi
}

run_ui_helper() {
    local stage_name="$1"
    echo "Running UI helper for $stage_name..."
    
    # 1. Attempt AppleScript keystrokes if Simulator GUI is available
    open -a Simulator --args -CurrentDeviceUDID "$SIM_UDID" 2>/dev/null || true
    sleep 1
    osascript -e 'tell application "Simulator" to activate' 2>/dev/null || true
    osascript -e 'tell application "System Events" to key code 36' 2>/dev/null || true # Return
    osascript -e 'tell application "System Events" to key code 49' 2>/dev/null || true # Space
    
    # 2. Execute XCUITest helper
    if [ -d tests/runtime_poc/ImportHelper/ImportHelper.xcodeproj ]; then
        echo "Executing XCUITest helper via xcodebuild..."
        (
            cd tests/runtime_poc/ImportHelper
            XCTESTRUN=$(find .build -name "*.xctestrun" 2>/dev/null | head -n 1)
            if [ -n "$XCTESTRUN" ] && [ -f "$XCTESTRUN" ]; then
                echo "Running test-without-building with $XCTESTRUN..."
                xcodebuild test-without-building \
                    -xctestrun "$XCTESTRUN" \
                    -destination "id=$SIM_UDID" \
                    -resultBundlePath "${PWD}/../../../artifacts/ImportHelper_${stage_name}.xcresult" \
                    CODE_SIGN_IDENTITY="-" \
                    2>&1 | tee "${PWD}/../../../artifacts/ui_helper_${stage_name}.log" || true
            else
                xcodebuild test \
                    -project ImportHelper.xcodeproj \
                    -scheme ImportHelper \
                    -destination "id=$SIM_UDID" \
                    -derivedDataPath .build \
                    -resultBundlePath "${PWD}/../../../artifacts/ImportHelper_${stage_name}.xcresult" \
                    CODE_SIGN_IDENTITY="-" \
                    2>&1 | tee "${PWD}/../../../artifacts/ui_helper_${stage_name}.log" || true
            fi
        )
    elif command -v xcodegen >/dev/null 2>&1 && [ -f tests/runtime_poc/ImportHelper/project.yml ]; then
        echo "Generating and executing XCUITest helper..."
        (
            cd tests/runtime_poc/ImportHelper
            xcodegen generate 2>&1 | tee "${PWD}/../../../artifacts/xcodegen_${stage_name}.log" || true
            xcodebuild test \
                -project ImportHelper.xcodeproj \
                -scheme ImportHelper \
                -destination "id=$SIM_UDID" \
                -derivedDataPath .build \
                -resultBundlePath "${PWD}/../../../artifacts/ImportHelper_${stage_name}.xcresult" \
                CODE_SIGN_IDENTITY="-" \
                2>&1 | tee "${PWD}/../../../artifacts/ui_helper_${stage_name}.log" || true
        )
    fi
}

echo "=================================================="
echo "   CHERRI iOS 27 SHORTCUTS RUNTIME PoC"
echo "=================================================="

# [1] ENVIRONMENT
echo ""
echo "=== [1] ENVIRONMENT ==="
sw_vers | tee artifacts/runtime-info.txt
xcodebuild -version | tee -a artifacts/runtime-info.txt
xcrun simctl list runtimes | tee -a artifacts/runtime-info.txt
xcrun simctl list devices available | tee artifacts/simulator-devices.txt

echo "Shortcuts CLI check:"
which shortcuts || true
shortcuts --help || true

# [2] BOOT SIMULATOR
echo ""
echo "=== [2] BOOT SIMULATOR ==="
python3 - << 'EOF'
import json, subprocess, sys

cmd = ['xcrun', 'simctl', 'list', 'devices', 'available', '-j']
data = json.loads(subprocess.check_output(cmd))
devices = data.get('devices', {})

selected_udid = None
selected_name = None
selected_runtime = None

# 1. Search available devices for an iOS 27 runtime
for runtime_id, dev_list in devices.items():
    if 'iOS-27' in runtime_id or 'iOS 27' in runtime_id or 'iOS.27' in runtime_id:
        for dev in dev_list:
            if dev.get('isAvailable', False) and 'iPhone' in dev.get('name', ''):
                selected_udid = dev['udid']
                selected_name = dev['name']
                selected_runtime = runtime_id
                break
    if selected_udid:
        break

# 2. Check any available device with 27 in runtime if iPhone match was not found
if not selected_udid:
    for runtime_id, dev_list in devices.items():
        if '27' in runtime_id:
            for dev in dev_list:
                if dev.get('isAvailable', False):
                    selected_udid = dev['udid']
                    selected_name = dev['name']
                    selected_runtime = runtime_id
                    break
        if selected_udid:
            break

# 3. If no pre-created device exists, check if iOS 27 runtime exists to create one
if not selected_udid:
    runtimes_data = json.loads(subprocess.check_output(['xcrun', 'simctl', 'list', 'runtimes', '-j']))
    target_runtime = None
    for r in runtimes_data.get('runtimes', []):
        if ('iOS 27' in r.get('name', '') or 'iOS-27' in r.get('identifier', '')) and r.get('isAvailable', False):
            target_runtime = r['identifier']
            break
    if target_runtime:
        print(f"Creating new iPhone on runtime {target_runtime}...")
        new_udid = subprocess.check_output(['xcrun', 'simctl', 'create', 'Cherri-iOS27-iPhone', 'iPhone 16', target_runtime]).decode().strip()
        selected_udid = new_udid
        selected_name = 'Cherri-iOS27-iPhone'
        selected_runtime = target_runtime

if not selected_udid:
    print("FATAL: Could not find or create an iOS 27 Simulator device.", file=sys.stderr)
    sys.exit(1)

print(f"SELECTED DEVICE: {selected_name} ({selected_udid})")
print(f"RUNTIME: {selected_runtime}")

with open('artifacts/selected_device.txt', 'w') as f:
    f.write(f"UDID={selected_udid}\nNAME={selected_name}\nRUNTIME={selected_runtime}\n")
EOF

SIM_UDID=$(grep '^UDID=' artifacts/selected_device.txt | cut -d= -f2)
SIM_NAME=$(grep '^NAME=' artifacts/selected_device.txt | cut -d= -f2)
SIM_RUNTIME=$(grep '^RUNTIME=' artifacts/selected_device.txt | cut -d= -f2)

echo "Selected device: $SIM_NAME ($SIM_UDID) running $SIM_RUNTIME"
echo "Booting simulator..."
xcrun simctl boot "$SIM_UDID" || {
    echo "Boot returned non-zero (might already be booted), checking status..."
}
xcrun simctl bootstatus "$SIM_UDID" -b
echo "Simulator booted successfully."
sleep 3

# [3] VERIFY SHORTCUTS APP
echo ""
echo "=== [3] VERIFY SHORTCUTS APP ==="
SHORTCUTS_APP_ID="com.apple.shortcuts"
echo "Attempting to launch $SHORTCUTS_APP_ID on $SIM_UDID..."
set +e
LAUNCH_OUT=$(xcrun simctl launch "$SIM_UDID" "$SHORTCUTS_APP_ID" 2>&1)
LAUNCH_EXIT=$?
set -e
echo "Launch exit code: $LAUNCH_EXIT, output: $LAUNCH_OUT"
if [ $LAUNCH_EXIT -ne 0 ]; then
    echo "Shortcuts launch failed. Inspecting installed simulator applications:"
    xcrun simctl listapps "$SIM_UDID" | tee artifacts/installed_apps.txt || true
    take_failure_screenshot
    echo "IOS27_SHORTCUTS_RUNTIME_POC=FAIL BLOCKER=Shortcuts.app failed to launch on iOS 27 simulator"
    exit 1
fi

sleep 2
xcrun simctl io "$SIM_UDID" screenshot artifacts/01_shortcuts_app.png || true

# Dismiss onboarding "What's New in Shortcuts"
echo "Dismissing onboarding if displayed..."
run_ui_helper "onboarding"
sleep 2
xcrun simctl io "$SIM_UDID" screenshot artifacts/01b_shortcuts_ready.png || true

# [4] BUILD CHERRI
echo ""
echo "=== [4] BUILD CHERRI ==="
go build -v -o cherri . 2>&1 | tee artifacts/build.log
./cherri --version

# [5] COMPILE POC
echo ""
echo "=== [5] COMPILE POC ==="
cp tests/runtime_poc/CherriRuntimePOC.cherri artifacts/CherriRuntimePOC.cherri
./cherri tests/runtime_poc/CherriRuntimePOC.cherri --skip-sign -d 2>&1 | tee artifacts/compile.log

cp -f tests/runtime_poc/CherriRuntimePOC_unsigned.shortcut artifacts/poc_unsigned.shortcut 2>/dev/null || true
cp -f tests/runtime_poc/CherriRuntimePOC_unsigned.shortcut artifacts/CherriRuntimePOC_unsigned.shortcut 2>/dev/null || true
cp -f tests/runtime_poc/CherriRuntimePOC.plist artifacts/CherriRuntimePOC.plist 2>/dev/null || true
cp -f tests/runtime_poc/CherriRuntimePOC.plist artifacts/poc.plist 2>/dev/null || true
cp -f tests/runtime_poc/CherriRuntimePOC_processed.cherri artifacts/CherriRuntimePOC_processed.cherri 2>/dev/null || true

if [ ! -f artifacts/poc_unsigned.shortcut ]; then
    echo "FATAL: artifacts/poc_unsigned.shortcut was not generated."
    take_failure_screenshot
    echo "IOS27_SHORTCUTS_RUNTIME_POC=FAIL BLOCKER=Compiler failed to produce poc_unsigned.shortcut"
    exit 1
fi
ls -la artifacts/

# [6] SIGN
echo ""
echo "=== [6] SIGN ==="
echo "Attempting native macOS shortcuts sign..."
set +e
shortcuts sign -i artifacts/poc_unsigned.shortcut -o artifacts/poc_signed.shortcut -m anyone > artifacts/sign.log 2>&1
SIGN_EXIT=$?
set -e
echo "Native shortcuts sign exit code: $SIGN_EXIT"
cat artifacts/sign.log

if [ $SIGN_EXIT -ne 0 ] || [ ! -f artifacts/poc_signed.shortcut ]; then
    echo "Native shortcuts sign failed or output missing. Attempting Cherri HubSign fallback..."
    ./cherri tests/runtime_poc/CherriRuntimePOC.cherri -s=anyone -d 2>&1 | tee -a artifacts/sign.log
    cp -f tests/runtime_poc/CherriRuntimePOC.shortcut artifacts/poc_signed.shortcut 2>/dev/null || true
fi

if [ ! -f artifacts/poc_signed.shortcut ]; then
    echo "FATAL: artifacts/poc_signed.shortcut was not generated by native sign or HubSign."
    take_failure_screenshot
    echo "IOS27_SHORTCUTS_RUNTIME_POC=FAIL BLOCKER=SIGNING_FAILED"
    exit 1
fi

# Assert signed payload has genuine AEA1 magic header
HEADER=$(head -c 4 artifacts/poc_signed.shortcut 2>/dev/null || true)
if [ "$HEADER" != "AEA1" ]; then
    echo "FATAL: artifacts/poc_signed.shortcut lacks AEA1 magic header (got '$HEADER'). Unsigned shortcut rejected."
    take_failure_screenshot
    echo "IOS27_SHORTCUTS_RUNTIME_POC=FAIL BLOCKER=SIGNING_FAILED Payload lacks AEA1 magic"
    exit 1
fi
echo "Signed payload verified: valid AEA1 header present."

cp -f artifacts/poc_signed.shortcut artifacts/CherriRuntimePOC.shortcut

file artifacts/poc_signed.shortcut || true
ls -lh artifacts/poc_unsigned.shortcut artifacts/poc_signed.shortcut artifacts/CherriRuntimePOC.shortcut

# [7] IMPORT
echo ""
echo "=== [7] IMPORT ==="
# Start streaming unified logs from simulator
xcrun simctl spawn "$SIM_UDID" log stream --predicate 'subsystem == "com.apple.shortcuts" || subsystem == "com.apple.WorkflowKit" || process == "Shortcuts" || process == "SpringBoard"' --level debug > artifacts/shortcuts-runtime.log 2>&1 &
LOG_PID=$!
echo "Simulator log stream active with PID $LOG_PID"



SHORTCUT_FILE="$(pwd)/artifacts/CherriRuntimePOC.shortcut"

echo "Opening shortcut file in simulator..."
xcrun simctl openurl "$SIM_UDID" "file://$SHORTCUT_FILE" > artifacts/import.log 2>&1 || true
open -a Simulator "$SHORTCUT_FILE" 2>/dev/null || true
sleep 3
xcrun simctl io "$SIM_UDID" screenshot artifacts/02_import_sheet_presented.png || true

echo "Confirming import via UI helper..."
run_ui_helper "import"
sleep 3
xcrun simctl io "$SIM_UDID" screenshot artifacts/02_after_import.png || true

# [8] RUN & [9] ASSERT CLIPBOARD
echo ""
echo "=== [8] RUN & [9] ASSERT CLIPBOARD ==="
# Reset clipboard to known non-matching value before execution
echo -n "BEFORE_RUNTIME_TEST" | xcrun simctl pbcopy "$SIM_UDID"
BEFORE_CLIP=$(xcrun simctl pbpaste "$SIM_UDID" 2>/dev/null || true)
echo "Simulator clipboard before execution: '$BEFORE_CLIP'"

RUN_URL="shortcuts://run-shortcut?name=CherriRuntimePOC"
echo "Triggering run URL via openurl: $RUN_URL"
xcrun simctl openurl "$SIM_UDID" "$RUN_URL" > artifacts/run.log 2>&1
RUN_EXIT=$?
echo "simctl openurl run exit code: $RUN_EXIT"
cat artifacts/run.log

ASSERT_PASSED=false
UI_AUTOMATION_USED="ImportHelperUITests + AppleScript"

echo "Polling simulator clipboard for up to 30 seconds..."
for i in $(seq 1 30); do
    CLIP_VAL=$(xcrun simctl pbpaste "$SIM_UDID" 2>/dev/null || true)
    echo "Poll $i/30: '$CLIP_VAL'"
    if echo "$CLIP_VAL" | grep -q "CHERRI_IOS27_RUNTIME_OK"; then
        if echo "$CLIP_VAL" | grep -q "VAR=updated" && \
           echo "$CLIP_VAL" | grep -q "IF=true" && \
           echo "$CLIP_VAL" | grep -q "LOOPS=0:A:0:1|0:A:1:2|1:B:0:1|1:B:1:2" && \
           echo "$CLIP_VAL" | grep -q "FUNCTION=21" && \
           echo "$CLIP_VAL" | grep -q "BASE64=Q0hFUlJJ"; then
            ASSERT_PASSED=true
            echo "ASSERTION PASSED on poll $i: Expected v2 conformance markers detected (VAR=updated, IF=true, LOOPS=..., FUNCTION=21, BASE64=Q0hFUlJJ)!"
            break
        fi
    fi

    # If not passed by poll 3, check for runtime permission dialog (Allow)
    if [ "$i" -eq 3 ]; then
        echo "Poll $i: Checking for runtime permission dialog (Allow)..."
        run_ui_helper "run_permission"
        xcrun simctl openurl "$SIM_UDID" "$RUN_URL" 2>/dev/null || true
    fi
    sleep 1
done

echo "$CLIP_VAL" > artifacts/clipboard-result.txt
xcrun simctl io "$SIM_UDID" screenshot artifacts/03_after_run.png || true

# [10] COLLECT ARTIFACTS
echo ""
echo "=== [10] COLLECT ARTIFACTS ==="
cleanup

echo "=== SUMMARY OF RESULTS ==="
echo "Device: $SIM_NAME ($SIM_UDID)"
echo "Runtime: $SIM_RUNTIME"
echo "Initial Clipboard: $BEFORE_CLIP"
echo "Final Clipboard: $CLIP_VAL"
echo "UI Automation Used: $UI_AUTOMATION_USED"

if [ "$ASSERT_PASSED" = "true" ]; then
    HEAD_SHA=$(git rev-parse HEAD 2>/dev/null || echo "")
    cat <<EOF > artifacts/runtime-evidence.json
[
  {
    "evidence_id": "ev-runtime-smoke",
    "requirements": ["BRG24"],
    "tier": "ios-runtime",
    "test_id": "ios27-runtime-poc:smoke",
    "implementation_sha": "$HEAD_SHA",
    "result": "passed",
    "exit_code": 0,
    "assertions": {
      "status": "CHERRI_IOS27_RUNTIME_OK"
    },
    "detail": "Simulator executed imported Cherri shortcut successfully"
  },
  {
    "evidence_id": "ev-runtime-variables",
    "requirements": ["V01", "V02", "V03", "RP03"],
    "tier": "ios-runtime",
    "test_id": "ios27-runtime-poc:variables",
    "implementation_sha": "$HEAD_SHA",
    "result": "passed",
    "exit_code": 0,
    "assertions": {
      "variable_result": "updated"
    },
    "detail": "Variable mutation and read verified in iOS Shortcuts runtime"
  },
  {
    "evidence_id": "ev-runtime-conditionals",
    "requirements": ["F01", "RP13"],
    "tier": "ios-runtime",
    "test_id": "ios27-runtime-poc:conditionals",
    "implementation_sha": "$HEAD_SHA",
    "result": "passed",
    "exit_code": 0,
    "assertions": {
      "conditional_result": "true"
    },
    "detail": "Conditional branch (10 > 5) calculated and verified in iOS Shortcuts runtime"
  },
  {
    "evidence_id": "ev-runtime-loops",
    "requirements": ["F02", "F04", "RP14"],
    "tier": "ios-runtime",
    "test_id": "ios27-runtime-poc:nested-loops",
    "implementation_sha": "$HEAD_SHA",
    "result": "passed",
    "exit_code": 0,
    "assertions": {
      "loop_trace": "0:A:0:1|0:A:1:2|1:B:0:1|1:B:1:2|"
    },
    "detail": "Nested loops with outer/inner item and index variables verified in iOS Shortcuts runtime"
  },
  {
    "evidence_id": "ev-runtime-functions",
    "requirements": ["FN01", "FN02", "FN03", "FN04"],
    "tier": "ios-runtime",
    "test_id": "ios27-runtime-poc:functions",
    "implementation_sha": "$HEAD_SHA",
    "result": "passed",
    "exit_code": 0,
    "assertions": {
      "function_result": "21"
    },
    "detail": "Function definition with parameters and return value execution verified in iOS Shortcuts runtime"
  },
  {
    "evidence_id": "ev-runtime-static-variants",
    "requirements": ["RP08", "BRG17"],
    "tier": "ios-runtime",
    "test_id": "ios27-runtime-poc:static-variants",
    "implementation_sha": "$HEAD_SHA",
    "result": "passed",
    "exit_code": 0,
    "assertions": {
      "base64_result": "Q0hFUlJJ"
    },
    "detail": "Static variant Base64 encoding verified in iOS Shortcuts runtime"
  }
]
EOF
    echo "=================================================="
    echo "IOS27_SHORTCUTS_RUNTIME_POC=PASS"
    echo "=================================================="
    exit 0
else
    take_failure_screenshot
    echo "=================================================="
    echo "IOS27_SHORTCUTS_RUNTIME_POC=FAIL BLOCKER=Clipboard value was '$CLIP_VAL', expected 'CHERRI_IOS27_RUNTIME_OK'"
    echo "=================================================="
    exit 1
fi
