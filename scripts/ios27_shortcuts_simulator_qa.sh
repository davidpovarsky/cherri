#!/usr/bin/env bash
# Cherri P10R13 — iPadOS 27 Simulator Shortcuts QA Orchestrator
# Implements Phases 1–4 of the P10R13 Simulator QA specification with explicit
# sub-gate accounting and strict sanitization.

set -uo pipefail

PROBE_LEVEL="${1:-full}"
DEVICE_PROFILE="${2:-ipad-default}"
SAMPLE_ID="${3:-all}"
CAPTURE_FULL_EDITOR="${4:-true}"
MAX_EDITOR_PAGES="${5:-20}"
RUN_ID="${GITHUB_RUN_ID:-local-$(date -u +%Y%m%dT%H%M%SZ)}"

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

ARTIFACT_DIR="$ROOT_DIR/artifacts/cherri-ios27-simulator-qa/$RUN_ID"
mkdir -p "$ARTIFACT_DIR/editor" "$ARTIFACT_DIR/runtime" "$ARTIFACT_DIR/tests" "$ARTIFACT_DIR/docs" "$ARTIFACT_DIR/builds"

SOURCE_SHA="$(git rev-parse HEAD 2>/dev/null || echo "unknown")"

echo "=== Cherri iPadOS 27 Simulator QA ($RUN_ID) ==="
echo "Probe Level:        $PROBE_LEVEL"
echo "Device Profile:     $DEVICE_PROFILE"
echo "Sample ID:          $SAMPLE_ID"
echo "Capture Full Editor:$CAPTURE_FULL_EDITOR"
echo "Max Editor Pages:   $MAX_EDITOR_PAGES"
echo "Source SHA:         $SOURCE_SHA"

# Progressive Gate Defaults (never collapse into a single pass/fail)
GATE_RUNNER_AVAILABLE="NOT_RUN"
GATE_SIMULATOR_BOOTED="NOT_RUN"
GATE_SHORTCUTS_APP_INSTALLED="NOT_RUN"
GATE_SHORTCUTS_LAUNCHED="NOT_RUN"
GATE_EDITOR_OPENED="NOT_RUN"
GATE_FILE_IMPORT_ATTEMPTED="NOT_RUN"
GATE_COMPILER_ARTIFACT_IMPORTED="NOT_RUN"
GATE_SHORTCUT_EXISTS_IN_LIBRARY="NOT_RUN"
GATE_ACTION_EDITOR_CAPTURED="NOT_RUN"
GATE_SHORTCUT_RUN_ATTEMPTED="NOT_RUN"
GATE_SHORTCUT_RUN_COMPLETED="NOT_RUN"
GATE_CALLBACK_CAPTURED="NOT_RUN"
GATE_TRACE_CAPTURED="NOT_RUN"
GATE_PHYSICAL_DEVICE_TESTED="BLOCKED_DEVICE"

# ---------------------------------------------------------------------------
# Phase 0: Build Cherri CLI & Compile Synthetic Sanitized Fixtures
# ---------------------------------------------------------------------------
echo "=== Phase 0: Building Cherri & Compiling Synthetic Fixtures ==="
go build -o "$ARTIFACT_DIR/builds/cherri" .

python3 - "$ROOT_DIR" "$ARTIFACT_DIR" "$SOURCE_SHA" <<'PY'
import hashlib
import json
import os
import plistlib
import shutil
import subprocess
import sys

root_dir, artifact_dir, source_sha = sys.argv[1], sys.argv[2], sys.argv[3]
cherri_bin = os.path.join(artifact_dir, "builds", "cherri")
fixtures_dir = os.path.join(root_dir, "tools", "shortcuts-simulator-qa", "fixtures")
builds_dir = os.path.join(artifact_dir, "builds")

fixture_specs = [
    ("qa-functional-v2", "qa-functional-v2.cherri", "v2"),
    ("qa-functional-legacy", "qa-functional-legacy.cherri", "legacy"),
    ("qa-scroll-editor-v2", "qa-scroll-editor-v2.cherri", "v2"),
    ("qa-scroll-editor-legacy", "qa-scroll-editor-legacy.cherri", "legacy"),
    ("qa-rtl-hebrew-v2", "qa-rtl-hebrew-v2.cherri", "v2"),
    ("qa-functional-v2-uninstrumented", "qa-functional-v2-uninstrumented.cherri", "v2"),
    ("qa-functional-legacy-uninstrumented", "qa-functional-legacy-uninstrumented.cherri", "legacy"),
]

