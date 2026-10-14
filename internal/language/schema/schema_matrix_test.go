/*
 * Copyright (c) Cherri
 */

package schema

import (
	"strings"
	"testing"
)

// GEN-001: generated schema complex static map preserves nested map, no "map[..." stringification
func TestGEN001_StaticParametersNestedMapsPreserved(t *testing.T) {
	reg := DefaultRegistry()
	if reg == nil {
		t.Fatalf("DefaultRegistry returned nil")
	}

	// Verify that in any action with StaticParameters, no values are formatted as "map[..."
	for _, act := range reg.AllActions() {
		for key, val := range act.StaticParameters {
			if s, isStr := val.(string); isStr {
				if strings.HasPrefix(s, "map[") {
					t.Fatalf("action %q static parameter %q is corrupted as stringified map: %s", act.CallableName, key, s)
				}
				if strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]") && strings.Contains(s, " ") {
					// Check for stringified slices
					if strings.Contains(s, "map[") {
						t.Fatalf("action %q static parameter %q contains stringified map in slice: %s", act.CallableName, key, s)
					}
				}
			}
		}
	}
}

// GEN-002: mutate canonical variant/default/AppIntent/literal changes fingerprint
func TestGEN002_FingerprintDetectsMutations(t *testing.T) {
	baseAction := &ActionSchema{
		ID:             "testAct",
		CallableName:   "testAct",
		AppleIdentifier: "is.workflow.actions.test",
		OutputTypeName: "Text",
		Docs:           ActionDocs{Title: "Test Action"},
		Parameters: []ParameterSchema{
			{
				ID:           "text",
				Label:        "text",
				TypeName:     "Text",
				WireKey:      "WFText",
				DefaultValue: "Initial",
			},
		},
	}

	regBase := NewRegistry([]*ActionSchema{baseAction}, nil)
	baseFP := regBase.Fingerprint()

	// 1. Mutate DefaultValue
	mutDefault := *baseAction
	mutDefault.Parameters = []ParameterSchema{
		{
			ID:           "text",
			Label:        "text",
			TypeName:     "Text",
			WireKey:      "WFText",
			DefaultValue: "Changed",
		},
	}
	regDefault := NewRegistry([]*ActionSchema{&mutDefault}, nil)
	if regDefault.Fingerprint() == baseFP {
		t.Fatalf("fingerprint did not change after mutating DefaultValue")
	}

	// 2. Mutate StaticParameters
	mutStatic := *baseAction
	mutStatic.StaticParameters = map[string]any{"Key": "Value"}
	regStatic := NewRegistry([]*ActionSchema{&mutStatic}, nil)
	if regStatic.Fingerprint() == baseFP {
		t.Fatalf("fingerprint did not change after adding StaticParameters")
	}

	// 3. Mutate AppIntent
	mutIntent := *baseAction
	mutIntent.AppIntent = &AppIntentDescriptor{
		Name:             "IntentName",
		BundleIdentifier: "com.apple.test",
	}
	regIntent := NewRegistry([]*ActionSchema{&mutIntent}, nil)
	if regIntent.Fingerprint() == baseFP {
		t.Fatalf("fingerprint did not change after adding AppIntent")
	}

	// 4. Mutate Variadic
	mutVariadic := *baseAction
	mutVariadic.Parameters = []ParameterSchema{
		{
			ID:           "text",
			Label:        "text",
			TypeName:     "Text",
			WireKey:      "WFText",
			DefaultValue: "Initial",
			Variadic:     true,
		},
	}
	regVariadic := NewRegistry([]*ActionSchema{&mutVariadic}, nil)
	if regVariadic.Fingerprint() == baseFP {
		t.Fatalf("fingerprint did not change after mutating Variadic flag")
	}

	// 5. Mutate MacOnly
	mutMacOnly := *baseAction
	mutMacOnly.MacOnly = true
	regMacOnly := NewRegistry([]*ActionSchema{&mutMacOnly}, nil)
	if regMacOnly.Fingerprint() == baseFP {
		t.Fatalf("fingerprint did not change after mutating MacOnly flag")
	}

	// 6. Mutate NonMacOnly
	mutNonMacOnly := *baseAction
	mutNonMacOnly.NonMacOnly = true
	regNonMacOnly := NewRegistry([]*ActionSchema{&mutNonMacOnly}, nil)
	if regNonMacOnly.Fingerprint() == baseFP {
		t.Fatalf("fingerprint did not change after mutating NonMacOnly flag")
	}

	// 7. Mutate MinVersion
	mutMinVer := *baseAction
	mutMinVer.MinVersion = 17.0
	regMinVer := NewRegistry([]*ActionSchema{&mutMinVer}, nil)
	if regMinVer.Fingerprint() == baseFP {
		t.Fatalf("fingerprint did not change after mutating MinVersion")
	}

	// 8. Mutate MaxVersion
	mutMaxVer := *baseAction
	mutMaxVer.MaxVersion = 26.0
	regMaxVer := NewRegistry([]*ActionSchema{&mutMaxVer}, nil)
	if regMaxVer.Fingerprint() == baseFP {
		t.Fatalf("fingerprint did not change after mutating MaxVersion")
	}

	// 9. Mutate Variant
	mutVariant := *baseAction
	mutVariant.Variant = "secondary"
	regVariant := NewRegistry([]*ActionSchema{&mutVariant}, nil)
	if regVariant.Fingerprint() == baseFP {
		t.Fatalf("fingerprint did not change after mutating Variant")
	}

	// 10. Mutate AcceptedForms
	mutForms := *baseAction
	mutForms.Parameters = []ParameterSchema{
		{
			ID:            "text",
			Label:         "text",
			TypeName:      "Text",
			WireKey:       "WFText",
			DefaultValue:  "Initial",
			AcceptedForms: []string{"literal", "tokenString"},
		},
	}
	regForms := NewRegistry([]*ActionSchema{&mutForms}, nil)
	if regForms.Fingerprint() == baseFP {
		t.Fatalf("fingerprint did not change after mutating AcceptedForms")
	}

	// 11. Mutate OmissionPolicy
	mutOmit := *baseAction
	mutOmit.Parameters = []ParameterSchema{
		{
			ID:             "text",
			Label:          "text",
			TypeName:       "Text",
			WireKey:        "WFText",
			DefaultValue:   "Initial",
			OmissionPolicy: OmitPolicyOmit,
		},
	}
	regOmit := NewRegistry([]*ActionSchema{&mutOmit}, nil)
	if regOmit.Fingerprint() == baseFP {
		t.Fatalf("fingerprint did not change after mutating OmissionPolicy")
	}
}

