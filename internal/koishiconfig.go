package internal

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// PluginInfo describes a YesImBot Koishi plugin package.
type PluginInfo struct {
	PackageName string `json:"packageName"`
	ConfigKey   string `json:"configKey"`
	Label       string `json:"label,omitempty"`
	Group       string `json:"group"` // "core", "provider" or "yesimbot"
	Enabled     bool   `json:"enabled"`
}

// DiscoverPlugins scans a YesImBot workspace for Koishi plugin packages.
func DiscoverPlugins(sourceDir string) ([]PluginInfo, error) {
	var plugins []PluginInfo
	dirs := []string{"core", "providers", "plugins", "packages"}

	for _, dir := range dirs {
		base := filepath.Join(sourceDir, dir)

		if dir == "core" {
			// core/ is itself a single package.
			name, err := readPackageName(filepath.Join(base, "package.json"))
			if err == nil && isKoishiPlugin(name) {
				plugins = append(plugins, classifyPlugin(name))
			}
			continue
		}

		entries, err := os.ReadDir(base)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			name, err := readPackageName(filepath.Join(base, entry.Name(), "package.json"))
			if err != nil || !isKoishiPlugin(name) {
				continue
			}
			plugins = append(plugins, classifyPlugin(name))
		}
	}

	if len(plugins) == 0 {
		return nil, fmt.Errorf("no Koishi plugin packages found in: %s", sourceDir)
	}

	sort.Slice(plugins, func(i, j int) bool {
		groupOrder := map[string]int{"core": 0, "provider": 1, "yesimbot": 2}
		gi, gj := groupOrder[plugins[i].Group], groupOrder[plugins[j].Group]
		if gi != gj {
			return gi < gj
		}
		return plugins[i].PackageName < plugins[j].PackageName
	})
	return plugins, nil
}

func readPackageName(pkgPath string) (string, error) {
	content, err := os.ReadFile(pkgPath)
	if err != nil {
		return "", err
	}
	var pkg struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(content, &pkg); err != nil {
		return "", err
	}
	return pkg.Name, nil
}

func isKoishiPlugin(name string) bool {
	return name == "koishi-plugin-yesimbot" ||
		strings.HasPrefix(name, "koishi-plugin-yesimbot-") ||
		strings.HasPrefix(name, "@yesimbot/koishi-plugin-provider-")
}

// PackageNameToConfigKey maps a package name to its Koishi config key:
//
//	koishi-plugin-yesimbot                  -> yesimbot
//	@yesimbot/koishi-plugin-provider-openai -> @yesimbot/provider-openai
//	koishi-plugin-yesimbot-workspace        -> yesimbot-workspace
func PackageNameToConfigKey(name string) string {
	if strings.HasPrefix(name, "@") {
		slashIdx := strings.Index(name, "/")
		if slashIdx >= 0 {
			scope := name[:slashIdx+1]
			rest := name[slashIdx+1:]
			if strings.HasPrefix(rest, "koishi-plugin-") {
				return scope + rest[len("koishi-plugin-"):]
			}
		}
		return name
	}
	if strings.HasPrefix(name, "koishi-plugin-") {
		return name[len("koishi-plugin-"):]
	}
	return name
}

func classifyPlugin(packageName string) PluginInfo {
	configKey := PackageNameToConfigKey(packageName)

	switch {
	case packageName == "koishi-plugin-yesimbot":
		return PluginInfo{PackageName: packageName, ConfigKey: configKey, Group: "core", Enabled: true}
	case strings.HasPrefix(packageName, "@yesimbot/koishi-plugin-provider-"):
		return PluginInfo{
			PackageName: packageName,
			ConfigKey:   configKey,
			Label:       strings.TrimPrefix(packageName, "@yesimbot/koishi-plugin-provider-"),
			Group:       "provider",
			Enabled:     false,
		}
	case strings.HasPrefix(packageName, "koishi-plugin-yesimbot-"):
		return PluginInfo{
			PackageName: packageName,
			ConfigKey:   configKey,
			Label:       strings.TrimPrefix(packageName, "koishi-plugin-yesimbot-"),
			Group:       "yesimbot",
			Enabled:     false,
		}
	}
	// Unreachable given isKoishiPlugin filtering.
	return PluginInfo{PackageName: packageName, ConfigKey: configKey, Group: "yesimbot", Enabled: false}
}

