package repository

import (
	"bedrud/config"
	"bedrud/internal/models"
	"bedrud/internal/testutil"
	"testing"
)

func TestSettingsRepository_GetSettings_CreatesDefault(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewSettingsRepository(db)

	settings, err := repo.GetSettings()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if settings == nil {
		t.Fatal("expected non-nil settings")
	}
	// Default: registration is enabled
	if !settings.RegistrationEnabled {
		t.Fatal("expected RegistrationEnabled to default to true")
	}
	if settings.TokenRegistrationOnly {
		t.Fatal("expected TokenRegistrationOnly to default to false")
	}
}

func TestSettingsRepository_GetSettings_Idempotent(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewSettingsRepository(db)

	s1, _ := repo.GetSettings()
	s2, _ := repo.GetSettings()

	if s1.ID != s2.ID {
		t.Fatalf("expected same ID on repeated GetSettings, got %d vs %d", s1.ID, s2.ID)
	}
}

func TestSettingsRepository_SaveSettings_TokenRegistrationOnly(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewSettingsRepository(db)

	// Ensure row exists first
	_, _ = repo.GetSettings()

	err := repo.SaveSettings(&models.SystemSettings{
		RegistrationEnabled:   true,
		TokenRegistrationOnly: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	saved, _ := repo.GetSettings()
	if !saved.TokenRegistrationOnly {
		t.Fatal("expected TokenRegistrationOnly to be true after save")
	}
}

func TestSettingsRepository_SaveSettings_MultipleUpdates(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewSettingsRepository(db)

	// First save
	_ = repo.SaveSettings(&models.SystemSettings{
		RegistrationEnabled:   true,
		TokenRegistrationOnly: true,
	})

	// Second save — overrides
	_ = repo.SaveSettings(&models.SystemSettings{
		RegistrationEnabled:   true,
		TokenRegistrationOnly: false,
	})

	saved, _ := repo.GetSettings()
	if saved.TokenRegistrationOnly {
		t.Fatal("expected TokenRegistrationOnly to be false after second save")
	}
}

func TestMergeFromConfig_EmailBranding_ConfigFillsEmpty(t *testing.T) {
	s := &models.SystemSettings{}
	cfg := testConfigWithEmailBranding()
	mergeFromConfig(s, cfg)

	if s.EmailInstanceName != "TestInstance" {
		t.Errorf("expected 'TestInstance', got %q", s.EmailInstanceName)
	}
	if s.EmailSupportEmail != "admin@test.com" {
		t.Errorf("expected 'admin@test.com', got %q", s.EmailSupportEmail)
	}
	if s.EmailInstanceURL != "https://test.com" {
		t.Errorf("expected 'https://test.com', got %q", s.EmailInstanceURL)
	}
	if s.EmailHeaderBg != "#ff0000" {
		t.Errorf("expected '#ff0000', got %q", s.EmailHeaderBg)
	}
	if s.EmailButtonBg != "#00ff00" {
		t.Errorf("expected '#00ff00', got %q", s.EmailButtonBg)
	}
}

func TestMergeFromConfig_EmailBranding_DBWins(t *testing.T) {
	s := &models.SystemSettings{
		EmailInstanceName: "DBName",
		EmailHeaderBg:     "#ffffff",
	}
	cfg := testConfigWithEmailBranding()
	mergeFromConfig(s, cfg)

	// DB values should be kept
	if s.EmailInstanceName != "DBName" {
		t.Errorf("expected 'DBName' (DB wins), got %q", s.EmailInstanceName)
	}
	if s.EmailHeaderBg != "#ffffff" {
		t.Errorf("expected '#ffffff' (DB wins), got %q", s.EmailHeaderBg)
	}
	// Config values fill only empty fields
	if s.EmailSupportEmail != "admin@test.com" {
		t.Errorf("expected 'admin@test.com' from config, got %q", s.EmailSupportEmail)
	}
}

func TestMergeFromConfig_EmailSubject_ConfigFillsEmpty(t *testing.T) {
	s := &models.SystemSettings{}
	cfg := testConfigWithEmailBranding()
	mergeFromConfig(s, cfg)

	if s.EmailSubjectVerify != "Verify your TestInstance email" {
		t.Errorf("expected subject from config, got %q", s.EmailSubjectVerify)
	}
	if s.EmailSubjectWelcome != "Welcome to TestInstance" {
		t.Errorf("expected subject from config, got %q", s.EmailSubjectWelcome)
	}
}

func TestMergeFromConfig_EmailSubject_DBWins(t *testing.T) {
	s := &models.SystemSettings{
		EmailSubjectVerify: "DB Override Subject",
	}
	cfg := testConfigWithEmailBranding()
	mergeFromConfig(s, cfg)

	if s.EmailSubjectVerify != "DB Override Subject" {
		t.Errorf("expected 'DB Override Subject' (DB wins), got %q", s.EmailSubjectVerify)
	}
	if s.EmailSubjectWelcome != "Welcome to TestInstance" {
		t.Errorf("expected subject from config, got %q", s.EmailSubjectWelcome)
	}
}

func TestMergeFromConfig_EmailPreheader_ConfigFillsEmpty(t *testing.T) {
	s := &models.SystemSettings{}
	cfg := testConfigWithEmailBranding()
	mergeFromConfig(s, cfg)

	if s.EmailPreheaderVerify != "Verify your email for TestInstance" {
		t.Errorf("expected preheader from config, got %q", s.EmailPreheaderVerify)
	}
}

func TestMergeFromConfig_EmailSMTP_ConfigFillsEmpty(t *testing.T) {
	s := &models.SystemSettings{}
	cfg := testConfigWithEmailBranding()
	mergeFromConfig(s, cfg)

	if s.EmailSMTPHost != "smtp.test.com" {
		t.Errorf("expected 'smtp.test.com', got %q", s.EmailSMTPHost)
	}
	if s.EmailSMTPPort != 587 {
		t.Errorf("expected 587, got %d", s.EmailSMTPPort)
	}
	if s.EmailFromAddress != "noreply@test.com" {
		t.Errorf("expected 'noreply@test.com', got %q", s.EmailFromAddress)
	}
}

func TestMergeFromConfig_EmailSMTP_DBWins(t *testing.T) {
	s := &models.SystemSettings{
		EmailSMTPHost: "db.smtp.com",
		EmailSMTPPort: 465,
	}
	cfg := testConfigWithEmailBranding()
	mergeFromConfig(s, cfg)

	if s.EmailSMTPHost != "db.smtp.com" {
		t.Errorf("expected 'db.smtp.com' (DB wins), got %q", s.EmailSMTPHost)
	}
	if s.EmailSMTPPort != 465 {
		t.Errorf("expected 465 (DB wins), got %d", s.EmailSMTPPort)
	}
}

// testConfigWithEmailBranding creates a test config with email branding fields set.
func testConfigWithEmailBranding() *config.Config {
	return &config.Config{
		Email: config.EmailConfig{
			SMTPHost:      "smtp.test.com",
			SMTPPort:      587,
			Username:      "user",
			Password:      "pass",
			FromAddress:   "noreply@test.com",
			FromName:      "Test",
			TLSSkipVerify: false,
			SMTPSMode:     false,
			Templates: config.EmailTemplateConfig{
				InstanceName:  "TestInstance",
				SupportEmail:  "admin@test.com",
				InstanceURL:   "https://test.com",
				HeaderBgColor: "#ff0000",
				ButtonBgColor: "#00ff00",
				SubjectLines: map[string]string{
					"verify_email":     "Verify your TestInstance email",
					"welcome":          "Welcome to TestInstance",
					"password_reset":   "Reset your TestInstance password",
					"password_changed": "Your TestInstance password was changed",
					"room_invite":      "You're invited to TestInstance",
				},
				PreheaderText: map[string]string{
					"verify_email": "Verify your email for TestInstance",
					"welcome":      "Welcome to TestInstance!",
				},
			},
		},
	}
}
