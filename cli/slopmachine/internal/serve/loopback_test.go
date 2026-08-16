package serve

import (
	"net"
	"path/filepath"
	"strings"
	"testing"

	"github.com/uinaf/ffsstack/cli/slopmachine/internal/store"
)

type stubListener struct {
	addr net.Addr
}

func (s stubListener) Accept() (net.Conn, error) { return nil, net.ErrClosed }
func (s stubListener) Close() error              { return nil }
func (s stubListener) Addr() net.Addr            { return s.addr }

func TestAssertBoundLoopback(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	if err := assertBoundLoopback(ln); err != nil {
		t.Fatal(err)
	}

	err = assertBoundLoopback(stubListener{addr: &net.TCPAddr{IP: net.ParseIP("8.8.8.8"), Port: 7780}})
	if err == nil || !strings.Contains(err.Error(), "loopback") {
		t.Fatalf("want loopback error, got %v", err)
	}
}

func TestNewAcceptsLocalhost(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "t.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if _, err := New(Options{Store: st, RepoKey: "repo", Addr: "localhost:7780"}); err != nil {
		t.Fatal(err)
	}
}
