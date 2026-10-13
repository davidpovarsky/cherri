package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

type TestEvent struct {
	Time    string  `json:"Time,omitempty"`
	Action  string  `json:"Action"`
	Package string  `json:"Package,omitempty"`
	Test    string  `json:"Test,omitempty"`
	Elapsed float64 `json:"Elapsed,omitempty"`
	Output  string  `json:"Output,omitempty"`
}

var requiredTestNames = []string{
	// CORE family (7)
	"TestCORE001_ShowNamedAction",
	"TestCORE002_VariantsSharingAppleID",
	"TestCORE003_UnknownDefinitionIDHardError",
	"TestCORE004_CustomParamActions",
	"TestCORE005_AppIntentActions",
	"TestCORE006_UnknownLabelError",
	"TestCORE007_DistinctValuesVsOmitted",

	// CODEC family (10)
	"TestCODEC001_LiteralBracesVsFString",
	"TestCODEC002_HebrewEmojiAttachmentOffsets",
	"TestCODEC003_LetDictionaryNativeAction",
	"TestCODEC004_VarDictionaryMaterialization",
	"TestCODEC005_DictInNamedActionArg",
	"TestCODEC006_NestedOrderedDictListWithRefsAndPrimitives",
	"TestCODEC007_ApproachA_JSONTextGetDictionary",
	"TestCODEC008_ApproachD_DictionaryCoercion",
	"TestCODEC009_ApproachC_SetNameJSON",
	"TestCODEC010_CoercionAndTransformationsMatrix",

	// CTRL family (10)
	"TestCTRL001_SimpleIfElse",
	"TestCTRL002_LegacyComparisonAndMultiConditions",
	"TestCTRL003_RepeatCountAndNestedRepeatEach",
	"TestCTRL004_ZeroBasedIndexingAndBounds",
	"TestCTRL005_MenuWithTwoCases",
	"TestCTRL006_ControlFlowWithYield",
	"TestCTRL007_DynamicLazyAndOr",
	"TestCTRL008_ASTControlFlowSourceOwnership",
	"TestCTRL009_RejectStringSliceForWFMenuItems",
	"TestCTRL010_LegacyConditionalParityMatrix",

	// IOS family (6)
	"TestIOS001_InvalidV2Source_DiagnosticNotLegacy",
	"TestIOS002_ValidLegacySource",
	"TestIOS003_PaletteActionSnippetsInV2",
	"TestIOS004_AdvancedV2SourceVisualEdit",
	"TestIOS005_UnsupportedVisualEdit_SourcePreserved",
	"TestIOS006_ConcurrentRepeatedCompile",

	// META family (3)
	"TestMETA001_WorkflowHeaderMetadata",
	"TestMETA002_NoInputBehaviorAndQuestions",
	"TestMETA003_Triggers",

	// LANG family (5)
	"TestLANG001_ImportsAndCustomAction",
	"TestLANG002_EnumAndTypeAliases",
	"TestLANG003_NativeEscapeAPIs",
	"TestLANG004_UnhandledDeclarationsNotSilent",
	"TestLANG005_MigrationSafeCommentsAndVars",

	// GEN family (3)
	"TestGEN001_StaticParametersNestedMapsPreserved",
	"TestGEN002_FingerprintDetectsMutations",
	"TestGEN003_DuplicateAppleIdentifierRetainsAllVariants",
}

type DiscoveryResult struct {
	TotalRequired int
	TotalPassed   int
	Missing       []string
	Passed        map[string]bool
	Failures      map[string]string
	FamilyCounts  map[string]int
}

func VerifyEvents(reader io.Reader) (*DiscoveryResult, error) {
	scanner := bufio.NewScanner(reader)
	// Allow large output lines
	buf := make([]byte, 1024*1024)
	scanner.Buffer(buf, 10*1024*1024)

	passedTopLevel := make(map[string]bool)
	failedTopLevel := make(map[string]string)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 || line[0] != '{' {
			continue
		}
		var ev TestEvent
		if err := json.Unmarshal(line, &ev); err != nil {
			continue
		}
		if ev.Test == "" {
			continue
		}
		// Count only top-level tests (no "/" subtests)
		if strings.Contains(ev.Test, "/") {
			continue
		}

		switch ev.Action {
		case "pass":
			passedTopLevel[ev.Test] = true
		case "fail":
			failedTopLevel[ev.Test] = "failed"
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading test event stream: %w", err)
	}

	result := &DiscoveryResult{
		TotalRequired: len(requiredTestNames),
		Passed:        passedTopLevel,
		Failures:      failedTopLevel,
		FamilyCounts:  make(map[string]int),
	}

	for _, req := range requiredTestNames {
		if passedTopLevel[req] {
			result.TotalPassed++
			family := extractFamily(req)
			result.FamilyCounts[family]++
		} else {
			result.Missing = append(result.Missing, req)
		}
	}

	return result, nil
}

