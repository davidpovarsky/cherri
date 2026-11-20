package main

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/types"
)

type p15NativeTypeAction struct {
	ActionClass string
	Name        string
	Input       struct {
		ParameterKey string
		Multiple     bool
		Types        []string
	}
	Output struct {
		OutputName string
		Multiple   bool
		Types      []string
	}
	Parameters []struct {
		Key          string
		DefaultValue any
	}
}

// The checked-in Apple action specimen is independent of Cherri's aliases,
// output declarations, semantic facets and generated registry.
func p15NativeTypeActions(t *testing.T) (string, string, map[string]p15NativeTypeAction) {
	t.Helper()
	data, err := os.ReadFile("docs/apple-runtime/macos-27/builtin-actions.json")
	if err != nil {
		t.Fatal(err)
	}
	var metadata map[string]p15NativeTypeAction
	if err := json.Unmarshal(data, &metadata); err != nil {
		t.Fatal(err)
	}
	fileID, inspectID := "", ""
	for id, action := range metadata {
		switch action.ActionClass {
		case "WFGetTypeAction":
			if fileID != "" {
				t.Fatal("ambiguous native file-type action")
			}
			fileID = id
		case "WFGetItemTypeAction":
			if inspectID != "" {
				t.Fatal("ambiguous native type inspector")
			}
			inspectID = id
		}
	}
	if fileID == "" || inspectID == "" || fileID == inspectID {
		t.Fatal("independent native actions are missing or collapsed")
	}
	return fileID, inspectID, metadata
}

func TestP15GetTypeCatalogMatchesAppleFileMetadata(t *testing.T) {
	fileID, inspectID, metadata := p15NativeTypeActions(t)
	native := metadata[fileID]
	if native.Name != "Get File of Type" || native.Input.ParameterKey != "WFInput" ||
		!native.Input.Multiple || !native.Output.Multiple || native.Output.OutputName != "File of Type" ||
		!reflect.DeepEqual(native.Output.Types, []string{"public.data"}) {
		t.Fatalf("unexpected Apple file-content contract: %+v", native)
	}
	if metadata[inspectID].Name != "Get Type" || !metadata[inspectID].Output.Multiple ||
		!reflect.DeepEqual(metadata[inspectID].Output.Types, []string{"NSString"}) {
		t.Fatal("independent inspector contract changed")
	}
	foundNativeFileType := false
	for _, parameter := range native.Parameters {
		if parameter.Key == "WFFileType" && parameter.DefaultValue == "public.rtf" {
			foundNativeFileType = true
		}
	}
	if !foundNativeFileType {
		t.Fatal("native optional UTI default changed")
	}

	// Use the same initialization as the public --actions-json route.
	resetParser()
	markBuiltins()
	defineRawAction()
	loadStandardActions()
	catalog := buildActionCatalog()
	if len(catalog) != 505 {
		t.Fatalf("native catalog boundary changed: %d", len(catalog))
	}
	found := false
	for _, action := range catalog {
		if action.Name != "getType" {
			continue
		}
		found = true
		if action.ShortcutIdentifier != fileID || action.Title != native.Name ||
			action.OutputType != string(Variable) || !strings.Contains(action.Description, "typeOf") {
			t.Errorf("canonical declaration misdescribes file output: %+v", action)
		}
		fileType := findCatalogParameter(t, action, "fileType")
		if fileType.Key != "WFFileType" || !fileType.Optional || fileType.Default != nil {
			t.Errorf("optional file type changed: %+v", fileType)
		}
		input := findCatalogParameter(t, action, "input")
		if input.Key != native.Input.ParameterKey || input.Type != string(Variable) {
			t.Errorf("native content input changed: %+v", input)
		}
	}
	if !found {
		t.Fatal("getType alias disappeared")
	}
	registry := schema.DefaultRegistry()
	fileAction, exists := registry.LookupAction("getType")
	if !exists {
		t.Fatal("getType absent from semantic registry")
	}
	if fileAction.AppleIdentifier != fileID || !fileAction.OutputType.Equals(types.File) ||
		fileAction.OutputTypeName != "File" || fileAction.RawOutputType != string(Variable) ||
		fileAction.Docs.Title != native.Name {
		t.Errorf("semantic file category is false: native=%s semantic=%+v", fileID, fileAction)
	}
	inspectAction, exists := registry.LookupAction("typeOf")
	if !exists || inspectAction.AppleIdentifier != inspectID || !inspectAction.OutputType.Equals(types.Text) {
		t.Fatal("typeOf must remain the distinct native Text inspector")
	}
}

