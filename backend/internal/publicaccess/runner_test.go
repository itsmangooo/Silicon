package publicaccess

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/itsmangooo/Silicon/backend/internal/cryptoenvelope"
	dnsprovider "github.com/itsmangooo/Silicon/backend/internal/providers/dns"
	tunnelprovider "github.com/itsmangooo/Silicon/backend/internal/providers/tunnel"
	"github.com/itsmangooo/Silicon/backend/internal/store"
)

type fakeStore struct {
	op        store.SystemPublicAccessOperation
	resources store.PublicAccessResources
	active    store.SystemPublicAccess
	hasActive bool
	activated bool
	deleted   bool
	statuses  []string
}

func (f *fakeStore) RequeueInterruptedSystemPublicAccessOperations(context.Context) error { return nil }
func (f *fakeStore) ClaimSystemPublicAccessOperation(context.Context) (store.SystemPublicAccessOperation, error) {
	if f.op.ID == uuid.Nil {
		return store.SystemPublicAccessOperation{}, store.ErrNotFound
	}
	op := f.op
	f.op.ID = uuid.Nil
	return op, nil
}
func (f *fakeStore) SetSystemPublicAccessOperationStatus(_ context.Context, _ uuid.UUID, status, _, _, record, origin string) error {
	f.statuses = append(f.statuses, status)
	if record != "" {
		f.op.ProviderRecordID = record
	}
	if origin != "" {
		f.op.LocalOrigin = origin
	}
	return nil
}
func (f *fakeStore) SetSystemPublicAccessOperationSnapshot(_ context.Context, _ uuid.UUID, url string, secure, forward bool, bind string) error {
	f.op.PreviousPublicURL = url
	f.op.PreviousCookieSecure = &secure
	f.op.PreviousTrustForwardedProto = &forward
	f.op.PreviousBindAddress = &bind
	return nil
}
func (f *fakeStore) PublicAccessResources(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, uuid.UUID) (store.PublicAccessResources, error) {
	return f.resources, nil
}
func (f *fakeStore) SystemPublicAccess(context.Context) (store.SystemPublicAccess, error) {
	if !f.hasActive {
		return store.SystemPublicAccess{}, store.ErrNotFound
	}
	return f.active, nil
}
func (f *fakeStore) ActivateSystemPublicAccess(_ context.Context, _ store.SystemPublicAccessOperation, record, origin, previous string, secure, forward bool, bind string) error {
	f.activated = true
	f.active.ProviderRecordID = record
	f.active.LocalOrigin = origin
	f.active.PreviousPublicURL = previous
	f.active.PreviousCookieSecure = secure
	f.active.PreviousTrustForwardedProto = forward
	f.active.PreviousBindAddress = bind
	f.hasActive = true
	return nil
}
func (f *fakeStore) DeleteSystemPublicAccess(context.Context) error {
	f.deleted = true
	f.hasActive = false
	return nil
}

type fakeProvider struct {
	routes        []tunnelprovider.Route
	records       []dnsprovider.Record
	configured    [][]tunnelprovider.Route
	deleted       []string
	failConfigure bool
}

