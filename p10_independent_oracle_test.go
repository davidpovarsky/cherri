/*
 * Copyright (c) Cherri
 */
package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"
)

const PinnedUpstreamCommit = "bcc0095754e2cc0baaeb4d93dae172b8153b4716"

// normalizeOracleValue normalizes volatile UUIDs and canonicalizes WFDictionaryFieldValueItems
// (sorting dictionary items by key and normalizing omitted WFItemType -> uint64(0), since
// upstream iterates a Go map in non-deterministic order and uses omitempty on WFItemType=0).
func normalizeOracleValue(v any) any {
	masked := maskVolatileUUIDs(v)
	return canonicalizeDictItems(masked)
}

func canonicalizeDictItems(v any) any {
	switch val := v.(type) {
	case map[string]any:
		res := make(map[string]any, len(val))
		for k, elem := range val {
			if k == "WFDictionaryFieldValueItems" {
				if slice, ok := elem.([]any); ok {
					normSlice := make([]any, len(slice))
					for i, rawItem := range slice {
						itemMap, isMap := rawItem.(map[string]any)
						if !isMap {
							normSlice[i] = rawItem
							continue
						}
						cp := make(map[string]any, len(itemMap)+1)
						for ik, iv := range itemMap {
							cp[ik] = canonicalizeDictItems(iv)
						}
						if _, hasType := cp["WFItemType"]; !hasType {
							cp["WFItemType"] = uint64(0)
						}
						normSlice[i] = cp
					}
					sort.Slice(normSlice, func(i, j int) bool {
						mi, _ := normSlice[i].(map[string]any)
						mj, _ := normSlice[j].(map[string]any)
						ki, _ := mi["WFKey"].(map[string]any)["Value"].(map[string]any)["string"].(string)
						kj, _ := mj["WFKey"].(map[string]any)["Value"].(map[string]any)["string"].(string)
						return ki < kj
					})
					res[k] = normSlice
					continue
				}
			}
			res[k] = canonicalizeDictItems(elem)
		}
		return res
	case []any:
		res := make([]any, len(val))
		for i, elem := range val {
			res[i] = canonicalizeDictItems(elem)
		}
		return res
	default:
		return v
	}
}

// assertOracleDocsEqual compares two parsed .shortcut plist documents produced by
// the upstream binary and our fork's binary, comparing action identifiers, parameter
// keys, and normalized values while safely handling actions with empty/nil parameter maps.
func assertOracleDocsEqual(t *testing.T, upstreamDoc, forkDoc map[string]any, ignoredKeys ...string) {
	t.Helper()
	upActions, ok1 := upstreamDoc["WFWorkflowActions"].([]any)
	forkActions, ok2 := forkDoc["WFWorkflowActions"].([]any)
	if !ok1 || !ok2 {
		t.Fatalf("missing WFWorkflowActions in upstream (%v) or fork (%v) plist", ok1, ok2)
	}
	if len(upActions) != len(forkActions) {
		t.Fatalf("action count mismatch: upstream=%d vs fork=%d", len(upActions), len(forkActions))
	}

	ignoreMap := map[string]bool{
		"UUID":               true,
		"GroupingIdentifier": true,
		"CustomOutputName":   true,
	}
	for _, k := range ignoredKeys {
		ignoreMap[k] = true
	}

	for i := range upActions {
		upMap := upActions[i].(map[string]any)
		forkMap := forkActions[i].(map[string]any)

		if upMap["WFWorkflowActionIdentifier"] != forkMap["WFWorkflowActionIdentifier"] {
			t.Fatalf("action[%d] identifier mismatch: upstream=%v vs fork=%v",
				i, upMap["WFWorkflowActionIdentifier"], forkMap["WFWorkflowActionIdentifier"])
		}

		upParams, _ := upMap["WFWorkflowActionParameters"].(map[string]any)
		if upParams == nil {
			upParams = map[string]any{}
		}
		forkParams, _ := forkMap["WFWorkflowActionParameters"].(map[string]any)
		if forkParams == nil {
			forkParams = map[string]any{}
		}

		for k, vUp := range upParams {
			if ignoreMap[k] {
				continue
			}
			vFork, found := forkParams[k]
			if !found {
				t.Fatalf("action[%d] (%v): upstream parameter %q missing in fork plist (fork params: %v)",
					i, upMap["WFWorkflowActionIdentifier"], k, forkParams)
			}
			upJSON, _ := json.Marshal(normalizeOracleValue(vUp))
			forkJSON, _ := json.Marshal(normalizeOracleValue(vFork))
			if string(upJSON) != string(forkJSON) {
				t.Fatalf("action[%d] (%v) parameter %q mismatch:\nupstream: %s\nfork:     %s",
					i, upMap["WFWorkflowActionIdentifier"], k, upJSON, forkJSON)
			}
		}

		for k := range forkParams {
			if ignoreMap[k] {
				continue
			}
			if _, found := upParams[k]; !found {
				t.Fatalf("action[%d] (%v): fork emitted unexpected parameter %q not present in upstream",
					i, forkMap["WFWorkflowActionIdentifier"], k)
			}
		}
	}
}

