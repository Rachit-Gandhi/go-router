package httpapi

import (
	"context"
	"errors"
	"fmt"
	"net/smtp"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
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
func (noopMagicLinkSender) DeliveryMode() string                                      { return "noop" }
func (noopMagicLinkSender) ExposeMagicLinkCode() bool                                 { return true }

type smtpMagicLinkSender struct {
	host    string
	port    int
	from    string
	baseURL string
	auth    smtp.Auth
}

type fileMagicLinkSender struct {
	path    string
	baseURL string
	mu      sync.Mutex
}

type magicLinkDeliveryDescriber interface {
	DeliveryMode() string
}

type magicLinkCodeExposer interface {
	ExposeMagicLinkCode() bool
}

func magicLinkDelivery(sender MagicLinkSender) string {
	if describer, ok := sender.(magicLinkDeliveryDescriber); ok {
		return describer.DeliveryMode()
	}
	return "custom"
}

func shouldExposeMagicLinkCode(sender MagicLinkSender) bool {
	exposer, ok := sender.(magicLinkCodeExposer)
	return ok && exposer.ExposeMagicLinkCode()
}

func newMagicLinkSenderFromEnv() (MagicLinkSender, error) {
	if strings.TrimSpace(os.Getenv("CONTROL_SMTP_HOST")) != "" {
		return newSMTPMagicLinkSenderFromEnv()
	}

	path := strings.TrimSpace(os.Getenv("CONTROL_MAGIC_LINK_LOG_PATH"))
	if path == "" {
		path = "control_magic_links.txt"
	}

	return &fileMagicLinkSender{
		path:    path,
		baseURL: strings.TrimSpace(os.Getenv("CONTROL_MAGIC_LINK_BASE_URL")),
	}, nil
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

func (s *smtpMagicLinkSender) DeliveryMode() string      { return "smtp" }
func (s *smtpMagicLinkSender) ExposeMagicLinkCode() bool { return false }

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

func (s *fileMagicLinkSender) DeliveryMode() string      { return "file" }
func (s *fileMagicLinkSender) ExposeMagicLinkCode() bool { return true }

func (s *fileMagicLinkSender) SendMagicLink(ctx context.Context, msg MagicLinkMessage) error {
	if err := ctx.Err(); err != nil {
		return err
	}

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

	var b strings.Builder
	b.WriteString("timestamp=")
	b.WriteString(time.Now().UTC().Format(time.RFC3339))
	b.WriteString("\n")
	b.WriteString("to=")
	b.WriteString(msg.ToEmail)
	b.WriteString("\n")
	b.WriteString("magic_link_id=")
	b.WriteString(msg.MagicLinkID)
	b.WriteString("\n")
	b.WriteString("code=")
	b.WriteString(msg.Code)
	b.WriteString("\n")
	b.WriteString("org_id=")
	b.WriteString(msg.OrgID)
	b.WriteString("\n")
	b.WriteString("user_id=")
	b.WriteString(msg.UserID)
	b.WriteString("\n")
	b.WriteString("role=")
	b.WriteString(msg.Role)
	b.WriteString("\n")
	b.WriteString("expires_at=")
	b.WriteString(msg.ExpiresAt.UTC().Format(time.RFC3339))
	b.WriteString("\n")
	if link != "" {
		b.WriteString("link=")
		b.WriteString(link)
		b.WriteString("\n")
	}
	b.WriteString("---\n")

	if dir := filepath.Dir(s.path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create magic link log dir: %w", err)
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	file, err := os.OpenFile(s.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open magic link log file: %w", err)
	}
	defer file.Close()

	if _, err := file.WriteString(b.String()); err != nil {
		return fmt.Errorf("write magic link log file: %w", err)
	}
	return nil
}