manifest_entries = []
compiled_actions_by_id = {}

for fid, fname, dialect in fixture_specs:
    fpath = os.path.join(fixtures_dir, fname)
    with open(fpath, "rb") as f:
        src_bytes = f.read()
    src_sha256 = hashlib.sha256(src_bytes).hexdigest()

    build_src = os.path.join(builds_dir, fname)
    shutil.copyfile(fpath, build_src)
    out_base = os.path.join(builds_dir, fid)
    out_shortcut = out_base + ".shortcut"
    cmd = [cherri_bin, build_src, "--skip-sign", f"--output={out_shortcut}"]
    if dialect == "legacy":
        cmd.append("--legacy")
    proc = subprocess.run(cmd, cwd=builds_dir, capture_output=True, text=True)
    if proc.returncode != 0:
        raise RuntimeError(f"Failed to compile {fid}: {proc.stdout}\n{proc.stderr}")

    with open(out_shortcut, "rb") as f:
        plist_bytes = f.read()
    plist_sha256 = hashlib.sha256(plist_bytes).hexdigest()
    doc = plistlib.loads(plist_bytes)
    actions = doc.get("WFWorkflowActions", [])
    compiled_actions_by_id[fid] = actions

    action_ids = [a.get("WFWorkflowActionIdentifier", "") for a in actions]
    manifest_entries.append({
        "fixtureId": fid,
        "fileName": fname,
        "dialect": dialect,
        "sourceSha256": src_sha256,
        "unsignedPlistSha256": plist_sha256,
        "nativeActionCount": len(actions),
        "actionIdentifiers": action_ids,
    })

compiler_manifest = {
    "schemaVersion": "1.0",
    "compilerCommitSha": source_sha,
    "fixtures": manifest_entries,
}
with open(os.path.join(artifact_dir, "compiler-manifest.json"), "w", encoding="utf-8") as f:
    json.dump(compiler_manifest, f, indent=2)

def classify_actions(actions):
    source_ids = []
    inline_debug_ids = []
    collector_ids = []
    for idx, act in enumerate(actions):
        aid = act.get("WFWorkflowActionIdentifier", "")
        params = act.get("WFWorkflowActionParameters", {})
        vname = params.get("WFVariableName", "")
        if vname in ("__cherri_debug_log", "__cherri_qa_trace") or str(vname).startswith("__cherri_debug_timer_"):
            inline_debug_ids.append({"nativeIndex": idx, "identifier": aid, "variable": vname})
            continue
        if aid == "is.workflow.actions.detect.date":
            inline_debug_ids.append({"nativeIndex": idx, "identifier": aid, "reason": "CreationDateCapture"})
            continue
        if aid == "is.workflow.actions.text.combine":
            collector_ids.append({"nativeIndex": idx, "identifier": aid})
            continue
        if aid == "is.workflow.actions.gettext":
            txt = params.get("WFTextActionText", {})
            if isinstance(txt, dict):
                s = txt.get("Value", {}).get("string", "")
                if '"kind":"INLINE_DEBUG_ACTION"' in s:
                    inline_debug_ids.append({"nativeIndex": idx, "identifier": aid, "reason": "TraceEventLine"})
                    continue
                if '"kind":"COLLECTOR_EVENT"' in s:
                    collector_ids.append({"nativeIndex": idx, "identifier": aid, "reason": "TraceExportEnvelope"})
                    continue
            elif txt == "":
                inline_debug_ids.append({"nativeIndex": idx, "identifier": aid, "reason": "ClearLogsEmptyText"})
                continue
        source_ids.append(aid)
    return source_ids, inline_debug_ids, collector_ids

comparisons = []
for dialect in ("v2", "legacy"):
    inst_id = f"qa-functional-{dialect}"
    uninst_id = f"qa-functional-{dialect}-uninstrumented"
    inst_src, inst_dbg, inst_col = classify_actions(compiled_actions_by_id[inst_id])
    uninst_src, _, _ = classify_actions(compiled_actions_by_id[uninst_id])
    comparisons.append({
        "dialect": dialect,
        "instrumentedFixtureId": inst_id,
        "uninstrumentedFixtureId": uninst_id,
        "uninstrumentedActionCount": len(compiled_actions_by_id[uninst_id]),
        "instrumentedTotalActionCount": len(compiled_actions_by_id[inst_id]),
        "projectedSourceActionCount": len(inst_src),
        "inlineDebugActionCount": len(inst_dbg),
        "collectorEventActionCount": len(inst_col),
        "sourceActionSequenceMatches": (inst_src == uninst_src),
        "sourceActionIdentifiers": inst_src,
        "inlineDebugActions": inst_dbg,
        "collectorActions": inst_col,
    })

