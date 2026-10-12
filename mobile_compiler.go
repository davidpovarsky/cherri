/*
 * Copyright (c) Cherri
 */

package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"maps"
	"strings"
	"sync"

	args "github.com/electrikmilk/args-parser"
	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/service"
	"github.com/electrikmilk/cherri/internal/language/source"
	"github.com/electrikmilk/cherri/internal/language/syntax"
	"howett.net/plist"
)

type MobileCompileMode string

const (
	MobileCompileModeV2     MobileCompileMode = "v2"
	MobileCompileModeLegacy MobileCompileMode = "legacy"
	MobileCompileModeAuto   MobileCompileMode = "auto"
)

type mobileCompileResponse struct {
	OK           bool   `json:"ok"`
	Name         string `json:"name,omitempty"`
	PlistBase64  string `json:"plistBase64,omitempty"`
	SignedBase64 string `json:"signedBase64,omitempty"`
	Source       string `json:"source,omitempty"`
	Error        string `json:"error,omitempty"`
	Line         int    `json:"line,omitempty"`
	Column       int    `json:"column,omitempty"`
	Mode         string `json:"mode,omitempty"`
}

type mobileActionParameter = catalogParameter
type mobileActionInfo = catalogActionInfo
type mobileActionCatalogResponse = actionCatalogResponse

var mobileCompileMu sync.Mutex
var mobileBaseActions map[string]*actionDefinition
var mobileBaseEnumerations map[string][]string
var mobileServiceOnce sync.Once
var mobileSharedService *service.Service

func sharedMobileService() *service.Service {
	mobileServiceOnce.Do(func() {
		mobileSharedService = service.NewService(schema.DefaultRegistry())
	})
	return mobileSharedService
}

// detectLegacySource detects whether a source file is intended for legacy Cherri.
// It checks strictly for anchored preprocessor directives (#include, #define, etc.)
// or explicit file directives. It NEVER treats substring tokens like "@", "Ask",
// "const" or braces in comments/strings as legacy markers.
func detectLegacySource(src string) bool {
	lines := strings.Split(src, "\n")
	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		if strings.HasPrefix(trimmed, "// cherri: legacy") ||
			strings.HasPrefix(trimmed, "// cherri: v1") ||
			strings.HasPrefix(trimmed, "/* cherri: legacy") {
			return true
		}
		if strings.HasPrefix(trimmed, "// cherri: v2") ||
			strings.HasPrefix(trimmed, "/* cherri: v2") {
			return false
		}
		if strings.HasPrefix(trimmed, "#include") ||
			strings.HasPrefix(trimmed, "#define") ||
			strings.HasPrefix(trimmed, "#import") ||
			strings.HasPrefix(trimmed, "#question") ||
			strings.HasPrefix(trimmed, "#ref") {
			return true
		}
	}
	return false
}

func compileForMobile(src string, requestedName string, sign bool) mobileCompileResponse {
	return compileForMobileMode(src, requestedName, string(MobileCompileModeAuto), sign)
}

