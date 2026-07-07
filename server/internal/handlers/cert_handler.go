package handlers

import (
	"bedrud/config"
	"bedrud/internal/utils"
	"bytes"
	"os"

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
//
// This endpoint is intentionally unauthenticated — TLS certificates are
// public by nature (sent to every client during the TLS handshake).
// Clients may fetch this endpoint to pin or trust the certificate.
//
// @Summary Download server certificate
// @Description Get the server's TLS certificate in PEM format. Only available when TLS is enabled. Public endpoint — certs are transmitted during TLS handshake.
// @Tags system
// @Produce application/x-pem-file
// @Success 200 {file} binary
// @Failure 404 {object} map[string]interface{}
// @Router /api/cert [get]
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

	if !bytes.Contains(pemData, []byte("-----BEGIN CERTIFICATE-----")) {
		log.Error().Str("path", certPath).Msg("File does not contain a valid PEM certificate")
		return c.Status(500).JSON(fiber.Map{"error": "Certificate file is invalid"})
	}

	c.Set("Content-Type", "application/x-pem-file")
	c.Set("Content-Disposition", "attachment; filename=bedrud-cert.pem")
	return c.Send(pemData)
}

// GetCertInfo returns metadata about the server's TLS certificate.
//
// @Summary Certificate status
// @Description Get TLS certificate metadata (subject, issuer, expiry, SANs, status).
// @Tags admin
// @Produce json
// @Success 200 {object} utils.CertInfo
// @Failure 503 {object} map[string]interface{}
// @Router /api/admin/cert-info [get]
func (h *CertHandler) GetCertInfo(c *fiber.Ctx) error {
	if !h.cfg.Server.EnableTLS || h.cfg.Server.DisableTLS {
		return c.JSON(fiber.Map{
			"enabled": false,
			"status":  "not_configured",
		})
	}

	certFile := h.cfg.Server.CertFile
	keyFile := h.cfg.Server.KeyFile
	if certFile == "" {
		certFile = "/etc/bedrud/cert.pem"
	}
	if keyFile == "" {
		keyFile = "/etc/bedrud/key.pem"
	}

	info, err := utils.ValidateTLSCertPair(certFile, keyFile)
	if err != nil {
		log.Error().Err(err).Msg("TLS certificate validation failed")
		return c.Status(503).JSON(fiber.Map{
			"enabled": true,
			"status":  "error",
			"error":   "TLS certificate validation failed",
		})
	}

	return c.JSON(fiber.Map{
		"enabled":       true,
		"status":        info.Status,
		"daysRemaining": info.DaysRemaining,
		"notAfter":      info.NotAfter,
		"subject":       info.Subject,
		"issuer":        info.Issuer,
		"notBefore":     info.NotBefore,
		"sans":          info.SANs,
	})
}
