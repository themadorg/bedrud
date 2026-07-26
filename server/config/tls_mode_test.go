package config

import "testing"

func TestServerConfig_TLSMode_ManualWinsOverACME(t *testing.T) {
	s := ServerConfig{
		EnableTLS: true,
		UseACME:   true,
		Domain:    "example.com",
		CertFile:  "/etc/ssl/fullchain.pem",
		KeyFile:   "/etc/ssl/privkey.pem",
	}
	if got := s.TLSMode(); got != TLSModeManual {
		t.Fatalf("TLSMode()=%q, want manual (explicit cert files must win over useACME)", got)
	}
}

func TestServerConfig_TLSMode_ACMEWhenNoCertFiles(t *testing.T) {
	s := ServerConfig{
		EnableTLS: true,
		UseACME:   true,
		Domain:    "example.com",
	}
	if got := s.TLSMode(); got != TLSModeACME {
		t.Fatalf("TLSMode()=%q, want acme", got)
	}
}

func TestServerConfig_TLSMode_NoneWhenDisabled(t *testing.T) {
	s := ServerConfig{EnableTLS: false, UseACME: true, Domain: "x.com"}
	if got := s.TLSMode(); got != TLSModeNone {
		t.Fatalf("TLSMode()=%q, want none", got)
	}
	s = ServerConfig{EnableTLS: true, DisableTLS: true, CertFile: "a", KeyFile: "b"}
	if got := s.TLSMode(); got != TLSModeNone {
		t.Fatalf("DisableTLS: TLSMode()=%q, want none", got)
	}
}

func TestServerConfig_TLSMode_ManualDefaults(t *testing.T) {
	// enableTLS without ACME and without paths → manual (defaults under /etc/bedrud)
	s := ServerConfig{EnableTLS: true}
	if got := s.TLSMode(); got != TLSModeManual {
		t.Fatalf("TLSMode()=%q, want manual", got)
	}
	cert, key := s.ResolveCertPaths()
	if cert != "/etc/bedrud/cert.pem" || key != "/etc/bedrud/key.pem" {
		t.Fatalf("defaults: %q %q", cert, key)
	}
}

func TestServerConfig_HasExplicitCertFiles(t *testing.T) {
	s := ServerConfig{}
	if s.HasExplicitCertFiles() {
		t.Fatal("empty paths should not count as explicit")
	}
	// Either path alone is operator intent (other half filled by ResolveCertPaths).
	s = ServerConfig{CertFile: "c.pem"}
	if !s.HasExplicitCertFiles() {
		t.Fatal("certFile alone must count as explicit")
	}
	s = ServerConfig{KeyFile: "k.pem"}
	if !s.HasExplicitCertFiles() {
		t.Fatal("keyFile alone must count as explicit")
	}
	s = ServerConfig{CertFile: "c.pem", KeyFile: "k.pem"}
	if !s.HasExplicitCertFiles() {
		t.Fatal("both set")
	}
}

// Operator-set certFile/keyFile must select manual TLS even when useACME is still true.
func TestServerConfig_TLSMode_CertOnlyWinsOverACME(t *testing.T) {
	s := ServerConfig{
		EnableTLS: true,
		UseACME:   true,
		Domain:    "example.com",
		CertFile:  "/etc/bedrud/cert_p.pem",
	}
	if got := s.TLSMode(); got != TLSModeManual {
		t.Fatalf("TLSMode()=%q, want manual when certFile is set", got)
	}
	cert, key := s.ResolveCertPaths()
	if cert != "/etc/bedrud/cert_p.pem" {
		t.Fatalf("ResolveCertPaths cert=%q, want /etc/bedrud/cert_p.pem", cert)
	}
	if key != DefaultKeyFile {
		t.Fatalf("ResolveCertPaths key=%q, want default %q", key, DefaultKeyFile)
	}
}

func TestServerConfig_TLSMode_ManualWhenUseACMEFalseWithCerts(t *testing.T) {
	s := ServerConfig{
		EnableTLS: true,
		UseACME:   false,
		CertFile:  "/etc/bedrud/cert_p.pem",
		KeyFile:   "/etc/bedrud/key_p.pem",
	}
	if got := s.TLSMode(); got != TLSModeManual {
		t.Fatalf("TLSMode()=%q, want manual", got)
	}
	cert, key := s.ResolveCertPaths()
	if cert != "/etc/bedrud/cert_p.pem" || key != "/etc/bedrud/key_p.pem" {
		t.Fatalf("ResolveCertPaths=%q %q", cert, key)
	}
}

