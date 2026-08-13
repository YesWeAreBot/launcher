package internal

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

const DefaultChannel = "nightly"

type SelfUpdateOptions struct {
	Channel        string
	CheckOnly      bool
	CurrentVersion string
}

type SelfUpdateResult struct {
	Executable     string
	AssetURL       string
	CurrentVersion string
	LatestVersion  string
	Applied        bool
}

var channelPattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

func SelfUpdate(options SelfUpdateOptions) (SelfUpdateResult, error) {
	channel := options.Channel
	if channel == "" {
		channel = DefaultChannel
	}
	if !channelPattern.MatchString(channel) {
		return SelfUpdateResult{}, fmt.Errorf("invalid channel: %s", channel)
	}

	executable, err := os.Executable()
	if err != nil {
		return SelfUpdateResult{}, fmt.Errorf("cannot locate current executable: %w", err)
	}
	asset, err := updateAssetURL(channel)
	if err != nil {
		return SelfUpdateResult{}, err
	}
	result := SelfUpdateResult{
		Executable:     executable,
		AssetURL:       asset,
		CurrentVersion: options.CurrentVersion,
	}

	client := &http.Client{Timeout: 5 * time.Minute}
	if options.CheckOnly {
		temp, err := downloadUpdate(client, asset, filepath.Dir(executable))
		if err != nil {
			return result, fmt.Errorf("failed to check update: %w", err)
		}
		defer os.Remove(temp)
		latest, err := binaryVersion(temp)
		if err != nil {
			return result, fmt.Errorf("failed to check update: %w", err)
		}
		result.LatestVersion = latest
		return result, nil
	}

	temp, err := downloadUpdate(client, asset, filepath.Dir(executable))
	if err != nil {
		return result, err
	}
	if err := installBinary(temp, executable); err != nil {
		os.Remove(temp)
		return result, fmt.Errorf("failed to install update: %w", err)
	}
	result.Applied = true
	return result, nil
}

func binaryVersion(path string) (string, error) {
	if runtime.GOOS != "windows" {
		if err := os.Chmod(path, 0o755); err != nil {
			return "", fmt.Errorf("cannot make release binary executable: %w", err)
		}
	}
	output, err := exec.Command(path, "--version").Output()
	if err != nil {
		return "", fmt.Errorf("cannot query release binary version: %w", err)
	}
	version := strings.TrimSpace(string(output))
	if index := strings.LastIndex(version, " "); index >= 0 {
		version = strings.TrimSpace(version[index+1:])
	}
	if version == "" {
		return "", fmt.Errorf("release binary returned an empty version")
	}
	return version, nil
}

func updateAssetURL(channel string) (string, error) {
	mirror := os.Getenv("GITHUB_MIRROR")
	if mirror == "" {
		mirror = "https://github.com"
	}
	asset := fmt.Sprintf("yesimbot-cli-%s-%s", runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS == "windows" {
		asset += ".exe"
	}
	return fmt.Sprintf("%s/YesWeAreBot/launcher/releases/download/%s/%s", mirror, channel, asset), nil
}

func downloadUpdate(client *http.Client, url, dir string) (string, error) {
	response, err := client.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to download update: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", fmt.Errorf("failed to download update: %s", response.Status)
	}

	pattern := ".yesimbot-cli-update-*"
	if runtime.GOOS == "windows" {
		pattern += ".exe"
	}
	file, err := os.CreateTemp(dir, pattern)
	if err != nil {
		return "", fmt.Errorf("failed to create temporary update file: %w", err)
	}
	if _, err := io.Copy(file, response.Body); err != nil {
		file.Close()
		os.Remove(file.Name())
		return "", fmt.Errorf("failed to save update: %w", err)
	}
	if err := file.Close(); err != nil {
		os.Remove(file.Name())
		return "", fmt.Errorf("failed to finalize update: %w", err)
	}
	return file.Name(), nil
}
