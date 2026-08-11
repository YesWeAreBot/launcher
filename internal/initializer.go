package internal

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const stepCount = 12

type InitOptions struct {
	Directory string
	Local     string
	Build     bool
}

type InitResult struct {
	AppDir     string
	SourceHead string
	Plugins    []PluginInfo
}

type initContext struct {
	options     InitOptions
	runner      CommandRunner
	paths       AppPaths
	registryURL string
	gitURL      string
	existing    bool
}

func logStep(step int, message string) {
	fmt.Printf("[%d/%d] %s\n", step, stepCount, message)
}

// Initialize runs the 12-step init flow. On failure the partial App is
// left in place (no user content is deleted) and no success state is
// written.
func Initialize(options InitOptions, runner CommandRunner) (InitResult, error) {
	// 1. Preflight the environment for this command.
	logStep(1, "Checking environment...")
	if err := preflight(runner, options); err != nil {
		return InitResult{}, err
	}

	// 2. Resolve and validate the target directory.
	logStep(2, "Resolving target directory...")
	appDir, err := ResolveInitTarget(options.Directory)
	if err != nil {
		return InitResult{}, err
	}
	existing := IsKoishiApp(appDir)
	if !existing {
		if err := AssertEmptyOrNew(appDir); err != nil {
			return InitResult{}, err
		}
	} else {
		fmt.Printf("WARNING: %s is an existing Koishi App. YesImBot will add dependencies and plugins without replacing existing settings.\n", appDir)
		proceed, err := AskUser("Continue installing YesImBot? [y/N] ")
		if err != nil {
			return InitResult{}, err
		}
		if !proceed {
			return InitResult{}, fmt.Errorf("installation cancelled")
		}
	}
	paths := Derive(appDir)

	// 3. Probe Git/Yarn candidate sources and prepare local config.
	logStep(3, "Probing package registries...")
	ctx := &initContext{
		options:     options,
		runner:      runner,
		paths:       paths,
		registryURL: probeYarnRegistry(),
		gitURL:      probeGitURL(),
		existing:    existing,
	}
	if options.Local != "" {
		localPath, err := filepath.Abs(options.Local)
		if err != nil {
			return InitResult{}, fmt.Errorf("invalid --local path: %v", err)
		}
		options.Local = localPath
		if _, err := os.Stat(localPath); os.IsNotExist(err) {
			return InitResult{}, fmt.Errorf("local YesImBot path does not exist: %s", localPath)
		}
	}

	// 4. Download the complete Koishi boilerplate for new Apps, or create only
	// launcher-owned directories for existing Apps.
	logStep(4, "Preparing Koishi App structure...")
	if err := createAppStructure(ctx); err != nil {
		return InitResult{}, err
	}

	// 5. Put the YesImBot source into .yesimbot/source.
	logStep(5, "Setting up YesImBot source...")
	if err := setupSource(ctx); err != nil {
		return InitResult{}, err
	}

	// 6. Read the YesImBot workspace package manifest.
	logStep(6, "Discovering plugins...")
	plugins, err := DiscoverPluginsWithConfig(paths.SourceDir, paths.LauncherConfigFile)
	if err != nil {
		return InitResult{}, err
	}
	fmt.Printf("  Found %d plugin(s)\n", len(plugins))

	// 7. Back up existing configuration, then add YesImBot dependencies.
	logStep(7, "Writing package.json...")
	if ctx.existing {
		if err := backupExistingAppConfig(paths, time.Now()); err != nil {
			return InitResult{}, err
		}
	}
	if err := writeAppPackageJson(ctx, plugins); err != nil {
		return InitResult{}, err
	}

	// 8. Add missing YesImBot plugins while retaining existing configuration.
	logStep(8, "Writing koishi.yml...")
	koishiContent, err := os.ReadFile(paths.KoishiYml)
	if err != nil {
		return InitResult{}, fmt.Errorf("failed to read koishi.yml: %v", err)
	}
	mergedKoishiContent, err := MergeExistingKoishiYml(koishiContent, plugins)
	if err != nil {
		return InitResult{}, err
	}
	if err := os.WriteFile(paths.KoishiYml, mergedKoishiContent, 0o644); err != nil {
		return InitResult{}, fmt.Errorf("failed to write koishi.yml: %v", err)
	}

	// 9. Install YesImBot workspace dependencies.
	logStep(9, "Installing YesImBot dependencies...")
	if err := installSourceDeps(ctx); err != nil {
		return InitResult{}, err
	}

	// 10. Build the YesImBot workspace.
	logStep(10, "Building YesImBot...")
	if err := buildSource(ctx); err != nil {
		return InitResult{}, err
	}

	// 11. Install/resolve the Koishi App dependencies.
	logStep(11, "Installing App dependencies...")
	if err := installAppDeps(ctx); err != nil {
		return InitResult{}, err
	}

	// 12. Write the initialized state.
	logStep(12, "Writing launcher state...")
	sourceHead, err := getSourceHead(ctx)
	if err != nil {
		sourceHead = "unknown"
	}
	if err := MarkInitialized(paths, sourceHead, plugins...); err != nil {
		return InitResult{}, err
	}

	fmt.Printf("\n✓ Initialization complete: %s\n", appDir)
	return InitResult{AppDir: appDir, SourceHead: sourceHead, Plugins: plugins}, nil
}

