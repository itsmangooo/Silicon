package mail

import (
	"context"
	"errors"
	"net/mail"
	"strings"
)

type Message struct {
	FromName string `json:"fromName"`
	From     string `json:"from"`
	To       string `json:"to"`
	ReplyTo  string `json:"replyTo,omitempty"`
	Subject  string `json:"subject"`
	Text     string `json:"text"`
	HTML     string `json:"html"`
}

type Capabilities struct {
	HTML        bool `json:"html"`
	ReplyTo     bool `json:"replyTo"`
	SecureTLS   bool `json:"secureTls"`
	APIProvider bool `json:"apiProvider"`
}

type Provider interface {
	Send(context.Context, Message) error
	Test(context.Context) error
	Capabilities() Capabilities
}

var ErrRejected = errors.New("mail provider rejected the message")

func ValidateMessage(message Message) error {
	if !plainAddress(message.From) {
		return errors.New("mail sender address is invalid")
	}
	if !plainAddress(message.To) {
		return errors.New("mail recipient address is invalid")
	}
	if message.ReplyTo != "" {
		if !plainAddress(message.ReplyTo) {
			return errors.New("mail reply-to address is invalid")
		}
	}
	if strings.ContainsAny(message.FromName, "\r\n") {
		return errors.New("mail sender name is invalid")
	}
	if strings.TrimSpace(message.Subject) == "" || len(message.Subject) > 998 || strings.ContainsAny(message.Subject, "\r\n") {
		return errors.New("mail subject is invalid")
	}
	if message.Text == "" && message.HTML == "" {
		return errors.New("mail body is required")
	}
	return nil
}

func plainAddress(value string) bool {
	value = strings.TrimSpace(value)
	parsed, err := mail.ParseAddress(value)
	return err == nil && strings.EqualFold(parsed.Address, value)
}

func FromAddress(name, address string) string {
	if strings.TrimSpace(name) == "" {
		return address
	}
	return (&mail.Address{Name: name, Address: address}).String()
}
