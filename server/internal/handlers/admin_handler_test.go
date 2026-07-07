package handlers

import (
	"bedrud/config"
	"bedrud/internal/auth"
	"bedrud/internal/models"
	"bedrud/internal/repository"
	"bedrud/internal/testutil"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
)

func setupAdminTestApp(t *testing.T) (*fiber.App, *repository.SettingsRepository, *repository.InviteTokenRepository) {
	t.Helper()
	config.SetForTest(&config.Config{})
	db := testutil.SetupTestDB(t)
	settingsRepo := repository.NewSettingsRepository(db)
	inviteTokenRepo := repository.NewInviteTokenRepository(db)
	webhookRepo := repository.NewWebhookRepository(db)
	recordingRepo := repository.NewRecordingRepository(db)
	adminHandler := NewAdminHandler(settingsRepo, inviteTokenRepo, webhookRepo, recordingRepo)

	app := fiber.New()
	// Inject admin claims for all routes
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user", &auth.Claims{
			UserID:   "admin-user-id",
			Email:    "admin@example.com",
			Name:     "Admin",
			Accesses: []string{"superadmin"},
		})
		c.Locals("userID", "admin-user-id")
		return c.Next()
	})

	app.Get("/admin/settings", adminHandler.GetSettings)
	app.Put("/admin/settings", adminHandler.UpdateSettings)
	app.Post("/admin/settings/validate", adminHandler.ValidateSettingsConnectivity)
	app.Post("/admin/settings/send-test-email", adminHandler.SendTestEmail)
	app.Get("/public/settings", adminHandler.GetPublicSettings)
	app.Get("/admin/invite-tokens", adminHandler.ListInviteTokens)
	app.Post("/admin/invite-tokens", adminHandler.CreateInviteToken)
	app.Delete("/admin/invite-tokens/:id", adminHandler.DeleteInviteToken)
	// Webhook routes
	app.Get("/admin/webhooks", adminHandler.ListWebhooks)
	app.Post("/admin/webhooks", adminHandler.CreateWebhook)
	app.Put("/admin/webhooks/:id", adminHandler.UpdateWebhook)
	app.Delete("/admin/webhooks/:id", adminHandler.DeleteWebhook)
	app.Post("/admin/webhooks/:id/rotate-secret", adminHandler.RotateWebhookSecret)
	app.Post("/admin/webhooks/:id/test", adminHandler.TestWebhook)

	return app, settingsRepo, inviteTokenRepo
}

func TestAdminHandler_GetSettings_Default(t *testing.T) {
	app, _, _ := setupAdminTestApp(t)

	req := httptest.NewRequest(http.MethodGet, "/admin/settings", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected %d, got %d: %s", http.StatusOK, resp.StatusCode, string(body))
	}
}

