package handlers

import (
	"os"

	"bedrud/config"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog/log"
)

type CertHandler struct {
	cfg *config.Config
}

func NewCertHandler(cfg *config.Config) *CertHandler {
	return &CertHandler{cfg: cfg}
}

// GetCert returns the server's TLS certificate in PEM format.
// Only available when TLS is enabled.
func (h *CertHandler) GetCert(c *fiber.Ctx) error {
	if !h.cfg.Server.EnableTLS || h.cfg.Server.DisableTLS {
		return c.Status(404).JSON(fiber.Map{"error": "TLS not enabled"})
	}

	certPath := h.cfg.Server.CertFile
	if certPath == "" {
		certPath = "/etc/bedrud/cert.pem"
	}

	pemData, err := os.ReadFile(certPath)
	if err != nil {
		log.Warn().Err(err).Str("path", certPath).Msg("Certificate not found for download")
		return c.Status(404).JSON(fiber.Map{"error": "Certificate not found"})
	}

	c.Set("Content-Type", "application/x-pem-file")
	c.Set("Content-Disposition", "attachment; filename=bedrud-cert.pem")
	return c.Send(pemData)
}
