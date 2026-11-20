package main

import "github.com/electrikmilk/cherri/internal/language/backend"

// Argument expressions have already executed when this policy is queried.
// Removed Release diagnostics still validate options, but do not consume a
// runtime container and therefore do not need compiler-owned Text carriers.
func (s *CanonicalBackendSession) ResolvedCallConsumesRuntimeArguments(call backend.ResolvedCall) bool {
	_, identifier, found := lookupCanonicalActionLocked(call)
	return !found || s.resolvedActionConsumesRuntimeArguments(identifier)
}

func (s *CanonicalBackendSession) resolvedActionConsumesRuntimeArguments(identifier string) bool {
	return s.buildProfile != BuildProfileRelease || !isRemovableDiagnosticMacro(identifier)
}
