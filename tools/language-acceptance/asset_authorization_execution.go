package main

import (
	"reflect"
	"strings"
)

var assetAuthorizationSecuritySubtests = []string{
	"TestP15RP29AssetAuthorizationAcrossServiceAndCompile/allowed_text",
	"TestP15RP29AssetAuthorizationAcrossServiceAndCompile/allowed_binary",
	"TestP15RP29AssetAuthorizationAcrossServiceAndCompile/missing",
	"TestP15RP29AssetAuthorizationAcrossServiceAndCompile/existing_parent_escape",
	"TestP15RP29AssetAuthorizationAcrossServiceAndCompile/file_symlink_escape",
	"TestP15RP29AssetAuthorizationAcrossServiceAndCompile/directory_symlink_escape",
}

var assetAuthorizationScenario = supplementalProductionScenario{
	PackagePath: ".", ModulePath: "github.com/electrikmilk/cherri",
	TestNames:       editorProductionTests["RP29"],
	Tiers:           []string{"security", "service", "benchmark"},
	ProductionPaths: editorProductionPathsForID("RP29"),
}

func assetAuthorizationRequiredSubtests(required []string) []string {
	for _, name := range required {
		if name == "TestP15RP29AssetAuthorizationAcrossServiceAndCompile" {
			return assetAuthorizationSecuritySubtests
		}
	}
	return nil
}

func editorProductionPathsForID(id string) []string {
	var paths = append([]string{}, editorProductionPaths...)
	if id == "RP29" {
		paths = append(paths, "p15_asset_authorization_repair_test.go")
	}
	return paths
}

func editorProductionTestForTier(id, tier string) string {
	if _, found := editorProductionTests[id]; !found {
		return ""
	}
	if tier == "benchmark" {
		return editorBenchmarkTest
	}
	if tier == "service" && (id == "S03" || id == "RP29") {
		return "TestP15S03SharedAssetCacheAcrossServiceEditsAndCanonicalCompile"
	}
	if tier == "security" && id == "RP29" {
		return "TestP15RP29AssetAuthorizationAcrossServiceAndCompile"
	}
	return ""
}

func editorProductionScope(id string) string {
	if id == "RP29" {
		return "actual workspace authorization, cache reauthorization and measured local Cherri service/cache; no network installation or native UI latency claim"
	}
	return "local Cherri service and canonical embedded asset cache; no native UI latency claim"
}

func (r *Runner) runAssetAuthorizationRequirement(id, title string, levels []string) bool {
	if id != "RP29" || title != "Asset and path authorization" ||
		!reflect.DeepEqual(parseRequiredTiers(strings.Join(levels, "+")), assetAuthorizationScenario.Tiers) {
		return false
	}
	r.runEditorPerformanceRequirement(id)
	return true
}
