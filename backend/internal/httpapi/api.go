package httpapi

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/itsmangooo/Silicon/backend/internal/auth"
	"github.com/itsmangooo/Silicon/backend/internal/authorization"
	"github.com/itsmangooo/Silicon/backend/internal/config"
	"github.com/itsmangooo/Silicon/backend/internal/deployments"
	"github.com/itsmangooo/Silicon/backend/internal/store"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type API struct {
	cfg    config.Config
	repo   store.Repository
	logger *slog.Logger
}

type contextKey string

const (
	userKey      contextKey = "user"
	csrfHashKey  contextKey = "csrf_hash"
	requestIDKey contextKey = "request_id"
	roleKey      contextKey = "role"
)

var slugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

func New(cfg config.Config, pool *pgxpool.Pool, logger *slog.Logger) *API {
	return &API{cfg: cfg, repo: store.Repository{Pool: pool}, logger: logger}
}

func (a *API) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", a.health)
	mux.HandleFunc("POST /api/v1/auth/register", a.register)
	mux.HandleFunc("POST /api/v1/auth/login", a.login)
	mux.Handle("GET /api/v1/auth/session", a.auth(http.HandlerFunc(a.session)))
	mux.Handle("POST /api/v1/auth/logout", a.auth(http.HandlerFunc(a.logout)))
	mux.Handle("GET /api/v1/organizations", a.auth(http.HandlerFunc(a.listOrganizations)))
	mux.Handle("POST /api/v1/organizations", a.auth(http.HandlerFunc(a.createOrganization)))

	mux.Handle("GET /api/v1/organizations/{organizationID}/access", a.org(authorization.OrganizationRead, http.HandlerFunc(a.access)))
	mux.Handle("GET /api/v1/organizations/{organizationID}/members", a.org(authorization.MemberRead, http.HandlerFunc(a.listMembers)))
	mux.Handle("POST /api/v1/organizations/{organizationID}/members", a.org(authorization.MemberManage, http.HandlerFunc(a.addMember)))
	mux.Handle("GET /api/v1/organizations/{organizationID}/projects", a.org(authorization.ProjectRead, http.HandlerFunc(a.listProjects)))
	mux.Handle("POST /api/v1/organizations/{organizationID}/projects", a.org(authorization.ProjectCreate, http.HandlerFunc(a.createProject)))
	mux.Handle("GET /api/v1/organizations/{organizationID}/projects/{projectID}", a.org(authorization.ProjectRead, http.HandlerFunc(a.getProject)))
	mux.Handle("PUT /api/v1/organizations/{organizationID}/projects/{projectID}", a.org(authorization.ProjectUpdate, http.HandlerFunc(a.updateProject)))
	mux.Handle("DELETE /api/v1/organizations/{organizationID}/projects/{projectID}", a.org(authorization.ProjectDelete, http.HandlerFunc(a.deleteProject)))
	mux.Handle("GET /api/v1/organizations/{organizationID}/projects/{projectID}/environments", a.org(authorization.EnvironmentRead, http.HandlerFunc(a.listEnvironments)))
	mux.Handle("POST /api/v1/organizations/{organizationID}/projects/{projectID}/environments", a.org(authorization.EnvironmentCreate, http.HandlerFunc(a.createEnvironment)))
	mux.Handle("GET /api/v1/organizations/{organizationID}/applications", a.org(authorization.ApplicationRead, http.HandlerFunc(a.listAllApplications)))
	mux.Handle("GET /api/v1/organizations/{organizationID}/environments/{environmentID}/applications", a.org(authorization.ApplicationRead, http.HandlerFunc(a.listApplications)))
	mux.Handle("POST /api/v1/organizations/{organizationID}/environments/{environmentID}/applications", a.org(authorization.ApplicationCreate, http.HandlerFunc(a.createApplication)))
	mux.Handle("GET /api/v1/organizations/{organizationID}/deployments", a.org(authorization.DeploymentRead, http.HandlerFunc(a.listAllDeployments)))
	mux.Handle("GET /api/v1/organizations/{organizationID}/applications/{applicationID}/deployments", a.org(authorization.DeploymentRead, http.HandlerFunc(a.listDeployments)))
	mux.Handle("POST /api/v1/organizations/{organizationID}/applications/{applicationID}/deployments", a.org(authorization.DeploymentCreate, http.HandlerFunc(a.createDeployment)))
	mux.Handle("POST /api/v1/organizations/{organizationID}/deployments/{deploymentID}/transitions", a.org(authorization.DeploymentCreate, http.HandlerFunc(a.transitionDeployment)))
	mux.Handle("GET /api/v1/organizations/{organizationID}/servers", a.org(authorization.ServerRead, http.HandlerFunc(a.listServers)))
	mux.Handle("POST /api/v1/organizations/{organizationID}/servers", a.org(authorization.ServerManage, http.HandlerFunc(a.createServer)))
	mux.Handle("GET /api/v1/organizations/{organizationID}/identity-providers", a.org(authorization.IdentityProviderRead, http.HandlerFunc(a.listIdentityProviders)))
	mux.Handle("POST /api/v1/organizations/{organizationID}/identity-providers", a.org(authorization.IdentityProviderManage, http.HandlerFunc(a.createIdentityProvider)))
	mux.Handle("GET /api/v1/organizations/{organizationID}/audit-events", a.org(authorization.AuditRead, http.HandlerFunc(a.listAuditEvents)))

	return a.recover(a.security(a.cors(a.requestLog(mux))))
}

