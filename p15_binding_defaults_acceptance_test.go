package main

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/service"
	"howett.net/plist"
)

func p15BindingFixture(t *testing.T, name string) string {
	t.Helper()
	var raw, err = os.ReadFile(filepath.Join("tests", "runtime_poc", name+".cherri"))
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func p15BindingDescriptors(value any, kind string) []map[string]any {
	var found []map[string]any
	var visit func(any)
	visit = func(value any) {
		switch v := value.(type) {
		case map[string]any:
			if v["Type"] == kind {
				found = append(found, v)
			}
			for _, child := range v {
				visit(child)
			}
		case []any:
			for _, child := range v {
				visit(child)
			}
		}
	}
	visit(value)
	return found
}

func p15BindingTokenText(value any) (string, map[string]any, bool) {
	var outer, ok = value.(map[string]any)
	if !ok || outer["WFSerializationType"] != "WFTextTokenString" {
		return "", nil, false
	}
	var body, bodyOK = outer["Value"].(map[string]any)
	if !bodyOK {
		return "", nil, false
	}
	var text, textOK = body["string"].(string)
	var refs, refsOK = body["attachmentsByRange"].(map[string]any)
	return text, refs, textOK && refsOK
}

func p15BindingSnapshotError(actions []map[string]any, kind string) error {
	var positions = map[string]int{}
	for index, action := range actions {
		if id, ok := p15C01Params(action)["UUID"].(string); ok && id != "" {
			if _, duplicate := positions[id]; duplicate {
				return fmt.Errorf("duplicate producer UUID %s", id)
			}
			positions[id] = index
		}
	}
	var captureIndex = -1
	var captureID string
	var captures = 0
	var first, later = -1, -1
	for index, action := range actions {
		var p = p15C01Params(action)
		var refs = p15BindingDescriptors(p["WFVariable"], kind)
		if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.getvariable" && len(refs) == 1 && (kind != "Variable" || refs[0]["VariableName"] == "numCaptureSrc") {
			captures++
			captureIndex = index
			captureID, _ = p["UUID"].(string)
		}
		if kind == "Variable" && action["WFWorkflowActionIdentifier"] == "is.workflow.actions.setvariable" && p["WFVariableName"] == "numCaptureSrc" {
			var inputs = p15BindingDescriptors(p["WFInput"], "ActionOutput")
			if len(inputs) != 1 {
				return fmt.Errorf("variable assignment lost exact Number producer")
			}
			var producerID, _ = inputs[0]["OutputUUID"].(string)
			var at, exists = positions[producerID]
			if !exists || at >= index || actions[at]["WFWorkflowActionIdentifier"] != "is.workflow.actions.number" {
				return fmt.Errorf("assignment has no preceding Number producer")
			}
			var number = p15C01Params(actions[at])["WFNumberActionNumber"]
			switch fmt.Sprint(number) {
			case "1":
				if first != -1 {
					return fmt.Errorf("duplicate initial assignment")
				}
				first = index
			case "2":
				if later != -1 {
					return fmt.Errorf("duplicate later assignment")
				}
				later = index
			default:
				return fmt.Errorf("assignment value changed: %v", number)
			}
		}
		if kind == "Clipboard" && action["WFWorkflowActionIdentifier"] == "is.workflow.actions.setclipboard" {
			var inputs = p15BindingDescriptors(p["WFInput"], "ActionOutput")
			if len(inputs) != 1 {
				return fmt.Errorf("clipboard write lost exact Text producer")
			}
			var producerID, _ = inputs[0]["OutputUUID"].(string)
			var at, exists = positions[producerID]
			if !exists || at >= index {
				return fmt.Errorf("clipboard write has no preceding producer")
			}
			switch p15C01Params(actions[at])["WFTextActionText"] {
			case "clip_initial":
				if first != -1 {
					return fmt.Errorf("duplicate initial clipboard write")
				}
				first = index
			case "clip_overwritten":
				if later != -1 {
					return fmt.Errorf("duplicate later clipboard write")
				}
				later = index
			}
		}
	}
	if captures != 1 || captureID == "" || first < 0 || later < 0 || !(first < captureIndex && captureIndex < later) {
		return fmt.Errorf("%s capture count=%d id=%q order first=%d capture=%d later=%d", kind, captures, captureID, first, captureIndex, later)
	}
	var consumed = 0
	for index, action := range actions {
		var p = p15C01Params(action)
		if kind == "Clipboard" && index != captureIndex && len(p15BindingDescriptors(p, kind)) > 0 {
			return fmt.Errorf("deferred clipboard read outside capture at %d", index)
		}
		var text, refs, ok = p15BindingTokenText(p["WFTextActionText"])
		if !ok || !strings.HasPrefix(text, "CHERRI_BINDING_CAPTURE_OK\n") {
			continue
		}
		if index <= later {
			return fmt.Errorf("consumer precedes later mutation")
		}
		for _, raw := range refs {
			var ref, ok = raw.(map[string]any)
			if !ok {
				return fmt.Errorf("malformed consumer reference")
			}
			if ref["Type"] == "ActionOutput" && ref["OutputUUID"] == captureID {
				consumed++
			}
			if kind == "Variable" && ref["Type"] == "Variable" && ref["VariableName"] == "numCaptureSrc" {
				return fmt.Errorf("consumer rereads mutable variable")
			}
		}
	}
	if consumed != 2 {
		return fmt.Errorf("original %s capture consumed %d times, want exactly twice", kind, consumed)
	}
	return p15BindingTerminalError(actions, kind, captureID, later, positions)
}