func (f *fakeProvider) TestConnection(context.Context) error { return nil }
func (f *fakeProvider) ListZones(context.Context, string) ([]dnsprovider.Zone, error) {
	return nil, nil
}
func (f *fakeProvider) FindRecords(_ context.Context, _ string, name string) ([]dnsprovider.Record, error) {
	result := []dnsprovider.Record{}
	for _, record := range f.records {
		if record.Name == name {
			result = append(result, record)
		}
	}
	return result, nil
}
func (f *fakeProvider) CreateRecord(_ context.Context, zone string, d dnsprovider.DesiredRecord) (dnsprovider.Record, error) {
	record := dnsprovider.Record{ID: "record-new", ZoneID: zone, Type: d.Type, Name: d.Name, Content: d.Content, Proxied: d.Proxied}
	f.records = append(f.records, record)
	return record, nil
}
func (f *fakeProvider) UpdateRecord(_ context.Context, zone, id string, d dnsprovider.DesiredRecord) (dnsprovider.Record, error) {
	updated := dnsprovider.Record{ID: id, ZoneID: zone, Type: d.Type, Name: d.Name, Content: d.Content, Proxied: d.Proxied}
	for index, record := range f.records {
		if record.ID == id {
			f.records[index] = updated
			return updated, nil
		}
	}
	return updated, nil
}
func (f *fakeProvider) DeleteRecord(_ context.Context, _, id string) error {
	f.deleted = append(f.deleted, id)
	remaining := f.records[:0]
	for _, record := range f.records {
		if record.ID != id {
			remaining = append(remaining, record)
		}
	}
	f.records = remaining
	return nil
}
func (f *fakeProvider) List(context.Context, string) ([]tunnelprovider.Tunnel, error) {
	return []tunnelprovider.Tunnel{{ID: "provider-tunnel", Status: "healthy"}}, nil
}
func (f *fakeProvider) Create(context.Context, string, string) (tunnelprovider.Tunnel, string, error) {
	return tunnelprovider.Tunnel{}, "", nil
}
func (f *fakeProvider) Routes(context.Context, string, string) ([]tunnelprovider.Route, error) {
	return append([]tunnelprovider.Route(nil), f.routes...), nil
}
func (f *fakeProvider) ConfigureRoutes(_ context.Context, _, _ string, routes []tunnelprovider.Route) error {
	if f.failConfigure {
		return errors.New("cloudflare failed")
	}
	copyRoutes := append([]tunnelprovider.Route(nil), routes...)
	f.configured = append(f.configured, copyRoutes)
	f.routes = copyRoutes
	return nil
}

type fakeHost struct {
	current HostConfig
	applied []DesiredHostConfig
	fail    bool
}

func (f *fakeHost) Current(context.Context) (HostConfig, error) { return f.current, nil }
func (f *fakeHost) Apply(_ context.Context, d DesiredHostConfig, progress func(string, string) error) (HostConfig, error) {
	f.applied = append(f.applied, d)
	if f.fail {
		return f.current, errors.New("health failed")
	}
	_ = progress("restarting", "restart")
	_ = progress("waiting_for_health", "health")
	previous := f.current
	f.current.DesiredHostConfig = d
	return previous, nil
}

