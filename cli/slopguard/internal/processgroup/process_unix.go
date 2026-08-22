//go:build darwin || linux

package processgroup

import (
	"errors"
	"os/exec"
	"syscall"
)

func configure(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func terminate(leaderPID int) (bool, error) {
	err := syscall.Kill(-leaderPID, syscall.SIGKILL)
	if errors.Is(err, syscall.ESRCH) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}
