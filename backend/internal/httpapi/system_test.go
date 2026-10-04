package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/itsmangooo/Silicon/backend/internal/buildinfo"
	"github.com/itsmangooo/Silicon/backend/internal/config"
)

func TestSystemVersionReportsInjectedBuildMetadata(t *testing.T) {
	previousVersion, previousCommit, previousTime := buildinfo.Version, buildinfo.CommitSHA, buildinfo.BuildTime
	t.Cleanup(func() {
		buildinfo.Version, buildinfo.CommitSHA, buildinfo.BuildTime = previousVersion, previousCommit, previousTime
	})
	buildinfo.Version = "v0.1.0"
	buildinfo.CommitSHA = "0123456789abcdef0123456789abcdef01234567"
	buildinfo.BuildTime = "2026-10-01T12:00:00Z"

	recorder := httptest.NewRecorder()
	(&API{}).systemVersion(recorder, httptest.NewRequest("GET", "/api/v1/system/version", nil))
	if recorder.Code != 200 {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var response buildinfo.Info
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if response.Version != buildinfo.Version || response.CommitSHA != buildinfo.CommitSHA || response.BuildTime != buildinfo.BuildTime {
		t.Fatalf("version API lost build metadata: %#v", response)
	}
}

func TestSecureCookieClearingRetainsSecureAttribute(t *testing.T) {
	api := &API{cfg: config.Config{CookieName: "silicon_session", CookieSecure: true}}
	recorder := httptest.NewRecorder()
	api.clearCookies(recorder)
	values := recorder.Header().Values("Set-Cookie")
	if len(values) != 2 {
		t.Fatalf("cookies=%v", values)
	}
	for _, value := range values {
		if !strings.Contains(value, "; Secure") {
			t.Fatalf("cookie lost Secure attribute: %s", value)
		}
	}
}

func TestPublicHTTPSOriginAndForwardedProtocolValidation(t *testing.T) {
	api := &API{cfg: config.Config{PublicURL: "https://silicon.example.com", FrontendOrigin: "https://silicon.example.com", TrustForwardedProto: true}}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/system/public-access", nil)
	request.Header.Set("Origin", "https://silicon.example.com")
	request.Header.Set("X-Forwarded-Proto", "https")
	if !api.requestOriginAllowed(request) {
		t.Fatal("expected canonical HTTPS origin to pass")
	}
	request.Header.Set("X-Forwarded-Proto", "http")
	if api.requestOriginAllowed(request) {
		t.Fatal("accepted downgraded forwarded protocol")
	}
	request.Header.Set("X-Forwarded-Proto", "https")
	request.Header.Set("Origin", "https://attacker.example")
	if api.requestOriginAllowed(request) {
		t.Fatal("accepted unrelated origin")
	}
}
