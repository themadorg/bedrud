package handlers

import (
	"bedrud/config"
	"bedrud/internal/auth"
	"bedrud/internal/lkutil"
	"bedrud/internal/models"
	"bedrud/internal/repository"
	"bedrud/internal/services"
	"bedrud/internal/storage"
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

func testCleanupSvc(t *testing.T, roomRepo *repository.RoomRepository, uploadTracker *storage.ChatUploadTracker) *services.RoomCleanupService {
	t.Helper()
	client := lkutil.NewClient(&config.LiveKitConfig{Host: "http://localhost:7880", APIKey: "test", APISecret: "testsecret1234567890123456789012"})
	return services.NewRoomCleanupService(roomRepo, nil, client, nil, "test", "testsecret1234567890123456789012", uploadTracker)
}

func testUsersHandler(userRepo *repository.UserRepository, roomRepo *repository.RoomRepository, passkeyRepo *repository.PasskeyRepository, prefsRepo *repository.UserPreferencesRepository, cleanupSvc *services.RoomCleanupService) *UsersHandler {
	return NewUsersHandler(userRepo, roomRepo, passkeyRepo, prefsRepo, cleanupSvc, nil)
}

// --- Helper ---

func setupUsersTestApp(t *testing.T) (*fiber.App, *repository.UserRepository) {
	t.Helper()
	config.SetForTest(&config.Config{})
	db := testutil.SetupTestDB(t)
	userRepo := repository.NewUserRepository(db)
	roomRepo := repository.NewRoomRepository(db)
	passkeyRepo := repository.NewPasskeyRepository(db)
	prefsRepo := repository.NewUserPreferencesRepository(db)
	uploadTracker := storage.NewChatUploadTracker(db, t.TempDir(), nil)
	cleanupSvc := testCleanupSvc(t, roomRepo, uploadTracker)
	usersHandler := testUsersHandler(userRepo, roomRepo, passkeyRepo, prefsRepo, cleanupSvc)

	app := fiber.New()
	// Simulate Protected middleware by injecting claims
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user", &auth.Claims{
			UserID:   "admin-user",
			Email:    "admin@ex.com",
			Name:     "Admin",
			Accesses: []string{"superadmin"},
		})
		return c.Next()
	})

	app.Get("/admin/users", usersHandler.ListUsers)
	app.Put("/admin/users/:id/status", usersHandler.UpdateUserStatus)
	app.Put("/admin/users/:id/password", usersHandler.SetUserPassword)
	app.Post("/admin/users/:id/verify", usersHandler.AdminVerifyEmail)
	app.Post("/admin/users/:id/verify/resend", usersHandler.AdminResendVerification)

	return app, userRepo
}

// --- ListUsers Tests ---

