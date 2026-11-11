/*
 * Copyright (c) Cherri
 */

package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"howett.net/plist"
)

func buildComplexSanitizedFidelityDoc() map[string]any {
	prod0 := "11111111-1111-4111-8111-111111111111"
	dangling := "DDDDDDDD-DDDD-4DDD-8DDD-DDDDDDDDDDDD"
	group0 := "AAAAAAAA-AAAA-4AAA-8AAA-AAAAAAAAAAAA"

	return map[string]any{
		"WFWorkflowClientVersion":              "2302.0.4",
		"WFWorkflowHasOutputFallback":          false,
		"WFWorkflowHasShortcutInputVariables":  false,
		"WFWorkflowIcon": map[string]any{
			"WFWorkflowIconGlyphNumber": uint64(59511),
			"WFWorkflowIconStartColor":  int64(-43634177),
		},
		"WFWorkflowImportQuestions": []any{
			map[string]any{
				"ActionIndex":  uint64(1),
				"Category":     "Parameter",
				"DefaultValue": "default-q",
				"ParameterKey": "WFTextActionText",
				"Text":         "Enter value",
			},
		},
		"WFWorkflowInputContentItemClasses": []any{},
		// Intentionally omit WFWorkflowOutputContentItemClasses & WFQuickActionSurfaces to verify empty-vs-absent preservation
		"WFWorkflowMinimumClientVersion":       uint64(1113),
		"WFWorkflowMinimumClientVersionString": "1113",
		"WFWorkflowTypes":                      []any{"NCWidget"},
		"WFWorkflowCustomUnknownKey":           "PreserveMeExact",
		"WFWorkflowActions": []any{
			map[string]any{
				"WFWorkflowActionIdentifier": "is.workflow.actions.gettext",
				"WFWorkflowActionParameters": map[string]any{
					"UUID":             prod0,
					"CustomOutputName": "תוצאה ראשונה! [item 1]",
					"WFTextActionText": "Literal {notAVar} ${alsoLiteral} {\"json\": \"braces\"} Hebrew: שלום\nLine2\tTab\"Quoted\"\\Slash",
				},
			},
			map[string]any{
				"WFWorkflowActionIdentifier": "is.workflow.actions.getvalueforkey",
				"WFWorkflowActionParameters": map[string]any{
					"WFDictionaryKey": "targetKey",
					"WFInput": map[string]any{
						"WFSerializationType": "WFTextTokenAttachment",
						"Value": map[string]any{
							"Type":       "ActionOutput",
							"OutputUUID": prod0,
							"OutputName": "תוצאה ראשונה! [item 1]",
							"Aggrandizements": []any{
								map[string]any{
									"Type":              "WFDictionaryValueVariableAggrandizement",
									"DictionaryKey":     "nested",
									"CoercionItemClass": "WFStringContentItem",
								},
							},
						},
					},
				},
			},
			map[string]any{
				"WFWorkflowActionIdentifier": "is.workflow.actions.calculateexpression",
				"WFWorkflowActionParameters": map[string]any{
					"Input": map[string]any{
						"WFSerializationType": "WFTextTokenString",
						"Value": map[string]any{
							"string": "1 + \uFFFC",
							"attachmentsByRange": map[string]any{
								"{4, 1}": map[string]any{
									"Type":         "Variable",
									"VariableName": "My Forward Var!",
									"OutputUUID":   dangling,
								},
							},
						},
					},
				},
			},
			map[string]any{
				"WFWorkflowActionIdentifier": "is.workflow.actions.comment",
				"WFWorkflowActionParameters": map[string]any{
					"WFCommentActionText": "Exact comment text with {braces}",
				},
			},
			map[string]any{
				"WFWorkflowActionIdentifier": "com.example.thirdparty.Intent",
				"WFWorkflowActionParameters": map[string]any{
					"AppIntentDescriptor": map[string]any{
						"ActionRequiresAppInstallation": "true",
						"TeamIdentifier":                "TEAM123",
						"BundleIdentifier":              "com.example.app",
						"NativeFlag":                    true,
					},
					"IntentAppDefinition": map[string]any{
						"Name":   "ExampleApp",
						"Strict": false,
					},
				},
			},
			// Present empty parameter container vs absent parameter container
			map[string]any{
				"WFWorkflowActionIdentifier": "is.workflow.actions.noop.empty",
				"WFWorkflowActionParameters": map[string]any{},
			},
			map[string]any{
				"WFWorkflowActionIdentifier": "is.workflow.actions.noop.absent",
			},
			// XML-1.0-safe multiline/tab/escaped string payload (tested across XML, bplist, and JSON)
			map[string]any{
				"WFWorkflowActionIdentifier": "is.workflow.actions.gettext",
				"WFWorkflowActionParameters": map[string]any{
					"WFTextActionText": "Before\nAfter\tLine\"Q\"\\S",
				},
			},
			// Orphan End If must be reproduced 1:1 verbatim, never dropped
			map[string]any{
				"WFWorkflowActionIdentifier": "is.workflow.actions.conditional",
				"WFWorkflowActionParameters": map[string]any{
					"GroupingIdentifier": group0,
					"WFControlFlowMode":  uint64(2),
				},
			},
		},
	}
}