// preflight checks only what init needs: Node.js >= 18, and Git
// (hard requirement for remote mode; --local only reads HEAD, so a
// missing Git is a warning there).
func preflight(runner CommandRunner, options InitOptions) error {
	_, major, err := NodeVersion(runner)
	if err != nil {
		return err
	}
	if major < 18 {
		return fmt.Errorf("Node.js 18+ is required, found Node.js %d\n  Debian/Ubuntu: sudo apt install nodejs npm\n  macOS:         brew install node\n  Windows:       winget install OpenJS.NodeJS.LTS", major)
	}

	result, err := runner.Run("git", []string{"--version"}, RunOptions{})
	if err != nil || result.ExitCode != 0 {
		if options.Local != "" {
			fmt.Println("  ⚠ Git is not available (continuing — only needed for source HEAD detection)")
			return nil
		}
		return fmt.Errorf("Git is not available\n  Debian/Ubuntu: sudo apt install git\n  macOS:         brew install git\n  Windows:       winget install Git.Git")
	}
	return nil
}

// probeYarnRegistry picks the fastest reachable npm registry; all
// candidates failing falls back to the official one and lets the actual
// yarn command report errors.
func probeYarnRegistry() string {
	return probeFastest([]string{
		"https://registry.npmjs.org",
		"https://registry.npmmirror.com",
	}, "https://registry.npmjs.org")
}

// probeGitURL picks the fastest reachable YesImBot repository URL.
// GITHUB_MIRROR, when set, contributes a candidate before the official
// GitHub address. No global git config is ever written.
func probeGitURL() string {
	candidates := []string{"https://github.com/YesWeAreBot/YesImBot.git"}
	if githubMirrorRoot() != "" {
		candidates = append([]string{githubURL("YesWeAreBot/YesImBot.git")}, candidates...)
	}
	return probeFastest(candidates, candidates[len(candidates)-1])
}

// probeFastest runs concurrent lightweight TCP probes and returns the
// first candidate accepting a connection within 3 seconds. A TCP
// handshake is a sufficient reachability probe for mirror selection:
// any candidate that accepts connections but then fails HTTP is
// reported by the actual git/yarn command, per the design's fallback
// rule. Dialing instead of net/http keeps the binary several MB
// smaller (no crypto/tls).
func probeFastest(candidates []string, fallback string) string {
	results := make(chan string, len(candidates))
	for _, c := range candidates {
		go func(c string) {
			if dialProbe(c) {
				results <- c
			}
		}(c)
	}
	select {
	case c := <-results:
		return c
	case <-time.After(3 * time.Second):
		return fallback
	}
}

