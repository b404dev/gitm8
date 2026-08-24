package git

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// Releases lists GitHub releases for the repository, newest first.
func (r Runner) Releases(ctx context.Context) ([]Release, error) {
	if _, err := exec.LookPath("gh"); err != nil {
		return nil, fmt.Errorf("gh CLI is required for releases: install gh and run gh auth login")
	}
	out, err := r.commandOutput(ctx, "gh", "release", "list", "--limit", "100", "--json", "tagName,name,publishedAt,isDraft,isPrerelease")
	if err != nil {
		return nil, err
	}
	releases, err := parseReleases(out)
	if err != nil {
		return nil, err
	}
	return releases, nil
}

// ReleaseDetails loads the full notes, metadata, and assets for one release.
func (r Runner) ReleaseDetails(ctx context.Context, tag string) (ReleaseDetail, error) {
	if _, err := exec.LookPath("gh"); err != nil {
		return ReleaseDetail{}, fmt.Errorf("gh CLI is required for releases: install gh and run gh auth login")
	}
	out, err := r.commandOutput(ctx, "gh", "release", "view", strings.TrimSpace(tag), "--json", "tagName,name,publishedAt,isDraft,isPrerelease,author,body,url,targetCommitish,createdAt,assets")
	if err != nil {
		return ReleaseDetail{}, err
	}
	return parseReleaseDetail(out)
}

func parseReleaseDetail(out string) (ReleaseDetail, error) {
	var row struct {
		Tag        string `json:"tagName"`
		Name       string `json:"name"`
		Published  string `json:"publishedAt"`
		Draft      bool   `json:"isDraft"`
		Prerelease bool   `json:"isPrerelease"`
		Author     struct {
			Login string `json:"login"`
		} `json:"author"`
		Body            string `json:"body"`
		URL             string `json:"url"`
		TargetCommitish string `json:"targetCommitish"`
		Created         string `json:"createdAt"`
		Assets          []struct {
			Name        string `json:"name"`
			Size        int64  `json:"size"`
			ContentType string `json:"contentType"`
			URL         string `json:"url"`
		} `json:"assets"`
	}
	if err := json.Unmarshal([]byte(out), &row); err != nil {
		return ReleaseDetail{}, fmt.Errorf("parse GitHub release details: %w", err)
	}
	detail := ReleaseDetail{Release: Release{Tag: row.Tag, Name: row.Name, Published: row.Published, Draft: row.Draft, Prerelease: row.Prerelease}, Author: row.Author.Login, Body: row.Body, URL: row.URL, TargetCommitish: row.TargetCommitish, Created: row.Created}
	for _, asset := range row.Assets {
		detail.Assets = append(detail.Assets, ReleaseAsset{Name: asset.Name, Size: asset.Size, ContentType: asset.ContentType, URL: asset.URL})
	}
	return detail, nil
}

func parseReleases(out string) ([]Release, error) {
	var rows []struct {
		Tag        string `json:"tagName"`
		Name       string `json:"name"`
		Published  string `json:"publishedAt"`
		Draft      bool   `json:"isDraft"`
		Prerelease bool   `json:"isPrerelease"`
	}
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		return nil, fmt.Errorf("parse GitHub releases: %w", err)
	}
	releases := make([]Release, 0, len(rows))
	for _, row := range rows {
		releases = append(releases, Release(row))
	}
	return releases, nil
}

// CreateRelease publishes a GitHub release. GitHub creates tag when it does not exist.
func (r Runner) CreateRelease(ctx context.Context, tag, title, notes string, draft, prerelease, generateNotes bool) (string, error) {
	tag = strings.TrimSpace(tag)
	title = strings.TrimSpace(title)
	if tag == "" {
		return "", fmt.Errorf("a release tag is required")
	}
	if title == "" {
		title = tag
	}
	if _, err := exec.LookPath("gh"); err != nil {
		return "", fmt.Errorf("gh CLI is required for releases: install gh and run gh auth login")
	}
	args := releaseCreateArgs(tag, title, notes, draft, prerelease, generateNotes)
	out, err := r.commandOutput(ctx, "gh", args...)
	if err != nil {
		return out, err
	}
	if strings.TrimSpace(out) == "" {
		return "Release created.\n", nil
	}
	return out, nil
}

func releaseCreateArgs(tag, title, notes string, draft, prerelease, generateNotes bool) []string {
	args := []string{"release", "create", tag, "--title", title}
	if strings.TrimSpace(notes) != "" {
		args = append(args, "--notes", notes)
	}
	if draft {
		args = append(args, "--draft")
	}
	if prerelease {
		args = append(args, "--prerelease")
	}
	if generateNotes {
		args = append(args, "--generate-notes")
	}
	return args
}
