// Package internal holds the launcher's internal dependencies:
// path rules, command execution, config generation, init orchestration
// and child-process lifecycle. Command wiring lives in cmd/.
package internal

import (
	"fmt"
	"os"
	"path/filepath"
)

// AppPaths holds every path the launcher manages inside a Koishi App directory.
type AppPaths struct {
	AppDir             string
	YesimbotDir        string
	SourceDir          string
	LogsDir            string
	LogFile            string
	LauncherConfigFile string
	StateFile          string
	KoishiYml          string
	PackageJson        string
	YarnBin            string
}

// Derive computes all launcher-managed paths for a Koishi App directory.
func Derive(appDir string) AppPaths {
	yesimbotDir := filepath.Join(appDir, ".yesimbot")
	logsDir := filepath.Join(yesimbotDir, "logs")
	return AppPaths{
		AppDir:             appDir,
		YesimbotDir:        yesimbotDir,
		SourceDir:          filepath.Join(yesimbotDir, "source"),
		LogsDir:            logsDir,
		LogFile:            filepath.Join(logsDir, "koishi.log"),
		LauncherConfigFile: filepath.Join(yesimbotDir, "launcher.yaml"),
		StateFile:          filepath.Join(yesimbotDir, "launcher-state.json"),
		KoishiYml:          filepath.Join(appDir, "koishi.yml"),
		PackageJson:        filepath.Join(appDir, "package.json"),
		YarnBin:            filepath.Join(appDir, ".yarn", "releases", "yarn-4.12.0.cjs"),
	}
}

// ResolveInitTarget returns the absolute target directory for init:
// the given directory, or ./yesimbot-app in the current working directory.
func ResolveInitTarget(directory string) (string, error) {
	if directory != "" {
		return filepath.Abs(directory)
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("cannot resolve current directory: %v", err)
	}
	return filepath.Join(cwd, "yesimbot-app"), nil
}

// ResolveAppDir returns the absolute App directory for start/stop/status:
// the --app value, or the current working directory.
func ResolveAppDir(appOption string) (string, error) {
	if appOption != "" {
		return filepath.Abs(appOption)
	}
	return os.Getwd()
}

// AssertEmptyOrNew rejects directories that exist and are not empty.
// init never overwrites or deletes existing content.
func AssertEmptyOrNew(directory string) error {
	info, err := os.Stat(directory)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("cannot stat directory: %v", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("not a directory: %s", directory)
	}

	entries, err := os.ReadDir(directory)
	if err != nil {
		return fmt.Errorf("cannot read directory: %v", err)
	}
	if len(entries) > 0 {
		return fmt.Errorf("directory is not empty: %s\n  init requires an empty or non-existent directory.\n  Found %d entries", directory, len(entries))
	}
	return nil
}

// AssertIsKoishiApp errors unless the directory contains both
// package.json and koishi.yml.
func AssertIsKoishiApp(directory string) error {
	for _, name := range []string{"package.json", "koishi.yml"} {
		info, err := os.Stat(filepath.Join(directory, name))
		if os.IsNotExist(err) {
			return fmt.Errorf("not a Koishi App: %s\n  Expected %s to exist.", directory, name)
		}
		if err != nil {
			return fmt.Errorf("cannot inspect %s: %v", directory, err)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("not a Koishi App: %s\n  Expected %s to be a file.", directory, name)
		}
	}
	return nil
}

// IsKoishiApp reports whether the directory looks like a Koishi App.
func IsKoishiApp(directory string) bool {
	return AssertIsKoishiApp(directory) == nil
}
