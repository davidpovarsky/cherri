/*
 * Copyright (c) Cherri Language v2.0
 * Legacy Parity Differential Harness
 */

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/electrikmilk/cherri/internal/language/migrate"
	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/shortcutcompare"
)

type FixtureResult struct {
	Name        string                       `json:"name"`
	Path        string                       `json:"path"`
	Status      string                       `json:"status"` // VERIFIED_PARITY, UPSTREAM_BASELINE_FAIL, MIGRATION_FAIL, V2_COMPILE_FAIL, PARITY_DIFFERENCE
	LegacyOk    bool                         `json:"legacy_ok"`
	MigratedOk  bool                         `json:"migrated_ok"`
	V2CompileOk bool                         `json:"v2_compile_ok"`
	Equal       bool                         `json:"equal"`
	Differences []shortcutcompare.Difference `json:"differences,omitempty"`
	Detail      string                       `json:"detail,omitempty"`
}

type ActionParityResult struct {
	CallableName string                       `json:"callable_name"`
	Category     string                       `json:"category"`
	AppleID      string                       `json:"apple_id"`
	LegacyPassed bool                         `json:"legacy_passed"`
	V2Passed     bool                         `json:"v2_passed"`
	Equal        bool                         `json:"equal"`
	Status       string                       `json:"status"` // VERIFIED, V2_ONLY, LEGACY_FAIL, PARITY_DIFFERENCE
	Differences  []shortcutcompare.Difference `json:"differences,omitempty"`
	Detail       string                       `json:"detail,omitempty"`
}

type ParityReport struct {
	Timestamp          string                `json:"timestamp"`
	TotalFixtures      int                   `json:"total_fixtures"`
	VerifiedParity     int                   `json:"verified_parity"`
	ProvenEquivalent   int                   `json:"proven_equivalent"`
	UpstreamFails      int                   `json:"upstream_baseline_fails"`
	MigrationFails     int                   `json:"migration_fails"`
	V2CompileFails     int                   `json:"v2_compile_fails"`
	ParityDifferences  int                   `json:"parity_differences"`
	TotalActionsTested int                   `json:"total_actions_tested"`
	ActionsPassedBoth  int                   `json:"actions_passed_both"`
	Fixtures           []FixtureResult       `json:"fixtures"`
	Actions            []ActionParityResult  `json:"actions"`
}

