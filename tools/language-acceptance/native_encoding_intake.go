package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"reflect"
	"regexp"
	"strings"
)

var nativeEncodingForms = []string{
	"string", "number", "bool", "attachment(variable)", "attachment(output)", "tokenString", "wrappedSubject",
}

const nativeEncodingNegativeID = "plainString.WFStringParameterState.string"

// Decode complete bounded JSON while rejecting duplicate keys. A later duplicate
// must never hide an earlier contradictory process result or required verdict.
func decodeNativeEncodingObject(data []byte) (map[string]any, error) {
	if len(data) == 0 || len(data) > 4*1024*1024 {
		return nil, fmt.Errorf("empty or oversized encoding evidence")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var read func(int) (any, error)
	read = func(depth int) (any, error) {
		if depth > 64 {
			return nil, fmt.Errorf("encoding evidence nesting is too deep")
		}
		token, err := decoder.Token()
		if err != nil {
			return nil, err
		}
		delim, container := token.(json.Delim)
		if !container {
			return token, nil
		}
		switch delim {
		case '{':
			object := map[string]any{}
			for decoder.More() {
				keyToken, err := decoder.Token()
				if err != nil {
					return nil, err
				}
				key, ok := keyToken.(string)
				if !ok {
					return nil, fmt.Errorf("non-string JSON object key")
				}
				if _, duplicate := object[key]; duplicate {
					return nil, fmt.Errorf("duplicate JSON object key")
				}
				value, err := read(depth + 1)
				if err != nil {
					return nil, err
				}
				object[key] = value
			}
			end, err := decoder.Token()
			if err != nil || end != json.Delim('}') {
				return nil, fmt.Errorf("incomplete JSON object")
			}
			return object, nil
		case '[':
			array := []any{}
			for decoder.More() {
				value, err := read(depth + 1)
				if err != nil {
					return nil, err
				}
				array = append(array, value)
			}
			end, err := decoder.Token()
			if err != nil || end != json.Delim(']') {
				return nil, fmt.Errorf("incomplete JSON array")
			}
			return array, nil
		default:
			return nil, fmt.Errorf("unexpected JSON delimiter")
		}
	}
	value, err := read(0)
	if err != nil {
		return nil, err
	}
	if _, err := decoder.Token(); err != io.EOF {
		return nil, fmt.Errorf("trailing JSON evidence")
	}
	object, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("encoding evidence must be an object")
	}
	return object, nil
}

func nativeEncodingDigest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func nativeEncodingInteger(value any, expected int) bool {
	number, ok := value.(json.Number)
	if !ok {
		return false
	}
	n, err := number.Int64()
	return err == nil && n == int64(expected)
}

func nativeEncodingPositiveInteger(value any) bool {
	number, ok := value.(json.Number)
	if !ok {
		return false
	}
	n, err := number.Int64()
	return err == nil && n > 0
}

func nativeEncodingActual(value any) (string, bool) {
	text, ok := value.(string)
	if !ok {
		return "", false
	}
	switch text {
	case "roundtrip":
		return "accept", true
	case "rejected":
		return "reject", true
	case "class not present", "threw: native parameter-state exception":
		return "unavailable", true
	}
	prefix := "accepted, reserialized as "
	if strings.HasPrefix(text, prefix) && len(text) > len(prefix) {
		return "accept", true
	}
	return "", false
}

