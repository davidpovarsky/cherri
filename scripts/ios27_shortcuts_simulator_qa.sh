#!/usr/bin/env bash
# Cherri P10R14 — Signed iPadOS 27 Simulator Shortcuts QA Orchestrator
# Implements the 14-gate state machine, signed/unsigned import verification,
# genuine editor card & anchor coverage, correlated x-callback-url runtime
# oracle, NDJSON trace validation, and fail-closed privacy/CI gates.

set -uo pipefail

PROBE_LEVEL="${1:-full}"
DEVICE_PROFILE="${2:-ipad-default}"
SAMPLE_ID="${3:-all}"
CAPTURE_FULL_EDITOR="${4:-true}"
MAX_EDITOR_PAGES="${5:-20}"
REQUIRE_RUNTIME="${6:-false}"
RUN_ID="${GITHUB_RUN_ID:-local-$(date -u +%Y%m%dT%H%M%SZ)}"

# Validate workflow/CLI inputs upfront (fail closed on invalid input)
case "$PROBE_LEVEL" in
  preflight|import|editor|runtime|full) ;;
  *) echo "ERROR: Invalid PROBE_LEVEL '$PROBE_LEVEL'" >&2; exit 2 ;;
esac
case "$DEVICE_PROFILE" in
  ipad-default|iphone-fallback) ;;
  *) echo "ERROR: Invalid DEVICE_PROFILE '$DEVICE_PROFILE'" >&2; exit 2 ;;
esac
case "$SAMPLE_ID" in
  all|qa-functional|qa-scroll-editor|qa-rtl-hebrew) ;;
  *) echo "ERROR: Invalid SAMPLE_ID '$SAMPLE_ID'" >&2; exit 2 ;;
esac
case "$CAPTURE_FULL_EDITOR" in
  true|false) ;;
  *) echo "ERROR: Invalid CAPTURE_FULL_EDITOR '$CAPTURE_FULL_EDITOR'" >&2; exit 2 ;;
esac
case "$REQUIRE_RUNTIME" in
  true|false) ;;
  *) echo "ERROR: Invalid REQUIRE_RUNTIME '$REQUIRE_RUNTIME'" >&2; exit 2 ;;
esac
if ! [[ "$MAX_EDITOR_PAGES" =~ ^[0-9]+$ ]] || [ "$MAX_EDITOR_PAGES" -lt 1 ] || [ "$MAX_EDITOR_PAGES" -gt 20 ]; then
  echo "ERROR: MAX_EDITOR_PAGES must be an integer in 1..20 (got '$MAX_EDITOR_PAGES')" >&2
  exit 2
fi

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
echo "Require Runtime:    $REQUIRE_RUNTIME"
echo "Source SHA:         $SOURCE_SHA"

# 14 Progressive Gate Defaults
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

# Select primary target fixture from SAMPLE_ID
case "$SAMPLE_ID" in
  qa-scroll-editor)
    TARGET_FIXTURE_ID="qa-scroll-editor-v2"
    LEGACY_COUNTERPART_ID="qa-scroll-editor-legacy"
    EXPECTED_SHORTCUT_NAME="qa-scroll-editor-v2"
    EXPECTED_SEMANTIC_OUTPUT="ANCHOR_CARD_26_TEXT_ZETA"
    ;;
  qa-rtl-hebrew)
    TARGET_FIXTURE_ID="qa-rtl-hebrew-v2"
    LEGACY_COUNTERPART_ID="qa-functional-legacy"
    EXPECTED_SHORTCUT_NAME="qa-rtl-hebrew-v2"
    EXPECTED_SEMANTIC_OUTPUT="CHERRI_QA_RTL_RESULT:ok:hebrew"
    ;;
  *)
    TARGET_FIXTURE_ID="qa-functional-v2"
    LEGACY_COUNTERPART_ID="qa-functional-legacy"
    EXPECTED_SHORTCUT_NAME="qa-functional-v2"
    EXPECTED_SEMANTIC_OUTPUT="CHERRI_QA_RESULT:ok:count=2:mode=v2"
    ;;
esac
TARGET_SOURCE_FILE="$ROOT_DIR/tools/shortcuts-simulator-qa/fixtures/${TARGET_FIXTURE_ID}.cherri"
LEGACY_COUNTERPART_SOURCE="$ROOT_DIR/tools/shortcuts-simulator-qa/fixtures/${LEGACY_COUNTERPART_ID}.cherri"

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
    "schemaVersion": "1.4",
    "compilerCommitSha": source_sha,
    "fixtures": manifest_entries,
}
with open(os.path.join(artifact_dir, "compiler-manifest.json"), "w", encoding="utf-8") as f:
    json.dump(compiler_manifest, f, indent=2)

# Structural classifier matching debug_stdlib.go (never naive substring heuristics)
def is_structural_inline_trace_text(actions, idx):
    if idx < 0 or idx + 1 >= len(actions):
        return False
    act = actions[idx]
    nxt = actions[idx + 1]
    if act.get("WFWorkflowActionIdentifier") != "is.workflow.actions.gettext":
        return False
    if nxt.get("WFWorkflowActionIdentifier") != "is.workflow.actions.appendvariable":
        return False
    nxt_params = nxt.get("WFWorkflowActionParameters", {})
    if nxt_params.get("WFVariableName") != "__cherri_qa_trace":
        return False
    act_uuid = act.get("WFWorkflowActionParameters", {}).get("UUID", "")
    inp = nxt_params.get("WFInput", {}).get("Value", {})
    if not act_uuid or inp.get("OutputUUID") != act_uuid:
        return False
    txt = act.get("WFWorkflowActionParameters", {}).get("WFTextActionText", {})
    if not isinstance(txt, dict) or txt.get("WFSerializationType") != "WFTextTokenString":
        return False
    atts = txt.get("Value", {}).get("attachmentsByRange", {})
    if len(atts) != 1:
        return False
    only_att = next(iter(atts.values()))
    return isinstance(only_att, dict) and only_att.get("Type") == "CurrentDate"

def is_structural_collector_envelope_text(actions, idx):
    if idx <= 0 or idx >= len(actions):
        return False
    prev = actions[idx - 1]
    act = actions[idx]
    if prev.get("WFWorkflowActionIdentifier") != "is.workflow.actions.text.combine":
        return False
    if act.get("WFWorkflowActionIdentifier") != "is.workflow.actions.gettext":
        return False
    prev_params = prev.get("WFWorkflowActionParameters", {})
    combine_uuid = prev_params.get("UUID", "")
    combine_var = prev_params.get("text", {}).get("Value", {}).get("VariableName", "")
    if not combine_uuid or combine_var != "__cherri_qa_trace":
        return False
    txt = act.get("WFWorkflowActionParameters", {}).get("WFTextActionText", {})
    if not isinstance(txt, dict) or txt.get("WFSerializationType") != "WFTextTokenString":
        return False
    s = txt.get("Value", {}).get("string", "")
    if "===CHERRI_QA_TRACE_NDJSON_V14===" not in s:
        return False
    atts = txt.get("Value", {}).get("attachmentsByRange", {})
    if len(atts) != 1:
        return False
    only_att = next(iter(atts.values()))
    return (
        isinstance(only_att, dict)
        and only_att.get("Type") == "ActionOutput"
        and only_att.get("OutputUUID") == combine_uuid
        and only_att.get("OutputName") == "Combined Text"
    )

def classify_actions(actions):
    source_ids = []
    inline_debug_ids = []
    collector_ids = []
    for idx, act in enumerate(actions):
        aid = act.get("WFWorkflowActionIdentifier", "")
        params = act.get("WFWorkflowActionParameters", {})
        vname = str(params.get("WFVariableName", ""))
        if vname in ("__cherri_debug_log", "__cherri_qa_trace") or vname.startswith("__cherri_debug_timer_"):
            inline_debug_ids.append({"nativeIndex": idx, "identifier": aid, "variable": vname})
            continue
        if aid == "is.workflow.actions.detect.date":
            aggs = params.get("WFInput", {}).get("Value", {}).get("Aggrandizements", [])
            if len(aggs) == 1 and aggs[0].get("PropertyUserInfo") == "WFFileCreationDate":
                inline_debug_ids.append({"nativeIndex": idx, "identifier": aid, "reason": "CreationDateCapture"})
                continue
        if aid == "is.workflow.actions.text.combine":
            cvar = params.get("text", {}).get("Value", {}).get("VariableName", "")
            if cvar in ("__cherri_debug_log", "__cherri_qa_trace"):
                collector_ids.append({"nativeIndex": idx, "identifier": aid})
                continue
        if aid == "is.workflow.actions.gettext":
            if is_structural_inline_trace_text(actions, idx):
                inline_debug_ids.append({"nativeIndex": idx, "identifier": aid, "reason": "TraceEventLine"})
                continue
            if is_structural_collector_envelope_text(actions, idx):
                collector_ids.append({"nativeIndex": idx, "identifier": aid, "reason": "TraceExportEnvelope"})
                continue
            if params.get("WFTextActionText") == "" and idx + 1 < len(actions):
                nxt = actions[idx + 1]
                if (
                    nxt.get("WFWorkflowActionIdentifier") == "is.workflow.actions.setvariable"
                    and nxt.get("WFWorkflowActionParameters", {}).get("WFVariableName") == "__cherri_debug_log"
                ):
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
    if inst_src != uninst_src:
        raise RuntimeError(f"SOURCE_ACTION projection mismatch for {dialect}: {inst_src} != {uninst_src}")
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
    json.dump({"schemaVersion": "1.4", "comparisons": comparisons}, f, indent=2)
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
prefer_ipad = (profile != "iphone-fallback")

def runtime_rank(r):
    rid = r.get("identifier", "")
    devs = devices_map.get(rid, [])
    has_avail_ipad = any("iPad" in d.get("name", "") and d.get("isAvailable", True) for d in devs)
    not_iphone_specific = ("iPhone" not in rid)
    return (
        str(r.get("version", "")),
        1 if (has_avail_ipad if prefer_ipad else not has_avail_ipad) else 0,
        1 if (not_iphone_specific if prefer_ipad else True) else 0,
    )

ios_runtimes.sort(key=runtime_rank, reverse=True)

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

