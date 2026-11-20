package analysis

import (
	"strings"

	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/source"
	"github.com/electrikmilk/cherri/internal/language/syntax"
	"github.com/electrikmilk/cherri/internal/language/types"
)

// Editor facts are collected only by the shared language service. They retain
// the analyzer's actual scope at each written argument; no editor type parser
// or global-name inference supplies compatibility or flow guarantees.
type EditorCallParameter struct {
	Name, Label string
	Type        types.Type
}

type EditorCallInfo struct {
	Callee     source.Span
	Parameters []EditorCallParameter
	Primary    int
}

type editorCallFacts struct {
	info   EditorCallInfo
	values map[int]map[string]types.Type
}

type editorArgumentOwner struct {
	call  *syntax.CallExpr
	index int
}

func (a *Analyzer) EnableEditorContexts() { a.editorContextsEnabled = true }

func (a *Analyzer) resetEditorContexts() {
	a.editorCalls = nil
	a.editorArguments = nil
	if a.editorContextsEnabled {
		a.editorCalls = make(map[*syntax.CallExpr]*editorCallFacts)
		a.editorArguments = make(map[syntax.Expression]editorArgumentOwner)
	}
}

func editorScopeTypes(scope *Scope) map[string]types.Type {
	result := make(map[string]types.Type)
	seen := make(map[string]bool)
	for current := scope; current != nil; current = current.parent {
		for name, symbol := range current.symbols {
			if seen[name] {
				continue
			}
			seen[name] = true
			if symbol == nil || !symbol.Initialized || symbol.Span.Start.Line == 0 ||
				symbol.Function != nil || symbol.Module != nil || symbol.Setup != nil || symbol.EnumDeclaration {
				continue
			}
			result[name] = symbol.readType()
		}
	}
	return result
}

func (a *Analyzer) beginEditorCall(call *syntax.CallExpr, scope *Scope) {
	if !a.editorContextsEnabled || call == nil || call.Callee == nil {
		return
	}
	a.editorCalls[call] = &editorCallFacts{
		info:   EditorCallInfo{Callee: call.Callee.NodeSpan(), Primary: -1},
		values: map[int]map[string]types.Type{0: editorScopeTypes(scope)},
	}
	index := 0
	if call.PrimaryArg != nil {
		a.editorArguments[call.PrimaryArg] = editorArgumentOwner{call, index}
		index++
	}
	for _, argument := range call.NamedArgs {
		if argument.Value != nil {
			a.editorArguments[argument.Value] = editorArgumentOwner{call, index}
		}
		index++
	}
}

func (a *Analyzer) recordEditorArgument(expr syntax.Expression, scope *Scope, after bool) {
	if !a.editorContextsEnabled {
		return
	}
	owner, found := a.editorArguments[expr]
	if !found {
		return
	}
	facts := a.editorCalls[owner.call]
	if facts == nil {
		return
	}
	index := owner.index
	if after {
		index++
	}
	facts.values[index] = editorScopeTypes(scope)
}

func (a *Analyzer) setEditorFunction(call *syntax.CallExpr, fn *syntax.FunctionDecl) {
	facts := a.editorCalls[call]
	if facts == nil || fn == nil {
		return
	}
	for _, parameter := range fn.Parameters {
		facts.info.Parameters = append(facts.info.Parameters, EditorCallParameter{
			Name: parameter.Name, Label: strings.TrimPrefix(parameter.Name, "_"),
			Type: a.resolveType(parameter.TypeExpr),
		})
	}
	if len(facts.info.Parameters) != 0 {
		facts.info.Primary = 0
	}
}

func (a *Analyzer) setEditorAction(call *syntax.CallExpr, action *schema.ActionSchema) {
	facts := a.editorCalls[call]
	if facts == nil || action == nil {
		return
	}
	for index, parameter := range action.Parameters {
		facts.info.Parameters = append(facts.info.Parameters, EditorCallParameter{
			Name: parameter.ID, Label: parameter.Label, Type: parameter.Type,
		})
		if primary, found := action.PrimaryParameter(); found && primary == &action.Parameters[index] {
			facts.info.Primary = index
		}
	}
}

// EditorCallAt uses exact original callee identity and written argument order.
// Missing/ambiguous source facts never acquire guarantees from a nearby call.
func (a *Analyzer) EditorCallAt(calleeEnd, argumentIndex int) (EditorCallInfo, map[string]types.Type, bool) {
	var matched *editorCallFacts
	for _, facts := range a.editorCalls {
		if facts.info.Callee.End.ByteOffset != calleeEnd {
			continue
		}
		if matched != nil {
			return EditorCallInfo{}, nil, false
		}
		matched = facts
	}
	if matched == nil || len(matched.info.Parameters) == 0 {
		return EditorCallInfo{}, nil, false
	}
	info := matched.info
	info.Parameters = append([]EditorCallParameter{}, info.Parameters...)
	values := make(map[string]types.Type)
	for name, value := range matched.values[argumentIndex] {
		values[name] = value
	}
	return info, values, true
}
