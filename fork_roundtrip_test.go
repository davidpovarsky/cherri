/*
 * Copyright (c) Cherri
 */
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"testing"

	"howett.net/plist"
)

// Automated structural round-trip coverage for fork-specific action behavior:
//
//	Cherri source -> unsigned Shortcut plist -> decompile -> Cherri source -> recompile -> structural compare
//
// The comparison mirrors tools/shortcut-corpus compare.go policy: volatile
// metadata (WFWorkflowClientVersion) is dropped and UUIDs are canonicalized to
// stable placeholders by first appearance during sorted traversal, so only
// genuine semantic differences fail. No Apple device or runtime execution is
// involved; this verifies Shortcut serialization fidelity only.
//
// Phases run through the real cherri CLI in a subprocess so compiler global
// state cannot leak between compile/decompile/recompile, matching the manual
// workflow documented in docs/corpus-platform.md.

var roundTripBinaryOnce sync.Once
var roundTripBinaryPath string
var roundTripBinaryErr error

func roundTripBinary(t *testing.T) string {
	t.Helper()
	roundTripBinaryOnce.Do(func() {
		var dir string
		dir, roundTripBinaryErr = os.MkdirTemp("", "cherri-roundtrip-bin")
		if roundTripBinaryErr != nil {
			return
		}
		roundTripBinaryPath = filepath.Join(dir, "cherri-roundtrip")
		var build *exec.Cmd
		if isWindowsRuntime() {
			roundTripBinaryPath += ".exe"
		}
		build = exec.Command("go", "build", "-o", roundTripBinaryPath, ".")
		var out strings.Builder
		build.Stderr = &out
		if buildErr := build.Run(); buildErr != nil {
			roundTripBinaryErr = fmt.Errorf("go build: %v: %s", buildErr, out.String())
		}
	})
	if roundTripBinaryErr != nil {
		t.Fatalf("build cherri binary: %v", roundTripBinaryErr)
	}
	return roundTripBinaryPath
}

func isWindowsRuntime() bool {
	return os.PathSeparator == '\\'
}

type roundTripRunner struct {
	t      *testing.T
	binary string
	dir    string
}

func newRoundTripRunner(t *testing.T) *roundTripRunner {
	return &roundTripRunner{
		t:      t,
		binary: roundTripBinary(t),
		dir:    t.TempDir(),
	}
}

// runCherri executes one CLI invocation, failing the test on non-zero exit.
func (r *roundTripRunner) runCherri(label string, args ...string) {
	r.t.Helper()
	var cmd = exec.Command(r.binary, args...)
	var out strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		r.t.Fatalf("%s: cherri %s failed: %v\n%s", label, strings.Join(args, " "), err, out.String())
	}
}

// compileSource writes Cherri source and compiles it to an unsigned Shortcut.
func (r *roundTripRunner) compileSource(label string, source string) string {
	r.t.Helper()
	var srcPath = filepath.Join(r.dir, label+".cherri")
	if err := os.WriteFile(srcPath, []byte(source), 0644); err != nil {
		r.t.Fatalf("%s: write source: %v", label, err)
	}
	// --skip-sign always writes <basename>_unsigned.shortcut next to the
	// source; the -o target is only used for the signed artifact.
	r.runCherri(label, srcPath, "--skip-sign", "--no-ansi")
	return filepath.Join(r.dir, label+"_unsigned.shortcut")
}

// decompileShortcut decompiles a Shortcut plist into Cherri source.
func (r *roundTripRunner) decompileShortcut(label string, shortcutPath string) string {
	r.t.Helper()
	var outPath = filepath.Join(r.dir, label+"_b.cherri")
	r.runCherri(label, "--import="+shortcutPath, "-o="+outPath, "--no-ansi")
	var source, err = os.ReadFile(outPath)
	if err != nil {
		r.t.Fatalf("%s: read decompiled source: %v", label, err)
	}
	return string(source)
}

// loadShortcutDocument parses a Shortcut plist into a generic document.
func loadRoundTripDocument(t *testing.T, path string) map[string]any {
	t.Helper()
	var data, err = os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var decoded any
	if _, err = plist.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	doc, ok := decoded.(map[string]any)
	if !ok {
		t.Fatalf("%s: top level is not a dictionary", path)
	}
	return doc
}

var roundTripUUIDPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
var roundTripVolatileTopLevelKeys = map[string]bool{
	"WFWorkflowClientVersion": true,
}

type roundTripCanonicalizer struct {
	uuidOrder []string
	uuidSeen  map[string]string
}

