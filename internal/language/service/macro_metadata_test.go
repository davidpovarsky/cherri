package service

import (
	"github.com/electrikmilk/cherri/internal/language/schema"
	"testing"
)

func TestCompilerMacrosReachCompletionFromSharedMetadata(t *testing.T) {
	svc := NewService(schema.DefaultRegistry())
	uri := "file:///macro-completion.cherri"
	svc.OpenDocument(uri, 1, "")
	items := svc.Complete(uri, 1, 1)
	counts := map[string]int{}
	for _, item := range items {
		counts[item.Label]++
		if item.Label == "measure" && (item.InsertText == "" || item.Documentation == "") {
			t.Fatal("scoped block completion lacks signature/docs")
		}
	}
	for _, name := range []string{"require", "measure", "debugLog"} {
		if counts[name] != 1 {
			t.Fatalf("%s completion count%d, want1", name, counts[name])
		}
	}
}
