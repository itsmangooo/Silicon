package updates

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

var stableVersionPattern = regexp.MustCompile(`^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)

var (
	ErrInvalidVersion = errors.New("version must be a stable semantic tag such as v0.4.2")
	ErrNoRelease      = errors.New("no stable GitHub release is available")
)

type Version struct {
	Major int
	Minor int
	Patch int
}

func ParseVersion(value string) (Version, error) {
	match := stableVersionPattern.FindStringSubmatch(value)
	if match == nil {
		return Version{}, ErrInvalidVersion
	}
	major, majorErr := strconv.Atoi(match[1])
	minor, minorErr := strconv.Atoi(match[2])
	patch, patchErr := strconv.Atoi(match[3])
	if majorErr != nil || minorErr != nil || patchErr != nil {
		return Version{}, ErrInvalidVersion
	}
	return Version{Major: major, Minor: minor, Patch: patch}, nil
}

func Compare(left, right Version) int {
	for _, pair := range [][2]int{{left.Major, right.Major}, {left.Minor, right.Minor}, {left.Patch, right.Patch}} {
		if pair[0] < pair[1] {
			return -1
		}
		if pair[0] > pair[1] {
			return 1
		}
	}
	return 0
}

type Release struct {
	TagName     string    `json:"tagName"`
	Name        string    `json:"name"`
	Notes       string    `json:"notes"`
	HTMLURL     string    `json:"htmlUrl"`
	PublishedAt time.Time `json:"publishedAt"`
	Draft       bool      `json:"-"`
	Prerelease  bool      `json:"-"`
}

type Source interface {
	LatestStable(context.Context) (Release, error)
	Release(context.Context, string) (Release, error)
}

type GitHubSource struct {
	Client     *http.Client
	APIBaseURL string
	Repository string
}

type githubRelease struct {
	TagName     string    `json:"tag_name"`
	Name        string    `json:"name"`
	Body        string    `json:"body"`
	HTMLURL     string    `json:"html_url"`
	PublishedAt time.Time `json:"published_at"`
	Draft       bool      `json:"draft"`
	Prerelease  bool      `json:"prerelease"`
}

func (source GitHubSource) LatestStable(ctx context.Context) (Release, error) {
	var response []githubRelease
	if err := source.get(ctx, "/repos/"+source.repository()+"/releases?per_page=50", &response); err != nil {
		return Release{}, err
	}
	var latest Release
	var latestVersion Version
	found := false
	for _, candidate := range response {
		version, err := ParseVersion(candidate.TagName)
		if err != nil || candidate.Draft || candidate.Prerelease {
			continue
		}
		if !found || Compare(version, latestVersion) > 0 {
			latest = fromGitHubRelease(candidate)
			latestVersion = version
			found = true
		}
	}
	if !found {
		return Release{}, ErrNoRelease
	}
	return latest, nil
}

func (source GitHubSource) Release(ctx context.Context, tag string) (Release, error) {
	if _, err := ParseVersion(tag); err != nil {
		return Release{}, err
	}
	var response githubRelease
	if err := source.get(ctx, "/repos/"+source.repository()+"/releases/tags/"+url.PathEscape(tag), &response); err != nil {
		return Release{}, err
	}
	if response.TagName != tag || response.Draft || response.Prerelease {
		return Release{}, ErrNoRelease
	}
	if _, err := ParseVersion(response.TagName); err != nil {
		return Release{}, err
	}
	return fromGitHubRelease(response), nil
}

func (source GitHubSource) get(ctx context.Context, path string, destination any) error {
	base := strings.TrimRight(source.APIBaseURL, "/")
	if base == "" {
		base = "https://api.github.com"
	}
	client := source.Client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, base+path, nil)
	if err != nil {
		return err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", "Silicon-update-checker")
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("check GitHub releases: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		return ErrNoRelease
	}
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("check GitHub releases: unexpected status %d", response.StatusCode)
	}
	decoder := json.NewDecoder(io.LimitReader(response.Body, 2<<20))
	if err := decoder.Decode(destination); err != nil {
		return fmt.Errorf("decode GitHub releases: %w", err)
	}
	return nil
}

func (source GitHubSource) repository() string {
	if source.Repository == "" {
		return "itsmangooo/Silicon"
	}
	return strings.Trim(source.Repository, "/")
}

func fromGitHubRelease(value githubRelease) Release {
	name := strings.TrimSpace(value.Name)
	if name == "" {
		name = value.TagName
	}
	return Release{TagName: value.TagName, Name: name, Notes: value.Body, HTMLURL: value.HTMLURL, PublishedAt: value.PublishedAt, Draft: value.Draft, Prerelease: value.Prerelease}
}

type Status struct {
	CurrentVersion  string    `json:"currentVersion"`
	CommitSHA       string    `json:"commitSha"`
	BuildTime       string    `json:"buildTime"`
	Latest          *Release  `json:"latestRelease"`
	UpdateAvailable bool      `json:"updateAvailable"`
	CheckedAt       time.Time `json:"checkedAt"`
}

type Checker struct {
	Source         Source
	CurrentVersion string
	CommitSHA      string
	BuildTime      string
	CacheTTL       time.Duration

	mu         sync.Mutex
	cached     Status
	cacheError error
}

func (checker *Checker) Check(ctx context.Context, force bool) (Status, error) {
	checker.mu.Lock()
	defer checker.mu.Unlock()
	ttl := checker.CacheTTL
	if ttl <= 0 {
		ttl = 15 * time.Minute
	}
	if !force && !checker.cached.CheckedAt.IsZero() && time.Since(checker.cached.CheckedAt) < ttl {
		return checker.cached, checker.cacheError
	}
	latest, err := checker.Source.LatestStable(ctx)
	if err != nil {
		checker.cached = Status{CurrentVersion: checker.CurrentVersion, CommitSHA: checker.CommitSHA, BuildTime: checker.BuildTime, CheckedAt: time.Now().UTC()}
		checker.cacheError = err
		return checker.cached, err
	}
	status := Status{CurrentVersion: checker.CurrentVersion, CommitSHA: checker.CommitSHA, BuildTime: checker.BuildTime, Latest: &latest, CheckedAt: time.Now().UTC()}
	current, currentErr := ParseVersion(checker.CurrentVersion)
	available, latestErr := ParseVersion(latest.TagName)
	status.UpdateAvailable = currentErr == nil && latestErr == nil && Compare(available, current) > 0
	checker.cached = status
	checker.cacheError = nil
	return status, nil
}

func (checker *Checker) VerifyTarget(ctx context.Context, tag string) (Release, error) {
	target, err := ParseVersion(tag)
	if err != nil {
		return Release{}, err
	}
	current, err := ParseVersion(checker.CurrentVersion)
	if err != nil {
		return Release{}, errors.New("the installed build is not a tagged semantic release")
	}
	if Compare(target, current) <= 0 {
		return Release{}, errors.New("target release must be newer than the installed version")
	}
	release, err := checker.Source.Release(ctx, tag)
	if err != nil {
		return Release{}, err
	}
	if release.TagName != tag {
		return Release{}, ErrNoRelease
	}
	return release, nil
}
