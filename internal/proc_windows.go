//go:build windows

package internal

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// IsProcessAlive checks tasklist for the recorded PID. Windows has no /proc
// equivalent, so the launcher state remains the association with Koishi.
func IsProcessAlive(pid int, _ string) bool {
	output, err := exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid), "/FO", "CSV", "/NH").Output()
	return err == nil && strings.Contains(string(output), fmt.Sprintf(",\"%d\",", pid))
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
	return exec.Command("taskkill", args...).Run()
}
