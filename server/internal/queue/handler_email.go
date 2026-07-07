package queue

import (
	"bedrud/config"
	"bedrud/internal/models"
	"bedrud/internal/utils"
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"net"
	"net/smtp"
	"time"

	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
)

//go:embed templates/*.html templates/*.txt
var emailTemplatesFS embed.FS

// emailHandler holds SMTP config, parsed templates, and email branding config.
type emailHandler struct {
	cfg        *config.EmailConfig
	tmpls      map[string]*template.Template
	plainTmpls map[string]*template.Template
}

// NewSendEmailHandler creates a handler that sends transactional emails via SMTP.
// When SMTP is not configured, the handler logs and skips (no-op, no error).
func NewSendEmailHandler(cfg *config.EmailConfig) Handler {
	h := &emailHandler{cfg: cfg}

	h.tmpls = make(map[string]*template.Template)
	h.plainTmpls = make(map[string]*template.Template)
	for _, name := range []string{"welcome", "room_invite", "password_reset", "password_changed", "verify_email", "generic"} {
		t, err := template.ParseFS(emailTemplatesFS, "templates/"+name+".html")
		if err != nil {
			log.Warn().Err(err).Str("template", name).Msg("email: failed to parse HTML template, will fall back to plain text")
			continue
		}
		h.tmpls[name] = t

		pt, err := template.ParseFS(emailTemplatesFS, "templates/"+name+".txt")
		if err == nil {
			h.plainTmpls[name] = pt
		}
	}

	h.validateConfig()

	return func(ctx context.Context, db *gorm.DB, job *models.Job) error {
		var payload SendEmailPayload
		if err := json.Unmarshal([]byte(job.Payload), &payload); err != nil {
			return fmt.Errorf("unmarshal send_email payload: %w", err)
		}

		branding := loadBranding(ctx, db, h.cfg)
		templateName := payload.TemplateName

		// Resolve subject: DB override -> config subject line -> payload subject -> hardcoded
		subject := resolveSubject(branding, templateName, payload.Subject)

		// Inject branding fields into template data
		data := payload.TemplateData
		if data == nil {
			data = make(map[string]any)
		}
		injectBranding(branding, templateName, data)

		bodyHTML, err := renderEmailBody(h.tmpls, SendEmailPayload{
			TemplateName: templateName,
			TemplateData: data,
		})
		if err != nil {
			return err
		}

		if h.cfg == nil || h.cfg.SMTPHost == "" || h.cfg.SMTPPort == 0 {
			log.Warn().Str("to", payload.To).Str("subject", subject).
				Str("template", templateName).
				Str("body", bodyHTML).
				Interface("data", data).
				Msg("email: SMTP not configured, skipping send — body logged")
			return nil
		}

		from := h.cfg.FromAddress
		if from == "" {
			from = "noreply@bedrud"
		}
		fromName := h.cfg.FromName
		if fromName == "" {
			fromName = "Bedrud"
		}

		bodyPlain := renderPlaintextBody(h.plainTmpls, SendEmailPayload{
			TemplateName: templateName,
			TemplateData: data,
		})

		msg := utils.BuildMessage(from, fromName, payload.To, subject, bodyHTML, bodyPlain)

		addr := net.JoinHostPort(h.cfg.SMTPHost, fmt.Sprint(h.cfg.SMTPPort))

		var auth smtp.Auth
		if h.cfg.Username != "" {
			auth = smtp.PlainAuth("", h.cfg.Username, h.cfg.Password, h.cfg.SMTPHost)
		}

		done := make(chan error, 1)
		go func() {
			defer func() {
				if r := recover(); r != nil {
					done <- fmt.Errorf("panic in sendSMTP: %v", r)
				}
			}()
			done <- utils.SendSMTP(addr, auth, from, []string{payload.To}, []byte(msg),
				h.cfg.SMTPHost, h.cfg.TLSSkipVerify, h.cfg.SMTPSMode)
		}()

		timer := time.NewTimer(30 * time.Second)
		defer timer.Stop()
		select {
		case err := <-done:
			if err != nil {
				return fmt.Errorf("smtp send: %w", err)
			}
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			return fmt.Errorf("smtp send timed out after 30s")
		}

		log.Info().Str("to", payload.To).Str("subject", subject).
			Str("template", templateName).
			Msg("email: sent successfully")
		return nil
	}
}

// emailBranding holds the merged branding values for email rendering.
type emailBranding struct {
	InstanceName  string
	SupportEmail  string
	InstanceURL   string
	HeaderBg      string
	ButtonBg      string
	SubjectLines  map[string]string
	PreheaderText map[string]string
}

