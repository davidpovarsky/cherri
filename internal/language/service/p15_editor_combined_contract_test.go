package service

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/source"
)

const p15RP39URI = "file:///p15-rp39-combined-unicode.cherri"
const p15RP39Symbol = "ערך"
const p15RP39OuterName = "סכום"
const p15RP39InnerName = "פנים"

const p15RP39Unformatted = `// ערך 🙂 é stays
let ערך=7
let literal="ערך 🙂 é"
if true{
let ערך=8
show(f"🙂{ערך}")
}
show(f"שלום🙂{ערך}")
show(ערך)
show(literal)
`

// Mark only the authored declaration and references that may change. These
// expectations do not use the production formatter, lexer or name resolver.
const p15RP39OuterMarked = `// ערך 🙂 é stays
let «ערך» = 7

let literal = "ערך 🙂 é"

if true {
    let ערך = 8
    show(f"🙂{ערך}")
}

show(f"שלום🙂{«ערך»}")

show(«ערך»)

show(literal)
`

const p15RP39InnerMarked = `// ערך 🙂 é stays
let סכום = 7

let literal = "ערך 🙂 é"

if true {
    let «ערך» = 8
    show(f"🙂{«ערך»}")
}

show(f"שלום🙂{סכום}")

show(סכום)

show(literal)
`

type p15RP39Document struct {
	URI     string
	Version int
	Text    string
}

type p15RP39Analysis struct {
	Version     int
	Fingerprint string
	Diagnostics []analysis.Diagnostic
}

type p15RP39Trace struct {
	FirstFormat, SecondFormat, OuterText, InnerText string
	Fingerprint                                     string
	OuterEdits, InnerEdits                          []TextEdit
	OuterDefinition, InnerDefinition                *source.Span
	AfterFirstFormat, AfterSecondFormat             p15RP39Document
	AfterOuterRename, AfterInnerRename              p15RP39Document
	AfterStale, AfterRecovery                       p15RP39Document
	Old, New, Stale, DelayedOld, Recovery           p15RP39Analysis
}

func p15RP39DocumentState(svc *Service) p15RP39Document {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	var doc = svc.documents[p15RP39URI]
	if doc == nil || doc.File == nil {
		return p15RP39Document{}
	}
	return p15RP39Document{doc.URI, doc.Version, doc.File.Content}
}

func p15RP39Fixture() (formatted, outer, final string, outerOffsets, innerOffsets []int) {
	formatted, outer, outerOffsets = p15E03MarkedSource(p15RP39OuterMarked, p15RP39Symbol, p15RP39OuterName)
	var innerInput string
	innerInput, final, innerOffsets = p15E03MarkedSource(p15RP39InnerMarked, p15RP39Symbol, p15RP39InnerName)
	if innerInput != outer {
		panic("RP39 independent same-document fixtures disagree")
	}
	return formatted, outer, final, outerOffsets, innerOffsets
}

