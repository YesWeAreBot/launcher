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
  adapter-onebot:
    package: koishi-plugin-adapter-onebot
    version: 6.9.4
    group: adapter
    label: OneBot
    enabled: false
  adapter-napcat:
    package: koishi-plugin-adapter-napcat
    version: 6.8.0-napcat.0
    group: adapter
    label: NapCat
    enabled: false
`

type LauncherConfig struct {
	Plugins map[string]PluginConfig `yaml:"plugins"`
}

type PluginConfig struct {
	Enabled     *bool  `yaml:"enabled"`
	PackageName string `yaml:"package,omitempty"`
	Version     string `yaml:"version,omitempty"`
	Group       string `yaml:"group,omitempty"`
	Label       string `yaml:"label,omitempty"`
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
		if entry, ok := config.Plugins[plugin.ConfigKey]; ok && entry.Enabled != nil {
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
	result := ApplyLauncherConfig(plugins, config)
	existing := make(map[string]bool, len(result)*2)
	for _, plugin := range result {
		existing[plugin.PackageName] = true
		existing[plugin.ConfigKey] = true
	}
	for key, entry := range config.Plugins {
		if entry.PackageName == "" || existing[key] || existing[entry.PackageName] {
			continue
		}
		group := entry.Group
		if group == "" {
			group = "adapter"
		}
		label := entry.Label
		if label == "" {
			label = key
		}
		enabled := false
		if entry.Enabled != nil {
			enabled = *entry.Enabled
		}
		result = append(result, PluginInfo{
			PackageName: entry.PackageName,
			ConfigKey:   key,
			Label:       label,
			Group:       group,
			Enabled:     enabled,
			Version:     entry.Version,
		})
	}
	return result, nil
}
