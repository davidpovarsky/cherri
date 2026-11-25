package lower

import (
	"testing"

	"github.com/electrikmilk/cherri/internal/language/syntax"
	"github.com/electrikmilk/cherri/internal/language/types"
)

func TestTaggedImportedBoundaryIsLocal(t *testing.T) {
	for _, provenance := range []bool{false, true} {
		expr := &syntax.IdentExpr{Name: "authored"}
		lowerer := &Lowerer{hasImportProvenance: provenance,
			bindings:  map[string]BindingRef{"authored": {Type: types.JsonNull, TaggedTransport: types.JsonNull}},
			exprTypes: map[syntax.Expression]types.Type{expr: types.JsonNull}}
		call := &syntax.CallExpr{PrimaryArg: expr}
		if lowerer.validateTaggedNativeBoundary("show", call) == nil {
			t.Fatal("file-level import provenance authorized a new authored private value")
		}
		if lowerer.validateTaggedNativeBoundary("native.action", call) == nil {
			t.Fatal("private reference escaped through raw native metadata")
		}
	}
}

func TestTaggedRawNoneAndNullRemainDistinct(t *testing.T) {
	lowerer := &Lowerer{}
	if err := lowerer.validateTaggedRawReference(&syntax.LiteralExpr{Type: syntax.TokenNone}); err != nil {
		t.Fatal("existing raw None syntax was changed", err)
	}
	if err := lowerer.validateTaggedRawReference(&syntax.LiteralExpr{Type: syntax.TokenNull}); err == nil {
		t.Fatal("JSON null was authorized as raw plist data")
	}
}
