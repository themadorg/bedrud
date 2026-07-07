package handlers

import (
	"bedrud/config"
	"bedrud/internal/auth"
	"bedrud/internal/database"
	"bedrud/internal/models"
	"bedrud/internal/queue"
	"bedrud/internal/repository"
	"bedrud/internal/storage"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/mail"
	"net/url"
	"strings"
	"time"
	"unicode"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/gorilla/sessions"
	"github.com/markbates/goth/gothic"
	"github.com/rs/zerolog/log"
)

type AuthHandler struct {
	authService     *auth.AuthService
	config          *config.Config
	settingsRepo    *repository.SettingsRepository
	inviteTokenRepo *repository.InviteTokenRepository
	challengeStore  *auth.ChallengeStore
	emailCooldown   *CooldownCache
	verifEventRepo  *repository.VerificationEventRepository
}

func NewAuthHandler(authService *auth.AuthService, cfg *config.Config, settingsRepo *repository.SettingsRepository, inviteTokenRepo *repository.InviteTokenRepository, challengeStore *auth.ChallengeStore, emailCooldown *CooldownCache, verifEventRepo *repository.VerificationEventRepository) *AuthHandler {
	return &AuthHandler{
		authService:     authService,
		config:          cfg,
		settingsRepo:    settingsRepo,
		inviteTokenRepo: inviteTokenRepo,
		challengeStore:  challengeStore,
		emailCooldown:   emailCooldown,
		verifEventRepo:  verifEventRepo,
	}
}

// setAuthCookies writes access and refresh tokens as HTTP-only cookies so the
// browser sends them on every subsequent request without JS involvement.
func setAuthCookies(c *fiber.Ctx, cfg *config.Config, accessToken, refreshToken string) {
	secure := cfg.Server.EnableTLS || cfg.Server.BehindProxy
	domain := cfg.Server.Domain
	sameSite := "Lax"
	if secure {
		sameSite = "None" // Required for cross-site requests over HTTPS
	}
	c.Cookie(&fiber.Cookie{
		Name:     "access_token",
		Value:    accessToken,
		MaxAge:   cfg.Auth.TokenDuration.Int() * 3600,
		HTTPOnly: true,
		Secure:   secure,
		SameSite: sameSite,
		Domain:   domain,
		Path:     "/",
	})
	c.Cookie(&fiber.Cookie{
		Name:     "refresh_token",
		Value:    refreshToken,
		MaxAge:   7 * 24 * 3600, // 7 days, matches GenerateTokenPair
		HTTPOnly: true,
		Secure:   secure,
		SameSite: sameSite,
		Domain:   domain,
		Path:     "/",
	})
}

// clearAuthCookies removes both auth cookies (used on logout).
func clearAuthCookies(c *fiber.Ctx, cfg *config.Config) {
	secure := cfg.Server.EnableTLS || cfg.Server.BehindProxy
	domain := cfg.Server.Domain
	sameSite := "Lax"
	if secure {
		sameSite = "None"
	}
	c.Cookie(&fiber.Cookie{
		Name:     "access_token",
		Value:    "",
		MaxAge:   -1,
		HTTPOnly: true,
		Secure:   secure,
		SameSite: sameSite,
		Domain:   domain,
		Path:     "/",
	})
	c.Cookie(&fiber.Cookie{
		Name:     "refresh_token",
		Value:    "",
		MaxAge:   -1,
		HTTPOnly: true,
		Secure:   secure,
		SameSite: sameSite,
		Domain:   domain,
		Path:     "/",
	})
}

func (h *AuthHandler) Register(c *fiber.Ctx) error {
	var input struct {
		Email       string `json:"email"`
		Password    string `json:"password"`
		Name        string `json:"name"`
		InviteToken string `json:"inviteToken"`
	}

	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid input",
		})
	}

	// Validate email format
	// Canonicalize: normalize Unicode, Punycode domain, lowercase
	input.Email = auth.CanonicalizeEmail(input.Email)

	if _, err := mail.ParseAddress(input.Email); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid email format",
		})
	}

	// Sanitize name: strip control characters and HTML special chars
	// Must run before length validation to prevent null byte / control char bypass
	input.Name = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || r == '<' || r == '>' || r == '&' || r == '"' || r == '\'' {
			return -1
		}
		return r
	}, input.Name)

	// Validate name
	if len(input.Name) < 2 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Name must be at least 2 characters",
		})
	}
	if len(input.Name) > 255 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Name must be at most 255 characters",
		})
	}

	// Check registration settings
	if h.settingsRepo != nil {
		settings, _ := h.settingsRepo.GetSettings()
		if settings != nil {
			if !settings.RegistrationEnabled {
				return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
					"error": "Registration is currently disabled",
				})
			}
			if settings.TokenRegistrationOnly {
				if input.InviteToken == "" {
					return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
						"error": "An invite token is required to register",
					})
				}
				tok, err := h.inviteTokenRepo.GetByToken(input.InviteToken)
				if err != nil || tok == nil || tok.UsedAt != nil || time.Now().After(tok.ExpiresAt) {
					return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
						"error": "Invalid or expired invite token",
					})
				}
				c.Locals("pendingInviteToken", tok.ID)
			}
		}
	}

	if len(input.Password) < MinPasswordLength {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fmt.Sprintf("Password must be at least %d characters", MinPasswordLength),
		})
	}

	if len(input.Password) > MaxPasswordLength {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fmt.Sprintf("Password must be at most %d characters", MaxPasswordLength),
		})
	}

	// Mark invite token as used BEFORE creating user (TOCTOU guard)
	var pendingTokenID string
	if tokID, ok := c.Locals("pendingInviteToken").(string); ok && tokID != "" && h.inviteTokenRepo != nil {
		if err := h.inviteTokenRepo.MarkUsed(tokID, ""); err != nil {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{
				"error": "Invite token already used or invalid",
			})
		}
		pendingTokenID = tokID
	}

	user, err := h.authService.Register(input.Email, input.Password, input.Name)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Email verification flow: when enabled, don't issue tokens — user must verify first.
	if h.config.Auth.RequireEmailVerification {
		enqueueVerificationEmail(h, c, user)

		return c.JSON(fiber.Map{
			"requiresVerification": true,
			"message":              "Please check your email to verify your account",
			"email":                user.Email,
		})
	}

	accessToken, refreshToken, err := auth.GenerateTokenPair(
		user.ID,
		user.Email,
		user.Name,
		user.Provider,
		user.Accesses,
		h.config,
		user.EmailVerifiedAt,
	)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to generate tokens",
		})
	}

	err = h.authService.UpdateRefreshToken(user.ID, refreshToken)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to save refresh token",
		})
	}

	// Update used_by on invite token with the actual user ID
	if pendingTokenID != "" && h.inviteTokenRepo != nil {
		if err := h.inviteTokenRepo.MarkUsed(pendingTokenID, user.ID); err != nil {
			log.Warn().Err(err).Str("tokenID", pendingTokenID).Msg("Failed to update used_by on invite token")
		}
	}

	// Enqueue welcome email (non-blocking — log on error, don't fail the request).
	loginURL := h.config.Auth.FrontendURL
	if loginURL == "" && h.config.Server.Domain != "" {
		loginURL = fmt.Sprintf("https://%s", h.config.Server.Domain)
	}
	if err := queue.Enqueue(context.Background(), database.GetDB(), "send_email",
		queue.SendEmailPayload{
			To:           user.Email,
			Subject:      "Welcome to Bedrud",
			TemplateName: "welcome",
			TemplateData: map[string]any{
				"Name":     user.Name,
				"LoginURL": loginURL,
			},
		},
	); err != nil {
		log.Warn().Err(err).Str("userID", user.ID).Str("email", user.Email).
			Msg("Failed to enqueue welcome email")
	}

	setAuthCookies(c, h.config, accessToken, refreshToken)
	return c.JSON(auth.LoginResponse{
		User: user,
		Token: auth.TokenPair{
			AccessToken:  accessToken,
			RefreshToken: refreshToken,
		},
	})
}

