package main

import (
	"os"
	"slices"
	"strings"
	"testing"
)

func TestVerifyGraphKeepsCombinedRaceAndCoverageGate(t *testing.T) {
	mise, err := os.ReadFile("../../mise.toml")
	if err != nil {
		t.Fatal(err)
	}
	verify := string(mise)
	if start := strings.Index(verify, "[tasks.verify]"); start >= 0 {
		verify = verify[start:]
		if end := strings.Index(verify[len("[tasks.verify]"):], "\n["); end >= 0 {
			verify = verify[:len("[tasks.verify]")+end]
		}
	} else {
		t.Fatal("mise.toml has no verify task")
	}
	dependsLine := ""
	for _, line := range strings.Split(verify, "\n") {
		if strings.HasPrefix(line, "depends = [") {
			dependsLine = line
			break
		}
	}
	if dependsLine == "" {
		t.Fatal("verify task has no dependencies")
	}
	raw := strings.TrimSuffix(strings.TrimPrefix(dependsLine, "depends = ["), "]")
	parts := strings.Split(raw, ",")
	dependencies := make([]string, 0, len(parts))
	for _, part := range parts {
		dependencies = append(dependencies, strings.Trim(strings.TrimSpace(part), `"`))
	}
	want := []string{"fmt:check", "test:coverage", "vet", "build", "conformance", "installer:check", "release:check"}
	if !slices.Equal(dependencies, want) {
		t.Fatalf("verify dependencies = %v, want %v", dependencies, want)
	}

	script, err := os.ReadFile("../../scripts/test-coverage.sh")
	if err != nil {
		t.Fatal(err)
	}
	proof := string(script)
	normalized := strings.ReplaceAll(proof, "\\\n", " ")
	commands := []string{}
	for _, line := range strings.Split(normalized, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") {
			continue
		}
		if start := strings.Index(line, "go test "); start >= 0 {
			commands = append(commands, strings.Join(strings.Fields(line[start:]), " "))
		}
	}
	wantCommand := `go test -count=1 -race -covermode=atomic -coverprofile="$coverage_profile" -p 4 -parallel 4 ./...`
	if len(commands) != 1 || commands[0] != wantCommand {
		t.Fatalf("test proof commands = %q, want exactly %q", commands, wantCommand)
	}
	if !strings.Contains(normalized, `SLOPMACHINE_COVERAGE_DIR="$raw_dir"`) {
		t.Fatal("combined test proof does not route child-process coverage")
	}
}
