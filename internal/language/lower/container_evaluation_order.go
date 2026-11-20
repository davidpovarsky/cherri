package lower

import (
	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/syntax"
)

// Semantic containers are assembled after all their entry expressions have
// emitted actions. An earlier mutable reference must therefore be consumed
// before a later sibling's assignment, just as a written call argument is.
// Keep the existing native representation when later siblings cannot write it.
func (l *Lowerer) semanticSiblingWrites(expressions []syntax.Expression) []map[string]bool {
	later := make([]map[string]bool, len(expressions))
	writes := map[string]bool{}
	for index := len(expressions) - 1; index >= 0; index-- {
		if len(writes) > 0 {
			later[index] = make(map[string]bool, len(writes))
			for name := range writes {
				later[index][name] = true
			}
		}
		for name := range analysis.OuterAssignedNames(expressions[index]) {
			if binding, found := l.bindings[name]; found && binding.IsVariable {
				writes[binding.VariableName] = true
			}
		}
	}
	return later
}
