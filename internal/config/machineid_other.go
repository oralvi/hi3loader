//go:build !windows && !linux && !darwin

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
	return "", fmt.Errorf("machine id is unavailable on this platform")
}