func (a *API) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *API) register(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Email       string `json:"email"`
		DisplayName string `json:"displayName"`
		Password    string `json:"password"`
	}
	if !decode(w, r, &input) {
		return
	}
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	if !validEmail(input.Email) || len(input.DisplayName) < 1 || len(input.DisplayName) > 120 {
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", "Enter a valid email and display name.")
		return
	}
	hash, err := auth.HashPassword(input.Password)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", err.Error())
		return
	}
	user, err := a.repo.CreateUser(r.Context(), input.Email, input.DisplayName, hash)
	if err != nil {
		a.persistenceError(w, err)
		return
	}
	requestID := requestID(r.Context())
	_ = a.repo.RecordAuthAudit(r.Context(), &user.ID, "auth.registered", requestID, clientIP(r), nil)
	csrf, err := a.startSession(w, r, user.ID)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"user": user, "csrfToken": csrf})
}

func (a *API) login(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !decode(w, r, &input) {
		return
	}
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	user, err := a.repo.UserByEmail(r.Context(), input.Email)
	if err != nil || user.Status != "active" || !auth.VerifyPassword(user.PasswordHash, input.Password) {
		_ = a.repo.RecordAuthAudit(r.Context(), nil, "auth.login_failed", requestID(r.Context()), clientIP(r), map[string]any{"email": input.Email})
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "Email or password is incorrect.")
		return
	}
	csrf, err := a.startSession(w, r, user.ID)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	_ = a.repo.RecordAuthAudit(r.Context(), &user.ID, "auth.login", requestID(r.Context()), clientIP(r), nil)
	writeJSON(w, http.StatusOK, map[string]any{"user": user, "csrfToken": csrf})
}

func (a *API) session(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"user": currentUser(r.Context())})
}

func (a *API) logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(a.cfg.CookieName); err == nil {
		if err := a.repo.DeleteSession(r.Context(), auth.HashToken(cookie.Value)); err != nil {
			a.serverError(w, r, err)
			return
		}
	}
	user := currentUser(r.Context())
	_ = a.repo.RecordAuthAudit(r.Context(), &user.ID, "auth.logout", requestID(r.Context()), clientIP(r), nil)
	a.clearCookies(w)
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) listOrganizations(w http.ResponseWriter, r *http.Request) {
	items, err := a.repo.ListOrganizations(r.Context(), currentUser(r.Context()).ID)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"organizations": items})
}

func (a *API) createOrganization(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name string `json:"name"`
		Slug string `json:"slug"`
	}
	if !decode(w, r, &input) {
		return
	}
	input.Name = strings.TrimSpace(input.Name)
	input.Slug = cleanSlug(input.Slug, input.Name)
	if len(input.Name) < 1 || len(input.Name) > 120 || !slugPattern.MatchString(input.Slug) {
		validation(w, "Provide a valid organization name and slug.")
		return
	}
	item, err := a.repo.CreateOrganization(r.Context(), currentUser(r.Context()).ID, input.Name, input.Slug, requestID(r.Context()), clientIP(r))
	if err != nil {
		a.persistenceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (a *API) access(w http.ResponseWriter, r *http.Request) {
	role, _ := r.Context().Value(roleKey).(string)
	writeJSON(w, http.StatusOK, map[string]any{"role": role, "permissions": authorization.Permissions(role)})
}

func (a *API) listMembers(w http.ResponseWriter, r *http.Request) {
	items, err := a.repo.ListMembers(r.Context(), pathUUID(r, "organizationID"))
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"members": items})
}