func TestAdminHandler_GetPublicSettings(t *testing.T) {
	app, _, _ := setupAdminTestApp(t)

	req := httptest.NewRequest(http.MethodGet, "/public/settings", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected %d, got %d", http.StatusOK, resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	_ = json.Unmarshal(body, &result)
	if _, ok := result["registrationEnabled"]; !ok {
		t.Fatal("expected 'registrationEnabled' in public settings response")
	}
	if _, ok := result["tokenRegistrationOnly"]; !ok {
		t.Fatal("expected 'tokenRegistrationOnly' in public settings response")
	}
}

func TestAdminHandler_UpdateSettings_Success(t *testing.T) {
	app, settingsRepo, _ := setupAdminTestApp(t)

	body, _ := json.Marshal(map[string]interface{}{
		"registrationEnabled":   false,
		"tokenRegistrationOnly": true,
	})
	req := httptest.NewRequest(http.MethodPut, "/admin/settings", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected %d, got %d: %s", http.StatusOK, resp.StatusCode, string(respBody))
	}

	// Verify the settings were persisted
	saved, err := settingsRepo.GetSettings()
	if err != nil {
		t.Fatalf("unexpected error reading settings: %v", err)
	}
	if !saved.TokenRegistrationOnly {
		t.Fatal("expected TokenRegistrationOnly to be true after update")
	}
}

func TestAdminHandler_UpdateSettings_FailsOnTypeMismatch(t *testing.T) {
	app, _, _ := setupAdminTestApp(t)

	tests := []struct {
		name string
		body map[string]interface{}
	}{
		{"bool instead of string", map[string]interface{}{"serverPort": true}},
		{"array instead of string", map[string]interface{}{"serverHost": []string{"a", "b"}}},
		{"string instead of bool", map[string]interface{}{"serverEnableTls": "yes"}},
		{"string instead of int", map[string]interface{}{"corsMaxAge": "abc"}},
		{"array instead of int", map[string]interface{}{"tokenDuration": []int{1, 2}}},
		{"object instead of int64", map[string]interface{}{"chatUploadMaxBytes": map[string]int{"x": 1}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			body, _ := json.Marshal(tc.body)
			req := httptest.NewRequest(http.MethodPut, "/admin/settings", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			resp, err := app.Test(req, -1)
			if err != nil {
				t.Fatalf("app.Test failed: %v", err)
			}
			defer func() { _ = resp.Body.Close() }()
			if resp.StatusCode != http.StatusBadRequest {
				respBody, _ := io.ReadAll(resp.Body)
				t.Fatalf("expected 400, got %d: %s", resp.StatusCode, string(respBody))
			}
		})
	}
}

func TestAdminHandler_UpdateSettings_InvalidBody(t *testing.T) {
	app, _, _ := setupAdminTestApp(t)

	req := httptest.NewRequest(http.MethodPut, "/admin/settings", bytes.NewReader([]byte("{invalid")))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

func TestAdminHandler_ListInviteTokens_Empty(t *testing.T) {
	app, _, _ := setupAdminTestApp(t)

	req := httptest.NewRequest(http.MethodGet, "/admin/invite-tokens", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected %d, got %d", http.StatusOK, resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	_ = json.Unmarshal(body, &result)
	tokens, _ := result["tokens"].([]interface{})
	if len(tokens) != 0 {
		t.Fatalf("expected empty token list, got %d", len(tokens))
	}
}

func TestAdminHandler_CreateInviteToken_Success(t *testing.T) {
	app, _, _ := setupAdminTestApp(t)

	body, _ := json.Marshal(map[string]interface{}{
		"email":          "invited@example.com",
		"expiresInHours": 48,
	})
	req := httptest.NewRequest(http.MethodPost, "/admin/invite-tokens", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected %d, got %d: %s", http.StatusCreated, resp.StatusCode, string(respBody))
	}

	respBody, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	_ = json.Unmarshal(respBody, &result)
	if result["token"] == nil || result["token"] == "" {
		t.Fatal("expected 'token' field in response")
	}
	if result["id"] == nil {
		t.Fatal("expected 'id' field in response")
	}
}

func TestAdminHandler_CreateInviteToken_DefaultExpiry(t *testing.T) {
	app, _, _ := setupAdminTestApp(t)

	body, _ := json.Marshal(map[string]interface{}{"email": "default@example.com"})
	req := httptest.NewRequest(http.MethodPost, "/admin/invite-tokens", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected %d, got %d", http.StatusCreated, resp.StatusCode)
	}
}

func TestAdminHandler_DeleteInviteToken_Success(t *testing.T) {
	app, _, inviteTokenRepo := setupAdminTestApp(t)

	createBody, _ := json.Marshal(map[string]interface{}{
		"email":          "todelete@example.com",
		"expiresInHours": 24,
	})
	createReq := httptest.NewRequest(http.MethodPost, "/admin/invite-tokens", bytes.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createResp, err := app.Test(createReq, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = createResp.Body.Close() }()

	createRespBody, _ := io.ReadAll(createResp.Body)
	var created map[string]interface{}
	_ = json.Unmarshal(createRespBody, &created)
	tokenID, _ := created["id"].(string)

	tokens, _, _ := inviteTokenRepo.List(repository.PaginationParams{Page: 1, Limit: 50})
	if len(tokens) == 0 {
		t.Fatal("expected at least one token before delete")
	}

	req := httptest.NewRequest(http.MethodDelete, "/admin/invite-tokens/"+tokenID, http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected %d, got %d: %s", http.StatusOK, resp.StatusCode, string(respBody))
	}
}

// ---------------------------------------------------------------------------
// validateSettings tests
// ---------------------------------------------------------------------------

func defaultSettings() models.SystemSettings {
	return models.SystemSettings{
		RegistrationEnabled:   true,
		TokenRegistrationOnly: false,
		PasskeysEnabled:       true,
		ServerPort:            "8090",
		ServerHost:            "localhost",
		FrontendURL:           "http://localhost:3000",
		LogLevel:              "info",
	}
}

func TestValidateSettings_HappyPath(t *testing.T) {
	tests := []struct {
		name     string
		settings models.SystemSettings
	}{
		{"defaults", defaultSettings()},
		{"empty port", models.SystemSettings{ServerPort: ""}},
		{"TLS with manual certs", models.SystemSettings{
			ServerEnableTLS: true,
			ServerCertFile:  "/etc/cert.pem",
			ServerKeyFile:   "/etc/key.pem",
		}},
		{"external LK with credentials", models.SystemSettings{
			LiveKitExternal:  true,
			LiveKitAPIKey:    "key123",
			LiveKitAPISecret: "secret456",
		}},
		{"CORS no credentials", models.SystemSettings{
			CORSAllowedOrigins:   "*",
			CORSAllowCredentials: false,
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := validateSettings(&tc.settings); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidateSettings_Port(t *testing.T) {
	tests := []struct {
		port  string
		valid bool
	}{
		{"", true},
		{"80", true},
		{"65535", true},
		{"1", true},
		{"0", false},
		{"-1", false},
		{"65536", false},
		{"abc", false},
		{"12.5", false},
	}
	for _, tc := range tests {
		t.Run("port="+tc.port, func(t *testing.T) {
			s := defaultSettings()
			s.ServerPort = tc.port
			err := validateSettings(&s)
			if tc.valid && err != nil {
				t.Fatalf("expected ok, got: %v", err)
			}
			if !tc.valid && err == nil {
				t.Fatal("expected error, got nil")
			}
		})
	}
}

func TestValidateSettings_UploadLimits(t *testing.T) {
	tests := []struct {
		name     string
		maxBytes int64
		inline   int64
		valid    bool
	}{
		{"both zero", 0, 0, true},
		{"negative max", -1, 0, false},
		{"negative inline", 0, -1, false},
		{"both negative", -1, -1, false},
		{"positive values", 10485760, 512000, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := defaultSettings()
			s.ChatUploadMaxBytes = tc.maxBytes
			s.ChatUploadInlineMax = tc.inline
			err := validateSettings(&s)
			if tc.valid && err != nil {
				t.Fatalf("expected ok, got: %v", err)
			}
			if !tc.valid && err == nil {
				t.Fatal("expected error, got nil")
			}
		})
	}
}

func TestValidateSettings_CORSCredentialsWildcard(t *testing.T) {
	tests := []struct {
		origins     string
		credentials bool
		valid       bool
	}{
		{"*", true, false},
		{"", true, false},
		{"*,http://x.com", true, false},
		{"http://x.com,*", true, false},
		{"http://x.com", true, true},
		{"http://a.com,http://b.com", true, true},
		{"*", false, true},
	}
	for _, tc := range tests {
		t.Run("origins="+tc.origins, func(t *testing.T) {
			s := defaultSettings()
			s.CORSAllowedOrigins = tc.origins
			s.CORSAllowCredentials = tc.credentials
			err := validateSettings(&s)
			if tc.valid && err != nil {
				t.Fatalf("expected ok, got: %v", err)
			}
			if !tc.valid && err == nil {
				t.Fatal("expected error, got nil")
			}
		})
	}
}

func TestValidateSettings_CORSMaxAge(t *testing.T) {
	tests := []struct {
		age   int
		valid bool
	}{
		{0, true},
		{86400, true},
		{-1, false},
		{86401, false},
		{999999, false},
	}
	for _, tc := range tests {
		t.Run(fmt.Sprintf("age=%d", tc.age), func(t *testing.T) {
			s := defaultSettings()
			s.CORSMaxAge = tc.age
			err := validateSettings(&s)
			if tc.valid && err != nil {
				t.Fatalf("expected ok, got: %v", err)
			}
			if !tc.valid && err == nil {
				t.Fatal("expected error, got nil")
			}
		})
	}
}

func TestValidateSettings_URLs(t *testing.T) {
	tests := []struct {
		field string
		value string
		valid bool
	}{
		{"frontendUrl", "javascript:alert(1)", false},
		{"frontendUrl", "file:///etc/passwd", false},
		{"frontendUrl", "data:text/plain,hello", false},
		{"frontendUrl", "http://example.com", true},
		{"frontendUrl", "https://example.com/path?q=1", true},
		{"frontendUrl", "not-a-url", false},
		{"frontendUrl", " http://example.com", false},
		{"frontendUrl", "", true},
		{"livekitHost", "ws://localhost:7880", true},
		{"livekitHost", "wss://lk.example.com", true},
		{"livekitHost", "", true},
		{"googleRedirectUrl", "not-a-url", false},
		{"googleRedirectUrl", "http://localhost:3000/callback", true},
	}
	for _, tc := range tests {
		t.Run(tc.field+"="+tc.value, func(t *testing.T) {
			s := defaultSettings()
			switch tc.field {
			case "frontendUrl":
				s.FrontendURL = tc.value
			case "livekitHost":
				s.LiveKitHost = tc.value
			case "googleRedirectUrl":
				s.GoogleRedirectURL = tc.value
			}
			err := validateSettings(&s)
			if tc.valid && err != nil {
				t.Fatalf("expected ok, got: %v", err)
			}
			if !tc.valid && err == nil {
				t.Fatal("expected error, got nil")
			}
		})
	}
}

func TestValidateSettings_Email(t *testing.T) {
	tests := []struct {
		email string
		valid bool
	}{
		{"admin@example.com", true},
		{"admin@localhost", true},
		{"user+tag@example.co.uk", true},
		{"not-email", false},
		{"", true},
	}
	for _, tc := range tests {
		t.Run("email="+tc.email, func(t *testing.T) {
			s := defaultSettings()
			s.ServerEmail = tc.email
			err := validateSettings(&s)
			if tc.valid && err != nil {
				t.Fatalf("expected ok, got: %v", err)
			}
			if !tc.valid && err == nil {
				t.Fatal("expected error, got nil")
			}
		})
	}
}

func TestValidateSettings_CrossField_TLSandACME(t *testing.T) {
	tests := []struct {
		name  string
		tls   bool
		acme  bool
		cert  string
		key   string
		email string
		valid bool
	}{
		{"TLS+ACME+email", true, true, "", "", "admin@x.com", true},
		{"TLS+ACME no email", true, true, "", "", "", false},
		{"TLS+!ACME cert missing", true, false, "", "/etc/key.pem", "", false},
		{"TLS+!ACME key missing", true, false, "/etc/cert.pem", "", "", false},
		{"TLS+!ACME both present", true, false, "/etc/cert.pem", "/etc/key.pem", "", true},
		{"ACME without TLS", false, true, "", "", "admin@x.com", false},
		{"no TLS no ACME", false, false, "", "", "", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := defaultSettings()
			s.ServerEnableTLS = tc.tls
			s.ServerUseACME = tc.acme
			s.ServerCertFile = tc.cert
			s.ServerKeyFile = tc.key
			s.ServerEmail = tc.email
			err := validateSettings(&s)
			if tc.valid && err != nil {
				t.Fatalf("expected ok, got: %v", err)
			}
			if !tc.valid && err == nil {
				t.Fatal("expected error, got nil")
			}
		})
	}
}

func TestValidateSettings_CrossField_LiveKitExternal(t *testing.T) {
	tests := []struct {
		external bool
		key      string
		secret   string
		valid    bool
	}{
		{true, "key", "secret", true},
		{true, "", "secret", false},
		{true, "key", "", false},
		{true, "", "", false},
		{false, "", "", true},
	}
	for _, tc := range tests {
		t.Run(fmt.Sprintf("external=%t", tc.external), func(t *testing.T) {
			s := defaultSettings()
			s.LiveKitExternal = tc.external
			s.LiveKitAPIKey = tc.key
			s.LiveKitAPISecret = tc.secret
			err := validateSettings(&s)
			if tc.valid && err != nil {
				t.Fatalf("expected ok, got: %v", err)
			}
			if !tc.valid && err == nil {
				t.Fatal("expected error, got nil")
			}
		})
	}
}

func TestValidateSettings_JWTSecret(t *testing.T) {
	tests := []struct {
		secret string
		valid  bool
	}{
		{"", true},
		{"abcd1234abcd1234abcd1234abcd1234", true},  // exactly 32
		{"abcd1234abcd1234abcd1234abcd12345", true}, // 33
		{"short", false},
	}
	for _, tc := range tests {
		t.Run(fmt.Sprintf("len=%d", len(tc.secret)), func(t *testing.T) {
			s := defaultSettings()
			s.JWTSecret = tc.secret
			err := validateSettings(&s)
			if tc.valid && err != nil {
				t.Fatalf("expected ok, got: %v", err)
			}
			if !tc.valid && err == nil {
				t.Fatal("expected error, got nil")
			}
		})
	}
}

func TestValidateSettings_TokenDuration(t *testing.T) {
	tests := []struct {
		dur   int
		valid bool
	}{
		{0, true},
		{1, true},
		{24, true},
		{8760, true},
		{-1, false},
		{8761, false},
	}
	for _, tc := range tests {
		t.Run("dur=", func(t *testing.T) {
			s := defaultSettings()
			s.TokenDuration = tc.dur
			err := validateSettings(&s)
			if tc.valid && err != nil {
				t.Fatalf("expected ok, got: %v", err)
			}
			if !tc.valid && err == nil {
				t.Fatal("expected error, got nil")
			}
		})
	}
}

func TestValidateSettings_LogLevel(t *testing.T) {
	tests := []struct {
		level string
		valid bool
	}{
		{"", true},
		{"debug", true},
		{"info", true},
		{"warn", true},
		{"error", true},
		{"trace", true},
		{"TRACE", false},
		{"info ", false},
		{"verbose", false},
	}
	for _, tc := range tests {
		t.Run("level="+tc.level, func(t *testing.T) {
			s := defaultSettings()
			s.LogLevel = tc.level
			err := validateSettings(&s)
			if tc.valid && err != nil {
				t.Fatalf("expected ok, got: %v", err)
			}
			if !tc.valid && err == nil {
				t.Fatal("expected error, got nil")
			}
		})
	}
}

func TestValidateSettings_RoomLimits(t *testing.T) {
	tests := []struct {
		maxParticipants int
		maxRooms        int
		valid           bool
	}{
		{0, 0, true},
		{1000, 100, true},
		{100000, 100000, true},
		{-1, 0, false},
		{100001, 0, false},
		{0, -1, false},
		{0, 100001, false},
	}
	for _, tc := range tests {
		t.Run("limits", func(t *testing.T) {
			s := defaultSettings()
			s.MaxParticipantsLimit = tc.maxParticipants
			s.MaxRoomsPerUser = tc.maxRooms
			err := validateSettings(&s)
			if tc.valid && err != nil {
				t.Fatalf("expected ok, got: %v", err)
			}
			if !tc.valid && err == nil {
				t.Fatal("expected error, got nil")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// applySettingsFields tests
// ---------------------------------------------------------------------------

func TestApplySettingsFields_TypeMismatch(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		wantErr string
	}{
		{"string instead of bool", `{"serverEnableTls": "yes"}`, "expected a boolean"},
		{"array instead of string", `{"serverHost": ["a"]}`, "expected a string"},
		{"object instead of int", `{"corsMaxAge": {"x": 1}}`, "expected an integer"},
		{"string instead of int", `{"corsMaxAge": "abc"}`, "expected an integer"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var raw map[string]json.RawMessage
			if err := json.Unmarshal([]byte(tc.json), &raw); err != nil {
				t.Fatal(err)
			}
			existing := defaultSettings()
			err := applySettingsFields(&existing, raw)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error %q does not contain %q", err.Error(), tc.wantErr)
			}
		})
	}
}

func TestApplySettingsFields_SecretsMasked(t *testing.T) {
	original := defaultSettings()
	original.LiveKitAPISecret = "original-secret"

	raw := map[string]json.RawMessage{
		"livekitApiSecret": json.RawMessage(`"••••••••"`),
	}
	if err := applySettingsFields(&original, raw); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if original.LiveKitAPISecret != "original-secret" {
		t.Fatalf("expected secret preserved as 'original-secret', got %q", original.LiveKitAPISecret)
	}
}

func TestApplySettingsFields_NewValueReplacesMasked(t *testing.T) {
	original := defaultSettings()
	original.LiveKitAPISecret = "old-secret"

	raw := map[string]json.RawMessage{
		"livekitApiSecret": json.RawMessage(`"new-secret"`),
	}
	if err := applySettingsFields(&original, raw); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if original.LiveKitAPISecret != "new-secret" {
		t.Fatalf("expected 'new-secret', got %q", original.LiveKitAPISecret)
	}
}

// ---------------------------------------------------------------------------
// ValidateSettingsConnectivity endpoint tests
// ---------------------------------------------------------------------------

func TestAdminHandler_ValidateSettingsConnectivity_EmptyBody(t *testing.T) {
	app, _, _ := setupAdminTestApp(t)

	body, _ := json.Marshal(map[string]interface{}{})
	req := httptest.NewRequest(http.MethodPost, "/admin/settings/validate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	respBody, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(respBody, &result); err != nil {
		t.Fatal(err)
	}
	checks, ok := result["checks"].(map[string]interface{})
	if !ok {
		t.Fatal("expected 'checks' object in response")
	}
	if len(checks) > 0 {
		t.Fatalf("expected empty checks, got %d keys", len(checks))
	}
}

func TestAdminHandler_ValidateSettingsConnectivity_InvalidBody(t *testing.T) {
	app, _, _ := setupAdminTestApp(t)

	req := httptest.NewRequest(http.MethodPost, "/admin/settings/validate", bytes.NewReader([]byte("{invalid")))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

// -- Admin Overview (GET /admin/overview) --

func setupAdminOverviewTestApp(t *testing.T) *fiber.App {
	t.Helper()
	db := testutil.SetupTestDB(t)
	roomRepo := repository.NewRoomRepository(db)
	userRepo := repository.NewUserRepository(db)
	settingsRepo := repository.NewSettingsRepository(db)
	handler := NewAdminOverviewHandler(roomRepo, userRepo, settingsRepo, &config.LiveKitConfig{}, nil, db, time.Now(), "test")

	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user", &auth.Claims{
			UserID:   "admin-user-id",
			Email:    "admin@example.com",
			Name:     "Admin",
			Accesses: []string{"superadmin"},
		})
		return c.Next()
	})
	app.Get("/admin/overview", handler.GetOverview)
	return app
}

func TestAdminOverviewHandler_GetOverview_Empty(t *testing.T) {
	app := setupAdminOverviewTestApp(t)

	req := httptest.NewRequest(http.MethodGet, "/admin/overview", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(body))
	}

	var result models.OverviewResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if result.Health.Status != "healthy" {
		t.Fatalf("expected healthy, got %s", result.Health.Status)
	}
	if result.Health.DBStatus != "connected" {
		t.Fatalf("expected connected, got %s", result.Health.DBStatus)
	}
	if result.KPIs.TotalUsers.Value != 0 {
		t.Fatalf("expected 0 total users, got %d", result.KPIs.TotalUsers.Value)
	}
	if result.KPIs.TotalRooms.Value != 0 {
		t.Fatalf("expected 0 total rooms, got %d", result.KPIs.TotalRooms.Value)
	}
	if result.RoomComposition.Live != 0 || result.RoomComposition.Stale != 0 {
		t.Fatal("expected zero composition values")
	}
	if len(result.ActivityTrend) != 7 {
		t.Fatalf("expected 7 activity trend days, got %d", len(result.ActivityTrend))
	}
	if result.NeedsAttention == nil {
		t.Fatal("expected non-nil needsAttention")
	}
	if result.RecentSignups == nil {
		t.Fatal("expected non-nil recentSignups")
	}
	if result.RecentEvents == nil {
		t.Fatal("expected non-nil recentRoomEvents")
	}
}

func TestAdminOverviewHandler_GetOverview_WithData(t *testing.T) {
	db := testutil.SetupTestDB(t)
	roomRepo := repository.NewRoomRepository(db)
	userRepo := repository.NewUserRepository(db)
	settingsRepo := repository.NewSettingsRepository(db)
	handler := NewAdminOverviewHandler(roomRepo, userRepo, settingsRepo, &config.LiveKitConfig{}, nil, db, time.Now().Add(-1*time.Hour), "test")

	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user", &auth.Claims{
			UserID:   "admin-user-id",
			Email:    "admin@example.com",
			Name:     "Admin",
			Accesses: []string{"superadmin"},
		})
		return c.Next()
	})
	app.Get("/admin/overview", handler.GetOverview)

	// Seed data
	db.Create(&models.User{ID: "ov-u1", Email: "ov1@ex.com", Name: "Ov1", Provider: "local", IsActive: true})
	db.Create(&models.User{ID: "ov-u2", Email: "ov2@ex.com", Name: "Ov2", Provider: "github", IsActive: true})
	db.Create(&models.User{ID: "ov-u3", Email: "ov3@ex.com", Name: "Ov3", Provider: "local", IsActive: true})

	room1, _ := roomRepo.CreateRoom("ov-u1", "ov-room-1", true, "standard", 0, &models.RoomSettings{})
	roomRepo.CreateRoom("ov-u2", "ov-room-2", false, "standard", 0, &models.RoomSettings{})

	roomRepo.AddParticipant(room1.ID, "ov-u1")
	roomRepo.AddParticipant(room1.ID, "ov-u2")

	req := httptest.NewRequest(http.MethodGet, "/admin/overview", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(body))
	}

	var result models.OverviewResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if result.KPIs.TotalUsers.Value != 3 {
		t.Fatalf("expected 3 total users, got %d", result.KPIs.TotalUsers.Value)
	}
	if result.KPIs.TotalRooms.Value != 2 {
		t.Fatalf("expected 2 total rooms, got %d", result.KPIs.TotalRooms.Value)
	}
	if result.KPIs.OnlineNow.Value < 1 {
		t.Fatalf("expected at least 1 online, got %d", result.KPIs.OnlineNow.Value)
	}
	if result.RoomComposition.Public < 1 || result.RoomComposition.Private < 1 {
		t.Fatal("expected both public and private room counts > 0")
	}
	if result.RoomComposition.Persistent != 0 {
		t.Fatalf("expected 0 persistent rooms, got %d", result.RoomComposition.Persistent)
	}
	if len(result.RecentSignups) != 3 {
		t.Fatalf("expected 3 recent signups, got %d", len(result.RecentSignups))
	}
	if len(result.ActivityTrend) > 0 && result.ActivityTrend[0].RoomsActive < 0 {
		t.Fatal("expected non-negative roomsActive in trend")
	}
	if len(result.RecentEvents) < 3 {
		t.Fatalf("expected at least 3 recent events, got %d", len(result.RecentEvents))
	}
	if result.KPIs.PendingActions.Value != 0 {
		t.Fatalf("expected 0 pending actions, got %d", result.KPIs.PendingActions.Value)
	}
	if result.InstanceInfo.Version != "test" {
		t.Fatalf("expected version 'test', got '%s'", result.InstanceInfo.Version)
	}
	if result.InstanceInfo.Name != "bedrud" {
		t.Fatalf("expected name 'bedrud', got '%s'", result.InstanceInfo.Name)
	}
	if result.InstanceInfo.UptimeSeconds <= 0 {
		t.Fatal("expected positive uptime")
	}
}

