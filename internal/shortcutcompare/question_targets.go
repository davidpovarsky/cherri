package shortcutcompare

import "math"

type parityQuestionTarget struct {
	question map[string]any
	origin   int
}

type parityQuestionOrigins struct {
	key     string
	targets []parityQuestionTarget
}

// Questions address the original native action array. Carry exact action
// identity through existing normalization before comparing the resulting index.
// No parameter, prompt or default is discarded, and identical actions retain
// distinct identities. Unrepresentable/removed targets disable normalization.
func bindParityQuestionTargets(document map[string]any, actions []any) (*parityQuestionOrigins, bool) {
	raw, exists := document["WFWorkflowImportQuestions"]
	if !exists {
		return nil, true
	}
	questions, okay := raw.([]any)
	if !okay {
		return nil, false
	}
	if len(questions) == 0 {
		return nil, true
	}
	result := &parityQuestionOrigins{key: "__cherri_parity_question_origin"}
	for parityContainsKey(document, result.key) {
		result.key += "_"
	}
	for _, rawQuestion := range questions {
		question, okay := rawQuestion.(map[string]any)
		if !okay {
			return nil, false
		}
		index, okay := toNumeric(question["ActionIndex"])
		if !okay || math.IsNaN(index) || math.IsInf(index, 0) || index < 0 || index >= float64(len(actions)) || math.Trunc(index) != index {
			return nil, false
		}
		action, okay := actions[int(index)].(map[string]any)
		if !okay || action == nil {
			return nil, false
		}
		result.targets = append(result.targets, parityQuestionTarget{question: question, origin: int(index)})
	}
	for _, target := range result.targets {
		actions[target.origin].(map[string]any)[result.key] = target.origin
	}
	return result, true
}

func rebindParityQuestionTargets(document map[string]any, actions []any, origins *parityQuestionOrigins) bool {
	if origins == nil {
		return true
	}
	indices, counts := map[int]int{}, map[int]int{}
	for index, raw := range actions {
		action, okay := raw.(map[string]any)
		if !okay {
			continue
		}
		if origin, okay := action[origins.key].(int); okay {
			indices[origin] = index
			counts[origin]++
		}
	}
	for _, target := range origins.targets {
		if counts[target.origin] != 1 {
			return false
		}
	}
	for _, target := range origins.targets {
		target.question["ActionIndex"] = indices[target.origin]
	}
	stripParityOriginKey(document, origins.key)
	return true
}

func parityContainsKey(value any, key string) bool {
	switch node := value.(type) {
	case map[string]any:
		if _, found := node[key]; found {
			return true
		}
		for _, child := range node {
			if parityContainsKey(child, key) {
				return true
			}
		}
	case []any:
		for _, child := range node {
			if parityContainsKey(child, key) {
				return true
			}
		}
	}
	return false
}

func stripParityOriginKey(value any, key string) {
	switch node := value.(type) {
	case map[string]any:
		delete(node, key)
		for _, child := range node {
			stripParityOriginKey(child, key)
		}
	case []any:
		for _, child := range node {
			stripParityOriginKey(child, key)
		}
	}
}
