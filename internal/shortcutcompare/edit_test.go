package shortcutcompare

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"

	"howett.net/plist"
)

func editTestWrite(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
}
func editTestPlist(t *testing.T, value map[string]any, format int) []byte {
	t.Helper()
	data, err := plist.Marshal(value, format)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
func editTestBase() map[string]any {
	return map[string]any{
		"WFWorkflowActions": []any{
			map[string]any{"WFWorkflowActionIdentifier": "is.workflow.actions.gettext",
				"WFWorkflowActionParameters": map[string]any{"UUID": "producer-original", "WFTextActionText": "before"}},
			map[string]any{"WFWorkflowActionIdentifier": "is.workflow.actions.showresult",
				"WFWorkflowActionParameters": map[string]any{"Text": map[string]any{"Value": map[string]any{"OutputUUID": "producer-original", "Type": "ActionOutput"}, "WFSerializationType": "WFTextTokenAttachment"}}},
		},
		"WFWorkflowClientVersion": "3100",
		"opaque":                  map[string]any{"UUID": "ordinary-id", "GroupingIdentifier": "ordinary-group", "WFInput": "1", "hash": "#", "large": uint64(9007199254740992), "blob": []byte{1, 2}, "zero": 0.0},
	}
}

func TestEditExactComparisonRejectsParityRelaxations(t *testing.T) {
	cases := []string{"opaque_group", "opaque_uuid", "string_number", "large_integer", "hash_escape", "data_array", "negative_zero", "producer_and_reference_alpha_rename", "metadata_loss", "false_zero"}
	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			a, b := editTestBase(), editTestBase()
			opaque := b["opaque"].(map[string]any)
			switch name {
			case "opaque_group":
				opaque["GroupingIdentifier"] = "changed"
			case "opaque_uuid":
				opaque["UUID"] = "changed"
			case "string_number":
				opaque["WFInput"] = uint64(1)
			case "large_integer":
				opaque["large"] = uint64(9007199254740993)
			case "hash_escape":
				opaque["hash"] = "u0023"
			case "data_array":
				opaque["blob"] = []any{uint64(1), uint64(2)}
			case "negative_zero":
				opaque["zero"] = math.Copysign(0, -1)
			case "producer_and_reference_alpha_rename":
				actions := b["WFWorkflowActions"].([]any)
				actions[0].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)["UUID"] = "producer-renamed"
				actions[1].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)["Text"].(map[string]any)["Value"].(map[string]any)["OutputUUID"] = "producer-renamed"
			case "metadata_loss":
				delete(b, "WFWorkflowClientVersion")
			case "false_zero":
				opaque["zero"] = false
			}
			if !CompareDocumentsExact(a, editTestBase()).Equal {
				t.Fatal("identical healthy control failed")
			}
			if CompareDocumentsExact(a, b).Equal {
				t.Fatal("undeclared typed/identity delta was accepted")
			}
		})
	}
}

