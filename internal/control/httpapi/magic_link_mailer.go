package httpapi

import (
	"context"
	"errors"
	"fmt"
	"net/smtp"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// MagicLinkMessage is the payload delivered to users for login.
type MagicLinkMessage struct {
	ToEmail     string
	MagicLinkID string
	Code        string
	OrgID       string
	UserID      string
	Role        string
	ExpiresAt   time.Time
}

// MagicLinkSender sends magic-link login messages.
type MagicLinkSender interface {
	SendMagicLink(ctx context.Context, msg MagicLinkMessage) error
}

type noopMagicLinkSender struct{}

func (noopMagicLinkSender) SendMagicLink(_ context.Context, _ MagicLinkMessage) error { return nil }

type smtpMagicLinkSender struct {
	host    string
	port    int
	from    string
	baseURL string
	auth    smtp.Auth
}

func newSMTPMagicLinkSenderFromEnv() (MagicLinkSender, error) {
	host := strings.TrimSpace(os.Getenv("CONTROL_SMTP_HOST"))
	if host == "" {
		return nil, errors.New("CONTROL_SMTP_HOST is required")
	}

	portRaw := strings.TrimSpace(os.Getenv("CONTROL_SMTP_PORT"))
	if portRaw == "" {
		return nil, errors.New("CONTROL_SMTP_PORT is required")
	}
	port, err := strconv.Atoi(portRaw)
	if err != nil || port <= 0 {
		return nil, errors.New("CONTROL_SMTP_PORT must be a positive integer")
	}

	from := strings.TrimSpace(os.Getenv("CONTROL_SMTP_FROM"))
	if from == "" {
		return nil, errors.New("CONTROL_SMTP_FROM is required")
	}

	username := strings.TrimSpace(os.Getenv("CONTROL_SMTP_USERNAME"))
	password := strings.TrimSpace(os.Getenv("CONTROL_SMTP_PASSWORD"))

	var auth smtp.Auth
	if username != "" || password != "" {
		if username == "" || password == "" {
			return nil, errors.New("CONTROL_SMTP_USERNAME and CONTROL_SMTP_PASSWORD must both be set when SMTP auth is enabled")
		}
		auth = smtp.PlainAuth("", username, password, host)
	}

	return &smtpMagicLinkSender{
		host:    host,
		port:    port,
		from:    from,
		baseURL: strings.TrimSpace(os.Getenv("CONTROL_MAGIC_LINK_BASE_URL")),
		auth:    auth,
	}, nil
}

func (s *smtpMagicLinkSender) SendMagicLink(ctx context.Context, msg MagicLinkMessage) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	subject := "Your Go Router sign-in code"
	link := ""
	if s.baseURL != "" {
		u, err := url.Parse(s.baseURL)
		if err != nil {
			return fmt.Errorf("parse CONTROL_MAGIC_LINK_BASE_URL: %w", err)
		}
		q := u.Query()
		q.Set("magic_link_id", msg.MagicLinkID)
		q.Set("code", msg.Code)
		u.RawQuery = q.Encode()
		link = u.String()
	}

	bodyLines := []string{
		"Use this one-time sign-in code:",
		msg.Code,
		"",
		"Code expires at: " + msg.ExpiresAt.UTC().Format(time.RFC3339),
	}
	if link != "" {
		bodyLines = append(bodyLines, "", "Or open this magic link:", link)
	}

	message := strings.Join([]string{
		"From: " + s.from,
		"To: " + msg.ToEmail,
		"Subject: " + subject,
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
		"",
		strings.Join(bodyLines, "\r\n"),
	}, "\r\n")

	addr := fmt.Sprintf("%s:%d", s.host, s.port)
	if err := smtp.SendMail(addr, s.auth, s.from, []string{msg.ToEmail}, []byte(message)); err != nil {
		return fmt.Errorf("send smtp mail: %w", err)
	}
	return nil
}
