package internal

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// LauncherState mirrors .yesimbot/launcher-state.json.
// StartedAt/StoppedAt are pointers so cleared values serialize as null,
// matching the design's example state file.
type LauncherState struct {
	InitializedAt string       `json:"initializedAt"`
	Pid           int          `json:"pid,omitempty"`
	Mode          string       `json:"mode,omitempty"`
	AppDir        string       `json:"appDir"`
	StartedAt     *string      `json:"startedAt"`
	StoppedAt     *string      `json:"stoppedAt"`
	SourceHead    string       `json:"sourceHead,omitempty"`
	Plugins       []PluginInfo `json:"plugins,omitempty"`
	UpdatedAt     string       `json:"updatedAt"`
}

// ReadState loads launcher state, returning safe defaults when the file
// is missing or unreadable.
func ReadState(paths AppPaths) LauncherState {
	state := LauncherState{AppDir: paths.AppDir}
	content, err := os.ReadFile(paths.StateFile)
	if err != nil {
		return state
	}
	var loaded LauncherState
	if err := json.Unmarshal(content, &loaded); err != nil {
		return state
	}
	state.AppDir = paths.AppDir
	state.InitializedAt = loaded.InitializedAt
	state.Pid = loaded.Pid
	state.Mode = loaded.Mode
	state.StartedAt = loaded.StartedAt
	state.StoppedAt = loaded.StoppedAt
	state.SourceHead = loaded.SourceHead
	state.Plugins = loaded.Plugins
	state.UpdatedAt = loaded.UpdatedAt
	return state
}

func writeState(paths AppPaths, state LauncherState) error {
	state.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(paths.StateFile), 0o755); err != nil {
		return fmt.Errorf("failed to create state directory: %v", err)
	}
	if err := os.WriteFile(paths.StateFile, data, 0o644); err != nil {
		return fmt.Errorf("failed to write state file: %v", err)
	}
	return nil
}

// StartRun records a running instance (pid, mode, startedAt).
func StartRun(paths AppPaths, pid int, mode string) error {
	state := ReadState(paths)
	now := time.Now().UTC().Format(time.RFC3339)
	state.Pid = pid
	state.Mode = mode
	state.StartedAt = &now
	state.StoppedAt = nil
	return writeState(paths, state)
}

// StopRun clears the run fields and records stoppedAt. Initialization
// info is preserved.
func StopRun(paths AppPaths) error {
	state := ReadState(paths)
	now := time.Now().UTC().Format(time.RFC3339)
	state.Pid = 0
	state.Mode = ""
	state.StartedAt = nil
	state.StoppedAt = &now
	return writeState(paths, state)
}

// MarkInitialized writes the init-complete state with no run fields.
func MarkInitialized(paths AppPaths, sourceHead string, plugins ...PluginInfo) error {
	state := LauncherState{
		InitializedAt: time.Now().UTC().Format(time.RFC3339),
		AppDir:        paths.AppDir,
		SourceHead:    sourceHead,
		Plugins:       plugins,
	}
	return writeState(paths, state)
}
