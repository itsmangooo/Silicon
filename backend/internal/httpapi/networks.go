package httpapi

import (
	"errors"
	"net/http"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/itsmangooo/Silicon/backend/internal/store"
	"github.com/jackc/pgx/v5/pgconn"
)

var internalHostnamePattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)*\.internal$`)

func (a *API) listNetworks(w http.ResponseWriter, r *http.Request) {
	items, err := a.repo.ListNetworks(r.Context(), pathUUID(r, "organizationID"))
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"networks": items})
}

func (a *API) createNetwork(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name        string    `json:"name"`
		CIDR        string    `json:"cidr"`
		HubServerID uuid.UUID `json:"hubServerId"`
		ListenPort  int       `json:"listenPort"`
	}
	if !decode(w, r, &input) {
		return
	}
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" || len(input.Name) > 120 {
		validation(w, "Network name is required and must not exceed 120 characters.")
		return
	}
	if input.HubServerID == uuid.Nil {
		validation(w, "Select a WireGuard hub server.")
		return
	}
	if input.ListenPort == 0 {
		input.ListenPort = 51820
	}
	organizationID := pathUUID(r, "organizationID")
	item, err := a.repo.CreateNetwork(r.Context(), organizationID, input.HubServerID, input.Name, strings.TrimSpace(input.CIDR), input.ListenPort)
	if err != nil {
		a.networkError(w, r, err)
		return
	}
	operation, err := a.repo.QueueNetworkReconcile(r.Context(), organizationID, item.ID, currentUser(r.Context()).ID)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	_ = a.repo.RecordOrganizationAudit(r.Context(), organizationID, currentUser(r.Context()).ID, "network.created", "network", &item.ID, requestID(r.Context()), map[string]any{"name": item.Name, "cidr": item.CIDR, "provider": "wireguard"}, clientIP(r))
	writeJSON(w, http.StatusCreated, map[string]any{"network": item, "operation": operation})
}

func (a *API) getNetwork(w http.ResponseWriter, r *http.Request) {
	detail, err := a.repo.NetworkDetail(r.Context(), pathUUID(r, "organizationID"), pathUUID(r, "networkID"))
	if err != nil {
		a.networkError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"network": detail})
}

func (a *API) deleteNetwork(w http.ResponseWriter, r *http.Request) {
	organizationID, networkID := pathUUID(r, "organizationID"), pathUUID(r, "networkID")
	operation, err := a.repo.QueueNetworkDelete(r.Context(), organizationID, networkID, currentUser(r.Context()).ID)
	if err != nil {
		a.networkError(w, r, err)
		return
	}
	_ = a.repo.RecordOrganizationAudit(r.Context(), organizationID, currentUser(r.Context()).ID, "network.deletion_requested", "network", &networkID, requestID(r.Context()), nil, clientIP(r))
	writeJSON(w, http.StatusAccepted, map[string]any{"operation": operation})
}

func (a *API) addNetworkMember(w http.ResponseWriter, r *http.Request) {
	var input struct {
		ServerID uuid.UUID `json:"serverId"`
	}
	if !decode(w, r, &input) {
		return
	}
	if input.ServerID == uuid.Nil {
		validation(w, "Select a server to add.")
		return
	}
	organizationID, networkID := pathUUID(r, "organizationID"), pathUUID(r, "networkID")
	item, err := a.repo.AddNetworkMember(r.Context(), organizationID, networkID, input.ServerID)
	if err != nil {
		a.networkError(w, r, err)
		return
	}
	operation, err := a.repo.QueueNetworkReconcile(r.Context(), organizationID, networkID, currentUser(r.Context()).ID)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	_ = a.repo.RecordOrganizationAudit(r.Context(), organizationID, currentUser(r.Context()).ID, "network.member_added", "network_member", &item.ID, requestID(r.Context()), map[string]any{"networkId": networkID, "serverId": item.ServerID, "address": item.Address}, clientIP(r))
	writeJSON(w, http.StatusCreated, map[string]any{"member": item, "operation": operation})
}

func (a *API) removeNetworkMember(w http.ResponseWriter, r *http.Request) {
	organizationID, networkID, memberID := pathUUID(r, "organizationID"), pathUUID(r, "networkID"), pathUUID(r, "memberID")
	err := a.repo.RemoveNetworkMember(r.Context(), organizationID, networkID, memberID)
	if err != nil {
		a.networkError(w, r, err)
		return
	}
	operation, err := a.repo.QueueNetworkReconcile(r.Context(), organizationID, networkID, currentUser(r.Context()).ID)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	_ = a.repo.RecordOrganizationAudit(r.Context(), organizationID, currentUser(r.Context()).ID, "network.member_removal_requested", "network_member", &memberID, requestID(r.Context()), map[string]any{"networkId": networkID}, clientIP(r))
	writeJSON(w, http.StatusAccepted, map[string]any{"operation": operation})
}

func (a *API) addNetworkService(w http.ResponseWriter, r *http.Request) {
	var input struct {
		ApplicationID uuid.UUID `json:"applicationId"`
		Hostname      string    `json:"hostname"`
		Protocol      string    `json:"protocol"`
		Port          int       `json:"port"`
	}
	if !decode(w, r, &input) {
		return
	}
	organizationID, networkID := pathUUID(r, "organizationID"), pathUUID(r, "networkID")
	input.Protocol = strings.ToLower(strings.TrimSpace(input.Protocol))
	if input.Protocol == "" {
		input.Protocol = "tcp"
	}
	if !oneOf(input.Protocol, "tcp", "udp") {
		validation(w, "Service protocol must be TCP or UDP.")
		return
	}
	if input.Port < 1 || input.Port > 65535 {
		validation(w, "Service port must be between 1 and 65535.")
		return
	}
	if input.ApplicationID == uuid.Nil {
		validation(w, "Select an application.")
		return
	}
	input.Hostname = strings.ToLower(strings.TrimSpace(input.Hostname))
	if input.Hostname == "" {
		app, err := a.repo.ApplicationByID(r.Context(), organizationID, input.ApplicationID)
		if err != nil {
			a.networkError(w, r, err)
			return
		}
		project, err := a.repo.GetProject(r.Context(), organizationID, app.ProjectID)
		if err != nil {
			a.networkError(w, r, err)
			return
		}
		environment, err := a.repo.GetEnvironment(r.Context(), organizationID, app.EnvironmentID)
		if err != nil {
			a.networkError(w, r, err)
			return
		}
		input.Hostname = cleanSlug(app.Name, app.Name) + "." + environment.Slug + "." + project.Slug + ".internal"
	}
	if len(input.Hostname) > 253 || !internalHostnamePattern.MatchString(input.Hostname) {
		validation(w, "Service hostname must be a valid name ending in .internal.")
		return
	}
	item, err := a.repo.AddNetworkService(r.Context(), organizationID, networkID, input.ApplicationID, input.Hostname, input.Protocol, input.Port)
	if err != nil {
		a.networkError(w, r, err)
		return
	}
	operation, err := a.repo.QueueNetworkReconcile(r.Context(), organizationID, networkID, currentUser(r.Context()).ID)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	_ = a.repo.RecordOrganizationAudit(r.Context(), organizationID, currentUser(r.Context()).ID, "network.service_attached", "network_service", &item.ID, requestID(r.Context()), map[string]any{"networkId": networkID, "applicationId": item.ApplicationID, "hostname": item.Hostname, "protocol": item.Protocol, "port": item.Port}, clientIP(r))
	writeJSON(w, http.StatusCreated, map[string]any{"service": item, "operation": operation})
}

func (a *API) removeNetworkService(w http.ResponseWriter, r *http.Request) {
	organizationID, networkID, serviceID := pathUUID(r, "organizationID"), pathUUID(r, "networkID"), pathUUID(r, "serviceID")
	if err := a.repo.RemoveNetworkService(r.Context(), organizationID, networkID, serviceID); err != nil {
		a.networkError(w, r, err)
		return
	}
	operation, err := a.repo.QueueNetworkReconcile(r.Context(), organizationID, networkID, currentUser(r.Context()).ID)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	_ = a.repo.RecordOrganizationAudit(r.Context(), organizationID, currentUser(r.Context()).ID, "network.service_detached", "network_service", &serviceID, requestID(r.Context()), map[string]any{"networkId": networkID}, clientIP(r))
	writeJSON(w, http.StatusAccepted, map[string]any{"operation": operation})
}

func (a *API) addNetworkPolicy(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name                 string    `json:"name"`
		SourceApplicationID  uuid.UUID `json:"sourceApplicationId"`
		DestinationServiceID uuid.UUID `json:"destinationServiceId"`
		Protocol             string    `json:"protocol"`
		Port                 int       `json:"port"`
		Action               string    `json:"action"`
	}
	if !decode(w, r, &input) {
		return
	}
	input.Name = strings.TrimSpace(input.Name)
	input.Protocol = strings.ToLower(strings.TrimSpace(input.Protocol))
	input.Action = strings.ToLower(strings.TrimSpace(input.Action))
	if input.Name == "" || len(input.Name) > 120 {
		validation(w, "Policy name is required and must not exceed 120 characters.")
		return
	}
	if input.SourceApplicationID == uuid.Nil || input.DestinationServiceID == uuid.Nil {
		validation(w, "Policy source application and destination service are required.")
		return
	}
	if !oneOf(input.Protocol, "tcp", "udp") || !oneOf(input.Action, "allow", "deny") || input.Port < 1 || input.Port > 65535 {
		validation(w, "Policy protocol, port, and action are invalid.")
		return
	}
	organizationID, networkID := pathUUID(r, "organizationID"), pathUUID(r, "networkID")
	item, err := a.repo.AddNetworkPolicy(r.Context(), organizationID, networkID, input.SourceApplicationID, input.DestinationServiceID, input.Name, input.Protocol, input.Action, input.Port)
	if err != nil {
		a.networkError(w, r, err)
		return
	}
	operation, err := a.repo.QueueNetworkReconcile(r.Context(), organizationID, networkID, currentUser(r.Context()).ID)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	_ = a.repo.RecordOrganizationAudit(r.Context(), organizationID, currentUser(r.Context()).ID, "network.policy_created", "network_policy", &item.ID, requestID(r.Context()), map[string]any{"networkId": networkID, "action": item.Action, "protocol": item.Protocol, "port": item.Port}, clientIP(r))
	writeJSON(w, http.StatusCreated, map[string]any{"policy": item, "operation": operation})
}

func (a *API) removeNetworkPolicy(w http.ResponseWriter, r *http.Request) {
	organizationID, networkID, policyID := pathUUID(r, "organizationID"), pathUUID(r, "networkID"), pathUUID(r, "policyID")
	if err := a.repo.RemoveNetworkPolicy(r.Context(), organizationID, networkID, policyID); err != nil {
		a.networkError(w, r, err)
		return
	}
	operation, err := a.repo.QueueNetworkReconcile(r.Context(), organizationID, networkID, currentUser(r.Context()).ID)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	_ = a.repo.RecordOrganizationAudit(r.Context(), organizationID, currentUser(r.Context()).ID, "network.policy_deleted", "network_policy", &policyID, requestID(r.Context()), map[string]any{"networkId": networkID}, clientIP(r))
	writeJSON(w, http.StatusAccepted, map[string]any{"operation": operation})
}

func (a *API) reconcileNetwork(w http.ResponseWriter, r *http.Request) {
	organizationID, networkID := pathUUID(r, "organizationID"), pathUUID(r, "networkID")
	operation, err := a.repo.QueueNetworkReconcile(r.Context(), organizationID, networkID, currentUser(r.Context()).ID)
	if err != nil {
		a.networkError(w, r, err)
		return
	}
	_ = a.repo.RecordOrganizationAudit(r.Context(), organizationID, currentUser(r.Context()).ID, "network.reconciliation_requested", "network", &networkID, requestID(r.Context()), nil, clientIP(r))
	writeJSON(w, http.StatusAccepted, map[string]any{"operation": operation})
}

func (a *API) networkError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "Resource not found.")
	case errors.Is(err, store.ErrNetworkHasServices):
		writeError(w, http.StatusConflict, "network_not_empty", "Detach all application services before deleting the network.")
	case errors.Is(err, store.ErrConflict):
		writeError(w, http.StatusConflict, "network_conflict", "The network member cannot be removed while it is the hub or still hosts attached services.")
	default:
		var databaseError *pgconn.PgError
		if errors.As(err, &databaseError) && (databaseError.Code == "23505" || databaseError.Code == "23514" || databaseError.Code == "23503") {
			writeError(w, http.StatusConflict, "network_conflict", "The requested network relationship conflicts with existing organization resources.")
			return
		}
		if strings.Contains(err.Error(), "CIDR") || strings.Contains(err.Error(), "network") || strings.Contains(err.Error(), "application") || strings.Contains(err.Error(), "policy") || strings.Contains(err.Error(), "WireGuard") || strings.Contains(err.Error(), "server") {
			validation(w, err.Error())
			return
		}
		a.serverError(w, r, err)
	}
}
