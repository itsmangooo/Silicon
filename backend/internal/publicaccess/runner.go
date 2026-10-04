package publicaccess

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/itsmangooo/Silicon/backend/internal/cryptoenvelope"
	cloudflareprovider "github.com/itsmangooo/Silicon/backend/internal/providers/cloudflare"
	dnsprovider "github.com/itsmangooo/Silicon/backend/internal/providers/dns"
	tunnelprovider "github.com/itsmangooo/Silicon/backend/internal/providers/tunnel"
	"github.com/itsmangooo/Silicon/backend/internal/store"
)

type Provider interface {
	dnsprovider.Provider
	tunnelprovider.Provider
}

type ProviderFactory func(token string) Provider

type OperationStore interface {
	RequeueInterruptedSystemPublicAccessOperations(context.Context) error
	ClaimSystemPublicAccessOperation(context.Context) (store.SystemPublicAccessOperation, error)
	SetSystemPublicAccessOperationStatus(context.Context, uuid.UUID, string, string, string, string, string) error
	SetSystemPublicAccessOperationSnapshot(context.Context, uuid.UUID, string, bool, bool, string) error
	PublicAccessResources(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, uuid.UUID) (store.PublicAccessResources, error)
	SystemPublicAccess(context.Context) (store.SystemPublicAccess, error)
	ActivateSystemPublicAccess(context.Context, store.SystemPublicAccessOperation, string, string, string, bool, bool, string) error
	DeleteSystemPublicAccess(context.Context) error
}

type DesiredHostConfig struct {
	PublicURL           string
	CookieSecure        bool
	TrustForwardedProto bool
	BindAddress         string
}

type HostConfig struct {
	DesiredHostConfig
	HTTPPort string
}

type HostApplier interface {
	Current(context.Context) (HostConfig, error)
	Apply(context.Context, DesiredHostConfig, func(string, string) error) (HostConfig, error)
}

type Runner struct {
	Repository      OperationStore
	Box             cryptoenvelope.Box
	ProviderFactory ProviderFactory
	Host            HostApplier
	Logger          *slog.Logger
	PollEvery       time.Duration
}

func (r Runner) Run(ctx context.Context) {
	delay := r.PollEvery
	if delay <= 0 {
		delay = 3 * time.Second
	}
	if err := r.Repository.RequeueInterruptedSystemPublicAccessOperations(ctx); err != nil {
		r.logger().Error("could not recover public access operations", "error", err)
	}
	for {
		processed, err := r.ProcessOne(ctx)
		if err != nil && !errors.Is(err, context.Canceled) {
			r.logger().Error("public access operation failed", "error", err)
		}
		if processed {
			continue
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}
	}
}

