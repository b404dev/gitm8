package logging

import (
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Level int

const (
	DEBUG Level = iota
	INFO
	WARN
	ERROR
)

const (
	maxLogSizeBytes = 100 * 1024 * 1024
	maxLogBackups   = 5
)

type Field struct {
	Key   string
	Value any
}

var state = struct {
	sync.Mutex
	enabled bool
	level   Level
	out     io.WriteCloser
}{
	enabled: false,
	level:   INFO,
}

func Configure(enabled bool, level string, path string) error {
	state.Lock()
	defer state.Unlock()

	if state.out != nil {
		_ = state.out.Close()
		state.out = nil
	}

	state.enabled = enabled
	state.level = parseLevel(level)
	if !enabled {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		state.enabled = false
		return err
	}
	if err := rotateIfNeeded(path, maxLogSizeBytes, maxLogBackups); err != nil {
		state.enabled = false
		return err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		state.enabled = false
		return err
	}
	state.out = file
	return nil
}

func rotateIfNeeded(path string, maxSize int64, backups int) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if info.Size() < maxSize {
		return nil
	}

	if backups < 1 {
		return os.Truncate(path, 0)
	}
	for i := backups; i >= 2; i-- {
		oldPath := rotatedPath(path, i-1)
		newPath := rotatedPath(path, i)
		if _, err := os.Stat(oldPath); err == nil {
			_ = os.Remove(newPath)
			if err := os.Rename(oldPath, newPath); err != nil {
				return err
			}
		}
	}

	firstBackup := rotatedPath(path, 1)
	_ = os.Remove(firstBackup)
	if err := gzipFile(path, firstBackup); err != nil {
		return err
	}
	return os.Remove(path)
}

func rotatedPath(path string, index int) string {
	return fmt.Sprintf("%s.%d.gz", path, index)
}

func gzipFile(src string, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer out.Close()

	gz := gzip.NewWriter(out)
	if _, err := io.Copy(gz, in); err != nil {
		_ = gz.Close()
		return err
	}
	return gz.Close()
}

func Close() {
	state.Lock()
	defer state.Unlock()
	if state.out != nil {
		_ = state.out.Close()
		state.out = nil
	}
	state.enabled = false
}

func Debug(component string, function string, event string, fields ...Field) {
	write(DEBUG, component, function, event, fields...)
}

func Info(component string, function string, event string, fields ...Field) {
	write(INFO, component, function, event, fields...)
}

func Warn(component string, function string, event string, fields ...Field) {
	write(WARN, component, function, event, fields...)
}

func Error(component string, function string, event string, fields ...Field) {
	write(ERROR, component, function, event, fields...)
}

// LogError logs an error object with context.
func LogError(component string, function string, event string, err error, fields ...Field) {
	fields = append(fields, Field{Key: "error", Value: err})
	write(ERROR, component, function, event, fields...)
}

func F(key string, value any) Field {
	return Field{Key: key, Value: value}
}

func Enabled() bool {
	state.Lock()
	defer state.Unlock()
	return state.enabled
}

func Format(t time.Time, level Level, component string, function string, event string, fields ...Field) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s %s %s %s %s", t.UTC().Format(time.RFC3339), levelString(level), component, function, event)
	for _, field := range fields {
		if field.Key == "" {
			continue
		}
		fmt.Fprintf(&b, " %s=%s", field.Key, formatValue(field.Key, field.Value))
	}
	return b.String()
}

func write(level Level, component string, function string, event string, fields ...Field) {
	state.Lock()
	defer state.Unlock()
	if !state.enabled || state.out == nil || level < state.level {
		return
	}
	_, _ = fmt.Fprintln(state.out, Format(time.Now(), level, component, function, event, fields...))
}

func parseLevel(level string) Level {
	switch strings.ToUpper(strings.TrimSpace(level)) {
	case "DEBUG":
		return DEBUG
	case "WARN", "WARNING":
		return WARN
	case "ERROR":
		return ERROR
	default:
		return INFO
	}
}

func levelString(level Level) string {
	switch level {
	case DEBUG:
		return "DEBUG"
	case WARN:
		return "WARN"
	case ERROR:
		return "ERROR"
	default:
		return "INFO"
	}
}

func formatValue(key string, value any) string {
	if isSecretKey(key) {
		return "[redacted]"
	}
	raw := fmt.Sprint(value)
	raw = strings.ReplaceAll(raw, "\n", "\\n")
	raw = strings.ReplaceAll(raw, "\r", "\\r")
	if raw == "" {
		return `""`
	}
	if strings.ContainsAny(raw, " \t\"=") {
		raw = strings.ReplaceAll(raw, `"`, `\"`)
		return `"` + raw + `"`
	}
	return raw
}

func isSecretKey(key string) bool {
	key = strings.ToLower(key)
	for _, part := range []string{"token", "secret", "password", "credential", "private_key"} {
		if strings.Contains(key, part) {
			return true
		}
	}
	return false
}