// ---------------------------------------------------------------------------
// Email branding validation tests
// ---------------------------------------------------------------------------

func TestValidateSettings_EmailHeaderBg_ValidHex(t *testing.T) {
	tests := []struct {
		color string
		valid bool
	}{
		{"#ff0000", true},
		{"#1a1a2e", true},
		{"#FFFFFF", true},
		{"#abc123", true},
		{"", true},
		{"#fff", false},
		{"#gggggg", false},
		{"not-a-color", false},
		{"#12345", false},
		{" #ff0000", false},
	}
	for _, tc := range tests {
		t.Run("color="+tc.color, func(t *testing.T) {
			s := defaultSettings()
			s.EmailHeaderBg = tc.color
			err := validateSettings(&s)
			if tc.valid && err != nil {
				t.Fatalf("expected ok, got: %v", err)
			}
			if !tc.valid && err == nil {
				t.Fatal("expected error, got nil")
			}
		})
	}
}

func TestValidateSettings_EmailButtonBg_ValidHex(t *testing.T) {
	s := defaultSettings()
	s.EmailButtonBg = "nothex"
	err := validateSettings(&s)
	if err == nil {
		t.Fatal("expected error for invalid hex color")
	}
}

func TestValidateSettings_EmailButtonBg_EmptyIsValid(t *testing.T) {
	s := defaultSettings()
	s.EmailButtonBg = ""
	err := validateSettings(&s)
	if err != nil {
		t.Fatalf("expected ok for empty color, got: %v", err)
	}
}

