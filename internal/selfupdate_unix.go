//go:build !windows

package internal

import "os"

func installBinary(source, target string) error {
	if err := os.Rename(source, target); err != nil {
		return err
	}
	return os.Chmod(target, 0o755)
}
