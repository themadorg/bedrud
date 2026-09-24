package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"bedrud/config"
	"bedrud/internal/auth"
	"bedrud/internal/models"
	"bedrud/internal/repository"
	"bedrud/internal/testutil"

	"github.com/gofiber/fiber/v2"
)

// resetPasswordFixture mounts the admin reset endpoint and the public reset endpoint on one
// app. The link mode is only worth anything if the URL it hands back can actually be redeemed,
// so these tests drive both halves rather than trusting the token's shape.
type resetPasswordFixture struct {
	app      *fiber.App
	userRepo *repository.UserRepository
	authSvc  *auth.AuthService
	claims   *auth.Claims
}

const resetFixturePassword = "originalPass123!"

func setupAdminResetPasswordApp(t *testing.T) *resetPasswordFixture {
	t.Helper()
	db := testutil.SetupTestDB(t)
	userRepo := repository.NewUserRepository(db)
	passkeyRepo := repository.NewPasskeyRepository(db)
	authSvc := auth.NewAuthService(userRepo, passkeyRepo)
	cfg := &config.Config{
		Auth: config.AuthConfig{
			JWTSecret:          "admin-reset-password-test-secret1",
			TokenDuration:      1,
			SessionSecret:      "session-secret-for-testing",
			ResetTokenTTLHours: 1,
		},
		Server: config.ServerConfig{Domain: "localhost"},
	}
	config.SetForTest(cfg)

	usersHandler := NewUsersHandler(
		userRepo,
		repository.NewRoomRepository(db),
		passkeyRepo,
		repository.NewUserPreferencesRepository(db),
		nil,
		nil,
	)
	authHandler := NewAuthHandler(
		authSvc, cfg,
		repository.NewSettingsRepository(db),
		repository.NewInviteTokenRepository(db),
		nil, NewCooldownCache(2*time.Minute), nil,
	)

	claims := &auth.Claims{UserID: "reset-admin", Email: "admin@ex.com", Accesses: []string{"user", "superadmin"}}
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user", claims)
		return c.Next()
	})
	app.Post("/admin/users/:id/reset-password", usersHandler.AdminResetPassword)
	app.Post("/api/auth/reset-password", authHandler.ResetPassword)

	hash, err := auth.HashPassword(resetFixturePassword)
	if err != nil {
		t.Fatal(err)
	}
	seed := []*models.User{
		{ID: "reset-admin", Email: "admin@ex.com", Name: "Admin", Provider: models.ProviderLocal, Password: hash, IsActive: true, Accesses: models.StringArray{"user", "superadmin"}},
		{ID: "reset-local", Email: "local@ex.com", Name: "Local", Provider: models.ProviderLocal, Password: hash, IsActive: true, Accesses: models.StringArray{"user"}},
		{ID: "reset-oauth", Email: "oauth@ex.com", Name: "OAuth", Provider: "google", IsActive: true, Accesses: models.StringArray{"user"}},
	}
	for _, u := range seed {
		if err := userRepo.CreateUser(u); err != nil {
			t.Fatalf("seed %s: %v", u.ID, err)
		}
		// The ban set is process-global, so leave it as it was found.
		userID := u.ID
		auth.UnbanUser(userID)
		t.Cleanup(func() { auth.UnbanUser(userID) })
	}

	return &resetPasswordFixture{app: app, userRepo: userRepo, authSvc: authSvc, claims: claims}
}

// reset posts a mode to the admin endpoint and returns the status with the decoded body.
func (f *resetPasswordFixture) reset(t *testing.T, userID, mode string) (int, map[string]any) {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"mode": mode})
	req := httptest.NewRequest(http.MethodPost, "/admin/users/"+userID+"/reset-password", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := f.app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var out map[string]any
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &out); err != nil {
			t.Fatalf("decode %s: %v", raw, err)
		}
	}
	return resp.StatusCode, out
}

func TestAdminResetPassword_GeneratedPasswordSignsTheUserIn(t *testing.T) {
	f := setupAdminResetPasswordApp(t)

	status, out := f.reset(t, "reset-local", "password")
	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d: %v", status, out)
	}

	password, ok := out["password"].(string)
	if !ok || password == "" {
		t.Fatalf("expected a password in the response, got %v", out)
	}
	if len(password) != generatedPasswordLength {
		t.Fatalf("expected a %d character password, got %d", generatedPasswordLength, len(password))
	}
	// The floor matters more than the exact length: anything shorter is a value the login and
	// reset paths reject, so the admin would be handing over a password that cannot be used.
	if len(password) < MinPasswordLength {
		t.Fatalf("generated password is under the %d character minimum", MinPasswordLength)
	}

	if _, err := f.authSvc.Login("local@ex.com", password); err != nil {
		t.Fatalf("expected the generated password to log in: %v", err)
	}
	if _, err := f.authSvc.Login("local@ex.com", resetFixturePassword); err == nil {
		t.Fatal("expected the previous password to stop working")
	}
}

