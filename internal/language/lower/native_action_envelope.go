package lower

import (
	"fmt"

	"github.com/electrikmilk/cherri/internal/language/backend"
	"github.com/electrikmilk/cherri/internal/language/ir"
)

func nativeActionEnvelopeValue(value any) (map[string]any, error) {
	var data any
	switch raw := value.(type) {
	case *ir.RawPlistValue:
		if raw != nil {
			data = raw.Data
		}
	case ir.RawPlistValue:
		data = raw.Data
	default:
		return nil, fmt.Errorf("NATIVE_ACTION_ENVELOPE_EXACT_REQUIRED: envelope requires native.plist")
	}
	envelope, ok := data.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("NATIVE_ACTION_ENVELOPE_SHAPE: envelope must be a dictionary, got %T", data)
	}
	return cloneLowerPlistValue(envelope).(map[string]any), nil
}

func setNativeActionEnvelope(node *ir.NativeActionNode, envelope map[string]any) error {
	params, present, err := backend.ValidateNativeActionEnvelope(node.AppleIdentifier, envelope)
	if err != nil {
		return err
	}
	node.ExactRawEnvelope = envelope
	node.ExactRawParameters = true
	node.HasExactRawParameters = present
	node.Parameters = params
	return nil
}
