/*
 * Copyright (c) Cherri Language v2.0
 * Canonical Registry Adapter
 */

package main

import (
	"sync"

	"github.com/electrikmilk/cherri/internal/language/backend"
)

var registryInitOnce sync.Once

func ensureStandardActionRegistry() {
	registryInitOnce.Do(func() {
		loadActionsByCategory()
	})
}

// lookupCanonicalAction maps a backend.ResolvedCall to the authoritative actionDefinition in actions.
func lookupCanonicalAction(call backend.ResolvedCall) (*actionDefinition, string, bool) {
	ensureStandardActionRegistry()

	// 1. Direct match by DefinitionID
	if call.DefinitionID != "" {
		if def, ok := actions[call.DefinitionID]; ok && def != nil {
			return def, call.DefinitionID, true
		}
	}

	// 2. Match by VariantID
	if call.VariantID != "" {
		if def, ok := actions[call.VariantID]; ok && def != nil {
			return def, call.VariantID, true
		}
	}

	// 3. Fallback search by AppleIdentifier
	if call.AppleIdentifier != "" {
		for ident, def := range actions {
			if def != nil {
				savedIdent := currentAction.identifier
				currentAction.identifier = ident
				currentAction.definition = *def
				fullID := getFullActionIdentifier()
				currentAction.identifier = savedIdent
				if fullID == call.AppleIdentifier {
					return def, ident, true
				}
			}
		}
	}

	return nil, "", false
}
