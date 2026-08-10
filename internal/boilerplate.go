package internal

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const boilerplateZipURL = "https://codeload.github.com/koishijs/boilerplate/zip/refs/heads/master"

func downloadBoilerplate(destination string) error {
	client := &http.Client{Timeout: time.Minute}
	response, err := client.Get(boilerplateZipURL)
	if err != nil {
		return fmt.Errorf("failed to download Koishi boilerplate: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download Koishi boilerplate: %s", response.Status)
	}

	file, err := os.CreateTemp("", "yesimbot-boilerplate-*.zip")
	if err != nil {
		return fmt.Errorf("failed to create boilerplate archive: %v", err)
	}
	archivePath := file.Name()
	defer os.Remove(archivePath)
	if _, err := io.Copy(file, response.Body); err != nil {
		file.Close()
		return fmt.Errorf("failed to save Koishi boilerplate: %v", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("failed to close boilerplate archive: %v", err)
	}

	archive, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("invalid Koishi boilerplate archive: %v", err)
	}
	defer archive.Close()
	return extractZip(&archive.Reader, destination)
}

func extractZip(archive *zip.Reader, destination string) error {
	root, err := zipRootDirectory(archive.File)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(destination, 0o755); err != nil {
		return fmt.Errorf("failed to create boilerplate directory: %v", err)
	}

	for _, entry := range archive.File {
		target, skip, err := zipTarget(destination, root, entry.Name)
		if err != nil {
			return err
		}
		if skip {
			continue
		}
		if entry.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return fmt.Errorf("failed to extract directory %s: %v", entry.Name, err)
			}
			continue
		}
		if entry.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing symbolic link in boilerplate archive: %s", entry.Name)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return fmt.Errorf("failed to create directory for %s: %v", entry.Name, err)
		}
		source, err := entry.Open()
		if err != nil {
			return fmt.Errorf("failed to read archive entry %s: %v", entry.Name, err)
		}
		mode := entry.Mode()
		if mode == 0 {
			mode = 0o644
		}
		dest, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
		if err != nil {
			source.Close()
			return fmt.Errorf("failed to create %s: %v", entry.Name, err)
		}
		_, err = io.Copy(dest, source)
		closeErr := dest.Close()
		source.Close()
		if err != nil {
			return fmt.Errorf("failed to extract %s: %v", entry.Name, err)
		}
		if closeErr != nil {
			return fmt.Errorf("failed to close extracted file %s: %v", entry.Name, closeErr)
		}
	}
	return nil
}

func zipRootDirectory(entries []*zip.File) (string, error) {
	if len(entries) == 0 {
		return "", fmt.Errorf("boilerplate archive is empty")
	}
	var root string
	for _, entry := range entries {
		name := strings.TrimPrefix(filepath.ToSlash(entry.Name), "./")
		part, _, _ := strings.Cut(name, "/")
		if part == "" || part == "." || part == ".." {
			return "", fmt.Errorf("invalid archive entry: %s", entry.Name)
		}
		if root == "" {
			root = part
		} else if root != part {
			return "", fmt.Errorf("boilerplate archive has multiple root directories")
		}
	}
	return root, nil
}

func zipTarget(destination, root, name string) (string, bool, error) {
	name = strings.TrimPrefix(filepath.ToSlash(name), "./")
	if name == root || name == root+"/" {
		return "", true, nil
	}
	prefix := root + "/"
	if !strings.HasPrefix(name, prefix) {
		return "", false, fmt.Errorf("archive entry is outside root directory: %s", name)
	}
	target := filepath.Join(destination, filepath.FromSlash(strings.TrimPrefix(name, prefix)))
	rel, err := filepath.Rel(destination, target)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false, fmt.Errorf("archive entry escapes destination: %s", name)
	}
	return target, false, nil
}
