package repo

import (
	"bytes"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Key returns a stable repo identity for the cwd git root.
// Remote URLs have userinfo stripped so tokens are not persisted or shown in status.
func Key(dir string) (string, error) {
	key, _, err := Keys(dir)
	return key, err
}

// ErrNotRepository marks a directory that is definitively outside any Git
// worktree, as opposed to a failed inspection (git missing, permission
// errors), which callers must treat as unknown rather than safe.
var ErrNotRepository = errors.New("not a git repository")

// Keys returns the credential-free repo identity and checkout root.
func Keys(dir string) (string, string, error) {
	root, err := gitOutput(dir, "rev-parse", "--show-toplevel")
	if err != nil {
		// git prints this exact fatal for directories outside any
		// worktree; every other failure is an inspection error. Message
		// sniffing is confined to this git boundary.
		if strings.Contains(err.Error(), "not a git repository") {
			return "", "", fmt.Errorf("%w: %w", ErrNotRepository, err)
		}
		return "", "", fmt.Errorf("inspect git worktree: %w", err)
	}
	root = filepath.Clean(root)
	remote, err := gitOutput(dir, "config", "--get", "remote.origin.url")
	if err == nil && remote != "" {
		return redactRemote(remote) + "|" + root, root, nil
	}
	return root, root, nil
}

func redactRemote(remote string) string {
	remote = strings.TrimSpace(remote)
	if strings.HasPrefix(remote, "//") {
		if u, err := url.Parse(remote); err == nil && u.Host != "" {
			u.User = nil
			u.Host = stripAuthorityUserinfo(u.Host)
			u.Path = stripURLSuffix(u.Path)
			u.RawPath = ""
			u.RawQuery = ""
			u.ForceQuery = false
			u.Fragment = ""
			return u.String()
		}
		return stripURLSuffix("//" + stripAuthorityUserinfo(remote[2:]))
	}
	if explicitLocalRemote(remote) {
		return stripLocalCredentialSuffix(remote)
	}
	if u, err := url.Parse(remote); err == nil && u.Scheme != "" {
		if u.Opaque != "" {
			u.Opaque = stripOpaqueUserinfo(u.Opaque)
			u.Opaque = stripURLSuffix(u.Opaque)
		} else {
			u.User = nil
			u.Host = stripAuthorityUserinfo(u.Host)
			if u.Host == "" {
				u.Path = stripFirstPathSegmentUserinfo(u.Path)
			}
			u.Path = stripURLSuffix(u.Path)
			u.RawPath = ""
		}
		u.RawQuery = ""
		u.ForceQuery = false
		u.Fragment = ""
		return u.String()
	}
	if marker := malformedSchemeMarker(remote); marker >= 0 {
		authorityStart := marker + 2
		return stripURLSuffix(remote[:authorityStart] + stripAuthorityUserinfo(remote[authorityStart:]))
	}
	if remoteColon := strings.Index(remote, ":"); remoteColon > 0 {
		firstSlash := strings.IndexAny(remote, `/\`)
		if firstSlash < 0 || remoteColon < firstSlash {
			remoteColon = colonRemoteSeparator(remote, remoteColon, firstSlash)
			if strings.Contains(remote, "://") {
				return stripURLSuffix(stripFallbackUserinfo(remote))
			}
			return stripURLSuffix(stripColonRemoteUserinfo(remote, remoteColon))
		}
	}
	return stripLocalCredentialSuffix(remote)
}

func stripOpaqueUserinfo(value string) string {
	if strings.HasPrefix(value, "//") {
		return "//" + stripAuthorityUserinfo(value[2:])
	}
	if marker := malformedSchemeMarker(value); marker >= 0 {
		authorityStart := marker + 2
		return value[:authorityStart] + stripOpaqueUserinfo(value[authorityStart:])
	}
	authorityEnd := strings.Index(value, "/")
	if authorityEnd < 0 {
		authorityEnd = len(value)
	}
	authority := value[:authorityEnd]
	separator := strings.Index(authority, ":")
	userinfoEnd := userinfoDelimiterEnd(authority)
	if separator >= 0 && (userinfoEnd < 0 || userinfoEnd <= separator) {
		return stripAuthorityUserinfo(authority[:separator]) + ":" + stripOpaqueUserinfo(value[separator+1:])
	}
	return stripAuthorityUserinfo(value)
}

func colonRemoteSeparator(remote string, separator, firstSlash int) int {
	userinfoEnd := userinfoDelimiterEnd(remote)
	if userinfoEnd <= separator || (firstSlash >= 0 && userinfoEnd > firstSlash) {
		return separator
	}
	if next := strings.Index(remote[userinfoEnd:], ":"); next >= 0 {
		return userinfoEnd + next
	}
	return separator
}

func stripFirstPathSegmentUserinfo(path string) string {
	start := 0
	for start < len(path) && path[start] == '/' {
		start++
	}
	end := strings.Index(path[start:], "/")
	if end < 0 {
		end = len(path)
	} else {
		end += start
	}
	if userinfoEnd := userinfoDelimiterEnd(path[start:end]); userinfoEnd >= 0 {
		return path[:start] + path[start+userinfoEnd:]
	}
	return path
}

func stripColonRemoteUserinfo(remote string, separator int) string {
	authority := remote[:separator]
	path := remote[separator:]
	if userinfoEnd := userinfoDelimiterEnd(authority); userinfoEnd >= 0 {
		return authority[userinfoEnd:] + path
	}
	return remote
}

func explicitLocalRemote(remote string) bool {
	if strings.HasPrefix(remote, "/") || strings.HasPrefix(remote, "./") || strings.HasPrefix(remote, "../") || strings.HasPrefix(remote, "~/") {
		return true
	}
	return len(remote) >= 2 && ((remote[0] >= 'a' && remote[0] <= 'z') || (remote[0] >= 'A' && remote[0] <= 'Z')) && remote[1] == ':' && !strings.HasPrefix(remote[2:], "//")
}

func malformedSchemeMarker(remote string) int {
	marker := strings.Index(remote, "//")
	if marker > 0 && validScheme(remote[:marker]) {
		return marker
	}
	return -1
}

func stripURLSuffix(remote string) string {
	end := len(remote)
	if literal := strings.IndexAny(remote, "?#"); literal >= 0 {
		end = literal
	}
	lower := strings.ToLower(remote)
	for _, encoded := range []string{"%3f", "%23"} {
		if index := strings.Index(lower, encoded); index >= 0 && index < end {
			end = index
		}
	}
	return stripLocalCredentialSuffix(remote[:end])
}

func stripLocalCredentialSuffix(remote string) string {
	lower := strings.ToLower(remote)
	for _, marker := range []string{"?", "#", ";", "%3f", "%23", "%3b"} {
		start := 0
		for {
			index := strings.Index(lower[start:], marker)
			if index < 0 {
				break
			}
			index += start
			suffix := lower[index+len(marker):]
			if credentialParameters(suffix) {
				return remote[:index]
			}
			start = index + len(marker)
		}
	}
	return remote
}

func credentialParameters(value string) bool {
	if decoded, err := url.QueryUnescape(value); err == nil {
		value = decoded
	}
	value = strings.ToLower(value)
	for _, part := range strings.FieldsFunc(value, func(r rune) bool { return r == '&' || r == ';' }) {
		key, _, found := strings.Cut(part, "=")
		if !found {
			continue
		}
		switch strings.TrimSpace(key) {
		case "access_token", "api_key", "apikey", "auth", "authorization", "client_secret", "credential", "oauth_token", "password", "private_token", "secret", "token":
			return true
		}
	}
	return false
}

// MatchesKey reports whether candidate is an unambiguous legacy identity for
// the same checkout root and normalized remote as current.
func MatchesKey(candidate, current, root string) bool {
	sanitizedCandidate, ok := SanitizeKey(candidate, root)
	if !ok {
		return false
	}
	sanitizedCurrent, ok := SanitizeKey(current, root)
	return ok && sanitizedCandidate == sanitizedCurrent
}

// SanitizeKey returns a credential-free identity without changing its remote.
func SanitizeKey(key, root string) (string, bool) {
	remote, ok := remoteFromKey(key, root)
	if !ok {
		return "", false
	}
	if remote == "" {
		return root, true
	}
	return redactRemote(remote) + "|" + root, true
}

func remoteFromKey(key, root string) (string, bool) {
	if key == root {
		return "", true
	}
	suffix := "|" + root
	if !strings.HasSuffix(key, suffix) {
		return "", false
	}
	remote := strings.TrimSuffix(key, suffix)
	if remote == "" || strings.Contains(remote, "|") {
		return "", false
	}
	return remote, true
}

func stripFallbackUserinfo(remote string) string {
	schemeEnd := strings.Index(remote, "://")
	if schemeEnd < 0 {
		return remote
	}
	rest := remote[schemeEnd+3:]
	authorityEnd := strings.Index(rest, "/")
	if authorityEnd < 0 {
		authorityEnd = len(rest)
	}
	authority := rest[:authorityEnd]
	userinfoEnd := userinfoDelimiterEnd(authority)
	if userinfoEnd < 0 {
		return remote
	}
	return remote[:schemeEnd+3] + authority[userinfoEnd:] + rest[authorityEnd:]
}

func stripAuthorityUserinfo(value string) string {
	authorityEnd := strings.Index(value, "/")
	if authorityEnd < 0 {
		authorityEnd = len(value)
	}
	authority := value[:authorityEnd]
	if userinfoEnd := userinfoDelimiterEnd(authority); userinfoEnd >= 0 {
		return authority[userinfoEnd:] + value[authorityEnd:]
	}
	return value
}

func userinfoDelimiterEnd(value string) int {
	end := -1
	if at := strings.LastIndex(value, "@"); at >= 0 {
		end = at + 1
	}
	if encodedAt := strings.LastIndex(strings.ToLower(value), "%40"); encodedAt >= 0 && encodedAt+3 > end {
		end = encodedAt + 3
	}
	return end
}

func validScheme(value string) bool {
	for i, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (i > 0 && ((r >= '0' && r <= '9') || r == '+' || r == '-' || r == '.')) {
			continue
		}
		return false
	}
	return value != ""
}

func gitOutput(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	// The not-a-repository sentinel matches git's fatal message; a stable
	// locale keeps it matchable everywhere.
	cmd.Env = append(os.Environ(), "LC_ALL=C", "LANG=C")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(stdout.String()), nil
}
