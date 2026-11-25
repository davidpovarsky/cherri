package main

import (
	"fmt"
	"maps"
	"strings"

	"github.com/electrikmilk/cherri/internal/language/backend"
	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/types"
)

// Legacy retains its existing parsed token/action/value carriers. A scoped
// body is parsed once by parse(), never rendered as text or reparsed by V2.
// Each step captures the lexical environment before its declaration, so the
// existing inline-text and reference codecs see the right binding even when a
// later declaration shadows that name.
type legacyScopedStep struct {
	tokens   []token
	vars     map[string]varValue
	bindings map[string]string
}

type legacyScopedBlock struct {
	label      string
	prefix     string
	next       int
	steps      []legacyScopedStep
	result     actionArgument
	resultStep legacyScopedStep
	resultType tokenType
}

type legacyScopedFrame struct {
	vars     map[string]varValue
	bindings map[string]string
	declared map[string]bool
}

type legacyScopedParseState struct {
	block     *legacyScopedBlock
	baseDepth int
	bindings  map[string]string
	declared  map[string]bool
	frames    map[int]legacyScopedFrame
	step      *legacyScopedStep
}

var legacyScopedParsing *legacyScopedParseState

func legacyScopedBlockDiagnostic(name string) string {
	if contract, found := schema.DefaultRegistry().LookupAction(name); found && contract.ScopedBlock {
		return fmt.Sprintf("E_BLOCK_UNSUPPORTED: %s has no Legacy scoped-body contract", name)
	}
	return ""
}

func legacyScopedBlockAvailable(name string) bool {
	contract, found := schema.DefaultRegistry().LookupAction(name)
	return found && contract.ScopedBlock && name == "measure"
}

func legacyScopedMayShadow(name string, constant bool) bool {
	if legacyScopedParsing == nil || !constant {
		return false
	}
	if legacyScopedParsing.declared[name] {
		parserError("E_BLOCK_REDECLARATION: a scoped binding may be declared only once in its lexical scope")
	}
	return true
}

func (block *legacyScopedBlock) bindingName() string {
	block.next++
	return fmt.Sprintf("%s_%d", block.prefix, block.next)
}

func legacyScopedControlResultAhead() bool {
	if legacyScopedParsing == nil {
		return false
	}
	ahead := strings.TrimSpace(lookAheadUntil('\n'))
	for _, keyword := range []string{"if ", "menu ", "repeat ", "for "} {
		if strings.HasPrefix(ahead, keyword) {
			return true
		}
	}
	return false
}

func legacyScopedAppendBinding(name string, operation, valueType tokenType, value any, constant bool) {
	state := legacyScopedParsing
	old, existed := variables[name]
	target := name
	if constant {
		if state.declared[name] {
			parserError("E_BLOCK_REDECLARATION: a scoped binding may be declared only once in its lexical scope")
		}
		target = state.block.bindingName()
	} else if local, found := state.bindings[name]; found {
		target = local
	} else if !existed {
		target = state.block.bindingName()
	} else if old.constant {
		parserError("E_BLOCK_CONSTANT_ASSIGNMENT: a captured constant cannot be reassigned")
	}
	// A scoped const reference snapshots the existing typed object. Retaining a
	// textual alias would accidentally retarget it after a later declaration.
	if constant && valueType == Variable {
		kind := legacyScopedValueType(actionArgument{valueType: valueType, value: value}, variables, nil)
		definition := *actions["getVariable"]
		definition.outputType = kind
		value = action{ident: "getVariable", def: &definition,
			args: []actionArgument{{valueType: valueType, value: value}}}
		valueType = Action
	}
	definition := varValue{variableType: "Variable", valueType: valueType, value: value, constant: constant}
	tokens = append(tokens, token{typeof: operation, ident: target, valueType: valueType, value: value})
	// The target's definition is available during generation, but the source
	// name still resolves through the pre-declaration environment for its RHS.
	state.step.vars[target] = definition
	variables[name] = definition
	variables[target] = definition
	if target != name {
		state.bindings[name] = target
		state.declared[name] = true
	}
}

