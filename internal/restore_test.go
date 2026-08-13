package internal

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRestoreWholeAppBackup(t *testing.T) {
	parent := t.TempDir()
	backup := filepath.Join(parent, "bot.yesimbot-uninstall-20260101T000000Z.bak")
	if err := os.MkdirAll(backup, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(backup, "package.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := RestoreBackup(backup); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(parent, "bot")); err != nil {
		t.Errorf("restored app missing: %v", err)
	}
	if _, err := os.Stat(backup); !os.IsNotExist(err) {
		t.Errorf("backup still exists after restore: %v", err)
	}
}

func TestRestoreKeepAppConfigBackup(t *testing.T) {
	appDir := t.TempDir()
	backup := filepath.Join(appDir, ".yesimbot-uninstall-20260101T000000Z.bak")
	if err := os.MkdirAll(backup, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(backup, "package.json"), []byte("original-package"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(backup, "koishi.yml"), []byte("original-koishi"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(appDir, "package.json"), []byte("changed-package"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(appDir, "koishi.yml"), []byte("changed-koishi"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := RestoreBackup(backup); err != nil {
		t.Fatal(err)
	}
	packageContent, err := os.ReadFile(filepath.Join(appDir, "package.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(packageContent) != "original-package" {
		t.Errorf("package.json = %q, want original", packageContent)
	}
	koishiContent, err := os.ReadFile(filepath.Join(appDir, "koishi.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(koishiContent) != "original-koishi" {
		t.Errorf("koishi.yml = %q, want original", koishiContent)
	}
}
