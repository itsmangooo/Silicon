package httpapi

import (
	"net/http"

	runtimeprovider "github.com/itsmangooo/Silicon/backend/internal/providers/runtime"
)

func (a *API) getApplicationRuntimeSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := a.repo.ApplicationRuntimeSettings(r.Context(), pathUUID(r, "organizationID"), pathUUID(r, "applicationID"))
	if err != nil {
		a.persistenceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

func (a *API) putApplicationRuntimeSettings(w http.ResponseWriter, r *http.Request) {
	var settings runtimeprovider.Settings
	if !decode(w, r, &settings) {
		return
	}
	if err := settings.Validate(); err != nil {
		validation(w, err.Error())
		return
	}
	err := a.repo.SetApplicationRuntimeSettings(r.Context(), pathUUID(r, "organizationID"), pathUUID(r, "applicationID"),
		currentUser(r.Context()).ID, settings, requestID(r.Context()), clientIP(r))
	if err != nil {
		a.persistenceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, settings)
}
