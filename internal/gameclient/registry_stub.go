//go:build !windows

package gameclient

import "fmt"

func DetectGameInstallPathFromRegistry() (string, string, error) {
	return "", "", fmt.Errorf("registry lookup is only available on windows")
}

func DetectLauncherExecutableFromRegistry() (string, string, error) {
	return "", "", fmt.Errorf("registry lookup is only available on windows")
}
