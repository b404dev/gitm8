package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func stubLookPath(t *testing.T, found map[string]bool) {
	t.Helper()
	original := lookPath
	lookPath = func(file string) (string, error) {
		if found[file] {
			return "/test/bin/" + file, nil
		}
		return "", errors.New("not found")
	}
	t.Cleanup(func() {
		lookPath = original
	})
}

// TestParseExport checks the supported shell-style export syntax.
func TestParseExport(t *testing.T) {
	key, value, ok := parseExport(`export GITM8_DEFAULT_BRANCH="trunk"`)
	if !ok {
		t.Fatal("expected export to parse")
	}
	if key != "GITM8_DEFAULT_BRANCH" {
		t.Fatalf("key = %q", key)
	}
	if value != "trunk" {
		t.Fatalf("value = %q", value)
	}
}

// TestParseExportIgnoresComments makes sure commented config lines do nothing.
func TestParseExportIgnoresComments(t *testing.T) {
	_, _, ok := parseExport(`# export GITM8_DEFAULT_BRANCH="trunk"`)
	if ok {
		t.Fatal("expected comment to be ignored")
	}
}

// TestParseExportStripsInlineComments makes sure a trailing comment after the
// value is not folded into the value itself.
func TestParseExportStripsInlineComments(t *testing.T) {
	cases := []struct {
		line  string
		key   string
		value string
	}{
		{`GITM8_AI_PROVIDER="claude" # codex, claude, or ollama`, "GITM8_AI_PROVIDER", "claude"},
		{`export GITM8_AI_PROVIDER="ollama"  # trailing`, "GITM8_AI_PROVIDER", "ollama"},
		{`GITM8_LOG_LEVEL=INFO # DEBUG, INFO, WARN, ERROR`, "GITM8_LOG_LEVEL", "INFO"},
		{`GITM8_THEME='catppuccin'`, "GITM8_THEME", "catppuccin"},
		{`GITM8_DEFAULT_BRANCH=main`, "GITM8_DEFAULT_BRANCH", "main"},
	}
	for _, tc := range cases {
		key, value, ok := parseExport(tc.line)
		if !ok {
			t.Fatalf("parseExport(%q) ok = false, want true", tc.line)
		}
		if key != tc.key || value != tc.value {
			t.Errorf("parseExport(%q) = (%q, %q), want (%q, %q)", tc.line, key, value, tc.key, tc.value)
		}
	}
}

// TestApplyEnvMattMode reads GITM8_MATT_MODE into the config.
func TestApplyEnvMattMode(t *testing.T) {
	t.Setenv("GITM8_MATT_MODE", "true")
	cfg := defaults()
	applyEnv(&cfg)
	if !cfg.MattMode {
		t.Fatal("MattMode = false, want true from GITM8_MATT_MODE")
	}
}

// TestDefaultConfigContentIncludesMattMode keeps matt_mode in generated configs.
func TestDefaultConfigContentIncludesMattMode(t *testing.T) {
	got := defaultConfigContent(defaults())
	if !strings.Contains(got, `export GITM8_MATT_MODE="false"`) {
		t.Fatalf("default config missing matt_mode line:\n%s", got)
	}
}

func TestNeedsFirstRunSetup(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "Github")
	if !NeedsFirstRunSetup(Config{WorkspaceDir: missing}) {
		t.Fatal("missing workspace should require setup")
	}
	if NeedsFirstRunSetup(Config{WorkspaceDir: missing, FirstRunDismissed: true}) {
		t.Fatal("dismissed setup should remain dismissed")
	}
	if err := os.Mkdir(missing, 0o755); err != nil {
		t.Fatal(err)
	}
	if NeedsFirstRunSetup(Config{WorkspaceDir: missing}) {
		t.Fatal("existing workspace should not require setup")
	}
}