func collectLegacyScopedBlockAction(name string) action {
	previousAction, previousSaved := currentAction, savedAction
	defer func() { currentAction, savedAction = previousAction, previousSaved }()
	definition := &actionDefinition{
		parameters: []parameterDefinition{{name: "label", validType: String, literal: true}},
		outputType: Variable,
	}
	advance() // '('
	setCurrentAction(name, definition)
	arguments := collectArguments()
	currentAction.arguments = arguments
	checkAction()
	if len(arguments) != 1 || (arguments[0].valueType != String && arguments[0].valueType != RawString) {
		parserError("E_BLOCK_LABEL: measure requires one non-empty literal label")
	}
	label, ok := arguments[0].value.(string)
	if !ok || strings.TrimSpace(label) == "" ||
		(arguments[0].valueType == String && checkInlineVarRegex.MatchString(label)) {
		parserError("E_BLOCK_LABEL: measure requires a non-empty literal label; interpolated labels are unsupported")
	}
	if char != ')' {
		parserError("E_BLOCK_LABEL: expected the end of the measure label")
	}
	advance()
	skipInlineWhitespace()
	if char != '{' {
		parserError("E_BLOCK_REQUIRED: measure requires a scoped body on the call's line")
	}
	salt := fmt.Sprintf("measure:%d:%d:%s", lineIdx, idx, label)
	block := &legacyScopedBlock{label: label, prefix: "__cherri_block_" + strings.ReplaceAll(createUUID(&salt), "-", "")}
	advance()
	previousTokens, previousVars, previousState := tokens, variables, legacyScopedParsing
	tokens = nil
	variables = maps.Clone(variables)
	state := &legacyScopedParseState{
		block: block, baseDepth: groupingIdx, bindings: map[string]string{},
		declared: map[string]bool{}, frames: map[int]legacyScopedFrame{},
	}
	legacyScopedParsing = state
	defer func() { tokens, variables, legacyScopedParsing = previousTokens, previousVars, previousState }()

	explicitResult := false
	for char != -1 {
		if char == '#' {
			parserError("E_BLOCK_DIRECTIVE: workflow metadata, questions and raw declarations belong outside a scoped body")
		}
		if char == '}' && groupingIdx == state.baseDepth {
			break
		}
		// Each menu case has its own lexical locals. Reuse the existing menu
		// parser; only restore the captured binding environment here.
		if strings.TrimSpace(lookAheadUntil(' ')) == "item" {
			if frame, exists := state.frames[groupingIdx]; exists {
				variables = maps.Clone(frame.vars)
				state.bindings, state.declared = maps.Clone(frame.bindings), map[string]bool{}
			}
		}
		step := legacyScopedStep{vars: maps.Clone(variables), bindings: maps.Clone(state.bindings)}
		state.step = &step
		start, depth := len(tokens), groupingIdx
		if consumeKeyword("yield") {
			if depth != state.baseDepth {
				parserError("E_BLOCK_EARLY_YIELD: Legacy yield must be the final value of this scoped body")
			}
			skipInlineWhitespace()
			collectValue(&block.result.valueType, &block.result.value, '\n')
			step.tokens = append([]token(nil), tokens[start:]...)
			if len(step.tokens) != 0 {
				block.steps = append(block.steps, step)
			}
			block.resultStep = step
			explicitResult = true
			for char != -1 && char != '}' {
				if char == ' ' || char == '\t' || char == '\n' {
					advance()
				} else if commentAhead() {
					collectComment()
				} else {
					parserError("E_BLOCK_EARLY_YIELD: no statement may follow the scoped result")
				}
			}
			break
		}
		beforeDeclared := maps.Clone(state.declared)
		parse()
		step.tokens = append([]token(nil), tokens[start:]...)
		if groupingIdx > depth {
			state.frames[groupingIdx] = legacyScopedFrame{
				vars: maps.Clone(step.vars), bindings: maps.Clone(step.bindings), declared: beforeDeclared,
			}
			variables = maps.Clone(variables)
			state.bindings, state.declared = maps.Clone(state.bindings), map[string]bool{}
			// The existing loop parser creates explicit mutable bindings for its
			// Repeat Item/Index. Give those bindings lexical native names.
			for index := range step.tokens {
				item := &step.tokens[index]
				if item.typeof != Variable {
					continue
				}
				if value, exists := variables[item.ident]; exists && value.repeatItem {
					sourceName, target := item.ident, block.bindingName()
					item.ident = target
					variables[target] = value
					step.vars[target] = value
					state.bindings[sourceName], state.declared[sourceName] = target, true
				}
			}
		} else if groupingIdx < depth {
			frame, exists := state.frames[depth]
			if !exists {
				parserError("E_BLOCK_CONTROL_SCOPE: scoped control environment is missing")
			}
			variables, state.bindings, state.declared = maps.Clone(frame.vars), maps.Clone(frame.bindings), maps.Clone(frame.declared)
			delete(state.frames, depth)
		} else if len(step.tokens) != 0 {
			last := step.tokens[len(step.tokens)-1]
			if last.typeof == Conditional && last.valueType == Else {
				frame := state.frames[depth]
				variables, state.bindings, state.declared = maps.Clone(frame.vars), maps.Clone(frame.bindings), map[string]bool{}
			}
		}
		if len(step.tokens) != 0 {
			block.steps = append(block.steps, step)
		}
	}
	if char != '}' || groupingIdx != state.baseDepth {
		parserError("E_BLOCK_UNTERMINATED: scoped body or nested control region is not closed")
	}
	hasResult := explicitResult
	if !explicitResult {
		stepIndex, tokenIndex := -1, -1
		for i := len(block.steps) - 1; i >= 0 && stepIndex < 0; i-- {
			for j := len(block.steps[i].tokens) - 1; j >= 0; j-- {
				if block.steps[i].tokens[j].typeof != Comment {
					stepIndex, tokenIndex = i, j
					break
				}
			}
		}
		if stepIndex >= 0 {
			lastStep := &block.steps[stepIndex]
			lastToken := &lastStep.tokens[tokenIndex]
			if lastToken.valueType == EndClosure && legacyScopedControlHasValue(block, lastToken.ident) {
				parserError("E_BLOCK_CONTROL_RESULT: an implicit control value requires an explicit scoped result; it cannot be discarded as Void")
			}
			if lastToken.typeof == Action && lastToken.valueType == Action &&
				legacyScopedValueType(actionArgument{valueType: Action, value: lastToken.value}, lastStep.vars, nil) != Nil {
				// Keep a value-producing call at its original position. A
				// statement-only body has no synthetic result binding.
				name := block.prefix + "_result"
				definition := varValue{variableType: "Variable", valueType: Action, value: lastToken.value, constant: true}
				lastToken.typeof, lastToken.ident = Variable, name
				lastStep.vars[name] = definition
				block.result = actionArgument{valueType: Variable, value: varValue{variableType: "Variable", valueType: Action, value: name, constant: true}}
				block.resultStep = *lastStep
				hasResult = true
			}
		}
	}
	if hasResult {
		block.resultType = legacyScopedValueType(block.result, block.resultStep.vars, nil)
		if block.resultType == Nil || block.result.value == nil {
			parserError("E_BLOCK_VALUE_REQUIRED: an explicit scoped result cannot be a void action or nil")
		}
	} else {
		block.resultType = Nil
	}
	definition.outputType = block.resultType
	advance()
	return action{ident: name, def: definition, args: arguments, block: block}
}

