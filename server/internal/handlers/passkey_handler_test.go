package handlers

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"bedrud/config"
	"bedrud/internal/auth"
	"bedrud/internal/database"
	"bedrud/internal/models"
	"bedrud/internal/repository"
	"bedrud/internal/testutil"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

func setupPasskeyTestApp(t *testing.T) (*fiber.App, *auth.AuthService, *config.Config) {
	t.Helper()
	db := testutil.SetupTestDB(t)
	userRepo := repository.NewUserRepository(db)
	passkeyRepo := repository.NewPasskeyRepository(db)
	settingsRepo := repository.NewSettingsRepository(db)
	authService := auth.NewAuthService(userRepo, passkeyRepo)
	cfg := &config.Config{
		Auth: config.AuthConfig{
			JWTSecret:     "handler-auth-test-secret-key-32b",
			TokenDuration: 1,
			SessionSecret: "session-secret-for-testing",
		},
		Server: config.ServerConfig{Domain: "localhost"},
	}
	config.SetForTest(cfg)
	auth.InitializeSessionStore(cfg.Auth.SessionSecret, false)
	cs := auth.NewChallengeStore(5)
	h := NewAuthHandler(authService, cfg, settingsRepo, nil, cs, NewCooldownCache(0), nil)

	authMW := func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": "missing"})
		}
		tokenStr := authHeader
		if len(authHeader) > 7 && authHeader[:7] == bearerPrefix {
			tokenStr = authHeader[7:]
		}
		claims, err := auth.ValidateToken(tokenStr, cfg)
		if err != nil {
			return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": "invalid"})
		}
		c.Locals("user", claims)
		return c.Next()
	}

	app := fiber.New()
	app.Get("/api/auth/passkeys", authMW, h.ListPasskeys)
	app.Post("/api/auth/passkey/register/begin", authMW, h.PasskeyRegisterBegin)
	app.Post("/api/auth/passkey/register/finish", authMW, h.PasskeyRegisterFinish)
	app.Post("/api/auth/passkey/login/begin", h.PasskeyLoginBegin)
	app.Post("/api/auth/passkey/login/finish", h.PasskeyLoginFinish)
	app.Post("/api/auth/passkey/signup/begin", h.PasskeySignupBegin)
	app.Post("/api/auth/passkey/signup/finish", h.PasskeySignupFinish)

	if _, err := authService.Register("pk@ex.com", "securepass123", "Passkey User"); err != nil {
		t.Fatal(err)
	}

	return app, authService, cfg
}

func TestPasskeyRegisterBegin_Success(t *testing.T) {
	app, authService, cfg := setupPasskeyTestApp(t)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/passkey/register/begin", http.NoBody)
	req.Header.Set("Authorization", authHeaderFor(t, authService, cfg, "pk@ex.com"))
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status %d: %s", resp.StatusCode, b)
	}
	var result map[string]interface{}
	_ = json.NewDecoder(resp.Body).Decode(&result)
	if result["challenge"] == nil || result["challenge"] == "" {
		t.Fatalf("expected challenge, got %#v", result)
	}
	if result["user"] == nil || result["rp"] == nil {
		t.Fatalf("expected user+rp shape, got %#v", result)
	}
	assertCredParams(t, result)
}

// assertCredParams guards the required WebAuthn members: without pubKeyCredParams
// navigator.credentials.create() throws "Required member is undefined" client-side.
func assertCredParams(t *testing.T, result map[string]interface{}) {
	t.Helper()
	params, ok := result["pubKeyCredParams"].([]interface{})
	if !ok || len(params) == 0 {
		t.Fatalf("expected pubKeyCredParams, got %#v", result["pubKeyCredParams"])
	}
	for _, p := range params {
		entry, ok := p.(map[string]interface{})
		if !ok || entry["type"] != "public-key" || entry["alg"] == nil {
			t.Fatalf("malformed pubKeyCredParams entry: %#v", p)
		}
	}
	sel, ok := result["authenticatorSelection"].(map[string]interface{})
	if !ok || sel["residentKey"] != "required" {
		t.Fatalf("expected discoverable credential request, got %#v", result["authenticatorSelection"])
	}
}

func TestPasskeyRegisterBegin_Unauthenticated(t *testing.T) {
	app, _, _ := setupPasskeyTestApp(t)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/passkey/register/begin", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestPasskeyRegisterFinish_NoChallenge(t *testing.T) {
	app, authService, cfg := setupPasskeyTestApp(t)
	body, _ := json.Marshal(map[string]string{
		"clientDataJSON":    "e30",
		"attestationObject": "e30",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/passkey/register/finish", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", authHeaderFor(t, authService, cfg, "pk@ex.com"))
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestPasskeyLoginBegin_Success(t *testing.T) {
	app, _, _ := setupPasskeyTestApp(t)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/passkey/login/begin", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status %d: %s", resp.StatusCode, b)
	}
	var result map[string]interface{}
	_ = json.NewDecoder(resp.Body).Decode(&result)
	if result["challenge"] == nil || result["challenge"] == "" {
		t.Fatalf("expected challenge, got %#v", result)
	}
}

