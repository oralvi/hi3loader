//go:build windows

package gameclient

import (
	"fmt"
	"path/filepath"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	shellShowNormal   = 1
	shellAccessDenied = 5
)

var (
	shell32Proc       = windows.NewLazySystemDLL("shell32.dll")
	procShellExecuteW = shell32Proc.NewProc("ShellExecuteW")
)

func launchExecutable(exePath string) error {
	if err := shellExecuteRunAs(exePath, filepath.Dir(exePath)); err != nil {
		return fmt.Errorf("launch game: %w", err)
	}
	return nil
}

func shellExecuteRunAs(exePath, workDir string) error {
	verb, err := windows.UTF16PtrFromString("runas")
	if err != nil {
		return err
	}
	file, err := windows.UTF16PtrFromString(exePath)
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
		0,
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
