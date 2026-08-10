package internal

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// ConsoleURL reads the Koishi server host/port from the App's koishi.yml
// (plugins.group:server.<server:*>). Defaults to 127.0.0.1:5140.
func ConsoleURL(paths AppPaths) string {
	content, err := os.ReadFile(paths.KoishiYml)
	if err != nil {
		return ""
	}
	var doc map[string]any
	if err := yaml.Unmarshal(content, &doc); err != nil {
		return ""
	}

	plugins, _ := doc["plugins"].(map[string]any)
	serverGroup, _ := plugins["group:server"].(map[string]any)

	host := "127.0.0.1"
	port := 5140
	for key, value := range serverGroup {
		if strings.HasPrefix(key, "~") || !strings.HasPrefix(key, "server") {
			continue
		}
		cfg, ok := value.(map[string]any)
		if !ok {
			continue
		}
		if h, ok := cfg["host"].(string); ok && h != "" {
			host = h
		}
		if p, ok := cfg["port"].(int); ok && p > 0 {
			port = p
		}
		break
	}

	if host == "0.0.0.0" {
		host = "127.0.0.1"
	}
	return fmt.Sprintf("http://%s:%d", host, port)
}
