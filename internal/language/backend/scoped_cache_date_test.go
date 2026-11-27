package backend

import (
	"fmt"
	"testing"
)

type cacheDateAction struct {
	id     string
	params map[string]any
	uuid   string
}

func cacheDateGraph(t *testing.T, ttl float64) ([]cacheDateAction, *Reference) {
	t.Helper()
	var actions []cacheDateAction
	var serial int
	var next = func() string { serial++; return fmt.Sprintf("date-id-%d", serial) }
	var emit = func(id string, params map[string]any, uuid, name string) error {
		actions = append(actions, cacheDateAction{id, params, uuid})
		return nil
	}
	var result, err = EmitScopedCache(ScopedCacheConfig{Namespace: "date-namespace", Kind: "date", TTL: ttl}, "date-cache", next, emit, func() (*Reference, error) {
		var uuid = next()
		emit("is.workflow.actions.date", map[string]any{"WFDateActionMode": "Specified Date", "WFDateActionDate": "2026-11-27T14:00:00.125Z"}, uuid, "source-date")
		return &Reference{Kind: RefActionResult, ProducerID: uuid, ProducerName: "source-date"}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return actions, result
}

func TestScopedCacheDateUsesTypedRoundtripAndLazyStorage(t *testing.T) {
	var actions, result = cacheDateGraph(t, 300)
	var source, writes, restores, dateGuards int
	var sourceIndex, storeIndex = -1, -1
	for index, action := range actions {
		switch action.id {
		case "is.workflow.actions.date":
			if action.params["WFDateActionDate"] == "2026-11-27T14:00:00.125Z" {
				source++
				sourceIndex = index
			}
		case "is.workflow.actions.setstoredcontent":
			writes++
			storeIndex = index
		case "is.workflow.actions.adjustdate":
			restores++
			if action.params["WFAdjustOperation"] != "Add" {
				t.Fatal("date restoration must use Add from epoch")
			}
			var quantity, ok = action.params["WFDuration"].(*QuantityValue)
			if !ok || quantity == nil {
				t.Fatal("date reconstruction bypassed the canonical quantity codec")
			}
			if quantity.Unit.Kind != ValString || quantity.Unit.StrVal != "sec" {
				t.Fatal("date magnitude has no Seconds unit")
			}
			if quantity.Magnitude.Kind != ValReference || quantity.Magnitude.Ref == nil {
				t.Fatal("dynamic seconds were stringified")
			}

		case "is.workflow.actions.conditional":
			if _, found := action.params["WFDate"]; found {
				dateGuards++
			}
		}
	}
	if source != 1 || writes != 1 || sourceIndex < 0 || storeIndex <= sourceIndex {
		t.Fatalf("wrong lazy source/store ordering: %d %d %d %d", source, writes, sourceIndex, storeIndex)
	}
	if restores != 2 || dateGuards != 1 {
		t.Fatalf("missing write and read reconstruction guards: restores=%d dateGuards=%d", restores, dateGuards)
	}
	if result == nil || result.Kind != RefActionResult {
		t.Fatal("lost original native result reference")
	}
}

func TestScopedCacheDateZeroTTLRetainsOnlyBodyAndForwarding(t *testing.T) {
	var actions, _ = cacheDateGraph(t, 0)
	if len(actions) != 2 || actions[0].id != "is.workflow.actions.date" || actions[1].id != "is.workflow.actions.getvariable" {
		t.Fatalf("zero TTL acquired archival helpers: %#v", actions)
	}
}

func TestScopedCacheDateDoesNotRelaxUnsupportedKinds(t *testing.T) {
	for _, kind := range []string{"file", "list", "dictionary", "optional", "null", "json"} {
		t.Run(kind, func(t *testing.T) {
			var emitted, called bool
			_, err := EmitScopedCache(ScopedCacheConfig{Namespace: "key", Kind: kind, TTL: 300}, "prefix", func() string { return "id" }, func(string, map[string]any, string, string) error { emitted = true; return nil }, func() (*Reference, error) { called = true; return nil, nil })
			if err == nil || emitted || called {
				t.Fatalf("unsupported persistence %q escaped admission", kind)
			}
		})
	}
}
