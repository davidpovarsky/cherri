package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// Diagnostic observation only. With no explicit output it is dormant. It never
// substitutes command results, source copies, tool bytes or acceptance checks.
var p15PhaseObserver = struct {
	sync.Mutex
	next, writes uint64
	overheadNS   int64
}{}

type p15PhaseObservation struct {
	directory, prefix string
	row               map[string]any
	operationStart    time.Time
	preObservationNS  int64
	ended             bool
}

func p15PhaseSelected(t *testing.T) bool {
	if os.Getenv("CHERRI_P15_PHASE_OUTPUT") == "" {
		return false
	}
	parent := strings.SplitN(t.Name(), "/", 2)[0]
	switch parent {
	case "TestP10_IsolatedCompilerMutationGate", "TestP15CanonicalBehaviorSourcePropagation",
		"TestP15RP45ExtractedInstallAndRollback", "TestP15RP42ExtractedInstallPublicCLI":
		return true
	}
	return false
}

func p15PhaseAccount(elapsed time.Duration, writes uint64) {
	p15PhaseObserver.Lock()
	p15PhaseObserver.overheadNS += elapsed.Nanoseconds()
	p15PhaseObserver.writes += writes
	p15PhaseObserver.Unlock()
}

func p15PhaseWrite(t *testing.T, path string, raw []byte) {
	t.Helper()
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		t.Errorf("phase observation exclusive file: %v", err)
		return
	}
	n, writeErr := file.Write(raw)
	closeErr := file.Close()
	if writeErr == nil && n != len(raw) {
		writeErr = io.ErrShortWrite
	}
	if writeErr != nil || closeErr != nil {
		t.Errorf("phase observation original write: %v; close: %v", writeErr, closeErr)
	}
}

func p15PhaseJSON(t *testing.T, path string, value any) {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Errorf("phase observation JSON: %v", err)
		return
	}
	p15PhaseWrite(t, path, append(raw, '\n'))
}

func p15PhaseBegin(t *testing.T, name string, detail map[string]any) *p15PhaseObservation {
	t.Helper()
	if !p15PhaseSelected(t) {
		return nil
	}
	observerStart := time.Now()
	directory, source := os.Getenv("CHERRI_P15_PHASE_OUTPUT"), os.Getenv("CHERRI_P15_PHASE_SOURCE")
	validSource := len(source) == 40
	for _, ch := range source {
		if !strings.ContainsRune("0123456789abcdef", ch) {
			validSource = false
		}
	}
	physical, err := filepath.EvalSymlinks(directory)
	root, rootErr := os.Getwd()
	physicalRoot, physicalErr := filepath.EvalSymlinks(root)
	info, statErr := os.Lstat(directory)
	relative, relErr := filepath.Rel(physicalRoot, physical)
	if !validSource || !filepath.IsAbs(directory) || err != nil || physical != directory ||
		rootErr != nil || physicalErr != nil || statErr != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 ||
		relErr != nil || relative == "." || (relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))) {
		t.Fatal("phase observation requires an exact source SHA and owned physical directory outside the checkout")
	}
	p15PhaseObserver.Lock()
	p15PhaseObserver.next++
	id := p15PhaseObserver.next
	p15PhaseObserver.Unlock()
	prefix := fmt.Sprintf("pid%d-phase%06d", os.Getpid(), id)
	row := map[string]any{"schema": "cherri.p15.phase-observation/v1", "id": id,
		"pid": os.Getpid(), "test": t.Name(), "phase": name, "source_commit": source,
		"observer_started_at": observerStart.UTC().Format(time.RFC3339Nano),
		"detail":              detail, "diagnostic_only": true, "formal_acceptance_granted": false}
	p15PhaseJSON(t, filepath.Join(directory, prefix+".start.json"), row)
	operationStart := time.Now()
	pre := operationStart.Sub(observerStart)
	p15PhaseAccount(pre, 1)
	return &p15PhaseObservation{directory: directory, prefix: prefix, row: row,
		operationStart: operationStart, preObservationNS: pre.Nanoseconds()}
}

