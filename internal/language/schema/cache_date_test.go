package schema

import (
	"github.com/electrikmilk/cherri/internal/language/types"
	"testing"
)

func TestCacheDatePersistenceAdmission(t *testing.T) {
	if got := CacheValueKind(types.Date); got != "date" {
		t.Fatalf("Date persistence kind = %q", got)
	}
	for _, value := range []types.Type{types.NewOptional(types.Date), types.File, types.JsonNull, types.JsonValue, types.NewList(types.Date)} {
		if got := CacheValueKind(value); got != "" {
			t.Fatalf("unqualified %s admitted as %q", value, got)
		}
	}
	var options = CacheOptions{Key: "date source", TTL: 300}
	if options.Namespace("date") == options.Namespace("number") {
		t.Fatal("Date shares numeric cache namespace")
	}
}
