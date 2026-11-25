package lower

import (
	"fmt"

	"github.com/electrikmilk/cherri/internal/language/syntax"
)

type lexicalLoweringState struct {
	privatePrefix         string
	scopes                []lexicalBindingScope
	nameCounter           int
	indexedActions        int
	claimedImports        map[string]bool
	materializedOutputs   map[string]bool
	materializedVariables map[string]bool
}

type lexicalBindingRestore struct {
	binding    BindingRef
	existed    bool
	provenance bool
}
type lexicalBindingScope map[string]lexicalBindingRestore

func (l *Lowerer) beginLexicalScope() func() {
	scope := lexicalBindingScope{}
	l.lexical.scopes = append(l.lexical.scopes, scope)
	return func() {
		for name, old := range scope {
			if old.existed {
				l.bindings[name] = old.binding
			} else {
				delete(l.bindings, name)
			}
			if old.provenance {
				l.provenanceSymbols[name] = true
			} else {
				delete(l.provenanceSymbols, name)
			}
		}
		l.lexical.scopes = l.lexical.scopes[:len(l.lexical.scopes)-1]
	}
}

func (l *Lowerer) rememberLexicalBinding(name string, old BindingRef, existed bool) {
	if len(l.lexical.scopes) == 0 {
		return
	}
	scope := l.lexical.scopes[len(l.lexical.scopes)-1]
	if _, already := scope[name]; !already {
		scope[name] = lexicalBindingRestore{old, existed, l.provenanceSymbols[name]}
	}
}

// Import provenance pins original native identities. It does not exempt a later
// authored declaration with the same spelling from lexical scope. Track actual
// emitted identities (including opaque raw producers), not just symbol names.
func (l *Lowerer) claimPendingImportedBinding(name string) bool {
	if !l.provenanceSymbols[name] {
		return false
	}
	if l.lexical.claimedImports == nil {
		l.lexical.claimedImports = map[string]bool{}
	}
	if l.lexical.claimedImports[name] {
		return false
	}
	if l.lexical.materializedOutputs == nil {
		l.lexical.materializedOutputs = map[string]bool{}
	}
	if l.lexical.materializedVariables == nil {
		l.lexical.materializedVariables = map[string]bool{}
	}
	for l.lexical.indexedActions < len(l.workflow.Actions) {
		node := l.workflow.Actions[l.lexical.indexedActions]
		l.lexical.indexedActions++
		if node.OutputUUID != "" {
			l.lexical.materializedOutputs[node.OutputUUID] = true
		}
		if uuid, ok := node.Parameters["UUID"].(string); ok && uuid != "" {
			l.lexical.materializedOutputs[uuid] = true
		}
		if node.AppleIdentifier == "is.workflow.actions.setvariable" || node.AppleIdentifier == "is.workflow.actions.appendvariable" {
			if variable, ok := node.Parameters["WFVariableName"].(string); ok && variable != "" {
				l.lexical.materializedVariables[variable] = true
			}
		}
	}
	reference, found := l.bindings[name]
	if !found {
		return false
	}
	if reference.IsVariable {
		if l.lexical.materializedVariables[reference.VariableName] {
			return false
		}
	} else if reference.OutputUUID == "" || l.lexical.materializedOutputs[reference.OutputUUID] {
		return false
	}
	l.lexical.claimedImports[name] = true
	return true
}

func (l *Lowerer) freshLexicalName(name string) string {
	return l.freshLexicalNameWithPrefix("__cherri_scope", name)
}

// Source declarations inside scoped constructs retain their distinct namespace;
// internal cell/presence helpers keep their existing role-specific namespace.
func (l *Lowerer) freshLexicalBindingName(name string) string {
	prefix := l.lexical.privatePrefix
	if prefix == "" {
		prefix = "__cherri_scope"
	}
	return l.freshLexicalNameWithPrefix(prefix, name)
}

func (l *Lowerer) freshLexicalNameWithPrefix(prefix, name string) string {
	for {
		l.lexical.nameCounter++
		candidate := fmt.Sprintf("%s_%d_%s", prefix, l.lexical.nameCounter, name)
		if _, found := l.bindings[candidate]; found {
			continue
		}
		collision := false
		for _, reference := range l.bindings {
			if reference.IsVariable && reference.VariableName == candidate {
				collision = true
				break
			}
		}
		if !collision {
			for _, variable := range l.provenanceVarNames {
				if variable == candidate {
					collision = true
					break
				}
			}
		}
		if !collision {
			return candidate
		}
	}
}

func (l *Lowerer) lowerLexicalBinding(binding *syntax.BindingStmt) error {
	if l.lexical.privatePrefix == "" && l.claimPendingImportedBinding(binding.Name) {
		// This is the original pinned definition; existing native references
		// intentionally remain usable across the native control region.
		return l.lowerBinding(binding)
	}
	_, shadowed := l.bindings[binding.Name]
	if !shadowed && l.lexical.privatePrefix == "" {
		if err := l.lowerBinding(binding); err != nil {
			return err
		}
		l.rememberLexicalBinding(binding.Name, BindingRef{}, false)
		delete(l.provenanceSymbols, binding.Name)
		return nil
	}

	// Only the backend declaration name changes. The original structured RHS,
	// source spans and expression-type/enum maps remain intact. Its references
	// resolve before the new binding exists, including side effects in a body.
	var localName = l.freshLexicalBindingName(binding.Name)
	if err := l.lowerBindingNamed(binding, localName); err != nil {
		return err
	}
	previous, existed := l.bindings[binding.Name]
	l.rememberLexicalBinding(binding.Name, previous, existed)
	l.bindings[binding.Name] = l.bindings[localName]
	delete(l.bindings, localName)
	delete(l.provenanceSymbols, binding.Name)
	return nil
}

func (l *Lowerer) lexicalLoopVariableName(name string) string {
	if l.lexical.privatePrefix == "" && l.claimPendingImportedBinding(name) {
		return l.effectiveVariableName(name)
	}
	previous, existed := l.bindings[name]
	l.rememberLexicalBinding(name, previous, existed)
	delete(l.provenanceSymbols, name)
	if existed || l.lexical.privatePrefix != "" {
		return l.freshLexicalBindingName(name)
	}
	return l.effectiveVariableName(name)
}

func (l *Lowerer) lowerLexicalBlock(body *syntax.BlockStmt) error {
	restore := l.beginLexicalScope()
	defer restore()
	for _, statement := range body.Statements {
		if err := l.lowerStatement(statement); err != nil {
			return err
		}
	}
	return nil
}
