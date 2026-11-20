/*
 * Copyright (c) Cherri Language v2.0
 */

package migrate

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/source"
	"github.com/electrikmilk/cherri/internal/language/syntax"
)

// MigrationDiagnostic represents an error or warning encountered during migration.
type MigrationDiagnostic struct {
	Line    int
	Column  int
	Message string
}

func (d MigrationDiagnostic) String() string {
	return fmt.Sprintf("line %d:%d: %s", d.Line, d.Column, d.Message)
}

// MigrateSource converts legacy Cherri syntax into canonical Cherri v2.0 syntax.
// If migration encounters errors, it returns the input unmodified.
func MigrateSource(input string) string {
	result, _, err := MigrateSourceWithDiagnostics(input)
	if err != nil {
		return input
	}
	return result
}

func expandLocalIncludes(src string, includeRoot string, seen map[string]bool) (string, []MigrationDiagnostic) {
	if seen == nil {
		seen = make(map[string]bool)
	}
	lexemes, diags := scanMigrationLexemes(src)
	if len(diags) != 0 {
		return "", diags
	}
	var out strings.Builder
	last := 0
	for _, item := range lexemes {
		if item.kind != "directive" {
			continue
		}
		trimmed := strings.TrimSpace(item.text)
		if !strings.HasPrefix(trimmed, "#include ") && !strings.HasPrefix(trimmed, "#include\t") {
			continue
		}
		mod := strings.Trim(strings.TrimSpace(trimmed[len("#include"):]), "\"' ")
		if strings.HasPrefix(mod, "actions/") || mod == "stdfunc" || mod == "stdlib" || mod == "" {
			continue
		}
		candidates := []string{mod, filepath.Clean(mod), filepath.Join("tests", mod)}
		if includeRoot != "" {
			candidate := mod
			if !filepath.IsAbs(candidate) {
				candidate = filepath.Join(includeRoot, candidate)
			}
			candidates = []string{candidate}
		}
		for _, candidate := range candidates {
			content, err := os.ReadFile(candidate)
			if err != nil {
				continue
			}
			found, err := filepath.Abs(candidate)
			if err != nil {
				continue
			}
			out.WriteString(src[last:item.start])
			if !seen[found] {
				seen[found] = true
				expanded, nestedDiags := expandLocalIncludes(string(content), includeRoot, seen)
				if len(nestedDiags) != 0 {
					return "", nestedDiags
				}
				out.WriteString(expanded)
			}
			last = item.end
			break
		}
	}
	out.WriteString(src[last:])
	return out.String(), nil
}

// MigrateSourceWithDiagnostics converts legacy Cherri syntax into canonical Cherri v2.0 syntax.
// It fails closed if unrecognized preprocessor directives, corrupted reference hashes,
// or invalid trigger definitions are encountered.
func MigrateSourceWithDiagnostics(input string) (string, []MigrationDiagnostic, error) {
	return migrateSourceWithDiagnostics(input, "")
}

// MigrateSourceWithDiagnosticsAtPath resolves local includes from the entry
// source directory, matching Legacy file compilation. It never changes the
// process working directory or falls back to an unrelated cwd dependency.
func MigrateSourceWithDiagnosticsAtPath(input, sourcePath string) (string, []MigrationDiagnostic, error) {
	if strings.TrimSpace(sourcePath) == "" {
		return "", []MigrationDiagnostic{{Line: 1, Column: 1, Message: "missing migration source path"}}, fmt.Errorf("missing migration source path")
	}
	path, err := filepath.Abs(sourcePath)
	if err != nil {
		return "", []MigrationDiagnostic{{Line: 1, Column: 1, Message: "cannot resolve migration source path"}}, err
	}
	return migrateSourceWithDiagnostics(input, filepath.Dir(path))
}

