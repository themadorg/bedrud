package livekit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// generateTempConfig is the whole contract between bedrud's settings and the
// LiveKit process it supervises: everything the embedded server knows about
// TURN, webhooks and its own network identity arrives through the file this
// writes. Nothing else validates it, and a mistake here surfaces as a working
// meeting with a broken edge — a webhook that never fires, TURN that is simply
// absent — rather than as a startup failure.

// isolateTempDir points os.CreateTemp at dir, which is what lets a test assert
// on everything the generator did or did not leave behind.
//
// All three variables, because os.TempDir reads TMPDIR on Unix and TMP then TEMP
// on Windows. Setting only TMPDIR would leave the Windows runs writing into the
// real system temp directory, where "nothing was left behind" would be checking
// an unrelated empty directory and passing for no reason.
func isolateTempDir(t *testing.T, dir string) {
	t.Helper()

	for _, key := range []string{"TMPDIR", "TMP", "TEMP"} {
		t.Setenv(key, dir)
	}
}

// writeConfig calls generateTempConfig into an isolated temp dir and returns the
// parsed result, so assertions are made against what LiveKit would actually read
// rather than against the struct we happened to build.
func writeConfig(t *testing.T, apiKey, apiSecret string, port int, nodeIP, certFile, keyFile, serverHost, httpPort string) (ConfigYAML, string) {
	t.Helper()

	isolateTempDir(t, t.TempDir())

	path, err := generateTempConfig(apiKey, apiSecret, port, nodeIP, certFile, keyFile, serverHost, httpPort)
	if err != nil {
		t.Fatalf("generateTempConfig: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(path) })

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read generated config: %v", err)
	}

	var cfg ConfigYAML
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("generated config is not valid YAML: %v\n%s", err, data)
	}
	return cfg, path
}

func TestGenerateTempConfig_PortAndKeys(t *testing.T) {
	cfg, _ := writeConfig(t, "devkey", "devsecret", 7880, "", "", "", "", "")

	if cfg.Port != 7880 {
		t.Errorf("port = %d, want 7880", cfg.Port)
	}
	// The key map is what authenticates every token bedrud mints; a wrong shape
	// here fails at join time, not at startup.
	if got := cfg.Keys["devkey"]; got != "devsecret" {
		t.Errorf("keys[devkey] = %q, want %q", got, "devsecret")
	}
	if len(cfg.Keys) != 1 {
		t.Errorf("want exactly one key, got %d", len(cfg.Keys))
	}
}

func TestGenerateTempConfig_NodeIP(t *testing.T) {
	tests := []struct {
		name          string
		nodeIP        string
		wantExternal  bool
		wantNodeIP    string
		wantErrSubstr string
	}{
		{
			// An explicit IP turns STUN off. That is the point of setting one:
			// on an air-gapped or firewalled network STUN detection fails and
			// media never connects.
			name:         "explicit IPv4 disables STUN",
			nodeIP:       "203.0.113.10",
			wantExternal: false,
			wantNodeIP:   "203.0.113.10",
		},
		{
			name:         "explicit IPv6 is accepted",
			nodeIP:       "2001:db8::1",
			wantExternal: false,
			wantNodeIP:   "2001:db8::1",
		},
		{
			// No IP means fall back to STUN, which is the documented default
			// rather than an error.
			name:         "empty falls back to STUN",
			nodeIP:       "",
			wantExternal: true,
			wantNodeIP:   "",
		},
		{
			name:          "a hostname is refused",
			nodeIP:        "livekit.example.com",
			wantErrSubstr: "invalid NodeIP",
		},
		{
			name:          "a malformed address is refused",
			nodeIP:        "203.0.113.999",
			wantErrSubstr: "invalid NodeIP",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.wantErrSubstr != "" {
				dir := t.TempDir()
				isolateTempDir(t, dir)

				_, err := generateTempConfig("k", "s", 7880, tc.nodeIP, "", "", "", "")
				if err == nil {
					t.Fatalf("want an error for nodeIP %q, got nil", tc.nodeIP)
				}
				if !strings.Contains(err.Error(), tc.wantErrSubstr) {
					t.Errorf("error = %q, want it to mention %q", err, tc.wantErrSubstr)
				}

				// Rejected before anything is written: a refused config must not
				// leave a file behind for something else to pick up.
				entries, readErr := os.ReadDir(dir)
				if readErr != nil {
					t.Fatalf("read temp dir: %v", readErr)
				}
				if len(entries) != 0 {
					t.Errorf("a rejected config left %d file(s) in the temp dir", len(entries))
				}
				return
			}

			cfg, _ := writeConfig(t, "k", "s", 7880, tc.nodeIP, "", "", "", "")

			if cfg.RTC.UseExternalIP != tc.wantExternal {
				t.Errorf("rtc.use_external_ip = %v, want %v", cfg.RTC.UseExternalIP, tc.wantExternal)
			}
			if cfg.RTC.NodeIP != tc.wantNodeIP {
				t.Errorf("rtc.node_ip = %q, want %q", cfg.RTC.NodeIP, tc.wantNodeIP)
			}
		})
	}
}

