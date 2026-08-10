package internal

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func zipFixture(t *testing.T, entries map[string]string) *zip.Reader {
	t.Helper()

	var archive bytes.Buffer
	writer := zip.NewWriter(&archive)
	for name, content := range entries {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	reader := bytes.NewReader(archive.Bytes())
	zipReader, err := zip.NewReader(reader, int64(reader.Len()))
	if err != nil {
		t.Fatal(err)
	}
	return zipReader
}

func TestExtractZipCopiesEveryBoilerplateFile(t *testing.T) {
	destination := filepath.Join(t.TempDir(), "app")
	archive := zipFixture(t, map[string]string{
		"boilerplate-master/README.md":                      "readme",
		"boilerplate-master/koishi.yml":                     "plugins: {}\n",
		"boilerplate-master/.github/workflows/ci.yml":       "name: CI\n",
		"boilerplate-master/.vscode/extensions.json":        "{}\n",
		"boilerplate-master/docker/Dockerfile":              "FROM node\n",
		"boilerplate-master/.yarn/releases/yarn-4.12.0.cjs": "yarn binary",
		"boilerplate-master/.yarnrc.yml":                    "yarnPath: .yarn/releases/yarn-4.12.0.cjs\n",
	})

	if err := extractZip(archive, destination); err != nil {
		t.Fatal(err)
	}

	for relativePath, want := range map[string]string{
		"README.md":                      "readme",
		"koishi.yml":                     "plugins: {}\n",
		".github/workflows/ci.yml":       "name: CI\n",
		".vscode/extensions.json":        "{}\n",
		"docker/Dockerfile":              "FROM node\n",
		".yarn/releases/yarn-4.12.0.cjs": "yarn binary",
		".yarnrc.yml":                    "yarnPath: .yarn/releases/yarn-4.12.0.cjs\n",
	} {
		content, err := os.ReadFile(filepath.Join(destination, relativePath))
		if err != nil {
			t.Errorf("missing %s: %v", relativePath, err)
			continue
		}
		if string(content) != want {
			t.Errorf("%s = %q, want %q", relativePath, content, want)
		}
	}
}

func TestExtractZipRejectsPathsOutsideDestination(t *testing.T) {
	root := t.TempDir()
	archive := zipFixture(t, map[string]string{
		"boilerplate-master/../../outside.txt": "must not write",
	})

	if err := extractZip(archive, filepath.Join(root, "app")); err == nil {
		t.Fatal("ZIP Slip archive accepted")
	}
	if _, err := os.Stat(filepath.Join(root, "outside.txt")); !os.IsNotExist(err) {
		t.Errorf("archive wrote outside destination: %v", err)
	}
}
