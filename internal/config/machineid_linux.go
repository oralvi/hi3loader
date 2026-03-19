//go:build linux

package config

import (
	"fmt"
	"os"
	"strings"
)

func loadMachineID() (string, error) {
	if override := strings.TrimSpace(os.Getenv(machineIDEnvVar)); override != "" {
		return override, nil
	}
	for _, candidate := range []string{"/etc/machine-id", "/var/lib/dbus/machine-id"} {
		data, err := os.ReadFile(candidate)
		if err != nil {
			continue
		}
		if value := strings.TrimSpace(string(data)); value != "" {
			return value, nil
		}
	}
	return "", fmt.Errorf("machine id is unavailable")
}