func TestGenerateTempConfig_TURN(t *testing.T) {
	tests := []struct {
		name        string
		certFile    string
		keyFile     string
		serverHost  string
		wantEnabled bool
		wantDomain  string
	}{
		{
			name:        "both cert and key enable TURN",
			certFile:    "/etc/bedrud/cert.pem",
			keyFile:     "/etc/bedrud/key.pem",
			serverHost:  "meet.example.com",
			wantEnabled: true,
			wantDomain:  "meet.example.com",
		},
		{
			// Half a TLS pair is not a TLS pair. Enabling TURN with a cert and
			// no key would make LiveKit fail to start rather than degrade.
			name:        "cert without key leaves TURN off",
			certFile:    "/etc/bedrud/cert.pem",
			wantEnabled: false,
		},
		{
			name:        "key without cert leaves TURN off",
			keyFile:     "/etc/bedrud/key.pem",
			wantEnabled: false,
		},
		{
			name:        "neither leaves TURN off",
			wantEnabled: false,
		},
		{
			// TURN is still configured without a domain — the certificate may
			// carry an IP SAN — so a missing host must not silently disable it.
			name:        "no server host still enables TURN",
			certFile:    "/etc/bedrud/cert.pem",
			keyFile:     "/etc/bedrud/key.pem",
			wantEnabled: true,
			wantDomain:  "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg, _ := writeConfig(t, "k", "s", 7880, "", tc.certFile, tc.keyFile, tc.serverHost, "")

			if cfg.TURN.Enabled != tc.wantEnabled {
				t.Fatalf("turn.enabled = %v, want %v", cfg.TURN.Enabled, tc.wantEnabled)
			}
			if !tc.wantEnabled {
				return
			}

			if cfg.TURN.Domain != tc.wantDomain {
				t.Errorf("turn.domain = %q, want %q", cfg.TURN.Domain, tc.wantDomain)
			}
			// The ports are the IANA-assigned TURN pair. They are what a client
			// behind a restrictive firewall dials, so they are part of the
			// contract rather than an implementation detail.
			if cfg.TURN.TLSPort != 5349 {
				t.Errorf("turn.tls_port = %d, want 5349", cfg.TURN.TLSPort)
			}
			if cfg.TURN.UDPPort != 3478 {
				t.Errorf("turn.udp_port = %d, want 3478", cfg.TURN.UDPPort)
			}
			if cfg.TURN.CertFile != tc.certFile {
				t.Errorf("turn.cert_file = %q, want %q", cfg.TURN.CertFile, tc.certFile)
			}
			if cfg.TURN.KeyFile != tc.keyFile {
				t.Errorf("turn.key_file = %q, want %q", cfg.TURN.KeyFile, tc.keyFile)
			}
		})
	}
}

