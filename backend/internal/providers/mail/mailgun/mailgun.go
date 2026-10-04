package mailgun

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	mailprovider "github.com/itsmangooo/Silicon/backend/internal/providers/mail"
)

type Provider struct {
	APIKey     []byte
	Domain     string
	BaseURL    string
	HTTPClient *http.Client
}

func (p Provider) Send(ctx context.Context, message mailprovider.Message) error {
	if err := mailprovider.ValidateMessage(message); err != nil {
		return err
	}
	values := url.Values{"from": {mailprovider.FromAddress(message.FromName, message.From)}, "to": {message.To}, "subject": {message.Subject}, "text": {message.Text}, "html": {message.HTML}}
	if message.ReplyTo != "" {
		values.Set("h:Reply-To", message.ReplyTo)
	}
	return p.request(ctx, http.MethodPost, "/v3/"+url.PathEscape(p.Domain)+"/messages", strings.NewReader(values.Encode()), "application/x-www-form-urlencoded")
}

func (p Provider) Test(ctx context.Context) error {
	return p.request(ctx, http.MethodGet, "/v3/domains/"+url.PathEscape(p.Domain), nil, "")
}

func (p Provider) Capabilities() mailprovider.Capabilities {
	return mailprovider.Capabilities{HTML: true, ReplyTo: true, SecureTLS: true, APIProvider: true}
}

func (p Provider) request(ctx context.Context, method, path string, body io.Reader, contentType string) error {
	if len(p.APIKey) == 0 || strings.TrimSpace(p.Domain) == "" {
		return errors.New("Mailgun credential and sending domain are required")
	}
	base := strings.TrimRight(p.BaseURL, "/")
	if base == "" {
		base = "https://api.mailgun.net"
	}
	request, err := http.NewRequestWithContext(ctx, method, base+path, body)
	if err != nil {
		return err
	}
	request.SetBasicAuth("api", string(p.APIKey))
	request.Header.Set("Content-Type", contentType)
	request.Header.Set("User-Agent", "Silicon")
	response, err := p.client().Do(request)
	if err != nil {
		return errors.New("Mailgun request failed")
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 64<<10))
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("%w: Mailgun returned status %d", mailprovider.ErrRejected, response.StatusCode)
	}
	return nil
}

func (p Provider) client() *http.Client {
	if p.HTTPClient != nil {
		return p.HTTPClient
	}
	return &http.Client{Timeout: 15 * time.Second}
}
