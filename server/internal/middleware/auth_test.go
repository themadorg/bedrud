package middleware

import (
	"bedrud/config"
	"bedrud/internal/auth"
	"bedrud/internal/models"
	"bedrud/internal/repository"
	"bedrud/internal/testutil"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
)

// setupTestConfig initializes a minimal config for testing middleware.
// Uses sync.Once internally via config.Load, so we create config inline.
var (
	testCfg     *config.Config
	testCfgOnce sync.Once
)

func getTestConfig() *config.Config {
	testCfgOnce.Do(func() {
		testCfg = &config.Config{
			Auth: config.AuthConfig{
				JWTSecret:     "middleware-test-secret",
				TokenDuration: 1,
			},
		}
		config.SetForTest(testCfg)
	})
	return testCfg
}

const testBearerPrefix = "Bearer "

// --- Tests using the real Protected() and RequireAccess() functions ---

func TestProtected_Real_NoAuthHeader(t *testing.T) {
	getTestConfig() // ensure config is set
	app := fiber.New()
	app.Use(Protected())
	app.Get("/test", func(c *fiber.Ctx) error { return c.SendString("ok") })

	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	resp, _ := app.Test(req)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != fiber.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestProtected_Real_ValidToken(t *testing.T) {
	cfg := getTestConfig()
	token, _ := auth.GenerateToken("u1", "t@ex.com", "T", "local", []string{"user"}, cfg, nil)

	app := fiber.New()
	app.Use(Protected())
	app.Get("/test", func(c *fiber.Ctx) error {
		claims := c.Locals("user").(*auth.Claims)
		return c.SendString(claims.UserID)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, _ := app.Test(req)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "u1" {
		t.Fatalf("expected 'u1', got '%s'", string(body))
	}
}

func TestProtected_Real_InvalidToken(t *testing.T) {
	getTestConfig()
	app := fiber.New()
	app.Use(Protected())
	app.Get("/test", func(c *fiber.Ctx) error { return c.SendString("ok") })

	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	req.Header.Set("Authorization", "Bearer bad-token")
	resp, _ := app.Test(req)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != fiber.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestProtected_Real_NoBearerPrefix(t *testing.T) {
	cfg := getTestConfig()
	token, _ := auth.GenerateToken("u2", "u2@ex.com", "U2", "local", []string{"user"}, cfg, nil)

	app := fiber.New()
	app.Use(Protected())
	app.Get("/test", func(c *fiber.Ctx) error {
		claims := c.Locals("user").(*auth.Claims)
		return c.SendString(claims.UserID)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	req.Header.Set("Authorization", token) // no "Bearer " prefix
	resp, _ := app.Test(req)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestRequireAccess_Real_HasAccess(t *testing.T) {
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user", &auth.Claims{UserID: "u1", Accesses: []string{"admin"}})
		return c.Next()
	})
	app.Use(RequireAccess(models.AccessAdmin))
	app.Get("/admin", func(c *fiber.Ctx) error { return c.SendString("ok") })

	req := httptest.NewRequest(http.MethodGet, "/admin", http.NoBody)
	resp, _ := app.Test(req)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestRequireAccess_Real_InsufficientAccess(t *testing.T) {
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user", &auth.Claims{UserID: "u1", Accesses: []string{"user"}})
		return c.Next()
	})
	app.Use(RequireAccess(models.AccessAdmin))
	app.Get("/admin", func(c *fiber.Ctx) error { return c.SendString("ok") })

	req := httptest.NewRequest(http.MethodGet, "/admin", http.NoBody)
	resp, _ := app.Test(req)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != fiber.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}

func TestProtected_NoAuthHeader(t *testing.T) {
	app := fiber.New()
	// We need to inject config for middleware to work. The middleware uses config.Get()
	// which panics if not loaded. We'll skip that by testing behavior directly.
	// Instead, let's test the handler logic inline.

	app.Use(func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Missing authorization header",
			})
		}
		return c.Next()
	})
	app.Get("/test", func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != fiber.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestProtected_WithValidBearerToken(t *testing.T) {
	cfg := getTestConfig()
	token, _ := auth.GenerateToken("user-1", "test@ex.com", "Test", "local", []string{"user"}, cfg, nil)

	app := fiber.New()
	// Inline middleware similar to Protected() but using our test config directly
	app.Use(func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Missing authorization header"})
		}
		tokenStr := authHeader
		if len(authHeader) > 7 && authHeader[:7] == testBearerPrefix {
			tokenStr = authHeader[7:]
		}
		claims, err := auth.ValidateToken(tokenStr, cfg)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid token"})
		}
		c.Locals("user", claims)
		return c.Next()
	})
	app.Get("/test", func(c *fiber.Ctx) error {
		claims := c.Locals("user").(*auth.Claims)
		return c.SendString(claims.UserID)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "user-1" {
		t.Fatalf("expected 'user-1', got '%s'", string(body))
	}
}

func TestProtected_WithTokenWithoutBearerPrefix(t *testing.T) {
	cfg := getTestConfig()
	token, _ := auth.GenerateToken("user-2", "test2@ex.com", "Test2", "local", []string{"user"}, cfg, nil)

	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Missing authorization header"})
		}
		tokenStr := authHeader
		if len(authHeader) > 7 && authHeader[:7] == testBearerPrefix {
			tokenStr = authHeader[7:]
		}
		claims, err := auth.ValidateToken(tokenStr, cfg)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid token"})
		}
		c.Locals("user", claims)
		return c.Next()
	})
	app.Get("/test", func(c *fiber.Ctx) error {
		claims := c.Locals("user").(*auth.Claims)
		return c.SendString(claims.UserID)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	req.Header.Set("Authorization", token) // No "Bearer " prefix
	resp, _ := app.Test(req)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestProtected_InvalidToken(t *testing.T) {
	cfg := getTestConfig()

	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Missing"})
		}
		tokenStr := authHeader
		if len(authHeader) > 7 && authHeader[:7] == testBearerPrefix {
			tokenStr = authHeader[7:]
		}
		_, err := auth.ValidateToken(tokenStr, cfg)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid token"})
		}
		return c.Next()
	})
	app.Get("/test", func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	req.Header.Set("Authorization", "Bearer invalid-token-content")
	resp, _ := app.Test(req)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != fiber.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestRequireAccess_HasAccess(t *testing.T) {
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		// Simulate Protected middleware setting claims
		c.Locals("user", &auth.Claims{
			UserID:   "user-1",
			Accesses: []string{"user", "admin"},
		})
		return c.Next()
	})
	app.Use(func(c *fiber.Ctx) error {
		claims := c.Locals("user").(*auth.Claims)
		for _, access := range claims.Accesses {
			if access == string(models.AccessAdmin) {
				return c.Next()
			}
		}
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Insufficient access rights"})
	})
	app.Get("/admin", func(c *fiber.Ctx) error {
		return c.SendString("admin page")
	})

	req := httptest.NewRequest(http.MethodGet, "/admin", http.NoBody)
	resp, _ := app.Test(req)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

