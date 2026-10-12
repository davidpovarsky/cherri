/*
 * Copyright (c) Cherri Language v2.0
 * Canonical Registry Adapter
 */

package main

import (
	"github.com/electrikmilk/cherri/internal/language/backend"
)

// lookupCanonicalAction maps a backend.ResolvedCall to the authoritative actionDefinition in actions.
func lookupCanonicalAction(call backend.ResolvedCall) (*actionDefinition, string, bool) {
	stdActions := EnsureCanonicalStandardActions()

	// 1. Direct match by DefinitionID
	if call.DefinitionID != "" {
		if def, ok := stdActions[call.DefinitionID]; ok && def != nil {
			return def, call.DefinitionID, true
		}
		if def, ok := actions[call.DefinitionID]; ok && def != nil {
			return def, call.DefinitionID, true
		}
	}

	// 2. Match by VariantID
	if call.VariantID != "" {
		if def, ok := stdActions[call.VariantID]; ok && def != nil {
			return def, call.VariantID, true
		}
		if def, ok := actions[call.VariantID]; ok && def != nil {
			return def, call.VariantID, true
		}
	}

	// 3. Fallback search by AppleIdentifier is deliberately disallowed for named calls.
	// Named action invocations must resolve by stable canonical definition or variant identity.
	return nil, "", false
}
