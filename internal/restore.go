package internal

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// RestoreBackup restores either a whole Koishi App backup created by the
// default uninstall flow, or the package.json/koishi.yml backup created by
// uninstall --keep-app.
func RestoreBackup(backupDir string) error {
	backup, err := filepath.Abs(backupDir)
	if err != nil {
		return err
	}
	info, err := os.Stat(backup)
	if err != nil {
		return fmt.Errorf("backup directory not found: %v", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("backup path is not a directory: %s", backup)
	}

	entries, err := os.ReadDir(backup)
	if err != nil {
		return err
	}
	names := make(map[string]bool, len(entries))
	for _, entry := range entries {
		names[entry.Name()] = true
	}

	if names["package.json"] && names["koishi.yml"] {
		appDir := filepath.Dir(backup)
		if err := restoreFile(filepath.Join(backup, "package.json"), filepath.Join(appDir, "package.json")); err != nil {
			return fmt.Errorf("failed to restore package.json: %v", err)
		}
		if err := restoreFile(filepath.Join(backup, "koishi.yml"), filepath.Join(appDir, "koishi.yml")); err != nil {
			return fmt.Errorf("failed to restore koishi.yml: %v", err)
		}
		fmt.Printf("Restored package.json and koishi.yml in %s\n", appDir)
		return nil
	}

	parent := filepath.Dir(backup)
	name := filepath.Base(backup)
	marker := ".yesimbot-uninstall-"
	index := strings.LastIndex(name, marker)
	if index <= 0 || !strings.HasSuffix(name, ".bak") {
		return fmt.Errorf("unrecognized backup directory: %s", backup)
	}
	original := filepath.Join(parent, name[:index])
	if _, err := os.Stat(original); err == nil {
		return fmt.Errorf("cannot restore: target already exists: %s", original)
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.Rename(backup, original); err != nil {
		return fmt.Errorf("failed to restore app: %v", err)
	}
	fmt.Printf("Restored app to %s\n", original)
	return nil
}

func restoreFile(source, target string) error {
	content, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	return os.WriteFile(target, content, 0o644)
}
