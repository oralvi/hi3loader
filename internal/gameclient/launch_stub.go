//go:build !windows

package gameclient

import "errors"

func launchExecutable(string) error {
	return errors.New("not supported on this platform")
}

func LaunchLauncher(string) (string, error) {
	return "", errors.New("not supported on this platform")
}

func ResolveLauncherExecutable(string) (string, error) {
	return "", errors.New("not supported on this platform")
}
