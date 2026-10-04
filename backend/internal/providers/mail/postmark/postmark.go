package postmark

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	mailprovider "github.com/itsmangooo/Silicon/backend/internal/providers/mail"
)

type Provider struct {
	ServerToken []byte
	BaseURL     string
	HTTPClient  *http.Client
}

func (p Provider) Send(ctx context.Context, message mailprovider.Message) error {
	if err := mailprovider.ValidateMessage(message); err != nil {
		return err
	}
	payload := map[string]any{"From": mailprovider.FromAddress(message.FromName, message.From), "To": message.To, "Subject": message.Subject, "TextBody": message.Text, "HtmlBody": message.HTML, "MessageStream": "outbound"}
	if message.ReplyTo != "" {
		payload["ReplyTo"] = message.ReplyTo
	}
	return p.request(ctx, http.MethodPost, "/email", payload)
}

func (p Provider) Test(ctx context.Context) error {
	return p.request(ctx, http.MethodGet, "/server", nil)
}

func (p Provider) Capabilities() mailprovider.Capabilities {
	return mailprovider.Capabilities{HTML: true, ReplyTo: true, SecureTLS: true, APIProvider: true}
}

func (p Provider) request(ctx context.Context, method, path string, payload any) error {
	if len(p.ServerToken) == 0 {
		return errors.New("Postmark credential is not configured")
	}
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(encoded)
	}
	base := strings.TrimRight(p.BaseURL, "/")
	if base == "" {
		base = "https://api.postmarkapp.com"
	}
	request, err := http.NewRequestWithContext(ctx, method, base+path, body)
	if err != nil {
		return err
	}
	request.Header.Set("X-Postmark-Server-Token", string(p.ServerToken))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("User-Agent", "Silicon")
	response, err := p.client().Do(request)
	if err != nil {
		return errors.New("Postmark request failed")
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 64<<10))
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("%w: Postmark returned status %d", mailprovider.ErrRejected, response.StatusCode)
	}
	return nil
}

func (p Provider) client() *http.Client {
	if p.HTTPClient != nil {
		return p.HTTPClient
	}
	return &http.Client{Timeout: 15 * time.Second}
}
