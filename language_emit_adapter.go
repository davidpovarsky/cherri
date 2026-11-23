/*
 * Copyright (c) Cherri Language v2.0
 */

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/electrikmilk/args-parser"
	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/ir"
	"github.com/electrikmilk/cherri/internal/language/lower"
	"github.com/electrikmilk/cherri/internal/language/migrate"
	"github.com/electrikmilk/cherri/internal/language/modules"
	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/source"
	"github.com/electrikmilk/cherri/internal/language/syntax"
	"howett.net/plist"
)

// EmitNativeWorkflow converts NativeWorkflow IR into a concrete Shortcut struct for plist serialization.
// QUARANTINED: Legacy/migration helper preserved for compatibility; unreachable from normal production CompileSourceToPlist().
func EmitNativeWorkflow(wf *ir.NativeWorkflow) Shortcut {
	sc := Shortcut{
		WFWorkflowIcon: ShortcutIcon{
			WFWorkflowIconGlyphNumber: int64(wf.IconGlyph),
			WFWorkflowIconStartColor:  wf.IconColor,
		},
		WFWorkflowClientVersion:              wf.ClientVersion,
		WFWorkflowMinimumClientVersion:       900,
		WFWorkflowMinimumClientVersionString: "900",
		WFWorkflowTypes:                      wf.WorkflowTypes,
		WFWorkflowHasShortcutInputVariables:  wf.HasShortcutInputVariables,
	}

	if sc.WFWorkflowIcon.WFWorkflowIconGlyphNumber == 0 {
		sc.WFWorkflowIcon.WFWorkflowIconGlyphNumber = 61440
	}
	if sc.WFWorkflowIcon.WFWorkflowIconStartColor == 0 {
		sc.WFWorkflowIcon.WFWorkflowIconStartColor = 3031607807
	}
	if len(sc.WFWorkflowTypes) == 0 {
		sc.WFWorkflowTypes = schema.DefaultWorkflowTypes()
	}
	if len(wf.InputContentItemClasses) > 0 {
		sc.WFWorkflowInputContentItemClasses = wf.InputContentItemClasses
	} else {
		sc.WFWorkflowInputContentItemClasses = schema.DefaultInputContentItemClasses()
	}

	for _, node := range wf.Actions {
		action := ShortcutAction{
			WFWorkflowActionIdentifier: node.AppleIdentifier,
			WFWorkflowActionParameters: make(map[string]any),
		}
		for k, v := range node.Parameters {
			if node.AppleIdentifier == "is.workflow.actions.conditional" && k == "WFInput" {
				if _, ok := v.(*ir.AttachmentToken); ok {
					action.WFWorkflowActionParameters[k] = map[string]any{
						"Type":     "Variable",
						"Variable": transformIRParamValue(v),
					}
					continue
				}
			}
			if node.AppleIdentifier == "is.workflow.actions.conditional" && k == "WFConditionalActionString" {
				if _, ok := v.(*ir.AttachmentToken); ok {
					action.WFWorkflowActionParameters[k] = map[string]any{
						"WFSerializationType": "WFTextTokenString",
						"Value": map[string]any{
							"string": "\uFFFC",
							"attachmentsByRange": map[string]any{
								"{0, 1}": transformIRParamValueContext(v, true),
							},
						},
					}
					continue
				}
			}
			action.WFWorkflowActionParameters[k] = transformIRParamValue(v)
		}
		if node.OutputUUID != "" {
			action.WFWorkflowActionParameters["UUID"] = node.OutputUUID
		}
		if node.OutputName != "" {
			action.WFWorkflowActionParameters["CustomOutputName"] = node.OutputName
		}
		if node.GroupingIdentifier != "" {
			action.WFWorkflowActionParameters["GroupingIdentifier"] = node.GroupingIdentifier
		}
		if _, hasLimit := action.WFWorkflowActionParameters["WFContentItemLimitNumber"]; hasLimit {
			action.WFWorkflowActionParameters["WFContentItemLimitEnabled"] = true
		}
		if action.WFWorkflowActionIdentifier == "is.workflow.actions.conditional" {
			if att, ok := action.WFWorkflowActionParameters["WFInput"].(WFTextTokenAttachment); ok {
				action.WFWorkflowActionParameters["WFInput"] = map[string]any{
					"Type":     "Variable",
					"Variable": att,
				}
			} else if input, ok := action.WFWorkflowActionParameters["WFInput"].(map[string]any); ok {
				if _, hasVar := input["Variable"]; !hasVar {
					action.WFWorkflowActionParameters["WFInput"] = map[string]any{
						"Type":     "Variable",
						"Variable": input,
					}
				}
			}
		}
		if action.WFWorkflowActionIdentifier == "is.workflow.actions.output" {
			if num, ok := action.WFWorkflowActionParameters["WFOutput"].(float64); ok {
				strVal := fmt.Sprintf("%v", num)
				if num == float64(int64(num)) {
					strVal = fmt.Sprintf("%d", int64(num))
				}
				action.WFWorkflowActionParameters["WFOutput"] = map[string]any{
					"WFSerializationType": "WFTextTokenString",
					"Value": map[string]any{
						"string": strVal,
					},
				}
			} else if num, ok := action.WFWorkflowActionParameters["WFOutput"].(int); ok {
				action.WFWorkflowActionParameters["WFOutput"] = map[string]any{
					"WFSerializationType": "WFTextTokenString",
					"Value": map[string]any{
						"string": fmt.Sprintf("%d", num),
					},
				}
			} else if str, ok := action.WFWorkflowActionParameters["WFOutput"].(string); ok {
				action.WFWorkflowActionParameters["WFOutput"] = map[string]any{
					"WFSerializationType": "WFTextTokenString",
					"Value": map[string]any{
						"string": str,
					},
				}
			} else if att, ok := action.WFWorkflowActionParameters["WFOutput"].(map[string]any); ok && att["WFSerializationType"] == "WFTextTokenAttachment" {
				action.WFWorkflowActionParameters["WFOutput"] = map[string]any{
					"WFSerializationType": "WFTextTokenString",
					"Value": map[string]any{
						"string": "\uFFFC",
						"attachmentsByRange": map[string]any{
							"{0, 1}": att["Value"],
						},
					},
				}
			} else if att, ok := action.WFWorkflowActionParameters["WFOutput"].(WFTextTokenAttachment); ok {
				action.WFWorkflowActionParameters["WFOutput"] = map[string]any{
					"WFSerializationType": "WFTextTokenString",
					"Value": map[string]any{
						"string": "\uFFFC",
						"attachmentsByRange": map[string]any{
							"{0, 1}": att.Value,
						},
					},
				}
			}
		}
		if action.WFWorkflowActionIdentifier == "is.workflow.actions.list" {
			if arr, ok := action.WFWorkflowActionParameters["WFItems"].(WFArrayValue); ok {
				action.WFWorkflowActionParameters["WFItems"] = arr.Value
			} else if m, ok := action.WFWorkflowActionParameters["WFItems"].(map[string]any); ok {
				if m["WFSerializationType"] == "WFArrayParameterState" {
					if val, exists := m["Value"]; exists {
						action.WFWorkflowActionParameters["WFItems"] = val
					}
				}
			} else if items, ok := action.WFWorkflowActionParameters["WFItems"].([]any); ok {
				var dictItems []WFDictionaryFieldValueItem
				for _, item := range items {
					if dItem, ok := item.(WFDictionaryFieldValueItem); ok {
						dictItems = append(dictItems, dItem)
					} else if m, ok := item.(map[string]any); ok && m["WFItemType"] != nil {
						itemType := 0
						if t, ok := m["WFItemType"].(int); ok {
							itemType = t
						}
						dictItems = append(dictItems, WFDictionaryFieldValueItem{
							WFItemType: itemType,
							WFValue:    m["WFValue"],
						})
					} else if m, ok := item.(map[string]interface{}); ok && m["WFItemType"] != nil {
						itemType := 0
						if t, ok := m["WFItemType"].(int); ok {
							itemType = t
						}
						dictItems = append(dictItems, WFDictionaryFieldValueItem{
							WFItemType: itemType,
							WFValue:    m["WFValue"],
						})
					} else if s, ok := item.(string); ok {
						dictItems = append(dictItems, WFDictionaryFieldValueItem{
							WFItemType: 0,
							WFValue:    s,
						})
					} else {
						dictItems = append(dictItems, makeDictionaryItem("", item))
					}
				}
				action.WFWorkflowActionParameters["WFItems"] = dictItems
			}
		}
		sc.WFWorkflowActions = append(sc.WFWorkflowActions, action)

	}

	for _, q := range wf.ImportQuestions {
		actionIndex := 0
		if idx, ok := q["ActionIndex"].(int); ok {
			actionIndex = idx
		} else if idx, ok := q["ActionIndex"].(int64); ok {
			actionIndex = int(idx)
		}
		paramKey, _ := q["ParameterKey"].(string)
		text, _ := q["Text"].(string)
		defVal, _ := q["DefaultValue"].(string)
		category, _ := q["Category"].(string)
		sc.WFWorkflowImportQuestions = append(sc.WFWorkflowImportQuestions, WFQuestion{
			ActionIndex:  actionIndex,
			ParameterKey: paramKey,
			Text:         text,
			DefaultValue: defVal,
			Category:     category,
		})
	}

	return sc
}

