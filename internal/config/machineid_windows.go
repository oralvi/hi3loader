//go:build windows

package config

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/sys/windows/registry"
)

func loadMachineID() (string, error) {
	if override := strings.TrimSpace(os.Getenv(machineIDEnvVar)); override != "" {
		return override, nil
	}
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Cryptography`, registry.QUERY_VALUE|registry.WOW64_64KEY)
	if err != nil {
		return "", fmt.Errorf("open machine guid key: %w", err)
	}
	defer key.Close()

	value, _, err := key.GetStringValue("MachineGuid")
	if err != nil {
		return "", fmt.Errorf("read machine guid: %w", err)
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("machine guid is empty")
	}
	return value, nil
}
