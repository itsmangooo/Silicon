package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"image"
	_ "image/jpeg"
	"image/png"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/itsmangooo/Silicon/backend/internal/auth"
	mailprovider "github.com/itsmangooo/Silicon/backend/internal/providers/mail"
	"github.com/itsmangooo/Silicon/backend/internal/store"
)

func (a *API) accountProfile(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r.Context())
	appearance, avatar, background, err := a.repo.AccountAppearance(r.Context(), user.ID)
	if err != nil {
		a.persistenceError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{"user": user, "appearance": appearance, "hasAvatar": avatar, "hasBackground": background})
}

func (a *API) updateAccountProfile(w http.ResponseWriter, r *http.Request) {
	var input struct {
		DisplayName string `json:"displayName"`
	}
	if !decode(w, r, &input) {
		return
	}
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	if len(input.DisplayName) == 0 || len(input.DisplayName) > 120 {
		validation(w, "Display name must be between 1 and 120 bytes.")
		return
	}
	if err := a.repo.SetDisplayName(r.Context(), currentUser(r.Context()).ID, input.DisplayName); err != nil {
		a.persistenceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) updateAccountAppearance(w http.ResponseWriter, r *http.Request) {
	var input store.Appearance
	if !decode(w, r, &input) {
		return
	}
	if err := validateAppearance(input); err != nil {
		validation(w, err.Error())
		return
	}
	if err := a.repo.SetAccountAppearance(r.Context(), currentUser(r.Context()).ID, input); err != nil {
		a.persistenceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, input)
}

func (a *API) accountAvatar(w http.ResponseWriter, r *http.Request) {
	avatar, err := a.repo.AccountAvatar(r.Context(), currentUser(r.Context()).ID)
	if err != nil {
		a.persistenceError(w, err)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = w.Write(avatar)
}

func (a *API) updateAccountAvatar(w http.ResponseWriter, r *http.Request) {
	var avatar []byte
	if r.Method == http.MethodPut {
		r.Body = http.MaxBytesReader(w, r.Body, 512<<10)
		body, err := io.ReadAll(r.Body)
		if err != nil {
			writeError(w, http.StatusRequestEntityTooLarge, "avatar_too_large", "Upload an image up to 512 KiB.")
			return
		}
		imageInfo, format, err := image.DecodeConfig(bytes.NewReader(body))
		if err != nil || !oneOf(format, "png", "jpeg") || imageInfo.Width < 1 || imageInfo.Height < 1 || imageInfo.Width > 512 || imageInfo.Height > 512 {
			validation(w, "Upload a PNG or JPEG up to 512 × 512 pixels.")
			return
		}
		decoded, _, err := image.Decode(bytes.NewReader(body))
		if err != nil {
			validation(w, "The avatar image could not be decoded.")
			return
		}
		var clean bytes.Buffer
		if err := png.Encode(&clean, decoded); err != nil || clean.Len() > 512<<10 {
			validation(w, "The normalized avatar is too large. Choose a smaller image.")
			return
		}
		avatar = clean.Bytes()
	}
	if err := a.repo.SetAccountAvatar(r.Context(), currentUser(r.Context()).ID, avatar); err != nil {
		a.persistenceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) updateAccountPassword(w http.ResponseWriter, r *http.Request) {
	var input struct {
		CurrentPassword string `json:"currentPassword"`
		Password        string `json:"password"`
		ConfirmPassword string `json:"confirmPassword"`
	}
	if !decode(w, r, &input) {
		return
	}
	if input.Password != input.ConfirmPassword || len(input.CurrentPassword) > 1024 {
		validation(w, "Provide the current password and matching new passwords.")
		return
	}
	user := currentUser(r.Context())
	if !a.allowAccountCredentialAttempt(w, r, "password", user.ID.String()) {
		return
	}
	stored, err := a.repo.UserByEmail(r.Context(), user.Email)
	if err != nil || !auth.VerifyPassword(stored.PasswordHash, input.CurrentPassword) {
		writeError(w, http.StatusForbidden, "invalid_credentials", "The current password is incorrect.")
		return
	}
	hash, err := auth.HashPassword(input.Password)
	if err != nil {
		validation(w, err.Error())
		return
	}
	err = a.repo.ChangeAccountPassword(r.Context(), user.ID, stored.PasswordHash, hash, requestID(r.Context()), clientIP(r))
	if errors.Is(err, store.ErrConflict) {
		writeError(w, http.StatusConflict, "password_changed", "The password changed. Sign in and try again.")
		return
	}
	if err != nil {
		a.persistenceError(w, err)
		return
	}
	a.clearCookies(w)
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) allowAccountCredentialAttempt(w http.ResponseWriter, r *http.Request, action, userID string) bool {
	allowed, err := a.repo.AllowPasswordResetRequest(r.Context(), a.privateHash("account-"+action, userID), a.privateHash("account-"+action+"-ip", clientIP(r).String()))
	if err != nil {
		a.serverError(w, r, err)
		return false
	}
	if !allowed {
		writeError(w, http.StatusTooManyRequests, "rate_limited", "Too many attempts. Try again later.")
	}
	return allowed
}

func (a *API) requestAccountEmail(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Email           string `json:"email"`
		CurrentPassword string `json:"currentPassword"`
	}
	if !decode(w, r, &input) {
		return
	}
	user := currentUser(r.Context())
	email := strings.ToLower(strings.TrimSpace(input.Email))
	if !validEmail(email) || strings.EqualFold(email, user.Email) || len(input.CurrentPassword) > 1024 {
		validation(w, "Enter a different valid email address and the current password.")
		return
	}
	if !a.allowAccountCredentialAttempt(w, r, "email", user.ID.String()) {
		return
	}
	stored, err := a.repo.UserByEmail(r.Context(), user.Email)
	if err != nil || !auth.VerifyPassword(stored.PasswordHash, input.CurrentPassword) {
		writeError(w, http.StatusForbidden, "invalid_credentials", "The current password is incorrect.")
		return
	}
	configuration, err := a.repo.ActiveSystemMailConfiguration(r.Context())
	if errors.Is(err, store.ErrNotFound) || a.box == nil {
		writeError(w, http.StatusServiceUnavailable, "mail_unavailable", "Configure system email before changing your address.")
		return
	}
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	token, hash, err := auth.NewToken()
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	message, err := json.Marshal(mailprovider.Message{FromName: configuration.FromName, From: configuration.FromAddress,
		ReplyTo: configuration.ReplyTo, To: email, Subject: "Verify your Silicon email address",
		Text: "Enter this verification code in Silicon Account Settings. It expires in 30 minutes.\n\n" + token})
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	defer wipe(message)
	deliveryID := uuid.New()
	encrypted, err := a.box.Seal(message, "mail-delivery:"+deliveryID.String())
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	if err := a.repo.QueueAccountEmailChange(r.Context(), user.ID, deliveryID, configuration.ID, email, configuration.Provider, hash, encrypted, time.Now().Add(30*time.Minute)); err != nil {
		a.serverError(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "queued"})
}

func (a *API) verifyAccountEmail(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Token string `json:"token"`
	}
	if !decode(w, r, &input) {
		return
	}
	user := currentUser(r.Context())
	if !a.allowAccountCredentialAttempt(w, r, "verify-email", user.ID.String()) {
		return
	}
	err := a.repo.VerifyAccountEmail(r.Context(), user.ID, auth.HashToken(strings.TrimSpace(input.Token)), requestID(r.Context()), clientIP(r))
	if errors.Is(err, store.ErrNotFound) {
		validation(w, "The verification code is invalid or expired.")
		return
	}
	if err != nil {
		a.persistenceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
