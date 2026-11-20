package lower

import (
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/source"
	"github.com/electrikmilk/cherri/internal/language/syntax"
)

// A partial session does not establish the action sequence by returning a UUID.
// IR-only lowering must still reject a preceding action with no value contract.
func TestLiteralCarrierInputRequiresProvenSequence(t *testing.T) {
	cases := []struct {
		name, previous string
		partialSession bool
	}{
		{"ir_void", "wait(0)\n", false},
		{"ir_unknown_native", "native.action(\"is.workflow.actions.p15unknown\", {})\n", false},
		{"partial_session", "text(\"SEED\")\n", true},
	}
	for _, tc := range cases {
		for _, explicit := range []bool{false, true} {
			name := tc.name + "_implicit"
			if explicit {
				name = tc.name + "_explicit_control"
			}
			t.Run(name, func(t *testing.T) {
				input := ""
				if explicit {
					input = "\"WFInput\":suffix,"
				}
				code := "let suffix=text(\"SUFFIX\")\n" + tc.previous +
					"native.action(\"is.workflow.actions.setitemname\", {" + input + "\"WFName\":f\"\uFFFC{suffix}\"})\n"
				parser := syntax.NewParser(source.NewFile("s01-boundary.cherri", "file:///s01-boundary.cherri", 1, code))
				program := parser.ParseProgram()
				if len(parser.Errors()) != 0 {
					t.Fatalf("parse: %v", parser.Errors())
				}
				lowerer := NewLowerer(schema.DefaultRegistry())
				if tc.partialSession {
					lowerer = NewLowererWithSession(schema.DefaultRegistry(), &testMockSession{meta: make(map[string]any)})
				}
				workflow, err := lowerer.LowerProgram(program)
				if explicit {
					if err != nil || workflow == nil {
						t.Fatalf("explicit input: workflow=%v error=%v", workflow, err)
					}
				} else if err == nil || workflow != nil || !strings.Contains(err.Error(), "S01_IMPLICIT_INPUT_UNAVAILABLE") {
					t.Fatalf("unproven implicit input was accepted: workflow=%v error=%v", workflow != nil, err)
				}
			})
		}
	}
}
