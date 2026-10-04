package mailservice

import (
	"html/template"
	"strings"

	mailprovider "github.com/itsmangooo/Silicon/backend/internal/providers/mail"
)

func PasswordResetMessage(fromName, fromAddress, replyTo, recipient, resetURL string) mailprovider.Message {
	text := "A password reset was requested for your Silicon account.\n\nOpen this link to choose a new password:\n" + resetURL + "\n\nThis link expires in 30 minutes and can be used once. If you did not request it, you can ignore this email."
	var html strings.Builder
	tmpl := template.Must(template.New("password-reset").Parse(`<!doctype html><html><body style="margin:0;padding:24px;background:#f4f6f5;color:#121816;font-family:Arial,sans-serif"><table role="presentation" width="100%"><tr><td align="center"><table role="presentation" width="560" style="max-width:100%;background:#fff;border:1px solid #d7dfdc"><tr><td style="padding:32px"><h1 style="font-size:22px;font-weight:600;margin:0 0 16px">Reset your Silicon password</h1><p style="line-height:1.6">A password reset was requested for your Silicon account.</p><p style="margin:24px 0"><a href="{{.}}" style="display:inline-block;background:#55e6b1;color:#082117;padding:12px 18px;text-decoration:none">Choose a new password</a></p><p style="line-height:1.6;color:#52605b">This link expires in 30 minutes and can be used once. If you did not request it, you can ignore this email.</p></td></tr></table></td></tr></table></body></html>`))
	_ = tmpl.Execute(&html, resetURL)
	return mailprovider.Message{FromName: fromName, From: fromAddress, To: recipient, ReplyTo: replyTo, Subject: "Reset your Silicon password", Text: text, HTML: html.String()}
}

func TestMessage(fromName, fromAddress, replyTo, recipient string) mailprovider.Message {
	return mailprovider.Message{FromName: fromName, From: fromAddress, To: recipient, ReplyTo: replyTo, Subject: "Silicon system email test", Text: "Silicon successfully sent this test message through the configured system email provider.", HTML: "<!doctype html><html><body><h1>Silicon system email test</h1><p>Silicon successfully sent this test message through the configured system email provider.</p></body></html>"}
}
