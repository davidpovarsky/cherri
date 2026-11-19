package main

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"howett.net/plist"

	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/types"
)

type storageProbeOperation struct {
	id     string
	global bool
}

func storageProbeExpected(id string) (string, []storageProbeOperation, int) {
	set, get, del := "setstoredcontent", "getstoredcontent", "deletestoredcontent"
	switch id {
	case "qa-storage-text-same-run-v2":
		return "cherri.qa.p15.storage.text.35a6b60", []storageProbeOperation{{set, true}, {get, true}, {del, true}}, 2
	case "qa-storage-scope-v2":
		return "cherri.qa.p15.storage.scope.35a6b60", []storageProbeOperation{
			{set, true}, {set, false}, {get, true}, {get, false}, {del, true}, {del, false}}, 4
	case "qa-storage-delete-v2":
		return "cherri.qa.p15.storage.delete.35a6b60", []storageProbeOperation{
			{set, true}, {get, true}, {del, true}, {get, true}}, 2
	default:
		return "", nil, 0
	}
}

// Inspect the production compiler's emitted graph. This does not execute or
// simulate storage, and cannot establish persistence or typed archival.
func validateStorageProbeWire(id string, actions []map[string]any) error {
	key, expected, conditionalCount := storageProbeExpected(id)
	if key == "" {
		return fmt.Errorf("unknown reviewed storage probe")
	}
	typed := make([]ShortcutAction, len(actions))
	for i, action := range actions {
		typed[i] = ShortcutAction{
			WFWorkflowActionIdentifier: fmt.Sprint(action["WFWorkflowActionIdentifier"]),
			WFWorkflowActionParameters: debugParamMap(action["WFWorkflowActionParameters"]),
		}
	}
	clearIndex, err := validateP15DiagnosticZeroClear(typed)
	if err != nil {
		return err
	}
	allowed := map[string]bool{
		"comment": true, "gettext": true, "setvariable": true, "appendvariable": true,
		"text.combine": true, "output": true, "setstoredcontent": true,
		"getstoredcontent": true, "deletestoredcontent": true, "getitemtype": true,
		"conditional": true, "count": true,
	}
	producers := map[string]map[string]any{}
	reads := map[string]bool{}
	used := map[string]bool{}
	var operations []storageProbeOperation
	conditions := 0
	var visit func(any)
	visit = func(value any) {
		switch item := value.(type) {
		case map[string]any:
			if item["Type"] == "ActionOutput" {
				if uuid, ok := item["OutputUUID"].(string); ok {
					used[uuid] = true
				}
			}
			for _, nested := range item {
				visit(nested)
			}
		case []any:
			for _, nested := range item {
				visit(nested)
			}
		}
	}
	for index, action := range actions {
		nativeID, _ := action["WFWorkflowActionIdentifier"].(string)
		suffix := strings.TrimPrefix(nativeID, "is.workflow.actions.")
		if (!allowed[suffix] && index != clearIndex) || nativeID != "is.workflow.actions."+suffix {
			return fmt.Errorf("unexpected effect: %s", nativeID)
		}
		p := debugParamMap(action["WFWorkflowActionParameters"])
		visit(p)
		if suffix == "conditional" && fmt.Sprint(p["WFControlFlowMode"]) == "0" {
			if fmt.Sprint(p["WFCondition"]) != "5" {
				return fmt.Errorf("semantic comparison changed")
			}
			conditions++
		}
		if strings.HasSuffix(suffix, "storedcontent") {
			if p["WFStoredContentKey"] != key {
				return fmt.Errorf("storage key escaped owned namespace")
			}
			global, ok := p["WFStoredContentGlobalValue"].(bool)
			if !ok {
				return fmt.Errorf("global/local scope is not an explicit native Bool")
			}
			operations = append(operations, storageProbeOperation{suffix, global})
			if suffix == "setstoredcontent" && id != "qa-storage-delete-v2" {
				input := debugParamMap(p["WFInput"])
				value := debugParamMap(input["Value"])
				attachments := debugParamMap(value["attachmentsByRange"])
				ref := debugParamMap(attachments["{0, 1}"])
				producer := producers[fmt.Sprint(ref["OutputUUID"])]
				if input["WFSerializationType"] != "WFTextTokenString" || value["string"] != "\ufffc" ||
					len(attachments) != 1 || ref["Type"] != "ActionOutput" ||
					producer["WFWorkflowActionIdentifier"] != "is.workflow.actions.gettext" {
					return fmt.Errorf("authored Text input lost its original Text producer")
				}
				params := debugParamMap(producer["WFWorkflowActionParameters"])
				if ref["OutputName"] != params["CustomOutputName"] {
					return fmt.Errorf("Text reference output identity mismatch")
				}
				text, ok := params["WFTextActionText"].(string)
				if !ok || !strings.HasPrefix(text, "P15_STORAGE_") {
					return fmt.Errorf("store did not consume the synthetic Text witness")
				}
				if id == "qa-storage-text-same-run-v2" &&
					(!strings.Contains(text, "\u05e9\u05b8\u05c1\u05dc\u05d5\u05b9\u05dd") ||
						!strings.Contains(text, "\U0001f352") || strings.Contains(text, "????")) {
					return fmt.Errorf("native Text Unicode bytes changed")
				}
			}
			if suffix == "getstoredcontent" {
				uuid, ok := p["UUID"].(string)
				if !ok || uuid == "" {
					return fmt.Errorf("readback output lacks native identity")
				}
				reads[uuid] = true
			}
		}
		if uuid, ok := p["UUID"].(string); ok {
			producers[uuid] = action
		}
	}
	if !reflect.DeepEqual(operations, expected) || conditions != conditionalCount {
		return fmt.Errorf("storage operation order/scope or semantic assertions changed: %+v / %d", operations, conditions)
	}
	for uuid := range reads {
		if !used[uuid] {
			return fmt.Errorf("readback output is unused")
		}
	}
	return nil
}

