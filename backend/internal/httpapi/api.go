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
	"github.com/itsmangooo/Silicon/backend/internal/buildinfo"
	"github.com/itsmangooo/Silicon/backend/internal/config"
	"github.com/itsmangooo/Silicon/backend/internal/cryptoenvelope"
	"github.com/itsmangooo/Silicon/backend/internal/deployments"
	cloudaws "github.com/itsmangooo/Silicon/backend/internal/providers/cloud/aws"
	connectionprovider "github.com/itsmangooo/Silicon/backend/internal/providers/connection"
	runtimeprovider "github.com/itsmangooo/Silicon/backend/internal/providers/runtime"
	secretprovider "github.com/itsmangooo/Silicon/backend/internal/providers/secrets"
	localsecrets "github.com/itsmangooo/Silicon/backend/internal/providers/secrets/local"
	"github.com/itsmangooo/Silicon/backend/internal/serverconnections"
	"github.com/itsmangooo/Silicon/backend/internal/store"
	"github.com/itsmangooo/Silicon/backend/internal/updates"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type API struct {
	cfg         config.Config
	repo        store.Repository
	logger      *slog.Logger
	box         *cryptoenvelope.Box
	runtime     runtimeprovider.Provider
	secrets     secretprovider.Provider
	connections serverconnections.Manager
	awsFactory  cloudaws.Factory
	updates     *updates.Checker
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
	return NewWithProviders(cfg, pool, logger, nil, nil)
}

func NewWithProviders(cfg config.Config, pool *pgxpool.Pool, logger *slog.Logger, runtime runtimeprovider.Provider, secrets secretprovider.Provider) *API {
	if cfg.RuntimeLogFollowTimeout <= 0 {
		cfg.RuntimeLogFollowTimeout = 5 * time.Minute
	}
	build := buildinfo.Current()
	api := &API{
		cfg: cfg, repo: store.Repository{Pool: pool}, logger: logger, runtime: runtime, secrets: secrets, awsFactory: cloudaws.SDKFactory{},
		updates: &updates.Checker{
			Source:         updates.GitHubSource{APIBaseURL: cfg.GitHubReleaseAPIURL, Repository: cfg.GitHubRepository},
			CurrentVersion: build.Version, CommitSHA: build.CommitSHA, BuildTime: build.BuildTime, CacheTTL: cfg.ReleaseCheckTTL,
		},
	}
	if box, err := cryptoenvelope.New(cfg.EncryptionKey); err == nil {
		api.box = &box
		if api.secrets == nil {
			api.secrets = localsecrets.Provider{Pool: pool, Box: box}
		}
	}
	api.connections = serverconnections.Manager{Repository: api.repo, Box: api.box, LocalEnabled: cfg.LocalDockerEnabled}
	return api
}

func (a *API) SetAWSFactory(factory cloudaws.Factory) {
	if factory != nil {
		a.awsFactory = factory
	}
}

func (a *API) SetAWSConnectionProvider(provider connectionprovider.Provider) {
	a.connections.AWS = provider
}

func (a *API) SetUpdateChecker(checker *updates.Checker) {
	if checker != nil {
		a.updates = checker
	}
}

