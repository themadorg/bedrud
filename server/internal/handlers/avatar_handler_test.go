package handlers

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"bedrud/config"
	"bedrud/internal/auth"
	"bedrud/internal/repository"
	"bedrud/internal/storage"
	"bedrud/internal/testutil"

	"github.com/gofiber/fiber/v2"
)

func jpegBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.White)
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 80}); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func setupAvatarTestApp(t *testing.T) (*fiber.App, *auth.AuthService, *config.Config, string) {
	t.Helper()
	dir := t.TempDir()
	restore := storage.SetAvatarDirForTest(dir)
	t.Cleanup(restore)

	db := testutil.SetupTestDB(t)
	userRepo := repository.NewUserRepository(db)
	passkeyRepo := repository.NewPasskeyRepository(db)
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
	h := NewAuthHandler(authService, cfg, nil, nil, nil, NewCooldownCache(0), nil)

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
	app.Post("/api/auth/me/avatar", authMW, h.UploadAvatar)
	app.Delete("/api/auth/me/avatar", authMW, h.DeleteAvatar)

	user, err := authService.Register("avatar@ex.com", "securepass123", "Avatar User")
	if err != nil {
		t.Fatal(err)
	}

	return app, authService, cfg, user.ID
}

func authHeaderFor(t *testing.T, authService *auth.AuthService, cfg *config.Config, email string) string {
	t.Helper()
	u, err := authService.GetUserByEmail(email)
	if err != nil || u == nil {
		t.Fatalf("get user %s: %v", email, err)
	}
	token, err := auth.GenerateToken(u.ID, u.Email, u.Name, u.Provider, u.Accesses, cfg, u.EmailVerifiedAt)
	if err != nil {
		t.Fatal(err)
	}
	return "Bearer " + token
}

func multipartAvatar(t *testing.T, field string, filename string, data []byte) (*bytes.Buffer, string) {
	t.Helper()
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	if field != "" {
		part, err := w.CreateFormFile(field, filename)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	_ = w.Close()
	return &body, w.FormDataContentType()
}

func TestUploadAvatar_Success(t *testing.T) {
	app, authService, cfg, _ := setupAvatarTestApp(t)
	body, ct := multipartAvatar(t, "avatar", "a.jpg", jpegBytes(t, 16, 16))
	req := httptest.NewRequest(http.MethodPost, "/api/auth/me/avatar", body)
	req.Header.Set("Content-Type", ct)
	req.Header.Set("Authorization", authHeaderFor(t, authService, cfg, "avatar@ex.com"))
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
	if result["avatarUrl"] == nil || result["avatarUrl"] == "" {
		t.Fatalf("expected avatarUrl, got %#v", result)
	}
}

func TestUploadAvatar_MissingFile(t *testing.T) {
	app, authService, cfg, _ := setupAvatarTestApp(t)
	body, ct := multipartAvatar(t, "", "", nil)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/me/avatar", body)
	req.Header.Set("Content-Type", ct)
	req.Header.Set("Authorization", authHeaderFor(t, authService, cfg, "avatar@ex.com"))
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestUploadAvatar_Oversize(t *testing.T) {
	app, authService, cfg, _ := setupAvatarTestApp(t)
	// Multipart size > 2MiB
	data := make([]byte, storage.AvatarMaxBytes()+1)
	body, ct := multipartAvatar(t, "avatar", "big.jpg", data)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/me/avatar", body)
	req.Header.Set("Content-Type", ct)
	req.Header.Set("Authorization", authHeaderFor(t, authService, cfg, "avatar@ex.com"))
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestDeleteAvatar_ClearsURL(t *testing.T) {
	app, authService, cfg, userID := setupAvatarTestApp(t)
	// seed avatar
	url, err := storage.SaveUserAvatar(userID, jpegBytes(t, 8, 8))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := authService.UpdateAvatarURL(userID, url); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/auth/me/avatar", http.NoBody)
	req.Header.Set("Authorization", authHeaderFor(t, authService, cfg, "avatar@ex.com"))
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
	if result["avatarUrl"] != "" && result["avatarUrl"] != nil {
		t.Fatalf("expected empty avatarUrl, got %#v", result["avatarUrl"])
	}
}

func TestUploadAvatar_Unauthenticated(t *testing.T) {
	app, _, _, _ := setupAvatarTestApp(t)
	body, ct := multipartAvatar(t, "avatar", "a.jpg", jpegBytes(t, 4, 4))
	req := httptest.NewRequest(http.MethodPost, "/api/auth/me/avatar", body)
	req.Header.Set("Content-Type", ct)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}
