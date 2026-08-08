package repo

import (
	"bytes"
	"fmt"
	"net/url"
	"os/exec"
	"path/filepath"
	"strings"
)

// Key returns a stable repo identity for the cwd git root.
// Remote URLs have userinfo stripped so tokens are not persisted or shown in status.
func Key(dir string) (string, error) {
	root, err := gitOutput(dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", fmt.Errorf("not a git repository: %w", err)
	}
	root = filepath.Clean(root)
	remote, err := gitOutput(dir, "config", "--get", "remote.origin.url")
	if err == nil && remote != "" {
		return redactRemote(remote) + "|" + root, nil
	}
	return root, nil
}

func redactRemote(remote string) string {
	remote = strings.TrimSpace(remote)
	if u, err := url.Parse(remote); err == nil && u.Scheme != "" && u.Host != "" {
		u.User = nil
		return u.String()
	}
	// scp-like git@host:path — no userinfo token form to strip
	if strings.Contains(remote, "://") {
		if at := strings.Index(remote, "://"); at >= 0 {
			rest := remote[at+3:]
			if slash := strings.Index(rest, "/"); slash >= 0 {
				authority := rest[:slash]
				path := rest[slash:]
				if userinfoEnd := strings.LastIndex(authority, "@"); userinfoEnd >= 0 {
					return remote[:at+3] + authority[userinfoEnd+1:] + path
				}
			}
		}
	}
	return remote
}

func gitOutput(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(stdout.String()), nil
}
