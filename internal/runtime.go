package internal

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const gracefulTimeout = 10 * time.Second

type StartOptions struct {
	AppDir string
	Daemon bool
}

type StopOptions struct {
	AppDir string
}

type StartResult struct {
	Pid  int
	Mode string // "foreground" or "daemon"
}

type StopResult struct {
	Pid      int
	Signal   string
	Graceful bool
}

type RuntimeStatus struct {
	Initialized bool
	Running     bool
	Pid         int
	Mode        string
	AppDir      string
	StartedAt   string
	StoppedAt   string
	SourceHead  string
	UpdatedAt   string
	ConsoleURL  string
	LogFile     string
}

// Start launches the Koishi child. Daemon mode returns after spawning;
// foreground mode blocks until the child exits.
func Start(opts StartOptions) (StartResult, error) {
	if err := AssertIsKoishiApp(opts.AppDir); err != nil {
		return StartResult{}, err
	}
	paths := Derive(opts.AppDir)
	state := ReadState(paths)

	if state.Pid > 0 && IsProcessAlive(state.Pid, paths.AppDir) {
		return StartResult{}, fmt.Errorf("YesImBot is already running (PID %d, mode: %s)\n  Use \"yesimbot-cli stop\" to stop it first.",
			state.Pid, state.Mode)
	}

	// Preflight: start executes Node, so Node must be present.
	if _, _, err := NodeVersion(NewRunner()); err != nil {
		return StartResult{}, err
	}

	entry := resolveKoishiBin(paths.AppDir)
	if _, err := os.Stat(entry); err != nil {
		return StartResult{}, fmt.Errorf("Koishi CLI entry not found: %s\n  Run \"yesimbot-cli init\" first, or check that dependencies are installed.",
			entry)
	}
	if err := os.MkdirAll(paths.LogsDir, 0o755); err != nil {
		return StartResult{}, fmt.Errorf("failed to create log directory: %v", err)
	}
	mode := "foreground"
	if opts.Daemon {
		mode = "daemon"
	}
	separator := fmt.Sprintf("\n%s\n[%s] Starting Koishi (%s)\n%s\n",
		strings.Repeat("=", 60),
		time.Now().UTC().Format(time.RFC3339),
		mode,
		strings.Repeat("=", 60))
	if err := appendToFile(paths.LogFile, separator); err != nil {
		return StartResult{}, fmt.Errorf("failed to write to log file: %v", err)
	}

	if opts.Daemon {
		return startDaemon(paths, entry)
	}
	return startForeground(paths, entry)
}

func startDaemon(paths AppPaths, koishiBin string) (StartResult, error) {
	logFile, err := os.OpenFile(paths.LogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return StartResult{}, fmt.Errorf("failed to open log file: %v", err)
	}
	defer logFile.Close()

	cmd := newKoishiCommand(koishiBin)
	cmd.Dir = paths.AppDir
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	configureDaemon(cmd)
	if err := cmd.Start(); err != nil {
		return StartResult{}, fmt.Errorf("failed to start daemon: %v", err)
	}
	pid := cmd.Process.Pid

	if err := StartRun(paths, pid, "daemon"); err != nil {
		return StartResult{}, err
	}

	// Reap the child in the background so it never becomes a zombie
	// while the launcher is still alive.
	go cmd.Wait()

	fmt.Printf("  Started in daemon mode (PID: %d)\n", pid)
	fmt.Printf("  Log: %s\n", paths.LogFile)
	return StartResult{Pid: pid, Mode: "daemon"}, nil
}

