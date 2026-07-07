package cli

import (
	"bedrud/internal/clioutput"
	"bedrud/internal/install"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newInstallCmd() *cobra.Command {
	cfg := install.InstallConfig{}
	var (
		enableTLS  bool
		selfSigned bool
		noTLS      bool
		udpRange   string
	)

	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install Bedrud on a Debian/Linux system",
		RunE: func(cmd *cobra.Command, args []string) error {
			if udpRange != "" {
				parts := strings.SplitN(udpRange, "-", 2)
				if len(parts) != 2 {
					return fmt.Errorf("invalid --lk-udp-range %q: expected start-end (e.g. 50000-60000)", udpRange)
				}
				cfg.LKUDPPortRangeStart = parts[0]
				cfg.LKUDPPortRangeEnd = parts[1]
			}
			cfg.EnableTLS = (enableTLS || selfSigned) && !noTLS
			cfg.SelfSigned = selfSigned && !noTLS
			cfg.DisableTLS = noTLS

			if err := install.LinuxInstall(&cfg); err != nil {
				return fmt.Errorf("installation: %w", err)
			}
			return clioutput.Success("✓ Bedrud installed successfully", map[string]any{
				"enableTls":   cfg.EnableTLS,
				"selfSigned":  cfg.SelfSigned,
				"disableTls":  cfg.DisableTLS,
				"behindProxy": cfg.BehindProxy,
				"domain":      cfg.Domain,
			})
		},
	}

	f := cmd.Flags()
	f.BoolVar(&enableTLS, "tls", false, "Enable HTTPS with self-signed certificate (alias for --self-signed)")
	f.BoolVar(&selfSigned, "self-signed", false, "Generate and use a self-signed TLS certificate")
	f.BoolVar(&noTLS, "no-tls", false, "Disable TLS entirely (overrides --tls/--self-signed)")
	f.StringVar(&cfg.OverrideIP, "ip", "", "Override detected IP address")
	f.StringVar(&cfg.Domain, "domain", "", "Domain for Let's Encrypt")
	f.StringVar(&cfg.Email, "email", "", "Email for Let's Encrypt")
	f.StringVar(&cfg.Port, "port", "", "Override default port (443 for TLS, 8090 for HTTP)")
	f.StringVar(&cfg.CertPath, "cert", "", "Path to existing certificate file")
	f.StringVar(&cfg.KeyPath, "key", "", "Path to existing private key file")
	f.StringVar(&cfg.LKPort, "lk-port", "", "Override LiveKit API port (default 7880)")
	f.StringVar(&cfg.LKTcpPort, "lk-tcp-port", "", "Override LiveKit RTC TCP port (default 7881)")
	f.StringVar(&cfg.LKUdpPort, "lk-udp-port", "", "Override LiveKit RTC UDP port (default 7882)")
	f.BoolVar(&cfg.Fresh, "fresh", false, "Remove existing installation before installing")
	f.BoolVar(&cfg.BehindProxy, "behind-proxy", false, "Running behind a CDN/reverse-proxy")
	f.StringVar(&cfg.ExternalLKURL, "external-livekit", "", "URL of a fully external LiveKit server")
	f.StringVar(&cfg.LKDomain, "livekit-domain", "", "Separate domain for the local LiveKit server")
	f.StringVar(&cfg.LKIP, "lk-ip", "", "Separate IP for LiveKit NodeIP (when server behind CDN)")
	f.StringVar(&udpRange, "lk-udp-range", "", "UDP port range for WebRTC media, e.g. 50000-60000")
	f.StringVar(&cfg.CertAlgorithm, "cert-algorithm", "", "Key algorithm for self-signed cert: ed25519 (default), ecdsa256, rsa2048, rsa4096")

	return cmd
}

func newUninstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "uninstall",
		Short: "Uninstall Bedrud from the system",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := install.LinuxUninstall(); err != nil {
				return fmt.Errorf("uninstallation: %w", err)
			}
			return clioutput.Success("✓ Bedrud uninstalled successfully", map[string]bool{"uninstalled": true})
		},
	}
}
