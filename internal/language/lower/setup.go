package lower

import (
	"fmt"

	"github.com/electrikmilk/cherri/internal/language/syntax"
)

type setupQuestionArgument struct{ name, wireKey string }
type setupQuestionLoweringBinding struct {
	question map[string]any
	nodeID   string
	wireKey  string
}

func (l *Lowerer) directSetupQuestion(expression syntax.Expression) (string, bool) {
	identifier, ok := expression.(*syntax.IdentExpr)
	if !ok {
		return "", false
	}
	if _, shadowed := l.bindings[identifier.Name]; shadowed {
		return "", false
	}
	_, ok = l.setupQuestions[identifier.Name]
	return identifier.Name, ok
}

func (l *Lowerer) bindSetupQuestion(name, wireKey, nodeID string) error {
	binding, ok := l.setupBindings[name]
	if !ok || binding.nodeID != "" {
		return fmt.Errorf("E_SETUP_BINDING: unknown or reused setup %q", name)
	}
	if wireKey == "" || nodeID == "" {
		return fmt.Errorf("E_SETUP_BINDING: missing node/parameter for setup %q", name)
	}
	if l.session != nil {
		if err := l.session.BindImportQuestion(name, wireKey, nodeID); err != nil {
			return err
		}
	}
	binding.nodeID, binding.wireKey = nodeID, wireKey
	return nil
}

func (l *Lowerer) finalizeSetupQuestionBindings() error {
	if len(l.setupBindings) == 0 {
		return nil
	}
	questions := make([]map[string]any, 0, len(l.setupBindings))
	// Retain declaration order, independently of parameter-key spelling.
	for _, binding := range l.setupDeclarationOrder {
		if binding == nil || binding.nodeID == "" {
			continue
		}
		question := binding.question
		if l.session == nil {
			found := -1
			for index, node := range l.workflow.Actions {
				if node.NodeID == binding.nodeID {
					if found != -1 {
						return fmt.Errorf("E_SETUP_BINDING: duplicate lowered setup node")
					}
					found = index
				}
			}
			if found < 0 {
				return fmt.Errorf("E_SETUP_BINDING: lowered setup node not found")
			}
			if _, ok := l.workflow.Actions[found].Parameters[binding.wireKey]; !ok {
				return fmt.Errorf("E_SETUP_BINDING: lowered setup parameter not found")
			}
			question["ActionIndex"] = found
		}
		question["ParameterKey"] = binding.wireKey
		questions = append(questions, question)
	}
	l.workflow.ImportQuestions = questions
	return nil
}