// Independent Oracle (Static + Behavioral): Compares action definitions and wire
// plist output against original pinned upstream Cherri
// (electrikmilk/cherri@bcc0095754e2cc0baaeb4d93dae172b8153b4716).
// This is an external, independent oracle check, not internal legacy parity.
func TestP10_IndependentOracle_UpstreamPinnedCommitComparison(t *testing.T) {
	// 1. Verify that the pinned upstream commit exists in the local Git object store
	cmdCheck := exec.Command("git", "cat-file", "-e", PinnedUpstreamCommit+"^{commit}")
	if out, err := cmdCheck.CombinedOutput(); err != nil {
		t.Fatalf("independent oracle: pinned upstream commit %s is not available in git database: %v\n%s",
			PinnedUpstreamCommit, err, string(out))
	}

	// 2. Fetch upstream math.cherri from pinned commit
	cmdShowMath := exec.Command("git", "show", PinnedUpstreamCommit+":actions/math.cherri")
	upstreamMathBytes, err := cmdShowMath.Output()
	if err != nil {
		t.Fatalf("independent oracle: failed to read upstream math.cherri at %s: %v", PinnedUpstreamCommit, err)
	}
	upstreamMath := string(upstreamMathBytes)

	// Upstream must define statistic action with WFInput
	statRegex := regexp.MustCompile(`action\s+'statistics'\s+statistic\([^)]*variable\s+input:\s*'([^']+)'`)
	upstreamMatches := statRegex.FindStringSubmatch(upstreamMath)
	if len(upstreamMatches) < 2 {
		t.Fatalf("independent oracle: failed to find statistic action definition in upstream commit %s", PinnedUpstreamCommit)
	}
	upstreamWireKey := upstreamMatches[1]
	if upstreamWireKey != "WFInput" {
		t.Fatalf("independent oracle: expected upstream wire key 'WFInput', got %q", upstreamWireKey)
	}
	t.Logf("Independent Oracle verified: Upstream commit %s defines statistic input wire key as %q",
		PinnedUpstreamCommit[:8], upstreamWireKey)

	// Verify our fork's local actions/math.cherri matches the upstream oracle
	cmdShowLocalMath := exec.Command("git", "show", "HEAD:actions/math.cherri")
	localMathBytes, err := cmdShowLocalMath.Output()
	if err != nil {
		t.Fatalf("failed to read local actions/math.cherri: %v", err)
	}
	localMatches := statRegex.FindStringSubmatch(string(localMathBytes))
	if len(localMatches) < 2 {
		t.Fatalf("failed to find statistic action definition in local actions/math.cherri")
	}
	localWireKey := localMatches[1]
	if localWireKey != upstreamWireKey {
		t.Fatalf("CRITICAL ORACLE DRIFT: local wire key %q differs from upstream oracle %q",
			localWireKey, upstreamWireKey)
	}
	t.Logf("Independent Oracle parity: local wire key matches upstream oracle (%q == %q)", localWireKey, upstreamWireKey)

	// 3. Upstream runworkflow definition inspection
	cmdShowStd := exec.Command("git", "show", PinnedUpstreamCommit+":actions_std.go")
	upstreamStdBytes, err := cmdShowStd.Output()
	if err != nil {
		t.Fatalf("failed to read upstream actions_std.go at %s: %v", PinnedUpstreamCommit, err)
	}
	upstreamStd := string(upstreamStdBytes)
	if !strings.Contains(upstreamStd, `"WFInput"`) || !strings.Contains(upstreamStd, `"runworkflow"`) {
		t.Fatalf("independent oracle: upstream actions_std.go missing runworkflow or WFInput key")
	}
	t.Logf("Independent Oracle verified: Upstream runworkflow uses WFInput serialization standard")
}

// buildUpstreamOracleBinary checks out PinnedUpstreamCommit into an isolated detached
// git worktree, builds the real upstream compiler binary, and returns its executable path.
func buildUpstreamOracleBinary(t *testing.T) string {
	t.Helper()

	cmdCheck := exec.Command("git", "cat-file", "-e", PinnedUpstreamCommit+"^{commit}")
	if out, err := cmdCheck.CombinedOutput(); err != nil {
		t.Fatalf("independent oracle: pinned upstream commit %s missing from git object store: %v\n%s",
			PinnedUpstreamCommit, err, string(out))
	}

	tmpBase := t.TempDir()
	worktreeDir := filepath.Join(tmpBase, "upstream-worktree")

	cmdAdd := exec.Command("git", "worktree", "add", "--detach", worktreeDir, PinnedUpstreamCommit)
	if out, err := cmdAdd.CombinedOutput(); err != nil {
		t.Fatalf("independent oracle: failed to create detached worktree for %s: %v\n%s",
			PinnedUpstreamCommit, err, string(out))
	}
	t.Cleanup(func() {
		_ = exec.Command("git", "worktree", "remove", "--force", worktreeDir).Run()
		_ = exec.Command("git", "worktree", "prune").Run()
	})

	exeExt := ""
	if runtime.GOOS == "windows" {
		exeExt = ".exe"
	}
	upstreamBin := filepath.Join(tmpBase, "upstream-cherri"+exeExt)

	cmdBuild := exec.Command("go", "build", "-o", upstreamBin, ".")
	cmdBuild.Dir = worktreeDir
	if out, err := cmdBuild.CombinedOutput(); err != nil {
		t.Fatalf("independent oracle: failed to build upstream binary at %s: %v\n%s",
			PinnedUpstreamCommit, err, string(out))
	}
	return upstreamBin
}