func TestGenerateTempConfig_Webhook(t *testing.T) {
	// The webhook is how bedrud learns a participant dropped. Without it rooms
	// keep phantom participants until something else notices, so its absence is
	// invisible until it matters.
	t.Run("http port configures the callback", func(t *testing.T) {
		cfg, _ := writeConfig(t, "devkey", "devsecret", 7880, "", "", "", "", "8080")

		want := "http://localhost:8080/api/livekit/webhook"
		if len(cfg.Webhook.URLs) != 1 || cfg.Webhook.URLs[0] != want {
			t.Errorf("webhook.urls = %v, want [%s]", cfg.Webhook.URLs, want)
		}
		// Signed with the LiveKit API key, which is what the receiving handler
		// verifies against. A different value here silently drops every event.
		if cfg.Webhook.APIKey != "devkey" {
			t.Errorf("webhook.api_key = %q, want the LiveKit API key %q", cfg.Webhook.APIKey, "devkey")
		}
	})

	t.Run("no http port leaves the webhook unset", func(t *testing.T) {
		cfg, _ := writeConfig(t, "devkey", "devsecret", 7880, "", "", "", "", "")

		if len(cfg.Webhook.URLs) != 0 {
			t.Errorf("webhook.urls = %v, want none", cfg.Webhook.URLs)
		}
	})
}

func TestResolveNodeIP(t *testing.T) {
	tests := []struct {
		name       string
		explicitIP string
		serverHost string
		want       string
	}{
		{
			name:       "an explicit address wins",
			explicitIP: "203.0.113.10",
			serverHost: "198.51.100.5",
			want:       "203.0.113.10",
		},
		{
			// Not validated here — generateTempConfig is what refuses it, and
			// refusing it there costs the whole generated config. See
			// TestGenerateTempConfig_NodeIP.
			name:       "an explicit value is returned unchecked",
			explicitIP: "not-an-ip",
			want:       "not-an-ip",
		},
		{
			name:       "a routable server host is used",
			serverHost: "198.51.100.5",
			want:       "198.51.100.5",
		},
		{
			// Handing LiveKit a loopback or wildcard node IP advertises an
			// address no remote participant can reach, so both fall through to
			// detection rather than being taken at face value.
			name:       "loopback is not a node IP",
			serverHost: "127.0.0.1",
		},
		{
			name:       "the unspecified address is not a node IP",
			serverHost: "0.0.0.0",
		},
		{
			name:       "the IPv6 unspecified address is not a node IP",
			serverHost: "::",
		},
		{
			name:       "a hostname is not an address",
			serverHost: "meet.example.com",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ResolveNodeIP(tc.explicitIP, tc.serverHost)

			if tc.want != "" {
				if got != tc.want {
					t.Errorf("ResolveNodeIP(%q, %q) = %q, want %q", tc.explicitIP, tc.serverHost, got, tc.want)
				}
				return
			}

			// The remaining cases fall through to outbound-interface detection,
			// whose answer depends on the machine. Assert the property that
			// matters instead of a value: whatever it picks, it is never an
			// address a remote peer cannot dial.
			if got == tc.serverHost {
				t.Errorf("ResolveNodeIP(%q, %q) = %q — the rejected host was used anyway",
					tc.explicitIP, tc.serverHost, got)
			}
			if got == "127.0.0.1" || got == "0.0.0.0" || got == "::" {
				t.Errorf("ResolveNodeIP(%q, %q) = %q, which no remote participant can reach",
					tc.explicitIP, tc.serverHost, got)
			}
		})
	}
}

// The generated file is handed to another process by path, so it has to survive
// being read by something that is not this test.
func TestGenerateTempConfig_WritesAReadableFile(t *testing.T) {
	_, path := writeConfig(t, "k", "s", 7880, "", "", "", "", "")

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat generated config: %v", err)
	}
	if info.Size() == 0 {
		t.Error("generated config is empty")
	}
	if filepath.Ext(path) != ".yaml" {
		t.Errorf("generated config is %q; LiveKit is passed it as --config and expects YAML", path)
	}
}
