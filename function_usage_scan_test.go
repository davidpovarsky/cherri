package main

import (
	"crypto/sha256"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

// Preserve the original lexical contract independently of the implementation.
// This deliberately includes calls inside strings/comments and ASCII suffixes
// after underscores or Unicode; changing those semantics is a separate task.
var p15FunctionUsageOriginalRegex = regexp.MustCompile(`([a-zA-Z0-9]+)\(`)
var p15FunctionUsageASCIIRuns = regexp.MustCompile(`[a-zA-Z0-9]+`)

func p15FunctionUsageOriginal(content string) {
	var matches = p15FunctionUsageOriginalRegex.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		var ref = strings.TrimSpace(match[1])
		if function, found := functions[ref]; found && !function.used {
			functions[ref].used = true
		}
	}
}

func p15FunctionUsageFlags() map[string]bool {
	var result = make(map[string]bool, len(functions))
	for name, function := range functions {
		result[name] = function.used
	}
	return result
}

func TestP15FunctionUsageASCIIContract(t *testing.T) {
	var original = functions
	t.Cleanup(func() { functions = original })
	var names = []string{"foo", "bar", "baz", "Ab9", "123", "9", "cho", "foo_bar", "λfoo", "foó", "already", "unused"}
	for _, tc := range []struct {
		name, source string
		used         []string
	}{
		{"empty", "", nil},
		{"identifier_without_call", "foo", nil},
		{"ordinary", "foo()", []string{"foo"}},
		{"nested", "foo(bar(baz()))", []string{"foo", "bar", "baz"}},
		{"adjacent_and_repeated", "foo()bar(foo())", []string{"foo", "bar"}},
		{"open_at_eof", "foo(", []string{"foo"}},
		{"digit_and_case", "123(Ab9(9(", []string{"123", "Ab9", "9"}},
		{"whitespace_before_open", "foo (bar\t(baz\n(", nil},
		{"underscore_suffix", "foo_bar(_foo(", []string{"bar", "foo"}},
		{"unicode_suffix", "λfoo(foó(écho(", []string{"foo", "cho"}},
		{"invalid_utf8", "\xfffoo(\xe2\x82bar(", []string{"foo", "bar"}},
		{"nul_boundary", "foo\x00bar(", []string{"bar"}},
		{"quoted_calls", "text(\"foo(\") 'bar('", []string{"foo", "bar"}},
		{"comment_calls", "// foo(\n/* bar( */", []string{"foo", "bar"}},
		{"escaped_open", `foo\(`, nil},
		{"punctuation", "@foo(.bar(,baz(", []string{"foo", "bar", "baz"}},
		{"unknown_and_empty_calls", "unknown((()", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			functions = make(map[string]*function, len(names))
			var expected = make(map[string]bool, len(names))
			for _, name := range names {
				functions[name] = &function{used: name == "already"}
				expected[name] = name == "already"
			}
			for _, name := range tc.used {
				expected[name] = true
			}
			checkFunctionUsage(tc.source)
			if got := p15FunctionUsageFlags(); !reflect.DeepEqual(got, expected) {
				t.Fatalf("usage differs for %q: got %v, want %v", tc.source, got, expected)
			}
		})
	}
}

func p15CompareFunctionUsage(t *testing.T, source string) {
	t.Helper()
	var names = map[string]bool{"already": true, "unused": false, source: false}
	// Every ASCII substring is an available function, so incorrect shortening,
	// prefix matches and skipped nested matches cannot disappear as unknowns.
	for _, run := range p15FunctionUsageASCIIRuns.FindAllString(source, -1) {
		for start := range len(run) {
			for end := start + 1; end <= len(run); end++ {
				names[run[start:end]] = false
			}
		}
	}
	var reset = func() {
		functions = make(map[string]*function, len(names))
		for name, used := range names {
			functions[name] = &function{used: used}
		}
	}
	reset()
	p15FunctionUsageOriginal(source)
	var expected = p15FunctionUsageFlags()
	reset()
	checkFunctionUsage(source)
	if got := p15FunctionUsageFlags(); !reflect.DeepEqual(got, expected) {
		t.Fatalf("original regex usage differs for %q: got %v, want %v", source, got, expected)
	}
}

func TestP15FunctionUsageRegexEquivalence(t *testing.T) {
	var original = functions
	t.Cleanup(func() { functions = original })
	var cases int
	var visit func(string, int)
	visit = func(source string, remaining int) {
		p15CompareFunctionUsage(t, source)
		cases++
		if remaining == 0 {
			return
		}
		for _, value := range []byte{'a', 'Z', '9', '_', '(', ' ', 0xff} {
			visit(source+string([]byte{value}), remaining-1)
		}
	}
	visit("", 5)
	for value := 0; value < 256; value++ {
		p15CompareFunctionUsage(t, "foo"+string([]byte{byte(value)})+"bar(")
		p15CompareFunctionUsage(t, "foo("+string([]byte{byte(value)})+"bar(")
		cases += 2
	}
	t.Logf("actual production scan equals original regex used-state for %d bounded public byte strings", cases)
}

func p15FunctionUsagePublicFixtures() map[string]string {
	var fixtures = make(map[string]string)
	var payloadBlock = "QUJDREVGR0hJSktMTU5PUFFSUlNUVldYWVo0123456789+/"
	for _, size := range []int{64 << 10, 1 << 20, 4 << 20} {
		var payload = strings.Repeat(payloadBlock, (size+len(payloadBlock)-1)/len(payloadBlock))[:size]
		fixtures[fmt.Sprintf("payload_%dKiB", size>>10)] = "const data = base64Decode(\"" + payload + "\")\nhelper(nested(data))\n"
	}
	var body = "const value = text(\"PUBLIC\")\nhelper(nested(value))\n// other helper(\n"
	fixtures["function_body_256KiB"] = strings.Repeat(body, (256<<10)/len(body))
	return fixtures
}

func TestP15FunctionUsageLargePublicPayload(t *testing.T) {
	var original = functions
	t.Cleanup(func() { functions = original })
	for name, source := range p15FunctionUsagePublicFixtures() {
		t.Run(name, func(t *testing.T) {
			var expected map[string]bool
			for _, scan := range []func(string){p15FunctionUsageOriginal, checkFunctionUsage} {
				functions = map[string]*function{"helper": {}, "nested": {}, "unused": {}, "already": {used: true}}
				scan(source)
				var got = p15FunctionUsageFlags()
				if expected == nil {
					expected = got
				} else if !reflect.DeepEqual(got, expected) || !got["helper"] || !got["nested"] || got["unused"] || !got["already"] {
					t.Fatal("large public payload changed actual function usage")
				}
			}
			t.Logf("public fixture bytes=%d sha256=%x", len(source), sha256.Sum256([]byte(source)))
		})
	}
}

func BenchmarkP15FunctionUsageScan(b *testing.B) {
	var original = functions
	b.Cleanup(func() { functions = original })
	for name, source := range p15FunctionUsagePublicFixtures() {
		for _, implementation := range []struct {
			name string
			scan func(string)
		}{{"original_regex", p15FunctionUsageOriginal}, {"production", checkFunctionUsage}} {
			b.Run(name+"/"+implementation.name, func(b *testing.B) {
				functions = map[string]*function{"helper": {}, "nested": {}, "unused": {}}
				b.SetBytes(int64(len(source)))
				b.ReportAllocs()
				b.ResetTimer()
				for range b.N {
					implementation.scan(source)
				}
			})
		}
	}
}