func TestP15StorageNativeMetadataDoesNotProveTypedArchive(t *testing.T) {
	data, err := os.ReadFile("docs/apple-runtime/macos-27/builtin-actions.json")
	if err != nil {
		t.Fatal(err)
	}
	var native map[string]map[string]any
	if err := json.Unmarshal(data, &native); err != nil {
		t.Fatal(err)
	}
	for _, row := range []struct{ id, class string }{
		{"setstoredcontent", "WFSetStoredContentAction"},
		{"getstoredcontent", "WFGetStoredContentAction"},
		{"deletestoredcontent", "WFDeleteStoredContentAction"},
	} {
		entry := native["is.workflow.actions."+row.id]
		if entry["ActionClass"] != row.class {
			t.Fatalf("independent native class mismatch: %s", row.id)
		}
		scopeFound := false
		for _, parameter := range entry["Parameters"].([]any) {
			p := debugParamMap(parameter)
			if p["Key"] == "WFStoredContentGlobalValue" {
				scopeFound = p["Class"] == "WFSwitchParameter" && p["DefaultValue"] == false
			}
		}
		if !scopeFound {
			t.Fatalf("independent native scope metadata missing: %s", row.id)
		}
	}
	store := native["is.workflow.actions.setstoredcontent"]
	if store["InputPassthrough"] != true {
		t.Fatal("native passthrough metadata changed")
	}
	input := debugParamMap(store["Input"])
	if input["ParameterKey"] != "WFInput" || input["Multiple"] != true ||
		!reflect.DeepEqual(input["Types"], []any{"WFContentItem"}) {
		t.Fatal("native generic content metadata changed")
	}
	textUI := false
	for _, parameter := range store["Parameters"].([]any) {
		p := debugParamMap(parameter)
		textUI = textUI || p["Key"] == "WFInput" && p["Class"] == "WFTextInputParameter"
	}
	if !textUI {
		t.Fatal("native input UI class changed; reassess the storage boundary")
	}
	action, ok := schema.DefaultRegistry().LookupAction("storeValue")
	if !ok || len(action.Parameters) != 3 || !action.Parameters[1].Type.Equals(types.Text) ||
		action.Parameters[1].Codec != "text" {
		t.Fatal("current authored Text boundary changed; typed archival needs separate acceptance")
	}
}

