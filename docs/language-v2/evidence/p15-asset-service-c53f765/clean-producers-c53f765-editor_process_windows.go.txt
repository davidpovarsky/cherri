//go:build windows

package main

import (
	"context"
	"os"
	"os/exec"
	"strconv"
	"time"
)

// Cancellation owns only the process tree started for this scenario.
func configureEditorProductionProcess(command *exec.Cmd) {
	command.WaitDelay = 5 * time.Second
	command.Cancel = func() error {
		if command.Process == nil {
			return os.ErrProcessDone
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		cleanup := exec.CommandContext(ctx, "taskkill.exe", "/PID", strconv.Itoa(command.Process.Pid), "/T", "/F")
		if err := cleanup.Run(); err != nil {
			return command.Process.Kill()
		}
		return nil
	}
}