func TestValidateSettings_EmailSupportEmail_Valid(t *testing.T) {
	s := defaultSettings()
	s.EmailSupportEmail = "admin@example.com"
	err := validateSettings(&s)
	if err != nil {
		t.Fatalf("expected ok for valid email, got: %v", err)
	}
}

func TestValidateSettings_EmailSupportEmail_Invalid(t *testing.T) {
	s := defaultSettings()
	s.EmailSupportEmail = "not-an-email"
	err := validateSettings(&s)
	if err == nil {
		t.Fatal("expected error for invalid support email")
	}
}

func TestValidateSettings_EmailInstanceURL_Valid(t *testing.T) {
	s := defaultSettings()
	s.EmailInstanceURL = "https://bedrud.example.com"
	err := validateSettings(&s)
	if err != nil {
		t.Fatalf("expected ok for valid URL, got: %v", err)
	}
}

func TestValidateSettings_EmailInstanceURL_Invalid(t *testing.T) {
	tests := []string{
		"not-a-url",
		"javascript:alert(1)",
		"ftp://example.com",
	}
	for _, u := range tests {
		t.Run("url="+u, func(t *testing.T) {
			s := defaultSettings()
			s.EmailInstanceURL = u
			err := validateSettings(&s)
			if err == nil {
				t.Fatal("expected error for invalid URL")
			}
		})
	}
}