simctl_openurl_resilient() {
  local target_udid="$1"
  local target_url="$2"
  local max_attempts="${3:-2}"
  local fallback_file="${4:-}"
  local attempt=1
  local out=""
  local rc=1
  while [ "$attempt" -le "$max_attempts" ]; do
    out="$(python3 - "$target_udid" "$target_url" <<'PY'
import subprocess, sys
udid, url = sys.argv[1], sys.argv[2]
try:
    p = subprocess.run(["xcrun", "simctl", "openurl", udid, url], stdin=subprocess.DEVNULL, capture_output=True, text=True, timeout=45)
    sys.stdout.write((p.stdout or "") + (p.stderr or ""))
    sys.exit(p.returncode)
except subprocess.TimeoutExpired as exc:
    out = (exc.stdout or b"").decode("utf-8", errors="replace") if isinstance(exc.stdout, bytes) else (exc.stdout or "")
    err = (exc.stderr or b"").decode("utf-8", errors="replace") if isinstance(exc.stderr, bytes) else (exc.stderr or "")
    sys.stdout.write(f"simctl openurl timed out after 45s {out} {err}".strip())
    sys.exit(124)
PY
)"
    rc=$?
    if [ "$rc" -eq 0 ]; then
      printf '%s' "$out"
      return 0
    fi
    if echo "$out" | grep -Eqi "Connection interrupted|server died|NSMachErrorDomain|CoreSimulatorService|timed out"; then
      python3 - "$target_udid" <<'PY' >/dev/null 2>&1 || true
import subprocess, sys
u = sys.argv[1]
for cmd in (
    ["open", "-g", "-a", "Simulator", "--args", "-CurrentDeviceUDID", u],
    ["xcrun", "simctl", "boot", u],
    ["xcrun", "simctl", "launch", u, "com.apple.shortcuts"],
):
    try:
        subprocess.run(cmd, stdin=subprocess.DEVNULL, capture_output=True, timeout=12)
    except Exception:
        pass
PY
    fi
    sleep 1
    attempt=$((attempt + 1))
  done
  if [ -n "$fallback_file" ] && [ -f "$fallback_file" ]; then
    python3 - "$fallback_file" <<'PY' >/dev/null 2>&1 || true
import subprocess, sys
try:
    subprocess.run(["open", "-a", "Simulator", sys.argv[1]], stdin=subprocess.DEVNULL, capture_output=True, timeout=12)
except Exception:
    pass
PY
  fi
  printf '%s' "$out"
  return "$rc"
}

if [ "$GATE_SHORTCUTS_APP_INSTALLED" = "PASS" ]; then
  python3 - "$UDID" <<'PY' >/dev/null 2>&1 || true
import subprocess, sys, time
u = sys.argv[1]
try:
    subprocess.run(["open", "-g", "-a", "Simulator", "--args", "-CurrentDeviceUDID", u], stdin=subprocess.DEVNULL, capture_output=True, timeout=15)
except Exception:
    pass
time.sleep(2)
for cmd in (
    ["xcrun", "simctl", "launch", u, "com.apple.shortcuts"],
):
    try:
        subprocess.run(cmd, stdin=subprocess.DEVNULL, capture_output=True, timeout=30)
    except Exception:
        pass
time.sleep(3)
try:
    subprocess.run(["xcrun", "simctl", "terminate", u, "com.apple.shortcuts"], stdin=subprocess.DEVNULL, capture_output=True, timeout=20)
except Exception:
    pass
PY
fi

MASKED_UDID="$(echo "$UDID" | cut -c1-8)-REDACTED"
python3 - "$ARTIFACT_DIR/environment.json" "$UNAME_OUT" "$SW_VERS_OUT" "$XCODE_SELECT_OUT" "$XCODEBUILD_VER_OUT" "$MASKED_UDID" "$DEVICE_NAME" "$RUNTIME_NAME" "$RUNTIME_VERSION" "$SHORTCUTS_BUNDLE_PRESENT" "$SOURCE_SHA" <<'PY'
import json
import sys