// Inspect the existing parsed control tokens and their captured type environments.
// A possible implicit value on any branch is not a Void proof. This is a type
// check over the original tokens, not another parser or a native-output guess.
func legacyScopedControlHasValue(block *legacyScopedBlock, target string) bool {
	type frame struct {
		group                  string
		tail, previousBranches bool
	}
	var stack []frame
	for _, step := range block.steps {
		for _, item := range step.tokens {
			if item.typeof == Comment {
				continue
			}
			control := item.typeof == Conditional || item.typeof == Repeat ||
				item.typeof == RepeatWithEach || item.typeof == Menu
			if control && item.valueType == EndClosure {
				if len(stack) == 0 || stack[len(stack)-1].group != item.ident {
					return true
				}
				last := stack[len(stack)-1]
				value := last.tail || last.previousBranches
				stack = stack[:len(stack)-1]
				if item.ident == target {
					return value
				}
				if len(stack) != 0 {
					stack[len(stack)-1].tail = value
				}
				continue
			}
			if item.typeof == Item || (item.typeof == Conditional && item.valueType == Else) {
				if len(stack) == 0 || stack[len(stack)-1].group != item.ident {
					return true
				}
				last := &stack[len(stack)-1]
				last.previousBranches = last.previousBranches || last.tail
				last.tail = false
				continue
			}
			if control {
				stack = append(stack, frame{group: item.ident})
				continue
			}
			if len(stack) == 0 {
				continue
			}
			last := &stack[len(stack)-1]
			switch item.typeof {
			case Variable, AddTo, SubFrom, MultiplyBy, DivideBy:
				last.tail = false
			case Action:
				last.tail = legacyScopedValueType(actionArgument{valueType: item.valueType, value: item.value}, step.vars, nil) != Nil
			default:
				last.tail = true
			}
		}
	}
	return true
}

