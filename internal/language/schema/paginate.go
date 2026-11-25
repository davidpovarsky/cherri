package schema

import (
	"fmt"
	"math"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/electrikmilk/cherri/internal/language/types"
)

const PaginateMaxPages = 100
const PaginateMaxItems = 10000

type PaginateOptions struct {
	StartURL       string
	Origin         string
	SiblingBaseURL string
	ItemsKey       string
	NextKey        string
	MaxPages       int
	MaxItems       int
}

var paginateKeyPattern = regexp.MustCompile("^[A-Za-z_][A-Za-z0-9_-]{0,63}$")

// ResolvePaginateOptions is shared by analysis and both canonical frontends.
// Scalar traversal options are literal and bounded. The ordinary HTTP parameter
// binder separately validates headers using the established Dictionary codec.
func ResolvePaginateOptions(params map[string]any) (PaginateOptions, error) {
	out := PaginateOptions{ItemsKey: "items", NextKey: "next", MaxPages: 20, MaxItems: 1000}
	start, ok := params["WFPaginateStartURL"].(string)
	if !ok || start == "" {
		return out, fmt.Errorf("PAGINATE_LITERAL: startURL must be a nonempty literal HTTP(S) URL")
	}
	parsed, err := url.Parse(start)
	if err != nil || parsed.Hostname() == "" || parsed.User != nil || parsed.Fragment != "" || parsed.Opaque != "" ||
		(parsed.Scheme != "http" && parsed.Scheme != "https") || strings.ContainsAny(start, " \t\r\n\\") {
		return out, fmt.Errorf("PAGINATE_URL: startURL must be an absolute HTTP(S) URL without userinfo, fragment or whitespace")
	}
	if parsed.Path == "" {
		parsed.Path = "/"
	}
	out.StartURL = parsed.String()
	out.Origin = parsed.Scheme + "://" + parsed.Host
	out.SiblingBaseURL = parsed.ResolveReference(&url.URL{Path: "."}).String()
	for _, option := range []struct {
		key    string
		target *string
	}{
		{"WFPaginateItemsKey", &out.ItemsKey},
		{"WFPaginateNextKey", &out.NextKey},
	} {
		if value, exists := params[option.key]; exists {
			text, literal := value.(string)
			if !literal || !paginateKeyPattern.MatchString(text) {
				return out, fmt.Errorf("PAGINATE_KEY: selectors must be literal simple keys of 1..64 ASCII letters, digits, underscore or hyphen")
			}
			*option.target = text
		}
	}
	if out.ItemsKey == out.NextKey {
		return out, fmt.Errorf("PAGINATE_KEY: itemsKey and nextKey must be different")
	}
	for _, option := range []struct {
		key    string
		target *int
		limit  int
	}{
		{"WFPaginateMaxPages", &out.MaxPages, PaginateMaxPages},
		{"WFPaginateMaxItems", &out.MaxItems, PaginateMaxItems},
	} {
		if value, exists := params[option.key]; exists {
			var number float64
			switch literal := value.(type) {
			case int:
				number = float64(literal)
			case int64:
				number = float64(literal)
			case float64:
				number = literal
			default:
				return out, fmt.Errorf("PAGINATE_LIMIT: limits must be integer literals")
			}
			if math.IsNaN(number) || math.IsInf(number, 0) || math.Trunc(number) != number || number < 0 || number > float64(option.limit) {
				return out, fmt.Errorf("PAGINATE_LIMIT: %s must be an integer in 0..%d", option.key, option.limit)
			}
			*option.target = int(number)
		}
	}
	return out, nil
}

func init() {
	literalText := func(id, title, key string, defaultValue *string, optional bool) ParameterSchema {
		return ParameterSchema{
			ID: id, Label: id, DisplayName: title, Type: types.Text, TypeName: "Text", RawType: "rawstring",
			WireKey: key, Optional: optional, Literal: true, AcceptedForms: []string{"literal"},
			DefaultValue: defaultValue, OmissionPolicy: OmitPolicyOmit, EvidenceStatus: EvidenceInferred,
		}
	}
	literalLimit := func(id, title, key string, defaultValue int) ParameterSchema {
		return ParameterSchema{
			ID: id, Label: id, DisplayName: title, Type: types.Number, TypeName: "Number", RawType: "number",
			WireKey: key, Optional: true, Literal: true, AcceptedForms: []string{"literal"},
			DefaultValue: StringDefault(strconv.Itoa(defaultValue)), OmissionPolicy: OmitPolicyOmit, EvidenceStatus: EvidenceInferred,
		}
	}
	result := types.NewList(types.NewMap(types.Text, types.AnyContent))
	compilerMacroSchemas["paginate"] = &ActionSchema{
		ID: "paginate", CallableName: "paginate", Module: "web", CompilerConstruct: true,
		AppleIdentifier: "is.workflow.actions.getvariable", PrimaryParameterID: "startURL",
		Parameters: []ParameterSchema{
			literalText("startURL", "Initial HTTP(S) URL", "WFPaginateStartURL", nil, false),
			literalText("itemsKey", "Dictionary items key", "WFPaginateItemsKey", StringDefault("items"), true),
			literalText("nextKey", "Next URL or sibling filename key", "WFPaginateNextKey", StringDefault("next"), true),
			literalLimit("maxPages", "Maximum native GET invocations (0..100)", "WFPaginateMaxPages", 20),
			literalLimit("maxItems", "Maximum returned Dictionary items (0..10000)", "WFPaginateMaxItems", 1000),
			{
				ID: "headers", Label: "headers", DisplayName: "Native HTTP headers",
				Type: types.NewMap(types.Text, types.AnyContent), TypeName: "Map<Text, AnyContent>", RawType: "dictionary",
				WireKey: "WFHTTPHeaders", Codec: "dictionary", Optional: true, Literal: true,
				AcceptedForms: []string{"literal"}, OmissionPolicy: OmitPolicyOmit, EvidenceStatus: EvidenceConfirmed,
			},
		},
		OutputType: result, OutputTypeName: result.String(), RawOutputType: "array",
		Docs: ActionDocs{
			Title: "Paginate Bounded Dictionary Pages", Category: "web",
			Description:      "Traverse native Dictionary page bodies with ordered Dictionary items and a body next marker. Next accepts same-origin absolute URLs or simple sibling filenames. Exact resolved-URL hash cycles, absent/empty next and literal caps stop traversal; either zero cap emits no GET. Optional headers use the native HTTP Dictionary codec with runtime leaf values. No Link parsing, HTTP status verification, retry, transport timeout or trailing block contract. Native runtime acceptance remains required.",
			InsertionSnippet: "paginate(\"https://example.invalid/pages/one.json\")",
		},
		Builtin: true, EvidenceStatus: EvidenceInferred,
	}
}
