package mail_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	mailprovider "github.com/itsmangooo/Silicon/backend/internal/providers/mail"
	"github.com/itsmangooo/Silicon/backend/internal/providers/mail/mailgun"
	"github.com/itsmangooo/Silicon/backend/internal/providers/mail/postmark"
	"github.com/itsmangooo/Silicon/backend/internal/providers/mail/resend"
)

func TestHTTPMailProviderRequests(t *testing.T) {
	message := mailprovider.Message{FromName: "Silicon", From: "silicon@example.com", To: "user@example.com", ReplyTo: "support@example.com", Subject: "Test message", Text: "plain", HTML: "<p>html</p>"}
	for _, test := range []struct {
		name  string
		make  func(string, *http.Client) mailprovider.Provider
		check func(*testing.T, *http.Request, string)
	}{
		{"resend", func(base string, client *http.Client) mailprovider.Provider {
			return resend.Provider{APIKey: []byte("secret"), BaseURL: base, HTTPClient: client}
		}, func(t *testing.T, r *http.Request, body string) {
			if r.URL.Path != "/emails" || r.Header.Get("Authorization") != "Bearer secret" || !strings.Contains(body, "support@example.com") {
				t.Fatalf("unexpected request %s %v %s", r.URL.Path, r.Header, body)
			}
		}},
		{"postmark", func(base string, client *http.Client) mailprovider.Provider {
			return postmark.Provider{ServerToken: []byte("secret"), BaseURL: base, HTTPClient: client}
		}, func(t *testing.T, r *http.Request, body string) {
			if r.URL.Path != "/email" || r.Header.Get("X-Postmark-Server-Token") != "secret" || !strings.Contains(body, "outbound") {
				t.Fatalf("unexpected request %s %v %s", r.URL.Path, r.Header, body)
			}
		}},
		{"mailgun", func(base string, client *http.Client) mailprovider.Provider {
			return mailgun.Provider{APIKey: []byte("secret"), Domain: "mg.example.com", BaseURL: base, HTTPClient: client}
		}, func(t *testing.T, r *http.Request, body string) {
			username, password, ok := r.BasicAuth()
			if r.URL.Path != "/v3/mg.example.com/messages" || !ok || username != "api" || password != "secret" || !strings.Contains(body, "h%3AReply-To") {
				t.Fatalf("unexpected request %s %v %s", r.URL.Path, r.Header, body)
			}
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				test.check(t, r, string(body))
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()
			if err := test.make(server.URL, server.Client()).Send(context.Background(), message); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestHTTPMailProviderErrorsAreBoundedAndSafe(t *testing.T) {
	message := mailprovider.Message{From: "silicon@example.com", To: "user@example.com", Subject: "Test", Text: "plain"}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `provider diagnostic includes secret-value`)
	}))
	defer server.Close()
	providers := []mailprovider.Provider{
		resend.Provider{APIKey: []byte("secret-value"), BaseURL: server.URL, HTTPClient: server.Client()},
		postmark.Provider{ServerToken: []byte("secret-value"), BaseURL: server.URL, HTTPClient: server.Client()},
		mailgun.Provider{APIKey: []byte("secret-value"), Domain: "mg.example.com", BaseURL: server.URL, HTTPClient: server.Client()},
	}
	for _, provider := range providers {
		err := provider.Send(context.Background(), message)
		if !errors.Is(err, mailprovider.ErrRejected) || strings.Contains(err.Error(), "secret-value") || strings.Contains(err.Error(), "diagnostic") {
			t.Fatalf("provider error was not classified and sanitized: %v", err)
		}
	}

	slow := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { time.Sleep(100 * time.Millisecond) }))
	defer slow.Close()
	client := slow.Client()
	client.Timeout = 20 * time.Millisecond
	started := time.Now()
	err := (resend.Provider{APIKey: []byte("secret-value"), BaseURL: slow.URL, HTTPClient: client}).Send(context.Background(), message)
	if err == nil || err.Error() != "Resend request failed" || time.Since(started) > 500*time.Millisecond {
		t.Fatalf("provider timeout was not bounded and safe: %v", err)
	}
}