func startForeground(paths AppPaths, koishiBin string) (StartResult, error) {
	logFile, err := os.OpenFile(paths.LogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return StartResult{}, fmt.Errorf("failed to open log file: %v", err)
	}
	defer logFile.Close()

	cmd := newKoishiCommand(koishiBin)
	cmd.Dir = paths.AppDir
	// Foreground logs to both the terminal and the log file.
	cmd.Stdout = io.MultiWriter(os.Stdout, logFile)
	cmd.Stderr = io.MultiWriter(os.Stderr, logFile)
	cmd.Stdin = os.Stdin

	if err := cmd.Start(); err != nil {
		return StartResult{}, fmt.Errorf("failed to start: %v", err)
	}
	pid := cmd.Process.Pid

	if err := StartRun(paths, pid, "foreground"); err != nil {
		cmd.Process.Kill()
		return StartResult{}, err
	}

	// Forward terminal stop requests where the platform supports them.
	stopRequested := watchForeground(cmd)

	err = cmd.Wait()
	if stopRequested() {
		// A child death caused by our own forwarded signal is a clean stop.
		err = nil
	}

	// Always clear the run state; keep init info and the failure log.
	if stopErr := StopRun(paths); stopErr != nil {
		return StartResult{Pid: pid, Mode: "foreground"}, stopErr
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() != 0 {
			return StartResult{Pid: pid, Mode: "foreground"}, &ExitError{Code: exitErr.ExitCode()}
		}
	}
	return StartResult{Pid: pid, Mode: "foreground"}, nil
}

// ExitError reports a foreground child that exited with a non-zero code.
type ExitError struct {
	Code int
}

func (e *ExitError) Error() string {
	return fmt.Sprintf("Koishi exited with code %d", e.Code)
}

// Stop requests a graceful termination, waits up to 10s, then forces it.
// State is cleared on every path.
func Stop(opts StopOptions) (StopResult, error) {
	if err := AssertIsKoishiApp(opts.AppDir); err != nil {
		return StopResult{}, err
	}
	paths := Derive(opts.AppDir)
	state := ReadState(paths)

	if state.Pid == 0 {
		return StopResult{}, fmt.Errorf("no running YesImBot instance found")
	}

	if !IsProcessAlive(state.Pid, paths.AppDir) {
		StopRun(paths) // stale state: clear run fields
		return StopResult{}, fmt.Errorf("PID %d is no longer running (stale state cleared)", state.Pid)
	}

	method, graceful, err := stopProcess(state.Pid, state.Mode)
	if err != nil {
		return StopResult{}, err
	}

	if err := StopRun(paths); err != nil {
		return StopResult{}, fmt.Errorf("failed to update state: %v", err)
	}

	fmt.Printf("  Stopped (PID: %d, graceful: %v)\n", state.Pid, graceful)
	return StopResult{Pid: state.Pid, Signal: method, Graceful: graceful}, nil
}

// Status reports init/running state, cleaning stale run fields when the
// recorded PID no longer matches a live Koishi process.
func Status(appDir string) (RuntimeStatus, error) {
	if err := AssertIsKoishiApp(appDir); err != nil {
		return RuntimeStatus{}, err
	}
	paths := Derive(appDir)
	state := ReadState(paths)

	initialized := state.InitializedAt != ""
	running := false
	pid, mode := state.Pid, state.Mode
	if pid > 0 {
		if IsProcessAlive(pid, paths.AppDir) {
			running = true
		} else {
			StopRun(paths) // stale state
			state = ReadState(paths)
			pid, mode = state.Pid, state.Mode
		}
	}

	consoleURL := ""
	if running {
		consoleURL = ConsoleURL(paths)
	}

	return RuntimeStatus{
		Initialized: initialized,
		Running:     running,
		Pid:         pid,
		Mode:        mode,
		AppDir:      paths.AppDir,
		StartedAt:   strOr(state.StartedAt),
		StoppedAt:   strOr(state.StoppedAt),
		SourceHead:  state.SourceHead,
		UpdatedAt:   state.UpdatedAt,
		ConsoleURL:  consoleURL,
		LogFile:     paths.LogFile,
	}, nil
}

// resolveKoishiBin locates the installed Koishi CLI entry. Running it through
// Node avoids platform-specific node_modules/.bin wrappers.
func resolveKoishiBin(appDir string) string {
	return filepath.Join(appDir, "node_modules", "koishi", "bin.js")
}

func newKoishiCommand(koishiBin string) *exec.Cmd {
	return exec.Command("node", koishiBin, "start")
}

func strOr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func appendToFile(filename, content string) error {
	f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(content)
	return err
}
