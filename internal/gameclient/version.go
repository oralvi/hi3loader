package gameclient

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var versionPattern = regexp.MustCompile(`\b\d+\.\d+\.\d+\b`)

func ReadVersion(path string) (string, error) {
	dir, err := ResolveDir(path)
	if err != nil {
		return "", err
	}

	version, err := readVersionFromFile(filepath.Join(dir, "config.ini"))
	if err != nil {
		return "", fmt.Errorf("read game version from config.ini: %w", err)
	}
	return version, nil
}

func CompareVersion(localVersion, remoteVersion string) (int, error) {
	localParts, err := parseVersionParts(localVersion)
	if err != nil {
		return 0, fmt.Errorf("parse local version: %w", err)
	}
	remoteParts, err := parseVersionParts(remoteVersion)
	if err != nil {
		return 0, fmt.Errorf("parse remote version: %w", err)
	}

	limit := len(localParts)
	if len(remoteParts) > limit {
		limit = len(remoteParts)
	}
	for idx := 0; idx < limit; idx++ {
		localPart := versionPartAt(localParts, idx)
		remotePart := versionPartAt(remoteParts, idx)
		switch {
		case localPart < remotePart:
			return -1, nil
		case localPart > remotePart:
			return 1, nil
		}
	}
	return 0, nil
}

func IsOutdated(localVersion, remoteVersion string) (bool, error) {
	comparison, err := CompareVersion(localVersion, remoteVersion)
	if err != nil {
		return false, err
	}
	return comparison < 0, nil
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

func parseVersionParts(value string) ([]int, error) {
	match := versionPattern.FindString(strings.TrimSpace(value))
	if match == "" {
		return nil, fmt.Errorf("missing dotted version")
	}
	parts := strings.Split(match, ".")
	parsed := make([]int, 0, len(parts))
	for _, part := range parts {
		n, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil {
			return nil, err
		}
		parsed = append(parsed, n)
	}
	return parsed, nil
}

func versionPartAt(parts []int, idx int) int {
	if idx < 0 || idx >= len(parts) {
		return 0
	}
	return parts[idx]
}