func transformIRParamValue(v any) any {
	return transformIRParamValueContext(v, false)
}

func transformIRParamValueContext(v any, inAttachmentsByRange bool) any {
	if raw, ok := v.(*ir.RawPlistValue); ok {
		return transformIRParamValueContext(raw.Data, inAttachmentsByRange)
	}
	if raw, ok := v.(ir.RawPlistValue); ok {
		return transformIRParamValueContext(raw.Data, inAttachmentsByRange)
	}

	if tok, ok := v.(*ir.AttachmentToken); ok {
		valMap := map[string]any{
			"Type": tok.Type,
		}
		if tok.OutputUUID != "" {
			valMap["OutputUUID"] = tok.OutputUUID
		}
		if tok.Type == "Variable" {
			valMap["VariableName"] = tok.OutputName
		} else if tok.Type == "Ask" && tok.OutputName != "" {
			valMap["Prompt"] = tok.OutputName
		} else if tok.OutputName != "" && tok.Type != "ExtensionInput" && tok.Type != "CurrentDate" && tok.Type != "Clipboard" && tok.Type != "DeviceDetails" {
			valMap["OutputName"] = tok.OutputName
		}
		if len(tok.Aggrandizements) > 0 {
			valMap["Aggrandizements"] = tok.Aggrandizements
		}
		if inAttachmentsByRange {
			return valMap
		}
		return map[string]any{
			"Value":               valMap,
			"WFSerializationType": "WFTextTokenAttachment",
		}
	}

	if m, ok := v.(map[string]interface{}); ok {
		res := make(map[string]any, len(m))
		for k, val := range m {
			res[k] = transformIRParamValueContext(val, inAttachmentsByRange || k == "attachmentsByRange")
		}
		return res
	}

	if s, ok := v.([]interface{}); ok {
		res := make([]any, len(s))
		for i, val := range s {
			res[i] = transformIRParamValueContext(val, inAttachmentsByRange)
		}
		return res
	}

	return v
}

