/*
 * Copyright (c) Cherri
 */

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const plistTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>WFWorkflowClientVersion</key>
	<string>%s</string>
	<key>WFWorkflowTypes</key>
	<array><string>ActionExtension</string></array>
	<key>WFWorkflowActions</key>
	<array>
		<dict>
			<key>WFWorkflowActionIdentifier</key>
			<string>is.workflow.actions.getvariable</string>
			<key>WFWorkflowActionParameters</key>
			<dict>
				<key>UUID</key>
				<string>%s</string>
				<key>WFVariable</key>
				<dict>
					<key>Value</key>
					<dict>
						<key>Type</key>
						<string>Variable</string>
						<key>OutputName</key>
						<string>Provided Input</string>
						<key>OutputUUID</key>
						<string>%s</string>
					</dict>
					<key>WFSerializationType</key>
					<string>WFTextTokenAttachment</string>
				</dict>
			</dict>
		</dict>
	</array>
</dict>
</plist>
`

func writeTempShortcut(t *testing.T, dir, name, clientVersion, actionUUID, outputUUID string) string {
	t.Helper()
	content := fmt.Sprintf(plistTemplate, clientVersion, actionUUID, outputUUID)
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestComparatorIgnoresDynamicValues(t *testing.T) {
	dir := t.TempDir()
	a := writeTempShortcut(t, dir, "a.shortcut", "3036.0.4.2",
		"11111111-2222-3333-4444-555555555555",
		"99999999-8888-7777-6666-555555555555")
	b := writeTempShortcut(t, dir, "b.shortcut", "4528.0.4.2",
		"aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
		"ffffffff-0000-1111-2222-333333333333")

	result, err := CompareShortcuts(a, b)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Equal {
		t.Fatalf("documents differing only in volatile metadata and UUIDs must be equal: %+v", result.Differences)
	}
}

func TestComparatorDetectsSemanticDifferences(t *testing.T) {
	dir := t.TempDir()
	a := writeTempShortcut(t, dir, "a.shortcut", "3036.0.4.2",
		"11111111-2222-3333-4444-555555555555",
		"99999999-8888-7777-6666-555555555555")

	b := writeTempShortcut(t, dir, "b.shortcut", "3036.0.4.2",
		"11111111-2222-3333-4444-555555555555",
		"99999999-8888-7777-6666-555555555555")
	raw, err := os.ReadFile(b)
	if err != nil {
		t.Fatal(err)
	}
	changed := strings.Replace(string(raw), "Provided Input", "Renamed Output", 1)
	if err = os.WriteFile(b, []byte(changed), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := CompareShortcuts(a, b)
	if err != nil {
		t.Fatal(err)
	}
	if result.Equal || len(result.Differences) == 0 {
		t.Fatal("meaningful value changes must be reported as differences")
	}
	found := false
	for _, difference := range result.Differences {
		if strings.Contains(difference.Path, "OutputName") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a difference on the OutputName path, got %+v", result.Differences)
	}

	printComparison(result, false)
	printComparison(result, true)
}

func TestCanonicalizerPreservesRelationshipsAcrossDocuments(t *testing.T) {
	docA := map[string]any{
		"a": map[string]any{"ref": "11111111-2222-3333-4444-555555555555"},
		"b": "11111111-2222-3333-4444-555555555555",
	}
	docB := map[string]any{
		"a": map[string]any{"ref": "ffffffff-0000-1111-2222-333333333333"},
		"b": "ffffffff-0000-1111-2222-333333333333",
	}
	canonicalizerA := &canonicalizer{uuidSeen: map[string]string{}}
	canonicalizerB := &canonicalizer{uuidSeen: map[string]string{}}
	if fmt.Sprint(canonicalizerA.walk(docA)) != fmt.Sprint(canonicalizerB.walk(docB)) {
		t.Fatal("same relationship structure with different UUID values must canonicalize identically")
	}

	docC := map[string]any{
		"a": map[string]any{"ref": "ffffffff-0000-1111-2222-333333333333"},
		"b": "eeeeeeee-0000-1111-2222-333333333333",
	}
	canonicalizerC := &canonicalizer{uuidSeen: map[string]string{}}
	if fmt.Sprint(canonicalizerA.walk(docA)) == fmt.Sprint(canonicalizerC.walk(docC)) {
		t.Fatal("different relationship structures must not canonicalize identically")
	}
}
