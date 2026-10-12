package main

import (
	"encoding/json"
	"fmt"
	"os"
)

type EmitterOwnershipRecord struct {
	File            string `json:"file"`
	Function        string `json:"function"`
	Symbol          string `json:"symbol"`
	AppleIdentifier string `json:"appleIdentifier,omitempty"`
	SourceKind      string `json:"sourceKind"` // "canonical-action", "control-flow", "function-dispatch", "binding-materialization", "native-escape", "quarantined"
	Owner           string `json:"owner"`      // "shared-core", "legacy-frontend", "v2-lowering", "native-escape"
	SharedEncoder   string `json:"sharedEncoder"`
	LegacyPath      string `json:"legacyPath"`
	V2Path          string `json:"v2Path"`
	Decision        string `json:"decision"` // "A", "B", "C", "D"
	Evidence        string `json:"evidence"`
	Tests           string `json:"tests"`
	Status          string `json:"status"` // "ACTIVE", "TARGET_CONSOLIDATION", "QUARANTINED"
}

func generateOwnership() error {
	records := []EmitterOwnershipRecord{
		{
			File:            "shortcutgen.go",
			Function:        "makeAction",
			Symbol:          "makeAction",
			AppleIdentifier: "dynamic (from actionDefinition)",
			SourceKind:      "canonical-action",
			Owner:           "shared-core",
			SharedEncoder:   "makeActionParams / actionDefinition",
			LegacyPath:      "shortcutgen.go:makeAction",
			V2Path:          "language_backend_adapter.go:EmitResolvedCall -> getActionParameters",
			Decision:        "A",
			Evidence:        "Standard Cherri action definition catalog and execution tests",
			Tests:           "TestCherriNoSign, TestActionParityMatrix, CORE-001..CORE-007",
			Status:          "ACTIVE",
		},
		{
			File:            "language_backend_adapter.go",
			Function:        "EmitResolvedCall",
			Symbol:          "CanonicalBackendSession.EmitResolvedCall",
			AppleIdentifier: "dynamic (from actionDefinition / ResolvedCall)",
			SourceKind:      "canonical-action",
			Owner:           "shared-core",
			SharedEncoder:   "lookupCanonicalAction -> checkAction -> getActionParameters",
			LegacyPath:      "shortcutgen.go:makeAction",
			V2Path:          "lower.go:lowerCall -> EmitResolvedCall",
			Decision:        "A",
			Evidence:        "Canonical definition resolution for named v2 calls",
			Tests:           "TestCompilerEndToEndParityMatrix, CORE-001..CORE-007",
			Status:          "ACTIVE",
		},
		{
			File:            "shared_codec.go",
			Function:        "EncodeReferenceValue",
			Symbol:          "EncodeReferenceValue",
			AppleIdentifier: "WFTextTokenAttachment / Value",
			SourceKind:      "value-codec",
			Owner:           "shared-core",
			SharedEncoder:   "EncodeReferenceValue / ReferenceDescriptor",
			LegacyPath:      "shortcutgen.go:attachmentValues / variableValue",
			V2Path:          "shared_codec.go:EncodeReferenceValue",
			Decision:        "B",
			Evidence:        "Shared pure reference and aggrandizement descriptor",
			Tests:           "TestCompilerEndToEndParityMatrix, CODE-001..CODE-010",
			Status:          "ACTIVE",
		},
		{
			File:            "shared_codec.go",
			Function:        "EncodeTextSegmentsShared",
			Symbol:          "EncodeTextSegmentsShared",
			AppleIdentifier: "WFTextTokenString",
			SourceKind:      "value-codec",
			Owner:           "shared-core",
			SharedEncoder:   "EncodeTextSegmentsShared (UTF-16 ranges)",
			LegacyPath:      "shortcutgen.go:attachmentValues",
			V2Path:          "shared_codec.go:EncodeTextSegmentsShared",
			Decision:        "B",
			Evidence:        "Deterministic UTF-16 code unit offset attachment serialization",
			Tests:           "TestForkActionRoundTrips/unicode, CODE-001, CODE-002",
			Status:          "ACTIVE",
		},
		{
			File:            "shared_codec.go",
			Function:        "EncodeDictionaryItemShared",
			Symbol:          "EncodeDictionaryItemShared",
			AppleIdentifier: "WFDictionaryFieldValue",
			SourceKind:      "value-codec",
			Owner:           "shared-core",
			SharedEncoder:   "makeDictionaryItem / EncodeDictionaryItemShared",
			LegacyPath:      "shortcutgen.go:makeDictionary",
			V2Path:          "shared_codec.go:EncodeDictionaryItemShared",
			Decision:        "B",
			Evidence:        "Apple-compliant WFDictionaryFieldValueItems encoding",
			Tests:           "CODE-003..CODE-008",
			Status:          "ACTIVE",
		},
		{
			File:            "shortcutgen.go",
			Function:        "makeConditionalAction",
			Symbol:          "makeConditionalAction",
			AppleIdentifier: "is.workflow.actions.conditional",
			SourceKind:      "control-flow",
			Owner:           "shared-core",
			SharedEncoder:   "shared control flow builder (extracted)",
			LegacyPath:      "shortcutgen.go:makeConditionalAction",
			V2Path:          "lower.go:lowerIfWithOutput -> shared control builder",
			Decision:        "B",
			Evidence:        "WFControlFlowMode 0/1/2, WFConditions/WFCondition, GroupingIdentifier",
			Tests:           "CTRL-001, CTRL-002",
			Status:          "TARGET_CONSOLIDATION",
		},
		{
			File:            "shortcutgen.go",
			Function:        "makeMenuAction / makeMenuItemAction",
			Symbol:          "makeMenuAction",
			AppleIdentifier: "is.workflow.actions.choosefrommenu",
			SourceKind:      "control-flow",
			Owner:           "shared-core",
			SharedEncoder:   "shared menu builder (extracted)",
			LegacyPath:      "shortcutgen.go:makeMenuAction",
			V2Path:          "lower.go:lowerMenuWithOutput -> shared menu builder",
			Decision:        "B",
			Evidence:        "WFMenuItems as typed WFMenuItem, AttributedTitle, GroupingIdentifier",
			Tests:           "CTRL-005",
			Status:          "TARGET_CONSOLIDATION",
		},
		{
			File:            "shortcutgen.go",
			Function:        "makeRepeatAction / makeRepeatEachAction",
			Symbol:          "makeRepeatAction",
			AppleIdentifier: "is.workflow.actions.repeat.count / repeat.each",
			SourceKind:      "control-flow",
			Owner:           "shared-core",
			SharedEncoder:   "shared repeat builder (extracted)",
			LegacyPath:      "shortcutgen.go:makeRepeatAction / makeRepeatEachAction",
			V2Path:          "lower.go:lowerRepeatWithOutput / lowerForWithOutput -> shared builder",
			Decision:        "B",
			Evidence:        "GroupingIdentifier, RepeatCount, RepeatEach item/index",
			Tests:           "CTRL-003, CTRL-004",
			Status:          "TARGET_CONSOLIDATION",
		},
		{
			File:            "language_backend_adapter.go",
			Function:        "BeginControl / EndControl",
			Symbol:          "CanonicalBackendSession.BeginControl",
			AppleIdentifier: "is.workflow.actions.conditional/repeat.count/repeat.each/choosefrommenu",
			SourceKind:      "control-flow",
			Owner:           "shared-core",
			SharedEncoder:   "shared control builder",
			LegacyPath:      "shortcutgen.go",
			V2Path:          "language_backend_adapter.go:BeginControl",
			Decision:        "B",
			Evidence:        "Consolidated backend session control flow segment emission",
			Tests:           "CTRL-001..CTRL-006",
			Status:          "TARGET_CONSOLIDATION",
		},
		{
			File:            "internal/language/lower/lower.go",
			Function:        "lowerProgramWithFunctions",
			Symbol:          "Lowerer.lowerProgramWithFunctions",
			AppleIdentifier: "is.workflow.actions.dictionary, runworkflow, output, exit",
			SourceKind:      "function-dispatch",
			Owner:           "v2-lowering",
			SharedEncoder:   "v2 function dispatch envelope + shared canonical action emitters",
			LegacyPath:      "functions.go (legacy function injection)",
			V2Path:          "lower.go:lowerProgramWithFunctions",
			Decision:        "C",
			Evidence:        "Typed named/default parameter function dispatch and recursion",
			Tests:           "FUNC-001..FUNC-004",
			Status:          "ACTIVE",
		},
		{
			File:            "internal/language/lower/lower.go",
			Function:        "lowerCall",
			Symbol:          "Lowerer.lowerCall (native escape branch)",
			AppleIdentifier: "user-specified",
			SourceKind:      "native-escape",
			Owner:           "native-escape",
			SharedEncoder:   "EmitRawAction / NativeActionNode",
			LegacyPath:      "raw_actions.go:makeRawAction",
			V2Path:          "lower.go:lowerCall (native.action/action/rawAction)",
			Decision:        "D",
			Evidence:        "Explicit lossless escape for unknown Apple payloads and proprietary extensions",
			Tests:           "TestForkActionRoundTrips/rawAction, LANG-003, DECOMP-002",
			Status:          "ACTIVE",
		},
		{
			File:            "internal/language/lower/lower.go",
			Function:        "lowerBinding",
			Symbol:          "Lowerer.lowerBinding",
			AppleIdentifier: "is.workflow.actions.setvariable, dictionary, gettext",
			SourceKind:      "binding-materialization",
			Owner:           "v2-lowering",
			SharedEncoder:   "Shared dictionary/text encoders + SetVariable",
			LegacyPath:      "shortcutgen.go:makeVariableValueAction",
			V2Path:          "lower.go:lowerBinding",
			Decision:        "B",
			Evidence:        "Dictionary literals for let/var emitted as native Dictionary action",
			Tests:           "CODE-003, CODE-004",
			Status:          "TARGET_CONSOLIDATION",
		},
		{
			File:            "language_backend_adapter.go",
			Function:        "NewCanonicalBackendSession / SetMetadata",
			Symbol:          "CanonicalBackendSession.SetMetadata",
			AppleIdentifier: "WorkflowHeader",
			SourceKind:      "workflow-header",
			Owner:           "shared-core",
			SharedEncoder:   "shared workflow header initializer (extracted)",
			LegacyPath:      "shortcutgen.go:generateShortcut",
			V2Path:          "language_backend_adapter.go:NewCanonicalBackendSession -> SetMetadata",
			Decision:        "B",
			Evidence:        "Unification of client version, QuickActions, input/output classes, questions",
			Tests:           "META-001..META-003",
			Status:          "TARGET_CONSOLIDATION",
		},
		{
			File:            "language_emit_adapter.go",
			Function:        "EmitNativeWorkflow",
			Symbol:          "EmitNativeWorkflow",
			AppleIdentifier: "dynamic",
			SourceKind:      "quarantined",
			Owner:           "quarantined",
			SharedEncoder:   "none (quarantined)",
			LegacyPath:      "none",
			V2Path:          "language_emit_adapter.go (quarantined, non-production)",
			Decision:        "B",
			Evidence:        "Preserved strictly for quarantine safety and isolation verification",
			Tests:           "language_emit_adapter_test.go",
			Status:          "QUARANTINED",
		},
	}

	data, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile("docs/architecture/action-emitter-ownership.json", data, 0644); err != nil {
		return err
	}
	fmt.Printf("Wrote docs/architecture/action-emitter-ownership.json (%d records)\n", len(records))
	return nil
}