func main() {
	fmt.Println("=== Cherri Legacy <-> v2 Differential Parity Runner ===")

	// Ensure compiler binary is available
	binPath, err := ensureCompilerBinary()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error locating/building compiler binary: %v\n", err)
		os.Exit(1)
	}

	repoRoot := findRepoRoot()
	fixturesDir := filepath.Join(repoRoot, "tests")
	manifestPath := filepath.Join(fixturesDir, "fixtures-manifest.json")

	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAILURE: Unable to read canonical fixtures manifest at %s: %v\n", manifestPath, err)
		os.Exit(1)
	}

	var manifestList []string
	if err := json.Unmarshal(manifestData, &manifestList); err != nil {
		fmt.Fprintf(os.Stderr, "FAILURE: Malformed fixtures-manifest.json: %v\n", err)
		os.Exit(1)
	}

	manifestSet := make(map[string]bool, len(manifestList))
	var matches []string
	for _, name := range manifestList {
		manifestSet[name] = true
		fixturePath := filepath.Join(fixturesDir, name)
		if _, err := os.Stat(fixturePath); os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "FAILURE: Required canonical fixture %q defined in manifest is missing at %s\n", name, fixturePath)
			os.Exit(1)
		}
		matches = append(matches, fixturePath)
	}
	sort.Strings(matches)

	// Guard against untracked or unexpected fixtures in tests/ directory
	globMatches, globErr := filepath.Glob(filepath.Join(fixturesDir, "*.cherri"))
	if globErr == nil {
		for _, g := range globMatches {
			base := filepath.Base(g)
			if !manifestSet[base] {
				fmt.Fprintf(os.Stderr, "FAILURE: Untracked or unexpected fixture %q found in tests/ (not in manifest)!\n", base)
				os.Exit(1)
			}
		}
	}

	tmpDir, err := os.MkdirTemp("", "cherri-legacy-parity-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating temp dir: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tmpDir)

	for _, f := range matches {
		if c, err := os.ReadFile(f); err == nil {
			_ = os.WriteFile(filepath.Join(tmpDir, filepath.Base(f)), c, 0644)
		}
	}

	report := ParityReport{
		Timestamp:     time.Now().UTC().Format(time.RFC3339),
		TotalFixtures: len(matches),
	}

	fmt.Printf("Discovered %d original test fixtures in tests/*.cherri\n\n", len(matches))

	for _, fixturePath := range matches {
		name := filepath.Base(fixturePath)
		res := testFixture(binPath, fixturePath, tmpDir, repoRoot)
		report.Fixtures = append(report.Fixtures, res)

		switch res.Status {
		case "VERIFIED_PARITY":
			report.VerifiedParity++
			fmt.Printf("  [PASS]  %-35s => Parity Verified (Bit-for-bit Plist Match)\n", name)
		case "PROVEN_EQUIVALENT":
			report.ProvenEquivalent++
			fmt.Printf("  [EQUIV] %-35s => Proven Semantic Equivalence (SSA / Norm Verified)\n", name)
		case "UPSTREAM_BASELINE_FAIL":
			report.UpstreamFails++
			fmt.Printf("  [SKIP]  %-35s => Upstream Baseline Fail: %s\n", name, res.Detail)
		case "MIGRATION_FAIL":
			report.MigrationFails++
			fmt.Printf("  [FAIL]  %-35s => Migration Failed: %s\n", name, res.Detail)
		case "V2_COMPILE_FAIL":
			report.V2CompileFails++
			fmt.Printf("  [FAIL]  %-35s => V2 Compile Failed: %s\n", name, res.Detail)
		case "PARITY_DIFFERENCE":
			report.ParityDifferences++
			fmt.Printf("  [DIFF]  %-35s => Structural Differences (%d)\n", name, len(res.Differences))
		}
	}

	fmt.Println("\n--- Testing Newly Added Apple Actions in Both Modes ---")
	actionResults := testAddedAppleActions(binPath, tmpDir, repoRoot)
	report.Actions = actionResults
	report.TotalActionsTested = len(actionResults)
	for _, ar := range actionResults {
		if ar.Equal && ar.Status == "VERIFIED" {
			report.ActionsPassedBoth++
			fmt.Printf("  [PASS] action %-25s => Legacy: OK, v2: OK, Plist: PARITY_VERIFIED\n", ar.CallableName)
		} else {
			fmt.Printf("  [FAIL] action %-25s => Legacy: %t, v2: %t, Equal: %t (%s)\n", ar.CallableName, ar.LegacyPassed, ar.V2Passed, ar.Equal, ar.Detail)
		}
	}

	// Write machine-readable report
	outJSONPath := filepath.Join(repoRoot, "docs", "language-v2", "legacy-parity-results.json")
	reportBytes, _ := json.MarshalIndent(report, "", "  ")
	_ = os.WriteFile(outJSONPath, reportBytes, 0644)
	fmt.Printf("\nSaved machine-readable parity report to: %s\n", outJSONPath)

	fmt.Printf("\n=== SUMMARY ===\n")
	fmt.Printf("Total Fixtures:        %d\n", report.TotalFixtures)
	fmt.Printf("Verified Parity:       %d\n", report.VerifiedParity)
	fmt.Printf("Proven Equivalent:     %d\n", report.ProvenEquivalent)
	fmt.Printf("Upstream Baseline Fail:%d\n", report.UpstreamFails)
	fmt.Printf("Migration Fails:       %d\n", report.MigrationFails)
	fmt.Printf("V2 Compile Fails:      %d\n", report.V2CompileFails)
	fmt.Printf("Parity Differences:    %d\n", report.ParityDifferences)
	fmt.Printf("New Actions Tested:    %d\n", report.TotalActionsTested)
	fmt.Printf("Actions Passed Both:   %d\n", report.ActionsPassedBoth)

	allOk := true
	if (report.VerifiedParity+report.ProvenEquivalent) < report.TotalFixtures || report.ParityDifferences > 0 || report.V2CompileFails > 0 || report.MigrationFails > 0 || report.UpstreamFails > 0 {
		allOk = false
	}
	if report.ActionsPassedBoth < report.TotalActionsTested {
		allOk = false
	}
	for _, ar := range report.Actions {
		if !ar.Equal || ar.Status != "VERIFIED" {
			allOk = false
			break
		}
	}

	if !allOk {
		fmt.Fprintf(os.Stderr, "\nFAILURE: Differential parity gate check failed.\n")
		os.Exit(1)
	}

	fmt.Printf("\nSUCCESS: All %d test fixtures and %d Apple actions passed full differential parity!\n", report.TotalFixtures, report.TotalActionsTested)
}