// --- RequireEmailVerified middleware tests ---

func TestRequireEmailVerified_Disabled(t *testing.T) {
	cfg := getTestConfig()
	cfg.Auth.RequireEmailVerification = false
	config.SetForTest(cfg)

	db := testutil.SetupTestDB(t)
	userRepo := repository.NewUserRepository(db)

	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user", &auth.Claims{UserID: "u1", Provider: "local"})
		return c.Next()
	})
	app.Use(RequireEmailVerified(cfg, userRepo))
	app.Get("/test", func(c *fiber.Ctx) error { return c.SendString("ok") })

	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	resp, _ := app.Test(req)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 when verification disabled, got %d", resp.StatusCode)
	}
}

func TestRequireEmailVerified_BlocksUnverified(t *testing.T) {
	cfg := getTestConfig()
	cfg.Auth.RequireEmailVerification = true
	config.SetForTest(cfg)

	db := testutil.SetupTestDB(t)
	userRepo := repository.NewUserRepository(db)

	// Create unverified user
	user := &models.User{
		ID: "unver-id", Email: "unver@test.com", Name: "Unverified",
		Provider: "local", IsActive: true, Accesses: models.StringArray{"user"},
	}
	if err := userRepo.CreateUser(user); err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user", &auth.Claims{UserID: "unver-id", Provider: "local"})
		return c.Next()
	})
	app.Use(RequireEmailVerified(cfg, userRepo))
	app.Get("/test", func(c *fiber.Ctx) error { return c.SendString("ok") })

	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	resp, _ := app.Test(req)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 for unverified user, got %d", resp.StatusCode)
	}
}

func TestRequireEmailVerified_AllowsVerified(t *testing.T) {
	cfg := getTestConfig()
	cfg.Auth.RequireEmailVerification = true
	config.SetForTest(cfg)

	db := testutil.SetupTestDB(t)
	userRepo := repository.NewUserRepository(db)

	// Create verified user
	now := time.Now()
	user := &models.User{
		ID: "ver-id", Email: "ver@test.com", Name: "Verified",
		Provider: "local", IsActive: true, Accesses: models.StringArray{"user"},
		EmailVerifiedAt: &now,
	}
	if err := userRepo.CreateUser(user); err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user", &auth.Claims{UserID: "ver-id", Provider: "local"})
		return c.Next()
	})
	app.Use(RequireEmailVerified(cfg, userRepo))
	app.Get("/test", func(c *fiber.Ctx) error { return c.SendString("ok") })

	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	resp, _ := app.Test(req)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for verified user, got %d", resp.StatusCode)
	}
}

func TestRequireEmailVerified_ExemptsGuest(t *testing.T) {
	cfg := getTestConfig()
	cfg.Auth.RequireEmailVerification = true
	config.SetForTest(cfg)

	db := testutil.SetupTestDB(t)
	userRepo := repository.NewUserRepository(db)

	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user", &auth.Claims{UserID: "guest-id", Provider: "guest"})
		return c.Next()
	})
	app.Use(RequireEmailVerified(cfg, userRepo))
	app.Get("/test", func(c *fiber.Ctx) error { return c.SendString("ok") })

	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	resp, _ := app.Test(req)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for guest, got %d", resp.StatusCode)
	}
}

