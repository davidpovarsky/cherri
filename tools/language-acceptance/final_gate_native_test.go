package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const finalGateHistoricalSource = "1e02755bbfaf470b4dba1802aa410f29ab4d1188"
const finalGateHistoricalRuntimeRun = "45544419558"
const finalGateHistoricalEncodingRun = "45545264693"
const finalGateHistoricalPrelude = "d6fbcaad5104562b04004b39ca098486449399c15c83176c49b2ac28f47a3461"

// Exact public synthetic-fixture archives from the historical K runs. Their
// API/source/authentication receipts remain historical; these are read-only
// parser controls, never a replay of Shortcuts or current-source acceptance.
func finalGateHistoricalMembers(t *testing.T, which string) map[string][]byte {
	t.Helper()
	name, digest := "ios27-runtime-poc-k1e02755.original.zip", "e6134d9997f319184414385088e9b191ce99863c67aa9cd0639691a47e08dfec"
	if which == "encoding" {
		name, digest = "encoding-k1e02755.original.zip", "caf97bd8c310b7b316b082cc6f90a06288634a829175d107eedd9493db0c13bf"
	}
	raw, err := os.ReadFile(filepath.Join("testdata", "final-required-gates", name))
	if err != nil {
		t.Fatal(err)
	}
	if ciIdentityDigest(raw) != digest {
		t.Fatal("historical original ZIP bytes changed")
	}
	z, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		t.Fatal(err)
	}
	members := map[string][]byte{}
	var total uint64
	for _, entry := range z.File {
		if path.Base(entry.Name) != entry.Name || entry.Mode()&os.ModeType != 0 || entry.UncompressedSize64 > 1024*1024 {
			t.Fatal("unexpected historical archive member")
		}
		if _, duplicate := members[entry.Name]; duplicate {
			t.Fatal("duplicate historical member")
		}
		total += entry.UncompressedSize64
		if total > 2*1024*1024 {
			t.Fatal("historical archive expanded bound")
		}
		stream, err := entry.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, readErr := io.ReadAll(io.LimitReader(stream, 1024*1024+1))
		closeErr := stream.Close()
		if readErr != nil || closeErr != nil || uint64(len(data)) != entry.UncompressedSize64 {
			t.Fatal("historical archive read")
		}
		members[entry.Name] = data
	}
	return members
}

func finalGateNativeClone(members map[string][]byte) map[string][]byte {
	out := map[string][]byte{}
	for name, data := range members {
		out[name] = append([]byte{}, data...)
	}
	return out
}
func finalGateNativeJSON(t *testing.T, members map[string][]byte, name string, mutate func(map[string]any)) {
	t.Helper()
	value, err := decodeNativeEncodingObject(members[name])
	if err != nil {
		t.Fatal(err)
	}
	mutate(value)
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	members[name] = raw
}
func finalGateNativeRecords(t *testing.T, members map[string][]byte, mutate func([]map[string]any) []map[string]any) {
	t.Helper()
	var rows []map[string]any
	decoder := json.NewDecoder(bytes.NewReader(members["runtime-diagnostics.json"]))
	decoder.UseNumber()
	if err := decoder.Decode(&rows); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(mutate(rows))
	if err != nil {
		t.Fatal(err)
	}
	members["runtime-diagnostics.json"] = raw
}
func finalGateRebindRuntimeArtifacts(t *testing.T, members map[string][]byte) {
	t.Helper()
	finalGateNativeRecords(t, members, func(rows []map[string]any) []map[string]any {
		for _, row := range rows {
			for _, value := range row["artifacts"].([]any) {
				a := value.(map[string]any)
				if data, ok := members[path.Base(a["path"].(string))]; ok {
					a["sha256"] = ciIdentityDigest(data)
				}
			}
		}
		return rows
	})
}

func TestFinalRequiredGatesHistoricalRuntimeOriginals(t *testing.T) {
	members := finalGateHistoricalMembers(t, "runtime")
	before := finalGateNativeClone(members)
	if err := finalGateRuntimeSourceReceipt(members, finalGateHistoricalSource, finalGateHistoricalRuntimeRun, "1"); err != nil {
		t.Fatal(err)
	}
	fields, ids, err := finalGateRuntimeComputed(members, finalGateHistoricalSource, finalGateHistoricalRuntimeRun, "1")
	if err != nil {
		t.Fatal(err)
	}
	if len(fields) != 30 || len(ids) != 17 || fields["primary/FUNCTION"] != "21" || fields["primary/VOID_OUT"] != "" || fields["coverage/F01_RESULTS"] != runtimePOCF01BooleanResults || fields["menu/Alpha"] != "CHERRI_MENU_ALPHA" || fields["menu/Beta"] != "CHERRI_MENU_BETA" {
		t.Fatalf("incomplete computed observations: %d fields/%d cases", len(fields), len(ids))
	}
	if !reflect.DeepEqual(before, members) {
		t.Fatal("parser changed historical originals")
	}
	// The historical requirements labels include obsolete generic mappings. The
	// parser validates original computed fields without granting any of those IDs.
	out := finalGateNativeObservation{RelatedRequirementsGranted: []string{}}
	if len(out.RelatedRequirementsGranted) != 0 {
		t.Fatal("historical labels acquired authority")
	}
}