func (h *AuthHandler) Login(c *fiber.Ctx) error {
	var input struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid input",
		})
	}

	// Canonicalize: normalize Unicode, Punycode domain, lowercase
	input.Email = auth.CanonicalizeEmail(input.Email)

	if input.Email == "" || input.Password == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Email and password are required",
		})
	}

	if len(input.Password) > MaxPasswordLength {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fmt.Sprintf("Password must be at most %d characters", MaxPasswordLength),
		})
	}

	loginResponse, err := h.authService.Login(input.Email, input.Password)
	if err != nil {
		// Detect structured email-not-verified error from service
		var emailNotVerified *auth.ErrEmailNotVerified
		if errors.As(err, &emailNotVerified) {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error":                "Please verify your email before signing in",
				"requiresVerification": true,
				"email":                emailNotVerified.Email,
			})
		}
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Invalid credentials",
		})
	}

	setAuthCookies(c, h.config, loginResponse.Token.AccessToken, loginResponse.Token.RefreshToken)
	return c.JSON(loginResponse)
}

// @Summary Guest login
// @Description Join as a guest without creating an account. Guests have limited permissions.
// @Tags auth
// @Accept json
// @Produce json
// @Param request body object true "Guest name"
// @Success 200 {object} auth.LoginResponse
// @Failure 400 {object} auth.ErrorResponse
// @Router /auth/guest-login [post]
func (h *AuthHandler) GuestLogin(c *fiber.Ctx) error {
	var input struct {
		Name string `json:"name"`
	}

	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid input",
		})
	}

	if input.Name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Name is required",
		})
	}

	// Sanitize name before length check to prevent control char bypass
	// (e.g. "\x00a" has raw length 2 but sanitizes to "a")
	input.Name = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || r == '<' || r == '>' || r == '&' || r == '"' || r == '\'' {
			return -1
		}
		return r
	}, input.Name)

	if len(input.Name) < 2 || len(input.Name) > 50 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Name must be between 2 and 50 characters",
		})
	}

	// Check registration settings
	if h.settingsRepo != nil {
		settings, _ := h.settingsRepo.GetSettings()
		if settings != nil {
			if !settings.RegistrationEnabled {
				return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
					"error": "Registration is currently disabled",
				})
			}
			if !settings.GuestLoginEnabled {
				return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
					"error": "Guest login is currently disabled",
				})
			}
		}
	}

	loginResponse, err := h.authService.GuestLogin(input.Name)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create guest user",
		})
	}

	setAuthCookies(c, h.config, loginResponse.Token.AccessToken, loginResponse.Token.RefreshToken)
	return c.JSON(loginResponse)
}

// RefreshRequest represents the refresh token request payload
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" example:"eyJhbGciOiJ..."`
}

// RefreshToken handles token refresh requests
// @Summary Refresh access token
// @Description Get new access token using refresh token
// @Tags auth
// @Accept json
// @Produce json
// @Param request body RefreshRequest true "Refresh token request"
// @Success 200 {object} auth.TokenResponse
// @Failure 400 {object} auth.ErrorResponse
// @Failure 401 {object} auth.ErrorResponse
// @Router /auth/refresh [post]
func (h *AuthHandler) RefreshToken(c *fiber.Ctx) error {
	var input RefreshRequest
	_ = c.BodyParser(&input) // fallback to cookie below

	// Fallback to HTTP-only cookie when body is empty (e.g. cookie-only clients)
	if input.RefreshToken == "" {
		input.RefreshToken = c.Cookies("refresh_token")
	}

	if input.RefreshToken == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "No refresh token provided",
		})
	}

	// Validate the refresh token
	claims, err := h.authService.ValidateRefreshToken(input.RefreshToken)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Invalid or expired refresh token",
		})
	}

	// Re-fetch user from DB for current accesses (may have changed since token was issued)
	user, err := h.authService.GetUserByID(claims.UserID)
	if err != nil || user == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "User not found",
		})
	}
	if !user.IsActive {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Account is deactivated",
		})
	}

	if h.config.Auth.RequireEmailVerification && user.EmailVerifiedAt == nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Please verify your email before signing in",
		})
	}

	// Generate new token pair with fresh user data
	accessToken, refreshToken, err := auth.GenerateTokenPair(
		user.ID,
		user.Email,
		user.Name,
		user.Provider,
		user.Accesses,
		h.config,
		user.EmailVerifiedAt,
	)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to generate tokens",
		})
	}

	// Atomically rotate the refresh token — only succeeds if the old token
	// hasn't already been rotated (prevents token reuse race condition).
	if err := h.authService.RotateRefreshToken(user.ID, input.RefreshToken, refreshToken); err != nil {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"error": "Refresh token has already been rotated",
		})
	}

	setAuthCookies(c, h.config, accessToken, refreshToken)
	return c.JSON(fiber.Map{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
	})
}