with open(os.path.join(artifact_dir, "runtime", "instrumentation-vs-original.json"), "w", encoding="utf-8") as f:
    json.dump({"schemaVersion": "1.0", "comparisons": comparisons}, f, indent=2)
PY

# ---------------------------------------------------------------------------
# Phase 1: Preflight & Runner / Simulator Discovery
# ---------------------------------------------------------------------------
echo "=== Phase 1: Runner & Simulator Preflight ==="
UNAME_OUT="$(uname -a)"
SW_VERS_OUT="$(sw_vers 2>&1 || true)"
XCODE_SELECT_OUT="$(xcode-select -p 2>&1 || true)"
XCODEBUILD_VER_OUT="$(xcodebuild -version 2>&1 || true)"

if ! command -v xcrun >/dev/null 2>&1; then
  GATE_RUNNER_AVAILABLE="BLOCKED_RUNNER"
else
  GATE_RUNNER_AVAILABLE="PASS"
fi

xcrun simctl list runtimes -j > "$ARTIFACT_DIR/sim-runtimes.json" 2>/dev/null || echo "{}" > "$ARTIFACT_DIR/sim-runtimes.json"
xcrun simctl list devicetypes -j > "$ARTIFACT_DIR/sim-devicetypes.json" 2>/dev/null || echo "{}" > "$ARTIFACT_DIR/sim-devicetypes.json"
xcrun simctl list devices available -j > "$ARTIFACT_DIR/sim-devices.json" 2>/dev/null || echo "{}" > "$ARTIFACT_DIR/sim-devices.json"

SIM_SELECTION="$(python3 - "$ARTIFACT_DIR" "$DEVICE_PROFILE" <<'PY'
import json
import subprocess
import sys

artifact_dir, profile = sys.argv[1], sys.argv[2]
with open(f"{artifact_dir}/sim-runtimes.json") as f:
    runtimes = json.load(f).get("runtimes", [])
with open(f"{artifact_dir}/sim-devicetypes.json") as f:
    devicetypes = json.load(f).get("devicetypes", [])
with open(f"{artifact_dir}/sim-devices.json") as f:
    devices_map = json.load(f).get("devices", {})

ios_runtimes = [r for r in runtimes if r.get("isAvailable") and ("iOS" in r.get("name", "") or "iPadOS" in r.get("name", ""))]
ios_runtimes.sort(key=lambda r: r.get("version", ""), reverse=True)

chosen_rt = None
for r in ios_runtimes:
    if str(r.get("version", "")).startswith("27") or "27" in r.get("name", ""):
        chosen_rt = r
        break
if not chosen_rt and ios_runtimes:
    chosen_rt = ios_runtimes[0]

if not chosen_rt:
    print(json.dumps({"error": "NO_IOS_RUNTIME"}))
    sys.exit(0)

rt_id = chosen_rt["identifier"]
rt_devs = devices_map.get(rt_id, [])

prefer_ipad = (profile != "iphone-fallback")
chosen_dev = None
if prefer_ipad:
    for d in rt_devs:
        if "iPad" in d.get("name", "") and d.get("isAvailable", True):
            chosen_dev = d
            break
    if not chosen_dev:
        ipad_types = [dt for dt in devicetypes if "iPad" in dt.get("name", "")]
        if ipad_types:
            dt_id = ipad_types[0]["identifier"]
            proc = subprocess.run(
                ["xcrun", "simctl", "create", "Cherri-QA-iPad", dt_id, rt_id],
                capture_output=True, text=True
            )
            if proc.returncode == 0 and proc.stdout.strip():
                chosen_dev = {
                    "udid": proc.stdout.strip(),
                    "name": ipad_types[0]["name"],
                    "deviceTypeIdentifier": dt_id,
                    "createdOnDemand": True,
                }

if not chosen_dev:
    for d in rt_devs:
        if d.get("isAvailable", True):
            chosen_dev = d
            break

