package git

import (
	"reflect"
	"testing"
)

func TestParseReleases(t *testing.T) {
	got, err := parseReleases(`[{"tagName":"v1.2.0","name":"Summer","publishedAt":"2026-08-23T10:00:00Z","isDraft":false,"isPrerelease":true}]`)
	if err != nil {
		t.Fatal(err)
	}
	want := []Release{{Tag: "v1.2.0", Name: "Summer", Published: "2026-08-23T10:00:00Z", Prerelease: true}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseReleases() = %#v, want %#v", got, want)
	}
}

func TestReleaseCreateArgs(t *testing.T) {
	got := releaseCreateArgs("v1.2.0", "Summer", "Notes", true, true, true)
	want := []string{"release", "create", "v1.2.0", "--title", "Summer", "--notes", "Notes", "--draft", "--prerelease", "--generate-notes"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("releaseCreateArgs() = %#v, want %#v", got, want)
	}
}