func reconcileNativeEncodingArtifacts(record EvidenceRecord, artifacts map[string]EvidenceArtifact, summaryBytes []byte) error {
	fail := func(detail string) error {
		return fmt.Errorf("required encoding record %s: %s", record.EvidenceID, detail)
	}
	summary, err := decodeNativeEncodingObject(summaryBytes)
	if err != nil {
		return fail("invalid summary JSON: " + err.Error())
	}
	canonicalBytes, err := os.ReadFile(resolveRepoPath("tools/shortcut-runtime/required-encoding-expectations.json"))
	if err != nil || nativeEncodingDigest(canonicalBytes) != record.FixtureSHA256 {
		return fail("required expectation fixture is missing or not bound to the record")
	}
	canonical, err := decodeNativeEncodingObject(canonicalBytes)
	if err != nil || !nativeEncodingInteger(canonical["schemaVersion"], 1) {
		return fail("invalid canonical expectation fixture")
	}
	required, ok := canonical["REQUIRED"].([]any)
	if !ok || len(required) != 23 {
		return fail("canonical REQUIRED coverage is incomplete")
	}
	phases, ok := summary["phases"].([]any)
	if !ok || len(phases) != 3 {
		return fail("missing actual process triplet")
	}
	readArtifact := func(name string) ([]byte, error) {
		artifact, present := artifacts[name]
		if !present {
			return nil, fail("missing actual artifact " + name)
		}
		data, err := os.ReadFile(artifact.Path)
		if err != nil || nativeEncodingDigest(data) != artifact.SHA256 {
			return nil, fail("actual artifact bytes do not match their declared digest: " + name)
		}
		return data, nil
	}
	var baselineCommand []any
	for i, name := range []string{"positive", "corrupt", "restored"} {
		phase, ok := phases[i].(map[string]any)
		if !ok {
			return fail("invalid actual phase object")
		}
		processBytes, err := readArtifact(name + ".process.json")
		if err != nil {
			return err
		}
		process, err := decodeNativeEncodingObject(processBytes)
		if err != nil || !reflect.DeepEqual(process, phase) {
			return fail("process receipt differs from the summary phase " + name)
		}
		command, ok := process["command"].([]any)
		wantedLength := 8
		if i == 1 {
			wantedLength = 9
		}
		if !ok || len(command) != wantedLength {
			return fail("missing exact osascript command for " + name)
		}
		for _, item := range command {
			if _, ok := item.(string); !ok {
				return fail("non-string actual command argument")
			}
		}
		if command[0] != "/usr/bin/osascript" || command[1] != "-l" || command[2] != "JavaScript" ||
			path.Base(command[3].(string)) != "verify.js" ||
			path.Base(command[4].(string)) != "parameter-encodings.json" ||
			path.Base(command[5].(string)) != "encoding-table.json" ||
			path.Base(command[6].(string)) != name+".json" ||
			path.Base(command[7].(string)) != "required-encoding-expectations.json" ||
			path.Dir(command[6].(string)) != path.Dir(command[3].(string)) {
			return fail("phase did not invoke the exact real WorkflowKit gate command")
		}
		if i == 0 {
			baselineCommand = command
		}
		for _, index := range []int{0, 1, 2, 3, 4, 5, 7} {
			if command[index] != baselineCommand[index] {
				return fail("phase changed a source or discovery input command")
			}
		}
		if i == 1 && (path.Base(command[8].(string)) != "isolated-corrupt-expectations.json" ||
			path.Dir(command[8].(string)) != path.Dir(command[3].(string))) {
			return fail("negative EXPECT was not an isolated gate input")
		}
		stderr, err := readArtifact(name + ".stderr.log")
		if err != nil {
			return err
		}
		diagnostic := bytes.Contains(stderr, []byte("E_REQUIRED_ENCODING_EXPECTATION")) && bytes.Contains(stderr, []byte(nativeEncodingNegativeID))
		if process["negativeDiagnosticObserved"] != diagnostic || diagnostic != (i == 1) {
			return fail("exact corrupt EXPECT diagnostic is absent or contradicts stderr")
		}
		reportBytes, err := readArtifact(name + ".json")
		if err != nil {
			return err
		}
		report, err := decodeNativeEncodingObject(reportBytes)
		if err != nil {
			return fail("invalid actual per-form report: " + err.Error())
		}
		if err := reconcileNativeEncodingReport(report, required, i == 1); err != nil {
			return fail(name + ": " + err.Error())
		}
	}
	return nil
}