func testFixture(binPath, fixturePath, tmpDir, repoRoot string) FixtureResult {
	name := filepath.Base(fixturePath)
	res := FixtureResult{
		Name: name,
		Path: fixturePath,
	}

	base := strings.TrimSuffix(name, ".cherri")
	legacyOut := filepath.Join(tmpDir, base+"_legacy.shortcut")
	v2Src := filepath.Join(tmpDir, base+"_v2.cherri")
	v2Out := filepath.Join(tmpDir, base+"_v2.shortcut")

	// 1. Compile original with --legacy --skip-sign
	legacyArgs := []string{fixturePath, "--legacy", "--skip-sign", "-o", legacyOut}
	if name != "decomp-expected.cherri" && name != "decomp-me.cherri" {
		legacyArgs = append(legacyArgs, "--load-all-actions")
	}
	cmdLegacy := exec.Command(binPath, legacyArgs...)
	cmdLegacy.Dir = repoRoot
	outLegacy, errLegacy := cmdLegacy.CombinedOutput()
	if errLegacy != nil {
		res.LegacyOk = false
		res.Status = "UPSTREAM_BASELINE_FAIL"
		res.Detail = strings.TrimSpace(string(outLegacy))
		return res
	}
	res.LegacyOk = true

	// 2. Migrate original source
	content, err := os.ReadFile(fixturePath)
	if err != nil {
		res.Status = "MIGRATION_FAIL"
		res.Detail = err.Error()
		return res
	}

	migrated, diags, errMig := migrate.MigrateSourceWithDiagnostics(string(content))
	if errMig != nil || len(diags) > 0 {
		res.MigratedOk = false
		res.Status = "MIGRATION_FAIL"
		if len(diags) > 0 {
			res.Detail = fmt.Sprintf("diagnostics: %v", diags)
		} else {
			res.Detail = errMig.Error()
		}
		return res
	}
	res.MigratedOk = true
	_ = os.WriteFile(v2Src, []byte(migrated), 0644)

	// 3. Compile migrated source with v2 compile --skip-sign
	cmdV2 := exec.Command(binPath, "compile", v2Src, "--skip-sign", "-o", v2Out)
	cmdV2.Dir = repoRoot
	outV2, errV2 := cmdV2.CombinedOutput()
	if errV2 != nil {
		res.V2CompileOk = false
		res.Status = "V2_COMPILE_FAIL"
		res.Detail = strings.TrimSpace(string(outV2))
		return res
	}
	res.V2CompileOk = true

	// 4. Compare plists
	cmpResult, errCmp := shortcutcompare.CompareFiles(legacyOut, v2Out)
	if errCmp != nil {
		res.Status = "PARITY_DIFFERENCE"
		res.Detail = fmt.Sprintf("comparator error: %v", errCmp)
		return res
	}

	if cmpResult.Equal {
		res.Equal = true
		res.Status = "VERIFIED_PARITY"
	} else {
		// Test under narrow semantic parity normalization rules (SSA forwarding, quirk cleanup)
		normResult, errNorm := shortcutcompare.CompareFilesNormalized(legacyOut, v2Out)
		if errNorm == nil && normResult.Equal {
			res.Equal = true
			res.Status = "PROVEN_EQUIVALENT"
		} else {
			res.Equal = false
			res.Status = "PARITY_DIFFERENCE"
			if errNorm == nil {
				res.Differences = normResult.Differences
			} else {
				res.Differences = cmpResult.Differences
			}
		}
	}
	return res
}

