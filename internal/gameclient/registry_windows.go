//go:build windows

package gameclient

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/sys/windows/registry"
)

const (
	hypRegistryRoot = `Software\miHoYo\HYP`
	bh3GameBiz      = "bh3_cn"
	launcherExeName = "launcher.exe"
)

type hypInstallSnapshot struct {
	RootKeyPath     string
	InstallPath     string
	GameInstallPath string
	LauncherExePath string
	GameKeyPath     string
}

func DetectGameInstallPathFromRegistry() (string, string, error) {
	for _, snapshot := range readHypInstallSnapshots(registry.CURRENT_USER, hypRegistryRoot, bh3GameBiz) {
		resolved, err := ResolveDir(snapshot.GameInstallPath)
		if err == nil {
			return resolved, snapshot.GameKeyPath, nil
		}
	}
	return "", "", fmt.Errorf("game install path not found in registry")
}

func DetectLauncherExecutableFromRegistry() (string, string, error) {
	for _, snapshot := range readHypInstallSnapshots(registry.CURRENT_USER, hypRegistryRoot, bh3GameBiz) {
		resolved, err := ResolveLauncherExecutable(snapshot.LauncherExePath)
		if err == nil {
			return resolved, snapshot.RootKeyPath, nil
		}
	}
	return "", "", fmt.Errorf("launcher path not found in registry")
}

func readHypInstallSnapshots(root registry.Key, basePath, gameBiz string) []hypInstallSnapshot {
	candidates := enumerateHypInstallKeys(root, basePath)
	if len(candidates) == 0 {
		return nil
	}

	snapshots := make([]hypInstallSnapshot, 0, len(candidates))
	for _, candidate := range candidates {
		installPath := readRegistryString(root, candidate, "InstallPath")
		gameInstallPath := readRegistryString(root, candidate+`\`+gameBiz, "GameInstallPath")
		snapshots = append(snapshots, hypInstallSnapshot{
			RootKeyPath:     candidate,
			InstallPath:     installPath,
			GameInstallPath: gameInstallPath,
			LauncherExePath: filepath.Join(NormalizePath(installPath), launcherExeName),
			GameKeyPath:     candidate + `\` + gameBiz,
		})
	}
	return snapshots
}

func readRegistryString(root registry.Key, keyPath, valueName string) string {
	keyPath = strings.TrimSpace(keyPath)
	valueName = strings.TrimSpace(valueName)
	if keyPath == "" || valueName == "" {
		return ""
	}

	key, err := registry.OpenKey(root, keyPath, registry.READ)
	if err != nil {
		return ""
	}
	defer key.Close()

	value, _, err := key.GetStringValue(valueName)
	if err != nil {
		return ""
	}
	return NormalizePath(value)
}

func enumerateHypInstallKeys(root registry.Key, basePath string) []string {
	baseKey, err := registry.OpenKey(root, basePath, registry.READ)
	if err != nil {
		return nil
	}
	defer baseKey.Close()

	subKeys, err := baseKey.ReadSubKeyNames(-1)
	if err != nil {
		return nil
	}
	sort.Sort(sort.Reverse(sort.StringSlice(subKeys)))

	result := make([]string, 0, len(subKeys))
	for _, name := range subKeys {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		result = append(result, basePath+`\`+name)
	}
	return result
}
