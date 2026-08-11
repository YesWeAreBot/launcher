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
	Version     string `json:"version,omitempty"`
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
		groupOrder := map[string]int{"core": 0, "provider": 1, "yesimbot": 2, "adapter": 3}
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

// IsManagedPluginName reports whether a dependency belongs to the YesImBot workspace.
func IsManagedPluginName(name string) bool {
	return isKoishiPlugin(name)
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

// MergeExistingAppPackageJSON adds missing YesImBot workspace entries and
// dependencies without replacing existing package fields or dependency versions.
func MergeExistingAppPackageJSON(content []byte, appDir, sourceDir string, plugins []PluginInfo) ([]byte, []string, error) {
	var pkg map[string]any
	if err := json.Unmarshal(content, &pkg); err != nil {
		return nil, nil, fmt.Errorf("invalid package.json: %v", err)
	}

	workspaces := []string{
		toRelativePosix(appDir, sourceDir),
		toRelativePosix(appDir, filepath.Join(sourceDir, "core")),
		toRelativePosix(appDir, filepath.Join(sourceDir, "packages", "*")),
		toRelativePosix(appDir, filepath.Join(sourceDir, "providers", "*")),
		toRelativePosix(appDir, filepath.Join(sourceDir, "plugins", "*")),
	}
	if err := addWorkspaces(pkg, workspaces); err != nil {
		return nil, nil, err
	}

	deps, ok := pkg["dependencies"].(map[string]any)
	if !ok {
		if _, exists := pkg["dependencies"]; exists {
			return nil, nil, fmt.Errorf("package.json dependencies must be an object")
		}
		deps = map[string]any{}
		pkg["dependencies"] = deps
	}
	var conflicts []string
	for _, plugin := range plugins {
		if _, exists := deps[plugin.PackageName]; exists {
			conflicts = append(conflicts, plugin.PackageName)
			continue
		}
		version := plugin.Version
		if version == "" {
			version = "workspace:^"
		}
		deps[plugin.PackageName] = version
	}

	data, err := json.MarshalIndent(pkg, "", "  ")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal package.json: %v", err)
	}
	return append(data, '\n'), conflicts, nil
}

func addWorkspaces(pkg map[string]any, want []string) error {
	workspaces, exists := pkg["workspaces"]
	if !exists {
		pkg["workspaces"] = stringsToAny(want)
		return nil
	}
	if entries, ok := workspaces.([]any); ok {
		pkg["workspaces"] = appendMissingStrings(entries, want)
		return nil
	}
	workspaceConfig, ok := workspaces.(map[string]any)
	if !ok {
		return fmt.Errorf("package.json workspaces must be an array or an object with packages")
	}
	entries, ok := workspaceConfig["packages"].([]any)
	if !ok {
		return fmt.Errorf("package.json workspaces.packages must be an array")
	}
	workspaceConfig["packages"] = appendMissingStrings(entries, want)
	return nil
}

func stringsToAny(values []string) []any {
	entries := make([]any, len(values))
	for i, value := range values {
		entries[i] = value
	}
	return entries
}

func appendMissingStrings(entries []any, want []string) []any {
	seen := map[string]bool{}
	for _, entry := range entries {
		if value, ok := entry.(string); ok {
			seen[value] = true
		}
	}
	for _, value := range want {
		if !seen[value] {
			entries = append(entries, value)
		}
	}
	return entries
}

// ManagedWorkspacePatterns returns the workspace globs added by launcher init.
func ManagedWorkspacePatterns(appDir, sourceDir string) []string {
	return []string{
		toRelativePosix(appDir, sourceDir),
		toRelativePosix(appDir, filepath.Join(sourceDir, "core")),
		toRelativePosix(appDir, filepath.Join(sourceDir, "packages", "*")),
		toRelativePosix(appDir, filepath.Join(sourceDir, "providers", "*")),
		toRelativePosix(appDir, filepath.Join(sourceDir, "plugins", "*")),
	}
}

