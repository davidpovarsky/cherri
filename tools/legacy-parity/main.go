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
	CallableName string `json:"callable_name"`
	Category     string `json:"category"`
	AppleID      string `json:"apple_id"`
	LegacyPassed bool   `json:"legacy_passed"`
	V2Passed     bool   `json:"v2_passed"`
	Status       string `json:"status"` // VERIFIED, V2_ONLY, LEGACY_FAIL
	Detail       string `json:"detail,omitempty"`
}

type ParityReport struct {
	Timestamp          string                `json:"timestamp"`
	TotalFixtures      int                   `json:"total_fixtures"`
	VerifiedParity     int                   `json:"verified_parity"`
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
	matches, err := filepath.Glob(filepath.Join(fixturesDir, "*.cherri"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error globbing tests: %v\n", err)
		os.Exit(1)
	}
	sort.Strings(matches)

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
			fmt.Printf("  [PASS] %-35s => Parity Verified\n", name)
		case "UPSTREAM_BASELINE_FAIL":
			report.UpstreamFails++
			fmt.Printf("  [SKIP] %-35s => Upstream Baseline Fail: %s\n", name, res.Detail)
		case "MIGRATION_FAIL":
			report.MigrationFails++
			fmt.Printf("  [FAIL] %-35s => Migration Failed: %s\n", name, res.Detail)
		case "V2_COMPILE_FAIL":
			report.V2CompileFails++
			fmt.Printf("  [FAIL] %-35s => V2 Compile Failed: %s\n", name, res.Detail)
		case "PARITY_DIFFERENCE":
			report.ParityDifferences++
			fmt.Printf("  [DIFF] %-35s => Structural Differences (%d)\n", name, len(res.Differences))
		}
	}

	fmt.Println("\n--- Testing Newly Added Apple Actions in Both Modes ---")
	actionResults := testAddedAppleActions(binPath, tmpDir, repoRoot)
	report.Actions = actionResults
	report.TotalActionsTested = len(actionResults)
	for _, ar := range actionResults {
		if ar.LegacyPassed && ar.V2Passed {
			report.ActionsPassedBoth++
			fmt.Printf("  [PASS] action %-25s => Legacy: OK, v2: OK\n", ar.CallableName)
		} else {
			fmt.Printf("  [WARN] action %-25s => Legacy: %t, v2: %t (%s)\n", ar.CallableName, ar.LegacyPassed, ar.V2Passed, ar.Detail)
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
	fmt.Printf("Upstream Baseline Fail:%d\n", report.UpstreamFails)
	fmt.Printf("Migration Fails:       %d\n", report.MigrationFails)
	fmt.Printf("V2 Compile Fails:      %d\n", report.V2CompileFails)
	fmt.Printf("Parity Differences:    %d\n", report.ParityDifferences)
	fmt.Printf("New Actions Tested:    %d\n", report.TotalActionsTested)
	fmt.Printf("Actions Passed Both:   %d\n", report.ActionsPassedBoth)
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
	cmdLegacy := exec.Command(binPath, fixturePath, "--legacy", "--skip-sign", "-o", legacyOut)
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
		res.Equal = false
		res.Status = "PARITY_DIFFERENCE"
		res.Differences = cmpResult.Differences
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
			LegacyCode: "#include 'actions/location'\n@loc = getParkedCarLocation()\n",
			V2Code:     "let loc = getParkedCarLocation()\n",
		},
		{
			Name:       "scanBarcode",
			Category:   "images",
			LegacyCode: "#include 'actions/images'\n@code = scanBarcode()\n",
			V2Code:     "let code = scanBarcode()\n",
		},
		{
			Name:       "filterPhotos",
			Category:   "photos",
			LegacyCode: "#include 'actions/photos'\n@p = filterPhotos()\n",
			V2Code:     "let p = filterPhotos()\n",
		},
		{
			Name:       "searchiTunes",
			Category:   "music",
			LegacyCode: "#include 'actions/music'\n@res = searchiTunes(\"Artist\")\n",
			V2Code:     "let res = searchiTunes(\"Artist\")\n",
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
			LegacyCode: "#include 'actions/mac'\n@app = getCurrentApp()\n",
			V2Code:     "let app = getCurrentApp()\n",
		},
		{
			Name:       "getSelectedText",
			Category:   "mac",
			LegacyCode: "#include 'actions/mac'\n@txt = getSelectedText()\n",
			V2Code:     "let txt = getSelectedText()\n",
		},
		{
			Name:       "getUpcomingEvents",
			Category:   "calendar",
			LegacyCode: "#include 'actions/calendar'\n@evs = getUpcomingEvents(5)\n",
			V2Code:     "let evs = getUpcomingEvents(5)\n",
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
			LegacyCode: "@val = \"hello\"\n@t = getType(@val)\n",
			V2Code:     "let val = \"hello\"\nlet t = getType(val)\n",
		},
		{
			Name:       "getTravelTime",
			Category:   "location",
			LegacyCode: "#include 'actions/location'\n@dest = \"Central Park\"\n@start = \"Times Square\"\n@t = getTravelTime(@dest, @start)\n",
			V2Code:     "let dest = \"Central Park\"\nlet start = \"Times Square\"\nlet t = getTravelTime(dest, start)\n",
		},
		{
			Name:       "storeValue",
			Category:   "storage",
			LegacyCode: "#include 'actions/storage'\nstoreValue(\"key\", \"val\")\n",
			V2Code:     "storeValue(\"key\", \"val\")\n",
		},
		{
			Name:       "getStoredValue",
			Category:   "storage",
			LegacyCode: "#include 'actions/storage'\n@val = getStoredValue(\"key\")\n",
			V2Code:     "let val = getStoredValue(\"key\")\n",
		},
		{
			Name:       "deleteStoredValue",
			Category:   "storage",
			LegacyCode: "#include 'actions/storage'\ndeleteStoredValue(\"key\")\n",
			V2Code:     "deleteStoredValue(\"key\")\n",
		},
		{
			Name:       "askModel",
			Category:   "intelligence",
			LegacyCode: "#include 'actions/intelligence'\n@ans = askModel(\"prompt\")\n",
			V2Code:     "let ans = askModel(\"prompt\")\n",
		},
		{
			Name:       "askProModel",
			Category:   "intelligence",
			LegacyCode: "#include 'actions/intelligence'\n@ans = askProModel(\"prompt\")\n",
			V2Code:     "let ans = askProModel(\"prompt\")\n",
		},
		{
			Name:       "askDeviceModel",
			Category:   "intelligence",
			LegacyCode: "#include 'actions/intelligence'\n@ans = askDeviceModel(\"prompt\")\n",
			V2Code:     "let ans = askDeviceModel(\"prompt\")\n",
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

		if res.LegacyPassed && res.V2Passed {
			res.Status = "VERIFIED"
		} else if !res.LegacyPassed {
			res.Status = "LEGACY_FAIL"
			res.Detail = strings.TrimSpace(string(outLeg))
		} else {
			res.Status = "V2_FAIL"
			res.Detail = strings.TrimSpace(string(outV2))
		}

		results = append(results, res)
	}

	return results
}

func ensureCompilerBinary() (string, error) {
	root := findRepoRoot()
	binName := "cherri"
	if strings.Contains(strings.ToLower(os.Getenv("OS")), "windows") {
		binName = "cherri.exe"
	}
	binPath := filepath.Join(root, binName)

	cmd := exec.Command("go", "build", "-o", binPath, ".")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("build failed: %s: %w", string(out), err)
	}
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
