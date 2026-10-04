package mailservice

import (
	"encoding/json"
	"errors"
	"net/mail"
	"strings"

	mailprovider "github.com/itsmangooo/Silicon/backend/internal/providers/mail"
	"github.com/itsmangooo/Silicon/backend/internal/providers/mail/mailgun"
	"github.com/itsmangooo/Silicon/backend/internal/providers/mail/postmark"
	"github.com/itsmangooo/Silicon/backend/internal/providers/mail/resend"
	"github.com/itsmangooo/Silicon/backend/internal/providers/mail/ses"
	smtpprovider "github.com/itsmangooo/Silicon/backend/internal/providers/mail/smtp"
)

type Settings struct {
	MailgunDomain  string `json:"mailgunDomain,omitempty"`
	MailgunRegion  string `json:"mailgunRegion,omitempty"`
	SESRegion      string `json:"sesRegion,omitempty"`
	SMTPPreset     string `json:"smtpPreset,omitempty"`
	SMTPHost       string `json:"smtpHost,omitempty"`
	SMTPPort       int    `json:"smtpPort,omitempty"`
	SMTPUsername   string `json:"smtpUsername,omitempty"`
	SMTPEncryption string `json:"smtpEncryption,omitempty"`
}

type Credentials struct {
	AccessKeyID  string `json:"accessKeyId,omitempty"`
	Secret       string `json:"secret"`
	SessionToken string `json:"sessionToken,omitempty"`
}

type Configuration struct {
	Provider    string
	FromName    string
	FromAddress string
	ReplyTo     string
	Settings    Settings
	Credentials Credentials
}

func ParseSettings(value json.RawMessage) (Settings, error) {
	var settings Settings
	if len(value) == 0 {
		return settings, nil
	}
	return settings, json.Unmarshal(value, &settings)
}

func ParseCredentials(value []byte) (Credentials, error) {
	var credentials Credentials
	return credentials, json.Unmarshal(value, &credentials)
}

func Validate(configuration Configuration) error {
	configuration.Provider = strings.ToLower(strings.TrimSpace(configuration.Provider))
	if !oneOf(configuration.Provider, "resend", "postmark", "mailgun", "ses", "smtp") {
		return errors.New("mail provider is invalid")
	}
	if len(strings.TrimSpace(configuration.FromName)) < 1 || len(configuration.FromName) > 120 || strings.ContainsAny(configuration.FromName, "\r\n") {
		return errors.New("from name is required")
	}
	if !plainAddress(configuration.FromAddress) {
		return errors.New("from address is invalid")
	}
	if configuration.ReplyTo != "" {
		if !plainAddress(configuration.ReplyTo) {
			return errors.New("reply-to address is invalid")
		}
	}
	if len(configuration.Credentials.Secret) > 65536 {
		return errors.New("provider credential is too large")
	}
	switch configuration.Provider {
	case "resend", "postmark":
		if configuration.Credentials.Secret == "" {
			return errors.New("provider credential is required")
		}
	case "mailgun":
		if configuration.Credentials.Secret == "" {
			return errors.New("Mailgun API key is required")
		}
		if configuration.Settings.MailgunDomain == "" || strings.ContainsAny(configuration.Settings.MailgunDomain, " /\\\t\r\n\x00") {
			return errors.New("Mailgun sending domain is required")
		}
		if configuration.Settings.MailgunRegion != "" && !oneOf(configuration.Settings.MailgunRegion, "us", "eu") {
			return errors.New("Mailgun region must be US or EU")
		}
	case "ses":
		if configuration.Settings.SESRegion == "" || configuration.Credentials.AccessKeyID == "" || configuration.Credentials.Secret == "" {
			return errors.New("Amazon SES region, access key ID, and secret access key are required")
		}
	case "smtp":
		if configuration.Settings.SMTPHost == "" || strings.ContainsAny(configuration.Settings.SMTPHost, " /\\\t\r\n\x00") || configuration.Settings.SMTPPort < 1 || configuration.Settings.SMTPPort > 65535 {
			return errors.New("SMTP host and port are invalid")
		}
		if !oneOf(configuration.Settings.SMTPEncryption, "starttls", "tls", "none") {
			return errors.New("SMTP encryption must be STARTTLS, implicit TLS, or none")
		}
		if configuration.Settings.SMTPUsername != "" && configuration.Credentials.Secret == "" {
			return errors.New("SMTP password is required when a username is configured")
		}
	}
	return nil
}

func plainAddress(value string) bool {
	value = strings.TrimSpace(value)
	parsed, err := mail.ParseAddress(value)
	return err == nil && strings.EqualFold(parsed.Address, value)
}

func Provider(configuration Configuration) (mailprovider.Provider, error) {
	if err := Validate(configuration); err != nil {
		return nil, err
	}
	secret := []byte(configuration.Credentials.Secret)
	switch configuration.Provider {
	case "resend":
		return resend.Provider{APIKey: secret}, nil
	case "postmark":
		return postmark.Provider{ServerToken: secret}, nil
	case "mailgun":
		baseURL := ""
		if configuration.Settings.MailgunRegion == "eu" {
			baseURL = "https://api.eu.mailgun.net"
		}
		return mailgun.Provider{APIKey: secret, Domain: configuration.Settings.MailgunDomain, BaseURL: baseURL}, nil
	case "ses":
		return ses.Provider{Region: configuration.Settings.SESRegion, AccessKeyID: configuration.Credentials.AccessKeyID, SecretAccessKey: secret, SessionToken: []byte(configuration.Credentials.SessionToken)}, nil
	case "smtp":
		return smtpprovider.Provider{Host: configuration.Settings.SMTPHost, Port: configuration.Settings.SMTPPort, Username: configuration.Settings.SMTPUsername, Password: secret, Encryption: configuration.Settings.SMTPEncryption}, nil
	default:
		return nil, errors.New("mail provider is unsupported")
	}
}

func ApplySMTPPreset(settings Settings) Settings {
	settings.SMTPPreset = strings.ToLower(strings.TrimSpace(settings.SMTPPreset))
	if settings.SMTPPort == 0 {
		switch settings.SMTPEncryption {
		case "tls":
			settings.SMTPPort = 465
		default:
			settings.SMTPPort = 587
		}
	}
	if settings.SMTPEncryption == "" {
		settings.SMTPEncryption = "starttls"
	}
	if !oneOf(settings.SMTPPreset, "billionmail", "stalwart", "mailcow", "postal", "generic", "") {
		settings.SMTPPreset = "generic"
	}
	return settings
}

func oneOf(value string, values ...string) bool {
	for _, item := range values {
		if value == item {
			return true
		}
	}
	return false
}
