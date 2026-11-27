package main

// This additive exporter is the native-runtime handoff for the original T01,
// FN04 and M02 scenarios. It calls their original source factories and complete
// graph oracles. Exporting bytes is not Apple execution or acceptance.
import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/lower"
	"github.com/electrikmilk/cherri/internal/language/migrate"
	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/syntax"
	"howett.net/plist"
)

const p15NativeContractExportSchema = "cherri-native-contract-fixture/1"

type p15NativeContractSlot struct {
	Name          string `json:"name"`
	AuthoredType  string `json:"authoredType"`
	Present       bool   `json:"present"`
	ExpectedValue any    `json:"expectedValue"`
}

type p15NativeContractFixture struct {
	Schema              string                  `json:"schema"`
	Requirement         string                  `json:"requirement"`
	CaseID              string                  `json:"caseId"`
	OriginalTest        string                  `json:"originalTest"`
	OriginalGraphOracle string                  `json:"originalGraphOracle"`
	CompileFilename     string                  `json:"compileFilename"`
	SourceBase64        string                  `json:"sourceBase64"`
	SourceSHA256        string                  `json:"sourceSha256"`
	SourceBytes         int                     `json:"sourceBytes"`
	UnsignedBase64      string                  `json:"unsignedBase64"`
	UnsignedSHA256      string                  `json:"unsignedSha256"`
	UnsignedBytes       int                     `json:"unsignedBytes"`
	ActionCount         int                     `json:"actionCount"`
	ObservationMode     string                  `json:"observationMode"`
	Slots               []p15NativeContractSlot `json:"slots"`
	RequiredEffects     []string                `json:"requiredEffects"`
	NativeExecuted      bool                    `json:"nativeExecuted"`
	FormalAccepted      bool                    `json:"formalAcceptanceGranted"`
}

func p15NativeContractSHA(raw []byte) string {
	return fmt.Sprintf("%x", sha256.Sum256(raw))
}

func p15NativeContractActions(t *testing.T, raw []byte) (map[string]any, []map[string]any) {
	t.Helper()
	var root map[string]any
	if _, err := plist.Unmarshal(raw, &root); err != nil {
		t.Fatal(err)
	}
	items, ok := root["WFWorkflowActions"].([]any)
	if !ok {
		t.Fatal("full native workflow has no action array")
	}
	actions := make([]map[string]any, len(items))
	for i, item := range items {
		action, ok := item.(map[string]any)
		if !ok {
			t.Fatalf("native action %d has no envelope", i)
		}
		actions[i] = action
	}
	return root, actions
}

func p15NativeContractCompile(t *testing.T, filename, source string) ([]byte, map[string]any, []map[string]any) {
	t.Helper()
	raw, err := CompileSourceToPlist(filename, source)
	if err != nil {
		t.Fatal(err)
	}
	root, actions := p15NativeContractActions(t, raw)
	return raw, root, actions
}

func p15NativeContractLiteral(t *testing.T, source string) any {
	t.Helper()
	var value any
	decoder := json.NewDecoder(strings.NewReader(source))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		t.Fatalf("authored fixed literal is not JSON: %v", err)
	}
	return value
}

func p15NativeContractEmit(t *testing.T, requirement, caseID, originalTest, oracle, filename, source, mode string, raw []byte, actions []map[string]any, slots []p15NativeContractSlot, effects []string) {
	t.Helper()
	if slots == nil {
		slots = []p15NativeContractSlot{}
	}
	if effects == nil {
		effects = []string{}
	}
	fixture := p15NativeContractFixture{
		Schema: p15NativeContractExportSchema, Requirement: requirement, CaseID: caseID,
		OriginalTest: originalTest, OriginalGraphOracle: oracle, CompileFilename: filename,
		SourceBase64: base64.StdEncoding.EncodeToString([]byte(source)),
		SourceSHA256: p15NativeContractSHA([]byte(source)), SourceBytes: len([]byte(source)),
		UnsignedBase64: base64.StdEncoding.EncodeToString(raw),
		UnsignedSHA256: p15NativeContractSHA(raw), UnsignedBytes: len(raw),
		ActionCount: len(actions), ObservationMode: mode, Slots: slots, RequiredEffects: effects,
		NativeExecuted: false, FormalAccepted: false,
	}
	encoded, err := json.Marshal(fixture)
	if err != nil {
		t.Fatal(err)
	}
	// The complete original is retained in the actual Go JSONL. Consumers join
	// this line to its running test, final pass, process and immutable source.
	t.Logf("P15_NATIVE_CONTRACT_FIXTURE sha256=%s bytes=%d base64=%s",
		p15NativeContractSHA(encoded), len(encoded), base64.StdEncoding.EncodeToString(encoded))
}

