package remote

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	TunnelModeDevTunnel = "devtunnel"
	TunnelModeWireGuard = "wireguard"
	TunnelModeSSH       = "ssh"
)

// Config describes the remote debug server profile (server/remote-debug.yaml).
type Config struct {
	// SSH is loaded from server/.env (REMOTE_DEBUG_SSH_*), not from YAML.
	SSH struct {
		Host         string
		User         string
		Port         int
		IdentityFile string
	}

	WireGuard struct {
		Interface      string `yaml:"interface"`
		ConfigFile     string `yaml:"configFile"`
		LocalTunnelIP  string `yaml:"localTunnelIP"`
		RemoteTunnelIP string `yaml:"remoteTunnelIP"`
		// Userspace runs wireguard-go via wg-quick (no kernel module). Default true.
		Userspace *bool `yaml:"userspace"`
	} `yaml:"wireguard"`

	// Tunnel selects how local dev ports reach the remote Traefik server.
	//   devtunnel — outbound TCP mux to devcli agent on server (recommended)
	//   wireguard — WireGuard to server (legacy)
	//   ssh       — rootless SSH -R/-L port forwards (legacy)
	Tunnel struct {
		Mode string `yaml:"mode"`
		SSH  struct {
			RemoteWebPort    int    `yaml:"remoteWebPort"`
			RemoteAPIPort    int    `yaml:"remoteAPIPort"`
			LocalLiveKitPort int    `yaml:"localLiveKitPort"`
			PIDFile          string `yaml:"pidFile"`
		} `yaml:"ssh"`
		DevTunnel struct {
			Port            int    `yaml:"port"`
			Token           string `yaml:"token"`
			TLSFingerprint  string `yaml:"tlsFingerprint"`
			PIDFile         string `yaml:"pidFile"`
		} `yaml:"devtunnel"`
	} `yaml:"tunnel"`

	URLs struct {
		PublicHost      string `yaml:"publicHost"`
		PublicBase      string `yaml:"publicBase"`
		LiveKitHost     string `yaml:"livekitHost"`
		LiveKitInternal string `yaml:"livekitInternal"`
	} `yaml:"urls"`

	Local struct {
		WebPort int `yaml:"webPort"`
		APIPort int `yaml:"apiPort"`
	} `yaml:"local"`

	LiveKit struct {
		APIKey    string `yaml:"apiKey"`
		APISecret string `yaml:"apiSecret"`
	} `yaml:"livekit"`

	Traefik struct {
		DynamicDir  string `yaml:"dynamicDir"`
		EntryPoint  string `yaml:"entryPoint"`
		LiveKitPort int    `yaml:"livekitPort"`
	} `yaml:"traefik"`

	Provision ProvisionConfig `yaml:"provision"`
}

// ProvisionConfig controls devcli remote provision (fresh Debian server bootstrap).
type ProvisionConfig struct {
	StateDir                 string `yaml:"stateDir"`
	WireGuardPort            int    `yaml:"wireguardPort"`
	WireGuardServerInterface string `yaml:"wireguardServerInterface"`
	LiveKitRTCPort           int    `yaml:"livekitRTCPort"`
	LiveKitRTCStart          int    `yaml:"livekitRTCStart"`
	LiveKitRTCEnd            int    `yaml:"livekitRTCEnd"`
	EnableACME               *bool  `yaml:"enableACME"`
	ACMEmail                 string `yaml:"acmeEmail"`
	// WebxdcWildcardCert/Key are remote paths to a TLS cert covering
	// *.publicHost (e.g. LE DNS-01 or Cloudflare Origin). Required for
	// browser-trusted WebXDC hosts; HTTP-01 ACME cannot issue wildcards.
	// Env: REMOTE_DEBUG_WEBXDC_WILDCARD_CERT / REMOTE_DEBUG_WEBXDC_WILDCARD_KEY
	WebxdcWildcardCert string `yaml:"webxdcWildcardCert"`
	WebxdcWildcardKey  string `yaml:"webxdcWildcardKey"`
	// WGEndpoint is loaded from server/.env (not YAML).
	WGEndpoint string
}

// DefaultPath returns server/remote-debug.yaml under repo root.
func DefaultPath(repo string) string {
	return filepath.Join(repo, "server", "remote-debug.yaml")
}