func (h *AuthHandler) GetMe(c *fiber.Ctx) error {
	claims := c.Locals("user").(*auth.Claims)
	user, err := h.authService.GetUserByID(claims.UserID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get user",
		})
	}
	if user == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "User not found",
		})
	}

	return c.JSON(user)
}

func profileResponse(user *models.User) fiber.Map {
	return fiber.Map{
		"id":        user.ID,
		"name":      user.Name,
		"email":     user.Email,
		"provider":  user.Provider,
		"accesses":  user.Accesses,
		"avatarUrl": user.AvatarURL,
	}
}

// @Summary Upload profile avatar
// @Description Upload a profile photo stored on the server
// @Tags auth
// @Accept mpfd
// @Produce json
// @Success 200 {object} object
// @Failure 400 {object} auth.ErrorResponse
// @Failure 401 {object} auth.ErrorResponse
// @Router /auth/me/avatar [post]
func (h *AuthHandler) UploadAvatar(c *fiber.Ctx) error {
	claims := c.Locals("user").(*auth.Claims)
	file, err := c.FormFile("avatar")
	if err != nil || file == nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Avatar file is required"})
	}
	if file.Size > storage.AvatarMaxBytes() {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Avatar too large (max 2 MB)"})
	}

	f, err := file.Open()
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Failed to read upload"})
	}
	defer func() { _ = f.Close() }()

	data, err := io.ReadAll(io.LimitReader(f, storage.AvatarMaxBytes()+1))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to read upload"})
	}
	if int64(len(data)) > storage.AvatarMaxBytes() {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Avatar too large (max 2 MB)"})
	}

	url, err := storage.SaveUserAvatar(claims.UserID, data)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	user, err := h.authService.UpdateAvatarURL(claims.UserID, url)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(profileResponse(user))
}

// @Summary Remove profile avatar
// @Description Remove a custom uploaded profile photo
// @Tags auth
// @Produce json
// @Success 200 {object} object
// @Failure 401 {object} auth.ErrorResponse
// @Router /auth/me/avatar [delete]
func (h *AuthHandler) DeleteAvatar(c *fiber.Ctx) error {
	claims := c.Locals("user").(*auth.Claims)
	user, err := h.authService.ClearAvatar(claims.UserID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(profileResponse(user))
}

// @Summary Update profile
// @Description Update user name or email. Email change triggers verification flow and issues new tokens.
// @Tags auth
// @Accept json
// @Produce json
// @Param request body object true "Profile update fields"
// @Success 200 {object} object
// @Failure 400 {object} auth.ErrorResponse
// @Failure 401 {object} auth.ErrorResponse
// @Router /auth/me [put]
func (h *AuthHandler) UpdateProfile(c *fiber.Ctx) error {
	claims := c.Locals("user").(*auth.Claims)
	var input struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid input"})
	}

	// Handle email change
	var newAccessToken, newRefreshToken string
	if input.Email != "" && auth.CanonicalizeEmail(input.Email) != auth.CanonicalizeEmail(claims.Email) {
		// Block email change for OAuth-only users — changing email disconnects OAuth identity
		if claims.Provider != "local" && claims.Provider != "passkey" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cannot change email for OAuth accounts"})
		}
		if _, err := mail.ParseAddress(input.Email); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid email format"})
		}

		if err := h.authService.ChangeEmail(claims.UserID, input.Email); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}

		// Rate limit verification email sends (prevents SMTP queue flooding)
		// Email still changes, user can resend via the resend endpoint (which has its own rate limits)
		emailChangeKey := "email_change:" + claims.UserID
		if h.emailCooldown.Allow(emailChangeKey) {
			h.auditVerificationEvent(claims.UserID, input.Email, models.VerificationEmailChange, c)
			enqueueVerificationEmail(h, c, &models.User{
				ID:    claims.UserID,
				Email: input.Email,
				Name:  input.Name,
			})
		}

		// Revoke old access token so it can't be used with old email
		oldAccessToken := strings.TrimPrefix(c.Get("Authorization"), "Bearer ")
		if oldAccessToken == "" {
			oldAccessToken = c.Cookies("access_token")
		}
		if oldAccessToken != "" {
			auth.RevokeAccessToken(oldAccessToken, h.config)
		}

		// Issue new JWT with updated email so user stays logged in after email change.
		// ChangeEmail clears the refresh token, so we generate new ones here.
		var tokErr error
		newAccessToken, newRefreshToken, tokErr = auth.GenerateTokenPair(
			claims.UserID,
			input.Email,
			input.Name,
			claims.Provider,
			claims.Accesses,
			h.config,
			nil, // EmailVerifiedAt reset after change
		)
		if tokErr == nil {
			_ = h.authService.UpdateRefreshToken(claims.UserID, newRefreshToken)
			setAuthCookies(c, h.config, newAccessToken, newRefreshToken)
		} else {
			log.Warn().Err(tokErr).Str("userID", claims.UserID).Msg("Failed to generate new tokens after email change")
		}
	}

	// Sanitize name: strip control characters and HTML special chars
	// Must run before length validation to prevent null byte / control char bypass
	input.Name = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || r == '<' || r == '>' || r == '&' || r == '"' || r == '\'' {
			return -1
		}
		return r
	}, input.Name)

	// Update name
	if len(input.Name) < 2 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Name must be at least 2 characters"})
	}
	if len(input.Name) > 255 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Name must be at most 255 characters"})
	}
	user, err := h.authService.UpdateProfile(claims.UserID, input.Name)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	res := profileResponse(user)
	if input.Email != "" && auth.CanonicalizeEmail(input.Email) != auth.CanonicalizeEmail(claims.Email) {
		res["requiresVerification"] = true
		res["message"] = "Verification email sent to new address"
		if newAccessToken != "" && newRefreshToken != "" {
			res["tokens"] = fiber.Map{
				"accessToken":  newAccessToken,
				"refreshToken": newRefreshToken,
			}
		}
	}
	return c.JSON(res)
}