func migrateSourceWithDiagnostics(input, includeRoot string) (string, []MigrationDiagnostic, error) {
	input, lexicalDiags := expandLocalIncludes(input, includeRoot, nil)
	if len(lexicalDiags) != 0 {
		for index := range lexicalDiags {
			lexicalDiags[index].Message = "generated V2 source is not representable: " + lexicalDiags[index].Message
		}
		return "", lexicalDiags, fmt.Errorf("migration produced invalid V2 syntax (Legacy lexical input)")
	}
	lexical, lexicalDiags := prepareMigrationSource(input)
	if len(lexicalDiags) != 0 {
		for index := range lexicalDiags {
			lexicalDiags[index].Message = "generated V2 source is not representable: " + lexicalDiags[index].Message
		}
		return "", lexicalDiags, fmt.Errorf("migration produced invalid V2 syntax (Legacy lexical input)")
	}
	reg := schema.DefaultRegistry()
	lines := strings.Split(lexical.text, "\n")
	var diags []MigrationDiagnostic

	addDiag := func(lineNum, col int, msg string) {
		diags = append(diags, MigrationDiagnostic{
			Line:    lineNum,
			Column:  col,
			Message: msg,
		})
	}

	// 1. Collect legacy directives (#define, #include, #question, #trigger, #ref)
	var shortcutName string
	var iconGlyph string
	var iconColor string
	var versionVal string
	var noinputVal string
	var inputList []string
	var outputList []string
	var imports []string
	otherDefines := make(map[string]string)

	type questionDef struct {
		name       string
		prompt     string
		defaultVal string
	}
	var questions []questionDef

	type triggerDef struct {
		family string
		args   string
	}
	var triggers []triggerDef

	type refDef struct {
		name    string
		jsonVal string
	}
	var refs []refDef

	validScreenshotLocations := map[string]bool{"photos": true, "files": true, "clipboard": true}
	validWifiEvents := map[string]bool{"joined": true, "disconnected": true, "both": true}
	validConnectionEvents := map[string]bool{"connect": true, "disconnect": true, "both": true}
	validStageManagerEvents := map[string]bool{"on": true, "off": true, "both": true}

	for _, directive := range lexical.directives {
		lineNum := directive.line
		trimmed := strings.TrimSpace(directive.text)
		if !strings.HasPrefix(trimmed, "#") {
			continue
		}

		if strings.HasPrefix(trimmed, "#define ") || strings.HasPrefix(trimmed, "#define\t") {
			rest := strings.TrimSpace(trimmed[len("#define"):])
			parts := strings.Fields(rest)
			if len(parts) >= 2 {
				key := parts[0]
				val := strings.TrimSpace(rest[len(key):])
				val = strings.Trim(val, "\"")
				switch key {
				case "name":
					shortcutName = val
				case "icon", "glyph":
					iconGlyph = val
				case "color":
					iconColor = val
				case "version":
					versionVal = val
				case "noinput", "noInput":
					noinputVal = val
				case "inputs":
					for _, item := range strings.Split(val, ",") {
						itemTrim := strings.TrimSpace(item)
						if itemTrim != "" {
							inputList = append(inputList, itemTrim)
						}
					}
				case "outputs":
					for _, item := range strings.Split(val, ",") {
						itemTrim := strings.TrimSpace(item)
						if itemTrim != "" {
							outputList = append(outputList, itemTrim)
						}
					}
				default:
					otherDefines[key] = val
				}
			} else if len(parts) == 1 {
				otherDefines[parts[0]] = "true"
			}
		} else if strings.HasPrefix(trimmed, "#include ") || strings.HasPrefix(trimmed, "#include\t") {
			mod := strings.TrimSpace(trimmed[len("#include"):])
			mod = strings.Trim(mod, "\"' ")
			if mod == "stdfunc" || mod == "stdlib" {
				imports = append(imports, "stdfunc")
			} else if !strings.HasPrefix(mod, "actions/") && mod != "" {
				imports = append(imports, mod)
			}
		} else if strings.HasPrefix(trimmed, "#question ") || strings.HasPrefix(trimmed, "#question\t") {
			rest := strings.TrimSpace(trimmed[len("#question"):])
			parts := strings.Split(rest, "\"")
			if len(parts) >= 3 {
				qName := strings.TrimSpace(parts[0])
				prompt := parts[1]
				defVal := ""
				if len(parts) >= 5 {
					defVal = parts[3]
				}
				questions = append(questions, questionDef{
					name:       qName,
					prompt:     prompt,
					defaultVal: defVal,
				})
			}
		} else if strings.HasPrefix(trimmed, "#trigger ") || strings.HasPrefix(trimmed, "#trigger\t") {
			rest := strings.TrimSpace(trimmed[len("#trigger"):])
			parts := strings.Fields(rest)
			if len(parts) == 0 {
				addDiag(lineNum, 1, "expected trigger family and parameters")
				continue
			}
			family := parts[0]
			argRest := strings.TrimSpace(rest[len(family):])

			if schema.TriggerFamilies[family] == "" {
				addDiag(lineNum, 1, fmt.Sprintf("unknown trigger family %q; supported families: battery, bluetooth, charging, display, screenshot, stageManager, wifi", family))
				continue
			}

			switch family {
			case "battery":
				if argRest == "" {
					addDiag(lineNum, 1, "missing battery level")
					continue
				}
				if _, err := strconv.ParseFloat(argRest, 64); err != nil {
					addDiag(lineNum, 1, fmt.Sprintf("invalid battery level %q: %v", argRest, err))
					continue
				}
				triggers = append(triggers, triggerDef{family: "battery", args: fmt.Sprintf("level: %s", argRest)})
			case "screenshot":
				if argRest == "" {
					addDiag(lineNum, 1, "missing screenshot location")
					continue
				}
				rawLocs := strings.Split(argRest, ",")
				var quotedLocs []string
				hasInvalid := false
				for _, loc := range rawLocs {
					locTrim := strings.Trim(strings.TrimSpace(loc), "\"'")
					if !validScreenshotLocations[locTrim] {
						addDiag(lineNum, 1, fmt.Sprintf("invalid screenshot location %q; must be photos, files, or clipboard", locTrim))
						hasInvalid = true
						break
					}
					quotedLocs = append(quotedLocs, fmt.Sprintf("%q", locTrim))
				}
				if !hasInvalid {
					triggers = append(triggers, triggerDef{family: "screenshot", args: fmt.Sprintf("locations: [%s]", strings.Join(quotedLocs, ", "))})
				}
			case "wifi":
				event := strings.Trim(argRest, "\"' ")
				if !validWifiEvents[event] {
					addDiag(lineNum, 1, fmt.Sprintf("invalid wifi event %q; must be joined, disconnected, or both", event))
					continue
				}
				triggers = append(triggers, triggerDef{family: "wifi", args: fmt.Sprintf("event: .%s", event)})
			case "bluetooth":
				event := strings.Trim(argRest, "\"' ")
				if !validConnectionEvents[event] {
					addDiag(lineNum, 1, fmt.Sprintf("invalid bluetooth event %q; must be connect, disconnect, or both", event))
					continue
				}
				triggers = append(triggers, triggerDef{family: "bluetooth", args: fmt.Sprintf("event: .%s", event)})
			case "stageManager":
				event := strings.Trim(argRest, "\"' ")
				if !validStageManagerEvents[event] {
					addDiag(lineNum, 1, fmt.Sprintf("invalid stageManager event %q; must be on, off, or both", event))
					continue
				}
				triggers = append(triggers, triggerDef{family: "stageManager", args: fmt.Sprintf("event: .%s", event)})
			case "display":
				event := strings.Trim(argRest, "\"' ")
				if !validConnectionEvents[event] {
					addDiag(lineNum, 1, fmt.Sprintf("invalid display event %q; must be connect, disconnect, or both", event))
					continue
				}
				triggers = append(triggers, triggerDef{family: "display", args: fmt.Sprintf("event: .%s", event)})
			case "charging":
				event := strings.Trim(argRest, "\"' ")
				if !validConnectionEvents[event] {
					addDiag(lineNum, 1, fmt.Sprintf("invalid charging event %q; must be connect, disconnect, or both", event))
					continue
				}
				triggers = append(triggers, triggerDef{family: "charging", args: fmt.Sprintf("event: .%s", event)})
			}
		} else if strings.HasPrefix(trimmed, "#ref ") || strings.HasPrefix(trimmed, "#ref\t") {
			rest := strings.TrimSpace(trimmed[len("#ref"):])
			parts := strings.Fields(rest)
			if len(parts) < 2 {
				addDiag(lineNum, 1, "expected reference identifier and hash in #ref directive")
				continue
			}
			ident := parts[0]
			hash := parts[1]
			decodedBytes, err := base64.StdEncoding.DecodeString(hash)
			if err != nil {
				addDiag(lineNum, 1, fmt.Sprintf("could not decode base64 reference hash for %q: %v", ident, err))
				continue
			}
			var refVal map[string]any
			if err := json.Unmarshal(decodedBytes, &refVal); err != nil {
				addDiag(lineNum, 1, fmt.Sprintf("could not unmarshal JSON reference data for %q: %v", ident, err))
				continue
			}
			compactJSON, _ := json.Marshal(refVal)
			refs = append(refs, refDef{name: ident, jsonVal: string(compactJSON)})
		} else {
			// Unrecognized preprocessor directive: fail closed
			addDiag(lineNum, 1, fmt.Sprintf("unrecognized preprocessor directive %q", trimmed))
		}
	}

	if len(diags) > 0 {
		return "", diags, fmt.Errorf("migration failed with %d error(s)", len(diags))
	}

	// 2. Track typed variable declarations: @var: type -> var var: Type
	typedVarRegex := regexp.MustCompile(`^(\s*)@([a-zA-Z0-9_]+)\s*:\s*([a-zA-Z0-9_]+)\s*$`)
	typedVars := make(map[string]string)
	for _, l := range lines {
		if m := typedVarRegex.FindStringSubmatch(l); m != nil {
			typedVars[m[2]] = mapLegacyType(m[3])
		}
	}

	// 3. Track variable assignments to choose 'let' vs 'var'
	varAssignmentCount := make(map[string]int)
	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		if strings.HasPrefix(trimmed, "@") {
			parts := strings.Fields(trimmed)
			if len(parts) > 0 {
				varName := strings.TrimPrefix(parts[0], "@")
				varName = strings.TrimSuffix(varName, ":")
				varAssignmentCount[varName]++
			}
		}
	}

	// 4. Pre-process menus with Python-style item indentation
	lines = migrateMenus(lines)

	seenVars := make(map[string]bool)
	for v := range typedVars {
		seenVars[v] = true
	}

	var outLines []string
	legacyBindingLines := make(map[int]string)
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// 2. Convert typed var declarations: @var: type -> var var: Type [= default]
		if m := typedVarRegex.FindStringSubmatch(line); m != nil {
			indent := m[1]
			varName := m[2]
			typeName := typedVars[varName]
			defVal := ""
			switch strings.ToLower(typeName) {
			case "dictionary", "dict", "map":
				defVal = " = {}"
			case "array", "list":
				defVal = ""
			case "text", "string":
				defVal = ` = ""`
			case "number", "int", "float":
				defVal = " = 0"
			case "bool", "boolean":
				defVal = " = false"
			}
			legacyBindingLines[len(outLines)+1] = varName
			outLines = append(outLines, fmt.Sprintf("%svar %s: %s%s", indent, varName, typeName, defVal))
			continue
		}

		// Keep Legacy repeat i for N spelling. Its one-based native index is a
		// semantic boundary, so migration/formatting must not rewrite it to the
		// modern zero-based repeat N as i form.

		// 4. Convert const name = val -> let name = val
		if strings.HasPrefix(trimmed, "const ") {
			line = strings.Replace(line, "const ", "let ", 1)
		}

		// 5. Convert legacy function signatures: function name(type _param) -> function name(_param: Type)
		if strings.HasPrefix(trimmed, "function ") {
			line = migrateLegacyFunctionDecl(line)
			trimmed = strings.TrimSpace(line)
		}

		// 6. Convert legacy output(...) -> return ...
		if strings.HasPrefix(trimmed, "output(") && strings.HasSuffix(trimmed, ")") {
			line = migrateLegacyOutputStmt(line)
			trimmed = strings.TrimSpace(line)
		}

		// 7. Convert legacy if legacy ... -> if ...
		if strings.HasPrefix(trimmed, "if legacy ") {
			line = strings.Replace(line, "if legacy ", "if ", 1)
			trimmed = strings.TrimSpace(line)
		}

		// 7b. Convert legacy negation conditions: if A !contains B -> if !(A contains B)
		if strings.Contains(line, " !contains ") {
			re := regexp.MustCompile(`(\s*if\s+)(.+?)\s+!contains\s+(.+?)(\s*\{)`)
			line = re.ReplaceAllString(line, "${1}!(${2} contains ${3})${4}")
			trimmed = strings.TrimSpace(line)
		}
		if strings.Contains(line, " !beginsWith ") {
			re := regexp.MustCompile(`(\s*if\s+)(.+?)\s+!beginsWith\s+(.+?)(\s*\{)`)
			line = re.ReplaceAllString(line, "${1}!(${2} beginsWith ${3})${4}")
			trimmed = strings.TrimSpace(line)
		}
		if strings.Contains(line, " !endsWith ") {
			re := regexp.MustCompile(`(\s*if\s+)(.+?)\s+!endsWith\s+(.+?)(\s*\{)`)
			line = re.ReplaceAllString(line, "${1}!(${2} endsWith ${3})${4}")
			trimmed = strings.TrimSpace(line)
		}
		if strings.Contains(line, " <> ") {
			re := regexp.MustCompile(`(\s*if\s+)(.+?)\s+<>\s+(\S+)\s+(\S+)(\s*\{)`)
			line = re.ReplaceAllString(line, "${1}isBetween(${2}, ${3}, ${4})${5}")
			trimmed = strings.TrimSpace(line)
		}
		if strings.Contains(line, " isToday") {
			re := regexp.MustCompile(`(\s*if\s+)(.+?)\s+isToday(\s*\{)`)
			line = re.ReplaceAllString(line, "${1}isToday(${2})${3}")
			trimmed = strings.TrimSpace(line)
		}

		// 8. Convert @var = val -> let/var var = val
		if strings.HasPrefix(trimmed, "@") {
			// Check if +=, -=, *=, /=
			if strings.Contains(line, "+=") || strings.Contains(line, "-=") ||
				strings.Contains(line, "*=") || strings.Contains(line, "/=") {
				line = strings.Replace(line, "@", "", 1)
			} else if strings.Contains(line, "=") {
				parts := strings.SplitN(trimmed, "=", 2)
				lhs := strings.TrimSpace(parts[0])
				rhs := parts[1]
				varName := strings.TrimPrefix(lhs, "@")
				indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]

				if typedVars[varName] != "" {
					line = fmt.Sprintf("%s%s =%s", indent, varName, rhs)
				} else if !seenVars[varName] {
					seenVars[varName] = true
					keyword := "let"
					if varAssignmentCount[varName] > 1 {
						keyword = "var"
					}
					line = fmt.Sprintf("%s%s %s =%s", indent, keyword, varName, rhs)
					legacyBindingLines[len(outLines)+1] = varName
				} else {
					line = fmt.Sprintf("%s%s =%s", indent, varName, rhs)
				}
			}
		}

		// Literal/comment contents are protected; only actual Legacy variable
		// token prefixes are removed from executable syntax.
		line = stripMigrationAtVariables(line)

		// 11. Label positional arguments for known action schemas
		line = labelCallArguments(line, reg)

		outLines = append(outLines, line)
	}

	// Build Header
	var outHeader strings.Builder
	hasHeader := shortcutName != "" || iconGlyph != "" || iconColor != "" || versionVal != "" ||
		noinputVal != "" || len(inputList) > 0 || len(outputList) > 0 || len(otherDefines) > 0 || len(triggers) > 0

	if hasHeader {
		name := shortcutName
		if name == "" {
			name = "Shortcut"
		}
		outHeader.WriteString(fmt.Sprintf("shortcut %q {\n", name))
		if iconGlyph != "" || iconColor != "" {
			if iconGlyph != "" && iconColor != "" {
				outHeader.WriteString(fmt.Sprintf("    icon: {\n        glyph: %s,\n        color: %s\n    }\n", iconGlyph, iconColor))
			} else if iconGlyph != "" {
				outHeader.WriteString(fmt.Sprintf("    icon: %s\n", iconGlyph))
			} else {
				outHeader.WriteString(fmt.Sprintf("    color: %s\n", iconColor))
			}
		}
		if versionVal != "" {
			outHeader.WriteString(fmt.Sprintf("    version: %s\n", versionVal))
		}
		if noinputVal != "" {
			outHeader.WriteString(fmt.Sprintf("    noinput: %s\n", noinputVal))
		}
		if len(inputList) > 0 {
			var mappedInputs []string
			for _, item := range inputList {
				cls := schema.LookupContentItemClassValue(item)
				if cls != "" {
					mappedInputs = append(mappedInputs, cls)
				} else {
					mappedInputs = append(mappedInputs, item)
				}
			}
			outHeader.WriteString(fmt.Sprintf("    inputs: [%s]\n", strings.Join(mappedInputs, ", ")))
		}
		if len(outputList) > 0 {
			var mappedOutputs []string
			for _, item := range outputList {
				cls := schema.LookupContentItemClassValue(item)
				if cls != "" {
					mappedOutputs = append(mappedOutputs, cls)
				} else {
					mappedOutputs = append(mappedOutputs, item)
				}
			}
			outHeader.WriteString(fmt.Sprintf("    outputs: [%s]\n", strings.Join(mappedOutputs, ", ")))
		}
		for k, v := range otherDefines {
			if strings.Contains(v, ",") {
				vTrim := strings.TrimSpace(v)
				if !strings.HasPrefix(vTrim, "[") {
					v = "[" + vTrim + "]"
				}
			}
			outHeader.WriteString(fmt.Sprintf("    %s: %s\n", k, v))
		}
		for _, trig := range triggers {
			outHeader.WriteString(fmt.Sprintf("    trigger %s(%s)\n", trig.family, trig.args))
		}
		outHeader.WriteString("}\n\n")
	}

	for _, imp := range imports {
		alias := filepath.Base(imp)
		alias = strings.TrimSuffix(alias, filepath.Ext(alias))
		alias = strings.ReplaceAll(alias, "-", "_")
		if !isValidIdent(alias) {
			alias = "importedModule"
		}
		outHeader.WriteString(fmt.Sprintf("import %q as %s\n", imp, alias))
	}
	if len(imports) > 0 {
		outHeader.WriteString("\n")
	}

	for _, q := range questions {
		outHeader.WriteString(fmt.Sprintf("setup %s: Text {\n    prompt: %q\n", q.name, q.prompt))
		if q.defaultVal != "" {
			outHeader.WriteString(fmt.Sprintf("    default: %q\n", q.defaultVal))
		}
		outHeader.WriteString("}\n\n")
	}

	for _, ref := range refs {
		outHeader.WriteString(fmt.Sprintf("let %s = native.plist(%q)\n", ref.name, ref.jsonVal))
	}
	if len(refs) > 0 {
		outHeader.WriteString("\n")
	}

	body := strings.Join(outLines, "\n")
	origins := make(map[int]string, len(legacyBindingLines))
	for line, name := range legacyBindingLines {
		prefix := strings.Join(outLines[:line-1], "\n")
		if line > 1 {
			prefix += "\n"
		}
		origins[1+strings.Count(lexical.restore(prefix), "\n")] = name
	}
	legacyBindingLines = origins
	result := outHeader.String() + lexical.restore(body)

	// Try formatting canonically
	file := source.NewFile("migration", "migration.cherri", 1, result)
	p := syntax.NewParser(file)
	prog := p.ParseProgram()
	if len(p.Errors()) == 0 {
		preserveLegacyOptionalBindings(prog, result, legacyBindingLines, strings.Count(outHeader.String(), "\n"), reg)
		referenceValues := make(map[string]string, len(refs))
		for _, ref := range refs {
			referenceValues[ref.name] = ref.jsonVal
		}
		if err := inlineMigrationReferences(prog, referenceValues, len(refs)); err != nil {
			addDiag(1, 1, "generated V2 source is not representable: "+err.Error())
			return "", diags, fmt.Errorf("migration produced invalid V2 syntax (reference identity)")
		}
		migrateVariadicCalls(prog, reg)
		return syntax.Format(prog), diags, nil
	}

	// A preview that cannot parse must never become a successful in-place write.
	// Keep this gate in the shared migration path so every CLI has the same rule.
	for _, parseError := range p.Errors() {
		addDiag(parseError.Span.Start.Line, parseError.Span.Start.Column, "generated V2 source is not representable: "+parseError.Message)
	}
	return "", diags, fmt.Errorf("migration produced invalid V2 syntax (%d error(s))", len(p.Errors()))
}

