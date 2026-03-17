//go:build !windows

package gameclient

import (
	"fmt"
	"os/exec"
	"path/filepath"
)

func launchExecutable(exePath string) error {
	cmd := exec.Command(exePath)
	cmd.Dir = filepath.Dir(exePath)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("launch game: %w", err)
	}
	return nil
}
