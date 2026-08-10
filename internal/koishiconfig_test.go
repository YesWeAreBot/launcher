package internal

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeFixture creates a minimal YesImBot-like workspace in a temp dir.
func writeFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	pkgs := map[string]string{
		"core":                         "koishi-plugin-yesimbot",
		"providers/provider-openai":    "@yesimbot/koishi-plugin-provider-openai",
		"providers/provider-anthropic": "@yesimbot/koishi-plugin-provider-anthropic",
		"plugins/yesimbot-workspace":   "koishi-plugin-yesimbot-workspace",
		"plugins/yesimbot-mcp-client":  "koishi-plugin-yesimbot-mcp-client",
		"packages/not-a-plugin":        "some-lib",
	}
	for dir, name := range pkgs {
		pkgDir := filepath.Join(root, dir)
		if err := os.MkdirAll(pkgDir, 0o755); err != nil {
			t.Fatal(err)
		}
		meta := map[string]string{"name": name}
		data, _ := json.Marshal(meta)
		if err := os.WriteFile(filepath.Join(pkgDir, "package.json"), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestDiscoverPlugins(t *testing.T) {
	plugins, err := DiscoverPlugins(writeFixture(t))
	if err != nil {
		t.Fatal(err)
	}

	byKey := map[string]PluginInfo{}
	for _, p := range plugins {
		byKey[p.PackageName] = p
	}

	if p, ok := byKey["koishi-plugin-yesimbot"]; !ok || p.Group != "core" || !p.Enabled || p.ConfigKey != "yesimbot" {
		t.Errorf("core plugin wrong: %+v", p)
	}
	if p, ok := byKey["@yesimbot/koishi-plugin-provider-openai"]; !ok || p.Group != "provider" || p.Enabled || p.Label != "openai" || p.ConfigKey != "@yesimbot/provider-openai" {
		t.Errorf("provider plugin wrong: %+v", p)
	}
	if p, ok := byKey["koishi-plugin-yesimbot-workspace"]; !ok || p.Group != "yesimbot" || p.Enabled || p.Label != "workspace" || p.ConfigKey != "yesimbot-workspace" {
		t.Errorf("yesimbot plugin wrong: %+v", p)
	}
	if _, ok := byKey["some-lib"]; ok {
		t.Error("non-plugin package discovered")
	}

	// Ordered core < provider < yesimbot.
	if plugins[0].Group != "core" || plugins[1].Group != "provider" {
		t.Errorf("unexpected order: %+v", plugins)
	}
}

func TestDiscoverPluginsEmpty(t *testing.T) {
	if _, err := DiscoverPlugins(t.TempDir()); err == nil {
		t.Error("expected error for empty source dir")
	}
}

func TestPackageNameToConfigKey(t *testing.T) {
	cases := map[string]string{
		"koishi-plugin-yesimbot":                  "yesimbot",
		"@yesimbot/koishi-plugin-provider-openai": "@yesimbot/provider-openai",
		"koishi-plugin-yesimbot-workspace":        "yesimbot-workspace",
		"plain-package":                           "plain-package",
	}
	for in, want := range cases {
		if got := PackageNameToConfigKey(in); got != want {
			t.Errorf("PackageNameToConfigKey(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestGenerateKoishiYml(t *testing.T) {
	plugins, err := DiscoverPlugins(writeFixture(t))
	if err != nil {
		t.Fatal(err)
	}
	content, err := GenerateKoishiYml(plugins, 0)
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{
		"group:server:", // infrastructure groups with safe defaults
		"server:launcher:",
		"host: 127.0.0.1",
		"port: 5140",
		"group:console:",
		"group:storage:",
		"database-sqlite:launcher:",
		"path: data/koishi.db",
		"yesimbot: {}", // core enabled at top level
		"group:provider:",
		"~@yesimbot/provider-openai:instance:", // disabled with ~ prefix
		"$label: openai",
		"group:yesimbot:",
		"~yesimbot-workspace:instance:",
		"$label: workspace",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("koishi.yml missing %q\n---\n%s", want, content)
		}
	}
	for _, banned := range []string{"maxPort", "assets-local", "~yesimbot:"} {
		if strings.Contains(content, banned) {
			t.Errorf("koishi.yml contains %q\n---\n%s", banned, content)
		}
	}
}

func TestGenerateAppPackageJson(t *testing.T) {
	plugins, err := DiscoverPlugins(writeFixture(t))
	if err != nil {
		t.Fatal(err)
	}
	appDir := filepath.Join(t.TempDir(), "yesimbot-app")
	sourceDir := filepath.Join(appDir, ".yesimbot", "source")

	pkg, err := GenerateAppPackageJson(struct {
		AppDir      string
		SourceDir   string
		Plugins     []PluginInfo
		YarnVersion string
	}{appDir, sourceDir, plugins, "4.12.0"})
	if err != nil {
		t.Fatal(err)
	}

	if pkg["name"] != "yesimbot-app" || pkg["packageManager"] != "yarn@4.12.0" {
		t.Errorf("pkg meta wrong: %v", pkg)
	}

	workspaces, ok := pkg["workspaces"].([]string)
	if !ok {
		t.Fatalf("workspaces not a list: %v", pkg["workspaces"])
	}
	if workspaces[0] != ".yesimbot/source" {
		t.Errorf("workspaces[0] = %q, want relative source path", workspaces[0])
	}
	for _, ws := range []string{".yesimbot/source/core", ".yesimbot/source/packages/*", ".yesimbot/source/providers/*", ".yesimbot/source/plugins/*"} {
		found := false
		for _, w := range workspaces {
			if w == ws {
				found = true
			}
		}
		if !found {
			t.Errorf("workspaces missing %q: %v", ws, workspaces)
		}
	}

	deps, ok := pkg["dependencies"].(map[string]string)
	if !ok {
		t.Fatalf("dependencies not map[string]string: %T", pkg["dependencies"])
	}
	if deps["koishi-plugin-yesimbot"] != "workspace:^" {
		t.Errorf("workspace dep wrong: %v", deps["koishi-plugin-yesimbot"])
	}
	if deps["koishi"] != "^4.18.4" {
		t.Errorf("infra dep wrong: %v", deps["koishi"])
	}
}

func TestGenerateYarnrc(t *testing.T) {
	content := GenerateYarnrc(".yarn/releases/yarn-4.12.0.cjs", "https://registry.npmmirror.com")
	for _, want := range []string{"nodeLinker: node-modules", "yarnPath: .yarn/releases/yarn-4.12.0.cjs", "npmRegistryServer: https://registry.npmmirror.com"} {
		if !strings.Contains(content, want) {
			t.Errorf(".yarnrc.yml missing %q", want)
		}
	}
}