func mapLegacyType(t string) string {
	switch strings.ToLower(t) {
	case "text", "string":
		return "Text"
	case "number", "int", "integer", "float", "real":
		return "Number"
	case "bool", "boolean":
		return "Bool"
	case "dictionary", "dict":
		return "Dictionary"
	case "array", "list", "collection":
		return "List"
	case "variable", "ref":
		return "Any"
	default:
		if len(t) > 0 {
			return strings.ToUpper(t[:1]) + t[1:]
		}
		return "Any"
	}
}

func migrateMenus(lines []string) []string {
	var out []string
	inMenu := false
	openCase := false
	openCaseIndent := ""

	menuRegex := regexp.MustCompile(`^(\s*)menu\s+("[^"]*"|[a-zA-Z0-9_]+)\s*\{\s*$`)
	itemRegex := regexp.MustCompile(`^(\s*)item\s+("[^"]*")\s*:\s*$`)

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if m := menuRegex.FindStringSubmatch(line); m != nil {
			indent := m[1]
			prompt := m[2]
			if !strings.HasPrefix(prompt, "(") {
				prompt = "(" + prompt + ")"
			}
			out = append(out, indent+"menu"+prompt+" {")
			inMenu = true
			openCase = false
			continue
		}

		if inMenu {
			if im := itemRegex.FindStringSubmatch(line); im != nil {
				indent := im[1]
				label := im[2]
				if openCase {
					out = append(out, openCaseIndent+"}")
				}
				out = append(out, indent+"case "+label+" {")
				openCase = true
				openCaseIndent = indent
				continue
			}

			if trimmed == "}" {
				if openCase {
					out = append(out, openCaseIndent+"}")
					openCase = false
				}
				inMenu = false
				out = append(out, line)
				continue
			}
		}

		out = append(out, line)
	}

	if openCase {
		out = append(out, openCaseIndent+"}")
	}
	return out
}

