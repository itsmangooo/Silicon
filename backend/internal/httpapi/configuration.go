package httpapi

import (
	"net/http"
	"strings"

	"github.com/google/uuid"
	appconfig "github.com/itsmangooo/Silicon/backend/internal/configuration"
	secretprovider "github.com/itsmangooo/Silicon/backend/internal/providers/secrets"
)

func variableValues(w http.ResponseWriter, r *http.Request) (map[string]string, bool) {
	var input struct {
		Variables []struct {
			Name  string `json:"name"`
			Value string `json:"value"`
		} `json:"variables"`
	}
	if !decode(w, r, &input) {
		return nil, false
	}
	if len(input.Variables) > 200 {
		validation(w, "At most 200 environment variables are allowed.")
		return nil, false
	}
	values := make(map[string]string, len(input.Variables))
	for _, variable := range input.Variables {
		variable.Name = strings.TrimSpace(variable.Name)
		if !environmentNamePattern.MatchString(variable.Name) || len(variable.Name) > 128 || len(variable.Value) > 32768 || strings.ContainsRune(variable.Value, 0) {
			validation(w, "Environment variable names or values are invalid.")
			return nil, false
		}
		if _, exists := values[variable.Name]; exists {
			validation(w, "Environment variable names must be unique.")
			return nil, false
		}
		values[variable.Name] = variable.Value
	}
	return values, true
}

func (a *API) listProjectVariables(w http.ResponseWriter, r *http.Request) {
	items, err := a.repo.ListProjectVariables(r.Context(), pathUUID(r, "organizationID"), pathUUID(r, "projectID"))
	if err != nil {
		a.persistenceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"variables": items})
}

func (a *API) replaceProjectVariables(w http.ResponseWriter, r *http.Request) {
	values, ok := variableValues(w, r)
	if !ok {
		return
	}
	err := a.repo.ReplaceProjectVariables(r.Context(), pathUUID(r, "organizationID"), pathUUID(r, "projectID"), currentUser(r.Context()).ID, values, requestID(r.Context()), clientIP(r))
	if err != nil {
		a.persistenceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"updated": len(values)})
}

func (a *API) listEnvironmentOverrideVariables(w http.ResponseWriter, r *http.Request) {
	items, err := a.repo.ListEnvironmentOverrideVariables(r.Context(), pathUUID(r, "organizationID"), pathUUID(r, "environmentID"))
	if err != nil {
		a.persistenceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"variables": items})
}

func (a *API) replaceEnvironmentOverrideVariables(w http.ResponseWriter, r *http.Request) {
	values, ok := variableValues(w, r)
	if !ok {
		return
	}
	err := a.repo.ReplaceEnvironmentOverrideVariables(r.Context(), pathUUID(r, "organizationID"), pathUUID(r, "environmentID"), currentUser(r.Context()).ID, values, requestID(r.Context()), clientIP(r))
	if err != nil {
		a.persistenceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"updated": len(values)})
}

func (a *API) parseDotEnv(w http.ResponseWriter, r *http.Request) {
	organizationID := pathUUID(r, "organizationID")
	var resourceErr error
	switch {
	case r.PathValue("projectID") != "":
		_, resourceErr = a.repo.GetProject(r.Context(), organizationID, pathUUID(r, "projectID"))
	case r.PathValue("environmentID") != "":
		_, resourceErr = a.repo.GetEnvironment(r.Context(), organizationID, pathUUID(r, "environmentID"))
	case r.PathValue("applicationID") != "":
		_, resourceErr = a.repo.ApplicationByID(r.Context(), organizationID, pathUUID(r, "applicationID"))
	}
	if resourceErr != nil {
		a.persistenceError(w, resourceErr)
		return
	}
	var input struct {
		Text string `json:"text"`
	}
	if !decode(w, r, &input) {
		return
	}
	items, err := appconfig.ParseDotEnv(input.Text)
	if err != nil {
		validation(w, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"variables": items})
}

func (a *API) getEffectiveConfiguration(w http.ResponseWriter, r *http.Request) {
	item, err := a.repo.ResolveApplicationConfiguration(r.Context(), pathUUID(r, "organizationID"), pathUUID(r, "applicationID"))
	if err != nil {
		a.persistenceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (a *API) listProjectSecrets(w http.ResponseWriter, r *http.Request) {
	items, err := a.repo.ListProjectSecretMetadata(r.Context(), pathUUID(r, "organizationID"), pathUUID(r, "projectID"))
	if err != nil {
		a.persistenceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"secrets": items})
}

func (a *API) listEnvironmentSecrets(w http.ResponseWriter, r *http.Request) {
	items, err := a.repo.ListEnvironmentSecretMetadata(r.Context(), pathUUID(r, "organizationID"), pathUUID(r, "environmentID"))
	if err != nil {
		a.persistenceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"secrets": items})
}

