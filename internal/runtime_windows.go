//go:build windows

package internal

import (
	"fmt"
	"os/exec"
)

func configureDaemon(_ *exec.Cmd) {}

func watchForeground(_ *exec.Cmd) func() bool {
	return func() bool { return false }
}

func stopProcess(pid int, _ string) (string, bool, error) {
	if err := taskkill(pid, false); err != nil {
		if !IsProcessAlive(pid, "") {
			return "taskkill", true, nil
		}
		fmt.Printf("  Graceful stop failed, forcing stop\n")
	} else if WaitForExit(pid, gracefulTimeout) {
		return "taskkill", true, nil
	}

	if err := taskkill(pid, true); err != nil && IsProcessAlive(pid, "") {
		return "", false, fmt.Errorf("failed to force terminate PID %d: %v", pid, err)
	}
	return "taskkill /F", false, nil
}
