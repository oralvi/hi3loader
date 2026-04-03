package gameclient

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const executableName = "BH3.exe"

func NormalizePath(path string) string {
	return strings.Trim(strings.TrimSpace(path), `"'`)
}

func ResolveDir(path string) (string, error) {
	path = NormalizePath(path)
	if path == "" {
		return "", fmt.Errorf("game path is empty")
	}

	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("stat game path: %w", err)
	}

	dir := path
	if !info.IsDir() {
		dir = filepath.Dir(path)
	}

	dir, err = filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("resolve game path: %w", err)
	}

	if hasFile(dir, "config.ini") || hasFile(dir, executableName) {
		return dir, nil
	}

	return "", fmt.Errorf("game path must point to a BH3 install directory or %s", executableName)
}

func ResolveExecutable(path string) (string, error) {
	path = NormalizePath(path)
	if path == "" {
		return "", fmt.Errorf("game path is empty")
	}

	info, err := os.Stat(path)
	if err == nil && !info.IsDir() && strings.EqualFold(info.Name(), executableName) {
		exePath, absErr := filepath.Abs(path)
		if absErr != nil {
			return "", fmt.Errorf("resolve executable: %w", absErr)
		}
		return exePath, nil
	}

	dir, err := ResolveDir(path)
	if err != nil {
		return "", err
	}

	exePath := filepath.Join(dir, executableName)
	if _, err := os.Stat(exePath); err != nil {
		return "", fmt.Errorf("find %s: %w", executableName, err)
	}

	exePath, err = filepath.Abs(exePath)
	if err != nil {
		return "", fmt.Errorf("resolve executable: %w", err)
	}
	return exePath, nil
}

func Launch(path string) (string, error) {
	exePath, err := ResolveExecutable(path)
	if err != nil {
		return "", err
	}
	if err := launchExecutable(exePath); err != nil {
		return exePath, err
	}
	return exePath, nil
}

func hasFile(dir, name string) bool {
	info, err := os.Stat(filepath.Join(dir, name))
	return err == nil && !info.IsDir()
}
