package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/migrate"
	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/syntax"
	"github.com/electrikmilk/cherri/internal/shortcutcompare"
)

type p15C03Case struct {
	name, label, identifier, key, shape string
	values                              []string
}

var p15C03Cases = []p15C03Case{
	{"addToReadingList", "url", "readinglist", "WFURL", "urls", []string{"https://example.test/first", "https://example.test/second", "https://example.test/third"}},
	{"emailAddress", "email", "email", "WFEmailAddress", "emailaddress", []string{"first@example.test", "second@example.test", "third@example.test"}},
	{"hideAllApps", "except", "hide.app", "WFAppsExcept", "apps", []string{"com.example.first", "com.example.second", "com.example.third"}},
	{"killAllApps", "except", "quit.app", "WFAppsExcept", "apps", []string{"com.example.first", "com.example.second", "com.example.third"}},
	{"list", "listItem", "list", "WFItems", "list", []string{"first", "second", "third"}},
	{"phoneNumber", "number", "phonenumber", "WFPhoneNumber", "phonenumber", []string{"+12025550101", "+12025550102", "+12025550103"}},
	{"quitAllApps", "except", "quit.app", "WFAppsExcept", "apps", []string{"com.example.first", "com.example.second", "com.example.third"}},
	{"url", "url", "url", "WFURLActionURL", "urls", []string{"https://example.test/first", "https://example.test/second", "https://example.test/third"}},
}

func p15C03Source(tc p15C03Case, count int, packed bool) string {
	values := make([]string, count)
	for index := range values {
		values[index] = strconv.Quote(tc.values[index])
	}
	args := strings.Join(values, ", ")
	if packed {
		args = tc.label + ": [" + args + "]"
	}
	return tc.name + "(" + args + ")\n"
}

func p15C03Actions(src string, legacy bool) (actions []ShortcutAction, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("compiler panic: %v", recovered)
		}
	}()
	var document map[string]any
	if legacy {
		document, err = compileLegacySourceToRawMapInMemory(src)
	} else {
		var raw []byte
		raw, err = CompileSourceToPlist("p15-variadic.cherri", src)
		if err == nil {
			document, err = decodeShortcutRawMap(raw)
		}
	}
	if err != nil {
		return nil, err
	}
	for _, entry := range p15C03Array(document["WFWorkflowActions"]) {
		action := debugParamMap(entry)
		actions = append(actions, ShortcutAction{
			WFWorkflowActionIdentifier: fmt.Sprint(action["WFWorkflowActionIdentifier"]),
			WFWorkflowActionParameters: debugParamMap(action["WFWorkflowActionParameters"]),
		})
	}
	return actions, nil
}

func p15C03Array(value any) []any {
	if value == nil {
		return nil
	}
	v := reflect.ValueOf(value)
	if v.Kind() != reflect.Slice && v.Kind() != reflect.Array {
		return nil
	}
	out := make([]any, v.Len())
	for i := range out {
		out[i] = v.Index(i).Interface()
	}
	return out
}

