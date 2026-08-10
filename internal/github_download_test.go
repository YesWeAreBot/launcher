package internal

import (
	"archive/zip"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestDownloadBoilerplateUsesGitHubMirror(t *testing.T) {
	const path = "/koishijs/boilerplate/zip/refs/heads/master"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != path {
			t.Errorf("requested path %q, want %q", r.URL.Path, path)
			http.NotFound(w, r)
			return
		}
		archive := zip.NewWriter(w)
		entry, err := archive.Create("boilerplate-master/koishi.yml")
		if err != nil {
			t.Error(err)
			return
		}
		if _, err := entry.Write([]byte("plugins: {}\n")); err != nil {
			t.Error(err)
			return
		}
		if err := archive.Close(); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	t.Setenv("GITHUB_MIRROR", server.URL)

	destination := filepath.Join(t.TempDir(), "app")
	if err := downloadBoilerplate(destination); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(destination, "koishi.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "plugins: {}\n" {
		t.Fatalf("koishi.yml = %q", content)
	}
}
