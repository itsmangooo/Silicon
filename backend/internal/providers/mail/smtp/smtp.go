package smtp

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"net"
	"net/mail"
	netsmtp "net/smtp"
	"strconv"
	"strings"
	"time"

	mailprovider "github.com/itsmangooo/Silicon/backend/internal/providers/mail"
)

type Provider struct {
	Host       string
	Port       int
	Username   string
	Password   []byte
	Encryption string
	Timeout    time.Duration
	Dialer     *net.Dialer
	TLSConfig  *tls.Config
}

func (p Provider) Send(ctx context.Context, message mailprovider.Message) error {
	if err := mailprovider.ValidateMessage(message); err != nil {
		return err
	}
	client, connection, err := p.connect(ctx)
	if err != nil {
		return err
	}
	defer connection.Close()
	defer client.Close()
	if err = p.authenticate(client); err != nil {
		return err
	}
	if err = client.Mail(message.From); err != nil {
		return errors.New("SMTP sender was rejected")
	}
	if err = client.Rcpt(message.To); err != nil {
		return errors.New("SMTP recipient was rejected")
	}
	writer, err := client.Data()
	if err != nil {
		return errors.New("SMTP message body was rejected")
	}
	content, err := encodeMessage(message)
	if err == nil {
		_, err = writer.Write(content)
	}
	if closeErr := writer.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return errors.New("SMTP message delivery failed")
	}
	if err = client.Quit(); err != nil {
		return errors.New("SMTP delivery could not be confirmed")
	}
	return nil
}

func (p Provider) Test(ctx context.Context) error {
	client, connection, err := p.connect(ctx)
	if err != nil {
		return err
	}
	defer connection.Close()
	defer client.Close()
	if err = p.authenticate(client); err != nil {
		return err
	}
	if err = client.Quit(); err != nil {
		return errors.New("SMTP connection test could not be confirmed")
	}
	return nil
}

func (p Provider) Capabilities() mailprovider.Capabilities {
	return mailprovider.Capabilities{HTML: true, ReplyTo: true, SecureTLS: p.Encryption != "none", APIProvider: false}
}

func (p Provider) connect(ctx context.Context) (*netsmtp.Client, net.Conn, error) {
	host := strings.TrimSpace(p.Host)
	if host == "" || p.Port < 1 || p.Port > 65535 || !oneOf(p.Encryption, "starttls", "tls", "none") {
		return nil, nil, errors.New("SMTP connection configuration is invalid")
	}
	address := net.JoinHostPort(host, strconv.Itoa(p.Port))
	timeout := p.Timeout
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	dialer := p.Dialer
	if dialer == nil {
		dialer = &net.Dialer{Timeout: timeout}
	}
	var connection net.Conn
	var err error
	if p.Encryption == "tls" {
		connection, err = (&tls.Dialer{NetDialer: dialer, Config: p.tlsConfig(host)}).DialContext(ctx, "tcp", address)
	} else {
		connection, err = dialer.DialContext(ctx, "tcp", address)
	}
	if err != nil {
		return nil, nil, errors.New("SMTP server is unreachable")
	}
	_ = connection.SetDeadline(time.Now().Add(timeout))
	client, err := netsmtp.NewClient(connection, host)
	if err != nil {
		connection.Close()
		return nil, nil, errors.New("SMTP server greeting is invalid")
	}
	if p.Encryption == "starttls" {
		if ok, _ := client.Extension("STARTTLS"); !ok {
			client.Close()
			connection.Close()
			return nil, nil, errors.New("SMTP server does not support STARTTLS")
		}
		if err = client.StartTLS(p.tlsConfig(host)); err != nil {
			client.Close()
			connection.Close()
			return nil, nil, errors.New("SMTP TLS negotiation failed")
		}
	}
	return client, connection, nil
}

func (p Provider) tlsConfig(host string) *tls.Config {
	if p.TLSConfig == nil {
		return &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}
	}
	configuration := p.TLSConfig.Clone()
	if configuration.ServerName == "" {
		configuration.ServerName = host
	}
	if configuration.MinVersion < tls.VersionTLS12 {
		configuration.MinVersion = tls.VersionTLS12
	}
	return configuration
}

func (p Provider) authenticate(client *netsmtp.Client) error {
	if p.Username == "" {
		return nil
	}
	if len(p.Password) == 0 {
		return errors.New("SMTP password is not configured")
	}
	if p.Encryption == "none" {
		return errors.New("SMTP authentication requires STARTTLS or implicit TLS")
	}
	if ok, _ := client.Extension("AUTH"); !ok {
		return errors.New("SMTP server does not advertise authentication")
	}
	if err := client.Auth(netsmtp.PlainAuth("", p.Username, string(p.Password), p.Host)); err != nil {
		return errors.New("SMTP authentication failed")
	}
	return nil
}

func encodeMessage(message mailprovider.Message) ([]byte, error) {
	boundaryBytes := make([]byte, 18)
	if _, err := rand.Read(boundaryBytes); err != nil {
		return nil, err
	}
	boundary := base64.RawURLEncoding.EncodeToString(boundaryBytes)
	headers := []string{
		"From: " + mailprovider.FromAddress(message.FromName, message.From),
		"To: " + (&mail.Address{Address: message.To}).String(),
		"Subject: " + message.Subject,
		"MIME-Version: 1.0",
		`Content-Type: multipart/alternative; boundary="` + boundary + `"`,
	}
	if message.ReplyTo != "" {
		headers = append(headers, "Reply-To: "+(&mail.Address{Address: message.ReplyTo}).String())
	}
	var body bytes.Buffer
	body.WriteString(strings.Join(headers, "\r\n") + "\r\n\r\n")
	body.WriteString("--" + boundary + "\r\nContent-Type: text/plain; charset=utf-8\r\nContent-Transfer-Encoding: 8bit\r\n\r\n" + message.Text + "\r\n")
	body.WriteString("--" + boundary + "\r\nContent-Type: text/html; charset=utf-8\r\nContent-Transfer-Encoding: 8bit\r\n\r\n" + message.HTML + "\r\n")
	body.WriteString("--" + boundary + "--\r\n")
	return body.Bytes(), nil
}

func oneOf(value string, allowed ...string) bool {
	for _, item := range allowed {
		if value == item {
			return true
		}
	}
	return false
}

var _ mailprovider.Provider = Provider{}
