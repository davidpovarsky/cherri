package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/electrikmilk/cherri/internal/language/assets"
)

// BuildProfile controls compiler-owned diagnostics; it never removes assertions.
type BuildProfile string

const (
	BuildProfileDebug   BuildProfile = "debug"
	BuildProfileRelease BuildProfile = "release"
)

// CompileOptions is explicit per compilation so library clients do not inherit CLI state.
type CompileOptions struct {
	BuildProfile   BuildProfile
	VirtualModules map[string]string
	// AssetCache allows the editor and compiler to share verified local file bytes.
	// Nil selects the process-wide bounded cache.
	AssetCache *assets.Cache
	// RequireAuthoredWorkflow rejects native.workflow provenance before any output
	// is finalized. Edit workflows instead supply an original and exact policy.
	RequireAuthoredWorkflow bool
}

var legacyActiveBuildProfile = BuildProfileDebug

func normalizeBuildProfile(profile BuildProfile) (BuildProfile, error) {
	switch profile {
	case "", BuildProfileDebug:
		return BuildProfileDebug, nil
	case BuildProfileRelease:
		return BuildProfileRelease, nil
	default:
		return "", fmt.Errorf("BUILD_PROFILE_INVALID: expected debug or release")
	}
}

func isRemovableDiagnosticMacro(name string) bool {
	switch name {
	case "debugLog", "debugLogPrevious", "debugShowLogs", "debugFinish",
		"debugClearLogs", "debugTrace", "debugStartTimer", "debugStopTimer",
		"debugTraceEvent", "debugExportTrace", "debugExport":
		return true
	default:
		return false
	}
}

func resolveCompilerMacroCall(name string, params map[string]any, prior []ShortcutAction, outputUUID, outputName string, profile BuildProfile) ([]ShortcutAction, error) {
	var selected, err = normalizeBuildProfile(profile)
	if err != nil {
		return nil, err
	}
	if selected == BuildProfileRelease && isRemovableDiagnosticMacro(name) {
		if err := validateRemovedDiagnosticOptions(name, params); err != nil {
			return nil, err
		}
		// Arguments have already been evaluated by the caller. Only compiler
		// instrumentation is omitted; source producers and effects stay intact.
		if currentDebugInstrumentation.strippedMacros == nil {
			currentDebugInstrumentation.strippedMacros = make(map[string]bool)
		}
		if currentDebugInstrumentation.strippedOutputs == nil {
			currentDebugInstrumentation.strippedOutputs = make(map[string]string)
		}
		currentDebugInstrumentation.strippedMacros[name] = true
		if outputUUID != "" {
			currentDebugInstrumentation.strippedOutputs[strings.ToUpper(outputUUID)] = name
		}
		return nil, nil
	}
	return resolveDebugMacroCall(name, params, prior, outputUUID, outputName)
}

func validateReleaseMacroReferences(actions []ShortcutAction, profile BuildProfile) error {
	if profile != BuildProfileRelease || len(currentDebugInstrumentation.strippedMacros) == 0 {
		return nil
	}
	var data, err = json.Marshal(actions)
	if err != nil {
		return fmt.Errorf("RELEASE_REFERENCE_CHECK: %w", err)
	}
	var tree any
	if err = json.Unmarshal(data, &tree); err != nil {
		return fmt.Errorf("RELEASE_REFERENCE_CHECK: %w", err)
	}
	return inspectReleaseMacroReferences(tree, currentDebugInstrumentation.strippedOutputs)
}

func inspectReleaseMacroReferences(value any, removed map[string]string) error {
	switch node := value.(type) {
	case map[string]any:
		if node["Type"] == "ActionOutput" {
			if id, ok := node["OutputUUID"].(string); ok {
				if macro, found := removed[strings.ToUpper(id)]; found {
					return fmt.Errorf("RELEASE_DIAGNOSTIC_VALUE: source consumes the removed result of %s; use an ordinary value-producing action", macro)
				}
			}
		}
		if node["Type"] == "Variable" {
			if name, ok := node["VariableName"].(string); ok &&
				(name == debugLogVariableName || name == debugLabelVariableName || name == debugQATraceVariableName ||
					strings.HasPrefix(name, debugLabelDisplayVariablePrefix) ||
					strings.HasPrefix(name, debugTimerVariablePrefix)) {
				return fmt.Errorf("RELEASE_DIAGNOSTIC_VALUE: source consumes execution-local diagnostic state")
			}
		}
		for _, child := range node {
			if err := inspectReleaseMacroReferences(child, removed); err != nil {
				return err
			}
		}
	case []any:
		for _, child := range node {
			if err := inspectReleaseMacroReferences(child, removed); err != nil {
				return err
			}
		}
	}
	return nil
}

// Removing diagnostics does not make invalid compile-time options valid.
// This validation never binds or changes a producer UUID.
func validateRemovedDiagnosticOptions(name string, params map[string]any) error {
	switch name {
	case "debugExport":
		_, err := parseDebugExportOptions(params)
		return err
	case "debugLog", "debugLogPrevious", "debugShowLogs", "debugFinish":
		if name == "debugLog" || name == "debugLogPrevious" {
			if _, _, err := debugLiteralLogLabel(params, name); err != nil {
				return err
			}
			if value, passed := params["WFInput"]; passed && value != nil {
				return fmt.Errorf("%s() accepts only display flags and a literal label; it captures the previous native output", name)
			}
		}
		for _, key := range []string{"WFDebugWithDates", "WFDebugShowLogs", "WFDebugShowLogsAsText"} {
			if _, err := debugLiteralFlag(params, key, name); err != nil {
				return err
			}
		}
		var show, _ = debugLiteralFlag(params, "WFDebugShowLogs", name)
		var asText, _ = debugLiteralFlag(params, "WFDebugShowLogsAsText", name)
		if show && asText {
			return fmt.Errorf("%s: showLogs and showLogsAsText are alternative display modes; select only one", name)
		}
	case "debugStartTimer", "debugStopTimer":
		if _, err := debugTimerLabel(params); err != nil {
			return err
		}
	case "debugTraceEvent":
		var stage, ok = params["WFDebugTraceStage"].(string)
		if !ok {
			return fmt.Errorf("debugTraceEvent requires a non-empty literal stage (before|after|error|end)")
		}
		switch stage {
		case "before", "after", "error", "end":
		default:
			return fmt.Errorf("debugTraceEvent stage must be one of before, after, error, end; got %q", stage)
		}
	case "debugTrace":
		if value, ok := params["WFInput"]; !ok || value == nil {
			return fmt.Errorf("debugTrace(value) requires an input")
		}
	}
	return nil
}
