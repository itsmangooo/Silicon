package httpapi

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/itsmangooo/Silicon/backend/internal/agent"
	"github.com/itsmangooo/Silicon/backend/internal/store"
)

const agentInstallerURL = "https://raw.githubusercontent.com/itsmangooo/Silicon/main/services/agent/install.sh"

type enrollmentOutput struct {
	ExpiresAt time.Time `json:"expiresAt"`
	Command   string    `json:"command"`
}

func (a *API) issueEnrollment(ctx context.Context, organizationID, serverID, actorID, requestID uuid.UUID, ip net.IP) (enrollmentOutput, error) {
	token, tokenHash, err := randomCredential()
	if err != nil {
		return enrollmentOutput{}, err
	}
	expiresAt := time.Now().UTC().Add(a.cfg.AgentEnrollmentTTL)
	if err = a.repo.CreateEnrollmentToken(ctx, organizationID, serverID, actorID, tokenHash, expiresAt, requestID, ip); err != nil {
		return enrollmentOutput{}, err
	}
	command := "curl -fsSL " + shellQuote(a.cfg.PublicURL+"/api/v1/agent/install.sh") + " | sudo bash -s -- --url " + shellQuote(a.cfg.PublicURL) + " --server " + shellQuote(serverID.String()) + " --token " + shellQuote(token)
	if a.cfg.AgentAllowInsecure {
		command += " --allow-insecure"
	}
	return enrollmentOutput{ExpiresAt: expiresAt, Command: command}, nil
}

