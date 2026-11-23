package lower

import (
	"fmt"
	"unicode/utf16"

	"github.com/electrikmilk/cherri/internal/language/ir"
	"github.com/electrikmilk/cherri/internal/language/syntax"
)

// Explicit native compatibility belongs to the actual condition expressions,
// not a dynamic ambient flag that could change a called function's own body.
func (l *Lowerer) markNativeTextComparisonScope(expr syntax.Expression) {
	if expr == nil {
		return
	}
	if l.nativeTextComparisons == nil {
		l.nativeTextComparisons = make(map[syntax.Expression]bool)
	}
	l.nativeTextComparisons[expr] = true
	switch node := expr.(type) {
	case *syntax.BinaryExpr:
		l.markNativeTextComparisonScope(node.Left)
		l.markNativeTextComparisonScope(node.Right)
	case *syntax.UnaryExpr:
		l.markNativeTextComparisonScope(node.Operand)
	case *syntax.MemberExpr:
		l.markNativeTextComparisonScope(node.Target)
	case *syntax.IndexExpr:
		l.markNativeTextComparisonScope(node.Target)
		l.markNativeTextComparisonScope(node.Index)
	case *syntax.CallExpr:
		l.markNativeTextComparisonScope(node.PrimaryArg)
		for _, argument := range node.NamedArgs {
			l.markNativeTextComparisonScope(argument.Value)
		}
		// A lexical block and a called function have their own authored Ifs.
	case *syntax.FStringExpr:
		for _, part := range node.Parts {
			if part.IsExpr {
				l.markNativeTextComparisonScope(part.Expr)
			}
		}
	case *syntax.ListExpr:
		for _, element := range node.Elements {
			l.markNativeTextComparisonScope(element)
		}
	case *syntax.MapExpr:
		for _, entry := range node.Entries {
			l.markNativeTextComparisonScope(entry.Key)
			l.markNativeTextComparisonScope(entry.Value)
		}
	case *syntax.RecordExpr:
		for _, field := range node.Fields {
			l.markNativeTextComparisonScope(field.Value)
		}
	}
}

func (l *Lowerer) exactTextComparison(expr syntax.Expression) bool {
	binary, okay := expr.(*syntax.BinaryExpr)
	return okay && !l.hasImportProvenance && !l.nativeTextComparisons[expr] &&
		(binary.Op == syntax.TokenEqual || binary.Op == syntax.TokenNotEqual) &&
		l.isStringLikeExpr(binary.Left) && l.isStringLikeExpr(binary.Right)
}

func (l *Lowerer) exactTextPredicate(expr syntax.Expression) bool {
	if l.exactTextComparison(expr) {
		return true
	}
	switch node := expr.(type) {
	case *syntax.UnaryExpr:
		return node.Op == syntax.TokenBang && l.exactTextPredicate(node.Operand)
	case *syntax.BinaryExpr:
		return (node.Op == syntax.TokenAnd || node.Op == syntax.TokenOr) &&
			(l.exactTextPredicate(node.Left) || l.exactTextPredicate(node.Right))
	}
	return false
}

