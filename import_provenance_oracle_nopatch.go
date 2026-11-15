//go:build cherri_oracle_nopatch

package main

// Oracle binary build intentionally skips post-compile metadata restoration while compiling
// the exact same Cherri source and capturing compiled pre-overlay native output.
func maybeApplyImportProvenanceToCompiledActions(_ map[string]any, _ []any) {
}
