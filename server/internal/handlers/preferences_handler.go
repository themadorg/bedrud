package handlers

import (
	"bedrud/internal/auth"
	"bedrud/internal/repository"
	"bytes"
	"encoding/json"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog/log"
)

type PreferencesHandler struct {
	prefsRepo *repository.UserPreferencesRepository
}

func NewPreferencesHandler(repo *repository.UserPreferencesRepository) *PreferencesHandler {
	return &PreferencesHandler{prefsRepo: repo}
}

// @Summary Get user preferences
// @Description Retrieve the authenticated user's preferences as a JSON blob.
// @Tags auth
// @Produce json
// @Success 200 {object} object
// @Failure 500 {object} auth.ErrorResponse
// @Router /auth/preferences [get]
func (h *PreferencesHandler) GetPreferences(c *fiber.Ctx) error {
	claims := c.Locals("user").(*auth.Claims)

	prefs, err := h.prefsRepo.GetByUserID(claims.UserID)
	if err != nil {
		log.Error().Err(err).Str("userID", claims.UserID).Msg("Failed to get preferences")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to get preferences"})
	}
	if prefs == nil {
		return c.JSON(fiber.Map{"preferencesJson": "{}"})
	}
	return c.JSON(fiber.Map{"preferencesJson": prefs.PreferencesJSON})
}

// @Summary Update user preferences
// @Description Update the authenticated user's preferences as a JSON blob (max 4 KB).
// @Tags auth
// @Accept json
// @Produce json
// @Param request body object true "Preferences JSON string"
// @Success 200 {object} object
// @Failure 400 {object} auth.ErrorResponse
// @Failure 413 {object} auth.ErrorResponse
// @Router /auth/preferences [put]
func (h *PreferencesHandler) UpdatePreferences(c *fiber.Ctx) error {
	claims := c.Locals("user").(*auth.Claims)

	var input struct {
		PreferencesJSON string `json:"preferencesJson"`
	}
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid input"})
	}
	if len(input.PreferencesJSON) > 4*1024 {
		return c.Status(fiber.StatusRequestEntityTooLarge).JSON(fiber.Map{"error": "preferencesJson too large (max 4 KB)"})
	}
	if !json.Valid([]byte(input.PreferencesJSON)) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "preferencesJson must be valid JSON"})
	}
	// Ensure it's a JSON object, not a scalar or array
	inputBytes := bytes.TrimSpace([]byte(input.PreferencesJSON))
	if len(inputBytes) < 2 || inputBytes[0] != '{' || inputBytes[len(inputBytes)-1] != '}' {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "preferencesJson must be a JSON object"})
	}

	if err := h.prefsRepo.Upsert(claims.UserID, input.PreferencesJSON); err != nil {
		log.Error().Err(err).Str("userID", claims.UserID).Msg("Failed to save preferences")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to save preferences"})
	}
	return c.JSON(fiber.Map{"message": "Preferences updated"})
}
