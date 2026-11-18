package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/schema"
	"howett.net/plist"
)

type p15CatalogDelta struct {
	Field  string `json:"field"`
	Before any    `json:"before"`
	After  any    `json:"after"`
}

type p15CatalogParameterMigration struct {
	BaselineIndex int              `json:"baselineIndex"`
	CurrentIndex  int              `json:"currentIndex"`
	Before        catalogParameter `json:"before"`
	After         catalogParameter `json:"after"`
	Disposition   string           `json:"disposition"`
}

type p15CatalogMigration struct {
	Name                     string                         `json:"name"`
	AppleIdentifier          string                         `json:"appleIdentifier"`
	CompilerConstruct        bool                           `json:"compilerConstruct"`
	BaselineDefinitionSHA256 string                         `json:"baselineDefinitionSha256,omitempty"`
	CurrentDefinitionSHA256  string                         `json:"currentDefinitionSha256"`
	Disposition              string                         `json:"disposition"`
	Changes                  []p15CatalogDelta              `json:"changes,omitempty"`
	Parameters               []p15CatalogParameterMigration `json:"parameterDispositions,omitempty"`
	BaselineAmbiguousLabels  []string                       `json:"baselineAmbiguousLabels,omitempty"`
	CurrentAmbiguousLabels   []string                       `json:"currentAmbiguousLabels,omitempty"`
}

type p15CatalogAliasGroup struct {
	AppleIdentifier string   `json:"appleIdentifier"`
	BaselineNames   []string `json:"baselineNames"`
	CurrentNames    []string `json:"currentNames"`
}

type p15CatalogNativeWitness struct {
	Dialect                 string           `json:"dialect"`
	Source                  string           `json:"source"`
	SourceSHA256            string           `json:"sourceSha256"`
	PlistSHA256             string           `json:"plistSha256"`
	Representation          string           `json:"representation"`
	NativeActionCount       int              `json:"nativeActionCount"`
	NativeActionIdentifiers []string         `json:"nativeActionIdentifiers"`
	TargetActionPayloads    []ShortcutAction `json:"targetActionPayloads"`
}

type p15CatalogExecution struct {
	p15CatalogMigration
	Legacy                    p15CatalogNativeWitness `json:"legacy"`
	V2                        p15CatalogNativeWitness `json:"v2"`
	ParameterContractVerified bool                    `json:"parameterContractVerified"`
	DecompileVerified         bool                    `json:"decompileVerified"`
	RoundtripVerified         bool                    `json:"roundtripVerified"`
	RoundtripComparison       string                  `json:"roundtripComparison"`
}

func p15CatalogDigest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func p15CatalogJSON(value any) []byte {
	data, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return data
}

func p15CurrentCatalog(t *testing.T) ([]catalogActionInfo, *schema.Registry) {
	t.Helper()
	resetCompilerStateFully()
	schema.ResetDefaultRegistry()
	relativePath = ""
	resetParser()
	markBuiltins()
	defineRawAction()
	loadStandardActions()
	t.Cleanup(func() { resetCompilerStateFully(); schema.ResetDefaultRegistry() })
	return buildActionCatalog(), schema.DefaultRegistry()
}

func p15BaselineCatalog(t *testing.T) ([]catalogActionInfo, []byte) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("docs", "language-v2", "baseline-catalog.json"))
	if err != nil {
		t.Fatal(err)
	}
	var baseline actionCatalogResponse
	if err := json.Unmarshal(data, &baseline); err != nil {
		t.Fatal(err)
	}
	if !baseline.OK || baseline.Count != len(baseline.Actions) || len(baseline.Actions) == 0 {
		t.Fatal("archived baseline envelope/count is inconsistent")
	}
	return baseline.Actions, data
}

func p15CatalogByName(entries []catalogActionInfo) (map[string]catalogActionInfo, error) {
	byName := make(map[string]catalogActionInfo, len(entries))
	for _, entry := range entries {
		if entry.Name == "" || entry.ShortcutIdentifier == "" {
			return nil, fmt.Errorf("definition missing name/native identity")
		}
		if _, duplicate := byName[entry.Name]; duplicate {
			return nil, fmt.Errorf("duplicate definition identity %q", entry.Name)
		}
		for _, p := range entry.Parameters {
			if p.Name == "" {
				return nil, fmt.Errorf("%s has empty parameter label", entry.Name)
			}
		}
		byName[entry.Name] = entry
	}
	return byName, nil
}

