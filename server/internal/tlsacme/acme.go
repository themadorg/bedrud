// Package tlsacme builds TLS configs for Let's Encrypt (HTTP-01 or DNS-01).
package tlsacme

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"bedrud/config"

	"github.com/caddyserver/certmagic"
	"github.com/libdns/cloudflare"
	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/acme/autocert"
)

const defaultCertCacheDir = "/var/lib/bedrud/certs"

// WildcardNames returns apex + wildcard names for ACME DNS-01 (and related hosts).
// Includes server.domain and webxdc.baseDomain (when set and different).
func WildcardNames(domain, webxdcBase string) []string {
	var out []string
	seen := map[string]struct{}{}
	add := func(h string) {
		h = strings.ToLower(strings.TrimSpace(h))
		h = strings.TrimPrefix(h, "*.")
		if h == "" {
			return
		}
		if _, ok := seen[h]; ok {
			return
		}
		seen[h] = struct{}{}
		out = append(out, h, "*."+h)
	}
	add(domain)
	add(webxdcBase)
	return out
}

// HTTP01Manager builds the classic autocert (HTTP-01) manager for the apex domain
// and any subdomain of it (WebXDC hosts get per-name certs on first request).
func HTTP01Manager(cfg *config.Config) *autocert.Manager {
	domain := strings.ToLower(strings.TrimSpace(cfg.Server.Domain))
	cacheDir := defaultCertCacheDir
	_ = os.MkdirAll(cacheDir, 0o700)

	hostPolicy := func(ctx context.Context, host string) error {
		h := strings.ToLower(strings.TrimSpace(host))
		if h == domain || strings.HasSuffix(h, "."+domain) {
			return nil
		}
		// Also allow webxdc base domain tree when different from apex.
		base := strings.ToLower(strings.TrimSpace(cfg.Webxdc.BaseDomain))
		if base != "" && base != domain {
			if h == base || strings.HasSuffix(h, "."+base) {
				return nil
			}
		}
		return fmt.Errorf("acme: host %q not allowed", host)
	}

	return &autocert.Manager{
		Prompt:     autocert.AcceptTOS,
		HostPolicy: hostPolicy,
		Email:      cfg.Server.Email,
		Cache:      autocert.DirCache(cacheDir),
	}
}

// DNS01Config runs certmagic with Cloudflare DNS-01 and returns a tls.Config
// that serves domain + *.domain (and webxdc base + wildcard when set).
// Certificates are obtained synchronously on start (ManageSync).
func DNS01Config(ctx context.Context, cfg *config.Config) (*tls.Config, error) {
	token := cfg.Server.ACME.CloudflareToken()
	if token == "" {
		return nil, fmt.Errorf("acme dns-01: cloudflare API token required (server.acme.cloudflareAPIToken or CLOUDFLARE_API_TOKEN)")
	}
	provider := strings.ToLower(strings.TrimSpace(cfg.Server.ACME.DNSProvider))
	if provider == "" {
		provider = "cloudflare"
	}
	if provider != "cloudflare" {
		return nil, fmt.Errorf("acme dns-01: unsupported dnsProvider %q (want cloudflare)", provider)
	}

	domain := strings.TrimSpace(cfg.Server.Domain)
	if domain == "" {
		return nil, fmt.Errorf("acme dns-01: server.domain is required")
	}

	cacheDir := defaultCertCacheDir
	if err := os.MkdirAll(cacheDir, 0o700); err != nil {
		return nil, fmt.Errorf("acme cache dir: %w", err)
	}

	// Prefer webxdc base for wildcards when WebXDC is on; always include apex.
	webxdcBase := ""
	if cfg.Webxdc.Enabled {
		webxdcBase = strings.TrimSpace(cfg.Webxdc.BaseDomain)
	}
	names := WildcardNames(domain, webxdcBase)
	if len(names) == 0 {
		return nil, fmt.Errorf("acme dns-01: no domain names to manage")
	}

	magic := certmagic.NewDefault()
	magic.Storage = &certmagic.FileStorage{Path: filepath.Join(cacheDir, "certmagic")}

	issuer := certmagic.NewACMEIssuer(magic, certmagic.ACMEIssuer{
		Email:  cfg.Server.Email,
		Agreed: true,
		// Prefer DNS-01 only; disable HTTP/TLS-ALPN challenges.
		DisableHTTPChallenge:    true,
		DisableTLSALPNChallenge: true,
		DNS01Solver: &certmagic.DNS01Solver{
			DNSManager: certmagic.DNSManager{
				DNSProvider: &cloudflare.Provider{
					APIToken: token,
				},
				// Give Cloudflare a moment after record create before checks.
				PropagationDelay: 15 * time.Second,
			},
		},
	})
	magic.Issuers = []certmagic.Issuer{issuer}

	log.Info().
		Strs("names", names).
		Str("provider", "cloudflare").
		Msg("ACME DNS-01: obtaining/renewing certificates via Cloudflare")

	obtainCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	if err := magic.ManageSync(obtainCtx, names); err != nil {
		return nil, fmt.Errorf("acme dns-01 ManageSync: %w", err)
	}

	tlsCfg := magic.TLSConfig()
	tlsCfg.MinVersion = tls.VersionTLS12
	return tlsCfg, nil
}

// StartHTTPRedirect listens on :80 and redirects all traffic to HTTPS.
// Used when DNS-01 is active (no HTTP-01 challenge handler needed on 80).
func StartHTTPRedirect() {
	go func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			target := "https://" + r.Host + r.URL.RequestURI()
			http.Redirect(w, r, target, http.StatusMovedPermanently)
		})
		log.Info().Msg("➜ HTTP→HTTPS redirect on :80")
		if err := http.ListenAndServe(":80", mux); err != nil {
			log.Error().Err(err).Msg("HTTP redirect server failed")
		}
	}()
}
