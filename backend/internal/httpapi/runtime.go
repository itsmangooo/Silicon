package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	runtimeprovider "github.com/itsmangooo/Silicon/backend/internal/providers/runtime"
	secretprovider "github.com/itsmangooo/Silicon/backend/internal/providers/secrets"
	"github.com/itsmangooo/Silicon/backend/internal/store"
)

var environmentNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func (a *API) listEnvironmentVariables(w http.ResponseWriter, r *http.Request) {
	items, err := a.repo.ListEnvironmentVariables(r.Context(), pathUUID(r, "organizationID"), pathUUID(r, "applicationID"))
	if err != nil {
		a.persistenceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"variables": items})
}

func (a *API) replaceEnvironmentVariables(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Variables []struct {
			Name  string `json:"name"`
			Value string `json:"value"`
		} `json:"variables"`
	}
	if !decode(w, r, &input) {
		return
	}
	if len(input.Variables) > 200 {
		validation(w, "At most 200 environment variables are allowed.")
		return
	}
	values := make(map[string]string, len(input.Variables))
	for _, variable := range input.Variables {
		variable.Name = strings.TrimSpace(variable.Name)
		if !environmentNamePattern.MatchString(variable.Name) || len(variable.Name) > 128 || len(variable.Value) > 32768 || strings.ContainsRune(variable.Value, 0) {
			validation(w, "Environment variable names or values are invalid.")
			return
		}
		if _, exists := values[variable.Name]; exists {
			validation(w, "Environment variable names must be unique.")
			return
		}
		values[variable.Name] = variable.Value
	}
	if err := a.repo.ReplaceEnvironmentVariables(r.Context(), pathUUID(r, "organizationID"), pathUUID(r, "applicationID"), currentUser(r.Context()).ID, values, requestID(r.Context()), clientIP(r)); err != nil {
		a.persistenceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"updated": len(values)})
}

func (a *API) listSecrets(w http.ResponseWriter, r *http.Request) {
	items, err := a.repo.ListSecretMetadata(r.Context(), pathUUID(r, "organizationID"), pathUUID(r, "applicationID"))
	if err != nil {
		a.persistenceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"secrets": items})
}

func (a *API) putSecret(w http.ResponseWriter, r *http.Request) {
	if a.secrets == nil {
		writeError(w, http.StatusServiceUnavailable, "encryption_unavailable", "Secret encryption is not configured.")
		return
	}
	name := strings.TrimSpace(r.PathValue("secret"))
	if !environmentNamePattern.MatchString(name) || len(name) > 128 {
		validation(w, "Provide a valid secret name.")
		return
	}
	var input struct {
		Value string `json:"value"`
	}
	if !decode(w, r, &input) {
		return
	}
	if input.Value == "" || len(input.Value) > 65536 || strings.ContainsRune(input.Value, 0) {
		validation(w, "Secret values must contain between 1 and 65536 bytes.")
		return
	}
	organizationID, applicationID := pathUUID(r, "organizationID"), pathUUID(r, "applicationID")
	application, err := a.repo.ApplicationByID(r.Context(), organizationID, applicationID)
	if err != nil {
		a.persistenceError(w, err)
		return
	}
	value := []byte(input.Value)
	err = a.secrets.Store(r.Context(), secretprovider.Reference{OrganizationID: organizationID.String(), EnvironmentID: application.EnvironmentID.String(), ApplicationID: applicationID.String(), Name: name}, value)
	for index := range value {
		value[index] = 0
	}
	input.Value = ""
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	if err = a.repo.RecordAudit(r.Context(), organizationID, currentUser(r.Context()).ID, "secret.changed", "application", applicationID, requestID(r.Context()), map[string]any{"name": name}, clientIP(r)); err != nil {
		a.serverError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"name": name, "status": "stored"})
}

func (a *API) deleteSecret(w http.ResponseWriter, r *http.Request) {
	if a.secrets == nil {
		writeError(w, http.StatusServiceUnavailable, "encryption_unavailable", "Secret encryption is not configured.")
		return
	}
	organizationID, applicationID := pathUUID(r, "organizationID"), pathUUID(r, "applicationID")
	application, err := a.repo.ApplicationByID(r.Context(), organizationID, applicationID)
	if err != nil {
		a.persistenceError(w, err)
		return
	}
	secretID, err := uuid.Parse(r.PathValue("secret"))
	if err != nil {
		validation(w, "Provide a valid secret ID.")
		return
	}
	err = a.secrets.Delete(r.Context(), secretprovider.Reference{OrganizationID: organizationID.String(), EnvironmentID: application.EnvironmentID.String(), ApplicationID: applicationID.String(), SecretID: secretID.String(), Name: "deleted"})
	if err != nil {
		a.persistenceError(w, err)
		return
	}
	if err = a.repo.RecordAudit(r.Context(), organizationID, currentUser(r.Context()).ID, "secret.deleted", "secret", secretID, requestID(r.Context()), map[string]any{}, clientIP(r)); err != nil {
		a.serverError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) getRuntime(w http.ResponseWriter, r *http.Request) {
	organizationID, applicationID := pathUUID(r, "organizationID"), pathUUID(r, "applicationID")
	instance, err := a.repo.CurrentRuntimeInstance(r.Context(), organizationID, applicationID)
	if errors.Is(err, store.ErrNotFound) {
		writeJSON(w, http.StatusOK, map[string]any{"instance": nil, "providerAvailable": a.runtime != nil})
		return
	}
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	if a.runtime == nil || instance.RemovedAt != nil {
		writeJSON(w, http.StatusOK, map[string]any{"instance": instance, "providerAvailable": false})
		return
	}
	status, inspectErr := a.runtime.Status(r.Context(), instance.ExternalID)
	if inspectErr != nil {
		instance.State, instance.Health = "unknown", "unknown"
		writeJSON(w, http.StatusOK, map[string]any{"instance": instance, "providerAvailable": true, "stale": true, "providerError": "Live Docker runtime state is unavailable"})
		return
	}
	instance, err = a.repo.UpdateRuntimeInstance(r.Context(), organizationID, instance.ID, status)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"instance": instance, "providerAvailable": true})
}