func TestPasskeyLoginFinish_NoChallenge(t *testing.T) {
	app, _, _ := setupPasskeyTestApp(t)
	body, _ := json.Marshal(map[string]string{
		"credentialId":      "e30",
		"clientDataJSON":    "e30",
		"authenticatorData": "e30",
		"signature":         "e30",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/passkey/login/finish", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestPasskeySignupBegin_Success(t *testing.T) {
	app, _, _ := setupPasskeyTestApp(t)
	body, _ := json.Marshal(map[string]string{
		"email": "newpk@ex.com",
		"name":  "New PK",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/passkey/signup/begin", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status %d: %s", resp.StatusCode, b)
	}
	var result map[string]interface{}
	_ = json.NewDecoder(resp.Body).Decode(&result)
	if result["challenge"] == nil {
		t.Fatalf("expected challenge, got %#v", result)
	}
	assertCredParams(t, result)
}

func TestPasskeySignupBegin_MissingFields(t *testing.T) {
	app, _, _ := setupPasskeyTestApp(t)
	body, _ := json.Marshal(map[string]string{"email": "x@ex.com"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/passkey/signup/begin", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestPasskeySignupFinish_NoChallenge(t *testing.T) {
	app, _, _ := setupPasskeyTestApp(t)
	body, _ := json.Marshal(map[string]string{
		"clientDataJSON":    "e30",
		"attestationObject": "e30",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/passkey/signup/finish", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestListPasskeys_ReturnsOnlyOwnPasskeysWithoutKeyMaterial(t *testing.T) {
	app, authService, cfg := setupPasskeyTestApp(t)
	if _, err := authService.Register("other@ex.com", "securepass123", "Other User"); err != nil {
		t.Fatal(err)
	}
	owner, _ := authService.GetUserByEmail("pk@ex.com")
	other, _ := authService.GetUserByEmail("other@ex.com")
	repo := repository.NewPasskeyRepository(database.GetDB())
	for _, pk := range []*models.Passkey{
		{ID: uuid.New().String(), UserID: owner.ID, CredentialID: []byte("owner-cred"), PublicKey: []byte("owner-key"), Name: "Laptop"},
		{ID: uuid.New().String(), UserID: other.ID, CredentialID: []byte("other-cred"), PublicKey: []byte("other-key"), Name: "Phone"},
	} {
		if err := repo.CreatePasskey(pk); err != nil {
			t.Fatal(err)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/api/auth/passkeys", http.NoBody)
	req.Header.Set("Authorization", authHeaderFor(t, authService, cfg, "pk@ex.com"))
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d: %s", resp.StatusCode, raw)
	}
	var out struct {
		Passkeys []map[string]any `json:"passkeys"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Passkeys) != 1 || out.Passkeys[0]["name"] != "Laptop" {
		t.Fatalf("expected only the caller's passkey, got %s", raw)
	}
	if out.Passkeys[0]["id"] == "" || out.Passkeys[0]["createdAt"] == nil {
		t.Fatalf("expected id and createdAt, got %s", raw)
	}
	for _, field := range []string{"credentialId", "publicKey", "userId", "counter"} {
		if _, ok := out.Passkeys[0][field]; ok {
			t.Fatalf("response exposes %s: %s", field, raw)
		}
	}
}

// The web sign-in flow reads an empty list as "offer to add a passkey", so it must be
// an empty array rather than null.
func TestListPasskeys_NoneRegistered(t *testing.T) {
	app, authService, cfg := setupPasskeyTestApp(t)
	req := httptest.NewRequest(http.MethodGet, "/api/auth/passkeys", http.NoBody)
	req.Header.Set("Authorization", authHeaderFor(t, authService, cfg, "pk@ex.com"))
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK || string(raw) != `{"passkeys":[]}` {
		t.Fatalf(`want 200 {"passkeys":[]}, got %d %s`, resp.StatusCode, raw)
	}
}

func TestListPasskeys_Unauthenticated(t *testing.T) {
	app, _, _ := setupPasskeyTestApp(t)
	req := httptest.NewRequest(http.MethodGet, "/api/auth/passkeys", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

// An authenticator that already holds a passkey for the account must be told so, or it
// replaces its own copy and leaves the server with a record nothing can use.
func TestPasskeyRegisterBegin_ExcludesExistingPasskeys(t *testing.T) {
	app, authService, cfg := setupPasskeyTestApp(t)
	owner, _ := authService.GetUserByEmail("pk@ex.com")
	credID := []byte("existing-credential")
	if err := repository.NewPasskeyRepository(database.GetDB()).CreatePasskey(&models.Passkey{
		ID: uuid.New().String(), UserID: owner.ID, CredentialID: credID, PublicKey: []byte("key"), Name: "Passkey",
	}); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/auth/passkey/register/begin", http.NoBody)
	req.Header.Set("Authorization", authHeaderFor(t, authService, cfg, "pk@ex.com"))
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var result struct {
		ExcludeCredentials []struct {
			Type string `json:"type"`
			ID   string `json:"id"`
		} `json:"excludeCredentials"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	want := base64.RawURLEncoding.EncodeToString(credID)
	if len(result.ExcludeCredentials) != 1 || result.ExcludeCredentials[0].ID != want ||
		result.ExcludeCredentials[0].Type != "public-key" {
		t.Fatalf("expected the existing passkey in excludeCredentials, got %+v", result.ExcludeCredentials)
	}
}
