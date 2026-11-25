package main

import (
	"fmt"
	"strings"

	"github.com/electrikmilk/cherri/internal/language/backend"
)

// Inspect without advancing the ordinary call cursor: default pagination must
// keep the same source-order/UUID behavior when no callback is authored.
func legacyPaginateBodyAhead() bool {
	for offset := 0; ; offset++ {
		value := next(offset)
		if value == ' ' || value == '\t' || value == '\r' {
			continue
		}
		return value == '{'
	}
}

func emitLegacyPaginateBlock(block *legacyScopedBlock, reference *WFActionReference) {
	previousTokens, previousAction := tokens, currentAction
	defer func() { tokens, currentAction = previousTokens, previousAction }()
	if block.resultType != Nil || block.pageTarget == "" {
		exit("E_PAGINATE_BODY_VOID: a page callback requires an immutable page and Void body")
	}
	setCurrentAction("paginate", actions["paginate"])
	currentAction.arguments = block.paginateArguments
	params := getActionParameters(block.paginateArguments)
	var resultUUID, resultName string
	if reference != nil {
		resultUUID, resultName = reference.UUID, reference.CustomOutputName
	}
	callback := backend.ScopedPageCallback{
		InputName: block.pageTarget,
		EmitAction: func(identifier string, parameters map[string]any) error {
			if !strings.HasPrefix(identifier, "is.workflow.actions.") {
				return fmt.Errorf("E_PAGINATE_NATIVE_ACTION: invalid native helper identifier")
			}
			addStdAction(strings.TrimPrefix(identifier, "is.workflow.actions."), parameters)
			return nil
		},
		Body: func(page backend.Reference) error {
			if page.Kind != backend.RefActionResult || page.ProducerID == "" || page.ProducerName != block.pageTarget {
				return fmt.Errorf("E_PAGINATE_PAGE_REFERENCE: original validated native page is required")
			}
			if uuids == nil {
				uuids = map[string]string{}
			}
			previous, existed := uuids[block.pageTarget]
			uuids[block.pageTarget] = page.ProducerID
			defer func() {
				if existed {
					uuids[block.pageTarget] = previous
				} else {
					delete(uuids, block.pageTarget)
				}
			}()
			for _, step := range block.steps {
				withLegacyScopedEnvironment(step, func() { tokens = step.tokens; generateActions() })
			}
			return nil
		},
	}
	if _, err := buildScopedPaginateMacroActions(params, resultUUID, resultName, &callback); err != nil {
		exit(err.Error())
	}
}
