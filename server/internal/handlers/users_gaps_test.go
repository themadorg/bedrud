package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"bedrud/config"
	"bedrud/internal/auth"
	"bedrud/internal/middleware"
	"bedrud/internal/models"
	"bedrud/internal/repository"
	"bedrud/internal/testutil"

	"github.com/gofiber/fiber/v2"
)

func setupUsersGapsApp(t *testing.T) (*fiber.App, *repository.UserRepository, *auth.Claims) {
	t.Helper()
	db := testutil.SetupTestDB(t)
	userRepo := repository.NewUserRepository(db)
	roomRepo := repository.NewRoomRepository(db)
	passkeyRepo := repository.NewPasskeyRepository(db)
	prefsRepo := repository.NewUserPreferencesRepository(db)
	h := NewUsersHandler(userRepo, roomRepo, passkeyRepo, prefsRepo, nil, nil)

	claims := &auth.Claims{UserID: "admin-1", Email: "admin@ex.com", Name: "Admin", Accesses: []string{"user", "superadmin"}}
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user", claims)
		return c.Next()
	})
	app.Post("/admin/users/:id/force-logout", h.ForceLogout)
	app.Delete("/admin/users/:id", h.DeleteUser)
	app.Post("/admin/users/bulk/ban", h.BulkBanUsers)
	app.Post("/admin/users/bulk/promote", h.BulkPromoteUsers)
	app.Post("/admin/users/bulk/delete", h.BulkDeleteUsers)

	_ = userRepo.CreateUser(&models.User{ID: "admin-1", Email: "admin@ex.com", Name: "Admin", Provider: "local", IsActive: true, Accesses: models.StringArray{"user", "superadmin"}})
	_ = userRepo.CreateUser(&models.User{ID: "admin-2", Email: "admin2@ex.com", Name: "Admin2", Provider: "local", IsActive: true, Accesses: models.StringArray{"user", "superadmin"}})
	_ = userRepo.CreateUser(&models.User{ID: "target-1", Email: "t1@ex.com", Name: "Target", Provider: "local", IsActive: true, Accesses: models.StringArray{"user"}})
	return app, userRepo, claims
}

func TestForceLogout_Success(t *testing.T) {
	app, _, _ := setupUsersGapsApp(t)
	req := httptest.NewRequest(http.MethodPost, "/admin/users/target-1/force-logout", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status %d: %s", resp.StatusCode, b)
	}
}

func TestForceLogout_RejectsOldAccessAndRefresh(t *testing.T) {
	db := testutil.SetupTestDB(t)
	userRepo := repository.NewUserRepository(db)
	auth.SetAccessTokenBlockStore(userRepo)
	passkeyRepo := repository.NewPasskeyRepository(db)
	authSvc := auth.NewAuthService(userRepo, passkeyRepo)
	cfg := &config.Config{
		Auth: config.AuthConfig{
			JWTSecret:     "force-logout-test-secret-key-32b",
			TokenDuration: 1,
		},
	}
	config.SetForTest(cfg)

	hash, _ := auth.HashPassword("pass12345")
	target := &models.User{
		ID: "target-sess", Email: "sess@ex.com", Name: "Sess", Provider: "local",
		Password: hash, IsActive: true, Accesses: models.StringArray{"user"},
	}
	_ = userRepo.CreateUser(target)
	login, err := authSvc.Login("sess@ex.com", "pass12345")
	if err != nil {
		t.Fatal(err)
	}
	oldAccess := login.Token.AccessToken
	oldRefresh := login.Token.RefreshToken

	h := NewUsersHandler(userRepo, repository.NewRoomRepository(db), passkeyRepo, repository.NewUserPreferencesRepository(db), nil, nil)
	adminClaims := &auth.Claims{UserID: "admin-1", Accesses: []string{"user", "superadmin"}}
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user", adminClaims)
		return c.Next()
	})
	app.Post("/admin/users/:id/force-logout", h.ForceLogout)
	app.Get("/protected", middleware.Protected(), func(c *fiber.Ctx) error { return c.SendString("ok") })

	flReq := httptest.NewRequest(http.MethodPost, "/admin/users/target-sess/force-logout", http.NoBody)
	flResp, err := app.Test(flReq, -1)
	if err != nil {
		t.Fatal(err)
	}
	flResp.Body.Close()
	if flResp.StatusCode != http.StatusOK {
		t.Fatalf("force logout status %d", flResp.StatusCode)
	}

	protReq := httptest.NewRequest(http.MethodGet, "/protected", http.NoBody)
	protReq.Header.Set("Authorization", "Bearer "+oldAccess)
	protResp, err := app.Test(protReq, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer protResp.Body.Close()
	if protResp.StatusCode != http.StatusForbidden {
		b, _ := io.ReadAll(protResp.Body)
		t.Fatalf("old access: want 403, got %d: %s", protResp.StatusCode, b)
	}

	if _, err := authSvc.ValidateRefreshToken(oldRefresh); err == nil {
		t.Fatal("old refresh should be rejected after force logout")
	}
}

