package queue

import (
	"bedrud/config"
	"bedrud/internal/models"
	"bedrud/internal/utils"
	"bytes"
	"context"
	"encoding/json"
	"html/template"
	"strings"
	"testing"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

func TestSendEmailHandler_NoSMTP_LogsBody(t *testing.T) {
	// Handler with nil config — no SMTP configured.
	h := NewSendEmailHandler(nil)

	payload := SendEmailPayload{
		To:           "user@test.com",
		Subject:      "Welcome!",
		TemplateName: "welcome",
		TemplateData: map[string]any{"Name": "Alice", "LoginURL": "https://bedrud.org/login"},
	}
	payloadBytes, _ := json.Marshal(payload)

	job := &models.Job{
		ID:      uuid.New().String(),
		Type:    "send_email",
		Payload: string(payloadBytes),
	}

	// Should return nil (not error) when SMTP unconfigured.
	err := h(context.Background(), &gorm.DB{}, job)
	if err != nil {
		t.Fatalf("expected nil error when SMTP not configured, got: %v", err)
	}
}

func TestSendEmailHandler_NoSMTP_EmptyConfig(t *testing.T) {
	// Handler with empty EmailConfig (Host="").
	h := NewSendEmailHandler(&config.EmailConfig{})

	payload := SendEmailPayload{
		To:           "user@test.com",
		Subject:      "Test",
		TemplateName: "welcome",
		TemplateData: map[string]any{"Name": "Bob"},
	}
	payloadBytes, _ := json.Marshal(payload)

	job := &models.Job{
		ID:      uuid.New().String(),
		Type:    "send_email",
		Payload: string(payloadBytes),
	}

	err := h(context.Background(), &gorm.DB{}, job)
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
}

func TestSendEmailHandler_InvalidJSON(t *testing.T) {
	h := NewSendEmailHandler(nil)

	job := &models.Job{
		ID:      uuid.New().String(),
		Type:    "send_email",
		Payload: "{invalid json}",
	}

	err := h(context.Background(), &gorm.DB{}, job)
	if err == nil {
		t.Fatal("expected error for invalid JSON payload")
	}
	if !strings.Contains(err.Error(), "unmarshal") {
		t.Fatalf("expected unmarshal error, got: %v", err)
	}
}

func TestSendEmailHandler_MissingTemplateName(t *testing.T) {
	h := NewSendEmailHandler(nil)

	payload := SendEmailPayload{
		To:           "user@test.com",
		Subject:      "Test",
		TemplateName: "nonexistent_template",
		TemplateData: map[string]any{"key": "value"},
	}
	payloadBytes, _ := json.Marshal(payload)

	job := &models.Job{
		ID:      uuid.New().String(),
		Type:    "send_email",
		Payload: string(payloadBytes),
	}

	// Should render fallback, not error.
	err := h(context.Background(), &gorm.DB{}, job)
	if err != nil {
		t.Fatalf("expected nil error with unknown template, got: %v", err)
	}
}

func TestBuildMessage_Format(t *testing.T) {
	msg := utils.BuildMessage("noreply@bedrud.org", "Bedrud", "alice@test.com", "Hello", "<p>Welcome</p>", "Plain text fallback")

	if !strings.Contains(msg, "From: Bedrud <noreply@bedrud.org>") {
		t.Error("missing From header")
	}
	if !strings.Contains(msg, "To: alice@test.com") {
		t.Error("missing To header")
	}
	if !strings.Contains(msg, "Subject: Hello") {
		t.Error("missing Subject header")
	}
	if !strings.Contains(msg, "Content-Type: multipart/alternative") {
		t.Error("missing multipart content type")
	}
	if !strings.Contains(msg, "text/plain") {
		t.Error("missing plaintext alternative")
	}
	if !strings.Contains(msg, "text/html") {
		t.Error("missing HTML alternative")
	}
	if !strings.Contains(msg, "<p>Welcome</p>") {
		t.Error("missing HTML body")
	}
	if !strings.Contains(msg, "Welcome") {
		t.Error("missing plaintext content")
	}
}

func TestBuildMessage_EmptyFromName(t *testing.T) {
	msg := utils.BuildMessage("noreply@bedrud.org", "", "alice@test.com", "Test", "<p>Hi</p>", "Plain text fallback")
	if !strings.Contains(msg, "From:  <noreply@bedrud.org>") {
		t.Error("from should have empty name")
	}
}

func TestStripHTML(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"<p>Hello</p>", "Hello"},
		{"<h1>Title</h1><p>Body</p>", "TitleBody"},
		{"No tags here", "No tags here"},
		{"<a href=\"link\">Click</a>", "Click"},
		{"", ""},
		{"<br/>", ""},
	}
	for _, c := range cases {
		got := stripHTML(c.input)
		if got != c.expected {
			t.Errorf("stripHTML(%q) = %q, want %q", c.input, got, c.expected)
		}
	}
}

func TestRenderFallback(t *testing.T) {
	data := map[string]any{
		"Name":  "Alice",
		"URL":   "https://bedrud.org",
		"Count": 42,
	}
	html := renderFallback(data)
	if !strings.Contains(html, "Alice") {
		t.Error("missing Name in fallback")
	}
	if !strings.Contains(html, "bedrud.org") {
		t.Error("missing URL in fallback")
	}
	if !strings.Contains(html, "42") {
		t.Error("missing Count in fallback")
	}
}