func TestP15NativeContractRuntimeFixtureExport(t *testing.T) {
	// An unenrolled original case must never silently inherit the Void oracle.
	for _, unknown := range []string{"", "__unregistered__", "return_zero_extra"} {
		if _, err := p15NativeContractMainResult(unknown); err == nil {
			t.Fatalf("unenrolled main case %q was accepted", unknown)
		}
	}
	t.Run("T01", func(t *testing.T) {
		for _, tc := range p15TypedDefaultCases() {
			t.Run("typed/"+tc.name, func(t *testing.T) {
				source := p15TypedDefaultSource(tc)
				p15FunctionTypedProgram(t, source, map[string]string{
					"echo(" + tc.supplied + ")": "AnyContent", "echo()": "AnyContent", "echo(value:" + tc.supplied + ")": "AnyContent",
				})
				raw, _, actions := p15NativeContractCompile(t, "p15-short-circuit.cherri", source)
				if _, err := p15TypedDefaultGraph(actions, tc); err != nil {
					t.Fatal(err)
				}
				supplied := p15NativeContractLiteral(t, tc.supplied)
				fallback := p15NativeContractLiteral(t, tc.fallback)
				slots := []p15NativeContractSlot{
					{"supplied", tc.annotation, true, supplied},
					{"omitted", tc.annotation, true, fallback},
					{"named", tc.annotation, true, supplied},
				}
				p15NativeContractEmit(t, "T01", "typed/"+tc.name,
					"TestP15T01TypedDefaultBoundaryGraphs/"+tc.name, "p15TypedDefaultGraph",
					"p15-short-circuit.cherri", source, "caller-slots-before-original-void", raw, actions, slots, nil)
			})
		}
		for _, tc := range p15OptionalMapDefaultCases() {
			for _, mode := range []string{"omitted", "none", "empty"} {
				t.Run("optional/"+tc.name+"/"+mode, func(t *testing.T) {
					source := p15OptionalMapDefaultSource(tc, mode)
					p15FunctionTypedProgram(t, source, map[string]string{})
					raw, _, actions := p15NativeContractCompile(t, "p15-short-circuit.cherri", source)
					if _, err := p15OptionalMapDefaultGraph(actions, tc, mode); err != nil {
						t.Fatal(err)
					}
					value := p15NativeContractLiteral(t, tc.source)
					if mode == "empty" {
						value = map[string]any{}
					} else if mode == "none" {
						value = nil
					}
					p15NativeContractEmit(t, "T01", "optional/"+tc.name+"/"+mode,
						"TestP15T01OptionalMapDefaultGraphs/"+tc.name+"/"+mode, "p15OptionalMapDefaultGraph",
						"p15-short-circuit.cherri", source, "optional-caller-slot-before-original-void", raw, actions,
						[]p15NativeContractSlot{{"result", "Map<Text,AnyContent>?", mode != "none", value}}, nil)
				})
			}
		}
		for _, optional := range []bool{false, true} {
			t.Run(fmt.Sprintf("effects/optional_%t", optional), func(t *testing.T) {
				source := p15MapDefaultEffectSource(optional)
				p15FunctionTypedProgram(t, source, map[string]string{})
				raw, _, actions := p15NativeContractCompile(t, "p15-short-circuit.cherri", source)
				if _, err := p15MapDefaultEffectGraph(actions, optional); err != nil {
					t.Fatal(err)
				}
				annotation := "Map<Text,AnyContent>"
				if optional {
					annotation += "?"
				}
				p15NativeContractEmit(t, "T01", fmt.Sprintf("effects/optional_%t", optional),
					fmt.Sprintf("TestP15T01MapDefaultEffectOrderAndScope/optional_%t", optional), "p15MapDefaultEffectGraph",
					"p15-short-circuit.cherri", source, "caller-slots-and-ordered-default-effects", raw, actions,
					[]p15NativeContractSlot{
						{"supplied", annotation, true, map[string]any{}},
						{"omitted", annotation, true, map[string]any{"seed": "SEED", "first": "FIRST", "second": "SECOND"}},
					}, []string{"omitted:FIRST", "omitted:SECOND"})
			})
		}
	})
	t.Run("FN04", func(t *testing.T) {
		for _, nested := range []bool{false, true} {
			t.Run(fmt.Sprintf("nested_%t", nested), func(t *testing.T) {
				source := p15CallerInputSource(nested)
				callee := "inspect"
				if nested {
					callee = "outer"
				}
				p15FunctionTypedProgram(t, source, map[string]string{callee + `("VALUE")`: "Text"})
				raw, _, actions := p15NativeContractCompile(t, "p15-short-circuit.cherri", source)
				if _, err := p15CallerInputGraph(actions, nested); err != nil {
					t.Fatal(err)
				}
				p15NativeContractEmit(t, "FN04", fmt.Sprintf("nested_%t", nested),
					fmt.Sprintf("TestP15FN04CallerInputCompleteNativeGraphs/nested_%t", nested), "p15CallerInputGraph",
					"p15-short-circuit.cherri", source, "fresh-caller-input-and-original-clipboard", raw, actions,
					[]p15NativeContractSlot{{"callerInput", "Text", true, nil}}, nil)
			})
		}
		t.Run("without_functions", func(t *testing.T) {
			source := "getVariable(system.input)\ngetVariable(ShortcutInput)"
			raw, _, actions := p15NativeContractCompile(t, "p15-short-circuit.cherri", source)
			want := map[string]any{"WFWorkflowActionIdentifier": "is.workflow.actions.getvariable", "WFWorkflowActionParameters": map[string]any{
				"WFVariable": p15FunctionAttachment(map[string]any{"Type": "ExtensionInput"}),
			}}
			if len(actions) != 3 {
				t.Fatal("ordinary caller input action inventory changed")
			}
			if err := p15M02MainVoidBoundaryError(actions); err != nil {
				t.Fatal(err)
			}
			for _, action := range actions[:2] {
				if !reflect.DeepEqual(p15FunctionComparable(action, false), want) {
					t.Fatal("ordinary caller input lost original ExtensionInput")
				}
			}
			p15NativeContractEmit(t, "FN04", "without_functions",
				"TestP15FN04CallerInputWithoutFunctions", "original-two-ExtensionInput-reads-and-main-Void",
				"p15-short-circuit.cherri", source, "two-original-input-reads-before-void", raw, actions,
				[]p15NativeContractSlot{{"system.input", "Text", true, nil}, {"ShortcutInput", "Text", true, nil}}, nil)
		})
	})
	t.Run("M02", func(t *testing.T) {
		for _, tc := range p15M02GraphCases() {
			t.Run("main/"+tc.name, func(t *testing.T) {
				raw, root, actions := p15NativeContractCompile(t, "p15-none-clear.cherri", tc.source)
				if err := p15M02GraphError(actions, tc); err != nil {
					t.Fatal(err)
				}
				if _, present := root["outputMode"]; present {
					t.Fatal("entry-mode compiler metadata escaped to native workflow")
				}
				slot, err := p15NativeContractMainResult(tc.name)
				if err != nil {
					t.Fatal(err)
				}
				p15NativeContractEmit(t, "M02", "main/"+tc.name,
					"TestP15M02CompleteMainGraphs/"+tc.name, "p15M02GraphError",
					"p15-none-clear.cherri", tc.source, "unaltered-target-and-separate-caller", raw, actions,
					[]p15NativeContractSlot{slot}, nil)
			})
		}
		for _, inputActions := range [][]any{
			{},
			{map[string]any{"WFWorkflowActionIdentifier": "is.workflow.actions.gettext", "WFWorkflowActionParameters": map[string]any{
				"WFTextActionText": "M02_NATIVE_RESULT", "UUID": "11111111-1111-4111-8111-111111111111",
			}}},
		} {
			t.Run(fmt.Sprintf("exact_native/actions_%d", len(inputActions)), func(t *testing.T) {
				original := map[string]any{"WFWorkflowName": "M02_NATIVE", "WFWorkflowActions": inputActions,
					"M02UnknownField": map[string]any{"empty": "", "false": false, "zero": 0}}
				data, err := plist.Marshal(original, plist.XMLFormat)
				if err != nil {
					t.Fatal(err)
				}
				source := "native.workflow(native.plist(" + formatCherriDoubleQuotedString(string(data)) + "), exact: true)\n"
				raw, got, actions := p15NativeContractCompile(t, "p15-none-clear.cherri", source)
				var want map[string]any
				if _, err := plist.Unmarshal(data, &want); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(got, want) {
					t.Fatal("complete exact native workflow changed")
				}
				slot := p15NativeContractSlot{"result", "Void", false, nil}
				if len(inputActions) == 1 {
					slot = p15NativeContractSlot{"result", "Text", true, "M02_NATIVE_RESULT"}
				}
				p15NativeContractEmit(t, "M02", fmt.Sprintf("exact_native/actions_%d", len(inputActions)),
					fmt.Sprintf("TestP15M02ExactNativeWorkflowBoundary/actions_%d", len(inputActions)), "whole-original-native-document-equality",
					"p15-none-clear.cherri", source, "unaltered-target-and-separate-caller", raw, actions,
					[]p15NativeContractSlot{slot}, nil)
			})
		}
		for _, legacy := range []string{"", "text(\"M02_MIGRATED\")", "#define name M02_MIGRATED_NAME\ntext(\"M02_MIGRATED\")"} {
			t.Run(fmt.Sprintf("migration/source_%d", len(legacy)), func(t *testing.T) {
				source, diagnostics, err := migrate.MigrateSourceWithDiagnostics(legacy)
				if err != nil || len(diagnostics) != 0 || strings.Count(source, "outputMode: \"legacy\"") != 1 {
					t.Fatal("original entry migration did not preserve explicit legacy mode", err, diagnostics)
				}
				raw, root, actions := p15NativeContractCompile(t, "p15-none-clear.cherri", source)
				g := &p15FunctionGraph{actual: actions, groups: map[string]bool{}}
				slot := p15NativeContractSlot{"result", "Void", false, nil}
				if legacy != "" {
					g.emit("gettext", map[string]any{"WFTextActionText": "M02_MIGRATED"})
					slot = p15NativeContractSlot{"result", "Text", true, "M02_MIGRATED"}
				}
				if err := g.compare(); err != nil {
					t.Fatal(err)
				}
				if _, exists := root["outputMode"]; exists {
					t.Fatal("migration metadata escaped to native workflow")
				}
				if strings.Contains(legacy, "#define name") {
					program := p15M02Parse(t, source)
					header, ok := program.Declarations[0].(*syntax.ShortcutDecl)
					if !ok || header.Name != "M02_MIGRATED_NAME" {
						t.Fatal("migration changed the existing declared name")
					}
					workflow, lowerErr := lower.NewLowerer(schema.DefaultRegistry()).LowerProgram(program)
					if lowerErr != nil || workflow.Name != "M02_MIGRATED_NAME" {
						t.Fatal("checked lowering lost the existing workflow name", lowerErr)
					}
				}
				// Ensure the whole document is serializable without hidden Go values.
				if _, err := json.Marshal(root); err != nil {
					t.Fatal(err)
				}
				p15NativeContractEmit(t, "M02", fmt.Sprintf("migration/source_%d", len(legacy)),
					fmt.Sprintf("TestP15M02MigratedEntryOutput/source_%d", len(legacy)), "original-migration-and-complete-output-graph",
					"p15-none-clear.cherri", source, "unaltered-target-and-separate-caller", raw, actions,
					[]p15NativeContractSlot{slot}, nil)
			})
		}
	})
}