// This is a native text-template operation, not fmt.Sprint or a new scalar
// coercion. Its input has already passed the exact static Text gate.
func exactTextTemplate(prefix string, value any, suffix string) (any, error) {
	switch text := value.(type) {
	case string:
		// Plain Text preserves literal U+FFFC; it is not a token placeholder.
		return prefix + text + suffix, nil
	case *ir.AttachmentToken:
		if text == nil {
			return nil, fmt.Errorf("N03_EXACT_TEXT_VALUE: nil Text reference")
		}
		offset := len(utf16.Encode([]rune(prefix)))
		return map[string]any{
			"WFSerializationType": "WFTextTokenString",
			"Value": map[string]any{
				"string": prefix + "\uFFFC" + suffix,
				"attachmentsByRange": map[string]any{
					fmt.Sprintf("{%d, 1}", offset): text,
				},
			},
		}, nil
	case map[string]any:
		if text["WFSerializationType"] != "WFTextTokenString" {
			break
		}
		body, valid := text["Value"].(map[string]any)
		if !valid {
			break
		}
		literal, valid := body["string"].(string)
		if !valid {
			break
		}
		attachments, valid := body["attachmentsByRange"].(map[string]any)
		if !valid {
			break
		}
		offset := len(utf16.Encode([]rune(prefix)))
		shifted := make(map[string]any, len(attachments))
		for key, attachment := range attachments {
			var start, length int
			if count, err := fmt.Sscanf(key, "{%d, %d}", &start, &length); err != nil || count != 2 ||
				start < 0 || length != 1 || key != fmt.Sprintf("{%d, 1}", start) {
				return nil, fmt.Errorf("N03_EXACT_TEXT_VALUE: malformed Text attachment range")
			}
			shifted[fmt.Sprintf("{%d, 1}", start+offset)] = attachment
		}
		return map[string]any{
			"WFSerializationType": "WFTextTokenString",
			"Value": map[string]any{
				"string":             prefix + literal + suffix,
				"attachmentsByRange": shifted,
			},
		}, nil
	}
	return nil, fmt.Errorf("N03_EXACT_TEXT_VALUE: unsupported typed Text carrier %T", value)
}

func (l *Lowerer) exactTextAction(identifier, name string, parameters map[string]any) *ir.AttachmentToken {
	uuid := l.GenerateUUID()
	l.emitAction(&ir.NativeActionNode{
		NodeID: uuid, AppleIdentifier: identifier, OutputUUID: uuid, OutputName: name,
		Parameters: parameters,
	})
	return &ir.AttachmentToken{Type: "ActionOutput", OutputUUID: uuid, OutputName: name}
}

func (l *Lowerer) exactTextSnapshot(value any, name string, preservePrevious bool) (*ir.AttachmentToken, error) {
	payload, err := exactTextTemplate("~", value, "")
	if err != nil {
		return nil, err
	}
	result := l.exactTextAction("is.workflow.actions.gettext", name, map[string]any{"WFTextActionText": payload})
	if l.emitErr != nil {
		return nil, l.emitErr
	}
	if preservePrevious {
		// Register this exact compiler-emitted action, not an authored Text
		// action whose content happens to begin with the sentinel.
		if session, okay := l.session.(interface {
			BindExactTextSnapshotInput(string, any, string, string, func(string) bool) error
		}); okay {
			if err := session.BindExactTextSnapshotInput(result.OutputUUID, payload,
				l.GenerateUUID(), l.GenerateUUID(), l.knownNativeValueOutput); err != nil {
				return nil, err
			}
		}
		if l.literalTextCarriers == nil {
			l.literalTextCarriers = make(map[*ir.NativeActionNode]bool)
		}
		current := l.workflow.Actions[len(l.workflow.Actions)-1]
		l.literalTextCarriers[current] = true
		if l.exactTextSnapshots == nil {
			l.exactTextSnapshots = make(map[*ir.NativeActionNode]bool)
		}
		l.exactTextSnapshots[current] = true
	}
	return result, nil
}

