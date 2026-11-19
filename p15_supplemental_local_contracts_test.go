package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/migrate"
	"howett.net/plist"
)

func p15SupplementalTextBytes(t *testing.T, raw []byte, want string) {
	t.Helper()
	var root map[string]any
	if _, err := plist.Unmarshal(raw, &root); err != nil {
		t.Fatal(err)
	}
	actions, ok := root["WFWorkflowActions"].([]any)
	if !ok || len(actions) != 1 {
		t.Fatalf("literal acquired extra effects: %#v", root)
	}
	action := actions[0].(map[string]any)
	if action["WFWorkflowActionIdentifier"] != "is.workflow.actions.gettext" {
		t.Fatal("literal took another native path")
	}
	value := action["WFWorkflowActionParameters"].(map[string]any)["WFTextActionText"]
	if err := p15CheckLiteralText(value, want); err != nil {
		t.Fatal(err)
	}
	if p15CheckLiteralText(value, want+"CORRUPTED") == nil {
		t.Fatal("literal oracle accepted changed source text")
	}
}

func TestP15RP05LiteralPublicRoutes(t *testing.T) {
	binary := p15AcceptanceCLI(t)
	for _, tc := range []struct{ name, literal, prefix string }{
		{"email", "reader@example.invalid", ""},
		{"plain_javascript", "const value = {key: 1}; value.key;", ""},
		{"include_text", "#include 'actions/web' is ordinary text", ""},
		{"legacy_comments", "comment control", "// const old = \"ignored\"; @value; #include 'ignored'\n/* @other = const #include */\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := tc.prefix + "text(" + strconv.Quote(tc.literal) + ")\n"
			raw, err := CompileSourceToPlist("literal.cherri", source)
			if err != nil {
				t.Fatal(err)
			}
			p15SupplementalTextBytes(t, raw, tc.literal)
			mobile := compileForMobileMode(source, "Literal control", "v2", false)
			if !mobile.OK {
				t.Fatal(mobile.Error)
			}
			raw, err = base64.StdEncoding.DecodeString(mobile.PlistBase64)
			if err != nil {
				t.Fatal(err)
			}
			p15SupplementalTextBytes(t, raw, tc.literal)
			dir := t.TempDir()
			input, output := filepath.Join(dir, "literal.cherri"), filepath.Join(dir, "literal.shortcut")
			if err := os.WriteFile(input, []byte(source), 0600); err != nil {
				t.Fatal(err)
			}
			out, stderr, err := p15RunCLI(t, binary, dir, nil, "check", input, "--json")
			if err != nil || p15DecodeOneJSON(t, out).(map[string]any)["valid"] != true {
				t.Fatalf("literal check: %v %s", err, stderr)
			}
			out, stderr, err = p15RunCLI(t, binary, dir, nil, "compile", input, "--skip-sign", "--output="+output)
			if err != nil {
				t.Fatalf("literal CLI: %v %s %s", err, out, stderr)
			}
			raw, err = os.ReadFile(output)
			if err != nil {
				t.Fatal(err)
			}
			p15SupplementalTextBytes(t, raw, tc.literal)
		})
	}
	for _, source := range []string{"const old = \"legacy\"\nshow(old)", "@old = \"legacy\"\nshow(@old)", "#include 'actions/web'\n"} {
		t.Run("reject_actual_"+strings.Split(source, " ")[0], func(t *testing.T) {
			data, err := CompileSourceToPlist("legacy.cherri", source)
			if err == nil || len(data) != 0 || !strings.Contains(err.Error(), analysis.CodeLegacySyntax) {
				t.Fatalf("embedded legacy route: %v", err)
			}
			mobile := compileForMobileMode(source, "Legacy rejected", "v2", false)
			if mobile.OK || mobile.PlistBase64 != "" || !strings.Contains(mobile.Error, analysis.CodeLegacySyntax) {
				t.Fatalf("mobile legacy route: %+v", mobile)
			}
			dir := t.TempDir()
			input, output := filepath.Join(dir, "legacy.cherri"), filepath.Join(dir, "old.shortcut")
			old := []byte("KEEP_EXISTING_OUTPUT")
			if err := os.WriteFile(input, []byte(source), 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(output, old, 0600); err != nil {
				t.Fatal(err)
			}
			for _, args := range [][]string{{"check", input, "--json"}, {"compile", input, "--skip-sign", "--output=" + output}} {
				out, stderr, err := p15RunCLI(t, binary, dir, nil, args...)
				if err == nil || !strings.Contains(string(out)+string(stderr), analysis.CodeLegacySyntax) {
					t.Fatalf("CLI legacy accepted or wrong diagnostic: %v %s %s", err, out, stderr)
				}
			}
			got, err := os.ReadFile(output)
			if err != nil || !bytes.Equal(old, got) {
				t.Fatal("rejected legacy source replaced existing output")
			}
		})
	}
}