func (a *API) addMember(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Email string `json:"email"`
		Role  string `json:"role"`
	}
	if !decode(w, r, &input) {
		return
	}
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	if !validEmail(input.Email) || !oneOf(input.Role, "owner", "admin", "developer", "viewer") {
		validation(w, "Provide an existing user email and valid role.")
		return
	}
	orgID := pathUUID(r, "organizationID")
	item, err := a.repo.AddMember(r.Context(), orgID, currentUser(r.Context()).ID, input.Email, input.Role, requestID(r.Context()), clientIP(r))
	if err != nil {
		a.persistenceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (a *API) listProjects(w http.ResponseWriter, r *http.Request) {
	items, err := a.repo.ListProjects(r.Context(), pathUUID(r, "organizationID"))
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"projects": items})
}

func projectInput(w http.ResponseWriter, r *http.Request) (struct {
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
}, bool) {
	var input struct {
		Name        string `json:"name"`
		Slug        string `json:"slug"`
		Description string `json:"description"`
	}
	if !decode(w, r, &input) {
		return input, false
	}
	input.Name = strings.TrimSpace(input.Name)
	input.Slug = cleanSlug(input.Slug, input.Name)
	input.Description = strings.TrimSpace(input.Description)
	if len(input.Name) < 1 || len(input.Name) > 120 || len(input.Description) > 2000 || !slugPattern.MatchString(input.Slug) {
		validation(w, "Provide a valid project name, slug, and description.")
		return input, false
	}
	return input, true
}