// RemoveManagedAppPackageJSON removes launcher-managed dependencies and workspace entries.
func RemoveManagedAppPackageJSON(content []byte, appDir, sourceDir string, plugins []PluginInfo) ([]byte, []string, error) {
	var pkg map[string]any
	if err := json.Unmarshal(content, &pkg); err != nil {
		return nil, nil, fmt.Errorf("invalid package.json: %v", err)
	}

	var removed []string
	if deps, ok := pkg["dependencies"].(map[string]any); ok {
		names := make(map[string]bool, len(plugins))
		for _, plugin := range plugins {
			names[plugin.PackageName] = true
		}
		for name := range deps {
			if len(names) > 0 {
				if !names[name] {
					continue
				}
			} else if !IsManagedPluginName(name) {
				continue
			}
			delete(deps, name)
			removed = append(removed, name)
		}
	}

	workspaceRemoved, err := removeManagedWorkspaces(pkg, ManagedWorkspacePatterns(appDir, sourceDir))
	if err != nil {
		return nil, nil, err
	}
	removed = append(removed, workspaceRemoved...)

	data, err := json.MarshalIndent(pkg, "", "  ")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal package.json: %v", err)
	}
	return append(data, '\n'), removed, nil
}

func removeManagedWorkspaces(pkg map[string]any, patterns []string) ([]string, error) {
	workspaces, exists := pkg["workspaces"]
	if !exists {
		return nil, nil
	}
	managed := make(map[string]bool, len(patterns))
	for _, pattern := range patterns {
		managed[pattern] = true
	}

	if entries, ok := workspaces.([]any); ok {
		kept, removed := filterManagedStrings(entries, managed)
		pkg["workspaces"] = kept
		return removed, nil
	}
	workspaceConfig, ok := workspaces.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("package.json workspaces must be an array or an object with packages")
	}
	entries, ok := workspaceConfig["packages"].([]any)
	if !ok {
		return nil, fmt.Errorf("package.json workspaces.packages must be an array")
	}
	kept, removed := filterManagedStrings(entries, managed)
	workspaceConfig["packages"] = kept
	return removed, nil
}

func filterManagedStrings(entries []any, managed map[string]bool) ([]any, []string) {
	kept := make([]any, 0, len(entries))
	var removed []string
	for _, entry := range entries {
		value, ok := entry.(string)
		if ok && managed[value] {
			removed = append(removed, value)
			continue
		}
		kept = append(kept, entry)
	}
	return kept, removed
}

func yesimPluginNodes(plugins []PluginInfo) (*yaml.Node, error) {
	pluginConfig := map[string]any{}
	for _, plugin := range plugins {
		value := map[string]any{}
		if plugin.Label != "" {
			value["$label"] = plugin.Label
		}
		if plugin.Group == "core" {
			pluginConfig[plugin.ConfigKey] = value
			continue
		}
		groupName := "group:" + plugin.Group
		group, ok := pluginConfig[groupName].(map[string]any)
		if !ok {
			group = map[string]any{"$collapsed": true}
			pluginConfig[groupName] = group
		}
		key := plugin.ConfigKey
		if !plugin.Enabled {
			key = "~" + key
		}
		group[key] = value
	}

	data, err := yaml.Marshal(map[string]any{"plugins": pluginConfig})
	if err != nil {
		return nil, fmt.Errorf("failed to build YesImBot plugin configuration: %v", err)
	}
	var document yaml.Node
	if err := yaml.Unmarshal(data, &document); err != nil {
		return nil, err
	}
	root := yamlMappingRoot(&document)
	pluginsNode, _ := yamlMapValue(root, "plugins")
	return pluginsNode, nil
}

// MergeExistingKoishiYml adds missing YesImBot plugin nodes while retaining
// every setting supplied by the Koishi boilerplate.
func MergeExistingKoishiYml(content []byte, plugins []PluginInfo) ([]byte, error) {
	var existing yaml.Node
	if err := yaml.Unmarshal(content, &existing); err != nil {
		return nil, fmt.Errorf("invalid koishi.yml: %v", err)
	}
	defaultPlugins, err := yesimPluginNodes(plugins)
	if err != nil {
		return nil, err
	}

	existingRoot := yamlMappingRoot(&existing)
	if existingRoot == nil || defaultPlugins == nil {
		return nil, fmt.Errorf("koishi.yml must contain a mapping")
	}
	existingPlugins, _ := yamlMapValue(existingRoot, "plugins")
	if existingPlugins == nil {
		existingRoot.Content = append(existingRoot.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "plugins"}, defaultPlugins)
	} else if existingPlugins.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("koishi.yml plugins must be a mapping")
	} else {
		mergePluginMapping(existingPlugins, defaultPlugins)
	}

	data, err := yaml.Marshal(&existing)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal koishi.yml: %v", err)
	}
	return data, nil
}

