package httpapi

import (
	"errors"
	"net/http"
	"time"

	"github.com/itsmangooo/Silicon/backend/internal/buildinfo"
	"github.com/itsmangooo/Silicon/backend/internal/store"
	"github.com/itsmangooo/Silicon/backend/internal/updates"
	"github.com/jackc/pgx/v5/pgconn"
)

func (a *API) systemVersion(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, buildinfo.Current())
}

func (a *API) getSystemUpdates(w http.ResponseWriter, r *http.Request) {
	a.writeSystemUpdateStatus(w, r, false)
}

func (a *API) checkSystemUpdates(w http.ResponseWriter, r *http.Request) {
	a.writeSystemUpdateStatus(w, r, true)
}

func (a *API) writeSystemUpdateStatus(w http.ResponseWriter, r *http.Request, force bool) {
	status, err := a.updates.Check(r.Context(), force)
	response := map[string]any{}
	if err != nil {
		status = updates.Status{CurrentVersion: a.updates.CurrentVersion, CommitSHA: a.updates.CommitSHA, BuildTime: a.updates.BuildTime, CheckedAt: time.Now().UTC()}
		response["releaseCheckError"] = "Silicon could not check GitHub Releases. The installed version remains unchanged."
	}
	response["version"] = status
	operation, err := a.repo.LatestSystemUpdate(r.Context())
	if err == nil {
		response["operation"] = operation
	} else if !errors.Is(err, store.ErrNotFound) {
		a.serverError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (a *API) createSystemUpdate(w http.ResponseWriter, r *http.Request) {
	var input struct {
		TargetVersion string `json:"targetVersion"`
	}
	if !decode(w, r, &input) {
		return
	}
	release, err := a.updates.VerifyTarget(r.Context(), input.TargetVersion)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "invalid_release", err.Error())
		return
	}
	user := currentUser(r.Context())
	operation, err := a.repo.CreateSystemUpdate(r.Context(), user.ID, a.updates.CurrentVersion, release.TagName, release.Notes)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			writeError(w, http.StatusConflict, "update_in_progress", "A Silicon update is already in progress.")
			return
		}
		a.persistenceError(w, err)
		return
	}
	_ = a.repo.RecordAuthAudit(r.Context(), &user.ID, "system.update_requested", requestID(r.Context()), clientIP(r), map[string]any{"targetVersion": release.TagName})
	writeJSON(w, http.StatusAccepted, operation)
}
