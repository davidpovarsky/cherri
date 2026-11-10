/*
 * Copyright (c) Cherri
 */

package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"unicode"

	"github.com/electrikmilk/args-parser"
	"howett.net/plist"
)

var filePath string
var filename string
var basename string
var contents string
var originalContents string
var relativePath string
var inputPath string
var outputPath string

var internalDirectoryPath = os.ExpandEnv("$HOME/.cherri")

const unsignedEnd = "_unsigned.shortcut"
const darwin = runtime.GOOS == "darwin"

func main() {
	if HandleLanguageV2CLI() {
		os.Exit(0)
	}

	filePath = fileArg()
	if filePath != "" {
		filename = checkFile(filePath)
		if !args.Using("legacy") {
			// Compile via Cherri v2 pipeline
			outPath := ""
			if args.Using("output") {
				outPath = args.Value("output")
			}
			skipSign := args.Using("skip-sign") || (!darwin && !args.Using("hubsign") && !args.Using("signing-server"))
			if err := CompileFileV2(filePath, outPath, skipSign); err != nil {
				fmt.Fprintf(os.Stderr, "Compilation error: %v\n", err)
				os.Exit(1)
			}
			return
		}

		if args.Using("load-all-actions") {
			loadStandardActions()
		}

		handleFile()

		initParse()

		generateShortcut()

		createShortcut()
		return
	}

	if args.Using("help") {
		args.PrintUsage()
		os.Exit(0)
	}

	if args.Using("version") {
		printVersion()
		os.Exit(0)
	}

	if args.Using("docs") {
		generateDocs()
		os.Exit(0)
	}

	if args.Using("init") {
		initPackage()
		os.Exit(0)
	}
	if args.Using("add-uri") {
		addUri()
		os.Exit(0)
	}
	if args.Using("install") {
		addPackage()
		os.Exit(0)
	}
	if args.Using("remove") {
		removePackage()
		os.Exit(0)
	}
	if args.Using("package") {
		listPackage()
		os.Exit(0)
	}
	if args.Using("packages") {
		listPackages()
		os.Exit(0)
	}
	if args.Using("tidy") {
		tidyPackage()
		os.Exit(0)
	}

	if args.Using("import") && args.Value("import") != "" {
		mode := "v2"
		if args.Using("legacy") || args.Value("mode") == "legacy" {
			mode = "legacy"
		}
		var out string
		if args.Using("output") {
			out = args.Value("output")
		}
		_, err := ExecuteImportPipeline(ImportOptions{
			InputPath:  args.Value("import"),
			OutputPath: out,
			Mode:       mode,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "import error: %v\n", err)
			os.Exit(1)
		}

		os.Exit(0)
	}

	if args.Using("refs") && args.Value("refs") != "" {
		var shortcutBytes = importShortcut(args.Value("refs"))
		extractReferences(shortcutBytes)

		os.Exit(0)
	}

	if args.Using("action") {
		markBuiltins()
		defineRawAction()
		loadStandardActions()
		handleActionSearch()
		os.Exit(0)
	}

	if args.Using("actions-json") {
		markBuiltins()
		defineRawAction()
		loadStandardActions()
		generateActionsJSON()
		os.Exit(0)
	}

	if args.Using("glyph") {
		handleGlyphSearch()
		os.Exit(0)
	}

	printLogo()
	printVersion()
	fmt.Print("\n")
	args.PrintUsage()
	os.Exit(1)
}

func yesNo() bool {
	var input string
	for {
		scan("(y/n) ", &input)
		input = strings.ToLower(input)
		if input == "y" || input == "n" {
			return input == "y"
		}
	}
}

func camelCase(s string) (c string) {
	var lastChar rune
	for i, r := range s {
		switch {
		case unicode.IsLetter(r):
			if i == 0 {
				c += strings.ToLower(string(r))
			} else if unicode.IsLetter(lastChar) {
				c += strings.ToLower(string(r))
			} else {
				c += strings.ToUpper(string(r))
			}
		case r == '_' || unicode.IsDigit(r):
			c += string(r)
		}
		lastChar = r
	}
	return
}

