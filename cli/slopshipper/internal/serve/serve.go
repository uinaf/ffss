package serve

import (
	"bytes"
	"context"
	"embed"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/uinaf/slopshipper/internal/machine"
	"github.com/uinaf/slopshipper/internal/status"
	"github.com/uinaf/slopshipper/internal/store"
)

//go:embed templates/*.html static/*
var assets embed.FS

// Options configures the loopback control plane.
type Options struct {
	Store   *store.Store
	RepoKey string
	Addr    string // default 127.0.0.1:7780
}

// Server is a read-only HTTP projector over the sqlite store.
type Server struct {
	opts   Options
	tmpl   *template.Template
	static http.Handler
}

func New(opts Options) (*Server, error) {
	if opts.Store == nil {
		return nil, fmt.Errorf("store is required")
	}
	if opts.RepoKey == "" {
		return nil, fmt.Errorf("repo key is required")
	}
	if opts.Addr == "" {
		opts.Addr = "127.0.0.1:7780"
	}
	if err := assertLoopback(opts.Addr); err != nil {
		return nil, err
	}

	funcs := template.FuncMap{
		"join":      strings.Join,
		"shortTime": shortTime,
		"statusDot": statusDotClass,
	}
	tmpl, err := template.New("").Funcs(funcs).ParseFS(assets, "templates/*.html")
	if err != nil {
		return nil, err
	}
	staticFS, err := fs.Sub(assets, "static")
	if err != nil {
		return nil, err
	}
	return &Server{
		opts:   opts,
		tmpl:   tmpl,
		static: http.FileServer(http.FS(staticFS)),
	}, nil
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /static/", http.StripPrefix("/static/", s.static))
	mux.HandleFunc("GET /{$}", s.handleIndex)
	mux.HandleFunc("GET /runs/{id}", s.handleRun)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !loopbackHost(r.Host) {
			http.Error(w, "forbidden host", http.StatusForbidden)
			return
		}
		mux.ServeHTTP(w, r)
	})
}

// ListenAndServe binds loopback and serves until ctx is cancelled.
func (s *Server) ListenAndServe(ctx context.Context) error {
	ln, err := net.Listen("tcp", s.opts.Addr)
	if err != nil {
		return err
	}
	defer ln.Close()
	if err := assertBoundLoopback(ln); err != nil {
		return err
	}

	httpServer := &http.Server{
		Handler:           s.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	errCh := make(chan error, 1)
	go func() {
		errCh <- httpServer.Serve(ln)
	}()

	fmt.Printf("slopshipper serve listening on http://%s (repo projector)\n", ln.Addr().String())

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdownCtx)
		return nil
	case err := <-errCh:
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	}
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	runs, err := s.opts.Store.ListRuns(s.opts.RepoKey)
	if err != nil {
		fmt.Fprintf(os.Stderr, "slopshipper serve: list runs: %v\n", err)
		http.Error(w, "store error", http.StatusInternalServerError)
		return
	}
	s.render(w, "index.html", map[string]any{
		"Title":   "Runs",
		"RepoKey": s.opts.RepoKey,
		"Runs":    runs,
	})
}

func (s *Server) handleRun(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	run, units, events, err := s.opts.Store.GetRunProjection(s.opts.RepoKey, id)
	if err != nil {
		if errors.Is(err, machine.ErrNotFound) {
			http.Error(w, "run not found", http.StatusNotFound)
			return
		}
		fmt.Fprintf(os.Stderr, "slopshipper serve: run %s: %v\n", id, err)
		http.Error(w, "store error", http.StatusInternalServerError)
		return
	}
	totals, err := s.opts.Store.TelemetryTotals(run.ID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "slopshipper serve: totals %s: %v\n", id, err)
		http.Error(w, "store error", http.StatusInternalServerError)
		return
	}
	doc := status.From(run, units)
	s.render(w, "run.html", map[string]any{
		"Title":            run.ID,
		"Run":              run,
		"Units":            units,
		"Events":           events,
		"Released":         run.Released(),
		"NextAction":       doc.NextAction,
		"RiskTier":         doc.RiskTier,
		"ReviewRequired":   doc.RequiredReviewers,
		"ReviewCompleted":  doc.CompletedReviewers,
		"DecisionQuestion": doc.DecisionQuestion,
		"StatusLine":       doc.CompactLine(),
		"Blocker":          run.BlockerReason,
		"CurrentUnitID":    run.CurrentUnitID,
		"Telemetry":        formatTotals(totals),
	})
}

// formatDuration keeps the stored total exact: sub-second totals render in
// milliseconds, larger ones as seconds with trailing zeros trimmed.
func formatDuration(ms int64) string {
	if ms < 1000 {
		return fmt.Sprintf("%dms", ms)
	}
	// Integer arithmetic keeps totals exact beyond float64's 2^53 range.
	seconds := strings.TrimRight(strings.TrimRight(fmt.Sprintf("%d.%03d", ms/1000, ms%1000), "0"), ".")
	return seconds + "s"
}

// formatTotals renders aggregated run telemetry for the projector; empty
// when no event recorded any.
func formatTotals(t store.Totals) string {
	if t.RecordedEvents == 0 {
		return ""
	}
	parts := []string{fmt.Sprintf("%d event(s)", t.RecordedEvents)}
	if t.DurationMS > 0 {
		parts = append(parts, formatDuration(t.DurationMS))
	}
	if t.Tokens > 0 {
		parts = append(parts, fmt.Sprintf("%d tokens", t.Tokens))
	}
	if t.CostCents > 0 {
		parts = append(parts, fmt.Sprintf("$%d.%02d", t.CostCents/100, t.CostCents%100))
	}
	return strings.Join(parts, " · ")
}

func (s *Server) render(w http.ResponseWriter, name string, data any) {
	var buf bytes.Buffer
	if err := s.tmpl.ExecuteTemplate(&buf, name, data); err != nil {
		fmt.Fprintf(os.Stderr, "slopshipper serve: render %s: %v\n", name, err)
		http.Error(w, "render error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = w.Write(buf.Bytes())
}

func statusDotClass(state string, open bool) string {
	switch machine.State(state) {
	case machine.StateBlocked:
		return "u-dot u-dot--error"
	case machine.StateNeedsDecision, machine.StateRework:
		return "u-dot u-dot--warn"
	case machine.StateRunDone:
		return "u-dot u-dot--ok"
	default:
		if open {
			return "u-dot u-dot--live"
		}
		return "u-dot u-dot--warn"
	}
}

func shortTime(value string) string {
	if value == "" {
		return "—"
	}
	if t, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return t.UTC().Format("15:04:05")
	}
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return t.UTC().Format("15:04:05")
	}
	if len(value) > 19 {
		return value[:19]
	}
	return value
}

func assertLoopback(addr string) error {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("addr: %w", err)
	}
	if host == "localhost" {
		// Final authority is assertBoundLoopback after Listen.
		return nil
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return fmt.Errorf("serve binds loopback only (got %q)", host)
	}
	return nil
}

func assertBoundLoopback(ln net.Listener) error {
	tcp, ok := ln.Addr().(*net.TCPAddr)
	if !ok || tcp.IP == nil || !tcp.IP.IsLoopback() {
		return fmt.Errorf("serve binds loopback only (got %q)", ln.Addr().String())
	}
	return nil
}

// loopbackHost rejects DNS-rebinding Host headers (evil.example → 127.0.0.1).
func loopbackHost(hostport string) bool {
	if hostport == "" {
		return false
	}
	host := hostport
	if h, _, err := net.SplitHostPort(hostport); err == nil {
		host = h
	}
	host = strings.Trim(host, "[]")
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
