package config

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/b404dev/gitm8/internal/logging"
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
	MattMode                  bool
	AIProvider                string
	OllamaURL                 string
	AIAvailable               bool
	AIUnavailableReason       string
	LogEnabled                bool
	LogLevel                  string
	LogFile                   string
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
	ensureDefaultConfig(cfg)
	logging.Debug("config", "Load", "defaults_loaded")
	for _, path := range configFilePaths() {
		loadFile(path)
	}
	loadFile(filepath.Join(homeDir(), ".gitm8", "credentials"))
	applyEnv(&cfg)
	applyAIAvailability(&cfg)
	cfg.Profiles = loadProfiles(filepath.Join(homeDir(), ".gitm8", "profiles"))
	logging.Info("config", "Load", "config_loaded",
		logging.F("theme", cfg.Theme),
		logging.F("default_branch", cfg.DefaultBranch),
		logging.F("ai_provider", cfg.AIProvider),
		logging.F("ollama_url", cfg.OllamaURL),
		logging.F("ai_available", cfg.AIAvailable),
		logging.F("ai_unavailable_reason", cfg.AIUnavailableReason),
		logging.F("matt_mode", cfg.MattMode),
		logging.F("profiles", len(cfg.Profiles)),
		logging.F("log_enabled", cfg.LogEnabled),
		logging.F("log_level", cfg.LogLevel),
		logging.F("log_file", cfg.LogFile),
	)
	return cfg
}

// LoadLogSettings reads just the logging settings so startup can configure the
// file logger before the full config load begins.
func LoadLogSettings() (bool, string, string) {
	cfg := defaults()
	ensureDefaultConfig(cfg)
	for _, path := range configFilePaths() {
		loadLogSettingsFile(path, &cfg)
	}
	loadLogSettingsFile(filepath.Join(homeDir(), ".gitm8", "credentials"), &cfg)
	cfg.LogEnabled = envBool("GITM8_LOG_ENABLED", cfg.LogEnabled)
	cfg.LogLevel = normalizeLogLevel(envString("GITM8_LOG_LEVEL", cfg.LogLevel))
	cfg.LogFile = expandHomePath(envString("GITM8_LOG_FILE", cfg.LogFile))
	return cfg.LogEnabled, cfg.LogLevel, cfg.LogFile
}

func loadLogSettingsFile(path string, cfg *Config) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		key, value, ok := parseExport(scanner.Text())
		if !ok || os.Getenv(key) != "" {
			continue
		}
		switch key {
		case "GITM8_LOG_ENABLED":
			parsed, err := strconv.ParseBool(value)
			if err == nil {
				cfg.LogEnabled = parsed
			}
		case "GITM8_LOG_LEVEL":
			cfg.LogLevel = normalizeLogLevel(value)
		case "GITM8_LOG_FILE":
			cfg.LogFile = expandHomePath(value)
		}
	}
}

func configFilePaths() []string {
	home := homeDir()
	return []string{
		filepath.Join(home, ".gitm8", ".gitm8rc"),
		filepath.Join(home, ".gitm8", "gitm8rc"),
		filepath.Join(home, ".gitm8rc"),
	}
}

func ensureDefaultConfig(cfg Config) {
	dir := filepath.Join(homeDir(), ".gitm8")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		logging.Warn("config", "ensureDefaultConfig", "config_dir_create_failed", logging.F("path", dir), logging.F("error", err))
		return
	}
	target := filepath.Join(dir, ".gitm8rc")
	if err := writeDefaultConfigIfMissing(target, configFilePaths(), cfg); err != nil {
		logging.Warn("config", "ensureDefaultConfig", "default_config_create_failed", logging.F("path", target), logging.F("error", err))
	}
}

func writeDefaultConfigIfMissing(target string, existingPaths []string, cfg Config) error {
	for _, path := range existingPaths {
		if _, err := os.Stat(path); err == nil {
			return nil
		} else if !os.IsNotExist(err) {
			return err
		}
	}

	file, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		if os.IsExist(err) {
			return nil
		}
		return err
	}
	defer file.Close()

	_, err = file.WriteString(defaultConfigContent(cfg))
	return err
}