type p15StaticVariantCase struct {
	name, source, identifier string
	fixed                    map[string]any
}

func p15StaticVariantCases() []p15StaticVariantCase {
	return []p15StaticVariantCase{
		{"convertToJPEG", "convertToJPEG(ShortcutInput)", "is.workflow.actions.image.convert", map[string]any{"WFImageFormat": "JPEG"}},
		{"base64Encode", "base64Encode(ShortcutInput)", "is.workflow.actions.base64encode", map[string]any{"input": "Encode", "WFEncodeMode": "Encode"}},
		{"base64Decode", "base64Decode(ShortcutInput)", "is.workflow.actions.base64encode", map[string]any{"WFEncodeMode": "Decode"}},
		{"DNDOn", "DNDOn()", "is.workflow.actions.dnd.set", map[string]any{"Enabled": float64(1)}},
		{"DNDOff", "DNDOff()", "is.workflow.actions.dnd.set", map[string]any{"Enabled": float64(0)}},
	}
}

func p15StaticVariantError(actions []ShortcutAction, tc p15StaticVariantCase) error {
	if len(actions) != 1 || actions[0].WFWorkflowActionIdentifier != tc.identifier {
		return fmt.Errorf("variant identity/cardinality changed: %+v", actions)
	}
	p := actions[0].WFWorkflowActionParameters
	for key, value := range tc.fixed {
		if !plistValuesStrictlyEqual(value, p[key]) {
			return fmt.Errorf("variant %s lost exact fixed field %s: %#v", tc.name, key, p[key])
		}
	}
	if strings.HasPrefix(tc.name, "DND") {
		focus, err := p15ReadDictionaryLiteral(p["FocusModes"])
		if err != nil || !reflect.DeepEqual(focus, map[string]any{"DisplayString": "Do Not Disturb", "Identifier": "com.apple.donotdisturb.mode.default"}) {
			return fmt.Errorf("FocusModes contract changed: %#v %v", focus, err)
		}
	}
	return nil
}

func TestP15RP09StaticVariantNativeDifferential(t *testing.T) {
	for _, tc := range p15StaticVariantCases() {
		t.Run(tc.name, func(t *testing.T) {
			v2 := compileDebugFixture(t, tc.source, false)
			legacy := compileDebugFixture(t, tc.source, true)
			if err := p15StaticVariantError(v2, tc); err != nil {
				t.Fatal(err)
			}
			if err := p15StaticVariantError(legacy, tc); err != nil {
				t.Fatal(err)
			}
			if !plistValuesStrictlyEqual(legacy[0].WFWorkflowActionParameters, v2[0].WFWorkflowActionParameters) {
				t.Fatal("static variant differs from canonical Legacy encoder")
			}
			for key := range tc.fixed {
				changed := append([]ShortcutAction(nil), v2...)
				changed[0].WFWorkflowActionParameters = deepClonePlistMap(v2[0].WFWorkflowActionParameters)
				delete(changed[0].WFWorkflowActionParameters, key)
				if p15StaticVariantError(changed, tc) == nil {
					t.Fatal("fixed-field deletion passed variant oracle", key)
				}
			}
			changed := append([]ShortcutAction(nil), v2...)
			changed[0].WFWorkflowActionIdentifier = "com.example.p15.wrong.variant"
			if p15StaticVariantError(changed, tc) == nil {
				t.Fatal("wrong action identity passed variant oracle")
			}
		})
	}
}