// This conservative type witness follows existing declarations and canonical
// action metadata. Unknown native values remain Variable; no content-class or
// collection identity is invented from a dynamic reference.
func legacyScopedValueType(argument actionArgument, environment map[string]varValue, seen map[string]bool) tokenType {
	switch argument.valueType {
	case RawString:
		return String
	case Action:
		call, ok := argument.value.(action)
		if !ok {
			return Nil
		}
		if call.block != nil {
			return call.block.resultType
		}
		if contract, found := schema.DefaultRegistry().LookupAction(call.ident); found &&
			contract.OutputType != nil && contract.OutputType.Kind() == types.KindVoid {
			return Nil
		}
		if call.def != nil && call.def.outputType != "" {
			return call.def.outputType
		}
		return Variable
	case Variable:
		reference, ok := argument.value.(varValue)
		if !ok {
			return Variable
		}
		if reference.getAs != "" || reference.coerce != "" || reference.descriptor != nil {
			return Variable
		}
		name, ok := reference.value.(string)
		if !ok {
			return Variable
		}
		if global, found := globals[name]; found {
			return global.valueType
		}
		if seen == nil {
			seen = map[string]bool{}
		}
		if seen[name] {
			return Variable
		}
		seen[name] = true
		if value, found := environment[name]; found {
			if value.repeatItem {
				return Variable
			}
			return legacyScopedValueType(actionArgument{valueType: value.valueType, value: value.value}, environment, seen)
		}
		return Variable
	default:
		return argument.valueType
	}
}

func withLegacyScopedEnvironment(step legacyScopedStep, emit func()) {
	previousVars, previousUUIDs := variables, uuids
	previousOutputNames, previousVariableNames := importedSymbolOutputNames, importedSymbolVariableNames
	previousIsVariable, previousValueTypes := importedSymbolIsVariable, importedSymbolValueTypes
	previousDictAggr, previousAppendVars := importedSymbolDictAggr, importedSymbolAppendVars
	importedSymbolIsVariable, importedSymbolValueTypes = maps.Clone(previousIsVariable), maps.Clone(previousValueTypes)
	importedSymbolDictAggr, importedSymbolAppendVars = maps.Clone(previousDictAggr), maps.Clone(previousAppendVars)
	variables, uuids = maps.Clone(step.vars), maps.Clone(previousUUIDs)
	if variables == nil {
		variables = map[string]varValue{}
	}
	if uuids == nil {
		uuids = map[string]string{}
	}
	importedSymbolOutputNames = maps.Clone(previousOutputNames)
	if importedSymbolOutputNames == nil {
		importedSymbolOutputNames = map[string]importedOutputNameSpec{}
	}
	importedSymbolVariableNames = maps.Clone(previousVariableNames)
	if importedSymbolVariableNames == nil {
		importedSymbolVariableNames = map[string]string{}
	}
	for sourceName, target := range step.bindings {
		// Imported hints describe the captured source binding, never its new local shadow.
		delete(importedSymbolIsVariable, sourceName)
		delete(importedSymbolValueTypes, sourceName)
		delete(importedSymbolDictAggr, sourceName)
		delete(importedSymbolAppendVars, sourceName)
		importedSymbolOutputNames[sourceName] = importedOutputNameSpec{name: target, hasName: true}
		importedSymbolVariableNames[sourceName] = target
		if id, exists := previousUUIDs[target]; exists {
			uuids[sourceName] = id
		} else {
			delete(uuids, sourceName)
		}
	}
	defer func() {
		// Keep new actual producer UUIDs, but remove the temporary source-name
		// aliases. Captured outer bindings must regain their original identities.
		for sourceName := range step.bindings {
			if id, exists := previousUUIDs[sourceName]; exists {
				uuids[sourceName] = id
			} else {
				delete(uuids, sourceName)
			}
		}
		variables = previousVars
		importedSymbolOutputNames, importedSymbolVariableNames = previousOutputNames, previousVariableNames
		importedSymbolIsVariable, importedSymbolValueTypes = previousIsVariable, previousValueTypes
		importedSymbolDictAggr, importedSymbolAppendVars = previousDictAggr, previousAppendVars
	}()
	emit()
}