func (a *API) getServer(w http.ResponseWriter, r *http.Request) {
	item, err := a.repo.ServerByID(r.Context(), pathUUID(r, "organizationID"), pathUUID(r, "serverID"))
	if err != nil {
		a.persistenceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (a *API) createServerEnrollment(w http.ResponseWriter, r *http.Request) {
	output, err := a.issueEnrollment(r.Context(), pathUUID(r, "organizationID"), pathUUID(r, "serverID"), currentUser(r.Context()).ID, requestID(r.Context()), clientIP(r))
	if err != nil {
		a.persistenceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, output)
}

func (a *API) revokeServerAgent(w http.ResponseWriter, r *http.Request) {
	organizationID, serverID := pathUUID(r, "organizationID"), pathUUID(r, "serverID")
	err := a.repo.RevokeAgent(r.Context(), organizationID, serverID, currentUser(r.Context()).ID, requestID(r.Context()), clientIP(r))
	if err != nil {
		a.persistenceError(w, err)
		return
	}
	if a.agents != nil {
		a.agents.Disconnect(serverID)
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) setApplicationServer(w http.ResponseWriter, r *http.Request) {
	var input struct {
		ServerID *uuid.UUID `json:"serverId"`
	}
	if !decode(w, r, &input) {
		return
	}
	item, err := a.repo.SetApplicationServer(r.Context(), pathUUID(r, "organizationID"), pathUUID(r, "applicationID"), currentUser(r.Context()).ID, input.ServerID, requestID(r.Context()), clientIP(r))
	if err != nil {
		a.persistenceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (a *API) agentInstaller(w http.ResponseWriter, r *http.Request) {
	path := filepath.Join(a.cfg.AgentArtifactDirectory, "install.sh")
	if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() {
		w.Header().Set("Content-Type", "text/x-shellscript; charset=utf-8")
		http.ServeFile(w, r, path)
		return
	}
	http.Redirect(w, r, agentInstallerURL, http.StatusTemporaryRedirect)
}

func (a *API) downloadAgent(w http.ResponseWriter, r *http.Request) {
	path, ok := a.agentArtifactPath(r.PathValue("arch"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", `attachment; filename="silicon-agent-linux-`+r.PathValue("arch")+`"`)
	http.ServeFile(w, r, path)
}

func (a *API) downloadAgentChecksum(w http.ResponseWriter, r *http.Request) {
	path, ok := a.agentArtifactPath(r.PathValue("arch"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	body, err := os.ReadFile(path + ".sha256")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write(body)
}

func (a *API) agentArtifactPath(architecture string) (string, bool) {
	if architecture != "amd64" && architecture != "arm64" {
		return "", false
	}
	path := filepath.Join(a.cfg.AgentArtifactDirectory, "silicon-agent-linux-"+architecture)
	if info, err := os.Stat(path); err != nil || !info.Mode().IsRegular() {
		return "", false
	}
	return path, true
}

func (a *API) enrollAgent(w http.ResponseWriter, r *http.Request) {
	if !a.agentTransportAllowed(r) {
		writeError(w, http.StatusUpgradeRequired, "tls_required", "Agent enrollment requires HTTPS.")
		return
	}
	var input struct {
		ServerID        uuid.UUID `json:"serverId"`
		Token           string    `json:"token"`
		ProtocolVersion int       `json:"protocolVersion"`
		AgentVersion    string    `json:"agentVersion"`
		Capabilities    []string  `json:"capabilities"`
	}
	if !decode(w, r, &input) {
		return
	}
	if input.ServerID == uuid.Nil || len(input.Token) < 32 || input.ProtocolVersion < 1 || len(input.AgentVersion) > 100 {
		validation(w, "A valid server, enrollment token, protocol, and Agent version are required.")
		return
	}
	credential, credentialHash, err := randomCredential()
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	tokenHash := sha256.Sum256([]byte(input.Token))
	compatibility := agent.Compatibility(input.ProtocolVersion, input.AgentVersion, a.cfg.AgentExpectedVersion)
	identity, err := a.repo.EnrollAgent(r.Context(), input.ServerID, tokenHash[:], credentialHash, input.ProtocolVersion, strings.TrimSpace(input.AgentVersion), compatibility, input.Capabilities)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusUnauthorized, "invalid_enrollment", "The enrollment token is invalid, expired, or already used.")
			return
		}
		a.serverError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"agentId": identity.ID, "serverId": identity.ServerID, "organizationId": identity.OrganizationID, "credential": credential, "protocolVersion": agent.ProtocolVersion})
}

func (a *API) connectAgent(w http.ResponseWriter, r *http.Request) {
	if !a.agentTransportAllowed(r) {
		writeError(w, http.StatusUpgradeRequired, "tls_required", "Agent connections require HTTPS.")
		return
	}
	if a.agents == nil {
		writeError(w, http.StatusServiceUnavailable, "agent_unavailable", "Agent transport is unavailable.")
		return
	}
	identity, err := a.authenticateAgent(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid_agent", "Agent authentication failed.")
		return
	}
	_ = a.agents.Serve(w, r, identity.ID, identity.ServerID, identity.Capabilities)
}

func (a *API) checkAgent(w http.ResponseWriter, r *http.Request) {
	if !a.agentTransportAllowed(r) {
		writeError(w, http.StatusUpgradeRequired, "tls_required", "Agent checks require HTTPS.")
		return
	}
	identity, err := a.authenticateAgent(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid_agent", "Agent authentication failed.")
		return
	}
	connected := a.agents != nil && a.agents.Connected(identity.ServerID)
	if !connected {
		writeError(w, http.StatusServiceUnavailable, "agent_disconnected", "Agent has not established its command connection.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"connected": true, "serverId": identity.ServerID})
}

func (a *API) authenticateAgent(r *http.Request) (store.AgentIdentity, error) {
	agentID, err := uuid.Parse(r.Header.Get("X-Silicon-Agent-ID"))
	credential := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
	if err != nil || credential == "" || credential == r.Header.Get("Authorization") {
		return store.AgentIdentity{}, store.ErrNotFound
	}
	hash := sha256.Sum256([]byte(credential))
	return a.repo.AuthenticateAgent(r.Context(), agentID, hash[:])
}

func (a *API) agentTransportAllowed(r *http.Request) bool {
	return a.cfg.AgentAllowInsecure || r.TLS != nil || a.cfg.TrustForwardedProto && strings.EqualFold(strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-Proto"), ",")[0]), "https")
}

func randomCredential() (string, []byte, error) {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", nil, err
	}
	token := base64.RawURLEncoding.EncodeToString(value)
	hash := sha256.Sum256([]byte(token))
	return token, hash[:], nil
}

func shellQuote(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'" }