// Replace only the real canonical definition used by production emission. The
// frontend and generated schema are unchanged, so the observed behavior can
// only originate in the shared callback/static/descriptor implementation.
func TestP15CanonicalCallbackCausality(t *testing.T) {
	cases := []struct{ name, source, kind, identifier string }{
		{"emailAddress", "emailAddress(email: \"reader@example.invalid\")", "make", "is.workflow.actions.email"},
		{"getPDFText", "getPDFText(ShortcutInput)", "append", "is.workflow.actions.gettextfrompdf"},
		{"DNDOn", "DNDOn()", "static", "is.workflow.actions.dnd.set"},
		{"turnOffAlarm", "turnOffAlarm(ShortcutInput)", "intent", "com.apple.mobiletimer-framework.MobileTimerIntents.MTToggleAlarmIntent"},
	}
	for _, tc := range cases {
		t.Run(tc.kind, func(t *testing.T) {
			definitions := EnsureCanonicalStandardActions()
			original := definitions[tc.name]
			if original == nil {
				t.Fatal("canonical fixture definition missing")
			}
			baseline := compileDebugFixture(t, tc.source, false)
			if len(baseline) != 1 || baseline[0].WFWorkflowActionIdentifier != tc.identifier {
				t.Fatal("baseline did not use expected action")
			}
			changed := *original
			calls := 0
			checkArguments := func(args []actionArgument) {
				calls++
				if len(args) != 1 {
					t.Fatalf("canonical callback received %d evaluated arguments", len(args))
				}
				if tc.kind == "make" && getArgValue(args[0]) != "reader@example.invalid" {
					t.Fatal("canonical callback lost evaluated literal")
				}
				if tc.kind == "append" && args[0].valueType != Variable {
					t.Fatal("canonical callback lost typed system reference")
				}
			}
			switch tc.kind {
			case "make":
				if original.makeParams == nil {
					t.Fatal("fixture has no full canonical callback")
				}
				changed.makeParams = func(args []actionArgument) map[string]any {
					checkArguments(args)
					result := original.makeParams(args)
					result["P15CallbackMarker"] = "make"
					return result
				}
			case "append":
				if original.appendParamsFunc == nil {
					t.Fatal("fixture has no appended canonical callback")
				}
				changed.appendParamsFunc = func(args []actionArgument) map[string]any {
					checkArguments(args)
					result := original.appendParamsFunc(args)
					result["P15CallbackMarker"] = "append"
					return result
				}
			case "static":
				changed.appendParams = deepClonePlistMap(original.appendParams)
				changed.appendParams["P15CallbackMarker"] = "static"
			case "intent":
				changed.appIntent = appIntent{name: "P15 intent", bundleIdentifier: "com.example.p15.intent", appIntentIdentifier: "P15Observed", teamIdentifier: "TEAM12345X", requiresAppInstallation: requiresAppInstallation(false)}
			}
			definitions[tc.name] = &changed
			defer func() { definitions[tc.name] = original }()
			emitted := compileDebugFixture(t, tc.source, false)
			if len(emitted) != 1 || emitted[0].WFWorkflowActionIdentifier != tc.identifier {
				t.Fatal("canonical mutation changed unrelated identity")
			}
			parameters := emitted[0].WFWorkflowActionParameters
			if tc.kind == "intent" {
				expected := map[string]any{"Name": "P15 intent", "BundleIdentifier": "com.example.p15.intent", "AppIntentIdentifier": "P15Observed", "TeamIdentifier": "TEAM12345X", "ActionRequiresAppInstallation": false}
				if !plistValuesStrictlyEqual(expected, parameters["AppIntentDescriptor"]) {
					t.Fatalf("complete canonical descriptor lost: %#v", parameters["AppIntentDescriptor"])
				}
				if plistValuesStrictlyEqual(expected, baseline[0].WFWorkflowActionParameters["AppIntentDescriptor"]) {
					t.Fatal("mutation was not causal")
				}
				for key := range expected {
					broken := deepClonePlistMap(expected)
					delete(broken, key)
					if plistValuesStrictlyEqual(expected, broken) {
						t.Fatal("descriptor omission accepted", key)
					}
				}
			} else {
				if parameters["P15CallbackMarker"] != tc.kind || baseline[0].WFWorkflowActionParameters["P15CallbackMarker"] != nil {
					t.Fatal("canonical mutation not observed only after installation")
				}
				if (tc.kind == "make" || tc.kind == "append") && calls != 1 {
					t.Fatal("canonical callback was not called exactly once", calls)
				}
			}
			definitions[tc.name] = original
			restored := compileDebugFixture(t, tc.source, false)
			if !plistValuesStrictlyEqual(baseline[0].WFWorkflowActionParameters, restored[0].WFWorkflowActionParameters) {
				t.Fatal("restored canonical definition differs from original control")
			}
		})
	}
}

