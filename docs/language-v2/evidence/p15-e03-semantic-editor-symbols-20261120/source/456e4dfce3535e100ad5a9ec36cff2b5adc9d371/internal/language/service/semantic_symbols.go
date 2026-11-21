package service

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/electrikmilk/cherri/internal/language/source"
	"github.com/electrikmilk/cherri/internal/language/syntax"
)

func validDocumentPosition(doc *DocumentState, line, col int) bool {
	if doc == nil || doc.File == nil || line < 1 || col < 1 {
		return false
	}
	lines := strings.Split(doc.File.Content, "\n")
	return line <= len(lines) && col <= utf8.RuneCountInString(lines[line-1])+1
}

// A lexical word is not by itself a reference. The exact AST-owned span must
// agree, so quoted text, comments, member fields, type names and action labels
// cannot acquire the identity of a same-spelled local declaration.
func semanticIdentifierAt(doc *DocumentState, line, col int) (identRef, []scopedDeclaration, []identRef, bool) {
	if !validDocumentPosition(doc, line, col) || doc.Program == nil {
		return identRef{}, nil, nil, false
	}
	lines := strings.Split(doc.File.Content, "\n")
	word, start, end, found := extractWordAtRuneCol(lines[line-1], col)
	if !found {
		return identRef{}, nil, nil, false
	}
	startOffset := byteOffsetAtLineCol(doc.File, line, start+1)
	endOffset := byteOffsetAtLineCol(doc.File, line, end+1)
	declarations, references := collectScopesAndRefs(doc.File, doc.Program)
	for _, ref := range references {
		if ref.name == word && ref.span.Start.ByteOffset == startOffset && ref.span.End.ByteOffset == endOffset {
			return ref, declarations, references, true
		}
	}
	return identRef{}, declarations, references, false
}

func resolveIdentifierReference(declarations []scopedDeclaration, ref identRef) *scopedDeclaration {
	if ref.declaration != nil {
		for i := range declarations {
			if declarations[i].declSpan == *ref.declaration {
				return &declarations[i]
			}
		}
		return nil
	}
	return resolveDeclForRef(declarations, ref.name, ref.span.Start.ByteOffset)
}

func sameDeclaration(a, b *scopedDeclaration) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.declSpan == b.declSpan
}

// Test resolution at original source coordinates before constructing edits.
// Renamed uses and all untouched uses must retain the same declaration, and
// unresolved canonical/global references must remain unresolved locally.
func validateRenameIdentity(declarations []scopedDeclaration, references []identRef, target *scopedDeclaration, newName string) error {
	changed := append([]scopedDeclaration{}, declarations...)
	for i := range changed {
		decl := &changed[i]
		if sameDeclaration(decl, target) {
			decl.name = newName
			continue
		}
		if decl.ownerSpan == target.ownerSpan {
			collision := decl.name == newName
			if decl.parameter && target.parameter {
				collision = strings.TrimPrefix(decl.name, "_") == strings.TrimPrefix(newName, "_")
			}
			if collision {
				return fmt.Errorf("rename would duplicate declaration %q in the same lexical scope", newName)
			}
		}
	}
	for _, ref := range references {
		before := resolveIdentifierReference(declarations, ref)
		afterRef := ref
		if sameDeclaration(before, target) {
			afterRef.name = newName
		}
		after := resolveIdentifierReference(changed, afterRef)
		if !sameDeclaration(before, after) {
			return fmt.Errorf("rename would change the semantic owner of %q at %s", ref.name, ref.span)
		}
	}
	return nil
}

func authoredFunctionForDeclaration(doc *DocumentState, decl *scopedDeclaration) *syntax.FunctionDecl {
	if doc == nil || doc.Program == nil || decl == nil || !decl.isFunc {
		return nil
	}
	for _, declaration := range doc.Program.Declarations {
		if fn, ok := declaration.(*syntax.FunctionDecl); ok && fn.Name == decl.name &&
			fn.Span.Start.ByteOffset <= decl.declSpan.Start.ByteOffset && fn.Span.End.ByteOffset >= decl.declSpan.End.ByteOffset {
			return fn
		}
	}
	return nil
}

// User-function labels are owned by that function's parameter, not by an
// unrelated local at the call site. Canonical action labels remain unbound.
func appendParameterLabelReferences(file *source.File, declarations []scopedDeclaration, references []identRef, functions map[int]*syntax.FunctionDecl, calls []*syntax.CallExpr) []identRef {
	for _, call := range calls {
		callee, ok := call.Callee.(*syntax.IdentExpr)
		if !ok {
			continue
		}
		owner := resolveDeclForRef(declarations, callee.Name, callee.Span.Start.ByteOffset)
		if owner == nil || !owner.isFunc {
			continue
		}
		function := functions[owner.declSpan.Start.ByteOffset]
		if function == nil {
			continue
		}
		for _, argument := range call.NamedArgs {
			for _, parameter := range function.Parameters {
				if strings.TrimPrefix(parameter.Name, "_") != argument.Label {
					continue
				}
				start := argument.Span.Start.ByteOffset
				end := start + len(argument.Label)
				if start < 0 || end > len(file.Content) || file.Content[start:end] != argument.Label {
					continue
				}
				declaration := parameter.Span
				references = append(references, identRef{
					name: argument.Label, span: file.SpanForOffsets(start, end),
					declaration: &declaration, label: true,
				})
			}
		}
	}
	return references
}

func functionSignature(doc *DocumentState, fn *syntax.FunctionDecl, context enumCallContext) *SignatureHelpResult {
	params := make([]string, len(fn.Parameters))
	parameters := make([]enumCompletionParameter, len(fn.Parameters))
	for i, p := range fn.Parameters {
		params[i] = parameterSignature(doc.File, p)
		parameters[i] = enumCompletionParameter{name: p.Name, label: strings.TrimPrefix(p.Name, "_")}
	}
	active := len(context.arguments) - 1
	if index, _ := activeCallParameter(parameters, 0, context); index >= 0 {
		active = index
	}
	if active >= len(params) && len(params) > 0 {
		active = len(params) - 1
	}
	return &SignatureHelpResult{
		Label:      fmt.Sprintf("function %s(%s) -> %s", fn.Name, strings.Join(params, ", "), annotationText(doc.File, fn.ReturnType, "Void")),
		Parameters: params, ActiveParameter: active,
	}
}
