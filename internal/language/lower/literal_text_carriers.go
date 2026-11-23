package lower

import (
	"fmt"

	"github.com/electrikmilk/cherri/internal/language/backend"
	"github.com/electrikmilk/cherri/internal/language/ir"
	"github.com/electrikmilk/cherri/internal/language/syntax"
	"github.com/electrikmilk/cherri/internal/language/types"
)

// Ordinary strings preserve literal U+FFFC. Only an actual reference makes the
// resulting FString a native token template that needs literal carrier actions.
func fStringValuesHaveReference(values []interface{}) bool {
	for _, value := range values {
		if _, ok := value.(*ir.AttachmentToken); ok {
			return true
		}
	}
	return false
}

// Track only the compiler's literal chunks, never authored Text actions that
// happen to contain the same bytes.
func (l *Lowerer) materializeLiteralTextCarrier(literal string) *ir.AttachmentToken {
	reference := l.materializeToAttachment(literal)
	if session, ok := l.session.(interface {
		BindLiteralTextCarrierInput(string, string, string, func(string) bool) error
	}); ok && l.emitErr == nil {
		l.emitErr = session.BindLiteralTextCarrierInput(reference.OutputUUID, l.GenerateUUID(), l.GenerateUUID(), l.knownNativeValueOutput)
	}
	if l.literalTextCarriers == nil {
		l.literalTextCarriers = make(map[*ir.NativeActionNode]bool)
	}
	l.literalTextCarriers[l.workflow.Actions[len(l.workflow.Actions)-1]] = true
	return reference
}

// A native escape can intentionally omit its input. Pure helpers used to
// replace that implicit input, including before later authored parameter calls.
// Restore only an existing value identity through the canonical materializer;
// opaque native parameters retain the existing result-consumption checks.
func (l *Lowerer) restoreNativeInputAfterLiteralCarriers(target *ir.NativeActionNode) error {
	if _, explicit := target.Parameters["WFInput"]; explicit {
		return nil
	}
	// Text's established source is its explicit WFTextActionText payload, not
	// Previous input. Do not resolve a deferred opaque Previous cursor solely
	// because a new exact-comparison snapshot precedes this self-contained
	// action. This finite exception does not infer input contracts for other
	// identifiers, arbitrary custom parameters or omitted Text payloads.
	if l.hasCurrentExactTextSnapshot() && target.AppleIdentifier == "is.workflow.actions.gettext" {
		if _, explicit := target.Parameters["WFTextActionText"]; explicit {
			return nil
		}
	}
	if session, ok := l.session.(interface {
		CurrentLiteralCarrierInput() (*backend.Reference, bool, error)
	}); ok {
		reference, found, err := session.CurrentLiteralCarrierInput()
		if err != nil || !found {
			return err
		}
		if reference == nil {
			l.materializeYieldValue(nil)
		} else {
			var input *ir.AttachmentToken
			switch reference.Kind {
			case backend.RefActionResult:
				input = &ir.AttachmentToken{Type: "ActionOutput", OutputUUID: reference.ProducerID}
			case backend.RefExtensionInput:
				l.workflow.HasShortcutInputVariables = true
				l.session.SetMetadata("hasShortcutInputVariables", true)
				input = &ir.AttachmentToken{Type: "ExtensionInput"}
			default:
				return fmt.Errorf("S01_IMPLICIT_INPUT_UNAVAILABLE: unsupported preserved input kind")
			}
			l.materializeYieldValue(input)
		}
		return l.emitErr
	}
	index := len(l.workflow.Actions) - 1
	if index < 0 || !l.literalTextCarriers[l.workflow.Actions[index]] {
		return nil
	}
	for index >= 0 && l.literalTextCarriers[l.workflow.Actions[index]] {
		index--
	}
	if index < 0 {
		input, err := l.lowerExpression(&syntax.IdentExpr{Name: "ShortcutInput"})
		if err != nil {
			return err
		}
		l.materializeYieldValue(input)
		return l.emitErr
	}
	previous := l.workflow.Actions[index]
	if previous.AppleIdentifier == "is.workflow.actions.nothing" {
		l.materializeYieldValue(nil)
		return l.emitErr
	}
	if !l.knownNativeValueOutput(previous.AppleIdentifier) {
		return fmt.Errorf("S01_IMPLICIT_INPUT_UNAVAILABLE: preceding action has no established value-output contract; supply an explicit native input")
	}
	if reference := l.nativeResultReferences[previous]; reference != nil {
		l.materializeYieldValue(reference)
	} else if previous.OutputUUID != "" {
		l.materializeYieldValue(&ir.AttachmentToken{
			Type: "ActionOutput", OutputUUID: previous.OutputUUID, OutputName: previous.OutputName,
		})
	} else {
		return fmt.Errorf("S01_IMPLICIT_INPUT_UNAVAILABLE: preceding native input has no provable value identity; supply an explicit native input")
	}
	return l.emitErr
}

// Native declaration variants must agree that the identifier produces a value.
// Builtin macros may share their final native identifier while describing the
// macro's aggregate result (for example embedFile and Text); they do not define
// the output contract of this already-emitted native action.
func (l *Lowerer) knownNativeValueOutput(identifier string) bool {
	foundNative := false
	for _, action := range l.registry.LookupByIdentifier(identifier) {
		if action.Builtin || action.CompilerConstruct {
			continue
		}
		foundNative = true
		if !knownNativeValueOutputType(action.OutputType) {
			return false
		}
	}
	return foundNative
}

func knownNativeValueOutputType(value types.Type) bool {
	if value == nil {
		return false
	}
	switch typed := value.(type) {
	case types.OptionalType:
		return knownNativeValueOutputType(typed.Inner)
	case types.UnionType:
		if len(typed.Members) == 0 {
			return false
		}
		for _, member := range typed.Members {
			if !knownNativeValueOutputType(member) {
				return false
			}
		}
		return true
	}
	return value.Kind() != types.KindUnknown && value.Kind() != types.KindVoid
}
