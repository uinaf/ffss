package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestSelfupdateRefusesDevBuild(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	exit := run(t.Context(), []string{"selfupdate", "--check"}, &stdout, &stderr, dependencies{})
	if exit != 2 || !strings.Contains(stderr.String(), "not a release build") {
		t.Fatalf("dev build must be refused: exit=%d stderr=%q", exit, stderr.String())
	}
}

func TestSelfupdateRejectsPositionalArguments(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	exit := run(t.Context(), []string{"selfupdate", "extra"}, &stdout, &stderr, dependencies{})
	if exit != 2 || !strings.Contains(stderr.String(), "positional") {
		t.Fatalf("positional arguments must be rejected: exit=%d stderr=%q", exit, stderr.String())
	}
}
