package internal

import "testing"

func TestGitHubURLUsesOfficialBaseByDefault(t *testing.T) {
	t.Setenv("GITHUB_MIRROR", "")
	if got := githubURL("owner/repo/releases/download/nightly/app"); got != "https://github.com/owner/repo/releases/download/nightly/app" {
		t.Fatalf("got %q", got)
	}
	if got := githubArchiveURL("owner/repo/zip/refs/heads/main"); got != "https://codeload.github.com/owner/repo/zip/refs/heads/main" {
		t.Fatalf("got %q", got)
	}
}

func TestGitHubURLUsesTrimmedMirror(t *testing.T) {
	t.Setenv("GITHUB_MIRROR", " https://mirror.example.com/// ")
	path := "owner/repo/releases/download/nightly/app"
	if got := githubURL(path); got != "https://mirror.example.com/owner/repo/releases/download/nightly/app" {
		t.Fatalf("got %q", got)
	}
	if got := githubArchiveURL("owner/repo/zip/refs/heads/main"); got != "https://mirror.example.com/owner/repo/zip/refs/heads/main" {
		t.Fatalf("got %q", got)
	}
}