func p15C03LiteralOracle(actions []ShortcutAction, tc p15C03Case, count int) error {
	if len(actions) != 1 || actions[0].WFWorkflowActionIdentifier != "is.workflow.actions."+tc.identifier {
		return fmt.Errorf("unexpected producer graph: %#v", actions)
	}
	params := actions[0].WFWorkflowActionParameters
	value, present := params[tc.key]
	if tc.shape == "apps" && count == 0 {
		if present {
			return fmt.Errorf("empty exclusions must remain omitted: %#v", value)
		}
	} else if !present {
		return fmt.Errorf("missing exact native field %s", tc.key)
	}
	var items []any
	arrayValue := value
	if tc.shape == "emailaddress" || tc.shape == "phonenumber" {
		field := debugParamMap(value)
		if field["WFSerializationType"] != "WFContactFieldValue" {
			return fmt.Errorf("contact field was flattened/stringified: %#v", field)
		}
		arrayValue = debugParamMap(field["Value"])["WFContactFieldValues"]
	}
	if !(tc.shape == "apps" && count == 0) {
		if arrayValue == nil || (reflect.TypeOf(arrayValue).Kind() != reflect.Slice && reflect.TypeOf(arrayValue).Kind() != reflect.Array) {
			return fmt.Errorf("native field must remain an array even when empty: %#v", arrayValue)
		}
		items = p15C03Array(arrayValue)
	}
	if len(items) != count {
		return fmt.Errorf("item count %d, want %d: %#v", len(items), count, params)
	}
	for index, item := range items {
		var got any
		switch tc.shape {
		case "urls":
			got = item
		case "apps":
			row := debugParamMap(item)
			got = row["BundleIdentifier"]
			if row["TeamIdentifier"] != "0000000000" {
				return fmt.Errorf("app row metadata changed: %#v", row)
			}
		case "list":
			row := debugParamMap(item)
			got = row["WFValue"]
			if fmt.Sprint(row["WFItemType"]) != "0" {
				return fmt.Errorf("list row type changed: %#v", row)
			}
		case "emailaddress", "phonenumber":
			row := debugParamMap(item)
			kind := "2"
			if tc.shape == "phonenumber" {
				kind = "1"
			}
			if fmt.Sprint(row["EntryType"]) != kind {
				return fmt.Errorf("contact entry kind changed: %#v", row)
			}
			got = debugParamMap(row["SerializedEntry"])["link.contentkit."+tc.shape]
		}
		if got != tc.values[index] {
			return fmt.Errorf("item %d = %#v, want %q", index, got, tc.values[index])
		}
	}
	if tc.shape == "urls" && params["Show-WFURLActionURL"] != true {
		return fmt.Errorf("URL editor visibility flag missing")
	}
	if tc.name == "hideAllApps" && params["WFHideAppMode"] != "All Apps" {
		return fmt.Errorf("hide mode changed")
	}
	if (tc.name == "quitAllApps" || tc.name == "killAllApps") && params["WFQuitAppMode"] != "All Apps" {
		return fmt.Errorf("quit mode changed")
	}
	if tc.name == "killAllApps" && params["WFAskToSaveChanges"] != false {
		return fmt.Errorf("kill policy changed")
	}
	return nil
}