// @Summary Change password
// @Description Change the current user's password. Requires current password for verification.
// @Tags auth
// @Accept json
// @Produce json
// @Param request body object true "Current and new password"
// @Success 200 {object} object
// @Failure 400 {object} auth.ErrorResponse
// @Failure 401 {object} auth.ErrorResponse
// @Router /auth/password [put]
func (h *AuthHandler) ChangePassword(c *fiber.Ctx) error {
	claims := c.Locals("user").(*auth.Claims)
	var input struct {
		CurrentPassword string `json:"currentPassword"`
		NewPassword     string `json:"newPassword"`
	}
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid input"})
	}
	if len(input.NewPassword) < MinPasswordLength {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": fmt.Sprintf("New password must be at least %d characters", MinPasswordLength)})
	}
	if len(input.NewPassword) > MaxPasswordLength {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": fmt.Sprintf("New password must be at most %d characters", MaxPasswordLength)})
	}
	// Extract access token to revoke it after password change
	accessToken := strings.TrimPrefix(c.Get("Authorization"), "Bearer ")
	if accessToken == "" {
		accessToken = c.Cookies("access_token")
	}
	if err := h.authService.ChangePassword(claims.UserID, input.CurrentPassword, input.NewPassword, accessToken); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	// Enqueue password change confirmation email (non-blocking).
	if err := queue.Enqueue(context.Background(), database.GetDB(), "send_email",
		queue.SendEmailPayload{
			To:           claims.Email,
			Subject:      "Your Bedrud password was changed",
			TemplateName: "password_changed",
			TemplateData: map[string]any{
				"IPAddress": c.IP(),
			},
		},
	); err != nil {
		log.Warn().Err(err).Str("userID", claims.UserID).Str("email", claims.Email).
			Msg("Failed to enqueue password change email")
	}

	return c.JSON(fiber.Map{"message": "Password updated successfully"})
}

// LogoutRequest represents the logout request payload
type LogoutRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// @Summary Logout user
// @Description Invalidate refresh token and logout the current user.
// @Tags auth
// @Accept json
// @Produce json
// @Param request body LogoutRequest true "Logout request"
// @Success 200 {object} map[string]string
// @Failure 401 {object} auth.ErrorResponse
// @Router /auth/logout [post]
func (h *AuthHandler) Logout(c *fiber.Ctx) error {
	var input LogoutRequest
	_ = c.BodyParser(&input) // non-fatal — fallback to cookie below

	// Get user from context (set by auth middleware)
	claims := c.Locals("user").(*auth.Claims)

	// Fallback to cookie when body is empty or parse fails
	if input.RefreshToken == "" {
		input.RefreshToken = c.Cookies("refresh_token")
	}

	// Extract the raw access token from Authorization header or cookie
	accessToken := strings.TrimPrefix(c.Get("Authorization"), "Bearer ")
	if accessToken == "" {
		accessToken = c.Cookies("access_token")
	}

	// Revoke access token and block refresh token (best-effort; clear cookies regardless)
	if input.RefreshToken != "" {
		if err := h.authService.Logout(claims.UserID, input.RefreshToken, accessToken); err != nil {
			log.Error().Err(err).Msg("Failed to invalidate tokens on logout")
		}
	} else if accessToken != "" {
		// No refresh token provided — at least revoke the access token
		auth.RevokeAccessToken(accessToken, h.config)
	}

	clearAuthCookies(c, h.config)
	return c.JSON(fiber.Map{
		"message": "Successfully logged out",
	})
}

// @Summary Request password reset
// @Description Send password reset email. Always returns 202 to prevent email enumeration. Requires SMTP configuration.
// @Tags auth
// @Accept json
// @Produce json
// @Param request body object true "Email address"
// @Success 202 {object} object
// @Failure 400 {object} auth.ErrorResponse
// @Router /auth/forgot-password [post]
func (h *AuthHandler) ForgotPassword(c *fiber.Ctx) error {
	var input struct {
		Email string `json:"email"`
	}
	if err := c.BodyParser(&input); err != nil || input.Email == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Email is required",
		})
	}

	// Canonicalize before hashing and DB lookup
	input.Email = auth.CanonicalizeEmail(input.Email)

	// Email-hash rate limit (same pattern as ResendVerification)
	hash := sha256.Sum256([]byte(input.Email))
	emailHash := hex.EncodeToString(hash[:])
	emailKey := "forgot_email:" + emailHash
	h.emailCooldown.Allow(emailKey) // consume a token; never blocks — uniform 200

	// Look up user by email
	user, err := h.authService.GetUserByEmail(input.Email)
	if err == nil && user != nil && (user.Provider == "local" || user.Provider == "passkey") {
		cooldownKey := "forgot:" + user.ID
		if h.emailCooldown.Allow(cooldownKey) {
			enqueuePasswordResetEmail(h, c, user)
		} else {
			log.Debug().Str("userID", user.ID).Msg("Password reset email suppressed by cooldown")
		}
	}

	// Uniform 200 regardless of email existence or provider
	return c.JSON(fiber.Map{
		"message": "If the account exists, a password reset email has been sent",
	})
}

