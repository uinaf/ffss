package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"
)

func TestVerificationPipeHelper(t *testing.T) {
	path := os.Getenv("SLOPMACHINE_PIPE_HELPER")
	if path == "" {
		return
	}
	if _, err := syscall.Setsid(); err != nil {
		os.Exit(2)
	}
	fmt.Fprint(os.Stdout, "helper stdout")
	fmt.Fprint(os.Stderr, "helper stderr")
	if err := os.WriteFile(path, []byte(strconv.Itoa(os.Getpid())), 0600); err != nil {
		os.Exit(2)
	}
	time.Sleep(30 * time.Second)
	os.Exit(0)
}

func TestRunShellDetachedPipes(t *testing.T) {
	for _, cancelRun := range []bool{false, true} {
		t.Run(fmt.Sprintf("cancel=%t", cancelRun), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "pid")
			executable, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			command := fmt.Sprintf(`SLOPMACHINE_PIPE_HELPER=%q %q -test.run='^TestVerificationPipeHelper$' & while [ ! -s %q ]; do sleep 0.01; done`, path, executable, path)
			if cancelRun {
				command += "; wait"
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			type result struct {
				code   int
				digest string
				err    error
			}
			done := make(chan result, 1)
			go func() { code, digest, err := runShell(ctx, command, true); done <- result{code, digest, err} }()
			pid := waitVerificationPID(t, path)
			defer syscall.Kill(pid, syscall.SIGKILL)
			if cancelRun {
				cancel()
			}
			select {
			case got := <-done:
				if cancelRun {
					if got.code != 130 || !errors.Is(got.err, errVerificationCommandCancelled) {
						t.Fatalf("cancellation = %+v", got)
					}
				} else if got.code != 1 || got.err != nil {
					t.Fatalf("incomplete output = %+v", got)
				}
				stdout, stderr := newOutputDigester(), newOutputDigester()
				stdout.Write([]byte("helper stdout"))
				stderr.Write([]byte("helper stderr"))
				if want := digestOutputs(stdout, stderr); got.digest != want {
					t.Fatalf("digest = %s, want %s", got.digest, want)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("inherited pipes blocked verification")
			}
		})
	}
}

func waitVerificationPID(t *testing.T, path string) int {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if data, err := os.ReadFile(path); err == nil {
			if pid, err := strconv.Atoi(string(data)); err == nil && pid > 0 {
				return pid
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("helper did not start")
	return 0
}

func TestRunShellNormalExitCleansGroup(t *testing.T) {
	for _, redirect := range []string{"", ">/dev/null 2>&1"} {
		t.Run(redirect, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "pid")
			command := fmt.Sprintf(`sleep 30 %s & printf '%%s' "$!" > %q; exit 7`, redirect, path)
			code, _, err := runShell(context.Background(), command, true)
			pid := waitVerificationPID(t, path)
			defer syscall.Kill(pid, syscall.SIGKILL)
			if code != 7 || err != nil {
				t.Fatalf("exit = (%d,%v)", code, err)
			}
			waitVerificationProcessTerminated(t, pid)
		})
	}
}

func TestRunShellGracefulCancellation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pid")
	handled := filepath.Join(t.TempDir(), "handled")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	command := fmt.Sprintf(`trap 'printf done > %q; exit 0' TERM; printf '%%s' "$$" > %q; while :; do sleep 0.05; done`, handled, path)
	go func() { _, _, err := runShell(ctx, command, true); done <- err }()
	pid := waitVerificationPID(t, path)
	defer syscall.Kill(-pid, syscall.SIGKILL)
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, errVerificationCommandCancelled) {
			t.Fatal(err)
		}
	case <-time.After(4 * time.Second):
		t.Fatal("cancellation blocked")
	}
	if data, err := os.ReadFile(handled); err != nil || string(data) != "done" {
		t.Fatalf("TERM handler = %q,%v", data, err)
	}
}

// A killed orphan can remain a zombie until init reaps it. Check kernel
// termination state instead of making fixture success depend on init timing.
func verificationProcessTerminated(pid int) bool {
	if errors.Is(syscall.Kill(pid, 0), syscall.ESRCH) {
		return true
	}
	return verificationProcessZombie(pid)
}

func waitVerificationProcessTerminated(t *testing.T, pid int) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if verificationProcessTerminated(pid) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("owned process %d remains live", pid)
}

func TestVerificationProcessTerminationStates(t *testing.T) {
	command := exec.Command("sleep", "30")
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = command.Process.Kill(); _ = command.Wait() }()
	pid := command.Process.Pid
	if verificationProcessTerminated(pid) {
		t.Fatal("live child accepted as terminated")
	}
	if err := command.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	if err := waitForExit(ctx, pid); err != nil {
		t.Fatal(err)
	}
	// WNOWAIT/kqueue preserves this exact child's zombie for the assertion.
	// Darwin may publish NOTE_EXIT before its zombie state becomes visible.
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) && !verificationProcessZombie(pid) {
		time.Sleep(time.Millisecond)
	}
	if !verificationProcessZombie(pid) || !verificationProcessTerminated(pid) {
		t.Fatal("unreaped zombie rejected as terminated")
	}
	_ = command.Wait()
	if !verificationProcessTerminated(pid) {
		t.Fatal("reaped child rejected as terminated")
	}
}
