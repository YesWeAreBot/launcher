package internal

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type RunOptions struct {
	Cwd string
	Env map[string]string
	// Stdio selects output handling: "pipe" (default, captured) or "inherit" (streamed to terminal).
	Stdio string
}

type RunResult struct {
	ExitCode int
	Stdout   string
	Stderr   string
}

// CommandRunner is the executable seam for internal commands (git, yarn, node).
// The CLI and initializer never call os/exec directly; tests substitute a fake.
type CommandRunner interface {
	Run(command string, args []string, options RunOptions) (RunResult, error)
}

// NewRunner returns the default CommandRunner backed by os/exec.
func NewRunner() CommandRunner {
	return defaultCommandRunner{}
}

type defaultCommandRunner struct{}

func (defaultCommandRunner) Run(command string, args []string, options RunOptions) (RunResult, error) {
	cmd := exec.Command(command, args...)
	if options.Cwd != "" {
		cmd.Dir = options.Cwd
	}
	// Merge with the parent environment: replacing it would drop PATH and
	// break node/yarn/git resolution inside the child.
	cmd.Env = os.Environ()
	for k, v := range options.Env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}

	var stdout, stderr bytes.Buffer
	if options.Stdio == "inherit" {
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	} else {
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
	}

	err := cmd.Run()
	if err == nil {
		return RunResult{Stdout: strings.TrimSpace(stdout.String()), Stderr: strings.TrimSpace(stderr.String())}, nil
	}

	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		return RunResult{}, fmt.Errorf("failed to run %s: %v", command, err)
	}
	return RunResult{
		ExitCode: exitErr.ExitCode(),
		Stdout:   strings.TrimSpace(stdout.String()),
		Stderr:   strings.TrimSpace(stderr.String()),
	}, nil
}

// Checked runs a command and returns an error with context when it fails.
func Checked(runner CommandRunner, command string, args []string, options RunOptions) (RunResult, error) {
	result, err := runner.Run(command, args, options)
	if err != nil {
		return RunResult{}, err
	}
	if result.ExitCode != 0 {
		detail := result.Stderr
		if detail == "" {
			detail = result.Stdout
		}
		if detail == "" {
			detail = "(no output)"
		}
		return RunResult{}, fmt.Errorf("command failed (exit %d): %s %s\n  cwd: %s\n  %s",
			result.ExitCode, command, strings.Join(args, " "), options.Cwd, detail)
	}
	return result, nil
}

// NodeVersion returns the installed Node.js version string and major
// version, or an error when Node is missing. Preflight-only; never
// invoked by stop/status, which do not execute Node.
func NodeVersion(runner CommandRunner) (version string, major int, err error) {
	result, err := runner.Run("node", []string{"--version"}, RunOptions{})
	if err != nil {
		return "", 0, fmt.Errorf("Node.js is not available\n  Debian/Ubuntu: sudo apt install nodejs npm\n  macOS:         brew install node\n  Windows:       winget install OpenJS.NodeJS.LTS")
	}
	if result.ExitCode != 0 {
		return "", 0, fmt.Errorf("Node.js is not available\n  Debian/Ubuntu: sudo apt install nodejs npm\n  macOS:         brew install node\n  Windows:       winget install OpenJS.NodeJS.LTS")
	}
	version = strings.TrimSpace(result.Stdout)
	version = strings.TrimPrefix(version, "v")
	if dot := strings.Index(version, "."); dot > 0 {
		version = version[:dot]
	}
	if _, err := fmt.Sscanf(version, "%d", &major); err != nil {
		return "", 0, fmt.Errorf("cannot parse Node.js version %q", result.Stdout)
	}
	return strings.TrimSpace(result.Stdout), major, nil
}
