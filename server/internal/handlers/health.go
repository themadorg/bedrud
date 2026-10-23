package handlers

import (
	"time"

	"bedrud/internal/repository"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog/log"
)

// Health returns the liveness handler, which also tells a client adding this server its display name and version.
//
// @Summary Health check endpoint
// @Description Get the health status of the service, plus the instance's display name and version. Needs no session.
// @Description serverName resolves to the admin-set name, then server.domain, then "Bedrud"; it is omitted if settings cannot be read.
// @Tags health
// @Produce json
// @Success 200 {object} map[string]interface{} "status, time, version, serverName"
// @Router /health [get]
// @Router /api/health [get]
func Health(settingsRepo *repository.SettingsRepository, version string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		body := fiber.Map{"status": "healthy", "time": time.Now().Unix(), "version": version}
		// A settings read failure must not turn liveness red, so the name is simply left out.
		if s, err := settingsRepo.GetEffectiveSettings(); err != nil {
			log.Warn().Err(err).Msg("Health check could not read server name")
		} else {
			body["serverName"] = s.ServerName
		}
		return c.JSON(body)
	}
}