out_path, uname_s, sw_vers_s, xcode_sel, xcode_ver, masked_udid, dev_name, rt_name, rt_ver, app_present, sha = sys.argv[1:12]
env_doc = {
    "schemaVersion": "1.4",
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
# Phase 2: Import Decision Tree (HubSign Exact-File Signing + 4 Import Routes)
# ---------------------------------------------------------------------------
SIGNED_ARTIFACT_VALID_AEA1="false"
SIGNING_METHOD="NONE"
SIGNED_SHA256=""
SIGNED_BYTE_SIZE="0"
LEGACY_SIGNING_VERIFIED="false"

if [ "$PROBE_LEVEL" != "preflight" ] && [ "$GATE_SHORTCUTS_APP_INSTALLED" = "PASS" ]; then
  echo "=== Phase 2: Testing Import Decision Tree & HubSign Signing ==="
  GATE_FILE_IMPORT_ATTEMPTED="PASS"

  CHERRI_BIN="$ARTIFACT_DIR/builds/cherri"
  mkdir -p "$ARTIFACT_DIR/builds/signed"
  UNSIGNED_FILE="$ARTIFACT_DIR/builds/${TARGET_FIXTURE_ID}.shortcut"
  SIGNED_FILE="$ARTIFACT_DIR/builds/signed/${EXPECTED_SHORTCUT_NAME}.shortcut"
  SIGNED_CANDIDATE="$ARTIFACT_DIR/builds/${TARGET_FIXTURE_ID}-signed.shortcut"
  PRE_SIGNED_FIXTURE="$ROOT_DIR/tools/shortcuts-simulator-qa/fixtures/signed/${TARGET_FIXTURE_ID}.shortcut"
  LEGACY_UNSIGNED_FILE="$ARTIFACT_DIR/builds/${LEGACY_COUNTERPART_ID}.shortcut"
  LEGACY_SIGNED_CANDIDATE="$ARTIFACT_DIR/builds/${LEGACY_COUNTERPART_ID}-signed.shortcut"
  if [ "$UNSIGNED_FILE" = "$SIGNED_FILE" ]; then
    echo "ERROR: UNSIGNED_FILE and SIGNED_FILE must never alias ($UNSIGNED_FILE)" >&2
    exit 1
  fi
  if [ ! -s "$UNSIGNED_FILE" ]; then
    echo "ERROR: Compiled unsigned plist missing at $UNSIGNED_FILE" >&2
    exit 1
  fi

  ROUTE1_RC=-1
  ROUTE1_OUT=""
  ROUTE2_RC=-1
  ROUTE2_OUT=""
  ROUTE4_RC=-1
  ROUTE4_OUT=""

  run_negative_and_scheme_probes() {
    # Route 2: Unsigned .shortcut via simctl openurl (verify rejection of unsigned file)
    ROUTE2_OUT="$(simctl_openurl_resilient "$UDID" "file://$UNSIGNED_FILE" 2)"
    ROUTE2_RC=$?
    sleep 2
    xcrun simctl io "$UDID" screenshot "$ARTIFACT_DIR/editor/route2-unsigned-openurl.png" 2>/dev/null || true
    xcrun simctl terminate "$UDID" com.apple.shortcuts >/dev/null 2>&1 || true

    # Route 4: Exploratory undocumented shortcuts://import-shortcut (explicitly tagged UNDOCUMENTED)
    ROUTE4_OUT="$(simctl_openurl_resilient "$UDID" "shortcuts://import-shortcut?name=CherriQAProbe" 2)"
    ROUTE4_RC=$?
    sleep 1
    xcrun simctl terminate "$UDID" com.apple.shortcuts >/dev/null 2>&1 || true

    # Route 1: Documented shortcuts://create-shortcut URL scheme
    ROUTE1_OUT="$(simctl_openurl_resilient "$UDID" "shortcuts://create-shortcut" 2)"
    ROUTE1_RC=$?
    sleep 2
    xcrun simctl io "$UDID" screenshot "$ARTIFACT_DIR/editor/route1-create-shortcut.png" 2>/dev/null || true
    if [ "$ROUTE1_RC" -eq 0 ]; then
      if [ "$GATE_SHORTCUTS_LAUNCHED" = "NOT_RUN" ]; then
        GATE_SHORTCUTS_LAUNCHED="PASS"
      fi
      if [ "$GATE_EDITOR_OPENED" = "NOT_RUN" ]; then
        GATE_EDITOR_OPENED="PASS"
      fi
    fi
    xcrun simctl terminate "$UDID" com.apple.shortcuts >/dev/null 2>&1 || true
  }

  # Route 3: Sign EXACT preflight-validated unsigned plist via Cherri HubSign (or CHERRI_SIGNING_SERVER if configured)
  rm -f "$SIGNED_CANDIDATE" "$SIGNED_FILE"
  SIGN_ARGS=(--hubsign)
  SIGN_CMD_LABEL="--hubsign"
  if [ -n "${CHERRI_SIGNING_SERVER:-}" ]; then
    SIGN_ARGS=(--signing-server="$CHERRI_SIGNING_SERVER")
    SIGN_CMD_LABEL="--signing-server=<redacted>"
  fi
  SIGN_OUT="$("$CHERRI_BIN" sign "$UNSIGNED_FILE" \
    --output="$SIGNED_CANDIDATE" \
    "${SIGN_ARGS[@]}" \
    --source-file="$TARGET_SOURCE_FILE" \
    --compiler-sha="$SOURCE_SHA" \
    --manifest-out="$ARTIFACT_DIR/builds/signing-provenance-target.json" \
    --no-ansi 2>&1)"
  SIGN_RC=$?

  if [ "$SIGN_RC" -eq 0 ] && [ -s "$SIGNED_CANDIDATE" ]; then
    HEADER_MAGIC="$(head -c 4 "$SIGNED_CANDIDATE" 2>/dev/null || true)"
    BYTE_COUNT="$(wc -c < "$SIGNED_CANDIDATE" | tr -d '[:space:]')"
    if [ "$HEADER_MAGIC" = "AEA1" ] && [ "${BYTE_COUNT:-0}" -ge 16 ]; then
      cp -f "$SIGNED_CANDIDATE" "$SIGNED_FILE"
      SIGNED_ARTIFACT_VALID_AEA1="true"
      SIGNING_METHOD="HUBSIGN_REMOTE_SERVICE"
      if [ -n "${CHERRI_SIGNING_SERVER:-}" ] && [[ "$CHERRI_SIGNING_SERVER" != *"shortcuty.app"* ]] && [[ "$CHERRI_SIGNING_SERVER" != *"hubsign.routinehub.services"* ]]; then
        SIGNING_METHOD="CUSTOM_REMOTE_SIGNING_SERVICE"
      fi
      SIGNED_SHA256="$(shasum -a 256 "$SIGNED_FILE" | awk '{print $1}')"
      SIGNED_BYTE_SIZE="$BYTE_COUNT"
    else
      rm -f "$SIGNED_CANDIDATE" "$SIGNED_FILE"
      SIGN_RC=1
      SIGN_OUT="HubSign returned invalid artifact header=$HEADER_MAGIC bytes=${BYTE_COUNT:-0}"
    fi
  fi

  # Also verify Legacy counterpart compiles and HubSigns cleanly using exact unsigned plist bytes
  rm -f "$LEGACY_SIGNED_CANDIDATE"
  LEGACY_SIGN_OUT=""
  LEGACY_SIGN_RC=1
  if [ -s "$LEGACY_UNSIGNED_FILE" ]; then
    LEGACY_SIGN_OUT="$("$CHERRI_BIN" sign "$LEGACY_UNSIGNED_FILE" \
      --output="$LEGACY_SIGNED_CANDIDATE" \
      "${SIGN_ARGS[@]}" \
      --source-file="$LEGACY_COUNTERPART_SOURCE" \
      --compiler-sha="$SOURCE_SHA" \
      --manifest-out="$ARTIFACT_DIR/builds/signing-provenance-legacy.json" \
      --no-ansi 2>&1)"
    LEGACY_SIGN_RC=$?
    if [ "$LEGACY_SIGN_RC" -eq 0 ] && [ -s "$LEGACY_SIGNED_CANDIDATE" ]; then
      LEG_MAGIC="$(head -c 4 "$LEGACY_SIGNED_CANDIDATE" 2>/dev/null || true)"
      LEG_SIZE="$(wc -c < "$LEGACY_SIGNED_CANDIDATE" | tr -d '[:space:]')"
      if [ "$LEG_MAGIC" = "AEA1" ] && [ "${LEG_SIZE:-0}" -ge 16 ]; then
        LEGACY_SIGNING_VERIFIED="true"
      else
        rm -f "$LEGACY_SIGNED_CANDIDATE"
        LEGACY_SIGN_RC=1
      fi
    fi
  fi

  # Diagnostic fallback ONLY if remote HubSign was unreachable or failed: record host CLI / pre-signed fixture behavior honestly
  if [ "$SIGNED_ARTIFACT_VALID_AEA1" != "true" ]; then
    if [ -x /usr/bin/shortcuts ]; then
      HOST_CLI_OUT="$(/usr/bin/shortcuts sign --mode anyone --input "$UNSIGNED_FILE" --output "$SIGNED_CANDIDATE" 2>&1)"
      HOST_CLI_RC=$?
      if [ "$HOST_CLI_RC" -eq 0 ] && [ -s "$SIGNED_CANDIDATE" ] && [ "$(head -c 4 "$SIGNED_CANDIDATE" 2>/dev/null || true)" = "AEA1" ]; then
        cp -f "$SIGNED_CANDIDATE" "$SIGNED_FILE"
        SIGNED_ARTIFACT_VALID_AEA1="true"
        SIGNING_METHOD="MACOS_HOST_SHORTCUTS_CLI"
        SIGNED_SHA256="$(shasum -a 256 "$SIGNED_FILE" | awk '{print $1}')"
        SIGNED_BYTE_SIZE="$(wc -c < "$SIGNED_FILE" | tr -d '[:space:]')"
      else
        SIGN_OUT="${SIGN_OUT} | macOS host CLI fallback failed: ${HOST_CLI_OUT}"
      fi
    fi
    if [ "$SIGNED_ARTIFACT_VALID_AEA1" != "true" ] && [ -s "$PRE_SIGNED_FIXTURE" ] && [ "$(head -c 4 "$PRE_SIGNED_FIXTURE" 2>/dev/null || true)" = "AEA1" ]; then
      cp -f "$PRE_SIGNED_FIXTURE" "$SIGNED_FILE"
      SIGNED_ARTIFACT_VALID_AEA1="true"
      SIGNING_METHOD="OWNER_PRE_SIGNED_SYNTHETIC_FIXTURE"
      SIGNED_SHA256="$(shasum -a 256 "$SIGNED_FILE" | awk '{print $1}')"
      SIGNED_BYTE_SIZE="$(wc -c < "$SIGNED_FILE" | tr -d '[:space:]')"
    fi
  fi

  # Write sanitized signing provenance summary (zero internal URLs, zero credentials, exact SHA-256 chain)
  python3 - "$ARTIFACT_DIR/runtime/signing-provenance.json" \
    "$ARTIFACT_DIR/builds/signing-provenance-target.json" \
    "$ARTIFACT_DIR/builds/signing-provenance-legacy.json" \
    "$SIGNED_ARTIFACT_VALID_AEA1" "$SIGNING_METHOD" "$SIGNED_SHA256" "$SIGNED_BYTE_SIZE" \
    "$LEGACY_SIGNING_VERIFIED" "$SOURCE_SHA" <<'PY'
import json, os, sys

out_path, tgt_p, leg_p, valid_s, method, sha, sz, leg_ok, commit_sha = sys.argv[1:10]
def load_opt(p):
    if os.path.exists(p):
        try:
            return json.load(open(p, encoding="utf-8"))
        except Exception:
            return None
    return None

doc = {
    "schemaVersion": "1.4",
    "compilerCommitSha": commit_sha,
    "signedArtifactValidAEA1": (valid_s == "true"),
    "signingMethod": method,
    "signedArtifactSha256": sha if sha else None,
    "signedByteSize": int(sz) if sz.isdigit() else 0,
    "legacyCounterpartSignedAEA1": (leg_ok == "true"),
    "targetProvenance": load_opt(tgt_p),
    "legacyProvenance": load_opt(leg_p),
}
with open(out_path, "w", encoding="utf-8") as f:
    json.dump(doc, f, indent=2)
PY

  if [ -n "${CHERRI_SIGNING_SERVER:-}" ]; then
    SIGN_OUT="${SIGN_OUT//$CHERRI_SIGNING_SERVER/<redacted-signing-server>}"
  fi
  ROUTE3_RC="$SIGN_RC"
  if [ "$SIGNED_ARTIFACT_VALID_AEA1" = "true" ]; then
    ROUTE3_OUT="AEA1 signed artifact verified ($SIGNED_BYTE_SIZE bytes, method=$SIGNING_METHOD, legacyCounterpart=$LEGACY_SIGNING_VERIFIED); deferred simctl openurl to coordinated XCUITest handshake."
  else
    ROUTE3_OUT="Blocked signing: $SIGN_OUT"
  fi

  write_import_attempts_json() {
    python3 - "$ARTIFACT_DIR/import-attempts.json" "$ROUTE1_RC" "$ROUTE1_OUT" "$ROUTE2_RC" "$ROUTE2_OUT" "$SIGN_RC" "$SIGN_OUT" "$ROUTE3_RC" "$ROUTE3_OUT" "$ROUTE4_RC" "$ROUTE4_OUT" "$SIGNED_ARTIFACT_VALID_AEA1" "$SIGNING_METHOD" "$SIGNED_SHA256" "$SIGNED_BYTE_SIZE" "$LEGACY_SIGNING_VERIFIED" "$SIGN_CMD_LABEL" <<'PY'
import json
import sys

out_path = sys.argv[1]
r1_rc, r1_out = int(sys.argv[2]), sys.argv[3]
r2_rc, r2_out = int(sys.argv[4]), sys.argv[5]
sign_rc, sign_out = int(sys.argv[6]), sys.argv[7]
r3_rc, r3_out = int(sys.argv[8]), sys.argv[9]
r4_rc, r4_out = int(sys.argv[10]), sys.argv[11]
aea1_valid, sign_method, signed_sha, signed_bytes, legacy_signed = (
    (sys.argv[12] == "true"),
    sys.argv[13],
    sys.argv[14],
    int(sys.argv[15]) if sys.argv[15].isdigit() else 0,
    (sys.argv[16] == "true"),
)
sign_cmd_label = sys.argv[17] if len(sys.argv) > 17 else "--hubsign"

attempts = {
    "schemaVersion": "1.4",
    "signedArtifactValidAEA1": aea1_valid,
    "signingMethod": sign_method,
    "signedArtifactSha256": signed_sha if signed_sha else None,
    "signedByteSize": signed_bytes,
    "legacyCounterpartSignedAEA1": legacy_signed,
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
            "status": "UNSIGNED_REJECTED_EXPECTED_UI_MODAL" if r2_rc == 0 else "BLOCKED_SIGNING",
            "observedOutput": r2_out[:512],
        },
        {
            "routeId": "route_3_signed_shortcut_openurl",
            "documented": True,
            "command": f"cherri sign <unsigned.shortcut> {sign_cmd_label} && coordinated xcrun simctl openurl <UDID> file://<signed.shortcut>",
            "hostSignExitCode": sign_rc,
            "hostSignOutput": sign_out[:512],
            "openUrlExitCode": r3_rc,
            "status": "SIGNED_ARTIFACT_READY_FOR_COORDINATED_IMPORT" if aea1_valid else "BLOCKED_SIGNING",
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
  }

  if [ "$PROBE_LEVEL" = "import" ]; then
    run_negative_and_scheme_probes
    write_import_attempts_json
  fi

  if [ "$SIGNED_ARTIFACT_VALID_AEA1" != "true" ]; then
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

    EXEC_RUN_NONCE="${TARGET_FIXTURE_ID}-${RUN_ID}"
    RUN_URL="$(python3 - "$EXPECTED_SHORTCUT_NAME" "$EXEC_RUN_NONCE" "$TARGET_FIXTURE_ID" <<'PY'
import sys, urllib.parse
name, run_nonce, fid = sys.argv[1:4]
q = urllib.parse.quote
x_success = f"cherriqa://success?runId={q(run_nonce)}&fixtureId={q(fid)}&shortcutName={q(name)}&source=shortcuts-x-callback"
x_cancel = f"cherriqa://cancel?runId={q(run_nonce)}&fixtureId={q(fid)}&shortcutName={q(name)}&source=shortcuts-x-callback"
x_error = f"cherriqa://error?runId={q(run_nonce)}&fixtureId={q(fid)}&shortcutName={q(name)}&source=shortcuts-x-callback"
print(f"shortcuts://x-callback-url/run-shortcut?name={q(name)}&input=text&text=CHERRI_QA_INPUT&x-source=CherriSimulatorQAReceiver&x-success={q(x_success, safe='')}&x-cancel={q(x_cancel, safe='')}&x-error={q(x_error, safe='')}")
PY
)"
    RUN_URL_MINIMAL="$(python3 - "$EXPECTED_SHORTCUT_NAME" "$EXEC_RUN_NONCE" "$TARGET_FIXTURE_ID" <<'PY'
import sys, urllib.parse
name, run_nonce, fid = sys.argv[1:4]
q = urllib.parse.quote
x_success = f"cherriqa://success?runId={q(run_nonce)}&fixtureId={q(fid)}&shortcutName={q(name)}&source=shortcuts-x-callback"
x_cancel = f"cherriqa://cancel?runId={q(run_nonce)}&fixtureId={q(fid)}&shortcutName={q(name)}&source=shortcuts-x-callback"
x_error = f"cherriqa://error?runId={q(run_nonce)}&fixtureId={q(fid)}&shortcutName={q(name)}&source=shortcuts-x-callback"
print(f"shortcuts://x-callback-url/run-shortcut?name={q(name)}&x-source=CherriSimulatorQAReceiver&x-success={q(x_success, safe='')}&x-cancel={q(x_cancel, safe='')}&x-error={q(x_error, safe='')}")
PY
)"
    RUN_URL_PLAIN="$(python3 - "$EXPECTED_SHORTCUT_NAME" <<'PY'