func canonicalizeRoundTripDocument(doc map[string]any) any {
	var c = &roundTripCanonicalizer{uuidSeen: map[string]string{}}
	var cleaned = make(map[string]any, len(doc))
	for key, value := range doc {
		if roundTripVolatileTopLevelKeys[key] {
			continue
		}
		cleaned[key] = c.walk(value)
	}
	return cleaned
}

// canonicalizeDictionaryItems converts an array of serialized dictionary
// items (maps carrying a WFKey) into a map keyed by the resolved item key.
// Serialized plist dictionaries are unordered item arrays built from Go maps
// on both compile paths; index-wise comparison would be brittle against
// serialization map ordering. It returns nil for arrays that are not
// dictionary items so genuine arrays keep array semantics.
func (c *roundTripCanonicalizer) canonicalizeDictionaryItems(items []any) map[string]any {
	if len(items) == 0 {
		return nil
	}
	var keyed = make(map[string]any, len(items))
	for _, item := range items {
		var itemMap, isMap = item.(map[string]any)
		if !isMap {
			return nil
		}
		if _, hasWFKey := itemMap["WFKey"]; !hasWFKey {
			return nil
		}
		if _, hasWFValue := itemMap["WFValue"]; !hasWFValue {
			return nil
		}
		var walkedKey = c.walk(itemMap["WFKey"])
		keyed[resolvedDictionaryKey(walkedKey)] = c.walk(itemMap["WFValue"])
	}
	return keyed
}

// resolvedDictionaryKey extracts the human-readable key from a walked WFKey
// value, which serializes either as a plain string or as a text token.
func resolvedDictionaryKey(walkedKey any) string {
	switch typed := walkedKey.(type) {
	case string:
		return typed
	case map[string]any:
		if value, found := typed["Value"]; found {
			if valueMap, isMap := value.(map[string]any); isMap {
				if stringValue, found := valueMap["string"]; found {
					return fmt.Sprintf("%v", stringValue)
				}
			}
		}
	}
	return describeRoundTrip(walkedKey)
}

func (c *roundTripCanonicalizer) walk(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		var keys = make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)

		var result = make(map[string]any, len(typed))
		for _, key := range keys {
			result[key] = c.walk(typed[key])
		}
		return result
	case []any:
		if keyed := c.canonicalizeDictionaryItems(typed); keyed != nil {
			return keyed
		}
		var result = make([]any, len(typed))
		for index, item := range typed {
			result[index] = c.walk(item)
		}
		return result
	case string:
		if roundTripUUIDPattern.MatchString(typed) {
			if label, seen := c.uuidSeen[typed]; seen {
				return label
			}
			var label = fmt.Sprintf("\uFFFDuuid:%d", len(c.uuidOrder)+1)
			c.uuidSeen[typed] = label
			c.uuidOrder = append(c.uuidOrder, typed)
			return label
		}
		return typed
	default:
		return value
	}
}

// assertStructurallyEqual compares two compiled Shortcut documents under the
// corpus comparator policy and fails with every difference found.
func assertStructurallyEqual(t *testing.T, label string, pathA, pathB string) {
	t.Helper()
	docA := canonicalizeRoundTripDocument(loadRoundTripDocument(t, pathA))
	docB := canonicalizeRoundTripDocument(loadRoundTripDocument(t, pathB))

	var differences []string
	compareRoundTripValues(docA, docB, "", &differences)
	sort.Strings(differences)
	if len(differences) > 0 {
		t.Errorf("%s: structural differences (%d):\n%s", label, len(differences), strings.Join(differences, "\n"))
	}
}

