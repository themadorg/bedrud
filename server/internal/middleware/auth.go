package middleware

import (
	"strings"

	"bedrud/config"
	"bedrud/internal/auth"
	"bedrud/internal/repository"

	"bedrud/internal/models"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog/log"
)

const bearerPrefix = "bearer "

func extractAccessToken(c *fiber.Ctx) string {
	token := ""
	if authHeader := c.Get("Authorization"); authHeader != "" {
		if strings.HasPrefix(strings.ToLower(authHeader), bearerPrefix) {
			token = authHeader[7:]
		} else {
			token = authHeader
		}
	}
	if token == "" {
		token = c.Cookies("access_token")
	}
	return token
}

// Protected middleware validates JWT and checks if user is active (not banned).
func Protected() fiber.Handler {
	return func(c *fiber.Ctx) error {
		token := extractAccessToken(c)

		if token == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Missing authorization",
			})
		}

		claims, err := auth.ValidateToken(token, config.Get())
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Invalid token",
			})
		}

		// Check if user is banned/deactivated (in-memory cache, no DB per request)
		if auth.IsUserBanned(claims.UserID) {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "Account is deactivated",
			})
		}

		// Add claims to context for use in protected routes
		c.Locals("user", claims)
		return c.Next()
	}
}

// OptionalAuth validates JWT when present and sets Locals("user").
// Missing/invalid tokens do not fail the request (handler decides).
func OptionalAuth() fiber.Handler {
	return func(c *fiber.Ctx) error {
		token := extractAccessToken(c)
		if token == "" {
			return c.Next()
		}
		claims, err := auth.ValidateToken(token, config.Get())
		if err != nil {
			return c.Next()
		}
		if auth.IsUserBanned(claims.UserID) {
			return c.Next()
		}
		c.Locals("user", claims)
		return c.Next()
	}
}

// accessLevelWeight maps access levels to numeric weights for hierarchical checks.
// Higher weight = more privileges. Superadmin passes admin/user checks; admin passes user checks.
var accessLevelWeight = map[models.AccessLevel]int{
	models.AccessSuperAdmin: 4,
	models.AccessAdmin:      3,
	models.AccessMod:        2,
	models.AccessUser:       1,
}

// RequireAccess middleware checks for specific access level or higher.
// Hierarchical: superadmin passes admin and user checks; admin passes user checks.
func RequireAccess(requiredAccess models.AccessLevel) fiber.Handler {
	requiredWeight, ok := accessLevelWeight[requiredAccess]
	if !ok {
		log.Warn().Str("requiredAccess", string(requiredAccess)).Msg("Unknown access level in RequireAccess middleware")
		requiredWeight = 9999 // Fail closed: require impossible weight
	}

	return func(c *fiber.Ctx) error {
		raw := c.Locals("user")
		if raw == nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
		}
		claims, ok := raw.(*auth.Claims)
		if !ok || claims == nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
		}

		for _, access := range claims.Accesses {
			if w, exists := accessLevelWeight[models.AccessLevel(access)]; exists && w >= requiredWeight {
				return c.Next()
			}
		}

		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Insufficient access rights",
		})
	}
}

// RequireBearerForMutations rejects state-changing requests that rely on cookie auth
// (no Authorization header). Prevents CSRF where external forms include cookies.
func RequireBearerForMutations() fiber.Handler {
	return func(c *fiber.Ctx) error {
		if c.Method() == "POST" || c.Method() == "PUT" || c.Method() == "DELETE" || c.Method() == "PATCH" {
			authHeader := c.Get("Authorization")
			if authHeader == "" || !strings.HasPrefix(strings.ToLower(authHeader), bearerPrefix) {
				return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
					"error": "Authorization header required for state-changing requests",
				})
			}
		}
		return c.Next()
	}
}

// RejectGuest returns a middleware that blocks requests from guest users.
// Use for profile/password/account endpoints that need a persistent identity.
func RejectGuest() fiber.Handler {
	return func(c *fiber.Ctx) error {
		raw := c.Locals("user")
		if raw == nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
		}
		claims, ok := raw.(*auth.Claims)
		if !ok || claims == nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
		}
		if claims.Provider == "guest" {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Not available for guest accounts"})
		}
		return c.Next()
	}
}

// RequireEmailVerified blocks requests from unverified users when email
// verification is enabled in config. Must be used after Protected().
// Guest users are exempt (no email).
//
// Always verifies against DB — does NOT trust JWT claim to avoid stale
// token bypass if admin un-verifies a user while their token is still valid.
func RequireEmailVerified(cfg *config.Config, userRepo *repository.UserRepository) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if !cfg.Auth.RequireEmailVerification {
			return c.Next()
		}

		raw := c.Locals("user")
		if raw == nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
		}
		claims, ok := raw.(*auth.Claims)
		if !ok || claims == nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
		}

		// Guest users have no email — exempt
		if claims.Provider == "guest" {
			return c.Next()
		}

		// Always verify against DB — don't trust stale JWT claim
		user, err := userRepo.GetUserByID(claims.UserID)
		if err != nil || user == nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "User not found"})
		}
		if user.EmailVerifiedAt == nil {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "Email not verified",
			})
		}
		return c.Next()
	}
}

// Example usage:
// app.Get("/admin", middleware.Protected(), middleware.RequireAccess(models.AccessAdmin), adminHandler)
