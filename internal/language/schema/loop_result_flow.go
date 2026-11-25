package schema

import "github.com/electrikmilk/cherri/internal/language/syntax"

// Origin sets are immutable after creation. Sharing a set cannot make an
// immutable alias follow later writes to its mutable source.
type loopResultOriginSet map[syntax.Expression]bool

func mergeLoopResultOrigins(sets ...loopResultOriginSet) loopResultOriginSet {
	var merged loopResultOriginSet
	for _, origins := range sets {
		for origin := range origins {
			if merged == nil {
				merged = loopResultOriginSet{}
			}
			merged[origin] = true
		}
	}
	return merged
}

func sameLoopResultOrigins(left, right loopResultOriginSet) bool {
	if len(left) != len(right) {
		return false
	}
	for origin := range left {
		if !right[origin] {
			return false
		}
	}
	return true
}

type loopResultLocalBinding struct {
	origins     loopResultOriginSet
	declaration *syntax.BindingStmt
}

type loopResultEnvironment struct {
	names  map[string]loopResultLocalBinding
	values map[*syntax.BindingStmt]loopResultOriginSet
}

func newLoopResultEnvironment() loopResultEnvironment {
	return loopResultEnvironment{
		names:  map[string]loopResultLocalBinding{},
		values: map[*syntax.BindingStmt]loopResultOriginSet{},
	}
}

func (scope loopResultEnvironment) clone() loopResultEnvironment {
	var copy = newLoopResultEnvironment()
	for name, binding := range scope.names {
		copy.names[name] = binding
	}
	for binding, origins := range scope.values {
		copy.values[binding] = origins
	}
	return copy
}

func (scope loopResultEnvironment) origins(name string) loopResultOriginSet {
	var binding = scope.names[name]
	if binding.declaration != nil && binding.declaration.Mutable {
		return scope.values[binding.declaration]
	}
	return binding.origins
}

func (scope loopResultEnvironment) declare(binding *syntax.BindingStmt, origins loopResultOriginSet) {
	scope.names[binding.Name] = loopResultLocalBinding{origins: origins, declaration: binding}
	if binding.Mutable && len(origins) != 0 {
		scope.values[binding] = origins
	}
}

// Only declarations visible on entry can be written through a lexical exit.
// Keeping values by declaration identity preserves an outer write even when
// a later declaration in the same block shadows its spelling.
func (scope loopResultEnvironment) takeWrites(local loopResultEnvironment) {
	for binding := range scope.values {
		scope.values[binding] = local.values[binding]
	}
}

func (scope loopResultEnvironment) join(paths ...loopResultEnvironment) bool {
	var changed bool
	for binding, previous := range scope.values {
		var origins loopResultOriginSet
		for _, path := range paths {
			origins = mergeLoopResultOrigins(origins, path.values[binding])
		}
		if !sameLoopResultOrigins(previous, origins) {
			changed = true
		}
		scope.values[binding] = origins
	}
	return changed
}