const p15SevenLegacyTriggers = "#trigger battery 80\n#trigger screenshot photos, files\n#trigger wifi joined\n#trigger bluetooth connect\n#trigger stageManager on\n#trigger display connect\n#trigger charging connect\ntext(\"P15_TRIGGER_CONTROL\")\n"

func p15SevenTriggerRowsError(root map[string]any) error {
	rows, ok := root["WFWorkflowTriggers"].([]any)
	if !ok || len(rows) != 7 {
		return fmt.Errorf("expected seven distinct serialized triggers")
	}
	expected := []struct {
		id     string
		params map[string]any
	}{
		{"WFBatteryLevelTrigger", map[string]any{"WFBatteryLevel": float64(79.99)}},
		{"WFScreenshotTrigger", map[string]any{"ScreenshotLocations": []any{"photos", "files"}}},
		{"WFWifiTrigger", map[string]any{"WFConnectionType": "joined"}},
		{"WFBluetoothTrigger", map[string]any{"WFBluetoothConnectionType": "connect"}},
		{"WFStageManagerTrigger", map[string]any{"WFStageManagerType": "on"}},
		{"WFExternalDisplayTrigger", map[string]any{"WFConnectionType": "connect"}},
		{"WFPlugInTrigger", map[string]any{"WFConnectionType": "connect"}},
	}
	ids := map[string]bool{}
	for index, want := range expected {
		row, ok := rows[index].(map[string]any)
		if !ok || row["WFTriggerIdentifier"] != want.id || !plistValuesStrictlyEqual(row["WFTriggerSerializedParameters"], want.params) {
			return fmt.Errorf("trigger %d changed order/type/payload: %#v", index, rows[index])
		}
		id, ok := row["WFTriggerUUID"].(string)
		if !ok || id == "" || ids[id] {
			return fmt.Errorf("trigger identity absent or repeated")
		}
		ids[id] = true
	}
	return nil
}