func TestP15V01DeclarationSnapshotProduction(t *testing.T) {
	var source = p15BindingFixture(t, "CherriBindingCapturePOC")
	for _, native := range []bool{false, true} {
		t.Run(fmt.Sprint(native), func(t *testing.T) {
			var current = source
			if native {
				current = strings.ReplaceAll(strings.ReplaceAll(current, "= 1\n", "= number(1)\n"), "= 2\n", "= number(2)\n")
			}
			var actions = p15CompiledActions(t, current)
			if err := p15BindingSnapshotError(actions, "Variable"); err != nil {
				t.Fatal(err)
			}
			for _, mutation := range []string{"remove", "duplicate", "after_mutation", "wrong_source", "wrong_identity", "deferred", "wrong_initial"} {
				t.Run(mutation, func(t *testing.T) {
					var broken = p15C01Clone(t, actions)
					var capture = -1
					var uuid string
					for i, action := range broken {
						var refs = p15BindingDescriptors(p15C01Params(action)["WFVariable"], "Variable")
						if len(refs) == 1 && refs[0]["VariableName"] == "numCaptureSrc" {
							capture = i
							uuid, _ = p15C01Params(action)["UUID"].(string)
						}
					}
					switch mutation {
					case "remove":
						broken = append(broken[:capture], broken[capture+1:]...)
					case "duplicate":
						broken = append(broken, broken[capture])
					case "after_mutation":
						var saved = broken[capture]
						broken = append(broken[:capture], broken[capture+1:]...)
						broken = append(broken, saved)
					case "wrong_source":
						p15BindingDescriptors(p15C01Params(broken[capture])["WFVariable"], "Variable")[0]["VariableName"] = "another"
					case "wrong_initial":
						for _, action := range broken {
							if fmt.Sprint(p15C01Params(action)["WFNumberActionNumber"]) == "1" {
								p15C01Params(action)["WFNumberActionNumber"] = 9
							}
						}
					default:
						for _, action := range broken {
							for _, ref := range p15BindingDescriptors(p15C01Params(action)["WFTextActionText"], "ActionOutput") {
								if ref["OutputUUID"] != uuid {
									continue
								}
								if mutation == "wrong_identity" {
									ref["OutputUUID"] = "not-the-original"
								} else {
									ref["Type"] = "Variable"
									ref["VariableName"] = "numCaptureSrc"
									delete(ref, "OutputUUID")
								}
							}
						}
					}
					if p15BindingSnapshotError(broken, "Variable") == nil {
						t.Fatal("corruption escaped oracle")
					}
				})
			}
		})
	}
}