func TestUsersHandler_ListUsers_Empty(t *testing.T) {
	app, _ := setupUsersTestApp(t)

	req := httptest.NewRequest(http.MethodGet, "/admin/users", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var result UserListResponse
	_ = json.Unmarshal(body, &result)
	// Users list should be empty (or nil since no users exist)
	if len(result.Users) > 0 {
		t.Fatalf("expected empty users, got %d", len(result.Users))
	}
}

func TestUsersHandler_ListUsers_WithUsers(t *testing.T) {
	app, userRepo := setupUsersTestApp(t)

	// Create some users
	_ = userRepo.CreateUser(&models.User{ID: "u1", Email: "u1@ex.com", Name: "User 1", Provider: "local", IsActive: true, Accesses: models.StringArray{"user"}})
	_ = userRepo.CreateUser(&models.User{ID: "u2", Email: "u2@ex.com", Name: "User 2", Provider: "google", IsActive: false, Accesses: models.StringArray{"admin"}})

	req := httptest.NewRequest(http.MethodGet, "/admin/users", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var result UserListResponse
	_ = json.Unmarshal(body, &result)
	if len(result.Users) != 2 {
		t.Fatalf("expected 2 users, got %d", len(result.Users))
	}

	// Verify user details
	foundU1 := false
	for _, u := range result.Users {
		if u.ID == "u1" {
			foundU1 = true
			if u.Email != "u1@ex.com" {
				t.Fatalf("expected email 'u1@ex.com', got '%s'", u.Email)
			}
			if u.Provider != "local" {
				t.Fatalf("expected provider 'local', got '%s'", u.Provider)
			}
			if !u.IsActive {
				t.Fatal("expected u1 to be active")
			}
		}
	}
	if !foundU1 {
		t.Fatal("expected to find u1 in response")
	}
}

// --- UpdateUserStatus Tests ---

func TestUsersHandler_UpdateUserStatus_Success(t *testing.T) {
	app, userRepo := setupUsersTestApp(t)

	_ = userRepo.CreateUser(&models.User{ID: "target-user", Email: "target@ex.com", Name: "Target", Provider: "local", IsActive: true})

	body, _ := json.Marshal(UserStatusUpdateRequest{Active: false})
	req := httptest.NewRequest(http.MethodPut, "/admin/users/target-user/status", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	// Verify user was deactivated
	user, _ := userRepo.GetUserByID("target-user")
	if user.IsActive {
		t.Fatal("expected user to be deactivated")
	}
}

func TestUsersHandler_UpdateUserStatus_Reactivate(t *testing.T) {
	app, userRepo := setupUsersTestApp(t)

	_ = userRepo.CreateUser(&models.User{ID: "inactive-user", Email: "inactive@ex.com", Name: "Inactive", Provider: "local", IsActive: false})

	body, _ := json.Marshal(UserStatusUpdateRequest{Active: true})
	req := httptest.NewRequest(http.MethodPut, "/admin/users/inactive-user/status", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	user, _ := userRepo.GetUserByID("inactive-user")
	if !user.IsActive {
		t.Fatal("expected user to be reactivated")
	}
}

func TestUsersHandler_UpdateUserStatus_NotFound(t *testing.T) {
	app, _ := setupUsersTestApp(t)

	body, _ := json.Marshal(UserStatusUpdateRequest{Active: false})
	req := httptest.NewRequest(http.MethodPut, "/admin/users/nonexistent-id/status", bytes.NewReader(body))
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

func TestUsersHandler_UpdateUserStatus_InvalidBody(t *testing.T) {
	app, _ := setupUsersTestApp(t)

	req := httptest.NewRequest(http.MethodPut, "/admin/users/some-id/status", bytes.NewReader([]byte("not json")))
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

// --- Handler model struct tests ---

func TestErrorResponse_Structure(t *testing.T) {
	e := ErrorResponse{Error: "test error"}
	b, _ := json.Marshal(e)
	var decoded map[string]string
	_ = json.Unmarshal(b, &decoded)
	if decoded["error"] != "test error" {
		t.Fatal("unexpected JSON representation")
	}
}

func TestAuthResponse_Structure(t *testing.T) {
	a := AuthResponse{
		User: UserResponse{
			ID:        "123",
			Email:     "test@ex.com",
			Name:      "Test",
			Provider:  "google",
			AvatarURL: "https://avatar.com/pic.jpg",
		},
		Token: "some-jwt-token",
	}
	if a.User.ID != "123" {
		t.Fatal("unexpected user ID")
	}
	if a.Token != "some-jwt-token" {
		t.Fatal("unexpected token")
	}
}

func TestUserResponse_JSONTags(t *testing.T) {
	u := UserResponse{
		ID:        "id",
		Email:     "e",
		Name:      "n",
		Provider:  "p",
		AvatarURL: "url",
	}
	b, _ := json.Marshal(u)
	var m map[string]interface{}
	_ = json.Unmarshal(b, &m)

	// Check JSON keys
	if _, ok := m["id"]; !ok {
		t.Fatal("expected 'id' key in JSON")
	}
	if _, ok := m["avatarUrl"]; !ok {
		t.Fatal("expected 'avatarUrl' key in JSON")
	}
}

func TestUsersHandler_UpdateUserAccesses_Success(t *testing.T) {
	_, userRepo := setupUsersTestApp(t)

	// Use a fresh DB/handler for these routes
	db := testutil.SetupTestDB(t)
	uRepo := repository.NewUserRepository(db)
	rRepo := repository.NewRoomRepository(db)
	pkRepo := repository.NewPasskeyRepository(db)
	prRepo := repository.NewUserPreferencesRepository(db)
	ut := storage.NewChatUploadTracker(db, t.TempDir(), nil)
	cleanupSvc := testCleanupSvc(t, rRepo, ut)
	h := testUsersHandler(uRepo, rRepo, pkRepo, prRepo, cleanupSvc)

	app2 := fiber.New()
	app2.Use(func(c *fiber.Ctx) error {
		c.Locals("user", &auth.Claims{
			UserID:   "admin-user",
			Email:    "admin@ex.com",
			Name:     "Admin",
			Accesses: []string{"superadmin"},
		})
		return c.Next()
	})
	app2.Put("/admin/users/:id/accesses", h.UpdateUserAccesses)
	app2.Get("/admin/users/:id", h.GetUserDetail)

	_ = uRepo.CreateUser(&models.User{ID: "target-acc", Email: "acc@ex.com", Name: "Acc", Provider: "local", IsActive: true, Accesses: models.StringArray{"user"}})

	body, _ := json.Marshal(map[string]interface{}{"accesses": []string{"admin", "user"}})
	req := httptest.NewRequest(http.MethodPut, "/admin/users/target-acc/accesses", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app2.Test(req, -1)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	_ = userRepo
}

func TestUsersHandler_UpdateUserAccesses_NotFound(t *testing.T) {
	db := testutil.SetupTestDB(t)
	uRepo := repository.NewUserRepository(db)
	rRepo := repository.NewRoomRepository(db)
	pkRepo := repository.NewPasskeyRepository(db)
	prRepo := repository.NewUserPreferencesRepository(db)
	ut := storage.NewChatUploadTracker(db, t.TempDir(), nil)
	cleanupSvc := testCleanupSvc(t, rRepo, ut)
	h := testUsersHandler(uRepo, rRepo, pkRepo, prRepo, cleanupSvc)

	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user", &auth.Claims{UserID: "admin", Accesses: []string{"superadmin"}})
		return c.Next()
	})
	app.Put("/admin/users/:id/accesses", h.UpdateUserAccesses)

	body, _ := json.Marshal(map[string]interface{}{"accesses": []string{"admin"}})
	req := httptest.NewRequest(http.MethodPut, "/admin/users/nonexistent/accesses", bytes.NewReader(body))
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

func TestUsersHandler_UpdateUserAccesses_Forbidden(t *testing.T) {
	db := testutil.SetupTestDB(t)
	uRepo := repository.NewUserRepository(db)
	rRepo := repository.NewRoomRepository(db)
	pkRepo := repository.NewPasskeyRepository(db)
	prRepo := repository.NewUserPreferencesRepository(db)
	ut := storage.NewChatUploadTracker(db, t.TempDir(), nil)
	cleanupSvc := testCleanupSvc(t, rRepo, ut)
	h := testUsersHandler(uRepo, rRepo, pkRepo, prRepo, cleanupSvc)

	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user", &auth.Claims{UserID: "regular-user", Accesses: []string{"user"}})
		return c.Next()
	})
	app.Put("/admin/users/:id/accesses", h.UpdateUserAccesses)

	body, _ := json.Marshal(map[string]interface{}{"accesses": []string{"admin"}})
	req := httptest.NewRequest(http.MethodPut, "/admin/users/someone/accesses", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}

func TestUsersHandler_GetUserDetail_Found(t *testing.T) {
	db := testutil.SetupTestDB(t)
	uRepo := repository.NewUserRepository(db)
	rRepo := repository.NewRoomRepository(db)
	pkRepo := repository.NewPasskeyRepository(db)
	prRepo := repository.NewUserPreferencesRepository(db)
	ut := storage.NewChatUploadTracker(db, t.TempDir(), nil)
	cleanupSvc := testCleanupSvc(t, rRepo, ut)
	h := testUsersHandler(uRepo, rRepo, pkRepo, prRepo, cleanupSvc)

	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user", &auth.Claims{UserID: "admin", Accesses: []string{"superadmin"}})
		return c.Next()
	})
	app.Get("/admin/users/:id", h.GetUserDetail)

	_ = uRepo.CreateUser(&models.User{ID: "detail-user", Email: "detail@ex.com", Name: "Detail", Provider: "local", IsActive: true, Accesses: models.StringArray{"user"}})

	req := httptest.NewRequest(http.MethodGet, "/admin/users/detail-user", http.NoBody)
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
	_ = json.Unmarshal(body, &result)
	if result["user"] == nil {
		t.Fatal("expected 'user' field in response")
	}
	if result["rooms"] == nil {
		t.Fatal("expected 'rooms' field in response")
	}
}

func TestUsersHandler_GetUserDetail_NotFound(t *testing.T) {
	db := testutil.SetupTestDB(t)
	uRepo := repository.NewUserRepository(db)
	rRepo := repository.NewRoomRepository(db)
	pkRepo := repository.NewPasskeyRepository(db)
	prRepo := repository.NewUserPreferencesRepository(db)
	ut := storage.NewChatUploadTracker(db, t.TempDir(), nil)
	cleanupSvc := testCleanupSvc(t, rRepo, ut)
	h := testUsersHandler(uRepo, rRepo, pkRepo, prRepo, cleanupSvc)

	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user", &auth.Claims{UserID: "admin", Accesses: []string{"superadmin"}})
		return c.Next()
	})
	app.Get("/admin/users/:id", h.GetUserDetail)

	req := httptest.NewRequest(http.MethodGet, "/admin/users/nonexistent", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

// --- SetUserPassword Tests ---

func TestUsersHandler_SetUserPassword_Success(t *testing.T) {
	app, userRepo := setupUsersTestApp(t)

	_ = userRepo.CreateUser(&models.User{
		ID:           "pw-target",
		Email:        "pw@ex.com",
		Name:         "PW Target",
		Provider:     "local",
		IsActive:     true,
		RefreshToken: "old-refresh-hash",
	})

	body, _ := json.Marshal(map[string]string{"password": "newSecurePass123"})
	req := httptest.NewRequest(http.MethodPut, "/admin/users/pw-target/password", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	user, _ := userRepo.GetUserByID("pw-target")
	if err := auth.VerifyPassword("newSecurePass123", user.Password); err != nil {
		t.Fatal("password was not updated correctly")
	}
	if user.RefreshToken != "" {
		t.Fatal("expected refresh token to be cleared")
	}
}

func TestUsersHandler_SetUserPassword_MaxLength(t *testing.T) {
	app, userRepo := setupUsersTestApp(t)

	_ = userRepo.CreateUser(&models.User{
		ID:       "pw-max",
		Email:    "pwmax@ex.com",
		Name:     "Max",
		Provider: "local",
	})

	pass := strings.Repeat("a", MaxPasswordLength)
	body, _ := json.Marshal(map[string]string{"password": pass})
	req := httptest.NewRequest(http.MethodPut, "/admin/users/pw-max/password", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(body))
	}
}

func TestUsersHandler_SetUserPassword_TooShort(t *testing.T) {
	app, userRepo := setupUsersTestApp(t)

	_ = userRepo.CreateUser(&models.User{
		ID:       "pw-short",
		Email:    "pwshort@ex.com",
		Name:     "Short",
		Provider: "local",
	})

	body, _ := json.Marshal(map[string]string{"password": "short"})
	req := httptest.NewRequest(http.MethodPut, "/admin/users/pw-short/password", bytes.NewReader(body))
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

func TestUsersHandler_SetUserPassword_TooLong(t *testing.T) {
	app, userRepo := setupUsersTestApp(t)

	_ = userRepo.CreateUser(&models.User{
		ID:       "pw-long",
		Email:    "pwlong@ex.com",
		Name:     "Long",
		Provider: "local",
	})

	// 73 chars (exceeds MaxPasswordLength=72)
	body, _ := json.Marshal(map[string]string{"password": strings.Repeat("a", MaxPasswordLength+1)})
	req := httptest.NewRequest(http.MethodPut, "/admin/users/pw-long/password", bytes.NewReader(body))
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

func TestUsersHandler_SetUserPassword_NotFound(t *testing.T) {
	app, _ := setupUsersTestApp(t)

	body, _ := json.Marshal(map[string]string{"password": "validPassword123"})
	req := httptest.NewRequest(http.MethodPut, "/admin/users/nonexistent/password", bytes.NewReader(body))
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

func TestUsersHandler_SetUserPassword_InvalidBody(t *testing.T) {
	app, _ := setupUsersTestApp(t)

	req := httptest.NewRequest(http.MethodPut, "/admin/users/some-id/password", bytes.NewReader([]byte("not json")))
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

func TestUsersHandler_SetUserPassword_Forbidden(t *testing.T) {
	db := testutil.SetupTestDB(t)
	uRepo := repository.NewUserRepository(db)
	rRepo := repository.NewRoomRepository(db)
	pkRepo := repository.NewPasskeyRepository(db)
	prRepo := repository.NewUserPreferencesRepository(db)
	ut := storage.NewChatUploadTracker(db, t.TempDir(), nil)
	cleanupSvc := testCleanupSvc(t, rRepo, ut)
	h := testUsersHandler(uRepo, rRepo, pkRepo, prRepo, cleanupSvc)

	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user", &auth.Claims{UserID: "regular-user", Accesses: []string{"user"}})
		return c.Next()
	})
	app.Put("/admin/users/:id/password", h.SetUserPassword)

	_ = uRepo.CreateUser(&models.User{ID: "target", Email: "t@ex.com", Name: "T", Provider: "local", Password: "original-hash"})

	original, _ := uRepo.GetUserByID("target")
	origPass := original.Password

	body, _ := json.Marshal(map[string]string{"password": "newSecurePass123"})
	req := httptest.NewRequest(http.MethodPut, "/admin/users/target/password", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}

	user, _ := uRepo.GetUserByID("target")
	if user.Password != origPass {
		t.Fatal("password was modified despite forbidden response")
	}
}

func TestUsersHandler_ListUserSessions_Found(t *testing.T) {
	db := testutil.SetupTestDB(t)
	uRepo := repository.NewUserRepository(db)
	rRepo := repository.NewRoomRepository(db)
	pkRepo := repository.NewPasskeyRepository(db)
	prRepo := repository.NewUserPreferencesRepository(db)
	ut := storage.NewChatUploadTracker(db, t.TempDir(), nil)
	cleanupSvc := testCleanupSvc(t, rRepo, ut)
	h := testUsersHandler(uRepo, rRepo, pkRepo, prRepo, cleanupSvc)

	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user", &auth.Claims{UserID: "admin", Accesses: []string{"superadmin"}})
		return c.Next()
	})
	app.Get("/admin/users/:id/sessions", h.ListUserSessions)

	_ = uRepo.CreateUser(&models.User{ID: "session-user", Email: "sess@ex.com", Name: "Session", Provider: "local", IsActive: true})
	room, _ := rRepo.CreateRoom("session-user", "session-room", false, "standard", 0, &models.RoomSettings{})
	_ = rRepo.AddParticipant(room.ID, "session-user")

	req := httptest.NewRequest(http.MethodGet, "/admin/users/session-user/sessions?page=1&limit=20", http.NoBody)
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
	_ = json.Unmarshal(body, &result)
	if result["sessions"] == nil {
		t.Fatal("expected 'sessions' field")
	}
	if result["total"] == nil {
		t.Fatal("expected 'total' field")
	}
	sessions := result["sessions"].([]interface{})
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	s := sessions[0].(map[string]interface{})
	if s["roomName"] != "session-room" {
		t.Fatalf("expected roomName 'session-room', got %v", s["roomName"])
	}
	if s["isActive"] != true {
		t.Fatal("expected isActive true")
	}
}

func TestUsersHandler_ListUserSessions_NotFound(t *testing.T) {
	db := testutil.SetupTestDB(t)
	uRepo := repository.NewUserRepository(db)
	rRepo := repository.NewRoomRepository(db)
	pkRepo := repository.NewPasskeyRepository(db)
	prRepo := repository.NewUserPreferencesRepository(db)
	ut := storage.NewChatUploadTracker(db, t.TempDir(), nil)
	cleanupSvc := testCleanupSvc(t, rRepo, ut)
	h := testUsersHandler(uRepo, rRepo, pkRepo, prRepo, cleanupSvc)

	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user", &auth.Claims{UserID: "admin", Accesses: []string{"superadmin"}})
		return c.Next()
	})
	app.Get("/admin/users/:id/sessions", h.ListUserSessions)

	req := httptest.NewRequest(http.MethodGet, "/admin/users/nonexistent/sessions", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestUsersHandler_ListUserSessions_Empty(t *testing.T) {
	db := testutil.SetupTestDB(t)
	uRepo := repository.NewUserRepository(db)
	rRepo := repository.NewRoomRepository(db)
	pkRepo := repository.NewPasskeyRepository(db)
	prRepo := repository.NewUserPreferencesRepository(db)
	ut := storage.NewChatUploadTracker(db, t.TempDir(), nil)
	cleanupSvc := testCleanupSvc(t, rRepo, ut)
	h := testUsersHandler(uRepo, rRepo, pkRepo, prRepo, cleanupSvc)

	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user", &auth.Claims{UserID: "admin", Accesses: []string{"superadmin"}})
		return c.Next()
	})
	app.Get("/admin/users/:id/sessions", h.ListUserSessions)

	_ = uRepo.CreateUser(&models.User{ID: "alone-user", Email: "alone@ex.com", Name: "Alone", Provider: "local", IsActive: true})

	req := httptest.NewRequest(http.MethodGet, "/admin/users/alone-user/sessions", http.NoBody)
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
	_ = json.Unmarshal(body, &result)
	sessions := result["sessions"].([]interface{})
	if len(sessions) != 0 {
		t.Fatalf("expected 0 sessions, got %d", len(sessions))
	}
}

func TestUsersHandler_ListUserSessions_Forbidden(t *testing.T) {
	db := testutil.SetupTestDB(t)
	uRepo := repository.NewUserRepository(db)
	rRepo := repository.NewRoomRepository(db)
	pkRepo := repository.NewPasskeyRepository(db)
	prRepo := repository.NewUserPreferencesRepository(db)
	ut := storage.NewChatUploadTracker(db, t.TempDir(), nil)
	cleanupSvc := testCleanupSvc(t, rRepo, ut)
	h := testUsersHandler(uRepo, rRepo, pkRepo, prRepo, cleanupSvc)

	app := fiber.New()
	// Non-superadmin claims
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user", &auth.Claims{UserID: "moderator", Accesses: []string{"moderator"}})
		return c.Next()
	})
	app.Get("/admin/users/:id/sessions", h.ListUserSessions)

	req := httptest.NewRequest(http.MethodGet, "/admin/users/any-user/sessions", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}

