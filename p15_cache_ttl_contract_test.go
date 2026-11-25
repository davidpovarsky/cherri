package main

import (
	"fmt"
	"strings"
	"testing"
)

func p15CacheNativeWireError(actions []ShortcutAction) error {
	conditions := 0
	for index, action := range actions {
		if action.WFWorkflowActionIdentifier != "is.workflow.actions.conditional" {
			continue
		}
		p := action.WFWorkflowActionParameters
		if fmt.Sprint(p["WFControlFlowMode"]) != "0" {
			continue
		}
		conditions++
		input, okay := p["WFInput"].(map[string]any)
		if !okay || len(input) != 2 || input["Type"] != "Variable" {
			return fmt.Errorf("conditional %d has no exact native Variable envelope", index)
		}
		token, okay := input["Variable"].(map[string]any)
		if !okay || len(token) != 2 || token["WFSerializationType"] != "WFTextTokenAttachment" {
			return fmt.Errorf("conditional %d has no typed attachment", index)
		}
		reference, okay := token["Value"].(map[string]any)
		if !okay || reference["Type"] != "ActionOutput" || reference["OutputUUID"] == nil || reference["OutputUUID"] == "" {
			return fmt.Errorf("conditional %d has no actual prior output reference", index)
		}
		_, number := p["WFNumberValue"]
		_, text := p["WFConditionalActionString"]
		if number == text {
			return fmt.Errorf("conditional %d lost its exact typed comparison field", index)
		}
	}
	if conditions == 0 {
		return fmt.Errorf("persistent cache has no native conditions")
	}
	return nil
}
func TestP15CacheNativeConditionEncoding(t *testing.T) {
	for _, index := range []int{0, 2, 4} {
		tc := p15CacheScalarCases()[index]
		for _, legacy := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/legacy=%t", tc.name, legacy), func(t *testing.T) {
				actual, err := p15CacheCompileRecord(t, p15CacheScalarSource(tc, legacy, 300), legacy, BuildProfileRelease)
				if err != nil {
					t.Fatal(err)
				}
				if err := p15CacheNativeWireError(actual); err != nil {
					t.Fatal(err)
				}
				condition := -1
				for i, a := range actual {
					if a.WFWorkflowActionIdentifier == "is.workflow.actions.conditional" && fmt.Sprint(a.WFWorkflowActionParameters["WFControlFlowMode"]) == "0" {
						condition = i
						break
					}
				}
				if condition < 0 {
					t.Fatal("no condition")
				}
				original := actual[condition].WFWorkflowActionParameters["WFInput"]
				corruptions := map[string]any{
					"dictionary_instead_of_envelope": map[string]any{"WFSerializationType": "WFDictionaryFieldValue", "Value": original},
					"bare_attachment":                original.(map[string]any)["Variable"],
					"missing_reference":              map[string]any{"Type": "Variable"},
					"wrong_envelope_kind":            map[string]any{"Type": "ActionOutput", "Variable": original.(map[string]any)["Variable"]},
					"extra_field":                    map[string]any{"Type": "Variable", "Variable": original.(map[string]any)["Variable"], "Forged": true},
					"text_attachment":                map[string]any{"Type": "Variable", "Variable": map[string]any{"WFSerializationType": "WFTextTokenString", "Value": map[string]any{"string": "value"}}},
				}
				for name, value := range corruptions {
					t.Run(name, func(t *testing.T) {
						actual[condition].WFWorkflowActionParameters["WFInput"] = value
						defer func() { actual[condition].WFWorkflowActionParameters["WFInput"] = original }()
						if err := p15CacheNativeWireError(actual); err == nil {
							t.Fatal("changed native input envelope was accepted")
						}
					})
				}
			})
		}
	}
}
func TestP15CacheRejectsUnqualifiedPersistence(t *testing.T) {
	cases := []struct{ name, body, legacyBody, options, legacyOptions string }{
		{"list", "yield [1,2]", "yield [1,2]", "", ""},
		{"dictionary", "yield {\"x\":1}", "yield {\"x\":1}", "", ""},
		{"optional_absence", "yield none", "yield nil", "", ""},
		{"json_null", "yield null", "yield nil", "", ""},
		{"date", "yield CurrentDate", "yield CurrentDate", "", ""},
		{"native_file", "yield getType(text(\"file\"),\"public.data\")", "yield getType(text(\"file\"),\"public.data\")", "", ""},
		{"void", "wait(1)", "wait(1)", "", ""},
		{"negative_ttl", "yield 1", "yield 1", "key:\"key\",ttl:-1", "\"key\",-1"},
		{"empty_key", "yield 1", "yield 1", "key:\"\",ttl:300", "\"\",300"},
		{"sensitive_without_permission", "yield 1", "yield 1", "key:\"key\",ttl:300,sensitive:true", "\"key\",300,false,true,false"},
		{"wrong_global_type", "yield 1", "yield 1", "key:\"key\",ttl:300,global:\"yes\"", "\"key\",300,\"yes\""},
		{"unknown_option", "yield 1", "yield 1", "key:\"key\",ttl:300,unknown:true", "\"key\",300,false,false,false,true"},
	}
	for _, tc := range cases {
		for _, legacy := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/legacy=%t", tc.name, legacy), func(t *testing.T) {
				options, body := tc.options, tc.body
				if options == "" {
					options = "key:\"key\",ttl:300"
				}
				source := "shortcut \"cache boundary\" { version:\"27\" }\nlet value=cache(" + options + "){\n" + body + "\n}\nreturn value\n"
				if legacy {
					options, body = tc.legacyOptions, tc.legacyBody
					if options == "" {
						options = "\"key\",300"
					}
					source = "#define version 27\nconst value=cache(" + options + "){\n" + body + "\n}\ngetVariable(value)\n"
				}
				_, err := p15CacheCompileRecord(t, source, legacy, BuildProfileRelease)
				if err == nil {
					t.Fatal("unqualified persistence was accepted")
				}
				message := err.Error()
				if strings.Contains(message, "Undefined action 'cache") || strings.Contains(message, "E_BLOCK_UNSUPPORTED") {
					t.Fatalf("cache support itself disappeared: %v", err)
				}
				if strings.Contains(message, "runtime error") || strings.Contains(message, "panic") {
					t.Fatalf("boundary panicked: %v", err)
				}
			})
		}
	}
}
func TestP15CacheVersionAndLiteralBoundaries(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		base := p15CacheScalarSource(p15CacheScalarCases()[2], legacy, 300)
		for _, mode := range []string{"target26", "dynamic_ttl", "missing_body", "escaped_local"} {
			t.Run(fmt.Sprintf("%s/legacy=%t", mode, legacy), func(t *testing.T) {
				source := base
				switch mode {
				case "target26":
					source = strings.Replace(source, "27", "26", 1)
				case "dynamic_ttl":
					if legacy {
						source = strings.Replace(source, "const cached", "const seconds=300\nconst cached", 1)
						source = strings.Replace(source, "\"P15_SCALAR_KEY\", 300", "\"P15_SCALAR_KEY\", seconds", 1)
					} else {
						source = strings.Replace(source, "let cached", "let seconds=300\nlet cached", 1)
						source = strings.Replace(source, "ttl:300", "ttl:seconds", 1)
					}
				case "missing_body":
					if legacy {
						source = "#define version 27\ncache(\"key\",300)\n"
					} else {
						source = "shortcut \"x\" { version:\"27\" }\ncache(key:\"key\",ttl:300)\n"
					}
				case "escaped_local":
					if legacy {
						source = "#define version 27\nconst value=cache(\"key\",300){\nconst onlyLocal=1\nyield onlyLocal\n}\ngetVariable(onlyLocal)\n"
					} else {
						source = "shortcut \"x\" { version:\"27\" }\nlet value=cache(key:\"key\",ttl:300){let onlyLocal=1\nyield onlyLocal}\nreturn onlyLocal"
					}
				}
				_, err := p15CacheCompileRecord(t, source, legacy, BuildProfileRelease)
				if err == nil {
					t.Fatal("invalid target, dynamic option or escaped scope accepted")
				}
				if strings.Contains(err.Error(), "Undefined action 'cache") || strings.Contains(err.Error(), "E_BLOCK_UNSUPPORTED") || strings.Contains(err.Error(), "runtime error") {
					t.Fatal(err)
				}
			})
		}
	}
}
