# P10 Physical iPadOS / iOS 27 Acceptance Test Kit (`tests/ipad_acceptance/`)

This directory contains the reproducible physical-device acceptance fixtures for **Cherri P10**.
While all static contracts, plist wire structures, 3-way upstream oracle comparisons (`bcc0095`), decompile/recompile round-trips, and macOS 27 runtime extraction checks are verified automatically in `go test ./...`, the following 9 `.cherri` test scenarios cover the 14 physical-device execution requirements that **must be run on a real iPadOS/iOS 27 device** before marking hardware-dependent Asana tasks as `COMPLETED`.

## Scenarios & Asana Mapping

| Fixture File | Asana Task GID | Target Feature / Scenario | Expected Physical Device Result |
| :--- | :--- | :--- | :--- |
| `01_run_fixed_and_dynamic_dict.cherri` | `1219750185341254` (A2) | `run()` with fixed target, dynamic target, and nested dictionary payload (`WFInput`, `WFWorkflowName`, `WFWorkflow`) | Target Shortcut `P10EchoTarget` receives full dictionary (`server`, `routeId`, `nested.city`) and returns output to caller. |
| `02_conditional_getvalue_string.cherri` | `1219750434313969` (A3) | `getValue(job, key: "server") == "General Context Server"` with `WFCoercionVariableAggrandizement=WFStringContentItem` | Displays `PASS_GETVALUE_STRING_LITERAL`, `PASS_GETVALUE_STRING_VARIABLE`, and `PASS_INDEX_STRING_LITERAL` without numeric coercion prompt. |
| `03_lumen_storage27_roundtrip.cherri` | `1219814215189434` (A4) | iOS/iPadOS 27 `storeValue`, `getStoredValue`, `deleteStoredValue` (global & local scopes) | Same-run displays `PASS_SAME_RUN: LUMEN_STORAGE_OK`; second run displays `PREV_RUN_VALUE: LUMEN_PERSISTENCE_20261025`; deleted key returns empty. |
| `04_jsonrequest_dynamic_fields.cherri` | `1219800356179243`, `1219800392994574` (B1/B2) | `jsonRequest` with literal dictionary containing dynamic variables, nested objects, arrays, numbers, booleans | `httpbin.org/post` echoes valid JSON body with `"city": "ירושלים"`, `"lineNumber": 480`, `"realtime": true`. |
| `05_filerequest_raw_vs_chained.cherri` | `1219800511200890`, `1219604360875370` (B3) | 4-action raw `gettext` -> `fileRequest` vs chained `replaceText` -> `fileRequest` (`WFHTTPBodyType=File`, `WFRequestVariable`) | Both requests transmit exact UTF-8 JSON body without parameter loss or truncation. |
| `06_html_preview_hebrew.cherri` | `1219604672882961` (B4) | Direct HTML rendering via `gettext` -> `setName("תוצאות מסלולים.html")` -> `showWebpage` (no base64) | Opens native QuickLook / WebView rendering RTL Hebrew HTML card directly without base64 data URI errors. |
| `07_content_item_filters.cherri` | `1220085573024106` (C5) | `filterReminders` (AND) and `filterEvents` (OR) with `WFContentPredicateTableTemplate` | Executes native Shortcuts filter queries without UI predicate corruption. |
| `08_edit_reminder_and_event.cherri` | `1220085573024106` (C5) | `editReminder` and `editEvent` in `Set` and `Remove` (`RemoveSpecifiedValue`) modes | Updates Notes/Location and removes specified Tag/URL on test Reminder/Event. |
| `09_app_intents_suite.cherri` | `1220085573024106` (C5) | Curated Apple App Intents (`createAlarm`, `startStopwatch`, `stopStopwatch`, `createRemindersList`, `searchSpotlight`) | Resolves `AppIntentDescriptor` bundles on iPadOS 27 and executes actions cleanly. |

## Automated Compilation & Round-Trip Verification

All 9 fixtures are compiled and verified automatically in Go test `TestP10_IPadAcceptanceKit_CompileAndStructure` (`p10_http_json_test.go`).
To sign on macOS for AirDrop / iCloud import onto a physical iPad:

```bash
for f in tests/ipad_acceptance/*.cherri; do
  ./cherri "$f"
done
```
