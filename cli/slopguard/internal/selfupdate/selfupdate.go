// Package selfupdate replaces the running binary with a published release.
// It mirrors the installer's contract: member-prefixed tags on the shared
// ffsstack repository, sha256 verification against the release's
// checksums.txt, and an atomic rename into place. Homebrew-managed installs
// are refused — the cask owns those binaries.
package selfupdate

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

// Options configures one selfupdate pass. Zero-value fields take the
// production defaults.
type Options struct {
	// Member is the released product name, for example "slopmachine".
	Member string
	// CurrentVersion is the running binary's release version (vX.Y.Z);
	// empty means a non-release build, which selfupdate refuses.
	CurrentVersion string
	// RequestVersion pins the target release (vX.Y.Z); empty resolves the
	// member's newest published release.
	RequestVersion string
	// APIBase lists releases; default https://api.github.com/repos/uinaf/ffsstack/releases.
	APIBase string
	// DownloadBase serves release assets; default https://github.com/uinaf/ffsstack.
	DownloadBase string
	// ExecutablePath is the binary to replace; empty resolves the running
	// executable through its symlinks.
	ExecutablePath string
	// Client performs HTTP requests; nil uses http.DefaultClient.
	Client *http.Client
	// OS and Arch identify the platform archive; empty uses the runtime's.
	OS, Arch string
}

// Result reports what one pass decided.
type Result struct {
	From    string
	To      string
	Updated bool
	Path    string
}

// ErrNotRelease marks a binary selfupdate refuses to replace.
var ErrNotRelease = errors.New("not a release build; install a published release first")

// ErrBrewManaged marks a Homebrew-managed install.
var ErrBrewManaged = errors.New("this binary is managed by Homebrew; run: brew upgrade --cask")