// One real Service and URI own every operation below. No E03/E04/E05 receipt or
// synthesized analysis result supplies this distinct combined scenario.
func p15RP39Observe(t *testing.T) p15RP39Trace {
	t.Helper()
	var formatted, _, final, outerOffsets, innerOffsets = p15RP39Fixture()
	var svc = NewService(nil)
	svc.OpenDocument(p15RP39URI, 1, p15RP39Unformatted)
	p15E03Healthy(t, svc, p15RP39URI)
	var trace = p15RP39Trace{Fingerprint: svc.SchemaFingerprint()}
	var err error
	trace.FirstFormat, err = svc.Format(p15RP39URI)
	if err != nil {
		t.Fatal(err)
	}
	trace.AfterFirstFormat = p15RP39DocumentState(svc)
	svc.ChangeDocument(p15RP39URI, 2, trace.FirstFormat)
	trace.SecondFormat, err = svc.Format(p15RP39URI)
	if err != nil {
		t.Fatal(err)
	}
	trace.AfterSecondFormat = p15RP39DocumentState(svc)
	// Select the outer reference after an emoji, so byte/rune/UTF-16 positions
	// differ and the existing independent span oracle must join all of them.
	var pos = p15EditorExpectedPosition(formatted, outerOffsets[1])
	trace.OuterDefinition = svc.Definition(p15RP39URI, pos.Line, pos.Column)
	trace.OuterText, trace.OuterEdits, err = svc.Rename(p15RP39URI, pos.Line, pos.Column, p15RP39OuterName)
	if err != nil {
		t.Fatal(err)
	}
	trace.AfterOuterRename = p15RP39DocumentState(svc)
	svc.ChangeDocument(p15RP39URI, 3, trace.OuterText)
	p15E03Healthy(t, svc, p15RP39URI)
	pos = p15EditorExpectedPosition(trace.OuterText, innerOffsets[1])
	trace.InnerDefinition = svc.Definition(p15RP39URI, pos.Line, pos.Column)
	trace.InnerText, trace.InnerEdits, err = svc.Rename(p15RP39URI, pos.Line, pos.Column, p15RP39InnerName)
	if err != nil {
		t.Fatal(err)
	}
	trace.AfterInnerRename = p15RP39DocumentState(svc)
	svc.ChangeDocument(p15RP39URI, 4, trace.InnerText)
	p15E03Healthy(t, svc, p15RP39URI)
	var analyze = func(version int, text string) p15RP39Analysis {
		var diagnostics, fingerprint, observedVersion = svc.AnalyzeVersioned(p15RP39URI, version, text)
		return p15RP39Analysis{observedVersion, fingerprint, diagnostics}
	}
	var oldText = final + "\nshow(missingOld)\n"
	var newText = final + "\nshow(missingNewer)\n"
	// Preserve an independent copy before a later service call could mutate a
	// shared diagnostics slice. The delayed value remains the actual response.
	var delayed = analyze(5, oldText)
	var savedOld []byte
	savedOld, err = json.Marshal(delayed)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(savedOld, &trace.Old); err != nil {
		t.Fatal(err)
	}
	// Observe the delayed old response after the new response completes. The
	// service owns its version; actual UI publication is separately pending.
	trace.New = analyze(6, newText)
	trace.Stale = analyze(5, oldText)
	trace.DelayedOld = delayed
	trace.AfterStale = p15RP39DocumentState(svc)
	trace.Recovery = analyze(7, final)
	svc.ChangeDocument(p15RP39URI, 6, newText)
	trace.AfterRecovery = p15RP39DocumentState(svc)
	return trace
}

