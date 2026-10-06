package main

import (
	"strings"
	"testing"
)

func TestMigrateSource(t *testing.T) {
	legacy := `#include "basic.cherri"
const title = "My Shortcut"
@count = 1
@count = 2
@msg = "Count is {count}"
show(@msg)
`
	migrated := MigrateSource(legacy)

	if strings.Contains(migrated, "#include") {
		t.Errorf("migrated code should not contain #include")
	}
	if strings.Contains(migrated, "@") {
		t.Errorf("migrated code should not contain @, got:\n%s", migrated)
	}
	if strings.Contains(migrated, "const ") {
		t.Errorf("migrated code should not contain const, got:\n%s", migrated)
	}
	if !strings.Contains(migrated, "let title =") {
		t.Errorf("migrated code should contain 'let title =', got:\n%s", migrated)
	}
	if !strings.Contains(migrated, "var count =") {
		t.Errorf("migrated code should contain 'var count =' (reassigned), got:\n%s", migrated)
	}
	if !strings.Contains(migrated, `f"Count is {count}"`) {
		t.Errorf("migrated code should contain f-string, got:\n%s", migrated)
	}
}