func TestEditPolicyBoundedReplacement(t *testing.T) {
	dir := t.TempDir()
	original, candidate, policy := filepath.Join(dir, "original.plist"), filepath.Join(dir, "candidate.plist"), filepath.Join(dir, "policy.json")
	originalBytes := editTestPlist(t, editTestBase(), plist.XMLFormat)
	editTestWrite(t, original, originalBytes)
	expected := editTestBase()
	expected["WFWorkflowActions"].([]any)[0].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)["WFTextActionText"] = "after"
	editTestWrite(t, candidate, editTestPlist(t, expected, plist.BinaryFormat))
	manifest := fmt.Sprintf(`{"schemaVersion":1,"originalSHA256":"%x","changes":[{"path":["WFWorkflowActions","0","WFWorkflowActionParameters","WFTextActionText"],"before":"before","after":"after"}]}`, sha256.Sum256(originalBytes))
	editTestWrite(t, policy, []byte(manifest))
	result, err := VerifyEditFiles(original, candidate, policy)
	if err != nil || !result.OK || result.AllowedChanges != 1 {
		t.Fatalf("explicit scalar replacement: %+v %v", result, err)
	}
	unchanged, err := VerifyEditFiles(original, original, "")
	if err != nil || !unchanged.OK {
		t.Fatalf("zero-change control: %+v %v", unchanged, err)
	}
	rejected, err := VerifyEditFiles(original, candidate, "")
	if err != nil || rejected.OK {
		t.Fatalf("undeclared replacement: %+v %v", rejected, err)
	}
	for _, fault := range []string{"metadata_loss", "retargeted_reference", "opaque_type", "changed_unlisted_uuid"} {
		t.Run(fault, func(t *testing.T) {
			changed := editTestBase()
			changed["WFWorkflowActions"].([]any)[0].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)["WFTextActionText"] = "after"
			switch fault {
			case "metadata_loss":
				delete(changed, "WFWorkflowClientVersion")
			case "retargeted_reference":
				changed["WFWorkflowActions"].([]any)[1].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)["Text"].(map[string]any)["Value"].(map[string]any)["OutputUUID"] = "missing-producer"
			case "opaque_type":
				changed["opaque"].(map[string]any)["WFInput"] = uint64(1)
			case "changed_unlisted_uuid":
				changed["opaque"].(map[string]any)["UUID"] = "different"
			}
			editTestWrite(t, candidate, editTestPlist(t, changed, plist.XMLFormat))
			result, err := VerifyEditFiles(original, candidate, policy)
			if err != nil || result.OK {
				t.Fatalf("extra change accepted: %+v %v", result, err)
			}
		})
	}
	if current, err := os.ReadFile(original); err != nil || string(current) != string(originalBytes) {
		t.Fatal("verification wrote original input")
	}
}

func TestEditPolicyRejectsAmbiguousAuthorizations(t *testing.T) {
	digest := fmt.Sprintf("%x", sha256.Sum256([]byte("synthetic")))
	valid := fmt.Sprintf(`{"schemaVersion":1,"originalSHA256":"%s","changes":[{"path":["field"],"before":"a","after":"b"}]}`, digest)
	for _, bad := range []string{
		`null`, `{}`,
		`{"schemaVersion":1,"schemaVersion":1,"originalSHA256":"` + digest + `","changes":[]}`,
		`{"schemaVersion":1,"originalSHA256":"` + digest + `","changes":[],"unknown":true}`,
		`{"schemaVersion":1,"originalSHA256":"` + digest + `","changes":[{"path":["a"],"before":"x","before":"y","after":"z"}]}`,
		`{"schemaVersion":1,"originalSHA256":"` + digest + `","changes":[{"path":["a"],"before":"x","after":"y"},{"path":["a"],"before":"y","after":"z"}]}`,
		valid + ` {}`,
	} {
		if _, err := decodeEditPolicy([]byte(bad)); err == nil {
			t.Fatalf("ambiguous policy accepted: %s", bad)
		}
	}
	if _, err := decodeEditPolicy([]byte(valid)); err != nil {
		t.Fatal(err)
	}
	cases := []EditChange{
		{Path: []string{"missing"}, Before: json.RawMessage(`"a"`), After: json.RawMessage(`"b"`)},
		{Path: []string{"field"}, Before: json.RawMessage(`"wrong"`), After: json.RawMessage(`"b"`)},
		{Path: []string{"field"}, Before: json.RawMessage(`"a"`), After: json.RawMessage(`1`)},
		{Path: []string{"field"}, Before: json.RawMessage(`"a"`), After: json.RawMessage(`null`)},
		{Path: []string{"field"}, Before: json.RawMessage(`"a"`), After: json.RawMessage(`"a"`)},
		{Path: []string{"list", "01"}, Before: json.RawMessage(`"x"`), After: json.RawMessage(`"y"`)},
		{Path: []string{"list"}, Before: json.RawMessage(`["x"]`), After: json.RawMessage(`["y"]`)},
	}
	for i, change := range cases {
		if err := applyEditChange(map[string]any{"field": "a", "list": []any{"x"}}, change); err == nil {
			t.Errorf("invalid change %d accepted", i)
		}
	}
}