// CompileSourceToPlist compiles Cherri v2 source code directly to unsigned plist bytes.
func CompileSourceToPlist(filePath string, content string) ([]byte, error) {
	return CompileSourceToPlistWithModules(filePath, content, nil)
}

// collectV2CompileErrorLines parses and analyzes V2 source without lowering/plist serialization
// so verify/isolation can demote all static error lines in one pass.
func collectV2CompileErrorLines(filePath string, content string) ([]int, error) {
	file := source.NewFile(source.DocumentID(filePath), filePath, 1, content)
	parser := syntax.NewParser(file)
	prog := parser.ParseProgram()
	if errs := parser.Errors(); len(errs) > 0 {
		linesOut := make([]int, 0, len(errs))
		for _, pe := range errs {
			if pe.Span.Start.Line > 0 {
				linesOut = append(linesOut, pe.Span.Start.Line)
			}
		}
		return linesOut, fmt.Errorf("syntax error: %s", errs[0])
	}
	reg := schema.DefaultRegistry()
	graph, issue := modules.Resolve(prog, filePath, nil, reg, migrate.MigrateModuleSource)
	if issue != nil {
		return []int{issue.Span.Start.Line}, issue
	}
	analyzer := analysis.NewAnalyzer(reg)
	analyzer.SetModuleGraph(graph)
	analyzer.Analyze(prog)
	var linesOut []int
	var firstErr error
	for _, diag := range analyzer.Diagnostics() {
		if diag.Severity == analysis.SeverityError {
			if firstErr == nil {
				firstErr = fmt.Errorf("semantic error [%s]: %s at %v", diag.Code, diag.Message, diag.Span)
			}
			if diag.Span.Start.Line > 0 {
				linesOut = append(linesOut, diag.Span.Start.Line)
			}
		}
	}
	return linesOut, firstErr
}

