package internal

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveInitTarget(t *testing.T) {
	dir, err := ResolveInitTarget("")
	if err != nil {
		t.Fatal(err)
	}
	cwd, _ := os.Getwd()
	if dir != filepath.Join(cwd, "yesimbot-app") {
		t.Errorf("default target = %q, want %q", dir, filepath.Join(cwd, "yesimbot-app"))
	}

	explicit, err := ResolveInitTarget("some/dir")
	if err != nil {
		t.Fatal(err)
	}
	want, _ := filepath.Abs("some/dir")
	if explicit != want {
		t.Errorf("explicit target = %q, want %q", explicit, want)
	}
}

func TestAssertEmptyOrNew(t *testing.T) {
	// Non-existent directory is acceptable.
	missing := filepath.Join(t.TempDir(), "missing")
	if err := AssertEmptyOrNew(missing); err != nil {
		t.Errorf("missing dir rejected: %v", err)
	}

	// Empty directory is acceptable.
	empty := t.TempDir()
	if err := AssertEmptyOrNew(empty); err != nil {
		t.Errorf("empty dir rejected: %v", err)
	}

	// Non-empty directory is rejected.
	full := t.TempDir()
	os.WriteFile(filepath.Join(full, "x"), []byte("x"), 0o644)
	if err := AssertEmptyOrNew(full); err == nil {
		t.Error("non-empty dir accepted, want rejection")
	}
}

func TestAssertIsKoishiApp(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "package.json"), []byte("{}"), 0o644)

	if IsKoishiApp(dir) {
		t.Error("app without koishi.yml reported as valid")
	}
	if err := AssertIsKoishiApp(dir); err == nil {
		t.Error("app without koishi.yml accepted")
	}

	os.WriteFile(filepath.Join(dir, "koishi.yml"), []byte("plugins: {}\n"), 0o644)
	if !IsKoishiApp(dir) {
		t.Error("complete app reported as invalid")
	}
	if err := AssertIsKoishiApp(dir); err != nil {
		t.Errorf("complete app rejected: %v", err)
	}
}
