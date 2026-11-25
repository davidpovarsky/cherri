package backend

// EmitScopedMeasure preserves the existing value-body contract for both
// frontends. Native references retain their identity across diagnostic work.
func EmitScopedMeasure[T any](
	timerKey string,
	timer func(name, key string) error,
	body func() (T, error),
	frozen func(T) bool,
	forward func(value T, final bool) (T, error),
) (T, error) {
	return emitScopedMeasure(timerKey, timer, func() (T, bool, error) {
		value, err := body()
		return value, true, err
	}, frozen, forward, nil)
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
) error {
	_, err := emitScopedMeasure(timerKey, timer, func() (struct{}, bool, error) {
		return struct{}{}, false, body()
	}, nil, nil, finishVoid)
	return err
}

func emitScopedMeasure[T any](
	timerKey string,
	timer func(name, key string) error,
	body func() (T, bool, error),
	frozen func(T) bool,
	forward func(value T, final bool) (T, error),
	finishVoid func() error,
) (T, error) {
	var zero T
	if err := timer("debugStartTimer", timerKey); err != nil {
		return zero, err
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
