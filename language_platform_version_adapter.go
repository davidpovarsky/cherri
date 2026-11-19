package main

import (
	"fmt"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/backend"
	"github.com/electrikmilk/cherri/internal/language/schema"
)

// This is the existing analyzed registry and its exact source target context,
// not a second catalog or an Apple parameter. The session only reads it.
type canonicalPlatformVersionContext struct {
	registry *schema.Registry
	targets  []string
	minimum  string
	maximum  string
}

func (s *CanonicalBackendSession) configurePlatformVersions(registry *schema.Registry, targets []string, minimum, maximum string) {
	s.platformVersions = &canonicalPlatformVersionContext{
		registry: registry,
		targets:  append([]string(nil), targets...),
		minimum:  minimum,
		maximum:  maximum,
	}
}

// The current action is already a copied definition under BackendTransaction.
// Keep all canonical argument/native checks; only the superseded generic version
// fields on that copy are cleared after the shared platform decision succeeds.
func (s *CanonicalBackendSession) applyPlatformVersionPolicy(call backend.ResolvedCall, canonicalName string) error {
	context := s.platformVersions
	if context == nil || context.registry == nil {
		return nil
	}
	action, found := context.registry.LookupAction(canonicalName)
	if !found || len(action.PlatformVersions) == 0 {
		return nil
	}
	if action.CallableName != canonicalName ||
		action.AppleIdentifier == "" || action.AppleIdentifier != call.AppleIdentifier ||
		action.AppleIdentifier != getFullActionIdentifier() ||
		action.Builtin != currentAction.definition.builtin || action.Custom == currentAction.definition.builtin {
		return fmt.Errorf("[%s] platform version descriptor does not match canonical action %q", analysis.CodeSchemaMismatch, canonicalName)
	}
	for _, issue := range schema.CheckPlatformVersions(action.PlatformVersions, context.targets, context.minimum, context.maximum) {
		var code string
		switch issue.Kind {
		case schema.PlatformVersionUnverified:
			// The analyzer/service retains the explicit unknown-availability
			// warning. Canonical emission does not manufacture another fact.
			continue
		case schema.PlatformVersionInvalid:
			code = analysis.CodePlatformVersionMetadata
		case schema.PlatformVersionBadTarget:
			code = analysis.CodeTargetValueShape
		default:
			code = analysis.CodeUnsupportedTarget
		}
		return fmt.Errorf("[%s] action %q: %s", code, canonicalName, issue.Message)
	}
	currentAction.definition.minVersion = 0
	currentAction.definition.maxVersion = 0
	currentAction.definition.minVersionString = ""
	currentAction.definition.maxVersionString = ""
	return nil
}