func TestP10R8_LosslessImportAndCompileParityLegacyAndV2(t *testing.T) {
	bin := roundTripBinary(t)
	doc := buildComplexSanitizedFidelityDoc()

	var xmlBytes bytes.Buffer
	if err := plist.NewEncoderForFormat(&xmlBytes, plist.XMLFormat).Encode(doc); err != nil {
		t.Fatalf("encode xml plist: %v", err)
	}
	var binBytes bytes.Buffer
	if err := plist.NewEncoderForFormat(&binBytes, plist.BinaryFormat).Encode(doc); err != nil {
		t.Fatalf("encode binary plist: %v", err)
	}
	jsonBytes, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("encode json shortcut: %v", err)
	}

	dir := t.TempDir()
	xmlIn := filepath.Join(dir, "sample.plist")
	binIn := filepath.Join(dir, "sample.shortcut")
	jsonIn := filepath.Join(dir, "sample.json")
	if err := os.WriteFile(xmlIn, xmlBytes.Bytes(), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(binIn, binBytes.Bytes(), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(jsonIn, jsonBytes, 0644); err != nil {
		t.Fatal(err)
	}

	for _, mode := range []string{"legacy", "v2"} {
		t.Run(mode, func(t *testing.T) {
			var firstRebuilt map[string]any
			var firstSrc string
			for _, tc := range []struct {
				label string
				path  string
			}{
				{"xml", xmlIn},
				{"bplist", binIn},
				{"json", jsonIn},
			} {
				cherriOut := filepath.Join(dir, tc.label+"_"+mode+".cherri")
				impArgs := []string{"import", tc.path, "--mode", mode, "-o", cherriOut}
				if out, err := exec.Command(bin, impArgs...).CombinedOutput(); err != nil {
					t.Fatalf("%s import (%s) failed: %v\n%s", tc.label, mode, err, string(out))
				}
				srcBytes, err := os.ReadFile(cherriOut)
				if err != nil {
					t.Fatalf("read %s: %v", cherriOut, err)
				}
				src := string(srcBytes)
				if mode == "v2" {
					if !strings.Contains(src, "native.workflow(") || strings.Contains(src, "#workflowheader") {
						t.Fatalf("expected genuine V2 lossless source, got:\n%s", src)
					}
				} else {
					if !strings.Contains(src, "#workflowheader") || !strings.Contains(src, "rawPlist(") {
						t.Fatalf("expected Legacy lossless source, got:\n%s", src)
					}
				}

				if firstSrc == "" {
					firstSrc = src
				} else if src != firstSrc {
					t.Fatalf("%s (%s) source differed across container formats", tc.label, mode)
				}

				scOut := filepath.Join(dir, tc.label+"_"+mode+".shortcut")
				var cmpArgs []string
				if mode == "legacy" {
					cmpArgs = []string{cherriOut, "--legacy", "--skip-sign", "-o", scOut}
				} else {
					cmpArgs = []string{"compile", cherriOut, "--skip-sign", "-o", scOut}
				}
				if out, err := exec.Command(bin, cmpArgs...).CombinedOutput(); err != nil {
					t.Fatalf("%s compile (%s) failed: %v\n%s", tc.label, mode, err, string(out))
				}
				rebBytes, err := os.ReadFile(scOut)
				if err != nil {
					t.Fatal(err)
				}
				rebDoc, err := decodeShortcutRawMap(rebBytes)
				if err != nil {
					t.Fatalf("decode rebuilt %s: %v", tc.label, err)
				}

				if !plistValuesStrictlyEqual(doc, rebDoc) {
					t.Fatalf("%s (%s) rebuilt plist differed from source at: %s",
						tc.label, mode, firstPlistDiffPath(doc, rebDoc, ""))
				}
				if firstRebuilt == nil {
					firstRebuilt = rebDoc
				} else if !plistValuesStrictlyEqual(firstRebuilt, rebDoc) {
					t.Fatalf("%s (%s) rebuilt plist differed from xml container result", tc.label, mode)
				}
			}
		})
	}
}

func TestP10R8_GlobalCompilerFixesAndDefaultsPreserved(t *testing.T) {
	bin := roundTripBinary(t)
	dir := t.TempDir()

	// 1. User-authored Cherri (Legacy) retains standard defaults and preserves empty dictionary `{}` literals.
	legSrc := filepath.Join(dir, "user_legacy.cherri")
	legCode := "@x = \"hello\"\n@empty = {}\nrawAction(\"is.workflow.actions.testempty\", {\"EmptySub\": {}})\n"
	if err := os.WriteFile(legSrc, []byte(legCode), 0644); err != nil {
		t.Fatal(err)
	}
	legSc := filepath.Join(dir, "user_legacy.shortcut")
	if out, err := exec.Command(bin, legSrc, "--legacy", "--skip-sign", "-o", legSc).CombinedOutput(); err != nil {
		t.Fatalf("legacy compile failed: %v\n%s", err, string(out))
	}
	legDoc, err := decodeShortcutRawMap(mustReadFile(t, legSc))
	if err != nil {
		t.Fatal(err)
	}
	if _, hasMin := legDoc["WFWorkflowMinimumClientVersion"]; !hasMin {
		t.Fatalf("expected user-authored Legacy shortcut to retain default WFWorkflowMinimumClientVersion")
	}
	legActs := legDoc["WFWorkflowActions"].([]any)
	foundDict := false
	foundNestedEmpty := false
	for _, a := range legActs {
		act := a.(map[string]any)
		switch act["WFWorkflowActionIdentifier"] {
		case "is.workflow.actions.dictionary":
			foundDict = true
		case "is.workflow.actions.testempty":
			foundNestedEmpty = true
			p, ok := act["WFWorkflowActionParameters"].(map[string]any)
			if !ok {
				t.Fatalf("expected testempty params map, got: %#v", act["WFWorkflowActionParameters"])
			}
			sub, ok := p["EmptySub"].(map[string]any)
			if !ok {
				t.Fatalf("expected nested {} literal map, got: %#v", p["EmptySub"])
			}
			valMap, hasVal := sub["Value"].(map[string]any)
			if sub["WFSerializationType"] != "WFDictionaryFieldValue" || !hasVal || len(valMap) != 0 {
				t.Fatalf("expected nested {} literal to emit WFDictionaryFieldValue with empty Value map, got: %#v", p["EmptySub"])
			}
		}
	}
	if !foundDict || !foundNestedEmpty {
		t.Fatalf("expected dictionary (%v) and testempty (%v) actions in legacy user-authored workflow", foundDict, foundNestedEmpty)
	}

	// 2. #declare registers variable references without generating setvariable or nothing actions.
	declSrc := filepath.Join(dir, "declare_legacy.cherri")
	declCode := "#declare futureVar\nrawAction(\"is.workflow.actions.showresult\", {\"Text\": \"{futureVar}\"})\n"
	if err := os.WriteFile(declSrc, []byte(declCode), 0644); err != nil {
		t.Fatal(err)
	}
	declSc := filepath.Join(dir, "declare_legacy.shortcut")
	if out, err := exec.Command(bin, declSrc, "--legacy", "--skip-sign", "-o", declSc).CombinedOutput(); err != nil {
		t.Fatalf("declare legacy compile failed: %v\n%s", err, string(out))
	}
	declDoc, err := decodeShortcutRawMap(mustReadFile(t, declSc))
	if err != nil {
		t.Fatal(err)
	}
	declActs := declDoc["WFWorkflowActions"].([]any)
	if len(declActs) != 1 || declActs[0].(map[string]any)["WFWorkflowActionIdentifier"] != "is.workflow.actions.showresult" {
		t.Fatalf("expected exactly 1 showresult action without hidden setvariable/nothing, got: %+v", declActs)
	}

	// 3. Semantic --import= keeps semantic curated output by default; --fidelity=semantic also works on `cherri import`.
	semPlist := filepath.Join(dir, "sem_test.plist")
	var semBuf bytes.Buffer
	_ = plist.NewEncoderForFormat(&semBuf, plist.XMLFormat).Encode(map[string]any{
		"WFWorkflowClientVersion": "2302.0.4",
		"WFWorkflowTypes":         []any{"NCWidget"},
		"WFWorkflowActions": []any{
			map[string]any{
				"WFWorkflowActionIdentifier": "is.workflow.actions.gettext",
				"WFWorkflowActionParameters": map[string]any{"WFTextActionText": "hello"},
			},
		},
	})
	if err := os.WriteFile(semPlist, semBuf.Bytes(), 0644); err != nil {
		t.Fatal(err)
	}
	semOut := filepath.Join(dir, "sem_flag.cherri")
	if out, err := exec.Command(bin, "--import="+semPlist, "-o", semOut).CombinedOutput(); err != nil {
		t.Fatalf("--import failed: %v\n%s", err, string(out))
	}
	semText := string(mustReadFile(t, semOut))
	if strings.Contains(semText, "native.plist(") || strings.Contains(semText, "#workflowheader") {
		t.Fatalf("--import= without --fidelity=lossless should emit semantic Cherri, got:\n%s", semText)
	}
	v2SemOut := filepath.Join(dir, "v2_sem.cherri")
	if out, err := exec.Command(bin, "import", semPlist, "--fidelity=semantic", "-o", v2SemOut).CombinedOutput(); err != nil {
		t.Fatalf("import --fidelity=semantic failed: %v\n%s", err, string(out))
	}
	if s := string(mustReadFile(t, v2SemOut)); strings.Contains(s, "native.workflow(") {
		t.Fatalf("import --fidelity=semantic should emit semantic source, not lossless native.workflow:\n%s", s)
	}
}

func TestP10R8_InProcessIsolationAndSyntaxGuard(t *testing.T) {
	// Two interleaved V2 compilations (exact lossless followed by normal user-authored) do not leak metadata.
	doc := buildComplexSanitizedFidelityDoc()
	v2LosslessSrc, err := renderLosslessV2Source(doc)
	if err != nil {
		t.Fatal(err)
	}
	out1, err := CompileSourceToPlist("lossless.cherri", v2LosslessSrc)
	if err != nil {
		t.Fatalf("CompileSourceToPlist lossless failed: %v", err)
	}
	dec1, err := decodeShortcutRawMap(out1)
	if err != nil {
		t.Fatal(err)
	}
	if !plistValuesStrictlyEqual(doc, dec1) {
		t.Fatalf("in-process V2 lossless round-trip differed at: %s", firstPlistDiffPath(doc, dec1, ""))
	}

	out2, err := CompileSourceToPlist("normal.cherri", "show(\"ok\")\n")
	if err != nil {
		t.Fatalf("CompileSourceToPlist normal after lossless failed: %v", err)
	}
	dec2, err := decodeShortcutRawMap(out2)
	if err != nil {
		t.Fatal(err)
	}
	if _, leaked := dec2["WFWorkflowCustomUnknownKey"]; leaked {
		t.Fatalf("global state leaked exact lossless header key into subsequent normal V2 compilation")
	}
	if dec2["WFWorkflowClientVersion"] == "2302.0.4" {
		t.Fatalf("source clientVersion leaked into subsequent normal V2 compilation")
	}

	// Malformed unmatched '}' fails closed without infinite loop / OOM in V2 parser.
	if _, err := CompileSourceToPlist("malformed.cherri", "}\n}\n}\n"); err == nil {
		t.Fatalf("expected syntax error for unmatched braces, got nil")
	}

	// Interleaved lossless and semantic ExecuteImportPipeline calls in one process do not leak fidelity state.
	dir := t.TempDir()
	xmlIn := filepath.Join(dir, "in.plist")
	var b bytes.Buffer
	_ = plist.NewEncoderForFormat(&b, plist.XMLFormat).Encode(doc)
	if err := os.WriteFile(xmlIn, b.Bytes(), 0644); err != nil {
		t.Fatal(err)
	}
	res1, err := ExecuteImportPipeline(ImportOptions{
		InputPath:  xmlIn,
		OutputPath: filepath.Join(dir, "out1.cherri"),
		Mode:       "v2",
		Fidelity:   "lossless",
	})
	if err != nil || !strings.Contains(res1.FinalSource, "native.workflow(") {
		t.Fatalf("expected lossless V2 pipeline output, err=%v", err)
	}
	res2, err := ExecuteImportPipeline(ImportOptions{
		InputPath:  xmlIn,
		OutputPath: filepath.Join(dir, "out2.cherri"),
		Mode:       "legacy",
		Fidelity:   "semantic",
	})
	if err != nil || strings.Contains(res2.FinalSource, "#workflowheader") {
		t.Fatalf("expected semantic legacy pipeline output without leaked lossless mode, err=%v", err)
	}
	res3, err := ExecuteImportPipeline(ImportOptions{
		InputPath:  xmlIn,
		OutputPath: filepath.Join(dir, "out3.cherri"),
		Mode:       "legacy",
		Fidelity:   "lossless",
	})
	if err != nil || !strings.Contains(res3.FinalSource, "#workflowheader") {
		t.Fatalf("expected lossless legacy pipeline output on second lossless call, err=%v", err)
	}
}

func TestP10R8_BinaryPlistBase64FallbackAndMalformedInputs(t *testing.T) {
	bin := roundTripBinary(t)
	dir := t.TempDir()

	// 1. Control byte \x0b / \r triggers encodePlistValueToLiteral -> "b64:" fallback and exact decodeRawPlistValueString round-trip.
	ctrlMap := map[string]any{
		"WFTextActionText": "Before\rAfter\x0bBell",
		"RawBytes":         []byte{0x00, 0x0b, 0xff},
	}
	lit, err := encodePlistValueToLiteral(ctrlMap)
	if err != nil {
		t.Fatalf("encodePlistValueToLiteral failed: %v", err)
	}
	if !strings.HasPrefix(lit, "b64:") {
		t.Fatalf("expected b64: fallback for XML-unsafe control byte, got %q", lit)
	}
	decodedCtrl, err := decodeRawPlistValueString(lit)
	if err != nil {
		t.Fatalf("decodeRawPlistValueString(b64:) failed: %v", err)
	}
	if !plistValuesStrictlyEqual(ctrlMap, decodedCtrl) {
		t.Fatalf("b64: round-trip mismatch: %s", firstPlistDiffPath(ctrlMap, decodedCtrl, ""))
	}

	// 2. Legacy rawPlist("b64:...") and V2 native.plist("b64:...") both decode b64 binary plist payloads cleanly during compilation.
	xmlSafePayload := map[string]any{
		"WFTextActionText": "ExactViaB64",
		"RawBinaryPayload": []byte{0x00, 0x0b, 0xff},
		"NestedFlag":       true,
		"Count":            uint64(7),
	}
	bplistBytes, err := plist.Marshal(xmlSafePayload, plist.BinaryFormat)
	if err != nil {
		t.Fatalf("marshal bplist: %v", err)
	}
	b64Lit := "b64:" + base64.StdEncoding.EncodeToString(bplistBytes)

	legPath := filepath.Join(dir, "b64_legacy.cherri")
	legSrc := "#workflowheader " + formatCherriDoubleQuotedString(encodeXMLSimpleHeader(t)) + "\n" +
		"rawAction(\"is.workflow.actions.gettext\", rawPlist(" + formatCherriDoubleQuotedString(b64Lit) + "))\n"
	if err := os.WriteFile(legPath, []byte(legSrc), 0644); err != nil {
		t.Fatal(err)
	}
	legSc := filepath.Join(dir, "b64_legacy.shortcut")
	if out, err := exec.Command(bin, legPath, "--legacy", "--skip-sign", "-o", legSc).CombinedOutput(); err != nil {
		t.Fatalf("legacy b64 compile failed: %v\n%s", err, string(out))
	}

	v2Path := filepath.Join(dir, "b64_v2.cherri")
	v2Src := "native.workflow(native.plist(" + formatCherriDoubleQuotedString(encodeXMLSimpleHeader(t)) + "), exact: true)\n" +
		"native.action(\"is.workflow.actions.gettext\", parameters: native.plist(" + formatCherriDoubleQuotedString(b64Lit) + "))\n"
	if err := os.WriteFile(v2Path, []byte(v2Src), 0644); err != nil {
		t.Fatal(err)
	}
	v2Sc := filepath.Join(dir, "b64_v2.shortcut")
	if out, err := exec.Command(bin, "compile", v2Path, "--skip-sign", "-o", v2Sc).CombinedOutput(); err != nil {
		t.Fatalf("v2 b64 compile failed: %v\n%s", err, string(out))
	}

	legDoc, err := decodeShortcutRawMap(mustReadFile(t, legSc))
	if err != nil {
		t.Fatal(err)
	}
	v2Doc, err := decodeShortcutRawMap(mustReadFile(t, v2Sc))
	if err != nil {
		t.Fatal(err)
	}
	if !plistValuesStrictlyEqual(legDoc, v2Doc) {
		t.Fatalf("legacy vs v2 b64: output mismatch at: %s", firstPlistDiffPath(legDoc, v2Doc, ""))
	}
	gotParams := v2Doc["WFWorkflowActions"].([]any)[0].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)
	if !plistValuesStrictlyEqual(xmlSafePayload, gotParams) {
		t.Fatalf("decoded b64 parameters mismatch at: %s", firstPlistDiffPath(xmlSafePayload, gotParams, ""))
	}

	// 3. Corrupt b64: / plist strings fail closed in both decodeRawPlistValueString and V2 CompileSourceToPlist.
	if _, err := decodeRawPlistValueString("b64:!!!not-base64!!!"); err == nil {
		t.Fatalf("expected error for invalid base64 payload")
	}
	if _, err := CompileSourceToPlist("bad_b64.cherri", `native.action(identifier: "is.workflow.actions.noop", parameters: native.plist("b64:!!!bad!!!"))`); err == nil {
		t.Fatalf("expected compile error for corrupt native.plist base64 payload")
	}
}