func reconcileNativeEncodingReport(report map[string]any, canonical []any, negative bool) error {
	fail := func(detail string) error { return fmt.Errorf("actual per-form evidence %s", detail) }
	if !nativeEncodingInteger(report["schemaVersion"], 2) || report["engineLoaded"] != true ||
		report["engine"] != "WorkflowKit parameter-state API" {
		return fail("does not identify a loaded WorkflowKit engine")
	}
	forms, ok := report["forms"].([]any)
	if !ok || len(forms) != len(nativeEncodingForms) {
		return fail("is missing input forms")
	}
	wantedForms := map[string]bool{}
	for _, form := range nativeEncodingForms {
		wantedForms[form] = true
	}
	for _, value := range forms {
		form, ok := value.(string)
		if !ok || !wantedForms[form] {
			return fail("has duplicate or unknown forms")
		}
		delete(wantedForms, form)
	}
	results, ok := report["results"].(map[string]any)
	discovery, found := report["discovery"].(map[string]any)
	if !ok || !found || !nativeEncodingPositiveInteger(discovery["parameterStateClasses"]) ||
		!nativeEncodingPositiveInteger(discovery["appIntentStateClasses"]) ||
		!nativeEncodingInteger(discovery["uniqueStateClasses"], len(results)) || len(results) == 0 {
		return fail("does not reconcile actual discovery and its matrix")
	}
	stateName := regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	for state, value := range results {
		if !stateName.MatchString(state) {
			return fail("has invalid state-class identity")
		}
		if value == "class not present" {
			continue
		}
		row, ok := value.(map[string]any)
		if !ok || len(row) != len(nativeEncodingForms) {
			return fail("has an incomplete state-class matrix")
		}
		for _, form := range nativeEncodingForms {
			value, exists := row[form]
			if _, recognized := nativeEncodingActual(value); !exists || !recognized || value == "class not present" {
				return fail("has an invalid per-form observation")
			}
		}
	}
	applied, ok := report["requiredExpectations"].([]any)
	observations, found := report["requiredResults"].([]any)
	if !ok || !found || len(applied) != len(canonical) || len(observations) != len(canonical) {
		return fail("is missing required expectations or observations")
	}
	failures := []any{}
	negativeTargetFound := false
	for i, raw := range canonical {
		original, ok := raw.(map[string]any)
		if !ok || len(original) != 5 {
			return fail("has a malformed independent expectation")
		}
		expected := map[string]any{}
		for key, value := range original {
			expected[key] = value
		}
		if negative && expected["id"] == nativeEncodingNegativeID {
			if expected["expect"] != "accept" {
				return fail("has no independent positive corruption target")
			}
			expected["expect"] = "reject"
			negativeTargetFound = true
		}
		if !reflect.DeepEqual(applied[i], expected) {
			return fail("changed a required EXPECT or its identity")
		}
		observed, ok := observations[i].(map[string]any)
		if !ok || len(observed) != 8 {
			return fail("has an incomplete required observation")
		}
		for _, key := range []string{"id", "kind", "stateClass", "form"} {
			if observed[key] != expected[key] {
				return fail("changed a required observation identity")
			}
		}
		state, stateOK := expected["stateClass"].(string)
		form, formOK := expected["form"].(string)
		if !stateOK || !formOK {
			return fail("has invalid independent expectation types")
		}
		outcome, exists := results[state]
		if row, ok := outcome.(map[string]any); ok {
			outcome, exists = row[form]
		}
		actual, recognized := nativeEncodingActual(outcome)
		if !exists || !recognized {
			return fail("has no actual required per-form result")
		}
		passed := actual == expected["expect"]
		if observed["expected"] != expected["expect"] || observed["actual"] != actual ||
			observed["observation"] != outcome || observed["passed"] != passed {
			return fail("verdict contradicts the actual required form")
		}
		if !passed {
			code := "E_REQUIRED_ENCODING_EXPECTATION"
			if actual == "unavailable" {
				code = "E_REQUIRED_ENCODING_ENGINE"
			}
			failures = append(failures, map[string]any{"code": code, "id": expected["id"], "expected": expected["expect"], "actual": actual})
		}
	}
	if !reflect.DeepEqual(report["failures"], failures) {
		return fail("failure list contradicts required observations")
	}
	if negative {
		wanted := []any{map[string]any{"code": "E_REQUIRED_ENCODING_EXPECTATION", "id": nativeEncodingNegativeID, "expected": "reject", "actual": "accept"}}
		if !negativeTargetFound || report["status"] != "FAIL" || !reflect.DeepEqual(failures, wanted) {
			return fail("did not fail solely for the isolated corrupt EXPECT")
		}
	} else if report["status"] != "PASS" || len(failures) != 0 {
		return fail("did not pass the complete correct required set")
	}
	return nil
}