func TestFinalRequiredGatesRuntimeCausalCorruptions(t *testing.T) {
	original := finalGateHistoricalMembers(t, "runtime")
	if _, _, err := finalGateRuntimeComputed(original, finalGateHistoricalSource, finalGateHistoricalRuntimeRun, "1"); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name       string
		mutate     func(*testing.T, map[string][]byte)
		diagnostic string
	}{
		{"computed_function_rebound", func(t *testing.T, m map[string][]byte) {
			m["clipboard-result.txt"] = bytes.Replace(m["clipboard-result.txt"], []byte("FUNCTION=21"), []byte("FUNCTION=22"), 1)
			finalGateRebindRuntimeArtifacts(t, m)
		}, "BRG31_PRIMARY_COMPUTED_ASSERTION"},
		{"empty_value_not_missing", func(t *testing.T, m map[string][]byte) {
			m["clipboard-result.txt"] = bytes.Replace(m["clipboard-result.txt"], []byte("VOID_OUT=\n"), nil, 1)
			finalGateRebindRuntimeArtifacts(t, m)
		}, "BRG31_RUNTIME_FIELD_INVENTORY"},
		{"unicode_normalization", func(t *testing.T, m map[string][]byte) {
			m["clipboard-result.txt"] = bytes.Replace(m["clipboard-result.txt"], []byte("שָׁלוֹם"), []byte("שלום"), 1)
			finalGateRebindRuntimeArtifacts(t, m)
		}, "BRG31_PRIMARY_COMPUTED_ASSERTION"},
		{"missing_final_lf", func(t *testing.T, m map[string][]byte) {
			m["clipboard-result.txt"] = bytes.TrimSuffix(m["clipboard-result.txt"], []byte("\n"))
			finalGateRebindRuntimeArtifacts(t, m)
		}, "BRG31_RUNTIME_OUTPUT_BOUNDARY"},
		{"extra_output", func(t *testing.T, m map[string][]byte) {
			m["clipboard-result.txt"] = append(m["clipboard-result.txt"], []byte("UNEXPECTED=pass\n")...)
			finalGateRebindRuntimeArtifacts(t, m)
		}, "BRG31_RUNTIME_FIELD_INVENTORY"},
		{"malformed_utf8", func(t *testing.T, m map[string][]byte) {
			m["clipboard-result.txt"] = append([]byte{0xff}, m["clipboard-result.txt"]...)
		}, "BRG31_RUNTIME_TEXT"},
		{"dictionary_false_value", func(t *testing.T, m map[string][]byte) {
			m["dict-clipboard-result.txt"] = bytes.Replace(m["dict-clipboard-result.txt"], []byte("DICT_D=valD"), []byte("DICT_D=valA"), 1)
			finalGateRebindRuntimeArtifacts(t, m)
		}, "BRG31_DICTIONARY_COMPUTED_ASSERTION"},
		{"legacy_unconditional_alias", func(t *testing.T, m map[string][]byte) {
			m["legacy-if-clipboard-result.txt"] = []byte("CHERRI_IOS27_RUNTIME_OK\n")
			finalGateRebindRuntimeArtifacts(t, m)
		}, "BRG31_LEGACY_COMPUTED_ASSERTION"},
		{"menu_wrong_terminal_owner", func(t *testing.T, m map[string][]byte) {
			m["ui_helper_menu_select_alpha.log"] = bytes.Replace(m["ui_helper_menu_select_alpha.log"], []byte("testInteractWithShortcuts]' passed"), []byte("testUnrelated]' passed"), 1)
			finalGateNativeJSON(t, m, "menu-alpha-selection-proof.json", func(o map[string]any) { o["log_sha256"] = ciIdentityDigest(m["ui_helper_menu_select_alpha.log"]) })
			finalGateRebindRuntimeArtifacts(t, m)
		}, "BRG31_MENU_ACTUAL_TEST_BOUNDARY"},
		{"menu_wrong_exact_label", func(t *testing.T, m map[string][]byte) {
			m["ui_helper_menu_select_alpha.log"] = bytes.Replace(m["ui_helper_menu_select_alpha.log"], []byte("actual_label=Alpha"), []byte("actual_label=AlphaExtra"), 1)
			finalGateRebindRuntimeArtifacts(t, m)
		}, "BRG31_MENU_EXACT_TAP"},
		{"menu_unowned_host", func(t *testing.T, m map[string][]byte) {
			m["ui_helper_menu_select_alpha.log"] = bytes.Replace(m["ui_helper_menu_select_alpha.log"], []byte("app=SpringBoard"), []byte("app=Unrelated"), 1)
			finalGateRebindRuntimeArtifacts(t, m)
		}, "BRG31_MENU_EXACT_TAP"},
		{"menu_duplicate_marker", func(t *testing.T, m map[string][]byte) {
			mark := finalGateMenuMarker.Find(m["ui_helper_menu_select_alpha.log"])
			m["ui_helper_menu_select_alpha.log"] = append(m["ui_helper_menu_select_alpha.log"], append(mark, '\n')...)
		}, "BRG31_MENU_ACTUAL_TEST_BOUNDARY"},
		{"menu_stale_run_proof", func(t *testing.T, m map[string][]byte) {
			finalGateNativeJSON(t, m, "menu-beta-selection-proof.json", func(o map[string]any) { o["run_id"] = "1" })
			finalGateRebindRuntimeArtifacts(t, m)
		}, "BRG31_MENU_PROOF_DETACHED"},
		{"menu_wrong_result", func(t *testing.T, m map[string][]byte) {
			m["menu-alpha-clipboard-result.txt"] = []byte("CHERRI_MENU_BETA\n")
			finalGateRebindRuntimeArtifacts(t, m)
		}, "BRG31_MENU_COMPUTED_RESULT"},
		{"missing_diagnostic", func(t *testing.T, m map[string][]byte) {
			finalGateNativeRecords(t, m, func(r []map[string]any) []map[string]any { return r[:len(r)-1] })
		}, "BRG31_COMPLETE_DIAGNOSTIC_INVENTORY"},
		{"duplicate_diagnostic", func(t *testing.T, m map[string][]byte) {
			finalGateNativeRecords(t, m, func(r []map[string]any) []map[string]any { r[len(r)-1] = r[0]; return r })
		}, "BRG31_COMPUTED_DIAGNOSTIC_CONTRADICTION"},
		{"wrong_diagnostic_source", func(t *testing.T, m map[string][]byte) {
			finalGateNativeRecords(t, m, func(r []map[string]any) []map[string]any {
				r[0]["implementation_sha"] = strings.Repeat("0", 40)
				return r
			})
		}, "BRG31_COMPUTED_DIAGNOSTIC_CONTRADICTION"},
		{"missing_exit_code", func(t *testing.T, m map[string][]byte) {
			finalGateNativeRecords(t, m, func(r []map[string]any) []map[string]any { delete(r[0], "exit_code"); return r })
		}, "BRG31_COMPUTED_DIAGNOSTIC_CONTRADICTION"},
		{"duplicate_json_key", func(t *testing.T, m map[string][]byte) {
			m["runtime-diagnostics.json"] = bytes.Replace(m["runtime-diagnostics.json"], []byte(`"exit_code": 0`), []byte(`"exit_code": 1, "exit_code": 0`), 1)
		}, "BRG31_COMPLETE_DIAGNOSTIC_INVENTORY"},
		{"diagnostic_artifact_other_run", func(t *testing.T, m map[string][]byte) {
			finalGateNativeRecords(t, m, func(r []map[string]any) []map[string]any {
				r[0]["artifacts"].([]any)[0].(map[string]any)["run_id"] = "1"
				return r
			})
		}, "BRG31_RUNTIME_RAW_ARTIFACT_BINDING"},
		{"diagnostic_missing_raw", func(t *testing.T, m map[string][]byte) {
			finalGateNativeRecords(t, m, func(r []map[string]any) []map[string]any {
				r[0]["artifacts"] = r[0]["artifacts"].([]any)[:19]
				return r
			})
		}, "BRG31_RUNTIME_RAW_ARTIFACT_INVENTORY"},
		{"coverage_changed_contract", func(t *testing.T, m map[string][]byte) {
			m["coverage-expected.json"] = append(m["coverage-expected.json"], ' ')
		}, "BRG31_COVERAGE_ORIGINAL_CONTRACT"},
		{"coverage_missing_reset", func(t *testing.T, m map[string][]byte) { delete(m, "coverage-reset-observed.txt") }, "BRG31_COVERAGE_FRESH_RESET"},
		{"coverage_wrong_attempt", func(t *testing.T, m map[string][]byte) {
			finalGateNativeJSON(t, m, "coverage-run-context.json", func(o map[string]any) { o["run_attempt"] = "2" })
		}, "BRG31_COVERAGE_CONTEXT_IDENTITY"},
		{"coverage_context_unknown_field", func(t *testing.T, m map[string][]byte) {
			finalGateNativeJSON(t, m, "coverage-run-context.json", func(o map[string]any) { o["late_override"] = true })
		}, "BRG31_COVERAGE_CONTEXT_IDENTITY"},
		{"coverage_wrong_device", func(t *testing.T, m map[string][]byte) {
			finalGateNativeJSON(t, m, "coverage-run-context.json", func(o map[string]any) { o["device_udid"] = "00000000-0000-0000-0000-000000000000" })
		}, "BRG31_COVERAGE_CONTEXT_IDENTITY"},
		{"device_not_owned", func(t *testing.T, m map[string][]byte) {
			finalGateNativeJSON(t, m, "runtime-poc-device.json", func(o map[string]any) { o["createdByRun"] = false })
		}, "BRG31_RUNTIME_OWNED_DEVICE"},
		{"device_not_deleted", func(t *testing.T, m map[string][]byte) {
			finalGateNativeJSON(t, m, "runtime-poc-device.json", func(o map[string]any) { o["cleanupStatus"] = "UNOBSERVED" })
		}, "BRG31_RUNTIME_OWNED_DEVICE"},
		{"coverage_false_result", func(t *testing.T, m map[string][]byte) {
			finalGateNativeJSON(t, m, "coverage-result.json", func(o map[string]any) { o["exact_output_match"] = false })
		}, "BRG31_COVERAGE_RESULT_REPLAY"},
		{"coverage_result_extra_claim", func(t *testing.T, m map[string][]byte) {
			finalGateNativeJSON(t, m, "coverage-result.json", func(o map[string]any) { o["formalAcceptanceGranted"] = true })
		}, "BRG31_COVERAGE_RESULT_REPLAY"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := finalGateNativeClone(original)
			tc.mutate(t, m)
			if reflect.DeepEqual(m, original) {
				t.Fatal("causal mutation did not change bytes")
			}
			_, _, err := finalGateRuntimeComputed(m, finalGateHistoricalSource, finalGateHistoricalRuntimeRun, "1")
			if err == nil || !strings.Contains(err.Error(), tc.diagnostic) {
				t.Fatalf("want %s, got %v", tc.diagnostic, err)
			}
		})
	}
}

