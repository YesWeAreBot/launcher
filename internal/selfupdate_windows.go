//go:build windows

package internal

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

func installBinary(source, target string) error {
	script := filepath.Join(filepath.Dir(target), ".yesimbot-cli-update.cmd")
	content := "@echo off\r\n" +
		"ping -n 2 127.0.0.1 >nul\r\n" +
		"move /Y \"" + source + "\" \"" + target + "\"\r\n" +
		"del \"%~f0\"\r\n"
	if err := os.WriteFile(script, []byte(content), 0o644); err != nil {
		return fmt.Errorf("failed to write update script: %w", err)
	}
	cmd := exec.Command("cmd", "/c", script)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: 0x00000008 | 0x08000000,
	}
	return cmd.Start()
}