import sys, urllib.parse
print(f"shortcuts://run-shortcut?name={urllib.parse.quote(sys.argv[1])}")
PY
)"

    export TEST_RUNNER_CHERRI_QA_SHARED_OUTPUT_DIR="$ARTIFACT_DIR/editor"
    export TEST_RUNNER_CHERRI_QA_MAX_EDITOR_PAGES="$MAX_EDITOR_PAGES"
    export TEST_RUNNER_CHERRI_QA_EXPECTED_SHORTCUT_NAME="$EXPECTED_SHORTCUT_NAME"
    export TEST_RUNNER_CHERRI_QA_EXPECT_SIGNED_IMPORT="$SIGNED_ARTIFACT_VALID_AEA1"
    export TEST_RUNNER_CHERRI_QA_EXPECT_RUNTIME_RUN="$SIGNED_ARTIFACT_VALID_AEA1"
    export TEST_RUNNER_CHERRI_QA_EXEC_RUN_NONCE="$EXEC_RUN_NONCE"
    export TEST_RUNNER_CHERRI_QA_RUN_URL="$RUN_URL"
    export TEST_RUNNER_CHERRI_QA_RUN_URL_MINIMAL="$RUN_URL_MINIMAL"
    export TEST_RUNNER_CHERRI_QA_RUN_URL_PLAIN="$RUN_URL_PLAIN"

    stop_bg_pid() {
      local pid="${1:-}"
      if [ -n "$pid" ]; then
        kill "$pid" 2>/dev/null || true
        sleep 0.5
        kill -9 "$pid" 2>/dev/null || true
      fi
    }
    run_cmd_timeout() {
      local timeout_sec="$1"
      shift
      python3 - "$timeout_sec" "$@" <<'PY'
import subprocess, sys
t = int(sys.argv[1])
cmd = sys.argv[2:]
try:
    p = subprocess.run(cmd, timeout=t)
    sys.exit(p.returncode)
except subprocess.TimeoutExpired:
    sys.stderr.write(f"command timed out after {t}s: {' '.join(cmd[:4])}\n")
    sys.exit(124)
PY
    }

    DERIVED_DATA_PATH="$ARTIFACT_DIR/builds/DerivedData"
    XCRESULT_PATH="$ARTIFACT_DIR/tests/CherriSimulatorQA.xcresult"
    rm -rf "$XCRESULT_PATH" "$DERIVED_DATA_PATH"

    mkdir -p "$ARTIFACT_DIR/builds"
    run_cmd_timeout 600 xcodebuild build-for-testing \
      -project "$ROOT_DIR/tools/shortcuts-simulator-qa/XcodeProject/CherriSimulatorQA.xcodeproj" \
      -scheme CherriSimulatorQAReceiver \
      -derivedDataPath "$DERIVED_DATA_PATH" \
      -destination "platform=iOS Simulator,id=$UDID" \
      CODE_SIGNING_ALLOWED=NO \
      > "$ARTIFACT_DIR/builds/xcodebuild-build.log" 2>&1
    XCODEBUILD_BUILD_RC=$?
    echo "xcodebuild build-for-testing exit code: $XCODEBUILD_BUILD_RC"
    if [ "$XCODEBUILD_BUILD_RC" -ne 0 ] && [ -f "$ARTIFACT_DIR/builds/xcodebuild-build.log" ]; then
      sed "s/$UDID/<UDID>/g" "$ARTIFACT_DIR/builds/xcodebuild-build.log" | tail -n 30 || true
    fi

    RAW_SIM_LOG="$ARTIFACT_DIR/runtime/raw-sim-log.txt"
    rm -f "$RAW_SIM_LOG"
    xcrun simctl spawn "$UDID" log stream --predicate '(process == "Shortcuts" || process == "BackgroundShortcutRunner" || process == "siriactionsd" || process == "SpringBoard" || subsystem == "com.apple.shortcuts" || subsystem == "com.apple.WorkflowKit" || process == "CherriSimulatorQAReceiver") AND NOT (subsystem == "com.apple.dt.xctest") AND NOT (subsystem == "com.apple.xpc.transaction")' --level info --style compact > "$RAW_SIM_LOG" 2>&1 &
    SIM_LOG_PID=$!

    rm -f "$ARTIFACT_DIR/editor/request-"* "$ARTIFACT_DIR/editor/ack-"* "$ARTIFACT_DIR/editor/handshake-stop.txt"
    (
      import_dispatched=0
      import_retried=0
      run_dispatched=0
      for _tick in $(seq 1 4500); do
        if [ "$import_dispatched" -eq 0 ] && [ -f "$ARTIFACT_DIR/editor/request-signed-import.txt" ]; then
          import_dispatched=1
          if [ "$SIGNED_ARTIFACT_VALID_AEA1" = "true" ] && [ -n "${SIGNED_FILE:-}" ] && [ -s "${SIGNED_FILE:-}" ]; then
            _t0=$SECONDS
            IMP_RC=0
            IMP_OUT="$(simctl_openurl_resilient "$UDID" "file://$SIGNED_FILE" 2 "$SIGNED_FILE")" || IMP_RC=$?
            _dt=$((SECONDS - _t0))
            printf 'rc=%s\ndurationSec=%s %s\n' "$IMP_RC" "$_dt" "${IMP_OUT:0:220}" > "$ARTIFACT_DIR/editor/ack-signed-import.txt"
          else
            printf 'rc=1\nNO_VALID_AEA1\n' > "$ARTIFACT_DIR/editor/ack-signed-import.txt"
          fi
        fi
        if [ "$import_retried" -eq 0 ] && [ -f "$ARTIFACT_DIR/editor/request-signed-import-retry.txt" ]; then
          import_retried=1
          if [ "$SIGNED_ARTIFACT_VALID_AEA1" = "true" ] && [ -n "${SIGNED_FILE:-}" ] && [ -s "${SIGNED_FILE:-}" ]; then
            _t0=$SECONDS
            IMP2_RC=0
            IMP2_OUT="$(simctl_openurl_resilient "$UDID" "file://$SIGNED_FILE" 2 "$SIGNED_FILE")" || IMP2_RC=$?
            _dt=$((SECONDS - _t0))
            printf 'rc=%s\ndurationSec=%s %s\n' "$IMP2_RC" "$_dt" "${IMP2_OUT:0:220}" > "$ARTIFACT_DIR/editor/ack-signed-import-retry.txt"
          fi
        fi
        if [ "$run_dispatched" -eq 0 ] && [ -f "$ARTIFACT_DIR/editor/request-shortcut-run.txt" ]; then
          run_dispatched=1
          _t0=$SECONDS
          RUN_RC=0
          RUN_OUT="$(simctl_openurl_resilient "$UDID" "$RUN_URL" 2)" || RUN_RC=$?
          _dt=$((SECONDS - _t0))
          printf 'rc=%s\ndurationSec=%s %s\n' "$RUN_RC" "$_dt" "${RUN_OUT:0:220}" > "$ARTIFACT_DIR/editor/ack-shortcut-run.txt"
        fi
        [ -f "$ARTIFACT_DIR/editor/handshake-stop.txt" ] && break
        sleep 0.2
      done
    ) &
    HANDSHAKE_PID=$!
    cleanup_handshake() {
      if [ -n "${HANDSHAKE_PID:-}" ]; then
        touch "$ARTIFACT_DIR/editor/handshake-stop.txt" 2>/dev/null || true
        stop_bg_pid "$HANDSHAKE_PID"
        HANDSHAKE_PID=""
      fi
      if [ -n "${SIM_LOG_PID:-}" ]; then
        stop_bg_pid "$SIM_LOG_PID"
        SIM_LOG_PID=""
      fi
      if [ -n "${PHASE4_LOG_PID:-}" ]; then
        stop_bg_pid "$PHASE4_LOG_PID"
        PHASE4_LOG_PID=""
      fi
    }
    trap 'cleanup_handshake; rm -rf "${XCRESULT_PATH:-}"' EXIT

    run_cmd_timeout 780 xcodebuild test-without-building \
      -project "$ROOT_DIR/tools/shortcuts-simulator-qa/XcodeProject/CherriSimulatorQA.xcodeproj" \
      -scheme CherriSimulatorQAReceiver \
      -derivedDataPath "$DERIVED_DATA_PATH" \
      -destination "platform=iOS Simulator,id=$UDID" \
      -resultBundlePath "$XCRESULT_PATH" \
      CODE_SIGNING_ALLOWED=NO \
      > "$ARTIFACT_DIR/builds/xcodebuild-test.log" 2>&1
    XCODEBUILD_TEST_RC=$?
    echo "xcodebuild test exit code: $XCODEBUILD_TEST_RC"
    if [ "$XCODEBUILD_TEST_RC" -ne 0 ] && [ -f "$ARTIFACT_DIR/builds/xcodebuild-test.log" ]; then
      grep -Ev "DVTAssertions|nativeArchitecture|Apple Watch|SimDevice :|Object:[[:space:]]+<DVTiPhoneSimulator|Method:[[:space:]]+-nativeArchitecture|Thread:[[:space:]]+<_NSMainThread|feedbackassistant\.apple\.com|^[[:space:]]*\)[[:space:]]*$" "$ARTIFACT_DIR/builds/xcodebuild-test.log" | sed "s/$UDID/<UDID>/g; s|/Users/[^/ ]*|/Users/<redacted>|g" | tail -n 35 || true
    fi

    cleanup_handshake
    trap - EXIT

    DID_HANDSHAKE_RUN="false"
    if [ -f "$ARTIFACT_DIR/editor/ack-shortcut-run.txt" ]; then
      DID_HANDSHAKE_RUN="true"
    fi
    python3 - "$ARTIFACT_DIR/editor" "$UDID" "$ROOT_DIR" <<'PY'