// Load reads and validates remote-debug.yaml from repo.
func Load(repo string) (*Config, error) {
	path := DefaultPath(repo)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("missing %s (copy from server/remote-debug.yaml.example)", path)
		}
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if err := loadSSHFromEnv(repo, &cfg); err != nil {
		return nil, err
	}
	if err := loadProvisionFromEnv(repo, &cfg); err != nil {
		return nil, err
	}
	cfg.applyDefaults()
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (c *Config) applyDefaults() {
	if c.SSH.Port == 0 {
		c.SSH.Port = 22
	}
	if c.Local.WebPort == 0 {
		c.Local.WebPort = 7070
	}
	if c.Local.APIPort == 0 {
		c.Local.APIPort = 7071
	}
	if c.Traefik.EntryPoint == "" {
		c.Traefik.EntryPoint = "websecure"
	}
	if c.Traefik.LiveKitPort == 0 {
		c.Traefik.LiveKitPort = 7072
	}
	if c.Tunnel.SSH.RemoteWebPort == 0 {
		c.Tunnel.SSH.RemoteWebPort = c.Local.WebPort
	}
	if c.Tunnel.SSH.RemoteAPIPort == 0 {
		c.Tunnel.SSH.RemoteAPIPort = c.Local.APIPort
	}
	if c.Tunnel.SSH.LocalLiveKitPort == 0 {
		c.Tunnel.SSH.LocalLiveKitPort = 17072
	}
	if c.URLs.LiveKitInternal == "" {
		switch c.TunnelMode() {
		case TunnelModeSSH, TunnelModeDevTunnel, TunnelModeWireGuard:
			c.URLs.LiveKitInternal = fmt.Sprintf("http://127.0.0.1:%d", c.Tunnel.SSH.LocalLiveKitPort)
		}
	}
	if c.Tunnel.DevTunnel.Port == 0 {
		c.Tunnel.DevTunnel.Port = 7079
	}
	if c.URLs.PublicBase == "" && c.URLs.PublicHost != "" {
		c.URLs.PublicBase = "https://" + c.URLs.PublicHost
	}
	if c.URLs.LiveKitHost == "" && c.URLs.PublicHost != "" {
		c.URLs.LiveKitHost = "wss://" + c.URLs.PublicHost + "/livekit"
	}
	c.SSH.Host = strings.TrimSpace(c.SSH.Host)
	c.SSH.User = strings.TrimSpace(c.SSH.User)
	c.SSH.IdentityFile = expandHome(c.SSH.IdentityFile)
	c.WireGuard.ConfigFile = expandHome(c.WireGuard.ConfigFile)

	if c.Provision.StateDir == "" {
		c.Provision.StateDir = "/etc/bedrud-debug"
	}
	if c.Provision.WireGuardPort == 0 {
		c.Provision.WireGuardPort = 51820
	}
	if c.Provision.WireGuardServerInterface == "" {
		c.Provision.WireGuardServerInterface = "wg0"
	}
	if c.Provision.LiveKitRTCPort == 0 {
		c.Provision.LiveKitRTCPort = 7073
	}
	if c.Provision.LiveKitRTCStart == 0 {
		c.Provision.LiveKitRTCStart = 7080
	}
	if c.Provision.LiveKitRTCEnd == 0 {
		c.Provision.LiveKitRTCEnd = 7180
	}
	if c.Provision.EnableACME == nil {
		t := true
		c.Provision.EnableACME = &t
	}
	if c.LiveKit.APIKey == "" {
		c.LiveKit.APIKey = "devkey"
	}
	if c.WireGuard.RemoteTunnelIP == "" {
		c.WireGuard.RemoteTunnelIP = "10.0.0.1"
	}
	if c.WireGuard.LocalTunnelIP == "" {
		c.WireGuard.LocalTunnelIP = "10.0.0.2"
	}
	if c.WireGuard.Interface == "" {
		c.WireGuard.Interface = "bedrud-debug"
	}
	if c.WireGuard.ConfigFile == "" {
		c.WireGuard.ConfigFile = "~/.config/wireguard/bedrud-debug.conf"
	}
	c.Tunnel.SSH.PIDFile = expandHome(c.Tunnel.SSH.PIDFile)
}