func TestValidateSettings_EmailSMTPPort_Invalid(t *testing.T) {
	s := defaultSettings()
	s.EmailSMTPPort = 99999
	err := validateSettings(&s)
	if err == nil {
		t.Fatal("expected error for invalid SMTP port")
	}
}

func TestValidateSettings_EmailSMTPPort_ZeroIsValid(t *testing.T) {
	s := defaultSettings()
	s.EmailSMTPPort = 0
	err := validateSettings(&s)
	if err != nil {
		t.Fatalf("expected ok for zero port, got: %v", err)
	}
}

func TestValidateSettings_EmailSMTPPort_Valid(t *testing.T) {
	s := defaultSettings()
	s.EmailSMTPPort = 587
	err := validateSettings(&s)
	if err != nil {
		t.Fatalf("expected ok for port 587, got: %v", err)
	}
}

func TestValidateSettings_EmailBranding_AllValid(t *testing.T) {
	s := defaultSettings()
	s.EmailHeaderBg = "#ff0000"
	s.EmailButtonBg = "#00ff00"
	s.EmailSupportEmail = "support@example.com"
	s.EmailInstanceURL = "https://bedrud.example.com"
	s.EmailSMTPPort = 465
	err := validateSettings(&s)
	if err != nil {
		t.Fatalf("expected ok for all valid email settings, got: %v", err)
	}
}

