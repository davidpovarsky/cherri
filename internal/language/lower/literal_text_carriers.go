package lower

import (
	"fmt"

	"github.com/electrikmilk/cherri/internal/language/ir"
	"github.com/electrikmilk/cherri/internal/language/syntax"
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
	} else if reference := l.nativeResultReferences[previous]; reference != nil {
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