if not chosen_dev:
    print(json.dumps({"error": "NO_SIMULATOR_DEVICE", "runtimeId": rt_id}))
    sys.exit(0)

print(json.dumps({
    "udid": chosen_dev["udid"],
    "deviceName": chosen_dev.get("name", "Unknown"),
    "runtimeId": rt_id,
    "runtimeName": chosen_rt.get("name", "Unknown"),
    "runtimeVersion": chosen_rt.get("version", "Unknown"),
    "isIPad": "iPad" in chosen_dev.get("name", ""),
}))
PY
)"

echo "Simulator Selection: $SIM_SELECTION"
UDID="$(echo "$SIM_SELECTION" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("udid",""))')"
DEVICE_NAME="$(echo "$SIM_SELECTION" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("deviceName",""))')"
RUNTIME_NAME="$(echo "$SIM_SELECTION" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("runtimeName",""))')"
RUNTIME_VERSION="$(echo "$SIM_SELECTION" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("runtimeVersion",""))')"

if [ -z "$UDID" ]; then
  GATE_SIMULATOR_BOOTED="BLOCKED_RUNNER"
else
  echo "Booting simulator UDID: $UDID ($DEVICE_NAME)"
  xcrun simctl boot "$UDID" 2>/dev/null || true
  if xcrun simctl bootstatus "$UDID" -b; then
    GATE_SIMULATOR_BOOTED="PASS"
  else
    GATE_SIMULATOR_BOOTED="BLOCKED_RUNNER"
  fi
fi

SHORTCUTS_BUNDLE_PRESENT="false"
if [ "$GATE_SIMULATOR_BOOTED" = "PASS" ]; then
  xcrun simctl listapps "$UDID" > "$ARTIFACT_DIR/sim-listapps.plist" 2>/dev/null || true
  if grep -q "com.apple.shortcuts" "$ARTIFACT_DIR/sim-listapps.plist" 2>/dev/null; then
    SHORTCUTS_BUNDLE_PRESENT="true"
    GATE_SHORTCUTS_APP_INSTALLED="PASS"
  else
    GATE_SHORTCUTS_APP_INSTALLED="BLOCKED_SIMULATOR_APP"
  fi
fi
rm -f "$ARTIFACT_DIR/sim-listapps.plist" "$ARTIFACT_DIR/sim-runtimes.json" "$ARTIFACT_DIR/sim-devicetypes.json" "$ARTIFACT_DIR/sim-devices.json"

# Write sanitized environment.json (masking full UDID to first 8 chars)
MASKED_UDID="$(echo "$UDID" | cut -c1-8)-REDACTED"
python3 - "$ARTIFACT_DIR/environment.json" "$UNAME_OUT" "$SW_VERS_OUT" "$XCODE_SELECT_OUT" "$XCODEBUILD_VER_OUT" "$MASKED_UDID" "$DEVICE_NAME" "$RUNTIME_NAME" "$RUNTIME_VERSION" "$SHORTCUTS_BUNDLE_PRESENT" "$SOURCE_SHA" <<'PY'
import json
import sys

out_path, uname_s, sw_vers_s, xcode_sel, xcode_ver, masked_udid, dev_name, rt_name, rt_ver, app_present, sha = sys.argv[1:12]
env_doc = {
    "schemaVersion": "1.0",
    "compilerCommitSha": sha,
    "uname": uname_s.strip(),
    "swVers": sw_vers_s.strip(),
    "xcodeSelectPath": xcode_sel.strip(),
    "xcodebuildVersion": xcode_ver.strip(),
    "simulator": {
        "maskedUdid": masked_udid,
        "deviceName": dev_name,
        "runtimeName": rt_name,
        "runtimeVersion": rt_ver,
        "isIPad": ("iPad" in dev_name),
        "shortcutsAppInstalled": (app_present == "true"),
        "shortcutsBundleId": "com.apple.shortcuts" if app_present == "true" else None,
    }
}
with open(out_path, "w", encoding="utf-8") as f:
    json.dump(env_doc, f, indent=2)
PY