func encodeXMLSimpleHeader(t *testing.T) string {
	t.Helper()
	b, err := plist.Marshal(map[string]any{
		"WFWorkflowClientVersion": "2302.0.4",
		"WFWorkflowTypes":         []any{},
	}, plist.XMLFormat)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return b
}

func firstPlistDiffPath(a, b any, path string) string {
	switch x := a.(type) {
	case map[string]any:
		y, ok := b.(map[string]any)
		if !ok {
			return fmt.Sprintf("%s: map vs %T", path, b)
		}
		for k, xv := range x {
			yv, exists := y[k]
			if !exists {
				return fmt.Sprintf("%s.%s: missing in rebuilt", path, k)
			}
			if !plistValuesStrictlyEqual(xv, yv) {
				return firstPlistDiffPath(xv, yv, path+"."+k)
			}
		}
		for k := range y {
			if _, exists := x[k]; !exists {
				return fmt.Sprintf("%s.%s: extra in rebuilt", path, k)
			}
		}
	case []any:
		y, ok := b.([]any)
		if !ok || len(x) != len(y) {
			return fmt.Sprintf("%s: slice vs %T (len %d)", path, b, len(x))
		}
		for i := range x {
			if !plistValuesStrictlyEqual(x[i], y[i]) {
				return firstPlistDiffPath(x[i], y[i], fmt.Sprintf("%s[%d]", path, i))
			}
		}
	}
	return fmt.Sprintf("%s: %T(%#v) != %T(%#v)", path, a, a, b, b)
}