func TestP15RP32SevenTriggerMigrationAndNativeRoundTrip(t *testing.T) {
	migrated, diagnostics, err := migrate.MigrateSourceWithDiagnostics(p15SevenLegacyTriggers)
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("seven-family migration: %v %+v", err, diagnostics)
	}
	legacy, err := compileLegacySourceToRawMapInMemory(p15SevenLegacyTriggers)
	if err != nil {
		t.Fatal(err)
	}
	if err := p15SevenTriggerRowsError(legacy); err != nil {
		t.Fatal("canonical legacy trigger contract", err)
	}
	native := p15MetadataNative(t, "p15-seven-triggers", migrated)
	if err := p15SevenTriggerRowsError(native); err != nil {
		t.Fatal(err)
	}
	p15NativeImportRoundTrip(t, "seven-trigger-native-preservation", native, plist.XMLFormat, plist.BinaryFormat)
	for index := 0; index < 7; index++ {
		t.Run(fmt.Sprintf("reject_missing_family_%d", index), func(t *testing.T) {
			changed := deepClonePlistMap(native)
			rows := changed["WFWorkflowTriggers"].([]any)
			changed["WFWorkflowTriggers"] = append(rows[:index], rows[index+1:]...)
			if p15SevenTriggerRowsError(changed) == nil || p15NativePreservationError(native, changed) == nil {
				t.Fatal("missing trigger family accepted")
			}
		})
	}
	for _, fault := range []string{"wrong_payload", "wrong_order", "duplicate_identity"} {
		t.Run(fault, func(t *testing.T) {
			changed := deepClonePlistMap(native)
			rows := changed["WFWorkflowTriggers"].([]any)
			switch fault {
			case "wrong_payload":
				rows[0].(map[string]any)["WFTriggerSerializedParameters"].(map[string]any)["WFBatteryLevel"] = float64(0)
			case "wrong_order":
				rows[1], rows[2] = rows[2], rows[1]
			case "duplicate_identity":
				rows[1].(map[string]any)["WFTriggerUUID"] = rows[0].(map[string]any)["WFTriggerUUID"]
			}
			if p15SevenTriggerRowsError(changed) == nil || p15NativePreservationError(native, changed) == nil {
				t.Fatal("trigger corruption accepted")
			}
		})
	}
	if _, diags, err := migrate.MigrateSourceWithDiagnostics("#trigger P15_UNKNOWN connect\ntext(\"x\")"); err == nil || len(diags) == 0 {
		t.Fatal("unknown trigger was silently migrated")
	}
}

func TestP15RP30HeaderAbsenceAndNameRoundTrip(t *testing.T) {
	root := baseTestHeader()
	root["WFWorkflowName"] = "P15 native name"
	root["WFWorkflowTypes"] = []any{"ActionExtension", "QuickActions", "P15FutureSurface"}
	root["WFQuickActionSurfaces"] = []any{"Finder", "P15FutureQuickAction"}
	root["WFWorkflowInputContentItemClasses"] = []any{"WFStringContentItem"}
	root["WFWorkflowOutputContentItemClasses"] = []any{"WFURLContentItem"}
	root["WFWorkflowNoInputBehavior"] = map[string]any{"Name": "WFWorkflowNoInputBehaviorShowError", "CustomErrorMessage": "P15 no input", "ExplicitFalse": false}
	root["WFWorkflowIcon"] = map[string]any{"WFWorkflowIconGlyphNumber": uint64(59789), "WFWorkflowIconStartColor": uint64(4282601983)}
	root["P15UnknownMetadata"] = map[string]any{"nested": []any{false, uint64(0), ""}}
	delete(root, "WFWorkflowMinimumClientVersion")
	delete(root, "WFWorkflowMinimumClientVersionString")
	root["WFWorkflowActions"] = []any{p15AcceptanceAction("is.workflow.actions.gettext", map[string]any{"WFTextActionText": "P15_HEADER_CONTROL"})}
	p15NativeImportRoundTrip(t, "header-name-and-absence", root, plist.XMLFormat, plist.BinaryFormat)
	for _, key := range []string{"WFWorkflowName", "WFWorkflowTypes", "WFQuickActionSurfaces", "WFWorkflowInputContentItemClasses", "WFWorkflowOutputContentItemClasses", "WFWorkflowNoInputBehavior", "WFWorkflowIcon", "P15UnknownMetadata"} {
		t.Run("reject_drop_"+key, func(t *testing.T) {
			changed := deepClonePlistMap(root)
			delete(changed, key)
			if p15NativePreservationError(root, changed) == nil {
				t.Fatal("metadata loss accepted")
			}
		})
	}
	for _, key := range []string{"WFWorkflowMinimumClientVersion", "WFWorkflowMinimumClientVersionString"} {
		t.Run("reject_invent_"+key, func(t *testing.T) {
			changed := deepClonePlistMap(root)
			changed[key] = "9999"
			if p15NativePreservationError(root, changed) == nil {
				t.Fatal("invented platform minimum accepted")
			}
		})
	}
}