func compileWithBinary(t *testing.T, binPath, workDir, name, source string, extraArgs ...string) string {
	t.Helper()
	srcPath := filepath.Join(workDir, name+".cherri")
	if err := os.WriteFile(srcPath, []byte(source), 0o600); err != nil {
		t.Fatalf("write %s: %v", srcPath, err)
	}
	args := []string{srcPath, "--skip-sign", "--no-ansi"}
	args = append(args, extraArgs...)
	cmd := exec.Command(binPath, args...)
	cmd.Dir = workDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%s (%s): compilation failed: %v\n%s", name, filepath.Base(binPath), err, string(out))
	}
	outPath := filepath.Join(workDir, name+"_unsigned.shortcut")
	if _, err := os.Stat(outPath); err != nil {
		altPath := filepath.Join(workDir, name+".shortcut")
		if _, altErr := os.Stat(altPath); altErr == nil {
			return altPath
		}
		t.Fatalf("%s: expected output shortcut %s missing: %v", name, outPath, err)
	}
	return outPath
}

func compileWithBinaryExpectError(t *testing.T, binPath, workDir, name, source string, extraArgs ...string) (string, error) {
	t.Helper()
	srcPath := filepath.Join(workDir, name+".cherri")
	if err := os.WriteFile(srcPath, []byte(source), 0o600); err != nil {
		t.Fatalf("write %s: %v", srcPath, err)
	}
	args := []string{srcPath, "--skip-sign", "--no-ansi"}
	args = append(args, extraArgs...)
	cmd := exec.Command(binPath, args...)
	cmd.Dir = workDir
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// Behavioral Independent Oracle: Builds the locked upstream Cherri binary at
// PinnedUpstreamCommit (bcc0095754e2cc0baaeb4d93dae172b8153b4716) and runs both
// the upstream binary and our fork's binary on shared supported cases, comparing
// the resulting plist structure, action identifiers, fields, and values.
func TestP10_IndependentOracle_UpstreamBinaryBehavioralComparison(t *testing.T) {
	upstreamBin := buildUpstreamOracleBinary(t)
	forkBin := roundTripBinary(t)
	workDir := t.TempDir()

	// 1. Behavioral Oracle: statistic action across operations (Average, Sum, Minimum, Maximum)
	t.Run("statistic-plist-equality", func(t *testing.T) {
		for _, op := range []string{"Average", "Sum", "Minimum", "Maximum"} {
			src := fmt.Sprintf("#include 'actions/math'\n@nums = [10, 20, 30]\n@res = statistic('%s', @nums)\n", op)
			upPlist := compileWithBinary(t, upstreamBin, workDir, "up_stat_"+op, src)
			forkLegacyPlist := compileWithBinary(t, forkBin, workDir, "fork_legacy_stat_"+op, src, "--legacy")

			upDoc := loadRoundTripDocument(t, upPlist)
			forkLegacyDoc := loadRoundTripDocument(t, forkLegacyPlist)

			// Full structural comparison of WFWorkflowActions between upstream binary and fork binary
			assertOracleDocsEqual(t, upDoc, forkLegacyDoc, "WFWorkflow")

			// Also compile V2 syntax with fork binary and compare the statistics action parameters against upstream
			v2Src := fmt.Sprintf("let nums = [10, 20, 30]\nlet res = statistic(%q, input: nums)\n", op)
			forkV2Plist := compileWithBinary(t, forkBin, workDir, "fork_v2_stat_"+op, v2Src)
			forkV2Doc := loadRoundTripDocument(t, forkV2Plist)

			findStatParams := func(doc map[string]any, label string) map[string]any {
				acts, _ := doc["WFWorkflowActions"].([]any)
				for _, a := range acts {
					m := a.(map[string]any)
					if m["WFWorkflowActionIdentifier"] == "is.workflow.actions.statistics" {
						return m["WFWorkflowActionParameters"].(map[string]any)
					}
				}
				t.Fatalf("%s: missing is.workflow.actions.statistics action", label)
				return nil
			}

			upParams := findStatParams(upDoc, "upstream")
			v2Params := findStatParams(forkV2Doc, "fork-v2")

			if upParams["WFStatisticsOperation"] != v2Params["WFStatisticsOperation"] {
				t.Errorf("statistic(%s): WFStatisticsOperation mismatch: upstream=%v vs fork-v2=%v",
					op, upParams["WFStatisticsOperation"], v2Params["WFStatisticsOperation"])
			}
			if _, hasDeprecated := v2Params["Input"]; hasDeprecated {
				t.Errorf("statistic(%s): fork-v2 emitted deprecated 'Input' wire key", op)
			}
			upInput, ok1 := upParams["WFInput"].(map[string]any)
			v2Input, ok2 := v2Params["WFInput"].(map[string]any)
			if !ok1 || !ok2 {
				t.Fatalf("statistic(%s): expected WFInput map in both upstream (%T) and fork-v2 (%T)", op, upParams["WFInput"], v2Params["WFInput"])
			}
			if upInput["WFSerializationType"] != v2Input["WFSerializationType"] {
				t.Errorf("statistic(%s): WFInput.WFSerializationType mismatch: upstream=%v vs fork-v2=%v",
					op, upInput["WFSerializationType"], v2Input["WFSerializationType"])
			}
		}
	})

	// 2. Behavioral Oracle: runworkflow with dictionary payload (direct upstream `run` via `#include 'actions/shortcuts'` + shared `runSelf(@payload)` + fork V2 `run("Target", input: payload)`)
	t.Run("runworkflow-dictionary-payload-plist-equality", func(t *testing.T) {
		// 2a. Direct 3-way behavioral oracle on `run("Target", payload)` across fixed target, dynamic target, text payload, dictionary payload, nested dictionary payload, and output capture!
		// In upstream bcc0095, `run` is defined in `actions/shortcuts.cherri` as:
		// `action 'runworkflow' run(text shortcutName: 'WFWorkflowName', variable input: 'WFInput')`
		directRunLegacySrc := `#include 'actions/shortcuts'
@targetName = "DynamicServer"
@textPayload = "hello-server"
@dictPayload = {"model": "gpt-4", "count": 2, "nested": {"env": "prod", "active": true}}
@out1 = run("FixedServer", @textPayload)
@out2 = run("FixedServer", @dictPayload)
@out3 = run(@targetName, @dictPayload)
`
		directRunV2Src := `let targetName = "DynamicServer"
let textPayload = "hello-server"
let dictPayload = {"model": "gpt-4", "count": 2, "nested": {"env": "prod", "active": true}}
let out1 = run("FixedServer", input: textPayload)
let out2 = run("FixedServer", input: dictPayload)
let out3 = run(targetName, input: dictPayload)
`
		upDirectRunPlist := compileWithBinary(t, upstreamBin, workDir, "up_direct_run", directRunLegacySrc)
		forkLegacyDirectRunPlist := compileWithBinary(t, forkBin, workDir, "fork_legacy_direct_run", directRunLegacySrc, "--legacy")
		forkV2DirectRunPlist := compileWithBinary(t, forkBin, workDir, "fork_v2_direct_run", directRunV2Src)

		upDirectDoc := loadRoundTripDocument(t, upDirectRunPlist)
		forkLegacyDirectDoc := loadRoundTripDocument(t, forkLegacyDirectRunPlist)
		forkV2DirectDoc := loadRoundTripDocument(t, forkV2DirectRunPlist)

		// Compare Upstream bcc0095 vs Fork Legacy on `run(...)`: every action and parameter must match
		// except our fork's intentional addition of `WFWorkflow` descriptor dictionary alongside `WFWorkflowName`.
		assertOracleDocsEqual(t, upDirectDoc, forkLegacyDirectDoc, "WFWorkflow")

		// Verify all 3 `runworkflow` actions in Fork Legacy and Fork V2 have both `WFWorkflowName`, `WFInput`, and `WFWorkflow` descriptor
		collectRunActions := func(doc map[string]any, label string) []map[string]any {
			var runs []map[string]any
			for _, a := range doc["WFWorkflowActions"].([]any) {
				m := a.(map[string]any)
				if m["WFWorkflowActionIdentifier"] == "is.workflow.actions.runworkflow" {
					runs = append(runs, m)
				}
			}
			if len(runs) != 3 {
				t.Fatalf("%s: expected 3 runworkflow actions, got %d", label, len(runs))
			}
			return runs
		}
		upRuns := collectRunActions(upDirectDoc, "upstream-direct-run")
		legRuns := collectRunActions(forkLegacyDirectDoc, "fork-legacy-direct-run")
		v2Runs := collectRunActions(forkV2DirectDoc, "fork-v2-direct-run")

		for i := 0; i < 3; i++ {
			upP := upRuns[i]["WFWorkflowActionParameters"].(map[string]any)
			legP := legRuns[i]["WFWorkflowActionParameters"].(map[string]any)
			v2P := v2Runs[i]["WFWorkflowActionParameters"].(map[string]any)

			// Verify WFInput presence and serialization parity across all 3 compilers
			upIn, ok1 := upP["WFInput"].(map[string]any)
			legIn, ok2 := legP["WFInput"].(map[string]any)
			v2In, ok3 := v2P["WFInput"].(map[string]any)
			if !ok1 || !ok2 || !ok3 {
				t.Fatalf("run[%d]: missing WFInput in upstream(%v), legacy(%v), or v2(%v)", i, ok1, ok2, ok3)
			}
			if upIn["WFSerializationType"] != "WFTextTokenAttachment" || legIn["WFSerializationType"] != "WFTextTokenAttachment" || v2In["WFSerializationType"] != "WFTextTokenAttachment" {
				t.Errorf("run[%d]: expected WFInput.WFSerializationType == WFTextTokenAttachment across all 3 compilers", i)
			}

			// Verify Fork Legacy & V2 emit WFWorkflow descriptor with isSelf=false and workflowName matching WFWorkflowName
			for modeLabel, p := range map[string]map[string]any{"legacy": legP, "v2": v2P} {
				wfDesc, ok := p["WFWorkflow"].(map[string]any)
				if !ok {
					t.Fatalf("run[%d] (%s): missing WFWorkflow descriptor map", i, modeLabel)
				}
				if wfDesc["isSelf"] != false {
					t.Errorf("run[%d] (%s): expected WFWorkflow.isSelf == false, got %v", i, modeLabel, wfDesc["isSelf"])
				}
				nameJSON, _ := json.Marshal(normalizeOracleValue(p["WFWorkflowName"]))
				descNameJSON, _ := json.Marshal(normalizeOracleValue(wfDesc["workflowName"]))
				if string(nameJSON) != string(descNameJSON) {
					t.Errorf("run[%d] (%s): WFWorkflowName (%s) != WFWorkflow.workflowName (%s)", i, modeLabel, nameJSON, descNameJSON)
				}
				if v2P["UUID"] == nil {
					t.Errorf("run[%d] (v2): expected output UUID on captured run() call", i)
				}
			}
		}

		// Also verify optional input in Fork Legacy & V2 (`run("NoInputTarget")`)
		noInLegPlist := compileWithBinary(t, forkBin, workDir, "fork_leg_run_noin", `run("NoInputTarget")`+"\n", "--legacy")
		noInV2Plist := compileWithBinary(t, forkBin, workDir, "fork_v2_run_noin", `run("NoInputTarget")`+"\n")
		assertOracleDocsEqual(t, loadRoundTripDocument(t, noInLegPlist), loadRoundTripDocument(t, noInV2Plist), "WFWorkflow")

		// 2b. Shared `runSelf(@payload)` comparison (documented separately from `run("Target", payload)` since `runSelf` sets `isSelf: true`)
		sharedSrc := `@payload = {"model": "gpt-4", "count": 2}
runSelf(@payload)
`
		upPlist := compileWithBinary(t, upstreamBin, workDir, "up_runself_dict", sharedSrc)
		forkLegacyPlist := compileWithBinary(t, forkBin, workDir, "fork_runself_dict", sharedSrc, "--legacy")

		upDoc := loadRoundTripDocument(t, upPlist)
		forkLegacyDoc := loadRoundTripDocument(t, forkLegacyPlist)

		// Exact structural equality on WFWorkflowActions (including WFDictionaryFieldValueItems and WFInput attachment)
		assertOracleDocsEqual(t, upDoc, forkLegacyDoc, "WFWorkflow")

		// Also compare upstream's dictionary items and WFInput token structure against our fork's 2-arg run("Target", payload)
		forkRunV2Src := `let payload = {"model": "gpt-4", "count": 2}
run("Target", input: payload)
`
		forkRunV2Plist := compileWithBinary(t, forkBin, workDir, "fork_run_v2_dict", forkRunV2Src)
		forkRunV2Doc := loadRoundTripDocument(t, forkRunV2Plist)

		extractDictAndRun := func(doc map[string]any, label string) (map[string]any, map[string]any) {
			var dictAct, runAct map[string]any
			for _, a := range doc["WFWorkflowActions"].([]any) {
				m := a.(map[string]any)
				switch m["WFWorkflowActionIdentifier"] {
				case "is.workflow.actions.dictionary":
					dictAct = m
				case "is.workflow.actions.runworkflow":
					runAct = m
				}
			}
			if dictAct == nil || runAct == nil {
				t.Fatalf("%s: expected both dictionary and runworkflow actions", label)
			}
			return dictAct, runAct
		}

		upDictAct, upRunAct := extractDictAndRun(upDoc, "upstream-runSelf")
		v2DictAct, v2RunAct := extractDictAndRun(forkRunV2Doc, "fork-v2-run")

		upDictItems := extractDictItems(t, upDictAct)
		v2DictItems := extractDictItems(t, v2DictAct)

		if len(upDictItems) != len(v2DictItems) {
			t.Fatalf("dictionary payload item count mismatch: upstream=%d vs fork-v2=%d", len(upDictItems), len(v2DictItems))
		}
		for k, upItem := range upDictItems {
			v2Item, ok := v2DictItems[k]
			if !ok {
				t.Errorf("fork-v2 dictionary payload missing key %q present in upstream", k)
				continue
			}
			if upItem.itemType != v2Item.itemType || upItem.val != v2Item.val {
				t.Errorf("dictionary payload key %q mismatch: upstream=%+v vs fork-v2=%+v", k, upItem, v2Item)
			}
		}

		upWFInput := upRunAct["WFWorkflowActionParameters"].(map[string]any)["WFInput"].(map[string]any)
		v2WFInput := v2RunAct["WFWorkflowActionParameters"].(map[string]any)["WFInput"].(map[string]any)
		if upWFInput["WFSerializationType"] != v2WFInput["WFSerializationType"] {
			t.Errorf("runworkflow WFInput.WFSerializationType mismatch: upstream=%v vs fork-v2=%v",
				upWFInput["WFSerializationType"], v2WFInput["WFSerializationType"])
		}
	})

	// 3. Behavioral Oracle: Regex quantifiers and JSON braces in strings
	t.Run("regex-quantifiers-and-json-strings-plist-equality", func(t *testing.T) {
		// Upstream uses single-quoted raw strings for regex/JSON with braces; our fork supports both single-quoted and double-quoted
		upSrc := `#include 'actions/text'
@m = matchText('[0-9a-fA-F]{8}-[0-9a-fA-F]{4}', "550e8400-e29b")
@j = '{"model":"gpt-4","count":2}'
`
		forkDoubleQuoteSrc := `#include 'actions/text'
@m = matchText("[0-9a-fA-F]{8}-[0-9a-fA-F]{4}", "550e8400-e29b")
@j = "{\"model\":\"gpt-4\",\"count\":2}"
`
		upPlist := compileWithBinary(t, upstreamBin, workDir, "up_regex_json", upSrc)
		forkSinglePlist := compileWithBinary(t, forkBin, workDir, "fork_single_regex_json", upSrc, "--legacy")
		forkDoublePlist := compileWithBinary(t, forkBin, workDir, "fork_double_regex_json", forkDoubleQuoteSrc, "--legacy")

		upDoc := loadRoundTripDocument(t, upPlist)
		forkSingleDoc := loadRoundTripDocument(t, forkSinglePlist)
		forkDoubleDoc := loadRoundTripDocument(t, forkDoublePlist)

		assertOracleDocsEqual(t, upDoc, forkSingleDoc, "WFWorkflow")

		// For double-quoted regex and JSON strings in our fork, verify the emitted matchText and gettext
		// actions match upstream's single-quoted raw string actions (ignoring only the OutputName Rawtext vs Text label on setvariable)
		assertOracleDocsEqual(t, upDoc, forkDoubleDoc, "WFWorkflow", "WFInput")
		upTextParam := upDoc["WFWorkflowActions"].([]any)[2].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)["WFTextActionText"]
		dblTextParam := forkDoubleDoc["WFWorkflowActions"].([]any)[2].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)["WFTextActionText"]
		if upTextParam != dblTextParam || dblTextParam != `{"model":"gpt-4","count":2}` {
			t.Fatalf("gettext WFTextActionText mismatch: upstream=%v vs fork-double=%v", upTextParam, dblTextParam)
		}
	})

	// 4. Behavioral Oracle: Variable interpolation (WFTextTokenString)
	t.Run("variable-interpolation-plist-equality", func(t *testing.T) {
		src := `@name = "Ada"
show("Hello {@name}!")
`
		upPlist := compileWithBinary(t, upstreamBin, workDir, "up_interp", src)
		forkLegacyPlist := compileWithBinary(t, forkBin, workDir, "fork_interp", src, "--legacy")

		upDoc := loadRoundTripDocument(t, upPlist)
		forkLegacyDoc := loadRoundTripDocument(t, forkLegacyPlist)

		assertOracleDocsEqual(t, upDoc, forkLegacyDoc, "WFWorkflow")
	})

	// 5. Behavioral Oracle: Invalid array equality conditional fails closed on both compilers
	t.Run("invalid-array-equality-conditional-fails-closed", func(t *testing.T) {
		legacySrc := `@matches = ["a", "b"]
if @matches == [] {
    alert("bad")
}
`
		upOut, upErr := compileWithBinaryExpectError(t, upstreamBin, workDir, "up_bad_cond", legacySrc)
		if upErr == nil {
			t.Fatalf("expected upstream binary to reject '@matches == []', but compilation succeeded")
		}
		if !strings.Contains(upOut, "Invalid type 'array' for conditional '=='") {
			t.Fatalf("unexpected upstream error output: %s", upOut)
		}

		forkLegacyOut, forkLegacyErr := compileWithBinaryExpectError(t, forkBin, workDir, "fork_legacy_bad_cond", legacySrc, "--legacy")
		if forkLegacyErr == nil {
			t.Fatalf("expected fork binary (--legacy) to reject '@matches == []', but compilation succeeded")
		}
		if !strings.Contains(forkLegacyOut, "Invalid type 'array' for conditional '=='") {
			t.Fatalf("unexpected fork (--legacy) error output: %s", forkLegacyOut)
		}

		v2Src := `let matches = ["a", "b"]
if matches == [] {
    alert(alert: "bad")
}
`
		forkV2Out, forkV2Err := compileWithBinaryExpectError(t, forkBin, workDir, "fork_v2_bad_cond", v2Src)
		if forkV2Err == nil {
			t.Fatalf("expected fork binary (v2) to reject 'matches == []', but compilation succeeded")
		}
		if !strings.Contains(forkV2Out, "Invalid type 'array' for conditional '=='") {
			t.Fatalf("unexpected fork (v2) error output: %s", forkV2Out)
		}
	})

	// 6. Behavioral Oracle: Negative fail-closed tests for invalid enums, missing required parameters, and macOnly guards
	t.Run("negative-fail-closed-enums-missing-args-and-platform-guards", func(t *testing.T) {
		// 6a. Invalid enum value in statistic
		badEnumLeg := "#include 'actions/math'\n@nums = [1, 2]\nstatistic('NotAnOperation', @nums)\n"
		if _, err := compileWithBinaryExpectError(t, upstreamBin, workDir, "up_bad_enum", badEnumLeg); err == nil {
			t.Fatalf("expected upstream binary to reject invalid statistic enum")
		}
		if _, err := compileWithBinaryExpectError(t, forkBin, workDir, "fork_leg_bad_enum", badEnumLeg, "--legacy"); err == nil {
			t.Fatalf("expected fork --legacy binary to reject invalid statistic enum")
		}
		badEnumV2 := "let nums = [1, 2]\nstatistic(\"NotAnOperation\", input: nums)\n"
		if out, err := compileWithBinaryExpectError(t, forkBin, workDir, "fork_v2_bad_enum", badEnumV2); err == nil || !strings.Contains(out, "invalid enum value") {
			t.Fatalf("expected fork v2 binary to reject invalid statistic enum with 'invalid enum value', got err=%v, out=%s", err, out)
		}

		// 6b. Missing required parameter
		missingArgV2 := "getValue({\"a\": 1})\n"
		if out, err := compileWithBinaryExpectError(t, forkBin, workDir, "fork_v2_missing_arg", missingArgV2); err == nil || !strings.Contains(out, "missing required argument") {
			t.Fatalf("expected fork v2 binary to reject missing required argument, got err=%v, out=%s", err, out)
		}

		// 6c. macOnly guard when targeting non-Mac (iOS/iPadOS)
		macOnlyLeg := "#define mac false\n#include 'actions/mac'\nsleep()\n"
		if out, err := compileWithBinaryExpectError(t, forkBin, workDir, "fork_leg_maconly", macOnlyLeg, "--legacy"); err == nil {
			t.Fatalf("expected fork --legacy binary to reject macOnly action when mac=false, got success: %s", out)
		}
	})
}

