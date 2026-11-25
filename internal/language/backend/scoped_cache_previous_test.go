package backend

import (
	"errors"
	"fmt"
	"testing"
)

type cacheSnapshotEmission struct {
	id     string
	params map[string]any
}

func cacheSnapshotVariable(value any) string {
	s, okay := value.(SemanticValue)
	if !okay || s.Kind != ValReference || s.Ref == nil || s.Ref.Kind != RefMutableBinding {
		return ""
	}
	return s.Ref.ProducerName
}
func TestScopedCachePreviousSnapshotBoundary(t *testing.T) {
	for _, ttl := range []float64{0, 300} {
		t.Run(fmt.Sprint(ttl), func(t *testing.T) {
			var emitted []cacheSnapshotEmission
			serial, bodyCalls, bodyAt := 0, 0, -1
			result, err := EmitScopedCache(ScopedCacheConfig{Namespace: "test-key", Kind: "number", TTL: ttl}, "private",
				func() string { serial++; return fmt.Sprintf("N-%d", serial) },
				func(id string, p map[string]any, _, _ string) error {
					emitted = append(emitted, cacheSnapshotEmission{id, p})
					return nil
				},
				func() (*Reference, error) {
					bodyCalls++
					bodyAt = len(emitted)
					emitted = append(emitted, cacheSnapshotEmission{"is.workflow.actions.number", map[string]any{"WFNumberActionNumber": "7"}})
					return &Reference{Kind: RefActionResult, ProducerID: "BODY", ProducerName: "Body"}, nil
				})
			if err != nil || result == nil || bodyCalls != 1 {
				t.Fatal(result, err, bodyCalls)
			}
			if ttl == 0 {
				if bodyAt != 0 || len(emitted) != 2 || emitted[0].id != "is.workflow.actions.number" || emitted[1].id != "is.workflow.actions.getvariable" {
					t.Fatal("TTL zero changed its direct value-only sequence", emitted)
				}
				return
			}
			if emitted[0].id != "is.workflow.actions.setvariable" || emitted[0].params["WFVariableName"] != "private_input" {
				t.Fatal("capture was not the first lifecycle action")
			}
			if _, present := emitted[0].params["WFInput"]; present {
				t.Fatal("capture invented an explicit native value")
			}
			if bodyAt < 2 || emitted[bodyAt-1].id != "is.workflow.actions.getvariable" || cacheSnapshotVariable(emitted[bodyAt-1].params["WFVariable"]) != "private_input" {
				t.Fatal("body did not immediately consume original input")
			}
			before := emitted[bodyAt-2]
			if before.id != "is.workflow.actions.conditional" || fmt.Sprint(before.params["WFControlFlowMode"]) != "0" || fmt.Sprint(before.params["WFCondition"]) != "4" || fmt.Sprint(before.params["WFNumberValue"]) != "0" {
				t.Fatal("restore escaped exact miss branch")
			}
		})
	}
}
func TestScopedCachePreviousEmissionFailuresDoNotRunBody(t *testing.T) {
	for _, stage := range []string{"capture", "restore", "body"} {
		t.Run(stage, func(t *testing.T) {
			sentinel := errors.New("controlled-" + stage)
			serial, bodyCalls := 0, 0
			_, err := EmitScopedCache(ScopedCacheConfig{Namespace: "test-key", Kind: "number", TTL: 300}, "private",
				func() string { serial++; return fmt.Sprintf("N-%d", serial) },
				func(id string, p map[string]any, _, _ string) error {
					if stage == "capture" && id == "is.workflow.actions.setvariable" && p["WFVariableName"] == "private_input" {
						return sentinel
					}
					if stage == "restore" && id == "is.workflow.actions.getvariable" && cacheSnapshotVariable(p["WFVariable"]) == "private_input" {
						return sentinel
					}
					return nil
				},
				func() (*Reference, error) { bodyCalls++; return nil, sentinel })
			want := 0
			if stage == "body" {
				want = 1
			}
			if !errors.Is(err, sentinel) || bodyCalls != want {
				t.Fatal("failure boundary ran or replaced authored body", err, bodyCalls, want)
			}
		})
	}
}
