package config

import "testing"

func TestWebxdcConfig_Validate_DisabledOK(t *testing.T) {
	w := WebxdcConfig{Enabled: false}
	if err := w.Validate(""); err != nil {
		t.Fatal(err)
	}
	if err := w.Validate("10.0.0.1"); err != nil {
		t.Fatal(err)
	}
}

func TestWebxdcConfig_Validate_RequiresDomainAndBase(t *testing.T) {
	w := WebxdcConfig{Enabled: true, BaseDomain: "wx.example.com", UploadPolicy: "owner_mod"}
	w.applyDefaults()
	if err := w.Validate(""); err == nil {
		t.Fatal("expected error without server domain")
	}
	if err := w.Validate("203.0.113.10"); err == nil {
		t.Fatal("expected error for IP-only server domain")
	}
	w.BaseDomain = ""
	if err := w.Validate("example.com"); err == nil {
		t.Fatal("expected error without baseDomain")
	}
	w.BaseDomain = "10.0.0.1"
	if err := w.Validate("example.com"); err == nil {
		t.Fatal("expected error for IP baseDomain")
	}
	w.BaseDomain = "wx.example.com"
	w.UploadPolicy = "nope"
	if err := w.Validate("example.com"); err == nil {
		t.Fatal("expected bad policy")
	}
	w.UploadPolicy = "owner_mod"
	if err := w.Validate("example.com"); err != nil {
		t.Fatal(err)
	}
}

func TestWebxdcConfig_Active(t *testing.T) {
	w := WebxdcConfig{Enabled: true, BaseDomain: "wx.example.com", UploadPolicy: "any_member"}
	w.applyDefaults()
	if !w.Active("example.com") {
		t.Fatal("expected active")
	}
	w.Enabled = false
	if w.Active("example.com") {
		t.Fatal("expected inactive")
	}
}

func TestLooksLikeIP(t *testing.T) {
	if !looksLikeIP("192.168.1.1") {
		t.Fatal("ipv4")
	}
	if looksLikeIP("example.com") {
		t.Fatal("domain")
	}
	if looksLikeIP("wx.example.com") {
		t.Fatal("subdomain")
	}
}

func TestWebxdcConfig_LocalhostSubdomainDefault(t *testing.T) {
	w := WebxdcConfig{Enabled: true, BaseDomain: "localhost", UploadPolicy: "owner_mod", DevPathMode: false}
	w.applyDefaults()
	if w.UsePathMode() {
		t.Fatal("default local should use subdomain mode (devPathMode false)")
	}
	if err := w.Validate("localhost"); err != nil {
		t.Fatal(err)
	}
}

func TestWebxdcConfig_ExplicitPathMode(t *testing.T) {
	w := WebxdcConfig{Enabled: true, BaseDomain: "localhost", UploadPolicy: "owner_mod", DevPathMode: true}
	w.applyDefaults()
	if !w.UsePathMode() {
		t.Fatal("expected path mode")
	}
}
