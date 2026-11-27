package backend

import "fmt"

// ScopedMeasureInput preserves the complete native input across timer setup.
// Both callbacks are supplied together only when diagnostic actions are emitted.
// An empty boundary retains the existing lifecycle without helper actions.
type ScopedMeasureInput struct {
	Capture func() error
	Restore func() error
}

// EmitScopedMeasure preserves the existing value-body contract for both
// frontends. Native references retain their identity across diagnostic work.
func EmitScopedMeasure[T any](
	timerKey string,
	timer func(name, key string) error,
	body func() (T, error),
	frozen func(T) bool,
	forward func(value T, final bool) (T, error),
	input ...ScopedMeasureInput,
) (T, error) {
	return emitScopedMeasure(timerKey, timer, func() (T, bool, error) {
		value, err := body()
		return value, true, err
	}, frozen, forward, nil, input...)
}

// EmitScopedMeasureVoid runs the same lifecycle without inventing a value.
// finishVoid emits the frontend's canonical Nothing boundary after diagnostics,
// in both Debug and Release. Native aborts still terminate at their own action;
// these callbacks generate a graph and do not implement runtime try/finally.
func EmitScopedMeasureVoid(
	timerKey string,
	timer func(name, key string) error,
	body func() error,
	finishVoid func() error,
	input ...ScopedMeasureInput,
) error {
	_, err := emitScopedMeasure(timerKey, timer, func() (struct{}, bool, error) {
		return struct{}{}, false, body()
	}, nil, nil, finishVoid, input...)
	return err
}

func emitScopedMeasure[T any](
	timerKey string,
	timer func(name, key string) error,
	body func() (T, bool, error),
	frozen func(T) bool,
	forward func(value T, final bool) (T, error),
	finishVoid func() error,
	inputs ...ScopedMeasureInput,
) (T, error) {
	var zero T
	if len(inputs) > 1 {
		return zero, fmt.Errorf("E_BLOCK_INPUT_BOUNDARY: one scoped input boundary is required")
	}
	var input ScopedMeasureInput
	if len(inputs) == 1 {
		input = inputs[0]
	}
	if (input.Capture == nil) != (input.Restore == nil) {
		return zero, fmt.Errorf("E_BLOCK_INPUT_BOUNDARY: capture and restore must be paired")
	}
	if input.Capture != nil {
		if err := input.Capture(); err != nil {
			return zero, err
		}
	}
	if err := timer("debugStartTimer", timerKey); err != nil {
		return zero, err
	}
	if input.Restore != nil {
		if err := input.Restore(); err != nil {
			return zero, err
		}
	}
	value, hasValue, err := body()
	if err != nil {
		return zero, err
	}
	if hasValue && !frozen(value) {
		value, err = forward(value, false)
		if err != nil {
			return zero, err
		}
	}
	if err := timer("debugStopTimer", timerKey); err != nil {
		return zero, err
	}
	if !hasValue {
		if err := finishVoid(); err != nil {
			return zero, err
		}
		return zero, nil
	}
	if _, err := forward(value, true); err != nil {
		return zero, err
	}
	return value, nil
}
