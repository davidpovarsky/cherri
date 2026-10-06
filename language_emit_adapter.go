/*
 * Copyright (c) Cherri Language v2.0
 */

package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/ir"
	"github.com/electrikmilk/cherri/internal/language/lower"
	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/source"
	"github.com/electrikmilk/cherri/internal/language/syntax"
	"howett.net/plist"
)

// EmitNativeWorkflow converts NativeWorkflow IR into a concrete Shortcut struct for plist serialization.
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
	}

	if sc.WFWorkflowIcon.WFWorkflowIconGlyphNumber == 0 {
		sc.WFWorkflowIcon.WFWorkflowIconGlyphNumber = 59789
	}
	if sc.WFWorkflowIcon.WFWorkflowIconStartColor == 0 {
		sc.WFWorkflowIcon.WFWorkflowIconStartColor = 4282601983
	}
	if len(sc.WFWorkflowTypes) == 0 {
		sc.WFWorkflowTypes = []string{"Watch", "WFWorkflowTypeShowInSearch"}
	}
	if len(wf.InputContentItemClasses) > 0 {
		sc.WFWorkflowInputContentItemClasses = wf.InputContentItemClasses
	}

	for _, node := range wf.Actions {
		action := ShortcutAction{
			WFWorkflowActionIdentifier: node.AppleIdentifier,
			WFWorkflowActionParameters: make(map[string]any),
		}
		for k, v := range node.Parameters {
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
	if tok, ok := v.(*ir.AttachmentToken); ok {
		valMap := map[string]any{
			"Type": tok.Type,
		}
		if tok.OutputUUID != "" {
			valMap["OutputUUID"] = tok.OutputUUID
		}
		if tok.OutputName != "" {
			valMap["OutputName"] = tok.OutputName
		}
		if tok.Type == "Variable" {
			valMap["VariableName"] = tok.OutputName
		}
		return map[string]any{
			"Type":                tok.Type,
			"Value":               valMap,
			"WFSerializationType": "WFTextTokenAttachment",
		}
	}

	if m, ok := v.(map[string]interface{}); ok {
		res := make(map[string]any, len(m))
		for k, val := range m {
			res[k] = transformIRParamValue(val)
		}
		return res
	}

	if s, ok := v.([]interface{}); ok {
		res := make([]any, len(s))
		for i, val := range s {
			res[i] = transformIRParamValue(val)
		}
		return res
	}

	return v
}

// CompileSourceToPlist compiles Cherri v2 source code directly to unsigned plist bytes.
func CompileSourceToPlist(filePath string, content string) ([]byte, error) {
	file := source.NewFile(source.DocumentID(filePath), filePath, 1, content)
	parser := syntax.NewParser(file)
	prog := parser.ParseProgram()
	if len(parser.Errors()) > 0 {
		return nil, fmt.Errorf("syntax error: %s", parser.Errors()[0])
	}

	reg := schema.DefaultRegistry()
	analyzer := analysis.NewAnalyzer(reg)
	analyzer.Analyze(prog)
	for _, diag := range analyzer.Diagnostics() {
		if diag.Severity == analysis.SeverityError {
			return nil, fmt.Errorf("semantic error [%s]: %s at %v", diag.Code, diag.Message, diag.Span)
		}
	}

	lowerer := lower.NewLowerer(reg)
	wf, err := lowerer.LowerProgram(prog)
	if err != nil {
		return nil, fmt.Errorf("lowering error: %w", err)
	}

	sc := EmitNativeWorkflow(wf)

	var buf bytes.Buffer
	enc := plist.NewEncoder(&buf)
	enc.Indent("\t")
	if err := enc.Encode(sc); err != nil {
		return nil, fmt.Errorf("plist encoding error: %w", err)
	}

	return buf.Bytes(), nil
}

// CompileFileV2 compiles a Cherri v2 file to an unsigned or signed .shortcut file.
func CompileFileV2(filePath string, outputPath string, skipSign bool) error {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	plistBytes, err := CompileSourceToPlist(filePath, string(content))
	if err != nil {
		return err
	}

	base := strings.TrimSuffix(filePath, ".cherri")
	unsignedPath := base + "_unsigned.shortcut"
	if err := os.WriteFile(unsignedPath, plistBytes, 0644); err != nil {
		return err
	}

	if outputPath != "" && outputPath != unsignedPath {
		_ = os.WriteFile(outputPath, plistBytes, 0644)
	}

	if skipSign {
		return nil
	}

	// Sign using existing sign machinery
	inputPath = unsignedPath
	if outputPath == "" {
		outputPath = base + ".shortcut"
	}
	_, _ = plist.Unmarshal(plistBytes, &shortcut)
	basename = strings.TrimSuffix(filepath.Base(filePath), ".cherri")
	sign()
	_ = os.Remove(unsignedPath)
	return nil
}
