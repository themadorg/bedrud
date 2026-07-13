package cli

import (
	"testing"

	"bedrud/config"
)

func TestBuildCertSANHosts_Basic(t *testing.T) {
	cfg := &config.Config{}
	cfg.Server.Domain = "meet.example.com"
	cfg.Server.Host = "203.0.113.10"
	cfg.Webxdc.Enabled = false

	hosts, wildcard := buildCertSANHosts(cfg)
	if wildcard != "" {
		t.Fatalf("expected no webxdc wildcard, got %q", wildcard)
	}
	want := map[string]bool{
		"meet.example.com": true,
		"203.0.113.10":     true,
		"localhost":        true,
		"127.0.0.1":        true,
		"::1":              true,
	}
	for _, h := range hosts {
		delete(want, h)
	}
	// OutboundIP may add another address; only assert required names are present
	for h := range want {
		if h == "203.0.113.10" || h == "meet.example.com" || h == "localhost" || h == "127.0.0.1" || h == "::1" {
			// re-check presence
			found := false
			for _, got := range hosts {
				if got == h {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("missing SAN %q in %v", h, hosts)
			}
		}
	}
}

func TestBuildCertSANHosts_WebxdcWildcard(t *testing.T) {
	cfg := &config.Config{}
	cfg.Server.Domain = "meet.example.com"
	cfg.Webxdc.Enabled = true
	cfg.Webxdc.BaseDomain = "wx.example.com"
	cfg.Webxdc.UploadPolicy = "owner_mod"

	hosts, wildcard := buildCertSANHosts(cfg)
	if wildcard != "*.wx.example.com" {
		t.Fatalf("wildcard = %q, want *.wx.example.com", wildcard)
	}
	var hasBase, hasWild bool
	for _, h := range hosts {
		if h == "wx.example.com" {
			hasBase = true
		}
		if h == "*.wx.example.com" {
			hasWild = true
		}
	}
	if !hasBase || !hasWild {
		t.Fatalf("expected base + wildcard SANs, got %v", hosts)
	}
}

func TestBuildCertSANHosts_WebxdcPathModeSkipsWildcard(t *testing.T) {
	cfg := &config.Config{}
	cfg.Server.Domain = "localhost"
	cfg.Webxdc.Enabled = true
	cfg.Webxdc.BaseDomain = "localhost"
	cfg.Webxdc.UploadPolicy = "owner_mod"
	cfg.Webxdc.DevPathMode = true

	_, wildcard := buildCertSANHosts(cfg)
	if wildcard != "" {
		t.Fatalf("path mode should skip wildcard, got %q", wildcard)
	}
}

func TestBuildCertSANHosts_WebxdcDisabled(t *testing.T) {
	cfg := &config.Config{}
	cfg.Server.Domain = "meet.example.com"
	cfg.Webxdc.Enabled = false
	cfg.Webxdc.BaseDomain = "wx.example.com"

	_, wildcard := buildCertSANHosts(cfg)
	if wildcard != "" {
		t.Fatalf("disabled webxdc should skip wildcard, got %q", wildcard)
	}
}

func TestMissingSANs(t *testing.T) {
	have := []string{"meet.example.com", "localhost"}
	want := []string{"meet.example.com", "*.wx.example.com", "localhost"}
	missing := missingSANs(have, want)
	if len(missing) != 1 || missing[0] != "*.wx.example.com" {
		t.Fatalf("got %v", missing)
	}
}

func TestValidateKeyAlgorithm(t *testing.T) {
	if err := validateKeyAlgorithm("ed25519"); err != nil {
		t.Fatal(err)
	}
	if err := validateKeyAlgorithm("bogus"); err == nil {
		t.Fatal("expected error")
	}
}
