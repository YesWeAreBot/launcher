package internal

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestRemoveManagedAppPackageJSON(t *testing.T) {
	appDir := filepath.Join(t.TempDir(), "bot")
	sourceDir := filepath.Join(appDir, ".yesimbot", "source")
	plugins := []PluginInfo{
		{PackageName: "koishi-plugin-yesimbot", ConfigKey: "yesimbot", Group: "core", Enabled: true},
		{PackageName: "koishi-plugin-yesimbot-workspace", ConfigKey: "yesimbot-workspace", Group: "yesimbot"},
	}
	content := []byte(`{
  "name": "bot",
  "dependencies": {
    "koishi": "^4.18.0",
    "koishi-plugin-yesimbot": "workspace:^",
    "koishi-plugin-yesimbot-workspace": "workspace:^",
    "custom-plugin": "^1.0.0"
  },
  "workspaces": [
    "packages/*",
    ".yesimbot/source",
    ".yesimbot/source/core",
    ".yesimbot/source/packages/*",
    ".yesimbot/source/providers/*",
    ".yesimbot/source/plugins/*"
  ]
}`)

	updated, removed, err := RemoveManagedAppPackageJSON(content, appDir, sourceDir, plugins)
	if err != nil {
		t.Fatal(err)
	}
	if len(removed) != 7 {
		t.Fatalf("removed = %v, want 2 dependencies and 5 workspace entries", removed)
	}

	var pkg map[string]any
	if err := json.Unmarshal(updated, &pkg); err != nil {
		t.Fatal(err)
	}
	deps := pkg["dependencies"].(map[string]any)
	if deps["koishi"] == nil || deps["custom-plugin"] == nil {
		t.Errorf("user dependencies changed: %v", deps)
	}
	if _, ok := deps["koishi-plugin-yesimbot"]; ok {
		t.Error("core dependency still present")
	}
	if _, ok := deps["koishi-plugin-yesimbot-workspace"]; ok {
		t.Error("workspace plugin dependency still present")
	}

	workspaces := pkg["workspaces"].([]any)
	if len(workspaces) != 1 || workspaces[0] != "packages/*" {
		t.Errorf("workspaces = %v, want only packages/*", workspaces)
	}
}

func TestRemoveManagedKoishiYml(t *testing.T) {
	plugins := []PluginInfo{
		{PackageName: "koishi-plugin-yesimbot", ConfigKey: "yesimbot", Group: "core", Enabled: true},
		{PackageName: "@yesimbot/koishi-plugin-provider-openai", ConfigKey: "@yesimbot/provider-openai", Group: "provider"},
		{PackageName: "koishi-plugin-yesimbot-workspace", ConfigKey: "yesimbot-workspace", Group: "yesimbot"},
	}
	content := []byte(`plugins:
  yesimbot:abc:
    enabled: true
  custom:
    x: 1
  group:provider:
    $collapsed: true
    ~@yesimbot/provider-openai:def: {}
    other-provider: {}
  group:yesimbot:
    $collapsed: true
    ~yesimbot-workspace:ghi: {}
    user-plugin: {}
`)

	updated, removed, err := RemoveManagedKoishiYml(content, plugins)
	if err != nil {
		t.Fatal(err)
	}
	if len(removed) != 3 {
		t.Fatalf("removed = %v, want 3 managed entries", removed)
	}

	var config map[string]any
	if err := yaml.Unmarshal(updated, &config); err != nil {
		t.Fatal(err)
	}
	pluginsNode := config["plugins"].(map[string]any)
	if _, ok := pluginsNode["yesimbot:abc"]; ok {
		t.Error("core plugin entry still present")
	}
	if _, ok := pluginsNode["custom"]; !ok {
		t.Error("custom top-level plugin was removed")
	}

	providerGroup := pluginsNode["group:provider"].(map[string]any)
	if _, ok := providerGroup["~@yesimbot/provider-openai:def"]; ok {
		t.Error("provider entry still present")
	}
	if _, ok := providerGroup["other-provider"]; !ok {
		t.Error("unrelated provider was removed")
	}

	yesimbotGroup := pluginsNode["group:yesimbot"].(map[string]any)
	if _, ok := yesimbotGroup["~yesimbot-workspace:ghi"]; ok {
		t.Error("workspace plugin entry still present")
	}
	if _, ok := yesimbotGroup["user-plugin"]; !ok {
		t.Error("unrelated group:yesimbot plugin was removed")
	}
}

