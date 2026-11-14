package httpapi

import (
	"errors"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/google/uuid"
	cfprovider "github.com/itsmangooo/Silicon/backend/internal/providers/cloudflare"
	"github.com/itsmangooo/Silicon/backend/internal/store"
)

func (a *API) verifyDomain(w http.ResponseWriter, r *http.Request) {
	orgID := pathUUID(r, "organizationID")
	domain, err := a.repo.Domain(r.Context(), orgID, pathUUID(r, "domainID"))
	if err != nil {
		a.persistenceError(w, err)
		return
	}
	client, integration, ok := a.cloudflareClient(r, w, orgID)
	if !ok {
		return
	}
	dnsState, routeState, err := a.checkProviderDomain(r, client, integration, domain)
	if err != nil {
		writeError(w, http.StatusBadGateway, "cloudflare_request_failed", "Cloudflare could not verify the configured records.")
		return
	}
	// This check performs DNS resolution only. It never opens a connection to
	// an arbitrary hostname supplied by an organization.
	resolved := false
	if values, err := net.DefaultResolver.LookupIPAddr(r.Context(), domain.Hostname); err == nil {
		resolved = len(values) > 0
	}
	writeJSON(w, http.StatusOK, map[string]any{"dnsMatch": dnsState, "tunnelMatch": routeState, "resolves": resolved})
}

func (a *API) checkProviderDomain(r *http.Request, client cfprovider.Client, integration store.CloudflareIntegration, domain store.Domain) (bool, *bool, error) {
	var tunnelMatch *bool
	if domain.RoutingMode == "cloudflare_tunnel" {
		match := false
		tunnelMatch = &match
		route, err := a.repo.TunnelRouteByDomain(r.Context(), domain.OrganizationID, domain.ID)
		if err == nil {
			tunnel, err := a.repo.Tunnel(r.Context(), domain.OrganizationID, route.TunnelID)
			if err != nil {
				return false, tunnelMatch, err
			}
			if tunnel.IntegrationID != integration.ID {
				return false, tunnelMatch, errors.New("cloudflare integration mismatch")
			}
			routes, err := client.Routes(r.Context(), integration.AccountID, tunnel.ProviderTunnelID)
			if err != nil {
				return false, tunnelMatch, err
			}
			count := 0
			for _, current := range routes {
				if strings.EqualFold(current.Hostname, domain.Hostname) {
					count++
					match = current.Service == route.ServiceURL && current.Path == ""
				}
			}
			match = count == 1 && match
		} else if !errors.Is(err, store.ErrNotFound) {
			return false, tunnelMatch, err
		}
	}
	if domain.ProviderZoneID == nil || domain.ProviderRecordID == nil {
		return false, tunnelMatch, nil
	}
	records, err := client.FindRecords(r.Context(), *domain.ProviderZoneID, domain.Hostname)
	if err != nil {
		return false, tunnelMatch, err
	}
	for _, record := range records {
		if record.ID == *domain.ProviderRecordID && record.Type == domain.DNSRecordType && strings.EqualFold(strings.TrimSuffix(record.Content, "."), strings.TrimSuffix(domain.DNSContent, ".")) && record.Proxied == domain.Proxied {
			return true, tunnelMatch, nil
		}
	}
	return false, tunnelMatch, nil
}

func (a *API) inspectExistingDomain(w http.ResponseWriter, r *http.Request) {
	a.existingTunnelDomain(w, r, false)
}

func (a *API) adoptExistingDomain(w http.ResponseWriter, r *http.Request) {
	a.existingTunnelDomain(w, r, true)
}

