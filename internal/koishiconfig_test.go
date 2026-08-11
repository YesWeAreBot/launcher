package internal

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
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
		"plugins/yesimbot-console":     "koishi-plugin-yesimbot-console",
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

func TestGenerateYarnrc(t *testing.T) {
	content := GenerateYarnrc(".yarn/releases/yarn-4.12.0.cjs", "https://registry.npmmirror.com")
	for _, want := range []string{"nodeLinker: node-modules", "yarnPath: .yarn/releases/yarn-4.12.0.cjs", "npmRegistryServer: https://registry.npmmirror.com"} {
		if !strings.Contains(content, want) {
			t.Errorf(".yarnrc.yml missing %q", want)
		}
	}
}

func TestMergeExistingAppPackageJSONKeepsUserValues(t *testing.T) {
	appDir := filepath.Join(t.TempDir(), "bot")
	sourceDir := filepath.Join(appDir, ".yesimbot", "source")
	plugins := []PluginInfo{
		{PackageName: "koishi-plugin-yesimbot", ConfigKey: "yesimbot", Group: "core", Enabled: true},
		{PackageName: "@yesimbot/koishi-plugin-provider-openai", ConfigKey: "@yesimbot/provider-openai", Group: "provider"},
	}

	merged, conflicts, err := MergeExistingAppPackageJSON([]byte(`{
  "name": "my-koishi",
  "packageManager": "yarn@4.12.0",
  "scripts": {"dev": "koishi start"},
  "devDependencies": {"yml-register": "^1.2.5"},
  "workspaces": {"packages": ["packages/*"]},
  "dependencies": {
    "koishi": "^4.18.0",
    "koishi-plugin-yesimbot": "^0.1.0",
    "other-plugin": "^1.0.0"
  }
}`), appDir, sourceDir, plugins)
	if err != nil {
		t.Fatal(err)
	}

	var pkg map[string]any
	if err := json.Unmarshal(merged, &pkg); err != nil {
		t.Fatal(err)
	}
	if pkg["name"] != "my-koishi" || pkg["packageManager"] != "yarn@4.12.0" || pkg["scripts"].(map[string]any)["dev"] != "koishi start" || pkg["devDependencies"].(map[string]any)["yml-register"] != "^1.2.5" {
		t.Errorf("non-YesImBot package fields changed: %v", pkg)
	}
	deps := pkg["dependencies"].(map[string]any)
	if deps["koishi-plugin-yesimbot"] != "^0.1.0" || deps["other-plugin"] != "^1.0.0" {
		t.Errorf("existing dependencies changed: %v", deps)
	}
	if deps["@yesimbot/koishi-plugin-provider-openai"] != "workspace:^" {
		t.Errorf("missing added YesImBot dependency: %v", deps)
	}
	if len(conflicts) != 1 || conflicts[0] != "koishi-plugin-yesimbot" {
		t.Errorf("conflicts = %v, want existing YesImBot dependency", conflicts)
	}

	workspaces := pkg["workspaces"].(map[string]any)["packages"].([]any)
	wantWorkspaces := []string{"packages/*", ".yesimbot/source", ".yesimbot/source/core", ".yesimbot/source/packages/*", ".yesimbot/source/providers/*", ".yesimbot/source/plugins/*"}
	if len(workspaces) != len(wantWorkspaces) {
		t.Fatalf("workspaces = %v, want %v", workspaces, wantWorkspaces)
	}
	for i, want := range wantWorkspaces {
		if workspaces[i] != want {
			t.Errorf("workspaces[%d] = %q, want %q", i, workspaces[i], want)
		}
	}
}

func TestMergeExistingKoishiYmlKeepsPluginConfiguration(t *testing.T) {
	plugins := []PluginInfo{
		{PackageName: "koishi-plugin-yesimbot", ConfigKey: "yesimbot", Group: "core", Enabled: true},
		{PackageName: "@yesimbot/koishi-plugin-provider-openai", ConfigKey: "@yesimbot/provider-openai", Label: "openai", Group: "provider"},
	}

	merged, err := MergeExistingKoishiYml([]byte(`plugins:
  yesimbot:
    endpoint: https://example.com/api
  custom:
    enabled: true
  group:provider:
    existing: {}
`), plugins)
	if err != nil {
		t.Fatal(err)
	}

	var config map[string]any
	if err := yaml.Unmarshal(merged, &config); err != nil {
		t.Fatal(err)
	}
	configuredPlugins := config["plugins"].(map[string]any)
	if configuredPlugins["yesimbot"].(map[string]any)["endpoint"] != "https://example.com/api" {
		t.Errorf("YesImBot configuration changed: %v", configuredPlugins["yesimbot"])
	}
	if configuredPlugins["custom"].(map[string]any)["enabled"] != true {
		t.Errorf("unrelated plugin changed: %v", configuredPlugins["custom"])
	}
	providerGroup := configuredPlugins["group:provider"].(map[string]any)
	if _, ok := providerGroup["existing"]; !ok {
		t.Errorf("existing provider changed: %v", providerGroup)
	}
	if _, ok := providerGroup["~@yesimbot/provider-openai"]; !ok {
		t.Errorf("missing added provider: %v", providerGroup)
	}
}