import json, os, re, sys
ed, udid, root_dir = sys.argv[1:4]
def read_ack(name):
    p = os.path.join(ed, name)
    if not os.path.exists(p):
        return None
    raw = open(p, encoding="utf-8", errors="replace").read()
    if udid:
        raw = raw.replace(udid, "<UDID>")
    if root_dir:
        raw = raw.replace(root_dir, "<REPO_ROOT>")
    raw = re.sub(r"/Users/[^/\s]+", "/Users/<redacted>", raw)
    lines = raw.splitlines()
    rc = None
    if lines and lines[0].startswith("rc="):
        try:
            rc = int(lines[0][3:].strip())
        except Exception:
            rc = None
        msg = "\n".join(lines[1:]).strip()
    else:
        msg = raw.strip()
    return {"rc": rc, "message": msg[:256]}

doc = {
    "schemaVersion": "1.4",
    "importRequested": os.path.exists(os.path.join(ed, "request-signed-import.txt")),
    "importAck": read_ack("ack-signed-import.txt"),
    "importRetried": os.path.exists(os.path.join(ed, "request-signed-import-retry.txt")),
    "importRetryAck": read_ack("ack-signed-import-retry.txt"),
    "runRequested": os.path.exists(os.path.join(ed, "request-shortcut-run.txt")),
    "runAck": read_ack("ack-shortcut-run.txt"),
}
with open(os.path.join(ed, "handshake-status.json"), "w", encoding="utf-8") as f:
    json.dump(doc, f, indent=2)
PY
    rm -f "$ARTIFACT_DIR/editor/request-"* "$ARTIFACT_DIR/editor/ack-"* "$ARTIFACT_DIR/editor/handshake-stop.txt"

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

    # Evaluate import & editor capture gates from editor-action-coverage.json
    if [ -f "$ARTIFACT_DIR/editor/editor-action-coverage.json" ]; then
      LAUNCHED_VAL="$(python3 -c 'import json,sys; d=json.load(open(sys.argv[1])); print("PASS" if d.get("shortcutForegroundVerified") else "FAIL_UI")' "$ARTIFACT_DIR/editor/editor-action-coverage.json")"
      EDITOR_VAL="$(python3 -c 'import json,sys; d=json.load(open(sys.argv[1])); print("PASS" if d.get("editorOpened") else "FAIL_UI")' "$ARTIFACT_DIR/editor/editor-action-coverage.json")"
      LIB_EXISTS_VAL="$(python3 -c 'import json,sys; d=json.load(open(sys.argv[1])); print("true" if d.get("shortcutExistsInLibrary") else "false")' "$ARTIFACT_DIR/editor/editor-action-coverage.json")"
      CAPTURE_STATUS="$(python3 -c 'import json,sys; d=json.load(open(sys.argv[1])); print(d.get("captureStatus","PARTIAL_EDITOR_CAPTURE"))' "$ARTIFACT_DIR/editor/editor-action-coverage.json")"

      GATE_SHORTCUTS_LAUNCHED="$LAUNCHED_VAL"
      GATE_EDITOR_OPENED="$EDITOR_VAL"

      if [ "$SIGNED_ARTIFACT_VALID_AEA1" = "true" ] && [ "$LIB_EXISTS_VAL" = "true" ]; then
        GATE_COMPILER_ARTIFACT_IMPORTED="PASS"
        GATE_SHORTCUT_EXISTS_IN_LIBRARY="PASS"
      fi

      # Enforce strict prerequisite: EDITOR_CAPTURE_COMPLETE requires SHORTCUT_EXISTS_IN_LIBRARY == PASS
      if [ "$CAPTURE_STATUS" = "EDITOR_CAPTURE_COMPLETE" ] && [ "$GATE_SHORTCUT_EXISTS_IN_LIBRARY" = "PASS" ]; then
        GATE_ACTION_EDITOR_CAPTURED="EDITOR_CAPTURE_COMPLETE"
      elif [ "$GATE_EDITOR_OPENED" = "PASS" ]; then
        GATE_ACTION_EDITOR_CAPTURED="PARTIAL_EDITOR_CAPTURE"
      else
        GATE_ACTION_EDITOR_CAPTURED="FAIL_UI"
      fi
    fi

    # Generate comparison of compiled plist actions/parameters against captured Shortcuts Editor cards
    if [ -n "${UNSIGNED_FILE:-}" ] && [ -s "${UNSIGNED_FILE:-}" ]; then
      python3 - "$UNSIGNED_FILE" "$ARTIFACT_DIR/editor/editor-action-coverage.json" "$ARTIFACT_DIR/editor/editor-tree.json" "$ARTIFACT_DIR/editor/plist-vs-editor-comparison.json" "$TARGET_FIXTURE_ID" <<'PY'
import json, os, plistlib, sys

plist_path, cov_path, tree_path, out_path, fid = sys.argv[1:6]
doc = plistlib.loads(open(plist_path, "rb").read())
actions = doc.get("WFWorkflowActions", [])
cov = json.load(open(cov_path, encoding="utf-8")) if os.path.exists(cov_path) else {}
tree = json.load(open(tree_path, encoding="utf-8")) if os.path.exists(tree_path) else []

visible_texts = set()
for card in cov.get("detectedActionCards", []):
    visible_texts.add(card)
for a in cov.get("detectedAnchors", []):
    visible_texts.add(a)
for n in tree:
    if n.get("visible"):
        for k in ("label", "value"):
            v = n.get(k)
            if v:
                visible_texts.add(v)
joined_ui = "\n".join(visible_texts)

plist_items = []
for idx, act in enumerate(actions):
    aid = act.get("WFWorkflowActionIdentifier", "")
    params = act.get("WFWorkflowActionParameters", {})
    marker = None
    if aid == "is.workflow.actions.comment":
        marker = params.get("WFCommentActionText", "")
    elif aid == "is.workflow.actions.gettext":
        txt = params.get("WFTextActionText", "")
        marker = txt.get("Value", {}).get("string", "") if isinstance(txt, dict) else str(txt)
    elif aid == "is.workflow.actions.number":
        marker = str(params.get("WFNumberActionNumber", ""))
    expected_ui_titles = {
        "is.workflow.actions.comment": "Comment",
        "is.workflow.actions.gettext": "Text",
        "is.workflow.actions.number": "Number",
        "is.workflow.actions.dictionary": "Dictionary",
        "is.workflow.actions.getvalueforkey": "Get Dictionary Value",
        "is.workflow.actions.conditional": "If",
        "is.workflow.actions.repeat.count": "Repeat",
        "is.workflow.actions.output": "Stop and Output",
    }
    title_hint = expected_ui_titles.get(aid, aid.split(".")[-1])
    matched_title = (title_hint.lower() in joined_ui.lower()) or ("Get Value" in joined_ui and aid == "is.workflow.actions.getvalueforkey")
    matched_marker = bool(marker and isinstance(marker, str) and len(marker) >= 6 and (marker.splitlines()[0][:24] in joined_ui))
    plist_items.append({
        "nativeIndex": idx,
        "actionIdentifier": aid,
        "expectedCardHint": title_hint,
        "markerPreview": str(marker)[:80] if marker else None,
        "matchedCardTitleInUI": matched_title,
        "matchedParameterInUI": matched_marker,
    })

summary = {
    "schemaVersion": "1.4",
    "fixtureId": fid,
    "compiledNativeActionCount": len(actions),
    "detectedAnchorsCount": len(cov.get("detectedAnchors", [])),
    "detectedAnchors": cov.get("detectedAnchors", []),
    "detectedActionCardsCount": len(cov.get("detectedActionCards", [])),
    "matchedCardTitlesCount": sum(1 for item in plist_items if item["matchedCardTitleInUI"]),
    "matchedParameterValuesCount": sum(1 for item in plist_items if item["matchedParameterInUI"]),
    "actions": plist_items,
}
with open(out_path, "w", encoding="utf-8") as f:
    json.dump(summary, f, indent=2)
PY
    fi

    # Phase 4: Correlated Runtime Execution of Imported Shortcut via x-callback-url
    PHASE4_SIM_LOG="$ARTIFACT_DIR/runtime/phase4-sim-log.txt"
    rm -f "$PHASE4_SIM_LOG"
    if [ "$GATE_SHORTCUT_EXISTS_IN_LIBRARY" = "PASS" ]; then
      GATE_SHORTCUT_RUN_ATTEMPTED="PASS"
      FOUND_NONCE="no"
      if [ -n "$RECEIVER_DATA_DIR" ] && [ -f "$RECEIVER_DATA_DIR/Documents/callback-events.json" ]; then
        cp "$RECEIVER_DATA_DIR/Documents/callback-events.json" "$ARTIFACT_DIR/runtime/callback-events.json"
        FOUND_NONCE="$(python3 -c 'import json,sys; evs=json.load(open(sys.argv[1])); print("yes" if any((not e.get("receiverSelfTest")) and e.get("runId")==sys.argv[2] for e in evs) else "no")' "$ARTIFACT_DIR/runtime/callback-events.json" "$EXEC_RUN_NONCE" 2>/dev/null || echo "no")"
      fi

      if [ "$FOUND_NONCE" != "yes" ]; then
        echo "=== Phase 4b: Post-XCTest ReceiverApp x-callback-url dispatch ($TARGET_FIXTURE_ID) ==="
        # Outside UITests.xctrunner (automationmode=0), launch ReceiverApp so UIApplication.shared.open passes sourceApplication appIdentifier
        xcrun simctl spawn "$UDID" log stream --predicate '(process == "Shortcuts" || process == "BackgroundShortcutRunner" || subsystem == "com.apple.shortcuts" || subsystem == "com.apple.WorkflowKit" || process == "CherriSimulatorQAReceiver") AND NOT (subsystem == "com.apple.dt.xctest") AND NOT (subsystem == "com.apple.xpc.transaction") AND NOT (subsystem == "com.apple.Accessibility") AND NOT (subsystem == "com.apple.defaults")' --level info --style compact > "$PHASE4_SIM_LOG" 2>&1 &
        PHASE4_LOG_PID=$!
        sleep 1
        TRIGGER_WRAPPER_URL="$(python3 - "$RUN_URL" <<'PY'