func compileForMobileMode(src string, requestedName string, mode string, sign bool) (response mobileCompileResponse) {
	mobileCompileMu.Lock()
	defer mobileCompileMu.Unlock()

	embeddedCompilerMode = true
	previousArgs := args.Args
	args.Args = map[string]string{"no-ansi": ""}

	defer func() {
		embeddedCompilerMode = false
		args.Args = previousArgs
		if recovered := recover(); recovered != nil {
			response = mobileCompileResponse{
				OK:     false,
				Error:  embeddedErrorMessage(recovered),
				Line:   max(lineIdx+1, 1),
				Column: max(lineCharIdx+1, 1),
				Mode:   mode,
			}
			resetEmbeddedFailureState()
		}
	}()

	resetMobileLanguageState()
	resetMobileDecompileState()
	name := normalizedMobileName(requestedName)

	selectedMode := MobileCompileMode(strings.ToLower(strings.TrimSpace(mode)))
	if selectedMode == "" {
		selectedMode = MobileCompileModeAuto
	}

	if selectedMode == MobileCompileModeAuto {
		if detectLegacySource(src) {
			selectedMode = MobileCompileModeLegacy
		} else {
			selectedMode = MobileCompileModeV2
		}
	}

	if selectedMode == MobileCompileModeV2 {
		v2Bytes, err := CompileSourceToPlist(name+".cherri", src)
		if err == nil {
			response = mobileCompileResponse{
				OK:          true,
				Name:        name,
				PlistBase64: base64.StdEncoding.EncodeToString(v2Bytes),
				Mode:        string(MobileCompileModeV2),
			}
			if sign {
				service := hubSign()
				signedShortcut, signErr := SignShortcutBytes(&service, name, v2Bytes)
				if signErr != nil {
					response.OK = false
					response.Error = fmt.Sprintf("Signing error: %v", signErr)
					return response
				}
				if len(signedShortcut) > 0 && looksLikeSignedShortcut(signedShortcut) {
					response.SignedBase64 = base64.StdEncoding.EncodeToString(signedShortcut)
				} else {
					response.OK = false
					response.Error = "Signing server response does not look like a signed Shortcut (missing AEA1 magic)"
					return response
				}
			}
			return response
		}

		// In v2 mode, NEVER fall back to legacy compiler.
		// Locate accurate line and column for diagnostics.
		line := 1
		col := 1
		file := source.NewFile(source.DocumentID(name+".cherri"), name+".cherri", 1, src)
		p := syntax.NewParser(file)
		prog := p.ParseProgram()
		if len(p.Errors()) > 0 {
			line = p.Errors()[0].Span.Start.Line
			col = p.Errors()[0].Span.Start.Column
		} else {
			reg := schema.DefaultRegistry()
			analyzer := analysis.NewAnalyzer(reg)
			analyzer.Analyze(prog)
			for _, d := range analyzer.Diagnostics() {
				if d.Severity == analysis.SeverityError {
					line = d.Span.Start.Line
					col = d.Span.Start.Column
					break
				}
			}
		}

		return mobileCompileResponse{
			OK:     false,
			Name:   name,
			Error:  err.Error(),
			Line:   line,
			Column: col,
			Mode:   string(MobileCompileModeV2),
		}
	}

	// Legacy compilation mode
	inputPath = ""
	outputPath = ""
	workflowName = name
	contents = src

	initParse()
	generateShortcut()

	var buf bytes.Buffer
	enc := plist.NewEncoder(&buf)
	enc.Indent("\t")
	if err := enc.Encode(shortcut); err != nil {
		panic(err)
	}
	legacyBytes := buf.Bytes()

	response = mobileCompileResponse{
		OK:          true,
		Name:        workflowName,
		PlistBase64: base64.StdEncoding.EncodeToString(legacyBytes),
		Mode:        string(MobileCompileModeLegacy),
	}

	if sign {
		service := hubSign()
		signedShortcut, signErr := SignShortcutBytes(&service, workflowName, legacyBytes)
		if signErr != nil {
			response.OK = false
			response.Error = fmt.Sprintf("Signing error: %v", signErr)
			return response
		}
		if len(signedShortcut) > 0 && looksLikeSignedShortcut(signedShortcut) {
			response.SignedBase64 = base64.StdEncoding.EncodeToString(signedShortcut)
		} else {
			response.OK = false
			response.Error = "Signing server response does not look like a signed Shortcut (missing AEA1 magic)"
			return response
		}
	}

	return response
}

func decompileForMobile(plistBytes []byte, requestedName string) (response mobileCompileResponse) {
	mobileCompileMu.Lock()
	defer mobileCompileMu.Unlock()

	embeddedCompilerMode = true
	previousArgs := args.Args
	args.Args = map[string]string{"no-ansi": ""}

	defer func() {
		embeddedCompilerMode = false
		args.Args = previousArgs
		if recovered := recover(); recovered != nil {
			response = mobileCompileResponse{
				OK:     false,
				Error:  embeddedErrorMessage(recovered),
				Line:   max(lineIdx+1, 1),
				Column: max(lineCharIdx+1, 1),
			}
			resetEmbeddedFailureState()
		}
	}()

	resetMobileLanguageState()
	resetMobileDecompileState()
	name := normalizedMobileName(requestedName)
	basename = strings.ReplaceAll(name, " ", "_")
	workflowName = name
	filename = name + ".shortcut"
	filePath = ""
	relativePath = ""
	inputPath = ""
	outputPath = ""

	if _, err := plist.Unmarshal(plistBytes, &shortcut); err != nil {
		if jsonErr := json.Unmarshal(plistBytes, &shortcut); jsonErr != nil {
			panic(embeddedCompilerPanic{message: "Unable to read Shortcut plist: " + err.Error()})
		}
	}

	loadBasicStandardActions()
	resetParse()
	firstChar()

	mapVariables()
	mapSplitActions()
	mapIdentifiers()
	mapControlFlowOutputs()
	defineName()
	decompileIcon()
	decompileWorkflowMetadata()
	decompileActions()

	return mobileCompileResponse{
		OK:     true,
		Name:   name,
		Source: code.String(),
	}
}