func (a *API) runtimeAction(w http.ResponseWriter, r *http.Request) {
	if a.runtime == nil {
		writeError(w, http.StatusServiceUnavailable, "runtime_unavailable", "No Docker runtime provider is available.")
		return
	}
	action := r.PathValue("action")
	if action != "start" && action != "stop" && action != "restart" && action != "remove" {
		writeError(w, http.StatusNotFound, "not_found", "Runtime action not found.")
		return
	}
	organizationID, applicationID := pathUUID(r, "organizationID"), pathUUID(r, "applicationID")
	instance, err := a.repo.CurrentRuntimeInstance(r.Context(), organizationID, applicationID)
	if err != nil {
		a.persistenceError(w, err)
		return
	}
	switch action {
	case "start":
		err = a.runtime.Start(r.Context(), instance.ExternalID)
	case "stop":
		err = a.runtime.Stop(r.Context(), instance.ExternalID)
	case "restart":
		err = a.runtime.Restart(r.Context(), instance.ExternalID)
	case "remove":
		err = a.runtime.Remove(r.Context(), instance.ExternalID)
	}
	if err != nil {
		a.logger.Warn("runtime lifecycle action failed", "action", action, "organization_id", organizationID, "application_id", applicationID, "error", err)
		writeError(w, http.StatusBadGateway, "runtime_action_failed", "Docker could not complete the requested lifecycle action.")
		return
	}
	if action == "remove" {
		if err = a.repo.MarkRuntimeRemoved(r.Context(), organizationID, instance.ID); err != nil {
			a.serverError(w, r, err)
			return
		}
		instance.State, instance.Health = "removed", "unknown"
		now := time.Now().UTC()
		instance.RemovedAt = &now
	} else {
		status, statusErr := a.runtime.Status(r.Context(), instance.ExternalID)
		if statusErr != nil {
			writeError(w, http.StatusBadGateway, "runtime_inspection_failed", "The action completed, but Docker status could not be read.")
			return
		}
		instance, err = a.repo.UpdateRuntimeInstance(r.Context(), organizationID, instance.ID, status)
		if err != nil {
			a.serverError(w, r, err)
			return
		}
	}
	if err = a.repo.RecordAudit(r.Context(), organizationID, currentUser(r.Context()).ID, "runtime."+action, "runtime_instance", instance.ID, requestID(r.Context()), map[string]any{"applicationId": applicationID, "state": instance.State}, clientIP(r)); err != nil {
		a.serverError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, instance)
}

func (a *API) runtimeLogs(w http.ResponseWriter, r *http.Request) {
	if a.runtime == nil {
		writeError(w, http.StatusServiceUnavailable, "runtime_unavailable", "The local Docker runtime is not enabled.")
		return
	}
	organizationID, applicationID := pathUUID(r, "organizationID"), pathUUID(r, "applicationID")
	instance, err := a.repo.CurrentRuntimeInstance(r.Context(), organizationID, applicationID)
	if err != nil {
		a.persistenceError(w, err)
		return
	}
	tail := 200
	if value := r.URL.Query().Get("tail"); value != "" {
		tail, err = strconv.Atoi(value)
		if err != nil || tail < 0 || tail > 1000 {
			validation(w, "Log tail must be between 0 and 1000 lines.")
			return
		}
	}
	follow := r.URL.Query().Get("follow") == "true"
	request := runtimeprovider.LogRequest{Tail: tail, Follow: follow}
	if value := r.URL.Query().Get("since"); value != "" {
		request.Since, err = time.Parse(time.RFC3339, value)
		if err != nil {
			validation(w, "Log since must be an RFC3339 timestamp.")
			return
		}
	}
	ctx := r.Context()
	if follow {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, a.cfg.RuntimeLogFollowTimeout)
		defer cancel()
		_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(a.cfg.RuntimeLogFollowTimeout + 5*time.Second))
	}
	lines, err := a.runtime.Logs(ctx, instance.ExternalID, request)
	if err != nil {
		writeError(w, http.StatusBadGateway, "runtime_logs_failed", "Docker logs could not be read.")
		return
	}
	if !follow {
		items := make([]runtimeprovider.LogLine, 0, tail)
		for line := range lines {
			items = append(items, line)
		}
		writeJSON(w, http.StatusOK, map[string]any{"logs": items})
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "stream_unavailable", "Live log streaming is unavailable.")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()
	for line := range lines {
		payload, marshalErr := json.Marshal(line)
		if marshalErr != nil {
			continue
		}
		if _, err = fmt.Fprintf(w, "event: log\ndata: %s\n\n", payload); err != nil {
			return
		}
		flusher.Flush()
	}
}
