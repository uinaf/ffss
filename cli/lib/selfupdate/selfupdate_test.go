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

func makeArchive(t *testing.T, member string, binary []byte) []byte {
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

func fixture(t *testing.T, member, version string, binary []byte) (*httptest.Server, string, string) {
	t.Helper()
	archive := makeArchive(t, member, binary)
	digest := sha256.Sum256(archive)
	archiveName := fmt.Sprintf("%s_%s_linux_amd64.tar.gz", member, version)
	checksums := fmt.Sprintf("%s  %s\n", hex.EncodeToString(digest[:]), archiveName)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("User-Agent"); got != member+"-selfupdate" {
			t.Errorf("User-Agent = %q", got)
		}
		switch {
		case strings.HasPrefix(r.URL.Path, "/releases-api"):
			fmt.Fprintf(w, `[{"tag_name":"other/v9.9.9"},{"tag_name":%q}]`, member+"/"+version)
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

func testTarget(t *testing.T, member string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), member)
	if err := os.WriteFile(path, []byte("old binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func options(t *testing.T, member, current string, server bool) Options {
	t.Helper()
	opts := Options{
		Member:         member,
		CurrentVersion: current,
		ExecutablePath: testTarget(t, member),
		OS:             "linux",
		Arch:           "amd64",
	}
	if server {
		_, api, download := fixture(t, member, "v1.2.3", []byte("new binary"))
		opts.APIBase = api
		opts.DownloadBase = download
	}
	return opts
}

func forEachMember(t *testing.T, run func(*testing.T, string)) {
	t.Helper()
	for _, member := range []string{"slopguard", "slopmachine"} {
		t.Run(member, func(t *testing.T) { run(t, member) })
	}
}

func TestRunUpdatesBothMembers(t *testing.T) {
	forEachMember(t, func(t *testing.T, member string) {
		opts := options(t, member, "v1.0.0", true)
		result, err := Run(t.Context(), opts)
		if err != nil {
			t.Fatal(err)
		}
		if !result.Updated || result.From != "v1.0.0" || result.To != "v1.2.3" {
			t.Fatalf("result = %+v", result)
		}
		body, err := os.ReadFile(opts.ExecutablePath)
		if err != nil || string(body) != "new binary" {
			t.Fatalf("binary = %q, err = %v", body, err)
		}
		info, err := os.Stat(opts.ExecutablePath)
		if err != nil || info.Mode().Perm() != 0o755 {
			t.Fatalf("mode = %v, err = %v", info.Mode(), err)
		}
	})
}

func TestCheckAndUpToDateDoNotTouchBinary(t *testing.T) {
	forEachMember(t, func(t *testing.T, member string) {
		for _, current := range []string{"v1.0.0", "v1.2.3"} {
			opts := options(t, member, current, true)
			var result Result
			var err error
			if current == "v1.0.0" {
				result, err = Check(t.Context(), opts)
			} else {
				result, err = Run(t.Context(), opts)
			}
			if err != nil || result.Updated || result.To != "v1.2.3" {
				t.Fatalf("current %s: result = %+v, err = %v", current, result, err)
			}
			body, _ := os.ReadFile(opts.ExecutablePath)
			if string(body) != "old binary" {
				t.Fatalf("current %s changed binary: %q", current, body)
			}
		}
	})
}

func TestPinnedVersionSkipsReleaseResolution(t *testing.T) {
	forEachMember(t, func(t *testing.T, member string) {
		opts := options(t, member, "v1.0.0", true)
		opts.APIBase = "http://127.0.0.1:1/unreachable"
		opts.RequestVersion = "v1.2.3"
		result, err := Run(t.Context(), opts)
		if err != nil || !result.Updated {
			t.Fatalf("result = %+v, err = %v", result, err)
		}
	})
}

func TestRefusalsPreserveSentinels(t *testing.T) {
	forEachMember(t, func(t *testing.T, member string) {
		for _, current := range []string{"", "dev", "v1.2.3-dirty"} {
			t.Run("non-release-"+current, func(t *testing.T) {
				opts := options(t, member, current, true)
				if _, err := Run(t.Context(), opts); !errors.Is(err, ErrNotRelease) {
					t.Fatalf("error = %v", err)
				}
			})
		}
		t.Run("invalid-member", func(t *testing.T) {
			opts := options(t, member, "v1.0.0", true)
			opts.Member = "slopguard/header\r\n"
			if _, err := Run(t.Context(), opts); err == nil || !strings.Contains(err.Error(), "invalid member name") {
				t.Fatalf("error = %v", err)
			}
		})
		t.Run("homebrew", func(t *testing.T) {
			opts := options(t, member, "v1.0.0", true)
			caskDir := filepath.Join(t.TempDir(), "Caskroom", member, "1.0.0")
			if err := os.MkdirAll(caskDir, 0o755); err != nil {
				t.Fatal(err)
			}
			opts.ExecutablePath = filepath.Join(caskDir, member)
			if _, err := Run(t.Context(), opts); !errors.Is(err, ErrBrewManaged) {
				t.Fatalf("error = %v", err)
			}
		})
		t.Run("invalid-version", func(t *testing.T) {
			opts := options(t, member, "v1.0.0", true)
			opts.RequestVersion = "nightly"
			if _, err := Run(t.Context(), opts); !errors.Is(err, ErrInvalidVersion) {
				t.Fatalf("error = %v", err)
			}
		})
	})
}

func TestChecksumMismatchFailsClosedForBothMembers(t *testing.T) {
	forEachMember(t, func(t *testing.T, member string) {
		opts := options(t, member, "v1.0.0", false)
		archive := makeArchive(t, member, []byte("tampered"))
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
			t.Fatalf("error = %v", err)
		}
		body, _ := os.ReadFile(opts.ExecutablePath)
		if string(body) != "old binary" {
			t.Fatalf("binary changed after failure: %q", body)
		}
	})
}

func TestChecksumManifestRequiresOneValidDigest(t *testing.T) {
	archive := "slopguard_v1.2.3_linux_amd64.tar.gz"
	valid := strings.Repeat("a", 64)
	for name, manifest := range map[string]string{
		"missing":   valid + "  other.tar.gz\n",
		"duplicate": valid + "  " + archive + "\n" + valid + "  " + archive + "\n",
		"short":     "abcd  " + archive + "\n",
		"non-hex":   strings.Repeat("z", 64) + "  " + archive + "\n",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := checksumFor(manifest, archive); err == nil {
				t.Fatal("malformed manifest accepted")
			}
		})
	}
	got, err := checksumFor(strings.ToUpper(valid)+"  "+archive+"\n", archive)
	if err != nil || got != valid {
		t.Fatalf("valid digest = %q, err = %v", got, err)
	}
}