func TestForceLogout_NotFound(t *testing.T) {
	app, _, _ := setupUsersGapsApp(t)
	req := httptest.NewRequest(http.MethodPost, "/admin/users/missing/force-logout", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestDeleteUser_Self(t *testing.T) {
	app, _, _ := setupUsersGapsApp(t)
	req := httptest.NewRequest(http.MethodDelete, "/admin/users/admin-1", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestDeleteUser_Success(t *testing.T) {
	app, _, _ := setupUsersGapsApp(t)
	req := httptest.NewRequest(http.MethodDelete, "/admin/users/target-1", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	// queues deletion → 202
	if resp.StatusCode != http.StatusAccepted {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status %d: %s", resp.StatusCode, b)
	}
}

func TestBulkBanUsers_Success(t *testing.T) {
	app, _, _ := setupUsersGapsApp(t)
	body, _ := json.Marshal(map[string][]string{"ids": {"target-1"}})
	req := httptest.NewRequest(http.MethodPost, "/admin/users/bulk/ban", bytes.NewReader(body))
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
}

func TestBulkBanUsers_Empty(t *testing.T) {
	app, _, _ := setupUsersGapsApp(t)
	body, _ := json.Marshal(map[string][]string{"ids": {}})
	req := httptest.NewRequest(http.MethodPost, "/admin/users/bulk/ban", bytes.NewReader(body))
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

func TestBulkPromoteUsers_Success(t *testing.T) {
	app, _, _ := setupUsersGapsApp(t)
	body, _ := json.Marshal(map[string][]string{"ids": {"target-1"}})
	req := httptest.NewRequest(http.MethodPost, "/admin/users/bulk/promote", bytes.NewReader(body))
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
}

func TestBulkDeleteUsers_Empty(t *testing.T) {
	app, _, _ := setupUsersGapsApp(t)
	body, _ := json.Marshal(map[string][]string{"ids": {}})
	req := httptest.NewRequest(http.MethodPost, "/admin/users/bulk/delete", bytes.NewReader(body))
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

func TestBulkDeleteUsers_Success(t *testing.T) {
	app, _, _ := setupUsersGapsApp(t)
	body, _ := json.Marshal(map[string][]string{"ids": {"target-1"}})
	req := httptest.NewRequest(http.MethodPost, "/admin/users/bulk/delete", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status %d: %s", resp.StatusCode, b)
	}
}

func TestUsersAdmin_ForbiddenWithoutSuperadmin(t *testing.T) {
	db := testutil.SetupTestDB(t)
	userRepo := repository.NewUserRepository(db)
	h := NewUsersHandler(userRepo, repository.NewRoomRepository(db), repository.NewPasskeyRepository(db), repository.NewUserPreferencesRepository(db), nil, nil)
	claims := &auth.Claims{UserID: "u", Accesses: []string{"user"}}
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user", claims)
		return c.Next()
	})
	app.Post("/admin/users/:id/force-logout", h.ForceLogout)

	req := httptest.NewRequest(http.MethodPost, "/admin/users/x/force-logout", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}
