package install

// InstallConfig holds all configuration parameters for the Bedrud installer.
type InstallConfig struct {
	EnableTLS           bool
	DisableTLS          bool
	SelfSigned          bool
	OverrideIP          string
	Domain              string
	Email               string
	Port                string
	CertPath            string
	KeyPath             string
	LKPort              string
	LKTcpPort           string
	LKUdpPort           string
	LKUDPPortRangeStart string
	LKUDPPortRangeEnd   string
	Fresh               bool
	BehindProxy         bool
	ExternalLKURL       string
	LKDomain            string
	LKIP                string
	CertAlgorithm       string
	// Experimental WebXDC (requires Domain)
	EnableWebxdc     bool
	WebxdcBaseDomain string
	WebxdcDNSAck     bool
	// Cloudflare DNS-01 for free wildcard TLS (*.domain / *.webxdcBaseDomain).
	// When set with UseACME, server.acme.challenge=dns-01 and provider=cloudflare.
	CloudflareAPIToken string
	// PreferCloudflareACME is true when the operator chose CF DNS-01 during install.
	PreferCloudflareACME bool
	// Version is recorded to /var/lib/bedrud/version on successful install (from CLI ldflags).
	Version string
}

// SetDefaults populates empty fields with their default values.
func (c *InstallConfig) SetDefaults() {
	if c.OverrideIP == "" {
		c.OverrideIP = getLocalIP()
	}
	if c.Port == "" {
		if c.EnableTLS {
			c.Port = "443"
		} else {
			c.Port = "8090"
		}
	}
	if c.LKPort == "" {
		c.LKPort = "7880"
	}
	if c.LKTcpPort == "" {
		c.LKTcpPort = "7881"
	}
	if c.LKUdpPort == "" {
		c.LKUdpPort = "7882"
	}
	if c.LKUDPPortRangeStart == "" {
		c.LKUDPPortRangeStart = "50000"
	}
	if c.LKUDPPortRangeEnd == "" {
		c.LKUDPPortRangeEnd = "60000"
	}
}
