package backend

// EmitScopedMeasure orders the shared compiler-level lifecycle for both
// frontends. The body emits once into the existing canonical backend. A mutable
// or system result is frozen before timing work; the final forwarding action
// restores implicit native input/Repeat Results without converting the value.
//
// T is each frontend's existing structured reference carrier, never source text
// or a newly serialized wire value. The callbacks emit native actions; they do
// not execute a runtime body or make native error-catching claims.
func EmitScopedMeasure[T any](
	timerKey string,
	timer func(name, key string) error,
	body func() (T, error),
	frozen func(T) bool,
	forward func(value T, final bool) (T, error),
) (T, error) {
	var zero T
	if err := timer("debugStartTimer", timerKey); err != nil {
		return zero, err
	}
	value, err := body()
	if err != nil {
		return zero, err
	}
	if !frozen(value) {
		value, err = forward(value, false)
		if err != nil {
			return zero, err
		}
	}
	if err := timer("debugStopTimer", timerKey); err != nil {
		return zero, err
	}
	if _, err := forward(value, true); err != nil {
		return zero, err
	}
	return value, nil
}