// TunnelMode returns the normalized tunnel transport.
func (c *Config) TunnelMode() string {
	mode := strings.ToLower(strings.TrimSpace(c.Tunnel.Mode))
	if mode == "" {
		return TunnelModeDevTunnel
	}
	return mode
}

// UsesSSHTunnel reports whether remote debug uses SSH port forwards.
func (c *Config) UsesSSHTunnel() bool {
	return c.TunnelMode() == TunnelModeSSH
}

// UsesWireGuard reports whether remote debug uses a WireGuard tunnel.
func (c *Config) UsesWireGuard() bool {
	return c.TunnelMode() == TunnelModeWireGuard
}

// UsesDevTunnel reports whether remote debug uses the devcli tunnel agent.
func (c *Config) UsesDevTunnel() bool {
	return c.TunnelMode() == TunnelModeDevTunnel
}

// DevTunnelServerAddr is host:port for the remote tunnel agent.
func (c *Config) DevTunnelServerAddr() string {
	return fmt.Sprintf("%s:%d", c.SSH.Host, c.Tunnel.DevTunnel.Port)
}

// DevTunnelTLSServerName is the TLS SNI hostname for the tunnel agent.
func (c *Config) DevTunnelTLSServerName() string {
	if host := strings.TrimSpace(c.URLs.PublicHost); host != "" {
		return host
	}
	return c.SSH.Host
}

// DevTunnelTLSCertPath returns the remote tunnel TLS certificate path.
func (c *Config) DevTunnelTLSCertPath() string {
	return c.Provision.StateDir + "/tunnel.crt"
}

// DevTunnelTLSKeyPath returns the remote tunnel TLS private key path.
func (c *Config) DevTunnelTLSKeyPath() string {
	return c.Provision.StateDir + "/tunnel.key"
}

// WireGuardUserspace reports whether the local tunnel uses wireguard-go.
func (c *Config) WireGuardUserspace() bool {
	if c.WireGuard.Userspace != nil {
		return *c.WireGuard.Userspace
	}
	return true
}

func (c *Config) acmeEnabled() bool {
	return c.Provision.EnableACME != nil && *c.Provision.EnableACME
}

// EffectiveTraefikEntryPoint returns websecure when ACME is on, else web (local/LAN testing).
func (c *Config) EffectiveTraefikEntryPoint() string {
	if c.acmeEnabled() {
		if c.Traefik.EntryPoint != "" && c.Traefik.EntryPoint != "web" {
			return c.Traefik.EntryPoint
		}
		return "websecure"
	}
	return "web"
}

// WireGuardEndpoint is the public host:port the local client connects to.
func (c *Config) WireGuardEndpoint() string {
	if c.Provision.WGEndpoint != "" {
		return c.Provision.WGEndpoint
	}
	return fmt.Sprintf("%s:%d", c.SSH.Host, c.Provision.WireGuardPort)
}

func (c *Config) validate() error {
	if c.SSH.Host == "" {
		return fmt.Errorf("%s is required in server/.env", envSSHHost)
	}
	if c.SSH.User == "" {
		return fmt.Errorf("%s is required in server/.env", envSSHUser)
	}
	switch c.TunnelMode() {
	case TunnelModeDevTunnel:
		if c.Tunnel.DevTunnel.Port == 0 {
			return fmt.Errorf("tunnel.devtunnel.port is required in remote-debug.yaml")
		}
	case TunnelModeWireGuard:
		if c.WireGuard.ConfigFile == "" {
			return fmt.Errorf("wireguard.configFile is required in remote-debug.yaml")
		}
		if c.WireGuard.LocalTunnelIP == "" {
			return fmt.Errorf("wireguard.localTunnelIP is required in remote-debug.yaml")
		}
		if c.WireGuard.RemoteTunnelIP == "" {
			return fmt.Errorf("wireguard.remoteTunnelIP is required in remote-debug.yaml")
		}
	case TunnelModeSSH:
		// SSH mode only needs server/.env credentials.
	default:
		return fmt.Errorf("tunnel.mode must be %q, %q, or %q (got %q)", TunnelModeDevTunnel, TunnelModeWireGuard, TunnelModeSSH, c.Tunnel.Mode)
	}
	if c.URLs.PublicHost == "" {
		return fmt.Errorf("urls.publicHost is required in remote-debug.yaml")
	}
	if c.URLs.LiveKitHost == "" {
		return fmt.Errorf("urls.livekitHost is required in remote-debug.yaml")
	}
	if c.URLs.LiveKitInternal == "" {
		return fmt.Errorf("urls.livekitInternal is required (or set wireguard.remoteTunnelIP)")
	}
	if c.Traefik.DynamicDir == "" {
		return fmt.Errorf("traefik.dynamicDir is required in remote-debug.yaml")
	}
	return nil
}

