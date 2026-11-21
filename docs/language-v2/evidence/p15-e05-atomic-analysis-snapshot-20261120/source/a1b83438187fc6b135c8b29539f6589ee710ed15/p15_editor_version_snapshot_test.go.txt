package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/protocol"
	"github.com/electrikmilk/cherri/internal/language/schema"
)

func p15E05Source(version int) string { return fmt.Sprintf("show(missingVersion%04d)\n", version) }

func p15E05SnapshotError(response protocol.AnalyzeResponse, uri string, maximum int) error {
	if response.URI != uri || response.Version < 1 || response.Version > maximum ||
		response.Valid || response.LanguageVersion != schema.LanguageVersion || len(response.SchemaFingerprint) != 64 ||
		len(response.Diagnostics) != 1 {
		return fmt.Errorf("snapshot envelope differs: %+v", response)
	}
	name := fmt.Sprintf("missingVersion%04d", response.Version)
	diagnostic := response.Diagnostics[0]
	if diagnostic.Code != "E_UNKNOWN_NAME" || diagnostic.Severity != "error" || !strings.Contains(diagnostic.Message, name) ||
		diagnostic.Range.Start.Line != 0 || diagnostic.Range.Start.Character != 5 ||
		diagnostic.Range.End.Line != 0 || diagnostic.Range.End.Character != 5+len(name) {
		return fmt.Errorf("active version%d and diagnostic are from different snapshots: %+v", response.Version, diagnostic)
	}
	return nil
}

func TestP15E05MobileVersionedSnapshot(t *testing.T) {
	const uri = "file:///p15-e05-concurrent-snapshot.cherri"
	const count = 128
	svc := sharedMobileService()
	svc.CloseDocument(uri)
	t.Cleanup(func() { svc.CloseDocument(uri) })
	start := make(chan struct{})
	var next atomic.Int64
	var wait sync.WaitGroup
	responses := make(chan protocol.AnalyzeResponse, count)
	for worker := 0; worker < 8; worker++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			for {
				version := int(next.Add(1))
				if version > count {
					return
				}
				responses <- analyzeVersionedForMobile(uri, version, p15E05Source(version))
			}
		}()
	}
	close(start)
	wait.Wait()
	close(responses)
	observed, mismatches := 0, 0
	for response := range responses {
		observed++
		if err := p15E05SnapshotError(response, uri, count); err != nil {
			mismatches++
			t.Errorf("actual concurrent snapshot mismatch: %v", err)
		}
	}
	if observed != count || svc.DocumentVersion(uri) != count {
		t.Fatalf("lost actual requests: %d, active%d", observed, svc.DocumentVersion(uri))
	}
	t.Logf("P15_E05_ACTUAL_MOBILE_RESPONSES=%d MISMATCHES=%d", observed, mismatches)
}