func p15NativeContractMainResult(name string) (p15NativeContractSlot, error) {
	switch name {
	case "both_branches_return":
		return p15NativeContractSlot{"result", "Text", true, "M02_THEN"}, nil
	case "parent_explicit_return_after_child":
		return p15NativeContractSlot{"result", "Text", true, "M02_CHILD"}, nil
	case "legacy_mode_preserves_incidental_result":
		return p15NativeContractSlot{"result", "Text", true, "M02_LEGACY"}, nil
	case "authored_native_output":
		return p15NativeContractSlot{"result", "Text", true, "M02_NATIVE_OUTPUT"}, nil
	case "return_text":
		return p15NativeContractSlot{"result", "Text", true, "M02_RETURNED"}, nil
	case "return_empty_text":
		return p15NativeContractSlot{"result", "Text", true, ""}, nil
	case "return_false":
		return p15NativeContractSlot{"result", "Bool", true, false}, nil
	case "return_zero":
		return p15NativeContractSlot{"result", "Number", true, json.Number("0")}, nil
	case "empty_input_boundary", "unused_text", "unused_number_binding",
		"void_action_after_value", "partial_early_return", "zero_iteration_return",
		"child_return_parent_falls_through", "child_bare_return_parent_continues",
		"raw_nothing_then_resolved_text", "native_word_is_only_text",
		"authored_native_stop", "explicit_mode_clears_incidental_result",
		"return_empty_", "return_empty_none":
		return p15NativeContractSlot{"result", "Void", false, nil}, nil
	default:
		return p15NativeContractSlot{}, fmt.Errorf("unenrolled original M02 case %q", name)
	}
}