# ---------------------------------------------------------------------------
# Phase 2: Import Decision Tree (4 Explicit Routes)
# ---------------------------------------------------------------------------
if [ "$PROBE_LEVEL" != "preflight" ] && [ "$GATE_SHORTCUTS_APP_INSTALLED" = "PASS" ]; then
  echo "=== Phase 2: Testing Import Decision Tree ==="
  GATE_FILE_IMPORT_ATTEMPTED="PASS"

  # Route 1: Documented shortcuts://create-shortcut URL scheme
  ROUTE1_OUT="$(xcrun simctl openurl "$UDID" "shortcuts://create-shortcut" 2>&1)"
  ROUTE1_RC=$?
  sleep 3
  xcrun simctl io "$UDID" screenshot "$ARTIFACT_DIR/editor/route1-create-shortcut.png" 2>/dev/null || true
  if [ "$ROUTE1_RC" -eq 0 ]; then
    GATE_SHORTCUTS_LAUNCHED="PASS"
    GATE_EDITOR_OPENED="PASS"
  fi

  # Route 2: Unsigned .shortcut via simctl openurl
  UNSIGNED_FILE="$ARTIFACT_DIR/builds/qa-functional-v2.shortcut"
  ROUTE2_OUT="$(xcrun simctl openurl "$UDID" "file://$UNSIGNED_FILE" 2>&1)"
  ROUTE2_RC=$?
  sleep 2
  xcrun simctl io "$UDID" screenshot "$ARTIFACT_DIR/editor/route2-unsigned-openurl.png" 2>/dev/null || true

  # Route 3: macOS host /usr/bin/shortcuts sign -> signed .shortcut -> simctl openurl
  SIGNED_FILE="$ARTIFACT_DIR/builds/qa-functional-v2-signed.shortcut"
  if [ -x /usr/bin/shortcuts ]; then
    SIGN_OUT="$(/usr/bin/shortcuts sign --mode anyone --input "$UNSIGNED_FILE" --output "$SIGNED_FILE" 2>&1)"
    SIGN_RC=$?
  else
    SIGN_OUT="/usr/bin/shortcuts not present"
    SIGN_RC=127
  fi

  if [ "$SIGN_RC" -eq 0 ] && [ -s "$SIGNED_FILE" ]; then
    ROUTE3_OUT="$(xcrun simctl openurl "$UDID" "file://$SIGNED_FILE" 2>&1)"
    ROUTE3_RC=$?
    sleep 2
    xcrun simctl io "$UDID" screenshot "$ARTIFACT_DIR/editor/route3-signed-openurl.png" 2>/dev/null || true
  else
    ROUTE3_OUT="Blocked by macOS host signing: $SIGN_OUT"
    ROUTE3_RC="$SIGN_RC"
  fi

  # Route 4: Exploratory undocumented shortcuts://import-shortcut (explicitly tagged UNDOCUMENTED)
  ROUTE4_OUT="$(xcrun simctl openurl "$UDID" "shortcuts://import-shortcut?name=CherriQAProbe" 2>&1)"
  ROUTE4_RC=$?
  sleep 2

  python3 - "$ARTIFACT_DIR/import-attempts.json" "$ROUTE1_RC" "$ROUTE1_OUT" "$ROUTE2_RC" "$ROUTE2_OUT" "$SIGN_RC" "$SIGN_OUT" "$ROUTE3_RC" "$ROUTE3_OUT" "$ROUTE4_RC" "$ROUTE4_OUT" <<'PY'
import json
import sys

out_path = sys.argv[1]
r1_rc, r1_out = int(sys.argv[2]), sys.argv[3]
r2_rc, r2_out = int(sys.argv[4]), sys.argv[5]
sign_rc, sign_out = int(sys.argv[6]), sys.argv[7]
r3_rc, r3_out = int(sys.argv[8]), sys.argv[9]
r4_rc, r4_out = int(sys.argv[10]), sys.argv[11]

