package main

import (
	"encoding/base64"
	"fmt"
	"strings"
	"testing"

	"howett.net/plist"
)

func p15RP52MobileWire(t *testing.T, response mobileCompileResponse, marker string, mainFallthrough bool) {
	t.Helper()
	if !response.OK || response.PlistBase64 == "" || response.SignedBase64 != "" {
		t.Fatalf("actual unsigned mobile compile failed: okay=%t mode=%s error=%s plist=%d signed=%d", response.OK, response.Mode, response.Error, len(response.PlistBase64), len(response.SignedBase64))
	}
	raw, err := base64.StdEncoding.DecodeString(response.PlistBase64)
	if err != nil {
		t.Fatal(err)
	}
	var root map[string]any
	if _, err := plist.Unmarshal(raw, &root); err != nil {
		t.Fatal(err)
	}
	rows, okay := root["WFWorkflowActions"].([]any)
	if !okay {
		t.Fatal("actual mobile output has no actions")
	}
	var actions []map[string]any
	for _, row := range rows {
		action, okay := row.(map[string]any)
		if !okay {
			t.Fatal("actual mobile output has malformed action")
		}
		actions = append(actions, action)
	}
	if err := p15U04PublicSourceWire(actions, marker, mainFallthrough); err != nil {
		t.Fatal(err)
	}
	// The expected mode comes from the authored test route, never from a response flag.
	if err := p15U04PublicSourceWire(actions, marker, !mainFallthrough); err == nil {
		t.Fatal("opposite main output contract accepted the actual mobile graph")
	}
	// A success flag alone cannot satisfy the observed payload/reference oracle.
	if err := p15U04PublicSourceWire(p15V02CorruptLastReference(t, actions, false, mainFallthrough), marker, mainFallthrough); err == nil {
		t.Fatal("wrong producer reference passed the actual mobile route oracle")
	}
}

func p15RP52LegacySources() []struct{ name, source string } {
	const body = "const route = text(\"RP52_LEGACY_REFERENCE\")\nshow(route)\n"
	return []struct{ name, source string }{
		{"plain", body},
		{"file_marker", "// cherri: legacy\n" + body},
		{"preprocessor", "#define from sharesheet\n" + body},
	}
}

func TestP15RP52DefaultMobileFrontend(t *testing.T) {
	const healthy = "let route = text(\"RP52_V2_DEFAULT\")\nshow(route)\n"
	routes := []struct {
		name    string
		compile func(string) mobileCompileResponse
	}{
		{"omitted", func(source string) mobileCompileResponse { return compileForMobile(source, "RP52 default", false) }},
		{"empty", func(source string) mobileCompileResponse {
			return compileForMobileMode(source, "RP52 empty", "", false)
		}},
		{"whitespace", func(source string) mobileCompileResponse {
			return compileForMobileMode(source, "RP52 whitespace", " \t\n", false)
		}},
		{"explicit_v2", func(source string) mobileCompileResponse {
			return compileForMobileMode(source, "RP52 explicit V2", "v2", false)
		}},
	}
	for _, route := range routes {
		t.Run(route.name+"_valid_v2", func(t *testing.T) {
			response := route.compile(healthy)
			p15RP52MobileWire(t, response, "RP52_V2_DEFAULT", true)
			if response.Mode != "v2" {
				t.Fatal("default did not report actual V2 selection", response.Mode)
			}
		})
		for _, specimen := range p15RP52LegacySources() {
			t.Run(route.name+"_reject_"+specimen.name, func(t *testing.T) {
				response := route.compile(specimen.source)
				if response.OK {
					p15RP52MobileWire(t, response, "RP52_LEGACY_REFERENCE", false)
					t.Fatalf("default/V2 public source route silently selected %q and compiled real Legacy source", response.Mode)
				}
				if response.PlistBase64 != "" || response.SignedBase64 != "" || !strings.Contains(response.Error, "E_LEGACY_SYNTAX") {
					t.Fatalf("old source produced bytes or lost migration diagnostic: mode=%q plist=%d signed=%d error=%s", response.Mode, len(response.PlistBase64), len(response.SignedBase64), response.Error)
				}
				recovered := route.compile(healthy)
				p15RP52MobileWire(t, recovered, "RP52_V2_DEFAULT", true)
				if recovered.Mode != "v2" {
					t.Fatal("rejected old source contaminated healthy default", recovered.Mode)
				}
			})
		}
	}
}

func TestP15RP52ExplicitLegacyReferenceModes(t *testing.T) {
	for _, specimen := range p15RP52LegacySources() {
		t.Run("explicit_legacy_"+specimen.name, func(t *testing.T) {
			response := compileForMobileMode(specimen.source, "RP52 trusted old reference", "legacy", false)
			p15RP52MobileWire(t, response, "RP52_LEGACY_REFERENCE", false)
			if response.Mode != "legacy" {
				t.Fatal("explicitly selected Legacy was not preserved", response.Mode)
			}
		})
	}
	// Explicit compatibility selection remains distinct from an omitted default.
	for _, specimen := range p15RP52LegacySources()[1:] {
		t.Run("explicit_auto_"+specimen.name, func(t *testing.T) {
			response := compileForMobileMode(specimen.source, "RP52 explicitly selected compatibility", "auto", false)
			p15RP52MobileWire(t, response, "RP52_LEGACY_REFERENCE", false)
			if response.Mode != "legacy" {
				t.Fatal("explicit compatibility route changed", response.Mode)
			}
		})
	}
	t.Run("invalid_mode", func(t *testing.T) {
		response := compileForMobileMode("show(\"RP52\")\n", "RP52 invalid mode", "RP52-no-such-mode", false)
		if response.OK || response.PlistBase64 != "" || response.SignedBase64 != "" || !strings.Contains(response.Error, "COMPILE_MODE_INVALID") {
			t.Fatal("invalid mode did not fail closed", fmt.Sprint(response))
		}
	})
	t.Run("recovery", func(t *testing.T) {
		response := compileForMobileMode("let route = text(\"RP52_V2_DEFAULT\")\nshow(route)\n", "RP52 final recovery", "v2", false)
		p15RP52MobileWire(t, response, "RP52_V2_DEFAULT", true)
	})
}
