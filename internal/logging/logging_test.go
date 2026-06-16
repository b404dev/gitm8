package logging

import (
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFormatUsesSyslogStyleFields(t *testing.T) {
	got := Format(time.Date(2026, 6, 13, 16, 30, 12, 0, time.UTC), INFO, "tui", "updateDashboardKey", "key_pressed", F("key", "r"), F("mode", "review"))
	want := "2026-06-13T16:30:12Z INFO tui updateDashboardKey key_pressed key=r mode=review"
	if got != want {
		t.Fatalf("Format() = %q, want %q", got, want)
	}
}

func TestRotateIfNeededCompressesAndKeepsBackups(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "gitm8.log")
	if err := os.WriteFile(path, []byte("current log"), 0o644); err != nil {
		t.Fatalf("WriteFile current error = %v", err)
	}
	for i := 1; i <= 5; i++ {
		if err := os.WriteFile(rotatedPath(path, i), []byte("old"), 0o644); err != nil {
			t.Fatalf("WriteFile backup error = %v", err)
		}
	}

	if err := rotateIfNeeded(path, 1, 5); err != nil {
		t.Fatalf("rotateIfNeeded() error = %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("active log should be removed after rotation, err=%v", err)
	}
	for i := 1; i <= 5; i++ {
		if _, err := os.Stat(rotatedPath(path, i)); err != nil {
			t.Fatalf("backup %d missing: %v", i, err)
		}
	}
	if _, err := os.Stat(rotatedPath(path, 6)); !os.IsNotExist(err) {
		t.Fatalf("backup 6 should not exist, err=%v", err)
	}

	got := readGzip(t, rotatedPath(path, 1))
	if got != "current log" {
		t.Fatalf("rotated backup = %q, want current log", got)
	}
}

func readGzip(t *testing.T, path string) string {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("Open gzip error = %v", err)
	}
	defer file.Close()
	gz, err := gzip.NewReader(file)
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	defer gz.Close()
	data, err := io.ReadAll(gz)
	if err != nil {
		t.Fatalf("ReadAll error = %v", err)
	}
	return string(data)
}

func TestFormatRedactsSecretFields(t *testing.T) {
	got := Format(time.Date(2026, 6, 13, 16, 30, 12, 0, time.UTC), INFO, "config", "applyEnv", "env_applied", F("github_token", "abc123"))
	if strings.Contains(got, "abc123") || !strings.Contains(got, "github_token=[redacted]") {
		t.Fatalf("Format() = %q, want redacted token", got)
	}
}

func TestConfigureFiltersByLevel(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gitm8.log")
	if err := Configure(true, "WARN", path); err != nil {
		t.Fatalf("Configure() error = %v", err)
	}
	defer Close()

	Info("tui", "test", "ignored")
	Error("tui", "test", "kept")
	Close()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	got := string(data)
	if strings.Contains(got, "ignored") {
		t.Fatalf("log contains filtered event: %q", got)
	}
	if !strings.Contains(got, "ERROR tui test kept") {
		t.Fatalf("log missing error event: %q", got)
	}
}
