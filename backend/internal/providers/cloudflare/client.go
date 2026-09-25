package cloudflare

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	dnsprovider "github.com/itsmangooo/Silicon/backend/internal/providers/dns"
	tunnelprovider "github.com/itsmangooo/Silicon/backend/internal/providers/tunnel"
)

const maxResponseBytes = 4 << 20

type Client struct {
	Token      string
	BaseURL    string
	HTTPClient *http.Client
}
type envelope[T any] struct {
	Success bool `json:"success"`
	Result  T    `json:"result"`
	Errors  []struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"errors"`
	ResultInfo struct {
		Page       int `json:"page"`
		TotalPages int `json:"total_pages"`
	} `json:"result_info"`
}

func (c Client) TestConnection(ctx context.Context) error {
	var response envelope[struct {
		Status string `json:"status"`
	}]
	if err := c.request(ctx, http.MethodGet, "/user/tokens/verify", nil, &response); err != nil {
		return err
	}
	if response.Result.Status != "active" {
		return errors.New("cloudflare token is not active")
	}
	return nil
}

func (c Client) ListZones(ctx context.Context, accountID string) ([]dnsprovider.Zone, error) {
	result := []dnsprovider.Zone{}
	for page := 1; page <= 100; page++ {
		path := "/zones?per_page=50&page=" + strconv.Itoa(page)
		if accountID != "" {
			path += "&account.id=" + url.QueryEscape(accountID)
		}
		var response envelope[[]struct {
			ID     string `json:"id"`
			Name   string `json:"name"`
			Status string `json:"status"`
		}]
		if err := c.request(ctx, http.MethodGet, path, nil, &response); err != nil {
			return nil, err
		}
		for _, item := range response.Result {
			result = append(result, dnsprovider.Zone{ID: item.ID, Name: item.Name, Status: item.Status})
		}
		if len(response.Result) < 50 || (response.ResultInfo.TotalPages > 0 && page >= response.ResultInfo.TotalPages) {
			break
		}
	}
	return result, nil
}

func (c Client) FindRecords(ctx context.Context, zoneID, name string) ([]dnsprovider.Record, error) {
	var response envelope[[]recordResponse]
	if err := c.request(ctx, http.MethodGet, "/zones/"+url.PathEscape(zoneID)+"/dns_records?name="+url.QueryEscape(name), nil, &response); err != nil {
		return nil, err
	}
	items := make([]dnsprovider.Record, 0, len(response.Result))
	for _, item := range response.Result {
		items = append(items, item.record(zoneID))
	}
	return items, nil
}

func (c Client) CreateRecord(ctx context.Context, zoneID string, desired dnsprovider.DesiredRecord) (dnsprovider.Record, error) {
	var response envelope[recordResponse]
	if err := c.request(ctx, http.MethodPost, "/zones/"+url.PathEscape(zoneID)+"/dns_records", desired, &response); err != nil {
		return dnsprovider.Record{}, err
	}
	return response.Result.record(zoneID), nil
}

func (c Client) UpdateRecord(ctx context.Context, zoneID, recordID string, desired dnsprovider.DesiredRecord) (dnsprovider.Record, error) {
	var response envelope[recordResponse]
	if err := c.request(ctx, http.MethodPut, "/zones/"+url.PathEscape(zoneID)+"/dns_records/"+url.PathEscape(recordID), desired, &response); err != nil {
		return dnsprovider.Record{}, err
	}
	return response.Result.record(zoneID), nil
}

func (c Client) DeleteRecord(ctx context.Context, zoneID, recordID string) error {
	return c.request(ctx, http.MethodDelete, "/zones/"+url.PathEscape(zoneID)+"/dns_records/"+url.PathEscape(recordID), nil, nil)
}

type recordResponse struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Name    string `json:"name"`
	Content string `json:"content"`
	Proxied bool   `json:"proxied"`
}

func (r recordResponse) record(zoneID string) dnsprovider.Record {
	return dnsprovider.Record{ID: r.ID, ZoneID: zoneID, Type: r.Type, Name: r.Name, Content: r.Content, Proxied: r.Proxied}
}

func (c Client) List(ctx context.Context, accountID string) ([]tunnelprovider.Tunnel, error) {
	var response envelope[[]struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
		Status string `json:"status"`
	}]
	if err := c.request(ctx, http.MethodGet, "/accounts/"+url.PathEscape(accountID)+"/cfd_tunnel?is_deleted=false", nil, &response); err != nil {
		return nil, err
	}
	items := make([]tunnelprovider.Tunnel, 0, len(response.Result))
	for _, item := range response.Result {
		items = append(items, tunnelprovider.Tunnel{ID: item.ID, Name: item.Name, Status: item.Status})
	}
	return items, nil
}

func (c Client) Create(ctx context.Context, accountID, name string) (tunnelprovider.Tunnel, string, error) {
	var response envelope[struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
		Status string `json:"status"`
	}]
	if err := c.request(ctx, http.MethodPost, "/accounts/"+url.PathEscape(accountID)+"/cfd_tunnel", map[string]any{"name": name, "config_src": "cloudflare"}, &response); err != nil {
		return tunnelprovider.Tunnel{}, "", err
	}
	var token envelope[string]
	if err := c.request(ctx, http.MethodGet, "/accounts/"+url.PathEscape(accountID)+"/cfd_tunnel/"+url.PathEscape(response.Result.ID)+"/token", nil, &token); err != nil {
		return tunnelprovider.Tunnel{}, "", err
	}
	if token.Result == "" {
		return tunnelprovider.Tunnel{}, "", errors.New("cloudflare returned an empty tunnel token")
	}
	return tunnelprovider.Tunnel{ID: response.Result.ID, Name: response.Result.Name, Status: response.Result.Status}, token.Result, nil
}

func (c Client) ConfigureRoutes(ctx context.Context, accountID, tunnelID string, routes []tunnelprovider.Route) error {
	ingress := make([]map[string]string, 0, len(routes)+1)
	for _, route := range routes {
		ingress = append(ingress, map[string]string{"hostname": route.Hostname, "service": route.Service})
	}
	ingress = append(ingress, map[string]string{"service": "http_status:404"})
	body := map[string]any{"config": map[string]any{"ingress": ingress}}
	return c.request(ctx, http.MethodPut, "/accounts/"+url.PathEscape(accountID)+"/cfd_tunnel/"+url.PathEscape(tunnelID)+"/configurations", body, nil)
}

func (c Client) Routes(ctx context.Context, accountID, tunnelID string) ([]tunnelprovider.Route, error) {
	var response envelope[struct {
		Config struct {
			Ingress []struct {
				Hostname string `json:"hostname"`
				Service  string `json:"service"`
			} `json:"ingress"`
		} `json:"config"`
	}]
	if err := c.request(ctx, http.MethodGet, "/accounts/"+url.PathEscape(accountID)+"/cfd_tunnel/"+url.PathEscape(tunnelID)+"/configurations", nil, &response); err != nil {
		return nil, err
	}
	routes := []tunnelprovider.Route{}
	for _, item := range response.Result.Config.Ingress {
		if item.Hostname != "" {
			routes = append(routes, tunnelprovider.Route{Hostname: item.Hostname, Service: item.Service})
		}
	}
	return routes, nil
}

func (c Client) request(ctx context.Context, method, path string, body any, result any) error {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(data)
	}
	base := strings.TrimRight(c.BaseURL, "/")
	if base == "" {
		base = "https://api.cloudflare.com/client/v4"
	}
	req, err := http.NewRequestWithContext(ctx, method, base+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Content-Type", "application/json")
	client := c.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	response, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("cloudflare request failed: %w", err)
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes))
	if err != nil {
		return err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("cloudflare request failed with status %d", response.StatusCode)
	}
	if len(data) > 0 {
		var header struct {
			Success bool `json:"success"`
		}
		if err := json.Unmarshal(data, &header); err != nil {
			return err
		}
		if !header.Success {
			return errors.New("cloudflare rejected the request")
		}
		if result != nil {
			if err := json.Unmarshal(data, result); err != nil {
				return err
			}
		}
	}
	return nil
}