var tabLevel int

// tabbedLine returns s prepended with tab characters at the current tabLevel.
func tabbedLine(s string) string {
	if tabLevel < 1 {
		return s
	}
	var str strings.Builder
	for i := 0; i < tabLevel; i++ {
		str.WriteRune('\t')
	}
	str.WriteString(s)

	return str.String()
}

func scan(prompt string, store *string) {
	fmt.Print(prompt)

	var scanner = bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		*store = scanner.Text()
	}

	var scanErr = scanner.Err()
	handle(scanErr)
}

func handle(err error) {
	if err == nil {
		return
	}

	fmt.Print(ansi("\nProgram panic :(\n\n", red, bold))
	fmt.Print(ansi("Please report this: https://github.com/electrikmilk/cherri/issues/new\n\n", red))

	if args.Using("debug") {
		panicDebug(err)
	} else {
		panic(err)
	}
}

func createInternalDir() {
	if _, statErr := os.Stat(internalDirectoryPath); os.IsNotExist(statErr) {
		var intDirErr = os.Mkdir(internalDirectoryPath, 0777)
		handle(intDirErr)
	}
}

func printLogo() {
	fmt.Print(ansi("\n           %############                      \n           %#################                 \n           %############*######               \n            ## #############**#*              \n            ##    ############****            \n            ##%     %#%                       \n", green))
	fmt.Print(ansi("             #####", red))
	fmt.Print(ansi("    %##    ####             \n         ###****######  #############         \n        ##**######################***#        \n       ############################*+*#       \n      #############################***#       \n       #############################*##       \n       ################################       \n        ##############  ##############        \n           #########      #########           \n\n", red))
}

func printVersion() {
	var color outputType
	if strings.Contains(version, "beta") {
		color = yellow
	} else {
		color = green
	}
	fmt.Println("Cherri Compiler", ansi(version, color))
}

func fileArg() string {
	if len(os.Args) < 2 {
		return ""
	}
	var fileName = os.Args[1]
	if !startsWith("-", fileName) && strings.Contains(fileName, ".cherri") {
		return fileName
	}
	return ""
}

// handleFile splits the file argument into parts.
func handleFile() {
	relativePath = strings.Replace(filePath, filename, "", 1)
	var nameParts = strings.Split(filename, ".")
	basename = nameParts[0]
	workflowName = basename

	var fileBytes, readErr = os.ReadFile(filePath)
	handle(readErr)
	contents = string(fileBytes)
}

// checkFile checks if the file exists and is a .cherri file.
func checkFile(filePath string) (filename string) {
	var file, statErr = os.Stat(filePath)
	if os.IsNotExist(statErr) {
		exit(fmt.Sprintf("File '%s' does not exist!", filePath))
	}
	var nameParts = strings.Split(file.Name(), ".")
	var ext = end(nameParts)
	if ext != "cherri" {
		exit(fmt.Sprintf("File '%s' is not a .cherri file!", filePath))
	}
	return file.Name()
}

func end(slice []string) string {
	return slice[len(slice)-1]
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	var char = s[0]
	var after, _ = strings.CutPrefix(s, string(char))
	return fmt.Sprintf("%c%s", unicode.ToUpper(rune(char)), after)
}

// startsWith determines if the beginning characters of `substr` match `s`.
func startsWith(s string, substr string) bool {
	var subStringChars = []rune(substr)
	var stringChars = []rune(s)
	var stringSize = len(s)
	var start string
	for i, char := range subStringChars {
		if stringSize < i+1 {
			break
		}
		if char != stringChars[i] {
			if len(start) > 0 {
				break
			}
			return false
		}
		start = fmt.Sprintf("%s%c", start, char)
	}

	return start == s
}

