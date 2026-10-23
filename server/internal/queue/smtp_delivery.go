package queue

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/smtp"
	"strconv"
	"time"

	"bedrud/internal/models"
	"bedrud/internal/utils"
)

const (
	defaultEmailFromAddress = "noreply@bedrud"
	smtpSendTimeout         = 30 * time.Second
)

var (
	// ErrSMTPNotConfigured reports that no SMTP host and port are set in settings or config.yaml.
	ErrSMTPNotConfigured = errors.New("mail is not configured: set the SMTP host and port in Email settings")
	// ErrSMTPTimeout reports that the mail server did not finish the exchange in time.
	ErrSMTPTimeout = fmt.Errorf("mail server did not respond within %s", smtpSendTimeout)
)

// deliverSMTP is the SMTP transport; tests replace it to avoid real network calls.
var deliverSMTP = utils.SendSMTP

// EffectiveSettingsSource returns system settings with database values overlaid on config.yaml.
type EffectiveSettingsSource interface {
	GetEffectiveSettings() (*models.SystemSettings, error)
}

// SMTPSettings is the SMTP transport resolved from effective system settings.
type SMTPSettings struct {
	Host          string
	Port          int
	Username      string
	Password      string
	FromAddress   string
	FromName      string
	TLSSkipVerify bool
	SMTPSMode     bool
}

// Configured reports whether a host and port are set, the minimum needed to connect.
func (s SMTPSettings) Configured() bool {
	return s.Host != "" && s.Port != 0
}

// SMTPSettingsFrom resolves the SMTP transport from effective settings, filling sender defaults.
func SMTPSettingsFrom(s *models.SystemSettings) SMTPSettings {
	if s == nil {
		s = &models.SystemSettings{}
	}
	resolved := SMTPSettings{
		Host:          s.EmailSMTPHost,
		Port:          s.EmailSMTPPort,
		Username:      s.EmailUsername,
		Password:      s.EmailPassword,
		FromAddress:   s.EmailFromAddress,
		FromName:      s.EmailFromName,
		TLSSkipVerify: s.EmailTLSSkipVerify,
		SMTPSMode:     s.EmailSMTPSMode,
	}
	if resolved.FromAddress == "" {
		resolved.FromAddress = defaultEmailFromAddress
	}
	if resolved.FromName == "" {
		resolved.FromName = s.EmailInstanceName
	}
	if resolved.FromName == "" {
		resolved.FromName = defaultEmailFromName
	}
	return resolved
}

// SendSMTPMail sends one message synchronously, bounded by ctx and smtpSendTimeout.
func SendSMTPMail(ctx context.Context, s SMTPSettings, to, subject, bodyHTML, bodyPlain string) error {
	if !s.Configured() {
		return ErrSMTPNotConfigured
	}

	addr := net.JoinHostPort(s.Host, strconv.Itoa(s.Port))
	var auth smtp.Auth
	if s.Username != "" {
		auth = smtp.PlainAuth("", s.Username, s.Password, s.Host)
	}
	msg := utils.BuildMessage(s.FromAddress, s.FromName, to, subject, bodyHTML, bodyPlain)

	ctx, cancel := context.WithTimeout(ctx, smtpSendTimeout)
	defer cancel()

	// Read the transport once here: a send that outlives the timeout must not race a later swap.
	deliver := deliverSMTP
	done := make(chan error, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				done <- fmt.Errorf("panic in SMTP send: %v", r)
			}
		}()
		done <- deliver(addr, auth, s.FromAddress, []string{to}, []byte(msg), s.Host, s.TLSSkipVerify, s.SMTPSMode)
	}()

	select {
	case err := <-done:
		if err != nil {
			return fmt.Errorf("smtp send: %w", err)
		}
		return nil
	case <-ctx.Done():
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return ErrSMTPTimeout
		}
		return ctx.Err()
	}
}
