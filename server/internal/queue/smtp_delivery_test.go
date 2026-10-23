package queue

import (
	"context"
	"errors"
	"net/smtp"
	"strings"
	"testing"
	"time"

	"bedrud/internal/models"
)

// capturedDelivery records the arguments one fake SMTP delivery received.
type capturedDelivery struct {
	addr string
	from string
	to   []string
	msg  string
}

// stubDeliverSMTP replaces the package SMTP transport for one test and restores it afterwards.
func stubDeliverSMTP(t *testing.T, result error) *capturedDelivery {
	t.Helper()
	captured := &capturedDelivery{}
	original := deliverSMTP
	deliverSMTP = func(addr string, auth smtp.Auth, from string, to []string, msg []byte, host string, tlsSkipVerify, smtpsMode bool) error {
		captured.addr = addr
		captured.from = from
		captured.to = to
		captured.msg = string(msg)
		return result
	}
	t.Cleanup(func() { deliverSMTP = original })
	return captured
}

func TestSMTPSettingsFrom_AppliesDefaults(t *testing.T) {
	settings := SMTPSettingsFrom(&models.SystemSettings{EmailSMTPHost: "smtp.example.com", EmailSMTPPort: 587})

	if settings.FromAddress != "noreply@bedrud" {
		t.Errorf("expected default from address, got %q", settings.FromAddress)
	}
	if settings.FromName != "Bedrud" {
		t.Errorf("expected default from name, got %q", settings.FromName)
	}
	if !settings.Configured() {
		t.Error("expected settings with host and port to be configured")
	}
}

func TestSMTPSettingsFrom_FromNameFallsBackToInstanceName(t *testing.T) {
	settings := SMTPSettingsFrom(&models.SystemSettings{EmailInstanceName: "Acme Meet"})

	if settings.FromName != "Acme Meet" {
		t.Errorf("expected instance name as from name, got %q", settings.FromName)
	}
}

func TestSMTPSettingsFrom_NilIsNotConfigured(t *testing.T) {
	if SMTPSettingsFrom(nil).Configured() {
		t.Error("expected nil settings to be unconfigured")
	}
}

func TestSendSMTPMail_NotConfigured(t *testing.T) {
	stubDeliverSMTP(t, nil)

	err := SendSMTPMail(context.Background(), SMTPSettings{}, "user@test.com", "Subject", "<p>Hi</p>", "Hi")

	if !errors.Is(err, ErrSMTPNotConfigured) {
		t.Fatalf("expected ErrSMTPNotConfigured, got %v", err)
	}
}

func TestSendSMTPMail_DeliversWithResolvedSettings(t *testing.T) {
	captured := stubDeliverSMTP(t, nil)
	settings := SMTPSettingsFrom(&models.SystemSettings{
		EmailSMTPHost:    "smtp.example.com",
		EmailSMTPPort:    2525,
		EmailFromAddress: "mail@example.com",
		EmailFromName:    "Example",
	})

	err := SendSMTPMail(context.Background(), settings, "user@test.com", "Subject", "<p>Hi</p>", "Hi")

	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if captured.addr != "smtp.example.com:2525" {
		t.Errorf("expected address smtp.example.com:2525, got %q", captured.addr)
	}
	if captured.from != "mail@example.com" || len(captured.to) != 1 || captured.to[0] != "user@test.com" {
		t.Errorf("unexpected envelope: from=%q to=%v", captured.from, captured.to)
	}
	if !strings.Contains(captured.msg, "From: Example <mail@example.com>\r\n") {
		t.Errorf("expected display name before address in From header, got message:\n%s", captured.msg)
	}
}

func TestSendSMTPMail_WrapsTransportError(t *testing.T) {
	transportErr := errors.New("smtp dial: connection refused")
	stubDeliverSMTP(t, transportErr)
	settings := SMTPSettings{Host: "smtp.example.com", Port: 587}

	err := SendSMTPMail(context.Background(), settings, "user@test.com", "Subject", "<p>Hi</p>", "Hi")

	if !errors.Is(err, transportErr) {
		t.Fatalf("expected wrapped transport error, got %v", err)
	}
}

func TestSendSMTPMail_DeadlineReturnsTimeout(t *testing.T) {
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	original := deliverSMTP
	deliverSMTP = func(string, smtp.Auth, string, []string, []byte, string, bool, bool) error {
		<-release
		return nil
	}
	t.Cleanup(func() { deliverSMTP = original })
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	err := SendSMTPMail(ctx, SMTPSettings{Host: "smtp.example.com", Port: 587}, "user@test.com", "Subject", "<p>Hi</p>", "Hi")

	if !errors.Is(err, ErrSMTPTimeout) {
		t.Fatalf("expected ErrSMTPTimeout, got %v", err)
	}
}
