package config

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Config is the final set of settings used by the app after defaults, files,
// and environment variables are applied.
type Config struct {
	DefaultBranch             string
	Editor                    string
	Theme                     string
	ConfirmDestructiveActions bool
	FetchOnStartup            bool
	ShowCommitGraph           bool
	CommitMessageProvider     string
	GithubToken               string
	GitlabToken               string
	Profiles                  []Profile
}

// Profile is a Git name/email pair the user can select inside gitm8.
type Profile struct {
	Label string
	Name  string
	Email string
}

// Load builds the final config from defaults, dotfiles, environment variables,
// and identity profiles.
func Load() Config {
	cfg := defaults()
	loadFile(filepath.Join(homeDir(), ".gitm8rc"))
	loadFile(filepath.Join(homeDir(), ".gitm8", "credentials"))
	applyEnv(&cfg)
	cfg.Profiles = loadProfiles(filepath.Join(homeDir(), ".gitm8", "profiles"))
	return cfg
}

// loadProfiles reads Git identities from ~/.gitm8/profiles.
//
// The file format is:
//
//	Work = Ada Lovelace <ada@work.example>
//	Personal = Ada <ada@personal.example>
//
// Blank lines and lines beginning with # are ignored, as are malformed lines.
func loadProfiles(path string) []Profile {
	file, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer file.Close()

	var profiles []Profile
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		label := strings.TrimSpace(parts[0])
		name, email, ok := parseIdentity(parts[1])
		if !ok || label == "" {
			continue
		}
		profiles = append(profiles, Profile{Label: label, Name: name, Email: email})
	}
	return profiles
}

// parseIdentity splits "Name <email>" into the two values Git config needs.
// It returns ok=false if the email part is missing or empty.
func parseIdentity(value string) (string, string, bool) {
	value = strings.TrimSpace(value)
	open := strings.Index(value, "<")
	closeIdx := strings.LastIndex(value, ">")
	if open < 0 || closeIdx < 0 || closeIdx < open {
		return "", "", false
	}
	name := strings.TrimSpace(value[:open])
	email := strings.TrimSpace(value[open+1 : closeIdx])
	if email == "" {
		return "", "", false
	}
	return name, email, true
}

// defaults returns the app settings used when the user has not configured anything.
func defaults() Config {
	return Config{
		DefaultBranch:             "main",
		Editor:                    firstNonEmpty(os.Getenv("EDITOR"), "vi"),
		Theme:                     "default",
		ConfirmDestructiveActions: true,
		FetchOnStartup:            false,
		ShowCommitGraph:           true,
		CommitMessageProvider:     "codex",
	}
}

// loadFile reads simple KEY=value lines and puts them into this process's environment.
func loadFile(path string) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		key, value, ok := parseExport(scanner.Text())
		if !ok {
			continue
		}
		if os.Getenv(key) == "" {
			_ = os.Setenv(key, value)
		}
	}
}

// parseExport reads one config line such as `export GITM8_THEME="catppuccin"`.
func parseExport(line string) (string, string, bool) {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return "", "", false
	}
	line = strings.TrimPrefix(line, "export ")
	parts := strings.SplitN(line, "=", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	key := strings.TrimSpace(parts[0])
	value := strings.TrimSpace(parts[1])
	value = strings.Trim(value, `"'`)
	return key, value, key != ""
}

// applyEnv applies supported GITM8_* environment variables to cfg.
func applyEnv(cfg *Config) {
	cfg.DefaultBranch = envString("GITM8_DEFAULT_BRANCH", cfg.DefaultBranch)
	cfg.Editor = envString("GITM8_EDITOR", cfg.Editor)
	cfg.Theme = envString("GITM8_THEME", cfg.Theme)
	cfg.ConfirmDestructiveActions = envBool("GITM8_CONFIRM_DESTRUCTIVE_ACTIONS", cfg.ConfirmDestructiveActions)
	cfg.FetchOnStartup = envBool("GITM8_FETCH_ON_STARTUP", cfg.FetchOnStartup)
	cfg.ShowCommitGraph = envBool("GITM8_SHOW_COMMIT_GRAPH", cfg.ShowCommitGraph)
	cfg.CommitMessageProvider = normalizeCommitMessageProvider(envString("GITM8_COMMIT_MESSAGE_PROVIDER", cfg.CommitMessageProvider))
	cfg.GithubToken = os.Getenv("GITM8_GITHUB_TOKEN")
	cfg.GitlabToken = os.Getenv("GITM8_GITLAB_TOKEN")
}

func normalizeCommitMessageProvider(provider string) string {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "claude":
		return "claude"
	default:
		return "codex"
	}
}

// envString returns an environment value or the provided fallback.
func envString(key string, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

// envBool parses a true/false environment value, or keeps the fallback if invalid.
func envBool(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

// homeDir returns the user's home directory, or "." if the OS cannot provide it.
func homeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	return home
}

// firstNonEmpty returns the first populated string from a fallback list.
func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
