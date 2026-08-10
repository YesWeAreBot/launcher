package internal

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestStateLifecycle(t *testing.T) {
	paths := Derive(t.TempDir())

	// Missing file -> defaults.
	state := ReadState(paths)
	if state.InitializedAt != "" || state.Pid != 0 || state.AppDir != paths.AppDir {
		t.Errorf("defaults wrong: %+v", state)
	}

	// Init.
	if err := MarkInitialized(paths, "abc123"); err != nil {
		t.Fatal(err)
	}
	state = ReadState(paths)
	if state.InitializedAt == "" || state.SourceHead != "abc123" || state.Pid != 0 {
		t.Errorf("init state wrong: %+v", state)
	}

	// Start run.
	if err := StartRun(paths, 4242, "daemon"); err != nil {
		t.Fatal(err)
	}
	state = ReadState(paths)
	if state.Pid != 4242 || state.Mode != "daemon" || state.StartedAt == nil || state.StoppedAt != nil {
		t.Errorf("start state wrong: %+v", state)
	}
	if state.InitializedAt == "" {
		t.Error("start lost initializedAt")
	}

	// Stop run.
	if err := StopRun(paths); err != nil {
		t.Fatal(err)
	}
	state = ReadState(paths)
	if state.Pid != 0 || state.Mode != "" || state.StartedAt != nil || state.StoppedAt == nil {
		t.Errorf("stop state wrong: %+v", state)
	}
	if state.InitializedAt == "" || state.SourceHead != "abc123" {
		t.Error("stop lost init info")
	}

	// Serialized shape: cleared run fields are null, not absent.
	content, err := os.ReadFile(paths.StateFile)
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]any
	if err := json.Unmarshal(content, &raw); err != nil {
		t.Fatal(err)
	}
	if raw["startedAt"] != nil || raw["stoppedAt"] == nil {
		t.Errorf("state json shape wrong: %s", content)
	}
}

func TestCorruptStateFallsBack(t *testing.T) {
	paths := Derive(t.TempDir())
	os.WriteFile(paths.StateFile, []byte("{not json"), 0o644)
	state := ReadState(paths)
	if state.Pid != 0 || state.AppDir != paths.AppDir {
		t.Errorf("corrupt state not handled: %+v", state)
	}
}

func TestIsProcessAlive(t *testing.T) {
	if IsProcessAlive(999999999, "/tmp") {
		t.Error("nonexistent pid reported alive")
	}
	if runtime.GOOS == "windows" {
		if !IsProcessAlive(os.Getpid(), "") {
			t.Error("own pid reported dead")
		}
		return
	}
	// Unix verifies cwd to avoid PID reuse targeting another process.
	if IsProcessAlive(os.Getpid(), "/definitely/not/our/cwd") {
		t.Error("pid with mismatched cwd reported alive")
	}
}

func TestWaitForExit(t *testing.T) {
	if !WaitForExit(999999999, 2*time.Second) {
		t.Error("WaitForExit on dead pid should return true immediately")
	}
}

func TestConsoleURL(t *testing.T) {
	paths := Derive(t.TempDir())
	plugins := []PluginInfo{{PackageName: "koishi-plugin-yesimbot", ConfigKey: "yesimbot", Group: "core", Enabled: true}}
	content, err := GenerateKoishiYml(plugins, 0)
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(paths.KoishiYml, []byte(content), 0o644)

	if url := ConsoleURL(paths); url != "http://127.0.0.1:5140" {
		t.Errorf("ConsoleURL = %q, want http://127.0.0.1:5140", url)
	}

	// Custom port via a Koishi-style instance key (like the real app).
	custom := "plugins:\n  group:server:\n    server:jlupo5:\n      port: 6001\n"
	os.WriteFile(paths.KoishiYml, []byte(custom), 0o644)
	if url := ConsoleURL(paths); url != "http://127.0.0.1:6001" {
		t.Errorf("ConsoleURL custom = %q, want http://127.0.0.1:6001", url)
	}
}

func TestStatusStaleCleanup(t *testing.T) {
	appDir := t.TempDir()
	os.WriteFile(filepath.Join(appDir, "package.json"), []byte("{}"), 0o644)
	os.WriteFile(filepath.Join(appDir, "koishi.yml"), []byte("plugins: {}\n"), 0o644)
	paths := Derive(appDir)

	if err := MarkInitialized(paths, "deadbeef"); err != nil {
		t.Fatal(err)
	}
	if err := StartRun(paths, 999999999, "daemon"); err != nil {
		t.Fatal(err)
	}

	s, err := Status(appDir)
	if err != nil {
		t.Fatal(err)
	}
	if s.Running || s.Pid != 0 || s.Mode != "" {
		t.Errorf("stale state not cleaned: %+v", s)
	}
	if !s.Initialized || s.SourceHead != "deadbeef" {
		t.Errorf("init info lost: %+v", s)
	}

	// State file itself should now be clean.
	state := ReadState(paths)
	if state.Pid != 0 || state.StoppedAt == nil {
		t.Errorf("state not persisted clean: %+v", state)
	}
}

func TestParseYesNo(t *testing.T) {
	for in, want := range map[string]bool{
		"": true, "y": true, "Y": true, "yes": true, "Yes": true,
		"n": false, "no": false, "cancel": false,
	} {
		if got := parseYesNo(in); got != want {
			t.Errorf("parseYesNo(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestKoishiCommandUsesNodeCLIEntry(t *testing.T) {
	entry := filepath.Join(t.TempDir(), "node_modules", "@koishijs", "cli", "bin", "koishi.mjs")
	cmd := newKoishiCommand(entry)
	want := []string{"node", entry, "start"}
	if len(cmd.Args) != len(want) {
		t.Fatalf("command args = %q, want %q", cmd.Args, want)
	}
	for i := range want {
		if cmd.Args[i] != want[i] {
			t.Errorf("command args = %q, want %q", cmd.Args, want)
			break
		}
	}
}