func TestSaveAndDismissFirstRunSetup(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	homeDir = func() string { return home }
	t.Cleanup(func() { homeDir = systemHomeDir })

	values := SetupValues{WorkspaceDir: filepath.Join(home, "Code"), DefaultBranch: "trunk", Editor: "nano"}
	if err := SaveFirstRunSetup(values); err != nil {
		t.Fatal(err)
	}
	if err := DismissFirstRunSetup(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(home, ".gitm8", ".gitm8rc"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	for _, want := range []string{`GITM8_WORKSPACE_DIR="` + values.WorkspaceDir, `GITM8_DEFAULT_BRANCH="trunk"`, `GITM8_EDITOR="nano"`, `GITM8_FIRST_RUN_DISMISSED="true"`} {
		if !strings.Contains(content, want) {
			t.Errorf("saved config missing %q:\n%s", want, content)
		}
	}
	if strings.Count(content, "GITM8_FIRST_RUN_DISMISSED=") != 1 {
		t.Fatalf("dismissed setting should be replaced, not duplicated:\n%s", content)
	}
}

// TestParseIdentity checks valid and invalid profile identity values.
func TestParseIdentity(t *testing.T) {
	cases := []struct {
		value     string
		wantName  string
		wantEmail string
		wantOK    bool
	}{
		{"Ada Lovelace <ada@work.example>", "Ada Lovelace", "ada@work.example", true},
		{"  Ada   <ada@personal.example>  ", "Ada", "ada@personal.example", true},
		{"<just@email.example>", "", "just@email.example", true},
		{"No Email Here", "", "", false},
		{"Empty <>", "", "", false},
	}
	for _, tc := range cases {
		name, email, ok := parseIdentity(tc.value)
		if ok != tc.wantOK || name != tc.wantName || email != tc.wantEmail {
			t.Errorf("parseIdentity(%q) = (%q, %q, %t), want (%q, %q, %t)",
				tc.value, name, email, ok, tc.wantName, tc.wantEmail, tc.wantOK)
		}
	}
}

// TestNormalizeAIProvider keeps unsupported values on the safe default.
func TestNormalizeAIProvider(t *testing.T) {
	cases := []struct {
		value string
		want  string
	}{
		{"codex", "codex"},
		{"claude", "claude"},
		{"ollama", "ollama"},
		{"olama", "ollama"},
		{" Claude ", "claude"},
		{"unknown", "codex"},
		{"", "codex"},
	}
	for _, tc := range cases {
		if got := normalizeAIProvider(tc.value); got != tc.want {
			t.Errorf("normalizeAIProvider(%q) = %q, want %q", tc.value, got, tc.want)
		}
	}
}

// TestNormalizeLogLevel keeps logging levels predictable.
func TestNormalizeLogLevel(t *testing.T) {
	cases := []struct {
		value string
		want  string
	}{
		{"debug", "DEBUG"},
		{"INFO", "INFO"},
		{"warning", "WARN"},
		{"ERROR", "ERROR"},
		{"nope", "INFO"},
		{"", "INFO"},
	}
	for _, tc := range cases {
		if got := normalizeLogLevel(tc.value); got != tc.want {
			t.Errorf("normalizeLogLevel(%q) = %q, want %q", tc.value, got, tc.want)
		}
	}
}

// TestDefaultAIProviderPrefersCodex chooses Codex when both providers exist.
func TestDefaultAIProviderPrefersCodex(t *testing.T) {
	stubLookPath(t, map[string]bool{"codex": true, "claude": true})

	provider, available, reason := defaultAIProvider()
	if provider != "codex" || !available || reason != "" {
		t.Fatalf("defaultAIProvider() = (%q, %t, %q), want codex available", provider, available, reason)
	}
}

// TestDefaultAIProviderFallsBackToClaude chooses Claude when Codex is missing.
func TestDefaultAIProviderFallsBackToClaude(t *testing.T) {
	stubLookPath(t, map[string]bool{"claude": true})

	provider, available, reason := defaultAIProvider()
	if provider != "claude" || !available || reason != "" {
		t.Fatalf("defaultAIProvider() = (%q, %t, %q), want claude available", provider, available, reason)
	}
}

// TestDefaultAIProviderMarksUnavailable keeps a clear default when no AI CLI exists.
func TestDefaultAIProviderMarksUnavailable(t *testing.T) {
	stubLookPath(t, map[string]bool{})

	provider, available, reason := defaultAIProvider()
	if provider != "codex" || available || !strings.Contains(reason, "install codex or claude") {
		t.Fatalf("defaultAIProvider() = (%q, %t, %q), want codex unavailable install message", provider, available, reason)
	}
}

// TestApplyAIAvailabilityDoesNotSwitchConfiguredProvider preserves user choice.
func TestApplyAIAvailabilityDoesNotSwitchConfiguredProvider(t *testing.T) {
	stubLookPath(t, map[string]bool{"codex": true})

	cfg := Config{AIProvider: "claude"}
	applyAIAvailability(&cfg)
	if cfg.AIProvider != "claude" {
		t.Fatalf("AIProvider = %q, want configured claude", cfg.AIProvider)
	}
	if cfg.AIAvailable {
		t.Fatal("AIAvailable = true, want false because claude is missing")
	}
	if !strings.Contains(cfg.AIUnavailableReason, "claude") {
		t.Fatalf("AIUnavailableReason = %q, want claude reason", cfg.AIUnavailableReason)
	}
}

// TestExpandHomePath supports shell-style examples in ~/.gitm8/.gitm8rc.
func TestExpandHomePath(t *testing.T) {
	if got := expandHomePath("/tmp/gitm8.log"); got != "/tmp/gitm8.log" {
		t.Fatalf("expandHomePath absolute = %q", got)
	}
	if got := expandHomePath("$HOME/.gitm8/gitm8.log"); got == "$HOME/.gitm8/gitm8.log" {
		t.Fatal("expandHomePath should expand $HOME prefix")
	}
	if got := expandHomePath("~/.gitm8/gitm8.log"); got == "~/.gitm8/gitm8.log" {
		t.Fatal("expandHomePath should expand ~/ prefix")
	}
}

// TestConfigFilePathsPrefersConfigDirectory checks the new dotfile location is first.
func TestConfigFilePathsPrefersConfigDirectory(t *testing.T) {
	paths := configFilePaths()
	if len(paths) != 3 {
		t.Fatalf("configFilePaths length = %d, want 3", len(paths))
	}
	if filepath.Base(paths[0]) != ".gitm8rc" || filepath.Base(filepath.Dir(paths[0])) != ".gitm8" {
		t.Fatalf("first config path = %q, want ~/.gitm8/.gitm8rc", paths[0])
	}
	if filepath.Base(paths[1]) != "gitm8rc" || filepath.Base(filepath.Dir(paths[1])) != ".gitm8" {
		t.Fatalf("second config path = %q, want fallback ~/.gitm8/gitm8rc", paths[1])
	}
	if filepath.Base(paths[2]) != ".gitm8rc" {
		t.Fatalf("third config path = %q, want legacy ~/.gitm8rc", paths[2])
	}
}

// TestWriteDefaultConfigIfMissingCreatesConfig writes defaults only when absent.
func TestWriteDefaultConfigIfMissingCreatesConfig(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, ".gitm8rc")
	cfg := Config{
		DefaultBranch:             "main",
		Editor:                    "vi",
		Theme:                     "default",
		ConfirmDestructiveActions: true,
		FetchOnStartup:            false,
		ShowCommitGraph:           true,
		AIProvider:                "codex",
		OllamaURL:                 "http://localhost:11434",
		AIAvailable:               true,
		LogEnabled:                true,
		LogLevel:                  "INFO",
	}

	if err := writeDefaultConfigIfMissing(target, []string{target}, cfg); err != nil {
		t.Fatalf("writeDefaultConfigIfMissing() error = %v", err)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	got := string(data)
	for _, want := range []string{
		`export GITM8_DEFAULT_BRANCH="main"`,
		`export GITM8_EDITOR="vi"`,
		`export GITM8_AI_PROVIDER="codex"`,
		`export GITM8_OLLAMA_URL="http://localhost:11434"`,
		`export GITM8_LOG_FILE="$HOME/.gitm8/gitm8.log"`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("default config missing %q in:\n%s", want, got)
		}
	}
}

// TestDefaultConfigContentIncludesAIUnavailableComment explains missing CLIs on first run.
func TestDefaultConfigContentIncludesAIUnavailableComment(t *testing.T) {
	cfg := Config{
		DefaultBranch:             "main",
		Editor:                    "vi",
		Theme:                     "default",
		ConfirmDestructiveActions: true,
		FetchOnStartup:            false,
		ShowCommitGraph:           true,
		AIProvider:                "codex",
		AIAvailable:               false,
		AIUnavailableReason:       "install codex or claude to use AI commit/PR generation",
		LogEnabled:                true,
		LogLevel:                  "INFO",
	}

	got := defaultConfigContent(cfg)
	if !strings.Contains(got, "# AI unavailable: install codex or claude to use AI commit/PR generation") {
		t.Fatalf("default config missing AI unavailable comment:\n%s", got)
	}
}

// TestWriteDefaultConfigIfMissingDoesNotReplaceExistingConfig preserves user config.
func TestWriteDefaultConfigIfMissingDoesNotReplaceExistingConfig(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, ".gitm8rc")
	existing := filepath.Join(dir, "legacy")
	if err := os.WriteFile(existing, []byte("GITM8_THEME=rose\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if err := writeDefaultConfigIfMissing(target, []string{target, existing}, defaults()); err != nil {
		t.Fatalf("writeDefaultConfigIfMissing() error = %v", err)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("target should not be created when existing config is present, err=%v", err)
	}
}

// TestLoadLogSettingsFile applies only logging keys from a config file.
func TestLoadLogSettingsFile(t *testing.T) {
	t.Setenv("GITM8_LOG_ENABLED", "")
	t.Setenv("GITM8_LOG_LEVEL", "")
	t.Setenv("GITM8_LOG_FILE", "")

	path := filepath.Join(t.TempDir(), "gitm8rc")
	content := []byte("GITM8_LOG_ENABLED=false\nGITM8_LOG_LEVEL=debug\nGITM8_LOG_FILE=$HOME/.gitm8/test.log\nGITM8_THEME=rose\n")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cfg := defaults()
	loadLogSettingsFile(path, &cfg)
	if cfg.LogEnabled {
		t.Fatal("LogEnabled = true, want false")
	}
	if cfg.LogLevel != "DEBUG" {
		t.Fatalf("LogLevel = %q, want DEBUG", cfg.LogLevel)
	}
	if cfg.LogFile == "$HOME/.gitm8/test.log" {
		t.Fatal("LogFile should expand $HOME")
	}
	if cfg.Theme != "default" {
		t.Fatalf("Theme = %q, want unchanged default", cfg.Theme)
	}
}
