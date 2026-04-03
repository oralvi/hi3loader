//go:build windows

package gameclient

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	shellShowNormal   = 1
	shellAccessDenied = 5
	launcherArg       = "--game=bh3_cn"
	shellVerbRunAs    = "runas"
	shellVerbOpen     = "open"
)

var (
	shell32Proc       = windows.NewLazySystemDLL("shell32.dll")
	procShellExecuteW = shell32Proc.NewProc("ShellExecuteW")
)

func launchExecutable(exePath string) error {
	if err := shellExecute(shellVerbRunAs, exePath, "", filepath.Dir(exePath)); err != nil {
		return fmt.Errorf("launch game: %w", err)
	}
	return nil
}

func LaunchLauncher(path string) (string, error) {
	launcherPath, err := ResolveLauncherExecutable(path)
	if err != nil {
		return "", err
	}
	if err := shellExecute(shellVerbOpen, launcherPath, launcherArg, filepath.Dir(launcherPath)); err != nil {
		return launcherPath, fmt.Errorf("launch launcher: %w", err)
	}
	return launcherPath, nil
}

func ResolveLauncherExecutable(path string) (string, error) {
	path = NormalizePath(path)
	if path == "" {
		return "", fmt.Errorf("launcher path is empty")
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("stat launcher path: %w", err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("launcher path must point to an executable file")
	}
	exePath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve launcher path: %w", err)
	}
	return exePath, nil
}

func shellExecute(verbName, exePath, args, workDir string) error {
	verb, err := windows.UTF16PtrFromString(strings.TrimSpace(verbName))
	if err != nil {
		return err
	}
	file, err := windows.UTF16PtrFromString(exePath)
	if err != nil {
		return err
	}
	params, err := windows.UTF16PtrFromString(strings.TrimSpace(args))
	if err != nil {
		return err
	}
	dir, err := windows.UTF16PtrFromString(workDir)
	if err != nil {
		return err
	}

	ret, _, callErr := procShellExecuteW.Call(
		0,
		uintptr(unsafe.Pointer(verb)),
		uintptr(unsafe.Pointer(file)),
		uintptr(unsafe.Pointer(params)),
		uintptr(unsafe.Pointer(dir)),
		uintptr(shellShowNormal),
	)
	if ret > 32 {
		return nil
	}
	if ret == shellAccessDenied {
		return fmt.Errorf("elevation was cancelled or denied")
	}
	if callErr != nil && callErr != syscall.Errno(0) {
		return callErr
	}
	return fmt.Errorf("ShellExecuteW failed with code %d", ret)
}
