package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/uinaf/ffss/cli/slopguard/internal/protocol"
)

func TestBinaryEnvironmentTemplateBoundary(t *testing.T) {
	binary := buildSlopguardBinary(t)
	for _, name := range []string{".env.example", ".env"} {
		t.Run(name, func(t *testing.T) {
			repository := cliRepository(t)
			before := "# Configure the service\nDEVICE_TARGET=192.168.0.10\nCLIENT_CONFIG=.client/<profile-name>.json\n"
			for _, content := range []string{before, strings.ReplaceAll(before, "192.168.0.10", "192.168.0.20")} {
				if err := os.WriteFile(filepath.Join(repository, name), []byte(content), 0o600); err != nil {
					t.Fatal(err)
				}
				gitCommand(t, repository, "add", name)
				gitCommand(t, repository, "-c", "user.name=Slopguard Test", "-c", "user.email=test@example.invalid", "commit", "-q", "-m", "configure service")
			}
			toolsDirectory, calls, _, _, _ := writeFakeReviewTools(t, "clean")
			command := exec.Command(binary, "review", "--repository", repository,
				"--mode", "branch", "--base", "HEAD~1", "--engine", "codex",
				"--retries", "0", "--timeout", "8s", "--web-access=false", "--output", "json",
				"--prompt", "Review the service configuration change.")
			command.Env = replaceEnvironment(os.Environ(), map[string]string{
				"PATH": toolsDirectory + ":/usr/bin:/bin", "XDG_CONFIG_HOME": t.TempDir(),
				"OPENAI_API_KEY": "fake-provider-credential",
			})
			var stdout, stderr bytes.Buffer
			command.Stdout, command.Stderr = &stdout, &stderr
			err := command.Run()
			var report protocol.Report
			if decodeErr := json.Unmarshal(stdout.Bytes(), &report); decodeErr != nil {
				t.Fatalf("decode report: %v; stderr=%s", decodeErr, stderr.String())
			}
			if validateErr := report.Validate(); validateErr != nil {
				t.Fatal(validateErr)
			}
			if name == ".env.example" {
				if err != nil || report.Status != protocol.StatusClean {
					t.Fatalf("template review: err=%v report=%s stderr=%s", err, stdout.String(), stderr.String())
				}
				if count, readErr := os.ReadFile(calls); readErr != nil || strings.TrimSpace(string(count)) != "1" {
					t.Fatalf("provider calls=%q err=%v", count, readErr)
				}
			} else {
				if exitCode(err) != 2 || report.Failure == nil || report.Failure.Class != protocol.FailureTarget {
					t.Fatalf("sensitive path review: err=%v report=%s", err, stdout.String())
				}
				if _, statErr := os.Stat(calls); !os.IsNotExist(statErr) {
					t.Fatalf("sensitive path reached provider: %v", statErr)
				}
			}
		})
	}
}