func defaultConfigContent(cfg Config) string {
	var b strings.Builder
	b.WriteString("# gitm8 config\n")
	b.WriteString("# Generated automatically. Edit these values to change gitm8 defaults.\n\n")
	b.WriteString("export GITM8_DEFAULT_BRANCH=\"" + configValue(cfg.DefaultBranch) + "\"\n")
	b.WriteString("export GITM8_EDITOR=\"" + configValue(cfg.Editor) + "\"\n")
	b.WriteString("export GITM8_THEME=\"" + configValue(cfg.Theme) + "\"\n")
	b.WriteString("export GITM8_CONFIRM_DESTRUCTIVE_ACTIONS=\"" + strconv.FormatBool(cfg.ConfirmDestructiveActions) + "\"\n")
	b.WriteString("export GITM8_FETCH_ON_STARTUP=\"" + strconv.FormatBool(cfg.FetchOnStartup) + "\"\n")
	b.WriteString("export GITM8_SHOW_COMMIT_GRAPH=\"" + strconv.FormatBool(cfg.ShowCommitGraph) + "\"\n")
	b.WriteString("# matt_mode: always force push with a raw --force (no safety net, no prompt).\n")
	b.WriteString("export GITM8_MATT_MODE=\"" + strconv.FormatBool(cfg.MattMode) + "\"\n")
	if !cfg.AIAvailable && cfg.AIUnavailableReason != "" {
		b.WriteString("# AI unavailable: " + configValue(cfg.AIUnavailableReason) + "\n")
	}
	b.WriteString("export GITM8_AI_PROVIDER=\"" + configValue(cfg.AIProvider) + "\"\n")
	b.WriteString("export GITM8_OLLAMA_URL=\"" + configValue(cfg.OllamaURL) + "\"\n")
	b.WriteString("export GITM8_LOG_ENABLED=\"" + strconv.FormatBool(cfg.LogEnabled) + "\"\n")
	b.WriteString("export GITM8_LOG_LEVEL=\"" + configValue(cfg.LogLevel) + "\"\n")
	b.WriteString("export GITM8_LOG_FILE=\"$HOME/.gitm8/gitm8.log\"\n")
	return b.String()
}

func configValue(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	return strings.ReplaceAll(value, `"`, `\"`)
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
		logging.Debug("config", "loadProfiles", "profiles_missing", logging.F("path", path))
		return nil
	}
	defer file.Close()
	logging.Debug("config", "loadProfiles", "profiles_opened", logging.F("path", path))

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
	logging.Info("config", "loadProfiles", "profiles_loaded", logging.F("path", path), logging.F("count", len(profiles)))
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
	home := homeDir()
	provider, available, reason := defaultAIProvider()
	return Config{
		DefaultBranch:             "main",
		Editor:                    firstNonEmpty(os.Getenv("EDITOR"), "vi"),
		Theme:                     "default",
		ConfirmDestructiveActions: true,
		FetchOnStartup:            false,
		ShowCommitGraph:           true,
		MattMode:                  false,
		AIProvider:                provider,
		OllamaURL:                 "http://localhost:11434",
		AIAvailable:               available,
		AIUnavailableReason:       reason,
		LogEnabled:                true,
		LogLevel:                  "INFO",
		LogFile:                   filepath.Join(home, ".gitm8", "gitm8.log"),
	}
}

