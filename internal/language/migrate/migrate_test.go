package migrate_test

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/migrate"
)

func TestMigrateTriggersSevenFamilies(t *testing.T) {
	legacy := `#trigger battery 80
#trigger screenshot photos, files
#trigger wifi joined
#trigger bluetooth connect
#trigger stageManager on
#trigger display connect
#trigger charging connect
show("Triggers Test")`

	migrated, diags, err := migrate.MigrateSourceWithDiagnostics(legacy)
	if err != nil {
		t.Fatalf("unexpected migration error: %v (diags: %v)", err, diags)
	}

	expectedTriggers := []string{
		"trigger battery(level: 80)",
		`trigger screenshot(locations: ["photos", "files"])`,
		"trigger wifi(event: .joined)",
		"trigger bluetooth(event: .connect)",
		"trigger stageManager(event: .on)",
		"trigger display(event: .connect)",
		"trigger charging(event: .connect)",
	}

	for _, trig := range expectedTriggers {
		if !strings.Contains(migrated, trig) {
			t.Errorf("expected migrated source to contain %q, but got:\n%s", trig, migrated)
		}
	}
}

func TestMigrateUnknownTriggerFailsClosed(t *testing.T) {
	legacy := `#trigger appLaunch "com.apple.mobilesafari"
show("test")`

	_, diags, err := migrate.MigrateSourceWithDiagnostics(legacy)
	if err == nil {
		t.Fatalf("expected error for unknown trigger family, but got nil")
	}
	if len(diags) == 0 {
		t.Fatalf("expected at least 1 diagnostic, got 0")
	}
	if !strings.Contains(diags[0].Message, "unknown trigger family") {
		t.Errorf("diagnostic message %q does not mention unknown trigger family", diags[0].Message)
	}
}

func TestMigrateUnknownDirectiveFailsClosed(t *testing.T) {
	legacy := `#foobar something
show("test")`

	_, diags, err := migrate.MigrateSourceWithDiagnostics(legacy)
	if err == nil {
		t.Fatalf("expected error for unknown directive #foobar, but got nil")
	}
	if len(diags) == 0 {
		t.Fatalf("expected diagnostic, got none")
	}
	if !strings.Contains(diags[0].Message, "unrecognized preprocessor directive") {
		t.Errorf("unexpected diag message: %v", diags[0].Message)
	}
}

func TestMigrateRefPreservation(t *testing.T) {
	refJSON := `{"fileLocation":{"fileProviderDomainID":"com.apple.files","relativeSubpath":"doc.pdf"}}`
	encoded := base64.StdEncoding.EncodeToString([]byte(refJSON))

	legacy := "#ref myDoc " + encoded + "\nshow(myDoc)"
	migrated, diags, err := migrate.MigrateSourceWithDiagnostics(legacy)
	if err != nil {
		t.Fatalf("unexpected error: %v (diags: %v)", err, diags)
	}

	if !strings.Contains(migrated, "let myDoc = native.plist(") {
		t.Errorf("expected native.plist reference declaration, got:\n%s", migrated)
	}
	if !strings.Contains(migrated, "fileProviderDomainID") {
		t.Errorf("expected decoded JSON payload in native.plist, got:\n%s", migrated)
	}
}

func TestMigrateCorruptRefFailsClosed(t *testing.T) {
	legacy := "#ref corruptRef NOT_VALID_BASE64_!@#\nshow(corruptRef)"
	_, diags, err := migrate.MigrateSourceWithDiagnostics(legacy)
	if err == nil {
		t.Fatalf("expected error for corrupt reference, got nil")
	}
	if len(diags) == 0 {
		t.Fatalf("expected diagnostic for corrupt reference")
	}
}

func TestMigrateMetadataAndDefines(t *testing.T) {
	legacy := `#define name "My Test"
#define icon apple
#define color red
#define version 27
#define noinput getclipboard
#define inputs image, text
#define outputs app, file
show("Hello")`

	migrated, diags, err := migrate.MigrateSourceWithDiagnostics(legacy)
	if err != nil {
		t.Fatalf("unexpected error: %v (diags: %v)", err, diags)
	}

	if !strings.Contains(migrated, `shortcut "My Test" {`) {
		t.Errorf("missing shortcut header in:\n%s", migrated)
	}
	if !strings.Contains(migrated, "version: 27") {
		t.Errorf("missing version: 27 in:\n%s", migrated)
	}
	if !strings.Contains(migrated, "noinput: getclipboard") {
		t.Errorf("missing noinput: getclipboard in:\n%s", migrated)
	}
	if !strings.Contains(migrated, "WFAppStoreAppContentItem") {
		t.Errorf("expected outputs 'app' to be mapped to WFAppStoreAppContentItem in:\n%s", migrated)
	}
}

func TestMigrateStdfuncAndIncludes(t *testing.T) {
	legacy := `#include 'stdfunc'
#include 'custom/helper'
#include 'actions/web'
show("Test")`

	migrated, diags, err := migrate.MigrateSourceWithDiagnostics(legacy)
	if err != nil {
		t.Fatalf("unexpected error: %v (diags: %v)", err, diags)
	}

	if !strings.Contains(migrated, `import "stdfunc" as stdfunc`) {
		t.Errorf("expected import stdfunc, got:\n%s", migrated)
	}
	if !strings.Contains(migrated, `import "custom/helper" as helper`) {
		t.Errorf("expected import custom/helper, got:\n%s", migrated)
	}
	if strings.Contains(migrated, "actions/web") {
		t.Errorf("standard action include should not be imported, got:\n%s", migrated)
	}
}

func TestMigrateTypedVariablesAndRepeats(t *testing.T) {
	legacy := `@counter: number
@counter = 0
repeat i for 5 {
    @counter += 1
}`

	migrated, diags, err := migrate.MigrateSourceWithDiagnostics(legacy)
	if err != nil {
		t.Fatalf("unexpected error: %v (diags: %v)", err, diags)
	}

	if !strings.Contains(migrated, "var counter: Number") {
		t.Errorf("expected var counter: Number, got:\n%s", migrated)
	}
	if !strings.Contains(migrated, "repeat 5 as i") {
		t.Errorf("expected repeat 5 as i, got:\n%s", migrated)
	}
}

func TestMigrateMenus(t *testing.T) {
	legacy := `menu "Select Action" {
    item "One":
        show("First")
    item "Two":
        show("Second")
}`

	migrated, diags, err := migrate.MigrateSourceWithDiagnostics(legacy)
	if err != nil {
		t.Fatalf("unexpected error: %v (diags: %v)", err, diags)
	}

	if !strings.Contains(migrated, `case "One" {`) || !strings.Contains(migrated, `case "Two" {`) {
		t.Errorf("expected case blocks in menu, got:\n%s", migrated)
	}
}

func TestMigrateCopyPastePreserved(t *testing.T) {
	legacy := `copy carbon {
    show("copied")
}
paste carbon`

	migrated, diags, err := migrate.MigrateSourceWithDiagnostics(legacy)
	if err != nil {
		t.Fatalf("unexpected error: %v (diags: %v)", err, diags)
	}

	if !strings.Contains(migrated, "copy carbon {") || !strings.Contains(migrated, "paste carbon") {
		t.Errorf("expected copy/paste to be preserved, got:\n%s", migrated)
	}
}