func TestFinalRequiredGatesRuntimeSourceReceipt(t *testing.T) {
	original := finalGateHistoricalMembers(t, "runtime")
	for _, fault := range []string{"source", "run", "attempt", "omitted_fixture", "duplicate_fixture", "wrong_fixture_hash", "synthetic_flag"} {
		t.Run(fault, func(t *testing.T) {
			m := finalGateNativeClone(original)
			finalGateNativeJSON(t, m, "runtime-poc-source-provenance.json", func(o map[string]any) {
				switch fault {
				case "source":
					o["implementationSha"] = strings.Repeat("0", 40)
				case "run":
					o["runId"] = "1"
				case "attempt":
					o["runAttempt"] = "2"
				case "omitted_fixture":
					o["sources"] = o["sources"].([]any)[:4]
				case "duplicate_fixture":
					s := o["sources"].([]any)
					s[4] = s[0]
				case "wrong_fixture_hash":
					o["sources"].([]any)[0].(map[string]any)["sourceSha256"] = strings.Repeat("0", 64)
				case "synthetic_flag":
					o["syntheticSourcesVerified"] = false
				}
			})
			if finalGateRuntimeSourceReceipt(m, finalGateHistoricalSource, finalGateHistoricalRuntimeRun, "1") == nil {
				t.Fatal("detached source provenance accepted")
			}
		})
	}
}

