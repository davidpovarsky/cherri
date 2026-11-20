package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// Execute the production compiler and the actual public-fixture graph guard.
// These controls prove source/causal wiring only; Apple observations are separate.
func TestP15RuntimeDiagnosticFileCarrierControls(t *testing.T) {
	root, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for _, fixture := range []string{
		"qa-file-carrier-text-data-v2",
		"qa-file-carrier-dictionary-data-v2",
		"qa-file-carrier-file-coercion-v2",
	} {
		t.Run(fixture, func(t *testing.T) {
			actions, _ := p15CompileDiagnosticGraph(t, fixture)
			if err := p15DiagnosticReferencesValid(actions); err != nil {
				t.Fatal(err)
			}
			data, err := json.Marshal(actions)
			if err != nil {
				t.Fatal(err)
			}
			graph := filepath.Join(t.TempDir(), "actual-graph.json")
			if err = os.WriteFile(graph, data, 0600); err != nil {
				t.Fatal(err)
			}
			python := os.Getenv("CHERRI_TEST_PYTHON")
			var args []string
			if python == "" {
				python = "python3"
				if isWindowsRuntime() {
					python, args = "py", []string{"-3"}
				}
			}
			const bridge = `import sys,json,pathlib
repo=pathlib.Path(sys.argv[1]); sys.path.insert(0,str(repo/'tools/shortcuts-simulator-qa'))
import file_carrier_controls as c
fid=sys.argv[2]; source=repo/'tools/shortcuts-simulator-qa/fixtures'/(fid+'.cherri')
receipt=c.verify_source(repo,fid,source.read_bytes())
try: c.verify_source(repo,fid,source.read_bytes()+b'\n// changed\n')
except ValueError: pass
else: raise AssertionError('changed source accepted')
actions=json.loads(pathlib.Path(sys.argv[3]).read_bytes())
result=c.verify_graph(fid,actions); mutations=c.verify_causal_mutations(fid,actions)
assert result['appleRuntime']=='NOT_RUN' and result['formalAcceptanceGranted'] is False
assert len(mutations['rejected'])==16
print(json.dumps({'source':receipt,'graph':result,'mutations':mutations}))`
			args = append(args, "-I", "-B", "-c", bridge, root, fixture, graph)
			ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
			defer cancel()
			command := exec.CommandContext(ctx, python, args...)
			output, err := command.CombinedOutput()
			if err != nil {
				t.Fatalf("actual compiler/Python causal guard: %v\n%s", err, output)
			}
			t.Logf("native actions=%d; %s", len(actions), output)
		})
	}
}
