package schema

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/electrikmilk/cherri/internal/language/syntax"
	"github.com/electrikmilk/cherri/internal/language/types"
)

// CacheOptions is an explicit persistence request. Sensitive source values must
// be marked by their author; the compiler does not pretend to detect secrets.
type CacheOptions struct {
	Key                               string
	TTL                               float64
	Global, Sensitive, AllowSensitive bool
}

func ResolveCacheOptions(values map[string]any) (CacheOptions, error) {
	var out CacheOptions
	for key := range values {
		switch key {
		case "key", "ttl", "global", "sensitive", "allowSensitive":
		default:
			return out, fmt.Errorf("E_CACHE_OPTION: unknown cache option %q", key)
		}
	}
	var okay bool
	out.Key, okay = values["key"].(string)
	if !okay || strings.TrimSpace(out.Key) == "" {
		return out, fmt.Errorf("E_CACHE_KEY: key must be a nonempty literal Text value")
	}
	switch ttl := values["ttl"].(type) {
	case int:
		out.TTL = float64(ttl)
	case int64:
		out.TTL = float64(ttl)
	case float64:
		out.TTL = ttl
	default:
		return out, fmt.Errorf("E_CACHE_TTL: ttl must be a finite nonnegative literal number of seconds")
	}
	if math.IsNaN(out.TTL) || math.IsInf(out.TTL, 0) || out.TTL < 0 {
		return out, fmt.Errorf("E_CACHE_TTL: ttl must be a finite nonnegative literal number of seconds")
	}
	for key, target := range map[string]*bool{"global": &out.Global, "sensitive": &out.Sensitive, "allowSensitive": &out.AllowSensitive} {
		if value, present := values[key]; present {
			flag, okay := value.(bool)
			if !okay {
				return out, fmt.Errorf("E_CACHE_OPTION: %s must be a literal Bool", key)
			}
			*target = flag
		}
	}
	if out.Sensitive && !out.AllowSensitive {
		return out, fmt.Errorf("E_CACHE_SENSITIVE: sensitive values require explicit allowSensitive:true")
	}
	return out, nil
}
func (options CacheOptions) Namespace(kind string) string {
	sum := sha256.Sum256([]byte(options.Key))
	scope := "local"
	if options.Global {
		scope = "global"
	}
	return "cherri.cache.v1." + scope + "." + kind + "." + hex.EncodeToString(sum[:])
}
func CacheValueKind(value types.Type) string {
	if value == nil || types.MayBeAbsent(value) {
		return ""
	}
	switch {
	case types.Text.Equals(value):
		return "text"
	case types.Number.Equals(value):
		return "number"
	case types.Bool.Equals(value):
		return "bool"
	default:
		return ""
	}
}
func CacheCallOptions(call *syntax.CallExpr) (CacheOptions, error) {
	values := map[string]any{}
	set := func(name string, expr syntax.Expression) error {
		if _, duplicate := values[name]; duplicate {
			return fmt.Errorf("E_CACHE_OPTION: duplicate cache option %q", name)
		}
		literal, okay := expr.(*syntax.LiteralExpr)
		if !okay {
			return fmt.Errorf("E_CACHE_LITERAL: cache options are literal; dynamic values are not evaluated")
		}
		switch literal.Type {
		case syntax.TokenString, syntax.TokenRawString:
			values[name] = literal.Value
		case syntax.TokenNumber:
			number, err := strconv.ParseFloat(literal.Value, 64)
			if err != nil {
				return fmt.Errorf("E_CACHE_TTL: invalid number")
			}
			values[name] = number
		case syntax.TokenTrue:
			values[name] = true
		case syntax.TokenFalse:
			values[name] = false
		default:
			return fmt.Errorf("E_CACHE_LITERAL: cache options must be literal Text, Number or Bool")
		}
		return nil
	}
	if call == nil {
		return CacheOptions{}, fmt.Errorf("E_CACHE_OPTION: missing source call")
	}
	if call.PrimaryArg != nil {
		if err := set("key", call.PrimaryArg); err != nil {
			return CacheOptions{}, err
		}
	}
	for _, arg := range call.NamedArgs {
		if err := set(arg.Label, arg.Value); err != nil {
			return CacheOptions{}, err
		}
	}
	return ResolveCacheOptions(values)
}
func init() {
	parameter := func(id string, t types.Type, optional bool) ParameterSchema {
		raw := strings.ToLower(t.String())
		return ParameterSchema{ID: id, Label: id, DisplayName: id, Type: t, TypeName: t.String(), RawType: raw,
			Optional: optional, Literal: true, AcceptedForms: []string{"literal"}, EvidenceStatus: EvidenceInferred}
	}
	compilerMacroSchemas["cache"] = &ActionSchema{
		ID: "cache", CallableName: "cache", Module: "storage", Builtin: true, CompilerConstruct: true, ScopedBlock: true,
		PrimaryParameterID: "key", MinVersion: 27, OutputType: types.AnyContent, OutputTypeName: "AnyContent",
		Parameters: []ParameterSchema{parameter("key", types.Text, false), parameter("ttl", types.Number, false),
			parameter("global", types.Bool, true), parameter("sensitive", types.Bool, true), parameter("allowSensitive", types.Bool, true)},
		EvidenceStatus: EvidenceInferred, Docs: ActionDocs{
			Title: "Cache Scalar Value with TTL", Category: "storage",
			Description:      "Legacy and V2 scalar scoped block with no block parameters. Lazy iOS27 Stored Content cache for source-typed Text, Number and Bool. The body must return one supported scalar; Void and complex/native objects are rejected. Literal key/ttl/scope; ttl0 bypasses storage. Versioned scalar envelope, post-body expiry, clock rollback and changed-policy misses. Sensitive values require explicit opt-in. Complex/native objects, atomic compare-and-set and recoverable native parser/storage errors are not supported; native cache acceptance remains pending.",
			InsertionSnippet: "cache(key:\"key\", ttl:300) { <#scalar value#> }",
		},
	}
}