func (a *API) createProject(w http.ResponseWriter, r *http.Request) {
	input, ok := projectInput(w, r)
	if !ok {
		return
	}
	orgID := pathUUID(r, "organizationID")
	item, err := a.repo.CreateProject(r.Context(), orgID, currentUser(r.Context()).ID, input.Name, input.Slug, input.Description, requestID(r.Context()), clientIP(r))
	if err != nil {
		a.persistenceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (a *API) getProject(w http.ResponseWriter, r *http.Request) {
	item, err := a.repo.GetProject(r.Context(), pathUUID(r, "organizationID"), pathUUID(r, "projectID"))
	if err != nil {
		a.persistenceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (a *API) updateProject(w http.ResponseWriter, r *http.Request) {
	input, ok := projectInput(w, r)
	if !ok {
		return
	}
	orgID := pathUUID(r, "organizationID")
	item, err := a.repo.UpdateProject(r.Context(), orgID, pathUUID(r, "projectID"), currentUser(r.Context()).ID, input.Name, input.Slug, input.Description, requestID(r.Context()), clientIP(r))
	if err != nil {
		a.persistenceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (a *API) deleteProject(w http.ResponseWriter, r *http.Request) {
	orgID := pathUUID(r, "organizationID")
	if err := a.repo.DeleteProject(r.Context(), orgID, pathUUID(r, "projectID"), currentUser(r.Context()).ID, requestID(r.Context()), clientIP(r)); err != nil {
		a.persistenceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) listEnvironments(w http.ResponseWriter, r *http.Request) {
	items, err := a.repo.ListEnvironments(r.Context(), pathUUID(r, "organizationID"), pathUUID(r, "projectID"))
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"environments": items})
}

func (a *API) createEnvironment(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name string `json:"name"`
		Slug string `json:"slug"`
	}
	if !decode(w, r, &input) {
		return
	}
	input.Name = strings.TrimSpace(input.Name)
	input.Slug = cleanSlug(input.Slug, input.Name)
	if input.Name == "" || !slugPattern.MatchString(input.Slug) {
		validation(w, "Provide a valid environment name and slug.")
		return
	}
	orgID := pathUUID(r, "organizationID")
	item, err := a.repo.CreateEnvironment(r.Context(), orgID, pathUUID(r, "projectID"), currentUser(r.Context()).ID, input.Name, input.Slug, requestID(r.Context()), clientIP(r))
	if err != nil {
		a.persistenceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (a *API) listApplications(w http.ResponseWriter, r *http.Request) {
	items, err := a.repo.ListApplications(r.Context(), pathUUID(r, "organizationID"), pathUUID(r, "environmentID"))
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"applications": items})
}
func (a *API) listAllApplications(w http.ResponseWriter, r *http.Request) {
	items, err := a.repo.ListAllApplications(r.Context(), pathUUID(r, "organizationID"))
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"applications": items})
}

func (a *API) createApplication(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name         string  `json:"name"`
		SourceType   string  `json:"sourceType"`
		Image        *string `json:"image"`
		InternalPort *int    `json:"internalPort"`
	}
	if !decode(w, r, &input) {
		return
	}
	input.Name = strings.TrimSpace(input.Name)
	if input.SourceType == "" {
		input.SourceType = "docker_image"
	}
	if input.Name == "" || !oneOf(input.SourceType, "docker_image", "git_dockerfile", "compose") || (input.InternalPort != nil && (*input.InternalPort < 1 || *input.InternalPort > 65535)) {
		validation(w, "Provide a valid application name, source type, and internal port.")
		return
	}
	orgID := pathUUID(r, "organizationID")
	item, err := a.repo.CreateApplication(r.Context(), orgID, pathUUID(r, "environmentID"), currentUser(r.Context()).ID, input.Name, input.SourceType, input.Image, input.InternalPort, requestID(r.Context()), clientIP(r))
	if err != nil {
		a.persistenceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (a *API) listAllDeployments(w http.ResponseWriter, r *http.Request) {
	items, err := a.repo.ListDeployments(r.Context(), pathUUID(r, "organizationID"), nil)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deployments": items})
}
func (a *API) listDeployments(w http.ResponseWriter, r *http.Request) {
	applicationID := pathUUID(r, "applicationID")
	items, err := a.repo.ListDeployments(r.Context(), pathUUID(r, "organizationID"), &applicationID)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deployments": items})
}

func (a *API) createDeployment(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Source         string `json:"source"`
		SourceRevision string `json:"sourceRevision"`
		Image          string `json:"image"`
	}
	if !decode(w, r, &input) {
		return
	}
	if len(input.Source) > 500 || len(input.SourceRevision) > 200 || len(input.Image) > 500 {
		validation(w, "Deployment fields are too long.")
		return
	}
	orgID := pathUUID(r, "organizationID")
	item, err := a.repo.CreateDeployment(r.Context(), orgID, pathUUID(r, "applicationID"), currentUser(r.Context()).ID, input.Source, input.SourceRevision, input.Image, requestID(r.Context()), clientIP(r))
	if err != nil {
		a.persistenceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (a *API) transitionDeployment(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Status  deployments.State `json:"status"`
		Message string            `json:"message"`
	}
	if !decode(w, r, &input) {
		return
	}
	if !input.Status.Valid() || len(input.Message) > 2000 {
		validation(w, "Provide a valid deployment state and message.")
		return
	}
	orgID := pathUUID(r, "organizationID")
	item, err := a.repo.TransitionDeployment(r.Context(), orgID, pathUUID(r, "deploymentID"), currentUser(r.Context()).ID, input.Status, input.Message, requestID(r.Context()), clientIP(r))
	if err != nil {
		if strings.Contains(err.Error(), "cannot transition") {
			writeError(w, http.StatusConflict, "invalid_transition", err.Error())
			return
		}
		a.persistenceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (a *API) listServers(w http.ResponseWriter, r *http.Request) {
	items, err := a.repo.ListServers(r.Context(), pathUUID(r, "organizationID"))
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"servers": items})
}

func (a *API) createServer(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name            string `json:"name"`
		Hostname        string `json:"hostname"`
		OperatingSystem string `json:"operatingSystem"`
		Architecture    string `json:"architecture"`
	}
	if !decode(w, r, &input) {
		return
	}
	input.Name = strings.TrimSpace(input.Name)
	input.Hostname = strings.TrimSpace(input.Hostname)
	if input.Name == "" || input.Hostname == "" {
		validation(w, "Name and hostname are required.")
		return
	}
	orgID := pathUUID(r, "organizationID")
	item, err := a.repo.CreateServer(r.Context(), orgID, currentUser(r.Context()).ID, input.Name, input.Hostname, input.OperatingSystem, input.Architecture, requestID(r.Context()), clientIP(r))
	if err != nil {
		a.persistenceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (a *API) listIdentityProviders(w http.ResponseWriter, r *http.Request) {
	items, err := a.repo.ListIdentityProviders(r.Context(), pathUUID(r, "organizationID"))
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"identityProviders": items})
}

func (a *API) createIdentityProvider(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name         string   `json:"name"`
		ProviderType string   `json:"providerType"`
		IssuerURL    string   `json:"issuerUrl"`
		ClientID     string   `json:"clientId"`
		CallbackURL  string   `json:"callbackUrl"`
		Scopes       []string `json:"scopes"`
	}
	if !decode(w, r, &input) {
		return
	}
	if input.ProviderType == "" {
		input.ProviderType = "oidc"
	}
	if len(input.Scopes) == 0 {
		input.Scopes = []string{"openid", "profile", "email"}
	}
	issuer, err := url.ParseRequestURI(input.IssuerURL)
	if input.Name == "" || err != nil || issuer.Scheme != "https" || !oneOf(input.ProviderType, "oidc", "authentik") || input.ClientID == "" || input.CallbackURL == "" {
		validation(w, "Provide valid OIDC provider metadata. Issuer URL must use HTTPS.")
		return
	}
	orgID := pathUUID(r, "organizationID")
	item, err := a.repo.CreateIdentityProvider(r.Context(), orgID, currentUser(r.Context()).ID, input.Name, input.ProviderType, input.IssuerURL, input.ClientID, input.CallbackURL, input.Scopes, requestID(r.Context()), clientIP(r))
	if err != nil {
		a.persistenceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (a *API) listAuditEvents(w http.ResponseWriter, r *http.Request) {
	items, err := a.repo.ListAuditEvents(r.Context(), pathUUID(r, "organizationID"))
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"auditEvents": items})
}

func (a *API) startSession(w http.ResponseWriter, r *http.Request, userID uuid.UUID) (string, error) {
	if old, err := r.Cookie(a.cfg.CookieName); err == nil {
		_ = a.repo.DeleteSession(r.Context(), auth.HashToken(old.Value))
	}
	token, tokenHash, err := auth.NewToken()
	if err != nil {
		return "", err
	}
	csrf, csrfHash, err := auth.NewToken()
	if err != nil {
		return "", err
	}
	expires := time.Now().Add(a.cfg.SessionTTL)
	if err = a.repo.CreateSession(r.Context(), userID, tokenHash, csrfHash, expires, r.UserAgent(), clientIP(r)); err != nil {
		return "", err
	}
	http.SetCookie(w, &http.Cookie{Name: a.cfg.CookieName, Value: token, Path: "/", HttpOnly: true, Secure: a.cfg.CookieSecure, SameSite: http.SameSiteLaxMode, Expires: expires, MaxAge: int(a.cfg.SessionTTL.Seconds())})
	http.SetCookie(w, &http.Cookie{Name: "silicon_csrf", Value: csrf, Path: "/", HttpOnly: false, Secure: a.cfg.CookieSecure, SameSite: http.SameSiteStrictMode, Expires: expires, MaxAge: int(a.cfg.SessionTTL.Seconds())})
	return csrf, nil
}

func (a *API) clearCookies(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: a.cfg.CookieName, Value: "", Path: "/", HttpOnly: true, Secure: a.cfg.CookieSecure, SameSite: http.SameSiteLaxMode, MaxAge: -1})
	http.SetCookie(w, &http.Cookie{Name: "silicon_csrf", Value: "", Path: "/", Secure: a.cfg.CookieSecure, SameSite: http.SameSiteStrictMode, MaxAge: -1})
}

