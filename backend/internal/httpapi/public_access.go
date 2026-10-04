package httpapi

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/itsmangooo/Silicon/backend/internal/publicaccess"
	"github.com/itsmangooo/Silicon/backend/internal/store"
	"github.com/jackc/pgx/v5/pgconn"
)

func (a *API) getSystemPublicAccess(w http.ResponseWriter, r *http.Request) {
	var active any
	configured, err := a.repo.SystemPublicAccess(r.Context())
	if err == nil {
		active = map[string]any{
			"organizationId": configured.OrganizationID, "integrationId": configured.IntegrationID,
			"zoneId": configured.ZoneID, "tunnelId": configured.TunnelID, "hostname": configured.Hostname,
			"localOrigin": configured.LocalOrigin, "publicUrl": "https://" + configured.Hostname,
			"localAccessUrl": configured.PreviousPublicURL, "createdAt": configured.CreatedAt, "updatedAt": configured.UpdatedAt,
		}
	} else if !errors.Is(err, store.ErrNotFound) {
		a.serverError(w, r, err)
		return
	}
	var operation any
	latest, err := a.repo.LatestSystemPublicAccessOperation(r.Context())
	if err == nil {
		operation = latest
	} else if !errors.Is(err, store.ErrNotFound) {
		a.serverError(w, r, err)
		return
	}
	options, err := a.repo.PublicAccessConnectionOptions(r.Context(), currentUser(r.Context()).ID)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	port := a.cfg.HTTPPort
	if port == 0 {
		port = 80
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"active": active, "operation": operation, "connections": options,
		"installation": map[string]any{"publicUrl": a.cfg.PublicURL, "httpPort": port, "bindAddress": a.cfg.BindAddress, "localOrigin": fmt.Sprintf("http://127.0.0.1:%d", port)},
	})
}

func (a *API) configureSystemPublicAccess(w http.ResponseWriter, r *http.Request) {
	var input struct {
		OrganizationID string `json:"organizationId"`
		IntegrationID  string `json:"integrationId"`
		ZoneID         string `json:"zoneId"`
		TunnelID       string `json:"tunnelId"`
		Hostname       string `json:"hostname"`
	}
	if !decode(w, r, &input) {
		return
	}
	orgID, err1 := uuid.Parse(input.OrganizationID)
	integrationID, err2 := uuid.Parse(input.IntegrationID)
	zoneID, err3 := uuid.Parse(input.ZoneID)
	tunnelID, err4 := uuid.Parse(input.TunnelID)
	hostname, hostErr := publicaccess.NormalizeHostname(input.Hostname)
	if err1 != nil || err2 != nil || err3 != nil || err4 != nil || hostErr != nil {
		validation(w, "Select a Cloudflare connection, zone, local tunnel, and enter a valid hostname.")
		return
	}
	resources, err := a.repo.PublicAccessResources(r.Context(), orgID, integrationID, zoneID, tunnelID)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "Selected Cloudflare resources were not found or cannot host Silicon.")
		return
	}
	if !publicaccess.HostnameInZone(hostname, resources.Zone.Name) {
		validation(w, "Custom domain must belong to the selected Cloudflare zone.")
		return
	}
	item, err := a.repo.CreateSystemPublicAccessOperation(r.Context(), currentUser(r.Context()).ID, orgID, integrationID, zoneID, tunnelID, hostname, requestID(r.Context()), clientIP(r))
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			writeError(w, http.StatusConflict, "public_access_in_progress", "A public access operation is already in progress.")
			return
		}
		a.persistenceError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, item)
}

func (a *API) disableSystemPublicAccess(w http.ResponseWriter, r *http.Request) {
	item, err := a.repo.CreateDisableSystemPublicAccessOperation(r.Context(), currentUser(r.Context()).ID, requestID(r.Context()), clientIP(r))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusConflict, "not_active", "Public access is not active.")
			return
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			writeError(w, http.StatusConflict, "public_access_in_progress", "A public access operation is already in progress.")
			return
		}
		a.persistenceError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, item)
}
