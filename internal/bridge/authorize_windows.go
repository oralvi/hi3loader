//go:build windows

package bridge

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"

	xwindows "golang.org/x/sys/windows"
)

const th32csSnapProcess = 0x00000002

var (
	kernel32                     = syscall.NewLazyDLL("kernel32.dll")
	procCreateToolhelp32Snapshot = kernel32.NewProc("CreateToolhelp32Snapshot")
	procProcess32FirstW          = kernel32.NewProc("Process32FirstW")
	procProcess32NextW           = kernel32.NewProc("Process32NextW")
)

type processEntry32 struct {
	Size            uint32
	Usage           uint32
	ProcessID       uint32
	DefaultHeapID   uintptr
	ModuleID        uint32
	Threads         uint32
	ParentProcessID uint32
	PriClassBase    int32
	Flags           uint32
	ExeFile         [syscall.MAX_PATH]uint16
}

func authorizeAuxRuntime() error {
	if err := authorizeHelperSecret(); err != nil {
		return err
	}

	currentExe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve helper executable: %w", err)
	}
	parentExe, err := parentExecutablePath()
	if err != nil {
		return fmt.Errorf("resolve parent process: %w", err)
	}
	if !sameExecutablePath(currentExe, parentExe) {
		return fmt.Errorf("helper invocation parent is not trusted")
	}
	return nil
}

func parentExecutablePath() (string, error) {
	parentPID, err := parentProcessID(uint32(os.Getpid()))
	if err != nil {
		return "", err
	}
	if parentPID == 0 {
		return "", fmt.Errorf("parent process not found")
	}

	handle, err := xwindows.OpenProcess(xwindows.PROCESS_QUERY_LIMITED_INFORMATION, false, parentPID)
	if err != nil {
		return "", fmt.Errorf("open parent process: %w", err)
	}
	defer xwindows.CloseHandle(handle)

	buf := make([]uint16, syscall.MAX_PATH)
	size := uint32(len(buf))
	if err := xwindows.QueryFullProcessImageName(handle, 0, &buf[0], &size); err != nil {
		return "", fmt.Errorf("query parent process image: %w", err)
	}
	return xwindows.UTF16ToString(buf[:size]), nil
}

func parentProcessID(pid uint32) (uint32, error) {
	snapshot, _, callErr := procCreateToolhelp32Snapshot.Call(uintptr(th32csSnapProcess), 0)
	if snapshot == uintptr(syscall.InvalidHandle) {
		if callErr != syscall.Errno(0) {
			return 0, callErr
		}
		return 0, fmt.Errorf("create process snapshot failed")
	}
	defer syscall.CloseHandle(syscall.Handle(snapshot))

	var entry processEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))
	ret, _, callErr := procProcess32FirstW.Call(snapshot, uintptr(unsafe.Pointer(&entry)))
	if ret == 0 {
		if callErr != syscall.Errno(0) {
			return 0, callErr
		}
		return 0, fmt.Errorf("enumerate process snapshot failed")
	}

	for {
		if entry.ProcessID == pid {
			return entry.ParentProcessID, nil
		}
		ret, _, _ = procProcess32NextW.Call(snapshot, uintptr(unsafe.Pointer(&entry)))
		if ret == 0 {
			break
		}
	}
	return 0, fmt.Errorf("parent process id not found")
}