// GEN-003: duplicate Apple identifier retains each named variant separately
func TestGEN003_DuplicateAppleIdentifierRetainsAllVariants(t *testing.T) {
	reg := DefaultRegistry()
	if reg == nil {
		t.Fatalf("DefaultRegistry returned nil")
	}

	// Test representative group: base64Encode vs base64Decode
	encodeAct, hasEncode := reg.LookupAction("base64Encode")
	if !hasEncode {
		t.Fatalf("base64Encode not found in registry")
	}
	decodeAct, hasDecode := reg.LookupAction("base64Decode")
	if !hasDecode {
		t.Fatalf("base64Decode not found in registry")
	}

	if encodeAct.CallableName == decodeAct.CallableName {
		t.Fatalf("base64Encode and base64Decode have same callable name")
	}
	if encodeAct.AppleIdentifier != decodeAct.AppleIdentifier {
		t.Fatalf("expected both to share Apple identifier %q, got %q and %q",
			"is.workflow.actions.base64encode", encodeAct.AppleIdentifier, decodeAct.AppleIdentifier)
	}

	// LookupByIdentifier returns BOTH variants
	variants := reg.LookupByIdentifier(encodeAct.AppleIdentifier)
	if len(variants) < 2 {
		t.Fatalf("expected at least 2 variants for %s, got %d", encodeAct.AppleIdentifier, len(variants))
	}

	foundEncode := false
	foundDecode := false
	for _, v := range variants {
		if v.CallableName == "base64Encode" {
			foundEncode = true
		}
		if v.CallableName == "base64Decode" {
			foundDecode = true
		}
	}
	if !foundEncode || !foundDecode {
		t.Fatalf("variants did not contain both base64Encode and base64Decode: %v", variants)
	}
}
