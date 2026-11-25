package backend

import (
	"errors"
	"reflect"
	"testing"
)

func TestScopedMeasureVoidRunsOnceAndStopsAtFailedStage(t *testing.T) {
	for _, fail := range []string{"", "debugStartTimer", "body", "debugStopTimer", "nothing"} {
		t.Run(map[bool]string{true: "healthy", false: fail}[fail == ""], func(t *testing.T) {
			var got []string
			failure := errors.New("scoped Void controlled failure")
			visit := func(stage string) error {
				got = append(got, stage)
				if stage == fail {
					return failure
				}
				return nil
			}
			err := EmitScopedMeasureVoid("scope",
				func(name, key string) error {
					if key != "scope" {
						t.Fatalf("timer identity changed: %q", key)
					}
					return visit(name)
				},
				func() error { return visit("body") },
				func() error { return visit("nothing") },
			)
			want := []string{"debugStartTimer", "body", "debugStopTimer", "nothing"}
			if fail != "" {
				for i, stage := range want {
					if stage == fail {
						want = want[:i+1]
						break
					}
				}
				if !errors.Is(err, failure) {
					t.Fatalf("producer failure changed: %v", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("body repeated, value invented, or emission continued after failure: got %v want %v", got, want)
			}
		})
	}
}

func TestScopedMeasureVoidNestedBoundaryOrder(t *testing.T) {
	var got []string
	timer := func(name, key string) error {
		got = append(got, name+":"+key)
		return nil
	}
	err := EmitScopedMeasureVoid("outer", timer, func() error {
		got = append(got, "outer-before")
		if err := EmitScopedMeasureVoid("inner", timer,
			func() error { got = append(got, "inner-effect"); return nil },
			func() error { got = append(got, "inner-nothing"); return nil },
		); err != nil {
			return err
		}
		got = append(got, "outer-after")
		return nil
	}, func() error { got = append(got, "outer-nothing"); return nil })
	want := []string{
		"debugStartTimer:outer", "outer-before",
		"debugStartTimer:inner", "inner-effect", "debugStopTimer:inner", "inner-nothing",
		"outer-after", "debugStopTimer:outer", "outer-nothing",
	}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("nested lifecycle lost source order: %v %v", got, err)
	}
}
