package migrate

import (
	"fmt"
	"sort"
	"strings"

	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/source"
	"github.com/electrikmilk/cherri/internal/language/syntax"
)

// ReconstructNativeListExpressions applies only to source produced by the
// native semantic importer. A native List may contain typed output references;
// the authored list(...) action instead promises text rows. Keep the imported
// content references in a List expression without weakening that action's type.
func ReconstructNativeListExpressions(input string) (string, error) {
	file := source.NewFile(source.DocumentID("semantic-native-lists"), "semantic-native-lists", 1, input)
	parser := syntax.NewParser(file)
	program := parser.ParseProgram()
	if errors := parser.Errors(); len(errors) != 0 {
		return "", fmt.Errorf("invalid semantic import source: %s", errors[0].Message)
	}
	action, ok := schema.DefaultRegistry().LookupAction("list")
	if !ok || !action.Builtin || action.AppleIdentifier != "is.workflow.actions.list" ||
		len(action.Parameters) != 1 || !action.Parameters[0].Variadic {
		return "", fmt.Errorf("canonical native List descriptor is unavailable")
	}
	declared := map[string]bool{}
	for _, declaration := range program.Declarations {
		switch item := declaration.(type) {
		case *syntax.FunctionDecl:
			declared[item.Name] = true
		case *syntax.ActionDecl:
			declared[item.Name] = true
		}
	}
	type edit struct {
		start, end int
		value      string
	}
	var edits []edit
	generated := 0
	walkMigrationBindings(program, nil, nil, func(call *syntax.CallExpr, scope *migrationBindingScope, standalone *syntax.ExprStmt) {
		callee, ok := call.Callee.(*syntax.IdentExpr)
		if !ok || callee.Name != action.CallableName || declared[callee.Name] || call.Block != nil {
			return
		}
		for current := scope; current != nil; current = current.parent {
			if _, shadowed := current.names[callee.Name]; shadowed {
				return
			}
		}
		if call.PrimaryArg != nil || len(call.NamedArgs) != 1 {
			return
		}
		argument := call.NamedArgs[0]
		parameter := action.Parameters[0]
		if argument.Label != parameter.Label && argument.Label != parameter.ID {
			return
		}
		items, ok := argument.Value.(*syntax.ListExpr)
		if !ok {
			return
		}
		prefix := ""
		if standalone != nil {
			// A bare List expression has no consumer and the lowerer can discard
			// it. An immutable binding retains the original native List action.
			var name string
			for {
				generated++
				name = fmt.Sprintf("__nativeList%d", generated)
				if !strings.Contains(input, name) {
					break
				}
			}
			prefix = "let " + name + " = "
		}
		edits = append(edits,
			edit{call.Span.Start.ByteOffset, items.Span.Start.ByteOffset, prefix},
			edit{items.Span.End.ByteOffset, call.Span.End.ByteOffset, ""},
		)
	})
	sort.Slice(edits, func(i, j int) bool { return edits[i].start > edits[j].start })
	previous := len(input)
	for _, change := range edits {
		if change.start < 0 || change.end < change.start || change.end > previous {
			return "", fmt.Errorf("overlapping or invalid native List source spans")
		}
		input = input[:change.start] + change.value + input[change.end:]
		previous = change.start
	}
	return input, nil
}