// @Summary Reset password with token
// @Description Set a new password using the reset token from the email.
// @Tags auth
// @Accept json
// @Produce json
// @Param request body object true "Reset token and new password"
// @Success 200 {object} object
// @Failure 400 {object} auth.ErrorResponse
// @Router /auth/reset-password [post]
func (h *AuthHandler) ResetPassword(c *fiber.Ctx) error {
	var input struct {
		Token       string `json:"token"`
		NewPassword string `json:"newPassword"`
	}
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid input",
		})
	}

	if input.Token == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Token is required",
		})
	}

	if len(input.NewPassword) < MinPasswordLength {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fmt.Sprintf("Password must be at least %d characters", MinPasswordLength),
		})
	}
	if len(input.NewPassword) > MaxPasswordLength {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fmt.Sprintf("Password must be at most %d characters", MaxPasswordLength),
		})
	}

	// Validate the reset token
	userID, tokenEmail, tokenPasswordChangedAt, err := auth.ValidateResetToken(input.Token, h.config)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid or expired reset token",
		})
	}

	// Get user to verify they exist and are local/passkey provider
	user, err := h.authService.GetUserByID(userID)
	if err != nil || user == nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid or expired reset token",
		})
	}

	// Verify token email matches user's current email (defense against email reuse after change)
	if tokenEmail != "" && auth.CanonicalizeEmail(tokenEmail) != auth.CanonicalizeEmail(user.Email) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid or expired reset token",
		})
	}

	// Check that the token wasn't issued before the user's last password change.
	// This prevents reuse of old reset tokens after a password has been changed.
	// If tokenPasswordChangedAt is nil (token issued before any password change)
	// and the user now has a PasswordChangedAt, the token must be rejected.
	if user.PasswordChangedAt != nil {
		if tokenPasswordChangedAt == nil || user.PasswordChangedAt.Unix() > *tokenPasswordChangedAt {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid or expired reset token",
			})
		}
	}

	// Only allow password reset for local/passkey accounts, not OAuth
	if user.Provider != "local" && user.Provider != "passkey" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Password reset is not available for OAuth accounts",
		})
	}

	// Reset password (clears refresh token, revokes sessions)
	if err := h.authService.ResetPassword(user.ID, input.NewPassword); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to reset password",
		})
	}

	// Enqueue password changed notification email (non-blocking)
	if err := queue.Enqueue(context.Background(), database.GetDB(), "send_email",
		queue.SendEmailPayload{
			To:           user.Email,
			Subject:      "Your Bedrud password was changed",
			TemplateName: "password_changed",
			TemplateData: map[string]any{
				"IPAddress": c.IP(),
			},
		},
	); err != nil {
		log.Warn().Err(err).Str("userID", user.ID).Str("email", user.Email).
			Msg("Failed to enqueue password changed email")
	}

	return c.JSON(fiber.Map{
		"message": "Password has been reset successfully. You can now log in with your new password.",
	})
}

// enqueuePasswordResetEmail generates a reset token and enqueues a send_email job.
func enqueuePasswordResetEmail(h *AuthHandler, c *fiber.Ctx, user *models.User) {
	token, err := auth.GenerateResetToken(user.ID, user.Email, user.PasswordChangedAt, h.config)
	if err != nil {
		log.Error().Err(err).Str("userID", user.ID).Msg("Failed to generate reset token")
		return
	}

	frontendURL := strings.TrimRight(h.config.Auth.FrontendURL, "/")
	if frontendURL == "" && h.config.Server.Domain != "" {
		frontendURL = fmt.Sprintf("https://%s", strings.TrimRight(h.config.Server.Domain, "/"))
	}
	resetURL := frontendURL + "/auth/reset-password?token=" + token

	if err := queue.Enqueue(context.Background(), database.GetDB(), "send_email",
		queue.SendEmailPayload{
			To:           user.Email,
			Subject:      "Reset your Bedrud password",
			TemplateName: "password_reset",
			TemplateData: map[string]any{
				"ResetURL":  resetURL,
				"IPAddress": c.IP(),
			},
		},
	); err != nil {
		log.Warn().Err(err).Str("userID", user.ID).Str("email", user.Email).
			Msg("Failed to enqueue password reset email")
	}
}

// enqueueVerificationEmail generates a verification token and enqueues a send_email job.
// Called during Register (first send) and ResendVerification (resend).
// The cooldown check must happen BEFORE this function is called.
func (h *AuthHandler) auditVerificationEvent(userID, email string, eventType models.VerificationEventType, c *fiber.Ctx) {
	if h.verifEventRepo == nil {
		return
	}
	ip := ""
	if c != nil {
		ip = c.IP()
	}
	if err := h.verifEventRepo.RecordEvent(userID, email, eventType, ip, ""); err != nil {
		log.Warn().Err(err).Str("userID", userID).Msg("Failed to record verification audit event")
	}
}

func enqueueVerificationEmail(h *AuthHandler, c *fiber.Ctx, user *models.User) {
	token, err := auth.GenerateVerificationToken(user.ID, user.Email, h.config)
	if err != nil {
		log.Error().Err(err).Str("userID", user.ID).Msg("Failed to generate verification token")
		return
	}

	frontendURL := strings.TrimRight(h.config.Auth.FrontendURL, "/")
	if frontendURL == "" && h.config.Server.Domain != "" {
		frontendURL = fmt.Sprintf("https://%s", strings.TrimRight(h.config.Server.Domain, "/"))
	}
	verifyURL := frontendURL + "/auth/verify?token=" + token

	if err := queue.Enqueue(context.Background(), database.GetDB(), "send_email",
		queue.SendEmailPayload{
			To:           user.Email,
			Subject:      "Verify your Bedrud email",
			TemplateName: "verify_email",
			TemplateData: map[string]any{
				"Name":      user.Name,
				"VerifyURL": verifyURL,
			},
		},
	); err != nil {
		log.Warn().Err(err).Str("userID", user.ID).Str("email", user.Email).
			Msg("Failed to enqueue verification email")
	}

	// Audit log
	h.auditVerificationEvent(user.ID, user.Email, models.VerificationSent, c)
}