func (r Runner) ProcessOne(ctx context.Context) (bool, error) {
	op, err := r.Repository.ClaimSystemPublicAccessOperation(ctx)
	if errors.Is(err, store.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	stage := "validating"
	fail := func(cause error) (bool, error) {
		message := sanitizeError(cause)
		_ = r.Repository.SetSystemPublicAccessOperationStatus(context.WithoutCancel(ctx), op.ID, "failed", "Public access failed safely. "+message, stage, "", "")
		return true, cause
	}
	if op.Action == "disable" {
		if err = r.disable(ctx, op, &stage); err != nil {
			return fail(err)
		}
		return true, nil
	}
	if err = r.configure(ctx, op, &stage); err != nil {
		return fail(err)
	}
	return true, nil
}

func (r Runner) configure(ctx context.Context, op store.SystemPublicAccessOperation, stage *string) error {
	if op.OrganizationID == nil || op.IntegrationID == nil || op.ZoneID == nil || op.TunnelID == nil {
		return errors.New("public access resources are incomplete")
	}
	resources, err := r.Repository.PublicAccessResources(ctx, *op.OrganizationID, *op.IntegrationID, *op.ZoneID, *op.TunnelID)
	if err != nil {
		return fmt.Errorf("load scoped Cloudflare resources: %w", err)
	}
	hostname, err := NormalizeHostname(op.Hostname)
	if err != nil {
		return err
	}
	if !HostnameInZone(hostname, resources.Zone.Name) {
		return fmt.Errorf("hostname %s does not belong to zone %s", hostname, resources.Zone.Name)
	}
	if resources.Integration.Status != "connected" || resources.Zone.Status != "active" {
		return errors.New("selected Cloudflare connection or zone is not active")
	}
	if resources.Tunnel.InstallationStatus != "installed" || resources.TunnelServerConnectionType != "local" {
		return errors.New("selected tunnel must be installed on the local Silicon host")
	}
	token, err := r.Box.Open(resources.Integration.EncryptedAPIToken, "cloudflare:"+resources.Integration.OrganizationID.String())
	if err != nil {
		return errors.New("Cloudflare credential could not be decrypted")
	}
	provider := r.ProviderFactory(string(token))
	if err = provider.TestConnection(ctx); err != nil {
		return fmt.Errorf("verify Cloudflare connection: %w", err)
	}
	tunnels, err := provider.List(ctx, resources.Integration.AccountID)
	if err != nil {
		return fmt.Errorf("verify Cloudflare tunnel: %w", err)
	}
	found := false
	for _, t := range tunnels {
		if t.ID == resources.Tunnel.ProviderTunnelID {
			found = true
			break
		}
	}
	if !found {
		return errors.New("selected Cloudflare tunnel is unavailable")
	}

	active, activeErr := r.Repository.SystemPublicAccess(ctx)
	hasActive := activeErr == nil
	if activeErr != nil && !errors.Is(activeErr, store.ErrNotFound) {
		return activeErr
	}
	previous := HostConfig{}
	if op.PreviousPublicURL == "" || op.PreviousCookieSecure == nil || op.PreviousTrustForwardedProto == nil || op.PreviousBindAddress == nil {
		previous, err = r.Host.Current(ctx)
		if err != nil {
			return err
		}
		if err = r.Repository.SetSystemPublicAccessOperationSnapshot(ctx, op.ID, previous.PublicURL, previous.CookieSecure, previous.TrustForwardedProto, previous.BindAddress); err != nil {
			return err
		}
	} else {
		previous = HostConfig{DesiredHostConfig: DesiredHostConfig{PublicURL: op.PreviousPublicURL, CookieSecure: *op.PreviousCookieSecure, TrustForwardedProto: *op.PreviousTrustForwardedProto, BindAddress: *op.PreviousBindAddress}}
		current, currentErr := r.Host.Current(ctx)
		if currentErr != nil {
			return currentErr
		}
		previous.HTTPPort = current.HTTPPort
	}
	*stage = "configuring_cloudflare"
	if err = r.Repository.SetSystemPublicAccessOperationStatus(ctx, op.ID, *stage, "Preparing the Cloudflare Tunnel hostname route.", "", "", ""); err != nil {
		return err
	}
	originalRoutes, err := provider.Routes(ctx, resources.Integration.AccountID, resources.Tunnel.ProviderTunnelID)
	if err != nil {
		return fmt.Errorf("read existing tunnel routes: %w", err)
	}
	service := ""
	rollbackRoutes := originalRoutes
	if op.ProviderRecordID != "" && !(hasActive && strings.EqualFold(active.Hostname, hostname) && active.ProviderRecordID == op.ProviderRecordID) {
		rollbackRoutes = withoutRoute(originalRoutes, hostname)
	}
	port, err := strconv.Atoi(previous.HTTPPort)
	if err != nil || port < 1 || port > 65535 {
		return errors.New("configured SILICON_HTTP_PORT is invalid")
	}
	service = "http://127.0.0.1:" + previous.HTTPPort
	for _, route := range originalRoutes {
		if strings.EqualFold(route.Hostname, hostname) && !(hasActive && strings.EqualFold(active.Hostname, hostname) && active.TunnelID == resources.Tunnel.ID) && op.ProviderRecordID == "" {
			return errors.New("hostname is already owned by an unrelated tunnel route")
		}
	}
	desiredRoutes := replaceRoute(originalRoutes, hostname, service)
	if err = provider.ConfigureRoutes(ctx, resources.Integration.AccountID, resources.Tunnel.ProviderTunnelID, desiredRoutes); err != nil {
		return fmt.Errorf("configure tunnel route: %w", err)
	}
	createdRecord := ""
	if op.ProviderRecordID != "" && (!hasActive || op.ProviderRecordID != active.ProviderRecordID) {
		createdRecord = op.ProviderRecordID
	}
	rollbackCloudflare := func() {
		_ = provider.ConfigureRoutes(context.WithoutCancel(ctx), resources.Integration.AccountID, resources.Tunnel.ProviderTunnelID, rollbackRoutes)
		if createdRecord != "" {
			_ = provider.DeleteRecord(context.WithoutCancel(ctx), resources.Zone.ProviderZoneID, createdRecord)
		}
	}
	records, err := provider.FindRecords(ctx, resources.Zone.ProviderZoneID, hostname)
	if err != nil {
		rollbackCloudflare()
		return fmt.Errorf("check DNS record: %w", err)
	}
	target := resources.Tunnel.ProviderTunnelID + ".cfargotunnel.com"
	recordID := ""
	if len(records) > 0 {
		if !(hasActive && strings.EqualFold(active.Hostname, hostname) && active.ProviderRecordID == records[0].ID) && op.ProviderRecordID != records[0].ID {
			rollbackCloudflare()
			return errors.New("hostname already has an unrelated Cloudflare DNS record")
		}
		updated, updateErr := provider.UpdateRecord(ctx, resources.Zone.ProviderZoneID, records[0].ID, dnsprovider.DesiredRecord{Type: "CNAME", Name: hostname, Content: target, Proxied: true})
		if updateErr != nil {
			rollbackCloudflare()
			return fmt.Errorf("update DNS record: %w", updateErr)
		}
		recordID = updated.ID
	} else {
		created, createErr := provider.CreateRecord(ctx, resources.Zone.ProviderZoneID, dnsprovider.DesiredRecord{Type: "CNAME", Name: hostname, Content: target, Proxied: true})
		if createErr != nil {
			rollbackCloudflare()
			return fmt.Errorf("create DNS record: %w", createErr)
		}
		recordID = created.ID
		createdRecord = created.ID
	}
	verifiedRoutes, verifyErr := provider.Routes(ctx, resources.Integration.AccountID, resources.Tunnel.ProviderTunnelID)
	if verifyErr != nil || !hasRoute(verifiedRoutes, hostname, service) {
		rollbackCloudflare()
		if verifyErr != nil {
			return fmt.Errorf("verify tunnel route: %w", verifyErr)
		}
		return errors.New("Cloudflare did not retain the requested tunnel route")
	}
	verifiedRecords, verifyErr := provider.FindRecords(ctx, resources.Zone.ProviderZoneID, hostname)
	if verifyErr != nil || !hasRecord(verifiedRecords, recordID, hostname, target) {
		rollbackCloudflare()
		if verifyErr != nil {
			return fmt.Errorf("verify DNS record: %w", verifyErr)
		}
		return errors.New("Cloudflare did not retain the requested DNS record")
	}
	*stage = "updating_configuration"
	if err = r.Repository.SetSystemPublicAccessOperationStatus(ctx, op.ID, *stage, "Updating the approved Silicon public URL settings.", "", recordID, service); err != nil {
		rollbackCloudflare()
		return err
	}
	_, err = r.Host.Apply(ctx, DesiredHostConfig{PublicURL: "https://" + hostname, CookieSecure: true, TrustForwardedProto: true, BindAddress: "127.0.0.1"}, func(s, m string) error {
		*stage = s
		return r.Repository.SetSystemPublicAccessOperationStatus(ctx, op.ID, s, m, "", recordID, service)
	})
	if err != nil {
		rollbackCloudflare()
		return err
	}
	if err = r.Repository.ActivateSystemPublicAccess(ctx, op, recordID, service, previous.PublicURL, previous.CookieSecure, previous.TrustForwardedProto, previous.BindAddress); err != nil {
		_, _ = r.Host.Apply(context.WithoutCancel(ctx), previous.DesiredHostConfig, func(string, string) error { return nil })
		rollbackCloudflare()
		return fmt.Errorf("persist active public access: %w", err)
	}
	// Only after the new URL is active, remove the previous Silicon-owned route and record.
	if hasActive && (!strings.EqualFold(active.Hostname, hostname) || active.TunnelID != resources.Tunnel.ID) {
		if oldResources, oldErr := r.Repository.PublicAccessResources(ctx, active.OrganizationID, active.IntegrationID, active.ZoneID, active.TunnelID); oldErr == nil {
			if oldToken, openErr := r.Box.Open(oldResources.Integration.EncryptedAPIToken, "cloudflare:"+active.OrganizationID.String()); openErr == nil {
				oldProvider := r.ProviderFactory(string(oldToken))
				if routes, routeErr := oldProvider.Routes(ctx, oldResources.Integration.AccountID, oldResources.Tunnel.ProviderTunnelID); routeErr == nil {
					_ = oldProvider.ConfigureRoutes(ctx, oldResources.Integration.AccountID, oldResources.Tunnel.ProviderTunnelID, withoutRoute(routes, active.Hostname))
				}
				if active.ProviderRecordID != "" {
					_ = oldProvider.DeleteRecord(ctx, oldResources.Zone.ProviderZoneID, active.ProviderRecordID)
				}
			}
		}
	}
	return r.Repository.SetSystemPublicAccessOperationStatus(context.WithoutCancel(ctx), op.ID, "active", "Public access is active. The browser can reconnect at https://"+hostname+".", "", recordID, service)
}

func (r Runner) disable(ctx context.Context, op store.SystemPublicAccessOperation, stage *string) error {
	active, err := r.Repository.SystemPublicAccess(ctx)
	if err != nil {
		return err
	}
	resources, err := r.Repository.PublicAccessResources(ctx, active.OrganizationID, active.IntegrationID, active.ZoneID, active.TunnelID)
	if err != nil {
		return err
	}
	*stage = "updating_configuration"
	if err = r.Repository.SetSystemPublicAccessOperationStatus(ctx, op.ID, *stage, "Restoring the previous local access settings.", "", "", active.LocalOrigin); err != nil {
		return err
	}
	_, err = r.Host.Apply(ctx, DesiredHostConfig{PublicURL: active.PreviousPublicURL, CookieSecure: active.PreviousCookieSecure, TrustForwardedProto: active.PreviousTrustForwardedProto, BindAddress: active.PreviousBindAddress}, func(s, m string) error {
		*stage = s
		return r.Repository.SetSystemPublicAccessOperationStatus(ctx, op.ID, s, m, "", "", active.LocalOrigin)
	})
	if err != nil {
		return err
	}
	token, err := r.Box.Open(resources.Integration.EncryptedAPIToken, "cloudflare:"+active.OrganizationID.String())
	if err != nil {
		return err
	}
	provider := r.ProviderFactory(string(token))
	*stage = "configuring_cloudflare"
	routes, err := provider.Routes(ctx, resources.Integration.AccountID, resources.Tunnel.ProviderTunnelID)
	if err == nil {
		err = provider.ConfigureRoutes(ctx, resources.Integration.AccountID, resources.Tunnel.ProviderTunnelID, withoutRoute(routes, active.Hostname))
	}
	if err == nil && active.ProviderRecordID != "" {
		err = provider.DeleteRecord(ctx, resources.Zone.ProviderZoneID, active.ProviderRecordID)
	}
	if err != nil {
		_, _ = r.Host.Apply(context.WithoutCancel(ctx), DesiredHostConfig{PublicURL: "https://" + active.Hostname, CookieSecure: true, TrustForwardedProto: true, BindAddress: "127.0.0.1"}, func(string, string) error { return nil })
		return fmt.Errorf("remove owned Cloudflare route: %w", err)
	}
	if err = r.Repository.DeleteSystemPublicAccess(ctx); err != nil {
		return err
	}
	return r.Repository.SetSystemPublicAccessOperationStatus(context.WithoutCancel(ctx), op.ID, "disabled", "Public access was disabled; the shared tunnel and unrelated routes were preserved.", "", "", active.LocalOrigin)
}

var labelPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$`)

func NormalizeHostname(value string) (string, error) {
	value = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(value), "."))
	if len(value) < 1 || len(value) > 253 || net.ParseIP(value) != nil {
		return "", errors.New("custom domain must be a valid DNS hostname")
	}
	for _, label := range strings.Split(value, ".") {
		if !labelPattern.MatchString(label) {
			return "", errors.New("custom domain must be a valid DNS hostname")
		}
	}
	if !strings.Contains(value, ".") {
		return "", errors.New("custom domain must contain a DNS suffix")
	}
	return value, nil
}
func HostnameInZone(hostname, zone string) bool {
	zone = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(zone), "."))
	return hostname == zone || strings.HasSuffix(hostname, "."+zone)
}
func replaceRoute(routes []tunnelprovider.Route, hostname, service string) []tunnelprovider.Route {
	result := withoutRoute(routes, hostname)
	return append(result, tunnelprovider.Route{Hostname: hostname, Service: service})
}
func withoutRoute(routes []tunnelprovider.Route, hostname string) []tunnelprovider.Route {
	result := make([]tunnelprovider.Route, 0, len(routes))
	for _, route := range routes {
		if !strings.EqualFold(route.Hostname, hostname) {
			result = append(result, route)
		}
	}
	return result
}
func hasRoute(routes []tunnelprovider.Route, hostname, service string) bool {
	for _, route := range routes {
		if strings.EqualFold(route.Hostname, hostname) && route.Service == service {
			return true
		}
	}
	return false
}
func hasRecord(records []dnsprovider.Record, id, hostname, target string) bool {
	for _, record := range records {
		if record.ID == id && strings.EqualFold(record.Name, hostname) && strings.EqualFold(record.Type, "CNAME") && strings.EqualFold(record.Content, target) && record.Proxied {
			return true
		}
	}
	return false
}
func sanitizeError(err error) string {
	value := strings.NewReplacer("\r", " ", "\n", " ").Replace(err.Error())
	if len(value) > 400 {
		value = value[:400]
	}
	return value
}
func (r Runner) logger() *slog.Logger {
	if r.Logger != nil {
		return r.Logger
	}
	return slog.Default()
}

func DefaultProviderFactory(baseURL string) ProviderFactory {
	return func(token string) Provider { return cloudflareprovider.Client{Token: token, BaseURL: baseURL} }
}

// ParseEnv is exported for focused safety tests and never returns secret values to an API.
func ParseEnv(raw []byte) map[string]string {
	values := map[string]string{}
	scanner := bufio.NewScanner(strings.NewReader(string(raw)))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		name, value, ok := strings.Cut(line, "=")
		if ok {
			values[name] = value
		}
	}
	return values
}