func yamlMappingRoot(node *yaml.Node) *yaml.Node {
	if node.Kind == yaml.DocumentNode && len(node.Content) == 1 {
		node = node.Content[0]
	}
	if node.Kind != yaml.MappingNode {
		return nil
	}
	return node
}

func yamlMapValue(mapping *yaml.Node, key string) (*yaml.Node, int) {
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			return mapping.Content[i+1], i
		}
	}
	return nil, -1
}

func yamlPluginValue(mapping *yaml.Node, key string) (*yaml.Node, int) {
	wanted := pluginBaseKey(key)
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if pluginBaseKey(mapping.Content[i].Value) == wanted {
			return mapping.Content[i+1], i
		}
	}
	return nil, -1
}

func pluginBaseKey(key string) string {
	key = strings.TrimPrefix(key, "~")
	base, _, _ := strings.Cut(key, ":")
	return base
}

func mergePluginMapping(existing, defaults *yaml.Node) {
	for i := 0; i+1 < len(defaults.Content); i += 2 {
		key, value := defaults.Content[i], defaults.Content[i+1]
		current, _ := yamlMapValue(existing, key.Value)
		if current == nil && !strings.HasPrefix(key.Value, "group:") {
			current, _ = yamlPluginValue(existing, key.Value)
		}
		if current == nil {
			existing.Content = append(existing.Content, key, value)
			continue
		}
		if strings.HasPrefix(key.Value, "group:") && current.Kind == yaml.MappingNode && value.Kind == yaml.MappingNode {
			mergePluginMapping(current, value)
		}
	}
}

// RemoveManagedKoishiYml removes launcher-managed YesImBot plugin entries
// while retaining unrelated user plugins.
func RemoveManagedKoishiYml(content []byte, plugins []PluginInfo) ([]byte, []string, error) {
	var document yaml.Node
	if err := yaml.Unmarshal(content, &document); err != nil {
		return nil, nil, fmt.Errorf("invalid koishi.yml: %v", err)
	}
	root := yamlMappingRoot(&document)
	if root == nil {
		return nil, nil, fmt.Errorf("koishi.yml must contain a mapping")
	}
	pluginsNode, _ := yamlMapValue(root, "plugins")
	if pluginsNode == nil {
		return content, nil, nil
	}
	if pluginsNode.Kind != yaml.MappingNode {
		return nil, nil, fmt.Errorf("koishi.yml plugins must be a mapping")
	}

	var removed []string
	removed = append(removed, removeMappingEntries(pluginsNode, func(key string) bool {
		return pluginBaseKey(key) == "yesimbot"
	})...)

	for _, groupName := range []string{"group:provider", "group:yesimbot", "group:adapter"} {
		group, _ := yamlMapValue(pluginsNode, groupName)
		if group == nil || group.Kind != yaml.MappingNode {
			continue
		}
		groupKey := "yesimbot"
		if groupName == "group:provider" {
			groupKey = "provider"
		} else if groupName == "group:adapter" {
			groupKey = "adapter"
		}
		removed = append(removed, removeMappingEntries(group, func(key string) bool {
			return isManagedGroupKey(pluginBaseKey(key), groupKey, plugins)
		})...)
		if len(group.Content) == 0 || onlyCollapsed(group) {
			removeMappingKey(pluginsNode, groupName)
		}
	}

	data, err := yaml.Marshal(&document)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal koishi.yml: %v", err)
	}
	return data, removed, nil
}

func isManagedGroupKey(base, group string, plugins []PluginInfo) bool {
	for _, plugin := range plugins {
		if plugin.Group == group && plugin.ConfigKey == base {
			return true
		}
	}
	if group == "provider" {
		return strings.HasPrefix(base, "@yesimbot/provider-")
	}
	return base == "yesimbot" || strings.HasPrefix(base, "yesimbot-")
}

func removeMappingEntries(mapping *yaml.Node, match func(string) bool) []string {
	var removed []string
	kept := make([]*yaml.Node, 0, len(mapping.Content))
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		key, value := mapping.Content[i], mapping.Content[i+1]
		if match(key.Value) {
			removed = append(removed, key.Value)
			continue
		}
		kept = append(kept, key, value)
	}
	mapping.Content = kept
	return removed
}

func removeMappingKey(mapping *yaml.Node, key string) bool {
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			mapping.Content = append(mapping.Content[:i], mapping.Content[i+2:]...)
			return true
		}
	}
	return false
}

func onlyCollapsed(mapping *yaml.Node) bool {
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value != "$collapsed" {
			return false
		}
	}
	return true
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