func TestRequireEmailVerified_NoUserClaims(t *testing.T) {
	cfg := getTestConfig()
	cfg.Auth.RequireEmailVerification = true
	config.SetForTest(cfg)

	db := testutil.SetupTestDB(t)
	userRepo := repository.NewUserRepository(db)

	app := fiber.New()
	// Don't set user — should get 401
	app.Use(RequireEmailVerified(cfg, userRepo))
	app.Get("/test", func(c *fiber.Ctx) error { return c.SendString("ok") })

	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	resp, _ := app.Test(req)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 when no user claims, got %d", resp.StatusCode)
	}
}

func TestRequireEmailVerified_UserNotFound(t *testing.T) {
	cfg := getTestConfig()
	cfg.Auth.RequireEmailVerification = true
	config.SetForTest(cfg)

	db := testutil.SetupTestDB(t)
	userRepo := repository.NewUserRepository(db)

	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user", &auth.Claims{UserID: "nonexistent-id", Provider: "local"})
		return c.Next()
	})
	app.Use(RequireEmailVerified(cfg, userRepo))
	app.Get("/test", func(c *fiber.Ctx) error { return c.SendString("ok") })

	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	resp, _ := app.Test(req)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 when user not found, got %d", resp.StatusCode)
	}
}

func TestRequireAccess_InsufficientAccess(t *testing.T) {
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user", &auth.Claims{
			UserID:   "user-1",
			Accesses: []string{"user"},
		})
		return c.Next()
	})
	app.Use(func(c *fiber.Ctx) error {
		claims := c.Locals("user").(*auth.Claims)
		for _, access := range claims.Accesses {
			if access == string(models.AccessAdmin) {
				return c.Next()
			}
		}
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Insufficient access rights"})
	})
	app.Get("/admin", func(c *fiber.Ctx) error {
		return c.SendString("admin page")
	})

	req := httptest.NewRequest(http.MethodGet, "/admin", http.NoBody)
	resp, _ := app.Test(req)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != fiber.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}

// ── Item 14: Verified user via DB lookup ─────────────────────────────

// TestRequireEmailVerified_VerifiedUserInDB verifies that the middleware
// allows a verified user through via DB lookup (not from JWT claim).
func TestRequireEmailVerified_VerifiedUserInDB(t *testing.T) {
	cfg := getTestConfig()
	cfg.Auth.RequireEmailVerification = true
	config.SetForTest(cfg)

	db := testutil.SetupTestDB(t)
	userRepo := repository.NewUserRepository(db)

	now := time.Now()
	user := &models.User{
		ID: "ver-user", Email: "ver@test.com", Name: "Verified",
		Provider: "local", IsActive: true, Accesses: models.StringArray{"user"},
		EmailVerifiedAt: &now,
	}
	if err := userRepo.CreateUser(user); err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	// Generate a token with EmailVerifiedAt in claims (mimics real flow)
	token, err := auth.GenerateToken("ver-user", "ver@test.com", "Verified", "local", []string{"user"}, cfg, &now)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	app := fiber.New()
	app.Get("/test", Protected(), RequireEmailVerified(cfg, userRepo))
	app.Get("/test", func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, _ := app.Test(req, -1)
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200 for verified user in DB, got %d: %s", resp.StatusCode, string(respBody))
	}
}

// TestRequireEmailVerified_RejectsStaleClaim verifies that the middleware does NOT
// trust a stale EmailVerifiedAt claim — it always checks DB.
// Regression test: fast-path claim trust was removed (was bug #1).
func TestRequireEmailVerified_RejectsStaleClaim(t *testing.T) {
	cfg := getTestConfig()
	cfg.Auth.RequireEmailVerification = true
	config.SetForTest(cfg)

	db := testutil.SetupTestDB(t)
	userRepo := repository.NewUserRepository(db)

	// Create user with EmailVerifiedAt = nil (unverified in DB)
	user := &models.User{
		ID: "stale-claim-id", Email: "stale@test.com", Name: "Stale",
		Provider: "local", IsActive: true, Accesses: models.StringArray{"user"},
		EmailVerifiedAt: nil, // NOT verified in DB
	}
	if err := userRepo.CreateUser(user); err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	// Generate token with a NON-NIL EmailVerifiedAt claim (simulates stale JWT
	// issued before admin un-verified the user)
	staleTime := time.Now().Add(-1 * time.Hour)
	token, err := auth.GenerateToken("stale-claim-id", "stale@test.com", "Stale", "local", []string{"user"}, cfg, &staleTime)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	app := fiber.New()
	app.Get("/test", Protected(), RequireEmailVerified(cfg, userRepo))
	app.Get("/test", func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, _ := app.Test(req, -1)
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusForbidden {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 403 for stale claim (DB says unverified), got %d: %s", resp.StatusCode, string(respBody))
	}
}