// CompileSourceToPlistWithModules compiles Cherri v2 source code with optional virtual modules.
func CompileSourceToPlistWithModules(filePath string, content string, virtualModules map[string]string) ([]byte, error) {
	return CompileSourceToPlistWithOptions(filePath, content, CompileOptions{VirtualModules: virtualModules})
}

// CompileSourceToPlistWithOptions applies an isolated build profile and module map.
func CompileSourceToPlistWithOptions(filePath string, content string, options CompileOptions) ([]byte, error) {
	return compileSourceToPlistWithSession(filePath, content, options, NewCanonicalBackendSession)
}

// The explicit session factory lets an embedded request reuse its owned native
// state lock without a global reentrancy flag or a recursive mutex acquisition.
func compileSourceToPlistWithSession(filePath string, content string, options CompileOptions, newSession func() *CanonicalBackendSession) ([]byte, error) {
	var profile, profileErr = normalizeBuildProfile(options.BuildProfile)
	if profileErr != nil {
		return nil, profileErr
	}
	file := source.NewFile(source.DocumentID(filePath), filePath, 1, content)
	parser := syntax.NewParser(file)
	prog := parser.ParseProgram()
	if len(parser.Errors()) > 0 {
		return nil, fmt.Errorf("syntax error: %s", parser.Errors()[0])
	}

	reg := schema.DefaultRegistry()
	graph, issue := modules.Resolve(prog, filePath, options.VirtualModules, reg, migrate.MigrateModuleSource)
	if issue != nil {
		return nil, issue
	}
	analyzer := analysis.NewAnalyzer(reg)
	analyzer.SetModuleGraph(graph)
	analyzer.Analyze(prog)
	for _, diag := range analyzer.Diagnostics() {
		if diag.Severity == analysis.SeverityError {
			return nil, fmt.Errorf("semantic error [%s]: %s at %v", diag.Code, diag.Message, diag.Span)
		}
	}

	session := newSession()
	defer session.Close()
	session.buildProfile = profile
	session.configureAssets(filePath, options.AssetCache)
	session.configurePlatformVersions(analyzer.Registry(), analyzer.Targets(), analyzer.MinTargetVersionString(), analyzer.MaxTargetVersionString())

	lowerer := lower.NewLowererWithSession(analyzer.Registry(), session)
	lowerer.WorkflowName = strings.TrimSuffix(filepath.Base(filePath), ".cherri")
	lowerer.SetBaseFilePath(filePath)
	lowerer.SetModuleGraph(graph)
	lowerer.SetModuleRegistries(analyzer.ModuleRegistries())
	if options.VirtualModules != nil {
		lowerer.SetVirtualModules(options.VirtualModules)
	}
	lowerer.SetExprTypes(analyzer.ExprTypes())
	lowerer.SetTypeAnnotations(analyzer.TypeAnnotations())
	lowerer.SetKnownPresentDictionaryIndexes(analyzer.KnownPresentDictionaryIndexes())
	lowerer.SetEnumValues(analyzer.EnumValues())
	if options.previewSourceMap != nil {
		lowerer.EnableVisualSourceOrigins(prog)
	}
	if err := lowerer.LowerProgramToSession(prog); err != nil {
		return nil, fmt.Errorf("lowering error: %w", err)
	}

	if options.RequireAuthoredWorkflow && session.hasRawWorkflow {
		return nil, fmt.Errorf("[CHERRI_EDIT_POLICY_REQUIRED] Native workflow provenance requires an original and explicit allowed-delta policy before signing")
	}

	plistBytes, err := session.Finalize()
	if err != nil {
		return nil, err
	}
	if options.previewSourceMap != nil {
		*options.previewSourceMap = session.finalizePreviewSourceMap(filePath, content, profile, plistBytes)
	}
	return plistBytes, nil
}

