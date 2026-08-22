//go:build darwin || linux

package processgroup

import (
	"os/exec"
	"testing"
)

func TestTerminateReportsMissingProcessGroup(t *testing.T) {
	terminated, err := terminate(1 << 30)
	if err != nil {
		t.Fatal(err)
	}
	if terminated {
		t.Fatal("missing process group was reported as terminated")
	}
}

func TestLeaderKilledBySIGKILL(t *testing.T) {
	killed := exec.Command("/bin/sh", "-c", "kill -KILL $$")
	if err := killed.Run(); !leaderKilledBySIGKILL(err) {
		t.Fatalf("SIGKILL exit was not recognized: %v", err)
	}
	exited := exec.Command("/bin/sh", "-c", "exit 7")
	if err := exited.Run(); leaderKilledBySIGKILL(err) {
		t.Fatalf("ordinary exit was recognized as SIGKILL: %v", err)
	}
	if leaderKilledBySIGKILL(nil) {
		t.Fatal("nil error was recognized as SIGKILL")
	}
}