func TestP15V03ClipboardCapturedOnceProduction(t *testing.T) {
	var source = p15BindingFixture(t, "CherriBindingCapturePOC")
	for _, spelling := range []string{"Clipboard", `native.ref("Clipboard")`} {
		t.Run(spelling, func(t *testing.T) {
			var actions = p15CompiledActions(t, strings.ReplaceAll(source, "= Clipboard\n", "= "+spelling+"\n"))
			if err := p15BindingSnapshotError(actions, "Clipboard"); err != nil {
				t.Fatal(err)
			}
			for _, mutation := range []string{"remove", "duplicate", "after_mutation", "wrong_system", "deferred", "wrong_identity"} {
				t.Run(mutation, func(t *testing.T) {
					var broken = p15C01Clone(t, actions)
					var capture = -1
					var uuid string
					for i, action := range broken {
						if len(p15BindingDescriptors(p15C01Params(action)["WFVariable"], "Clipboard")) == 1 {
							capture = i
							uuid, _ = p15C01Params(action)["UUID"].(string)
						}
					}
					switch mutation {
					case "remove":
						broken = append(broken[:capture], broken[capture+1:]...)
					case "duplicate":
						broken = append(broken, broken[capture])
					case "after_mutation":
						var saved = broken[capture]
						broken = append(broken[:capture], broken[capture+1:]...)
						broken = append(broken, saved)
					case "wrong_system":
						p15BindingDescriptors(p15C01Params(broken[capture])["WFVariable"], "Clipboard")[0]["Type"] = "CurrentDate"
					default:
						for _, action := range broken {
							for _, ref := range p15BindingDescriptors(p15C01Params(action)["WFTextActionText"], "ActionOutput") {
								if ref["OutputUUID"] != uuid {
									continue
								}
								if mutation == "wrong_identity" {
									ref["OutputUUID"] = "later-write-producer"
								} else {
									ref["Type"] = "Clipboard"
									delete(ref, "OutputUUID")
									delete(ref, "OutputName")
								}
							}
						}
					}
					if p15BindingSnapshotError(broken, "Clipboard") == nil {
						t.Fatal("corruption escaped oracle")
					}
				})
			}
		})
	}
}

func p15T01PresenceEnvelopeError(fields map[string]any, supplied []bool) error {
	for i, present := range supplied {
		var presence, err = p15LiteralText(fields[fmt.Sprintf("has_arg_%d", i)])
		var want = "0"
		if present {
			want = "1"
		}
		if err != nil || presence != want {
			return fmt.Errorf("slot %d presence=%q, want %s: %v", i, presence, want, err)
		}
		var value, exists = fields[fmt.Sprintf("arg_%d", i)]
		if exists != present {
			return fmt.Errorf("slot %d argument field presence=%v, supplied=%v", i, exists, present)
		}
		if !present {
			continue
		}
		if i < 3 {
			var expected = []any{p15LiteralNumber("0"), p15LiteralNumber("0"), ""}[i]
			var actual, err = p15ReadDictionaryRow(map[string]any{"WFValue": value, "WFItemType": []int{3, 3, 0}[i]})
			if err != nil || !reflect.DeepEqual(actual, expected) {
				return fmt.Errorf("slot %d scalar value=%#v, want %#v: %v", i, actual, expected, err)
			}
			continue
		}
		// This characterizes the existing shared raw-parameter empty encoding.
		// It proves field presence/type, not that Apple executes the empty value.
		// The separate signed fixture must establish both container counts.
		var expected any = map[string]any{"WFSerializationType": "WFArrayParameterState"}
		if i == 4 {
			expected = map[string]any{"WFSerializationType": "WFDictionaryFieldValue", "Value": map[string]any{}}
		}
		if !reflect.DeepEqual(value, expected) {
			return fmt.Errorf("slot %d current empty container representation changed: %#v", i, value)
		}
	}
	return nil
}