// @Summary Verify email
// @Description Complete email verification with token from verification link. Returns tokens so frontend can auto-login.
// @Tags auth
// @Accept json
// @Produce json
// @Param request body object true "Verification token"
// @Success 200 {object} auth.TokenResponse
// @Failure 400 {object} auth.ErrorResponse
// @Failure 401 {object} auth.ErrorResponse
// @Failure 404 {object} auth.ErrorResponse
// @Failure 409 {object} map[string]interface{}
// @Router /auth/verify [post]
func (h *AuthHandler) VerifyEmail(c *fiber.Ctx) error {
	var input struct {
		Token string `json:"token"`
	}
	if err := c.BodyParser(&input); err != nil || input.Token == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "No verification token provided",
		})
	}

	userID, tokenEmail, err := auth.ValidateVerificationToken(input.Token, h.config)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "The verification link has expired",
		})
	}

	user, err := h.authService.GetUserByID(userID)
	if err != nil || user == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "User not found",
		})
	}

	if tokenEmail != "" && auth.CanonicalizeEmail(tokenEmail) != auth.CanonicalizeEmail(user.Email) {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "The verification link has expired",
		})
	}

	if user.EmailVerifiedAt != nil {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"error":            "Email already verified",
			"already_verified": true,
		})
	}

	now := time.Now()
	user.EmailVerifiedAt = &now
	if err := h.authService.UpdateUser(user); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "An error occurred while verifying your email",
		})
	}

	accessToken, refreshToken, tokErr := auth.GenerateTokenPair(
		user.ID, user.Email, user.Name, user.Provider, user.Accesses, h.config, user.EmailVerifiedAt,
	)
	if tokErr != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to generate tokens",
		})
	}
	_ = h.authService.UpdateRefreshToken(user.ID, refreshToken)

	h.auditVerificationEvent(user.ID, user.Email, models.VerificationSuccess, c)

	return c.JSON(fiber.Map{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
		"verified":      true,
	})
}

// @Summary Check email verification status
// @Description Returns whether the current user's email is verified.
// @Tags auth
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} auth.ErrorResponse
// @Router /auth/verify/status [get]
func (h *AuthHandler) CheckVerificationStatus(c *fiber.Ctx) error {
	raw := c.Locals("user")
	if raw == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}
	claims, ok := raw.(*auth.Claims)
	if !ok || claims == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	verified := false
	// First check if we have an in-claim cached value
	// Then fall back to DB for legacy tokens
	user, err := h.authService.GetUserByID(claims.UserID)
	if err == nil && user != nil && user.EmailVerifiedAt != nil {
		verified = true
	}

	return c.JSON(fiber.Map{
		"verified": verified,
		"email":    claims.Email,
	})
}

// @Summary Resend verification email
// @Description Resend the email verification link. Always returns 200 to prevent email enumeration. Silent cooldown via verificationEmailCooldownMins.
// @Tags auth
// @Accept json
// @Produce json
// @Param request body object true "Email address"
// @Success 200 {object} object
// @Failure 400 {object} auth.ErrorResponse
// @Router /auth/verify/resend [post]
func (h *AuthHandler) ResendVerification(c *fiber.Ctx) error {
	var input struct {
		Email string `json:"email"`
	}
	if err := c.BodyParser(&input); err != nil || input.Email == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Email is required",
		})
	}

	// Canonicalize before hashing and DB lookup
	input.Email = auth.CanonicalizeEmail(input.Email)

	// Email-hash rate limit (prevents botnet bypass of IP-based limit)
	hash := sha256.Sum256([]byte(input.Email))
	emailHash := hex.EncodeToString(hash[:])
	emailKey := "resend_email:" + emailHash
	h.emailCooldown.Allow(emailKey) // consume a token; never blocks — uniform 200

	// Look up user by email
	user, err := h.authService.GetUserByEmail(input.Email)
	if err == nil && user != nil && user.EmailVerifiedAt == nil {
		// Silently enforce cooldown — no 429, no timing difference
		cooldownKey := "verify:" + user.ID
		if h.emailCooldown.Allow(cooldownKey) {
			enqueueVerificationEmail(h, c, user)
		}
		h.auditVerificationEvent(user.ID, user.Email, models.VerificationResent, c)
	}

	// Uniform 200 regardless of email existence, verification status, or cooldown
	return c.JSON(fiber.Map{
		"message": "If the account exists, a verification email has been sent",
	})
}

// Passkey handlers

func (h *AuthHandler) getSession(c *fiber.Ctx) (*sessions.Session, *http.Request, error) {
	req := &http.Request{
		Method: http.MethodGet,
		URL: &url.URL{
			Scheme: c.Protocol(),
			Host:   string(c.Context().Host()),
			Path:   c.Path(),
		},
		Header:     make(http.Header),
		RemoteAddr: c.IP(),
	}
	c.Request().Header.VisitAll(func(key, value []byte) {
		req.Header.Add(string(key), string(value))
	})
	sess, err := gothic.Store.Get(req, gothic.SessionName)
	return sess, req, err
}

func (h *AuthHandler) getRPID(c *fiber.Ctx) string {
	rpid := h.config.Server.Domain
	if rpid == "" {
		rpid = c.Hostname()
	}
	return rpid
}

func (h *AuthHandler) getOrigin(c *fiber.Ctx) string {
	origin := h.config.Auth.FrontendURL
	if origin == "" {
		host := string(c.Context().Host())
		proto := c.Protocol()
		// Try to respect X-Forwarded-Proto if available
		if forwardedProto := c.Get("X-Forwarded-Proto"); forwardedProto != "" {
			proto = forwardedProto
		}
		origin = proto + "://" + host
	}
	return origin
}

func (h *AuthHandler) saveSession(c *fiber.Ctx, sess *sessions.Session, req *http.Request) error {
	w := newResponseWriter(c)
	if err := sess.Save(req, w); err != nil {
		return err
	}
	// Copy headers from w.Header() to c.Response().Header
	for key, values := range w.Header() {
		for _, value := range values {
			c.Response().Header.Add(key, value)
		}
	}
	return nil
}

