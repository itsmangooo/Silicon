package httpapi

import (
	"errors"
	"net/http"
	"time"

	"github.com/itsmangooo/Silicon/backend/internal/authorization"
	"github.com/jackc/pgx/v5"
)

type capability struct {
	ID                   string                     `json:"id"`
	Name                 string                     `json:"name"`
	Description          string                     `json:"description"`
	Category             string                     `json:"category"`
	ProviderType         string                     `json:"providerType"`
	ImplementationStatus string                     `json:"implementationStatus"`
	ConfigurationStatus  string                     `json:"configurationStatus"`
	RequiredDependencies []string                   `json:"requiredDependencies"`
	Operations           []string                   `json:"operations"`
	Permissions          []authorization.Permission `json:"permissions"`
	Documentation        string                     `json:"documentation"`
	SettingsPath         string                     `json:"settingsPath,omitempty"`
	LastVerifiedAt       *time.Time                 `json:"lastVerifiedAt,omitempty"`
}

const capabilityDocsRoot = "https://itsmangooo.github.io/Silicon-Docs/docs/guides/"

func (a *API) listCapabilities(w http.ResponseWriter, r *http.Request) {
	ctx, org := r.Context(), pathUUID(r, "organizationID")
	docker := capability{
		ID: "docker", Name: "Docker applications", Category: "Hosting", ProviderType: "runtime",
		Description:          "Deploy a Docker image or build an exact Git revision on a supported Docker target.",
		ImplementationStatus: "implemented", ConfigurationStatus: "not_configured",
		RequiredDependencies: []string{"Docker on a local or connected server"},
		Operations:           []string{"deploy", "inspect", "start", "stop", "restart", "logs"},
		Permissions:          []authorization.Permission{authorization.ApplicationCreate, authorization.DeploymentCreate, authorization.LogsRead},
		Documentation:        capabilityDocsRoot + "docker-runtime-settings", SettingsPath: "/servers",
	}
	servers, err := a.repo.ListServers(ctx, org)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	if a.cfg.LocalDockerEnabled && a.runtime != nil {
		docker.ConfigurationStatus = "unverified"
	}
	for _, server := range servers {
		if server.ConnectionType == "local" && !a.cfg.LocalDockerEnabled {
			continue
		}
		if server.ConnectionStatus == "connected" && server.DockerAvailable && a.runtime != nil && server.LastCheckedAt != nil {
			docker.ConfigurationStatus = "available"
			docker.LastVerifiedAt = latestVerification(docker.LastVerifiedAt, server.LastCheckedAt)
		} else if docker.ConfigurationStatus == "not_configured" && server.LastCheckedAt != nil {
			docker.ConfigurationStatus = "unavailable"
		} else if docker.ConfigurationStatus == "not_configured" {
			docker.ConfigurationStatus = "unverified"
		}
	}
	github := capability{
		ID: "github", Name: "GitHub App", Category: "Source", ProviderType: "git",
		Description:          "Choose granted repositories, deploy exact commits, and handle signed push events.",
		ImplementationStatus: "implemented", ConfigurationStatus: "not_configured",
		RequiredDependencies: []string{"GitHub App credentials", "GitHub App installation"},
		Operations:           []string{"discover repositories", "bind source", "resolve exact commits"},
		Permissions:          []authorization.Permission{authorization.IntegrationManage, authorization.ApplicationUpdate},
		Documentation:        capabilityDocsRoot + "github", SettingsPath: "/integrations",
	}
	gitConnection, err := a.repo.GitHubIntegration(ctx, org)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
	case err != nil:
		a.serverError(w, r, err)
		return
	case gitConnection.Status == "error":
		github.ConfigurationStatus = "unavailable"
	case gitConnection.Status == "connected":
		if a.cfg.GitHubAppID <= 0 || a.cfg.GitHubPrivateKey == "" {
			github.ConfigurationStatus = "unavailable"
		} else {
			github.ConfigurationStatus = "available"
			github.LastVerifiedAt = &gitConnection.UpdatedAt
		}
	}
	aws := capability{
		ID: "aws-ec2", Name: "AWS infrastructure", Category: "Cloud", ProviderType: "aws",
		Description:          "Manage supported EC2, VPC and EBS resources. Readiness reflects AWS identity checks; individual IAM permissions still apply.",
		ImplementationStatus: "implemented", ConfigurationStatus: "not_configured",
		RequiredDependencies: []string{"Verified AWS account", "Region and scoped AWS credentials"},
		Operations:           []string{"inventory", "provision owned resources", "inspect cost"},
		Permissions:          []authorization.Permission{authorization.CloudRead, authorization.CloudProvision, authorization.CostRead},
		Documentation:        capabilityDocsRoot + "aws", SettingsPath: "/aws/accounts",
	}
	accounts, err := a.repo.ListAWSAccounts(ctx, org)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	for _, account := range accounts {
		switch {
		case a.box == nil:
			aws.ConfigurationStatus = "unavailable"
		case account.Status == "connected" && account.LastCheckedAt != nil:
			aws.ConfigurationStatus = "available"
			aws.LastVerifiedAt = latestVerification(aws.LastVerifiedAt, account.LastCheckedAt)
		case aws.ConfigurationStatus != "available" && account.Status == "error":
			aws.ConfigurationStatus = "unavailable"
		case aws.ConfigurationStatus == "not_configured":
			aws.ConfigurationStatus = "unverified"
		}
	}
	cloudflare := capability{
		ID: "cloudflare-dns", Name: "Cloudflare DNS", Category: "Routing", ProviderType: "dns",
		Description:          "Manage Silicon-owned DNS records in selected zones. The last check verifies the connection, and each domain reports its own routing state.",
		ImplementationStatus: "implemented", ConfigurationStatus: "not_configured",
		RequiredDependencies: []string{"Scoped Cloudflare token", "Selected DNS zone"},
		Operations:           []string{"import zones", "create DNS records", "verify DNS", "remove owned DNS records"},
		Permissions:          []authorization.Permission{authorization.IntegrationManage, authorization.DomainManage},
		Documentation:        capabilityDocsRoot + "cloudflare", SettingsPath: "/integrations",
	}
	connection, err := a.repo.CloudflareIntegration(ctx, org)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
	case err != nil:
		a.serverError(w, r, err)
		return
	case connection.Status == "error" || a.box == nil:
		cloudflare.ConfigurationStatus = "unavailable"
	case connection.Status == "connected":
		zones, zoneErr := a.repo.ListCloudflareZones(ctx, org)
		if zoneErr != nil {
			a.serverError(w, r, zoneErr)
			return
		}
		for _, zone := range zones {
			if zone.Selected && zone.IntegrationID == connection.ID {
				cloudflare.ConfigurationStatus = "unverified"
				if connection.LastCheckedAt != nil {
					cloudflare.ConfigurationStatus = "available"
					cloudflare.LastVerifiedAt = connection.LastCheckedAt
				}
				break
			}
		}
	}
	external := capability{
		ID: "external-routing", Name: "External reverse proxy", Category: "Routing", ProviderType: "routing",
		Description:          "Associate hostnames with applications and supply the upstream address to a proxy you configure.",
		ImplementationStatus: "implemented", ConfigurationStatus: "unverified",
		RequiredDependencies: []string{"Operator-managed reverse proxy and HTTPS"},
		Operations:           []string{"associate hostname", "resolve application origin"},
		Permissions:          []authorization.Permission{authorization.DomainManage},
		Documentation:        capabilityDocsRoot + "external-proxy", SettingsPath: "/domains",
	}
	compose := capability{
		ID: "compose-validation", Name: "Compose file validation", Category: "Hosting", ProviderType: "runtime",
		Description:          "Check an image-based YAML file and preview service order through the API.",
		ImplementationStatus: "experimental", ConfigurationStatus: "available",
		RequiredDependencies: []string{}, Operations: []string{"validate YAML", "preview service order"},
		Permissions:   []authorization.Permission{authorization.ApplicationCreate},
		Documentation: capabilityDocsRoot + "feature-reference",
	}
	items := []capability{docker, github, aws, cloudflare, external, compose}
	planned := []struct{ id, name, category, provider, description string }{
		{"docker-compose", "Compose deployments", "Hosting", "runtime", "Deploy and manage multiple services as one application."},
		{"traefik", "Traefik adapter", "Routing", "routing", "Manage Silicon-owned application routes in a connected Traefik installation."},
		{"automatic-tls", "Automatic TLS", "Routing", "certificate", "Issue and renew certificates for managed routes."},
		{"nginx", "Nginx adapter", "Routing", "routing", "Manage Silicon-owned application routes in a connected Nginx installation."},
		{"x3", "X3 Gateway", "Routing", "routing", "Connect a compatible external routing provider."},
		{"aws-s3", "Amazon S3", "Cloud", "aws", "Discover and manage buckets and objects."},
		{"aws-route53", "Amazon Route 53", "Cloud", "dns", "Discover hosted zones and manage records."},
		{"aws-rds", "Amazon RDS", "Cloud", "aws", "Discover and operate managed databases."},
		{"aws-ecs", "Amazon ECS", "Cloud", "runtime", "Deploy workloads to ECS services."},
		{"aws-lambda", "AWS Lambda", "Cloud", "runtime", "Discover and deploy functions."},
		{"aws-asg", "AWS Auto Scaling Groups", "Cloud", "aws", "Inspect and manage scaling capacity."},
		{"aws-eks", "Amazon EKS", "Cloud", "aws", "Discover and connect to managed Kubernetes clusters."},
		{"azure", "Microsoft Azure", "Cloud", "azure", "Discover and connect Azure compute resources."},
		{"kubernetes", "Kubernetes", "Hosting", "runtime", "Connect existing clusters and manage workloads."},
		{"saml", "SAML single sign-on", "Identity", "identity", "Evaluate federated authentication for installations that need it."},
		{"kafka", "Kafka", "Architecture", "events", "Evaluate event-broker needs."},
		{"service-mesh", "Service mesh", "Architecture", "network", "Evaluate existing mesh integrations for multi-host deployments."},
		{"microservices", "Microservices", "Architecture", "architecture", "Evaluate separate services where scaling or isolation requires them."},
	}
	for _, entry := range planned {
		items = append(items, capability{
			ID: entry.id, Name: entry.name, Category: entry.category, ProviderType: entry.provider, Description: entry.description,
			ImplementationStatus: "planned", ConfigurationStatus: "not_applicable",
			RequiredDependencies: []string{}, Operations: []string{}, Permissions: []authorization.Permission{},
			Documentation: "https://github.com/itsmangooo/Silicon/blob/main/ROADMAP.md",
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"capabilities": items})
}

func latestVerification(current, candidate *time.Time) *time.Time {
	if current == nil || (candidate != nil && candidate.After(*current)) {
		return candidate
	}
	return current
}
