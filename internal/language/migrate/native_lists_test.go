package migrate

import (
	"strings"
	"testing"
)

func TestNativeListReconstructionKeepsTypedReferences(t *testing.T) {
	input := "let first = number(10)\nlet second = number(20)\nlet values = list(listItem: [first, second])\n"
	got, err := ReconstructNativeListExpressions(input)
	if err != nil || got != "let first = number(10)\nlet second = number(20)\nlet values = [first, second]\n" {
		t.Fatalf("typed native List references changed: %v\n%s", err, got)
	}
	ordinary, _, err := MigrateSourceWithDiagnostics("const first = number(10)\nconst values = list(first)\n")
	if err != nil || !strings.Contains(ordinary, "list(listItem: [first])") {
		t.Fatalf("ordinary authored list migration lost its canonical text action: %v\n%s", err, ordinary)
	}
}

func TestNativeListReconstructionRetainsStandaloneAndScope(t *testing.T) {
	for _, tc := range []struct{ name, input, want string }{
		{"standalone", "list(listItem: [])\n", "let __nativeList1 = []\n"},
		{"collision", "let __nativeList1 = 7\nlist(listItem: [number(2)])\n", "let __nativeList1 = 7\nlet __nativeList2 = [number(2)]\n"},
		{"nested_call", "count(list(listItem: [number(2)]))\n", "count([number(2)])\n"},
		{"text_untouched", "text(\"list(listItem: [99])\")\n", "text(\"list(listItem: [99])\")\n"},
		{"binding_shadow", "let list = number(1)\nlist(listItem: [number(2)])\n", "let list = number(1)\nlist(listItem: [number(2)])\n"},
		{"function_shadow", "function list(listItem: List<Number>) -> List<Number> { return listItem }\nlist(listItem: [2])\n", "function list(listItem: List<Number>) -> List<Number> { return listItem }\nlist(listItem: [2])\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ReconstructNativeListExpressions(tc.input)
			if err != nil || got != tc.want {
				t.Fatalf("native List reconstruction changed scope or dropped output: %v\n%s", err, got)
			}
		})
	}
	if _, err := ReconstructNativeListExpressions("let broken = ["); err == nil {
		t.Fatal("invalid semantic import source accepted")
	}
}