func dialProbe(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	port := u.Port()
	if port == "" {
		if u.Scheme == "http" {
			port = "80"
		} else {
			port = "443"
		}
	}
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(u.Hostname(), port), 2*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// createAppStructure downloads a complete Koishi App for a new target. An
// existing App receives only launcher-owned directories.
func createAppStructure(ctx *initContext) error {
	paths := ctx.paths
	if !ctx.existing {
		if err := downloadBoilerplate(paths.AppDir); err != nil {
			return err
		}
	}
	for _, dir := range []string{paths.YesimbotDir, paths.SourceDir, paths.LogsDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("failed to create directory %s: %v", dir, err)
		}
	}
	return EnsureLauncherConfig(paths.LauncherConfigFile)
}

// setupSource links --local into .yesimbot/source without touching the
// external repo, or clones the chosen remote at origin/dev.
func setupSource(ctx *initContext) error {
	paths := ctx.paths
	info, err := os.Lstat(paths.SourceDir)
	if err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			if ctx.options.Local != "" {
				target, evalErr := filepath.EvalSymlinks(paths.SourceDir)
				if evalErr == nil && samePath(target, ctx.options.Local) {
					fmt.Printf("  Linked: %s → %s\n", paths.SourceDir, ctx.options.Local)
					return nil
				}
			}
			if err := os.Remove(paths.SourceDir); err != nil {
				return fmt.Errorf("failed to prepare source directory: %v", err)
			}
		} else if info.IsDir() {
			fmt.Printf("  Using existing source directory: %s\n", paths.SourceDir)
			return nil
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to inspect source directory: %v", err)
	}

	if ctx.options.Local != "" {
		if err := os.Symlink(ctx.options.Local, paths.SourceDir); err != nil {
			return fmt.Errorf("failed to create symlink: %v", err)
		}
		fmt.Printf("  Linked: %s → %s\n", paths.SourceDir, ctx.options.Local)
		return nil
	}

	repo := ctx.gitURL
	if _, err := Checked(ctx.runner, "git", []string{
		"clone", "--branch", "dev", "--single-branch", "--depth", "1", repo, paths.SourceDir,
	}, RunOptions{Cwd: paths.AppDir}); err != nil {
		return err
	}
	return nil
}

func samePath(left, right string) bool {
	leftAbs, leftErr := filepath.Abs(left)
	rightAbs, rightErr := filepath.Abs(right)
	if leftErr != nil || rightErr != nil {
		return false
	}
	return strings.EqualFold(filepath.Clean(leftAbs), filepath.Clean(rightAbs))
}

func writeAppPackageJson(ctx *initContext, plugins []PluginInfo) error {
	content, err := os.ReadFile(ctx.paths.PackageJson)
	if err != nil {
		return fmt.Errorf("failed to read package.json: %v", err)
	}
	data, conflicts, err := MergeExistingAppPackageJSON(content, ctx.paths.AppDir, ctx.paths.SourceDir, plugins)
	if err != nil {
		return err
	}
	for _, packageName := range conflicts {
		fmt.Printf("  Keeping existing dependency version: %s\n", packageName)
	}
	return os.WriteFile(ctx.paths.PackageJson, data, 0o644)
}

func backupExistingAppConfig(paths AppPaths, timestamp time.Time) error {
	suffix := ".yesimbot." + timestamp.UTC().Format("20060102T150405Z") + ".bak"
	for _, source := range []string{paths.KoishiYml, paths.PackageJson} {
		if err := backupFile(source, source+suffix); err != nil {
			return fmt.Errorf("failed to back up %s: %v", filepath.Base(source), err)
		}
	}
	return nil
}

func backupFile(source, destination string) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()

	output, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	defer output.Close()
	_, err = io.Copy(output, input)
	return err
}

