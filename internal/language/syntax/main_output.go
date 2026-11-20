package syntax

import "github.com/electrikmilk/cherri/internal/language/source"

// MainOutputMode controls only natural main-body fallthrough. An authored
// return and an explicit native.workflow retain their existing semantics.
type MainOutputMode string

const (
	MainOutputExplicit MainOutputMode = "explicit"
	MainOutputLegacy   MainOutputMode = "legacy"
)

type MainOutputModeError struct {
	Span    source.Span
	Message string
}

func (e *MainOutputModeError) Error() string { return e.Message }

// ResolveMainOutputMode is shared by analysis and checked lowering so a
// service result and a direct lowering call cannot disagree about the boundary.
// The annotation is compiler metadata and is never an Apple plist field.
func ResolveMainOutputMode(program *Program) (MainOutputMode, *MainOutputModeError) {
	mode := MainOutputExplicit
	seen := false
	if program == nil {
		return mode, nil
	}
	for _, declaration := range program.Declarations {
		header, ok := declaration.(*ShortcutDecl)
		if !ok || header == nil || header.Metadata == nil {
			continue
		}
		for _, field := range header.Metadata.Fields {
			if field.Name != "outputMode" {
				continue
			}
			span := header.Span
			if field.Value != nil {
				span = field.Value.NodeSpan()
			}
			if seen {
				return mode, &MainOutputModeError{Span: span, Message: "outputMode may be declared only once for the main program"}
			}
			seen = true
			literal, ok := field.Value.(*LiteralExpr)
			if !ok || literal == nil || (literal.Type != TokenString && literal.Type != TokenRawString) {
				return mode, &MainOutputModeError{Span: span, Message: "outputMode requires the literal text \"explicit\" or \"legacy\""}
			}
			switch MainOutputMode(literal.Value) {
			case MainOutputExplicit, MainOutputLegacy:
				mode = MainOutputMode(literal.Value)
			default:
				return mode, &MainOutputModeError{Span: span, Message: "outputMode requires the literal text \"explicit\" or \"legacy\""}
			}
		}
	}
	return mode, nil
}
