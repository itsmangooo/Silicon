package httpapi

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/itsmangooo/Silicon/backend/internal/auth"
	"github.com/itsmangooo/Silicon/backend/internal/mailservice"
	"github.com/itsmangooo/Silicon/backend/internal/store"
)

const genericResetResponse = "If an active account exists for that email, a password reset message will be sent."

type systemMailInput struct {
	Provider     string               `json:"provider"`
	FromName     string               `json:"fromName"`
	FromAddress  string               `json:"fromAddress"`
	ReplyTo      string               `json:"replyTo"`
	Settings     mailservice.Settings `json:"settings"`
	AccessKeyID  string               `json:"accessKeyId"`
	Credential   string               `json:"credential"`
	SessionToken string               `json:"sessionToken"`
}

func (a *API) getSystemMail(w http.ResponseWriter, r *http.Request) {
	configuration, err := a.repo.ActiveSystemMailConfiguration(r.Context())
	if errors.Is(err, store.ErrNotFound) {
		writeJSON(w, http.StatusOK, map[string]any{"configured": false})
		return
	}
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	configuration.EncryptedCredentials = nil
	writeJSON(w, http.StatusOK, map[string]any{"configured": true, "configuration": configuration})
}

func (a *API) putSystemMail(w http.ResponseWriter, r *http.Request) {
	if a.box == nil {
		writeError(w, http.StatusServiceUnavailable, "encryption_unavailable", "System email requires SILICON_ENCRYPTION_KEY.")
		return
	}
	var input systemMailInput
	if !decode(w, r, &input) {
		return
	}
	input.Provider = strings.ToLower(strings.TrimSpace(input.Provider))
	input.FromName = strings.TrimSpace(input.FromName)
	input.FromAddress = strings.ToLower(strings.TrimSpace(input.FromAddress))
	input.ReplyTo = strings.ToLower(strings.TrimSpace(input.ReplyTo))
	if input.Provider == "smtp" {
		input.Settings = mailservice.ApplySMTPPreset(input.Settings)
	}
	configurationID := uuid.New()
	credential := mailservice.Credentials{AccessKeyID: input.AccessKeyID, Secret: input.Credential, SessionToken: input.SessionToken}
	current, currentErr := a.repo.ActiveSystemMailConfiguration(r.Context())
	if currentErr != nil && !errors.Is(currentErr, store.ErrNotFound) {
		a.serverError(w, r, currentErr)
		return
	}
	var encrypted []byte
	if input.Credential == "" && currentErr == nil && current.Provider == input.Provider {
		configurationID = current.ID
		encrypted = current.EncryptedCredentials
		plaintext, openErr := a.box.Open(encrypted, "system-mail-config:"+configurationID.String())
		if openErr != nil {
			a.serverError(w, r, errors.New("stored system mail credential could not be decrypted"))
			return
		}
		defer wipe(plaintext)
		if parseErr := json.Unmarshal(plaintext, &credential); parseErr != nil {
			a.serverError(w, r, errors.New("stored system mail credential is invalid"))
			return
		}
	}
	configuration := mailservice.Configuration{Provider: input.Provider, FromName: input.FromName, FromAddress: input.FromAddress, ReplyTo: input.ReplyTo, Settings: input.Settings, Credentials: credential}
	if err := mailservice.Validate(configuration); err != nil {
		validation(w, err.Error()+".")
		return
	}
	settingsJSON, err := json.Marshal(input.Settings)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	if encrypted == nil || input.Credential != "" {
		credentialJSON, marshalErr := json.Marshal(credential)
		if marshalErr != nil {
			a.serverError(w, r, marshalErr)
			return
		}
		defer wipe(credentialJSON)
		encrypted, err = a.box.Seal(credentialJSON, "system-mail-config:"+configurationID.String())
		if err != nil {
			a.serverError(w, r, err)
			return
		}
	}
	user := currentUser(r.Context())
	saved, err := a.repo.SaveSystemMailConfiguration(r.Context(), store.SystemMailConfigurationInput{
		ID: configurationID, Provider: input.Provider, FromName: input.FromName, FromAddress: input.FromAddress, ReplyTo: input.ReplyTo,
		Settings: settingsJSON, EncryptedCredentials: encrypted, CreatedBy: user.ID,
	})
	if err != nil {
		a.persistenceError(w, err)
		return
	}
	_ = a.repo.RecordAuthAudit(r.Context(), &user.ID, "system.mail_configured", requestID(r.Context()), clientIP(r), map[string]any{"provider": input.Provider})
	saved.EncryptedCredentials = nil
	writeJSON(w, http.StatusOK, map[string]any{"configuration": saved})
}