// Exact Text equality is a quoted full-input match. Prefixing both actual Text
// values with the same nonempty sentinel is injective and also avoids a
// zero-length native match or empty Replace Text subject. Embedded \E closes
// quoting only after the literal replacement below has escaped it.
//
// ICU/NSRegularExpression define \A, \z and \Q...\E. The native case-sensitive
// field and primitive identities come from the existing registered actions.
// This source does not claim that a local graph compilation executes Apple.
func (l *Lowerer) lowerExactTextComparison(expr *syntax.BinaryExpr) (any, error) {
	binding := l.detachBindingName()
	defer func() { l.currentBindingName = binding }()

	left, err := l.lowerCallValue(expr.Left)
	if err != nil {
		return nil, err
	}
	var leftSnapshot *ir.AttachmentToken
	if _, literal := left.(string); !literal {
		// Freeze the left value before an effectful right operand. Its pure
		// Text snapshot retains the actual source Previous-input identity.
		leftSnapshot, err = l.exactTextSnapshot(left, "_n03_exact_left", true)
		if err != nil {
			return nil, err
		}
	}
	right, err := l.lowerCallValue(expr.Right)
	if err != nil {
		return nil, err
	}
	if a, firstLiteral := left.(string); firstLiteral {
		if b, secondLiteral := right.(string); secondLiteral {
			if expr.Op == syntax.TokenNotEqual {
				return a != b, nil
			}
			return a == b, nil
		}
		leftSnapshot, err = l.exactTextSnapshot(left, "_n03_exact_left", false)
		if err != nil {
			return nil, err
		}
	}
	rightSnapshot, err := l.exactTextSnapshot(right, "_n03_exact_right", false)
	if err != nil {
		return nil, err
	}
	subject, err := exactTextTemplate("", rightSnapshot, "")
	if err != nil {
		return nil, err
	}
	quoted := l.exactTextAction("is.workflow.actions.text.replace", "_n03_exact_quote", map[string]any{
		"WFReplaceTextFind": "\\E", "WFReplaceTextReplace": "\\E\\\\E\\Q",
		"WFInput": subject, "WFReplaceTextCaseSensitive": true,
		"WFReplaceTextRegularExpression": false,
	})
	pattern, err := exactTextTemplate("\\A\\Q", quoted, "\\E\\z")
	if err != nil {
		return nil, err
	}
	subject, err = exactTextTemplate("", leftSnapshot, "")
	if err != nil {
		return nil, err
	}
	matches := l.exactTextAction("is.workflow.actions.text.match", "_n03_exact_match", map[string]any{
		"WFMatchTextPattern": pattern, "text": subject, "WFMatchTextCaseSensitive": true,
	})
	count := l.exactTextAction("is.workflow.actions.count", "_n03_exact_count", map[string]any{"Input": matches})
	if l.emitErr != nil {
		return nil, l.emitErr
	}
	groupUUID, resultUUID := l.GenerateUUID(), l.GenerateUUID()
	code := 4
	if expr.Op == syntax.TokenNotEqual {
		code = 5
	}
	l.emitIfStart(groupUUID, count, code, 1.0, true)
	l.emitAction(&ir.NativeActionNode{NodeID: l.GenerateUUID(), AppleIdentifier: "is.workflow.actions.number",
		Parameters: map[string]any{"WFNumberActionNumber": 1.0}})
	l.emitIfElse(groupUUID)
	l.emitAction(&ir.NativeActionNode{NodeID: l.GenerateUUID(), AppleIdentifier: "is.workflow.actions.number",
		Parameters: map[string]any{"WFNumberActionNumber": 0.0}})
	l.emitIfEnd(groupUUID, resultUUID, "")
	if l.emitErr != nil {
		return nil, l.emitErr
	}
	return &ir.AttachmentToken{Type: "ActionOutput", OutputUUID: resultUUID}, nil
}

// Query the actual compiler-owned snapshot identity. Pure literal carriers may
// carry the same saved source cursor, but authored Text content never enrolls it.
func (l *Lowerer) hasCurrentExactTextSnapshot() bool {
	if session, okay := l.session.(interface{ HasCurrentExactTextSnapshot() bool }); okay {
		return session.HasCurrentExactTextSnapshot()
	}
	for index := len(l.workflow.Actions) - 1; index >= 0 && l.literalTextCarriers[l.workflow.Actions[index]]; index-- {
		if l.exactTextSnapshots[l.workflow.Actions[index]] {
			return true
		}
	}
	return false
}

// User-declared actions emit the same native parameter dictionary as an escape.
// Only an actual pending compiler-owned snapshot introduces the preservation
// boundary; ordinary/custom calls outside this route retain original emission.
func (l *Lowerer) restoreExactTextSnapshotForDeclaredAction(target *ir.NativeActionNode) error {
	if !l.hasCurrentExactTextSnapshot() {
		return nil
	}
	return l.restoreNativeInputAfterLiteralCarriers(target)
}