func TestP15T01FunctionPresenceEnvelope(t *testing.T) {
	var prefix = "function t01(flag: Bool = true, num: Number = 9, txt: Text = \"def\", items: List<Text> = [\"default\"], object: Map<Text, Text> = {\"default\": \"present\"}) -> Text { return txt }\n"
	var cases = []struct {
		name, call string
		supplied   []bool
	}{
		{"explicit", "t01(false,num:0,txt:\"\",items:[],object:{})", []bool{true, true, true, true, true}},
		{"omitted", "t01()", []bool{false, false, false, false, false}},
		{"mixed", "t01(items:[],flag:false)", []bool{true, false, false, true, false}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var actions = p15CompiledActions(t, prefix+tc.call)
			var fields = p15C01CallFields(t, actions, false)
			if err := p15T01PresenceEnvelopeError(fields, tc.supplied); err != nil {
				t.Fatal(err)
			}
			for i, supplied := range tc.supplied {
				t.Run(fmt.Sprintf("presence_%d", i), func(t *testing.T) {
					var mutated = p15C01CallFields(t, p15C01Clone(t, actions), false)
					mutated[fmt.Sprintf("has_arg_%d", i)] = map[string]any{"WFSerializationType": "WFTextTokenString", "Value": map[string]any{"string": "unexpected"}}
					if p15T01PresenceEnvelopeError(mutated, tc.supplied) == nil {
						t.Fatal("wrong presence marker accepted")
					}
				})
				t.Run(fmt.Sprintf("argument_%d", i), func(t *testing.T) {
					var mutated = p15C01CallFields(t, p15C01Clone(t, actions), false)
					var key = fmt.Sprintf("arg_%d", i)
					if supplied {
						delete(mutated, key)
					} else {
						mutated[key] = map[string]any{"WFSerializationType": "WFTextTokenString", "Value": map[string]any{"string": "invented"}}
					}
					if p15T01PresenceEnvelopeError(mutated, tc.supplied) == nil {
						t.Fatal("supplied/omitted argument confusion accepted")
					}
				})
				if supplied {
					t.Run(fmt.Sprintf("wrong_value_%d", i), func(t *testing.T) {
						var mutated = p15C01CallFields(t, p15C01Clone(t, actions), false)
						mutated[fmt.Sprintf("arg_%d", i)] = map[string]any{"WFSerializationType": "WFTextTokenString", "Value": map[string]any{"string": "DEFAULT_MUST_NOT_REPLACE_EXPLICIT"}}
						if p15T01PresenceEnvelopeError(mutated, tc.supplied) == nil {
							t.Fatal("default replaced explicit value")
						}
					})
				}
			}
		})
	}
}

func TestP15BindingDefaultsPublicRoutes(t *testing.T) {
	var binary = p15AcceptanceCLI(t)
	for _, name := range []string{"CherriBindingCapturePOC", "CherriTypedDefaultsPOC"} {
		t.Run(name, func(t *testing.T) {
			var source = p15BindingFixture(t, name)
			var svc = service.NewService(nil)
			var uri = "file:///" + name + ".cherri"
			svc.OpenDocument(uri, 1, source)
			var diagnostics, _ = svc.Analyze(uri)
			for _, d := range diagnostics {
				if d.Severity == analysis.SeverityError {
					t.Fatal(d)
				}
			}
			var mobile = compileForMobileMode(source, name, "v2", false)
			if !mobile.OK {
				t.Fatal(mobile.Error)
			}
			var mobileRaw, err = base64.StdEncoding.DecodeString(mobile.PlistBase64)
			if err != nil {
				t.Fatal(err)
			}
			var dir = t.TempDir()
			var input, output = filepath.Join(dir, name+".cherri"), filepath.Join(dir, name+".shortcut")
			if err := os.WriteFile(input, []byte(source), 0600); err != nil {
				t.Fatal(err)
			}
			var stdout, stderr, checkErr = p15RunCLI(t, binary, dir, nil, "check", input, "--json")
			if checkErr != nil || p15DecodeOneJSON(t, stdout).(map[string]any)["valid"] != true {
				t.Fatalf("CLI check %v %s", checkErr, stderr)
			}
			stdout, stderr, err = p15RunCLI(t, binary, dir, nil, "compile", input, "--skip-sign", "--output="+output)
			if err != nil {
				t.Fatalf("CLI compile %v %s %s", err, stdout, stderr)
			}
			var cliRaw, readErr = os.ReadFile(output)
			if readErr != nil {
				t.Fatal(readErr)
			}
			for route, raw := range map[string][]byte{"mobile": mobileRaw, "cli": cliRaw} {
				var doc map[string]any
				if _, err := plist.Unmarshal(raw, &doc); err != nil {
					t.Fatal(err)
				}
				var rawActions, ok = doc["WFWorkflowActions"].([]any)
				if !ok || len(rawActions) == 0 {
					t.Fatalf("%s lost native actions", route)
				}
				var actions = make([]map[string]any, len(rawActions))
				for i, rawAction := range rawActions {
					actions[i] = rawAction.(map[string]any)
				}
				if name == "CherriTypedDefaultsPOC" {
					if err := p15T01DefaultGuardError(actions); err != nil {
						t.Fatalf("%s: %v", route, err)
					}
					var payloads = p15T01PayloadFields(t, actions, "typedDefaults")
					var supplied = [][]bool{{true, true, true, true, true}, {false, false, false, false, false}, {true, false, false, true, false}}
					if len(payloads) != 3 {
						t.Fatalf("%s payload count %d", route, len(payloads))
					}
					for i, payload := range payloads {
						if err := p15T01PresenceEnvelopeError(payload, supplied[i]); err != nil {
							t.Fatalf("%s call %d: %v", route, i, err)
						}
					}
				}
				if name == "CherriBindingCapturePOC" {
					for _, kind := range []string{"Variable", "Clipboard"} {
						if err := p15BindingSnapshotError(actions, kind); err != nil {
							t.Fatalf("%s %s: %v", route, kind, err)
						}
					}
				}
			}
		})
	}
}

