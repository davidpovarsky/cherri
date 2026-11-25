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
	OK               bool              `json:"ok"`
	Name             string            `json:"name,omitempty"`
	PlistBase64      string            `json:"plistBase64,omitempty"`
	SignedBase64     string            `json:"signedBase64,omitempty"`
	Source           string            `json:"source,omitempty"`
	Error            string            `json:"error,omitempty"`
	Line             int               `json:"line,omitempty"`
	Column           int               `json:"column,omitempty"`
	Mode             string            `json:"mode,omitempty"`
	PreviewSourceMap *previewSourceMap `json:"previewSourceMap,omitempty"`
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

// detectLegacySource selects only actual, line-anchored file directives.
// Shared lexer tokens keep raw/triple strings and nested comments opaque; their
// contents cannot switch the frontend. Default and explicit V2 never use auto.
func detectLegacySource(src string) bool {
	file := source.NewFile("mobile-mode", "mobile-mode", 1, src)
	lexer := syntax.NewLexer(file)
	anchored := func(offset int) bool {
		for i := offset - 1; i >= 0 && src[i] != '\n'; i-- {
			switch src[i] {
			case ' ', '\t', '\r':
			default:
				// The shared lexer accepts a UTF-8 BOM only at the start.
				return i == 2 && strings.HasPrefix(src, "\ufeff")
			}
		}
		return true
	}
	marker := func(comment source.Trivia) (legacy, found bool) {
		if !anchored(comment.Span.Start.ByteOffset) {
			return false, false
		}
		var body string
		switch comment.Kind {
		case source.TriviaLineComment:
			body = strings.TrimPrefix(comment.Text, "//")
		case source.TriviaBlockComment:
			body = strings.TrimSuffix(strings.TrimPrefix(comment.Text, "/*"), "*/")
		default:
			return false, false
		}
		parts := strings.Fields(body)
		if len(parts) < 2 || parts[0] != "cherri:" {
			return false, false
		}
		switch parts[1] {
		case "legacy", "v1":
			return true, true
		case "v2":
			return false, true
		}
		return false, false
	}

	fStringDepth := 0
	for {
		token := lexer.NextToken()
		if fStringDepth == 0 {
			for _, trivia := range token.Leading {
				if legacy, found := marker(trivia); found {
					return legacy
				}
			}
			if token.Type == syntax.TokenDirective && anchored(token.Span.Start.ByteOffset) {
				switch token.Text {
				case "#include", "#define", "#import", "#question", "#ref", "#workflowheader", "#declare":
					return true
				}
			}
		}
		switch token.Type {
		case syntax.TokenFStringStart:
			fStringDepth++
		case syntax.TokenFStringEnd:
			if fStringDepth > 0 {
				fStringDepth--
			}
		case syntax.TokenEOF, syntax.TokenError:
			// Invalid source remains on V2; a later substring is never a
			// reason to retry a different parser after lexical failure.
			return false
		}
	}
}

func compileForMobile(src string, requestedName string, sign bool) mobileCompileResponse {
	return compileForMobileMode(src, requestedName, string(MobileCompileModeV2), sign)
}

func compileForMobileMode(src string, requestedName string, mode string, sign bool) mobileCompileResponse {
	return compileForMobileModeWithOptions(src, requestedName, mode, sign, CompileOptions{})
}