// copyCompilerWorkspaceForMutation copies the minimal compiler source tree needed
// to compile and run Go tests into an isolated temporary directory so real source
// mutations can be injected and verified without touching the working tree.
func copyCompilerWorkspaceForMutation(t *testing.T) string {
	t.Helper()
	dstRoot := t.TempDir()

	copyFile := func(relPath string) {
		data, err := os.ReadFile(relPath)
		if err != nil {
			t.Fatalf("mutation gate: read %s: %v", relPath, err)
		}
		dstPath := filepath.Join(dstRoot, relPath)
		if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
			t.Fatalf("mutation gate: mkdir %s: %v", filepath.Dir(dstPath), err)
		}
		if err := os.WriteFile(dstPath, data, 0o600); err != nil {
			t.Fatalf("mutation gate: write %s: %v", dstPath, err)
		}
	}

	copyDir := func(relDir string) {
		err := filepath.WalkDir(relDir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			copyFile(path)
			return nil
		})
		if err != nil {
			t.Fatalf("mutation gate: walk %s: %v", relDir, err)
		}
	}

	// Copy root Go files (excluding p10_independent_oracle_test.go to avoid recursive execution),
	// go.mod, go.sum, stdfunc.cherri, actions/, and internal/.
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("mutation gate: readdir .: %v", err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if name == "p10_independent_oracle_test.go" {
			continue
		}
		if strings.HasSuffix(name, ".go") || name == "go.mod" || name == "go.sum" || name == "stdfunc.cherri" {
			copyFile(name)
		}
	}
	copyDir("actions")
	copyDir("internal")

	return dstRoot
}

