package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/migrate"
	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/syntax"
)

func p15C03SingletonReference(actions []ShortcutAction, tc p15C03Case) error {
	if len(actions) != 2 || actions[0].WFWorkflowActionIdentifier != "is.workflow.actions.gettext" ||
		actions[0].WFWorkflowActionParameters["WFTextActionText"] != tc.values[0] ||
		actions[1].WFWorkflowActionIdentifier != "is.workflow.actions."+tc.identifier {
		return fmt.Errorf("single-item graph changed: %#v", actions)
	}
	value := actions[1].WFWorkflowActionParameters[tc.key]
	if tc.shape == "urls" || tc.shape == "list" {
		items := p15C03Array(value)
		if len(items) != 1 {
			return fmt.Errorf("one reference did not stay one source item")
		}
		value = items[0]
		if tc.shape == "list" {
			row := debugParamMap(value)
			if fmt.Sprint(row["WFItemType"]) != "0" {
				return fmt.Errorf("list reference row is not text")
			}
			value = row["WFValue"]
		}
	}
	uuid, err := p15C03TokenUUID(value)
	if err != nil {
		return err
	}
	if uuid != actions[0].WFWorkflowActionParameters["UUID"] {
		return fmt.Errorf("single dynamic item lost its exact producer")
	}
	return nil
}

func TestP15C03SingletonReferencesAndTemplates(t *testing.T) {
	for _, tc := range p15C03Cases {
		for _, mode := range []string{"legacy_reference", "v2_reference", "v2_template"} {
			t.Run(tc.name+"/"+mode, func(t *testing.T) {
				prefix := "let value = text(" + fmt.Sprintf("%q", tc.values[0]) + ")\n"
				value := "value"
				if mode == "v2_template" {
					value = "f\"{value}\""
				}
				args := tc.label + ": [" + value + "]"
				legacy := mode == "legacy_reference"
				if legacy {
					prefix = "const value = text(" + fmt.Sprintf("%q", tc.values[0]) + ")\n"
					args = value
				}
				actions, err := p15C03Actions(prefix+tc.name+"("+args+")", legacy)
				if err != nil {
					t.Fatal(err)
				}
				if err = p15C03SingletonReference(actions, tc); err != nil {
					t.Fatal(err)
				}
				broken := p15C03Clone(t, actions)
				broken[0].WFWorkflowActionParameters["UUID"] = "foreign-producer"
				if p15C03SingletonReference(broken, tc) == nil {
					t.Fatal("reference oracle accepted producer substitution")
				}
			})
		}
	}
}

func TestP15C03TemplateMixturesFailClosed(t *testing.T) {
	bin := roundTripBinary(t)
	for _, name := range []string{"emailAddress", "phoneNumber", "hideAllApps", "quitAllApps", "killAllApps"} {
		var tc p15C03Case
		for _, candidate := range p15C03Cases {
			if candidate.name == name {
				tc = candidate
			}
		}
		for _, first := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/templateFirst=%t", name, first), func(t *testing.T) {
				left, right := fmt.Sprintf("%q", tc.values[0]), "f\"{value}\""
				if first {
					left, right = right, left
				}
				source := "let value = text(" + fmt.Sprintf("%q", tc.values[1]) + ")\n" + name + "(" + tc.label + ": [" + left + ", " + right + "])"
				tmp := t.TempDir()
				input, output := filepath.Join(tmp, "mixed.cherri"), filepath.Join(tmp, "mixed.shortcut")
				if err := os.WriteFile(input, []byte(source), 0600); err != nil {
					t.Fatal(err)
				}
				stdout, stderr, err := p15RunCLI(t, bin, "", nil, "compile", input, "--skip-sign", "-o", output)
				if err == nil {
					t.Fatalf("actual CLI accepted mixed dynamic template: %s %s", stdout, stderr)
				}
				if _, stat := os.Stat(output); !os.IsNotExist(stat) {
					t.Fatalf("invalid template emitted output: %v", stat)
				}
				if !strings.Contains(string(stdout)+string(stderr), "one variable/template") {
					t.Fatalf("wrong capability diagnostic: %s %s", stdout, stderr)
				}
			})
		}
	}
}