var releaseTag = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+$`)

// Check resolves the target version without touching the binary.
func Check(ctx context.Context, opts Options) (Result, error) {
	opts, err := withDefaults(opts)
	if err != nil {
		return Result{}, err
	}
	target, err := targetVersion(ctx, opts)
	if err != nil {
		return Result{}, err
	}
	return Result{
		From:    opts.CurrentVersion,
		To:      target,
		Updated: false,
		Path:    opts.ExecutablePath,
	}, nil
}

// Run updates the binary in place when the target differs from the current
// version. The downloaded archive is verified against the release's
// checksums.txt before a byte lands on the executable path.
func Run(ctx context.Context, opts Options) (Result, error) {
	opts, err := withDefaults(opts)
	if err != nil {
		return Result{}, err
	}
	target, err := targetVersion(ctx, opts)
	if err != nil {
		return Result{}, err
	}
	result := Result{From: opts.CurrentVersion, To: target, Path: opts.ExecutablePath}
	if target == opts.CurrentVersion {
		return result, nil
	}
	binary, err := fetchVerifiedBinary(ctx, opts, target)
	if err != nil {
		return Result{}, err
	}
	if err := replaceExecutable(opts.ExecutablePath, binary); err != nil {
		return Result{}, err
	}
	result.Updated = true
	return result, nil
}

func withDefaults(opts Options) (Options, error) {
	if opts.Member == "" {
		return opts, fmt.Errorf("member name required")
	}
	if opts.CurrentVersion == "" {
		return opts, ErrNotRelease
	}
	if opts.APIBase == "" {
		opts.APIBase = "https://api.github.com/repos/uinaf/ffsstack/releases"
	}
	if opts.DownloadBase == "" {
		opts.DownloadBase = "https://github.com/uinaf/ffsstack"
	}
	if opts.Client == nil {
		// Bounded end to end: a stalled server must fail the update, not
		// hang automation. Archives are a few megabytes; ten minutes is
		// generous for the slowest links.
		opts.Client = &http.Client{Timeout: 10 * time.Minute}
	}
	if opts.OS == "" {
		opts.OS = runtime.GOOS
	}
	if opts.Arch == "" {
		opts.Arch = runtime.GOARCH
	}
	if opts.ExecutablePath == "" {
		executable, err := os.Executable()
		if err != nil {
			return opts, fmt.Errorf("resolve executable: %w", err)
		}
		resolved, err := filepath.EvalSymlinks(executable)
		if err != nil {
			return opts, fmt.Errorf("resolve executable symlinks: %w", err)
		}
		opts.ExecutablePath = resolved
	}
	// Homebrew owns its Caskroom; replacing the file under it desyncs the
	// cask and the next brew operation clobbers the update.
	if strings.Contains(opts.ExecutablePath, "/Caskroom/") {
		return opts, fmt.Errorf("%w %s", ErrBrewManaged, opts.Member)
	}
	if opts.RequestVersion != "" && !releaseTag.MatchString(opts.RequestVersion) {
		return opts, fmt.Errorf("invalid release version: %s", opts.RequestVersion)
	}
	for _, base := range []string{opts.APIBase, opts.DownloadBase} {
		if err := requireHTTPS(base); err != nil {
			return opts, err
		}
	}
	return opts, nil
}

// requireHTTPS keeps the installer's transport rail: release endpoints are
// HTTPS, with loopback exempt so local fixtures and mirrors can serve tests.
func requireHTTPS(endpoint string) error {
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return fmt.Errorf("release endpoint %q is not a valid URL: %v", endpoint, err)
	}
	if parsed.Scheme == "https" {
		return nil
	}
	host := parsed.Hostname()
	if parsed.Scheme == "http" && (host == "localhost" || host == "127.0.0.1" || host == "::1") {
		return nil
	}
	return fmt.Errorf("release endpoint %q must use HTTPS", endpoint)
}

func targetVersion(ctx context.Context, opts Options) (string, error) {
	if opts.RequestVersion != "" {
		return opts.RequestVersion, nil
	}
	prefix := opts.Member + "/"
	for page := 1; page <= 10; page++ {
		url := fmt.Sprintf("%s?per_page=100&page=%d", opts.APIBase, page)
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return "", err
		}
		request.Header.Set("Accept", "application/vnd.github+json")
		request.Header.Set("User-Agent", opts.Member+"-selfupdate")
		response, err := opts.Client.Do(request)
		if err != nil {
			return "", fmt.Errorf("list releases: %w", err)
		}
		body, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
		_ = response.Body.Close()
		if err != nil {
			return "", fmt.Errorf("list releases: %w", err)
		}
		if response.StatusCode != http.StatusOK {
			return "", fmt.Errorf("list releases: HTTP %d", response.StatusCode)
		}
		var releases []struct {
			TagName string `json:"tag_name"`
		}
		if err := json.Unmarshal(body, &releases); err != nil {
			return "", fmt.Errorf("decode releases: %w", err)
		}
		if len(releases) == 0 {
			break
		}
		for _, release := range releases {
			version := strings.TrimPrefix(release.TagName, prefix)
			if version != release.TagName && releaseTag.MatchString(version) {
				return version, nil
			}
		}
	}
	return "", fmt.Errorf("no published %s release found", opts.Member)
}

// fetchVerifiedBinary downloads the platform archive and checksums.txt,
// verifies the archive digest, and returns the extracted binary bytes.
func fetchVerifiedBinary(ctx context.Context, opts Options, version string) ([]byte, error) {
	archive := fmt.Sprintf("%s_%s_%s_%s.tar.gz", opts.Member, version, opts.OS, opts.Arch)
	base := fmt.Sprintf("%s/releases/download/%s%%2F%s", opts.DownloadBase, opts.Member, version)
	checksums, err := fetch(ctx, opts, base+"/checksums.txt")
	if err != nil {
		return nil, fmt.Errorf("download checksums.txt: %w", err)
	}
	expected, err := checksumFor(string(checksums), archive)
	if err != nil {
		return nil, err
	}
	payload, err := fetch(ctx, opts, base+"/"+archive)
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", archive, err)
	}
	digest := sha256.Sum256(payload)
	if hex.EncodeToString(digest[:]) != expected {
		return nil, fmt.Errorf("checksum mismatch for %s", archive)
	}
	return extractBinary(payload, opts.Member)
}

func fetch(ctx context.Context, opts Options, url string) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("User-Agent", opts.Member+"-selfupdate")
	response, err := opts.Client.Do(request)
	if err != nil {
		return nil, err
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", response.StatusCode)
	}
	return io.ReadAll(io.LimitReader(response.Body, 128<<20))
}

func checksumFor(checksums, archive string) (string, error) {
	matches := 0
	expected := ""
	for _, line := range strings.Split(checksums, "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 || fields[1] != archive {
			continue
		}
		matches++
		expected = strings.ToLower(fields[0])
	}
	if matches != 1 {
		return "", fmt.Errorf("checksums.txt must contain exactly one entry for %s", archive)
	}
	if len(expected) != 64 {
		return "", fmt.Errorf("malformed checksum for %s", archive)
	}
	for _, r := range expected {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return "", fmt.Errorf("malformed checksum for %s", archive)
		}
	}
	return expected, nil
}

func extractBinary(archive []byte, member string) ([]byte, error) {
	unzipped, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, fmt.Errorf("decompress archive: %w", err)
	}
	reader := tar.NewReader(unzipped)
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read archive: %w", err)
		}
		if filepath.Clean(header.Name) != member || header.Typeflag != tar.TypeReg {
			continue
		}
		binary, err := io.ReadAll(io.LimitReader(reader, 512<<20))
		if err != nil {
			return nil, fmt.Errorf("extract %s: %w", member, err)
		}
		if len(binary) == 0 {
			return nil, fmt.Errorf("archive entry %s is empty", member)
		}
		return binary, nil
	}
	return nil, fmt.Errorf("archive does not contain %s", member)
}

// replaceExecutable writes the new binary next to the target and renames it
// into place, so the swap is atomic and a failure never leaves a truncated
// executable.
func replaceExecutable(path string, binary []byte) error {
	directory := filepath.Dir(path)
	staged, err := os.CreateTemp(directory, "."+filepath.Base(path)+".")
	if err != nil {
		return fmt.Errorf("stage update: %w", err)
	}
	stagedPath := staged.Name()
	defer func() { _ = os.Remove(stagedPath) }()
	if _, err := staged.Write(binary); err != nil {
		_ = staged.Close()
		return fmt.Errorf("stage update: %w", err)
	}
	if err := staged.Close(); err != nil {
		return fmt.Errorf("stage update: %w", err)
	}
	if err := os.Chmod(stagedPath, 0o755); err != nil {
		return fmt.Errorf("stage update: %w", err)
	}
	if err := os.Rename(stagedPath, path); err != nil {
		return fmt.Errorf("replace %s: %w", path, err)
	}
	return nil
}
