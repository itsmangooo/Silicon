package httpapi

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/itsmangooo/Silicon/backend/internal/buildinfo"
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
