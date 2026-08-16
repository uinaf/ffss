package selfupdate

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const member = "slopmachine"

func makeArchive(t *testing.T, binary []byte) []byte {
	t.Helper()
	var buffer bytes.Buffer
	zipper := gzip.NewWriter(&buffer)
	writer := tar.NewWriter(zipper)
	for _, entry := range []struct {
		name string
		body []byte
	}{
		{"LICENSE", []byte("license\n")},
		{"README.md", []byte("readme\n")},
		{member, binary},
	} {
		if err := writer.WriteHeader(&tar.Header{Name: entry.name, Mode: 0o755, Size: int64(len(entry.body)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		if _, err := writer.Write(entry.body); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := zipper.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

// fixture serves the releases API and download assets for one published
// version of the member.
func fixture(t *testing.T, version string, binary []byte) (*httptest.Server, string, string) {
	t.Helper()
	archive := makeArchive(t, binary)
	digest := sha256.Sum256(archive)
	archiveName := fmt.Sprintf("%s_%s_linux_amd64.tar.gz", member, version)
	checksums := fmt.Sprintf("%s  %s\n", hex.EncodeToString(digest[:]), archiveName)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/releases-api"):
			if r.URL.Query().Get("page") == "1" {
				fmt.Fprintf(w, `[{"tag_name":"othermember/v9.9.9"},{"tag_name":%q}]`, member+"/"+version)
				return
			}
			fmt.Fprint(w, `[]`)
		case strings.HasSuffix(r.URL.Path, "/checksums.txt"):
			_, _ = w.Write([]byte(checksums))
		case strings.HasSuffix(r.URL.Path, "/"+archiveName):
			_, _ = w.Write(archive)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	return server, server.URL + "/releases-api", server.URL
}

func testTarget(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), member)
	if err := os.WriteFile(path, []byte("old binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func options(t *testing.T, current string, server bool) Options {
	t.Helper()
	opts := Options{
		Member:         member,
		CurrentVersion: current,
		ExecutablePath: testTarget(t),
		OS:             "linux",
		Arch:           "amd64",
	}
	if server {
		_, api, download := fixture(t, "v1.2.3", []byte("new binary"))
		opts.APIBase = api
		opts.DownloadBase = download
	}
	return opts
}

func TestRunUpdatesToLatest(t *testing.T) {
	opts := options(t, "v1.0.0", true)
	result, err := Run(t.Context(), opts)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Updated || result.From != "v1.0.0" || result.To != "v1.2.3" {
		t.Fatalf("unexpected result: %+v", result)
	}
	body, err := os.ReadFile(opts.ExecutablePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "new binary" {
		t.Fatalf("binary not replaced: %q", body)
	}
	info, err := os.Stat(opts.ExecutablePath)
	if err != nil || info.Mode().Perm() != 0o755 {
		t.Fatalf("replaced binary must stay executable: %v %v", info.Mode(), err)
	}
}

func TestRunUpToDateIsNoOp(t *testing.T) {
	opts := options(t, "v1.2.3", true)
	result, err := Run(t.Context(), opts)
	if err != nil {
		t.Fatal(err)
	}
	if result.Updated {
		t.Fatalf("up-to-date must not update: %+v", result)
	}
	body, _ := os.ReadFile(opts.ExecutablePath)
	if string(body) != "old binary" {
		t.Fatalf("binary must be untouched: %q", body)
	}
}

func TestCheckReportsWithoutTouching(t *testing.T) {
	opts := options(t, "v1.0.0", true)
	result, err := Check(t.Context(), opts)
	if err != nil {
		t.Fatal(err)
	}
	if result.Updated || result.To != "v1.2.3" {
		t.Fatalf("unexpected check result: %+v", result)
	}
	body, _ := os.ReadFile(opts.ExecutablePath)
	if string(body) != "old binary" {
		t.Fatalf("check must not touch the binary: %q", body)
	}
}

func TestPinnedVersionSkipsResolution(t *testing.T) {
	opts := options(t, "v1.0.0", true)
	opts.APIBase = "http://127.0.0.1:1/unreachable" // resolution must not run
	opts.RequestVersion = "v1.2.3"
	result, err := Run(t.Context(), opts)
	if err != nil || !result.Updated {
		t.Fatalf("pinned update failed: %+v %v", result, err)
	}
}

func TestNonReleaseBuildRefused(t *testing.T) {
	opts := options(t, "", true)
	if _, err := Run(t.Context(), opts); !errors.Is(err, ErrNotRelease) {
		t.Fatalf("non-release build must be refused: %v", err)
	}
}

func TestBrewManagedRefused(t *testing.T) {
	opts := options(t, "v1.0.0", true)
	caskDir := filepath.Join(t.TempDir(), "Caskroom", member, "1.0.0")
	if err := os.MkdirAll(caskDir, 0o755); err != nil {
		t.Fatal(err)
	}
	opts.ExecutablePath = filepath.Join(caskDir, member)
	if err := os.WriteFile(opts.ExecutablePath, []byte("brew binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Run(t.Context(), opts); !errors.Is(err, ErrBrewManaged) {
		t.Fatalf("brew-managed binary must be refused: %v", err)
	}
}

func TestChecksumMismatchFailsClosed(t *testing.T) {
	opts := options(t, "v1.0.0", false)
	archive := makeArchive(t, []byte("tampered"))
	archiveName := fmt.Sprintf("%s_v1.2.3_linux_amd64.tar.gz", member)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/releases-api"):
			fmt.Fprintf(w, `[{"tag_name":%q}]`, member+"/v1.2.3")
		case strings.HasSuffix(r.URL.Path, "/checksums.txt"):
			fmt.Fprintf(w, "%s  %s\n", strings.Repeat("0", 64), archiveName)
		default:
			_, _ = w.Write(archive)
		}
	}))
	t.Cleanup(server.Close)
	opts.APIBase = server.URL + "/releases-api"
	opts.DownloadBase = server.URL
	if _, err := Run(t.Context(), opts); err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("tampered archive must fail closed: %v", err)
	}
	body, _ := os.ReadFile(opts.ExecutablePath)
	if string(body) != "old binary" {
		t.Fatalf("binary must be untouched after failure: %q", body)
	}
}

func TestInvalidPinnedVersionRejected(t *testing.T) {
	opts := options(t, "v1.0.0", true)
	opts.RequestVersion = "nightly"
	if _, err := Run(t.Context(), opts); err == nil || !strings.Contains(err.Error(), "invalid release version") {
		t.Fatalf("invalid pin must be rejected: %v", err)
	}
}

func TestNoPublishedReleaseFound(t *testing.T) {
	opts := options(t, "v1.0.0", false)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `[{"tag_name":"othermember/v9.9.9"}]`)
	}))
	t.Cleanup(server.Close)
	opts.APIBase = server.URL + "/releases-api"
	opts.DownloadBase = server.URL
	// One page of foreign tags, then the pager stops on repetition of the
	// same content only after the bound; keep it simple: the fake always
	// returns the same page, so the resolver walks its bounded pages and
	// reports no release.
	if _, err := Run(t.Context(), opts); err == nil || !strings.Contains(err.Error(), "no published") {
		t.Fatalf("foreign tags only must resolve nothing: %v", err)
	}
}

func TestNonLoopbackHTTPRefused(t *testing.T) {
	opts := options(t, "v1.0.0", true)
	opts.APIBase = "http://mirror.example/releases"
	if _, err := Run(t.Context(), opts); err == nil || !strings.Contains(err.Error(), "must use HTTPS") {
		t.Fatalf("plain-HTTP endpoint must be refused: %v", err)
	}
}

func TestRedirectToPlainHTTPRefused(t *testing.T) {
	opts := options(t, "v1.0.0", false)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/releases-api") {
			http.Redirect(w, r, "http://mirror.example/releases", http.StatusFound)
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(server.Close)
	opts.APIBase = server.URL + "/releases-api"
	opts.DownloadBase = server.URL
	if _, err := Run(t.Context(), opts); err == nil || !strings.Contains(err.Error(), "must use HTTPS") {
		t.Fatalf("redirect onto cleartext must be refused: %v", err)
	}
}

func TestInvalidPinIsCallerInput(t *testing.T) {
	opts := options(t, "v1.0.0", true)
	opts.RequestVersion = "nightly"
	if _, err := Run(t.Context(), opts); !errors.Is(err, ErrInvalidVersion) {
		t.Fatalf("malformed pin must carry the invalid-version sentinel: %v", err)
	}
}
