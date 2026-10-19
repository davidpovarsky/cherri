package main

import "os/exec"

// resolvePythonCommand returns the available python executable on PATH ("python" or "python3").
func resolvePythonCommand() string {
	if p, err := exec.LookPath("python"); err == nil && p != "" {
		return "python"
	}
	if p3, err := exec.LookPath("python3"); err == nil && p3 != "" {
		return "python3"
	}
	return "python"
}
