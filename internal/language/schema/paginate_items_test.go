package schema

import (
	"github.com/electrikmilk/cherri/internal/language/types"
	"strings"
	"testing"
)

func TestPaginateSelectionSourceContract(t *testing.T) {
	for _, row := range []struct {
		kind    string
		element types.Type
	}{
		{"dictionary", types.NewMap(types.Text, types.AnyContent)}, {"text", types.Text}, {"number", types.Number}, {"bool", types.Bool},
	} {
		for _, key := range []string{"", "payload", "_x-1", strings.Repeat("a", 64)} {
			t.Run(row.kind+"/"+key, func(t *testing.T) {
				params := map[string]any{"WFPaginateStartURL": "https://example.invalid/p", "WFPaginateItemType": row.kind}
				if key != "" {
					params["WFPaginateItemKey"] = key
				}
				options, err := ResolvePaginateOptions(params)
				if err != nil {
					t.Fatal(err)
				}
				if options.ItemType != row.kind || options.ItemKey != key || !options.ResultType().Equals(types.NewList(row.element)) {
					t.Fatalf("selector and output contract diverged: %+v / %v", options, options.ResultType())
				}
				if types.PrivateValueTransport(options.ResultType()) {
					t.Fatal("public native result unexpectedly requires private cells")
				}
			})
		}
	}
	options, err := ResolvePaginateOptions(map[string]any{"WFPaginateStartURL": "https://example.invalid/p"})
	if err != nil || options.ItemType != "dictionary" || options.ItemKey != "" ||
		!options.ResultType().Equals(types.NewList(types.NewMap(types.Text, types.AnyContent))) {
		t.Fatalf("default Dictionary contract changed: %+v %v", options, err)
	}
}

func TestPaginateSelectionRejectsInvalidSourceContracts(t *testing.T) {
	for _, row := range []struct {
		name, key, diagnostic string
		value                 any
	}{
		{"unknown_type", "WFPaginateItemType", "PAGINATE_ITEM_TYPE:", "file"},
		{"list", "WFPaginateItemType", "PAGINATE_ITEM_TYPE:", "list"},
		{"optional", "WFPaginateItemType", "PAGINATE_ITEM_TYPE:", "optional"},
		{"null", "WFPaginateItemType", "PAGINATE_ITEM_TYPE:", "null"},
		{"mixed", "WFPaginateItemType", "PAGINATE_ITEM_TYPE:", "any"},
		{"capital_type", "WFPaginateItemType", "PAGINATE_ITEM_TYPE:", "Text"},
		{"empty_type", "WFPaginateItemType", "PAGINATE_ITEM_TYPE:", ""},
		{"dynamic_type", "WFPaginateItemType", "PAGINATE_ITEM_TYPE:", map[string]any{}},
		{"empty_key", "WFPaginateItemKey", "PAGINATE_ITEM_KEY:", ""},
		{"digit_prefix", "WFPaginateItemKey", "PAGINATE_ITEM_KEY:", "1x"},
		{"dotted_key", "WFPaginateItemKey", "PAGINATE_ITEM_KEY:", "a.b"},
		{"path_key", "WFPaginateItemKey", "PAGINATE_ITEM_KEY:", "a/b"},
		{"long_key", "WFPaginateItemKey", "PAGINATE_ITEM_KEY:", strings.Repeat("a", 65)},
		{"control_key", "WFPaginateItemKey", "PAGINATE_ITEM_KEY:", "x\n"},
		{"dynamic_key", "WFPaginateItemKey", "PAGINATE_ITEM_KEY:", map[string]any{}},
	} {
		t.Run(row.name, func(t *testing.T) {
			_, err := ResolvePaginateOptions(map[string]any{"WFPaginateStartURL": "https://example.invalid/p", row.key: row.value})
			if err == nil || !strings.HasPrefix(err.Error(), row.diagnostic) {
				t.Fatalf("missing precise selector diagnostic: %v", err)
			}
		})
	}
}

func TestPaginateSelectionSchemaEnumeratesLiteralKinds(t *testing.T) {
	macro, found := DefaultRegistry().LookupAction("paginate")
	if !found {
		t.Fatal("paginate absent")
	}
	key, found := macro.ParameterByLabel("itemKey")
	if !found || !key.Optional || !key.Literal || !key.Type.Equals(types.Text) || key.DefaultValue != nil {
		t.Fatal("selector key contract changed")
	}
	kind, found := macro.ParameterByLabel("itemType")
	if !found || !kind.Optional || !kind.Literal || !kind.Type.Equals(types.Text) || kind.DefaultValue == nil || *kind.DefaultValue != "dictionary" || strings.Join(kind.EnumValues, ",") != "dictionary,text,number,bool" {
		t.Fatal("finite literal item kind contract missing")
	}
}
