package serve_test

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/uinaf/ffsstack/cli/slopmachine/internal/machine"
	"github.com/uinaf/ffsstack/cli/slopmachine/internal/serve"
	"github.com/uinaf/ffsstack/cli/slopmachine/internal/store"
)

func TestRejectsNonLoopback(t *testing.T) {
	s, err := store.Open(filepath.Join(t.TempDir(), "t.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	_, err = serve.New(serve.Options{Store: s, RepoKey: "repo", Addr: "0.0.0.0:7780"})
	if err == nil || !strings.Contains(err.Error(), "loopback") {
		t.Fatalf("want loopback error, got %v", err)
	}
}

func TestIndexAndRunProjection(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "t.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	run := machine.NewRun("demo", "repo-a")
	units := []machine.Unit{{ID: "u1", Title: "one"}}
	if err := st.CreateRun(run, units, nil); err != nil {
		t.Fatal(err)
	}
	res, err := machine.Apply(run, units, machine.CmdIntake, machine.ApplyInput{
		ExpectedRevision: run.Revision,
		Intake:           &machine.IntakePatch{SeriesBound: intPtr(1)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SaveApply(res); err != nil {
		t.Fatal(err)
	}

	srv, err := serve.New(serve.Options{Store: st, RepoKey: "repo-a", Addr: "127.0.0.1:0"})
	if err != nil {
		t.Fatal(err)
	}
	handler := srv.Handler()

	index := httptest.NewRecorder()
	handler.ServeHTTP(index, loopbackGet("/"))
	if index.Code != 200 || !strings.Contains(index.Body.String(), "demo") || !strings.Contains(index.Body.String(), "u-table") {
		t.Fatalf("index: code=%d body=%s", index.Code, index.Body.String())
	}

	detail := httptest.NewRecorder()
	handler.ServeHTTP(detail, loopbackGet("/runs/demo"))
	if detail.Code != 200 || !strings.Contains(detail.Body.String(), "evidence timeline") || !strings.Contains(detail.Body.String(), "intake") || !strings.Contains(detail.Body.String(), "next command") {
		t.Fatalf("run: code=%d body=%s", detail.Code, detail.Body.String())
	}

	missing := httptest.NewRecorder()
	handler.ServeHTTP(missing, loopbackGet("/runs/nope"))
	if missing.Code != 404 || !strings.Contains(missing.Body.String(), "run not found") {
		t.Fatalf("missing: %d %s", missing.Code, missing.Body.String())
	}

	crossSrv, err := serve.New(serve.Options{Store: st, RepoKey: "repo-b", Addr: "127.0.0.1:0"})
	if err != nil {
		t.Fatal(err)
	}
	cross := httptest.NewRecorder()
	crossSrv.Handler().ServeHTTP(cross, loopbackGet("/runs/demo"))
	if cross.Code != 404 || strings.Contains(cross.Body.String(), "different repo") {
		t.Fatalf("cross-repo body leaked: %d %s", cross.Code, cross.Body.String())
	}

	rebinding := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Host = "evil.example:7780"
	handler.ServeHTTP(rebinding, req)
	if rebinding.Code != http.StatusForbidden {
		t.Fatalf("rebinding host: %d", rebinding.Code)
	}

	for _, path := range []string{"/", "/runs/demo"} {
		post := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, path, nil)
		req.Host = "127.0.0.1:7780"
		handler.ServeHTTP(post, req)
		if post.Code != http.StatusMethodNotAllowed {
			t.Fatalf("POST %s: want 405 got %d", path, post.Code)
		}
	}
}

func loopbackGet(path string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Host = "127.0.0.1:7780"
	return req
}

func intPtr(v int) *int { return &v }