func extractFamily(testName string) string {
	s := strings.TrimPrefix(testName, "Test")
	for i, r := range s {
		if r >= '0' && r <= '9' {
			return s[:i]
		}
	}
	return s
}

// RunCommandAndVerify executes a command streaming test events, verifies them, and propagates child exit errors.
func RunCommandAndVerify(cmd *exec.Cmd, out io.Writer) (*DiscoveryResult, error) {
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to get stdout pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start command: %w", err)
	}

	var r io.Reader = stdout
	if out != nil {
		r = io.TeeReader(stdout, out)
	}
	res, vErr := VerifyEvents(r)
	waitErr := cmd.Wait()

	if waitErr != nil {
		return res, fmt.Errorf("child test process failed: %w", waitErr)
	}
	if vErr != nil {
		return nil, fmt.Errorf("verification error: %w", vErr)
	}
	return res, nil
}

func main() {
	runRegex := flag.String("run", "", "If set, executes go test with -json using this regex and verifies output")
	inputFile := flag.String("file", "", "Path to test output JSON file (default: stdin)")
	flag.Parse()

	var reader io.Reader

	if *runRegex != "" {
		fmt.Printf("=== Running go test -json with regex: %s ===\n", *runRegex)
		cmd := exec.Command("go", "test", "-count=1", "-timeout=15m", "-json", "-run", *runRegex, "./...")
		cmd.Stderr = os.Stderr
		res, err := RunCommandAndVerify(cmd, os.Stdout)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}
		printReportAndExit(res)
		return
	}

	if *inputFile != "" {
		f, err := os.Open(*inputFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "cannot open input file: %v\n", err)
			os.Exit(1)
		}
		defer f.Close()
		reader = f
	} else {
		// Check if input is piped into stdin
		stat, _ := os.Stdin.Stat()
		isPipe := (stat.Mode() & os.ModeNamedPipe) != 0
		hasData := stat.Size() > 0

		if !isPipe && !hasData {
			// No stdin piped, default to running the canonical matrix regex
			fmt.Println("No input piped; running canonical full matrix regex: ^Test(CORE|CODEC|CTRL|META|LANG|IOS|GEN)")
			cmd := exec.Command("go", "test", "-count=1", "-timeout=15m", "-json", "-run", "^Test(CORE|CODEC|CTRL|META|LANG|IOS|GEN)", "./...")
			cmd.Stderr = os.Stderr
			res, err := RunCommandAndVerify(cmd, os.Stdout)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%v\n", err)
				os.Exit(1)
			}
			printReportAndExit(res)
			return
		}
		reader = os.Stdin
	}

	res, err := VerifyEvents(reader)
	if err != nil {
		fmt.Fprintf(os.Stderr, "verification error: %v\n", err)
		os.Exit(1)
	}
	printReportAndExit(res)
}

func printReportAndExit(res *DiscoveryResult) {
	fmt.Println("\n==================================================")
	fmt.Println("       TEST DISCOVERY & REGRESSION GATE")
	fmt.Println("==================================================")
	families := []string{"CORE", "CODEC", "CTRL", "IOS", "META", "LANG", "GEN"}
	expectedPerFamily := map[string]int{
		"CORE":  7,
		"CODEC": 10,
		"CTRL":  10,
		"IOS":   6,
		"META":  3,
		"LANG":  5,
		"GEN":   3,
	}

	for _, fam := range families {
		got := res.FamilyCounts[fam]
		expected := expectedPerFamily[fam]
		status := "PASS"
		if got < expected {
			status = "FAIL"
		}
		fmt.Printf("  [%s] Family %-6s: %2d / %2d passed\n", status, fam, got, expected)
	}

	fmt.Printf("\nTotal Required: %d\n", res.TotalRequired)
	fmt.Printf("Total Passed:   %d\n", res.TotalPassed)

	if len(res.Missing) > 0 {
		fmt.Printf("\nGATE FAILED: %d mandatory tests were NOT discovered or did not pass:\n", len(res.Missing))
		for _, m := range res.Missing {
			fmt.Printf("  MISSING: %s\n", m)
		}
		os.Exit(1)
	}

	if len(res.Failures) > 0 {
		fmt.Printf("\nGATE FAILED: %d tests failed:\n", len(res.Failures))
		for name, reason := range res.Failures {
			fmt.Printf("  FAILED: %s (%s)\n", name, reason)
		}
		os.Exit(1)
	}

	fmt.Printf("\nALL %d MANDATORY TOP-LEVEL REGRESSION TESTS VERIFIED PASS!\n", len(requiredTestNames))
	fmt.Println("GATE: PASS")
	os.Exit(0)
}