func LoopResultScopedCacheBody(call *syntax.CallExpr, action *ActionSchema) *syntax.BlockStmt {
	canonical := compilerMacroSchemas["cache"]
	if call == nil || call.Block == nil || call.Block.Body == nil || action == nil || canonical == nil ||
		action.ID != "cache" || action.CallableName != "cache" || action.Module != "storage" ||
		!action.Builtin || action.Custom || !action.CompilerConstruct || !action.ScopedBlock ||
		action.AppleIdentifier != "" || action.Variant != "" || action.AppIntent != nil ||
		len(action.StaticParameters) != 0 || len(action.EmittedKeys) != 0 ||
		action.PrimaryParameterID != "key" || action.MinVersion != 27 || action.MaxVersion != 0 ||
		action.OutputType == nil || !types.AnyContent.Equals(action.OutputType) ||
		len(action.Parameters) != len(canonical.Parameters) {
		return nil
	}
	name, okay := call.Callee.(*syntax.IdentExpr)
	if !okay || name.Name != "cache" {
		return nil
	}
	for index, parameter := range action.Parameters {
		expected := canonical.Parameters[index]
		if parameter.ID != expected.ID || parameter.Label != expected.Label || parameter.Type == nil ||
			!expected.Type.Equals(parameter.Type) || parameter.Optional != expected.Optional ||
			!parameter.Literal || parameter.Reference || parameter.Variadic || parameter.DefaultValue != nil ||
			parameter.WireKey != "" || parameter.Codec != "" || parameter.EnumName != "" || len(parameter.EnumValues) != 0 ||
			len(parameter.AcceptedForms) != 1 || parameter.AcceptedForms[0] != "literal" {
			return nil
		}
	}
	if _, err := CacheCallOptions(call); err != nil {
		return nil
	}
	return call.Block.Body
}