func TestMergeExistingKoishiYmlKeepsRandomizedPluginInstances(t *testing.T) {
	plugins := []PluginInfo{
		{PackageName: "koishi-plugin-yesimbot", ConfigKey: "yesimbot", Group: "core", Enabled: true},
		{PackageName: "@yesimbot/koishi-plugin-provider-openai", ConfigKey: "@yesimbot/provider-openai", Label: "openai", Group: "provider"},
	}

	merged, err := MergeExistingKoishiYml([]byte(`plugins:
  group:server:
    server:epkrgb:
      port: 5140
  group:console:
    config:9o711a: {}
  group:provider:
    ~@yesimbot/provider-openai:ogypwj:
      $label: openai
  yesimbot:j8hpco: {}
`), plugins)
	if err != nil {
		t.Fatal(err)
	}

	var config map[string]any
	if err := yaml.Unmarshal(merged, &config); err != nil {
		t.Fatal(err)
	}
	configuredPlugins := config["plugins"].(map[string]any)
	if _, exists := configuredPlugins["group:storage"]; exists {
		t.Error("merge added generated storage configuration instead of retaining the template")
	}
	if group := configuredPlugins["group:console"].(map[string]any); len(group) != 1 || group["config:9o711a"] == nil {
		t.Errorf("console instances changed: %v", group)
	}
	if group := configuredPlugins["group:provider"].(map[string]any); len(group) != 2 || group["$collapsed"] != true || group["~@yesimbot/provider-openai:ogypwj"] == nil {
		t.Errorf("provider instances changed: %v", group)
	}
	if _, exists := configuredPlugins["yesimbot"]; exists {
		t.Error("merge added an unrandomized YesImBot instance")
	}
	if _, exists := configuredPlugins["yesimbot:j8hpco"]; !exists {
		t.Error("existing randomized YesImBot instance was removed")
	}
}

func TestMergeExistingKoishiYmlCreatesCollapsedYesImBotGroups(t *testing.T) {
	plugins := []PluginInfo{
		{PackageName: "koishi-plugin-yesimbot", ConfigKey: "yesimbot", Group: "core", Enabled: true},
		{PackageName: "@yesimbot/koishi-plugin-provider-openai", ConfigKey: "@yesimbot/provider-openai", Label: "openai", Group: "provider"},
		{PackageName: "koishi-plugin-yesimbot-workspace", ConfigKey: "yesimbot-workspace", Label: "workspace", Group: "yesimbot"},
	}

	merged, err := MergeExistingKoishiYml([]byte(`plugins:
  group:server:
    $collapsed: true
`), plugins)
	if err != nil {
		t.Fatal(err)
	}

	var config map[string]any
	if err := yaml.Unmarshal(merged, &config); err != nil {
		t.Fatal(err)
	}
	configuredPlugins := config["plugins"].(map[string]any)
	serverGroup := configuredPlugins["group:server"].(map[string]any)
	if len(serverGroup) != 1 || serverGroup["$collapsed"] != true {
		t.Errorf("YesImBot plugins leaked into group:server: %v", serverGroup)
	}
	for groupName, pluginKey := range map[string]string{
		"group:provider": "~@yesimbot/provider-openai",
		"group:yesimbot": "~yesimbot-workspace",
	} {
		group, ok := configuredPlugins[groupName].(map[string]any)
		if !ok {
			t.Errorf("missing %s: %v", groupName, configuredPlugins)
			continue
		}
		if group["$collapsed"] != true {
			t.Errorf("%s is not collapsed: %v", groupName, group)
		}
		if _, ok := group[pluginKey]; !ok {
			t.Errorf("%s missing %s: %v", groupName, pluginKey, group)
		}
	}
}