func p15RP39TraceError(trace p15RP39Trace) error {
	var formatted, outer, final, outerOffsets, innerOffsets = p15RP39Fixture()
	var fail = func(code string) error { return fmt.Errorf("%s", code) }
	if trace.FirstFormat != formatted {
		return fail("RP39_FORMAT_FIRST")
	}
	if trace.SecondFormat != formatted || trace.SecondFormat != trace.FirstFormat {
		return fail("RP39_FORMAT_TWICE")
	}
	if trace.AfterFirstFormat != (p15RP39Document{p15RP39URI, 1, p15RP39Unformatted}) ||
		trace.AfterSecondFormat != (p15RP39Document{p15RP39URI, 2, formatted}) {
		return fail("RP39_FORMAT_DOCUMENT")
	}
	if err := p15E03RenameError(formatted, p15RP39URI, p15RP39Symbol, p15RP39OuterName, trace.OuterText, outer, trace.OuterEdits, outerOffsets); err != nil {
		return fail("RP39_OUTER_RENAME")
	}
	if err := p15E03RenameError(outer, p15RP39URI, p15RP39Symbol, p15RP39InnerName, trace.InnerText, final, trace.InnerEdits, innerOffsets); err != nil {
		return fail("RP39_INNER_RENAME")
	}
	var definition = func(text string, offsets []int, actual *source.Span) bool {
		if actual == nil {
			return false
		}
		var expected = source.Span{Document: source.DocumentID(p15RP39URI),
			Start: p15EditorExpectedPosition(text, offsets[0]),
			End:   p15EditorExpectedPosition(text, offsets[0]+len(p15RP39Symbol))}
		return *actual == expected
	}
	if !definition(formatted, outerOffsets, trace.OuterDefinition) || !definition(outer, innerOffsets, trace.InnerDefinition) {
		return fail("RP39_DEFINITION_OWNER")
	}
	if trace.AfterOuterRename != (p15RP39Document{p15RP39URI, 2, formatted}) ||
		trace.AfterInnerRename != (p15RP39Document{p15RP39URI, 3, outer}) {
		return fail("RP39_RENAME_DOCUMENT")
	}
	if len(trace.Fingerprint) != 64 {
		return fail("RP39_ANALYSIS_SCHEMA")
	}
	for _, response := range []p15RP39Analysis{trace.Old, trace.New, trace.Stale, trace.DelayedOld, trace.Recovery} {
		if response.Fingerprint != trace.Fingerprint {
			return fail("RP39_ANALYSIS_SCHEMA")
		}
	}
	var checkAnalysis = func(actual p15RP39Analysis, version int, text, missing string) bool {
		if actual.Version != version || len(actual.Diagnostics) != 1 {
			return false
		}
		var wanted = []p15EditorDiagnosticRequirement{{analysis.CodeUnknownName, missing}}
		return p15E02DiagnosticError(text, p15RP39URI, actual.Diagnostics, wanted, false, 1) == nil
	}
	var oldText = final + "\nshow(missingOld)\n"
	var newText = final + "\nshow(missingNewer)\n"
	if !checkAnalysis(trace.Old, 5, oldText, "missingOld") {
		return fail("RP39_OLD_SNAPSHOT")
	}
	if !checkAnalysis(trace.New, 6, newText, "missingNewer") {
		return fail("RP39_NEW_SNAPSHOT")
	}
	if !checkAnalysis(trace.Stale, 6, newText, "missingNewer") || !reflect.DeepEqual(trace.Stale, trace.New) {
		return fail("RP39_STALE_SNAPSHOT")
	}
	if !reflect.DeepEqual(trace.DelayedOld, trace.Old) || trace.DelayedOld.Version >= trace.New.Version {
		return fail("RP39_DELAYED_OLD_RESPONSE")
	}
	if trace.AfterStale != (p15RP39Document{p15RP39URI, 6, newText}) {
		return fail("RP39_STALE_ACTIVE_DOCUMENT")
	}
	if trace.Recovery.Version != 7 || len(trace.Recovery.Diagnostics) != 0 {
		return fail("RP39_RECOVERY")
	}
	if trace.AfterRecovery != (p15RP39Document{p15RP39URI, 7, final}) {
		return fail("RP39_RECOVERY_ACTIVE_DOCUMENT")
	}
	return nil
}

func TestP15RP39CombinedDocumentService(t *testing.T) {
	if err := p15RP39TraceError(p15RP39Observe(t)); err != nil {
		t.Fatal(err)
	}
}