func p15CatalogChanges(before, after catalogActionInfo) []p15CatalogDelta {
	var a, b map[string]any
	_ = json.Unmarshal(p15CatalogJSON(before), &a)
	_ = json.Unmarshal(p15CatalogJSON(after), &b)
	fields := map[string]bool{}
	for key := range a {
		fields[key] = true
	}
	for key := range b {
		fields[key] = true
	}
	names := make([]string, 0, len(fields))
	for key := range fields {
		names = append(names, key)
	}
	sort.Strings(names)
	var changes []p15CatalogDelta
	for _, key := range names {
		if !reflect.DeepEqual(a[key], b[key]) {
			changes = append(changes, p15CatalogDelta{key, a[key], b[key]})
		}
	}
	return changes
}

// Old catalogs contain duplicate labels with different native wire keys.
// Match each occurrence once, with exact label+wire identity first, then an
// explicit recorded rename/wire refinement. Never collapse them into a map.
func p15ReconcileCatalogParameters(before, after []catalogParameter) ([]p15CatalogParameterMigration, error) {
	used := make([]bool, len(after))
	result := make([]p15CatalogParameterMigration, 0, len(before))
	for oldIndex, previous := range before {
		match := -1
		for priority := 0; priority < 3 && match < 0; priority++ {
			for newIndex, current := range after {
				if used[newIndex] {
					continue
				}
				compatible := priority == 0 && previous.Name == current.Name && previous.Key == current.Key ||
					priority == 1 && previous.Key != "" && previous.Key == current.Key ||
					priority == 2 && previous.Name == current.Name
				if compatible {
					match = newIndex
					break
				}
			}
		}
		if match < 0 {
			return nil, fmt.Errorf("occurrence %d (%s / %s) was dropped", oldIndex, previous.Name, previous.Key)
		}
		used[match] = true
		disposition := "RETAINED_UNCHANGED"
		if !reflect.DeepEqual(previous, after[match]) {
			disposition = "RETAINED_WITH_DECLARED_CONTRACT_CHANGE"
		}
		result = append(result, p15CatalogParameterMigration{oldIndex, match, previous, after[match], disposition})
	}
	return result, nil
}

func p15CatalogAmbiguousLabels(parameters []catalogParameter) []string {
	counts := map[string]int{}
	for _, p := range parameters {
		counts[p.Name]++
	}
	result := []string{}
	for label, count := range counts {
		if count > 1 {
			result = append(result, label)
		}
	}
	sort.Strings(result)
	return result
}