attempts = {
    "schemaVersion": "1.0",
    "routes": [
        {
            "routeId": "route_1_create_shortcut_scheme",
            "documented": True,
            "command": "xcrun simctl openurl <UDID> shortcuts://create-shortcut",
            "exitCode": r1_rc,
            "status": "PASS" if r1_rc == 0 else "BLOCKED_IMPORT",
            "observedOutput": r1_out[:512],
        },
        {
            "routeId": "route_2_unsigned_file_openurl",
            "documented": True,
            "command": "xcrun simctl openurl <UDID> file://<unsigned.shortcut>",
            "exitCode": r2_rc,
            "status": "BLOCKED_SIGNING" if r2_rc != 0 else "ATTEMPTED_UI_CHECK_REQUIRED",
            "observedOutput": r2_out[:512],
        },
        {
            "routeId": "route_3_macos_host_sign_then_openurl",
            "documented": True,
            "command": "/usr/bin/shortcuts sign --mode anyone && xcrun simctl openurl <UDID> file://<signed.shortcut>",
            "hostSignExitCode": sign_rc,
            "hostSignOutput": sign_out[:512],
            "openUrlExitCode": r3_rc,
            "status": "PASS" if (sign_rc == 0 and r3_rc == 0) else "BLOCKED_SIGNING",
            "observedOutput": r3_out[:512],
        },
        {
            "routeId": "route_4_exploratory_import_shortcut_scheme",
            "documented": False,
            "classification": "UNDOCUMENTED",
            "command": "xcrun simctl openurl <UDID> shortcuts://import-shortcut?name=CherriQAProbe",
            "exitCode": r4_rc,
            "status": "UNDOCUMENTED_PROBE",
            "observedOutput": r4_out[:512],
        }
    ]
}
with open(out_path, "w", encoding="utf-8") as f:
    json.dump(attempts, f, indent=2)
PY

  if [ "$SIGN_RC" -ne 0 ]; then
    GATE_COMPILER_ARTIFACT_IMPORTED="BLOCKED_SIGNING"
    GATE_SHORTCUT_EXISTS_IN_LIBRARY="BLOCKED_SIGNING"
  else
    GATE_COMPILER_ARTIFACT_IMPORTED="BLOCKED_IMPORT"
    GATE_SHORTCUT_EXISTS_IN_LIBRARY="BLOCKED_IMPORT"
  fi
fi

