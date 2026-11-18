package schema

import (
	"fmt"

	"github.com/electrikmilk/cherri/internal/language/source"
	"github.com/electrikmilk/cherri/internal/language/syntax"
)

// VariadicListError identifies an ambiguous argument pack, with its original
// source span retained for the ordinary duplicate-argument diagnostic.
type VariadicListError struct {
	Span    source.Span
	Message string
}

func (e *VariadicListError) Error() string { return e.Message }

// ExpandNamedVariadicLists expands exactly one outer, explicitly named List
// expression into the existing canonical argument vector. Children retain their
// identity, spans and written order. Nested lists and runtime list references
// remain individual values: this is not a recursive or runtime spread operation.
// The caller and original syntax tree are never mutated.
func ExpandNamedVariadicLists(call *syntax.CallExpr, action *ActionSchema) (*syntax.CallExpr, *VariadicListError) {
	var packs = make(map[int]*syntax.ListExpr)
	var counts = make(map[int]int)
	var packByArgument = make(map[int]int)
	bound := make(map[int]bool)
	if call.PrimaryArg != nil {
		if primary, ok := action.PrimaryParameter(); ok {
			for index := range action.Parameters {
				if &action.Parameters[index] == primary {
					bound[index] = true
					counts[index]++
					break
				}
			}
		}
	}
	positional := 0
	for index, argument := range call.NamedArgs {
		matched := -1
		if argument.Label != "" {
			for parameterIndex, parameter := range action.Parameters {
				if argument.Label == parameter.Label || argument.Label == parameter.ID {
					if matched < 0 {
						matched = parameterIndex
					}
					if !bound[parameterIndex] {
						matched = parameterIndex
						break
					}
				}
			}
		} else {
			for positional < len(action.Parameters) && bound[positional] {
				positional++
			}
			if positional < len(action.Parameters) {
				matched = positional
				positional++
			} else if len(action.Parameters) > 0 && action.Parameters[len(action.Parameters)-1].Variadic {
				matched = len(action.Parameters) - 1
			}
		}
		if matched < 0 {
			continue
		}
		bound[matched] = true
		counts[matched]++
		if !action.Parameters[matched].Variadic || argument.Label == "" {
			continue
		}
		if list, ok := argument.Value.(*syntax.ListExpr); ok {
			packs[matched] = list
			packByArgument[index] = matched
		}
	}
	if len(packs) == 0 {
		return call, nil
	}
	for index, argument := range call.NamedArgs {
		if matched, ok := packByArgument[index]; ok && counts[matched] > 1 {
			return nil, &VariadicListError{Span: argument.Span, Message: fmt.Sprintf("explicit variadic list for %q cannot be combined with another argument for the same parameter", action.Parameters[matched].Label)}
		}
	}
	expanded := *call
	expanded.NamedArgs = make([]syntax.NamedArg, 0, len(call.NamedArgs))
	for index, argument := range call.NamedArgs {
		if matched, ok := packByArgument[index]; ok {
			for _, item := range packs[matched].Elements {
				expanded.NamedArgs = append(expanded.NamedArgs, syntax.NamedArg{Span: item.NodeSpan(), Label: argument.Label, Value: item})
			}
		} else {
			expanded.NamedArgs = append(expanded.NamedArgs, argument)
		}
	}
	return &expanded, nil
}
