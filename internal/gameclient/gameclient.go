package gameclient

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const executableName = "BH3.exe"

var versionPattern = regexp.MustCompile(`\b\d+\.\d+\.\d+\b`)

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

func ReadVersion(path string) (string, error) {
	dir, err := ResolveDir(path)
	if err != nil {
		return "", err
	}

	for _, candidate := range []string{
		filepath.Join(dir, "config.ini"),
		filepath.Join(dir, "pkg_version"),
		filepath.Join(dir, "BH3_Data", "StreamingAssets", "build_info.txt"),
	} {
		version, candidateErr := readVersionFromFile(candidate)
		if candidateErr == nil {
			return version, nil
		}
	}

	return "", fmt.Errorf("game version not found under %s", dir)
}

func readVersionFromFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	text := string(data)
	if strings.EqualFold(filepath.Base(path), "config.ini") {
		if version := parseINIValue(text, "game_version"); version != "" {
			return version, nil
		}
		if version := parseAnyVersionLine(text); version != "" {
			return version, nil
		}
	}

	if version := versionPattern.FindString(text); version != "" {
		return version, nil
	}

	return "", fmt.Errorf("version not found in %s", path)
}

func parseINIValue(text, targetKey string) string {
	for _, rawLine := range strings.Split(text, "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "[") || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(key), targetKey) {
			continue
		}
		if version := versionPattern.FindString(strings.TrimSpace(value)); version != "" {
			return version
		}
	}
	return ""
}

func parseAnyVersionLine(text string) string {
	for _, rawLine := range strings.Split(text, "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "[") {
			continue
		}
		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		if !strings.Contains(strings.ToLower(strings.TrimSpace(key)), "version") {
			continue
		}
		if version := versionPattern.FindString(strings.TrimSpace(value)); version != "" {
			return version
		}
	}
	return ""
}

func hasFile(dir, name string) bool {
	info, err := os.Stat(filepath.Join(dir, name))
	return err == nil && !info.IsDir()
}