func TestP15C03LegacyWireControl(t *testing.T) {
	for _, tc := range p15C03Cases {
		for _, count := range []int{0, 1, 3} {
			t.Run(fmt.Sprintf("%s/%d", tc.name, count), func(t *testing.T) {
				actions, err := p15C03Actions(p15C03Source(tc, count, false), true)
				if err != nil {
					t.Fatal(err)
				}
				if err := p15C03LiteralOracle(actions, tc, count); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestP15C03NamedListNativeStructure(t *testing.T) {
	for _, tc := range p15C03Cases {
		for _, count := range []int{0, 1, 3} {
			t.Run(fmt.Sprintf("%s/%d", tc.name, count), func(t *testing.T) {
				actions, err := p15C03Actions(p15C03Source(tc, count, true), false)
				if err != nil {
					t.Fatal(err)
				}
				if err := p15C03LiteralOracle(actions, tc, count); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestP15C03MigrationStructure(t *testing.T) {
	expected := map[string]bool{}
	for _, tc := range p15C03Cases {
		expected[tc.name] = true
	}
	for _, action := range schema.DefaultRegistry().AllActions() {
		for _, param := range action.Parameters {
			if param.Variadic && !expected[action.CallableName] {
				t.Fatalf("uncovered canonical variadic %s", action.CallableName)
			}
		}
	}
	for _, tc := range p15C03Cases {
		for _, count := range []int{0, 1, 3} {
			t.Run(fmt.Sprintf("%s/%d", tc.name, count), func(t *testing.T) {
				migrated, diagnostics, err := migrate.MigrateSourceWithDiagnostics(p15C03Source(tc, count, false))
				if err != nil || len(diagnostics) > 0 {
					t.Fatalf("migration: %v %v", err, diagnostics)
				}
				program, errors := p15ParseProgram(migrated)
				if len(errors) > 0 || len(program.Statements) != 1 {
					t.Fatalf("parsed migrated shape: %v %s", errors, migrated)
				}
				statement, ok := program.Statements[0].(*syntax.ExprStmt)
				if !ok {
					t.Fatalf("not expression: %s", migrated)
				}
				call, ok := statement.Expr.(*syntax.CallExpr)
				if !ok || call.PrimaryArg != nil || len(call.NamedArgs) != 1 || call.NamedArgs[0].Label != tc.label {
					t.Fatalf("migration lacks explicit named pack: %s", migrated)
				}
				list, ok := call.NamedArgs[0].Value.(*syntax.ListExpr)
				if !ok || len(list.Elements) != count {
					t.Fatalf("wrong pack cardinality: %s", migrated)
				}
				actions, err := p15C03Actions(migrated, false)
				if err != nil {
					t.Fatal(err)
				}
				if err := p15C03LiteralOracle(actions, tc, count); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func p15C03Clone(t *testing.T, actions []ShortcutAction) []ShortcutAction {
	t.Helper()
	raw, err := json.Marshal(actions)
	if err != nil {
		t.Fatal(err)
	}
	var copy []ShortcutAction
	if err = json.Unmarshal(raw, &copy); err != nil {
		t.Fatal(err)
	}
	return copy
}

func TestP15C03LiteralOracleMutations(t *testing.T) {
	for _, tc := range p15C03Cases {
		t.Run(tc.name, func(t *testing.T) {
			healthy, err := p15C03Actions(p15C03Source(tc, 3, true), false)
			if err != nil {
				t.Fatal(err)
			}
			for _, mutation := range []string{"drop", "reverse", "duplicate", "scalar", "wrong_native"} {
				broken := p15C03Clone(t, healthy)
				params := broken[0].WFWorkflowActionParameters
				var items []any
				if tc.shape == "emailaddress" || tc.shape == "phonenumber" {
					inner := debugParamMap(debugParamMap(params[tc.key])["Value"])
					items = p15C03Array(inner["WFContactFieldValues"])
					switch mutation {
					case "drop":
						items = items[:2]
					case "reverse":
						items[0], items[2] = items[2], items[0]
					case "duplicate":
						items = append(items, items[0])
					case "scalar":
						inner["WFContactFieldValues"] = "stringified"
					}
					if mutation != "scalar" {
						inner["WFContactFieldValues"] = items
					}
				} else {
					items = p15C03Array(params[tc.key])
					switch mutation {
					case "drop":
						items = items[:2]
					case "reverse":
						items[0], items[2] = items[2], items[0]
					case "duplicate":
						items = append(items, items[0])
					case "scalar":
						params[tc.key] = "stringified"
					}
					if mutation != "scalar" {
						params[tc.key] = items
					}
				}
				if mutation == "wrong_native" {
					broken[0].WFWorkflowActionIdentifier = "is.workflow.actions.text"
				}
				if p15C03LiteralOracle(broken, tc, 3) == nil {
					t.Fatalf("oracle accepted %s", mutation)
				}
			}
		})
	}
}

func p15C03TokenUUID(value any) (string, error) {
	token := debugParamMap(value)
	if token["WFSerializationType"] != "WFTextTokenString" {
		return "", fmt.Errorf("not token string: %#v", token)
	}
	inner := debugParamMap(token["Value"])
	if inner["string"] != "\uFFFC" {
		return "", fmt.Errorf("not one complete source reference: %#v", inner)
	}
	attachments := debugParamMap(inner["attachmentsByRange"])
	if len(attachments) != 1 {
		return "", fmt.Errorf("reference count changed: %#v", attachments)
	}
	ref := debugParamMap(attachments["{0, 1}"])
	if ref["Type"] != "ActionOutput" {
		return "", fmt.Errorf("not producer-bound: %#v", ref)
	}
	uuid, ok := ref["OutputUUID"].(string)
	if !ok || uuid == "" {
		return "", fmt.Errorf("missing producer UUID: %#v", ref)
	}
	return uuid, nil
}

func p15C03EffectOracle(actions []ShortcutAction) error {
	if len(actions) != 4 {
		return fmt.Errorf("expected three once-only producers plus URL, got %d", len(actions))
	}
	seen := map[string]bool{}
	for index, want := range []string{"FIRST", "SECOND", "THIRD"} {
		action := actions[index]
		if action.WFWorkflowActionIdentifier != "is.workflow.actions.gettext" || action.WFWorkflowActionParameters["WFTextActionText"] != want {
			return fmt.Errorf("producer %d missing or reordered: %#v", index, action)
		}
		uuid, _ := action.WFWorkflowActionParameters["UUID"].(string)
		if uuid == "" || seen[uuid] {
			return fmt.Errorf("producer identity missing or reused")
		}
		seen[uuid] = true
	}
	consumer := actions[3]
	if consumer.WFWorkflowActionIdentifier != "is.workflow.actions.url" {
		return fmt.Errorf("missing URL consumer")
	}
	items := p15C03Array(consumer.WFWorkflowActionParameters["WFURLActionURL"])
	if len(items) != 3 {
		return fmt.Errorf("packed arguments were flattened or dropped")
	}
	for index, item := range items {
		uuid, err := p15C03TokenUUID(item)
		if err != nil {
			return err
		}
		if uuid != actions[index].WFWorkflowActionParameters["UUID"] {
			return fmt.Errorf("item %d retargeted", index)
		}
	}
	return nil
}

func TestP15C03ElementOrderAndReferences(t *testing.T) {
	legacy := "const first = text(\"FIRST\")\nconst second = text(\"SECOND\")\nconst third = text(\"THIRD\")\nurl(first, second, third)\n"
	migrated, diagnostics, err := migrate.MigrateSourceWithDiagnostics(legacy)
	if err != nil || len(diagnostics) > 0 {
		t.Fatalf("%v %v", err, diagnostics)
	}
	for _, tc := range []struct {
		name, src string
		legacy    bool
	}{
		{"legacy", legacy, true},
		{"migrated", migrated, false},
		{"named", "url(url: [text(\"FIRST\"), text(\"SECOND\"), text(\"THIRD\")])", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			actions, err := p15C03Actions(tc.src, tc.legacy)
			if err != nil {
				t.Fatal(err)
			}
			if err = p15C03EffectOracle(actions); err != nil {
				t.Fatal(err)
			}
			for _, mutation := range []string{"reorder", "drop", "duplicate", "retarget", "stringify"} {
				broken := p15C03Clone(t, actions)
				switch mutation {
				case "reorder":
					broken[0], broken[1] = broken[1], broken[0]
				case "drop":
					broken = broken[1:]
				case "duplicate":
					broken = append(broken, broken[0])
				case "retarget":
					items := p15C03Array(broken[3].WFWorkflowActionParameters["WFURLActionURL"])
					attachments := debugParamMap(debugParamMap(debugParamMap(items[0])["Value"])["attachmentsByRange"])
					debugParamMap(attachments["{0, 1}"])["OutputUUID"] = broken[1].WFWorkflowActionParameters["UUID"]
				case "stringify":
					broken[3].WFWorkflowActionParameters["WFURLActionURL"] = []any{"FIRST", "SECOND", "THIRD"}
				}
				if p15C03EffectOracle(broken) == nil {
					t.Fatalf("effect oracle accepted %s", mutation)
				}
			}
		})
	}
}

func TestP15C03PackBoundaries(t *testing.T) {
	for _, src := range []string{
		"url(url: [[\"nested\"], \"tail\"])",
		"let values = [\"a\", \"b\"]\nurl(url: [values])",
		"url([\"a\", \"b\"])",
		"url(url: [1])",
		"function c03Void() -> Void { nothing() }\nurl(url: [c03Void()])",
		"url(url: [], url: [])",
		"url(\"prefix\", url: [\"tail\"])",
		"url(url: [\"first\"], \"tail\")",
		"url(url: [\"first\"], url: \"tail\")",
		"url(unknown: [\"first\"])",
	} {
		t.Run(src, func(t *testing.T) {
			raw, err := CompileSourceToPlist("p15-c03-negative.cherri", src)
			if err == nil || len(raw) != 0 {
				t.Fatalf("invalid/ambiguous pack accepted: bytes=%d err=%v", len(raw), err)
			}
			if !strings.Contains(err.Error(), "E_ARGUMENT_TYPE") && !strings.Contains(err.Error(), "E_DUPLICATE_ARGUMENT") && !strings.Contains(err.Error(), "E_UNKNOWN_ARGUMENT") {
				t.Fatalf("wrong diagnostic: %v", err)
			}
		})
	}
	// Existing scalar source compatibility remains a single item.
	tc := p15C03Cases[len(p15C03Cases)-1]
	for _, src := range []string{"url(url: \"https://example.test/first\")", "url(\"https://example.test/first\")"} {
		actions, err := p15C03Actions(src, false)
		if err != nil {
			t.Fatal(err)
		}
		if err = p15C03LiteralOracle(actions, tc, 1); err != nil {
			t.Fatal(err)
		}
	}
}

func TestP15C03DynamicContactAndAppBoundary(t *testing.T) {
	bin := roundTripBinary(t)
	for _, name := range []string{"emailAddress", "phoneNumber", "hideAllApps", "quitAllApps", "killAllApps"} {
		var tc p15C03Case
		for _, candidate := range p15C03Cases {
			if candidate.name == name {
				tc = candidate
			}
		}
		for _, legacy := range []bool{true, false} {
			for _, first := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/legacy=%t/dynamicFirst=%t", name, legacy, first), func(t *testing.T) {
					left, right := strconv.Quote(tc.values[0]), "value"
					if first {
						left, right = right, left
					}
					prefix := "let value = text(" + strconv.Quote(tc.values[1]) + ")\n"
					args := tc.label + ": [" + left + ", " + right + "]"
					if legacy {
						prefix = "const value = text(" + strconv.Quote(tc.values[1]) + ")\n"
						args = left + ", " + right
					}
					source := prefix + name + "(" + args + ")"
					if legacy {
						actions, err := p15C03Actions(source, true)
						if err == nil {
							t.Fatalf("unsupported mixed native constructor silently accepted: %#v", actions)
						}
						if !strings.Contains(err.Error(), "one variable") && !strings.Contains(err.Error(), "single variable") {
							t.Fatalf("wrong native capability error: %v", err)
						}
						return
					}
					tmp := t.TempDir()
					input, output := filepath.Join(tmp, "mixed.cherri"), filepath.Join(tmp, "mixed.shortcut")
					if err := os.WriteFile(input, []byte(source), 0600); err != nil {
						t.Fatal(err)
					}
					stdout, stderr, err := p15RunCLI(t, bin, "", nil, "compile", input, "--skip-sign", "-o", output)
					if err == nil {
						t.Fatalf("actual CLI accepted unsupported mixed native constructor: %s %s", stdout, stderr)
					}
					if _, stat := os.Stat(output); !os.IsNotExist(stat) {
						t.Fatalf("invalid constructor emitted output: %v", stat)
					}
					message := string(stdout) + string(stderr)
					if !strings.Contains(message, "one variable") && !strings.Contains(message, "single variable") {
						t.Fatalf("wrong native capability error: %s", message)
					}
				})
			}
		}
	}
}

func TestP15C03OriginalCLIParity(t *testing.T) {
	bin := roundTripBinary(t)
	for _, name := range []string{"arrays", "functions", "math", "menus", "repeats"} {
		t.Run(name, func(t *testing.T) {
			fixture := filepath.Join("tests", name+".cherri")
			original, err := os.ReadFile(fixture)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				after, err := os.ReadFile(fixture)
				if err != nil || !bytes.Equal(after, original) {
					t.Errorf("original fixture changed")
				}
			}()
			migrated, stderr, err := p15RunCLI(t, bin, "", nil, "migrate", fixture)
			if err != nil {
				t.Fatalf("actual migration: %v %s", err, stderr)
			}
			if !bytes.Contains(migrated, []byte("list(listItem: [")) {
				t.Fatalf("actual source did not migrate list constructor: %s", migrated)
			}
			tmp := t.TempDir()
			source := filepath.Join(tmp, name+"_v2.cherri")
			if err = os.WriteFile(source, migrated, 0600); err != nil {
				t.Fatal(err)
			}
			legacyOut, v2Out := filepath.Join(tmp, "legacy.shortcut"), filepath.Join(tmp, "v2.shortcut")
			for _, args := range [][]string{
				{fixture, "--legacy", "--skip-sign", "--load-all-actions", "-o", legacyOut},
				{"compile", source, "--skip-sign", "-o", v2Out},
			} {
				stdout, stderr, err := p15RunCLI(t, bin, "", nil, args...)
				if err != nil {
					t.Fatalf("actual compiler: %v %s %s", err, stdout, stderr)
				}
			}
			comparison, err := shortcutcompare.CompareFilesNormalized(legacyOut, v2Out)
			if err != nil || !comparison.Equal {
				t.Fatalf("actual original differential: %v %+v", err, comparison)
			}
		})
	}
}