import sys, urllib.parse
print("cherriqa://trigger?target=" + urllib.parse.quote(sys.argv[1], safe=""))
PY
)"
        run_cmd_timeout 15 xcrun simctl terminate "$UDID" com.apple.shortcuts >/dev/null 2>&1 || true
        run_cmd_timeout 15 xcrun simctl terminate "$UDID" org.cherrilang.qa.Receiver >/dev/null 2>&1 || true
        sleep 1
        RECEIVER_BUNDLE_PATH="$(xcrun simctl get_app_container "$UDID" org.cherrilang.qa.Receiver app 2>/dev/null || true)"
        if [ -n "$RECEIVER_BUNDLE_PATH" ] && [ -d "$RECEIVER_BUNDLE_PATH" ]; then
          SIGNED_RECEIVER_TMP="/tmp/cherri-receiver-signed-${RUN_ID}.app"
          rm -rf "$SIGNED_RECEIVER_TMP"
          cp -R "$RECEIVER_BUNDLE_PATH" "$SIGNED_RECEIVER_TMP"
          CS_OUT="$(codesign --force --sign - --identifier org.cherrilang.qa.Receiver "$SIGNED_RECEIVER_TMP" 2>&1 || true)"
          INST_OUT="$(xcrun simctl install "$UDID" "$SIGNED_RECEIVER_TMP" 2>&1 || true)"
          rm -rf "$SIGNED_RECEIVER_TMP"
          RECEIVER_BUNDLE_PATH="$(xcrun simctl get_app_container "$UDID" org.cherrilang.qa.Receiver app 2>/dev/null || true)"
          NEW_DATA_DIR="$(xcrun simctl get_app_container "$UDID" org.cherrilang.qa.Receiver data 2>/dev/null || true)"
          [ -n "$NEW_DATA_DIR" ] && RECEIVER_DATA_DIR="$NEW_DATA_DIR"
          echo "=== ReceiverApp codesign + install ==="
          [ -n "$CS_OUT" ] && printf '%s\n' "$CS_OUT" | sed "s/$UDID/<UDID>/g; s|/Users/[^/ ]*|/Users/<redacted>|g" | head -n 5 || true
          [ -n "$INST_OUT" ] && printf '%s\n' "$INST_OUT" | sed "s/$UDID/<UDID>/g; s|/Users/[^/ ]*|/Users/<redacted>|g" | head -n 5 || true
          codesign -dvv "$RECEIVER_BUNDLE_PATH" 2>&1 | sed "s/$UDID/<UDID>/g; s|/Users/[^/ ]*|/Users/<redacted>|g" | head -n 12 || true
        fi
        LAUNCH_RC=0
        LAUNCH_OUT="$( ( export SIMCTL_CHILD_CHERRI_TRIGGER_SHORTCUT_URL="$RUN_URL"; run_cmd_timeout 25 xcrun simctl launch "$UDID" org.cherrilang.qa.Receiver ) 2>&1 )" || LAUNCH_RC=$?
        echo "Phase4b simctl launch rc=$LAUNCH_RC out=$(printf '%s' "$LAUNCH_OUT" | sed "s/$UDID/<UDID>/g" | tr '\n' ' ' | cut -c1-200)"
        FALLBACK_TRIGGER_SENT="no"
        for poll_idx in $(seq 1 16); do
          if [ -n "$RECEIVER_DATA_DIR" ] && [ -f "$RECEIVER_DATA_DIR/Documents/callback-events.json" ]; then
            cp "$RECEIVER_DATA_DIR/Documents/callback-events.json" "$ARTIFACT_DIR/runtime/callback-events.json"
            FOUND_NONCE="$(python3 -c 'import json,sys; evs=json.load(open(sys.argv[1])); print("yes" if any((not e.get("receiverSelfTest")) and e.get("runId")==sys.argv[2] for e in evs) else "no")' "$ARTIFACT_DIR/runtime/callback-events.json" "$EXEC_RUN_NONCE" 2>/dev/null || echo "no")"
            if [ "$FOUND_NONCE" = "yes" ]; then
              run_cmd_timeout 15 xcrun simctl io "$UDID" screenshot "$ARTIFACT_DIR/editor/runtime-callback.png" >/dev/null 2>&1 || true
              break
            fi
          fi
          if [ "$poll_idx" -eq 5 ] && [ "$FALLBACK_TRIGGER_SENT" = "no" ]; then
            OPEN_OK="no"
            if [ -n "$RECEIVER_DATA_DIR" ] && [ -f "$RECEIVER_DATA_DIR/Documents/trigger-debug.txt" ] && grep -q "openCompletion=true" "$RECEIVER_DATA_DIR/Documents/trigger-debug.txt"; then
              OPEN_OK="yes"
            fi
            if [ "$OPEN_OK" != "yes" ]; then
              FALLBACK_TRIGGER_SENT="yes"
              TRIG_RC=0
              TRIG_OUT="$(run_cmd_timeout 15 xcrun simctl openurl "$UDID" "$TRIGGER_WRAPPER_URL" 2>&1)" || TRIG_RC=$?
              echo "Phase4b fallback cherriqa://trigger openurl rc=$TRIG_RC out=$(printf '%s' "$TRIG_OUT" | sed "s/$UDID/<UDID>/g" | tr '\n' ' ' | cut -c1-200)"
            fi
          fi
          sleep 1
        done

        if [ "$FOUND_NONCE" != "yes" ]; then
          run_cmd_timeout 15 xcrun simctl io "$UDID" screenshot "$ARTIFACT_DIR/editor/phase4-after-receiver-openurl.png" >/dev/null 2>&1 || true
          echo "=== Phase 4b-confirm: Checking for Shortcuts open/allow consent dialog via test-without-building ==="
          CONFIRM_XCRESULT="$ARTIFACT_DIR/tests/CherriSimulatorQAConfirm.xcresult"
          rm -rf "$CONFIRM_XCRESULT"
          echo "1" > "$ARTIFACT_DIR/editor/confirm-dialog-mode.txt"
          TEST_RUNNER_CHERRI_QA_DIALOG_CONFIRM_ONLY=1 run_cmd_timeout 120 xcodebuild test-without-building \
            -project "$ROOT_DIR/tools/shortcuts-simulator-qa/XcodeProject/CherriSimulatorQA.xcodeproj" \
            -scheme CherriSimulatorQAReceiver \
            -derivedDataPath "$DERIVED_DATA_PATH" \
            -destination "platform=iOS Simulator,id=$UDID" \
            -only-testing:CherriSimulatorQAUITests/CherriSimulatorQAUITests/test04ConfirmPendingRuntimeDialog \
            -resultBundlePath "$CONFIRM_XCRESULT" \
            CODE_SIGNING_ALLOWED=NO \
            > "$ARTIFACT_DIR/builds/xcodebuild-confirm.log" 2>&1 || true
          if [ -f "$ARTIFACT_DIR/builds/xcodebuild-confirm.log" ]; then
            sed "s/$UDID/<UDID>/g; s|/Users/[^/ ]*|/Users/<redacted>|g" "$ARTIFACT_DIR/builds/xcodebuild-confirm.log" | tail -n 60 || true
          fi
          rm -rf "$CONFIRM_XCRESULT" "$ARTIFACT_DIR/editor/confirm-dialog-mode.txt"
          for poll_idx in $(seq 1 10); do
            if [ -n "$RECEIVER_DATA_DIR" ] && [ -f "$RECEIVER_DATA_DIR/Documents/callback-events.json" ]; then
              cp "$RECEIVER_DATA_DIR/Documents/callback-events.json" "$ARTIFACT_DIR/runtime/callback-events.json"
              FOUND_NONCE="$(python3 -c 'import json,sys; evs=json.load(open(sys.argv[1])); print("yes" if any((not e.get("receiverSelfTest")) and e.get("runId")==sys.argv[2] for e in evs) else "no")' "$ARTIFACT_DIR/runtime/callback-events.json" "$EXEC_RUN_NONCE" 2>/dev/null || echo "no")"
              if [ "$FOUND_NONCE" = "yes" ]; then
                if [ ! -f "$ARTIFACT_DIR/editor/runtime-callback.png" ]; then
                  run_cmd_timeout 15 xcrun simctl io "$UDID" screenshot "$ARTIFACT_DIR/editor/runtime-callback.png" >/dev/null 2>&1 || true
                fi
                break
              fi
            fi
            sleep 1
          done
        fi

        if [ -n "${PHASE4_LOG_PID:-}" ]; then
          stop_bg_pid "$PHASE4_LOG_PID"
          PHASE4_LOG_PID=""
        fi
        if [ -n "$RECEIVER_DATA_DIR" ] && [ -f "$RECEIVER_DATA_DIR/Documents/trigger-debug.txt" ]; then
          sed "s/$UDID/<UDID>/g" "$RECEIVER_DATA_DIR/Documents/trigger-debug.txt" > "$ARTIFACT_DIR/runtime/receiver-trigger-debug.txt" || true
          echo "=== ReceiverApp trigger-debug.txt ==="
          cat "$ARTIFACT_DIR/runtime/receiver-trigger-debug.txt" || true
        fi
        echo "=== Phase 4b finished (FOUND_NONCE=$FOUND_NONCE) ==="
      fi

      # Evaluate callback & NDJSON trace using strict oracle rules (never hardcode BLOCKED_IMPORT!)
      RUNTIME_EVAL="$(python3 - "$ARTIFACT_DIR/runtime/callback-events.json" "$EXEC_RUN_NONCE" "$TARGET_FIXTURE_ID" "$EXPECTED_SEMANTIC_OUTPUT" "$ARTIFACT_DIR/runtime/runtime-trace.jsonl" <<'PY'
import json, sys

cb_path, expected_nonce, expected_trace_run_id, expected_out, trace_out_path = sys.argv[1:6]
try:
    events = json.load(open(cb_path, encoding="utf-8"))
except Exception as exc:
    print(json.dumps({"runGate": "FAIL_RUNTIME", "traceGate": "FAIL_RUNTIME", "reason": f"failed to read callback-events.json: {exc}"}))
    sys.exit(0)