func labelCallArguments(line string, reg *schema.Registry) string {
	if !strings.Contains(line, "(") || !strings.Contains(line, ")") {
		return line
	}

	openParen := strings.Index(line, "(")
	closeParen := strings.LastIndex(line, ")")
	if openParen >= closeParen {
		return line
	}

	prefix := line[:openParen]
	callee := ""
	for i := len(prefix) - 1; i >= 0; i-- {
		ch := prefix[i]
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_' {
			callee = string(ch) + callee
		} else {
			break
		}
	}

	if callee == "" || callee == "rawAction" || callee == "action" || callee == "native" {
		return line
	}

	actionSchema, ok := reg.LookupAction(callee)
	if !ok || len(actionSchema.Parameters) <= 1 {
		return line
	}

	argsStr := line[openParen+1 : closeParen]
	rawArgs := splitCallArgs(argsStr)
	if len(rawArgs) <= 1 {
		return line
	}

	var newArgs []string
	for i, arg := range rawArgs {
		trimmed := strings.TrimSpace(arg)
		colonIdx := strings.Index(trimmed, ":")
		isLabeled := false
		if colonIdx > 0 {
			lbl := strings.TrimSpace(trimmed[:colonIdx])
			if isValidIdent(lbl) {
				isLabeled = true
			}
		}

		if i > 0 && !isLabeled && i < len(actionSchema.Parameters) && actionSchema.Parameters[i].Label != "" && !strings.HasPrefix(trimmed, "{") {
			paramIdx := i
			if (callee == "editReminder" || callee == "editEvent") && len(rawArgs) == 3 && (trimmed == `"Remove"` || trimmed == `"Set"`) {
				paramIdx = 3
			}
			param := actionSchema.Parameters[paramIdx]
			newArgs = append(newArgs, param.Label+": "+trimmed)
		} else {
			newArgs = append(newArgs, trimmed)
		}
	}

	return line[:openParen+1] + strings.Join(newArgs, ", ") + line[closeParen:]
}