func TestP15RP39CombinedDocumentOracleMutations(t *testing.T) {
	var original = p15RP39Observe(t)
	if err := p15RP39TraceError(original); err != nil {
		t.Fatal(err)
	}
	var raw, err = json.Marshal(original)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ fault, code string }{
		{"format_first", "RP39_FORMAT_FIRST"},
		{"format_not_idempotent", "RP39_FORMAT_TWICE"},
		{"format_changes_document", "RP39_FORMAT_DOCUMENT"},
		{"wrong_scope", "RP39_OUTER_RENAME"},
		{"literal_changed", "RP39_OUTER_RENAME"},
		{"comment_changed", "RP39_OUTER_RENAME"},
		{"missing_edit", "RP39_OUTER_RENAME"},
		{"byte_span", "RP39_OUTER_RENAME"},
		{"utf16_span", "RP39_OUTER_RENAME"},
		{"foreign_edit_document", "RP39_OUTER_RENAME"},
		{"inner_changes_outer", "RP39_INNER_RENAME"},
		{"wrong_definition", "RP39_DEFINITION_OWNER"},
		{"rename_changes_document", "RP39_RENAME_DOCUMENT"},
		{"schema", "RP39_ANALYSIS_SCHEMA"},
		{"old_relabelled_new", "RP39_OLD_SNAPSHOT"},
		{"new_uses_old_diagnostic", "RP39_NEW_SNAPSHOT"},
		{"stale_returns_old", "RP39_STALE_SNAPSHOT"},
		{"stale_drops_error", "RP39_STALE_SNAPSHOT"},
		{"delayed_old_mutated", "RP39_DELAYED_OLD_RESPONSE"},
		{"stale_replaces_active", "RP39_STALE_ACTIVE_DOCUMENT"},
		{"recovery_retains_error", "RP39_RECOVERY"},
		{"lower_version_overwrites_recovery", "RP39_RECOVERY_ACTIVE_DOCUMENT"},
	} {
		t.Run(tc.fault, func(t *testing.T) {
			var changed p15RP39Trace
			if err := json.Unmarshal(raw, &changed); err != nil {
				t.Fatal(err)
			}
			if err := p15RP39TraceError(changed); err != nil {
				t.Fatal("unchanged actual trace clone rejected", err)
			}
			switch tc.fault {
			case "format_first":
				changed.FirstFormat += " "
			case "format_not_idempotent":
				changed.SecondFormat += " "
			case "format_changes_document":
				changed.AfterFirstFormat.Text = changed.FirstFormat
			case "wrong_scope":
				changed.OuterEdits[0] = changed.InnerEdits[0]
			case "literal_changed":
				changed.OuterText = strings.Replace(changed.OuterText, `"ערך 🙂 é"`, `"סכום 🙂 é"`, 1)
			case "comment_changed":
				changed.OuterText = strings.Replace(changed.OuterText, "// ערך", "// סכום", 1)
			case "missing_edit":
				changed.OuterEdits = changed.OuterEdits[:len(changed.OuterEdits)-1]
			case "byte_span":
				changed.OuterEdits[1].Span.Start.ByteOffset++
			case "utf16_span":
				changed.OuterEdits[1].Span.Start.UTF16Column = changed.OuterEdits[1].Span.Start.Column
			case "foreign_edit_document":
				changed.OuterEdits[1].Span.Document = "file:///other.cherri"
			case "inner_changes_outer":
				changed.InnerText = strings.Replace(changed.InnerText, "let סכום", "let פנים", 1)
			case "wrong_definition":
				changed.OuterDefinition = changed.InnerDefinition
			case "rename_changes_document":
				changed.AfterOuterRename.Text = changed.OuterText
			case "schema":
				changed.New.Fingerprint = strings.Repeat("e", 64)
			case "old_relabelled_new":
				changed.Old.Version = 6
			case "new_uses_old_diagnostic":
				changed.New.Diagnostics = changed.Old.Diagnostics
			case "stale_returns_old":
				changed.Stale = changed.Old
			case "stale_drops_error":
				changed.Stale.Diagnostics = nil
			case "delayed_old_mutated":
				changed.DelayedOld = changed.New
			case "stale_replaces_active":
				changed.AfterStale.Text = changed.InnerText + "\nshow(missingOld)\n"
			case "recovery_retains_error":
				changed.Recovery.Diagnostics = changed.New.Diagnostics
			case "lower_version_overwrites_recovery":
				changed.AfterRecovery = changed.AfterStale
			default:
				t.Fatal("unknown mutation", tc.fault)
			}
			var err = p15RP39TraceError(changed)
			if err == nil || err.Error() != tc.code {
				t.Fatalf("%s: got %v, want %s", tc.fault, err, tc.code)
			}
		})
	}
}
