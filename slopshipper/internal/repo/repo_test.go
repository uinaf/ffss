package repo

import "testing"

func TestRedactRemoteStripsUserinfo(t *testing.T) {
	got := redactRemote("https://TOKEN@github.com/uinaf/slopomatic.git")
	want := "https://github.com/uinaf/slopomatic.git"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	if got := redactRemote("git@github.com:uinaf/slopomatic.git"); got != "git@github.com:uinaf/slopomatic.git" {
		t.Fatalf("scp form changed: %q", got)
	}
}