// Membership and native identity are reconciled before any count is reported.
// A same-count substitution, dropped alias, or construct reclassification fails.
func p15ReconcileCatalogs(baseline, current []catalogActionInfo, reg *schema.Registry) ([]p15CatalogMigration, []p15CatalogAliasGroup, error) {
	old, err := p15CatalogByName(baseline)
	if err != nil {
		return nil, nil, err
	}
	now, err := p15CatalogByName(current)
	if err != nil {
		return nil, nil, err
	}
	for _, previous := range baseline {
		entry, found := now[previous.Name]
		if !found {
			return nil, nil, fmt.Errorf("baseline identity %s was dropped", previous.Name)
		}
		if entry.ShortcutIdentifier != previous.ShortcutIdentifier {
			return nil, nil, fmt.Errorf("baseline identity %s was retargeted", entry.Name)
		}
		if entry.CompilerConstruct != previous.CompilerConstruct {
			return nil, nil, fmt.Errorf("baseline construct classification changed for %s", entry.Name)
		}
		if _, err := p15ReconcileCatalogParameters(previous.Parameters, entry.Parameters); err != nil {
			return nil, nil, fmt.Errorf("baseline parameter %s: %w", entry.Name, err)
		}
	}
	registered := map[string]*schema.ActionSchema{}
	for _, action := range reg.AllActions() {
		if action == nil || action.CallableName == "" || registered[action.CallableName] != nil {
			return nil, nil, fmt.Errorf("schema contains duplicate/empty definition")
		}
		registered[action.CallableName] = action
	}
	for _, entry := range current {
		action := registered[entry.Name]
		if action == nil || action.AppleIdentifier != entry.ShortcutIdentifier || action.CompilerConstruct != entry.CompilerConstruct {
			return nil, nil, fmt.Errorf("schema/catalog identity mismatch for %s", entry.Name)
		}
		delete(registered, entry.Name)
	}
	if len(registered) != 0 {
		return nil, nil, fmt.Errorf("schema has definitions missing from the live declaration catalog")
	}

	rows := make([]p15CatalogMigration, 0, len(current))
	for _, entry := range current {
		row := p15CatalogMigration{Name: entry.Name, AppleIdentifier: entry.ShortcutIdentifier,
			CompilerConstruct: entry.CompilerConstruct, CurrentDefinitionSHA256: p15CatalogDigest(p15CatalogJSON(entry)),
			Disposition: "ADDED_AFTER_ARCHIVED_BASELINE"}
		if previous, found := old[entry.Name]; found {
			row.BaselineDefinitionSHA256 = p15CatalogDigest(p15CatalogJSON(previous))
			row.Changes = p15CatalogChanges(previous, entry)
			row.Parameters, _ = p15ReconcileCatalogParameters(previous.Parameters, entry.Parameters)
			row.BaselineAmbiguousLabels = p15CatalogAmbiguousLabels(previous.Parameters)
			row.CurrentAmbiguousLabels = p15CatalogAmbiguousLabels(entry.Parameters)
			row.Disposition = "RETAINED_UNCHANGED"
			if len(row.Changes) != 0 {
				row.Disposition = "RETAINED_IDENTITY_WITH_DECLARED_METADATA_CHANGES"
			}
		}
		rows = append(rows, row)
	}
	oldGroups, newGroups := map[string][]string{}, map[string][]string{}
	for _, entry := range baseline {
		oldGroups[entry.ShortcutIdentifier] = append(oldGroups[entry.ShortcutIdentifier], entry.Name)
	}
	for _, entry := range current {
		newGroups[entry.ShortcutIdentifier] = append(newGroups[entry.ShortcutIdentifier], entry.Name)
	}
	identifiers := []string{}
	for id, names := range newGroups {
		if len(names) > 1 || len(oldGroups[id]) > 1 {
			identifiers = append(identifiers, id)
		}
	}
	sort.Strings(identifiers)
	groups := make([]p15CatalogAliasGroup, 0, len(identifiers))
	for _, id := range identifiers {
		sort.Strings(oldGroups[id])
		sort.Strings(newGroups[id])
		groups = append(groups, p15CatalogAliasGroup{id, oldGroups[id], newGroups[id]})
	}
	return rows, groups, nil
}

func p15CaptureCatalogNative(entry catalogActionInfo, dialect, source, expectedSource string, data []byte) (p15CatalogNativeWitness, error) {
	witness := p15CatalogNativeWitness{Dialect: dialect, Source: source,
		SourceSHA256: p15CatalogDigest([]byte(source)), PlistSHA256: p15CatalogDigest(data)}
	if source != expectedSource {
		return witness, fmt.Errorf("%s %s source is not the named canonical fixture", entry.Name, dialect)
	}
	if !entry.CompilerConstruct && !strings.Contains(source, entry.Name+"(") {
		return witness, fmt.Errorf("%s fixture did not invoke the actual wrapper", entry.Name)
	}
	witness.Representation = "NAMED_CALLABLE"
	if entry.CompilerConstruct {
		witness.Representation = "LANGUAGE_CONSTRUCT"
	}
	if strings.Contains(source, "rawAction(") {
		if entry.Name != "rawAction" && !(entry.CompilerConstruct && entry.Name == "calculateExpression" && dialect == "v2") {
			return witness, fmt.Errorf("%s silently substituted raw transport for a named wrapper", entry.Name)
		}
		witness.Representation = "EXPLICIT_RAW_TRANSPORT_CONTROL"
	}
	var doc Shortcut
	if _, err := plist.Unmarshal(data, &doc); err != nil {
		return witness, fmt.Errorf("%s native plist: %w", entry.Name, err)
	}
	witness.NativeActionCount = len(doc.WFWorkflowActions)
	target := entry.ShortcutIdentifier
	if entry.Name == "rawAction" {
		target = "is.workflow.actions.custom.p10test"
	}
	for _, action := range doc.WFWorkflowActions {
		witness.NativeActionIdentifiers = append(witness.NativeActionIdentifiers, action.WFWorkflowActionIdentifier)
		if action.WFWorkflowActionIdentifier == target {
			witness.TargetActionPayloads = append(witness.TargetActionPayloads, action)
		}
	}
	if len(witness.TargetActionPayloads) == 0 {
		return witness, fmt.Errorf("%s %s emitted no required native target %s", entry.Name, dialect, target)
	}
	return witness, nil
}