func (a *API) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(a.cfg.CookieName)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "authentication_required", "Sign in to continue.")
			return
		}
		user, csrfHash, err := a.repo.UserBySession(r.Context(), auth.HashToken(cookie.Value))
		if err != nil {
			a.clearCookies(w)
			writeError(w, http.StatusUnauthorized, "authentication_required", "Sign in to continue.")
			return
		}
		if unsafe(r.Method) {
			provided := r.Header.Get("X-CSRF-Token")
			if provided == "" || subtle.ConstantTimeCompare(auth.HashToken(provided), csrfHash) != 1 {
				writeError(w, http.StatusForbidden, "csrf_failed", "The request could not be verified.")
				return
			}
		}
		ctx := context.WithValue(r.Context(), userKey, user)
		ctx = context.WithValue(ctx, csrfHashKey, csrfHash)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (a *API) org(permission authorization.Permission, next http.Handler) http.Handler {
	return a.auth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		orgID, err := uuid.Parse(r.PathValue("organizationID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_id", "Organization ID is invalid.")
			return
		}
		role, err := a.repo.MembershipRole(r.Context(), orgID, currentUser(r.Context()).ID)
		if err != nil {
			writeError(w, http.StatusNotFound, "not_found", "Resource not found.")
			return
		}
		if !authorization.Allowed(role, permission) {
			writeError(w, http.StatusForbidden, "forbidden", "You do not have permission to perform this action.")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), roleKey, role)))
	}))
}

