//go:build windows

package service

import "syscall"

const moduleCreateNoWindow = 0x08000000

func moduleSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: moduleCreateNoWindow,
	}
}
