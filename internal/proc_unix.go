//go:build !windows

package internal

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
)

// IsProcessAlive reports whether pid exists and belongs to our Koishi
// instance: it must run from the App directory and its command line must
// reference koishi. Both checks guard against PID reuse killing an
// unrelated process.
func IsProcessAlive(pid int, expectedCwd string) bool {
	if err := syscall.Kill(pid, 0); err != nil {
		return false
	}
	if runtime.GOOS != "linux" {
		return true
	}
	cwd, err := os.Readlink(fmt.Sprintf("/proc/%d/cwd", pid))
	if err != nil || filepath.Clean(cwd) != filepath.Clean(expectedCwd) {
		return false
	}
	raw, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
	return err == nil && strings.Contains(strings.ReplaceAll(string(raw), "\x00", " "), "koishi")
}

// WaitForExit polls pid until it exits or the timeout elapses.
func WaitForExit(pid int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if err := syscall.Kill(pid, 0); err != nil {
			return true
		}
		time.Sleep(200 * time.Millisecond)
	}
	return false
}
