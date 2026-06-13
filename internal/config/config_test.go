package config

import "testing"

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

// TestNormalizeCommitMessageProvider keeps unsupported values on the safe default.
func TestNormalizeCommitMessageProvider(t *testing.T) {
	cases := []struct {
		value string
		want  string
	}{
		{"codex", "codex"},
		{"claude", "claude"},
		{" Claude ", "claude"},
		{"unknown", "codex"},
		{"", "codex"},
	}
	for _, tc := range cases {
		if got := normalizeCommitMessageProvider(tc.value); got != tc.want {
			t.Errorf("normalizeCommitMessageProvider(%q) = %q, want %q", tc.value, got, tc.want)
		}
	}
}