func TestUserDetails_Structure(t *testing.T) {
	d := UserDetails{
		ID:        "u1",
		Email:     "e@e.com",
		Name:      "N",
		Provider:  "local",
		IsActive:  true,
		Accesses:  []string{"user"},
		CreatedAt: "2025-01-01 12:00:00",
	}
	if d.ID != "u1" || d.CreatedAt != "2025-01-01 12:00:00" {
		t.Fatal("unexpected UserDetails values")
	}
}

// --- AdminVerifyEmail Tests ---

func TestAdminVerifyEmail_Success(t *testing.T) {
	app, userRepo := setupUsersTestApp(t)

	_ = userRepo.CreateUser(&models.User{
		ID: "verify-target", Email: "verify@ex.com", Name: "Verify Target",
		Provider: "local", IsActive: true, Accesses: models.StringArray{"user"},
	})

	req := httptest.NewRequest(http.MethodPost, "/admin/users/verify-target/verify", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(respBody))
	}

	user, _ := userRepo.GetUserByID("verify-target")
	if user == nil || user.EmailVerifiedAt == nil {
		t.Fatal("expected EmailVerifiedAt to be set after admin verify")
	}
}

func TestAdminVerifyEmail_AlreadyVerified(t *testing.T) {
	app, userRepo := setupUsersTestApp(t)
	now := time.Now()

	_ = userRepo.CreateUser(&models.User{
		ID: "already-verified", Email: "alr@ex.com", Name: "Already",
		Provider: "local", IsActive: true, Accesses: models.StringArray{"user"},
		EmailVerifiedAt: &now,
	})

	req := httptest.NewRequest(http.MethodPost, "/admin/users/already-verified/verify", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 (already verified), got %d", resp.StatusCode)
	}
}

