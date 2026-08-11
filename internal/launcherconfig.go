package internal

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const defaultLauncherConfigYAML = `# Controls the default enabled state for discovered YesImBot plugins.
# Omitted plugins keep the launcher defaults (core enabled, others disabled).
plugins:
  koishi-plugin-yesimbot-console:
    enabled: true
  koishi-plugin-yesimbot-usage:
    enabled: true
`

type LauncherConfig struct {
	Plugins map[string]PluginConfig `yaml:"plugins"`
}

type PluginConfig struct {
	Enabled *bool `yaml:"enabled"`
}

func EnsureLauncherConfig(path string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to inspect launcher config: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("failed to create launcher config directory: %w", err)
	}
	if err := os.WriteFile(path, []byte(defaultLauncherConfigYAML), 0o644); err != nil {
		return fmt.Errorf("failed to write launcher config: %w", err)
	}
	return nil
}

func ReadLauncherConfig(path string) (LauncherConfig, error) {
	content, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return LauncherConfig{}, nil
	}
	if err != nil {
		return LauncherConfig{}, fmt.Errorf("failed to read launcher config: %w", err)
	}
	var config LauncherConfig
	if err := yaml.Unmarshal(content, &config); err != nil {
		return LauncherConfig{}, fmt.Errorf("invalid launcher config: %w", err)
	}
	return config, nil
}

func ApplyLauncherConfig(plugins []PluginInfo, config LauncherConfig) []PluginInfo {
	result := make([]PluginInfo, len(plugins))
	for i, plugin := range plugins {
		if entry, ok := config.Plugins[plugin.PackageName]; ok && entry.Enabled != nil {
			plugin.Enabled = *entry.Enabled
		}
		result[i] = plugin
	}
	return result
}

func DiscoverPluginsWithConfig(sourceDir, configPath string) ([]PluginInfo, error) {
	plugins, err := DiscoverPlugins(sourceDir)
	if err != nil {
		return nil, err
	}
	config, err := ReadLauncherConfig(configPath)
	if err != nil {
		return nil, err
	}
	return ApplyLauncherConfig(plugins, config), nil
}
