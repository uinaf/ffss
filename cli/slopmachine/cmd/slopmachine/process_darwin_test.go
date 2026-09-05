package main

import (
	"os/exec"
	"syscall"
	"testing"

	"golang.org/x/sys/unix"
)

func TestCleanupDoesNotIgnoreLiveLeader(t *testing.T) {
	command := exec.Command("sleep", "30")
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = command.Process.Kill()
		_ = command.Wait()
	}()
	pid := command.Process.Pid
	if safe, pending := cleanupStateAfterExit(pid); safe || !pending {
		t.Fatalf("live leader cleanup state = (%t, %t)", safe, pending)
	}
	if ignoreCleanupErrorAfterExit(pid, unix.EPERM) {
		t.Fatal("permission error ignored for a live process group")
	}
}

func verificationProcessZombie(pid int) bool {
	process, err := unix.SysctlKinfoProc("kern.proc.pid", pid)
	return err == nil && isZombieLeader(process, pid)
}