func (p *p15PhaseObservation) End(t *testing.T, result map[string]any, originals map[string][]byte) {
	t.Helper()
	if p == nil {
		return
	}
	operationEnd := time.Now()
	if p.ended {
		t.Error("phase observation ended twice")
		return
	}
	p.ended = true
	retained := map[string]any{}
	writes := uint64(1)
	for name, raw := range originals {
		if name != "combined" && name != "stdout" && name != "stderr" && name != "native" {
			t.Error("phase observation unexpected original kind")
			continue
		}
		if len(raw) > 64<<20 {
			t.Error("phase observation original exceeds 64 MiB")
			continue
		}
		filename := p.prefix + "." + name + ".original"
		p15PhaseWrite(t, filepath.Join(p.directory, filename), raw)
		retained[name] = map[string]any{"path": filename, "bytes": len(raw), "sha256": fmt.Sprintf("%x", sha256.Sum256(raw))}
		writes++
	}
	p.row["operation_started_at"] = p.operationStart.UTC().Format(time.RFC3339Nano)
	p.row["operation_finished_at"] = operationEnd.UTC().Format(time.RFC3339Nano)
	p.row["operation_elapsed_ns"] = operationEnd.Sub(p.operationStart).Nanoseconds()
	p.row["operation_elapsed_includes_result_and_argument_preparation"] = true
	p.row["operation_timer_boundary"] = "begin receipt finished to End entry; not exact subprocess-only duration"
	p.row["observer_before_ns"] = p.preObservationNS
	p.row["observer_after_before_end_write_ns"] = time.Since(operationEnd).Nanoseconds()
	p.row["result"], p.row["originals"] = result, retained
	p.row["test_failed_when_observed"] = t.Failed()
	p15PhaseJSON(t, filepath.Join(p.directory, p.prefix+".end.json"), p.row)
	p15PhaseAccount(time.Since(operationEnd), writes)
}

func (p *p15PhaseObservation) ScopeEnd(t *testing.T) {
	p.End(t, map[string]any{"scope_end_reached": true, "operation_success_inferred": false}, nil)
}

func (p *p15PhaseObservation) ValueEnd(t *testing.T, err error, originals map[string][]byte) {
	t.Helper()
	if p == nil {
		return
	}
	result := map[string]any{"returned": true, "error": nil}
	if err != nil {
		result["error"] = err.Error()
	}
	p.End(t, result, originals)
}

func p15PhaseCommand(t *testing.T, name string, command *exec.Cmd) *p15PhaseObservation {
	t.Helper()
	return p15PhaseBegin(t, name, map[string]any{"argv": append([]string(nil), command.Args...), "cwd": command.Dir})
}

func (p *p15PhaseObservation) CommandEnd(t *testing.T, command *exec.Cmd, err error, originals map[string][]byte) {
	t.Helper()
	if p == nil {
		return
	}
	result := map[string]any{"returned": true, "error": nil, "exit_code": nil}
	if err != nil {
		result["error"] = err.Error()
	}
	if command.ProcessState != nil {
		result["exit_code"] = command.ProcessState.ExitCode()
	}
	p.End(t, result, originals)
}

func p15PhaseCombinedOutput(t *testing.T, name string, command *exec.Cmd) ([]byte, error) {
	t.Helper()
	phase := p15PhaseCommand(t, name, command)
	out, err := command.CombinedOutput()
	phase.CommandEnd(t, command, err, map[string][]byte{"combined": out})
	return out, err
}

func p15PhaseParent(t *testing.T) func() {
	t.Helper()
	if !p15PhaseSelected(t) {
		return func() {}
	}
	p15PhaseObserver.Lock()
	beforeNS, beforeWrites := p15PhaseObserver.overheadNS, p15PhaseObserver.writes
	p15PhaseObserver.Unlock()
	body := p15PhaseBegin(t, "parent_body", nil)
	var cleanup *p15PhaseObservation
	// Registered before existing fixtures: LIFO executes this after their real
	// cleanup callbacks. The returned defer starts timing after the body defers.
	t.Cleanup(func() {
		cleanup.ScopeEnd(t)
		p15PhaseObserver.Lock()
		ns, writes := p15PhaseObserver.overheadNS-beforeNS, p15PhaseObserver.writes-beforeWrites
		p15PhaseObserver.Unlock()
		row, err := json.Marshal(map[string]any{"schema": "cherri.p15.phase-observer-cost/v1",
			"test": t.Name(), "observer_elapsed_ns": ns, "original_file_writes": writes,
			"final_log_emission_excluded": true, "counter_bookkeeping_excluded": true,
			"selection_checks_excluded": true, "diagnostic_only": true})
		if err != nil {
			t.Error(err)
			return
		}
		t.Logf("P15_PHASE_OBSERVER_TOTAL %s", row)
	})
	return func() {
		body.ScopeEnd(t)
		cleanup = p15PhaseBegin(t, "parent_cleanup", nil)
	}
}
