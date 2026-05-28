// TODO oncoming feature
package middleware

import (
	"bedrud/internal/repository"

	"github.com/gofiber/fiber/v2"
)

// RecordingsEnabled returns middleware that checks if recordings are enabled
// in the system settings. Returns 403 if disabled.
// This is the first gate — early reject before handler logic runs.
// The service layer performs a second check (belt-and-suspenders).
func RecordingsEnabled(settingsRepo *repository.SettingsRepository) fiber.Handler {
	return func(c *fiber.Ctx) error {
		settings, err := settingsRepo.GetSettings()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to check recording settings",
			})
		}
		if !settings.RecordingsEnabled {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "Recordings are disabled on this server",
			})
		}
		return c.Next()
	}
}
