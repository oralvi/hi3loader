//go:build !windows

package bridge

import "os/exec"

func configureCommand(_ *exec.Cmd) {}