func TestTransportRails(t *testing.T) {
	opts := options(t, "slopguard", "v1.0.0", true)
	opts.APIBase = "http://mirror.example/releases"
	if _, err := Run(t.Context(), opts); err == nil || !strings.Contains(err.Error(), "must use HTTPS") {
		t.Fatalf("plain HTTP error = %v", err)
	}

	opts = options(t, "slopguard", "v1.0.0", false)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://mirror.example/releases", http.StatusFound)
	}))
	t.Cleanup(server.Close)
	opts.APIBase = server.URL + "/releases-api"
	opts.DownloadBase = server.URL
	if _, err := Run(t.Context(), opts); err == nil || !strings.Contains(err.Error(), "must use HTTPS") {
		t.Fatalf("redirect error = %v", err)
	}

	opts = options(t, "slopguard", "v1.0.0", false)
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://mirror.example/releases", http.StatusFound)
	}))
	t.Cleanup(server.Close)
	customClient := &http.Client{}
	opts.Client = customClient
	opts.APIBase = server.URL + "/releases-api"
	opts.DownloadBase = server.URL
	if _, err := Run(t.Context(), opts); err == nil || !strings.Contains(err.Error(), "must use HTTPS") {
		t.Fatalf("custom-client redirect error = %v", err)
	}
	if customClient.CheckRedirect != nil {
		t.Fatal("caller-owned HTTP client was mutated")
	}
}

func TestNoPublishedMemberRelease(t *testing.T) {
	opts := options(t, "slopmachine", "v1.0.0", false)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `[{"tag_name":"slopguard/v9.9.9"}]`)
	}))
	t.Cleanup(server.Close)
	opts.APIBase = server.URL + "/releases-api"
	opts.DownloadBase = server.URL
	if _, err := Run(t.Context(), opts); err == nil || !strings.Contains(err.Error(), "no published slopmachine release") {
		t.Fatalf("error = %v", err)
	}
}
