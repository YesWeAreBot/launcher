//go:build !windows

package internal

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
)

func configureDaemon(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}

func watchForeground(cmd *exec.Cmd) func() bool {
	var stopped atomic.Bool
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-sigChan:
				stopped.Store(true)
				_ = cmd.Process.Signal(os.Interrupt)
			case <-done:
				return
			}
		}
	}()
	return func() bool {
		close(done)
		signal.Stop(sigChan)
		return stopped.Load()
	}
}

func stopProcess(pid int, mode string) (string, bool, error) {
	sig, name := syscall.SIGTERM, "SIGTERM"
	if mode == "foreground" {
		sig, name = syscall.SIGINT, "SIGINT"
	}
	if err := signalProcess(pid, sig); err != nil {
		return "", false, fmt.Errorf("failed to send %s to PID %d: %v", name, pid, err)
	}
	fmt.Printf("  Sent %s to PID %d...\n", name, pid)
	if WaitForExit(pid, gracefulTimeout) {
		return name, true, nil
	}
	fmt.Printf("  Process did not exit within %ds, sending SIGKILL...\n", int(gracefulTimeout.Seconds()))
	if err := signalProcess(pid, syscall.SIGKILL); err != nil {
		return "", false, fmt.Errorf("failed to send SIGKILL to PID %d: %v", pid, err)
	}
	return "SIGKILL", false, nil
}

func signalProcess(pid int, sig syscall.Signal) error {
	if runtimeGOOSLinux() {
		if pgid := processGroup(pid); pgid > 0 {
			return syscall.Kill(-pgid, sig)
		}
	}
	return syscall.Kill(pid, sig)
}

func runtimeGOOSLinux() bool { return runtime.GOOS == "linux" }

func processGroup(pid int) int {
	stat, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return 0
	}
	i := strings.LastIndex(string(stat), ")")
	if i < 0 {
		return 0
	}
	rest := strings.Fields(string(stat)[i+1:])
	if len(rest) < 3 {
		return 0
	}
	pgid, err := strconv.Atoi(rest[2])
	if err != nil || pgid <= 0 {
		return 0
	}
	return pgid
}