func compareRoundTripValues(a, b any, path string, differences *[]string) {
	if len(*differences) > 50 {
		return
	}
	switch typedA := a.(type) {
	case map[string]any:
		typedB, ok := b.(map[string]any)
		if !ok {
			*differences = append(*differences, fmt.Sprintf("%s: type %T vs %T", path, a, b))
			return
		}
		for _, key := range roundTripUnionKeys(typedA, typedB) {
			valueA, inA := typedA[key]
			valueB, inB := typedB[key]
			childPath := path + "." + key
			switch {
			case !inA:
				*differences = append(*differences, fmt.Sprintf("%s: missing-in-a b=%v", childPath, describeRoundTrip(valueB)))
			case !inB:
				*differences = append(*differences, fmt.Sprintf("%s: missing-in-b a=%v", childPath, describeRoundTrip(valueA)))
			default:
				compareRoundTripValues(valueA, valueB, childPath, differences)
			}
		}
	case []any:
		typedB, ok := b.([]any)
		if !ok {
			*differences = append(*differences, fmt.Sprintf("%s: type %T vs %T", path, a, b))
			return
		}
		if len(typedA) != len(typedB) {
			*differences = append(*differences, fmt.Sprintf("%s.length: %d vs %d", path, len(typedA), len(typedB)))
		}
		var limit = len(typedA)
		if len(typedB) < limit {
			limit = len(typedB)
		}
		for index := 0; index < limit; index++ {
			compareRoundTripValues(typedA[index], typedB[index], fmt.Sprintf("%s[%d]", path, index), differences)
		}
	default:
		if fmt.Sprintf("%v", a) != fmt.Sprintf("%v", b) {
			*differences = append(*differences, fmt.Sprintf("%s: value a=%v b=%v", path, describeRoundTrip(a), describeRoundTrip(b)))
		}
	}
}

