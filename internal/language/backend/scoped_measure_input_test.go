package backend

import (
	"errors"
	"reflect"
	"testing"
)

func TestScopedMeasureInputCaptureRestoreOrderAndFailures(t *testing.T) {
	for _, void := range []bool{false, true} {
		for _, failed := range []string{"", "capture", "debugStartTimer", "restore", "body", "debugStopTimer"} {
			t.Run(map[bool]string{true: "void/", false: "value/"}[void]+failed, func(t *testing.T) {
				var stages []string
				sentinel := errors.New("original scoped-input failure")
				visit := func(name string) error {
					stages = append(stages, name)
					if name == failed {
						return sentinel
					}
					return nil
				}
				input := ScopedMeasureInput{Capture: func() error { return visit("capture") }, Restore: func() error { return visit("restore") }}
				timer := func(name, key string) error { return visit(name) }
				var err error
				if void {
					err = EmitScopedMeasureVoid("scope", timer, func() error { return visit("body") }, func() error { return visit("nothing") }, input)
				} else {
					var result int
					result, err = EmitScopedMeasure("scope", timer, func() (int, error) { return 37, visit("body") }, func(int) bool { return true }, func(v int, final bool) (int, error) { return v, visit("forward") }, input)
					if failed == "" && result != 37 {
						t.Fatal("body value replaced")
					}
					if failed != "" && result != 0 {
						t.Fatal("result published after failed lifecycle")
					}
				}
				want := []string{"capture", "debugStartTimer", "restore", "body", "debugStopTimer"}
				if failed != "" {
					for i, stage := range want {
						if stage == failed {
							want = want[:i+1]
							break
						}
					}
					if !errors.Is(err, sentinel) {
						t.Fatalf("original error lost: %v", err)
					}
				} else {
					want = append(want, map[bool]string{true: "nothing", false: "forward"}[void])
					if err != nil {
						t.Fatal(err)
					}
				}
				if !reflect.DeepEqual(stages, want) {
					t.Fatalf("input/body lifecycle order: %v, want %v", stages, want)
				}
			})
		}
	}
}

func TestScopedMeasureRejectsIncompleteInputBoundaryBeforeEffects(t *testing.T) {
	for _, input := range []ScopedMeasureInput{{Capture: func() error { return nil }}, {Restore: func() error { return nil }}} {
		visited := false
		err := EmitScopedMeasureVoid("scope", func(string, string) error { visited = true; return nil }, func() error { visited = true; return nil }, func() error { visited = true; return nil }, input)
		if err == nil || visited {
			t.Fatal("incomplete input boundary emitted effects")
		}
	}
}
