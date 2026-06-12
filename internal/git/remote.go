package git

import (
	"context"
	"strings"
)

// repoWebURL turns the default remote into a web URL when possible.
func (r Runner) repoWebURL(ctx context.Context) string {
	remote := r.defaultRemote(ctx)
	if remote == "" {
		return ""
	}
	raw := strings.TrimSpace(r.bestEffort(ctx, "remote", "get-url", remote))
	return remoteToWebURL(raw)
}

// remoteToWebURL turns common Git remote formats into plain https URLs.
func remoteToWebURL(raw string) string {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimSuffix(raw, ".git")
	if raw == "" {
		return ""
	}
	if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
		return raw
	}
	if strings.HasPrefix(raw, "git@") {
		withoutUser := strings.TrimPrefix(raw, "git@")
		parts := strings.SplitN(withoutUser, ":", 2)
		if len(parts) == 2 {
			return "https://" + parts[0] + "/" + parts[1]
		}
	}
	if strings.HasPrefix(raw, "ssh://git@") {
		withoutScheme := strings.TrimPrefix(raw, "ssh://git@")
		return "https://" + withoutScheme
	}
	return raw
}
