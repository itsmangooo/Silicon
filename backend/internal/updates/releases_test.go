package updates

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestVersionComparison(t *testing.T) {
	tests := []struct {
		left, right string
		want        int
	}{
		{"v0.4.1", "v0.4.2", -1},
		{"v0.5.0", "v0.4.9", 1},
		{"v1.0.0", "v1.0.0", 0},
	}
	for _, test := range tests {
		left, err := ParseVersion(test.left)
		if err != nil {
			t.Fatal(err)
		}
		right, err := ParseVersion(test.right)
		if err != nil {
			t.Fatal(err)
		}
		if got := Compare(left, right); got != test.want {
			t.Fatalf("Compare(%s,%s)=%d want %d", test.left, test.right, got, test.want)
		}
	}
	for _, invalid := range []string{"main", "0.4.2", "v0.4", "v0.4.2-beta.1", "v01.2.3", "latest", "v999999999999999999999.1.1"} {
		if _, err := ParseVersion(invalid); err == nil {
			t.Fatalf("ParseVersion(%q) accepted malformed tag", invalid)
		}
	}
}

func TestGitHubReleaseDiscoveryIgnoresUnstableAndOlderTags(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/repos/itsmangooo/Silicon/releases":
			fmt.Fprint(w, `[
                  {"tag_name":"main","name":"Main","body":"ignore","draft":false,"prerelease":false},
                  {"tag_name":"v0.5.0-beta.1","name":"Beta","draft":false,"prerelease":true},
                  {"tag_name":"v0.4.1","name":"Old","draft":false,"prerelease":false},
                  {"tag_name":"v0.4.2","name":"Stable","body":"Security fixes","html_url":"https://example.test/v0.4.2","published_at":"2026-09-28T10:00:00Z","draft":false,"prerelease":false}
                ]`)
		case "/repos/itsmangooo/Silicon/releases/tags/v0.4.2":
			fmt.Fprint(w, `{"tag_name":"v0.4.2","name":"Stable","body":"Security fixes","draft":false,"prerelease":false}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	source := GitHubSource{Client: server.Client(), APIBaseURL: server.URL}
	checker := Checker{Source: source, CurrentVersion: "v0.4.1", CommitSHA: "abc123", CacheTTL: time.Hour}
	status, err := checker.Check(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if !status.UpdateAvailable || status.Latest == nil || status.Latest.TagName != "v0.4.2" {
		t.Fatalf("unexpected release status: %#v", status)
	}
	release, err := checker.VerifyTarget(context.Background(), "v0.4.2")
	if err != nil || release.TagName != "v0.4.2" {
		t.Fatalf("verify exact tag: release=%#v err=%v", release, err)
	}
	if _, err = checker.VerifyTarget(context.Background(), "main"); err == nil {
		t.Fatal("main was accepted as an update target")
	}
	if _, err = checker.VerifyTarget(context.Background(), "v0.4.1"); err == nil {
		t.Fatal("installed version was accepted as an update target")
	}
}

type countingSource struct{ calls int }

func (source *countingSource) LatestStable(context.Context) (Release, error) {
	source.calls++
	return Release{TagName: "v0.4.2"}, nil
}
func (*countingSource) Release(context.Context, string) (Release, error) { return Release{}, nil }

func TestReleaseChecksAreCached(t *testing.T) {
	source := &countingSource{}
	checker := Checker{Source: source, CurrentVersion: "v0.4.1", CacheTTL: time.Hour}
	if _, err := checker.Check(context.Background(), false); err != nil {
		t.Fatal(err)
	}
	if _, err := checker.Check(context.Background(), false); err != nil {
		t.Fatal(err)
	}
	if source.calls != 1 {
		t.Fatalf("release source called %d times, want 1", source.calls)
	}
	if _, err := checker.Check(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	if source.calls != 2 {
		t.Fatalf("forced release source calls=%d want 2", source.calls)
	}
}
