package repo

import "testing"

func TestRedactRemoteStripsUserinfo(t *testing.T) {
	got := redactRemote("https://TOKEN@github.com/uinaf/slopinator.git")
	want := "https://github.com/uinaf/slopinator.git"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	if got := redactRemote("git@github.com:uinaf/slopinator.git"); got != "git@github.com:uinaf/slopinator.git" {
		t.Fatalf("scp form changed: %q", got)
	}
}
