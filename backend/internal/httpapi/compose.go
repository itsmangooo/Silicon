package httpapi

import (
	"net/http"

	"github.com/itsmangooo/Silicon/backend/internal/compose"
)

func (a *API) validateCompose(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Text string `json:"text"`
	}
	if !decodeJSON(w, r, &request, 4*compose.MaxBytes) {
		return
	}
	preview, err := compose.Validate(request.Text)
	if err != nil {
		validation(w, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, preview)
}