matched = [e for e in events if not e.get("receiverSelfTest") and e.get("runId") == expected_nonce]
if len(matched) == 0:
    print(json.dumps({"runGate": "FAIL_RUNTIME", "traceGate": "FAIL_RUNTIME", "reason": f"no non-self-test callback for runId {expected_nonce}"}))
    sys.exit(0)
if len(matched) > 1:
    print(json.dumps({"runGate": "FAIL_RUNTIME", "traceGate": "FAIL_RUNTIME", "reason": f"duplicate callback replay ({len(matched)}) for runId {expected_nonce}"}))
    sys.exit(0)

ev = matched[0]
if ev.get("status") != "SUCCESS":
    print(json.dumps({"runGate": "FAIL_RUNTIME", "traceGate": "FAIL_RUNTIME", "reason": f"callback status={ev.get('status')} error={ev.get('errorMessage')}"}))
    sys.exit(0)

summary_text = ev.get("resultSummary") or ""
trace_text = ev.get("tracePayload") or ""
combined = (summary_text + "\n" + trace_text).strip()
if expected_out and expected_out not in combined:
    print(json.dumps({"runGate": "FAIL_RUNTIME", "traceGate": "FAIL_RUNTIME", "reason": f"expected output {expected_out!r} not in {summary_text!r}"}))
    sys.exit(0)

# Validate NDJSON trace lines
raw_lines = [l.strip() for l in (trace_text or summary_text).splitlines() if l.strip() and l.strip() != "===CHERRI_QA_TRACE_NDJSON_V14==="]
valid_records = []
has_begin = False
has_end = False
inline_count = 0
trace_err = None

for idx, line in enumerate(raw_lines):
    if not (line.startswith("{") and line.endswith("}")):
        continue
    try:
        rec = json.loads(line)
    except Exception as exc:
        trace_err = f"line {idx+1} invalid JSON: {exc}"
        break
    kind = rec.get("kind")
    stage = rec.get("stage")
    if kind == "COLLECTOR_EVENT":
        if rec.get("runId") and rec.get("runId") != expected_trace_run_id:
            trace_err = f"collector runId mismatch: {rec.get('runId')} != {expected_trace_run_id}"
            break
        if stage == "collector_begin":
            has_begin = True
        elif stage == "collector_end":
            has_end = True
        valid_records.append(rec)
    elif kind == "INLINE_DEBUG_ACTION":
        if stage not in ("before", "after", "error", "end") or not rec.get("timestamp") or rec.get("stepSequence", -1) < 0:
            trace_err = f"invalid INLINE_DEBUG_ACTION record at line {idx+1}"
            break
        inline_count += 1
        valid_records.append(rec)

if not trace_err and (not has_begin or not has_end or inline_count == 0):
    trace_err = f"incomplete trace stream (begin={has_begin}, end={has_end}, inlineCount={inline_count})"

if trace_err:
    print(json.dumps({"runGate": "PASS", "traceGate": "FAIL_RUNTIME", "reason": trace_err}))
else:
    with open(trace_out_path, "w", encoding="utf-8") as tf:
        for r in valid_records:
            tf.write(json.dumps(r, ensure_ascii=False) + "\n")
    print(json.dumps({"runGate": "PASS", "traceGate": "PASS", "reason": "verified correlated callback output and NDJSON trace"}))
PY
)"
      GATE_SHORTCUT_RUN_COMPLETED="$(echo "$RUNTIME_EVAL" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("runGate","FAIL_RUNTIME"))')"
      GATE_TRACE_CAPTURED="$(echo "$RUNTIME_EVAL" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("traceGate","FAIL_RUNTIME"))')"
      RUNTIME_EVAL_REASON="$(echo "$RUNTIME_EVAL" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("reason",""))')"
    else
      # Do NOT fake runtime execution from an unimported shortcut; propagate exact upstream blocker
      GATE_SHORTCUT_RUN_ATTEMPTED="$GATE_SHORTCUT_EXISTS_IN_LIBRARY"
      GATE_SHORTCUT_RUN_COMPLETED="$GATE_SHORTCUT_EXISTS_IN_LIBRARY"
      GATE_TRACE_CAPTURED="$GATE_SHORTCUT_EXISTS_IN_LIBRARY"
      RUNTIME_EVAL_REASON="Simulator Shortcuts library import blocked ($GATE_SHORTCUT_EXISTS_IN_LIBRARY); runtime execution skipped fail-closed."
    fi

    if [ "${GATE_SHORTCUT_RUN_COMPLETED:-}" != "PASS" ] && { [ -f "${PHASE4_SIM_LOG:-}" ] || [ -f "${RAW_SIM_LOG:-}" ]; }; then
      LOG_SRC="${PHASE4_SIM_LOG:-}"
      if [ ! -s "$LOG_SRC" ]; then
        LOG_SRC="${RAW_SIM_LOG:-}"
      fi
      python3 - "$UDID" "$ROOT_DIR" "$LOG_SRC" "$ARTIFACT_DIR/runtime/shortcuts-log-excerpt.txt" <<'PY' || true
import re, sys
udid, root_dir, in_path, out_path = sys.argv[1:5]
lines = open(in_path, encoding="utf-8", errors="replace").read().splitlines()
forbidden = re.compile(r"(?i)(cherri-shortcut-audit-20261110|\b[0-3][0-9]\.plist\b|BEGIN [A-Z ]*PRIVATE KEY|ghp_[A-Za-z0-9_]+|github_pat_[A-Za-z0-9_]+|sk-[A-Za-z0-9]{16,}|Bearer\s+[A-Za-z0-9._\-]+|AppleID|iCloudToken)")
noise = (
    "ToolKitIndexing", "ToolKitDatabase", "approvedForPublicDrawerToDisplayActionIdentifier",
    "ToolDatabaseWriter", "CFBundle:strings", "CFBundle:resources",
    "com.apple.Accessibility", "com.apple.defaults:User Defaults",
    "com.apple.AppleMediaServices", "com.apple.clouddocs", "com.apple.accounts",
    "com.apple.foundation.filecoordination", "com.apple.launchservices:binding",
    "com.apple.cloudkit.sim", "BackBoardServices", "FrontBoardServices:Workspace",
    "com.apple.dt.xctest", "com.apple.xpc.transaction", "XC_kAXXCAttribute",
    "com.apple.runningboard:monitor", "com.apple.siri:Client",
    "com.apple.launchservices:datasep", "application record search init",
    "com.apple.appintents:Connection", "com.apple.shortcuts:AppIntentsMetadata",
    "Realizing settings extension", "SceneExtension", "FBSScene", "UIScene",
    "UIKeyboard", "RBSAssertion", "com.apple.runningboard", "CoreAnalytics",
    "container_create_or_lookup_app_group_path",
    "com.apple.UIKit", "com.apple.BackBoard", "com.apple.FrontBoard",
    "com.apple.GenerativeModels", "com.apple.BacklightServices",
    "com.apple.BoardServices", "libMobileGestalt.dylib", "com.apple.xpc:connection",
    "com.apple.xpc:session", "com.apple.KeyboardArbiter", "com.apple.TextInputUI",
    "com.apple.useractivity", "com.apple.containermanager", "com.apple.network:",
    "com.apple.CloudKit", "com.apple.cloudkit", "com.apple.launchservices:datasep",
    "com.apple.launchservices:privacy", "Error retrieving application record",
    "ToolEmbeddingDatabase", "RunningBoardServices", "CoreSpotlight", "SpotlightSync",
    "com.apple.ManagedConfiguration", "com.apple.purplebuddy", "PrototypeTools",
    "com.apple.calls.callkit", "com.apple.UIIntelligenceSupport",
    "com.apple.foundation.filecoordination", "Updating Key-Value Observers",
    "Loading Preferences From User Session CFPrefsD", "CoreAnalytics",
    "com.apple.CoreAnalytics", "com.apple.shortcuts:Files",
    "com.apple.PlugInKit", "com.apple.launchservices:db", "com.apple.shortcuts:ActionRegistry"
)
uuid_re = re.compile(r"[0-9A-Fa-f]{8}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{12}")
users_re = re.compile(r"/Users/[^/\s]+")
filtered = []
exec_lines = []
exec_keywords = (
    "CherriReceiver", "BackgroundShortcutRunner", "WFOutputAction",
    "WorkflowExecution", "RunningLifecycle", "XCallback", "x-callback",
    "taint", "appIdentifier", "destination app", "could not be determined",
    "cherriqa", "VoiceShortcutClient", "WFXCallback", "openURL",
    "OpenURL", "WFLibraryRunCoordinator", "Shortcuts:URL", "WFWorkflow"
)
for ln in lines:
    if udid:
        ln = ln.replace(udid, "<UDID>")
    if root_dir:
        ln = ln.replace(root_dir, "<REPO_ROOT>")
    ln = users_re.sub("/Users/<redacted>", ln)
    ln = uuid_re.sub("<UUID>", ln)
    if forbidden.search(ln):
        continue
    if "CherriReceiver" not in ln and any(n in ln for n in noise):
        continue
    filtered.append(ln[:320])
    if any(k in ln for k in exec_keywords) and "ToolEmbedding" not in ln:
        exec_lines.append(ln[:320])
with open(out_path, "w", encoding="utf-8") as f:
    kept = filtered[:120] + ["--- [truncated middle] ---"] + filtered[-330:] if len(filtered) > 450 else filtered
    f.write("\n".join(kept) + ("\n" if kept else ""))
exec_path = out_path.replace("shortcuts-log-excerpt.txt", "phase4-execution-log.txt")
with open(exec_path, "w", encoding="utf-8") as ef:
    ef.write("\n".join(exec_lines[-300:]) + ("\n" if exec_lines else ""))
print(f"scrubbed Phase4/sim log excerpt ({in_path}): {len(filtered)} lines -> kept {len(kept)}, execLines={len(exec_lines)}")
for ln in exec_lines[:100]:
    print("PHASE4_EXEC:", ln)
preview = filtered[:60] + (["---"] + filtered[-40:] if len(filtered) > 60 else [])
for ln in preview:
    print("PHASE4LOG:", ln)
PY
    fi
    rm -f "${RAW_SIM_LOG:-}" "${PHASE4_SIM_LOG:-}"

    if declare -F run_negative_and_scheme_probes >/dev/null 2>&1; then
      run_negative_and_scheme_probes
      write_import_attempts_json
    fi
  fi
fi