func p15GetTypeWire(acts []ShortcutAction, fileID string, explicitType bool) error {
	var original, file string
	var fileAction, preview *ShortcutAction
	for i := range acts {
		action := &acts[i]
		switch action.WFWorkflowActionIdentifier {
		case "is.workflow.actions.gettext":
			if original == "" {
				original, _ = action.WFWorkflowActionParameters["UUID"].(string)
			}
		case fileID:
			if fileAction != nil {
				return fmt.Errorf("duplicated native file action")
			}
			fileAction = action
			file, _ = action.WFWorkflowActionParameters["UUID"].(string)
		case "is.workflow.actions.previewdocument":
			preview = action
		case "is.workflow.actions.getitemtype":
			return fmt.Errorf("file extraction was replaced by a type inspector")
		}
	}
	if original == "" || file == "" || fileAction == nil || preview == nil {
		return fmt.Errorf("missing native producer or consumer")
	}
	if requireOutputUUID(fileAction.WFWorkflowActionParameters["WFInput"]) != original {
		return fmt.Errorf("file input lost original producer identity")
	}
	if requireOutputUUID(preview.WFWorkflowActionParameters["WFInput"]) != file {
		return fmt.Errorf("file result was coerced or replaced before Quick Look")
	}
	value, present := fileAction.WFWorkflowActionParameters["WFFileType"]
	if present != explicitType || (explicitType && value != "public.plain-text") {
		return fmt.Errorf("native optional UTI changed: present=%t value=%#v", present, value)
	}
	return nil
}

func TestP15GetTypeDualFrontendPreservesNativeWire(t *testing.T) {
	fileID, _, _ := p15NativeTypeActions(t)
	for _, legacy := range []bool{false, true} {
		for _, explicit := range []bool{false, true} {
			t.Run(fmt.Sprintf("legacy=%t/explicit=%t", legacy, explicit), func(t *testing.T) {
				declaration, argument := "let", ""
				if explicit {
					argument = ", fileType: \"public.plain-text\""
				}
				if legacy {
					declaration = "const"
					if explicit {
						argument = ", \"public.plain-text\""
					}
				}
				src := declaration + " original = text(\"payload\")\n" +
					declaration + " converted = getType(original" + argument + ")\nquicklook(converted)"
				acts := compileDebugFixture(t, src, legacy)
				if err := p15GetTypeWire(acts, fileID, explicit); err != nil {
					t.Fatal(err)
				}
				// A valid-looking attachment to the wrong producer must fail.
				for i := range acts {
					if acts[i].WFWorkflowActionIdentifier == fileID {
						acts[i].WFWorkflowActionParameters["WFInput"] = map[string]any{
							"WFSerializationType": "WFTextTokenAttachment",
							"Value":               map[string]any{"Type": "ActionOutput", "OutputUUID": "wrong-producer"},
						}
						break
					}
				}
				if p15GetTypeWire(acts, fileID, explicit) == nil {
					t.Fatal("wrong-input UUID mutation passed")
				}
			})
		}
	}
}

func TestP15GetTypeInferencePreservesNativeFileCategory(t *testing.T) {
	for _, input := range []string{"ShortcutInput", "[\"one\", \"two\"]"} {
		src := "let converted = getType(" + input + ", fileType: \"public.plain-text\")\n" +
			"let inspected = typeOf(converted)\nlet checked = require(converted, nonEmpty: true)\nquicklook(checked)"
		program, errors := p15ParseProgram(src)
		if len(errors) != 0 {
			t.Fatal(errors)
		}
		analyzer := analysis.NewAnalyzer(schema.DefaultRegistry())
		analyzer.Analyze(program)
		for _, diagnostic := range analyzer.Diagnostics() {
			if diagnostic.Severity == analysis.SeverityError {
				t.Fatal(diagnostic)
			}
		}
		for name, expected := range map[string]types.Type{"converted": types.File, "inspected": types.Text, "checked": types.File} {
			actual := analyzer.ExprTypes()[p15BindingValue(t, program, name)]
			if actual == nil || !actual.Equals(expected) {
				t.Errorf("input=%s binding=%s type=%v want=%v", input, name, actual, expected)
			}
		}
		compileDebugFixture(t, src, false)
	}
}
