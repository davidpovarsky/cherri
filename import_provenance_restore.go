//go:build !cherri_oracle_nopatch

package main

func maybeApplyImportProvenanceToCompiledActions(prov map[string]any, compiled []any) {
	applyImportProvenanceToCompiledActions(prov, compiled)
}
