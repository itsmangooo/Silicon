package resend

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
	APIKey     []byte
	BaseURL    string
	HTTPClient *http.Client
}

func (p Provider) Send(ctx context.Context, message mailprovider.Message) error {
	if err := mailprovider.ValidateMessage(message); err != nil {
		return err
	}
	payload := map[string]any{"from": mailprovider.FromAddress(message.FromName, message.From), "to": []string{message.To}, "subject": message.Subject, "text": message.Text, "html": message.HTML}
	if message.ReplyTo != "" {
		payload["reply_to"] = message.ReplyTo
	}
	return p.request(ctx, http.MethodPost, "/emails", payload)
}

func (p Provider) Test(ctx context.Context) error {
	return p.request(ctx, http.MethodGet, "/api-keys", nil)
}

func (p Provider) Capabilities() mailprovider.Capabilities {
	return mailprovider.Capabilities{HTML: true, ReplyTo: true, SecureTLS: true, APIProvider: true}
}

func (p Provider) request(ctx context.Context, method, path string, payload any) error {
	if len(p.APIKey) == 0 {
		return errors.New("Resend credential is not configured")
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
		base = "https://api.resend.com"
	}
	request, err := http.NewRequestWithContext(ctx, method, base+path, body)
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+string(p.APIKey))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("User-Agent", "Silicon")
	response, err := p.client().Do(request)
	if err != nil {
		return errors.New("Resend request failed")
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 64<<10))
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("%w: Resend returned status %d", mailprovider.ErrRejected, response.StatusCode)
	}
	return nil
}

func (p Provider) client() *http.Client {
	if p.HTTPClient != nil {
		return p.HTTPClient
	}
	return &http.Client{Timeout: 15 * time.Second}
}
