package internal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureLauncherConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".yesimbot", "launcher.yaml")
	if err := EnsureLauncherConfig(path); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"koishi-plugin-yesimbot-console",
		"koishi-plugin-yesimbot-usage",
		"koishi-plugin-adapter-onebot",
		"koishi-plugin-adapter-napcat",
	} {
		if !strings.Contains(string(content), want) {
			t.Errorf("default launcher config missing %q", want)
		}
	}
	if err := EnsureLauncherConfig(path); err != nil {
		t.Fatalf("second ensure should be a no-op: %v", err)
	}
}

func TestReadLauncherConfigMissing(t *testing.T) {
	config, err := ReadLauncherConfig(filepath.Join(t.TempDir(), "missing.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(config.Plugins) != 0 {
		t.Errorf("missing config should be empty: %+v", config)
	}
}

func TestApplyLauncherConfig(t *testing.T) {
	enabled := true
	plugins := []PluginInfo{
		{PackageName: "koishi-plugin-yesimbot-console", Enabled: false},
		{PackageName: "koishi-plugin-yesimbot-usage", Enabled: false},
	}
	config := LauncherConfig{Plugins: map[string]PluginConfig{
		"koishi-plugin-yesimbot-console": {Enabled: &enabled},
	}}

	result := ApplyLauncherConfig(plugins, config)
	if !result[0].Enabled {
		t.Error("console plugin should be enabled by config")
	}
	if result[1].Enabled {
		t.Error("plugin without config entry should keep its default")
	}
}

func TestDiscoverPluginsWithConfig(t *testing.T) {
	sourceDir := writeFixture(t)
	configPath := filepath.Join(sourceDir, ".yesimbot", "launcher.yaml")
	if err := EnsureLauncherConfig(configPath); err != nil {
		t.Fatal(err)
	}

	plugins, err := DiscoverPluginsWithConfig(sourceDir, configPath)
	if err != nil {
		t.Fatal(err)
	}
	byKey := map[string]PluginInfo{}
	for _, plugin := range plugins {
		byKey[plugin.PackageName] = plugin
	}
	if !byKey["koishi-plugin-yesimbot-console"].Enabled {
		t.Error("console plugin should be enabled through launcher config")
	}
	if plugin := byKey["koishi-plugin-adapter-onebot"]; plugin.Enabled || plugin.Version != "6.9.4" || plugin.Group != "adapter" {
		t.Errorf("onebot adapter config wrong: %+v", plugin)
	}
	if plugin := byKey["koishi-plugin-adapter-napcat"]; plugin.Enabled || plugin.Version != "6.8.0-napcat.0" || plugin.Group != "adapter" {
		t.Errorf("napcat adapter config wrong: %+v", plugin)
	}
}
