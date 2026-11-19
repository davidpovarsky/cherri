package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
)

// Native ordered predicates already carry a deliberately opaque input
// attachment. Import must retain that reference boundary without asserting a
// scalar type or adding coercion actions.
func TestP15HybridOpaqueOrderedOperands(t *testing.T) {
	for _, operator := range []string{"<", "<=", ">", ">="} {
		for _, operand := range []struct{ name, prefix, expression, right string }{
			{"shortcut_input", "", `native.ref("ShortcutInput")`, "7"},
			{"opaque_output", "let input = run(\"SYNTHETIC_NOT_EXECUTED\")\n", `native.ref(input)`, "7"},
			{"opaque_rhs", "", "7", `native.ref("ShortcutInput")`},
			{"both_opaque", "let input = run(\"SYNTHETIC_NOT_EXECUTED\")\n", `native.ref("ShortcutInput")`, `native.ref(input)`},
		} {
			t.Run(operand.name+operator, func(t *testing.T) {
				source := operand.prefix + fmt.Sprintf("if modern: %s %s %s { text(\"then\") } else { text(\"else\") }", operand.expression, operator, operand.right)
				raw, err := CompileSourceToPlist("synthetic-import-ordered.cherri", source)
				if err != nil {
					t.Fatal(err)
				}
				doc, err := DecodeNativeShortcutDocument(raw)
				if err != nil {
					t.Fatal(err)
				}
				plan, _, _, preByDialect, err := verifyCorpusDualHybridWithPreOverlay(doc, "synthetic ordered input", true)
				if err != nil {
					t.Fatal(err)
				}
				if plan.Lifecycle == nil || plan.Lifecycle.DemotedActionCount != 0 {
					t.Fatalf("valid native predicate was demoted: %+v", plan.Lifecycle)
				}
				controls := 0
				for i, decision := range plan.Decisions {
					if decision.AppleIdentifier == "is.workflow.actions.conditional" {
						controls++
						if decision.Classification != "NATURAL_VERIFIED" {
							t.Fatalf("control marker %d stayed fallback: %s", i, decision.ReasonCode)
						}
					}
					if decision.Classification != "NATURAL_VERIFIED" {
						continue
					}
					for _, dialect := range []string{"legacy", "v2"} {
						if err := verifyPreOverlayActionSemanticParity("synthetic ordered input", dialect, i, doc.Actions[i], preByDialect[dialect][i].(map[string]any)); err != nil {
							t.Fatal(err)
						}
					}
				}
				if controls != 3 {
					t.Fatalf("expected three original native control markers, got %d", controls)
				}
			})
		}
	}
}

func TestP15HybridNativePromptDefaultReference(t *testing.T) {
	source := "let value = number(7)\nprompt(\"synthetic\", inputType: \"Number\", defaultValue: native.ref(value))"
	raw, err := CompileSourceToPlist("synthetic-import-prompt.cherri", source)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := DecodeNativeShortcutDocument(raw)
	if err != nil {
		t.Fatal(err)
	}
	plan, _, _, preByDialect, err := verifyCorpusDualHybridWithPreOverlay(doc, "synthetic attached default", true)
	if err != nil {
		t.Fatal(err)
	}
	if plan.NaturalVerifiedCount != 2 || plan.Lifecycle == nil || plan.Lifecycle.DemotedActionCount != 0 {
		t.Fatalf("native default attachment lost its natural representation: natural=%d lifecycle=%+v", plan.NaturalVerifiedCount, plan.Lifecycle)
	}
	for i := range doc.Actions {
		for _, dialect := range []string{"legacy", "v2"} {
			if err := verifyPreOverlayActionSemanticParity("synthetic attached default", dialect, i, doc.Actions[i], preByDialect[dialect][i].(map[string]any)); err != nil {
				t.Fatal(err)
			}
		}
	}
	// The native reference is explicit. Bare authored Numbers still obey the
	// canonical Text parameter contract.
	authored := `prompt("synthetic", inputType: "Number", defaultValue: 7)`
	diagnostics := p15TypeDiagnostics(t, authored)
	if err := p15ExactLiteralDiagnostic(diagnostics, authored, "7", analysis.CodeArgumentType); err != nil {
		t.Fatal(err)
	}
	if raw, err := CompileSourceToPlist("synthetic-authored-prompt.cherri", authored); err == nil || len(raw) != 0 || !strings.Contains(err.Error(), analysis.CodeArgumentType) {
		t.Fatalf("authored parameter check was weakened: bytes=%d err=%v", len(raw), err)
	}
}