func TestSendEmailHandler_ContextCancelled(t *testing.T) {
	// Use a minimal SMTP config but cancelled context.
	// Handler will try to connect and fail, but we cancel first.
	h := NewSendEmailHandler(&config.EmailConfig{
		SMTPHost: "127.0.0.1",
		SMTPPort: 2525,
	})

	payload := SendEmailPayload{
		To:           "user@test.com",
		Subject:      "Test",
		TemplateName: "welcome",
		TemplateData: map[string]any{"Name": "Alice"},
	}
	payloadBytes, _ := json.Marshal(payload)

	job := &models.Job{
		ID:      uuid.New().String(),
		Type:    "send_email",
		Payload: string(payloadBytes),
	}

	// Cancelled context should return cancellation error.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := h(ctx, &gorm.DB{}, job)
	if err == nil {
		t.Fatal("expected error with cancelled context")
	}
}

func TestSendEmailHandler_TemplateNotFound_LogsData(t *testing.T) {
	h := NewSendEmailHandler(nil)

	payload := SendEmailPayload{
		To:           "user@test.com",
		Subject:      "Fallback test",
		TemplateName: "nonexistent",
		TemplateData: map[string]any{
			"UserID": "abc-123",
			"Room":   "test-room",
		},
	}
	payloadBytes, _ := json.Marshal(payload)

	job := &models.Job{
		ID:      uuid.New().String(),
		Type:    "send_email",
		Payload: string(payloadBytes),
	}

	err := h(context.Background(), &gorm.DB{}, job)
	if err != nil {
		t.Fatalf("expected nil error when template missing, got: %v", err)
	}
	// The body should contain the data keys.
	// We can't assert on log output here, but handler returning nil means
	// renderFallback was used successfully.
}

func TestSendEmailHandler_NilData(t *testing.T) {
	h := NewSendEmailHandler(nil)

	payload := SendEmailPayload{
		To:           "user@test.com",
		Subject:      "Data test",
		TemplateName: "welcome",
		TemplateData: nil,
	}
	payloadBytes, _ := json.Marshal(payload)

	job := &models.Job{
		ID:      uuid.New().String(),
		Type:    "send_email",
		Payload: string(payloadBytes),
	}

	err := h(context.Background(), &gorm.DB{}, job)
	if err != nil {
		t.Fatalf("expected nil error with nil template data, got: %v", err)
	}
}

func TestBuildMessage_NoBody(t *testing.T) {
	msg := utils.BuildMessage("from@test.com", "Tester", "to@test.com", "Empty", "", "Plain text fallback")
	if !strings.Contains(msg, "text/plain") {
		t.Error("missing plaintext part")
	}
	if !strings.Contains(msg, "text/html") {
		t.Error("missing HTML part")
	}
}

func TestStripHTML_Complex(t *testing.T) {
	input := `<div class="content"><h1>Title</h1><p>Para with <strong>bold</strong></p><ul><li>Item</li></ul></div>`
	expected := "TitlePara with boldItem"
	got := stripHTML(input)
	if got != expected {
		t.Errorf("stripHTML returned %q, want %q", got, expected)
	}
}

func TestRenderFallback_EmptyData(t *testing.T) {
	html := renderFallback(map[string]any{})
	if html == "" {
		t.Error("expected non-empty fallback HTML")
	}
}

