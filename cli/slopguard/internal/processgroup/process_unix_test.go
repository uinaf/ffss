//go:build darwin || linux

package processgroup

import "testing"

func TestTerminateReportsMissingProcessGroup(t *testing.T) {
	terminated, err := terminate(1 << 30)
	if err != nil {
		t.Fatal(err)
	}
	if terminated {
		t.Fatal("missing process group was reported as terminated")
	}
}