func secretValue(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	var input struct {
		Value string `json:"value"`
	}
	if !decode(w, r, &input) {
		return nil, false
	}
	if input.Value == "" || len(input.Value) > 65536 || strings.ContainsRune(input.Value, 0) {
		validation(w, "Secret values must contain between 1 and 65536 bytes.")
		return nil, false
	}
	return []byte(input.Value), true
}

func secretName(w http.ResponseWriter, r *http.Request) (string, bool) {
	name := strings.TrimSpace(r.PathValue("secret"))
	if !environmentNamePattern.MatchString(name) || len(name) > 128 {
		validation(w, "Provide a valid secret name.")
		return "", false
	}
	return name, true
}

func (a *API) putProjectSecret(w http.ResponseWriter, r *http.Request) {
	if a.secrets == nil {
		writeError(w, http.StatusServiceUnavailable, "encryption_unavailable", "Secret encryption is not configured.")
		return
	}
	name, ok := secretName(w, r)
	if !ok {
		return
	}
	value, ok := secretValue(w, r)
	if !ok {
		return
	}
	organizationID, projectID := pathUUID(r, "organizationID"), pathUUID(r, "projectID")
	err := a.secrets.Store(r.Context(), secretprovider.Reference{OrganizationID: organizationID.String(), ProjectID: projectID.String(), Name: name}, value)
	clear(value)
	if err != nil {
		a.persistenceError(w, err)
		return
	}
	if err = a.repo.RecordAudit(r.Context(), organizationID, currentUser(r.Context()).ID, "secret.changed", "project", projectID, requestID(r.Context()), map[string]any{"name": name}, clientIP(r)); err != nil {
		a.serverError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"name": name, "status": "stored"})
}

func (a *API) putEnvironmentSecret(w http.ResponseWriter, r *http.Request) {
	if a.secrets == nil {
		writeError(w, http.StatusServiceUnavailable, "encryption_unavailable", "Secret encryption is not configured.")
		return
	}
	name, ok := secretName(w, r)
	if !ok {
		return
	}
	value, ok := secretValue(w, r)
	if !ok {
		return
	}
	organizationID, environmentID := pathUUID(r, "organizationID"), pathUUID(r, "environmentID")
	environment, err := a.repo.GetEnvironment(r.Context(), organizationID, environmentID)
	if err != nil {
		a.persistenceError(w, err)
		return
	}
	err = a.secrets.Store(r.Context(), secretprovider.Reference{OrganizationID: organizationID.String(), ProjectID: environment.ProjectID.String(), EnvironmentID: environmentID.String(), Name: name}, value)
	clear(value)
	if err != nil {
		a.persistenceError(w, err)
		return
	}
	if err = a.repo.RecordAudit(r.Context(), organizationID, currentUser(r.Context()).ID, "secret.changed", "environment", environmentID, requestID(r.Context()), map[string]any{"name": name}, clientIP(r)); err != nil {
		a.serverError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"name": name, "status": "stored"})
}

func (a *API) deleteProjectSecret(w http.ResponseWriter, r *http.Request) {
	a.deleteScopedSecret(w, r, "project")
}

func (a *API) deleteEnvironmentSecret(w http.ResponseWriter, r *http.Request) {
	a.deleteScopedSecret(w, r, "environment")
}

func (a *API) deleteScopedSecret(w http.ResponseWriter, r *http.Request, scope string) {
	if a.secrets == nil {
		writeError(w, http.StatusServiceUnavailable, "encryption_unavailable", "Secret encryption is not configured.")
		return
	}
	secretID, err := uuid.Parse(r.PathValue("secret"))
	if err != nil {
		validation(w, "Provide a valid secret ID.")
		return
	}
	organizationID := pathUUID(r, "organizationID")
	reference := secretprovider.Reference{OrganizationID: organizationID.String(), SecretID: secretID.String(), Name: "deleted"}
	resourceID := uuid.Nil
	if scope == "project" {
		resourceID = pathUUID(r, "projectID")
		reference.ProjectID = resourceID.String()
	} else {
		resourceID = pathUUID(r, "environmentID")
		reference.EnvironmentID = resourceID.String()
	}
	if err = a.secrets.Delete(r.Context(), reference); err != nil {
		a.persistenceError(w, err)
		return
	}
	if err = a.repo.RecordAudit(r.Context(), organizationID, currentUser(r.Context()).ID, "secret.deleted", "secret", secretID, requestID(r.Context()), map[string]any{"scope": scope, "resourceId": resourceID}, clientIP(r)); err != nil {
		a.serverError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