func (a *API) testSystemMail(w http.ResponseWriter, r *http.Request) {
	if a.box == nil {
		writeError(w, http.StatusServiceUnavailable, "encryption_unavailable", "System email requires SILICON_ENCRYPTION_KEY.")
		return
	}
	var input struct {
		Recipient string `json:"recipient"`
	}
	if !decode(w, r, &input) {
		return
	}
	input.Recipient = strings.ToLower(strings.TrimSpace(input.Recipient))
	if !validEmail(input.Recipient) {
		validation(w, "Enter a valid test recipient.")
		return
	}
	configuration, err := a.repo.ActiveSystemMailConfiguration(r.Context())
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusConflict, "mail_not_configured", "Configure system email before sending a test message.")
		return
	}
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	deliveryID := uuid.New()
	message := mailservice.TestMessage(configuration.FromName, configuration.FromAddress, configuration.ReplyTo, input.Recipient)
	messageJSON, err := json.Marshal(message)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	defer wipe(messageJSON)
	encrypted, err := a.box.Seal(messageJSON, "mail-delivery:"+deliveryID.String())
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	if err = a.repo.QueueMailDelivery(r.Context(), deliveryID, configuration.ID, "system_test", input.Recipient, configuration.Provider, encrypted); err != nil {
		a.serverError(w, r, err)
		return
	}
	user := currentUser(r.Context())
	_ = a.repo.RecordAuthAudit(r.Context(), &user.ID, "system.mail_test_queued", requestID(r.Context()), clientIP(r), map[string]any{"provider": configuration.Provider})
	writeJSON(w, http.StatusAccepted, map[string]any{"deliveryId": deliveryID, "status": "queued"})
}

func (a *API) requestPasswordReset(w http.ResponseWriter, r *http.Request) {
	if !a.requestOriginAllowed(r) {
		writeError(w, http.StatusForbidden, "origin_failed", "The request origin could not be verified.")
		return
	}
	var input struct {
		Email string `json:"email"`
	}
	if !decode(w, r, &input) {
		return
	}
	email := strings.ToLower(strings.TrimSpace(input.Email))
	if !validEmail(email) {
		writeJSON(w, http.StatusAccepted, map[string]string{"message": genericResetResponse})
		return
	}
	if a.box == nil || len(a.cfg.EncryptionKey) == 0 {
		writeJSON(w, http.StatusAccepted, map[string]string{"message": genericResetResponse})
		return
	}
	allowed, err := a.repo.AllowPasswordResetRequest(r.Context(), a.privateHash("email", email), a.privateHash("ip", clientIP(r).String()))
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	if !allowed {
		writeJSON(w, http.StatusAccepted, map[string]string{"message": genericResetResponse})
		return
	}
	configuration, configErr := a.repo.ActiveSystemMailConfiguration(r.Context())
	user, userErr := a.repo.UserByEmail(r.Context(), email)
	if configErr != nil || userErr != nil || user.Status != "active" {
		writeJSON(w, http.StatusAccepted, map[string]string{"message": genericResetResponse})
		return
	}
	token, tokenHash, err := auth.NewToken()
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	deliveryID := uuid.New()
	resetURL := strings.TrimRight(a.cfg.PublicURL, "/") + "/reset-password?token=" + url.QueryEscape(token)
	message := mailservice.PasswordResetMessage(configuration.FromName, configuration.FromAddress, configuration.ReplyTo, user.Email, resetURL)
	messageJSON, err := json.Marshal(message)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	defer wipe(messageJSON)
	encrypted, err := a.box.Seal(messageJSON, "mail-delivery:"+deliveryID.String())
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	if err = a.repo.CreatePasswordResetDelivery(r.Context(), user.ID, tokenHash, a.privateHash("ip", clientIP(r).String()), time.Now().Add(30*time.Minute), deliveryID, configuration.ID, user.Email, configuration.Provider, encrypted); err != nil {
		a.serverError(w, r, err)
		return
	}
	_ = a.repo.RecordPasswordResetRequest(r.Context(), user.ID, requestID(r.Context()), clientIP(r))
	writeJSON(w, http.StatusAccepted, map[string]string{"message": genericResetResponse})
}

func (a *API) completePasswordReset(w http.ResponseWriter, r *http.Request) {
	if !a.requestOriginAllowed(r) {
		writeError(w, http.StatusForbidden, "origin_failed", "The request origin could not be verified.")
		return
	}
	var input struct {
		Token           string `json:"token"`
		Password        string `json:"password"`
		ConfirmPassword string `json:"confirmPassword"`
	}
	if !decode(w, r, &input) {
		return
	}
	if input.Token == "" || len(input.Token) > 256 || input.Password != input.ConfirmPassword {
		validation(w, "Reset link or password confirmation is invalid.")
		return
	}
	passwordHash, err := auth.HashPassword(input.Password)
	if err != nil {
		validation(w, err.Error()+".")
		return
	}
	_, err = a.repo.ConsumePasswordResetToken(r.Context(), auth.HashToken(input.Token), passwordHash, requestID(r.Context()), clientIP(r))
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusUnprocessableEntity, "invalid_reset_token", "Reset link is invalid or expired.")
		return
	}
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	a.clearCookies(w)
	writeJSON(w, http.StatusOK, map[string]string{"message": "Password updated. Sign in with your new password."})
}

func (a *API) privateHash(kind, value string) []byte {
	key := a.cfg.EncryptionKey
	if len(key) == 0 {
		return nil
	}
	hash := hmac.New(sha256.New, key)
	_, _ = hash.Write([]byte(kind + "\x00" + value))
	return hash.Sum(nil)
}

func wipe(value []byte) {
	for index := range value {
		value[index] = 0
	}
}
