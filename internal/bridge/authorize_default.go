//go:build !windows

package bridge

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func authorizeAuxRuntime() error {
	if err := authorizeHelperSecret(); err != nil {
		return err
	}

	currentExe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve helper executable: %w", err)
	}
	parentCommand, err := parentExecutableCommand()
	if err != nil {
		return fmt.Errorf("resolve parent process: %w", err)
	}
	if !sameExecutablePath(currentExe, parentCommand) && !commandMatchesExecutable(parentCommand, currentExe) {
		return fmt.Errorf("helper invocation parent is not trusted")
	}
	return nil
}

func parentExecutableCommand() (string, error) {
	ppid := os.Getppid()
	if ppid <= 1 {
		return "", fmt.Errorf("parent process not found")
	}

	output, err := exec.Command("ps", "-o", "command=", "-p", fmt.Sprintf("%d", ppid)).Output()
	if err != nil {
		return "", err
	}

	command := strings.TrimSpace(string(output))
	if command == "" {
		return "", fmt.Errorf("parent process command is empty")
	}
	return command, nil
}

func commandMatchesExecutable(commandLine, executable string) bool {
	commandLine = strings.TrimSpace(commandLine)
	executable = strings.TrimSpace(executable)
	if commandLine == "" || executable == "" {
		return false
	}

	if strings.EqualFold(commandLine, executable) || strings.HasPrefix(commandLine, executable+" ") {
		return true
	}

	trimmed := strings.Trim(commandLine, `"'`)
	return strings.EqualFold(trimmed, executable) || strings.HasPrefix(trimmed, executable+" ")
}