// CompileFileV2 compiles a Cherri v2 file to an unsigned or signed .shortcut file.
func CompileFileV2(filePath string, targetOutPath string, skipSign bool) error {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	var profile, profileErr = normalizeBuildProfile(BuildProfile(args.Value("build-profile")))
	if profileErr != nil {
		return profileErr
	}
	plistBytes, err := CompileSourceToPlistWithOptions(filePath, string(content), CompileOptions{BuildProfile: profile, RequireAuthoredWorkflow: args.Using("require-authored-workflow")})
	if err != nil {
		return err
	}

	base := strings.TrimSuffix(filePath, ".cherri")
	// An explicit unsigned destination is the complete output contract. Do not
	// overwrite a sibling source artifact while callers stage a verified build.
	if skipSign && targetOutPath != "" {
		return atomicWriteFile(targetOutPath, plistBytes, 0644)
	}
	if targetOutPath == "" {
		targetOutPath = base + ".shortcut"
	}
	if skipSign {
		var unsignedPath = base + "_unsigned.shortcut"
		if err := atomicWriteFile(unsignedPath, plistBytes, 0644); err != nil {
			return err
		}
		if targetOutPath != unsignedPath {
			return atomicWriteFile(targetOutPath, plistBytes, 0644)
		}
		return nil
	}

	// Signing input is private scratch data. Keep any existing sibling unsigned
	// artifact intact, and send these exact compiled bytes to the signer.
	var stagingDir, stagingErr = os.MkdirTemp(filepath.Dir(targetOutPath), "cherri-compile-*")
	if stagingErr != nil {
		return fmt.Errorf("failed to create compilation staging directory: %w", stagingErr)
	}
	defer os.RemoveAll(stagingDir)
	var unsignedPath = filepath.Join(stagingDir, filepath.Base(base)+"_unsigned.shortcut")
	if err := os.WriteFile(unsignedPath, plistBytes, 0600); err != nil {
		return err
	}

	// Prepare shortcut and basename for signing
	_, _ = plist.Unmarshal(plistBytes, &shortcut)
	basename = strings.TrimSuffix(filepath.Base(filePath), ".cherri")

	signingMode := "people-who-know-me"
	if args.Using("share") && args.Value("share") == "anyone" {
		signingMode = "anyone"
	}

	if err := SignShortcut(unsignedPath, targetOutPath, signingMode); err != nil {
		return fmt.Errorf("signing failed: %w", err)
	}
	return nil
}
