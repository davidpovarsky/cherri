package backend

import (
	"errors"
	"reflect"
	"testing"
)

func TestScopedMeasurePreservesTypedIdentityAndStopsOnProducerFailure(t *testing.T) {
	type reference struct {
		nativeID string
		frozen   bool
	}
	original := &reference{nativeID: "original"}
	snapshot := &reference{nativeID: "snapshot", frozen: true}
	for _, immutable := range []bool{false, true} {
		for _, failedStage := range []string{"", "debugStartTimer", "body", "snapshot", "debugStopTimer", "forward"} {
			if immutable && failedStage == "snapshot" {
				continue
			}
			t.Run(failedStage+map[bool]string{true: "/immutable", false: "/mutable"}[immutable], func(t *testing.T) {
				original.frozen = immutable
				var observed []string
				sentinel := errors.New("controlled producer failure")
				visit := func(stage string) error {
					observed = append(observed, stage)
					if stage == failedStage {
						return sentinel
					}
					return nil
				}
				result, err := EmitScopedMeasure("scope",
					func(name, key string) error {
						if key != "scope" {
							t.Fatal("timer key changed")
						}
						return visit(name)
					},
					func() (*reference, error) { return original, visit("body") },
					func(value *reference) bool { return value.frozen },
					func(value *reference, final bool) (*reference, error) {
						if !final {
							if value != original {
								t.Fatal("original reference changed before snapshot")
							}
							return snapshot, visit("snapshot")
						}
						expected := original
						if !immutable {
							expected = snapshot
						}
						if value != expected {
							t.Fatal("final forwarding changed native identity")
						}
						return &reference{nativeID: "forward"}, visit("forward")
					})
				want := []string{"debugStartTimer", "body"}
				if !immutable {
					want = append(want, "snapshot")
				}
				want = append(want, "debugStopTimer", "forward")
				if failedStage != "" {
					for i, stage := range want {
						if stage == failedStage {
							want = want[:i+1]
							break
						}
					}
					if !errors.Is(err, sentinel) || result != nil {
						t.Fatalf("failure not propagated: result=%v err=%v", result, err)
					}
				} else {
					expected := original
					if !immutable {
						expected = snapshot
					}
					if err != nil || result != expected {
						t.Fatalf("body native result replaced: result=%v err=%v", result, err)
					}
				}
				if !reflect.DeepEqual(observed, want) {
					t.Fatalf("body repeated or work emitted after failure: got %v want %v", observed, want)
				}
			})
		}
	}
}
