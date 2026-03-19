//go:build darwin

package config

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

var ioPlatformUUIDPattern = regexp.MustCompile(`"IOPlatformUUID"\s*=\s*"([^"]+)"`)

func loadMachineID() (string, error) {
	if override := strings.TrimSpace(os.Getenv(machineIDEnvVar)); override != "" {
		return override, nil
	}
	output, err := exec.Command("ioreg", "-rd1", "-c", "IOPlatformExpertDevice").Output()
	if err != nil {
		return "", fmt.Errorf("read ioplatform uuid: %w", err)
	}
	match := ioPlatformUUIDPattern.FindStringSubmatch(string(output))
	if len(match) < 2 || strings.TrimSpace(match[1]) == "" {
		return "", fmt.Errorf("ioplatform uuid is unavailable")
	}
	return strings.TrimSpace(match[1]), nil
}