func (a *API) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", a.health)
	mux.HandleFunc("POST /api/v1/auth/register", a.register)
	mux.HandleFunc("POST /api/v1/auth/login", a.login)
	mux.HandleFunc("POST /api/v1/webhooks/github", a.githubWebhook)
	mux.Handle("GET /api/v1/auth/session", a.auth(http.HandlerFunc(a.session)))
	mux.Handle("POST /api/v1/auth/logout", a.auth(http.HandlerFunc(a.logout)))
	mux.Handle("GET /api/v1/organizations", a.auth(http.HandlerFunc(a.listOrganizations)))
	mux.Handle("POST /api/v1/organizations", a.auth(http.HandlerFunc(a.createOrganization)))
	mux.Handle("GET /api/v1/system/version", a.auth(http.HandlerFunc(a.systemVersion)))
	mux.Handle("GET /api/v1/system/updates", a.auth(http.HandlerFunc(a.getSystemUpdates)))
	mux.Handle("POST /api/v1/system/updates/check", a.systemAdmin(http.HandlerFunc(a.checkSystemUpdates)))
	mux.Handle("POST /api/v1/system/updates", a.systemAdmin(http.HandlerFunc(a.createSystemUpdate)))
	mux.Handle("GET /api/v1/system/public-access", a.systemAdmin(http.HandlerFunc(a.getSystemPublicAccess)))
	mux.Handle("POST /api/v1/system/public-access", a.systemAdmin(http.HandlerFunc(a.configureSystemPublicAccess)))
	mux.Handle("DELETE /api/v1/system/public-access", a.systemAdmin(http.HandlerFunc(a.disableSystemPublicAccess)))

	mux.Handle("GET /api/v1/organizations/{organizationID}/access", a.org(authorization.OrganizationRead, http.HandlerFunc(a.access)))
	mux.Handle("GET /api/v1/organizations/{organizationID}/search", a.org(authorization.OrganizationRead, http.HandlerFunc(a.search)))
	mux.Handle("GET /api/v1/organizations/{organizationID}/members", a.org(authorization.MemberRead, http.HandlerFunc(a.listMembers)))
	mux.Handle("POST /api/v1/organizations/{organizationID}/members", a.org(authorization.MemberManage, http.HandlerFunc(a.addMember)))
	mux.Handle("GET /api/v1/organizations/{organizationID}/projects", a.org(authorization.ProjectRead, http.HandlerFunc(a.listProjects)))
	mux.Handle("POST /api/v1/organizations/{organizationID}/projects", a.org(authorization.ProjectCreate, http.HandlerFunc(a.createProject)))
	mux.Handle("GET /api/v1/organizations/{organizationID}/projects/{projectID}", a.org(authorization.ProjectRead, http.HandlerFunc(a.getProject)))
	mux.Handle("PUT /api/v1/organizations/{organizationID}/projects/{projectID}", a.org(authorization.ProjectUpdate, http.HandlerFunc(a.updateProject)))
	mux.Handle("DELETE /api/v1/organizations/{organizationID}/projects/{projectID}", a.org(authorization.ProjectDelete, http.HandlerFunc(a.deleteProject)))
	mux.Handle("GET /api/v1/organizations/{organizationID}/projects/{projectID}/environments", a.org(authorization.EnvironmentRead, http.HandlerFunc(a.listEnvironments)))
	mux.Handle("POST /api/v1/organizations/{organizationID}/projects/{projectID}/environments", a.org(authorization.EnvironmentCreate, http.HandlerFunc(a.createEnvironment)))
	mux.Handle("GET /api/v1/organizations/{organizationID}/environments/{environmentID}", a.org(authorization.EnvironmentRead, http.HandlerFunc(a.getEnvironment)))
	mux.Handle("PUT /api/v1/organizations/{organizationID}/environments/{environmentID}", a.org(authorization.EnvironmentUpdate, http.HandlerFunc(a.updateEnvironment)))
	mux.Handle("DELETE /api/v1/organizations/{organizationID}/environments/{environmentID}", a.org(authorization.EnvironmentDelete, http.HandlerFunc(a.deleteEnvironment)))
	mux.Handle("GET /api/v1/organizations/{organizationID}/applications", a.org(authorization.ApplicationRead, http.HandlerFunc(a.listAllApplications)))
	mux.Handle("GET /api/v1/organizations/{organizationID}/environments/{environmentID}/applications", a.org(authorization.ApplicationRead, http.HandlerFunc(a.listApplications)))
	mux.Handle("POST /api/v1/organizations/{organizationID}/environments/{environmentID}/applications", a.org(authorization.ApplicationCreate, http.HandlerFunc(a.createApplication)))
	mux.Handle("GET /api/v1/organizations/{organizationID}/applications/{applicationID}", a.org(authorization.ApplicationRead, http.HandlerFunc(a.getApplication)))
	mux.Handle("PUT /api/v1/organizations/{organizationID}/applications/{applicationID}", a.org(authorization.ApplicationUpdate, http.HandlerFunc(a.updateApplication)))
	mux.Handle("DELETE /api/v1/organizations/{organizationID}/applications/{applicationID}", a.org(authorization.ApplicationDelete, http.HandlerFunc(a.deleteApplication)))
	mux.Handle("GET /api/v1/organizations/{organizationID}/deployments", a.org(authorization.DeploymentRead, http.HandlerFunc(a.listAllDeployments)))
	mux.Handle("GET /api/v1/organizations/{organizationID}/applications/{applicationID}/deployments", a.org(authorization.DeploymentRead, http.HandlerFunc(a.listDeployments)))
	mux.Handle("POST /api/v1/organizations/{organizationID}/applications/{applicationID}/deployments", a.org(authorization.DeploymentCreate, http.HandlerFunc(a.createDeployment)))
	mux.Handle("GET /api/v1/organizations/{organizationID}/deployments/{deploymentID}", a.org(authorization.DeploymentRead, http.HandlerFunc(a.getDeployment)))
	mux.Handle("POST /api/v1/organizations/{organizationID}/deployments/{deploymentID}/transitions", a.org(authorization.DeploymentCreate, http.HandlerFunc(a.transitionDeployment)))
	mux.Handle("GET /api/v1/organizations/{organizationID}/projects/{projectID}/environment-variables", a.org(authorization.ProjectRead, http.HandlerFunc(a.listProjectVariables)))
	mux.Handle("PUT /api/v1/organizations/{organizationID}/projects/{projectID}/environment-variables", a.org(authorization.ProjectUpdate, http.HandlerFunc(a.replaceProjectVariables)))
	mux.Handle("POST /api/v1/organizations/{organizationID}/projects/{projectID}/environment-variables/parse", a.org(authorization.ProjectUpdate, http.HandlerFunc(a.parseDotEnv)))
	mux.Handle("GET /api/v1/organizations/{organizationID}/environments/{environmentID}/environment-variables", a.org(authorization.EnvironmentRead, http.HandlerFunc(a.listEnvironmentOverrideVariables)))
	mux.Handle("PUT /api/v1/organizations/{organizationID}/environments/{environmentID}/environment-variables", a.org(authorization.EnvironmentUpdate, http.HandlerFunc(a.replaceEnvironmentOverrideVariables)))
	mux.Handle("POST /api/v1/organizations/{organizationID}/environments/{environmentID}/environment-variables/parse", a.org(authorization.EnvironmentUpdate, http.HandlerFunc(a.parseDotEnv)))
	mux.Handle("GET /api/v1/organizations/{organizationID}/applications/{applicationID}/environment-variables", a.org(authorization.ApplicationRead, http.HandlerFunc(a.listEnvironmentVariables)))
	mux.Handle("PUT /api/v1/organizations/{organizationID}/applications/{applicationID}/environment-variables", a.org(authorization.ApplicationUpdate, http.HandlerFunc(a.replaceEnvironmentVariables)))
	mux.Handle("POST /api/v1/organizations/{organizationID}/applications/{applicationID}/environment-variables/parse", a.org(authorization.ApplicationUpdate, http.HandlerFunc(a.parseDotEnv)))
	mux.Handle("GET /api/v1/organizations/{organizationID}/applications/{applicationID}/configuration", a.org(authorization.ApplicationRead, http.HandlerFunc(a.getEffectiveConfiguration)))
	mux.Handle("GET /api/v1/organizations/{organizationID}/projects/{projectID}/secrets", a.org(authorization.ProjectRead, http.HandlerFunc(a.listProjectSecrets)))
	mux.Handle("PUT /api/v1/organizations/{organizationID}/projects/{projectID}/secrets/{secret}", a.org(authorization.SecretWrite, http.HandlerFunc(a.putProjectSecret)))
	mux.Handle("DELETE /api/v1/organizations/{organizationID}/projects/{projectID}/secrets/{secret}", a.org(authorization.SecretWrite, http.HandlerFunc(a.deleteProjectSecret)))
	mux.Handle("GET /api/v1/organizations/{organizationID}/environments/{environmentID}/secrets", a.org(authorization.EnvironmentRead, http.HandlerFunc(a.listEnvironmentSecrets)))
	mux.Handle("PUT /api/v1/organizations/{organizationID}/environments/{environmentID}/secrets/{secret}", a.org(authorization.SecretWrite, http.HandlerFunc(a.putEnvironmentSecret)))
	mux.Handle("DELETE /api/v1/organizations/{organizationID}/environments/{environmentID}/secrets/{secret}", a.org(authorization.SecretWrite, http.HandlerFunc(a.deleteEnvironmentSecret)))
	mux.Handle("GET /api/v1/organizations/{organizationID}/applications/{applicationID}/secrets", a.org(authorization.ApplicationRead, http.HandlerFunc(a.listSecrets)))
	mux.Handle("PUT /api/v1/organizations/{organizationID}/applications/{applicationID}/secrets/{secret}", a.org(authorization.SecretWrite, http.HandlerFunc(a.putSecret)))
	mux.Handle("DELETE /api/v1/organizations/{organizationID}/applications/{applicationID}/secrets/{secret}", a.org(authorization.SecretWrite, http.HandlerFunc(a.deleteSecret)))
	mux.Handle("GET /api/v1/organizations/{organizationID}/applications/{applicationID}/runtime", a.org(authorization.ApplicationRead, http.HandlerFunc(a.getRuntime)))
	mux.Handle("POST /api/v1/organizations/{organizationID}/applications/{applicationID}/runtime/{action}", a.org(authorization.DeploymentCreate, http.HandlerFunc(a.runtimeAction)))
	mux.Handle("GET /api/v1/organizations/{organizationID}/applications/{applicationID}/runtime/logs", a.org(authorization.LogsRead, http.HandlerFunc(a.runtimeLogs)))
	mux.Handle("GET /api/v1/organizations/{organizationID}/servers", a.org(authorization.ServerRead, http.HandlerFunc(a.listServers)))
	mux.Handle("POST /api/v1/organizations/{organizationID}/servers", a.org(authorization.ServerManage, http.HandlerFunc(a.createServer)))
	mux.Handle("GET /api/v1/organizations/{organizationID}/servers/{serverID}", a.org(authorization.ServerRead, http.HandlerFunc(a.getServer)))
	mux.Handle("PUT /api/v1/organizations/{organizationID}/servers/{serverID}/connection", a.org(authorization.ServerManage, http.HandlerFunc(a.updateServerConnection)))
	mux.Handle("POST /api/v1/organizations/{organizationID}/servers/{serverID}/check", a.org(authorization.ServerManage, http.HandlerFunc(a.checkServerConnection)))
	mux.Handle("POST /api/v1/organizations/{organizationID}/servers/{serverID}/trust-host-key", a.org(authorization.ServerManage, http.HandlerFunc(a.trustServerHostKey)))
	mux.Handle("GET /api/v1/organizations/{organizationID}/networks", a.org(authorization.NetworkRead, http.HandlerFunc(a.listNetworks)))
	mux.Handle("POST /api/v1/organizations/{organizationID}/networks", a.org(authorization.NetworkManage, http.HandlerFunc(a.createNetwork)))
	mux.Handle("GET /api/v1/organizations/{organizationID}/networks/{networkID}", a.org(authorization.NetworkRead, http.HandlerFunc(a.getNetwork)))
	mux.Handle("DELETE /api/v1/organizations/{organizationID}/networks/{networkID}", a.org(authorization.NetworkManage, http.HandlerFunc(a.deleteNetwork)))
	mux.Handle("POST /api/v1/organizations/{organizationID}/networks/{networkID}/members", a.org(authorization.NetworkManage, http.HandlerFunc(a.addNetworkMember)))
	mux.Handle("DELETE /api/v1/organizations/{organizationID}/networks/{networkID}/members/{memberID}", a.org(authorization.NetworkManage, http.HandlerFunc(a.removeNetworkMember)))
	mux.Handle("POST /api/v1/organizations/{organizationID}/networks/{networkID}/services", a.org(authorization.NetworkManage, http.HandlerFunc(a.addNetworkService)))
	mux.Handle("DELETE /api/v1/organizations/{organizationID}/networks/{networkID}/services/{serviceID}", a.org(authorization.NetworkManage, http.HandlerFunc(a.removeNetworkService)))
	mux.Handle("POST /api/v1/organizations/{organizationID}/networks/{networkID}/policies", a.org(authorization.NetworkManage, http.HandlerFunc(a.addNetworkPolicy)))
	mux.Handle("DELETE /api/v1/organizations/{organizationID}/networks/{networkID}/policies/{policyID}", a.org(authorization.NetworkManage, http.HandlerFunc(a.removeNetworkPolicy)))
	mux.Handle("POST /api/v1/organizations/{organizationID}/networks/{networkID}/reconcile", a.org(authorization.NetworkManage, http.HandlerFunc(a.reconcileNetwork)))
	mux.Handle("PUT /api/v1/organizations/{organizationID}/applications/{applicationID}/server", a.org(authorization.ApplicationUpdate, http.HandlerFunc(a.setApplicationServer)))
	mux.Handle("GET /api/v1/organizations/{organizationID}/identity-providers", a.org(authorization.IdentityProviderRead, http.HandlerFunc(a.listIdentityProviders)))
	mux.Handle("POST /api/v1/organizations/{organizationID}/identity-providers", a.org(authorization.IdentityProviderManage, http.HandlerFunc(a.createIdentityProvider)))
	mux.Handle("GET /api/v1/organizations/{organizationID}/audit-events", a.org(authorization.AuditRead, http.HandlerFunc(a.listAuditEvents)))
	mux.Handle("GET /api/v1/organizations/{organizationID}/integrations/github", a.org(authorization.IntegrationRead, http.HandlerFunc(a.getGitHubIntegration)))
	mux.Handle("POST /api/v1/organizations/{organizationID}/integrations/github", a.org(authorization.IntegrationManage, http.HandlerFunc(a.connectGitHub)))
	mux.Handle("DELETE /api/v1/organizations/{organizationID}/integrations/github", a.org(authorization.IntegrationManage, http.HandlerFunc(a.disconnectGitHub)))
	mux.Handle("GET /api/v1/organizations/{organizationID}/integrations/github/repositories", a.org(authorization.IntegrationRead, http.HandlerFunc(a.listGitHubRepositories)))
	mux.Handle("GET /api/v1/organizations/{organizationID}/applications/{applicationID}/git-source", a.org(authorization.ApplicationRead, http.HandlerFunc(a.getGitSource)))
	mux.Handle("PUT /api/v1/organizations/{organizationID}/applications/{applicationID}/git-source", a.org(authorization.ApplicationUpdate, http.HandlerFunc(a.updateGitSource)))
	mux.Handle("GET /api/v1/organizations/{organizationID}/integrations/cloudflare", a.org(authorization.IntegrationRead, http.HandlerFunc(a.getCloudflareIntegration)))
	mux.Handle("POST /api/v1/organizations/{organizationID}/integrations/cloudflare", a.org(authorization.IntegrationManage, http.HandlerFunc(a.connectCloudflare)))
	mux.Handle("GET /api/v1/organizations/{organizationID}/integrations/cloudflare/zones", a.org(authorization.IntegrationRead, http.HandlerFunc(a.listCloudflareZones)))
	mux.Handle("PUT /api/v1/organizations/{organizationID}/integrations/cloudflare/zones/{zoneID}", a.org(authorization.IntegrationManage, http.HandlerFunc(a.selectCloudflareZone)))
	mux.Handle("GET /api/v1/organizations/{organizationID}/domains", a.org(authorization.OrganizationRead, http.HandlerFunc(a.listDomains)))
	mux.Handle("POST /api/v1/organizations/{organizationID}/applications/{applicationID}/domains", a.org(authorization.DomainManage, http.HandlerFunc(a.createDomain)))
	mux.Handle("POST /api/v1/organizations/{organizationID}/domains/{domainID}/sync", a.org(authorization.DomainManage, http.HandlerFunc(a.syncDomain)))
	mux.Handle("PUT /api/v1/organizations/{organizationID}/domains/{domainID}", a.org(authorization.DomainManage, http.HandlerFunc(a.updateDomain)))
	mux.Handle("DELETE /api/v1/organizations/{organizationID}/domains/{domainID}", a.org(authorization.DomainManage, http.HandlerFunc(a.deleteDomain)))
	mux.Handle("GET /api/v1/organizations/{organizationID}/integrations/cloudflare/tunnels", a.org(authorization.IntegrationRead, http.HandlerFunc(a.listTunnels)))
	mux.Handle("POST /api/v1/organizations/{organizationID}/integrations/cloudflare/tunnels", a.org(authorization.IntegrationManage, http.HandlerFunc(a.createTunnel)))
	mux.Handle("POST /api/v1/organizations/{organizationID}/integrations/cloudflare/tunnels/{tunnelID}/install", a.org(authorization.IntegrationManage, http.HandlerFunc(a.installTunnel)))
	mux.Handle("POST /api/v1/organizations/{organizationID}/integrations/cloudflare/tunnels/{tunnelID}/routes", a.org(authorization.DomainManage, http.HandlerFunc(a.createTunnelRoute)))
	mux.Handle("GET /api/v1/organizations/{organizationID}/aws/accounts", a.org(authorization.CloudRead, http.HandlerFunc(a.listAWSAccounts)))
	mux.Handle("POST /api/v1/organizations/{organizationID}/aws/accounts", a.org(authorization.CloudManage, http.HandlerFunc(a.connectAWSAccount)))
	mux.Handle("DELETE /api/v1/organizations/{organizationID}/aws/accounts/{accountID}", a.org(authorization.CloudManage, http.HandlerFunc(a.disconnectAWSAccount)))
	mux.Handle("POST /api/v1/organizations/{organizationID}/aws/accounts/{accountID}/test", a.org(authorization.CloudManage, http.HandlerFunc(a.testAWSAccount)))
	mux.Handle("GET /api/v1/organizations/{organizationID}/aws/accounts/{accountID}/regions", a.org(authorization.CloudRead, http.HandlerFunc(a.listAWSRegions)))
	mux.Handle("GET /api/v1/organizations/{organizationID}/aws/accounts/{accountID}/inventory", a.org(authorization.CloudRead, http.HandlerFunc(a.getAWSInventory)))
	mux.Handle("GET /api/v1/organizations/{organizationID}/aws/accounts/{accountID}/instance-types/{instanceType}", a.org(authorization.CloudRead, http.HandlerFunc(a.getAWSInstanceType)))
	mux.Handle("POST /api/v1/organizations/{organizationID}/aws/accounts/{accountID}/estimate", a.org(authorization.CloudProvision, http.HandlerFunc(a.estimateAWSMachine)))
	mux.Handle("POST /api/v1/organizations/{organizationID}/aws/accounts/{accountID}/machines", a.org(authorization.CloudProvision, http.HandlerFunc(a.provisionAWSMachine)))
	mux.Handle("POST /api/v1/organizations/{organizationID}/aws/accounts/{accountID}/instances/{instanceID}/actions/{action}", a.org(authorization.CloudManage, http.HandlerFunc(a.awsInstanceAction)))
	mux.Handle("POST /api/v1/organizations/{organizationID}/aws/accounts/{accountID}/vpcs", a.org(authorization.CloudProvision, http.HandlerFunc(a.createAWSVPC)))
	mux.Handle("DELETE /api/v1/organizations/{organizationID}/aws/accounts/{accountID}/vpcs/{resourceID}", a.org(authorization.CloudDelete, http.HandlerFunc(a.deleteAWSVPC)))
	mux.Handle("POST /api/v1/organizations/{organizationID}/aws/accounts/{accountID}/subnets", a.org(authorization.CloudProvision, http.HandlerFunc(a.createAWSSubnet)))
	mux.Handle("POST /api/v1/organizations/{organizationID}/aws/accounts/{accountID}/security-groups", a.org(authorization.CloudProvision, http.HandlerFunc(a.createAWSSecurityGroup)))
	mux.Handle("POST /api/v1/organizations/{organizationID}/aws/accounts/{accountID}/security-groups/{resourceID}/rules", a.org(authorization.CloudManage, http.HandlerFunc(a.addAWSSecurityRule)))
	mux.Handle("DELETE /api/v1/organizations/{organizationID}/aws/accounts/{accountID}/security-groups/{resourceID}/rules", a.org(authorization.CloudManage, http.HandlerFunc(a.removeAWSSecurityRule)))
	mux.Handle("PUT /api/v1/organizations/{organizationID}/aws/accounts/{accountID}/instances/{instanceID}/security-groups", a.org(authorization.CloudManage, http.HandlerFunc(a.setAWSInstanceSecurityGroups)))
	mux.Handle("POST /api/v1/organizations/{organizationID}/aws/accounts/{accountID}/elastic-ips", a.org(authorization.CloudProvision, http.HandlerFunc(a.allocateAWSElasticIP)))
	mux.Handle("POST /api/v1/organizations/{organizationID}/aws/accounts/{accountID}/elastic-ips/{resourceID}/associate", a.org(authorization.CloudManage, http.HandlerFunc(a.associateAWSElasticIP)))
	mux.Handle("POST /api/v1/organizations/{organizationID}/aws/accounts/{accountID}/elastic-ips/{resourceID}/disassociate", a.org(authorization.CloudManage, http.HandlerFunc(a.disassociateAWSElasticIP)))
	mux.Handle("DELETE /api/v1/organizations/{organizationID}/aws/accounts/{accountID}/elastic-ips/{resourceID}", a.org(authorization.CloudDelete, http.HandlerFunc(a.releaseAWSElasticIP)))
	mux.Handle("POST /api/v1/organizations/{organizationID}/aws/accounts/{accountID}/volumes", a.org(authorization.CloudProvision, http.HandlerFunc(a.createAWSVolume)))
	mux.Handle("POST /api/v1/organizations/{organizationID}/aws/accounts/{accountID}/volumes/{resourceID}/attach", a.org(authorization.CloudManage, http.HandlerFunc(a.attachAWSVolume)))
	mux.Handle("POST /api/v1/organizations/{organizationID}/aws/accounts/{accountID}/volumes/{resourceID}/detach", a.org(authorization.CloudManage, http.HandlerFunc(a.detachAWSVolume)))
	mux.Handle("DELETE /api/v1/organizations/{organizationID}/aws/accounts/{accountID}/volumes/{resourceID}", a.org(authorization.CloudDelete, http.HandlerFunc(a.deleteAWSVolume)))
	mux.Handle("POST /api/v1/organizations/{organizationID}/aws/accounts/{accountID}/snapshots", a.org(authorization.CloudProvision, http.HandlerFunc(a.createAWSSnapshot)))
	mux.Handle("DELETE /api/v1/organizations/{organizationID}/aws/accounts/{accountID}/snapshots/{resourceID}", a.org(authorization.CloudDelete, http.HandlerFunc(a.deleteAWSSnapshot)))
	mux.Handle("GET /api/v1/organizations/{organizationID}/aws/costs", a.org(authorization.CostRead, http.HandlerFunc(a.getAWSCosts)))
	mux.Handle("GET /api/v1/organizations/{organizationID}/aws/operations", a.org(authorization.CloudRead, http.HandlerFunc(a.listAWSOperations)))
	mux.Handle("GET /api/v1/organizations/{organizationID}/budgets", a.org(authorization.BudgetRead, http.HandlerFunc(a.listBudgets)))
	mux.Handle("POST /api/v1/organizations/{organizationID}/budgets", a.org(authorization.BudgetManage, http.HandlerFunc(a.createBudget)))
	mux.Handle("DELETE /api/v1/organizations/{organizationID}/budgets/{budgetID}", a.org(authorization.BudgetManage, http.HandlerFunc(a.deleteBudget)))

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
		a.persistenceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"environments": items})
}

