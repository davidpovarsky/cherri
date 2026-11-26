package main

import (
	"fmt"
	"testing"

	"github.com/electrikmilk/cherri/internal/shortcutcompare"
	"howett.net/plist"
)

// Literal options use the same authored value in analysis and lowering.
// Compare the complete native graph against the direct-literal form, so folding
// cannot introduce effects, change the guard, or replace the original producer.
func TestP15RequireConstantOptionsPreserveNativeGraph(t *testing.T) {
	for _, tc := range []struct{ name, folded, literal string }{
		{"type_concat", `type: "te" + "xt"`, `type: "text"`},
		{"type_template", `type: f"te{"xt"}"`, `type: "text"`},
		{"type_raw", `type: r" Text "`, `type: "text"`},
		{"nonempty_not", `nonEmpty: !false`, `nonEmpty: true`},
		{"nonempty_and", `nonEmpty: true && !false`, `nonEmpty: true`},
		{"nonempty_or", `nonEmpty: false || true`, `nonEmpty: true`},
		{"nonempty_equal", `nonEmpty: true == !false`, `nonEmpty: true`},
		{"nonempty_unequal", `nonEmpty: false != true`, `nonEmpty: true`},
		{"type_with_false_nonempty", `type: "te" + "xt", nonEmpty: !true`, `type: "text", nonEmpty: false`},
		{"named_both", `type: "te" + "xt", nonEmpty: !false`, `type: "text", nonEmpty: true`},
		{"positional_both", `"te" + "xt", !false`, `"text", true`},
	} {
		for _, profile := range []BuildProfile{BuildProfileDebug, BuildProfileRelease} {
			t.Run(fmt.Sprintf("%s/%s", tc.name, profile), func(t *testing.T) {
				documents := make([]map[string]any, 0, 2)
				for _, options := range []string{tc.folded, tc.literal} {
					source := `let checked = require(text("effect-once"), ` + options + ")\ngetVariable(checked)\n"
					raw, err := CompileSourceToPlistWithOptions("require-constant-options.cherri", source, CompileOptions{BuildProfile: profile})
					if err != nil {
						t.Fatal(err)
					}
					var document map[string]any
					if _, err := plist.Unmarshal(raw, &document); err != nil {
						t.Fatal(err)
					}
					producers := 0
					for _, row := range document["WFWorkflowActions"].([]any) {
						action := row.(map[string]any)
						if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.gettext" &&
							p15Param(action)["WFTextActionText"] == "effect-once" {
							producers++
						}
					}
					if producers != 1 {
						t.Fatalf("authored producer evaluated %d times", producers)
					}
					documents = append(documents, document)
				}
				if comparison := shortcutcompare.CompareDocumentsStrict(documents[0], documents[1]); !comparison.Equal {
					t.Fatalf("folded options changed native effects or values: %+v", comparison.Differences)
				}
			})
		}
	}
}