func TestServerConfig_ResolveHTTPPort(t *testing.T) {
	tests := []struct {
		name string
		cfg  ServerConfig
		want string
	}{
		{"empty defaults to 80", ServerConfig{}, "80"},
		{"whitespace only defaults to 80", ServerConfig{HTTPPort: "  \t"}, "80"},
		{"custom unprivileged", ServerConfig{HTTPPort: "8080"}, "8080"},
		{"trims spaces", ServerConfig{HTTPPort: " 8080 "}, "8080"},
		{"explicit 80", ServerConfig{HTTPPort: "80"}, "80"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cfg.ResolveHTTPPort(); got != tt.want {
				t.Fatalf("ResolveHTTPPort()=%q, want %q", got, tt.want)
			}
		})
	}
}

func TestServerConfig_ResolveACMEHTTPSPort(t *testing.T) {
	tests := []struct {
		name string
		cfg  ServerConfig
		want string
	}{
		{"empty defaults to 443", ServerConfig{}, "443"},
		{"whitespace only defaults to 443", ServerConfig{Port: "  "}, "443"},
		{"custom", ServerConfig{Port: "8443"}, "8443"},
		{"trims spaces", ServerConfig{Port: " 8443 "}, "8443"},
		{"explicit 443", ServerConfig{Port: "443"}, "443"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cfg.ResolveACMEHTTPSPort(); got != tt.want {
				t.Fatalf("ResolveACMEHTTPSPort()=%q, want %q", got, tt.want)
			}
		})
	}
}

func TestServerConfig_ListenAddr(t *testing.T) {
	tests := []struct {
		name string
		host string
		port string
		want string
	}{
		{"empty host", "", "80", ":80"},
		{"all interfaces", "0.0.0.0", "8080", "0.0.0.0:8080"},
		{"loopback", "127.0.0.1", "443", "127.0.0.1:443"},
		{"trims host", " 127.0.0.1 ", "443", "127.0.0.1:443"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := (ServerConfig{Host: tt.host}).ListenAddr(tt.port)
			if got != tt.want {
				t.Fatalf("ListenAddr=%q, want %q", got, tt.want)
			}
		})
	}
}

// TestACMEListenAddrsFromConfig covers issue #65: ACME challenge/redirect and
// HTTPS must use httpPort/port, not hard-coded :80/:443 alone.
func TestACMEListenAddrsFromConfig(t *testing.T) {
	tests := []struct {
		name         string
		cfg          ServerConfig
		wantHTTPAddr string
		wantTLSAddr  string
	}{
		{
			name:         "stock ACME defaults (public 80/443)",
			cfg:          ServerConfig{},
			wantHTTPAddr: ":80",
			wantTLSAddr:  ":443",
		},
		{
			name: "non-root / reverse-proxy (issue #65)",
			cfg: ServerConfig{
				Host:     "0.0.0.0",
				HTTPPort: "8080",
				Port:     "8443",
			},
			wantHTTPAddr: "0.0.0.0:8080",
			wantTLSAddr:  "0.0.0.0:8443",
		},
		{
			name: "loopback only",
			cfg: ServerConfig{
				Host:     "127.0.0.1",
				HTTPPort: "18080",
				Port:     "18443",
			},
			wantHTTPAddr: "127.0.0.1:18080",
			wantTLSAddr:  "127.0.0.1:18443",
		},
		{
			name: "httpPort set, ACME TLS still defaults to 443 when port empty",
			cfg: ServerConfig{
				Host:     "0.0.0.0",
				HTTPPort: "8080",
			},
			wantHTTPAddr: "0.0.0.0:8080",
			wantTLSAddr:  "0.0.0.0:443",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			httpAddr := tt.cfg.ListenAddr(tt.cfg.ResolveHTTPPort())
			tlsAddr := tt.cfg.ListenAddr(tt.cfg.ResolveACMEHTTPSPort())
			if httpAddr != tt.wantHTTPAddr {
				t.Errorf("HTTP listen addr=%q, want %q", httpAddr, tt.wantHTTPAddr)
			}
			if tlsAddr != tt.wantTLSAddr {
				t.Errorf("TLS listen addr=%q, want %q", tlsAddr, tt.wantTLSAddr)
			}
			// Regression: custom httpPort must never collapse to bare :80.
			if tt.cfg.HTTPPort != "" && tt.cfg.HTTPPort != "80" && httpAddr == ":80" {
				t.Errorf("custom HTTPPort %q produced :80 (issue #65 regression)", tt.cfg.HTTPPort)
			}
		})
	}
}