func currentMobileActionCatalog() []mobileActionInfo {
	reg := schema.DefaultRegistry()
	var catalog []mobileActionInfo
	for _, act := range reg.AllActions() {
		var params []catalogParameter
		for _, p := range act.Parameters {
			params = append(params, catalogParameter{
				Name:       p.Label,
				Key:        p.WireKey,
				Type:       p.TypeName,
				Optional:   p.Optional,
				Enum:       p.EnumName,
				EnumValues: p.EnumValues,
				Default:    p.DefaultValue,
			})
		}
		var intent *catalogAppIntent
		if act.AppIntent != nil {
			intent = &catalogAppIntent{
				Name:                act.AppIntent.Name,
				BundleIdentifier:    act.AppIntent.BundleIdentifier,
				AppIntentIdentifier: act.AppIntent.AppIntentIdentifier,
				TeamIdentifier:      act.AppIntent.TeamIdentifier,
			}
		}
		catalog = append(catalog, catalogActionInfo{
			Name:               act.CallableName,
			ShortcutIdentifier: act.AppleIdentifier,
			Title:              act.Docs.Title,
			Description:        act.Docs.Description,
			Category:           act.Docs.Category,
			Subcategory:        act.Docs.Subcategory,
			Parameters:         params,
			OutputType:         act.OutputTypeName,
			CompilerConstruct:  act.CompilerConstruct,
			InsertionSnippet:   act.Docs.InsertionSnippet,
			AppIntent:          intent,
		})
	}
	return catalog
}

func ensureMobileBaseLanguageState() {
	if mobileBaseActions == nil {
		mobileBaseActions = maps.Clone(actions)
	}
	if mobileBaseEnumerations == nil {
		mobileBaseEnumerations = maps.Clone(enumerations)
	}
}

func resetMobileLanguageState() {
	resetCompilerStateFully()
	included = []string{}
	includes = []include{}
	includedBasicStandardActions = false
	includedFile = false
	functions = nil
	usingFunctions = false
	hasShortcutInputVariables = false
	inputs = []string{}
	outputs = []string{}
	definedWorkflowTypes = []string{}
	definedQuickActions = []string{}
	noInput = nil
	iconColor = 3031607807
	iconGlyph = 61440
	iosVersion = 27.0
	clientVersion = versions["27"]
	automationTriggers = nil
	shortcut = Shortcut{}
}

func normalizedMobileName(requestedName string) string {
	name := strings.TrimSpace(requestedName)
	if name == "" {
		return "Shortcut"
	}
	return name
}

func embeddedErrorMessage(recovered any) string {
	switch value := recovered.(type) {
	case embeddedCompilerPanic:
		return value.Error()
	case error:
		return value.Error()
	default:
		return fmt.Sprint(value)
	}
}

func resetMobileDecompileState() {
	code.Reset()
	decompiledIncludes = nil
	actionIndex = 0
	tabLevel = 0
	groupingIdx = 0
	currentVariableValue = ""
	varUUIDs = nil
	constUUIDs = nil
	identifierMap = nil
	variables = map[string]varValue{}
	questions = map[string]*question{}
	menus = map[string][]varValue{}
	uuids = map[string]string{}
	importQuestionByActionParam = nil
	controlFlowGroups = map[int]controlFlowGroup{}
	includes = []include{}
	definitions = map[string]any{}
	contents = ""
	originalContents = ""
	chars = []rune{}
	lines = []string{}
	char = -1
	idx = -1
	lineIdx = 0
	lineCharIdx = -1
}

func resetEmbeddedFailureState() {
	resetMobileDecompileState()
	tokens = []token{}
}