func testRunner(t *testing.T) (Runner, *fakeStore, *fakeProvider, *fakeHost) {
	t.Helper()
	box, err := cryptoenvelope.New(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	org, integration, zone, tunnel := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	token, err := box.Seal([]byte("token"), "cloudflare:"+org.String())
	if err != nil {
		t.Fatal(err)
	}
	operation := store.SystemPublicAccessOperation{ID: uuid.New(), Action: "configure", OrganizationID: &org, IntegrationID: &integration, ZoneID: &zone, TunnelID: &tunnel, Hostname: "silicon.example.com"}
	repository := &fakeStore{op: operation, resources: store.PublicAccessResources{Integration: store.CloudflareIntegration{ID: integration, OrganizationID: org, AccountID: "account", Status: "connected", EncryptedAPIToken: token}, Zone: store.CloudflareZone{ID: zone, OrganizationID: org, IntegrationID: integration, ProviderZoneID: "provider-zone", Name: "example.com", Status: "active"}, Tunnel: store.CloudflareTunnel{ID: tunnel, OrganizationID: org, IntegrationID: integration, ProviderTunnelID: "provider-tunnel", InstallationStatus: "installed"}, TunnelServerConnectionType: "local"}}
	provider := &fakeProvider{routes: []tunnelprovider.Route{{Hostname: "grafana.example.com", Service: "http://127.0.0.1:3001"}}}
	host := &fakeHost{current: HostConfig{DesiredHostConfig: DesiredHostConfig{PublicURL: "http://192.0.2.10", BindAddress: "0.0.0.0"}, HTTPPort: "8080"}}
	runner := Runner{Repository: repository, Box: box, ProviderFactory: func(string) Provider { return provider }, Host: host}
	return runner, repository, provider, host
}

func TestHostnameValidationAndZoneMatching(t *testing.T) {
	for _, invalid := range []string{"localhost", "https://example.com", "bad_.example.com", "127.0.0.1"} {
		if _, err := NormalizeHostname(invalid); err == nil {
			t.Errorf("accepted %q", invalid)
		}
	}
	hostname, err := NormalizeHostname("Silicon.Example.COM.")
	if err != nil || hostname != "silicon.example.com" {
		t.Fatalf("normalize: %q %v", hostname, err)
	}
	if !HostnameInZone(hostname, "example.com") || HostnameInZone("silicon.example.net", "example.com") {
		t.Fatal("zone matching failed")
	}
}

func TestConfigureCreatesLoopbackRouteAndPreservesUnrelatedRoutes(t *testing.T) {
	runner, repository, provider, host := testRunner(t)
	processed, err := runner.ProcessOne(context.Background())
	if err != nil || !processed {
		t.Fatalf("process: %v", err)
	}
	if !repository.activated {
		t.Fatal("configuration not activated")
	}
	if len(provider.routes) != 2 || provider.routes[0].Hostname != "grafana.example.com" {
		t.Fatalf("unrelated routes changed: %#v", provider.routes)
	}
	route := provider.routes[1]
	if route.Hostname != "silicon.example.com" || route.Service != "http://127.0.0.1:8080" {
		t.Fatalf("wrong route: %#v", route)
	}
	if len(host.applied) != 1 || host.applied[0].BindAddress != "127.0.0.1" || !host.applied[0].CookieSecure || !host.applied[0].TrustForwardedProto {
		t.Fatalf("unsafe host config: %#v", host.applied)
	}
	if len(provider.records) != 1 || provider.records[0].Content != "provider-tunnel.cfargotunnel.com" || !provider.records[0].Proxied {
		t.Fatalf("wrong DNS record: %#v", provider.records)
	}
}

func TestConfigureRejectsUnrelatedRouteConflict(t *testing.T) {
	runner, _, provider, host := testRunner(t)
	provider.routes = append(provider.routes, tunnelprovider.Route{Hostname: "silicon.example.com", Service: "http://unrelated:80"})
	_, err := runner.ProcessOne(context.Background())
	if err == nil {
		t.Fatal("expected conflict")
	}
	if len(host.applied) != 0 {
		t.Fatal("host configuration changed on conflict")
	}
}

func TestConfigureStopsBeforeHostMutationWhenCloudflareFails(t *testing.T) {
	runner, _, provider, host := testRunner(t)
	provider.failConfigure = true
	if _, err := runner.ProcessOne(context.Background()); err == nil {
		t.Fatal("expected Cloudflare failure")
	}
	if len(host.applied) != 0 {
		t.Fatal("host configuration changed before Cloudflare was prepared")
	}
}

func TestConfigureRollsBackCloudflareAfterHealthFailure(t *testing.T) {
	runner, repository, provider, host := testRunner(t)
	host.fail = true
	_, err := runner.ProcessOne(context.Background())
	if err == nil {
		t.Fatal("expected health failure")
	}
	if repository.activated {
		t.Fatal("failed configuration became active")
	}
	if len(provider.routes) != 1 || provider.routes[0].Hostname != "grafana.example.com" {
		t.Fatalf("routes not rolled back: %#v", provider.routes)
	}
	if len(provider.deleted) != 1 || provider.deleted[0] != "record-new" {
		t.Fatalf("owned DNS record not removed: %#v", provider.deleted)
	}
}

func TestResumedConfigurationUsesPersistedSnapshotAndRollsBackOwnedResources(t *testing.T) {
	runner, repository, provider, host := testRunner(t)
	secure, forwarded, bind := false, false, "0.0.0.0"
	repository.op.ProviderRecordID = "record-new"
	repository.op.PreviousPublicURL = "http://192.0.2.10"
	repository.op.PreviousCookieSecure = &secure
	repository.op.PreviousTrustForwardedProto = &forwarded
	repository.op.PreviousBindAddress = &bind
	provider.routes = append(provider.routes, tunnelprovider.Route{Hostname: "silicon.example.com", Service: "http://127.0.0.1:8080"})
	provider.records = append(provider.records, dnsprovider.Record{ID: "record-new", Name: "silicon.example.com", Type: "CNAME", Content: "provider-tunnel.cfargotunnel.com", Proxied: true})
	host.current.DesiredHostConfig = DesiredHostConfig{PublicURL: "https://silicon.example.com", CookieSecure: true, TrustForwardedProto: true, BindAddress: "127.0.0.1"}
	host.fail = true
	if _, err := runner.ProcessOne(context.Background()); err == nil {
		t.Fatal("expected health failure")
	}
	if len(provider.routes) != 1 || provider.routes[0].Hostname != "grafana.example.com" {
		t.Fatalf("resumed route not rolled back: %#v", provider.routes)
	}
	if len(provider.deleted) != 1 || provider.deleted[0] != "record-new" {
		t.Fatalf("resumed DNS not rolled back: %#v", provider.deleted)
	}
}

func TestDisableRestoresLocalConfigAndPreservesSharedTunnel(t *testing.T) {
	runner, repository, provider, host := testRunner(t)
	repository.op.Action = "disable"
	repository.hasActive = true
	repository.active = store.SystemPublicAccess{OrganizationID: *repository.op.OrganizationID, IntegrationID: *repository.op.IntegrationID, ZoneID: *repository.op.ZoneID, TunnelID: *repository.op.TunnelID, Hostname: "silicon.example.com", LocalOrigin: "http://127.0.0.1:8080", ProviderRecordID: "record-owned", PreviousPublicURL: "http://192.0.2.10", PreviousBindAddress: "0.0.0.0"}
	provider.routes = append(provider.routes, tunnelprovider.Route{Hostname: "silicon.example.com", Service: "http://127.0.0.1:8080"})
	_, err := runner.ProcessOne(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !repository.deleted {
		t.Fatal("active record not cleared")
	}
	if len(provider.routes) != 1 || provider.routes[0].Hostname != "grafana.example.com" {
		t.Fatalf("shared route changed: %#v", provider.routes)
	}
	if len(provider.deleted) != 1 || provider.deleted[0] != "record-owned" {
		t.Fatalf("owned record not deleted: %#v", provider.deleted)
	}
	if host.applied[0].PublicURL != "http://192.0.2.10" || host.applied[0].BindAddress != "0.0.0.0" {
		t.Fatalf("local config not restored: %#v", host.applied[0])
	}
}

func TestDomainChangePreparesNewRouteThenRemovesOnlyOldOwnedRoute(t *testing.T) {
	runner, repository, provider, _ := testRunner(t)
	repository.hasActive = true
	repository.active = store.SystemPublicAccess{OrganizationID: *repository.op.OrganizationID, IntegrationID: *repository.op.IntegrationID, ZoneID: *repository.op.ZoneID, TunnelID: *repository.op.TunnelID, Hostname: "old.example.com", ProviderRecordID: "record-old", PreviousPublicURL: "http://192.0.2.10", PreviousBindAddress: "0.0.0.0"}
	provider.routes = append(provider.routes, tunnelprovider.Route{Hostname: "old.example.com", Service: "http://127.0.0.1:8080"})
	provider.records = append(provider.records, dnsprovider.Record{ID: "record-old", Name: "old.example.com", Type: "CNAME", Content: "provider-tunnel.cfargotunnel.com", Proxied: true})
	if _, err := runner.ProcessOne(context.Background()); err != nil {
		t.Fatal(err)
	}
	seenNew, seenOld, seenShared := false, false, false
	for _, route := range provider.routes {
		switch route.Hostname {
		case "silicon.example.com":
			seenNew = true
		case "old.example.com":
			seenOld = true
		case "grafana.example.com":
			seenShared = true
		}
	}
	if !seenNew || seenOld || !seenShared {
		t.Fatalf("unsafe changed routes: %#v", provider.routes)
	}
	if len(provider.deleted) != 1 || provider.deleted[0] != "record-old" {
		t.Fatalf("old owned record cleanup=%#v", provider.deleted)
	}
}
