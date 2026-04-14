//go:build !windows

package service

import "syscall"

func moduleSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{}
}