func emitLegacyScopedBlock(block *legacyScopedBlock, reference *WFActionReference) {
	previousTokens, previousAction := tokens, currentAction
	defer func() { tokens, currentAction = previousTokens, previousAction }()
	emitTimer := func(name, label string) error {
		setCurrentAction(name, actions[name])
		makeAction([]actionArgument{{valueType: RawString, value: label}}, &WFActionReference{})
		return nil
	}
	emitBody := func() {
		for _, step := range block.steps {
			withLegacyScopedEnvironment(step, func() {
				tokens = step.tokens
				generateActions()
			})
		}
	}
	if block.resultType == Nil {
		if reference != nil && (reference.UUID != "" || reference.CustomOutputName != "") {
			exit("E_BLOCK_VOID_VALUE: a Void scoped body has no value to bind or consume")
		}
		err := backend.EmitScopedMeasureVoid(
			block.label+":"+block.prefix,
			emitTimer,
			func() error { emitBody(); return nil },
			func() error {
				addStdAction("nothing", map[string]any{})
				return nil
			},
		)
		if err != nil {
			exit(err.Error())
		}
		return
	}
	_, err := backend.EmitScopedMeasure(
		block.label+":"+block.prefix,
		emitTimer,
		func() (WFTextTokenAttachment, error) {
			emitBody()
			var result WFTextTokenAttachment
			var resultErr error
			withLegacyScopedEnvironment(block.resultStep, func() {
				if block.result.valueType == Variable {
					value := variableValue(block.result.value.(varValue))
					var ok bool
					result, ok = value.(WFTextTokenAttachment)
					if !ok {
						resultErr = fmt.Errorf("E_BLOCK_REFERENCE: scoped result has no canonical native attachment")
					}
					return
				}
				name := block.prefix + "_result"
				id := createUUIDReference(name)
				if block.result.valueType == Arr {
					variables[name] = varValue{variableType: "Variable", valueType: Arr}
					makeVariableAction(&token{typeof: Variable, ident: name, valueType: Arr, value: block.result.value})
					result = inputValue(name, "")
					return
				}
				value := block.result.value
				makeVariableValue(&WFActionReference{UUID: id, CustomOutputName: name}, block.result.valueType, &value)
				result = inputValue(name, id)
			})
			return result, resultErr
		},
		func(value WFTextTokenAttachment) bool { return value.Value.Type == "ActionOutput" },
		func(value WFTextTokenAttachment, final bool) (WFTextTokenAttachment, error) {
			name := block.prefix + "_snapshot"
			if final {
				name = block.prefix + "_forward"
			}
			id := createUUIDReference(name)
			if final && reference != nil {
				if reference.UUID != "" {
					id = reference.UUID
				}
				if reference.CustomOutputName != "" {
					name = reference.CustomOutputName
				}
			}
			addStdAction("getvariable", attachReferenceToParams(
				map[string]any{"WFVariable": value},
				&WFActionReference{UUID: id, CustomOutputName: name},
			))
			return EncodeReferenceAttachment(ReferenceDescriptor{Kind: "ActionOutput", OutputUUID: id, OutputName: name},
				"WFTextTokenAttachment").(WFTextTokenAttachment), nil
		},
	)
	if err != nil {
		exit(err.Error())
	}
}