func compileForMobileModeWithOptions(src string, requestedName string, mode string, sign bool, options CompileOptions) (response mobileCompileResponse) {
	var profile, profileErr = normalizeBuildProfile(options.BuildProfile)
	if profileErr != nil {
		return mobileCompileResponse{OK: false, Error: profileErr.Error(), Line: 1, Column: 1, Mode: mode}
	}
	options.BuildProfile = profile
	mobileCompileMu.Lock()
	defer mobileCompileMu.Unlock()
	// This embedded request shares state ownership with canonical emission.
	// Hold the mutex before changing globals and through deferred recovery.
	compilerSessionMutex.Lock()
	defer compilerSessionMutex.Unlock()

	embeddedCompilerMode = true
	previousArgs := args.Args
	args.Args = map[string]string{"no-ansi": ""}
	if val, ok := previousArgs["comments"]; ok {
		args.Args["comments"] = val
	}

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
	filePath = ""
	relativePath = ""
	inputPath = ""
	outputPath = ""
	name := normalizedMobileName(requestedName)

	selectedMode := MobileCompileMode(strings.ToLower(strings.TrimSpace(mode)))
	if selectedMode == "" {
		selectedMode = MobileCompileModeV2
	}

	if selectedMode == MobileCompileModeAuto {
		if detectLegacySource(src) {
			selectedMode = MobileCompileModeLegacy
		} else {
			selectedMode = MobileCompileModeV2
		}
	}

	if selectedMode != MobileCompileModeV2 && selectedMode != MobileCompileModeLegacy {
		return mobileCompileResponse{OK: false, Error: "COMPILE_MODE_INVALID: expected v2, legacy or auto", Line: 1, Column: 1, Mode: mode}
	}

	if selectedMode == MobileCompileModeV2 {
		var previewMap previewSourceMap
		options.previewSourceMap = &previewMap
		v2Bytes, err := compileSourceToPlistWithSession(name+".cherri", src, options, newCanonicalBackendSessionLocked)
		if err == nil {
			response = mobileCompileResponse{
				OK:               true,
				Name:             name,
				PlistBase64:      base64.StdEncoding.EncodeToString(v2Bytes),
				Mode:             string(MobileCompileModeV2),
				PreviewSourceMap: &previewMap,
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
	generateShortcutWithProfile(profile)

	var buf bytes.Buffer
	enc := plist.NewEncoder(&buf)
	enc.Indent("\t")
	if importedWorkflowHeader != nil {
		merged := mergeWorkflowHeaderMapWithDialect("legacy", importedWorkflowHeader, shortcut.WFWorkflowActions, exactActionParamsPresence, shortcut, false)
		if err := enc.Encode(merged); err != nil {
			panic(err)
		}
	} else {
		if err := enc.Encode(shortcut); err != nil {
			panic(err)
		}
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

func decompileForMobileWithOptions(plistBytes []byte, requestedName string, mode string, fidelity string) mobileCompileResponse {
	f := strings.ToLower(strings.TrimSpace(fidelity))
	if f == "" {
		f = "semantic"
	}
	m := strings.ToLower(strings.TrimSpace(mode))
	if m == "" {
		m = "legacy"
	}
	if f == "semantic" && m == "legacy" {
		return decompileForMobile(plistBytes, requestedName)
	}

	name := normalizedMobileName(requestedName)
	doc, err := DecodeNativeShortcutDocument(plistBytes)
	if err != nil {
		return mobileCompileResponse{
			OK:    false,
			Name:  name,
			Error: err.Error(),
			Mode:  m,
		}
	}

	if f == "lossless" {
		var src string
		if m == "legacy" {
			src, err = RenderLosslessLegacySourceFromDocument(doc)
		} else {
			src, err = RenderLosslessV2SourceFromDocument(doc)
		}
		if err != nil {
			return mobileCompileResponse{OK: false, Name: name, Error: err.Error(), Mode: m}
		}
		return mobileCompileResponse{OK: true, Name: name, Source: src, Mode: m}
	}

	if f == "hybrid" {
		plan, err := PlanHybridImport(doc)
		if err != nil {
			return mobileCompileResponse{OK: false, Name: name, Error: err.Error(), Mode: m}
		}
		var src string
		if m == "legacy" {
			src, err = RenderHybridLegacySource(doc, plan)
		} else {
			src, err = RenderHybridV2Source(doc, plan)
		}
		if err != nil {
			return mobileCompileResponse{OK: false, Name: name, Error: err.Error(), Mode: m}
		}
		return mobileCompileResponse{OK: true, Name: name, Source: src, Mode: m}
	}

	// semantic + v2
	legResp := decompileForMobile(plistBytes, requestedName)
	if !legResp.OK {
		return legResp
	}
	v2Src, err := MigrateSemanticLegacySourceToV2Strict(legResp.Source)
	if err != nil {
		return mobileCompileResponse{OK: false, Name: name, Error: err.Error(), Mode: "v2"}
	}
	return mobileCompileResponse{OK: true, Name: name, Source: v2Src, Mode: "v2"}
}

func decompileForMobile(plistBytes []byte, requestedName string) (response mobileCompileResponse) {
	mobileCompileMu.Lock()
	defer mobileCompileMu.Unlock()
	// This embedded request shares state ownership with canonical emission.
	// Hold the mutex before changing globals and through deferred recovery.
	compilerSessionMutex.Lock()
	defer compilerSessionMutex.Unlock()

	embeddedCompilerMode = true
	previousArgs := args.Args
	args.Args = map[string]string{"no-ansi": ""}
	if val, ok := previousArgs["comments"]; ok {
		args.Args["comments"] = val
	}

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
	filePath = ""
	relativePath = ""
	inputPath = ""
	outputPath = ""
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
	loadAllStandardActionsForDecompile()
	skipNextLoopVarAction = false
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
	var constructs []mobileActionInfo
	for _, act := range reg.AllActions() {
		var params []catalogParameter
		if !act.CompilerConstruct || len(act.Parameters) > 0 {
			params = make([]catalogParameter, 0, len(act.Parameters))
		}
		for _, p := range act.Parameters {
			paramType := p.RawType
			if paramType == "" {
				paramType = strings.ToLower(p.TypeName)
			}
			params = append(params, catalogParameter{
				Name:       p.Label,
				Key:        p.WireKey,
				Type:       paramType,
				Optional:   p.Optional,
				Infinite:   p.Variadic,
				Reference:  p.Reference,
				Literal:    p.Literal,
				Enum:       p.EnumName,
				EnumValues: p.EnumValues,
				Default:    p.DefaultValue,
			})
		}
		var intent *catalogAppIntent
		if act.AppIntent != nil {
			intent = &catalogAppIntent{
				Name:                    act.AppIntent.Name,
				BundleIdentifier:        act.AppIntent.BundleIdentifier,
				AppIntentIdentifier:     act.AppIntent.AppIntentIdentifier,
				TeamIdentifier:          act.AppIntent.TeamIdentifier,
				RequiresAppInstallation: act.AppIntent.RequiresAppInstallation,
			}
		}
		info := catalogActionInfo{
			Name:               act.CallableName,
			ShortcutIdentifier: act.AppleIdentifier,
			Title:              act.Docs.Title,
			Description:        act.Docs.Description,
			Category:           act.Docs.Category,
			Subcategory:        act.Docs.Subcategory,
			Parameters:         params,
			EmittedKeys:        act.EmittedKeys,
			OutputType:         act.RawOutputType,
			MacOnly:            act.MacOnly,
			NonMacOnly:         act.NonMacOnly,
			MinVersion:         act.MinVersion,
			MaxVersion:         act.MaxVersion,
			Builtin:            act.Builtin,
			Custom:             act.Custom,
			CompilerConstruct:  act.CompilerConstruct,
			InsertionSnippet:   act.Docs.InsertionSnippet,
			AppIntent:          intent,
			StaticParameters:   act.StaticParameters,
		}
		if act.CompilerConstruct {
			constructs = append(constructs, info)
		} else {
			catalog = append(catalog, info)
		}
	}
	catalog = append(catalog, constructs...)
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
	macDefinition = false
	setMacDefinition = false
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