func splitCallArgs(s string) []string {
	var args []string
	var current strings.Builder
	inQuote := false
	quoteChar := byte(0)
	escaped := false
	depth := 0

	for i := 0; i < len(s); i++ {
		ch := s[i]
		if inQuote {
			current.WriteByte(ch)
			if escaped {
				escaped = false
			} else if ch == '\\' {
				escaped = true
			} else if ch == quoteChar {
				inQuote = false
			}
			continue
		}

		switch ch {
		case '"', '\'':
			inQuote = true
			quoteChar = ch
			current.WriteByte(ch)
		case '(', '[', '{':
			depth++
			current.WriteByte(ch)
		case ')', ']', '}':
			depth--
			current.WriteByte(ch)
		case ',':
			if depth == 0 {
				args = append(args, current.String())
				current.Reset()
			} else {
				current.WriteByte(ch)
			}
		default:
			current.WriteByte(ch)
		}
	}
	if current.Len() > 0 {
		args = append(args, current.String())
	}
	return args
}

func isValidIdent(s string) bool {
	if len(s) == 0 {
		return false
	}
	for i, r := range s {
		if i == 0 && !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '_') {
			return false
		}
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_') {
			return false
		}
	}
	return true
}

func migrateLegacyFunctionDecl(line string) string {
	idxOpen := strings.Index(line, "(")
	idxClose := strings.LastIndex(line, ")")
	if idxOpen == -1 || idxClose == -1 || idxOpen >= idxClose {
		return line
	}
	prefix := line[:idxOpen]
	rawParams := line[idxOpen+1 : idxClose]
	suffix := line[idxClose+1:]

	if strings.TrimSpace(rawParams) == "" {
		return line
	}

	parts := strings.Split(rawParams, ",")
	var newParams []string
	needsMigration := false
	for _, p := range parts {
		trimmedP := strings.TrimSpace(p)
		if strings.Contains(trimmedP, ":") {
			newParams = append(newParams, trimmedP)
			continue
		}
		defaultVal := ""
		pDecl := trimmedP
		if strings.Contains(trimmedP, "=") {
			eqParts := strings.SplitN(trimmedP, "=", 2)
			pDecl = strings.TrimSpace(eqParts[0])
			defaultVal = " = " + strings.TrimSpace(eqParts[1])
		}
		fields := strings.Fields(pDecl)
		if len(fields) == 2 {
			needsMigration = true
			typeStr := fields[0]
			nameStr := fields[1]
			switch strings.ToLower(typeStr) {
			case "text", "string":
				typeStr = "Text"
			case "array", "list":
				typeStr = "List"
			case "number", "int", "float":
				typeStr = "Number"
			case "bool", "boolean":
				typeStr = "Bool"
			case "dict", "dictionary":
				typeStr = "Dictionary"
			default:
				r := []rune(typeStr)
				r[0] = unicode.ToUpper(r[0])
				typeStr = string(r)
			}
			newParams = append(newParams, fmt.Sprintf("%s: %s%s", nameStr, typeStr, defaultVal))
		} else if len(fields) == 1 {
			newParams = append(newParams, fields[0])
		} else {
			newParams = append(newParams, trimmedP)
		}
	}
	if !needsMigration {
		return line
	}
	return prefix + "(" + strings.Join(newParams, ", ") + ")" + suffix
}

func migrateLegacyOutputStmt(line string) string {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "output(") || !strings.HasSuffix(trimmed, ")") {
		return line
	}
	arg := trimmed[len("output(") : len(trimmed)-1]
	argTrim := strings.TrimSpace(arg)
	indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
	return indent + "return " + argTrim
}