// GenerateKoishiYml builds the App koishi.yml: infrastructure groups with
// safe defaults, the YesImBot core plugin enabled at the top level, and
// providers/YesImBot plugins listed but disabled via the ~ prefix.
// Follows the design doc's default blocks exactly (server, database-sqlite,
// maxPort: 5199, no assets-local).
func GenerateKoishiYml(plugins []PluginInfo, serverPort int) (string, error) {
	if serverPort == 0 {
		serverPort = 5140
	}

	pluginsSection := map[string]any{
		"group:server": map[string]any{
			"server": map[string]any{
				"host":    "127.0.0.1",
				"port":    serverPort,
				"maxPort": 5199,
			},
		},
		"group:console": map[string]any{
			"console":  map[string]any{},
			"config":   map[string]any{},
			"explorer": map[string]any{},
			"logger":   map[string]any{},
			"status":   map[string]any{},
		},
		"group:storage": map[string]any{
			"database-sqlite": map[string]any{
				"path": "data/koishi.db",
			},
		},
	}

	// Core plugin at the plugins top level.
	for _, p := range plugins {
		if p.Group == "core" {
			pluginsSection[p.ConfigKey] = map[string]any{}
			break
		}
	}

	// Provider and YesImBot groups, keyed plugin:instance with $label;
	// disabled plugins keep the full key with ~ prefix.
	for _, group := range []string{"provider", "yesimbot"} {
		groupSection := map[string]any{}
		for _, p := range plugins {
			if p.Group != group {
				continue
			}
			key := p.ConfigKey
			if !p.Enabled {
				key = "~" + key
			}
			value := map[string]any{}
			if p.Label != "" {
				value["$label"] = p.Label
			}
			groupSection[key] = value
		}
		if len(groupSection) > 0 {
			pluginsSection["group:"+group] = groupSection
		}
	}

	data, err := yaml.Marshal(map[string]any{"plugins": pluginsSection})
	if err != nil {
		return "", fmt.Errorf("failed to marshal koishi.yml: %v", err)
	}
	return string(data), nil
}

// GenerateAppPackageJson builds the App package.json: workspaces pointing
// at the YesImBot source, workspace:^ deps for YesImBot packages, and
// pinned Koishi infrastructure dependencies.
func GenerateAppPackageJson(opts struct {
	AppDir      string
	SourceDir   string
	Plugins     []PluginInfo
	YarnVersion string
}) (map[string]any, error) {
	relSource := toRelativePosix(opts.AppDir, opts.SourceDir)

	workspaces := []string{
		relSource,
		relSource + "/core",
		relSource + "/packages/*",
		relSource + "/providers/*",
		relSource + "/plugins/*",
	}

	deps := map[string]string{
		"@koishijs/plugin-console":         "^5.30.4",
		"@koishijs/plugin-config":          "^2.8.6",
		"@koishijs/plugin-database-sqlite": "^4.6.0",
		"@koishijs/plugin-explorer":        "^1.5.5",
		"@koishijs/plugin-logger":          "^2.6.9",
		"@koishijs/plugin-server":          "^3.2.4",
		"@koishijs/plugin-status":          "^7.4.10",
		"koishi":                           "^4.18.4",
	}
	for _, p := range opts.Plugins {
		deps[p.PackageName] = "workspace:^"
	}

	keys := make([]string, 0, len(deps))
	for k := range deps {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	sorted := make(map[string]string, len(keys))
	for _, k := range keys {
		sorted[k] = deps[k]
	}

	return map[string]any{
		"name":            "yesimbot-app",
		"version":         "0.0.0",
		"private":         true,
		"type":            "module",
		"packageManager":  "yarn@" + opts.YarnVersion,
		"workspaces":      workspaces,
		"scripts":         map[string]string{"start": "koishi start"},
		"dependencies":    sorted,
		"devDependencies": map[string]string{"yml-register": "^1.2.5"},
	}, nil
}

// GenerateYarnrc builds the App .yarnrc.yml content.
func GenerateYarnrc(yarnRelativePath, registryURL string) string {
	return "nodeLinker: node-modules\n" +
		"yarnPath: " + yarnRelativePath + "\n" +
		"npmRegistryServer: " + registryURL + "\n"
}

func toRelativePosix(from, to string) string {
	rel, err := filepath.Rel(from, to)
	if err != nil {
		return to
	}
	return filepath.ToSlash(rel)
}
