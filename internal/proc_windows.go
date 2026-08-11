//go:build windows

package internal

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	processQueryLimitedInformation = 0x1000
	stillActive                    = 259
)

// IsProcessAlive checks the process handle directly instead of parsing
// tasklist output, which can fail with access-denied errors even when the
// recorded PID still exists.
func IsProcessAlive(pid int, _ string) bool {
	if pid <= 0 {
		return false
	}
	handle, err := syscall.OpenProcess(processQueryLimitedInformation, false, uint32(pid))
	if err != nil {
		if err == syscall.ERROR_ACCESS_DENIED {
			return true
		}
		return false
	}
	defer syscall.CloseHandle(handle)

	var exitCode uint32
	if err := syscall.GetExitCodeProcess(handle, &exitCode); err != nil {
		return false
	}
	return exitCode == stillActive
}

func WaitForExit(pid int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !IsProcessAlive(pid, "") {
			return true
		}
		time.Sleep(200 * time.Millisecond)
	}
	return false
}

func taskkill(pid int, force bool) error {
	args := []string{"/PID", strconv.Itoa(pid), "/T"}
	if force {
		args = append(args, "/F")
	}
	output, err := exec.Command("taskkill", args...).CombinedOutput()
	if err != nil {
		detail := strings.TrimSpace(string(output))
		if detail == "" {
			detail = err.Error()
		}
		return fmt.Errorf("taskkill %s failed: %w: %s", strings.Join(args, " "), err, detail)
	}
	return nil
}
