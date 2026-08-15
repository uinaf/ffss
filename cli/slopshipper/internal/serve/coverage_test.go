package serve

import (
	"context"
	"html/template"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/uinaf/slopshipper/internal/store"
)

func TestNewValidationAndDefault(t *testing.T) {
	if _, err := New(Options{}); err == nil || !strings.Contains(err.Error(), "store") {
		t.Fatalf("missing store: %v", err)
	}
	st, err := store.Open(filepath.Join(t.TempDir(), "t.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if _, err := New(Options{Store: st}); err == nil || !strings.Contains(err.Error(), "repo key") {
		t.Fatalf("missing repo: %v", err)
	}
	srv, err := New(Options{Store: st, RepoKey: "repo"})
	if err != nil || srv.opts.Addr != "127.0.0.1:7780" {
		t.Fatalf("default addr=%q err=%v", srv.opts.Addr, err)
	}
}

func TestListenAndServeCancellationAndBindFailure(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "t.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	srv, err := New(Options{Store: st, RepoKey: "repo", Addr: "127.0.0.1:0"})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := srv.ListenAndServe(ctx); err != nil {
		t.Fatalf("cancel: %v", err)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	srv, err = New(Options{Store: st, RepoKey: "repo", Addr: ln.Addr().String()})
	if err != nil {
		t.Fatal(err)
	}
	if err := srv.ListenAndServe(context.Background()); err == nil {
		t.Fatal("expected bind failure")
	}
}

func TestHandlerHostsStaticAndRenderFailure(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "t.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	srv, err := New(Options{Store: st, RepoKey: "repo", Addr: "127.0.0.1:0"})
	if err != nil {
		t.Fatal(err)
	}
	for _, host := range []string{"localhost:7780", "[::1]:7780", "127.0.0.1"} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/static/app.css", nil)
		req.Host = host
		srv.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("host %q: %d", host, rec.Code)
		}
	}
	for _, host := range []string{"", "example.com"} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Host = host
		srv.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("host %q: %d", host, rec.Code)
		}
	}
	srv.tmpl = template.New("broken")
	rec := httptest.NewRecorder()
	srv.render(rec, "missing", nil)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("render: %d", rec.Code)
	}
}

func TestPresentationAndLoopbackHelpers(t *testing.T) {
	classes := map[string]string{
		"BLOCKED": "error", "NEEDS_DECISION": "warn", "REWORK": "warn", "RUN_DONE": "ok",
	}
	for state, want := range classes {
		if got := statusDotClass(state, true); !strings.Contains(got, want) {
			t.Errorf("statusDotClass(%s)=%q", state, got)
		}
	}
	if got := statusDotClass("INTAKE", true); !strings.Contains(got, "live") {
		t.Fatalf("open dot: %q", got)
	}
	if got := statusDotClass("INTAKE", false); !strings.Contains(got, "warn") {
		t.Fatalf("closed dot: %q", got)
	}
	for value, want := range map[string]string{
		"": "—", "2026-08-08T12:34:56Z": "12:34:56", "2026-08-08T12:34:56.123Z": "12:34:56",
		"a very long invalid timestamp": "a very long invalid", "short": "short",
	} {
		if got := shortTime(value); got != want {
			t.Errorf("shortTime(%q)=%q want %q", value, got, want)
		}
	}
	for _, addr := range []string{"localhost:1", "127.0.0.1:1", "[::1]:1"} {
		if err := assertLoopback(addr); err != nil {
			t.Errorf("assertLoopback(%q): %v", addr, err)
		}
	}
	for _, addr := range []string{"bad", "example.com:1"} {
		if err := assertLoopback(addr); err == nil {
			t.Errorf("assertLoopback(%q) succeeded", addr)
		}
	}
	if loopbackHost("") || loopbackHost("example.com:1") {
		t.Fatal("non-loopback host accepted")
	}
	if !loopbackHost("LOCALHOST") || !loopbackHost("::1") {
		t.Fatal("loopback host rejected")
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	if err := assertBoundLoopback(ln); err != nil {
		t.Fatal(err)
	}
	unix, err := net.Listen("unix", filepath.Join(t.TempDir(), "sock"))
	if err != nil {
		t.Fatal(err)
	}
	defer unix.Close()
	if err := assertBoundLoopback(unix); err == nil {
		t.Fatal("unix listener accepted")
	}
}
