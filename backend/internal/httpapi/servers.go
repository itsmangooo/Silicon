package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/itsmangooo/Silicon/backend/internal/cryptoenvelope"
	"github.com/itsmangooo/Silicon/backend/internal/providers/connection"
	"github.com/itsmangooo/Silicon/backend/internal/serverconnections"
	"golang.org/x/crypto/ssh"
)

type serverConnectionInput struct {
	ConnectionType     string `json:"connectionType"`
	Host               string `json:"host"`
	Port               int    `json:"port"`
	Username           string `json:"username"`
	PrivateKey         string `json:"privateKey"`
	HostKeyFingerprint string `json:"hostKeyFingerprint"`
	PublicAddress      string `json:"publicAddress"`
}

func (a *API) getServer(w http.ResponseWriter, r *http.Request) {
	item, err := a.repo.ServerByID(r.Context(), pathUUID(r, "organizationID"), pathUUID(r, "serverID"))
	if err != nil {
		a.persistenceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (a *API) updateServerConnection(w http.ResponseWriter, r *http.Request) {
	var input serverConnectionInput
	if !decode(w, r, &input) {
		return
	}
	input = normalizeConnectionInput(input)
	if message := validateConnectionInput(input, false); message != "" {
		validation(w, message)
		return
	}
	organizationID, serverID := pathUUID(r, "organizationID"), pathUUID(r, "serverID")
	encrypted, ok := a.encryptSSHKey(w, organizationID, serverID, input.PrivateKey)
	input.PrivateKey = ""
	if !ok {
		return
	}
	item, err := a.repo.UpdateServerConnection(r.Context(), organizationID, serverID, currentUser(r.Context()).ID, input.ConnectionType, input.Host, input.Port, input.Username, input.PublicAddress, encrypted, input.HostKeyFingerprint, input.ConnectionType == "local", requestID(r.Context()), clientIP(r))
	if err != nil {
		a.persistenceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (a *API) checkServerConnection(w http.ResponseWriter, r *http.Request) {
	organizationID, serverID := pathUUID(r, "organizationID"), pathUUID(r, "serverID")
	status, err := a.connections.Check(r.Context(), organizationID, serverID)
	if err != nil {
		var keyError *connection.HostKeyError
		if errors.As(err, &keyError) {
			code := "host_key_untrusted"
			message := "Review and explicitly trust the SSH host key before connecting."
			if keyError.Changed {
				code = "host_key_changed"
				message = "The SSH host key changed unexpectedly. Connection is blocked until it is explicitly re-trusted."
			}
			writeJSON(w, http.StatusConflict, map[string]any{"error": code, "message": message, "fingerprint": keyError.Fingerprint})
			return
		}
		switch {
		case errors.Is(err, connection.ErrNotConfigured):
			writeError(w, http.StatusUnprocessableEntity, "connection_not_configured", "Configure a local or SSH connection first.")
		case errors.Is(err, connection.ErrAuthenticationFailed):
			writeError(w, http.StatusUnprocessableEntity, "authentication_failed", "SSH authentication failed.")
		case errors.Is(err, connection.ErrDockerUnavailable):
			writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"error": "docker_unavailable", "message": "SSH is reachable, but Docker is unavailable.", "status": status})
		default:
			writeError(w, http.StatusBadGateway, "server_unreachable", "The server could not be reached.")
		}
		return
	}
	item, err := a.repo.ServerByID(r.Context(), organizationID, serverID)
	if err != nil {
		a.persistenceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"server": item, "status": status})
}

func (a *API) trustServerHostKey(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Fingerprint string `json:"fingerprint"`
	}
	if !decode(w, r, &input) {
		return
	}
	input.Fingerprint = strings.TrimSpace(input.Fingerprint)
	organizationID, serverID := pathUUID(r, "organizationID"), pathUUID(r, "serverID")
	if err := a.connections.VerifyHostKey(r.Context(), organizationID, serverID, input.Fingerprint); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "host_key_verification_failed", "The supplied fingerprint was not presented by the server with the configured credential.")
		return
	}
	if err := a.repo.TrustServerHostKey(r.Context(), organizationID, serverID, currentUser(r.Context()).ID, input.Fingerprint, requestID(r.Context()), clientIP(r)); err != nil {
		a.persistenceError(w, err)
		return
	}
	status, err := a.connections.Check(r.Context(), organizationID, serverID)
	if err != nil && !errors.Is(err, connection.ErrDockerUnavailable) {
		writeError(w, http.StatusBadGateway, "server_check_failed", "The host key was trusted, but the server check failed.")
		return
	}
	item, err := a.repo.ServerByID(r.Context(), organizationID, serverID)
	if err != nil {
		a.persistenceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"server": item, "status": status})
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

func normalizeConnectionInput(input serverConnectionInput) serverConnectionInput {
	input.ConnectionType = strings.ToLower(strings.TrimSpace(input.ConnectionType))
	input.Host = strings.TrimSpace(input.Host)
	input.Username = strings.TrimSpace(input.Username)
	input.HostKeyFingerprint = strings.TrimSpace(input.HostKeyFingerprint)
	input.PublicAddress = strings.TrimSpace(input.PublicAddress)
	if input.Port == 0 {
		input.Port = 22
	}
	if input.ConnectionType == "local" && input.Host == "" {
		input.Host = "localhost"
	}
	return input
}

func validateConnectionInput(input serverConnectionInput, requireKey bool) string {
	if input.ConnectionType != "local" && input.ConnectionType != "ssh" {
		return "Connection type must be local or ssh."
	}
	if len(input.Host) < 1 || len(input.Host) > 255 || strings.ContainsAny(input.Host, " \t\r\n\x00") {
		return "A valid host is required."
	}
	if input.ConnectionType == "ssh" {
		if input.Port < 1 || input.Port > 65535 || input.Username == "" || strings.ContainsAny(input.Username, " \t\r\n\x00") {
			return "SSH port and username are required."
		}
		if requireKey && strings.TrimSpace(input.PrivateKey) == "" {
			return "An SSH private key is required."
		}
		if input.HostKeyFingerprint != "" && !strings.HasPrefix(input.HostKeyFingerprint, "SHA256:") {
			return "Host-key fingerprints must use the SHA256 format."
		}
	}
	return ""
}

func (a *API) encryptSSHKey(w http.ResponseWriter, organizationID, serverID uuid.UUID, value string) ([]byte, bool) {
	if strings.TrimSpace(value) == "" {
		return nil, true
	}
	if a.box == nil {
		writeError(w, http.StatusServiceUnavailable, "encryption_unavailable", cryptoenvelope.ErrKeyUnavailable.Error())
		return nil, false
	}
	plaintext := []byte(value)
	defer func() {
		for index := range plaintext {
			plaintext[index] = 0
		}
	}()
	if _, err := ssh.ParsePrivateKey(plaintext); err != nil {
		validation(w, "The SSH private key is invalid or requires an unsupported passphrase.")
		return nil, false
	}
	encrypted, err := a.box.Seal(plaintext, serverconnections.CredentialContext(organizationID, serverID))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "encryption_failed", "The SSH credential could not be encrypted.")
		return nil, false
	}
	return encrypted, true
}