func TestValidateSettings_EmailPassword_MinLength(t *testing.T) {
	tests := []struct {
		password string
		valid    bool
	}{
		{"", true},
		{"abcd", true},
		{"abc", false},
		{"a", false},
	}
	for _, tc := range tests {
		t.Run(fmt.Sprintf("len=%d", len(tc.password)), func(t *testing.T) {
			s := defaultSettings()
			s.EmailPassword = tc.password
			err := validateSettings(&s)
			if tc.valid && err != nil {
				t.Fatalf("expected ok, got: %v", err)
			}
			if !tc.valid && err == nil {
				t.Fatal("expected error, got nil")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// SendTestEmail endpoint tests
// ---------------------------------------------------------------------------

func TestAdminHandler_SendTestEmail_InvalidEmail(t *testing.T) {
	app, _, _ := setupAdminTestApp(t)
	body, _ := json.Marshal(map[string]interface{}{"to": "not-an-email"})
	req := httptest.NewRequest(http.MethodPost, "/admin/settings/send-test-email",
		bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestAdminHandler_SendTestEmail_MissingTo(t *testing.T) {
	app, _, _ := setupAdminTestApp(t)
	body, _ := json.Marshal(map[string]interface{}{})
	req := httptest.NewRequest(http.MethodPost, "/admin/settings/send-test-email",
		bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestAdminHandler_SendTestEmail_SMTPNotConfigured(t *testing.T) {
	app, _, _ := setupAdminTestApp(t)
	body, _ := json.Marshal(map[string]interface{}{"to": "test@example.com"})
	req := httptest.NewRequest(http.MethodPost, "/admin/settings/send-test-email",
		bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	// Should return 400 with "SMTP not configured" error
	if resp.StatusCode != http.StatusBadRequest {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 400, got %d: %s", resp.StatusCode, string(respBody))
	}
}

// ---------------------------------------------------------------------------
// Webhook CRUD Tests
// ---------------------------------------------------------------------------

func TestWebhook_List_Empty(t *testing.T) {
	app, _, _ := setupAdminTestApp(t)

	req := httptest.NewRequest(http.MethodGet, "/admin/webhooks", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	webhooks, _ := result["webhooks"].([]interface{})
	if len(webhooks) != 0 {
		t.Fatalf("expected empty list, got %d", len(webhooks))
	}
	total, _ := result["total"].(float64)
	if total != 0 {
		t.Fatalf("expected total 0, got %f", total)
	}
}

func TestWebhook_Create_Success(t *testing.T) {
	app, _, _ := setupAdminTestApp(t)

	body, _ := json.Marshal(map[string]interface{}{
		"name":   "Test Webhook",
		"url":    "https://example.com/webhook",
		"events": []string{"room.created"},
	})
	req := httptest.NewRequest(http.MethodPost, "/admin/webhooks", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 201, got %d: %s", resp.StatusCode, string(respBody))
	}

	var result map[string]interface{}
	respBody, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(respBody, &result); err != nil {
		t.Fatal(err)
	}
	if result["id"] == nil || result["id"] == "" {
		t.Fatal("expected 'id' in response")
	}
	if result["secret"] == nil || result["secret"] == "" {
		t.Fatal("expected 'secret' in create response")
	}
	if result["name"] != "Test Webhook" {
		t.Fatalf("expected name 'Test Webhook', got %v", result["name"])
	}
}

func TestWebhook_Create_MissingName(t *testing.T) {
	app, _, _ := setupAdminTestApp(t)

	body, _ := json.Marshal(map[string]interface{}{
		"url":    "https://example.com/webhook",
		"events": []string{"room.created"},
	})
	req := httptest.NewRequest(http.MethodPost, "/admin/webhooks", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestWebhook_Create_MissingURL(t *testing.T) {
	app, _, _ := setupAdminTestApp(t)

	body, _ := json.Marshal(map[string]interface{}{
		"name":   "No URL",
		"events": []string{"room.created"},
	})
	req := httptest.NewRequest(http.MethodPost, "/admin/webhooks", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestWebhook_Create_InvalidURL(t *testing.T) {
	app, _, _ := setupAdminTestApp(t)

	body, _ := json.Marshal(map[string]interface{}{
		"name":   "Bad URL",
		"url":    "not-a-url",
		"events": []string{"room.created"},
	})
	req := httptest.NewRequest(http.MethodPost, "/admin/webhooks", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestWebhook_Create_InvalidEvents(t *testing.T) {
	app, _, _ := setupAdminTestApp(t)

	body, _ := json.Marshal(map[string]interface{}{
		"name":   "Bad Events",
		"url":    "https://example.com/hook",
		"events": []string{"invalid.event.name"},
	})
	req := httptest.NewRequest(http.MethodPost, "/admin/webhooks", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestWebhook_List_AfterCreate(t *testing.T) {
	app, _, _ := setupAdminTestApp(t)

	// Create one webhook
	createBody, _ := json.Marshal(map[string]interface{}{
		"name":   "List Test",
		"url":    "https://example.com/list-test",
		"events": []string{"room.created"},
	})
	createReq := httptest.NewRequest(http.MethodPost, "/admin/webhooks", bytes.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createResp, err := app.Test(createReq, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = createResp.Body.Close() }()
	if createResp.StatusCode != http.StatusCreated {
		t.Fatalf("failed to create webhook: %d", createResp.StatusCode)
	}

	// List should return 1
	req := httptest.NewRequest(http.MethodGet, "/admin/webhooks", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	webhooks, _ := result["webhooks"].([]interface{})
	if len(webhooks) != 1 {
		t.Fatalf("expected 1 webhook, got %d", len(webhooks))
	}
	// Secret should be masked in list response
	first := webhooks[0].(map[string]interface{})
	if secret, ok := first["secret"].(string); ok && secret != "" && len(secret) > 8 && !strings.Contains(secret, "••••") {
		t.Fatal("expected masked secret in list response, got plaintext")
	}
}

func TestWebhook_Update_Success(t *testing.T) {
	app, _, _ := setupAdminTestApp(t)

	// Create
	createBody, _ := json.Marshal(map[string]interface{}{
		"name":   "Original",
		"url":    "https://example.com/original",
		"events": []string{"room.created"},
	})
	createReq := httptest.NewRequest(http.MethodPost, "/admin/webhooks", bytes.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createResp, err := app.Test(createReq, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = createResp.Body.Close() }()
	var created map[string]interface{}
	if err := json.NewDecoder(createResp.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}
	id := created["id"].(string)

	// Update name and events
	updateBody, _ := json.Marshal(map[string]interface{}{
		"name":   "Updated",
		"events": []string{"room.created", "room.ended"},
	})
	updateReq := httptest.NewRequest(http.MethodPut, "/admin/webhooks/"+id, bytes.NewReader(updateBody))
	updateReq.Header.Set("Content-Type", "application/json")
	updateResp, err := app.Test(updateReq, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = updateResp.Body.Close() }()
	if updateResp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(updateResp.Body)
		t.Fatalf("expected 200, got %d: %s", updateResp.StatusCode, string(respBody))
	}

	var updated map[string]interface{}
	if err := json.NewDecoder(updateResp.Body).Decode(&updated); err != nil {
		t.Fatal(err)
	}
	if updated["name"] != "Updated" {
		t.Fatalf("expected name 'Updated', got %v", updated["name"])
	}
}

func TestWebhook_Update_NotFound(t *testing.T) {
	app, _, _ := setupAdminTestApp(t)

	body, _ := json.Marshal(map[string]interface{}{"name": "Nope"})
	req := httptest.NewRequest(http.MethodPut, "/admin/webhooks/nonexistent", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestWebhook_Delete_Success(t *testing.T) {
	app, _, _ := setupAdminTestApp(t)

	// Create
	createBody, _ := json.Marshal(map[string]interface{}{
		"name":   "Delete Me",
		"url":    "https://example.com/delete-me",
		"events": []string{"room.created"},
	})
	createReq := httptest.NewRequest(http.MethodPost, "/admin/webhooks", bytes.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createResp, err := app.Test(createReq, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = createResp.Body.Close() }()
	var created map[string]interface{}
	if err := json.NewDecoder(createResp.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}
	id := created["id"].(string)

	// Delete
	delReq := httptest.NewRequest(http.MethodDelete, "/admin/webhooks/"+id, http.NoBody)
	delResp, err := app.Test(delReq, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = delResp.Body.Close() }()
	if delResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", delResp.StatusCode)
	}

	// Verify list is empty
	listReq := httptest.NewRequest(http.MethodGet, "/admin/webhooks", http.NoBody)
	listResp, err := app.Test(listReq, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = listResp.Body.Close() }()
	body, _ := io.ReadAll(listResp.Body)
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatal(err)
	}
	webhooks, _ := result["webhooks"].([]interface{})
	if len(webhooks) != 0 {
		t.Fatalf("expected empty after delete, got %d", len(webhooks))
	}
}

func TestWebhook_Delete_NotFound(t *testing.T) {
	app, _, _ := setupAdminTestApp(t)

	req := httptest.NewRequest(http.MethodDelete, "/admin/webhooks/nonexistent", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestWebhook_RotateSecret(t *testing.T) {
	app, _, _ := setupAdminTestApp(t)

	// Create
	createBody, _ := json.Marshal(map[string]interface{}{
		"name":   "Rotate Test",
		"url":    "https://example.com/rotate-test",
		"events": []string{"room.created"},
	})
	createReq := httptest.NewRequest(http.MethodPost, "/admin/webhooks", bytes.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createResp, err := app.Test(createReq, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = createResp.Body.Close() }()
	var created map[string]interface{}
	if err := json.NewDecoder(createResp.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}
	id := created["id"].(string)
	oldSecret := created["secret"].(string)

	// Rotate
	rotReq := httptest.NewRequest(http.MethodPost, "/admin/webhooks/"+id+"/rotate-secret", http.NoBody)
	rotResp, err := app.Test(rotReq, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = rotResp.Body.Close() }()
	if rotResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", rotResp.StatusCode)
	}

	var rotated map[string]interface{}
	if err := json.NewDecoder(rotResp.Body).Decode(&rotated); err != nil {
		t.Fatal(err)
	}
	newSecret, _ := rotated["secret"].(string)
	if newSecret == "" {
		t.Fatal("expected new secret in response")
	}
	if newSecret == oldSecret {
		t.Fatal("new secret should differ from old secret")
	}
}

func TestWebhook_RotateSecret_NotFound(t *testing.T) {
	app, _, _ := setupAdminTestApp(t)

	req := httptest.NewRequest(http.MethodPost, "/admin/webhooks/nonexistent/rotate-secret", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestWebhook_Test_NotFound(t *testing.T) {
	app, _, _ := setupAdminTestApp(t)

	req := httptest.NewRequest(http.MethodPost, "/admin/webhooks/nonexistent/test", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestWebhook_Test_Success(t *testing.T) {
	app, _, _ := setupAdminTestApp(t)

	// Create webhook pointing at a test server that accepts any request
	// Use httptest.NewServer as target
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	createBody, _ := json.Marshal(map[string]interface{}{
		"name":   "Test Sender",
		"url":    ts.URL,
		"events": []string{"room.created"},
	})
	createReq := httptest.NewRequest(http.MethodPost, "/admin/webhooks", bytes.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createResp, err := app.Test(createReq, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = createResp.Body.Close() }()
	var created map[string]interface{}
	if err := json.NewDecoder(createResp.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}
	id := created["id"].(string)

	// Send test
	testReq := httptest.NewRequest(http.MethodPost, "/admin/webhooks/"+id+"/test", http.NoBody)
	testResp, err := app.Test(testReq, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = testResp.Body.Close() }()
	if testResp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(testResp.Body)
		t.Fatalf("expected 200, got %d: %s", testResp.StatusCode, string(respBody))
	}
}

// -------------------------------------------------------------------------
// Pagination Edge Case Tests
// -------------------------------------------------------------------------

func TestListInviteTokens_Pagination_Defaults(t *testing.T) {
	app, _, inviteTokenRepo := setupAdminTestApp(t)

	// Seed tokens
	for i := range 3 {
		inviteTokenRepo.Create(&models.InviteToken{
			ID:        "tok-" + fmt.Sprint(i),
			Token:     "seed-" + fmt.Sprint(i),
			CreatedBy: "admin",
			ExpiresAt: time.Now().Add(24 * time.Hour),
		})
	}

	t.Run("page=0 defaults to 1", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/invite-tokens?page=0", http.NoBody)
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatalf("app.Test failed: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		var body map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		tokens, _ := body["tokens"].([]any)
		if len(tokens) != 3 {
			t.Fatalf("expected 3 tokens with page=0, got %d", len(tokens))
		}
		total, _ := body["total"].(float64)
		if total != 3 {
			t.Fatalf("expected total 3, got %f", total)
		}
	})

	t.Run("limit=0 defaults to 50", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/invite-tokens?limit=0", http.NoBody)
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatalf("app.Test failed: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		var body map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		tokens, _ := body["tokens"].([]any)
		if len(tokens) != 3 {
			t.Fatalf("expected 3 tokens with limit=0, got %d", len(tokens))
		}
	})

	t.Run("limit=-1 defaults to 50", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/invite-tokens?limit=-1", http.NoBody)
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatalf("app.Test failed: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		var body map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		total, _ := body["total"].(float64)
		if total != 3 {
			t.Fatalf("expected total 3, got %f", total)
		}
	})

	t.Run("page > total returns empty", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/invite-tokens?page=999", http.NoBody)
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatalf("app.Test failed: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		var body map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		tokens, _ := body["tokens"].([]any)
		if len(tokens) != 0 {
			t.Fatalf("expected empty list for page beyond total, got %d", len(tokens))
		}
	})
}

func TestListWebhooks_Pagination(t *testing.T) {
	app, _, _ := setupAdminTestApp(t)

	// Seed webhooks via POST endpoint
	for i := range 3 {
		body, _ := json.Marshal(map[string]interface{}{
			"name":   "WH-" + fmt.Sprint(i),
			"url":    "https://example.com/" + fmt.Sprint(i),
			"events": []string{"room.created"},
		})
		req := httptest.NewRequest(http.MethodPost, "/admin/webhooks", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := app.Test(req, -1)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("failed to seed webhook %d: %d", i, resp.StatusCode)
		}
	}

	t.Run("response has pagination fields", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/webhooks", http.NoBody)
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatalf("app.Test failed: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		var body map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if _, ok := body["webhooks"]; !ok {
			t.Fatal("expected webhooks key in response")
		}
		if _, ok := body["total"]; !ok {
			t.Fatal("expected total key in response")
		}
		if _, ok := body["page"]; !ok {
			t.Fatal("expected page key in response")
		}
		if _, ok := body["limit"]; !ok {
			t.Fatal("expected limit key in response")
		}
	})

	t.Run("page=0 defaults to 1", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/webhooks?page=0", http.NoBody)
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatalf("app.Test failed: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()
		var body map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		webhooks, _ := body["webhooks"].([]any)
		if len(webhooks) != 3 {
			t.Fatalf("expected 3 webhooks with page=0, got %d", len(webhooks))
		}
	})

	t.Run("limit=0 defaults to 50", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/webhooks?limit=0", http.NoBody)
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatalf("app.Test failed: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()
		var body map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		webhooks, _ := body["webhooks"].([]any)
		if len(webhooks) != 3 {
			t.Fatalf("expected 3 webhooks with limit=0, got %d", len(webhooks))
		}
	})
}