func TestP15StorageProbeWireContractsAndCausalMutations(t *testing.T) {
	for _, id := range []string{"qa-storage-text-same-run-v2", "qa-storage-scope-v2", "qa-storage-delete-v2"} {
		source, err := os.ReadFile("tools/shortcuts-simulator-qa/fixtures/" + id + ".cherri")
		if err != nil {
			t.Fatal(err)
		}
		compiled, err := compileBuildProfileFixture(t, string(source), false, BuildProfileDebug)
		if err != nil {
			t.Fatal(err)
		}
		encoded, err := plist.Marshal(compiled, plist.XMLFormat)
		if err != nil {
			t.Fatal(err)
		}
		var actions []map[string]any
		if _, err := plist.Unmarshal(encoded, &actions); err != nil {
			t.Fatal(err)
		}
		if err := validateStorageProbeWire(id, actions); err != nil {
			t.Fatalf("%s: %v", id, err)
		}
		for _, fault := range []string{"foreign-key", "scope", "no-cleanup", "constant-read", "comparison", "extra-network", "old-empty-seed", "clear-input", "nothing-input", "clear-target", "extra-nothing", "consumed-clear"} {
			bytes, _ := json.Marshal(actions)
			var altered []map[string]any
			_ = json.Unmarshal(bytes, &altered)
			switch fault {
			case "foreign-key", "scope", "no-cleanup", "constant-read", "comparison":
				for index, action := range altered {
					name := fmt.Sprint(action["WFWorkflowActionIdentifier"])
					p := debugParamMap(action["WFWorkflowActionParameters"])
					if fault == "foreign-key" && strings.HasSuffix(name, "setstoredcontent") {
						p["WFStoredContentKey"] = "unreviewed.storage.key"
						break
					}
					if fault == "scope" && strings.HasSuffix(name, "setstoredcontent") {
						p["WFStoredContentGlobalValue"] = !p["WFStoredContentGlobalValue"].(bool)
						break
					}
					if fault == "no-cleanup" && strings.HasSuffix(name, "deletestoredcontent") {
						altered = append(altered[:index], altered[index+1:]...)
						break
					}
					if fault == "constant-read" && strings.HasSuffix(name, "getstoredcontent") {
						action["WFWorkflowActionIdentifier"] = "is.workflow.actions.gettext"
						break
					}
					if fault == "comparison" && name == "is.workflow.actions.conditional" &&
						fmt.Sprint(p["WFControlFlowMode"]) == "0" {
						p["WFCondition"] = 4
						break
					}
				}
			case "extra-network":
				altered = append(altered, map[string]any{"WFWorkflowActionIdentifier": "is.workflow.actions.downloadurl"})
			case "old-empty-seed", "clear-input", "nothing-input", "clear-target", "extra-nothing", "consumed-clear":
				for index, action := range altered {
					if action["WFWorkflowActionIdentifier"] != "is.workflow.actions.nothing" {
						continue
					}
					p := debugParamMap(action["WFWorkflowActionParameters"])
					next := debugParamMap(altered[index+1]["WFWorkflowActionParameters"])
					ref := map[string]any{"Value": map[string]any{"Type": "ActionOutput", "OutputUUID": p["UUID"]}}
					switch fault {
					case "old-empty-seed":
						action["WFWorkflowActionIdentifier"] = "is.workflow.actions.gettext"
						p["WFTextActionText"] = ""
					case "clear-input":
						next["WFInput"] = ref
					case "nothing-input":
						p["WFInput"] = "retained previous value"
					case "clear-target":
						next["WFVariableName"] = "unreviewed_buffer"
					case "extra-nothing":
						altered = append(altered, map[string]any{"WFWorkflowActionIdentifier": "is.workflow.actions.nothing"})
					case "consumed-clear":
						debugParamMap(altered[len(altered)-1]["WFWorkflowActionParameters"])["WFOutput"] = ref
					}
					break
				}
			}
			if err := validateStorageProbeWire(id, altered); err == nil {
				t.Fatalf("%s: causal %s mutation passed", id, fault)
			}
			if err := validateStorageProbeWire(id, actions); err != nil {
				t.Fatalf("%s: restored control failed: %v", id, err)
			}
		}
	}
}

func TestP15StorageTextPrimitivesBothFrontendsProfiles(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		for _, profile := range []BuildProfile{BuildProfileDebug, BuildProfileRelease} {
			decl := "let"
			if legacy {
				decl = "const"
			}
			source := decl + " value = text(\"P15_STORAGE_PRIMITIVE\")\n" +
				"storeValue(\"cherri.qa.p15.storage.primitive.35a6b60\", value, true)\n" +
				decl + " restored = getStoredValue(\"cherri.qa.p15.storage.primitive.35a6b60\", true)\n" +
				"typeOf(restored)\ndeleteStoredValue(\"cherri.qa.p15.storage.primitive.35a6b60\", true)\n"
			if legacy {
				source = "#include 'actions/storage'\n" + source
			}
			actions, err := compileBuildProfileFixture(t, source, legacy, profile)
			if err != nil {
				t.Fatalf("legacy=%t/%s: %v", legacy, profile, err)
			}
			var actual []string
			for _, action := range actions {
				name := action.WFWorkflowActionIdentifier
				if strings.HasSuffix(name, "storedcontent") {
					actual = append(actual, name)
				}
			}
			expected := []string{"is.workflow.actions.setstoredcontent", "is.workflow.actions.getstoredcontent", "is.workflow.actions.deletestoredcontent"}
			if !reflect.DeepEqual(actual, expected) {
				t.Fatalf("legacy=%t/%s: storage effects changed: %v", legacy, profile, actual)
			}
		}
	}
}