// installSourceDeps runs yarn install in the source workspace. Remote
// mode always runs it; --local skips by default, prompting when
// node_modules is missing unless --build forces the run.
func installSourceDeps(ctx *initContext) error {
	realSourceDir, err := filepath.EvalSymlinks(ctx.paths.SourceDir)
	if err != nil {
		return fmt.Errorf("cannot resolve source directory: %v", err)
	}

	if ctx.options.Local != "" {
		installed := dirExists(filepath.Join(realSourceDir, "node_modules"))
		if installed && !ctx.options.Build {
			fmt.Println("  Source dependencies already installed (skipping)")
			return nil
		}
		if !installed && !ctx.options.Build {
			proceed, err := AskUser("  Source node_modules not found. Install dependencies now? [Y/n] ")
			if err != nil {
				return err
			}
			if !proceed {
				fmt.Println("  Skipping source dependency install")
				return nil
			}
		}
	}

	yarnPath := findYarnBinary(realSourceDir, ctx.paths.YarnBin)
	_, err = Checked(ctx.runner, "node", []string{yarnPath, "install"}, RunOptions{
		Cwd: realSourceDir,
		Env: map[string]string{
			"YARN_NPM_REGISTRY_SERVER":       ctx.registryURL,
			"YARN_ENABLE_IMMUTABLE_INSTALLS": "false",
		},
		Stdio: "inherit",
	})
	return err
}

// buildSource runs yarn build in the source workspace under the same
// skip/prompt/force rules as installSourceDeps, checking core/dist.
func buildSource(ctx *initContext) error {
	realSourceDir, err := filepath.EvalSymlinks(ctx.paths.SourceDir)
	if err != nil {
		return fmt.Errorf("cannot resolve source directory: %v", err)
	}

	if ctx.options.Local != "" {
		built := dirExists(filepath.Join(realSourceDir, "core", "dist")) ||
			dirExists(filepath.Join(realSourceDir, "core", "lib"))
		if built && !ctx.options.Build {
			fmt.Println("  Source already built (skipping)")
			return nil
		}
		if !built && !ctx.options.Build {
			proceed, err := AskUser("  Source not built (core/dist missing). Build now? [Y/n] ")
			if err != nil {
				return err
			}
			if !proceed {
				fmt.Println("  Skipping source build")
				return nil
			}
		}
	}

	yarnPath := findYarnBinary(realSourceDir, ctx.paths.YarnBin)
	_, err = Checked(ctx.runner, "node", []string{yarnPath, "build"}, RunOptions{
		Cwd:   realSourceDir,
		Env:   map[string]string{"YARN_NPM_REGISTRY_SERVER": ctx.registryURL},
		Stdio: "inherit",
	})
	return err
}

func installAppDeps(ctx *initContext) error {
	_, err := Checked(ctx.runner, "node", []string{ctx.paths.YarnBin, "install"}, RunOptions{
		Cwd: ctx.paths.AppDir,
		Env: map[string]string{
			"YARN_NPM_REGISTRY_SERVER":       ctx.registryURL,
			"YARN_ENABLE_IMMUTABLE_INSTALLS": "false",
		},
		Stdio: "inherit",
	})
	return err
}

func getSourceHead(ctx *initContext) (string, error) {
	realSourceDir, err := filepath.EvalSymlinks(ctx.paths.SourceDir)
	if err != nil {
		return "", err
	}
	result, err := ctx.runner.Run("git", []string{"rev-parse", "HEAD"}, RunOptions{Cwd: realSourceDir})
	if err != nil || result.ExitCode != 0 {
		return "", fmt.Errorf("cannot read source HEAD")
	}
	return strings.TrimSpace(result.Stdout), nil
}

// findYarnBinary prefers the source's own yarn release, falling back to
// the App's copy.
func findYarnBinary(sourceRoot, appYarnBin string) string {
	entries, err := os.ReadDir(filepath.Join(sourceRoot, ".yarn", "releases"))
	if err == nil {
		for _, entry := range entries {
			name := entry.Name()
			if strings.HasPrefix(name, "yarn-") && strings.HasSuffix(name, ".cjs") {
				return filepath.Join(sourceRoot, ".yarn", "releases", name)
			}
		}
	}
	return appYarnBin
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// AskUser prompts yes/no; non-interactive stdin defaults to yes.
func AskUser(question string) (bool, error) {
	if !isTerminal() {
		return true, nil
	}
	fmt.Print(question)
	answer, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && err != io.EOF {
		return false, err
	}
	return parseYesNo(answer), nil
}

func parseYesNo(answer string) bool {
	trimmed := strings.TrimSpace(strings.ToLower(answer))
	return trimmed == "" || trimmed == "y" || trimmed == "yes"
}

func isTerminal() bool {
	info, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}