// Isolated Compiler Mutation Gate: Copies the real compiler source tree into an
// isolated temporary workspace, verifies the unmutated baseline passes, and then
// injects each of the 4 original P10 compiler bugs into the actual source files
// one at a time, asserting that `go test` fails on the mutated compiler.
func TestP10_IsolatedCompilerMutationGate(t *testing.T) {
	mutDir := copyCompilerWorkspaceForMutation(t)

	runMutatedTests := func(testPattern string) (string, error) {
		cmd := exec.Command("go", "test", "-run", testPattern, ".")
		cmd.Dir = mutDir
		out, err := cmd.CombinedOutput()
		return string(out), err
	}

	// 0. Baseline verification: unmutated copy must pass all P10 bugfix tests
	baselinePattern := "^(TestP10_RunActionPreservesWFInput|TestP10_StatisticActionEmitsWFInputNotInput|TestP10_ArrayEqualityComparison_FailsClosed|TestP10_RegexQuantifiersPreserved|TestP10_JSONBracesInStringNotParsedAsInlineVars)$"
	if out, err := runMutatedTests(baselinePattern); err != nil {
		t.Fatalf("mutation gate baseline failed on unmutated source copy: %v\n%s", err, out)
	}

	mutateAndVerifyFailure := func(
		subtestName string,
		relFile string,
		target string,
		replacement string,
		testPattern string,
		expectedFailureSubstring string,
	) {
		t.Run(subtestName, func(t *testing.T) {
			filePath := filepath.Join(mutDir, relFile)
			origBytes, err := os.ReadFile(filePath)
			if err != nil {
				t.Fatalf("read %s: %v", filePath, err)
			}
			origContent := string(origBytes)
			if !strings.Contains(origContent, target) {
				t.Fatalf("mutation target snippet not found in %s:\n%s", relFile, target)
			}

			mutatedContent := strings.Replace(origContent, target, replacement, 1)
			if err := os.WriteFile(filePath, []byte(mutatedContent), 0o600); err != nil {
				t.Fatalf("write mutated %s: %v", filePath, err)
			}
			defer func() {
				_ = os.WriteFile(filePath, origBytes, 0o600)
			}()

			out, testErr := runMutatedTests(testPattern)
			if testErr == nil {
				t.Fatalf("MUTATION GATE ESCAPE (%s): go test -run %s PASSED when %s was mutated!\nOutput:\n%s",
					subtestName, testPattern, relFile, out)
			}
			if !strings.Contains(out, expectedFailureSubstring) {
				t.Fatalf("mutation %s failed as expected, but output did not contain %q:\n%s",
					subtestName, expectedFailureSubstring, out)
			}
			t.Logf("Mutation %s caught by test suite as required (%s)", subtestName, expectedFailureSubstring)
		})
	}

	// 1. Real Compiler Mutation 1: Revert `run` in actions_std.go from `appendParamsFunc` to `makeParams`
	// (the original bug that dropped WFInput when a payload was passed to `run`).
	mutateAndVerifyFailure(
		"mutation-1-run-makeParams-drops-WFInput",
		"actions_std.go",
		"appendParamsFunc: func(args []actionArgument) map[string]any {\n\t\t\tif len(args) == 0 {\n\t\t\t\treturn map[string]any{}\n\t\t\t}\n\t\t\tvar name = argumentValue(args, 0)",
		"makeParams: func(args []actionArgument) map[string]any {\n\t\t\tif len(args) == 0 {\n\t\t\t\treturn map[string]any{}\n\t\t\t}\n\t\t\tvar name = argumentValue(args, 0)",
		"^TestP10_RunActionPreservesWFInput$",
		"WFInput parameter is nil/missing in run action",
	)

	// 2. Real Compiler Mutation 2: Revert `statistic` in actions/math.cherri from `'WFInput'` to `'Input'`
	// (the original bug that emitted the wrong wire parameter key).
	mutateAndVerifyFailure(
		"mutation-2-statistic-emits-Input-instead-of-WFInput",
		filepath.Join("actions", "math.cherri"),
		"action 'statistics' statistic(statisticOperations operation: 'WFStatisticsOperation', variable input: 'WFInput')",
		"action 'statistics' statistic(statisticOperations operation: 'WFStatisticsOperation', variable input: 'Input')",
		"^TestP10_StatisticActionEmitsWFInputNotInput$",
		"statistic action missing WFInput wire key",
	)

	// 3. Real Compiler Mutation 3: Revert `checkInlineVarRegex` in parser.go to greedy `\{(.*?)...}`
	// (the original bug that treated JSON braces `{"model":"test"}` and regex quantifiers as inline variables).
	mutateAndVerifyFailure(
		"mutation-3a-parser-greedy-inline-var-regex-on-json-braces",
		"parser.go",
		"var checkInlineVarRegex = regexp.MustCompile(`\\{(@?[a-zA-Z_][a-zA-Z0-9_]*)(?:\\['(.*?)'])?(?:\\.(.*?))?}`)",
		"var checkInlineVarRegex = regexp.MustCompile(`\\{(.*?)(?:\\['(.*?)'])?(?:\\.(.*?))?}`)",
		"^TestP10_JSONBracesInStringNotParsedAsInlineVars$",
		"Undefined inline reference",
	)

	// 3b. Real Compiler Mutation 3b: Revert `collectInlineVarRegex` and `replaceVarRegex` in shortcutgen.go
	// so regex quantifiers `{8}` are replaced with ObjectReplaceChar (\uFFFC).
	mutateAndVerifyFailure(
		"mutation-3b-shortcutgen-greedy-inline-var-regex-on-quantifiers",
		"shortcutgen.go",
		"var collectInlineVarRegex = regexp.MustCompile(`\\{@?([a-zA-Z_][a-zA-Z0-9_]*)(?:\\['(.*?)'])?(?:\\.(.*?))?}`)\nvar replaceVarRegex = regexp.MustCompile(`\\{@?[a-zA-Z_][a-zA-Z0-9_]*(?:\\['.*?'])?(?:\\..*?)?}`)",
		"var collectInlineVarRegex = regexp.MustCompile(`\\{@?(.*?)(?:\\['(.*?)'])?(?:\\.(.*?))?}`)\nvar replaceVarRegex = regexp.MustCompile(`(\\{@?.*?})`)",
		"^TestP10_RegexQuantifiersPreserved$",
		"regex WFTextTokenString struct corrupted",
	)

	// 4. Real Compiler Mutation 4: Disable the array equality check in internal/language/lower/lower.go
	// (the original bug that allowed `if matches == []` to compile instead of failing closed).
	mutateAndVerifyFailure(
		"mutation-4-lower-array-equality-conditional-accepted",
		filepath.Join("internal", "language", "lower", "lower.go"),
		"func (l *Lowerer) isArrayExpr(expr syntax.Expression) bool {",
		"func (l *Lowerer) isArrayExpr(expr syntax.Expression) bool {\n\treturn false",
		"^TestP10_ArrayEqualityComparison_FailsClosed$",
		"expected compilation to fail for array == [] comparison, but succeeded",
	)
}