func TestAdminResetPassword_GeneratedPasswordRevokesSessions(t *testing.T) {
	f := setupAdminResetPasswordApp(t)

	if _, err := f.authSvc.Login("local@ex.com", resetFixturePassword); err != nil {
		t.Fatal(err)
	}
	before, err := f.userRepo.GetUserByID("reset-local")
	if err != nil || before == nil {
		t.Fatalf("seed lookup: %v", err)
	}
	if before.RefreshToken == "" {
		t.Fatal("expected the login to store a refresh token")
	}

	if status, out := f.reset(t, "reset-local", "password"); status != http.StatusOK {
		t.Fatalf("expected 200, got %d: %v", status, out)
	}

	after, err := f.userRepo.GetUserByID("reset-local")
	if err != nil || after == nil {
		t.Fatalf("post-reset lookup: %v", err)
	}
	if after.RefreshToken != "" {
		t.Fatal("expected the refresh token to be cleared")
	}
}

func TestAdminResetPassword_LinkIsRedeemable(t *testing.T) {
	f := setupAdminResetPasswordApp(t)

	status, out := f.reset(t, "reset-local", "link")
	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d: %v", status, out)
	}

	resetURL, ok := out["resetUrl"].(string)
	if !ok || resetURL == "" {
		t.Fatalf("expected a resetUrl in the response, got %v", out)
	}
	const marker = "/auth/reset-password?token="
	idx := strings.Index(resetURL, marker)
	if idx < 0 {
		t.Fatalf("expected the link to point at the reset page, got %q", resetURL)
	}
	if out["expiresAt"] == nil {
		t.Fatalf("expected an expiry alongside the link, got %v", out)
	}

	// Redeem it the way the user would, through the public endpoint.
	token := resetURL[idx+len(marker):]
	body, _ := json.Marshal(map[string]string{"token": token, "newPassword": "chosenByUser456!"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/reset-password", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := f.app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		t.Fatalf("redeeming the link: expected 200, got %d: %s", resp.StatusCode, raw)
	}

	if _, err := f.authSvc.Login("local@ex.com", "chosenByUser456!"); err != nil {
		t.Fatalf("expected the password the user chose to log in: %v", err)
	}
}

func TestAdminResetPassword_LinkLeavesTheCurrentPasswordAlone(t *testing.T) {
	f := setupAdminResetPasswordApp(t)

	if status, out := f.reset(t, "reset-local", "link"); status != http.StatusOK {
		t.Fatalf("expected 200, got %d: %v", status, out)
	}

	// Handing out a link must not lock the user out before they act on it.
	if _, err := f.authSvc.Login("local@ex.com", resetFixturePassword); err != nil {
		t.Fatalf("expected the existing password to still work: %v", err)
	}
}

func TestAdminResetPassword_LinkModeReturnsNoPassword(t *testing.T) {
	f := setupAdminResetPasswordApp(t)

	_, out := f.reset(t, "reset-local", "link")
	if _, present := out["password"]; present {
		t.Fatalf("link mode must not disclose a password, got %v", out)
	}
}

func TestAdminResetPassword_RejectsUnknownMode(t *testing.T) {
	f := setupAdminResetPasswordApp(t)

	for _, mode := range []string{"", "reset", "PASSWORD"} {
		status, _ := f.reset(t, "reset-local", mode)
		if status != http.StatusBadRequest {
			t.Fatalf("mode %q: expected 400, got %d", mode, status)
		}
	}

	// A rejected request must not have touched the account.
	if _, err := f.authSvc.Login("local@ex.com", resetFixturePassword); err != nil {
		t.Fatalf("expected the password to be untouched: %v", err)
	}
}

func TestAdminResetPassword_RejectsAccountWithNoPassword(t *testing.T) {
	f := setupAdminResetPasswordApp(t)

	for _, mode := range []string{"password", "link"} {
		status, out := f.reset(t, "reset-oauth", mode)
		if status != http.StatusBadRequest {
			t.Fatalf("mode %q: expected 400 for an OAuth account, got %d: %v", mode, status, out)
		}
	}
}

func TestAdminResetPassword_UnknownUser(t *testing.T) {
	f := setupAdminResetPasswordApp(t)

	if status, _ := f.reset(t, "no-such-user", "password"); status != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", status)
	}
}

func TestAdminResetPassword_RequiresSuperadmin(t *testing.T) {
	f := setupAdminResetPasswordApp(t)
	f.claims.Accesses = []string{"user", "admin"}

	if status, _ := f.reset(t, "reset-local", "password"); status != http.StatusForbidden {
		t.Fatalf("expected 403 for a plain admin, got %d", status)
	}
	if _, err := f.authSvc.Login("local@ex.com", resetFixturePassword); err != nil {
		t.Fatalf("expected the password to be untouched: %v", err)
	}
}