func finalGateHistoricalEncodingWitnesses() map[string]string {
	return map[string]string{
		"tools/shortcut-runtime/jxa/verify_encodings.js":             "ea16f22360d2af239effee2ced4b80e55b7e42b6e57dce99e7834059ffbde41a",
		"tools/shortcut-runtime/required-encoding-expectations.json": "fe521c4c782ccb74a7c3327d28dcc48c358f2943bd5f070a1f2a20bc63e2ce38",
		"tools/shortcut-runtime/required_encoding_gate.py":           "50a8f32d4e6f2e51d369b093b8cec2dc7b4cb255db7b780370689f53452b45af",
		"docs/apple-runtime/parameter-encodings.json":                "27c0a367f9856d83445fbad83a333b5aa5e82717846c0c5c34e72d656457b546",
		"docs/apple-runtime/encoding-table.json":                     "b9dd843f63ba90e24739027017178cbf09ef602538b1f9337d92e4407d5138fc",
	}
}
func finalGateRebindEncodingArtifacts(t *testing.T, m map[string][]byte) {
	t.Helper()
	finalGateNativeJSON(t, m, "required-encoding-evidence.json", func(o map[string]any) {
		for _, v := range o["records"].([]any) {
			for _, a := range v.(map[string]any)["artifacts"].([]any) {
				artifact := a.(map[string]any)
				if data, ok := m[path.Base(artifact["path"].(string))]; ok {
					artifact["sha256"] = ciIdentityDigest(data)
				}
			}
		}
	})
}