func p15T01PayloadFields(t *testing.T, actions []map[string]any, function string) []map[string]any {
	t.Helper()
	var payloads []map[string]any
	for _, action := range actions {
		if action["WFWorkflowActionIdentifier"] != "is.workflow.actions.dictionary" {
			continue
		}
		var rows = p15LiteralDictionaryFields(t, p15C01Params(action)["WFItems"])
		if rows["_cherri_magic"] == nil {
			continue
		}
		var name, err = p15LiteralText(rows["function"]["WFValue"])
		if err != nil || name != function {
			t.Fatalf("unexpected dispatch target %q %v", name, err)
		}
		var fields = map[string]any{}
		for key, row := range rows {
			fields[key] = row["WFValue"]
		}
		payloads = append(payloads, fields)
	}
	return payloads
}

func p15T01DefaultGuardError(actions []map[string]any) error {
	type guard struct{ slot, start, alternate, end int }
	var getters = map[string]int{}
	var groups = map[string]*guard{}
	var bySlot = map[int]*guard{}
	for index, action := range actions {
		var p = p15C01Params(action)
		if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.getvalueforkey" {
			var key, _ = p["WFDictionaryKey"].(string)
			for slot := 0; slot < 5; slot++ {
				if key == fmt.Sprintf("has_arg_%d", slot) {
					var id, _ = p["UUID"].(string)
					if id == "" {
						return fmt.Errorf("presence getter lost identity")
					}
					getters[id] = slot
				}
			}
		}
		if action["WFWorkflowActionIdentifier"] != "is.workflow.actions.conditional" {
			continue
		}
		var group, _ = p["GroupingIdentifier"].(string)
		switch p15Int(p["WFControlFlowMode"]) {
		case 0:
			var refs = p15BindingDescriptors(p["WFInput"], "ActionOutput")
			if len(refs) != 1 {
				continue
			}
			var id, _ = refs[0]["OutputUUID"].(string)
			var slot, found = getters[id]
			if !found {
				continue
			}
			if p15Int(p["WFCondition"]) != 4 || p["WFConditionalActionString"] != "1" {
				return fmt.Errorf("slot %d guard tests value truthiness or wrong presence", slot)
			}
			if bySlot[slot] != nil || group == "" {
				return fmt.Errorf("ambiguous presence guard")
			}
			var g = &guard{slot: slot, start: index, alternate: -1, end: -1}
			groups[group] = g
			bySlot[slot] = g
		case 1:
			if g := groups[group]; g != nil {
				if g.alternate != -1 {
					return fmt.Errorf("duplicate presence else")
				}
				g.alternate = index
			}
		case 2:
			if g := groups[group]; g != nil {
				if g.end != -1 {
					return fmt.Errorf("duplicate presence end")
				}
				g.end = index
			}
		}
	}
	if len(getters) != 5 || len(bySlot) != 5 {
		return fmt.Errorf("five independent presence guards missing: getters=%d guards=%d", len(getters), len(bySlot))
	}
	for slot := 0; slot < 5; slot++ {
		var g = bySlot[slot]
		if !(g.start < g.alternate && g.alternate < g.end) {
			return fmt.Errorf("slot %d invalid branch boundaries", slot)
		}
		var supplied = 0
		for i := g.start + 1; i < g.alternate; i++ {
			if actions[i]["WFWorkflowActionIdentifier"] == "is.workflow.actions.getvalueforkey" && p15C01Params(actions[i])["WFDictionaryKey"] == fmt.Sprintf("arg_%d", slot) {
				supplied++
			}
		}
		if supplied != 1 {
			return fmt.Errorf("slot %d supplied getter count %d", slot, supplied)
		}
		var defaults = 0
		for i := g.alternate + 1; i < g.end; i++ {
			var action = actions[i]
			var p = p15C01Params(action)
			switch slot {
			case 0, 1:
				var want = []string{"1", "9"}[slot]
				if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.number" && fmt.Sprint(p["WFNumberActionNumber"]) == want {
					defaults++
				}
			case 2:
				if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.gettext" && p["WFTextActionText"] == "def" {
					defaults++
				}
			case 3:
				if action["WFWorkflowActionIdentifier"] != "is.workflow.actions.list" {
					continue
				}
				var rows, ok = p["WFItems"].([]any)
				if !ok || len(rows) != 1 {
					return fmt.Errorf("List default cardinality changed")
				}
				var row, rowOK = rows[0].(map[string]any)
				if !rowOK {
					return fmt.Errorf("List default row malformed")
				}
				var value, err = p15ReadDictionaryRow(row)
				if err != nil || value != "default" {
					return fmt.Errorf("List default value changed: %#v %v", value, err)
				}
				defaults++
			case 4:
				if action["WFWorkflowActionIdentifier"] != "is.workflow.actions.dictionary" {
					continue
				}
				var value, err = p15ReadDictionaryLiteral(p["WFItems"])
				if err != nil || !reflect.DeepEqual(value, map[string]any{"default": "present"}) {
					return fmt.Errorf("Map default changed: %#v %v", value, err)
				}
				defaults++
			}
		}
		if defaults != 1 {
			return fmt.Errorf("slot %d default producer count %d", slot, defaults)
		}
	}
	return nil
}

