package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/electrikmilk/cherri/internal/language/source"
	"github.com/electrikmilk/cherri/internal/language/syntax"
	"github.com/electrikmilk/cherri/internal/shortcutcompare"
	"howett.net/plist"
)

const p15BRG19Before = "BRG19_BEFORE"
const p15BRG19After = "BRG19_AFTER"

func p15BRG19LegacyProgram() (string, string) {
	reference := base64.StdEncoding.EncodeToString([]byte(`{"fileLocation":{"fileProviderDomainID":"com.apple.files","relativeSubpath":"brg19-public-folder"}}`))
	return `#define name BRG19_WORKFLOW
#define glyph apple
#define color red
#define inputs text
#define outputs text
#ref publicFolder ` + reference + `
#include 'helper.cherri'
#include 'actions/documents'
/*
#define color blue
*/
@payload = "BRG19_PAYLOAD"
saveFile("brg19-public.txt", @payload, false, publicFolder)
comment("BRG19_BEFORE")
show(@payload)
`, "comment(\"BRG19_HELPER_KEEP\")\n"
}

func p15BRG19Write(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
}

func p15BRG19Read(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func p15BRG19Decode(t *testing.T, data []byte) map[string]any {
	t.Helper()
	var document map[string]any
	if _, err := plist.Unmarshal(data, &document); err != nil {
		t.Fatal(err)
	}
	if _, okay := document["WFWorkflowActions"].([]any); !okay {
		t.Fatal("actual native document has no action array")
	}
	return document
}

func p15BRG19Encode(t *testing.T, document map[string]any, format int) []byte {
	t.Helper()
	data, err := plist.Marshal(document, format)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func p15BRG19Parse(text string) (*syntax.Program, error) {
	parser := syntax.NewParser(source.NewFile("brg19", "brg19.cherri", 1, text))
	program := parser.ParseProgram()
	if len(parser.Errors()) != 0 {
		return nil, fmt.Errorf("public output is not canonical V2: %v", parser.Errors())
	}
	return program, nil
}

func p15BRG19Structure(program *syntax.Program) error {
	var shortcut *syntax.ShortcutDecl
	for _, declaration := range program.Declarations {
		if value, okay := declaration.(*syntax.ShortcutDecl); okay {
			if shortcut != nil {
				return fmt.Errorf("duplicate shortcut metadata")
			}
			shortcut = value
		}
	}
	if shortcut == nil || shortcut.Name != "BRG19_WORKFLOW" {
		return fmt.Errorf("metadata did not survive structural migration")
	}
	helper, editTarget, fileReference := 0, 0, 0
	for _, statement := range program.Statements {
		expression, okay := statement.(*syntax.ExprStmt)
		if !okay {
			continue
		}
		call, okay := expression.Expr.(*syntax.CallExpr)
		if !okay {
			continue
		}
		callee, okay := call.Callee.(*syntax.IdentExpr)
		if !okay {
			continue
		}
		if callee.Name == "comment" {
			literal, okay := call.PrimaryArg.(*syntax.LiteralExpr)
			if okay && literal.Value == "BRG19_HELPER_KEEP" {
				helper++
			}
			if okay && literal.Value == p15BRG19Before {
				editTarget++
			}
		}
		if callee.Name == "saveFile" {
			for _, argument := range call.NamedArgs {
				if argument.Label != "folder" {
					continue
				}
				ref, okay := argument.Value.(*syntax.CallExpr)
				if !okay {
					continue
				}
				member, okay := ref.Callee.(*syntax.MemberExpr)
				if !okay || member.Property != "plist" {
					continue
				}
				namespace, okay := member.Target.(*syntax.IdentExpr)
				if !okay || namespace.Name != "native" {
					continue
				}
				literal, okay := ref.PrimaryArg.(*syntax.LiteralExpr)
				if !okay {
					continue
				}
				var payload map[string]any
				if json.Unmarshal([]byte(literal.Value), &payload) != nil {
					continue
				}
				want := map[string]any{"fileLocation": map[string]any{"fileProviderDomainID": "com.apple.files", "relativeSubpath": "brg19-public-folder"}}
				if reflect.DeepEqual(payload, want) {
					fileReference++
				}
			}
		}
	}
	if helper != 1 || editTarget != 1 || fileReference != 1 {
		return fmt.Errorf("include, edit target or reference changed: helper=%d target=%d reference=%d", helper, editTarget, fileReference)
	}
	return nil
}

type p15BRG19ScenarioResult struct {
	directory, originalPath, candidatePath, policyPath string
	original, candidate, migrated, imported            []byte
	policy                                             shortcutcompare.EditPolicy
	target, sibling                                    int
	originalFiles                                      map[string][]byte
}

func p15BRG19RunScenario(t *testing.T, binary string, format int) p15BRG19ScenarioResult {
	t.Helper()
	directory := t.TempDir()
	legacy, helper := p15BRG19LegacyProgram()
	entry := filepath.Join(directory, "legacy.cherri")
	helperPath := filepath.Join(directory, "helper.cherri")
	p15BRG19Write(t, entry, []byte(legacy))
	p15BRG19Write(t, helperPath, []byte(helper))
	legacyNative := filepath.Join(directory, "legacy.shortcut")
	out, stderr, err := p15RunCLI(t, binary, directory, nil, entry, "--legacy", "--skip-sign", "--output="+legacyNative)
	if err != nil {
		t.Fatalf("BRG19 Legacy scenario invalid: %v\n%s\n%s", err, out, stderr)
	}
	converted, stderr, err := p15RunCLI(t, binary, directory, nil, "migrate", entry)
	if err != nil {
		t.Fatalf("actual migration failed: %v\n%s\n%s", err, converted, stderr)
	}
	if !bytes.Equal(p15BRG19Read(t, entry), []byte(legacy)) || !bytes.Equal(p15BRG19Read(t, helperPath), []byte(helper)) {
		t.Fatal("migration preview changed original source/include")
	}
	program, err := p15BRG19Parse(string(converted))
	if err != nil {
		t.Fatal(err)
	}
	if err := p15BRG19Structure(program); err != nil {
		t.Fatal(err, string(converted))
	}
	v2Source := filepath.Join(directory, "converted.cherri")
	v2Native := filepath.Join(directory, "converted.shortcut")
	p15BRG19Write(t, v2Source, converted)
	out, stderr, err = p15RunCLI(t, binary, directory, nil, "compile", v2Source, "--skip-sign", "--output="+v2Native)
	if err != nil {
		t.Fatalf("public migrated V2 compile failed: %v\n%s\n%s", err, out, stderr)
	}
	comparison, err := shortcutcompare.CompareFilesNormalized(legacyNative, v2Native)
	if err != nil || !comparison.Equal {
		t.Fatalf("migration did not preserve original native program: %v %+v", err, comparison)
	}
	forbidden := filepath.Join(directory, "implicit-legacy.shortcut")
	out, stderr, err = p15RunCLI(t, binary, directory, nil, "compile", entry, "--skip-sign", "--output="+forbidden)
	if err == nil || !strings.Contains(string(out)+string(stderr), "E_LEGACY_SYNTAX") {
		t.Fatalf("default public compiler silently retried Legacy: %v %s %s", err, out, stderr)
	}
	if _, err := os.Stat(forbidden); !os.IsNotExist(err) {
		t.Fatalf("rejected Legacy source emitted an artifact: %v", err)
	}
	unsupported := filepath.Join(directory, "unsupported.cherri")
	p15BRG19Write(t, unsupported, []byte("#brg19Unsupported unsafe\n"+legacy))
	beforeFailure := p15DirectoryDigest(t, directory)
	for _, mode := range []string{"preview", "write"} {
		arguments := []string{"migrate", unsupported}
		if mode == "write" {
			arguments = append(arguments, "--write")
		}
		out, stderr, err = p15RunCLI(t, binary, directory, nil, arguments...)
		if err == nil || len(out) != 0 || !strings.Contains(string(stderr), "Migration") {
			t.Fatalf("unsupported conversion %s was accepted: %v %s %s", mode, err, out, stderr)
		}
		if after := p15DirectoryDigest(t, directory); !reflect.DeepEqual(beforeFailure, after) {
			t.Fatalf("failed %s conversion modified original source/include/native output or left a temporary artifact", mode)
		}
	}
	restored, stderr, err := p15RunCLI(t, binary, directory, nil, "migrate", entry)
	if err != nil || !bytes.Equal(restored, converted) {
		t.Fatalf("failed conversion contaminated a later healthy migration: %v %s", err, stderr)
	}

	document := p15BRG19Decode(t, p15BRG19Read(t, v2Native))
	document["BRG19Opaque"] = map[string]any{
		"data": []byte{0, 1, 2, 255}, "large": uint64(9007199254740993),
		"date": time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC), "flag": false,
		"UUID": "brg19-ordinary-opaque-id", "GroupingIdentifier": "brg19-ordinary-group",
	}
	original := p15BRG19Encode(t, document, format)
	originalPath := filepath.Join(directory, "import-original.shortcut")
	p15BRG19Write(t, originalPath, original)
	originalDocument := p15BRG19Decode(t, original)
	actions := originalDocument["WFWorkflowActions"].([]any)
	target, sibling := -1, -1
	for index, raw := range actions {
		action := raw.(map[string]any)
		if action["WFWorkflowActionIdentifier"] != "is.workflow.actions.comment" {
			continue
		}
		text := action["WFWorkflowActionParameters"].(map[string]any)["WFCommentActionText"]
		if text == p15BRG19Before {
			if target != -1 {
				t.Fatal("native edit target is not unique")
			}
			target = index
		}
		if text == "BRG19_HELPER_KEEP" {
			sibling = index
		}
	}
	if target < 0 || sibling < 0 || target == sibling {
		t.Fatal("native original lost the target or included sibling")
	}
	importedPath := filepath.Join(directory, "imported.cherri")
	planPath := filepath.Join(directory, "import-plan.json")
	out, stderr, err = p15RunCLI(t, binary, directory, nil, "import", originalPath, "--fidelity=hybrid", "--output="+importedPath, "--plan-json="+planPath)
	if err != nil {
		t.Fatalf("actual public hybrid V2 import failed: %v %s %s", err, out, stderr)
	}
	imported := p15BRG19Read(t, importedPath)
	importedProgram, err := p15BRG19Parse(string(imported))
	if err != nil {
		t.Fatal(err)
	}
	var plan HybridImportPlan
	if err := json.Unmarshal(p15BRG19Read(t, planPath), &plan); err != nil || len(plan.Decisions) != len(actions) || plan.Decisions[target].Classification != "NATURAL_VERIFIED" {
		t.Fatalf("authorized action is not a verified editable natural import: %v %+v", err, plan)
	}
	roundTripPath := filepath.Join(directory, "unmodified.shortcut")
	out, stderr, err = p15RunCLI(t, binary, directory, nil, "compile", importedPath, "--skip-sign", "--output="+roundTripPath)
	if err != nil {
		t.Fatalf("public imported V2 compile failed: %v %s %s", err, out, stderr)
	}
	if exact := shortcutcompare.CompareDocumentsExact(originalDocument, p15BRG19Decode(t, p15BRG19Read(t, roundTripPath))); !exact.Equal {
		t.Fatalf("unedited native roundtrip changed data/types/identities: %+v", exact)
	}
	var editable *syntax.LiteralExpr
	for _, statement := range importedProgram.Statements {
		expression, okay := statement.(*syntax.ExprStmt)
		if !okay {
			continue
		}
		call, okay := expression.Expr.(*syntax.CallExpr)
		if !okay {
			continue
		}
		callee, okay := call.Callee.(*syntax.IdentExpr)
		if !okay || callee.Name != "comment" {
			continue
		}
		literal, okay := call.PrimaryArg.(*syntax.LiteralExpr)
		if okay && literal.Value == p15BRG19Before {
			if editable != nil {
				t.Fatal("more than one editable source target")
			}
			editable = literal
		}
	}
	if editable == nil {
		t.Fatal("public hybrid import did not expose the authorized natural parameter", string(imported))
	}
	start, end := editable.Span.Start.ByteOffset, editable.Span.End.ByteOffset
	if start < 0 || end <= start || end > len(imported) || string(imported[start:end]) != strconv.Quote(p15BRG19Before) {
		t.Fatal("natural parameter AST span does not identify its exact literal")
	}
	// Edit only the parser-observed source span; never replace the same bytes in
	// the exact native header/provenance payload or another action.
	edited := append([]byte(nil), imported[:start]...)
	edited = append(edited, strconv.Quote(p15BRG19After)...)
	edited = append(edited, imported[end:]...)
	editedPath := filepath.Join(directory, "edited.cherri")
	p15BRG19Write(t, editedPath, edited)
	candidatePath := filepath.Join(directory, "edited.shortcut")
	out, stderr, err = p15RunCLI(t, binary, directory, nil, "compile", editedPath, "--skip-sign", "--output="+candidatePath)
	if err != nil {
		t.Fatalf("authorized source edit did not compile through default V2: %v %s %s", err, out, stderr)
	}
	candidate := p15BRG19Read(t, candidatePath)
	expected := p15BRG19Decode(t, original)
	expected["WFWorkflowActions"].([]any)[target].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)["WFCommentActionText"] = p15BRG19After
	if exact := shortcutcompare.CompareDocumentsExact(expected, p15BRG19Decode(t, candidate)); !exact.Equal {
		t.Fatalf("authorized source edit caused collateral native changes: %+v", exact)
	}
	policy := shortcutcompare.EditPolicy{SchemaVersion: 1, OriginalSHA256: fmt.Sprintf("%x", sha256.Sum256(original)), Changes: []shortcutcompare.EditChange{{
		Path: []string{"WFWorkflowActions", strconv.Itoa(target), "WFWorkflowActionParameters", "WFCommentActionText"}, Before: json.RawMessage(strconv.Quote(p15BRG19Before)), After: json.RawMessage(strconv.Quote(p15BRG19After)),
	}}}
	policyBytes, err := json.MarshalIndent(policy, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	policyPath := filepath.Join(directory, "authorized-edit.json")
	p15BRG19Write(t, policyPath, policyBytes)
	verification, err := shortcutcompare.VerifyEditFiles(originalPath, candidatePath, policyPath)
	if err != nil || !verification.OK || verification.AllowedChanges != 1 || verification.OriginalSHA256 != policy.OriginalSHA256 {
		t.Fatalf("actual exact native edit authorization failed: %v %+v", err, verification)
	}
	result := p15BRG19ScenarioResult{directory: directory, originalPath: originalPath, candidatePath: candidatePath, policyPath: policyPath, original: original, candidate: candidate, migrated: converted, imported: imported, policy: policy, target: target, sibling: sibling,
		originalFiles: map[string][]byte{entry: []byte(legacy), helperPath: []byte(helper), legacyNative: p15BRG19Read(t, legacyNative), v2Source: converted, v2Native: p15BRG19Read(t, v2Native), originalPath: original, importedPath: imported},
	}
	result.assertOriginals(t)
	receipt := map[string]any{"format": format, "target_index": target, "sibling_index": sibling, "policy": policy, "default_v2": true, "unsupported_preview_and_write_rejected": true, "legacy_migration_native_equal": true, "unmodified_native_exact": true, "single_edit_native_exact": true}
	for name, data := range map[string][]byte{"legacy_source": []byte(legacy), "helper_source": []byte(helper), "migrated_source": converted, "legacy_native": result.originalFiles[legacyNative], "migrated_native": result.originalFiles[v2Native], "import_original": original, "imported_source": imported, "edited_source": edited, "edited_native": candidate, "policy_json": policyBytes} {
		receipt[name] = map[string]any{"bytes": len(data), "sha256": fmt.Sprintf("%x", sha256.Sum256(data)), "base64": base64.StdEncoding.EncodeToString(data)}
	}
	encoded, err := json.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	t.Log("BRG19_NATIVE_EDIT_RECEIPT:" + string(encoded))
	return result
}

func (result p15BRG19ScenarioResult) assertOriginals(t *testing.T) {
	t.Helper()
	for path, original := range result.originalFiles {
		if !bytes.Equal(p15BRG19Read(t, path), original) {
			t.Fatalf("scenario or edit verifier modified original %s", filepath.Base(path))
		}
	}
}

func TestBackendRecovery19StructuralMigrationIntegration(t *testing.T) {
	binary := p15AcceptanceCLI(t)
	result := p15BRG19RunScenario(t, binary, plist.XMLFormat)
	for _, fault := range []string{"metadata", "include", "reference"} {
		t.Run(fault, func(t *testing.T) {
			program, err := p15BRG19Parse(string(result.migrated))
			if err != nil {
				t.Fatal(err)
			}
			mutated := false
			if fault == "metadata" {
				for _, declaration := range program.Declarations {
					if shortcut, okay := declaration.(*syntax.ShortcutDecl); okay {
						shortcut.Name = "WRONG"
						mutated = true
					}
				}
			} else {
				for index, statement := range program.Statements {
					expression, okay := statement.(*syntax.ExprStmt)
					if !okay {
						continue
					}
					call, okay := expression.Expr.(*syntax.CallExpr)
					if !okay {
						continue
					}
					callee, okay := call.Callee.(*syntax.IdentExpr)
					if !okay {
						continue
					}
					if fault == "include" && callee.Name == "comment" {
						if literal, okay := call.PrimaryArg.(*syntax.LiteralExpr); okay && literal.Value == "BRG19_HELPER_KEEP" {
							program.Statements = append(program.Statements[:index], program.Statements[index+1:]...)
							mutated = true
							break
						}
					}
					if fault == "reference" && callee.Name == "saveFile" {
						for index := range call.NamedArgs {
							if call.NamedArgs[index].Label == "folder" {
								call.NamedArgs[index].Value = &syntax.LiteralExpr{Type: syntax.TokenString, Value: "not-the-original-reference"}
								mutated = true
							}
						}
					}
				}
			}
			if !mutated || p15BRG19Structure(program) == nil {
				t.Fatal("structural scenario corruption was not rejected", fault)
			}
		})
	}
	result.assertOriginals(t)
}

func TestBackendRecovery19AuthorizedNativeRoundTrip(t *testing.T) {
	binary := p15AcceptanceCLI(t)
	for _, input := range []struct {
		name   string
		format int
	}{{"xml", plist.XMLFormat}, {"binary", plist.BinaryFormat}} {
		t.Run(input.name, func(t *testing.T) {
			result := p15BRG19RunScenario(t, binary, input.format)
			p15BRG19CandidateControls(t, result)
			p15BRG19PolicyControls(t, result)
			result.assertOriginals(t)
		})
	}
}

func p15BRG19CandidateControls(t *testing.T, result p15BRG19ScenarioResult) {
	t.Helper()
	faults := []string{"metadata_loss", "opaque_data_type", "opaque_large_integer", "opaque_date", "opaque_false_zero", "opaque_identity", "target_before", "target_type", "action_identifier", "sibling_parameter", "action_order", "action_removed", "producer_reference_rename", "raw_folder", "provenance_leak"}
	for _, fault := range faults {
		t.Run("candidate_"+fault, func(t *testing.T) {
			candidate := p15BRG19Decode(t, result.candidate)
			actions := candidate["WFWorkflowActions"].([]any)
			opaque := candidate["BRG19Opaque"].(map[string]any)
			target := actions[result.target].(map[string]any)
			parameters := target["WFWorkflowActionParameters"].(map[string]any)
			switch fault {
			case "metadata_loss":
				delete(candidate, "BRG19Opaque")
			case "opaque_data_type":
				opaque["data"] = []any{uint64(0), uint64(1), uint64(2), uint64(255)}
			case "opaque_large_integer":
				opaque["large"] = uint64(9007199254740994)
			case "opaque_date":
				opaque["date"] = opaque["date"].(time.Time).Add(time.Second)
			case "opaque_false_zero":
				opaque["flag"] = uint64(0)
			case "opaque_identity":
				opaque["UUID"] = "changed-ordinary-id"
			case "target_before":
				parameters["WFCommentActionText"] = p15BRG19Before
			case "target_type":
				parameters["WFCommentActionText"] = uint64(1)
			case "action_identifier":
				target["WFWorkflowActionIdentifier"] = "is.workflow.actions.gettext"
			case "sibling_parameter":
				actions[result.sibling].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)["WFCommentActionText"] = "UNAUTHORIZED_SIBLING_CHANGE"
			case "action_order":
				actions[result.target], actions[result.sibling] = actions[result.sibling], actions[result.target]
			case "action_removed":
				candidate["WFWorkflowActions"] = append(actions[:result.sibling], actions[result.sibling+1:]...)
			case "producer_reference_rename":
				id := ""
				for _, raw := range actions {
					params, _ := raw.(map[string]any)["WFWorkflowActionParameters"].(map[string]any)
					value, _ := params["UUID"].(string)
					if value != "" && p15BRG19ReplaceIdentity(candidate, value, value, false) >= 2 {
						id = value
						break
					}
				}
				if id == "" || p15BRG19ReplaceIdentity(candidate, id, "BRG19_RENAMED_PRODUCER", true) < 2 {
					t.Fatal("fixture lacks an actual producer/reference pair")
				}
			case "raw_folder":
				found := false
				var visit func(any)
				visit = func(value any) {
					switch v := value.(type) {
					case map[string]any:
						if v["relativeSubpath"] == "brg19-public-folder" {
							v["relativeSubpath"] = "wrong-public-folder"
							found = true
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
				visit(actions)
				if !found {
					t.Fatal("fixture lost actual raw folder reference")
				}
			case "provenance_leak":
				candidate["__cherri_import_provenance"] = "not-native-output"
			}
			faultPath := filepath.Join(result.directory, "candidate-"+fault+".shortcut")
			p15BRG19Write(t, faultPath, p15BRG19Encode(t, candidate, plist.XMLFormat))
			verification, err := shortcutcompare.VerifyEditFiles(result.originalPath, faultPath, result.policyPath)
			if err != nil || verification.OK {
				t.Fatalf("actual native collateral change was not rejected: %s %v %+v", fault, err, verification)
			}
			expected := p15BRG19Decode(t, result.original)
			expected["WFWorkflowActions"].([]any)[result.target].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)["WFCommentActionText"] = p15BRG19After
			if shortcutcompare.CompareDocumentsExact(expected, p15BRG19Decode(t, p15BRG19Read(t, faultPath))).Equal {
				t.Fatal("independent whole-tree oracle accepted collateral change")
			}
			result.assertOriginals(t)
		})
	}
}

func p15BRG19ReplaceIdentity(value any, old, replacement string, mutate bool) int {
	count := 0
	switch v := value.(type) {
	case map[string]any:
		for key, child := range v {
			if text, okay := child.(string); okay && (key == "UUID" || key == "OutputUUID") && text == old {
				count++
				if mutate {
					v[key] = replacement
				}
			} else {
				count += p15BRG19ReplaceIdentity(child, old, replacement, mutate)
			}
		}
	case []any:
		for _, child := range v {
			count += p15BRG19ReplaceIdentity(child, old, replacement, mutate)
		}
	}
	return count
}

func p15BRG19PolicyControls(t *testing.T, result p15BRG19ScenarioResult) {
	t.Helper()
	for _, fault := range []string{"missing", "wrong_original", "wrong_before", "wrong_target", "wrong_after", "duplicate_path", "wrong_type", "empty_changes"} {
		t.Run("policy_"+fault, func(t *testing.T) {
			encoded, err := json.Marshal(result.policy)
			if err != nil {
				t.Fatal(err)
			}
			var policy shortcutcompare.EditPolicy
			if err := json.Unmarshal(encoded, &policy); err != nil {
				t.Fatal(err)
			}
			switch fault {
			case "wrong_original":
				policy.OriginalSHA256 = strings.Repeat("0", 64)
			case "wrong_before":
				policy.Changes[0].Before = json.RawMessage(`"WRONG"`)
			case "wrong_target":
				policy.Changes[0].Path[1] = strconv.Itoa(result.sibling)
			case "wrong_after":
				policy.Changes[0].After = json.RawMessage(`"WRONG"`)
			case "duplicate_path":
				policy.Changes = append(policy.Changes, policy.Changes[0])
			case "wrong_type":
				policy.Changes[0].After = json.RawMessage(`1`)
			case "empty_changes":
				policy.Changes = []shortcutcompare.EditChange{}
			}
			path := ""
			if fault != "missing" {
				encoded, err = json.Marshal(policy)
				if err != nil {
					t.Fatal(err)
				}
				path = filepath.Join(result.directory, "policy-"+fault+".json")
				p15BRG19Write(t, path, encoded)
			}
			verification, err := shortcutcompare.VerifyEditFiles(result.originalPath, result.candidatePath, path)
			if err == nil && verification.OK {
				t.Fatal("unauthorized or misbound edit policy was accepted", fault)
			}
			result.assertOriginals(t)
		})
	}
}
