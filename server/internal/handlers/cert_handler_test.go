package handlers

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"bedrud/config"

	"github.com/gofiber/fiber/v2"
)

func TestGetCert_TLSDisabled(t *testing.T) {
	cfg := &config.Config{Server: config.ServerConfig{EnableTLS: false}}
	h := NewCertHandler(cfg)
	app := fiber.New()
	app.Get("/api/cert", h.GetCert)

	req := httptest.NewRequest(http.MethodGet, "/api/cert", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestGetCert_Success(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "cert.pem")
	pem := []byte("-----BEGIN CERTIFICATE-----\nMIIB\n-----END CERTIFICATE-----\n")
	if err := os.WriteFile(certPath, pem, 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{Server: config.ServerConfig{EnableTLS: true, CertFile: certPath}}
	h := NewCertHandler(cfg)
	app := fiber.New()
	app.Get("/api/cert", h.GetCert)

	req := httptest.NewRequest(http.MethodGet, "/api/cert", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status %d: %s", resp.StatusCode, b)
	}
}

func TestGetCertInfo_NotConfigured(t *testing.T) {
	cfg := &config.Config{Server: config.ServerConfig{EnableTLS: false}}
	h := NewCertHandler(cfg)
	app := fiber.New()
	app.Get("/api/admin/cert-info", h.GetCertInfo)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/cert-info", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}