func TestFinalRequiredGatesHistoricalEncodingOriginals(t *testing.T) {
	m := finalGateHistoricalMembers(t, "encoding")
	before := finalGateNativeClone(m)
	w := finalGateHistoricalEncodingWitnesses()
	var out finalGateNativeObservation
	if err := finalGateEncodingOriginals(m, finalGateHistoricalSource, finalGateHistoricalEncodingRun, 1, finalGateHistoricalPrelude, w, &out); err != nil {
		t.Fatal(err)
	}
	if out.EncodingRequiredCases != 23 || out.EncodingPositiveExit != 0 || out.EncodingCorruptExit != 1 || out.EncodingRestoredExit != 0 || len(out.EncodingSourceFiles) != 6 || len(w) != 5 || !reflect.DeepEqual(before, m) {
		t.Fatal("historical exact triplet or immutable source join changed")
	}
}

func TestFinalRequiredGatesEncodingCausalCorruptions(t *testing.T) {
	original := finalGateHistoricalMembers(t, "encoding")
	for _, fault := range []string{"wrong_source", "wrong_run", "wrong_prelude", "wrong_source_pin", "not_darwin", "missing_process", "corrupt_zero_exit", "wrong_interpreter", "missing_case", "unexpected_record", "duplicate_record"} {
		t.Run(fault, func(t *testing.T) {
			m := finalGateNativeClone(original)
			w := finalGateHistoricalEncodingWitnesses()
			source, run, prelude := finalGateHistoricalSource, finalGateHistoricalEncodingRun, finalGateHistoricalPrelude
			switch fault {
			case "wrong_source":
				source = strings.Repeat("0", 40)
			case "wrong_run":
				run = "1"
			case "wrong_prelude":
				prelude = strings.Repeat("0", 64)
			case "wrong_source_pin":
				w["tools/shortcut-runtime/required_encoding_gate.py"] = strings.Repeat("0", 64)
			case "not_darwin":
				finalGateNativeJSON(t, m, "required-encoding-gate.json", func(o map[string]any) { o["host"].(map[string]any)["system"] = "Linux" })
			case "missing_process":
				delete(m, "corrupt.process.json")
			case "corrupt_zero_exit":
				finalGateNativeJSON(t, m, "required-encoding-gate.json", func(o map[string]any) { o["phases"].([]any)[1].(map[string]any)["exitCode"] = json.Number("0") })
			case "wrong_interpreter":
				finalGateNativeJSON(t, m, "required-encoding-gate.json", func(o map[string]any) {
					phase := o["phases"].([]any)[1].(map[string]any)
					phase["command"].([]any)[0] = "/bin/echo"
					m["corrupt.process.json"], _ = json.Marshal(phase)
				})
			case "missing_case":
				finalGateNativeJSON(t, m, "required-encoding-gate.json", func(o map[string]any) { o["requiredCases"] = json.Number("22") })
			case "unexpected_record":
				finalGateNativeJSON(t, m, "required-encoding-evidence.json", func(o map[string]any) { o["records"].([]any)[0].(map[string]any)["requirements"] = []any{"BRG31"} })
			case "duplicate_record":
				finalGateNativeJSON(t, m, "required-encoding-evidence.json", func(o map[string]any) { rows := o["records"].([]any); rows[1] = rows[0] })
			}
			finalGateRebindEncodingArtifacts(t, m)
			var out finalGateNativeObservation
			if err := finalGateEncodingOriginals(m, source, run, 1, prelude, w, &out); err == nil {
				t.Fatal("counterfeit encoding original accepted")
			}
		})
	}
}
