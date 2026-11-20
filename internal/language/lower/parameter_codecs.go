package lower

import (
	"github.com/electrikmilk/cherri/internal/language/backend"
	"github.com/electrikmilk/cherri/internal/language/ir"
	"github.com/electrikmilk/cherri/internal/language/syntax"
)

// The accepted-form contract takes precedence over a codec's default. Both
// canonical argument spellings use the same rule; a public custom declaration
// opts into reference materialization by explicitly declaring the token codec.
func parameterRequiresReference(codec, wireKey string, acceptedForms []string) bool {
	for _, form := range acceptedForms {
		if form == "literal" {
			return false
		}
	}
	return codec == backend.CodecToken || (wireKey == "WFInput" && codec != backend.CodecText)
}

// Reuse the existing native scalar producers and shared reference encoder.
// This consumes an already evaluated value and never evaluates an expression
// again or converts an existing reference into a second producer.
func (l *Lowerer) materializeCodecScalar(value backend.SemanticValue) *ir.AttachmentToken {
	switch value.Kind {
	case backend.ValBool:
		return l.materializeToAttachment(value.BoolVal)
	case backend.ValInt:
		return l.materializeToAttachment(value.IntVal)
	case backend.ValFloat:
		return l.materializeToAttachment(value.FloatVal)
	case backend.ValString:
		return l.materializeToAttachment(value.StrVal)
	default:
		return nil
	}
}

func (l *Lowerer) lowerCodecSemanticValue(expr syntax.Expression, codec, wireKey string, forms []string, consumesRuntimeValue bool) (backend.SemanticValue, error) {
	value, err := l.lowerSemanticValue(expr)
	if err != nil {
		return backend.SemanticValue{}, err
	}
	if consumesRuntimeValue && parameterRequiresReference(codec, wireKey, forms) {
		if token := l.materializeCodecScalar(value); token != nil {
			return l.toSemanticValue(token), nil
		}
	}
	return value, nil
}

func (l *Lowerer) lowerCustomCodecValue(declaration *syntax.ActionDecl, parameter string, value interface{}) interface{} {
	binding := declaration.Bindings[parameter]
	if binding.Codec == backend.CodecToken {
		if token := l.materializeCodecScalar(l.toSemanticValue(value)); token != nil {
			return token
		}
	}
	return value
}
