package analysis

import (
	"fmt"
	"sort"
	"strings"

	"github.com/electrikmilk/cherri/internal/language/backend"
	"github.com/electrikmilk/cherri/internal/language/syntax"
)

// Validate declaration facts before publishing a document-local schema. An
// unused malformed declaration is still an error, and a bad wire mapping must
// never silently fall back to the source parameter's name.
func (a *Analyzer) validateActionDeclaration(declaration *syntax.ActionDecl) bool {
	valid := true
	schemaError := func(message string) {
		a.error(CodeSchemaMismatch, declaration.Span, message)
		valid = false
	}
	parameters := map[string]bool{}
	for _, parameter := range declaration.Parameters {
		if parameters[parameter.Name] {
			a.error(CodeDuplicateName, parameter.Span, fmt.Sprintf("duplicate action parameter %q", parameter.Name))
			valid = false
		}
		parameters[parameter.Name] = true
	}
	if strings.TrimSpace(declaration.AppleIdentifier) == "" {
		schemaError("custom action identifier must be nonempty")
	}
	if declaration.PrimaryParam != "" && !parameters[declaration.PrimaryParam] {
		schemaError(fmt.Sprintf("primary parameter %q is not declared", declaration.PrimaryParam))
	}
	var names []string
	for name := range declaration.Bindings {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		binding := declaration.Bindings[name]
		if !parameters[name] {
			schemaError(fmt.Sprintf("action binding %q references an undeclared parameter", name))
		}
		if strings.TrimSpace(binding.WireKey) == "" {
			schemaError(fmt.Sprintf("action binding %q requires a nonempty wire key", name))
		}
		if binding.Codec != "" && !backend.IsRegisteredParameterCodec(binding.Codec) {
			schemaError(fmt.Sprintf("action binding %q uses unregistered codec %q", name, binding.Codec))
		}
	}
	wireOwners := map[string]string{}
	for _, parameter := range declaration.Parameters {
		key := parameter.Name
		if binding, present := declaration.Bindings[parameter.Name]; present {
			key = binding.WireKey
		}
		if key == "" {
			continue
		}
		if owner, duplicate := wireOwners[key]; duplicate && owner != parameter.Name {
			schemaError(fmt.Sprintf("action parameters %q and %q share wire key %q", owner, parameter.Name, key))
		}
		wireOwners[key] = parameter.Name
	}
	return valid
}