func TestAdminVerifyEmail_NotFound(t *testing.T) {
	app, _ := setupUsersTestApp(t)

	req := httptest.NewRequest(http.MethodPost, "/admin/users/nonexistent/verify", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

// --- AdminResendVerification Tests ---

func TestAdminResendVerification_Success(t *testing.T) {
	app, userRepo := setupUsersTestApp(t)

	_ = userRepo.CreateUser(&models.User{
		ID: "resend-target", Email: "resend@ex.com", Name: "Resend Target",
		Provider: "local", IsActive: true, Accesses: models.StringArray{"user"},
	})

	req := httptest.NewRequest(http.MethodPost, "/admin/users/resend-target/verify/resend", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	// With no SMTP config, the queue will enqueue but delivery fails gracefully
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(respBody))
	}
}

func TestAdminResendVerification_NotFound(t *testing.T) {
	app, _ := setupUsersTestApp(t)

	req := httptest.NewRequest(http.MethodPost, "/admin/users/nonexistent/verify/resend", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

// -------------------------------------------------------------------------
// Pagination Edge Case Tests
// -------------------------------------------------------------------------

func TestListUsers_Pagination(t *testing.T) {
	app, userRepo := setupUsersTestApp(t)

	// Seed 5 users
	for i := range 5 {
		userRepo.CreateUser(&models.User{
			ID:       fmt.Sprintf("pag-user-%d", i),
			Email:    fmt.Sprintf("pag%d@ex.com", i),
			Name:     fmt.Sprintf("User %d", i),
			Provider: "local",
			IsActive: true,
		})
	}

	t.Run("page=0 defaults to 1", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/users?page=0", http.NoBody)
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
		page, _ := body["page"].(float64)
		if page != 1 {
			t.Fatalf("expected page 1, got %f", page)
		}
	})

	t.Run("limit=0 defaults to 50", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/users?limit=0", http.NoBody)
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatalf("app.Test failed: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()
		var body map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		users, _ := body["users"].([]any)
		if len(users) != 5 {
			t.Fatalf("expected 5 users with limit=0, got %d", len(users))
		}
	})

	t.Run("limit=-1 defaults to 50", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/users?limit=-1", http.NoBody)
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatalf("app.Test failed: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()
		var body map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		users, _ := body["users"].([]any)
		if len(users) != 5 {
			t.Fatalf("expected 5 users with limit=-1, got %d", len(users))
		}
	})

	t.Run("limit > 100 clamped to 50", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/users?limit=200", http.NoBody)
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatalf("app.Test failed: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()
		var body map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		l, _ := body["limit"].(float64)
		if l != 50 {
			t.Fatalf("expected limit clamped to 50, got %f", l)
		}
	})

	t.Run("page > total returns empty", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/users?page=999", http.NoBody)
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatalf("app.Test failed: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()
		var body map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		users, _ := body["users"].([]any)
		if len(users) != 0 {
			t.Fatalf("expected empty users for page beyond total, got %d", len(users))
		}
	})
}