func (a *API) createEnvironment(w http.ResponseWriter, r *http.Request) {
	input, ok := environmentInput(w, r)
	if !ok {
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

func environmentInput(w http.ResponseWriter, r *http.Request) (struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}, bool) {
	var input struct {
		Name string `json:"name"`
		Slug string `json:"slug"`
	}
	if !decode(w, r, &input) {
		return input, false
	}
	input.Name = strings.TrimSpace(input.Name)
	input.Slug = cleanSlug(input.Slug, input.Name)
	if input.Name == "" || len(input.Name) > 120 || !slugPattern.MatchString(input.Slug) {
		validation(w, "Provide a valid environment name and slug.")
		return input, false
	}
	return input, true
}

func (a *API) getEnvironment(w http.ResponseWriter, r *http.Request) {
	item, err := a.repo.GetEnvironment(r.Context(), pathUUID(r, "organizationID"), pathUUID(r, "environmentID"))
	if err != nil {
		a.persistenceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (a *API) updateEnvironment(w http.ResponseWriter, r *http.Request) {
	input, ok := environmentInput(w, r)
	if !ok {
		return
	}
	item, err := a.repo.UpdateEnvironment(r.Context(), pathUUID(r, "organizationID"), pathUUID(r, "environmentID"), currentUser(r.Context()).ID, input.Name, input.Slug, requestID(r.Context()), clientIP(r))
	if err != nil {
		a.persistenceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (a *API) deleteEnvironment(w http.ResponseWriter, r *http.Request) {
	err := a.repo.DeleteEnvironment(r.Context(), pathUUID(r, "organizationID"), pathUUID(r, "environmentID"), currentUser(r.Context()).ID, requestID(r.Context()), clientIP(r))
	if err != nil {
		a.persistenceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) listApplications(w http.ResponseWriter, r *http.Request) {
	items, err := a.repo.ListApplications(r.Context(), pathUUID(r, "organizationID"), pathUUID(r, "environmentID"))
	if err != nil {
		a.persistenceError(w, err)
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

type applicationInputPayload struct {
	Name          string     `json:"name"`
	SourceType    string     `json:"sourceType"`
	Image         *string    `json:"image"`
	InternalPort  *int       `json:"internalPort"`
	HostAddress   *string    `json:"hostAddress"`
	PublishedPort *int       `json:"publishedPort"`
	ServerID      *uuid.UUID `json:"serverId"`
}

func readApplicationInput(w http.ResponseWriter, r *http.Request) (applicationInputPayload, bool) {
	var input applicationInputPayload
	if !decode(w, r, &input) {
		return input, false
	}
	input.Name = strings.TrimSpace(input.Name)
	if input.Image != nil {
		value := strings.TrimSpace(*input.Image)
		input.Image = &value
	}
	if input.SourceType == "" {
		input.SourceType = "docker_image"
	}
	if input.HostAddress != nil {
		value := strings.TrimSpace(*input.HostAddress)
		if value == "" {
			input.HostAddress = nil
		} else {
			input.HostAddress = &value
		}
	}
	if message := validateApplicationInput(input); message != "" {
		validation(w, message)
		return input, false
	}
	return input, true
}

func validateApplicationInput(input applicationInputPayload) string {
	switch {
	case input.Name == "":
		return "Application name is required."
	case len(input.Name) > 120:
		return "Application name must be 120 characters or fewer."
	case input.SourceType == "compose":
		return "Docker Compose applications are not supported yet."
	case !oneOf(input.SourceType, "docker_image", "git_dockerfile"):
		return "Source type must be docker_image or git_dockerfile."
	case input.SourceType == "docker_image" && (input.Image == nil || *input.Image == ""):
		return "Docker image applications require an image reference."
	case input.Image != nil && (len(*input.Image) > 500 || strings.HasPrefix(*input.Image, "-") || strings.ContainsAny(*input.Image, " \t\r\n\x00")):
		return "Image reference is invalid."
	case input.InternalPort != nil && (*input.InternalPort < 1 || *input.InternalPort > 65535):
		return "Internal port must be between 1 and 65535."
	case input.PublishedPort != nil && (*input.PublishedPort < 1 || *input.PublishedPort > 65535):
		return "Published port must be between 1 and 65535."
	case input.PublishedPort != nil && input.InternalPort == nil:
		return "Published port requires an internal container port."
	case input.PublishedPort != nil && input.HostAddress == nil:
		return "Host address is required when publishing a host port."
	case input.HostAddress != nil && input.PublishedPort == nil:
		return "Published host port is required when a host address is provided."
	case input.HostAddress != nil && net.ParseIP(*input.HostAddress) == nil:
		return "Host address must be a valid IP address."
	default:
		return ""
	}
}

func (a *API) applicationTargetAvailable(w http.ResponseWriter, r *http.Request, organizationID uuid.UUID, serverID *uuid.UUID) bool {
	if serverID == nil {
		if !a.cfg.LocalDockerEnabled {
			validation(w, "Local Docker runtime is disabled. Select a connected server.")
			return false
		}
		return true
	}
	server, err := a.repo.ServerByID(r.Context(), organizationID, *serverID)
	if err != nil {
		a.persistenceError(w, err)
		return false
	}
	if server.ConnectionStatus == "docker_unavailable" {
		validation(w, "Selected server does not have an available Docker runtime.")
		return false
	}
	if server.ConnectionStatus != "connected" {
		validation(w, "Selected server is not connected.")
		return false
	}
	if !server.DockerAvailable {
		validation(w, "Selected server does not have an available Docker runtime.")
		return false
	}
	return true
}

func sameUUID(left, right *uuid.UUID) bool {
	return left == nil && right == nil || left != nil && right != nil && *left == *right
}

func (a *API) createApplication(w http.ResponseWriter, r *http.Request) {
	input, ok := readApplicationInput(w, r)
	if !ok {
		return
	}
	orgID := pathUUID(r, "organizationID")
	if !a.applicationTargetAvailable(w, r, orgID, input.ServerID) {
		return
	}
	item, err := a.repo.CreateApplication(r.Context(), orgID, pathUUID(r, "environmentID"), currentUser(r.Context()).ID, input.Name, input.SourceType, input.Image, input.InternalPort, input.HostAddress, input.PublishedPort, input.ServerID, requestID(r.Context()), clientIP(r))
	if err != nil {
		a.persistenceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (a *API) getApplication(w http.ResponseWriter, r *http.Request) {
	item, err := a.repo.ApplicationByID(r.Context(), pathUUID(r, "organizationID"), pathUUID(r, "applicationID"))
	if err != nil {
		a.persistenceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (a *API) updateApplication(w http.ResponseWriter, r *http.Request) {
	input, ok := readApplicationInput(w, r)
	if !ok {
		return
	}
	organizationID, applicationID := pathUUID(r, "organizationID"), pathUUID(r, "applicationID")
	current, err := a.repo.ApplicationByID(r.Context(), organizationID, applicationID)
	if err != nil {
		a.persistenceError(w, err)
		return
	}
	if !sameUUID(current.ServerID, input.ServerID) && !a.applicationTargetAvailable(w, r, organizationID, input.ServerID) {
		return
	}
	item, err := a.repo.UpdateApplication(r.Context(), organizationID, applicationID, currentUser(r.Context()).ID, input.Name, input.SourceType, input.Image, input.InternalPort, input.HostAddress, input.PublishedPort, input.ServerID, requestID(r.Context()), clientIP(r))
	if err != nil {
		a.persistenceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (a *API) deleteApplication(w http.ResponseWriter, r *http.Request) {
	err := a.repo.DeleteApplication(r.Context(), pathUUID(r, "organizationID"), pathUUID(r, "applicationID"), currentUser(r.Context()).ID, requestID(r.Context()), clientIP(r))
	if err != nil {
		a.persistenceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
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
		a.persistenceError(w, err)
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

func (a *API) getDeployment(w http.ResponseWriter, r *http.Request) {
	item, err := a.repo.GetDeploymentDetail(r.Context(), pathUUID(r, "organizationID"), pathUUID(r, "deploymentID"))
	if err != nil {
		a.persistenceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
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
	writeJSON(w, http.StatusOK, map[string]any{"servers": items, "localRuntimeAvailable": a.cfg.LocalDockerEnabled})
}

func (a *API) createServer(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name             string `json:"name"`
		ConnectivityType string `json:"connectivityType"`
		serverConnectionInput
	}
	if !decode(w, r, &input) {
		return
	}
	input.Name = strings.TrimSpace(input.Name)
	if input.ConnectivityType == "" {
		input.ConnectivityType = "public"
	}
	input.serverConnectionInput = normalizeConnectionInput(input.serverConnectionInput)
	if input.Name == "" || !oneOf(input.ConnectivityType, "public", "self_hosted", "private") {
		validation(w, "A valid server name and connectivity type are required.")
		return
	}
	if message := validateConnectionInput(input.serverConnectionInput, input.ConnectionType == "ssh"); message != "" {
		validation(w, message)
		return
	}
	orgID := pathUUID(r, "organizationID")
	serverID := uuid.New()
	encrypted, ok := a.encryptSSHKey(w, orgID, serverID, input.PrivateKey)
	input.PrivateKey = ""
	if !ok {
		return
	}
	item, err := a.repo.CreateServer(r.Context(), orgID, currentUser(r.Context()).ID, store.ServerInput{ID: serverID, Name: input.Name, Hostname: input.Host, ConnectivityType: input.ConnectivityType, ConnectionType: input.ConnectionType, PublicAddress: input.PublicAddress, SSHPort: input.Port, SSHUsername: input.Username, EncryptedPrivateKey: encrypted, HostKeyFingerprint: input.HostKeyFingerprint}, requestID(r.Context()), clientIP(r))
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
			if !a.requestOriginAllowed(r) {
				writeError(w, http.StatusForbidden, "origin_failed", "The request origin could not be verified.")
				return
			}
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

func (a *API) requestOriginAllowed(r *http.Request) bool {
	origin := strings.TrimRight(strings.TrimSpace(r.Header.Get("Origin")), "/")
	if origin == "" {
		return true
	}
	if origin != strings.TrimRight(a.cfg.FrontendOrigin, "/") && origin != strings.TrimRight(a.cfg.PublicURL, "/") {
		return false
	}
	if a.cfg.TrustForwardedProto && strings.HasPrefix(a.cfg.PublicURL, "https://") {
		forwarded := strings.ToLower(strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-Proto"), ",")[0]))
		return forwarded == "https"
	}
	return true
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

func (a *API) systemAdmin(next http.Handler) http.Handler {
	return a.auth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !currentUser(r.Context()).IsSystemAdmin {
			writeError(w, http.StatusForbidden, "system_admin_required", "Installation administrator access is required.")
			return
		}
		next.ServeHTTP(w, r)
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
	if errors.Is(err, store.ErrConflict) {
		writeError(w, http.StatusConflict, "dependent_records", "Remove dependent resources before deleting this record.")
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
