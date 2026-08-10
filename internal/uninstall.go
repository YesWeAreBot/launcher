package internal

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// UninstallOptions controls the uninstall command.
type UninstallOptions struct {
	AppDir   string
	KeepApp  bool
	SkipDeps bool
}

// UninstallResult reports what uninstall changed.
type UninstallResult struct {
	AppDir    string
	BackupDir string
	KeptApp   bool
	Removed   []string
}

// Uninstall removes YesImBot from a Koishi App. By default it moves the whole
// App to a sibling backup directory so the operation is reversible.
func Uninstall(opts UninstallOptions, runner CommandRunner) (UninstallResult, error) {
	appDir, err := ResolveAppDir(opts.AppDir)
	if err != nil {
		return UninstallResult{}, err
	}
	if err := AssertIsKoishiApp(appDir); err != nil {
		return UninstallResult{}, err
	}

	paths := Derive(appDir)
	state := ReadState(paths)
	if state.Pid > 0 && IsProcessAlive(state.Pid, paths.AppDir) {
		if _, err := Stop(StopOptions{AppDir: appDir}); err != nil {
			return UninstallResult{}, err
		}
	}

	if opts.KeepApp {
		return uninstallFromApp(paths, state, opts.SkipDeps, runner)
	}
	return uninstallWholeApp(paths)
}

func uninstallWholeApp(paths AppPaths) (UninstallResult, error) {
	if filepath.Dir(paths.AppDir) == paths.AppDir {
		return UninstallResult{}, fmt.Errorf("refusing to uninstall filesystem root: %s", paths.AppDir)
	}
	backupDir, err := backupPath(paths.AppDir)
	if err != nil {
		return UninstallResult{}, err
	}
	if err := os.Rename(paths.AppDir, backupDir); err != nil {
		return UninstallResult{}, fmt.Errorf("failed to move app to backup: %v", err)
	}
	return UninstallResult{AppDir: paths.AppDir, BackupDir: backupDir}, nil
}

func uninstallFromApp(paths AppPaths, state LauncherState, skipDeps bool, runner CommandRunner) (UninstallResult, error) {
	plugins := state.Plugins
	if len(plugins) == 0 && dirExists(paths.SourceDir) {
		if discovered, err := DiscoverPlugins(paths.SourceDir); err == nil {
			plugins = discovered
		}
	}

	packageContent, err := os.ReadFile(paths.PackageJson)
	if err != nil {
		return UninstallResult{}, err
	}
	updatedPackage, removed, err := RemoveManagedAppPackageJSON(packageContent, paths.AppDir, paths.SourceDir, plugins)
	if err != nil {
		return UninstallResult{}, err
	}
	if err := os.WriteFile(paths.PackageJson, updatedPackage, 0o644); err != nil {
		return UninstallResult{}, err
	}

	koishiContent, err := os.ReadFile(paths.KoishiYml)
	if err != nil {
		return UninstallResult{}, err
	}
	updatedKoishi, configKeys, err := RemoveManagedKoishiYml(koishiContent, plugins)
	if err != nil {
		return UninstallResult{}, err
	}
	if err := os.WriteFile(paths.KoishiYml, updatedKoishi, 0o644); err != nil {
		return UninstallResult{}, err
	}
	removed = append(removed, configKeys...)

	if !skipDeps {
		if err := syncAppDependencies(paths, runner); err != nil {
			return UninstallResult{}, err
		}
	}
	if err := removeYesimbotDir(paths); err != nil {
		return UninstallResult{}, err
	}
	return UninstallResult{AppDir: paths.AppDir, KeptApp: true, Removed: removed}, nil
}

func backupPath(appDir string) (string, error) {
	parent := filepath.Dir(appDir)
	base := filepath.Base(appDir)
	stamp := time.Now().UTC().Format("20060102T150405Z")
	for index := 1; ; index++ {
		suffix := ""
		if index > 1 {
			suffix = fmt.Sprintf("-%d", index)
		}
		candidate := filepath.Join(parent, fmt.Sprintf("%s.yesimbot-uninstall-%s%s.bak", base, stamp, suffix))
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate, nil
		} else if err != nil {
			return "", err
		}
	}
}

func removeYesimbotDir(paths AppPaths) error {
	if !dirExists(paths.YesimbotDir) {
		return nil
	}
	relative, err := filepath.Rel(paths.AppDir, paths.YesimbotDir)
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return fmt.Errorf("refusing to remove directory outside app: %s", paths.YesimbotDir)
	}
	return os.RemoveAll(paths.YesimbotDir)
}

func syncAppDependencies(paths AppPaths, runner CommandRunner) error {
	_, err := Checked(runner, "node", []string{paths.YarnBin, "install"}, RunOptions{
		Cwd: paths.AppDir,
		Env: map[string]string{
			"YARN_ENABLE_IMMUTABLE_INSTALLS": "false",
		},
		Stdio: "inherit",
	})
	if err != nil {
		return fmt.Errorf("failed to sync app dependencies: %v", err)
	}
	return nil
}