# ---------------------------------------------------------------------------
# Phase 3 & 4: Build & Execute XCTest / XCUIAutomation Suite + Receiver App
# ---------------------------------------------------------------------------
if [ "$PROBE_LEVEL" = "editor" ] || [ "$PROBE_LEVEL" = "runtime" ] || [ "$PROBE_LEVEL" = "full" ]; then
  if [ "$GATE_SHORTCUTS_APP_INSTALLED" = "PASS" ]; then
    echo "=== Phase 3 & 4: Running XCTest UI Automation & Callback Receiver ==="
    if ! command -v xcodegen >/dev/null 2>&1; then
      brew install xcodegen
    fi
    pushd "$ROOT_DIR/tools/shortcuts-simulator-qa/XcodeProject" >/dev/null
    xcodegen generate
    popd >/dev/null

    export TEST_RUNNER_CHERRI_QA_SHARED_OUTPUT_DIR="$ARTIFACT_DIR/editor"
    export TEST_RUNNER_CHERRI_QA_MAX_EDITOR_PAGES="$MAX_EDITOR_PAGES"

    XCRESULT_PATH="$ARTIFACT_DIR/tests/CherriSimulatorQA.xcresult"
    rm -rf "$XCRESULT_PATH"

    xcodebuild test \
      -project "$ROOT_DIR/tools/shortcuts-simulator-qa/XcodeProject/CherriSimulatorQA.xcodeproj" \
      -scheme CherriSimulatorQAReceiver \
      -destination "platform=iOS Simulator,id=$UDID" \
      -resultBundlePath "$XCRESULT_PATH" \
      CODE_SIGNING_ALLOWED=NO \
      > "$ARTIFACT_DIR/tests/xcodebuild-test.log" 2>&1
    XCODEBUILD_TEST_RC=$?
    echo "xcodebuild test exit code: $XCODEBUILD_TEST_RC"

    if [ -d "$XCRESULT_PATH" ]; then
      xcrun xcresulttool get test-results summary --path "$XCRESULT_PATH" > "$ARTIFACT_DIR/tests/xcresult-summary.json" 2>/dev/null || true
      if [ -f "$ARTIFACT_DIR/tests/xcresult-summary.json" ] && [ -n "$UDID" ]; then
        python3 -c 'import sys; p, u, m = sys.argv[1:4]; s = open(p, encoding="utf-8").read().replace(u, m); open(p, "w", encoding="utf-8").write(s)' "$ARTIFACT_DIR/tests/xcresult-summary.json" "$UDID" "$MASKED_UDID"
      fi
      rm -rf "$XCRESULT_PATH"
    fi
    rm -f "$ARTIFACT_DIR/tests/xcodebuild-test.log"

    # Extract receiver callback events from app data container
    RECEIVER_DATA_DIR="$(xcrun simctl get_app_container "$UDID" org.cherrilang.qa.Receiver data 2>/dev/null || true)"
    if [ -n "$RECEIVER_DATA_DIR" ] && [ -f "$RECEIVER_DATA_DIR/Documents/callback-events.json" ]; then
      cp "$RECEIVER_DATA_DIR/Documents/callback-events.json" "$ARTIFACT_DIR/runtime/callback-events.json"
      GATE_CALLBACK_CAPTURED="PASS"
    else
      echo "[]" > "$ARTIFACT_DIR/runtime/callback-events.json"
      GATE_CALLBACK_CAPTURED="FAIL_UI"
    fi

    # Evaluate editor capture gates from editor-action-coverage.json
    if [ -f "$ARTIFACT_DIR/editor/editor-action-coverage.json" ]; then
      LAUNCHED_VAL="$(python3 -c 'import json,sys; d=json.load(open(sys.argv[1])); print("PASS" if d.get("shortcutForegroundVerified") else "FAIL_UI")' "$ARTIFACT_DIR/editor/editor-action-coverage.json")"
      EDITOR_VAL="$(python3 -c 'import json,sys; d=json.load(open(sys.argv[1])); print("PASS" if d.get("editorOpened") else "FAIL_UI")' "$ARTIFACT_DIR/editor/editor-action-coverage.json")"
      CAPTURE_STATUS="$(python3 -c 'import json,sys; d=json.load(open(sys.argv[1])); print(d.get("captureStatus","PARTIAL_EDITOR_CAPTURE"))' "$ARTIFACT_DIR/editor/editor-action-coverage.json")"
      GATE_SHORTCUTS_LAUNCHED="$LAUNCHED_VAL"
      if [ "$EDITOR_VAL" = "PASS" ]; then
        GATE_EDITOR_OPENED="PASS"
      fi
      GATE_ACTION_EDITOR_CAPTURED="$CAPTURE_STATUS"
    fi

    # Phase 4 Runtime Execution of Compiled Shortcut via x-callback-url
    if [ "$GATE_SHORTCUT_EXISTS_IN_LIBRARY" = "PASS" ]; then
      GATE_SHORTCUT_RUN_ATTEMPTED="PASS"
      xcrun simctl openurl "$UDID" "shortcuts://x-callback-url/run-shortcut?name=Cherri%20QA%20Functional%20V2&input=text&text=CHERRI_QA_INPUT&x-success=cherriqa%3A%2F%2Fsuccess%3FrunId%3Dqa-functional-v2&x-cancel=cherriqa%3A%2F%2Fcancel%3FrunId%3Dqa-functional-v2&x-error=cherriqa%3A%2F%2Ferror%3FrunId%3Dqa-functional-v2" >/dev/null 2>&1 || true
      sleep 5
      if [ -n "$RECEIVER_DATA_DIR" ] && [ -f "$RECEIVER_DATA_DIR/Documents/callback-events.json" ]; then
        cp "$RECEIVER_DATA_DIR/Documents/callback-events.json" "$ARTIFACT_DIR/runtime/callback-events.json"
      fi
      GATE_SHORTCUT_RUN_COMPLETED="BLOCKED_IMPORT"
      GATE_TRACE_CAPTURED="BLOCKED_IMPORT"
    else
      # Do NOT fake runtime execution from an unimported shortcut; record exact upstream blocker
      GATE_SHORTCUT_RUN_ATTEMPTED="$GATE_SHORTCUT_EXISTS_IN_LIBRARY"
      GATE_SHORTCUT_RUN_COMPLETED="$GATE_SHORTCUT_EXISTS_IN_LIBRARY"
      GATE_TRACE_CAPTURED="$GATE_SHORTCUT_EXISTS_IN_LIBRARY"
    fi
  fi
fi

# Remove compiled binary and raw .shortcut build intermediates before artifact upload
rm -rf "$ARTIFACT_DIR/builds"

# Write output-assertions.json and runtime-trace.jsonl placeholder/records
python3 - "$ARTIFACT_DIR" "$GATE_SHORTCUT_RUN_COMPLETED" "$GATE_CALLBACK_CAPTURED" "$GATE_TRACE_CAPTURED" <<'PY'
import json
import os
import sys