# Remove compiled binary, raw sim log, and raw .shortcut build intermediates before artifact upload
rm -rf "$ARTIFACT_DIR/builds" "$ARTIFACT_DIR/tests/"*.log "${RAW_SIM_LOG:-}" "${PHASE4_SIM_LOG:-}"

# Write output-assertions.json and runtime-trace.jsonl
python3 - "$ARTIFACT_DIR" "$GATE_SHORTCUT_RUN_COMPLETED" "$GATE_CALLBACK_CAPTURED" "$GATE_TRACE_CAPTURED" "${RUNTIME_EVAL_REASON:-}" "$TARGET_FIXTURE_ID" "$EXPECTED_SEMANTIC_OUTPUT" <<'PY'
import json
import os
import sys

artifact_dir, run_status, cb_status, trace_status, reason, fid, expected_out = sys.argv[1:8]
cb_path = os.path.join(artifact_dir, "runtime", "callback-events.json")
events = []
if os.path.exists(cb_path):
    try:
        events = json.load(open(cb_path, encoding="utf-8"))
    except Exception:
        events = []

self_cb_ok = any(e.get("receiverSelfTest") and e.get("status") == "SUCCESS" for e in events)
shortcut_cb_ok = any((not e.get("receiverSelfTest")) and e.get("status") == "SUCCESS" for e in events)

assertions = {
    "schemaVersion": "1.4",
    "targetFixtureId": fid,
    "expectedSemanticOutput": expected_out,
    "shortcutRunCompletedGate": run_status,
    "callbackCapturedGate": cb_status,
    "traceCapturedGate": trace_status,
    "receiverSelfCallbackVerified": self_cb_ok,
    "compiledShortcutCallbackVerified": shortcut_cb_ok,
    "compiledShortcutRuntimeExecuted": (run_status == "PASS"),
    "diagnosticReason": reason,
    "notes": "Receiver cherriqa:// self-test callback is tracked separately from compiled Shortcut x-callback-url completion; SHORTCUT_RUN_COMPLETED=PASS requires non-self-test correlated runId + expected output."
}
with open(os.path.join(artifact_dir, "runtime", "output-assertions.json"), "w", encoding="utf-8") as f:
    json.dump(assertions, f, indent=2)

trace_jsonl_path = os.path.join(artifact_dir, "runtime", "runtime-trace.jsonl")
if not os.path.exists(trace_jsonl_path):
    with open(trace_jsonl_path, "w", encoding="utf-8") as f:
        f.write(json.dumps({
            "schemaVersion": "1.4",
            "kind": "COLLECTOR_EVENT",
            "stage": "collector_end",
            "runId": fid,
            "status": trace_status,
            "omittedReason": reason if trace_status != "PASS" else None,
        }) + "\n")
PY

# Perform internal privacy scan across all text and binary files before writing summary.json
PRIVACY_SCAN_OUT="$(python3 - "$ARTIFACT_DIR" "$UDID" <<'PY'
import json, os, re, sys

root, raw_udid = sys.argv[1], sys.argv[2]
forbidden = re.compile(rb"(?i)(cherri-shortcut-audit-20261110|\b[0-3][0-9]\.plist\b|BEGIN [A-Z ]*PRIVATE KEY|ghp_[A-Za-z0-9_]+|github_pat_[A-Za-z0-9_]+|sk-[A-Za-z0-9]{16,})")
violations = []

for dirpath, _, filenames in os.walk(root):
    for fn in filenames:
        p = os.path.join(dirpath, fn)
        with open(p, "rb") as f:
            data = f.read()
        m = forbidden.search(data)
        if m:
            violations.append(f"{fn}: forbidden pattern {m.group(0).decode('utf-8', errors='replace')}")
        if raw_udid and len(raw_udid) >= 36 and raw_udid.encode("utf-8") in data:
            violations.append(f"{fn}: unmasked simulator UDID present")

print(json.dumps({"passed": len(violations) == 0, "violations": violations}))
PY
)"

# Write summary.json and docs/report.md with strict prerequisite invariant verification
python3 - "$ARTIFACT_DIR" "$RUN_ID" "$SOURCE_SHA" "$REQUIRE_RUNTIME" \
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
  "$GATE_PHYSICAL_DEVICE_TESTED" "$UDID" <<'PY'
import json
import os
import re
import sys

artifact_dir, run_id, sha, req_rt = sys.argv[1:5]
raw_udid = sys.argv[19] if len(sys.argv) > 19 else ""
gates = {
    "RUNNER_AVAILABLE": sys.argv[5],
    "SIMULATOR_BOOTED": sys.argv[6],
    "SHORTCUTS_APP_INSTALLED": sys.argv[7],
    "SHORTCUTS_LAUNCHED": sys.argv[8],
    "EDITOR_OPENED": sys.argv[9],
    "FILE_IMPORT_ATTEMPTED": sys.argv[10],
    "COMPILER_ARTIFACT_IMPORTED": sys.argv[11],
    "SHORTCUT_EXISTS_IN_LIBRARY": sys.argv[12],
    "ACTION_EDITOR_CAPTURED": sys.argv[13],
    "SHORTCUT_RUN_ATTEMPTED": sys.argv[14],
    "SHORTCUT_RUN_COMPLETED": sys.argv[15],
    "CALLBACK_CAPTURED": sys.argv[16],
    "TRACE_CAPTURED": sys.argv[17],
    "PHYSICAL_DEVICE_TESTED": sys.argv[18],
}

# Enforce prerequisite invariants fail-closed
if gates["COMPILER_ARTIFACT_IMPORTED"] != "PASS":
    gates["SHORTCUT_EXISTS_IN_LIBRARY"] = gates["COMPILER_ARTIFACT_IMPORTED"]
if gates["SHORTCUT_EXISTS_IN_LIBRARY"] != "PASS":
    if gates["ACTION_EDITOR_CAPTURED"] == "EDITOR_CAPTURE_COMPLETE":
        gates["ACTION_EDITOR_CAPTURED"] = "PARTIAL_EDITOR_CAPTURE"
    gates["SHORTCUT_RUN_ATTEMPTED"] = gates["SHORTCUT_EXISTS_IN_LIBRARY"]
    gates["SHORTCUT_RUN_COMPLETED"] = gates["SHORTCUT_EXISTS_IN_LIBRARY"]
    gates["TRACE_CAPTURED"] = gates["SHORTCUT_EXISTS_IN_LIBRARY"]
if gates["SHORTCUT_RUN_COMPLETED"] != "PASS" and gates["TRACE_CAPTURED"] == "PASS":
    gates["TRACE_CAPTURED"] = "FAIL_RUNTIME"

if (
    gates["SHORTCUT_RUN_COMPLETED"] == "PASS"
    and gates["TRACE_CAPTURED"] == "PASS"
    and gates["ACTION_EDITOR_CAPTURED"] == "EDITOR_CAPTURE_COMPLETE"
):
    readiness = "READY_SIMULATOR_VERIFIED"
elif gates["COMPILER_ARTIFACT_IMPORTED"] == "BLOCKED_SIGNING":
    readiness = "PARTIAL_READY_WITH_BLOCKED_SIGNING"
else:
    readiness = "PARTIAL_READY"

# Scan all artifact files (including binary PNGs) for forbidden patterns and raw UDIDs
forbidden = re.compile(rb"(?i)(cherri-shortcut-audit-20261110|\b[0-3][0-9]\.plist\b|BEGIN [A-Z ]*PRIVATE KEY|ghp_[A-Za-z0-9_]+|github_pat_[A-Za-z0-9_]+|sk-[A-Za-z0-9]{16,})")
custom_server = os.environ.get("CHERRI_SIGNING_SERVER", "").strip()
privacy_violations = []
for dirpath, _, filenames in os.walk(artifact_dir):
    for fn in filenames:
        p = os.path.join(dirpath, fn)
        with open(p, "rb") as f:
            data = f.read()
        m = forbidden.search(data)
        if m:
            privacy_violations.append(f"{fn}: {m.group(0).decode('utf-8', errors='replace')}")
        if raw_udid and len(raw_udid) >= 36 and raw_udid.encode("utf-8") in data:
            privacy_violations.append(f"{fn}: unmasked simulator UDID present")
        if custom_server and "sign.shortcuty.app" not in custom_server and custom_server.encode("utf-8") in data:
            privacy_violations.append(f"{fn}: custom signing URL leaked into artifact")

privacy_passed = (len(privacy_violations) == 0)

summary = {
    "schemaVersion": "1.4",
    "runId": run_id,
    "compilerCommitSha": sha,
    "requireRuntime": (req_rt == "true"),
    "releaseReadiness": readiness,
    "privacyGatePassed": privacy_passed,
    "privacyViolations": privacy_violations,
    "gates": gates,
}
with open(os.path.join(artifact_dir, "summary.json"), "w", encoding="utf-8") as f:
    json.dump(summary, f, indent=2)

lines = [
    f"# Cherri iPadOS 27 Simulator QA Report (`{run_id}`)",
    "",
    f"- **Compiler Commit SHA:** `{sha}`",
    f"- **Release Readiness:** `{readiness}`",
    f"- **Require Runtime Mode:** `{req_rt}`",
    f"- **Privacy Gate Passed:** `{privacy_passed}`",
    "",
    "## Progressive Sub-Gate Outcomes",
    "",
    "| Sub-Gate | Status |",
    "|---|---|",
]
for k, v in gates.items():
    lines.append(f"| `{k}` | `{v}` |")

report_md = "\n".join(lines) + "\n"
with open(os.path.join(artifact_dir, "docs", "report.md"), "w", encoding="utf-8") as f:
    f.write(report_md)

step_summary = os.environ.get("GITHUB_STEP_SUMMARY")
if step_summary:
    with open(step_summary, "a", encoding="utf-8") as sf:
        sf.write(report_md)
PY

echo "=== QA Run Complete: $ARTIFACT_DIR/summary.json ==="
cat "$ARTIFACT_DIR/summary.json"

if [ "$REQUIRE_RUNTIME" = "true" ]; then
  if [ "$GATE_SHORTCUT_RUN_COMPLETED" != "PASS" ] || [ "$GATE_TRACE_CAPTURED" != "PASS" ]; then
    echo "ERROR: require_runtime=true but SHORTCUT_RUN_COMPLETED=$GATE_SHORTCUT_RUN_COMPLETED TRACE_CAPTURED=$GATE_TRACE_CAPTURED" >&2
    exit 1
  fi
fi
