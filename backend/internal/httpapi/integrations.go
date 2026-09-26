package httpapi

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/itsmangooo/Silicon/backend/internal/cryptoenvelope"
	cfprovider "github.com/itsmangooo/Silicon/backend/internal/providers/cloudflare"
	"github.com/itsmangooo/Silicon/backend/internal/providers/connection"
	dnsprovider "github.com/itsmangooo/Silicon/backend/internal/providers/dns"
	githubprovider "github.com/itsmangooo/Silicon/backend/internal/providers/git/github"
	tunnelprovider "github.com/itsmangooo/Silicon/backend/internal/providers/tunnel"
	"github.com/itsmangooo/Silicon/backend/internal/store"
)

const maxWebhookBytes = 1 << 20

var dnsNamePattern = regexp.MustCompile(`^(?i:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)+)$`)

func (a *API) githubClient() githubprovider.Client {
	return githubprovider.Client{AppID: a.cfg.GitHubAppID, PrivateKey: a.cfg.GitHubPrivateKey, BaseURL: a.cfg.GitHubAPIURL}
}

func (a *API) getGitHubIntegration(w http.ResponseWriter, r *http.Request) {
	item, err := a.repo.GitHubIntegration(r.Context(), pathUUID(r, "organizationID"))
	if err != nil {
		a.persistenceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}
func (a *API) connectGitHub(w http.ResponseWriter, r *http.Request) {
	var input struct {
		InstallationID int64 `json:"installationId"`
	}
	if !decode(w, r, &input) {
		return
	}
	if input.InstallationID < 1 {
		validation(w, "A valid GitHub App installation ID is required.")
		return
	}
	if a.cfg.GitHubAppID < 1 || a.cfg.GitHubPrivateKey == "" {
		writeError(w, http.StatusServiceUnavailable, "github_app_unavailable", "The GitHub App is not configured on this Silicon instance.")
		return
	}
	installation, err := a.githubClient().Installation(r.Context(), input.InstallationID)
	if err != nil {
		writeError(w, http.StatusBadGateway, "github_connection_failed", "GitHub could not verify this installation.")
		return
	}
	orgID := pathUUID(r, "organizationID")
	item, err := a.repo.UpsertGitHubIntegration(r.Context(), orgID, currentUser(r.Context()).ID, installation.ID, installation.Account, requestID(r.Context()), clientIP(r))
	if err != nil {
		a.persistenceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}
func (a *API) disconnectGitHub(w http.ResponseWriter, r *http.Request) {
	if err := a.repo.DisconnectGitHub(r.Context(), pathUUID(r, "organizationID"), currentUser(r.Context()).ID, requestID(r.Context()), clientIP(r)); err != nil {
		a.persistenceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (a *API) listGitHubRepositories(w http.ResponseWriter, r *http.Request) {
	integration, err := a.repo.GitHubIntegration(r.Context(), pathUUID(r, "organizationID"))
	if err != nil {
		a.persistenceError(w, err)
		return
	}
	items, err := a.githubClient().ListRepositories(r.Context(), integration.InstallationID)
	if err != nil {
		writeError(w, http.StatusBadGateway, "github_request_failed", "GitHub repositories could not be loaded.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"repositories": items})
}
func (a *API) getGitSource(w http.ResponseWriter, r *http.Request) {
	item, err := a.repo.GitSource(r.Context(), pathUUID(r, "organizationID"), pathUUID(r, "applicationID"))
	if err != nil {
		a.persistenceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}
func (a *API) updateGitSource(w http.ResponseWriter, r *http.Request) {
	var input struct {
		RepositoryID       int64  `json:"repositoryId"`
		RepositoryFullName string `json:"repositoryFullName"`
		Branch             string `json:"branch"`
		AutoDeploy         bool   `json:"autoDeploy"`
	}
	if !decode(w, r, &input) {
		return
	}
	input.RepositoryFullName = strings.TrimSpace(input.RepositoryFullName)
	input.Branch = strings.TrimSpace(input.Branch)
	if input.RepositoryID < 1 || input.RepositoryFullName == "" || input.Branch == "" || len(input.Branch) > 255 {
		validation(w, "Repository and branch are required.")
		return
	}
	orgID := pathUUID(r, "organizationID")
	integration, err := a.repo.GitHubIntegration(r.Context(), orgID)
	if err != nil {
		a.persistenceError(w, err)
		return
	}
	repos, err := a.githubClient().ListRepositories(r.Context(), integration.InstallationID)
	if err != nil {
		writeError(w, http.StatusBadGateway, "github_request_failed", "GitHub could not verify repository access.")
		return
	}
	verified := false
	for _, repo := range repos {
		if repo.ID == input.RepositoryID && strings.EqualFold(repo.FullName, input.RepositoryFullName) {
			input.RepositoryFullName = repo.FullName
			verified = true
			break
		}
	}
	if !verified {
		writeError(w, http.StatusUnprocessableEntity, "repository_unavailable", "The selected repository is not available to this installation.")
		return
	}
	item, err := a.repo.UpsertGitSource(r.Context(), orgID, pathUUID(r, "applicationID"), currentUser(r.Context()).ID, input.RepositoryID, input.RepositoryFullName, input.Branch, input.AutoDeploy, requestID(r.Context()), clientIP(r))
	if err != nil {
		a.persistenceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (a *API) githubWebhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, maxWebhookBytes+1))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_webhook", "The webhook body could not be read.")
		return
	}
	if len(body) > maxWebhookBytes {
		writeError(w, http.StatusRequestEntityTooLarge, "webhook_too_large", "The webhook body exceeds the allowed size.")
		return
	}
	if !githubprovider.VerifySignature(a.cfg.GitHubWebhookSecret, body, r.Header.Get("X-Hub-Signature-256")) {
		writeError(w, http.StatusUnauthorized, "invalid_signature", "The webhook signature is invalid.")
		return
	}
	delivery := strings.TrimSpace(r.Header.Get("X-GitHub-Delivery"))
	event := strings.TrimSpace(r.Header.Get("X-GitHub-Event"))
	if delivery == "" || len(delivery) > 200 {
		writeError(w, http.StatusBadRequest, "invalid_delivery", "A valid delivery ID is required.")
		return
	}
	if event != "push" {
		writeJSON(w, http.StatusAccepted, map[string]any{"status": "ignored", "reason": "unsupported_event"})
		return
	}
	var payload struct {
		Ref        string `json:"ref"`
		After      string `json:"after"`
		Repository struct {
			ID       int64  `json:"id"`
			FullName string `json:"full_name"`
		} `json:"repository"`
		Installation struct {
			ID int64 `json:"id"`
		} `json:"installation"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_payload", "The webhook payload is invalid.")
		return
	}
	branch, ok := githubprovider.ParseRef(payload.Ref)
	if !ok || !validCommitSHA(payload.After) || payload.Repository.ID < 1 || payload.Repository.FullName == "" || payload.Installation.ID < 1 {
		writeError(w, http.StatusUnprocessableEntity, "invalid_payload", "The push payload is incomplete.")
		return
	}
	if strings.Trim(payload.After, "0") == "" {
		writeJSON(w, http.StatusAccepted, map[string]any{"status": "ignored", "reason": "branch_deleted"})
		return
	}
	items, err := a.repo.AcceptGitHubPush(r.Context(), delivery, payload.Installation.ID, payload.Repository.ID, payload.Repository.FullName, branch, payload.After, store.PayloadDigest(body))
	if errors.Is(err, store.ErrDuplicateDelivery) {
		writeJSON(w, http.StatusOK, map[string]any{"status": "duplicate", "deployments": 0})
		return
	}
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"status": map[bool]string{true: "accepted", false: "ignored"}[len(items) > 0], "deployments": len(items)})
}

func validCommitSHA(value string) bool {
	if len(value) != 40 && len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func (a *API) requireBox(w http.ResponseWriter) (*cryptoenvelope.Box, bool) {
	if a.box == nil {
		writeError(w, http.StatusServiceUnavailable, "encryption_unavailable", "Provider credential encryption is not configured.")
		return nil, false
	}
	return a.box, true
}

func (a *API) getCloudflareIntegration(w http.ResponseWriter, r *http.Request) {
	item, err := a.repo.CloudflareIntegration(r.Context(), pathUUID(r, "organizationID"))
	if err != nil {
		a.persistenceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}
func (a *API) connectCloudflare(w http.ResponseWriter, r *http.Request) {
	var input struct {
		AccountID string `json:"accountId"`
		APIToken  string `json:"apiToken"`
	}
	if !decode(w, r, &input) {
		return
	}
	input.AccountID = strings.TrimSpace(input.AccountID)
	input.APIToken = strings.TrimSpace(input.APIToken)
	if input.AccountID == "" || input.APIToken == "" || len(input.AccountID) > 100 || len(input.APIToken) > 1000 {
		validation(w, "Cloudflare account ID and API token are required.")
		return
	}
	box, ok := a.requireBox(w)
	if !ok {
		return
	}
	client := cfprovider.Client{Token: input.APIToken, BaseURL: a.cfg.CloudflareAPIURL}
	if err := client.TestConnection(r.Context()); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "cloudflare_connection_failed", "Cloudflare rejected the API token.")
		return
	}
	orgID := pathUUID(r, "organizationID")
	encrypted, err := box.Seal([]byte(input.APIToken), "cloudflare:"+orgID.String())
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	item, err := a.repo.UpsertCloudflareIntegration(r.Context(), orgID, currentUser(r.Context()).ID, input.AccountID, encrypted, requestID(r.Context()), clientIP(r))
	if err != nil {
		a.persistenceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (a *API) cloudflareClient(r *http.Request, w http.ResponseWriter, orgID uuid.UUID) (cfprovider.Client, store.CloudflareIntegration, bool) {
	box, ok := a.requireBox(w)
	if !ok {
		return cfprovider.Client{}, store.CloudflareIntegration{}, false
	}
	integration, err := a.repo.CloudflareIntegration(r.Context(), orgID)
	if err != nil {
		a.persistenceError(w, err)
		return cfprovider.Client{}, integration, false
	}
	token, err := box.Open(integration.EncryptedAPIToken, "cloudflare:"+orgID.String())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "credential_unavailable", "The Cloudflare credential could not be decrypted.")
		return cfprovider.Client{}, integration, false
	}
	return cfprovider.Client{Token: string(token), BaseURL: a.cfg.CloudflareAPIURL}, integration, true
}
func (a *API) listCloudflareZones(w http.ResponseWriter, r *http.Request) {
	orgID := pathUUID(r, "organizationID")
	client, integration, ok := a.cloudflareClient(r, w, orgID)
	if !ok {
		return
	}
	zones, err := client.ListZones(r.Context(), integration.AccountID)
	if err != nil {
		_ = a.repo.SetCloudflareHealth(r.Context(), orgID, "error", "zone discovery failed")
		writeError(w, http.StatusBadGateway, "cloudflare_request_failed", "Cloudflare zones could not be loaded.")
		return
	}
	stored := make([]store.CloudflareZone, 0, len(zones))
	for _, zone := range zones {
		stored = append(stored, store.CloudflareZone{OrganizationID: orgID, IntegrationID: integration.ID, ProviderZoneID: zone.ID, Name: zone.Name, Status: zone.Status, Selected: true})
	}
	if err := a.repo.ReplaceCloudflareZones(r.Context(), orgID, integration.ID, stored); err != nil {
		a.serverError(w, r, err)
		return
	}
	_ = a.repo.SetCloudflareHealth(r.Context(), orgID, "connected", "")
	storedZones, err := a.repo.ListCloudflareZones(r.Context(), orgID)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"zones": storedZones})
}

func (a *API) selectCloudflareZone(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Selected bool `json:"selected"`
	}
	if !decode(w, r, &input) {
		return
	}
	orgID := pathUUID(r, "organizationID")
	if err := a.repo.SetCloudflareZoneSelected(r.Context(), orgID, pathUUID(r, "zoneID"), input.Selected); err != nil {
		a.persistenceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (a *API) listDomains(w http.ResponseWriter, r *http.Request) {
	items, err := a.repo.ListDomains(r.Context(), pathUUID(r, "organizationID"))
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"domains": items})
}
func (a *API) createDomain(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Hostname    string `json:"hostname"`
		TargetPort  int    `json:"targetPort"`
		Protocol    string `json:"protocol"`
		RoutingMode string `json:"routingMode"`
	}
	if !decode(w, r, &input) {
		return
	}
	input.Hostname = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(input.Hostname), "."))
	input.Protocol = strings.ToLower(strings.TrimSpace(input.Protocol))
	input.RoutingMode = strings.ToLower(strings.TrimSpace(input.RoutingMode))
	if !validDNSName(input.Hostname) || input.TargetPort < 1 || input.TargetPort > 65535 || !oneOf(input.Protocol, "http", "https", "tcp") || !oneOf(input.RoutingMode, "dns_only", "cloudflare_proxied", "cloudflare_tunnel") {
		validation(w, "Provide a hostname, target port, protocol, and routing mode.")
		return
	}
	if input.Protocol == "tcp" && input.RoutingMode == "cloudflare_proxied" {
		validation(w, "TCP origins require DNS-only or Cloudflare Tunnel routing.")
		return
	}
	orgID := pathUUID(r, "organizationID")
	item, err := a.repo.CreateDomain(r.Context(), orgID, pathUUID(r, "applicationID"), currentUser(r.Context()).ID, input.Hostname, input.TargetPort, input.Protocol, input.RoutingMode, requestID(r.Context()), clientIP(r))
	if err != nil {
		a.persistenceError(w, err)
		return
	}
	if input.RoutingMode == "cloudflare_tunnel" {
		writeJSON(w, http.StatusCreated, item)
		return
	}
	synced, status := a.reconcileDomain(r, w, item)
	if !status {
		return
	}
	writeJSON(w, http.StatusCreated, synced)
}

func validDNSName(value string) bool { return len(value) <= 253 && dnsNamePattern.MatchString(value) }
func (a *API) syncDomain(w http.ResponseWriter, r *http.Request) {
	item, err := a.repo.Domain(r.Context(), pathUUID(r, "organizationID"), pathUUID(r, "domainID"))
	if err != nil {
		a.persistenceError(w, err)
		return
	}
	synced, ok := a.reconcileDomain(r, w, item)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, synced)
}

func (a *API) updateDomain(w http.ResponseWriter, r *http.Request) {
	var input struct {
		TargetPort  int    `json:"targetPort"`
		Protocol    string `json:"protocol"`
		RoutingMode string `json:"routingMode"`
	}
	if !decode(w, r, &input) {
		return
	}
	input.Protocol = strings.ToLower(strings.TrimSpace(input.Protocol))
	input.RoutingMode = strings.ToLower(strings.TrimSpace(input.RoutingMode))
	if input.TargetPort < 1 || input.TargetPort > 65535 || !oneOf(input.Protocol, "http", "https", "tcp") || !oneOf(input.RoutingMode, "dns_only", "cloudflare_proxied", "cloudflare_tunnel") {
		validation(w, "Provide a target port, protocol, and routing mode.")
		return
	}
	if input.Protocol == "tcp" && input.RoutingMode == "cloudflare_proxied" {
		validation(w, "TCP origins require DNS-only or Cloudflare Tunnel routing.")
		return
	}
	orgID := pathUUID(r, "organizationID")
	domainID := pathUUID(r, "domainID")
	if err := a.repo.UpdateDomainDesired(r.Context(), orgID, domainID, currentUser(r.Context()).ID, input.TargetPort, input.Protocol, input.RoutingMode, "A", "", input.RoutingMode == "cloudflare_proxied", requestID(r.Context()), clientIP(r)); err != nil {
		a.persistenceError(w, err)
		return
	}
	domain, err := a.repo.Domain(r.Context(), orgID, domainID)
	if err != nil {
		a.persistenceError(w, err)
		return
	}
	if input.RoutingMode == "cloudflare_tunnel" {
		writeJSON(w, http.StatusOK, domain)
		return
	}
	synced, ok := a.reconcileDomain(r, w, domain)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, synced)
}
func matchingZone(host string, zones []store.CloudflareZone) (store.CloudflareZone, bool) {
	sort.Slice(zones, func(i, j int) bool { return len(zones[i].Name) > len(zones[j].Name) })
	for _, z := range zones {
		zone := strings.ToLower(z.Name)
		if z.Selected && (host == zone || strings.HasSuffix(host, "."+zone)) {
			return z, true
		}
	}
	return store.CloudflareZone{}, false
}
func (a *API) reconcileDomain(r *http.Request, w http.ResponseWriter, domain store.Domain) (store.Domain, bool) {
	if domain.RoutingMode != "cloudflare_tunnel" {
		target, err := a.connections.ResolveOrigin(r.Context(), domain)
		if err != nil {
			_ = a.repo.SetDomainSync(r.Context(), domain.OrganizationID, domain.ID, nil, nil, "error", false, "origin target could not be resolved")
			writeError(w, http.StatusConflict, "origin_unavailable", "The selected application target could not be resolved.")
			return domain, false
		}
		if !target.PublicDNS {
			_ = a.repo.SetDomainSync(r.Context(), domain.OrganizationID, domain.ID, nil, nil, "error", false, "target has no public DNS address")
			writeError(w, http.StatusConflict, "public_origin_required", "This server has no public address. Configure one or use Cloudflare Tunnel.")
			return domain, false
		}
		domain.DNSRecordType = target.DNSRecordType
		domain.DNSContent = target.Address
		domain.Proxied = domain.RoutingMode == "cloudflare_proxied"
		if err := a.repo.SetDomainOriginDesired(r.Context(), domain.OrganizationID, domain.ID, domain.DNSRecordType, domain.DNSContent, domain.Proxied); err != nil {
			a.serverError(w, r, err)
			return domain, false
		}
	}
	client, _, ok := a.cloudflareClient(r, w, domain.OrganizationID)
	if !ok {
		return domain, false
	}
	zones, err := a.repo.ListCloudflareZones(r.Context(), domain.OrganizationID)
	if err != nil {
		a.serverError(w, r, err)
		return domain, false
	}
	zone, found := matchingZone(strings.ToLower(domain.Hostname), zones)
	if !found {
		_ = a.repo.SetDomainSync(r.Context(), domain.OrganizationID, domain.ID, nil, nil, "error", false, "no matching selected Cloudflare zone")
		writeError(w, http.StatusConflict, "zone_not_connected", "No selected Cloudflare zone matches this hostname.")
		return domain, false
	}
	desired := dnsprovider.DesiredRecord{Type: domain.DNSRecordType, Name: domain.Hostname, Content: domain.DNSContent, Proxied: domain.Proxied}
	records, err := client.FindRecords(r.Context(), zone.ProviderZoneID, domain.Hostname)
	if err != nil {
		_ = a.repo.SetDomainSync(r.Context(), domain.OrganizationID, domain.ID, &zone.ProviderZoneID, nil, "error", false, "record lookup failed")
		writeError(w, http.StatusBadGateway, "cloudflare_request_failed", "The DNS record could not be inspected.")
		return domain, false
	}
	var record dnsprovider.Record
	if domain.ProviderRecordID != nil && domain.SiliconManaged {
		ownedRecordPresent := false
		for _, candidate := range records {
			if candidate.ID == *domain.ProviderRecordID {
				ownedRecordPresent = true
				break
			}
		}
		if ownedRecordPresent {
			record, err = client.UpdateRecord(r.Context(), zone.ProviderZoneID, *domain.ProviderRecordID, desired)
		} else if len(records) == 0 {
			record, err = client.CreateRecord(r.Context(), zone.ProviderZoneID, desired)
		} else {
			_ = a.repo.SetDomainSync(r.Context(), domain.OrganizationID, domain.ID, &zone.ProviderZoneID, nil, "conflict", false, "the previously managed record was replaced externally")
			writeError(w, http.StatusConflict, "dns_conflict", "The previously managed DNS record was replaced by an unrelated record.")
			return domain, false
		}
	} else if len(records) == 0 {
		record, err = client.CreateRecord(r.Context(), zone.ProviderZoneID, desired)
	} else {
		_ = a.repo.SetDomainSync(r.Context(), domain.OrganizationID, domain.ID, &zone.ProviderZoneID, nil, "conflict", false, "an unrelated DNS record already exists")
		writeError(w, http.StatusConflict, "dns_conflict", "An existing DNS record is not owned by Silicon.")
		return domain, false
	}
	if err != nil {
		_ = a.repo.SetDomainSync(r.Context(), domain.OrganizationID, domain.ID, &zone.ProviderZoneID, nil, "error", domain.SiliconManaged, "Cloudflare update failed")
		writeError(w, http.StatusBadGateway, "cloudflare_request_failed", "Cloudflare could not reconcile the DNS record.")
		return domain, false
	}
	if err := a.repo.SetDomainSync(r.Context(), domain.OrganizationID, domain.ID, &zone.ProviderZoneID, &record.ID, "active", true, ""); err != nil {
		a.serverError(w, r, err)
		return domain, false
	}
	updated, err := a.repo.Domain(r.Context(), domain.OrganizationID, domain.ID)
	if err != nil {
		a.serverError(w, r, err)
		return domain, false
	}
	return updated, true
}
func (a *API) deleteDomain(w http.ResponseWriter, r *http.Request) {
	orgID := pathUUID(r, "organizationID")
	domain, err := a.repo.Domain(r.Context(), orgID, pathUUID(r, "domainID"))
	if err != nil {
		a.persistenceError(w, err)
		return
	}
	if domain.SiliconManaged && domain.ProviderRecordID != nil && domain.ProviderZoneID != nil {
		client, _, ok := a.cloudflareClient(r, w, orgID)
		if !ok {
			return
		}
		records, err := client.FindRecords(r.Context(), *domain.ProviderZoneID, domain.Hostname)
		if err != nil {
			writeError(w, http.StatusBadGateway, "cloudflare_request_failed", "The DNS record could not be verified before deletion.")
			return
		}
		ownedPresent := false
		for _, record := range records {
			if record.ID == *domain.ProviderRecordID {
				ownedPresent = true
				break
			}
		}
		if ownedPresent {
			if err := client.DeleteRecord(r.Context(), *domain.ProviderZoneID, *domain.ProviderRecordID); err != nil {
				writeError(w, http.StatusBadGateway, "cloudflare_request_failed", "The Silicon-managed DNS record could not be deleted.")
				return
			}
		}
	}
	if err := a.repo.DeleteDomainRecord(r.Context(), orgID, domain.ID, currentUser(r.Context()).ID, requestID(r.Context()), clientIP(r)); err != nil {
		a.persistenceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) listTunnels(w http.ResponseWriter, r *http.Request) {
	items, err := a.repo.ListTunnels(r.Context(), pathUUID(r, "organizationID"))
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tunnels": items})
}
func (a *API) createTunnel(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name             string     `json:"name"`
		ProviderTunnelID string     `json:"providerTunnelId"`
		Ownership        string     `json:"ownership"`
		ServerID         *uuid.UUID `json:"serverId"`
	}
	if !decode(w, r, &input) {
		return
	}
	input.Name = strings.TrimSpace(input.Name)
	input.ProviderTunnelID = strings.TrimSpace(input.ProviderTunnelID)
	if input.Name == "" {
		validation(w, "Tunnel name is required.")
		return
	}
	orgID := pathUUID(r, "organizationID")
	if input.ServerID != nil {
		server, loadErr := a.repo.ServerByID(r.Context(), orgID, *input.ServerID)
		if loadErr != nil || !oneOf(server.ConnectionType, "local", "ssh") {
			writeError(w, http.StatusNotFound, "not_found", "Target server not found.")
			return
		}
	}
	client, integration, ok := a.cloudflareClient(r, w, orgID)
	if !ok {
		return
	}
	providerID := input.ProviderTunnelID
	status := "inactive"
	ownership := input.Ownership
	var token string
	var err error
	if providerID == "" {
		if input.ServerID == nil || *input.ServerID == uuid.Nil {
			validation(w, "A target server is required for a Silicon-managed tunnel.")
			return
		}
		ownership = "silicon"
		created, createdToken, createErr := client.Create(r.Context(), integration.AccountID, input.Name)
		err = createErr
		providerID = created.ID
		status = created.Status
		token = createdToken
	} else {
		if !oneOf(ownership, "imported", "external") {
			validation(w, "Existing tunnels must be marked imported or external.")
			return
		}
		if ownership == "imported" && (input.ServerID == nil || *input.ServerID == uuid.Nil) {
			validation(w, "A target server is required for an imported tunnel whose routes Silicon manages.")
			return
		}
		available, loadErr := client.List(r.Context(), integration.AccountID)
		err = loadErr
		found := false
		for _, t := range available {
			if t.ID == providerID {
				status = t.Status
				found = true
				break
			}
		}
		if err == nil && !found {
			err = errors.New("tunnel not found")
		}
	}
	if err != nil {
		writeError(w, http.StatusBadGateway, "cloudflare_request_failed", "Cloudflare could not create or verify the tunnel.")
		return
	}
	var encrypted []byte
	if token != "" {
		box, _ := a.requireBox(w)
		encrypted, err = box.Seal([]byte(token), "cloudflare-tunnel:"+orgID.String()+":"+providerID)
		if err != nil {
			a.serverError(w, r, err)
			return
		}
	}
	item, err := a.repo.SaveTunnel(r.Context(), orgID, integration.ID, providerID, input.Name, ownership, status, encrypted, input.ServerID)
	if err != nil {
		a.persistenceError(w, err)
		return
	}
	if ownership == "silicon" {
		tokenBytes := []byte(token)
		installErr := a.connections.InstallTunnel(r.Context(), orgID, *input.ServerID, connection.TunnelInstallation{Name: input.Name, Token: tokenBytes})
		for index := range tokenBytes {
			tokenBytes[index] = 0
		}
		if installErr != nil {
			_ = a.repo.SetTunnelInstallation(r.Context(), orgID, item.ID, "error", "cloudflared installation failed")
			writeError(w, http.StatusBadGateway, "tunnel_installation_failed", "The tunnel was created, but cloudflared could not be configured on the target server.")
			return
		}
		if err := a.repo.SetTunnelInstallation(r.Context(), orgID, item.ID, "installed", ""); err != nil {
			a.serverError(w, r, err)
			return
		}
		item, _ = a.repo.Tunnel(r.Context(), orgID, item.ID)
	}
	writeJSON(w, http.StatusCreated, item)
}

func (a *API) installTunnel(w http.ResponseWriter, r *http.Request) {
	organizationID := pathUUID(r, "organizationID")
	tunnel, err := a.repo.Tunnel(r.Context(), organizationID, pathUUID(r, "tunnelID"))
	if err != nil {
		a.persistenceError(w, err)
		return
	}
	if tunnel.Ownership != "silicon" || tunnel.ServerID == nil || len(tunnel.EncryptedTunnelToken) == 0 {
		writeError(w, http.StatusConflict, "tunnel_not_installable", "Only a Silicon-created tunnel with a target server can be installed.")
		return
	}
	box, ok := a.requireBox(w)
	if !ok {
		return
	}
	token, err := box.Open(tunnel.EncryptedTunnelToken, "cloudflare-tunnel:"+organizationID.String()+":"+tunnel.ProviderTunnelID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "credential_unavailable", "The encrypted tunnel credential could not be opened.")
		return
	}
	defer func() {
		for index := range token {
			token[index] = 0
		}
	}()
	if err = a.connections.InstallTunnel(r.Context(), organizationID, *tunnel.ServerID, connection.TunnelInstallation{Name: tunnel.Name, Token: token}); err != nil {
		_ = a.repo.SetTunnelInstallation(r.Context(), organizationID, tunnel.ID, "error", "cloudflared installation failed")
		writeError(w, http.StatusBadGateway, "tunnel_installation_failed", "cloudflared could not be configured on the target server.")
		return
	}
	if err = a.repo.SetTunnelInstallation(r.Context(), organizationID, tunnel.ID, "installed", ""); err != nil {
		a.serverError(w, r, err)
		return
	}
	item, err := a.repo.Tunnel(r.Context(), organizationID, tunnel.ID)
	if err != nil {
		a.persistenceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (a *API) createTunnelRoute(w http.ResponseWriter, r *http.Request) {
	var input struct {
		DomainID uuid.UUID `json:"domainId"`
	}
	if !decode(w, r, &input) {
		return
	}
	if input.DomainID == uuid.Nil {
		validation(w, "Domain is required.")
		return
	}
	orgID := pathUUID(r, "organizationID")
	tunnel, err := a.repo.Tunnel(r.Context(), orgID, pathUUID(r, "tunnelID"))
	if err != nil {
		a.persistenceError(w, err)
		return
	}
	if tunnel.Ownership == "external" {
		writeError(w, http.StatusConflict, "tunnel_externally_managed", "This tunnel is marked externally managed and cannot be changed by Silicon.")
		return
	}
	if tunnel.ServerID == nil || !oneOf(tunnel.InstallationStatus, "installed", "external") {
		writeError(w, http.StatusConflict, "tunnel_not_installed", "The tunnel is not installed on a Silicon-managed target server.")
		return
	}
	domain, err := a.repo.Domain(r.Context(), orgID, input.DomainID)
	if err != nil {
		a.persistenceError(w, err)
		return
	}
	target, err := a.connections.ResolveOrigin(r.Context(), domain)
	if err != nil {
		writeError(w, http.StatusConflict, "origin_unavailable", "The selected application target could not be resolved.")
		return
	}
	if target.ServerID != *tunnel.ServerID || !target.Tunnel {
		writeError(w, http.StatusConflict, "tunnel_target_mismatch", "The tunnel must be installed on the domain target server.")
		return
	}
	serviceURL := target.ServiceURL()
	routes, err := a.repo.TunnelRoutes(r.Context(), orgID, tunnel.ID)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	client, integration, ok := a.cloudflareClient(r, w, orgID)
	if !ok {
		return
	}
	providerRoutes, err := client.Routes(r.Context(), integration.AccountID, tunnel.ProviderTunnelID)
	if err != nil {
		writeError(w, http.StatusBadGateway, "cloudflare_request_failed", "Cloudflare tunnel routes could not be inspected.")
		return
	}
	managedHostname := false
	for _, route := range routes {
		if strings.EqualFold(route.Hostname, domain.Hostname) && route.SiliconManaged {
			managedHostname = true
			break
		}
	}
	replaced := false
	for index, route := range providerRoutes {
		if strings.EqualFold(route.Hostname, domain.Hostname) {
			if !managedHostname {
				writeError(w, http.StatusConflict, "tunnel_route_conflict", "This tunnel hostname is not owned by Silicon.")
				return
			}
			providerRoutes[index] = tunnelprovider.Route{Hostname: domain.Hostname, Service: serviceURL}
			replaced = true
		}
	}
	if !replaced {
		providerRoutes = append(providerRoutes, tunnelprovider.Route{Hostname: domain.Hostname, Service: serviceURL})
	}
	if err := client.ConfigureRoutes(r.Context(), integration.AccountID, tunnel.ProviderTunnelID, providerRoutes); err != nil {
		writeError(w, http.StatusBadGateway, "cloudflare_request_failed", "Cloudflare could not configure the tunnel route.")
		return
	}
	route, err := a.repo.SaveTunnelRoute(r.Context(), orgID, tunnel.ID, domain.ID, domain.Hostname, serviceURL)
	if err != nil {
		a.persistenceError(w, err)
		return
	}
	if err := a.repo.UpdateDomainDesired(r.Context(), orgID, domain.ID, currentUser(r.Context()).ID, domain.TargetPort, domain.Protocol, "cloudflare_tunnel", "CNAME", tunnel.ProviderTunnelID+".cfargotunnel.com", true, requestID(r.Context()), clientIP(r)); err != nil {
		a.persistenceError(w, err)
		return
	}
	domain, _ = a.repo.Domain(r.Context(), orgID, domain.ID)
	if _, ok := a.reconcileDomain(r, w, domain); !ok {
		return
	}
	writeJSON(w, http.StatusCreated, route)
}
