//go:build darwin

package processgroup

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestRunWaitsForSuccessfulLeader(t *testing.T) {
	result := Run(t.Context(), exec.Command("/usr/bin/true"))
	if err := result.Err(); err != nil || result.ContextCaused {
		t.Fatalf("Run() result = %+v, error = %v", result, err)
	}
}

func TestRunCancellationKillsProcessGroup(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "ready")
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	command := exec.Command("/bin/sh", "-c", `printf ready > "$1"; exec /bin/sleep 10`, "slopguard-processgroup-test", marker)
	done := make(chan Result, 1)
	go func() { done <- Run(ctx, command) }()
	deadline := time.Now().Add(10 * time.Second)
	for {
		if _, err := os.Stat(marker); err == nil {
			break
		} else if !errors.Is(err, os.ErrNotExist) {
			t.Fatal(err)
		}
		if time.Now().After(deadline) {
			t.Fatal("process group did not become ready")
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	select {
	case result := <-done:
		if !errors.Is(result.CommandErr, context.Canceled) || result.CleanupErr != nil || !result.ContextCaused {
			t.Fatalf("Run() result = %+v", result)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Run() did not return after cancellation")
	}
}

func TestContainsOnlyZombieLeader(t *testing.T) {
	t.Parallel()
	leader := unix.KinfoProc{}
	leader.Proc.P_pid = 42
	leader.Proc.P_stat = 5
	liveChild := unix.KinfoProc{}
	liveChild.Proc.P_pid = 43
	liveChild.Proc.P_stat = 2

	tests := []struct {
		name      string
		processes []unix.KinfoProc
		want      bool
	}{
		{name: "zombie leader only", processes: []unix.KinfoProc{leader}, want: true},
		{name: "live leader", processes: []unix.KinfoProc{{Proc: unix.ExternProc{P_pid: 42, P_stat: 2}}}},
		{name: "different zombie", processes: []unix.KinfoProc{{Proc: unix.ExternProc{P_pid: 41, P_stat: 5}}}},
		{name: "live descendant", processes: []unix.KinfoProc{leader, liveChild}},
		{name: "empty group"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := containsOnlyZombieLeader(test.processes, 42); got != test.want {
				t.Fatalf("containsOnlyZombieLeader() = %t, want %t", got, test.want)
			}
		})
	}
}

func TestIsZombieLeader(t *testing.T) {
	t.Parallel()

	zombie := unix.KinfoProc{}
	zombie.Proc.P_pid = 42
	zombie.Proc.P_stat = 5
	live := zombie
	live.Proc.P_stat = 2
	if !isZombieLeader(&zombie, 42) {
		t.Fatal("isZombieLeader() rejected the expected zombie")
	}
	if isZombieLeader(&live, 42) || isZombieLeader(&zombie, 41) || isZombieLeader(nil, 42) {
		t.Fatal("isZombieLeader() accepted a non-matching process")
	}
}
