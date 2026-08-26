/*
 * Copyright (c) Cherri
 */

package main

import (
	"maps"
	"sync"
)

// Baseline snapshots of the process-global definition caches. Standard-action
// and user-DSL definitions accumulate into these maps for the life of the
// process (the CLI relies on that; see checkMissingStandardInclude), but the
// ACCUMULATED state leaks into later compilations' observable output: the
// decompiler decides whether to emit `#include 'actions/<cat>'` based on
// whether a category is already loaded. The snapshots are captured before any
// compilation runs so a fresh-process baseline can be restored on demand,
// mirroring resetMobileLanguageState in ios_bridge.go.
var (
	captureBaselineOnce   sync.Once
	baselineActions       map[string]*actionDefinition
	baselineEnumerations  map[string][]string
	baselineCategoriesLen int
)

func captureCompilerBaselines() {
	captureBaselineOnce.Do(func() {
		baselineActions = maps.Clone(actions)
		baselineEnumerations = maps.Clone(enumerations)
		baselineCategoriesLen = len(actionCategories)
	})
}

// init captures the fresh-process baseline before any compilation can load
// additional definitions.
func init() {
	captureCompilerBaselines()
}

// restoreCompilerDefinitions rolls the definition caches back to the captured
// fresh-process baseline so the next compile/decompile observes the same
// definition availability a new process would.
func restoreCompilerDefinitions() {
	captureCompilerBaselines()
	actions = maps.Clone(baselineActions)
	enumerations = maps.Clone(baselineEnumerations)
	actionCategories = actionCategories[:baselineCategoriesLen]
	includedStandardActions = false
	includedBasicStandardActions = false
}

// resetCompilerStateFully is resetCompilerState plus a rollback of the
// process-global definition caches. Sequential compile/decompile TESTS and
// other multi-run entry points that compare observable output should prefer
// this variant; pure in-memory scratch-state resets use resetCompilerState.
func resetCompilerStateFully() {
	resetCompilerState()
	restoreCompilerDefinitions()
}

// resetCompilerState restores every per-compilation mutable global to the
// documented baseline so sequential compilations or decompilations in one
// process cannot observe state from a previous run. It is the single
// authoritative reset helper: the CLI compiles once per process, but the iOS
// host, tests, and future batch entry points compile repeatedly.
//
// Baseline values mirror the production defaults declared next to each global
// and the values used by resetMobileLanguageState/resetMobileDecompileState:
// iconColor 3031607807, iconGlyph 61440, iosVersion 26.4,
// clientVersion versions["26.4"].
//
// Deliberately NOT touched here:
//   - immutable configuration and caches: actions/enumerations base maps,
//     static lookup maps, compiled regexps, embedded standard actions;
//   - subsystem state owned by other entry points: signing flags, package
//     manager state, ToolKit import state, mobile base snapshots;
//   - specialCharsRegex, a deterministic lazily-built cache.
func resetCompilerState() {
	// Parser source/cursor state.
	contents = ""
	originalContents = ""
	lines = []string{}
	chars = []rune{}
	char = -1
	idx = 0
	lineIdx = 0
	lineCharIdx = -1
	tokens = []token{}

	// Control flow and comments.
	controlFlowGroups = map[int]controlFlowGroup{}
	groupingIdx = 0
	isFirstCommentAction = true

	// Variables, questions, menus, functions.
	variables = map[string]varValue{}
	questions = map[string]*question{}
	menus = map[string][]varValue{}
	uuids = map[string]string{}
	functions = map[string]*function{}
	usingFunctions = false

	// Shortcut metadata definitions.
	workflowName = ""
	iconColor = 3031607807
	iconGlyph = 61440
	clientVersion = versions["26.4"]
	iosVersion = 26.4
	hasShortcutInputVariables = false
	definedWorkflowTypes = []string{}
	definedQuickActions = []string{}
	inputs = []string{}
	outputs = []string{}
	noInput = map[string]any{}
	definitions = map[string]any{}
	includedFile = false

	// Includes.
	included = []string{}
	includes = []include{}

	// Generation state.
	tabLevel = 0
	shortcut = Shortcut{}
	actionIndex = 0
	varPositions = map[string]Value{}
	inlineVariables = []inlineVariable{}
	varIndex = []attachmentVariable{}
	filterTemplates = nil
	currentOutputName = ""
	duplicateDelta = 0
	usedEnums = nil

	// Decompilation state.
	code.Reset()
	varUUIDs = nil
	constUUIDs = nil
	identifierMap = nil
	extractedReferences = nil
	references = map[string]map[string]any{}
	currentVariableValue = ""
	decompilingText = false
	decompilingDictionary = false
	macDefinition = false
	setMacDefinition = false

	// Action-processing scratch state.
	currentAction = actionReference{}
	savedAction = actionReference{}
	appIds = nil
	pasteables = nil
	repeatItemIndex = 1
	repeatIndexDepth = 1
	currentCategory = ""

	// Package-manager operation state. Safe to clear unconditionally:
	// preParse reloads the CWD manifest (and currentPkg with it) on every
	// compilation, so repeated package operations observe fresh conditions.
	currentPkg = nil
	visitedPackages = nil
}
