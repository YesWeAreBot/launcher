package internal

import (
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

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

func TestBackupExistingAppConfigPreservesOriginalBytes(t *testing.T) {
	appDir := t.TempDir()
	paths := Derive(appDir)
	packageContent := []byte("{\n  \"name\": \"my-bot\"\n}\n")
	koishiContent := []byte("plugins:\n  custom: {}\n")
	if err := os.WriteFile(paths.PackageJson, packageContent, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.KoishiYml, koishiContent, 0o644); err != nil {
		t.Fatal(err)
	}

	timestamp := time.Date(2026, time.August, 10, 12, 0, 0, 0, time.UTC)
	if err := backupExistingAppConfig(paths, timestamp); err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string][]byte{
		paths.PackageJson + ".yesimbot.20260810T120000Z.bak": packageContent,
		paths.KoishiYml + ".yesimbot.20260810T120000Z.bak":   koishiContent,
	} {
		got, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("missing backup %s: %v", path, err)
			continue
		}
		if string(got) != string(want) {
			t.Errorf("backup %s = %q, want %q", path, got, want)
		}
	}
}

func TestCreateAppStructureExistingKeepsKoishiFiles(t *testing.T) {
	appDir := t.TempDir()
	paths := Derive(appDir)
	packageContent := []byte("{\"name\":\"my-bot\"}\n")
	koishiContent := []byte("plugins:\n  custom: {}\n")
	if err := os.WriteFile(paths.PackageJson, packageContent, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.KoishiYml, koishiContent, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := createAppStructure(&initContext{paths: paths, existing: true}); err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string][]byte{
		paths.PackageJson: packageContent,
		paths.KoishiYml:   koishiContent,
	} {
		got, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("missing existing file %s: %v", path, err)
			continue
		}
		if string(got) != string(want) {
			t.Errorf("existing file %s changed to %q", path, got)
		}
	}
	for _, dir := range []string{paths.YesimbotDir, paths.SourceDir, paths.LogsDir} {
		if info, err := os.Stat(dir); err != nil || !info.IsDir() {
			t.Errorf("launcher directory %s missing: %v", dir, err)
		}
	}
}

func TestSetupSourceReusesExistingDirectory(t *testing.T) {
	appDir := t.TempDir()
	paths := Derive(appDir)
	if err := os.MkdirAll(paths.SourceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	keep := filepath.Join(paths.SourceDir, "keep.txt")
	if err := os.WriteFile(keep, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}

	localDir := t.TempDir()
	if err := setupSource(&initContext{paths: paths, options: InitOptions{Local: localDir}}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(keep); err != nil {
		t.Errorf("existing source directory was replaced: %v", err)
	}
}