func TestP15C03NestedMigrationAndSourceIdentity(t *testing.T) {
	for _, tc := range []struct {
		name, legacy string
		want         []string
	}{
		{"nested_calls", "url(text(\"FIRST\"), text(\"SECOND\"), text(\"THIRD\"))\n", []string{"url(url: [text(\"FIRST\"), text(\"SECOND\"), text(\"THIRD\")])"}},
		{"nested_list_call", "list(text(\"FIRST\"), text(\"SECOND\"))\n", []string{"list(listItem: [text(\"FIRST\"), text(\"SECOND\")])"}},
		{"text_contents", "text(\"list(inside string), url(also inside)\")\n", []string{"text(\"list(inside string), url(also inside)\")"}},
		{"comment_contents", "// list(do not rewrite this)\nurl(\"https://example.test/first\")\n", []string{"// list(do not rewrite this)", "url(url: [\"https://example.test/first\"])"}},
		{"control_body", "if 1 == 1 { url(\"https://example.test/first\", \"https://example.test/second\") }\n", []string{"url(url: [\"https://example.test/first\", \"https://example.test/second\"])"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			migrated, diags, err := migrate.MigrateSourceWithDiagnostics(tc.legacy)
			if err != nil || len(diags) != 0 {
				t.Fatalf("migration: %v %v", err, diags)
			}
			for _, part := range tc.want {
				if !strings.Contains(migrated, part) {
					t.Fatalf("structured migration changed source identity; want %q in %s", part, migrated)
				}
			}
			program, parseErrors := p15ParseProgram(migrated)
			if len(parseErrors) > 0 {
				t.Fatal(parseErrors)
			}
			before := syntax.Format(program)
			analyzer := analysis.NewAnalyzer(schema.DefaultRegistry())
			analyzer.Analyze(program)
			if after := syntax.Format(program); after != before {
				t.Fatalf("analysis mutated the authored pack:\n%s\n%s", before, after)
			}
			actions, err := p15C03Actions(migrated, false)
			if err != nil {
				t.Fatal(err)
			}
			if tc.name == "nested_calls" {
				if err = p15C03EffectOracle(actions); err != nil {
					t.Fatal(err)
				}
			}
			if tc.name == "nested_list_call" {
				if len(actions) != 3 {
					t.Fatalf("nested constructor effects not once-only: %#v", actions)
				}
				items := p15C03Array(actions[2].WFWorkflowActionParameters["WFItems"])
				if len(items) != 2 {
					t.Fatalf("nested effects flattened/dropped")
				}
				for index, item := range items {
					uuid, err := p15C03TokenUUID(debugParamMap(item)["WFValue"])
					if err != nil || uuid != actions[index].WFWorkflowActionParameters["UUID"] {
						t.Fatalf("wrong nested List producer %d: %v", index, err)
					}
				}
			}
			if tc.name == "text_contents" && (len(actions) != 1 || actions[0].WFWorkflowActionParameters["WFTextActionText"] != "list(inside string), url(also inside)") {
				t.Fatalf("text rewritten: %#v", actions)
			}
		})
	}
	for _, source := range []string{
		"function url(_value: Text) -> Text { return _value }\nurl(\"https://example.test/first\")",
		"function list(_value: Text) -> Text { return _value }\nlist(\"first\")",
	} {
		migrated, diags, err := migrate.MigrateSourceWithDiagnostics(source)
		if err != nil || len(diags) > 0 {
			t.Fatalf("%v %v", err, diags)
		}
		if strings.Contains(migrated, "url(url: [") || strings.Contains(migrated, "list(listItem: [") {
			t.Fatalf("user function was mistaken for native variadic: %s", migrated)
		}
		if _, parseErrors := p15ParseProgram(migrated); len(parseErrors) != 0 {
			t.Fatalf("shadow fixture must parse: %v", parseErrors)
		}
		compiled, err := CompileSourceToPlist("c03-shadow-function.cherri", migrated)
		if err != nil || len(compiled) == 0 {
			t.Fatalf("actual shadowed function compile: %v", err)
		}
	}
}

func TestP15C03EmptyIsNotAnEmptyItem(t *testing.T) {
	for _, tc := range p15C03Cases {
		actions, err := p15C03Actions(p15C03Source(tc, 0, true), false)
		if err != nil {
			t.Fatal(err)
		}
		broken := p15C03Clone(t, actions)
		broken[0].WFWorkflowActionParameters[tc.key] = ""
		if p15C03LiteralOracle(broken, tc, 0) == nil {
			t.Fatalf("empty %s pack accepted a scalar field", tc.name)
		}
	}
	tc := p15C03Cases[len(p15C03Cases)-1]
	tc.values = []string{""}
	actions, err := p15C03Actions("url(url: [\"\"])", false)
	if err != nil {
		t.Fatal(err)
	}
	if err = p15C03LiteralOracle(actions, tc, 1); err != nil {
		t.Fatal(err)
	}
	if p15C03LiteralOracle(actions, tc, 0) == nil {
		t.Fatal("single empty string collapsed into zero arguments")
	}
}

func TestP15C03ListTypeEvidence(t *testing.T) {
	for _, source := range []string{
		"let values: List<Text> = list(listItem: [])",
		"let values: List<Text> = list(listItem: [\"first\", text(\"second\")])",
		"let values = list(listItem: [\"first\", \"second\"])\nvar last: Text = \"\"\nfor item in values { last = item }",
	} {
		raw, err := CompileSourceToPlist("c03-known-list.cherri", source)
		if err != nil || len(raw) == 0 {
			t.Fatalf("proven Text item contract lost: %v", err)
		}
	}
	for _, source := range []string{
		"let values: List<Text> = list(listItem: [native.ref(\"Variable\", name: \"external\")])",
		"let values: List<Text> = list(listItem: [CurrentDate])",
	} {
		raw, err := CompileSourceToPlist("c03-opaque-list.cherri", source)
		if err == nil || len(raw) != 0 || !strings.Contains(err.Error(), "E_ARGUMENT_TYPE") {
			t.Fatalf("native compatibility minted Text element proof: bytes=%d err=%v", len(raw), err)
		}
	}
}