func TestP15E05MobileStaleAndIndependentDocuments(t *testing.T) {
	const first = "file:///p15-e05-first.cherri"
	const second = "file:///p15-e05-second.cherri"
	svc := sharedMobileService()
	for _, uri := range []string{first, second} {
		svc.CloseDocument(uri)
	}
	t.Cleanup(func() { svc.CloseDocument(first); svc.CloseDocument(second) })
	t.Run("old_edit_returns_latest_snapshot", func(t *testing.T) {
		current := analyzeVersionedForMobile(first, 40, p15E05Source(40))
		if err := p15E05SnapshotError(current, first, 40); err != nil {
			t.Fatal(err)
		}
		stale := analyzeVersionedForMobile(first, 3, "show(otherOldName)")
		if !reflect.DeepEqual(current, stale) {
			t.Fatalf("old edit overwrote current analysis: current%+v stale%+v", current, stale)
		}
	})
	t.Run("another_document_has_independent_version", func(t *testing.T) {
		other := analyzeVersionedForMobile(second, 2, p15E05Source(2))
		if err := p15E05SnapshotError(other, second, 2); err != nil {
			t.Fatal(err)
		}
		current := analyzeVersionedForMobile(first, 4, "show(otherOldName)")
		if err := p15E05SnapshotError(current, first, 40); err != nil {
			t.Fatal(err)
		}
		if current.Version != 40 {
			t.Fatal("document version leaked", current)
		}
	})
	t.Run("returned_diagnostics_are_owned", func(t *testing.T) {
		current := analyzeVersionedForMobile(first, 40, p15E05Source(40))
		current.Diagnostics[0].Message = "forged"
		current.Diagnostics[0].Range.Start.Character = 999
		actual := analyzeVersionedForMobile(first, 1, "show(otherOldName)")
		if err := p15E05SnapshotError(actual, first, 40); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("newest_healthy_edit_clears_errors", func(t *testing.T) {
		actual := analyzeVersionedForMobile(first, 41, "show(\"healthy\")\n")
		if actual.Version != 41 || !actual.Valid || len(actual.Diagnostics) != 0 {
			t.Fatal("healthy recovery failed", actual)
		}
		stale := analyzeVersionedForMobile(first, 40, p15E05Source(40))
		if !reflect.DeepEqual(actual, stale) {
			t.Fatal("old diagnostic replaced healthy state", stale)
		}
	})
	t.Run("close_reopen_resets_version", func(t *testing.T) {
		svc.CloseDocument(first)
		actual := analyzeVersionedForMobile(first, 1, p15E05Source(1))
		if err := p15E05SnapshotError(actual, first, 1); err != nil {
			t.Fatal(err)
		}
	})
}

func TestP15E05MobileSnapshotOracleMutations(t *testing.T) {
	const uri = "file:///p15-e05-snapshot-oracle.cherri"
	sharedMobileService().CloseDocument(uri)
	t.Cleanup(func() { sharedMobileService().CloseDocument(uri) })
	actual := analyzeVersionedForMobile(uri, 7, p15E05Source(7))
	if err := p15E05SnapshotError(actual, uri, 8); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"version", "document", "valid", "language", "schema", "diagnostic_missing", "code", "severity", "message", "start", "end"} {
		t.Run(name, func(t *testing.T) {
			changed := actual
			changed.Diagnostics = append([]protocol.DiagnosticItem{}, actual.Diagnostics...)
			switch name {
			case "version":
				changed.Version = 8
			case "document":
				changed.URI = "file:///other.cherri"
			case "valid":
				changed.Valid = true
			case "language":
				changed.LanguageVersion = "legacy"
			case "schema":
				changed.SchemaFingerprint = ""
			case "diagnostic_missing":
				changed.Diagnostics = nil
			case "code":
				changed.Diagnostics[0].Code = "E_FOREIGN"
			case "severity":
				changed.Diagnostics[0].Severity = "warning"
			case "message":
				changed.Diagnostics[0].Message = "other"
			case "start":
				changed.Diagnostics[0].Range.Start.Character++
			case "end":
				changed.Diagnostics[0].Range.End.Character--
			}
			if p15E05SnapshotError(changed, uri, 8) == nil {
				t.Fatal("mixed snapshot accepted", name)
			}
		})
	}
}

// The portable test executes the actual response producer called by the C ABI.
// This structural control proves delegation, not execution of the C/iOS ABI.
func TestP15E05VersionedCExportDelegatesSnapshot(t *testing.T) {
	raw, err := os.ReadFile("ios_bridge.go")
	if err != nil {
		t.Fatal(err)
	}
	file, err := parser.ParseFile(token.NewFileSet(), "ios_bridge.go", raw, 0)
	if err != nil {
		t.Fatal(err)
	}
	var target *ast.FuncDecl
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == "CherriAnalyzeVersioned" {
			target = fn
		}
	}
	if target == nil {
		t.Fatal("actual export absent")
	}
	calls := 0
	ast.Inspect(target.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		if name, ok := call.Fun.(*ast.Ident); ok && name.Name == "analyzeVersionedForMobile" {
			calls++
		}
		if name, ok := call.Fun.(*ast.SelectorExpr); ok && []string{"ChangeDocument", "Analyze", "DocumentVersion"}[0] == name.Sel.Name {
			t.Error("export bypasses shared snapshot producer")
		}
		return true
	})
	if calls != 1 {
		t.Fatalf("export must call actual shared response producer exactly once, got%d", calls)
	}
}
