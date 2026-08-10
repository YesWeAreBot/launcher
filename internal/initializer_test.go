package internal

import (
	"net"
	"os"
	"path/filepath"
	"testing"
)

func TestReadYarnVersion(t *testing.T) {
	dir := t.TempDir()

	// Missing package.json -> default.
	if v := readYarnVersion(dir); v != "4.12.0" {
		t.Errorf("missing package.json: got %q", v)
	}

	os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"packageManager": "yarn@4.3.1"}`), 0o644)
	if v := readYarnVersion(dir); v != "4.3.1" {
		t.Errorf("got %q, want 4.3.1", v)
	}

	os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"packageManager": "pnpm@9"}`), 0o644)
	if v := readYarnVersion(dir); v != "4.12.0" {
		t.Errorf("non-yarn manager: got %q, want default", v)
	}
}

func TestFindYarnBinary(t *testing.T) {
	source := t.TempDir()
	releases := filepath.Join(source, ".yarn", "releases")
	os.MkdirAll(releases, 0o755)
	os.WriteFile(filepath.Join(releases, "yarn-4.12.0.cjs"), []byte("bin"), 0o644)
	os.WriteFile(filepath.Join(releases, "yarn-4.3.1.cjs"), []byte("bin"), 0o644)

	got := findYarnBinary(source, "/fallback/yarn.cjs")
	if got != filepath.Join(releases, "yarn-4.12.0.cjs") {
		t.Errorf("got %q, want first release sorted", got)
	}

	if got := findYarnBinary(t.TempDir(), "/fallback/yarn.cjs"); got != "/fallback/yarn.cjs" {
		t.Errorf("missing releases: got %q, want fallback", got)
	}
}

func TestProbeFastestFallback(t *testing.T) {
	// Unreachable candidates must fall back rather than hang.
	url := probeFastest([]string{"http://127.0.0.1:1/x", "http://127.0.0.1:2/y"}, "https://fallback.invalid")
	if url != "https://fallback.invalid" {
		t.Errorf("got %q, want fallback", url)
	}
}

func TestProbeFastestLocal(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	live := "http://" + ln.Addr().String() + "/probe"
	got := probeFastest([]string{"http://127.0.0.1:1/x", live}, "fallback")
	if got != live {
		t.Errorf("got %q, want live listener %q", got, live)
	}
}
