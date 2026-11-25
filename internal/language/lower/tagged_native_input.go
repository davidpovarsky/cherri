package lower

import (
	"fmt"

	"github.com/electrikmilk/cherri/internal/language/ir"
	"github.com/electrikmilk/cherri/internal/language/types"
)

func (l *Lowerer) rememberTaggedRuntimeValue(value any, layout types.Type) {
	if !types.PrivateValueTransport(layout) {
		return
	}
	reference, ok := value.(*ir.AttachmentToken)
	if !ok || reference == nil {
		return
	}
	switch reference.Type {
	case "ActionOutput":
		if reference.OutputUUID != "" {
			if l.taggedNativeOutputs == nil {
				l.taggedNativeOutputs = map[string]bool{}
			}
			l.taggedNativeOutputs[reference.OutputUUID] = true
		}
	case "Variable":
		if reference.OutputName != "" {
			if l.taggedNativeVariables == nil {
				l.taggedNativeVariables = map[string]bool{}
			}
			l.taggedNativeVariables[reference.OutputName] = true
		}
	}
}

func (l *Lowerer) rememberTaggedBinding(binding BindingRef) {
	layout := binding.TaggedTransport
	if layout == nil {
		layout = binding.Type
	}
	if binding.IsVariable {
		l.rememberTaggedRuntimeValue(&ir.AttachmentToken{Type: "Variable", OutputName: binding.VariableName}, layout)
	} else {
		l.rememberTaggedRuntimeValue(&ir.AttachmentToken{Type: "ActionOutput", OutputUUID: binding.OutputUUID}, layout)
	}
}

// Ordinary explicit input retains its source contract. An omitted native input
// must be observed in the complete canonical sequence, including macro output;
// the partial lowering IR is insufficient evidence for Previous.
func (l *Lowerer) validateTaggedImplicitNativeInput(target *ir.NativeActionNode) error {
	if _, explicit := target.Parameters["WFInput"]; explicit {
		return nil
	}
	if target.AppleIdentifier == "is.workflow.actions.gettext" {
		if _, explicit := target.Parameters["WFTextActionText"]; explicit {
			return nil
		}
	}
	if len(l.taggedNativeOutputs) == 0 && len(l.taggedNativeVariables) == 0 {
		return nil
	}
	observer, okay := l.session.(interface {
		CurrentNativeInputUsesPrivateValue(map[string]bool, map[string]bool, func(string) bool) (bool, error)
	})
	if !okay {
		return fmt.Errorf("E_TARGET_VALUE_SHAPE: implicit native input after structured values requires the canonical value observer")
	}
	private, err := observer.CurrentNativeInputUsesPrivateValue(l.taggedNativeOutputs, l.taggedNativeVariables, l.knownNativeValueOutput)
	if err != nil {
		return err
	}
	if private {
		return fmt.Errorf("E_TARGET_VALUE_SHAPE: omitted native input would consume a private structured value; provide an explicit converted input")
	}
	return nil
}