// loadBranding loads effective email branding from SystemSettings DB overlaid on config.yaml defaults.
func loadBranding(ctx context.Context, db *gorm.DB, cfg *config.EmailConfig) *emailBranding {
	b := &emailBranding{
		InstanceName:  "Bedrud",
		HeaderBg:      "#1a1a2e",
		ButtonBg:      "#e11d48",
		SubjectLines:  make(map[string]string),
		PreheaderText: make(map[string]string),
	}

	if cfg != nil {
		if cfg.Templates.InstanceName != "" {
			b.InstanceName = cfg.Templates.InstanceName
		}
		if cfg.Templates.SupportEmail != "" {
			b.SupportEmail = cfg.Templates.SupportEmail
		}
		if cfg.Templates.InstanceURL != "" {
			b.InstanceURL = cfg.Templates.InstanceURL
		}
		if cfg.Templates.HeaderBgColor != "" {
			b.HeaderBg = cfg.Templates.HeaderBgColor
		}
		if cfg.Templates.ButtonBgColor != "" {
			b.ButtonBg = cfg.Templates.ButtonBgColor
		}
		for k, v := range cfg.Templates.SubjectLines {
			b.SubjectLines[k] = v
		}
		for k, v := range cfg.Templates.PreheaderText {
			b.PreheaderText[k] = v
		}
	}

	if db == nil || db.Statement == nil || db.Statement.DB == nil {
		return b
	}

	db = db.WithContext(ctx)
	var s models.SystemSettings
	if err := db.First(&s, 1).Error; err != nil {
		return b
	}

	if s.EmailInstanceName != "" {
		b.InstanceName = s.EmailInstanceName
	}
	if s.EmailSupportEmail != "" {
		b.SupportEmail = s.EmailSupportEmail
	}
	if s.EmailInstanceURL != "" {
		b.InstanceURL = s.EmailInstanceURL
	}
	if s.EmailHeaderBg != "" {
		b.HeaderBg = s.EmailHeaderBg
	}
	if s.EmailButtonBg != "" {
		b.ButtonBg = s.EmailButtonBg
	}

	if s.EmailSubjectVerify != "" {
		b.SubjectLines["verify_email"] = s.EmailSubjectVerify
	}
	if s.EmailSubjectWelcome != "" {
		b.SubjectLines["welcome"] = s.EmailSubjectWelcome
	}
	if s.EmailSubjectReset != "" {
		b.SubjectLines["password_reset"] = s.EmailSubjectReset
	}
	if s.EmailSubjectChanged != "" {
		b.SubjectLines["password_changed"] = s.EmailSubjectChanged
	}
	if s.EmailSubjectInvite != "" {
		b.SubjectLines["room_invite"] = s.EmailSubjectInvite
	}

	if s.EmailPreheaderVerify != "" {
		b.PreheaderText["verify_email"] = s.EmailPreheaderVerify
	}
	if s.EmailPreheaderWelcome != "" {
		b.PreheaderText["welcome"] = s.EmailPreheaderWelcome
	}
	if s.EmailPreheaderReset != "" {
		b.PreheaderText["password_reset"] = s.EmailPreheaderReset
	}
	if s.EmailPreheaderChanged != "" {
		b.PreheaderText["password_changed"] = s.EmailPreheaderChanged
	}
	if s.EmailPreheaderInvite != "" {
		b.PreheaderText["room_invite"] = s.EmailPreheaderInvite
	}

	return b
}

// resolveSubject returns the configured subject line or falls back to the payload subject.
func resolveSubject(b *emailBranding, templateName, fallback string) string {
	if b != nil {
		if s, ok := b.SubjectLines[templateName]; ok && s != "" {
			return s
		}
	}
	return fallback
}

// injectBranding merges branding fields into the template data map.
func injectBranding(b *emailBranding, templateName string, data map[string]any) {
	if b == nil || data == nil {
		return
	}
	if _, ok := data["InstanceName"]; !ok {
		data["InstanceName"] = b.InstanceName
	}
	if _, ok := data["SupportEmail"]; !ok {
		data["SupportEmail"] = b.SupportEmail
	}
	if _, ok := data["InstanceURL"]; !ok {
		data["InstanceURL"] = b.InstanceURL
	}
	if _, ok := data["HeaderBg"]; !ok {
		data["HeaderBg"] = b.HeaderBg
	}
	if _, ok := data["ButtonBg"]; !ok {
		data["ButtonBg"] = b.ButtonBg
	}
	if _, ok := data["Preheader"]; !ok {
		if pt, ok := b.PreheaderText[templateName]; ok {
			data["Preheader"] = pt
		}
	}
}

// renderEmailBody resolves the template by name and renders it with the payload data.
func renderEmailBody(tmpls map[string]*template.Template, payload SendEmailPayload) (string, error) {
	tmpl, ok := tmpls[payload.TemplateName]
	if ok {
		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, payload.TemplateData); err != nil {
			return "", fmt.Errorf("render template %s: %w", payload.TemplateName, err)
		}
		return buf.String(), nil
	}
	if genericTmpl, ok := tmpls["generic"]; ok {
		var fb bytes.Buffer
		if err := genericTmpl.Execute(&fb, payload.TemplateData); err != nil {
			return renderFallback(payload.TemplateData), nil
		}
		return fb.String(), nil
	}
	return renderFallback(payload.TemplateData), nil
}

func (h *emailHandler) validateConfig() {
	if h.cfg == nil || h.cfg.SMTPHost == "" {
		return
	}
	if h.cfg.FromAddress == "" {
		log.Warn().Msg("email: smtpHost set but fromAddress empty — using 'noreply@bedrud' (may be rejected)")
	}
	if h.cfg.SMTPPort == 0 {
		log.Warn().Msg("email: smtpPort is 0 — SMTP connection will fail")
	}
	if h.cfg.Username != "" && h.cfg.Password == "" {
		log.Warn().Msg("email: username set but password empty — auth may fail")
	}
}

func renderPlaintextBody(plainTmpls map[string]*template.Template, payload SendEmailPayload) string {
	pt, ok := plainTmpls[payload.TemplateName]
	if ok && pt != nil {
		var buf bytes.Buffer
		if err := pt.Execute(&buf, payload.TemplateData); err == nil {
			return buf.String()
		}
	}
	bodyHTML, _ := renderEmailBody(nil, payload)
	return stripHTML(bodyHTML)
}

func stripHTML(html string) string {
	var buf bytes.Buffer
	inTag := false
	for _, r := range html {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
		case !inTag:
			buf.WriteRune(r)
		}
	}
	return buf.String()
}

func renderFallback(data map[string]any) string {
	var buf bytes.Buffer
	buf.WriteString("<html><body><pre>")
	for k, v := range data {
		buf.WriteString(fmt.Sprintf("%s: %v\n", k, v))
	}
	buf.WriteString("</pre></body></html>")
	return buf.String()
}
