package handlers

import (
	"errors"
	"io"
	"time"

	"bedrud/internal/auth"
	"bedrud/internal/models"
	"bedrud/internal/storage"

	"github.com/gofiber/fiber/v2"
)

const welcomeSoundFormField = "welcomeSound"

func welcomeSoundLimitsFrom(settings *models.SystemSettings) storage.WelcomeSoundLimits {
	return storage.WelcomeSoundLimits{
		MaxBytes:           settings.WelcomeSoundMaxBytes,
		MaxDurationSeconds: settings.WelcomeSoundMaxDurationSeconds,
		MaxPeakDbfs:        settings.WelcomeSoundMaxPeakDbfs,
		MaxLoudnessLufs:    settings.WelcomeSoundMaxLoudnessLufs,
	}
}

// @Summary Upload welcome sound
// @Description Upload a short MP3 that plays for the room when you join. Requires the feature to be enabled; limited to two changes per UTC day.
// @Tags auth
// @Accept mpfd
// @Produce json
// @Success 200 {object} object
// @Failure 400 {object} auth.ErrorResponse
// @Failure 401 {object} auth.ErrorResponse
// @Failure 403 {object} auth.ErrorResponse
// @Failure 429 {object} auth.ErrorResponse
// @Router /auth/me/welcome-sound [post]
func (h *AuthHandler) UploadWelcomeSound(c *fiber.Ctx) error {
	claims := c.Locals("user").(*auth.Claims)
	if claims.Provider == models.ProviderGuest {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Guests cannot set a welcome sound"})
	}

	settings, err := h.settingsRepo.GetEffectiveSettings()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to load settings"})
	}
	if !settings.WelcomeSoundEnabled {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Welcome sounds are turned off on this server"})
	}
	limits := welcomeSoundLimitsFrom(settings)

	file, err := c.FormFile(welcomeSoundFormField)
	if err != nil || file == nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Sound file is required"})
	}
	if file.Size > int64(limits.MaxBytes) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": storage.ErrWelcomeSoundTooLarge.Error()})
	}
	f, err := file.Open()
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Failed to read upload"})
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, int64(limits.MaxBytes)+1))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to read upload"})
	}

	user, err := h.authService.SetWelcomeSound(claims.UserID, data, limits, time.Now())
	if errors.Is(err, auth.ErrWelcomeSoundDailyLimit) {
		return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{"error": err.Error()})
	}
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(profileResponse(user))
}

// @Summary Remove welcome sound
// @Description Remove your welcome sound. Always allowed and does not count against the daily limit.
// @Tags auth
// @Produce json
// @Success 200 {object} object
// @Failure 401 {object} auth.ErrorResponse
// @Router /auth/me/welcome-sound [delete]
func (h *AuthHandler) DeleteWelcomeSound(c *fiber.Ctx) error {
	claims := c.Locals("user").(*auth.Claims)
	user, err := h.authService.ClearWelcomeSound(claims.UserID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(profileResponse(user))
}