// SSHTarget returns user@host for ssh/scp.
func (c *Config) SSHTarget() string {
	return fmt.Sprintf("%s@%s", c.SSH.User, c.SSH.Host)
}

// WebBackend is the Traefik upstream for the local Vite dev server.
func (c *Config) WebBackend() string {
	if c.UsesSSHTunnel() || c.UsesDevTunnel() {
		return fmt.Sprintf("http://127.0.0.1:%d", c.Tunnel.SSH.RemoteWebPort)
	}
	return fmt.Sprintf("http://%s:%d", c.WireGuard.LocalTunnelIP, c.Local.WebPort)
}

// APIBackend is the Traefik upstream for the local Go API.
func (c *Config) APIBackend() string {
	if c.UsesSSHTunnel() || c.UsesDevTunnel() {
		return fmt.Sprintf("http://127.0.0.1:%d", c.Tunnel.SSH.RemoteAPIPort)
	}
	return fmt.Sprintf("http://%s:%d", c.WireGuard.LocalTunnelIP, c.Local.APIPort)
}

// LiveKitBackend is the Traefik upstream for remote LiveKit (on the debug server).
func (c *Config) LiveKitBackend() string {
	return fmt.Sprintf("http://127.0.0.1:%d", c.Traefik.LiveKitPort)
}

// WebEnv returns env overrides for the local Vite dev server in remote debug mode.
func (c *Config) WebEnv() []string {
	if c.URLs.PublicHost == "" {
		return nil
	}
	// BEDRUD_PUBLIC_BASE drives Vite HMR protocol/port (wss+443 behind Traefik TLS).
	env := []string{
		"BEDRUD_ALLOWED_HOSTS=" + c.URLs.PublicHost,
		"BEDRUD_PUBLIC_BASE=" + c.URLs.PublicBase,
		"BEDRUD_HMR=1",
	}
	// WireGuard TUN breaks SCTP unless the browser uses TURN/TLS relay only.
	if c.UsesWireGuard() {
		env = append(env, "VITE_LIVEKIT_ICE_RELAY=1")
	}
	return env
}

// APIEnv returns env overrides for the local API in remote debug mode.
func (c *Config) APIEnv() []string {
	corsOrigins := []string{
		c.URLs.PublicBase,
		"http://localhost:" + fmt.Sprint(c.Local.WebPort),
		"https://" + c.URLs.PublicHost,
		"http://" + c.URLs.PublicHost,
	}
	seen := make(map[string]struct{}, len(corsOrigins))
	var uniqueOrigins []string
	for _, o := range corsOrigins {
		if o == "" {
			continue
		}
		if _, ok := seen[o]; ok {
			continue
		}
		seen[o] = struct{}{}
		uniqueOrigins = append(uniqueOrigins, o)
	}

	env := []string{
		// Browser WebSocket always hits the remote server (/livekit on Traefik).
		"LIVEKIT_HOST=" + c.URLs.LiveKitHost,
		// Local API → remote LiveKit HTTP API (SSH -L forward).
		"LIVEKIT_INTERNAL_HOST=" + c.URLs.LiveKitInternal,
		"AUTH_FRONTEND_URL=" + c.URLs.PublicBase,
		"CORS_ALLOWED_ORIGINS=" + strings.Join(uniqueOrigins, ","),
	}
	if c.LiveKit.APIKey != "" {
		env = append(env, "LIVEKIT_API_KEY="+c.LiveKit.APIKey)
	}
	if c.LiveKit.APISecret != "" {
		env = append(env, "LIVEKIT_API_SECRET="+c.LiveKit.APISecret)
	}
	return env
}

func expandHome(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if path == "~" {
		home, _ := os.UserHomeDir()
		return home
	}
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[2:])
	}
	return path
}