func TestP15C03CustomActionShadow(t *testing.T) {
	// Exercise the actual installed CLI boundary for original and migrated custom
	// declarations. Repeated in-process registry isolation is independently
	// covered by p15_registry_lifetime_test.go and its retained causal baseline.
	bin := roundTripBinary(t)
	for _, name := range []string{"url", "list"} {
		t.Run(name, func(t *testing.T) {
			source := "action " + name + "(value: Text) -> Text {\nidentifier: \"com.example.c03." + name + "\"\nprimary: \"value\"\nbindings: {value: {key: \"Value\", codec: \"text\"}}\n}\n" + name + "(\"FIRST\")\n"
			migrated, diags, err := migrate.MigrateSourceWithDiagnostics(source)
			if err != nil || len(diags) != 0 {
				t.Fatalf("migration: %v %v", err, diags)
			}
			_, parseErrors := p15ParseProgram(migrated)
			if len(parseErrors) != 0 {
				t.Fatalf("actual custom declaration route must remain parsed: %v", parseErrors)
			}
			for index, input := range []string{source, migrated} {
				tmp := t.TempDir()
				file, output := filepath.Join(tmp, fmt.Sprintf("shadow-%d.cherri", index)), filepath.Join(tmp, "shadow.shortcut")
				if err = os.WriteFile(file, []byte(input), 0600); err != nil {
					t.Fatal(err)
				}
				stdout, stderr, err := p15RunCLI(t, bin, "", nil, "compile", file, "--skip-sign", "-o", output)
				if err != nil {
					t.Fatalf("custom action was rebound to builtin pack: %v\n%s\n%s", err, stdout, stderr)
				}
				raw, err := os.ReadFile(output)
				if err != nil {
					t.Fatal(err)
				}
				document, err := decodeShortcutRawMap(raw)
				if err != nil {
					t.Fatal(err)
				}
				actions := p15C03Array(document["WFWorkflowActions"])
				if len(actions) != 1 {
					t.Fatalf("custom native action count changed: %d", len(actions))
				}
				action := debugParamMap(actions[0])
				params := debugParamMap(action["WFWorkflowActionParameters"])
				if action["WFWorkflowActionIdentifier"] != "com.example.c03."+name || params["Value"] != "FIRST" {
					t.Fatalf("wrong custom native identity/field: %#v", action)
				}
			}
		})
	}
}

func p15C03SnapshotOracle(actions []ShortcutAction) error {
	capture, mutation, consumer := -1, -1, -1
	stores := 0
	for index, action := range actions {
		params := action.WFWorkflowActionParameters
		switch action.WFWorkflowActionIdentifier {
		case "is.workflow.actions.setvariable":
			if params["WFVariableName"] == "snapshot" {
				stores++
				if stores == 2 {
					mutation = index
				}
			}
		case "is.workflow.actions.getvariable":
			value := debugParamMap(params["WFVariable"])
			if inner := debugParamMap(value["Value"]); inner != nil {
				value = inner
			}
			if value["Type"] == "Variable" && value["VariableName"] == "snapshot" {
				if capture >= 0 {
					return fmt.Errorf("snapshot was evaluated more than once")
				}
				capture = index
			}
		case "is.workflow.actions.url":
			consumer = index
		}
	}
	if stores != 2 || capture < 0 || mutation <= capture || consumer <= mutation {
		return fmt.Errorf("wrong source evaluation boundary: stores=%d capture=%d mutation=%d consumer=%d", stores, capture, mutation, consumer)
	}
	items := p15C03Array(actions[consumer].WFWorkflowActionParameters["WFURLActionURL"])
	if len(items) != 2 {
		return fmt.Errorf("pack did not retain two items")
	}
	uuid, err := p15C03TokenUUID(items[0])
	if err != nil {
		return err
	}
	if uuid != actions[capture].WFWorkflowActionParameters["UUID"] {
		return fmt.Errorf("first element reads a later value")
	}
	return nil
}

func TestP15C03PackElementSnapshot(t *testing.T) {
	source := "var snapshot = \"BEFORE\"\nurl(url: [snapshot, if true { snapshot = \"AFTER\"\n\"TAIL\" } else { \"SKIPPED\" }])\n"
	actions, err := p15C03Actions(source, false)
	if err != nil {
		t.Fatal(err)
	}
	if err = p15C03SnapshotOracle(actions); err != nil {
		t.Fatal(err)
	}
	broken := p15C03Clone(t, actions)
	for _, action := range broken {
		if action.WFWorkflowActionIdentifier == "is.workflow.actions.url" {
			items := p15C03Array(action.WFWorkflowActionParameters["WFURLActionURL"])
			refs := debugParamMap(debugParamMap(debugParamMap(items[0])["Value"])["attachmentsByRange"])
			value := debugParamMap(refs["{0, 1}"])
			value["Type"] = "Variable"
			value["VariableName"] = "snapshot"
			delete(value, "OutputUUID")
		}
	}
	if p15C03SnapshotOracle(broken) == nil {
		t.Fatal("snapshot oracle accepted deferred post-mutation lookup")
	}
}
