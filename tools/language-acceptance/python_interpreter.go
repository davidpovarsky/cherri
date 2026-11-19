package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"time"
)

const python3Probe = `import json, pathlib, runpy, sys
print(json.dumps({"protocol": "cherri-python3-isolated-v1", "major": sys.version_info.major,
                  "isolated": bool(sys.flags.isolated)}))
`

func validatePython3Probe(output []byte) error {
	var receipt struct {
		Protocol string `json:"protocol"`
		Major    int    `json:"major"`
		Isolated bool   `json:"isolated"`
	}
	if len(output) > 4096 || json.Unmarshal(output, &receipt) != nil ||
		receipt.Protocol != "cherri-python3-isolated-v1" || receipt.Major != 3 || !receipt.Isolated {
		return fmt.Errorf("interpreter did not execute the required isolated Python 3 probe")
	}
	return nil
}

func probePython3Interpreter(path string) error {
	var ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var command = exec.CommandContext(ctx, path, "-I", "-B", "-c", python3Probe)
	command.WaitDelay = time.Second
	var output, err = command.Output()
	if err != nil {
		// An unavailable Windows Store alias is discoverable through LookPath,
		// but it is not a working interpreter. Never publish arbitrary stderr.
		return fmt.Errorf("Python 3 execution probe failed: %v (context: %v)", err, ctx.Err())
	}
	return validatePython3Probe(output)
}

func selectPython3Interpreter(configured string, candidates []string) (string, error) {
	if configured != "" {
		var path, err = exec.LookPath(configured)
		if err != nil {
			return "", fmt.Errorf("configured Python 3 interpreter is unavailable")
		}
		if err := probePython3Interpreter(path); err != nil {
			return "", fmt.Errorf("configured Python 3 interpreter is unusable: %w", err)
		}
		return path, nil
	}
	for _, candidate := range candidates {
		if path, err := exec.LookPath(candidate); err == nil && probePython3Interpreter(path) == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("a working Python 3 interpreter with isolated-mode support is required")
}

func acceptancePython3(configurationEnvironment string) (string, error) {
	var candidates = []string{"python3", "python"}
	if runtime.GOOS == "windows" {
		candidates = []string{"python", "python3"}
	}
	return selectPython3Interpreter(os.Getenv(configurationEnvironment), candidates)
}
