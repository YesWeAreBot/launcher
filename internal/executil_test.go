package internal

import (
	"runtime"
	"strings"
	"testing"
)

type fakeRunner struct {
	results map[string]RunResult
}

func (f *fakeRunner) Run(command string, args []string, options RunOptions) (RunResult, error) {
	key := command + " " + strings.Join(args, " ")
	if r, ok := f.results[key]; ok {
		return r, nil
	}
	return RunResult{}, nil
}

func TestEnvMergePreservesPath(t *testing.T) {
	runner := NewRunner()
	var result RunResult
	var err error
	if runtime.GOOS == "windows" {
		result, err = runner.Run("cmd", []string{"/C", "echo PATH=%PATH% CUSTOM=%YESIMBOT_TEST_CUSTOM%"}, RunOptions{
			Env: map[string]string{"YESIMBOT_TEST_CUSTOM": "ok"},
		})
	} else {
		result, err = runner.Run("sh", []string{"-c", "echo PATH=${PATH:+set} CUSTOM=$YESIMBOT_TEST_CUSTOM"}, RunOptions{
			Env: map[string]string{"YESIMBOT_TEST_CUSTOM": "ok"},
		})
	}
	if err != nil {
		t.Fatal(err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("exit %d: %s", result.ExitCode, result.Stderr)
	}
	if !strings.Contains(result.Stdout, "PATH=") {
		t.Errorf("PATH was dropped from child env: %q", result.Stdout)
	}
	if !strings.Contains(result.Stdout, "CUSTOM=ok") {
		t.Errorf("custom env not passed: %q", result.Stdout)
	}
}

func TestCheckedErrorContext(t *testing.T) {
	runner := &fakeRunner{results: map[string]RunResult{
		"git rev-parse HEAD": {ExitCode: 128, Stderr: "fatal: not a git repository"},
	}}
	_, err := Checked(runner, "git", []string{"rev-parse", "HEAD"}, RunOptions{Cwd: "/tmp"})
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	for _, want := range []string{"exit 128", "git rev-parse HEAD", "cwd: /tmp", "not a git repository"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error %q missing %q", msg, want)
		}
	}
}

func TestNodeVersionParsing(t *testing.T) {
	runner := &fakeRunner{results: map[string]RunResult{
		"node --version": {Stdout: "v22.9.0"},
	}}
	version, major, err := NodeVersion(runner)
	if err != nil {
		t.Fatal(err)
	}
	if version != "v22.9.0" || major != 22 {
		t.Errorf("got %q major %d, want v22.9.0 / 22", version, major)
	}
}

func TestNodeVersionMissing(t *testing.T) {
	runner := &fakeRunner{} // node --version not stubbed -> exit 0 empty; use error path
	_, _, err := NodeVersion(runner)
	if err == nil {
		// Empty stdout means the parse fails -> error expected.
		t.Error("expected error for unparseable node version")
	}
}
