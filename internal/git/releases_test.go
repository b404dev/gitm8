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

func TestParseReleaseDetail(t *testing.T) {
	got, err := parseReleaseDetail(`{"tagName":"v2.0.0","name":"Two","publishedAt":"2026-08-23T10:00:00Z","author":{"login":"bill"},"body":"Changes","url":"https://example.test/release","targetCommitish":"main","assets":[{"name":"gitm8","size":2048,"contentType":"application/octet-stream","url":"https://example.test/asset"}]}`)
	if err != nil {
		t.Fatal(err)
	}
	if got.Tag != "v2.0.0" || got.Author != "bill" || got.Body != "Changes" || len(got.Assets) != 1 || got.Assets[0].Size != 2048 {
		t.Fatalf("parseReleaseDetail() = %#v", got)
	}
}
