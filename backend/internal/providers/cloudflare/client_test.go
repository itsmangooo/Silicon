package cloudflare

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	dnsprovider "github.com/itsmangooo/Silicon/backend/internal/providers/dns"
	tunnelprovider "github.com/itsmangooo/Silicon/backend/internal/providers/tunnel"
)

func TestConnectionZonesAndDNSLifecycle(t *testing.T) {
	var created, updated dnsprovider.DesiredRecord
	deleted := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer scoped-token" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/user/tokens/verify":
			io.WriteString(w, `{"success":true,"result":{"status":"active"}}`)
		case r.URL.Path == "/zones":
			io.WriteString(w, `{"success":true,"result":[{"id":"zone-1","name":"example.com","status":"active"}]}`)
		case r.URL.Path == "/zones/zone-1/dns_records" && r.Method == http.MethodGet:
			io.WriteString(w, `{"success":true,"result":[]}`)
		case r.URL.Path == "/zones/zone-1/dns_records" && r.Method == http.MethodPost:
			_ = json.NewDecoder(r.Body).Decode(&created)
			io.WriteString(w, `{"success":true,"result":{"id":"record-1","type":"A","name":"api.example.com","content":"203.0.113.10","proxied":true}}`)
		case r.URL.Path == "/zones/zone-1/dns_records/record-1" && r.Method == http.MethodPut:
			_ = json.NewDecoder(r.Body).Decode(&updated)
			io.WriteString(w, `{"success":true,"result":{"id":"record-1","type":"A","name":"api.example.com","content":"203.0.113.11","proxied":false}}`)
		case r.URL.Path == "/zones/zone-1/dns_records/record-1" && r.Method == http.MethodDelete:
			deleted = true
			io.WriteString(w, `{"success":true,"result":{"id":"record-1"}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	client := Client{Token: "scoped-token", BaseURL: server.URL}
	if err := client.TestConnection(context.Background()); err != nil {
		t.Fatal(err)
	}
	zones, err := client.ListZones(context.Background(), "account-1")
	if err != nil || len(zones) != 1 || zones[0].Name != "example.com" {
		t.Fatalf("zones=%v err=%v", zones, err)
	}
	records, err := client.FindRecords(context.Background(), "zone-1", "api.example.com")
	if err != nil || len(records) != 0 {
		t.Fatalf("records=%v err=%v", records, err)
	}
	createdRecord, err := client.CreateRecord(context.Background(), "zone-1", dnsprovider.DesiredRecord{Type: "A", Name: "api.example.com", Content: "203.0.113.10", Proxied: true})
	if err != nil || !createdRecord.Proxied || !created.Proxied {
		t.Fatalf("created=%v body=%v err=%v", createdRecord, created, err)
	}
	updatedRecord, err := client.UpdateRecord(context.Background(), "zone-1", "record-1", dnsprovider.DesiredRecord{Type: "A", Name: "api.example.com", Content: "203.0.113.11", Proxied: false})
	if err != nil || updatedRecord.Proxied || updated.Content != "203.0.113.11" {
		t.Fatalf("updated=%v body=%v err=%v", updatedRecord, updated, err)
	}
	if err := client.DeleteRecord(context.Background(), "zone-1", "record-1"); err != nil || !deleted {
		t.Fatalf("delete=%v deleted=%v", err, deleted)
	}
}

func TestInvalidCredentialAndRedaction(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "token scoped-token rejected", http.StatusForbidden)
	}))
	defer server.Close()
	err := (Client{Token: "scoped-token", BaseURL: server.URL}).TestConnection(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(err.Error(), "scoped-token") {
		t.Fatalf("credential leaked in error: %v", err)
	}
}

func TestProviderSuccessFalseIsAnError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"success":false,"errors":[{"code":1000,"message":"rejected"}]}`)
	}))
	defer server.Close()
	if err := (Client{Token: "token", BaseURL: server.URL}).DeleteRecord(context.Background(), "zone", "record"); err == nil {
		t.Fatal("Cloudflare success=false was accepted")
	}
}

func TestTunnelCreateListAndSharedRoutes(t *testing.T) {
	var ingress []map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/accounts/account/cfd_tunnel" && r.Method == http.MethodPost:
			io.WriteString(w, `{"success":true,"result":{"id":"tunnel-1","name":"silicon","status":"inactive"}}`)
		case r.URL.Path == "/accounts/account/cfd_tunnel/tunnel-1/token" && r.Method == http.MethodGet:
			io.WriteString(w, `{"success":true,"result":"runner-token"}`)
		case r.URL.Path == "/accounts/account/cfd_tunnel" && r.Method == http.MethodGet:
			io.WriteString(w, `{"success":true,"result":[{"id":"shared-1","name":"shared","status":"healthy"}]}`)
		case r.URL.Path == "/accounts/account/cfd_tunnel/tunnel-1/configurations":
			if r.Method == http.MethodGet {
				io.WriteString(w, `{"success":true,"result":{"config":{"ingress":[{"hostname":"existing.example.com","service":"http://existing:8080"},{"service":"http_status:404"}]}}}`)
				return
			}
			var body struct {
				Config struct {
					Ingress []map[string]string `json:"ingress"`
				} `json:"config"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			ingress = body.Config.Ingress
			io.WriteString(w, `{"success":true,"result":{}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	client := Client{Token: "token", BaseURL: server.URL}
	tunnel, token, err := client.Create(context.Background(), "account", "silicon")
	if err != nil || tunnel.ID != "tunnel-1" || token != "runner-token" {
		t.Fatalf("tunnel=%v token=%q err=%v", tunnel, token, err)
	}
	tunnels, err := client.List(context.Background(), "account")
	if err != nil || len(tunnels) != 1 || tunnels[0].ID != "shared-1" {
		t.Fatalf("tunnels=%v err=%v", tunnels, err)
	}
	existingRoutes, err := client.Routes(context.Background(), "account", "tunnel-1")
	if err != nil || len(existingRoutes) != 1 || existingRoutes[0].Hostname != "existing.example.com" {
		t.Fatalf("existing routes=%v err=%v", existingRoutes, err)
	}
	err = client.ConfigureRoutes(context.Background(), "account", "tunnel-1", []tunnelprovider.Route{{Hostname: "api.example.com", Service: "http://api:3000"}, {Hostname: "web.example.com", Service: "http://web:8080"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(ingress) != 3 || ingress[2]["service"] != "http_status:404" {
		t.Fatalf("ingress=%v", ingress)
	}
}
