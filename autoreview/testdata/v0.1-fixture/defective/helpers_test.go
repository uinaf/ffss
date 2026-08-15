package calculator

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHelpersNominalCases(t *testing.T) {
	if counts := CountByOwner(nil); len(counts) != 0 {
		t.Fatalf("CountByOwner(nil) = %v", counts)
	}
	batches := Batch([]string{"a", "b", "c", "d"}, 2)
	if len(batches) != 2 || len(batches[0]) != 2 || len(batches[1]) != 2 {
		t.Fatalf("Batch even split = %v", batches)
	}
	path := filepath.Join(t.TempDir(), "config.txt")
	if err := os.WriteFile(path, []byte("ok"), 0o600); err != nil {
		t.Fatal(err)
	}
	content, err := ReadConfig(path)
	if err != nil || string(content) != "ok" {
		t.Fatalf("ReadConfig() = %q, %v", content, err)
	}
}