func (a *API) requestLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := uuid.New()
		ctx := context.WithValue(r.Context(), requestIDKey, requestID)
		start := time.Now()
		wrapped := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(wrapped, r.WithContext(ctx))
		a.logger.Info("http request", "request_id", requestID, "method", r.Method, "path", r.URL.Path, "status", wrapped.status, "duration_ms", time.Since(start).Milliseconds())
	})
}

func (a *API) security(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		next.ServeHTTP(w, r)
	})
}

func (a *API) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == a.cfg.FrontendOrigin {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-CSRF-Token")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Add("Vary", "Origin")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (a *API) recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if value := recover(); value != nil {
				a.logger.Error("panic recovered", "request_id", requestID(r.Context()), "error", fmt.Sprint(value))
				writeError(w, http.StatusInternalServerError, "internal_error", "The request could not be completed.")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func (a *API) persistenceError(w http.ResponseWriter, err error) {
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "Resource not found.")
		return
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			writeError(w, http.StatusConflict, "conflict", "A record with these values already exists.")
			return
		case "23503", "23514", "22P02":
			validation(w, "The request references invalid data.")
			return
		}
	}
	a.logger.Error("persistence error", "error", err)
	writeError(w, http.StatusInternalServerError, "internal_error", "The request could not be completed.")
}
func (a *API) serverError(w http.ResponseWriter, r *http.Request, err error) {
	a.logger.Error("request failed", "request_id", requestID(r.Context()), "error", err)
	writeError(w, http.StatusInternalServerError, "internal_error", "The request could not be completed.")
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) { w.status = code; w.ResponseWriter.WriteHeader(code) }

func decode(w http.ResponseWriter, r *http.Request, destination any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Request body must contain valid JSON.")
		return false
	}
	if decoder.Decode(&struct{}{}) == nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Request body must contain one JSON object.")
		return false
	}
	return true
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}
func validation(w http.ResponseWriter, message string) {
	writeError(w, http.StatusUnprocessableEntity, "validation_failed", message)
}
func currentUser(ctx context.Context) store.User {
	user, _ := ctx.Value(userKey).(store.User)
	return user
}
func requestID(ctx context.Context) uuid.UUID {
	id, _ := ctx.Value(requestIDKey).(uuid.UUID)
	if id == uuid.Nil {
		return uuid.New()
	}
	return id
}
func pathUUID(r *http.Request, name string) uuid.UUID {
	id, err := uuid.Parse(r.PathValue(name))
	if err != nil {
		return uuid.Nil
	}
	return id
}
func unsafe(method string) bool {
	return method != http.MethodGet && method != http.MethodHead && method != http.MethodOptions
}
func validEmail(value string) bool {
	address, err := url.Parse("mailto:" + value)
	return err == nil && strings.Contains(value, "@") && address.Opaque == value && len(value) <= 320
}
func cleanSlug(value, name string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		value = strings.ToLower(strings.TrimSpace(name))
		value = regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(value, "-")
		value = strings.Trim(value, "-")
	}
	return value
}
func oneOf(value string, options ...string) bool {
	for _, option := range options {
		if value == option {
			return true
		}
	}
	return false
}
func clientIP(r *http.Request) net.IP {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	return net.ParseIP(host)
}