func roundTripUnionKeys(a, b map[string]any) []string {
	var set = make(map[string]bool, len(a)+len(b))
	for key := range a {
		set[key] = true
	}
	for key := range b {
		set[key] = true
	}
	var keys = make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func describeRoundTrip(value any) string {
	switch typed := value.(type) {
	case map[string]any:
		var keys = make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		return "object{" + strings.Join(keys, ",") + "}"
	case []any:
		return fmt.Sprintf("array(%d)", len(typed))
	default:
		return fmt.Sprintf("%v", value)
	}
}

// runForkActionRoundTrip executes the full loop for one fixture and asserts:
//   - no rawAction fallback appears for known curated actions
//   - the decompiled source passes each caller-provided assertion
//   - original and recompiled documents are structurally equal
func (r *roundTripRunner) runForkActionRoundTrip(label string, source string, inspect func(decompiled string)) {
	r.t.Helper()
	var shortcutA = r.compileSource(label, source)
	var decompiled = r.decompileShortcut(label, shortcutA)
	if !strings.Contains(decompiled, "rawAction(") {
		// expected for curated actions; asserted explicitly below per fixture
	} else {
		r.t.Errorf("%s: decompiled output degraded to rawAction fallback:\n%s", label, decompiled)
	}
	if inspect != nil {
		inspect(decompiled)
	}
	var sourceBPath = filepath.Join(r.dir, label+"_b.cherri")
	// decompileShortcut already wrote the file; reuse it.
	var shortcutB = filepath.Join(r.dir, label+"_b_unsigned.shortcut")
	r.runCherri(label+"/recompile", sourceBPath, "--skip-sign", "--no-ansi")
	assertStructurallyEqual(r.t, label, shortcutA, shortcutB)
}

// TestForkActionRoundTrips closes the automated round-trip matrix for every
// fork-specific action behavior previously marked PARTIAL. Forms are chosen
// from the provenance evidence notes; interactive/device-only payloads are
// intentionally excluded (see individual comments).
func TestForkActionRoundTrips(t *testing.T) {
	var r = newRoundTripRunner(t)

	t.Run("runJavaScriptOnWebpage/without-input", func(t *testing.T) {
		r.runForkActionRoundTrip("jsweb-noinput", "#include 'actions/web'\n\nrunJavaScriptOnWebpage(\"document.title;\")\n", nil)
	})

	t.Run("runJavaScriptOnWebpage/with-input", func(t *testing.T) {
		r.runForkActionRoundTrip("jsweb-input",
			"#include 'actions/web'\n\n@content = \"batch evidence\"\nrunJavaScriptOnWebpage(\"document.title;\", @content)\n",
			func(decompiled string) {
				if !strings.Contains(decompiled, "runJavaScriptOnWebpage(") {
					t.Errorf("decompiled source lost runJavaScriptOnWebpage call:\n%s", decompiled)
				}
			})
	})

	t.Run("saveFile/legacy-path", func(t *testing.T) {
		r.runForkActionRoundTrip("savefile-legacy",
			"#include 'actions/documents'\n\n@content = \"batch evidence\"\nsaveFile(\"Documents/batch001.txt\", @content, true)\n",
			nil)
	})

	t.Run("saveFile/folder-reference", func(t *testing.T) {
		// Real usage pattern: folder comes from selectFolder(), so WFFolder is a
		// variable reference attachment rather than a device picker payload.
		r.runForkActionRoundTrip("savefile-folder",
			"#include 'actions/documents'\n\n@content = \"batch evidence\"\n@folder = selectFolder()\nsaveFile(\"batch002.txt\", @content, false, @folder)\n",
			nil)
	})

	t.Run("openApp/normal", func(t *testing.T) {
		r.runForkActionRoundTrip("openapp-normal", "openApp(\"com.apple.mobilesafari\")\n", nil)
	})

	t.Run("openApp/slideOver", func(t *testing.T) {
		r.runForkActionRoundTrip("openapp-slideover",
			"openApp(\"com.apple.mobilesafari\", true)\n",
			func(decompiled string) {
				if !strings.Contains(decompiled, ", true)") {
					t.Errorf("slideOver argument missing from decompiled openApp:\n%s", decompiled)
				}
			})
	})

	t.Run("run/modern-dict-and-legacy-name", func(t *testing.T) {
		// workflowIdentifier is a generated placeholder UUID (documented
		// limitation): Shortcuts resolves through workflowName. Both sides carry
		// different placeholder UUIDs at the same position, which the
		// comparator canonicalizes, so this does not hide semantic drift.
		r.runForkActionRoundTrip("run-workflow",
			"run(\"My Workflow\")\n",
			func(decompiled string) {
				if !strings.Contains(decompiled, "run(\"My Workflow\")") {
					t.Errorf("decompiled source lost run target name:\n%s", decompiled)
				}
			})
	})

	t.Run("base64/encode-without-lineBreakMode", func(t *testing.T) {
		assertBase64DecodesAsEncode(t, r, "b64-encode-plain",
			"#include 'actions/crypto'\n\n@input = \"hello\"\n@encoded = base64Encode(@input)\n")
	})

	t.Run("base64/encode-with-lineBreakMode", func(t *testing.T) {
		assertBase64DecodesAsEncode(t, r, "b64-encode-lbm",
			"#include 'actions/crypto'\n\n@input = \"hello\"\n@encoded = base64Encode(@input, \"None\")\n")
	})

	t.Run("base64/decode-without-lineBreakMode", func(t *testing.T) {
		assertBase64DecodesAsDecode(t, r, "b64-decode-plain",
			"#include 'actions/crypto'\n\n@input = \"aGVsbG8=\"\n@decoded = base64Decode(@input)\n")
	})

	t.Run("base64/decode-with-lineBreakMode", func(t *testing.T) {
		assertBase64DecodesAsDecode(t, r, "b64-decode-lbm",
			"#include 'actions/crypto'\n\n@input = \"aGVsbG8=\"\n@decoded = base64Decode(@input, \"Every 64 Characters\")\n")
	})

	t.Run("getUpcomingEvents/empty", func(t *testing.T) {
		r.runForkActionRoundTrip("upcoming-empty", "#include 'actions/calendar'\n\n@events = getUpcomingEvents()\n", nil)
	})

	t.Run("getUpcomingEvents/count-only", func(t *testing.T) {
		r.runForkActionRoundTrip("upcoming-count", "#include 'actions/calendar'\n\n@events = getUpcomingEvents(10)\n", nil)
	})

	t.Run("getUpcomingEvents/dateSpecifier-and-specifiedDate", func(t *testing.T) {
		r.runForkActionRoundTrip("upcoming-dates",
			"#include 'actions/calendar'\n\n@events = getUpcomingEvents(5, \"Specified Day\", \"May 20, 2025\")\n", nil)
	})
}

func assertBase64DecodesAsEncode(t *testing.T, r *roundTripRunner, label string, source string) {
	t.Helper()
	r.runForkActionRoundTrip(label, source, func(decompiled string) {
		if strings.Contains(decompiled, "base64Decode(") {
			t.Errorf("%s: encode payload decompiled as base64Decode:\n%s", label, decompiled)
		}
		if !strings.Contains(decompiled, "base64Encode(") {
			t.Errorf("%s: decompiled source lost base64Encode call:\n%s", label, decompiled)
		}
	})
}

func assertBase64DecodesAsDecode(t *testing.T, r *roundTripRunner, label string, source string) {
	t.Helper()
	r.runForkActionRoundTrip(label, source, func(decompiled string) {
		if strings.Contains(decompiled, "base64Encode(") {
			t.Errorf("%s: decode payload decompiled as base64Encode:\n%s", label, decompiled)
		}
		if !strings.Contains(decompiled, "base64Decode(") {
			t.Errorf("%s: decompiled source lost base64Decode call:\n%s", label, decompiled)
		}
	})
}

var _ = json.Marshal