func (a *API) existingTunnelDomain(w http.ResponseWriter, r *http.Request, adopt bool) {
	var input struct {
		ApplicationID uuid.UUID `json:"applicationId"`
		TunnelID      uuid.UUID `json:"tunnelId"`
		Hostname      string    `json:"hostname"`
	}
	if !decode(w, r, &input) {
		return
	}
	host := strings.ToLower(strings.TrimSuffix(strings.TrimSpace(input.Hostname), "."))
	if !validDNSName(host) || input.ApplicationID == uuid.Nil || input.TunnelID == uuid.Nil {
		validation(w, "Provide a hostname, application, and tunnel.")
		return
	}
	if !a.allowApplicationHostname(w, r, host) {
		return
	}
	orgID := pathUUID(r, "organizationID")
	app, err := a.repo.ApplicationByID(r.Context(), orgID, input.ApplicationID)
	if err != nil {
		a.persistenceError(w, err)
		return
	}
	tunnel, err := a.repo.Tunnel(r.Context(), orgID, input.TunnelID)
	if err != nil {
		a.persistenceError(w, err)
		return
	}
	if app.ServerID == nil || tunnel.ServerID == nil || *app.ServerID != *tunnel.ServerID || tunnel.Ownership == "external" || !oneOf(tunnel.InstallationStatus, "installed", "external") {
		writeError(w, http.StatusConflict, "tunnel_target_mismatch", "Select a manageable tunnel on the application's server.")
		return
	}
	instance, err := a.repo.CurrentRuntimeInstance(r.Context(), orgID, app.ID)
	if err != nil {
		a.persistenceError(w, err)
		return
	}
	if instance.ServerID == nil || *instance.ServerID != *app.ServerID || instance.HostPort == nil || *instance.HostPort < 1 || instance.RemovedAt != nil || instance.State != "running" || instance.HostAddress == nil || !oneOf(*instance.HostAddress, "127.0.0.1", "0.0.0.0") {
		writeError(w, http.StatusConflict, "origin_unavailable", "Adoption requires a running Silicon deployment with a loopback-reachable published port.")
		return
	}
	client, integration, ok := a.cloudflareClient(r, w, orgID)
	if !ok {
		return
	}
	if tunnel.IntegrationID != integration.ID {
		writeError(w, http.StatusConflict, "integration_mismatch", "The selected tunnel belongs to a different Cloudflare connection.")
		return
	}
	zones, err := a.repo.ListCloudflareZones(r.Context(), orgID)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	zone, ok := matchingZone(host, zones)
	if !ok {
		writeError(w, http.StatusConflict, "zone_not_connected", "No selected Cloudflare zone matches this hostname.")
		return
	}
	routes, err := client.Routes(r.Context(), integration.AccountID, tunnel.ProviderTunnelID)
	if err != nil {
		writeError(w, http.StatusBadGateway, "cloudflare_request_failed", "Cloudflare tunnel routes could not be inspected.")
		return
	}
	port := strconv.Itoa(*instance.HostPort)
	var service string
	matches := 0
	for _, route := range routes {
		if strings.EqualFold(route.Hostname, host) {
			if route.Path != "" {
				writeError(w, http.StatusConflict, "tunnel_route_mismatch", "Path-specific tunnel routes require separate management.")
				return
			}
			service = route.Service
			matches++
		}
	}
	if matches != 1 || (service != "http://127.0.0.1:"+port && service != "http://localhost:"+port) {
		writeError(w, http.StatusConflict, "tunnel_route_mismatch", "The existing hostname must route directly to this deployment's published loopback port.")
		return
	}
	records, err := client.FindRecords(r.Context(), zone.ProviderZoneID, host)
	if err != nil {
		writeError(w, http.StatusBadGateway, "cloudflare_request_failed", "Cloudflare DNS could not be inspected.")
		return
	}
	content := tunnel.ProviderTunnelID + ".cfargotunnel.com"
	if len(records) != 1 || records[0].Type != "CNAME" || !records[0].Proxied || !strings.EqualFold(strings.TrimSuffix(records[0].Content, "."), content) {
		writeError(w, http.StatusConflict, "dns_target_mismatch", "The hostname must have one proxied CNAME pointing to the selected tunnel.")
		return
	}
	if !adopt {
		writeJSON(w, http.StatusOK, map[string]any{"hostname": host, "service": service, "zone": zone.Name, "dnsTarget": content})
		return
	}
	domain, err := a.repo.AdoptTunnelDomain(r.Context(), store.Domain{
		OrganizationID: orgID, ApplicationID: app.ID, Hostname: host, TargetPort: *instance.HostPort,
		Protocol: "http", TargetServerID: app.ServerID, TargetRuntimeInstanceID: &instance.ID,
		ProviderZoneID: &zone.ProviderZoneID, ProviderRecordID: &records[0].ID, DNSContent: content,
	}, tunnel.ID, currentUser(r.Context()).ID, requestID(r.Context()), service, clientIP(r))
	if errors.Is(err, store.ErrConflict) {
		writeError(w, http.StatusConflict, "domain_already_managed", "This provider record is already managed by Silicon.")
		return
	}
	if err != nil {
		a.persistenceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, domain)
}

func (a *API) installationHostname(r *http.Request, host string) (bool, error) {
	if publicURL, err := url.Parse(a.cfg.PublicURL); err == nil && strings.EqualFold(publicURL.Hostname(), host) {
		return true, nil
	}
	active, err := a.repo.SystemPublicAccess(r.Context())
	if errors.Is(err, store.ErrNotFound) {
		return false, nil
	}
	return strings.EqualFold(active.Hostname, host), err
}

func (a *API) allowApplicationHostname(w http.ResponseWriter, r *http.Request, host string) bool {
	reserved, err := a.installationHostname(r, host)
	if err != nil {
		a.serverError(w, r, err)
		return false
	}
	if reserved {
		writeError(w, http.StatusConflict, "installation_hostname", "This hostname is reserved for the Silicon installation.")
		return false
	}
	return true
}