// @Summary Begin passkey registration
// @Description Start FIDO2/WebAuthn registration ceremony for the authenticated user.
// @Tags auth
// @Produce json
// @Success 200 {object} object
// @Failure 500 {object} auth.ErrorResponse
// @Router /auth/passkey/register/begin [post]
func (h *AuthHandler) PasskeyRegisterBegin(c *fiber.Ctx) error {
	claims := c.Locals("user").(*auth.Claims)
	challenge, err := h.authService.BeginRegisterPasskey(claims.UserID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(internalError(err))
	}

	h.challengeStore.Set("passkey_register:"+claims.UserID, challenge, claims.UserID, nil)

	return c.JSON(fiber.Map{
		"challenge": challenge,
		"user": fiber.Map{
			"id":          base64.RawURLEncoding.EncodeToString([]byte(claims.UserID)),
			"name":        claims.Email,
			"displayName": claims.Name,
		},
		"rp": fiber.Map{
			"id":   h.getRPID(c),
			"name": h.getRPID(c),
		},
	})
}

// @Summary Finish passkey registration
// @Description Complete FIDO2/WebAuthn registration with authenticator response.
// @Tags auth
// @Accept json
// @Produce json
// @Param request body object true "WebAuthn registration response"
// @Success 200 {object} object
// @Failure 400 {object} auth.ErrorResponse
// @Router /auth/passkey/register/finish [post]
func (h *AuthHandler) PasskeyRegisterFinish(c *fiber.Ctx) error {
	claims := c.Locals("user").(*auth.Claims)
	var input struct {
		ClientDataJSON    string `json:"clientDataJSON"`
		AttestationObject string `json:"attestationObject"`
	}
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid input"})
	}

	challenge, _, ok := h.challengeStore.GetAndVerify("passkey_register:"+claims.UserID, "")
	if !ok {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Challenge not found or expired"})
	}

	clientData, err := base64.RawURLEncoding.DecodeString(input.ClientDataJSON)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid clientDataJSON encoding"})
	}
	attestation, err := base64.RawURLEncoding.DecodeString(input.AttestationObject)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid attestationObject encoding"})
	}

	err = h.authService.FinishRegisterPasskey(claims.UserID, challenge, clientData, attestation, h.getRPID(c), h.getOrigin(c))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	h.challengeStore.Delete("passkey_register:" + claims.UserID)

	return c.JSON(fiber.Map{"message": "Passkey registered successfully"})
}

// @Summary Begin passkey login
// @Description Start FIDO2/WebAuthn authentication ceremony.
// @Tags auth
// @Produce json
// @Success 200 {object} object
// @Failure 500 {object} auth.ErrorResponse
// @Router /auth/passkey/login/begin [post]
func (h *AuthHandler) PasskeyLoginBegin(c *fiber.Ctx) error {
	challenge, err := h.authService.BeginLoginPasskey()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(internalError(err))
	}

	challengeID := make([]byte, 16)
	if _, err := rand.Read(challengeID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to generate challenge ID"})
	}
	id := base64.RawURLEncoding.EncodeToString(challengeID)

	h.challengeStore.Set("passkey_login:"+id, challenge, "", nil)

	sess, req, err := h.getSession(c)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Session unavailable"})
	}
	sess.Values["passkey_login_challenge_id"] = id
	if err := h.saveSession(c, sess, req); err != nil {
		h.challengeStore.Delete("passkey_login:" + id)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to save session"})
	}

	return c.JSON(fiber.Map{
		"challenge": challenge,
		"rpId":      h.getRPID(c),
	})
}

// @Summary Finish passkey login
// @Description Complete FIDO2/WebAuthn authentication with authenticator response.
// @Tags auth
// @Accept json
// @Produce json
// @Param request body object true "WebAuthn authentication response"
// @Success 200 {object} auth.LoginResponse
// @Failure 400 {object} auth.ErrorResponse
// @Failure 401 {object} auth.ErrorResponse
// @Router /auth/passkey/login/finish [post]
func (h *AuthHandler) PasskeyLoginFinish(c *fiber.Ctx) error {
	var input struct {
		CredentialID      string `json:"credentialId"`
		ClientDataJSON    string `json:"clientDataJSON"`
		AuthenticatorData string `json:"authenticatorData"`
		Signature         string `json:"signature"`
	}
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid input"})
	}

	sess, req, err := h.getSession(c)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Session unavailable"})
	}
	challengeID, ok := sess.Values["passkey_login_challenge_id"].(string)
	if !ok {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Challenge not found or expired"})
	}

	challenge, _, ok := h.challengeStore.GetAndVerify("passkey_login:"+challengeID, "")
	if !ok {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Challenge not found or expired"})
	}

	credID, err := base64.RawURLEncoding.DecodeString(input.CredentialID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid credentialId encoding"})
	}
	clientData, err := base64.RawURLEncoding.DecodeString(input.ClientDataJSON)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid clientDataJSON encoding"})
	}
	authData, err := base64.RawURLEncoding.DecodeString(input.AuthenticatorData)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid authenticatorData encoding"})
	}
	sig, err := base64.RawURLEncoding.DecodeString(input.Signature)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid signature encoding"})
	}

	loginResponse, err := h.authService.FinishLoginPasskey(challenge, credID, clientData, authData, sig, h.getRPID(c), h.getOrigin(c))
	if err != nil {
		var emailNotVerified *auth.ErrEmailNotVerified
		if errors.As(err, &emailNotVerified) {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error":                "Please verify your email before signing in",
				"requiresVerification": true,
				"email":                emailNotVerified.Email,
			})
		}
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": err.Error()})
	}

	h.challengeStore.Delete("passkey_login:" + challengeID)
	delete(sess.Values, "passkey_login_challenge_id")
	_ = h.saveSession(c, sess, req)

	setAuthCookies(c, h.config, loginResponse.Token.AccessToken, loginResponse.Token.RefreshToken)
	return c.JSON(loginResponse)
}