func p15CheckCatalogExecution(entry catalogActionInfo, evidence ActionExecutionEvidence, captures map[string]p15CatalogNativeWitness) error {
	if evidence.Name != entry.Name || evidence.ShortcutIdentifier != entry.ShortcutIdentifier || evidence.CompilerConstruct != entry.CompilerConstruct {
		return fmt.Errorf("execution identity does not match definition")
	}
	for _, dialect := range []string{"legacy", "v2"} {
		witness, ok := captures[dialect]
		if !ok || witness.Dialect != dialect || witness.NativeActionCount == 0 || len(witness.TargetActionPayloads) == 0 {
			return fmt.Errorf("missing actual %s native output capture", dialect)
		}
	}
	if !evidence.LegacySupported || !evidence.V2Supported || !evidence.PlistIdentifierVerified ||
		!evidence.ParameterContractVerified || !evidence.DecompileSupported ||
		!evidence.RoundTripVerified || !evidence.StructuralRoundTripPass {
		return fmt.Errorf("local execution incomplete: legacy=%s; v2=%s; parameter=%s; decompile=%s; roundtrip=%s",
			evidence.LegacyError, evidence.V2Error, evidence.ParameterContractError, evidence.DecompileError, evidence.RoundTripError)
	}
	return nil
}

func TestP15B02AC01CatalogMigrationAndNativeOutputs(t *testing.T) {
	catalog, reg := p15CurrentCatalog(t)
	baseline, baselineBytes := p15BaselineCatalog(t)
	rows, groups, err := p15ReconcileCatalogs(baseline, catalog, reg)
	if err != nil {
		t.Fatal(err)
	}
	sourceSHA, shaErr := exec.Command("git", "rev-parse", "HEAD").Output()
	state, stateErr := exec.Command("git", "status", "--porcelain", "--untracked-files=normal").Output()
	if shaErr != nil || stateErr != nil {
		t.Fatal("source identity could not be captured")
	}
	identities := map[string][]string{}
	for _, entry := range catalog {
		identities[entry.ShortcutIdentifier] = append(identities[entry.ShortcutIdentifier], entry.Name)
	}
	categories := buildActionIncludeCategoryMap()
	executions := make([]p15CatalogExecution, 0, len(catalog))
	retained, additions, changed, constructs := 0, 0, 0, 0
	capturedOutputs, verifiedRoundtrips := 0, 0
	for i, entry := range catalog {
		captures := map[string]p15CatalogNativeWitness{}
		var captureErr error
		evidence := executeSingleCatalogEntry(entry, reg, identities, categories, nil,
			func(dialect, source string, data []byte) {
				expected := buildLegacySourceForEntry(entry, categories)
				if dialect == "v2" {
					expected = buildV2SourceForEntry(entry, reg)
				}
				capture, err := p15CaptureCatalogNative(entry, dialect, source, expected, data)
				if err != nil {
					captureErr = err
					return
				}
				if _, duplicate := captures[dialect]; duplicate {
					captureErr = fmt.Errorf("duplicate %s capture", dialect)
					return
				}
				captures[dialect] = capture
			})
		if captureErr != nil {
			t.Errorf("%s native capture: %v", entry.Name, captureErr)
		}
		if err := p15CheckCatalogExecution(entry, evidence, captures); err != nil {
			t.Errorf("%s: %v", entry.Name, err)
		}
		capturedOutputs += len(captures)
		if evidence.StructuralRoundTripPass {
			verifiedRoundtrips++
		}
		row := rows[i]
		if row.BaselineDefinitionSHA256 == "" {
			additions++
		} else {
			retained++
		}
		if len(row.Changes) > 0 {
			changed++
		}
		if entry.CompilerConstruct {
			constructs++
		}
		executions = append(executions, p15CatalogExecution{row, captures["legacy"], captures["v2"],
			evidence.ParameterContractVerified, evidence.DecompileSupported, evidence.StructuralRoundTripPass, evidence.ComparisonModeUsed})
	}
	status := "PASS_LOCAL_CATALOG_RECONCILIATION"
	if t.Failed() {
		status = "FAIL_LOCAL_CATALOG_RECONCILIATION"
	}
	report := map[string]any{
		"schemaVersion": 1, "status": status,
		"sourceCommit": strings.TrimSpace(string(sourceSHA)), "sourceTreeCleanAtStart": len(state) == 0,
		"baselineCatalogPath": "docs/language-v2/baseline-catalog.json", "baselineCatalogSha256": p15CatalogDigest(baselineBytes),
		"schemaFingerprint": reg.Fingerprint(), "compilerMacroFingerprint": reg.CompilerMacroFingerprint(),
		"counts": map[string]int{"archivedDefinitions": len(baseline), "currentDefinitions": len(catalog),
			"retainedIdentities": retained, "newDefinitions": additions, "changedMetadataRows": changed,
			"constructs": constructs, "currentSharedNativeIdentityGroups": len(groups),
			"compilerMacrosOutsideNativeCatalog": len(reg.CompilerMacros()), "capturedNativeOutputs": capturedOutputs, "verifiedRoundtrips": verifiedRoundtrips},
		"scope": "Per-definition local capture, identity reconciliation, named compilation, parameter contract, decompile and native-action roundtrip. Shared-native-ID variants are separate wrappers, not every optional parameter combination.",
		"limits": []string{"Archived catalog contains no historical native plist corpus; historical native-byte parity is not asserted.",
			"Metadata changes are recorded explicitly, not a claim that every old source signature remains valid.",
			"calculateExpression V2 historical wire control uses explicit raw transport; separate canonical insertion syntax proves ordinary arithmetic grammar.",
			"Apple runtime, complete native parameter combinations, provider installation, Pure/Pack and release closure are outside this producer."},
		"variants": groups, "definitions": executions,
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if output := os.Getenv("CHERRI_CATALOG_ACCEPTANCE_OUTPUT"); output != "" {
		if err := os.MkdirAll(filepath.Dir(output), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(output, append(data, '\n'), 0644); err != nil {
			t.Fatal(err)
		}
	}
	t.Logf("P15_CATALOG_RECONCILIATION: source=%s clean=%t baseline=%d retained=%d current=%d added=%d changed=%d constructs=%d variantGroups=%d nativeCaptures=%d exactRoundtrips=%d reportSha256=%s",
		strings.TrimSpace(string(sourceSHA)), len(state) == 0, len(baseline), retained, len(catalog), additions, changed, constructs, len(groups), capturedOutputs, verifiedRoundtrips, p15CatalogDigest(append(data, '\n')))
}

func TestP15B02AC01RejectsCatalogIdentityFaults(t *testing.T) {
	current, reg := p15CurrentCatalog(t)
	baseline, _ := p15BaselineCatalog(t)
	index := func(name string) int {
		for i, e := range current {
			if e.Name == name {
				return i
			}
		}
		t.Fatalf("missing %s", name)
		return -1
	}
	for _, fault := range []string{"same_count_substitution", "duplicate_identity", "native_retarget", "construct_callable", "removed_parameter", "collapsed_duplicate_label", "schema_retarget"} {
		t.Run(fault, func(t *testing.T) {
			changed := append([]catalogActionInfo(nil), current...)
			changedReg := reg
			want := ""
			switch fault {
			case "same_count_substitution":
				changed[index("text")].Name = "dummyCountPadding"
				want = "baseline identity text was dropped"
			case "duplicate_identity":
				changed[index("text")] = changed[index("show")]
				want = "duplicate definition identity"
			case "native_retarget":
				changed[index("text")].ShortcutIdentifier = "is.workflow.actions.wrong"
				want = "retargeted"
			case "construct_callable":
				changed[index("conditional")].CompilerConstruct = false
				want = "construct classification changed"
			case "removed_parameter":
				changed[index("text")].Parameters = nil
				want = "baseline parameter"
			case "collapsed_duplicate_label":
				i := index("getDeviceUsage")
				changed[i].Parameters = append([]catalogParameter(nil), changed[i].Parameters[:len(changed[i].Parameters)-1]...)
				want = "baseline parameter getDeviceUsage"
			case "schema_retarget":
				schemas := append([]*schema.ActionSchema(nil), reg.AllActions()...)
				for i, a := range schemas {
					if a.CallableName == "text" {
						copy := *a
						copy.AppleIdentifier = "is.workflow.actions.wrong"
						schemas[i] = &copy
					}
				}
				changedReg = schema.NewRegistry(schemas, reg.AllEnums())
				want = "schema/catalog identity mismatch"
			}
			_, _, err := p15ReconcileCatalogs(baseline, changed, changedReg)
			if err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("fault %s did not causally reject: %v", fault, err)
			}
		})
	}
}

func TestP15B02AC01RejectsNativeCaptureFaults(t *testing.T) {
	current, reg := p15CurrentCatalog(t)
	var entry catalogActionInfo
	for _, e := range current {
		if e.Name == "text" {
			entry = e
		}
	}
	source := buildV2SourceForEntry(entry, reg)
	data, err := CompileSourceToPlist("catalog-capture.cherri", source)
	if err != nil {
		t.Fatal(err)
	}
	capture, err := p15CaptureCatalogNative(entry, "v2", source, source, data)
	if err != nil {
		t.Fatal(err)
	}
	for _, fault := range []string{"wrong_source", "malformed_plist", "retarget_native", "missing_native", "missing_capture"} {
		t.Run(fault, func(t *testing.T) {
			changed, observedSource := data, source
			switch fault {
			case "wrong_source":
				observedSource = strings.ReplaceAll(source, "text(", "show(")
			case "malformed_plist":
				changed = []byte("not a native plist")
			case "retarget_native", "missing_native":
				var doc Shortcut
				_, _ = plist.Unmarshal(data, &doc)
				if fault == "missing_native" {
					doc.WFWorkflowActions = nil
				} else {
					doc.WFWorkflowActions[0].WFWorkflowActionIdentifier = "is.workflow.actions.wrong"
				}
				changed, err = plist.Marshal(doc, plist.XMLFormat)
				if err != nil {
					t.Fatal(err)
				}
			case "missing_capture":
				evidence := ActionExecutionEvidence{Name: entry.Name, ShortcutIdentifier: entry.ShortcutIdentifier,
					LegacySupported: true, V2Supported: true, PlistIdentifierVerified: true, ParameterContractVerified: true,
					DecompileSupported: true, RoundTripVerified: true, StructuralRoundTripPass: true}
				if err := p15CheckCatalogExecution(entry, evidence, map[string]p15CatalogNativeWitness{"v2": capture}); err == nil || !strings.Contains(err.Error(), "missing actual legacy") {
					t.Fatalf("declaration-only success passed: %v", err)
				}
				return
			}
			if _, err := p15CaptureCatalogNative(entry, "v2", observedSource, source, changed); err == nil {
				t.Fatal("corrupt native capture passed")
			}
		})
	}
}

func TestP15AC01ConstructGrammarAudit(t *testing.T) {
	catalog, _ := p15CurrentCatalog(t)
	for _, entry := range catalog {
		if !entry.CompilerConstruct {
			continue
		}
		t.Run(entry.Name, func(t *testing.T) {
			source := p15FilledConstructSnippet(t, entry.Name, entry.InsertionSnippet)
			if entry.Name == "appendVariable" {
				source = "var variable = []\n" + source
			}
			if strings.TrimSpace(source) == "" || strings.Contains(source, "rawAction(") || strings.Contains(source, "native.action(") {
				t.Fatal("construct palette source is missing or replaced with raw transport")
			}
			compiled, err := CompileSourceToPlist("construct-palette.cherri", source)
			if err != nil {
				t.Fatalf("canonical construct insertion is not valid in its declared context: %v", err)
			}
			var doc Shortcut
			if _, err := plist.Unmarshal(compiled, &doc); err != nil || len(doc.WFWorkflowActions) == 0 {
				t.Fatal("construct emitted no native graph")
			}
			_, callErr := CompileSourceToPlist("construct-not-callable.cherri", fmt.Sprintf("%s()\n", entry.Name))
			if callErr == nil {
				t.Fatalf("compiler construct incorrectly accepts a zero-argument callable: %s", entry.Name)
			}
			// A missing-required-argument diagnostic would not establish the
			// syntax/construct boundary. Require the actual grammar or canonical
			// definition rejection, and separately try populated callable forms.
			if !strings.HasPrefix(callErr.Error(), "syntax error:") &&
				!strings.Contains(callErr.Error(), "canonical definition not found for action "+fmt.Sprintf("%q", entry.Name)) {
				t.Fatalf("construct rejected for an unrelated reason: %v", callErr)
			}
			for _, arguments := range []string{"Ask", "Ask, Ask"} {
				if _, err := CompileSourceToPlist("construct-populated-call.cherri", fmt.Sprintf("%s(%s)\n", entry.Name, arguments)); err == nil {
					t.Fatalf("construct accepted populated callable syntax: %s(%s)", entry.Name, arguments)
				}
			}
			ids := []string{}
			for _, action := range doc.WFWorkflowActions {
				ids = append(ids, action.WFWorkflowActionIdentifier)
			}
			t.Logf("construct=%s canonicalSyntax=%q nativeIdentifiers=%v directCallRejected=true diagnostic=%v", entry.Name, source, ids, callErr)
		})
	}
}