// loadFile reads simple KEY=value lines and puts them into this process's environment.
func loadFile(path string) {
	logging.Debug("config", "loadFile", "config_file_attempt", logging.F("path", path))
	file, err := os.Open(path)
	if err != nil {
		logging.Debug("config", "loadFile", "config_file_missing", logging.F("path", path))
		return
	}
	defer file.Close()
	logging.Info("config", "loadFile", "config_file_opened", logging.F("path", path))

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		key, value, ok := parseExport(scanner.Text())
		if !ok {
			continue
		}
		if os.Getenv(key) == "" {
			_ = os.Setenv(key, value)
			logging.Debug("config", "loadFile", "config_key_applied", logging.F("path", path), logging.F("key", key))
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
	value := parseValue(parts[1])
	return key, value, key != ""
}

// parseValue extracts the value from a config line, honoring surrounding quotes
// and stripping any trailing inline comment such as
// `"claude" # codex, claude, or ollama`.
func parseValue(raw string) string {
	raw = strings.TrimSpace(raw)
	if len(raw) > 0 && (raw[0] == '"' || raw[0] == '\'') {
		quote := raw[0]
		if end := strings.IndexByte(raw[1:], quote); end >= 0 {
			return raw[1 : 1+end]
		}
		// No closing quote; drop the leading quote and keep the rest.
		return strings.TrimSpace(raw[1:])
	}
	// Unquoted value: an inline comment begins at a '#' that starts a word.
	for i := 0; i < len(raw); i++ {
		if raw[i] == '#' && (i == 0 || raw[i-1] == ' ' || raw[i-1] == '\t') {
			raw = raw[:i]
			break
		}
	}
	return strings.TrimSpace(raw)
}

// applyEnv applies supported GITM8_* environment variables to cfg.
func applyEnv(cfg *Config) {
	cfg.DefaultBranch = envString("GITM8_DEFAULT_BRANCH", cfg.DefaultBranch)
	cfg.Editor = envString("GITM8_EDITOR", cfg.Editor)
	cfg.Theme = envString("GITM8_THEME", cfg.Theme)
	cfg.ConfirmDestructiveActions = envBool("GITM8_CONFIRM_DESTRUCTIVE_ACTIONS", cfg.ConfirmDestructiveActions)
	cfg.FetchOnStartup = envBool("GITM8_FETCH_ON_STARTUP", cfg.FetchOnStartup)
	cfg.ShowCommitGraph = envBool("GITM8_SHOW_COMMIT_GRAPH", cfg.ShowCommitGraph)
	cfg.MattMode = envBool("GITM8_MATT_MODE", cfg.MattMode)
	provider := envString("GITM8_AI_PROVIDER", envString("GITM8_COMMIT_MESSAGE_PROVIDER", cfg.AIProvider))
	cfg.AIProvider = normalizeAIProvider(provider)
	cfg.OllamaURL = strings.TrimRight(envString("GITM8_OLLAMA_URL", cfg.OllamaURL), "/")
	cfg.LogEnabled = envBool("GITM8_LOG_ENABLED", cfg.LogEnabled)
	cfg.LogLevel = normalizeLogLevel(envString("GITM8_LOG_LEVEL", cfg.LogLevel))
	cfg.LogFile = expandHomePath(envString("GITM8_LOG_FILE", cfg.LogFile))
	cfg.GithubToken = os.Getenv("GITM8_GITHUB_TOKEN")
	cfg.GitlabToken = os.Getenv("GITM8_GITLAB_TOKEN")
}

func normalizeAIProvider(provider string) string {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "claude":
		return "claude"
	case "ollama", "olama":
		return "ollama"
	default:
		return "codex"
	}
}

var lookPath = exec.LookPath

func defaultAIProvider() (string, bool, string) {
	if aiBinaryInstalled("codex") {
		return "codex", true, ""
	}
	if aiBinaryInstalled("claude") {
		return "claude", true, ""
	}
	return "codex", false, "install codex or claude, or set GITM8_AI_PROVIDER=ollama, to use AI commit/PR generation"
}

func applyAIAvailability(cfg *Config) {
	provider := normalizeAIProvider(cfg.AIProvider)
	cfg.AIProvider = provider
	if provider == "ollama" {
		if aiBinaryInstalled("ollama") {
			cfg.AIAvailable = true
			cfg.AIUnavailableReason = ""
			logging.Info("config", "applyAIAvailability", "ai_provider_available", logging.F("provider", provider))
			return
		}
		cfg.AIAvailable = false
		cfg.AIUnavailableReason = "ollama is not installed or not on PATH"
		logging.Warn("config", "applyAIAvailability", "ai_provider_unavailable",
			logging.F("provider", provider),
			logging.F("reason", cfg.AIUnavailableReason),
		)
		return
	}
	if aiBinaryInstalled(provider) {
		cfg.AIAvailable = true
		cfg.AIUnavailableReason = ""
		logging.Info("config", "applyAIAvailability", "ai_provider_available", logging.F("provider", provider))
		return
	}
	cfg.AIAvailable = false
	cfg.AIUnavailableReason = provider + " is not installed or not on PATH"
	logging.Warn("config", "applyAIAvailability", "ai_provider_unavailable",
		logging.F("provider", provider),
		logging.F("reason", cfg.AIUnavailableReason),
	)
}

func aiBinaryInstalled(provider string) bool {
	binary := normalizeAIProvider(provider)
	if _, err := lookPath(binary); err == nil {
		logging.Debug("config", "aiBinaryInstalled", "ai_binary_found", logging.F("provider", binary))
		return true
	}
	logging.Debug("config", "aiBinaryInstalled", "ai_binary_missing", logging.F("provider", binary))
	return false
}

func normalizeLogLevel(level string) string {
	switch strings.ToUpper(strings.TrimSpace(level)) {
	case "DEBUG":
		return "DEBUG"
	case "WARN", "WARNING":
		return "WARN"
	case "ERROR":
		return "ERROR"
	default:
		return "INFO"
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

func expandHomePath(path string) string {
	home := homeDir()
	switch {
	case path == "~":
		return home
	case strings.HasPrefix(path, "~/"):
		return filepath.Join(home, strings.TrimPrefix(path, "~/"))
	case strings.HasPrefix(path, "$HOME/"):
		return filepath.Join(home, strings.TrimPrefix(path, "$HOME/"))
	default:
		return path
	}
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