func TestUninstallMovesAppToBackup(t *testing.T) {
	parent := t.TempDir()
	appDir := filepath.Join(parent, "bot")
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(appDir, "package.json"), []byte(`{"name":"bot"}`))
	writeFile(t, filepath.Join(appDir, "koishi.yml"), []byte("plugins: {}\n"))
	if err := os.MkdirAll(filepath.Join(appDir, ".yesimbot"), 0o755); err != nil {
		t.Fatal(err)
	}

	result, err := Uninstall(UninstallOptions{AppDir: appDir}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.BackupDir == "" || result.KeptApp {
		t.Fatalf("unexpected result: %+v", result)
	}
	if _, err := os.Stat(appDir); !os.IsNotExist(err) {
		t.Errorf("app directory still exists: %v", err)
	}
	if _, err := os.Stat(result.BackupDir); err != nil {
		t.Errorf("backup directory missing: %v", err)
	}
	entries, err := os.ReadDir(parent)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || !strings.Contains(entries[0].Name(), ".yesimbot-uninstall-") {
		t.Errorf("backup entries = %v", entries)
	}
}

func TestUninstallKeepApp(t *testing.T) {
	appDir := filepath.Join(t.TempDir(), "bot")
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(appDir, "package.json"), []byte(`{
  "name": "bot",
  "dependencies": {
    "koishi": "^4.18.0",
    "koishi-plugin-yesimbot": "workspace:^",
    "custom-plugin": "^1.0.0"
  },
  "workspaces": [".yesimbot/source", "packages/*"]
}`))
	writeFile(t, filepath.Join(appDir, "koishi.yml"), []byte(`plugins:
  yesimbot:abc: {}
  custom: {}
`))
	plugins := []PluginInfo{
		{PackageName: "koishi-plugin-yesimbot", ConfigKey: "yesimbot", Group: "core", Enabled: true},
	}
	if err := MarkInitialized(Derive(appDir), "abc123", plugins...); err != nil {
		t.Fatal(err)
	}

	result, err := Uninstall(UninstallOptions{AppDir: appDir, KeepApp: true, SkipDeps: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !result.KeptApp || result.BackupDir != "" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if _, err := os.Stat(filepath.Join(appDir, ".yesimbot")); !os.IsNotExist(err) {
		t.Errorf("launcher directory still exists: %v", err)
	}

	packageContent, err := os.ReadFile(filepath.Join(appDir, "package.json"))
	if err != nil {
		t.Fatal(err)
	}
	var pkg map[string]any
	if err := json.Unmarshal(packageContent, &pkg); err != nil {
		t.Fatal(err)
	}
	deps := pkg["dependencies"].(map[string]any)
	if _, ok := deps["koishi-plugin-yesimbot"]; ok {
		t.Error("core dependency still present")
	}
	if deps["custom-plugin"] == nil || deps["koishi"] == nil {
		t.Errorf("user dependencies changed: %v", deps)
	}

	koishiContent, err := os.ReadFile(filepath.Join(appDir, "koishi.yml"))
	if err != nil {
		t.Fatal(err)
	}
	var config map[string]any
	if err := yaml.Unmarshal(koishiContent, &config); err != nil {
		t.Fatal(err)
	}
	pluginsNode := config["plugins"].(map[string]any)
	if _, ok := pluginsNode["yesimbot:abc"]; ok {
		t.Error("managed plugin entry still present")
	}
	if _, ok := pluginsNode["custom"]; !ok {
		t.Error("custom plugin was removed")
	}
}

func writeFile(t *testing.T, path string, content []byte) {
	t.Helper()
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
}