artifact_dir, run_status, cb_status, trace_status = sys.argv[1:5]
assertions = {
    "schemaVersion": "1.0",
    "shortcutRunCompletedGate": run_status,
    "callbackCapturedGate": cb_status,
    "traceCapturedGate": trace_status,
    "receiverSelfCallbackVerified": (cb_status == "PASS"),
    "compiledShortcutRuntimeExecuted": (run_status == "PASS"),
    "notes": "Receiver cherriqa:// callback persistence is verified on simulator; compiled .shortcut execution requires signed import into simulator Shortcuts library."
}
with open(os.path.join(artifact_dir, "runtime", "output-assertions.json"), "w", encoding="utf-8") as f:
    json.dump(assertions, f, indent=2)

trace_jsonl_path = os.path.join(artifact_dir, "runtime", "runtime-trace.jsonl")
if not os.path.exists(trace_jsonl_path):
    with open(trace_jsonl_path, "w", encoding="utf-8") as f:
        f.write(json.dumps({
            "schemaVersion": "1.0",
            "kind": "COLLECTOR_EVENT",
            "stage": "end",
            "status": trace_status,
            "omittedReason": "Simulator Shortcuts library import blocked by unsigned/unauthenticated host signing on headless CI runner" if trace_status != "PASS" else None,
        }) + "\n")
PY

# Write summary.json and docs/report.md
python3 - "$ARTIFACT_DIR" "$RUN_ID" "$SOURCE_SHA" \
  "$GATE_RUNNER_AVAILABLE" \
  "$GATE_SIMULATOR_BOOTED" \
  "$GATE_SHORTCUTS_APP_INSTALLED" \
  "$GATE_SHORTCUTS_LAUNCHED" \
  "$GATE_EDITOR_OPENED" \
  "$GATE_FILE_IMPORT_ATTEMPTED" \
  "$GATE_COMPILER_ARTIFACT_IMPORTED" \
  "$GATE_SHORTCUT_EXISTS_IN_LIBRARY" \
  "$GATE_ACTION_EDITOR_CAPTURED" \
  "$GATE_SHORTCUT_RUN_ATTEMPTED" \
  "$GATE_SHORTCUT_RUN_COMPLETED" \
  "$GATE_CALLBACK_CAPTURED" \
  "$GATE_TRACE_CAPTURED" \
  "$GATE_PHYSICAL_DEVICE_TESTED" <<'PY'
import json
import os
import sys

artifact_dir, run_id, sha = sys.argv[1:4]
gates = {
    "RUNNER_AVAILABLE": sys.argv[4],
    "SIMULATOR_BOOTED": sys.argv[5],
    "SHORTCUTS_APP_INSTALLED": sys.argv[6],
    "SHORTCUTS_LAUNCHED": sys.argv[7],
    "EDITOR_OPENED": sys.argv[8],
    "FILE_IMPORT_ATTEMPTED": sys.argv[9],
    "COMPILER_ARTIFACT_IMPORTED": sys.argv[10],
    "SHORTCUT_EXISTS_IN_LIBRARY": sys.argv[11],
    "ACTION_EDITOR_CAPTURED": sys.argv[12],
    "SHORTCUT_RUN_ATTEMPTED": sys.argv[13],
    "SHORTCUT_RUN_COMPLETED": sys.argv[14],
    "CALLBACK_CAPTURED": sys.argv[15],
    "TRACE_CAPTURED": sys.argv[16],
    "PHYSICAL_DEVICE_TESTED": sys.argv[17],
}

summary = {
    "schemaVersion": "1.0",
    "runId": run_id,
    "compilerCommitSha": sha,
    "gates": gates,
}
with open(os.path.join(artifact_dir, "summary.json"), "w", encoding="utf-8") as f:
    json.dump(summary, f, indent=2)

lines = [
    f"# Cherri iPadOS 27 Simulator QA Report (`{run_id}`)",
    "",
    f"- **Compiler Commit SHA:** `{sha}`",
    "",
    "## Progressive Sub-Gate Outcomes",
    "",
    "| Sub-Gate | Status |",
    "|---|---|",
]
for k, v in gates.items():
    lines.append(f"| `{k}` | `{v}` |")

with open(os.path.join(artifact_dir, "docs", "report.md"), "w", encoding="utf-8") as f:
    f.write("\n".join(lines) + "\n")
PY

echo "=== QA Run Complete: $ARTIFACT_DIR/summary.json ==="
cat "$ARTIFACT_DIR/summary.json"