// @Summary Begin passkey signup
// @Description Start FIDO2/WebAuthn registration for a new user (no session required).
// @Tags auth
// @Accept json
// @Produce json
// @Param request body object true "Signup info"
// @Success 200 {object} object
// @Failure 400 {object} auth.ErrorResponse
// @Failure 403 {object} auth.ErrorResponse
// @Router /auth/passkey/signup/begin [post]
func (h *AuthHandler) PasskeySignupBegin(c *fiber.Ctx) error {
	var input struct {
		Email       string `json:"email"`
		Name        string `json:"name"`
		InviteToken string `json:"inviteToken"`
	}
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid input"})
	}

	// Canonicalize: normalize Unicode, Punycode domain, lowercase
	input.Email = auth.CanonicalizeEmail(input.Email)

	if input.Email == "" || input.Name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Email and Name are required"})
	}

	if _, err := mail.ParseAddress(input.Email); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid email format"})
	}

	if len(input.Name) < 2 || len(input.Name) > 100 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Name must be between 2 and 100 characters"})
	}

	// Check registration settings (mirrors Register())
	if h.settingsRepo != nil {
		settings, _ := h.settingsRepo.GetSettings()
		if settings != nil {
			if !settings.RegistrationEnabled {
				return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Registration is currently disabled"})
			}
			if settings.TokenRegistrationOnly {
				if input.InviteToken == "" {
					return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "An invite token is required to register"})
				}
				tok, err := h.inviteTokenRepo.GetByToken(input.InviteToken)
				if err != nil || tok == nil || tok.UsedAt != nil || time.Now().After(tok.ExpiresAt) {
					return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Invalid or expired invite token"})
				}
				c.Locals("pendingPasskeyInviteToken", tok.ID)
			}
		}
	}

	// Check if user already exists
	existing, _ := h.authService.GetUserByEmail(input.Email)
	if existing != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Email already registered"})
	}

	userID := uuid.New().String()
	challenge, err := h.authService.BeginRegisterPasskey(userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(internalError(err))
	}

	challengeID := make([]byte, 16)
	if _, err := rand.Read(challengeID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to generate challenge ID"})
	}
	id := base64.RawURLEncoding.EncodeToString(challengeID)

	extra := map[string]string{
		"email":  input.Email,
		"name":   input.Name,
		"userID": userID,
	}
	if tokenID, ok := c.Locals("pendingPasskeyInviteToken").(string); ok && tokenID != "" {
		extra["inviteToken"] = tokenID
	}
	h.challengeStore.Set("passkey_signup:"+id, challenge, "", extra)

	sess, req, err := h.getSession(c)
	if err != nil {
		h.challengeStore.Delete("passkey_signup:" + id)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Session unavailable"})
	}
	sess.Values["passkey_signup_challenge_id"] = id
	if err := h.saveSession(c, sess, req); err != nil {
		h.challengeStore.Delete("passkey_signup:" + id)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to save session"})
	}

	return c.JSON(fiber.Map{
		"challenge": challenge,
		"user": fiber.Map{
			"id":          base64.RawURLEncoding.EncodeToString([]byte(userID)),
			"name":        input.Email,
			"displayName": input.Name,
		},
		"rp": fiber.Map{
			"id":   h.getRPID(c),
			"name": h.getRPID(c),
		},
	})
}

// @Summary Finish passkey signup
// @Description Complete FIDO2/WebAuthn registration for a new user.
// @Tags auth
// @Accept json
// @Produce json
// @Param request body object true "WebAuthn registration response"
// @Success 200 {object} auth.LoginResponse
// @Failure 400 {object} auth.ErrorResponse
// @Router /auth/passkey/signup/finish [post]
func (h *AuthHandler) PasskeySignupFinish(c *fiber.Ctx) error {
	var input struct {
		ClientDataJSON    string `json:"clientDataJSON"`
		AttestationObject string `json:"attestationObject"`
	}
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid input"})
	}

	sess, req, err := h.getSession(c)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Session unavailable"})
	}
	challengeID, ok := sess.Values["passkey_signup_challenge_id"].(string)
	if !ok {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Signup session expired or not found"})
	}

	challenge, extra, ok := h.challengeStore.GetAndVerify("passkey_signup:"+challengeID, "")
	if !ok {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Signup session expired or not found"})
	}

	email := extra["email"]
	name := extra["name"]
	userID := extra["userID"]
	if email == "" || name == "" || userID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Signup session expired or not found"})
	}

	clientData, err := base64.RawURLEncoding.DecodeString(input.ClientDataJSON)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid clientDataJSON encoding"})
	}
	attestation, err := base64.RawURLEncoding.DecodeString(input.AttestationObject)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid attestationObject encoding"})
	}

	loginResponse, err := h.authService.FinishSignupPasskey(userID, email, name, challenge, clientData, attestation, h.getRPID(c), h.getOrigin(c))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	// Email verification gate: when FinishSignupPasskey returns empty tokens,
	// the user was created but needs to verify their email before accessing the app.
	if h.config.Auth.RequireEmailVerification && loginResponse.Token.AccessToken == "" {
		enqueueVerificationEmail(h, c, loginResponse.User)
		return c.JSON(fiber.Map{
			"requiresVerification": true,
			"message":              "Please check your email to verify your account",
			"email":                loginResponse.User.Email,
		})
	}

	// Mark invite token used if passkey signup required one
	if tokenID := extra["inviteToken"]; tokenID != "" && h.inviteTokenRepo != nil {
		if err := h.inviteTokenRepo.MarkUsed(tokenID, loginResponse.User.ID); err != nil {
			log.Error().Err(err).Str("tokenID", tokenID).Msg("Failed to mark passkey invite token as used")
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Signup succeeded but failed to record token usage",
			})
		}
	}

	h.challengeStore.Delete("passkey_signup:" + challengeID)
	delete(sess.Values, "passkey_signup_challenge_id")
	_ = h.saveSession(c, sess, req)

	setAuthCookies(c, h.config, loginResponse.Token.AccessToken, loginResponse.Token.RefreshToken)
	return c.JSON(loginResponse)
}
