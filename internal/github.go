package internal

import (
	"os"
	"strings"
)

const githubURLBase = "https://github.com"

func githubMirrorRoot() string {
	mirror := strings.TrimSpace(os.Getenv("GITHUB_MIRROR"))
	if mirror == "" {
		return ""
	}
	return strings.TrimRight(mirror, "/")
}

func githubURL(path string) string {
	base := githubURLBase
	if mirror := githubMirrorRoot(); mirror != "" {
		base = mirror
	}
	return base + "/" + strings.TrimLeft(path, "/")
}

func githubArchiveURL(path string) string {
	if mirror := githubMirrorRoot(); mirror != "" {
		return mirror + "/" + strings.TrimLeft(path, "/")
	}
	return "https://codeload.github.com/" + strings.TrimLeft(path, "/")
}