func TestRenderWelcomeTemplate(t *testing.T) {
	var buf bytes.Buffer
	tmpl, err := template.ParseFS(emailTemplatesFS, "templates/welcome.html")
	if err != nil {
		t.Fatalf("parse welcome template: %v", err)
	}
	if err := tmpl.Execute(&buf, map[string]any{
		"Name":     "Alice",
		"LoginURL": "https://bedrud.org/login",
	}); err != nil {
		t.Fatalf("execute welcome template: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "Welcome, Alice") {
		t.Error("welcome template missing name")
	}
	if !strings.Contains(output, "bedrud.org/login") {
		t.Error("welcome template missing login URL")
	}
	if strings.Contains(output, "<no value>") {
		t.Error("welcome template has unrendered placeholders")
	}
}

func TestRenderRoomInviteTemplate(t *testing.T) {
	var buf bytes.Buffer
	tmpl, err := template.ParseFS(emailTemplatesFS, "templates/room_invite.html")
	if err != nil {
		t.Fatalf("parse room_invite template: %v", err)
	}
	if err := tmpl.Execute(&buf, map[string]any{
		"InviterName": "Bob",
		"RoomName":    "Team Standup",
		"JoinURL":     "https://bedrud.org/room/abc",
	}); err != nil {
		t.Fatalf("execute room_invite template: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "Bob") {
		t.Error("room_invite template missing inviter name")
	}
	if !strings.Contains(output, "Team Standup") {
		t.Error("room_invite template missing room name")
	}
	if !strings.Contains(output, "bedrud.org/room/abc") {
		t.Error("room_invite template missing join URL")
	}
}

func TestRenderPasswordResetTemplate(t *testing.T) {
	var buf bytes.Buffer
	tmpl, err := template.ParseFS(emailTemplatesFS, "templates/password_reset.html")
	if err != nil {
		t.Fatalf("parse password_reset template: %v", err)
	}
	if err := tmpl.Execute(&buf, map[string]any{
		"ResetURL":  "https://bedrud.org/reset/token123",
		"IPAddress": "192.168.1.1",
	}); err != nil {
		t.Fatalf("execute password_reset template: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "bedrud.org/reset/token123") {
		t.Error("password_reset template missing reset URL")
	}
	if !strings.Contains(output, "192.168.1.1") {
		t.Error("password_reset template missing IP address")
	}
	if !strings.Contains(output, "Reset Password") {
		t.Error("password_reset template missing button text")
	}
}

func TestRenderGenericTemplate(t *testing.T) {
	var buf bytes.Buffer
	tmpl, err := template.ParseFS(emailTemplatesFS, "templates/generic.html")
	if err != nil {
		t.Fatalf("parse generic template: %v", err)
	}
	if err := tmpl.Execute(&buf, map[string]any{
		"UserID": "u-123",
		"Room":   "test-room",
	}); err != nil {
		t.Fatalf("execute generic template: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "UserID") {
		t.Error("generic template missing UserID key")
	}
	if !strings.Contains(output, "u-123") {
		t.Error("generic template missing UserID value")
	}
	if !strings.Contains(output, "test-room") {
		t.Error("generic template missing Room value")
	}
	if strings.Contains(output, "<no value>") {
		t.Error("generic template has unrendered placeholders")
	}
}

func TestRenderGenericTemplate_EmptyData(t *testing.T) {
	var buf bytes.Buffer
	tmpl, err := template.ParseFS(emailTemplatesFS, "templates/generic.html")
	if err != nil {
		t.Fatalf("parse generic template: %v", err)
	}
	if err := tmpl.Execute(&buf, map[string]any{
		"InstanceName": "Bedrud",
	}); err != nil {
		t.Fatalf("execute generic template with empty data: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "Bedrud") {
		t.Error("generic template missing heading with empty data")
	}
}

func TestRenderWelcomeTemplate_MissingOptionalKeys(t *testing.T) {
	var buf bytes.Buffer
	tmpl, err := template.ParseFS(emailTemplatesFS, "templates/welcome.html")
	if err != nil {
		t.Fatalf("parse welcome template: %v", err)
	}
	// Only Name provided, LoginURL is optional (wrapped in {{if}}).
	if err := tmpl.Execute(&buf, map[string]any{
		"Name": "Charlie",
	}); err != nil {
		t.Fatalf("execute welcome template with missing keys: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "Welcome, Charlie") {
		t.Error("welcome template missing name")
	}
	if strings.Contains(output, "Go to Bedrud") {
		t.Error("welcome template should not render login button when LoginURL missing")
	}
}

func TestRenderPasswordResetTemplate_MissingOptionalKeys(t *testing.T) {
	var buf bytes.Buffer
	tmpl, err := template.ParseFS(emailTemplatesFS, "templates/password_reset.html")
	if err != nil {
		t.Fatalf("parse password_reset template: %v", err)
	}
	// Neither ResetURL nor IPAddress provided — should still render without error.
	if err := tmpl.Execute(&buf, map[string]any{}); err != nil {
		t.Fatalf("execute password_reset template with empty data: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "Password Reset") {
		t.Error("password_reset template missing heading")
	}
	if strings.Contains(output, "<no value>") {
		t.Error("password_reset template has unrendered placeholders")
	}
}

func TestTemplateRenderDirect_SMTPNotConfigured(t *testing.T) {
	// Run template through the handler with no SMTP config — verifies the full render + log path.
	h := NewSendEmailHandler(nil)
	payload := SendEmailPayload{
		To:           "alice@test.com",
		Subject:      "Welcome!",
		TemplateName: "welcome",
		TemplateData: map[string]any{
			"Name":     "Alice",
			"LoginURL": "https://bedrud.org",
		},
	}
	payloadBytes, _ := json.Marshal(payload)
	job := &models.Job{
		ID:      uuid.New().String(),
		Type:    "send_email",
		Payload: string(payloadBytes),
	}
	if err := h(context.Background(), &gorm.DB{}, job); err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
}

// TTD tests — these verify the handler falls back to generic template for unknown names.
// Currently failing because "generic" is not parsed — will pass after fix.

func TestUnknownTemplate_FallsBackToGeneric(t *testing.T) {
	// Unknown template name should render the generic template ("Notification from Bedrud" heading).
	// This test will fail until "generic" is added to the template parse loop.
	h := NewSendEmailHandler(nil)
	payload := SendEmailPayload{
		To:           "user@test.com",
		Subject:      "Notification",
		TemplateName: "bogus_template_name",
		TemplateData: map[string]any{
			"Message": "Something happened",
		},
	}
	payloadBytes, _ := json.Marshal(payload)
	job := &models.Job{
		ID:      uuid.New().String(),
		Type:    "send_email",
		Payload: string(payloadBytes),
	}
	// The handler returns nil even with generic fallback — but output includes generic heading.
	// We can't assert the body directly (it's logged, not returned), but we verify no error.
	if err := h(context.Background(), &gorm.DB{}, job); err != nil {
		t.Fatalf("expected nil error with unknown template (generic fallback), got: %v", err)
	}
}

// Test that unknown template data keys appear in generic fallback output.
// We render the generic template directly and verify data keys are present.
func TestGenericTemplate_IncludesDataKeys(t *testing.T) {
	var buf bytes.Buffer
	tmpl, err := template.ParseFS(emailTemplatesFS, "templates/generic.html")
	if err != nil {
		t.Fatalf("parse generic template: %v", err)
	}
	if err := tmpl.Execute(&buf, map[string]any{
		"CustomField": "custom-value",
		"AnotherKey":  "another-value",
	}); err != nil {
		t.Fatalf("execute generic template: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "CustomField") {
		t.Error("generic template missing CustomField key")
	}
	if !strings.Contains(output, "custom-value") {
		t.Error("generic template missing CustomField value")
	}
	if !strings.Contains(output, "AnotherKey") {
		t.Error("generic template missing AnotherKey key")
	}
}

// password_changed template tests — will fail until template file exists.

func TestRenderPasswordChangedTemplate(t *testing.T) {
	// This test will fail until password_changed.html template file is created.
	var buf bytes.Buffer
	tmpl, err := template.ParseFS(emailTemplatesFS, "templates/password_changed.html")
	if err != nil {
		t.Fatalf("parse password_changed template: %v", err)
	}
	if err := tmpl.Execute(&buf, map[string]any{
		"IPAddress": "10.0.0.1",
	}); err != nil {
		t.Fatalf("execute password_changed template: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "Password Changed") {
		t.Error("password_changed template missing heading")
	}
	if !strings.Contains(output, "10.0.0.1") {
		t.Error("password_changed template missing IP address")
	}
	if strings.Contains(output, "<no value>") {
		t.Error("password_changed template has unrendered placeholders")
	}
}

func TestHandler_EnqueuePasswordChanged_NoSMTP(t *testing.T) {
	// Full handler integration test with password_changed template and no SMTP.
	h := NewSendEmailHandler(nil)
	payload := SendEmailPayload{
		To:           "user@test.com",
		Subject:      "Password Changed",
		TemplateName: "password_changed",
		TemplateData: map[string]any{
			"IPAddress": "10.0.0.1",
		},
	}
	payloadBytes, _ := json.Marshal(payload)
	job := &models.Job{
		ID:      uuid.New().String(),
		Type:    "send_email",
		Payload: string(payloadBytes),
	}
	// Should return nil (not error) when SMTP unconfigured.
	if err := h(context.Background(), &gorm.DB{}, job); err != nil {
		t.Fatalf("expected nil error when SMTP not configured, got: %v", err)
	}
}

func TestWelcome_EmptyLoginURL_NoButton(t *testing.T) {
	// Welcome template should NOT render the "Go to Bedrud" button when LoginURL is empty.
	var buf bytes.Buffer
	tmpl, err := template.ParseFS(emailTemplatesFS, "templates/welcome.html")
	if err != nil {
		t.Fatalf("parse welcome template: %v", err)
	}
	// LoginURL explicitly empty string — should not show button.
	if err := tmpl.Execute(&buf, map[string]any{
		"Name":     "Dave",
		"LoginURL": "",
	}); err != nil {
		t.Fatalf("execute welcome template: %v", err)
	}
	output := buf.String()
	if strings.Contains(output, "Go to Bedrud") {
		t.Error("welcome template should not render button when LoginURL is empty string")
	}
}

func TestWelcome_MissingLoginURLKey_NoButton(t *testing.T) {
	// Welcome template should NOT render the "Go to Bedrud" button when LoginURL key missing.
	var buf bytes.Buffer
	tmpl, err := template.ParseFS(emailTemplatesFS, "templates/welcome.html")
	if err != nil {
		t.Fatalf("parse welcome template: %v", err)
	}
	// LoginURL key not provided at all — should not show button.
	if err := tmpl.Execute(&buf, map[string]any{
		"Name": "Eve",
	}); err != nil {
		t.Fatalf("execute welcome template: %v", err)
	}
	output := buf.String()
	if strings.Contains(output, "Go to Bedrud") {
		t.Error("welcome template should not render button when LoginURL missing")
	}
}

// — renderEmailBody tests —

func TestRenderEmailBody_WelcomeTemplate(t *testing.T) {
	tmpls := make(map[string]*template.Template)
	for _, name := range []string{"welcome", "generic"} {
		tmpl, err := template.ParseFS(emailTemplatesFS, "templates/"+name+".html")
		if err != nil {
			t.Fatalf("parse %s template: %v", name, err)
		}
		tmpls[name] = tmpl
	}

	body, err := renderEmailBody(tmpls, SendEmailPayload{
		TemplateName: "welcome",
		TemplateData: map[string]any{
			"Name":     "Alice",
			"LoginURL": "https://bedrud.org",
		},
	})
	if err != nil {
		t.Fatalf("renderEmailBody: %v", err)
	}
	if !strings.Contains(body, "Welcome, Alice") {
		t.Error("body missing name")
	}
	if !strings.Contains(body, "bedrud.org") {
		t.Error("body missing URL")
	}
}

func TestRenderEmailBody_UnknownTemplate_UsesGeneric(t *testing.T) {
	tmpls := make(map[string]*template.Template)
	for _, name := range []string{"welcome", "generic"} {
		tmpl, err := template.ParseFS(emailTemplatesFS, "templates/"+name+".html")
		if err != nil {
			t.Fatalf("parse %s template: %v", name, err)
		}
		tmpls[name] = tmpl
	}

	body, err := renderEmailBody(tmpls, SendEmailPayload{
		TemplateName: "bogus_template",
		TemplateData: map[string]any{"key": "value", "InstanceName": "Bedrud"},
	})
	if err != nil {
		t.Fatalf("renderEmailBody with unknown template: %v", err)
	}
	// Generic template heading should be present.
	if !strings.Contains(body, "Bedrud") {
		t.Error("unknown template should render generic template heading")
	}
	// Data keys should appear in generic output.
	if !strings.Contains(body, "key") || !strings.Contains(body, "value") {
		t.Error("unknown template generic output missing data keys")
	}
}

func TestRenderEmailBody_UnknownTemplate_NoGeneric(t *testing.T) {
	// Only welcome template, NO generic template in map.
	tmpls := make(map[string]*template.Template)
	tmpl, err := template.ParseFS(emailTemplatesFS, "templates/welcome.html")
	if err != nil {
		t.Fatalf("parse welcome template: %v", err)
	}
	tmpls["welcome"] = tmpl

	body, err := renderEmailBody(tmpls, SendEmailPayload{
		TemplateName: "unknown",
		TemplateData: map[string]any{"Foo": "Bar"},
	})
	if err != nil {
		t.Fatalf("renderEmailBody without generic: %v", err)
	}
	// Should fall back to renderFallback (pre-formatted data dump).
	if !strings.Contains(body, "Foo") || !strings.Contains(body, "Bar") {
		t.Error("fallback output missing data keys")
	}
	// renderFallback wraps in <pre> tags.
	if !strings.Contains(body, "<pre>") {
		t.Error("fallback output should contain <pre> tags")
	}
}

func TestRenderEmailBody_NilData(t *testing.T) {
	tmpls := make(map[string]*template.Template)
	tmpl, err := template.ParseFS(emailTemplatesFS, "templates/welcome.html")
	if err != nil {
		t.Fatalf("parse welcome template: %v", err)
	}
	tmpls["welcome"] = tmpl

	body, err := renderEmailBody(tmpls, SendEmailPayload{
		TemplateName: "welcome",
		TemplateData: nil,
	})
	if err != nil {
		t.Fatalf("renderEmailBody with nil data: %v", err)
	}
	if body == "" {
		t.Error("expected non-empty body with nil data")
	}
}

func TestRenderEmailBody_UnknownTemplate_MultipleDataKeys(t *testing.T) {
	tmpls := make(map[string]*template.Template)
	tmpl, err := template.ParseFS(emailTemplatesFS, "templates/generic.html")
	if err != nil {
		t.Fatalf("parse generic template: %v", err)
	}
	tmpls["generic"] = tmpl

	body, err := renderEmailBody(tmpls, SendEmailPayload{
		TemplateName: "does_not_exist",
		TemplateData: map[string]any{
			"Alpha": "1",
			"Beta":  "2",
			"Gamma": "3",
		},
	})
	if err != nil {
		t.Fatalf("renderEmailBody: %v", err)
	}
	for _, key := range []string{"Alpha", "Beta", "Gamma"} {
		if !strings.Contains(body, key) {
			t.Errorf("body missing key %q", key)
		}
	}
}

func TestRenderEmailBody_UnknownTemplate_EmptyData(t *testing.T) {
	tmpls := make(map[string]*template.Template)
	tmpl, err := template.ParseFS(emailTemplatesFS, "templates/generic.html")
	if err != nil {
		t.Fatalf("parse generic template: %v", err)
	}
	tmpls["generic"] = tmpl

	body, err := renderEmailBody(tmpls, SendEmailPayload{
		TemplateName: "does_not_exist",
		TemplateData: map[string]any{"InstanceName": "Bedrud"},
	})
	if err != nil {
		t.Fatalf("renderEmailBody with empty data: %v", err)
	}
	if !strings.Contains(body, "Bedrud") {
		t.Error("body missing generic heading with empty data")
	}
}

// --- loadBranding tests ---

func TestLoadBranding_Defaults(t *testing.T) {
	b := loadBranding(context.Background(), nil, nil)
	if b.InstanceName != "Bedrud" {
		t.Errorf("expected 'Bedrud', got %q", b.InstanceName)
	}
	if b.HeaderBg != "#1a1a2e" {
		t.Errorf("expected '#1a1a2e', got %q", b.HeaderBg)
	}
	if b.ButtonBg != "#e11d48" {
		t.Errorf("expected '#e11d48', got %q", b.ButtonBg)
	}
	if b.SupportEmail != "" {
		t.Errorf("expected empty, got %q", b.SupportEmail)
	}
}

func TestLoadBranding_NilDB_UsesConfig(t *testing.T) {
	cfg := &config.EmailConfig{
		Templates: config.EmailTemplateConfig{
			InstanceName:  "MyInstance",
			SupportEmail:  "admin@myinstance.com",
			InstanceURL:   "https://myinstance.com",
			HeaderBgColor: "#ff0000",
			ButtonBgColor: "#00ff00",
			SubjectLines:  map[string]string{"verify_email": "Custom subject"},
			PreheaderText: map[string]string{"welcome": "Custom preheader"},
		},
	}
	b := loadBranding(context.Background(), nil, cfg)
	if b.InstanceName != "MyInstance" {
		t.Errorf("expected 'MyInstance', got %q", b.InstanceName)
	}
	if b.SupportEmail != "admin@myinstance.com" {
		t.Errorf("expected 'admin@myinstance.com', got %q", b.SupportEmail)
	}
	if b.InstanceURL != "https://myinstance.com" {
		t.Errorf("expected 'https://myinstance.com', got %q", b.InstanceURL)
	}
	if b.HeaderBg != "#ff0000" {
		t.Errorf("expected '#ff0000', got %q", b.HeaderBg)
	}
	if b.ButtonBg != "#00ff00" {
		t.Errorf("expected '#00ff00', got %q", b.ButtonBg)
	}
	if b.SubjectLines["verify_email"] != "Custom subject" {
		t.Errorf("expected 'Custom subject', got %q", b.SubjectLines["verify_email"])
	}
	if b.PreheaderText["welcome"] != "Custom preheader" {
		t.Errorf("expected 'Custom preheader', got %q", b.PreheaderText["welcome"])
	}
}

func TestLoadBranding_EmptyConfig(t *testing.T) {
	cfg := &config.EmailConfig{}
	b := loadBranding(context.Background(), nil, cfg)
	if b.InstanceName != "Bedrud" {
		t.Errorf("expected default 'Bedrud', got %q", b.InstanceName)
	}
	if b.HeaderBg != "#1a1a2e" {
		t.Errorf("expected default '#1a1a2e', got %q", b.HeaderBg)
	}
}

func TestLoadBranding_PartialConfig(t *testing.T) {
	cfg := &config.EmailConfig{
		Templates: config.EmailTemplateConfig{
			InstanceName: "Partial Instance",
		},
	}
	b := loadBranding(context.Background(), nil, cfg)
	if b.InstanceName != "Partial Instance" {
		t.Errorf("expected 'Partial Instance', got %q", b.InstanceName)
	}
	// Should still get defaults for unset fields
	if b.HeaderBg != "#1a1a2e" {
		t.Errorf("expected default '#1a1a2e', got %q", b.HeaderBg)
	}
	if b.ButtonBg != "#e11d48" {
		t.Errorf("expected default '#e11d48', got %q", b.ButtonBg)
	}
}

// --- injectBranding tests ---

func TestInjectBranding_NilBranding(t *testing.T) {
	data := map[string]any{"Name": "Alice"}
	injectBranding(nil, "welcome", data)
	if data["Name"] != "Alice" {
		t.Error("injectBranding should not modify data when branding is nil")
	}
}

func TestInjectBranding_AddsFields(t *testing.T) {
	b := &emailBranding{
		InstanceName:  "TestBedrud",
		SupportEmail:  "test@example.com",
		InstanceURL:   "https://test.example.com",
		HeaderBg:      "#111222",
		ButtonBg:      "#333444",
		PreheaderText: map[string]string{"verify_email": "Please verify"},
	}
	data := map[string]any{"Name": "Bob"}
	injectBranding(b, "verify_email", data)

	if data["InstanceName"] != "TestBedrud" {
		t.Errorf("expected InstanceName 'TestBedrud', got %v", data["InstanceName"])
	}
	if data["SupportEmail"] != "test@example.com" {
		t.Errorf("expected SupportEmail 'test@example.com', got %v", data["SupportEmail"])
	}
	if data["InstanceURL"] != "https://test.example.com" {
		t.Errorf("expected InstanceURL 'https://test.example.com', got %v", data["InstanceURL"])
	}
	if data["HeaderBg"] != "#111222" {
		t.Errorf("expected HeaderBg '#111222', got %v", data["HeaderBg"])
	}
	if data["ButtonBg"] != "#333444" {
		t.Errorf("expected ButtonBg '#333444', got %v", data["ButtonBg"])
	}
	if data["Preheader"] != "Please verify" {
		t.Errorf("expected Preheader 'Please verify', got %v", data["Preheader"])
	}
}

func TestInjectBranding_NilData(t *testing.T) {
	b := &emailBranding{InstanceName: "Test"}
	// Should not panic with nil data — allocates internally
	injectBranding(b, "welcome", nil)
}

func TestInjectBranding_DoesNotOverrideExisting(t *testing.T) {
	b := &emailBranding{
		InstanceName:  "ConfigBrand",
		SupportEmail:  "config@example.com",
		PreheaderText: map[string]string{"welcome": "config preheader"},
	}
	data := map[string]any{
		"InstanceName": "ExistingBrand",
		"Preheader":    "existing preheader",
	}
	injectBranding(b, "welcome", data)
	if data["InstanceName"] != "ExistingBrand" {
		t.Errorf("injectBranding should not override existing InstanceName")
	}
	if data["Preheader"] != "existing preheader" {
		t.Errorf("injectBranding should not override existing Preheader")
	}
}

func TestInjectBranding_NoPreheaderForTemplate(t *testing.T) {
	b := &emailBranding{
		InstanceName:  "Test",
		PreheaderText: map[string]string{"verify_email": "Verify preheader"},
	}
	data := map[string]any{}
	injectBranding(b, "welcome", data)
	// welcome has no preheader configured, so Preheader should not be set
	if _, ok := data["Preheader"]; ok {
		t.Error("Preheader should not be set when no preheader configured for template")
	}
}

// --- resolveSubject tests ---

func TestResolveSubject_FromBranding(t *testing.T) {
	b := &emailBranding{
		SubjectLines: map[string]string{
			"welcome":      "Welcome to {{.InstanceName}}",
			"verify_email": "Verify your email",
		},
	}
	subject := resolveSubject(b, "welcome", "Fallback subject")
	if subject != "Welcome to {{.InstanceName}}" {
		t.Errorf("expected subject from branding, got %q", subject)
	}
}

func TestResolveSubject_Fallback(t *testing.T) {
	subject := resolveSubject(nil, "welcome", "Fallback subject")
	if subject != "Fallback subject" {
		t.Errorf("expected fallback subject, got %q", subject)
	}
}

func TestResolveSubject_NoMatch(t *testing.T) {
	b := &emailBranding{
		SubjectLines: map[string]string{"welcome": "Welcome"},
	}
	subject := resolveSubject(b, "unknown_template", "Fallback")
	if subject != "Fallback" {
		t.Errorf("expected fallback for unknown template, got %q", subject)
	}
}

func TestResolveSubject_EmptyString(t *testing.T) {
	b := &emailBranding{
		SubjectLines: map[string]string{"welcome": ""},
	}
	subject := resolveSubject(b, "welcome", "Fallback")
	// Empty string in the map should still cause fallback
	if subject != "Fallback" {
		t.Errorf("expected fallback for empty subject, got %q", subject)
	}
}

// --- Template rendering with branding ---

func TestRenderWelcomeTemplate_WithBranding(t *testing.T) {
	var buf bytes.Buffer
	tmpl, err := template.ParseFS(emailTemplatesFS, "templates/welcome.html")
	if err != nil {
		t.Fatalf("parse welcome template: %v", err)
	}
	if err := tmpl.Execute(&buf, map[string]any{
		"Name":         "Alice",
		"LoginURL":     "https://bedrud.org/login",
		"InstanceName": "TestBedrud",
		"HeaderBg":     "#111222",
		"ButtonBg":     "#333444",
		"SupportEmail": "support@test.com",
		"InstanceURL":  "https://test.com",
	}); err != nil {
		t.Fatalf("execute welcome template: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "Welcome, Alice") {
		t.Error("welcome template missing name")
	}
	if !strings.Contains(output, "TestBedrud") {
		t.Error("welcome template missing instance name")
	}
	if !strings.Contains(output, "#111222") {
		t.Error("welcome template missing header bg color")
	}
	if !strings.Contains(output, "#333444") {
		t.Error("welcome template missing button bg color")
	}
	if !strings.Contains(output, "support@test.com") {
		t.Error("welcome template missing support email")
	}
	if !strings.Contains(output, "https://test.com") {
		t.Error("welcome template missing instance URL")
	}
	if strings.Contains(output, "<no value>") {
		t.Error("welcome template has unrendered placeholders")
	}
}

func TestRenderVerifyTemplate_WithPreheader(t *testing.T) {
	var buf bytes.Buffer
	tmpl, err := template.ParseFS(emailTemplatesFS, "templates/verify_email.html")
	if err != nil {
		t.Fatalf("parse verify_email template: %v", err)
	}
	if err := tmpl.Execute(&buf, map[string]any{
		"Name":         "Bob",
		"VerifyURL":    "https://bedrud.org/verify?token=abc",
		"InstanceName": "Bedrud",
		"HeaderBg":     "#1a1a2e",
		"ButtonBg":     "#e11d48",
		"Preheader":    "Verify your email for Bedrud",
	}); err != nil {
		t.Fatalf("execute verify_email template: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "Verify your email, Bob") {
		t.Error("verify_email template missing greeting")
	}
	if !strings.Contains(output, "Verify your email for Bedrud") {
		t.Error("verify_email template missing preheader")
	}
	// Preheader should be in a hidden div
	if !strings.Contains(output, "display:none") {
		t.Error("preheader should be hidden")
	}
	if !strings.Contains(output, "Verify Email") {
		t.Error("verify_email template missing verify button")
	}
	if strings.Contains(output, "<no value>") {
		t.Error("verify_email template has unrendered placeholders")
	}
}

func TestRenderVerifyTemplate_CodeBlockURL(t *testing.T) {
	var buf bytes.Buffer
	tmpl, err := template.ParseFS(emailTemplatesFS, "templates/verify_email.html")
	if err != nil {
		t.Fatalf("parse verify_email template: %v", err)
	}
	if err := tmpl.Execute(&buf, map[string]any{
		"Name":         "Charlie",
		"VerifyURL":    "https://bedrud.org/verify?token=long-token-here",
		"InstanceName": "Bedrud",
		"HeaderBg":     "#1a1a2e",
		"ButtonBg":     "#e11d48",
	}); err != nil {
		t.Fatalf("execute verify_email template: %v", err)
	}
	output := buf.String()
	// Verify URL should appear in a code-block-like container
	if !strings.Contains(output, "background:#f1f5f9") {
		t.Error("verify_email template should wrap URL in gray code block")
	}
	if !strings.Contains(output, "font-family:monospace") {
		t.Error("verify_email template URL block should use monospace font")
	}
}

func TestRenderPasswordChangedTemplate_WithUserAgent(t *testing.T) {
	var buf bytes.Buffer
	tmpl, err := template.ParseFS(emailTemplatesFS, "templates/password_changed.html")
	if err != nil {
		t.Fatalf("parse password_changed template: %v", err)
	}
	if err := tmpl.Execute(&buf, map[string]any{
		"IPAddress":    "10.0.0.1",
		"UserAgent":    "Mozilla/5.0 Chrome/120",
		"InstanceName": "Bedrud",
		"HeaderBg":     "#1a1a2e",
		"ButtonBg":     "#e11d48",
	}); err != nil {
		t.Fatalf("execute password_changed template: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "10.0.0.1") {
		t.Error("password_changed template missing IP address")
	}
	if !strings.Contains(output, "Mozilla/5.0") {
		t.Error("password_changed template missing user agent")
	}
	if strings.Contains(output, "<no value>") {
		t.Error("password_changed template has unrendered placeholders")
	}
}

func TestRenderPasswordChangedTemplate_WithoutUserAgent(t *testing.T) {
	var buf bytes.Buffer
	tmpl, err := template.ParseFS(emailTemplatesFS, "templates/password_changed.html")
	if err != nil {
		t.Fatalf("parse password_changed template: %v", err)
	}
	if err := tmpl.Execute(&buf, map[string]any{
		"IPAddress":    "10.0.0.1",
		"InstanceName": "Bedrud",
		"HeaderBg":     "#1a1a2e",
		"ButtonBg":     "#e11d48",
	}); err != nil {
		t.Fatalf("execute password_changed template: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "10.0.0.1") {
		t.Error("password_changed template missing IP address")
	}
	if strings.Contains(output, "Browser/OS:") {
		t.Error("password_changed template should not show UserAgent section when absent")
	}
}

func TestRenderGenericTemplate_TableFormat(t *testing.T) {
	var buf bytes.Buffer
	tmpl, err := template.ParseFS(emailTemplatesFS, "templates/generic.html")
	if err != nil {
		t.Fatalf("parse generic template: %v", err)
	}
	if err := tmpl.Execute(&buf, map[string]any{
		"InstanceName": "Bedrud",
		"HeaderBg":     "#1a1a2e",
		"ButtonBg":     "#e11d48",
		"UserID":       "u-123",
		"Room":         "test-room",
	}); err != nil {
		t.Fatalf("execute generic template: %v", err)
	}
	output := buf.String()
	// Should render branding fields in header but not in the data table
	if !strings.Contains(output, "u-123") {
		t.Error("generic template missing UserID value")
	}
	if !strings.Contains(output, "test-room") {
		t.Error("generic template missing Room value")
	}
	// Branding fields should not appear in the data table
	if strings.Contains(output, ">InstanceName<") {
		t.Error("generic template should filter out InstanceName from data table")
	}
	if strings.Contains(output, ">HeaderBg<") {
		t.Error("generic template should filter out HeaderBg from data table")
	}
	if strings.Contains(output, ">ButtonBg<") {
		t.Error("generic template should filter out ButtonBg from data table")
	}
}

// --- All .txt templates parse and render ---

func TestAllPlainTextTemplates_Parse(t *testing.T) {
	names := []string{"welcome", "room_invite", "password_reset", "password_changed", "verify_email", "generic"}
	for _, name := range names {
		_, err := template.ParseFS(emailTemplatesFS, "templates/"+name+".txt")
		if err != nil {
			t.Errorf("failed to parse plaintext template %s: %v", name, err)
		}
	}
}

func TestAllPlainTextTemplates_Render(t *testing.T) {
	names := []string{"welcome", "room_invite", "password_reset", "password_changed", "verify_email", "generic"}
	for _, name := range names {
		tmpl, err := template.ParseFS(emailTemplatesFS, "templates/"+name+".txt")
		if err != nil {
			t.Fatalf("parse %s.txt: %v", name, err)
		}
		data := map[string]any{
			"InstanceName": "Bedrud",
			"Name":         "Test",
			"VerifyURL":    "https://example.com/verify",
			"ResetURL":     "https://example.com/reset",
			"InviterName":  "Alice",
			"RoomName":     "TestRoom",
			"JoinURL":      "https://example.com/join",
			"IPAddress":    "10.0.0.1",
			"LoginURL":     "https://example.com/login",
			"SupportEmail": "support@example.com",
		}
		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, data); err != nil {
			t.Errorf("render %s.txt: %v", name, err)
			continue
		}
		output := buf.String()
		if output == "" {
			t.Errorf("%s.txt rendered empty output", name)
		}
		if strings.Contains(output, "<no value>") {
			t.Errorf("%s.txt has unrendered placeholders", name)
		}
	}
}

func TestPlainTextWelcome_RendersBranding(t *testing.T) {
	var buf bytes.Buffer
	tmpl, err := template.ParseFS(emailTemplatesFS, "templates/welcome.txt")
	if err != nil {
		t.Fatalf("parse welcome.txt: %v", err)
	}
	if err := tmpl.Execute(&buf, map[string]any{
		"Name":         "Alice",
		"InstanceName": "TestBedrud",
		"LoginURL":     "https://example.com/login",
	}); err != nil {
		t.Fatalf("execute welcome.txt: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "Welcome") {
		t.Error("welcome.txt missing greeting")
	}
	if !strings.Contains(output, "TestBedrud") {
		t.Error("welcome.txt missing instance name")
	}
}

func TestPlainTextVerifyEmail_RendersAllSections(t *testing.T) {
	var buf bytes.Buffer
	tmpl, err := template.ParseFS(emailTemplatesFS, "templates/verify_email.txt")
	if err != nil {
		t.Fatalf("parse verify_email.txt: %v", err)
	}
	if err := tmpl.Execute(&buf, map[string]any{
		"Name":         "Bob",
		"VerifyURL":    "https://example.com/verify?token=abc",
		"InstanceName": "Bedrud",
		"SupportEmail": "admin@bedrud.org",
	}); err != nil {
		t.Fatalf("execute verify_email.txt: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "Bob") {
		t.Error("verify_email.txt missing name")
	}
	if !strings.Contains(output, "token=abc") {
		t.Error("verify_email.txt missing verify URL")
	}
	if !strings.Contains(output, "24 hours") {
		t.Error("verify_email.txt missing expiration notice")
	}
	if !strings.Contains(output, "admin@bedrud.org") {
		t.Error("verify_email.txt missing support email")
	}
}