func lineReport(label string) {
	fmt.Printf("--- %s ---\n", label)
	if idx != 0 {
		fmt.Println("Previous Character:")
		var prevChar = prev(1)
		if prevChar != '\n' {
			printChar(prevChar, lineIdx, lineCharIdx-1)
		} else {
			printChar(prevChar, lineIdx-1, len(lines[lineIdx-1]))
		}
	}

	fmt.Println("\nCurrent Character:")
	printChar(char, lineIdx, lineCharIdx)
	fmt.Print("\n")

	if len(contents) > idx+1 {
		fmt.Println("Next Character:")
		var nextChar = next(1)
		if char != '\n' {
			printChar(nextChar, lineIdx, lineCharIdx+1)
		} else {
			printChar(nextChar, lineIdx+1, 0)
		}
		fmt.Print("\n")
	}

	if len(lines) > lineIdx {
		fmt.Printf("Current Line:\n%s\n", lines[lineIdx])
	}
}

func panicDebug(err error) {
	fmt.Println(ansi("###################\n#   DEBUG PANIC   #\n###################\n", bold, red))
	printParsingDebug()
	printShortcutGenDebug()
	printFunctionsDebug()
	printIncludesDebug()
	fmt.Println(ansi("#############################################################\n", bold, red))

	if err != nil {
		panic(err)
	}

	panic("debug")
}

// normalizePlistNumbers recursively converts float64 integers and json.Number to exact uint64/int64
// so plist.Marshal / plist.Unmarshal preserves integer types without 'real' type mismatches.
func normalizePlistNumbers(val any) any {
	if val == nil {
		return nil
	}
	switch v := val.(type) {
	case float64:
		if !math.IsNaN(v) && !math.IsInf(v, 0) && v == math.Floor(v) {
			if v >= 0 && v <= math.MaxUint64 {
				return uint64(v)
			}
			return int64(v)
		}
		return v
	case json.Number:
		s := v.String()
		if strings.ContainsAny(s, ".eE") {
			if f, err := strconv.ParseFloat(s, 64); err == nil {
				if !math.IsNaN(f) && !math.IsInf(f, 0) && f == math.Floor(f) {
					if f >= 0 && f <= math.MaxUint64 {
						return uint64(f)
					}
					return int64(f)
				}
				return f
			}
		}
		if u, err := strconv.ParseUint(s, 10, 64); err == nil {
			return u
		}
		if i, err := strconv.ParseInt(s, 10, 64); err == nil {
			return i
		}
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			return f
		}
		return s
	case map[string]any:
		res := make(map[string]any, len(v))
		for k, item := range v {
			res[k] = normalizePlistNumbers(item)
		}
		return res
	case []any:
		res := make([]any, len(v))
		for i, item := range v {
			res[i] = normalizePlistNumbers(item)
		}
		return res
	default:
		return v
	}
}

// Converts a map[string]interface{} to a matching struct data type safely.
// Never prints raw private user data to stdout on error.
func mapToStruct(data any, structure any) {
	if data == nil || structure == nil {
		return
	}
	normalized := normalizePlistNumbers(data)
	var plistBytes, marshalErr = plist.Marshal(normalized, plist.XMLFormat)
	if marshalErr != nil {
		if args.Using("debug") {
			fmt.Fprintf(os.Stderr, "mapToStruct marshal error: %v\n", marshalErr)
		}
		return
	}

	var _, unmarshalErr = plist.Unmarshal(plistBytes, structure)
	if unmarshalErr != nil {
		// Attempt fallback using Binary plist format
		if binBytes, binErr := plist.Marshal(normalized, plist.BinaryFormat); binErr == nil {
			_, _ = plist.Unmarshal(binBytes, structure)
		} else if args.Using("debug") {
			fmt.Fprintf(os.Stderr, "mapToStruct unmarshal error: %v\n", unmarshalErr)
		}
	}
}

// waitFor takes functions and uses a WaitGroup to wait for them all to finish.
func waitFor(functions ...func()) {
	var wg sync.WaitGroup
	wg.Add(len(functions))
	for _, function := range functions {
		go func() {
			defer wg.Done()
			function()
		}()
	}
	wg.Wait()
}
