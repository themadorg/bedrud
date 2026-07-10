package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"bedrud/internal/auth"
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