func testAddedAppleActions(binPath, tmpDir, repoRoot string) []ActionParityResult {
	reg := schema.DefaultRegistry()

	// Representative added Apple actions to test across both Legacy and v2
	type ActionTestSpec struct {
		Name         string
		Category     string
		LegacyCode   string
		V2Code       string
	}

	specs := []ActionTestSpec{
		{
			Name:       "getParkedCarLocation",
			Category:   "location",
			LegacyCode: "#include 'actions/location'\ngetParkedCarLocation()\n",
			V2Code:     "getParkedCarLocation()\n",
		},
		{
			Name:       "scanBarcode",
			Category:   "images",
			LegacyCode: "#include 'actions/images'\nscanBarcode()\n",
			V2Code:     "scanBarcode()\n",
		},
		{
			Name:       "filterPhotos",
			Category:   "photos",
			LegacyCode: "#include 'actions/photos'\nfilterPhotos(nil, nil, \"Creation Date\", \"Latest First\", 5)\n",
			V2Code:     "filterPhotos(nil, nil, \"Creation Date\", \"Latest First\", 5)\n",
		},
		{
			Name:       "searchiTunes",
			Category:   "music",
			LegacyCode: "#include 'actions/music'\nsearchiTunes(\"The Beatles\", 10, \"US\", nil, nil, \"Music\")\n",
			V2Code:     "searchiTunes(\"The Beatles\", 10, \"US\", nil, nil, \"Music\")\n",
		},
		{
			Name:       "intercom",
			Category:   "device",
			LegacyCode: "#include 'actions/device'\nintercom(\"Dinner is ready\")\n",
			V2Code:     "intercom(\"Dinner is ready\")\n",
		},
		{
			Name:       "getCurrentApp",
			Category:   "mac",
			LegacyCode: "#include 'actions/mac'\ngetCurrentApp()\n",
			V2Code:     "getCurrentApp()\n",
		},
		{
			Name:       "getSelectedText",
			Category:   "mac",
			LegacyCode: "#include 'actions/mac'\ngetSelectedText()\n",
			V2Code:     "getSelectedText()\n",
		},
		{
			Name:       "getUpcomingEvents",
			Category:   "calendar",
			LegacyCode: "#include 'actions/calendar'\ngetUpcomingEvents(5, \"Any Day\")\n",
			V2Code:     "getUpcomingEvents(5, \"Any Day\")\n",
		},
		{
			Name:       "addCalendar",
			Category:   "calendar",
			LegacyCode: "#include 'actions/calendar'\naddCalendar(\"Work Calendar\")\n",
			V2Code:     "addCalendar(\"Work Calendar\")\n",
		},
		{
			Name:       "getType",
			Category:   "basic",
			LegacyCode: "getType(Ask)\n",
			V2Code:     "getType(Ask)\n",
		},
		{
			Name:       "getTravelTime",
			Category:   "location",
			LegacyCode: "#include 'actions/location'\ngetTravelTime(nil, nil, \"Driving\", true, false)\n",
			V2Code:     "getTravelTime(nil, nil, \"Driving\", true, false)\n",
		},
		{
			Name:       "storeValue",
			Category:   "storage",
			LegacyCode: "#include 'actions/storage'\nstoreValue(\"myKey\", \"myValue\")\n",
			V2Code:     "storeValue(\"myKey\", \"myValue\")\n",
		},
		{
			Name:       "getStoredValue",
			Category:   "storage",
			LegacyCode: "#include 'actions/storage'\ngetStoredValue(\"myKey\")\n",
			V2Code:     "getStoredValue(\"myKey\")\n",
		},
		{
			Name:       "deleteStoredValue",
			Category:   "storage",
			LegacyCode: "#include 'actions/storage'\ndeleteStoredValue(\"myKey\")\n",
			V2Code:     "deleteStoredValue(\"myKey\")\n",
		},
		{
			Name:       "askModel",
			Category:   "intelligence",
			LegacyCode: "#include 'actions/intelligence'\naskModel(\"Summarize this text\", true, true)\n",
			V2Code:     "askModel(\"Summarize this text\", true, true)\n",
		},
		{
			Name:       "askProModel",
			Category:   "intelligence",
			LegacyCode: "#include 'actions/intelligence'\naskProModel(\"Deep reasoning prompt\", true, true)\n",
			V2Code:     "askProModel(\"Deep reasoning prompt\", true, true)\n",
		},
		{
			Name:       "askDeviceModel",
			Category:   "intelligence",
			LegacyCode: "#include 'actions/intelligence'\naskDeviceModel(\"On-device offline prompt\")\n",
			V2Code:     "askDeviceModel(\"On-device offline prompt\")\n",
		},
	}

	var results []ActionParityResult
	for _, spec := range specs {
		act, _ := reg.LookupAction(spec.Name)
		appleID := ""
		if act != nil {
			appleID = act.AppleIdentifier
		}

		res := ActionParityResult{
			CallableName: spec.Name,
			Category:     spec.Category,
			AppleID:      appleID,
		}

		// Test Legacy
		legacyFile := filepath.Join(tmpDir, "act_legacy_"+spec.Name+".cherri")
		legacyOut := filepath.Join(tmpDir, "act_legacy_"+spec.Name+".shortcut")
		_ = os.WriteFile(legacyFile, []byte(spec.LegacyCode), 0644)

		cmdLeg := exec.Command(binPath, legacyFile, "--legacy", "--skip-sign", "-o", legacyOut)
		cmdLeg.Dir = repoRoot
		outLeg, errLeg := cmdLeg.CombinedOutput()
		res.LegacyPassed = (errLeg == nil)

		// Test V2
		v2File := filepath.Join(tmpDir, "act_v2_"+spec.Name+".cherri")
		v2Out := filepath.Join(tmpDir, "act_v2_"+spec.Name+".shortcut")
		_ = os.WriteFile(v2File, []byte(spec.V2Code), 0644)

		cmdV2 := exec.Command(binPath, "compile", v2File, "--skip-sign", "-o", v2Out)
		cmdV2.Dir = repoRoot
		outV2, errV2 := cmdV2.CombinedOutput()
		res.V2Passed = (errV2 == nil)

		if !res.LegacyPassed {
			res.Status = "LEGACY_FAIL"
			res.Detail = strings.TrimSpace(string(outLeg))
		} else if !res.V2Passed {
			res.Status = "V2_FAIL"
			res.Detail = strings.TrimSpace(string(outV2))
		} else {
			// Compare plists
			cmpResult, errCmp := shortcutcompare.CompareFiles(legacyOut, v2Out)
			if errCmp != nil {
				res.Status = "COMPARATOR_ERROR"
				res.Detail = errCmp.Error()
			} else if !cmpResult.Equal {
				res.Equal = false
				res.Status = "PARITY_DIFFERENCE"
				res.Differences = cmpResult.Differences
			} else {
				res.Equal = true
				res.Status = "VERIFIED"
			}
		}

		results = append(results, res)
	}

	return results
}

var (
	compilerBinMu sync.Mutex
	cachedBinPath string
)

func ensureCompilerBinary() (string, error) {
	compilerBinMu.Lock()
	defer compilerBinMu.Unlock()

	if cachedBinPath != "" {
		if fi, err := os.Stat(cachedBinPath); err == nil && fi.Size() > 0 {
			return cachedBinPath, nil
		}
	}

	if bin := os.Getenv("CHERRI_BIN"); bin != "" {
		if fi, err := os.Stat(bin); err == nil && fi.Size() > 0 {
			cachedBinPath = bin
			return bin, nil
		}
	}
	root := findRepoRoot()
	binName := "cherri"
	if strings.Contains(strings.ToLower(os.Getenv("OS")), "windows") {
		binName = "cherri.exe"
	}
	binPath := filepath.Join(root, binName)
	if fi, err := os.Stat(binPath); err == nil && fi.Size() > 0 {
		cachedBinPath = binPath
		return binPath, nil
	}

	cmd := exec.Command("go", "build", "-o", binPath, ".")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("build failed: %s: %w", string(out), err)
	}
	cachedBinPath = binPath
	return binPath, nil
}

func findRepoRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		return "."
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "."
}