func TestP15T01DefaultsGuardedByPresence(t *testing.T) {
	var actions = p15CompiledActions(t, p15BindingFixture(t, "CherriTypedDefaultsPOC"))
	if err := p15T01DefaultGuardError(actions); err != nil {
		t.Fatal(err)
	}
	var payloads = p15T01PayloadFields(t, actions, "typedDefaults")
	var supplied = [][]bool{{true, true, true, true, true}, {false, false, false, false, false}, {true, false, false, true, false}}
	if len(payloads) != len(supplied) {
		t.Fatalf("function call count %d", len(payloads))
	}
	for i, payload := range payloads {
		if err := p15T01PresenceEnvelopeError(payload, supplied[i]); err != nil {
			t.Fatal(i, err)
		}
	}
	for slot := 0; slot < 5; slot++ {
		t.Run(fmt.Sprintf("guard_%d", slot), func(t *testing.T) {
			var broken = p15C01Clone(t, actions)
			var getter string
			for _, action := range broken {
				var p = p15C01Params(action)
				if p["WFDictionaryKey"] == fmt.Sprintf("has_arg_%d", slot) {
					getter, _ = p["UUID"].(string)
				}
			}
			for _, action := range broken {
				var p = p15C01Params(action)
				if action["WFWorkflowActionIdentifier"] != "is.workflow.actions.conditional" || p15Int(p["WFControlFlowMode"]) != 0 {
					continue
				}
				var refs = p15BindingDescriptors(p["WFInput"], "ActionOutput")
				if len(refs) == 1 && refs[0]["OutputUUID"] == getter {
					p["WFConditionalActionString"] = "0"
				}
			}
			if p15T01DefaultGuardError(broken) == nil {
				t.Fatal("flipped presence branch accepted")
			}
		})
	}
}
